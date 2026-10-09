package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// rulesetView keeps the fields bootstrap owns and indexes rules by type, so
// GitHub's rule ordering and extra fields never count as drift. A rule someone
// removed does count; an extra rule someone added (stricter) does not.
func rulesetView(r map[string]any) map[string]any {
	rules := map[string]any{}
	for _, raw := range asList(r["rules"]) {
		if rule, ok := raw.(map[string]any); ok {
			if t, ok := rule["type"].(string); ok {
				rules[t] = rule
			}
		}
	}
	return map[string]any{
		"name":          r["name"],
		"target":        r["target"],
		"enforcement":   r["enforcement"],
		"bypass_actors": asList(r["bypass_actors"]),
		"conditions":    r["conditions"],
		"rules":         rules,
	}
}

func (b *Bootstrapper) rulesetIDs(ctx context.Context) (map[string]int64, error) {
	var list []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	// per_page=100 is a single page by design: bootstrap manages 2 rulesets per repo,
	// so pagination is intentionally not implemented.
	if err := b.API.Get(ctx, b.repoPath()+"/rulesets?includes_parents=false&per_page=100", &list); err != nil {
		return nil, err
	}
	ids := make(map[string]int64, len(list))
	for _, r := range list {
		ids[r.Name] = r.ID
	}
	return ids, nil
}

func (b *Bootstrapper) rulesetDrift(ctx context.Context, id int64, want Ruleset) ([]string, error) {
	var live map[string]any
	if err := b.API.Get(ctx, fmt.Sprintf("%s/rulesets/%d", b.repoPath(), id), &live); err != nil {
		return nil, err
	}
	desired, err := normalize(want)
	if err != nil {
		return nil, err
	}
	drift := Mismatches(rulesetView(desired.(map[string]any)), rulesetView(live))
	// GitHub returns bypass_actors only to callers with write access. A live
	// ruleset with the key absent or null means we cannot verify it, so fail
	// closed instead of comparing against []. Present-but-empty still compares
	// normally. For apply this triggers a PUT, which GitHub rejects for a token
	// without write access; that is acceptable (apply needs a write token anyway).
	if v, ok := live["bypass_actors"]; !ok || v == nil {
		drift = withoutPath(drift, "$.bypass_actors")
		drift = append(drift, bypassHiddenDrift)
	}
	return drift, nil
}

const bypassHiddenDrift = "$.bypass_actors (not returned to this token; GitHub only shows bypass actors to callers with write access)"

// withoutPath drops generic mismatches at path (and below) so the dedicated
// hidden-field entry is not accompanied by a misleading comparison against [].
func withoutPath(paths []string, path string) []string {
	kept := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == path || strings.HasPrefix(p, path+".") || strings.HasPrefix(p, path+"[") {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

type envView struct {
	ReviewerIDs          []int64  `json:"reviewer_ids"`
	ProtectedBranches    bool     `json:"protected_branches"`
	CustomBranchPolicies bool     `json:"custom_branch_policies"`
	Branches             []string `json:"branches"`
}

func (e Environment) view() envView {
	ids := append([]int64{}, e.ReviewerIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return envView{ReviewerIDs: ids, ProtectedBranches: false, CustomBranchPolicies: true, Branches: []string{e.Branch}}
}

type branchPolicy struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// branchPolicies lists an environment's deployment branch policies. GitHub
// answers 404 when the environment has no custom policies yet.
func (b *Bootstrapper) branchPolicies(ctx context.Context, env string) ([]branchPolicy, error) {
	var resp struct {
		BranchPolicies []branchPolicy `json:"branch_policies"`
	}
	err := b.API.Get(ctx, b.repoPath()+"/environments/"+env+"/deployment-branch-policies", &resp)
	if errors.Is(err, ghapi.ErrNotFound) {
		return nil, nil
	}
	return resp.BranchPolicies, err
}

// environmentDrift compares the live environment with want. A missing
// environment is reported as the single path "missing".
func (b *Bootstrapper) environmentDrift(ctx context.Context, want Environment) ([]string, error) {
	var live struct {
		ProtectionRules []struct {
			Type      string `json:"type"`
			Reviewers []struct {
				Reviewer struct {
					ID int64 `json:"id"`
				} `json:"reviewer"`
			} `json:"reviewers"`
		} `json:"protection_rules"`
		DeploymentBranchPolicy *struct {
			ProtectedBranches    bool `json:"protected_branches"`
			CustomBranchPolicies bool `json:"custom_branch_policies"`
		} `json:"deployment_branch_policy"`
	}
	err := b.API.Get(ctx, b.repoPath()+"/environments/"+want.Name, &live)
	if errors.Is(err, ghapi.ErrNotFound) {
		return []string{"missing"}, nil
	}
	if err != nil {
		return nil, err
	}
	got := envView{ReviewerIDs: []int64{}, Branches: []string{}}
	for _, rule := range live.ProtectionRules {
		if rule.Type != "required_reviewers" {
			continue
		}
		for _, r := range rule.Reviewers {
			got.ReviewerIDs = append(got.ReviewerIDs, r.Reviewer.ID)
		}
	}
	sort.Slice(got.ReviewerIDs, func(i, j int) bool { return got.ReviewerIDs[i] < got.ReviewerIDs[j] })
	if live.DeploymentBranchPolicy != nil {
		got.ProtectedBranches = live.DeploymentBranchPolicy.ProtectedBranches
		got.CustomBranchPolicies = live.DeploymentBranchPolicy.CustomBranchPolicies
	}
	policies, err := b.branchPolicies(ctx, want.Name)
	if err != nil {
		return nil, err
	}
	for _, p := range policies {
		got.Branches = append(got.Branches, p.Name)
	}
	sort.Strings(got.Branches)
	desired, err := normalize(want.view())
	if err != nil {
		return nil, err
	}
	actual, err := normalize(got)
	if err != nil {
		return nil, err
	}
	return Mismatches(desired, actual), nil
}
