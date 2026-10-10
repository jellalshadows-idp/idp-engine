// Package ghops holds the pipelines' conversations with GitHub: the pull
// request behind a commit, the bot's sticky plan comment, and labelled issues.
package ghops

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// ActionsBot authors every comment made with a workflow's GITHUB_TOKEN.
const ActionsBot = "github-actions[bot]"

// Repo is one repository reached through API.
type Repo struct {
	API   *ghapi.Client
	Owner string
	Name  string
}

// ParseRepo splits "OWNER/NAME".
func ParseRepo(api *ghapi.Client, full string) (Repo, error) {
	owner, name, ok := strings.Cut(full, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return Repo{}, fmt.Errorf("repo %q: want OWNER/NAME", full)
	}
	return Repo{API: api, Owner: owner, Name: name}, nil
}

func (r Repo) base() string {
	return "/repos/" + url.PathEscape(r.Owner) + "/" + url.PathEscape(r.Name)
}

// PullRequest is the part of a pull request the gate needs.
type PullRequest struct {
	Number         int     `json:"number"`
	MergedAt       *string `json:"merged_at"`
	MergeCommitSHA string  `json:"merge_commit_sha"`
	Head           struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

// MergedPullForCommit returns the merged pull request whose merge commit is
// sha (the commit a push to main carries), or nil.
func (r Repo) MergedPullForCommit(ctx context.Context, sha string) (*PullRequest, error) {
	prs, err := ghapi.List[PullRequest](ctx, r.API, r.base()+"/commits/"+url.PathEscape(sha)+"/pulls")
	if err != nil {
		return nil, err
	}
	for i := range prs {
		if prs[i].MergedAt != nil && prs[i].MergeCommitSHA == sha {
			return &prs[i], nil
		}
	}
	return nil, nil
}

// Comment is an issue or pull request comment.
type Comment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
}

func (c Comment) byActionsBot() bool { return c.User.Login == ActionsBot && c.User.Type == "Bot" }

// FindBotComment returns the newest comment on issue or pull request number
// that the Actions bot wrote and that contains marker, or nil. Comments by
// anyone else are ignored, whatever they contain (ADR-0018).
func (r Repo) FindBotComment(ctx context.Context, number int, marker string) (*Comment, error) {
	comments, err := ghapi.List[Comment](ctx, r.API, fmt.Sprintf("%s/issues/%d/comments", r.base(), number))
	if err != nil {
		return nil, err
	}
	for i := len(comments) - 1; i >= 0; i-- {
		if comments[i].byActionsBot() && strings.Contains(comments[i].Body, marker) {
			return &comments[i], nil
		}
	}
	return nil, nil
}
