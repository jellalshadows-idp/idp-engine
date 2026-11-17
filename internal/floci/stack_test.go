package floci

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

func TestWriteStack(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dev", "component")
	tg := targets[0]
	if err := writeStack(dir, "component", tg, "http://localhost:4566", componentInputs(tg)); err != nil {
		t.Fatal(err)
	}
	read := func(name string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return v
	}
	wantMain := map[string]any{
		"module": map[string]any{"component": map[string]any{
			"source":            "../../modules/aws/component",
			"github_repository": "acme/api",
			"environment":       "dev",
			"repository_name":   "api",
			"role_name":         "api-dev-ci",
			"oidc_provider_arn": "arn:aws:iam::000000000001:oidc-provider/token.actions.githubusercontent.com",
		}},
		"provider": map[string]any{"aws": map[string]any{"region": "eu-west-1"}},
	}
	if got := read("main.tf.json"); !reflect.DeepEqual(got, wantMain) {
		t.Errorf("main.tf.json = %v, want %v", got, wantMain)
	}
	wantOverride := map[string]any{"provider": map[string]any{"aws": map[string]any{
		"endpoints":                   map[string]any{"ecr": "http://localhost:4566", "iam": "http://localhost:4566", "sts": "http://localhost:4566"},
		"skip_credentials_validation": true,
		"skip_metadata_api_check":     "true",
		"skip_requesting_account_id":  true,
	}}}
	if got := read("floci_override.tf.json"); !reflect.DeepEqual(got, wantOverride) {
		t.Errorf("floci_override.tf.json = %v, want %v", got, wantOverride)
	}
	lock, err := os.ReadFile(filepath.Join(dir, ".terraform.lock.hcl"))
	if err != nil || !bytes.Equal(lock, lockfiles.AWS) {
		t.Errorf("the stack must carry the engine's AWS lock file (err %v)", err)
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
