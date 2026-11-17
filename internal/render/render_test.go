package render

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/fs"
	"maps"
	"math/rand/v2"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
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

// checkGolden compares one rendered file, rel, with its golden file under
// testdata/<fixture>/expected, and rewrites that file first when the test runs
// with -update.
func checkGolden(t *testing.T, fixture, rel string, got []byte) {
	t.Helper()
	golden := filepath.Join("testdata", fixture, "expected", filepath.FromSlash(rel))
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
		t.Errorf("%s: %s differs from its golden file:\n%s", fixture, rel, got)
	}
}

// checkGoldens compares every stack of a render with fixture's golden files:
// each main.tf.json byte for byte, and the set of stacks, so a stack that
// appears or disappears fails too. -update rewrites the golden files of the
// rendered stacks; a golden file of a stack that is no longer rendered must be
// deleted by hand.
func checkGoldens(t *testing.T, fixture string, files map[string][]byte) {
	t.Helper()
	var rendered []string
	for p := range files {
		if path.Base(p) == "main.tf.json" {
			rendered = append(rendered, p)
		}
	}
	slices.Sort(rendered)
	for _, p := range rendered {
		checkGolden(t, fixture, p, files[p])
	}
	expected := filepath.Join("testdata", fixture, "expected")
	var golden []string
	err := filepath.WalkDir(expected, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "main.tf.json" {
			return err
		}
		rel, err := filepath.Rel(expected, p)
		golden = append(golden, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(golden)
	if !slices.Equal(rendered, golden) {
		t.Errorf("%s: rendered stacks %v, golden stacks %v", fixture, rendered, golden)
	}
}

// renderedAWSStack is what the tests read of a rendered AWS stack.
type renderedAWSStack struct {
	Provider struct {
		AWS struct {
			Region string `json:"region"`
		} `json:"aws"`
	} `json:"provider"`
	Module map[string]map[string]any `json:"module"`
}

// awsStackOf decodes the AWS stack rendered in dir.
func awsStackOf(t *testing.T, files map[string][]byte, dir string) renderedAWSStack {
	t.Helper()
	b, ok := files[dir+"/main.tf.json"]
	if !ok {
		t.Fatalf("no stack %s in %v", dir, slices.Sorted(maps.Keys(files)))
	}
	var s renderedAWSStack
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// stackDirs lists the stacks of a render: the directories of its main.tf.json files.
func stackDirs(files map[string][]byte) []string {
	var out []string
	for p := range files {
		if path.Base(p) == "main.tf.json" {
			out = append(out, path.Dir(p))
		}
	}
	slices.Sort(out)
	return out
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

// TestRenderGolden pins the render of a claims repo without any aws part:
// only the github stack, whose golden file Phases 2b and 2d left unchanged,
// byte for byte. A Component without aws.registry passes no variables, and an
// environment without an aws block has no AWS stack, so existing claims repos
// plan no change.
func TestRenderGolden(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(files)); !slices.Equal(got, []string{"github/.terraform.lock.hcl", "github/main.tf.json"}) {
		t.Errorf("rendered files = %v, want only the github stack", got)
	}
	checkGoldens(t, "basic", files)
}

// TestRenderEnvVariablesGolden renders the aws-registry fixture: basic plus
// aws.registry and the environments' aws blocks. Each environment of the
// Component gets AWS_ROLE_ARN, ECR_REPOSITORY and AWS_REGION (spec §4.4), and
// pro's are spec §5.3's example, checked verbatim here so that -update cannot
// change them unnoticed. TestRenderAWSStacksGolden checks the fixture's AWS
// stacks.
func TestRenderEnvVariablesGolden(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	if !m.Components[0].AWS.Registry || m.Platform.Environments["dev"].AWS == nil || m.Platform.Environments["pro"].AWS == nil {
		t.Fatal("the fixture must set aws.registry and give every environment an aws block")
	}
	files, err := Render(m, Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "aws-registry", "github/main.tf.json", files["github/main.tf.json"])
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
// Component without aws.registry: the environments' aws blocks alone leave the
// github stack basic's golden file, byte for byte, and add only each
// environment's baseline, never a Component stack.
func TestRenderWithoutRegistryPassesNoVariables(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Components[0].AWS.Registry = false
	files, err := Render(m, Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "basic", "github/main.tf.json", files["github/main.tf.json"])
	if got, want := stackDirs(files), []string{"aws/dev/_baseline", "aws/pro/_baseline", "github"}; !slices.Equal(got, want) {
		t.Errorf("stacks = %v, want %v", got, want)
	}
}

// TestRenderAWSStacksGolden pins the AWS stacks of the aws-registry fixture
// (spec §5.4): each environment's baseline and the Component's stack in each
// of its environments, each with the engine's AWS lock file.
func TestRenderAWSStacksGolden(t *testing.T) {
	files, err := Render(loadFixture(t, "aws-registry"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"aws/dev/_baseline", "aws/dev/components/api", "aws/pro/_baseline", "aws/pro/components/api", "github"}
	if got := stackDirs(files); !slices.Equal(got, want) {
		t.Errorf("stacks = %v, want %v", got, want)
	}
	if len(files) != 2*len(want) {
		t.Errorf("rendered %d files, want a main.tf.json and a lock file for each of the %d stacks: %v", len(files), len(want), slices.Sorted(maps.Keys(files)))
	}
	for _, dir := range want[:4] {
		if !bytes.Equal(files[dir+"/.terraform.lock.hcl"], lockfiles.AWS) {
			t.Errorf("%s must carry the engine's AWS lock file", dir)
		}
	}
	checkGoldens(t, "aws-registry", files)
}

// TestRenderAWSStacksFollowTheAWSBlocks: every environment with an aws block
// gets its baseline, used by a Component or not, and an environment without one
// gets no AWS stack; a registry Component gets a stack in each environment it
// lists, and only there; a Component without aws.registry gets none.
func TestRenderAWSStacksFollowTheAWSBlocks(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Platform.Environments["staging"] = claims.Environment{AWS: &claims.AWS{AccountID: "000000000002", Region: "eu-central-1"}}
	m.Platform.Environments["qa"] = claims.Environment{}
	web := m.Components[0]
	web.Name, web.Environments = "web", []string{"dev", "qa", "staging"}
	web.AWS.Registry = false
	m.Components = append(m.Components, web)
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"aws/dev/_baseline", "aws/dev/components/api", "aws/pro/_baseline", "aws/pro/components/api", "aws/staging/_baseline", "github"}
	if got := stackDirs(files); !slices.Equal(got, want) {
		t.Errorf("stacks = %v, want %v", got, want)
	}
	if got := awsStackOf(t, files, "aws/staging/_baseline").Provider.AWS.Region; got != "eu-central-1" {
		t.Errorf("staging baseline region = %q, want its environment's eu-central-1", got)
	}
}

// TestRenderAWSStacksUseNamingsIdentities pins spec §5.5's identities in the
// rendered Component stacks, outside the golden files, so that -update cannot
// change them unnoticed: pro's module call verbatim, and in every environment
// the role, the repository and the region that the github stack's variables
// name, and the account's GitHub OIDC provider.
func TestRenderAWSStacksUseNamingsIdentities(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"source":            "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/aws/component?ref=v1",
		"github_repository": "acme/api",
		"environment":       "pro",
		"repository_name":   "api",
		"role_name":         "api-pro-ci",
		"oidc_provider_arn": "arn:aws:iam::000000000003:oidc-provider/token.actions.githubusercontent.com",
	}
	if got := awsStackOf(t, files, "aws/pro/components/api").Module; !reflect.DeepEqual(got, map[string]map[string]any{"component": want}) {
		t.Errorf("pro module = %v, want only component = %v", got, want)
	}
	vars := componentEnvironments(t, files["github/main.tf.json"], "api")
	for _, env := range m.Components[0].Environments {
		aws := m.Platform.Environments[env].AWS
		s := awsStackOf(t, files, "aws/"+env+"/components/api")
		call := s.Module["component"]
		v := vars[env].Variables
		if got, want := "arn:aws:iam::"+aws.AccountID+":role/"+call["role_name"].(string), v["AWS_ROLE_ARN"]; got != want {
			t.Errorf("%s: the stack creates the role %s, the github stack passes AWS_ROLE_ARN %s", env, got, want)
		}
		if got, want := aws.AccountID+".dkr.ecr."+aws.Region+".amazonaws.com/"+call["repository_name"].(string), v["ECR_REPOSITORY"]; got != want {
			t.Errorf("%s: the stack creates the repository %s, the github stack passes ECR_REPOSITORY %s", env, got, want)
		}
		if s.Provider.AWS.Region != v["AWS_REGION"] {
			t.Errorf("%s: the stack's region %s, the github stack passes AWS_REGION %s", env, s.Provider.AWS.Region, v["AWS_REGION"])
		}
		if got, want := call["oidc_provider_arn"], naming.OIDCProviderARN(aws.AccountID); got != want {
			t.Errorf("%s: oidc_provider_arn = %v, want the account's %s (naming.OIDCProviderARN)", env, got, want)
		}
		if got := awsStackOf(t, files, "aws/"+env+"/_baseline").Module; !reflect.DeepEqual(got, map[string]map[string]any{"baseline": {"source": "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/aws/baseline?ref=v1"}}) {
			t.Errorf("%s baseline module = %v, want only the baseline call, with no inputs", env, got)
		}
	}
}

// TestRenderAWSStacksAreThin: an AWS stack holds the pinned versions, a local
// backend in its own directory, no encryption block, the region and one module
// call (spec §5.3, ADR-0030), and nothing about floci, which CI adds in
// floci_override.tf.json (spec §5.6).
func TestRenderAWSStacksAreThin(t *testing.T) {
	files, err := Render(loadFixture(t, "aws-registry"), Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range stackDirs(files) {
		if !strings.HasPrefix(dir, "aws/") {
			continue
		}
		b := files[dir+"/main.tf.json"]
		var doc struct {
			Terraform map[string]any            `json:"terraform"`
			Provider  map[string]any            `json:"provider"`
			Module    map[string]map[string]any `json:"module"`
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&doc); err != nil {
			t.Errorf("%s: %v: a stack holds only terraform, provider and module", dir, err)
			continue
		}
		wantTerraform := map[string]any{
			"required_version":   TofuVersion,
			"required_providers": map[string]any{"aws": map[string]any{"source": "hashicorp/aws", "version": AWSProviderVersion}},
			"backend":            map[string]any{"local": map[string]any{"path": "terraform.tfstate"}},
		}
		if !reflect.DeepEqual(doc.Terraform, wantTerraform) {
			t.Errorf("%s: terraform = %v, want %v", dir, doc.Terraform, wantTerraform)
		}
		if want := map[string]any{"aws": map[string]any{"region": "eu-west-1"}}; !reflect.DeepEqual(doc.Provider, want) {
			t.Errorf("%s: provider = %v, want only the region, %v", dir, doc.Provider, want)
		}
		if len(doc.Module) != 1 {
			t.Errorf("%s: %d module calls, want one", dir, len(doc.Module))
		}
		for _, word := range []string{"floci", "localhost", "4566", "endpoint", "skip_", "encryption"} {
			if bytes.Contains(bytes.ToLower(b), []byte(word)) {
				t.Errorf("%s mentions %q: emulator settings belong to CI (spec §5.6), and the AWS stacks keep no encrypted state (ADR-0030):\n%s", dir, word, b)
			}
		}
	}
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
	if stacks, err := awsStacks(m, Options{ModuleRef: "v1"}); err == nil || err.Error() != want || stacks != nil {
		t.Errorf("awsStacks = %d stacks, error %v; want no stacks and the error %q", len(stacks), err, want)
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
	if !regexp.MustCompile(`version\s*=\s*"` + regexp.QuoteMeta(AWSProviderVersion) + `"`).Match(lockfiles.AWS) {
		t.Errorf("the AWS lock file does not pin AWSProviderVersion %s", AWSProviderVersion)
	}
}

// TestModuleSource: ModulesDir is relative to the github stack, so a deeper
// stack climbs to the output root and descends into github first. Every form
// resolves to the same directory from its own stack, and the github stack's
// sources stay ModulesDir as given, byte for byte.
func TestModuleSource(t *testing.T) {
	tests := []struct{ modulesDir, stackDir, module, want string }{
		{"../../modules", "github", "github/group", "../../modules/github/group"},
		{"../../modules/", "github", "github/group", "../../modules/github/group"},
		{"./mods", "github", "github/component", "./mods/github/component"},
		{"../../modules", "aws/dev/_baseline", "aws/baseline", "../../../../modules/aws/baseline"},
		{"../../modules/", "aws/dev/_baseline", "aws/baseline", "../../../../modules/aws/baseline"},
		{"./mods", "aws/dev/_baseline", "aws/baseline", "../../../github/mods/aws/baseline"},
		{"../../modules", "aws/dev/components/api", "aws/component", "../../../../../modules/aws/component"},
		{"./mods", "aws/dev/components/api", "aws/component", "../../../../github/mods/aws/component"},
	}
	for _, tt := range tests {
		if got := moduleSource(Options{ModulesDir: tt.modulesDir}, tt.stackDir, tt.module); got != tt.want {
			t.Errorf("moduleSource(%q, %q, %q) = %q, want %q", tt.modulesDir, tt.stackDir, tt.module, got, tt.want)
		}
	}
	if got, want := moduleSource(Options{ModuleRef: "v1"}, "aws/dev/_baseline", "aws/baseline"), "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/aws/baseline?ref=v1"; got != want {
		t.Errorf("git source = %q, want %q", got, want)
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
	files, err = Render(loadFixture(t, "aws-registry"), Options{ModulesDir: "../../modules"})
	if err != nil {
		t.Fatal(err)
	}
	for dir, want := range map[string]string{
		"aws/dev/_baseline":      `"source": "../../../../modules/aws/baseline"`,
		"aws/dev/components/api": `"source": "../../../../../modules/aws/component"`,
	} {
		if !strings.Contains(string(files[dir+"/main.tf.json"]), want) {
			t.Errorf("%s: missing %s, the same directory seen from the stack", dir, want)
		}
	}
}

// TestRenderIsDeterministic is spec §8.2's determinism test at the boundary Render
// consumes: any claim order, and any order of a Component's environments, gives
// identical files, env variables and AWS stacks included. The platform's
// environments are a map, which Go ranges over in a new order on every render.
// (Load's half, sorted output whatever the read order, is
// TestLoadSortsClaimsByName in internal/claims.)
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
		for c := range shuffled.Components {
			envs := slices.Clone(shuffled.Components[c].Environments)
			r.Shuffle(len(envs), func(a, b int) { envs[a], envs[b] = envs[b], envs[a] })
			shuffled.Components[c].Environments = envs
		}
		got, err := Render(&shuffled, Options{ModuleRef: "v1"})
		if err != nil {
			t.Fatal(err)
		}
		if !maps.EqualFunc(got, want, bytes.Equal) {
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
