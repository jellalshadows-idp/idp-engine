# Phase 1d execution log: end-to-end harness and hardening

- Period: 2026-10-13 to 2026-10-17
- Plan: [2026-10-13-phase-1d-e2e-harness.md](../superpowers/plans/2026-10-13-phase-1d-e2e-harness.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md), amended by A8
- Decisions: ADR-0026 and ADR-0027 (see "Tasks" and "Rulings")
- Previous phase: [Phase 1c log](phase-1c.md)
- Status: **complete**. The harness merged through [PR #8](https://github.com/jellalshadows-idp/idp-engine/pull/8) (merge commit `3b9beed`) and the topics fix through [PR #9](https://github.com/jellalshadows-idp/idp-engine/pull/9) (merge commit `344f55d`). Live runs 2 to 4 passed.

This file is the durable copy of the execution ledger that was kept while the plan ran. The ledger lived in a git-ignored scratch folder that is deleted later, so what it recorded is reproduced here: the rulings, each task's review outcome, the live runs, the disposition of every finding and what is carried forward.

## Summary

Phase 1d adds a repeatable end-to-end check of the pipelines and closes the hardening items that Phase 1c deferred. The release machinery, Renovate, `v0.1.0` and onboarding the production `idp-claims` repo were re-split into Phase 1e (ruling D-5).

- **The harness.** `cmd/idp-e2e` over `internal/e2e` drives `idp-claims-e2e` through a scripted scenario: pin the callers, create a Group and a Component, update the Component and remove its topics, change only the case of a member login, delete the claims, and clean up after itself. It runs from `.github/workflows/e2e.yaml`, nightly and on dispatch, from `main`. It is not shipped in a release (ruling D-6).
- **A third GitHub App**, `<org>-bot` (ADR-0026), so that the pull requests the harness opens trigger CI, with no org setting weakened. The harness mints and renews its own installation tokens (ADR-0027, amended in execution).
- **Gate hardening.** The gate ignores plan comments edited by anyone but the bot (a GraphQL editor read). `FindOpenIssue` counts only issues the bot opened. Wet-push branch names are validated.
- **Topics.** `github_repository_topics` replaces the inline `topics` of `github_repository` in the Component module.
- **Bootstrap.** `idp bootstrap` accepts several approvers and can create the bot App.
- **Contract tests.** The curated Phase 1c minors were closed in `internal/workflowspec` (ruling D-13).

How it was executed: subagent-driven, as in Phase 1c. Each task had an implementer and a separate reviewer, with fix rounds on findings. A whole-branch review by the most capable model followed Task 9, and one fix wave closed it. A residual finding outside the wave (the bot token lifetime) got its own fix and review. The live runs then found one defect (topics), which got its own fix, review and re-review.

CI evidence on idp-engine, all green (`go`, `workflows`, `modules (github/component)`, `modules (github/group)`, `render-smoke`):

- run 38778842886 on PR #8 at `bdcb83d`, before the final review;
- run 38784074334 at `8e63304`, after the final fix wave;
- run 38821994114 at `fbbb471`, after the account swap, the last commit before the merge.

PR #9 (the topics fix) passed CI at `0947f0a` (run 38833958777) before its merge.

Live harness evidence. Run 1 failed at the update step and exposed the topics defect; after the fix, run 2 (dispatched) and two nightly runs passed (see "The live harness runs").

## Tasks

"Review" is the review done after the task by a separate reviewer. Commit ranges are `base..tip` on `feat/phase-1d-e2e-harness`, created from `main` at `1274f81` (the plan commit).

| Task | What | Commits | Review |
|---|---|---|---|
| 1 | ADR-0026, ADR-0027 and spec amendment A8 | `1274f81..f6436f8` | Clean |
| 2 | Gate ignores plan comments edited by others (GraphQL editor read) | `f6436f8..73c12fd` | Clean |
| 3 | `FindOpenIssue` requires the bot author; wet-push branch validation | `73c12fd..6005e1c` | Clean |
| 4 | Component topics through `github_repository_topics` | `6005e1c..16504d2` | Clean |
| 5 | `bootstrap`: several approvers, bot App role | `16504d2..38c6f9d` | Clean |
| 6 | Contract-test hardening | `38c6f9d..6ba5227` | Clean |
| 7 | Harness scenario (`internal/e2e`) | `6ba5227..335440f` | Clean after 2 fix rounds |
| 8 | GitHub forge and the `idp-e2e` command | `335440f..2bd40fc` | Clean |
| 9 | `e2e.yaml` workflow, `docs/e2e.md` | `2bd40fc..bdcb83d` | Clean |
| 10 | Push, PR #8, CI, whole-branch review, fix wave, token fix, credentials | PR #8; wave `bdcb83d..8e63304`; token fix `8e63304..bd4480b`; token follow-up `87702ba`, `fe7acaf`; account swap `dbc1899`, `7441c3c`, `fbbb471` (and `78eeb9a` on `main`) | Whole branch: with fixes (0 Critical, 6 Important, 6 Minor), then all addressed after the wave and a scoped re-review; token fix: approved; follow-up checked by the controller |
| 11 | Merge, bootstrap, live runs | merge `3b9beed`; topics fix PR #9 (`8a1da0b`, `4aacad7`, `357a856`, then `5b4946c`, `0947f0a`), merge `344f55d` | Topics fix: needs fixes (1 Important), then all addressed on re-review |
| 12 | Nightly run and this log | this commit | not applicable |

Task details:

- **Task 1.** `f6436f8` records the decisions as ADR-0026 (automation App) and ADR-0027 (end-to-end harness) and spec amendment A8.
- **Task 2.** `73c12fd`. The exact GraphQL query was verified live by the controller (the `editor` of a bot-authored comment is null).
- **Task 3.** `6005e1c`. Closes the `FindOpenIssue` item parked at the Phase 1c fix wave and Phase 1b M7.
- **Task 4.** `16504d2`. The `tofu test` suite passed 9/9 for the component module.
- **Task 5.** `38c6f9d`. `--approver` is repeatable and `--role bot` creates the bot App.
- **Task 6.** `6ba5227`. See "Final disposition of every finding".
- **Task 7.** `6383427` (scenario), fix round 1 `379ecf8`, fix round 2 `335440f`. Reviewed by the most capable model; see rulings T7-I1 and T7-I2.
- **Task 8.** `2bd40fc`. Adds the GitHub implementation of `ClaimFiles`, `OrgResources`, `ActiveRuns` and `Reject`, and a retry on 5xx and transport errors for GET requests only. Review checked the named risks: reads-only retry, approver token only on `pending_deployments`, escaping, exit codes.
- **Task 9.** `bdcb83d`. actionlint clean and zizmor with no findings.
- **Task 10.** The local verification passed: `go test`, `go vet`, `gofmt`, actionlint, zizmor, and `tofu fmt` and `tofu test` for the component (9/9) and group (5/5) modules. The whole-branch review (see "Notable findings caught by review") produced the fix wave `bed3e9b` (code) and `8e63304` (docs). A residual finding outside the wave, the bot installation token expiring after one hour, produced `49c0627` and `bd4480b`, then a follow-up `87702ba` and `fe7acaf`. Credentials were set up in the same task (see "Credentials and bootstrap").
- **Task 11.** The merge, the bootstrap re-apply and live run 1 (see "The live harness runs"), which found the topics defect.

## Rulings

A ruling is a controller or owner decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong. The labels are the ledger's.

Planning rulings (all recorded in the plan; D-1 to D-4 and the credential part of D-2 are owner decisions):

- **D-1. A third App, `<org>-bot`, for the harness, release-please and Renovate (ADR-0026).** *Why:* pull requests opened with the workflow token do not trigger CI, and no org setting was to be weakened. *Cost if wrong:* one more key to rotate.
- **D-2. A second reviewer identity is needed to approve in `idp-claims-e2e`, because only a person can approve a deployment (ADR-0027).** *Why:* the harness must approve the deployments of its own reconcile runs. *Cost:* an expiring token. The credential finally chosen is in ruling "Account swap" below.
- **D-3. Renovate is self-hosted with the bot App, in Phase 1e.** *Cost if wrong:* a different update bot later.
- **D-4. The workflows download and verify the release binary and fall back to a source build, in Phase 1e.** *Cost if wrong:* the source build stays.
- **D-5. Re-split Phase 1 into 1d (harness and hardening) and 1e (releases, Renovate, `v0.1.0`, production onboarding).** *Why:* size, and the harness must exist before release PRs can run it. *Cost:* one more plan.
- **D-6. The harness is `cmd/idp-e2e` over `internal/e2e` and is not shipped.** *Cost:* none.
- **D-7. The harness edits files with the Contents API, not `wetpush`.** *Why:* it needs single-file deletes. *Cost:* one commit per file.
- **D-8. Coverage excludes partial apply, recover and forks.** *Why:* the Phase 1c smoke proved them. *Cost:* those paths are not re-proven nightly.
- **D-9. Triggers are nightly plus dispatch, from `main`; release PRs wait for Phase 1e.** *Cost:* none.
- **D-10. Phase 1b minors.** M1 (forget column) goes to Phase 4, where `removed` blocks may first appear. M4 (wet marker for `--wet`) is won't fix: the path is a constant pinned by a contract test, and a marker would need a migration of every wet branch. M7 (branch escaping) is fixed by validating branch names (`6005e1c`). *Cost:* low.
- **D-11. Topics go through `github_repository_topics`.** *Why:* the plan said the provider's `topics` attribute on `github_repository` is Optional and Computed, so it never clears. No managed repositories existed, so no state move was needed. The plan also said the provider's documentation promises that an empty array clears the topics. That claim turned out to be wrong for the new resource (see live run 1). *Cost if wrong:* a create of the topics resource on existing repos later.
- **D-12. The gate editor check (Phase 1b I1 option B, Phase 1c ruling P-4) is implemented now.** *Cost:* one GraphQL call per gate.
- **D-13. Phase 1c contract-test minors.** A curated set is fixed (see the disposition list); the rest go to Phase 5. *Cost:* low.
- **D-14. The optional narrower plan token stays an owner note (ADR-0025); `adopt`'s read-only token is Phase 4.** *Cost:* none in this phase.

Execution rulings:

- **Bootstrap re-apply timing.** The `idp-claims-e2e` bootstrap re-apply runs immediately before the live harness run, not earlier. *Why:* the e2e drift workflow still ran the Phase 1c engine, whose parameter parser rejects the new `approverIds` field; the harness's first step re-pins the callers. *Cost if wrong:* one scheduled e2e drift run could fail in the window.
- **Task 7 I1.** Preflight refuses on any `e2e-` claim on `main`, any `e2e-r` repo or team, and any unfinished reconcile, drift or recover run. The pin must plan "No infrastructure changes.". The cleanup delete is tolerant (no WARNING required, approves only if asked); the main-path delete stays strict. *Why:* leftovers were never detected, and cleanup could not remove claims that were never applied. *Cost if wrong:* a stale e2e resource blocks the nightly until someone cleans it, and it fails loudly.
- **Task 7 I2.** A reconcile of the harness's own merge that unexpectedly waits for approval is rejected with the approver token (nothing is applied, the queue frees), recorded in ADR-0027. *Why:* an unexpected approval wait held the `idp-wet` queue and deadlocked the cleanup. *Cost if wrong:* a rejected run that a human wanted, in a repo the harness owns.
- **Task 7 minors fixed in the same round.** The delete refuses foreign destroy or replace rows; input validation; the drift run is found by unseen id; the `change()` signature was trimmed; `plan.CommentMarker` is reused. *Cost:* none.
- **Forge interface growth.** `Forge` gained `ClaimFiles`, `OrgResources`, `ActiveRuns` and `Reject`; Task 8 implements them on GitHub, with the GET-only retry. *Cost:* none.
- **`OrgResources` reads public org repos with the repo-scoped bot token.** The harness repos are public and the teams need `members:read`. *Cost if wrong:* leftover repos without a claim or team would slip past preflight (the team and claim checks still catch most). The ledger asked to prove this live; it worked (see "Watch items").
- **Task 7 fix round 2.** The destroy check reads the fingerprint marker (`plan.DecodeMarker`) instead of rows; `flipCase` has a lowercase fallback; preflight matches `^e2e-r[a-z0-9]{1,12}-(app|team)$`; templates are required. *Why:* the row-based check missed truncated plans, and the cleanup auto-approves. *Cost:* none.
- **Task 10 order.** Steps 1 to 3 (verify, push and PR, CI), then the final whole-branch review and its fix wave, then the credentials, right before Task 11. *Cost if wrong:* none, no step depended on the credentials earlier.
- **Final wave I1.** The harness checks the editor on the plan comment (parity with the gate), requires `main` to equal its own last merge before a delete, and requires the merge's parent to equal the PR's base, else it rejects. *Why:* a person's approval token must not be usable as a confused deputy by an unattended nightly. *Cost if wrong:* more forge surface to prove live.
- **Final wave I2 to I6 and M1 to M6.** `Merge` is pinned to the verified head sha (I2); the approver token is probed in preflight (I3); the harness deletes the branches it merged and a re-run guard refuses `run_attempt != 1` (I4); the `idp-default-branch` ruleset is asserted active and the team is asserted to maintain the repo without admin (I5, "files" go to Phase 4); the bot key rotation runbook was corrected (I6); the harness builds without secrets (M1); the waits are budgeted under the job timeout, 15/30/15 minutes with a 330-minute job (M2); the harness closes its own open PRs on failure (M3); docs for M4 and M6; the marker head must equal the PR head (M5). *Cost if wrong:* more to prove live.
- **Out of the wave.** The PR files check is not added (covered by the sha pin, the editor check, the fingerprint and the parent check). A Merge 405 retry is added only if seen live. The re-pin upgrade note goes to the Phase 1e release notes. *Cost:* low.
- **Bot token lifetime.** A bot installation token expires after one hour while a run budget is about 300 minutes. The harness mints and refreshes its own tokens (`internal/ghapp`, `ghapi.NewWithTokens`), scoped to `idp-claims-e2e` and the forge's permissions; the App key reaches only the harness step, built without secrets; ADR-0027 was amended. *Cost if wrong:* the App key sits in one more process environment, with the same trust as the action that minted the token before.
- **Head injection before `OpenPR` is not fixed.** A foreign commit pushed before the PR opens can only add creates; destroys are fingerprint-checked and creates never need approval, so a writer gains nothing through the harness that they could not merge directly. This corrects the earlier ledger reason. *Cost if wrong:* a foreign create rides a harness merge in the e2e repo.
- **Token probe scope.** The approver probe catches expired or revoked tokens only, not a lost Deployments write. Accepted, with hedged wording in the docs.
- **Token-refresh minors fixed now.** A cached-token fallback when a refresh fails (more than one minute left), an empty-token guard in `ghapi`, a docs entry for a mint failure, and a runbook step to keep the bot key until the e2e setup. *Cost:* none. Stale text in the historical plan was left.
- **Account swap.** Owner decision of 2026-10-16. The approver is a fine-grained token of `jellalshadows`, limited to `idp-claims-e2e`, with Actions read and Deployments read and write; `idp-claims-e2e` keeps `jellalshadows` as its only `idp-approval` reviewer. The harness team's member is `Adrian-Manuel`, the owner's second personal account. The second account first chosen was not a dedicated test account and was replaced; it is no longer an org member. Current files that named it were updated, without a history rewrite. ADR-0027's rejected alternative "the owner's token" was amended, not contradicted: it targeted an unscoped token, and a fine-grained token never exceeds its grant, so the exposure equals that of a second account's token. *Cost if wrong:* harness approvals are indistinguishable from the owner's in the e2e audit log; reverting is one secret plus one `--approver`.
- **Merge before the live run.** Task 11 could not run from the branch: `workflow_dispatch` needs the workflow file on the default branch, and `e2e.yaml` was not on `main` (`gh workflow list --all` lacked it; `contents?ref=main` returned 404). Owner decision: merge PR #8 (`3b9beed`) first and make the first live run from `main`. The temporary branch policy was never created. *Cost if wrong:* live-run bugs are fixed in follow-up PRs while `main` carries the harness; callers pin by SHA, so nothing else moves.
- **Topics fix, no gate exception.** Removing the last topic destroys the topics resource, so it waits for approval like every delete (owner decision). The module uses `count` on `github_repository_topics` with no `moved` block: no state holds the unindexed address (Phase 1d was unreleased, the harness repo is deleted, `idp-claims` is unmanaged). The harness update step requires exactly the topics delete and approves only it, with the same base, head-fingerprint and merge-parent checks as the delete step. A run therefore has two approvals. *Cost if wrong:* one more live-run iteration.

## Credentials and bootstrap

- **Bot App.** `jellalshadows-idp-bot` (id 5337822) was created with `idp bootstrap app --role bot`. Its installation (171893621) is `selected`: exactly `idp-engine`, `idp-claims` and `idp-claims-e2e`, verified with a metadata-only bot token that was then revoked. The environment `idp-e2e` on `idp-engine` has a branch policy for `main`, the secret `IDP_BOT_PRIVATE_KEY` and the variable `IDP_BOT_CLIENT_ID`.
- **Approver.** The approver token is a fine-grained token of `jellalshadows`, limited to `idp-claims-e2e`, with Actions read and Deployments read and write, stored as `IDP_E2E_APPROVER_TOKEN` in `idp-e2e`. ADR-0027 was amended for it. `Adrian-Manuel` accepted the org invitation and is the harness team's member.
- **Bootstrap, `idp-claims` (production).** `bootstrap check` found only `IDP_BOOTSTRAP` differing (expected, because Phase 1d changed the record). `apply` updated it and the re-check showed no drift. Harmless, because `idp-claims` has no workflows yet.
- **Bootstrap, `idp-claims-e2e`.** Applied right before live run 1, with `jellalshadows` as the only approver: `check` found only `IDP_BOOTSTRAP`, `apply` updated it, and the re-check showed no drift.

## The live harness runs

Repository under test: `jellalshadows-idp/idp-claims-e2e`. The harness workflow runs in `jellalshadows-idp/idp-engine`; run ids are on `https://github.com/jellalshadows-idp/idp-engine/actions/runs/<id>`. All runs started from `main`.

| Run | Trigger | Result | PRs | Duration |
|---|---|---|---|---|
| [38827460953](https://github.com/jellalshadows-idp/idp-engine/actions/runs/38827460953) | dispatch | **Failed** at the update step; cleanup worked | #10 to #13 | about 17 min (00:05 to 00:22 UTC, 2026-10-16) |
| [38834099590](https://github.com/jellalshadows-idp/idp-engine/actions/runs/38834099590) | dispatch | Passed | #14 to #18 | about 24 min (01:15 to 01:38 UTC, 2026-10-16) |
| [38874029988](https://github.com/jellalshadows-idp/idp-engine/actions/runs/38874029988) | nightly | Passed | #19 to #22 | about 20 min (08:29 to 08:49 UTC, 2026-10-16) |
| [39015815202](https://github.com/jellalshadows-idp/idp-engine/actions/runs/39015815202) | nightly | Passed | #23 to #26 | about 20 min (08:01 to 08:21 UTC, 2026-10-17) |

**Run 1.** Preflight passed. The pin PR #10 and the create PR #11 passed, including the new ruleset and team assertions. The update PR #12 failed: the repository kept its topics (`[e2e harness]`) after the claim removed them all. The harness cleanup then worked: delete PR #13 was approved (reconcile run 38828904337), the resources were gone, and no branches or issues were left.

*Root cause.* In the GitHub provider source (v6.13.0 and current `main`, `resource_github_repository_topics.go`, around line 55), create and update call `ReplaceAllTopics` only for a non-empty set, and only delete clears the topics. So an empty array does not clear them. Plan ruling D-11 said the provider's documentation promised that an empty array clears the topics; the code does not do that. The fix is PR #9, merged as `344f55d`: the module creates the topics resource only when the list is non-empty (`count`), so removing the last topic destroys the resource, which waits for approval. It was reviewed (one Important finding: the docs and a comment claimed a no-drift check before step 3, plus minors), went through fix round 1 (`5b4946c`, `0947f0a`) and a re-review that found everything addressed.

*Evidence for the fix.* The module test now asserts `github_repository_topics.this[0].topics` for a non-empty list and `length(github_repository_topics.this) == 0` for an empty one; `tofu test` passed 9/9. The harness update step is covered by `TestUpdateApprovesExactlyTheTopicsDelete`, `TestUpdateRefusesAPlanThatDestroysAnythingElse`, `TestUpdateRefusesAPlanThatIsNotDestructive`, `TestUpdateRequiresTheFingerprintOfThePRHead`, `TestUpdateRefusesWhenMainMovedSinceTheLastMerge`, `TestUpdateRejectsWhenMainMovedWhileItsPRWasOpen`, and in round 1 `TestUpdateRequiresTheDestructiveWarning` and `TestUpdateRequiresTheReconcileToAskForApproval`.

**Runs 2 to 4.** Each run did the same sequence: pin the callers (PR #14 in run 2; on the nightlies the pin was a no-op, "the callers already pin 344f55d"), create the Group and the Component (resources match the claims), update and remove the topics (approved reconcile), change only the case of a member login (it planned no change), a drift run with no drift, delete the claims (approved reconcile, resources gone), and a final drift run with no drift. Two approvals per run. The approved reconcile runs were 38835074665 and 38835989021 (run 2), 38874834589 and 38875925001 (run 3), and 39016273879 and 39016856478 (run 4).

The schedule in `e2e.yaml` says 03:41 UTC, but GitHub started the nightly runs at about 08:00 to 08:30 UTC. GitHub delays scheduled runs under load; the times above are the observed ones.

### Watch items from Task 11

- **Seen, and they work.**
  - The GraphQL editor read with the bot installation token (the update and delete steps read the plan comment through it).
  - The ruleset read and the team's repository read with the scoped bot token on a public repository outside the installation scope: the create step's assertions passed in all four runs.
  - The empty-topics set: it failed in run 1 and was fixed.
- **Not observed.** The token refresh never ran live: no run passed the one-hour mark of a minted token, so only the unit tests cover the refresh.
- **Not observed in four runs.** A `DeleteBranch` answering 422 and a `Merge` answering 405. No run log or ledger entry records either.

## Notable findings caught by review

| Finding | Where caught | Fix |
|---|---|---|
| Row-based destroy check missed truncated plans, and cleanup auto-approves | Task 7 re-review | Task 7 round 2, `335440f` (fingerprint marker) |
| Leftovers were never detected and cleanup could not remove claims that were never applied | Task 7 review (I1) | Task 7 round 1, `379ecf8` |
| An unexpected approval wait held the `idp-wet` queue | Task 7 review (I2) | `379ecf8`: reject it |
| The harness approved on a plan comment others can edit | Final review I1 | `bed3e9b`: editor check, main equals last merge, merge parent equals PR base |
| `Merge` was not pinned to the verified head | Final review I2 | `bed3e9b`: sha-pinned merge |
| An expired approver token was found only after resources existed | Final review I3 | `bed3e9b`: probe in preflight |
| Merged branches stayed and re-run attempts collided | Final review I4 | `bed3e9b`: delete merged branches, re-run guard |
| The component ruleset and the team's grant were not asserted | Final review I5 | `bed3e9b` |
| The bot key rotation procedure was wrong | Final review I6 | `8e63304` |
| The bot installation token lasts one hour while a run budget is about 300 minutes | Residual of the final review | `49c0627`, `87702ba` |
| Setting topics to empty did not clear them | Live run 1 | PR #9, `344f55d` |

Findings checked and found not to be problems: the head injection before `OpenPR` (a foreign commit can only add creates, which need no approval); the editor-check ordering window (same as the gate's, accepted).

## Final disposition of every finding

Statuses: **Fixed** (done, with the commits), **Phase 1e**, **Phase 4**, **Phase 5**, **Won't fix** (consciously rejected, with the reason). No finding is left open without a destination.

### Fixed

- Task 7 Important findings I1 and I2 and the round 2 minors: **Fixed**, `379ecf8` and `335440f`; the scoped re-reviews found them addressed.
- Final review I1 to I6 and M1 to M6: **Fixed**, `bed3e9b` and `8e63304`; the scoped re-review addressed all twelve and found nothing new.
- Bot token lifetime: **Fixed**, `49c0627`, `bd4480b`, `87702ba`, `fe7acaf`.
- Topics did not clear (live run 1): **Fixed**, PR #9 (`344f55d`); the Important review finding and its minors: **Fixed**, `5b4946c` and `0947f0a`.
- Phase 1c carry-over, GraphQL editor check for the gate: **Fixed**, Task 2 (`73c12fd`).
- Phase 1c carry-over, `FindOpenIssue` must require the bot author: **Fixed**, Task 3 (`6005e1c`).
- Phase 1b M7 (`wetpush` branch escaping): **Fixed** by branch validation, `6005e1c`.
- Phase 1c contract-test minors fixed in Task 6 (`6ba5227`): the reusable workflow list is explicit (`reusableNames`, `internalWorkflows`); `# unreleased` is accepted only for refs under `/idp-engine/.github/workflows/`; YAML anchors and aliases are rejected; `-refresh` is forbidden in the reconcile and drift plan steps; the `idp-gate` row was added to `docs/pipelines.md`.
- The harness workflow secret guard that was a substring check (Task 9 minor): **Fixed** in the token fix (`TestE2ESecretsReachOnlyTheHarness`).

### Phase 4

- Phase 1b M1 (forget column), where `removed` blocks may first appear (D-10).
- `adopt` needs a read-only token again, because the reconcile no longer holds the reader (D-14).
- Asserting the component repository's files in the harness (final review I5, "files").

### Phase 5

The reasons below are the plan's ruling D-13 (the remainder is over-strict matching, wording and refactors, with low risk and no effect on behaviour) and the reviewers' own notes.

- **Phase 1c contract-test and polish minors not closed in Task 6.** `usesRefs` matches any `uses` key; the `docker://` message; `commandFields` skips only a first-field `echo` or `printf`; `checkTofu` inspects the first `tofu` per line and reads `-chdir stack` as the subcommand; `TimeoutMinutes` and `Environment` fail loudly on expression forms; `wantRun` substring matching; guards and `wet-base` tests pin only the `if` line; tests do not pin the push token after base, `close` after push without `always()`, or the drift `*) exit 1` branches; `writerKeyCaller` accepts any owner's `idp-engine` (templates pin the owner); template pin freshness is checked only by the re-pin discipline; `recover with:` is compared to a hard-coded map; the `pr` workflow items (step conditions not pinned because they fail closed, state-copy path, stale sticky comment, validate render path, wording); the reconcile and drift items (issue body with a missing artifact, first 100 issues only, recover's issue match, drift issue wording, `check` exit 1, error path wording, bare bash error on missing `wet-base`); the render items (escaper duplication, untested topic, name and member escaping, escape table cases, `claims.md` wording). *Reason:* each is cosmetic, fails closed, or has no behavioural effect today; D-13 fixed only the ones that closed a real blind spot. *Not verified:* whether any of these was fixed incidentally in Task 6; the ledger lists only the five fixed items above.
- **Other Phase 1b test and polish minors** listed in the [Phase 1b log](phase-1b.md) that were not named in D-10. *Reason:* the same.
- **Task 2 minors.** The `... on IssueComment` fragment reads a non-IssueComment node as never edited (unreachable, because the id comes from a REST comment; add a `__typename` check); an empty `NodeID` is unguarded (the real API errors; the fake would pass); no end-to-end tests for a GraphQL error or a null node through the gate; the docs do not spell out exit 1 on an unreadable editor; the `/graphql` path is github.com-only (GHES uses `/api/graphql`). *Reason:* unreachable or fail-closed today.
- **Editor-check TOCTOU.** The comment body and its editor are read in two requests, in the gate and in the harness. *Follow-up:* read both in one GraphQL query. *Reason:* the window needs the PR workflow to re-run on the same head inside the window, and the sha-pinned merge and head checks refuse the other cases.
- **Task 3 minors.** `validateBranch` checks `.lock` only at the end and allows a segment ending in `.` (not an escaping risk); the reconcile and recover `jq` filters check `user.login` but not `user.type == Bot`; no test for `x.lock/y`.
- **Task 4 minors.** The `mock_provider` test pins wiring, not provider clearing (proven live in run 1 and runs 2 to 4); no assert that `github_repository` has no topics; an `error_message` cites an internal tracker pointer.
- **Task 5 minors.** Drift reports `IDP_BOOTSTRAP` until `apply` rewrites it (a planned side effect); the duplicate-login message names the id; `--approver ""` reaches `UserID`; the indirect `--role bot` acceptance test; the runbook step 2 "All repositories" lacks a pointer to the bot exception.
- **Task 6 minors.** `checkUses` matches any owner's `idp-engine` repo name (plan-mandated); the `-refresh` check is a substring of `run` (`TF_CLI_ARGS_plan` would escape it); no reverse check for listed-but-missing internal workflows.
- **Task 7 minors.** The reconcile run is picked by head SHA, so a manual dispatch of `reconcile.yaml` at the same SHA could become `runs[0]` (approve and reject path); `TestDeleteRefusesACommentWithoutFingerprint` hits the no-marker branch, so the unreadable-marker error branch is untested.
- **Task 8 minors.** A duplicate `var _ Forge = GitHub{}` in `github_test.go`; contents paths are not `PathEscape`d (internal fixed paths); a body-read failure is not retried; a template read error exits 1 (brief-mandated).
- **Task 9 minors.** The secret guard wording "Both live only in".
- **Task 10 concerns.** `DELETE git/refs` on a missing ref may answer 422 rather than 404 (logged only, never observed); a `Parent` read failure after the delete merged returns at once and leaves a waiting reconcile holding the `idp-wet` queue until a person acts (accepted, documented); the refresh path is covered only by unit tests; `-race` could not run locally (no cgo), CI runs it.
- **Topics fix deferred minors.** The `waitReconcile` mode and note coupling (`scenario.go`, around line 645) stays; the head check in `fingerprintOf` is unreachable after `waitChecks` and is kept as defence in depth (guarded by `TestUpdateRequiresTheFingerprintOfThePRHead`); the RED tofu output was only a tail in the report.

### Phase 1e

- Spec and ADR wording left stale by Task 1: the spec §7.3 heading still says "Two GitHub Apps" (renaming breaks the ADR-0026 anchor); the §7.3 role text omits `<org>-bot`; the ADR-0018 accepted-risk lead-in is partly stale (plan-mandated); the §10 table Phase 1 row still says it ends at 0.1.0, which moved to Phase 1e. The ledger records these as deferred without naming a phase; Phase 1e is the natural place because it edits the same text.
- The upgrade note for re-pinning callers (see below).

### Won't fix

- Phase 1b M4 (wet marker for `--wet`): the path is a constant pinned by a contract test, and a marker would need a migration of every wet branch (D-10).
- The PR files check in the harness: covered by the sha pin, the editor check, the fingerprint and the parent check.
- The head injection before `OpenPR`: see rulings.
- A Merge 405 retry: not added, because it was never seen live.

## Carried into Phase 1e

- Releases: release-please, GoReleaser, attestations; Renovate self-hosted with the bot App (D-3); `v0.1.0`; the templates re-pin to the release and drop `# unreleased`.
- Workflows that download and verify the release binary, with the source build as a fallback (D-4).
- Onboarding the production `idp-claims`, pinned to a release.
- The upgrade note that re-pinning callers needs `bootstrap apply`, because the `IDP_BOOTSTRAP` format changed (the old engine's parser rejects `approverIds`).
- The harness running on release pull requests (D-9).
- Optionally, reporting the provider's documentation and code mismatch for `github_repository_topics` upstream.
- The Task 1 doc touch-ups listed above.
- Carried from Phase 1c and still open: the optional narrower plan token (`administration: write` with the rest read), only after a live run proves the narrower set suffices (ADR-0025).
- Carried to Phase 5: the editor TOCTOU single-query follow-up and the deferred minors listed above.

## Owner notes

- Rotate the approver token before its expiry date; the procedure is in [docs/e2e.md](../e2e.md), section "Rotating the approver token". A run with an expired token fails at preflight, before it creates anything.
- The bot key rotation procedure is in [docs/runbooks/bootstrap.md](../runbooks/bootstrap.md). `apply` never writes the bot key secret, so a rotation also sets `IDP_BOT_PRIVATE_KEY` in the `idp-e2e` environment.
- `Adrian-Manuel` must stay an org member, because the harness's team uses it as the member.
- The pending owner items of earlier phases stay as recorded in their logs ([Phase 1c](phase-1c.md), "Owner notes", and earlier); they are not new asks.

## Process notes

- **Execution:** subagent-driven. Fix rounds used: Task 7 (2), the final wave (1), the bot token fix (1 plus a follow-up), the topics fix (1).
- **Naming rule:** the naming check ran on the work of each fix and on the account swap, and was clean on `main`.
- **Local verification constraint:** no local container tooling; `-race` could not run locally (no cgo). CI runs both.
- **Ledger note:** the Task 11 ledger entry added during the final wave is kept in the scratch ledger only, because `.superpowers/` is excluded from git.

## Status

**Phase 1d is complete.** The harness, the gate and topics hardening and the contract-test closures shipped, verified by green CI runs (38778842886, 38784074334, 38821994114, and 38833958777 for PR #9), by live run 2 and by the two nightly runs (see "The live harness runs"). Every finding has a final disposition above.

Next: Phase 1e (releases, Renovate, `v0.1.0`, onboarding `idp-claims`).
