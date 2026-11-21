package workflowspec

import (
	"maps"
	"path"
	"slices"
	"strings"
	"testing"
)

// TestFlociJobsHoldNoSecret: floci's ECR runs a registry container, so floci
// reaches the runner's Docker socket (ADR-0004), and with it every file and
// environment variable of its job. A job that runs floci therefore holds no
// secret, no token beyond contents: read, no environment, and saves no cache
// (ADR-0031). Its floci is CI's, pinned by digest, which TestCIFlociJob pins.
func TestFlociJobsHoldNoSecret(t *testing.T) {
	ci := mustJob(t, loadWorkflow(t, ciPath), "floci")
	ciFloci := ci.Services["floci"]
	_, ciReady := mustStep(t, "ci floci", ci.Steps, "floci-ready")
	var found []string
	for _, path := range reusableWorkflows(t) {
		w := loadWorkflow(t, path)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			j := w.Jobs[id]
			s, ok := j.Services["floci"]
			if !ok {
				if len(j.Services) > 0 {
					t.Errorf("%s %s: a service other than floci", path, id)
				}
				continue
			}
			where := path + " " + id
			found = append(found, where)
			if s.Image != ciFloci.Image || !slices.Equal(s.Ports, ciFloci.Ports) || !slices.Equal(s.Volumes, ciFloci.Volumes) || len(s.Env) > 0 || s.Options != "" {
				t.Errorf("%s: floci = %+v, want CI's %+v", where, s, ciFloci)
			}
			if len(j.Services) != 1 {
				t.Errorf("%s: services besides floci", where)
			}
			if j.Environment != "" || len(j.Env) > 0 {
				t.Errorf("%s: environment %q, env %v; want none (ADR-0031)", where, j.Environment, j.Env)
			}
			wantPerms(t, where, j.Permissions, map[string]string{"contents": "read"})
			// A floci job's outputs would reach later jobs as trusted values, a
			// container would run beside the socket, and a cache would be read back.
			if len(j.Outputs) > 0 {
				t.Errorf("%s: outputs %v; a floci job declares none: a later job would trust them (ADR-0031)", where, j.Outputs)
			}
			if mapValue(jobNode(w, id), "container") != nil {
				t.Errorf("%s: declares a container; want none (ADR-0031)", where)
			}
			for _, st := range j.Steps {
				if strings.HasPrefix(st.Uses, "actions/cache") {
					t.Errorf("%s: step %s uses %s; a floci job saves and restores no cache (ADR-0031)", where, st.label(), st.Uses)
				}
			}
			for _, v := range scalars(jobNode(w, id)) {
				for _, banned := range []string{"secrets.", "github.token", "steps.encryption", "IDP_STATE_PASSPHRASE", "TF_ENCRYPTION"} {
					if strings.Contains(v, banned) {
						t.Errorf("%s: %q names %s: a job that runs floci holds no secret (ADR-0031)", where, v, banned)
					}
				}
			}
			_, ready := mustStep(t, where, j.Steps, "floci-ready")
			if ready.Run != ciReady.Run {
				t.Errorf("%s: floci-ready differs from CI's floci-ready", where)
			}
		}
	}
	want := []string{
		".github/workflows/pr.yaml plan-aws",
		".github/workflows/reconcile.yaml apply-aws",
		".github/workflows/reconcile.yaml plan-aws",
	}
	if slices.Sort(found); !slices.Equal(found, want) {
		t.Errorf("jobs that run floci = %v, want %v", found, want)
	}
}

// checkAWSMatrix pins the AWS stacks' matrix (spec §11 risk 3): one job per
// stack from idp stacks, which refuses more than a matrix runs; every leg
// finishes even when another fails; at most ten floci at a time; and no job,
// so no floci, when no AWS stack is affected. The renders come from the plan
// job by artifact ID, never by name (ADR-0031): the download is guarded, since
// an empty artifact-ids downloads every artifact of the run, and merges the one
// artifact so it extracts directly into the renders directory.
func checkAWSMatrix(t *testing.T, w *workflow, id string) {
	t.Helper()
	j := mustJob(t, w, id)
	if want := id + " ${{ matrix.stack }}"; j.Name != want {
		t.Errorf("%s name = %q, want %q", id, j.Name, want)
	}
	if !strings.Contains(j.If, "needs.plan.outputs.aws != '[]'") {
		t.Errorf("%s if = %q: it must skip the job when no AWS stack is affected, since an empty matrix fails", id, j.If)
	}
	strategy := mapValue(jobNode(w, id), "strategy")
	if strategy == nil || mapValue(strategy, "matrix") == nil {
		t.Fatalf("%s has no strategy.matrix: it runs one job per AWS stack", id)
	}
	got := map[string]string{}
	for _, key := range []string{"fail-fast", "max-parallel"} {
		if v := mapValue(strategy, key); v != nil {
			got[key] = v.Value
		}
	}
	if v := mapValue(mapValue(strategy, "matrix"), "include"); v != nil {
		got["matrix.include"] = v.Value
	}
	want := map[string]string{"fail-fast": "false", "max-parallel": "10", "matrix.include": "${{ fromJSON(needs.plan.outputs.aws) }}"}
	if !maps.Equal(got, want) || len(mapValue(strategy, "matrix").Content) != 2 {
		t.Errorf("%s strategy = %v, want %v and nothing else in the matrix", id, got, want)
	}
	_, dl := mustStep(t, id, j.Steps, "download")
	wantWith(t, id+" download", dl, map[string]string{
		"artifact-ids":   "${{ needs.plan.outputs.renders-id }}",
		"path":           "${{ runner.temp }}/idp/renders",
		"merge-multiple": "true",
	})
	for _, k := range []string{"name", "pattern"} {
		if _, ok := dl.With[k]; ok {
			t.Errorf("%s download sets %s: the renders go by artifact ID only (ADR-0031)", id, k)
		}
	}
	if want := "needs.plan.outputs.renders-id != ''"; dl.If != want {
		t.Errorf("%s download if = %q, want %q: an empty artifact-ids downloads every artifact of the run", id, dl.If, want)
	}
	// The renders extract flat into the download path, so the --new and --wet
	// trees idp aws-stack reads are that path's new and wet directories.
	dir := strings.Replace(dl.With["path"], "${{ runner.temp }}", "$RUNNER_TEMP", 1)
	found := false
	for _, st := range j.Steps {
		if !invokes(st.Run, "aws-stack") {
			continue
		}
		found = true
		wantRun(t, id+" "+st.ID, st, `--new "`+dir+`/new" --wet "`+dir+`/wet"`)
	}
	if !found {
		t.Errorf("%s runs no idp aws-stack", id)
	}
}

// checkRendersKept pins the plan job's copy of the AWS renders: the tree the
// job uploads is the one plan-aws and apply-aws read as new/ and wet/.
func checkRendersKept(t *testing.T, w *workflow) {
	t.Helper()
	plan := mustJob(t, w, "plan")
	_, up := mustStep(t, "plan", plan.Steps, "upload-renders")
	dir := strings.Replace(up.With["path"], "${{ runner.temp }}", "$RUNNER_TEMP", 1)
	_, r := mustStep(t, "plan", plan.Steps, "renders")
	wantRun(t, "plan renders", r,
		`mkdir -p "`+dir+`/new" "`+dir+`/wet"`,
		`"`+dir+`/new/aws"`,
		`"`+dir+`/wet/aws"`,
	)
}

// checkPlanLayout pins how a secret-less job (comment, gate) reads the reduced
// plans. actions/download-artifact extracts into path itself whenever a download
// resolves to one artifact, and into path/<artifact name>/ otherwise, so the
// layout must not depend on the artifact count: the github plan, by ID, is
// merged into plans/idp-plan-github, where plan-summary reads it; the AWS
// plans, which floci jobs upload, go by pattern into a directory apart from
// plans, so nothing a floci artifact holds can land under idp-plan-github; and
// a step copies each plan the trusted matrix names into plans/idp-plan-<A>/
// (ADR-0031).
func checkPlanLayout(t *testing.T, w *workflow, id, summaryID string) {
	t.Helper()
	j := mustJob(t, w, id)
	const plans = "${{ runner.temp }}/idp/plans"
	iGH, gh := mustStep(t, id, j.Steps, "download-github")
	iAWS, aws := mustStep(t, id, j.Steps, "download-aws")
	iLay, lay := mustStep(t, id, j.Steps, "layout")
	iSum, sum := mustStep(t, id, j.Steps, summaryID)
	wantWith(t, id+" download-github", gh, map[string]string{
		"artifact-ids":   "${{ needs.plan.outputs.plan-github-id }}",
		"merge-multiple": "true",
		"path":           plans + "/idp-plan-github",
	})
	if want := "needs.plan.outputs.plan-github-id != ''"; gh.If != want {
		t.Errorf("%s download-github if = %q, want %q: the github plan is uploaded only when the github stack is affected", id, gh.If, want)
	}
	wantWith(t, id+" download-aws", aws, map[string]string{
		"pattern":        "idp-plan-aws.*",
		"merge-multiple": "true",
		"path":           "${{ runner.temp }}/idp/aws-plans",
	})
	if p := aws.With["path"]; p == plans || strings.HasPrefix(p, plans+"/") {
		t.Errorf("%s download-aws path = %q, under %s: a floci artifact could land beside or over idp-plan-github", id, p, plans)
	}
	if want := "needs.plan.outputs.aws != '[]'"; aws.If != want {
		t.Errorf("%s download-aws if = %q, want %q: with no AWS stack, no plan was uploaded", id, aws.If, want)
	}
	pattern := aws.With["pattern"]
	if ok, err := path.Match(pattern, "idp-plan-github"); err != nil || ok {
		t.Errorf("%s pattern %q matches idp-plan-github (err %v): the github plan goes by ID only", id, pattern, err)
	}
	for _, name := range []string{"idp-plan-aws.dev._baseline", "idp-plan-aws.dev.components.api"} {
		if ok, err := path.Match(pattern, name); err != nil || !ok {
			t.Errorf("%s pattern %q does not match %s (err %v)", id, pattern, name, err)
		}
	}
	for _, s := range j.Steps {
		if strings.HasPrefix(s.Uses, "actions/download-artifact@") {
			if _, ok := s.With["name"]; ok {
				t.Errorf("%s %s downloads by name", id, s.ID)
			}
		}
	}
	if want := "needs.plan.outputs.aws != '[]'"; lay.If != want {
		t.Errorf("%s layout if = %q, want %q", id, lay.If, want)
	}
	wantEnv(t, id+" layout", lay, map[string]string{"AWS_STACKS": "${{ needs.plan.outputs.aws }}"})
	wantRun(t, id+" layout", lay,
		`artifacts="$(jq -r '.[].artifact' <<<"$AWS_STACKS")"`,
		`[ -n "$artifact" ] || continue`,
		`f="$RUNNER_TEMP/idp/aws-plans/$artifact.json"`,
		`if [ -f "$f" ] && [ ! -L "$f" ]; then`,
		`mkdir -p "$RUNNER_TEMP/idp/plans/idp-plan-$artifact"`,
		`cp "$f" "$RUNNER_TEMP/idp/plans/idp-plan-$artifact/$artifact.json"`,
		// A skipped plan is named: plan-summary then fails on that stack.
		`echo "::warning::no regular file $artifact.json among the downloaded AWS plans; plan-summary fails on the missing plan of that stack"`,
		`done <<<"$artifacts"`,
	)
	if strings.Contains(lay.Run, "${{") {
		t.Errorf("%s layout interpolates an expression in run: read the matrix through env", id)
	}
	if !(iGH < iLay && iAWS < iLay && iLay < iSum) {
		t.Errorf("%s must download both plans, lay them out, then summarize (got indexes %d, %d, %d, %d)", id, iGH, iAWS, iLay, iSum)
	}
	wantRun(t, id+" "+summaryID, sum, `--plan-dir "$RUNNER_TEMP/idp/plans"`)
}
