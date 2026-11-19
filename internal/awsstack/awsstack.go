// Package awsstack replays, plans and applies one rendered AWS stack against
// floci, the way the pipelines' plan-aws and apply-aws jobs do (spec §6.1 step
// 4, §6.2 step 3). Each job starts an empty floci (ADR-0004) and rebuilds what
// wet records: the environment's baseline, then the wet version of the stack,
// whose new version is planned against the state that replay wrote, in the same
// scratch directory with the configuration swapped (ADR-0030). The stack's
// state never leaves the job, and a job that runs floci holds no secret
// (ADR-0031).
package awsstack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

// ErrApplyFailed marks an Apply whose final tofu apply failed: unlike a refusal
// or a failed replay or plan, part of the plan may have been applied.
var ErrApplyFailed = errors.New("tofu apply failed, part of the plan may have been applied")

// Runner runs OpenTofu in a directory and returns what it prints on stdout.
type Runner interface {
	Tofu(dir string, args ...string) ([]byte, error)
}

// Config is one stack's job.
type Config struct {
	Stack    string // aws/<env>/_baseline or aws/<env>/components/<name>
	New      string // the fresh render's root; a stack missing from it is an orphan
	Wet      string // wet's rendered/ root; it may be missing, before the first reconcile
	Work     string // a scratch directory outside both renders; Work/rendered must not exist yet
	Endpoint string // floci's endpoint, written into floci_override.tf.json
}

// Result is a stack's plan.
type Result struct {
	Plan    *plan.Plan // the stack's changes
	Reduced []byte     // the plan as plan.Reduce writes it, for the plan artifact
	dir     string     // where it was planned
	saved   string     // the saved plan, which Apply applies
}

// stateFiles survive the swap from the wet version to the new one: the state
// the replay wrote, and the providers and modules init installed.
var stateFiles = map[string]bool{".terraform": true, "terraform.tfstate": true, "terraform.tfstate.backup": true}

// Plan rebuilds what wet records and plans the stack (spec §6.1 step 4). In
// Work/rendered, laid out like a render so relative module sources resolve the
// same way:
//  1. a Component stack's environment baseline is applied first: wet's
//     version, which the applied Component stacks were planned with, or the new
//     one when wet has none yet;
//  2. the wet version of the stack, when it has one, is applied;
//  3. the new version is planned in the same directory, against that state, or,
//     for an orphan, a destroy with the wet configuration.
//
// The replays apply with -auto-approve: they only rebuild the emulator's state.
func Plan(cfg Config, run Runner) (*Result, error) {
	env, baseline, err := parse(cfg.Stack)
	if err != nil {
		return nil, err
	}
	// tofu runs in the stack directory and resolves a relative -out there, so
	// every path built from Work is absolute.
	if cfg.Work, err = filepath.Abs(cfg.Work); err != nil {
		return nil, err
	}
	if err := checkWork(cfg); err != nil {
		return nil, err
	}
	fresh, old := stackDir(cfg.New, cfg.Stack), stackDir(cfg.Wet, cfg.Stack)
	hasNew, err := isStack(fresh)
	if err != nil {
		return nil, err
	}
	hasWet, err := isStack(old)
	if err != nil {
		return nil, err
	}
	if !hasNew && !hasWet {
		return nil, fmt.Errorf("%s: in neither the new render nor wet", cfg.Stack)
	}
	rendered := filepath.Join(cfg.Work, "rendered")
	if !baseline {
		base := "aws/" + env + "/_baseline"
		src, err := baselineSource(cfg, base)
		if err != nil {
			return nil, err
		}
		dir := stackDir(rendered, base)
		if err := stage(src, dir, cfg.Endpoint); err != nil {
			return nil, err
		}
		if err := replay(run, dir); err != nil {
			return nil, err
		}
	}
	dir := stackDir(rendered, cfg.Stack)
	destroy := false
	switch {
	case hasWet:
		if err := stage(old, dir, cfg.Endpoint); err != nil {
			return nil, err
		}
		if err := replay(run, dir); err != nil {
			return nil, err
		}
		if !hasNew {
			destroy = true
			break
		}
		if err := swap(fresh, dir, cfg.Endpoint); err != nil {
			return nil, err
		}
		if err := initStack(run, dir); err != nil {
			return nil, err
		}
	default:
		if err := stage(fresh, dir, cfg.Endpoint); err != nil {
			return nil, err
		}
		if err := initStack(run, dir); err != nil {
			return nil, err
		}
	}
	saved := filepath.Join(cfg.Work, wetdiff.Escape(cfg.Stack)+".tfplan")
	args := []string{"plan", "-input=false", "-lock=false", "-out=" + saved}
	if destroy {
		args = append(args, "-destroy")
	}
	if _, err := run.Tofu(dir, args...); err != nil {
		return nil, err
	}
	js, err := run.Tofu(dir, "show", "-json", saved)
	if err != nil {
		return nil, err
	}
	p, reduced, err := plan.Reduce(cfg.Stack, bytes.NewReader(js))
	if err != nil {
		return nil, err
	}
	return &Result{Plan: p, Reduced: reduced, dir: dir, saved: saved}, nil
}

// Apply re-plans the stack as Plan does, requires its changes to equal the
// gated fingerprint's for the stack, both ways, and applies the saved plan
// (spec §6.2 step 3). Anything else applies nothing.
func Apply(cfg Config, run Runner, gated plan.Fingerprint) (*Result, error) {
	want, ok := gated[cfg.Stack]
	if !ok {
		return nil, fmt.Errorf("%s: the gated fingerprint has no entry for this stack, so nothing was applied", cfg.Stack)
	}
	res, err := Plan(cfg, run)
	if err != nil {
		return nil, err
	}
	if err := sameChanges(cfg.Stack, res.Plan, want); err != nil {
		return nil, err
	}
	if _, err := run.Tofu(res.dir, "apply", "-input=false", res.saved); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", cfg.Stack, ErrApplyFailed, err)
	}
	return res, nil
}

// sameChanges compares a plan's changes with the gated ones for its stack.
func sameChanges(stack string, p *plan.Plan, gated []plan.Change) error {
	got, want := plan.FingerprintOf(p), plan.Fingerprint{stack: gated}
	extra, lost := got.Missing(want), want.Missing(got)
	if len(extra)+len(lost) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)", stack)
	for _, e := range extra {
		fmt.Fprintf(&b, "\n  not gated: %s", e)
	}
	for _, l := range lost {
		fmt.Fprintf(&b, "\n  gated, not planned: %s", l)
	}
	return errors.New(b.String())
}

// parse splits an AWS stack's slash path into its environment and whether it is
// the environment's baseline.
func parse(stack string) (env string, baseline bool, err error) {
	if !strings.HasPrefix(stack, "aws/") || !wetdiff.Reconcilable(stack) {
		return "", false, fmt.Errorf("%q is not an AWS stack this engine version reconciles (aws/<env>/_baseline, aws/<env>/components/<name>)", stack)
	}
	parts := strings.Split(stack, "/")
	return parts[1], parts[2] == "_baseline", nil
}

// checkWork keeps tofu out of the renders: Work is a fresh scratch directory
// outside both (ADR-0030).
func checkWork(cfg Config) error {
	work, err := filepath.Abs(cfg.Work)
	if err != nil {
		return err
	}
	for _, root := range []string{cfg.Wet, cfg.New} {
		abs, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		if rel, err := filepath.Rel(abs, work); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("work %s is inside the render %s: a stack runs in a scratch copy, never in a rendered tree (ADR-0030)", cfg.Work, root)
		}
	}
	if _, err := os.Stat(filepath.Join(work, "rendered")); err == nil {
		return fmt.Errorf("work %s already holds a rendered tree: give each stack a fresh scratch directory", cfg.Work)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// baselineSource is the baseline a Component stack's job applies first: wet's
// if it has one, else the new render's.
func baselineSource(cfg Config, base string) (string, error) {
	for _, root := range []string{cfg.Wet, cfg.New} {
		dir := stackDir(root, base)
		ok, err := isStack(dir)
		if err != nil {
			return "", err
		}
		if ok {
			return dir, nil
		}
	}
	return "", fmt.Errorf("%s: no baseline %s in wet or in the new render", cfg.Stack, base)
}

func stackDir(root, stack string) string { return filepath.Join(root, filepath.FromSlash(stack)) }

// isStack reports whether dir holds a stack; a missing directory, or a missing
// root, holds none.
func isStack(dir string) (bool, error) {
	_, err := os.Stat(filepath.Join(dir, wetdiff.StackFile))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	}
	return false, err
}

// stage copies a rendered stack into dir and adds floci's override.
func stage(src, dir, endpoint string) error {
	if err := copyTree(src, dir); err != nil {
		return err
	}
	return WriteOverride(dir, endpoint)
}

// copyTree copies src's directories and regular files into dir and refuses
// anything else: a link could point tofu, or WriteOverride, outside the scratch
// copy.
func copyTree(src, dir string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case !d.Type().IsRegular():
			return fmt.Errorf("%s: not a regular file: a render holds only files and directories", p)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

// swap replaces the configuration in dir with src's and keeps the state, the
// providers and the modules (ADR-0030: one state for both versions).
func swap(src, dir, endpoint string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !stateFiles[e.Name()] {
			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return stage(src, dir, endpoint)
}

func initStack(run Runner, dir string) error {
	_, err := run.Tofu(dir, "init", "-input=false", "-lockfile=readonly")
	return err
}

// replay inits and applies a stack version, to rebuild its state in floci.
func replay(run Runner, dir string) error {
	if err := initStack(run, dir); err != nil {
		return err
	}
	_, err := run.Tofu(dir, "apply", "-input=false", "-auto-approve")
	return err
}

// WriteOverride writes into a staged AWS stack what CI adds for floci (spec
// §5.6): floci_override.tf.json, with the endpoint and the provider flags that
// OpenTofu merges into the stack's provider block. The pipelines and the floci
// tests both write it with this function.
func WriteOverride(dir, endpoint string) error {
	b, err := json.MarshalIndent(map[string]any{
		"provider": map[string]any{"aws": map[string]any{
			"endpoints":                   map[string]any{"ecr": endpoint, "iam": endpoint, "sts": endpoint},
			"skip_credentials_validation": true,
			"skip_metadata_api_check":     "true",
			"skip_requesting_account_id":  true,
		}},
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "floci_override.tf.json"), append(b, '\n'), 0o644)
}
