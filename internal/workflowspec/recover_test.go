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
		`info="$(gh api "repos/$GITHUB_REPOSITORY/actions/runs/$RUN_ID" --jq '[.event, .head_branch, (.conclusion // "")] | @tsv')"`,
		`read -r event branch conclusion <<<"$info"`,
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
		`if [ "$(jq length <<<"$named")" != 1 ]; then`,
		// The issue body names the only artifact the recovery considers; artifact-check trusts it only once the API confirms it.
		`idp-wet-root artifact id: ([0-9]+)`,
		`if [ -z "$artifact_id" ]; then`,
		`names no idp-wet-root artifact: that run saved no wet root artifact, or it predates this engine version, so the workflow cannot recover it. Follow the runbook instead (docs/runbooks/pipelines.md#a-wet-push-failed).`,
		`echo "artifact-id=$artifact_id" >> "$GITHUB_OUTPUT"`,
	)
	// The issue body can be edited by anyone with issue write access, so the
	// API must name the artifact as that run's unexpired idp-wet-root (D-7).
	iArt, art := mustStep(t, "recover", r.Steps, "artifact-check")
	wantEnv(t, "recover artifact-check", art, map[string]string{
		"GH_TOKEN":    "${{ github.token }}",
		"RUN_ID":      "${{ inputs.run-id }}",
		"ARTIFACT_ID": "${{ steps.issue-check.outputs.artifact-id }}",
	})
	wantRun(t, "recover artifact-check", art,
		`if ! info="$(gh api "repos/$GITHUB_REPOSITORY/actions/artifacts/$ARTIFACT_ID" --jq '[.name, (.workflow_run.id | tostring), (.expired | tostring)] | @tsv')"; then`,
		`cannot be read in this repository: it was deleted, or someone edited the issue.`,
		// Tab only: the default IFS would trim spaces, so "idp-wet-root " would pass.
		`IFS=$'\t' read -r name run expired <<<"$info"`,
		`if [ "$name" != idp-wet-root ] || [ "$run" != "$RUN_ID" ]; then`,
		`if [ "$expired" != false ]; then`,
		`echo "artifact-id=$ARTIFACT_ID" >> "$GITHUB_OUTPUT"`,
	)
	// Both checks gate the download: neither may be skipped or let fail, or
	// an empty artifact ID would make the download fetch every artifact of the run.
	for _, s := range []step{issue, art} {
		if s.If != "" || s.ContinueOnError != "" {
			t.Errorf("recover %s: if %q, continue-on-error %q; it must always run and stop the job when it fails", s.ID, s.If, s.ContinueOnError)
		}
	}
	iDL, dl := mustStep(t, "recover", r.Steps, "download")
	wantWith(t, "recover download", dl, map[string]string{
		"artifact-ids":   "${{ steps.artifact-check.outputs.artifact-id }}",
		"run-id":         "${{ inputs.run-id }}",
		"github-token":   "${{ github.token }}",
		"merge-multiple": "true",
		"path":           "${{ runner.temp }}/idp/wet-root",
	})
	// A floci job of the failed run could have uploaded idp-wet-root first, so
	// the download never goes by name or pattern (ADR-0031).
	for _, k := range []string{"name", "pattern"} {
		if _, ok := dl.With[k]; ok {
			t.Errorf("recover download sets %s: the wet root goes by the artifact ID the issue names", k)
		}
	}
	iBase, base := mustStep(t, "recover", r.Steps, "base")
	wantRun(t, "recover base", base, `[[ "$base" =~ ^[0-9a-f]{40}$ ]]`, `if [ "$base" != "$head" ]; then`)
	iTok, tok := mustStep(t, "recover", r.Steps, "push-token")
	checkPushToken(t, "recover push-token", tok)
	iPush, push := mustStep(t, "recover", r.Steps, "push")
	wantEnv(t, "recover push", push, map[string]string{
		"GH_TOKEN": "${{ steps.push-token.outputs.token }}",
		"RENDER":   "${{ steps.base.outputs.render }}",
	})
	// A reconcile names the paths it commits in wet-paths, AWS renders
	// included (spec §6.2 step 4); a wet root saved before wet-paths existed
	// holds only the github stack.
	wantRun(t, "recover push", push,
		`if [ -f "$root/wet-paths" ]; then`,
		`while IFS= read -r name || [ -n "$name" ]; do paths+=(--path "$name"); done < "$root/wet-paths"`,
		`paths=(--path tfstate/github.tfstate)`,
		`if [ "${#paths[@]}" = 0 ]; then`,
		`idp wet-push --repo "$GITHUB_REPOSITORY" --root "$root"`,
	)
	if !(iReq < iRun && iRun < iIssue && iIssue < iArt && iArt < iDL && iDL < iBase && iBase < iPush) {
		t.Errorf("recover must check the request, the run, the issue and the artifact, then download, compare the base, and only then push (got %d %d %d %d %d %d %d)", iReq, iRun, iIssue, iArt, iDL, iBase, iPush)
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
