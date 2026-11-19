package awsstack

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

// fakeTofu records every tofu run as "<dir under Work> [<config>, <state>]:
// <args>", with Work shown as $WORK. <config> is the version in the directory's
// main.tf.json, and <state> the content of its terraform.tfstate, so a line
// shows which configuration ran against which state. A replay (apply
// -auto-approve) writes "state of <config>"; show prints the plan JSON set in
// show. Every run must find floci_override.tf.json beside the stack.
type fakeTofu struct {
	t     *testing.T
	work  string
	show  string
	calls []string
	// failOn, when set, makes the runs it returns true for (the directory
	// under Work, the args) fail with errBoom after they are recorded.
	failOn func(rel string, args []string) bool
}

var errBoom = errors.New("boom")

func (f *fakeTofu) Tofu(dir string, args ...string) ([]byte, error) {
	f.t.Helper()
	rel, err := filepath.Rel(f.work, dir)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "floci_override.tf.json")); err != nil {
		f.t.Errorf("tofu %v in %s: no floci_override.tf.json", args, rel)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "main.tf.json"))
	if err != nil {
		f.t.Fatalf("tofu %v in %s: %v", args, rel, err)
	}
	var doc struct{ V string }
	if err := json.Unmarshal(raw, &doc); err != nil {
		f.t.Fatal(err)
	}
	state := "no state"
	if b, err := os.ReadFile(filepath.Join(dir, "terraform.tfstate")); err == nil {
		state = string(b)
	}
	line := fmt.Sprintf("%s [%s, %s]: %s", rel, doc.V, state, strings.ReplaceAll(strings.Join(args, " "), f.work, "$WORK"))
	f.calls = append(f.calls, filepath.ToSlash(line))
	if f.failOn != nil && f.failOn(filepath.ToSlash(rel), args) {
		return nil, errBoom
	}
	switch {
	case args[0] == "apply" && slices.Contains(args, "-auto-approve"):
		if err := os.WriteFile(filepath.Join(dir, "terraform.tfstate"), []byte("state of "+doc.V), 0o644); err != nil {
			f.t.Fatal(err)
		}
	case args[0] == "show":
		return []byte(f.show), nil
	}
	return nil, nil
}

// renders writes stacks into two trees, new and wet, each stack a directory
// with a main.tf.json naming its version and a lock file; a nil map leaves the
// tree out, as on a first reconcile.
func renders(t *testing.T, fresh, wet map[string]string) Config {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{New: filepath.Join(dir, "new"), Wet: filepath.Join(dir, "wet"), Work: filepath.Join(dir, "work"), Endpoint: "http://localhost:4566"}
	for root, stacks := range map[string]map[string]string{cfg.New: fresh, cfg.Wet: wet} {
		for stack, v := range stacks {
			d := filepath.Join(root, filepath.FromSlash(stack))
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, content := range map[string]string{"main.tf.json": `{"v":"` + v + `"}`, ".terraform.lock.hcl": "# lock " + v} {
				if err := os.WriteFile(filepath.Join(d, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	return cfg
}

// noChanges is the plan JSON of a plan that changes nothing.
const noChanges = `{"format_version":"1.2","resource_changes":[]}`

// roleUpdate is the plan JSON of a changed Component stack: its role's trust
// policy changes in place.
const roleUpdate = `{"format_version":"1.2","resource_changes":[{"address":"module.component.aws_iam_role.ci","change":{"actions":["update"],"before":{"name":"api-dev-ci"}}}]}`

func wantCalls(t *testing.T, f *fakeTofu, want ...string) {
	t.Helper()
	if !slices.Equal(f.calls, want) {
		t.Errorf("tofu runs:\n%s\nwant:\n%s", strings.Join(f.calls, "\n"), strings.Join(want, "\n"))
	}
}

// TestPlanANewComponent: no wet version, so nothing to replay. The
// environment's baseline is applied first, from the new render since wet has
// none, and the new version is planned in its own scratch copy.
func TestPlanANewComponent(t *testing.T) {
	cfg := renders(t, map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"}, nil)
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
	if _, err := Plan(cfg, f); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [new base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [new base, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [new, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/components/api [new, no state]: plan -input=false -lock=false -out=$WORK/aws.dev.components.api.tfplan",
		"rendered/aws/dev/components/api [new, no state]: show -json $WORK/aws.dev.components.api.tfplan",
	)
}

// TestPlanAChangedComponentReplaysWet: the baseline comes from wet, the wet
// version is applied, and the new version is planned in the same directory,
// against the state the replay wrote (ADR-0030), never against an empty one.
func TestPlanAChangedComponentReplaysWet(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: roleUpdate}
	res, err := Plan(cfg, f)
	if err != nil {
		t.Fatal(err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [wet base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [wet base, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [wet, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/components/api [wet, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [new, state of wet]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/components/api [new, state of wet]: plan -input=false -lock=false -out=$WORK/aws.dev.components.api.tfplan",
		"rendered/aws/dev/components/api [new, state of wet]: show -json $WORK/aws.dev.components.api.tfplan",
	)
	want := &plan.Plan{Stack: cfg.Stack, Changes: []plan.Change{{Address: "module.component.aws_iam_role.ci", Action: plan.Update}}}
	if !slices.Equal(res.Plan.Changes, want.Changes) || res.Plan.Stack != want.Stack {
		t.Errorf("plan = %+v, want %+v", res.Plan, want)
	}
	if got := string(res.Reduced); got != `{"format_version":"1.2","resource_changes":[{"address":"module.component.aws_iam_role.ci","change":{"actions":["update"]}}]}`+"\n" {
		t.Errorf("reduced = %s", got)
	}
	// The swap replaced the configuration: nothing of wet's version is left
	// beside the state, the lock file included.
	lock, err := os.ReadFile(filepath.Join(cfg.Work, "rendered", "aws", "dev", "components", "api", ".terraform.lock.hcl"))
	if err != nil || string(lock) != "# lock new" {
		t.Errorf("lock file after the swap = %q, %v; want the new version's", lock, err)
	}
}

// TestPlanAnOrphanDestroysWithItsWetConfig: a stack only wet has is replayed
// and planned as a destroy with its old configuration (spec §6.1 step 4.3).
func TestPlanAnOrphanDestroysWithItsWetConfig(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
	if _, err := Plan(cfg, f); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [wet base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [wet base, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [wet, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/components/api [wet, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [wet, state of wet]: plan -input=false -lock=false -out=$WORK/aws.dev.components.api.tfplan -destroy",
		"rendered/aws/dev/components/api [wet, state of wet]: show -json $WORK/aws.dev.components.api.tfplan",
	)
}

// TestPlanABaseline: a baseline needs no other stack first.
func TestPlanABaseline(t *testing.T) {
	cfg := renders(t, map[string]string{"aws/dev/_baseline": "new base"}, map[string]string{"aws/dev/_baseline": "wet base"})
	cfg.Stack = "aws/dev/_baseline"
	f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
	if _, err := Plan(cfg, f); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [wet base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [wet base, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/_baseline [new base, state of wet base]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [new base, state of wet base]: plan -input=false -lock=false -out=$WORK/aws.dev._baseline.tfplan",
		"rendered/aws/dev/_baseline [new base, state of wet base]: show -json $WORK/aws.dev._baseline.tfplan",
	)
}

// TestApplyAppliesTheGatedPlan: a re-plan equal to the gated fingerprint is
// applied from the saved plan, never with -auto-approve.
func TestApplyAppliesTheGatedPlan(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: roleUpdate}
	gated := plan.Fingerprint{cfg.Stack: {{Address: "module.component.aws_iam_role.ci", Action: plan.Update}}, "github": {}}
	if _, err := Apply(cfg, f, gated); err != nil {
		t.Fatal(err)
	}
	if got, want := f.calls[len(f.calls)-1], "rendered/aws/dev/components/api [new, state of wet]: apply -input=false $WORK/aws.dev.components.api.tfplan"; got != want {
		t.Errorf("last tofu run = %q, want %q", got, want)
	}
}

// TestApplyRefusesAPlanThatDiffersFromTheGatedOne: the re-plan must equal the
// gated fingerprint, both ways (spec §6.2 step 3); otherwise nothing is
// applied.
func TestApplyRefusesAPlanThatDiffersFromTheGatedOne(t *testing.T) {
	for name, gated := range map[string]plan.Fingerprint{
		"less was gated": {"aws/dev/components/api": {}},
		"more was gated": {"aws/dev/components/api": {
			{Address: "module.component.aws_iam_role.ci", Action: plan.Update},
			{Address: "module.component.aws_ecr_repository.this", Action: plan.Delete},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := renders(t,
				map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
				map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
			cfg.Stack = "aws/dev/components/api"
			f := &fakeTofu{t: t, work: cfg.Work, show: roleUpdate}
			_, err := Apply(cfg, f, gated)
			want := map[string]string{
				"less was gated": "aws/dev/components/api: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)\n  not gated: aws/dev/components/api: update module.component.aws_iam_role.ci",
				"more was gated": "aws/dev/components/api: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)\n  gated, not planned: aws/dev/components/api: delete module.component.aws_ecr_repository.this",
			}[name]
			if err == nil || err.Error() != want {
				t.Errorf("err = %v, want %q", err, want)
			}
			if last := f.calls[len(f.calls)-1]; !strings.Contains(last, ": show -json ") {
				t.Errorf("a tofu run followed the refused plan: %q", last)
			}
		})
	}
}

// TestApplyRefusesAStackTheGateNeverSaw: no entry in the gated fingerprint
// means no gate decision covers the stack; nothing runs.
func TestApplyRefusesAStackTheGateNeverSaw(t *testing.T) {
	cfg := renders(t, map[string]string{"aws/dev/_baseline": "new base"}, nil)
	cfg.Stack = "aws/dev/_baseline"
	f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
	_, err := Apply(cfg, f, plan.Fingerprint{"github": {}})
	if want := "aws/dev/_baseline: the gated fingerprint has no entry for this stack, so nothing was applied"; err == nil || err.Error() != want || len(f.calls) != 0 {
		t.Errorf("err = %v, tofu runs %v; want %q and none", err, f.calls, want)
	}
}

func TestPlanRefuses(t *testing.T) {
	base := map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"}
	for name, c := range map[string]struct {
		stack string
		fresh map[string]string
		edit  func(*Config)
		want  string
	}{
		"github":           {stack: "github", fresh: base, want: `"github" is not an AWS stack this engine version reconciles (aws/<env>/_baseline, aws/<env>/components/<name>)`},
		"a workspace":      {stack: "aws/dev/workspaces/db", fresh: base, want: `"aws/dev/workspaces/db" is not an AWS stack this engine version reconciles (aws/<env>/_baseline, aws/<env>/components/<name>)`},
		"in neither":       {stack: "aws/dev/components/web", fresh: base, want: "aws/dev/components/web: in neither the new render nor wet"},
		"without baseline": {stack: "aws/dev/components/api", fresh: map[string]string{"aws/dev/components/api": "new"}, want: "aws/dev/components/api: no baseline aws/dev/_baseline in wet or in the new render"},
		"work in wet":      {stack: "aws/dev/components/api", fresh: base, edit: func(c *Config) { c.Work = filepath.Join(c.Wet, "aws", "work") }, want: "is inside the render"},
		"work in a render": {stack: "aws/dev/components/api", fresh: base, edit: func(c *Config) { c.Work = filepath.Join(c.New, "aws", "work") }, want: "is inside the render"},
		"used work":        {stack: "aws/dev/components/api", fresh: base, edit: func(c *Config) { _ = os.MkdirAll(filepath.Join(c.Work, "rendered"), 0o755) }, want: "already holds a rendered tree"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := renders(t, c.fresh, nil)
			cfg.Stack = c.stack
			if c.edit != nil {
				c.edit(&cfg)
			}
			f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
			_, err := Plan(cfg, f)
			if err == nil || !strings.Contains(err.Error(), c.want) || len(f.calls) != 0 {
				t.Errorf("err = %v, tofu runs %v; want %q and none", err, f.calls, c.want)
			}
		})
	}
}

// finalApply is the saved plan's apply, as opposed to a replay's.
func finalApply(_ string, args []string) bool {
	return args[0] == "apply" && !slices.Contains(args, "-auto-approve")
}

// TestApplyReportsAFailedApply: tofu apply failing may have applied part of
// the plan, so the error says so and can be told apart from a refusal.
func TestApplyReportsAFailedApply(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: roleUpdate, failOn: finalApply}
	gated := plan.Fingerprint{cfg.Stack: {{Address: "module.component.aws_iam_role.ci", Action: plan.Update}}}
	res, err := Apply(cfg, f, gated)
	want := "aws/dev/components/api: tofu apply failed, part of the plan may have been applied: boom"
	if err == nil || err.Error() != want || !errors.Is(err, ErrApplyFailed) || !errors.Is(err, errBoom) || res != nil {
		t.Errorf("res = %v, err = %v; want nil and %q wrapping ErrApplyFailed", res, err, want)
	}
}

// TestApplyRefusalsAreNotApplyFailures: only the final tofu apply failing is an
// ErrApplyFailed; a refused plan applied nothing.
func TestApplyRefusalsAreNotApplyFailures(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: roleUpdate}
	if _, err := Apply(cfg, f, plan.Fingerprint{cfg.Stack: {}}); err == nil || errors.Is(err, ErrApplyFailed) {
		t.Errorf("refusal err = %v, want one that is not ErrApplyFailed", err)
	}
}

// TestPlanStopsWhenTheReplayFails: a failed replay apply leaves a partial
// state, so the new version is never planned against it.
func TestPlanStopsWhenTheReplayFails(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: roleUpdate, failOn: func(rel string, args []string) bool {
		return rel == "rendered/aws/dev/components/api" && args[0] == "apply"
	}}
	res, err := Plan(cfg, f)
	if err != errBoom || res != nil {
		t.Errorf("res = %v, err = %v; want nil and boom", res, err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [wet base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [wet base, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [wet, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/components/api [wet, no state]: apply -input=false -auto-approve",
	)
}

// TestPlanWithARelativeWork: tofu resolves -out against the stack directory,
// so the saved plan's path is absolute even when --work is relative.
func TestPlanWithARelativeWork(t *testing.T) {
	cfg := renders(t, map[string]string{"aws/dev/_baseline": "new base"}, nil)
	cfg.Stack = "aws/dev/_baseline"
	parent, err := filepath.EvalSymlinks(filepath.Dir(cfg.Work))
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeTofu{t: t, work: filepath.Join(parent, "work"), show: noChanges}
	t.Chdir(parent)
	cfg.Work = "work"
	if _, err := Plan(cfg, f); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [new base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [new base, no state]: plan -input=false -lock=false -out=$WORK/aws.dev._baseline.tfplan",
		"rendered/aws/dev/_baseline [new base, no state]: show -json $WORK/aws.dev._baseline.tfplan",
	)
}

// TestPlanRefusesASymlinkInARender: a render holds files and directories only;
// a link could point tofu, or the override write, outside the scratch copy.
func TestPlanRefusesASymlinkInARender(t *testing.T) {
	cfg := renders(t, map[string]string{"aws/dev/_baseline": "new base"}, nil)
	cfg.Stack = "aws/dev/_baseline"
	stack := filepath.Join(cfg.New, "aws", "dev", "_baseline")
	if err := os.Symlink(filepath.Join(stack, "main.tf.json"), filepath.Join(stack, "link.tf.json")); err != nil {
		t.Skipf("cannot create a symlink here (needs privileges on Windows; CI on Linux runs it): %v", err)
	}
	f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
	_, err := Plan(cfg, f)
	want := filepath.Join(stack, "link.tf.json") + ": not a regular file: a render holds only files and directories"
	if err == nil || err.Error() != want || len(f.calls) != 0 {
		t.Errorf("err = %v, tofu runs %v; want %q and none", err, f.calls, want)
	}
}

// TestPlanKeepsTheRenderUntouched: the scratch copy is the only place tofu
// writes; the render stays as rendered (ADR-0030).
func TestPlanKeepsTheRenderUntouched(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	if _, err := Plan(cfg, &fakeTofu{t: t, work: cfg.Work, show: noChanges}); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{cfg.New, cfg.Wet} {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && d.Name() != "main.tf.json" && d.Name() != ".terraform.lock.hcl" {
				err = errors.New(p + " was written into the render")
			}
			return err
		})
		if err != nil {
			t.Error(err)
		}
	}
}

// TestPlanStopsWhenTheBaselineReplayFails: a Component stack's plan replays its
// baseline first; when that apply fails, nothing else runs.
func TestPlanStopsWhenTheBaselineReplayFails(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: roleUpdate, failOn: func(rel string, args []string) bool {
		return rel == "rendered/aws/dev/_baseline" && args[0] == "apply"
	}}
	res, err := Plan(cfg, f)
	if err != errBoom || res != nil {
		t.Errorf("res = %v, err = %v; want nil and boom", res, err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [wet base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [wet base, no state]: apply -input=false -auto-approve",
	)
}
