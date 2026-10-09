package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/jellalshadows-idp/idp-engine/bootstrap/apps"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// AppRole selects the manifest and the App name suffix.
type AppRole string

const (
	RoleReader AppRole = "reader"
	RoleWriter AppRole = "writer"
)

// maxAppName is GitHub's limit for App names, which must also be unique across GitHub.
const maxAppName = 34

// Manifest returns the GitHub App manifest for org and role, named <org>-<role>.
func Manifest(org string, role AppRole, callback string) (map[string]any, error) {
	if role != RoleReader && role != RoleWriter {
		return nil, fmt.Errorf("unknown app role %q (want reader or writer)", role)
	}
	name := org + "-" + string(role)
	if len(name) > maxAppName {
		return nil, fmt.Errorf("app name %q is %d characters; GitHub allows at most %d", name, len(name), maxAppName)
	}
	raw, err := apps.FS.ReadFile(string(role) + ".json")
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	m["name"] = name
	m["url"] = "https://github.com/" + org
	m["redirect_url"] = callback
	return m, nil
}

// AppFlow runs GitHub's manifest flow through a local callback server: the
// person confirms creation in the browser, GitHub redirects back with a code,
// and the code is exchanged for the App's id, client id and private key.
type AppFlow struct {
	Org       string
	Role      AppRole
	API       *ghapi.Client // unauthenticated is fine for the conversion call
	OutDir    string
	Log       io.Writer
	GitHubWeb string // https://github.com; overridable in tests
}

var formTmpl = template.Must(template.New("form").Parse(`<!doctype html>
<html><body>
<form id="f" method="post" action="{{.Action}}">
<input type="hidden" name="manifest" value="{{.Manifest}}">
<noscript><button type="submit">Create GitHub App</button></noscript>
</form>
<script>document.getElementById("f").submit()</script>
</body></html>`))

type flowResult struct {
	creds AppCredentials
	err   error
}

// Run serves the form on ln and blocks until GitHub calls back or ctx ends.
func (f *AppFlow) Run(ctx context.Context, ln net.Listener) (AppCredentials, error) {
	state, err := randomState()
	if err != nil {
		return AppCredentials{}, err
	}
	manifest, err := Manifest(f.Org, f.Role, "http://"+ln.Addr().String()+"/callback")
	if err != nil {
		return AppCredentials{}, err
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return AppCredentials{}, err
	}
	action := fmt.Sprintf("%s/organizations/%s/settings/apps/new?state=%s", f.GitHubWeb, url.PathEscape(f.Org), url.QueryEscape(state))

	result := make(chan flowResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_ = formTmpl.Execute(w, map[string]string{"Action": action, "Manifest": string(manifestJSON)})
	})
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		creds, err := f.convert(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		} else {
			fmt.Fprintf(w, "App %s created. You can close this tab.\n", creds.Slug)
		}
		select {
		case result <- flowResult{creds: creds, err: err}:
		default: // a duplicate callback (browser refresh) is ignored
		}
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()
	fmt.Fprintf(f.Log, "Open http://%s/ in your browser to create %s-%s\n", ln.Addr(), f.Org, f.Role)

	select {
	case <-ctx.Done():
		return AppCredentials{}, ctx.Err()
	case res := <-result:
		if res.err != nil {
			return AppCredentials{}, res.err
		}
		return res.creds, f.save(res.creds)
	}
}

func (f *AppFlow) convert(ctx context.Context, code string) (AppCredentials, error) {
	if code == "" {
		return AppCredentials{}, errors.New("callback without code")
	}
	var resp struct {
		ID       int64  `json:"id"`
		ClientID string `json:"client_id"`
		Slug     string `json:"slug"`
		PEM      string `json:"pem"`
	}
	if err := f.API.Post(ctx, "/app-manifests/"+url.PathEscape(code)+"/conversions", nil, &resp); err != nil {
		return AppCredentials{}, err
	}
	return AppCredentials{ID: resp.ID, ClientID: resp.ClientID, Slug: resp.Slug, PrivateKey: resp.PEM}, nil
}

// save writes <slug>.json (no key) and <slug>.pem, both readable only by the owner.
func (f *AppFlow) save(c AppCredentials) error {
	if err := os.MkdirAll(f.OutDir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.OutDir, c.Slug+".pem"), []byte(c.PrivateKey), 0o600); err != nil {
		return err
	}
	meta, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(f.OutDir, c.Slug+".json"), append(meta, '\n'), 0o600)
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
