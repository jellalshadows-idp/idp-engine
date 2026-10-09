package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

func TestApplyValidatesLocalInputsBeforeCallingGitHub(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(`{"id": 1}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	for _, role := range []string{"reader", "writer"} {
		os.WriteFile(filepath.Join(dir, role+".json"), []byte(`{"id":1,"client_id":"Iv","slug":"`+role+`"}`), 0o600)
		os.WriteFile(filepath.Join(dir, role+".pem"), []byte("PEM"), 0o600)
	}
	pass := filepath.Join(dir, "pass")
	os.WriteFile(pass, []byte("short"), 0o600)
	env := func(k string) string {
		switch k {
		case "GH_TOKEN":
			return "t"
		case "IDP_GITHUB_API":
			return srv.URL
		}
		return ""
	}
	base := []string{"bootstrap", "apply", "--org", "o", "--claims-repo", "r", "--approver", "a",
		"--reader", filepath.Join(dir, "reader.json"), "--writer", filepath.Join(dir, "writer.json")}

	for name, args := range map[string][]string{
		"bad passphrase":      append(base[:len(base):len(base)], "--passphrase-file", pass),
		"missing credentials": {"bootstrap", "check", "--org", "o", "--claims-repo", "r", "--approver", "a", "--reader", filepath.Join(dir, "nope.json"), "--writer", filepath.Join(dir, "writer.json")},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, env); code != 1 {
			t.Errorf("%s: exit code = %d, want 1 (stderr %q)", name, code, stderr.String())
		}
		if n := calls.Load(); n != 0 {
			t.Errorf("%s: made %d GitHub call(s) before local validation", name, n)
		}
	}
}
