package fakegithub

import (
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
	r.errs = append(r.errs, format)
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
