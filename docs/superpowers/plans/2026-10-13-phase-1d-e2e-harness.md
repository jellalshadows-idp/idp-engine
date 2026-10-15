# Phase 1d — End-to-End Harness and Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the end-to-end harness (spec §8.5) that drives `idp-claims-e2e` through the real pipelines without a person — create, update, remove all topics, a case-only login edit, drift, a destructive delete with an approval, drift — plus the credentials it needs and the hardening carried from Phases 1b and 1c, and prove it with a live run.

**Architecture:**
- **The harness is a separate program,** `cmd/idp-e2e` over `internal/e2e`, not part of the released CLI. A `Forge` interface separates the scenario (steps, waits, assertions, cleanup) from GitHub; scenario tests run against a scripted fake, and a thin GitHub implementation over `internal/ghapi` is tested with `httptest`.
- **Two credentials, one environment.** A third GitHub App, `<org>-bot`, does the repository work (branches, files, pull requests, merges, dispatches) on `idp-claims-e2e` only. The token of a test account that is a second `idp-approval` reviewer in `idp-claims-e2e` approves the destructive reconcile, because only a person can approve a deployment. Both live in the `idp-e2e` environment of `idp-engine` (main only).
- **Hardening.** The gate ignores a plan comment edited by anyone but the Actions bot; labelled issues must be the bot's; `wet-push` validates branch names; topics move to the authoritative `github_repository_topics` resource; bootstrap accepts several approvers and creates the bot App; the workflow contract tests close their known blind spots.

**Tech Stack:** Go 1.26 (standard library plus the existing `go.yaml.in/yaml/v3`; no new dependencies), GitHub REST and GraphQL APIs, OpenTofu 1.12.6 with `integrations/github` 6.13.0, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §6.2, §7.3, §8.5, §9.2 and §10. Context: `docs/phases/phase-1c.md` ("Carried into Phase 1d", "Final disposition of every finding"), `docs/phases/phase-1a.md` (minor #3, F1), ADR-0018, ADR-0023, ADR-0025.

## Phase 1 re-split (decided while planning)

The remaining Phase 1 work is two independent subsystems, each worth its own reviewable plan, and each ships working software. The harness comes first, because spec §8.5 runs it on release PRs and Phase 1's exit criterion is an E2E run.

| Plan | Delivers |
|---|---|
| 1a–1c (done) | Claims → render; pipeline commands; reusable workflows proven by a live smoke |
| **1d (this plan)** | The E2E harness and its credentials (bot App, approver account); hardening carried from 1b/1c (gate editor check, bot-authored issues, branch validation, topics resource, approver lists, contract-test blind spots) |
| 1e | release-please + GoReleaser + attestations, the workflows' download-and-verify of the release binary (owner decision), self-hosted Renovate (owner decision), `v0.1.0`, the templates re-pinned to the release, onboarding the production `idp-claims` |

## Decisions taken while planning

The executor copies these into the ledger as rulings before Task 1. D-1 to D-4 are the owner's (AskUserQuestion, 2026-10-13).

- **D-1 (owner). A third App, `<org>-bot`,** created with the manifest flow and installed only on `idp-engine`, `idp-claims` and `idp-claims-e2e`, opens and merges pull requests for the harness now and for release-please and Renovate in Phase 1e (ADR-0026). Its manifest already carries the permissions Phase 1e needs, so there is no second browser step.
- **D-2 (owner). The second account joins the org** and becomes a second `idp-approval` reviewer in `idp-claims-e2e` only; its fine-grained token, limited to `idp-claims-e2e` with Actions read and Deployments write, approves the harness's destructive reconcile (ADR-0027).
- **D-3 (owner). Renovate runs self-hosted with the bot App** — Phase 1e.
- **D-4 (owner). The workflows download the release binary and verify its attestation**, building from source only when the pinned commit has no release — Phase 1e (spec §7.5 as written).
- **D-5. Re-split** (table above).
- **D-6. The harness is `cmd/idp-e2e`,** a separate binary over `internal/e2e`; GoReleaser (1e) will build only `cmd/idp`.
- **D-7. The harness changes files with the Contents API** (one commit per file), not `wet-push`: it needs precise single-file deletes, which `wet-push` refuses by design.
- **D-8. Coverage.** The harness covers creation, update, removing all topics, a case-only login edit, drift, a destructive delete with an approval, and a final drift. A partial apply failure and the `recover` workflow stay proven by the Phase 1c smoke (they need a broken input or a broken push on purpose).
- **D-9. Triggers.** Nightly (`41 3 * * *`) and `workflow_dispatch`, from `main`. Release PRs join in Phase 1e, once release-please exists.
- **D-10. Phase 1b minors.** M1 (forget column) goes to Phase 4, where `removed` blocks may first appear; M4 (wet marker for `--wet`) is won't fix: the path is a constant pinned by a contract test, and a marker would need a migration of every wet branch; M7 (branch escaping) is fixed by validating branch names.
- **D-11. Topics move to `github_repository_topics`.** Phase 1a minor #3 is confirmed in the provider source (v6.13.0): `github_repository.topics` is Optional+Computed, so an empty list never clears the topics; `github_repository_topics` documents "Send an empty array ([]) to clear all topics". No repository is managed today (`idp-claims-e2e` is empty, `idp-claims` not onboarded), so no state moves.
- **D-12. The gate editor check (Phase 1c P-4) is implemented now:** a first release should not ship with a known, cheap-to-close trust gap.
- **D-13. Phase 1c contract-test minors.** Fixed: the reusable workflow list becomes explicit; `# unreleased` is accepted only for engine workflow refs; YAML anchors and aliases are rejected; the reconcile and drift plans may not use `-refresh`; `docs/pipelines.md` gets the `idp-gate` row. The rest (over-strict matching, wording, refactors) go to Phase 5 polish, each with its reason in the phase log.
- **D-14. Not in 1d:** the optional narrower plan token stays an owner note (ADR-0025); `adopt`'s read-only token is Phase 4.

## Global Constraints

**Pinned actions** (all at least 14 days old; unchanged from Phase 1c):

| Action | Version | SHA |
|---|---|---|
| `actions/checkout` | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` |
| `actions/setup-go` | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |
| `actions/create-github-app-token` | v3.2.0 | `bcd2ba49218906704ab6c1aa796996da409d3eb1` |

**Names** (copy verbatim)
- Bot App role `bot`; App name and slug `<org>-bot` (here `jellalshadows-idp-bot`); manifest `bootstrap/apps/bot.json`.
- Bot App permissions (repository): `actions: write`, `checks: read`, `contents: write`, `issues: write`, `metadata: read`, `pull_requests: write`, `statuses: read`, `workflows: write`; (organization): `members: read`. No events.
- `idp-engine` environment `idp-e2e` (deployment branch `main`); its secrets `IDP_BOT_PRIVATE_KEY`, `IDP_E2E_APPROVER_TOKEN`; its variable `IDP_BOT_CLIENT_ID`.
- Approvers: `idp-claims-e2e` → `jellalshadows`, the second account; `idp-claims` → `jellalshadows`.
- Harness: binary `cmd/idp-e2e`, package `internal/e2e`, workflow `.github/workflows/e2e.yaml` (`name: idp-e2e`), concurrency group `idp-e2e`, cron `41 3 * * *`.
- Harness claim names: `e2e-r<run>-team` (Group) and `e2e-r<run>-app` (Component), where `<run>` is the workflow run id; branches `e2e/<run>/<step>`.
- Harness bot token: repositories `idp-claims-e2e`; permissions `actions: write`, `checks: read`, `contents: write`, `issues: read`, `members: read`, `pull-requests: write`, `workflows: write`.
- GraphQL login of the Actions bot: `github-actions` (type `Bot`); REST login: `github-actions[bot]`.

**Process**
- Branch `feat/phase-1d-e2e-harness`; one PR, merged with a merge commit.
- Conventional commits, never with Co-Authored-By or any AI attribution.
- No `go build` locally. Use `go test ./...`, `go vet ./...`, `gofmt -l .` (must print nothing); `-race` runs in CI. `tofu` only through the module tests (`$HOME/.idp/tools/tofu-1.12.6/tofu.exe`, or CI).
- Lint: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`; zizmor 1.30.1 `--offline` (the controller installs it in pre-flight, as in Phase 1c).
- Shell rule: never use `cat`, `grep`, `find`, `sed` or `ls`, never write files with shell heredocs, never `perl`. Use the Read/Write/Edit tools, `rg` (with `--hidden` for `.github`), `gh`, `git`, `python -I`.
- Never delete or overwrite `~/.idp/e2e.pass`, `~/.idp/main.pass` or `~/.idp/apps/*`.
- Docs in English. Owner rule: docs describe the platform on its own terms, never naming another product as inspiration or comparison; the controller gives each dispatch the check that enforces it.

**Owner actions** (only the owner can do these; the plan stops and asks at each)
- O-1. Confirm "Create GitHub App" in the browser for the bot App (the controller starts `idp bootstrap app --role bot`).
- O-2. Install the bot App on **only** `idp-engine`, `idp-claims`, `idp-claims-e2e`.
- O-3. Accept the org invitation from the second account.
- O-4. As the second account, create the fine-grained token (resource owner `jellalshadows-idp`; repository `idp-claims-e2e` only; Actions read, Deployments read and write; the longest expiry the org allows), approve it in the org settings if the org asks, and store it: `! gh secret set IDP_E2E_APPROVER_TOKEN --env idp-e2e --repo jellalshadows-idp/idp-engine`.

**Side effects this plan authorizes** (controller, after the owner approves the plan)
1. Push `feat/phase-1d-e2e-harness`, open its PR, merge it after review and the live run.
2. Start `idp bootstrap app --org jellalshadows-idp --role bot` locally (loopback callback); it writes `~/.idp/apps/jellalshadows-idp-bot.{json,pem}`.
3. Invite the second account to `jellalshadows-idp` as a member (owner token).
4. In `idp-engine`: create environment `idp-e2e` with deployment branch policy `main`; set its secret `IDP_BOT_PRIVATE_KEY` from the bot's `.pem` and its variable `IDP_BOT_CLIENT_ID` from the bot's `.json`; for the pre-merge live run, add the branch `feat/phase-1d-e2e-harness` to that policy, and remove it right after.
5. `idp bootstrap apply` for `idp-claims-e2e` (approvers `jellalshadows` and the second account) and `idp-claims` (approver `jellalshadows`), which rewrites `IDP_BOOTSTRAP` in the new format and sets the e2e approval reviewers.
6. Harness runs: in `idp-claims-e2e`, branches, pull requests, merges, workflow dispatches, one approval per run with the approver token; through the platform, creating and deleting `e2e-r<run>-*` repositories and teams.
7. Doc-only commits straight to `main` after the merge (phase log, README).

Deleting anything by hand is **not** authorized: stop and ask.

## Review Focus

1. **A run that fails halfway leaves `e2e-` resources behind.**
   - **Expected:** the harness deletes its own claims through the platform (PR, approval, apply) and reports both errors; it never deletes a GitHub resource directly and never touches a name without the `e2e-` prefix.
   - **Pinned by:** `TestScenarioCleansUpAfterAFailure`, `TestNamesRefuseOtherPrefixes` (Task 7).
2. **The approver token approves something else.**
   - **Expected:** it approves only the `idp-approval` deployment of the reconcile run of the harness's own delete merge; a reconcile that unexpectedly waits for approval is an error, never approved.
   - **Pinned by:** `TestApproveOnlyTheDeleteRun` (Task 7), `TestGitHubApprovesWithTheApproverToken` (Task 8).
3. **A person edits the bot's plan comment to widen the fingerprint.**
   - **Expected:** the gate no longer trusts it, so the reconcile asks for approval; the bot's own edits stay trusted.
   - **Pinned by:** `TestGateIgnoresACommentEditedByAPerson`, `TestGateTrustsACommentTheBotEdited` (Task 2).
4. **Removing every topic from a claim.**
   - **Expected:** the repository ends with no topics.
   - **Pinned by:** the `topics_can_be_cleared` module test (Task 4) and `TestScenarioRemovesAllTopics` (Task 7); proven live in Task 11.
5. **A person opens an issue with the bot's label and title.**
   - **Expected:** the platform never edits, closes or counts it; it opens its own.
   - **Pinned by:** `TestFindOpenIssueIgnoresIssuesByOthers` (Task 3).

## File Structure

```
docs/adr/0026-automation-app.md, 0027-end-to-end-harness.md, README.md   # Task 1
docs/adr/0018-plan-fingerprint-and-gate-trust.md                         # Task 1 (amendment)
docs/superpowers/specs/2026-10-08-idp-on-actions-design.md               # Task 1 (amendment A8)
internal/ghapi/graphql.go, graphql_test.go                               # Task 2
internal/fakegithub/fakegithub.go                                        # (modify) Tasks 2, 3
internal/ghops/ghops.go, ghops edit test                                 # Task 2 (EditedByOthers); Task 3 (Issue.User)
internal/cli/gate.go, gate_test.go, docs/cli.md                          # Task 2
internal/ghops/write.go, write_test.go                                   # Task 3
internal/wetpush/wetpush.go, wetpush_test.go                             # Task 3
modules/github/component/main.tf, tests/component.tftest.hcl             # Task 4
internal/bootstrap/{config.go,desired.go,appflow.go}, tests              # Task 5
bootstrap/apps/bot.json                                                  # Task 5
internal/cli/bootstrap.go, tests; docs/cli.md; docs/runbooks/bootstrap.md # Task 5
internal/workflowspec/{load_test.go,rules_test.go,common_test.go,reconcile_test.go,drift_test.go,templates_test.go}  # Task 6
docs/pipelines.md                                                        # Task 6
internal/e2e/{names.go,claims.go,pin.go,clock.go,forge.go,scenario.go}, tests  # Task 7
internal/e2e/{github.go,main.go}, tests; cmd/idp-e2e/main.go             # Task 8
.github/workflows/e2e.yaml; internal/workflowspec/e2e_test.go            # Task 9
docs/e2e.md, README.md                                                   # Task 9
docs/phases/phase-1d.md                                                  # Task 12
```

## Pre-flight (controller, before Task 1)

- [ ] `gh api user --jq .login` prints `jellalshadows` (else `gh auth switch -u jellalshadows`).
- [ ] Recreate the zizmor venv in the session scratchpad (`python -m venv …; pip install zizmor==1.30.1`) and give its path to dispatches as `ZIZMOR`.
- [ ] Ledger the rulings D-1 to D-14.

---

### Task 1: Decisions first — ADRs 0026–0027, ADR-0018 amendment, spec amendment A8

**Files:**
- Create: `docs/adr/0026-automation-app.md`, `docs/adr/0027-end-to-end-harness.md`
- Modify: `docs/adr/README.md`, `docs/adr/0018-plan-fingerprint-and-gate-trust.md`, `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`

**Interfaces:**
- Produces: the decisions Tasks 2–12 implement and cite.

- [ ] **Step 1: Create the branch**

Run: `git switch main && git pull --ff-only && git switch -c feat/phase-1d-e2e-harness`

- [ ] **Step 2: Write `docs/adr/0026-automation-app.md`**

````markdown
# 0026. An automation App for releases, dependency updates and the end-to-end harness

- Status: Accepted
- Date: 2026-10-13
- Spec: [§7.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#73-two-github-apps-and-three-secret-scopes), [§8.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#85-level-4-e2e), [§9.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#91-releases)

## Context

Three jobs need to push branches and open or merge pull requests: release-please in `idp-engine`, Renovate in the engine and claims repos, and the end-to-end harness in `idp-claims-e2e`. Pull requests opened with a workflow's `GITHUB_TOKEN` do not trigger workflows, so a release PR would get no CI, and letting Actions open pull requests needs the org setting that also lets Actions approve them, which spec §9.2 keeps off because it weakens required reviews. The reader App is read-only by design; the writer App's key is the apply key and lives only in the claims repos' `idp-write` environments.

## Decision

- A third App, `<org>-bot`, is created with `idp bootstrap app --role bot` from the manifest `bootstrap/apps/bot.json`. Repository permissions: actions write, checks read, contents write, issues write, metadata read, pull requests write, statuses read, workflows write; organization permission: members read; no events.
- It is installed on **only** `idp-engine`, `idp-claims` and `idp-claims-e2e`.
- Its key lives in `idp-engine` environments with deployment branch `main` (`idp-e2e` now; the release and Renovate environments in Phase 1e). Each job mints a token for the repositories and permissions it needs (ADR-0023).
- It is not a bypass actor anywhere: its pull requests in claims repos pass `idp-gate` like anyone's.

## Consequences

### Positive

- Its pull requests trigger CI, and no org setting is weakened.
- One identity for automation, limited to three repositories, with its permissions reviewable in the repo.

### Negative / costs

- One more key to rotate (bootstrap runbook).
- It can merge pull requests in `idp-claims` once Renovate uses it (Phase 1e). The `idp-gate` check and the approval of destructive changes still apply; requiring a human review on claims repos remains the owner's decision (ADR-0022).

## Alternatives considered

- **`GITHUB_TOKEN` with "Allow GitHub Actions to create and approve pull requests".** Rejected: release PRs would not trigger CI, and the setting weakens required reviews (spec §9.2).
- **A personal access token.** Rejected: personal, long-lived, and spec §9.1 rules out a PAT for releases.
- **The hosted Renovate App.** Rejected by the owner: a third party with write access to the repos.
````

- [ ] **Step 3: Write `docs/adr/0027-end-to-end-harness.md`**

````markdown
# 0027. The end-to-end harness

- Status: Accepted
- Date: 2026-10-13
- Spec: [§8.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#85-level-4-e2e), [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main)

## Context

Phase 1c proved the pipelines once, by hand, in `idp-claims-e2e`. A repeatable check has to run without a person, including the approval of a destructive change. GitHub lets only a required reviewer approve a deployment, and reviewers are users or teams, never Apps.

## Decision

- `cmd/idp-e2e` (over `internal/e2e`, not part of the released CLI) drives `idp-claims-e2e` through the real pipelines at the engine commit under test:
  1. pins the four caller workflows to that commit, through a PR;
  2. creates a Group `e2e-r<run>-team` and a Component `e2e-r<run>-app` through a PR, checks the plan comment, merges, waits for the automatic reconcile and asserts the repository, team, memberships and environment through the API;
  3. updates the description and removes every topic, and asserts them;
  4. changes only the case of a member login, and expects `No infrastructure changes.`;
  5. runs the drift check and expects no drift issue;
  6. deletes the claims through a PR, expects the destructive warning, approves the `idp-approval` deployment of that reconcile run only, and asserts the repository and team are gone;
  7. runs the drift check again.
- Names are unique per run and always start with `e2e-`. The harness refuses any other name, touches only its own claim files, and never deletes a GitHub resource directly: after a failure it deletes its claims through the platform and reports both errors.
- It runs nightly at 03:41 UTC and on demand from `idp-engine` `main`, in environment `idp-e2e` (deployment branch `main`), one run at a time (concurrency group `idp-e2e`). Release PRs join in Phase 1e.
- Credentials, both only in `idp-e2e`:
  - the bot App token (ADR-0026), minted for `idp-claims-e2e` only with actions write, checks read, contents write, issues read, members read, pull requests write and workflows write;
  - the approver token: a fine-grained token of the second account, an org member and, in `idp-claims-e2e` only, a second `idp-approval` reviewer. It is limited to `idp-claims-e2e` with Actions read and Deployments write.
- Not covered, and proven by the Phase 1c smoke: a partial apply failure, the `recover` workflow, pull requests from forks.

## Consequences

### Positive

- Every engine change can be proven against the real org, approval path included.
- A leaked approver token can approve deployments of the test repo and nothing else.

### Negative / costs

- A test account and a token that expires; the runbook covers rotation.
- A run takes tens of minutes of CI, creates and deletes real `e2e-` resources, and shares the org's rate limit.
- `idp-claims-e2e` belongs to the harness: people should not commit there while it runs.

## Alternatives considered

- **The owner's token.** Rejected: an org owner's token in CI is a takeover of the org if it leaks.
- **Approving by hand.** Rejected by the owner: a nightly run must finish on its own.
- **A custom deployment protection App.** Rejected: more machinery than a test account for one approval.
````

- [ ] **Step 4: Add the rows to `docs/adr/README.md`** (after the `0025` row)

```markdown
| 0026 | [An automation App for releases, dependency updates and the end-to-end harness](0026-automation-app.md) | Accepted |
| 0027 | [The end-to-end harness](0027-end-to-end-harness.md) | Accepted |
```

- [ ] **Step 5: Amend `docs/adr/0018-plan-fingerprint-and-gate-trust.md`**

In "Decision", add this bullet at the end of the list:

```markdown
- The comment is trusted only when it was never edited, or when the Actions bot itself made the last edit (GraphQL `editor` is null, or the Bot `github-actions`). *(Amended 2026-10-13, Phase 1d.)*
```

In "Negative / costs", in the bullet that starts "**Accepted risk**", replace "(1) People with write access can edit the bot's comment, and the author stays `github-actions[bot]`." with "(1) People with write access can edit the bot's comment, and the author stays `github-actions[bot]`; closed in Phase 1d by the editor check above."

Replace the "Follow-up" bullet (it starts "Deferred to Phase 1d by the Phase 1c plan") with:

```markdown
- Resolved in Phase 1d: the gate ignores a plan comment that anyone but the Actions bot last edited. The second route (a workflow on a same-repo pull request branch writing as `github-actions[bot]`) remains accepted.
```

- [ ] **Step 6: Apply amendment A8 to the spec** (exact replacements with Edit)

1. §6.2 step 1.5, old:

```markdown
      The gate trusts only the newest `<!-- idp-plan -->` comment written by
      `github-actions[bot]` on the merged PR, and only when its marker `sha` equals
      the PR's head SHA (ADR-0018). *(Amendment A3, 2026-10-10.)*
```

new:

```markdown
      The gate trusts only the newest `<!-- idp-plan -->` comment written by
      `github-actions[bot]` on the merged PR, and only when its marker `sha` equals
      the PR's head SHA (ADR-0018). *(Amendment A3, 2026-10-10.)* It also ignores
      that comment when anyone but the Actions bot made its last edit (ADR-0018).
      *(Amendment A8, 2026-10-13.)*
```

2. §7.3 table: add two rows right after the row that starts `| Approval |`:

```markdown
| `idp-bot` App key | Pull requests, contents and workflows on the engine and claims repos; members read | Only in `idp-engine` environments with deployment branch `main` (`idp-e2e`; the release and Renovate environments in Phase 1e); installed on `idp-engine`, `idp-claims` and `idp-claims-e2e` only (ADR-0026). *(Amendment A8.)* |
| E2E approver token | Approves `idp-approval` deployments in `idp-claims-e2e` only | Only in `idp-engine` environment `idp-e2e` (ADR-0027). *(Amendment A8.)* |
```

3. §8.5, old:

```markdown
`idp-claims-e2e` uses `archiveOnDestroy: false`. The harness runs nightly and on
engine release PRs, not on every PR, to protect the rate limit and CI minutes.
```

new:

```markdown
`idp-claims-e2e` uses `archiveOnDestroy: false`. The harness runs nightly and on
engine release PRs, not on every PR, to protect the rate limit and CI minutes.
The harness is `cmd/idp-e2e`, run by the engine workflow `e2e.yaml` in environment
`idp-e2e`. Besides the steps above, it removes every topic, changes only the case of
a member login (expecting no change) and runs drift. It approves the destructive
reconcile with the token of a test account that is a second `idp-approval` reviewer
in `idp-claims-e2e` only, because only a person can approve a deployment (ADR-0027).
Release PRs join in Phase 1e. *(Amendment A8, 2026-10-13.)*
```

4. §9.2, old:

```markdown
- **`idp bootstrap app`:** runs GitHub's manifest flow through a local callback
  server. A person still confirms in the browser, as GitHub requires. The manifests
  are versioned in `idp-engine/bootstrap/apps/{reader,writer}.json`, so the
  permissions are reviewable code.
```

new:

```markdown
- **`idp bootstrap app`:** runs GitHub's manifest flow through a local callback
  server. A person still confirms in the browser, as GitHub requires. The manifests
  are versioned in `idp-engine/bootstrap/apps/{reader,writer,bot}.json`, so the
  permissions are reviewable code. The `bot` App opens pull requests for releases,
  Renovate and the E2E harness (ADR-0026). `apply` and `check` accept several
  approvers. *(Amendment A8, 2026-10-13.)*
```

5. §10, old:

```markdown
Phase 1 is delivered as four plans, each shipping working software: **1a** claims → render (validation, the GitHub stack, the `github/group` and `github/component` modules); **1b** the pipeline subcommands (`diff`, `plan-summary`, `gate`, `comment`, `issue`, `wet-push`, `encryption-env`, drift-ready `bootstrap check`); **1c** the reusable workflows and a live smoke run in `idp-claims-e2e`; **1d** the E2E harness and the `v0.1.0` release. *(2026-10-09, re-split 2026-10-10.)*
```

new:

```markdown
Phase 1 is delivered as five plans, each shipping working software: **1a** claims → render (validation, the GitHub stack, the `github/group` and `github/component` modules); **1b** the pipeline subcommands (`diff`, `plan-summary`, `gate`, `comment`, `issue`, `wet-push`, `encryption-env`, drift-ready `bootstrap check`); **1c** the reusable workflows and a live smoke run in `idp-claims-e2e`; **1d** the E2E harness, its credentials and the hardening carried from 1b and 1c; **1e** releases (release-please, GoReleaser, attestations), Renovate, the `v0.1.0` release and onboarding `idp-claims`. *(2026-10-09, re-split 2026-10-10 and 2026-10-13.)*
```

- [ ] **Step 7: Check and commit**

Run: `rg -n "0026-|0027-" docs/adr/README.md` (both rows link existing files) and the owner naming check the controller gives you (it must print nothing).

```bash
git add docs/adr docs/superpowers/specs/2026-10-08-idp-on-actions-design.md
git commit -m "docs: record the e2e harness decisions (adr 0026-0027, spec amendment a8)"
```

---

### Task 2: The gate ignores plan comments edited by anyone but the bot

**Files:**
- Create: `internal/ghapi/graphql.go`, `internal/ghapi/graphql_test.go`
- Modify: `internal/fakegithub/fakegithub.go`, `internal/ghops/ghops.go`, `internal/cli/gate.go`, `internal/cli/gate_test.go`, `docs/cli.md`
- Test: `internal/ghops/` (a new `edit_test.go`)

**Interfaces:**
- Consumes: `ghapi.Client.Post(ctx, path string, in, out any) error`; `ghops.Repo`, `ghops.Comment`, `FindBotComment`; `trustedPRPlan` in `internal/cli/gate.go`; the gate test helpers `seedMergedPR`, `planComment`, `gateRun` and the constants `mergeSHA`, `prHead` in `internal/cli/gate_test.go`.
- Produces: `func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]any, out any) error`; `Comment.NodeID string` (`json:"node_id"`); `func (r Repo) EditedByOthers(ctx context.Context, nodeID string) (bool, string, error)`; fakegithub field `CommentEditors map[string]map[string]any` (comment node id → GraphQL editor object, absent or nil = never edited) and a `POST /graphql` route that is not recorded as a write.

- [ ] **Step 1: Write the failing tests**

`internal/ghapi/graphql_test.go`:

```go
package ghapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGraphQLDecodesData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if r.Method != http.MethodPost || r.URL.Path != "/graphql" {
			t.Errorf("%s %s, want POST /graphql", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Variables["id"] != "C1" || !strings.Contains(in.Query, "node(") {
			t.Errorf("body %+v, %v", in, err)
		}
		_, _ = w.Write([]byte(`{"data":{"node":{"editor":null}}}`))
	}))
	defer srv.Close()
	var out struct {
		Node struct {
			Editor *struct{ Login string } `json:"editor"`
		} `json:"node"`
	}
	if err := New(srv.URL, "t").GraphQL(context.Background(), `query($id: ID!) { node(id: $id) { id } }`, map[string]any{"id": "C1"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Node.Editor != nil {
		t.Errorf("editor = %+v, want nil", out.Node.Editor)
	}
}

func TestGraphQLReportsErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"Could not resolve to a node"},{"message":"second"}]}`))
	}))
	defer srv.Close()
	err := New(srv.URL, "t").GraphQL(context.Background(), "query { x }", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Could not resolve to a node; second") {
		t.Errorf("err = %v", err)
	}
}
```

`internal/ghops/edit_test.go`:

```go
package ghops

import (
	"context"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

func TestEditedByOthers(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	fake.CommentEditors = map[string]map[string]any{
		"BOT":    {"login": "github-actions", "__typename": "Bot"},
		"PERSON": {"login": "mallory", "__typename": "User"},
		"OTHERBOT": {"login": "renovate", "__typename": "Bot"},
	}
	repo := Repo{API: ghapi.New(fake.URL, "t"), Owner: "acme", Name: "idp-claims"}
	tests := []struct {
		node   string
		edited bool
		editor string
	}{
		{"NEVER", false, ""},
		{"BOT", false, ""},
		{"PERSON", true, "mallory"},
		{"OTHERBOT", true, "renovate"},
	}
	for _, tt := range tests {
		edited, editor, err := repo.EditedByOthers(context.Background(), tt.node)
		if err != nil || edited != tt.edited || editor != tt.editor {
			t.Errorf("%s: EditedByOthers = %v, %q, %v; want %v, %q", tt.node, edited, editor, err, tt.edited, tt.editor)
		}
	}
	if len(fake.Writes) != 0 {
		t.Errorf("a GraphQL query is a read, got writes %v", fake.Writes)
	}
}
```

Add to `internal/cli/gate_test.go`:

```go
func TestGateIgnoresACommentEditedByAPerson(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	shown := plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}}}
	seedMergedPR(fake, map[string]any{"id": 1, "node_id": "IC_1", "body": planComment(t, prHead, shown), "user": map[string]any{"login": "github-actions[bot]", "type": "Bot"}})
	fake.CommentEditors = map[string]map[string]any{"IC_1": {"login": "mallory", "__typename": "User"}}
	code, decision, out := gateRun(t, fake, shown)
	if code != 0 || decision != "decision=approval" || !strings.Contains(out, "edited by mallory") {
		t.Errorf("exit %d, %q, %q: a comment a person edited must not be trusted", code, decision, out)
	}
}

func TestGateTrustsACommentTheBotEdited(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	shown := plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}}}
	seedMergedPR(fake, map[string]any{"id": 1, "node_id": "IC_1", "body": planComment(t, prHead, shown), "user": map[string]any{"login": "github-actions[bot]", "type": "Bot"}})
	fake.CommentEditors = map[string]map[string]any{"IC_1": {"login": "github-actions", "__typename": "Bot"}}
	code, decision, out := gateRun(t, fake, shown)
	if code != 0 || decision != "decision=auto" {
		t.Errorf("exit %d, %q, %q: the bot updates its own comment on every push", code, decision, out)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ghapi/ ./internal/ghops/ ./internal/cli/ -run 'GraphQL|EditedByOthers|TestGateIgnoresACommentEdited|TestGateTrustsACommentTheBot'`
Expected: FAIL to compile (`undefined: GraphQL`, `fake.CommentEditors undefined`, `repo.EditedByOthers undefined`).

- [ ] **Step 3: Implement**

`internal/ghapi/graphql.go`:

```go
package ghapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GraphQL runs one query against the GraphQL endpoint and decodes its "data"
// into out. A response that carries "errors" is an error, even with data.
func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	var resp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := c.Post(ctx, "/graphql", map[string]any{"query": query, "variables": variables}, &resp); err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		msgs := make([]string, len(resp.Errors))
		for i, e := range resp.Errors {
			msgs[i] = e.Message
		}
		return fmt.Errorf("graphql: %s", strings.Join(msgs, "; "))
	}
	if out == nil || len(resp.Data) == 0 || string(resp.Data) == "null" {
		return nil
	}
	return json.Unmarshal(resp.Data, out)
}
```

`internal/fakegithub/fakegithub.go`: add the exported field

```go
	// CommentEditors maps a comment node id to the GraphQL editor of its last
	// edit; a missing or nil entry means the comment was never edited.
	CommentEditors map[string]map[string]any
```

and route `POST /graphql` before any write bookkeeping: decode `{"query", "variables"}`, take `variables.id`, and answer `{"data":{"node":{"editor": <CommentEditors[id] or null>}}}`. It must not be appended to `Writes` or `Requests` (a query is a read). Read the file to place it where `ServeHTTP` handles methods; keep `CommentEditors` nil-safe.

`internal/ghops/ghops.go`: add `NodeID string \`json:"node_id"\`` to `Comment`, and:

```go
// actionsBotGraphQL is the Actions bot's login in GraphQL, which has no "[bot]" suffix.
const actionsBotGraphQL = "github-actions"

// EditedByOthers reports whether anyone but the Actions bot made the last edit
// of a comment, and who (ADR-0018). An unedited comment, and one the bot itself
// updated, are not edited by others.
func (r Repo) EditedByOthers(ctx context.Context, nodeID string) (bool, string, error) {
	const q = `query($id: ID!) { node(id: $id) { ... on IssueComment { editor { login __typename } } } }`
	var out struct {
		Node *struct {
			Editor *struct {
				Login    string `json:"login"`
				Typename string `json:"__typename"`
			} `json:"editor"`
		} `json:"node"`
	}
	if err := r.API.GraphQL(ctx, q, map[string]any{"id": nodeID}, &out); err != nil {
		return false, "", fmt.Errorf("read the last editor of comment %s: %w", nodeID, err)
	}
	if out.Node == nil {
		return false, "", fmt.Errorf("comment %s not found", nodeID)
	}
	e := out.Node.Editor
	if e == nil || (e.Typename == "Bot" && e.Login == actionsBotGraphQL) {
		return false, "", nil
	}
	return true, e.Login, nil
}
```

`internal/cli/gate.go` `trustedPRPlan`: right after the `if c == nil { … }` block, add:

```go
	edited, editor, err := repo.EditedByOthers(ctx, c.NodeID)
	if err != nil {
		return nil, "", err
	}
	if edited {
		return nil, fmt.Sprintf("the plan comment on PR #%d was edited by %s", pr.Number, editor), nil
	}
```

The fake answers a node for any id (with a null editor unless configured), so the existing gate tests keep passing without a `node_id`.

`docs/cli.md`, section `gate`: append to the paragraph that starts "A fingerprint is trusted only when" the sentence: "A comment that anyone but the Actions bot last edited is not trusted either; the gate reads its last editor through the GraphQL API ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md))."

- [ ] **Step 4: Verify**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: PASS; gofmt prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/ghapi internal/fakegithub internal/ghops internal/cli/gate.go internal/cli/gate_test.go docs/cli.md
git commit -m "feat(gate): ignore plan comments edited by anyone but the bot"
```

---

### Task 3: Labelled issues must be the bot's; `wet-push` validates branch names

**Files:**
- Modify: `internal/ghops/write.go`, `internal/ghops/write_test.go`, `internal/fakegithub/fakegithub.go`, `internal/wetpush/wetpush.go`, `internal/wetpush/wetpush_test.go`

**Interfaces:**
- Consumes: `ghops.FindOpenIssue`, `OpenIssue`, `CloseIssue`, `ActionsBot`; fakegithub `Lists`, `Writes`, `Actor`; `wetpush.Pusher.Sync`.
- Produces: `ghops.Issue.User struct{Login, Type string}` (`json:"user"`); `FindOpenIssue` matches only issues authored by `github-actions[bot]` (type `Bot`); fakegithub-created issues carry `user: Actor`; `wetpush` rejects invalid branch names before any API call.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ghops/write_test.go` (it already builds a `Repo` against `fakegithub`; reuse its construction helper if it has one, otherwise `Repo{API: ghapi.New(fake.URL, "t"), Owner: "acme", Name: "idp-claims"}`):

```go
func TestFindOpenIssueIgnoresIssuesByOthers(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	fake.Lists["/repos/acme/idp-claims/issues"] = []any{map[string]any{
		"number": 3, "title": "Drift detected", "state": "open",
		"labels": []any{map[string]any{"name": "drift"}},
		"user":   map[string]any{"login": "mallory", "type": "User"},
	}}
	repo := Repo{API: ghapi.New(fake.URL, "t"), Owner: "acme", Name: "idp-claims"}
	got, err := repo.FindOpenIssue(context.Background(), "drift", "Drift detected")
	if err != nil || got != nil {
		t.Fatalf("FindOpenIssue = %+v, %v: an issue by a person is never the platform's", got, err)
	}
	number, created, err := repo.OpenIssue(context.Background(), "drift", "Drift detected", "body")
	if err != nil || !created || number == 3 {
		t.Errorf("OpenIssue = %d, %v, %v: it must open its own issue, not edit #3", number, created, err)
	}
	_, closed, err := repo.CloseIssue(context.Background(), "drift", "Drift detected", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range fake.Writes {
		if strings.Contains(w, "/issues/3") {
			t.Errorf("the platform touched the person's issue: %s (closed=%v)", w, closed)
		}
	}
}
```

Every existing test in `internal/ghops` and `internal/cli` that seeds an open issue the platform must find gets `"user": map[string]any{"login": "github-actions[bot]", "type": "Bot"}` in its seed.

Add to `internal/wetpush/wetpush_test.go`:

```go
func TestSyncRejectsInvalidBranchNames(t *testing.T) {
	for _, b := range []string{"", "/wet", "wet/", "a..b", "a b", "a%2Fb", "x.lock", "a//b", "-x", ".x"} {
		p := Pusher{API: ghapi.New("http://127.0.0.1:1", "t"), Owner: "acme", Repo: "idp-claims", Branch: b}
		if _, err := p.Sync(context.Background(), t.TempDir(), []string{"tfstate"}, "m"); err == nil || !strings.Contains(err.Error(), "invalid branch name") {
			t.Errorf("Sync with branch %q: err = %v, want an invalid branch name error before any API call", b, err)
		}
	}
	for _, b := range []string{"wet", "feat/x", "e2e/38314547633/pin", "release-please--branches--main"} {
		if err := validateBranch(b); err != nil {
			t.Errorf("validateBranch(%q) = %v", b, err)
		}
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ghops/ ./internal/wetpush/ -run 'IgnoresIssuesByOthers|InvalidBranch'`
Expected: FAIL (the person's issue is found and edited; `undefined: validateBranch`).

- [ ] **Step 3: Implement**

`internal/ghops/write.go`: add to `Issue`

```go
	User struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
```

and in `FindOpenIssue` require `i.User.Login == ActionsBot && i.User.Type == "Bot"` together with the existing checks; extend its doc comment: "Only issues the Actions bot opened count: a person's issue with the same label and title is never edited, closed or counted."

`internal/fakegithub/fakegithub.go`: issues created by `POST …/issues` carry `"user": f.Actor`.

`internal/wetpush/wetpush.go`:

```go
var branchSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// validateBranch accepts the branch names the pipelines use, so a name never
// needs escaping in an API path (Phase 1b M7).
func validateBranch(b string) error {
	parts := strings.Split(b, "/")
	for _, p := range parts {
		if !branchSegment.MatchString(p) {
			return fmt.Errorf("invalid branch name %q", b)
		}
	}
	if strings.Contains(b, "..") || strings.HasSuffix(b, ".lock") {
		return fmt.Errorf("invalid branch name %q", b)
	}
	return nil
}
```

Call `validateBranch(p.Branch)` first thing in `Sync` and return its error.

- [ ] **Step 4: Verify**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: PASS; gofmt prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/ghops internal/fakegithub internal/cli internal/wetpush
git commit -m "fix(ghops): count only issues the bot opened; validate wet-push branch names"
```

---

### Task 4: Topics through `github_repository_topics`

**Files:**
- Modify: `modules/github/component/main.tf`, `modules/github/component/tests/component.tftest.hcl`

**Interfaces:**
- Consumes: module variable `topics` (`list(string)`, default `[]`).
- Produces: resource `github_repository_topics.this` with `repository = github_repository.this.name`, `topics = var.topics`; `github_repository.this` no longer sets `topics`.

- [ ] **Step 1: Write the failing test**

In `modules/github/component/tests/component.tftest.hcl`, replace the assert of `repository_is_public_and_follows_the_claim` that reads `tolist(github_repository.this.topics)` with:

```hcl
  assert {
    condition     = tolist(github_repository_topics.this.topics) == tolist(["java", "orders"])
    error_message = "topics must follow the claim, through github_repository_topics"
  }
```

and append:

```hcl
run "topics_can_be_cleared" {
  command = plan

  variables {
    topics = []
  }

  assert {
    condition     = length(github_repository_topics.this.topics) == 0
    error_message = "an empty topic list must be sent as [], which clears the topics; github_repository's Optional+Computed topics would keep the old ones (phase 1a minor #3)"
  }
  assert {
    condition     = github_repository_topics.this.repository == "api"
    error_message = "the topics resource must target the component's repository"
  }
}
```

- [ ] **Step 2: Run it to see it fail**

Run (from `modules/github/component`, with the repo's lock file copied as CI does):

```bash
cp ../../../lockfiles/github.terraform.lock.hcl .terraform.lock.hcl
"$HOME/.idp/tools/tofu-1.12.6/tofu.exe" init -input=false -lockfile=readonly
"$HOME/.idp/tools/tofu-1.12.6/tofu.exe" test
```

Expected: FAIL (`github_repository_topics.this` is not declared). Delete the copied `.terraform.lock.hcl` and `.terraform/` afterwards (they are not committed; check with `git status`).

- [ ] **Step 3: Implement** — in `modules/github/component/main.tf` remove the line `topics                 = var.topics` from `github_repository.this` (realign the `=` of the remaining attributes as `tofu fmt` wants) and add after that resource:

```hcl
# Topics have their own resource: github_repository's topics is Optional and
# Computed, so an empty list would leave the old topics in place.
resource "github_repository_topics" "this" {
  repository = github_repository.this.name
  topics     = var.topics
}
```

- [ ] **Step 4: Verify** — `tofu fmt -check -recursive` and `tofu test` in the module directory pass (same commands as Step 2); clean the copied lock file and `.terraform/`. `go test ./...` still passes (the render golden is unchanged: the module call does not change).

- [ ] **Step 5: Commit**

```bash
git add modules/github/component
git commit -m "fix(modules): manage component topics with github_repository_topics"
```

---

### Task 5: Bootstrap — several approvers, and the bot App

**Files:**
- Create: `bootstrap/apps/bot.json`
- Modify: `internal/bootstrap/config.go`, `internal/bootstrap/desired.go`, `internal/bootstrap/appflow.go`, `internal/cli/bootstrap.go`, their tests, `docs/cli.md`, `docs/runbooks/bootstrap.md`

**Interfaces:**
- Consumes: `bootstrap.Config`, `Params`, `ParseParams`, `Environments`, `UserID`, `AppRole`, `Manifest`; `runBootstrapRepo` and the `bootstrap app` command in `internal/cli/bootstrap.go`.
- Produces: `Config.ApproverIDs []int64` (replaces `ApproverID`); `Params.ApproverIDs []int64` (`json:"approverIds"`), with `ParseParams` still reading a legacy `{"approverId": N}`; `RoleBot AppRole = "bot"`; `--approver` repeatable; `func installAdvice(role bootstrap.AppRole) string`.

- [ ] **Step 1: Write the failing tests**

In `internal/bootstrap` tests: replace every `ApproverID: 42` with `ApproverIDs: []int64{42}`; update the expected IDP_BOOTSTRAP string to `{"approverIds":[42],"reader":{"id":1,"clientId":"Iv-reader","slug":"acme-reader"},"writer":{"id":2,"clientId":"Iv-writer","slug":"acme-writer"}}`; update the "every problem is reported" expectation from "approver id must be positive" to "at least one approver is required". Add:

```go
func TestParseParamsReadsTheSingleApproverOfOlderValues(t *testing.T) {
	got, err := ParseParams(`{"approverId":42,"reader":{"id":1,"clientId":"Iv-reader","slug":"acme-reader"},"writer":{"id":2,"clientId":"Iv-writer","slug":"acme-writer"}}`)
	if err != nil || !slices.Equal(got.ApproverIDs, []int64{42}) {
		t.Fatalf("ParseParams = %+v, %v: a value written before approver lists must still be read", got, err)
	}
	if !strings.Contains(got.String(), `"approverIds":[42]`) || strings.Contains(got.String(), `"approverId":`) {
		t.Errorf("String() = %s: the next apply writes the list form", got.String())
	}
}

func TestParseParamsRejectsBothApproverFields(t *testing.T) {
	if _, err := ParseParams(`{"approverId":1,"approverIds":[1],"reader":{"id":1,"clientId":"a","slug":"b"},"writer":{"id":2,"clientId":"c","slug":"d"}}`); err == nil {
		t.Error("approverId and approverIds together must be rejected")
	}
}

func TestApproverValidation(t *testing.T) {
	base := Config{Org: "acme", ClaimsRepo: "idp-claims", Reader: AppCredentials{ID: 1, ClientID: "a", Slug: "b"}, Writer: AppCredentials{ID: 2, ClientID: "c", Slug: "d"}}
	for _, tt := range []struct {
		ids  []int64
		want string
	}{
		{nil, "at least one approver is required"},
		{[]int64{0}, "approver ids must be positive"},
		{[]int64{42, 7, 42}, "approver 42 is listed twice"},
	} {
		c := base
		c.ApproverIDs = tt.ids
		if err := c.ValidateCheck(); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ApproverIDs %v: err = %v, want %q", tt.ids, err, tt.want)
		}
	}
}

func TestEnvironmentsListEveryApproverSorted(t *testing.T) {
	for _, e := range Environments(Config{ApproverIDs: []int64{42, 7}}) {
		if e.Name == EnvApproval && !slices.Equal(e.ReviewerIDs, []int64{7, 42}) {
			t.Errorf("idp-approval reviewers = %v, want [7 42]", e.ReviewerIDs)
		}
	}
}

func TestBotManifest(t *testing.T) {
	// Follow the shape of the existing reader/writer manifest test: build the
	// manifest for RoleBot and decode its default_permissions.
	want := map[string]string{"actions": "write", "checks": "read", "contents": "write", "issues": "write", "members": "read", "metadata": "read", "pull_requests": "write", "statuses": "read", "workflows": "write"}
	got := botManifestPermissions(t) // helper: Manifest("acme", RoleBot, "http://127.0.0.1:1/cb") decoded to default_permissions
	if !maps.Equal(got, want) {
		t.Errorf("bot permissions = %v, want %v (ADR-0026)", got, want)
	}
}
```

Write `botManifestPermissions` in the same test file by decoding whatever `Manifest` returns (read `appflow.go` and its test to see the type) into `struct{ DefaultPermissions map[string]string \`json:"default_permissions"\` }`; also assert the manifest's name is `acme-bot`.

In `internal/cli` bootstrap tests add:

```go
func TestInstallAdvice(t *testing.T) {
	if got := installAdvice(bootstrap.RoleBot); !strings.Contains(got, "ONLY these repositories: idp-engine, idp-claims, idp-claims-e2e") {
		t.Errorf("bot advice = %q", got)
	}
	if got := installAdvice(bootstrap.RoleWriter); !strings.Contains(got, "all repositories") {
		t.Errorf("writer advice = %q", got)
	}
}
```

and a bootstrap `apply` test with two approvers, modelled on the existing apply test: `--approver jellalshadows --approver <second account>` with both users seeded in the fake (`/users/<login>` → `{"id": N}`); assert the environment write for `idp-approval` lists both reviewer ids, and that `--role bot` is accepted by `bootstrap app` flag validation while `--role admin` fails with "--role must be reader, writer or bot".

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/bootstrap/ ./internal/cli/ -run 'Approver|Environments|BotManifest|InstallAdvice|Bootstrap'`
Expected: FAIL to compile (`ApproverIDs`, `RoleBot`, `installAdvice` undefined).

- [ ] **Step 3: Implement**

`bootstrap/apps/bot.json` (same shape as `reader.json`):

```json
{
  "description": "IDP bot: opens and merges pull requests for releases, dependency updates and the end-to-end harness. Install it only on idp-engine, idp-claims and idp-claims-e2e.",
  "public": false,
  "default_permissions": {
    "actions": "write",
    "checks": "read",
    "contents": "write",
    "issues": "write",
    "members": "read",
    "metadata": "read",
    "pull_requests": "write",
    "statuses": "read",
    "workflows": "write"
  },
  "default_events": []
}
```

`internal/bootstrap/appflow.go`: add `RoleBot AppRole = "bot"` and accept it wherever reader and writer are accepted.

`internal/bootstrap/config.go`:
- `Config.ApproverIDs []int64` replaces `ApproverID` (doc: "GitHub user ids of the people who approve runs; sorted, unique, at least one").
- In `baseProblems`, replace the approver check with: none → "at least one approver is required"; any id ≤ 0 → "approver ids must be positive"; a repeated id → `fmt.Sprintf("approver %d is listed twice", id)`.
- `Params`:

```go
type Params struct {
	ApproverIDs []int64     `json:"approverIds"`
	Reader      AppIdentity `json:"reader"`
	Writer      AppIdentity `json:"writer"`
}
```

- `Config.Params()` copies a sorted copy of `ApproverIDs`; `Params.Config(org, repo)` copies them back.
- `ParseParams`:

```go
// ParseParams reads an IDP_BOOTSTRAP value; unknown fields are rejected. A value
// written before approver lists existed carries one "approverId": it is read as
// a one-element list, and the next apply rewrites it.
func ParseParams(s string) (Params, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	var raw struct {
		Params
		ApproverID int64 `json:"approverId"`
	}
	if err := dec.Decode(&raw); err != nil {
		return Params{}, fmt.Errorf("parse %s: %w", VarParams, err)
	}
	p := raw.Params
	if raw.ApproverID != 0 {
		if len(p.ApproverIDs) > 0 {
			return Params{}, fmt.Errorf("parse %s: approverId and approverIds are both set", VarParams)
		}
		p.ApproverIDs = []int64{raw.ApproverID}
	}
	return p, nil
}
```

`internal/bootstrap/desired.go`: the `idp-approval` environment's `ReviewerIDs` is a sorted copy of `cfg.ApproverIDs`.

`internal/cli/bootstrap.go`:
- a repeatable flag:

```go
// loginList collects a repeatable --approver flag.
type loginList []string

func (l *loginList) String() string     { return strings.Join(*l, ",") }
func (l *loginList) Set(v string) error { *l = append(*l, v); return nil }
```

  registered as `fs.Var(&approvers, "approver", "login of a person who approves runs (repeat for several)")`, replacing the string flag; the `--params-env` conflict test becomes `len(approvers) > 0`; resolve every login with `bootstrap.UserID` into `cfg.ApproverIDs`.
- `--role` validation accepts `bot`, with the message "--role must be reader, writer or bot".
- `installAdvice`:

```go
// installAdvice tells the owner how to install a new App: the reader and the
// writer need every repository; the bot only the engine and the claims repos
// (ADR-0026).
func installAdvice(role bootstrap.AppRole) string {
	if role == bootstrap.RoleBot {
		return "Install it on ONLY these repositories: idp-engine, idp-claims, idp-claims-e2e."
	}
	return "Install it on all repositories."
}
```

  printed next to the install link that `bootstrap app` already prints (replace any hard-coded "all repositories" wording there).

`docs/cli.md`: in the `bootstrap` synopsis write `--approver LOGIN [--approver LOGIN]...` for `apply` and `check`, and `--role reader|writer|bot` for `app`; under `bootstrap app` add "`bot` creates the automation App of [ADR-0026](adr/0026-automation-app.md); install it on only the engine and claims repos."; under `apply`, "`--approver`: a person who approves runs; repeat it for several (each becomes an `idp-approval` reviewer)."

`docs/runbooks/bootstrap.md`: add a section "The bot App" (create with `idp bootstrap app --org <org> --role bot`, confirm in the browser, install on only `idp-engine`, `idp-claims`, `idp-claims-e2e`; rotate like the other keys) and, in the `apply` section, "Repeat `--approver` for each reviewer. `idp-claims-e2e` also lists the E2E approver account (ADR-0027)."

- [ ] **Step 4: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass.

- [ ] **Step 5: Commit**

```bash
git add bootstrap/apps/bot.json internal/bootstrap internal/cli docs/cli.md docs/runbooks/bootstrap.md
git commit -m "feat(bootstrap): accept several approvers and create the bot app"
```

---

### Task 6: Contract-test hardening

**Files:**
- Modify: `internal/workflowspec/load_test.go`, `internal/workflowspec/rules_test.go`, `internal/workflowspec/common_test.go`, `internal/workflowspec/reconcile_test.go`, `internal/workflowspec/drift_test.go`, `internal/workflowspec/templates_test.go`, `docs/pipelines.md`

**Interfaces:**
- Consumes: `reusableWorkflows(t)`, `checkUses`, `workflowFiles`, `readYAML`, `mustStep`, `glob`, `templateNames`.
- Produces: `var reusableNames = []string{"drift.yaml", "pr.yaml", "reconcile.yaml", "recover.yaml"}`; `var internalWorkflows = []string{"ci.yaml", "e2e.yaml"}`; `reusableWorkflows(t)` built from `reusableNames`; `func anchors(n *yaml.Node) []int`.

- [ ] **Step 1: Write the failing tests**

`internal/workflowspec/common_test.go`:

```go
func TestEveryEngineWorkflowIsClassified(t *testing.T) {
	known := append(slices.Clone(reusableNames), internalWorkflows...)
	for _, f := range glob(t, ".github/workflows/*.yaml") {
		if !slices.Contains(known, path.Base(f)) {
			t.Errorf("%s is neither a reusable workflow nor an internal one: add it to reusableNames or internalWorkflows", f)
		}
	}
}

func TestWorkflowsUseNoAnchors(t *testing.T) {
	for _, f := range append(workflowFiles(t), actionFiles(t)...) {
		if lines := anchors(readYAML(t, f, &struct{}{})); len(lines) > 0 {
			t.Errorf("%s: YAML anchors or aliases at lines %v; the contract rules read node values and cannot follow an alias", f, lines)
		}
	}
}

func TestAnchors(t *testing.T) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("a: &x 1\nb: *x\nc: 2\n"), &doc); err != nil {
		t.Fatal(err)
	}
	if got := anchors(&doc); !slices.Equal(got, []int{1, 2}) {
		t.Errorf("anchors = %v, want [1 2]", got)
	}
}
```

`internal/workflowspec/rules_test.go` `TestCheckUses`, add the case:

```go
		{"another repo's workflow marked unreleased", "acme/other/.github/workflows/x.yaml@" + sha, "# unreleased", false},
```

`internal/workflowspec/reconcile_test.go` and `drift_test.go`: in the tests that read each plan step (`TestApplyCommitsTheRenderOnlyAfterASuccessfulApply` or the plan-token test for reconcile; `TestDriftPlansTheWetRenderWithTheReaderToken`'s successor for drift — read the files to pick the test that holds the `plan` step), add:

```go
	if strings.Contains(p.Run, "-refresh") {
		t.Errorf("the reconcile/drift plan must refresh (ADR-0025): no -refresh flag")
	}
```

(with `p` the `plan` step of that job).

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/`
Expected: FAIL to compile (`undefined: reusableNames`, `internalWorkflows`, `anchors`) and the new `TestCheckUses` case fails.

- [ ] **Step 3: Implement**

`internal/workflowspec/load_test.go`:

```go
// reusableNames are the engine workflows claims repos call (ADR-0021);
// internalWorkflows run only in idp-engine.
var (
	reusableNames     = []string{"drift.yaml", "pr.yaml", "reconcile.yaml", "recover.yaml"}
	internalWorkflows = []string{"ci.yaml", "e2e.yaml"}
)

func reusableWorkflows(t *testing.T) []string {
	t.Helper()
	out := make([]string, 0, len(reusableNames))
	for _, n := range reusableNames {
		out = append(out, ".github/workflows/"+n)
	}
	return out
}

// anchors returns the line of every YAML anchor or alias under n.
func anchors(n *yaml.Node) []int {
	var lines []int
	if n.Anchor != "" || n.Kind == yaml.AliasNode {
		lines = append(lines, n.Line)
	}
	for _, c := range n.Content {
		lines = append(lines, anchors(c)...)
	}
	return lines
}
```

(replace the existing `reusableWorkflows`; `TestTheReusableWorkflowSet` keeps comparing it with the templates.)

`internal/workflowspec/rules_test.go` `checkUses`: accept `# unreleased` only when `strings.Contains(value, "/idp-engine/.github/workflows/")`.

`docs/pipelines.md`, permissions table: add the row `| \`idp-gate\` (caller) | none | none | none |` after `pr / verdict`.

- [ ] **Step 4: Verify** — `go test ./... && go vet ./... && gofmt -l .`; actionlint; `"$ZIZMOR" --offline .github/workflows .github/actions/setup-idp/action.yaml examples/claims-repo/.github/workflows/*.yaml`.

- [ ] **Step 5: Commit**

```bash
git add internal/workflowspec docs/pipelines.md
git commit -m "test(workflowspec): close the remaining contract blind spots"
```

---

### Task 7: The harness scenario

**Files:**
- Create: `internal/e2e/names.go`, `internal/e2e/claims.go`, `internal/e2e/pin.go`, `internal/e2e/clock.go`, `internal/e2e/forge.go`, `internal/e2e/scenario.go`, `internal/e2e/names_test.go`, `internal/e2e/claims_test.go`, `internal/e2e/pin_test.go`, `internal/e2e/clock_test.go`, `internal/e2e/fake_test.go`, `internal/e2e/scenario_test.go`

**Interfaces:**
- Produces (package `e2e`): `Names{Group, Component string}`, `NewNames(run string) (Names, error)`, `(Names) guard() error`, `GroupPath()`, `ComponentPath()`; `Member{User, Role string}`, `GroupClaim(name, description string, members []Member) []byte`, `ComponentClaim(name, description, group string, topics []string) []byte`; `PinTemplates(templates map[string][]byte, sha string) (map[string][]byte, error)`; `Clock{Now() time.Time; Sleep(time.Duration)}`, `realClock`, `poll(ctx, Clock, interval, timeout time.Duration, what string, check func() (bool, error)) error`; `PR{Number int; HeadSHA string}`, `Run{ID int64; Status, Conclusion string}`, the `Forge` interface below; `Timeouts`, `DefaultTimeouts`, `Scenario` and `(*Scenario) Execute(ctx) error`.

- [ ] **Step 1: Write the failing tests**

`internal/e2e/names_test.go`:

```go
package e2e

import (
	"strings"
	"testing"
)

func TestNewNames(t *testing.T) {
	n, err := NewNames("38314547633")
	if err != nil || n.Group != "e2e-r38314547633-team" || n.Component != "e2e-r38314547633-app" {
		t.Fatalf("NewNames = %+v, %v", n, err)
	}
	if n.GroupPath() != "claims/groups/e2e-r38314547633-team.yaml" || n.ComponentPath() != "claims/components/e2e-r38314547633-app.yaml" {
		t.Errorf("paths %s, %s", n.GroupPath(), n.ComponentPath())
	}
	for _, bad := range []string{"", "ABC", "a-b", "1234567890123", "x y"} {
		if _, err := NewNames(bad); err == nil {
			t.Errorf("NewNames(%q) must fail", bad)
		}
	}
}

func TestNamesRefuseOtherPrefixes(t *testing.T) {
	err := Names{Group: "platform", Component: "e2e-r1-app"}.guard()
	if err == nil || !strings.Contains(err.Error(), `refusing to touch "platform"`) {
		t.Errorf("guard = %v", err)
	}
}
```

`internal/e2e/claims_test.go`:

```go
package e2e

import "testing"

func TestGroupClaim(t *testing.T) {
	got := string(GroupClaim("e2e-r1-team", "End-to-end harness team", []Member{{"owner", "maintainer"}, {"Member-One", "member"}}))
	want := "apiVersion: idp/v1\nkind: Group\nname: e2e-r1-team\ndescription: End-to-end harness team\nmembers:\n  - user: owner\n    role: maintainer\n  - user: Member-One\n    role: member\n"
	if got != want {
		t.Errorf("GroupClaim =\n%s\nwant\n%s", got, want)
	}
}

func TestComponentClaim(t *testing.T) {
	got := string(ComponentClaim("e2e-r1-app", "End-to-end harness repository", "e2e-r1-team", []string{"e2e", "harness"}))
	want := "apiVersion: idp/v1\nkind: Component\nname: e2e-r1-app\ndescription: End-to-end harness repository\nowner: group:e2e-r1-team\nenvironments: [dev]\ngithub:\n  topics: [e2e, harness]\n"
	if got != want {
		t.Errorf("ComponentClaim =\n%s\nwant\n%s", got, want)
	}
	noTopics := string(ComponentClaim("e2e-r1-app", "d", "e2e-r1-team", nil))
	if noTopics != "apiVersion: idp/v1\nkind: Component\nname: e2e-r1-app\ndescription: d\nowner: group:e2e-r1-team\nenvironments: [dev]\n" {
		t.Errorf("no topics:\n%s", noTopics)
	}
}
```

`internal/e2e/pin_test.go`:

```go
package e2e

import (
	"strings"
	"testing"
)

func TestPinTemplates(t *testing.T) {
	sha := strings.Repeat("b", 40)
	in := map[string][]byte{"pr.yaml": []byte("    uses: jellalshadows-idp/idp-engine/.github/workflows/pr.yaml@" + strings.Repeat("0", 40) + " # unreleased\n")}
	out, err := PinTemplates(in, sha)
	if err != nil {
		t.Fatal(err)
	}
	if want := "    uses: jellalshadows-idp/idp-engine/.github/workflows/pr.yaml@" + sha + " # unreleased\n"; string(out["pr.yaml"]) != want {
		t.Errorf("pinned = %q, want %q", out["pr.yaml"], want)
	}
	if _, err := PinTemplates(in, "abc"); err == nil {
		t.Error("a short SHA must be rejected")
	}
	if _, err := PinTemplates(map[string][]byte{"x.yaml": []byte("jobs: {}\n")}, sha); err == nil {
		t.Error("a template without an engine workflow ref must be rejected")
	}
}
```

`internal/e2e/clock_test.go`:

```go
package e2e

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time        { return c.now }
func (c *fakeClock) Sleep(d time.Duration) { c.now = c.now.Add(d) }

func TestPoll(t *testing.T) {
	c := &fakeClock{}
	calls := 0
	err := poll(context.Background(), c, time.Second, time.Minute, "x", func() (bool, error) { calls++; return calls == 3, nil })
	if err != nil || calls != 3 {
		t.Errorf("poll = %v after %d calls", err, calls)
	}
	err = poll(context.Background(), c, 10*time.Second, 30*time.Second, "the thing", func() (bool, error) { return false, nil })
	if err == nil || !strings.Contains(err.Error(), "timed out after 30s waiting for the thing") {
		t.Errorf("timeout err = %v", err)
	}
	boom := errors.New("boom")
	if err := poll(context.Background(), c, time.Second, time.Minute, "x", func() (bool, error) { return false, boom }); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}
```

`internal/e2e/fake_test.go`:

```go
package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	drifts   int
	head     int
	log      []string
}

func newFakeForge() *fakeForge {
	return &fakeForge{
		main: map[string][]byte{}, branches: map[string]map[string][]byte{}, prs: map[int]string{},
		merged: map[string]string{}, paths: map[string]any{}, issues: map[string]int{},
		gate: map[string]string{}, comments: map[string]string{}, effects: map[string]func(*fakeForge){},
		approval: map[string]bool{}, approved: map[int64]bool{},
	}
}

func slugOf(branch string) string { return branch[strings.LastIndex(branch, "/")+1:] }
func fakeSHA(n int) string        { return fmt.Sprintf("%040d", n) }
func shaNumber(sha string) int    { var n int; _, _ = fmt.Sscanf(sha, "%d", &n); return n }
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
	return nil
}

func (f *fakeForge) DeleteFile(_ context.Context, branch, path, _ string) error {
	if _, ok := f.main[path]; !ok {
		return fmt.Errorf("%s does not exist", path)
	}
	f.branches[branch][path] = nil
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
	c, ok := f.comments[slugOf(f.prs[pr])]
	if !ok {
		c = "<code>github</code>: +2 ~0 -0 ±0"
	}
	return planMarker + "\n" + c + "\n<!-- idp-fingerprint:v1 sha=" + fakeSHA(1000+pr) + " data=x -->", nil
}

func (f *fakeForge) Merge(_ context.Context, pr int, _ string) (string, error) {
	branch := f.prs[pr]
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
	if eff := f.effects[slugOf(branch)]; eff != nil {
		eff(f)
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
```

`internal/e2e/scenario_test.go`:

```go
package e2e

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"
	"time"
)

const testSHA = "abcdefabcdefabcdefabcdefabcdefabcdefabcd"

func testTemplates() map[string][]byte {
	t := map[string][]byte{}
	for _, n := range []string{"drift.yaml", "pr.yaml", "reconcile.yaml", "recover.yaml"} {
		t[n] = []byte("jobs:\n  x:\n    uses: jellalshadows-idp/idp-engine/.github/workflows/" + n + "@" + strings.Repeat("0", 40) + " # unreleased\n")
	}
	return t
}

// world returns a fake whose callers already pin testSHA and whose pipelines
// behave: create makes the resources, update changes them, the case-only edit
// plans nothing, delete removes everything after an approval.
func world(t *testing.T) (*fakeForge, *Scenario) {
	t.Helper()
	f := newFakeForge()
	pinned, err := PinTemplates(testTemplates(), testSHA)
	if err != nil {
		t.Fatal(err)
	}
	for n, b := range pinned {
		f.main[".github/workflows/"+n] = b
	}
	repo, team := "/repos/acme/e2e-r42-app", "/orgs/acme/teams/e2e-r42-team"
	f.effects["create"] = func(f *fakeForge) {
		f.paths[repo] = map[string]any{"visibility": "public", "description": "End-to-end harness repository", "topics": []string{"e2e", "harness"}}
		f.paths[team] = map[string]any{"privacy": "closed"}
		f.paths[team+"/memberships/owner"] = map[string]any{"role": "maintainer", "state": "active"}
		f.paths[team+"/memberships/member-one"] = map[string]any{"role": "member", "state": "active"}
		f.paths[repo+"/environments"] = map[string]any{"environments": []any{map[string]any{"name": "dev"}}}
	}
	f.effects["update"] = func(f *fakeForge) {
		f.paths[repo] = map[string]any{"visibility": "public", "description": "End-to-end harness repository, updated", "topics": []string{}}
	}
	f.comments["case"] = "No infrastructure changes."
	f.comments["delete"] = "<code>github</code>: +0 ~0 -8 ±0\n> [!WARNING]"
	f.approval["delete"] = true
	f.effects["delete"] = func(f *fakeForge) {
		for p := range f.paths {
			if strings.HasPrefix(p, repo) || strings.HasPrefix(p, team) {
				delete(f.paths, p)
			}
		}
	}
	s := &Scenario{
		F: f, Clock: &fakeClock{}, Log: io.Discard, Org: "acme", Run: "42", EngineSHA: testSHA,
		Templates: testTemplates(), Owner: "owner", Member: "member-one",
		Timeouts: Timeouts{Interval: time.Second, Checks: time.Minute, Reconcile: time.Minute, Drift: time.Minute},
	}
	return f, s
}

func TestScenarioPassesAgainstWorkingPipelines(t *testing.T) {
	f, s := world(t)
	if err := s.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"branch e2e/42/create", "pr create", "merge create",
		"branch e2e/42/update", "pr update", "merge update",
		"branch e2e/42/case", "pr case", "merge case",
		"drift",
		"branch e2e/42/delete", "pr delete", "merge delete", "approve 504",
		"drift",
	}
	if !slices.Equal(f.log, want) {
		t.Errorf("steps = %v\nwant    %v", f.log, want)
	}
}

func TestScenarioPinsTheCallersFirst(t *testing.T) {
	f, s := world(t)
	s.EngineSHA = strings.Repeat("b", 40)
	if err := s.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.log) < 3 || f.log[0] != "branch e2e/42/pin" || f.log[2] != "merge pin" {
		t.Errorf("steps = %v: the callers must be pinned to the commit under test first", f.log)
	}
	if got := string(f.main[".github/workflows/pr.yaml"]); !strings.Contains(got, "@"+strings.Repeat("b", 40)+" # unreleased") {
		t.Errorf("pr.yaml = %s", got)
	}
}

func TestApproveOnlyTheDeleteRun(t *testing.T) {
	f, s := world(t)
	f.approval["update"] = true
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unexpectedly waits for approval") {
		t.Fatalf("err = %v", err)
	}
	if f.approved[502] {
		t.Error("the harness approved the update's reconcile: only its own delete may be approved")
	}
	if len(f.approved) != 1 {
		t.Errorf("approved %v: only the cleanup's delete run", f.approved)
	}
}

func TestScenarioCleansUpAfterAFailure(t *testing.T) {
	f, s := world(t)
	f.gate["update"] = "failure"
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update: idp-gate on PR #2 concluded failure") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := f.paths["/repos/acme/e2e-r42-app"]; ok {
		t.Error("the cleanup must delete the claims through the platform")
	}
	if !slices.Contains(f.log, "merge delete") {
		t.Errorf("steps = %v: no delete PR", f.log)
	}
}

func TestScenarioRemovesAllTopics(t *testing.T) {
	f, s := world(t)
	f.effects["update"] = func(f *fakeForge) {
		f.paths["/repos/acme/e2e-r42-app"] = map[string]any{"visibility": "public", "description": "End-to-end harness repository, updated", "topics": []string{"e2e", "harness"}}
	}
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "topics = [e2e harness], want []") {
		t.Fatalf("err = %v", err)
	}
}

func TestScenarioRequiresACaseOnlyEditToPlanNothing(t *testing.T) {
	f, s := world(t)
	f.comments["case"] = "<code>github</code>: +1 ~0 -1 ±0"
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "a case-only login edit must plan no change") {
		t.Fatalf("err = %v", err)
	}
}

func TestScenarioRefusesAnUnrecoveredWetPush(t *testing.T) {
	f, s := world(t)
	f.issues["idp-wet-push/Wet push failed"] = 1
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "recover first") || len(f.log) != 0 {
		t.Fatalf("err = %v, steps %v", err, f.log)
	}
}

func TestScenarioRefusesLeftovers(t *testing.T) {
	f, s := world(t)
	f.paths["/repos/acme/e2e-r42-app"] = map[string]any{}
	if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v", err)
	}
}

func TestScenarioRequiresTheDeleteToBeFlaggedDestructive(t *testing.T) {
	f, s := world(t)
	f.comments["delete"] = "<code>github</code>: +0 ~0 -8 ±0"
	if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "the delete plan must be flagged destructive") {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/e2e/`
Expected: FAIL to compile (the package's non-test code does not exist).

- [ ] **Step 3: Implement**

`internal/e2e/names.go`:

```go
// Package e2e is the end-to-end harness (spec §8.5, ADR-0027): it drives a
// claims repo through the real pipelines and checks the results through the
// GitHub API.
package e2e

import (
	"fmt"
	"regexp"
	"strings"
)

// prefix is the only claim prefix the harness creates, changes or deletes.
const prefix = "e2e-"

var runPattern = regexp.MustCompile(`^[a-z0-9]{1,12}$`)

// Names are the claim names one run owns.
type Names struct {
	Group     string
	Component string
}

// NewNames derives the names of a run from its id.
func NewNames(run string) (Names, error) {
	if !runPattern.MatchString(run) {
		return Names{}, fmt.Errorf("run %q must be 1 to 12 lowercase letters or digits", run)
	}
	n := Names{Group: prefix + "r" + run + "-team", Component: prefix + "r" + run + "-app"}
	return n, n.guard()
}

func (n Names) guard() error {
	for _, name := range []string{n.Group, n.Component} {
		if !strings.HasPrefix(name, prefix) {
			return fmt.Errorf("refusing to touch %q: the harness manages only names that start with %q", name, prefix)
		}
	}
	return nil
}

func (n Names) GroupPath() string     { return "claims/groups/" + n.Group + ".yaml" }
func (n Names) ComponentPath() string { return "claims/components/" + n.Component + ".yaml" }
```

`internal/e2e/claims.go`:

```go
package e2e

import (
	"fmt"
	"strings"
)

// Member is one Group membership.
type Member struct {
	User string
	Role string
}

// GroupClaim renders a Group claim.
func GroupClaim(name, description string, members []Member) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: idp/v1\nkind: Group\nname: %s\ndescription: %s\nmembers:\n", name, description)
	for _, m := range members {
		fmt.Fprintf(&b, "  - user: %s\n    role: %s\n", m.User, m.Role)
	}
	return []byte(b.String())
}

// ComponentClaim renders a Component claim in environment dev; no topics means
// no github block.
func ComponentClaim(name, description, group string, topics []string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: idp/v1\nkind: Component\nname: %s\ndescription: %s\nowner: group:%s\nenvironments: [dev]\n", name, description, group)
	if len(topics) > 0 {
		fmt.Fprintf(&b, "github:\n  topics: [%s]\n", strings.Join(topics, ", "))
	}
	return []byte(b.String())
}
```

`internal/e2e/pin.go`:

```go
package e2e

import (
	"fmt"
	"regexp"
)

var (
	shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	enginePin  = regexp.MustCompile(`(idp-engine/\.github/workflows/[a-z]+\.yaml@)[0-9a-f]{40}`)
)

// PinTemplates returns the caller templates with every engine workflow ref
// pinned to sha.
func PinTemplates(templates map[string][]byte, sha string) (map[string][]byte, error) {
	if !shaPattern.MatchString(sha) {
		return nil, fmt.Errorf("engine sha %q is not a 40-character commit SHA", sha)
	}
	out := make(map[string][]byte, len(templates))
	for name, b := range templates {
		if !enginePin.Match(b) {
			return nil, fmt.Errorf("template %s has no engine workflow ref", name)
		}
		out[name] = enginePin.ReplaceAll(b, []byte("${1}"+sha))
	}
	return out, nil
}
```

`internal/e2e/clock.go`:

```go
package e2e

import (
	"context"
	"fmt"
	"time"
)

// Clock is the harness's notion of time, so tests never sleep.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type realClock struct{}

func (realClock) Now() time.Time        { return time.Now() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

// poll calls check every interval until it reports done or fails, or until
// timeout passes; what names the wait in the timeout error.
func poll(ctx context.Context, c Clock, interval, timeout time.Duration, what string, check func() (bool, error)) error {
	deadline := c.Now().Add(timeout)
	for {
		done, err := check()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !c.Now().Before(deadline) {
			return fmt.Errorf("timed out after %s waiting for %s", timeout, what)
		}
		c.Sleep(interval)
	}
}
```

`internal/e2e/forge.go`:

```go
package e2e

import "context"

// PR is a pull request the harness opened.
type PR struct {
	Number  int
	HeadSHA string
}

// Run is one workflow run.
type Run struct {
	ID         int64
	Status     string // queued, in_progress, waiting, completed
	Conclusion string // empty until completed
}

// Forge is everything the scenario needs from GitHub. The GitHub
// implementation holds two tokens: the bot App's for repository work and the
// approver account's for idp-approval deployments (ADR-0027).
type Forge interface {
	HeadSHA(ctx context.Context, branch string) (string, error)
	CreateBranch(ctx context.Context, branch, fromSHA string) error
	// File returns a file at ref, and false when it does not exist.
	File(ctx context.Context, ref, path string) ([]byte, bool, error)
	PutFile(ctx context.Context, branch, path string, content []byte, message string) error
	DeleteFile(ctx context.Context, branch, path, message string) error
	OpenPR(ctx context.Context, head, title, body string) (PR, error)
	// Checks maps each check run on sha to its conclusion, empty while it runs.
	Checks(ctx context.Context, sha string) (map[string]string, error)
	// PlanComment is the newest plan comment github-actions[bot] wrote on pr,
	// or "" when there is none yet.
	PlanComment(ctx context.Context, pr int) (string, error)
	Merge(ctx context.Context, pr int, title string) (string, error)
	// Runs lists a workflow's runs for a head commit, newest first.
	Runs(ctx context.Context, workflow, sha string) ([]Run, error)
	// Jobs maps each job of a run to its conclusion.
	Jobs(ctx context.Context, runID int64) (map[string]string, error)
	Dispatch(ctx context.Context, workflow, ref string) error
	// PendingApproval returns the idp-approval environment a run waits on.
	PendingApproval(ctx context.Context, runID int64) (int64, bool, error)
	Approve(ctx context.Context, runID, envID int64, comment string) error
	// Exists reports whether an API path answers (false on 404).
	Exists(ctx context.Context, path string) (bool, error)
	GetJSON(ctx context.Context, path string, out any) error
	// OpenIssues counts open issues the Actions bot opened with a label and title.
	OpenIssues(ctx context.Context, label, title string) (int, error)
}
```

`internal/e2e/scenario.go`:

```go
package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"
)

const planMarker = "<!-- idp-plan -->"

// Timeouts bound each kind of wait.
type Timeouts struct {
	Interval  time.Duration // between polls
	Checks    time.Duration // a pull request's idp-gate
	Reconcile time.Duration // a reconcile run, including the approval wait
	Drift     time.Duration // a drift run
}

// DefaultTimeouts suit the real pipelines, where every job builds idp and plans.
var DefaultTimeouts = Timeouts{Interval: 15 * time.Second, Checks: 20 * time.Minute, Reconcile: 40 * time.Minute, Drift: 20 * time.Minute}

// Scenario is one harness run against a claims repo (spec §8.5, ADR-0027).
type Scenario struct {
	F         Forge
	Clock     Clock
	Log       io.Writer
	Org       string
	Run       string            // unique id of this run
	EngineSHA string            // the engine commit under test
	Templates map[string][]byte // caller templates by file name
	Owner     string            // the harness team's maintainer
	Member    string            // the harness team's member; its case changes in one step
	Timeouts  Timeouts
	names     Names
}

// Execute runs every step. When a step fails after the claims may have been
// merged, it still deletes them through the platform and reports both errors.
func (s *Scenario) Execute(ctx context.Context) error {
	names, err := NewNames(s.Run)
	if err != nil {
		return err
	}
	s.names = names
	if err := s.preflight(ctx); err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if err := s.pin(ctx); err != nil {
		return fmt.Errorf("pin: %w", err)
	}
	for _, step := range []struct {
		name string
		run  func(context.Context) error
	}{{"create", s.create}, {"update", s.update}, {"case-only login edit", s.caseOnly}, {"drift", s.drift}} {
		if err := step.run(ctx); err != nil {
			return s.withCleanup(ctx, fmt.Errorf("%s: %w", step.name, err))
		}
	}
	if err := s.deleteClaims(ctx, []string{s.names.ComponentPath(), s.names.GroupPath()}); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if err := s.drift(ctx); err != nil {
		return fmt.Errorf("final drift: %w", err)
	}
	s.logf("e2e: passed")
	return nil
}

func (s *Scenario) preflight(ctx context.Context) error {
	n, err := s.F.OpenIssues(ctx, "idp-wet-push", "Wet push failed")
	if err != nil {
		return err
	}
	if n > 0 {
		return errors.New(`an open "Wet push failed" issue holds every reconcile: recover first`)
	}
	for _, p := range []string{s.repoPath(), s.teamPath()} {
		ok, err := s.F.Exists(ctx, p)
		if err != nil {
			return err
		}
		if ok {
			return fmt.Errorf("%s already exists: a previous run left it behind", p)
		}
	}
	return nil
}

func (s *Scenario) pin(ctx context.Context) error {
	want, err := PinTemplates(s.Templates, s.EngineSHA)
	if err != nil {
		return err
	}
	edits := map[string][]byte{}
	for _, name := range slices.Sorted(maps.Keys(want)) {
		path := ".github/workflows/" + name
		got, ok, err := s.F.File(ctx, "main", path)
		if err != nil {
			return err
		}
		if !ok || !bytes.Equal(got, want[name]) {
			edits[path] = want[name]
		}
	}
	if len(edits) == 0 {
		s.logf("e2e: the callers already pin %s", short(s.EngineSHA))
		return nil
	}
	title := "chore: pin the idp pipelines to " + short(s.EngineSHA)
	pr, err := s.openPR(ctx, "pin", title, edits, nil)
	if err != nil {
		return err
	}
	if _, err := s.waitChecks(ctx, pr); err != nil {
		return err
	}
	sha, err := s.F.Merge(ctx, pr.Number, title)
	if err != nil {
		return err
	}
	_, err = s.waitReconcile(ctx, sha, false)
	return err
}

func (s *Scenario) create(ctx context.Context) error {
	n := s.names
	title := "feat: add the e2e harness claims " + n.Component
	put := map[string][]byte{
		n.GroupPath():     GroupClaim(n.Group, "End-to-end harness team", []Member{{s.Owner, "maintainer"}, {s.Member, "member"}}),
		n.ComponentPath(): ComponentClaim(n.Component, "End-to-end harness repository", n.Group, []string{"e2e", "harness"}),
	}
	if err := s.change(ctx, "create", title, put, nil, false); err != nil {
		return err
	}
	return s.assertResources(ctx, "End-to-end harness repository", []string{"e2e", "harness"})
}

func (s *Scenario) update(ctx context.Context) error {
	n := s.names
	put := map[string][]byte{n.ComponentPath(): ComponentClaim(n.Component, "End-to-end harness repository, updated", n.Group, nil)}
	if err := s.change(ctx, "update", "feat: update "+n.Component+" and remove its topics", put, nil, false); err != nil {
		return err
	}
	return s.assertResources(ctx, "End-to-end harness repository, updated", nil)
}

func (s *Scenario) caseOnly(ctx context.Context) error {
	n := s.names
	put := map[string][]byte{n.GroupPath(): GroupClaim(n.Group, "End-to-end harness team", []Member{{s.Owner, "maintainer"}, {flipCase(s.Member), "member"}})}
	title := "feat: change only the case of a member login"
	pr, err := s.openPR(ctx, "case", title, put, nil)
	if err != nil {
		return err
	}
	comment, err := s.waitChecks(ctx, pr)
	if err != nil {
		return err
	}
	if !strings.Contains(comment, "No infrastructure changes.") {
		return errors.New("a case-only login edit must plan no change (phase 1a F1)")
	}
	sha, err := s.F.Merge(ctx, pr.Number, title)
	if err != nil {
		return err
	}
	jobs, err := s.waitReconcile(ctx, sha, false)
	if err != nil {
		return err
	}
	if jobs["reconcile / approve"] != "skipped" {
		return fmt.Errorf("the gate asked for approval (approve: %s)", jobs["reconcile / approve"])
	}
	return nil
}

func (s *Scenario) drift(ctx context.Context) error {
	head, err := s.F.HeadSHA(ctx, "main")
	if err != nil {
		return err
	}
	before, err := s.F.Runs(ctx, "drift.yaml", head)
	if err != nil {
		return err
	}
	if err := s.F.Dispatch(ctx, "drift.yaml", "main"); err != nil {
		return err
	}
	var run Run
	err = poll(ctx, s.Clock, s.Timeouts.Interval, s.Timeouts.Drift, "the drift run", func() (bool, error) {
		runs, err := s.F.Runs(ctx, "drift.yaml", head)
		if err != nil || len(runs) <= len(before) {
			return false, err
		}
		run = runs[0]
		return run.Status == "completed", nil
	})
	if err != nil {
		return err
	}
	if run.Conclusion != "success" {
		return fmt.Errorf("drift run %d concluded %s", run.ID, run.Conclusion)
	}
	open, err := s.F.OpenIssues(ctx, "drift", "Drift detected")
	if err != nil {
		return err
	}
	if open != 0 {
		return fmt.Errorf("drift run %d left a drift issue open", run.ID)
	}
	s.logf("e2e: drift run %d found no drift", run.ID)
	return nil
}

// deleteClaims deletes the given claim files through a pull request, approves
// the destructive reconcile of that merge only, and checks the resources are
// gone.
func (s *Scenario) deleteClaims(ctx context.Context, paths []string) error {
	if err := s.names.guard(); err != nil {
		return err
	}
	title := "feat: delete the e2e harness claims " + s.names.Component
	pr, err := s.openPR(ctx, "delete", title, nil, paths)
	if err != nil {
		return err
	}
	comment, err := s.waitChecks(ctx, pr)
	if err != nil {
		return err
	}
	if !strings.Contains(comment, "[!WARNING]") {
		return errors.New("the delete plan must be flagged destructive")
	}
	sha, err := s.F.Merge(ctx, pr.Number, title)
	if err != nil {
		return err
	}
	if _, err := s.waitReconcile(ctx, sha, true); err != nil {
		return err
	}
	for _, p := range []string{s.repoPath(), s.teamPath()} {
		ok, err := s.F.Exists(ctx, p)
		if err != nil {
			return err
		}
		if ok {
			return fmt.Errorf("%s still exists after the delete", p)
		}
	}
	s.logf("e2e: %s and %s are gone", s.names.Component, s.names.Group)
	return nil
}

// withCleanup deletes this run's claims through the platform after a failure,
// so a broken run does not leave resources behind; it never touches anything
// else (Review Focus 1).
func (s *Scenario) withCleanup(ctx context.Context, cause error) error {
	s.logf("e2e: %v; cleaning up", cause)
	var present []string
	for _, p := range []string{s.names.ComponentPath(), s.names.GroupPath()} {
		_, ok, err := s.F.File(ctx, "main", p)
		if err != nil {
			return errors.Join(cause, fmt.Errorf("cleanup: %w", err))
		}
		if ok {
			present = append(present, p)
		}
	}
	if len(present) == 0 {
		return cause
	}
	if err := s.deleteClaims(ctx, present); err != nil {
		return errors.Join(cause, fmt.Errorf("cleanup: %w; see docs/e2e.md for the leftovers", err))
	}
	return cause
}

// change opens a pull request with put and del, waits for idp-gate, refuses a
// destructive plan unless destructive is set, merges, and requires the
// reconcile to apply without approval.
func (s *Scenario) change(ctx context.Context, slug, title string, put map[string][]byte, del []string, destructive bool) error {
	pr, err := s.openPR(ctx, slug, title, put, del)
	if err != nil {
		return err
	}
	comment, err := s.waitChecks(ctx, pr)
	if err != nil {
		return err
	}
	if !destructive && strings.Contains(comment, "[!WARNING]") {
		return fmt.Errorf("the %s plan is destructive", slug)
	}
	sha, err := s.F.Merge(ctx, pr.Number, title)
	if err != nil {
		return err
	}
	jobs, err := s.waitReconcile(ctx, sha, false)
	if err != nil {
		return err
	}
	if jobs["reconcile / approve"] != "skipped" {
		return fmt.Errorf("the gate asked for approval (approve: %s)", jobs["reconcile / approve"])
	}
	if jobs["reconcile / apply-github"] != "success" {
		return fmt.Errorf("apply-github: %s", jobs["reconcile / apply-github"])
	}
	return nil
}

func (s *Scenario) openPR(ctx context.Context, slug, title string, put map[string][]byte, del []string) (PR, error) {
	base, err := s.F.HeadSHA(ctx, "main")
	if err != nil {
		return PR{}, err
	}
	branch := "e2e/" + s.Run + "/" + slug
	if err := s.F.CreateBranch(ctx, branch, base); err != nil {
		return PR{}, err
	}
	for _, p := range slices.Sorted(maps.Keys(put)) {
		if err := s.F.PutFile(ctx, branch, p, put[p], title); err != nil {
			return PR{}, err
		}
	}
	for _, p := range del {
		if err := s.F.DeleteFile(ctx, branch, p, title); err != nil {
			return PR{}, err
		}
	}
	pr, err := s.F.OpenPR(ctx, branch, title, "Opened by the idp end-to-end harness, run "+s.Run+".")
	if err != nil {
		return PR{}, err
	}
	s.logf("e2e: opened PR #%d (%s)", pr.Number, title)
	return pr, nil
}

// waitChecks waits for the pull request's idp-gate check, which mirrors the
// whole PR pipeline (ADR-0022), and returns the plan comment for its head.
func (s *Scenario) waitChecks(ctx context.Context, pr PR) (string, error) {
	var gate string
	err := poll(ctx, s.Clock, s.Timeouts.Interval, s.Timeouts.Checks, fmt.Sprintf("idp-gate on PR #%d", pr.Number), func() (bool, error) {
		checks, err := s.F.Checks(ctx, pr.HeadSHA)
		if err != nil {
			return false, err
		}
		gate = checks["idp-gate"]
		return gate != "", nil
	})
	if err != nil {
		return "", err
	}
	if gate != "success" {
		return "", fmt.Errorf("idp-gate on PR #%d concluded %s", pr.Number, gate)
	}
	comment, err := s.F.PlanComment(ctx, pr.Number)
	if err != nil {
		return "", err
	}
	if !strings.Contains(comment, planMarker) || !strings.Contains(comment, "sha="+pr.HeadSHA) {
		return "", fmt.Errorf("PR #%d has no plan comment for its head %s", pr.Number, short(pr.HeadSHA))
	}
	return comment, nil
}

// waitReconcile waits for the reconcile of a merge commit and returns its job
// conclusions. With approve it approves the idp-approval deployment of that run
// and of no other (Review Focus 2); without it, a run that waits for approval
// is an error.
func (s *Scenario) waitReconcile(ctx context.Context, sha string, approve bool) (map[string]string, error) {
	var run Run
	approved := false
	err := poll(ctx, s.Clock, s.Timeouts.Interval, s.Timeouts.Reconcile, "the reconcile of "+short(sha), func() (bool, error) {
		runs, err := s.F.Runs(ctx, "reconcile.yaml", sha)
		if err != nil || len(runs) == 0 {
			return false, err
		}
		run = runs[0]
		if run.Status == "waiting" && !approve {
			return false, fmt.Errorf("reconcile run %d unexpectedly waits for approval", run.ID)
		}
		if run.Status == "waiting" && !approved {
			env, ok, err := s.F.PendingApproval(ctx, run.ID)
			if err != nil {
				return false, err
			}
			if ok {
				if err := s.F.Approve(ctx, run.ID, env, "idp end-to-end harness run "+s.Run+": delete "+s.names.Component+" and "+s.names.Group); err != nil {
					return false, err
				}
				approved = true
				s.logf("e2e: approved reconcile run %d", run.ID)
			}
		}
		return run.Status == "completed", nil
	})
	if err != nil {
		return nil, err
	}
	if run.Conclusion != "success" {
		return nil, fmt.Errorf("reconcile run %d concluded %s", run.ID, run.Conclusion)
	}
	if approve && !approved {
		return nil, fmt.Errorf("reconcile run %d finished without asking for approval", run.ID)
	}
	return s.F.Jobs(ctx, run.ID)
}

func (s *Scenario) assertResources(ctx context.Context, description string, topics []string) error {
	var repo struct {
		Visibility  string   `json:"visibility"`
		Description string   `json:"description"`
		Topics      []string `json:"topics"`
		Archived    bool     `json:"archived"`
	}
	if err := s.F.GetJSON(ctx, s.repoPath(), &repo); err != nil {
		return err
	}
	if repo.Visibility != "public" || repo.Description != description || repo.Archived {
		return fmt.Errorf("repository %s = %+v, want public, unarchived, description %q", s.names.Component, repo, description)
	}
	if !sameSet(repo.Topics, topics) {
		return fmt.Errorf("repository %s topics = %v, want %v", s.names.Component, repo.Topics, topics)
	}
	var team struct {
		Privacy string `json:"privacy"`
	}
	if err := s.F.GetJSON(ctx, s.teamPath(), &team); err != nil {
		return err
	}
	if team.Privacy != "closed" {
		return fmt.Errorf("team %s privacy = %q, want closed", s.names.Group, team.Privacy)
	}
	for _, m := range []Member{{strings.ToLower(s.Owner), "maintainer"}, {strings.ToLower(s.Member), "member"}} {
		var got struct {
			Role string `json:"role"`
		}
		if err := s.F.GetJSON(ctx, s.teamPath()+"/memberships/"+m.User, &got); err != nil {
			return err
		}
		if got.Role != m.Role {
			return fmt.Errorf("membership of %s in %s = %q, want %q", m.User, s.names.Group, got.Role, m.Role)
		}
	}
	var envs struct {
		Environments []struct {
			Name string `json:"name"`
		} `json:"environments"`
	}
	if err := s.F.GetJSON(ctx, s.repoPath()+"/environments", &envs); err != nil {
		return err
	}
	if len(envs.Environments) != 1 || envs.Environments[0].Name != "dev" {
		return fmt.Errorf("environments of %s = %+v, want [dev]", s.names.Component, envs.Environments)
	}
	s.logf("e2e: %s and %s match the claims", s.names.Component, s.names.Group)
	return nil
}

func (s *Scenario) repoPath() string { return "/repos/" + s.Org + "/" + s.names.Component }
func (s *Scenario) teamPath() string { return "/orgs/" + s.Org + "/teams/" + s.names.Group }

func (s *Scenario) logf(format string, a ...any) { fmt.Fprintf(s.Log, format+"\n", a...) }

// flipCase upper-cases the first letter of each hyphen-separated part, so the
// result differs from login only in case.
func flipCase(login string) string {
	parts := strings.Split(login, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	if out := strings.Join(parts, "-"); out != login {
		return out
	}
	return strings.ToUpper(login)
}

func sameSet(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
```

- [ ] **Step 4: Verify** — `go test ./internal/e2e/` passes (every scenario test), then `go test ./... && go vet ./... && gofmt -l .`.

If `TestScenarioPassesAgainstWorkingPipelines` disagrees with the expected step list only because of a deliberate, better ordering, stop and report it (DONE_WITH_CONCERNS) instead of editing the expected list.

- [ ] **Step 5: Commit**

```bash
git add internal/e2e
git commit -m "feat(e2e): add the end-to-end harness scenario"
```

---

### Task 8: The GitHub forge and the `idp-e2e` command

**Files:**
- Create: `internal/e2e/github.go`, `internal/e2e/github_test.go`, `internal/e2e/main.go`, `internal/e2e/main_test.go`, `cmd/idp-e2e/main.go`

**Interfaces:**
- Consumes: Task 7's `Forge`, `PR`, `Run`, `Scenario`, `DefaultTimeouts`, `realClock`, `shaPattern`; `ghapi.Client` (`Get`, `Post`, `Put`, `Do`, `ErrNotFound`, `List`, `New`, `DefaultBaseURL`); `ghops.Repo.FindBotComment`, `ghops.ActionsBot`.
- Produces: `type GitHub struct{ Bot, Approver *ghapi.Client; Owner, Repo string }` implementing `Forge`; `func Main(args []string, stdout, stderr io.Writer, getenv func(string) string) int`; the binary `cmd/idp-e2e`.

- [ ] **Step 1: Write the failing tests**

`internal/e2e/github_test.go`:

```go
package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
			if b["merge_method"] != "merge" {
				t.Errorf("merge body = %v", b)
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
	if sha, err := g.Merge(context.Background(), 4, "t"); err != nil || sha != "m1" {
		t.Errorf("Merge = %q, %v", sha, err)
	}
}
```

`internal/e2e/main_test.go`:

```go
package e2e

import (
	"bytes"
	"strings"
	"testing"
)

func TestMainUsage(t *testing.T) {
	env := func(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }
	tokens := map[string]string{"E2E_BOT_TOKEN": "b", "E2E_APPROVER_TOKEN": "a"}
	sha := strings.Repeat("a", 40)
	tests := []struct {
		name string
		args []string
		vars map[string]string
		want string
	}{
		{"no repo", []string{"--engine-sha", sha, "--run", "1"}, tokens, "--repo OWNER/NAME, --engine-sha and --run are required"},
		{"bad sha", []string{"--repo", "acme/x", "--engine-sha", "abc", "--run", "1"}, tokens, "--engine-sha must be a 40-character commit SHA"},
		{"no tokens", []string{"--repo", "acme/x", "--engine-sha", sha, "--run", "1"}, map[string]string{}, "E2E_BOT_TOKEN and E2E_APPROVER_TOKEN must be set"},
	}
	for _, tt := range tests {
		var out, errb bytes.Buffer
		if code := Main(tt.args, &out, &errb, env(tt.vars)); code != 2 || !strings.Contains(errb.String(), tt.want) {
			t.Errorf("%s: exit %d, stderr %q, want 2 and %q", tt.name, code, errb.String(), tt.want)
		}
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/e2e/ -run 'GitHub|MainUsage'`
Expected: FAIL to compile (`GitHub`, `Main` undefined).

- [ ] **Step 3: Implement**

`internal/e2e/github.go`:

```go
package e2e

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
	"github.com/jellalshadows-idp/idp-engine/internal/ghops"
)

// GitHub implements Forge over the REST API with the two tokens of ADR-0027:
// Bot for everything except approvals, Approver for idp-approval only.
type GitHub struct {
	Bot      *ghapi.Client
	Approver *ghapi.Client
	Owner    string
	Repo     string
}

func (g GitHub) path(p string) string { return "/repos/" + g.Owner + "/" + g.Repo + p }

func (g GitHub) HeadSHA(ctx context.Context, branch string) (string, error) {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := g.Bot.Get(ctx, g.path("/git/ref/heads/"+branch), &ref); err != nil {
		return "", err
	}
	return ref.Object.SHA, nil
}

func (g GitHub) CreateBranch(ctx context.Context, branch, fromSHA string) error {
	return g.Bot.Post(ctx, g.path("/git/refs"), map[string]string{"ref": "refs/heads/" + branch, "sha": fromSHA}, nil)
}

type contentsFile struct {
	SHA      string `json:"sha"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func (g GitHub) contents(ctx context.Context, ref, path string) (*contentsFile, error) {
	var f contentsFile
	err := g.Bot.Get(ctx, g.path("/contents/"+path+"?ref="+url.QueryEscape(ref)), &f)
	if errors.Is(err, ghapi.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (g GitHub) File(ctx context.Context, ref, path string) ([]byte, bool, error) {
	f, err := g.contents(ctx, ref, path)
	if err != nil || f == nil {
		return nil, false, err
	}
	if f.Encoding != "base64" {
		return nil, false, fmt.Errorf("%s: unexpected encoding %q", path, f.Encoding)
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(f.Content, "\n", ""))
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	return b, true, nil
}

func (g GitHub) PutFile(ctx context.Context, branch, path string, content []byte, message string) error {
	f, err := g.contents(ctx, branch, path)
	if err != nil {
		return err
	}
	body := map[string]any{"message": message, "content": base64.StdEncoding.EncodeToString(content), "branch": branch}
	if f != nil {
		body["sha"] = f.SHA
	}
	return g.Bot.Put(ctx, g.path("/contents/"+path), body, nil)
}

func (g GitHub) DeleteFile(ctx context.Context, branch, path, message string) error {
	f, err := g.contents(ctx, branch, path)
	if err != nil {
		return err
	}
	if f == nil {
		return fmt.Errorf("%s does not exist on %s", path, branch)
	}
	return g.Bot.Do(ctx, http.MethodDelete, g.path("/contents/"+path), map[string]any{"message": message, "sha": f.SHA, "branch": branch}, nil)
}

func (g GitHub) OpenPR(ctx context.Context, head, title, body string) (PR, error) {
	var pr struct {
		Number int `json:"number"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := g.Bot.Post(ctx, g.path("/pulls"), map[string]string{"title": title, "head": head, "base": "main", "body": body}, &pr); err != nil {
		return PR{}, err
	}
	return PR{Number: pr.Number, HeadSHA: pr.Head.SHA}, nil
}

func (g GitHub) Checks(ctx context.Context, sha string) (map[string]string, error) {
	var resp struct {
		CheckRuns []struct {
			ID         int64   `json:"id"`
			Name       string  `json:"name"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := g.Bot.Get(ctx, g.path("/commits/"+sha+"/check-runs?per_page=100"), &resp); err != nil {
		return nil, err
	}
	newest := map[string]int64{}
	out := map[string]string{}
	for _, c := range resp.CheckRuns {
		if c.ID < newest[c.Name] {
			continue
		}
		newest[c.Name] = c.ID
		out[c.Name] = ""
		if c.Status == "completed" && c.Conclusion != nil {
			out[c.Name] = *c.Conclusion
		}
	}
	return out, nil
}

func (g GitHub) PlanComment(ctx context.Context, pr int) (string, error) {
	c, err := ghops.Repo{API: g.Bot, Owner: g.Owner, Name: g.Repo}.FindBotComment(ctx, pr, planMarker)
	if err != nil || c == nil {
		return "", err
	}
	return c.Body, nil
}

func (g GitHub) Merge(ctx context.Context, pr int, title string) (string, error) {
	var resp struct {
		SHA    string `json:"sha"`
		Merged bool   `json:"merged"`
	}
	if err := g.Bot.Put(ctx, g.path(fmt.Sprintf("/pulls/%d/merge", pr)), map[string]string{"merge_method": "merge", "commit_title": title}, &resp); err != nil {
		return "", err
	}
	if !resp.Merged {
		return "", fmt.Errorf("PR #%d was not merged", pr)
	}
	return resp.SHA, nil
}

func (g GitHub) Runs(ctx context.Context, workflow, sha string) ([]Run, error) {
	var resp struct {
		WorkflowRuns []struct {
			ID         int64   `json:"id"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
		} `json:"workflow_runs"`
	}
	if err := g.Bot.Get(ctx, g.path("/actions/workflows/"+workflow+"/runs?head_sha="+url.QueryEscape(sha)+"&per_page=20"), &resp); err != nil {
		return nil, err
	}
	runs := make([]Run, 0, len(resp.WorkflowRuns))
	for _, r := range resp.WorkflowRuns {
		run := Run{ID: r.ID, Status: r.Status}
		if r.Conclusion != nil {
			run.Conclusion = *r.Conclusion
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (g GitHub) Jobs(ctx context.Context, runID int64) (map[string]string, error) {
	var resp struct {
		Jobs []struct {
			Name       string  `json:"name"`
			Conclusion *string `json:"conclusion"`
		} `json:"jobs"`
	}
	if err := g.Bot.Get(ctx, g.path(fmt.Sprintf("/actions/runs/%d/jobs?per_page=100", runID)), &resp); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, j := range resp.Jobs {
		out[j.Name] = ""
		if j.Conclusion != nil {
			out[j.Name] = *j.Conclusion
		}
	}
	return out, nil
}

func (g GitHub) Dispatch(ctx context.Context, workflow, ref string) error {
	return g.Bot.Post(ctx, g.path("/actions/workflows/"+workflow+"/dispatches"), map[string]string{"ref": ref}, nil)
}

func (g GitHub) PendingApproval(ctx context.Context, runID int64) (int64, bool, error) {
	var pending []struct {
		Environment struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"environment"`
		CurrentUserCanApprove bool `json:"current_user_can_approve"`
	}
	if err := g.Approver.Get(ctx, g.path(fmt.Sprintf("/actions/runs/%d/pending_deployments", runID)), &pending); err != nil {
		return 0, false, err
	}
	for _, p := range pending {
		if p.Environment.Name == "idp-approval" && p.CurrentUserCanApprove {
			return p.Environment.ID, true, nil
		}
	}
	return 0, false, nil
}

func (g GitHub) Approve(ctx context.Context, runID, envID int64, comment string) error {
	body := map[string]any{"environment_ids": []int64{envID}, "state": "approved", "comment": comment}
	return g.Approver.Post(ctx, g.path(fmt.Sprintf("/actions/runs/%d/pending_deployments", runID)), body, nil)
}

func (g GitHub) Exists(ctx context.Context, path string) (bool, error) {
	err := g.Bot.Get(ctx, path, nil)
	if errors.Is(err, ghapi.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (g GitHub) GetJSON(ctx context.Context, path string, out any) error {
	return g.Bot.Get(ctx, path, out)
}

func (g GitHub) OpenIssues(ctx context.Context, label, title string) (int, error) {
	type issue struct {
		Title       string `json:"title"`
		PullRequest *struct{} `json:"pull_request"`
		User        struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	issues, err := ghapi.List[issue](ctx, g.Bot, g.path("/issues?state=open&labels="+url.QueryEscape(label)))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, i := range issues {
		if i.PullRequest == nil && i.Title == title && i.User.Login == ghops.ActionsBot {
			n++
		}
	}
	return n, nil
}
```

`internal/e2e/main.go`:

```go
package e2e

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// Main runs the harness:
//
//	idp-e2e --repo OWNER/NAME --engine-sha SHA --run ID [--templates DIR] [--owner LOGIN] [--member LOGIN]
//
// with the bot and approver tokens in E2E_BOT_TOKEN and E2E_APPROVER_TOKEN.
// It exits 0 when every step passed, 1 when one failed, 2 on a usage error.
func Main(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("idp-e2e", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", "", "claims repository, OWNER/NAME")
	engineSHA := fs.String("engine-sha", "", "the idp-engine commit under test")
	run := fs.String("run", "", "unique id of this run (the workflow run id)")
	templates := fs.String("templates", "examples/claims-repo/.github/workflows", "directory of the caller templates")
	owner := fs.String("owner", "jellalshadows", "login of the harness team's maintainer")
	member := fs.String("member", "<second account>", "login of the harness team's member")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	org, name, ok := strings.Cut(*repo, "/")
	if !ok || org == "" || name == "" || *engineSHA == "" || *run == "" {
		fmt.Fprintln(stderr, "idp-e2e: --repo OWNER/NAME, --engine-sha and --run are required")
		return 2
	}
	if !shaPattern.MatchString(*engineSHA) {
		fmt.Fprintln(stderr, "idp-e2e: --engine-sha must be a 40-character commit SHA")
		return 2
	}
	bot, approver := getenv("E2E_BOT_TOKEN"), getenv("E2E_APPROVER_TOKEN")
	if bot == "" || approver == "" {
		fmt.Fprintln(stderr, "idp-e2e: E2E_BOT_TOKEN and E2E_APPROVER_TOKEN must be set")
		return 2
	}
	tpl, err := readTemplates(*templates)
	if err != nil {
		fmt.Fprintln(stderr, "idp-e2e:", err)
		return 1
	}
	base := getenv("IDP_GITHUB_API")
	if base == "" {
		base = ghapi.DefaultBaseURL
	}
	s := &Scenario{
		F:     GitHub{Bot: ghapi.New(base, bot), Approver: ghapi.New(base, approver), Owner: org, Repo: name},
		Clock: realClock{}, Log: stdout, Org: org, Run: *run, EngineSHA: *engineSHA,
		Templates: tpl, Owner: *owner, Member: *member, Timeouts: DefaultTimeouts,
	}
	if err := s.Execute(context.Background()); err != nil {
		fmt.Fprintln(stderr, "idp-e2e:", err)
		return 1
	}
	return 0
}

func readTemplates(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, n := range []string{"drift.yaml", "pr.yaml", "reconcile.yaml", "recover.yaml"} {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, fmt.Errorf("read template: %w", err)
		}
		out[n] = b
	}
	return out, nil
}
```

`cmd/idp-e2e/main.go`:

```go
// Command idp-e2e is the end-to-end harness (spec §8.5, ADR-0027). It drives a
// claims repo through the real pipelines; it is not part of the released CLI.
package main

import (
	"os"

	"github.com/jellalshadows-idp/idp-engine/internal/e2e"
)

func main() {
	os.Exit(e2e.Main(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}
```

Check `ghapi.List`'s path handling before relying on it: it appends `per_page=100` and must join with `&` when the path already has a query; if it does not, pass the query another way and say so in the report.

- [ ] **Step 4: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass.

- [ ] **Step 5: Commit**

```bash
git add internal/e2e cmd/idp-e2e
git commit -m "feat(e2e): add the github forge and the idp-e2e command"
```

---

### Task 9: The E2E workflow, its contract test and docs

**Files:**
- Create: `.github/workflows/e2e.yaml`, `internal/workflowspec/e2e_test.go`, `docs/e2e.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: Task 6's `internalWorkflows` (which lists `e2e.yaml`); the helpers `loadWorkflow`, `mustJob`, `mustStep`, `wantPerms`, `wantEnv`, `wantWith`, `wantRun`, `concurrency`.
- Produces: the workflow `idp-e2e`; `docs/e2e.md` (anchor `#when-a-run-fails` for leftovers).

- [ ] **Step 1: Write the failing test** — `internal/workflowspec/e2e_test.go`:

```go
package workflowspec

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

const e2ePath = ".github/workflows/e2e.yaml"

func TestE2EWorkflow(t *testing.T) {
	w := loadWorkflow(t, e2ePath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"schedule", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want schedule and workflow_dispatch (release PRs join in Phase 1e)", got)
	}
	n, ok := w.trigger("schedule")
	if !ok {
		t.Fatal("no schedule")
	}
	var crons []struct {
		Cron string `yaml:"cron"`
	}
	if err := n.Decode(&crons); err != nil || len(crons) != 1 || crons[0].Cron != "41 3 * * *" {
		t.Errorf("schedule = %v, %v", crons, err)
	}
	if w.Concurrency == nil || *w.Concurrency != (concurrency{Group: "idp-e2e"}) {
		t.Errorf("concurrency = %+v, want group idp-e2e without cancellation", w.Concurrency)
	}
	j := mustJob(t, w, "e2e")
	if j.Environment != "idp-e2e" {
		t.Errorf("environment = %q, want idp-e2e: the bot key and the approver token live only there (ADR-0027)", j.Environment)
	}
	wantPerms(t, "e2e", j.Permissions, map[string]string{"contents": "read"})
}

func TestE2ETokensAreScoped(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	_, bot := mustStep(t, "e2e", j.Steps, "bot")
	want := map[string]string{
		"client-id":                "${{ vars.IDP_BOT_CLIENT_ID }}",
		"private-key":              "${{ secrets.IDP_BOT_PRIVATE_KEY }}",
		"owner":                    "${{ github.repository_owner }}",
		"repositories":             "idp-claims-e2e",
		"permission-actions":       "write",
		"permission-checks":        "read",
		"permission-contents":      "write",
		"permission-issues":        "read",
		"permission-members":       "read",
		"permission-pull-requests": "write",
		"permission-workflows":     "write",
	}
	if !maps.Equal(map[string]string(bot.With), want) {
		t.Errorf("bot token inputs = %v, want %v", bot.With, want)
	}
	_, h := mustStep(t, "e2e", j.Steps, "harness")
	wantEnv(t, "e2e harness", h, map[string]string{
		"E2E_BOT_TOKEN":      "${{ steps.bot.outputs.token }}",
		"E2E_APPROVER_TOKEN": "${{ secrets.IDP_E2E_APPROVER_TOKEN }}",
	})
	wantRun(t, "e2e harness", h, `go run ./cmd/idp-e2e --repo "$GITHUB_REPOSITORY_OWNER/idp-claims-e2e" --engine-sha "$GITHUB_SHA" --run "$GITHUB_RUN_ID"`)
	for _, s := range j.Steps {
		if s.ID != "harness" && s.ID != "config" && strings.Contains(s.Env["E2E_APPROVER_TOKEN"]+s.Run, "APPROVER") {
			t.Errorf("step %q sees the approver token: only the harness step may", s.label())
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/workflowspec/ -run TestE2E`
Expected: FAIL (`read .github/workflows/e2e.yaml: …`).

- [ ] **Step 3: Write `.github/workflows/e2e.yaml`**

```yaml
name: idp-e2e

# End-to-end harness (spec §8.5, ADR-0027): drives idp-claims-e2e through the
# real pipelines at this commit, nightly and on demand from main.

on:
  schedule:
    - cron: '41 3 * * *'
  workflow_dispatch:

permissions: {}

defaults:
  run:
    shell: bash

concurrency:
  group: idp-e2e
  cancel-in-progress: false

jobs:
  e2e:
    name: e2e
    runs-on: ubuntu-24.04
    timeout-minutes: 150
    environment: idp-e2e
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: false
      - name: check the environment
        id: config
        env:
          BOT_CLIENT_ID: ${{ vars.IDP_BOT_CLIENT_ID }}
          BOT_KEY_SET: ${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}
          APPROVER_SET: ${{ secrets.IDP_E2E_APPROVER_TOKEN != '' }}
        run: |
          missing=""
          [ -n "$BOT_CLIENT_ID" ] || missing="$missing variable IDP_BOT_CLIENT_ID,"
          [ "$BOT_KEY_SET" = true ] || missing="$missing secret IDP_BOT_PRIVATE_KEY,"
          [ "$APPROVER_SET" = true ] || missing="$missing secret IDP_E2E_APPROVER_TOKEN,"
          if [ -n "$missing" ]; then
            echo "::error::the idp-e2e environment is missing:${missing%,} (docs/e2e.md)"
            exit 1
          fi
      - name: mint the bot token
        id: bot
        uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0
        with:
          client-id: ${{ vars.IDP_BOT_CLIENT_ID }}
          private-key: ${{ secrets.IDP_BOT_PRIVATE_KEY }}
          owner: ${{ github.repository_owner }}
          repositories: idp-claims-e2e
          permission-actions: write
          permission-checks: read
          permission-contents: write
          permission-issues: read
          permission-members: read
          permission-pull-requests: write
          permission-workflows: write
      - name: run the harness
        id: harness
        env:
          E2E_BOT_TOKEN: ${{ steps.bot.outputs.token }}
          E2E_APPROVER_TOKEN: ${{ secrets.IDP_E2E_APPROVER_TOKEN }}
        run: go run ./cmd/idp-e2e --repo "$GITHUB_REPOSITORY_OWNER/idp-claims-e2e" --engine-sha "$GITHUB_SHA" --run "$GITHUB_RUN_ID"
```

- [ ] **Step 4: Write `docs/e2e.md`**

````markdown
# End-to-end harness

The harness drives `idp-claims-e2e` through the real pipelines at an engine commit and checks the results through the GitHub API ([ADR-0027](adr/0027-end-to-end-harness.md), spec §8.5). It is `cmd/idp-e2e`, run by the workflow `.github/workflows/e2e.yaml`.

## What a run does

1. Pins the four caller workflows of `idp-claims-e2e` to the commit under test (a PR, when they differ).
2. Creates `e2e-r<run>-team` and `e2e-r<run>-app` through a PR, merges it after `idp-gate`, waits for the automatic reconcile, and checks the repository, team, memberships and environment.
3. Updates the description and removes every topic.
4. Changes only the case of a member login and expects `No infrastructure changes.`.
5. Runs the drift check and expects no drift issue.
6. Deletes the claims through a PR, approves that reconcile's `idp-approval` deployment, and checks the repository and team are gone.
7. Runs the drift check again.

A partial apply failure, the `recover` workflow and pull requests from forks are not covered; the Phase 1c smoke proved them (docs/phases/phase-1c.md).

## When it runs

Nightly at 03:41 UTC and on demand: `gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main`. One run at a time. Do not commit to `idp-claims-e2e` while a run is in progress.

## Credentials

Both live only in the `idp-engine` environment `idp-e2e` (deployment branch `main`):

| Name | What | Scope |
|---|---|---|
| `IDP_BOT_PRIVATE_KEY` (secret), `IDP_BOT_CLIENT_ID` (variable) | The bot App ([ADR-0026](adr/0026-automation-app.md)) | A token per run for `idp-claims-e2e` only: actions write, checks read, contents write, issues read, members read, pull requests write, workflows write |
| `IDP_E2E_APPROVER_TOKEN` (secret) | A fine-grained token of the second account, an org member and a second `idp-approval` reviewer in `idp-claims-e2e` only | `idp-claims-e2e` only: Actions read, Deployments read and write |

### Setting up

```bash
# The environment, with deployment branch main.
gh api -X PUT repos/jellalshadows-idp/idp-engine/environments/idp-e2e \
  -F 'deployment_branch_policy[protected_branches]=false' -F 'deployment_branch_policy[custom_branch_policies]=true'
gh api -X POST repos/jellalshadows-idp/idp-engine/environments/idp-e2e/deployment-branch-policies -f name=main -f type=branch
# The bot App credentials, from the files `idp bootstrap app --role bot` wrote.
gh secret set IDP_BOT_PRIVATE_KEY --env idp-e2e --repo jellalshadows-idp/idp-engine < ~/.idp/apps/jellalshadows-idp-bot.pem
gh variable set IDP_BOT_CLIENT_ID --env idp-e2e --repo jellalshadows-idp/idp-engine --body "<client id from jellalshadows-idp-bot.json>"
# The approver token: the owner pastes it; it is never written to disk.
gh secret set IDP_E2E_APPROVER_TOKEN --env idp-e2e --repo jellalshadows-idp/idp-engine
```

The approver account must be an org member and listed as an approver of `idp-claims-e2e`: `idp bootstrap apply --org jellalshadows-idp --claims-repo idp-claims-e2e --approver jellalshadows --approver <second account> …` ([bootstrap runbook](runbooks/bootstrap.md)).

### Rotating the approver token

Fine-grained tokens expire. Before the expiry date, sign in as the second account, create a new token with the same scope, approve it in the org settings if asked, run the `gh secret set IDP_E2E_APPROVER_TOKEN …` line above, then delete the old token. A run with an expired token fails at its first approval.

## When a run fails

- The log of the `run the harness` step names the step and the cause.
- After a failure past the creation step, the harness deletes its claims through the platform (a delete PR and its approval). If that also fails, the run reports both errors and leaves the resources; finish the cleanup **through a PR in `idp-claims-e2e`** that deletes the `e2e-r<run>-*` claim files. Never delete repositories or teams by hand.
- The PR branches `e2e/<run>/<step>` stay in `idp-claims-e2e` as evidence; GitHub deletes merged branches automatically.
- An open "Wet push failed" issue in `idp-claims-e2e` stops the harness before it starts: follow [A wet push failed](runbooks/pipelines.md#a-wet-push-failed).
````

`README.md`: add `- End-to-end harness: [docs/e2e.md](docs/e2e.md)` after the Pipelines line.

- [ ] **Step 5: Verify** — `go test ./... && go vet ./... && gofmt -l .`; actionlint (no output); `"$ZIZMOR" --offline .github/workflows .github/actions/setup-idp/action.yaml examples/claims-repo/.github/workflows/*.yaml` (no unsuppressed findings; the bot token names its repository and permissions, so `github-app` should not fire — if it does, report the exact finding).

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/e2e.yaml internal/workflowspec/e2e_test.go docs/e2e.md README.md
git commit -m "feat(workflows): run the end-to-end harness nightly"
```

---

### Task 10: Ship I — verification, PR, credentials, final review

Controller task (tests and linters are delegated to a subagent).

- [ ] **Step 1: Local verification** — `go test ./... && go vet ./... && gofmt -l .`; both actionlint invocations (engine workflows; templates with the `queue` ignore); zizmor on the workflows, the composite action and the templates; the component module's `tofu fmt -check` and `tofu test`.
- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feat/phase-1d-e2e-harness
gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-1d-e2e-harness --title "feat: phase 1d end-to-end harness and hardening" --body "Implements docs/superpowers/plans/2026-10-13-phase-1d-e2e-harness.md (ADRs 0026-0027, ADR-0018 amendment, spec amendment A8): the end-to-end harness (cmd/idp-e2e, e2e.yaml), the bot App and several approvers in bootstrap, the gate editor check, bot-authored issues, branch validation, topics through github_repository_topics, and contract-test hardening. The live harness run is recorded in the phase log."
```

- [ ] **Step 3: CI** — `gh pr checks --repo jellalshadows-idp/idp-engine --watch`: `go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `render-smoke` pass. A failure goes through the fix loop.
- [ ] **Step 4: Credentials** (owner actions O-1 to O-4 and side effects 2–5, in this order):
  1. Start `go run ./cmd/idp bootstrap app --org jellalshadows-idp --role bot` and give the owner the printed link — **O-1**. Then the install link — **O-2** (only `idp-engine`, `idp-claims`, `idp-claims-e2e`). Verify: `gh api orgs/jellalshadows-idp/installations --jq '.installations[] | [.app_slug, .repository_selection] | @tsv'` lists `jellalshadows-idp-bot	selected`.
  2. Invite: `gh api -X PUT orgs/jellalshadows-idp/memberships/<second account> -f role=member` — then **O-3**. Verify: `gh api orgs/jellalshadows-idp/members/<second account> --silent` succeeds.
  3. Bootstrap (git bash, from the branch): `idp bootstrap check` then `apply` then `check` for `idp-claims-e2e` with `--approver jellalshadows --approver <second account> --passphrase-file ~/.idp/e2e.pass`, and for `idp-claims` with `--approver jellalshadows --passphrase-file ~/.idp/main.pass` (same flags as the Phase 1c plan's Task 9 Step 4). Expected: the first check reports `IDP_BOOTSTRAP` (old format) and, for e2e, the `idp-approval` reviewers; `apply` changes exactly those; the second check prints `bootstrap check: no drift`.
  4. The `idp-e2e` environment, `IDP_BOT_PRIVATE_KEY` and `IDP_BOT_CLIENT_ID`, with the commands in `docs/e2e.md` ("Setting up"; the controller never prints the key).
  5. **O-4** (the approver token). Verify: `gh secret list --env idp-e2e --repo jellalshadows-idp/idp-engine` shows both secrets.
- [ ] **Step 5: Final whole-branch review** (most capable model) with the package `main..HEAD`, the spec, ADRs 0018, 0026, 0027 and this plan; one fix wave; one scoped re-review; rulings in the ledger.

---

### Task 11: Live harness run (pre-merge) and fix loop

- [ ] **Step 1: Allow the branch for one run** — `gh api -X POST repos/jellalshadows-idp/idp-engine/environments/idp-e2e/deployment-branch-policies -f name=feat/phase-1d-e2e-harness -f type=branch` (note the returned `id`).
- [ ] **Step 2: Run** — `gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref feat/phase-1d-e2e-harness`; find the run (`gh run list --repo jellalshadows-idp/idp-engine --workflow e2e.yaml --limit 1 --json databaseId --jq '.[0].databaseId'`) and watch it (`gh run watch <id> --repo jellalshadows-idp/idp-engine --exit-status`).
- [ ] **Step 3: Expected** — the run succeeds and its log shows every step: the pin PR (the callers move from the 1c merge commit to the branch head), create, update with topics removed, `No infrastructure changes.` for the case-only edit, a clean drift, the delete with one approval by the second account, a clean final drift, `e2e: passed`. Record every PR number and run URL in `idp-claims-e2e` for the phase log.
- [ ] **Step 4: Fix loop** — when a step fails: read the harness log and the failing run in `idp-claims-e2e`; rule on the cause; fix on the branch with a test that pins it (a scenario test, a module test, or a workflow contract test), scoped review, push, CI; re-run from Step 2 (the harness cleans up after itself; if it could not, stop and ask the owner before anything is deleted). Known candidates to watch: the case-only login edit planning a membership change (fix: lowercase the login in the render or the module, with a test), and the topics resource on create.
- [ ] **Step 5: Remove the temporary branch policy** — `gh api -X DELETE repos/jellalshadows-idp/idp-engine/environments/idp-e2e/deployment-branch-policies/<id>`.

---

### Task 12: Ship II — merge, nightly run, execution log

- [ ] **Step 1: Merge** — confirm the PR's latest CI is green; `gh pr merge --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`.
- [ ] **Step 2: Run from main once** — `gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main` and watch it: it pins `idp-claims-e2e` to the merge commit and passes. This is the run the nightly schedule repeats.
- [ ] **Step 3: Write `docs/phases/phase-1d.md`** in the style of `docs/phases/phase-1c.md` (summary; tasks with commits and reviews; every ruling with what, why and cost if wrong; the live harness runs with PRs and run URLs; the final disposition of every finding, including the Phase 1c minors ruled to Phase 5 with their reasons; carried into Phase 1e; owner notes), and update the README status lines to say Phase 1d is complete and Phase 1e (releases, Renovate, `v0.1.0`, onboarding `idp-claims`) is next. Commit doc-only to `main` as `docs: add the phase 1d execution log` and push.
