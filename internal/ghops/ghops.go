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
	ID     int64  `json:"id"`
	NodeID string `json:"node_id"`
	Body   string `json:"body"`
	User   struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
}

func (c Comment) byActionsBot() bool { return c.User.Login == ActionsBot && c.User.Type == "Bot" }

// actionsBotGraphQL is the Actions bot's login in GraphQL, which has no "[bot]" suffix.
const actionsBotGraphQL = "github-actions"

// EditedByOthers reports whether anyone but the Actions bot made the last edit
// of a comment, and who (ADR-0018). An unedited comment, and one the bot itself
// updated, are not edited by others.
func (r Repo) EditedByOthers(ctx context.Context, nodeID string) (bool, string, error) {
	const q = `query($id: ID!) { node(id: $id) { ... on IssueComment { editor { login __typename } } } }`
	var out struct {
		Node *struct {
			Editor *struct {
				Login    string `json:"login"`
				Typename string `json:"__typename"`
			} `json:"editor"`
		} `json:"node"`
	}
	if err := r.API.GraphQL(ctx, q, map[string]any{"id": nodeID}, &out); err != nil {
		return false, "", fmt.Errorf("read the last editor of comment %s: %w", nodeID, err)
	}
	if out.Node == nil {
		return false, "", fmt.Errorf("comment %s not found", nodeID)
	}
	e := out.Node.Editor
	if e == nil || (e.Typename == "Bot" && e.Login == actionsBotGraphQL) {
		return false, "", nil
	}
	return true, e.Login, nil
}

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
