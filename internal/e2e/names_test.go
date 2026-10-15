package e2e

import (
	"strings"
	"testing"
)

func TestNewNames(t *testing.T) {
	n, err := NewNames("38314547633")
	if err != nil || n.Group != "e2e-r38314547633-team" || n.Component != "e2e-r38314547633-app" {
		t.Fatalf("NewNames = %+v, %v", n, err)
	}
	if n.GroupPath() != "claims/groups/e2e-r38314547633-team.yaml" || n.ComponentPath() != "claims/components/e2e-r38314547633-app.yaml" {
		t.Errorf("paths %s, %s", n.GroupPath(), n.ComponentPath())
	}
	for _, bad := range []string{"", "ABC", "a-b", "1234567890123", "x y"} {
		if _, err := NewNames(bad); err == nil {
			t.Errorf("NewNames(%q) must fail", bad)
		}
	}
}

func TestNamesRefuseOtherPrefixes(t *testing.T) {
	err := Names{Group: "platform", Component: "e2e-r1-app"}.guard()
	if err == nil || !strings.Contains(err.Error(), `refusing to touch "platform"`) {
		t.Errorf("guard = %v", err)
	}
}
