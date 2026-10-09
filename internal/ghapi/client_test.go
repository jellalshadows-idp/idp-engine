package ghapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
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
