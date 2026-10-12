package workflowspec

import (
	"maps"
	"slices"
	"testing"
)

const prPath = ".github/workflows/pr.yaml"

func TestPRInterface(t *testing.T) {
	w := loadWorkflow(t, prPath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_call"}) {
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	wantOptionalSecrets(t, prPath, w.call(t), "IDP_READER_PRIVATE_KEY", "IDP_STATE_PASSPHRASE")
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"comment", "plan", "validate", "verdict"}) {
		t.Errorf("jobs = %v", got)
	}
}

func TestPRJobPermissions(t *testing.T) {
	w := loadWorkflow(t, prPath)
	wantPerms(t, "validate", mustJob(t, w, "validate").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "plan", mustJob(t, w, "plan").Permissions, map[string]string{"contents": "read"})
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
	if got := mustJob(t, w, "comment").Needs; !slices.Equal(got, strlist{"plan"}) {
		t.Errorf("comment needs = %v, want [plan]", got)
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
	if got := slices.Sorted(slices.Values(v.Needs)); !slices.Equal(got, []string{"comment", "plan", "validate"}) {
		t.Errorf("verdict needs = %v", got)
	}
	_, s := mustStep(t, "verdict", v.Steps, "decide")
	wantEnv(t, "verdict decide", s, map[string]string{
		"SAME_REPO": "${{ github.event.pull_request.head.repo.full_name == github.repository }}",
		"VALIDATE":  "${{ needs.validate.result }}",
		"PLAN":      "${{ needs.plan.result }}",
		"COMMENT":   "${{ needs.comment.result }}",
	})
	wantRun(t, "verdict decide", s,
		`if [ "$SAME_REPO" != true ]; then`,
		"a maintainer must push this branch to the repo",
		`if [ "$VALIDATE" != success ] || [ "$PLAN" != success ] || [ "$COMMENT" != success ]; then`,
	)
}

func TestPRPlanSummaryCoversTheAffectedStacksAtTheHeadCommit(t *testing.T) {
	w := loadWorkflow(t, prPath)
	plan := mustJob(t, w, "plan")
	_, d := mustStep(t, "plan", plan.Steps, "diff")
	wantRun(t, "plan diff", d, `idp diff --new "$RUNNER_TEMP/idp/work/rendered" --wet .idp/wet/rendered`)
	_, st := mustStep(t, "plan", plan.Steps, "stacks")
	wantRun(t, "plan stacks", st, `jq -e 'all(.[]; . == "github")'`)
	_, s := mustStep(t, "plan", plan.Steps, "summary")
	if s.If != "" {
		t.Errorf("summary if = %q: it must run whatever is affected, so a PR with no changes still gets its comment", s.If)
	}
	wantEnv(t, "plan summary", s, map[string]string{
		"AFFECTED":     "${{ steps.diff.outputs.affected }}",
		"GITHUB_STACK": "${{ steps.stacks.outputs.github }}",
		"HEAD_SHA":     "${{ github.event.pull_request.head.sha }}",
	})
	wantRun(t, "plan summary", s, `--stacks "$AFFECTED"`, `--head-sha "$HEAD_SHA"`, `plans+=(--plan "github=$RUNNER_TEMP/idp/plans/github.json")`)
	_, up := mustStep(t, "plan", plan.Steps, "upload")
	wantWith(t, "plan upload", up, map[string]string{
		"name":              "idp-pr-comment",
		"path":              "${{ runner.temp }}/idp/out/comment.md",
		"retention-days":    "1",
		"overwrite":         "true",
		"if-no-files-found": "error",
	})
}

func TestPRCommentPostsTheSavedComment(t *testing.T) {
	w := loadWorkflow(t, prPath)
	c := mustJob(t, w, "comment")
	_, dl := mustStep(t, "comment", c.Steps, "download")
	wantWith(t, "comment download", dl, map[string]string{"name": "idp-pr-comment", "path": "${{ runner.temp }}/idp/out"})
	_, post := mustStep(t, "comment", c.Steps, "post")
	wantEnv(t, "comment post", post, map[string]string{"PR_NUMBER": "${{ github.event.pull_request.number }}"})
	wantRun(t, "comment post", post, `idp comment --repo "$GITHUB_REPOSITORY" --pr "$PR_NUMBER" --body-file "$RUNNER_TEMP/idp/out/comment.md"`)
}
