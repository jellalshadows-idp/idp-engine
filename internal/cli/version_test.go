package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

func TestVersionPrintsTheBuildVersion(t *testing.T) {
	old := version.Version
	version.Version = "v9.8.7"
	t.Cleanup(func() { version.Version = old })
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"version"}, &stdout, &stderr, noEnv); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if stdout.String() != "v9.8.7\n" || stderr.Len() != 0 {
		t.Errorf("stdout %q, stderr %q; want exactly the version and a newline: setup-idp compares it with the release tag", stdout.String(), stderr.String())
	}
}

func TestVersionRejectsArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"version", "extra"}, &stdout, &stderr, noEnv); code != 2 {
		t.Errorf("exit %d, want 2 (usage error, ADR-0016)", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), `idp version: unexpected argument "extra"`) {
		t.Errorf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestUsageListsVersion(t *testing.T) {
	if !strings.Contains(usage, "\n  version         Print this build's version (a release tag, a commit or dev)\n") {
		t.Error("idp help must list the version command")
	}
}
