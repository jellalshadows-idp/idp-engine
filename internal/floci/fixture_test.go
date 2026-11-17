package floci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
)

// fixtureDir is the claims repo whose render TestFloci applies: the renderer's
// aws-registry fixture, whose golden files pin the same stacks.
const fixtureDir = "../render/testdata/aws-registry"

// modulesDir is the engine's AWS modules, seen from this package's directory.
const modulesDir = "../../modules/aws"

// target is one environment of the test platform: its account and region.
type target struct {
	env, accountID, region string
}

// The test platform, written out so that TestFloci's checks never take their
// expectations from the render they check: one registry Component, api of the
// GitHub org acme, in two environments in two accounts of one floci
// (ADR-0004). TestTheFixtureIsTheTestPlatform keeps it equal to the fixture.
const (
	org       = "acme"
	component = "api"
)

var targets = []target{
	{env: "dev", accountID: "000000000001", region: "eu-west-1"},
	{env: "pro", accountID: "000000000003", region: "eu-west-1"},
}

// githubRepository is the Component's repository, <org>/<name>.
func githubRepository() string { return org + "/" + component }

// repositoryARN is the ARN AWS gives the Component's repository. naming has no
// function for it: the push policy reads it from the repository resource.
func repositoryARN(tg target) string {
	return "arn:aws:ecr:" + tg.region + ":" + tg.accountID + ":repository/" + naming.RepositoryName(component)
}

// stacks are the rendered AWS stacks of tg's environment, as slash paths under
// rendered/, in apply order: the baseline, whose OIDC provider the Component's
// role trusts, then the Component's stack.
func stacks(tg target) []string {
	return []string{"aws/" + tg.env + "/_baseline", "aws/" + tg.env + "/components/" + component}
}

// renderFixture renders the fixture into root/rendered the way CI's
// render-smoke does, with module sources in root/modules, which the caller
// fills with the engine's modules.
func renderFixture(root string) error {
	m, diags, err := claims.Load(fixtureDir)
	if err != nil {
		return err
	}
	if len(diags) > 0 {
		return fmt.Errorf("%s: %v", fixtureDir, diags)
	}
	files, err := render.Render(m, render.Options{ModulesDir: "../../modules"})
	if err != nil {
		return err
	}
	return render.WriteTree(filepath.Join(root, "rendered"), files)
}

// writeOverride writes into a rendered AWS stack what CI adds for floci (spec
// §5.6): floci_override.tf.json, with the endpoint and the provider flags that
// OpenTofu merges into the stack's provider block.
func writeOverride(dir, endpoint string) error {
	b, err := json.MarshalIndent(map[string]any{
		"provider": map[string]any{"aws": map[string]any{
			"endpoints":                   map[string]any{"ecr": endpoint, "iam": endpoint, "sts": endpoint},
			"skip_credentials_validation": true,
			"skip_metadata_api_check":     "true",
			"skip_requesting_account_id":  true,
		}},
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "floci_override.tf.json"), append(b, '\n'), 0o644)
}
