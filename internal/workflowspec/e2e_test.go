package workflowspec

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

const e2ePath = ".github/workflows/e2e.yaml"

func TestE2EWorkflow(t *testing.T) {
	w := loadWorkflow(t, e2ePath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"schedule", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want schedule and workflow_dispatch (release PRs join in Phase 1e)", got)
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
	wantPerms(t, "e2e", j.Permissions, map[string]string{"contents": "read"})
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

func TestE2ETokensAreScoped(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	_, bot := mustStep(t, "e2e", j.Steps, "bot")
	want := map[string]string{
		"client-id":                "${{ vars.IDP_BOT_CLIENT_ID }}",
		"private-key":              "${{ secrets.IDP_BOT_PRIVATE_KEY }}",
		"owner":                    "${{ github.repository_owner }}",
		"repositories":             "idp-claims-e2e",
		"permission-actions":       "write",
		"permission-checks":        "read",
		"permission-contents":      "write",
		"permission-issues":        "read",
		"permission-members":       "read",
		"permission-pull-requests": "write",
		"permission-workflows":     "write",
	}
	if !maps.Equal(map[string]string(bot.With), want) {
		t.Errorf("bot token inputs = %v, want %v", bot.With, want)
	}
	_, h := mustStep(t, "e2e", j.Steps, "harness")
	wantEnv(t, "e2e harness", h, map[string]string{
		"E2E_BOT_TOKEN":      "${{ steps.bot.outputs.token }}",
		"E2E_APPROVER_TOKEN": "${{ secrets.IDP_E2E_APPROVER_TOKEN }}",
	})
	wantRun(t, "e2e harness", h, `"$RUNNER_TEMP/idp-e2e" --repo "$GITHUB_REPOSITORY_OWNER/idp-claims-e2e" --engine-sha "$GITHUB_SHA" --run "$GITHUB_RUN_ID"`)
	if strings.Contains(h.Run, "go ") {
		t.Errorf("harness run = %q: it runs the binary built without secrets, never the go tool", h.Run)
	}
	for _, s := range j.Steps {
		if s.ID != "harness" && s.ID != "config" && strings.Contains(s.Env["E2E_APPROVER_TOKEN"]+s.Run, "APPROVER") {
			t.Errorf("step %q sees the approver token: only the harness step may", s.label())
		}
	}
}
