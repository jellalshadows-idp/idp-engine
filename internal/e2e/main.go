package e2e

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// Main runs the harness:
//
//	idp-e2e --repo OWNER/NAME --engine-sha SHA --run ID [--templates DIR] [--owner LOGIN] [--member LOGIN]
//
// with the bot and approver tokens in E2E_BOT_TOKEN and E2E_APPROVER_TOKEN.
// It exits 0 when every step passed, 1 when one failed, 2 on a usage error.
func Main(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("idp-e2e", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", "", "claims repository, OWNER/NAME")
	engineSHA := fs.String("engine-sha", "", "the idp-engine commit under test")
	run := fs.String("run", "", "unique id of this run (the workflow run id)")
	templates := fs.String("templates", "examples/claims-repo/.github/workflows", "directory of the caller templates")
	owner := fs.String("owner", "jellalshadows", "login of the harness team's maintainer")
	member := fs.String("member", "adrian-da-silva", "login of the harness team's member")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	org, name, ok := strings.Cut(*repo, "/")
	if !ok || org == "" || name == "" || *engineSHA == "" || *run == "" {
		fmt.Fprintln(stderr, "idp-e2e: --repo OWNER/NAME, --engine-sha and --run are required")
		return 2
	}
	if !shaPattern.MatchString(*engineSHA) {
		fmt.Fprintln(stderr, "idp-e2e: --engine-sha must be a 40-character commit SHA")
		return 2
	}
	bot, approver := getenv("E2E_BOT_TOKEN"), getenv("E2E_APPROVER_TOKEN")
	if bot == "" || approver == "" {
		fmt.Fprintln(stderr, "idp-e2e: E2E_BOT_TOKEN and E2E_APPROVER_TOKEN must be set")
		return 2
	}
	tpl, err := readTemplates(*templates)
	if err != nil {
		fmt.Fprintln(stderr, "idp-e2e:", err)
		return 1
	}
	base := getenv("IDP_GITHUB_API")
	if base == "" {
		base = ghapi.DefaultBaseURL
	}
	s := &Scenario{
		F:     GitHub{Bot: ghapi.New(base, bot), Approver: ghapi.New(base, approver), Owner: org, Repo: name},
		Clock: realClock{}, Log: stdout, Org: org, Run: *run, EngineSHA: *engineSHA,
		Templates: tpl, Owner: *owner, Member: *member, Timeouts: DefaultTimeouts,
	}
	if err := s.Execute(context.Background()); err != nil {
		fmt.Fprintln(stderr, "idp-e2e:", err)
		return 1
	}
	return 0
}

func readTemplates(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, n := range []string{"drift.yaml", "pr.yaml", "reconcile.yaml", "recover.yaml"} {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, fmt.Errorf("read template: %w", err)
		}
		out[n] = b
	}
	return out, nil
}
