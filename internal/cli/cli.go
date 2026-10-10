// Package cli implements the idp command-line interface.
package cli

import (
	"fmt"
	"io"
)

const usage = `idp — IDP engine CLI

Usage:
  idp <command> [flags]

Commands:
  validate        Validate a claims repo (schema + semantic checks)
  render          Render a claims repo into OpenTofu stacks
  diff            List the stacks whose render differs from the wet branch
  plan-summary    Summarize OpenTofu plans as a PR comment and a fingerprint
  gate            Decide whether a reconcile applies automatically or waits for approval
  comment         Create or update the sticky plan comment on a pull request
  issue           Open or close a labelled issue (drift, failed wet pushes)
  bootstrap      Create or verify the protections an org needs before the IDP runs
  help            Show this help

Exit codes: 0 ok, 1 the command failed, 2 usage error,
3 the command found problems (invalid claims, drift).
`

// Env looks up an environment variable: os.Getenv in production, a map in tests.
type Env func(key string) string

// Run executes the CLI and returns the process exit code (ADR-0016).
func Run(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	case "validate":
		return runValidate(args[1:], stdout, stderr, env)
	case "render":
		return runRender(args[1:], stdout, stderr, env)
	case "diff":
		return runDiff(args[1:], stdout, stderr, env)
	case "plan-summary":
		return runPlanSummary(args[1:], stdout, stderr, env)
	case "gate":
		return runGate(args[1:], stdout, stderr, env)
	case "comment":
		return runComment(args[1:], stdout, stderr, env)
	case "issue":
		return runIssue(args[1:], stdout, stderr, env)
	case "bootstrap":
		return runBootstrap(args[1:], stdout, stderr, env)
	default:
		fmt.Fprintf(stderr, "idp: unknown command %q\n\n%s", args[0], usage)
		return exitUsage
	}
}
