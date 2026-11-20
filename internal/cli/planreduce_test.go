package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

// planFixture is a tofu show -json fixture of internal/plan.
func planFixture(name string) string { return filepath.Join("..", "plan", "testdata", name) }

// TestPlanReduceCommand: idp plan-reduce writes the stack's reduced plan under
// its escaped name, prints its counts and sets the changes output.
func TestPlanReduceCommand(t *testing.T) {
	dir := t.TempDir()
	plans, out := filepath.Join(dir, "plans"), filepath.Join(dir, "out")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plan-reduce", "--stack", "aws/dev/components/api", "--plan", planFixture("mixed.json"), "--out-dir", plans},
		&stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if got := stdout.String(); got != "aws/dev/components/api: +1 ~1 -1 ±1\n" {
		t.Errorf("stdout = %q", got)
	}
	f, err := os.Open(filepath.Join(plans, "aws.dev.components.api.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if p, err := plan.Parse("aws/dev/components/api", f); err != nil || len(p.Changes) != 4 {
		t.Errorf("reduced plan = %+v, %v; want mixed.json's 4 changes", p, err)
	}
	if got, _ := os.ReadFile(out); string(got) != "changes=4\n" {
		t.Errorf("GITHUB_OUTPUT = %q", got)
	}
}

func TestPlanReduceUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--plan", "p.json", "--out-dir", "d"},
		{"--stack", "github", "--out-dir", "d"},
		{"--stack", "github", "--plan", "p.json"},
		{"--stack", "aws/dev/workspaces/db", "--plan", "p.json", "--out-dir", "d"},
		{"--stack", "github", "--plan", "p.json", "--out-dir", "d", "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"plan-reduce"}, args...), &stdout, &stderr, noEnv); code != 2 || !strings.HasPrefix(stderr.String(), "idp plan-reduce: ") {
			t.Errorf("%v: exit %d, stderr %q; want 2 and an idp plan-reduce message", args, code, stderr.String())
		}
	}
}

// TestPlanSummaryReadsAPlanDir: the collector job's path. Reduced plans of a
// slash stack and of github, read by escaped name, give a comment whose table
// and fingerprint marker name both stacks by their slash paths.
func TestPlanSummaryReadsAPlanDir(t *testing.T) {
	dir := t.TempDir()
	plans := filepath.Join(dir, "plans")
	// Each plan lies in a directory of its own, where the pipelines lay it out. A
	// github.json that an AWS stack's directory, or the directory itself, holds is
	// never read (ADR-0031).
	for _, c := range []struct{ stack, fixture, artifact string }{
		{"github", "mixed.json", "idp-plan-github"},
		{"aws/dev/components/api", "creates.json", "idp-plan-aws.dev.components.api"},
		{"github", "creates.json", "idp-plan-aws.dev.components.api"},
		{"github", "creates.json", ""},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"plan-reduce", "--stack", c.stack, "--plan", planFixture(c.fixture), "--out-dir", filepath.Join(plans, c.artifact)}, &stdout, &stderr, noEnv); code != 0 {
			t.Fatalf("plan-reduce %s: exit %d, stderr %q", c.stack, code, stderr.String())
		}
	}
	comment, fp := filepath.Join(dir, "c.md"), filepath.Join(dir, "fp.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plan-summary", "--stacks", `["aws/dev/components/api","github"]`, "--plan-dir", plans,
		"--head-sha", headSHA, "--comment-out", comment, "--fingerprint-out", fp}, &stdout, &stderr, noEnv)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	want := "aws/dev/components/api: +4 ~0 -0 ±0\ngithub: +1 ~1 -1 ±1\nplan-summary: 8 change(s), destructive: true\n"
	if stdout.String() != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout.String(), want)
	}
	body, err := os.ReadFile(comment)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "| <code>aws/dev/components/api</code> |") {
		t.Errorf("comment has no table row for the slash stack:\n%s", body)
	}
	sha, f, ok, err := plan.DecodeMarker(string(body))
	if err != nil || !ok || sha != headSHA || len(f["aws/dev/components/api"]) != 4 || len(f["github"]) != 4 || len(f) != 2 {
		t.Errorf("marker: sha %q, fingerprint %v, ok %t, err %v", sha, f, ok, err)
	}
}

// TestPlanSummaryTakesPlanOrPlanDir: the two ways to name the plans exclude each
// other, and a stack missing from the directory fails.
func TestPlanSummaryTakesPlanOrPlanDir(t *testing.T) {
	dir := t.TempDir()
	common := []string{"plan-summary", "--stacks", `["github"]`, "--head-sha", headSHA, "--comment-out", filepath.Join(dir, "c.md"), "--fingerprint-out", filepath.Join(dir, "fp.json")}
	var stdout, stderr bytes.Buffer
	if code := Run(append(common, "--plan-dir", dir, "--plan", "github="+planFixture("mixed.json")), &stdout, &stderr, noEnv); code != 2 || stderr.String() != "idp plan-summary: give --plan or --plan-dir, not both\n" {
		t.Errorf("both: exit %d, stderr %q", code, stderr.String())
	}
	stderr.Reset()
	if code := Run(append(common, "--plan-dir", dir), &stdout, &stderr, noEnv); code != 1 || !strings.Contains(stderr.String(), "github.json") {
		t.Errorf("missing: exit %d, stderr %q; want 1 naming github.json", code, stderr.String())
	}
}

// TestPlanSummaryPlanDirReadsOnlyReconcilableStacks: two names escape to one
// file only when one of them is not reconcilable (a.b and a/b), so --plan-dir
// refuses any such name.
func TestPlanSummaryPlanDirReadsOnlyReconcilableStacks(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plan-summary", "--stacks", `["github","a.b"]`, "--head-sha", headSHA, "--plan-dir", dir,
		"--comment-out", filepath.Join(dir, "c.md"), "--fingerprint-out", filepath.Join(dir, "fp.json")}, &stdout, &stderr, noEnv)
	want := "idp plan-summary: --plan-dir reads only github, aws/<env>/_baseline and aws/<env>/components/<name>; got \"a.b\"\n"
	if code != 2 || stderr.String() != want {
		t.Errorf("exit %d, stderr %q; want 2 and %q", code, stderr.String(), want)
	}
}
