package e2e

import (
	"strings"
	"testing"
)

func TestPinTemplates(t *testing.T) {
	sha := strings.Repeat("b", 40)
	in := map[string][]byte{"pr.yaml": []byte("    uses: jellalshadows-idp/idp-engine/.github/workflows/pr.yaml@" + strings.Repeat("0", 40) + " # unreleased\n")}
	out, err := PinTemplates(in, sha)
	if err != nil {
		t.Fatal(err)
	}
	if want := "    uses: jellalshadows-idp/idp-engine/.github/workflows/pr.yaml@" + sha + " # unreleased\n"; string(out["pr.yaml"]) != want {
		t.Errorf("pinned = %q, want %q", out["pr.yaml"], want)
	}
	if _, err := PinTemplates(in, "abc"); err == nil {
		t.Error("a short SHA must be rejected")
	}
	if _, err := PinTemplates(map[string][]byte{"x.yaml": []byte("jobs: {}\n")}, sha); err == nil {
		t.Error("a template without an engine workflow ref must be rejected")
	}
}
