package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
	"github.com/jellalshadows-idp/idp-engine/internal/ghops"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

const issueUsage = `Usage:
  idp issue open  --repo OWNER/REPO --label LABEL --title TITLE --body-file FILE
  idp issue close --repo OWNER/REPO --label LABEL --title TITLE [--comment TEXT]
`

// githubRepo builds the repository client every conversation command needs,
// or returns a usage message.
func githubRepo(full string, env Env) (ghops.Repo, string) {
	tok := token(env)
	if tok == "" {
		return ghops.Repo{}, "set GH_TOKEN or GITHUB_TOKEN"
	}
	repo, err := ghops.ParseRepo(ghapi.New(apiBase(env), tok), full)
	if err != nil {
		return ghops.Repo{}, err.Error()
	}
	return repo, ""
}

func runComment(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp comment", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoFlag := fs.String("repo", "", "repository, OWNER/REPO")
	pr := fs.Int("pr", 0, "pull request number")
	bodyFile := fs.String("body-file", "", "comment Markdown; must contain the marker")
	marker := fs.String("marker", plan.CommentMarker, "marker that identifies the sticky comment")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 || *repoFlag == "" || *pr <= 0 || *bodyFile == "" {
		fmt.Fprintln(stderr, "idp comment: --repo, --pr and --body-file are required")
		return exitUsage
	}
	repo, msg := githubRepo(*repoFlag, env)
	if msg != "" {
		fmt.Fprintln(stderr, "idp comment:", msg)
		return exitUsage
	}
	body, err := os.ReadFile(*bodyFile)
	if err != nil {
		return fail(stderr, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	id, created, err := repo.UpsertBotComment(ctx, *pr, *marker, string(body))
	if err != nil {
		return fail(stderr, err)
	}
	verb := "updated"
	if created {
		verb = "created"
	}
	fmt.Fprintf(stdout, "comment: %s %d on #%d\n", verb, id, *pr)
	return exitOK
}

func runIssue(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 || (args[0] != "open" && args[0] != "close") {
		fmt.Fprint(stderr, issueUsage)
		return exitUsage
	}
	mode := args[0]
	fs := flag.NewFlagSet("idp issue "+mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoFlag := fs.String("repo", "", "repository, OWNER/REPO")
	label := fs.String("label", "", "label that identifies the issue")
	title := fs.String("title", "", "issue title")
	bodyFile := fs.String("body-file", "", "issue body (open only)")
	comment := fs.String("comment", "", "comment to leave when closing (close only)")
	if err := fs.Parse(args[1:]); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 || *repoFlag == "" || *label == "" || *title == "" || (mode == "open" && *bodyFile == "") {
		fmt.Fprint(stderr, issueUsage)
		return exitUsage
	}
	repo, msg := githubRepo(*repoFlag, env)
	if msg != "" {
		fmt.Fprintln(stderr, "idp issue:", msg)
		return exitUsage
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if mode == "close" {
		n, closed, err := repo.CloseIssue(ctx, *label, *title, *comment)
		if err != nil {
			return fail(stderr, err)
		}
		if closed {
			fmt.Fprintf(stdout, "issue: closed #%d\n", n)
		} else {
			fmt.Fprintln(stdout, "issue: none open")
		}
		return exitOK
	}
	body, err := os.ReadFile(*bodyFile)
	if err != nil {
		return fail(stderr, err)
	}
	n, created, err := repo.OpenIssue(ctx, *label, *title, string(body))
	if err != nil {
		return fail(stderr, err)
	}
	verb := "updated"
	if created {
		verb = "opened"
	}
	fmt.Fprintf(stdout, "issue: %s #%d\n", verb, n)
	return exitOK
}
