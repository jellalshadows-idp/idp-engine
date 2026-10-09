package render

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func sampleFiles() map[string][]byte {
	return map[string][]byte{"github/main.tf.json": []byte("{}\n"), "github/.terraform.lock.hcl": []byte("# lock\n")}
}

func TestWriteTreeCreatesTheTree(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rendered")
	if err := WriteTree(dir, sampleFiles()); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"github/main.tf.json", "github/.terraform.lock.hcl", Marker} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestWriteTreeReplacesAPreviousRender(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rendered")
	if err := WriteTree(dir, map[string][]byte{"aws/dev/stale.tf.json": []byte("{}")}); err != nil {
		t.Fatal(err)
	}
	if err := WriteTree(dir, sampleFiles()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "aws", "dev", "stale.tf.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stale file survived a re-render: %v", err)
	}
}

func TestWriteTreeRefusesUnmarkedDirectory(t *testing.T) {
	dir := t.TempDir()
	precious := filepath.Join(dir, "precious.txt")
	if err := os.WriteFile(precious, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteTree(dir, sampleFiles()); err == nil {
		t.Fatal("want a refusal for a non-empty directory without the render marker")
	}
	if _, err := os.Stat(precious); err != nil {
		t.Errorf("a refused write deleted data: %v", err)
	}
}

func TestWriteTreeAcceptsAnEmptyDirectory(t *testing.T) {
	if err := WriteTree(t.TempDir(), sampleFiles()); err != nil {
		t.Fatal(err)
	}
}
