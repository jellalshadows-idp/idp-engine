package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

func runDiff(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	newDir := fs.String("new", "", "fresh render (the --out of idp render)")
	wetDir := fs.String("wet", "", "rendered/ of the wet branch checkout (may not exist yet)")
	all := fs.Bool("all", false, "treat every stack as affected (manual reconcile)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp diff: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	var missing []string
	if *newDir == "" {
		missing = append(missing, "--new")
	}
	if *wetDir == "" {
		missing = append(missing, "--wet")
	}
	if len(missing) > 0 {
		fmt.Fprintf(stderr, "idp diff: missing %s\n", strings.Join(missing, ", "))
		return exitUsage
	}
	if _, err := os.Stat(filepath.Join(*newDir, render.Marker)); err != nil {
		return fail(stderr, fmt.Errorf("%s is not an idp render (no %s): pass the --out directory of idp render", *newDir, render.Marker))
	}
	res, err := wetdiff.Diff(*newDir, *wetDir, *all)
	if err != nil {
		return fail(stderr, err)
	}
	for _, s := range res.Stacks {
		fmt.Fprintf(stdout, "%s: %s\n", s.Path, s.Status)
	}
	fmt.Fprintf(stdout, "diff: %d affected stack(s)\n", len(res.Affected))
	affected, err := json.Marshal(res.Affected)
	if err != nil {
		return fail(stderr, err)
	}
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "affected", string(affected)); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}
