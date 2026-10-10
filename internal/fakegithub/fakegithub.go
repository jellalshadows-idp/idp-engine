// Package fakegithub is test-only infrastructure: an in-memory GitHub REST
// server that answers in the shapes the real API uses and records every write.
// It implements the endpoints the idp commands call.
package fakegithub

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/nacl/box"
)

// PublicKey/PrivateKey is the keypair the fake hands out as every secrets public
// key, so tests can open the sealed values bootstrap stored.
var PublicKey, PrivateKey = mustKeyPair()

func mustKeyPair() (*[32]byte, *[32]byte) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return pub, priv
}

var (
	reOrgRepos    = regexp.MustCompile(`^/orgs/([^/]+)/repos$`)
	reRulesetID   = regexp.MustCompile(`^/repos/[^/]+/[^/]+/rulesets/(\d+)$`)
	reEnv         = regexp.MustCompile(`^/repos/[^/]+/[^/]+/environments/[^/]+$`)
	reEnvPolicies = regexp.MustCompile(`^/repos/[^/]+/[^/]+/environments/[^/]+/deployment-branch-policies$`)
	reEnvPolicyID = regexp.MustCompile(`^(/repos/[^/]+/[^/]+/environments/[^/]+/deployment-branch-policies)/(\d+)$`)
	reOrgWorkflow = regexp.MustCompile(`^/orgs/[^/]+/actions/permissions/workflow$`)
	reSecret      = regexp.MustCompile(`^/repos/[^/]+/[^/]+/(environments/[^/]+/|actions/)secrets/[^/]+$`)
	reVariable    = regexp.MustCompile(`^/repos/[^/]+/[^/]+/(environments/[^/]+/|actions/)variables/[^/]+$`)

	reIssueComment   = regexp.MustCompile(`^(/repos/[^/]+/[^/]+)/issues/comments/(\d+)$`)
	reIssue          = regexp.MustCompile(`^(/repos/[^/]+/[^/]+/issues)/(\d+)$`)
	reLabels         = regexp.MustCompile(`^/repos/[^/]+/[^/]+/labels$`)
	reIssueComments  = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/\d+/comments$`)
	reIssues         = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues$`)
	rePullsForCommit = regexp.MustCompile(`^/repos/[^/]+/[^/]+/commits/[^/]+/pulls$`)
)

// Request is one recorded write.
type Request struct {
	Method string
	Path   string
	Body   map[string]any
}

// Server is an in-memory GitHub. Every non-GET request is recorded in Writes so
// tests can assert idempotency. Tests may read and mutate the exported fields
// directly, but only while no request is in flight.
type Server struct {
	// URL is the base URL of the underlying httptest server.
	URL string
	// Writes holds "METHOD /path" for every non-GET request, in order.
	Writes []string
	// Objects maps a GET path to its JSON body.
	Objects map[string]any
	// Rulesets maps a ruleset id to its stored body.
	Rulesets map[int64]map[string]any
	// Forbidden lists paths that answer 403.
	Forbidden map[string]bool
	// LastAuthorization is the Authorization header of the latest request.
	LastAuthorization string
	// Queries maps a request path to the raw query string of its latest request.
	Queries map[string]string
	// Lists maps a GET path to every item of a paginated list endpoint. The fake
	// serves pages with per_page and page, and a Link rel="next" header.
	Lists map[string][]any
	// MaxPerPage, when positive, caps per_page so tests can force several pages.
	MaxPerPage int
	// Requests holds every non-GET request with its decoded JSON body, in order.
	Requests []Request
	// Actor is the user the fake attributes the comments it creates to.
	Actor map[string]any

	t      testing.TB
	mu     sync.Mutex
	nextID int64
}

// New starts a fake GitHub for org, seeded with the org workflow permissions and
// an empty installations list, and shuts it down when the test ends.
func New(t testing.TB, org string) *Server {
	t.Helper()
	f := &Server{
		t:         t,
		Objects:   map[string]any{},
		Rulesets:  map[int64]map[string]any{},
		Forbidden: map[string]bool{},
		Queries:   map[string]string{},
		Lists:     map[string][]any{},
		Actor:     map[string]any{"login": "github-actions[bot]", "type": "Bot"},
		nextID:    100,
	}
	f.Objects["/orgs/"+org+"/actions/permissions/workflow"] = map[string]any{
		"default_workflow_permissions": "write", "can_approve_pull_request_reviews": false,
	}
	f.Objects["/orgs/"+org+"/installations"] = map[string]any{"total_count": 0, "installations": []any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	f.URL = srv.URL
	return f
}

func (f *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.LastAuthorization = r.Header.Get("Authorization")
	f.Queries[r.URL.Path] = r.URL.RawQuery
	var body map[string]any
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			f.t.Errorf("fakegithub: %s %s: invalid JSON body: %v", r.Method, r.URL.Path, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	if r.Method != http.MethodGet {
		f.Writes = append(f.Writes, r.Method+" "+r.URL.Path)
		f.Requests = append(f.Requests, Request{Method: r.Method, Path: r.URL.Path, Body: body})
	}
	if r.Method == http.MethodGet {
		if items, ok := f.Lists[r.URL.Path]; ok || isListPath(r.URL.Path) {
			f.servePage(w, r, items)
			return
		}
	}
	status, resp := f.route(r.Method, r.URL.Path, body)
	if resp == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// isListPath reports whether path is a list endpoint the idp commands read.
func isListPath(path string) bool {
	return reIssueComments.MatchString(path) || reIssues.MatchString(path) || rePullsForCommit.MatchString(path)
}

// servePage answers one page of a list endpoint the way GitHub does.
func (f *Server) servePage(w http.ResponseWriter, r *http.Request, items []any) {
	q := r.URL.Query()
	per, _ := strconv.Atoi(q.Get("per_page"))
	if per <= 0 {
		per = 30
	}
	if per > 100 {
		per = 100
	}
	if f.MaxPerPage > 0 && per > f.MaxPerPage {
		per = f.MaxPerPage
	}
	page, _ := strconv.Atoi(q.Get("page"))
	if page <= 0 {
		page = 1
	}
	start := min((page-1)*per, len(items))
	end := min(start+per, len(items))
	if end < len(items) {
		q.Set("page", strconv.Itoa(page+1))
		w.Header().Set("Link", fmt.Sprintf(`<%s%s?%s>; rel="next"`, f.URL, r.URL.Path, q.Encode()))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(append([]any{}, items[start:end]...))
}

func notFound() (int, any) { return http.StatusNotFound, map[string]any{"message": "Not Found"} }

func (f *Server) route(method, path string, body map[string]any) (int, any) {
	if f.Forbidden[path] {
		return http.StatusForbidden, map[string]any{"message": "Must have admin rights to Repository."}
	}
	switch {
	case method == http.MethodPost && reOrgRepos.MatchString(path):
		org := reOrgRepos.FindStringSubmatch(path)[1]
		name, ok := f.stringField(method, path, body, "name")
		if !ok {
			return http.StatusBadRequest, map[string]any{}
		}
		f.Objects["/repos/"+org+"/"+name] = map[string]any{"name": name, "visibility": body["visibility"], "default_branch": "main"}
		return http.StatusCreated, map[string]any{"name": name}
	case method == http.MethodPost && strings.HasSuffix(path, "/git/trees"):
		return http.StatusCreated, map[string]any{"sha": "tree-sha"}
	case method == http.MethodPost && strings.HasSuffix(path, "/git/commits"):
		return http.StatusCreated, map[string]any{"sha": "commit-sha"}
	case method == http.MethodPost && strings.HasSuffix(path, "/git/refs"):
		refName, ok := f.stringField(method, path, body, "ref")
		if !ok {
			return http.StatusBadRequest, map[string]any{}
		}
		ref := strings.TrimPrefix(refName, "refs/")
		f.Objects[strings.TrimSuffix(path, "/git/refs")+"/git/ref/"+ref] = map[string]any{"ref": body["ref"]}
		return http.StatusCreated, map[string]any{"ref": body["ref"]}
	case method == http.MethodGet && strings.HasSuffix(path, "/rulesets"):
		list := []any{}
		for _, id := range f.rulesetIDs() {
			list = append(list, map[string]any{"id": id, "name": f.Rulesets[id]["name"]})
		}
		return http.StatusOK, list
	// The fake echoes ruleset bodies verbatim; live-ruleset-*.json fixtures pin the real GitHub shape.
	case method == http.MethodPost && strings.HasSuffix(path, "/rulesets"):
		if body == nil {
			return f.bodyRequired(method, path)
		}
		f.nextID++
		body["id"] = f.nextID
		f.Rulesets[f.nextID] = body
		return http.StatusCreated, body
	case (method == http.MethodGet || method == http.MethodPut) && reRulesetID.MatchString(path):
		id, _ := strconv.ParseInt(reRulesetID.FindStringSubmatch(path)[1], 10, 64)
		if method == http.MethodPut {
			if body == nil {
				return f.bodyRequired(method, path)
			}
			body["id"] = id
			f.Rulesets[id] = body
		}
		rs, ok := f.Rulesets[id]
		if !ok {
			return notFound()
		}
		return http.StatusOK, rs
	case method == http.MethodPut && reEnv.MatchString(path):
		f.Objects[path] = envGetShape(body)
		return http.StatusOK, f.Objects[path]
	case (method == http.MethodGet || method == http.MethodPost) && reEnvPolicies.MatchString(path):
		policies := asList(f.Objects[path])
		if method == http.MethodPost {
			f.nextID++
			policy := map[string]any{"id": f.nextID, "name": body["name"]}
			f.Objects[path] = append(policies, policy)
			return http.StatusOK, policy
		}
		return http.StatusOK, map[string]any{"total_count": len(policies), "branch_policies": policies}
	case method == http.MethodDelete && reEnvPolicyID.MatchString(path):
		m := reEnvPolicyID.FindStringSubmatch(path)
		kept := []any{}
		for _, p := range asList(f.Objects[m[1]]) {
			pm, _ := p.(map[string]any)
			if strconv.FormatInt(int64(toFloat(pm["id"])), 10) != m[2] {
				kept = append(kept, p)
			}
		}
		f.Objects[m[1]] = kept
		return http.StatusNoContent, nil
	case method == http.MethodGet && strings.HasSuffix(path, "/public-key"):
		return http.StatusOK, map[string]any{"key_id": "key-1", "key": base64.StdEncoding.EncodeToString(PublicKey[:])}
	case method == http.MethodGet && strings.HasSuffix(path, "/secrets"):
		list := []any{}
		for p := range f.Objects {
			name, ok := strings.CutPrefix(p, path+"/")
			if ok && name != "public-key" && !strings.Contains(name, "/") {
				list = append(list, map[string]any{"name": name})
			}
		}
		return http.StatusOK, map[string]any{"total_count": len(list), "secrets": list}
	case method == http.MethodPost && strings.HasSuffix(path, "/variables"):
		name, ok := f.stringField(method, path, body, "name")
		if !ok {
			return http.StatusBadRequest, map[string]any{}
		}
		f.Objects[path+"/"+name] = map[string]any{"name": body["name"], "value": body["value"]}
		return http.StatusCreated, map[string]any{}
	case method == http.MethodPut && (reOrgWorkflow.MatchString(path) || reSecret.MatchString(path)),
		method == http.MethodPatch && reVariable.MatchString(path):
		f.Objects[path] = body
		return http.StatusNoContent, nil
	case method == http.MethodPost && reIssueComments.MatchString(path):
		f.nextID++
		comment := map[string]any{"id": f.nextID, "body": body["body"], "user": f.Actor}
		f.Lists[path] = append(f.Lists[path], comment)
		return http.StatusCreated, comment
	case method == http.MethodPatch && reIssueComment.MatchString(path):
		m := reIssueComment.FindStringSubmatch(path)
		for listPath, items := range f.Lists {
			if !strings.HasPrefix(listPath, m[1]+"/issues/") || !strings.HasSuffix(listPath, "/comments") {
				continue
			}
			for _, it := range items {
				c, _ := it.(map[string]any)
				if strconv.FormatInt(int64(toFloat(c["id"])), 10) == m[2] {
					c["body"] = body["body"]
					return http.StatusOK, c
				}
			}
		}
		return notFound()
	case method == http.MethodPost && reIssues.MatchString(path):
		labels := []any{}
		for _, l := range asList(body["labels"]) {
			labels = append(labels, map[string]any{"name": l})
		}
		issue := map[string]any{"number": len(f.Lists[path]) + 1, "title": body["title"], "body": body["body"], "state": "open", "labels": labels}
		f.Lists[path] = append(f.Lists[path], issue)
		return http.StatusCreated, issue
	case method == http.MethodPatch && reIssue.MatchString(path):
		m := reIssue.FindStringSubmatch(path)
		for _, it := range f.Lists[m[1]] {
			is, _ := it.(map[string]any)
			if strconv.Itoa(int(toFloat(is["number"]))) == m[2] {
				for k, v := range body {
					is[k] = v
				}
				return http.StatusOK, is
			}
		}
		return notFound()
	case method == http.MethodPost && reLabels.MatchString(path):
		name, ok := f.stringField(method, path, body, "name")
		if !ok {
			return http.StatusBadRequest, map[string]any{}
		}
		f.Objects[path+"/"+name] = body
		return http.StatusCreated, body
	case method == http.MethodGet:
		obj, ok := f.Objects[path]
		if !ok {
			return notFound()
		}
		return http.StatusOK, obj
	}
	f.t.Errorf("fakegithub: unexpected %s %s", method, path)
	return http.StatusTeapot, map[string]any{}
}

// bodyRequired fails the test and answers 400 for a write that needs a JSON body.
func (f *Server) bodyRequired(method, path string) (int, any) {
	f.t.Errorf("fakegithub: %s %s: request body is required", method, path)
	return http.StatusBadRequest, map[string]any{}
}

// stringField returns body[field] as a string. When it is absent or not a
// string it fails the test with a clear message and reports false, so the caller
// can answer 400 instead of panicking inside the HTTP handler.
func (f *Server) stringField(method, path string, body map[string]any, field string) (string, bool) {
	v, ok := body[field].(string)
	if !ok {
		f.t.Errorf("fakegithub: %s %s: missing string field %s", method, path, field)
	}
	return v, ok
}

func (f *Server) rulesetIDs() []int64 {
	ids := make([]int64, 0, len(f.Rulesets))
	for id := range f.Rulesets {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// RulesetByName returns the stored body of the named ruleset, or nil.
func (f *Server) RulesetByName(name string) map[string]any {
	for _, rs := range f.Rulesets {
		if rs["name"] == name {
			return rs
		}
	}
	return nil
}

// envGetShape converts a PUT environment body into what GET returns.
func envGetShape(put map[string]any) map[string]any {
	rules := []any{}
	if reviewers := asList(put["reviewers"]); len(reviewers) > 0 {
		shaped := []any{}
		for _, r := range reviewers {
			rm, _ := r.(map[string]any)
			shaped = append(shaped, map[string]any{"type": rm["type"], "reviewer": map[string]any{"id": rm["id"]}})
		}
		rules = append(rules, map[string]any{"type": "required_reviewers", "prevent_self_review": put["prevent_self_review"], "reviewers": shaped})
	}
	return map[string]any{"protection_rules": rules, "deployment_branch_policy": put["deployment_branch_policy"]}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

// asList returns v as a list, or an empty list when the field is absent.
func asList(v any) []any {
	l, _ := v.([]any)
	if l == nil {
		return []any{}
	}
	return l
}
