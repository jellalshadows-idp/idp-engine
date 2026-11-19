package floci

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/awsstack"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
)

// endpointEnv names floci's endpoint. CI's floci job sets it; without it
// TestFloci skips, so go test ./... needs no emulator.
const endpointEnv = "IDP_FLOCI_ENDPOINT"

// fakeSecret is the secret access key floci receives. floci does not check
// signatures, and its account is the 12-digit access key ID (spec §5.6, §7.6).
const fakeSecret = "floci-does-not-check-it"

// floci runs tofu and the AWS CLI against one floci, as one account at a time.
type floci struct {
	endpoint, tofu, aws, root string
}

// env is the environment of a command run as the account of tg.
func (f *floci) env(tg target) []string {
	return append(os.Environ(),
		"AWS_ACCESS_KEY_ID="+tg.accountID,
		"AWS_SECRET_ACCESS_KEY="+fakeSecret,
		"AWS_REGION="+tg.region,
		"AWS_DEFAULT_REGION="+tg.region,
		"AWS_PAGER=",
		"TF_IN_AUTOMATION=1",
	)
}

// run runs a command as the account of tg and returns its combined output and
// exit code. A command that cannot start fails the test.
func (f *floci) run(t *testing.T, tg target, dir, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = f.env(tg)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exit):
		return string(out), exit.ExitCode()
	}
	t.Fatalf("%s %v: %v", name, args, err)
	return "", -1
}

// tofuOK runs tofu in dir and fails the test unless it exits 0.
func (f *floci) tofuOK(t *testing.T, tg target, dir string, args ...string) {
	t.Helper()
	if out, code := f.run(t, tg, dir, f.tofu, args...); code != 0 {
		t.Fatalf("tofu %v in %s exited %d:\n%s", args, dir, code, out)
	}
}

// awsCLI runs an AWS CLI command against floci as the account of tg and
// returns its stdout and stderr apart, so a warning on stderr never reaches
// the JSON decoder, and its exit code. A command that cannot start fails the
// test.
func (f *floci) awsCLI(t testing.TB, tg target, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	args = append(args, "--endpoint-url", f.endpoint, "--output", "json")
	cmd := exec.Command(f.aws, args...)
	cmd.Dir = f.root
	cmd.Env = f.env(tg)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out.String(), errOut.String(), 0
	case errors.As(err, &exit):
		return out.String(), errOut.String(), exit.ExitCode()
	}
	t.Fatalf("aws %v: %v", args, err)
	return "", "", -1
}

// awsJSON runs an AWS CLI command against floci as the account of tg and
// decodes its JSON output into out. It returns false, and logs the error, when
// the command fails.
func (f *floci) awsJSON(t *testing.T, tg target, out any, args ...string) bool {
	t.Helper()
	stdout, stderr, code := f.awsCLI(t, tg, args...)
	if code != 0 {
		t.Logf("aws %v exited %d:\n%s", args, code, stderr)
		return false
	}
	if err := json.NewDecoder(strings.NewReader(stdout)).Decode(out); err != nil {
		t.Fatalf("aws %v: %v\n%s", args, err, stdout)
	}
	return true
}

// wantMissing runs an AWS CLI read that must fail with NoSuchEntity. A success,
// or a failure for any other reason, fails the test, so an unreachable or
// broken floci never passes for a missing resource (TestWantMissing).
func (f *floci) wantMissing(t testing.TB, tg target, what string, args ...string) {
	t.Helper()
	_, stderr, code := f.awsCLI(t, tg, args...)
	switch {
	case code == 0:
		t.Errorf("%s: found, want NoSuchEntity", what)
	case !strings.Contains(stderr, "NoSuchEntity"):
		t.Errorf("%s: aws exited %d without NoSuchEntity:\n%s", what, code, stderr)
	}
}

// apply creates one rendered stack of tg's environment, a slash path under
// rendered/, the way the pipelines do (spec §5.6, §6.1): it adds
// floci_override.tf.json, inits with the rendered lock file, applies, then
// replays, a plan of the same version that must find nothing to change (spec
// §8.4). It registers the stack's destroy before applying, so whatever an apply
// created is destroyed, even when the test fails.
func (f *floci) apply(t *testing.T, tg target, stack string) {
	t.Helper()
	dir := filepath.Join(f.root, "rendered", filepath.FromSlash(stack))
	if err := awsstack.WriteOverride(dir, f.endpoint); err != nil {
		t.Fatal(err)
	}
	f.tofuOK(t, tg, dir, "init", "-input=false", "-lockfile=readonly", "-no-color")
	t.Cleanup(func() {
		if out, code := f.run(t, tg, dir, f.tofu, "destroy", "-input=false", "-auto-approve", "-no-color"); code != 0 {
			t.Errorf("%s: destroy exited %d:\n%s", stack, code, out)
		}
	})
	f.tofuOK(t, tg, dir, "apply", "-input=false", "-auto-approve", "-no-color")
	if out, code := f.run(t, tg, dir, f.tofu, "plan", "-input=false", "-detailed-exitcode", "-no-color"); code != 0 {
		t.Errorf("%s: the replay plan exited %d, want 0 (no changes):\n%s", stack, code, out)
	}
}

// TestFloci is the level-3 test of the AWS modules and the AWS stacks that
// render them (spec §8.4). It renders the aws-registry fixture with local
// module sources and, against one floci, applies as each account the
// environment's rendered baseline and then its rendered Component stack, each
// with only floci_override.tf.json added. It checks through the AWS API that
// the created identities are naming's (ADR-0006), that the trust and push
// policies are exactly spec §4.4's, and that one account cannot see another's
// role (ADR-0004). It destroys everything and checks that nothing is left
// (spec §6.3).
func TestFloci(t *testing.T) {
	endpoint := os.Getenv(endpointEnv)
	if endpoint == "" {
		t.Skipf("set %s to run the level-3 tests against floci (spec §8.4); CI's floci job does", endpointEnv)
	}
	f := &floci{endpoint: endpoint, root: t.TempDir()}
	for _, tool := range []struct {
		name string
		path *string
	}{{"tofu", &f.tofu}, {"aws", &f.aws}} {
		p, err := exec.LookPath(tool.name)
		if err != nil {
			t.Fatalf("%s is not on PATH: %v", tool.name, err)
		}
		*tool.path = p
	}
	if err := os.CopyFS(filepath.Join(f.root, "modules", "aws"), os.DirFS(modulesDir)); err != nil {
		t.Fatal(err)
	}
	if err := renderFixture(f.root); err != nil {
		t.Fatal(err)
	}

	// Registered first, so it runs last, after every destroy.
	t.Cleanup(func() {
		for _, tg := range targets {
			f.wantNothingLeft(t, tg)
		}
	})
	for _, tg := range targets {
		for _, stack := range stacks(tg) {
			f.apply(t, tg, stack)
		}
	}
	for _, tg := range targets {
		t.Run(tg.env, func(t *testing.T) { f.wantCreated(t, tg) })
	}
	t.Run("accounts are isolated", func(t *testing.T) {
		dev, pro := targets[0], targets[1]
		f.wantMissing(t, pro, "account "+pro.accountID+" reading the role of account "+dev.accountID+" (ADR-0004)",
			"iam", "get-role", "--role-name", naming.RoleName(component, dev.env))
	})
}

// wantCreated checks the stacks of one environment through the AWS API. It
// compares names and ARNs, never the registry URL, whose host floci changes
// (naming.RegistryURL, ADR-0013).
func (f *floci) wantCreated(t *testing.T, tg target) {
	var providers struct {
		OpenIDConnectProviderList []struct{ Arn string }
	}
	if !f.awsJSON(t, tg, &providers, "iam", "list-open-id-connect-providers") {
		t.Fatal("list-open-id-connect-providers failed")
	}
	if got, want := providers.OpenIDConnectProviderList, naming.OIDCProviderARN(tg.accountID); len(got) != 1 || got[0].Arn != want {
		t.Errorf("OIDC providers = %+v, want exactly %s (naming.OIDCProviderARN)", got, want)
	}
	var provider struct {
		Url          string
		ClientIDList []string
	}
	if !f.awsJSON(t, tg, &provider, "iam", "get-open-id-connect-provider", "--open-id-connect-provider-arn", naming.OIDCProviderARN(tg.accountID)) {
		t.Fatal("get-open-id-connect-provider failed")
	}
	if provider.Url != naming.OIDCProviderHost || !slices.Equal(provider.ClientIDList, []string{"sts.amazonaws.com"}) {
		t.Errorf("OIDC provider = %+v, want URL %s and the client id sts.amazonaws.com", provider, naming.OIDCProviderHost)
	}

	var repos struct {
		Repositories []struct {
			RepositoryName             string
			RepositoryArn              string
			RepositoryUri              string
			ImageTagMutability         string
			ImageScanningConfiguration struct{ ScanOnPush bool }
		}
	}
	if !f.awsJSON(t, tg, &repos, "ecr", "describe-repositories") {
		t.Fatal("describe-repositories failed")
	}
	got := repos.Repositories
	if len(got) != 1 || got[0].RepositoryName != naming.RepositoryName(component) || got[0].RepositoryArn != repositoryARN(tg) ||
		got[0].ImageTagMutability != "IMMUTABLE" || !got[0].ImageScanningConfiguration.ScanOnPush {
		t.Errorf("repositories = %+v, want exactly %s (%s), IMMUTABLE and scanned on push", got, naming.RepositoryName(component), repositoryARN(tg))
	}
	for _, r := range got {
		// Logged for the phase log, never compared (ADR-0013).
		t.Logf("floci's repository URI for %s: %s", r.RepositoryName, r.RepositoryUri)
	}

	roleName := naming.RoleName(component, tg.env)
	var role struct {
		Role struct {
			Arn                      string
			AssumeRolePolicyDocument json.RawMessage
		}
	}
	if !f.awsJSON(t, tg, &role, "iam", "get-role", "--role-name", roleName) {
		t.Fatalf("get-role %s failed", roleName)
	}
	if want := naming.RoleARN(tg.accountID, component, tg.env); role.Role.Arn != want {
		t.Errorf("role ARN = %s, want %s (naming.RoleARN)", role.Role.Arn, want)
	}
	f.wantPolicy(t, "trust policy", role.Role.AssumeRolePolicyDocument, trustPolicy(naming.OIDCProviderARN(tg.accountID), githubRepository(), tg.env))

	var inline struct{ PolicyNames []string }
	var attached struct{ AttachedPolicies []struct{ PolicyArn string } }
	if !f.awsJSON(t, tg, &inline, "iam", "list-role-policies", "--role-name", roleName) ||
		!f.awsJSON(t, tg, &attached, "iam", "list-attached-role-policies", "--role-name", roleName) {
		t.Fatal("listing the role's policies failed")
	}
	if !slices.Equal(inline.PolicyNames, []string{"ecr-push"}) || len(attached.AttachedPolicies) != 0 {
		t.Errorf("role policies: inline %v, attached %+v; want only the inline ecr-push", inline.PolicyNames, attached.AttachedPolicies)
	}
	var push struct{ PolicyDocument json.RawMessage }
	if !f.awsJSON(t, tg, &push, "iam", "get-role-policy", "--role-name", roleName, "--policy-name", "ecr-push") {
		t.Fatal("get-role-policy ecr-push failed")
	}
	f.wantPolicy(t, "push policy", push.PolicyDocument, pushPolicy(repositoryARN(tg)))
}

func (f *floci) wantPolicy(t *testing.T, what string, raw json.RawMessage, want policy) {
	t.Helper()
	got, err := decodePolicy(raw)
	if err != nil {
		t.Errorf("%s: %v", what, err)
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %+v, want exactly %+v (spec §4.4)", what, got, want)
	}
}

// wantNothingLeft checks, after the destroys, that tg's account holds no OIDC
// provider, repository or CI role: deleting a Component destroys its
// repository and role (spec §6.3).
func (f *floci) wantNothingLeft(t *testing.T, tg target) {
	var providers struct {
		OpenIDConnectProviderList []struct{ Arn string }
	}
	if !f.awsJSON(t, tg, &providers, "iam", "list-open-id-connect-providers") || len(providers.OpenIDConnectProviderList) != 0 {
		t.Errorf("%s: OIDC providers left after destroy: %+v", tg.env, providers.OpenIDConnectProviderList)
	}
	var repos struct {
		Repositories []struct{ RepositoryName string }
	}
	if !f.awsJSON(t, tg, &repos, "ecr", "describe-repositories") || len(repos.Repositories) != 0 {
		t.Errorf("%s: repositories left after destroy: %+v", tg.env, repos.Repositories)
	}
	f.wantMissing(t, tg, tg.env+": role "+naming.RoleName(component, tg.env)+" after destroy",
		"iam", "get-role", "--role-name", naming.RoleName(component, tg.env))
}
