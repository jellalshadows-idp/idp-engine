package wetdiff

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

func TestDiffNewRootMissingIsAnError(t *testing.T) {
	if _, err := Diff(filepath.Join(t.TempDir(), "nope"), t.TempDir(), false); err == nil {
		t.Error("want an error for a missing new render")
	}
}
