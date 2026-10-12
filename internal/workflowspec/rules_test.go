package workflowspec

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var (
	pinnedRef   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}$`)
	versionNote = regexp.MustCompile(`^# v\d+\.\d+\.\d+(\s|$)`)
	unreleased  = regexp.MustCompile(`^# unreleased(\s|$)`)
)

// checkUses enforces spec §7.5: actions and reusable workflows from other
// repositories are pinned to a full commit SHA, with the version as a trailing
// comment that Renovate keeps in step. An engine workflow ref of an unreleased
// engine says "# unreleased" instead. Local actions (./...) are checked by
// TestJobsBuildIdpFromTheWorkflowCommit.
func checkUses(value, comment string) error {
	if strings.HasPrefix(value, "./") {
		return nil
	}
	if !pinnedRef.MatchString(value) {
		return fmt.Errorf("%q is not pinned to a full commit SHA", value)
	}
	if strings.Contains(value, "/.github/workflows/") && unreleased.MatchString(comment) {
		return nil
	}
	if !versionNote.MatchString(comment) {
		return fmt.Errorf("%q needs a trailing '# vX.Y.Z' comment, got %q", value, comment)
	}
	return nil
}

// checkTofu enforces the wet hygiene rules of the Phase 1b log on one run
// script: init verifies the shipped lock file, plans are written under
// $RUNNER_TEMP/idp/ (never into a stack directory that is later synced to wet),
// and an apply only ever applies a saved plan.
func checkTofu(run string) []error {
	var errs []error
	for _, l := range logicalLines(run) {
		f := commandFields(l)
		i := slices.Index(f, "tofu")
		if i < 0 {
			continue
		}
		sub := ""
		for _, w := range f[i+1:] {
			if !strings.HasPrefix(w, "-") {
				sub = w
				break
			}
		}
		switch sub {
		case "init":
			if !slices.Contains(f, "-lockfile=readonly") {
				errs = append(errs, fmt.Errorf("%q: tofu init must use -lockfile=readonly", l))
			}
		case "plan":
			if !slices.ContainsFunc(f, func(w string) bool {
				return strings.HasPrefix(w, `-out="$RUNNER_TEMP/idp/`) || strings.HasPrefix(w, `-out=$RUNNER_TEMP/idp/`)
			}) {
				errs = append(errs, fmt.Errorf("%q: tofu plan must write its plan under $RUNNER_TEMP/idp/", l))
			}
		case "apply":
			if slices.Contains(f, "-auto-approve") || !slices.ContainsFunc(f, func(w string) bool {
				return strings.Contains(w, "$RUNNER_TEMP/idp/") && (strings.HasSuffix(w, `.tfplan`) || strings.HasSuffix(w, `.tfplan"`))
			}) {
				errs = append(errs, fmt.Errorf("%q: tofu apply must apply a saved plan from $RUNNER_TEMP/idp/", l))
			}
		}
	}
	return errs
}

// logicalLines joins backslash-continued lines of a script into single lines.
func logicalLines(run string) []string {
	var out []string
	var cur strings.Builder
	for _, l := range strings.Split(run, "\n") {
		trimmed := strings.TrimRight(l, " \t\r")
		if before, ok := strings.CutSuffix(trimmed, `\`); ok {
			cur.WriteString(before)
			cur.WriteString(" ")
			continue
		}
		cur.WriteString(trimmed)
		out = append(out, cur.String())
		cur.Reset()
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func TestCheckUses(t *testing.T) {
	sha := strings.Repeat("a", 40)
	tests := []struct {
		name, value, comment string
		ok                   bool
	}{
		{"pinned action", "actions/checkout@" + sha, "# v7.0.1", true},
		{"pinned action with a trailing note", "actions/create-github-app-token@" + sha, "# v3.2.0 # zizmor: ignore[github-app] reason", true},
		{"local action", "./.idp/engine/.github/actions/setup-idp", "", true},
		{"reusable workflow, unreleased", "acme/idp-engine/.github/workflows/pr.yaml@" + sha, "# unreleased", true},
		{"reusable workflow, released", "acme/idp-engine/.github/workflows/pr.yaml@" + sha, "# v0.1.0", true},
		{"tag", "actions/checkout@v7", "# v7", false},
		{"short sha", "actions/checkout@" + sha[:12], "# v7.0.1", false},
		{"uppercase sha", "actions/checkout@" + strings.Repeat("A", 40), "# v7.0.1", false},
		{"no comment", "actions/checkout@" + sha, "", false},
		{"comment without a version", "actions/checkout@" + sha, "# latest", false},
		{"unreleased on an action", "actions/checkout@" + sha, "# unreleased", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkUses(tt.value, tt.comment)
			if (err == nil) != tt.ok {
				t.Errorf("checkUses(%q, %q) = %v, want ok=%v", tt.value, tt.comment, err, tt.ok)
			}
		})
	}
}

func TestCheckTofu(t *testing.T) {
	tests := []struct {
		name, run string
		errs      int
	}{
		{"init with the lock file", "tofu init -input=false -lockfile=readonly", 0},
		{"init without it", "tofu init -input=false", 1},
		{"plan under RUNNER_TEMP/idp", `tofu plan -input=false -lock=false -out="$RUNNER_TEMP/idp/plans/github.tfplan"`, 0},
		{"plan into the stack directory", "tofu plan -input=false -out=github.tfplan", 1},
		{"plan without a plan file", "tofu plan -input=false", 1},
		{"apply a saved plan", `tofu apply -input=false "$RUNNER_TEMP/idp/in/github.tfplan"`, 0},
		{"apply without a plan", "tofu apply -input=false -auto-approve", 1},
		{"show is not checked", `tofu show -json "$RUNNER_TEMP/idp/plans/github.tfplan" > out.json`, 0},
		{"two bad lines", "tofu init\ntofu apply -auto-approve", 2},
		{"continued plan", "tofu plan -input=false \\\n  -out=\"$RUNNER_TEMP/idp/plans/github.tfplan\"", 0},
		{"continued init", "tofu init -input=false \\\n  -lockfile=readonly", 0},
		{"comment line", "# tofu init", 0},
		{"echo line", `echo "run tofu plan later"`, 0},
		{"chdir before the subcommand", "tofu -chdir=stack plan -input=false", 1},
		{"two spaces", "tofu  init", 1},
		{"apply a plan from elsewhere", "tofu apply -input=false other/github.tfplan", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkTofu(tt.run); len(got) != tt.errs {
				t.Errorf("checkTofu(%q) = %v, want %d error(s)", tt.run, got, tt.errs)
			}
		})
	}
}
