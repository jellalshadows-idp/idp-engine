package workflowspec

import (
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const ciPath = ".github/workflows/ci.yaml"

// TestCITestsEveryModule: the modules job tests every module, each with the lock
// file of its provider.
func TestCITestsEveryModule(t *testing.T) {
	w := loadWorkflow(t, ciPath)
	n := mapValue(mapValue(mapValue(jobNode(w, "modules"), "strategy"), "matrix"), "module")
	if n == nil {
		t.Fatal("ci modules: no strategy.matrix.module")
	}
	var matrix []string
	if err := n.Decode(&matrix); err != nil {
		t.Fatal(err)
	}
	if got, want := slices.Sorted(slices.Values(matrix)), moduleDirs(t); !slices.Equal(got, want) {
		t.Errorf("ci modules matrix = %v, want every module: %v", got, want)
	}
	_, s := mustStep(t, "ci modules", mustJob(t, w, "modules").Steps, "tofu-test")
	if s.WorkingDirectory != "modules/${{ matrix.module }}" || s.Env["MODULE"] != "${{ matrix.module }}" {
		t.Errorf("ci modules tofu-test: working-directory %q, MODULE %q; want the matrix module", s.WorkingDirectory, s.Env["MODULE"])
	}
	wantLines(t, "ci modules tofu-test", s.Run, []string{
		"tofu fmt -check -recursive",
		`cp "$GITHUB_WORKSPACE/lockfiles/${MODULE%%/*}.terraform.lock.hcl" .terraform.lock.hcl`,
		"tofu init -input=false -lockfile=readonly",
		"tofu test",
	})
}

// TestCIRenderSmokeCoversEveryFixture: render-smoke renders and validates every
// fixture that has a golden render, from one list.
func TestCIRenderSmokeCoversEveryFixture(t *testing.T) {
	var want []string
	for _, d := range glob(t, "internal/render/testdata/*/expected") {
		want = append(want, path.Base(path.Dir(d)))
	}
	loop := regexp.MustCompile(`(?m)^\s*for fixture in ([^;]+); do$`)
	var lists [][]string
	for _, s := range mustJob(t, loadWorkflow(t, ciPath), "render-smoke").Steps {
		for _, m := range loop.FindAllStringSubmatch(s.Run, -1) {
			lists = append(lists, strings.Fields(m[1]))
		}
	}
	if len(lists) != 1 {
		t.Fatalf("render-smoke lists the fixtures %d times, want once: %v", len(lists), lists)
	}
	if got := slices.Sorted(slices.Values(lists[0])); !slices.Equal(got, want) {
		t.Errorf("render-smoke fixtures = %v, want every fixture with a golden render: %v", got, want)
	}
	_, s := mustStep(t, "ci render-smoke", mustJob(t, loadWorkflow(t, ciPath), "render-smoke").Steps, "render-validate")
	wantLines(t, "ci render-smoke", s.Run, []string{
		`go run ./cmd/idp render --dir "internal/render/testdata/$fixture" --out "$RUNNER_TEMP/rendered/$fixture" --modules-dir modules`,
		`tofu -chdir="$RUNNER_TEMP/rendered/$fixture/github" init -backend=false -input=false`,
		`tofu -chdir="$RUNNER_TEMP/rendered/$fixture/github" validate`,
	})
}

// TestCIFlociJob pins the level-3 job (spec §8.4): floci pinned by tag and
// index digest, so Renovate moves both (spec §7.5); the Docker socket for its
// ECR; a readiness wait that fails instead of continuing; and the test step
// with the endpoint set, so TestFloci runs instead of skipping.
func TestCIFlociJob(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, ciPath), "floci")
	wantPerms(t, "ci floci", j.Permissions, map[string]string{"contents": "read"})
	if j.TimeoutMinutes < 1 || j.TimeoutMinutes > 30 {
		t.Errorf("ci floci timeout-minutes = %d, want 1 to 30: a hung emulator must not hold a runner for 6 hours", j.TimeoutMinutes)
	}
	if got := slices.Sorted(maps.Keys(j.Services)); !slices.Equal(got, []string{"floci"}) {
		t.Fatalf("ci floci services = %v, want floci", got)
	}
	svc := j.Services["floci"]
	if !regexp.MustCompile(`^floci/floci:\d+\.\d+\.\d+@sha256:[0-9a-f]{64}$`).MatchString(svc.Image) {
		t.Errorf("floci image = %q, want floci/floci:X.Y.Z@sha256:<index digest> (ADR-0004)", svc.Image)
	}
	if !slices.Equal(svc.Ports, []string{"4566:4566"}) || !slices.Equal(svc.Volumes, []string{"/var/run/docker.sock:/var/run/docker.sock"}) {
		t.Errorf("floci ports %v, volumes %v; want 4566:4566 and the Docker socket", svc.Ports, svc.Volumes)
	}
	if len(svc.Env) != 0 || svc.Options != "" {
		t.Errorf("floci env %v, options %q; want floci's defaults", svc.Env, svc.Options)
	}
	var tofu []string
	for _, s := range j.Steps {
		if strings.HasPrefix(s.Uses, "opentofu/setup-opentofu@") {
			tofu = append(tofu, s.With["tofu_version"])
		}
	}
	if !slices.Equal(tofu, []string{ciTofuVersion(t)}) {
		t.Errorf("ci floci installs OpenTofu %v, want once the modules job's %s", tofu, ciTofuVersion(t))
	}
	ri, ready := mustStep(t, "ci floci", j.Steps, "floci-ready")
	ti, test := mustStep(t, "ci floci", j.Steps, "floci-test")
	if ri > ti {
		t.Error("ci floci: the tests run before floci is ready")
	}
	for _, s := range []step{ready, test} {
		if s.If != "" || s.ContinueOnError != "" {
			t.Errorf("ci floci %s: if %q, continue-on-error %q; it must run, and fail the job", s.ID, s.If, s.ContinueOnError)
		}
	}
	wantLines(t, "ci floci-ready", ready.Run, []string{
		`echo "::error::floci did not report ready on localhost:4566 within 60 seconds"`,
		"exit 1",
	})
	if test.Env["IDP_FLOCI_ENDPOINT"] != "http://localhost:4566" {
		t.Errorf("ci floci-test env IDP_FLOCI_ENDPOINT = %q, want http://localhost:4566: without it TestFloci skips", test.Env["IDP_FLOCI_ENDPOINT"])
	}
	if got, want := strings.TrimSpace(test.Run), "go test -count=1 -timeout=15m -v ./internal/floci/"; got != want {
		t.Errorf("ci floci-test run = %q, want %q", got, want)
	}
}
