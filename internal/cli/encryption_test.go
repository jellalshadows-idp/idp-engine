package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptionEnvWritesTFEncryption(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "env")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"encryption-env"}, &stdout, &stderr, envOf(map[string]string{
		"GITHUB_ENV": envFile, "IDP_STATE_PASSPHRASE": "correct-horse-battery-staple",
	}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	got, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "TF_ENCRYPTION<<IDP_EOF_") || !strings.Contains(string(got), `passphrase = "correct-horse-battery-staple"`) {
		t.Errorf("GITHUB_ENV = %q", got)
	}
	if strings.Contains(stdout.String(), "correct-horse") || strings.Contains(stderr.String(), "correct-horse") {
		t.Error("the passphrase must never be printed")
	}
}

func TestEncryptionEnvMasksTheEscapedPassphrase(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "env")
	pass := `correct"horse\battery${x}`
	var stdout, stderr bytes.Buffer
	code := Run([]string{"encryption-env"}, &stdout, &stderr, envOf(map[string]string{
		"GITHUB_ENV": envFile, "IDP_STATE_PASSPHRASE": pass, "GITHUB_ACTIONS": "true",
	}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if _, err := os.Stat(envFile); err != nil {
		t.Fatalf("GITHUB_ENV not written: %v", err)
	}
	raw := "::add-mask::" + pass
	escaped := `::add-mask::correct\"horse\\battery$${x}`
	i, j := strings.Index(stdout.String(), raw), strings.Index(stdout.String(), escaped)
	if i < 0 || j < 0 || i > j {
		t.Errorf("stdout = %q, want the raw mask line, then the escaped one", stdout.String())
	}
}

func TestEncryptionEnvMasksOnlyOnceWhenNotEscaped(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Run([]string{"encryption-env"}, &stdout, &stderr, envOf(map[string]string{
		"GITHUB_ENV": filepath.Join(t.TempDir(), "env"), "IDP_STATE_PASSPHRASE": "correct-horse-battery-staple", "GITHUB_ACTIONS": "true",
	}))
	if n := strings.Count(stdout.String(), "::add-mask::"); n != 1 {
		t.Errorf("%d mask lines, want 1: %q", n, stdout.String())
	}
}

func TestEncryptionEnvNamesTheMissingSecret(t *testing.T) {
	for name, pass := range map[string]string{"empty": "", "short": "too-short", "two lines": "correct-horse-battery\nstaple"} {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"encryption-env"}, &stdout, &stderr, envOf(map[string]string{
			"GITHUB_ENV": filepath.Join(t.TempDir(), "env"), "IDP_STATE_PASSPHRASE": pass, "GITHUB_ACTIONS": "true",
		}))
		if code != 1 || !strings.Contains(stderr.String(), "IDP_STATE_PASSPHRASE") || !strings.Contains(stdout.String(), "::error title=State passphrase::") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, code, stdout.String(), stderr.String())
		}
	}
}

func TestEncryptionEnvNeedsGitHubEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"encryption-env"}, &stdout, &stderr, envOf(map[string]string{"IDP_STATE_PASSPHRASE": "correct-horse-battery-staple"})); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}
