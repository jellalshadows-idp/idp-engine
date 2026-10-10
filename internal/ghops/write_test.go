package ghops

import (
	"context"
	"slices"
	"testing"
)

const marker = "<!-- idp-plan -->"

func TestUpsertBotCommentCreatesThenUpdates(t *testing.T) {
	fake, repo := newRepo(t)
	ctx := context.Background()
	id, created, err := repo.UpsertBotComment(ctx, 7, marker, marker+"\nfirst")
	if err != nil || !created {
		t.Fatalf("first upsert: created=%v err=%v", created, err)
	}
	id2, created, err := repo.UpsertBotComment(ctx, 7, marker, marker+"\nsecond")
	if err != nil || created || id2 != id {
		t.Fatalf("second upsert: id=%d created=%v err=%v, want an update of %d", id2, created, err, id)
	}
	comments := fake.Lists["/repos/acme/idp-claims/issues/7/comments"]
	if len(comments) != 1 || comments[0].(map[string]any)["body"] != marker+"\nsecond" {
		t.Errorf("comments = %v, want one updated comment", comments)
	}
}

func TestUpsertBotCommentNeverEditsSomeoneElsesComment(t *testing.T) {
	fake, repo := newRepo(t)
	fake.Lists["/repos/acme/idp-claims/issues/7/comments"] = []any{
		map[string]any{"id": 1, "body": marker + " pasted by a human", "user": user("alice", "User")},
	}
	if _, created, err := repo.UpsertBotComment(context.Background(), 7, marker, marker+"\nplan"); err != nil || !created {
		t.Errorf("created=%v err=%v, want a new bot comment", created, err)
	}
	if slices.Contains(fake.Writes, "PATCH /repos/acme/idp-claims/issues/comments/1") {
		t.Error("edited a comment the bot did not write")
	}
}

func TestUpsertBotCommentRequiresTheMarker(t *testing.T) {
	_, repo := newRepo(t)
	if _, _, err := repo.UpsertBotComment(context.Background(), 7, marker, "no marker"); err == nil {
		t.Error("want an error for a body without the marker")
	}
}

func TestOpenIssueCreatesLabelAndIssue(t *testing.T) {
	fake, repo := newRepo(t)
	n, created, err := repo.OpenIssue(context.Background(), "drift", "Drift detected", "plan output")
	if err != nil || !created || n != 1 {
		t.Fatalf("n=%d created=%v err=%v", n, created, err)
	}
	if _, ok := fake.Objects["/repos/acme/idp-claims/labels/drift"]; !ok {
		t.Error("label drift was not created")
	}
}

func TestOpenIssueUpdatesExistingAcrossPages(t *testing.T) {
	fake, repo := newRepo(t)
	fake.MaxPerPage = 1
	fake.Objects["/repos/acme/idp-claims/labels/drift"] = map[string]any{"name": "drift"}
	issues := []any{}
	for i := 1; i <= 3; i++ {
		issues = append(issues, map[string]any{"number": i, "title": "other", "state": "open", "labels": []any{map[string]any{"name": "drift"}}})
	}
	issues = append(issues,
		map[string]any{"number": 4, "title": "Drift detected", "state": "open", "labels": []any{map[string]any{"name": "drift"}}, "pull_request": map[string]any{}},
		map[string]any{"number": 5, "title": "Drift detected", "state": "open", "labels": []any{map[string]any{"name": "drift"}}},
	)
	fake.Lists["/repos/acme/idp-claims/issues"] = issues
	n, created, err := repo.OpenIssue(context.Background(), "drift", "Drift detected", "new plan")
	if err != nil || created || n != 5 {
		t.Fatalf("n=%d created=%v err=%v, want an update of #5 (not the PR #4)", n, created, err)
	}
	if slices.Contains(fake.Writes, "POST /repos/acme/idp-claims/issues") {
		t.Error("created a duplicate issue")
	}
}

func TestCloseIssue(t *testing.T) {
	fake, repo := newRepo(t)
	fake.Lists["/repos/acme/idp-claims/issues"] = []any{
		map[string]any{"number": 9, "title": "Drift detected", "state": "open", "labels": []any{map[string]any{"name": "drift"}}},
	}
	n, closed, err := repo.CloseIssue(context.Background(), "drift", "Drift detected", "No drift on the last run.")
	if err != nil || !closed || n != 9 {
		t.Fatalf("n=%d closed=%v err=%v", n, closed, err)
	}
	if state := fake.Lists["/repos/acme/idp-claims/issues"][0].(map[string]any)["state"]; state != "closed" {
		t.Errorf("state = %v, want closed", state)
	}
	if len(fake.Lists["/repos/acme/idp-claims/issues/9/comments"]) != 1 {
		t.Error("closing comment missing")
	}
	if _, closed, err := repo.CloseIssue(context.Background(), "drift", "Drift detected", ""); err != nil || closed {
		t.Errorf("second close: closed=%v err=%v, want nothing to close", closed, err)
	}
}
