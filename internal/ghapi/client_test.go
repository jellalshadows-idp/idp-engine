package ghapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPostSendsHeadersAndBodyAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tkn" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer tkn")
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
			t.Errorf("X-GitHub-Api-Version = %q", got)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/orgs/acme/repos" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		var in map[string]string
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in["name"] != "x" {
			t.Errorf("body = %v (err %v), want name=x", in, err)
		}
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id": 7}`)
	}))
	defer srv.Close()

	var out struct {
		ID int `json:"id"`
	}
	err := New(srv.URL, "tkn").Post(context.Background(), "/orgs/acme/repos", map[string]string{"name": "x"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != 7 {
		t.Errorf("ID = %d, want 7", out.ID)
	}
}

func TestEmptyTokenSendsNoAuthorization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want none", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := New(srv.URL, "").Delete(context.Background(), "/x"); err != nil {
		t.Fatal(err)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		wantNotFound bool
		wantStatus   int
	}{
		{name: "404 wraps ErrNotFound", status: http.StatusNotFound, wantNotFound: true},
		{name: "422 is an APIError", status: http.StatusUnprocessableEntity, wantStatus: http.StatusUnprocessableEntity},
		{name: "403 is an APIError", status: http.StatusForbidden, wantStatus: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				io.WriteString(w, `{"message":"nope"}`)
			}))
			defer srv.Close()

			err := New(srv.URL, "").Get(context.Background(), "/x", nil)
			if got := errors.Is(err, ErrNotFound); got != tt.wantNotFound {
				t.Errorf("errors.Is(err, ErrNotFound) = %v, want %v (err %v)", got, tt.wantNotFound, err)
			}
			if tt.wantStatus != 0 {
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.Status != tt.wantStatus {
					t.Errorf("err = %v, want *APIError with status %d", err, tt.wantStatus)
				}
			}
		})
	}
}

func TestNoContentWithOutIsFine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var out map[string]any
	if err := New(srv.URL, "t").Put(context.Background(), "/x", map[string]any{}, &out); err != nil {
		t.Fatal(err)
	}
}

func TestNewSetsATimeout(t *testing.T) {
	if got := New("http://x", "").http.Timeout; got != 30*time.Second {
		t.Errorf("http timeout = %v, want 30s", got)
	}
}

func TestOversizedBodyIsRejected(t *testing.T) {
	old := maxBodyBytes
	maxBodyBytes = 16
	defer func() { maxBodyBytes = old }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strings.Repeat("a", 64))
	}))
	defer srv.Close()

	err := New(srv.URL, "").Get(context.Background(), "/x", nil)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err = %v, want a body-too-large error", err)
	}
}

func TestNotFoundKeepsGitHubMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "  {\"message\":\"Not Found\"}\n")
	}))
	defer srv.Close()

	err := New(srv.URL, "").Get(context.Background(), "/x", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("errors.Is(err, ErrNotFound) = false (err %v)", err)
	}
	if !strings.Contains(err.Error(), `{"message":"Not Found"}`) {
		t.Errorf("err = %q, want it to include the response body", err)
	}
}

func TestNotFoundWithEmptyBodyIsPlain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, " \n")
	}))
	defer srv.Close()

	err := New(srv.URL, "").Get(context.Background(), "/x", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("errors.Is(err, ErrNotFound) = false (err %v)", err)
	}
	if got, want := err.Error(), "github GET /x: not found"; got != want {
		t.Errorf("err = %q, want %q", got, want)
	}
}

// TestBodyLimitBoundary mutates the package-level maxBodyBytes, so tests in
// this package must not use t.Parallel().
func TestBodyLimitBoundary(t *testing.T) {
	old := maxBodyBytes
	maxBodyBytes = 16
	defer func() { maxBodyBytes = old }()

	tests := []struct {
		name    string
		size    int64
		wantErr bool
	}{
		{name: "exactly the limit is accepted", size: 16},
		{name: "one byte over is rejected", size: 17, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, strings.Repeat(" ", int(tt.size)-2)+"{}")
			}))
			defer srv.Close()

			var out map[string]any
			err := New(srv.URL, "").Get(context.Background(), "/x", &out)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "exceeds") {
					t.Fatalf("err = %v, want an over-limit error", err)
				}
			} else if err != nil {
				t.Fatalf("err = %v, want success", err)
			}
		})
	}
}

func TestMalformedJSONOnSuccessIsADecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":`)
	}))
	defer srv.Close()

	var out map[string]any
	err := New(srv.URL, "").Get(context.Background(), "/x", &out)
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("err = %v, want a decode error", err)
	}
}

func TestCancelledContextFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := New(srv.URL, "").Get(ctx, "/x", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
