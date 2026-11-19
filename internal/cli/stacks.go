package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

// maxMatrix is the number of jobs GitHub Actions runs from one matrix.
const maxMatrix = 256

// placeholderAccount runs the orphan baseline of an environment that the claims
// removed and that had no Component stack: nothing names its account, and a
// baseline's plan is the same in any account (its configuration names none).
const placeholderAccount = "000000000000"

// awsJob is one entry of the plan-aws and apply-aws matrix (spec §6.1 step 4).
type awsJob struct {
	Stack    string `json:"stack"`
	Artifact string `json:"artifact"` // wetdiff.Escape(Stack), for artifact names
	Account  string `json:"account"`  // the floci account the job runs as (ADR-0004)
}

// runStacks splits idp diff's affected stacks into the github stack and the AWS
// matrix, and fails on any stack this engine version does not reconcile.
func runStacks(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp stacks", flag.ContinueOnError)
	fs.SetOutput(stderr)
	affectedJSON := fs.String("affected", "", "JSON array of the affected stacks, as written by idp diff (required)")
	dir := fs.String("dir", ".", "claims repo root, whose config/platform.yaml gives each environment's account")
	wet := fs.String("wet", "", "the wet branch's rendered/ tree, whose Component stacks name the account of an environment the claims removed (required)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp stacks: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	if *affectedJSON == "" || *wet == "" {
		fmt.Fprintln(stderr, "idp stacks: --affected and --wet are required")
		return exitUsage
	}
	var affected []string
	if err := json.Unmarshal([]byte(*affectedJSON), &affected); err != nil || affected == nil {
		fmt.Fprintf(stderr, "idp stacks: --affected must be a JSON array of stack names, got %q\n", *affectedJSON)
		return exitUsage
	}
	github := false
	var aws, other []string
	listed := map[string]bool{}
	for _, s := range affected {
		if s == "" || listed[s] {
			fmt.Fprintf(stderr, "idp stacks: --affected has an empty or duplicate stack name %q\n", s)
			return exitUsage
		}
		listed[s] = true
		switch {
		case s == "github":
			github = true
		case wetdiff.Reconcilable(s):
			aws = append(aws, s)
		default:
			other = append(other, s)
		}
	}
	if len(other) > 0 {
		fmt.Fprintf(stderr, "idp stacks: this engine version reconciles only github, aws/<env>/_baseline and aws/<env>/components/<name>; also affected: %s\n", strings.Join(other, ", "))
		return exitError
	}
	if len(aws) > maxMatrix {
		fmt.Fprintf(stderr, "idp stacks: %d AWS stacks are affected; a matrix runs at most %d jobs (spec §11 risk 3): split the change\n", len(aws), maxMatrix)
		return exitError
	}
	jobs := []awsJob{}
	if len(aws) > 0 {
		m, code := loadClaims(*dir, stdout, stderr, env)
		if code != exitOK {
			return code
		}
		sort.Strings(aws)
		for _, s := range aws {
			envName := strings.Split(s, "/")[1]
			account := ""
			if e, ok := m.Platform.Environments[envName]; ok && e.AWS != nil {
				account = e.AWS.AccountID
			} else {
				a, err := wetAccount(*wet, envName)
				if err != nil {
					fmt.Fprintf(stderr, "idp stacks: %v\n", err)
					return exitError
				}
				account = a
			}
			jobs = append(jobs, awsJob{Stack: s, Artifact: wetdiff.Escape(s), Account: account})
			fmt.Fprintf(stdout, "%s: account %s\n", s, account)
		}
	}
	matrix, err := json.Marshal(jobs)
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "stacks: github %t, %d AWS stack(s)\n", github, len(jobs))
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "github", fmt.Sprint(github)); err != nil {
		return fail(stderr, err)
	}
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "aws", string(matrix)); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

// oidcAccount reads the account out of a Component stack's oidc_provider_arn,
// naming.OIDCProviderARN(accountId) (spec §5.5).
var oidcAccount = regexp.MustCompile(`^arn:aws:iam::([0-9]{12}):oidc-provider/`)

// wetAccount is the account of an environment the claims no longer give one:
// the account its wet Component stacks name, or placeholderAccount when it has
// no Component stack. A Component stack that names no account is an error: a
// replay in the wrong account would plan a create instead of the real change.
func wetAccount(wet, envName string) (string, error) {
	stacks, err := filepath.Glob(filepath.Join(wet, "aws", envName, "components", "*", wetdiff.StackFile))
	if err != nil {
		return "", err
	}
	sort.Strings(stacks)
	for _, p := range stacks {
		raw, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		var doc struct {
			Module struct {
				Component struct {
					OIDCProviderARN string `json:"oidc_provider_arn"`
				} `json:"component"`
			} `json:"module"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return "", fmt.Errorf("%s: %w", p, err)
		}
		arn := doc.Module.Component.OIDCProviderARN
		m := oidcAccount.FindStringSubmatch(arn)
		if m == nil {
			return "", fmt.Errorf("%s: module.component.oidc_provider_arn %q names no account (spec §5.5)", p, arn)
		}
		return m[1], nil
	}
	return placeholderAccount, nil
}
