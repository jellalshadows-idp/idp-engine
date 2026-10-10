// Package wetpush makes the one commit to the wet branch that ends a reconcile
// (spec §6.2 step 4.3), through the Git Data API (ADR-0019).
package wetpush

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// Pusher commits to one branch of one repository.
type Pusher struct {
	API    *ghapi.Client
	Owner  string
	Repo   string
	Branch string
}

// Result reports the commit made; Commit is empty when nothing changed.
type Result struct {
	Commit  string
	Changed int // files added or modified
	Deleted int
}

type treeEntry struct {
	Path string  `json:"path"`
	Mode string  `json:"mode"`
	Type string  `json:"type"`
	SHA  *string `json:"sha"` // null deletes the path
}

// Sync makes the branch's files under each path equal to the files under
// root/path, in one commit on top of the current head, and updates the
// branch without force. A path is a file or a directory; .terraform
// directories are never synced. Nothing changed means no commit. A path that
// is a single file on the branch is never deleted: a missing local copy is an
// error, because deleting the state is never part of a reconcile.
func (p Pusher) Sync(ctx context.Context, root string, paths []string, message string) (Result, error) {
	if err := validatePaths(paths); err != nil {
		return Result{}, err
	}
	if info, err := os.Stat(root); err != nil {
		return Result{}, fmt.Errorf("root %s: %w", root, err)
	} else if !info.IsDir() {
		return Result{}, fmt.Errorf("root %s: not a directory", root)
	}
	base := "/repos/" + p.Owner + "/" + p.Repo
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := p.API.Get(ctx, base+"/git/ref/heads/"+p.Branch, &ref); err != nil {
		return Result{}, fmt.Errorf("read branch %s: %w", p.Branch, err)
	}
	head := ref.Object.SHA
	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := p.API.Get(ctx, base+"/git/commits/"+head, &commit); err != nil {
		return Result{}, err
	}
	var tree struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"tree"`
	}
	if err := p.API.Get(ctx, base+"/git/trees/"+commit.Tree.SHA+"?recursive=1", &tree); err != nil {
		return Result{}, err
	}
	if tree.Truncated {
		return Result{}, errors.New("the wet tree listing is truncated; refusing to compute deletions from a partial listing")
	}
	remote := map[string]string{} // path -> blob sha, only under the synced paths
	for _, e := range tree.Tree {
		if e.Type == "blob" && underAny(e.Path, paths) {
			remote[e.Path] = e.SHA
		}
	}
	local, err := readLocal(root, paths)
	if err != nil {
		return Result{}, err
	}
	for _, p := range paths {
		if _, onBranch := remote[p]; onBranch {
			if _, ok := local[p]; !ok {
				return Result{}, fmt.Errorf("refusing to delete %s: it is missing under %s (a reconcile never deletes a whole file path; check --root)", p, root)
			}
		}
	}
	var entries []treeEntry
	res := Result{}
	for _, rel := range sortedKeys(local) {
		data := local[rel]
		if remote[rel] == blobSHA(data) {
			continue
		}
		var blob struct {
			SHA string `json:"sha"`
		}
		if err := p.API.Post(ctx, base+"/git/blobs", map[string]string{"content": base64.StdEncoding.EncodeToString(data), "encoding": "base64"}, &blob); err != nil {
			return Result{}, err
		}
		sha := blob.SHA
		entries = append(entries, treeEntry{Path: rel, Mode: "100644", Type: "blob", SHA: &sha})
		res.Changed++
	}
	for _, rel := range sortedKeys(remote) {
		if _, ok := local[rel]; !ok {
			entries = append(entries, treeEntry{Path: rel, Mode: "100644", Type: "blob", SHA: nil})
			res.Deleted++
		}
	}
	if len(entries) == 0 {
		return Result{}, nil
	}
	var newTree, newCommit struct {
		SHA string `json:"sha"`
	}
	if err := p.API.Post(ctx, base+"/git/trees", map[string]any{"base_tree": commit.Tree.SHA, "tree": entries}, &newTree); err != nil {
		return Result{}, err
	}
	if err := p.API.Post(ctx, base+"/git/commits", map[string]any{"message": message, "tree": newTree.SHA, "parents": []string{head}}, &newCommit); err != nil {
		return Result{}, err
	}
	if err := p.API.Patch(ctx, base+"/git/refs/heads/"+p.Branch, map[string]any{"sha": newCommit.SHA, "force": false}, nil); err != nil {
		return Result{}, fmt.Errorf("update branch %s: %w", p.Branch, err)
	}
	res.Commit = newCommit.SHA
	return res, nil
}

func validatePaths(paths []string) error {
	if len(paths) == 0 {
		return errors.New("no paths to sync")
	}
	for _, p := range paths {
		if p == "" || strings.Contains(p, `\`) || path.IsAbs(p) || path.Clean(p) != p || p == "." || strings.HasPrefix(p, "../") || p == ".." {
			return fmt.Errorf("path %q: want a clean relative slash path", p)
		}
		if slices.Contains(strings.Split(p, "/"), ".terraform") {
			return fmt.Errorf("path %q: .terraform directories are never synced", p)
		}
	}
	return nil
}

func underAny(rel string, paths []string) bool {
	for _, p := range paths {
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

// readLocal reads the regular files under root/path for each path, keyed by
// slash path relative to root. A missing path has no files (it was removed).
func readLocal(root string, paths []string) (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, p := range paths {
		abs := filepath.Join(root, filepath.FromSlash(p))
		info, err := os.Lstat(abs)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(abs)
			if err != nil {
				return nil, err
			}
			files[p] = data
			continue
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%s: not a regular file or directory", p)
		}
		err = filepath.WalkDir(abs, func(walked string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".terraform" {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return fmt.Errorf("%s: not a regular file", walked)
			}
			rel, err := filepath.Rel(root, walked)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(walked)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(rel)] = data
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// blobSHA is git's object id for a blob with this content.
func blobSHA(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
