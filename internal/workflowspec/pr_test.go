package workflowspec

import (
	"maps"
	"path"
	"slices"
	"strings"
	"testing"
)

const prPath = ".github/workflows/pr.yaml"

func TestPRInterface(t *testing.T) {
	w := loadWorkflow(t, prPath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_call"}) {
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	wantOptionalSecrets(t, prPath, w.call(t), "IDP_READER_PRIVATE_KEY", "IDP_STATE_PASSPHRASE")
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"comment", "plan", "plan-aws", "validate", "verdict"}) {
		t.Errorf("jobs = %v", got)
	}
}

func TestPRJobPermissions(t *testing.T) {
	w := loadWorkflow(t, prPath)
	wantPerms(t, "validate", mustJob(t, w, "validate").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "plan", mustJob(t, w, "plan").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "plan-aws", mustJob(t, w, "plan-aws").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "comment", mustJob(t, w, "comment").Permissions, map[string]string{"contents": "read", "pull-requests": "write"})
	wantPerms(t, "verdict", mustJob(t, w, "verdict").Permissions, map[string]string{})
}

func TestPRPlansOnlySameRepoBranches(t *testing.T) {
	w := loadWorkflow(t, prPath)
	plan := mustJob(t, w, "plan")
	if want := "github.event.pull_request.head.repo.full_name == github.repository"; plan.If != want {
		t.Errorf("plan if = %q, want %q (spec §6.1: forks only validate and render)", plan.If, want)
	}
	if !slices.Equal(plan.Needs, strlist{"validate"}) {
		t.Errorf("plan needs = %v, want [validate]", plan.Needs)
	}
	if got := mustJob(t, w, "plan-aws").Needs; !slices.Equal(got, strlist{"plan"}) {
		t.Errorf("plan-aws needs = %v, want [plan]: it runs only after the same-repo plan job", got)
	}
	c := mustJob(t, w, "comment")
	if got := slices.Sorted(slices.Values(c.Needs)); !slices.Equal(got, []string{"plan", "plan-aws"}) {
		t.Errorf("comment needs = %v, want [plan plan-aws]", got)
	}
	// A PR without AWS stacks skips plan-aws; the comment still runs.
	if want := "always() && needs.plan.result == 'success' && (needs.plan-aws.result == 'success' || needs.plan-aws.result == 'skipped')"; c.If != want {
		t.Errorf("comment if = %q, want %q", c.If, want)
	}
}

func TestPRPlanDoesNotRefresh(t *testing.T) {
	// The read-only reader token cannot see admin-only repository fields, so a
	// refresh with it plans phantom updates. The PR plan compares the claims with
	// the state the last apply wrote (ADR-0025).
	w := loadWorkflow(t, prPath)
	_, p := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "plan")
	wantEnv(t, "plan plan", p, map[string]string{"GITHUB_TOKEN": "${{ steps.reader.outputs.token }}"})
	wantRun(t, "plan plan", p, `tofu plan -input=false -lock=false -refresh=false -out="$RUNNER_TEMP/idp/plans/github.tfplan"`)
}

func TestPRVerdictFailsForksAndFailures(t *testing.T) {
	w := loadWorkflow(t, prPath)
	v := mustJob(t, w, "verdict")
	if v.If != "always()" {
		t.Errorf("verdict if = %q, want always(): it must report even when a job failed or was skipped", v.If)
	}
	if got := slices.Sorted(slices.Values(v.Needs)); !slices.Equal(got, []string{"comment", "plan", "plan-aws", "validate"}) {
		t.Errorf("verdict needs = %v", got)
	}
	_, s := mustStep(t, "verdict", v.Steps, "decide")
	wantEnv(t, "verdict decide", s, map[string]string{
		"SAME_REPO": "${{ github.event.pull_request.head.repo.full_name == github.repository }}",
		"VALIDATE":  "${{ needs.validate.result }}",
		"PLAN":      "${{ needs.plan.result }}",
		"PLAN_AWS":  "${{ needs.plan-aws.result }}",
		"COMMENT":   "${{ needs.comment.result }}",
	})
	wantRun(t, "verdict decide", s,
		`if [ "$SAME_REPO" != true ]; then`,
		"a maintainer must push this branch to the repo",
		`if [ "$VALIDATE" != success ] || [ "$PLAN" != success ] || [ "$COMMENT" != success ]; then`,
		`if [ "$PLAN_AWS" != success ] && [ "$PLAN_AWS" != skipped ]; then`,
	)
}

func TestPRPlanSummaryCoversTheAffectedStacksAtTheHeadCommit(t *testing.T) {
	w := loadWorkflow(t, prPath)
	plan := mustJob(t, w, "plan")
	_, d := mustStep(t, "plan", plan.Steps, "diff")
	wantRun(t, "plan diff", d, `idp diff --new "$RUNNER_TEMP/idp/work/rendered" --wet .idp/wet/rendered`)
	_, st := mustStep(t, "plan", plan.Steps, "stacks")
	wantEnv(t, "plan stacks", st, map[string]string{"AFFECTED": "${{ steps.diff.outputs.affected }}"})
	wantRun(t, "plan stacks", st, `idp stacks --affected "$AFFECTED" --wet .idp/wet/rendered`)
	want := map[string]string{
		"affected":       "${{ steps.diff.outputs.affected }}",
		"aws":            "${{ steps.stacks.outputs.aws }}",
		"renders-id":     "${{ steps.upload-renders.outputs.artifact-id }}",
		"plan-github-id": "${{ steps.upload.outputs.artifact-id }}",
	}
	if !maps.Equal(map[string]string(plan.Outputs), want) {
		t.Errorf("plan outputs = %v, want %v", plan.Outputs, want)
	}
	_, s := mustStep(t, "comment", mustJob(t, w, "comment").Steps, "summary")
	if s.If != "" {
		t.Errorf("summary if = %q: it must run whatever is affected, so a PR with no changes still gets its comment", s.If)
	}
	wantEnv(t, "comment summary", s, map[string]string{
		"AFFECTED": "${{ needs.plan.outputs.affected }}",
		"HEAD_SHA": "${{ github.event.pull_request.head.sha }}",
	})
	wantRun(t, "comment summary", s, `--stacks "$AFFECTED"`, `--head-sha "$HEAD_SHA"`, `--plan-dir "$RUNNER_TEMP/idp/plans"`)
}

func TestPRCommentPostsTheSavedComment(t *testing.T) {
	w := loadWorkflow(t, prPath)
	c := mustJob(t, w, "comment")
	iGH, gh := mustStep(t, "comment", c.Steps, "download-github")
	iAWS, aws := mustStep(t, "comment", c.Steps, "download-aws")
	wantWith(t, "comment download-github", gh, map[string]string{"artifact-ids": "${{ needs.plan.outputs.plan-github-id }}", "path": "${{ runner.temp }}/idp/plans"})
	if want := "needs.plan.outputs.plan-github-id != ''"; gh.If != want {
		t.Errorf("comment download-github if = %q, want %q: the github plan is uploaded only when the github stack is affected", gh.If, want)
	}
	wantWith(t, "comment download-aws", aws, map[string]string{"pattern": "idp-plan-aws.*", "path": "${{ runner.temp }}/idp/plans"})
	if want := "needs.plan.outputs.aws != '[]'"; aws.If != want {
		t.Errorf("comment download-aws if = %q, want %q: with no AWS stack, no plan was uploaded", aws.If, want)
	}
	iSum, _ := mustStep(t, "comment", c.Steps, "summary")
	iPost, post := mustStep(t, "comment", c.Steps, "post")
	if !(iGH < iSum && iAWS < iSum && iSum < iPost) {
		t.Errorf("comment must download the plans, summarize them, then post (got indexes %d, %d, %d, %d)", iGH, iAWS, iSum, iPost)
	}
	wantEnv(t, "comment post", post, map[string]string{"PR_NUMBER": "${{ github.event.pull_request.number }}"})
	wantRun(t, "comment post", post, `idp comment --repo "$GITHUB_REPOSITORY" --pr "$PR_NUMBER" --body-file "$RUNNER_TEMP/idp/out/comment.md"`)
}

// TestPRPlanDownloadsNeverMergeAndNeverShadow: no plan download merges
// artifacts, so each lands in plans/idp-plan-<S>/ and idp plan-summary reads a
// stack's plan only from its own artifact's directory; the AWS pattern, the only
// download by pattern (floci jobs' artifacts), cannot match the github plan
// (ADR-0031).
func TestPRPlanDownloadsNeverMergeAndNeverShadow(t *testing.T) {
	w := loadWorkflow(t, prPath)
	c := mustJob(t, w, "comment")
	pattern := ""
	for _, s := range c.Steps {
		if !strings.HasPrefix(s.Uses, "actions/download-artifact@") {
			continue
		}
		if v, ok := s.With["merge-multiple"]; ok {
			t.Errorf("comment %s sets merge-multiple=%s: a floci job's artifact could shadow idp-plan-github's file", s.ID, v)
		}
		if p, ok := s.With["pattern"]; ok {
			pattern = p
		}
	}
	if pattern == "" {
		t.Fatal("comment downloads no AWS plans by pattern")
	}
	if ok, err := path.Match(pattern, "idp-plan-github"); err != nil || ok {
		t.Errorf("pattern %q matches idp-plan-github (err %v): the github plan goes by ID only", pattern, err)
	}
	for _, name := range []string{"idp-plan-aws.dev._baseline", "idp-plan-aws.dev.components.api"} {
		if ok, err := path.Match(pattern, name); err != nil || !ok {
			t.Errorf("pattern %q does not match %s (err %v)", pattern, name, err)
		}
	}
}

// TestPRTrustedArtifactsGoByID: an artifact that a job without floci uploads
// and a later job trusts is downloaded by the artifact ID its producer outputs,
// never by name or pattern (ADR-0031). An empty artifact-ids downloads every
// artifact of the run, so each such step runs only when its ID is not empty.
func TestPRTrustedArtifactsGoByID(t *testing.T) {
	w := loadWorkflow(t, prPath)
	byID := map[string]string{
		"plan-aws download":       "needs.plan.outputs.renders-id",
		"comment download-github": "needs.plan.outputs.plan-github-id",
	}
	seen := map[string]bool{}
	for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
		for _, s := range w.Jobs[id].Steps {
			if !strings.HasPrefix(s.Uses, "actions/download-artifact@") {
				continue
			}
			where := id + " " + s.ID
			ids, ok := s.With["artifact-ids"]
			if !ok {
				if _, name := s.With["name"]; name {
					t.Errorf("%s downloads by name", where)
				}
				if s.With["pattern"] != "idp-plan-aws.*" {
					t.Errorf("%s downloads by pattern %q: only the AWS plans, which floci jobs upload, may", where, s.With["pattern"])
				}
				continue
			}
			seen[where] = true
			want, known := byID[where]
			if !known || ids != "${{ "+want+" }}" {
				t.Errorf("%s artifact-ids = %q, want %q", where, ids, "${{ "+want+" }}")
			}
			if !strings.Contains(s.If, want+" != ''") {
				t.Errorf("%s if = %q: it must require %s != '' (an empty artifact-ids downloads every artifact of the run)", where, s.If, want)
			}
			if _, name := s.With["name"]; name {
				t.Errorf("%s sets name beside artifact-ids", where)
			}
			if _, pat := s.With["pattern"]; pat {
				t.Errorf("%s sets pattern beside artifact-ids", where)
			}
		}
	}
	for _, where := range slices.Sorted(maps.Keys(byID)) {
		if !seen[where] {
			t.Errorf("%s does not download by artifact-ids", where)
		}
	}
}

func TestPRPlansEachAWSStackInItsOwnJob(t *testing.T) {
	w := loadWorkflow(t, prPath)
	plan := mustJob(t, w, "plan")
	_, r := mustStep(t, "plan", plan.Steps, "upload-renders")
	if want := "steps.stacks.outputs.aws != '[]'"; r.If != want {
		t.Errorf("upload-renders if = %q, want %q", r.If, want)
	}
	wantWith(t, "plan upload-renders", r, map[string]string{
		"name":                 "idp-renders",
		"path":                 "${{ runner.temp }}/idp/renders",
		"include-hidden-files": "true",
		"retention-days":       "1",
	})
	checkAWSMatrix(t, w, "plan-aws")
	_, p := mustStep(t, "plan-aws", mustJob(t, w, "plan-aws").Steps, "plan")
	wantEnv(t, "plan-aws plan", p, map[string]string{"STACK": "${{ matrix.stack }}", "ACCOUNT": "${{ matrix.account }}"})
	wantRun(t, "plan-aws plan", p, `idp aws-stack plan --stack "$STACK" --account "$ACCOUNT"`, `--endpoint http://localhost:4566 --out-dir "$RUNNER_TEMP/idp/reduced"`)
}

// TestPRPlansCrossJobsReduced: tofu show -json holds the values a plan sets,
// in plaintext, so only reduced plans leave a job (ADR-0031).
func TestPRPlansCrossJobsReduced(t *testing.T) {
	w := loadWorkflow(t, prPath)
	plan := mustJob(t, w, "plan")
	_, red := mustStep(t, "plan", plan.Steps, "reduce")
	wantRun(t, "plan reduce", red, `idp plan-reduce --stack github --plan "$RUNNER_TEMP/idp/plans/github.json" --out-dir "$RUNNER_TEMP/idp/reduced"`)
	_, up := mustStep(t, "plan", plan.Steps, "upload")
	if want := "steps.stacks.outputs.github == 'true'"; up.If != want {
		t.Errorf("plan upload if = %q, want %q", up.If, want)
	}
	wantWith(t, "plan upload", up, map[string]string{
		"name":              "idp-plan-github",
		"path":              "${{ runner.temp }}/idp/reduced/github.json",
		"retention-days":    "1",
		"if-no-files-found": "error",
	})
	_, aws := mustStep(t, "plan-aws", mustJob(t, w, "plan-aws").Steps, "upload")
	wantWith(t, "plan-aws upload", aws, map[string]string{
		"name":              "idp-plan-${{ matrix.artifact }}",
		"path":              "${{ runner.temp }}/idp/reduced/${{ matrix.artifact }}.json",
		"retention-days":    "1",
		"if-no-files-found": "error",
	})
}
