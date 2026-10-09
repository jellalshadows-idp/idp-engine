package claims

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	validPlatform  = "apiVersion: idp/v1\nkind: Platform\ngithub:\n  org: acme\n  writerAppId: 5255579\nenvironments:\n  dev: {}\n  pro:\n    protected: true\n"
	validGroup     = "apiVersion: idp/v1\nkind: Group\nname: platform\nmembers:\n  - user: alice\n    role: maintainer\n"
	validComponent = "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nenvironments: [dev, pro]\n"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func baseRepo() map[string]string {
	return map[string]string{
		"config/platform.yaml":        validPlatform,
		"claims/groups/platform.yaml": validGroup,
		"claims/components/api.yaml":  validComponent,
	}
}

func lines(ds []Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.String()
	}
	return out
}

func TestLoadValidRepo(t *testing.T) {
	m, ds, err := Load(writeTree(t, baseRepo()))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 0 {
		t.Fatalf("diagnostics: %v", lines(ds))
	}
	if m.Platform.GitHub.Org != "acme" || m.Platform.GitHub.WriterAppID != 5255579 {
		t.Errorf("platform github = %+v", m.Platform.GitHub)
	}
	if !m.Platform.ArchiveOnDestroy() || m.Platform.RequiredApprovals() != 1 {
		t.Error("platform defaults must be archiveOnDestroy=true and requiredApprovals=1")
	}
	if !m.Platform.Environments["pro"].Protected || m.Platform.Environments["dev"].Protected {
		t.Errorf("environments = %+v", m.Platform.Environments)
	}
	if len(m.Groups) != 1 || m.Groups[0].Name != "platform" || m.Groups[0].Members[0].Role != "maintainer" {
		t.Errorf("groups = %+v", m.Groups)
	}
	if len(m.Components) != 1 || m.Components[0].OwnerGroup() != "platform" {
		t.Errorf("components = %+v", m.Components)
	}
}

func TestLoadSortsClaimsByName(t *testing.T) {
	repo := baseRepo()
	repo["claims/components/zeta.yaml"] = strings.Replace(validComponent, "name: api", "name: zeta", 1)
	repo["claims/components/alpha.yaml"] = strings.Replace(validComponent, "name: api", "name: alpha", 1)
	m, ds, err := Load(writeTree(t, repo))
	if err != nil || len(ds) != 0 {
		t.Fatalf("err %v, diagnostics %v", err, lines(ds))
	}
	got := []string{m.Components[0].Name, m.Components[1].Name, m.Components[2].Name}
	if strings.Join(got, ",") != "alpha,api,zeta" {
		t.Errorf("order = %v", got)
	}
}

func TestLoadDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]string)
		want   []string // exact Diagnostic.String() lines, in order
	}{
		{"missing platform", func(r map[string]string) { delete(r, "config/platform.yaml") },
			[]string{"config/platform.yaml: file not found"}},
		{"name must match the file", func(r map[string]string) {
			r["claims/components/api.yaml"] = strings.Replace(validComponent, "name: api", "name: orders", 1)
		}, []string{`claims/components/api.yaml:3: name "orders" must match the file name "api"`}},
		{"owner group must exist", func(r map[string]string) {
			r["claims/components/api.yaml"] = strings.Replace(validComponent, "group:platform", "group:payments", 1)
		}, []string{`claims/components/api.yaml:4: owner group "payments" does not exist (no claims/groups/payments.yaml)`}},
		{"environment must exist", func(r map[string]string) {
			r["claims/components/api.yaml"] = strings.Replace(validComponent, "[dev, pro]", "[dev, staging]", 1)
		}, []string{`claims/components/api.yaml:5: environment "staging" is not defined in config/platform.yaml`}},
		{"duplicate member, case-insensitive", func(r map[string]string) {
			r["claims/groups/platform.yaml"] = validGroup + "  - user: Alice\n    role: member\n"
		}, []string{`claims/groups/platform.yaml:7: member "Alice" is listed more than once`}},
		{"required prefix", func(r map[string]string) {
			r["config/platform.yaml"] = validPlatform + "naming:\n  requiredPrefix: e2e-\n"
		}, []string{
			`claims/components/api.yaml:3: name "api" must start with "e2e-" in this claims repo (config/platform.yaml naming.requiredPrefix)`,
			`claims/groups/platform.yaml:3: name "platform" must start with "e2e-" in this claims repo (config/platform.yaml naming.requiredPrefix)`,
		}},
		{"reserved prefix", func(r map[string]string) {
			r["config/platform.yaml"] = validPlatform + "naming:\n  reservedPrefixes: [e2e-]\n"
			r["claims/groups/e2e-team.yaml"] = strings.Replace(validGroup, "name: platform", "name: e2e-team", 1)
		}, []string{`claims/groups/e2e-team.yaml:3: name "e2e-team" uses the reserved prefix "e2e-" (config/platform.yaml naming.reservedPrefixes)`}},
		{"contradictory naming", func(r map[string]string) {
			r["config/platform.yaml"] = validPlatform + "naming:\n  requiredPrefix: e2e-\n  reservedPrefixes: [e2e-]\n"
			delete(r, "claims/groups/platform.yaml")
			delete(r, "claims/components/api.yaml")
		}, []string{`config/platform.yaml:11: naming.requiredPrefix "e2e-" is also listed in naming.reservedPrefixes`}},
		{".yml file", func(r map[string]string) { r["claims/components/web.yml"] = validComponent },
			[]string{"claims/components/web.yml: unexpected file; claim files must end with .yaml"}},
		{"subdirectory", func(r map[string]string) { r["claims/groups/team/x.yaml"] = validGroup },
			[]string{"claims/groups/team: unexpected directory; claims are files directly in claims/groups"}},
		{"workspaces are not supported yet", func(r map[string]string) { r["claims/workspaces/dev/logs.yaml"] = "x: 1\n" },
			[]string{"claims/workspaces: Workspace claims are not supported yet (they arrive in Phase 3)"}},
		{"unexpected entry in claims", func(r map[string]string) { r["claims/notes.md"] = "# notes\n" },
			[]string{"claims/notes.md: unexpected entry; claims/ holds only groups/ and components/"}},
		{"mismatched name never borrows another file's lines", func(r map[string]string) {
			r["claims/components/api.yaml"] = strings.Replace(strings.Replace(validComponent, "name: api", "name: orders", 1), "[dev, pro]", "[staging]", 1)
			r["claims/components/orders.yaml"] = strings.Replace(validComponent, "name: api", "name: orders", 1)
		}, []string{`claims/components/api.yaml:3: name "orders" must match the file name "api"`}},
		{"gitkeep ignored", func(r map[string]string) { r["claims/groups/.gitkeep"] = "" }, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := baseRepo()
			tt.mutate(repo)
			_, ds, err := Load(writeTree(t, repo))
			if err != nil {
				t.Fatal(err)
			}
			got := lines(ds)
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("diagnostics =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestLoadInvalidOwnerGroupDoesNotCascade(t *testing.T) {
	repo := baseRepo()
	repo["claims/groups/platform.yaml"] = strings.Replace(validGroup, "maintainer", "owner", 1)
	_, ds, err := Load(writeTree(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if strings.Contains(d.Message, "does not exist") {
			t.Errorf("misleading cascade: %v", d)
		}
	}
	if len(ds) != 1 || ds[0].File != "claims/groups/platform.yaml" || ds[0].Line != 6 {
		t.Errorf("diagnostics = %v, want only the group's role error at line 6", lines(ds))
	}
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	repo := baseRepo()
	repo["claims/groups/platform.yaml"] = strings.Replace(validGroup, "maintainer", "owner", 1)
	repo["claims/components/api.yaml"] = strings.Replace(validComponent, "[dev, pro]", "[dev, staging]", 1)
	_, ds, err := Load(writeTree(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 2 || ds[0].File != "claims/components/api.yaml" || ds[1].File != "claims/groups/platform.yaml" {
		t.Errorf("diagnostics = %v, want two, sorted by file", lines(ds))
	}
}

func TestLoadRejectsSymlinkedClaim(t *testing.T) {
	repo := baseRepo()
	repo["elsewhere/web.yaml"] = strings.Replace(validComponent, "name: api", "name: web", 1)
	dir := writeTree(t, repo)
	if err := os.Symlink(filepath.Join(dir, "elsewhere", "web.yaml"), filepath.Join(dir, "claims", "components", "web.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, ds, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := "claims/components/web.yaml: unexpected symlink or special file; claim files must be regular files"
	if got := lines(ds); len(got) != 1 || got[0] != want {
		t.Errorf("diagnostics = %v, want exactly %q", got, want)
	}
}

func TestLoadRejectsSymlinkedPlatform(t *testing.T) {
	repo := baseRepo()
	repo["elsewhere/platform.yaml"] = validPlatform
	delete(repo, "config/platform.yaml")
	dir := writeTree(t, repo)
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "elsewhere", "platform.yaml"), filepath.Join(dir, "config", "platform.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, ds, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := "config/platform.yaml: must be a regular file, not a symlink or special file"
	found := false
	for _, l := range lines(ds) {
		found = found || l == want
	}
	if !found {
		t.Errorf("diagnostics = %v, want to contain %q", lines(ds), want)
	}
}
