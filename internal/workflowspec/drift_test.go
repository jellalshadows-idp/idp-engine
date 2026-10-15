package workflowspec

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

const driftPath = ".github/workflows/drift.yaml"

func TestDriftInterface(t *testing.T) {
	w := loadWorkflow(t, driftPath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_call"}) {
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	// The writer key is required, because the plan refreshes with the writer token
	// (ADR-0025) and an environment secret reaches a reusable workflow only when
	// the caller passes it by name (ADR-0023). The reader key and the passphrase
	// stay optional, so the job names the missing one (ruling P-11).
	wantSecrets(t, driftPath, w.call(t), map[string]bool{
		"IDP_READER_PRIVATE_KEY": false,
		"IDP_STATE_PASSPHRASE":   false,
		"IDP_WRITER_PRIVATE_KEY": true,
	})
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"drift"}) {
		t.Errorf("jobs = %v", got)
	}
	d := mustJob(t, w, "drift")
	wantPerms(t, "drift", d.Permissions, map[string]string{"contents": "read", "issues": "write"})
	if d.Environment != "idp-write" {
		t.Errorf("drift environment = %q, want idp-write: the plan refreshes with the writer token (ADR-0025)", d.Environment)
	}
}

func TestDriftPlansTheWetRenderWithTheWriterToken(t *testing.T) {
	// The read-only reader token cannot see admin-only repository fields, so the
	// drift plan refreshes with a writer token in idp-write (ADR-0025).
	w := loadWorkflow(t, driftPath)
	d := mustJob(t, w, "drift")
	_, cfg := mustStep(t, "drift", d.Steps, "config")
	wantEnv(t, "drift config", cfg, map[string]string{
		"WRITER_CLIENT_ID": "${{ vars.IDP_WRITER_CLIENT_ID }}",
		"WRITER_KEY_SET":   "${{ secrets.IDP_WRITER_PRIVATE_KEY != '' }}",
	})
	iReader, _ := mustStep(t, "drift", d.Steps, "reader")
	iTok, tok := mustStep(t, "drift", d.Steps, "plan-token")
	if want := "steps.wet.outputs.stack == 'true'"; tok.If != want {
		t.Errorf("plan-token if = %q, want %q", tok.If, want)
	}
	checkManifestToken(t, "drift plan-token", tok)
	iPlan, p := mustStep(t, "drift", d.Steps, "plan")
	if !(iReader < iTok && iTok < iPlan) {
		t.Errorf("drift must mint the reader token, then the plan token, then plan (got indexes %d, %d, %d)", iReader, iTok, iPlan)
	}
	if want := "steps.wet.outputs.stack == 'true'"; p.If != want {
		t.Errorf("plan if = %q, want %q", p.If, want)
	}
	if p.WorkingDirectory != ".idp/wet/rendered/github" {
		t.Errorf("plan working-directory = %q, want the wet render", p.WorkingDirectory)
	}
	wantEnv(t, "drift plan", p, map[string]string{"GITHUB_TOKEN": "${{ steps.plan-token.outputs.token }}"})
	if strings.Contains(p.Run, "-refresh") {
		t.Errorf("the drift plan must refresh (ADR-0025): no -refresh flag")
	}
	wantRun(t, "drift plan", p, "-detailed-exitcode", `2) echo "drift=true" >> "$GITHUB_OUTPUT" ;;`)
}

func TestDriftChecksTheBootstrapWithTheRecordedIdentity(t *testing.T) {
	// The check keeps the reader token: it reads org settings that the writer App
	// has no permission for (ADR-0017, ADR-0025).
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
