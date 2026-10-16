package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

const (
	planMarker = plan.CommentMarker
	noChanges  = "No infrastructure changes."
	warning    = "[!WARNING]"
)

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)

// Timeouts bound each kind of wait.
type Timeouts struct {
	Interval  time.Duration // between polls
	Checks    time.Duration // a pull request's idp-gate
	Reconcile time.Duration // a reconcile run, including the approval wait
	Drift     time.Duration // a drift run
}

// DefaultTimeouts suit the real pipelines, where every job builds idp and plans.
// They fit under the e2e job's timeout-minutes (330), so GitHub never kills a
// run before its own bounded waits end and its cleanup runs. Worst case, seven
// waited steps and a cleanup: pin, create, update, case-only and delete each
// wait for a gate and a reconcile (15 + 30 = 45 minutes), the two drift checks
// for a drift run (15 each), and the cleanup for a gate and a reconcile (45):
// 5×45 + 2×15 + 45 = 300 minutes, leaving 30 for setup, the build and polling.
var DefaultTimeouts = Timeouts{Interval: 15 * time.Second, Checks: 15 * time.Minute, Reconcile: 30 * time.Minute, Drift: 15 * time.Minute}

// approval says how a step treats a reconcile that waits on idp-approval.
type approval int

const (
	// rejectApproval: the step expects no approval; a wait is rejected (nothing
	// is applied, the idp-wet queue frees) and the step fails.
	rejectApproval approval = iota
	// requireApproval: the run must ask for approval, and only it is approved.
	requireApproval
	// allowApproval: approve the run if it asks (cleanup).
	allowApproval
)

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
	lastMerge string         // the merge commit of the harness's last merge
	open      map[int]string // PRs this run opened and has not merged → their branches
}

// Execute runs every step. When a step fails after the claims may have been
// merged, it still deletes them through the platform and reports both errors.
// After any failure it closes the pull requests it opened and did not merge.
func (s *Scenario) Execute(ctx context.Context) error {
	err := s.execute(ctx)
	if err != nil {
		s.closeOpenPRs(ctx)
	}
	return err
}

func (s *Scenario) execute(ctx context.Context) error {
	if err := s.validate(); err != nil {
		return err
	}
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
	if err := s.deleteClaims(ctx, []string{s.names.ComponentPath(), s.names.GroupPath()}, false); err != nil {
		return fmt.Errorf("delete: %w; see docs/e2e.md for the leftovers", err)
	}
	if err := s.drift(ctx); err != nil {
		return fmt.Errorf("final drift: %w", err)
	}
	s.logf("e2e: passed")
	return nil
}

func (s *Scenario) validate() error {
	switch {
	case s.F == nil || s.Clock == nil || s.Log == nil:
		return errors.New("forge, clock and log are required")
	case len(s.Templates) == 0:
		return errors.New("templates are required")
	case s.Org == "":
		return errors.New("org is required")
	case !loginPattern.MatchString(s.Owner) || !loginPattern.MatchString(s.Member):
		return fmt.Errorf("owner %q and member %q must be GitHub logins", s.Owner, s.Member)
	case !strings.ContainsAny(strings.ToLower(s.Member), "abcdefghijklmnopqrstuvwxyz"):
		return fmt.Errorf("member %q needs a letter, so the case-only step can change it", s.Member)
	case s.Timeouts.Interval <= 0:
		return errors.New("the poll interval must be positive")
	}
	return nil
}

// preflight refuses a run that could not finish cleanly: an approver token
// that no longer works, an unrecovered wet push, pipeline runs still going, or
// anything an earlier run left behind.
func (s *Scenario) preflight(ctx context.Context) error {
	if err := s.F.CheckApprover(ctx); err != nil {
		return err
	}
	n, err := s.F.OpenIssues(ctx, "idp-wet-push", "Wet push failed")
	if err != nil {
		return err
	}
	if n > 0 {
		return errors.New(`an open "Wet push failed" issue holds every reconcile: recover first`)
	}
	active, err := s.F.ActiveRuns(ctx)
	if err != nil {
		return err
	}
	if len(active) > 0 {
		return fmt.Errorf("%d pipeline run(s) of the claims repo are still running or waiting (first: %d); let them finish or resolve their approval first", len(active), active[0].ID)
	}
	files, err := s.F.ClaimFiles(ctx)
	if err != nil {
		return err
	}
	for _, f := range files {
		if leftover.MatchString(strings.TrimSuffix(path.Base(f), ".yaml")) {
			return fmt.Errorf("%s is on main: an earlier run left its claims behind; delete them through a PR (docs/e2e.md)", f)
		}
	}
	left, err := s.F.OrgResources(ctx, prefix+"r")
	if err != nil {
		return err
	}
	var mine []string
	for _, name := range left {
		if leftover.MatchString(name) {
			mine = append(mine, name)
		}
	}
	if len(mine) > 0 {
		return fmt.Errorf("%v exist: an earlier run left them behind (docs/e2e.md)", mine)
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
		p := ".github/workflows/" + name
		got, ok, err := s.F.File(ctx, "main", p)
		if err != nil {
			return err
		}
		if !ok || !bytes.Equal(got, want[name]) {
			edits[p] = want[name]
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
	comment, err := s.waitChecks(ctx, pr)
	if err != nil {
		return err
	}
	if !strings.Contains(comment, noChanges) {
		return errors.New("the pin PR must plan no infrastructure change: claims the platform has not applied are on main (docs/e2e.md)")
	}
	sha, err := s.merge(ctx, pr, title)
	if err != nil {
		return err
	}
	_, err = s.waitReconcile(ctx, sha, rejectApproval, "")
	return err
}

func (s *Scenario) create(ctx context.Context) error {
	n := s.names
	put := map[string][]byte{
		n.GroupPath():     GroupClaim(n.Group, "End-to-end harness team", []Member{{s.Owner, "maintainer"}, {s.Member, "member"}}),
		n.ComponentPath(): ComponentClaim(n.Component, "End-to-end harness repository", n.Group, []string{"e2e", "harness"}),
	}
	if err := s.change(ctx, "create", "feat: add the e2e harness claims "+n.Component, put); err != nil {
		return err
	}
	return s.assertResources(ctx, "End-to-end harness repository", []string{"e2e", "harness"})
}

func (s *Scenario) update(ctx context.Context) error {
	n := s.names
	put := map[string][]byte{n.ComponentPath(): ComponentClaim(n.Component, "End-to-end harness repository, updated", n.Group, nil)}
	if err := s.removeTopics(ctx, "feat: update "+n.Component+" and remove its topics", put); err != nil {
		return err
	}
	return s.assertResources(ctx, "End-to-end harness repository, updated", nil)
}

// removeTopics opens a pull request that removes the component's last topics.
// The provider clears topics only when their resource is destroyed, so its plan
// must delete exactly that resource and the reconcile waits for approval. Like
// deleteClaims, it starts only from the main the harness's last merge left,
// approves only the reconcile of its own merge, and approves nothing when main
// moved while the PR was open.
func (s *Scenario) removeTopics(ctx context.Context, title string, put map[string][]byte) error {
	base, err := s.baseOfLastMerge(ctx)
	if err != nil {
		return err
	}
	pr, err := s.openPRFrom(ctx, base, "update", title, put, nil)
	if err != nil {
		return err
	}
	comment, err := s.waitChecks(ctx, pr)
	if err != nil {
		return err
	}
	fp, err := fingerprintOf(comment, pr)
	if err != nil {
		return err
	}
	want := []string{"github: delete module.component_" + s.names.Component + ".github_repository_topics.this[0]"}
	if got := fp.Destructive(); !strings.Contains(comment, warning) || !slices.Equal(got, want) {
		return fmt.Errorf("the update plan must destroy exactly %v, got %v", want, got)
	}
	sha, err := s.merge(ctx, pr, title)
	if err != nil {
		return err
	}
	if err := s.requireParentIsBase(ctx, sha, pr); err != nil {
		return err
	}
	jobs, err := s.waitReconcile(ctx, sha, requireApproval, "remove the topics of "+s.names.Component)
	if err != nil {
		return err
	}
	if jobs["reconcile / apply-github"] != "success" {
		return fmt.Errorf("apply-github: %s", jobs["reconcile / apply-github"])
	}
	return nil
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
	if !strings.Contains(comment, noChanges) {
		return errors.New("a case-only login edit must plan no change (phase 1a F1)")
	}
	sha, err := s.merge(ctx, pr, title)
	if err != nil {
		return err
	}
	_, err = s.waitReconcile(ctx, sha, rejectApproval, "")
	return err
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
	seen := map[int64]bool{}
	for _, r := range before {
		seen[r.ID] = true
	}
	if err := s.F.Dispatch(ctx, "drift.yaml", "main"); err != nil {
		return err
	}
	var run Run
	err = poll(ctx, s.Clock, s.Timeouts.Interval, s.Timeouts.Drift, "the drift run", func() (bool, error) {
		runs, err := s.F.Runs(ctx, "drift.yaml", head)
		if err != nil {
			return false, err
		}
		for _, r := range runs { // newest first: the first unseen run is the dispatched one
			if !seen[r.ID] {
				run = r
				return r.Status == "completed", nil
			}
		}
		return false, nil
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

// deleteClaims deletes claim files through a pull request and checks the
// resources are gone. The main path (cleanup false) starts only from the main
// the harness's last merge left, requires a destructive plan and an approval
// request, and approves only that run; the cleanup path accepts claims that
// were never applied and approves only if its run asks. Both refuse a plan that
// would destroy anything but this run's names, and approve nothing when main
// moved while the PR was open.
func (s *Scenario) deleteClaims(ctx context.Context, paths []string, cleanup bool) error {
	if err := s.names.guard(); err != nil {
		return err
	}
	var base string
	var err error
	if cleanup {
		base, err = s.F.HeadSHA(ctx, "main")
	} else {
		base, err = s.baseOfLastMerge(ctx)
	}
	if err != nil {
		return err
	}
	title := "feat: delete the e2e harness claims " + s.names.Component
	pr, err := s.openPRFrom(ctx, base, "delete", title, nil, paths)
	if err != nil {
		return err
	}
	comment, err := s.waitChecks(ctx, pr)
	if err != nil {
		return err
	}
	if !cleanup && !strings.Contains(comment, warning) {
		return errors.New("the delete plan must be flagged destructive")
	}
	if err := s.destroysOnlyOwnNames(comment, pr); err != nil {
		return err
	}
	sha, err := s.merge(ctx, pr, title)
	if err != nil {
		return err
	}
	if err := s.requireParentIsBase(ctx, sha, pr); err != nil {
		return err
	}
	mode := requireApproval
	if cleanup {
		mode = allowApproval
	}
	if _, err := s.waitReconcile(ctx, sha, mode, "delete "+s.names.Component+" and "+s.names.Group); err != nil {
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

// baseOfLastMerge returns the main head a destructive step may start from: the
// merge the harness made last. The last merge reconciled and a drift run then
// found nothing to change, so a destructive plan from that main can touch only
// this run's resources.
func (s *Scenario) baseOfLastMerge(ctx context.Context) (string, error) {
	base, err := s.F.HeadSHA(ctx, "main")
	if err != nil {
		return "", err
	}
	if base != s.lastMerge {
		return "", fmt.Errorf("main moved since the harness's last merge (now %s, expected %s): someone changed the claims repo during the run; see docs/e2e.md", short(base), short(s.lastMerge))
	}
	return base, nil
}

// requireParentIsBase checks the merge commit sits directly on the main the
// plan was made from. If not, someone else's commit reconciles with this merge:
// its plan is not the one the harness checked, so a wait for approval is
// rejected and the step fails.
func (s *Scenario) requireParentIsBase(ctx context.Context, sha string, pr PR) error {
	parent, err := s.F.Parent(ctx, sha)
	if err != nil {
		return err
	}
	if parent == pr.BaseSHA {
		return nil
	}
	if _, err := s.waitReconcile(ctx, sha, rejectApproval, ""); err != nil {
		s.logf("e2e: the reconcile of %s: %v", short(sha), err)
	}
	return fmt.Errorf("main moved while PR #%d was open: nothing was approved", pr.Number)
}

// fingerprintOf returns the fingerprint the plan comment carries, which must
// have been planned for the PR's head.
func fingerprintOf(comment string, pr PR) (plan.Fingerprint, error) {
	head, fp, ok, err := plan.DecodeMarker(comment)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("the plan comment carries no fingerprint")
	}
	if head != pr.HeadSHA {
		return nil, fmt.Errorf("the fingerprint on PR #%d was planned for %s, not its head %s", pr.Number, short(head), short(pr.HeadSHA))
	}
	return fp, nil
}

// destroysOnlyOwnNames checks the plan comment's fingerprint was planned for
// the PR's head, and that every delete or replace in it (complete even when
// the rendered rows are truncated) belongs to one of this run's modules.
func (s *Scenario) destroysOnlyOwnNames(comment string, pr PR) error {
	fp, err := fingerprintOf(comment, pr)
	if err != nil {
		return err
	}
	own := []string{"module.group_" + s.names.Group + ".", "module.component_" + s.names.Component + "."}
	for _, entry := range fp.Destructive() {
		_, address, _ := strings.Cut(entry, " ") // "stack: action address"
		_, address, _ = strings.Cut(address, " ")
		if !strings.HasPrefix(address, own[0]) && !strings.HasPrefix(address, own[1]) {
			return fmt.Errorf("the delete would destroy something the harness does not own: %s", entry)
		}
	}
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
	if err := s.deleteClaims(ctx, present, true); err != nil {
		return errors.Join(cause, fmt.Errorf("cleanup: %w; see docs/e2e.md for the leftovers", err))
	}
	return cause
}

// change opens a pull request with put, waits for idp-gate, refuses a
// destructive plan, merges, and requires the reconcile to apply without
// approval.
func (s *Scenario) change(ctx context.Context, slug, title string, put map[string][]byte) error {
	pr, err := s.openPR(ctx, slug, title, put, nil)
	if err != nil {
		return err
	}
	comment, err := s.waitChecks(ctx, pr)
	if err != nil {
		return err
	}
	if strings.Contains(comment, warning) {
		return fmt.Errorf("the %s plan is destructive", slug)
	}
	sha, err := s.merge(ctx, pr, title)
	if err != nil {
		return err
	}
	jobs, err := s.waitReconcile(ctx, sha, rejectApproval, "")
	if err != nil {
		return err
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
	return s.openPRFrom(ctx, base, slug, title, put, del)
}

// openPRFrom opens a pull request from a branch created at base, the main head
// the harness read, and records it as open.
func (s *Scenario) openPRFrom(ctx context.Context, base, slug, title string, put map[string][]byte, del []string) (PR, error) {
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
	pr.BaseSHA = base
	if s.open == nil {
		s.open = map[int]string{}
	}
	s.open[pr.Number] = branch
	s.logf("e2e: opened PR #%d (%s)", pr.Number, title)
	return pr, nil
}

// merge merges pr pinned to its verified head, records the merge commit, and
// deletes the PR's branch. A branch it cannot delete is only logged.
func (s *Scenario) merge(ctx context.Context, pr PR, title string) (string, error) {
	sha, err := s.F.Merge(ctx, pr, title)
	if err != nil {
		return "", err
	}
	s.lastMerge = sha
	branch := s.open[pr.Number]
	delete(s.open, pr.Number)
	if err := s.F.DeleteBranch(ctx, branch); err != nil {
		s.logf("e2e: could not delete branch %s: %v", branch, err)
	}
	return sha, nil
}

// closeOpenPRs closes, best effort, the pull requests this run opened and did
// not merge; their branches stay as evidence.
func (s *Scenario) closeOpenPRs(ctx context.Context) {
	for _, n := range slices.Sorted(maps.Keys(s.open)) {
		if err := s.F.ClosePR(ctx, n); err != nil {
			s.logf("e2e: could not close PR #%d: %v", n, err)
			continue
		}
		delete(s.open, n)
		s.logf("e2e: closed PR #%d", n)
	}
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
	head, _, ok, err := plan.DecodeMarker(comment)
	if err != nil {
		return "", fmt.Errorf("the plan comment on PR #%d: %w", pr.Number, err)
	}
	if !strings.Contains(comment, planMarker) || !ok || head != pr.HeadSHA {
		return "", fmt.Errorf("PR #%d has no plan comment for its head %s", pr.Number, short(pr.HeadSHA))
	}
	return comment, nil
}

// waitReconcile waits for the reconcile of a merge commit and returns its job
// conclusions. mode says what to do when it waits on idp-approval (Review
// Focus 2): reject it, require and approve it, or approve it if it asks; approves
// completes the approval comment. Only the run of this merge commit is ever
// approved or rejected.
func (s *Scenario) waitReconcile(ctx context.Context, sha string, mode approval, approves string) (map[string]string, error) {
	var run Run
	approved := false
	err := poll(ctx, s.Clock, s.Timeouts.Interval, s.Timeouts.Reconcile, "the reconcile of "+short(sha), func() (bool, error) {
		runs, err := s.F.Runs(ctx, "reconcile.yaml", sha)
		if err != nil || len(runs) == 0 {
			return false, err
		}
		run = runs[0]
		if run.Status == "waiting" && !approved {
			env, ok, err := s.F.PendingApproval(ctx, run.ID)
			if err != nil || !ok {
				return false, err
			}
			if mode == rejectApproval {
				if err := s.F.Reject(ctx, run.ID, env, "idp end-to-end harness run "+s.Run+": this change was not expected to need approval; rejected, nothing is applied"); err != nil {
					return false, err
				}
				return false, fmt.Errorf("reconcile run %d unexpectedly waits for approval; the harness rejected it", run.ID)
			}
			if err := s.F.Approve(ctx, run.ID, env, "idp end-to-end harness run "+s.Run+": "+approves); err != nil {
				return false, err
			}
			approved = true
			s.logf("e2e: approved reconcile run %d", run.ID)
		}
		return run.Status == "completed", nil
	})
	if err != nil {
		return nil, err
	}
	if run.Conclusion != "success" {
		return nil, fmt.Errorf("reconcile run %d concluded %s", run.ID, run.Conclusion)
	}
	if mode == requireApproval && !approved {
		return nil, fmt.Errorf("reconcile run %d finished without asking for approval", run.ID)
	}
	jobs, err := s.F.Jobs(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	if mode == rejectApproval && jobs["reconcile / approve"] != "skipped" {
		return nil, fmt.Errorf("the gate asked for approval (approve: %s)", jobs["reconcile / approve"])
	}
	return jobs, nil
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
	type ruleset struct {
		Name        string `json:"name"`
		Enforcement string `json:"enforcement"`
	}
	var rulesets []ruleset
	if err := s.F.GetJSON(ctx, s.repoPath()+"/rulesets", &rulesets); err != nil {
		return err
	}
	if !slices.ContainsFunc(rulesets, func(r ruleset) bool { return r.Name == "idp-default-branch" && r.Enforcement == "active" }) {
		return fmt.Errorf("repository %s has no active idp-default-branch ruleset (rulesets %+v)", s.names.Component, rulesets)
	}
	type grant struct {
		Name        string `json:"name"`
		Permissions struct {
			Admin    bool `json:"admin"`
			Maintain bool `json:"maintain"`
		} `json:"permissions"`
	}
	var granted []grant
	if err := s.F.GetJSON(ctx, s.teamPath()+"/repos", &granted); err != nil {
		return err
	}
	i := slices.IndexFunc(granted, func(g grant) bool { return g.Name == s.names.Component })
	if i < 0 || !granted[i].Permissions.Maintain || granted[i].Permissions.Admin {
		return fmt.Errorf("team %s must maintain %s without admin (repositories %+v)", s.names.Group, s.names.Component, granted)
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
	if out := strings.ToUpper(login); out != login {
		return out
	}
	return strings.ToLower(login)
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
