package e2e

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapp"
)

func TestMainUsage(t *testing.T) {
	env := func(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }
	with := func(k, v string) map[string]string {
		vars := map[string]string{"E2E_BOT_CLIENT_ID": "Iv23li", "E2E_BOT_PRIVATE_KEY": "pem", "E2E_READER_CLIENT_ID": "Iv23lr", "E2E_READER_PRIVATE_KEY": "pem", "E2E_APPROVER_TOKEN": "a"}
		if v == "" {
			delete(vars, k)
		} else {
			vars[k] = v
		}
		return vars
	}
	const missing = "idp-e2e: E2E_BOT_CLIENT_ID, E2E_BOT_PRIVATE_KEY, E2E_READER_CLIENT_ID, E2E_READER_PRIVATE_KEY and E2E_APPROVER_TOKEN must be set"
	garbage := "this is no key, and must never be printed"
	corrupt := "-----BEGIN RSA PRIVATE KEY-----\nc2VjcmV0IGtleSBtYXRlcmlhbCBuZXZlciBwcmludGVk\n-----END RSA PRIVATE KEY-----\n"
	badKey := func(name, pemText string) string {
		_, err := ghapp.ParseKey([]byte(pemText))
		if err == nil {
			t.Fatalf("ParseKey accepted %q", pemText)
		}
		return "idp-e2e: " + name + ": " + err.Error()
	}
	good := testKeyPEM(t)
	withGoodBot := func(k, v string) map[string]string {
		vars := with(k, v)
		vars["E2E_BOT_PRIVATE_KEY"] = good
		return vars
	}
	sha := strings.Repeat("a", 40)
	args := []string{"--repo", "acme/x", "--engine-sha", sha, "--run", "1"}
	tests := []struct {
		name   string
		args   []string
		vars   map[string]string
		want   string
		secret string // stderr must never contain it
	}{
		{"no repo", []string{"--engine-sha", sha, "--run", "1"}, with("", ""), "--repo OWNER/NAME, --engine-sha and --run are required", ""},
		{"bad sha", []string{"--repo", "acme/x", "--engine-sha", "abc", "--run", "1"}, with("", ""), "--engine-sha must be a 40-character commit SHA", ""},
		{"no inputs", args, map[string]string{}, missing, ""},
		{"no client id", args, with("E2E_BOT_CLIENT_ID", ""), missing, ""},
		{"no private key", args, with("E2E_BOT_PRIVATE_KEY", ""), missing, ""},
		{"no approver token", args, with("E2E_APPROVER_TOKEN", ""), missing, ""},
		{"old bot token only", args, map[string]string{"E2E_BOT_TOKEN": "b", "E2E_APPROVER_TOKEN": "a"}, missing, ""},
		{"garbage key", args, with("E2E_BOT_PRIVATE_KEY", garbage), badKey("E2E_BOT_PRIVATE_KEY", garbage), garbage},
		{"corrupt key", args, with("E2E_BOT_PRIVATE_KEY", corrupt), badKey("E2E_BOT_PRIVATE_KEY", corrupt), "c2VjcmV0IGtleSBtYXRlcmlhbCBuZXZlciBwcmludGVk"},
		{"no reader client id", args, with("E2E_READER_CLIENT_ID", ""), missing, ""},
		{"no reader key", args, with("E2E_READER_PRIVATE_KEY", ""), missing, ""},
		{"garbage reader key", args, withGoodBot("E2E_READER_PRIVATE_KEY", garbage), badKey("E2E_READER_PRIVATE_KEY", garbage), garbage},
		{"corrupt reader key", args, withGoodBot("E2E_READER_PRIVATE_KEY", corrupt), badKey("E2E_READER_PRIVATE_KEY", corrupt), "c2VjcmV0IGtleSBtYXRlcmlhbCBuZXZlciBwcmludGVk"},
	}
	for _, tt := range tests {
		var out, errb bytes.Buffer
		if code := Main(tt.args, &out, &errb, env(tt.vars)); code != 2 || !strings.Contains(errb.String(), tt.want) {
			t.Errorf("%s: exit %d, stderr %q, want 2 and %q", tt.name, code, errb.String(), tt.want)
		}
		if tt.secret != "" && strings.Contains(errb.String()+out.String(), tt.secret) {
			t.Errorf("%s: the output repeats the private key", tt.name)
		}
	}
}

func TestReaderPermissions(t *testing.T) {
	if want := map[string]string{"environments": "read"}; !maps.Equal(readerPermissions, want) {
		t.Errorf("readerPermissions = %v, want %v: a reader token only reads the env variables of the run's repository (ADR-0027)", readerPermissions, want)
	}
}

// testKeyPEM is a fresh RSA key in PKCS#1 PEM, which ghapp.ParseKey accepts.
func testKeyPEM(t *testing.T) string {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}))
}

// TestMainRefusesABadRunAsBefore pins what a bad --run gets: exit 1 and the
// message NewNames gives, before any call to GitHub. Main derives the names
// itself once the reader's tokens are scoped to the run's repository.
func TestMainRefusesABadRunAsBefore(t *testing.T) {
	dir := t.TempDir()
	for n, b := range testTemplates() {
		if err := os.WriteFile(filepath.Join(dir, n), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	key := testKeyPEM(t)
	vars := map[string]string{
		"E2E_BOT_CLIENT_ID": "Iv23li", "E2E_BOT_PRIVATE_KEY": key,
		"E2E_READER_CLIENT_ID": "Iv23lr", "E2E_READER_PRIVATE_KEY": key,
		"E2E_APPROVER_TOKEN": "a", "IDP_GITHUB_API": "http://127.0.0.1:1",
	}
	var out, errb bytes.Buffer
	args := []string{"--repo", "acme/x", "--engine-sha", strings.Repeat("a", 40), "--run", "Bad_Run", "--templates", dir}
	code := Main(args, &out, &errb, func(k string) string { return vars[k] })
	if want := "idp-e2e: run \"Bad_Run\" must be 1 to 12 lowercase letters or digits\n"; code != 1 || errb.String() != want || out.Len() != 0 {
		t.Errorf("exit %d, stderr %q, stdout %q; want 1 and %q", code, errb.String(), out.String(), want)
	}
}

func TestBotPermissions(t *testing.T) {
	want := map[string]string{
		"actions": "write", "checks": "read", "contents": "write", "issues": "read",
		"members": "read", "pull_requests": "write", "workflows": "write",
	}
	if !maps.Equal(botPermissions, want) {
		t.Errorf("botPermissions = %v, want %v: every bot token gets exactly what the GitHub forge calls (ADR-0027)", botPermissions, want)
	}
}

func TestMainDefaultMemberIsTheOwnersSecondAccount(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"-h"}, &out, &errb, func(string) string { return "" }); code != 2 {
		t.Fatalf("exit %d, want 2 for -h", code)
	}
	if !strings.Contains(errb.String(), `(default "Adrian-Manuel")`) {
		t.Errorf("usage does not pin the default --member to Adrian-Manuel:\n%s", errb.String())
	}
}
