package e2e

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
	"github.com/jellalshadows-idp/idp-engine/internal/ghops"
)

// GitHub implements Forge over the REST API with the two tokens of ADR-0027:
// Bot for everything except approvals, Approver for idp-approval only.
type GitHub struct {
	Bot      *ghapi.Client
	Approver *ghapi.Client
	Owner    string
	Repo     string
}

var _ Forge = GitHub{}

// retryDelays space the retries of a read that hit a transient error.
var retryDelays = []time.Duration{2 * time.Second, 5 * time.Second}

// read runs a GET, retrying it after a 5xx or a transport error: the harness
// polls for up to an hour, and one transient error must not fail a nightly
// run. Writes are never retried.
func (g GitHub) read(ctx context.Context, get func() error) error {
	err := get()
	for _, d := range retryDelays {
		if !transient(ctx, err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
		err = get()
	}
	return err
}

func transient(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	var apiErr *ghapi.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status >= 500
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

func (g GitHub) path(p string) string { return "/repos/" + g.Owner + "/" + g.Repo + p }

func (g GitHub) HeadSHA(ctx context.Context, branch string) (string, error) {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := g.read(ctx, func() error { return g.Bot.Get(ctx, g.path("/git/ref/heads/"+branch), &ref) }); err != nil {
		return "", err
	}
	return ref.Object.SHA, nil
}

func (g GitHub) CreateBranch(ctx context.Context, branch, fromSHA string) error {
	return g.Bot.Post(ctx, g.path("/git/refs"), map[string]string{"ref": "refs/heads/" + branch, "sha": fromSHA}, nil)
}

type contentsFile struct {
	SHA      string `json:"sha"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func (g GitHub) contents(ctx context.Context, ref, path string) (*contentsFile, error) {
	var f contentsFile
	err := g.read(ctx, func() error {
		return g.Bot.Get(ctx, g.path("/contents/"+path+"?ref="+url.QueryEscape(ref)), &f)
	})
	if errors.Is(err, ghapi.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (g GitHub) File(ctx context.Context, ref, path string) ([]byte, bool, error) {
	f, err := g.contents(ctx, ref, path)
	if err != nil || f == nil {
		return nil, false, err
	}
	if f.Encoding != "base64" {
		return nil, false, fmt.Errorf("%s: unexpected encoding %q", path, f.Encoding)
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(f.Content, "\n", ""))
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	return b, true, nil
}

func (g GitHub) PutFile(ctx context.Context, branch, path string, content []byte, message string) error {
	f, err := g.contents(ctx, branch, path)
	if err != nil {
		return err
	}
	body := map[string]any{"message": message, "content": base64.StdEncoding.EncodeToString(content), "branch": branch}
	if f != nil {
		body["sha"] = f.SHA
	}
	return g.Bot.Put(ctx, g.path("/contents/"+path), body, nil)
}

func (g GitHub) DeleteFile(ctx context.Context, branch, path, message string) error {
	f, err := g.contents(ctx, branch, path)
	if err != nil {
		return err
	}
	if f == nil {
		return fmt.Errorf("%s does not exist on %s", path, branch)
	}
	return g.Bot.Do(ctx, http.MethodDelete, g.path("/contents/"+path), map[string]any{"message": message, "sha": f.SHA, "branch": branch}, nil)
}

func (g GitHub) OpenPR(ctx context.Context, head, title, body string) (PR, error) {
	var pr struct {
		Number int `json:"number"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := g.Bot.Post(ctx, g.path("/pulls"), map[string]string{"title": title, "head": head, "base": "main", "body": body}, &pr); err != nil {
		return PR{}, err
	}
	return PR{Number: pr.Number, HeadSHA: pr.Head.SHA}, nil
}

func (g GitHub) Checks(ctx context.Context, sha string) (map[string]string, error) {
	var resp struct {
		CheckRuns []struct {
			ID         int64   `json:"id"`
			Name       string  `json:"name"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := g.read(ctx, func() error {
		return g.Bot.Get(ctx, g.path("/commits/"+sha+"/check-runs?per_page=100"), &resp)
	}); err != nil {
		return nil, err
	}
	newest := map[string]int64{}
	out := map[string]string{}
	for _, c := range resp.CheckRuns {
		if c.ID < newest[c.Name] {
			continue
		}
		newest[c.Name] = c.ID
		out[c.Name] = ""
		if c.Status == "completed" && c.Conclusion != nil {
			out[c.Name] = *c.Conclusion
		}
	}
	return out, nil
}

// PlanComment returns the bot's plan comment, and refuses one that anyone but
// the bot edited, as the gate does (ADR-0018): the harness approves on it.
func (g GitHub) PlanComment(ctx context.Context, pr int) (string, error) {
	repo := ghops.Repo{API: g.Bot, Owner: g.Owner, Name: g.Repo}
	var c *ghops.Comment
	err := g.read(ctx, func() (err error) {
		c, err = repo.FindBotComment(ctx, pr, planMarker)
		return err
	})
	if err != nil || c == nil {
		return "", err
	}
	var edited bool
	var editor string
	if err := g.read(ctx, func() (err error) {
		edited, editor, err = repo.EditedByOthers(ctx, c.NodeID)
		return err
	}); err != nil {
		return "", err
	}
	if edited {
		return "", fmt.Errorf("the plan comment on PR #%d was edited by %s: the harness trusts only the bot's own comment", pr, editor)
	}
	return c.Body, nil
}

// Merge merges pr with a merge commit, pinned to the head the harness
// verified: GitHub answers 409 when the head moved.
func (g GitHub) Merge(ctx context.Context, pr PR, title string) (string, error) {
	var resp struct {
		SHA    string `json:"sha"`
		Merged bool   `json:"merged"`
	}
	body := map[string]string{"merge_method": "merge", "commit_title": title, "sha": pr.HeadSHA}
	if err := g.Bot.Put(ctx, g.path(fmt.Sprintf("/pulls/%d/merge", pr.Number)), body, &resp); err != nil {
		return "", err
	}
	if !resp.Merged {
		return "", fmt.Errorf("PR #%d was not merged", pr.Number)
	}
	return resp.SHA, nil
}

// Parent returns the first parent of a commit: for a merge commit, the head
// the base branch had when it landed.
func (g GitHub) Parent(ctx context.Context, sha string) (string, error) {
	var commit struct {
		Parents []struct {
			SHA string `json:"sha"`
		} `json:"parents"`
	}
	if err := g.read(ctx, func() error { return g.Bot.Get(ctx, g.path("/commits/"+sha), &commit) }); err != nil {
		return "", err
	}
	if len(commit.Parents) == 0 {
		return "", fmt.Errorf("commit %s has no parent", short(sha))
	}
	return commit.Parents[0].SHA, nil
}

// CheckApprover proves the approver token still reads the claims repo's runs,
// so an expired token stops a run before it creates anything.
func (g GitHub) CheckApprover(ctx context.Context) error {
	if err := g.read(ctx, func() error { return g.Approver.Get(ctx, g.path("/actions/runs?per_page=1"), nil) }); err != nil {
		return fmt.Errorf("the approver token cannot read the claims repo's runs (expired or missing Actions read?): rotate it, see docs/e2e.md: %w", err)
	}
	return nil
}

// DeleteBranch deletes a branch with the bot token; a 404 means it is already
// gone, which is success.
func (g GitHub) DeleteBranch(ctx context.Context, branch string) error {
	err := g.Bot.Delete(ctx, g.path("/git/refs/heads/"+branch))
	if errors.Is(err, ghapi.ErrNotFound) {
		return nil
	}
	return err
}

// ClosePR closes a pull request with the bot token.
func (g GitHub) ClosePR(ctx context.Context, number int) error {
	return g.Bot.Patch(ctx, g.path(fmt.Sprintf("/pulls/%d", number)), map[string]string{"state": "closed"}, nil)
}

func (g GitHub) Runs(ctx context.Context, workflow, sha string) ([]Run, error) {
	var resp struct {
		WorkflowRuns []struct {
			ID         int64   `json:"id"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
		} `json:"workflow_runs"`
	}
	if err := g.read(ctx, func() error {
		return g.Bot.Get(ctx, g.path("/actions/workflows/"+workflow+"/runs?head_sha="+url.QueryEscape(sha)+"&per_page=20"), &resp)
	}); err != nil {
		return nil, err
	}
	runs := make([]Run, 0, len(resp.WorkflowRuns))
	for _, r := range resp.WorkflowRuns {
		run := Run{ID: r.ID, Status: r.Status}
		if r.Conclusion != nil {
			run.Conclusion = *r.Conclusion
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (g GitHub) Jobs(ctx context.Context, runID int64) (map[string]string, error) {
	var resp struct {
		Jobs []struct {
			Name       string  `json:"name"`
			Conclusion *string `json:"conclusion"`
		} `json:"jobs"`
	}
	if err := g.read(ctx, func() error {
		return g.Bot.Get(ctx, g.path(fmt.Sprintf("/actions/runs/%d/jobs?per_page=100", runID)), &resp)
	}); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, j := range resp.Jobs {
		out[j.Name] = ""
		if j.Conclusion != nil {
			out[j.Name] = *j.Conclusion
		}
	}
	return out, nil
}

func (g GitHub) Dispatch(ctx context.Context, workflow, ref string) error {
	return g.Bot.Post(ctx, g.path("/actions/workflows/"+workflow+"/dispatches"), map[string]string{"ref": ref}, nil)
}

func (g GitHub) PendingApproval(ctx context.Context, runID int64) (int64, bool, error) {
	var pending []struct {
		Environment struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"environment"`
		CurrentUserCanApprove bool `json:"current_user_can_approve"`
	}
	if err := g.read(ctx, func() error {
		return g.Approver.Get(ctx, g.path(fmt.Sprintf("/actions/runs/%d/pending_deployments", runID)), &pending)
	}); err != nil {
		return 0, false, err
	}
	for _, p := range pending {
		if p.Environment.Name == "idp-approval" && p.CurrentUserCanApprove {
			return p.Environment.ID, true, nil
		}
	}
	return 0, false, nil
}

func (g GitHub) Approve(ctx context.Context, runID, envID int64, comment string) error {
	body := map[string]any{"environment_ids": []int64{envID}, "state": "approved", "comment": comment}
	return g.Approver.Post(ctx, g.path(fmt.Sprintf("/actions/runs/%d/pending_deployments", runID)), body, nil)
}

// Reject rejects a pending idp-approval deployment with the approver token, so
// nothing is applied and the idp-wet queue frees (ADR-0027).
func (g GitHub) Reject(ctx context.Context, runID, envID int64, comment string) error {
	body := map[string]any{"environment_ids": []int64{envID}, "state": "rejected", "comment": comment}
	return g.Approver.Post(ctx, g.path(fmt.Sprintf("/actions/runs/%d/pending_deployments", runID)), body, nil)
}

func (g GitHub) Exists(ctx context.Context, path string) (bool, error) {
	err := g.read(ctx, func() error { return g.Bot.Get(ctx, path, nil) })
	if errors.Is(err, ghapi.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (g GitHub) GetJSON(ctx context.Context, path string, out any) error {
	return g.read(ctx, func() error { return g.Bot.Get(ctx, path, out) })
}

func (g GitHub) OpenIssues(ctx context.Context, label, title string) (int, error) {
	type issue struct {
		Title       string    `json:"title"`
		PullRequest *struct{} `json:"pull_request"`
		User        struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	var issues []issue
	err := g.read(ctx, func() (err error) {
		issues, err = ghapi.List[issue](ctx, g.Bot, g.path("/issues?state=open&labels="+url.QueryEscape(label)))
		return err
	})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, i := range issues {
		if i.PullRequest == nil && i.Title == title && i.User.Login == ghops.ActionsBot {
			n++
		}
	}
	return n, nil
}

// ClaimFiles lists the claim files on main.
func (g GitHub) ClaimFiles(ctx context.Context) ([]string, error) {
	var out []string
	for _, dir := range []string{"claims/groups", "claims/components"} {
		var entries []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		}
		err := g.read(ctx, func() error { return g.Bot.Get(ctx, g.path("/contents/"+dir+"?ref=main"), &entries) })
		if errors.Is(err, ghapi.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Type == "file" && strings.HasSuffix(e.Path, ".yaml") {
				out = append(out, e.Path)
			}
		}
	}
	return out, nil
}

// OrgResources lists the org's repository names and team slugs that start
// with prefix. The harness's repositories are public, so the bot token sees
// them although it is scoped to the claims repo; teams need its members:read.
func (g GitHub) OrgResources(ctx context.Context, prefix string) ([]string, error) {
	type named struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	var out []string
	for _, p := range []string{"/orgs/" + g.Owner + "/repos", "/orgs/" + g.Owner + "/teams"} {
		var items []named
		err := g.read(ctx, func() (err error) {
			items, err = ghapi.List[named](ctx, g.Bot, p)
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			name := it.Name
			if it.Slug != "" {
				name = it.Slug
			}
			if strings.HasPrefix(name, prefix) {
				out = append(out, name)
			}
		}
	}
	return out, nil
}

// ActiveRuns lists the claims repo's reconcile, drift and recover runs that
// have not completed; they share the idp-wet queue (spec §6).
func (g GitHub) ActiveRuns(ctx context.Context) ([]Run, error) {
	var out []Run
	for _, wf := range []string{"reconcile.yaml", "drift.yaml", "recover.yaml"} {
		var resp struct {
			WorkflowRuns []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"workflow_runs"`
		}
		err := g.read(ctx, func() error { return g.Bot.Get(ctx, g.path("/actions/workflows/"+wf+"/runs?per_page=100"), &resp) })
		if errors.Is(err, ghapi.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, r := range resp.WorkflowRuns {
			if r.Status != "completed" {
				out = append(out, Run{ID: r.ID, Status: r.Status})
			}
		}
	}
	return out, nil
}
