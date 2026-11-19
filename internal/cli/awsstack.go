package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/awsstack"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

// newTofu runs the tofu binary; tests replace it.
var newTofu = func(path string, env []string, log io.Writer) awsstack.Runner {
	return awsstack.Exec{Path: path, Env: env, Log: log}
}

var accountID = regexp.MustCompile(`^[0-9]{12}$`)

// secretNames are what a job that runs floci must never hold (ADR-0031).
var secretNames = []string{"TF_ENCRYPTION", "IDP_STATE_PASSPHRASE"}

// runAWSStack plans one AWS stack against floci for plan-aws, or re-plans and
// applies it for apply-aws (spec §6.1 step 4, §6.2 step 3).
func runAWSStack(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 || (args[0] != "plan" && args[0] != "apply") {
		fmt.Fprintln(stderr, "idp aws-stack: want plan or apply")
		return exitUsage
	}
	mode := args[0]
	fs := flag.NewFlagSet("idp aws-stack "+mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var cfg awsstack.Config
	fs.StringVar(&cfg.Stack, "stack", "", "the AWS stack, as idp diff names it (required)")
	account := fs.String("account", "", "the 12-digit floci account the stack runs as, from idp stacks (required)")
	fs.StringVar(&cfg.New, "new", "", "the fresh render's root (required)")
	fs.StringVar(&cfg.Wet, "wet", "", "wet's rendered/ root; it may be missing (required)")
	fs.StringVar(&cfg.Work, "work", "", "a fresh scratch directory outside both renders (required)")
	fs.StringVar(&cfg.Endpoint, "endpoint", "", "floci's endpoint (required)")
	tofu := fs.String("tofu", "tofu", "the OpenTofu binary")
	outDir := fs.String("out-dir", "", "plan: write the reduced plan into this directory, as idp plan-reduce does (required)")
	gatedFile := fs.String("fingerprint", "", "apply: the fingerprint the gate decided on (required)")
	appliedDir := fs.String("applied-dir", "", "apply: after a successful apply, mark the stack applied in this directory (required)")
	if err := fs.Parse(args[1:]); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp aws-stack %s: unexpected argument %q\n", mode, fs.Arg(0))
		return exitUsage
	}
	if cfg.Stack == "" || *account == "" || cfg.New == "" || cfg.Wet == "" || cfg.Work == "" || cfg.Endpoint == "" {
		fmt.Fprintf(stderr, "idp aws-stack %s: --stack, --account, --new, --wet, --work and --endpoint are required\n", mode)
		return exitUsage
	}
	if !accountID.MatchString(*account) {
		fmt.Fprintf(stderr, "idp aws-stack %s: --account %q is not a 12-digit account ID\n", mode, *account)
		return exitUsage
	}
	if mode == "plan" && *outDir == "" {
		fmt.Fprintln(stderr, "idp aws-stack plan: --out-dir is required")
		return exitUsage
	}
	if mode == "apply" && (*gatedFile == "" || *appliedDir == "") {
		fmt.Fprintln(stderr, "idp aws-stack apply: --fingerprint and --applied-dir are required")
		return exitUsage
	}
	for _, name := range secretNames {
		if env(name) != "" {
			fmt.Fprintf(stderr, "idp aws-stack: %s is set: a job that runs floci shares the runner's Docker socket, so it holds no secret (ADR-0031)\n", name)
			return exitError
		}
	}
	run := newTofu(*tofu, awsstack.Env(os.Environ(), *account), stderr)
	if mode == "plan" {
		return awsStackPlan(cfg, run, *outDir, stdout, stderr, env)
	}
	return awsStackApply(cfg, run, *gatedFile, *appliedDir, stdout, stderr, env)
}

func awsStackPlan(cfg awsstack.Config, run awsstack.Runner, outDir string, stdout, stderr io.Writer, env Env) int {
	res, err := awsstack.Plan(cfg, run)
	if err != nil {
		return fail(stderr, err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(planFileIn(outDir, cfg.Stack), res.Reduced, 0o644); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, countsLine(res.Plan))
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "changes", strconv.Itoa(res.Plan.Counts().Total())); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

func awsStackApply(cfg awsstack.Config, run awsstack.Runner, gatedFile, appliedDir string, stdout, stderr io.Writer, env Env) int {
	f, err := os.Open(gatedFile)
	if err != nil {
		return fail(stderr, err)
	}
	gated, err := plan.ReadFingerprint(f)
	f.Close()
	if err != nil {
		return fail(stderr, err)
	}
	heading := "### `" + cfg.Stack + "`\n\n"
	res, err := awsstack.Apply(cfg, run, gated)
	if err != nil {
		lead := "Nothing was applied:"
		if errors.Is(err, awsstack.ErrApplyFailed) {
			lead = "The apply failed partway, so part of the plan may have been applied. The stack's render stays old in wet, so the next reconcile re-plans it:"
		}
		if err := actions.AddSummary(env("GITHUB_STEP_SUMMARY"), heading+lead+"\n\n```\n"+err.Error()+"\n```\n"); err != nil {
			fmt.Fprintln(stderr, "idp:", err)
		}
		return fail(stderr, err)
	}
	if err := os.MkdirAll(appliedDir, 0o755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(appliedDir, wetdiff.Escape(cfg.Stack)), []byte(cfg.Stack+"\n"), 0o644); err != nil {
		return fail(stderr, err)
	}
	line := countsLine(res.Plan)
	fmt.Fprintln(stdout, line+": applied")
	var b strings.Builder
	b.WriteString(heading + "The re-plan equals the gated fingerprint, and it applied: " + strings.TrimPrefix(line, cfg.Stack+": ") + ".\n")
	if len(res.Plan.Changes) > 0 {
		b.WriteString("\n")
	}
	for _, c := range res.Plan.Changes {
		fmt.Fprintf(&b, "- %s `%s`\n", c.Action, c.Address)
	}
	if err := actions.AddSummary(env("GITHUB_STEP_SUMMARY"), b.String()); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}
