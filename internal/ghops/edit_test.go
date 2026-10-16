package ghops

import (
	"context"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

func TestEditedByOthers(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	fake.CommentEditors = map[string]map[string]any{
		"BOT":      {"login": "github-actions", "__typename": "Bot"},
		"PERSON":   {"login": "mallory", "__typename": "User"},
		"OTHERBOT": {"login": "renovate", "__typename": "Bot"},
	}
	repo := Repo{API: ghapi.New(fake.URL, "t"), Owner: "acme", Name: "idp-claims"}
	tests := []struct {
		node   string
		edited bool
		editor string
	}{
		{"NEVER", false, ""},
		{"BOT", false, ""},
		{"PERSON", true, "mallory"},
		{"OTHERBOT", true, "renovate"},
	}
	for _, tt := range tests {
		edited, editor, err := repo.EditedByOthers(context.Background(), tt.node)
		if err != nil || edited != tt.edited || editor != tt.editor {
			t.Errorf("%s: EditedByOthers = %v, %q, %v; want %v, %q", tt.node, edited, editor, err, tt.edited, tt.editor)
		}
	}
	if len(fake.Writes) != 0 {
		t.Errorf("a GraphQL query is a read, got writes %v", fake.Writes)
	}
}
