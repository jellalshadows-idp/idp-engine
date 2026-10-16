package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// forgeAgainst returns a GitHub forge whose bot and approver clients talk to h.
func forgeAgainst(t *testing.T, h http.HandlerFunc) GitHub {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return GitHub{Bot: ghapi.New(srv.URL, "bot-token"), Approver: ghapi.New(srv.URL, "approver-token"), Owner: "acme", Repo: "idp-claims-e2e"}
}

func TestGitHubPutFileCreatesAndUpdates(t *testing.T) {
	var bodies []map[string]any
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/idp-claims-e2e/contents/claims/a.yaml":
			if r.URL.Query().Get("ref") == "new" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"sha":"old-blob","content":"eA==","encoding":"base64"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/repos/acme/idp-claims-e2e/contents/claims/a.yaml":
			var b map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			bodies = append(bodies, b)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
		}
	})
	for _, branch := range []string{"new", "old"} {
		if err := g.PutFile(context.Background(), branch, "claims/a.yaml", []byte("hello"), "m"); err != nil {
			t.Fatal(err)
		}
	}
	if len(bodies) != 2 || bodies[0]["sha"] != nil || bodies[1]["sha"] != "old-blob" {
		t.Errorf("bodies = %v: create sends no sha, update sends the current blob sha", bodies)
	}
	if bodies[0]["content"] != base64.StdEncoding.EncodeToString([]byte("hello")) || bodies[0]["branch"] != "new" {
		t.Errorf("create body = %v", bodies[0])
	}
}

func TestGitHubFileDecodesAndReportsMissing(t *testing.T) {
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/missing.yaml") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"content":"aGVs\nbG8=\n","encoding":"base64","sha":"x"}`))
	})
	b, ok, err := g.File(context.Background(), "main", "claims/a.yaml")
	if err != nil || !ok || string(b) != "hello" {
		t.Errorf("File = %q, %v, %v", b, ok, err)
	}
	if _, ok, err := g.File(context.Background(), "main", "claims/missing.yaml"); err != nil || ok {
		t.Errorf("missing file: ok=%v err=%v", ok, err)
	}
}

func TestGitHubApprovesWithTheApproverToken(t *testing.T) {
	var auth string
	var body map[string]any
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/idp-claims-e2e/actions/runs/9/pending_deployments":
			_, _ = w.Write([]byte(`[{"environment":{"id":5,"name":"idp-write"},"current_user_can_approve":true},{"environment":{"id":7,"name":"idp-approval"},"current_user_can_approve":true}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/idp-claims-e2e/actions/runs/9/pending_deployments":
			auth = r.Header.Get("Authorization")
			_ = json.NewDecoder(r.Body).Decode(&body)
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
		}
	})
	env, ok, err := g.PendingApproval(context.Background(), 9)
	if err != nil || !ok || env != 7 {
		t.Fatalf("PendingApproval = %d, %v, %v: only idp-approval counts", env, ok, err)
	}
	if err := g.Approve(context.Background(), 9, 7, "ok"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(auth, "approver-token") {
		t.Errorf("Authorization = %q: approvals use the approver token, never the bot's", auth)
	}
	if ids, _ := body["environment_ids"].([]any); len(ids) != 1 || ids[0] != float64(7) || body["state"] != "approved" || body["comment"] != "ok" {
		t.Errorf("approve body = %v", body)
	}
}

func TestGitHubChecksKeepTheNewestRunPerName(t *testing.T) {
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"check_runs":[{"id":1,"name":"idp-gate","status":"completed","conclusion":"failure"},{"id":3,"name":"idp-gate","status":"in_progress","conclusion":null},{"id":2,"name":"pr / plan","status":"completed","conclusion":"success"}]}`))
	})
	got, err := g.Checks(context.Background(), strings.Repeat("a", 40))
	if err != nil || got["idp-gate"] != "" || got["pr / plan"] != "success" {
		t.Errorf("Checks = %v, %v: a re-run in progress replaces the old failure", got, err)
	}
}

func TestGitHubRunsAndMerge(t *testing.T) {
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/idp-claims-e2e/actions/workflows/reconcile.yaml/runs" && r.URL.Query().Get("head_sha") == "abc":
			_, _ = w.Write([]byte(`{"workflow_runs":[{"id":11,"status":"waiting","conclusion":null}]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/repos/acme/idp-claims-e2e/pulls/4/merge":
			var b map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["merge_method"] != "merge" || b["sha"] != "h1" {
				t.Errorf("merge body = %v: want a merge pinned to the verified head h1", b)
			}
			_, _ = w.Write([]byte(`{"sha":"m1","merged":true}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
		}
	})
	runs, err := g.Runs(context.Background(), "reconcile.yaml", "abc")
	if err != nil || len(runs) != 1 || runs[0].ID != 11 || runs[0].Status != "waiting" {
		t.Errorf("Runs = %+v, %v", runs, err)
	}
	if sha, err := g.Merge(context.Background(), PR{Number: 4, HeadSHA: "h1"}, "t"); err != nil || sha != "m1" {
		t.Errorf("Merge = %q, %v", sha, err)
	}
}

func TestGitHubPlanCommentTrustsOnlyTheBotsEdits(t *testing.T) {
	var editor string
	var graphAuth string
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/idp-claims-e2e/issues/3/comments":
			fmt.Fprint(w, `[{"id":1,"node_id":"IC_1","body":"<!-- idp-plan -->\nplan","user":{"login":"github-actions[bot]","type":"Bot"}}]`)
		case r.Method == http.MethodPost && r.URL.Path == "/graphql":
			graphAuth = r.Header.Get("Authorization")
			var b struct {
				Variables map[string]any `json:"variables"`
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b.Variables["id"] != "IC_1" {
				t.Errorf("graphql variables = %v, want the comment's node id", b.Variables)
			}
			fmt.Fprintf(w, `{"data":{"node":{"editor":%s}}}`, editor)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
		}
	})
	editor = `{"login":"mallory","__typename":"User"}`
	_, err := g.PlanComment(context.Background(), 3)
	if err == nil || err.Error() != "the plan comment on PR #3 was edited by mallory: the harness trusts only the bot's own comment" {
		t.Fatalf("err = %v", err)
	}
	if graphAuth != "Bearer bot-token" {
		t.Errorf("Authorization = %q: the editor check reads with the bot token", graphAuth)
	}
	for _, e := range []string{`null`, `{"login":"github-actions","__typename":"Bot"}`} {
		editor = e
		if body, err := g.PlanComment(context.Background(), 3); err != nil || body != "<!-- idp-plan -->\nplan" {
			t.Errorf("editor %s: PlanComment = %q, %v; an unedited comment and the bot's own edit are trusted", e, body, err)
		}
	}
}

func TestGitHubChecksTheApproverToken(t *testing.T) {
	status := http.StatusOK
	var auth string
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/acme/idp-claims-e2e/actions/runs" || r.URL.Query().Get("per_page") != "1" {
			t.Errorf("unexpected %s %s", r.Method, r.URL)
		}
		auth = r.Header.Get("Authorization")
		w.WriteHeader(status)
		fmt.Fprint(w, `{"total_count":0,"workflow_runs":[]}`)
	})
	if err := g.CheckApprover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer approver-token" {
		t.Errorf("Authorization = %q: the check must use the approver token", auth)
	}
	status = http.StatusUnauthorized
	err := g.CheckApprover(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "the approver token cannot read the claims repo's runs (expired or missing Actions read?): rotate it, see docs/e2e.md: ") {
		t.Fatalf("err = %v", err)
	}
}

func TestGitHubParentIsTheFirstParent(t *testing.T) {
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/idp-claims-e2e/commits/m1":
			fmt.Fprint(w, `{"sha":"m1","parents":[{"sha":"base1"},{"sha":"head1"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/idp-claims-e2e/commits/root":
			fmt.Fprint(w, `{"sha":"root","parents":[]}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
		}
	})
	if p, err := g.Parent(context.Background(), "m1"); err != nil || p != "base1" {
		t.Errorf("Parent = %q, %v; want the first parent", p, err)
	}
	if _, err := g.Parent(context.Background(), "root"); err == nil {
		t.Error("a commit without parents is not a merge")
	}
}

func TestGitHubDeleteBranchAcceptsAMissingBranch(t *testing.T) {
	status := http.StatusNoContent
	var got []string
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		w.WriteHeader(status)
	})
	if err := g.DeleteBranch(context.Background(), "e2e/42/create"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"DELETE /repos/acme/idp-claims-e2e/git/refs/heads/e2e/42/create Bearer bot-token"}; !slices.Equal(got, want) {
		t.Errorf("requests = %v, want %v", got, want)
	}
	status = http.StatusNotFound
	if err := g.DeleteBranch(context.Background(), "e2e/42/create"); err != nil {
		t.Errorf("a branch that is already gone is deleted: %v", err)
	}
	status = http.StatusUnprocessableEntity
	if err := g.DeleteBranch(context.Background(), "e2e/42/create"); err == nil {
		t.Error("want the error of a failed delete")
	}
}

func TestGitHubClosePR(t *testing.T) {
	var got string
	var body map[string]any
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.Path + " " + r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, `{}`)
	})
	if err := g.ClosePR(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	if got != "PATCH /repos/acme/idp-claims-e2e/pulls/5 Bearer bot-token" || len(body) != 1 || body["state"] != "closed" {
		t.Errorf("request = %s %v", got, body)
	}
}

func TestGitHubClaimFilesListsBothDirectories(t *testing.T) {
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/idp-claims-e2e/contents/claims/groups" && r.URL.Query().Get("ref") == "main":
			fmt.Fprint(w, `[{"path":"claims/groups/platform.yaml","type":"file"},{"path":"claims/groups/README.md","type":"file"},{"path":"claims/groups/old","type":"dir"}]`)
		default:
			http.NotFound(w, r)
		}
	})
	got, err := g.ClaimFiles(context.Background())
	if err != nil || !slices.Equal(got, []string{"claims/groups/platform.yaml"}) {
		t.Fatalf("ClaimFiles = %v, %v; want only the yaml file, and a missing directory is empty", got, err)
	}
}

func TestGitHubOrgResourcesMatchesRepoNamesAndTeamSlugs(t *testing.T) {
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/acme/repos":
			fmt.Fprint(w, `[{"name":"e2e-r7-app"},{"name":"idp-claims"}]`)
		case "/orgs/acme/teams":
			fmt.Fprint(w, `[{"name":"E2E r7 team","slug":"e2e-r7-team"},{"name":"platform","slug":"platform"}]`)
		default:
			http.NotFound(w, r)
		}
	})
	got, err := g.OrgResources(context.Background(), "e2e-r")
	if err != nil || !slices.Equal(got, []string{"e2e-r7-app", "e2e-r7-team"}) {
		t.Fatalf("OrgResources = %v, %v", got, err)
	}
}

func TestGitHubActiveRunsSkipsCompletedRuns(t *testing.T) {
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/idp-claims-e2e/actions/workflows/reconcile.yaml/runs":
			fmt.Fprint(w, `{"workflow_runs":[{"id":1,"status":"completed"},{"id":2,"status":"waiting"}]}`)
		case "/repos/acme/idp-claims-e2e/actions/workflows/drift.yaml/runs":
			fmt.Fprint(w, `{"workflow_runs":[{"id":3,"status":"queued"}]}`)
		default:
			http.NotFound(w, r) // recover.yaml missing: no runs
		}
	})
	got, err := g.ActiveRuns(context.Background())
	if err != nil || len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("ActiveRuns = %v, %v", got, err)
	}
}

func TestGitHubRejectsWithTheApproverToken(t *testing.T) {
	var body map[string]any
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/acme/idp-claims-e2e/actions/runs/9/pending_deployments" || r.Header.Get("Authorization") != "Bearer approver-token" {
			http.Error(w, "unexpected request", http.StatusTeapot)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `[]`)
	})
	if err := g.Reject(context.Background(), 9, 77, "not expected"); err != nil {
		t.Fatal(err)
	}
	if body["state"] != "rejected" || body["comment"] != "not expected" {
		t.Errorf("body = %v", body)
	}
}

func TestGitHubRetriesTransientReadsOnly(t *testing.T) {
	old := retryDelays
	retryDelays = []time.Duration{0, 0}
	t.Cleanup(func() { retryDelays = old })

	calls := 0
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, `{"object":{"sha":"abc"}}`)
	})
	if sha, err := g.HeadSHA(context.Background(), "main"); err != nil || sha != "abc" || calls != 2 {
		t.Fatalf("HeadSHA = %q, %v after %d calls; a 502 must be retried", sha, err, calls)
	}

	calls = 0
	g = forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "forbidden", http.StatusForbidden)
	})
	if _, err := g.HeadSHA(context.Background(), "main"); err == nil || calls != 1 {
		t.Fatalf("err = %v after %d calls; a 403 must not be retried", err, calls)
	}

	calls = 0
	g = forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "bad gateway", http.StatusBadGateway)
	})
	if err := g.Dispatch(context.Background(), "drift.yaml", "main"); err == nil || calls != 1 {
		t.Fatalf("err = %v after %d calls; a write must never be retried", err, calls)
	}
}

var _ Forge = GitHub{}
