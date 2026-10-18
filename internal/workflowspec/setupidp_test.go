package workflowspec

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

// notInstalled is the condition of the steps that build idp from source: they
// run only when the download step installed no release (ADR-0028).
const notInstalled = "steps.download.outputs.installed != 'true'"

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
	if goStep.If != notInstalled {
		t.Errorf("setup-go if = %q, want %q: Go is needed only for a source build", goStep.If, notInstalled)
	}
	if tofuStep.If != "" {
		t.Errorf("setup-opentofu if = %q: every pipeline runs tofu, whatever installed idp", tofuStep.If)
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
	if build.If != notInstalled {
		t.Errorf("build if = %q, want %q", build.If, notInstalled)
	}
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

// engineRepo is the engine's owner/name; the download looks up only its
// releases, like the module sources the renderer writes.
var engineRepo = strings.TrimPrefix(version.Repo, "github.com/")

func downloadStep(t *testing.T) (int, step) {
	t.Helper()
	return mustStep(t, setupIDPPath, loadAction(t, setupIDPPath).Runs.Steps, "download")
}

func TestSetupIDPDownloadsAndVerifiesTheRelease(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	di, dl := downloadStep(t)
	gi, _ := mustStep(t, setupIDPPath, a.Runs.Steps, "go")
	bi, _ := mustStep(t, setupIDPPath, a.Runs.Steps, "build")
	if di != 0 || gi < di || bi < di {
		t.Errorf("steps: download %d, go %d, build %d; the download decides first", di, gi, bi)
	}
	if dl.Shell != "bash" || dl.WorkingDirectory != "" || dl.If != "" {
		t.Errorf("download shell %q working-directory %q if %q, want bash, none, none", dl.Shell, dl.WorkingDirectory, dl.If)
	}
	if want := map[string]string{"GH_TOKEN": "${{ github.token }}", "IDP_VERSION": "${{ inputs.version }}"}; !maps.Equal(map[string]string(dl.Env), want) {
		t.Errorf("download env = %v, want %v: the job's own token reads the public engine's releases (ADR-0023)", dl.Env, want)
	}
	// The archive the download asks for must be one GoReleaser uploads.
	assets := releaseAssets(t, "${tag#v}")
	for _, arch := range []string{"amd64", "arm64"} {
		if want := "idp_${tag#v}_linux_" + arch + ".tar.gz"; !slices.Contains(assets, want) {
			t.Errorf("GoReleaser uploads %q; the download needs %q for the %s runner", assets, want, arch)
		}
	}
	if !slices.Contains(assets, "checksums.txt") {
		t.Errorf("GoReleaser uploads %q; the download needs checksums.txt", assets)
	}
	wantLines(t, "setup-idp download", dl.Run, []string{`archive="idp_${tag#v}_linux_${arch}.tar.gz"`})
	wantLines(t, "setup-idp download", dl.Run, []string{
		`if ! gh release download "$tag" --repo ` + engineRepo + ` --pattern "$archive" --pattern checksums.txt --dir "$dl" || [ ! -f "$dl/$archive" ] || [ ! -f "$dl/checksums.txt" ]; then`,
		`echo "::error::setup-idp: $IDP_VERSION is release $tag, but $archive and checksums.txt could not be downloaded from it. A release commit never falls back to a source build: finish the release's publish job or pin another release (docs/runbooks/releases.md)"`,
		"exit 1",
		"fi",
	})
	wantLines(t, "setup-idp download", dl.Run, []string{
		`if ! verified="$(cd "$dl" && sha256sum --check --ignore-missing checksums.txt)" || [ "$verified" != "$archive: OK" ]; then`,
		`echo "::error::setup-idp: $archive does not match the checksums.txt of release $tag"`,
		"exit 1",
		"fi",
	})
	wantLines(t, "setup-idp download", dl.Run, []string{
		`if ! gh attestation verify "$dl/$archive" --repo ` + engineRepo + ` --signer-workflow ` + engineRepo + "/" + releasePath + ` --source-digest "$IDP_VERSION" --source-ref refs/heads/main; then`,
		`echo "::error::setup-idp: the provenance of $archive does not show that release.yaml built it on main from $IDP_VERSION"`,
		"exit 1",
		"fi",
	})
	wantLines(t, "setup-idp download", dl.Run, []string{
		`tar -xzf "$dl/$archive" -C "$RUNNER_TEMP/idp-bin" idp`,
		`got="$("$RUNNER_TEMP/idp-bin/idp" version)"`,
		`if [ "$got" != "$tag" ]; then`,
		`echo "::error::setup-idp: the idp binary of release $tag prints version $got"`,
		"exit 1",
		"fi",
		`echo "$RUNNER_TEMP/idp-bin" >> "$GITHUB_PATH"`,
		`echo "installed=true" >> "$GITHUB_OUTPUT"`,
	})
	// Installing comes last: every check runs before the binary is announced.
	last := -1
	for _, marker := range []string{"gh release download", "sha256sum", "gh attestation verify", `tar -xzf`, `"$got" != "$tag"`, `installed=true`} {
		i := strings.Index(dl.Run, marker)
		if i <= last {
			t.Errorf("%q must come after the previous check in the download script", marker)
		}
		last = i
	}
}

func TestSetupIDPFailsWhenTheTagListingFails(t *testing.T) {
	_, dl := downloadStep(t)
	wantLines(t, "setup-idp download", dl.Run, []string{
		`if ! listing="$(gh api --paginate "repos/` + engineRepo + `/git/matching-refs/tags/v" --jq '.[] | [.ref, .object.type, .object.sha] | @tsv')"; then`,
		`echo "::error::setup-idp: could not list the idp-engine release tags, so it cannot tell whether $IDP_VERSION is a release; re-run the job"`,
		"exit 1",
		"fi",
	})
	// Only a successful listing with no release tag decides "build from source".
	wantLines(t, "setup-idp download", dl.Run, []string{
		`if [ "${#tags[@]}" -eq 0 ]; then`,
		`echo "setup-idp: $IDP_VERSION is not a release; building idp from source"`,
		`echo "installed=false" >> "$GITHUB_OUTPUT"`,
		"exit 0",
		"fi",
	})
	if strings.Index(dl.Run, `installed=false`) < strings.Index(dl.Run, `if ! listing=`) {
		t.Error("installed=false may be decided only after the listing succeeded")
	}
	// A failed download step must not let the build steps run: their if is exactly
	// notInstalled, with no always() or failure() to override the failure.
	a := loadAction(t, setupIDPPath)
	for _, id := range []string{"download", "go", "build"} {
		_, s := mustStep(t, setupIDPPath, a.Runs.Steps, id)
		if s.ContinueOnError != "" {
			t.Errorf("step %s continue-on-error = %q: a failed download must fail the job", id, s.ContinueOnError)
		}
		if id != "download" && s.If != notInstalled {
			t.Errorf("step %s if = %q, want %q", id, s.If, notInstalled)
		}
	}
}

func TestSetupIDPNeverBuildsATaggedCommit(t *testing.T) {
	_, dl := downloadStep(t)
	if n := strings.Count(dl.Run, `echo "installed=false" >> "$GITHUB_OUTPUT"`); n != 1 {
		t.Errorf("installed=false is written %d times, want once: only for a commit without a release tag", n)
	}
	i := strings.Index(dl.Run, `tag="${tags[0]}"`)
	if i < 0 {
		t.Fatal(`no tag="${tags[0]}" line`)
	}
	if strings.Contains(dl.Run[i:], "installed=false") {
		t.Error("once a release tag is found, the step may only install or fail")
	}
	// The failure of a release without its assets names the rule and the runbook;
	// TestSetupIDPDownloadsAndVerifiesTheRelease pins the whole block.
	if !strings.Contains(dl.Run, "never falls back to a source build") || !strings.Contains(dl.Run, "docs/runbooks/releases.md") {
		t.Error("a release without its assets must fail with a message that says it never falls back to a build and points to docs/runbooks/releases.md")
	}
}

func TestSetupIDPIgnoresNonReleaseTags(t *testing.T) {
	_, dl := downloadStep(t)
	re := bashMatch(t, "setup-idp download", dl.Run, "ref")
	for _, ref := range []string{"refs/tags/v0.1.0", "refs/tags/v12.3.45"} {
		if !re.MatchString(ref) {
			t.Errorf("the lookup ignores %q, a release tag", ref)
		}
	}
	for _, ref := range []string{"refs/tags/v0.1.0-rc.1", "refs/tags/0.1.0", "refs/tags/v0.1", "refs/tags/idp-v0.1.0", "refs/heads/v0.1.0", "refs/tags/v0.1.0/x"} {
		if re.MatchString(ref) {
			t.Errorf("the lookup takes %q as a release: its commit would wait for archives that never exist", ref)
		}
	}
	// Only a tag that points at this very commit counts.
	wantLines(t, "setup-idp download", dl.Run, []string{
		`if [[ "$type" == commit && "$sha" == "$IDP_VERSION" && "$ref" =~ ` + re.String() + ` ]]; then`,
		`tags+=("${ref#refs/tags/}")`,
		"fi",
	})
	wantLines(t, "setup-idp download", dl.Run, []string{
		`if [ "${#tags[@]}" -gt 1 ]; then`,
		`echo "::error::setup-idp: $IDP_VERSION carries several release tags (${tags[*]}); a release commit has exactly one"`,
		"exit 1",
		"fi",
	})
}

func TestSetupIDPPicksTheRunnerArchive(t *testing.T) {
	_, dl := downloadStep(t)
	wantLines(t, "setup-idp download", dl.Run, []string{
		`case "$RUNNER_ARCH" in`,
		`X64) arch=amd64 ;;`,
		`ARM64) arch=arm64 ;;`,
		`*) echo "::error::setup-idp: idp releases have no linux archive for runner architecture $RUNNER_ARCH"; exit 1 ;;`,
		"esac",
	})
}

func TestSetupIDPHoldsTheTokenOnlyInTheDownload(t *testing.T) {
	for _, s := range loadAction(t, setupIDPPath).Runs.Steps {
		if s.ID == "download" {
			continue
		}
		texts := []string{s.Run}
		texts = append(texts, slices.Collect(maps.Keys(s.Env))...)
		texts = append(texts, slices.Collect(maps.Values(s.Env))...)
		texts = append(texts, slices.Collect(maps.Values(s.With))...)
		for _, txt := range texts {
			if l := strings.ToLower(txt); strings.Contains(l, "github.token") || strings.Contains(l, "gh_token") {
				t.Errorf("step %s reads the job token (%q): only the download step may hold it (ADR-0023)", s.label(), txt)
			}
		}
	}
}
