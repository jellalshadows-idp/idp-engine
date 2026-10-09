package bootstrap

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/nacl/box"
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

// dropRule removes every rule of the given type from a normalized ruleset.
func dropRule(m map[string]any, typ string) {
	kept := []any{}
	for _, r := range asList(m["rules"]) {
		if r.(map[string]any)["type"] != typ {
			kept = append(kept, r)
		}
	}
	m["rules"] = kept
}

func TestRulesetDriftDetectsWeakening(t *testing.T) {
	cases := []struct {
		name   string
		weaken func(live map[string]any)
		want   string // path that must be reported; "" means no mismatch at all
	}{
		{
			name:   "removed deletion rule is drift",
			weaken: func(live map[string]any) { dropRule(live, "deletion") },
			want:   "$.rules.deletion",
		},
		{
			name: "extra bypass actor is drift",
			weaken: func(live map[string]any) {
				live["bypass_actors"] = []any{map[string]any{"actor_id": float64(9), "actor_type": "User", "bypass_mode": "always"}}
			},
			want: "$.bypass_actors",
		},
		{
			name: "extra stricter rule is tolerated",
			weaken: func(live map[string]any) {
				live["rules"] = append(asList(live["rules"]), map[string]any{"type": "required_linear_history"})
			},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			desired, err := normalize(MainRuleset())
			if err != nil {
				t.Fatal(err)
			}
			liveAny, err := normalize(MainRuleset())
			if err != nil {
				t.Fatal(err)
			}
			live := liveAny.(map[string]any)
			tc.weaken(live)

			got := Mismatches(rulesetView(desired.(map[string]any)), rulesetView(live))
			if tc.want == "" {
				if len(got) != 0 {
					t.Errorf("drift = %v, want none", got)
				}
				return
			}
			if !slices.Contains(got, tc.want) {
				t.Errorf("drift = %v, want it to include %q", got, tc.want)
			}
		})
	}
}

func TestApplyFromScratchWritesInOrder(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}

	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"PUT /orgs/acme/actions/permissions/workflow",
		"POST /orgs/acme/repos",
		"POST /repos/acme/idp-claims/git/trees",
		"POST /repos/acme/idp-claims/git/commits",
		"POST /repos/acme/idp-claims/git/refs",
		"POST /repos/acme/idp-claims/rulesets",
		"POST /repos/acme/idp-claims/rulesets",
		"PUT /repos/acme/idp-claims/environments/idp-approval",
		"POST /repos/acme/idp-claims/environments/idp-approval/deployment-branch-policies",
		"PUT /repos/acme/idp-claims/environments/idp-write",
		"POST /repos/acme/idp-claims/environments/idp-write/deployment-branch-policies",
		"PUT /repos/acme/idp-claims/actions/secrets/IDP_READER_PRIVATE_KEY",
		"PUT /repos/acme/idp-claims/actions/secrets/IDP_STATE_PASSPHRASE",
		"PUT /repos/acme/idp-claims/environments/idp-write/secrets/IDP_WRITER_PRIVATE_KEY",
		"POST /repos/acme/idp-claims/actions/variables",
		"POST /repos/acme/idp-claims/environments/idp-write/variables",
	}
	if !slices.Equal(fake.writes, want) {
		t.Errorf("writes =\n%s\nwant\n%s", strings.Join(fake.writes, "\n"), strings.Join(want, "\n"))
	}
}

func TestApplyTwiceIsIdempotent(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	var log bytes.Buffer
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: &log}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	fake.writes = nil
	log.Reset()

	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if len(fake.writes) != 0 {
		t.Errorf("second apply wrote %v, want nothing", fake.writes)
	}
	if !strings.Contains(log.String(), "kept existing secret IDP_STATE_PASSPHRASE") {
		t.Errorf("log = %q, want it to say existing secrets were kept (they cannot be compared)", log.String())
	}
}

func TestApplyStoresSealedPassphraseAndWriterKeyInIdpWrite(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	cfg := validConfig()
	b := &Bootstrapper{API: api, Cfg: cfg, Log: io.Discard}
	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"/repos/acme/idp-claims/actions/secrets/IDP_STATE_PASSPHRASE":                  cfg.Passphrase,
		"/repos/acme/idp-claims/environments/idp-write/secrets/IDP_WRITER_PRIVATE_KEY": cfg.Writer.PrivateKey,
	} {
		stored, ok := fake.objects[path].(map[string]any)
		if !ok {
			t.Fatalf("%s was not stored", path)
		}
		raw, err := base64.StdEncoding.DecodeString(stored["encrypted_value"].(string))
		if err != nil {
			t.Fatal(err)
		}
		opened, ok := box.OpenAnonymous(nil, raw, testPub, testPriv)
		if !ok || string(opened) != want {
			t.Errorf("%s decrypts to %q, want %q", path, opened, want)
		}
	}
	if _, ok := fake.objects["/repos/acme/idp-claims/actions/secrets/IDP_WRITER_PRIVATE_KEY"]; ok {
		t.Error("the writer key must never be a repo-level secret (spec §7.3)")
	}
}

func TestApplyUpdatesChangedVariable(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	b.Cfg.Reader.ClientID = "Iv-reader-rotated"
	fake.writes = nil

	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	want := []string{"PATCH /repos/acme/idp-claims/actions/variables/IDP_READER_CLIENT_ID"}
	if !slices.Equal(fake.writes, want) {
		t.Errorf("writes = %v, want %v", fake.writes, want)
	}
}
