package workflowspec

import (
	"maps"
	"path"
	"regexp"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"
)

const templatesDir = "examples/claims-repo/.github/workflows/"

var templateNames = []string{"drift.yaml", "pr.yaml", "reconcile.yaml", "recover.yaml"}

var callRef = regexp.MustCompile(`^jellalshadows-idp/idp-engine/\.github/workflows/([a-z]+\.yaml)@([0-9a-f]{40})$`)

func TestTheReusableWorkflowSet(t *testing.T) {
	var reusable, templates []string
	for _, f := range reusableWorkflows(t) {
		reusable = append(reusable, path.Base(f))
	}
	for _, f := range glob(t, templatesDir+"*.yaml") {
		templates = append(templates, path.Base(f))
	}
	if !slices.Equal(reusable, templateNames) || !slices.Equal(templates, templateNames) {
		t.Errorf("reusable workflows %v and templates %v must both be %v", reusable, templates, templateNames)
	}
}

// caller returns the one job of a template that calls a reusable workflow.
func caller(t *testing.T, w *workflow) (string, *job) {
	t.Helper()
	var ids []string
	for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
		if w.Jobs[id].Uses != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) != 1 {
		t.Fatalf("%s: want exactly one job that calls a reusable workflow, got %v", w.path, ids)
	}
	return ids[0], w.Jobs[ids[0]]
}

func TestTemplatesCallTheirReusableWorkflowAtOneCommit(t *testing.T) {
	shas := map[string]bool{}
	for _, name := range templateNames {
		w := loadWorkflow(t, templatesDir+name)
		id, j := caller(t, w)
		m := callRef.FindStringSubmatch(j.Uses)
		if m == nil || m[1] != name {
			t.Errorf("%s: job %s must call jellalshadows-idp/idp-engine/.github/workflows/%s@<commit>, got %q", name, id, name, j.Uses)
			continue
		}
		shas[m[2]] = true
		if j.Secrets.Kind == yaml.ScalarNode {
			t.Errorf("%s: secrets: inherit hands every secret over; pass them one by one", name)
			continue
		}
		var passed strmap
		if j.Secrets.Kind != 0 {
			if err := j.Secrets.Decode(&passed); err != nil {
				t.Fatalf("%s: secrets: %v", name, err)
			}
		}
		want := map[string]string{}
		for s := range loadWorkflow(t, ".github/workflows/"+name).call(t).Secrets {
			if s != "IDP_WRITER_PRIVATE_KEY" { // lives only in the idp-write environment (ADR-0023)
				want[s] = "${{ secrets." + s + " }}"
			}
		}
		if !maps.Equal(map[string]string(passed), want) {
			t.Errorf("%s: secrets passed = %v, want %v", name, passed, want)
		}
	}
	if len(shas) != 1 {
		t.Errorf("the templates must pin one engine commit, got %v", slices.Sorted(maps.Keys(shas)))
	}
}

func TestTemplatesGrantExactlyWhatTheReusableJobsNeed(t *testing.T) {
	rank := map[string]int{"": 0, "none": 0, "read": 1, "write": 2}
	for _, name := range templateNames {
		need := map[string]string{}
		for _, j := range loadWorkflow(t, ".github/workflows/"+name).Jobs {
			for scope, level := range j.Permissions.levels {
				if rank[level] > rank[need[scope]] {
					need[scope] = level
				}
			}
		}
		id, j := caller(t, loadWorkflow(t, templatesDir+name))
		wantPerms(t, name+" job "+id, j.Permissions, need)
	}
}

func wantConcurrency(t *testing.T, w *workflow, want concurrency) {
	t.Helper()
	if w.Concurrency == nil || *w.Concurrency != want {
		t.Errorf("%s: concurrency = %+v, want %+v", w.path, w.Concurrency, want)
	}
}

var wetGroup = concurrency{Group: "idp-wet", CancelInProgress: false, Queue: "max"}

func TestPRTemplate(t *testing.T) {
	w := loadWorkflow(t, templatesDir+"pr.yaml")
	if got := w.triggerNames(); !slices.Equal(got, []string{"pull_request"}) {
		t.Errorf("triggers = %v, want only pull_request", got)
	}
	n, ok := w.trigger("pull_request")
	if !ok {
		t.Fatal("no pull_request trigger")
	}
	var pr struct {
		Branches    []string `yaml:"branches"`
		Paths       []string `yaml:"paths"`
		PathsIgnore []string `yaml:"paths-ignore"`
	}
	if err := n.Decode(&pr); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pr.Branches, []string{"main"}) {
		t.Errorf("pull_request branches = %v, want [main]", pr.Branches)
	}
	if len(pr.Paths)+len(pr.PathsIgnore) != 0 {
		t.Errorf("a path filter would leave idp-gate pending on a PR that touches no claims; the required check must always report (ADR-0022)")
	}
	wantConcurrency(t, w, concurrency{Group: "idp-plan-${{ github.event.pull_request.number }}", CancelInProgress: true})
}

func TestPRTemplateGateMirrorsTheResult(t *testing.T) {
	w := loadWorkflow(t, templatesDir+"pr.yaml")
	g := mustJob(t, w, "idp-gate")
	if g.Name != "idp-gate" {
		t.Errorf("the job name is the required check's name: want idp-gate, got %q", g.Name)
	}
	if !slices.Equal(g.Needs, strlist{"pr"}) {
		t.Errorf("idp-gate needs = %v, want [pr]", g.Needs)
	}
	if g.If != "always()" {
		t.Errorf("idp-gate if = %q, want always(): a skipped required check counts as passing (ADR-0022)", g.If)
	}
	wantPerms(t, "idp-gate", g.Permissions, map[string]string{})
	if len(g.Steps) != 1 {
		t.Fatalf("idp-gate must have exactly one step, got %d", len(g.Steps))
	}
	wantEnv(t, "idp-gate", g.Steps[0], map[string]string{"RESULT": "${{ needs.pr.result }}"})
	wantRun(t, "idp-gate", g.Steps[0], `test "$RESULT" = success`)
}

func TestReconcileTemplate(t *testing.T) {
	w := loadWorkflow(t, templatesDir+"reconcile.yaml")
	if got := w.triggerNames(); !slices.Equal(got, []string{"push", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want push and workflow_dispatch", got)
	}
	n, ok := w.trigger("push")
	if !ok {
		t.Fatal("no push trigger")
	}
	var push struct {
		Branches []string `yaml:"branches"`
	}
	if err := n.Decode(&push); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(push.Branches, []string{"main"}) {
		t.Errorf("push branches = %v, want [main]", push.Branches)
	}
	wantConcurrency(t, w, wetGroup)
}

func TestDriftTemplate(t *testing.T) {
	w := loadWorkflow(t, templatesDir+"drift.yaml")
	if got := w.triggerNames(); !slices.Equal(got, []string{"schedule", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want schedule and workflow_dispatch", got)
	}
	n, ok := w.trigger("schedule")
	if !ok {
		t.Fatal("no schedule trigger")
	}
	var crons []struct {
		Cron string `yaml:"cron"`
	}
	if err := n.Decode(&crons); err != nil {
		t.Fatal(err)
	}
	if len(crons) != 1 || crons[0].Cron != "23 5 * * *" {
		t.Errorf("schedule = %v, want one cron '23 5 * * *' (spec §6.4)", crons)
	}
	wantConcurrency(t, w, wetGroup)
}

func TestRecoverTemplate(t *testing.T) {
	w := loadWorkflow(t, templatesDir+"recover.yaml")
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_dispatch"}) {
		t.Errorf("triggers = %v, want only workflow_dispatch", got)
	}
	n, ok := w.trigger("workflow_dispatch")
	if !ok {
		t.Fatal("no workflow_dispatch trigger")
	}
	var d struct {
		Inputs map[string]struct {
			Required bool   `yaml:"required"`
			Type     string `yaml:"type"`
		} `yaml:"inputs"`
	}
	if err := n.Decode(&d); err != nil {
		t.Fatal(err)
	}
	if in, ok := d.Inputs["run-id"]; !ok || !in.Required || in.Type != "string" || len(d.Inputs) != 1 {
		t.Errorf("inputs = %v, want exactly a required string run-id", d.Inputs)
	}
	_, j := caller(t, w)
	if !maps.Equal(map[string]string(j.With), map[string]string{"run-id": "${{ inputs.run-id }}"}) {
		t.Errorf("recover with = %v", j.With)
	}
	wantConcurrency(t, w, wetGroup)
}
