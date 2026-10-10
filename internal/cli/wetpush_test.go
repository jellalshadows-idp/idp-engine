package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
)

func TestWetPushCommand(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	fake.Objects["/repos/acme/idp-claims/git/ref/heads/wet"] = map[string]any{"object": map[string]any{"sha": "head-sha"}}
	fake.Objects["/repos/acme/idp-claims/git/commits/head-sha"] = map[string]any{"tree": map[string]any{"sha": "base-tree"}}
	fake.Objects["/repos/acme/idp-claims/git/trees/base-tree"] = map[string]any{"truncated": false, "tree": []any{}}
	root := t.TempDir()
	writeFileAll(t, filepath.Join(root, "tfstate", "github.tfstate"), "state")
	out := filepath.Join(t.TempDir(), "out")
	env := envOf(map[string]string{"GH_TOKEN": "writer-token", "IDP_GITHUB_API": fake.URL, "GITHUB_OUTPUT": out})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"wet-push", "--repo", "acme/idp-claims", "--root", root, "--path", "tfstate/github.tfstate", "--message", "reconcile"}, &stdout, &stderr, env)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wet-push: commit-sha (1 changed, 0 deleted)") {
		t.Errorf("stdout = %q", stdout.String())
	}
	if got, _ := os.ReadFile(out); string(got) != "commit=commit-sha\n" {
		t.Errorf("GITHUB_OUTPUT = %q", got)
	}
}

func TestWetPushUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"wet-push"},
		{"wet-push", "--repo", "acme/idp-claims", "--root", "r", "--message", "m"},
		{"wet-push", "--repo", "acme", "--root", "r", "--path", "p", "--message", "m"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, envOf(map[string]string{"GH_TOKEN": "t"})); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
