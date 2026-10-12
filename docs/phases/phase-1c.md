# Phase 1c execution log: reusable pipelines

- Period: 2026-10-11 to 2026-10-13
- Plan: [2026-10-11-phase-1c-pipelines.md](../superpowers/plans/2026-10-11-phase-1c-pipelines.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md) (§6 workflows, §7 trust and gate, §10 roadmap), amended by A6 and A7
- Decisions: ADR-0021 to ADR-0025 (see "Spec and ADR changes")
- Previous phase: [Phase 1b log](phase-1b.md)
- Status: **complete** (merged through [PR #7](https://github.com/jellalshadows-idp/idp-engine/pull/7), merge commit `a5445a2`).

This file is the durable copy of the execution ledger that was kept while the plan ran. The ledger lived in a git-ignored scratch folder that is deleted later, so everything it recorded is reproduced here: the rulings, every task's review outcome, every finding with its final disposition, the live smoke, and what is carried into the next phase.

## Summary

Phase 1c turns the Phase 1b subcommands into pipelines. A claims repo now reconciles end to end: a pull request is validated and planned, a merge is gated and applied, the state is committed to `wet`, drift is checked daily, and a failed wet push can be recovered with one dispatch.

- Four reusable workflows: `pr`, `reconcile` (plan, approve, apply-github), `drift` and `recover`. The required `idp-gate` check lives in the caller (ADR-0022).
- The `setup-idp` composite action builds `idp` from the workflow's own commit (ADR-0021), so a caller pinned to a commit runs exactly that engine.
- Caller templates under `examples/` for a claims repo, pinned to an engine commit with an `# unreleased` marker until releases exist.
- A contract-test harness (`internal/workflowspec`) that pins the security-relevant shape of every workflow and template: token scopes, which jobs may hold the writer key, `tofu` subcommands, environments, concurrency, and the common rules.
- ADR-0021 to ADR-0025, spec amendments A6 (pipeline decisions) and A7 (plan tokens), the pipelines guide ([docs/pipelines.md](../pipelines.md)) and the pipelines runbook ([docs/runbooks/pipelines.md](../runbooks/pipelines.md)).
- A render hardening that was not in the plan (Task 4b): claim text can no longer inject template sequences into the rendered OpenTofu JSON.

How it was executed: subagent-driven. For each task, the controller dispatched an implementer, then a separate reviewer; findings went back as fix rounds (at most five per task). A whole-branch review by the most capable model preceded the smoke run, and one fix wave closed it. The live smoke then stopped at S3 on a design-level defect; the owner decided the fix, a scoped fix loop with its own review followed, and the smoke resumed and passed. The work was pushed as [PR #7](https://github.com/jellalshadows-idp/idp-engine/pull/7) and merged with a merge commit (`a5445a2`), which keeps the per-task commits.

CI evidence on idp-engine, all fully green (`go`, `workflows` including shellcheck and zizmor, `modules (github/group)`, `modules (github/component)`, `render-smoke`):

- run [38266246040](https://github.com/jellalshadows-idp/idp-engine/actions/runs/38266246040), the first run on PR #7 (before the final review);
- run [38301969850](https://github.com/jellalshadows-idp/idp-engine/actions/runs/38301969850) at `575fbc1`, the end of the final fix wave;
- run [38310955401](https://github.com/jellalshadows-idp/idp-engine/actions/runs/38310955401) at `089c63e`, the end of the smoke fix loop.

Live smoke. Steps S0 to S11 passed in `idp-claims-e2e` (see "The live smoke"). After the merge, [idp-claims-e2e PR #9](https://github.com/jellalshadows-idp/idp-claims-e2e/pull/9) re-pinned the four callers to `a5445a2`: all checks passed including `idp-gate` (run [38314317870](https://github.com/jellalshadows-idp/idp-claims-e2e/actions/runs/38314317870)) and it merged as `1dd76a4`. Its reconcile run [38314547633](https://github.com/jellalshadows-idp/idp-claims-e2e/actions/runs/38314547633) succeeded with `diff: 0 affected stack(s)` and `gate: auto (no changes to apply)`; `approve` and `apply-github` were skipped, because a render with no claims has no module refs.

Bootstrap re-apply during the phase: `idp-claims-e2e` and `idp-claims` each got only `created variable IDP_BOOTSTRAP` (the callers read it), the secrets were kept, and `bootstrap check` reported no drift afterwards.

Scope that moved to Phase 1d: the E2E harness, the release machinery and `v0.1.0`, and onboarding the production `idp-claims` repo with a pinned release.

## Tasks

"Review" is the review done after the task by a separate reviewer. Commit ranges are `base..tip` on `feat/phase-1c-pipelines`, created from `main` at `8c4c5d3` (the plan commit).

| Task | What | Commits | Review |
|---|---|---|---|
| 1 | ADR-0021 to ADR-0024 and spec amendment A6 | `8c4c5d3..bcd3d99` | Clean |
| 2 | Workflow contract harness and common rules | `bcd3d99..1249a30` | Clean after 1 fix round (4 Important) |
| 3 | `setup-idp` composite action | `1249a30..f9cebd1` | Clean |
| 4 | Reusable `pr` workflow | `f9cebd1..c61fd5b` | Clean |
| 4b | Controller-added: escape template sequences in claim text | `c61fd5b..ffedcf1` | Clean |
| 5 | Reusable `reconcile` workflow | `ffedcf1..97116d0` | Clean after 1 fix round (1 Important) |
| 6 | Reusable `drift` and `recover` workflows | `97116d0..90b8f55` | Clean |
| 7 | Claims repo caller templates and their lint | `90b8f55..9f50553` | Clean |
| 8 | Pipelines guide and runbook | `9f50553..6c56481` | Clean |
| 9 | Push, PR #7, CI, bootstrap re-apply, whole-branch review, fix wave | PR #7; fix wave `6c56481..575fbc1` | Whole branch: with fixes (1 Critical, 3 Important), then clean after the wave and a scoped re-review |
| 10 | Live smoke in `idp-claims-e2e`, stopped at S3, fix loop, resumed | fix loop `575fbc1..089c63e` | Smoke fix: no Critical or Important |
| 11 | Merge, post-merge re-pin, this log | merge `a5445a2`; e2e PR #9 | not applicable |

Task 4b was added by the controller. While reviewing Task 4, a reviewer noted out of scope that claim descriptions reach `main.tf.json` verbatim, and OpenTofu evaluates `${...}` and `%{...}` inside JSON strings. The controller verified it in `internal/render/render.go`: a claim could inject a template that reads a token or `TF_ENCRYPTION`, including the writer token during the reconcile apply. Claims are data (spec §7.1) and the reconcile runs with the writer key, so the render now escapes `${` to `$${` and `%{` to `%%{` in every claim-derived string; `owner_team_id` stays a live reference. A fixture, a golden file and the `render-smoke` CI job (`tofu validate`) prove OpenTofu accepts the escaped output, and `docs/claims.md` says so.

Final fix wave commits (`6c56481..575fbc1`): `7187b79`, `063a67c`, `917e795`, `488f041`, `575fbc1`. Smoke fix loop commits (`575fbc1..089c63e`): `43515fc`, `aa0f2b4`, `5730de3`, `089c63e`.

## Rulings

A ruling is a controller decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong. The labels are the ledger's, in the order they were made.

Planning rulings:

- **P-1. Add a `recover` workflow (ADR-0024).** Spec §6.2 has the artifact and the issue but no button. *Cost if wrong:* one extra workflow to delete.
- **P-2. Templates pin a commit with `# unreleased` until releases exist.** No release exists yet. *Cost:* re-pin churn until Phase 1d.
- **P-3. Build `idp` from source at `job.workflow_sha` (ADR-0021).** No release binary yet. *Cost:* about one minute of Go build per job.
- **P-4. The GraphQL editor check (Phase 1b I1 option B) is deferred to Phase 1d.** Editors with write access can already merge with 0 approvals. *Cost:* the accepted risk stays open one more phase.
- **P-5. Phase 1b minors M1, M4 and M7 are deferred to Phase 1d.** The workflows never produce a forget, pass a constant `--wet`, and push only to `wet`. *Cost:* low.
- **P-6. Onboarding the production `idp-claims` is deferred to Phase 1d (pin a release).** *Cost:* production is not reconciled until 1d.
- **P-7. Drift does not fail the run on drift.** It opens an issue and emits a warning; the spec is silent. *Cost:* people who ignore issues would miss a signal a red X would have given.
- **P-8. The final whole-branch review comes before the smoke.** The smoke then exercises reviewed code. *Cost:* smoke fixes are reviewed only by scoped reviews.
- **P-9. A reconcile run is never re-run (attempt guard and wet-moved guard).** A re-run could apply a saved plan onto stale state. *Cost:* a transient plan failure needs a new dispatch instead of a re-run.
- **P-10. `upload-artifact` v7.0.1 and `download-artifact` v8.0.1.** v7.0.2 and v8.0.2 are under the 14-day age floor. *Cost:* none.
- **P-11. Reusable-workflow secrets are declared optional, with guard steps.** Fork PRs get no secrets and must still reach a verdict. *Cost:* a caller that forgets a secret fails at the guard instead of at parse time.

Execution rulings:

- **T9. `go run ./cmd/idp bootstrap` is allowed.** It is an operation documented in the bootstrap runbook, not a post-change build check. *Cost if wrong:* none (no artifact is left behind).
- **Task 2. Accept all four plan-mandated findings and fix them in round 1.** The spec §7 security rules bind, and the plan's code was the argument. `invokes` became field-based; the writer-key scan walks job YAML nodes (including `inherit`, `secrets[`, `toJSON(secrets)` and the approval job); `checkTofu` reads logical lines and detects the subcommand; `helpers_test.go` has inline-YAML negatives. *Cost if wrong:* about 150 extra lines of test code.
- **Task 4b. Add the claim-text escaping task.** See "Tasks". *Cost if wrong:* a Phase 1a behaviour change inside the 1c PR, small.
- **Task 5 I-1. The reconcile `plan` job refuses while an open `idp-wet-push` issue exists.** After a failed wet push the next queued or dispatched reconcile would plan against a stale `wet`. This is the cheapest guard that keeps the recovery usable; ADR-0024 gained the bullet and the accepted risk (a lost runner before the issue). *Cost if wrong:* a manually closed issue or a lost runner still lets a stale plan through.
- **Task 5 minor 7 and minor 2a fixed in the same round.** `echo $(git ...)` hid a failure, and the unconditional state copy was unpinned. *Cost:* none.
- **Task 6. The drift `wet` step uses the SHA assignment form** (the same reason as Task 5 F2), a deviation from the brief carried in the dispatch. *Cost:* none.
- **Task 8. The controller added documentation items to the brief:** the unrecovered-push guard, cancel versus a started apply, late approval after the artifact expires, the rejected-approve re-run, the cancelled and lost-runner check, the recover dead end and the cancelled-run refusal, and moving the Task 4b `claims.md` note out of the `writes:` list. The reviews of Tasks 4b, 5 and 6 had produced behaviour the brief did not document. *Cost:* none.
- **Final review C1. Callers pass the writer key by name (reverses a planning assumption).** The plan said callers never pass the writer key. GitHub requires the caller to pass an environment secret by name; without it the called workflow sees an empty string. The reconcile and recover callers (and, after ADR-0025, the drift caller) pass `IDP_WRITER_PRIVATE_KEY` by name, the reusables declare it `required: true`, and `writerKeyErrors` allows exactly that mapping for those callers only. This changes the Global Constraints, the Task 7 secrets map and ADR-0023. *Cost if wrong:* none for reach: the value resolves only in `idp-write` jobs, on `main` only.
- **Final review I1. `recover` accepts failed or cancelled runs and must match the open bot-authored "Wet push failed" issue that names the run id.** This closes the wrong-run and stuck-cancelled paths. *Cost:* a manually edited issue body can block a recovery (it fails closed).
- **Final review I2, option (a). The apply step runs `if: always() && steps.apply-token.outcome == 'success'`.** A started apply runs to completion, so OpenTofu saves the state it reached and the job commits it; it never runs after a failed step or an earlier cancel. *Cost:* a cancel cannot stop a started apply (documented, with a warning about force-cancel).
- **Final review I3 and M1 to M5 fixed in the same wave;** the other deferred minors triaged to Phase 1d. *Cost:* none.
- **Revert the fix wave's partial edit of the plan file.** The plan stays as approved; the reversal lives in ADR-0023 and this log. *Cost:* none.
- **Align `docs/pipelines.md` and ADR-0024 with the M2 guard** (the bot-authored "Wet push failed" issue). *Cost:* none.
- **Parked at the fix wave:** the guards require the author `github-actions[bot]`, but `idp issue`'s `FindOpenIssue` matches label and title only, so a pre-existing human issue with the same label and title could be reused and the guard would see zero. Real but low likelihood (needs triage access) and nothing builds on it: Phase 1d makes `FindOpenIssue` require the bot author. *Cost if wrong:* one stale-wet path for a triage-level actor.
- **Parked at the fix wave:** the runbook lost-runner bullet missed a runner lost after `keep` (artifact but no issue); ADR-0024 said a lost runner was recoverable and a cancelled run failed. Docs only, fixed in this commit. *Cost:* none.
- **Smoke S3 stop and the owner decision (2026-10-12).** See "The smoke stop at S3". The owner chose "writer on main, PR without refresh". *Cost if wrong:* the reconcile and drift plan jobs hold a write-capable token while `tofu plan` runs (main only, same pinned modules as the apply).
- **Amend spec §7.1 and add an ADR-0025 note to ADR-0009.** "Plan-time tokens are read-only" became "PR plan tokens are read-only; reconcile and drift plans on `main` use the writer (ADR-0025)", and ADR-0009's statement became false the same way. *Cost:* none.
- **Parked at the smoke fix:** ADR-0025 said "spec §7.1 is unchanged" (A7 amended it) and overstated that remediation always asks for approval (a drifted resource that is also in the PR's plan with the same action applies automatically). Fixed in this commit. *Cost:* none.

## The smoke stop at S3 and the owner decision

Steps S0 to S2 passed against engine `575fbc1`: PR #1 checks green, plan `+8`, gate `auto`, apply ok, the writer key reached the called job, and the `wet` commit `49af2fb` was verified and the state encrypted.

S3 (a clean drift run right after the apply) stopped the smoke. Drift run [38302684806](https://github.com/jellalshadows-idp/idp-claims-e2e/actions/runs/38302684806) planned `github: +0 ~2 -0 ±0` and opened a drift issue that should not exist: the smoke Component's `github_repository` merge settings changed from `false`/`null` to `true`/set, and the default-branch ruleset wanted its `bypass_actors` block added.

Root cause, verified by the controller. The GitHub API returns the admin-only fields (`allow_merge_commit`, `allow_squash_merge`, `allow_rebase_merge`, `delete_branch_on_merge`, the four merge and squash commit message fields, and the ruleset `bypass_actors`) as `null` or absent to a token without admin rights on the repository. Evidence: the owner's token sees the real values, the read-only test account sees `null`. Every plan that refreshed with the read-only reader token therefore planned phantom updates: PR plans, reconcile plans and drift plans alike. It is a design-level problem (spec §6.2 and §7.3, ADR-0017), and it touches the token model, so the controller escalated instead of patching.

The owner's decision of 2026-10-12, from the options offered: **"writer on main, PR without refresh"**.

- The PR plan keeps the reader token and plans with `-refresh=false`: it compares the claims with the state the last apply wrote, which shows exactly what the PR changes.
- The reconcile `plan` job and the `drift` job run in environment `idp-write` (deployment branch `main`) and refresh with a writer token. A dispatched reconcile can remediate drift (spec §6.4).
- `idp bootstrap check` keeps the reader token (ADR-0017 still holds).
- Recorded as [ADR-0025](../adr/0025-plans-refresh-with-the-writer-on-main.md) and spec amendment A7.

The fix loop (`43515fc`, `aa0f2b4`, `5730de3`, `089c63e`) was reviewed on its own (`575fbc1..089c63e`, no Critical or Important findings). The smoke then resumed: F1 re-copied and re-pinned the e2e callers to `089c63e` through a PR, F2 re-ran S3 and the drift issue closed, then S4 to S11 ran. The watch-point was that merges with no drift must be gate `auto`; they were.

## The live smoke

Repository: `jellalshadows-idp/idp-claims-e2e`. Engine `575fbc1` for S0 to S3 (the stop), `089c63e` after the fix loop. Run ids are on `https://github.com/jellalshadows-idp/idp-claims-e2e/actions/runs/<id>`.

| Step | What | Result |
|---|---|---|
| S0 | Preconditions: the right `gh` account; the smoke App, team and fake login do not exist | OK |
| S1 | PR #1 onboards a smoke Component. Run 38302192458: `validate`, `plan`, `comment`, `verdict`, `idp-gate` pass. Comment `github: +8 ~0 -0 ±0`, fingerprint matches the head SHA | OK |
| S2 | Merge `714fe1a`. Reconcile run 38302430162: `plan` ok, `approve` skipped, `apply-github` ok, `gate: auto (every change was shown in PR #1)`, the unrecovered-push guard passed, the writer key reached the called job. `wet` head `49af2fb` verified by the writer App, state encrypted | OK |
| S3 | Clean drift run 38302684806 | **Stopped**: phantom drift, issue #2 (root cause above) |
| F1 | PR #3 re-pins the callers to `089c63e` (checks run 38311142356, comment "No infrastructure changes."). Merge `481132b`, reconcile run 38311362517: `github: +0 ~0 -0 ±0`, `gate: auto`, apply skipped, `wet` touched only the rendered file | OK |
| F2 / S3 | Drift run 38311551300: `+0 ~0 -0 ±0`, `bootstrap check: no drift`, issue #2 closed | OK |
| S4 | Induced drift (description patched by hand): drift run 38311671920 `+0 ~1`, warning "drift found", issue #4 opened | OK |
| S5 | Dispatched reconcile run 38311793718: `gate: approval (changes not shown in PR #3 ...)`; approved through the API; the description was restored. Drift run 38312035111 clean, issue #4 closed. Approval 1 | OK |
| S6 | PR #5 (checks run 38312190217, `+0 ~1`), merge `116a3dc`, reconcile run 38312397391: `gate: auto (every change was shown in PR #5)`; description and topics updated | OK |
| S7 | Partial apply. PR #6 (checks run 38312607945, `+1 ~1`) with a bad member, merge `38ef6a8`, reconcile run 38312868076: `apply-github` failed in "apply the saved plan", the warning "only the state is committed" was emitted, the commit to `wet` succeeded (state only, head `0b8974b`), no wet-push issue; artifacts `idp-reconcile` and `idp-wet-root` kept | OK |
| S8 | PR #7 fixes the member (checks run 38313096442), merge `65fb9cc`, reconcile run 38313344732: `gate: auto (no changes to apply)`; `wet` head `39f5639` touched only the rendered file | OK |
| S9 | `recover` dispatched with run 38312868076 (run 38313518484): refused at "check that the open wet-push issue names this run" because that run opened no wet-push issue. `wet` unchanged. This is a changed expectation after the final review: the guard now requires the issue | OK |
| S10 | PR #8 deletes the smoke Component (checks run 38313576019, `+0 ~0 -8` with a WARNING), merge `4671ae5`, reconcile run 38313818900: `gate: approval (destructive changes ...)`; approved through the API; the repo and team are gone (404); the rendered module key is gone from `wet`. Approval 2 | OK |
| S11 | Final drift run 38314046025: `+0 ~0 -0 ±0`, `issue: none open`, 0 open drift issues. `main` holds only `.github`, `config` and the README; `wet` head `e6cfc24` | OK |

Approvals used: runs 38311793718 (S5) and 38313818900 (S10). Pull requests in the e2e repo: #1 onboard, #3 re-pin, #5 update, #6 bad member, #7 fix member, #8 delete; drift issues #2 and #4 were both closed. After the merge, PR #9 re-pinned to `a5445a2` (see "Summary").

What S2 and S9 proved about the final review's Critical: with the old callers the writer key would have been an empty string in the called job; with the fix, the key reached the job and the apply minted its token.

## Notable findings caught by review

| Finding | Where caught | Fix |
|---|---|---|
| Claim text reached `main.tf.json` verbatim, and OpenTofu evaluates `${...}`/`%{...}`: a claim could leak a token or `TF_ENCRYPTION`, including the writer token in the reconcile apply | Task 4 review (out of scope), verified by the controller | Task 4b, `ffedcf1` |
| Contract-harness blind spots: `invokes` matching, writer-key scan limited to steps, `checkTofu` missing continuations and comments, no negative tests | Task 2 review | `1249a30` |
| After a failed wet push the next queued or dispatched reconcile planned against the stale `wet` | Task 5 review | `97116d0` (unrecovered-push guard, ADR-0024) |
| **Critical:** callers did not pass the writer key, so the called workflow saw an empty string. GitHub requires the caller to pass an environment secret by name; the planning assumption was reversed | Final review | `7187b79`, ADR-0023, `writerKeyErrors` allows the named mapping only |
| `recover` could bind to the wrong run and refused cancelled runs | Final review I1 | `7187b79`: accepts failure or cancelled and must match the bot-authored open issue naming the run |
| A normal cancel could cut `tofu apply` mid-way, leaving state uncommitted | Final review I2 | `063a67c`: the apply step finishes once started; documented, force-cancel warned |
| Runbook incident paths were dead ends | Final review I3 | `917e795` and `575fbc1` |
| The unrecovered-push guard matched on the label only | Final review M2 | `7187b79` and `575fbc1`: also requires the bot author |
| Plans via the read-only token show phantom updates for admin-only fields (S3) | Live smoke | ADR-0025, A7, `43515fc` |
| ADR-0025 said spec §7.1 was unchanged and overstated that remediation always asks for approval | Smoke fix review (parked) | This commit |

Findings checked and found not to be problems: the live-runner behaviour of `job.workflow_*` and the action inputs, the `idp-wet` concurrency for drift and recover, and `concurrency.queue` (all covered by template tests and the smoke); the Task 2 `LSP undefined` diagnostics (stale; `go vet` was clean).

## Final disposition of every finding

Statuses: **Fixed** (done, with the commits), **Phase 1d**, **Won't fix** (consciously rejected, with the reason), **Obsolete** (resolved by later work). No finding is left open without a destination.

### Fixed

- Task 1 minor (ADR-0023 links `docs/pipelines.md` before Task 8 creates it): **Fixed** by Task 8 (`6c56481`).
- Task 2 four Important findings (harness blind spots): **Fixed**, `1249a30`.
- Task 2 minor (the helper `mentions` became unused): **Fixed** in the final fix wave (M5, dead helper).
- Task 4 out-of-scope injection finding: **Fixed**, Task 4b `ffedcf1`.
- Task 4b minor (the `claims.md` bullet sat in the `writes:` list): **Fixed** with the Task 8 additions (`6c56481`).
- Task 5 Important (stale `wet` after a failed push) and minors 7 and 2a: **Fixed**, `97116d0`.
- Task 5 minors on cancel versus a started apply, late approval after artifact expiry, rejected-approve re-run: **Fixed** (documented) in `6c56481`, cancel text corrected in `917e795`.
- Task 6 minors on the recover dead end, cancelled-run refusal and picking the run id the issue names: **Fixed**, documented in `6c56481` and made behaviour in `7187b79`.
- Task 8 minors: the runbook condition for "A wet push failed" (also when minting the wet-commit token fails) and the recover run-check wording in `docs/pipelines.md`: **Fixed**, `917e795` and `575fbc1`; the cancel overstatement: **Fixed**, `917e795`.
- Final review C1, I1, I2, I3 and M1 to M5: **Fixed**, `7187b79`, `063a67c`, `917e795`, `488f041`, `575fbc1`; the scoped re-review addressed all nine and found nothing new.
- Smoke S3 phantom drift: **Fixed**, `43515fc`, `aa0f2b4`, `5730de3`, `089c63e` (ADR-0025, A7).
- ADR-0025 wording ("spec §7.1 is unchanged", "remediation asks for approval"), the ADR-0025 alternatives entry for a narrower plan token, the runbook lost-runner bullet, ADR-0024 (a lost runner is outside the recovery; a run can end cancelled): **Fixed**, this commit.
- Phase 1b carry-over, the misplaced `SetOutput` doc comment: **Fixed**, `4c817f6`. The other Phase 1b carry-overs for the workflows (the wet hygiene rules, the bounded issue body, the `GITHUB_TOKEN` for `idp comment` and `idp issue`) were part of the workflow tasks and their reviews.

### Phase 1d

- The E2E harness; the release machinery (release-please, GoReleaser, attestations, Renovate) and `v0.1.0`; onboarding the production `idp-claims` (pin a release, remove the `# unreleased` pins).
- The GraphQL editor check for the gate (Phase 1b I1 option B, ruling P-4).
- Phase 1b M1 (forget column), M4 (wet marker for `--wet`), M7 (`wetpush` `PathEscape`) (ruling P-5), and the other Phase 1b test and polish minors listed in the [Phase 1b log](phase-1b.md).
- `FindOpenIssue` must require the bot author (parked at the fix wave).
- A test forbidding `-refresh` in the reconcile and drift plans, and a pin for the PR plan's `-refresh=false` against drift in the other direction (the PR side is already pinned).
- An optional narrower plan token (ADR-0025, alternatives).
- Phase 4 `adopt` needs a read-only token again, because the reconcile no longer holds the reader.
- Contract tests: `usesRefs` matches any `uses` key; `# unreleased` is accepted for any `/.github/workflows/` ref; the `docker://` message; `commandFields` skips only a first-field `echo` or `printf`; `checkTofu` inspects the first `tofu` per line and reads `-chdir stack` (separate arg) as the subcommand; `TimeoutMinutes` and `Environment` fail loudly on expression forms; `wantRun` substring matching; guards and `wet-base` tests pin only the `if` line, not `exit 1`; tests do not pin the push token after base, `close` after push without `always()`, or the drift `*) exit 1` branches; YAML aliases escape the writer-key scalar scan; `writerKeyCaller` accepts any owner's `idp-engine` (templates pin the owner); the template pin freshness is checked only by the re-pin discipline; `recover with:` is compared to a hard-coded map.
- `pr` workflow: the reader, encryption and plan step conditions are not pinned by tests (they fail closed); the state-copy path is not pinned against `render.stateFile`; the sticky comment goes stale after a failed plan; the validate render goes to `$RUNNER_TEMP/idp/rendered`, outside the scratch set; the PR error says "reconciles" and uses one message for a non-array `affected`.
- Reconcile and drift: the issue body can reference a missing artifact when `keep` also failed; the unrecovered guard reads the first 100 open issues only (a check-then-act, bounded by `queue: max` and the apply-time wet-base check); recover does not confirm the run is the one the open issue names beyond the issue match; the drift issue reuses the PR comment wording and markers and truncates the bootstrap text without a note; `check` exit 1 hides plan drift already found; the recover error points to an engine-relative runbook path; a missing `wet-base` fails with a bare bash error.
- Render: the escape rules duplicate `encryption.go`'s `hclEscaper` (one package-level escaper would do); topics, name and member escaping are untested (they are pattern-checked anyway); the escape table lacks trailing `$` or `%` and `$$x` cases; the `claims.md` note says "every value" although env names and the owner reference are protected by patterns.
- `docs/pipelines.md`: the permissions table lacks the caller `idp-gate` row.

### Won't fix

- `TF_ENCRYPTION` lingers in `GITHUB_ENV` for later first-party steps: inherent to how `GITHUB_ENV` works; it is masked and the steps are the engine's own.
- Tokens are present during `tofu init` in the reconcile `plan`: plan-mandated by a test; the modules run are the same pinned ones the apply runs.

### Obsolete

- The planning assumption that callers never pass the writer key: reversed (final review C1).
- Task 6 minor ("`recover` could accept cancelled too"): obsolete since final review I1.

## Carried into Phase 1d

- The E2E harness: the scripted equivalent of this smoke, kept as a repeatable check.
- release-please, GoReleaser, attestations and Renovate, then `v0.1.0`; the templates re-pin to the release and drop `# unreleased`.
- Production `idp-claims` onboarding, pinned to a release.
- The GraphQL editor check for the gate (P-4), and M1, M4 and M7 from Phase 1b.
- `FindOpenIssue` must require the bot author.
- A test forbidding `-refresh` in the reconcile and drift plans.
- Phase 4 `adopt` needs a read-only token again, because the reconcile no longer holds the reader.
- An optional narrower plan token: `administration: write` with the rest read. Pin it only after a live run proves the narrower set suffices.
- The remaining deferred minors listed under "Phase 1d" above.

## Owner notes

- Require a review on `.github/` in claims repos, and pin `idp-gate` to the GitHub Actions integration (ADR-0022). Both are still the owner's decision. Without them, a writer can change the caller workflow in a PR, and the `idp-gate` check is not tied to the Actions integration. The gate fails safe: without a trusted plan comment it asks for approval.
- The plan tokens keep the writer manifest's full permissions, by the owner's choice. Narrowing is optional; see ADR-0025.

## Process notes

- **Owner rule:** the docs describe the platform on its own terms. The naming check ran on every task and on the whole branch (including hidden directories), and was clean each time. Three shell-rule slips by subagents were recorded (one `ls`, one `sed`, both read-only, with no effect).
- **Execution:** subagent-driven. Per-task reviews, one final whole-branch review and one fix wave, then a smoke-driven fix with its own scoped review. Fix rounds used: Task 2 (1), Task 5 (1), final wave (1), smoke fix (1 plus a docs round).
- **Interruptions:** the owner paused the run once (2026-10-11, no new agents) and it resumed on 2026-10-12. A power loss in the middle of the final fix-wave re-review was recovered from the ledger and the saved transcript: the repo was intact at `575fbc1`, the remote at `6c56481`, and the same reviewer resumed.
- **Plan handling:** the planning assumption about the writer key was wrong and was reversed by ruling, not by rewriting the plan; the plan stays as approved.
- **Local verification constraint:** no local container tooling, so `tofu validate` of the escaped fixture and shellcheck run in CI (`render-smoke`, `workflows`).

## Status

**Phase 1c is complete.** The four reusable workflows, `setup-idp`, the caller templates and the contract tests shipped and are verified by green CI runs on PR #7, by the live smoke S0 to S11 in `idp-claims-e2e`, and by the post-merge re-pin (PR #9, reconcile run 38314547633). Every finding has a final disposition above.

Next: Phase 1d (the E2E harness, the release machinery and `v0.1.0`, production onboarding).
