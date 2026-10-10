package plan

import (
	"fmt"
	"strings"
)

// Decision is what the reconcile gate decided.
type Decision string

const (
	Auto     Decision = "auto"     // apply without waiting
	Approval Decision = "approval" // wait for a reviewer in idp-approval
)

// Verdict is a decision and why it was made.
type Verdict struct {
	Decision Decision
	Reason   string
}

// PRPlan is the fingerprint a reviewer saw on the merged pull request.
type PRPlan struct {
	Number      int
	Fingerprint Fingerprint
}

// NeedsPR reports whether Decide needs the pull request's fingerprint, so a
// caller can skip the GitHub lookups when the answer is already known.
func NeedsPR(current Fingerprint) bool {
	return !current.Empty() && len(current.Destructive()) == 0
}

// Decide applies spec §6.2 step 1.5 and amendment A3 (ADR-0018). Nothing to
// apply is auto. Any delete or replace is approval. Otherwise it is auto only
// when every change appears in the same stack of the PR's fingerprint.
// whyNoPR explains a nil pr in the reason.
func Decide(current Fingerprint, pr *PRPlan, whyNoPR string) Verdict {
	if current.Empty() {
		return Verdict{Auto, "no changes to apply"}
	}
	if d := current.Destructive(); len(d) > 0 {
		return Verdict{Approval, "destructive changes: " + summarize(d)}
	}
	if pr == nil {
		if whyNoPR == "" {
			whyNoPR = "no pull request plan found for this commit"
		}
		return Verdict{Approval, whyNoPR}
	}
	if missing := current.Missing(pr.Fingerprint); len(missing) > 0 {
		return Verdict{Approval, fmt.Sprintf("changes not shown in PR #%d: %s", pr.Number, summarize(missing))}
	}
	return Verdict{Auto, fmt.Sprintf("every change was shown in PR #%d", pr.Number)}
}

func summarize(entries []string) string {
	const limit = 5
	if len(entries) <= limit {
		return strings.Join(entries, "; ")
	}
	return strings.Join(entries[:limit], "; ") + fmt.Sprintf(" and %d more", len(entries)-limit)
}
