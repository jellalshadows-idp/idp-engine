package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

func validConfig() Config {
	return Config{
		Org: "acme", ClaimsRepo: "idp-claims", ApproverID: 42,
		Reader:     AppCredentials{ID: 1, ClientID: "Iv-reader", Slug: "acme-reader", PrivateKey: "reader-pem"},
		Writer:     AppCredentials{ID: 2, ClientID: "Iv-writer", Slug: "acme-writer", PrivateKey: "writer-pem"},
		Passphrase: "correct-horse-battery-staple",
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Config)
		apply     bool
		wantError string
	}{
		{name: "valid for apply", mutate: func(*Config) {}, apply: true},
		{name: "check ignores keys and passphrase", mutate: func(c *Config) { c.Passphrase = ""; c.Reader.PrivateKey = "" }, apply: false},
		{name: "every problem is reported", mutate: func(c *Config) { c.Org = ""; c.ApproverID = 0 }, apply: true, wantError: "org is required; approver id must be positive"},
		{name: "short passphrase", mutate: func(c *Config) { c.Passphrase = "short" }, apply: true, wantError: "passphrase must be at least 16 characters"},
		{name: "missing writer key", mutate: func(c *Config) { c.Writer.PrivateKey = "" }, apply: true, wantError: "writer private key is missing"},
		{name: "incomplete reader app", mutate: func(c *Config) { c.Reader.ClientID = "" }, apply: false, wantError: "reader app id, client id and slug are required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConfig()
			tt.mutate(&c)
			var err error
			if tt.apply {
				err = c.ValidateApply()
			} else {
				err = c.ValidateCheck()
			}
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.wantError)
			}
		})
	}
}

func TestLoadAppCredentials(t *testing.T) {
	dir := t.TempDir()
	json := `{"id": 77, "client_id": "Iv23abc", "slug": "acme-writer"}`
	if err := os.WriteFile(filepath.Join(dir, "acme-writer.json"), []byte(json), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "acme-writer.pem"), []byte("PEM"), 0o600); err != nil {
		t.Fatal(err)
	}

	noKey, err := LoadAppCredentials(filepath.Join(dir, "acme-writer.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	if noKey.ID != 77 || noKey.ClientID != "Iv23abc" || noKey.Slug != "acme-writer" || noKey.PrivateKey != "" {
		t.Errorf("without key = %+v", noKey)
	}
	withKey, err := LoadAppCredentials(filepath.Join(dir, "acme-writer.json"), true)
	if err != nil {
		t.Fatal(err)
	}
	if withKey.PrivateKey != "PEM" {
		t.Errorf("PrivateKey = %q, want PEM", withKey.PrivateKey)
	}
}

func TestReadPassphrase(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		want      string
		wantError bool
	}{
		{name: "crlf from windows editors is trimmed", content: "correct-horse-battery-staple\r\n", want: "correct-horse-battery-staple"},
		{name: "lf is trimmed", content: "correct-horse-battery-staple\n", want: "correct-horse-battery-staple"},
		{name: "inner spaces are kept", content: "correct horse battery staple", want: "correct horse battery staple"},
		{name: "too short after trimming", content: "fifteen-chars!!\r\n", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pass")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadPassphrase(path)
			if tt.wantError {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ReadPassphrase = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestUserID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/jellalshadows" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"login":"jellalshadows","id":4242}`))
	}))
	defer srv.Close()

	id, err := UserID(context.Background(), ghapi.New(srv.URL, "t"), "jellalshadows")
	if err != nil || id != 4242 {
		t.Fatalf("UserID = %d, %v; want 4242", id, err)
	}
	if _, err := UserID(context.Background(), ghapi.New(srv.URL, "t"), "ghost"); err == nil {
		t.Fatal("want error for unknown user")
	}
}
