package workflowspec

import (
	"maps"
	"slices"
	"testing"
)

const driftPath = ".github/workflows/drift.yaml"

func TestDriftInterface(t *testing.T) {
	w := loadWorkflow(t, driftPath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_call"}) {
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	wantOptionalSecrets(t, driftPath, w.call(t), "IDP_READER_PRIVATE_KEY", "IDP_STATE_PASSPHRASE")
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"drift"}) {
		t.Errorf("jobs = %v", got)
	}
	d := mustJob(t, w, "drift")
	wantPerms(t, "drift", d.Permissions, map[string]string{"contents": "read", "issues": "write"})
	if d.Environment != "" {
		t.Errorf("drift must not use an environment: it only reads")
	}
}

func TestDriftPlansTheWetRenderWithTheReaderToken(t *testing.T) {
	w := loadWorkflow(t, driftPath)
	d := mustJob(t, w, "drift")
	_, p := mustStep(t, "drift", d.Steps, "plan")
	if want := "steps.wet.outputs.stack == 'true'"; p.If != want {
		t.Errorf("plan if = %q, want %q", p.If, want)
	}
	if p.WorkingDirectory != ".idp/wet/rendered/github" {
		t.Errorf("plan working-directory = %q, want the wet render", p.WorkingDirectory)
	}
	wantEnv(t, "drift plan", p, map[string]string{"GITHUB_TOKEN": "${{ steps.reader.outputs.token }}"})
	wantRun(t, "drift plan", p, "-detailed-exitcode", `2) echo "drift=true" >> "$GITHUB_OUTPUT" ;;`)
}

func TestDriftChecksTheBootstrapWithTheRecordedIdentity(t *testing.T) {
	w := loadWorkflow(t, driftPath)
	_, c := mustStep(t, "drift", mustJob(t, w, "drift").Steps, "check")
	wantEnv(t, "drift check", c, map[string]string{
		"GH_TOKEN":      "${{ steps.reader.outputs.token }}",
		"IDP_BOOTSTRAP": "${{ vars.IDP_BOOTSTRAP }}",
	})
	wantRun(t, "drift check", c, "--params-env IDP_BOOTSTRAP --allow-hidden-bypass", `3) echo "drift=true" >> "$GITHUB_OUTPUT" ;;`)
}

func TestDriftReportsThroughOneBoundedIssue(t *testing.T) {
	w := loadWorkflow(t, driftPath)
	_, r := mustStep(t, "drift", mustJob(t, w, "drift").Steps, "report")
	wantEnv(t, "drift report", r, map[string]string{
		"PLAN_DRIFT":  "${{ steps.plan.outputs.drift }}",
		"CHECK_DRIFT": "${{ steps.check.outputs.drift }}",
	})
	wantRun(t, "drift report", r,
		`idp issue open --repo "$GITHUB_REPOSITORY" --label drift --title "Drift detected"`,
		`idp issue close --repo "$GITHUB_REPOSITORY" --label drift --title "Drift detected"`,
		`head -c 4000 "$RUNNER_TEMP/idp/out/bootstrap.txt"`,
	)
}
