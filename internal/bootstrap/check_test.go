package bootstrap

import (
	"context"
	"io"
	"strings"
	"testing"
)

func installBoth(fake *fakeGitHub) {
	fake.objects["/orgs/acme/installations"] = map[string]any{"total_count": 2, "installations": []any{
		map[string]any{"app_slug": "acme-reader"}, map[string]any{"app_slug": "acme-writer"},
	}}
}

func TestCheckCleanOrgHasNoFindings(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	installBoth(fake)
	fake.writes = nil

	findings, err := b.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
	if len(fake.writes) != 0 {
		t.Errorf("check wrote %v; it must be read-only", fake.writes)
	}
}

func TestCheckReportsDrift(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*fakeGitHub)
		install bool
		want    string
	}{
		{name: "app not installed", mutate: func(*fakeGitHub) {}, install: false, want: "app acme-writer: not installed on org acme"},
		{name: "bypass added to main", install: true, mutate: func(f *fakeGitHub) {
			f.rulesetByName("idp-main")["bypass_actors"] = []any{map[string]any{"actor_id": float64(9), "actor_type": "User", "bypass_mode": "always"}}
		}, want: "ruleset idp-main: drift at"},
		{name: "secret deleted", install: true, mutate: func(f *fakeGitHub) {
			delete(f.objects, "/repos/acme/idp-claims/actions/secrets/IDP_STATE_PASSPHRASE")
		}, want: "secret IDP_STATE_PASSPHRASE: missing"},
		{name: "writer key leaked to repo level", install: true, mutate: func(f *fakeGitHub) {
			f.objects["/repos/acme/idp-claims/actions/secrets/IDP_WRITER_PRIVATE_KEY"] = map[string]any{"name": "IDP_WRITER_PRIVATE_KEY"}
		}, want: "secret IDP_WRITER_PRIVATE_KEY: present at repo level; it must live only in idp-write"},
		{name: "secret in idp-approval", install: true, mutate: func(f *fakeGitHub) {
			f.objects["/repos/acme/idp-claims/environments/idp-approval/secrets/X"] = map[string]any{"name": "X"}
		}, want: "environment idp-approval: holds 1 secret(s); it must hold none"},
		{name: "private repo", install: true, mutate: func(f *fakeGitHub) {
			f.objects["/repos/acme/idp-claims"].(map[string]any)["visibility"] = "private"
		}, want: `repo acme/idp-claims: visibility is "private", want public`},
		{name: "reviewer removed", install: true, mutate: func(f *fakeGitHub) {
			f.objects["/repos/acme/idp-claims/environments/idp-approval"].(map[string]any)["protection_rules"] = []any{}
		}, want: "environment idp-approval: drift at"},
		{name: "variable changed", install: true, mutate: func(f *fakeGitHub) {
			f.objects["/repos/acme/idp-claims/actions/variables/IDP_READER_CLIENT_ID"] = map[string]any{"name": "IDP_READER_CLIENT_ID", "value": "other"}
		}, want: "variable IDP_READER_CLIENT_ID: value differs"},
		{name: "org defaults loosened", install: true, mutate: func(f *fakeGitHub) {
			f.objects["/orgs/acme/actions/permissions/workflow"] = map[string]any{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": true}
		}, want: "org acme: workflow permissions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, api := newFakeGitHub(t, "acme")
			b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
			if err := b.Apply(context.Background()); err != nil {
				t.Fatal(err)
			}
			if tt.install {
				installBoth(fake)
			}
			tt.mutate(fake)

			findings, err := b.Check(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			for _, f := range findings {
				lines = append(lines, f.String())
			}
			if !strings.Contains(strings.Join(lines, "\n"), tt.want) {
				t.Errorf("findings =\n%s\nwant one containing %q", strings.Join(lines, "\n"), tt.want)
			}
		})
	}
}

func TestCheckMissingRepoStopsEarly(t *testing.T) {
	_, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}

	findings, err := b.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var sawRepo bool
	for _, f := range findings {
		if f.Resource == "repo acme/idp-claims" && f.Problem == "missing" {
			sawRepo = true
		}
		if strings.HasPrefix(f.Resource, "ruleset") || strings.HasPrefix(f.Resource, "secret") {
			t.Errorf("unexpected finding %v after a missing repo", f)
		}
	}
	if !sawRepo {
		t.Errorf("findings = %v, want repo missing", findings)
	}
}
