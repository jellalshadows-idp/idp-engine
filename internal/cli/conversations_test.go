package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
)

func ghEnv(fake *fakegithub.Server) Env {
	return envOf(map[string]string{"GH_TOKEN": "test-token", "IDP_GITHUB_API": fake.URL})
}

func TestCommentCommand(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	body := filepath.Join(t.TempDir(), "comment.md")
	if err := os.WriteFile(body, []byte("<!-- idp-plan -->\nplan"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"comment", "--repo", "acme/idp-claims", "--pr", "7", "--body-file", body}
	for i, want := range []string{"comment: created", "comment: updated"} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, ghEnv(fake)); code != 0 || !strings.Contains(stdout.String(), want) {
			t.Errorf("run %d: exit %d, stdout %q, stderr %q", i, code, stdout.String(), stderr.String())
		}
	}
}

func TestIssueOpenAndClose(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	body := filepath.Join(t.TempDir(), "issue.md")
	if err := os.WriteFile(body, []byte("drift details"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		args []string
		want string
	}{
		{[]string{"issue", "open", "--repo", "acme/idp-claims", "--label", "drift", "--title", "Drift detected", "--body-file", body}, "issue: opened #1"},
		{[]string{"issue", "open", "--repo", "acme/idp-claims", "--label", "drift", "--title", "Drift detected", "--body-file", body}, "issue: updated #1"},
		{[]string{"issue", "close", "--repo", "acme/idp-claims", "--label", "drift", "--title", "Drift detected", "--comment", "No drift."}, "issue: closed #1"},
		{[]string{"issue", "close", "--repo", "acme/idp-claims", "--label", "drift", "--title", "Drift detected"}, "issue: none open"},
	}
	for _, s := range steps {
		var stdout, stderr bytes.Buffer
		if code := Run(s.args, &stdout, &stderr, ghEnv(fake)); code != 0 || !strings.Contains(stdout.String(), s.want) {
			t.Errorf("%v: exit %d, stdout %q, stderr %q, want %q", s.args, code, stdout.String(), stderr.String(), s.want)
		}
	}
}

func TestCommentRejectsAnEmptyMarker(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"comment", "--repo", "acme/idp-claims", "--pr", "7", "--body-file", "f", "--marker", ""},
		&stdout, &stderr, envOf(map[string]string{"GH_TOKEN": "t"}))
	if code != 2 || !strings.Contains(stderr.String(), "idp comment: --marker must not be empty") {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestConversationUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"comment", "--repo", "acme/idp-claims", "--pr", "x", "--body-file", "f"},
		{"comment", "--repo", "acme/idp-claims", "--pr", "7"},
		{"issue"},
		{"issue", "reopen"},
		{"issue", "open", "--repo", "acme/idp-claims", "--label", "drift", "--title", "T"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, envOf(map[string]string{"GH_TOKEN": "t"})); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
