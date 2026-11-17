package floci

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

// modulesDir is the engine's AWS modules, seen from this package's directory.
const modulesDir = "../../modules/aws"

// target is one environment of the test platform: its account and region.
type target struct {
	env, accountID, region string
}

// The test platform: one registry Component, api of the GitHub org acme, in two
// environments in two accounts of one floci (ADR-0004).
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

// componentInputs are the inputs modules/aws/component takes in an environment.
// They come from internal/naming, as Phase 2d's render will pass them (spec
// §5.5), so a module that does not create exactly these names fails here.
func componentInputs(tg target) map[string]any {
	return map[string]any{
		"github_repository": githubRepository(),
		"environment":       tg.env,
		"repository_name":   naming.RepositoryName(component),
		"role_name":         naming.RoleName(component, tg.env),
		"oidc_provider_arn": naming.OIDCProviderARN(tg.accountID),
	}
}

// repositoryARN is the ARN AWS gives the Component's repository. naming has no
// function for it: the push policy reads it from the repository resource.
func repositoryARN(tg target) string {
	return "arn:aws:ecr:" + tg.region + ":" + tg.accountID + ":repository/" + naming.RepositoryName(component)
}

// writeStack writes the root config of one stack into dir, which sits two
// levels below the copied modules: the module call with its inputs and the
// region, as the render will write it, then what CI adds for floci (spec
// §5.6): floci_override.tf.json, with the endpoint and the provider flags, and
// the engine's lock file.
func writeStack(dir, module string, tg target, endpoint string, inputs map[string]any) error {
	call := map[string]any{"source": "../../modules/aws/" + module}
	maps.Copy(call, inputs)
	files := map[string]any{
		"main.tf.json": map[string]any{
			"module":   map[string]any{module: call},
			"provider": map[string]any{"aws": map[string]any{"region": tg.region}},
		},
		"floci_override.tf.json": map[string]any{
			"provider": map[string]any{"aws": map[string]any{
				"endpoints":                   map[string]any{"ecr": endpoint, "iam": endpoint, "sts": endpoint},
				"skip_credentials_validation": true,
				"skip_metadata_api_check":     "true",
				"skip_requesting_account_id":  true,
			}},
		},
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, doc := range files {
		b, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, ".terraform.lock.hcl"), lockfiles.AWS, 0o644)
}
