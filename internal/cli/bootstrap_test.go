package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestBootstrapUsageErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "no subcommand", args: []string{"bootstrap"}, wantStderr: "idp bootstrap apply"},
		{name: "unknown subcommand", args: []string{"bootstrap", "nope"}, wantStderr: `unknown subcommand "nope"`},
		{name: "apply lists every missing flag", args: []string{"bootstrap", "apply"}, wantStderr: "missing --org, --claims-repo, --approver, --reader, --writer, --passphrase-file\n"},
		{name: "check does not need a passphrase", args: []string{"bootstrap", "check"}, wantStderr: "missing --org, --claims-repo, --approver, --reader, --writer\n"},
		{name: "token is required", args: []string{"bootstrap", "check", "--org", "o", "--claims-repo", "r", "--approver", "a", "--reader", "r.json", "--writer", "w.json"}, wantStderr: "set GH_TOKEN"},
		{name: "app needs a valid role", args: []string{"bootstrap", "app", "--org", "o", "--role", "admin"}, wantStderr: "--role reader|writer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr, noEnv)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2 (stderr %q)", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestHelpListsBootstrap(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Run([]string{"help"}, &stdout, &stderr, noEnv)
	if !strings.Contains(stdout.String(), "bootstrap") {
		t.Errorf("help = %q, want it to list bootstrap", stdout.String())
	}
}
