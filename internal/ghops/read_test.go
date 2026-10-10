package ghops

import (
	"context"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

func newRepo(t *testing.T) (*fakegithub.Server, Repo) {
	t.Helper()
	fake := fakegithub.New(t, "acme")
	repo, err := ParseRepo(ghapi.New(fake.URL, "test-token"), "acme/idp-claims")
	if err != nil {
		t.Fatal(err)
	}
	return fake, repo
}

func user(login, kind string) map[string]any { return map[string]any{"login": login, "type": kind} }

func TestParseRepo(t *testing.T) {
	for _, bad := range []string{"", "acme", "acme/", "/repo", "a/b/c"} {
		if _, err := ParseRepo(nil, bad); err == nil {
			t.Errorf("ParseRepo(%q) succeeded, want an error", bad)
		}
	}
}

func TestMergedPullForCommit(t *testing.T) {
	fake, repo := newRepo(t)
	merged := "2026-10-10T10:00:00Z"
	fake.Lists["/repos/acme/idp-claims/commits/m/pulls"] = []any{
		map[string]any{"number": 1, "merged_at": nil, "merge_commit_sha": "m", "head": map[string]any{"sha": "h1"}},
		map[string]any{"number": 2, "merged_at": merged, "merge_commit_sha": "other", "head": map[string]any{"sha": "h2"}},
		map[string]any{"number": 3, "merged_at": merged, "merge_commit_sha": "m", "head": map[string]any{"sha": "h3"}},
	}
	pr, err := repo.MergedPullForCommit(context.Background(), "m")
	if err != nil {
		t.Fatal(err)
	}
	if pr == nil || pr.Number != 3 || pr.Head.SHA != "h3" {
		t.Errorf("pr = %+v, want #3", pr)
	}
	none, err := repo.MergedPullForCommit(context.Background(), "unknown")
	if err != nil || none != nil {
		t.Errorf("unknown commit: pr=%+v err=%v, want nil, nil", none, err)
	}
}

func TestFindBotCommentNewestTrustedOnly(t *testing.T) {
	fake, repo := newRepo(t)
	fake.Lists["/repos/acme/idp-claims/issues/7/comments"] = []any{
		map[string]any{"id": 1, "body": "<!-- idp-plan --> old", "user": user(ActionsBot, "Bot")},
		map[string]any{"id": 2, "body": "<!-- idp-plan --> newer", "user": user(ActionsBot, "Bot")},
		map[string]any{"id": 3, "body": "<!-- idp-plan --> forged", "user": user("mallory", "User")},
		map[string]any{"id": 4, "body": "unrelated bot note", "user": user(ActionsBot, "Bot")},
		map[string]any{"id": 5, "body": "<!-- idp-plan --> same login, wrong type", "user": user(ActionsBot, "User")},
	}
	c, err := repo.FindBotComment(context.Background(), 7, "<!-- idp-plan -->")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.ID != 2 {
		t.Errorf("comment = %+v, want id 2 (newest by the Actions bot with the marker)", c)
	}
}

func TestFindBotCommentAcrossPages(t *testing.T) {
	fake, repo := newRepo(t)
	fake.MaxPerPage = 1
	fake.Lists["/repos/acme/idp-claims/issues/7/comments"] = []any{
		map[string]any{"id": 1, "body": "hi", "user": user("alice", "User")},
		map[string]any{"id": 2, "body": "hello", "user": user("bob", "User")},
		map[string]any{"id": 3, "body": "<!-- idp-plan --> plan", "user": user(ActionsBot, "Bot")},
	}
	c, err := repo.FindBotComment(context.Background(), 7, "<!-- idp-plan -->")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.ID != 3 {
		t.Errorf("comment = %+v, want id 3 from the third page", c)
	}
}
