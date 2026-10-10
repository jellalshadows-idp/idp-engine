package cli

import (
	"bytes"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "no args prints usage to stderr", args: nil, wantCode: 2, wantStderr: "Usage:"},
		{name: "help prints usage to stdout", args: []string{"help"}, wantCode: 0, wantStdout: "Usage:"},
		{name: "unknown command fails", args: []string{"nope"}, wantCode: 2, wantStderr: `unknown command "nope"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr, noEnv)
			if code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestUsageColumnsAlign(t *testing.T) {
	section := strings.SplitN(strings.SplitN(usage, "Commands:\n", 2)[1], "\n\n", 2)[0]
	for _, line := range strings.Split(section, "\n") {
		if len(line) < 19 || line[:2] != "  " || line[18] == ' ' || line[17] != ' ' {
			t.Errorf("usage line %q: the description must start at column 19 (2-space indent, 16-character command column)", line)
		}
	}
}
