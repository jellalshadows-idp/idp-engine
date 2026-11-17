package wetdiff

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
)

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiffStatuses(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, fresh, map[string]string{
		".idp-rendered":                       "marker",
		"github/main.tf.json":                 `{"v":2}`,
		"github/.terraform.lock.hcl":          "lock",
		"aws/dev/_baseline/main.tf.json":      "{}",
		"aws/dev/components/api/main.tf.json": "{}",
	})
	write(t, wet, map[string]string{
		"github/main.tf.json":                  `{"v":1}`,
		"github/.terraform.lock.hcl":           "lock",
		"aws/dev/_baseline/main.tf.json":       "{}",
		"aws/dev/workspaces/logs/main.tf.json": "{}",
	})
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{
		Stacks: []Stack{
			{Path: "aws/dev/_baseline", Status: Unchanged},
			{Path: "aws/dev/components/api", Status: New},
			{Path: "aws/dev/workspaces/logs", Status: Orphan},
			{Path: "github", Status: Changed},
		},
		Affected: []string{"aws/dev/components/api", "aws/dev/workspaces/logs", "github"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDiffAddedNestedFileIsChanged(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, wet, map[string]string{"github/main.tf.json": "{}"})
	write(t, fresh, map[string]string{"github/main.tf.json": "{}", "github/files/api/.github/CODEOWNERS": "* @acme/platform"})
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stacks[0].Status != Changed {
		t.Errorf("status = %s, want changed", got.Stacks[0].Status)
	}
}

func TestDiffWetMissing(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "new")
	write(t, fresh, map[string]string{"github/main.tf.json": "{}"})
	got, err := Diff(fresh, filepath.Join(dir, "does-not-exist"), false)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{Stacks: []Stack{{Path: "github", Status: New}}, Affected: []string{"github"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff = %+v, want %+v", got, want)
	}
}

func TestDiffAllMarksEveryStackAffected(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, fresh, map[string]string{"github/main.tf.json": "{}"})
	write(t, wet, map[string]string{"github/main.tf.json": "{}"})
	got, err := Diff(fresh, wet, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stacks[0].Status != Unchanged || !reflect.DeepEqual(got.Affected, []string{"github"}) {
		t.Errorf("Diff = %+v, want github unchanged but affected", got)
	}
}

func TestDiffIgnoresDotTerraform(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, fresh, map[string]string{"github/main.tf.json": "{}", "github/.terraform/providers/p": "binary"})
	write(t, wet, map[string]string{"github/main.tf.json": "{}"})
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stacks[0].Status != Unchanged || len(got.Affected) != 0 {
		t.Errorf("Diff = %+v, want github unchanged and nothing affected", got)
	}
}

func TestDiffNoStacksGivesEmptyLists(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "new")
	write(t, fresh, map[string]string{".idp-rendered": "marker"})
	got, err := Diff(fresh, filepath.Join(dir, "wet"), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stacks == nil || got.Affected == nil || len(got.Stacks) != 0 || len(got.Affected) != 0 {
		t.Errorf("Diff = %#v, want empty, non-nil lists", got)
	}
}

func TestDiffRejectsFilesAsRoots(t *testing.T) {
	dir := t.TempDir()
	validNew := filepath.Join(dir, "new")
	write(t, validNew, map[string]string{"github/main.tf.json": "{}"})
	fileRoot := filepath.Join(dir, "file")
	if err := os.WriteFile(fileRoot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Diff(fileRoot, t.TempDir(), false); err == nil {
		t.Error("want an error for a regular file as newRoot")
	}
	_, err := Diff(validNew, fileRoot, false)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("wetRoot as file: err = %v, want 'not a directory'", err)
	}
}

// TestDiffNestedStackOwnsItsFiles: a file belongs to the deepest stack that
// contains it, so a change inside a stack nested in another changes only the
// nested one.
func TestDiffNestedStackOwnsItsFiles(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, wet, map[string]string{"aws/dev/main.tf.json": "{}", "aws/dev/_baseline/main.tf.json": `{"v":1}`})
	write(t, fresh, map[string]string{"aws/dev/main.tf.json": "{}", "aws/dev/_baseline/main.tf.json": `{"v":2}`})
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{
		Stacks:   []Stack{{Path: "aws/dev", Status: Unchanged}, {Path: "aws/dev/_baseline", Status: Changed}},
		Affected: []string{"aws/dev/_baseline"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff =\n%+v\nwant\n%+v", got, want)
	}
}

// TestDiffRootIsNeverAStack: a main.tf.json directly under a root is ignored,
// like the render marker; only directories below the root are stacks.
func TestDiffRootIsNeverAStack(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, wet, map[string]string{"main.tf.json": `{"v":1}`, "github/main.tf.json": "{}"})
	write(t, fresh, map[string]string{"main.tf.json": `{"v":2}`, "github/main.tf.json": "{}"})
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{Stacks: []Stack{{Path: "github", Status: Unchanged}}, Affected: []string{}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff = %+v, want %+v", got, want)
	}
}

// TestDiffRejectsSymlinks: a render holds only directories and regular files,
// so a symbolic link in either tree is an error, never followed.
func TestDiffRejectsSymlinks(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, fresh, map[string]string{"github/main.tf.json": "{}"})
	write(t, wet, map[string]string{"github/main.tf.json": "{}"})
	if err := os.Symlink(filepath.Join(fresh, "github", "main.tf.json"), filepath.Join(wet, "github", "link.json")); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
	_, err := Diff(fresh, wet, false)
	if want := "wet render: github/link.json: not a regular file or directory"; err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

// TestDiffSeesTheRenderedAWSStacks diffs two real renders (spec §5.4): the
// claims after an environment, staging, and a registry Component, web, were
// removed, against the render that still had them. Every AWS stack is found
// at its nested path, and the removed ones are orphans.
func TestDiffSeesTheRenderedAWSStacks(t *testing.T) {
	m, diags, err := claims.Load("../render/testdata/aws-registry")
	if err != nil || len(diags) > 0 {
		t.Fatalf("load: %v %v", err, diags)
	}
	current, err := render.Render(m, render.Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	old := *m
	old.Platform.Environments = map[string]claims.Environment{"staging": {AWS: &claims.AWS{AccountID: "000000000002", Region: "eu-west-1"}}}
	for name, env := range m.Platform.Environments {
		old.Platform.Environments[name] = env
	}
	api, web := m.Components[0], m.Components[0]
	api.Environments = []string{"dev", "pro", "staging"}
	web.Name, web.Environments = "web", []string{"dev"}
	old.Components = []claims.Component{api, web}
	previous, err := render.Render(&old, render.Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	if err := render.WriteTree(fresh, current); err != nil {
		t.Fatal(err)
	}
	if err := render.WriteTree(wet, previous); err != nil {
		t.Fatal(err)
	}
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{
		Stacks: []Stack{
			{Path: "aws/dev/_baseline", Status: Unchanged},
			{Path: "aws/dev/components/api", Status: Unchanged},
			{Path: "aws/dev/components/web", Status: Orphan},
			{Path: "aws/pro/_baseline", Status: Unchanged},
			{Path: "aws/pro/components/api", Status: Unchanged},
			{Path: "aws/staging/_baseline", Status: Orphan},
			{Path: "aws/staging/components/api", Status: Orphan},
			{Path: "github", Status: Changed},
		},
		Affected: []string{"aws/dev/components/web", "aws/staging/_baseline", "aws/staging/components/api", "github"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDiffNewRootMissingIsAnError(t *testing.T) {
	if _, err := Diff(filepath.Join(t.TempDir(), "nope"), t.TempDir(), false); err == nil {
		t.Error("want an error for a missing new render")
	}
}
