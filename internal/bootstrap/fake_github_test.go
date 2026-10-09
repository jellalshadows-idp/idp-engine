package bootstrap

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
	"golang.org/x/crypto/nacl/box"
)

// testPub/testPriv is the keypair the fake hands out as every secrets public key,
// so tests can open the sealed values bootstrap stored.
var testPub, testPriv = mustKeyPair()

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
)

// fakeGitHub is an in-memory GitHub that implements exactly the endpoints
// bootstrap calls, answering in the shapes the real API uses. Every non-GET
// request is recorded in writes so tests can assert idempotency.
type fakeGitHub struct {
	t         *testing.T
	mu        sync.Mutex
	objects   map[string]any           // GET path -> JSON body
	rulesets  map[int64]map[string]any // ruleset id -> body
	forbidden map[string]bool          // paths that answer 403
	nextID    int64
	writes    []string
}

func newFakeGitHub(t *testing.T, org string) (*fakeGitHub, *ghapi.Client) {
	t.Helper()
	f := &fakeGitHub{
		t:         t,
		objects:   map[string]any{},
		rulesets:  map[int64]map[string]any{},
		forbidden: map[string]bool{},
		nextID:    100,
	}
	f.objects["/orgs/"+org+"/actions/permissions/workflow"] = map[string]any{
		"default_workflow_permissions": "write", "can_approve_pull_request_reviews": false,
	}
	f.objects["/orgs/"+org+"/installations"] = map[string]any{"total_count": 0, "installations": []any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, ghapi.New(srv.URL, "test-token")
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	if r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if r.Method != http.MethodGet {
		f.writes = append(f.writes, r.Method+" "+r.URL.Path)
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

func notFound() (int, any) { return http.StatusNotFound, map[string]any{"message": "Not Found"} }

func (f *fakeGitHub) route(method, path string, body map[string]any) (int, any) {
	if f.forbidden[path] {
		return http.StatusForbidden, map[string]any{"message": "Must have admin rights to Repository."}
	}
	switch {
	case method == http.MethodPost && reOrgRepos.MatchString(path):
		org := reOrgRepos.FindStringSubmatch(path)[1]
		name := body["name"].(string)
		f.objects["/repos/"+org+"/"+name] = map[string]any{"name": name, "visibility": body["visibility"], "default_branch": "main"}
		return http.StatusCreated, map[string]any{"name": name}
	case method == http.MethodPost && strings.HasSuffix(path, "/git/trees"):
		return http.StatusCreated, map[string]any{"sha": "tree-sha"}
	case method == http.MethodPost && strings.HasSuffix(path, "/git/commits"):
		return http.StatusCreated, map[string]any{"sha": "commit-sha"}
	case method == http.MethodPost && strings.HasSuffix(path, "/git/refs"):
		ref := strings.TrimPrefix(body["ref"].(string), "refs/")
		f.objects[strings.TrimSuffix(path, "/git/refs")+"/git/ref/"+ref] = map[string]any{"ref": body["ref"]}
		return http.StatusCreated, map[string]any{"ref": body["ref"]}
	case method == http.MethodGet && strings.HasSuffix(path, "/rulesets"):
		list := []any{}
		for _, id := range f.rulesetIDs() {
			list = append(list, map[string]any{"id": id, "name": f.rulesets[id]["name"]})
		}
		return http.StatusOK, list
	case method == http.MethodPost && strings.HasSuffix(path, "/rulesets"):
		f.nextID++
		body["id"] = f.nextID
		f.rulesets[f.nextID] = body
		return http.StatusCreated, body
	case reRulesetID.MatchString(path):
		id, _ := strconv.ParseInt(reRulesetID.FindStringSubmatch(path)[1], 10, 64)
		if method == http.MethodPut {
			body["id"] = id
			f.rulesets[id] = body
		}
		rs, ok := f.rulesets[id]
		if !ok {
			return notFound()
		}
		return http.StatusOK, rs
	case method == http.MethodPut && reEnv.MatchString(path):
		f.objects[path] = envGetShape(body)
		return http.StatusOK, f.objects[path]
	case reEnvPolicies.MatchString(path):
		policies := asList(f.objects[path])
		if method == http.MethodPost {
			f.nextID++
			policy := map[string]any{"id": f.nextID, "name": body["name"]}
			f.objects[path] = append(policies, policy)
			return http.StatusOK, policy
		}
		return http.StatusOK, map[string]any{"total_count": len(policies), "branch_policies": policies}
	case method == http.MethodDelete && reEnvPolicyID.MatchString(path):
		m := reEnvPolicyID.FindStringSubmatch(path)
		kept := []any{}
		for _, p := range asList(f.objects[m[1]]) {
			if strconv.FormatInt(int64(toFloat(p.(map[string]any)["id"])), 10) != m[2] {
				kept = append(kept, p)
			}
		}
		f.objects[m[1]] = kept
		return http.StatusNoContent, nil
	case method == http.MethodGet && strings.HasSuffix(path, "/public-key"):
		return http.StatusOK, map[string]any{"key_id": "key-1", "key": base64.StdEncoding.EncodeToString(testPub[:])}
	case method == http.MethodPost && strings.HasSuffix(path, "/variables"):
		f.objects[path+"/"+body["name"].(string)] = map[string]any{"name": body["name"], "value": body["value"]}
		return http.StatusCreated, map[string]any{}
	case method == http.MethodPut || method == http.MethodPatch:
		f.objects[path] = body
		return http.StatusNoContent, nil
	case method == http.MethodGet:
		obj, ok := f.objects[path]
		if !ok {
			return notFound()
		}
		return http.StatusOK, obj
	}
	f.t.Errorf("fakeGitHub: unexpected %s %s", method, path)
	return http.StatusTeapot, map[string]any{}
}

func (f *fakeGitHub) rulesetIDs() []int64 {
	ids := make([]int64, 0, len(f.rulesets))
	for id := range f.rulesets {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// rulesetByName returns the stored body of the named ruleset, or nil.
func (f *fakeGitHub) rulesetByName(name string) map[string]any {
	for _, rs := range f.rulesets {
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
			rm := r.(map[string]any)
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
