package wetdiff

import (
	"strings"
	"testing"
)

// TestReconcilable pins the stacks this engine version plans and applies:
// github and the AWS stacks of spec §5.4 without Workspaces (Phase 3), with the
// claims schemas' name patterns.
func TestReconcilable(t *testing.T) {
	for _, s := range []string{
		"github",
		"aws/dev/_baseline",
		"aws/pro/components/api",
		"aws/staging2/components/my-api-2",
	} {
		if !Reconcilable(s) {
			t.Errorf("Reconcilable(%q) = false, want true", s)
		}
	}
	for _, s := range []string{
		"",
		"aws/dev/workspaces/db",
		"aws/dev",
		"aws/dev/components",
		"aws/dev/components/api/extra",
		"aws/Dev/_baseline",
		"aws/abcdefghijk/_baseline",
		"aws/dev/components/-api",
		"aws/dev/components/api-",
		"aws/dev/_baseline/",
		"github/x",
		"rendered/github",
	} {
		if Reconcilable(s) {
			t.Errorf("Reconcilable(%q) = true, want false", s)
		}
	}
}

// TestEscape: a stack's escaped name holds no character that an artifact name
// refuses, and distinct stacks never share one.
func TestEscape(t *testing.T) {
	for stack, want := range map[string]string{
		"github":                 "github",
		"aws/dev/_baseline":      "aws.dev._baseline",
		"aws/dev/components/api": "aws.dev.components.api",
	} {
		if got := Escape(stack); got != want {
			t.Errorf("Escape(%q) = %q, want %q", stack, got, want)
		}
	}
	seen := map[string]string{}
	for _, s := range []string{"github", "aws/dev/_baseline", "aws/dev/components/api", "aws/dev/components/baseline", "aws/devcomponents/_baseline", "aws/dev/components/dev-api"} {
		e := Escape(s)
		if strings.ContainsAny(e, "\":<>|*?\r\n\\/") {
			t.Errorf("Escape(%q) = %q holds a character artifact names refuse", s, e)
		}
		if other, ok := seen[e]; ok {
			t.Errorf("Escape(%q) = Escape(%q) = %q", s, other, e)
		}
		seen[e] = s
	}
}
