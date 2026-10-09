# Phase 1a — Claims to Render Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn a claims repo (`config/platform.yaml`, `claims/groups/*.yaml`, `claims/components/*.yaml`) into a validated model and a deterministic OpenTofu GitHub stack, backed by tested `github/group` and `github/component` modules.

**Architecture:**
- **Validation, in two layers.**
  - Embedded JSON Schemas (draft 2020-12) are the single source of truth for claim structure.
  - A Go loader adds semantic checks and reports every problem at once, with file and line.
- **Render.** A pure function emits `rendered/github/main.tf.json`: provider, local backend, enforced encryption and one module call per claim. It also copies a pinned provider lock file.
- **Modules.** Thin, with all GitHub resource logic in `modules/github/*`, verified with `tofu test` and mocked providers.

**Tech Stack:**
- Go 1.26, `go.yaml.in/yaml/v3` v3.0.5, `github.com/santhosh-tekuri/jsonschema/v6` v6.0.3.
- OpenTofu 1.12.6, `integrations/github` 6.13.0.

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §4.1–§4.4, §4.6, §4.7, §5.1–§5.4, §7.2, §8.2 and §8.3. Phase 0 context is in `docs/phases/phase-0.md`.

## Phase 1 decomposition (decided while planning)

Phase 1 in spec §10 is too large for one plan, so it is split into three plans. Each one produces working, testable software, and each starts only after the previous one ships:

| Plan | Delivers |
|---|---|
| **1a (this plan)** | Claims → validated model → rendered GitHub stack, plus the `github/group` and `github/component` modules |
| 1b | `idp diff`, `idp plan-summary`, `idp gate`, the reusable workflows (`pr.yaml`, `reconcile.yaml`, `drift.yaml`), the `wet` commit, and the ADRs for the drift token strategy and exit codes |
| 1c | E2E harness, release-please + GoReleaser + attestations, Renovate, `v0.1.0` |

## Global Constraints

**Toolchain and versions**
- Go module: `github.com/jellalshadows-idp/idp-engine`, Go directive `go 1.26.0`.
- New dependencies allowed: `go.yaml.in/yaml/v3 v3.0.5` and `github.com/santhosh-tekuri/jsonschema/v6 v6.0.3`, plus whatever `go mod tidy` selects for them (e.g. `golang.org/x/text`). Every version must be ≥ 14 days old on adoption; report the selected versions.
- OpenTofu `1.12.6`; provider `integrations/github` `6.13.0`, pinned exactly.
- Pinned actions (same pins as Phase 0):
  - `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1` (v7.0.1)
  - `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` (v7.0.0)
  - `opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4` (v2.0.2)

**Claim format**
- `apiVersion: idp/v1` everywhere.
- Claim names: `^[a-z][a-z0-9-]{0,38}[a-z0-9]$`. Environment names: `^[a-z][a-z0-9]{0,9}$`. GitHub logins and org names: `^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`.
- The directory decides the kind: `claims/groups` holds Group, `claims/components` holds Component, and `config/platform.yaml` is Platform.
- Workspace claims arrive in Phase 3. Until then, a `claims/workspaces/` directory is an error.
- Platform config gains `github.writerAppId` (required) and `naming.requiredPrefix` / `naming.reservedPrefixes` (optional). This is spec amendment A1 in Task 1.

**Rendered output**
- Files: `<out>/github/main.tf.json` and `<out>/github/.terraform.lock.hcl`, plus the marker `<out>/.idp-rendered`.
- Module sources: `git::https://github.com/jellalshadows-idp/idp-engine.git//modules/github/<module>?ref=<ref>`, or `<modules-dir>/github/<module>` for validation.
- Local backend path: `../../tfstate/github.tfstate`.
- Encryption: `terraform.encryption.state.enforced` and `plan.enforced` are `true`. Key material always comes from `TF_ENCRYPTION`.

**Process**
- Branch `feat/phase-1a-render`; one PR at the end (Task 13).
- Conventional commits, never with Co-Authored-By or any AI attribution.
- No `go build`. Use `go test ./...` locally; `-race` runs only in CI.
- Shell rule: never use `cat`, `grep`, `find`, `sed` or `ls`, and never write files with shell heredocs. Use the Read/Write/Edit tools, `rg` and `gh`.
- Docs are in English.

## Review Focus

1. **A claim file holding two YAML documents (`---`).** The second document must not be silently ignored; it is an explicit error. Pinned by `TestParseFile` ("two documents") in Task 3.
2. **A claim file named `api.yml`, or any stray file or subdirectory in a claims directory.** This must be an explicit error, not a silently missing claim. Dotfiles such as `.gitkeep` are ignored. Pinned by `TestLoadDiagnostics` (".yml file", "subdirectory", "gitkeep ignored") in Task 5.
3. **A Component whose owner Group file exists but is invalid.** Only the Group's own errors are reported, never a misleading "owner group does not exist". Pinned by `TestLoadInvalidOwnerGroupDoesNotCascade` in Task 5.
4. **`idp render --out` pointing at a non-empty directory that is not a previous render.** The render refuses and deletes nothing. Pinned by `TestWriteTreeRefusesUnmarkedDirectory` in Task 9 and `TestRenderRefusesForeignOutDir` in Task 10.
5. **Descriptions with `&`, `<`, `>` or non-ASCII text** (e.g. `Pagos & cobros <beta> — ñ`). They must render literally, with no `\u0026`, and byte-stable. Pinned by `TestRenderKeepsTextLiteral` in Task 9.

## File Structure

```
idp-engine/
├─ schemas/
│  ├─ schemas.go                 # embeds *.json; Kinds; File(kind)
│  ├─ schemas_test.go
│  ├─ platform.json  group.json  component.json
├─ internal/claims/
│  ├─ diag.go                    # Diagnostic, String, Annotation, sortDiagnostics
│  ├─ yamlfile.go                # parseFile (one doc, mapping), lineIndex, document.line, pointer
│  ├─ schema.go                  # validator (compiled schemas), validate → diagnostics with lines
│  ├─ model.go                   # Model, Platform, Group, Member, Component (+ defaults)
│  ├─ load.go                    # Load(dir): files, decode, semantic checks
│  ├─ diag_test.go  yamlfile_test.go  schema_test.go  load_test.go
├─ internal/render/
│  ├─ render.go                  # Options, Render, versions, module sources, marshal
│  ├─ write.go                   # Marker, WriteTree
│  ├─ render_test.go  write_test.go
│  └─ testdata/basic/{config/platform.yaml, claims/groups/platform.yaml, claims/components/api.yaml, expected/github/main.tf.json}
├─ internal/version/version.go   # Version (ldflags), Repo
├─ internal/cli/
│  ├─ claims.go                  # runValidate, runRender, loadClaims, relativeModulesDir
│  ├─ claims_test.go
│  └─ cli.go                     # (modify) usage + dispatch
├─ lockfiles/
│  ├─ lockfiles.go               # embeds github.terraform.lock.hcl
│  └─ github.terraform.lock.hcl
├─ modules/github/group/{versions.tf, variables.tf, main.tf, outputs.tf, tests/group.tftest.hcl}
├─ modules/github/component/{versions.tf, variables.tf, main.tf, outputs.tf, tests/component.tftest.hcl}
├─ docs/claims.md                # claims reference
├─ docs/adr/0015-claim-parsing-and-validation.md
├─ docs/phases/phase-1a.md       # execution log (Task 13)
└─ .github/workflows/ci.yaml     # (modify) modules + render-smoke jobs
```

---

### Task 1: Decisions first — spec amendments and ADR-0015

**Files:**
- Modify: `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` (§4.6, §5.3, §10)
- Create: `docs/adr/0015-claim-parsing-and-validation.md`
- Modify: `docs/adr/README.md` (index row)

**Interfaces:**
- Produces: the platform fields `github.writerAppId`, `naming.requiredPrefix` and `naming.reservedPrefixes`, used by every later task.

- [ ] **Step 1: Create the branch**

Run: `git switch main && git pull --ff-only && git switch -c feat/phase-1a-render`

- [ ] **Step 2: Amend spec §4.6 (amendment A1)**

In the §4.6 YAML example, replace the `github:` block and add a `naming:` block, so it reads:

```yaml
apiVersion: idp/v1
kind: Platform
github:
  org: <org>
  writerAppId: 5255579            # the writer App's id: bypass actor of every Component ruleset
  archiveOnDestroy: true          # false in idp-claims-e2e
  requiredApprovals: 1
naming:                           # optional (ADR-0014)
  reservedPrefixes: [e2e-, spike-] # idp-claims; idp-claims-e2e sets `requiredPrefix: e2e-` instead
environments:                     # names match ^[a-z][a-z0-9]{0,9}$
  dev:     { aws: { accountId: "000000000001", region: eu-west-1 } }
  staging: { aws: { accountId: "000000000002", region: eu-west-1 } }
  pro:     { aws: { accountId: "000000000003", region: eu-west-1 }, protected: true }
modules:
  allowedSources: ["git::https://github.com/<org>/"]
```

Below the example, after the sentence that ends "defined in §5.6.", add:

```markdown
`github.writerAppId` is required, because rendering is pure (§5.1): the bypass actor of every Component ruleset (§4.4) cannot be read from the environment, so it lives in the config. `naming` turns ADR-0014's prefix isolation into validation: a claim name may not start with any `reservedPrefixes` entry and must start with `requiredPrefix` when one is set. *(Amendment A1, 2026-10-09, Phase 1a planning.)*
```

- [ ] **Step 2b: Amend spec §5.3 (amendment A2)**

In the §5.3 JSON example, replace the line `"owner_team": "platform",` with:

```json
  "owner_team_id": "${module.group_platform.team_id}",
```

Directly below the example, before "All resource logic lives in the modules", add:

```markdown
A Component receives its owner team as a reference to the Group module's output, not as a slug. A `data "github_team"` lookup by slug would fail on the first plan, when the team and the repository are created in the same apply. The reference also orders the apply: team first, then repository access and environment reviewers. *(Amendment A2, 2026-10-09, Phase 1a planning.)*
```

- [ ] **Step 3: Amend spec §10**

Directly under the §10 roadmap table, add:

```markdown
Phase 1 is delivered as three plans, each shipping working software: **1a** claims → render (validation, the GitHub stack, the `github/group` and `github/component` modules); **1b** pipelines (`diff`, `plan-summary`, `gate`, the reusable workflows, the `wet` commit); **1c** E2E harness and the `v0.1.0` release. *(2026-10-09.)*
```

- [ ] **Step 4: Write `docs/adr/0015-claim-parsing-and-validation.md`**

```markdown
# 0015. Claim parsing and validation libraries

- Status: Accepted
- Date: 2026-10-09
- Spec: [§4.7](../superpowers/specs/2026-10-08-idp-on-actions-design.md#47-validation)

## Context

Spec §4.7 requires two validation layers (JSON Schema per kind, then semantic checks) and that **all** errors are reported together as GitHub annotations with file **and line**. That needs a YAML parser that keeps node positions, and a JSON Schema validator for draft 2020-12 whose errors carry the failing instance location, so each error can be mapped back to a line.

The usual Go YAML library, `gopkg.in/yaml.v3` (github.com/go-yaml/yaml), is archived; its README says "THIS PROJECT IS UNMAINTAINED" (verified 2026-10-09). The YAML organization maintains a fork at github.com/yaml/go-yaml, published as `go.yaml.in/yaml/v3`; its v4 line is still release candidates.

## Decision

- Parse claims with `go.yaml.in/yaml/v3` (v3.0.5). Use `yaml.Node` to build a JSON-pointer → line index per file, and accept exactly one YAML document per file.
- Validate with `github.com/santhosh-tekuri/jsonschema/v6` (v6.0.3). It passes the official 2020-12 test suite, and its `ValidationError` tree carries `InstanceLocation` for every leaf.
- The JSON Schemas in `schemas/*.json` are the **single source of truth** for claim structure. They are embedded in the binary and published at each tag, so editors get completion through `# yaml-language-server: $schema=…`.
- Schemas are strict (`additionalProperties: false`). Fields of later phases (`aws`, `features`, Workspace claims) are rejected until the phase that implements them extends the schema.
- The directory decides the kind (`claims/groups` → Group, `claims/components` → Component). A file whose `kind` disagrees fails the schema's `const`.

## Consequences

**Positive**
- Every structural error has a file and line, with no hand-written structural checks duplicating the schema.
- Unknown or not-yet-supported fields fail fast instead of being ignored.

**Negative / costs**
- Two new dependencies (plus `golang.org/x/text` for error messages). All are pinned ≥ 14 days old.
- Error wording comes from the validator library; tests assert locations and pointers, not its exact text.

**Follow-ups**
- Phases 2–4 extend the schemas (Component `aws`, `features`; Workspace) and add their semantic checks.

## Alternatives considered

- **`gopkg.in/yaml.v3`.** Rejected: archived and unmaintained.
- **`go.yaml.in/yaml/v4`.** Rejected: only release candidates exist.
- **Go structs with hand-written validation.** Rejected: a second source of truth next to the published schemas, and no editor completion.
- **Choosing the kind from the `kind` field.** Rejected: a misfiled claim would validate against the wrong rules silently.
```

- [ ] **Step 5: Add the ADR to the index**

In `docs/adr/README.md`, add after the 0014 row:

```markdown
| 0015 | [Claim parsing and validation libraries](0015-claim-parsing-and-validation.md) | Accepted |
```

- [ ] **Step 6: Commit**

```bash
git add docs
git commit -m "docs: amend spec for phase 1a (a1, a2) and add adr 0015"
```

---

### Task 2: JSON Schemas (embedded, single source of truth)

**Files:**
- Create: `schemas/schemas.go`, `schemas/platform.json`, `schemas/group.json`, `schemas/component.json`, `schemas/schemas_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces (package `github.com/jellalshadows-idp/idp-engine/schemas`):
  - `var FS embed.FS`
  - `var Kinds = []string{"Platform", "Group", "Component"}`
  - `func File(kind string) string` (`"Group"` → `"group.json"`)

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/santhosh-tekuri/jsonschema/v6@v6.0.3`

- [ ] **Step 2: Write the failing test `schemas/schemas_test.go`**

```go
package schemas

import (
	"bytes"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compile(t *testing.T, kind string) *jsonschema.Schema {
	t.Helper()
	raw, err := FS.ReadFile(File(kind))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	url := "https://schemas.idp.invalid/" + File(kind)
	if err := c.AddResource(url, doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		t.Fatalf("%s does not compile: %v", kind, err)
	}
	return sch
}

func instance(t *testing.T, js string) any {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(js))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFile(t *testing.T) {
	if got := File("Component"); got != "component.json" {
		t.Errorf("File(Component) = %q", got)
	}
}

func TestSchemas(t *testing.T) {
	tests := []struct {
		kind  string
		name  string
		doc   string
		valid bool
	}{
		{"Platform", "minimal", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{}}}`, true},
		{"Platform", "full", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":5255579,"archiveOnDestroy":false,"requiredApprovals":2},"naming":{"requiredPrefix":"e2e-","reservedPrefixes":["spike-"]},"environments":{"dev":{"aws":{"accountId":"000000000001","region":"eu-west-1"}},"pro":{"protected":true}},"modules":{"allowedSources":["git::https://github.com/acme/"]}}`, true},
		{"Platform", "missing writerAppId", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme"},"environments":{"dev":{}}}`, false},
		{"Platform", "bad environment name", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"Production":{}}}`, false},
		{"Platform", "approvals out of range", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1,"requiredApprovals":11},"environments":{"dev":{}}}`, false},
		{"Group", "valid", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[{"user":"alice","role":"maintainer"}]}`, true},
		{"Group", "bad role", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[{"user":"alice","role":"owner"}]}`, false},
		{"Group", "no members", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[]}`, false},
		{"Group", "unknown field", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[{"user":"a","role":"member"}],"team":"x"}`, false},
		{"Component", "valid", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","environments":["dev","pro"],"github":{"topics":["java"]}}`, true},
		{"Component", "owner without prefix", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"platform"}`, false},
		{"Component", "aws not supported yet", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{"registry":true}}`, false},
		{"Component", "duplicate environment", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","environments":["dev","dev"]}`, false},
		{"Component", "wrong kind", `{"apiVersion":"idp/v1","kind":"Group","name":"api","owner":"group:platform"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.kind+"/"+tt.name, func(t *testing.T) {
			err := compile(t, tt.kind).Validate(instance(t, tt.doc))
			if tt.valid && err != nil {
				t.Errorf("want valid, got %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("want invalid, got valid")
			}
		})
	}
}
```

- [ ] **Step 3: Run it and see it fail**

Run: `go test ./schemas/`
Expected: FAIL to compile, with `undefined: FS` and `undefined: File`.

- [ ] **Step 4: Write the schemas and `schemas/schemas.go`**

`schemas/schemas.go`:
```go
// Package schemas embeds the JSON Schemas (draft 2020-12) of every claim kind.
// They are the single source of truth for claim structure (ADR-0015): the CLI
// validates with them, and editors use them through
// `# yaml-language-server: $schema=...`.
package schemas

import (
	"embed"
	"strings"
)

// FS holds platform.json, group.json and component.json.
//
//go:embed *.json
var FS embed.FS

// Kinds lists the kinds that have a schema, in a stable order.
var Kinds = []string{"Platform", "Group", "Component"}

// File returns the schema file name of a kind ("Group" → "group.json").
func File(kind string) string { return strings.ToLower(kind) + ".json" }
```

`schemas/platform.json`:
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Platform",
  "description": "config/platform.yaml of a claims repo (spec §4.6).",
  "type": "object",
  "additionalProperties": false,
  "required": ["apiVersion", "kind", "github", "environments"],
  "properties": {
    "apiVersion": { "const": "idp/v1" },
    "kind": { "const": "Platform" },
    "github": {
      "type": "object",
      "additionalProperties": false,
      "required": ["org", "writerAppId"],
      "properties": {
        "org": { "type": "string", "pattern": "^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$" },
        "writerAppId": { "type": "integer", "minimum": 1 },
        "archiveOnDestroy": { "type": "boolean", "default": true },
        "requiredApprovals": { "type": "integer", "minimum": 0, "maximum": 10, "default": 1 }
      }
    },
    "naming": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "requiredPrefix": { "type": "string", "pattern": "^[a-z][a-z0-9]*-$" },
        "reservedPrefixes": {
          "type": "array",
          "uniqueItems": true,
          "items": { "type": "string", "pattern": "^[a-z][a-z0-9]*-$" }
        }
      }
    },
    "environments": {
      "type": "object",
      "minProperties": 1,
      "propertyNames": { "pattern": "^[a-z][a-z0-9]{0,9}$" },
      "additionalProperties": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
          "protected": { "type": "boolean", "default": false },
          "aws": {
            "type": "object",
            "additionalProperties": false,
            "required": ["accountId", "region"],
            "properties": {
              "accountId": { "type": "string", "pattern": "^[0-9]{12}$" },
              "region": { "type": "string", "pattern": "^[a-z]{2}(-[a-z]+)+-[0-9]$" }
            }
          }
        }
      }
    },
    "modules": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "allowedSources": { "type": "array", "items": { "type": "string", "minLength": 1 } }
      }
    }
  }
}
```

`schemas/group.json`:
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Group",
  "description": "A GitHub team and its members (spec §4.3).",
  "type": "object",
  "additionalProperties": false,
  "required": ["apiVersion", "kind", "name", "members"],
  "properties": {
    "apiVersion": { "const": "idp/v1" },
    "kind": { "const": "Group" },
    "name": { "type": "string", "pattern": "^[a-z][a-z0-9-]{0,38}[a-z0-9]$" },
    "description": { "type": "string", "maxLength": 350 },
    "members": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["user", "role"],
        "properties": {
          "user": { "type": "string", "pattern": "^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$" },
          "role": { "enum": ["maintainer", "member"] }
        }
      }
    }
  }
}
```

`schemas/component.json`:
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Component",
  "description": "A service: its GitHub repository and environments (spec §4.4). The aws and features fields arrive in Phases 2 and 4.",
  "type": "object",
  "additionalProperties": false,
  "required": ["apiVersion", "kind", "name", "owner"],
  "properties": {
    "apiVersion": { "const": "idp/v1" },
    "kind": { "const": "Component" },
    "name": { "type": "string", "pattern": "^[a-z][a-z0-9-]{0,38}[a-z0-9]$" },
    "description": { "type": "string", "maxLength": 350 },
    "owner": { "type": "string", "pattern": "^group:[a-z][a-z0-9-]{0,38}[a-z0-9]$" },
    "environments": {
      "type": "array",
      "uniqueItems": true,
      "items": { "type": "string", "pattern": "^[a-z][a-z0-9]{0,9}$" }
    },
    "github": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "topics": {
          "type": "array",
          "maxItems": 20,
          "uniqueItems": true,
          "items": { "type": "string", "pattern": "^[a-z0-9][a-z0-9-]{0,49}$" }
        }
      }
    }
  }
}
```

- [ ] **Step 5: Run the tests and see them pass**

Run: `go test ./schemas/ && go vet ./... && gofmt -l .`
Expected: `ok`. Then run `go mod tidy` and report the versions it selected (`go list -m all | rg "jsonschema|x/text"`).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum schemas
git commit -m "feat(schemas): add json schemas for platform, group and component claims"
```

---

### Task 3: Diagnostics and YAML parsing with line positions

**Files:**
- Create: `internal/claims/diag.go`, `internal/claims/yamlfile.go`, `internal/claims/diag_test.go`, `internal/claims/yamlfile_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces (package `internal/claims`):
  - `type Diagnostic struct { File string; Line int; Message string }`, with `String() string` and `Annotation() string`.
  - Unexported:
    - `sortDiagnostics([]Diagnostic)`
    - `type document struct { file string; root *yaml.Node; lines map[string]int }`
    - `parseFile(file string, data []byte) (*document, []Diagnostic)`
    - `(d *document) line(tokens []string) int`
    - `pointer(tokens []string) string`

- [ ] **Step 1: Add the dependency**

Run: `go get go.yaml.in/yaml/v3@v3.0.5`

- [ ] **Step 2: Write the failing tests**

`internal/claims/diag_test.go`:
```go
package claims

import "testing"

func TestDiagnosticString(t *testing.T) {
	tests := []struct {
		d    Diagnostic
		want string
	}{
		{Diagnostic{File: "claims/groups/a.yaml", Line: 3, Message: "bad"}, "claims/groups/a.yaml:3: bad"},
		{Diagnostic{File: "config/platform.yaml", Message: "file not found"}, "config/platform.yaml: file not found"},
	}
	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestDiagnosticAnnotationEscapes(t *testing.T) {
	d := Diagnostic{File: "claims/a,b.yaml", Line: 4, Message: "100% wrong:\nsecond line"}
	want := "::error file=claims/a%2Cb.yaml,line=4::100%25 wrong:%0Asecond line"
	if got := d.Annotation(); got != want {
		t.Errorf("Annotation() = %q, want %q", got, want)
	}
	noLine := Diagnostic{File: "config/platform.yaml", Message: "file not found"}
	if got := noLine.Annotation(); got != "::error file=config/platform.yaml::file not found" {
		t.Errorf("Annotation() without line = %q", got)
	}
}

func TestSortDiagnostics(t *testing.T) {
	ds := []Diagnostic{{File: "b", Line: 1, Message: "x"}, {File: "a", Line: 9, Message: "y"}, {File: "a", Line: 2, Message: "z"}}
	sortDiagnostics(ds)
	if ds[0].File != "a" || ds[0].Line != 2 || ds[1].Line != 9 || ds[2].File != "b" {
		t.Errorf("sorted = %v", ds)
	}
}
```

`internal/claims/yamlfile_test.go`:
```go
package claims

import (
	"strings"
	"testing"
)

func TestParseFile(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		wantMsg  string // empty means valid
		wantLine int    // -1 means "any line > 0"
	}{
		{name: "valid mapping", data: "a: 1\n"},
		{name: "empty file", data: "", wantMsg: "file is empty", wantLine: 0},
		{name: "two documents", data: "a: 1\n---\nb: 2\n", wantMsg: "only one YAML document per file is allowed", wantLine: -1},
		{name: "not a mapping", data: "- a\n- b\n", wantMsg: "the document must be a mapping", wantLine: 1},
		{name: "invalid YAML reports a line", data: "a: 1\nb: [unclosed\n", wantMsg: "invalid YAML", wantLine: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, ds := parseFile("f.yaml", []byte(tt.data))
			if tt.wantMsg == "" {
				if len(ds) != 0 || doc == nil {
					t.Fatalf("want a document, got %v", ds)
				}
				return
			}
			if doc != nil || len(ds) != 1 || !strings.Contains(ds[0].Message, tt.wantMsg) {
				t.Fatalf("diagnostics = %v, want one containing %q", ds, tt.wantMsg)
			}
			if tt.wantLine == -1 && ds[0].Line <= 0 {
				t.Errorf("line = %d, want > 0", ds[0].Line)
			}
			if tt.wantLine >= 0 && ds[0].Line != tt.wantLine {
				t.Errorf("line = %d, want %d", ds[0].Line, tt.wantLine)
			}
		})
	}
}

func TestDocumentLine(t *testing.T) {
	src := "apiVersion: idp/v1\nkind: Group\nname: platform\nmembers:\n  - user: alice\n    role: owner\na/b: 1\n"
	doc, ds := parseFile("g.yaml", []byte(src))
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	tests := []struct {
		tokens []string
		want   int
	}{
		{nil, 1},
		{[]string{"name"}, 3},
		{[]string{"members"}, 4},
		{[]string{"members", "0"}, 5},
		{[]string{"members", "0", "role"}, 6},
		{[]string{"members", "0", "missing"}, 5},
		{[]string{"a/b"}, 7},
	}
	for _, tt := range tests {
		if got := doc.line(tt.tokens); got != tt.want {
			t.Errorf("line(%v) = %d, want %d", tt.tokens, got, tt.want)
		}
	}
	if got := pointer([]string{"a/b", "c~d"}); got != "/a~1b/c~0d" {
		t.Errorf("pointer = %q", got)
	}
}
```

- [ ] **Step 3: Run them and see them fail**

Run: `go test ./internal/claims/`
Expected: FAIL to compile, with `undefined: Diagnostic`.

- [ ] **Step 4: Implement `internal/claims/diag.go`**

```go
// Package claims loads and validates a claims repo: config/platform.yaml,
// claims/groups/*.yaml and claims/components/*.yaml (spec §4). Every problem is
// reported at once, located by file and line (spec §4.7).
package claims

import (
	"fmt"
	"sort"
	"strings"
)

// Diagnostic is one problem found in a claims repo.
type Diagnostic struct {
	File    string // slash-separated path relative to the claims repo root
	Line    int    // 1-based; 0 when the problem has no line (e.g. a missing file)
	Message string
}

func (d Diagnostic) String() string {
	if d.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", d.File, d.Line, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.File, d.Message)
}

// Annotation formats d as a GitHub Actions error annotation, escaped as the
// workflow-commands docs require, so the problem shows inline on the PR.
func (d Diagnostic) Annotation() string {
	props := "file=" + escapeProperty(d.File)
	if d.Line > 0 {
		props += fmt.Sprintf(",line=%d", d.Line)
	}
	return "::error " + props + "::" + escapeData(d.Message)
}

var (
	dataEscaper     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

func escapeData(s string) string     { return dataEscaper.Replace(s) }
func escapeProperty(s string) string { return propertyEscaper.Replace(s) }

// sortDiagnostics orders diagnostics by file, line and message, so output is stable.
func sortDiagnostics(ds []Diagnostic) {
	sort.Slice(ds, func(i, j int) bool {
		if ds[i].File != ds[j].File {
			return ds[i].File < ds[j].File
		}
		if ds[i].Line != ds[j].Line {
			return ds[i].Line < ds[j].Line
		}
		return ds[i].Message < ds[j].Message
	})
}
```

- [ ] **Step 5: Implement `internal/claims/yamlfile.go`**

```go
package claims

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// document is one parsed claim file: its top-level mapping and an index from
// JSON pointers (RFC 6901) to the line where each value is declared.
type document struct {
	file  string
	root  *yaml.Node
	lines map[string]int
}

var yamlErrorLine = regexp.MustCompile(`line (\d+)`)

// parseFile parses exactly one YAML document whose root is a mapping. Anything
// else (empty, several documents, a list, invalid YAML) is a diagnostic.
func parseFile(file string, data []byte) (*document, []Diagnostic) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, []Diagnostic{{File: file, Message: "file is empty"}}
		}
		return nil, []Diagnostic{parseDiagnostic(file, err)}
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, []Diagnostic{parseDiagnostic(file, err)}
		}
		line := extra.Line
		if line == 0 {
			line = 1
		}
		return nil, []Diagnostic{{File: file, Line: line, Message: "only one YAML document per file is allowed"}}
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		line := doc.Line
		if line == 0 {
			line = 1
		}
		return nil, []Diagnostic{{File: file, Line: line, Message: "the document must be a mapping (key: value pairs)"}}
	}
	root := doc.Content[0]
	return &document{file: file, root: root, lines: lineIndex(root)}, nil
}

func parseDiagnostic(file string, err error) Diagnostic {
	d := Diagnostic{File: file, Message: "invalid YAML: " + strings.TrimPrefix(err.Error(), "yaml: ")}
	if m := yamlErrorLine.FindStringSubmatch(err.Error()); m != nil {
		d.Line, _ = strconv.Atoi(m[1])
	}
	return d
}

// lineIndex maps JSON pointers into the tree to lines. Object members point at
// their key's line, which is where an editor should put the cursor.
func lineIndex(root *yaml.Node) map[string]int {
	idx := map[string]int{"": root.Line}
	var walk func(n *yaml.Node, ptr string)
	walk = func(n *yaml.Node, ptr string) {
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				key, val := n.Content[i], n.Content[i+1]
				child := ptr + "/" + escapePointer(key.Value)
				idx[child] = key.Line
				walk(val, child)
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				child := ptr + "/" + strconv.Itoa(i)
				idx[child] = item.Line
				walk(item, child)
			}
		case yaml.AliasNode:
			if n.Alias != nil {
				walk(n.Alias, ptr)
			}
		}
	}
	walk(root, "")
	return idx
}

var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

func escapePointer(token string) string { return pointerEscaper.Replace(token) }

// pointer builds an RFC 6901 JSON pointer from unescaped tokens.
func pointer(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString("/")
		b.WriteString(escapePointer(t))
	}
	return b.String()
}

// line returns the line of the deepest indexed ancestor of the location, so a
// missing property points at the object that should contain it.
func (d *document) line(tokens []string) int {
	for n := len(tokens); n >= 0; n-- {
		if l, ok := d.lines[pointer(tokens[:n])]; ok {
			return l
		}
	}
	return d.root.Line
}
```

- [ ] **Step 6: Run the tests and see them pass**

Run: `go test ./internal/claims/ && go vet ./... && gofmt -l .`
Expected: `ok`. If the YAML library reports a different line for the "two documents" or "invalid YAML" case, the tests accept any line > 0 there. Fix the code only if a line is 0.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/claims
git commit -m "feat(claims): add diagnostics and yaml parsing with line positions"
```

---

### Task 4: Schema validation mapped to lines

**Files:**
- Create: `internal/claims/schema.go`, `internal/claims/schema_test.go`

**Interfaces:**
- Consumes: `schemas.FS`, `schemas.Kinds`, `schemas.File` (Task 2); `document`, `Diagnostic`, `pointer` (Task 3).
- Produces (unexported):
  - `type validator struct{...}`
  - `newValidator() (*validator, error)`
  - `(v *validator) validate(kind string, doc *document) []Diagnostic`

- [ ] **Step 1: Write the failing test `internal/claims/schema_test.go`**

```go
package claims

import (
	"strings"
	"testing"
)

func mustDoc(t *testing.T, src string) *document {
	t.Helper()
	doc, ds := parseFile("claims/x.yaml", []byte(src))
	if len(ds) > 0 {
		t.Fatalf("fixture does not parse: %v", ds)
	}
	return doc
}

func TestValidate(t *testing.T) {
	v, err := newValidator()
	if err != nil {
		t.Fatal(err)
	}
	group := "apiVersion: idp/v1\nkind: Group\nname: platform\nmembers:\n  - user: alice\n    role: maintainer\n"
	tests := []struct {
		name         string
		kind         string
		src          string
		wantLine     int    // 0 means "valid"
		wantContains string // substring of the diagnostic message
	}{
		{name: "valid group", kind: "Group", src: group},
		{name: "bad role points at the role line", kind: "Group", src: strings.Replace(group, "maintainer", "owner", 1), wantLine: 6, wantContains: "/members/0/role"},
		{name: "missing members points at the document", kind: "Group", src: "apiVersion: idp/v1\nkind: Group\nname: platform\n", wantLine: 1, wantContains: "members"},
		{name: "unknown field points at its key", kind: "Group", src: group + "team: x\n", wantLine: 7, wantContains: "team"},
		{name: "bad name", kind: "Group", src: strings.Replace(group, "name: platform", "name: Platform", 1), wantLine: 3, wantContains: "/name"},
		{name: "valid component", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nenvironments: [dev, pro]\n"},
		{name: "owner without group prefix", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: platform\n", wantLine: 4, wantContains: "/owner"},
		{name: "aws is not supported yet", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\naws:\n  registry: true\n", wantLine: 5, wantContains: "aws"},
		{name: "a group filed as a component", kind: "Component", src: group, wantLine: 2, wantContains: "/kind"},
		{name: "non-string keys", kind: "Group", src: "1: a\n", wantLine: 1, wantContains: "mapping keys must be strings"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := v.validate(tt.kind, mustDoc(t, tt.src))
			if tt.wantLine == 0 {
				if len(ds) != 0 {
					t.Fatalf("want valid, got %v", ds)
				}
				return
			}
			for _, d := range ds {
				if d.Line == tt.wantLine && strings.Contains(d.Message, tt.wantContains) {
					return
				}
			}
			t.Fatalf("diagnostics = %v, want one at line %d containing %q", ds, tt.wantLine, tt.wantContains)
		})
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/claims/ -run TestValidate`
Expected: FAIL to compile, with `undefined: newValidator`.

- [ ] **Step 3: Implement `internal/claims/schema.go`**

```go
package claims

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"go.yaml.in/yaml/v3"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/jellalshadows-idp/idp-engine/schemas"
)

// schemaBaseURL identifies the embedded schemas inside the compiler. Nothing is
// fetched from it: every schema is added as an in-memory resource first.
const schemaBaseURL = "https://schemas.idp.invalid/"

// validator holds the compiled schema of every claim kind.
type validator struct {
	byKind  map[string]*jsonschema.Schema
	printer *message.Printer
}

func newValidator() (*validator, error) {
	c := jsonschema.NewCompiler()
	v := &validator{byKind: map[string]*jsonschema.Schema{}, printer: message.NewPrinter(language.English)}
	for _, k := range schemas.Kinds {
		raw, err := schemas.FS.ReadFile(schemas.File(k))
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", k, err)
		}
		url := schemaBaseURL + schemas.File(k)
		if err := c.AddResource(url, doc); err != nil {
			return nil, fmt.Errorf("schema %s: %w", k, err)
		}
		sch, err := c.Compile(url)
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", k, err)
		}
		v.byKind[k] = sch
	}
	return v, nil
}

// validate checks doc against the schema of kind and returns one diagnostic per
// failing leaf of the validation error tree, each located at its YAML line.
func (v *validator) validate(k string, doc *document) []Diagnostic {
	inst, err := jsonInstance(doc.root)
	if err != nil {
		return []Diagnostic{{File: doc.file, Line: doc.root.Line, Message: err.Error()}}
	}
	err = v.byKind[k].Validate(inst)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []Diagnostic{{File: doc.file, Line: doc.root.Line, Message: err.Error()}}
	}
	var out []Diagnostic
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		loc := e.InstanceLocation
		// An unknown property is reported on its parent object; point at the
		// offending key instead, which is where the fix goes.
		if ap, ok := e.ErrorKind.(*kind.AdditionalProperties); ok && len(ap.Properties) > 0 {
			loc = append(append([]string{}, loc...), ap.Properties[0])
		}
		out = append(out, Diagnostic{
			File:    doc.file,
			Line:    doc.line(loc),
			Message: fmt.Sprintf("%s: %s", locationLabel(e.InstanceLocation), e.ErrorKind.LocalizedString(v.printer)),
		})
	}
	walk(ve)
	return out
}

func locationLabel(tokens []string) string {
	if len(tokens) == 0 {
		return "(document)"
	}
	return pointer(tokens)
}

// jsonInstance converts a YAML mapping into the JSON value model the validator
// expects (json.Number numbers, string-keyed objects).
func jsonInstance(root *yaml.Node) (any, error) {
	var v any
	if err := root.Decode(&v); err != nil {
		return nil, fmt.Errorf("cannot read document: %w", err)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("unsupported YAML (mapping keys must be strings): %w", err)
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}
```

`kind.AdditionalProperties{Properties []string}` and `ValidationError{InstanceLocation []string; ErrorKind; Causes}` were verified against the v6.0.3 source while planning.

- [ ] **Step 4: Run the tests and see them pass**

Run: `go test ./internal/claims/ && go vet ./... && gofmt -l .`
Expected: `ok`. Then `go mod tidy`, and report the `golang.org/x/text` version.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/claims
git commit -m "feat(claims): validate claims against their schemas with line positions"
```

---

### Task 5: Model, loader and semantic checks

**Files:**
- Create: `internal/claims/model.go`, `internal/claims/load.go`, `internal/claims/load_test.go`

**Interfaces:**
- Consumes: `parseFile`, `document`, `Diagnostic`, `sortDiagnostics` (Task 3); `newValidator`, `validate` (Task 4).
- Produces:
  - `type Model struct { Platform Platform; Groups []Group; Components []Component }`. Groups and Components are sorted by name.
  - `type Platform` with `GitHub.Org string`, `GitHub.WriterAppID int64`, `Naming.RequiredPrefix string`, `Naming.ReservedPrefixes []string`, `Environments map[string]Environment`; methods `ArchiveOnDestroy() bool` (default true) and `RequiredApprovals() int` (default 1).
  - `type Environment struct { Protected bool }`
  - `type Group struct { Name, Description string; Members []Member }` and `type Member struct { User, Role string }`.
  - `type Component struct { Name, Description, Owner string; Environments []string; GitHub struct{ Topics []string } }`, with `OwnerGroup() string`.
  - `func Load(dir string) (*Model, []Diagnostic, error)`

- [ ] **Step 1: Write the failing test `internal/claims/load_test.go`**

```go
package claims

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	validPlatform  = "apiVersion: idp/v1\nkind: Platform\ngithub:\n  org: acme\n  writerAppId: 5255579\nenvironments:\n  dev: {}\n  pro:\n    protected: true\n"
	validGroup     = "apiVersion: idp/v1\nkind: Group\nname: platform\nmembers:\n  - user: alice\n    role: maintainer\n"
	validComponent = "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nenvironments: [dev, pro]\n"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func baseRepo() map[string]string {
	return map[string]string{
		"config/platform.yaml":         validPlatform,
		"claims/groups/platform.yaml":  validGroup,
		"claims/components/api.yaml":   validComponent,
	}
}

func lines(ds []Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.String()
	}
	return out
}

func TestLoadValidRepo(t *testing.T) {
	m, ds, err := Load(writeTree(t, baseRepo()))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 0 {
		t.Fatalf("diagnostics: %v", lines(ds))
	}
	if m.Platform.GitHub.Org != "acme" || m.Platform.GitHub.WriterAppID != 5255579 {
		t.Errorf("platform github = %+v", m.Platform.GitHub)
	}
	if !m.Platform.ArchiveOnDestroy() || m.Platform.RequiredApprovals() != 1 {
		t.Error("platform defaults must be archiveOnDestroy=true and requiredApprovals=1")
	}
	if !m.Platform.Environments["pro"].Protected || m.Platform.Environments["dev"].Protected {
		t.Errorf("environments = %+v", m.Platform.Environments)
	}
	if len(m.Groups) != 1 || m.Groups[0].Name != "platform" || m.Groups[0].Members[0].Role != "maintainer" {
		t.Errorf("groups = %+v", m.Groups)
	}
	if len(m.Components) != 1 || m.Components[0].OwnerGroup() != "platform" {
		t.Errorf("components = %+v", m.Components)
	}
}

func TestLoadSortsClaimsByName(t *testing.T) {
	repo := baseRepo()
	repo["claims/components/zeta.yaml"] = strings.Replace(validComponent, "name: api", "name: zeta", 1)
	repo["claims/components/alpha.yaml"] = strings.Replace(validComponent, "name: api", "name: alpha", 1)
	m, ds, err := Load(writeTree(t, repo))
	if err != nil || len(ds) != 0 {
		t.Fatalf("err %v, diagnostics %v", err, lines(ds))
	}
	got := []string{m.Components[0].Name, m.Components[1].Name, m.Components[2].Name}
	if strings.Join(got, ",") != "alpha,api,zeta" {
		t.Errorf("order = %v", got)
	}
}

func TestLoadDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]string)
		want   []string // exact Diagnostic.String() lines, in order
	}{
		{"missing platform", func(r map[string]string) { delete(r, "config/platform.yaml") },
			[]string{"config/platform.yaml: file not found"}},
		{"name must match the file", func(r map[string]string) {
			r["claims/components/api.yaml"] = strings.Replace(validComponent, "name: api", "name: orders", 1)
		}, []string{`claims/components/api.yaml:3: name "orders" must match the file name "api"`}},
		{"owner group must exist", func(r map[string]string) {
			r["claims/components/api.yaml"] = strings.Replace(validComponent, "group:platform", "group:payments", 1)
		}, []string{`claims/components/api.yaml:4: owner group "payments" does not exist (no claims/groups/payments.yaml)`}},
		{"environment must exist", func(r map[string]string) {
			r["claims/components/api.yaml"] = strings.Replace(validComponent, "[dev, pro]", "[dev, staging]", 1)
		}, []string{`claims/components/api.yaml:5: environment "staging" is not defined in config/platform.yaml`}},
		{"duplicate member, case-insensitive", func(r map[string]string) {
			r["claims/groups/platform.yaml"] = validGroup + "  - user: Alice\n    role: member\n"
		}, []string{`claims/groups/platform.yaml:7: member "Alice" is listed more than once`}},
		{"required prefix", func(r map[string]string) {
			r["config/platform.yaml"] = validPlatform + "naming:\n  requiredPrefix: e2e-\n"
		}, []string{
			`claims/components/api.yaml:3: name "api" must start with "e2e-" in this claims repo (config/platform.yaml naming.requiredPrefix)`,
			`claims/groups/platform.yaml:3: name "platform" must start with "e2e-" in this claims repo (config/platform.yaml naming.requiredPrefix)`,
		}},
		{"reserved prefix", func(r map[string]string) {
			r["config/platform.yaml"] = validPlatform + "naming:\n  reservedPrefixes: [e2e-]\n"
			r["claims/groups/e2e-team.yaml"] = strings.Replace(validGroup, "name: platform", "name: e2e-team", 1)
		}, []string{`claims/groups/e2e-team.yaml:3: name "e2e-team" uses the reserved prefix "e2e-" (config/platform.yaml naming.reservedPrefixes)`}},
		{"contradictory naming", func(r map[string]string) {
			r["config/platform.yaml"] = validPlatform + "naming:\n  requiredPrefix: e2e-\n  reservedPrefixes: [e2e-]\n"
			delete(r, "claims/groups/platform.yaml")
			delete(r, "claims/components/api.yaml")
		}, []string{`config/platform.yaml:11: naming.requiredPrefix "e2e-" is also listed in naming.reservedPrefixes`}},
		{".yml file", func(r map[string]string) { r["claims/components/web.yml"] = validComponent },
			[]string{"claims/components/web.yml: unexpected file; claim files must end with .yaml"}},
		{"subdirectory", func(r map[string]string) { r["claims/groups/team/x.yaml"] = validGroup },
			[]string{"claims/groups/team: unexpected directory; claims are files directly in claims/groups"}},
		{"workspaces are not supported yet", func(r map[string]string) { r["claims/workspaces/dev/logs.yaml"] = "x: 1\n" },
			[]string{"claims/workspaces: Workspace claims are not supported yet (they arrive in Phase 3)"}},
		{"unexpected entry in claims", func(r map[string]string) { r["claims/notes.md"] = "# notes\n" },
			[]string{"claims/notes.md: unexpected entry; claims/ holds only groups/ and components/"}},
		{"gitkeep ignored", func(r map[string]string) { r["claims/groups/.gitkeep"] = "" }, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := baseRepo()
			tt.mutate(repo)
			_, ds, err := Load(writeTree(t, repo))
			if err != nil {
				t.Fatal(err)
			}
			got := lines(ds)
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("diagnostics =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestLoadInvalidOwnerGroupDoesNotCascade(t *testing.T) {
	repo := baseRepo()
	repo["claims/groups/platform.yaml"] = strings.Replace(validGroup, "maintainer", "owner", 1)
	_, ds, err := Load(writeTree(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if strings.Contains(d.Message, "does not exist") {
			t.Errorf("misleading cascade: %v", d)
		}
	}
	if len(ds) != 1 || ds[0].File != "claims/groups/platform.yaml" || ds[0].Line != 6 {
		t.Errorf("diagnostics = %v, want only the group's role error at line 6", lines(ds))
	}
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	repo := baseRepo()
	repo["claims/groups/platform.yaml"] = strings.Replace(validGroup, "maintainer", "owner", 1)
	repo["claims/components/api.yaml"] = strings.Replace(validComponent, "[dev, pro]", "[dev, staging]", 1)
	_, ds, err := Load(writeTree(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 2 || ds[0].File != "claims/components/api.yaml" || ds[1].File != "claims/groups/platform.yaml" {
		t.Errorf("diagnostics = %v, want two, sorted by file", lines(ds))
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/claims/ -run 'TestLoad'`
Expected: FAIL to compile, with `undefined: Load`.

- [ ] **Step 3: Implement `internal/claims/model.go`**

```go
package claims

import "strings"

// Model is a loaded claims repo. It is only meaningful when Load returned no
// diagnostics.
type Model struct {
	Platform   Platform
	Groups     []Group     // sorted by name
	Components []Component // sorted by name
}

// Platform is config/platform.yaml (spec §4.6, amendment A1).
type Platform struct {
	GitHub struct {
		Org               string `yaml:"org"`
		WriterAppID       int64  `yaml:"writerAppId"`
		ArchiveOnDestroy  *bool  `yaml:"archiveOnDestroy"`
		RequiredApprovals *int   `yaml:"requiredApprovals"`
	} `yaml:"github"`
	Naming struct {
		RequiredPrefix   string   `yaml:"requiredPrefix"`
		ReservedPrefixes []string `yaml:"reservedPrefixes"`
	} `yaml:"naming"`
	Environments map[string]Environment `yaml:"environments"`
}

// Environment is one entry of Platform.Environments. Its aws block is used from Phase 2.
type Environment struct {
	Protected bool `yaml:"protected"`
}

// ArchiveOnDestroy reports github.archiveOnDestroy, which defaults to true.
func (p Platform) ArchiveOnDestroy() bool {
	if p.GitHub.ArchiveOnDestroy == nil {
		return true
	}
	return *p.GitHub.ArchiveOnDestroy
}

// RequiredApprovals reports github.requiredApprovals, which defaults to 1.
func (p Platform) RequiredApprovals() int {
	if p.GitHub.RequiredApprovals == nil {
		return 1
	}
	return *p.GitHub.RequiredApprovals
}

// Group is a GitHub team and its members (spec §4.3).
type Group struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Members     []Member `yaml:"members"`
}

// Member is one team member.
type Member struct {
	User string `yaml:"user"`
	Role string `yaml:"role"`
}

// Component is a service's GitHub part (spec §4.4); aws and features arrive later.
type Component struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Owner        string   `yaml:"owner"`
	Environments []string `yaml:"environments"`
	GitHub       struct {
		Topics []string `yaml:"topics"`
	} `yaml:"github"`
}

// OwnerGroup is the Group name in Owner ("group:platform" → "platform").
func (c Component) OwnerGroup() string { return strings.TrimPrefix(c.Owner, "group:") }
```

- [ ] **Step 4: Implement `internal/claims/load.go`**

```go
package claims

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Load reads and validates the claims repo rooted at dir. It reports every problem
// at once; the model is only meaningful when there are no diagnostics. The error
// is reserved for failures of the tool itself (e.g. a broken embedded schema).
func Load(dir string) (*Model, []Diagnostic, error) {
	v, err := newValidator()
	if err != nil {
		return nil, nil, err
	}
	l := &loader{dir: dir, v: v, model: &Model{}, groupFiles: map[string]bool{}, docs: map[string]*document{}}
	l.loadPlatform()
	l.checkClaimsRoot()
	l.loadGroups()
	l.loadComponents()
	l.checkSemantics()
	sort.Slice(l.model.Groups, func(i, j int) bool { return l.model.Groups[i].Name < l.model.Groups[j].Name })
	sort.Slice(l.model.Components, func(i, j int) bool { return l.model.Components[i].Name < l.model.Components[j].Name })
	sortDiagnostics(l.diags)
	return l.model, l.diags, nil
}

type loader struct {
	dir        string
	v          *validator
	model      *Model
	diags      []Diagnostic
	platformOK bool                 // config/platform.yaml decoded cleanly
	groupFiles map[string]bool      // group names that have a file, valid or not
	docs       map[string]*document // "Group/<name>" or "Component/<name>" → its document
}

func (l *loader) add(ds ...Diagnostic) { l.diags = append(l.diags, ds...) }

func (l *loader) read(rel string) (*document, bool) {
	data, err := os.ReadFile(filepath.Join(l.dir, filepath.FromSlash(rel)))
	if err != nil {
		msg := err.Error()
		if errors.Is(err, fs.ErrNotExist) {
			msg = "file not found"
		}
		l.add(Diagnostic{File: rel, Message: msg})
		return nil, false
	}
	doc, ds := parseFile(rel, data)
	l.add(ds...)
	return doc, doc != nil
}

// decode validates doc against its kind's schema and, only if valid, decodes it.
func (l *loader) decode(kind string, doc *document, out any) bool {
	ds := l.v.validate(kind, doc)
	l.add(ds...)
	if len(ds) > 0 {
		return false
	}
	if err := doc.root.Decode(out); err != nil {
		l.add(Diagnostic{File: doc.file, Line: doc.root.Line, Message: err.Error()})
		return false
	}
	return true
}

func (l *loader) loadPlatform() {
	doc, ok := l.read("config/platform.yaml")
	if !ok || !l.decode("Platform", doc, &l.model.Platform) {
		return
	}
	l.platformOK = true
	n := l.model.Platform.Naming
	for _, p := range n.ReservedPrefixes {
		if n.RequiredPrefix != "" && p == n.RequiredPrefix {
			l.add(Diagnostic{File: doc.file, Line: doc.lines["/naming/requiredPrefix"],
				Message: fmt.Sprintf("naming.requiredPrefix %q is also listed in naming.reservedPrefixes", p)})
		}
	}
}

// checkClaimsRoot reports anything under claims/ other than groups/ and components/.
func (l *loader) checkClaimsRoot() {
	entries, err := os.ReadDir(filepath.Join(l.dir, "claims"))
	if errors.Is(err, fs.ErrNotExist) {
		return // a repo with no claims yet is valid
	}
	if err != nil {
		l.add(Diagnostic{File: "claims", Message: err.Error()})
		return
	}
	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasPrefix(name, "."):
		case e.IsDir() && (name == "groups" || name == "components"):
		case e.IsDir() && name == "workspaces":
			l.add(Diagnostic{File: "claims/workspaces", Message: "Workspace claims are not supported yet (they arrive in Phase 3)"})
		default:
			l.add(Diagnostic{File: "claims/" + name, Message: "unexpected entry; claims/ holds only groups/ and components/"})
		}
	}
}

// claimFiles lists the *.yaml files of a claims directory and reports anything
// else. Dotfiles (e.g. .gitkeep) are ignored.
func (l *loader) claimFiles(dirRel string) []string {
	entries, err := os.ReadDir(filepath.Join(l.dir, filepath.FromSlash(dirRel)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		l.add(Diagnostic{File: dirRel, Message: err.Error()})
		return nil
	}
	var files []string
	for _, e := range entries {
		rel := dirRel + "/" + e.Name()
		switch {
		case strings.HasPrefix(e.Name(), "."):
		case e.IsDir():
			l.add(Diagnostic{File: rel, Message: "unexpected directory; claims are files directly in " + dirRel})
		case strings.HasSuffix(e.Name(), ".yaml"):
			files = append(files, rel)
		default:
			l.add(Diagnostic{File: rel, Message: "unexpected file; claim files must end with .yaml"})
		}
	}
	sort.Strings(files)
	return files
}

func fileStem(rel string) string { return strings.TrimSuffix(path.Base(rel), ".yaml") }

func (l *loader) checkFileName(doc *document, name string) {
	if want := fileStem(doc.file); name != want {
		l.add(Diagnostic{File: doc.file, Line: doc.lines["/name"],
			Message: fmt.Sprintf("name %q must match the file name %q", name, want)})
	}
}

func (l *loader) loadGroups() {
	for _, rel := range l.claimFiles("claims/groups") {
		l.groupFiles[fileStem(rel)] = true
		doc, ok := l.read(rel)
		if !ok {
			continue
		}
		var g Group
		if !l.decode("Group", doc, &g) {
			continue
		}
		l.checkFileName(doc, g.Name)
		l.docs["Group/"+g.Name] = doc
		l.model.Groups = append(l.model.Groups, g)
	}
}

func (l *loader) loadComponents() {
	for _, rel := range l.claimFiles("claims/components") {
		doc, ok := l.read(rel)
		if !ok {
			continue
		}
		var c Component
		if !l.decode("Component", doc, &c) {
			continue
		}
		l.checkFileName(doc, c.Name)
		l.docs["Component/"+c.Name] = doc
		l.model.Components = append(l.model.Components, c)
	}
}

func (l *loader) checkSemantics() {
	for _, g := range l.model.Groups {
		doc := l.docs["Group/"+g.Name]
		l.checkPrefix(doc, g.Name)
		seen := map[string]bool{}
		for i, m := range g.Members {
			key := strings.ToLower(m.User) // GitHub logins are case-insensitive
			if seen[key] {
				l.add(Diagnostic{File: doc.file, Line: doc.lines[fmt.Sprintf("/members/%d/user", i)],
					Message: fmt.Sprintf("member %q is listed more than once", m.User)})
			}
			seen[key] = true
		}
	}
	for _, c := range l.model.Components {
		doc := l.docs["Component/"+c.Name]
		l.checkPrefix(doc, c.Name)
		if !l.groupFiles[c.OwnerGroup()] {
			l.add(Diagnostic{File: doc.file, Line: doc.lines["/owner"],
				Message: fmt.Sprintf("owner group %q does not exist (no claims/groups/%s.yaml)", c.OwnerGroup(), c.OwnerGroup())})
		}
		if !l.platformOK {
			continue
		}
		for i, env := range c.Environments {
			if _, ok := l.model.Platform.Environments[env]; !ok {
				l.add(Diagnostic{File: doc.file, Line: doc.lines[fmt.Sprintf("/environments/%d", i)],
					Message: fmt.Sprintf("environment %q is not defined in config/platform.yaml", env)})
			}
		}
	}
}

// checkPrefix enforces config/platform.yaml naming (ADR-0014 isolation).
func (l *loader) checkPrefix(doc *document, name string) {
	n := l.model.Platform.Naming
	line := doc.lines["/name"]
	if n.RequiredPrefix != "" && !strings.HasPrefix(name, n.RequiredPrefix) {
		l.add(Diagnostic{File: doc.file, Line: line,
			Message: fmt.Sprintf("name %q must start with %q in this claims repo (config/platform.yaml naming.requiredPrefix)", name, n.RequiredPrefix)})
	}
	for _, p := range n.ReservedPrefixes {
		if strings.HasPrefix(name, p) {
			l.add(Diagnostic{File: doc.file, Line: line,
				Message: fmt.Sprintf("name %q uses the reserved prefix %q (config/platform.yaml naming.reservedPrefixes)", name, p)})
		}
	}
}
```

- [ ] **Step 5: Run the tests and see them pass**

Run: `go test ./internal/claims/ && go vet ./... && gofmt -l .`
Expected: `ok`. The exact strings in `TestLoadDiagnostics` are this package's own messages, so they must match exactly. If a line number differs, the fixture's YAML is the source of truth: count its lines, then fix the code or the expectation and explain which in the report.

- [ ] **Step 6: Commit**

```bash
git add internal/claims
git commit -m "feat(claims): load claims repos with semantic checks and prefix rules"
```

---

### Task 6: Module `github/group`

**Files:**
- Create: `modules/github/group/versions.tf`, `variables.tf`, `main.tf`, `outputs.tf`, `tests/group.tftest.hcl`
- Create: `.gitignore` (the repo has none yet; the SDD workspace is excluded through `.git/info/exclude`)

**Interfaces:**
- Produces (module inputs/outputs used by the renderer and by `github/component`):
  - inputs `name` (string), `description` (string, default `""`), `members` (list(object({ user = string, role = string })));
  - outputs `team_id` and `slug`.

- [ ] **Step 0: Install OpenTofu 1.12.6 locally (one-time; no Docker needed)**

```bash
T="$HOME/.idp/tools/tofu-1.12.6"
mkdir -p "$T" && cd "$T"
gh release download v1.12.6 --repo opentofu/opentofu --pattern 'tofu_1.12.6_windows_amd64.zip' --pattern 'tofu_1.12.6_SHA256SUMS' --clobber
rg 'windows_amd64.zip' tofu_1.12.6_SHA256SUMS > expected.sum && sha256sum -c expected.sum
python -I -m zipfile -e tofu_1.12.6_windows_amd64.zip .
./tofu.exe version
```
Expected: `sha256sum` prints `tofu_1.12.6_windows_amd64.zip: OK` and the version prints `OpenTofu v1.12.6`. Later steps call it as `"$HOME/.idp/tools/tofu-1.12.6/tofu.exe"` (below: `$TOFU`).

- [ ] **Step 1: Ignore OpenTofu working files**

Create `.gitignore`. Module lock files are not committed: a module does not choose provider builds, only the root stack does, and the stack gets the lock file from `lockfiles/` (Task 8).
```gitignore
# OpenTofu working files (tofu init / tofu test in module directories)
.terraform/
modules/**/.terraform.lock.hcl
# Local output of `idp render`
/rendered/
```

- [ ] **Step 2: Write the failing test `modules/github/group/tests/group.tftest.hcl`**

```hcl
mock_provider "github" {}

variables {
  name        = "platform"
  description = "Platform team"
  members = [
    { user = "alice", role = "maintainer" },
    { user = "bob", role = "member" },
  ]
}

run "team_is_closed_and_named_after_the_claim" {
  command = plan

  assert {
    condition     = github_team.this.name == "platform"
    error_message = "the team must be named after the Group claim"
  }
  assert {
    condition     = github_team.this.privacy == "closed"
    error_message = "teams must be closed (visible to org members), spec §4.3"
  }
}

run "one_membership_per_member_with_its_role" {
  command = plan

  assert {
    condition     = length(github_team_membership.this) == 2
    error_message = "expected one membership per member"
  }
  assert {
    condition     = github_team_membership.this["alice"].role == "maintainer" && github_team_membership.this["bob"].role == "member"
    error_message = "membership roles must follow the claim"
  }
}

run "rejects_unknown_roles" {
  command = plan

  variables {
    members = [{ user = "carol", role = "owner" }]
  }

  expect_failures = [var.members]
}
```

- [ ] **Step 3: Run it and see it fail**

Run: `TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"; cd modules/github/group && "$TOFU" init -input=false && "$TOFU" test`
Expected: FAIL. Either `init` or `test` reports no resources or unknown references, because the module files don't exist yet.

- [ ] **Step 4: Write the module**

`modules/github/group/versions.tf`:
```hcl
terraform {
  required_version = ">= 1.12.0"

  required_providers {
    github = {
      source  = "integrations/github"
      version = "6.13.0"
    }
  }
}
```

`modules/github/group/variables.tf`:
```hcl
variable "name" {
  description = "Team name and slug: the Group claim name."
  type        = string
}

variable "description" {
  description = "Team description."
  type        = string
  default     = ""
}

variable "members" {
  description = "Team members: GitHub login and role (maintainer or member)."
  type = list(object({
    user = string
    role = string
  }))

  validation {
    condition     = alltrue([for m in var.members : contains(["maintainer", "member"], m.role)])
    error_message = "Each member role must be maintainer or member."
  }
}
```

`modules/github/group/main.tf`:
```hcl
resource "github_team" "this" {
  name        = var.name
  description = var.description
  privacy     = "closed"
}

resource "github_team_membership" "this" {
  for_each = { for m in var.members : m.user => m }

  team_id  = github_team.this.id
  username = each.value.user
  role     = each.value.role
}
```

`modules/github/group/outputs.tf`:
```hcl
output "team_id" {
  description = "Team id, used by Component modules for repository access and environment reviewers."
  value       = github_team.this.id
}

output "slug" {
  description = "Team slug."
  value       = github_team.this.slug
}
```

- [ ] **Step 5: Run the tests and see them pass**

Run: `TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"; cd modules/github/group && "$TOFU" fmt -check -recursive && "$TOFU" test`
Expected: `Success! 3 passed, 0 failed.` If an assertion path doesn't exist in the provider schema, adjust the path but not the intent, and record it in the report.

- [ ] **Step 6: Commit**

```bash
git add .gitignore modules/github/group
git commit -m "feat(modules): add github group module with tofu tests"
```

---

### Task 7: Module `github/component`

**Files:**
- Create: `modules/github/component/versions.tf`, `variables.tf`, `main.tf`, `outputs.tf`, `tests/component.tftest.hcl`

**Interfaces:**
- Consumes: the `team_id` output of `github/group`, which the renderer passes as `owner_team_id`.
- Produces inputs:
  - `name`, `description` (default `""`), `topics` (list(string), default `[]`);
  - `owner_team_id` (string);
  - `environments` (map(object({ protected = bool })), default `{}`);
  - `required_approvals` (number, 0–10);
  - `archive_on_destroy` (bool), `writer_app_id` (number), `default_branch` (string, default `"main"`).
- Produces outputs: `repository` and `html_url`.

- [ ] **Step 1: Write the failing test `modules/github/component/tests/component.tftest.hcl`**

```hcl
mock_provider "github" {}

variables {
  name               = "api"
  description        = "Orders API"
  topics             = ["java", "orders"]
  owner_team_id      = "4242"
  environments       = { dev = { protected = false }, pro = { protected = true } }
  required_approvals = 1
  archive_on_destroy = true
  writer_app_id      = 5255579
}

run "repository_is_public_and_follows_the_claim" {
  command = plan

  assert {
    condition     = github_repository.this.name == "api" && github_repository.this.visibility == "public"
    error_message = "the repository must be named after the claim and public (Free plan, spec §2.3)"
  }
  assert {
    condition     = github_repository.this.auto_init && github_repository.this.delete_branch_on_merge
    error_message = "auto_init and delete_branch_on_merge must be on (spec §4.4)"
  }
  assert {
    condition     = github_repository.this.archive_on_destroy
    error_message = "archive_on_destroy must follow the platform config"
  }
  assert {
    condition     = tolist(github_repository.this.topics) == tolist(["java", "orders"])
    error_message = "topics must follow the claim"
  }
}

run "archive_on_destroy_can_be_turned_off" {
  command = plan

  variables {
    archive_on_destroy = false
  }

  assert {
    condition     = !github_repository.this.archive_on_destroy
    error_message = "archive_on_destroy must follow the variable (false in idp-claims-e2e)"
  }
}

run "vulnerability_alerts_are_on" {
  command = plan

  assert {
    condition     = github_repository_vulnerability_alerts.this.enabled
    error_message = "vulnerability alerts must be on (spec §4.4)"
  }
}

run "owner_team_maintains_the_repository" {
  command = plan

  assert {
    condition     = github_team_repository.owner.permission == "maintain" && github_team_repository.owner.team_id == "4242"
    error_message = "the owner team must get maintain permission"
  }
}

run "ruleset_requires_prs_and_lets_only_the_writer_bypass" {
  command = plan

  assert {
    condition     = github_repository_ruleset.default_branch.enforcement == "active" && github_repository_ruleset.default_branch.target == "branch"
    error_message = "the ruleset must be an active branch ruleset"
  }
  assert {
    condition     = tolist(github_repository_ruleset.default_branch.conditions[0].ref_name[0].include) == tolist(["~DEFAULT_BRANCH"])
    error_message = "the ruleset must target the default branch"
  }
  assert {
    condition     = length(github_repository_ruleset.default_branch.bypass_actors) == 1 && github_repository_ruleset.default_branch.bypass_actors[0].actor_id == 5255579 && github_repository_ruleset.default_branch.bypass_actors[0].actor_type == "Integration"
    error_message = "only the writer App may bypass (spec §4.4)"
  }
  assert {
    condition     = github_repository_ruleset.default_branch.rules[0].deletion && github_repository_ruleset.default_branch.rules[0].non_fast_forward
    error_message = "deletion and force pushes must be blocked"
  }
  assert {
    condition     = github_repository_ruleset.default_branch.rules[0].pull_request[0].required_approving_review_count == 1
    error_message = "PR approvals must follow github.requiredApprovals"
  }
}

run "protected_environments_require_the_owner_team" {
  command = plan

  assert {
    condition     = length(github_repository_environment.this) == 2
    error_message = "one environment per claimed environment"
  }
  assert {
    condition     = length(github_repository_environment.this["pro"].reviewers) == 1 && contains(tolist(github_repository_environment.this["pro"].reviewers[0].teams), 4242)
    error_message = "a protected environment must require the owner team"
  }
  assert {
    condition     = length(github_repository_environment.this["dev"].reviewers) == 0
    error_message = "an unprotected environment must not require reviewers"
  }
}

run "environments_deploy_only_from_the_default_branch" {
  command = plan

  assert {
    condition     = alltrue([for e in github_repository_environment.this : !e.deployment_branch_policy[0].protected_branches && e.deployment_branch_policy[0].custom_branch_policies])
    error_message = "environments must use a custom branch policy"
  }
  assert {
    condition     = length(github_repository_environment_deployment_policy.default_branch) == 2 && alltrue([for p in github_repository_environment_deployment_policy.default_branch : p.branch_pattern == "main"])
    error_message = "every environment must deploy only from the default branch"
  }
}

run "rejects_out_of_range_approvals" {
  command = plan

  variables {
    required_approvals = 11
  }

  expect_failures = [var.required_approvals]
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"; cd modules/github/component && "$TOFU" init -input=false && "$TOFU" test`
Expected: FAIL. Resources are not defined yet.

- [ ] **Step 3: Write the module**

`modules/github/component/versions.tf`:
```hcl
terraform {
  required_version = ">= 1.12.0"

  required_providers {
    github = {
      source  = "integrations/github"
      version = "6.13.0"
    }
  }
}
```

`modules/github/component/variables.tf`:
```hcl
variable "name" {
  description = "Repository name: the Component claim name."
  type        = string
}

variable "description" {
  description = "Repository description."
  type        = string
  default     = ""
}

variable "topics" {
  description = "Repository topics."
  type        = list(string)
  default     = []
}

variable "owner_team_id" {
  description = "Id of the owner Group's team (output team_id of the github/group module)."
  type        = string
}

variable "environments" {
  description = "GitHub environments to create; protected ones require the owner team's approval."
  type = map(object({
    protected = bool
  }))
  default = {}
}

variable "required_approvals" {
  description = "Approvals a PR to the default branch needs (platform github.requiredApprovals)."
  type        = number

  validation {
    condition     = var.required_approvals >= 0 && var.required_approvals <= 10 && floor(var.required_approvals) == var.required_approvals
    error_message = "required_approvals must be an integer between 0 and 10."
  }
}

variable "archive_on_destroy" {
  description = "Archive instead of delete when the Component claim is removed (platform github.archiveOnDestroy)."
  type        = bool
}

variable "writer_app_id" {
  description = "Id of the writer GitHub App: the only bypass actor of the default-branch ruleset."
  type        = number
}

variable "default_branch" {
  description = "Default branch name; environments deploy only from it."
  type        = string
  default     = "main"
}
```

`modules/github/component/main.tf`:
```hcl
resource "github_repository" "this" {
  name                   = var.name
  description            = var.description
  visibility             = "public"
  auto_init              = true
  delete_branch_on_merge = true
  archive_on_destroy     = var.archive_on_destroy
  topics                 = var.topics
}

resource "github_repository_vulnerability_alerts" "this" {
  repository = github_repository.this.name
  enabled    = true
}

resource "github_team_repository" "owner" {
  team_id    = var.owner_team_id
  repository = github_repository.this.name
  permission = "maintain"
}

resource "github_repository_ruleset" "default_branch" {
  name        = "idp-default-branch"
  repository  = github_repository.this.name
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
      required_approving_review_count = var.required_approvals
    }
  }
}

resource "github_repository_environment" "this" {
  for_each = var.environments

  environment = each.key
  repository  = github_repository.this.name

  dynamic "reviewers" {
    for_each = each.value.protected ? [1] : []
    content {
      teams = [var.owner_team_id]
    }
  }

  deployment_branch_policy {
    protected_branches     = false
    custom_branch_policies = true
  }

  # The team must have access to the repository before it can be a reviewer.
  depends_on = [github_team_repository.owner]
}

resource "github_repository_environment_deployment_policy" "default_branch" {
  for_each = var.environments

  repository     = github_repository.this.name
  environment    = github_repository_environment.this[each.key].environment
  branch_pattern = var.default_branch
}
```

`modules/github/component/outputs.tf`:
```hcl
output "repository" {
  description = "Repository name."
  value       = github_repository.this.name
}

output "html_url" {
  description = "Repository URL."
  value       = github_repository.this.html_url
}
```

- [ ] **Step 4: Run the tests and see them pass**

Run: `TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"; cd modules/github/component && "$TOFU" fmt -check -recursive && "$TOFU" test`
Expected: `Success! 8 passed, 0 failed.` If a nested block's attribute path differs in the 6.13.0 schema, fix the assertion path (never its intent) and record it in the report. If `reviewers[0].teams` is a list instead of a set, `tolist` still works.

- [ ] **Step 5: Commit**

```bash
git add modules/github/component
git commit -m "feat(modules): add github component module with tofu tests"
```

---

### Task 8: Pinned provider lock file

**Files:**
- Create: `lockfiles/lockfiles.go`, `lockfiles/github.terraform.lock.hcl`, `lockfiles/lockfiles_test.go`

**Interfaces:**
- Produces: `var lockfiles.GitHub []byte` (package `github.com/jellalshadows-idp/idp-engine/lockfiles`).

- [ ] **Step 1: Write the failing test `lockfiles/lockfiles_test.go`**

```go
package lockfiles

import (
	"regexp"
	"testing"
)

func TestGitHubLockPinsTheProvider(t *testing.T) {
	s := string(GitHub)
	if !regexp.MustCompile(`provider "registry\.opentofu\.org/integrations/github"`).MatchString(s) {
		t.Fatal("lock file does not lock integrations/github")
	}
	if !regexp.MustCompile(`version\s*=\s*"6\.13\.0"`).MatchString(s) {
		t.Error("lock file does not pin version 6.13.0")
	}
	if n := len(regexp.MustCompile(`"h1:`).FindAllString(s, -1)); n < 4 {
		t.Errorf("lock file has %d h1 hashes; want one per platform (linux/windows/darwin amd64 + darwin arm64)", n)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./lockfiles/`
Expected: FAIL to compile, with `undefined: GitHub`.

- [ ] **Step 3: Generate the lock file**

With the Write tool, create `$HOME/.idp/tmp/lockgen/versions.tf`:
```hcl
terraform {
  required_providers {
    github = {
      source  = "integrations/github"
      version = "6.13.0"
    }
  }
}
```
Then run:
`TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"; cd "$HOME/.idp/tmp/lockgen" && "$TOFU" providers lock -platform=linux_amd64 -platform=windows_amd64 -platform=darwin_amd64 -platform=darwin_arm64 && cp .terraform.lock.hcl "C:/Users/Usuario/idp-engine/lockfiles/github.terraform.lock.hcl"`
Expected: `Success! OpenTofu has updated the lock file.`

- [ ] **Step 4: Write `lockfiles/lockfiles.go`**

```go
// Package lockfiles embeds the OpenTofu dependency lock files the renderer copies
// into each stack, so `tofu init` verifies provider checksums (spec §5.3, §7.5).
// Regenerate with `tofu providers lock` for linux/windows/darwin amd64 and darwin
// arm64 whenever a provider version changes.
package lockfiles

import _ "embed"

// GitHub is the lock file of the GitHub stack (integrations/github).
//
//go:embed github.terraform.lock.hcl
var GitHub []byte
```

- [ ] **Step 5: Run the tests and see them pass**

Run: `go test ./lockfiles/ && go vet ./... && gofmt -l .`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add lockfiles
git commit -m "feat(lockfiles): pin the github provider lock file"
```

---

### Task 9: Render package (pure) and safe tree writer

**Files:**
- Create: `internal/version/version.go`, `internal/render/render.go`, `internal/render/write.go`, `internal/render/render_test.go`, `internal/render/write_test.go`
- Create: `internal/render/testdata/basic/config/platform.yaml`, `.../claims/groups/platform.yaml`, `.../claims/components/api.yaml`, `.../expected/github/main.tf.json`

**Interfaces:**
- Consumes: `claims.Load`, `claims.Model` (Task 5); `lockfiles.GitHub` (Task 8).
- Produces:
  - `version.Version` (var, `"dev"` by default) and `version.Repo` (`"github.com/jellalshadows-idp/idp-engine"`).
  - `render.Options{ModuleRef, ModulesDir string}`, constants `render.TofuVersion` (`"1.12.6"`) and `render.GitHubProviderVersion` (`"6.13.0"`).
  - `func render.Render(m *claims.Model, opts Options) (map[string][]byte, error)`, whose keys are `github/main.tf.json` and `github/.terraform.lock.hcl`.
  - `render.Marker` (`".idp-rendered"`) and `func render.WriteTree(dir string, files map[string][]byte) error`.

- [ ] **Step 1: Write the fixture**

`internal/render/testdata/basic/config/platform.yaml`:
```yaml
apiVersion: idp/v1
kind: Platform
github:
  org: acme
  writerAppId: 5255579
environments:
  dev: {}
  pro:
    protected: true
```

`internal/render/testdata/basic/claims/groups/platform.yaml`:
```yaml
apiVersion: idp/v1
kind: Group
name: platform
description: Platform team
members:
  - user: alice
    role: maintainer
  - user: bob
    role: member
```

`internal/render/testdata/basic/claims/components/api.yaml`:
```yaml
apiVersion: idp/v1
kind: Component
name: api
description: Orders API
owner: group:platform
environments: [dev, pro]
github:
  topics: [java, orders]
```

- [ ] **Step 2: Write the failing tests**

`internal/render/render_test.go`:
```go
package render

import (
	"bytes"
	"flag"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

var update = flag.Bool("update", false, "rewrite golden files")

func loadFixture(t *testing.T, name string) *claims.Model {
	t.Helper()
	m, ds, err := claims.Load(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) > 0 {
		t.Fatalf("fixture %s has diagnostics: %v", name, ds)
	}
	return m
}

func TestRenderGolden(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	got := files["github/main.tf.json"]
	golden := filepath.Join("testdata", "basic", "expected", "github", "main.tf.json")
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("main.tf.json differs from its golden file:\n%s", got)
	}
}

func TestRenderCopiesTheLockFile(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files["github/.terraform.lock.hcl"], lockfiles.GitHub) {
		t.Error("the stack must carry the engine's lock file")
	}
	if !regexp.MustCompile(`version\s*=\s*"` + regexp.QuoteMeta(GitHubProviderVersion) + `"`).Match(lockfiles.GitHub) {
		t.Errorf("lock file does not pin GitHubProviderVersion %s", GitHubProviderVersion)
	}
}

func TestRenderLocalModules(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModulesDir: "../../modules"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(files["github/main.tf.json"])
	for _, want := range []string{`"source": "../../modules/github/group"`, `"source": "../../modules/github/component"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
}

// TestRenderIsDeterministic is spec §8.2's determinism test at the boundary Render
// consumes: any claim order gives identical bytes. (Load's half, sorted output
// whatever the read order, is TestLoadSortsClaimsByName in internal/claims.)
func TestRenderIsDeterministic(t *testing.T) {
	m := loadFixture(t, "basic")
	// Two claims of each kind, so their order can actually vary.
	data := m.Groups[0]
	data.Name = "data"
	m.Groups = append(m.Groups, data)
	web := m.Components[0]
	web.Name, web.Owner = "web", "group:data"
	m.Components = append(m.Components, web)

	want, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 20; i++ {
		shuffled := *m
		shuffled.Groups = append([]claims.Group{}, m.Groups...)
		shuffled.Components = append([]claims.Component{}, m.Components...)
		r.Shuffle(len(shuffled.Groups), func(a, b int) {
			shuffled.Groups[a], shuffled.Groups[b] = shuffled.Groups[b], shuffled.Groups[a]
		})
		r.Shuffle(len(shuffled.Components), func(a, b int) {
			shuffled.Components[a], shuffled.Components[b] = shuffled.Components[b], shuffled.Components[a]
		})
		got, err := Render(&shuffled, Options{ModuleRef: "v1"})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got["github/main.tf.json"], want["github/main.tf.json"]) {
			t.Fatalf("iteration %d: render output depends on claim order", i)
		}
	}
}

func TestRenderKeepsTextLiteral(t *testing.T) {
	m := loadFixture(t, "basic")
	m.Components[0].Description = "Pagos & cobros <beta> — ñ"
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files["github/main.tf.json"]), `"description": "Pagos & cobros <beta> — ñ"`) {
		t.Errorf("description was not rendered literally:\n%s", files["github/main.tf.json"])
	}
}

func TestRenderWithoutClaimsHasNoModules(t *testing.T) {
	m := loadFixture(t, "basic")
	m.Groups, m.Components = nil, nil
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(files["github/main.tf.json"])
	if strings.Contains(s, `"module"`) || !strings.Contains(s, `"owner": "acme"`) {
		t.Errorf("unexpected stack without claims:\n%s", s)
	}
}

func TestRenderNeedsAModuleSource(t *testing.T) {
	if _, err := Render(loadFixture(t, "basic"), Options{}); err == nil {
		t.Error("want an error when neither ModuleRef nor ModulesDir is set")
	}
}
```

`internal/render/write_test.go`:
```go
package render

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func sampleFiles() map[string][]byte {
	return map[string][]byte{"github/main.tf.json": []byte("{}\n"), "github/.terraform.lock.hcl": []byte("# lock\n")}
}

func TestWriteTreeCreatesTheTree(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rendered")
	if err := WriteTree(dir, sampleFiles()); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"github/main.tf.json", "github/.terraform.lock.hcl", Marker} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestWriteTreeReplacesAPreviousRender(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rendered")
	if err := WriteTree(dir, map[string][]byte{"aws/dev/stale.tf.json": []byte("{}")}); err != nil {
		t.Fatal(err)
	}
	if err := WriteTree(dir, sampleFiles()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "aws", "dev", "stale.tf.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stale file survived a re-render: %v", err)
	}
}

func TestWriteTreeRefusesUnmarkedDirectory(t *testing.T) {
	dir := t.TempDir()
	precious := filepath.Join(dir, "precious.txt")
	if err := os.WriteFile(precious, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteTree(dir, sampleFiles()); err == nil {
		t.Fatal("want a refusal for a non-empty directory without the render marker")
	}
	if _, err := os.Stat(precious); err != nil {
		t.Errorf("a refused write deleted data: %v", err)
	}
}

func TestWriteTreeAcceptsAnEmptyDirectory(t *testing.T) {
	if err := WriteTree(t.TempDir(), sampleFiles()); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 3: Run them and see them fail**

Run: `go test ./internal/render/`
Expected: FAIL to compile, with `undefined: Render`.

- [ ] **Step 4: Implement `internal/version/version.go`**

```go
// Package version identifies this build of idp.
package version

// Version is the release tag of this build, set at release time with
// -ldflags "-X github.com/jellalshadows-idp/idp-engine/internal/version.Version=v0.1.0".
// Rendered module sources pin it (spec §5.3).
var Version = "dev"

// Repo is the engine's module path; rendered module sources point at it.
const Repo = "github.com/jellalshadows-idp/idp-engine"
```

- [ ] **Step 5: Implement `internal/render/render.go`**

```go
// Package render turns a validated claims model into OpenTofu stacks (spec §5).
// Render is pure: the same model and options always produce the same bytes.
package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/version"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

// Pinned versions the rendered stacks require (spec §7.5).
const (
	TofuVersion           = "1.12.6"
	GitHubProviderVersion = "6.13.0"
)

// stateFile is the GitHub stack's local backend path, relative to rendered/github
// in a checkout of the wet branch (spec §5.4).
const stateFile = "../../tfstate/github.tfstate"

// Options decides where module sources point.
type Options struct {
	// ModuleRef is the idp-engine git ref module sources pin: the renderer's own
	// release tag (spec §5.3), or a commit for pre-release testing.
	ModuleRef string
	// ModulesDir replaces git sources with local paths, relative to the github stack
	// directory and starting with ./ or ../. CI uses it to validate a render before
	// its ref exists.
	ModulesDir string
}

// Render returns the rendered files, keyed by slash-separated path relative to the
// output root.
func Render(m *claims.Model, opts Options) (map[string][]byte, error) {
	if opts.ModuleRef == "" && opts.ModulesDir == "" {
		return nil, errors.New("render: set ModuleRef or ModulesDir")
	}
	stack, err := marshal(githubStack(m, opts))
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"github/main.tf.json":        stack,
		"github/.terraform.lock.hcl": lockfiles.GitHub,
	}, nil
}

func githubStack(m *claims.Model, opts Options) map[string]any {
	p := m.Platform
	modules := map[string]any{}
	for _, g := range m.Groups {
		members := make([]map[string]any, 0, len(g.Members))
		for _, mem := range g.Members {
			members = append(members, map[string]any{"user": mem.User, "role": mem.Role})
		}
		modules["group_"+g.Name] = map[string]any{
			"source":      moduleSource(opts, "group"),
			"name":        g.Name,
			"description": g.Description,
			"members":     members,
		}
	}
	for _, c := range m.Components {
		envs := map[string]any{}
		for _, e := range c.Environments {
			envs[e] = map[string]any{"protected": p.Environments[e].Protected}
		}
		modules["component_"+c.Name] = map[string]any{
			"source":             moduleSource(opts, "component"),
			"name":               c.Name,
			"description":        c.Description,
			"topics":             append([]string{}, c.GitHub.Topics...),
			"owner_team_id":      fmt.Sprintf("${module.group_%s.team_id}", c.OwnerGroup()),
			"environments":       envs,
			"required_approvals": p.RequiredApprovals(),
			"archive_on_destroy": p.ArchiveOnDestroy(),
			"writer_app_id":      p.GitHub.WriterAppID,
		}
	}
	doc := map[string]any{
		"terraform": map[string]any{
			"required_version": TofuVersion,
			"required_providers": map[string]any{
				"github": map[string]any{"source": "integrations/github", "version": GitHubProviderVersion},
			},
			"backend": map[string]any{"local": map[string]any{"path": stateFile}},
			// The code says THAT state and plans are encrypted; TF_ENCRYPTION says HOW (spec §7.2).
			"encryption": map[string]any{
				"state": map[string]any{"enforced": true},
				"plan":  map[string]any{"enforced": true},
			},
		},
		"provider": map[string]any{"github": map[string]any{"owner": p.GitHub.Org}},
	}
	if len(modules) > 0 {
		doc["module"] = modules
	}
	return doc
}

func moduleSource(opts Options, module string) string {
	if opts.ModulesDir != "" {
		return strings.TrimSuffix(opts.ModulesDir, "/") + "/github/" + module
	}
	return fmt.Sprintf("git::https://%s.git//modules/github/%s?ref=%s", version.Repo, module, opts.ModuleRef)
}

// marshal encodes with sorted keys (encoding/json sorts map keys), two-space
// indentation, a trailing newline and no HTML escaping, so text stays literal.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
```

- [ ] **Step 6: Implement `internal/render/write.go`**

```go
package render

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Marker is written into every render output directory. WriteTree replaces a
// non-empty directory only if it carries the marker, so a mistyped --out can never
// wipe unrelated data.
const Marker = ".idp-rendered"

// WriteTree replaces dir with files. dir must not exist, be empty, or hold a
// previous render (it contains Marker).
func WriteTree(dir string, files map[string][]byte) error {
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case len(entries) > 0:
		if _, err := os.Stat(filepath.Join(dir, Marker)); err != nil {
			return fmt.Errorf("%s is not empty and is not a previous render (no %s); refusing to replace it", dir, Marker)
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for p, data := range files {
		target := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, Marker), []byte("Generated by idp render. This directory is replaced on every render.\n"), 0o644)
}
```

- [ ] **Step 7: Generate the golden and check it against this plan**

Run: `go test ./internal/render/ -run TestRenderGolden -update`
Then read `internal/render/testdata/basic/expected/github/main.tf.json`. It must be **exactly**:

```json
{
  "module": {
    "component_api": {
      "archive_on_destroy": true,
      "description": "Orders API",
      "environments": {
        "dev": {
          "protected": false
        },
        "pro": {
          "protected": true
        }
      },
      "name": "api",
      "owner_team_id": "${module.group_platform.team_id}",
      "required_approvals": 1,
      "source": "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/github/component?ref=v0.0.0-test",
      "topics": [
        "java",
        "orders"
      ],
      "writer_app_id": 5255579
    },
    "group_platform": {
      "description": "Platform team",
      "members": [
        {
          "role": "maintainer",
          "user": "alice"
        },
        {
          "role": "member",
          "user": "bob"
        }
      ],
      "name": "platform",
      "source": "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/github/group?ref=v0.0.0-test"
    }
  },
  "provider": {
    "github": {
      "owner": "acme"
    }
  },
  "terraform": {
    "backend": {
      "local": {
        "path": "../../tfstate/github.tfstate"
      }
    },
    "encryption": {
      "plan": {
        "enforced": true
      },
      "state": {
        "enforced": true
      }
    },
    "required_providers": {
      "github": {
        "source": "integrations/github",
        "version": "6.13.0"
      }
    },
    "required_version": "1.12.6"
  }
}
```

If it differs, the code is wrong. Fix the code, never the golden.

- [ ] **Step 8: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok` for every package.

- [ ] **Step 9: Commit**

```bash
git add internal/version internal/render
git commit -m "feat(render): render the github stack deterministically with a safe tree writer"
```

---

### Task 10: CLI `idp validate` and `idp render`

**Files:**
- Create: `internal/cli/claims.go`, `internal/cli/claims_test.go`
- Modify: `internal/cli/cli.go` (usage text and dispatch)

**Interfaces:**
- Consumes: `claims.Load`, `claims.Diagnostic` (Tasks 3 and 5); `render.Render`, `render.WriteTree`, `render.Options` (Task 9); `version.Version`.
- Produces the CLI contract:
  - `idp validate [--dir DIR]` exits 0 when valid, 1 on diagnostics, 2 on a usage error.
  - `idp render [--dir DIR] [--out DIR] [--module-ref REF] [--modules-dir PATH]` exits 0, 1 on diagnostics or failure, 2 on a usage error.
  - When `GITHUB_ACTIONS=true`, every diagnostic is also printed to stdout as a GitHub annotation.

- [ ] **Step 1: Write the failing test `internal/cli/claims_test.go`**

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

const (
	cliPlatform  = "apiVersion: idp/v1\nkind: Platform\ngithub:\n  org: acme\n  writerAppId: 5255579\nenvironments:\n  dev: {}\n"
	cliGroup     = "apiVersion: idp/v1\nkind: Group\nname: platform\nmembers:\n  - user: alice\n    role: maintainer\n"
	cliComponent = "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nenvironments: [dev]\n"
)

// writeFileAll is writeFile (bootstrap_test.go) plus the parent directories.
func writeFileAll(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, content)
}

func claimsRepo(t *testing.T, component string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range map[string]string{
		"config/platform.yaml":        cliPlatform,
		"claims/groups/platform.yaml": cliGroup,
		"claims/components/api.yaml":  component,
	} {
		writeFileAll(t, filepath.Join(dir, filepath.FromSlash(rel)), content)
	}
	return dir
}

func envOf(m map[string]string) Env { return func(k string) string { return m[k] } }

func TestValidate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"validate", "--dir", claimsRepo(t, cliComponent)}, &stdout, &stderr, noEnv); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "validate: ok (1 group(s), 1 component(s))") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestValidateReportsProblemsAndAnnotations(t *testing.T) {
	bad := strings.Replace(cliComponent, "[dev]", "[staging]", 1)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"validate", "--dir", claimsRepo(t, bad)}, &stdout, &stderr, envOf(map[string]string{"GITHUB_ACTIONS": "true"}))
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `claims/components/api.yaml:5: environment "staging" is not defined in config/platform.yaml`) || !strings.Contains(stderr.String(), "validate: 1 problem(s)") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "::error file=claims/components/api.yaml,line=5::") {
		t.Errorf("stdout = %q, want a GitHub annotation", stdout.String())
	}
}

func TestValidateWithoutActionsPrintsNoAnnotations(t *testing.T) {
	bad := strings.Replace(cliComponent, "[dev]", "[staging]", 1)
	var stdout, stderr bytes.Buffer
	Run([]string{"validate", "--dir", claimsRepo(t, bad)}, &stdout, &stderr, noEnv)
	if strings.Contains(stdout.String(), "::error") {
		t.Errorf("annotations outside GitHub Actions: %q", stdout.String())
	}
}

func TestRenderWritesTheStack(t *testing.T) {
	out := filepath.Join(t.TempDir(), "rendered")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", "--dir", claimsRepo(t, cliComponent), "--out", out, "--module-ref", "v9.9.9"}, &stdout, &stderr, noEnv)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(out, "github", "main.tf.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "?ref=v9.9.9") {
		t.Errorf("module sources do not pin the ref:\n%s", data)
	}
	if !strings.Contains(stdout.String(), "render: wrote ") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRenderDevBuildNeedsASource(t *testing.T) {
	if version.Version != "dev" {
		t.Skip("only meaningful for development builds")
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", "--dir", claimsRepo(t, cliComponent), "--out", filepath.Join(t.TempDir(), "r")}, &stdout, &stderr, noEnv)
	if code != 2 || !strings.Contains(stderr.String(), "pass --module-ref or --modules-dir") {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestRenderLocalModulesAreRelative(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")
	modules := filepath.Join(root, "engine", "modules")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", "--dir", claimsRepo(t, cliComponent), "--out", out, "--modules-dir", modules}, &stdout, &stderr, noEnv)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(out, "github", "main.tf.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"source": "../../engine/modules/github/group"`) {
		t.Errorf("local module source not relative to the stack dir:\n%s", data)
	}
}

func TestRenderRefusesForeignOutDir(t *testing.T) {
	out := t.TempDir()
	writeFile(t, filepath.Join(out, "precious.txt"), "keep me")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", "--dir", claimsRepo(t, cliComponent), "--out", out, "--module-ref", "v1"}, &stdout, &stderr, noEnv)
	if code != 1 || !strings.Contains(stderr.String(), "refusing to replace it") {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(out, "precious.txt")); err != nil {
		t.Errorf("render deleted data: %v", err)
	}
}

func TestRenderStopsOnDiagnostics(t *testing.T) {
	out := filepath.Join(t.TempDir(), "rendered")
	bad := strings.Replace(cliComponent, "[dev]", "[staging]", 1)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"render", "--dir", claimsRepo(t, bad), "--out", out, "--module-ref", "v1"}, &stdout, &stderr, noEnv); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("render wrote output despite diagnostics")
	}
}

func TestClaimsUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"validate", "extra"}, {"render", "extra"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, noEnv); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
```

`writeFile(t, path, content)` (in `bootstrap_test.go`; it writes with mode 0600 and does not create directories) and `noEnv` (in `cli_test.go`) already exist from Phase 0. `fail(stderr, err) int` exists in `bootstrap.go`. Do not redefine any of them.

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/cli/`
Expected: FAIL. `validate` and `render` are unknown commands (exit 2).

- [ ] **Step 3: Implement `internal/cli/claims.go`**

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

func runValidate(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", ".", "claims repo root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp validate: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	m, ok := loadClaims(*dir, stdout, stderr, env)
	if !ok {
		return 1
	}
	fmt.Fprintf(stdout, "validate: ok (%d group(s), %d component(s))\n", len(m.Groups), len(m.Components))
	return 0
}

func runRender(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", ".", "claims repo root")
	out := fs.String("out", "rendered", "output directory (created, or replaced if it holds a previous render)")
	ref := fs.String("module-ref", "", "idp-engine git ref that module sources pin (default: this build's version)")
	modulesDir := fs.String("modules-dir", "", "use local module sources from this directory (validation only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp render: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	opts := render.Options{ModuleRef: *ref}
	if *modulesDir != "" {
		rel, err := relativeModulesDir(*out, *modulesDir)
		if err != nil {
			fmt.Fprintln(stderr, "idp render:", err)
			return 2
		}
		opts.ModulesDir = rel
	}
	if opts.ModuleRef == "" && opts.ModulesDir == "" {
		if version.Version == "dev" {
			fmt.Fprintln(stderr, "idp render: this is a development build; pass --module-ref or --modules-dir")
			return 2
		}
		opts.ModuleRef = version.Version
	}
	m, ok := loadClaims(*dir, stdout, stderr, env)
	if !ok {
		return 1
	}
	files, err := render.Render(m, opts)
	if err != nil {
		return fail(stderr, err)
	}
	if err := render.WriteTree(*out, files); err != nil {
		return fail(stderr, err)
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintf(stdout, "render: wrote %s\n", path.Join(filepath.ToSlash(*out), p))
	}
	return 0
}

// loadClaims loads and validates a claims repo and prints every diagnostic: always
// to stderr, and also to stdout as a GitHub annotation inside GitHub Actions.
func loadClaims(dir string, stdout, stderr io.Writer, env Env) (*claims.Model, bool) {
	m, diags, err := claims.Load(dir)
	if err != nil {
		fmt.Fprintln(stderr, "idp:", err)
		return nil, false
	}
	inActions := env("GITHUB_ACTIONS") == "true"
	for _, d := range diags {
		if inActions {
			fmt.Fprintln(stdout, d.Annotation())
		}
		fmt.Fprintln(stderr, d)
	}
	if len(diags) > 0 {
		fmt.Fprintf(stderr, "validate: %d problem(s)\n", len(diags))
		return nil, false
	}
	return m, true
}

// relativeModulesDir expresses modulesDir relative to <out>/github, the directory
// OpenTofu resolves local module sources from. Local sources must start with ./ or ../.
func relativeModulesDir(out, modulesDir string) (string, error) {
	stack, err := filepath.Abs(filepath.Join(out, "github"))
	if err != nil {
		return "", err
	}
	mods, err := filepath.Abs(modulesDir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(stack, mods)
	if err != nil {
		return "", fmt.Errorf("--modules-dir must be on the same volume as --out: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "../") && !strings.HasPrefix(rel, "./") {
		rel = "./" + rel
	}
	return rel, nil
}
```

- [ ] **Step 4: Wire it into `internal/cli/cli.go`**

In the `usage` const, add these two lines above `bootstrap`:

```
  validate    Validate a claims repo (schema + semantic checks)
  render      Render a claims repo into OpenTofu stacks
```

In `Run`, add before the `bootstrap` case:

```go
	case "validate":
		return runValidate(args[1:], stdout, stderr, env)
	case "render":
		return runRender(args[1:], stdout, stderr, env)
```

- [ ] **Step 5: Run the tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok` for every package.

- [ ] **Step 6: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): add idp validate and idp render"
```

---

### Task 11: CI — module tests and a real render smoke test

**Files:**
- Modify: `.github/workflows/ci.yaml`

**Interfaces:**
- Consumes: modules (Tasks 6–7), `idp render` (Task 10), the render fixture (Task 9).
- Produces: CI jobs `modules` (matrix) and `render-smoke`.

- [ ] **Step 1: Add the jobs**

Append under `jobs:` in `.github/workflows/ci.yaml`:

```yaml
  modules:
    runs-on: ubuntu-24.04
    permissions:
      contents: read
    strategy:
      fail-fast: false
      matrix:
        module: [github/group, github/component]
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4 # v2.0.2
        with:
          tofu_version: 1.12.6
          tofu_wrapper: false
      - name: tofu fmt and test
        working-directory: modules/${{ matrix.module }}
        run: |
          tofu fmt -check -recursive
          tofu init -input=false
          tofu test

  render-smoke:
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
      - uses: opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4 # v2.0.2
        with:
          tofu_version: 1.12.6
          tofu_wrapper: false
      - name: render the fixture with local module sources
        run: go run ./cmd/idp render --dir internal/render/testdata/basic --out "$RUNNER_TEMP/rendered" --modules-dir modules
      - name: tofu validate the rendered stack (proves the JSON syntax, lock file and modules)
        working-directory: ${{ runner.temp }}/rendered/github
        env:
          TF_ENCRYPTION: |
            key_provider "pbkdf2" "ci" {
              passphrase = "render-smoke-only-not-a-secret"
            }
            method "aes_gcm" "ci" {
              keys = key_provider.pbkdf2.ci
            }
            state {
              method = method.aes_gcm.ci
            }
            plan {
              method = method.aes_gcm.ci
            }
        run: |
          tofu init -backend=false -input=false
          tofu validate
```

- [ ] **Step 2: Validate the workflow locally**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`
Expected: no output (the clean exit). zizmor runs in CI.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yaml
git commit -m "ci: run module tests and a rendered-stack validation"
```

---

### Task 12: Claims reference documentation

**Files:**
- Create: `docs/claims.md`
- Modify: `README.md` (link)

**Interfaces:**
- Consumes: the schemas (Task 2), the diagnostics (Tasks 3–5), the CLI (Task 10).

- [ ] **Step 1: Write `docs/claims.md`**

````markdown
# Claims reference

A claims repo describes the desired state of a GitHub org. `idp validate` checks it, and `idp render` turns it into OpenTofu stacks. The JSON Schemas in [`schemas/`](../schemas) are the source of truth for every field ([ADR-0015](adr/0015-claim-parsing-and-validation.md)).

## Layout

```
config/platform.yaml          # one per claims repo
claims/groups/<name>.yaml     # kind: Group
claims/components/<name>.yaml # kind: Component
```

- The directory decides the kind. A Group file under `claims/components/` fails validation.
- Each file holds exactly one YAML document, its `name` must equal the file name, and the file must end in `.yaml`. Dotfiles such as `.gitkeep` are ignored.
- Because `name` equals the file name and names are lowercase, names are unique within their kind (spec §4.7). A Group and a Component may share a name.
- Workspace claims (`claims/workspaces/<env>/`) arrive in Phase 3. Until then that directory is an error.

## Editor completion

Put this first line in a claim file to get completion and inline errors in editors with the YAML language server:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jellalshadows-idp/idp-engine/main/schemas/component.json
```

Use `group.json` or `platform.json` for the other kinds. Pin a release tag instead of `main` once one exists.

## Platform (`config/platform.yaml`)

| Field | Required | Default | Meaning |
|---|---|---|---|
| `github.org` | yes | | The GitHub org |
| `github.writerAppId` | yes | | The writer App's id; the only bypass actor of every Component's default-branch ruleset |
| `github.archiveOnDestroy` | no | `true` | Archive (not delete) a repo when its Component claim is removed |
| `github.requiredApprovals` | no | `1` | PR approvals on Component repos (0–10) |
| `naming.requiredPrefix` | no | | Every claim name must start with it (e.g. `e2e-` in `idp-claims-e2e`) |
| `naming.reservedPrefixes` | no | | No claim name may start with these (e.g. `[e2e-, spike-]` in `idp-claims`) |
| `environments.<env>` | yes (≥ 1) | | Env names match `^[a-z][a-z0-9]{0,9}$` |
| `environments.<env>.protected` | no | `false` | Protected envs require the owner team's approval |
| `environments.<env>.aws` | no | | `accountId` and `region`; used from Phase 2 |
| `modules.allowedSources` | no | | Allowed Workspace module prefixes; used from Phase 3 |

## Group (`claims/groups/<name>.yaml`)

```yaml
apiVersion: idp/v1
kind: Group
name: platform
description: Platform team   # optional
members:
  - user: alice            # GitHub login; each login at most once (case-insensitive)
    role: maintainer       # maintainer | member
```

A Group renders a closed GitHub team with these memberships. Adding someone who is not yet an org member sends them an org invitation; Phase 0 measured that a pending invitation causes no plan drift.

## Component (`claims/components/<name>.yaml`)

```yaml
apiVersion: idp/v1
kind: Component
name: api                  # = repository name
description: Orders API    # optional
owner: group:platform      # must name an existing Group
environments: [dev, pro]   # must exist in config/platform.yaml
github:
  topics: [java, orders]   # optional; lowercase, ≤ 20
```

A Component renders:
- a public repository (auto-init, delete branch on merge, vulnerability alerts on);
- `maintain` access for the owner team;
- a default-branch ruleset that requires PRs, blocks force pushes and deletion, and lets only the writer App bypass;
- one environment per entry, each deploying only from `main`. Protected environments require the owner team.

The `aws` and `features` fields arrive in Phases 2 and 4. Until then they fail validation.

## Validation

`idp validate --dir <claims repo>` reports every problem at once as `file:line: message`. Inside GitHub Actions (`GITHUB_ACTIONS=true`) it also prints GitHub annotations, so errors show inline on the PR. It exits 0 when valid, 1 with problems, and 2 on a usage error.

## Rendering

`idp render --dir <claims repo> --out rendered` writes:

- `rendered/github/main.tf.json`: the provider, the local backend at `../../tfstate/github.tfstate`, enforced state and plan encryption, and one module call per claim, pinned to the engine's release tag;
- `rendered/github/.terraform.lock.hcl`: the pinned provider checksums;
- `rendered/.idp-rendered`: a marker. `--out` must not exist, be empty, or hold a previous render, so a mistyped path can never wipe unrelated data.

Development builds must pass `--module-ref <ref>`, or `--modules-dir <path>` to validate against local modules.
````

- [ ] **Step 2: Link it from `README.md`**

Under the `- Decisions:` line, add: `- Claims reference: [docs/claims.md](docs/claims.md)`

- [ ] **Step 3: Commit**

```bash
git add docs/claims.md README.md
git commit -m "docs: add the claims reference"
```

---

### Task 13: Ship — PR, CI, merge, execution log

**Files:**
- Create: `docs/phases/phase-1a.md`
- Modify: `README.md` (status line)

**Interfaces:**
- Consumes: everything above.

- [ ] **Step 1: Push and open the PR**

```bash
git push -u origin feat/phase-1a-render
gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-1a-render --title "feat: phase 1a claims validation and github stack render" --body "Implements docs/superpowers/plans/2026-10-09-phase-1a-claims-render.md (spec amendment A1, ADR-0015)."
```

- [ ] **Step 2: Wait for CI**

Run: `gh pr checks --repo jellalshadows-idp/idp-engine --watch`
Expected: `go`, `workflows`, `modules (github/group)`, `modules (github/component)` and `render-smoke` all pass. If `render-smoke` fails on the JSON syntax of `required_providers`, `backend` or `encryption`, that is the unverified assumption this job exists to catch. Fix the renderer's JSON shape (and the golden), not the job.

- [ ] **Step 3: Merge**

Run: `gh pr merge --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`

- [ ] **Step 4: Write `docs/phases/phase-1a.md` and update the README status**

Write the execution log in the same style as `docs/phases/phase-0.md`:
- summary;
- a table of tasks with commits and review outcomes;
- every ruling, with what, why and cost if wrong;
- every deferred finding with its final disposition (fixed / Phase 1b–1c / won't fix with reason);
- the CI evidence, including the render-smoke result that validated the JSON syntax.

In `README.md`, replace the whole `> **Status:** …` line with:

```markdown
> **Status:** Phase 1a is complete: `idp validate` and `idp render` turn a claims repo into a validated GitHub stack, backed by the tested `github/group` and `github/component` modules; see the [phase 1a log](docs/phases/phase-1a.md). Phase 1b (pipelines) is next, so nothing is reconciled automatically yet.
```

- [ ] **Step 5: Commit (doc-only, straight to main) and push**

```bash
git add docs/phases/phase-1a.md README.md
git commit -m "docs: add the phase 1a execution log"
git push
```
