# Phase 1c — Reusable Pipelines Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the reusable `pr`, `reconcile`, `drift` and `recover` workflows (spec §6), the claims repo caller templates and their contract tests, then prove them with a live smoke run in `idp-claims-e2e` that creates, updates, drifts, partially fails, self-heals and deletes an `e2e-` repository and team.

**Architecture:**
- **One version per run.** Every reusable job checks out the engine at `job.workflow_sha` and builds `idp` from it with the local composite action `setup-idp`, so the workflow, the CLI and the rendered module refs are one commit (ADR-0021).
- **Thin callers.** A claims repo carries four caller workflows copied from `examples/claims-repo/.github/workflows/`. The PR caller adds the `idp-gate` job that the `main` ruleset requires; it mirrors the reusable workflow's `verdict` job (ADR-0022).
- **Least privilege per step.** Reader token for plans and drift, a writer token for the apply, and a second writer token that can only write the claims repo for the wet commit; bot-authored writes use `GITHUB_TOKEN` (ADR-0023).
- **Fail-safe wet commits.** The apply job saves the exact tree it commits as an artifact before pushing; a failed push is recovered by the `recover` workflow, and a reconcile is never re-run (ADR-0024).
- **Contract tests.** A test-only Go package, `internal/workflowspec`, reads the YAML and pins the rules the spec and the ADRs set.

**Tech Stack:** GitHub Actions (reusable workflows, composite action, environments), Go 1.26 tests with `go.yaml.in/yaml/v3` (already a dependency), OpenTofu 1.12.6, actionlint v1.7.12, zizmor 1.30.1, `gh`.

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §6 (all of it), §7.3–§7.5, §8.5 and §10. Context: `docs/cli.md` (every command the workflows call), `docs/phases/phase-1b.md` ("Carried into Phase 1c and 1d", "Owner notes"), ADR-0016 to ADR-0020.

## Scope

Phase 1 is delivered as four plans (spec §10). This is **1c**: the reusable workflows and a live smoke run. It changes no CLI behaviour; every command it calls shipped in 1b. Not in scope, and why:

| Item | Where it goes |
|---|---|
| E2E harness (scripted, nightly), release-please, GoReleaser, attestations, Renovate, `v0.1.0` | Phase 1d |
| Onboarding the production `idp-claims` repo (its callers should pin a release, not a branch commit) | Phase 1d. 1c only re-runs its bootstrap so `IDP_BOOTSTRAP` exists |
| GraphQL editor check on the bot comment (1b finding I1, option B) | Phase 1d (ruling P-4) |
| 1b minors M1 (forget column), M4 (wet marker for `--wet`), M7 (`wetpush` `PathEscape`) | Phase 1d (ruling P-5) |
| AWS stacks, `fetch`/`adopt` | Phases 2 and 4. The workflows fail when any stack other than `github` is affected |

## Decisions taken while planning

The executor copies these into the ledger as rulings before Task 1.

- **P-1. A `recover` workflow.** Spec §6.2 only says a failed wet push uploads the state, opens an issue and fails. 1c adds the button that uses that artifact. ADR-0024.
- **P-2. Templates pin a commit.** Until releases exist, callers use `@<40-hex commit> # unreleased`. The templates pin the newest commit that holds the final reusable workflows; Phase 1d replaces this with release tags.
- **P-3. Build `idp` from source** at `job.workflow_sha` (ADR-0021). Release binaries with attestation are Phase 1d's call.
- **P-4. I1-B deferred to Phase 1d.** Anyone who can edit the bot's comment has write access and, while `idp-main` requires no approving review, can already merge. ADR-0018's follow-up line says so.
- **P-5. M1, M4 and M7 deferred to Phase 1d.** The workflows never produce `forget` actions, pass a constant `--wet` path that a contract test pins, and push only to the branch `wet`.
- **P-6. Production onboarding deferred** (see Scope).
- **P-7. Drift does not fail on drift.** The run fails only when the check cannot run; drift is reported by the issue and a warning annotation (spec amendment A6).
- **P-8. The final whole-branch review runs before the smoke**, so the smoke exercises reviewed code. Fixes the smoke forces get a scoped review each.
- **P-9. A reconcile is never re-run.** `plan` and `apply-github` refuse `GITHUB_RUN_ATTEMPT != 1`, and `apply-github` refuses when `wet` moved since its plan. A failed reconcile is retried with a new dispatch (ADR-0024).
- **P-10. Artifact action pins.** `actions/upload-artifact` v7.0.2 and `actions/download-artifact` v8.0.2 were published on 2026-10-07, under the 14-day floor, so v7.0.1 (2026-04-10) and v8.0.1 (2026-03-11) are pinned.
- **P-11. Reusable secrets are declared optional.** A fork PR gets no secrets and must still reach `verdict`; each job that needs a secret checks it first and names what is missing.

## Global Constraints

**Pinned actions** (every one at least 14 days old; SHAs verified against the tags):

| Action | Version | SHA |
|---|---|---|
| `actions/checkout` | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` |
| `actions/setup-go` | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |
| `opentofu/setup-opentofu` | v2.0.2 | `a1320f892987e89d278cc92dc5adc984fb93aca4` |
| `actions/create-github-app-token` | v3.2.0 | `bcd2ba49218906704ab6c1aa796996da409d3eb1` |
| `actions/upload-artifact` | v7.0.1 | `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` |
| `actions/download-artifact` | v8.0.1 | `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c` |

**Tools**
- OpenTofu `1.12.6` (the same version `ci.yaml` tests the modules with).
- actionlint `v1.7.12`: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`.
- zizmor `1.30.1`: in CI `pipx run zizmor==1.30.1`; locally the binary the controller installs in pre-flight, always with `--offline`.
- Runners: `ubuntu-24.04`.

**Names** (copy verbatim)
- Reusable workflows: `.github/workflows/{pr,reconcile,drift,recover}.yaml`, named `idp-pr`, `idp-reconcile`, `idp-drift`, `idp-recover`.
- Jobs: `pr`: `validate`, `plan`, `comment`, `verdict`. `reconcile`: `plan`, `approve`, `apply-github`. `drift`: `drift`. `recover`: `recover`. Caller job ids: `pr` (plus `idp-gate`), `reconcile`, `drift`, `recover`.
- Composite action: `.github/actions/setup-idp/action.yaml`, inputs `engine-dir` and `version`.
- Checkout paths: engine `.idp/engine`, wet `.idp/wet`. Scratch: `$RUNNER_TEMP/idp/{work,out,in,plans,wet-root}`.
- Artifacts: `idp-pr-comment` (1 day), `idp-reconcile` (1 day), `idp-wet-root` (7 days).
- Issues: label `drift` / title `Drift detected`; label `idp-wet-push` / title `Wet push failed`.
- Environments: `idp-approval`, `idp-write`. Secrets: `IDP_READER_PRIVATE_KEY`, `IDP_STATE_PASSPHRASE` (repo), `IDP_WRITER_PRIVATE_KEY` (environment `idp-write`). Variables: `IDP_READER_CLIENT_ID`, `IDP_BOOTSTRAP` (repo), `IDP_WRITER_CLIENT_ID` (environment `idp-write`).
- Concurrency: `idp-plan-${{ github.event.pull_request.number }}` with `cancel-in-progress: true`; `idp-wet` with `cancel-in-progress: false` and `queue: max`.
- Drift schedule: `cron: '23 5 * * *'`.

**Workflow rules** (the contract tests enforce each one)
- Top-level `permissions: {}`; every job lists its scopes.
- Every remote `uses:` is pinned to a full SHA with a `# vX.Y.Z` comment (`# unreleased` for engine workflow refs).
- Every `actions/checkout` sets `persist-credentials: false`.
- No `${{ }}` inside `run:`; values reach scripts through `env:`.
- Tokens are set on the one step that uses them. `idp comment` and `idp issue` run with `GITHUB_TOKEN: ${{ github.token }}` and never with `GH_TOKEN`.
- The writer key appears only in jobs with `environment: idp-write`; the `idp-approval` job has no secrets and no actions.
- `tofu init` uses `-lockfile=readonly`; plans are written under `$RUNNER_TEMP/idp/`; `tofu apply` only applies a saved plan.
- The wet branch is synced from a clean copy of the render artifact, never from a directory `tofu init` touched.

**Process**
- Branch `feat/phase-1c-pipelines`; one PR to `main`, merged with a merge commit (branch commits stay reachable, so commit pins stay valid).
- Conventional commits, never with Co-Authored-By or any AI attribution.
- No `go build` locally. Use `go test ./...`, `go vet ./...` and `gofmt -l .` (must print nothing); `-race` runs in CI. CI and the workflows themselves build `idp`.
- Shell rule: never use `cat`, `grep`, `find`, `sed` or `ls`, and never write files with shell heredocs. Use the Read/Write/Edit tools, `rg`, `gh` and `python -I`. (Scripts that run on Actions runners are product code and may use any tool.)
- Never delete or overwrite `~/.idp/e2e.pass`, `~/.idp/main.pass` or `~/.idp/apps/*`. They are read, never written.
- Docs are in English.
- Owner rule: docs describe the platform on its own terms. They never name another product as inspiration or comparison. The controller gives each dispatch the check that enforces this.

**Side effects this plan authorizes** (outside the local repo; nothing else is done without asking)
1. Push `feat/phase-1c-pipelines` to `jellalshadows-idp/idp-engine`, open its PR, and merge it (merge commit, branch deleted) after review and the smoke.
2. Run `idp bootstrap apply` for `idp-claims-e2e` and `idp-claims` with the owner's `gh` token, which writes the `IDP_BOOTSTRAP` variable (and nothing else on an unchanged org).
3. In `idp-claims-e2e`: push branches, open and merge (merge commit) the smoke PRs of Task 10 (nine, plus one per fix-loop re-pin), dispatch its `drift`, `reconcile` and `recover` workflows, and approve two `idp-approval` deployments through the API as `jellalshadows`.
4. Through the platform itself: create, update and delete the repository `jellalshadows-idp/e2e-smoke-app` and the team `e2e-smoke`; one manual description edit on `e2e-smoke-app` to cause drift.
5. Issues labelled `drift` and `idp-wet-push` in `idp-claims-e2e`, opened and closed by `github-actions[bot]`.
6. Doc-only commits straight to `idp-engine` `main` after the merge (phase log, README status).

Deleting anything by hand (a repository or team left behind by a broken run) is **not** authorized: stop and ask the owner. It also needs the `delete_repo` scope, which the local token lacks.

## Review Focus

1. **A cancelled or skipped run must never pass `idp-gate`.**
   - **Input:** a newer push cancels a PR run, or the reusable workflow is skipped.
   - **Expected:** `idp-gate` reports failure, never "skipped" (a skipped required check counts as passing).
   - **Pinned by:** `TestPRTemplateGateMirrorsTheResult` (Task 7) and `TestPRVerdictFailsForksAndFailures` (Task 4).
2. **A pull request that touches no claim file must still report `idp-gate`.**
   - **Input:** a README-only PR.
   - **Expected:** the PR pipeline runs, posts a comment that says `No infrastructure changes.`, and passes. A path filter would leave the required check pending forever.
   - **Pinned by:** `TestPRTemplate` (Task 7) and `TestPRPlanSummaryCoversTheAffectedStacksAtTheHeadCommit` (Task 4); exercised live by smoke step S8.
3. **An apply that fails halfway.**
   - **Input:** some resources apply, one fails.
   - **Expected:** the state is still committed to `wet`; the render is not, so the next reconcile retries the stack (spec §6.2, "the system self-heals").
   - **Pinned by:** `TestApplyCommitsTheRenderOnlyAfterASuccessfulApply` (Task 5); exercised live by smoke steps S7 and S8.
4. **A re-run, or a `wet` that moved since the plan.**
   - **Input:** "Re-run failed jobs" on a reconcile, or a second writer to `wet` between plan and apply.
   - **Expected:** the apply refuses; it never applies a saved plan onto an older state.
   - **Pinned by:** `TestReconcileIsNeverReRun` and `TestApplyRefusesWhenWetMoved` (Task 5).
5. **A recovery pointed at the wrong run, or after `wet` moved.**
   - **Input:** `recover` dispatched with the id of a PR run, a successful run, or a failed run whose push was followed by newer wet commits.
   - **Expected:** it refuses and changes nothing; it never overwrites a newer state.
   - **Pinned by:** `TestRecoverChecksTheRunAndTheWetBase` (Task 6); exercised live by smoke step S9.

## File Structure

```
.github/actions/setup-idp/action.yaml        # Task 3: Go + OpenTofu + build idp at a commit
.github/actionlint.yaml                      # Task 4: the one narrow actionlint ignore
.github/workflows/pr.yaml                    # Task 4
.github/workflows/reconcile.yaml             # Task 5
.github/workflows/drift.yaml                 # Task 6
.github/workflows/recover.yaml               # Task 6
.github/workflows/ci.yaml                    # (modify) Tasks 3 and 7: lint the new files
examples/claims-repo/.github/workflows/{pr,reconcile,drift,recover}.yaml   # Task 7
internal/workflowspec/doc.go                 # Task 2
internal/workflowspec/load_test.go           # Task 2: YAML model and helpers
internal/workflowspec/rules_test.go          # Task 2: pure rule functions and their tables
internal/workflowspec/common_test.go         # Task 2: rules over every workflow and action
internal/workflowspec/setupidp_test.go       # Task 3
internal/workflowspec/pr_test.go             # Task 4
internal/workflowspec/reconcile_test.go      # Task 5
internal/workflowspec/drift_test.go          # Task 6
internal/workflowspec/recover_test.go        # Task 6
internal/workflowspec/templates_test.go      # Task 7
internal/actions/actions.go                  # (modify) Task 2: misplaced doc comment from 1b
docs/adr/0021…0024-*.md, docs/adr/README.md, docs/adr/0018-*.md   # Task 1
docs/superpowers/specs/2026-10-08-idp-on-actions-design.md         # Task 1: amendment A6
docs/pipelines.md, docs/runbooks/pipelines.md, README.md           # Task 8
docs/phases/phase-1c.md                      # Task 11
```

## Pre-flight (controller, before Task 1)

- [ ] **Confirm the GitHub account.** Run `gh api user --jq .login`. Expected: `jellalshadows`. If not, run `gh auth switch -u jellalshadows`.
- [ ] **Install zizmor locally** (the CI version, in the session scratchpad; `$SCRATCH` is the scratchpad path):

```bash
python -m venv "$SCRATCH/zizmor-venv"
"$SCRATCH/zizmor-venv/Scripts/python" -m pip install --disable-pip-version-check zizmor==1.30.1
"$SCRATCH/zizmor-venv/Scripts/zizmor" --version
```

Expected: `zizmor 1.30.1`. Every dispatch that touches YAML gets this path as `ZIZMOR`.

- [ ] **Ledger the rulings P-1 to P-11** from "Decisions taken while planning".

---

### Task 1: Decisions first — ADRs 0021–0024, spec amendment A6, ADR-0018 follow-up

**Files:**
- Create: `docs/adr/0021-reusable-workflows-build-idp-from-their-commit.md`, `docs/adr/0022-required-check-lives-in-the-caller.md`, `docs/adr/0023-pipeline-tokens-per-step.md`, `docs/adr/0024-failed-wet-push-recovery.md`
- Modify: `docs/adr/README.md`, `docs/adr/0018-plan-fingerprint-and-gate-trust.md`, `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`

**Interfaces:**
- Produces: the decisions Tasks 3–8 implement and cite by number.

- [ ] **Step 1: Create the branch**

Run: `git switch main && git pull --ff-only && git switch -c feat/phase-1c-pipelines`

- [ ] **Step 2: Write `docs/adr/0021-reusable-workflows-build-idp-from-their-commit.md`**

````markdown
# 0021. The reusable workflows build idp from their own commit

- Status: Accepted
- Date: 2026-10-11
- Spec: [§6](../superpowers/specs/2026-10-08-idp-on-actions-design.md#6-pipelines), [§7.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#75-supply-chain)

## Context

A claims repo calls the engine's reusable workflows by commit. Each job needs the `idp` CLI at the same version as the workflow file, and the render pins module sources to the CLI's own version (spec §5.3). No release exists yet, so there is no binary to download and verify (spec §7.5).

Inside a called workflow, `github.workflow_sha` is the **caller's** commit. The job context has `job.workflow_repository` and `job.workflow_sha`, which name the reusable workflow's own repository and commit.

## Decision

Every reusable job that runs `idp`:

1. checks out `${{ job.workflow_repository }}` at `${{ job.workflow_sha }}` into `.idp/engine`, without persisted credentials;
2. runs the local composite action `.idp/engine/.github/actions/setup-idp`, which installs Go from the engine's `go.mod` without a cache, installs OpenTofu 1.12.6, and builds `idp` with `version.Version` set to that commit;
3. renders with that default module ref, so the workflow, the CLI and the modules are one commit.

Callers pin the reusable workflows to a full commit SHA. The templates mark the pin `# unreleased` until releases exist.

## Consequences

### Positive

- One version for the workflow, the CLI and the modules, with no release machinery yet.
- No binary travels between jobs or runs, so there is no artifact to trust.

### Negative / costs

- Each job spends a Go build. There is no module or build cache, on purpose: a cache restored into the job that holds the writer key is a poisoning path.
- actionlint v1.7.12 does not know `job.workflow_repository` and `job.workflow_sha`. `.github/actionlint.yaml` ignores exactly that message, only for `.github/workflows/`.
- zizmor's `self-repository` audit flags `uses: ./…` in a reusable workflow, because `./` resolves in the caller's workspace. That is why the engine is checked out there first; each such line carries an inline ignore that says so.
- Module sources pin a commit, not a tag, until releases. Phase 1d decides how releases change this (spec §7.5).

## Alternatives considered

- **`github.workflow_sha`.** Rejected: in a called workflow it is the caller's commit.
- **Download a released binary and verify its attestation.** Not possible before the first release; Phase 1d revisits it.
- **Build once and pass the binary between jobs as an artifact.** Rejected: it saves under a minute per job and adds a trust hop.
- **The `$/` prefix for local actions.** Rejected for now: it needs a recent runner (2.336.0 or later), and how it resolves inside a called workflow is not settled. The explicit checkout is plain and testable.
````

- [ ] **Step 3: Write `docs/adr/0022-required-check-lives-in-the-caller.md`**

````markdown
# 0022. The required check lives in the caller

- Status: Accepted
- Date: 2026-10-11
- Spec: [§6.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#61-pr-pull_request--main), [§7.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#74-rulesets)

## Context

The `main` ruleset of a claims repo requires one status check, `idp-gate` (spec §6.1, §7.4). A job inside a reusable workflow reports a check named `<caller job> / <job>`, so no engine job can report exactly `idp-gate`. A required check that is **skipped** counts as passing, and pull requests from forks skip the plan on purpose.

## Decision

- The reusable PR workflow ends with a `verdict` job that always runs. It fails pull requests from forks with "a maintainer must push this branch to the repo", and it fails unless `validate`, `plan` and `comment` all succeeded.
- The caller template has a job named `idp-gate` with `needs: pr`, `if: always()` and no permissions. It passes only when the reusable workflow's result is `success`.
- `if: always()`, never `!cancelled()`, keeps `idp-gate` from ever being skipped: a cancelled run reports a failing check, not a skipped one.
- The PR template has no path filters, so every pull request reports `idp-gate`. A filter would leave the required check pending forever on a PR that touches no claims.

## Consequences

### Positive

- The check name is stable, and the logic behind it is versioned with the engine.

### Negative / costs

- The mirror job lives in a workflow file of the claims repo, so a pull request can change it. This is the trust boundary ADR-0018 already accepts: members who can push branches can run workflows as `github-actions[bot]`, and while `required_approving_review_count` is 0 they can merge. The owner decides whether to require a review on `.github/` (CODEOWNERS plus one approval) and to pin `idp-gate` to the GitHub Actions integration in the ruleset (phase 1b owner note).

## Alternatives considered

- **A job named `idp-gate` inside the reusable workflow.** Rejected: its check would be `pr / idp-gate`.
- **Every reusable job as a required check.** Rejected: the names depend on the caller's job id, and skipped jobs pass.
- **Ruleset-enforced workflows that run a workflow from another repository.** They would take the check out of the PR's reach. Not evaluated in this phase; recorded for Phase 5.
````

- [ ] **Step 4: Write `docs/adr/0023-pipeline-tokens-per-step.md`**

````markdown
# 0023. Pipeline tokens are minted per step with the narrowest scope that works

- Status: Accepted
- Date: 2026-10-11
- Spec: [§7.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#73-two-github-apps-and-three-secret-scopes)

## Context

The pipelines use the reader App, the writer App and the workflow's `GITHUB_TOKEN` (spec §7.3). `actions/create-github-app-token` mints a token for the whole installation unless `repositories` narrows it, with all of the App's permissions unless `permission-*` inputs narrow them. zizmor's `github-app` audit flags both defaults.

## Decision

| Token | Used by | Repositories | Permissions |
|---|---|---|---|
| Reader | PR plan, reconcile plan, drift (plan and `bootstrap check`) | the whole installation | all of the reader App's, which are read-only by design |
| Writer, apply | `tofu apply` in `apply-github` | the whole installation | exactly the writer manifest's: administration, contents, environments, members and workflows, all write |
| Writer, wet commit | `idp wet-push` in `apply-github` and `recover` | the claims repo only | `contents: write` |
| `GITHUB_TOKEN` | PR comment, issues, gate reads, artifact download | the claims repo | per job, as listed in [docs/pipelines.md](../pipelines.md) |

- Every token is set on the one step that uses it, never at job or workflow level.
- `idp comment` and `idp issue` run with `GITHUB_TOKEN` and never with `GH_TOKEN` set, so the bot authors what the gate trusts (ADR-0018).
- The apply token is minted right before the apply, and only when the plan has changes.

## Consequences

### Positive

- The token that commits to `wet` cannot touch any other repository.
- A contract test pins the apply token's permissions to `bootstrap/apps/writer.json`, so the two cannot drift apart.

### Negative / costs

- The reader token is not narrowed. The drift check reads repository variables, which need the App's `actions_variables` permission, and `create-github-app-token` v3.2.0 declares no input for it; a narrowed token would break the check. The App itself is read-only. The step carries a zizmor inline ignore that says so.
- The apply token spans every repository, because the claims may declare any repository of the org. Its step carries a zizmor inline ignore.

## Alternatives considered

- **One writer token for the apply and the commit.** Rejected: the commit needs one repository and one permission.
- **The provider's `app_auth` with the key file.** Rejected in ADR-0013: minted tokens fit the measured apply times.
````

- [ ] **Step 5: Write `docs/adr/0024-failed-wet-push-recovery.md`**

````markdown
# 0024. A failed wet push is recovered from the run's own artifact

- Status: Accepted
- Date: 2026-10-11
- Spec: [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main), [§7.7](../superpowers/specs/2026-10-08-idp-on-actions-design.md#77-runbooks)

## Context

After an apply, the runner holds the only copy of the new state until `idp wet-push` commits it (spec §6.2). If the push fails (an API error, an expired token, a lost runner), the next reconcile would plan against the old state in `wet` while the real resources already changed. Re-running the apply job has the same problem: it would apply its saved plan onto the state that plan read, even when `wet` moved since.

## Decision

- Before every push, `apply-github` writes the exact tree it commits (`tfstate/github.tfstate`, plus `rendered/github` when the apply succeeded or had nothing to apply) and a `wet-base` file with the `wet` commit its plan read. It uploads that tree as the artifact `idp-wet-root`, kept 7 days, hidden files included.
- If the push fails, the job opens one issue labelled `idp-wet-push`, titled "Wet push failed", that links the runbook, and the run fails.
- The `recover` workflow (a dispatch with the run id, in environment `idp-write`) checks that the run is a failed reconcile of `main`, downloads its `idp-wet-root`, refuses when `wet` no longer points at `wet-base`, then commits the tree with the wet-commit token (ADR-0023) and closes the issue.
- A reconcile run is never re-run: `plan` and `apply-github` refuse a second attempt, and `apply-github` refuses when `wet` moved since its plan. A failed reconcile is retried with a new dispatch.

## Consequences

### Positive

- Recovering is one dispatch: no local tools, no passphrase, no key.
- Neither the recovery nor a re-run can overwrite a newer state.

### Negative / costs

- When `wet` moved, or after 7 days, the recovery refuses, and the state must be rebuilt with `import` blocks (spec §7.7).
- Artifacts of a public repo are public. The state in them is encrypted, like the state in `wet` (ADR-0002).
- A whole-run re-run after a transient plan failure is refused too; the runbook says to dispatch instead.

## Alternatives considered

- **Retry the push inside the job.** Rejected as the only answer: it does not cover an expired token or a lost runner, so the recovery path is needed anyway.
- **Recover by hand with the passphrase.** Kept only as the fallback when `wet` moved.
````

- [ ] **Step 6: Add the four rows to `docs/adr/README.md`**

Append after the `0020` row:

```markdown
| 0021 | [The reusable workflows build idp from their own commit](0021-reusable-workflows-build-idp-from-their-commit.md) | Accepted |
| 0022 | [The required check lives in the caller](0022-required-check-lives-in-the-caller.md) | Accepted |
| 0023 | [Pipeline tokens are minted per step with the narrowest scope that works](0023-pipeline-tokens-per-step.md) | Accepted |
| 0024 | [A failed wet push is recovered from the run's own artifact](0024-failed-wet-push-recovery.md) | Accepted |
```

- [ ] **Step 7: Update the follow-up of `docs/adr/0018-plan-fingerprint-and-gate-trust.md`**

Replace the line

```markdown
- Phase 1c decides whether to reject comments edited by anyone other than the Actions bot (GraphQL `editor` / `lastEditedAt`).
```

with

```markdown
- Deferred to Phase 1d by the Phase 1c plan: rejecting comments edited by anyone other than the Actions bot (GraphQL `editor` / `lastEditedAt`). Anyone who can edit the bot's comment has write access and, while `idp-main` requires no approving review, can already merge; the check pays off once reviews are required.
```

- [ ] **Step 8: Apply amendment A6 to the spec**

Six exact replacements in `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` (use Edit; the old text is quoted exactly):

1. §6 intro. Old:

```markdown
`idp-claims` contains three thin workflows. Each one is
`uses: <org>/idp-engine/.github/workflows/<x>.yaml@vX.Y.Z`, and Renovate bumps the
ref. Each workflow sets `permissions: {}`, and each job asks for what it needs.
```

New:

```markdown
`idp-claims` contains four thin workflows, copied from the engine's
`examples/claims-repo/.github/workflows/`: `pr`, `reconcile`, `drift`, and `recover`,
the recovery button for a failed wet push (ADR-0024). Each one is
`uses: <org>/idp-engine/.github/workflows/<x>.yaml@<commit SHA> # <version>`, and
Renovate bumps the ref and the comment. Each workflow sets `permissions: {}`, and each
job asks for what it needs. Every reusable job builds `idp` from the reusable
workflow's own commit, so the workflow, the CLI and the module refs are one version
(ADR-0021). Until Phase 2 the only stack is `github`, and until Phase 4 there are no
features to fetch or adopt: the workflows fail if any other stack is affected.
*(Amendment A6, 2026-10-11.)*
```

2. §6.1 step 7. Old:

```markdown
7. **`idp-gate`** is the only required status check, because matrix job names vary.
```

New:

```markdown
7. **`idp-gate`** is the only required status check, because matrix job names vary.
   It is a job of the caller workflow that always runs and passes only when the
   reusable workflow succeeded; that workflow's last job, `verdict`, fails forks and
   any failed job (ADR-0022). *(Amendment A6, 2026-10-11.)*
```

3. §6.2 step 4.4. Old:

```markdown
   4. If the push fails, it uploads the encrypted state as an artifact, opens an
      issue that links the runbook, and fails.
```

New:

```markdown
   4. Before the push it uploads the exact tree it commits, with the `wet` commit the
      plan read, as the artifact `idp-wet-root` (7 days). If the push fails, it opens
      an issue that links the runbook, and fails; the `recover` workflow commits that
      tree while `wet` has not moved (ADR-0024). A reconcile run is never re-run: a
      failed run is retried with a new dispatch. *(Amendment A6, 2026-10-11.)*
```

4. §6.4, last bullet. Old:

```markdown
- Scheduled workflows in public repos are disabled after 60 days without activity.
  Renovate PRs keep the repo active, and this is documented.
```

New:

```markdown
- Scheduled workflows in public repos are disabled after 60 days without activity.
  Renovate PRs keep the repo active, and this is documented.
- It also runs on `workflow_dispatch`. A run fails only when the check cannot run;
  drift itself is reported by the issue and a warning annotation.
  *(Amendment A6, 2026-10-11.)*
```

5. §7.3. Old:

```markdown
- Tokens are minted per job with `actions/create-github-app-token` and expire after
  1 hour. A longer apply is a known limitation, and Phase 0 measures how long
  applies take.
```

New:

```markdown
- Tokens are minted per step with `actions/create-github-app-token`, with the
  narrowest scope that works (ADR-0023), and expire after 1 hour. A longer apply is a
  known limitation, and Phase 0 measures how long applies take.
  *(Amendment A6, 2026-10-11.)*
```

6. §7.5. Old:

```markdown
- The CLI is released with checksums and a provenance attestation
  (`actions/attest-build-provenance`). The workflows run `gh attestation verify`
  before they execute it.
```

New:

```markdown
- The CLI is released with checksums and a provenance attestation
  (`actions/attest-build-provenance`). The workflows run `gh attestation verify`
  before they execute it. Until the first release, the workflows build `idp` from
  source at the pinned commit instead (ADR-0021); Phase 1d decides how releases
  change that. *(Amendment A6, 2026-10-11.)*
```

- [ ] **Step 9: Check the links**

Run: `rg -n "0021-|0022-|0023-|0024-" docs/adr/README.md docs/adr/002*.md`
Expected: every linked file name matches a file created in Steps 2–5.

- [ ] **Step 10: Commit**

```bash
git add docs/adr docs/superpowers/specs/2026-10-08-idp-on-actions-design.md
git commit -m "docs: record the pipeline decisions (adr 0021-0024, spec amendment a6)"
```

---

### Task 2: The workflow contract harness and the common rules

**Files:**
- Create: `internal/workflowspec/doc.go`, `internal/workflowspec/load_test.go`, `internal/workflowspec/rules_test.go`, `internal/workflowspec/common_test.go`
- Modify: `internal/actions/actions.go:39-43` (1b leftover: `Mask` sits between `SetOutput`'s doc comment and `func SetOutput`)

**Interfaces:**
- Produces (package `workflowspec`, test files; later tasks use these exact names):
  - constants `repoRoot`, `setupIDPPath`, `setupIDP`, `engineDir`, `workflowRepo`, `workflowSHA`;
  - types `strmap`, `strlist`, `perms{present bool; scalar string; levels map[string]string}`, `workflow{path; root; On yaml.Node; Env strmap; Permissions perms; Concurrency *concurrency; Jobs map[string]*job}`, `concurrency{Group string; CancelInProgress bool; Queue string}`, `job{Name; Needs strlist; If; Environment; TimeoutMinutes int; Env strmap; Permissions perms; Outputs strmap; Uses; With strmap; Secrets yaml.Node; Steps []step}`, `step{ID; Name; If; Uses; Run; Shell; WorkingDirectory; With strmap; Env strmap}`, `action{path; root; Inputs map[string]struct{Required bool}; Runs struct{Using string; Steps []step}}`, `callSpec{Inputs map[string]struct{Required bool; Type string}; Secrets map[string]struct{Required bool}}`, `located{where string; step step}`, `usesRef{value, comment string; line int}`;
  - functions `loadWorkflow(t, rel) *workflow`, `loadAction(t, rel) *action`, `glob(t, pattern) []string`, `workflowFiles(t) []string`, `reusableWorkflows(t) []string`, `actionFiles(t) []string`, `everyStep(t) []located`, `usesRefs(*yaml.Node) []usesRef`, `(w *workflow) trigger(name) (*yaml.Node, bool)`, `(w *workflow) triggerNames() []string`, `(w *workflow) call(t) callSpec`, `mustJob(t, w, id) *job`, `mustStep(t, where, steps, id) (int, step)`, `wantPerms(t, where, p, want)`, `wantEnv(t, where, s, want)`, `wantWith(t, where, s, want)`, `wantRun(t, where, s, subs...)`, `wantOptionalSecrets(t, where, c, names...)`, `mentions(s, sub) bool`, `invokes(run, sub) bool`, `checkUses(value, comment) error`, `checkTofu(run) []error`.

- [ ] **Step 1: Write the failing rule tables in `internal/workflowspec/rules_test.go`**

```go
package workflowspec

import (
	"strings"
	"testing"
)

func TestCheckUses(t *testing.T) {
	sha := strings.Repeat("a", 40)
	tests := []struct {
		name, value, comment string
		ok                   bool
	}{
		{"pinned action", "actions/checkout@" + sha, "# v7.0.1", true},
		{"pinned action with a trailing note", "actions/create-github-app-token@" + sha, "# v3.2.0 # zizmor: ignore[github-app] reason", true},
		{"local action", "./.idp/engine/.github/actions/setup-idp", "", true},
		{"reusable workflow, unreleased", "acme/idp-engine/.github/workflows/pr.yaml@" + sha, "# unreleased", true},
		{"reusable workflow, released", "acme/idp-engine/.github/workflows/pr.yaml@" + sha, "# v0.1.0", true},
		{"tag", "actions/checkout@v7", "# v7", false},
		{"short sha", "actions/checkout@" + sha[:12], "# v7.0.1", false},
		{"uppercase sha", "actions/checkout@" + strings.Repeat("A", 40), "# v7.0.1", false},
		{"no comment", "actions/checkout@" + sha, "", false},
		{"comment without a version", "actions/checkout@" + sha, "# latest", false},
		{"unreleased on an action", "actions/checkout@" + sha, "# unreleased", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkUses(tt.value, tt.comment)
			if (err == nil) != tt.ok {
				t.Errorf("checkUses(%q, %q) = %v, want ok=%v", tt.value, tt.comment, err, tt.ok)
			}
		})
	}
}

func TestCheckTofu(t *testing.T) {
	tests := []struct {
		name, run string
		errs      int
	}{
		{"init with the lock file", "tofu init -input=false -lockfile=readonly", 0},
		{"init without it", "tofu init -input=false", 1},
		{"plan under RUNNER_TEMP/idp", `tofu plan -input=false -lock=false -out="$RUNNER_TEMP/idp/plans/github.tfplan"`, 0},
		{"plan into the stack directory", "tofu plan -input=false -out=github.tfplan", 1},
		{"plan without a plan file", "tofu plan -input=false", 1},
		{"apply a saved plan", `tofu apply -input=false "$RUNNER_TEMP/idp/in/github.tfplan"`, 0},
		{"apply without a plan", "tofu apply -input=false -auto-approve", 1},
		{"show is not checked", `tofu show -json "$RUNNER_TEMP/idp/plans/github.tfplan" > out.json`, 0},
		{"two bad lines", "tofu init\ntofu apply -auto-approve", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkTofu(tt.run); len(got) != tt.errs {
				t.Errorf("checkTofu(%q) = %v, want %d error(s)", tt.run, got, tt.errs)
			}
		})
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/`
Expected: FAIL to compile — `undefined: checkUses`, `undefined: checkTofu` (and possibly "no non-test Go files", fixed in Step 3).

- [ ] **Step 3: Write `internal/workflowspec/doc.go` and the rule functions**

`internal/workflowspec/doc.go`:

```go
// Package workflowspec holds the contract tests of the engine's GitHub Actions
// files: the reusable workflows, the setup-idp composite action and the claims
// repo templates. The tests read the YAML and pin the rules that the spec and
// ADRs 0018 and 0021–0024 set, so a change that breaks one fails go test.
package workflowspec
```

Prepend to `internal/workflowspec/rules_test.go` (keep the tests below it; merge the imports):

```go
package workflowspec

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var (
	pinnedRef   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}$`)
	versionNote = regexp.MustCompile(`^# v\d+\.\d+\.\d+(\s|$)`)
	unreleased  = regexp.MustCompile(`^# unreleased(\s|$)`)
)

// checkUses enforces spec §7.5: actions and reusable workflows from other
// repositories are pinned to a full commit SHA, with the version as a trailing
// comment that Renovate keeps in step. An engine workflow ref of an unreleased
// engine says "# unreleased" instead. Local actions (./...) are checked by
// TestJobsBuildIdpFromTheWorkflowCommit.
func checkUses(value, comment string) error {
	if strings.HasPrefix(value, "./") {
		return nil
	}
	if !pinnedRef.MatchString(value) {
		return fmt.Errorf("%q is not pinned to a full commit SHA", value)
	}
	if strings.Contains(value, "/.github/workflows/") && unreleased.MatchString(comment) {
		return nil
	}
	if !versionNote.MatchString(comment) {
		return fmt.Errorf("%q needs a trailing '# vX.Y.Z' comment, got %q", value, comment)
	}
	return nil
}

// checkTofu enforces the wet hygiene rules of the Phase 1b log on one run
// script: init verifies the shipped lock file, plans are written under
// $RUNNER_TEMP/idp/ (never into a stack directory that is later synced to wet),
// and an apply only ever applies a saved plan.
func checkTofu(run string) []error {
	var errs []error
	for _, l := range strings.Split(run, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.Contains(l, "tofu init") && !strings.Contains(l, "-lockfile=readonly"):
			errs = append(errs, fmt.Errorf("%q: tofu init must use -lockfile=readonly", l))
		case strings.Contains(l, "tofu plan") && !strings.Contains(l, `-out="$RUNNER_TEMP/idp/`):
			errs = append(errs, fmt.Errorf("%q: tofu plan must write its plan under $RUNNER_TEMP/idp/", l))
		case strings.Contains(l, "tofu apply") && (strings.Contains(l, "-auto-approve") || !strings.Contains(l, ".tfplan")):
			errs = append(errs, fmt.Errorf("%q: tofu apply must apply a saved plan", l))
		}
	}
	return errs
}
```

- [ ] **Step 4: Run the rule tables**

Run: `go test ./internal/workflowspec/ -run 'TestCheck'`
Expected: PASS.

- [ ] **Step 5: Write the YAML model and helpers, `internal/workflowspec/load_test.go`**

```go
package workflowspec

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// repoRoot is the repository root, seen from this package's directory.
const repoRoot = "../.."

// Paths and values the rules share (ADR-0021).
const (
	setupIDPPath = ".github/actions/setup-idp/action.yaml"
	setupIDP     = "./.idp/engine/.github/actions/setup-idp"
	engineDir    = ".idp/engine"
	workflowRepo = "${{ job.workflow_repository }}"
	workflowSHA  = "${{ job.workflow_sha }}"
)

// strmap decodes a mapping of scalars, keeping each value's literal text
// whatever its YAML type (false, 7 and "x" all become strings).
type strmap map[string]string

func (m *strmap) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: want a mapping", n.Line)
	}
	out := strmap{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if v.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: %s must be a scalar", v.Line, k.Value)
		}
		out[k.Value] = v.Value
	}
	*m = out
	return nil
}

// strlist decodes a scalar or a sequence of scalars (needs: a, or needs: [a, b]).
type strlist []string

func (l *strlist) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*l = strlist{n.Value}
		return nil
	}
	var s []string
	if err := n.Decode(&s); err != nil {
		return err
	}
	*l = s
	return nil
}

// perms is a permissions block; present is false when the key is absent.
type perms struct {
	present bool
	scalar  string            // read-all or write-all, which the rules forbid
	levels  map[string]string // scope -> read | write | none
}

func (p *perms) UnmarshalYAML(n *yaml.Node) error {
	p.present = true
	if n.Kind == yaml.ScalarNode {
		p.scalar = n.Value
		return nil
	}
	var m strmap
	if err := n.Decode(&m); err != nil {
		return err
	}
	p.levels = map[string]string(m)
	return nil
}

type workflow struct {
	path        string
	root        *yaml.Node
	On          yaml.Node       `yaml:"on"`
	Env         strmap          `yaml:"env"`
	Permissions perms           `yaml:"permissions"`
	Concurrency *concurrency    `yaml:"concurrency"`
	Jobs        map[string]*job `yaml:"jobs"`
}

type concurrency struct {
	Group            string `yaml:"group"`
	CancelInProgress bool   `yaml:"cancel-in-progress"`
	Queue            string `yaml:"queue"`
}

type job struct {
	Name           string    `yaml:"name"`
	Needs          strlist   `yaml:"needs"`
	If             string    `yaml:"if"`
	Environment    string    `yaml:"environment"`
	TimeoutMinutes int       `yaml:"timeout-minutes"`
	Env            strmap    `yaml:"env"`
	Permissions    perms     `yaml:"permissions"`
	Outputs        strmap    `yaml:"outputs"`
	Uses           string    `yaml:"uses"`
	With           strmap    `yaml:"with"`
	Secrets        yaml.Node `yaml:"secrets"`
	Steps          []step    `yaml:"steps"`
}

type step struct {
	ID               string `yaml:"id"`
	Name             string `yaml:"name"`
	If               string `yaml:"if"`
	Uses             string `yaml:"uses"`
	Run              string `yaml:"run"`
	Shell            string `yaml:"shell"`
	WorkingDirectory string `yaml:"working-directory"`
	With             strmap `yaml:"with"`
	Env              strmap `yaml:"env"`
}

// label names a step in failure messages.
func (s step) label() string {
	for _, l := range []string{s.ID, s.Name, s.Uses} {
		if l != "" {
			return l
		}
	}
	first, _, _ := strings.Cut(s.Run, "\n")
	return first
}

type action struct {
	path   string
	root   *yaml.Node
	Inputs map[string]struct {
		Required bool `yaml:"required"`
	} `yaml:"inputs"`
	Runs struct {
		Using string `yaml:"using"`
		Steps []step `yaml:"steps"`
	} `yaml:"runs"`
}

// callSpec is the workflow_call interface of a reusable workflow.
type callSpec struct {
	Inputs map[string]struct {
		Required bool   `yaml:"required"`
		Type     string `yaml:"type"`
	} `yaml:"inputs"`
	Secrets map[string]struct {
		Required bool `yaml:"required"`
	} `yaml:"secrets"`
}

// readYAML parses a repo-relative file into out and returns its document node.
func readYAML(t *testing.T, rel string, out any) *yaml.Node {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	if err := doc.Decode(out); err != nil {
		t.Fatalf("decode %s: %v", rel, err)
	}
	return &doc
}

func loadWorkflow(t *testing.T, rel string) *workflow {
	t.Helper()
	w := &workflow{path: rel}
	w.root = readYAML(t, rel, w)
	return w
}

func loadAction(t *testing.T, rel string) *action {
	t.Helper()
	a := &action{path: rel}
	a.root = readYAML(t, rel, a)
	return a
}

// glob lists the repo-relative slash paths matching pattern, sorted.
func glob(t *testing.T, pattern string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(pattern)))
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		rel, err := filepath.Rel(repoRoot, m)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	slices.Sort(out)
	return out
}

// workflowFiles is every workflow the common rules cover: the engine's own and
// the claims repo templates.
func workflowFiles(t *testing.T) []string {
	t.Helper()
	files := append(glob(t, ".github/workflows/*.yaml"), glob(t, "examples/claims-repo/.github/workflows/*.yaml")...)
	if len(files) == 0 {
		t.Fatal("no workflow files found; is repoRoot right?")
	}
	return files
}

// reusableWorkflows lists the engine workflows that claims repos call: every
// workflow except ci.yaml (ADR-0021).
func reusableWorkflows(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, f := range glob(t, ".github/workflows/*.yaml") {
		if f != ".github/workflows/ci.yaml" {
			out = append(out, f)
		}
	}
	return out
}

func actionFiles(t *testing.T) []string {
	t.Helper()
	return glob(t, ".github/actions/*/action.yaml")
}

// located is a step with where it lives, for failure messages.
type located struct {
	where string
	step  step
}

// everyStep returns the steps of every workflow job and every composite action.
func everyStep(t *testing.T) []located {
	t.Helper()
	var out []located
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			for _, s := range w.Jobs[id].Steps {
				out = append(out, located{where: f + " job " + id, step: s})
			}
		}
	}
	for _, f := range actionFiles(t) {
		for _, s := range loadAction(t, f).Runs.Steps {
			out = append(out, located{where: f, step: s})
		}
	}
	return out
}

// usesRef is one uses: value with its trailing comment.
type usesRef struct {
	value, comment string
	line           int
}

// usesRefs walks a document and returns every uses: entry.
func usesRefs(n *yaml.Node) []usesRef {
	var out []usesRef
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Value == "uses" && v.Kind == yaml.ScalarNode {
				c := v.LineComment
				if c == "" {
					c = k.LineComment
				}
				out = append(out, usesRef{value: v.Value, comment: c, line: v.Line})
			}
		}
	}
	for _, c := range n.Content {
		out = append(out, usesRefs(c)...)
	}
	return out
}

// trigger returns the node of one on: event and whether it is present. A bare
// "workflow_dispatch:" is present with a null node.
func (w *workflow) trigger(name string) (*yaml.Node, bool) {
	if w.On.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(w.On.Content); i += 2 {
		if w.On.Content[i].Value == name {
			return w.On.Content[i+1], true
		}
	}
	return nil, false
}

// triggerNames lists the on: events in file order.
func (w *workflow) triggerNames() []string {
	var out []string
	if w.On.Kind == yaml.MappingNode {
		for i := 0; i < len(w.On.Content); i += 2 {
			out = append(out, w.On.Content[i].Value)
		}
	}
	return out
}

func (w *workflow) call(t *testing.T) callSpec {
	t.Helper()
	n, ok := w.trigger("workflow_call")
	if !ok {
		t.Fatalf("%s: no workflow_call trigger", w.path)
	}
	var c callSpec
	if err := n.Decode(&c); err != nil {
		t.Fatalf("%s: workflow_call: %v", w.path, err)
	}
	return c
}

func mustJob(t *testing.T, w *workflow, id string) *job {
	t.Helper()
	j, ok := w.Jobs[id]
	if !ok {
		t.Fatalf("%s: no job %q", w.path, id)
	}
	return j
}

// mustStep returns the index and the step with the given id.
func mustStep(t *testing.T, where string, steps []step, id string) (int, step) {
	t.Helper()
	for i, s := range steps {
		if s.ID == id {
			return i, s
		}
	}
	t.Fatalf("%s: no step with id %q", where, id)
	return -1, step{}
}

// wantPerms compares a permissions block with the exact scopes expected.
func wantPerms(t *testing.T, where string, p perms, want map[string]string) {
	t.Helper()
	if !p.present || p.scalar != "" {
		t.Errorf("%s: want permissions %v, got none or a scalar", where, want)
		return
	}
	if !maps.Equal(p.levels, want) {
		t.Errorf("%s: permissions = %v, want %v", where, p.levels, want)
	}
}

func wantEnv(t *testing.T, where string, s step, want map[string]string) {
	t.Helper()
	for _, k := range slices.Sorted(maps.Keys(want)) {
		if got := s.Env[k]; got != want[k] {
			t.Errorf("%s: env %s = %q, want %q", where, k, got, want[k])
		}
	}
}

func wantWith(t *testing.T, where string, s step, want map[string]string) {
	t.Helper()
	for _, k := range slices.Sorted(maps.Keys(want)) {
		if got := s.With[k]; got != want[k] {
			t.Errorf("%s: with %s = %q, want %q", where, k, got, want[k])
		}
	}
}

func wantRun(t *testing.T, where string, s step, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(s.Run, sub) {
			t.Errorf("%s: run lacks %q", where, sub)
		}
	}
}

// wantOptionalSecrets checks that a reusable workflow declares exactly these
// secrets and requires none: a fork PR gets no secrets and must still reach the
// verdict, and each job that needs a secret names the missing one (ruling P-11).
func wantOptionalSecrets(t *testing.T, where string, c callSpec, names ...string) {
	t.Helper()
	if got, want := slices.Sorted(maps.Keys(c.Secrets)), slices.Sorted(slices.Values(names)); !slices.Equal(got, want) {
		t.Errorf("%s: secrets = %v, want %v", where, got, want)
	}
	for _, n := range slices.Sorted(maps.Keys(c.Secrets)) {
		if c.Secrets[n].Required {
			t.Errorf("%s: secret %s must not be required", where, n)
		}
	}
}

// mentions reports whether a step's run, if, with or env text contains sub.
func mentions(s step, sub string) bool {
	if strings.Contains(s.Run, sub) || strings.Contains(s.If, sub) {
		return true
	}
	for _, m := range []strmap{s.With, s.Env} {
		for _, v := range m {
			if strings.Contains(v, sub) {
				return true
			}
		}
	}
	return false
}

// invokes reports whether a run script calls "idp <sub>" at the start of a
// line. Messages that merely mention idp start with echo and do not count.
func invokes(run, sub string) bool {
	for _, l := range strings.Split(run, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "idp "+sub) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6: Write the common rules, `internal/workflowspec/common_test.go`**

```go
package workflowspec

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

func TestWorkflowFilesUseTheYamlExtension(t *testing.T) {
	for _, pattern := range []string{".github/workflows/*.yml", "examples/claims-repo/.github/workflows/*.yml", ".github/actions/*/action.yml"} {
		if got := glob(t, pattern); len(got) != 0 {
			t.Errorf("rename %v to .yaml: the contract tests read only .yaml files", got)
		}
	}
}

func TestWorkflowsDenyPermissionsByDefault(t *testing.T) {
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		wantPerms(t, f+" (top level)", w.Permissions, map[string]string{})
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			p := w.Jobs[id].Permissions
			if !p.present || p.scalar != "" {
				t.Errorf("%s job %s: list the job's permission scopes explicitly", f, id)
			}
		}
	}
}

func TestRemoteUsesArePinned(t *testing.T) {
	for _, f := range append(workflowFiles(t), actionFiles(t)...) {
		root := readYAML(t, f, &struct{}{})
		for _, u := range usesRefs(root) {
			if err := checkUses(u.value, u.comment); err != nil {
				t.Errorf("%s:%d: %v", f, u.line, err)
			}
		}
	}
}

func TestCheckoutNeverPersistsCredentials(t *testing.T) {
	for _, l := range everyStep(t) {
		if strings.HasPrefix(l.step.Uses, "actions/checkout@") && l.step.With["persist-credentials"] != "false" {
			t.Errorf("%s: checkout %q must set persist-credentials: false", l.where, l.step.label())
		}
	}
}

func TestRunScriptsTakeValuesFromEnv(t *testing.T) {
	for _, l := range everyStep(t) {
		if strings.Contains(l.step.Run, "${{") {
			t.Errorf("%s: step %q expands an expression inside run; pass it through env instead", l.where, l.step.label())
		}
	}
}

func TestTokensAreStepScoped(t *testing.T) {
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
			if _, ok := w.Env[k]; ok {
				t.Errorf("%s: workflow-level env sets %s; set tokens on the step that uses them (ADR-0023)", f, k)
			}
			for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
				if _, ok := w.Jobs[id].Env[k]; ok {
					t.Errorf("%s job %s: job-level env sets %s; set tokens on the step that uses them (ADR-0023)", f, id, k)
				}
			}
		}
	}
}

func TestBotWritesUseTheWorkflowToken(t *testing.T) {
	for _, l := range everyStep(t) {
		if !invokes(l.step.Run, "comment ") && !invokes(l.step.Run, "issue ") {
			continue
		}
		if l.step.Env["GITHUB_TOKEN"] != "${{ github.token }}" {
			t.Errorf("%s: step %q must set GITHUB_TOKEN: ${{ github.token }}, so github-actions[bot] authors it (ADR-0018)", l.where, l.step.label())
		}
		if _, ok := l.step.Env["GH_TOKEN"]; ok {
			t.Errorf("%s: step %q must not set GH_TOKEN; it would take precedence over GITHUB_TOKEN", l.where, l.step.label())
		}
	}
}

func TestWriterKeyStaysInTheWriteEnvironment(t *testing.T) {
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			j := w.Jobs[id]
			for _, s := range j.Steps {
				if mentions(s, "secrets.IDP_WRITER_PRIVATE_KEY") && j.Environment != "idp-write" {
					t.Errorf("%s job %s: step %q reads the writer key outside environment idp-write", f, id, s.label())
				}
				if j.Environment == "idp-approval" && (s.Uses != "" || mentions(s, "secrets.")) {
					t.Errorf("%s job %s: the approval job must hold no secrets and run no actions (spec §6.2)", f, id)
				}
			}
		}
	}
}

func TestJobsBuildIdpFromTheWorkflowCommit(t *testing.T) {
	var inputs map[string]bool
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			where := f + " job " + id
			checkedOut := false
			for _, s := range w.Jobs[id].Steps {
				if strings.HasPrefix(s.Uses, "actions/checkout@") && s.With["path"] == engineDir {
					if s.With["repository"] != workflowRepo || s.With["ref"] != workflowSHA {
						t.Errorf("%s: the engine checkout must use repository %s and ref %s (ADR-0021)", where, workflowRepo, workflowSHA)
					}
					checkedOut = true
				}
				if !strings.HasPrefix(s.Uses, "./") {
					continue
				}
				if s.Uses != setupIDP {
					t.Errorf("%s: unexpected local action %q", where, s.Uses)
					continue
				}
				if !checkedOut {
					t.Errorf("%s: setup-idp runs before the engine checkout", where)
				}
				if s.With["engine-dir"] != engineDir || s.With["version"] != workflowSHA {
					t.Errorf("%s: setup-idp needs engine-dir %s and version %s, got %v", where, engineDir, workflowSHA, s.With)
				}
				if inputs == nil {
					inputs = map[string]bool{}
					for k := range loadAction(t, setupIDPPath).Inputs {
						inputs[k] = true
					}
				}
				for _, k := range slices.Sorted(maps.Keys(s.With)) {
					if !inputs[k] {
						t.Errorf("%s: setup-idp has no input %q", where, k)
					}
				}
			}
		}
	}
}

func TestIdpRunsOnlyAfterSetup(t *testing.T) {
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			ready := false
			for _, s := range w.Jobs[id].Steps {
				if s.Uses == setupIDP {
					ready = true
				}
				if invokes(s.Run, "") && !ready {
					t.Errorf("%s job %s: step %q runs idp before setup-idp", f, id, s.label())
				}
			}
		}
	}
}

func TestTofuFollowsTheWetHygieneRules(t *testing.T) {
	for _, f := range reusableWorkflows(t) {
		w := loadWorkflow(t, f)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			for _, s := range w.Jobs[id].Steps {
				for _, err := range checkTofu(s.Run) {
					t.Errorf("%s job %s step %q: %v", f, id, s.label(), err)
				}
			}
		}
	}
}
```

- [ ] **Step 7: Run the whole package**

Run: `go test ./internal/workflowspec/`
Expected: PASS. Only `ci.yaml` exists yet, and it already follows every common rule (its `tofu` steps are outside `reusableWorkflows`).

If `TestRemoteUsesArePinned` fails on `ci.yaml` because yaml.v3 attaches the trailing comment elsewhere than the value or key node, fix `usesRefs` (not the rule) and say so in the report.

- [ ] **Step 8: Fix the misplaced doc comment in `internal/actions/actions.go`**

Replace

```go
// SetOutput appends a step output to the GITHUB_OUTPUT file at path.
// Mask returns the workflow command that tells the runner to redact value in all later log output.
func Mask(value string) string { return "::add-mask::" + dataEscaper.Replace(value) }

func SetOutput(path, name, value string) error { return appendEntry(path, name, value) }
```

with

```go
// Mask returns the workflow command that tells the runner to redact value in all later log output.
func Mask(value string) string { return "::add-mask::" + dataEscaper.Replace(value) }

// SetOutput appends a step output to the GITHUB_OUTPUT file at path.
func SetOutput(path, name, value string) error { return appendEntry(path, name, value) }
```

- [ ] **Step 9: Verify and commit**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: tests pass, vet is silent, gofmt prints nothing.

```bash
git add internal/workflowspec
git commit -m "test(workflowspec): add the workflow contract harness and common rules"
git add internal/actions/actions.go
git commit -m "docs(actions): put SetOutput's doc comment back on it"
```

---

### Task 3: The `setup-idp` composite action

**Files:**
- Create: `.github/actions/setup-idp/action.yaml`, `internal/workflowspec/setupidp_test.go`
- Modify: `.github/workflows/ci.yaml` (the zizmor step)

**Interfaces:**
- Consumes: Task 2 helpers (`loadAction`, `loadWorkflow`, `mustJob`, `mustStep`, `wantRun`, `setupIDPPath`).
- Produces: the action `./.idp/engine/.github/actions/setup-idp` with inputs `engine-dir` (required) and `version` (required, 40 hex). After it runs, `idp` and `tofu` (1.12.6, no wrapper) are on `PATH`, and `idp`'s `version.Version` is `version`.

- [ ] **Step 1: Write the failing test, `internal/workflowspec/setupidp_test.go`**

```go
package workflowspec

import (
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

func TestSetupIDPInterface(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	if a.Runs.Using != "composite" {
		t.Fatalf("runs.using = %q, want composite", a.Runs.Using)
	}
	if len(a.Inputs) != 2 || !a.Inputs["engine-dir"].Required || !a.Inputs["version"].Required {
		t.Errorf("inputs = %v, want exactly engine-dir and version, both required", a.Inputs)
	}
}

func TestSetupIDPInstallsTheToolchain(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	_, goStep := mustStep(t, setupIDPPath, a.Runs.Steps, "go")
	_, tofuStep := mustStep(t, setupIDPPath, a.Runs.Steps, "tofu")
	if !strings.HasPrefix(goStep.Uses, "actions/setup-go@") || !strings.HasPrefix(tofuStep.Uses, "opentofu/setup-opentofu@") {
		t.Fatalf("steps go and tofu must use setup-go and setup-opentofu, got %q and %q", goStep.Uses, tofuStep.Uses)
	}
	if got := goStep.With["go-version-file"]; got != "${{ inputs.engine-dir }}/go.mod" {
		t.Errorf("setup-go go-version-file = %q", got)
	}
	if goStep.With["cache"] != "false" {
		t.Errorf("setup-go must set cache: false: a cache restored into the job that holds the writer key is a poisoning path (ADR-0021)")
	}
	if tofuStep.With["tofu_wrapper"] != "false" {
		t.Errorf("setup-opentofu must set tofu_wrapper: false")
	}
	if got, want := tofuStep.With["tofu_version"], ciTofuVersion(t); got != want {
		t.Errorf("setup-idp installs OpenTofu %q, CI tests the modules with %q: keep one version", got, want)
	}
}

// ciTofuVersion is the OpenTofu version CI tests the modules with.
func ciTofuVersion(t *testing.T) string {
	t.Helper()
	w := loadWorkflow(t, ".github/workflows/ci.yaml")
	for _, s := range mustJob(t, w, "modules").Steps {
		if strings.HasPrefix(s.Uses, "opentofu/setup-opentofu@") {
			return s.With["tofu_version"]
		}
	}
	t.Fatal("ci.yaml job modules has no setup-opentofu step")
	return ""
}

func TestSetupIDPBuildsTheGivenCommit(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	_, build := mustStep(t, setupIDPPath, a.Runs.Steps, "build")
	if build.Shell != "bash" {
		t.Errorf("build shell = %q, want bash", build.Shell)
	}
	if build.WorkingDirectory != "${{ inputs.engine-dir }}" {
		t.Errorf("build working-directory = %q", build.WorkingDirectory)
	}
	if build.Env["IDP_VERSION"] != "${{ inputs.version }}" {
		t.Errorf("build env IDP_VERSION = %q", build.Env["IDP_VERSION"])
	}
	wantRun(t, "setup-idp build", build,
		`[[ "$IDP_VERSION" =~ ^[0-9a-f]{40}$ ]]`,
		`-X `+version.Repo+`/internal/version.Version=${IDP_VERSION}`,
		`-o "$RUNNER_TEMP/idp-bin/idp" ./cmd/idp`,
		`echo "$RUNNER_TEMP/idp-bin" >> "$GITHUB_PATH"`,
	)
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/workflowspec/ -run TestSetupIDP`
Expected: FAIL with `read .github/actions/setup-idp/action.yaml: open …` (the file does not exist).

- [ ] **Step 3: Write `.github/actions/setup-idp/action.yaml`**

```yaml
name: setup-idp
description: Install Go and OpenTofu, and build the idp CLI from an idp-engine checkout (ADR-0021).

inputs:
  engine-dir:
    description: The idp-engine checkout, relative to the workspace.
    required: true
  version:
    description: The commit of that checkout, 40 hex characters. The build embeds it, and rendered module sources pin it.
    required: true

runs:
  using: composite
  steps:
    - id: go
      uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
      with:
        go-version-file: ${{ inputs.engine-dir }}/go.mod
        cache: false
    - id: tofu
      uses: opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4 # v2.0.2
      with:
        tofu_version: 1.12.6
        tofu_wrapper: false
    - id: build
      name: build idp
      shell: bash
      working-directory: ${{ inputs.engine-dir }}
      env:
        IDP_VERSION: ${{ inputs.version }}
      run: | # zizmor: ignore[github-env] appends a fixed directory under RUNNER_TEMP to GITHUB_PATH; no input reaches the value
        if ! [[ "$IDP_VERSION" =~ ^[0-9a-f]{40}$ ]]; then
          echo "::error::setup-idp: the version input must be a full 40-character commit SHA"
          exit 1
        fi
        mkdir -p "$RUNNER_TEMP/idp-bin"
        go build -trimpath -ldflags "-X github.com/jellalshadows-idp/idp-engine/internal/version.Version=${IDP_VERSION}" -o "$RUNNER_TEMP/idp-bin/idp" ./cmd/idp
        echo "$RUNNER_TEMP/idp-bin" >> "$GITHUB_PATH"
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/workflowspec/`
Expected: PASS (the common rules now also cover the action's steps).

- [ ] **Step 5: Lint the action in CI too**

In `.github/workflows/ci.yaml`, job `workflows`, replace

```yaml
      - name: zizmor
        run: pipx run zizmor==1.30.1 .github/workflows
```

with

```yaml
      - name: zizmor
        run: pipx run zizmor==1.30.1 .github/workflows .github/actions/setup-idp/action.yaml
```

- [ ] **Step 6: Run the linters locally**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`
Expected: no output, exit 0.

Run: `"$ZIZMOR" --offline .github/workflows .github/actions/setup-idp/action.yaml`
Expected: `No findings to report` (ignored findings may be listed as suppressed). If the `github-env` ignore is not honoured, report the exact finding; do not widen the ignore beyond that rule and step.

- [ ] **Step 7: Commit**

```bash
git add .github/actions/setup-idp/action.yaml internal/workflowspec/setupidp_test.go .github/workflows/ci.yaml
git commit -m "feat(actions): add the setup-idp composite action"
```

---

### Task 4: The reusable PR pipeline

**Files:**
- Create: `.github/workflows/pr.yaml`, `.github/actionlint.yaml`, `internal/workflowspec/pr_test.go`

**Interfaces:**
- Consumes: Task 2 helpers; Task 3's `setup-idp`; CLI: `idp validate`, `idp render --out`, `idp diff --new --wet` (output `affected`), `idp encryption-env`, `idp plan-summary --stacks --head-sha --run-url --comment-out --fingerprint-out [--plan github=FILE]`, `idp comment --repo --pr --body-file` (docs/cli.md).
- Produces: `jellalshadows-idp/idp-engine/.github/workflows/pr.yaml`, `on: workflow_call` with optional secrets `IDP_READER_PRIVATE_KEY`, `IDP_STATE_PASSPHRASE`; jobs `validate`, `plan`, `comment`, `verdict`. The caller grants `contents: read` and `pull-requests: write`. The run's result is `success` only when `verdict` passed.

- [ ] **Step 1: Write the failing test, `internal/workflowspec/pr_test.go`**

```go
package workflowspec

import (
	"maps"
	"slices"
	"testing"
)

const prPath = ".github/workflows/pr.yaml"

func TestPRInterface(t *testing.T) {
	w := loadWorkflow(t, prPath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_call"}) {
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	wantOptionalSecrets(t, prPath, w.call(t), "IDP_READER_PRIVATE_KEY", "IDP_STATE_PASSPHRASE")
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"comment", "plan", "validate", "verdict"}) {
		t.Errorf("jobs = %v", got)
	}
}

func TestPRJobPermissions(t *testing.T) {
	w := loadWorkflow(t, prPath)
	wantPerms(t, "validate", mustJob(t, w, "validate").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "plan", mustJob(t, w, "plan").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "comment", mustJob(t, w, "comment").Permissions, map[string]string{"contents": "read", "pull-requests": "write"})
	wantPerms(t, "verdict", mustJob(t, w, "verdict").Permissions, map[string]string{})
}

func TestPRPlansOnlySameRepoBranches(t *testing.T) {
	w := loadWorkflow(t, prPath)
	plan := mustJob(t, w, "plan")
	if want := "github.event.pull_request.head.repo.full_name == github.repository"; plan.If != want {
		t.Errorf("plan if = %q, want %q (spec §6.1: forks only validate and render)", plan.If, want)
	}
	if !slices.Equal(plan.Needs, strlist{"validate"}) {
		t.Errorf("plan needs = %v, want [validate]", plan.Needs)
	}
	if got := mustJob(t, w, "comment").Needs; !slices.Equal(got, strlist{"plan"}) {
		t.Errorf("comment needs = %v, want [plan]", got)
	}
}

func TestPRVerdictFailsForksAndFailures(t *testing.T) {
	w := loadWorkflow(t, prPath)
	v := mustJob(t, w, "verdict")
	if v.If != "always()" {
		t.Errorf("verdict if = %q, want always(): it must report even when a job failed or was skipped", v.If)
	}
	if got := slices.Sorted(slices.Values(v.Needs)); !slices.Equal(got, []string{"comment", "plan", "validate"}) {
		t.Errorf("verdict needs = %v", got)
	}
	_, s := mustStep(t, "verdict", v.Steps, "decide")
	wantEnv(t, "verdict decide", s, map[string]string{
		"SAME_REPO": "${{ github.event.pull_request.head.repo.full_name == github.repository }}",
		"VALIDATE":  "${{ needs.validate.result }}",
		"PLAN":      "${{ needs.plan.result }}",
		"COMMENT":   "${{ needs.comment.result }}",
	})
	wantRun(t, "verdict decide", s,
		`if [ "$SAME_REPO" != true ]; then`,
		"a maintainer must push this branch to the repo",
		`if [ "$VALIDATE" != success ] || [ "$PLAN" != success ] || [ "$COMMENT" != success ]; then`,
	)
}

func TestPRPlanSummaryCoversTheAffectedStacksAtTheHeadCommit(t *testing.T) {
	w := loadWorkflow(t, prPath)
	plan := mustJob(t, w, "plan")
	_, d := mustStep(t, "plan", plan.Steps, "diff")
	wantRun(t, "plan diff", d, `idp diff --new "$RUNNER_TEMP/idp/work/rendered" --wet .idp/wet/rendered`)
	_, st := mustStep(t, "plan", plan.Steps, "stacks")
	wantRun(t, "plan stacks", st, `jq -e 'all(.[]; . == "github")'`)
	_, s := mustStep(t, "plan", plan.Steps, "summary")
	if s.If != "" {
		t.Errorf("summary if = %q: it must run whatever is affected, so a PR with no changes still gets its comment", s.If)
	}
	wantEnv(t, "plan summary", s, map[string]string{
		"AFFECTED":     "${{ steps.diff.outputs.affected }}",
		"GITHUB_STACK": "${{ steps.stacks.outputs.github }}",
		"HEAD_SHA":     "${{ github.event.pull_request.head.sha }}",
	})
	wantRun(t, "plan summary", s, `--stacks "$AFFECTED"`, `--head-sha "$HEAD_SHA"`, `plans+=(--plan "github=$RUNNER_TEMP/idp/plans/github.json")`)
	_, up := mustStep(t, "plan", plan.Steps, "upload")
	wantWith(t, "plan upload", up, map[string]string{
		"name":              "idp-pr-comment",
		"path":              "${{ runner.temp }}/idp/out/comment.md",
		"retention-days":    "1",
		"overwrite":         "true",
		"if-no-files-found": "error",
	})
}

func TestPRCommentPostsTheSavedComment(t *testing.T) {
	w := loadWorkflow(t, prPath)
	c := mustJob(t, w, "comment")
	_, dl := mustStep(t, "comment", c.Steps, "download")
	wantWith(t, "comment download", dl, map[string]string{"name": "idp-pr-comment", "path": "${{ runner.temp }}/idp/out"})
	_, post := mustStep(t, "comment", c.Steps, "post")
	wantEnv(t, "comment post", post, map[string]string{"PR_NUMBER": "${{ github.event.pull_request.number }}"})
	wantRun(t, "comment post", post, `idp comment --repo "$GITHUB_REPOSITORY" --pr "$PR_NUMBER" --body-file "$RUNNER_TEMP/idp/out/comment.md"`)
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/workflowspec/ -run TestPR`
Expected: FAIL with `read .github/workflows/pr.yaml: open …`.

- [ ] **Step 3: Write `.github/actionlint.yaml`**

```yaml
# actionlint configuration. Every ignore is narrow and says why.
paths:
  .github/workflows/*.yaml:
    ignore:
      # job.workflow_repository and job.workflow_sha are documented job-context
      # properties (the reusable workflow's own repository and commit, ADR-0021)
      # that actionlint v1.7.12 does not know yet.
      - 'property "workflow_(repository|sha)" is not defined in object type'
```

- [ ] **Step 4: Write `.github/workflows/pr.yaml`**

```yaml
name: idp-pr

# Reusable PR pipeline (spec §6.1): validate and render, diff against wet, plan
# the affected stacks with the reader App, keep one sticky plan comment, and end
# in a verdict that the caller's idp-gate job mirrors (ADR-0022).

on:
  workflow_call:
    secrets:
      IDP_READER_PRIVATE_KEY:
        description: The reader App's private key (repo secret).
        required: false
      IDP_STATE_PASSPHRASE:
        description: The OpenTofu state passphrase (repo secret).
        required: false

permissions: {}

defaults:
  run:
    shell: bash

jobs:
  validate:
    name: validate
    runs-on: ubuntu-24.04
    timeout-minutes: 15
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          path: .idp/engine
          persist-credentials: false
      - uses: ./.idp/engine/.github/actions/setup-idp # zizmor: ignore[self-repository] the engine is checked out at job.workflow_sha in the step above (ADR-0021)
        with:
          engine-dir: .idp/engine
          version: ${{ job.workflow_sha }}
      - name: validate the claims
        run: idp validate
      - name: render the claims
        run: idp render --out "$RUNNER_TEMP/idp/rendered"

  plan:
    name: plan
    needs: validate
    if: github.event.pull_request.head.repo.full_name == github.repository
    runs-on: ubuntu-24.04
    timeout-minutes: 30
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          path: .idp/engine
          persist-credentials: false
      - uses: ./.idp/engine/.github/actions/setup-idp # zizmor: ignore[self-repository] the engine is checked out at job.workflow_sha in the step above (ADR-0021)
        with:
          engine-dir: .idp/engine
          version: ${{ job.workflow_sha }}
      - name: check the repository configuration
        id: config
        env:
          READER_CLIENT_ID: ${{ vars.IDP_READER_CLIENT_ID }}
          READER_KEY_SET: ${{ secrets.IDP_READER_PRIVATE_KEY != '' }}
          PASSPHRASE_SET: ${{ secrets.IDP_STATE_PASSPHRASE != '' }}
        run: |
          missing=""
          [ -n "$READER_CLIENT_ID" ] || missing="$missing variable IDP_READER_CLIENT_ID,"
          [ "$READER_KEY_SET" = true ] || missing="$missing secret IDP_READER_PRIVATE_KEY,"
          [ "$PASSPHRASE_SET" = true ] || missing="$missing secret IDP_STATE_PASSPHRASE,"
          if [ -n "$missing" ]; then
            echo "::error::the claims repo is missing:${missing%,}. Re-run the bootstrap (docs/runbooks/bootstrap.md) and pass the secrets from the caller workflow."
            exit 1
          fi
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: wet
          path: .idp/wet
          persist-credentials: false
      - name: render the claims
        id: render
        run: idp render --out "$RUNNER_TEMP/idp/work/rendered"
      - name: diff against wet
        id: diff
        run: idp diff --new "$RUNNER_TEMP/idp/work/rendered" --wet .idp/wet/rendered
      - name: check the affected stacks
        id: stacks
        env:
          AFFECTED: ${{ steps.diff.outputs.affected }}
        run: |
          if ! jq -e 'all(.[]; . == "github")' <<<"$AFFECTED" >/dev/null; then
            echo "::error::this engine version reconciles only the github stack; affected: $AFFECTED"
            exit 1
          fi
          echo "github=$(jq -r 'any(.[]; . == "github")' <<<"$AFFECTED")" >> "$GITHUB_OUTPUT"
      - name: mint the reader token
        id: reader
        if: steps.stacks.outputs.github == 'true'
        uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0 # zizmor: ignore[github-app] the read-only reader App plans every managed repo (ADR-0023)
        with:
          client-id: ${{ vars.IDP_READER_CLIENT_ID }}
          private-key: ${{ secrets.IDP_READER_PRIVATE_KEY }}
          owner: ${{ github.repository_owner }}
      - name: export TF_ENCRYPTION
        id: encryption
        if: steps.stacks.outputs.github == 'true'
        env:
          IDP_STATE_PASSPHRASE: ${{ secrets.IDP_STATE_PASSPHRASE }}
        run: idp encryption-env
      - name: plan the github stack
        id: plan
        if: steps.stacks.outputs.github == 'true'
        working-directory: ${{ runner.temp }}/idp/work/rendered/github
        env:
          GITHUB_TOKEN: ${{ steps.reader.outputs.token }}
        run: |
          mkdir -p "$RUNNER_TEMP/idp/work/tfstate" "$RUNNER_TEMP/idp/plans"
          if [ -f "$GITHUB_WORKSPACE/.idp/wet/tfstate/github.tfstate" ]; then
            cp "$GITHUB_WORKSPACE/.idp/wet/tfstate/github.tfstate" "$RUNNER_TEMP/idp/work/tfstate/github.tfstate"
          fi
          tofu init -input=false -lockfile=readonly
          tofu plan -input=false -lock=false -out="$RUNNER_TEMP/idp/plans/github.tfplan"
          tofu show -json "$RUNNER_TEMP/idp/plans/github.tfplan" > "$RUNNER_TEMP/idp/plans/github.json"
      - name: summarize the plan
        id: summary
        env:
          AFFECTED: ${{ steps.diff.outputs.affected }}
          GITHUB_STACK: ${{ steps.stacks.outputs.github }}
          HEAD_SHA: ${{ github.event.pull_request.head.sha }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
        run: |
          plans=()
          if [ "$GITHUB_STACK" = true ]; then plans+=(--plan "github=$RUNNER_TEMP/idp/plans/github.json"); fi
          mkdir -p "$RUNNER_TEMP/idp/out"
          idp plan-summary --stacks "$AFFECTED" --head-sha "$HEAD_SHA" --run-url "$RUN_URL" \
            --comment-out "$RUNNER_TEMP/idp/out/comment.md" \
            --fingerprint-out "$RUNNER_TEMP/idp/out/fingerprint.json" "${plans[@]}"
      - name: keep the comment for the comment job
        id: upload
        uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1
        with:
          name: idp-pr-comment
          path: ${{ runner.temp }}/idp/out/comment.md
          retention-days: 1
          overwrite: true
          if-no-files-found: error

  comment:
    name: comment
    needs: plan
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    permissions:
      contents: read
      pull-requests: write
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          path: .idp/engine
          persist-credentials: false
      - uses: ./.idp/engine/.github/actions/setup-idp # zizmor: ignore[self-repository] the engine is checked out at job.workflow_sha in the step above (ADR-0021)
        with:
          engine-dir: .idp/engine
          version: ${{ job.workflow_sha }}
      - name: fetch the comment
        id: download
        uses: actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c # v8.0.1
        with:
          name: idp-pr-comment
          path: ${{ runner.temp }}/idp/out
      - name: post the sticky plan comment
        id: post
        env:
          GITHUB_TOKEN: ${{ github.token }}
          PR_NUMBER: ${{ github.event.pull_request.number }}
        run: idp comment --repo "$GITHUB_REPOSITORY" --pr "$PR_NUMBER" --body-file "$RUNNER_TEMP/idp/out/comment.md"

  verdict:
    name: verdict
    needs: [validate, plan, comment]
    if: always()
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    permissions: {}
    steps:
      - name: decide
        id: decide
        env:
          SAME_REPO: ${{ github.event.pull_request.head.repo.full_name == github.repository }}
          VALIDATE: ${{ needs.validate.result }}
          PLAN: ${{ needs.plan.result }}
          COMMENT: ${{ needs.comment.result }}
        run: |
          if [ "$SAME_REPO" != true ]; then
            echo "::error::a maintainer must push this branch to the repo: pull requests from forks are validated but never planned"
            exit 1
          fi
          if [ "$VALIDATE" != success ] || [ "$PLAN" != success ] || [ "$COMMENT" != success ]; then
            echo "::error::the idp pipeline did not succeed (validate: $VALIDATE, plan: $PLAN, comment: $COMMENT)"
            exit 1
          fi
          echo "idp: validated, planned and commented"
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/workflowspec/`
Expected: PASS, including every common rule over `pr.yaml`.

- [ ] **Step 6: Run the linters**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`
Expected: no output. Without `.github/actionlint.yaml` it would report `property "workflow_sha" is not defined in object type`; with it, nothing. Note: on the CI runner actionlint also runs shellcheck over every `run:` script (shellcheck is installed there, not locally), so CI can report script findings this local run cannot; fix them in the script, never with a blanket ignore.

Run: `"$ZIZMOR" --offline .github/workflows .github/actions/setup-idp/action.yaml`
Expected: no unsuppressed findings. Any finding the inline ignores do not cover is reported with its exact text; fix the YAML, or add an ignore only for that rule on that line, with the reason in the comment.

- [ ] **Step 7: Commit**

```bash
git add .github/workflows/pr.yaml .github/actionlint.yaml internal/workflowspec/pr_test.go
git commit -m "feat(workflows): add the reusable pr pipeline"
```

---

### Task 5: The reusable reconcile pipeline

**Files:**
- Create: `.github/workflows/reconcile.yaml`, `internal/workflowspec/reconcile_test.go`

**Interfaces:**
- Consumes: Task 2 helpers; Task 3's `setup-idp`; CLI: `render`, `diff [--all]`, `encryption-env`, `plan-summary` (outputs `changes`, `destructive`), `gate --repo --sha --fingerprint` (output `decision`), `wet-push --repo --root --message --path…`, `issue open` (docs/cli.md); `bootstrap/apps/writer.json`.
- Produces: `reconcile.yaml`, `on: workflow_call` with optional secrets `IDP_READER_PRIVATE_KEY`, `IDP_STATE_PASSPHRASE`, `IDP_WRITER_PRIVATE_KEY`; jobs `plan`, `approve`, `apply-github`; the artifact `idp-wet-root` (layout: `wet-base`, `tfstate/github.tfstate`, optional `rendered/github/`), which Task 6's `recover.yaml` consumes. Also the test helper `checkPushToken(t *testing.T, where string, s step)` that Task 6 reuses.

- [ ] **Step 1: Write the failing test, `internal/workflowspec/reconcile_test.go`**

```go
package workflowspec

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const reconcilePath = ".github/workflows/reconcile.yaml"

func TestReconcileInterface(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_call"}) {
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	wantOptionalSecrets(t, reconcilePath, w.call(t), "IDP_READER_PRIVATE_KEY", "IDP_STATE_PASSPHRASE", "IDP_WRITER_PRIVATE_KEY")
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"apply-github", "approve", "plan"}) {
		t.Errorf("jobs = %v", got)
	}
	wantPerms(t, "plan", mustJob(t, w, "plan").Permissions, map[string]string{"contents": "read", "issues": "read", "pull-requests": "read"})
	wantPerms(t, "approve", mustJob(t, w, "approve").Permissions, map[string]string{})
	wantPerms(t, "apply-github", mustJob(t, w, "apply-github").Permissions, map[string]string{"contents": "read", "issues": "write"})
}

func TestReconcileIsNeverReRun(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	for _, id := range []string{"plan", "apply-github"} {
		j := mustJob(t, w, id)
		if len(j.Steps) == 0 || j.Steps[0].ID != "guard" {
			t.Errorf("%s: the first step must be the guard", id)
			continue
		}
		wantRun(t, id+" guard", j.Steps[0], `if [ "$GITHUB_RUN_ATTEMPT" != 1 ]; then`)
	}
	_, g := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "guard")
	wantEnv(t, "plan guard", g, map[string]string{"REF": "${{ github.ref }}"})
	wantRun(t, "plan guard", g, `if [ "$REF" != refs/heads/main ]; then`)
}

func TestReconcileDiffsAgainstTheCurrentWet(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	plan := mustJob(t, w, "plan")
	_, d := mustStep(t, "plan", plan.Steps, "diff")
	wantEnv(t, "plan diff", d, map[string]string{"EVENT": "${{ github.event_name }}"})
	wantRun(t, "plan diff", d,
		`if [ "$EVENT" = workflow_dispatch ]; then all=(--all); fi`,
		`idp diff --new "$RUNNER_TEMP/idp/out/rendered" --wet .idp/wet/rendered "${all[@]}"`,
	)
	want := map[string]string{
		"wet-sha":  "${{ steps.wet.outputs.sha }}",
		"github":   "${{ steps.stacks.outputs.github }}",
		"changes":  "${{ steps.summary.outputs.changes }}",
		"decision": "${{ steps.gate.outputs.decision }}",
	}
	if !maps.Equal(map[string]string(plan.Outputs), want) {
		t.Errorf("plan outputs = %v, want %v", plan.Outputs, want)
	}
}

func TestReconcileGatesWithTheWorkflowToken(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	_, g := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "gate")
	wantEnv(t, "plan gate", g, map[string]string{"GITHUB_TOKEN": "${{ github.token }}"})
	wantRun(t, "plan gate", g, `idp gate --repo "$GITHUB_REPOSITORY" --sha "$GITHUB_SHA" --fingerprint "$RUNNER_TEMP/idp/out/fingerprint.json"`)
	_, s := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "summary")
	wantRun(t, "plan summary", s, `--stacks "$AFFECTED"`, `--head-sha "$GITHUB_SHA"`)
}

func TestApprovalIsOneSecretlessJob(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "approve")
	if a.Environment != "idp-approval" {
		t.Errorf("approve environment = %q, want idp-approval", a.Environment)
	}
	if want := "needs.plan.outputs.decision == 'approval'"; a.If != want {
		t.Errorf("approve if = %q, want %q", a.If, want)
	}
	if !slices.Equal(a.Needs, strlist{"plan"}) {
		t.Errorf("approve needs = %v, want [plan]", a.Needs)
	}
}

func TestApplyRunsOnlyAfterTheGate(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "apply-github")
	want := "always() && needs.plan.result == 'success' && needs.plan.outputs.github == 'true' && (needs.plan.outputs.decision == 'auto' || needs.approve.result == 'success')"
	if a.If != want {
		t.Errorf("apply-github if = %q, want %q", a.If, want)
	}
	if a.Environment != "idp-write" {
		t.Errorf("apply-github environment = %q, want idp-write", a.Environment)
	}
	if got := slices.Sorted(slices.Values(a.Needs)); !slices.Equal(got, []string{"approve", "plan"}) {
		t.Errorf("apply-github needs = %v", got)
	}
}

func TestApplyRefusesWhenWetMoved(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "apply-github")
	iCheckout, co := mustStep(t, "apply-github", a.Steps, "wet-checkout")
	wantWith(t, "apply-github wet-checkout", co, map[string]string{"ref": "wet", "path": ".idp/wet"})
	iBase, base := mustStep(t, "apply-github", a.Steps, "wet-base")
	wantEnv(t, "apply-github wet-base", base, map[string]string{"PLANNED": "${{ needs.plan.outputs.wet-sha }}"})
	wantRun(t, "apply-github wet-base", base, `if [ "$(git -C .idp/wet rev-parse HEAD)" != "$PLANNED" ]; then`)
	iApply, _ := mustStep(t, "apply-github", a.Steps, "apply")
	if !(iCheckout < iBase && iBase < iApply) {
		t.Errorf("apply-github must check out wet, then compare it with the plan's, then apply (got indexes %d, %d, %d)", iCheckout, iBase, iApply)
	}
}

// writerManifestPermissions maps bootstrap/apps/writer.json to permission-*
// inputs. metadata is left out: every installation token carries it.
func writerManifestPermissions(t *testing.T) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "bootstrap", "apps", "writer.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		DefaultPermissions map[string]string `json:"default_permissions"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for k, v := range m.DefaultPermissions {
		if k != "metadata" {
			out["permission-"+strings.ReplaceAll(k, "_", "-")] = v
		}
	}
	return out
}

func TestApplyTokenHasTheWriterManifestPermissions(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	_, s := mustStep(t, "apply-github", mustJob(t, w, "apply-github").Steps, "apply-token")
	if want := "needs.plan.outputs.changes != '0'"; s.If != want {
		t.Errorf("apply-token if = %q, want %q", s.If, want)
	}
	if s.With["repositories"] != "" {
		t.Errorf("the apply token spans the installation: the claims may declare any repository (ADR-0023)")
	}
	got := map[string]string{}
	for k, v := range s.With {
		if strings.HasPrefix(k, "permission-") {
			got[k] = v
		}
	}
	if want := writerManifestPermissions(t); !maps.Equal(got, want) {
		t.Errorf("apply token permissions = %v, want the writer manifest's %v", got, want)
	}
}

// checkPushToken pins the wet-commit token: the claims repo only, contents:
// write only (ADR-0023).
func checkPushToken(t *testing.T, where string, s step) {
	t.Helper()
	wantWith(t, where, s, map[string]string{
		"client-id":           "${{ vars.IDP_WRITER_CLIENT_ID }}",
		"private-key":         "${{ secrets.IDP_WRITER_PRIVATE_KEY }}",
		"owner":               "${{ github.repository_owner }}",
		"repositories":        "${{ github.event.repository.name }}",
		"permission-contents": "write",
	})
	for k := range s.With {
		if strings.HasPrefix(k, "permission-") && k != "permission-contents" {
			t.Errorf("%s: the wet-commit token must hold only contents: write, got %s", where, k)
		}
	}
}

func TestWetPushTokenReachesOnlyTheClaimsRepo(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	_, s := mustStep(t, "apply-github", mustJob(t, w, "apply-github").Steps, "push-token")
	checkPushToken(t, "apply-github push-token", s)
}

func TestApplyCommitsTheRenderOnlyAfterASuccessfulApply(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "apply-github")
	_, apply := mustStep(t, "apply-github", a.Steps, "apply")
	if want := "needs.plan.outputs.changes != '0'"; apply.If != want {
		t.Errorf("apply if = %q, want %q (never always(): it must not run after a failed step)", apply.If, want)
	}
	wantRun(t, "apply-github apply", apply, "tofu init -input=false -lockfile=readonly", `tofu apply -input=false "$RUNNER_TEMP/idp/in/github.tfplan"`)
	_, prep := mustStep(t, "apply-github", a.Steps, "prepare")
	if want := "always() && steps.stage.outcome == 'success'"; prep.If != want {
		t.Errorf("prepare if = %q, want %q", prep.If, want)
	}
	wantEnv(t, "apply-github prepare", prep, map[string]string{
		"CHANGES":       "${{ needs.plan.outputs.changes }}",
		"APPLY_OUTCOME": "${{ steps.apply.outcome }}",
		"WET_SHA":       "${{ needs.plan.outputs.wet-sha }}",
	})
	wantRun(t, "apply-github prepare", prep,
		`echo "$WET_SHA" > "$root/wet-base"`,
		`if [ "$CHANGES" = 0 ] || [ "$APPLY_OUTCOME" = success ]; then`,
		`cp -R "$RUNNER_TEMP/idp/in/rendered/github" "$root/rendered/github"`,
	)
	_, push := mustStep(t, "apply-github", a.Steps, "push")
	wantEnv(t, "apply-github push", push, map[string]string{
		"GH_TOKEN": "${{ steps.push-token.outputs.token }}",
		"RENDER":   "${{ steps.prepare.outputs.render }}",
	})
	wantRun(t, "apply-github push", push,
		`paths=(--path tfstate/github.tfstate)`,
		`if [ "$RENDER" = true ]; then paths+=(--path rendered/github); fi`,
		`idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root"`,
	)
}

func TestWetRootIsSavedBeforeThePush(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "apply-github")
	iKeep, keep := mustStep(t, "apply-github", a.Steps, "keep")
	iPush, push := mustStep(t, "apply-github", a.Steps, "push")
	_, issue := mustStep(t, "apply-github", a.Steps, "issue")
	if iKeep > iPush {
		t.Errorf("the wet root must be uploaded before the push")
	}
	if want := "always() && steps.prepare.outcome == 'success'"; keep.If != want {
		t.Errorf("keep if = %q, want %q", keep.If, want)
	}
	wantWith(t, "apply-github keep", keep, map[string]string{
		"name":                 "idp-wet-root",
		"path":                 "${{ runner.temp }}/idp/wet-root",
		"include-hidden-files": "true",
		"retention-days":       "7",
		"if-no-files-found":    "error",
	})
	if want := "always() && steps.push-token.outcome == 'success'"; push.If != want {
		t.Errorf("push if = %q, want %q", push.If, want)
	}
	if want := "always() && steps.prepare.outcome == 'success' && steps.push.outcome != 'success'"; issue.If != want {
		t.Errorf("issue if = %q, want %q", issue.If, want)
	}
	wantRun(t, "apply-github issue", issue, `idp issue open --repo "$GITHUB_REPOSITORY" --label idp-wet-push --title "Wet push failed"`)
}

func TestRenderArtifactsKeepHiddenFiles(t *testing.T) {
	// .terraform.lock.hcl and .idp-rendered are dotfiles, which upload-artifact
	// drops by default; without the lock file the apply's tofu init fails.
	w := loadWorkflow(t, reconcilePath)
	_, up := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "upload")
	if want := "steps.stacks.outputs.github == 'true'"; up.If != want {
		t.Errorf("upload if = %q, want %q", up.If, want)
	}
	wantWith(t, "plan upload", up, map[string]string{
		"name":                 "idp-reconcile",
		"path":                 "${{ runner.temp }}/idp/out",
		"include-hidden-files": "true",
		"retention-days":       "1",
		"if-no-files-found":    "error",
	})
	a := mustJob(t, w, "apply-github")
	_, dl := mustStep(t, "apply-github", a.Steps, "download")
	wantWith(t, "apply-github download", dl, map[string]string{"name": "idp-reconcile", "path": "${{ runner.temp }}/idp/in"})
	_, stage := mustStep(t, "apply-github", a.Steps, "stage")
	wantRun(t, "apply-github stage", stage, `cp -R "$RUNNER_TEMP/idp/in/rendered" "$RUNNER_TEMP/idp/work/rendered"`)
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/workflowspec/ -run 'TestReconcile|TestApprov|TestApply|TestWet|TestRender'`
Expected: FAIL with `read .github/workflows/reconcile.yaml: open …`.

- [ ] **Step 3: Write `.github/workflows/reconcile.yaml`**

```yaml
name: idp-reconcile

# Reusable reconcile pipeline (spec §6.2): re-render and diff against the current
# wet, plan, gate, wait for approval when the gate asks for it, apply the saved
# plan, and commit the state and the render to wet in one commit (ADR-0019).
# A run is never re-run; a failed run is retried with a new dispatch (ADR-0024).

on:
  workflow_call:
    secrets:
      IDP_READER_PRIVATE_KEY:
        description: The reader App's private key (repo secret).
        required: false
      IDP_STATE_PASSPHRASE:
        description: The OpenTofu state passphrase (repo secret).
        required: false
      IDP_WRITER_PRIVATE_KEY:
        description: Never pass it. The apply job reads it from the idp-write environment, which takes precedence (ADR-0023).
        required: false

permissions: {}

defaults:
  run:
    shell: bash

jobs:
  plan:
    name: plan
    runs-on: ubuntu-24.04
    timeout-minutes: 30
    permissions:
      contents: read
      issues: read
      pull-requests: read
    outputs:
      wet-sha: ${{ steps.wet.outputs.sha }}
      github: ${{ steps.stacks.outputs.github }}
      changes: ${{ steps.summary.outputs.changes }}
      decision: ${{ steps.gate.outputs.decision }}
    steps:
      - name: reconcile main, once
        id: guard
        env:
          REF: ${{ github.ref }}
        run: |
          if [ "$REF" != refs/heads/main ]; then
            echo "::error::the reconcile runs only on main"
            exit 1
          fi
          if [ "$GITHUB_RUN_ATTEMPT" != 1 ]; then
            echo "::error::a reconcile run is never re-run, because its plan may be stale: dispatch the reconcile workflow instead (docs/runbooks/pipelines.md)"
            exit 1
          fi
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          path: .idp/engine
          persist-credentials: false
      - uses: ./.idp/engine/.github/actions/setup-idp # zizmor: ignore[self-repository] the engine is checked out at job.workflow_sha in the step above (ADR-0021)
        with:
          engine-dir: .idp/engine
          version: ${{ job.workflow_sha }}
      - name: check the repository configuration
        id: config
        env:
          READER_CLIENT_ID: ${{ vars.IDP_READER_CLIENT_ID }}
          READER_KEY_SET: ${{ secrets.IDP_READER_PRIVATE_KEY != '' }}
          PASSPHRASE_SET: ${{ secrets.IDP_STATE_PASSPHRASE != '' }}
        run: |
          missing=""
          [ -n "$READER_CLIENT_ID" ] || missing="$missing variable IDP_READER_CLIENT_ID,"
          [ "$READER_KEY_SET" = true ] || missing="$missing secret IDP_READER_PRIVATE_KEY,"
          [ "$PASSPHRASE_SET" = true ] || missing="$missing secret IDP_STATE_PASSPHRASE,"
          if [ -n "$missing" ]; then
            echo "::error::the claims repo is missing:${missing%,}. Re-run the bootstrap (docs/runbooks/bootstrap.md) and pass the secrets from the caller workflow."
            exit 1
          fi
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: wet
          path: .idp/wet
          persist-credentials: false
      - name: record the wet commit
        id: wet
        run: echo "sha=$(git -C .idp/wet rev-parse HEAD)" >> "$GITHUB_OUTPUT"
      - name: render the claims
        id: render
        run: |
          idp render --out "$RUNNER_TEMP/idp/out/rendered"
          mkdir -p "$RUNNER_TEMP/idp/work"
          cp -R "$RUNNER_TEMP/idp/out/rendered" "$RUNNER_TEMP/idp/work/rendered"
      - name: diff against wet
        id: diff
        env:
          EVENT: ${{ github.event_name }}
        run: |
          all=()
          if [ "$EVENT" = workflow_dispatch ]; then all=(--all); fi
          idp diff --new "$RUNNER_TEMP/idp/out/rendered" --wet .idp/wet/rendered "${all[@]}"
      - name: check the affected stacks
        id: stacks
        env:
          AFFECTED: ${{ steps.diff.outputs.affected }}
        run: |
          if ! jq -e 'all(.[]; . == "github")' <<<"$AFFECTED" >/dev/null; then
            echo "::error::this engine version reconciles only the github stack; affected: $AFFECTED"
            exit 1
          fi
          echo "github=$(jq -r 'any(.[]; . == "github")' <<<"$AFFECTED")" >> "$GITHUB_OUTPUT"
      - name: mint the reader token
        id: reader
        if: steps.stacks.outputs.github == 'true'
        uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0 # zizmor: ignore[github-app] the read-only reader App plans every managed repo (ADR-0023)
        with:
          client-id: ${{ vars.IDP_READER_CLIENT_ID }}
          private-key: ${{ secrets.IDP_READER_PRIVATE_KEY }}
          owner: ${{ github.repository_owner }}
      - name: export TF_ENCRYPTION
        id: encryption
        if: steps.stacks.outputs.github == 'true'
        env:
          IDP_STATE_PASSPHRASE: ${{ secrets.IDP_STATE_PASSPHRASE }}
        run: idp encryption-env
      - name: plan the github stack
        id: plan
        if: steps.stacks.outputs.github == 'true'
        working-directory: ${{ runner.temp }}/idp/work/rendered/github
        env:
          GITHUB_TOKEN: ${{ steps.reader.outputs.token }}
        run: |
          mkdir -p "$RUNNER_TEMP/idp/work/tfstate" "$RUNNER_TEMP/idp/plans"
          if [ -f "$GITHUB_WORKSPACE/.idp/wet/tfstate/github.tfstate" ]; then
            cp "$GITHUB_WORKSPACE/.idp/wet/tfstate/github.tfstate" "$RUNNER_TEMP/idp/work/tfstate/github.tfstate"
          fi
          tofu init -input=false -lockfile=readonly
          tofu plan -input=false -lock=false -out="$RUNNER_TEMP/idp/out/github.tfplan"
          tofu show -json "$RUNNER_TEMP/idp/out/github.tfplan" > "$RUNNER_TEMP/idp/plans/github.json"
      - name: summarize the plan
        id: summary
        env:
          AFFECTED: ${{ steps.diff.outputs.affected }}
          GITHUB_STACK: ${{ steps.stacks.outputs.github }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
        run: |
          plans=()
          if [ "$GITHUB_STACK" = true ]; then plans+=(--plan "github=$RUNNER_TEMP/idp/plans/github.json"); fi
          idp plan-summary --stacks "$AFFECTED" --head-sha "$GITHUB_SHA" --run-url "$RUN_URL" \
            --comment-out "$RUNNER_TEMP/idp/out/comment.md" \
            --fingerprint-out "$RUNNER_TEMP/idp/out/fingerprint.json" "${plans[@]}"
          cat "$RUNNER_TEMP/idp/out/comment.md" >> "$GITHUB_STEP_SUMMARY"
      - name: gate
        id: gate
        env:
          GITHUB_TOKEN: ${{ github.token }}
        run: idp gate --repo "$GITHUB_REPOSITORY" --sha "$GITHUB_SHA" --fingerprint "$RUNNER_TEMP/idp/out/fingerprint.json"
      - name: keep the plan and the render for the apply
        id: upload
        if: steps.stacks.outputs.github == 'true'
        uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1
        with:
          name: idp-reconcile
          path: ${{ runner.temp }}/idp/out
          include-hidden-files: true
          retention-days: 1
          if-no-files-found: error

  approve:
    name: approve
    needs: plan
    if: needs.plan.outputs.decision == 'approval'
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    environment: idp-approval
    permissions: {}
    steps:
      - name: record the approval
        id: approved
        run: echo "approved in the idp-approval environment; the apply continues"

  apply-github:
    name: apply-github
    needs: [plan, approve]
    if: always() && needs.plan.result == 'success' && needs.plan.outputs.github == 'true' && (needs.plan.outputs.decision == 'auto' || needs.approve.result == 'success')
    runs-on: ubuntu-24.04
    timeout-minutes: 50
    environment: idp-write
    permissions:
      contents: read
      issues: write
    steps:
      - name: apply once, with the idp-write credentials
        id: guard
        env:
          WRITER_CLIENT_ID: ${{ vars.IDP_WRITER_CLIENT_ID }}
          WRITER_KEY_SET: ${{ secrets.IDP_WRITER_PRIVATE_KEY != '' }}
          PASSPHRASE_SET: ${{ secrets.IDP_STATE_PASSPHRASE != '' }}
        run: |
          if [ "$GITHUB_RUN_ATTEMPT" != 1 ]; then
            echo "::error::a reconcile run is never re-run, because its plan may be stale: dispatch the reconcile workflow instead (docs/runbooks/pipelines.md)"
            exit 1
          fi
          missing=""
          [ -n "$WRITER_CLIENT_ID" ] || missing="$missing environment variable IDP_WRITER_CLIENT_ID,"
          [ "$WRITER_KEY_SET" = true ] || missing="$missing environment secret IDP_WRITER_PRIVATE_KEY,"
          [ "$PASSPHRASE_SET" = true ] || missing="$missing secret IDP_STATE_PASSPHRASE,"
          if [ -n "$missing" ]; then
            echo "::error::the apply job is missing:${missing%,}. Re-run the bootstrap (docs/runbooks/bootstrap.md)."
            exit 1
          fi
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          path: .idp/engine
          persist-credentials: false
      - uses: ./.idp/engine/.github/actions/setup-idp # zizmor: ignore[self-repository] the engine is checked out at job.workflow_sha in the step above (ADR-0021)
        with:
          engine-dir: .idp/engine
          version: ${{ job.workflow_sha }}
      - name: check out wet
        id: wet-checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: wet
          path: .idp/wet
          persist-credentials: false
      - name: refuse when wet moved since the plan
        id: wet-base
        env:
          PLANNED: ${{ needs.plan.outputs.wet-sha }}
        run: |
          if [ "$(git -C .idp/wet rev-parse HEAD)" != "$PLANNED" ]; then
            echo "::error::wet moved since this run's plan, so the saved plan may be stale: dispatch the reconcile workflow instead"
            exit 1
          fi
      - name: fetch the saved plan and render
        id: download
        uses: actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c # v8.0.1
        with:
          name: idp-reconcile
          path: ${{ runner.temp }}/idp/in
      - name: export TF_ENCRYPTION
        id: encryption
        env:
          IDP_STATE_PASSPHRASE: ${{ secrets.IDP_STATE_PASSPHRASE }}
        run: idp encryption-env
      - name: stage the saved render and the state the plan read
        id: stage
        run: |
          mkdir -p "$RUNNER_TEMP/idp/work/tfstate"
          cp -R "$RUNNER_TEMP/idp/in/rendered" "$RUNNER_TEMP/idp/work/rendered"
          if [ -f .idp/wet/tfstate/github.tfstate ]; then
            cp .idp/wet/tfstate/github.tfstate "$RUNNER_TEMP/idp/work/tfstate/github.tfstate"
          fi
      - name: mint the writer token for the apply
        id: apply-token
        if: needs.plan.outputs.changes != '0'
        uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0
        with:
          client-id: ${{ vars.IDP_WRITER_CLIENT_ID }}
          private-key: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
          owner: ${{ github.repository_owner }} # zizmor: ignore[github-app] the apply manages every repository the claims declare (ADR-0023)
          permission-administration: write
          permission-contents: write
          permission-environments: write
          permission-members: write
          permission-workflows: write
      - name: apply the saved plan
        id: apply
        if: needs.plan.outputs.changes != '0'
        working-directory: ${{ runner.temp }}/idp/work/rendered/github
        env:
          GITHUB_TOKEN: ${{ steps.apply-token.outputs.token }}
        run: |
          tofu init -input=false -lockfile=readonly
          tofu apply -input=false "$RUNNER_TEMP/idp/in/github.tfplan"
      - name: prepare the wet commit
        id: prepare
        if: always() && steps.stage.outcome == 'success'
        env:
          CHANGES: ${{ needs.plan.outputs.changes }}
          APPLY_OUTCOME: ${{ steps.apply.outcome }}
          WET_SHA: ${{ needs.plan.outputs.wet-sha }}
        run: |
          root="$RUNNER_TEMP/idp/wet-root"
          mkdir -p "$root/tfstate"
          echo "$WET_SHA" > "$root/wet-base"
          if [ -f "$RUNNER_TEMP/idp/work/tfstate/github.tfstate" ]; then
            cp "$RUNNER_TEMP/idp/work/tfstate/github.tfstate" "$root/tfstate/github.tfstate"
          fi
          # The state is committed always: real resources may have changed. The render
          # only when the apply succeeded or had nothing to apply; a failed stack keeps
          # its old render in wet, so the next reconcile retries it (spec §6.2).
          if [ "$CHANGES" = 0 ] || [ "$APPLY_OUTCOME" = success ]; then
            mkdir -p "$root/rendered"
            cp -R "$RUNNER_TEMP/idp/in/rendered/github" "$root/rendered/github"
            echo "render=true" >> "$GITHUB_OUTPUT"
          else
            echo "::warning::the apply did not succeed: only the state is committed, and the old render stays in wet so the next reconcile retries"
            echo "render=false" >> "$GITHUB_OUTPUT"
          fi
      - name: keep the wet commit as an artifact
        id: keep
        if: always() && steps.prepare.outcome == 'success'
        uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1
        with:
          name: idp-wet-root
          path: ${{ runner.temp }}/idp/wet-root
          include-hidden-files: true
          retention-days: 7
          if-no-files-found: error
      - name: mint the wet-commit token
        id: push-token
        if: always() && steps.prepare.outcome == 'success'
        uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0
        with:
          client-id: ${{ vars.IDP_WRITER_CLIENT_ID }}
          private-key: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
          owner: ${{ github.repository_owner }}
          repositories: ${{ github.event.repository.name }}
          permission-contents: write
      - name: commit to wet
        id: push
        if: always() && steps.push-token.outcome == 'success'
        env:
          GH_TOKEN: ${{ steps.push-token.outputs.token }}
          RENDER: ${{ steps.prepare.outputs.render }}
        run: |
          paths=(--path tfstate/github.tfstate)
          if [ "$RENDER" = true ]; then paths+=(--path rendered/github); fi
          idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root" \
            --message "idp: reconcile $GITHUB_SHA (run $GITHUB_RUN_ID)" "${paths[@]}"
      - name: open the wet-push issue
        id: issue
        if: always() && steps.prepare.outcome == 'success' && steps.push.outcome != 'success'
        env:
          GITHUB_TOKEN: ${{ github.token }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
          RUNBOOK_URL: ${{ github.server_url }}/${{ job.workflow_repository }}/blob/${{ job.workflow_sha }}/docs/runbooks/pipelines.md#a-wet-push-failed
        run: |
          body="$RUNNER_TEMP/idp/wet-push-issue.md"
          {
            echo "The reconcile run $RUN_URL could not commit to the wet branch."
            echo
            echo "What it was about to commit is saved as the artifact idp-wet-root of that run, kept for 7 days."
            echo "Run the idp-recover workflow with run-id $GITHUB_RUN_ID, following $RUNBOOK_URL"
          } > "$body"
          idp issue open --repo "$GITHUB_REPOSITORY" --label idp-wet-push --title "Wet push failed" --body-file "$body"
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/workflowspec/`
Expected: PASS.

- [ ] **Step 5: Run the linters**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`
Expected: no output.

Run: `"$ZIZMOR" --offline .github/workflows .github/actions/setup-idp/action.yaml`
Expected: no unsuppressed findings. The `github-app` ignore on the apply token sits on the `owner:` line, which is that finding's primary location; if zizmor still reports it, move the comment to the `uses:` line after the version comment (`# v3.2.0 # zizmor: ignore[github-app] …`) and say so in the report.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/reconcile.yaml internal/workflowspec/reconcile_test.go
git commit -m "feat(workflows): add the reusable reconcile pipeline"
```

---

### Task 6: The reusable drift and recover pipelines

**Files:**
- Create: `.github/workflows/drift.yaml`, `.github/workflows/recover.yaml`, `internal/workflowspec/drift_test.go`, `internal/workflowspec/recover_test.go`

**Interfaces:**
- Consumes: Task 2 helpers; Task 5's `checkPushToken` and the `idp-wet-root` layout (`wet-base`, `tfstate/github.tfstate`, optional `rendered/github/`); CLI: `encryption-env`, `plan-summary`, `bootstrap check --org --claims-repo --params-env IDP_BOOTSTRAP --allow-hidden-bypass` (exit 0 / 3 / other), `issue open|close`, `wet-push`.
- Produces: `drift.yaml` (`workflow_call`, optional secrets `IDP_READER_PRIVATE_KEY`, `IDP_STATE_PASSPHRASE`; caller grants `contents: read`, `issues: write`) and `recover.yaml` (`workflow_call` with required string input `run-id`, optional secret `IDP_WRITER_PRIVATE_KEY`; caller grants `actions: read`, `contents: read`, `issues: write`).

- [ ] **Step 1: Write the failing tests**

`internal/workflowspec/drift_test.go`:

```go
package workflowspec

import (
	"maps"
	"slices"
	"testing"
)

const driftPath = ".github/workflows/drift.yaml"

func TestDriftInterface(t *testing.T) {
	w := loadWorkflow(t, driftPath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_call"}) {
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	wantOptionalSecrets(t, driftPath, w.call(t), "IDP_READER_PRIVATE_KEY", "IDP_STATE_PASSPHRASE")
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"drift"}) {
		t.Errorf("jobs = %v", got)
	}
	d := mustJob(t, w, "drift")
	wantPerms(t, "drift", d.Permissions, map[string]string{"contents": "read", "issues": "write"})
	if d.Environment != "" {
		t.Errorf("drift must not use an environment: it only reads")
	}
}

func TestDriftPlansTheWetRenderWithTheReaderToken(t *testing.T) {
	w := loadWorkflow(t, driftPath)
	d := mustJob(t, w, "drift")
	_, p := mustStep(t, "drift", d.Steps, "plan")
	if want := "steps.wet.outputs.stack == 'true'"; p.If != want {
		t.Errorf("plan if = %q, want %q", p.If, want)
	}
	if p.WorkingDirectory != ".idp/wet/rendered/github" {
		t.Errorf("plan working-directory = %q, want the wet render", p.WorkingDirectory)
	}
	wantEnv(t, "drift plan", p, map[string]string{"GITHUB_TOKEN": "${{ steps.reader.outputs.token }}"})
	wantRun(t, "drift plan", p, "-detailed-exitcode", `2) echo "drift=true" >> "$GITHUB_OUTPUT" ;;`)
}

func TestDriftChecksTheBootstrapWithTheRecordedIdentity(t *testing.T) {
	w := loadWorkflow(t, driftPath)
	_, c := mustStep(t, "drift", mustJob(t, w, "drift").Steps, "check")
	wantEnv(t, "drift check", c, map[string]string{
		"GH_TOKEN":      "${{ steps.reader.outputs.token }}",
		"IDP_BOOTSTRAP": "${{ vars.IDP_BOOTSTRAP }}",
	})
	wantRun(t, "drift check", c, "--params-env IDP_BOOTSTRAP --allow-hidden-bypass", `3) echo "drift=true" >> "$GITHUB_OUTPUT" ;;`)
}

func TestDriftReportsThroughOneBoundedIssue(t *testing.T) {
	w := loadWorkflow(t, driftPath)
	_, r := mustStep(t, "drift", mustJob(t, w, "drift").Steps, "report")
	wantEnv(t, "drift report", r, map[string]string{
		"PLAN_DRIFT":  "${{ steps.plan.outputs.drift }}",
		"CHECK_DRIFT": "${{ steps.check.outputs.drift }}",
	})
	wantRun(t, "drift report", r,
		`idp issue open --repo "$GITHUB_REPOSITORY" --label drift --title "Drift detected"`,
		`idp issue close --repo "$GITHUB_REPOSITORY" --label drift --title "Drift detected"`,
		`head -c 4000 "$RUNNER_TEMP/idp/out/bootstrap.txt"`,
	)
}
```

`internal/workflowspec/recover_test.go`:

```go
package workflowspec

import (
	"maps"
	"slices"
	"testing"
)

const recoverPath = ".github/workflows/recover.yaml"

func TestRecoverInterface(t *testing.T) {
	w := loadWorkflow(t, recoverPath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_call"}) {
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	c := w.call(t)
	if in, ok := c.Inputs["run-id"]; !ok || !in.Required || in.Type != "string" || len(c.Inputs) != 1 {
		t.Errorf("inputs = %v, want exactly a required string run-id", c.Inputs)
	}
	wantOptionalSecrets(t, recoverPath, c, "IDP_WRITER_PRIVATE_KEY")
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"recover"}) {
		t.Errorf("jobs = %v", got)
	}
	r := mustJob(t, w, "recover")
	if r.Environment != "idp-write" {
		t.Errorf("recover environment = %q, want idp-write", r.Environment)
	}
	wantPerms(t, "recover", r.Permissions, map[string]string{"actions": "read", "contents": "read", "issues": "write"})
}

func TestRecoverChecksTheRunAndTheWetBase(t *testing.T) {
	w := loadWorkflow(t, recoverPath)
	r := mustJob(t, w, "recover")
	iReq, req := mustStep(t, "recover", r.Steps, "request")
	wantEnv(t, "recover request", req, map[string]string{"REF": "${{ github.ref }}", "RUN_ID": "${{ inputs.run-id }}"})
	wantRun(t, "recover request", req, `if [ "$REF" != refs/heads/main ]; then`, `if ! [[ "$RUN_ID" =~ ^[0-9]+$ ]]; then`)
	iRun, run := mustStep(t, "recover", r.Steps, "run-check")
	wantEnv(t, "recover run-check", run, map[string]string{"GH_TOKEN": "${{ github.token }}", "RUN_ID": "${{ inputs.run-id }}"})
	wantRun(t, "recover run-check", run,
		`[ "$branch" != main ]`,
		`[ "$event" != push ] && [ "$event" != workflow_dispatch ]`,
		`[ "$conclusion" != failure ]`,
	)
	iDL, dl := mustStep(t, "recover", r.Steps, "download")
	wantWith(t, "recover download", dl, map[string]string{
		"name":         "idp-wet-root",
		"run-id":       "${{ inputs.run-id }}",
		"github-token": "${{ github.token }}",
		"path":         "${{ runner.temp }}/idp/wet-root",
	})
	iBase, base := mustStep(t, "recover", r.Steps, "base")
	wantRun(t, "recover base", base, `[[ "$base" =~ ^[0-9a-f]{40}$ ]]`, `if [ "$base" != "$head" ]; then`)
	_, tok := mustStep(t, "recover", r.Steps, "push-token")
	checkPushToken(t, "recover push-token", tok)
	iPush, push := mustStep(t, "recover", r.Steps, "push")
	wantEnv(t, "recover push", push, map[string]string{
		"GH_TOKEN": "${{ steps.push-token.outputs.token }}",
		"RENDER":   "${{ steps.base.outputs.render }}",
	})
	wantRun(t, "recover push", push, `paths=(--path tfstate/github.tfstate)`, `idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root"`)
	if !(iReq < iRun && iRun < iDL && iDL < iBase && iBase < iPush) {
		t.Errorf("recover must check the request, then the run, download, compare the base, and only then push (got %d %d %d %d %d)", iReq, iRun, iDL, iBase, iPush)
	}
	_, cl := mustStep(t, "recover", r.Steps, "close")
	wantRun(t, "recover close", cl, `idp issue close --repo "$GITHUB_REPOSITORY" --label idp-wet-push --title "Wet push failed"`)
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/ -run 'TestDrift|TestRecover'`
Expected: FAIL with `read .github/workflows/drift.yaml: open …` and the same for `recover.yaml`.

- [ ] **Step 3: Write `.github/workflows/drift.yaml`**

````yaml
name: idp-drift

# Reusable drift check (spec §6.4): plan the wet render and check the bootstrap
# protections, both with the reader App, and keep one drift issue open while
# anything drifted. There is no auto-remediation. The run fails only when the
# check cannot run (spec amendment A6).

on:
  workflow_call:
    secrets:
      IDP_READER_PRIVATE_KEY:
        description: The reader App's private key (repo secret).
        required: false
      IDP_STATE_PASSPHRASE:
        description: The OpenTofu state passphrase (repo secret).
        required: false

permissions: {}

defaults:
  run:
    shell: bash

jobs:
  drift:
    name: drift
    runs-on: ubuntu-24.04
    timeout-minutes: 30
    permissions:
      contents: read
      issues: write
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          path: .idp/engine
          persist-credentials: false
      - uses: ./.idp/engine/.github/actions/setup-idp # zizmor: ignore[self-repository] the engine is checked out at job.workflow_sha in the step above (ADR-0021)
        with:
          engine-dir: .idp/engine
          version: ${{ job.workflow_sha }}
      - name: check the repository configuration
        id: config
        env:
          READER_CLIENT_ID: ${{ vars.IDP_READER_CLIENT_ID }}
          BOOTSTRAP: ${{ vars.IDP_BOOTSTRAP }}
          READER_KEY_SET: ${{ secrets.IDP_READER_PRIVATE_KEY != '' }}
          PASSPHRASE_SET: ${{ secrets.IDP_STATE_PASSPHRASE != '' }}
        run: |
          missing=""
          [ -n "$READER_CLIENT_ID" ] || missing="$missing variable IDP_READER_CLIENT_ID,"
          [ -n "$BOOTSTRAP" ] || missing="$missing variable IDP_BOOTSTRAP,"
          [ "$READER_KEY_SET" = true ] || missing="$missing secret IDP_READER_PRIVATE_KEY,"
          [ "$PASSPHRASE_SET" = true ] || missing="$missing secret IDP_STATE_PASSPHRASE,"
          if [ -n "$missing" ]; then
            echo "::error::the claims repo is missing:${missing%,}. Re-run the bootstrap (docs/runbooks/bootstrap.md) and pass the secrets from the caller workflow."
            exit 1
          fi
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: wet
          path: .idp/wet
          persist-credentials: false
      - name: inspect wet
        id: wet
        run: |
          echo "sha=$(git -C .idp/wet rev-parse HEAD)" >> "$GITHUB_OUTPUT"
          if [ -f .idp/wet/rendered/github/main.tf.json ]; then
            echo "stack=true" >> "$GITHUB_OUTPUT"
          else
            echo "::notice::wet has no github stack yet: only the bootstrap protections are checked"
            echo "stack=false" >> "$GITHUB_OUTPUT"
          fi
      - name: mint the reader token
        id: reader
        uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0 # zizmor: ignore[github-app] the read-only reader App reads every managed repo and the org settings (ADR-0023)
        with:
          client-id: ${{ vars.IDP_READER_CLIENT_ID }}
          private-key: ${{ secrets.IDP_READER_PRIVATE_KEY }}
          owner: ${{ github.repository_owner }}
      - name: export TF_ENCRYPTION
        id: encryption
        if: steps.wet.outputs.stack == 'true'
        env:
          IDP_STATE_PASSPHRASE: ${{ secrets.IDP_STATE_PASSPHRASE }}
        run: idp encryption-env
      - name: plan the github stack from wet
        id: plan
        if: steps.wet.outputs.stack == 'true'
        working-directory: .idp/wet/rendered/github
        env:
          GITHUB_TOKEN: ${{ steps.reader.outputs.token }}
        run: |
          mkdir -p "$RUNNER_TEMP/idp/plans"
          tofu init -input=false -lockfile=readonly
          code=0
          tofu plan -input=false -lock=false -detailed-exitcode -out="$RUNNER_TEMP/idp/plans/github.tfplan" || code=$?
          case "$code" in
            0) echo "drift=false" >> "$GITHUB_OUTPUT" ;;
            2) echo "drift=true" >> "$GITHUB_OUTPUT" ;;
            *) echo "::error::the drift plan failed (exit $code)"; exit 1 ;;
          esac
          tofu show -json "$RUNNER_TEMP/idp/plans/github.tfplan" > "$RUNNER_TEMP/idp/plans/github.json"
      - name: summarize the drift plan
        id: summary
        if: steps.wet.outputs.stack == 'true'
        env:
          WET_SHA: ${{ steps.wet.outputs.sha }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
        run: |
          mkdir -p "$RUNNER_TEMP/idp/out"
          idp plan-summary --stacks '["github"]' --head-sha "$WET_SHA" --run-url "$RUN_URL" \
            --comment-out "$RUNNER_TEMP/idp/out/plan.md" \
            --fingerprint-out "$RUNNER_TEMP/idp/out/fingerprint.json" \
            --plan "github=$RUNNER_TEMP/idp/plans/github.json"
      - name: check the bootstrap protections
        id: check
        env:
          GH_TOKEN: ${{ steps.reader.outputs.token }}
          IDP_BOOTSTRAP: ${{ vars.IDP_BOOTSTRAP }}
        run: |
          mkdir -p "$RUNNER_TEMP/idp/out"
          code=0
          idp bootstrap check --org "$GITHUB_REPOSITORY_OWNER" --claims-repo "${GITHUB_REPOSITORY#*/}" \
            --params-env IDP_BOOTSTRAP --allow-hidden-bypass > "$RUNNER_TEMP/idp/out/bootstrap.txt" 2>&1 || code=$?
          cat "$RUNNER_TEMP/idp/out/bootstrap.txt"
          case "$code" in
            0) echo "drift=false" >> "$GITHUB_OUTPUT" ;;
            3) echo "drift=true" >> "$GITHUB_OUTPUT" ;;
            *) echo "::error::idp bootstrap check could not run (exit $code)"; exit 1 ;;
          esac
      - name: report
        id: report
        env:
          GITHUB_TOKEN: ${{ github.token }}
          PLAN_DRIFT: ${{ steps.plan.outputs.drift }}
          CHECK_DRIFT: ${{ steps.check.outputs.drift }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
        run: |
          if [ "$PLAN_DRIFT" != true ] && [ "$CHECK_DRIFT" != true ]; then
            idp issue close --repo "$GITHUB_REPOSITORY" --label drift --title "Drift detected" --comment "No drift found by $RUN_URL."
            exit 0
          fi
          body="$RUNNER_TEMP/idp/out/drift-issue.md"
          {
            echo "The drift check $RUN_URL found differences. Fix the claims, or run the reconcile workflow; nothing is remediated automatically."
            echo
            if [ "$PLAN_DRIFT" = true ]; then
              echo "## GitHub stack"
              echo
              cat "$RUNNER_TEMP/idp/out/plan.md"
              echo
            fi
            if [ "$CHECK_DRIFT" = true ]; then
              echo "## Bootstrap protections"
              echo
              echo '```text'
              head -c 4000 "$RUNNER_TEMP/idp/out/bootstrap.txt"
              echo
              echo '```'
            fi
          } > "$body"
          idp issue open --repo "$GITHUB_REPOSITORY" --label drift --title "Drift detected" --body-file "$body"
          echo "::warning::drift found: see the open drift issue"
````

The issue body stays under GitHub's 65536-character limit: the plan comment is bounded to 60000 bytes by `plan-summary`, and the bootstrap findings to 4000. No `echo` in these scripts may contain the words `tofu init`, `tofu plan` or `tofu apply`: `checkTofu` reads every line that does as a command.

- [ ] **Step 4: Write `.github/workflows/recover.yaml`**

```yaml
name: idp-recover

# Reusable recovery for a failed wet push (ADR-0024): commit the tree a failed
# reconcile run saved as its idp-wet-root artifact, only while wet still points
# at the commit that run's plan read.

on:
  workflow_call:
    inputs:
      run-id:
        description: The id of the reconcile run whose wet push failed.
        required: true
        type: string
    secrets:
      IDP_WRITER_PRIVATE_KEY:
        description: Never pass it. The job reads it from the idp-write environment, which takes precedence (ADR-0023).
        required: false

permissions: {}

defaults:
  run:
    shell: bash

jobs:
  recover:
    name: recover
    runs-on: ubuntu-24.04
    timeout-minutes: 15
    environment: idp-write
    permissions:
      actions: read
      contents: read
      issues: write
    steps:
      - name: check the request
        id: request
        env:
          REF: ${{ github.ref }}
          RUN_ID: ${{ inputs.run-id }}
          WRITER_CLIENT_ID: ${{ vars.IDP_WRITER_CLIENT_ID }}
          WRITER_KEY_SET: ${{ secrets.IDP_WRITER_PRIVATE_KEY != '' }}
        run: |
          if [ "$REF" != refs/heads/main ]; then
            echo "::error::the recovery runs only on main"
            exit 1
          fi
          if ! [[ "$RUN_ID" =~ ^[0-9]+$ ]]; then
            echo "::error::run-id must be the numeric id of a reconcile run"
            exit 1
          fi
          if [ -z "$WRITER_CLIENT_ID" ] || [ "$WRITER_KEY_SET" != true ]; then
            echo "::error::the idp-write environment must hold the variable IDP_WRITER_CLIENT_ID and the secret IDP_WRITER_PRIVATE_KEY. Re-run the bootstrap (docs/runbooks/bootstrap.md)."
            exit 1
          fi
      - name: check that the run is a failed reconcile of main
        id: run-check
        env:
          GH_TOKEN: ${{ github.token }}
          RUN_ID: ${{ inputs.run-id }}
        run: |
          read -r event branch conclusion < <(gh api "repos/$GITHUB_REPOSITORY/actions/runs/$RUN_ID" --jq '[.event, .head_branch, (.conclusion // "")] | @tsv')
          if [ "$branch" != main ] || { [ "$event" != push ] && [ "$event" != workflow_dispatch ]; }; then
            echo "::error::run $RUN_ID is not a reconcile of main (event: $event, branch: $branch)"
            exit 1
          fi
          if [ "$conclusion" != failure ]; then
            echo "::error::run $RUN_ID did not fail (conclusion: $conclusion): there is nothing to recover"
            exit 1
          fi
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          path: .idp/engine
          persist-credentials: false
      - uses: ./.idp/engine/.github/actions/setup-idp # zizmor: ignore[self-repository] the engine is checked out at job.workflow_sha in the step above (ADR-0021)
        with:
          engine-dir: .idp/engine
          version: ${{ job.workflow_sha }}
      - name: fetch the saved wet commit
        id: download
        uses: actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c # v8.0.1
        with:
          name: idp-wet-root
          run-id: ${{ inputs.run-id }}
          github-token: ${{ github.token }}
          path: ${{ runner.temp }}/idp/wet-root
      - name: check out wet
        id: wet-checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: wet
          path: .idp/wet
          persist-credentials: false
      - name: refuse when wet moved since that run
        id: base
        run: |
          root="$RUNNER_TEMP/idp/wet-root"
          base="$(tr -d '[:space:]' < "$root/wet-base")"
          head="$(git -C .idp/wet rev-parse HEAD)"
          if ! [[ "$base" =~ ^[0-9a-f]{40}$ ]]; then
            echo "::error::the artifact has no valid wet-base"
            exit 1
          fi
          if [ "$base" != "$head" ]; then
            echo "::error::wet moved since that run read it (then $base, now $head), so its saved state may be older than wet's. Follow the runbook instead (docs/runbooks/pipelines.md)."
            exit 1
          fi
          if [ -d "$root/rendered/github" ]; then
            echo "render=true" >> "$GITHUB_OUTPUT"
          else
            echo "render=false" >> "$GITHUB_OUTPUT"
          fi
      - name: mint the wet-commit token
        id: push-token
        uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0
        with:
          client-id: ${{ vars.IDP_WRITER_CLIENT_ID }}
          private-key: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
          owner: ${{ github.repository_owner }}
          repositories: ${{ github.event.repository.name }}
          permission-contents: write
      - name: commit to wet
        id: push
        env:
          GH_TOKEN: ${{ steps.push-token.outputs.token }}
          RENDER: ${{ steps.base.outputs.render }}
          RUN_ID: ${{ inputs.run-id }}
        run: |
          paths=(--path tfstate/github.tfstate)
          if [ "$RENDER" = true ]; then paths+=(--path rendered/github); fi
          idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root" \
            --message "idp: recover the wet commit of run $RUN_ID" "${paths[@]}"
      - name: close the wet-push issue
        id: close
        env:
          GITHUB_TOKEN: ${{ github.token }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
        run: idp issue close --repo "$GITHUB_REPOSITORY" --label idp-wet-push --title "Wet push failed" --comment "Recovered by $RUN_URL."
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/workflowspec/`
Expected: PASS.

- [ ] **Step 6: Run the linters**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`
Expected: no output.

Run: `"$ZIZMOR" --offline .github/workflows .github/actions/setup-idp/action.yaml`
Expected: no unsuppressed findings.

- [ ] **Step 7: Commit**

```bash
git add .github/workflows/drift.yaml .github/workflows/recover.yaml internal/workflowspec/drift_test.go internal/workflowspec/recover_test.go
git commit -m "feat(workflows): add the reusable drift and recover pipelines"
```

---

### Task 7: The claims repo caller templates

**Files:**
- Create: `examples/claims-repo/.github/workflows/pr.yaml`, `examples/claims-repo/.github/workflows/reconcile.yaml`, `examples/claims-repo/.github/workflows/drift.yaml`, `examples/claims-repo/.github/workflows/recover.yaml`, `internal/workflowspec/templates_test.go`
- Modify: `.github/workflows/ci.yaml` (lint the templates)

**Interfaces:**
- Consumes: Tasks 4–6 (each reusable workflow's `workflow_call` secrets and per-job permissions).
- Produces: the four caller files a claims repo copies, all pinned to one engine commit `<ENGINE_SHA>`; the `idp-gate` job; and the re-pin command Tasks 9–11 reuse.

`<ENGINE_SHA>` below is the output of `git rev-parse HEAD` at the start of this task: the commit that holds the final reusable workflows. Substitute it literally in all four files.

- [ ] **Step 1: Write the failing test, `internal/workflowspec/templates_test.go`**

```go
package workflowspec

import (
	"maps"
	"path"
	"regexp"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"
)

const templatesDir = "examples/claims-repo/.github/workflows/"

var templateNames = []string{"drift.yaml", "pr.yaml", "reconcile.yaml", "recover.yaml"}

var callRef = regexp.MustCompile(`^jellalshadows-idp/idp-engine/\.github/workflows/([a-z]+\.yaml)@([0-9a-f]{40})$`)

func TestTheReusableWorkflowSet(t *testing.T) {
	var reusable, templates []string
	for _, f := range reusableWorkflows(t) {
		reusable = append(reusable, path.Base(f))
	}
	for _, f := range glob(t, templatesDir+"*.yaml") {
		templates = append(templates, path.Base(f))
	}
	if !slices.Equal(reusable, templateNames) || !slices.Equal(templates, templateNames) {
		t.Errorf("reusable workflows %v and templates %v must both be %v", reusable, templates, templateNames)
	}
}

// caller returns the one job of a template that calls a reusable workflow.
func caller(t *testing.T, w *workflow) (string, *job) {
	t.Helper()
	var ids []string
	for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
		if w.Jobs[id].Uses != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) != 1 {
		t.Fatalf("%s: want exactly one job that calls a reusable workflow, got %v", w.path, ids)
	}
	return ids[0], w.Jobs[ids[0]]
}

func TestTemplatesCallTheirReusableWorkflowAtOneCommit(t *testing.T) {
	shas := map[string]bool{}
	for _, name := range templateNames {
		w := loadWorkflow(t, templatesDir+name)
		id, j := caller(t, w)
		m := callRef.FindStringSubmatch(j.Uses)
		if m == nil || m[1] != name {
			t.Errorf("%s: job %s must call jellalshadows-idp/idp-engine/.github/workflows/%s@<commit>, got %q", name, id, name, j.Uses)
			continue
		}
		shas[m[2]] = true
		if j.Secrets.Kind == yaml.ScalarNode {
			t.Errorf("%s: secrets: inherit hands every secret over; pass them one by one", name)
			continue
		}
		var passed strmap
		if j.Secrets.Kind != 0 {
			if err := j.Secrets.Decode(&passed); err != nil {
				t.Fatalf("%s: secrets: %v", name, err)
			}
		}
		want := map[string]string{}
		for s := range loadWorkflow(t, ".github/workflows/"+name).call(t).Secrets {
			if s != "IDP_WRITER_PRIVATE_KEY" { // lives only in the idp-write environment (ADR-0023)
				want[s] = "${{ secrets." + s + " }}"
			}
		}
		if !maps.Equal(map[string]string(passed), want) {
			t.Errorf("%s: secrets passed = %v, want %v", name, passed, want)
		}
	}
	if len(shas) != 1 {
		t.Errorf("the templates must pin one engine commit, got %v", slices.Sorted(maps.Keys(shas)))
	}
}

func TestTemplatesGrantExactlyWhatTheReusableJobsNeed(t *testing.T) {
	rank := map[string]int{"": 0, "none": 0, "read": 1, "write": 2}
	for _, name := range templateNames {
		need := map[string]string{}
		for _, j := range loadWorkflow(t, ".github/workflows/"+name).Jobs {
			for scope, level := range j.Permissions.levels {
				if rank[level] > rank[need[scope]] {
					need[scope] = level
				}
			}
		}
		id, j := caller(t, loadWorkflow(t, templatesDir+name))
		wantPerms(t, name+" job "+id, j.Permissions, need)
	}
}

func wantConcurrency(t *testing.T, w *workflow, want concurrency) {
	t.Helper()
	if w.Concurrency == nil || *w.Concurrency != want {
		t.Errorf("%s: concurrency = %+v, want %+v", w.path, w.Concurrency, want)
	}
}

var wetGroup = concurrency{Group: "idp-wet", CancelInProgress: false, Queue: "max"}

func TestPRTemplate(t *testing.T) {
	w := loadWorkflow(t, templatesDir+"pr.yaml")
	if got := w.triggerNames(); !slices.Equal(got, []string{"pull_request"}) {
		t.Errorf("triggers = %v, want only pull_request", got)
	}
	n, ok := w.trigger("pull_request")
	if !ok {
		t.Fatal("no pull_request trigger")
	}
	var pr struct {
		Branches    []string `yaml:"branches"`
		Paths       []string `yaml:"paths"`
		PathsIgnore []string `yaml:"paths-ignore"`
	}
	if err := n.Decode(&pr); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pr.Branches, []string{"main"}) {
		t.Errorf("pull_request branches = %v, want [main]", pr.Branches)
	}
	if len(pr.Paths)+len(pr.PathsIgnore) != 0 {
		t.Errorf("a path filter would leave idp-gate pending on a PR that touches no claims; the required check must always report (ADR-0022)")
	}
	wantConcurrency(t, w, concurrency{Group: "idp-plan-${{ github.event.pull_request.number }}", CancelInProgress: true})
}

func TestPRTemplateGateMirrorsTheResult(t *testing.T) {
	w := loadWorkflow(t, templatesDir+"pr.yaml")
	g := mustJob(t, w, "idp-gate")
	if g.Name != "idp-gate" {
		t.Errorf("the job name is the required check's name: want idp-gate, got %q", g.Name)
	}
	if !slices.Equal(g.Needs, strlist{"pr"}) {
		t.Errorf("idp-gate needs = %v, want [pr]", g.Needs)
	}
	if g.If != "always()" {
		t.Errorf("idp-gate if = %q, want always(): a skipped required check counts as passing (ADR-0022)", g.If)
	}
	wantPerms(t, "idp-gate", g.Permissions, map[string]string{})
	if len(g.Steps) != 1 {
		t.Fatalf("idp-gate must have exactly one step, got %d", len(g.Steps))
	}
	wantEnv(t, "idp-gate", g.Steps[0], map[string]string{"RESULT": "${{ needs.pr.result }}"})
	wantRun(t, "idp-gate", g.Steps[0], `test "$RESULT" = success`)
}

func TestReconcileTemplate(t *testing.T) {
	w := loadWorkflow(t, templatesDir+"reconcile.yaml")
	if got := w.triggerNames(); !slices.Equal(got, []string{"push", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want push and workflow_dispatch", got)
	}
	n, ok := w.trigger("push")
	if !ok {
		t.Fatal("no push trigger")
	}
	var push struct {
		Branches []string `yaml:"branches"`
	}
	if err := n.Decode(&push); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(push.Branches, []string{"main"}) {
		t.Errorf("push branches = %v, want [main]", push.Branches)
	}
	wantConcurrency(t, w, wetGroup)
}

func TestDriftTemplate(t *testing.T) {
	w := loadWorkflow(t, templatesDir+"drift.yaml")
	if got := w.triggerNames(); !slices.Equal(got, []string{"schedule", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want schedule and workflow_dispatch", got)
	}
	n, ok := w.trigger("schedule")
	if !ok {
		t.Fatal("no schedule trigger")
	}
	var crons []struct {
		Cron string `yaml:"cron"`
	}
	if err := n.Decode(&crons); err != nil {
		t.Fatal(err)
	}
	if len(crons) != 1 || crons[0].Cron != "23 5 * * *" {
		t.Errorf("schedule = %v, want one cron '23 5 * * *' (spec §6.4)", crons)
	}
	wantConcurrency(t, w, wetGroup)
}

func TestRecoverTemplate(t *testing.T) {
	w := loadWorkflow(t, templatesDir+"recover.yaml")
	if got := w.triggerNames(); !slices.Equal(got, []string{"workflow_dispatch"}) {
		t.Errorf("triggers = %v, want only workflow_dispatch", got)
	}
	n, ok := w.trigger("workflow_dispatch")
	if !ok {
		t.Fatal("no workflow_dispatch trigger")
	}
	var d struct {
		Inputs map[string]struct {
			Required bool   `yaml:"required"`
			Type     string `yaml:"type"`
		} `yaml:"inputs"`
	}
	if err := n.Decode(&d); err != nil {
		t.Fatal(err)
	}
	if in, ok := d.Inputs["run-id"]; !ok || !in.Required || in.Type != "string" || len(d.Inputs) != 1 {
		t.Errorf("inputs = %v, want exactly a required string run-id", d.Inputs)
	}
	_, j := caller(t, w)
	if !maps.Equal(map[string]string(j.With), map[string]string{"run-id": "${{ inputs.run-id }}"}) {
		t.Errorf("recover with = %v", j.With)
	}
	wantConcurrency(t, w, wetGroup)
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/workflowspec/ -run 'TestTheReusable|TestTemplates|Template'`
Expected: FAIL: `TestTheReusableWorkflowSet` reports the templates as `[]`, and the others fail to read `examples/claims-repo/.github/workflows/*.yaml`.

- [ ] **Step 3: Write the four templates**

`examples/claims-repo/.github/workflows/pr.yaml`:

```yaml
name: idp-pr

# Claims repo caller of the engine's PR pipeline (docs/pipelines.md). Keep the
# pinned engine commit identical in all four idp workflows. The idp-gate job is
# the check the main ruleset requires (ADR-0022): keep its name, and add no path
# filters, so every pull request reports it.

on:
  pull_request:
    branches: [main]

permissions: {}

concurrency:
  group: idp-plan-${{ github.event.pull_request.number }}
  cancel-in-progress: true

jobs:
  pr:
    uses: jellalshadows-idp/idp-engine/.github/workflows/pr.yaml@<ENGINE_SHA> # unreleased
    permissions:
      contents: read
      pull-requests: write
    secrets:
      IDP_READER_PRIVATE_KEY: ${{ secrets.IDP_READER_PRIVATE_KEY }}
      IDP_STATE_PASSPHRASE: ${{ secrets.IDP_STATE_PASSPHRASE }}

  idp-gate:
    name: idp-gate
    needs: pr
    if: always()
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    permissions: {}
    steps:
      - name: require the idp pipeline
        env:
          RESULT: ${{ needs.pr.result }}
        run: |
          echo "idp pipeline: $RESULT"
          test "$RESULT" = success
```

`examples/claims-repo/.github/workflows/reconcile.yaml`:

```yaml
name: idp-reconcile

# Claims repo caller of the engine's reconcile pipeline (docs/pipelines.md).
# A dispatch is the recovery button: it plans every stack against the current
# wet. Never re-run a reconcile run; dispatch a new one (ADR-0024).

on:
  push:
    branches: [main]
  workflow_dispatch:

permissions: {}

concurrency:
  group: idp-wet
  cancel-in-progress: false
  queue: max

jobs:
  reconcile:
    uses: jellalshadows-idp/idp-engine/.github/workflows/reconcile.yaml@<ENGINE_SHA> # unreleased
    permissions:
      contents: read
      issues: write
      pull-requests: read
    secrets:
      IDP_READER_PRIVATE_KEY: ${{ secrets.IDP_READER_PRIVATE_KEY }}
      IDP_STATE_PASSPHRASE: ${{ secrets.IDP_STATE_PASSPHRASE }}
```

`examples/claims-repo/.github/workflows/drift.yaml`:

```yaml
name: idp-drift

# Claims repo caller of the engine's drift check (docs/pipelines.md). GitHub
# disables schedules in public repos after 60 days without activity; re-enable
# with: gh workflow enable drift.yaml

on:
  schedule:
    - cron: '23 5 * * *'
  workflow_dispatch:

permissions: {}

concurrency:
  group: idp-wet
  cancel-in-progress: false
  queue: max

jobs:
  drift:
    uses: jellalshadows-idp/idp-engine/.github/workflows/drift.yaml@<ENGINE_SHA> # unreleased
    permissions:
      contents: read
      issues: write
    secrets:
      IDP_READER_PRIVATE_KEY: ${{ secrets.IDP_READER_PRIVATE_KEY }}
      IDP_STATE_PASSPHRASE: ${{ secrets.IDP_STATE_PASSPHRASE }}
```

`examples/claims-repo/.github/workflows/recover.yaml`:

```yaml
name: idp-recover

# Claims repo caller of the engine's wet push recovery (docs/runbooks/pipelines.md).
# Run it with the id of the reconcile run that opened the "Wet push failed" issue.

on:
  workflow_dispatch:
    inputs:
      run-id:
        description: The id of the reconcile run whose wet push failed (the number in its URL).
        required: true
        type: string

permissions: {}

concurrency:
  group: idp-wet
  cancel-in-progress: false
  queue: max

jobs:
  recover:
    uses: jellalshadows-idp/idp-engine/.github/workflows/recover.yaml@<ENGINE_SHA> # unreleased
    permissions:
      actions: read
      contents: read
      issues: write
    with:
      run-id: ${{ inputs.run-id }}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/workflowspec/`
Expected: PASS, including every common rule over the templates.

- [ ] **Step 5: Lint the templates in CI**

In `.github/workflows/ci.yaml`, job `workflows`, replace

```yaml
      - name: actionlint
        run: go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
      - name: zizmor
        run: pipx run zizmor==1.30.1 .github/workflows .github/actions/setup-idp/action.yaml
```

with

```yaml
      - name: actionlint
        run: go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
      - name: actionlint (claims repo templates)
        # concurrency.queue is a documented GitHub key (spec §6.2) that actionlint v1.7.12 does not know yet.
        run: go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -ignore 'unexpected key "queue" for "concurrency" section' examples/claims-repo/.github/workflows/*.yaml
      - name: zizmor
        run: pipx run zizmor==1.30.1 .github/workflows .github/actions/setup-idp/action.yaml examples/claims-repo/.github/workflows/*.yaml
```

- [ ] **Step 6: Run the linters**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 && go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -ignore 'unexpected key "queue" for "concurrency" section' examples/claims-repo/.github/workflows/*.yaml`
Expected: no output.

Run: `"$ZIZMOR" --offline .github/workflows .github/actions/setup-idp/action.yaml examples/claims-repo/.github/workflows/*.yaml`
Expected: no unsuppressed findings.

- [ ] **Step 7: Commit**

```bash
git add examples internal/workflowspec/templates_test.go
git commit -m "feat(examples): add the claims repo caller templates"
git add .github/workflows/ci.yaml
git commit -m "ci: lint the claims repo templates"
```

**The re-pin command** (used whenever a later commit changes a reusable workflow, the composite action, or anything `idp` is built from or renders: `cmd/`, `internal/`, `modules/`, `lockfiles/`). Run from the repo root, then commit `examples` with `chore(examples): pin the templates to <short sha>`:

```bash
python -I -c "import pathlib,re,sys; sha=sys.argv[1]; [p.write_text(re.sub(r'(idp-engine/\.github/workflows/[a-z]+\.yaml@)[0-9a-f]{40}', lambda m: m.group(1)+sha, p.read_text(encoding='utf-8')), encoding='utf-8', newline='\n') for p in pathlib.Path(sys.argv[2]).glob('*.yaml')]" "$(git rev-parse HEAD)" examples/claims-repo/.github/workflows
```

---

### Task 8: The pipelines guide, the runbook and the README

**Files:**
- Create: `docs/pipelines.md`, `docs/runbooks/pipelines.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: the names, permissions, artifacts and behaviours of Tasks 3–7.
- Produces: the runbook anchor `#a-wet-push-failed` that `reconcile.yaml`'s wet-push issue links.

- [ ] **Step 1: Write `docs/pipelines.md`**

````markdown
# Pipelines

A claims repo reconciles through four thin workflows that call the engine's reusable workflows (spec §6). This page shows how to onboard a claims repo and what each pipeline does. When something goes wrong, see the [pipelines runbook](runbooks/pipelines.md).

## Onboard a claims repo

1. Bootstrap the repo first ([bootstrap runbook](runbooks/bootstrap.md)). The pipelines need what `idp bootstrap apply` creates: the `wet` branch, the rulesets, the `idp-approval` and `idp-write` environments, the secrets `IDP_READER_PRIVATE_KEY` and `IDP_STATE_PASSPHRASE`, the variables `IDP_READER_CLIENT_ID` and `IDP_BOOTSTRAP`, and, in `idp-write`, the secret `IDP_WRITER_PRIVATE_KEY` and the variable `IDP_WRITER_CLIENT_ID`.
2. Copy the four files of [`examples/claims-repo/.github/workflows/`](../examples/claims-repo/.github/workflows/) into the claims repo's `.github/workflows/`. Keep the pinned engine commit; all four must pin the same one.
3. Add `config/platform.yaml` ([claims reference](claims.md)), and claims when you want them.
4. Open a pull request. The `idp-gate` check that the `main` ruleset requires comes from the PR workflow.

## The four workflows

| Workflow | Trigger | Checks | What it does |
|---|---|---|---|
| `pr.yaml` | pull request to `main` | `pr / validate`, `pr / plan`, `pr / comment`, `pr / verdict`, `idp-gate` | Validates and renders the claims, diffs the render against `wet`, plans the affected stacks with the reader App, and keeps one sticky plan comment on the PR. |
| `reconcile.yaml` | push to `main`, manual dispatch | `reconcile / plan`, `reconcile / approve`, `reconcile / apply-github` | Re-renders and diffs against the current `wet` (every stack on a dispatch), plans, gates, waits for approval when the gate asks for it, applies the saved plan with the writer App, and commits the state and the render to `wet`. |
| `drift.yaml` | daily at 05:23 UTC, manual dispatch | `drift / drift` | Plans the `wet` render and checks the bootstrap protections, both with the reader App, and keeps one `drift` issue open while anything drifted. |
| `recover.yaml` | manual dispatch with a run id | `recover / recover` | Commits the tree a reconcile run saved when its wet push failed. |

### Pull requests

- Pull requests from forks are validated and rendered, never planned: they get no secrets, and `pr / verdict` fails with "a maintainer must push this branch to the repo" (spec §6.1).
- The plan comment carries a hidden fingerprint of the plan for the PR's head commit. The reconcile gate trusts only that comment, written by `github-actions[bot]` ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).
- `idp-gate` always runs and passes only when the reusable workflow succeeded, so a cancelled or failed run never counts as passing ([ADR-0022](adr/0022-required-check-lives-in-the-caller.md)).

### Reconcile

- The gate applies automatically when nothing is deleted or replaced and every change was in the merged PR's plan comment; otherwise the `idp-approval` environment asks a platform admin first. The rules are in [docs/cli.md](cli.md#gate).
- The apply uses the plan saved by `reconcile / plan` and the render from the same run; it never re-renders.
- The state is always committed to `wet`, even when the apply failed, because real resources may have changed. The render is committed only when the apply succeeded or had nothing to apply, so a failed stack is retried by the next reconcile.
- A reconcile run is never re-run: its jobs refuse a second attempt, and the apply refuses a `wet` that moved since its plan. Retry with a new dispatch ([ADR-0024](adr/0024-failed-wet-push-recovery.md)).

### Drift

- A run fails only when the check cannot run. Drift opens (or updates) the issue labelled `drift`, titled "Drift detected", and adds a warning annotation; a clean run closes it. Nothing is remediated automatically.
- Ruleset bypass lists are invisible to the reader token, so `bootstrap check` reports them as notices ([ADR-0017](adr/0017-drift-check-token.md)).

### Recover

- See the [runbook](runbooks/pipelines.md#a-wet-push-failed). The recovery refuses a run that is not a failed reconcile of `main`, and refuses when `wet` moved since that run's plan.

## Permissions and credentials

| Job | `GITHUB_TOKEN` scopes | App token | Environment |
|---|---|---|---|
| `pr / validate` | `contents: read` | none | none |
| `pr / plan` | `contents: read` | reader, whole installation | none |
| `pr / comment` | `contents: read`, `pull-requests: write` | none | none |
| `pr / verdict` | none | none | none |
| `reconcile / plan` | `contents`, `issues`, `pull-requests`: read | reader | none |
| `reconcile / approve` | none | none | `idp-approval` |
| `reconcile / apply-github` | `contents: read`, `issues: write` | writer for the apply (the writer manifest's permissions, whole installation); writer for the wet commit (`contents: write`, the claims repo only) | `idp-write` |
| `drift / drift` | `contents: read`, `issues: write` | reader | none |
| `recover / recover` | `actions: read`, `contents: read`, `issues: write` | writer for the wet commit | `idp-write` |

Each token is set only on the step that uses it ([ADR-0023](adr/0023-pipeline-tokens-per-step.md)).

## Concurrency and artifacts

- `idp-plan-<PR number>` cancels older runs of the same PR.
- `idp-wet` (with `queue: max`) runs reconcile, drift and recover one at a time. A reconcile waiting for approval holds it.
- Artifacts: `idp-pr-comment` (the plan comment, 1 day), `idp-reconcile` (the encrypted plan and the render, 1 day), `idp-wet-root` (the wet commit with the encrypted state, 7 days). Artifacts of public repos are public; the plan and the state are encrypted ([ADR-0002](adr/0002-encrypted-opentofu-state-in-git.md)).

## How the workflows get idp

Each job checks out the engine at the reusable workflow's own commit and builds `idp` from it with the `setup-idp` composite action, so the workflow, the CLI and the rendered module refs are one version ([ADR-0021](adr/0021-reusable-workflows-build-idp-from-their-commit.md)).

## Limitations

- Only the `github` stack is reconciled for now; the workflows fail when any other stack is affected.
- The required check comes from a workflow file in the claims repo, so a pull request can change it. Requiring a review on `.github/` closes that gap; it is the owner's decision ([ADR-0022](adr/0022-required-check-lives-in-the-caller.md)).
- GitHub disables scheduled workflows in public repos after 60 days without activity. Re-enable the drift check with `gh workflow enable drift.yaml`.
````

- [ ] **Step 2: Write `docs/runbooks/pipelines.md`**

````markdown
# Pipelines runbook

Every procedure here uses only GitHub: the Actions tab, `gh`, and the claims repo. None needs the state passphrase or an App key on your machine. In the commands, `<repo>` is the claims repo, for example `jellalshadows-idp/idp-claims`.

## idp-gate failed

Open the run's `pr / verdict` job: it says what failed.

- **"a maintainer must push this branch to the repo"**: the PR comes from a fork, so nothing is planned (spec §6.1). A maintainer pushes the branch to the claims repo and opens the PR from there.
- **`pr / validate` failed**: the annotations on the PR point at each claim error ([claims reference](../claims.md)).
- **`pr / plan` failed in "check the repository configuration"**: a secret or variable is missing. Re-run the bootstrap ([bootstrap runbook](bootstrap.md)), and check that the caller passes both secrets.
- **`pr / plan` failed in `tofu`**: read the log. An error while reading a resource usually means the reader App lacks a permission.
- A transient failure: re-run the failed jobs of the PR run, or push again.

## A reconcile waits for approval

The gate asked for approval: the plan deletes or replaces something, or has a change that the merged PR's plan comment did not show ([ADR-0018](../adr/0018-plan-fingerprint-and-gate-trust.md)). The plan is in the summary of the `reconcile / plan` job.

1. Read the plan.
2. Approve or reject the `idp-approval` deployment in the run.
3. Rejecting applies nothing; the next reconcile plans again.

While it waits, the run holds the `idp-wet` concurrency group: drift and other reconciles queue behind it. Decide promptly.

## A reconcile failed

Never re-run a reconcile run: its jobs refuse a second attempt, because the plan may be stale. Dispatch a new one, which plans every stack against the current `wet`:

```bash
gh workflow run reconcile.yaml --repo <repo> --ref main
```

- **`apply-github` failed in "apply the saved plan"**: the state was still committed to `wet`, because real resources may have changed, and the old render stayed, so the next reconcile retries the stack. Fix the cause (a claims PR, or wait out a transient error), then dispatch. The new plan shows only what is left; the gate asks for approval when that is not in the last merged PR's plan.
- **`apply-github` failed in "refuse when wet moved since the plan"**: something committed to `wet` after this run planned. Dispatch a new reconcile.

## A wet push failed

The apply job opened an issue labelled `idp-wet-push`, titled "Wet push failed". What the job was about to commit is saved as the artifact `idp-wet-root` of that run, for 7 days.

1. Run the recovery with the failed run's id (the number in its URL):

   ```bash
   gh workflow run recover.yaml --repo <repo> --ref main -f run-id=<run id>
   ```

2. It checks that the run is a failed reconcile of `main`, downloads the artifact, and refuses if `wet` moved since that run's plan. Otherwise it commits the saved state, and the render if the apply had succeeded, and closes the issue.
3. If it refuses because `wet` moved, or the 7 days have passed, do not force anything: the saved state is older than `wet`'s. The state has to be rebuilt with `import` blocks (spec §7.7); that runbook is not written yet, so open an issue in the engine repo.

## A drift issue is open

The drift check found that GitHub differs from the `wet` render, or that a bootstrap protection changed. The issue holds the plan summary and the bootstrap findings.

- Fix the claims in a PR, or dispatch the reconcile to put GitHub back to the claims. A remediation is not in the last merged PR's plan, so it waits for approval.
- For bootstrap findings, re-run `idp bootstrap apply` ([bootstrap runbook](bootstrap.md)).
- The issue closes on the next clean drift run. To run one now:

  ```bash
  gh workflow run drift.yaml --repo <repo> --ref main
  ```

## The drift schedule stopped

GitHub disables scheduled workflows in public repos after 60 days without activity. Re-enable it:

```bash
gh workflow enable drift.yaml --repo <repo>
```
````

- [ ] **Step 3: Update `README.md`**

Replace

```markdown
- Claims reference: [docs/claims.md](docs/claims.md)
- CLI reference: [docs/cli.md](docs/cli.md)
```

with

```markdown
- Claims reference: [docs/claims.md](docs/claims.md)
- Pipelines: [docs/pipelines.md](docs/pipelines.md)
- CLI reference: [docs/cli.md](docs/cli.md)
- Runbooks: [docs/runbooks/](docs/runbooks/)
```

and append to "Honest limitations (v1)":

```markdown
- Phase 1 reconciles **GitHub only** (teams, repositories, rulesets, environments); AWS arrives in Phase 2.
- A reconcile that needs approval holds the claims repo's pipelines until someone decides.
```

(The Status lines change in Task 11, once the smoke has run.)

- [ ] **Step 4: Check the links and the anchor**

Run: `rg -n "a-wet-push-failed|## A wet push failed" docs .github`
Expected: the heading in `docs/runbooks/pipelines.md`, the link in `docs/pipelines.md`, and the `RUNBOOK_URL` in `.github/workflows/reconcile.yaml`.

- [ ] **Step 5: Commit**

```bash
git add docs/pipelines.md docs/runbooks/pipelines.md README.md
git commit -m "docs: add the pipelines guide and runbook"
```

---

### Task 9: Ship I — local checks, PR, CI, bootstrap re-apply, final review

Controller task. Running tests and linters is delegated to a subagent (owner rule).

**Files:** none new (fix-wave edits only).

- [ ] **Step 1: Full local verification**

```bash
go test ./... && go vet ./... && gofmt -l .
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -ignore 'unexpected key "queue" for "concurrency" section' examples/claims-repo/.github/workflows/*.yaml
"$ZIZMOR" --offline .github/workflows .github/actions/setup-idp/action.yaml examples/claims-repo/.github/workflows/*.yaml
```

Expected: all green; gofmt and actionlint print nothing; zizmor reports no unsuppressed findings.

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feat/phase-1c-pipelines
gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-1c-pipelines --title "feat: phase 1c reusable pipelines" --body "Implements docs/superpowers/plans/2026-10-11-phase-1c-pipelines.md (ADRs 0021-0024, spec amendment A6). Adds the reusable pr, reconcile, drift and recover workflows, the setup-idp composite action, the claims repo caller templates, and their contract tests. The live smoke run in idp-claims-e2e is recorded in the phase log."
```

- [ ] **Step 3: Wait for CI**

Run: `gh pr checks --repo jellalshadows-idp/idp-engine --watch`
Expected: `go`, `workflows`, `modules (github/group)`, `modules (github/component)` and `render-smoke` pass. A failure goes through the fix loop (implementer dispatch, scoped re-review).

- [ ] **Step 4: Re-run the bootstrap of both claims repos (adds `IDP_BOOTSTRAP`)**

Git Bash, from the repo root. The `.pass` and App files are only read.

```bash
for pair in "idp-claims-e2e e2e" "idp-claims main"; do
  set -- $pair
  GH_TOKEN="$(gh auth token -u jellalshadows)" go run ./cmd/idp bootstrap check --org jellalshadows-idp --claims-repo "$1" \
    --approver jellalshadows --reader ~/.idp/apps/jellalshadows-idp-reader.json --writer ~/.idp/apps/jellalshadows-idp-writer.json || true
  GH_TOKEN="$(gh auth token -u jellalshadows)" go run ./cmd/idp bootstrap apply --org jellalshadows-idp --claims-repo "$1" \
    --approver jellalshadows --reader ~/.idp/apps/jellalshadows-idp-reader.json --writer ~/.idp/apps/jellalshadows-idp-writer.json \
    --passphrase-file ~/.idp/"$2".pass
  GH_TOKEN="$(gh auth token -u jellalshadows)" go run ./cmd/idp bootstrap check --org jellalshadows-idp --claims-repo "$1" \
    --approver jellalshadows --reader ~/.idp/apps/jellalshadows-idp-reader.json --writer ~/.idp/apps/jellalshadows-idp-writer.json
done
```

Expected per repo: the first check reports the missing `IDP_BOOTSTRAP` variable (exit 3); `apply` prints the variable it creates, `kept existing secret …` lines and `bootstrap apply: done`; the second check prints `bootstrap check: no drift` (exit 0). Any other change `apply` reports is a finding: stop and report it before continuing.

Then: `gh api repos/jellalshadows-idp/idp-claims-e2e/actions/variables/IDP_BOOTSTRAP --jq .name` prints `IDP_BOOTSTRAP`.

- [ ] **Step 5: Final whole-branch review (before the smoke, ruling P-8)**

Dispatch the final reviewer on the most capable model with the whole-branch package (`main..HEAD`), the spec, ADRs 0018 and 0021–0024 and this plan. Then one fix dispatch, one scoped re-review, and a ruling on every residual, all in the ledger. If a fix changes a reusable workflow, the composite action or anything `idp` is built from, re-pin the templates (Task 7's command) in a separate commit, push, and wait for CI again.

---

### Task 10: Live smoke in `idp-claims-e2e`

Run by a dispatched subagent on a mid-tier model, step by step. It **stops at the first unexpected result** and reports the evidence; the controller decides (fix loop below). Every command is Git Bash. Record every PR number, run URL and commit SHA in the report: the phase log quotes them.

**Setup**

```bash
E2E=jellalshadows-idp/idp-claims-e2e
ENGINE_SHA=$(git -C /c/Users/Usuario/idp-engine rev-parse origin/feat/phase-1c-pipelines)
gh api user --jq .login                                    # jellalshadows
gh api repos/jellalshadows-idp/e2e-smoke-app --silent      # must fail with HTTP 404
gh api orgs/jellalshadows-idp/teams/e2e-smoke --silent     # must fail with HTTP 404
gh repo clone "$E2E" "$SCRATCH/idp-claims-e2e"
```

`SCRATCH` is the session scratchpad as a Windows path (`C:\…`), because `python` is a Windows program and does not understand `/c/…` paths.

The plan comment (`plan-summary`) has one `<code>github</code>: +C ~U -D ±R` summary per stack with changes, the line `No infrastructure changes.` when there are none, and a `> [!WARNING]` block when anything is deleted or replaced. The run logs print the same counts as `github: +C ~U -D ±R`, and the gate prints `gate: auto (…)` or `gate: approval (…)`.

Helpers used below (define them in the same shell):

```bash
# The newest run of a workflow for a commit.
run_for() { gh run list --repo "$E2E" --workflow "$1" --commit "$2" --limit 1 --json databaseId --jq '.[0].databaseId'; }
# The plan comment of a PR, by the bot.
plan_comment() { gh api "repos/$E2E/issues/$1/comments" --jq '[.[] | select(.user.login == "github-actions[bot]" and (.body | contains("<!-- idp-plan -->")))] | last | .body'; }
# Approve the pending idp-approval deployment of a run.
approve() {
  env_id=$(gh api "repos/$E2E/actions/runs/$1/pending_deployments" --jq '.[0].environment.id')
  gh api -X POST "repos/$E2E/actions/runs/$1/pending_deployments" -F "environment_ids[]=$env_id" -f state=approved -f comment="$2"
}
```

A run appears a few seconds after its trigger; when `run_for` prints nothing, wait and retry. Waiting on a condition uses a polling loop (or the Monitor tool when foreground `sleep` is blocked).

**S1. Onboard (create through a PR).** In the clone, on branch `smoke/onboard`:
- copy the four templates from `idp-engine/examples/claims-repo/.github/workflows/` to `.github/workflows/`, then re-pin them to `$ENGINE_SHA` with Task 7's re-pin command (pass the SHA and the clone's `.github/workflows` path instead of `HEAD` and `examples/...`);
- write `config/platform.yaml`:

```yaml
apiVersion: idp/v1
kind: Platform
github:
  org: jellalshadows-idp
  writerAppId: 5255579
  archiveOnDestroy: false
naming:
  requiredPrefix: e2e-
environments:
  dev: {}
```

- write `claims/groups/e2e-smoke.yaml`:

```yaml
apiVersion: idp/v1
kind: Group
name: e2e-smoke
description: Pipeline smoke team
members:
  - user: jellalshadows
    role: maintainer
```

- write `claims/components/e2e-smoke-app.yaml`:

```yaml
apiVersion: idp/v1
kind: Component
name: e2e-smoke-app
description: Pipeline smoke repository
owner: group:e2e-smoke
environments: [dev]
github:
  topics: [e2e]
```

Commit (`feat: onboard the idp pipelines with a smoke group and component`), push, open the PR (`gh pr create --repo "$E2E" --base main --head smoke/onboard --title "feat: onboard the idp pipelines" --body "Phase 1c smoke: callers pinned to $ENGINE_SHA, platform config, one group and one component."`).

Expected: `gh pr checks <PR> --repo "$E2E" --watch` ends with `pr / validate`, `pr / plan`, `pr / comment`, `pr / verdict` and `idp-gate` passing. `plan_comment <PR>` contains `<!-- idp-plan -->`, a `<code>github</code>: +N ~0 -0 ±0` summary with N above 0 and no `[!WARNING]`, and `idp-fingerprint:v1 sha=<the PR head SHA>` (compare with `gh pr view <PR> --repo "$E2E" --json headRefOid --jq .headRefOid`).

**S2. Merge → automatic apply.** `gh pr merge <PR> --repo "$E2E" --merge --delete-branch`; `MERGE=$(gh pr view <PR> --repo "$E2E" --json mergeCommit --jq .mergeCommit.oid)`; `RUN=$(run_for reconcile.yaml "$MERGE")`; `gh run watch "$RUN" --repo "$E2E" --exit-status`.

Expected:
- jobs (`gh run view "$RUN" --repo "$E2E" --json jobs --jq '.jobs[] | [.name, .conclusion] | @tsv'`): `reconcile / plan` success, `reconcile / approve` skipped, `reconcile / apply-github` success; the plan log has `gate: auto`.
- `gh api repos/jellalshadows-idp/e2e-smoke-app --jq '[.visibility, .description, (.topics | join(","))] | @tsv'` → `public	Pipeline smoke repository	e2e`.
- `gh api orgs/jellalshadows-idp/teams/e2e-smoke --jq .privacy` → `closed`; `gh api orgs/jellalshadows-idp/teams/e2e-smoke/memberships/jellalshadows --jq .role` → `maintainer`.
- `gh api repos/jellalshadows-idp/e2e-smoke-app/environments --jq '[.environments[].name] | join(",")'` → `dev`; `gh api repos/jellalshadows-idp/e2e-smoke-app/rulesets --jq length` → at least `1`.
- `gh api "repos/$E2E/commits/wet" --jq '[.commit.verification.verified, .author.login, .commit.message] | @tsv'` → `true	jellalshadows-idp-writer[bot]	idp: reconcile <MERGE> (run <RUN>)`.
- The state in `wet` is encrypted: `gh api "repos/$E2E/contents/tfstate/github.tfstate?ref=wet" --jq .content | python -I -c "import base64,json,sys; d=json.loads(base64.b64decode(sys.stdin.read())); print('encrypted_data' in d, 'resources' in d)"` → `True False`.

**S3. Drift, clean.** `gh workflow run drift.yaml --repo "$E2E" --ref main`; find and watch the run (`gh run list --repo "$E2E" --workflow drift.yaml --event workflow_dispatch --limit 1 --json databaseId --jq '.[0].databaseId'`).
Expected: success; the log shows `bootstrap check: no drift` (notices allowed) and the report step prints `issue: none open`.

**S4. Drift, induced.** `gh api -X PATCH repos/jellalshadows-idp/e2e-smoke-app -f description='drifted by the phase 1c smoke' --silent`, then dispatch drift again.
Expected: success with the warning "drift found"; `gh issue list --repo "$E2E" --label drift --state open --json number,title` shows one issue `Drift detected`, whose body has `<code>github</code>: +0 ~1 -0 ±0`.

**S5. Remediation through a dispatch (the approval path).** `gh workflow run reconcile.yaml --repo "$E2E" --ref main`; get the run id; wait until `gh api "repos/$E2E/actions/runs/$RUN/pending_deployments" --jq length` is `1`.
Expected before approving: the plan job log has `github: +0 ~1 -0 ±0` (the description) and `gate: approval`. Only then: `approve "$RUN" "phase 1c smoke: restore the drifted description"`; watch the run.
Expected: apply-github success; the description is back to `Pipeline smoke repository`. Dispatch drift: success, and `gh issue list --repo "$E2E" --label drift --state closed --limit 1 --json title` shows the closed `Drift detected`.

**S6. Update through a PR (automatic, subset).** Branch `smoke/update` from the latest `main`: description `Pipeline smoke repository, updated`, topics `[e2e, smoke]`. PR → checks → merge → reconcile.
Expected: the PR comment has `<code>github</code>: +0 ~1 -0 ±0`; the reconcile has `gate: auto`; the repository has the new description and topics `e2e,smoke`.

**S7. Partial apply failure.** First pick a login that does not exist: `gh api users/e2e-no-such-login-idp1c --silent` must fail with HTTP 404 (if it exists, append a digit and check again). Branch `smoke/bad-member`: add `{ user: <that login>, role: member }` to the group and set the description to `Pipeline smoke repository, partial apply`. PR → checks pass (the comment has `<code>github</code>: +1 ~1 -0 ±0`: a plan never calls the API for a membership it creates) → merge → reconcile (`RUN_S7`).
Expected:
- the plan job has `gate: auto`; the run fails: `reconcile / apply-github` fails in "apply the saved plan" (the membership cannot be created);
- "prepare the wet commit" emits the warning "only the state is committed"; "commit to wet" succeeds; "open the wet-push issue" is skipped;
- `gh api "repos/$E2E/commits/wet" --jq '[.files[].filename] | join(",")'` → `tfstate/github.tfstate`;
- the description is `Pipeline smoke repository, partial apply` (the other change applied);
- the run has an `idp-wet-root` artifact (`gh api "repos/$E2E/actions/runs/$RUN_S7/artifacts" --jq '[.artifacts[].name] | join(",")'` includes it).

**S8. Self-heal.** Branch `smoke/fix-member`: remove the bad member. PR → checks.
Expected: the PR comment says `No infrastructure changes.` (the render differs from `wet`, but the state already matches it). Merge → reconcile: `gate: auto`; in apply-github the token and apply steps are skipped and "commit to wet" succeeds; `gh api "repos/$E2E/commits/wet" --jq '[.files[].filename] | join(",")'` → `rendered/github/main.tf.json`; and `gh api "repos/$E2E/contents/rendered/github/main.tf.json?ref=wet" --jq .content | python -I -c "import base64,sys; t=base64.b64decode(sys.stdin.read()).decode(); print('e2e-no-such-login' in t, 'partial apply' in t)"` → `False True`.

**S9. Recovery refuses a stale run.** `WET_BEFORE=$(gh api "repos/$E2E/commits/wet" --jq .sha)`; `gh workflow run recover.yaml --repo "$E2E" --ref main -f run-id="$RUN_S7"`; watch.
Expected: the run fails in "refuse when wet moved since that run" with "wet moved since that run read it"; `gh api "repos/$E2E/commits/wet" --jq .sha` still equals `$WET_BEFORE`.

**S10. Delete through a PR (destructive, approval).** Branch `smoke/delete`: `git rm` both claim files. PR → checks.
Expected: the PR comment has a `<code>github</code>: +0 ~0 -N ±0` summary with N above 0 and the `[!WARNING]` block. Merge → reconcile: the plan log has the same deletions and `gate: approval`. Then `approve "$RUN" "phase 1c smoke: delete the smoke repo and team"`; watch.
Expected: success; `gh api repos/jellalshadows-idp/e2e-smoke-app --silent` and `gh api orgs/jellalshadows-idp/teams/e2e-smoke --silent` fail with HTTP 404; the `wet` render has no `module` key: `gh api "repos/$E2E/contents/rendered/github/main.tf.json?ref=wet" --jq .content | python -I -c "import base64,json,sys; print('module' in json.loads(base64.b64decode(sys.stdin.read())))"` → `False`.

**S11. Final drift.** Dispatch drift.
Expected: success, no drift, and `gh issue list --repo "$E2E" --label drift --state open --json number --jq length` → `0`.

**Fix loop.** When a step fails unexpectedly:
1. The subagent stops and reports the step, the run URL and the failing log lines (`gh run view <id> --repo "$E2E" --log-failed`).
2. The controller rules on the cause. A workflow defect is fixed on the engine branch by an implementer dispatch, with a contract test that pins it where one can, then a scoped review; push; wait for CI; re-pin the templates if needed.
3. Re-pin the e2e callers to the new branch head through a PR in `idp-claims-e2e` (its checks must pass; merge it).
4. Resume at the failed step. If the failure left `e2e-` resources behind and the platform can no longer delete them, stop and ask the owner (manual deletion is not authorized).

---

### Task 11: Ship II — merge, re-pin the e2e callers, execution log

**Files:**
- Create: `docs/phases/phase-1c.md`
- Modify: `README.md` (Status lines)

- [ ] **Step 1: Merge the engine PR**

Confirm the PR's latest CI run is green, then:

```bash
gh pr merge --repo jellalshadows-idp/idp-engine --merge --delete-branch
git switch main && git pull --ff-only
MERGE=$(git rev-parse HEAD)
```

- [ ] **Step 2: Re-pin the e2e callers to the merge commit**

In the `idp-claims-e2e` clone, from the latest `main`, on branch `chore/pin-engine`: run Task 7's re-pin command with `$MERGE` and the clone's `.github/workflows`; commit `chore: pin the idp pipelines to <short merge sha>`; push; open the PR; wait for its checks; merge it.

Expected: `idp-gate` passes at the merge commit; the reconcile run for the merge succeeds with nothing affected (`diff: 0 affected stack(s)`), because a render with no claims has no module refs.

- [ ] **Step 3: Write `docs/phases/phase-1c.md` and update the README status**

Write the execution log in the style of `docs/phases/phase-1b.md`:
- the summary;
- a task table with commits and review outcomes;
- every ruling (P-1 to P-11 and those made during execution), with what, why and cost if wrong;
- the smoke evidence, step by step: PR numbers, run URLs, wet commits, the two approvals;
- every finding with its final disposition (fixed, Phase 1d, or won't fix with a reason);
- carried into Phase 1d: I1-B, M1, M4, M7, production onboarding, release mode for `idp` (spec §7.5), the E2E harness;
- owner notes: require a review on `.github/` in claims repos and pin `idp-gate` to the GitHub Actions integration (ADR-0022), still the owner's decision.

Then replace the two status lines of `README.md`:

```markdown
> **Status:** Phase 1c is complete: the reusable `pr`, `reconcile`, `drift` and `recover` workflows reconcile a claims repo end to end, proven by a live smoke run in `idp-claims-e2e`; see the [phase 1c log](docs/phases/phase-1c.md) and [docs/pipelines.md](docs/pipelines.md). Phase 1d (the E2E harness and the first release) is next.

**What exists today:** `idp-claims-e2e` runs the pipelines against the org; `idp-claims` is bootstrapped and protected, and gets its workflows with the first release.
```

- [ ] **Step 4: Commit (doc-only, straight to main) and push**

```bash
git add docs/phases/phase-1c.md README.md
git commit -m "docs: add the phase 1c execution log"
git push
```
