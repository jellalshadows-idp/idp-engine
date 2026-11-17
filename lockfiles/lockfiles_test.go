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

func TestAWSLockPinsTheProvider(t *testing.T) {
	s := string(AWS)
	if !regexp.MustCompile(`provider "registry\.opentofu\.org/hashicorp/aws"`).MatchString(s) {
		t.Fatal("lock file does not lock hashicorp/aws")
	}
	if !regexp.MustCompile(`version\s*=\s*"6\.71\.0"`).MatchString(s) {
		t.Error("lock file does not pin version 6.71.0")
	}
	if n := len(regexp.MustCompile(`"h1:`).FindAllString(s, -1)); n < 4 {
		t.Errorf("lock file has %d h1 hashes; want one per platform (linux/windows/darwin amd64 + darwin arm64)", n)
	}
	if n := len(regexp.MustCompile(`(?m)^provider `).FindAllString(s, -1)); n != 1 {
		t.Errorf("lock file locks %d providers, want only hashicorp/aws: the AWS stacks use no other", n)
	}
}
