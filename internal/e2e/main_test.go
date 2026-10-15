package e2e

import (
	"bytes"
	"strings"
	"testing"
)

func TestMainUsage(t *testing.T) {
	env := func(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }
	tokens := map[string]string{"E2E_BOT_TOKEN": "b", "E2E_APPROVER_TOKEN": "a"}
	sha := strings.Repeat("a", 40)
	tests := []struct {
		name string
		args []string
		vars map[string]string
		want string
	}{
		{"no repo", []string{"--engine-sha", sha, "--run", "1"}, tokens, "--repo OWNER/NAME, --engine-sha and --run are required"},
		{"bad sha", []string{"--repo", "acme/x", "--engine-sha", "abc", "--run", "1"}, tokens, "--engine-sha must be a 40-character commit SHA"},
		{"no tokens", []string{"--repo", "acme/x", "--engine-sha", sha, "--run", "1"}, map[string]string{}, "E2E_BOT_TOKEN and E2E_APPROVER_TOKEN must be set"},
	}
	for _, tt := range tests {
		var out, errb bytes.Buffer
		if code := Main(tt.args, &out, &errb, env(tt.vars)); code != 2 || !strings.Contains(errb.String(), tt.want) {
			t.Errorf("%s: exit %d, stderr %q, want 2 and %q", tt.name, code, errb.String(), tt.want)
		}
	}
}
