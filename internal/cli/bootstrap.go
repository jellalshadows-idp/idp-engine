package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/bootstrap"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

const bootstrapUsage = `Usage:
  idp bootstrap app   --org ORG --role reader|writer [--out-dir DIR] [--listen ADDR]
  idp bootstrap apply --org ORG --claims-repo REPO --approver LOGIN --reader FILE --writer FILE --passphrase-file FILE
  idp bootstrap check --org ORG --claims-repo REPO --approver LOGIN --reader FILE --writer FILE

FILE for --reader/--writer is the <slug>.json written by "idp bootstrap app".
apply and check read a token from GH_TOKEN or GITHUB_TOKEN (e.g. GH_TOKEN=$(gh auth token)).
`

func runBootstrap(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, bootstrapUsage)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	switch args[0] {
	case "app":
		return runBootstrapApp(ctx, args[1:], stdout, stderr, env)
	case "apply":
		return runBootstrapRepo(ctx, "apply", args[1:], stdout, stderr, env)
	case "check":
		return runBootstrapRepo(ctx, "check", args[1:], stdout, stderr, env)
	default:
		fmt.Fprintf(stderr, "idp bootstrap: unknown subcommand %q\n\n%s", args[0], bootstrapUsage)
		return 2
	}
}

func apiBase(env Env) string {
	if v := env("IDP_GITHUB_API"); v != "" {
		return v
	}
	return ghapi.DefaultBaseURL
}

func token(env Env) string {
	if v := env("GH_TOKEN"); v != "" {
		return v
	}
	return env("GITHUB_TOKEN")
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "idp:", err)
	return 1
}

func runBootstrapRepo(ctx context.Context, mode string, args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp bootstrap "+mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	org := fs.String("org", "", "GitHub organization")
	repo := fs.String("claims-repo", "", "claims repository name")
	approver := fs.String("approver", "", "login of the platform admin who approves runs")
	readerFile := fs.String("reader", "", "reader app <slug>.json")
	writerFile := fs.String("writer", "", "writer app <slug>.json")
	passFile := fs.String("passphrase-file", "", "file with the OpenTofu state passphrase (apply only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	required := []struct{ flag, value string }{
		{"--org", *org}, {"--claims-repo", *repo}, {"--approver", *approver},
		{"--reader", *readerFile}, {"--writer", *writerFile},
	}
	if mode == "apply" {
		required = append(required, struct{ flag, value string }{"--passphrase-file", *passFile})
	}
	var missing []string
	for _, r := range required {
		if r.value == "" {
			missing = append(missing, r.flag)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(stderr, "idp bootstrap %s: missing %s\n", mode, strings.Join(missing, ", "))
		return 2
	}
	tok := token(env)
	if tok == "" {
		fmt.Fprintln(stderr, "idp bootstrap: set GH_TOKEN or GITHUB_TOKEN (e.g. GH_TOKEN=$(gh auth token))")
		return 2
	}

	api := ghapi.New(apiBase(env), tok)
	withKeys := mode == "apply"
	cfg := bootstrap.Config{Org: *org, ClaimsRepo: *repo}
	var err error
	if cfg.Reader, err = bootstrap.LoadAppCredentials(*readerFile, withKeys); err != nil {
		return fail(stderr, err)
	}
	if cfg.Writer, err = bootstrap.LoadAppCredentials(*writerFile, withKeys); err != nil {
		return fail(stderr, err)
	}
	if cfg.ApproverID, err = bootstrap.UserID(ctx, api, *approver); err != nil {
		return fail(stderr, err)
	}

	if mode == "apply" {
		if cfg.Passphrase, err = bootstrap.ReadPassphrase(*passFile); err != nil {
			return fail(stderr, err)
		}
		if err := cfg.ValidateApply(); err != nil {
			return fail(stderr, err)
		}
		b := &bootstrap.Bootstrapper{API: api, Cfg: cfg, Log: stdout}
		if err := b.Apply(ctx); err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintln(stdout, "bootstrap apply: done")
		return 0
	}

	if err := cfg.ValidateCheck(); err != nil {
		return fail(stderr, err)
	}
	b := &bootstrap.Bootstrapper{API: api, Cfg: cfg, Log: stdout}
	found, err := b.Check(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	for _, f := range found {
		fmt.Fprintln(stdout, f)
	}
	if len(found) > 0 {
		fmt.Fprintf(stdout, "bootstrap check: %d finding(s)\n", len(found))
		return 1
	}
	fmt.Fprintln(stdout, "bootstrap check: no drift")
	return 0
}

func runBootstrapApp(ctx context.Context, args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp bootstrap app", flag.ContinueOnError)
	fs.SetOutput(stderr)
	org := fs.String("org", "", "GitHub organization that will own the App")
	role := fs.String("role", "", "reader or writer")
	outDir := fs.String("out-dir", "", "where to write <slug>.json and <slug>.pem (default ~/.idp/apps)")
	listen := fs.String("listen", "127.0.0.1:0", "local address for the callback server")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *org == "" || (*role != string(bootstrap.RoleReader) && *role != string(bootstrap.RoleWriter)) {
		fmt.Fprintln(stderr, "idp bootstrap app: --org and --role reader|writer are required")
		return 2
	}
	dir := *outDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fail(stderr, err)
		}
		dir = filepath.Join(home, ".idp", "apps")
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return fail(stderr, err)
	}
	flow := &bootstrap.AppFlow{
		Org: *org, Role: bootstrap.AppRole(*role), API: ghapi.New(apiBase(env), ""),
		OutDir: dir, Log: stdout, GitHubWeb: "https://github.com",
	}
	creds, err := flow.Run(ctx, ln)
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "Created %s (id %d). Credentials in %s\n", creds.Slug, creds.ID, dir)
	fmt.Fprintf(stdout, "Now install it on %s with access to All repositories: https://github.com/apps/%s/installations/new\n", *org, creds.Slug)
	return 0
}
