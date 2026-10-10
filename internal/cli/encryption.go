package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/bootstrap"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
)

func runEncryptionEnv(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp encryption-env", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp encryption-env: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	envFile := env("GITHUB_ENV")
	if envFile == "" {
		fmt.Fprintln(stderr, "idp encryption-env: GITHUB_ENV is not set; run it as a GitHub Actions step")
		return exitUsage
	}
	pass := env("IDP_STATE_PASSPHRASE")
	if err := bootstrap.ValidatePassphrase(pass); err != nil {
		problem := fmt.Sprintf("is invalid (%v)", err)
		if pass == "" {
			problem = "is empty or missing"
		}
		msg := fmt.Sprintf("IDP_STATE_PASSPHRASE %s: pass the repo secret to the workflow as IDP_STATE_PASSPHRASE; see docs/runbooks/bootstrap.md", problem)
		if env("GITHUB_ACTIONS") == "true" {
			fmt.Fprintln(stdout, actions.ErrorAnnotation("", 0, "State passphrase", msg))
		}
		fmt.Fprintln(stderr, "idp:", msg)
		return exitError
	}
	if env("GITHUB_ACTIONS") == "true" {
		// GITHUB_ENV values show up in later steps' log headers, and the runner
		// masks only the exact secret, so register the escaped form too.
		fmt.Fprintln(stdout, actions.Mask(pass))
		if escaped := render.EscapeHCLString(pass); escaped != pass {
			fmt.Fprintln(stdout, actions.Mask(escaped))
		}
	}
	if err := actions.SetEnv(envFile, "TF_ENCRYPTION", render.EncryptionConfig(pass)); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, "encryption-env: TF_ENCRYPTION is set for the next steps")
	return exitOK
}
