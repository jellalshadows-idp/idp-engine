// Package wetdiff compares a fresh render with the render stored on the wet
// branch and lists the stacks to plan (spec §6.1 step 2, §6.2 step 1.1).
package wetdiff

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Status is how a stack's fresh render relates to the one in wet.
type Status string

const (
	New       Status = "new"       // only in the fresh render
	Changed   Status = "changed"   // in both, with different files
	Unchanged Status = "unchanged" // in both, byte for byte
	Orphan    Status = "orphan"    // only in wet: its claims are gone
)

// Stack is one stack directory and its status.
type Stack struct {
	Path   string `json:"path"`
	Status Status `json:"status"`
}

// Result lists every stack and the ones that need a plan.
type Result struct {
	Stacks   []Stack  `json:"stacks"`   // sorted by path
	Affected []string `json:"affected"` // new, changed and orphan stacks (every stack with all)
}

// StackFile marks a directory as a stack.
const StackFile = "main.tf.json"

type stackFiles map[string][]byte // path inside the stack -> content

// Diff compares the rendered trees at newRoot and wetRoot. wetRoot may not
// exist yet (the first reconcile). With all, every stack of the fresh render
// is affected, for a manual reconcile that must re-plan everything.
func Diff(newRoot, wetRoot string, all bool) (Result, error) {
	fresh, err := readStacks(newRoot)
	if err != nil {
		return Result{}, fmt.Errorf("new render: %w", err)
	}
	stored, err := readStacks(wetRoot)
	if errors.Is(err, fs.ErrNotExist) {
		stored = map[string]stackFiles{}
	} else if err != nil {
		return Result{}, fmt.Errorf("wet render: %w", err)
	}
	res := Result{Stacks: []Stack{}, Affected: []string{}}
	for p, files := range fresh {
		status := New
		if old, ok := stored[p]; ok {
			status = Unchanged
			if !sameFiles(files, old) {
				status = Changed
			}
		}
		res.Stacks = append(res.Stacks, Stack{Path: p, Status: status})
		if all || status != Unchanged {
			res.Affected = append(res.Affected, p)
		}
	}
	for p := range stored {
		if _, ok := fresh[p]; !ok {
			res.Stacks = append(res.Stacks, Stack{Path: p, Status: Orphan})
			res.Affected = append(res.Affected, p)
		}
	}
	sort.Slice(res.Stacks, func(i, j int) bool { return res.Stacks[i].Path < res.Stacks[j].Path })
	sort.Strings(res.Affected)
	return res, nil
}

// readStacks maps every stack directory under root (slash path) to its files.
// .terraform directories are skipped; files outside any stack (the render
// marker) are ignored. A missing root returns an error wrapping fs.ErrNotExist.
func readStacks(root string) (map[string]stackFiles, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	var dirs []string
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case d.IsDir():
			if d.Name() == ".terraform" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(p, StackFile)); err == nil && rel != "." {
				dirs = append(dirs, rel)
			}
			return nil
		case d.Type().IsRegular():
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files[rel] = data
			return nil
		default:
			return fmt.Errorf("%s: not a regular file or directory", rel)
		}
	})
	if err != nil {
		return nil, err
	}
	stacks := map[string]stackFiles{}
	for _, d := range dirs {
		stacks[d] = stackFiles{}
	}
	for rel, data := range files {
		if owner := ownerStack(rel, dirs); owner != "" {
			stacks[owner][strings.TrimPrefix(rel, owner+"/")] = data
		}
	}
	return stacks, nil
}

// ownerStack is the deepest stack directory that contains rel, or "".
func ownerStack(rel string, dirs []string) string {
	owner := ""
	for _, d := range dirs {
		if strings.HasPrefix(rel, d+"/") && len(d) > len(owner) {
			owner = d
		}
	}
	return owner
}

func sameFiles(a, b stackFiles) bool {
	if len(a) != len(b) {
		return false
	}
	for p, data := range a {
		if other, ok := b[p]; !ok || !bytes.Equal(data, other) {
			return false
		}
	}
	return true
}
