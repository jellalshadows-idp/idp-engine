# Phase 2b — The Deterministic Env Variables Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render and create the GitHub environment variables of a Component with `aws.registry: true`. For each of its environments, the renderer passes `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION`, computed by `internal/naming` from the names and the environment's `aws` block (spec §4.4, §5.5), and `modules/github/component` creates one `github_actions_environment_variable` per environment and name. A Component without `aws.registry` renders exactly as today, byte for byte, and its stack plans no change. The phase also closes the five review items that Phase 2a carried.

**Architecture:**
- **The module takes the variables inside each environment,** as spec §5.3's rendered example already shows: `environments.<env>.variables`, an optional `map(string)` that defaults to `{}`. A local flattens them into one entry per environment and name, keyed `"<env>/<name>"`, and each variable reads its environment from `github_repository_environment.this[<env>]`, so it is created after that environment and destroyed before it. With no variables the resource has no instances.
- **The renderer adds `variables` only for a registry Component,** and only to the environments it lists. It fails, rather than render less, when such an environment has no `aws` block, which `claims.Load` already rejects (Phase 2a, spec §4.7).
- **`TestRenderIgnoresTheAWSPart` becomes the env-variables golden** (`TestRenderEnvVariablesGolden`), with the `aws-registry` fixture's own golden file; 2a's guard that the `aws` blocks alone change nothing survives as a separate test against `basic`'s golden file, which stays unchanged.
- **CI's `render-smoke` validates both fixtures** against the local modules, so the rendered `variables` are proven to fit the module's type.
- Only the github stack is rendered (the AWS stacks are Phase 2d), and the pipelines are unchanged.

**Tech Stack:** Go 1.26 (standard library; no new dependencies), OpenTofu modules (`integrations/github` 6.13.0, unchanged), one CI job's steps. Local `tofu test` with OpenTofu 1.12.6; CI runs 1.13.1 (D-14).

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §3.3, §4.4, §5.3, §5.5, §8.3, §10, §11 and amendment A10. Context: `docs/phases/phase-2a.md` ("Carried into Phase 2b", "Notable findings caught by review"), the Phase 2a plan, ADR-0006, ADR-0013.

## Phase 2 split (owner decision, 2026-11-03)

Phase 2 is delivered as six plans, each written after the previous one ships (Phase 2a ruling D-1, spec §10 under A10).

| Plan | Delivers |
|---|---|
| 2a (shipped, PR #29, `fca5c0b`) | The AWS model of the Platform and the Component, `internal/naming`, the schema and semantic validation, spec amendment A10 |
| **2b (this plan)** | The deterministic env variables (`AWS_ROLE_ARN`, `ECR_REPOSITORY`, `AWS_REGION`) in `modules/github/component`, rendered from `internal/naming`; the 2a review items; spec amendment A11 |
| 2c | `modules/aws/{baseline,component}` (the OIDC provider; the ECR repository and the CI role), the AWS lock file and the floci CI job (spec §8.4) |
| 2d | Rendering the AWS stacks (`aws/<env>/_baseline`, `aws/<env>/components/<name>`) |
| 2e | The workflows' AWS plan and apply against floci |
| 2f | The E2E env-variable checks, the `0.2.0` release and the Phase 2 log |

## Decisions taken while planning

The executor copies these into the ledger as rulings before Task 1. Phase 2a's owner decisions still hold: Phase 2 is split (2a D-1), and v1 is floci only, with no real AWS account (2a D-2). D-1 onwards below were decided while planning, from verified facts; facts read live are dated 2026-11-03.

- **D-1. The input shape: `variables` inside each entry of `environments`.** The type becomes `map(object({ protected = bool, variables = optional(map(string), {}) }))`. This is spec §5.3's own rendered example (`"environments": { "pro": { "protected": true, "variables": { "AWS_ROLE_ARN": …, "ECR_REPOSITORY": …, "AWS_REGION": … } } }`), so §5.3 stays exact and needs no amendment. The alternative, a separate `variables` input of type `map(map(string))` keyed by environment, would contradict §5.3's shape and need a validation across two inputs. Existing callers keep working: an environment without `variables` gets `{}`. *Cost if wrong:* moving to a separate input changes the module's `variables.tf` and `main.tf`, its three new runs, six lines of the render and one golden file; nothing is applied from a release before `0.2.0`.
- **D-2. No module validation, so no validation message.** A variable can be declared only inside an environment entry, and that entry is the environment it belongs to: a variable for an environment the module does not create cannot be expressed. Variable names are not restricted (D-4). The module run `only_environments_with_variables_get_them` replaces the "unknown environment" test that a separate input would have needed: each variable goes to the environment that declares it, and an environment without variables gets none. On the render side, `TestRenderEnvVariablesFollowEachEnvironment` shows that an environment the Component does not list gets nothing, even when the platform gives it an `aws` block. *Cost if wrong:* a validation can be added later without rejecting any input that is valid today.
- **D-3. The resource address is `github_actions_environment_variable.this["<env>/<name>"]`.** `local.environment_variables` builds the keys with `merge([for env, e in var.environments : {for name, value in e.variables : "${env}/${name}" => {…}}]...)`. Environment names match `^[a-z][a-z0-9]{0,9}$` (spec §4.6), so they never contain a slash, and each key names exactly one variable. OpenTofu orders `for_each` keys lexically, so the plan lists them in a stable order. Each instance takes its `environment` from `github_repository_environment.this[each.value.environment].environment`, like the existing deployment policy, so it depends on that environment. Verified with OpenTofu 1.12.6: `merge([]...)` is `{}`, so an empty `environments` map works. *Cost if wrong:* renaming the keys later moves state addresses (one `moved` block per key); before any registry Component is applied there is no state to move.
- **D-4. Only what the render passes (YAGNI).** The type is a plain `map(string)`: the module neither restricts the names nor adds anything for non-AWS variables. Its only caller is the renderer, which passes exactly the three names; the golden file pins them. *Cost if wrong:* none; a later caller passes other names without a module change, or a validation restricts them then.
- **D-5. Existing state is untouched.** For a Component without `aws.registry` the new resource has zero instances; input types are not stored in state; no existing resource is renamed. Live facts: neither `idp-claims` nor `idp-claims-e2e` has any file under `claims/` on `main`; production `idp-claims` pins the engine at `v0.1.1`; the harness's claims (`internal/e2e/claims.go`) carry no `aws` block. Proof in this plan: `basic`'s golden file is unchanged byte for byte (Task 4), the module run `environments_have_no_variables_by_default` (Task 3), and live, the harness on the release PR, whose pin PR and case-only step both require `No infrastructure changes.` (watch list). *Cost if wrong:* an unexpected create would fail the harness's pin or case-only step before any release.
- **D-6. The renderer passes `variables` when, and only when, the Component has `aws.registry: true`,** for each environment it lists. The values come from `naming.RoleARN(accountID, component, env)`, `naming.RegistryURL(accountID, region, component)` and the region, through `literal` like every other value from the claims. A registry Component in an environment without an `aws` block makes `Render` return the error `render: component "<name>" has aws.registry, but environment "<env>" has no aws block`. It must never panic, and never render the Component without its variables, because that render would plan their deletion. `claims.Load` already reports that case (2a's D-8), so the error reaches only a caller that bypassed `Load`. *Cost if wrong:* none; the message is seen only by such a caller.
- **D-7. The render tests.** `TestRenderIgnoresTheAWSPart` becomes `TestRenderEnvVariablesGolden`. It compares the `aws-registry` fixture with its own golden file, `internal/render/testdata/aws-registry/expected/github/main.tf.json`, and checks `pro`'s variables against spec §5.3's values verbatim, outside the golden file, so that `-update` cannot change them unnoticed. 2a's interim guard becomes `TestRenderWithoutRegistryPassesNoVariables`: the same fixture with `registry: false` must render `basic`'s golden file. `TestRenderIsDeterministic` moves to the `aws-registry` fixture and also shuffles each Component's environments. A helper `checkGolden` serves both golden tests. `TestRenovateCustomManagersFindThePins` must list the new golden file: its file walk finds it, and the existing Renovate patterns (`internal/render/testdata/[^/]+/expected/github/main.tf.json`) already move its OpenTofu and provider pins. Verified: without that test change, adding the golden file fails the test. *Cost if wrong:* none.
- **D-8. Carried minor (c): document "standard `aws` partition only"; do not narrow the region pattern.** Naming's identities use `arn:aws:` and `amazonaws.com`. The platform schema checks only a region's shape, so it also accepts GovCloud (`us-gov-*`) and China (`cn-*`) regions, whose ARNs and hosts differ. The schema validator and naming's tests use Go's `regexp`, which has no lookahead, so narrowing would mean an allowlist of region prefixes, which a future standard region could break. In v1 every region is the floci emulator, where the partition does not matter. Task 2 documents the limit in naming's package doc and in `docs/claims.md`. The `schemas_test.go` row "aws region with three parts" (`us-gov-west-1`, valid) stays as it is, on purpose. *Cost if wrong:* going live in another partition needs naming to derive the partition and the host suffix from the region, and narrowing the schema then would reject configs that use those regions; none does.
- **D-9. Carried minor (b): future tense, and `aud` in §8.3 now.** ADR-0013's S3 note and spec §11 risk 4 say, in the present tense, that module tests assert the trust policy. Those tests arrive with the AWS module in 2c, so both become "will be asserted by the AWS module tests (Phase 2c)". Spec §8.3 lists only `sub`, while §4.4, §11 and ADR-0013 name `sub` and `aud`. A11 adds `aud` to §8.3 now: the reworded sentences point at §8.3, and 2c then implements one complete list. *Cost if wrong:* one line of spec text.
- **D-10. Amendment A11 covers only text that 2b makes stale or that a carried item fixes:** §3.3's `internal/claims` and `internal/config` rows (minor (d)), §8.3 (D-9) and §11 risk 4 (minor (b)). Not stale: §4.4; §5.3, whose shape D-1 keeps and whose values `TestRenderEnvVariablesGolden` pins; §5.5; §6; §10. `docs/pipelines.md` ("Only the `github` stack is reconciled for now") stays true until 2e. *Cost if wrong:* none.
- **D-11. Carried item (e), R3-001 at `docs/claims.md:101`: a real wording issue.** "nothing renders from it until Phases 2b to 2d" reads as "until a range of phases", does not say which part arrives when, and is no longer true once 2b ships. Task 4 rewrites the bullet. *Cost if wrong:* wording only.
- **D-12. When GitHub creates the variables: when the pipelines apply the github stack, which they already do.** Rendering creates nothing. Reconcile's `apply-github` applies the whole github stack, so once a claims repo runs an engine with 2b, a merged registry Component gets its variables. No App permission changes. GitHub's "Permissions required for GitHub Apps" page lists `POST`, `PATCH` and `DELETE /repos/{owner}/{repo}/environments/{environment_name}/variables` under the "Environments" repository permission (write), which the writer App has (`bootstrap/apps/writer.json`). The reads used to refresh the stack need "Environments" (read), which the reader App has. This corrects the brief's "their application is still Phase 2e": what waits for 2d and 2e is the ECR repository and the CI role that the variables name. No claims repo runs an engine with 2b before `0.2.0` (2a's D-15). `docs/claims.md` says this, and a delete (turning `aws.registry` off) waits for approval, like every delete. *Cost if wrong:* documentation wording.
- **D-13. `render-smoke` also validates the `aws-registry` render** in the same job, by looping over the two fixtures, so the check's name stays `render-smoke`. Verified with OpenTofu 1.12.6: `tofu validate` rejects a rendered `variables` value of the wrong type (`attribute "variables": map of string required`). It silently drops an attribute the module does not declare, so it cannot catch a rename. The attribute's name is therefore pinned on both sides instead: the module runs count instances created from `variables`, and the render test reads `environments.pro.variables`. *Cost if wrong:* a few seconds more of `render-smoke`.
- **D-14. Local `tofu test` uses OpenTofu 1.12.6; CI's 1.13.1 is the source of truth.** The binary is `~/.idp/tools/tofu-1.12.6/tofu.exe`. It is executed, never written to, as `docs/runbooks/dependencies.md` already does. The module is copied into a `mktemp -d` directory for `init` and `test`, so the worktree gets no `.terraform`. No OpenTofu binary is downloaded; `init` fetches only the provider, checked against the shipped lock file. The module's `required_version` is `>= 1.12.0`, and nothing this plan uses is newer than 1.12. Verified: on 1.12.6 the module's 9 runs pass on `main`, the RED of Task 3 is as written, and the 12 runs pass after it. *Cost if wrong:* one CI round trip if 1.13.1 differs.
- **D-15. The lock file and the provider pin are unchanged.** `github_actions_environment_variable` exists in `integrations/github` 6.13.0. It requires `repository`, `environment`, `variable_name` and `value`, and computes `created_at`, `updated_at` and `repository_id` (read with `tofu providers schema -json`). *Cost if wrong:* none.
- **D-16. No release in 2b.** After the merge, release PR #28 is refreshed and still proposes `0.2.0`; it stays open until 2f (2a's D-15). Every push to `main` re-runs the harness on it, about 20 minutes each. *Cost if wrong:* if it proposes another version, the rule is the same: do not merge it.
- **D-17. No new ADR.** ADR-0006 (identities computed by `internal/naming`, never read from AWS) decides 2b. A11 records the spec edits, and ADR-0013 gets one amended sentence (D-9). *Cost if wrong:* one ADR later.

## Global Constraints

**Names** (copy verbatim)
- Branch `feat/phase-2b-env-variables`; one PR, merged with a merge commit.
- `modules/github/component`: the input `environments` of type `map(object({ protected = bool, variables = optional(map(string), {}) }))`; `local.environment_variables`; the resource `github_actions_environment_variable.this`, keyed `"<env>/<name>"`; the runs `environments_have_no_variables_by_default`, `variables_for_two_environments` and `only_environments_with_variables_get_them`.
- `internal/render`: `func githubStack(m *claims.Model, opts Options) (map[string]any, error)`; `func envVariables(component, env string, aws *claims.AWS) (map[string]string, error)`; the variable names `AWS_ROLE_ARN`, `ECR_REPOSITORY`, `AWS_REGION`; the error of D-6, verbatim: `render: component %q has aws.registry, but environment %q has no aws block`.
- Render tests: `checkGolden`, `renderedEnvironment`, `componentEnvironments`, `TestRenderEnvVariablesGolden` (renamed from `TestRenderIgnoresTheAWSPart`), `TestRenderWithoutRegistryPassesNoVariables`, `TestRenderEnvVariablesFollowEachEnvironment`, `TestRenderRejectsARegistryEnvironmentWithoutAWS`; the golden file `internal/render/testdata/aws-registry/expected/github/main.tf.json`.
- Spec amendment **A11**; phase log `docs/phases/phase-2b.md`.

**Process**
- Strict TDD: failing test, run it to see it fail, implement, run it to see it pass, commit. Tests that pin existing behavior say so and pass before the change; a mutation check shows what they catch.
- No `go build` and no GoReleaser run locally (no `go run` either). Use `go test ./...`, `go vet ./...` and `gofmt -l .` (must print nothing); `-race` runs in CI.
- Module changes run `tofu fmt -check` and `tofu test` locally with the module test snippet below, and always in CI (`modules (github/component)`). The workflow change of Task 5 is checked only in CI (actionlint, zizmor, `render-smoke`).
- The lock file `lockfiles/github.terraform.lock.hcl` and every provider pin are unchanged in 2b.
- Shell rule: never use `cat`, `grep`, `find`, `sed` or `ls`, never write files with shell heredocs, never `perl`. Use the Read/Write/Edit tools, `rg`, `gh`, `git`, `python -I`.
- Conventional commits, never with Co-Authored-By or any AI attribution.
- Docs in English. Owner rule: docs describe the platform on its own terms, never naming another product as inspiration or comparison, and never use the owner's work account; the controller gives each dispatch the naming check that enforces both (it must print nothing).
- Doc-only changes after the merge go straight to `main`; code goes through PRs.
- Never write into `~/.idp/`. Running `~/.idp/tools/tofu-1.12.6/tofu.exe` only reads it (D-14).
- Scratch state lives in `mktemp -d` directories outside the worktree, removed after use.
- Risk lines: Tasks 1, 2, 3 and 4 `Risk: none` (Task 3 adds an optional attribute whose default creates nothing, D-5); Task 5 `Risk: item 5 (CI)`, checked by the PR's CI run and the final whole-branch review.

**Module test snippet** (Git Bash, from the repository root; D-14):

```bash
TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"
"$TOFU" fmt -check -recursive modules/github/component
MOD="$(mktemp -d)"
cp -r modules/github/component/. "$MOD"
cp lockfiles/github.terraform.lock.hcl "$MOD/.terraform.lock.hcl"
(cd "$MOD" && "$TOFU" init -input=false -lockfile=readonly >/dev/null && "$TOFU" test -no-color)
rm -rf "$MOD"
```

`fmt -check` prints nothing when the files are formatted. `init` downloads the provider from `registry.opentofu.org` and checks it against the shipped lock file.

**Spec rules**
- No real cloud credential anywhere (spec §7.6, 2a's D-2): nothing in this plan creates an AWS account, credential or resource.
- The AWS values reach the GitHub stack only from `internal/naming`, never from AWS state (spec §5.5, ADR-0006).
- Rendering stays pure. In 2b it renders only the github stack (the AWS stacks are 2d).

**Owner actions:** none beyond approving this plan and its side effects.

**Side effects this plan authorizes** (controller, after the owner approves the plan)
1. Push `feat/phase-2b-env-variables`, open its PR, merge it after CI and the final review.
2. Doc-only commits straight to `main`: this plan, the phase log, the README.
3. Delete the local, git-ignored SDD workspace `.superpowers/sdd/` after the phase log is committed.

Not authorized, stop and ask: merging any release PR (including #28, which proposes `0.2.0`, D-16), merging a Renovate PR, changing any repository or org setting, App, ruleset, environment, variable or secret, any change to `idp-claims` or `idp-claims-e2e`, creating any cloud resource.

## Review Focus

1. **A registry Component whose environments are in different accounts and regions.**
   - **Expected:** each environment's variables use its own account ID and region; nothing is shared or taken from another environment.
   - **Pinned by:** `TestRenderEnvVariablesFollowEachEnvironment` (Task 4: `dev` in `000000000001`/`eu-west-1`, `pro` in `000000000003`/`us-east-1`); the module run `variables_for_two_environments` (Task 3, two regions). Mutation-checked in Task 4.
2. **A Component without `aws.registry`, including one whose environments have `aws` blocks.**
   - **Expected:** no `variables` key, a render that is byte-identical to today's, and zero variable resources, so existing stacks plan no change.
   - **Pinned by:** `TestRenderGolden` against `basic`'s unchanged golden file and `TestRenderWithoutRegistryPassesNoVariables` (Task 4); the module run `environments_have_no_variables_by_default` (Task 3), mutation-checked.
3. **`aws.registry` set but an environment has no `aws` block (a model `Load` rejects).**
   - **Expected:** `Render` returns D-6's error and no files; no panic, and no render without the variables.
   - **Pinned by:** `TestRenderRejectsARegistryEnvironmentWithoutAWS` (Task 4).
4. **Variables for an environment the Component does not list.**
   - **Expected:** impossible in the module by construction (D-2); the render passes variables only for the listed environments, even when the platform has another environment with an `aws` block.
   - **Pinned by:** the module run `only_environments_with_variables_get_them` (Task 3); `TestRenderEnvVariablesFollowEachEnvironment`, whose `staging` has an `aws` block and gets nothing (Task 4).
5. **Deterministic ordering and addresses.**
   - **Expected:** the module's keys are exactly `"<env>/<name>"`; the render's bytes do not depend on the order of claims or of a Component's environments.
   - **Pinned by:** the module run `variables_for_two_environments` (Task 3, the exact sorted key list); `TestRenderIsDeterministic` on the `aws-registry` fixture with shuffled environments and the `aws-registry` golden file (Task 4).

Also tested, outside the five: spec §5.3's values, verbatim (Task 4); Renovate moves the pins of both golden files (Task 4); the rendered `aws-registry` stack validates against the local modules (Task 5, CI); the carried naming cases `aBc`, `dEv` and `deV` (Task 2).

## Watch list for the live steps

Record each result in the ledger:
- After the merge, release PR #28 is refreshed and still proposes `0.2.0` (D-16). Task 6 Step 6. Record its title and leave it open.
- The harness run dispatched for that release PR passes (D-5). Its success means the pin PR and the case-only step both planned `No infrastructure changes.` with the new module. Task 6 Step 6. A failure is a regression to fix before 2c.

## File Structure

```
docs/superpowers/specs/2026-10-08-idp-on-actions-design.md (A11), docs/adr/0013-phase-0-spike-findings.md                 # Task 1
internal/naming/naming_test.go, internal/naming/naming.go, docs/claims.md                                               # Task 2
modules/github/component/variables.tf, modules/github/component/main.tf, modules/github/component/tests/component.tftest.hcl  # Task 3
internal/render/render.go, internal/render/render_test.go, internal/render/testdata/aws-registry/expected/github/main.tf.json,
internal/workflowspec/renovate_test.go, docs/claims.md, docs/runbooks/dependencies.md                                   # Task 4
.github/workflows/ci.yaml                                                                                               # Task 5
docs/phases/phase-2b.md, README.md                                                                                      # Task 6
```

## Pre-flight (controller, before Task 1)

- [ ] `gh api user --jq .login` prints `jellalshadows` (else `gh auth switch -u jellalshadows`).
- [ ] `git switch main && git pull --ff-only`, then `git log -1 --format=%h` prints `854c62a` or a later commit, and `git status --short` prints nothing.
- [ ] `go version` prints `go1.26` or a later 1.26 patch, and `go test ./...` passes on `main`, so any later failure is this plan's.
- [ ] `"$HOME/.idp/tools/tofu-1.12.6/tofu.exe" version` prints `OpenTofu v1.12.6`, and the module test snippet on `main` ends with `Success! 9 passed, 0 failed.`
- [ ] The owner approves this plan, its side effects and the rulings D-1 to D-17; ledger them. Commit this plan to `main` as `docs: add the phase 2b plan` and push. The push refreshes release PR #28 and re-runs the harness on it; leave the PR open (D-16).

---

### Task 1: Decisions first — spec amendment A11 and ADR-0013

**Files:**
- Modify: `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, `docs/adr/0013-phase-0-spike-findings.md`

**Interfaces:**
- Produces: spec §3.3 without the never-extracted `internal/config` dependency (carried minor (d)); §8.3 listing the trust policy's `sub` and `aud`; §11 risk 4 and ADR-0013 in the future tense (carried minor (b), D-9).

- [ ] **Step 1: Create the branch**

Run: `git switch main && git pull --ff-only && git switch -c feat/phase-2b-env-variables`

- [ ] **Step 2: Apply amendment A11 to the spec** (exact replacements with Edit)

1. §3.3 units table, old:

```markdown
| `internal/claims` | Load, parse and validate claims (schema + semantics) | `internal/config`, JSON Schemas |
| `internal/config` | Load `config/platform.yaml` | — |
```

new:

```markdown
| `internal/claims` | Load, parse and validate claims (schema + semantics), and load `config/platform.yaml` | JSON Schemas *(Amendment A11)* |
| `internal/config` | Not extracted (A4, A10): `internal/claims` loads `config/platform.yaml` *(Amendment A11)* | — |
```

2. §8.3, old:

```markdown
- The trust policy `sub` is `repo:<org>/<name>:environment:<env>`.
```

new:

```markdown
- The trust policy `sub` is `repo:<org>/<name>:environment:<env>`, and its `aud` is
  `sts.amazonaws.com` (§4.4). *(Amendment A11, 2026-11-03.)*
```

3. §11, risk 4, old:

```markdown
Real `AssumeRoleWithWebIdentity` not exercised; out of scope for v1, which has no real AWS account (§2.2, §7.6): the module tests assert the trust policy (§8.3) (owner decision; *Amendment A10, 2026-11-03*) |
```

new:

```markdown
Real `AssumeRoleWithWebIdentity` not exercised; out of scope for v1, which has no real AWS account (§2.2, §7.6): the AWS module tests will assert the trust policy's `sub` and `aud` (Phase 2c, §8.3) (owner decision; *Amendment A10, 2026-11-03; reworded by Amendment A11, 2026-11-03*) |
```

- [ ] **Step 3: Amend ADR-0013** (exact replacement with Edit, in "S3 — floci", the notes), old:

```markdown
The trust policy's `sub` and `aud` are asserted by the module tests (spec §8.3).
```

new:

```markdown
The trust policy's `sub` and `aud` will be asserted by the AWS module tests (Phase 2c, spec §8.3). *(Amended in Phase 2b, 2026-11-03; this said that the module tests assert them.)*
```

- [ ] **Step 4: Check and commit**

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
- `rg -c "Amendment A11" docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` prints `4`;
- `rg -c "Amendment A10" docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` prints `6` (unchanged);
- `rg -c "Amended in Phase 2b" docs/adr/0013-phase-0-spike-findings.md` prints `1`;
- `rg -n 'are asserted by the module tests|the module tests assert the trust policy|`internal/config`, JSON Schemas' docs/adr docs/superpowers/specs` prints nothing;
- the naming check prints nothing.

```bash
git add docs/superpowers/specs/2026-10-08-idp-on-actions-design.md docs/adr/0013-phase-0-spike-findings.md
git commit -m "docs: reword the trust policy test claims and the config unit (spec amendment a11)"
```

---

### Task 2: Naming — uppercase letters after the first position, and the partition

**Files:**
- Modify: `internal/naming/naming_test.go`, `internal/naming/naming.go`, `docs/claims.md`

**Interfaces:**
- Consumes: `TestSchemaNameRulesKeepAWSNamesValid`, `nameRules` (`internal/naming/naming_test.go`); `schemas/component.json`.
- Produces: the cases `aBc` (Component rule) and `dEv`, `deV` (environment rule), carried minor (a); naming's package doc and the `docs/claims.md` region row stating "standard `aws` partition only", carried minor (c), D-8.

- [ ] **Step 1: Show the gap** (nothing is committed)

In `schemas/component.json`, Edit the Component name rule, old:

```json
    "name": { "type": "string", "pattern": "^[a-z][a-z0-9-]{0,38}[a-z0-9]$" },
```

new:

```json
    "name": { "type": "string", "pattern": "^[a-z][a-zA-Z0-9-]{0,38}[a-z0-9]$" },
```

Run: `go test ./internal/naming/ -count=1`
Expected: `ok`. Today's cases (`Api`, `API`) put the uppercase letter first, so a rule that allows one later goes unnoticed. Keep the edit for Step 3.

- [ ] **Step 2: Add the cases** — `internal/naming/naming_test.go`, in `TestSchemaNameRulesKeepAWSNamesValid` (Edit), old:

```go
		{"API", false},
```

new:

```go
		{"API", false},
		{"aBc", false},
```

and old:

```go
	for _, env := range []string{"Dev", "DEV", "pro-1", "1dev", "dev_1"} {
```

new:

```go
	for _, env := range []string{"Dev", "DEV", "dEv", "deV", "pro-1", "1dev", "dev_1"} {
```

- [ ] **Step 3: Run them against the widened rule to see them fail**

Run: `go test ./internal/naming/ -count=1`
Expected: `FAIL`, with `--- FAIL: TestSchemaNameRulesKeepAWSNamesValid/aBc` and `the schema allows "aBc": true, want false`.

Then restore the rule: `git checkout schemas/component.json`, and `git status --short` lists only `internal/naming/naming_test.go`.

- [ ] **Step 4: Run them to see them pass** (they pin the real schema's existing behavior)

Run: `go test ./internal/naming/ -v -run TestSchemaNameRulesKeepAWSNamesValid -count=1`
Expected: `PASS`, with 13 subtests listed (`aBc` among them).

- [ ] **Step 5: Commit**

```bash
git add internal/naming/naming_test.go
git commit -m "test(naming): reject uppercase letters after the first position"
```

- [ ] **Step 6: Document the partition** (Edit)

`internal/naming/naming.go`, old:

```go
// environment name of at most 10), so none can fail.
package naming
```

new:

```go
// environment name of at most 10), so none can fail.
//
// The identities are those of the standard aws partition: ARNs start with
// arn:aws: and the registry host ends in amazonaws.com. The platform schema
// checks only a region's shape, so it also accepts GovCloud (us-gov-*) and China
// (cn-*) regions, whose identities differ; they are not supported. In v1 every
// region is the floci emulator (spec §5.6), where the partition does not matter.
// Going live in another partition means deriving the partition and the host
// suffix from the region here.
package naming
```

`docs/claims.md`, the Platform table, old:

```markdown
| `environments.<env>.aws.region` | yes, in `aws` | | An AWS region, such as `eu-west-1` |
```

new:

```markdown
| `environments.<env>.aws.region` | yes, in `aws` | | An AWS region of the standard `aws` partition, such as `eu-west-1`. The schema checks only the shape, so GovCloud (`us-gov-*`) and China (`cn-*`) regions pass it, but they are not supported: the identities are computed for the standard partition (`arn:aws:` ARNs and the `amazonaws.com` registry host) |
```

- [ ] **Step 7: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing); the naming check prints nothing.

- [ ] **Step 8: Commit**

```bash
git add internal/naming/naming.go docs/claims.md
git commit -m "docs(naming): support only the standard aws partition"
```

---

### Task 3: The module creates the environments' variables

**Files:**
- Modify: `modules/github/component/variables.tf`, `modules/github/component/main.tf`, `modules/github/component/tests/component.tftest.hcl`

**Interfaces:**
- Consumes: `github_repository.this`, `github_repository_environment.this`, the test file's `mock_provider "github" {}` and its file-level `variables` (environments `dev` and `pro`, without variables).
- Produces: `environments.<env>.variables` (`optional(map(string), {})`, D-1); `local.environment_variables`; `github_actions_environment_variable.this["<env>/<name>"]` (D-3). Task 4's render and Task 5's `render-smoke` depend on this shape.

- [ ] **Step 1: Write the failing runs** — `modules/github/component/tests/component.tftest.hcl` (Edit), old:

```hcl
run "rejects_out_of_range_approvals" {
```

new:

```hcl
run "environments_have_no_variables_by_default" {
  command = plan

  assert {
    condition     = length(github_actions_environment_variable.this) == 0
    error_message = "environments without variables must create no variable, so a Component without aws.registry plans no change"
  }
}

run "variables_for_two_environments" {
  command = plan

  variables {
    environments = {
      dev = {
        protected = false
        variables = {
          AWS_ROLE_ARN   = "arn:aws:iam::000000000001:role/api-dev-ci"
          ECR_REPOSITORY = "000000000001.dkr.ecr.eu-west-1.amazonaws.com/api"
          AWS_REGION     = "eu-west-1"
        }
      }
      pro = {
        protected = true
        variables = {
          AWS_ROLE_ARN   = "arn:aws:iam::000000000003:role/api-pro-ci"
          ECR_REPOSITORY = "000000000003.dkr.ecr.us-east-1.amazonaws.com/api"
          AWS_REGION     = "us-east-1"
        }
      }
    }
  }

  assert {
    condition     = sort(keys(github_actions_environment_variable.this)) == tolist(["dev/AWS_REGION", "dev/AWS_ROLE_ARN", "dev/ECR_REPOSITORY", "pro/AWS_REGION", "pro/AWS_ROLE_ARN", "pro/ECR_REPOSITORY"])
    error_message = "one variable per environment and name, addressed \"<env>/<name>\""
  }
  assert {
    condition     = alltrue([for k, v in github_actions_environment_variable.this : v.repository == "api" && "${v.environment}/${v.variable_name}" == k])
    error_message = "each variable must belong to its own environment of the component's repository"
  }
  assert {
    condition     = github_actions_environment_variable.this["dev/AWS_ROLE_ARN"].value == "arn:aws:iam::000000000001:role/api-dev-ci" && github_actions_environment_variable.this["pro/ECR_REPOSITORY"].value == "000000000003.dkr.ecr.us-east-1.amazonaws.com/api" && github_actions_environment_variable.this["pro/AWS_REGION"].value == "us-east-1"
    error_message = "each variable must carry its own environment's value"
  }
  assert {
    condition     = length(github_repository_environment.this) == 2 && length(github_repository_environment.this["pro"].reviewers) == 1 && length(github_repository_environment.this["dev"].reviewers) == 0
    error_message = "variables must not change the environments themselves"
  }
}

run "only_environments_with_variables_get_them" {
  command = plan

  variables {
    environments = {
      dev = { protected = false }
      pro = { protected = true, variables = { AWS_REGION = "eu-west-1" } }
    }
  }

  assert {
    condition     = sort(keys(github_actions_environment_variable.this)) == tolist(["pro/AWS_REGION"]) && github_actions_environment_variable.this["pro/AWS_REGION"].environment == "pro"
    error_message = "a variable belongs to the environment that declares it, and an environment without variables gets none"
  }
}

run "rejects_out_of_range_approvals" {
```

In expressions a resource with `for_each` is an object, and `keys` of it never equals a `tolist([...])` (seen on OpenTofu 1.12.6), so the runs sort the keys into a list first, as the existing runs compare lists.

- [ ] **Step 2: Run them to see them fail** — the module test snippet.

Expected: `fmt -check` prints nothing; `run "environments_have_no_variables_by_default"... fail` with `Reference to undeclared resource` and `There is no managed resource "github_actions_environment_variable" "this" definition in the root module.`; the later runs are skipped; the summary is `Failure! 8 passed, 1 failed, 3 skipped.`

- [ ] **Step 3: Implement the input** — `modules/github/component/variables.tf` (Edit), old:

```hcl
variable "environments" {
  description = "GitHub environments to create; protected ones require the owner team's approval."
  type = map(object({
    protected = bool
  }))
  default = {}
}
```

new:

```hcl
variable "environments" {
  description = "GitHub environments to create; protected ones require the owner team's approval. Each environment's variables become its GitHub Actions variables, by name."
  type = map(object({
    protected = bool
    variables = optional(map(string), {})
  }))
  default = {}
}
```

- [ ] **Step 4: Implement the resource** — `modules/github/component/main.tf` (Edit), old:

```hcl
resource "github_repository_environment_deployment_policy" "default_branch" {
  for_each = var.environments

  repository     = github_repository.this.name
  environment    = github_repository_environment.this[each.key].environment
  branch_pattern = var.default_branch
}
```

new:

```hcl
resource "github_repository_environment_deployment_policy" "default_branch" {
  for_each = var.environments

  repository     = github_repository.this.name
  environment    = github_repository_environment.this[each.key].environment
  branch_pattern = var.default_branch
}

locals {
  # One entry per environment variable, keyed "<env>/<name>". Environment names
  # never contain a slash (spec §4.6), so each key names exactly one variable,
  # and the addresses read github_actions_environment_variable.this["pro/AWS_REGION"].
  environment_variables = merge([
    for env, e in var.environments : {
      for name, value in e.variables : "${env}/${name}" => {
        environment = env
        name        = name
        value       = value
      }
    }
  ]...)
}

# The environments' GitHub Actions variables. The renderer passes them only for a
# Component with aws.registry: AWS_ROLE_ARN, ECR_REPOSITORY and AWS_REGION, computed
# from the names and the environment's aws block (spec §4.4, §5.5). Without
# variables this resource has no instances, so existing stacks plan no change.
resource "github_actions_environment_variable" "this" {
  for_each = local.environment_variables

  repository    = github_repository.this.name
  environment   = github_repository_environment.this[each.value.environment].environment
  variable_name = each.value.name
  value         = each.value.value
}
```

- [ ] **Step 5: Run them to see them pass** — the module test snippet.

Expected: `fmt -check` prints nothing, every run passes, and the summary is `Success! 12 passed, 0 failed.`

- [ ] **Step 6: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing); `git status --short` lists only the three module files (the snippet ran in a temporary copy).

- [ ] **Step 7: Commit**

```bash
git add modules/github/component
git commit -m "feat(modules): create the environments' github actions variables"
```

- [ ] **Step 8: Mutation check** (prove the default-zero run bites; nothing is committed)

In `modules/github/component/main.tf`, change `for name, value in e.variables :` to `for name, value in merge(e.variables, { MUTANT = "1" }) :`. Run the module test snippet. Expected: `Failure! 9 passed, 3 failed.`, with `environments_have_no_variables_by_default`, `variables_for_two_environments` and `only_environments_with_variables_get_them` failing. Restore it with `git checkout modules/github/component/main.tf`; `git status --short` prints nothing.

---

### Task 4: The render passes the env variables

**Files:**
- Create: `internal/render/testdata/aws-registry/expected/github/main.tf.json`
- Modify: `internal/workflowspec/renovate_test.go`, `internal/render/render_test.go`, `internal/render/render.go`, `docs/claims.md`, `docs/runbooks/dependencies.md`

**Interfaces:**
- Consumes: `naming.RoleARN`, `naming.RegistryURL` (`internal/naming`); `claims.Model`, `claims.Environment`, `claims.AWS`, `Component.AWS.Registry`; `loadFixture`, `update`, the fixture `internal/render/testdata/aws-registry/` (2a, unchanged); `TestRenovateCustomManagersFindThePins`, `repoFiles`; Task 3's module shape.
- Produces: `githubStack` returning `(map[string]any, error)`; `envVariables`; D-6's error; the `aws-registry` golden file; the tests of D-7.

- [ ] **Step 1: Write the expected render** — create `internal/render/testdata/aws-registry/expected/github/main.tf.json` (Write tool). It is `basic`'s golden file plus each environment's `variables`; `pro`'s are spec §5.3's:

```json
{
  "module": {
    "component_api": {
      "archive_on_destroy": true,
      "description": "Orders API $${not.evaluated} %%{ literal }",
      "environments": {
        "dev": {
          "protected": false,
          "variables": {
            "AWS_REGION": "eu-west-1",
            "AWS_ROLE_ARN": "arn:aws:iam::000000000001:role/api-dev-ci",
            "ECR_REPOSITORY": "000000000001.dkr.ecr.eu-west-1.amazonaws.com/api"
          }
        },
        "pro": {
          "protected": true,
          "variables": {
            "AWS_REGION": "eu-west-1",
            "AWS_ROLE_ARN": "arn:aws:iam::000000000003:role/api-pro-ci",
            "ECR_REPOSITORY": "000000000003.dkr.ecr.eu-west-1.amazonaws.com/api"
          }
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
    "required_version": "1.13.1"
  }
}
```

- [ ] **Step 2: See the Renovate guard bite**

Run: `go test ./internal/workflowspec/ -run TestRenovateCustomManagersFindThePins -count=1`
Expected: `FAIL`. The `found` map lists `internal/render/testdata/aws-registry/expected/github/main.tf.json=6.13.0` under `github` and `…=1.13.1` under `opentofu`, which `want` lacks. The existing patterns already match every fixture's golden file, so Renovate moves this file's pins too; the test must say so.

- [ ] **Step 3: List the new golden file** — `internal/workflowspec/renovate_test.go`, in `TestRenovateCustomManagersFindThePins` (Edit), old:

```go
	ci, rn := ".github/workflows/ci.yaml", renovatePath
	golden := "internal/render/testdata/basic/expected/github/main.tf.json"
```

new:

```go
	ci, rn := ".github/workflows/ci.yaml", renovatePath
	// Every golden render carries both pins, so Renovate must move them all.
	awsGolden := "internal/render/testdata/aws-registry/expected/github/main.tf.json"
	golden := "internal/render/testdata/basic/expected/github/main.tf.json"
```

and old:

```go
		"opentofu":                    {"internal/render/render.go=" + render.TofuVersion, golden + "=" + render.TofuVersion},
		"github":                      {"internal/render/render.go=" + render.GitHubProviderVersion, golden + "=" + render.GitHubProviderVersion},
```

new:

```go
		"opentofu":                    {"internal/render/render.go=" + render.TofuVersion, awsGolden + "=" + render.TofuVersion, golden + "=" + render.TofuVersion},
		"github":                      {"internal/render/render.go=" + render.GitHubProviderVersion, awsGolden + "=" + render.GitHubProviderVersion, golden + "=" + render.GitHubProviderVersion},
```

Run: `go test ./internal/workflowspec/ -count=1`
Expected: `ok`.

- [ ] **Step 4: Write the failing render tests** — `internal/render/render_test.go` (Edit).

1. Imports, old:

```go
	"path/filepath"
	"regexp"
```

new:

```go
	"path/filepath"
	"reflect"
	"regexp"
```

2. Old:

```go
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

new:

```go
// checkGolden compares a rendered github stack with the golden file of fixture,
// and rewrites that file first when the test runs with -update.
func checkGolden(t *testing.T, fixture string, got []byte) {
	t.Helper()
	golden := filepath.Join("testdata", fixture, "expected", "github", "main.tf.json")
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
		t.Errorf("%s: main.tf.json differs from its golden file:\n%s", fixture, got)
	}
}

// renderedEnvironment is one entry of the environments a component module receives.
type renderedEnvironment struct {
	Protected bool              `json:"protected"`
	Variables map[string]string `json:"variables"`
}

// componentEnvironments decodes the environments that a rendered github stack
// passes to the module component_<name>.
func componentEnvironments(t *testing.T, stack []byte, name string) map[string]renderedEnvironment {
	t.Helper()
	var doc struct {
		Module map[string]struct {
			Environments map[string]renderedEnvironment `json:"environments"`
		} `json:"module"`
	}
	if err := json.Unmarshal(stack, &doc); err != nil {
		t.Fatal(err)
	}
	c, ok := doc.Module["component_"+name]
	if !ok {
		t.Fatalf("no module component_%s in:\n%s", name, stack)
	}
	return c.Environments
}

// TestRenderGolden pins the render of a claims repo without any aws part. Phase 2b
// left this golden file unchanged, byte for byte: a Component without
// aws.registry passes no variables, so existing stacks plan no change.
func TestRenderGolden(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "basic", files["github/main.tf.json"])
}

// TestRenderEnvVariablesGolden renders the aws-registry fixture: basic plus
// aws.registry and the environments' aws blocks. Each environment of the
// Component gets AWS_ROLE_ARN, ECR_REPOSITORY and AWS_REGION (spec §4.4), and
// pro's are spec §5.3's example, checked verbatim here so that -update cannot
// change them unnoticed. Only the github stack is rendered until Phase 2d adds
// the AWS stacks.
func TestRenderEnvVariablesGolden(t *testing.T) {
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
	checkGolden(t, "aws-registry", files["github/main.tf.json"])
	want := map[string]string{
		"AWS_ROLE_ARN":   "arn:aws:iam::000000000003:role/api-pro-ci",
		"ECR_REPOSITORY": "000000000003.dkr.ecr.eu-west-1.amazonaws.com/api",
		"AWS_REGION":     "eu-west-1",
	}
	if got := componentEnvironments(t, files["github/main.tf.json"], "api")["pro"].Variables; !maps.Equal(got, want) {
		t.Errorf("pro variables = %v, want spec §5.3's %v", got, want)
	}
}

// TestRenderWithoutRegistryPassesNoVariables keeps Phase 2a's guard for every
// Component without aws.registry: the environments' aws blocks alone change
// nothing, so the render is basic's golden file, byte for byte.
func TestRenderWithoutRegistryPassesNoVariables(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Components[0].AWS.Registry = false
	files, err := Render(m, Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "basic", files["github/main.tf.json"])
}

// TestRenderEnvVariablesFollowEachEnvironment gives the Component's environments
// different accounts and regions, and the platform an aws environment that the
// Component does not list: each listed environment gets its own values, and the
// other gets nothing.
func TestRenderEnvVariablesFollowEachEnvironment(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Platform.Environments["pro"] = claims.Environment{Protected: true, AWS: &claims.AWS{AccountID: "000000000003", Region: "us-east-1"}}
	m.Platform.Environments["staging"] = claims.Environment{AWS: &claims.AWS{AccountID: "000000000002", Region: "eu-central-1"}}
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]renderedEnvironment{
		"dev": {Protected: false, Variables: map[string]string{
			"AWS_ROLE_ARN":   "arn:aws:iam::000000000001:role/api-dev-ci",
			"ECR_REPOSITORY": "000000000001.dkr.ecr.eu-west-1.amazonaws.com/api",
			"AWS_REGION":     "eu-west-1",
		}},
		"pro": {Protected: true, Variables: map[string]string{
			"AWS_ROLE_ARN":   "arn:aws:iam::000000000003:role/api-pro-ci",
			"ECR_REPOSITORY": "000000000003.dkr.ecr.us-east-1.amazonaws.com/api",
			"AWS_REGION":     "us-east-1",
		}},
	}
	if got := componentEnvironments(t, files["github/main.tf.json"], "api"); !reflect.DeepEqual(got, want) {
		t.Errorf("environments = %+v, want %+v", got, want)
	}
}

// TestRenderRejectsARegistryEnvironmentWithoutAWS covers a model that Load
// rejects (spec §4.7): rendering it must fail, never panic, and never drop the
// variables, which would plan their deletion.
func TestRenderRejectsARegistryEnvironmentWithoutAWS(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Platform.Environments["pro"] = claims.Environment{Protected: true}
	files, err := Render(m, Options{ModuleRef: "v1"})
	want := `render: component "api" has aws.registry, but environment "pro" has no aws block`
	if err == nil || err.Error() != want || files != nil {
		t.Errorf("Render = %d files, error %v; want no files and the error %q", len(files), err, want)
	}
}
```

3. Old:

```go
// TestRenderIsDeterministic is spec §8.2's determinism test at the boundary Render
// consumes: any claim order gives identical bytes. (Load's half, sorted output
// whatever the read order, is TestLoadSortsClaimsByName in internal/claims.)
func TestRenderIsDeterministic(t *testing.T) {
	m := loadFixture(t, "basic")
```

new:

```go
// TestRenderIsDeterministic is spec §8.2's determinism test at the boundary Render
// consumes: any claim order, and any order of a Component's environments, gives
// identical bytes, env variables included. (Load's half, sorted output whatever
// the read order, is TestLoadSortsClaimsByName in internal/claims.)
func TestRenderIsDeterministic(t *testing.T) {
	m := loadFixture(t, "aws-registry")
```

4. Old:

```go
		r.Shuffle(len(shuffled.Components), func(a, b int) {
			shuffled.Components[a], shuffled.Components[b] = shuffled.Components[b], shuffled.Components[a]
		})
		got, err := Render(&shuffled, Options{ModuleRef: "v1"})
```

new:

```go
		r.Shuffle(len(shuffled.Components), func(a, b int) {
			shuffled.Components[a], shuffled.Components[b] = shuffled.Components[b], shuffled.Components[a]
		})
		for i := range shuffled.Components {
			envs := slices.Clone(shuffled.Components[i].Environments)
			r.Shuffle(len(envs), func(a, b int) { envs[a], envs[b] = envs[b], envs[a] })
			shuffled.Components[i].Environments = envs
		}
		got, err := Render(&shuffled, Options{ModuleRef: "v1"})
```

The environments are cloned before shuffling because `shuffled.Components` copies the structs, not their slices.

- [ ] **Step 5: Run them to see them fail**

Run: `go test ./internal/render/ -count=1`
Expected: `FAIL` on exactly three tests:
- `TestRenderEnvVariablesGolden`: `aws-registry: main.tf.json differs from its golden file:` and `pro variables = map[], want spec §5.3's map[AWS_REGION:eu-west-1 AWS_ROLE_ARN:arn:aws:iam::000000000003:role/api-pro-ci ECR_REPOSITORY:000000000003.dkr.ecr.eu-west-1.amazonaws.com/api]`;
- `TestRenderEnvVariablesFollowEachEnvironment`: `environments = map[dev:{Protected:false Variables:map[]} pro:{Protected:true Variables:map[]}], want …`;
- `TestRenderRejectsARegistryEnvironmentWithoutAWS`: `Render = 2 files, error <nil>; want no files and the error "render: component \"api\" has aws.registry, but environment \"pro\" has no aws block"`.

`TestRenderGolden`, `TestRenderWithoutRegistryPassesNoVariables` and `TestRenderIsDeterministic` pass already: they pin today's behavior.

- [ ] **Step 6: Implement** — `internal/render/render.go` (Edit).

1. Imports, old:

```go
	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/version"
```

new:

```go
	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/internal/version"
```

2. Old:

```go
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
```

new:

```go
// Render returns the rendered files, keyed by slash-separated path relative to the
// output root. It fails, rather than render less, on a model that Load rejects in
// a way the render depends on, such as a registry Component in an environment
// without an aws block.
func Render(m *claims.Model, opts Options) (map[string][]byte, error) {
	if opts.ModuleRef == "" && opts.ModulesDir == "" {
		return nil, errors.New("render: set ModuleRef or ModulesDir")
	}
	doc, err := githubStack(m, opts)
	if err != nil {
		return nil, err
	}
	stack, err := marshal(doc)
	if err != nil {
		return nil, err
	}
```

3. Old:

```go
func githubStack(m *claims.Model, opts Options) map[string]any {
```

new:

```go
func githubStack(m *claims.Model, opts Options) (map[string]any, error) {
```

4. Old:

```go
		envs := map[string]any{}
		for _, e := range c.Environments {
			envs[e] = map[string]any{"protected": p.Environments[e].Protected}
		}
```

new:

```go
		envs := map[string]any{}
		for _, e := range c.Environments {
			env := map[string]any{"protected": p.Environments[e].Protected}
			if c.AWS.Registry {
				vars, err := envVariables(c.Name, e, p.Environments[e].AWS)
				if err != nil {
					return nil, err
				}
				env["variables"] = vars
			}
			envs[e] = env
		}
```

5. Old:

```go
	if len(modules) > 0 {
		doc["module"] = modules
	}
	return doc
}
```

new:

```go
	if len(modules) > 0 {
		doc["module"] = modules
	}
	return doc, nil
}

// envVariables are the GitHub environment variables of a registry Component in
// one environment (spec §4.4). internal/naming computes them from the names and
// the environment's aws block, never from AWS state (spec §5.5).
func envVariables(component, env string, aws *claims.AWS) (map[string]string, error) {
	if aws == nil {
		// Load reports this (spec §4.7). Rendering the Component without its
		// variables would plan their deletion, so fail instead.
		return nil, fmt.Errorf("render: component %q has aws.registry, but environment %q has no aws block", component, env)
	}
	return map[string]string{
		"AWS_ROLE_ARN":   literal(naming.RoleARN(aws.AccountID, component, env)),
		"ECR_REPOSITORY": literal(naming.RegistryURL(aws.AccountID, aws.Region, component)),
		"AWS_REGION":     literal(aws.Region),
	}, nil
}
```

- [ ] **Step 7: Run them to see them pass**

Run: `go test ./internal/render/ -v -count=1 -run 'TestRenderGolden|TestRenderEnvVariables|TestRenderWithoutRegistry|TestRenderRejectsARegistry|TestRenderIsDeterministic'`
Expected: `PASS`, with `TestRenderGolden`, `TestRenderEnvVariablesGolden`, `TestRenderWithoutRegistryPassesNoVariables`, `TestRenderEnvVariablesFollowEachEnvironment`, `TestRenderRejectsARegistryEnvironmentWithoutAWS` and `TestRenderIsDeterministic` listed. `git diff --stat internal/render/testdata/basic` prints nothing: `basic`'s golden file is unchanged.

- [ ] **Step 8: Update the docs** (exact replacements with Edit)

1. `docs/claims.md`, the Component's `aws` bullets, old:

```markdown
- **Not rendered yet.** Since Phase 2a the claims accept and validate the `aws` part, but nothing renders from it until Phases 2b to 2d, and the pipelines apply it from Phase 2e.
```

new:

```markdown
- **The variables come first.** Since Phase 2b the github stack carries them, and GitHub creates them when the pipelines apply that stack, after the merge (see "Rendering"). The ECR repository and the role they name are rendered from Phase 2d and applied, to the floci emulator, from Phase 2e.
- Turning `aws.registry` off, or removing one of the Component's environments, deletes those variables; that reconcile waits for approval in `idp-approval`, like every delete.
```

2. `docs/claims.md`, "Rendering", old:

```markdown
A Component's `aws` part renders nothing yet: Phase 2b adds its environment variables to the github stack, and Phase 2d its AWS stacks.
```

new:

```markdown
A Component with `aws.registry: true` passes `variables` to its module for each of its environments: `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION`, computed from the names and the environment's `aws` block (spec §5.3). A Component without it renders exactly as before, so its stack plans no change. Rendering creates nothing: GitHub creates the variables only when the pipelines apply the github stack (reconcile, after the merge), like every other GitHub resource. The AWS stacks, with the ECR repository and the CI role, render from Phase 2d.
```

3. `docs/runbooks/dependencies.md`, "What it updates", old:

```markdown
| OpenTofu | `setup-idp`, `ci.yaml`, `TofuVersion` in `internal/render/render.go` and the golden render | group "opentofu" |
| The GitHub provider | `modules/github/*/versions.tf`, `GitHubProviderVersion` in `render.go` and the golden render | group "github provider", label `needs-lockfile` |
```

new:

```markdown
| OpenTofu | `setup-idp`, `ci.yaml`, `TofuVersion` in `internal/render/render.go` and the golden renders | group "opentofu" |
| The GitHub provider | `modules/github/*/versions.tf`, `GitHubProviderVersion` in `render.go` and the golden renders | group "github provider", label `needs-lockfile` |
```

Then `rg -n "renders nothing yet|Not rendered yet|until Phases 2b to 2d" docs/claims.md` prints nothing, and the naming check prints nothing.

- [ ] **Step 9: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

- [ ] **Step 10: Commit**

```bash
git add internal/render internal/workflowspec/renovate_test.go docs/claims.md docs/runbooks/dependencies.md
git commit -m "feat(render): pass the aws env variables of registry components"
```

- [ ] **Step 11: Mutation check** (prove the per-environment tests bite; nothing is committed)

In `internal/render/render.go`, change `envVariables(c.Name, e, p.Environments[e].AWS)` to `envVariables(c.Name, e, p.Environments[c.Environments[0]].AWS)`. Run `go test ./internal/render/ -count=1`. Expected: `FAIL` on `TestRenderEnvVariablesGolden`, `TestRenderEnvVariablesFollowEachEnvironment`, `TestRenderRejectsARegistryEnvironmentWithoutAWS` and `TestRenderIsDeterministic`. Restore it with `git checkout internal/render/render.go`; `git status --short` prints nothing.

---

### Task 5: CI validates the env-variables render

**Files:**
- Modify: `.github/workflows/ci.yaml`

**Interfaces:**
- Consumes: the `aws-registry` fixture (Task 4) and the module shape (Task 3); `idp render --modules-dir`, which expresses the modules directory relative to `<out>/github` whatever the depth of `--out` (`internal/cli/claims.go`, `relativeModulesDir`).
- Produces: `render-smoke` renders and validates `basic` and `aws-registry`; the job and check name stay `render-smoke` (D-13).

- [ ] **Step 1: Extend `render-smoke`** — `.github/workflows/ci.yaml` (Edit), old:

```yaml
      - name: render the fixture with local module sources
        run: go run ./cmd/idp render --dir internal/render/testdata/basic --out "$RUNNER_TEMP/rendered" --modules-dir modules
      - name: tofu validate the rendered stack (proves the JSON syntax, lock file and modules)
        working-directory: ${{ runner.temp }}/rendered/github
        env:
```

new:

```yaml
      - name: render the fixtures with local module sources
        # basic has no aws part; aws-registry passes each environment's variables.
        run: |
          for fixture in basic aws-registry; do
            go run ./cmd/idp render --dir "internal/render/testdata/$fixture" --out "$RUNNER_TEMP/rendered/$fixture" --modules-dir modules
          done
      - name: tofu validate the rendered stacks (proves the JSON syntax, lock file and modules)
        working-directory: ${{ runner.temp }}/rendered
        env:
```

and old:

```yaml
        run: |
          tofu init -backend=false -input=false
          tofu validate
```

new:

```yaml
        run: |
          for fixture in basic aws-registry; do
            tofu -chdir="$fixture/github" init -backend=false -input=false
            tofu -chdir="$fixture/github" validate
          done
```

The default `run` shell is `bash -e`, so a failure inside the loop fails the step.

- [ ] **Step 2: Verify locally what can be** — `go test ./internal/workflowspec/ -count=1` prints `ok` (it parses `ci.yaml`; no test pins `render-smoke`'s steps). actionlint, zizmor and the job itself run only in CI (no local `go run`), in Task 6 Step 3; its log must show `Success! The configuration is valid.` twice.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yaml
git commit -m "ci: validate the env-variables render against the local modules"
```

---

### Task 6: Ship — verification, PR, review, merge, phase log, README

Controller task (tests are delegated to a subagent).

- [ ] **Step 1: Local verification** — `go test ./... && go vet ./... && gofmt -l .` (gofmt prints nothing); the module test snippet ends with `Success! 12 passed, 0 failed.`; the naming check over the branch's changes prints nothing; `git diff --stat main..HEAD` lists exactly the 14 files of the File Structure (Tasks 1 to 5); `git diff main..HEAD -- lockfiles modules/github/group modules/github/component/versions.tf internal/render/testdata/basic` prints nothing (lock file, provider pins and `basic`'s golden file unchanged).

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feat/phase-2b-env-variables
gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-2b-env-variables --title "feat: phase 2b deterministic env variables" --body "Implements docs/superpowers/plans/2026-11-03-phase-2b-env-variables.md (spec amendment A11): modules/github/component creates one github_actions_environment_variable per environment and name from environments.<env>.variables (spec §5.3); the renderer passes AWS_ROLE_ARN, ECR_REPOSITORY and AWS_REGION from internal/naming for a Component with aws.registry, and a Component without it renders byte for byte as before; render-smoke validates both fixtures. Also closes the Phase 2a review items. No release: 0.2.0 ships in Phase 2f."
```

- [ ] **Step 3: CI** — `gh pr checks <number> --repo jellalshadows-idp/idp-engine --watch`. Expected: `go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `render-smoke` and `release-snapshot` pass. `modules (github/component)` runs the 12 runs on OpenTofu 1.13.1 (D-14), and `render-smoke`'s log shows `Success! The configuration is valid.` twice. A failure goes through the fix loop (fix with a test, scoped review, push).

- [ ] **Step 4: Final whole-branch review** (most capable model) with the package `main..HEAD`, the spec (§3.3, §4.4, §5.3, §5.5, §8.3, §11), ADR-0006, ADR-0013 and this plan; one fix wave; one scoped re-review; rulings in the ledger. CI green again after the wave.

- [ ] **Step 5: Merge** — confirm the PR's latest CI is green, then `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`.

- [ ] **Step 6: The release PR and the harness** (watch list) — the merge's push runs `release.yaml`. Read the release PR with `gh pr list --repo jellalshadows-idp/idp-engine --head release-please--branches--main --json number,title --jq '.[] | "\(.number) \(.title)"'`. Expected: one line `28 chore(main): release 0.2.0`. Record it and leave the PR open (D-16). Record the harness run that `e2e-release-pr` dispatched (`gh run list --repo jellalshadows-idp/idp-engine --workflow e2e.yaml --limit 1 --json databaseId,status,conclusion`) and, when it ends, its conclusion. Expected: `success`, which includes the pin PR's and the case-only step's `No infrastructure changes.` (D-5); a failure is a regression to fix before 2c.

- [ ] **Step 7: Write `docs/phases/phase-2b.md`** in the style of `docs/phases/phase-2a.md`. Include:
  - the header list: period, plan, spec amended by A11, ADR-0013 amended, previous phase (the Phase 2a log), and status with the PR and its merge commit;
  - the summary, and the tasks with their commits and reviews;
  - every ruling (D-1 to D-17 and the execution rulings) with what, why and cost if wrong, including D-12's correction: the pipelines already apply the variables with the github stack;
  - the Review Focus table with each result, and the watch list with each result (the release PR's number and version, the harness run);
  - notable findings caught by review, and the final disposition of every finding. That disposition includes the five carried items, all **Fixed**: Minor 1 in Task 2, Minor 2 in Task 1 (with `aud` added to §8.3), Minor 3 in Task 2 (documented, D-8), Minor 4 in Task 1, and R3-001 in Task 4.

  Then "Carried into later sub-phases":
  - **2c:** the AWS modules create exactly naming's identities and their tests assert the trust policy's `sub` and `aud` (§8.3 under A11); the baseline's lifecycle and the shared-account question (2a's D-11); the AWS lock file and the provider pin (`hashicorp/aws` 6.66.0 was proven on OpenTofu 1.12.6, and the repo pins 1.13.1); the floci ECR host form (ADR-0013).
  - **2d:** rendering the AWS stacks; the nested-stack discovery items of `docs/phases/phase-1b.md`.
  - **2e:** the AWS reconcile design.
  - **2f:** the harness's token to read environment variables (the bot token is scoped to the claims repo and has no environments permission), the `aws` block for `dev` in `idp-claims-e2e`'s `config/platform.yaml`, and an `aws.registry` harness claim whose variables the E2E checks.

  Then "Owner notes": release PR #28 stays open until 2f, and every push to `main` re-runs the harness on it; the earlier owner items stay as recorded. Close with the process notes (inline or delegated execution, TDD evidence, the three mutation checks, local OpenTofu 1.12.6 against CI's 1.13.1) and the status.

- [ ] **Step 8: Update `README.md`** (exact replacements with Edit)

1. Old:

```markdown
> **Status:** Phase 2a is complete: claims accept a Component's AWS part (`aws.registry`) and each environment's AWS account, validate them, and `internal/naming` computes the AWS identities; nothing AWS is rendered or applied yet. See the [phase 2a log](docs/phases/phase-2a.md) and [docs/claims.md](docs/claims.md). Phase 2b (the deterministic env variables) is next.
```

new:

```markdown
> **Status:** Phase 2b is complete: a Component with `aws.registry` renders its environment variables (`AWS_ROLE_ARN`, `ECR_REPOSITORY`, `AWS_REGION`), computed by `internal/naming`, and `modules/github/component` creates them; the AWS resources they name are not rendered yet. See the [phase 2b log](docs/phases/phase-2b.md) and [docs/claims.md](docs/claims.md). Phase 2c (the AWS modules and their floci tests) is next.
```

2. Old:

```markdown
- The pipelines reconcile **GitHub only** (teams, repositories, rulesets, environments). Claims accept a Component's AWS part since Phase 2a, but it is rendered from Phase 2b and applied from Phase 2e.
```

new:

```markdown
- The pipelines reconcile **GitHub only** (teams, repositories, rulesets, environments and, since Phase 2b, the environment variables of a Component with `aws.registry`). The ECR repository and the CI role those variables name are rendered from Phase 2d and applied from Phase 2e.
```

- [ ] **Step 9: Commit doc-only to `main`** (side effect 2) — the naming check prints nothing; then:

```bash
git add docs/phases/phase-2b.md README.md
git commit -m "docs: add the phase 2b execution log"
git push
```

The push runs `release.yaml`, which refreshes the open release PR and re-runs the harness on it. Leave the PR open.

- [ ] **Step 10: Delete the SDD workspace** (side effect 3) — after checking that the phase log reproduces the ledger: `rm -rf .superpowers/sdd`, then `git status --short` prints nothing.
