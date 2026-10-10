package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// Bootstrapper applies or checks the org-level setup the IDP depends on.
type Bootstrapper struct {
	API *ghapi.Client
	Cfg Config
	Log io.Writer // one line per change

	// AllowHiddenBypass reports ruleset bypass actors the token cannot see as
	// notices instead of drift, for read-only tokens (drift workflow, ADR-0017).
	AllowHiddenBypass bool
}

// Apply makes the org match the desired state. It is safe to run any number of
// times: a second run against an unchanged org performs no writes.
func (b *Bootstrapper) Apply(ctx context.Context) error {
	steps := []func(context.Context) error{
		b.ensureOrgWorkflowPermissions,
		b.ensureRepo,
		b.ensureWetBranch,
		b.ensureRulesets,
		b.ensureEnvironments,
		b.ensureSecrets,
		b.ensureVariables,
	}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			return err
		}
	}
	return nil
}

type orgWorkflowPermissions struct {
	Default    string `json:"default_workflow_permissions"`
	CanApprove bool   `json:"can_approve_pull_request_reviews"`
}

// Read-only defaults. Letting Actions create PRs is deferred to Phase 4
// because GitHub couples it with approving PRs (spec §9.2).
var wantOrgWorkflowPermissions = orgWorkflowPermissions{Default: "read", CanApprove: false}

func (b *Bootstrapper) orgWorkflowPermissionsPath() string {
	return "/orgs/" + b.Cfg.Org + "/actions/permissions/workflow"
}

func (b *Bootstrapper) ensureOrgWorkflowPermissions(ctx context.Context) error {
	var cur orgWorkflowPermissions
	if err := b.API.Get(ctx, b.orgWorkflowPermissionsPath(), &cur); err != nil {
		return orgAdminHint(err)
	}
	if cur == wantOrgWorkflowPermissions {
		return nil
	}
	body := map[string]any{
		"default_workflow_permissions":     wantOrgWorkflowPermissions.Default,
		"can_approve_pull_request_reviews": wantOrgWorkflowPermissions.CanApprove,
	}
	if err := b.API.Put(ctx, b.orgWorkflowPermissionsPath(), body, nil); err != nil {
		return orgAdminHint(err)
	}
	b.logf("updated org %s workflow permissions to read-only", b.Cfg.Org)
	return nil
}

// orgAdminHint explains the usual cause of 401/403/404 on org settings: a gh
// token without the admin:org scope.
func orgAdminHint(err error) error {
	var apiErr *ghapi.APIError
	denied := errors.As(err, &apiErr) && (apiErr.Status == http.StatusForbidden || apiErr.Status == http.StatusUnauthorized)
	if denied || errors.Is(err, ghapi.ErrNotFound) {
		return fmt.Errorf("%w (the token needs the admin:org scope: run `gh auth refresh -s admin:org`)", err)
	}
	return err
}

func (b *Bootstrapper) ensureRepo(ctx context.Context) error {
	var repo struct {
		Visibility string `json:"visibility"`
	}
	err := b.API.Get(ctx, b.repoPath(), &repo)
	if err == nil {
		if repo.Visibility != "public" {
			return fmt.Errorf("repo %s is %q; rulesets on the GitHub Free plan require a public repo", b.repoFullName(), repo.Visibility)
		}
		return nil
	}
	if !errors.Is(err, ghapi.ErrNotFound) {
		return err
	}
	body := map[string]any{
		"name":        b.Cfg.ClaimsRepo,
		"visibility":  "public",
		"auto_init":   true,
		"description": "IDP claims: desired state on main, rendered output on wet",
	}
	if err := b.API.Post(ctx, "/orgs/"+b.Cfg.Org+"/repos", body, nil); err != nil {
		return err
	}
	b.logf("created repo %s", b.repoFullName())
	return nil
}

const wetReadme = "# wet\n\nRendered output and encrypted state. Written only by the reconcile workflow; never edit by hand.\n"

// ensureWetBranch creates wet as an orphan branch (no shared history with main).
func (b *Bootstrapper) ensureWetBranch(ctx context.Context) error {
	err := b.API.Get(ctx, b.repoPath()+"/git/ref/heads/"+WetBranch, nil)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ghapi.ErrNotFound) {
		return err
	}
	var tree, commit struct {
		SHA string `json:"sha"`
	}
	treeBody := map[string]any{"tree": []map[string]any{
		{"path": "README.md", "mode": "100644", "type": "blob", "content": wetReadme},
	}}
	if err := b.API.Post(ctx, b.repoPath()+"/git/trees", treeBody, &tree); err != nil {
		return err
	}
	commitBody := map[string]any{"message": "chore: initialize wet branch", "tree": tree.SHA, "parents": []string{}}
	if err := b.API.Post(ctx, b.repoPath()+"/git/commits", commitBody, &commit); err != nil {
		return err
	}
	refBody := map[string]any{"ref": "refs/heads/" + WetBranch, "sha": commit.SHA}
	if err := b.API.Post(ctx, b.repoPath()+"/git/refs", refBody, nil); err != nil {
		return err
	}
	b.logf("created branch %s in %s", WetBranch, b.repoFullName())
	return nil
}

func (b *Bootstrapper) repoPath() string     { return "/repos/" + b.repoFullName() }
func (b *Bootstrapper) repoFullName() string { return b.Cfg.Org + "/" + b.Cfg.ClaimsRepo }

func (b *Bootstrapper) logf(format string, args ...any) {
	fmt.Fprintf(b.Log, format+"\n", args...)
}

func (b *Bootstrapper) ensureRulesets(ctx context.Context) error {
	ids, err := b.rulesetIDs(ctx)
	if err != nil {
		return err
	}
	for _, want := range []Ruleset{MainRuleset(), WetRuleset(b.Cfg.Writer.ID)} {
		id, found := ids[want.Name]
		if !found {
			if err := b.API.Post(ctx, b.repoPath()+"/rulesets", want, nil); err != nil {
				return err
			}
			b.logf("created ruleset %s", want.Name)
			continue
		}
		drift, err := b.rulesetDrift(ctx, id, want)
		if err != nil {
			return err
		}
		if len(drift) == 0 {
			continue
		}
		if err := b.API.Put(ctx, fmt.Sprintf("%s/rulesets/%d", b.repoPath(), id), want, nil); err != nil {
			return err
		}
		b.logf("updated ruleset %s (drift at %v)", want.Name, drift)
	}
	return nil
}

func (b *Bootstrapper) ensureEnvironments(ctx context.Context) error {
	for _, env := range Environments(b.Cfg) {
		drift, err := b.environmentDrift(ctx, env)
		if err != nil {
			return err
		}
		if len(drift) == 0 {
			continue
		}
		if err := b.API.Put(ctx, b.repoPath()+"/environments/"+env.Name, env.putBody(), nil); err != nil {
			return err
		}
		if err := b.ensureBranchPolicy(ctx, env); err != nil {
			return err
		}
		b.logf("configured environment %s (drift at %v)", env.Name, drift)
	}
	return nil
}

// ensureBranchPolicy leaves exactly env.Branch as the environment's only deployment branch.
func (b *Bootstrapper) ensureBranchPolicy(ctx context.Context, env Environment) error {
	policies, err := b.branchPolicies(ctx, env.Name)
	if err != nil {
		return err
	}
	base := b.repoPath() + "/environments/" + env.Name + "/deployment-branch-policies"
	present := false
	for _, p := range policies {
		if p.Name == env.Branch {
			present = true
			continue
		}
		if err := b.API.Delete(ctx, fmt.Sprintf("%s/%d", base, p.ID)); err != nil {
			return err
		}
	}
	if present {
		return nil
	}
	return b.API.Post(ctx, base, map[string]any{"name": env.Branch, "type": "branch"}, nil)
}

// secretSpec is one secret; an empty scope means repository-level,
// otherwise the scope is an environment name.
type secretSpec struct{ scope, name, value string }

func (s secretSpec) label() string {
	if s.scope == "" {
		return s.name
	}
	return s.scope + "/" + s.name
}

func (b *Bootstrapper) secretSpecs() []secretSpec {
	return []secretSpec{
		{scope: "", name: SecretReaderKey, value: b.Cfg.Reader.PrivateKey},
		{scope: "", name: SecretPassphrase, value: b.Cfg.Passphrase},
		{scope: EnvWrite, name: SecretWriterKey, value: b.Cfg.Writer.PrivateKey},
	}
}

func (b *Bootstrapper) secretsBase(scope string) string {
	if scope == "" {
		return b.repoPath() + "/actions/secrets"
	}
	return b.repoPath() + "/environments/" + scope + "/secrets"
}

// ensureSecrets creates missing secrets. Secret values are write-only, so an
// existing secret is kept as is; rotating means deleting it in GitHub first.
func (b *Bootstrapper) ensureSecrets(ctx context.Context) error {
	for _, s := range b.secretSpecs() {
		base := b.secretsBase(s.scope)
		err := b.API.Get(ctx, base+"/"+s.name, nil)
		if err == nil {
			b.logf("kept existing secret %s (values are write-only; delete it in GitHub to rotate)", s.label())
			continue
		}
		if !errors.Is(err, ghapi.ErrNotFound) {
			return err
		}
		if err := b.createSecret(ctx, base, s); err != nil {
			return err
		}
	}
	return nil
}

// createSecret seals s.value with the scope's public key and stores it under base.
func (b *Bootstrapper) createSecret(ctx context.Context, base string, s secretSpec) error {
	var key struct {
		KeyID string `json:"key_id"`
		Key   string `json:"key"`
	}
	if err := b.API.Get(ctx, base+"/public-key", &key); err != nil {
		return err
	}
	sealed, err := Seal(key.Key, s.value)
	if err != nil {
		return err
	}
	if err := b.API.Put(ctx, base+"/"+s.name, map[string]any{"encrypted_value": sealed, "key_id": key.KeyID}, nil); err != nil {
		return err
	}
	b.logf("created secret %s", s.label())
	return nil
}

// variableSpec is one Actions variable; scope works as in secretSpec.
type variableSpec struct{ scope, name, value string }

func (v variableSpec) label() string {
	if v.scope == "" {
		return v.name
	}
	return v.scope + "/" + v.name
}

func (b *Bootstrapper) variableSpecs() []variableSpec {
	return []variableSpec{
		{scope: "", name: VarReaderClient, value: b.Cfg.Reader.ClientID},
		{scope: EnvWrite, name: VarWriterClient, value: b.Cfg.Writer.ClientID},
		{scope: "", name: VarParams, value: b.Cfg.Params().String()},
	}
}

func (b *Bootstrapper) variablesBase(scope string) string {
	if scope == "" {
		return b.repoPath() + "/actions/variables"
	}
	return b.repoPath() + "/environments/" + scope + "/variables"
}

func (b *Bootstrapper) ensureVariables(ctx context.Context) error {
	for _, v := range b.variableSpecs() {
		base := b.variablesBase(v.scope)
		var cur struct {
			Value string `json:"value"`
		}
		err := b.API.Get(ctx, base+"/"+v.name, &cur)
		switch {
		case err == nil && cur.Value == v.value:
			continue
		case err == nil:
			if err := b.API.Patch(ctx, base+"/"+v.name, map[string]any{"name": v.name, "value": v.value}, nil); err != nil {
				return err
			}
			b.logf("updated variable %s", v.label())
		case errors.Is(err, ghapi.ErrNotFound):
			if err := b.API.Post(ctx, base, map[string]any{"name": v.name, "value": v.value}, nil); err != nil {
				return err
			}
			b.logf("created variable %s", v.label())
		default:
			return err
		}
	}
	return nil
}
