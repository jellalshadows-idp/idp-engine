package e2e

import (
	"bytes"
	"maps"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapp"
)

func TestMainUsage(t *testing.T) {
	env := func(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }
	with := func(k, v string) map[string]string {
		vars := map[string]string{"E2E_BOT_CLIENT_ID": "Iv23li", "E2E_BOT_PRIVATE_KEY": "pem", "E2E_APPROVER_TOKEN": "a"}
		if v == "" {
			delete(vars, k)
		} else {
			vars[k] = v
		}
		return vars
	}
	const missing = "idp-e2e: E2E_BOT_CLIENT_ID, E2E_BOT_PRIVATE_KEY and E2E_APPROVER_TOKEN must be set"
	garbage := "this is no key, and must never be printed"
	corrupt := "-----BEGIN RSA PRIVATE KEY-----\nc2VjcmV0IGtleSBtYXRlcmlhbCBuZXZlciBwcmludGVk\n-----END RSA PRIVATE KEY-----\n"
	badKey := func(pemText string) string {
		_, err := ghapp.ParseKey([]byte(pemText))
		if err == nil {
			t.Fatalf("ParseKey accepted %q", pemText)
		}
		return "idp-e2e: E2E_BOT_PRIVATE_KEY: " + err.Error()
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
		{"garbage key", args, with("E2E_BOT_PRIVATE_KEY", garbage), badKey(garbage), garbage},
		{"corrupt key", args, with("E2E_BOT_PRIVATE_KEY", corrupt), badKey(corrupt), "c2VjcmV0IGtleSBtYXRlcmlhbCBuZXZlciBwcmludGVk"},
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

func TestBotPermissions(t *testing.T) {
	want := map[string]string{
		"actions": "write", "checks": "read", "contents": "write", "issues": "read",
		"members": "read", "pull_requests": "write", "workflows": "write",
	}
	if !maps.Equal(botPermissions, want) {
		t.Errorf("botPermissions = %v, want %v: every bot token gets exactly what the GitHub forge calls (ADR-0027)", botPermissions, want)
	}
}
