package lockfiles

import (
	"regexp"
	"testing"
)

func TestGitHubLockPinsTheProvider(t *testing.T) {
	s := string(GitHub)
	if !regexp.MustCompile(`provider "registry\.opentofu\.org/integrations/github"`).MatchString(s) {
		t.Fatal("lock file does not lock integrations/github")
	}
	if !regexp.MustCompile(`version\s*=\s*"6\.13\.0"`).MatchString(s) {
		t.Error("lock file does not pin version 6.13.0")
	}
	if n := len(regexp.MustCompile(`"h1:`).FindAllString(s, -1)); n < 4 {
		t.Errorf("lock file has %d h1 hashes; want one per platform (linux/windows/darwin amd64 + darwin arm64)", n)
	}
}
