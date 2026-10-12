package workflowspec

import (
	"maps"
	"slices"
	"testing"
)

const recoverPath = ".github/workflows/recover.yaml"

func TestRecoverInterface(t *testing.T) {
	w := loadWorkflow(t, recoverPath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_call"}) {
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	c := w.call(t)
	if in, ok := c.Inputs["run-id"]; !ok || !in.Required || in.Type != "string" || len(c.Inputs) != 1 {
		t.Errorf("inputs = %v, want exactly a required string run-id", c.Inputs)
	}
	// Required: an environment secret reaches a reusable workflow only when the
	// caller passes it by name (ADR-0023).
	wantSecrets(t, recoverPath, c, map[string]bool{"IDP_WRITER_PRIVATE_KEY": true})
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"recover"}) {
		t.Errorf("jobs = %v", got)
	}
	r := mustJob(t, w, "recover")
	if r.Environment != "idp-write" {
		t.Errorf("recover environment = %q, want idp-write", r.Environment)
	}
	wantPerms(t, "recover", r.Permissions, map[string]string{"actions": "read", "contents": "read", "issues": "write"})
}

func TestRecoverChecksTheRunAndTheWetBase(t *testing.T) {
	w := loadWorkflow(t, recoverPath)
	r := mustJob(t, w, "recover")
	iReq, req := mustStep(t, "recover", r.Steps, "request")
	wantEnv(t, "recover request", req, map[string]string{"REF": "${{ github.ref }}", "RUN_ID": "${{ inputs.run-id }}"})
	wantRun(t, "recover request", req, `if [ "$REF" != refs/heads/main ]; then`, `if ! [[ "$RUN_ID" =~ ^[0-9]+$ ]]; then`)
	iRun, run := mustStep(t, "recover", r.Steps, "run-check")
	wantEnv(t, "recover run-check", run, map[string]string{"GH_TOKEN": "${{ github.token }}", "RUN_ID": "${{ inputs.run-id }}"})
	wantRun(t, "recover run-check", run,
		`[ "$branch" != main ]`,
		`[ "$event" != push ] && [ "$event" != workflow_dispatch ]`,
		// A cancelled reconcile still commits through its always() steps, and its
		// push can fail too.
		`if [ "$conclusion" != failure ] && [ "$conclusion" != cancelled ]; then`,
	)
	iIssue, issue := mustStep(t, "recover", r.Steps, "issue-check")
	wantEnv(t, "recover issue-check", issue, map[string]string{"GH_TOKEN": "${{ github.token }}", "RUN_ID": "${{ inputs.run-id }}"})
	wantRun(t, "recover issue-check", issue,
		`.title == "Wet push failed" and .user.login == "github-actions[bot]"`,
		`test("/actions/runs/" + $id + "([^0-9]|$)")`,
		`if [ "$named" != 1 ]; then`,
	)
	iDL, dl := mustStep(t, "recover", r.Steps, "download")
	wantWith(t, "recover download", dl, map[string]string{
		"name":         "idp-wet-root",
		"run-id":       "${{ inputs.run-id }}",
		"github-token": "${{ github.token }}",
		"path":         "${{ runner.temp }}/idp/wet-root",
	})
	iBase, base := mustStep(t, "recover", r.Steps, "base")
	wantRun(t, "recover base", base, `[[ "$base" =~ ^[0-9a-f]{40}$ ]]`, `if [ "$base" != "$head" ]; then`)
	iTok, tok := mustStep(t, "recover", r.Steps, "push-token")
	checkPushToken(t, "recover push-token", tok)
	iPush, push := mustStep(t, "recover", r.Steps, "push")
	wantEnv(t, "recover push", push, map[string]string{
		"GH_TOKEN": "${{ steps.push-token.outputs.token }}",
		"RENDER":   "${{ steps.base.outputs.render }}",
	})
	wantRun(t, "recover push", push, `paths=(--path tfstate/github.tfstate)`, `idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root"`)
	if !(iReq < iRun && iRun < iIssue && iIssue < iDL && iDL < iBase && iBase < iPush) {
		t.Errorf("recover must check the request, the run and the issue, then download, compare the base, and only then push (got %d %d %d %d %d %d)", iReq, iRun, iIssue, iDL, iBase, iPush)
	}
	if iTok < iBase {
		t.Errorf("recover must mint the wet-commit token only after comparing the base (got push-token %d, base %d)", iTok, iBase)
	}
	iClose, cl := mustStep(t, "recover", r.Steps, "close")
	wantRun(t, "recover close", cl, `idp issue close --repo "$GITHUB_REPOSITORY" --label idp-wet-push --title "Wet push failed"`)
	if iClose < iPush {
		t.Errorf("recover must close the issue only after the push (got close %d, push %d)", iClose, iPush)
	}
	if cl.If != "" {
		t.Errorf("recover close if = %q, want none: the issue must stay open when the push failed", cl.If)
	}
}
