package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

// planFlags collects repeated --plan STACK=FILE values.
type planFlags []string

func (p *planFlags) String() string { return strings.Join(*p, ",") }

func (p *planFlags) Set(v string) error {
	stack, file, ok := strings.Cut(v, "=")
	if !ok || stack == "" || file == "" {
		return fmt.Errorf("want STACK=FILE, got %q", v)
	}
	*p = append(*p, v)
	return nil
}

func runPlanSummary(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp plan-summary", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var plans planFlags
	fs.Var(&plans, "plan", "STACK=FILE: the tofu show -json output of one stack (repeatable)")
	headSHA := fs.String("head-sha", "", "commit the plans are for (40 hex characters)")
	runURL := fs.String("run-url", "", "link to the workflow run, shown in the comment")
	commentOut := fs.String("comment-out", "", "write the PR comment Markdown here")
	fpOut := fs.String("fingerprint-out", "", "write the fingerprint JSON here")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp plan-summary: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	if *commentOut == "" || *fpOut == "" || *headSHA == "" {
		fmt.Fprintln(stderr, "idp plan-summary: --head-sha, --comment-out and --fingerprint-out are required")
		return exitUsage
	}
	if !plan.ValidSHA(*headSHA) {
		fmt.Fprintf(stderr, "idp plan-summary: --head-sha %q is not a full commit SHA\n", *headSHA)
		return exitUsage
	}
	seen := map[string]bool{}
	for _, entry := range plans {
		stack, _, _ := strings.Cut(entry, "=")
		if seen[stack] {
			fmt.Fprintf(stderr, "idp plan-summary: stack %q given twice\n", stack)
			return exitUsage
		}
		seen[stack] = true
	}
	parsed := []*plan.Plan{}
	for _, entry := range plans {
		stack, file, _ := strings.Cut(entry, "=")
		p, err := parsePlanFile(stack, file)
		if err != nil {
			return fail(stderr, err)
		}
		parsed = append(parsed, p)
	}
	body, err := plan.Comment(parsed, *headSHA, *runURL)
	if err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(*commentOut, []byte(body), 0o644); err != nil {
		return fail(stderr, err)
	}
	fp := plan.FingerprintOf(parsed...)
	raw, err := json.Marshal(fp)
	if err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(*fpOut, append(raw, '\n'), 0o644); err != nil {
		return fail(stderr, err)
	}
	total := 0
	for _, p := range parsed {
		c := p.Counts()
		total += c.Total()
		fmt.Fprintf(stdout, "%s: +%d ~%d -%d ±%d\n", p.Stack, c.Create, c.Update, c.Delete, c.Replace)
	}
	destructive := len(fp.Destructive()) > 0
	fmt.Fprintf(stdout, "plan-summary: %d change(s), destructive: %t\n", total, destructive)
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "changes", strconv.Itoa(total)); err != nil {
		return fail(stderr, err)
	}
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "destructive", strconv.FormatBool(destructive)); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

func parsePlanFile(stack, file string) (*plan.Plan, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return plan.Parse(stack, f)
}
