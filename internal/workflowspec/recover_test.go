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
	wantOptionalSecrets(t, recoverPath, c, "IDP_WRITER_PRIVATE_KEY")
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
		`[ "$conclusion" != failure ]`,
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
	_, tok := mustStep(t, "recover", r.Steps, "push-token")
	checkPushToken(t, "recover push-token", tok)
	iPush, push := mustStep(t, "recover", r.Steps, "push")
	wantEnv(t, "recover push", push, map[string]string{
		"GH_TOKEN": "${{ steps.push-token.outputs.token }}",
		"RENDER":   "${{ steps.base.outputs.render }}",
	})
	wantRun(t, "recover push", push, `paths=(--path tfstate/github.tfstate)`, `idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root"`)
	if !(iReq < iRun && iRun < iDL && iDL < iBase && iBase < iPush) {
		t.Errorf("recover must check the request, then the run, download, compare the base, and only then push (got %d %d %d %d %d)", iReq, iRun, iDL, iBase, iPush)
	}
	_, cl := mustStep(t, "recover", r.Steps, "close")
	wantRun(t, "recover close", cl, `idp issue close --repo "$GITHUB_REPOSITORY" --label idp-wet-push --title "Wet push failed"`)
}
