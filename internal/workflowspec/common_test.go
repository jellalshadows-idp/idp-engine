package workflowspec

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestWorkflowFilesUseTheYamlExtension(t *testing.T) {
	for _, pattern := range []string{".github/workflows/*.yml", "examples/claims-repo/.github/workflows/*.yml", ".github/actions/*/action.yml"} {
		if got := glob(t, pattern); len(got) != 0 {
			t.Errorf("rename %v to .yaml: the contract tests read only .yaml files", got)
		}
	}
}

func TestWorkflowsDenyPermissionsByDefault(t *testing.T) {
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		wantPerms(t, f+" (top level)", w.Permissions, map[string]string{})
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			p := w.Jobs[id].Permissions
			if !p.present || p.scalar != "" {
				t.Errorf("%s job %s: list the job's permission scopes explicitly", f, id)
			}
		}
	}
}

func TestRemoteUsesArePinned(t *testing.T) {
	for _, f := range append(workflowFiles(t), actionFiles(t)...) {
		root := readYAML(t, f, &struct{}{})
		for _, u := range usesRefs(root) {
			if err := checkUses(u.value, u.comment); err != nil {
				t.Errorf("%s:%d: %v", f, u.line, err)
			}
		}
	}
}

func TestCheckoutNeverPersistsCredentials(t *testing.T) {
	for _, l := range everyStep(t) {
		if strings.HasPrefix(l.step.Uses, "actions/checkout@") && l.step.With["persist-credentials"] != "false" {
			t.Errorf("%s: checkout %q must set persist-credentials: false", l.where, l.step.label())
		}
	}
}

func TestRunScriptsTakeValuesFromEnv(t *testing.T) {
	for _, l := range everyStep(t) {
		if strings.Contains(l.step.Run, "${{") {
			t.Errorf("%s: step %q expands an expression inside run; pass it through env instead", l.where, l.step.label())
		}
	}
}

func TestTokensAreStepScoped(t *testing.T) {
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
			if _, ok := w.Env[k]; ok {
				t.Errorf("%s: workflow-level env sets %s; set tokens on the step that uses them (ADR-0023)", f, k)
			}
			for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
				if _, ok := w.Jobs[id].Env[k]; ok {
					t.Errorf("%s job %s: job-level env sets %s; set tokens on the step that uses them (ADR-0023)", f, id, k)
				}
			}
		}
	}
}

// botWriteErrors checks one step: when it posts a comment or opens an issue it
// authors them as github-actions[bot] through the workflow token.
func botWriteErrors(where string, s step) []string {
	if !invokes(s.Run, "comment") && !invokes(s.Run, "issue") {
		return nil
	}
	var errs []string
	if s.Env["GITHUB_TOKEN"] != "${{ github.token }}" {
		errs = append(errs, fmt.Sprintf("%s: step %q must set GITHUB_TOKEN: ${{ github.token }}, so github-actions[bot] authors it (ADR-0018)", where, s.label()))
	}
	if _, ok := s.Env["GH_TOKEN"]; ok {
		errs = append(errs, fmt.Sprintf("%s: step %q must not set GH_TOKEN; it would take precedence over GITHUB_TOKEN", where, s.label()))
	}
	return errs
}

func TestBotWritesUseTheWorkflowToken(t *testing.T) {
	for _, l := range everyStep(t) {
		for _, e := range botWriteErrors(l.where, l.step) {
			t.Error(e)
		}
	}
}

// writerKeyErrors reports every way a workflow can leak the writer key or let
// the approval job touch a secret. It scans every scalar under a job, not only
// the steps: job env, with, outputs and a caller job's secrets: block count too.
func writerKeyErrors(w *workflow) []string {
	var errs []string
	for k, v := range w.Env {
		if strings.Contains(v, "secrets.") {
			errs = append(errs, fmt.Sprintf("%s: workflow-level env %s reads a secret; read secrets in the step that needs them", w.path, k))
		}
	}
	for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
		j := w.Jobs[id]
		where := w.path + " job " + id
		node := jobNode(w, id)
		if node == nil {
			errs = append(errs, where+": job node not found")
			continue
		}
		if j.Secrets.Kind == yaml.ScalarNode && j.Secrets.Value == "inherit" {
			errs = append(errs, where+": secrets: inherit hands every secret to the called workflow; pass named secrets")
		}
		for _, s := range scalars(node) {
			if strings.Contains(s, "secrets[") || strings.Contains(s, "toJSON(secrets") {
				errs = append(errs, fmt.Sprintf("%s: %q reaches secrets without naming them", where, s))
			}
		}
		// units: the job's own keys apart from steps, then each step.
		type unit struct {
			label string
			text  []string
		}
		var units []unit
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value != "steps" {
				units = append(units, unit{"job level", scalars(node.Content[i+1])})
			}
		}
		for i, s := range j.Steps {
			if sn := stepNode(node, i); sn != nil {
				units = append(units, unit{fmt.Sprintf("step %q", s.label()), scalars(sn)})
			}
		}
		for _, u := range units {
			for _, s := range u.text {
				if j.Environment != "idp-write" && strings.Contains(s, "IDP_WRITER_PRIVATE_KEY") {
					errs = append(errs, fmt.Sprintf("%s: %s reads the writer key outside environment idp-write", where, u.label))
				}
				if j.Environment == "idp-approval" && strings.Contains(s, "secrets.") {
					errs = append(errs, fmt.Sprintf("%s: %s reads a secret; the approval job holds none (spec §6.2)", where, u.label))
				}
			}
		}
		if j.Environment == "idp-approval" {
			if j.Uses != "" {
				errs = append(errs, where+": the approval job must not call a workflow (spec §6.2)")
			}
			for _, s := range j.Steps {
				if s.Uses != "" {
					errs = append(errs, fmt.Sprintf("%s: step %q runs an action; the approval job runs none (spec §6.2)", where, s.label()))
				}
			}
		}
	}
	return errs
}

func TestWriterKeyStaysInTheWriteEnvironment(t *testing.T) {
	for _, f := range workflowFiles(t) {
		for _, e := range writerKeyErrors(loadWorkflow(t, f)) {
			t.Error(e)
		}
	}
}

func TestJobsBuildIdpFromTheWorkflowCommit(t *testing.T) {
	var inputs map[string]bool
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			where := f + " job " + id
			checkedOut := false
			for _, s := range w.Jobs[id].Steps {
				if strings.HasPrefix(s.Uses, "actions/checkout@") && s.With["path"] == engineDir {
					if s.With["repository"] != workflowRepo || s.With["ref"] != workflowSHA {
						t.Errorf("%s: the engine checkout must use repository %s and ref %s (ADR-0021)", where, workflowRepo, workflowSHA)
					}
					checkedOut = true
				}
				if !strings.HasPrefix(s.Uses, "./") {
					continue
				}
				if s.Uses != setupIDP {
					t.Errorf("%s: unexpected local action %q", where, s.Uses)
					continue
				}
				if !checkedOut {
					t.Errorf("%s: setup-idp runs before the engine checkout", where)
				}
				if s.With["engine-dir"] != engineDir || s.With["version"] != workflowSHA {
					t.Errorf("%s: setup-idp needs engine-dir %s and version %s, got %v", where, engineDir, workflowSHA, s.With)
				}
				if inputs == nil {
					inputs = map[string]bool{}
					for k := range loadAction(t, setupIDPPath).Inputs {
						inputs[k] = true
					}
				}
				for _, k := range slices.Sorted(maps.Keys(s.With)) {
					if !inputs[k] {
						t.Errorf("%s: setup-idp has no input %q", where, k)
					}
				}
			}
		}
	}
}

func TestIdpRunsOnlyAfterSetup(t *testing.T) {
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			ready := false
			for _, s := range w.Jobs[id].Steps {
				if s.Uses == setupIDP {
					ready = true
				}
				if invokes(s.Run, "") && !ready {
					t.Errorf("%s job %s: step %q runs idp before setup-idp", f, id, s.label())
				}
			}
		}
	}
}

func TestTofuFollowsTheWetHygieneRules(t *testing.T) {
	for _, f := range reusableWorkflows(t) {
		w := loadWorkflow(t, f)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			for _, s := range w.Jobs[id].Steps {
				for _, err := range checkTofu(s.Run) {
					t.Errorf("%s job %s step %q: %v", f, id, s.label(), err)
				}
			}
		}
	}
}
