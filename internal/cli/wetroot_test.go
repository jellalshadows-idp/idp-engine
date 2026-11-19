package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wetRootInputs writes what apply-github has when it prepares the wet commit:
// the new render, the github stack's state and the applied-stack markers of
// apply-aws, one per stack that applied.
func wetRootInputs(t *testing.T, applied ...string) (dir string, flags []string) {
	t.Helper()
	dir = t.TempDir()
	for _, stack := range []string{"github", "aws/dev/_baseline", "aws/dev/components/api"} {
		writeFileAll(t, filepath.Join(dir, "new", filepath.FromSlash(stack), "main.tf.json"), `{"stack":"`+stack+`"}`)
		writeFileAll(t, filepath.Join(dir, "new", filepath.FromSlash(stack), ".terraform.lock.hcl"), "# lock")
	}
	writeFileAll(t, filepath.Join(dir, "work", "github.tfstate"), "encrypted state")
	for _, stack := range applied {
		writeFileAll(t, filepath.Join(dir, "applied", strings.ReplaceAll(stack, "/", ".")), stack+"\n")
	}
	if err := os.MkdirAll(filepath.Join(dir, "wet"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, []string{"--out", filepath.Join(dir, "wet-root"), "--wet-base", "0123456789abcdef0123456789abcdef01234567",
		"--new", filepath.Join(dir, "new"), "--applied-dir", filepath.Join(dir, "applied"), "--wet", filepath.Join(dir, "wet")}
}

func wetRoot(t *testing.T, args ...string) (code int, stdout, stderr, output string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "output")
	var o, e bytes.Buffer
	code = Run(append([]string{"wet-root"}, args...), &o, &e, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	b, _ := os.ReadFile(out)
	return code, o.String(), e.String(), string(b)
}

func wantFile(t *testing.T, path, want string) {
	t.Helper()
	if got, err := os.ReadFile(path); err != nil || string(got) != want {
		t.Errorf("%s = %q, %v; want %q", path, got, err, want)
	}
}

func wantNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s: %v, want it missing", path, err)
	}
}

// TestWetRootCommand: the wet commit holds the github stack's state, its
// render after a successful apply, and the render of each AWS stack that
// applied; an applied orphan's path is listed with no render, so wet-push
// deletes it (spec §6.2 step 4). A stack that did not apply keeps its old
// render in wet.
func TestWetRootCommand(t *testing.T) {
	dir, flags := wetRootInputs(t, "aws/dev/components/api", "aws/staging/components/api")
	code, stdout, stderr, output := wetRoot(t, append(flags, "--github-state", filepath.Join(dir, "work", "github.tfstate"), "--github-render")...)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	root := filepath.Join(dir, "wet-root")
	wantFile(t, filepath.Join(root, "wet-base"), "0123456789abcdef0123456789abcdef01234567\n")
	wantFile(t, filepath.Join(root, "tfstate", "github.tfstate"), "encrypted state")
	wantFile(t, filepath.Join(root, "rendered", "github", "main.tf.json"), `{"stack":"github"}`)
	wantFile(t, filepath.Join(root, "rendered", "aws", "dev", "components", "api", ".terraform.lock.hcl"), "# lock")
	wantNoFile(t, filepath.Join(root, "rendered", "aws", "dev", "_baseline"))
	wantNoFile(t, filepath.Join(root, "rendered", "aws", "staging"))
	wantFile(t, filepath.Join(root, "wet-paths"), "rendered/aws/dev/components/api\nrendered/aws/staging/components/api\nrendered/github\ntfstate/github.tfstate\n")
	want := "rendered/aws/dev/components/api: the applied render\n" +
		"rendered/aws/staging/components/api: deleted, the orphan's destroy applied\n" +
		"rendered/github: the applied render\n" +
		"tfstate/github.tfstate: the github stack's state\n" +
		"wet-root: 4 path(s) to commit\n"
	if stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout, want)
	}
	if output != "paths=4\n" {
		t.Errorf("GITHUB_OUTPUT = %q", output)
	}
}

// TestWetRootAfterAFailedGitHubApply: the state is committed always, since real
// resources may have changed; the github render is not, so the next reconcile
// retries it (spec §6.2).
func TestWetRootAfterAFailedGitHubApply(t *testing.T) {
	dir, flags := wetRootInputs(t)
	code, stdout, stderr, output := wetRoot(t, append(flags, "--github-state", filepath.Join(dir, "work", "github.tfstate"))...)
	if code != 0 || stdout != "tfstate/github.tfstate: the github stack's state\nwet-root: 1 path(s) to commit\n" || output != "paths=1\n" {
		t.Errorf("exit %d, stdout %q, stderr %q, output %q", code, stdout, stderr, output)
	}
	wantNoFile(t, filepath.Join(dir, "wet-root", "rendered"))
}

// TestWetRootWithNothingToCommit: no github stack and no AWS stack that
// applied, as when every apply-aws leg failed, leaves nothing to push.
func TestWetRootWithNothingToCommit(t *testing.T) {
	dir, flags := wetRootInputs(t)
	code, stdout, stderr, output := wetRoot(t, flags...)
	if code != 0 || stdout != "wet-root: 0 path(s) to commit\n" || output != "paths=0\n" {
		t.Errorf("exit %d, stdout %q, stderr %q, output %q", code, stdout, stderr, output)
	}
	wantFile(t, filepath.Join(dir, "wet-root", "wet-paths"), "")
}

// TestWetRootWithoutGitHubStateYet: a first reconcile that changed nothing in
// GitHub has no state to commit, and names no state path, so wet-push never
// deletes one.
func TestWetRootWithoutGitHubStateYet(t *testing.T) {
	dir, flags := wetRootInputs(t)
	code, stdout, _, _ := wetRoot(t, append(flags, "--github-state", filepath.Join(dir, "work", "missing.tfstate"), "--github-render")...)
	if code != 0 || stdout != "rendered/github: the applied render\nwet-root: 1 path(s) to commit\n" {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

// TestWetRootLeavesOutAForeignMarker: a marker idp aws-stack apply does not
// write is left out and named on stderr; it never drops the github state's
// commit, and the valid markers still count.
func TestWetRootLeavesOutAForeignMarker(t *testing.T) {
	for name, marker := range map[string][2]string{
		"github":        {"github", "github\n"},
		"misnamed":      {"aws.dev._baseline", "aws/dev/components/api\n"},
		"not a stack":   {"aws.dev.workspaces.db", "aws/dev/workspaces/db\n"},
		"no final line": {"aws.dev._baseline", "aws/dev/_baseline"},
	} {
		t.Run(name, func(t *testing.T) {
			dir, flags := wetRootInputs(t, "aws/dev/components/api")
			file := filepath.Join(dir, "applied", marker[0])
			writeFileAll(t, file, marker[1])
			code, stdout, stderr, output := wetRoot(t, append(flags, "--github-state", filepath.Join(dir, "work", "github.tfstate"))...)
			want := "idp wet-root: left out " + file + ": not an applied-stack marker: want a file named after an AWS stack, each / as ., that holds the stack and a newline\n"
			if code != 0 || stderr != want {
				t.Errorf("exit %d, stderr %q; want 0 and %q", code, stderr, want)
			}
			if stdout != "rendered/aws/dev/components/api: the applied render\ntfstate/github.tfstate: the github stack's state\nwet-root: 2 path(s) to commit\n" || output != "paths=2\n" {
				t.Errorf("stdout %q, output %q", stdout, output)
			}
			root := filepath.Join(dir, "wet-root")
			wantFile(t, filepath.Join(root, "tfstate", "github.tfstate"), "encrypted state")
			wantFile(t, filepath.Join(root, "wet-paths"), "rendered/aws/dev/components/api\ntfstate/github.tfstate\n")
			wantNoFile(t, filepath.Join(root, "rendered", "aws", "dev", "_baseline"))
		})
	}
}

// TestWetRootLeavesOutADirectoryMarker: an entry that is a directory is not a
// marker either.
func TestWetRootLeavesOutADirectoryMarker(t *testing.T) {
	dir, flags := wetRootInputs(t)
	entry := filepath.Join(dir, "applied", "aws.dev._baseline")
	if err := os.MkdirAll(entry, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr, _ := wetRoot(t, append(flags, "--github-state", filepath.Join(dir, "work", "github.tfstate"))...)
	want := "idp wet-root: left out " + entry + ": not an applied-stack marker: want a file named after an AWS stack, each / as ., that holds the stack and a newline\n"
	if code != 0 || stderr != want || stdout != "tfstate/github.tfstate: the github stack's state\nwet-root: 1 path(s) to commit\n" {
		t.Errorf("exit %d, stdout %q, stderr %q; want 0 and %q", code, stdout, stderr, want)
	}
	wantFile(t, filepath.Join(dir, "wet-root", "tfstate", "github.tfstate"), "encrypted state")
}

const keptAPI = "idp wet-root: kept rendered/aws/staging/_baseline in wet: rendered/aws/staging/components/api stays there, and a Component stack needs its baseline (spec §6.2 step 4)\n"

func wetRender(t *testing.T, dir, stack string) {
	t.Helper()
	writeFileAll(t, filepath.Join(dir, "wet", filepath.FromSlash(stack), "main.tf.json"), `{"stack":"`+stack+`"}`)
}

// TestWetRootKeepsAnOrphanBaselineWhileAComponentStays: a baseline's render
// stays in wet while a Component of its environment does (spec §6.2 step 4).
func TestWetRootKeepsAnOrphanBaselineWhileAComponentStays(t *testing.T) {
	dir, flags := wetRootInputs(t, "aws/staging/_baseline")
	wetRender(t, dir, "aws/staging/components/api")
	code, stdout, stderr, output := wetRoot(t, flags...)
	if code != 0 || stderr != keptAPI || stdout != "wet-root: 0 path(s) to commit\n" || output != "paths=0\n" {
		t.Errorf("exit %d, stdout %q, stderr %q, output %q", code, stdout, stderr, output)
	}
	wantFile(t, filepath.Join(dir, "wet-root", "wet-paths"), "")
	wantNoFile(t, filepath.Join(dir, "wet-root", "rendered"))
}

// TestWetRootKeepsTheBaselineForTheComponentsThatStay: a Component whose
// destroy applied leaves wet; those that did not apply stay, sorted.
func TestWetRootKeepsTheBaselineForTheComponentsThatStay(t *testing.T) {
	dir, flags := wetRootInputs(t, "aws/staging/_baseline", "aws/staging/components/api")
	for _, c := range []string{"api", "web", "db"} {
		wetRender(t, dir, "aws/staging/components/"+c)
	}
	code, stdout, stderr, _ := wetRoot(t, flags...)
	want := "idp wet-root: kept rendered/aws/staging/_baseline in wet: rendered/aws/staging/components/db, rendered/aws/staging/components/web stays there, and a Component stack needs its baseline (spec §6.2 step 4)\n"
	if code != 0 || stderr != want || stdout != "rendered/aws/staging/components/api: deleted, the orphan's destroy applied\nwet-root: 1 path(s) to commit\n" {
		t.Errorf("exit %d, stdout %q, stderr %q; want 0 and %q", code, stdout, stderr, want)
	}
	wantFile(t, filepath.Join(dir, "wet-root", "wet-paths"), "rendered/aws/staging/components/api\n")
}

// TestWetRootDeletesAnOrphanBaselineWhenItsComponentsLeave: the baseline goes
// with the last Component of its environment, in the same commit.
func TestWetRootDeletesAnOrphanBaselineWhenItsComponentsLeave(t *testing.T) {
	dir, flags := wetRootInputs(t, "aws/staging/_baseline", "aws/staging/components/api")
	wetRender(t, dir, "aws/staging/components/api")
	code, stdout, stderr, _ := wetRoot(t, flags...)
	want := "rendered/aws/staging/_baseline: deleted, the orphan's destroy applied\nrendered/aws/staging/components/api: deleted, the orphan's destroy applied\nwet-root: 2 path(s) to commit\n"
	if code != 0 || stderr != "" || stdout != want {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	wantFile(t, filepath.Join(dir, "wet-root", "wet-paths"), "rendered/aws/staging/_baseline\nrendered/aws/staging/components/api\n")
}

// TestWetRootDeletesAnOrphanBaselineWithNoComponentLeftInWet: only a Component
// of the same environment keeps the baseline; wet holds another environment's.
func TestWetRootDeletesAnOrphanBaselineWithNoComponentLeftInWet(t *testing.T) {
	dir, flags := wetRootInputs(t, "aws/staging/_baseline")
	wetRender(t, dir, "aws/dev/components/api")
	wetRender(t, dir, "aws/staging/_baseline")
	code, stdout, stderr, _ := wetRoot(t, flags...)
	if code != 0 || stderr != "" || stdout != "rendered/aws/staging/_baseline: deleted, the orphan's destroy applied\nwet-root: 1 path(s) to commit\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	wantFile(t, filepath.Join(dir, "wet-root", "wet-paths"), "rendered/aws/staging/_baseline\n")
}

func TestWetRootUsageErrors(t *testing.T) {
	dir, flags := wetRootInputs(t)
	if err := os.MkdirAll(filepath.Join(dir, "wet-root"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		args []string
		code int
	}{
		"no flags":                {nil, exitUsage},
		"render without state":    {append(flags[2:], "--out", filepath.Join(dir, "x"), "--github-render"), exitUsage},
		"extra argument":          {append(flags, "extra"), exitUsage},
		"no --wet":                {flags[:len(flags)-2], exitUsage},
		"an existing output root": {flags, exitError},
	} {
		code, _, stderr, _ := wetRoot(t, c.args...)
		if code != c.code || !strings.HasPrefix(stderr, "idp wet-root") {
			t.Errorf("%s: exit %d, stderr %q; want %d and an idp wet-root message", name, code, stderr, c.code)
		}
	}
}

// TestWetRootLeavesOutStateFiles: wet-push refuses any state but the github
// stack's, and that refusal would drop the github state's commit too, so a state
// file inside a render never reaches the wet root (ADR-0030). It is named on
// stderr and left out; the rest of the commit goes on.
func TestWetRootLeavesOutStateFiles(t *testing.T) {
	dir, flags := wetRootInputs(t, "aws/dev/components/api")
	api := filepath.Join(dir, "new", "aws", "dev", "components", "api")
	writeFileAll(t, filepath.Join(api, "terraform.tfstate"), "aws state")
	writeFileAll(t, filepath.Join(api, "sub", "x.TfState.backup"), "aws state")
	code, _, stderr, _ := wetRoot(t, append(flags, "--github-state", filepath.Join(dir, "work", "github.tfstate"), "--github-render")...)
	want := "idp wet-root: left out rendered/aws/dev/components/api/sub/x.TfState.backup: wet holds no state but tfstate/github.tfstate (ADR-0030)\n" +
		"idp wet-root: left out rendered/aws/dev/components/api/terraform.tfstate: wet holds no state but tfstate/github.tfstate (ADR-0030)\n"
	if code != 0 || stderr != want {
		t.Fatalf("exit %d, stderr %q; want 0 and %q", code, stderr, want)
	}
	root := filepath.Join(dir, "wet-root")
	wantFile(t, filepath.Join(root, "tfstate", "github.tfstate"), "encrypted state")
	wantFile(t, filepath.Join(root, "rendered", "aws", "dev", "components", "api", "main.tf.json"), `{"stack":"aws/dev/components/api"}`)
	wantNoFile(t, filepath.Join(root, "rendered", "aws", "dev", "components", "api", "terraform.tfstate"))
	wantNoFile(t, filepath.Join(root, "rendered", "aws", "dev", "components", "api", "sub", "x.TfState.backup"))
}

// TestWetRootLeavesOutAMissingGitHubRender: --github-render with no github
// stack in the new render never deletes rendered/github from wet: the render is
// left out and named on stderr, and the github state still commits.
func TestWetRootLeavesOutAMissingGitHubRender(t *testing.T) {
	dir, flags := wetRootInputs(t)
	if err := os.RemoveAll(filepath.Join(dir, "new", "github")); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr, _ := wetRoot(t, append(flags, "--github-state", filepath.Join(dir, "work", "github.tfstate"), "--github-render")...)
	wantStderr := "idp wet-root: left out rendered/github: the new render has no github stack, so wet keeps its render\n"
	wantStdout := "tfstate/github.tfstate: the github stack's state\nwet-root: 1 path(s) to commit\n"
	if code != 0 || stderr != wantStderr || stdout != wantStdout {
		t.Errorf("exit %d, stdout %q, stderr %q; want 0, %q and %q", code, stdout, stderr, wantStdout, wantStderr)
	}
	wantFile(t, filepath.Join(dir, "wet-root", "wet-paths"), "tfstate/github.tfstate\n")
}
