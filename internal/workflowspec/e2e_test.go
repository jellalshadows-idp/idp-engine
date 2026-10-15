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
	wantRun(t, "e2e harness", h, `go run ./cmd/idp-e2e --repo "$GITHUB_REPOSITORY_OWNER/idp-claims-e2e" --engine-sha "$GITHUB_SHA" --run "$GITHUB_RUN_ID"`)
	for _, s := range j.Steps {
		if s.ID != "harness" && s.ID != "config" && strings.Contains(s.Env["E2E_APPROVER_TOKEN"]+s.Run, "APPROVER") {
			t.Errorf("step %q sees the approver token: only the harness step may", s.label())
		}
	}
}
