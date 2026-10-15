package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

// fakeForge models just enough of GitHub and the pipelines for the scenario:
// merging a PR applies its files to main and runs the effects the test
// scripted for that step; a reconcile run completes at once, or waits for an
// approval when the test says so.
type fakeForge struct {
	main     map[string][]byte            // files on main
	branches map[string]map[string][]byte // branch → files; a nil value deletes
	prs      map[int]string               // PR number → branch
	merged   map[string]string            // merge sha → step slug
	paths    map[string]any               // API path → JSON object; present = exists
	issues   map[string]int               // "label/title" → open issues
	gate     map[string]string            // slug → idp-gate conclusion (default success)
	comments map[string]string            // slug → plan comment text
	effects  map[string]func(*fakeForge)  // slug → what its reconcile does
	approval map[string]bool              // slug → its reconcile waits for idp-approval
	approved map[int64]bool
	rejected map[int64]bool
	raw      map[string]string           // slug → whole plan comment, verbatim
	fps      map[string]plan.Fingerprint // slug → fingerprint its plan comment carries
	failRun  map[string]bool             // slug → its reconcile concludes failure
	parent   map[string]string           // slug → first parent its merge commit reports (default: main's head before the merge)
	checkErr error                       // what CheckApprover answers
	unbranch error                       // what DeleteBranch answers
	active   []Run
	written  []string
	drifts   int
	head     int // main's head is fakeSHA(head); an effect that bumps it is someone else's push
	log      []string
}

func newFakeForge() *fakeForge {
	return &fakeForge{
		main: map[string][]byte{}, branches: map[string]map[string][]byte{}, prs: map[int]string{},
		merged: map[string]string{}, paths: map[string]any{}, issues: map[string]int{},
		gate: map[string]string{}, comments: map[string]string{}, effects: map[string]func(*fakeForge){},
		approval: map[string]bool{}, approved: map[int64]bool{},
		rejected: map[int64]bool{}, raw: map[string]string{}, fps: map[string]plan.Fingerprint{}, failRun: map[string]bool{},
		parent: map[string]string{},
	}
}

func slugOf(branch string) string   { return branch[strings.LastIndex(branch, "/")+1:] }
func fakeSHA(n int) string          { return fmt.Sprintf("%040d", n) }
func shaNumber(sha string) int      { var n int; _, _ = fmt.Sscanf(sha, "%d", &n); return n }
func reconcileRun(sha string) int64 { return int64(500 + shaNumber(sha)) }

func (f *fakeForge) HeadSHA(_ context.Context, branch string) (string, error) {
	if branch != "main" {
		return "", fmt.Errorf("unexpected branch %s", branch)
	}
	return fakeSHA(f.head), nil
}

func (f *fakeForge) CreateBranch(_ context.Context, branch, _ string) error {
	if _, ok := f.branches[branch]; ok {
		return fmt.Errorf("branch %s exists", branch)
	}
	f.branches[branch] = map[string][]byte{}
	f.log = append(f.log, "branch "+branch)
	return nil
}

func (f *fakeForge) File(_ context.Context, ref, path string) ([]byte, bool, error) {
	if ref != "main" {
		return nil, false, fmt.Errorf("unexpected ref %s", ref)
	}
	b, ok := f.main[path]
	return b, ok, nil
}

func (f *fakeForge) PutFile(_ context.Context, branch, path string, content []byte, _ string) error {
	f.branches[branch][path] = content
	f.written = append(f.written, path)
	return nil
}

func (f *fakeForge) DeleteFile(_ context.Context, branch, path, _ string) error {
	if _, ok := f.main[path]; !ok {
		return fmt.Errorf("%s does not exist", path)
	}
	f.branches[branch][path] = nil
	f.written = append(f.written, path)
	return nil
}

func (f *fakeForge) OpenPR(_ context.Context, head, _, _ string) (PR, error) {
	n := len(f.prs) + 1
	f.prs[n] = head
	f.log = append(f.log, "pr "+slugOf(head))
	return PR{Number: n, HeadSHA: fakeSHA(1000 + n)}, nil
}

func (f *fakeForge) Checks(_ context.Context, sha string) (map[string]string, error) {
	slug := slugOf(f.prs[shaNumber(sha)-1000])
	c := f.gate[slug]
	if c == "" {
		c = "success"
	}
	return map[string]string{"idp-gate": c}, nil
}

func (f *fakeForge) PlanComment(_ context.Context, pr int) (string, error) {
	slug := slugOf(f.prs[pr])
	if raw, ok := f.raw[slug]; ok { // replaces the whole comment; %SHA% is the PR head
		return strings.ReplaceAll(raw, "%SHA%", fakeSHA(1000+pr)), nil
	}
	c, ok := f.comments[slug]
	if !ok {
		c = "<code>github</code>: +2 ~0 -0 ±0"
	}
	fp := f.fps[slug]
	if fp == nil {
		fp = plan.Fingerprint{}
	}
	marker, err := plan.EncodeMarker(fakeSHA(1000+pr), fp)
	if err != nil {
		return "", err
	}
	return planMarker + "\n" + c + "\n" + marker, nil
}

func (f *fakeForge) Merge(_ context.Context, pr PR, _ string) (string, error) {
	if pr.HeadSHA != fakeSHA(1000+pr.Number) {
		return "", fmt.Errorf("PR #%d: 409 head moved", pr.Number) // GitHub refuses a merge whose sha is not the head
	}
	branch := f.prs[pr.Number]
	for p, b := range f.branches[branch] {
		if b == nil {
			delete(f.main, p)
		} else {
			f.main[p] = b
		}
	}
	f.head++
	sha := fakeSHA(f.head)
	f.merged[sha] = slugOf(branch)
	if fn := f.effects[slugOf(branch)]; fn != nil {
		fn(f)
	}
	f.log = append(f.log, "merge "+slugOf(branch))
	return sha, nil
}

func (f *fakeForge) Runs(_ context.Context, workflow, sha string) ([]Run, error) {
	switch workflow {
	case "reconcile.yaml":
		slug, ok := f.merged[sha]
		if !ok {
			return nil, nil
		}
		id := reconcileRun(sha)
		if f.rejected[id] || f.failRun[slug] {
			return []Run{{ID: id, Status: "completed", Conclusion: "failure"}}, nil
		}
		if f.approval[slug] && !f.approved[id] {
			return []Run{{ID: id, Status: "waiting"}}, nil
		}
		return []Run{{ID: id, Status: "completed", Conclusion: "success"}}, nil
	case "drift.yaml":
		runs := make([]Run, f.drifts)
		for i := range runs {
			runs[i] = Run{ID: int64(9000 + f.drifts - i), Status: "completed", Conclusion: "success"}
		}
		return runs, nil
	}
	return nil, fmt.Errorf("unexpected workflow %s", workflow)
}

func (f *fakeForge) Jobs(_ context.Context, id int64) (map[string]string, error) {
	approve := "skipped"
	if f.approval[f.merged[fakeSHA(int(id-500))]] {
		approve = "success"
	}
	return map[string]string{"reconcile / plan": "success", "reconcile / approve": approve, "reconcile / apply-github": "success"}, nil
}

func (f *fakeForge) Dispatch(_ context.Context, workflow, _ string) error {
	if workflow != "drift.yaml" {
		return fmt.Errorf("unexpected dispatch %s", workflow)
	}
	f.drifts++
	f.log = append(f.log, "drift")
	return nil
}

func (f *fakeForge) PendingApproval(_ context.Context, id int64) (int64, bool, error) {
	return 77, f.approval[f.merged[fakeSHA(int(id-500))]] && !f.approved[id], nil
}

func (f *fakeForge) Approve(_ context.Context, id, env int64, _ string) error {
	if env != 77 {
		return errors.New("wrong environment")
	}
	f.approved[id] = true
	f.log = append(f.log, fmt.Sprintf("approve %d", id))
	return nil
}

func (f *fakeForge) ClaimFiles(_ context.Context) ([]string, error) {
	var out []string
	for p := range f.main {
		if strings.HasPrefix(p, "claims/") {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeForge) OrgResources(_ context.Context, prefix string) ([]string, error) {
	var out []string
	for p := range f.paths {
		for _, root := range []string{"/repos/acme/", "/orgs/acme/teams/"} {
			name, ok := strings.CutPrefix(p, root)
			if ok && !strings.Contains(name, "/") && strings.HasPrefix(name, prefix) {
				out = append(out, name)
			}
		}
	}
	return out, nil
}

func (f *fakeForge) ActiveRuns(_ context.Context) ([]Run, error) { return f.active, nil }

func (f *fakeForge) Reject(_ context.Context, id, env int64, _ string) error {
	if env != 77 {
		return errors.New("wrong environment")
	}
	f.rejected[id] = true
	f.log = append(f.log, fmt.Sprintf("reject %d", id))
	return nil
}

func (f *fakeForge) Exists(_ context.Context, path string) (bool, error) {
	_, ok := f.paths[path]
	return ok, nil
}

func (f *fakeForge) GetJSON(_ context.Context, path string, out any) error {
	v, ok := f.paths[path]
	if !ok {
		return fmt.Errorf("GET %s: 404", path)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func (f *fakeForge) OpenIssues(_ context.Context, label, title string) (int, error) {
	return f.issues[label+"/"+title], nil
}

func (f *fakeForge) Parent(_ context.Context, sha string) (string, error) {
	slug, ok := f.merged[sha]
	if !ok {
		return "", fmt.Errorf("unexpected commit %s", sha)
	}
	if p, ok := f.parent[slug]; ok {
		return p, nil
	}
	return fakeSHA(shaNumber(sha) - 1), nil // the head main had when the merge landed
}

func (f *fakeForge) CheckApprover(context.Context) error { return f.checkErr }

func (f *fakeForge) DeleteBranch(_ context.Context, branch string) error {
	if f.unbranch != nil {
		return f.unbranch
	}
	delete(f.branches, branch)
	f.log = append(f.log, "delete branch "+branch)
	return nil
}

func (f *fakeForge) ClosePR(_ context.Context, number int) error {
	f.log = append(f.log, fmt.Sprintf("close %d", number))
	return nil
}
