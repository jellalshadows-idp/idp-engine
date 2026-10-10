package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
	"github.com/jellalshadows-idp/idp-engine/internal/ghops"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

func runGate(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp gate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoFlag := fs.String("repo", "", "claims repository, OWNER/REPO")
	sha := fs.String("sha", "", "commit being reconciled (40 hex characters)")
	fpFile := fs.String("fingerprint", "", "fingerprint JSON written by idp plan-summary")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 || *repoFlag == "" || *sha == "" || *fpFile == "" {
		fmt.Fprintln(stderr, "idp gate: --repo, --sha and --fingerprint are required")
		return exitUsage
	}
	if !plan.ValidSHA(*sha) {
		fmt.Fprintf(stderr, "idp gate: --sha %q is not a full commit SHA\n", *sha)
		return exitUsage
	}
	repo, err := ghops.ParseRepo(nil, *repoFlag)
	if err != nil {
		fmt.Fprintln(stderr, "idp gate:", err)
		return exitUsage
	}
	f, err := os.Open(*fpFile)
	if err != nil {
		return fail(stderr, err)
	}
	current, err := plan.ReadFingerprint(f)
	f.Close()
	if err != nil {
		return fail(stderr, err)
	}
	var pr *plan.PRPlan
	why := ""
	if plan.NeedsPR(current) {
		tok := token(env)
		if tok == "" {
			fmt.Fprintln(stderr, "idp gate: set GH_TOKEN or GITHUB_TOKEN")
			return exitUsage
		}
		repo.API = ghapi.New(apiBase(env), tok)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if pr, why, err = trustedPRPlan(ctx, repo, *sha); err != nil {
			return fail(stderr, err)
		}
	}
	v := plan.Decide(current, pr, why)
	fmt.Fprintf(stdout, "gate: %s (%s)\n", v.Decision, v.Reason)
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "decision", string(v.Decision)); err != nil {
		return fail(stderr, err)
	}
	if err := actions.AddSummary(env("GITHUB_STEP_SUMMARY"), fmt.Sprintf("**Gate:** `%s` — %s", v.Decision, v.Reason)); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

// trustedPRPlan finds the fingerprint a reviewer saw (ADR-0018): the newest
// plan comment by the Actions bot on the PR merged as sha, made for that PR's
// final head commit. A string explains why there is none.
func trustedPRPlan(ctx context.Context, repo ghops.Repo, sha string) (*plan.PRPlan, string, error) {
	pr, err := repo.MergedPullForCommit(ctx, sha)
	if err != nil {
		return nil, "", err
	}
	if pr == nil {
		return nil, fmt.Sprintf("no merged pull request has %s as its merge commit", sha[:7]), nil
	}
	c, err := repo.FindBotComment(ctx, pr.Number, plan.CommentMarker)
	if err != nil {
		return nil, "", err
	}
	if c == nil {
		return nil, fmt.Sprintf("PR #%d has no plan comment from %s", pr.Number, ghops.ActionsBot), nil
	}
	head, fp, ok, err := plan.DecodeMarker(c.Body)
	switch {
	case err != nil:
		return nil, fmt.Sprintf("the plan comment on PR #%d has an unreadable fingerprint: %v", pr.Number, err), nil
	case !ok:
		return nil, fmt.Sprintf("the plan comment on PR #%d has no fingerprint", pr.Number), nil
	case head != pr.Head.SHA:
		return nil, fmt.Sprintf("the plan comment on PR #%d is for %s, but the PR merged %s", pr.Number, head[:7], shortSHA(pr.Head.SHA)), nil
	}
	return &plan.PRPlan{Number: pr.Number, Fingerprint: fp}, "", nil
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
