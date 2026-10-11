package workflowspec

import (
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

func TestSetupIDPInterface(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	if a.Runs.Using != "composite" {
		t.Fatalf("runs.using = %q, want composite", a.Runs.Using)
	}
	if len(a.Inputs) != 2 || !a.Inputs["engine-dir"].Required || !a.Inputs["version"].Required {
		t.Errorf("inputs = %v, want exactly engine-dir and version, both required", a.Inputs)
	}
}

func TestSetupIDPInstallsTheToolchain(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	_, goStep := mustStep(t, setupIDPPath, a.Runs.Steps, "go")
	_, tofuStep := mustStep(t, setupIDPPath, a.Runs.Steps, "tofu")
	if !strings.HasPrefix(goStep.Uses, "actions/setup-go@") || !strings.HasPrefix(tofuStep.Uses, "opentofu/setup-opentofu@") {
		t.Fatalf("steps go and tofu must use setup-go and setup-opentofu, got %q and %q", goStep.Uses, tofuStep.Uses)
	}
	if got := goStep.With["go-version-file"]; got != "${{ inputs.engine-dir }}/go.mod" {
		t.Errorf("setup-go go-version-file = %q", got)
	}
	if goStep.With["cache"] != "false" {
		t.Errorf("setup-go must set cache: false: a cache restored into the job that holds the writer key is a poisoning path (ADR-0021)")
	}
	if tofuStep.With["tofu_wrapper"] != "false" {
		t.Errorf("setup-opentofu must set tofu_wrapper: false")
	}
	if got, want := tofuStep.With["tofu_version"], ciTofuVersion(t); got != want {
		t.Errorf("setup-idp installs OpenTofu %q, CI tests the modules with %q: keep one version", got, want)
	}
}

// ciTofuVersion is the OpenTofu version CI tests the modules with.
func ciTofuVersion(t *testing.T) string {
	t.Helper()
	w := loadWorkflow(t, ".github/workflows/ci.yaml")
	for _, s := range mustJob(t, w, "modules").Steps {
		if strings.HasPrefix(s.Uses, "opentofu/setup-opentofu@") {
			return s.With["tofu_version"]
		}
	}
	t.Fatal("ci.yaml job modules has no setup-opentofu step")
	return ""
}

func TestSetupIDPBuildsTheGivenCommit(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	_, build := mustStep(t, setupIDPPath, a.Runs.Steps, "build")
	if build.Shell != "bash" {
		t.Errorf("build shell = %q, want bash", build.Shell)
	}
	if build.WorkingDirectory != "${{ inputs.engine-dir }}" {
		t.Errorf("build working-directory = %q", build.WorkingDirectory)
	}
	if build.Env["IDP_VERSION"] != "${{ inputs.version }}" {
		t.Errorf("build env IDP_VERSION = %q", build.Env["IDP_VERSION"])
	}
	wantRun(t, "setup-idp build", build,
		`[[ "$IDP_VERSION" =~ ^[0-9a-f]{40}$ ]]`,
		`-X `+version.Repo+`/internal/version.Version=${IDP_VERSION}`,
		`-o "$RUNNER_TEMP/idp-bin/idp" ./cmd/idp`,
		`echo "$RUNNER_TEMP/idp-bin" >> "$GITHUB_PATH"`,
	)
}
