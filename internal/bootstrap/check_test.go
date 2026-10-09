package bootstrap

import (
	"context"
	"io"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
)

func installBoth(fake *fakegithub.Server) {
	fake.Objects["/orgs/acme/installations"] = map[string]any{"total_count": 2, "installations": []any{
		map[string]any{"app_slug": "acme-reader"}, map[string]any{"app_slug": "acme-writer"},
	}}
}

func TestCheckCleanOrgHasNoFindings(t *testing.T) {
	fake, api := newFake(t)
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	installBoth(fake)
	fake.Writes = nil

	got, err := b.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
	if len(fake.Writes) != 0 {
		t.Errorf("check wrote %v; it must be read-only", fake.Writes)
	}
}

func TestCheckListsInstallationsWithPageSize(t *testing.T) {
	fake, api := newFake(t)
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	if _, err := b.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, want := fake.Queries["/orgs/acme/installations"], "per_page=100"; got != want {
		t.Errorf("installations query = %q, want %q", got, want)
	}
}

func TestCheckReportsDrift(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*fakegithub.Server)
		install bool
		want    []string // the exact findings, sorted
	}{
		{name: "app not installed", mutate: func(*fakegithub.Server) {}, install: false, want: []string{
			"app acme-reader: not installed on org acme",
			"app acme-writer: not installed on org acme",
		}},
		{name: "bypass added to main", install: true, mutate: func(f *fakegithub.Server) {
			f.RulesetByName("idp-main")["bypass_actors"] = []any{map[string]any{"actor_id": float64(9), "actor_type": "User", "bypass_mode": "always"}}
		}, want: []string{"ruleset idp-main: drift at [$.bypass_actors]"}},
		{name: "bypass actors hidden from token", install: true, mutate: func(f *fakegithub.Server) {
			delete(f.RulesetByName("idp-main"), "bypass_actors")
		}, want: []string{"ruleset idp-main: drift at [" + bypassHiddenMsg + "]"}},
		{name: "secret deleted", install: true, mutate: func(f *fakegithub.Server) {
			delete(f.Objects, "/repos/acme/idp-claims/actions/secrets/IDP_STATE_PASSPHRASE")
		}, want: []string{"secret IDP_STATE_PASSPHRASE: missing"}},
		{name: "writer key leaked to repo level", install: true, mutate: func(f *fakegithub.Server) {
			f.Objects["/repos/acme/idp-claims/actions/secrets/IDP_WRITER_PRIVATE_KEY"] = map[string]any{"name": "IDP_WRITER_PRIVATE_KEY"}
		}, want: []string{"secret IDP_WRITER_PRIVATE_KEY: present at repo level; it must live only in idp-write"}},
		{name: "secret in idp-approval", install: true, mutate: func(f *fakegithub.Server) {
			f.Objects["/repos/acme/idp-claims/environments/idp-approval/secrets/X"] = map[string]any{"name": "X"}
		}, want: []string{"environment idp-approval: holds 1 secret(s); it must hold none"}},
		{name: "private repo", install: true, mutate: func(f *fakegithub.Server) {
			f.Objects["/repos/acme/idp-claims"].(map[string]any)["visibility"] = "private"
		}, want: []string{`repo acme/idp-claims: visibility is "private", want public (Free-plan rulesets need public repos)`}},
		{name: "reviewer removed", install: true, mutate: func(f *fakegithub.Server) {
			f.Objects["/repos/acme/idp-claims/environments/idp-approval"].(map[string]any)["protection_rules"] = []any{}
		}, want: []string{"environment idp-approval: drift at [$.reviewer_ids]"}},
		{name: "variable changed", install: true, mutate: func(f *fakegithub.Server) {
			f.Objects["/repos/acme/idp-claims/actions/variables/IDP_READER_CLIENT_ID"] = map[string]any{"name": "IDP_READER_CLIENT_ID", "value": "other"}
		}, want: []string{"variable IDP_READER_CLIENT_ID: value differs"}},
		{name: "org defaults loosened", install: true, mutate: func(f *fakegithub.Server) {
			f.Objects["/orgs/acme/actions/permissions/workflow"] = map[string]any{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": true}
		}, want: []string{"org acme: workflow permissions are {Default:write CanApprove:true}, want {Default:read CanApprove:false}"}},
		{name: "wet branch missing", install: true, mutate: func(f *fakegithub.Server) {
			delete(f.Objects, "/repos/acme/idp-claims/git/ref/heads/wet")
		}, want: []string{"branch wet: missing"}},
		{name: "wet ruleset missing", install: true, mutate: func(f *fakegithub.Server) {
			delete(f.Rulesets, f.RulesetByName("idp-wet")["id"].(int64))
		}, want: []string{"ruleset idp-wet: missing"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, api := newFake(t)
			b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
			if err := b.Apply(context.Background()); err != nil {
				t.Fatal(err)
			}
			if tt.install {
				installBoth(fake)
			}
			tt.mutate(fake)

			found, err := b.Check(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, f := range found {
				got = append(got, f.String())
			}
			sort.Strings(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("findings =\n%s\nwant exactly\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestCheckMissingRepoStopsEarly(t *testing.T) {
	_, api := newFake(t)
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}

	got, err := b.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var sawRepo bool
	for _, f := range got {
		if f.Resource == "repo acme/idp-claims" && f.Problem == "missing" {
			sawRepo = true
		}
		if strings.HasPrefix(f.Resource, "ruleset") || strings.HasPrefix(f.Resource, "secret") {
			t.Errorf("unexpected finding %v after a missing repo", f)
		}
	}
	if !sawRepo {
		t.Errorf("findings = %v, want repo missing", got)
	}
}
