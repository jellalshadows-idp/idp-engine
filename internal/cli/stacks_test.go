package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// awsFixture is a claims repo with dev (000000000001) and pro (000000000003).
var awsFixture = filepath.Join("..", "render", "testdata", "aws-registry")

// runStacksCmd runs idp stacks and returns its exit code, stdout, stderr and
// GITHUB_OUTPUT.
func runStacksCmd(t *testing.T, args ...string) (int, string, string, string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "github-output")
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"stacks"}, args...), &stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	got, err := os.ReadFile(out)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String(), string(got)
}

// TestStacksCommand: the AWS stacks become the plan-aws and apply-aws matrix,
// each with its escaped name and its environment's account: from the claims,
// or, for an environment the claims removed, from its wet Component stacks.
func TestStacksCommand(t *testing.T) {
	wet := t.TempDir()
	writeFileAll(t, filepath.Join(wet, "aws", "staging", "components", "api", "main.tf.json"),
		`{"module":{"component":{"oidc_provider_arn":"arn:aws:iam::000000000002:oidc-provider/token.actions.githubusercontent.com"}}}`)
	affected := `["aws/dev/_baseline","aws/dev/components/api","aws/staging/_baseline","aws/staging/components/api","github"]`
	code, stdout, stderr, output := runStacksCmd(t, "--affected", affected, "--dir", awsFixture, "--wet", wet)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	wantOut := "aws/dev/_baseline: account 000000000001\n" +
		"aws/dev/components/api: account 000000000001\n" +
		"aws/staging/_baseline: account 000000000002\n" +
		"aws/staging/components/api: account 000000000002\n" +
		"stacks: github true, 4 AWS stack(s)\n"
	if stdout != wantOut {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout, wantOut)
	}
	wantOutput := "github=true\n" + `aws=[{"stack":"aws/dev/_baseline","artifact":"aws.dev._baseline","account":"000000000001"},` +
		`{"stack":"aws/dev/components/api","artifact":"aws.dev.components.api","account":"000000000001"},` +
		`{"stack":"aws/staging/_baseline","artifact":"aws.staging._baseline","account":"000000000002"},` +
		`{"stack":"aws/staging/components/api","artifact":"aws.staging.components.api","account":"000000000002"}]` + "\n"
	if output != wantOutput {
		t.Errorf("GITHUB_OUTPUT =\n%s\nwant\n%s", output, wantOutput)
	}
}

// TestStacksPlaceholderAccount: a removed environment that had only its
// baseline names no account anywhere; its baseline's plan is the same in any
// account, so it runs as 000000000000.
func TestStacksPlaceholderAccount(t *testing.T) {
	code, stdout, stderr, _ := runStacksCmd(t, "--affected", `["aws/qa/_baseline"]`, "--dir", awsFixture, "--wet", t.TempDir())
	if code != 0 || stdout != "aws/qa/_baseline: account 000000000000\nstacks: github false, 1 AWS stack(s)\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// TestStacksWithoutAWS: no AWS stack means an empty matrix, which the
// workflows skip, and no claims to read.
func TestStacksWithoutAWS(t *testing.T) {
	for affected, github := range map[string]string{`[]`: "false", `["github"]`: "true"} {
		code, _, stderr, output := runStacksCmd(t, "--affected", affected, "--dir", filepath.Join(t.TempDir(), "none"), "--wet", t.TempDir())
		if want := "github=" + github + "\naws=[]\n"; code != 0 || output != want {
			t.Errorf("%s: exit %d, GITHUB_OUTPUT %q, stderr %q; want %q", affected, code, output, stderr, want)
		}
	}
}

// TestStacksRefusesWhatThisVersionDoesNotReconcile: any other stack fails the
// run, by name, until Phase 3 reconciles Workspaces.
func TestStacksRefusesWhatThisVersionDoesNotReconcile(t *testing.T) {
	code, _, stderr, output := runStacksCmd(t, "--affected", `["aws/dev/workspaces/db","github","rendered/x"]`, "--dir", awsFixture, "--wet", t.TempDir())
	want := "idp stacks: this engine version reconciles only github, aws/<env>/_baseline and aws/<env>/components/<name>; also affected: aws/dev/workspaces/db, rendered/x\n"
	if code != 1 || stderr != want || output != "" {
		t.Errorf("exit %d, stderr %q, GITHUB_OUTPUT %q; want 1, %q and no output", code, stderr, output, want)
	}
}

// TestStacksRefusesMoreThanAMatrixRuns: GitHub runs at most 256 jobs from one
// matrix (spec §11 risk 3); 256 still run.
func TestStacksRefusesMoreThanAMatrixRuns(t *testing.T) {
	affectedOf := func(n int) string {
		var stacks []string
		for i := range n {
			stacks = append(stacks, fmt.Sprintf("aws/dev/components/c%03d", i))
		}
		affected, err := json.Marshal(stacks)
		if err != nil {
			t.Fatal(err)
		}
		return string(affected)
	}
	code, stdout, stderr, _ := runStacksCmd(t, "--affected", affectedOf(256), "--dir", awsFixture, "--wet", t.TempDir())
	if code != 0 || !strings.HasSuffix(stdout, "stacks: github false, 256 AWS stack(s)\n") {
		t.Errorf("256: exit %d, stderr %q; want 0 and 256 AWS stacks", code, stderr)
	}
	code, _, stderr, _ = runStacksCmd(t, "--affected", affectedOf(257), "--dir", awsFixture, "--wet", t.TempDir())
	if want := "idp stacks: 257 AWS stacks are affected; a matrix runs at most 256 jobs (spec §11 risk 3): split the change\n"; code != 1 || stderr != want {
		t.Errorf("257: exit %d, stderr %q; want 1 and %q", code, stderr, want)
	}
}

// TestStacksRefusesEmptyOrDuplicateNames: each stack is one matrix job and one
// artifact name, so a name given twice would make two jobs upload one artifact.
func TestStacksRefusesEmptyOrDuplicateNames(t *testing.T) {
	for affected, name := range map[string]string{
		`["github","github"]`:                       "github",
		`["aws/dev/_baseline","aws/dev/_baseline"]`: "aws/dev/_baseline",
		`[""]`: "",
	} {
		code, _, stderr, output := runStacksCmd(t, "--affected", affected, "--dir", awsFixture, "--wet", t.TempDir())
		want := fmt.Sprintf("idp stacks: --affected has an empty or duplicate stack name %q\n", name)
		if code != 2 || stderr != want || output != "" {
			t.Errorf("%s: exit %d, stderr %q, GITHUB_OUTPUT %q; want 2, %q and no output", affected, code, stderr, output, want)
		}
	}
}

// TestStacksAccountOfAnEnvironmentWithoutAnAWSBlock: an environment the claims
// keep without its aws block is replayed as the account its wet Component
// stacks name, as a removed environment is.
func TestStacksAccountOfAnEnvironmentWithoutAnAWSBlock(t *testing.T) {
	claims := t.TempDir()
	if err := os.CopyFS(claims, os.DirFS(awsFixture)); err != nil {
		t.Fatal(err)
	}
	platform := filepath.Join(claims, "config", "platform.yaml")
	raw, err := os.ReadFile(platform)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(platform, append(raw, []byte("  qa: {}\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	wet := t.TempDir()
	writeFileAll(t, filepath.Join(wet, "aws", "qa", "components", "api", "main.tf.json"),
		`{"module":{"component":{"oidc_provider_arn":"arn:aws:iam::000000000009:oidc-provider/token.actions.githubusercontent.com"}}}`)
	code, stdout, stderr, _ := runStacksCmd(t, "--affected", `["aws/qa/_baseline","aws/qa/components/api"]`, "--dir", claims, "--wet", wet)
	want := "aws/qa/_baseline: account 000000000009\naws/qa/components/api: account 000000000009\nstacks: github false, 2 AWS stack(s)\n"
	if code != 0 || stdout != want {
		t.Errorf("exit %d, stdout %q, stderr %q; want 0 and %q", code, stdout, stderr, want)
	}
}

// TestStacksRefusesAWetStackThatNamesNoAccount: a wet Component stack whose
// oidc_provider_arn names no account fails the run instead of replaying it in
// the wrong account, where it would plan a create instead of the real change.
func TestStacksRefusesAWetStackThatNamesNoAccount(t *testing.T) {
	wet := t.TempDir()
	file := filepath.Join(wet, "aws", "qa", "components", "api", "main.tf.json")
	writeFileAll(t, file, `{"module":{"component":{"oidc_provider_arn":"arn:aws:iam::1:oidc-provider/x"}}}`)
	code, _, stderr, output := runStacksCmd(t, "--affected", `["aws/qa/components/api"]`, "--dir", awsFixture, "--wet", wet)
	want := fmt.Sprintf("idp stacks: %s: module.component.oidc_provider_arn %q names no account (spec §5.5)\n", file, "arn:aws:iam::1:oidc-provider/x")
	if code != 1 || stderr != want || output != "" {
		t.Errorf("exit %d, stderr %q, GITHUB_OUTPUT %q; want 1, %q and no output", code, stderr, output, want)
	}
}

func TestStacksUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--wet", "w"},
		{"--affected", "[]"},
		{"--affected", "github", "--wet", "w"},
		{"--affected", "[]", "--wet", "w", "extra"},
	} {
		if code, _, stderr, _ := runStacksCmd(t, args...); code != 2 || !strings.HasPrefix(stderr, "idp stacks: ") {
			t.Errorf("%v: exit %d, stderr %q; want 2 and an idp stacks message", args, code, stderr)
		}
	}
}
