package floci

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

// TestTheFixtureIsTheTestPlatform keeps TestFloci's written-out expectations
// equal to the fixture it renders: the org, the one registry Component, and
// each environment's account and region.
func TestTheFixtureIsTheTestPlatform(t *testing.T) {
	m, diags, err := claims.Load(fixtureDir)
	if err != nil || len(diags) > 0 {
		t.Fatalf("load %s: %v %v", fixtureDir, err, diags)
	}
	if m.Platform.GitHub.Org != org {
		t.Errorf("org = %q, want %q", m.Platform.GitHub.Org, org)
	}
	var envs []string
	for _, tg := range targets {
		envs = append(envs, tg.env)
		if got := m.Platform.Environments[tg.env].AWS; got == nil || *got != (claims.AWS{AccountID: tg.accountID, Region: tg.region}) {
			t.Errorf("environment %s: aws = %+v, want account %s in %s", tg.env, got, tg.accountID, tg.region)
		}
	}
	if len(m.Platform.Environments) != len(targets) {
		t.Errorf("the fixture has %d environments, TestFloci applies %d", len(m.Platform.Environments), len(targets))
	}
	if len(m.Components) != 1 || m.Components[0].Name != component || !m.Components[0].AWS.Registry || !slices.Equal(m.Components[0].Environments, envs) {
		t.Errorf("components = %+v, want only %s, with aws.registry, in %v", m.Components, component, envs)
	}
}

// TestRenderFixture: the render TestFloci applies holds exactly the AWS stacks
// it applies, each with the engine's AWS lock file, a module source in the
// modules it copies, and nothing of floci's, which awsstack.WriteOverride adds.
func TestRenderFixture(t *testing.T) {
	root := t.TempDir()
	if err := renderFixture(root); err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, tg := range targets {
		want = append(want, stacks(tg)...)
	}
	slices.Sort(want)
	rendered := filepath.Join(root, "rendered")
	var got []string
	err := filepath.WalkDir(filepath.Join(rendered, "aws"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Name() != "main.tf.json" {
			return err
		}
		dir := filepath.Dir(p)
		rel, err := filepath.Rel(rendered, dir)
		if err != nil {
			return err
		}
		got = append(got, filepath.ToSlash(rel))
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if bytes.Contains(b, []byte("floci")) || bytes.Contains(b, []byte("endpoint")) {
			t.Errorf("%s: the render mentions the emulator (spec §5.6)", rel)
		}
		var doc struct {
			Module map[string]struct {
				Source string `json:"source"`
			} `json:"module"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			return err
		}
		for name, call := range doc.Module {
			want := filepath.Join(root, "modules", "aws", name)
			if src := filepath.Join(dir, filepath.FromSlash(call.Source)); src != want {
				t.Errorf("%s: module %s resolves to %s, want the copied modules' %s", rel, name, src, want)
			}
		}
		lock, err := os.ReadFile(filepath.Join(dir, ".terraform.lock.hcl"))
		if err != nil || !bytes.Equal(lock, lockfiles.AWS) {
			t.Errorf("%s must carry the engine's AWS lock file (err %v)", rel, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("rendered AWS stacks = %v, TestFloci applies %v", got, want)
	}
}

// TestModulesUseNamingsIssuer pins the one identity the modules write
// themselves: GitHub Actions' issuer, whose host is in every OIDC provider ARN
// that internal/naming computes and in the trust policy's condition keys.
func TestModulesUseNamingsIssuer(t *testing.T) {
	for file, want := range map[string]string{
		"baseline/main.tf":       `url = "https://` + naming.OIDCProviderHost + `"`,
		"component/main.tf":      `issuer = "` + naming.OIDCProviderHost + `"`,
		"component/variables.tf": strings.ReplaceAll(naming.OIDCProviderHost, ".", `\\.`) + `$"`,
	} {
		b, err := os.ReadFile(filepath.Join(modulesDir, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), want) {
			t.Errorf("modules/aws/%s does not contain %s: the modules and internal/naming must name the same issuer", file, want)
		}
	}
}
