package floci

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// fakeAWSExit makes this test binary act as the AWS CLI: when it is set, the
// binary writes fakeAWSStderr to stderr and exits with fakeAWSExit, before any
// test runs.
const (
	fakeAWSExit   = "IDP_FLOCI_FAKE_AWS_EXIT"
	fakeAWSStderr = "IDP_FLOCI_FAKE_AWS_STDERR"
)

func TestMain(m *testing.M) {
	if code, ok := os.LookupEnv(fakeAWSExit); ok {
		fmt.Fprint(os.Stderr, os.Getenv(fakeAWSStderr))
		n, err := strconv.Atoi(code)
		if err != nil {
			n = 125
		}
		os.Exit(n)
	}
	os.Exit(m.Run())
}

// recorder keeps the errors a check reports instead of failing the test.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

// TestWantMissing runs wantMissing against a stub AWS CLI, offline: only a
// failure that names NoSuchEntity passes, so a found resource and an
// unreachable or broken floci both fail (Phase 2c, R3-001).
func TestWantMissing(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f := &floci{endpoint: "http://localhost:4566", aws: exe, root: t.TempDir()}
	for _, tt := range []struct{ name, exit, stderr, want string }{
		{"missing", "254", "An error occurred (NoSuchEntity) when calling the GetRole operation: Role not found", ""},
		{"found", "0", "", "role: found, want NoSuchEntity"},
		{"unreachable", "255", "Could not connect to the endpoint URL", "role: aws exited 255 without NoSuchEntity:\nCould not connect to the endpoint URL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(fakeAWSExit, tt.exit)
			t.Setenv(fakeAWSStderr, tt.stderr)
			r := &recorder{TB: t}
			f.wantMissing(r, targets[0], "role", "iam", "get-role", "--role-name", "api-dev-ci")
			if got := strings.Join(r.errors, "\n"); got != tt.want {
				t.Errorf("wantMissing reported %q, want %q", got, tt.want)
			}
		})
	}
}
