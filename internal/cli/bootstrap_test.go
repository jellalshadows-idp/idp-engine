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

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
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
		{name: "app needs an org", args: []string{"bootstrap", "app", "--role", "reader"}, wantStderr: "idp bootstrap app: --org is required\n"},
		{name: "app needs a valid role", args: []string{"bootstrap", "app", "--org", "o", "--role", "admin"}, wantStderr: "idp bootstrap app: --role must be reader or writer\n"},
		{name: "app listen 0.0.0.0:8080 is rejected", args: []string{"bootstrap", "app", "--org", "o", "--role", "reader", "--listen", "0.0.0.0:8080"}, wantStderr: "idp bootstrap app: --listen must be a loopback address (e.g. 127.0.0.1:0)\n"},
		{name: "app listen :8080 is rejected", args: []string{"bootstrap", "app", "--org", "o", "--role", "reader", "--listen", ":8080"}, wantStderr: "idp bootstrap app: --listen must be a loopback address (e.g. 127.0.0.1:0)\n"},
		{name: "app listen not-an-address is rejected", args: []string{"bootstrap", "app", "--org", "o", "--role", "reader", "--listen", "not-an-address"}, wantStderr: "idp bootstrap app: --listen must be a loopback address (e.g. 127.0.0.1:0)\n"},
		{name: "app listen 192.168.1.5:80 is rejected", args: []string{"bootstrap", "app", "--org", "o", "--role", "reader", "--listen", "192.168.1.5:80"}, wantStderr: "idp bootstrap app: --listen must be a loopback address (e.g. 127.0.0.1:0)\n"},
		{name: "app listen 127.0.0.1 is rejected", args: []string{"bootstrap", "app", "--org", "o", "--role", "reader", "--listen", "127.0.0.1"}, wantStderr: "idp bootstrap app: --listen must be a loopback address (e.g. 127.0.0.1:0)\n"},
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
		writeFile(t, filepath.Join(dir, role+".json"), `{"id":1,"client_id":"Iv","slug":"`+role+`"}`)
		writeFile(t, filepath.Join(dir, role+".pem"), "PEM")
	}
	pass := filepath.Join(dir, "pass")
	writeFile(t, pass, "short")
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// e2eFixture is a fake GitHub for org "acme" plus the credential and passphrase
// files the apply and check subcommands read.
type e2eFixture struct {
	fake *fakegithub.Server
	args []string // flags shared by apply and check
	pass string
}

func newE2E(t *testing.T) *e2eFixture {
	t.Helper()
	fake := fakegithub.New(t, "acme")
	fake.Objects["/users/jellalshadows"] = map[string]any{"login": "jellalshadows", "id": 42}
	dir := t.TempDir()
	for _, role := range []string{"reader", "writer"} {
		writeFile(t, filepath.Join(dir, role+".json"), `{"id":1,"client_id":"Iv-`+role+`","slug":"acme-`+role+`"}`)
		writeFile(t, filepath.Join(dir, "acme-"+role+".pem"), "PEM")
	}
	pass := filepath.Join(dir, "pass")
	writeFile(t, pass, "correct-horse-battery-staple\n")
	return &e2eFixture{
		fake: fake, pass: pass,
		args: []string{"--org", "acme", "--claims-repo", "idp-claims", "--approver", "jellalshadows",
			"--reader", filepath.Join(dir, "reader.json"), "--writer", filepath.Join(dir, "writer.json")},
	}
}

func (f *e2eFixture) env(extra map[string]string) Env {
	vars := map[string]string{"GH_TOKEN": "test-token", "IDP_GITHUB_API": f.fake.URL}
	for k, v := range extra {
		vars[k] = v
	}
	return func(k string) string { return vars[k] }
}

func (f *e2eFixture) run(env Env, sub string, extra ...string) (code int, stdout, stderr string) {
	args := append([]string{"bootstrap", sub}, f.args...)
	args = append(args, extra...)
	var out, errb bytes.Buffer
	code = Run(args, &out, &errb, env)
	return code, out.String(), errb.String()
}

func TestApplyThenCheckEndToEnd(t *testing.T) {
	f := newE2E(t)

	code, stdout, stderr := f.run(f.env(nil), "apply", "--passphrase-file", f.pass)
	if code != 0 {
		t.Fatalf("apply exit code = %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.HasSuffix(strings.TrimRight(stdout, "\n"), "bootstrap apply: done") {
		t.Errorf("apply stdout = %q, want it to end with %q", stdout, "bootstrap apply: done")
	}

	f.fake.Objects["/orgs/acme/installations"] = map[string]any{"total_count": 2, "installations": []any{
		map[string]any{"app_slug": "acme-reader"}, map[string]any{"app_slug": "acme-writer"},
	}}
	code, stdout, stderr = f.run(f.env(nil), "check")
	if code != 0 {
		t.Fatalf("check exit code = %d, want 0 (stdout %q, stderr %q)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "bootstrap check: no drift") {
		t.Errorf("check stdout = %q, want %q", stdout, "bootstrap check: no drift")
	}
}

func TestCheckExitsOneOnFindings(t *testing.T) {
	f := newE2E(t)

	code, stdout, stderr := f.run(f.env(nil), "check")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr %q)", code, stderr)
	}
	for _, want := range []string{"repo acme/idp-claims: missing", "finding(s)"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want it to contain %q", stdout, want)
		}
	}
}

func TestTokenFallsBackToGithubToken(t *testing.T) {
	f := newE2E(t)

	f.run(f.env(map[string]string{"GH_TOKEN": "", "GITHUB_TOKEN": "fallback-token"}), "check")

	if want := "Bearer fallback-token"; f.fake.LastAuthorization != want {
		t.Errorf("Authorization = %q, want %q", f.fake.LastAuthorization, want)
	}
}
