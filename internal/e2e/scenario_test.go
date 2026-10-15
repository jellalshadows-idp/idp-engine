package e2e

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"
	"time"
)

const testSHA = "abcdefabcdefabcdefabcdefabcdefabcdefabcd"

func testTemplates() map[string][]byte {
	t := map[string][]byte{}
	for _, n := range []string{"drift.yaml", "pr.yaml", "reconcile.yaml", "recover.yaml"} {
		t[n] = []byte("jobs:\n  x:\n    uses: jellalshadows-idp/idp-engine/.github/workflows/" + n + "@" + strings.Repeat("0", 40) + " # unreleased\n")
	}
	return t
}

// world returns a fake whose callers already pin testSHA and whose pipelines
// behave: create makes the resources, update changes them, the case-only edit
// plans nothing, delete removes everything after an approval.
func world(t *testing.T) (*fakeForge, *Scenario) {
	t.Helper()
	f := newFakeForge()
	pinned, err := PinTemplates(testTemplates(), testSHA)
	if err != nil {
		t.Fatal(err)
	}
	for n, b := range pinned {
		f.main[".github/workflows/"+n] = b
	}
	repo, team := "/repos/acme/e2e-r42-app", "/orgs/acme/teams/e2e-r42-team"
	f.effects["create"] = func(f *fakeForge) {
		f.paths[repo] = map[string]any{"visibility": "public", "description": "End-to-end harness repository", "topics": []string{"e2e", "harness"}}
		f.paths[team] = map[string]any{"privacy": "closed"}
		f.paths[team+"/memberships/owner"] = map[string]any{"role": "maintainer", "state": "active"}
		f.paths[team+"/memberships/member-one"] = map[string]any{"role": "member", "state": "active"}
		f.paths[repo+"/environments"] = map[string]any{"environments": []any{map[string]any{"name": "dev"}}}
	}
	f.effects["update"] = func(f *fakeForge) {
		f.paths[repo] = map[string]any{"visibility": "public", "description": "End-to-end harness repository, updated", "topics": []string{}}
	}
	f.comments["case"] = "No infrastructure changes."
	f.comments["pin"] = "No infrastructure changes."
	f.comments["delete"] = "<code>github</code>: +0 ~0 -8 ±0\n> [!WARNING]"
	f.approval["delete"] = true
	f.effects["delete"] = func(f *fakeForge) {
		for p := range f.paths {
			if strings.HasPrefix(p, repo) || strings.HasPrefix(p, team) {
				delete(f.paths, p)
			}
		}
	}
	s := &Scenario{
		F: f, Clock: &fakeClock{}, Log: io.Discard, Org: "acme", Run: "42", EngineSHA: testSHA,
		Templates: testTemplates(), Owner: "owner", Member: "member-one",
		Timeouts: Timeouts{Interval: time.Second, Checks: time.Minute, Reconcile: time.Minute, Drift: time.Minute},
	}
	return f, s
}

func TestScenarioPassesAgainstWorkingPipelines(t *testing.T) {
	f, s := world(t)
	if err := s.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"branch e2e/42/create", "pr create", "merge create",
		"branch e2e/42/update", "pr update", "merge update",
		"branch e2e/42/case", "pr case", "merge case",
		"drift",
		"branch e2e/42/delete", "pr delete", "merge delete", "approve 504",
		"drift",
	}
	if !slices.Equal(f.log, want) {
		t.Errorf("steps = %v\nwant    %v", f.log, want)
	}
}

func TestScenarioPinsTheCallersFirst(t *testing.T) {
	f, s := world(t)
	s.EngineSHA = strings.Repeat("b", 40)
	if err := s.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.log) < 3 || f.log[0] != "branch e2e/42/pin" || f.log[2] != "merge pin" {
		t.Errorf("steps = %v: the callers must be pinned to the commit under test first", f.log)
	}
	if got := string(f.main[".github/workflows/pr.yaml"]); !strings.Contains(got, "@"+strings.Repeat("b", 40)+" # unreleased") {
		t.Errorf("pr.yaml = %s", got)
	}
}

func TestApproveOnlyTheDeleteRun(t *testing.T) {
	f, s := world(t)
	f.approval["update"] = true
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unexpectedly waits for approval; the harness rejected it") {
		t.Fatalf("err = %v", err)
	}
	if !f.rejected[502] || f.approved[502] {
		t.Errorf("the update's reconcile must be rejected, never approved (rejected %v, approved %v)", f.rejected, f.approved)
	}
	if len(f.approved) != 1 {
		t.Errorf("approved %v: only the cleanup's delete run", f.approved)
	}
}

func TestScenarioCleansUpAfterAFailure(t *testing.T) {
	f, s := world(t)
	f.gate["update"] = "failure"
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: idp-gate on PR #2 concluded failure") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := f.paths["/repos/acme/e2e-r42-app"]; ok {
		t.Error("the cleanup must delete the claims through the platform")
	}
	if !slices.Contains(f.log, "merge delete") {
		t.Errorf("steps = %v: no delete PR", f.log)
	}
}

func TestScenarioRemovesAllTopics(t *testing.T) {
	f, s := world(t)
	f.effects["update"] = func(f *fakeForge) {
		f.paths["/repos/acme/e2e-r42-app"] = map[string]any{"visibility": "public", "description": "End-to-end harness repository, updated", "topics": []string{"e2e", "harness"}}
	}
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "topics = [e2e harness], want []") {
		t.Fatalf("err = %v", err)
	}
}

func TestScenarioRequiresACaseOnlyEditToPlanNothing(t *testing.T) {
	f, s := world(t)
	f.comments["case"] = "<code>github</code>: +1 ~0 -1 ±0"
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "a case-only login edit must plan no change") {
		t.Fatalf("err = %v", err)
	}
}

func TestScenarioRefusesAnUnrecoveredWetPush(t *testing.T) {
	f, s := world(t)
	f.issues["idp-wet-push/Wet push failed"] = 1
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "recover first") || len(f.log) != 0 {
		t.Fatalf("err = %v, steps %v", err, f.log)
	}
}

func TestScenarioRefusesLeftovers(t *testing.T) {
	f, s := world(t)
	f.paths["/repos/acme/e2e-r7-app"] = map[string]any{}
	if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "an earlier run left them behind") {
		t.Fatalf("err = %v", err)
	}
}

func TestScenarioRequiresTheDeleteToBeFlaggedDestructive(t *testing.T) {
	f, s := world(t)
	f.comments["delete"] = "<code>github</code>: +0 ~0 -8 ±0"
	if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "the delete plan must be flagged destructive") {
		t.Fatalf("err = %v", err)
	}
}

func TestScenarioRefusesLeftoverClaims(t *testing.T) {
	f, s := world(t)
	f.main["claims/components/e2e-r7-app.yaml"] = []byte("x")
	if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "an earlier run left its claims behind") {
		t.Fatalf("err = %v", err)
	}
}

func TestScenarioRefusesActiveRuns(t *testing.T) {
	f, s := world(t)
	f.active = []Run{{ID: 9, Status: "waiting"}}
	if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "still running or waiting (first: 9)") {
		t.Fatalf("err = %v", err)
	}
}

func TestScenarioRequiresThePinToPlanNothing(t *testing.T) {
	f, s := world(t)
	s.EngineSHA = strings.Repeat("b", 40)
	f.comments["pin"] = "<code>github</code>: +2 ~0 -0 ±0"
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "the pin PR must plan no infrastructure change") {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(f.log, "merge pin") {
		t.Error("the pin PR must not be merged")
	}
}

func TestDeleteRefusesForeignDestroys(t *testing.T) {
	f, s := world(t)
	f.comments["delete"] = "| ⚠️ delete | <code>module.group_e2e-r42-team.github_team.this</code> |\n| ⚠️ delete | <code>module.group_platform.github_team.this</code> |\n> [!WARNING]"
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "the delete would destroy something the harness does not own") {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(f.log, "merge delete") {
		t.Error("a delete that destroys something else must not be merged")
	}
}

func TestCleanupRemovesClaimsThatWereNeverApplied(t *testing.T) {
	f, s := world(t)
	f.effects["create"] = nil
	f.failRun["create"] = true
	f.comments["delete"] = "No infrastructure changes."
	f.approval["delete"] = false
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "create: reconcile run 501 concluded failure") || strings.Contains(err.Error(), "cleanup:") {
		t.Fatalf("err = %v: the cleanup must succeed for claims that were never applied", err)
	}
	if _, ok := f.main["claims/components/e2e-r42-app.yaml"]; ok {
		t.Error("the cleanup must remove the claim files from main")
	}
}

func TestCleanupFailureKeepsBothErrors(t *testing.T) {
	f, s := world(t)
	f.gate["update"] = "failure"
	f.gate["delete"] = "failure"
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: idp-gate on PR #2 concluded failure") || !strings.Contains(err.Error(), "cleanup: idp-gate on PR #3 concluded failure") {
		t.Fatalf("err = %v: both causes must be kept", err)
	}
}

func TestScenarioWritesOnlyItsOwnFiles(t *testing.T) {
	f, s := world(t)
	s.EngineSHA = strings.Repeat("b", 40)
	f.main["claims/groups/platform.yaml"] = []byte("keep")
	if err := s.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, p := range f.written {
		own := p == "claims/groups/e2e-r42-team.yaml" || p == "claims/components/e2e-r42-app.yaml" || strings.HasPrefix(p, ".github/workflows/")
		if !own {
			t.Errorf("the harness wrote %s", p)
		}
	}
	if string(f.main["claims/groups/platform.yaml"]) != "keep" {
		t.Error("the harness touched a claim it does not own")
	}
}

func TestScenarioValidatesItsInputs(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*Scenario)
		want   string
	}{
		{"no org", func(s *Scenario) { s.Org = "" }, "org is required"},
		{"digit-only member", func(s *Scenario) { s.Member = "1234" }, "needs a letter"},
		{"bad login", func(s *Scenario) { s.Owner = "a b" }, "must be GitHub logins"},
		{"zero interval", func(s *Scenario) { s.Timeouts.Interval = 0 }, "poll interval must be positive"},
	} {
		_, s := world(t)
		tt.mutate(s)
		if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.want)
		}
	}
}
