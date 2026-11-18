package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

// planFileIn is where a stack's reduced plan lives in a plan directory: idp
// plan-reduce and idp aws-stack plan write it there, and idp plan-summary
// --plan-dir reads it (spec §6.1 step 5).
func planFileIn(dir, stack string) string { return filepath.Join(dir, wetdiff.Escape(stack)+".json") }

// countsLine is a stack's one-line summary: +create ~update -delete ±replace.
func countsLine(p *plan.Plan) string {
	c := p.Counts()
	return fmt.Sprintf("%s: +%d ~%d -%d ±%d", p.Stack, c.Create, c.Update, c.Delete, c.Replace)
}

func runPlanReduce(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp plan-reduce", flag.ContinueOnError)
	fs.SetOutput(stderr)
	stack := fs.String("stack", "", "the stack the plan is for, as idp diff names it (required)")
	planFile := fs.String("plan", "", "the tofu show -json output of the stack's plan (required)")
	outDir := fs.String("out-dir", "", "write the reduced plan into this directory, as <stack with each / as .>.json (required)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp plan-reduce: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	if *stack == "" || *planFile == "" || *outDir == "" {
		fmt.Fprintln(stderr, "idp plan-reduce: --stack, --plan and --out-dir are required")
		return exitUsage
	}
	if !wetdiff.Reconcilable(*stack) {
		fmt.Fprintf(stderr, "idp plan-reduce: --stack %q is not a stack this engine version reconciles\n", *stack)
		return exitUsage
	}
	f, err := os.Open(*planFile)
	if err != nil {
		return fail(stderr, err)
	}
	defer f.Close()
	p, reduced, err := plan.Reduce(*stack, f)
	if err != nil {
		return fail(stderr, err)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(planFileIn(*outDir, *stack), reduced, 0o644); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, countsLine(p))
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "changes", strconv.Itoa(p.Counts().Total())); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}
