package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// Finding is one way the live org differs from what bootstrap would create.
type Finding struct {
	Resource string
	Problem  string
}

func (f Finding) String() string { return f.Resource + ": " + f.Problem }

type findings []Finding

func (f *findings) add(resource, format string, args ...any) {
	*f = append(*f, Finding{Resource: resource, Problem: fmt.Sprintf(format, args...)})
}

// Check reports drift without changing anything (spec §9.2, run by drift §6.4).
func (b *Bootstrapper) Check(ctx context.Context) ([]Finding, error) {
	var f findings
	if err := b.checkOrg(ctx, &f); err != nil {
		return nil, err
	}
	exists, err := b.checkRepo(ctx, &f)
	if err != nil {
		return nil, err
	}
	if !exists {
		return f, nil
	}
	for _, step := range []func(context.Context, *findings) error{
		b.checkWetBranch, b.checkRulesets, b.checkEnvironments, b.checkSecrets, b.checkForbiddenSecrets, b.checkVariables,
	} {
		if err := step(ctx, &f); err != nil {
			return nil, err
		}
	}
	return f, nil
}

func (b *Bootstrapper) checkOrg(ctx context.Context, f *findings) error {
	var perms orgWorkflowPermissions
	if err := b.API.Get(ctx, b.orgWorkflowPermissionsPath(), &perms); err != nil {
		return orgAdminHint(err)
	}
	if perms != wantOrgWorkflowPermissions {
		f.add("org "+b.Cfg.Org, "workflow permissions are %+v, want %+v", perms, wantOrgWorkflowPermissions)
	}
	var inst struct {
		Installations []struct {
			AppSlug string `json:"app_slug"`
		} `json:"installations"`
	}
	if err := b.API.Get(ctx, "/orgs/"+b.Cfg.Org+"/installations?per_page=100", &inst); err != nil {
		return orgAdminHint(err)
	}
	installed := map[string]bool{}
	for _, i := range inst.Installations {
		installed[i.AppSlug] = true
	}
	for _, slug := range []string{b.Cfg.Reader.Slug, b.Cfg.Writer.Slug} {
		if !installed[slug] {
			f.add("app "+slug, "not installed on org %s", b.Cfg.Org)
		}
	}
	return nil
}

func (b *Bootstrapper) checkRepo(ctx context.Context, f *findings) (bool, error) {
	var repo struct {
		Visibility string `json:"visibility"`
	}
	err := b.API.Get(ctx, b.repoPath(), &repo)
	if errors.Is(err, ghapi.ErrNotFound) {
		f.add("repo "+b.repoFullName(), "missing")
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if repo.Visibility != "public" {
		f.add("repo "+b.repoFullName(), "visibility is %q, want public (Free-plan rulesets need public repos)", repo.Visibility)
	}
	return true, nil
}

func (b *Bootstrapper) checkWetBranch(ctx context.Context, f *findings) error {
	err := b.API.Get(ctx, b.repoPath()+"/git/ref/heads/"+WetBranch, nil)
	if errors.Is(err, ghapi.ErrNotFound) {
		f.add("branch "+WetBranch, "missing")
		return nil
	}
	return err
}

func (b *Bootstrapper) checkRulesets(ctx context.Context, f *findings) error {
	ids, err := b.rulesetIDs(ctx)
	if err != nil {
		return err
	}
	for _, want := range []Ruleset{MainRuleset(), WetRuleset(b.Cfg.Writer.ID)} {
		id, found := ids[want.Name]
		if !found {
			f.add("ruleset "+want.Name, "missing")
			continue
		}
		drift, err := b.rulesetDrift(ctx, id, want)
		if err != nil {
			return err
		}
		if len(drift) > 0 {
			f.add("ruleset "+want.Name, "drift at %v", drift)
		}
	}
	return nil
}

func (b *Bootstrapper) checkEnvironments(ctx context.Context, f *findings) error {
	for _, env := range Environments(b.Cfg) {
		drift, err := b.environmentDrift(ctx, env)
		if err != nil {
			return err
		}
		if len(drift) > 0 {
			f.add("environment "+env.Name, "drift at %v", drift)
		}
	}
	return nil
}

func (b *Bootstrapper) checkSecrets(ctx context.Context, f *findings) error {
	for _, s := range b.secretSpecs() {
		err := b.API.Get(ctx, b.secretsBase(s.scope)+"/"+s.name, nil)
		if errors.Is(err, ghapi.ErrNotFound) {
			f.add("secret "+s.label(), "missing")
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// checkForbiddenSecrets verifies the "must not exist" rules (spec §6.2, §7.3):
// the writer key lives only in idp-write, and idp-approval holds no secrets.
func (b *Bootstrapper) checkForbiddenSecrets(ctx context.Context, f *findings) error {
	err := b.API.Get(ctx, b.repoPath()+"/actions/secrets/"+SecretWriterKey, nil)
	switch {
	case err == nil:
		f.add("secret "+SecretWriterKey, "present at repo level; it must live only in idp-write")
	case !errors.Is(err, ghapi.ErrNotFound):
		return err
	}
	var list struct {
		TotalCount int `json:"total_count"`
	}
	err = b.API.Get(ctx, b.repoPath()+"/environments/"+EnvApproval+"/secrets", &list)
	if err != nil && !errors.Is(err, ghapi.ErrNotFound) {
		return err
	}
	if list.TotalCount > 0 {
		f.add("environment "+EnvApproval, "holds %d secret(s); it must hold none", list.TotalCount)
	}
	return nil
}

func (b *Bootstrapper) checkVariables(ctx context.Context, f *findings) error {
	for _, v := range b.variableSpecs() {
		var cur struct {
			Value string `json:"value"`
		}
		err := b.API.Get(ctx, b.variablesBase(v.scope)+"/"+v.name, &cur)
		if errors.Is(err, ghapi.ErrNotFound) {
			f.add("variable "+v.label(), "missing")
			continue
		}
		if err != nil {
			return err
		}
		if cur.Value != v.value {
			f.add("variable "+v.label(), "value differs")
		}
	}
	return nil
}
