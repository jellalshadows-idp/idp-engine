package workflowspec

import (
	"fmt"
	"regexp"
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
	for _, l := range strings.Split(run, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.Contains(l, "tofu init") && !strings.Contains(l, "-lockfile=readonly"):
			errs = append(errs, fmt.Errorf("%q: tofu init must use -lockfile=readonly", l))
		case strings.Contains(l, "tofu plan") && !strings.Contains(l, `-out="$RUNNER_TEMP/idp/`):
			errs = append(errs, fmt.Errorf("%q: tofu plan must write its plan under $RUNNER_TEMP/idp/", l))
		case strings.Contains(l, "tofu apply") && (strings.Contains(l, "-auto-approve") || !strings.Contains(l, ".tfplan")):
			errs = append(errs, fmt.Errorf("%q: tofu apply must apply a saved plan", l))
		}
	}
	return errs
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkTofu(tt.run); len(got) != tt.errs {
				t.Errorf("checkTofu(%q) = %v, want %d error(s)", tt.run, got, tt.errs)
			}
		})
	}
}
