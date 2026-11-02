package workflowspec

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const releasePath = ".github/workflows/release.yaml"

// secretReaders lists the steps of a job that read secret, and the job-level
// env that does; a step that only tests whether the secret is set (it compares
// the secret with the empty string) does not count. Expression contexts are
// case-insensitive, so the match is too.
func secretReaders(j *job, secret string) []string {
	ref := strings.ToLower("secrets." + secret)
	var out []string
	for _, k := range slices.Sorted(maps.Keys(j.Env)) {
		if strings.Contains(strings.ToLower(j.Env[k]), ref) {
			out = append(out, "job env "+k)
		}
	}
	for _, s := range j.Steps {
		texts := []string{s.Run}
		texts = append(texts, slices.Collect(maps.Values(s.Env))...)
		texts = append(texts, slices.Collect(maps.Values(s.With))...)
		for _, txt := range texts {
			if strings.ToLower(txt) == "${{ "+ref+" != '' }}" {
				continue
			}
			if strings.Contains(strings.ToLower(txt), ref) {
				out = append(out, s.label())
				break
			}
		}
	}
	return out
}

// blockLines splits a block scalar into its trimmed, non-empty lines.
func blockLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func TestReleaseWorkflow(t *testing.T) {
	w := loadWorkflow(t, releasePath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"push", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want push and workflow_dispatch", got)
	}
	n, ok := w.trigger("push")
	if !ok {
		t.Fatal("no push trigger")
	}
	var push map[string][]string
	if err := n.Decode(&push); err != nil || len(push) != 1 || !slices.Equal(push["branches"], []string{"main"}) {
		t.Errorf("push = %v (%v), want exactly branches: [main]: a paths or tags filter would skip some pushes, and a skipped push drops its release", push, err)
	}
	if w.Concurrency == nil || *w.Concurrency != (concurrency{Group: "idp-release", Queue: "max"}) {
		t.Errorf("concurrency = %+v, want group idp-release, no cancellation, queue max: a dropped run of the release merge would leave its release to the run of another commit", w.Concurrency)
	}
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"e2e-release-pr", "publish", "release-please"}) {
		t.Errorf("jobs = %v, want e2e-release-pr, publish and release-please", got)
	}
}

func TestReleasePleaseRunsWithTheBotToken(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, releasePath), "release-please")
	if j.Environment != "idp-release" {
		t.Errorf("environment = %q, want idp-release: the bot key lives only in idp-engine environments limited to main (ADR-0026)", j.Environment)
	}
	wantPerms(t, "release-please", j.Permissions, map[string]string{})
	_, bot := mustStep(t, "release-please", j.Steps, "bot")
	wantBotToken(t, "release-please bot", bot, "idp-engine", map[string]string{
		"contents":      "write",
		"issues":        "write",
		"pull-requests": "write",
	})
	_, rp := mustStep(t, "release-please", j.Steps, "release")
	if !strings.HasPrefix(rp.Uses, "googleapis/release-please-action@") || !maps.Equal(map[string]string(rp.With), map[string]string{"token": "${{ steps.bot.outputs.token }}"}) {
		t.Errorf("release step = %q %v, want release-please-action with only the bot token: a GITHUB_TOKEN pull request would get no CI", rp.Uses, rp.With)
	}
	wantOutputs := map[string]string{
		"release_created": "${{ steps.release.outputs.release_created }}",
		"tag_name":        "${{ steps.release.outputs.tag_name }}",
		"sha":             "${{ steps.release.outputs.sha }}",
		"prs_created":     "${{ steps.release.outputs.prs_created }}",
		"pr":              "${{ steps.release.outputs.pr }}",
	}
	if !maps.Equal(map[string]string(j.Outputs), wantOutputs) {
		t.Errorf("outputs = %v, want %v", j.Outputs, wantOutputs)
	}
	if got := secretReaders(j, "IDP_BOT_PRIVATE_KEY"); !slices.Equal(got, []string{"bot"}) {
		t.Errorf("steps reading the bot key = %v, want only the token step", got)
	}
}

func TestReleasePleaseNamesAMissingCredential(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, releasePath), "release-please")
	if len(j.Steps) == 0 || j.Steps[0].ID != "config" {
		t.Fatal("the first step must check the environment, so a missing credential fails with its name instead of a token error")
	}
	c := j.Steps[0]
	wantEnv(t, "release-please config", c, map[string]string{
		"BOT_CLIENT_ID": "${{ vars.IDP_BOT_CLIENT_ID }}",
		"BOT_KEY_SET":   "${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}",
	})
	wantRun(t, "release-please config", c,
		`missing="$missing variable IDP_BOT_CLIENT_ID,"`,
		`missing="$missing secret IDP_BOT_PRIVATE_KEY,"`,
		`echo "::error::the idp-release environment is missing:${missing%,} (docs/runbooks/releases.md)"`,
		"exit 1")
}

func TestReleasePublishesOnlyACreatedRelease(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, releasePath), "publish")
	if !slices.Equal(j.Needs, strlist{"release-please"}) || j.If != "needs.release-please.outputs.release_created == 'true'" {
		t.Errorf("needs %v if %q, want release-please and release_created == 'true' (spec §9.1)", j.Needs, j.If)
	}
	if j.Environment != "" {
		t.Errorf("environment = %q: publishing holds no bot key", j.Environment)
	}
	wantPerms(t, "publish", j.Permissions, map[string]string{"attestations": "write", "contents": "write", "id-token": "write"})
	gi, guard := mustStep(t, "publish", j.Steps, "guard")
	_, co := mustStep(t, "publish", j.Steps, "checkout")
	_, gs := mustStep(t, "publish", j.Steps, "go")
	ri, rel := mustStep(t, "publish", j.Steps, "goreleaser")
	vi, check := mustStep(t, "publish", j.Steps, "version")
	ai, att := mustStep(t, "publish", j.Steps, "attest")
	if gi != 0 || !(ri < vi && vi < ai) {
		t.Errorf("steps: guard %d, goreleaser %d, version %d, attest %d; want the guard first, then GoReleaser, the version check and the attestation", gi, ri, vi, ai)
	}
	wantEnv(t, "publish guard", guard, map[string]string{
		"TAG":         "${{ needs.release-please.outputs.tag_name }}",
		"RELEASE_SHA": "${{ needs.release-please.outputs.sha }}",
	})
	// The whole guard script, so that no fail-stop can be dropped unseen. The
	// tag test and its error are pinned by TestReleasePublishRefusesANonSemverTag.
	wantLines(t, "publish guard", guard.Run, []string{
		`if [ "$RELEASE_SHA" != "$GITHUB_SHA" ]; then`,
		`echo "::error::release $TAG tags $RELEASE_SHA, but this run is for $GITHUB_SHA: its attestation would name the wrong commit (docs/runbooks/releases.md#publishing-failed)"`,
		"exit 1",
		"fi",
		`echo "version=${TAG#v}" >> "$GITHUB_OUTPUT"`,
	})
	for _, s := range []step{guard, check, att} {
		if s.If != "" || s.ContinueOnError != "" {
			t.Errorf("publish %s: if %q, continue-on-error %q; a publish step must run, and stop the job when it fails", s.ID, s.If, s.ContinueOnError)
		}
	}
	wantWith(t, "publish checkout", co, map[string]string{"ref": "${{ needs.release-please.outputs.sha }}", "fetch-depth": "0", "persist-credentials": "false"})
	wantWith(t, "publish setup-go", gs, map[string]string{"go-version-file": "go.mod", "cache": "false"})
	if !strings.HasPrefix(rel.Uses, "goreleaser/goreleaser-action@") {
		t.Errorf("goreleaser step uses %q", rel.Uses)
	}
	wantWith(t, "publish goreleaser", rel, map[string]string{"distribution": "goreleaser", "version": ciGoreleaserVersion(t), "args": "release --clean"})
	if want := map[string]string{"GITHUB_TOKEN": "${{ github.token }}", "GORELEASER_CURRENT_TAG": "${{ needs.release-please.outputs.tag_name }}"}; !maps.Equal(map[string]string(rel.Env), want) {
		t.Errorf("goreleaser env = %v, want %v: it uploads with the workflow token, never a personal one (spec §9.1)", rel.Env, want)
	}
	wantEnv(t, "publish version", check, map[string]string{
		"TAG":     "${{ needs.release-please.outputs.tag_name }}",
		"VERSION": "${{ steps.guard.outputs.version }}",
	})
	wantRun(t, "publish version", check,
		`tar -xzf "dist/idp_${VERSION}_linux_amd64.tar.gz" -C "$RUNNER_TEMP/idp-check"`,
		`got="$("$RUNNER_TEMP/idp-check/idp" version)"`)
	wantLines(t, "publish version", check.Run, []string{
		`if [ "$got" != "$TAG" ]; then`,
		`echo "::error::the linux/amd64 binary of $TAG prints $got: the version ldflag must use {{.Tag}} (.goreleaser.yaml)"`,
		"exit 1",
		"fi",
	})
	if !strings.HasPrefix(att.Uses, "actions/attest-build-provenance@") {
		t.Errorf("attest step uses %q", att.Uses)
	}
	var want []string
	for _, a := range releaseAssets(t, "${{ steps.guard.outputs.version }}") {
		want = append(want, "dist/"+a)
	}
	if got := blockLines(att.With["subject-path"]); !slices.Equal(got, want) {
		t.Errorf("attested subjects = %v, want every uploaded file %v", got, want)
	}
}

func TestReleasePublishRefusesANonSemverTag(t *testing.T) {
	_, guard := mustStep(t, "publish", mustJob(t, loadWorkflow(t, releasePath), "publish").Steps, "guard")
	re := bashMatch(t, "publish guard", guard.Run, "TAG")
	for _, tag := range []string{"v0.1.0", "v1.2.3", "v10.20.30"} {
		if !re.MatchString(tag) {
			t.Errorf("the guard refuses %q, a release tag", tag)
		}
	}
	for _, tag := range []string{"v0.1.0-rc.1", "0.1.0", "v0.1", "v0.1.0+build", "idp-v0.1.0", "v0.1.0 "} {
		if re.MatchString(tag) {
			t.Errorf("the guard accepts %q: rendered module sources would pin it, and only vX.Y.Z is an engine release", tag)
		}
	}
	// The tag test must be the first thing the guard does and must fail the step.
	lines := blockLines(guard.Run)
	wantTest := `if ! [[ "$TAG" =~ ` + re.String() + ` ]]; then`
	if len(lines) < 4 || lines[0] != wantTest || !strings.HasPrefix(lines[1], `echo "::error::refusing to publish $TAG`) || lines[2] != "exit 1" || lines[3] != "fi" {
		t.Errorf("guard starts with %q, want the tag test %q, its error, exit 1 and fi", lines[:min(4, len(lines))], wantTest)
	}
}

func TestReleasePRGetsTheHarness(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, releasePath), "e2e-release-pr")
	if !slices.Equal(j.Needs, strlist{"release-please"}) || j.If != "needs.release-please.outputs.prs_created == 'true'" {
		t.Errorf("needs %v if %q, want release-please and prs_created == 'true'", j.Needs, j.If)
	}
	wantPerms(t, "e2e-release-pr", j.Permissions, map[string]string{"actions": "write", "pull-requests": "read"})
	_, d := mustStep(t, "e2e-release-pr", j.Steps, "dispatch")
	if want := map[string]string{"GH_TOKEN": "${{ github.token }}", "PR_NUMBER": "${{ fromJSON(needs.release-please.outputs.pr).number }}"}; !maps.Equal(map[string]string(d.Env), want) {
		t.Errorf("dispatch env = %v, want %v", d.Env, want)
	}
	wantRun(t, "e2e-release-pr dispatch", d,
		`head="$(gh pr view "$PR_NUMBER" --repo "$GITHUB_REPOSITORY" --json headRefOid --jq .headRefOid)"`,
		`gh workflow run e2e.yaml --repo "$GITHUB_REPOSITORY" --ref main -f release-pr-sha="$head"`)
	re := bashMatch(t, "e2e-release-pr dispatch", d.Run, "head")
	if !re.MatchString(strings.Repeat("a", 40)) || !re.MatchString("0123456789abcdef0123456789abcdef01234567") {
		t.Errorf("head pattern %q must accept a full commit SHA", re)
	}
	for _, bad := range []string{"", strings.Repeat("a", 39), strings.Repeat("a", 41), strings.Repeat("A", 40), strings.Repeat("g", 40)} {
		if re.MatchString(bad) {
			t.Errorf("head pattern %q accepts %q, want only a full lowercase commit SHA", re, bad)
		}
	}
	// Each check must stop the job: a dispatch with a bad ref would run the harness on nothing.
	wantLines(t, "e2e-release-pr dispatch", d.Run, []string{
		`if ! [[ "$PR_NUMBER" =~ ^[0-9]+$ ]]; then`,
		`echo "::error::release-please reported a release PR without a number"`,
		"exit 1",
		"fi",
	})
	wantLines(t, "e2e-release-pr dispatch", d.Run, []string{
		`if ! [[ "$head" =~ ^[0-9a-f]{40}$ ]]; then`,
		`echo "::error::could not read the head commit of release PR #$PR_NUMBER"`,
		"exit 1",
		"fi",
	})
}

func TestReleasePleaseConfig(t *testing.T) {
	var c struct {
		Schema       string `json:"$schema"`
		AlwaysUpdate bool   `json:"always-update"`
		Packages     map[string]struct {
			ReleaseType           string `json:"release-type"`
			InitialVersion        string `json:"initial-version"`
			BumpMinorPreMajor     bool   `json:"bump-minor-pre-major"`
			IncludeComponentInTag *bool  `json:"include-component-in-tag"`
			IncludeVInTag         *bool  `json:"include-v-in-tag"`
			ChangelogSections     []struct {
				Type    string `json:"type"`
				Section string `json:"section"`
				Hidden  bool   `json:"hidden"`
			} `json:"changelog-sections"`
		} `json:"packages"`
	}
	strictJSON(t, "release-please-config.json", &c)
	if !c.AlwaysUpdate {
		t.Error("always-update must be true: every push to main rebuilds the release PR and re-runs the harness on it")
	}
	p, ok := c.Packages["."]
	if !ok || len(c.Packages) != 1 {
		t.Fatalf("packages = %v, want exactly the root package: one version for everything (spec §3.1)", slices.Sorted(maps.Keys(c.Packages)))
	}
	if p.ReleaseType != "go" || p.InitialVersion != "0.1.0" || !p.BumpMinorPreMajor {
		t.Errorf("release-type %q initial-version %q bump-minor-pre-major %v; want go, 0.1.0 (release-please's own default is 1.0.0) and true (spec §9.1)", p.ReleaseType, p.InitialVersion, p.BumpMinorPreMajor)
	}
	if p.IncludeComponentInTag == nil || *p.IncludeComponentInTag || p.IncludeVInTag == nil || !*p.IncludeVInTag {
		t.Error("tags must be vX.Y.Z: include-component-in-tag false, include-v-in-tag true")
	}
	var visible, hidden, titles []string
	for _, s := range p.ChangelogSections {
		titles = append(titles, s.Type+"="+s.Section)
		if s.Hidden {
			hidden = append(hidden, s.Type)
		} else {
			visible = append(visible, s.Type)
		}
	}
	wantTitles := []string{"feat=Features", "fix=Bug Fixes", "perf=Performance Improvements", "refactor=Code Refactoring", "docs=Documentation", "revert=Reverts", "chore=Miscellaneous Chores", "test=Tests"}
	if !slices.Equal(titles, wantTitles) {
		t.Errorf("changelog sections (type=title) = %q, want %q", titles, wantTitles)
	}
	if !slices.Equal(visible, []string{"feat", "fix", "perf", "refactor", "docs", "revert"}) || !slices.Equal(hidden, []string{"chore", "test"}) {
		t.Errorf("changelog sections: visible %v hidden %v", visible, hidden)
	}
	var m map[string]string
	strictJSON(t, ".release-please-manifest.json", &m)
	if len(m) != 1 || !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(m["."]) {
		t.Errorf("manifest = %v, want only the root package at an X.Y.Z version", m)
	}
}

// wantLines checks that the block lines of a script contain want, in order and
// with nothing between them: a fail-stop is the test, its error, exit 1 and fi.
func wantLines(t *testing.T, where, run string, want []string) {
	t.Helper()
	got := blockLines(run)
	for i := 0; i+len(want) <= len(got); i++ {
		if slices.Equal(got[i:i+len(want)], want) {
			return
		}
	}
	t.Errorf("%s: the script lacks the consecutive lines %q", where, want)
}

// strictJSON decodes a repo-relative JSON file into out, refusing unknown keys.
func strictJSON(t *testing.T, rel string, out any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		t.Fatalf("decode %s: %v (every key must be known to this test)", rel, err)
	}
}
