package workflowspec

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const reconcilePath = ".github/workflows/reconcile.yaml"

func TestReconcileInterface(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_call"}) {
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	// The writer key is required: an environment secret reaches a reusable workflow
	// only when the caller passes it by name (ADR-0023). The passphrase stays
	// optional, so each job names it when it is missing (ruling P-11). The reader
	// key is gone: the plan refreshes with the writer token (ADR-0025).
	wantSecrets(t, reconcilePath, w.call(t), map[string]bool{
		"IDP_STATE_PASSPHRASE":   false,
		"IDP_WRITER_PRIVATE_KEY": true,
	})
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"apply-github", "approve", "plan"}) {
		t.Errorf("jobs = %v", got)
	}
	wantPerms(t, "plan", mustJob(t, w, "plan").Permissions, map[string]string{"contents": "read", "issues": "read", "pull-requests": "read"})
	wantPerms(t, "approve", mustJob(t, w, "approve").Permissions, map[string]string{})
	wantPerms(t, "apply-github", mustJob(t, w, "apply-github").Permissions, map[string]string{"contents": "read", "issues": "write"})
}

func TestReconcileIsNeverReRun(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	for _, id := range []string{"plan", "apply-github"} {
		j := mustJob(t, w, id)
		if len(j.Steps) == 0 || j.Steps[0].ID != "guard" {
			t.Errorf("%s: the first step must be the guard", id)
			continue
		}
		wantRun(t, id+" guard", j.Steps[0], `if [ "$GITHUB_RUN_ATTEMPT" != 1 ]; then`)
	}
	_, g := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "guard")
	wantEnv(t, "plan guard", g, map[string]string{"REF": "${{ github.ref }}"})
	wantRun(t, "plan guard", g, `if [ "$REF" != refs/heads/main ]; then`)
}

func TestReconcileRefusesWhileAWetPushIsUnrecovered(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	plan := mustJob(t, w, "plan")
	iGuard, _ := mustStep(t, "plan", plan.Steps, "guard")
	iUnrec, u := mustStep(t, "plan", plan.Steps, "unrecovered")
	if iUnrec != iGuard+1 {
		t.Errorf("the unrecovered-push check must come right after the guard, before anything reads wet")
	}
	wantEnv(t, "plan unrecovered", u, map[string]string{"GH_TOKEN": "${{ github.token }}"})
	wantRun(t, "plan unrecovered", u,
		`open="$(gh api "repos/$GITHUB_REPOSITORY/issues?labels=idp-wet-push&state=open&per_page=100" --jq '[.[] | select(.pull_request == null and .title == "Wet push failed" and .user.login == "github-actions[bot]")] | length')"`,
		`if [ "$open" != 0 ]; then`,
	)
}

func TestReconcileDiffsAgainstTheCurrentWet(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	plan := mustJob(t, w, "plan")
	_, d := mustStep(t, "plan", plan.Steps, "diff")
	wantEnv(t, "plan diff", d, map[string]string{"EVENT": "${{ github.event_name }}"})
	wantRun(t, "plan diff", d,
		`if [ "$EVENT" = workflow_dispatch ]; then all=(--all); fi`,
		`idp diff --new "$RUNNER_TEMP/idp/out/rendered" --wet .idp/wet/rendered "${all[@]}"`,
	)
	want := map[string]string{
		"wet-sha":  "${{ steps.wet.outputs.sha }}",
		"github":   "${{ steps.stacks.outputs.github }}",
		"changes":  "${{ steps.summary.outputs.changes }}",
		"decision": "${{ steps.gate.outputs.decision }}",
	}
	if !maps.Equal(map[string]string(plan.Outputs), want) {
		t.Errorf("plan outputs = %v, want %v", plan.Outputs, want)
	}
}

func TestReconcileGatesWithTheWorkflowToken(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	_, g := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "gate")
	wantEnv(t, "plan gate", g, map[string]string{"GITHUB_TOKEN": "${{ github.token }}"})
	wantRun(t, "plan gate", g, `idp gate --repo "$GITHUB_REPOSITORY" --sha "$GITHUB_SHA" --fingerprint "$RUNNER_TEMP/idp/out/fingerprint.json"`)
	_, s := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "summary")
	wantRun(t, "plan summary", s, `--stacks "$AFFECTED"`, `--head-sha "$GITHUB_SHA"`)
}

func TestApprovalIsOneSecretlessJob(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "approve")
	if a.Environment != "idp-approval" {
		t.Errorf("approve environment = %q, want idp-approval", a.Environment)
	}
	if want := "needs.plan.outputs.decision == 'approval'"; a.If != want {
		t.Errorf("approve if = %q, want %q", a.If, want)
	}
	if !slices.Equal(a.Needs, strlist{"plan"}) {
		t.Errorf("approve needs = %v, want [plan]", a.Needs)
	}
}

func TestApplyRunsOnlyAfterTheGate(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "apply-github")
	want := "always() && needs.plan.result == 'success' && needs.plan.outputs.github == 'true' && (needs.plan.outputs.decision == 'auto' || needs.approve.result == 'success')"
	if a.If != want {
		t.Errorf("apply-github if = %q, want %q", a.If, want)
	}
	if a.Environment != "idp-write" {
		t.Errorf("apply-github environment = %q, want idp-write", a.Environment)
	}
	if got := slices.Sorted(slices.Values(a.Needs)); !slices.Equal(got, []string{"approve", "plan"}) {
		t.Errorf("apply-github needs = %v", got)
	}
}

func TestApplyRefusesWhenWetMoved(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "apply-github")
	iCheckout, co := mustStep(t, "apply-github", a.Steps, "wet-checkout")
	wantWith(t, "apply-github wet-checkout", co, map[string]string{"ref": "wet", "path": ".idp/wet"})
	iBase, base := mustStep(t, "apply-github", a.Steps, "wet-base")
	wantEnv(t, "apply-github wet-base", base, map[string]string{"PLANNED": "${{ needs.plan.outputs.wet-sha }}"})
	wantRun(t, "apply-github wet-base", base, `if [ "$(git -C .idp/wet rev-parse HEAD)" != "$PLANNED" ]; then`)
	iApply, _ := mustStep(t, "apply-github", a.Steps, "apply")
	if !(iCheckout < iBase && iBase < iApply) {
		t.Errorf("apply-github must check out wet, then compare it with the plan's, then apply (got indexes %d, %d, %d)", iCheckout, iBase, iApply)
	}
}

// writerManifestPermissions maps bootstrap/apps/writer.json to permission-*
// inputs. metadata is left out: every installation token carries it.
func writerManifestPermissions(t *testing.T) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "bootstrap", "apps", "writer.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		DefaultPermissions map[string]string `json:"default_permissions"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for k, v := range m.DefaultPermissions {
		if k != "metadata" {
			out["permission-"+strings.ReplaceAll(k, "_", "-")] = v
		}
	}
	return out
}

// checkManifestToken pins a writer token that plans or applies the GitHub
// stack: exactly the writer App's client id and key, the whole installation
// (no repositories input), because the claims may declare any repository, and
// exactly the writer manifest's permissions (ADR-0023, ADR-0025).
func checkManifestToken(t *testing.T, where string, s step) {
	t.Helper()
	if !strings.HasPrefix(s.Uses, "actions/create-github-app-token@") {
		t.Errorf("%s: uses = %q, want actions/create-github-app-token", where, s.Uses)
	}
	want := writerManifestPermissions(t)
	want["client-id"] = "${{ vars.IDP_WRITER_CLIENT_ID }}"
	want["private-key"] = "${{ secrets.IDP_WRITER_PRIVATE_KEY }}"
	want["owner"] = "${{ github.repository_owner }}"
	if !maps.Equal(map[string]string(s.With), want) {
		t.Errorf("%s: with = %v, want %v (the writer manifest's permissions over the whole installation)", where, s.With, want)
	}
}

func TestApplyTokenHasTheWriterManifestPermissions(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	_, s := mustStep(t, "apply-github", mustJob(t, w, "apply-github").Steps, "apply-token")
	if want := "needs.plan.outputs.changes != '0'"; s.If != want {
		t.Errorf("apply-token if = %q, want %q", s.If, want)
	}
	checkManifestToken(t, "apply-github apply-token", s)
}

func TestReconcilePlansWithTheWriterToken(t *testing.T) {
	// The read-only reader token cannot see admin-only repository fields, so the
	// plan refreshes with a writer token in idp-write, on main only (ADR-0025).
	w := loadWorkflow(t, reconcilePath)
	plan := mustJob(t, w, "plan")
	if plan.Environment != "idp-write" {
		t.Errorf("plan environment = %q, want idp-write: the plan refreshes with the writer token (ADR-0025)", plan.Environment)
	}
	for _, s := range plan.Steps {
		if s.ID == "reader" {
			t.Errorf("plan: a step with id reader remains; the reconcile plan uses no reader token (ADR-0025)")
		}
	}
	_, cfg := mustStep(t, "plan", plan.Steps, "config")
	wantEnv(t, "plan config", cfg, map[string]string{
		"WRITER_CLIENT_ID": "${{ vars.IDP_WRITER_CLIENT_ID }}",
		"WRITER_KEY_SET":   "${{ secrets.IDP_WRITER_PRIVATE_KEY != '' }}",
		"PASSPHRASE_SET":   "${{ secrets.IDP_STATE_PASSPHRASE != '' }}",
	})
	iTok, tok := mustStep(t, "plan", plan.Steps, "plan-token")
	if want := "steps.stacks.outputs.github == 'true'"; tok.If != want {
		t.Errorf("plan-token if = %q, want %q", tok.If, want)
	}
	checkManifestToken(t, "plan plan-token", tok)
	iPlan, p := mustStep(t, "plan", plan.Steps, "plan")
	if iTok > iPlan {
		t.Errorf("plan-token must come before the plan")
	}
	wantEnv(t, "plan plan", p, map[string]string{"GITHUB_TOKEN": "${{ steps.plan-token.outputs.token }}"})
	if strings.Contains(p.Run, "-refresh") {
		t.Errorf("the reconcile plan must refresh (ADR-0025): no -refresh flag")
	}
	wantRun(t, "plan plan", p, `tofu plan -input=false -lock=false -out="$RUNNER_TEMP/idp/out/github.tfplan"`)
}

// checkPushToken pins the wet-commit token: the claims repo only, contents:
// write only (ADR-0023).
func checkPushToken(t *testing.T, where string, s step) {
	t.Helper()
	wantWith(t, where, s, map[string]string{
		"client-id":           "${{ vars.IDP_WRITER_CLIENT_ID }}",
		"private-key":         "${{ secrets.IDP_WRITER_PRIVATE_KEY }}",
		"owner":               "${{ github.repository_owner }}",
		"repositories":        "${{ github.event.repository.name }}",
		"permission-contents": "write",
	})
	for k := range s.With {
		if strings.HasPrefix(k, "permission-") && k != "permission-contents" {
			t.Errorf("%s: the wet-commit token must hold only contents: write, got %s", where, k)
		}
	}
}

func TestWetPushTokenReachesOnlyTheClaimsRepo(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	_, s := mustStep(t, "apply-github", mustJob(t, w, "apply-github").Steps, "push-token")
	checkPushToken(t, "apply-github push-token", s)
}

func TestApplyCommitsTheRenderOnlyAfterASuccessfulApply(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "apply-github")
	_, apply := mustStep(t, "apply-github", a.Steps, "apply")
	// apply-token keeps the implicit success() and the changes condition, so the
	// apply never starts after a failed step or an earlier cancel; once started, a
	// cancel must not kill OpenTofu mid-apply.
	if want := "always() && steps.apply-token.outcome == 'success'"; apply.If != want {
		t.Errorf("apply if = %q, want %q (it runs only after the apply token was minted, and a cancel must not stop it once it started)", apply.If, want)
	}
	wantRun(t, "apply-github apply", apply, "tofu init -input=false -lockfile=readonly", `tofu apply -input=false "$RUNNER_TEMP/idp/in/github.tfplan"`)
	_, prep := mustStep(t, "apply-github", a.Steps, "prepare")
	if want := "always() && steps.stage.outcome == 'success'"; prep.If != want {
		t.Errorf("prepare if = %q, want %q", prep.If, want)
	}
	wantEnv(t, "apply-github prepare", prep, map[string]string{
		"CHANGES":       "${{ needs.plan.outputs.changes }}",
		"APPLY_OUTCOME": "${{ steps.apply.outcome }}",
		"WET_SHA":       "${{ needs.plan.outputs.wet-sha }}",
	})
	wantRun(t, "apply-github prepare", prep,
		`echo "$WET_SHA" > "$root/wet-base"`,
		`if [ "$CHANGES" = 0 ] || [ "$APPLY_OUTCOME" = success ]; then`,
		`cp -R "$RUNNER_TEMP/idp/in/rendered/github" "$root/rendered/github"`,
	)
	stateCopy := strings.Index(prep.Run, `cp "$RUNNER_TEMP/idp/work/tfstate/github.tfstate" "$root/tfstate/github.tfstate"`)
	renderBranch := strings.Index(prep.Run, `if [ "$CHANGES" = 0 ] || [ "$APPLY_OUTCOME" = success ]; then`)
	if stateCopy < 0 || renderBranch < 0 || stateCopy > renderBranch {
		t.Errorf("prepare must copy the state before, and outside, the render branch: the state is committed even when the apply failed")
	}
	_, push := mustStep(t, "apply-github", a.Steps, "push")
	wantEnv(t, "apply-github push", push, map[string]string{
		"GH_TOKEN": "${{ steps.push-token.outputs.token }}",
		"RENDER":   "${{ steps.prepare.outputs.render }}",
	})
	wantRun(t, "apply-github push", push,
		`paths=(--path tfstate/github.tfstate)`,
		`if [ "$RENDER" = true ]; then paths+=(--path rendered/github); fi`,
		`idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root"`,
	)
}

func TestWetRootIsSavedBeforeThePush(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "apply-github")
	iKeep, keep := mustStep(t, "apply-github", a.Steps, "keep")
	iPush, push := mustStep(t, "apply-github", a.Steps, "push")
	_, issue := mustStep(t, "apply-github", a.Steps, "issue")
	if iKeep > iPush {
		t.Errorf("the wet root must be uploaded before the push")
	}
	if want := "always() && steps.prepare.outcome == 'success'"; keep.If != want {
		t.Errorf("keep if = %q, want %q", keep.If, want)
	}
	wantWith(t, "apply-github keep", keep, map[string]string{
		"name":                 "idp-wet-root",
		"path":                 "${{ runner.temp }}/idp/wet-root",
		"include-hidden-files": "true",
		"retention-days":       "7",
		"if-no-files-found":    "error",
	})
	if want := "always() && steps.push-token.outcome == 'success'"; push.If != want {
		t.Errorf("push if = %q, want %q", push.If, want)
	}
	if want := "always() && steps.prepare.outcome == 'success' && steps.push.outcome != 'success'"; issue.If != want {
		t.Errorf("issue if = %q, want %q", issue.If, want)
	}
	wantRun(t, "apply-github issue", issue, `idp issue open --repo "$GITHUB_REPOSITORY" --label idp-wet-push --title "Wet push failed"`)
}

func TestRenderArtifactsKeepHiddenFiles(t *testing.T) {
	// .terraform.lock.hcl and .idp-rendered are dotfiles, which upload-artifact
	// drops by default; without the lock file the apply's tofu init fails.
	w := loadWorkflow(t, reconcilePath)
	_, up := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "upload")
	if want := "steps.stacks.outputs.github == 'true'"; up.If != want {
		t.Errorf("upload if = %q, want %q", up.If, want)
	}
	wantWith(t, "plan upload", up, map[string]string{
		"name":                 "idp-reconcile",
		"path":                 "${{ runner.temp }}/idp/out",
		"include-hidden-files": "true",
		"retention-days":       "1",
		"if-no-files-found":    "error",
	})
	a := mustJob(t, w, "apply-github")
	_, dl := mustStep(t, "apply-github", a.Steps, "download")
	wantWith(t, "apply-github download", dl, map[string]string{"name": "idp-reconcile", "path": "${{ runner.temp }}/idp/in"})
	_, stage := mustStep(t, "apply-github", a.Steps, "stage")
	wantRun(t, "apply-github stage", stage, `cp -R "$RUNNER_TEMP/idp/in/rendered" "$RUNNER_TEMP/idp/work/rendered"`)
}
