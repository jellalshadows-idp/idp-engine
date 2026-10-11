package render

import (
	"bytes"
	"encoding/json"
	"flag"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

var update = flag.Bool("update", false, "rewrite golden files")

func loadFixture(t *testing.T, name string) *claims.Model {
	t.Helper()
	m, ds, err := claims.Load(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) > 0 {
		t.Fatalf("fixture %s has diagnostics: %v", name, ds)
	}
	return m
}

func TestRenderGolden(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	got := files["github/main.tf.json"]
	golden := filepath.Join("testdata", "basic", "expected", "github", "main.tf.json")
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("main.tf.json differs from its golden file:\n%s", got)
	}
}

func TestRenderCopiesTheLockFile(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files["github/.terraform.lock.hcl"], lockfiles.GitHub) {
		t.Error("the stack must carry the engine's lock file")
	}
	if !regexp.MustCompile(`version\s*=\s*"` + regexp.QuoteMeta(GitHubProviderVersion) + `"`).Match(lockfiles.GitHub) {
		t.Errorf("lock file does not pin GitHubProviderVersion %s", GitHubProviderVersion)
	}
}

func TestRenderLocalModules(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModulesDir: "../../modules"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(files["github/main.tf.json"])
	for _, want := range []string{`"source": "../../modules/github/group"`, `"source": "../../modules/github/component"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
}

// TestRenderIsDeterministic is spec §8.2's determinism test at the boundary Render
// consumes: any claim order gives identical bytes. (Load's half, sorted output
// whatever the read order, is TestLoadSortsClaimsByName in internal/claims.)
func TestRenderIsDeterministic(t *testing.T) {
	m := loadFixture(t, "basic")
	// Two claims of each kind, so their order can actually vary.
	data := m.Groups[0]
	data.Name = "data"
	m.Groups = append(m.Groups, data)
	web := m.Components[0]
	web.Name, web.Owner = "web", "group:data"
	m.Components = append(m.Components, web)

	want, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 20; i++ {
		shuffled := *m
		shuffled.Groups = append([]claims.Group{}, m.Groups...)
		shuffled.Components = append([]claims.Component{}, m.Components...)
		r.Shuffle(len(shuffled.Groups), func(a, b int) {
			shuffled.Groups[a], shuffled.Groups[b] = shuffled.Groups[b], shuffled.Groups[a]
		})
		r.Shuffle(len(shuffled.Components), func(a, b int) {
			shuffled.Components[a], shuffled.Components[b] = shuffled.Components[b], shuffled.Components[a]
		})
		got, err := Render(&shuffled, Options{ModuleRef: "v1"})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got["github/main.tf.json"], want["github/main.tf.json"]) {
			t.Fatalf("iteration %d: render output depends on claim order", i)
		}
	}
}

func TestRenderKeepsTextLiteral(t *testing.T) {
	m := loadFixture(t, "basic")
	m.Components[0].Description = "Pagos & cobros <beta> — ñ"
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files["github/main.tf.json"]), `"description": "Pagos & cobros <beta> — ñ"`) {
		t.Errorf("description was not rendered literally:\n%s", files["github/main.tf.json"])
	}
}

func TestLiteral(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"empty", "", ""},
		{"plain", "plain", "plain"},
		{"interpolation", "${x}", "$${x}"},
		{"directive", "%{ if true }a%{ endif }", "%%{ if true }a%%{ endif }"},
		{"already escaped", "$${already}", "$$${already}"},
		{"no brace", "cost: $5, 100%", "cost: $5, 100%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := literal(tt.in); got != tt.want {
				t.Errorf("literal(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRenderEscapesTemplateSequencesInClaimText(t *testing.T) {
	m := loadFixture(t, "basic")
	m.Groups[0].Description = `${base64encode(file("/proc/self/environ"))}`
	m.Components[0].Description = `%{ for x in [1] }x%{ endfor }`
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Module map[string]map[string]any `json:"module"`
	}
	if err := json.Unmarshal(files["github/main.tf.json"], &doc); err != nil {
		t.Fatal(err)
	}
	g := doc.Module["group_"+m.Groups[0].Name]
	if want := `$${base64encode(file("/proc/self/environ"))}`; g["description"] != want {
		t.Errorf("group description = %q, want %q", g["description"], want)
	}
	c := doc.Module["component_"+m.Components[0].Name]
	if want := `%%{ for x in [1] }x%%{ endfor }`; c["description"] != want {
		t.Errorf("component description = %q, want %q", c["description"], want)
	}
	if want := "${module.group_" + m.Components[0].OwnerGroup() + ".team_id}"; c["owner_team_id"] != want {
		t.Errorf("owner_team_id = %q, want the live reference %q", c["owner_team_id"], want)
	}
}

func TestRenderWithoutClaimsHasNoModules(t *testing.T) {
	m := loadFixture(t, "basic")
	m.Groups, m.Components = nil, nil
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(files["github/main.tf.json"])
	if strings.Contains(s, `"module"`) || !strings.Contains(s, `"owner": "acme"`) {
		t.Errorf("unexpected stack without claims:\n%s", s)
	}
}

func TestRenderNeedsAModuleSource(t *testing.T) {
	if _, err := Render(loadFixture(t, "basic"), Options{}); err == nil {
		t.Error("want an error when neither ModuleRef nor ModulesDir is set")
	}
}
