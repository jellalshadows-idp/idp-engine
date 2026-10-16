package e2e

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
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
		f.paths[repo+"/rulesets"] = []any{map[string]any{"name": "idp-default-branch", "enforcement": "active"}}
		f.paths[team+"/repos"] = []any{map[string]any{"name": "e2e-r42-app", "permissions": map[string]any{"admin": false, "maintain": true, "push": true}}}
	}
	f.effects["update"] = func(f *fakeForge) {
		f.paths[repo] = map[string]any{"visibility": "public", "description": "End-to-end harness repository, updated", "topics": []string{}}
	}
	f.comments["update"] = "<code>github</code>: +0 ~1 -1 ±0\n> [!WARNING]"
	f.fps["update"] = plan.Fingerprint{"github": {
		{Address: "module.component_e2e-r42-app.github_repository.this", Action: plan.Update},
		{Address: "module.component_e2e-r42-app.github_repository_topics.this[0]", Action: plan.Delete},
	}}
	f.approval["update"] = true
	f.comments["case"] = "No infrastructure changes."
	f.comments["pin"] = "No infrastructure changes."
	f.comments["delete"] = "<code>github</code>: +0 ~0 -8 ±0\n> [!WARNING]"
	f.fps["delete"] = plan.Fingerprint{"github": {
		{Address: "module.component_e2e-r42-app.github_repository.this", Action: plan.Delete},
		{Address: "module.group_e2e-r42-team.github_team.this", Action: plan.Delete},
	}}
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
		"branch e2e/42/create", "pr create", "merge create", "delete branch e2e/42/create",
		"branch e2e/42/update", "pr update", "merge update", "delete branch e2e/42/update", "approve 502",
		"branch e2e/42/case", "pr case", "merge case", "delete branch e2e/42/case",
		"drift",
		"branch e2e/42/delete", "pr delete", "merge delete", "delete branch e2e/42/delete", "approve 504",
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

func TestApproveOnlyTheExpectedRuns(t *testing.T) {
	f, s := world(t)
	f.approval["case"] = true
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unexpectedly waits for approval; the harness rejected it") {
		t.Fatalf("err = %v", err)
	}
	if !f.rejected[503] || f.approved[503] {
		t.Errorf("the case-only edit's reconcile must be rejected, never approved (rejected %v, approved %v)", f.rejected, f.approved)
	}
	if len(f.approved) != 2 || !f.approved[502] || !f.approved[504] {
		t.Errorf("approved %v: only the update's topics delete and the cleanup's delete run", f.approved)
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
	f.fps["delete"] = plan.Fingerprint{"github": {
		{Address: "module.group_e2e-r42-team.github_team.this", Action: plan.Delete},
		{Address: "module.group_e2e-r42-team2.github_team.this", Action: plan.Replace},
	}}
	f.comments["delete"] = "| ⚠️ delete | <code>module.group_e2e-r42-team.github_team.this</code> |\n| … | and 1 more changes |\n> [!WARNING]"
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "the delete would destroy something the harness does not own: github: replace module.group_e2e-r42-team2.github_team.this") {
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
	f.fps["delete"] = nil
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
		{"no templates", func(s *Scenario) { s.Templates = nil }, "templates are required"},
		{"zero interval", func(s *Scenario) { s.Timeouts.Interval = 0 }, "poll interval must be positive"},
	} {
		_, s := world(t)
		tt.mutate(s)
		if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.want)
		}
	}
}

func TestDeleteRefusesACommentWithoutFingerprint(t *testing.T) {
	f, s := world(t)
	f.raw["delete"] = planMarker + "\n<code>github</code>: +0 ~0 -8 ±0\n> [!WARNING]\n<!-- idp-fingerprint:v1 sha=%SHA% data=!! -->"
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "delete: ") || slices.Contains(f.log, "merge delete") {
		t.Fatalf("err = %v: a delete whose fingerprint cannot be read must not be merged", err)
	}
}

func TestFlipCaseAlwaysChangesALoginWithALetter(t *testing.T) {
	for _, login := range []string{"Adrian-Manuel", "Adrian", "ADRIAN", "a1", "A1"} {
		if got := flipCase(login); got == login || !strings.EqualFold(got, login) {
			t.Errorf("flipCase(%q) = %q", login, got)
		}
	}
}

func TestPreflightIgnoresUnrelatedE2ENames(t *testing.T) {
	f, s := world(t)
	f.paths["/repos/acme/e2e-reports"] = map[string]any{}
	f.main["claims/components/e2e-reports.yaml"] = []byte("x")
	if err := s.Execute(context.Background()); err != nil {
		t.Fatalf("unrelated e2e- names must not block a run: %v", err)
	}
}

func TestDefaultTimeoutsFitTheJob(t *testing.T) {
	d := DefaultTimeouts
	if d.Interval != 15*time.Second || d.Checks != 15*time.Minute || d.Reconcile != 30*time.Minute || d.Drift != 15*time.Minute {
		t.Errorf("DefaultTimeouts = %+v", d)
	}
	// pin, create, update, case-only and delete wait for a gate and a reconcile,
	// both drift checks for a drift run, and a cleanup for a gate and a reconcile.
	if worst := 6*(d.Checks+d.Reconcile) + 2*d.Drift; worst > 300*time.Minute {
		t.Errorf("worst case %s: the job's timeout-minutes (330) must outlast every bounded wait", worst)
	}
}

func TestDeleteRefusesWhenMainMovedSinceTheLastMerge(t *testing.T) {
	f, s := world(t)
	f.effects["case"] = func(f *fakeForge) { f.head++ } // someone else pushes to main after the case merge
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "delete: main moved since the harness's last merge (now ") ||
		!strings.Contains(err.Error(), "someone changed the claims repo during the run; see docs/e2e.md") {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(f.log, "pr delete") {
		t.Errorf("steps = %v: no delete PR may be opened on a main the harness did not verify", f.log)
	}
	if f.approved[504] || len(f.approved) != 1 {
		t.Errorf("approved %v: only the update's topics delete may be approved", f.approved)
	}
}

func TestDeleteRejectsWhenMainMovedWhileItsPRWasOpen(t *testing.T) {
	f, s := world(t)
	f.parent["delete"] = fakeSHA(99) // someone else's commit landed on main before the delete merged
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "delete: main moved while PR #4 was open: nothing was approved") {
		t.Fatalf("err = %v", err)
	}
	if !f.rejected[504] || f.approved[504] {
		t.Errorf("the delete's reconcile must be rejected, never approved (rejected %v, approved %v)", f.rejected, f.approved)
	}
}

func TestCleanupRejectsWhenMainMovedWhileItsPRWasOpen(t *testing.T) {
	f, s := world(t)
	f.gate["update"] = "failure"
	f.parent["delete"] = fakeSHA(99)
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: idp-gate on PR #2 concluded failure") ||
		!strings.Contains(err.Error(), "cleanup: main moved while PR #3 was open: nothing was approved") {
		t.Fatalf("err = %v", err)
	}
	if !f.rejected[502] || len(f.approved) != 0 {
		t.Errorf("the cleanup's reconcile must be rejected, never approved (rejected %v, approved %v)", f.rejected, f.approved)
	}
}

func TestDeleteRequiresTheFingerprintOfThePRHead(t *testing.T) {
	_, s := world(t)
	names, err := NewNames(s.Run)
	if err != nil {
		t.Fatal(err)
	}
	s.names = names
	marker, err := plan.EncodeMarker(strings.Repeat("a", 40), plan.Fingerprint{})
	if err != nil {
		t.Fatal(err)
	}
	err = s.destroysOnlyOwnNames(planMarker+"\n"+marker, PR{Number: 4, HeadSHA: strings.Repeat("b", 40)})
	if err == nil || !strings.Contains(err.Error(), "the fingerprint on PR #4 was planned for aaaaaaa, not its head bbbbbbb") {
		t.Fatalf("err = %v", err)
	}
}

func TestScenarioRequiresThePlanCommentOfThePRHead(t *testing.T) {
	f, s := world(t)
	marker, err := plan.EncodeMarker(strings.Repeat("a", 40), plan.Fingerprint{})
	if err != nil {
		t.Fatal(err)
	}
	f.raw["create"] = planMarker + "\n<code>github</code>: +2 ~0 -0 ±0\nplanned for sha=%SHA%\n" + marker
	err = s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "create: PR #1 has no plan comment for its head") {
		t.Fatalf("err = %v: the fingerprint marker, not a substring, names the planned head", err)
	}
	if slices.Contains(f.log, "merge create") {
		t.Error("a PR whose plan is for another head must not be merged")
	}
}

func TestPreflightChecksTheApproverTokenFirst(t *testing.T) {
	f, s := world(t)
	f.checkErr = errors.New("the approver token cannot read the claims repo's runs")
	f.issues["idp-wet-push/Wet push failed"] = 1
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "preflight: the approver token cannot read the claims repo's runs") || len(f.log) != 0 {
		t.Fatalf("err = %v, steps %v: an unusable approver token must stop the run before anything else", err, f.log)
	}
}

func TestScenarioDeletesTheBranchesItMerged(t *testing.T) {
	f, s := world(t)
	s.EngineSHA = strings.Repeat("b", 40) // the pin PR merges too
	if err := s.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	merges := 0
	for i, step := range f.log {
		slug, ok := strings.CutPrefix(step, "merge ")
		if !ok {
			continue
		}
		merges++
		if i+1 == len(f.log) || f.log[i+1] != "delete branch e2e/42/"+slug {
			t.Errorf("steps = %v: the branch of %s must be deleted right after its merge", f.log, slug)
		}
		if _, ok := f.branches["e2e/42/"+slug]; ok {
			t.Errorf("branch e2e/42/%s is still there", slug)
		}
	}
	if merges != 5 {
		t.Errorf("steps = %v: want 5 merges", f.log)
	}
}

func TestScenarioOnlyLogsABranchItCannotDelete(t *testing.T) {
	f, s := world(t)
	var out bytes.Buffer
	s.Log = &out
	f.unbranch = errors.New("boom")
	if err := s.Execute(context.Background()); err != nil {
		t.Fatalf("a branch that cannot be deleted must not fail the run: %v", err)
	}
	if !strings.Contains(out.String(), "e2e: could not delete branch e2e/42/create: boom") {
		t.Errorf("log = %s", out.String())
	}
}

func TestScenarioClosesItsOpenPRsAfterAFailure(t *testing.T) {
	f, s := world(t)
	f.gate["update"] = "failure"
	if err := s.Execute(context.Background()); err == nil {
		t.Fatal("want the update's gate failure")
	}
	closed := slices.Index(f.log, "close 2")
	if closed < 0 || closed < slices.Index(f.log, "merge delete") {
		t.Errorf("steps = %v: the failed update PR must be closed after the cleanup", f.log)
	}
	for _, step := range f.log {
		if strings.HasPrefix(step, "close ") && step != "close 2" {
			t.Errorf("steps = %v: only PRs that were not merged are closed", f.log)
		}
	}
	if _, ok := f.branches["e2e/42/update"]; !ok || slices.Contains(f.log, "delete branch e2e/42/update") {
		t.Error("the branch of a PR that was not merged stays as evidence")
	}
}

func TestScenarioRequiresTheDefaultBranchRuleset(t *testing.T) {
	for _, rulesets := range [][]any{
		{},
		{map[string]any{"name": "idp-default-branch", "enforcement": "evaluate"}},
		{map[string]any{"name": "other", "enforcement": "active"}},
	} {
		f, s := world(t)
		create := f.effects["create"]
		f.effects["create"] = func(f *fakeForge) { create(f); f.paths["/repos/acme/e2e-r42-app/rulesets"] = rulesets }
		err := s.Execute(context.Background())
		if err == nil || !strings.Contains(err.Error(), "create: repository e2e-r42-app has no active idp-default-branch ruleset") {
			t.Errorf("rulesets %v: err = %v", rulesets, err)
		}
	}
}

func TestScenarioRequiresTheTeamToMaintainTheRepository(t *testing.T) {
	for _, repos := range [][]any{
		{},
		{map[string]any{"name": "e2e-r42-app", "permissions": map[string]any{"admin": false, "maintain": false, "push": true}}},
		{map[string]any{"name": "e2e-r42-app", "permissions": map[string]any{"admin": true, "maintain": true, "push": true}}},
		{map[string]any{"name": "other", "permissions": map[string]any{"admin": false, "maintain": true, "push": true}}},
	} {
		f, s := world(t)
		create := f.effects["create"]
		f.effects["create"] = func(f *fakeForge) { create(f); f.paths["/orgs/acme/teams/e2e-r42-team/repos"] = repos }
		err := s.Execute(context.Background())
		if err == nil || !strings.Contains(err.Error(), "create: team e2e-r42-team must maintain e2e-r42-app without admin") {
			t.Errorf("repos %v: err = %v", repos, err)
		}
	}
}

func TestUpdateApprovesExactlyTheTopicsDelete(t *testing.T) {
	f, s := world(t)
	if err := s.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.approved) != 2 || !f.approved[502] || !f.approved[504] {
		t.Errorf("approved %v: want the update's and the delete's runs", f.approved)
	}
	if got := f.notes[502]; got != "idp end-to-end harness run 42: remove the topics of e2e-r42-app" {
		t.Errorf("update approval comment = %q", got)
	}
	if got := f.notes[504]; got != "idp end-to-end harness run 42: delete e2e-r42-app and e2e-r42-team" {
		t.Errorf("delete approval comment = %q", got)
	}
}

func TestUpdateRefusesAPlanThatDestroysAnythingElse(t *testing.T) {
	f, s := world(t)
	f.fps["update"] = plan.Fingerprint{"github": {
		{Address: "module.component_e2e-r42-app.github_repository.this", Action: plan.Replace},
		{Address: "module.component_e2e-r42-app.github_repository_topics.this[0]", Action: plan.Delete},
	}}
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: the update plan must destroy exactly [github: delete module.component_e2e-r42-app.github_repository_topics.this[0]], got [github: delete module.component_e2e-r42-app.github_repository_topics.this[0]; github: replace module.component_e2e-r42-app.github_repository.this]") {
		t.Fatalf("err = %v", err)
	}
	// Run 502 is reused by the cleanup's delete, so a topics note there would mean the update was approved.
	if slices.Contains(f.log, "merge update") || strings.Contains(f.notes[502], "topics") {
		t.Errorf("steps = %v, approvals %v: the update must be neither merged nor approved", f.log, f.notes)
	}
}

func TestUpdateRefusesAPlanThatIsNotDestructive(t *testing.T) {
	f, s := world(t)
	f.comments["update"] = "<code>github</code>: +0 ~1 -0 ±0"
	f.fps["update"] = nil
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: the update plan must destroy exactly [") || !strings.Contains(err.Error(), "], got []") {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(f.log, "merge update") {
		t.Error("an update that does not delete the topics must not be merged")
	}
}

func TestUpdateRequiresTheFingerprintOfThePRHead(t *testing.T) {
	f, s := world(t)
	marker, err := plan.EncodeMarker(strings.Repeat("a", 40), plan.Fingerprint{})
	if err != nil {
		t.Fatal(err)
	}
	f.raw["update"] = planMarker + "\n<code>github</code>: +0 ~1 -1 ±0\n> [!WARNING]\n" + marker
	err = s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: PR #2 has no plan comment for its head") {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(f.log, "merge update") {
		t.Error("a PR whose plan is for another head must not be merged")
	}
}

func TestUpdateRefusesWhenMainMovedSinceTheLastMerge(t *testing.T) {
	f, s := world(t)
	create := f.effects["create"]
	f.effects["create"] = func(f *fakeForge) { create(f); f.head++ } // someone else pushes to main after the create merge
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: main moved since the harness's last merge (now ") ||
		!strings.Contains(err.Error(), "someone changed the claims repo during the run; see docs/e2e.md") {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(f.log, "pr update") || f.approved[502] {
		t.Errorf("steps = %v, approved %v: no update PR may be opened on a main the harness did not verify", f.log, f.approved)
	}
}

func TestUpdateRejectsWhenMainMovedWhileItsPRWasOpen(t *testing.T) {
	f, s := world(t)
	f.parent["update"] = fakeSHA(99) // someone else's commit landed on main before the update merged
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: main moved while PR #2 was open: nothing was approved") {
		t.Fatalf("err = %v", err)
	}
	if !f.rejected[502] || f.approved[502] {
		t.Errorf("the update's reconcile must be rejected, never approved (rejected %v, approved %v)", f.rejected, f.approved)
	}
}

func TestUpdateRequiresTheDestructiveWarning(t *testing.T) {
	f, s := world(t)
	f.comments["update"] = "<code>github</code>: +0 ~1 -1 ±0" // the right fingerprint, but no warning
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: the update plan must destroy exactly [") {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(f.log, "merge update") || strings.Contains(f.notes[502], "topics") {
		t.Errorf("steps = %v, approvals %v: the update must be neither merged nor approved", f.log, f.notes)
	}
}

func TestUpdateRequiresTheReconcileToAskForApproval(t *testing.T) {
	f, s := world(t)
	f.approval["update"] = false
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: reconcile run 502 finished without asking for approval") {
		t.Fatalf("err = %v", err)
	}
}
