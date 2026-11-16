package render

import (
	"bytes"
	"encoding/json"
	"flag"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
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

// checkGolden compares a rendered github stack with the golden file of fixture,
// and rewrites that file first when the test runs with -update.
func checkGolden(t *testing.T, fixture string, got []byte) {
	t.Helper()
	golden := filepath.Join("testdata", fixture, "expected", "github", "main.tf.json")
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
		t.Errorf("%s: main.tf.json differs from its golden file:\n%s", fixture, got)
	}
}

// renderedEnvironment is one entry of the environments a component module receives.
type renderedEnvironment struct {
	Protected bool              `json:"protected"`
	Variables map[string]string `json:"variables"`
}

// componentEnvironments decodes the environments that a rendered github stack
// passes to the module component_<name>.
func componentEnvironments(t *testing.T, stack []byte, name string) map[string]renderedEnvironment {
	t.Helper()
	var doc struct {
		Module map[string]struct {
			Environments map[string]renderedEnvironment `json:"environments"`
		} `json:"module"`
	}
	if err := json.Unmarshal(stack, &doc); err != nil {
		t.Fatal(err)
	}
	c, ok := doc.Module["component_"+name]
	if !ok {
		t.Fatalf("no module component_%s in:\n%s", name, stack)
	}
	return c.Environments
}

// TestRenderGolden pins the render of a claims repo without any aws part. Phase 2b
// left this golden file unchanged, byte for byte: a Component without
// aws.registry passes no variables, so existing stacks plan no change.
func TestRenderGolden(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "basic", files["github/main.tf.json"])
}

// TestRenderEnvVariablesGolden renders the aws-registry fixture: basic plus
// aws.registry and the environments' aws blocks. Each environment of the
// Component gets AWS_ROLE_ARN, ECR_REPOSITORY and AWS_REGION (spec §4.4), and
// pro's are spec §5.3's example, checked verbatim here so that -update cannot
// change them unnoticed. Only the github stack is rendered until Phase 2d adds
// the AWS stacks.
func TestRenderEnvVariablesGolden(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	if !m.Components[0].AWS.Registry || m.Platform.Environments["dev"].AWS == nil || m.Platform.Environments["pro"].AWS == nil {
		t.Fatal("the fixture must set aws.registry and give every environment an aws block")
	}
	files, err := Render(m, Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(files)); !slices.Equal(got, []string{"github/.terraform.lock.hcl", "github/main.tf.json"}) {
		t.Errorf("rendered files = %v, want only the github stack", got)
	}
	checkGolden(t, "aws-registry", files["github/main.tf.json"])
	want := map[string]string{
		"AWS_ROLE_ARN":   "arn:aws:iam::000000000003:role/api-pro-ci",
		"ECR_REPOSITORY": "000000000003.dkr.ecr.eu-west-1.amazonaws.com/api",
		"AWS_REGION":     "eu-west-1",
	}
	if got := componentEnvironments(t, files["github/main.tf.json"], "api")["pro"].Variables; !maps.Equal(got, want) {
		t.Errorf("pro variables = %v, want spec §5.3's %v", got, want)
	}
}

// TestRenderWithoutRegistryPassesNoVariables keeps Phase 2a's guard for every
// Component without aws.registry: the environments' aws blocks alone change
// nothing, so the render is basic's golden file, byte for byte.
func TestRenderWithoutRegistryPassesNoVariables(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Components[0].AWS.Registry = false
	files, err := Render(m, Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "basic", files["github/main.tf.json"])
}

// TestRenderEnvVariablesFollowEachEnvironment gives the Component's environments
// different accounts and regions, and the platform an aws environment that the
// Component does not list: each listed environment gets its own values, and the
// other gets nothing.
func TestRenderEnvVariablesFollowEachEnvironment(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Platform.Environments["pro"] = claims.Environment{Protected: true, AWS: &claims.AWS{AccountID: "000000000003", Region: "us-east-1"}}
	m.Platform.Environments["staging"] = claims.Environment{AWS: &claims.AWS{AccountID: "000000000002", Region: "eu-central-1"}}
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]renderedEnvironment{
		"dev": {Protected: false, Variables: map[string]string{
			"AWS_ROLE_ARN":   "arn:aws:iam::000000000001:role/api-dev-ci",
			"ECR_REPOSITORY": "000000000001.dkr.ecr.eu-west-1.amazonaws.com/api",
			"AWS_REGION":     "eu-west-1",
		}},
		"pro": {Protected: true, Variables: map[string]string{
			"AWS_ROLE_ARN":   "arn:aws:iam::000000000003:role/api-pro-ci",
			"ECR_REPOSITORY": "000000000003.dkr.ecr.us-east-1.amazonaws.com/api",
			"AWS_REGION":     "us-east-1",
		}},
	}
	if got := componentEnvironments(t, files["github/main.tf.json"], "api"); !reflect.DeepEqual(got, want) {
		t.Errorf("environments = %+v, want %+v", got, want)
	}
}

// TestRenderRejectsARegistryEnvironmentWithoutAWS covers a model that Load
// rejects (spec §4.7): rendering it must fail, never panic, and never drop the
// variables, which would plan their deletion.
func TestRenderRejectsARegistryEnvironmentWithoutAWS(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Platform.Environments["pro"] = claims.Environment{Protected: true}
	files, err := Render(m, Options{ModuleRef: "v1"})
	want := `render: component "api" has aws.registry, but environment "pro" has no aws block`
	if err == nil || err.Error() != want || files != nil {
		t.Errorf("Render = %d files, error %v; want no files and the error %q", len(files), err, want)
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
// consumes: any claim order, and any order of a Component's environments, gives
// identical bytes, env variables included. (Load's half, sorted output whatever
// the read order, is TestLoadSortsClaimsByName in internal/claims.)
func TestRenderIsDeterministic(t *testing.T) {
	m := loadFixture(t, "aws-registry")
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
		for i := range shuffled.Components {
			envs := slices.Clone(shuffled.Components[i].Environments)
			r.Shuffle(len(envs), func(a, b int) { envs[a], envs[b] = envs[b], envs[a] })
			shuffled.Components[i].Environments = envs
		}
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
