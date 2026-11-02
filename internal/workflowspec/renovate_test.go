package workflowspec

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/render"
)

const (
	renovatePath       = ".github/workflows/renovate.yaml"
	renovateConfigPath = ".github/renovate-global.json"
	// botGitAuthor is the bot App's identity; GitHub authors its API commits as it.
	botGitAuthor = "jellalshadows-idp-bot[bot] <343234638+jellalshadows-idp-bot[bot]@users.noreply.github.com>"
)

// renovateConfig is .github/renovate-global.json. Decoding refuses unknown keys,
// so every option in the file is one this contract knows.
type renovateConfig struct {
	Platform             string             `json:"platform"`
	Repositories         []string           `json:"repositories"`
	Onboarding           *bool              `json:"onboarding"`
	RequireConfig        string             `json:"requireConfig"`
	PlatformCommit       string             `json:"platformCommit"`
	GitAuthor            string             `json:"gitAuthor"`
	DependencyDashboard  *bool              `json:"dependencyDashboard"`
	Automerge            *bool              `json:"automerge"`
	MinimumReleaseAge    string             `json:"minimumReleaseAge"`
	PRHourlyLimit        *int               `json:"prHourlyLimit"`
	PRConcurrentLimit    *int               `json:"prConcurrentLimit"`
	InternalChecksFilter string             `json:"internalChecksFilter"`
	StatusCheckNames     map[string]*string `json:"statusCheckNames"`
	EnabledManagers      []string           `json:"enabledManagers"`
	PackageRules         []renovateRule     `json:"packageRules"`
	CustomManagers       []customManager    `json:"customManagers"`
}

type renovateRule struct {
	Description       string          `json:"description"`
	MatchPackageNames []string        `json:"matchPackageNames"`
	MatchDepNames     []string        `json:"matchDepNames"`
	MatchDepTypes     []string        `json:"matchDepTypes"`
	MatchManagers     []string        `json:"matchManagers"`
	MatchDatasources  []string        `json:"matchDatasources"`
	MatchUpdateTypes  []string        `json:"matchUpdateTypes"`
	GroupName         string          `json:"groupName"`
	ExtractVersion    string          `json:"extractVersion"`
	Labels            []string        `json:"labels"`
	RegistryURLs      []string        `json:"registryUrls"`
	PostUpdateOptions []string        `json:"postUpdateOptions"`
	RangeStrategy     string          `json:"rangeStrategy"`
	Enabled           *bool           `json:"enabled"`
	MinimumReleaseAge json.RawMessage `json:"minimumReleaseAge"` // absent (nil), null, or a duration
}

type customManager struct {
	CustomType          string   `json:"customType"`
	Description         string   `json:"description"`
	ManagerFilePatterns []string `json:"managerFilePatterns"`
	MatchStrings        []string `json:"matchStrings"`
	DatasourceTemplate  string   `json:"datasourceTemplate"`
	DepNameTemplate     string   `json:"depNameTemplate"`
	PackageNameTemplate string   `json:"packageNameTemplate"`
	VersioningTemplate  string   `json:"versioningTemplate"`
	RegistryURLTemplate string   `json:"registryUrlTemplate"`
}

func loadRenovateConfig(t *testing.T) renovateConfig {
	t.Helper()
	var c renovateConfig
	strictJSON(t, renovateConfigPath, &c)
	return c
}

// renovateVersion is the Renovate version renovate.yaml runs.
func renovateVersion(t *testing.T) string {
	t.Helper()
	_, r := mustStep(t, "renovate", mustJob(t, loadWorkflow(t, renovatePath), "renovate").Steps, "renovate")
	v := r.With["renovate-version"]
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(v) {
		t.Fatalf("renovate-version = %q, want an exact X.Y.Z (spec §7.5)", v)
	}
	return v
}

func TestRenovateWorkflow(t *testing.T) {
	w := loadWorkflow(t, renovatePath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"schedule", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want schedule and workflow_dispatch", got)
	}
	n, ok := w.trigger("schedule")
	if !ok {
		t.Fatal("no schedule")
	}
	var crons []struct {
		Cron string `yaml:"cron"`
	}
	if err := n.Decode(&crons); err != nil || len(crons) != 1 || crons[0].Cron != "17 4 * * 1" {
		t.Errorf("schedule = %v (%v), want weekly '17 4 * * 1' (owner decision)", crons, err)
	}
	if w.Concurrency == nil || *w.Concurrency != (concurrency{Group: "idp-renovate"}) {
		t.Errorf("concurrency = %+v, want group idp-renovate without cancellation", w.Concurrency)
	}
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"renovate"}) {
		t.Errorf("jobs = %v, want renovate", got)
	}
	j := mustJob(t, w, "renovate")
	if j.Environment != "idp-renovate" {
		t.Errorf("environment = %q, want idp-renovate: the bot key lives only in idp-engine environments limited to main (ADR-0026)", j.Environment)
	}
	wantPerms(t, "renovate", j.Permissions, map[string]string{"contents": "read"})
	ci, c := mustStep(t, "renovate", j.Steps, "config")
	co, _ := mustStep(t, "renovate", j.Steps, "checkout")
	bi, bot := mustStep(t, "renovate", j.Steps, "bot")
	ri, r := mustStep(t, "renovate", j.Steps, "renovate")
	if ci != 0 || !(co < bi && bi < ri) {
		t.Errorf("steps: config %d, checkout %d, bot %d, renovate %d; want the environment check first, then checkout, the token and Renovate", ci, co, bi, ri)
	}
	for _, s := range j.Steps {
		if s.If != "" || s.ContinueOnError != "" {
			t.Errorf("step %s: if %q, continue-on-error %q; every step must run, and stop the job when it fails", s.label(), s.If, s.ContinueOnError)
		}
	}
	if len(w.Env) != 0 || len(j.Env) != 0 {
		t.Errorf("workflow env %v, job env %v: the token and the bot key are set on their own steps only", w.Env, j.Env)
	}
	wantBotToken(t, "renovate bot", bot, "idp-engine,idp-claims", map[string]string{
		"checks":        "read",
		"contents":      "write",
		"issues":        "write",
		"pull-requests": "write",
		"statuses":      "read",
		"workflows":     "write",
	})
	if !strings.HasPrefix(r.Uses, "renovatebot/github-action@") {
		t.Errorf("renovate step uses %q", r.Uses)
	}
	if want := map[string]string{"configurationFile": renovateConfigPath, "renovate-version": renovateVersion(t), "token": "${{ steps.bot.outputs.token }}"}; !maps.Equal(map[string]string(r.With), want) {
		t.Errorf("renovate with = %v, want %v", r.With, want)
	}
	if len(r.Env) != 0 {
		t.Errorf("renovate env = %v, want none: the token comes through with.token", r.Env)
	}
	if got := secretReaders(j, "IDP_BOT_PRIVATE_KEY"); !slices.Equal(got, []string{"bot"}) {
		t.Errorf("steps reading the bot key = %v, want only the token step", got)
	}
	if want := map[string]string{"BOT_CLIENT_ID": "${{ vars.IDP_BOT_CLIENT_ID }}", "BOT_KEY_SET": "${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}"}; !maps.Equal(map[string]string(c.Env), want) {
		t.Errorf("renovate config env = %v, want %v", c.Env, want)
	}
	// The whole check, so that no fail-stop can be dropped unseen.
	wantLines(t, "renovate config", c.Run, []string{
		`missing=""`,
		`[ -n "$BOT_CLIENT_ID" ] || missing="$missing variable IDP_BOT_CLIENT_ID,"`,
		`[ "$BOT_KEY_SET" = true ] || missing="$missing secret IDP_BOT_PRIVATE_KEY,"`,
		`if [ -n "$missing" ]; then`,
		`echo "::error::the idp-renovate environment is missing:${missing%,} (docs/runbooks/dependencies.md)"`,
		"exit 1",
		"fi",
	})
}

func TestRenovateCoversExactlyTheTokenRepositories(t *testing.T) {
	_, bot := mustStep(t, "renovate", mustJob(t, loadWorkflow(t, renovatePath), "renovate").Steps, "bot")
	var want []string
	for _, r := range strings.Split(bot.With["repositories"], ",") {
		want = append(want, "jellalshadows-idp/"+strings.TrimSpace(r))
	}
	got := loadRenovateConfig(t).Repositories
	if !slices.Equal(got, want) {
		t.Errorf("repositories = %v, want the token's %v", got, want)
	}
	if slices.Contains(got, "jellalshadows-idp/idp-claims-e2e") {
		t.Error("idp-claims-e2e is left out: the harness pins its callers on every run (owner decision)")
	}
}

func TestRenovateGlobalConfig(t *testing.T) {
	c := loadRenovateConfig(t)
	if c.Platform != "github" || c.RequireConfig != "ignored" || c.PlatformCommit != "enabled" {
		t.Errorf("platform %q requireConfig %q platformCommit %q, want github, ignored, enabled", c.Platform, c.RequireConfig, c.PlatformCommit)
	}
	if c.Onboarding == nil || *c.Onboarding || c.DependencyDashboard == nil || !*c.DependencyDashboard {
		t.Error("onboarding must be false and dependencyDashboard true")
	}
	if c.PRHourlyLimit == nil || *c.PRHourlyLimit != 0 {
		t.Errorf("prHourlyLimit = %v, want 0: the weekly run opens every update, and engine bumps do not wait", c.PRHourlyLimit)
	}
	if c.PRConcurrentLimit == nil || *c.PRConcurrentLimit != 0 {
		t.Errorf("prConcurrentLimit = %v, want 0: the default of 10 would make the engine bump wait behind open third-party PRs", c.PRConcurrentLimit)
	}
	if c.GitAuthor != botGitAuthor {
		t.Errorf("gitAuthor = %q, want %q", c.GitAuthor, botGitAuthor)
	}
	if !slices.Equal(c.EnabledManagers, []string{"github-actions", "gomod", "terraform", "custom.regex"}) {
		t.Errorf("enabledManagers = %v", c.EnabledManagers)
	}
	if got := slices.Sorted(maps.Keys(c.StatusCheckNames)); !slices.Equal(got, []string{"artifactError", "configValidation", "mergeConfidence", "minimumReleaseAge"}) {
		t.Errorf("statusCheckNames keys = %v, want all four", got)
	}
	for k, v := range c.StatusCheckNames {
		if v != nil {
			t.Errorf("statusCheckNames.%s = %q, want null: the bot cannot write statuses, and a failed status write stops Renovate's run of the repository", k, *v)
		}
	}
}

func TestRenovateNeverAutomerges(t *testing.T) {
	c := loadRenovateConfig(t)
	if c.Automerge == nil || *c.Automerge {
		t.Error("automerge must be false: the owner merges every Renovate PR (owner decision)")
	}
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(renovateConfigPath)))
	if err != nil {
		t.Fatal(err)
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, x := range v {
				if (k == "automerge" || k == "platformAutomerge") && x == true {
					t.Errorf("%s.%s is true", path, k)
				}
				walk(path+"."+k, x)
			}
		case []any:
			for _, x := range v {
				walk(path+"[]", x)
			}
		}
	}
	walk("$", raw)
}

func TestRenovateWaitsOnlyForThirdPartyReleases(t *testing.T) {
	c := loadRenovateConfig(t)
	if c.MinimumReleaseAge != "14 days" || c.InternalChecksFilter != "strict" {
		t.Errorf("minimumReleaseAge %q internalChecksFilter %q, want 14 days and strict (spec §7.5)", c.MinimumReleaseAge, c.InternalChecksFilter)
	}
}

func TestRenovateMovesTheEnginePinsTogether(t *testing.T) {
	rules := loadRenovateConfig(t).PackageRules
	n := 0
	for _, name := range templateNames {
		for _, u := range usesRefs(readYAML(t, templatesDir+name, &struct{}{})) {
			if !strings.HasPrefix(u.value, "jellalshadows-idp/idp-engine/") {
				continue
			}
			n++
			const pkg = "jellalshadows-idp/idp-engine" // what Renovate extracts from owner/repo/path@ref
			var groups []string
			for _, r := range rules {
				if slices.Contains(r.MatchPackageNames, pkg) {
					groups = append(groups, r.GroupName)
				}
			}
			if !slices.Equal(groups, []string{"idp engine"}) {
				t.Errorf("%s pin %q: rules for %s group it as %q, want one rule, group \"idp engine\": the four callers must move in one PR, or a claims repo would pin two engine commits", name, u.value, pkg, groups)
			}
		}
	}
	if n != 4 {
		t.Errorf("the templates pin the engine %d times, want 4: one group moves them all", n)
	}
}

func TestRenovateRules(t *testing.T) {
	f := false
	want := []renovateRule{
		{MatchPackageNames: []string{"jellalshadows-idp/idp-engine"}, GroupName: "idp engine", MinimumReleaseAge: json.RawMessage("null")},
		{MatchPackageNames: []string{"opentofu/opentofu"}, GroupName: "opentofu", ExtractVersion: "^v(?<version>.+)$"},
		{MatchPackageNames: []string{"integrations/github"}, GroupName: "github provider", Labels: []string{"needs-lockfile"}},
		{MatchDatasources: []string{"terraform-provider"}, RegistryURLs: []string{"https://registry.opentofu.org"}},
		{MatchManagers: []string{"terraform"}, MatchDepTypes: []string{"required_version"}, Enabled: &f},
		{MatchManagers: []string{"gomod"}, PostUpdateOptions: []string{"gomodTidy"}},
		{MatchManagers: []string{"gomod"}, MatchDepTypes: []string{"golang"}, MatchDepNames: []string{"go"}, RangeStrategy: "bump"},
		{MatchManagers: []string{"gomod"}, MatchDepTypes: []string{"golang"}, MatchDepNames: []string{"go"}, MatchUpdateTypes: []string{"minor", "major"}, Enabled: &f},
		{MatchDatasources: []string{"github-runners"}, Enabled: &f},
		{MatchManagers: []string{"github-actions"}, MatchDatasources: []string{"docker"}, MatchPackageNames: []string{"ghcr.io/renovatebot/renovate"}, Enabled: &f},
	}
	got := slices.Clone(loadRenovateConfig(t).PackageRules)
	for i := range got {
		if got[i].Description == "" {
			t.Errorf("packageRules[%d] has no description: say why the rule exists", i)
		}
		got[i].Description = ""
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("packageRules =\n%+v\nwant\n%+v", got, want)
	}
}

func TestRenovateCustomManagers(t *testing.T) {
	type id struct{ dep, datasource, pkg, registry string }
	var got []id
	for _, m := range loadRenovateConfig(t).CustomManagers {
		if m.CustomType != "regex" || m.Description == "" {
			t.Errorf("custom manager %q: customType %q, description %q; want regex and a description", m.DepNameTemplate, m.CustomType, m.Description)
		}
		got = append(got, id{m.DepNameTemplate, m.DatasourceTemplate, m.PackageNameTemplate, m.RegistryURLTemplate})
	}
	want := []id{
		{"github.com/rhysd/actionlint", "go", "", ""},
		{"zizmor", "pypi", "", ""},
		{"renovatebot/renovate", "github-releases", "", ""},
		{"opentofu", "github-releases", "opentofu/opentofu", ""},
		{"github", "terraform-provider", "integrations/github", "https://registry.opentofu.org"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("custom managers = %v, want %v", got, want)
	}
}

// TestRenovateCustomManagersFindThePins runs each regex manager over the files
// it matches, as Renovate would, and checks it finds each pin it exists for.
func TestRenovateCustomManagersFindThePins(t *testing.T) {
	files := repoFiles(t)
	found := map[string][]string{} // depName -> "file=value", sorted
	for _, m := range loadRenovateConfig(t).CustomManagers {
		var pats []*regexp.Regexp
		for _, p := range m.ManagerFilePatterns {
			if len(p) < 2 || p[0] != '/' || p[len(p)-1] != '/' {
				t.Errorf("%s: managerFilePatterns entry %q is not a /regex/", m.DepNameTemplate, p)
				continue
			}
			pats = append(pats, regexp.MustCompile(p[1:len(p)-1]))
		}
		for _, f := range files {
			if !slices.ContainsFunc(pats, func(re *regexp.Regexp) bool { return re.MatchString(f) }) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(f)))
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range m.MatchStrings {
				re := regexp.MustCompile(s)
				i := re.SubexpIndex("currentValue")
				if i < 0 {
					t.Errorf("%q has no currentValue group", s)
					continue
				}
				for _, sub := range re.FindAllStringSubmatch(string(b), -1) {
					found[m.DepNameTemplate] = append(found[m.DepNameTemplate], f+"="+sub[i])
				}
			}
		}
	}
	for _, v := range found {
		slices.Sort(v)
	}
	ci, rn := ".github/workflows/ci.yaml", renovatePath
	golden := "internal/render/testdata/basic/expected/github/main.tf.json"
	values := func(dep string) []string {
		var out []string
		for _, e := range found[dep] {
			out = append(out, e[strings.Index(e, "=")+1:])
		}
		return slices.Compact(slices.Sorted(slices.Values(out)))
	}
	actionlint := values("github.com/rhysd/actionlint")
	zizmor := values("zizmor")
	if len(actionlint) != 1 || len(zizmor) != 1 {
		t.Fatalf("actionlint versions %v, zizmor versions %v: want one each", actionlint, zizmor)
	}
	want := map[string][]string{
		"github.com/rhysd/actionlint": {ci + "=" + actionlint[0], ci + "=" + actionlint[0]},
		"zizmor":                      {ci + "=" + zizmor[0]},
		"renovatebot/renovate":        {ci + "=" + renovateVersion(t), rn + "=" + renovateVersion(t)},
		"opentofu":                    {"internal/render/render.go=" + render.TofuVersion, golden + "=" + render.TofuVersion},
		"github":                      {"internal/render/render.go=" + render.GitHubProviderVersion, golden + "=" + render.GitHubProviderVersion},
	}
	if !reflect.DeepEqual(found, want) {
		t.Errorf("regex managers found\n%v\nwant\n%v", found, want)
	}
}

func TestOneOpenTofuAndProviderVersion(t *testing.T) {
	if got := ciTofuVersion(t); got != render.TofuVersion {
		t.Errorf("CI tests with OpenTofu %s, the renderer requires %s: the opentofu group moves them together, so they must start equal", got, render.TofuVersion)
	}
	for _, m := range []string{"group", "component"} {
		b, err := os.ReadFile(filepath.Join(repoRoot, "modules", "github", m, "versions.tf"))
		if err != nil {
			t.Fatal(err)
		}
		// Anchored to the line start, so required_version = ">= 1.12.0" does not match.
		sub := regexp.MustCompile(`(?m)^[ \t]*version[ \t]*=[ \t]*"([^"]+)"`).FindSubmatch(b)
		if sub == nil || string(sub[1]) != render.GitHubProviderVersion {
			t.Errorf("modules/github/%s pins the provider at %q, the renderer at %s", m, sub, render.GitHubProviderVersion)
		}
	}
}

func TestCIValidatesTheRenovateConfig(t *testing.T) {
	_, s := mustStep(t, "ci workflows", mustJob(t, loadWorkflow(t, ".github/workflows/ci.yaml"), "workflows").Steps, "renovate-config")
	if s.If != "" || s.ContinueOnError != "" {
		t.Errorf("ci renovate-config: if %q, continue-on-error %q; the validation must run, and fail the job", s.If, s.ContinueOnError)
	}
	want := `docker run --rm -v "$GITHUB_WORKSPACE/` + renovateConfigPath + `:/tmp/renovate-global.json:ro" ` +
		"ghcr.io/renovatebot/renovate:" + renovateVersion(t) + " renovate-config-validator --strict /tmp/renovate-global.json"
	if got := strings.TrimSpace(s.Run); got != want {
		t.Errorf("ci renovate-config run = %q, want %q", got, want)
	}
}
