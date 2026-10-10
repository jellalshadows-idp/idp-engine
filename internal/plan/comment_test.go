package plan

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

const (
	testSHA = "0123456789abcdef0123456789abcdef01234567"
	testRun = "https://github.com/acme/idp-claims/actions/runs/1"
)

// withoutMarker drops the final fingerprint line, whose gzip bytes are not
// part of the reviewed text.
func withoutMarker(t *testing.T, body string) string {
	t.Helper()
	i := strings.LastIndex(body, "<!-- idp-fingerprint:v1 ")
	if i < 0 || !strings.HasSuffix(body, " -->\n") {
		t.Fatalf("comment does not end with a fingerprint marker:\n%s", body)
	}
	return body[:i]
}

func TestCommentGolden(t *testing.T) {
	body, err := Comment([]*Plan{parseFixture(t, "mixed.json")}, testSHA, testRun)
	if err != nil {
		t.Fatal(err)
	}
	got := withoutMarker(t, body)
	golden := filepath.Join("testdata", "comment-mixed.golden.md")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("comment differs from golden:\n%s", got)
	}
}

func TestCommentRoundTripsTheFingerprint(t *testing.T) {
	mixed := parseFixture(t, "mixed.json")
	body, err := Comment([]*Plan{mixed}, testSHA, testRun)
	if err != nil {
		t.Fatal(err)
	}
	sha, f, ok, err := DecodeMarker(body)
	if err != nil || !ok {
		t.Fatalf("DecodeMarker: ok=%v err=%v", ok, err)
	}
	if sha != testSHA || !reflect.DeepEqual(f, FingerprintOf(mixed)) {
		t.Errorf("decoded sha=%s fingerprint=%v", sha, f)
	}
}

func TestCommentWithoutChanges(t *testing.T) {
	body, err := Comment(nil, testSHA, "")
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- idp-plan -->\n## IDP plan\n\nNo infrastructure changes.\n\nCommit `0123456`\n"
	if got := withoutMarker(t, body); got != want {
		t.Errorf("comment = %q, want %q", got, want)
	}
}

func TestCommentTruncatesLongStacks(t *testing.T) {
	p := &Plan{Stack: "github"}
	for i := 0; i < 105; i++ {
		p.Changes = append(p.Changes, Change{fmt.Sprintf("module.component_c%03d.github_repository.this", i), Create})
	}
	body, err := Comment([]*Plan{p}, testSHA, testRun)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(body, "| create |"); got != 100 {
		t.Errorf("rows = %d, want 100", got)
	}
	if !strings.Contains(body, "| … | and 5 more changes |") {
		t.Error("missing the truncation row")
	}
}

func TestCommentOmitsListsWhenTooLarge(t *testing.T) {
	var plans []*Plan
	for s := 0; s < 30; s++ {
		p := &Plan{Stack: fmt.Sprintf("aws/dev/components/c%02d", s)}
		for i := 0; i < 100; i++ {
			p.Changes = append(p.Changes, Change{fmt.Sprintf("module.component_c%02d.aws_iam_role_policy_attachment.very_long_name_%03d", s, i), Create})
		}
		plans = append(plans, p)
	}
	body, err := Comment(plans, testSHA, testRun)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > 65536 {
		t.Errorf("comment is %d bytes, over GitHub's 65536 limit", len(body))
	}
	if !strings.Contains(body, "Change lists are omitted") {
		t.Error("missing the omission note")
	}
	if _, f, ok, err := DecodeMarker(body); err != nil || !ok || len(f) != 30 {
		t.Errorf("fingerprint must survive: ok=%v err=%v stacks=%d", ok, err, len(f))
	}
}

func TestMarkerErrors(t *testing.T) {
	if _, err := EncodeMarker("not-a-sha", Fingerprint{}); err == nil {
		t.Error("EncodeMarker accepted an invalid sha")
	}
	if _, _, ok, err := DecodeMarker("no marker here"); ok || err != nil {
		t.Errorf("DecodeMarker without marker: ok=%v err=%v", ok, err)
	}
	broken := "<!-- idp-fingerprint:v1 sha=" + testSHA + " data=bm90Z3ppcA== -->"
	if _, _, ok, err := DecodeMarker(broken); !ok || err == nil {
		t.Errorf("DecodeMarker with bad data: ok=%v err=%v, want ok and an error", ok, err)
	}
}

func TestValidSHA(t *testing.T) {
	if !ValidSHA(testSHA) || ValidSHA("0123456") || ValidSHA(strings.ToUpper(testSHA)) {
		t.Error("ValidSHA must accept exactly 40 lowercase hex characters")
	}
}
