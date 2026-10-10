package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestErrorAnnotation(t *testing.T) {
	tests := []struct {
		file    string
		line    int
		title   string
		message string
		want    string
	}{
		{"claims/a,b.yaml", 4, "", "100% wrong:\nsecond line", "::error file=claims/a%2Cb.yaml,line=4::100%25 wrong:%0Asecond line"},
		{"config/platform.yaml", 0, "", "file not found", "::error file=config/platform.yaml::file not found"},
		{"", 0, "State passphrase", "IDP_STATE_PASSPHRASE is empty", "::error title=State passphrase::IDP_STATE_PASSPHRASE is empty"},
		{"", 0, "", "bare", "::error::bare"},
	}
	for _, tt := range tests {
		if got := ErrorAnnotation(tt.file, tt.line, tt.title, tt.message); got != tt.want {
			t.Errorf("ErrorAnnotation(%q, %d, %q, %q) = %q, want %q", tt.file, tt.line, tt.title, tt.message, got, tt.want)
		}
	}
}

func TestMask(t *testing.T) {
	if got, want := Mask("a%b\nc"), "::add-mask::a%25b%0Ac"; got != want {
		t.Errorf("Mask = %q, want %q", got, want)
	}
}

func TestSetOutputSingleLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out")
	if err := SetOutput(path, "decision", "auto"); err != nil {
		t.Fatal(err)
	}
	if err := SetOutput(path, "affected", `["github"]`); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "decision=auto\naffected=[\"github\"]\n" {
		t.Errorf("file = %q", got)
	}
}

func TestSetEnvMultiLineUsesARandomDelimiter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env")
	value := "line one\nline two"
	if err := SetEnv(path, "TF_ENCRYPTION", value); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	head, rest, ok := strings.Cut(got, "\n")
	if !ok || !strings.HasPrefix(head, "TF_ENCRYPTION<<IDP_EOF_") {
		t.Fatalf("file = %q, want a heredoc entry", got)
	}
	delim := strings.TrimPrefix(head, "TF_ENCRYPTION<<")
	if rest != value+"\n"+delim+"\n" {
		t.Errorf("file = %q, want the value followed by the delimiter %q", got, delim)
	}
}

func TestEmptyPathIsANoop(t *testing.T) {
	if err := SetOutput("", "a", "b"); err != nil {
		t.Error(err)
	}
	if err := AddSummary("", "x"); err != nil {
		t.Error(err)
	}
}

func TestInvalidNameIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out")
	for _, name := range []string{"", "a=b", "a\nb"} {
		if err := SetOutput(path, name, "v"); err == nil {
			t.Errorf("SetOutput(%q) succeeded, want an error", name)
		}
	}
}

func TestAddSummaryAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary")
	if err := AddSummary(path, "**Gate:** auto"); err != nil {
		t.Fatal(err)
	}
	if err := AddSummary(path, "done"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "**Gate:** auto\ndone\n" {
		t.Errorf("file = %q", got)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
