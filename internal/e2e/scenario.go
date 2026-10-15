package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"
)

const planMarker = "<!-- idp-plan -->"

// Timeouts bound each kind of wait.
type Timeouts struct {
	Interval  time.Duration // between polls
	Checks    time.Duration // a pull request's idp-gate
	Reconcile time.Duration // a reconcile run, including the approval wait
	Drift     time.Duration // a drift run
}

// DefaultTimeouts suit the real pipelines, where every job builds idp and plans.
var DefaultTimeouts = Timeouts{Interval: 15 * time.Second, Checks: 20 * time.Minute, Reconcile: 40 * time.Minute, Drift: 20 * time.Minute}

// Scenario is one harness run against a claims repo (spec §8.5, ADR-0027).
type Scenario struct {
	F         Forge
	Clock     Clock
	Log       io.Writer
	Org       string
	Run       string            // unique id of this run
	EngineSHA string            // the engine commit under test
	Templates map[string][]byte // caller templates by file name
	Owner     string            // the harness team's maintainer
	Member    string            // the harness team's member; its case changes in one step
	Timeouts  Timeouts
	names     Names
}

// Execute runs every step. When a step fails after the claims may have been
// merged, it still deletes them through the platform and reports both errors.
func (s *Scenario) Execute(ctx context.Context) error {
	names, err := NewNames(s.Run)
	if err != nil {
		return err
	}
	s.names = names
	if err := s.preflight(ctx); err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if err := s.pin(ctx); err != nil {
		return fmt.Errorf("pin: %w", err)
	}
	for _, step := range []struct {
		name string
		run  func(context.Context) error
	}{{"create", s.create}, {"update", s.update}, {"case-only login edit", s.caseOnly}, {"drift", s.drift}} {
		if err := step.run(ctx); err != nil {
			return s.withCleanup(ctx, fmt.Errorf("%s: %w", step.name, err))
		}
	}
	if err := s.deleteClaims(ctx, []string{s.names.ComponentPath(), s.names.GroupPath()}); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if err := s.drift(ctx); err != nil {
		return fmt.Errorf("final drift: %w", err)
	}
	s.logf("e2e: passed")
	return nil
}

func (s *Scenario) preflight(ctx context.Context) error {
	n, err := s.F.OpenIssues(ctx, "idp-wet-push", "Wet push failed")
	if err != nil {
		return err
	}
	if n > 0 {
		return errors.New(`an open "Wet push failed" issue holds every reconcile: recover first`)
	}
	for _, p := range []string{s.repoPath(), s.teamPath()} {
		ok, err := s.F.Exists(ctx, p)
		if err != nil {
			return err
		}
		if ok {
			return fmt.Errorf("%s already exists: a previous run left it behind", p)
		}
	}
	return nil
}

func (s *Scenario) pin(ctx context.Context) error {
	want, err := PinTemplates(s.Templates, s.EngineSHA)
	if err != nil {
		return err
	}
	edits := map[string][]byte{}
	for _, name := range slices.Sorted(maps.Keys(want)) {
		path := ".github/workflows/" + name
		got, ok, err := s.F.File(ctx, "main", path)
		if err != nil {
			return err
		}
		if !ok || !bytes.Equal(got, want[name]) {
			edits[path] = want[name]
		}
	}
	if len(edits) == 0 {
		s.logf("e2e: the callers already pin %s", short(s.EngineSHA))
		return nil
	}
	title := "chore: pin the idp pipelines to " + short(s.EngineSHA)
	pr, err := s.openPR(ctx, "pin", title, edits, nil)
	if err != nil {
		return err
	}
	if _, err := s.waitChecks(ctx, pr); err != nil {
		return err
	}
	sha, err := s.F.Merge(ctx, pr.Number, title)
	if err != nil {
		return err
	}
	_, err = s.waitReconcile(ctx, sha, false)
	return err
}

func (s *Scenario) create(ctx context.Context) error {
	n := s.names
	title := "feat: add the e2e harness claims " + n.Component
	put := map[string][]byte{
		n.GroupPath():     GroupClaim(n.Group, "End-to-end harness team", []Member{{s.Owner, "maintainer"}, {s.Member, "member"}}),
		n.ComponentPath(): ComponentClaim(n.Component, "End-to-end harness repository", n.Group, []string{"e2e", "harness"}),
	}
	if err := s.change(ctx, "create", title, put, nil, false); err != nil {
		return err
	}
	return s.assertResources(ctx, "End-to-end harness repository", []string{"e2e", "harness"})
}

func (s *Scenario) update(ctx context.Context) error {
	n := s.names
	put := map[string][]byte{n.ComponentPath(): ComponentClaim(n.Component, "End-to-end harness repository, updated", n.Group, nil)}
	if err := s.change(ctx, "update", "feat: update "+n.Component+" and remove its topics", put, nil, false); err != nil {
		return err
	}
	return s.assertResources(ctx, "End-to-end harness repository, updated", nil)
}

func (s *Scenario) caseOnly(ctx context.Context) error {
	n := s.names
	put := map[string][]byte{n.GroupPath(): GroupClaim(n.Group, "End-to-end harness team", []Member{{s.Owner, "maintainer"}, {flipCase(s.Member), "member"}})}
	title := "feat: change only the case of a member login"
	pr, err := s.openPR(ctx, "case", title, put, nil)
	if err != nil {
		return err
	}
	comment, err := s.waitChecks(ctx, pr)
	if err != nil {
		return err
	}
	if !strings.Contains(comment, "No infrastructure changes.") {
		return errors.New("a case-only login edit must plan no change (phase 1a F1)")
	}
	sha, err := s.F.Merge(ctx, pr.Number, title)
	if err != nil {
		return err
	}
	jobs, err := s.waitReconcile(ctx, sha, false)
	if err != nil {
		return err
	}
	if jobs["reconcile / approve"] != "skipped" {
		return fmt.Errorf("the gate asked for approval (approve: %s)", jobs["reconcile / approve"])
	}
	return nil
}

func (s *Scenario) drift(ctx context.Context) error {
	head, err := s.F.HeadSHA(ctx, "main")
	if err != nil {
		return err
	}
	before, err := s.F.Runs(ctx, "drift.yaml", head)
	if err != nil {
		return err
	}
	if err := s.F.Dispatch(ctx, "drift.yaml", "main"); err != nil {
		return err
	}
	var run Run
	err = poll(ctx, s.Clock, s.Timeouts.Interval, s.Timeouts.Drift, "the drift run", func() (bool, error) {
		runs, err := s.F.Runs(ctx, "drift.yaml", head)
		if err != nil || len(runs) <= len(before) {
			return false, err
		}
		run = runs[0]
		return run.Status == "completed", nil
	})
	if err != nil {
		return err
	}
	if run.Conclusion != "success" {
		return fmt.Errorf("drift run %d concluded %s", run.ID, run.Conclusion)
	}
	open, err := s.F.OpenIssues(ctx, "drift", "Drift detected")
	if err != nil {
		return err
	}
	if open != 0 {
		return fmt.Errorf("drift run %d left a drift issue open", run.ID)
	}
	s.logf("e2e: drift run %d found no drift", run.ID)
	return nil
}

// deleteClaims deletes the given claim files through a pull request, approves
// the destructive reconcile of that merge only, and checks the resources are
// gone.
func (s *Scenario) deleteClaims(ctx context.Context, paths []string) error {
	if err := s.names.guard(); err != nil {
		return err
	}
	title := "feat: delete the e2e harness claims " + s.names.Component
	pr, err := s.openPR(ctx, "delete", title, nil, paths)
	if err != nil {
		return err
	}
	comment, err := s.waitChecks(ctx, pr)
	if err != nil {
		return err
	}
	if !strings.Contains(comment, "[!WARNING]") {
		return errors.New("the delete plan must be flagged destructive")
	}
	sha, err := s.F.Merge(ctx, pr.Number, title)
	if err != nil {
		return err
	}
	if _, err := s.waitReconcile(ctx, sha, true); err != nil {
		return err
	}
	for _, p := range []string{s.repoPath(), s.teamPath()} {
		ok, err := s.F.Exists(ctx, p)
		if err != nil {
			return err
		}
		if ok {
			return fmt.Errorf("%s still exists after the delete", p)
		}
	}
	s.logf("e2e: %s and %s are gone", s.names.Component, s.names.Group)
	return nil
}

// withCleanup deletes this run's claims through the platform after a failure,
// so a broken run does not leave resources behind; it never touches anything
// else (Review Focus 1).
func (s *Scenario) withCleanup(ctx context.Context, cause error) error {
	s.logf("e2e: %v; cleaning up", cause)
	var present []string
	for _, p := range []string{s.names.ComponentPath(), s.names.GroupPath()} {
		_, ok, err := s.F.File(ctx, "main", p)
		if err != nil {
			return errors.Join(cause, fmt.Errorf("cleanup: %w", err))
		}
		if ok {
			present = append(present, p)
		}
	}
	if len(present) == 0 {
		return cause
	}
	if err := s.deleteClaims(ctx, present); err != nil {
		return errors.Join(cause, fmt.Errorf("cleanup: %w; see docs/e2e.md for the leftovers", err))
	}
	return cause
}

// change opens a pull request with put and del, waits for idp-gate, refuses a
// destructive plan unless destructive is set, merges, and requires the
// reconcile to apply without approval.
func (s *Scenario) change(ctx context.Context, slug, title string, put map[string][]byte, del []string, destructive bool) error {
	pr, err := s.openPR(ctx, slug, title, put, del)
	if err != nil {
		return err
	}
	comment, err := s.waitChecks(ctx, pr)
	if err != nil {
		return err
	}
	if !destructive && strings.Contains(comment, "[!WARNING]") {
		return fmt.Errorf("the %s plan is destructive", slug)
	}
	sha, err := s.F.Merge(ctx, pr.Number, title)
	if err != nil {
		return err
	}
	jobs, err := s.waitReconcile(ctx, sha, false)
	if err != nil {
		return err
	}
	if jobs["reconcile / approve"] != "skipped" {
		return fmt.Errorf("the gate asked for approval (approve: %s)", jobs["reconcile / approve"])
	}
	if jobs["reconcile / apply-github"] != "success" {
		return fmt.Errorf("apply-github: %s", jobs["reconcile / apply-github"])
	}
	return nil
}

func (s *Scenario) openPR(ctx context.Context, slug, title string, put map[string][]byte, del []string) (PR, error) {
	base, err := s.F.HeadSHA(ctx, "main")
	if err != nil {
		return PR{}, err
	}
	branch := "e2e/" + s.Run + "/" + slug
	if err := s.F.CreateBranch(ctx, branch, base); err != nil {
		return PR{}, err
	}
	for _, p := range slices.Sorted(maps.Keys(put)) {
		if err := s.F.PutFile(ctx, branch, p, put[p], title); err != nil {
			return PR{}, err
		}
	}
	for _, p := range del {
		if err := s.F.DeleteFile(ctx, branch, p, title); err != nil {
			return PR{}, err
		}
	}
	pr, err := s.F.OpenPR(ctx, branch, title, "Opened by the idp end-to-end harness, run "+s.Run+".")
	if err != nil {
		return PR{}, err
	}
	s.logf("e2e: opened PR #%d (%s)", pr.Number, title)
	return pr, nil
}

// waitChecks waits for the pull request's idp-gate check, which mirrors the
// whole PR pipeline (ADR-0022), and returns the plan comment for its head.
func (s *Scenario) waitChecks(ctx context.Context, pr PR) (string, error) {
	var gate string
	err := poll(ctx, s.Clock, s.Timeouts.Interval, s.Timeouts.Checks, fmt.Sprintf("idp-gate on PR #%d", pr.Number), func() (bool, error) {
		checks, err := s.F.Checks(ctx, pr.HeadSHA)
		if err != nil {
			return false, err
		}
		gate = checks["idp-gate"]
		return gate != "", nil
	})
	if err != nil {
		return "", err
	}
	if gate != "success" {
		return "", fmt.Errorf("idp-gate on PR #%d concluded %s", pr.Number, gate)
	}
	comment, err := s.F.PlanComment(ctx, pr.Number)
	if err != nil {
		return "", err
	}
	if !strings.Contains(comment, planMarker) || !strings.Contains(comment, "sha="+pr.HeadSHA) {
		return "", fmt.Errorf("PR #%d has no plan comment for its head %s", pr.Number, short(pr.HeadSHA))
	}
	return comment, nil
}

// waitReconcile waits for the reconcile of a merge commit and returns its job
// conclusions. With approve it approves the idp-approval deployment of that run
// and of no other (Review Focus 2); without it, a run that waits for approval
// is an error.
func (s *Scenario) waitReconcile(ctx context.Context, sha string, approve bool) (map[string]string, error) {
	var run Run
	approved := false
	err := poll(ctx, s.Clock, s.Timeouts.Interval, s.Timeouts.Reconcile, "the reconcile of "+short(sha), func() (bool, error) {
		runs, err := s.F.Runs(ctx, "reconcile.yaml", sha)
		if err != nil || len(runs) == 0 {
			return false, err
		}
		run = runs[0]
		if run.Status == "waiting" && !approve {
			return false, fmt.Errorf("reconcile run %d unexpectedly waits for approval", run.ID)
		}
		if run.Status == "waiting" && !approved {
			env, ok, err := s.F.PendingApproval(ctx, run.ID)
			if err != nil {
				return false, err
			}
			if ok {
				if err := s.F.Approve(ctx, run.ID, env, "idp end-to-end harness run "+s.Run+": delete "+s.names.Component+" and "+s.names.Group); err != nil {
					return false, err
				}
				approved = true
				s.logf("e2e: approved reconcile run %d", run.ID)
			}
		}
		return run.Status == "completed", nil
	})
	if err != nil {
		return nil, err
	}
	if run.Conclusion != "success" {
		return nil, fmt.Errorf("reconcile run %d concluded %s", run.ID, run.Conclusion)
	}
	if approve && !approved {
		return nil, fmt.Errorf("reconcile run %d finished without asking for approval", run.ID)
	}
	return s.F.Jobs(ctx, run.ID)
}

func (s *Scenario) assertResources(ctx context.Context, description string, topics []string) error {
	var repo struct {
		Visibility  string   `json:"visibility"`
		Description string   `json:"description"`
		Topics      []string `json:"topics"`
		Archived    bool     `json:"archived"`
	}
	if err := s.F.GetJSON(ctx, s.repoPath(), &repo); err != nil {
		return err
	}
	if repo.Visibility != "public" || repo.Description != description || repo.Archived {
		return fmt.Errorf("repository %s = %+v, want public, unarchived, description %q", s.names.Component, repo, description)
	}
	if !sameSet(repo.Topics, topics) {
		return fmt.Errorf("repository %s topics = %v, want %v", s.names.Component, repo.Topics, topics)
	}
	var team struct {
		Privacy string `json:"privacy"`
	}
	if err := s.F.GetJSON(ctx, s.teamPath(), &team); err != nil {
		return err
	}
	if team.Privacy != "closed" {
		return fmt.Errorf("team %s privacy = %q, want closed", s.names.Group, team.Privacy)
	}
	for _, m := range []Member{{strings.ToLower(s.Owner), "maintainer"}, {strings.ToLower(s.Member), "member"}} {
		var got struct {
			Role string `json:"role"`
		}
		if err := s.F.GetJSON(ctx, s.teamPath()+"/memberships/"+m.User, &got); err != nil {
			return err
		}
		if got.Role != m.Role {
			return fmt.Errorf("membership of %s in %s = %q, want %q", m.User, s.names.Group, got.Role, m.Role)
		}
	}
	var envs struct {
		Environments []struct {
			Name string `json:"name"`
		} `json:"environments"`
	}
	if err := s.F.GetJSON(ctx, s.repoPath()+"/environments", &envs); err != nil {
		return err
	}
	if len(envs.Environments) != 1 || envs.Environments[0].Name != "dev" {
		return fmt.Errorf("environments of %s = %+v, want [dev]", s.names.Component, envs.Environments)
	}
	s.logf("e2e: %s and %s match the claims", s.names.Component, s.names.Group)
	return nil
}

func (s *Scenario) repoPath() string { return "/repos/" + s.Org + "/" + s.names.Component }
func (s *Scenario) teamPath() string { return "/orgs/" + s.Org + "/teams/" + s.names.Group }

func (s *Scenario) logf(format string, a ...any) { fmt.Fprintf(s.Log, format+"\n", a...) }

// flipCase upper-cases the first letter of each hyphen-separated part, so the
// result differs from login only in case.
func flipCase(login string) string {
	parts := strings.Split(login, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	if out := strings.Join(parts, "-"); out != login {
		return out
	}
	return strings.ToUpper(login)
}

func sameSet(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
