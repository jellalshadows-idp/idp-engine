package workflowspec

import (
	"maps"
	"slices"
	"strings"
	"testing"
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

func TestBotWritesUseTheWorkflowToken(t *testing.T) {
	for _, l := range everyStep(t) {
		if !invokes(l.step.Run, "comment ") && !invokes(l.step.Run, "issue ") {
			continue
		}
		if l.step.Env["GITHUB_TOKEN"] != "${{ github.token }}" {
			t.Errorf("%s: step %q must set GITHUB_TOKEN: ${{ github.token }}, so github-actions[bot] authors it (ADR-0018)", l.where, l.step.label())
		}
		if _, ok := l.step.Env["GH_TOKEN"]; ok {
			t.Errorf("%s: step %q must not set GH_TOKEN; it would take precedence over GITHUB_TOKEN", l.where, l.step.label())
		}
	}
}

func TestWriterKeyStaysInTheWriteEnvironment(t *testing.T) {
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			j := w.Jobs[id]
			for _, s := range j.Steps {
				if mentions(s, "secrets.IDP_WRITER_PRIVATE_KEY") && j.Environment != "idp-write" {
					t.Errorf("%s job %s: step %q reads the writer key outside environment idp-write", f, id, s.label())
				}
				if j.Environment == "idp-approval" && (s.Uses != "" || mentions(s, "secrets.")) {
					t.Errorf("%s job %s: the approval job must hold no secrets and run no actions (spec §6.2)", f, id)
				}
			}
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
