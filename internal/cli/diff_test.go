package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffCommand(t *testing.T) {
	dir := t.TempDir()
	writeFileAll(t, filepath.Join(dir, "new", "github", "main.tf.json"), "{}")
	out := filepath.Join(dir, "github-output")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"diff", "--new", filepath.Join(dir, "new"), "--wet", filepath.Join(dir, "wet")}, &stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "github: new") || !strings.Contains(stdout.String(), "diff: 1 affected stack(s)") {
		t.Errorf("stdout = %q", stdout.String())
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "affected=[\"github\"]\n" {
		t.Errorf("GITHUB_OUTPUT = %q", got)
	}
}

func TestDiffUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"diff"}, {"diff", "--new", "x"}, {"diff", "--new", "x", "--wet", "y", "extra"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, noEnv); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
