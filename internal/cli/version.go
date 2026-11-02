package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

// runVersion prints the build's version: the release tag of a released binary,
// the engine commit of a pipeline's source build, or "dev" (ADR-0028).
func runVersion(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("idp version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp version: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	fmt.Fprintln(stdout, version.Version)
	return exitOK
}
