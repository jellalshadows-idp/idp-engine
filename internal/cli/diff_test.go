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
	writeFileAll(t, filepath.Join(dir, "new", ".idp-rendered"), "marker")
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

// TestDiffNamesTheNestedAWSStacks: idp diff prints each AWS stack by its slash
// path under rendered/ and writes the affected ones to GITHUB_OUTPUT, an
// orphan environment included.
func TestDiffNamesTheNestedAWSStacks(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join("..", "render", "testdata", "aws-registry")
	for _, out := range []string{"new", "wet"} {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"render", "--dir", fixture, "--out", filepath.Join(dir, out), "--module-ref", "v1"}, &stdout, &stderr, noEnv); code != 0 {
			t.Fatalf("render %s: exit %d, stderr %q", out, code, stderr.String())
		}
	}
	writeFileAll(t, filepath.Join(dir, "wet", "aws", "staging", "_baseline", "main.tf.json"), "{}")
	writeFileAll(t, filepath.Join(dir, "new", "aws", "dev", "components", "api", "extra.tf.json"), "{}")
	ghOut := filepath.Join(dir, "github-output")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"diff", "--new", filepath.Join(dir, "new"), "--wet", filepath.Join(dir, "wet")}, &stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": ghOut}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	want := "aws/dev/_baseline: unchanged\n" +
		"aws/dev/components/api: changed\n" +
		"aws/pro/_baseline: unchanged\n" +
		"aws/pro/components/api: unchanged\n" +
		"aws/staging/_baseline: orphan\n" +
		"github: unchanged\n" +
		"diff: 2 affected stack(s)\n"
	if stdout.String() != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout.String(), want)
	}
	got, err := os.ReadFile(ghOut)
	if err != nil {
		t.Fatal(err)
	}
	if want := "affected=[\"aws/dev/components/api\",\"aws/staging/_baseline\"]\n"; string(got) != want {
		t.Errorf("GITHUB_OUTPUT = %q, want %q", got, want)
	}
}

func TestDiffRequiresARenderMarker(t *testing.T) {
	dir := t.TempDir()
	writeFileAll(t, filepath.Join(dir, "new", "github", "main.tf.json"), "{}")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"diff", "--new", filepath.Join(dir, "new"), "--wet", filepath.Join(dir, "wet")}, &stdout, &stderr, noEnv)
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "is not an idp render") {
		t.Errorf("stderr = %q", stderr.String())
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
