package e2e

import "testing"

func TestGroupClaim(t *testing.T) {
	got := string(GroupClaim("e2e-r1-team", "End-to-end harness team", []Member{{"owner", "maintainer"}, {"Member-One", "member"}}))
	want := "apiVersion: idp/v1\nkind: Group\nname: e2e-r1-team\ndescription: End-to-end harness team\nmembers:\n  - user: owner\n    role: maintainer\n  - user: Member-One\n    role: member\n"
	if got != want {
		t.Errorf("GroupClaim =\n%s\nwant\n%s", got, want)
	}
}

func TestComponentClaim(t *testing.T) {
	got := string(ComponentClaim("e2e-r1-app", "End-to-end harness repository", "e2e-r1-team", []string{"e2e", "harness"}))
	want := "apiVersion: idp/v1\nkind: Component\nname: e2e-r1-app\ndescription: End-to-end harness repository\nowner: group:e2e-r1-team\nenvironments: [dev]\ngithub:\n  topics: [e2e, harness]\n"
	if got != want {
		t.Errorf("ComponentClaim =\n%s\nwant\n%s", got, want)
	}
	noTopics := string(ComponentClaim("e2e-r1-app", "d", "e2e-r1-team", nil))
	if noTopics != "apiVersion: idp/v1\nkind: Component\nname: e2e-r1-app\ndescription: d\nowner: group:e2e-r1-team\nenvironments: [dev]\n" {
		t.Errorf("no topics:\n%s", noTopics)
	}
}
