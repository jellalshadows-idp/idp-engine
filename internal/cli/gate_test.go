package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

const (
	mergeSHA = "1111111111111111111111111111111111111111"
	prHead   = "2222222222222222222222222222222222222222"
	oldHead  = "3333333333333333333333333333333333333333"
)

func writeFingerprint(t *testing.T, f plan.Fingerprint) string {
	t.Helper()
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "fingerprint.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func planComment(t *testing.T, head string, f plan.Fingerprint) string {
	t.Helper()
	m, err := plan.EncodeMarker(head, f)
	if err != nil {
		t.Fatal(err)
	}
	return plan.CommentMarker + "\n## IDP plan\n" + m + "\n"
}

func seedMergedPR(fake *fakegithub.Server, comments ...any) {
	fake.Lists["/repos/acme/idp-claims/commits/"+mergeSHA+"/pulls"] = []any{map[string]any{
		"number": 7, "merged_at": "2026-10-10T10:00:00Z", "merge_commit_sha": mergeSHA, "head": map[string]any{"sha": prHead},
	}}
	fake.Lists["/repos/acme/idp-claims/issues/7/comments"] = comments
}

// gateRun runs idp gate against fake (nil when no API call is expected) and
// returns the exit code, the decision output line and everything printed.
func gateRun(t *testing.T, fake *fakegithub.Server, current plan.Fingerprint) (code int, decision, stdout string) {
	t.Helper()
	dir := t.TempDir()
	out, summary := filepath.Join(dir, "out"), filepath.Join(dir, "summary")
	vars := map[string]string{"GITHUB_OUTPUT": out, "GITHUB_STEP_SUMMARY": summary, "GH_TOKEN": "test-token"}
	if fake != nil {
		vars["IDP_GITHUB_API"] = fake.URL
	}
	var so, se bytes.Buffer
	code = Run([]string{"gate", "--repo", "acme/idp-claims", "--sha", mergeSHA, "--fingerprint", writeFingerprint(t, current)}, &so, &se, envOf(vars))
	got, _ := os.ReadFile(out)
	return code, strings.TrimSpace(string(got)), so.String() + se.String()
}

func TestGateAutoWhenNothingPlanned(t *testing.T) {
	code, decision, _ := gateRun(t, nil, plan.Fingerprint{"github": {}})
	if code != 0 || decision != "decision=auto" {
		t.Errorf("exit %d, %q", code, decision)
	}
}

func TestGateApprovalWhenDestructive(t *testing.T) {
	code, decision, out := gateRun(t, nil, plan.Fingerprint{"github": {{Address: "a", Action: plan.Delete}}})
	if code != 0 || decision != "decision=approval" || !strings.Contains(out, "destructive") {
		t.Errorf("exit %d, %q, %q", code, decision, out)
	}
}

func TestGateAutoWhenSubsetOfTrustedComment(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	shown := plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}, {Address: "b", Action: plan.Create}}}
	seedMergedPR(fake, map[string]any{"id": 1, "body": planComment(t, prHead, shown), "user": map[string]any{"login": "github-actions[bot]", "type": "Bot"}})
	code, decision, out := gateRun(t, fake, plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}}})
	if code != 0 || decision != "decision=auto" || !strings.Contains(out, "PR #7") {
		t.Errorf("exit %d, %q, %q", code, decision, out)
	}
}

func TestGateIgnoresUntrustedComments(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	wide := plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}}}
	seedMergedPR(fake,
		map[string]any{"id": 1, "body": planComment(t, oldHead, wide), "user": map[string]any{"login": "github-actions[bot]", "type": "Bot"}},
		map[string]any{"id": 2, "body": planComment(t, prHead, wide), "user": map[string]any{"login": "mallory", "type": "User"}},
	)
	code, decision, out := gateRun(t, fake, wide)
	if code != 0 || decision != "decision=approval" || !strings.Contains(out, "is for 3333333") {
		t.Errorf("exit %d, %q, %q: the bot comment is stale and the fresh one is not the bot's", code, decision, out)
	}
}

func TestGateApprovalWhenNoMergedPR(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	fake.Lists["/repos/acme/idp-claims/commits/"+mergeSHA+"/pulls"] = []any{}
	code, decision, out := gateRun(t, fake, plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}}})
	if code != 0 || decision != "decision=approval" || !strings.Contains(out, "no merged pull request") {
		t.Errorf("exit %d, %q, %q", code, decision, out)
	}
}

func TestGateApprovalWhenChangeMissingFromPR(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	seedMergedPR(fake, map[string]any{"id": 1, "body": planComment(t, prHead, plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}}}),
		"user": map[string]any{"login": "github-actions[bot]", "type": "Bot"}})
	code, decision, out := gateRun(t, fake, plan.Fingerprint{"github": {{Address: "a", Action: plan.Update}}})
	if code != 0 || decision != "decision=approval" || !strings.Contains(out, "not shown in PR #7") {
		t.Errorf("exit %d, %q, %q", code, decision, out)
	}
}

func TestGateUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"gate"},
		{"gate", "--repo", "acme/idp-claims", "--sha", "short", "--fingerprint", "f"},
		{"gate", "--repo", "acme", "--sha", mergeSHA, "--fingerprint", "f"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, noEnv); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
