# Phase 2a — The AWS Model of the Platform and the Component Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the claims model its AWS half without rendering any of it. `internal/naming` computes the deterministic AWS identities of spec §5.5; the Platform model reads each environment's `aws` account and region; the Component schema and model accept `aws.registry`; validation requires a registry Component to list at least one environment and every one of them to have an `aws` block. The renderer stays GitHub-only, and a test pins that an `aws` part leaves the GitHub render unchanged, byte for byte.

**Architecture:**
- **`internal/naming` is pure over plain values.** Five functions of component and environment names, account IDs and regions, plus the OIDC issuer host. It imports nothing from the engine and returns no errors. Its tests read the name patterns from the embedded schemas and prove that the longest legal names fit IAM's and ECR's rules, so a schema change that would break an AWS name fails there first.
- **The Platform config stays in `internal/claims`** (Phase 1a ruling R-3.3, amendment A4). `Environment` gains `AWS *AWS`, which is nil when the environment has no `aws` block. No `internal/config` is extracted.
- **The Component** gains `AWS.Registry`; the schema accepts `aws: {registry: boolean}` and still rejects `features`. A new semantic check (amendment A10, spec §4.7) reports a registry Component with no environment, and each of its environments that has no `aws` block.
- **The renderer is untouched.** A fixture with `aws.registry` renders exactly `basic`'s golden file.

**Tech Stack:** Go 1.26 (standard library plus the existing `go.yaml.in/yaml/v3` and `github.com/santhosh-tekuri/jsonschema/v6`; no new dependencies). No OpenTofu, module or workflow change.

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §3.3, §4.4, §4.6, §4.7, §5.5, §6, §10 and §11. Context: `docs/phases/phase-1e.md` ("Carried into Phase 2", "Owner notes"), `docs/phases/phase-1a.md` (ruling R-3.3, the Task 2 minor), ADR-0006, ADR-0013, ADR-0015.

## Phase 2 split (owner decision, 2026-11-03)

Phase 2 is delivered as six plans, each written after the previous one ships, as Phase 1 was (spec §10). The local, low-risk Go work comes first; floci is proven on the current OpenTofu before anything depends on it; the workflow change is isolated in its own plan.

| Plan | Delivers |
|---|---|
| **2a (this plan)** | The AWS model of the Platform and the Component, `internal/naming`, the schema and semantic validation, spec amendment A10 |
| 2b | The deterministic env variables (`AWS_ROLE_ARN`, `ECR_REPOSITORY`, `AWS_REGION`) in `modules/github/component`, rendered from `internal/naming` |
| 2c | `modules/aws/{baseline,component}` (the OIDC provider; the ECR repository and the CI role), the AWS lock file and the floci CI job (spec §8.4) |
| 2d | Rendering the AWS stacks (`aws/<env>/_baseline`, `aws/<env>/components/<name>`) |
| 2e | The workflows' AWS plan and apply against floci |
| 2f | The E2E env-variable checks, the `0.2.0` release and the Phase 2 log |

## Decisions taken while planning

The executor copies these into the ledger as rulings before Task 1. D-1 and D-2 are the owner's (2026-11-03); D-3 onwards were decided while planning, from verified facts.

**Owner decisions**

- **D-1 (owner). Phase 2 is split** into 2a to 2f (table above), one plan per sub-phase, each written after the previous one ships. 2a ships no release; `0.2.0` is 2f's. *Cost if wrong:* six review cycles instead of fewer, each of them smaller.
- **D-2 (owner). floci only, no real AWS account** (spec §2.2, §7.6). The pipelines will authenticate to the emulator with fake keys (spec §5.6). A real `sts:AssumeRoleWithWebIdentity`, which ADR-0013 left to the Phase 2 floci tests, is out of scope for v1; Task 1 records that in amendment A10 (§11, risk 4) and in ADR-0013. *Cost if wrong:* going live needs an account and the OIDC credentials step that spec §5.6 already describes; claims and renders do not change.

**Decided while planning (verified)**

- **D-3. No `internal/config`.** R-3.3 allows the extraction "when `internal/naming` needs the Platform without the claims". It does not: naming takes plain strings (component and environment names, account IDs, regions), so it imports neither `internal/claims` nor a config package, and the renderer passes it the values it already holds. Amendment A10 replaces `internal/config` in spec §3.3's dependency column. *Cost if wrong:* a later sub-phase that wants a Platform-typed API imports `internal/claims` (amendment A4 allows it) or extracts the package then; a small move either way.
- **D-4. The scope of `internal/naming` is exactly the AWS identities of spec §4.4 and §5.5:** `RepositoryName`, `RoleName`, `RoleARN`, `RegistryURL`, `OIDCProviderARN` and the constant `OIDCProviderHost`, in the `aws` partition and with the `amazonaws.com` host, as the spec writes them. Not in 2a: the trust policy's subject `repo:<org>/<component>:environment:<env>` and audience `sts.amazonaws.com`, which only the AWS module uses (2c, asserted by its `tofu test`, spec §8.3); Terraform addresses and AWS stack paths, which ADR-0006 puts in this package and which move there when 2d renders the AWS stacks (`group_<name>` and `component_<name>` stay in `internal/render` until then). *Cost if wrong:* a few functions added in 2c or 2d.
- **D-5. AWS's name limits are guaranteed by the schemas and proven by naming's tests, not checked at run time.** The AWS API references (read 2026-11-03): IAM `CreateRole` takes a `RoleName` of 1 to 64 characters matching `[\w+=,.@-]+`, unique without regard to case; ECR `CreateRepository` takes a `repositoryName` of 2 to 256 characters matching `[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(\/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*` (lowercase only; runs of hyphens allowed). Component names (`^[a-z][a-z0-9-]{0,38}[a-z0-9]$`) and environment names (`^[a-z][a-z0-9]{0,9}$`) are lowercase, so every legal name is valid for both, and the role name is at most 54 characters (spec §5.5). The tests read those two patterns from `schemas/*.json` instead of copying them. *Cost if wrong:* none at run time; a schema change that breaks the bound fails `TestNamesFitAWSLimits` before it ships.
- **D-6. The Platform model:** `Environment.AWS *AWS` and `AWS{AccountID, Region string}`; nil means no `aws` block (the schema requires both fields inside one). The account ID stays a string, which keeps its leading zeros. An unquoted ID is a YAML number (`go.yaml.in/yaml/v3` reads `000000000001` with `strconv.ParseInt(plain, 0, 64)`, so as octal 1), and the existing schema rejects it at its line; Task 3 pins that and `docs/claims.md` says to quote it. *Cost:* none.
- **D-7. The Component schema and model:** `aws` is an object with one optional boolean `registry` (default `false`) and `additionalProperties: false`; the model is `AWS struct { Registry bool }`, anonymous like `GitHub`. `aws: {}` and `registry: false` are valid and ask for nothing, because the AWS part is "rendered only when `aws.registry: true`" (spec §4.4). `features` stays rejected, pinned by a test in each schema suite. *Cost if wrong:* rejecting `aws: {}` later would break claims that use it.
- **D-8. The semantic check and its wording.** The spec gives no wording; amendment A10 adds the rule to §4.7. The messages, verbatim:
  - `environment "<env>" needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true`, one per such environment, at that environment's line in `environments`, like the existing "is not defined" diagnostic;
  - `aws.registry needs at least one environment: the registry and its CI role are created per environment`, at the `aws.registry` line.

  An environment missing from `config/platform.yaml` reports only "is not defined", never both. The per-environment check runs only when the platform config decoded, like the existing one; the empty-list check needs no platform config. *Cost if wrong:* wording only.
- **D-9. `aws.registry: true` with no environments is an error, not a silent no-op.** Every AWS resource of the part is per environment (spec §4.4), so such a claim asks for a registry and would get none. Starting strict is the reversible choice: relaxing a rule later never breaks a valid claim, while tightening one does. *Cost if wrong:* a claim lists an environment before it turns the registry on.
- **D-10. The renderer stays GitHub-only.** `TestRenderIgnoresTheAWSPart` renders a fixture that is `basic` plus `aws.registry` and the environments' `aws` blocks, and requires `basic`'s golden file byte for byte and only the github stack. It pins 2a's interim contract: 2b (the env variables) and 2d (the AWS stacks) change it on purpose. *Cost:* none.
- **D-11. Not validated in 2a: two environments that share an `accountId`.** With floci every job runs its own emulator, so nothing collides in v1; on real AWS the per-account OIDC provider and the repository, unique per account and region, would. 2c, which decides the baseline's lifecycle, rules on it. *Cost if wrong:* rejecting it later breaks a config that shares an account; none does (production uses three IDs).
- **D-12. No new ADR.** ADR-0006 (identities computed by `internal/naming`) and ADR-0015 (later phases extend the schemas and add their semantic checks) already decide 2a; amendment A10 records the rule, and ADR-0013 gets amended sentences for D-2. *Cost if wrong:* one ADR later.
- **D-13. Amendment A10 covers what 2a makes stale or adds, and nothing else:** §3.3 (naming's dependency, and the sentence after the units), §4.7 (the AWS check), §6 ("Until Phase 2" becomes Phases 2d and 2e), §10 (the Phase 2 split), §11 risk 4 (D-2). §4.4, §4.6 and §5.5 are not stale and stay as they are; `docs/pipelines.md` ("Only the `github` stack is reconciled for now") stays true until 2e. *Cost:* none.
- **D-14. Phase 1a's Task 2 minor, AWS part.** The platform schema's account ID and region boundaries get tests in Task 3, now that the `aws` block gates real claims; name length and case are pinned through naming's tests (Task 2). These tests pin existing schema behavior, so they pass before and after the change. The minor's other items (logins, prefixes, the topics cap, the prefixes' `maxLength`) stay where Phase 1a left them. *Cost:* none.
- **D-15. No release in 2a.** After the merge, release-please's PR on `main` is expected to propose `0.2.0` (a `feat` on `0.1.1`, with `bump-minor-pre-major` and without `bump-patch-for-minor-pre-major` in `release-please-config.json`). It stays open until 2f: a release in between would accept `aws.registry: true` claims that render nothing. While it is open, every push to `main` re-runs the harness on it (ADR-0028), about 20 minutes each. *Cost if wrong:* if release-please proposes another version, the rule is the same: do not merge it.

## Global Constraints

**Names** (copy verbatim)
- Branch `feat/phase-2a-aws-model`; one PR, merged with a merge commit.
- Package `internal/naming`, files `naming.go` and `naming_test.go`: `const OIDCProviderHost = "token.actions.githubusercontent.com"`; `func RepositoryName(component string) string`; `func RoleName(component, env string) string`; `func RoleARN(accountID, component, env string) string`; `func RegistryURL(accountID, region, component string) string`; `func OIDCProviderARN(accountID string) string`.
- `internal/claims`: `type AWS struct { AccountID string; Region string }` (YAML `accountId`, `region`), the field `Environment.AWS *AWS` (YAML `aws`), and `Component.AWS.Registry bool` (YAML `aws.registry`).
- The two diagnostics of D-8, verbatim.
- Render fixture `internal/render/testdata/aws-registry/`; test `TestRenderIgnoresTheAWSPart`.
- Spec amendment **A10**; phase log `docs/phases/phase-2a.md`.

**Process**
- Strict TDD: failing test, run it to see it fail, implement, run it to see it pass, commit. Tests that pin existing behavior say so and pass before the change.
- No `go build` and no GoReleaser run locally (no `go run` either). Use `go test ./...`, `go vet ./...` and `gofmt -l .` (must print nothing); `-race` runs in CI. No workflow, action or module changes, so no local actionlint, zizmor or `tofu test`; CI runs them.
- Shell rule: never use `cat`, `grep`, `find`, `sed` or `ls`, never write files with shell heredocs, never `perl`. Use the Read/Write/Edit tools, `rg`, `gh`, `git`, `python -I`.
- Conventional commits, never with Co-Authored-By or any AI attribution.
- Docs in English. Owner rule: docs describe the platform on its own terms, never naming another product as inspiration or comparison, and never use the owner's work account; the controller gives each dispatch the naming check that enforces both (it must print nothing).
- Doc-only changes after the merge go straight to `main`; code goes through PRs.
- Never touch `~/.idp/`.

**Spec rules**
- No real cloud credential anywhere (spec §7.6, D-2): nothing in this plan creates an AWS account, credential or resource.
- The JSON Schemas stay the single source of truth for claim structure (ADR-0015).
- Rendering stays pure and, in 2a, GitHub-only (D-10).

**Owner actions:** none beyond approving this plan and its side effects.

**Side effects this plan authorizes** (controller, after the owner approves the plan)
1. Push `feat/phase-2a-aws-model`, open its PR, merge it after CI and the final review.
2. Doc-only commits straight to `main`: this plan, the phase log, the README.
3. Delete the local, git-ignored SDD workspace `.superpowers/sdd/` after the phase log is committed.

Not authorized, stop and ask: merging any release PR (including the one proposing `0.2.0`, D-15), merging a Renovate PR, changing any repository or org setting, App, ruleset, environment, variable or secret, any change to `idp-claims` or `idp-claims-e2e`, creating any cloud resource.

## Review Focus

1. **A registry Component targets an environment without `aws`.**
   - **Expected:** one diagnostic per such environment, at its line in `environments`, with D-8's wording; environments with an `aws` block pass.
   - **Pinned by:** `TestLoadDiagnostics` cases "aws.registry needs an aws block in every environment" and "aws.registry checks each environment" (Task 5).
2. **A Component with an `aws` block but `registry: false`, or an empty block.**
   - **Expected:** valid; no environment needs `aws`; the model reports `AWS.Registry == false`.
   - **Pinned by:** `TestLoadDiagnostics` cases "registry false needs no aws block" and "an empty aws block needs no aws block" (Task 5); `TestSchemas` rows "registry false" and "empty aws block" (Task 4).
3. **Names beyond AWS's length limits (IAM role 64, ECR 256) or with uppercase letters.**
   - **Expected:** impossible by construction: the longest names the schemas allow give a 54-character role name and a valid ECR name, and the schemas reject uppercase.
   - **Pinned by:** `TestNamesFitAWSLimits` and `TestSchemaNameRulesKeepAWSNamesValid` (Task 2).
4. **`aws.registry: true` with no environments.**
   - **Expected:** one diagnostic at the `aws.registry` line, whether `environments` is missing or empty.
   - **Pinned by:** `TestLoadDiagnostics` cases "aws.registry needs an environment" and "aws.registry needs a non-empty environment list" (Task 5).
5. **The GitHub render with an `aws` claim present.**
   - **Expected:** `basic`'s golden file byte for byte, and only the github stack's two files.
   - **Pinned by:** `TestRenderIgnoresTheAWSPart` (Task 6), mutation-checked.

Also tested, outside the five: an environment of a registry Component that is missing from `config/platform.yaml` reports only "is not defined" (Task 5); an unquoted account ID is rejected at its line (Task 3); the account ID and region boundaries and the rejection of an emulator endpoint key (Task 3); `features` still rejected (Task 4); the spec's identity examples, verbatim (Task 2).

## Watch list for the live steps

Record each result in the ledger:
- After the merge, release-please's release PR proposes `0.2.0` (D-15). Task 7 Step 6. Record its number and version and leave it open.
- The harness run dispatched for that release PR passes; its claims carry no `aws` block, so the schema change must not affect it. Task 7 Step 6. A failure is a regression to fix before 2b.

## File Structure

```
docs/superpowers/specs/2026-10-08-idp-on-actions-design.md (A10), docs/adr/0013-phase-0-spike-findings.md, docs/claims.md   # Task 1
internal/naming/naming.go, internal/naming/naming_test.go                                                              # Task 2
internal/claims/model.go, internal/claims/load_test.go, internal/claims/schema_test.go, schemas/schemas_test.go        # Task 3
schemas/component.json, schemas/schemas_test.go, internal/claims/model.go, internal/claims/schema_test.go, internal/claims/load_test.go  # Task 4
internal/claims/load.go, internal/claims/load_test.go                                                                  # Task 5
internal/render/render_test.go, internal/render/testdata/aws-registry/{config/platform.yaml,claims/groups/platform.yaml,claims/components/api.yaml}  # Task 6
docs/phases/phase-2a.md, README.md                                                                                     # Task 7
```

## Pre-flight (controller, before Task 1)

- [ ] `gh api user --jq .login` prints `jellalshadows` (else `gh auth switch -u jellalshadows`).
- [ ] `git switch main && git pull --ff-only`, then `git log -1 --format=%h` prints `150b7d3` or a later commit, and `git status --short` prints nothing.
- [ ] `go version` prints `go1.26` or a later 1.26 patch, and `go test ./...` passes on `main`, so any later failure is this plan's.
- [ ] The owner approves this plan, its side effects and the rulings D-1 to D-15; ledger them. Commit this plan to `main` as `docs: add the phase 2a plan` and push. The `docs` commit is user-facing, so release-please may open a release PR; leave it open (D-15).

---

### Task 1: Decisions first — spec amendment A10, ADR-0013, the claims reference

**Files:**
- Modify: `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, `docs/adr/0013-phase-0-spike-findings.md`, `docs/claims.md`

**Interfaces:**
- Produces: the rule Tasks 2 to 6 implement and cite (spec §4.7 under A10), the Phase 2 split in spec §10, and the user-facing reference of the `aws` fields, with the diagnostic of D-8.

- [ ] **Step 1: Create the branch**

Run: `git switch main && git pull --ff-only && git switch -c feat/phase-2a-aws-model`

- [ ] **Step 2: Apply amendment A10 to the spec** (exact replacements with Edit)

1. §3.3 units table, old:

```markdown
| `internal/naming` | Deterministic identities: ARNs, ECR URLs, role names, Terraform addresses | `internal/config` |
```

new:

```markdown
| `internal/naming` | Deterministic identities: ARNs, ECR URLs, role names, Terraform addresses | None: pure functions of names, account IDs and regions *(Amendment A10)* |
```

2. §3.3, the paragraph after the units, old:

```markdown
separate `internal/config`. *(Amendment A4, 2026-10-10.)*
```

new:

```markdown
separate `internal/config`. *(Amendment A4, 2026-10-10.)* `internal/naming` takes
plain values (component and environment names, account IDs and regions), so it
depends on neither, and no `internal/config` is extracted (Phase 2a ruling D-3).
*(Amendment A10, 2026-11-03.)*
```

3. §4.7, old:

```markdown
   - Every env exists in the platform config.
```

new:

```markdown
   - Every env exists in the platform config.
   - A Component with `aws.registry: true` lists at least one env, and each of its
     envs has an `aws` block (`accountId` and `region`) in the platform config,
     because its registry, CI role and env variables are computed per env (§5.5).
     *(Amendment A10, 2026-11-03.)*
```

4. §6, old:

```markdown
commit is built from source (ADR-0028; *Amendment A9, 2026-10-18*). Until Phase 2 the only stack is `github`, and until Phase 4 there are no
features to fetch or adopt: the workflows fail if any other stack is affected.
*(Amendment A6, 2026-10-11.)*
```

new:

```markdown
commit is built from source (ADR-0028; *Amendment A9, 2026-10-18*). Until Phase 2
renders and reconciles the AWS stacks (Phases 2d and 2e, §10), the only stack is
`github`, and until Phase 4 there are no
features to fetch or adopt: the workflows fail if any other stack is affected.
*(Amendment A6, 2026-10-11; Amendment A10, 2026-11-03.)*
```

5. §10, old:

```markdown
releases (release-please, GoReleaser, attestations), Renovate, the `v0.1.0` release and onboarding `idp-claims`. *(2026-10-09, re-split 2026-10-10 and 2026-10-13.)*
```

new:

```markdown
releases (release-please, GoReleaser, attestations), Renovate, the `v0.1.0` release and onboarding `idp-claims`. *(2026-10-09, re-split 2026-10-10 and 2026-10-13.)*

Phase 2 is delivered as six plans, each written after the previous one ships: **2a** the AWS model of the Platform and the Component, `internal/naming` and the AWS validation; **2b** the deterministic env variables in `modules/github/component`; **2c** `modules/aws/{baseline,component}` (the OIDC provider, the ECR repository and the CI role), the AWS lock file and the floci CI job; **2d** rendering the AWS stacks; **2e** the workflows' AWS plan and apply against floci; **2f** the E2E env-variable checks, the `0.2.0` release and the phase log. *(Amendment A10, 2026-11-03.)*
```

6. §11, risk 4, old:

```markdown
| 4 | floci fidelity gaps (IAM/ECR) | The OIDC provider is already confirmed to exist in floci. Gaps get documented and skipped in level 3 | No gaps found for IAM, OIDC and ECR in S3. ECR URL uses the `localhost:4566` host form (cosmetic). Real `AssumeRoleWithWebIdentity` not exercised |
```

new:

```markdown
| 4 | floci fidelity gaps (IAM/ECR) | The OIDC provider is already confirmed to exist in floci. Gaps get documented and skipped in level 3 | No gaps found for IAM, OIDC and ECR in S3. ECR URL uses the `localhost:4566` host form (cosmetic). Real `AssumeRoleWithWebIdentity` not exercised; out of scope for v1, which has no real AWS account (§2.2, §7.6): the module tests assert the trust policy (§8.3) (owner decision; *Amendment A10, 2026-11-03*) |
```

- [ ] **Step 3: Amend ADR-0013** (exact replacements with Edit, in `docs/adr/0013-phase-0-spike-findings.md`)

1. In "S3 — floci", the notes, old:

```markdown
The OIDC provider existing and the role carrying its trust policy is confirmed; a real assume-role is left to the Phase 2 floci integration tests.
```

new:

```markdown
The OIDC provider existing and the role carrying its trust policy is confirmed; a real assume-role is left to the Phase 2 floci integration tests. *(Amended in Phase 2a, 2026-11-03.)* The owner put a real assume-role out of scope for v1: v1 has no real AWS account (spec §2.2, §7.6), and Phase 2 runs against floci only. The trust policy's `sub` and `aud` are asserted by the module tests (spec §8.3).
```

2. In "Decisions taken from these numbers", old:

```markdown
Unverified: a real `AssumeRoleWithWebIdentity`, which Phase 2 covers.
```

new:

```markdown
Unverified: a real `AssumeRoleWithWebIdentity`, out of scope for v1 by owner decision (see the S3 notes). *(Amended in Phase 2a, 2026-11-03; this said that Phase 2 covers it.)*
```

- [ ] **Step 4: Update `docs/claims.md`** (exact replacements with Edit)

1. The Platform example, old:

````markdown
environments:
  dev: {}
  pro:
    protected: true
```
````

new:

````markdown
environments:
  dev:
    aws: { accountId: "000000000001", region: eu-west-1 }
  pro:
    protected: true
    aws: { accountId: "000000000003", region: eu-west-1 }
```
````

2. The Platform table, old:

```markdown
| `environments.<env>.aws` | no | | `accountId` and `region`; used from Phase 2 |
```

new:

```markdown
| `environments.<env>.aws` | no | | The environment's AWS account. Required for every environment of a Component with `aws.registry: true` |
| `environments.<env>.aws.accountId` | yes, in `aws` | | 12 digits, quoted (`"000000000001"`), so YAML keeps it a string with its leading zeros |
| `environments.<env>.aws.region` | yes, in `aws` | | An AWS region, such as `eu-west-1` |
```

3. After the table, old:

```markdown
| `modules.allowedSources` | no | | Allowed Workspace module prefixes; used from Phase 3 |
```

new:

```markdown
| `modules.allowedSources` | no | | Allowed Workspace module prefixes; used from Phase 3 |

An `aws` block is shaped like real AWS: an account ID and a region, never an emulator endpoint (the schema rejects any other key). In v1, AWS is the floci emulator in CI (spec §5.6), so the IDs can be fake ones like those above. An environment needs an `aws` block only when a Component with `aws.registry: true` lists it.
```

4. The Component example, old:

```markdown
github:
  topics: [java, orders]   # optional; at most 20, unique, each lowercase letters, digits or hyphens, ≤ 50 characters
```

new:

```markdown
github:
  topics: [java, orders]   # optional; at most 20, unique, each lowercase letters, digits or hyphens, ≤ 50 characters
aws:
  registry: true           # optional; default false
```

5. After the Component's list of what it renders, old:

```markdown
The `aws` and `features` fields arrive in Phases 2 and 4. Until then they fail validation.
```

new:

```markdown
With `aws.registry: true`, a Component also gets, in each of its environments: an ECR repository named after it; the IAM role `<name>-<env>-ci`, which may push to that repository and trusts only GitHub's OIDC tokens for `repo:<org>/<name>:environment:<env>`; and the environment variables `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION` (spec §4.4). Their values are computed from the names and the environment's `aws` block, never read from AWS (spec §5.5). For `api` in `pro` above they are `arn:aws:iam::000000000003:role/api-pro-ci`, `000000000003.dkr.ecr.eu-west-1.amazonaws.com/api` and `eu-west-1`.

- `aws.registry: true` needs at least one environment, and each environment it lists needs an `aws` block in `config/platform.yaml`.
- `aws: {}` and `registry: false` are valid and ask for nothing.
- **Not rendered yet.** Since Phase 2a the claims accept and validate the `aws` part, but nothing renders from it until Phases 2b to 2d, and the pipelines apply it from Phase 2e.

The `features` field arrives in Phase 4. Until then it fails validation.
```

6. "Validation", old:

```markdown
on a usage error ([ADR-0016](adr/0016-exit-codes.md)).
```

new:

````markdown
on a usage error ([ADR-0016](adr/0016-exit-codes.md)).

Beyond the schemas, the checks include: each name matches its file name; the owner Group exists; every environment exists in `config/platform.yaml`; a Component with `aws.registry: true` lists at least one environment, and each of them has an `aws` block. For the Component above, in a platform whose `pro` has no `aws` block:

```text
claims/components/api.yaml:6: environment "pro" needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true
```
````

7. "Rendering", old:

```markdown
- `rendered/.idp-rendered`: a marker. `--out` must not exist, be empty, or hold a previous render, so a mistyped path can never wipe unrelated data.
```

new:

```markdown
- `rendered/.idp-rendered`: a marker. `--out` must not exist, be empty, or hold a previous render, so a mistyped path can never wipe unrelated data.

A Component's `aws` part renders nothing yet: Phase 2b adds its environment variables to the github stack, and Phase 2d its AWS stacks.
```

- [ ] **Step 5: Check and commit**

Run:

```bash
python -I -c "
import re,pathlib
spec=pathlib.Path('docs/superpowers/specs/2026-10-08-idp-on-actions-design.md').read_text(encoding='utf-8')
def slug(h):
    h=h.strip().lower(); h=re.sub(r'[^\w\- ]','',h); return h.replace(' ','-')
anchors={slug(m.group(1)) for m in re.finditer(r'^#+ (.+)$', spec, re.M)}
bad=[(str(p),a) for p in pathlib.Path('docs').rglob('*.md') if 'superpowers' not in p.parts for a in re.findall(r'2026-10-08-idp-on-actions-design\.md#([\w\-]+)', p.read_text(encoding='utf-8')) if a not in anchors]
print(bad or 'all spec anchors resolve')
"
```

Expected: `all spec anchors resolve`. Then:
- `rg -c "Amendment A10" docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` prints `6`;
- `rg -c "Amended in Phase 2a" docs/adr/0013-phase-0-spike-findings.md` prints `2`;
- `rg -n "used from Phase 2|arrive in Phases 2 and 4|which Phase 2 covers|Until Phase 2 the only" docs/claims.md docs/adr docs/superpowers/specs` prints nothing;
- the naming check prints nothing.

```bash
git add docs/superpowers/specs/2026-10-08-idp-on-actions-design.md docs/adr/0013-phase-0-spike-findings.md docs/claims.md
git commit -m "docs: record the phase 2 split and the aws validation rule (spec amendment a10)"
```

---

### Task 2: `internal/naming`

**Files:**
- Create: `internal/naming/naming.go`, `internal/naming/naming_test.go`

**Interfaces:**
- Consumes (tests only): `schemas.FS`, `schemas.File` (`github.com/jellalshadows-idp/idp-engine/schemas`).
- Produces: `const OIDCProviderHost`; `func RepositoryName(component string) string`; `func RoleName(component, env string) string`; `func RoleARN(accountID, component, env string) string`; `func RegistryURL(accountID, region, component string) string`; `func OIDCProviderARN(accountID string) string`. Test helpers in `naming_test.go`: `var iamRoleName, ecrRepositoryName *regexp.Regexp`, `func schemaPattern(t *testing.T, kind string, path ...string) *regexp.Regexp`, `func nameRules(t *testing.T) (component, env *regexp.Regexp)`.

- [ ] **Step 1: Write the failing tests** — `internal/naming/naming_test.go`:

```go
package naming

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/schemas"
)

// AWS's own name rules, from the API references of IAM CreateRole and ECR
// CreateRepository (read 2026-11-03).
var (
	iamRoleName       = regexp.MustCompile(`^[\w+=,.@-]{1,64}$`)
	ecrRepositoryName = regexp.MustCompile(`^[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*$`)
)

func TestIdentities(t *testing.T) {
	tests := []struct {
		name, got, want string
	}{
		{"role ARN, spec §5.3", RoleARN("000000000003", "api", "pro"), "arn:aws:iam::000000000003:role/api-pro-ci"},
		{"registry URL, spec §5.3", RegistryURL("000000000003", "eu-west-1", "api"), "000000000003.dkr.ecr.eu-west-1.amazonaws.com/api"},
		{"role ARN, ADR-0013", RoleARN("000000000001", "api", "dev"), "arn:aws:iam::000000000001:role/api-dev-ci"},
		{"OIDC provider ARN, spec §5.5", OIDCProviderARN("000000000003"), "arn:aws:iam::000000000003:oidc-provider/token.actions.githubusercontent.com"},
		{"repository name, spec §4.4", RepositoryName("api"), "api"},
		{"role name, spec §4.4", RoleName("api", "staging"), "api-staging-ci"},
		{"harness component", RoleName("e2e-r123-app", "dev"), "e2e-r123-app-dev-ci"},
		{"registry URL keeps the region", RegistryURL("000000000002", "ap-southeast-2", "e2e-r123-app"), "000000000002.dkr.ecr.ap-southeast-2.amazonaws.com/e2e-r123-app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// schemaPattern returns the regular expression at path inside the embedded schema
// of kind, so these tests follow the schemas instead of copying their rules.
func schemaPattern(t *testing.T, kind string, path ...string) *regexp.Regexp {
	t.Helper()
	raw, err := schemas.FS.ReadFile(schemas.File(kind))
	if err != nil {
		t.Fatal(err)
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	for _, key := range path {
		obj, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%s: no object holds %q", schemas.File(kind), key)
		}
		node = obj[key]
	}
	pattern, ok := node.(string)
	if !ok {
		t.Fatalf("%s: %s is not a pattern", schemas.File(kind), strings.Join(path, "."))
	}
	return regexp.MustCompile(pattern)
}

// nameRules returns the schemas' rules for a Component name and an environment
// name, and checks that a Component's environments follow the Platform's rule.
func nameRules(t *testing.T) (component, env *regexp.Regexp) {
	t.Helper()
	component = schemaPattern(t, "Component", "properties", "name", "pattern")
	env = schemaPattern(t, "Platform", "properties", "environments", "propertyNames", "pattern")
	if claimEnv := schemaPattern(t, "Component", "properties", "environments", "items", "pattern"); claimEnv.String() != env.String() {
		t.Fatalf("a Component's environments (%s) and the Platform's (%s) follow different rules", claimEnv, env)
	}
	return component, env
}

// TestNamesFitAWSLimits proves spec §5.5's bound: the longest names the schemas
// allow, a 40-character Component (§4.2) and a 10-character environment (§4.6),
// give a 54-character role name, within IAM's 64, and a valid ECR repository name.
func TestNamesFitAWSLimits(t *testing.T) {
	componentRule, envRule := nameRules(t)
	longestComponent := "a" + strings.Repeat("b-", 19) + "c"
	longestEnv := "e" + strings.Repeat("9", 9)
	for _, n := range []struct {
		what, value string
		rule        *regexp.Regexp
	}{
		{"Component name", longestComponent, componentRule},
		{"environment name", longestEnv, envRule},
	} {
		if !n.rule.MatchString(n.value) {
			t.Fatalf("the schema rejects the %s %q; this test assumes it is the longest allowed", n.what, n.value)
		}
		if n.rule.MatchString(n.value + "x") {
			t.Fatalf("the schema allows a %s longer than %d characters: recheck the AWS limits (spec §5.5) before widening it", n.what, len(n.value))
		}
	}
	if role := RoleName(longestComponent, longestEnv); len(role) != 54 || !iamRoleName.MatchString(role) {
		t.Errorf("RoleName = %q (%d characters), want 54 characters that IAM accepts", role, len(role))
	}
	if repo := RepositoryName(longestComponent); len(repo) > 256 || !ecrRepositoryName.MatchString(repo) {
		t.Errorf("RepositoryName = %q, want a name ECR accepts", repo)
	}
}

// TestSchemaNameRulesKeepAWSNamesValid checks names at the edges of the schema
// rules: every name they accept gives AWS-valid names, and they reject the rest.
// ECR allows only lowercase, and IAM compares role names without regard to case,
// so an uppercase name must never reach naming.
func TestSchemaNameRulesKeepAWSNamesValid(t *testing.T) {
	componentRule, envRule := nameRules(t)
	tests := []struct {
		component string
		allowed   bool
	}{
		{"ab", true},
		{"a--b", true},
		{"a1-2b", true},
		{"e2e-r123-app", true},
		{"a", false},
		{"Api", false},
		{"API", false},
		{"a_b", false},
		{"a.b", false},
		{"-ab", false},
		{"ab-", false},
		{"1ab", false},
	}
	for _, tt := range tests {
		t.Run(tt.component, func(t *testing.T) {
			if got := componentRule.MatchString(tt.component); got != tt.allowed {
				t.Fatalf("the schema allows %q: %v, want %v", tt.component, got, tt.allowed)
			}
			if !tt.allowed {
				return
			}
			if repo := RepositoryName(tt.component); len(repo) < 2 || len(repo) > 256 || !ecrRepositoryName.MatchString(repo) {
				t.Errorf("RepositoryName(%q) = %q, which ECR rejects", tt.component, repo)
			}
			if role := RoleName(tt.component, "dev"); !iamRoleName.MatchString(role) {
				t.Errorf("RoleName(%q, \"dev\") = %q, which IAM rejects", tt.component, role)
			}
		})
	}
	for _, env := range []string{"Dev", "DEV", "pro-1", "1dev", "dev_1"} {
		if envRule.MatchString(env) {
			t.Errorf("the environment rule allows %q", env)
		}
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/naming/`
Expected: `FAIL	github.com/jellalshadows-idp/idp-engine/internal/naming [build failed]`, with `undefined: RoleARN`, `undefined: RegistryURL`, `undefined: OIDCProviderARN`, `undefined: RepositoryName` and `undefined: RoleName`.

- [ ] **Step 3: Implement** — `internal/naming/naming.go`:

```go
// Package naming computes the deterministic AWS identities of a Component in an
// environment (spec §5.5, ADR-0006). The GitHub stack receives these values from
// here, never from AWS state, and the AWS modules must create exactly these
// names: they are a contract. Every function is pure, over names the schemas
// already constrain (lowercase; a Component name of at most 40 characters and an
// environment name of at most 10), so none can fail.
package naming

// OIDCProviderHost is the GitHub Actions OIDC issuer. Each environment's account
// has one IAM OIDC provider for it, created by the env's _baseline stack (spec §5.4).
const OIDCProviderHost = "token.actions.githubusercontent.com"

// RepositoryName is the ECR repository of a Component: its own name (spec §4.4).
func RepositoryName(component string) string { return component }

// RoleName is the IAM role that a Component's CI assumes in an environment to push
// to its repository (spec §4.4). It is at most 54 characters.
func RoleName(component, env string) string { return component + "-" + env + "-ci" }

// RoleARN is the ARN of RoleName in the environment's account (spec §5.5).
func RoleARN(accountID, component, env string) string {
	return "arn:aws:iam::" + accountID + ":role/" + RoleName(component, env)
}

// RegistryURL is the URL of a Component's ECR repository in an environment's
// account and region (spec §5.5). floci answers with a localhost:4566 host instead
// (ADR-0013), so emulator output is compared by repository name, never by URL.
func RegistryURL(accountID, region, component string) string {
	return accountID + ".dkr.ecr." + region + ".amazonaws.com/" + RepositoryName(component)
}

// OIDCProviderARN is the ARN of the GitHub OIDC provider in an account (spec §5.5).
func OIDCProviderARN(accountID string) string {
	return "arn:aws:iam::" + accountID + ":oidc-provider/" + OIDCProviderHost
}
```

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./internal/naming/ -v -run 'TestIdentities|TestNamesFitAWSLimits|TestSchemaNameRulesKeepAWSNamesValid'`
Expected: `PASS`, with the 8 `TestIdentities` subtests and the 12 `TestSchemaNameRulesKeepAWSNamesValid` subtests listed.

- [ ] **Step 5: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

- [ ] **Step 6: Commit**

```bash
git add internal/naming
git commit -m "feat(naming): add the deterministic aws identities"
```

---

### Task 3: The Platform's AWS model

**Files:**
- Modify: `internal/claims/model.go`, `internal/claims/load_test.go`, `internal/claims/schema_test.go`, `schemas/schemas_test.go`

**Interfaces:**
- Consumes: `Load`, `baseRepo`, `writeTree`, `lines`, `validPlatform`, `validComponent` (`internal/claims/load_test.go`); `newValidator`, `mustDoc` (`internal/claims/schema_test.go`); `compile`, `instance` (`schemas/schemas_test.go`).
- Produces: `type AWS struct { AccountID string; Region string }`; the field `Environment.AWS *AWS`; the test constant `awsPlatform` in `load_test.go` (environments `dev` and `pro` with an `aws` block, `pro` protected, `sandbox` without one), used again by Tasks 4 and 5.

- [ ] **Step 1: Pin the existing platform schema boundaries** (D-14; these pass before the change)

`schemas/schemas_test.go`, in `TestSchemas`, add after the `{"Platform", "approvals out of range", …}` row:

```go
		{"Platform", "aws account id of 11 digits", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"00000000001","region":"eu-west-1"}}}}`, false},
		{"Platform", "aws account id of 13 digits", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"0000000000001","region":"eu-west-1"}}}}`, false},
		{"Platform", "aws account id as a number", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":1,"region":"eu-west-1"}}}}`, false},
		{"Platform", "aws region with three parts", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"000000000001","region":"us-gov-west-1"}}}}`, true},
		{"Platform", "aws region in uppercase", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"000000000001","region":"EU-west-1"}}}}`, false},
		{"Platform", "aws region without its number", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"000000000001","region":"eu-west"}}}}`, false},
		{"Platform", "aws block without a region", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"000000000001"}}}}`, false},
		{"Platform", "aws emulator endpoint", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"000000000001","region":"eu-west-1","endpoint":"http://localhost:4566"}}}}`, false},
```

`internal/claims/schema_test.go`, in `TestValidate`, add before the `self-referential anchor` row:

```go
		{name: "unquoted aws account id points at it", kind: "Platform", src: "apiVersion: idp/v1\nkind: Platform\ngithub:\n  org: acme\n  writerAppId: 1\nenvironments:\n  dev:\n    aws:\n      accountId: 000000000001\n      region: eu-west-1\n", wantLine: 9, wantContains: "/environments/dev/aws/accountId"},
```

- [ ] **Step 2: Run them; they pass already**

Run: `go test ./schemas/ ./internal/claims/ -run 'TestSchemas|TestValidate'`
Expected: `ok` for both packages. The unquoted ID is a YAML number (D-6), so the schema's `"type": "string"` rejects it at line 9.

- [ ] **Step 3: Write the failing model test**

`internal/claims/load_test.go`: replace the `const` block with:

```go
const (
	validPlatform  = "apiVersion: idp/v1\nkind: Platform\ngithub:\n  org: acme\n  writerAppId: 5255579\nenvironments:\n  dev: {}\n  pro:\n    protected: true\n"
	awsPlatform    = "apiVersion: idp/v1\nkind: Platform\ngithub:\n  org: acme\n  writerAppId: 5255579\nenvironments:\n  dev:\n    aws:\n      accountId: \"000000000001\"\n      region: eu-west-1\n  pro:\n    protected: true\n    aws:\n      accountId: \"000000000003\"\n      region: eu-west-1\n  sandbox: {}\n"
	validGroup     = "apiVersion: idp/v1\nkind: Group\nname: platform\nmembers:\n  - user: alice\n    role: maintainer\n"
	validComponent = "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nenvironments: [dev, pro]\n"
)
```

and add after `TestLoadValidRepo`:

```go
func TestLoadPlatformAWS(t *testing.T) {
	repo := baseRepo()
	repo["config/platform.yaml"] = awsPlatform
	m, ds, err := Load(writeTree(t, repo))
	if err != nil || len(ds) != 0 {
		t.Fatalf("err %v, diagnostics %v", err, lines(ds))
	}
	envs := m.Platform.Environments
	if got := envs["dev"].AWS; got == nil || *got != (AWS{AccountID: "000000000001", Region: "eu-west-1"}) {
		t.Errorf("dev aws = %+v, want account 000000000001 in eu-west-1", got)
	}
	if got := envs["pro"]; !got.Protected || got.AWS == nil || got.AWS.AccountID != "000000000003" {
		t.Errorf("pro = %+v, want protected, with account 000000000003", got)
	}
	if got := envs["sandbox"].AWS; got != nil {
		t.Errorf("sandbox aws = %+v, want nil: the environment has no aws block", got)
	}
}
```

- [ ] **Step 4: Run it to see it fail**

Run: `go test ./internal/claims/ -run TestLoadPlatformAWS`
Expected: `FAIL	github.com/jellalshadows-idp/idp-engine/internal/claims [build failed]`, with `envs["dev"].AWS undefined (type Environment has no field or method AWS)` and `undefined: AWS`.

- [ ] **Step 5: Implement** — `internal/claims/model.go`, old:

```go
// Environment is one entry of Platform.Environments. Its aws block is used from Phase 2.
type Environment struct {
	Protected bool `yaml:"protected"`
}
```

new:

```go
// Environment is one entry of Platform.Environments. AWS is nil when the
// environment has no aws block.
type Environment struct {
	Protected bool `yaml:"protected"`
	AWS       *AWS `yaml:"aws"`
}

// AWS is an environment's AWS account and region (spec §4.6), shaped like real
// AWS; CI maps them to the emulator (spec §5.6). The account ID stays a string,
// which keeps its leading zeros, and the schema requires both fields.
type AWS struct {
	AccountID string `yaml:"accountId"`
	Region    string `yaml:"region"`
}
```

- [ ] **Step 6: Run it to see it pass**

Run: `go test ./internal/claims/ -run 'TestLoadPlatformAWS|TestLoadValidRepo'`
Expected: `ok`.

- [ ] **Step 7: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

- [ ] **Step 8: Commit**

```bash
git add internal/claims schemas/schemas_test.go
git commit -m "feat(claims): model the environments' aws account and region"
```

---

### Task 4: The Component's AWS part in the schema and the model

**Files:**
- Modify: `schemas/component.json`, `schemas/schemas_test.go`, `internal/claims/model.go`, `internal/claims/schema_test.go`, `internal/claims/load_test.go`

**Interfaces:**
- Consumes: `compile`, `instance` (`schemas/schemas_test.go`); `newValidator`, `mustDoc` (`internal/claims/schema_test.go`); `Load`, `baseRepo`, `writeTree`, `lines`, `validComponent`, `awsPlatform` (`internal/claims/load_test.go`, Task 3).
- Produces: the schema property `aws` (`{registry: boolean}`, default `false`, no other key); the field `Component.AWS.Registry bool`.

- [ ] **Step 1: Write the failing schema tests**

`schemas/schemas_test.go`, in `TestSchemas`, old:

```go
		{"Component", "aws not supported yet", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{"registry":true}}`, false},
```

new:

```go
		{"Component", "aws registry", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","environments":["dev"],"aws":{"registry":true}}`, true},
		{"Component", "empty aws block", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{}}`, true},
		{"Component", "registry false", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{"registry":false}}`, true},
		{"Component", "registry as a string", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{"registry":"true"}}`, false},
		{"Component", "unknown aws field", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{"registry":true,"bucket":"logs"}}`, false},
		{"Component", "aws is not a mapping", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":true}`, false},
		{"Component", "features not supported yet", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","features":[]}`, false},
```

`internal/claims/schema_test.go`, in `TestValidate`, old:

```go
		{name: "aws is not supported yet", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\naws:\n  registry: true\n", wantLine: 5, wantContains: "aws"},
```

new:

```go
		{name: "valid component with an aws registry", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nenvironments: [dev]\naws:\n  registry: true\n"},
		{name: "unknown aws field points at its key", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\naws:\n  registry: true\n  bucket: logs\n", wantLine: 7, wantContains: "bucket"},
		{name: "registry that is not a boolean points at it", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\naws:\n  registry: \"yes\"\n", wantLine: 6, wantContains: "/aws/registry"},
		{name: "features are not supported yet", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nfeatures: []\n", wantLine: 5, wantContains: "features"},
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./schemas/ ./internal/claims/ -run 'TestSchemas|TestValidate'`
Expected: FAIL in both packages:
- `TestSchemas/Component/aws_registry`, `TestSchemas/Component/empty_aws_block` and `TestSchemas/Component/registry_false`: `want valid, got …`;
- `TestValidate/valid_component_with_an_aws_registry`: `want valid, got …`;
- `TestValidate/unknown_aws_field_points_at_its_key` and `TestValidate/registry_that_is_not_a_boolean_points_at_it`: the only diagnostic is at line 5, `(document): additional properties 'aws' not allowed`.

The `registry as a string`, `unknown aws field`, `aws is not a mapping` and both `features` rows pass already.

- [ ] **Step 3: Implement the schema** — replace `schemas/component.json` with:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Component",
  "description": "A service: its GitHub repository and environments, and its optional AWS part (spec §4.4). The features field arrives in Phase 4.",
  "type": "object",
  "additionalProperties": false,
  "required": ["apiVersion", "kind", "name", "owner"],
  "properties": {
    "apiVersion": { "const": "idp/v1" },
    "kind": { "const": "Component" },
    "name": { "type": "string", "pattern": "^[a-z][a-z0-9-]{0,38}[a-z0-9]$" },
    "description": { "type": "string", "maxLength": 350, "pattern": "^[^\\r\\n]*$" },
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
    },
    "aws": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "registry": { "type": "boolean", "default": false }
      }
    }
  }
}
```

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./schemas/ ./internal/claims/ -run 'TestSchemas|TestValidate'`
Expected: `ok` for both packages.

- [ ] **Step 5: Write the failing model test** — `internal/claims/load_test.go`, add after `TestLoadPlatformAWS`:

```go
func TestLoadComponentAWS(t *testing.T) {
	repo := baseRepo()
	repo["config/platform.yaml"] = awsPlatform
	repo["claims/components/web.yaml"] = strings.Replace(validComponent, "name: api", "name: web", 1) + "aws:\n  registry: true\n"
	m, ds, err := Load(writeTree(t, repo))
	if err != nil || len(ds) != 0 {
		t.Fatalf("err %v, diagnostics %v", err, lines(ds))
	}
	if m.Components[0].Name != "api" || m.Components[0].AWS.Registry {
		t.Errorf("api = %+v, want no aws registry: its claim has no aws block", m.Components[0])
	}
	if m.Components[1].Name != "web" || !m.Components[1].AWS.Registry {
		t.Errorf("web = %+v, want aws.registry true", m.Components[1])
	}
}
```

- [ ] **Step 6: Run it to see it fail**

Run: `go test ./internal/claims/ -run TestLoadComponentAWS`
Expected: `FAIL	github.com/jellalshadows-idp/idp-engine/internal/claims [build failed]`, with `m.Components[0].AWS undefined (type Component has no field or method AWS)`.

- [ ] **Step 7: Implement the model** — `internal/claims/model.go`, old:

```go
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
```

new:

```go
// Component is a service (spec §4.4): its GitHub part, and its AWS part when
// AWS.Registry is set. Features arrive in Phase 4.
type Component struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Owner        string   `yaml:"owner"`
	Environments []string `yaml:"environments"`
	GitHub       struct {
		Topics []string `yaml:"topics"`
	} `yaml:"github"`
	AWS struct {
		Registry bool `yaml:"registry"`
	} `yaml:"aws"`
}
```

If `gofmt -l internal/claims` lists `model.go`, run `gofmt -w internal/claims/model.go` and keep its layout.

- [ ] **Step 8: Run it to see it pass**

Run: `go test ./internal/claims/ ./schemas/`
Expected: `ok` for both packages.

- [ ] **Step 9: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

- [ ] **Step 10: Commit**

```bash
git add schemas internal/claims
git commit -m "feat(claims): accept a component's aws registry"
```

---

### Task 5: The semantic check

**Files:**
- Modify: `internal/claims/load.go`, `internal/claims/load_test.go`

**Interfaces:**
- Consumes: `Component.AWS.Registry` (Task 4), `Environment.AWS` (Task 3), `loader.add`, `loader.platformOK`, `document.lines`, `Diagnostic`; in tests `TestLoadDiagnostics`, `baseRepo`, `validPlatform`, `validComponent`, `awsPlatform`.
- Produces: the two diagnostics of D-8, verbatim.

- [ ] **Step 1: Write the failing tests** — `internal/claims/load_test.go`, in `TestLoadDiagnostics`, add before the `gitkeep ignored` row:

```go
		{"aws.registry needs an aws block in every environment", func(r map[string]string) {
			r["claims/components/api.yaml"] = validComponent + "aws:\n  registry: true\n"
		}, []string{
			`claims/components/api.yaml:5: environment "dev" needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true`,
			`claims/components/api.yaml:5: environment "pro" needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true`,
		}},
		{"aws.registry checks each environment", func(r map[string]string) {
			r["config/platform.yaml"] = strings.Replace(validPlatform, "  dev: {}\n", "  dev:\n    aws:\n      accountId: \"000000000001\"\n      region: eu-west-1\n", 1)
			r["claims/components/api.yaml"] = validComponent + "aws:\n  registry: true\n"
		}, []string{`claims/components/api.yaml:5: environment "pro" needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true`}},
		{"registry false needs no aws block", func(r map[string]string) {
			r["claims/components/api.yaml"] = validComponent + "aws:\n  registry: false\n"
		}, nil},
		{"an empty aws block needs no aws block", func(r map[string]string) {
			r["claims/components/api.yaml"] = validComponent + "aws: {}\n"
		}, nil},
		{"aws.registry needs an environment", func(r map[string]string) {
			r["claims/components/api.yaml"] = "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\naws:\n  registry: true\n"
		}, []string{`claims/components/api.yaml:6: aws.registry needs at least one environment: the registry and its CI role are created per environment`}},
		{"aws.registry needs a non-empty environment list", func(r map[string]string) {
			r["claims/components/api.yaml"] = strings.Replace(validComponent, "[dev, pro]", "[]", 1) + "aws:\n  registry: true\n"
		}, []string{`claims/components/api.yaml:7: aws.registry needs at least one environment: the registry and its CI role are created per environment`}},
		{"an undefined environment is not also missing aws", func(r map[string]string) {
			r["config/platform.yaml"] = awsPlatform
			r["claims/components/api.yaml"] = strings.Replace(validComponent, "[dev, pro]", "[dev, staging]", 1) + "aws:\n  registry: true\n"
		}, []string{`claims/components/api.yaml:5: environment "staging" is not defined in config/platform.yaml`}},
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/claims/ -run TestLoadDiagnostics`
Expected: FAIL on exactly four subtests, each printing empty `diagnostics =` against the wanted lines: `aws.registry_needs_an_aws_block_in_every_environment`, `aws.registry_checks_each_environment`, `aws.registry_needs_an_environment` and `aws.registry_needs_a_non-empty_environment_list`. The other three new rows pass already.

- [ ] **Step 3: Implement** — `internal/claims/load.go`, in `checkSemantics`, old:

```go
		if !l.platformOK {
			continue
		}
		for i, env := range c.Environments {
			if _, ok := l.model.Platform.Environments[env]; !ok {
				l.add(Diagnostic{File: doc.file, Line: doc.lines[fmt.Sprintf("/environments/%d", i)],
					Message: fmt.Sprintf("environment %q is not defined in config/platform.yaml", env)})
			}
		}
```

new:

```go
		if c.AWS.Registry && len(c.Environments) == 0 {
			l.add(Diagnostic{File: doc.file, Line: doc.lines["/aws/registry"],
				Message: "aws.registry needs at least one environment: the registry and its CI role are created per environment"})
		}
		if !l.platformOK {
			continue
		}
		for i, env := range c.Environments {
			line := doc.lines[fmt.Sprintf("/environments/%d", i)]
			e, ok := l.model.Platform.Environments[env]
			if !ok {
				l.add(Diagnostic{File: doc.file, Line: line,
					Message: fmt.Sprintf("environment %q is not defined in config/platform.yaml", env)})
				continue
			}
			// The registry, its CI role and the env variables are computed per
			// environment from its account and region (spec §4.7, §5.5).
			if c.AWS.Registry && e.AWS == nil {
				l.add(Diagnostic{File: doc.file, Line: line,
					Message: fmt.Sprintf("environment %q needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true", env)})
			}
		}
```

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./internal/claims/`
Expected: `ok` (the existing `TestLoadDiagnostics` rows, `TestLoadReportsEveryProblemAtOnce` and `TestLoadInvalidOwnerGroupDoesNotCascade` included).

- [ ] **Step 5: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

- [ ] **Step 6: Commit**

```bash
git add internal/claims
git commit -m "feat(claims): require an aws block for every environment of an aws component"
```

---

### Task 6: The GitHub render is unaffected by the AWS part

**Files:**
- Create: `internal/render/testdata/aws-registry/config/platform.yaml`, `internal/render/testdata/aws-registry/claims/groups/platform.yaml`, `internal/render/testdata/aws-registry/claims/components/api.yaml`
- Modify: `internal/render/render_test.go`

**Interfaces:**
- Consumes: `loadFixture`, `Render`, `Options`, the golden file `internal/render/testdata/basic/expected/github/main.tf.json`; `Component.AWS.Registry` and `Environment.AWS`.
- Produces: the fixture `aws-registry` (`basic` plus `aws.registry: true` and an `aws` block in `dev` and `pro`); `TestRenderIgnoresTheAWSPart`.

- [ ] **Step 1: Write the failing test** — `internal/render/render_test.go`: add `"maps"` and `"slices"` to the imports (keep them sorted: `bytes`, `encoding/json`, `flag`, `maps`, `math/rand/v2`, `os`, `path/filepath`, `regexp`, `slices`, `strings`, `testing`), and add after `TestRenderGolden`:

```go
// TestRenderIgnoresTheAWSPart pins Phase 2a's interim contract: a Component's aws
// part is validated but renders nothing yet. The aws-registry fixture is basic
// plus aws.registry and the environments' aws blocks, so its render must be
// basic's golden file, byte for byte, and still only the github stack. Phase 2b
// (the env variables) and Phase 2d (the AWS stacks) change this test on purpose.
func TestRenderIgnoresTheAWSPart(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	if !m.Components[0].AWS.Registry || m.Platform.Environments["dev"].AWS == nil || m.Platform.Environments["pro"].AWS == nil {
		t.Fatal("the fixture must set aws.registry and give every environment an aws block")
	}
	files, err := Render(m, Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(files)); !slices.Equal(got, []string{"github/.terraform.lock.hcl", "github/main.tf.json"}) {
		t.Errorf("rendered files = %v, want only the github stack", got)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "basic", "expected", "github", "main.tf.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files["github/main.tf.json"], want) {
		t.Errorf("an aws part changed the github stack:\n%s", files["github/main.tf.json"])
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/render/ -run TestRenderIgnoresTheAWSPart`
Expected: FAIL: `fixture aws-registry has diagnostics: [config/platform.yaml: file not found]`.

- [ ] **Step 3: Create the fixture** (Write tool)

`internal/render/testdata/aws-registry/config/platform.yaml`:

```yaml
apiVersion: idp/v1
kind: Platform
github:
  org: acme
  writerAppId: 5255579
environments:
  dev:
    aws:
      accountId: "000000000001"
      region: eu-west-1
  pro:
    protected: true
    aws:
      accountId: "000000000003"
      region: eu-west-1
```

`internal/render/testdata/aws-registry/claims/groups/platform.yaml` (identical to `basic`'s):

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

`internal/render/testdata/aws-registry/claims/components/api.yaml` (`basic`'s, plus the `aws` block):

```yaml
apiVersion: idp/v1
kind: Component
name: api
description: Orders API ${not.evaluated} %{ literal }
owner: group:platform
environments: [dev, pro]
github:
  topics: [java, orders]
aws:
  registry: true
```

- [ ] **Step 4: Run it to see it pass**

Run: `go test ./internal/render/ -run 'TestRenderIgnoresTheAWSPart|TestRenderGolden'`
Expected: `ok`.

- [ ] **Step 5: Mutation check** (prove the guard bites; nothing is committed)

In `internal/render/render.go`, add the line `"aws_registry":       c.AWS.Registry,` to the `component_<name>` module map after `"archive_on_destroy": p.ArchiveOnDestroy(),`. Run `go test ./internal/render/ -run TestRenderIgnoresTheAWSPart`. Expected: FAIL with `an aws part changed the github stack:` followed by a render that contains `"aws_registry": true`. Remove the line again; `git diff --stat internal/render/render.go` prints nothing.

- [ ] **Step 6: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

- [ ] **Step 7: Commit**

```bash
git add internal/render
git commit -m "test(render): pin that the aws part leaves the github render unchanged"
```

---

### Task 7: Ship — verification, PR, review, merge, phase log, README

Controller task (tests are delegated to a subagent).

- [ ] **Step 1: Local verification** — `go test ./... && go vet ./... && gofmt -l .` (gofmt prints nothing); the naming check over the branch's changes (prints nothing); `git diff --stat main..HEAD` lists only the files of the File Structure (Tasks 1 to 6). No workflow, action or module changed, so there is no local actionlint, zizmor or `tofu test`.

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feat/phase-2a-aws-model
gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-2a-aws-model --title "feat: phase 2a aws model of the platform and the component" --body "Implements docs/superpowers/plans/2026-11-03-phase-2a-aws-model.md (spec amendment A10): internal/naming computes the deterministic AWS identities (spec §5.5); the Platform model reads each environment's aws account and region; the Component schema and model accept aws.registry; validation requires a registry Component to list at least one environment, each with an aws block. The renderer stays GitHub-only, pinned by a test. No release: 0.2.0 ships in Phase 2f."
```

- [ ] **Step 3: CI** — `gh pr checks <number> --repo jellalshadows-idp/idp-engine --watch`. Expected: `go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `render-smoke` and `release-snapshot` pass. A failure goes through the fix loop (fix with a test, scoped review, push).

- [ ] **Step 4: Final whole-branch review** (most capable model) with the package `main..HEAD`, the spec (§3.3, §4.4, §4.6, §4.7, §5.5, §10, §11), ADR-0006, ADR-0013, ADR-0015 and this plan; one fix wave; one scoped re-review; rulings in the ledger. CI green again after the wave.

- [ ] **Step 5: Merge** — confirm the PR's latest CI is green, then `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`.

- [ ] **Step 6: The release PR** (watch list) — the merge's push runs `release.yaml`. Read the release PR with `gh pr list --repo jellalshadows-idp/idp-engine --head release-please--branches--main --json number,title --jq '.[] | "\(.number) \(.title)"'`. Expected: one line `<number> chore(main): release 0.2.0`. Record it and leave the PR open (D-15). Record the harness run that `e2e-release-pr` dispatched (`gh run list --repo jellalshadows-idp/idp-engine --workflow e2e.yaml --limit 1 --json databaseId,status,conclusion`) and, when it ends, its conclusion. Expected: `success`; a failure is a regression to fix before 2b.

- [ ] **Step 7: Write `docs/phases/phase-2a.md`** in the style of `docs/phases/phase-1e.md`: the header list (period, plan, spec amended by A10, ADR-0013 amended, previous phase, status with the PR and its merge commit); summary; tasks with commits and reviews; every ruling (D-1 to D-15 and the execution rulings) with what, why and cost if wrong; the Review Focus table with each result; the watch list with each result (the release PR's number and version, the harness run); notable findings caught by review; the final disposition of every finding. The disposition includes these carried items: Phase 1a Task 2 minor, its AWS account ID and region part **Fixed** (Task 3) and its name length and case part **Fixed** (Task 2), the rest unchanged; Phase 1a's "Phase 2 candidate" to extract `internal/config` **Won't fix** (D-3: naming takes plain values); ADR-0013's real `AssumeRoleWithWebIdentity` test **Won't fix**, out of scope for v1 (D-2). Then "Carried into Phase 2b": the `variables` input and `github_actions_environment_variable` in `modules/github/component`, rendered from `internal/naming`; `TestRenderIgnoresTheAWSPart` becomes the env-variables golden. "Carried into later sub-phases": the baseline's lifecycle and the shared-account question (D-11) in 2c; the AWS stacks' backend, encryption, lock file and provider pin (hashicorp/aws 6.66.0 was proven on OpenTofu 1.12.6, and the repo now pins 1.13.1) in 2c and 2d; the nested-stack discovery items of `docs/phases/phase-1b.md` in 2d; the AWS reconcile design (plan and apply matrices, required check names, wet paths) in 2e; the harness's token to read environment variables, the `aws` block for `dev` in `idp-claims-e2e`'s `config/platform.yaml`, and the floci ECR host form (ADR-0013) in 2c and 2f. "Owner notes": the release PR proposing `0.2.0` stays open until 2f (D-15), and every push to `main` re-runs the harness on it while it is open; the earlier owner items stay as recorded in the Phase 1e log. Process notes and status.

- [ ] **Step 8: Update `README.md`** (exact replacements with Edit)

1. Old:

```markdown
> **Status:** Phase 1e is complete: the engine is released with verified binaries, Renovate keeps every pin current, and the production `idp-claims` runs the pipelines at a release; see the [phase 1e log](docs/phases/phase-1e.md), the [releases runbook](docs/runbooks/releases.md) and [docs/pipelines.md](docs/pipelines.md). Phase 2 (the Component's AWS part) is next.
```

new:

```markdown
> **Status:** Phase 2a is complete: claims accept a Component's AWS part (`aws.registry`) and each environment's AWS account, validate them, and `internal/naming` computes the AWS identities; nothing AWS is rendered or applied yet. See the [phase 2a log](docs/phases/phase-2a.md) and [docs/claims.md](docs/claims.md). Phase 2b (the deterministic env variables) is next.
```

2. Old:

```markdown
- Phase 1 reconciles **GitHub only** (teams, repositories, rulesets, environments); AWS arrives in Phase 2.
```

new:

```markdown
- The pipelines reconcile **GitHub only** (teams, repositories, rulesets, environments). Claims accept a Component's AWS part since Phase 2a, but it is rendered from Phase 2b and applied from Phase 2e.
```

- [ ] **Step 9: Commit doc-only to `main`** (side effect 2) — the naming check prints nothing; then:

```bash
git add docs/phases/phase-2a.md README.md
git commit -m "docs: add the phase 2a execution log"
git push
```

The push runs `release.yaml`, which refreshes the open release PR and re-runs the harness on it. Leave the PR open.

- [ ] **Step 10: Delete the SDD workspace** (side effect 3) — after checking that the phase log reproduces the ledger: `rm -rf .superpowers/sdd`, then `git status --short` prints nothing.
