package e2e

import "context"

// PR is a pull request the harness opened.
type PR struct {
	Number  int
	HeadSHA string
	BaseSHA string // the main head its branch was created from
}

// Run is one workflow run.
type Run struct {
	ID         int64
	Status     string // queued, in_progress, waiting, completed
	Conclusion string // empty until completed
}

// Forge is everything the scenario needs from GitHub. The GitHub
// implementation holds two tokens: the bot App's for repository work and the
// approver account's for idp-approval deployments (ADR-0027).
type Forge interface {
	HeadSHA(ctx context.Context, branch string) (string, error)
	CreateBranch(ctx context.Context, branch, fromSHA string) error
	// File returns a file at ref, and false when it does not exist.
	File(ctx context.Context, ref, path string) ([]byte, bool, error)
	PutFile(ctx context.Context, branch, path string, content []byte, message string) error
	DeleteFile(ctx context.Context, branch, path, message string) error
	OpenPR(ctx context.Context, head, title, body string) (PR, error)
	// Checks maps each check run on sha to its conclusion, empty while it runs.
	Checks(ctx context.Context, sha string) (map[string]string, error)
	// PlanComment is the newest plan comment github-actions[bot] wrote on pr,
	// or "" when there is none yet; a comment anyone else edited is an error.
	PlanComment(ctx context.Context, pr int) (string, error)
	// Merge merges pr with a merge commit, only while its head is pr.HeadSHA.
	Merge(ctx context.Context, pr PR, title string) (string, error)
	// Runs lists a workflow's runs for a head commit, newest first.
	Runs(ctx context.Context, workflow, sha string) ([]Run, error)
	// Jobs maps each job of a run to its conclusion.
	Jobs(ctx context.Context, runID int64) (map[string]string, error)
	Dispatch(ctx context.Context, workflow, ref string) error
	// PendingApproval returns the idp-approval environment a run waits on.
	PendingApproval(ctx context.Context, runID int64) (int64, bool, error)
	Approve(ctx context.Context, runID, envID int64, comment string) error
	// Exists reports whether an API path answers (false on 404).
	Exists(ctx context.Context, path string) (bool, error)
	GetJSON(ctx context.Context, path string, out any) error
	// OpenIssues counts open issues the Actions bot opened with a label and title.
	OpenIssues(ctx context.Context, label, title string) (int, error)
	// ClaimFiles lists the claim file paths on the default branch
	// (claims/groups/*.yaml and claims/components/*.yaml).
	ClaimFiles(ctx context.Context) ([]string, error)
	// OrgResources lists the org's repository and team names that start with prefix.
	OrgResources(ctx context.Context, prefix string) ([]string, error)
	// ActiveRuns lists the claims repo's reconcile, drift and recover runs that
	// have not completed.
	ActiveRuns(ctx context.Context) ([]Run, error)
	// Reject rejects a pending idp-approval deployment, so nothing is applied.
	Reject(ctx context.Context, runID, envID int64, comment string) error
	// Parent returns a commit's first parent: for a merge commit, main's head
	// when it landed.
	Parent(ctx context.Context, sha string) (string, error)
	// CheckApprover fails when the approver token cannot read the claims repo's runs.
	CheckApprover(ctx context.Context) error
	// DeleteBranch deletes a branch; one that is already gone is not an error.
	DeleteBranch(ctx context.Context, branch string) error
	ClosePR(ctx context.Context, number int) error
}
