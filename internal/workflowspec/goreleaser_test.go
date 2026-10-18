package workflowspec

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

const (
	goreleaserPath  = ".goreleaser.yaml"
	archiveTemplate = "idp_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
)

// goreleaserConfig is the part of .goreleaser.yaml the release contract reads.
type goreleaserConfig struct {
	Version int `yaml:"version"`
	Builds  []struct {
		Main    string   `yaml:"main"`
		Binary  string   `yaml:"binary"`
		Env     []string `yaml:"env"`
		Flags   []string `yaml:"flags"`
		Ldflags []string `yaml:"ldflags"`
		Goos    []string `yaml:"goos"`
		Goarch  []string `yaml:"goarch"`
	} `yaml:"builds"`
	Archives []struct {
		Formats         []string `yaml:"formats"`
		NameTemplate    string   `yaml:"name_template"`
		FormatOverrides []struct {
			Goos    string   `yaml:"goos"`
			Formats []string `yaml:"formats"`
		} `yaml:"format_overrides"`
	} `yaml:"archives"`
	Checksum struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"checksum"`
	Changelog struct {
		Disable bool `yaml:"disable"`
	} `yaml:"changelog"`
	Release struct {
		Mode string `yaml:"mode"`
	} `yaml:"release"`
}

func loadGoreleaser(t *testing.T) goreleaserConfig {
	t.Helper()
	var c goreleaserConfig
	readYAML(t, goreleaserPath, &c)
	if len(c.Builds) != 1 || len(c.Archives) != 1 {
		t.Fatalf("%s: %d builds and %d archives, want one of each: the idp CLI only (the harness is never released, ADR-0027)", goreleaserPath, len(c.Builds), len(c.Archives))
	}
	return c
}

// versionLdflag is the -X the release build sets. {{.Tag}} keeps the v:
// rendered module sources pin ?ref=<this value>, and GoReleaser's {{.Version}}
// (0.1.0) names no ref of the engine (Phase 1a, final review minor #8).
var versionLdflag = "-X " + version.Repo + "/internal/version.Version={{.Tag}}"

var exactVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

func TestGoreleaserBuildsOnlyTheCLI(t *testing.T) {
	c := loadGoreleaser(t)
	if c.Version != 2 {
		t.Errorf("version = %d, want 2", c.Version)
	}
	b := c.Builds[0]
	if b.Main != "./cmd/idp" || b.Binary != "idp" {
		t.Errorf("build main %q binary %q, want ./cmd/idp and idp", b.Main, b.Binary)
	}
	if !slices.Equal(b.Env, []string{"CGO_ENABLED=0"}) || !slices.Equal(b.Flags, []string{"-trimpath"}) {
		t.Errorf("env %v flags %v, want [CGO_ENABLED=0] and [-trimpath]", b.Env, b.Flags)
	}
	// The exact match also refuses {{.Version}}, which drops the v: rendered
	// module sources would pin ?ref=0.1.0, a ref the engine does not have.
	if want := []string{"-s -w " + versionLdflag}; !slices.Equal(b.Ldflags, want) {
		t.Errorf("ldflags = %q, want %q", b.Ldflags, want)
	}
	if !slices.Equal(b.Goos, []string{"darwin", "linux", "windows"}) || !slices.Equal(b.Goarch, []string{"amd64", "arm64"}) {
		t.Errorf("goos %v goarch %v, want darwin, linux, windows and amd64, arm64", b.Goos, b.Goarch)
	}
}

func TestGoreleaserUploadsIntoTheReleasePleaseRelease(t *testing.T) {
	c := loadGoreleaser(t)
	a := c.Archives[0]
	if a.NameTemplate != archiveTemplate {
		t.Errorf("archive name_template = %q, want %q: setup-idp downloads idp_<version>_linux_<arch>.tar.gz", a.NameTemplate, archiveTemplate)
	}
	if !slices.Equal(a.Formats, []string{"tar.gz"}) || len(a.FormatOverrides) != 1 || a.FormatOverrides[0].Goos != "windows" || !slices.Equal(a.FormatOverrides[0].Formats, []string{"zip"}) {
		t.Errorf("archive formats %v, overrides %+v; want tar.gz, and zip on windows", a.Formats, a.FormatOverrides)
	}
	if c.Checksum.NameTemplate != "checksums.txt" {
		t.Errorf("checksum name_template = %q, want checksums.txt", c.Checksum.NameTemplate)
	}
	if !c.Changelog.Disable {
		t.Error("changelog.disable must be true: release-please writes the release notes")
	}
	if c.Release.Mode != "keep-existing" {
		t.Errorf("release.mode = %q, want keep-existing: GoReleaser uploads into the release release-please created and keeps its notes", c.Release.Mode)
	}
}

func TestCIProvesTheGoreleaserConfig(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, ".github/workflows/ci.yaml"), "release-snapshot")
	where := "ci release-snapshot"
	wantPerms(t, where, j.Permissions, map[string]string{"contents": "read"})
	_, co := mustStep(t, where, j.Steps, "checkout")
	wantWith(t, where+" checkout", co, map[string]string{"fetch-depth": "0", "persist-credentials": "false"})
	_, gs := mustStep(t, where, j.Steps, "go")
	wantWith(t, where+" setup-go", gs, map[string]string{"go-version-file": "go.mod", "cache": "false"})
	ci, check := mustStep(t, where, j.Steps, "check")
	si, snap := mustStep(t, where, j.Steps, "snapshot")
	ti, tc := mustStep(t, where, j.Steps, "tag-check")
	if !(ci < si && si < ti) {
		t.Errorf("steps out of order: check %d, snapshot %d, tag-check %d", ci, si, ti)
	}
	for _, s := range []step{check, snap} {
		if !strings.HasPrefix(s.Uses, "goreleaser/goreleaser-action@") || s.With["distribution"] != "goreleaser" {
			t.Errorf("%s: want goreleaser/goreleaser-action with distribution goreleaser, got %q %v", s.ID, s.Uses, s.With)
		}
		if !exactVersion.MatchString(s.With["version"]) {
			t.Errorf("%s: GoReleaser version %q must be an exact vX.Y.Z pin (spec §7.5)", s.ID, s.With["version"])
		}
		if len(s.Env) != 0 {
			t.Errorf("%s env = %v: the snapshot publishes nothing and needs no token", s.ID, s.Env)
		}
	}
	if check.With["version"] != snap.With["version"] {
		t.Errorf("check runs GoReleaser %q, snapshot %q: keep one version", check.With["version"], snap.With["version"])
	}
	if check.With["args"] != "check" || snap.With["args"] != "release --snapshot --clean --skip=publish" {
		t.Errorf("args = %q and %q, want check and release --snapshot --clean --skip=publish", check.With["args"], snap.With["args"])
	}
	wantRun(t, where+" tag-check", tc,
		`tar -xzf "${archives[0]}" -C "$RUNNER_TEMP/idp-check"`,
		`got="$("$RUNNER_TEMP/idp-check/idp" version)"`)
	re := bashMatch(t, where+" tag-check", tc.Run, "got")
	if !re.MatchString("v0.0.0") || re.MatchString("0.0.0-SNAPSHOT-abc1234") {
		t.Errorf("tag-check pattern %q must accept a v-tag and refuse a {{.Version}} value", re)
	}
}
