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
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"apply-aws", "apply-github", "approve", "gate", "plan", "plan-aws"}) {
		t.Errorf("jobs = %v", got)
	}
	wantPerms(t, "plan", mustJob(t, w, "plan").Permissions, map[string]string{"contents": "read", "issues": "read"})
	wantPerms(t, "plan-aws", mustJob(t, w, "plan-aws").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "gate", mustJob(t, w, "gate").Permissions, map[string]string{"contents": "read", "issues": "read", "pull-requests": "read"})
	wantPerms(t, "approve", mustJob(t, w, "approve").Permissions, map[string]string{})
	wantPerms(t, "apply-aws", mustJob(t, w, "apply-aws").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "apply-github", mustJob(t, w, "apply-github").Permissions, map[string]string{"contents": "read", "issues": "write"})
}

func TestReconcileIsNeverReRun(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	for _, id := range []string{"plan", "apply-aws", "apply-github"} {
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
	_, st := mustStep(t, "plan", plan.Steps, "stacks")
	wantRun(t, "plan stacks", st, `idp stacks --affected "$AFFECTED" --wet .idp/wet/rendered`)
	// The three IDs are what the later jobs download by (ADR-0031).
	want := map[string]string{
		"wet-sha":        "${{ steps.wet.outputs.sha }}",
		"affected":       "${{ steps.diff.outputs.affected }}",
		"github":         "${{ steps.stacks.outputs.github }}",
		"aws":            "${{ steps.stacks.outputs.aws }}",
		"changes":        "${{ steps.reduce.outputs.changes }}",
		"renders-id":     "${{ steps.upload-renders.outputs.artifact-id }}",
		"reconcile-id":   "${{ steps.upload.outputs.artifact-id }}",
		"plan-github-id": "${{ steps.upload-plan.outputs.artifact-id }}",
	}
	if !maps.Equal(map[string]string(plan.Outputs), want) {
		t.Errorf("plan outputs = %v, want %v", plan.Outputs, want)
	}
}

// TestReconcileGatesWithTheWorkflowToken: the gate decides on every affected
// stack's reduced plan, in a job that holds no secret (spec §6.2 step 2).
func TestReconcileGatesWithTheWorkflowToken(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	gate := mustJob(t, w, "gate")
	if got := slices.Sorted(slices.Values(gate.Needs)); !slices.Equal(got, []string{"plan", "plan-aws"}) {
		t.Errorf("gate needs = %v, want [plan plan-aws]", got)
	}
	if want := "!cancelled() && needs.plan.result == 'success' && (needs.plan-aws.result == 'success' || needs.plan-aws.result == 'skipped')"; gate.If != want {
		t.Errorf("gate if = %q, want %q", gate.If, want)
	}
	if gate.Environment != "" {
		t.Errorf("gate environment = %q, want none: the gate holds no secret", gate.Environment)
	}
	wantOut := strmap{
		"decision": "${{ steps.gate.outputs.decision }}",
		"gated-id": "${{ steps.upload.outputs.artifact-id }}",
	}
	if !maps.Equal(gate.Outputs, wantOut) {
		t.Errorf("gate outputs = %v, want %v", gate.Outputs, wantOut)
	}
	checkPlanLayout(t, w, "gate", "summary")
	iGH, _ := mustStep(t, "gate", gate.Steps, "download-github")
	iAWS, _ := mustStep(t, "gate", gate.Steps, "download-aws")
	iSum, s := mustStep(t, "gate", gate.Steps, "summary")
	iGate, g := mustStep(t, "gate", gate.Steps, "gate")
	iUp, up := mustStep(t, "gate", gate.Steps, "upload")
	if !(iGH < iSum && iAWS < iSum && iSum < iGate && iGate < iUp) {
		t.Errorf("gate must download the plans, summarize them, gate, then upload the fingerprint (got indexes %d, %d, %d, %d, %d)", iGH, iAWS, iSum, iGate, iUp)
	}
	wantEnv(t, "gate summary", s, map[string]string{"AFFECTED": "${{ needs.plan.outputs.affected }}"})
	wantRun(t, "gate summary", s, `--stacks "$AFFECTED"`, `--head-sha "$GITHUB_SHA"`, `--plan-dir "$RUNNER_TEMP/idp/plans"`)
	wantEnv(t, "gate gate", g, map[string]string{"GITHUB_TOKEN": "${{ github.token }}"})
	wantRun(t, "gate gate", g, `idp gate --repo "$GITHUB_REPOSITORY" --sha "$GITHUB_SHA" --fingerprint "$RUNNER_TEMP/idp/out/fingerprint.json"`)
	if up.If != "" {
		t.Errorf("gate upload if = %q: the fingerprint always uploads, so apply-aws always finds its ID", up.If)
	}
	wantWith(t, "gate upload", up, map[string]string{
		"name":              "idp-gated",
		"path":              "${{ runner.temp }}/idp/out/fingerprint.json",
		"retention-days":    "1",
		"if-no-files-found": "error",
	})
}

// TestReconcilePlansEachAWSStackInItsOwnJob: plan-aws plans each stack as the
// PR did; only reduced plans leave a job (ADR-0031).
func TestReconcilePlansEachAWSStackInItsOwnJob(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	plan := mustJob(t, w, "plan")
	_, red := mustStep(t, "plan", plan.Steps, "reduce")
	wantRun(t, "plan reduce", red, `idp plan-reduce --stack github --plan "$RUNNER_TEMP/idp/plans/github.json" --out-dir "$RUNNER_TEMP/idp/reduced"`)
	_, up := mustStep(t, "plan", plan.Steps, "upload-plan")
	if want := "steps.stacks.outputs.github == 'true'"; up.If != want {
		t.Errorf("plan upload-plan if = %q, want %q", up.If, want)
	}
	wantWith(t, "plan upload-plan", up, map[string]string{"name": "idp-plan-github", "path": "${{ runner.temp }}/idp/reduced/github.json", "retention-days": "1"})
	_, r := mustStep(t, "plan", plan.Steps, "upload-renders")
	if want := "steps.stacks.outputs.aws != '[]'"; r.If != want {
		t.Errorf("plan upload-renders if = %q, want %q", r.If, want)
	}
	wantWith(t, "plan upload-renders", r, map[string]string{"name": "idp-renders", "path": "${{ runner.temp }}/idp/renders", "include-hidden-files": "true", "retention-days": "1"})
	checkAWSMatrix(t, w, "plan-aws")
	checkRendersKept(t, w)
	pa := mustJob(t, w, "plan-aws")
	if !slices.Equal(pa.Needs, strlist{"plan"}) {
		t.Errorf("plan-aws needs = %v, want [plan]", pa.Needs)
	}
	_, p := mustStep(t, "plan-aws", pa.Steps, "plan")
	wantEnv(t, "plan-aws plan", p, map[string]string{"STACK": "${{ matrix.stack }}", "ACCOUNT": "${{ matrix.account }}"})
	wantRun(t, "plan-aws plan", p, `idp aws-stack plan --stack "$STACK" --account "$ACCOUNT"`, `--endpoint http://localhost:4566 --out-dir "$RUNNER_TEMP/idp/reduced"`)
	_, pu := mustStep(t, "plan-aws", pa.Steps, "upload")
	wantWith(t, "plan-aws upload", pu, map[string]string{"name": "idp-plan-${{ matrix.artifact }}", "path": "${{ runner.temp }}/idp/reduced/${{ matrix.artifact }}.json"})
}

// TestReconcileGatePlanLayoutIsIndependentOfTheArtifactCount: where a plan
// lands must not depend on how many artifacts a download resolves to, and no
// floci artifact may land under the github plan's directory (ADR-0031).
func TestReconcileGatePlanLayoutIsIndependentOfTheArtifactCount(t *testing.T) {
	checkPlanLayout(t, loadWorkflow(t, reconcilePath), "gate", "summary")
}

// byIDDownloads is every download of an artifact a job without floci uploads
// and a later job trusts: by the artifact ID its producer outputs (ADR-0031).
var byIDDownloads = map[string]string{
	"plan-aws download":     "needs.plan.outputs.renders-id",
	"apply-aws download":    "needs.plan.outputs.renders-id",
	"apply-aws gated":       "needs.gate.outputs.gated-id",
	"gate download-github":  "needs.plan.outputs.plan-github-id",
	"apply-github download": "needs.plan.outputs.reconcile-id",
	"apply-github renders":  "needs.plan.outputs.renders-id",
}

// byPatternDownloads is every download by pattern: only what the floci jobs
// upload, which no later job trusts by name.
var byPatternDownloads = map[string]string{
	"gate download-aws":    "idp-plan-aws.*",
	"apply-github applied": "idp-applied-*",
}

// TestReconcileTrustedArtifactsGoByID: an artifact that a job without floci
// uploads and a later job trusts is downloaded by the artifact ID its producer
// outputs, never by name or pattern. An empty artifact-ids downloads every
// artifact of the run, so each such step runs only when its ID is not empty.
func TestReconcileTrustedArtifactsGoByID(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	seen := map[string]bool{}
	for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
		for _, s := range w.Jobs[id].Steps {
			if !strings.HasPrefix(s.Uses, "actions/download-artifact@") {
				continue
			}
			where := id + " " + s.ID
			seen[where] = true
			if _, name := s.With["name"]; name {
				t.Errorf("%s downloads by name", where)
			}
			ids, ok := s.With["artifact-ids"]
			if !ok {
				if want, known := byPatternDownloads[where]; !known || s.With["pattern"] != want {
					t.Errorf("%s downloads by pattern %q: only the floci jobs' artifacts may, %v", where, s.With["pattern"], byPatternDownloads)
				}
				continue
			}
			want, known := byIDDownloads[where]
			if !known || ids != "${{ "+want+" }}" {
				t.Errorf("%s artifact-ids = %q, want %q", where, ids, "${{ "+want+" }}")
			}
			if !strings.Contains(s.If, want+" != ''") {
				t.Errorf("%s if = %q: it must require %s != '' (an empty artifact-ids downloads every artifact of the run)", where, s.If, want)
			}
			if _, pat := s.With["pattern"]; pat {
				t.Errorf("%s sets pattern beside artifact-ids", where)
			}
		}
	}
	for _, m := range []map[string]string{byIDDownloads, byPatternDownloads} {
		for _, where := range slices.Sorted(maps.Keys(m)) {
			if !seen[where] {
				t.Errorf("%s does not download an artifact", where)
			}
		}
	}
}

// TestReconcileArtifactIDsComeFromTheirUploads: each ID a job outputs is the
// artifact-id of the upload step of that very artifact.
func TestReconcileArtifactIDsComeFromTheirUploads(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	for _, c := range []struct{ job, output, step, artifact string }{
		{"plan", "renders-id", "upload-renders", "idp-renders"},
		{"plan", "reconcile-id", "upload", "idp-reconcile"},
		{"plan", "plan-github-id", "upload-plan", "idp-plan-github"},
		{"gate", "gated-id", "upload", "idp-gated"},
	} {
		j := mustJob(t, w, c.job)
		if want := "${{ steps." + c.step + ".outputs.artifact-id }}"; j.Outputs[c.output] != want {
			t.Errorf("%s output %s = %q, want %q", c.job, c.output, j.Outputs[c.output], want)
		}
		_, up := mustStep(t, c.job, j.Steps, c.step)
		if !strings.HasPrefix(up.Uses, "actions/upload-artifact@") || up.With["name"] != c.artifact {
			t.Errorf("%s %s uploads %q with %q, want %s", c.job, c.step, up.With["name"], up.Uses, c.artifact)
		}
	}
}

// TestApplyAWSAppliesWhatTheGateDecided: each AWS stack re-plans in its own
// job, refuses a plan that differs from the gated one, applies, and marks the
// stack applied for the wet commit (spec §6.2 step 3).
func TestApplyAWSAppliesWhatTheGateDecided(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	checkAWSMatrix(t, w, "apply-aws")
	a := mustJob(t, w, "apply-aws")
	want := "!cancelled() && needs.gate.result == 'success' && needs.plan.outputs.aws != '[]' && (needs.gate.outputs.decision == 'auto' || needs.approve.result == 'success')"
	if a.If != want {
		t.Errorf("apply-aws if = %q, want %q", a.If, want)
	}
	if got := slices.Sorted(slices.Values(a.Needs)); !slices.Equal(got, []string{"approve", "gate", "plan"}) {
		t.Errorf("apply-aws needs = %v", got)
	}
	_, g := mustStep(t, "apply-aws", a.Steps, "gated")
	wantWith(t, "apply-aws gated", g, map[string]string{
		"artifact-ids":   "${{ needs.gate.outputs.gated-id }}",
		"path":           "${{ runner.temp }}/idp/gated",
		"merge-multiple": "true",
	})
	_, ap := mustStep(t, "apply-aws", a.Steps, "apply")
	wantEnv(t, "apply-aws apply", ap, map[string]string{"STACK": "${{ matrix.stack }}", "ACCOUNT": "${{ matrix.account }}"})
	wantRun(t, "apply-aws apply", ap,
		`idp aws-stack apply --stack "$STACK" --account "$ACCOUNT"`,
		`--fingerprint "$RUNNER_TEMP/idp/gated/fingerprint.json" --applied-dir "$RUNNER_TEMP/idp/applied"`)
	_, up := mustStep(t, "apply-aws", a.Steps, "upload")
	if up.If != "" {
		t.Errorf("apply-aws upload if = %q: it must keep the implicit success(), so a refused or failed apply marks nothing", up.If)
	}
	wantWith(t, "apply-aws upload", up, map[string]string{
		"name":              "idp-applied-${{ matrix.artifact }}",
		"path":              "${{ runner.temp }}/idp/applied/${{ matrix.artifact }}",
		"retention-days":    "1",
		"if-no-files-found": "error",
	})
}

func TestApprovalIsOneSecretlessJob(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "approve")
	if a.Environment != "idp-approval" {
		t.Errorf("approve environment = %q, want idp-approval", a.Environment)
	}
	if want := "needs.gate.outputs.decision == 'approval'"; a.If != want {
		t.Errorf("approve if = %q, want %q", a.If, want)
	}
	if !slices.Equal(a.Needs, strlist{"gate"}) {
		t.Errorf("approve needs = %v, want [gate]", a.Needs)
	}
}

func TestApplyRunsOnlyAfterTheGate(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "apply-github")
	// apply-github runs after apply-aws, whatever its legs concluded, so it
	// commits the github stack and every AWS stack that applied.
	want := "!cancelled() && needs.gate.result == 'success' && needs.plan.outputs.affected != '[]' && (needs.gate.outputs.decision == 'auto' || needs.approve.result == 'success')"
	if a.If != want {
		t.Errorf("apply-github if = %q, want %q", a.If, want)
	}
	if a.Environment != "idp-write" {
		t.Errorf("apply-github environment = %q, want idp-write", a.Environment)
	}
	if got := slices.Sorted(slices.Values(a.Needs)); !slices.Equal(got, []string{"apply-aws", "approve", "gate", "plan"}) {
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
	if want := "needs.plan.outputs.github == 'true' && needs.plan.outputs.changes != '0'"; s.If != want {
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
		"GITHUB_STACK":  "${{ needs.plan.outputs.github }}",
		"CHANGES":       "${{ needs.plan.outputs.changes }}",
		"APPLY_OUTCOME": "${{ steps.apply.outcome }}",
		"WET_SHA":       "${{ needs.plan.outputs.wet-sha }}",
	})
	// --wet is the wet tree the reconcile planned against, from the renders the
	// plan job uploaded: the download gives wet-root the wet AWS renders when an
	// AWS stack is affected, and wet-root treats a missing tree as empty (a
	// github-only reconcile skips the download).
	wantRun(t, "apply-github prepare", prep,
		`if [ "$CHANGES" = 0 ] || [ "$APPLY_OUTCOME" = success ]; then`,
		`args+=(--github-render)`,
		`idp wet-root --out "$RUNNER_TEMP/idp/wet-root" --wet-base "$WET_SHA"`,
		`--new "$RUNNER_TEMP/idp/in/rendered" --wet "$RUNNER_TEMP/idp/renders/wet" --applied-dir "$RUNNER_TEMP/idp/applied" "${args[@]}"`,
	)
	stateArg := strings.Index(prep.Run, `args+=(--github-state "$RUNNER_TEMP/idp/work/tfstate/github.tfstate")`)
	renderBranch := strings.Index(prep.Run, `if [ "$CHANGES" = 0 ] || [ "$APPLY_OUTCOME" = success ]; then`)
	if stateArg < 0 || renderBranch < 0 || stateArg > renderBranch {
		t.Errorf("prepare must name the state before, and outside, the render branch: the state is committed even when the apply failed")
	}
	iApplied, applied := mustStep(t, "apply-github", a.Steps, "applied")
	iRenders, renders := mustStep(t, "apply-github", a.Steps, "renders")
	iPrep, _ := mustStep(t, "apply-github", a.Steps, "prepare")
	if want := "always() && needs.plan.outputs.aws != '[]'"; applied.If != want || iApplied > iPrep {
		t.Errorf("applied if = %q, want %q, before prepare", applied.If, want)
	}
	wantWith(t, "apply-github applied", applied, map[string]string{"pattern": "idp-applied-*", "path": "${{ runner.temp }}/idp/applied", "merge-multiple": "true"})
	// A floci job could upload a huge marker artifact to stall this download
	// before the github state commit, so it is bounded, and prepare does not
	// depend on its outcome: a failed or timed-out download still commits.
	if tm := mapValue(stepNode(jobNode(w, "apply-github"), iApplied), "timeout-minutes"); tm == nil || tm.Value != "5" {
		t.Errorf("apply-github applied timeout-minutes = %v, want 5", tm)
	}
	if strings.Contains(prep.If, "steps.applied") {
		t.Errorf("prepare if = %q: it must not depend on the applied download's outcome", prep.If)
	}
	if iRenders > iPrep {
		t.Errorf("apply-github must fetch the renders before prepare, so wet-root finds the wet AWS renders under --wet")
	}
	wantWith(t, "apply-github renders", renders, map[string]string{
		"artifact-ids":   "${{ needs.plan.outputs.renders-id }}",
		"path":           "${{ runner.temp }}/idp/renders",
		"merge-multiple": "true",
	})
	_, push := mustStep(t, "apply-github", a.Steps, "push")
	wantEnv(t, "apply-github push", push, map[string]string{"GH_TOKEN": "${{ steps.push-token.outputs.token }}"})
	wantRun(t, "apply-github push", push,
		`while IFS= read -r name || [ -n "$name" ]; do paths+=(--path "$name"); done < "$RUNNER_TEMP/idp/wet-root/wet-paths"`,
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
		// A floci job of the run may have uploaded the name first; keep replaces
		// it, after every floci job ended, and the issue names the new ID.
		"overwrite": "true",
	})
	_, tok := mustStep(t, "apply-github", a.Steps, "push-token")
	if want := "always() && steps.prepare.outcome == 'success' && steps.prepare.outputs.paths != '0'"; tok.If != want {
		t.Errorf("push-token if = %q, want %q: with nothing to commit, nothing is pushed", tok.If, want)
	}
	if want := "always() && steps.push-token.outcome == 'success'"; push.If != want {
		t.Errorf("push if = %q, want %q", push.If, want)
	}
	if want := "always() && steps.prepare.outcome == 'success' && steps.prepare.outputs.paths != '0' && steps.push.outcome != 'success'"; issue.If != want {
		t.Errorf("issue if = %q, want %q", issue.If, want)
	}
	wantRun(t, "apply-github issue", issue, `idp issue open --repo "$GITHUB_REPOSITORY" --label idp-wet-push --title "Wet push failed"`)
	// The issue names the saved artifact by ID on its own line: idp-recover
	// downloads that ID, never the name a floci job could have uploaded first.
	wantEnv(t, "apply-github issue", issue, map[string]string{"KEEP_ID": "${{ steps.keep.outputs.artifact-id }}"})
	wantRun(t, "apply-github issue", issue,
		`if [ -n "$KEEP_ID" ]; then`,
		`echo "idp-wet-root artifact id: $KEEP_ID"`,
		`echo "The wet commit was not saved as an artifact, so the idp-recover workflow cannot recover it. Follow $RUNBOOK_URL"`,
	)
}

func TestRenderArtifactsKeepHiddenFiles(t *testing.T) {
	// .terraform.lock.hcl and .idp-rendered are dotfiles, which upload-artifact
	// drops by default; without the lock file the apply's tofu init fails.
	w := loadWorkflow(t, reconcilePath)
	_, up := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "upload")
	if want := "steps.diff.outputs.affected != '[]'"; up.If != want {
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
	wantWith(t, "apply-github download", dl, map[string]string{
		"artifact-ids":   "${{ needs.plan.outputs.reconcile-id }}",
		"path":           "${{ runner.temp }}/idp/in",
		"merge-multiple": "true",
	})
	_, stage := mustStep(t, "apply-github", a.Steps, "stage")
	wantRun(t, "apply-github stage", stage, `cp -R "$RUNNER_TEMP/idp/in/rendered" "$RUNNER_TEMP/idp/work/rendered"`)
}
