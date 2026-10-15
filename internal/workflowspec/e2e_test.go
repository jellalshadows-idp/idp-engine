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

// e2eSecrets are the idp-e2e environment's secrets: only the harness step reads them.
var e2eSecrets = []string{"secrets.IDP_BOT_PRIVATE_KEY", "secrets.IDP_E2E_APPROVER_TOKEN"}

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
// and sees a boolean, never the value. Expression contexts are case-insensitive,
// so the match is too.
func TestE2ESecretsReachOnlyTheHarness(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	refs := func(text string) []string {
		var out []string
		for _, s := range e2eSecrets {
			if strings.Contains(strings.ToLower(text), strings.ToLower(s)) {
				out = append(out, s)
			}
		}
		return out
	}
	for _, k := range slices.Sorted(maps.Keys(j.Env)) {
		if r := refs(j.Env[k]); len(r) > 0 {
			t.Errorf("job env %s reads %v: every step would see it; only the harness step may", k, r)
		}
	}
	for _, s := range j.Steps {
		if s.ID == "harness" {
			continue
		}
		texts := map[string]string{"run": s.Run}
		for k, v := range s.Env {
			texts["env "+k] = v
		}
		for k, v := range s.With {
			texts["with "+k] = v
		}
		for _, where := range slices.Sorted(maps.Keys(texts)) {
			for _, r := range refs(texts[where]) {
				if s.ID == "config" && texts[where] == "${{ "+r+" != '' }}" {
					continue
				}
				t.Errorf("step %q %s reads %s: only the harness step may (the config step only as ${{ %s != '' }})", s.label(), where, r, r)
			}
		}
	}
	_, c := mustStep(t, "e2e", j.Steps, "config")
	wantEnv(t, "e2e config", c, map[string]string{
		"BOT_KEY_SET":  "${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}",
		"APPROVER_SET": "${{ secrets.IDP_E2E_APPROVER_TOKEN != '' }}",
	})
}
