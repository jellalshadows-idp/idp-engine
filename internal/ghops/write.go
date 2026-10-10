package ghops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// UpsertBotComment edits the Actions bot's comment that contains marker, or
// creates one, so a pull request keeps a single plan comment (spec §6.1).
func (r Repo) UpsertBotComment(ctx context.Context, number int, marker, body string) (int64, bool, error) {
	if !strings.Contains(body, marker) {
		return 0, false, fmt.Errorf("comment body does not contain marker %q", marker)
	}
	existing, err := r.FindBotComment(ctx, number, marker)
	if err != nil {
		return 0, false, err
	}
	payload := map[string]string{"body": body}
	if existing != nil {
		if err := r.API.Patch(ctx, fmt.Sprintf("%s/issues/comments/%d", r.base(), existing.ID), payload, nil); err != nil {
			return 0, false, err
		}
		return existing.ID, false, nil
	}
	var created Comment
	if err := r.API.Post(ctx, fmt.Sprintf("%s/issues/%d/comments", r.base(), number), payload, &created); err != nil {
		return 0, false, err
	}
	return created.ID, true, nil
}

// Issue is the part of an issue the pipelines need. The issues API also
// returns pull requests; those carry pull_request.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest *json.RawMessage `json:"pull_request"`
}

func (i Issue) hasLabel(label string) bool {
	for _, l := range i.Labels {
		if l.Name == label {
			return true
		}
	}
	return false
}

// FindOpenIssue returns the open issue (never a pull request) with label and
// title, or nil.
func (r Repo) FindOpenIssue(ctx context.Context, label, title string) (*Issue, error) {
	issues, err := ghapi.List[Issue](ctx, r.API, r.base()+"/issues?state=open&labels="+url.QueryEscape(label))
	if err != nil {
		return nil, err
	}
	for i := range issues {
		is := issues[i]
		if is.PullRequest == nil && is.State == "open" && is.Title == title && is.hasLabel(label) {
			return &is, nil
		}
	}
	return nil, nil
}

// EnsureLabel creates label unless the repository already has it.
func (r Repo) EnsureLabel(ctx context.Context, label, color, description string) error {
	err := r.API.Get(ctx, r.base()+"/labels/"+url.PathEscape(label), nil)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ghapi.ErrNotFound) {
		return err
	}
	return r.API.Post(ctx, r.base()+"/labels", map[string]string{"name": label, "color": color, "description": description}, nil)
}

// OpenIssue keeps one open issue per label and title (spec §6.4): it replaces
// the body of the existing one, or creates it.
func (r Repo) OpenIssue(ctx context.Context, label, title, body string) (int, bool, error) {
	if err := r.EnsureLabel(ctx, label, "d93f0b", "Opened by the IDP pipelines"); err != nil {
		return 0, false, err
	}
	existing, err := r.FindOpenIssue(ctx, label, title)
	if err != nil {
		return 0, false, err
	}
	if existing != nil {
		if err := r.API.Patch(ctx, fmt.Sprintf("%s/issues/%d", r.base(), existing.Number), map[string]string{"body": body}, nil); err != nil {
			return 0, false, err
		}
		return existing.Number, false, nil
	}
	var created Issue
	if err := r.API.Post(ctx, r.base()+"/issues", map[string]any{"title": title, "body": body, "labels": []string{label}}, &created); err != nil {
		return 0, false, err
	}
	return created.Number, true, nil
}

// CloseIssue closes the open issue with label and title, after adding comment
// (when not empty). It reports false when there was nothing to close.
func (r Repo) CloseIssue(ctx context.Context, label, title, comment string) (int, bool, error) {
	existing, err := r.FindOpenIssue(ctx, label, title)
	if err != nil || existing == nil {
		return 0, false, err
	}
	if comment != "" {
		if err := r.API.Post(ctx, fmt.Sprintf("%s/issues/%d/comments", r.base(), existing.Number), map[string]string{"body": comment}, nil); err != nil {
			return 0, false, err
		}
	}
	if err := r.API.Patch(ctx, fmt.Sprintf("%s/issues/%d", r.base(), existing.Number), map[string]string{"state": "closed", "state_reason": "completed"}, nil); err != nil {
		return 0, false, err
	}
	return existing.Number, true, nil
}
