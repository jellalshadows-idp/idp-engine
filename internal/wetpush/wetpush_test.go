package wetpush

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

const repoPath = "/repos/acme/idp-claims"

func setup(t *testing.T, remote map[string]string) (*fakegithub.Server, Pusher) {
	t.Helper()
	fake := fakegithub.New(t, "acme")
	fake.Objects[repoPath+"/git/ref/heads/wet"] = map[string]any{"ref": "refs/heads/wet", "object": map[string]any{"sha": "head-sha"}}
	fake.Objects[repoPath+"/git/commits/head-sha"] = map[string]any{"sha": "head-sha", "tree": map[string]any{"sha": "base-tree"}}
	entries := []any{}
	for p, content := range remote {
		entries = append(entries, map[string]any{"path": p, "mode": "100644", "type": "blob", "sha": blobSHA([]byte(content))})
	}
	fake.Objects[repoPath+"/git/trees/base-tree"] = map[string]any{"sha": "base-tree", "truncated": false, "tree": entries}
	return fake, Pusher{API: ghapi.New(fake.URL, "writer-token"), Owner: "acme", Repo: "idp-claims", Branch: "wet"}
}

func writeLocal(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func lastRequest(t *testing.T, fake *fakegithub.Server, method, suffix string) map[string]any {
	t.Helper()
	for i := len(fake.Requests) - 1; i >= 0; i-- {
		if r := fake.Requests[i]; r.Method == method && strings.HasSuffix(r.Path, suffix) {
			return r.Body
		}
	}
	t.Fatalf("no %s ...%s request in %v", method, suffix, fake.Writes)
	return nil
}

func TestSyncCommitsChangesAndDeletionsOnly(t *testing.T) {
	fake, p := setup(t, map[string]string{
		"README.md":                    "# wet",
		"tfstate/github.tfstate":       "old state",
		"rendered/github/main.tf.json": "{}",
		"rendered/github/old.tf.json":  "gone",
	})
	root := writeLocal(t, map[string]string{
		"tfstate/github.tfstate":               "new state",
		"rendered/github/main.tf.json":         "{}",
		"rendered/github/.terraform.lock.hcl":  "lock",
		"rendered/github/.terraform/providers": "binary",
	})
	res, err := p.Sync(context.Background(), root, []string{"tfstate/github.tfstate", "rendered/github"}, "reconcile: abc")
	if err != nil {
		t.Fatal(err)
	}
	if res != (Result{Commit: "commit-sha", Changed: 2, Deleted: 1}) {
		t.Errorf("result = %+v", res)
	}
	tree := lastRequest(t, fake, "POST", "/git/trees")
	if tree["base_tree"] != "base-tree" {
		t.Errorf("base_tree = %v", tree["base_tree"])
	}
	got := map[string]any{}
	for _, e := range tree["tree"].([]any) {
		entry := e.(map[string]any)
		got[entry["path"].(string)] = entry["sha"]
	}
	if len(got) != 3 || got["rendered/github/old.tf.json"] != nil {
		t.Errorf("tree entries = %v, want 2 uploads and a null-sha deletion", got)
	}
	for _, p := range []string{"tfstate/github.tfstate", "rendered/github/.terraform.lock.hcl"} {
		if sha, _ := got[p].(string); !strings.HasPrefix(sha, "blob-") {
			t.Errorf("%s sha = %v, want an uploaded blob", p, got[p])
		}
	}
	commit := lastRequest(t, fake, "POST", "/git/commits")
	if commit["message"] != "reconcile: abc" || commit["tree"] != "tree-sha" || !reflect.DeepEqual(commit["parents"], []any{"head-sha"}) {
		t.Errorf("commit = %v", commit)
	}
	ref := lastRequest(t, fake, "PATCH", "/git/refs/heads/wet")
	if ref["sha"] != "commit-sha" || ref["force"] != false {
		t.Errorf("ref update = %v, want commit-sha without force", ref)
	}
}

func TestSyncNoChangesMakesNoCommit(t *testing.T) {
	fake, p := setup(t, map[string]string{"tfstate/github.tfstate": "same"})
	root := writeLocal(t, map[string]string{"tfstate/github.tfstate": "same"})
	res, err := p.Sync(context.Background(), root, []string{"tfstate/github.tfstate"}, "m")
	if err != nil {
		t.Fatal(err)
	}
	if res != (Result{}) || len(fake.Writes) != 0 {
		t.Errorf("result = %+v, writes = %v, want nothing", res, fake.Writes)
	}
}

func TestSyncCreatesFilesUnderNewPrefix(t *testing.T) {
	fake, p := setup(t, map[string]string{"README.md": "# wet"})
	root := writeLocal(t, map[string]string{"tfstate/github.tfstate": "state", "rendered/github/main.tf.json": "{}"})
	res, err := p.Sync(context.Background(), root, []string{"tfstate/github.tfstate", "rendered/github"}, "first reconcile")
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed != 2 || res.Deleted != 0 || res.Commit != "commit-sha" {
		t.Errorf("result = %+v", res)
	}
	if n := len(lastRequest(t, fake, "POST", "/git/trees")["tree"].([]any)); n != 2 {
		t.Errorf("tree entries = %d, want 2", n)
	}
}

func TestSyncDeletesAPrefixRemovedLocally(t *testing.T) {
	_, p := setup(t, map[string]string{"rendered/aws/dev/_baseline/main.tf.json": "{}"})
	res, err := p.Sync(context.Background(), t.TempDir(), []string{"rendered/aws/dev/_baseline"}, "orphan destroyed")
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted != 1 || res.Changed != 0 {
		t.Errorf("result = %+v", res)
	}
}

func TestSyncRefusesNonFastForward(t *testing.T) {
	fake, p := setup(t, nil)
	fake.RejectRefUpdates[repoPath+"/git/refs/heads/wet"] = true
	root := writeLocal(t, map[string]string{"tfstate/github.tfstate": "state"})
	_, err := p.Sync(context.Background(), root, []string{"tfstate/github.tfstate"}, "m")
	var apiErr *ghapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 422 {
		t.Errorf("err = %v, want the 422 from the ref update", err)
	}
}

func TestSyncRejectsTruncatedTrees(t *testing.T) {
	fake, p := setup(t, nil)
	fake.Objects[repoPath+"/git/trees/base-tree"] = map[string]any{"sha": "base-tree", "truncated": true, "tree": []any{}}
	if _, err := p.Sync(context.Background(), t.TempDir(), []string{"tfstate/github.tfstate"}, "m"); err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Errorf("err = %v, want a truncated-tree error", err)
	}
}

func TestSyncValidatesPaths(t *testing.T) {
	_, p := setup(t, nil)
	for _, bad := range [][]string{nil, {""}, {"../x"}, {"/abs"}, {"a/../b"}, {"a\\b"}, {"rendered/github/.terraform"}, {".terraform"}} {
		if _, err := p.Sync(context.Background(), t.TempDir(), bad, "m"); err == nil {
			t.Errorf("paths %q: want an error", bad)
		}
	}
}

func TestSyncRejectsAMissingRoot(t *testing.T) {
	fake, p := setup(t, map[string]string{"tfstate/github.tfstate": "state"})
	_, err := p.Sync(context.Background(), filepath.Join(t.TempDir(), "nope"), []string{"tfstate/github.tfstate"}, "m")
	if err == nil || len(fake.Writes) != 0 {
		t.Errorf("err = %v, writes = %v, want an error and no writes", err, fake.Writes)
	}
}

func TestSyncRefusesToDeleteAFilePath(t *testing.T) {
	fake, p := setup(t, map[string]string{"tfstate/github.tfstate": "state"})
	_, err := p.Sync(context.Background(), t.TempDir(), []string{"tfstate/github.tfstate"}, "m")
	if err == nil || !strings.Contains(err.Error(), "refusing to delete tfstate/github.tfstate") || len(fake.Writes) != 0 {
		t.Errorf("err = %v, writes = %v, want a refusal and no writes", err, fake.Writes)
	}
}

func TestBlobSHAMatchesGit(t *testing.T) {
	// echo -n "hello" | git hash-object --stdin
	if got := blobSHA([]byte("hello")); got != "b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0" {
		t.Errorf("blobSHA = %s", got)
	}
}
