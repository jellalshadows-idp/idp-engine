package workflowspec

import (
	"maps"
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
}
