package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

const (
	cliPlatform  = "apiVersion: idp/v1\nkind: Platform\ngithub:\n  org: acme\n  writerAppId: 5255579\nenvironments:\n  dev: {}\n"
	cliGroup     = "apiVersion: idp/v1\nkind: Group\nname: platform\nmembers:\n  - user: alice\n    role: maintainer\n"
	cliComponent = "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nenvironments: [dev]\n"
)

// writeFileAll is writeFile (bootstrap_test.go) plus the parent directories.
func writeFileAll(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, content)
}

func claimsRepo(t *testing.T, component string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range map[string]string{
		"config/platform.yaml":        cliPlatform,
		"claims/groups/platform.yaml": cliGroup,
		"claims/components/api.yaml":  component,
	} {
		writeFileAll(t, filepath.Join(dir, filepath.FromSlash(rel)), content)
	}
	return dir
}

func envOf(m map[string]string) Env { return func(k string) string { return m[k] } }

func TestValidate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"validate", "--dir", claimsRepo(t, cliComponent)}, &stdout, &stderr, noEnv); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "validate: ok (1 group(s), 1 component(s))") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestValidateReportsProblemsAndAnnotations(t *testing.T) {
	bad := strings.Replace(cliComponent, "[dev]", "[staging]", 1)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"validate", "--dir", claimsRepo(t, bad)}, &stdout, &stderr, envOf(map[string]string{"GITHUB_ACTIONS": "true"}))
	if code != 3 {
		t.Fatalf("exit %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), `claims/components/api.yaml:5: environment "staging" is not defined in config/platform.yaml`) || !strings.Contains(stderr.String(), "validate: 1 problem(s)") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "::error file=claims/components/api.yaml,line=5::") {
		t.Errorf("stdout = %q, want a GitHub annotation", stdout.String())
	}
}

func TestValidateWithoutActionsPrintsNoAnnotations(t *testing.T) {
	bad := strings.Replace(cliComponent, "[dev]", "[staging]", 1)
	var stdout, stderr bytes.Buffer
	Run([]string{"validate", "--dir", claimsRepo(t, bad)}, &stdout, &stderr, noEnv)
	if strings.Contains(stdout.String(), "::error") {
		t.Errorf("annotations outside GitHub Actions: %q", stdout.String())
	}
}

func TestRenderWritesTheStack(t *testing.T) {
	out := filepath.Join(t.TempDir(), "rendered")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", "--dir", claimsRepo(t, cliComponent), "--out", out, "--module-ref", "v9.9.9"}, &stdout, &stderr, noEnv)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(out, "github", "main.tf.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "?ref=v9.9.9") {
		t.Errorf("module sources do not pin the ref:\n%s", data)
	}
	if !strings.Contains(stdout.String(), "render: wrote ") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRenderDevBuildNeedsASource(t *testing.T) {
	if version.Version != "dev" {
		t.Skip("only meaningful for development builds")
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", "--dir", claimsRepo(t, cliComponent), "--out", filepath.Join(t.TempDir(), "r")}, &stdout, &stderr, noEnv)
	if code != 2 || !strings.Contains(stderr.String(), "pass --module-ref or --modules-dir") {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestRenderLocalModulesAreRelative(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")
	modules := filepath.Join(root, "engine", "modules")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", "--dir", claimsRepo(t, cliComponent), "--out", out, "--modules-dir", modules}, &stdout, &stderr, noEnv)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(out, "github", "main.tf.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"source": "../../engine/modules/github/group"`) {
		t.Errorf("local module source not relative to the stack dir:\n%s", data)
	}
}

func TestRenderRefusesForeignOutDir(t *testing.T) {
	out := t.TempDir()
	writeFile(t, filepath.Join(out, "precious.txt"), "keep me")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", "--dir", claimsRepo(t, cliComponent), "--out", out, "--module-ref", "v1"}, &stdout, &stderr, noEnv)
	if code != 1 || !strings.Contains(stderr.String(), "refusing to replace it") {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(out, "precious.txt")); err != nil {
		t.Errorf("render deleted data: %v", err)
	}
}

func TestRenderStopsOnDiagnostics(t *testing.T) {
	out := filepath.Join(t.TempDir(), "rendered")
	bad := strings.Replace(cliComponent, "[dev]", "[staging]", 1)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"render", "--dir", claimsRepo(t, bad), "--out", out, "--module-ref", "v1"}, &stdout, &stderr, noEnv); code != 3 {
		t.Fatalf("exit %d, want 3", code)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("render wrote output despite diagnostics")
	}
}

func TestValidateAnnotationsAreRelativeToTheWorkspace(t *testing.T) {
	workspace := t.TempDir()
	dir := filepath.Join(workspace, "claims-repo")
	bad := strings.Replace(cliComponent, "[dev]", "[staging]", 1)
	for rel, content := range map[string]string{
		"config/platform.yaml":        cliPlatform,
		"claims/groups/platform.yaml": cliGroup,
		"claims/components/api.yaml":  bad,
	} {
		writeFileAll(t, filepath.Join(dir, filepath.FromSlash(rel)), content)
	}
	var stdout, stderr bytes.Buffer
	env := envOf(map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_WORKSPACE": workspace})
	if code := Run([]string{"validate", "--dir", dir}, &stdout, &stderr, env); code != 3 {
		t.Fatalf("exit %d, want 3", code)
	}
	if !strings.Contains(stdout.String(), "::error file=claims-repo/claims/components/api.yaml,line=5::") {
		t.Errorf("stdout = %q, want an annotation relative to GITHUB_WORKSPACE", stdout.String())
	}
	if !strings.Contains(stderr.String(), "claims/components/api.yaml:5:") {
		t.Errorf("stderr = %q, want the path relative to --dir", stderr.String())
	}
}

func TestAnnotationFile(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name, workspace, dir, want string
	}{
		{"no workspace", "", filepath.Join(root, "x"), "claims/a.yaml"},
		{"dir is the workspace", root, root, "claims/a.yaml"},
		{"dir below the workspace", root, filepath.Join(root, "sub", "repo"), "sub/repo/claims/a.yaml"},
		{"dir outside the workspace", filepath.Join(root, "ws"), filepath.Join(root, "other"), "claims/a.yaml"},
	}
	for _, tt := range tests {
		if got := annotationFile(tt.workspace, tt.dir, "claims/a.yaml"); got != tt.want {
			t.Errorf("%s: annotationFile = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestClaimsUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"validate", "extra"}, {"render", "extra"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, noEnv); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
