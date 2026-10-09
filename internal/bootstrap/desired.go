package bootstrap

// Names shared with the Phase 1 reusable workflows. Renaming one is a breaking change.
const (
	WetBranch        = "wet"
	EnvApproval      = "idp-approval"
	EnvWrite         = "idp-write"
	GateCheck        = "idp-gate"
	SecretReaderKey  = "IDP_READER_PRIVATE_KEY"
	SecretPassphrase = "IDP_STATE_PASSPHRASE"
	SecretWriterKey  = "IDP_WRITER_PRIVATE_KEY"
	VarReaderClient  = "IDP_READER_CLIENT_ID"
	VarWriterClient  = "IDP_WRITER_CLIENT_ID"
)

// Ruleset is the part of GitHub's ruleset API that bootstrap owns.
type Ruleset struct {
	Name         string        `json:"name"`
	Target       string        `json:"target"`
	Enforcement  string        `json:"enforcement"`
	BypassActors []BypassActor `json:"bypass_actors"`
	Conditions   Conditions    `json:"conditions"`
	Rules        []Rule        `json:"rules"`
}

// BypassActor may skip the ruleset.
type BypassActor struct {
	ActorID    int64  `json:"actor_id"`
	ActorType  string `json:"actor_type"`
	BypassMode string `json:"bypass_mode"`
}

// Conditions select the refs a ruleset applies to.
type Conditions struct {
	RefName RefName `json:"ref_name"`
}

// RefName lists ref patterns to include and exclude.
type RefName struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

// Rule is one ruleset rule; Parameters is omitted for rules without any.
type Rule struct {
	Type       string         `json:"type"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

// MainRuleset protects the claims repo's default branch: PR, idp-gate,
// up to date, no force push, no deletion, and no bypass at all (spec §7.4).
// Approval of applies lives in the idp-approval environment, so PRs need
// zero review approvals here.
func MainRuleset() Ruleset {
	return Ruleset{
		Name:         "idp-main",
		Target:       "branch",
		Enforcement:  "active",
		BypassActors: []BypassActor{},
		Conditions:   Conditions{RefName: RefName{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{}}},
		Rules: []Rule{
			{Type: "deletion"},
			{Type: "non_fast_forward"},
			{Type: "pull_request", Parameters: map[string]any{
				"dismiss_stale_reviews_on_push":     false,
				"require_code_owner_review":         false,
				"require_last_push_approval":        false,
				"required_approving_review_count":   0,
				"required_review_thread_resolution": false,
			}},
			{Type: "required_status_checks", Parameters: map[string]any{
				"strict_required_status_checks_policy": true,
				"required_status_checks":               []map[string]any{{"context": GateCheck}},
			}},
		},
	}
}

// WetRuleset lets only the writer App update the wet branch (spec §7.4).
func WetRuleset(writerAppID int64) Ruleset {
	return Ruleset{
		Name:         "idp-wet",
		Target:       "branch",
		Enforcement:  "active",
		BypassActors: []BypassActor{{ActorID: writerAppID, ActorType: "Integration", BypassMode: "always"}},
		Conditions:   Conditions{RefName: RefName{Include: []string{"refs/heads/" + WetBranch}, Exclude: []string{}}},
		Rules: []Rule{
			{Type: "deletion"},
			{Type: "non_fast_forward"},
			{Type: "update", Parameters: map[string]any{"update_allows_fetch_and_merge": false}},
		},
	}
}

// Environment is the desired shape of one deployment environment.
type Environment struct {
	Name        string
	ReviewerIDs []int64 // users; empty means no required reviewers
	Branch      string  // the only branch allowed to deploy
}

// Environments returns idp-approval (reviewers, no secrets) and idp-write
// (writer key, no reviewers), both restricted to main (spec §6.2, §7.3).
func Environments(cfg Config) []Environment {
	return []Environment{
		{Name: EnvApproval, ReviewerIDs: []int64{cfg.ApproverID}, Branch: "main"},
		{Name: EnvWrite, ReviewerIDs: []int64{}, Branch: "main"},
	}
}

// putBody is the JSON for PUT /repos/{owner}/{repo}/environments/{name}.
func (e Environment) putBody() map[string]any {
	reviewers := []map[string]any{}
	for _, id := range e.ReviewerIDs {
		reviewers = append(reviewers, map[string]any{"type": "User", "id": id})
	}
	return map[string]any{
		"wait_timer":          0,
		"prevent_self_review": false,
		"reviewers":           reviewers,
		"deployment_branch_policy": map[string]any{
			"protected_branches":     false,
			"custom_branch_policies": true,
		},
	}
}
