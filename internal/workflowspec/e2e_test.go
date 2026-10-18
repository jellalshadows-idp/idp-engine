package workflowspec

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

const e2ePath = ".github/workflows/e2e.yaml"

// releasePRInput is the e2e.yaml input release.yaml fills with the release PR's
// head commit (ADR-0028).
const releasePRInput = "release-pr-sha"

func TestE2EWorkflow(t *testing.T) {
	w := loadWorkflow(t, e2ePath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"schedule", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want schedule and workflow_dispatch: release PRs reach the harness through a dispatch from release.yaml (ADR-0028)", got)
	}
	n, ok := w.trigger("schedule")
	if !ok {
		t.Fatal("no schedule")
	}
	var crons []struct {
		Cron string `yaml:"cron"`
	}
	if err := n.Decode(&crons); err != nil || len(crons) != 1 || crons[0].Cron != "41 3 * * *" {
		t.Errorf("schedule = %v, %v", crons, err)
	}
	if w.Concurrency == nil || *w.Concurrency != (concurrency{Group: "idp-e2e"}) {
		t.Errorf("concurrency = %+v, want group idp-e2e without cancellation", w.Concurrency)
	}
	j := mustJob(t, w, "e2e")
	if j.Environment != "idp-e2e" {
		t.Errorf("environment = %q, want idp-e2e: the bot key and the approver token live only there (ADR-0027)", j.Environment)
	}
	wantPerms(t, "e2e", j.Permissions, map[string]string{"contents": "read", "statuses": "write"})
	// The harness's waits are bounded (DefaultTimeouts, about 300 minutes in
	// the worst case), so the job must outlast them and the cleanup.
	if j.TimeoutMinutes != 330 {
		t.Errorf("timeout-minutes = %d, want 330", j.TimeoutMinutes)
	}
}

func TestE2EIsNeverReRun(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	if len(j.Steps) == 0 || j.Steps[0].ID != "guard" {
		t.Fatal("the first step must be the guard: a re-run reuses the run id and collides with the first attempt's branches")
	}
	wantRun(t, "e2e guard", j.Steps[0],
		`if [ "$GITHUB_RUN_ATTEMPT" != 1 ]; then`,
		`echo "::error::re-running idp-e2e reuses its run id and collides with the first attempt's branches; dispatch a new run (docs/e2e.md)"`,
		"exit 1")
	if len(j.Steps[0].Env) != 0 {
		t.Errorf("guard env = %v, want none", j.Steps[0].Env)
	}
}

func TestE2EBuildsTheHarnessWithoutSecrets(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	i, b := mustStep(t, "e2e", j.Steps, "build")
	wantRun(t, "e2e build", b, `go build -o "$RUNNER_TEMP/idp-e2e" ./cmd/idp-e2e`)
	if len(b.Env) != 0 || len(b.With) != 0 {
		t.Errorf("build env = %v, with = %v: the build step sees no secret", b.Env, b.With)
	}
	if h, _ := mustStep(t, "e2e", j.Steps, "harness"); h < i {
		t.Error("the harness runs after it is built")
	}
}

// e2eSecrets are the idp-e2e environment's secrets: only the harness step reads them.
var e2eSecrets = []string{"IDP_BOT_PRIVATE_KEY", "IDP_E2E_APPROVER_TOKEN"}

func TestE2EHarnessMintsTheBotToken(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	for _, s := range j.Steps {
		if strings.Contains(s.Uses, "actions/create-github-app-token") {
			t.Errorf("step %q mints a bot token: a token lives one hour and a run can last longer, so the harness mints and refreshes its own (ADR-0027)", s.label())
		}
	}
	_, h := mustStep(t, "e2e", j.Steps, "harness")
	want := map[string]string{
		"E2E_BOT_CLIENT_ID":   "${{ vars.IDP_BOT_CLIENT_ID }}",
		"E2E_BOT_PRIVATE_KEY": "${{ secrets.IDP_BOT_PRIVATE_KEY }}",
		"E2E_APPROVER_TOKEN":  "${{ secrets.IDP_E2E_APPROVER_TOKEN }}",
	}
	if !maps.Equal(map[string]string(h.Env), want) {
		t.Errorf("harness env = %v, want exactly %v", h.Env, want)
	}
	wantRun(t, "e2e harness", h, `"$RUNNER_TEMP/idp-e2e" --repo "$GITHUB_REPOSITORY_OWNER/idp-claims-e2e" --engine-sha "$GITHUB_SHA" --run "$GITHUB_RUN_ID"`)
	if strings.Contains(h.Run, "go ") {
		t.Errorf("harness run = %q: it runs the binary built without secrets, never the go tool", h.Run)
	}
}

// TestE2ESecretsReachOnlyTheHarness: the bot key and the approver token reach
// the harness step and nothing else. The config step only checks they are set,
// and sees a boolean, never the value (secretReaders does not count that).
func TestE2ESecretsReachOnlyTheHarness(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	for _, secret := range e2eSecrets {
		if got := secretReaders(j, secret); !slices.Equal(got, []string{"harness"}) {
			t.Errorf("%s is read by %v, want only the harness step: a job env would show it to every step", secret, got)
		}
	}
	_, c := mustStep(t, "e2e", j.Steps, "config")
	wantEnv(t, "e2e config", c, map[string]string{
		"BOT_KEY_SET":  "${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}",
		"APPROVER_SET": "${{ secrets.IDP_E2E_APPROVER_TOKEN != '' }}",
	})
}

func TestE2ETakesTheReleasePRHead(t *testing.T) {
	n, ok := loadWorkflow(t, e2ePath).trigger("workflow_dispatch")
	if !ok {
		t.Fatal("no workflow_dispatch trigger")
	}
	var d struct {
		Inputs map[string]struct {
			Required bool   `yaml:"required"`
			Type     string `yaml:"type"`
			Default  string `yaml:"default"`
		} `yaml:"inputs"`
	}
	if err := n.Decode(&d); err != nil {
		t.Fatal(err)
	}
	in, ok := d.Inputs[releasePRInput]
	if !ok || len(d.Inputs) != 1 || in.Required || in.Type != "string" || in.Default != "" {
		t.Errorf("inputs = %+v, want exactly an optional string %s that defaults to empty, so the nightly and a plain dispatch report nothing", d.Inputs, releasePRInput)
	}
}

func TestE2EReportsEveryOutcomeOnTheReleasePR(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	if len(j.Steps) < 3 || j.Steps[1].ID != "pending" {
		t.Fatal("the step right after the guard must post pending on the release PR, before anything can fail")
	}
	// post is the gh call that sets the status: state is the -f state=... value,
	// description the -f description=... value.
	post := func(state, description string) []string {
		return []string{
			`gh api -X POST "repos/$GITHUB_REPOSITORY/statuses/$RELEASE_PR_SHA" -f ` + state + ` -f context=idp-e2e \`,
			`-f description="` + description + `" \`,
			`-f target_url="$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID" > /dev/null`,
		}
	}
	statusEnv := map[string]string{"GH_TOKEN": "${{ github.token }}", "RELEASE_PR_SHA": "${{ inputs.release-pr-sha }}"}

	p := j.Steps[1]
	if p.If != "inputs.release-pr-sha != ''" {
		t.Errorf("pending if = %q, want inputs.release-pr-sha != '': the nightly and a plain dispatch report nothing", p.If)
	}
	if !maps.Equal(map[string]string(p.Env), statusEnv) {
		t.Errorf("pending env = %v, want %v", p.Env, statusEnv)
	}
	// The SHA check must stop the step: the input reaches an API path.
	wantLines(t, "e2e pending", p.Run, []string{
		`if ! [[ "$RELEASE_PR_SHA" =~ ^[0-9a-f]{40}$ ]]; then`,
		`echo "::error::release-pr-sha must be a full 40-character commit SHA"`,
		"exit 1",
		"fi",
	})
	wantLines(t, "e2e pending", p.Run, post("state=pending", "the end-to-end harness is running on main"))

	r := j.Steps[len(j.Steps)-1]
	if r.ID != "result" {
		t.Fatalf("the last step must post the result, got %q", r.label())
	}
	if r.If != "always() && steps.pending.outcome == 'success'" {
		t.Errorf("result if = %q: a failure anywhere after pending, preflight included, must still post failure; a run that never posted pending posts nothing", r.If)
	}
	resultEnv := maps.Clone(statusEnv)
	resultEnv["JOB_STATUS"] = "${{ job.status }}"
	if !maps.Equal(map[string]string(r.Env), resultEnv) {
		t.Errorf("result env = %v, want %v", r.Env, resultEnv)
	}
	wantLines(t, "e2e result", r.Run, append([]string{
		"state=failure",
		`if [ "$JOB_STATUS" = success ]; then state=success; fi`,
	}, post(`state="$state"`, "the end-to-end harness on main: $JOB_STATUS")...))
	if h, _ := mustStep(t, "e2e", j.Steps, "harness"); h >= len(j.Steps)-1 {
		t.Error("the result step must come after the harness")
	}
}
