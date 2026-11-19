package floci

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/awsstack"
	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
)

// rehearsal is a wet and a new render of the fixture for one scenario of
// TestReplay, in an account of its own, so the scenarios share one floci the
// way the pipelines' jobs each have a floci of their own.
type rehearsal struct {
	root, endpoint string
	runs           int
}

// renderInto renders the fixture, changed by edit, into dir with module sources
// in the modules each scratch directory copies.
func renderInto(t *testing.T, dir string, edit func(*claims.Model)) {
	t.Helper()
	m, diags, err := claims.Load(fixtureDir)
	if err != nil || len(diags) > 0 {
		t.Fatalf("%s: %v %v", fixtureDir, err, diags)
	}
	edit(m)
	files, err := render.Render(m, render.Options{ModulesDir: "../../modules"})
	if err != nil {
		t.Fatal(err)
	}
	if err := render.WriteTree(dir, files); err != nil {
		t.Fatal(err)
	}
}

// newRehearsal renders the new claims, and the wet ones unless wet is nil, as
// the first reconcile has no wet render.
func newRehearsal(t *testing.T, endpoint string, wet, fresh func(*claims.Model)) *rehearsal {
	r := &rehearsal{root: t.TempDir(), endpoint: endpoint}
	if wet != nil {
		renderInto(t, filepath.Join(r.root, "wet"), wet)
	}
	renderInto(t, filepath.Join(r.root, "new"), fresh)
	return r
}

// config is a job's view of stack: a fresh scratch directory holding the
// engine's AWS modules where the rendered relative sources point.
func (r *rehearsal) config(t *testing.T, stack string) awsstack.Config {
	t.Helper()
	r.runs++
	work := filepath.Join(r.root, fmt.Sprintf("work%d", r.runs))
	if err := os.CopyFS(filepath.Join(work, "modules", "aws"), os.DirFS(modulesDir)); err != nil {
		t.Fatal(err)
	}
	return awsstack.Config{Stack: stack, New: filepath.Join(r.root, "new"), Wet: filepath.Join(r.root, "wet"), Work: work, Endpoint: r.endpoint}
}

// inAccount gives dev the account of a scenario.
func inAccount(account string, more ...func(*claims.Model)) func(*claims.Model) {
	return func(m *claims.Model) {
		m.Platform.Environments["dev"].AWS.AccountID = account
		for _, f := range more {
			f(m)
		}
	}
}

// renamedOrg changes what every Component's trust policy names: the org.
func renamedOrg(m *claims.Model) { m.Platform.GitHub.Org = "acme-renamed" }

// withStaging adds an environment, staging, in its own account, with api in it.
func withStaging(account string) func(*claims.Model) {
	return func(m *claims.Model) {
		m.Platform.Environments["staging"] = claims.Environment{AWS: &claims.AWS{AccountID: account, Region: "eu-west-1"}}
		m.Components[0].Environments = append(m.Components[0].Environments, "staging")
	}
}

func wantPlan(t *testing.T, got *plan.Plan, want ...plan.Change) {
	t.Helper()
	if want == nil {
		want = []plan.Change{}
	}
	if !reflect.DeepEqual(got.Changes, want) {
		t.Errorf("%s: changes %v, want %v", got.Stack, got.Changes, want)
	}
}

// The Component stack's resources and the baseline's (modules/aws/*/main.tf).
var (
	repoAddr = "module.component.aws_ecr_repository.this"
	roleAddr = "module.component.aws_iam_role.ci"
	pushAddr = "module.component.aws_iam_role_policy.push"
	oidcAddr = "module.baseline.aws_iam_openid_connect_provider.this"
)

// TestReplay rehearses the pipelines' AWS jobs against floci with awsstack, the
// code that plan-aws and apply-aws run (spec §6.1 step 4, §6.2 step 3;
// ADR-0030): a new stack, a changed one, an unchanged one, an orphan destroyed
// with its wet configuration, and a plan refused because it differs from the
// gated one. It checks the results through the AWS API too.
func TestReplay(t *testing.T) {
	endpoint := os.Getenv(endpointEnv)
	if endpoint == "" {
		t.Skipf("set %s to rehearse the pipelines' AWS jobs against floci (spec §8.4); CI's floci job does", endpointEnv)
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
	runner := func(account string) awsstack.Runner {
		return awsstack.Exec{Path: f.tofu, Env: awsstack.Env(os.Environ(), account), Log: testLog{t}}
	}
	const api = "aws/dev/components/api"

	t.Run("a new stack is planned as a create", func(t *testing.T) {
		const account = "000000000101"
		r := newRehearsal(t, endpoint, nil, inAccount(account))
		// The baseline's job first: it plans and applies nothing, so the
		// Component's job, which applies the baseline, still finds the account
		// as empty as its own floci would be.
		res, err := awsstack.Plan(r.config(t, "aws/dev/_baseline"), runner(account))
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan, plan.Change{Address: oidcAddr, Action: plan.Create})
		res, err = awsstack.Plan(r.config(t, api), runner(account))
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan, plan.Change{Address: repoAddr, Action: plan.Create}, plan.Change{Address: roleAddr, Action: plan.Create}, plan.Change{Address: pushAddr, Action: plan.Create})
	})

	t.Run("a changed stack is planned against its replayed state and applied", func(t *testing.T) {
		const account = "000000000102"
		tg := target{env: "dev", accountID: account, region: "eu-west-1"}
		r := newRehearsal(t, endpoint, inAccount(account), inAccount(account, renamedOrg))
		update := plan.Change{Address: roleAddr, Action: plan.Update}
		res, err := awsstack.Apply(r.config(t, api), runner(account), plan.Fingerprint{api: {update}})
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan, update)
		f.wantTrust(t, tg, "acme-renamed/api")
	})

	t.Run("an unchanged stack plans no change", func(t *testing.T) {
		const account = "000000000103"
		r := newRehearsal(t, endpoint, inAccount(account), inAccount(account))
		res, err := awsstack.Plan(r.config(t, api), runner(account))
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan)
	})

	t.Run("an orphan is destroyed with its wet configuration", func(t *testing.T) {
		const account, staging = "000000000104", "000000000114"
		tg := target{env: "staging", accountID: staging, region: "eu-west-1"}
		r := newRehearsal(t, endpoint, inAccount(account, withStaging(staging)), inAccount(account))
		destroyed := []plan.Change{{Address: repoAddr, Action: plan.Delete}, {Address: roleAddr, Action: plan.Delete}, {Address: pushAddr, Action: plan.Delete}}
		const orphan = "aws/staging/components/api"
		res, err := awsstack.Apply(r.config(t, orphan), runner(staging), plan.Fingerprint{orphan: destroyed})
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan, destroyed...)
		f.wantMissing(t, tg, "the orphan's role after its destroy", "iam", "get-role", "--role-name", naming.RoleName(component, "staging"))
		// The orphan baseline's job runs in a floci of its own: here, an
		// account no other job used.
		const staging2 = "000000000124"
		r = newRehearsal(t, endpoint, inAccount(account, withStaging(staging2)), inAccount(account))
		res, err = awsstack.Plan(r.config(t, "aws/staging/_baseline"), runner(staging2))
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan, plan.Change{Address: oidcAddr, Action: plan.Delete})
	})

	t.Run("a plan that differs from the gated one is refused", func(t *testing.T) {
		const account = "000000000105"
		tg := target{env: "dev", accountID: account, region: "eu-west-1"}
		r := newRehearsal(t, endpoint, inAccount(account), inAccount(account, renamedOrg))
		_, err := awsstack.Apply(r.config(t, api), runner(account), plan.Fingerprint{api: {}})
		want := api + ": the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)\n  not gated: " + api + ": update " + roleAddr
		if err == nil || err.Error() != want {
			t.Fatalf("err = %v, want %q", err, want)
		}
		f.wantTrust(t, tg, "acme/api")
	})
}

// wantTrust checks that the dev CI role of tg's account trusts repo's
// environment, as the trust policy that modules/aws/component writes.
func (f *floci) wantTrust(t *testing.T, tg target, repo string) {
	t.Helper()
	roleName := naming.RoleName(component, tg.env)
	var role struct {
		Role struct{ AssumeRolePolicyDocument json.RawMessage }
	}
	if !f.awsJSON(t, tg, &role, "iam", "get-role", "--role-name", roleName) {
		t.Fatalf("get-role %s failed", roleName)
	}
	f.wantPolicy(t, "trust policy", role.Role.AssumeRolePolicyDocument, trustPolicy(naming.OIDCProviderARN(tg.accountID), repo, tg.env))
}

// testLog writes tofu's output to the test log.
type testLog struct{ t *testing.T }

func (l testLog) Write(p []byte) (int, error) {
	l.t.Log(string(p))
	return len(p), nil
}
