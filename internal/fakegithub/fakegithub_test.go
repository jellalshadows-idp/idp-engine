package fakegithub

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// recorder is a testing.TB that captures failures instead of failing.
type recorder struct {
	testing.TB
	errs    []string
	cleanup []func()
}

func (r *recorder) Helper()          {}
func (r *recorder) Cleanup(f func()) { r.cleanup = append(r.cleanup, f) }
func (r *recorder) Errorf(format string, args ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

func TestWritesOutsideTheBootstrapSetAreRejected(t *testing.T) {
	tests := []struct {
		method, path string
		wantOK       bool
	}{
		{http.MethodPut, "/orgs/acme/actions/permissions/workflow", true},
		{http.MethodPut, "/repos/acme/r/actions/secrets/S", true},
		{http.MethodPut, "/repos/acme/r/environments/e/secrets/S", true},
		{http.MethodPatch, "/repos/acme/r/actions/variables/V", true},
		{http.MethodPatch, "/repos/acme/r/environments/e/variables/V", true},
		{http.MethodPatch, "/repos/acme/r/actions/secrets/S", false},
		{http.MethodPut, "/repos/acme/r/actions/variables/V", false},
		{http.MethodPut, "/anything/else", false},
		{http.MethodGet, "/repos/acme/r/rulesets/7", true},
		{http.MethodPatch, "/repos/acme/r/rulesets/7", false},
		{http.MethodDelete, "/repos/acme/r/rulesets/7", false},
		{http.MethodPut, "/repos/acme/r/environments/e/deployment-branch-policies", false},
		{http.MethodPatch, "/repos/acme/r/environments/e/deployment-branch-policies", false},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := &recorder{TB: t}
			srv := New(rec, "acme")
			defer func() {
				for _, f := range rec.cleanup {
					f()
				}
			}()
			req, err := http.NewRequest(tt.method, srv.URL+tt.path, strings.NewReader(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if gotOK := resp.StatusCode != http.StatusTeapot; gotOK != tt.wantOK {
				t.Errorf("status = %d, wantOK = %v", resp.StatusCode, tt.wantOK)
			}
			if (len(rec.errs) == 0) != tt.wantOK {
				t.Errorf("recorded errors = %v, wantOK = %v", rec.errs, tt.wantOK)
			}
		})
	}
}

func TestBadRulesetBodiesFailClearlyInsteadOfPanicking(t *testing.T) {
	tests := []struct {
		name, method, path, body, want string
	}{
		{"body-less POST", http.MethodPost, "/repos/acme/r/rulesets", "",
			"fakegithub: POST /repos/acme/r/rulesets: request body is required"},
		{"malformed PUT", http.MethodPut, "/repos/acme/r/rulesets/7", `{"name":`,
			"fakegithub: PUT /repos/acme/r/rulesets/7: invalid JSON body: unexpected EOF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{TB: t}
			srv := New(rec, "acme")
			defer func() {
				for _, f := range rec.cleanup {
					f()
				}
			}()
			req, err := http.NewRequest(tt.method, srv.URL+tt.path, strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
			if len(rec.errs) != 1 || rec.errs[0] != tt.want {
				t.Errorf("recorded errors = %q, want [%q]", rec.errs, tt.want)
			}
		})
	}
}

func TestMalformedBodyFailsClearlyInsteadOfPanicking(t *testing.T) {
	rec := &recorder{TB: t}
	srv := New(rec, "acme")
	defer func() {
		for _, f := range rec.cleanup {
			f()
		}
	}()
	resp, err := http.Post(srv.URL+"/repos/acme/r/actions/variables", "application/json", strings.NewReader(`{"value":"v"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	want := "fakegithub: POST /repos/acme/r/actions/variables: missing string field name"
	if len(rec.errs) != 1 || rec.errs[0] != want {
		t.Errorf("recorded errors = %q, want [%q]", rec.errs, want)
	}
}
