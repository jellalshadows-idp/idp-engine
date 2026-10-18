package e2e

import (
	"strings"
	"testing"
)

func TestPinTemplates(t *testing.T) {
	sha := strings.Repeat("b", 40)
	ref := "    uses: jellalshadows-idp/idp-engine/.github/workflows/pr.yaml@"
	want := ref + sha + " # unreleased\n"
	for name, in := range map[string]string{
		"a release pin":     ref + strings.Repeat("0", 40) + " # v0.1.0\n",
		"an unreleased pin": ref + strings.Repeat("0", 40) + " # unreleased\n",
		"no comment":        ref + strings.Repeat("0", 40) + "\n",
	} {
		out, err := PinTemplates(map[string][]byte{"pr.yaml": []byte(in)}, sha)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := string(out["pr.yaml"]); got != want {
			t.Errorf("%s: pinned = %q, want %q: the commit under test is usually not a release", name, got, want)
		}
	}
	in := map[string][]byte{"pr.yaml": []byte(ref + strings.Repeat("0", 40) + " # v0.1.0\n")}
	if _, err := PinTemplates(in, "abc"); err == nil {
		t.Error("a short SHA must be rejected")
	}
	if _, err := PinTemplates(map[string][]byte{"x.yaml": []byte("jobs: {}\n")}, sha); err == nil {
		t.Error("a template without an engine workflow ref must be rejected")
	}
}
