package cli

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/awsstack"
)

// stubTofu answers every tofu run of idp aws-stack: show prints its plan JSON.
type stubTofu struct {
	show string
	env  []string
	runs []string
	// failOn, when set, makes the runs it returns true for fail with errBoom
	// after they are recorded.
	failOn func(args []string) bool
}

var errBoom = errors.New("boom")

func (s *stubTofu) Tofu(dir string, args ...string) ([]byte, error) {
	s.runs = append(s.runs, strings.Join(args, " "))
	if s.failOn != nil && s.failOn(args) {
		return nil, errBoom
	}
	if args[0] == "show" {
		return []byte(s.show), nil
	}
	return nil, nil
}

// useStubTofu makes idp aws-stack run s instead of the tofu binary.
func useStubTofu(t *testing.T, show string) *stubTofu {
	t.Helper()
	s := &stubTofu{show: show}
	old := newTofu
	newTofu = func(_ string, env []string, _ io.Writer) awsstack.Runner {
		s.env = env
		return s
	}
	t.Cleanup(func() { newTofu = old })
	return s
}

// awsStackDirs renders a wet and a new version of aws/dev/components/api and
// its baseline, and returns the flags that point idp aws-stack at them.
func awsStackDirs(t *testing.T) (dir string, flags []string) {
	t.Helper()
	dir = t.TempDir()
	for _, root := range []string{"new", "wet"} {
		for _, stack := range []string{"aws/dev/_baseline", "aws/dev/components/api"} {
			writeFileAll(t, filepath.Join(dir, root, filepath.FromSlash(stack), "main.tf.json"), `{"v":"`+root+`"}`)
		}
	}
	return dir, []string{"--stack", "aws/dev/components/api", "--account", "000000000001",
		"--new", filepath.Join(dir, "new"), "--wet", filepath.Join(dir, "wet"), "--work", filepath.Join(dir, "work"),
		"--endpoint", "http://localhost:4566"}
}

const updateRole = `{"format_version":"1.2","resource_changes":[{"address":"module.component.aws_iam_role.ci","change":{"actions":["update"]}}]}`

// TestAWSStackPlan: plan writes the stack's reduced plan for the plan-aws
// artifact, prints its counts, and runs tofu as the account.
func TestAWSStackPlan(t *testing.T) {
	s := useStubTofu(t, updateRole)
	dir, flags := awsStackDirs(t)
	plans, out := filepath.Join(dir, "plans"), filepath.Join(dir, "out")
	var stdout, stderr bytes.Buffer
	code := Run(append(append([]string{"aws-stack", "plan"}, flags...), "--out-dir", plans), &stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if got := stdout.String(); got != "aws/dev/components/api: +0 ~1 -0 ±0\n" {
		t.Errorf("stdout = %q", got)
	}
	got, err := os.ReadFile(filepath.Join(plans, "aws.dev.components.api.json"))
	if err != nil || string(got) != updateRole+"\n" {
		t.Errorf("reduced plan = %q, %v", got, err)
	}
	if got, _ := os.ReadFile(out); string(got) != "changes=1\n" {
		t.Errorf("GITHUB_OUTPUT = %q", got)
	}
	if !slices.Contains(s.env, "AWS_ACCESS_KEY_ID=000000000001") {
		t.Errorf("tofu env lacks the account's access key ID")
	}
}

// TestAWSStackKeepsTheJobEnvironmentFromTofu: idp aws-stack hands tofu only
// the allowlisted variables of its environment, so a token of the job never
// reaches OpenTofu (R1-001). TF_ENCRYPTION and IDP_STATE_PASSPHRASE are not
// set here: the command refuses to run with them (ADR-0031).
func TestAWSStackKeepsTheJobEnvironmentFromTofu(t *testing.T) {
	for _, k := range []string{"GITHUB_TOKEN", "ACTIONS_RUNTIME_TOKEN", "AWS_SESSION_TOKEN"} {
		t.Setenv(k, "must-not-reach-tofu")
	}
	s := useStubTofu(t, updateRole)
	dir, flags := awsStackDirs(t)
	var stdout, stderr bytes.Buffer
	args := append(append([]string{"aws-stack", "plan"}, flags...), "--out-dir", filepath.Join(dir, "plans"))
	if code := Run(args, &stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": filepath.Join(dir, "out")})); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	for _, kv := range s.env {
		if strings.Contains(kv, "must-not-reach-tofu") {
			t.Errorf("tofu env holds %s", kv)
		}
	}
	if !slices.Contains(s.env, "AWS_ACCESS_KEY_ID=000000000001") {
		t.Error("tofu env lacks the account's access key ID")
	}
}

// TestAWSStackApply: apply re-plans, applies the gated plan, marks the stack
// applied for the wet commit and writes the evidence to the job summary.
func TestAWSStackApply(t *testing.T) {
	s := useStubTofu(t, updateRole)
	dir, flags := awsStackDirs(t)
	gated, applied, summary := filepath.Join(dir, "gated.json"), filepath.Join(dir, "applied"), filepath.Join(dir, "summary.md")
	writeFileAll(t, gated, `{"aws/dev/components/api":[{"address":"module.component.aws_iam_role.ci","action":"update"}],"github":[]}`)
	var stdout, stderr bytes.Buffer
	code := Run(append(append([]string{"aws-stack", "apply"}, flags...), "--fingerprint", gated, "--applied-dir", applied),
		&stdout, &stderr, envOf(map[string]string{"GITHUB_STEP_SUMMARY": summary}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if last := s.runs[len(s.runs)-1]; !strings.HasPrefix(last, "apply -input=false ") || !strings.HasSuffix(last, "aws.dev.components.api.tfplan") {
		t.Errorf("last tofu run = %q, want the saved plan's apply", last)
	}
	if got := stdout.String(); got != "aws/dev/components/api: +0 ~1 -0 ±0: applied\n" {
		t.Errorf("stdout = %q", got)
	}
	if got, err := os.ReadFile(filepath.Join(applied, "aws.dev.components.api")); err != nil || string(got) != "aws/dev/components/api\n" {
		t.Errorf("applied marker = %q, %v", got, err)
	}
	want := "### `aws/dev/components/api`\n\nThe re-plan equals the gated fingerprint, and it applied: +0 ~1 -0 ±0.\n\n- update `module.component.aws_iam_role.ci`\n\n"
	if got, _ := os.ReadFile(summary); string(got) != want {
		t.Errorf("summary =\n%s\nwant\n%s", got, want)
	}
}

// TestAWSStackApplyRefusal: a re-plan that differs from the gated one applies
// nothing and marks nothing, so the stack's render stays old in wet and the
// next reconcile retries it (spec §6.2 step 4).
func TestAWSStackApplyRefusal(t *testing.T) {
	useStubTofu(t, updateRole)
	dir, flags := awsStackDirs(t)
	gated, applied, summary := filepath.Join(dir, "gated.json"), filepath.Join(dir, "applied"), filepath.Join(dir, "summary.md")
	writeFileAll(t, gated, `{"aws/dev/components/api":[]}`)
	var stdout, stderr bytes.Buffer
	code := Run(append(append([]string{"aws-stack", "apply"}, flags...), "--fingerprint", gated, "--applied-dir", applied),
		&stdout, &stderr, envOf(map[string]string{"GITHUB_STEP_SUMMARY": summary}))
	refusal := "aws/dev/components/api: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)\n  not gated: aws/dev/components/api: update module.component.aws_iam_role.ci"
	if code != 1 || stderr.String() != "idp: "+refusal+"\n" {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
	if _, err := os.Stat(applied); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("applied dir: %v, want none", err)
	}
	if got, _ := os.ReadFile(summary); string(got) != "### `aws/dev/components/api`\n\nNothing was applied:\n\n```\n"+refusal+"\n```\n\n" {
		t.Errorf("summary = %q", got)
	}
}

// runApply runs idp aws-stack apply against the stub, with a gate that matches
// its plan, and returns the exit code, stderr and the paths it may have written.
func runApply(t *testing.T) (code int, stderr, applied, summary string) {
	t.Helper()
	dir, flags := awsStackDirs(t)
	gated := filepath.Join(dir, "gated.json")
	applied, summary = filepath.Join(dir, "applied"), filepath.Join(dir, "summary.md")
	writeFileAll(t, gated, `{"aws/dev/components/api":[{"address":"module.component.aws_iam_role.ci","action":"update"}]}`)
	var out, errb bytes.Buffer
	code = Run(append(append([]string{"aws-stack", "apply"}, flags...), "--fingerprint", gated, "--applied-dir", applied),
		&out, &errb, envOf(map[string]string{"GITHUB_STEP_SUMMARY": summary}))
	return code, errb.String(), applied, summary
}

// TestAWSStackApplyFailsPartway: a failed final tofu apply may have applied
// part of the plan; it exits 1, marks nothing (the render stays old in wet, so
// the next reconcile re-plans the stack), and the summary does not claim that
// nothing was applied.
func TestAWSStackApplyFailsPartway(t *testing.T) {
	s := useStubTofu(t, updateRole)
	s.failOn = func(args []string) bool { return args[0] == "apply" && !slices.Contains(args, "-auto-approve") }
	code, stderr, applied, summary := runApply(t)
	failure := "aws/dev/components/api: tofu apply failed, part of the plan may have been applied: boom"
	if code != 1 || stderr != "idp: "+failure+"\n" {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(applied); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("applied dir: %v, want none", err)
	}
	want := "### `aws/dev/components/api`\n\nThe apply failed partway, so part of the plan may have been applied. The stack's render stays old in wet, so the next reconcile re-plans it:\n\n```\n" + failure + "\n```\n\n"
	if got, _ := os.ReadFile(summary); string(got) != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

// TestAWSStackApplyReplayFails: a replay failing applied nothing of the new
// version: no plan runs, nothing is marked, and the summary says nothing was
// applied.
func TestAWSStackApplyReplayFails(t *testing.T) {
	s := useStubTofu(t, updateRole)
	s.failOn = func(args []string) bool { return args[0] == "apply" && slices.Contains(args, "-auto-approve") }
	code, stderr, applied, summary := runApply(t)
	if code != 1 || stderr != "idp: boom\n" {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	for _, run := range s.runs {
		if strings.HasPrefix(run, "plan ") {
			t.Errorf("a plan ran after the failed replay: %q", run)
		}
	}
	if _, err := os.Stat(applied); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("applied dir: %v, want none", err)
	}
	if got, _ := os.ReadFile(summary); string(got) != "### `aws/dev/components/api`\n\nNothing was applied:\n\n```\nboom\n```\n\n" {
		t.Errorf("summary = %q", got)
	}
}

// TestAWSStackRefusesTheStatePassphrase: a job that runs floci shares the
// runner's Docker socket, so it never holds the state passphrase (ADR-0031).
func TestAWSStackRefusesTheStatePassphrase(t *testing.T) {
	for _, name := range []string{"TF_ENCRYPTION", "IDP_STATE_PASSPHRASE"} {
		s := useStubTofu(t, updateRole)
		_, flags := awsStackDirs(t)
		var stdout, stderr bytes.Buffer
		code := Run(append(append([]string{"aws-stack", "plan"}, flags...), "--out-dir", t.TempDir()), &stdout, &stderr, envOf(map[string]string{name: "x"}))
		want := "idp aws-stack: " + name + " is set: a job that runs floci shares the runner's Docker socket, so it holds no secret (ADR-0031)\n"
		if code != 1 || stderr.String() != want || len(s.runs) != 0 {
			t.Errorf("%s: exit %d, stderr %q, tofu runs %v; want 1, %q and none", name, code, stderr.String(), s.runs, want)
		}
	}
}

func TestAWSStackUsageErrors(t *testing.T) {
	_, flags := awsStackDirs(t)
	without := func(flag string) []string {
		i := slices.Index(flags, flag)
		return slices.Concat(flags[:i], flags[i+2:])
	}
	for name, args := range map[string][]string{
		"no mode":              nil,
		"unknown mode":         {"destroy"},
		"plan without out-dir": append([]string{"plan"}, flags...),
		"apply without gate":   append(append([]string{"apply"}, flags...), "--applied-dir", "a"),
		"apply without marker": append(append([]string{"apply"}, flags...), "--fingerprint", "f"),
		"no stack":             append(append([]string{"plan"}, without("--stack")...), "--out-dir", "o"),
		"no endpoint":          append(append([]string{"plan"}, without("--endpoint")...), "--out-dir", "o"),
		"bad account":          append(append([]string{"plan"}, flags...), "--out-dir", "o", "--account", "1"),
		"extra argument":       append(append([]string{"plan"}, flags...), "--out-dir", "o", "extra"),
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"aws-stack"}, args...), &stdout, &stderr, noEnv); code != 2 || !strings.HasPrefix(stderr.String(), "idp aws-stack") {
			t.Errorf("%s: exit %d, stderr %q; want 2 and an idp aws-stack message", name, code, stderr.String())
		}
	}
}
