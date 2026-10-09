package bootstrap

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestApplyCreatesOrgSettingsRepoAndWetBranch(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	var log bytes.Buffer
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: &log}

	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"PUT /orgs/acme/actions/permissions/workflow",
		"POST /orgs/acme/repos",
		"POST /repos/acme/idp-claims/git/trees",
		"POST /repos/acme/idp-claims/git/commits",
		"POST /repos/acme/idp-claims/git/refs",
	} {
		if !slices.Contains(fake.writes, want) {
			t.Errorf("missing write %q in %v", want, fake.writes)
		}
	}
	perms := fake.objects["/orgs/acme/actions/permissions/workflow"].(map[string]any)
	if perms["default_workflow_permissions"] != "read" || perms["can_approve_pull_request_reviews"] != false {
		t.Errorf("org workflow permissions = %v, want read-only and no PR approval", perms)
	}
	repo := fake.objects["/repos/acme/idp-claims"].(map[string]any)
	if repo["visibility"] != "public" {
		t.Errorf("repo visibility = %v, want public", repo["visibility"])
	}
	if _, ok := fake.objects["/repos/acme/idp-claims/git/ref/heads/wet"]; !ok {
		t.Error("wet branch was not created")
	}
	if !strings.Contains(log.String(), "created repo acme/idp-claims") {
		t.Errorf("log = %q", log.String())
	}
}

func TestApplyExplainsMissingAdminOrgScope(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	fake.forbidden["/orgs/acme/actions/permissions/workflow"] = true
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}

	err := b.Apply(context.Background())
	if err == nil || !strings.Contains(err.Error(), "gh auth refresh -s admin:org") {
		t.Fatalf("err = %v, want a hint to add the admin:org scope", err)
	}
}

func TestApplyCreatesRulesetsAndEnvironments(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}

	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.rulesetByName("idp-main") == nil || fake.rulesetByName("idp-wet") == nil {
		t.Fatalf("rulesets = %v, want idp-main and idp-wet", fake.rulesets)
	}
	wet := fake.rulesetByName("idp-wet")
	actor := asList(wet["bypass_actors"])[0].(map[string]any)
	if actor["actor_id"] != float64(2) || actor["actor_type"] != "Integration" {
		t.Errorf("idp-wet bypass = %v, want the writer app (id 2)", actor)
	}
	approval := fake.objects["/repos/acme/idp-claims/environments/idp-approval"].(map[string]any)
	if len(asList(approval["protection_rules"])) != 1 {
		t.Errorf("idp-approval protection rules = %v, want one required_reviewers rule", approval["protection_rules"])
	}
	for _, env := range []string{"idp-approval", "idp-write"} {
		policies := asList(fake.objects["/repos/acme/idp-claims/environments/"+env+"/deployment-branch-policies"])
		if len(policies) != 1 || policies[0].(map[string]any)["name"] != "main" {
			t.Errorf("%s branch policies = %v, want only main", env, policies)
		}
	}
}

func TestApplyRepairsRulesetDrift(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	// Someone adds a bypass actor to idp-main by hand.
	main := fake.rulesetByName("idp-main")
	main["bypass_actors"] = []any{map[string]any{"actor_id": float64(9), "actor_type": "User", "bypass_mode": "always"}}
	fake.writes = nil

	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	want := "PUT /repos/acme/idp-claims/rulesets/" + strconv.FormatInt(int64(main["id"].(int64)), 10)
	if !slices.Equal(fake.writes, []string{want}) {
		t.Errorf("writes = %v, want only %q", fake.writes, want)
	}
}

func TestApplyRemovesExtraBranchPolicy(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	path := "/repos/acme/idp-claims/environments/idp-write/deployment-branch-policies"
	fake.objects[path] = append(asList(fake.objects[path]), map[string]any{"id": float64(999), "name": "dev"})
	fake.writes = nil

	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(fake.writes, "DELETE "+path+"/999") {
		t.Errorf("writes = %v, want the dev policy deleted", fake.writes)
	}
}

func TestRulesetDriftIgnoresGitHubExtras(t *testing.T) {
	desired, err := normalize(MainRuleset())
	if err != nil {
		t.Fatal(err)
	}
	live := decode(t, `{
	  "id": 7, "name": "idp-main", "target": "branch", "source_type": "Repository", "source": "acme/idp-claims",
	  "enforcement": "active", "node_id": "RRS_x", "created_at": "2026-10-08T00:00:00Z",
	  "_links": {"self": {"href": "https://api.github.com/x"}},
	  "conditions": {"ref_name": {"include": ["~DEFAULT_BRANCH"], "exclude": []}},
	  "rules": [
	    {"type": "required_status_checks", "parameters": {"strict_required_status_checks_policy": true, "do_not_enforce_on_create": false, "required_status_checks": [{"context": "idp-gate", "integration_id": null}]}},
	    {"type": "pull_request", "parameters": {"allowed_merge_methods": ["merge", "squash", "rebase"], "automatic_copilot_code_review_enabled": false, "dismiss_stale_reviews_on_push": false, "require_code_owner_review": false, "require_last_push_approval": false, "required_approving_review_count": 0, "required_review_thread_resolution": false, "required_reviewers": []}},
	    {"type": "non_fast_forward"},
	    {"type": "deletion"}
	  ]
	}`).(map[string]any)

	if got := Mismatches(rulesetView(desired.(map[string]any)), rulesetView(live)); len(got) != 0 {
		t.Errorf("drift = %v, want none (extra fields, omitted bypass_actors and rule order must not count)", got)
	}
}
