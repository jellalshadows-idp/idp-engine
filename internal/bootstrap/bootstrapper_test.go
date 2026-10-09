package bootstrap

import (
	"bytes"
	"context"
	"io"
	"slices"
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
