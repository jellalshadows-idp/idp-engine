# Phase 0 — Bootstrap and Spike Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the two GitHub orgs, their Apps and protected claims repos with a
tested `idp bootstrap` command. Then measure the four unknowns the design depends
on, and record everything in ADRs.

**Architecture:**
- **The bootstrap is real product code.** It is a Go subcommand in `idp-engine`
  (`idp bootstrap app|apply|check`), built TDD against an in-memory fake of the
  GitHub REST API, then run for real against both claims repos of the single org.
- **The spikes are throwaway.** They live in a separate throwaway repo
  (`jellalshadows-idp/idp-spike`) and run in GitHub Actions, because
  floci needs Docker and the owner has none locally. Their output is numbers and
  verdicts in ADR-0013; their code is not kept.

**Tech Stack:**
- Go 1.26, standard library plus `golang.org/x/crypto/nacl/box`.
- GitHub REST API (`X-GitHub-Api-Version: 2022-11-28`).
- OpenTofu 1.12.6, providers `integrations/github` 6.13.0 and `hashicorp/aws`
  6.66.0.
- floci 2.1.0, and GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially
§2.3, §7, §9.2, §10 and §11.

## Global Constraints

**Names**
- Go module: `github.com/jellalshadows-idp/idp-engine`. Go directive `go 1.26.0`.
  The only non-stdlib dependency allowed in this phase is
  `golang.org/x/crypto v0.57.0`.
- Org: a single GitHub Free org, `jellalshadows-idp` (owner decision 2026-10-09: no
  sandbox org; tests are isolated by repo and by the `e2e-`/`spike-` prefix, spec §8.5).
- Claims repos: `idp-claims` (production) and `idp-claims-e2e` (tests), each with its own
  `wet` branch, encrypted state and passphrase.
- Apps: `jellalshadows-idp-reader` and `jellalshadows-idp-writer` (at most 34 characters),
  created once and shared by both claims repos.
- Approver: `jellalshadows`. Test account: `adrian-da-silva`.
- Names shared with Phase 1 workflows (changing one is a breaking change):
  - branch `wet`;
  - environments `idp-approval` and `idp-write`;
  - check `idp-gate`;
  - secrets `IDP_READER_PRIVATE_KEY`, `IDP_STATE_PASSPHRASE` and
    `IDP_WRITER_PRIVATE_KEY` (the last one only in `idp-write`);
  - variables `IDP_READER_CLIENT_ID` and `IDP_WRITER_CLIENT_ID` (the last one only
    in `idp-write`).

**Pinned versions.** Every pin must be at least 14 days old when adopted:
- `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1` (v7.0.1)
- `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` (v7.0.0)
- `actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1` (v3.2.0)
- `opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4` (v2.0.2)
- OpenTofu `1.12.6`, `integrations/github` `6.13.0`, `hashicorp/aws` `6.66.0`
- `floci/floci:2.1.0@sha256:2e2343974a15137a6bda6de5a9e0b16207f97a786cc57ee9c2bf25d14a67b84c`
- actionlint `v1.7.12`, zizmor `1.30.1`

**Policies**
- Org workflow defaults set by bootstrap: `default_workflow_permissions: read` and
  `can_approve_pull_request_reviews: false`. Enabling Actions-created PRs is
  Phase 4 (spec §9.2).
- Commits use conventional messages with **no AI attribution or Co-Authored-By**.
  Doc-only changes go straight to `main`; code goes through a PR.
- Docs are in English. License: Apache-2.0.
- No local Docker: floci and anything needing containers run only in CI.
- No `go build` steps (owner rule). `go test` is the feedback loop. Locally use
  `go test ./...`; `-race` runs only in CI, because it needs cgo.
- Shell rule for executors: never use `cat`, `grep`, `find`, `sed` or `ls`. Use the
  Read tool, `rg`, `eza` or `gh --jq`.
- **Outward-facing steps need the owner's explicit OK right before running them.**
  That covers creating repos, pushing, setting secrets, and running
  `bootstrap apply` against a real org.

## Review Focus

These are the input classes most likely to bite, each with the test that pins it:

1. **GitHub returns rulesets with extra or defaulted fields, and in another rule
   order.** Without handling, `apply` would "update" forever and `check` would
   always report drift. Pinned by `TestRulesetDriftIgnoresGitHubExtras` in Task 8.
2. **The passphrase file ends with CRLF**, because it was written on Windows. If
   the `\r` were kept, the stored passphrase would differ from the owner's copy and
   state would be unreadable later. Pinned by `TestReadPassphrase` in Task 5.
3. **The `gh` token lacks `admin:org`**, which is the owner's case today. The
   command must say exactly how to fix it, not dump a bare 403 or 404. Pinned by
   `TestApplyExplainsMissingAdminOrgScope` in Task 7.
4. **Re-running `apply` after rotating an App key.** Secrets are write-only, so
   `apply` keeps the old value. The owner must be told, not left believing it
   rotated. Pinned by `TestApplyTwiceIsIdempotent`, which asserts the
   "kept existing secret" log, in Task 9.
5. **The claims repo exists but is private**, for example created by hand.
   Rulesets on Free need public repos, so `check` must flag it. Pinned by the
   `private repo` case of `TestCheckReportsDrift` in Task 10.

## File Structure

```
idp-engine/
├─ .gitattributes                       # LF everywhere (CI is Linux)
├─ LICENSE                              # Apache-2.0
├─ README.md                            # status + honest limitations
├─ go.mod, go.sum
├─ cmd/idp/main.go                      # os.Exit(cli.Run(...))
├─ internal/cli/
│  ├─ cli.go                            # Run + top-level dispatch
│  ├─ cli_test.go
│  ├─ bootstrap.go                      # `idp bootstrap app|apply|check` flags → bootstrap pkg
│  └─ bootstrap_test.go
├─ internal/ghapi/
│  ├─ client.go                         # minimal REST client, ErrNotFound, APIError
│  └─ client_test.go
├─ internal/bootstrap/
│  ├─ compare.go                        # Mismatches (subset diff), normalize, asList
│  ├─ compare_test.go
│  ├─ config.go                         # Config, AppCredentials, validation, LoadAppCredentials, ReadPassphrase, UserID
│  ├─ config_test.go
│  ├─ desired.go                        # names, Ruleset types, MainRuleset, WetRuleset, Environments
│  ├─ desired_test.go                   # golden tests
│  ├─ testdata/*.golden.json
│  ├─ seal.go                           # libsodium sealed box for the secrets API
│  ├─ seal_test.go
│  ├─ bootstrapper.go                   # Bootstrapper, Apply, ensure* steps
│  ├─ drift.go                          # rulesetView/rulesetDrift, environmentDrift, branch policies
│  ├─ check.go                          # Check, Finding
│  ├─ fake_github_test.go               # in-memory GitHub used by bootstrapper/check tests
│  ├─ bootstrapper_test.go
│  ├─ check_test.go
│  ├─ appflow.go                        # Manifest + AppFlow (local manifest-flow server)
│  └─ appflow_test.go
├─ bootstrap/apps/
│  ├─ apps.go                           # //go:embed *.json
│  ├─ reader.json                       # read-only manifest
│  └─ writer.json                       # write manifest (incl. workflows: write)
├─ docs/runbooks/bootstrap.md
├─ docs/adr/0001…0013-*.md
└─ .github/workflows/ci.yaml            # go test/vet/gofmt + actionlint + zizmor

idp-spike/  (separate local folder → jellalshadows-idp/idp-spike, THROWAWAY)
├─ README.md
├─ state/main.tf                        # S1: encrypted state round-trip
├─ github/main.tf                       # S2: GitHub provider with App token
├─ aws/main.tf                          # S3: floci multi-account + replay timing
├─ ci/floci_override.tf.json            # S3: emulator settings, copied in by CI
└─ .github/workflows/{spike-state,spike-github,spike-floci}.yaml
```

---

### Task 0: Owner setup (manual, browser)

**Files:** none.

**Interfaces:**
- Produces: the org `jellalshadows-idp`, and a
  `gh` token for `jellalshadows` with the `admin:org` scope.

- [ ] **Step 1: Create the org**

Go to https://github.com/account/organizations/new, choose the **Free** plan, and
create `jellalshadows-idp`. The owner is `jellalshadows`. (Done by the owner on
2026-10-09.)

- [ ] **Step 2: Add the `admin:org` scope to the local gh token**

Run: `gh auth switch --user jellalshadows; gh auth refresh -h github.com -s admin:org`
Expected: the browser device flow completes. Then
`gh auth status 2>&1 | rg "Token scopes"` lists `admin:org`.

- [ ] **Step 3: Verify the org exists**

Run: `gh api user/orgs --jq '.[].login'`
Expected: `jellalshadows-idp`.

---

### Task 1: Repo hygiene and publish `idp-engine`

**Files:**
- Create: `.gitattributes`, `LICENSE`, `README.md`
- Modify: none (the two spec commits already exist)

**Interfaces:**
- Consumes: Task 0 (the org exists).
- Produces: the public repo `jellalshadows-idp/idp-engine` with `main` pushed.

- [ ] **Step 1: Write `.gitattributes`**

```gitattributes
* text=auto eol=lf
*.pem binary
```

- [ ] **Step 2: Renormalize existing files to LF**

Run: `git add --renormalize . && git status --short`
Expected: the spec file shows as modified (CRLF → LF) or nothing changes. Either is fine.

- [ ] **Step 3: Write `LICENSE`**

Run: `gh api licenses/apache-2.0 --jq .body > LICENSE`
Expected: `rg -c "Apache License" LICENSE` prints a number ≥ 1.

- [ ] **Step 4: Write `README.md`**

```markdown
# idp-engine

The engine of an internal developer platform (IDP) that reconciles on **GitHub Actions** instead of a Kubernetes operator. It is inspired by [Firestartr](https://github.com/firestartr-pro/firestartr).

> **Status:** Phase 0 (bootstrap + spike). Nothing here is usable yet.

- Design: [docs/superpowers/specs/2026-10-08-idp-on-actions-design.md](docs/superpowers/specs/2026-10-08-idp-on-actions-design.md)
- Plans: [docs/superpowers/plans/](docs/superpowers/plans/)

## Honest limitations (v1)

- AWS is **emulated** with [floci](https://floci.io). No real cloud account is used.
- Every repository is **public**: on the GitHub Free plan, rulesets and environment reviewers exist only for public repos.
- **No secrets** flow through the platform.

## License

[Apache-2.0](LICENSE)
```

- [ ] **Step 5: Commit (doc/meta only, straight to main)**

```bash
git add .gitattributes LICENSE README.md docs
git commit -m "chore: add license, readme and lf normalization"
```

- [ ] **Step 6: Create the public repo and push** (outward-facing: ask the owner first)

Run: `gh repo create jellalshadows-idp/idp-engine --public --source . --remote origin --push --description "IDP engine: claims to GitHub/AWS via GitHub Actions (Firestartr-inspired)"`
Expected: the URL `https://github.com/jellalshadows-idp/idp-engine` is printed, and
`git log origin/main --oneline | rg -c .` prints `4`: spec, spec amendment, this
plan, and the chore commit.

---

### Task 2: Go module, CLI skeleton and CI

**Files:**
- Create: `go.mod`, `cmd/idp/main.go`, `internal/cli/cli.go`, `internal/cli/cli_test.go`, `.github/workflows/ci.yaml`

**Interfaces:**
- Produces:
  - `type Env func(key string) string`
  - `func Run(args []string, stdout, stderr io.Writer, env Env) int` in package
    `github.com/jellalshadows-idp/idp-engine/internal/cli`. Exit codes: `0` ok,
    `1` runtime failure or drift, `2` usage error.

- [ ] **Step 1: Create the branch**

Run: `git switch -c feat/phase-0-bootstrap`

- [ ] **Step 2: Write `go.mod`**

```
module github.com/jellalshadows-idp/idp-engine

go 1.26.0
```

- [ ] **Step 3: Write the failing test `internal/cli/cli_test.go`**

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "no args prints usage to stderr", args: nil, wantCode: 2, wantStderr: "Usage:"},
		{name: "help prints usage to stdout", args: []string{"help"}, wantCode: 0, wantStdout: "Usage:"},
		{name: "unknown command fails", args: []string{"nope"}, wantCode: 2, wantStderr: `unknown command "nope"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr, noEnv)
			if code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
```

- [ ] **Step 4: Run it and see it fail**

Run: `go test ./internal/cli/`
Expected: FAIL to compile, with `undefined: Run`.

- [ ] **Step 5: Implement `internal/cli/cli.go`**

```go
// Package cli implements the idp command-line interface.
package cli

import (
	"fmt"
	"io"
)

const usage = `idp — IDP engine CLI

Usage:
  idp <command> [flags]

Commands:
  help        Show this help
`

// Env looks up an environment variable: os.Getenv in production, a map in tests.
type Env func(key string) string

// Run executes the CLI and returns the process exit code
// (0 ok, 1 runtime failure or drift, 2 usage error).
func Run(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "idp: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
```

- [ ] **Step 6: Write `cmd/idp/main.go`**

```go
package main

import (
	"os"

	"github.com/jellalshadows-idp/idp-engine/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}
```

- [ ] **Step 7: Run the tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok .../internal/cli`, no vet output, and `gofmt -l` prints nothing.

- [ ] **Step 8: Write `.github/workflows/ci.yaml`**

```yaml
name: ci

on:
  pull_request:
  push:
    branches: [main]

permissions: {}

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

jobs:
  go:
    runs-on: ubuntu-24.04
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - name: gofmt
        run: |
          unformatted="$(gofmt -l .)"
          if [ -n "$unformatted" ]; then echo "$unformatted"; exit 1; fi
      - run: go vet ./...
      - run: go test -race ./...

  workflows:
    runs-on: ubuntu-24.04
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - name: actionlint
        run: go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
      - name: zizmor
        run: pipx run zizmor==1.30.1 .github/workflows
```

- [ ] **Step 9: Commit, push, and open the PR** (outward-facing: ask first)

```bash
git add go.mod cmd internal .github
git commit -m "feat(cli): add idp cli skeleton and ci workflow"
git push -u origin feat/phase-0-bootstrap
gh pr create --repo jellalshadows-idp/idp-engine --title "feat: phase 0 bootstrap command" --body "Implements docs/superpowers/plans/2026-10-08-phase-0-bootstrap-and-spike.md Tasks 2-12."
```
Expected: CI runs on the PR, and both `go` and `workflows` jobs pass. If zizmor
reports findings, fix the workflow (do not suppress them) and push again.

---

### Task 3: GitHub REST client

**Files:**
- Create: `internal/ghapi/client.go`, `internal/ghapi/client_test.go`

**Interfaces:**
- Produces (package `github.com/jellalshadows-idp/idp-engine/internal/ghapi`):
  - `const DefaultBaseURL = "https://api.github.com"`
  - `var ErrNotFound error`, returned wrapped on 404 (`errors.Is(err, ghapi.ErrNotFound)`)
  - `type APIError struct { Method, Path string; Status int; Body string }`
  - `func New(baseURL, token string) *Client`. An empty token means unauthenticated.
  - `(*Client).Get(ctx, path string, out any) error`
  - `(*Client).Post` / `Put` / `Patch(ctx, path string, in, out any) error`
  - `(*Client).Delete(ctx, path string) error`

- [ ] **Step 1: Write the failing tests `internal/ghapi/client_test.go`**

```go
package ghapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostSendsHeadersAndBodyAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tkn" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer tkn")
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
			t.Errorf("X-GitHub-Api-Version = %q", got)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/orgs/acme/repos" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		var in map[string]string
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in["name"] != "x" {
			t.Errorf("body = %v (err %v), want name=x", in, err)
		}
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id": 7}`)
	}))
	defer srv.Close()

	var out struct {
		ID int `json:"id"`
	}
	err := New(srv.URL, "tkn").Post(context.Background(), "/orgs/acme/repos", map[string]string{"name": "x"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != 7 {
		t.Errorf("ID = %d, want 7", out.ID)
	}
}

func TestEmptyTokenSendsNoAuthorization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want none", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := New(srv.URL, "").Delete(context.Background(), "/x"); err != nil {
		t.Fatal(err)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		wantNotFound bool
		wantStatus   int
	}{
		{name: "404 wraps ErrNotFound", status: http.StatusNotFound, wantNotFound: true},
		{name: "422 is an APIError", status: http.StatusUnprocessableEntity, wantStatus: http.StatusUnprocessableEntity},
		{name: "403 is an APIError", status: http.StatusForbidden, wantStatus: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				io.WriteString(w, `{"message":"nope"}`)
			}))
			defer srv.Close()

			err := New(srv.URL, "").Get(context.Background(), "/x", nil)
			if got := errors.Is(err, ErrNotFound); got != tt.wantNotFound {
				t.Errorf("errors.Is(err, ErrNotFound) = %v, want %v (err %v)", got, tt.wantNotFound, err)
			}
			if tt.wantStatus != 0 {
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.Status != tt.wantStatus {
					t.Errorf("err = %v, want *APIError with status %d", err, tt.wantStatus)
				}
			}
		})
	}
}

func TestNoContentWithOutIsFine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var out map[string]any
	if err := New(srv.URL, "t").Put(context.Background(), "/x", map[string]any{}, &out); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/ghapi/`
Expected: FAIL to compile, with `undefined: New`.

- [ ] **Step 3: Implement `internal/ghapi/client.go`**

```go
// Package ghapi is a minimal GitHub REST client: JSON in, JSON out, typed errors.
package ghapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultBaseURL is the public GitHub REST API.
const DefaultBaseURL = "https://api.github.com"

const apiVersion = "2022-11-28"

// ErrNotFound is returned (wrapped) when the API answers 404.
var ErrNotFound = errors.New("not found")

// APIError is any non-2xx answer other than 404.
type APIError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github %s %s: %d %s", e.Method, e.Path, e.Status, e.Body)
}

// Client talks to the GitHub REST API. Build it with New.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New returns a client. An empty token sends unauthenticated requests.
func New(baseURL, token string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{}}
}

// Get decodes the JSON answer of GET path into out (out may be nil).
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.Do(ctx, http.MethodGet, path, nil, out)
}

// Post sends in as JSON (in may be nil) and decodes the answer into out (out may be nil).
func (c *Client) Post(ctx context.Context, path string, in, out any) error {
	return c.Do(ctx, http.MethodPost, path, in, out)
}

// Put sends in as JSON and decodes the answer into out (out may be nil).
func (c *Client) Put(ctx context.Context, path string, in, out any) error {
	return c.Do(ctx, http.MethodPut, path, in, out)
}

// Patch sends in as JSON and decodes the answer into out (out may be nil).
func (c *Client) Patch(ctx context.Context, path string, in, out any) error {
	return c.Do(ctx, http.MethodPatch, path, in, out)
}

// Delete sends DELETE path.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.Do(ctx, http.MethodDelete, path, nil, nil)
}

// Do performs one request. A 404 returns an error wrapping ErrNotFound;
// any other status >= 300 returns *APIError.
func (c *Client) Do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s %s: %w", method, path, err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("github %s %s: %w", method, path, ErrNotFound)
	}
	if resp.StatusCode >= 300 {
		return &APIError{Method: method, Path: path, Status: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests and see them pass**

Run: `go test ./internal/ghapi/ && gofmt -l .`
Expected: `ok`, and `gofmt -l` prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/ghapi
git commit -m "feat(ghapi): add minimal github rest client"
```

---

### Task 4: Subset diff (`Mismatches`)

**Files:**
- Create: `internal/bootstrap/compare.go`, `internal/bootstrap/compare_test.go`

**Interfaces:**
- Produces (package `github.com/jellalshadows-idp/idp-engine/internal/bootstrap`):
  - `func Mismatches(desired, actual any) []string`. It returns JSON-path-like
    strings such as `$.rules.pull_request.parameters.x`, or `nil` when actual
    contains desired.
  - `func normalize(v any) (any, error)`. It round-trips through `encoding/json`
    into `map[string]any`, `[]any` and scalars.
  - `func asList(v any) []any`. It returns `[]any{}` when `v` is not a list.

- [ ] **Step 1: Write the failing test `internal/bootstrap/compare_test.go`**

```go
package bootstrap

import (
	"encoding/json"
	"reflect"
	"testing"
)

func decode(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMismatches(t *testing.T) {
	tests := []struct {
		name    string
		desired string
		actual  string
		want    []string
	}{
		{name: "identical", desired: `{"a":1}`, actual: `{"a":1}`, want: nil},
		{name: "extra keys in actual are ignored", desired: `{"a":1}`, actual: `{"a":1,"node_id":"x"}`, want: nil},
		{name: "missing key", desired: `{"a":1,"b":2}`, actual: `{"a":1}`, want: []string{"$.b"}},
		{name: "different scalar", desired: `{"a":"read"}`, actual: `{"a":"write"}`, want: []string{"$.a"}},
		{name: "array length differs", desired: `{"l":[1,2]}`, actual: `{"l":[1]}`, want: []string{"$.l"}},
		{name: "nested element differs", desired: `{"l":[{"x":1},{"x":2}]}`, actual: `{"l":[{"x":1},{"x":3}]}`, want: []string{"$.l[1].x"}},
		{name: "type differs", desired: `{"m":{"k":1}}`, actual: `{"m":"k"}`, want: []string{"$.m"}},
		{name: "empty array vs missing key", desired: `{"l":[]}`, actual: `{}`, want: []string{"$.l"}},
		{name: "several mismatches are sorted by key", desired: `{"b":1,"a":1}`, actual: `{"b":2,"a":2}`, want: []string{"$.a", "$.b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mismatches(decode(t, tt.desired), decode(t, tt.actual))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Mismatches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeTurnsStructsIntoGenericJSON(t *testing.T) {
	got, err := normalize(struct {
		N int      `json:"n"`
		L []string `json:"l"`
	}{N: 2, L: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"n": float64(2), "l": []any{"a"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalize = %#v, want %#v", got, want)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/bootstrap/`
Expected: FAIL to compile, with `undefined: Mismatches`.

- [ ] **Step 3: Implement `internal/bootstrap/compare.go`**

```go
// Package bootstrap creates and verifies the org-level setup the IDP depends on
// (spec §9.2): the claims repo, the wet branch, rulesets, environments,
// secrets and variables. It deliberately lives outside the claims loop.
package bootstrap

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// Mismatches lists the paths where actual does not contain desired.
// Objects are compared as subsets, so keys GitHub adds on its own never count.
// Arrays must have the same length and match element by element. Both values
// must be in encoding/json's generic form (see normalize).
func Mismatches(desired, actual any) []string {
	var out []string
	walk("$", desired, actual, &out)
	return out
}

func walk(path string, desired, actual any, out *[]string) {
	switch d := desired.(type) {
	case map[string]any:
		a, ok := actual.(map[string]any)
		if !ok {
			*out = append(*out, path)
			return
		}
		keys := make([]string, 0, len(d))
		for k := range d {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walk(path+"."+k, d[k], a[k], out)
		}
	case []any:
		a, ok := actual.([]any)
		if !ok || len(a) != len(d) {
			*out = append(*out, path)
			return
		}
		for i := range d {
			walk(fmt.Sprintf("%s[%d]", path, i), d[i], a[i], out)
		}
	default:
		if !reflect.DeepEqual(desired, actual) {
			*out = append(*out, path)
		}
	}
}

// normalize turns any JSON-encodable value into the generic form Mismatches expects.
func normalize(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	err = json.Unmarshal(raw, &out)
	return out, err
}

// asList returns v as a list, or an empty list when GitHub omitted the field.
func asList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return []any{}
}
```

- [ ] **Step 4: Run the tests and see them pass**

Run: `go test ./internal/bootstrap/ && gofmt -l .`
Expected: `ok`, and nothing from gofmt.

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap
git commit -m "feat(bootstrap): add subset diff for desired vs live json"
```

---

### Task 5: Config, credentials files and passphrase

**Files:**
- Create: `internal/bootstrap/config.go`, `internal/bootstrap/config_test.go`

**Interfaces:**
- Consumes: `ghapi.Client` (Task 3).
- Produces:
  - `type AppCredentials struct { ID int64 \`json:"id"\`; ClientID string \`json:"client_id"\`; Slug string \`json:"slug"\`; PrivateKey string \`json:"-"\` }`
  - `type Config struct { Org, ClaimsRepo string; ApproverID int64; Reader, Writer AppCredentials; Passphrase string }`
  - `func (c Config) ValidateCheck() error` and `func (c Config) ValidateApply() error`.
    Both report every problem at once, joined with `"; "`.
  - `func LoadAppCredentials(jsonPath string, withKey bool) (AppCredentials, error)`.
    It reads `<slug>.pem` next to the JSON file when `withKey` is set.
  - `func ReadPassphrase(path string) (string, error)`. It trims trailing `\r\n`
    and requires at least 16 characters.
  - `func UserID(ctx context.Context, api *ghapi.Client, login string) (int64, error)`

- [ ] **Step 1: Write the failing tests `internal/bootstrap/config_test.go`**

```go
package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

func validConfig() Config {
	return Config{
		Org: "acme", ClaimsRepo: "idp-claims", ApproverID: 42,
		Reader:     AppCredentials{ID: 1, ClientID: "Iv-reader", Slug: "acme-reader", PrivateKey: "reader-pem"},
		Writer:     AppCredentials{ID: 2, ClientID: "Iv-writer", Slug: "acme-writer", PrivateKey: "writer-pem"},
		Passphrase: "correct-horse-battery-staple",
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Config)
		apply     bool
		wantError string
	}{
		{name: "valid for apply", mutate: func(*Config) {}, apply: true},
		{name: "check ignores keys and passphrase", mutate: func(c *Config) { c.Passphrase = ""; c.Reader.PrivateKey = "" }, apply: false},
		{name: "every problem is reported", mutate: func(c *Config) { c.Org = ""; c.ApproverID = 0 }, apply: true, wantError: "org is required; approver id must be positive"},
		{name: "short passphrase", mutate: func(c *Config) { c.Passphrase = "short" }, apply: true, wantError: "passphrase must be at least 16 characters"},
		{name: "missing writer key", mutate: func(c *Config) { c.Writer.PrivateKey = "" }, apply: true, wantError: "writer private key is missing"},
		{name: "incomplete reader app", mutate: func(c *Config) { c.Reader.ClientID = "" }, apply: false, wantError: "reader app id, client id and slug are required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConfig()
			tt.mutate(&c)
			var err error
			if tt.apply {
				err = c.ValidateApply()
			} else {
				err = c.ValidateCheck()
			}
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.wantError)
			}
		})
	}
}

func TestLoadAppCredentials(t *testing.T) {
	dir := t.TempDir()
	json := `{"id": 77, "client_id": "Iv23abc", "slug": "acme-writer"}`
	if err := os.WriteFile(filepath.Join(dir, "acme-writer.json"), []byte(json), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "acme-writer.pem"), []byte("PEM"), 0o600); err != nil {
		t.Fatal(err)
	}

	noKey, err := LoadAppCredentials(filepath.Join(dir, "acme-writer.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	if noKey.ID != 77 || noKey.ClientID != "Iv23abc" || noKey.Slug != "acme-writer" || noKey.PrivateKey != "" {
		t.Errorf("without key = %+v", noKey)
	}
	withKey, err := LoadAppCredentials(filepath.Join(dir, "acme-writer.json"), true)
	if err != nil {
		t.Fatal(err)
	}
	if withKey.PrivateKey != "PEM" {
		t.Errorf("PrivateKey = %q, want PEM", withKey.PrivateKey)
	}
}

func TestReadPassphrase(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		want      string
		wantError bool
	}{
		{name: "crlf from windows editors is trimmed", content: "correct-horse-battery-staple\r\n", want: "correct-horse-battery-staple"},
		{name: "lf is trimmed", content: "correct-horse-battery-staple\n", want: "correct-horse-battery-staple"},
		{name: "inner spaces are kept", content: "correct horse battery staple", want: "correct horse battery staple"},
		{name: "too short after trimming", content: "fifteen-chars!!\r\n", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pass")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadPassphrase(path)
			if tt.wantError {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ReadPassphrase = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestUserID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/jellalshadows" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"login":"jellalshadows","id":4242}`))
	}))
	defer srv.Close()

	id, err := UserID(context.Background(), ghapi.New(srv.URL, "t"), "jellalshadows")
	if err != nil || id != 4242 {
		t.Fatalf("UserID = %d, %v; want 4242", id, err)
	}
	if _, err := UserID(context.Background(), ghapi.New(srv.URL, "t"), "ghost"); err == nil {
		t.Fatal("want error for unknown user")
	}
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/bootstrap/`
Expected: FAIL to compile, with `undefined: Config`.

- [ ] **Step 3: Implement `internal/bootstrap/config.go`**

```go
package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// AppCredentials identify one GitHub App created by `idp bootstrap app`.
// The JSON file never contains the key; it lives in <slug>.pem next to it.
type AppCredentials struct {
	ID         int64  `json:"id"`
	ClientID   string `json:"client_id"`
	Slug       string `json:"slug"`
	PrivateKey string `json:"-"`
}

// Config is everything one org bootstrap needs.
type Config struct {
	Org        string
	ClaimsRepo string
	ApproverID int64 // GitHub user id of the platform admin who approves runs
	Reader     AppCredentials
	Writer     AppCredentials
	Passphrase string // OpenTofu state passphrase (apply only)
}

// ValidateCheck reports what `check` needs: identities, no key material.
func (c Config) ValidateCheck() error {
	return joinProblems(c.baseProblems())
}

// ValidateApply reports what `apply` needs: identities plus keys and passphrase.
func (c Config) ValidateApply() error {
	problems := c.baseProblems()
	if c.Reader.PrivateKey == "" {
		problems = append(problems, "reader private key is missing")
	}
	if c.Writer.PrivateKey == "" {
		problems = append(problems, "writer private key is missing")
	}
	if utf8.RuneCountInString(c.Passphrase) < 16 {
		problems = append(problems, "passphrase must be at least 16 characters")
	}
	return joinProblems(problems)
}

func (c Config) baseProblems() []string {
	var problems []string
	if c.Org == "" {
		problems = append(problems, "org is required")
	}
	if c.ClaimsRepo == "" {
		problems = append(problems, "claims repo is required")
	}
	if c.ApproverID <= 0 {
		problems = append(problems, "approver id must be positive")
	}
	for _, app := range []struct {
		role  string
		creds AppCredentials
	}{{"reader", c.Reader}, {"writer", c.Writer}} {
		if app.creds.ID <= 0 || app.creds.ClientID == "" || app.creds.Slug == "" {
			problems = append(problems, app.role+" app id, client id and slug are required")
		}
	}
	return problems
}

func joinProblems(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

// LoadAppCredentials reads the <slug>.json written by `idp bootstrap app` and,
// when withKey is set, the <slug>.pem next to it.
func LoadAppCredentials(jsonPath string, withKey bool) (AppCredentials, error) {
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return AppCredentials{}, err
	}
	var c AppCredentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return AppCredentials{}, fmt.Errorf("%s: %w", jsonPath, err)
	}
	if withKey {
		pem, err := os.ReadFile(filepath.Join(filepath.Dir(jsonPath), c.Slug+".pem"))
		if err != nil {
			return AppCredentials{}, err
		}
		c.PrivateKey = string(pem)
	}
	return c, nil
}

// ReadPassphrase loads the state passphrase, trimming the line ending editors
// add (LF or CRLF), and enforces OpenTofu's PBKDF2 minimum of 16 characters.
func ReadPassphrase(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	p := strings.TrimRight(string(raw), "\r\n")
	if utf8.RuneCountInString(p) < 16 {
		return "", fmt.Errorf("passphrase in %s is shorter than 16 characters", path)
	}
	return p, nil
}

// UserID resolves a login to its numeric id; environment reviewers need ids.
func UserID(ctx context.Context, api *ghapi.Client, login string) (int64, error) {
	var u struct {
		ID int64 `json:"id"`
	}
	if err := api.Get(ctx, "/users/"+url.PathEscape(login), &u); err != nil {
		return 0, fmt.Errorf("resolve user %s: %w", login, err)
	}
	return u.ID, nil
}
```

- [ ] **Step 4: Run the tests and see them pass**

Run: `go test ./internal/bootstrap/ && gofmt -l .`
Expected: `ok`, and nothing from gofmt.

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap
git commit -m "feat(bootstrap): add config validation, credentials and passphrase loading"
```

---

### Task 6: Desired rulesets and environments (golden tests)

**Files:**
- Create: `internal/bootstrap/desired.go`, `internal/bootstrap/desired_test.go`, `internal/bootstrap/testdata/{ruleset-main,ruleset-wet,environment-approval,environment-write}.golden.json`

**Interfaces:**
- Consumes: `Config` (Task 5).
- Produces:
  - constants `WetBranch`, `EnvApproval`, `EnvWrite`, `GateCheck`,
    `SecretReaderKey`, `SecretPassphrase`, `SecretWriterKey`, `VarReaderClient`
    and `VarWriterClient`, with the values from Global Constraints;
  - types `Ruleset`, `BypassActor`, `Conditions`, `RefName` and `Rule`;
  - `func MainRuleset() Ruleset` and `func WetRuleset(writerAppID int64) Ruleset`;
  - `type Environment struct { Name string; ReviewerIDs []int64; Branch string }`;
  - `func Environments(cfg Config) []Environment`, which returns `idp-approval`
    first and `idp-write` second;
  - `func (e Environment) putBody() map[string]any`.

- [ ] **Step 1: Write the failing golden test `internal/bootstrap/desired_test.go`**

```go
package bootstrap

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestDesiredGolden(t *testing.T) {
	cfg := Config{ApproverID: 42, Writer: AppCredentials{ID: 2}}
	envs := Environments(cfg)
	cases := []struct {
		name  string
		value any
	}{
		{"ruleset-main", MainRuleset()},
		{"ruleset-wet", WetRuleset(cfg.Writer.ID)},
		{"environment-approval", envs[0].putBody()},
		{"environment-write", envs[1].putBody()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.MarshalIndent(tc.value, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", tc.name+".golden.json")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run `go test ./internal/bootstrap/ -run TestDesiredGolden -update` to create it)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s differs from its golden file:\n%s", tc.name, got)
			}
		})
	}
}

func TestEnvironmentsOrderAndReviewers(t *testing.T) {
	envs := Environments(Config{ApproverID: 42})
	if len(envs) != 2 || envs[0].Name != EnvApproval || envs[1].Name != EnvWrite {
		t.Fatalf("Environments = %+v, want [idp-approval idp-write]", envs)
	}
	if len(envs[0].ReviewerIDs) != 1 || envs[0].ReviewerIDs[0] != 42 {
		t.Errorf("idp-approval reviewers = %v, want [42]", envs[0].ReviewerIDs)
	}
	if len(envs[1].ReviewerIDs) != 0 {
		t.Errorf("idp-write must have no reviewers (approval lives in idp-approval), got %v", envs[1].ReviewerIDs)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/bootstrap/`
Expected: FAIL to compile, with `undefined: Environments`.

- [ ] **Step 3: Implement `internal/bootstrap/desired.go`**

```go
package bootstrap

// Names shared with the Phase 1 reusable workflows. Renaming one is a breaking change.
const (
	WetBranch        = "wet"
	EnvApproval      = "idp-approval"
	EnvWrite         = "idp-write"
	GateCheck        = "idp-gate"
	SecretReaderKey  = "IDP_READER_PRIVATE_KEY"
	SecretPassphrase = "IDP_STATE_PASSPHRASE"
	SecretWriterKey  = "IDP_WRITER_PRIVATE_KEY"
	VarReaderClient  = "IDP_READER_CLIENT_ID"
	VarWriterClient  = "IDP_WRITER_CLIENT_ID"
)

// Ruleset is the part of GitHub's ruleset API that bootstrap owns.
type Ruleset struct {
	Name         string        `json:"name"`
	Target       string        `json:"target"`
	Enforcement  string        `json:"enforcement"`
	BypassActors []BypassActor `json:"bypass_actors"`
	Conditions   Conditions    `json:"conditions"`
	Rules        []Rule        `json:"rules"`
}

// BypassActor may skip the ruleset.
type BypassActor struct {
	ActorID    int64  `json:"actor_id"`
	ActorType  string `json:"actor_type"`
	BypassMode string `json:"bypass_mode"`
}

// Conditions select the refs a ruleset applies to.
type Conditions struct {
	RefName RefName `json:"ref_name"`
}

// RefName lists ref patterns to include and exclude.
type RefName struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

// Rule is one ruleset rule; Parameters is omitted for rules without any.
type Rule struct {
	Type       string         `json:"type"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

// MainRuleset protects the claims repo's default branch: PR, idp-gate,
// up to date, no force push, no deletion, and no bypass at all (spec §7.4).
// Approval of applies lives in the idp-approval environment, so PRs need
// zero review approvals here.
func MainRuleset() Ruleset {
	return Ruleset{
		Name:         "idp-main",
		Target:       "branch",
		Enforcement:  "active",
		BypassActors: []BypassActor{},
		Conditions:   Conditions{RefName: RefName{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{}}},
		Rules: []Rule{
			{Type: "deletion"},
			{Type: "non_fast_forward"},
			{Type: "pull_request", Parameters: map[string]any{
				"dismiss_stale_reviews_on_push":     false,
				"require_code_owner_review":         false,
				"require_last_push_approval":        false,
				"required_approving_review_count":   0,
				"required_review_thread_resolution": false,
			}},
			{Type: "required_status_checks", Parameters: map[string]any{
				"strict_required_status_checks_policy": true,
				"required_status_checks":               []map[string]any{{"context": GateCheck}},
			}},
		},
	}
}

// WetRuleset lets only the writer App update the wet branch (spec §7.4).
func WetRuleset(writerAppID int64) Ruleset {
	return Ruleset{
		Name:         "idp-wet",
		Target:       "branch",
		Enforcement:  "active",
		BypassActors: []BypassActor{{ActorID: writerAppID, ActorType: "Integration", BypassMode: "always"}},
		Conditions:   Conditions{RefName: RefName{Include: []string{"refs/heads/" + WetBranch}, Exclude: []string{}}},
		Rules: []Rule{
			{Type: "deletion"},
			{Type: "non_fast_forward"},
			{Type: "update", Parameters: map[string]any{"update_allows_fetch_and_merge": false}},
		},
	}
}

// Environment is the desired shape of one deployment environment.
type Environment struct {
	Name        string
	ReviewerIDs []int64 // users; empty means no required reviewers
	Branch      string  // the only branch allowed to deploy
}

// Environments returns idp-approval (reviewers, no secrets) and idp-write
// (writer key, no reviewers), both restricted to main (spec §6.2, §7.3).
func Environments(cfg Config) []Environment {
	return []Environment{
		{Name: EnvApproval, ReviewerIDs: []int64{cfg.ApproverID}, Branch: "main"},
		{Name: EnvWrite, ReviewerIDs: []int64{}, Branch: "main"},
	}
}

// putBody is the JSON for PUT /repos/{owner}/{repo}/environments/{name}.
func (e Environment) putBody() map[string]any {
	reviewers := []map[string]any{}
	for _, id := range e.ReviewerIDs {
		reviewers = append(reviewers, map[string]any{"type": "User", "id": id})
	}
	return map[string]any{
		"wait_timer":          0,
		"prevent_self_review": false,
		"reviewers":           reviewers,
		"deployment_branch_policy": map[string]any{
			"protected_branches":     false,
			"custom_branch_policies": true,
		},
	}
}
```

- [ ] **Step 4: Generate the goldens and review them against this plan**

Run: `go test ./internal/bootstrap/ -run TestDesiredGolden -update`
Then read `internal/bootstrap/testdata/ruleset-main.golden.json`. It must be **exactly**:

```json
{
  "name": "idp-main",
  "target": "branch",
  "enforcement": "active",
  "bypass_actors": [],
  "conditions": {
    "ref_name": {
      "include": [
        "~DEFAULT_BRANCH"
      ],
      "exclude": []
    }
  },
  "rules": [
    {
      "type": "deletion"
    },
    {
      "type": "non_fast_forward"
    },
    {
      "type": "pull_request",
      "parameters": {
        "dismiss_stale_reviews_on_push": false,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_approving_review_count": 0,
        "required_review_thread_resolution": false
      }
    },
    {
      "type": "required_status_checks",
      "parameters": {
        "required_status_checks": [
          {
            "context": "idp-gate"
          }
        ],
        "strict_required_status_checks_policy": true
      }
    }
  ]
}
```

`ruleset-wet.golden.json` must be exactly:

```json
{
  "name": "idp-wet",
  "target": "branch",
  "enforcement": "active",
  "bypass_actors": [
    {
      "actor_id": 2,
      "actor_type": "Integration",
      "bypass_mode": "always"
    }
  ],
  "conditions": {
    "ref_name": {
      "include": [
        "refs/heads/wet"
      ],
      "exclude": []
    }
  },
  "rules": [
    {
      "type": "deletion"
    },
    {
      "type": "non_fast_forward"
    },
    {
      "type": "update",
      "parameters": {
        "update_allows_fetch_and_merge": false
      }
    }
  ]
}
```

`environment-approval.golden.json` must be exactly:

```json
{
  "deployment_branch_policy": {
    "custom_branch_policies": true,
    "protected_branches": false
  },
  "prevent_self_review": false,
  "reviewers": [
    {
      "id": 42,
      "type": "User"
    }
  ],
  "wait_timer": 0
}
```

`environment-write.golden.json` is the same, except that `"reviewers": []`.

If any file differs, the code is wrong. Fix the code, not the golden.

- [ ] **Step 5: Run the tests without `-update` and see them pass**

Run: `go test ./internal/bootstrap/ && gofmt -l .`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/bootstrap
git commit -m "feat(bootstrap): define desired rulesets and environments with goldens"
```

---

### Task 7: Sealed secrets, the fake GitHub, and the first apply steps

**Files:**
- Create: `internal/bootstrap/seal.go`, `internal/bootstrap/seal_test.go`, `internal/bootstrap/fake_github_test.go`, `internal/bootstrap/bootstrapper.go`, `internal/bootstrap/bootstrapper_test.go`
- Modify: `go.mod`, `go.sum` (add `golang.org/x/crypto v0.57.0`)

**Interfaces:**
- Consumes: `ghapi` (Task 3), `Config` (Task 5), and the constants (Task 6).
- Produces:
  - `func Seal(publicKeyB64, plaintext string) (string, error)`
  - `type Bootstrapper struct { API *ghapi.Client; Cfg Config; Log io.Writer }`
  - `func (b *Bootstrapper) Apply(ctx context.Context) error`. Its steps in this
    task are org workflow permissions, repo, and wet branch. Tasks 8 and 9 append
    more.
  - Unexported helpers: `(b *Bootstrapper) repoPath() string` (=
    `/repos/<org>/<repo>`), `repoFullName() string`, `logf(format, args...)`,
    the type `orgWorkflowPermissions` with `orgWorkflowPermissionsPath()`, the var
    `wantOrgWorkflowPermissions`, and `orgAdminHint(err error) error`.
  - Test-only:
    - `newFakeGitHub(t *testing.T, org string) (*fakeGitHub, *ghapi.Client)`;
    - fields `writes []string` (each `"METHOD /path"`),
      `objects map[string]any`, `rulesets map[int64]map[string]any` and
      `forbidden map[string]bool`;
    - package vars `testPub`, `testPriv *[32]byte`.

- [ ] **Step 1: Add the dependency**

Run: `go get golang.org/x/crypto@v0.57.0`
Expected: `go.mod` gains `require golang.org/x/crypto v0.57.0`.

- [ ] **Step 2: Write the failing seal test `internal/bootstrap/seal_test.go`**

```go
package bootstrap

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/nacl/box"
)

func TestSealRoundTrip(t *testing.T) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := Seal(base64.StdEncoding.EncodeToString(pub[:]), "s3cret-value")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		t.Fatal(err)
	}
	opened, ok := box.OpenAnonymous(nil, raw, pub, priv)
	if !ok || string(opened) != "s3cret-value" {
		t.Fatalf("OpenAnonymous = %q, %v", opened, ok)
	}
}

func TestSealRejectsBadKeys(t *testing.T) {
	for _, key := range []string{"not base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := Seal(key, "x"); err == nil {
			t.Errorf("Seal(%q) succeeded, want error", key)
		}
	}
}
```

- [ ] **Step 3: Run it, see it fail, implement `internal/bootstrap/seal.go`, and see it pass**

Run: `go test ./internal/bootstrap/ -run TestSeal`
Expected first: FAIL with `undefined: Seal`.

```go
package bootstrap

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

// Seal encrypts a value for GitHub's secrets API: a libsodium sealed box with
// the repository's (or environment's) public key, base64-encoded.
func Seal(publicKeyB64, plaintext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("public key is %d bytes, want 32", len(raw))
	}
	var key [32]byte
	copy(key[:], raw)
	sealed, err := box.SealAnonymous(nil, []byte(plaintext), &key, rand.Reader)
	if err != nil {
		return "", fmt.Errorf("seal: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sealed), nil
}
```

Run again: `go test ./internal/bootstrap/ -run TestSeal`
Expected: `ok`.

- [ ] **Step 4: Write the fake GitHub `internal/bootstrap/fake_github_test.go`**

```go
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
```

- [ ] **Step 5: Write the failing tests `internal/bootstrap/bootstrapper_test.go`**

```go
package bootstrap

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestApplyCreatesOrgSettingsRepoAndWetBranch(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	var log bytes.Buffer
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: &log}

	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"PUT /orgs/acme/actions/permissions/workflow",
		"POST /orgs/acme/repos",
		"POST /repos/acme/idp-claims/git/trees",
		"POST /repos/acme/idp-claims/git/commits",
		"POST /repos/acme/idp-claims/git/refs",
	} {
		if !slices.Contains(fake.writes, want) {
			t.Errorf("missing write %q in %v", want, fake.writes)
		}
	}
	perms := fake.objects["/orgs/acme/actions/permissions/workflow"].(map[string]any)
	if perms["default_workflow_permissions"] != "read" || perms["can_approve_pull_request_reviews"] != false {
		t.Errorf("org workflow permissions = %v, want read-only and no PR approval", perms)
	}
	repo := fake.objects["/repos/acme/idp-claims"].(map[string]any)
	if repo["visibility"] != "public" {
		t.Errorf("repo visibility = %v, want public", repo["visibility"])
	}
	if _, ok := fake.objects["/repos/acme/idp-claims/git/ref/heads/wet"]; !ok {
		t.Error("wet branch was not created")
	}
	if !strings.Contains(log.String(), "created repo acme/idp-claims") {
		t.Errorf("log = %q", log.String())
	}
}

func TestApplyExplainsMissingAdminOrgScope(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	fake.forbidden["/orgs/acme/actions/permissions/workflow"] = true
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}

	err := b.Apply(context.Background())
	if err == nil || !strings.Contains(err.Error(), "gh auth refresh -s admin:org") {
		t.Fatalf("err = %v, want a hint to add the admin:org scope", err)
	}
}
```

- [ ] **Step 6: Run them and see them fail**

Run: `go test ./internal/bootstrap/`
Expected: FAIL to compile, with `undefined: Bootstrapper`.

- [ ] **Step 7: Implement `internal/bootstrap/bootstrapper.go`**

```go
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// Bootstrapper applies or checks the org-level setup the IDP depends on.
type Bootstrapper struct {
	API *ghapi.Client
	Cfg Config
	Log io.Writer // one line per change
}

// Apply makes the org match the desired state. It is safe to run any number of
// times: a second run against an unchanged org performs no writes.
func (b *Bootstrapper) Apply(ctx context.Context) error {
	steps := []func(context.Context) error{
		b.ensureOrgWorkflowPermissions,
		b.ensureRepo,
		b.ensureWetBranch,
	}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			return err
		}
	}
	return nil
}

type orgWorkflowPermissions struct {
	Default    string `json:"default_workflow_permissions"`
	CanApprove bool   `json:"can_approve_pull_request_reviews"`
}

// Read-only defaults. Letting Actions create PRs is deferred to Phase 4
// because GitHub couples it with approving PRs (spec §9.2).
var wantOrgWorkflowPermissions = orgWorkflowPermissions{Default: "read", CanApprove: false}

func (b *Bootstrapper) orgWorkflowPermissionsPath() string {
	return "/orgs/" + b.Cfg.Org + "/actions/permissions/workflow"
}

func (b *Bootstrapper) ensureOrgWorkflowPermissions(ctx context.Context) error {
	var cur orgWorkflowPermissions
	if err := b.API.Get(ctx, b.orgWorkflowPermissionsPath(), &cur); err != nil {
		return orgAdminHint(err)
	}
	if cur == wantOrgWorkflowPermissions {
		return nil
	}
	body := map[string]any{
		"default_workflow_permissions":     wantOrgWorkflowPermissions.Default,
		"can_approve_pull_request_reviews": wantOrgWorkflowPermissions.CanApprove,
	}
	if err := b.API.Put(ctx, b.orgWorkflowPermissionsPath(), body, nil); err != nil {
		return orgAdminHint(err)
	}
	b.logf("updated org %s workflow permissions to read-only", b.Cfg.Org)
	return nil
}

// orgAdminHint explains the usual cause of 401/403/404 on org settings: a gh
// token without the admin:org scope.
func orgAdminHint(err error) error {
	var apiErr *ghapi.APIError
	denied := errors.As(err, &apiErr) && (apiErr.Status == http.StatusForbidden || apiErr.Status == http.StatusUnauthorized)
	if denied || errors.Is(err, ghapi.ErrNotFound) {
		return fmt.Errorf("%w (the token needs the admin:org scope: run `gh auth refresh -s admin:org`)", err)
	}
	return err
}

func (b *Bootstrapper) ensureRepo(ctx context.Context) error {
	err := b.API.Get(ctx, b.repoPath(), nil)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ghapi.ErrNotFound) {
		return err
	}
	body := map[string]any{
		"name":        b.Cfg.ClaimsRepo,
		"visibility":  "public",
		"auto_init":   true,
		"description": "IDP claims: desired state on main, rendered output on wet",
	}
	if err := b.API.Post(ctx, "/orgs/"+b.Cfg.Org+"/repos", body, nil); err != nil {
		return err
	}
	b.logf("created repo %s", b.repoFullName())
	return nil
}

const wetReadme = "# wet\n\nRendered output and encrypted state. Written only by the reconcile workflow; never edit by hand.\n"

// ensureWetBranch creates wet as an orphan branch (no shared history with main).
func (b *Bootstrapper) ensureWetBranch(ctx context.Context) error {
	err := b.API.Get(ctx, b.repoPath()+"/git/ref/heads/"+WetBranch, nil)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ghapi.ErrNotFound) {
		return err
	}
	var tree, commit struct {
		SHA string `json:"sha"`
	}
	treeBody := map[string]any{"tree": []map[string]any{
		{"path": "README.md", "mode": "100644", "type": "blob", "content": wetReadme},
	}}
	if err := b.API.Post(ctx, b.repoPath()+"/git/trees", treeBody, &tree); err != nil {
		return err
	}
	commitBody := map[string]any{"message": "chore: initialize wet branch", "tree": tree.SHA, "parents": []string{}}
	if err := b.API.Post(ctx, b.repoPath()+"/git/commits", commitBody, &commit); err != nil {
		return err
	}
	refBody := map[string]any{"ref": "refs/heads/" + WetBranch, "sha": commit.SHA}
	if err := b.API.Post(ctx, b.repoPath()+"/git/refs", refBody, nil); err != nil {
		return err
	}
	b.logf("created branch %s in %s", WetBranch, b.repoFullName())
	return nil
}

func (b *Bootstrapper) repoPath() string     { return "/repos/" + b.repoFullName() }
func (b *Bootstrapper) repoFullName() string { return b.Cfg.Org + "/" + b.Cfg.ClaimsRepo }

func (b *Bootstrapper) logf(format string, args ...any) {
	fmt.Fprintf(b.Log, format+"\n", args...)
}
```

- [ ] **Step 8: Run the tests and see them pass**

Run: `go test ./internal/bootstrap/ && go vet ./... && gofmt -l .`
Expected: `ok`, with no vet or gofmt output.

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum internal/bootstrap
git commit -m "feat(bootstrap): apply org workflow defaults, claims repo and wet branch"
```

---

### Task 8: Rulesets and environments (apply + drift)

**Files:**
- Create: `internal/bootstrap/drift.go`
- Modify: `internal/bootstrap/bootstrapper.go` (add two steps to `Apply`, plus `ensureRulesets` and `ensureEnvironments`), `internal/bootstrap/bootstrapper_test.go`

**Interfaces:**
- Consumes: `Mismatches`, `normalize`, `asList` (Task 4); `MainRuleset`,
  `WetRuleset`, `Environments`, `putBody` (Task 6); the fake (Task 7).
- Produces (unexported, used by Check in Task 10):
  - `func rulesetView(r map[string]any) map[string]any`
  - `(b *Bootstrapper) rulesetIDs(ctx) (map[string]int64, error)`
  - `(b *Bootstrapper) rulesetDrift(ctx, id int64, want Ruleset) ([]string, error)`
  - `(b *Bootstrapper) environmentDrift(ctx, want Environment) ([]string, error)`.
    It returns `[]string{"missing"}` for a 404.
  - `type branchPolicy struct { ID int64; Name string }` and
    `(b *Bootstrapper) branchPolicies(ctx, env string) ([]branchPolicy, error)`

- [ ] **Step 1: Write the failing tests (append to `internal/bootstrap/bootstrapper_test.go`)**

```go
func TestApplyCreatesRulesetsAndEnvironments(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}

	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.rulesetByName("idp-main") == nil || fake.rulesetByName("idp-wet") == nil {
		t.Fatalf("rulesets = %v, want idp-main and idp-wet", fake.rulesets)
	}
	wet := fake.rulesetByName("idp-wet")
	actor := asList(wet["bypass_actors"])[0].(map[string]any)
	if actor["actor_id"] != float64(2) || actor["actor_type"] != "Integration" {
		t.Errorf("idp-wet bypass = %v, want the writer app (id 2)", actor)
	}
	approval := fake.objects["/repos/acme/idp-claims/environments/idp-approval"].(map[string]any)
	if len(asList(approval["protection_rules"])) != 1 {
		t.Errorf("idp-approval protection rules = %v, want one required_reviewers rule", approval["protection_rules"])
	}
	for _, env := range []string{"idp-approval", "idp-write"} {
		policies := asList(fake.objects["/repos/acme/idp-claims/environments/"+env+"/deployment-branch-policies"])
		if len(policies) != 1 || policies[0].(map[string]any)["name"] != "main" {
			t.Errorf("%s branch policies = %v, want only main", env, policies)
		}
	}
}

func TestApplyRepairsRulesetDrift(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	// Someone adds a bypass actor to idp-main by hand.
	main := fake.rulesetByName("idp-main")
	main["bypass_actors"] = []any{map[string]any{"actor_id": float64(9), "actor_type": "User", "bypass_mode": "always"}}
	fake.writes = nil

	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	want := "PUT /repos/acme/idp-claims/rulesets/" + strconv.FormatInt(int64(main["id"].(int64)), 10)
	if !slices.Equal(fake.writes, []string{want}) {
		t.Errorf("writes = %v, want only %q", fake.writes, want)
	}
}

func TestApplyRemovesExtraBranchPolicy(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	path := "/repos/acme/idp-claims/environments/idp-write/deployment-branch-policies"
	fake.objects[path] = append(asList(fake.objects[path]), map[string]any{"id": float64(999), "name": "dev"})
	fake.writes = nil

	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(fake.writes, "DELETE "+path+"/999") {
		t.Errorf("writes = %v, want the dev policy deleted", fake.writes)
	}
}

func TestRulesetDriftIgnoresGitHubExtras(t *testing.T) {
	desired, err := normalize(MainRuleset())
	if err != nil {
		t.Fatal(err)
	}
	live := decode(t, `{
	  "id": 7, "name": "idp-main", "target": "branch", "source_type": "Repository", "source": "acme/idp-claims",
	  "enforcement": "active", "node_id": "RRS_x", "created_at": "2026-10-08T00:00:00Z",
	  "_links": {"self": {"href": "https://api.github.com/x"}},
	  "conditions": {"ref_name": {"include": ["~DEFAULT_BRANCH"], "exclude": []}},
	  "rules": [
	    {"type": "required_status_checks", "parameters": {"strict_required_status_checks_policy": true, "do_not_enforce_on_create": false, "required_status_checks": [{"context": "idp-gate", "integration_id": null}]}},
	    {"type": "pull_request", "parameters": {"allowed_merge_methods": ["merge", "squash", "rebase"], "automatic_copilot_code_review_enabled": false, "dismiss_stale_reviews_on_push": false, "require_code_owner_review": false, "require_last_push_approval": false, "required_approving_review_count": 0, "required_review_thread_resolution": false, "required_reviewers": []}},
	    {"type": "non_fast_forward"},
	    {"type": "deletion"}
	  ]
	}`).(map[string]any)

	if got := Mismatches(rulesetView(desired.(map[string]any)), rulesetView(live)); len(got) != 0 {
		t.Errorf("drift = %v, want none (extra fields, omitted bypass_actors and rule order must not count)", got)
	}
}
```

Also add `"strconv"` to the imports of `bootstrapper_test.go`.

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/bootstrap/`
Expected: FAIL to compile, with `undefined: rulesetView`.

- [ ] **Step 3: Implement `internal/bootstrap/drift.go`**

```go
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// rulesetView keeps the fields bootstrap owns and indexes rules by type, so
// GitHub's rule ordering and extra fields never count as drift. A rule someone
// removed does count; an extra rule someone added (stricter) does not.
func rulesetView(r map[string]any) map[string]any {
	rules := map[string]any{}
	for _, raw := range asList(r["rules"]) {
		if rule, ok := raw.(map[string]any); ok {
			if t, ok := rule["type"].(string); ok {
				rules[t] = rule
			}
		}
	}
	return map[string]any{
		"name":          r["name"],
		"target":        r["target"],
		"enforcement":   r["enforcement"],
		"bypass_actors": asList(r["bypass_actors"]),
		"conditions":    r["conditions"],
		"rules":         rules,
	}
}

func (b *Bootstrapper) rulesetIDs(ctx context.Context) (map[string]int64, error) {
	var list []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := b.API.Get(ctx, b.repoPath()+"/rulesets", &list); err != nil {
		return nil, err
	}
	ids := make(map[string]int64, len(list))
	for _, r := range list {
		ids[r.Name] = r.ID
	}
	return ids, nil
}

func (b *Bootstrapper) rulesetDrift(ctx context.Context, id int64, want Ruleset) ([]string, error) {
	var live map[string]any
	if err := b.API.Get(ctx, fmt.Sprintf("%s/rulesets/%d", b.repoPath(), id), &live); err != nil {
		return nil, err
	}
	desired, err := normalize(want)
	if err != nil {
		return nil, err
	}
	return Mismatches(rulesetView(desired.(map[string]any)), rulesetView(live)), nil
}

type envView struct {
	ReviewerIDs          []int64  `json:"reviewer_ids"`
	ProtectedBranches    bool     `json:"protected_branches"`
	CustomBranchPolicies bool     `json:"custom_branch_policies"`
	Branches             []string `json:"branches"`
}

func (e Environment) view() envView {
	ids := append([]int64{}, e.ReviewerIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return envView{ReviewerIDs: ids, ProtectedBranches: false, CustomBranchPolicies: true, Branches: []string{e.Branch}}
}

type branchPolicy struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// branchPolicies lists an environment's deployment branch policies. GitHub
// answers 404 when the environment has no custom policies yet.
func (b *Bootstrapper) branchPolicies(ctx context.Context, env string) ([]branchPolicy, error) {
	var resp struct {
		BranchPolicies []branchPolicy `json:"branch_policies"`
	}
	err := b.API.Get(ctx, b.repoPath()+"/environments/"+env+"/deployment-branch-policies", &resp)
	if errors.Is(err, ghapi.ErrNotFound) {
		return nil, nil
	}
	return resp.BranchPolicies, err
}

// environmentDrift compares the live environment with want. A missing
// environment is reported as the single path "missing".
func (b *Bootstrapper) environmentDrift(ctx context.Context, want Environment) ([]string, error) {
	var live struct {
		ProtectionRules []struct {
			Type      string `json:"type"`
			Reviewers []struct {
				Reviewer struct {
					ID int64 `json:"id"`
				} `json:"reviewer"`
			} `json:"reviewers"`
		} `json:"protection_rules"`
		DeploymentBranchPolicy *struct {
			ProtectedBranches    bool `json:"protected_branches"`
			CustomBranchPolicies bool `json:"custom_branch_policies"`
		} `json:"deployment_branch_policy"`
	}
	err := b.API.Get(ctx, b.repoPath()+"/environments/"+want.Name, &live)
	if errors.Is(err, ghapi.ErrNotFound) {
		return []string{"missing"}, nil
	}
	if err != nil {
		return nil, err
	}
	got := envView{ReviewerIDs: []int64{}, Branches: []string{}}
	for _, rule := range live.ProtectionRules {
		if rule.Type != "required_reviewers" {
			continue
		}
		for _, r := range rule.Reviewers {
			got.ReviewerIDs = append(got.ReviewerIDs, r.Reviewer.ID)
		}
	}
	sort.Slice(got.ReviewerIDs, func(i, j int) bool { return got.ReviewerIDs[i] < got.ReviewerIDs[j] })
	if live.DeploymentBranchPolicy != nil {
		got.ProtectedBranches = live.DeploymentBranchPolicy.ProtectedBranches
		got.CustomBranchPolicies = live.DeploymentBranchPolicy.CustomBranchPolicies
	}
	policies, err := b.branchPolicies(ctx, want.Name)
	if err != nil {
		return nil, err
	}
	for _, p := range policies {
		got.Branches = append(got.Branches, p.Name)
	}
	sort.Strings(got.Branches)
	desired, err := normalize(want.view())
	if err != nil {
		return nil, err
	}
	actual, err := normalize(got)
	if err != nil {
		return nil, err
	}
	return Mismatches(desired, actual), nil
}
```

- [ ] **Step 4: Add the steps to `internal/bootstrap/bootstrapper.go`**

Replace the `steps` slice in `Apply` with:

```go
	steps := []func(context.Context) error{
		b.ensureOrgWorkflowPermissions,
		b.ensureRepo,
		b.ensureWetBranch,
		b.ensureRulesets,
		b.ensureEnvironments,
	}
```

Append to the file:

```go
func (b *Bootstrapper) ensureRulesets(ctx context.Context) error {
	ids, err := b.rulesetIDs(ctx)
	if err != nil {
		return err
	}
	for _, want := range []Ruleset{MainRuleset(), WetRuleset(b.Cfg.Writer.ID)} {
		id, found := ids[want.Name]
		if !found {
			if err := b.API.Post(ctx, b.repoPath()+"/rulesets", want, nil); err != nil {
				return err
			}
			b.logf("created ruleset %s", want.Name)
			continue
		}
		drift, err := b.rulesetDrift(ctx, id, want)
		if err != nil {
			return err
		}
		if len(drift) == 0 {
			continue
		}
		if err := b.API.Put(ctx, fmt.Sprintf("%s/rulesets/%d", b.repoPath(), id), want, nil); err != nil {
			return err
		}
		b.logf("updated ruleset %s (drift at %v)", want.Name, drift)
	}
	return nil
}

func (b *Bootstrapper) ensureEnvironments(ctx context.Context) error {
	for _, env := range Environments(b.Cfg) {
		drift, err := b.environmentDrift(ctx, env)
		if err != nil {
			return err
		}
		if len(drift) == 0 {
			continue
		}
		if err := b.API.Put(ctx, b.repoPath()+"/environments/"+env.Name, env.putBody(), nil); err != nil {
			return err
		}
		if err := b.ensureBranchPolicy(ctx, env); err != nil {
			return err
		}
		b.logf("configured environment %s (drift at %v)", env.Name, drift)
	}
	return nil
}

// ensureBranchPolicy leaves exactly env.Branch as the environment's only deployment branch.
func (b *Bootstrapper) ensureBranchPolicy(ctx context.Context, env Environment) error {
	policies, err := b.branchPolicies(ctx, env.Name)
	if err != nil {
		return err
	}
	base := b.repoPath() + "/environments/" + env.Name + "/deployment-branch-policies"
	present := false
	for _, p := range policies {
		if p.Name == env.Branch {
			present = true
			continue
		}
		if err := b.API.Delete(ctx, fmt.Sprintf("%s/%d", base, p.ID)); err != nil {
			return err
		}
	}
	if present {
		return nil
	}
	return b.API.Post(ctx, base, map[string]any{"name": env.Branch, "type": "branch"}, nil)
}
```

- [ ] **Step 5: Run the tests and see them pass**

Run: `go test ./internal/bootstrap/ && go vet ./... && gofmt -l .`
Expected: `ok`.

`TestApplyRepairsRulesetDrift` reads `main["id"].(int64)`. That works because
the fake stores the id it assigned as `int64`.

- [ ] **Step 6: Commit**

```bash
git add internal/bootstrap
git commit -m "feat(bootstrap): apply rulesets and environments with drift repair"
```

---

### Task 9: Secrets, variables and full idempotency

**Files:**
- Modify: `internal/bootstrap/bootstrapper.go`, `internal/bootstrap/bootstrapper_test.go`

**Interfaces:**
- Consumes: `Seal` (Task 7), and the constants (Task 6).
- Produces (unexported, used by Check in Task 10):
  - `type secretSpec struct { scope, name, value string }` with `label() string`
  - `(b *Bootstrapper) secretSpecs() []secretSpec` and `secretsBase(scope string) string`
  - `type variableSpec struct { scope, name, value string }` with `label() string`
  - `(b *Bootstrapper) variableSpecs() []variableSpec` and `variablesBase(scope string) string`

- [ ] **Step 1: Write the failing tests (append to `internal/bootstrap/bootstrapper_test.go`)**

```go
func TestApplyFromScratchWritesInOrder(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}

	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"PUT /orgs/acme/actions/permissions/workflow",
		"POST /orgs/acme/repos",
		"POST /repos/acme/idp-claims/git/trees",
		"POST /repos/acme/idp-claims/git/commits",
		"POST /repos/acme/idp-claims/git/refs",
		"POST /repos/acme/idp-claims/rulesets",
		"POST /repos/acme/idp-claims/rulesets",
		"PUT /repos/acme/idp-claims/environments/idp-approval",
		"POST /repos/acme/idp-claims/environments/idp-approval/deployment-branch-policies",
		"PUT /repos/acme/idp-claims/environments/idp-write",
		"POST /repos/acme/idp-claims/environments/idp-write/deployment-branch-policies",
		"PUT /repos/acme/idp-claims/actions/secrets/IDP_READER_PRIVATE_KEY",
		"PUT /repos/acme/idp-claims/actions/secrets/IDP_STATE_PASSPHRASE",
		"PUT /repos/acme/idp-claims/environments/idp-write/secrets/IDP_WRITER_PRIVATE_KEY",
		"POST /repos/acme/idp-claims/actions/variables",
		"POST /repos/acme/idp-claims/environments/idp-write/variables",
	}
	if !slices.Equal(fake.writes, want) {
		t.Errorf("writes =\n%s\nwant\n%s", strings.Join(fake.writes, "\n"), strings.Join(want, "\n"))
	}
}

func TestApplyTwiceIsIdempotent(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	var log bytes.Buffer
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: &log}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	fake.writes = nil
	log.Reset()

	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if len(fake.writes) != 0 {
		t.Errorf("second apply wrote %v, want nothing", fake.writes)
	}
	if !strings.Contains(log.String(), "kept existing secret IDP_STATE_PASSPHRASE") {
		t.Errorf("log = %q, want it to say existing secrets were kept (they cannot be compared)", log.String())
	}
}

func TestApplyStoresSealedPassphraseAndWriterKeyInIdpWrite(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	cfg := validConfig()
	b := &Bootstrapper{API: api, Cfg: cfg, Log: io.Discard}
	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"/repos/acme/idp-claims/actions/secrets/IDP_STATE_PASSPHRASE":                 cfg.Passphrase,
		"/repos/acme/idp-claims/environments/idp-write/secrets/IDP_WRITER_PRIVATE_KEY": cfg.Writer.PrivateKey,
	} {
		stored, ok := fake.objects[path].(map[string]any)
		if !ok {
			t.Fatalf("%s was not stored", path)
		}
		raw, err := base64.StdEncoding.DecodeString(stored["encrypted_value"].(string))
		if err != nil {
			t.Fatal(err)
		}
		opened, ok := box.OpenAnonymous(nil, raw, testPub, testPriv)
		if !ok || string(opened) != want {
			t.Errorf("%s decrypts to %q, want %q", path, opened, want)
		}
	}
	if _, ok := fake.objects["/repos/acme/idp-claims/actions/secrets/IDP_WRITER_PRIVATE_KEY"]; ok {
		t.Error("the writer key must never be a repo-level secret (spec §7.3)")
	}
}

func TestApplyUpdatesChangedVariable(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	b.Cfg.Reader.ClientID = "Iv-reader-rotated"
	fake.writes = nil

	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	want := []string{"PATCH /repos/acme/idp-claims/actions/variables/IDP_READER_CLIENT_ID"}
	if !slices.Equal(fake.writes, want) {
		t.Errorf("writes = %v, want %v", fake.writes, want)
	}
}
```

Add `"encoding/base64"` and `"golang.org/x/crypto/nacl/box"` to the test imports.

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/bootstrap/`
Expected: FAIL. `TestApplyFromScratchWritesInOrder` is missing the secret and
variable writes, and `TestApplyTwiceIsIdempotent` has no "kept" log.

- [ ] **Step 3: Implement it in `internal/bootstrap/bootstrapper.go`**

Replace the `steps` slice in `Apply` with the final list:

```go
	steps := []func(context.Context) error{
		b.ensureOrgWorkflowPermissions,
		b.ensureRepo,
		b.ensureWetBranch,
		b.ensureRulesets,
		b.ensureEnvironments,
		b.ensureSecrets,
		b.ensureVariables,
	}
```

Append to the file:

```go
// secretSpec is one secret; an empty scope means repository-level,
// otherwise the scope is an environment name.
type secretSpec struct{ scope, name, value string }

func (s secretSpec) label() string {
	if s.scope == "" {
		return s.name
	}
	return s.scope + "/" + s.name
}

func (b *Bootstrapper) secretSpecs() []secretSpec {
	return []secretSpec{
		{scope: "", name: SecretReaderKey, value: b.Cfg.Reader.PrivateKey},
		{scope: "", name: SecretPassphrase, value: b.Cfg.Passphrase},
		{scope: EnvWrite, name: SecretWriterKey, value: b.Cfg.Writer.PrivateKey},
	}
}

func (b *Bootstrapper) secretsBase(scope string) string {
	if scope == "" {
		return b.repoPath() + "/actions/secrets"
	}
	return b.repoPath() + "/environments/" + scope + "/secrets"
}

// ensureSecrets creates missing secrets. Secret values are write-only, so an
// existing secret is kept as is; rotating means deleting it in GitHub first.
func (b *Bootstrapper) ensureSecrets(ctx context.Context) error {
	for _, s := range b.secretSpecs() {
		base := b.secretsBase(s.scope)
		err := b.API.Get(ctx, base+"/"+s.name, nil)
		if err == nil {
			b.logf("kept existing secret %s (values are write-only; delete it in GitHub to rotate)", s.label())
			continue
		}
		if !errors.Is(err, ghapi.ErrNotFound) {
			return err
		}
		var key struct {
			KeyID string `json:"key_id"`
			Key   string `json:"key"`
		}
		if err := b.API.Get(ctx, base+"/public-key", &key); err != nil {
			return err
		}
		sealed, err := Seal(key.Key, s.value)
		if err != nil {
			return err
		}
		if err := b.API.Put(ctx, base+"/"+s.name, map[string]any{"encrypted_value": sealed, "key_id": key.KeyID}, nil); err != nil {
			return err
		}
		b.logf("created secret %s", s.label())
	}
	return nil
}

// variableSpec is one Actions variable; scope works as in secretSpec.
type variableSpec struct{ scope, name, value string }

func (v variableSpec) label() string {
	if v.scope == "" {
		return v.name
	}
	return v.scope + "/" + v.name
}

func (b *Bootstrapper) variableSpecs() []variableSpec {
	return []variableSpec{
		{scope: "", name: VarReaderClient, value: b.Cfg.Reader.ClientID},
		{scope: EnvWrite, name: VarWriterClient, value: b.Cfg.Writer.ClientID},
	}
}

func (b *Bootstrapper) variablesBase(scope string) string {
	if scope == "" {
		return b.repoPath() + "/actions/variables"
	}
	return b.repoPath() + "/environments/" + scope + "/variables"
}

func (b *Bootstrapper) ensureVariables(ctx context.Context) error {
	for _, v := range b.variableSpecs() {
		base := b.variablesBase(v.scope)
		var cur struct {
			Value string `json:"value"`
		}
		err := b.API.Get(ctx, base+"/"+v.name, &cur)
		switch {
		case err == nil && cur.Value == v.value:
			continue
		case err == nil:
			if err := b.API.Patch(ctx, base+"/"+v.name, map[string]any{"name": v.name, "value": v.value}, nil); err != nil {
				return err
			}
			b.logf("updated variable %s", v.label())
		case errors.Is(err, ghapi.ErrNotFound):
			if err := b.API.Post(ctx, base, map[string]any{"name": v.name, "value": v.value}, nil); err != nil {
				return err
			}
			b.logf("created variable %s", v.label())
		default:
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run all the bootstrap tests and see them pass**

Run: `go test ./internal/bootstrap/ && go vet ./... && gofmt -l .`
Expected: `ok`. All tests from Tasks 4 to 9 pass.

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap
git commit -m "feat(bootstrap): apply sealed secrets and variables idempotently"
```

---

### Task 10: `Check` (read-only drift report)

**Files:**
- Create: `internal/bootstrap/check.go`, `internal/bootstrap/check_test.go`

**Interfaces:**
- Consumes: everything unexported from Tasks 7 to 9.
- Produces:
  - `type Finding struct { Resource, Problem string }` with `String() string`
    (`"<resource>: <problem>"`).
  - `func (b *Bootstrapper) Check(ctx context.Context) ([]Finding, error)`. It
    never writes.

- [ ] **Step 1: Write the failing tests `internal/bootstrap/check_test.go`**

```go
package bootstrap

import (
	"context"
	"io"
	"strings"
	"testing"
)

func installBoth(fake *fakeGitHub) {
	fake.objects["/orgs/acme/installations"] = map[string]any{"total_count": 2, "installations": []any{
		map[string]any{"app_slug": "acme-reader"}, map[string]any{"app_slug": "acme-writer"},
	}}
}

func TestCheckCleanOrgHasNoFindings(t *testing.T) {
	fake, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	installBoth(fake)
	fake.writes = nil

	findings, err := b.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
	if len(fake.writes) != 0 {
		t.Errorf("check wrote %v; it must be read-only", fake.writes)
	}
}

func TestCheckReportsDrift(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*fakeGitHub)
		install bool
		want    string
	}{
		{name: "app not installed", mutate: func(*fakeGitHub) {}, install: false, want: "app acme-writer: not installed on org acme"},
		{name: "bypass added to main", install: true, mutate: func(f *fakeGitHub) {
			f.rulesetByName("idp-main")["bypass_actors"] = []any{map[string]any{"actor_id": float64(9), "actor_type": "User", "bypass_mode": "always"}}
		}, want: "ruleset idp-main: drift at"},
		{name: "secret deleted", install: true, mutate: func(f *fakeGitHub) {
			delete(f.objects, "/repos/acme/idp-claims/actions/secrets/IDP_STATE_PASSPHRASE")
		}, want: "secret IDP_STATE_PASSPHRASE: missing"},
		{name: "private repo", install: true, mutate: func(f *fakeGitHub) {
			f.objects["/repos/acme/idp-claims"].(map[string]any)["visibility"] = "private"
		}, want: `repo acme/idp-claims: visibility is "private", want public`},
		{name: "reviewer removed", install: true, mutate: func(f *fakeGitHub) {
			f.objects["/repos/acme/idp-claims/environments/idp-approval"].(map[string]any)["protection_rules"] = []any{}
		}, want: "environment idp-approval: drift at"},
		{name: "variable changed", install: true, mutate: func(f *fakeGitHub) {
			f.objects["/repos/acme/idp-claims/actions/variables/IDP_READER_CLIENT_ID"] = map[string]any{"name": "IDP_READER_CLIENT_ID", "value": "other"}
		}, want: "variable IDP_READER_CLIENT_ID: value differs"},
		{name: "org defaults loosened", install: true, mutate: func(f *fakeGitHub) {
			f.objects["/orgs/acme/actions/permissions/workflow"] = map[string]any{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": true}
		}, want: "org acme: workflow permissions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, api := newFakeGitHub(t, "acme")
			b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
			if err := b.Apply(context.Background()); err != nil {
				t.Fatal(err)
			}
			if tt.install {
				installBoth(fake)
			}
			tt.mutate(fake)

			findings, err := b.Check(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			for _, f := range findings {
				lines = append(lines, f.String())
			}
			if !strings.Contains(strings.Join(lines, "\n"), tt.want) {
				t.Errorf("findings =\n%s\nwant one containing %q", strings.Join(lines, "\n"), tt.want)
			}
		})
	}
}

func TestCheckMissingRepoStopsEarly(t *testing.T) {
	_, api := newFakeGitHub(t, "acme")
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}

	findings, err := b.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var sawRepo bool
	for _, f := range findings {
		if f.Resource == "repo acme/idp-claims" && f.Problem == "missing" {
			sawRepo = true
		}
		if strings.HasPrefix(f.Resource, "ruleset") || strings.HasPrefix(f.Resource, "secret") {
			t.Errorf("unexpected finding %v after a missing repo", f)
		}
	}
	if !sawRepo {
		t.Errorf("findings = %v, want repo missing", findings)
	}
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/bootstrap/`
Expected: FAIL to compile, with `b.Check undefined`.

- [ ] **Step 3: Implement `internal/bootstrap/check.go`**

```go
package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// Finding is one way the live org differs from what bootstrap would create.
type Finding struct {
	Resource string
	Problem  string
}

func (f Finding) String() string { return f.Resource + ": " + f.Problem }

type findings []Finding

func (f *findings) add(resource, format string, args ...any) {
	*f = append(*f, Finding{Resource: resource, Problem: fmt.Sprintf(format, args...)})
}

// Check reports drift without changing anything (spec §9.2, run by drift §6.4).
func (b *Bootstrapper) Check(ctx context.Context) ([]Finding, error) {
	var f findings
	if err := b.checkOrg(ctx, &f); err != nil {
		return nil, err
	}
	exists, err := b.checkRepo(ctx, &f)
	if err != nil {
		return nil, err
	}
	if !exists {
		return f, nil
	}
	for _, step := range []func(context.Context, *findings) error{
		b.checkWetBranch, b.checkRulesets, b.checkEnvironments, b.checkSecrets, b.checkVariables,
	} {
		if err := step(ctx, &f); err != nil {
			return nil, err
		}
	}
	return f, nil
}

func (b *Bootstrapper) checkOrg(ctx context.Context, f *findings) error {
	var perms orgWorkflowPermissions
	if err := b.API.Get(ctx, b.orgWorkflowPermissionsPath(), &perms); err != nil {
		return orgAdminHint(err)
	}
	if perms != wantOrgWorkflowPermissions {
		f.add("org "+b.Cfg.Org, "workflow permissions are %+v, want %+v", perms, wantOrgWorkflowPermissions)
	}
	var inst struct {
		Installations []struct {
			AppSlug string `json:"app_slug"`
		} `json:"installations"`
	}
	if err := b.API.Get(ctx, "/orgs/"+b.Cfg.Org+"/installations", &inst); err != nil {
		return orgAdminHint(err)
	}
	installed := map[string]bool{}
	for _, i := range inst.Installations {
		installed[i.AppSlug] = true
	}
	for _, slug := range []string{b.Cfg.Reader.Slug, b.Cfg.Writer.Slug} {
		if !installed[slug] {
			f.add("app "+slug, "not installed on org %s", b.Cfg.Org)
		}
	}
	return nil
}

func (b *Bootstrapper) checkRepo(ctx context.Context, f *findings) (bool, error) {
	var repo struct {
		Visibility string `json:"visibility"`
	}
	err := b.API.Get(ctx, b.repoPath(), &repo)
	if errors.Is(err, ghapi.ErrNotFound) {
		f.add("repo "+b.repoFullName(), "missing")
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if repo.Visibility != "public" {
		f.add("repo "+b.repoFullName(), "visibility is %q, want public (Free-plan rulesets need public repos)", repo.Visibility)
	}
	return true, nil
}

func (b *Bootstrapper) checkWetBranch(ctx context.Context, f *findings) error {
	err := b.API.Get(ctx, b.repoPath()+"/git/ref/heads/"+WetBranch, nil)
	if errors.Is(err, ghapi.ErrNotFound) {
		f.add("branch "+WetBranch, "missing")
		return nil
	}
	return err
}

func (b *Bootstrapper) checkRulesets(ctx context.Context, f *findings) error {
	ids, err := b.rulesetIDs(ctx)
	if err != nil {
		return err
	}
	for _, want := range []Ruleset{MainRuleset(), WetRuleset(b.Cfg.Writer.ID)} {
		id, found := ids[want.Name]
		if !found {
			f.add("ruleset "+want.Name, "missing")
			continue
		}
		drift, err := b.rulesetDrift(ctx, id, want)
		if err != nil {
			return err
		}
		if len(drift) > 0 {
			f.add("ruleset "+want.Name, "drift at %v", drift)
		}
	}
	return nil
}

func (b *Bootstrapper) checkEnvironments(ctx context.Context, f *findings) error {
	for _, env := range Environments(b.Cfg) {
		drift, err := b.environmentDrift(ctx, env)
		if err != nil {
			return err
		}
		if len(drift) > 0 {
			f.add("environment "+env.Name, "drift at %v", drift)
		}
	}
	return nil
}

func (b *Bootstrapper) checkSecrets(ctx context.Context, f *findings) error {
	for _, s := range b.secretSpecs() {
		err := b.API.Get(ctx, b.secretsBase(s.scope)+"/"+s.name, nil)
		if errors.Is(err, ghapi.ErrNotFound) {
			f.add("secret "+s.label(), "missing")
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Bootstrapper) checkVariables(ctx context.Context, f *findings) error {
	for _, v := range b.variableSpecs() {
		var cur struct {
			Value string `json:"value"`
		}
		err := b.API.Get(ctx, b.variablesBase(v.scope)+"/"+v.name, &cur)
		if errors.Is(err, ghapi.ErrNotFound) {
			f.add("variable "+v.label(), "missing")
			continue
		}
		if err != nil {
			return err
		}
		if cur.Value != v.value {
			f.add("variable "+v.label(), "value differs")
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the tests and see them pass**

Run: `go test ./internal/bootstrap/ && go vet ./... && gofmt -l .`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap
git commit -m "feat(bootstrap): add read-only check for org protections"
```

---

### Task 11: App manifests and the local manifest flow

**Files:**
- Create: `bootstrap/apps/apps.go`, `bootstrap/apps/reader.json`, `bootstrap/apps/writer.json`, `internal/bootstrap/appflow.go`, `internal/bootstrap/appflow_test.go`

**Interfaces:**
- Consumes: `ghapi` (Task 3), and `AppCredentials` and `LoadAppCredentials` (Task 5).
- Produces:
  - `var apps.FS embed.FS` in package `github.com/jellalshadows-idp/idp-engine/bootstrap/apps`
  - `type AppRole string` with `RoleReader = "reader"` and `RoleWriter = "writer"`
  - `func Manifest(org string, role AppRole, callback string) (map[string]any, error)`.
    It errors when `org + "-" + role` is longer than 34 characters, or the role is
    unknown.
  - `type AppFlow struct { Org string; Role AppRole; API *ghapi.Client; OutDir string; Log io.Writer; GitHubWeb string }`
  - `func (f *AppFlow) Run(ctx context.Context, ln net.Listener) (AppCredentials, error)`.
    It writes `<OutDir>/<slug>.json` and `<OutDir>/<slug>.pem`, both mode 0600.

- [ ] **Step 1: Write the manifests**

`bootstrap/apps/writer.json`:
```json
{
  "description": "IDP writer: applies rendered GitHub stacks and commits to the wet branch. Its key lives only in the idp-write environment.",
  "public": false,
  "default_permissions": {
    "administration": "write",
    "contents": "write",
    "environments": "write",
    "members": "write",
    "metadata": "read",
    "workflows": "write"
  },
  "default_events": []
}
```

`bootstrap/apps/reader.json`:
```json
{
  "description": "IDP reader: plans on pull requests and checks drift. Read-only by design.",
  "public": false,
  "default_permissions": {
    "administration": "read",
    "contents": "read",
    "environments": "read",
    "members": "read",
    "metadata": "read",
    "organization_administration": "read",
    "secrets": "read"
  },
  "default_events": []
}
```

`bootstrap/apps/apps.go`:
```go
// Package apps embeds the GitHub App manifests, so the Apps' permissions are
// reviewable code (spec §9.2).
package apps

import "embed"

// FS holds reader.json and writer.json.
//
//go:embed *.json
var FS embed.FS
```

- [ ] **Step 2: Write the failing tests `internal/bootstrap/appflow_test.go`**

```go
package bootstrap

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

func TestManifestWriter(t *testing.T) {
	m, err := Manifest("acme", RoleWriter, "http://127.0.0.1:1234/callback")
	if err != nil {
		t.Fatal(err)
	}
	if m["name"] != "acme-writer" || m["url"] != "https://github.com/acme" || m["redirect_url"] != "http://127.0.0.1:1234/callback" {
		t.Errorf("manifest identity = %v / %v / %v", m["name"], m["url"], m["redirect_url"])
	}
	if m["public"] != false {
		t.Errorf("public = %v, want false", m["public"])
	}
	perms := m["default_permissions"].(map[string]any)
	if perms["workflows"] != "write" {
		t.Errorf("workflows = %v; features write .github/workflows, so the writer needs write", perms["workflows"])
	}
}

func TestManifestReaderIsReadOnly(t *testing.T) {
	m, err := Manifest("acme", RoleReader, "cb")
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range m["default_permissions"].(map[string]any) {
		if v != "read" {
			t.Errorf("reader permission %s = %v, want read", k, v)
		}
	}
}

func TestManifestRejects(t *testing.T) {
	if _, err := Manifest("a-very-long-organization-name", RoleWriter, "cb"); err == nil || !strings.Contains(err.Error(), "34") {
		t.Errorf("long name: err = %v, want the 34-character limit", err)
	}
	if _, err := Manifest("acme", AppRole("admin"), "cb"); err == nil {
		t.Error("unknown role: want error")
	}
}

func httpGet(t *testing.T, url string, wantStatus int) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s = %d, want %d: %s", url, resp.StatusCode, wantStatus, body)
	}
	return string(body)
}

func startFlow(t *testing.T, apiURL, outDir string) (base string, done chan error, creds *AppCredentials, cancel context.CancelFunc) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	flow := &AppFlow{Org: "acme", Role: RoleWriter, API: ghapi.New(apiURL, ""), OutDir: outDir, Log: io.Discard, GitHubWeb: "https://github.example"}
	done = make(chan error, 1)
	creds = &AppCredentials{}
	go func() {
		c, err := flow.Run(ctx, ln)
		*creds = c
		done <- err
	}()
	return "http://" + ln.Addr().String(), done, creds, cancel
}

func TestAppFlowCreatesAndSavesApp(t *testing.T) {
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/app-manifests/the-code/conversions" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, `{"id": 77, "client_id": "Iv23abc", "slug": "acme-writer", "pem": "-----BEGIN RSA PRIVATE KEY-----\nx\n-----END RSA PRIVATE KEY-----\n"}`)
	}))
	defer gh.Close()
	out := t.TempDir()
	base, done, creds, cancel := startFlow(t, gh.URL, out)
	defer cancel()

	form := httpGet(t, base+"/", http.StatusOK)
	if !strings.Contains(form, "https://github.example/organizations/acme/settings/apps/new?state=") {
		t.Fatalf("form does not post to the org's new-app page:\n%s", form)
	}
	state := regexp.MustCompile(`state=([A-Za-z0-9_-]+)`).FindStringSubmatch(form)[1]
	httpGet(t, base+"/callback?code=the-code&state="+state, http.StatusOK)

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if creds.ID != 77 || creds.Slug != "acme-writer" || !strings.Contains(creds.PrivateKey, "BEGIN RSA") {
		t.Errorf("creds = %+v", *creds)
	}
	loaded, err := LoadAppCredentials(filepath.Join(out, "acme-writer.json"), true)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ClientID != "Iv23abc" || loaded.PrivateKey != creds.PrivateKey {
		t.Errorf("saved credentials = %+v", loaded)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(out, "acme-writer.pem"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("pem mode = %v, want 0600", info.Mode().Perm())
		}
	}
}

func TestAppFlowRejectsWrongState(t *testing.T) {
	base, done, _, cancel := startFlow(t, "http://127.0.0.1:1", t.TempDir())
	httpGet(t, base+"/callback?code=x&state=forged", http.StatusBadRequest)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("want the context error after cancel, got nil")
	}
}
```

- [ ] **Step 3: Run them and see them fail**

Run: `go test ./internal/bootstrap/`
Expected: FAIL to compile, with `undefined: Manifest`.

- [ ] **Step 4: Implement `internal/bootstrap/appflow.go`**

```go
package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/jellalshadows-idp/idp-engine/bootstrap/apps"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// AppRole selects the manifest and the App name suffix.
type AppRole string

const (
	RoleReader AppRole = "reader"
	RoleWriter AppRole = "writer"
)

// maxAppName is GitHub's limit for App names, which must also be unique across GitHub.
const maxAppName = 34

// Manifest returns the GitHub App manifest for org and role, named <org>-<role>.
func Manifest(org string, role AppRole, callback string) (map[string]any, error) {
	if role != RoleReader && role != RoleWriter {
		return nil, fmt.Errorf("unknown app role %q (want reader or writer)", role)
	}
	name := org + "-" + string(role)
	if len(name) > maxAppName {
		return nil, fmt.Errorf("app name %q is %d characters; GitHub allows at most %d", name, len(name), maxAppName)
	}
	raw, err := apps.FS.ReadFile(string(role) + ".json")
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	m["name"] = name
	m["url"] = "https://github.com/" + org
	m["redirect_url"] = callback
	return m, nil
}

// AppFlow runs GitHub's manifest flow through a local callback server: the
// person confirms creation in the browser, GitHub redirects back with a code,
// and the code is exchanged for the App's id, client id and private key.
type AppFlow struct {
	Org       string
	Role      AppRole
	API       *ghapi.Client // unauthenticated is fine for the conversion call
	OutDir    string
	Log       io.Writer
	GitHubWeb string // https://github.com; overridable in tests
}

var formTmpl = template.Must(template.New("form").Parse(`<!doctype html>
<html><body>
<form id="f" method="post" action="{{.Action}}">
<input type="hidden" name="manifest" value="{{.Manifest}}">
<noscript><button type="submit">Create GitHub App</button></noscript>
</form>
<script>document.getElementById("f").submit()</script>
</body></html>`))

type flowResult struct {
	creds AppCredentials
	err   error
}

// Run serves the form on ln and blocks until GitHub calls back or ctx ends.
func (f *AppFlow) Run(ctx context.Context, ln net.Listener) (AppCredentials, error) {
	state, err := randomState()
	if err != nil {
		return AppCredentials{}, err
	}
	manifest, err := Manifest(f.Org, f.Role, "http://"+ln.Addr().String()+"/callback")
	if err != nil {
		return AppCredentials{}, err
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return AppCredentials{}, err
	}
	action := fmt.Sprintf("%s/organizations/%s/settings/apps/new?state=%s", f.GitHubWeb, url.PathEscape(f.Org), url.QueryEscape(state))

	result := make(chan flowResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_ = formTmpl.Execute(w, map[string]string{"Action": action, "Manifest": string(manifestJSON)})
	})
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		creds, err := f.convert(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		} else {
			fmt.Fprintf(w, "App %s created. You can close this tab.\n", creds.Slug)
		}
		select {
		case result <- flowResult{creds: creds, err: err}:
		default: // a duplicate callback (browser refresh) is ignored
		}
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()
	fmt.Fprintf(f.Log, "Open http://%s/ in your browser to create %s-%s\n", ln.Addr(), f.Org, f.Role)

	select {
	case <-ctx.Done():
		return AppCredentials{}, ctx.Err()
	case res := <-result:
		if res.err != nil {
			return AppCredentials{}, res.err
		}
		return res.creds, f.save(res.creds)
	}
}

func (f *AppFlow) convert(ctx context.Context, code string) (AppCredentials, error) {
	if code == "" {
		return AppCredentials{}, errors.New("callback without code")
	}
	var resp struct {
		ID       int64  `json:"id"`
		ClientID string `json:"client_id"`
		Slug     string `json:"slug"`
		PEM      string `json:"pem"`
	}
	if err := f.API.Post(ctx, "/app-manifests/"+url.PathEscape(code)+"/conversions", nil, &resp); err != nil {
		return AppCredentials{}, err
	}
	return AppCredentials{ID: resp.ID, ClientID: resp.ClientID, Slug: resp.Slug, PrivateKey: resp.PEM}, nil
}

// save writes <slug>.json (no key) and <slug>.pem, both readable only by the owner.
func (f *AppFlow) save(c AppCredentials) error {
	if err := os.MkdirAll(f.OutDir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.OutDir, c.Slug+".pem"), []byte(c.PrivateKey), 0o600); err != nil {
		return err
	}
	meta, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(f.OutDir, c.Slug+".json"), append(meta, '\n'), 0o600)
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

- [ ] **Step 5: Run the tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok` for every package.

- [ ] **Step 6: Commit**

```bash
git add bootstrap internal/bootstrap
git commit -m "feat(bootstrap): add app manifests and local manifest flow"
```

---

### Task 12: Wire `idp bootstrap app|apply|check`, write the runbook, merge

**Files:**
- Create: `internal/cli/bootstrap.go`, `internal/cli/bootstrap_test.go`, `docs/runbooks/bootstrap.md`
- Modify: `internal/cli/cli.go` (usage plus the `bootstrap` case)

**Interfaces:**
- Consumes: the `bootstrap` package (Tasks 5 to 11) and `ghapi` (Task 3).
- Produces the CLI contract:
  - `idp bootstrap app --org ORG --role reader|writer [--out-dir DIR] [--listen ADDR]`
  - `idp bootstrap apply --org --claims-repo --approver --reader FILE --writer FILE --passphrase-file FILE`
  - `idp bootstrap check --org --claims-repo --approver --reader FILE --writer FILE`
  - Exit codes: `apply` returns 0 or 1; `check` returns 0 (no drift) or 1
    (findings); usage errors return 2.
  - The token comes from `GH_TOKEN`, falling back to `GITHUB_TOKEN`.
    `IDP_GITHUB_API` overrides the API base URL.

- [ ] **Step 1: Write the failing tests `internal/cli/bootstrap_test.go`**

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestBootstrapUsageErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "no subcommand", args: []string{"bootstrap"}, wantStderr: "idp bootstrap apply"},
		{name: "unknown subcommand", args: []string{"bootstrap", "nope"}, wantStderr: `unknown subcommand "nope"`},
		{name: "apply lists every missing flag", args: []string{"bootstrap", "apply"}, wantStderr: "missing --org, --claims-repo, --approver, --reader, --writer, --passphrase-file\n"},
		{name: "check does not need a passphrase", args: []string{"bootstrap", "check"}, wantStderr: "missing --org, --claims-repo, --approver, --reader, --writer\n"},
		{name: "token is required", args: []string{"bootstrap", "check", "--org", "o", "--claims-repo", "r", "--approver", "a", "--reader", "r.json", "--writer", "w.json"}, wantStderr: "set GH_TOKEN"},
		{name: "app needs a valid role", args: []string{"bootstrap", "app", "--org", "o", "--role", "admin"}, wantStderr: "--role reader|writer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr, noEnv)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2 (stderr %q)", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestHelpListsBootstrap(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Run([]string{"help"}, &stdout, &stderr, noEnv)
	if !strings.Contains(stdout.String(), "bootstrap") {
		t.Errorf("help = %q, want it to list bootstrap", stdout.String())
	}
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/cli/`
Expected: FAIL. `bootstrap` is an unknown command, so stderr has the wrong text.

- [ ] **Step 3: Update `internal/cli/cli.go`**

Replace the `usage` const with:

```go
const usage = `idp — IDP engine CLI

Usage:
  idp <command> [flags]

Commands:
  bootstrap   Create or verify the protections an org needs before the IDP runs
  help        Show this help
`
```

In `Run`, add this case before `default`:

```go
	case "bootstrap":
		return runBootstrap(args[1:], stdout, stderr, env)
```

- [ ] **Step 4: Implement `internal/cli/bootstrap.go`**

```go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/bootstrap"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

const bootstrapUsage = `Usage:
  idp bootstrap app   --org ORG --role reader|writer [--out-dir DIR] [--listen ADDR]
  idp bootstrap apply --org ORG --claims-repo REPO --approver LOGIN --reader FILE --writer FILE --passphrase-file FILE
  idp bootstrap check --org ORG --claims-repo REPO --approver LOGIN --reader FILE --writer FILE

FILE for --reader/--writer is the <slug>.json written by "idp bootstrap app".
apply and check read a token from GH_TOKEN or GITHUB_TOKEN (e.g. GH_TOKEN=$(gh auth token)).
`

func runBootstrap(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, bootstrapUsage)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	switch args[0] {
	case "app":
		return runBootstrapApp(ctx, args[1:], stdout, stderr, env)
	case "apply":
		return runBootstrapRepo(ctx, "apply", args[1:], stdout, stderr, env)
	case "check":
		return runBootstrapRepo(ctx, "check", args[1:], stdout, stderr, env)
	default:
		fmt.Fprintf(stderr, "idp bootstrap: unknown subcommand %q\n\n%s", args[0], bootstrapUsage)
		return 2
	}
}

func apiBase(env Env) string {
	if v := env("IDP_GITHUB_API"); v != "" {
		return v
	}
	return ghapi.DefaultBaseURL
}

func token(env Env) string {
	if v := env("GH_TOKEN"); v != "" {
		return v
	}
	return env("GITHUB_TOKEN")
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "idp:", err)
	return 1
}

func runBootstrapRepo(ctx context.Context, mode string, args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp bootstrap "+mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	org := fs.String("org", "", "GitHub organization")
	repo := fs.String("claims-repo", "", "claims repository name")
	approver := fs.String("approver", "", "login of the platform admin who approves runs")
	readerFile := fs.String("reader", "", "reader app <slug>.json")
	writerFile := fs.String("writer", "", "writer app <slug>.json")
	passFile := fs.String("passphrase-file", "", "file with the OpenTofu state passphrase (apply only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	required := []struct{ flag, value string }{
		{"--org", *org}, {"--claims-repo", *repo}, {"--approver", *approver},
		{"--reader", *readerFile}, {"--writer", *writerFile},
	}
	if mode == "apply" {
		required = append(required, struct{ flag, value string }{"--passphrase-file", *passFile})
	}
	var missing []string
	for _, r := range required {
		if r.value == "" {
			missing = append(missing, r.flag)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(stderr, "idp bootstrap %s: missing %s\n", mode, strings.Join(missing, ", "))
		return 2
	}
	tok := token(env)
	if tok == "" {
		fmt.Fprintln(stderr, "idp bootstrap: set GH_TOKEN or GITHUB_TOKEN (e.g. GH_TOKEN=$(gh auth token))")
		return 2
	}

	api := ghapi.New(apiBase(env), tok)
	withKeys := mode == "apply"
	cfg := bootstrap.Config{Org: *org, ClaimsRepo: *repo}
	var err error
	if cfg.Reader, err = bootstrap.LoadAppCredentials(*readerFile, withKeys); err != nil {
		return fail(stderr, err)
	}
	if cfg.Writer, err = bootstrap.LoadAppCredentials(*writerFile, withKeys); err != nil {
		return fail(stderr, err)
	}
	if cfg.ApproverID, err = bootstrap.UserID(ctx, api, *approver); err != nil {
		return fail(stderr, err)
	}

	if mode == "apply" {
		if cfg.Passphrase, err = bootstrap.ReadPassphrase(*passFile); err != nil {
			return fail(stderr, err)
		}
		if err := cfg.ValidateApply(); err != nil {
			return fail(stderr, err)
		}
		b := &bootstrap.Bootstrapper{API: api, Cfg: cfg, Log: stdout}
		if err := b.Apply(ctx); err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintln(stdout, "bootstrap apply: done")
		return 0
	}

	if err := cfg.ValidateCheck(); err != nil {
		return fail(stderr, err)
	}
	b := &bootstrap.Bootstrapper{API: api, Cfg: cfg, Log: stdout}
	found, err := b.Check(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	for _, f := range found {
		fmt.Fprintln(stdout, f)
	}
	if len(found) > 0 {
		fmt.Fprintf(stdout, "bootstrap check: %d finding(s)\n", len(found))
		return 1
	}
	fmt.Fprintln(stdout, "bootstrap check: no drift")
	return 0
}

func runBootstrapApp(ctx context.Context, args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp bootstrap app", flag.ContinueOnError)
	fs.SetOutput(stderr)
	org := fs.String("org", "", "GitHub organization that will own the App")
	role := fs.String("role", "", "reader or writer")
	outDir := fs.String("out-dir", "", "where to write <slug>.json and <slug>.pem (default ~/.idp/apps)")
	listen := fs.String("listen", "127.0.0.1:0", "local address for the callback server")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *org == "" || (*role != string(bootstrap.RoleReader) && *role != string(bootstrap.RoleWriter)) {
		fmt.Fprintln(stderr, "idp bootstrap app: --org and --role reader|writer are required")
		return 2
	}
	dir := *outDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fail(stderr, err)
		}
		dir = filepath.Join(home, ".idp", "apps")
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return fail(stderr, err)
	}
	flow := &bootstrap.AppFlow{
		Org: *org, Role: bootstrap.AppRole(*role), API: ghapi.New(apiBase(env), ""),
		OutDir: dir, Log: stdout, GitHubWeb: "https://github.com",
	}
	creds, err := flow.Run(ctx, ln)
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "Created %s (id %d). Credentials in %s\n", creds.Slug, creds.ID, dir)
	fmt.Fprintf(stdout, "Now install it on %s with access to All repositories: https://github.com/apps/%s/installations/new\n", *org, creds.Slug)
	return 0
}
```

- [ ] **Step 5: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok` for every package.

- [ ] **Step 6: Write `docs/runbooks/bootstrap.md`**

````markdown
# Runbook: bootstrap an org

Bootstrap is deliberately **outside** the IDP: if the claims repo's protections were managed by claims, a PR could disable the checks that guard it (spec §9.2, ADR-0011).

## Prerequisites

- A GitHub **Free** organization you own.
- `gh` logged in as an org owner, **with the `admin:org` scope**:
  `gh auth refresh -h github.com -s admin:org`
- Go 1.26, to run the CLI from a checkout: `go run ./cmd/idp ...`
- A passphrase of at least 16 characters, saved in your password manager **first**. If you lose it, the encrypted state in `wet` can no longer be read.

## 1. Create the two Apps

The names are `<org>-reader` and `<org>-writer`. GitHub App names are unique across GitHub and limited to 34 characters.

```bash
go run ./cmd/idp bootstrap app --org <org> --role reader
go run ./cmd/idp bootstrap app --org <org> --role writer
```

For each one, open the printed `http://127.0.0.1:<port>/` link and confirm **Create GitHub App** on GitHub. The credentials land in `~/.idp/apps/<slug>.json` and `<slug>.pem`. **Never commit them.**

## 2. Install both Apps

Open each printed `https://github.com/apps/<slug>/installations/new` link, choose the org, and select **All repositories**. The writer creates repositories, so it needs org-wide access.

## 3. Apply

```bash
GH_TOKEN="$(gh auth token)" go run ./cmd/idp bootstrap apply \
  --org <org> --claims-repo <claims-repo> --approver <your-login> \
  --reader ~/.idp/apps/<org>-reader.json --writer ~/.idp/apps/<org>-writer.json \
  --passphrase-file <file>
```

Running it again is safe. A second run prints only `kept existing secret …` lines and `bootstrap apply: done`.

## 4. Verify

```bash
GH_TOKEN="$(gh auth token)" go run ./cmd/idp bootstrap check \
  --org <org> --claims-repo <claims-repo> --approver <your-login> \
  --reader ~/.idp/apps/<org>-reader.json --writer ~/.idp/apps/<org>-writer.json
```

Expected: `bootstrap check: no drift` and exit code 0.

## Rotating a secret

Secret values are write-only, so `apply` never overwrites an existing secret. To rotate one:

1. Delete the secret in the repo or environment settings.
2. Run `apply` again.

## Known gaps

- Until Phase 1 adds the `idp-gate` check, no PR can merge into the claims repo. This is expected.
- Letting Actions create PRs (needed by the `release-please` feature) is enabled in Phase 4, not here.
````

- [ ] **Step 7: Commit, push, wait for green CI, and merge** (outward-facing: ask first)

```bash
git add internal/cli docs/runbooks
git commit -m "feat(cli): wire idp bootstrap app, apply and check"
git push
gh pr checks --repo jellalshadows-idp/idp-engine --watch
gh pr merge --repo jellalshadows-idp/idp-engine --squash --delete-branch
git switch main && git pull --ff-only
```
Expected: all checks pass before the merge. After the merge, the CI run on `main`
is green.

---

### Task 13: Create the Apps and bootstrap `idp-claims-e2e` (real run)

**Files:** none in git. The credentials go to `~/.idp/apps/`.

**Interfaces:**
- Consumes: the merged CLI (Task 12) and the orgs (Task 0).
- Produces:
  - Apps `jellalshadows-idp-reader` and `jellalshadows-idp-writer`,
    installed on all repositories.
  - Repo `jellalshadows-idp/idp-claims-e2e`, bootstrapped, with
    `check` clean.

- [ ] **Step 1: The owner creates the e2e passphrase**

The owner generates a passphrase of 24+ characters in their password manager,
saves it there as "idp e2e state", and writes it to a local file **outside any
repo**, for example `~/.idp/e2e.pass`.

- [ ] **Step 2: Create both Apps** (the owner confirms in the browser)

Run: `go run ./cmd/idp bootstrap app --org jellalshadows-idp --role reader`
Then: `go run ./cmd/idp bootstrap app --org jellalshadows-idp --role writer`
Expected: `Created jellalshadows-idp-reader (id N)` and the same for the
writer. The Glob tool on `~/.idp/apps/*` lists 4 files (two `.json`, two `.pem`).

- [ ] **Step 3: Install both Apps** (the owner, in the browser)

Open the two printed install links and select **All repositories**.

- [ ] **Step 4: Check before applying (expect findings)**

Run: `GH_TOKEN="$(gh auth token)" go run ./cmd/idp bootstrap check --org jellalshadows-idp --claims-repo idp-claims-e2e --approver jellalshadows --reader ~/.idp/apps/jellalshadows-idp-reader.json --writer ~/.idp/apps/jellalshadows-idp-writer.json`
Expected: exit code 1, with `repo jellalshadows-idp/idp-claims-e2e: missing`
and **no** `app … not installed` lines. If those lines appear, Step 3 is incomplete.

- [ ] **Step 5: Apply** (outward-facing: ask first)

Run: `GH_TOKEN="$(gh auth token)" go run ./cmd/idp bootstrap apply --org jellalshadows-idp --claims-repo idp-claims-e2e --approver jellalshadows --reader ~/.idp/apps/jellalshadows-idp-reader.json --writer ~/.idp/apps/jellalshadows-idp-writer.json --passphrase-file ~/.idp/e2e.pass`
Expected: lines like `created repo …`, `created ruleset idp-main`, and so on, ending
with `bootstrap apply: done`.

- [ ] **Step 6: Prove idempotency and a clean check against real GitHub**

Run the Step 5 command again.
Expected: only `kept existing secret …` lines and `bootstrap apply: done`. There
must be no `created`, `updated` or `configured` lines. If there are, the real API
shape differs from the fake. Capture the live JSON with
`gh api repos/jellalshadows-idp/idp-claims-e2e/rulesets/<id>`, add it as
a case to `TestRulesetDriftIgnoresGitHubExtras`, fix the code through a PR, and
re-run.

Run the Step 4 command again.
Expected: `bootstrap check: no drift`, exit code 0.

- [ ] **Step 7: Capture the live ruleset shape (owner token)** (added by ruling R12)

For each ruleset id listed by `gh api repos/jellalshadows-idp/idp-claims-e2e/rulesets --jq '.[].id'`,
run `gh api repos/jellalshadows-idp/idp-claims-e2e/rulesets/<id>` and save the
output in the scratch notes for ADR-0013. Record one fact explicitly: for
`idp-main`, whose bypass list is empty, does GitHub return `"bypass_actors": []`
or omit the field, even for an admin token? This decides how `check` must treat
a missing `bypass_actors` (final-review finding I3).

---

### Task 14: Spike S1 — encrypted state round-trip through Git (THROWAWAY)

**Files** (local folder `C:\Users\Usuario\idp-spike`, a new git repo):
- Create: `README.md`, `state/main.tf`, `.github/workflows/spike-state.yaml`

**Interfaces:**
- Consumes: the org (Task 0).
- Produces: job-summary numbers `state_bytes` and `apply_seconds`, plus three
  verdicts (round-trip, tamper rejected, plaintext refused), for ADR-0013.

- [ ] **Step 0: Prerequisite** — Task 13 has created both Apps; Task 15 Step 8 also needs the reader's credentials.

- [ ] **Step 1: Create the spike repo locally**

```bash
git init -b main C:/Users/Usuario/idp-spike
```

`README.md`:
```markdown
# idp-spike (THROWAWAY)

Phase 0 spikes for [idp-engine](https://github.com/jellalshadows-idp/idp-engine). The results live in ADR-0013; this repo is archived once that ADR merges. Do not reuse this code.
```

- [ ] **Step 2: Write `state/main.tf`**

```hcl
terraform {
  required_version = "1.12.6"

  # The code says THAT state and plans are encrypted; TF_ENCRYPTION says HOW (spec §7.2).
  encryption {
    state {
      enforced = true
    }
    plan {
      enforced = true
    }
  }
}

resource "terraform_data" "marker" {
  input = "phase-0-spike"
}
```

- [ ] **Step 3: Write `.github/workflows/spike-state.yaml`**

```yaml
name: spike-state

on: workflow_dispatch

permissions: {}

jobs:
  write:
    runs-on: ubuntu-24.04
    permissions:
      contents: write
    env:
      TF_ENCRYPTION: ${{ secrets.SPIKE_TF_ENCRYPTION }}
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4 # v2.0.2
        with:
          tofu_version: 1.12.6
          tofu_wrapper: false
      - name: apply and commit the encrypted state to a branch
        working-directory: state
        run: |
          start=$(date +%s)
          tofu init -input=false
          tofu apply -input=false -auto-approve
          echo "apply_seconds=$(( $(date +%s) - start ))" >> "$GITHUB_STEP_SUMMARY"
          echo "state_bytes=$(wc -c < terraform.tfstate)" >> "$GITHUB_STEP_SUMMARY"
          python3 -c "import json;d=json.load(open('terraform.tfstate'));print('state_keys=' + ','.join(sorted(d)))" >> "$GITHUB_STEP_SUMMARY"
          git config user.name "idp-spike"
          git config user.email "idp-spike@users.noreply.github.com"
          git switch -c spike-wet
          git add -f terraform.tfstate
          git commit -m "chore: spike encrypted state"
          git push -f origin spike-wet

  read:
    needs: write
    runs-on: ubuntu-24.04
    permissions:
      contents: read
    env:
      TF_ENCRYPTION: ${{ secrets.SPIKE_TF_ENCRYPTION }}
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: spike-wet
          persist-credentials: false
      - uses: opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4 # v2.0.2
        with:
          tofu_version: 1.12.6
          tofu_wrapper: false
      - name: a fresh job reads the committed state and sees no changes
        working-directory: state
        run: |
          tofu init -input=false
          tofu plan -input=false -detailed-exitcode
          echo "roundtrip=ok (plan exit 0)" >> "$GITHUB_STEP_SUMMARY"
      - name: tampered ciphertext is rejected
        working-directory: state
        run: |
          cp terraform.tfstate /tmp/good.tfstate
          python3 - <<'EOF'
          import json
          d = json.load(open("terraform.tfstate"))
          data = d["encrypted_data"]
          flipped = ("B" if data[40] == "A" else "A")
          d["encrypted_data"] = data[:40] + flipped + data[41:]
          json.dump(d, open("terraform.tfstate", "w"))
          EOF
          if tofu plan -input=false; then
            echo "::error::tampered state was accepted"; exit 1
          fi
          echo "tamper=rejected" >> "$GITHUB_STEP_SUMMARY"
          cp /tmp/good.tfstate terraform.tfstate
      - name: without TF_ENCRYPTION, writing plaintext is refused
        run: |
          mkdir /tmp/fresh && cp state/main.tf /tmp/fresh/
          cd /tmp/fresh
          tofu init -input=false
          if TF_ENCRYPTION="" tofu apply -input=false -auto-approve; then
            echo "::error::enforced=true did not stop a plaintext write"; exit 1
          fi
          echo "plaintext=refused" >> "$GITHUB_STEP_SUMMARY"
```

If `encrypted_data` is not the ciphertext field name in OpenTofu 1.12.6, the
tamper step fails with a `KeyError`. In that case, read `state_keys` from the
`write` job summary, use the field that holds the ciphertext, and re-run.

- [ ] **Step 4: Create the spike repo, set the secret, push** (outward-facing: ask first)

```bash
cd C:/Users/Usuario/idp-spike
git add . && git commit -m "chore: add phase 0 spike s1"
gh repo create jellalshadows-idp/idp-spike --public --source . --remote origin --push --description "THROWAWAY phase 0 spikes for idp-engine"
```

Set `SPIKE_TF_ENCRYPTION`. Write this HCL to a local temp file outside the repo,
with a fresh 24+ character spike-only passphrase in place of the quoted value, and
pipe it in:

```hcl
key_provider "pbkdf2" "main" {
  passphrase = "spike-only-passphrase-change-me-0001"
}
method "aes_gcm" "main" {
  keys = key_provider.pbkdf2.main
}
state {
  method = method.aes_gcm.main
}
plan {
  method = method.aes_gcm.main
}
```

Run: `gh secret set SPIKE_TF_ENCRYPTION --repo jellalshadows-idp/idp-spike < <that file>`
Then delete the temp file.

- [ ] **Step 5: Run it and collect the results**

Run: `gh workflow run spike-state.yaml --repo jellalshadows-idp/idp-spike`
Then: `gh run watch --repo jellalshadows-idp/idp-spike $(gh run list --repo jellalshadows-idp/idp-spike --workflow spike-state.yaml --limit 1 --json databaseId --jq '.[0].databaseId')`
Expected: both jobs are green, and the summaries show `roundtrip=ok`,
`tamper=rejected` and `plaintext=refused`. Copy `apply_seconds`, `state_bytes`,
`state_keys` and the three verdicts into a scratch note for ADR-0013.

---

### Task 15: Spike S2 — GitHub provider with an App token, timings, pending invite (THROWAWAY)

**Files** (in `idp-spike`):
- Create: `github/main.tf`, `.github/workflows/spike-github.yaml`

**Interfaces:**
- Consumes: the writer App (Task 13) and the spike repo (Task 14).
- Produces, for ADR-0013:
  - apply, plan and destroy seconds for 10 resources;
  - whether the ruleset bypass works (files committed after the ruleset exists);
  - whether `.github/workflows/*` can be written;
  - the pending-invite plan exit code before and after acceptance.

- [ ] **Step 1: Write `github/main.tf`**

```hcl
terraform {
  required_version = "1.12.6"
  required_providers {
    github = {
      source  = "integrations/github"
      version = "6.13.0"
    }
  }
}

variable "writer_app_id" {
  type = number
}

provider "github" {
  owner = "jellalshadows-idp"
  # token: GITHUB_TOKEN env, minted from the writer App
}

resource "github_team" "spike" {
  name    = "spike-team"
  privacy = "closed"
}

# adrian-da-silva is NOT an org member: this sends an org invitation (pending).
resource "github_team_membership" "invitee" {
  team_id  = github_team.spike.id
  username = "adrian-da-silva"
  role     = "member"
}

resource "github_repository" "spike" {
  name                   = "spike-component"
  visibility             = "public"
  auto_init              = true
  delete_branch_on_merge = true
  vulnerability_alerts   = true
  archive_on_destroy     = false
}

resource "github_team_repository" "spike" {
  team_id    = github_team.spike.id
  repository = github_repository.spike.name
  permission = "maintain"
}

resource "github_repository_ruleset" "main" {
  name        = "main"
  repository  = github_repository.spike.name
  target      = "branch"
  enforcement = "active"

  conditions {
    ref_name {
      include = ["~DEFAULT_BRANCH"]
      exclude = []
    }
  }

  bypass_actors {
    actor_id    = var.writer_app_id
    actor_type  = "Integration"
    bypass_mode = "always"
  }

  rules {
    deletion         = true
    non_fast_forward = true
    pull_request {
      required_approving_review_count = 1
    }
  }
}

resource "github_repository_environment" "pro" {
  environment = "pro"
  repository  = github_repository.spike.name
  reviewers {
    teams = [github_team.spike.id]
  }
  deployment_branch_policy {
    protected_branches     = false
    custom_branch_policies = true
  }
  depends_on = [github_team_repository.spike]
}

resource "github_repository_environment_deployment_policy" "pro_main" {
  repository     = github_repository.spike.name
  environment    = github_repository_environment.pro.environment
  branch_pattern = "main"
}

resource "github_actions_environment_variable" "role" {
  repository    = github_repository.spike.name
  environment   = github_repository_environment.pro.environment
  variable_name = "AWS_ROLE_ARN"
  value         = "arn:aws:iam::000000000003:role/spike-component-pro-ci"
}

# Both files are committed to main AFTER the ruleset requires PRs: they only
# succeed through the writer App's bypass.
resource "github_repository_file" "codeowners" {
  repository          = github_repository.spike.name
  branch              = "main"
  file                = ".github/CODEOWNERS"
  content             = "* @jellalshadows-idp/spike-team\n"
  overwrite_on_create = true
  depends_on          = [github_repository_ruleset.main]
}

# Writing under .github/workflows needs the App's Workflows permission.
resource "github_repository_file" "workflow" {
  repository          = github_repository.spike.name
  branch              = "main"
  file                = ".github/workflows/hello.yaml"
  content             = "name: hello\non: workflow_dispatch\npermissions: {}\njobs:\n  hello:\n    runs-on: ubuntu-24.04\n    steps:\n      - run: echo hello\n"
  overwrite_on_create = true
  depends_on          = [github_repository_ruleset.main]
}
```

- [ ] **Step 2: Write `.github/workflows/spike-github.yaml`**

```yaml
name: spike-github

on:
  workflow_dispatch:
    inputs:
      action:
        type: choice
        options: [apply, plan, destroy]
        default: apply

permissions: {}

concurrency:
  group: spike-github
  cancel-in-progress: false

jobs:
  run:
    runs-on: ubuntu-24.04
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0
        id: app
        with:
          client-id: ${{ vars.SPIKE_WRITER_CLIENT_ID }}
          private-key: ${{ secrets.SPIKE_WRITER_PRIVATE_KEY }}
          owner: jellalshadows-idp
      - uses: opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4 # v2.0.2
        with:
          tofu_version: 1.12.6
          tofu_wrapper: false
      - name: restore state from the spike-github-state branch
        run: |
          if git fetch origin spike-github-state; then
            git checkout origin/spike-github-state -- github/terraform.tfstate
          else
            echo "no state yet"
          fi
      - name: tofu
        working-directory: github
        env:
          GITHUB_TOKEN: ${{ steps.app.outputs.token }}
          TF_VAR_writer_app_id: ${{ vars.SPIKE_WRITER_APP_ID }}
          ACTION: ${{ inputs.action }}
        run: |
          tofu init -input=false
          start=$(date +%s)
          set +e
          case "$ACTION" in
            apply)   tofu apply -input=false -auto-approve ;;
            plan)    tofu plan -input=false -detailed-exitcode ;;
            destroy) tofu destroy -input=false -auto-approve ;;
          esac
          code=$?
          set -e
          echo "${ACTION}_seconds=$(( $(date +%s) - start ))" >> "$GITHUB_STEP_SUMMARY"
          echo "${ACTION}_exit=${code}" >> "$GITHUB_STEP_SUMMARY"
          echo "resources_in_state=$(tofu state list 2>/dev/null | wc -l)" >> "$GITHUB_STEP_SUMMARY"
          if [ "$ACTION" != "plan" ] && [ "$code" != "0" ]; then exit "$code"; fi
      - name: save state (spike only; plaintext of spike- resources is acceptable)
        if: always()
        run: |
          [ -f github/terraform.tfstate ] || exit 0
          git config user.name "idp-spike"
          git config user.email "idp-spike@users.noreply.github.com"
          cp github/terraform.tfstate /tmp/state
          git fetch origin spike-github-state || true
          git switch -C spike-github-state
          mkdir -p github && cp /tmp/state github/terraform.tfstate
          git add -f github/terraform.tfstate
          git commit -m "chore: spike github state" || exit 0
          git push -f origin spike-github-state
```

- [ ] **Step 3: Give the spike repo the writer credentials** (outward-facing: ask first)

```bash
cd C:/Users/Usuario/idp-spike
git add . && git commit -m "chore: add phase 0 spike s2" && git push
W=~/.idp/apps/jellalshadows-idp-writer
gh variable set SPIKE_WRITER_CLIENT_ID --repo jellalshadows-idp/idp-spike --body "$(rg -o --no-filename '"client_id":\s*"([^"]+)"' -r '$1' "$W.json")"
gh variable set SPIKE_WRITER_APP_ID --repo jellalshadows-idp/idp-spike --body "$(rg -o --no-filename '"id":\s*(\d+)' -r '$1' "$W.json")"
gh secret set SPIKE_WRITER_PRIVATE_KEY --repo jellalshadows-idp/idp-spike < "$W.pem"
gh variable list --repo jellalshadows-idp/idp-spike
```

- [ ] **Step 4: Apply and measure**

Run: `gh workflow run spike-github.yaml --repo jellalshadows-idp/idp-spike -f action=apply`, then watch it as in Task 14 Step 5.
Expected: green. `resources_in_state=10`, and `apply_seconds` is recorded.
- If the file resources fail with `refusing to allow a GitHub App to create or
  update workflow`, the writer manifest lacks `workflows: write`. That is a Task 11
  bug: record it and fix it via PR.
- If they fail with a ruleset violation, the bypass did not work. Record that
  verdict, because ADR-0009 depends on it.

- [ ] **Step 5: Plan while the invite is pending**

Run the workflow with `-f action=plan`.
Record `plan_exit`: 0 means no diff, 2 means a diff. Also record `plan_seconds`,
which is the refresh time for 10 resources.

- [ ] **Step 6: Accept the invitation as adrian-da-silva, then plan again** (manual)

The owner logs in to github.com as `adrian-da-silva` (for example in a private
window), opens https://github.com/orgs/jellalshadows-idp/invitation, and
accepts. Then run the workflow with `-f action=plan` again and record `plan_exit`.

Decision rule for ADR-0013 and spec §11 risk 1:
- If the pending plan exits 2 **and** the accepted plan exits 0, the diff is
  temporary. Adopt the fallback from spec §11 (members must already be org members,
  checked by validation) and record it.
- If both exit 0, pending invites are harmless. Record "no fallback needed".

- [ ] **Step 7: Destroy and clean up**

Run the workflow with `-f action=destroy`. Record `destroy_seconds`.
Then (outward-facing: ask first):
`gh api --method DELETE /orgs/jellalshadows-idp/members/adrian-da-silva`
Expected: `gh api orgs/jellalshadows-idp/members --jq '.[].login'` no
longer lists `adrian-da-silva`.

- [ ] **Step 8: Reader probe — what the reader App can actually see** (added by ruling R12)

Give the spike repo the reader credentials, the same way Step 3 did for the
writer (outward-facing: ask first):

```bash
R=~/.idp/apps/jellalshadows-idp-reader
gh variable set SPIKE_READER_CLIENT_ID --repo jellalshadows-idp/idp-spike --body "$(rg -o --no-filename '"client_id":\s*"([^"]+)"' -r '$1' "$R.json")"
gh secret set SPIKE_READER_PRIVATE_KEY --repo jellalshadows-idp/idp-spike < "$R.pem"
```

Add `.github/workflows/spike-reader-probe.yaml`:

```yaml
name: spike-reader-probe

on: workflow_dispatch

permissions: {}

jobs:
  probe:
    runs-on: ubuntu-24.04
    permissions: {}
    steps:
      - uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0
        id: app
        with:
          client-id: ${{ vars.SPIKE_READER_CLIENT_ID }}
          private-key: ${{ secrets.SPIKE_READER_PRIVATE_KEY }}
          owner: jellalshadows-idp
      - name: probe what the reader token can read
        env:
          GH_TOKEN: ${{ steps.app.outputs.token }}
          R: jellalshadows-idp/idp-claims-e2e
        run: |
          probe() {
            code=$( (gh api -i "$1" 2>/dev/null || true) | head -n1 | awk '{print $2}')
            echo "| \`$1\` | ${code:-error} |" >> "$GITHUB_STEP_SUMMARY"
          }
          { echo "| endpoint | status |"; echo "|---|---|"; } >> "$GITHUB_STEP_SUMMARY"
          probe "repos/$R"
          probe "repos/$R/rulesets"
          probe "repos/$R/environments/idp-approval"
          probe "repos/$R/environments/idp-approval/deployment-branch-policies"
          probe "repos/$R/environments/idp-approval/secrets"
          probe "repos/$R/environments/idp-write/variables/IDP_WRITER_CLIENT_ID"
          probe "repos/$R/actions/variables/IDP_READER_CLIENT_ID"
          probe "repos/$R/actions/secrets/IDP_STATE_PASSPHRASE"
          probe "orgs/jellalshadows-idp/actions/permissions/workflow"
          probe "orgs/jellalshadows-idp/installations"
          for id in $(gh api "repos/$R/rulesets" --jq '.[].id'); do
            gh api "repos/$R/rulesets/$id" --jq '"- ruleset " + .name + ": bypass_actors present = " + (has("bypass_actors") | tostring)' >> "$GITHUB_STEP_SUMMARY"
          done
```

Commit, push, then run: `gh workflow run spike-reader-probe.yaml --repo jellalshadows-idp/idp-spike`. Watch it as in Task 14 Step 5.

Decision rules, recorded in ADR-0013:
- **Any 403/404 in the table** means the reader manifest lacks a permission. Fix
  `bootstrap/apps/reader.json` through a PR, update the App's permissions in its
  settings, and accept the change on the installation **before Task 17**.
- **`bypass_actors present = false`** confirms final-review finding I3: a reader
  token cannot verify ruleset bypass lists. Phase 1 must decide in an ADR how the
  drift check verifies them before `drift.yaml` exists.

---

### Task 16: Spike S3 — floci multi-account, OIDC/IAM/ECR, replay timings (THROWAWAY)

**Files** (in `idp-spike`):
- Create: `aws/main.tf`, `ci/floci_override.tf.json`, `.github/workflows/spike-floci.yaml`

**Interfaces:**
- Consumes: the spike repo (Task 14).
- Produces, for ADR-0013:
  - `floci_ready_seconds`, `init_cold_seconds`, `init_cached_seconds`,
    `apply_v1_seconds` and `plan_v2_seconds`;
  - the account-isolation verdict;
  - the role ARN and repository URL formats.

- [ ] **Step 1: Write `aws/main.tf`**

```hcl
terraform {
  required_version = "1.12.6"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.66.0"
    }
  }
}

variable "env" {
  type = string
}

variable "image_tag_mutability" {
  type = string
}

# Credentials come from AWS_ACCESS_KEY_ID (= the 12-digit account id) and
# AWS_SECRET_ACCESS_KEY. Endpoints and skip flags come from floci_override.tf.json,
# written by CI (spec §5.6), so this file stays emulator-agnostic.
provider "aws" {
  region = "eu-west-1"
}

resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
}

resource "aws_ecr_repository" "api" {
  name                 = "api"
  image_tag_mutability = var.image_tag_mutability
}

resource "aws_iam_role" "ci" {
  name = "api-${var.env}-ci"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Federated = aws_iam_openid_connect_provider.github.arn }
      Action    = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
          "token.actions.githubusercontent.com:sub" = "repo:jellalshadows-idp/api:environment:${var.env}"
        }
      }
    }]
  })
}

resource "aws_iam_role_policy" "push" {
  name = "ecr-push"
  role = aws_iam_role.ci.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      { Effect = "Allow", Action = ["ecr:GetAuthorizationToken"], Resource = "*" },
      {
        Effect   = "Allow"
        Action   = ["ecr:BatchCheckLayerAvailability", "ecr:CompleteLayerUpload", "ecr:InitiateLayerUpload", "ecr:PutImage", "ecr:UploadLayerPart"]
        Resource = aws_ecr_repository.api.arn
      }
    ]
  })
}

output "role_arn" {
  value = aws_iam_role.ci.arn
}

output "repository_url" {
  value = aws_ecr_repository.api.repository_url
}
```

`ci/floci_override.tf.json` lives outside `aws/`, so the stack itself never
mentions floci. The workflow copies it in, as the real CI will (spec §5.6):

```json
{
  "provider": {
    "aws": {
      "skip_credentials_validation": true,
      "skip_metadata_api_check": true,
      "skip_requesting_account_id": true,
      "endpoints": {
        "iam": "http://localhost:4566",
        "sts": "http://localhost:4566",
        "ecr": "http://localhost:4566"
      }
    }
  }
}
```

- [ ] **Step 2: Write `.github/workflows/spike-floci.yaml`**

```yaml
name: spike-floci

on: workflow_dispatch

permissions: {}

jobs:
  floci:
    runs-on: ubuntu-24.04
    permissions:
      contents: read
    env:
      AWS_SECRET_ACCESS_KEY: test
      AWS_REGION: eu-west-1
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4 # v2.0.2
        with:
          tofu_version: 1.12.6
          tofu_wrapper: false
      - name: start floci (ECR needs the Docker socket, per floci's README)
        run: |
          start=$(date +%s)
          docker run -d --name floci -p 4566:4566 -u root \
            -v /var/run/docker.sock:/var/run/docker.sock \
            floci/floci:2.1.0@sha256:2e2343974a15137a6bda6de5a9e0b16207f97a786cc57ee9c2bf25d14a67b84c
          for _ in $(seq 1 90); do
            code=$(curl -s -o /dev/null -w '%{http_code}' http://localhost:4566/ || true)
            if [ "$code" != "000" ]; then break; fi
            sleep 1
          done
          echo "floci_ready_seconds=$(( $(date +%s) - start ))" >> "$GITHUB_STEP_SUMMARY"
      - name: drop in the CI-owned override (spec §5.6)
        run: cp ci/floci_override.tf.json aws/
      - name: dev account (000000000001) — cold init, apply v1
        working-directory: aws
        env:
          AWS_ACCESS_KEY_ID: "000000000001"
          TF_VAR_env: dev
          TF_VAR_image_tag_mutability: MUTABLE
        run: |
          start=$(date +%s); tofu init -input=false
          echo "init_cold_seconds=$(( $(date +%s) - start ))" >> "$GITHUB_STEP_SUMMARY"
          start=$(date +%s); tofu apply -input=false -auto-approve
          echo "apply_v1_seconds=$(( $(date +%s) - start ))" >> "$GITHUB_STEP_SUMMARY"
          arn=$(tofu output -raw role_arn)
          echo "role_arn=$arn" >> "$GITHUB_STEP_SUMMARY"
          echo "repository_url=$(tofu output -raw repository_url)" >> "$GITHUB_STEP_SUMMARY"
          if [ "$arn" != "arn:aws:iam::000000000001:role/api-dev-ci" ]; then
            echo "::error::role ARN does not carry the 12-digit account id"; exit 1
          fi
      - name: replay — plan v2 on top of v1 (this is what every PR plan does)
        working-directory: aws
        env:
          AWS_ACCESS_KEY_ID: "000000000001"
          TF_VAR_env: dev
          TF_VAR_image_tag_mutability: IMMUTABLE
        run: |
          start=$(date +%s)
          set +e; tofu plan -input=false -detailed-exitcode; code=$?; set -e
          echo "plan_v2_seconds=$(( $(date +%s) - start ))" >> "$GITHUB_STEP_SUMMARY"
          echo "plan_v2_exit=$code (want 2: one in-place update)" >> "$GITHUB_STEP_SUMMARY"
          if [ "$code" != "2" ]; then echo "::error::expected exactly a diff"; exit 1; fi
      - name: pro account (000000000003) is isolated from dev
        env:
          AWS_ACCESS_KEY_ID: "000000000003"
        run: |
          if aws --endpoint-url http://localhost:4566 iam get-role --role-name api-dev-ci; then
            echo "::error::account isolation broken: pro can see dev's role"; exit 1
          fi
          echo "isolation=ok" >> "$GITHUB_STEP_SUMMARY"
      - name: cached init (provider already in the plugin cache)
        env:
          TF_PLUGIN_CACHE_DIR: /tmp/plugin-cache
        run: |
          mkdir -p /tmp/plugin-cache /tmp/a /tmp/b
          cp aws/main.tf /tmp/a/ && cp aws/main.tf /tmp/b/
          (cd /tmp/a && tofu init -input=false >/dev/null)
          start=$(date +%s); (cd /tmp/b && tofu init -input=false >/dev/null)
          echo "init_cached_seconds=$(( $(date +%s) - start ))" >> "$GITHUB_STEP_SUMMARY"
      - name: floci logs on failure
        if: failure()
        run: docker logs floci | tail -n 200
```

- [ ] **Step 3: Push and run**

```bash
cd C:/Users/Usuario/idp-spike
git add . && git commit -m "chore: add phase 0 spike s3" && git push
gh workflow run spike-floci.yaml --repo jellalshadows-idp/idp-spike
```
Watch it as in Task 14 Step 5.
Expected: green, with `isolation=ok`, the role ARN containing `000000000001`, and
every timing recorded.

If a step fails because floci rejects an operation (an IAM, ECR or OIDC fidelity
gap):
1. Record the exact error from the `floci logs` step.
2. Remove only that resource from `aws/main.tf`.
3. Re-run to get the timings.
4. Write the gap into ADR-0013 under spec §11 risk 4.

---

### Task 17: Bootstrap the production claims repo `idp-claims` (real run)

**Files:** none in git.

**Interfaces:**
- Consumes: the CLI (Task 12) and the Apps created in Task 13. It runs after the
  spikes, so if they expose a manifest permission gap, the Apps' permissions are
  updated (and re-approved on the installation) before production depends on them.
- Produces: the repo `jellalshadows-idp/idp-claims` with `check` clean.

- [ ] **Step 1: The owner creates the production passphrase**

As in Task 13 Step 1, but a **different** passphrase, saved as "idp main state",
in the file `~/.idp/main.pass`.

- [ ] **Step 2: Reuse the Apps** (no new Apps: there is a single org)

Confirm `~/.idp/apps/jellalshadows-idp-reader.json` and `…-writer.json` exist from
Task 13. If Task 15 changed the writer manifest, update the App's permissions in
its GitHub settings and accept the change on the installation first.

- [ ] **Step 3: Apply** (outward-facing: ask first)

Run: `GH_TOKEN="$(gh auth token)" go run ./cmd/idp bootstrap apply --org jellalshadows-idp --claims-repo idp-claims --approver jellalshadows --reader ~/.idp/apps/jellalshadows-idp-reader.json --writer ~/.idp/apps/jellalshadows-idp-writer.json --passphrase-file ~/.idp/main.pass`
Expected: it ends with `bootstrap apply: done`.

- [ ] **Step 4: Check**

Run the same command with `check` and without `--passphrase-file`.
Expected: `bootstrap check: no drift`, exit code 0.

---

### Task 18: ADRs, spec risk update, archive the spike

**Files:**
- Create: `docs/adr/0001-edge-triggered-reconciliation-on-actions.md` … `docs/adr/0014-single-org-isolated-by-repo-and-prefix.md`, and `docs/phases/phase-0.md` (execution log: every ledger ruling with cost-if-wrong, every task's review outcome and fix rounds, every deferred minor finding with its triage — written before the SDD workspace is deleted)
- Modify: `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` (§11: add a "Phase 0 result" column), `README.md` (link the ADRs)

**Interfaces:**
- Consumes: the scratch notes from Tasks 14 to 16, and the outcomes of Tasks 13
  and 17.
- Produces: the ADR index that Phase 1's plan starts from.

- [ ] **Step 1: Write ADRs 0001 to 0012 and 0014 with this exact template**

```markdown
# NNNN. <Title>

- Status: Accepted
- Date: 2026-10-08
- Spec: [§<section>](../superpowers/specs/2026-10-08-idp-on-actions-design.md)

## Context

<context>

## Decision

<decision>

## Consequences

<bullets>

## Alternatives considered

<bullets>
```

Content per ADR. Use these sentences as the body. Do not invent new claims.

| # / file slug | Spec | Context | Decision | Consequences | Alternatives |
|---|---|---|---|---|---|
| 0001 `edge-triggered-reconciliation-on-actions` | §1, §6 | Firestartr reconciles with a Kubernetes operator; this project must not require a cluster. | Reconcile on GitHub Actions events: PR, merge, manual dispatch, and a daily drift report. | No continuous reconciliation; drift is reported, never auto-fixed; zero infrastructure to run. | Kubernetes operator (Firestartr): level-triggered, but needs a cluster. |
| 0002 `encrypted-opentofu-state-in-git` | §7.2 | The GitHub stack needs durable state, with no cloud backend available. | Keep the GitHub stack's state in `wet`, encrypted with OpenTofu (PBKDF2 + AES-GCM, `enforced = true`). | Authenticated encryption detects tampering; every commit is a state version; the passphrase must never be lost. | An S3 backend (no real AWS in v1); plaintext state in a public repo. |
| 0003 `wet-branch-and-single-pr` | §3.2 | Firestartr needs two PRs (claims, then hydrate). | Render into a `wet` branch that only the reconcile writes; humans review one PR. | Smaller loop; `wet` history shows exactly what was applied. | Two PRs as in Firestartr; rendering into `main`. |
| 0004 `floci-multi-account-emulation` | §5.6 | AWS is emulated; envs map to accounts. | One floci per job; the 12-digit access key ID selects the account. | Real-looking ARNs; nothing persists between jobs, so stacks are replayed from `wet`. | One floci per env on separate ports: the original design, replaced once floci's multi-account support was verified. |
| 0005 `tf-json-render-output` | §5.2 | The renderer must be deterministic and testable. | Emit `.tf.json` via `encoding/json`, which sorts map keys. | Byte-stable golden tests; humans read plans, not JSON. | Generating HCL text with templates. |
| 0006 `deterministic-identities` | §5.5 | The GitHub stack needs AWS identifiers, but AWS state is ephemeral. | Compute ARNs and URLs from the platform config by convention. | Stacks are independent and plan in parallel; names are constrained (≤ 40-character claims, ≤ 10-character envs). | `terraform_remote_state` between stacks. |
| 0007 `user-managed-files-via-state-rm` | §5.7 | Some feature files must become the user's after creation. | Create once, `state rm`, record in `.idp/manifest.json`, as Firestartr does. | User edits are never reverted or deleted; mode switches are rejected in v1. | `ignore_changes` (deletes on removal); a `removed` block (no instance keys). |
| 0008 `no-dflook` | §6.5 | The gate needs fingerprint subsets, policies and one commit per run. | Own steps plus `idp plan-summary` instead of the dflook actions. | More code, all of it testable; removes the undocumented-encryption risk. | dflook tofu-plan/apply. |
| 0009 `two-apps-and-split-environments` | §7.3 | Required reviewers gate every job that references an environment. | Reader App for plans; writer App key only in `idp-write`; approval in a secret-less `idp-approval`. | PR code never sees a write token; one approval per run. | One App with the key in the approval env (several approvals per run). |
| 0010 `public-repos-on-free-plan` | §2.3 | On Free, rulesets and environment reviewers exist only for public repos. | Every repo, including Component repos, is public. | Nothing secret may live in repos, logs or state; private repos need the Team plan. | Team plan; private repos without protections. |
| 0011 `bootstrap-outside-the-idp` | §9.2 | The IDP must not manage the protections that guard it. | `idp bootstrap apply/check`, run by an org owner; `check` runs in drift. | Apps are created via the browser manifest flow; there is a small manual runbook. | Managing the claims repo through claims; a bootstrap OpenTofu stack (state chicken-and-egg). |
| 0012 `required-workspace-policy` | §4.5 | Firestartr defaults to `observe`, so a forgotten field silently does nothing. | `policy` is required; only `full-control` may delete. | Explicit intent in every Workspace; deleting a non-`full-control` workspace takes two PRs. | Defaulting to `observe` or to `full-control`. |
| 0014 `single-org-isolated-by-repo-and-prefix` | §3.1, §8.5 | The plan called for a separate sandbox org for E2E and spikes. | Owner decision (2026-10-09): one org; tests are isolated by claims repo (`idp-claims-e2e`, own `wet`, state and passphrase) and by the `e2e-`/`spike-` prefix. | Simpler: two Apps instead of four, one bootstrap of the org settings; Apps, keys, org settings and rate limit are shared between production and tests; the harness must refuse to delete unprefixed resources and `idp-claims` must reject prefixed names. | A sandbox org per environment (stronger isolation, double the Apps and bootstrap). |

- [ ] **Step 2: Write `docs/adr/0013-phase-0-spike-findings.md`**

Use the same header (Status: Accepted, Spec: §10–§11), then these sections. Fill
every cell from the scratch notes and job summaries.

```markdown
## Pins adopted (all ≥ 14 days old on 2026-10-08)

OpenTofu 1.12.6 · integrations/github 6.13.0 · hashicorp/aws 6.66.0 · floci 2.1.0 (digest in the Phase 0 plan) · actions as pinned in `.github/workflows/ci.yaml`.

## S1 — encrypted state through Git

| Measure | Value |
|---|---|
| state_bytes (1 resource) | <from Task 14> |
| apply_seconds | <from Task 14> |
| round-trip / tamper / plaintext | <ok/rejected/refused, or the failure> |

## S2 — GitHub provider with an App token (10 resources)

| Measure | Value |
|---|---|
| apply / plan (refresh) / destroy seconds | <from Task 15> |
| Ruleset bypass for the writer App | <works / fails + error> |
| Writing `.github/workflows/*` | <works / fails + error> |
| Pending-invite plan exit → after acceptance | <e.g. 2 → 0> |
| Extrapolation: seconds per resource × 450 resources (50 components) vs the 1-hour token | <number> |

## S3 — floci

| Measure | Value |
|---|---|
| floci_ready / init cold / init cached / apply v1 / plan v2 (seconds) | <from Task 16> |
| Account isolation | <ok / broken> |
| role_arn / repository_url formats | <values> |
| Fidelity gaps | <none / list> |

## Decisions taken from these numbers

- Risk 1 (pending invite): <apply the Task 15 Step 6 rule>.
- Risk 2 (token expiry): if the extrapolation is under 30 minutes, keep minted tokens; otherwise Phase 1 adopts the provider's `app_auth`.
- Risk 3 (CI minutes): replay cost per stack = init cached + apply v1 + plan v2, from S3.
- Risk 4 (fidelity): <gaps and how Phase 2 handles each one>.
- Reader App permissions for running `check` in drift are validated in Phase 1. The REST permission schema exposes no `variables` permission, so a gap there is fixed by updating `reader.json` then.
```

- [ ] **Step 3: Update spec §11**

Add a "Phase 0 result" column to the risk table, with one short verdict per row,
taken from ADR-0013.

- [ ] **Step 4: Link the ADRs from `README.md`**

Under the "Design" line, add: `- Decisions: [docs/adr/](docs/adr/)`

- [ ] **Step 5: Commit (doc-only, straight to main) and push** (outward-facing: ask first)

```bash
git add docs README.md
git commit -m "docs: add adrs 0001-0013 and phase 0 findings"
git push
```

- [ ] **Step 6: Archive the spike repo** (outward-facing: ask first)

Run: `gh repo archive jellalshadows-idp/idp-spike --yes`
Expected: the repo shows as archived, and ADR-0013 is the only place the results
live.
