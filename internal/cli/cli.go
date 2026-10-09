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
  bootstrap   Create or verify the protections an org needs before the IDP runs
  help        Show this help
`

// Env looks up an environment variable: os.Getenv in production, a map in tests.
type Env func(key string) string

// Run executes the CLI and returns the process exit code
// (0 ok, 1 runtime failure or drift, 2 usage error).
func Run(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "bootstrap":
		return runBootstrap(args[1:], stdout, stderr, env)
	default:
		fmt.Fprintf(stderr, "idp: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
