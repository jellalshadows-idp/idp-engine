package bootstrap

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

func TestManifestWriter(t *testing.T) {
	m, err := Manifest("acme", RoleWriter, "http://127.0.0.1:1234/callback")
	if err != nil {
		t.Fatal(err)
	}
	if m["name"] != "acme-writer" || m["url"] != "https://github.com/acme" || m["redirect_url"] != "http://127.0.0.1:1234/callback" {
		t.Errorf("manifest identity = %v / %v / %v", m["name"], m["url"], m["redirect_url"])
	}
	if m["public"] != false {
		t.Errorf("public = %v, want false", m["public"])
	}
	perms := m["default_permissions"].(map[string]any)
	if perms["workflows"] != "write" {
		t.Errorf("workflows = %v; features write .github/workflows, so the writer needs write", perms["workflows"])
	}
}

func TestManifestReaderIsReadOnly(t *testing.T) {
	m, err := Manifest("acme", RoleReader, "cb")
	if err != nil {
		t.Fatal(err)
	}
	perms := m["default_permissions"].(map[string]any)
	for k, v := range perms {
		if v != "read" {
			t.Errorf("reader permission %s = %v, want read", k, v)
		}
	}
	// check reads environments, their branch policies, secrets and variables.
	for _, need := range []string{"actions", "actions_variables", "environments", "secrets", "organization_administration", "metadata"} {
		if _, ok := perms[need]; !ok {
			t.Errorf("reader manifest lacks permission %q that check needs", need)
		}
	}
}

func TestManifestRejects(t *testing.T) {
	if _, err := Manifest("a-very-long-organization-name", RoleWriter, "cb"); err == nil || !strings.Contains(err.Error(), "34") {
		t.Errorf("long name: err = %v, want the 34-character limit", err)
	}
	if _, err := Manifest("acme", AppRole("admin"), "cb"); err == nil {
		t.Error("unknown role: want error")
	}
}

func httpGet(t *testing.T, url string, wantStatus int) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s = %d, want %d: %s", url, resp.StatusCode, wantStatus, body)
	}
	return string(body)
}

func startFlow(t *testing.T, apiURL, outDir string) (base string, done chan error, creds *AppCredentials, cancel context.CancelFunc) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	flow := &AppFlow{Org: "acme", Role: RoleWriter, API: ghapi.New(apiURL, ""), OutDir: outDir, Log: io.Discard, GitHubWeb: "https://github.example"}
	done = make(chan error, 1)
	creds = &AppCredentials{}
	go func() {
		c, err := flow.Run(ctx, ln)
		*creds = c
		done <- err
	}()
	return "http://" + ln.Addr().String(), done, creds, cancel
}

func TestAppFlowCreatesAndSavesApp(t *testing.T) {
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/app-manifests/the-code/conversions" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, `{"id": 77, "client_id": "Iv23abc", "slug": "acme-writer", "pem": "-----BEGIN RSA PRIVATE KEY-----\nx\n-----END RSA PRIVATE KEY-----\n"}`)
	}))
	defer gh.Close()
	out := t.TempDir()
	base, done, creds, cancel := startFlow(t, gh.URL, out)
	defer cancel()

	form := httpGet(t, base+"/", http.StatusOK)
	if !strings.Contains(form, "https://github.example/organizations/acme/settings/apps/new?state=") {
		t.Fatalf("form does not post to the org's new-app page:\n%s", form)
	}
	state := regexp.MustCompile(`state=([A-Za-z0-9_-]+)`).FindStringSubmatch(form)[1]
	httpGet(t, base+"/callback?code=the-code&state="+state, http.StatusOK)

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if creds.ID != 77 || creds.Slug != "acme-writer" || !strings.Contains(creds.PrivateKey, "BEGIN RSA") {
		t.Errorf("creds = %+v", *creds)
	}
	loaded, err := LoadAppCredentials(filepath.Join(out, "acme-writer.json"), true)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ClientID != "Iv23abc" || loaded.PrivateKey != creds.PrivateKey {
		t.Errorf("saved credentials = %+v", loaded)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(out, "acme-writer.pem"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("pem mode = %v, want 0600", info.Mode().Perm())
		}
	}
}

func TestAppFlowRejectsWrongState(t *testing.T) {
	base, done, _, cancel := startFlow(t, "http://127.0.0.1:1", t.TempDir())
	httpGet(t, base+"/callback?code=x&state=forged", http.StatusBadRequest)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("want the context error after cancel, got nil")
	}
}

func TestAppFlowConvertsOnlyOnce(t *testing.T) {
	var conversions atomic.Int32
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conversions.Add(1)
		time.Sleep(300 * time.Millisecond)
		io.WriteString(w, `{"id": 77, "client_id": "Iv23abc", "slug": "acme-writer", "pem": "-----BEGIN RSA PRIVATE KEY-----\nx\n-----END RSA PRIVATE KEY-----\n"}`)
	}))
	defer gh.Close()
	base, done, creds, cancel := startFlow(t, gh.URL, t.TempDir())
	defer cancel()

	state := regexp.MustCompile(`state=([A-Za-z0-9_-]+)`).FindStringSubmatch(httpGet(t, base+"/", http.StatusOK))[1]

	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := http.Get(base + "/callback?code=the-code&state=" + state)
			if err != nil {
				t.Error(err)
				return
			}
			resp.Body.Close()
			statuses[i] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := conversions.Load(); n != 1 {
		t.Errorf("conversion calls = %d, want exactly 1", n)
	}
	got := map[int]bool{statuses[0]: true, statuses[1]: true}
	if !got[http.StatusOK] || !got[http.StatusConflict] {
		t.Errorf("callback statuses = %v, want one 200 and one 409", statuses)
	}
	if creds.Slug != "acme-writer" {
		t.Errorf("creds = %+v", *creds)
	}
}

func TestAppFlowSaveTightensExistingPemMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not enforced on Windows")
	}
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id": 77, "client_id": "Iv23abc", "slug": "acme-writer", "pem": "-----BEGIN RSA PRIVATE KEY-----\nx\n-----END RSA PRIVATE KEY-----\n"}`)
	}))
	defer gh.Close()
	out := t.TempDir()
	pem := filepath.Join(out, "acme-writer.pem")
	if err := os.WriteFile(pem, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	base, done, _, cancel := startFlow(t, gh.URL, out)
	defer cancel()

	state := regexp.MustCompile(`state=([A-Za-z0-9_-]+)`).FindStringSubmatch(httpGet(t, base+"/", http.StatusOK))[1]
	httpGet(t, base+"/callback?code=the-code&state="+state, http.StatusOK)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(pem)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("pem mode = %v, want 0600 even when the file pre-existed", info.Mode().Perm())
	}
}
