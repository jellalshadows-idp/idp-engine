package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/wetpush"
)

func runWetPush(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp wet-push", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoFlag := fs.String("repo", "", "claims repository, OWNER/REPO")
	branch := fs.String("branch", "wet", "branch to commit to")
	root := fs.String("root", "", "local directory laid out like the branch")
	message := fs.String("message", "", "commit message")
	var paths []string
	fs.Func("path", "slash path under --root to sync, file or directory (repeatable)", func(v string) error {
		if v == "" {
			return fmt.Errorf("empty --path")
		}
		paths = append(paths, v)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 || *repoFlag == "" || *root == "" || *message == "" || len(paths) == 0 {
		fmt.Fprintln(stderr, "idp wet-push: --repo, --root, --message and at least one --path are required")
		return exitUsage
	}
	repo, msg := githubRepo(*repoFlag, env)
	if msg != "" {
		fmt.Fprintln(stderr, "idp wet-push:", msg)
		return exitUsage
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	p := wetpush.Pusher{API: repo.API, Owner: repo.Owner, Repo: repo.Name, Branch: *branch}
	res, err := p.Sync(ctx, *root, paths, *message)
	if err != nil {
		return fail(stderr, err)
	}
	if res.Commit == "" {
		fmt.Fprintln(stdout, "wet-push: no changes")
	} else {
		fmt.Fprintf(stdout, "wet-push: %s (%d changed, %d deleted)\n", res.Commit, res.Changed, res.Deleted)
	}
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "commit", res.Commit); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}
