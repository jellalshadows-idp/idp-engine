package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

const headSHA = "0123456789abcdef0123456789abcdef01234567"

func TestPlanSummaryCommand(t *testing.T) {
	dir := t.TempDir()
	comment, fp, out := filepath.Join(dir, "c.md"), filepath.Join(dir, "fp.json"), filepath.Join(dir, "out")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plan-summary", "--stacks", `["github"]`, "--plan", "github=" + filepath.Join("..", "plan", "testdata", "mixed.json"),
		"--head-sha", headSHA, "--run-url", "https://example.test/run", "--comment-out", comment, "--fingerprint-out", fp},
		&stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	body, err := os.ReadFile(comment)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), plan.CommentMarker) {
		t.Errorf("comment = %q", body)
	}
	f, err := os.Open(fp)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := plan.ReadFingerprint(f)
	if err != nil || len(got["github"]) != 4 {
		t.Errorf("fingerprint = %v, err %v", got, err)
	}
	outputs, _ := os.ReadFile(out)
	if string(outputs) != "changes=4\ndestructive=true\n" {
		t.Errorf("GITHUB_OUTPUT = %q", outputs)
	}
	if !strings.Contains(stdout.String(), "github: +1 ~1 -1 ±1") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestPlanSummaryWithoutPlans(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "fp.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plan-summary", "--stacks", "[]", "--head-sha", headSHA, "--comment-out", filepath.Join(dir, "c.md"), "--fingerprint-out", fp}, &stdout, &stderr, noEnv)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if got, _ := os.ReadFile(fp); string(got) != "{}\n" {
		t.Errorf("fingerprint = %q, want {}", got)
	}
}

func TestPlanSummaryUsageErrors(t *testing.T) {
	dir := t.TempDir()
	base := []string{"--comment-out", filepath.Join(dir, "c"), "--fingerprint-out", filepath.Join(dir, "f")}
	mixed := filepath.Join("..", "plan", "testdata", "mixed.json")
	for name, tc := range map[string]struct {
		args []string
		msg  string
	}{
		"bad sha":         {append([]string{"plan-summary", "--stacks", "[]", "--head-sha", "abc"}, base...), ""},
		"plan without =":  {append([]string{"plan-summary", "--stacks", `["github"]`, "--head-sha", headSHA, "--plan", "github"}, base...), ""},
		"duplicate stack": {append([]string{"plan-summary", "--stacks", `["github"]`, "--head-sha", headSHA, "--plan", "github=a", "--plan", "github=b"}, base...), ""},
		"missing outputs": {[]string{"plan-summary", "--stacks", "[]", "--head-sha", headSHA}, ""},
		"missing stacks":  {append([]string{"plan-summary", "--head-sha", headSHA}, base...), "--stacks"},
		"affected without plan": {append([]string{"plan-summary", "--stacks", `["github"]`, "--head-sha", headSHA}, base...),
			`stack "github" is affected but has no --plan`},
		"plan not in stacks": {append([]string{"plan-summary", "--stacks", "[]", "--head-sha", headSHA, "--plan", "github=" + mixed}, base...),
			`--plan "github" is not in --stacks`},
		"stacks not json":     {append([]string{"plan-summary", "--stacks", "not-json", "--head-sha", headSHA}, base...), ""},
		"stacks not an array": {append([]string{"plan-summary", "--stacks", `{"a":1}`, "--head-sha", headSHA}, base...), ""},
		"stacks empty name":   {append([]string{"plan-summary", "--stacks", `[""]`, "--head-sha", headSHA}, base...), ""},
		"stacks duplicate":    {append([]string{"plan-summary", "--stacks", `["github","github"]`, "--head-sha", headSHA}, base...), ""},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(tc.args, &stdout, &stderr, noEnv); code != 2 {
			t.Errorf("%s: exit %d, want 2 (stderr %q)", name, code, stderr.String())
		}
		if !strings.Contains(stderr.String(), tc.msg) {
			t.Errorf("%s: stderr %q does not contain %q", name, stderr.String(), tc.msg)
		}
	}
}
