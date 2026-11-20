# Phase 2e execution log: the pipelines' AWS plan and apply

- Period: 2026-11-18 to 2026-11-20
- Plan: [2026-11-18-phase-2e-aws-pipelines.md](../superpowers/plans/2026-11-18-phase-2e-aws-pipelines.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md), amended by A14 (A15, the roadmap amendment, was approved after the plan and landed first)
- Decisions: [ADR-0031](../adr/0031-floci-jobs-hold-no-secret.md) added (jobs that run floci hold no secret)
- Previous phase: [Phase 2d log](phase-2d.md)
- Status: **complete**. The phase merged through four PRs: [#39](https://github.com/jellalshadows-idp/idp-engine/pull/39) (`434633d`, Tasks 1 to 4), [#40](https://github.com/jellalshadows-idp/idp-engine/pull/40) (`fb24c40`, Tasks 5 to 9) and [#41](https://github.com/jellalshadows-idp/idp-engine/pull/41) (`531d63b`, Tasks 10 to 14's code) and the fix [#42](https://github.com/jellalshadows-idp/idp-engine/pull/42) (`a577c34`, the approval after a skipped `plan-aws`, which the harness caught), all merge commits. It ships no release: the release PR [#28](https://github.com/jellalshadows-idp/idp-engine/pull/28) "chore(main): release 0.2.0" stays open until Phase 2f.

This file is the durable copy of the execution ledger that was kept while the plan ran. The ledger lived in a git-ignored scratch folder that is deleted afterwards, so what it recorded is reproduced here: the rulings, each task's outcome, the reviews, the live results, the disposition of every finding and what is carried forward. Anything the ledger does not state is written as unknown or pending.

## Summary

Phase 2e makes the reusable workflows plan and apply the rendered AWS stacks against floci (spec §6.1 step 4, §6.2 steps 1 to 4). It is the fifth of the six plans of Phase 2 (2a to 2f).

- **`internal/awsstack`** holds the per-stack sequence: stage the environment's baseline and the stack's `wet` version in a scratch copy laid out like a render, apply them, swap in the new version against the same state and plan it (or plan an orphan's destroy with its `wet` configuration); `Apply` re-plans and requires equality with the gated fingerprint before it applies the saved plan.
- **New `idp` commands:** `stacks` (the allowlist guard, the AWS matrix and each stack's account), `plan-reduce` (a plan's addresses and actions only), `aws-stack plan|apply` (one stack against floci; it refuses any secret) and `wet-root` (the wet commit of a reconcile). `plan-summary` reads reduced plans with `--plan-dir`; `wet-push` refuses any state file but `tfstate/github.tfstate`.
- **`pr.yaml`** plans each affected AWS stack in a `plan-aws` job with a floci of its own; a secret-less `comment` job summarizes every stack's reduced plan.
- **`reconcile.yaml`** gains `plan-aws`, a secret-less `gate`, `apply-aws` (re-plan, require the gated fingerprint, apply, mark applied) and an `apply-github` that commits the `github` state and the renders of what applied. `recover.yaml` pushes the path list the reconcile wrote.
- **No job that runs floci holds a secret** (ADR-0031), and the AWS state never leaves its job (ADR-0030). Every artifact a later job trusts is downloaded by artifact ID (the by-ID trust rule, below).
- **`TestReplay`** rehearses `plan-aws` and `apply-aws` against floci in CI's `floci` job: a new stack, a changed one, an unchanged one, an orphan and the refusal of a plan that differs from the gated one.
- Drift stays GitHub-only. No Workspaces, no features, no release.

How it was executed: inline by the parent session, with bounded sonnet writers for some units (PR 2's fix wave: two parallel writers; PR 3: Tasks 10 and 11 and the fix wave), an Opus whole-PR review of each PR, sonnet scoped re-reviews, and a Gentle AI receipt-driven review of every candidate with the owner's consent. Every task followed strict TDD with an observed RED (see "Process notes"). The work shipped as three PRs, each cut from `main` after the previous one merged (D-12), and a fourth, #42, fixed what the harness caught after PR 3's merge.

## Tasks

Commits are on `feat/phase-2e-aws-pipelines`, re-created from `main` for each PR. The plan commit is `1d739c0` and the roadmap amendment A15 is `7ff0dcd`, both on `main` before PR 1.

### PR 1 (#39, merged as `434633d`): Tasks 1 to 4

| Task | What | Commit | Outcome |
|---|---|---|---|
| 1 | Spec amendment A14 (seven edits), ADR-0031 | `880beb8` | Complete: anchors resolve, naming clean |
| 2 | `wet-push` refuses any state but the `github` stack's | `614c33e` | Complete. RED: 4 lines. GREEN. Two mutations caught |
| 3 | `idp stacks`: the allowlist and the AWS matrix | `750996f` | Complete. RED: unknown command. GREEN. Two mutations caught |
| 4 | Reduced plans and `plan-summary --plan-dir` | `b10da82` | Complete. RED as planned. GREEN. Two mutations caught. PR 1 local: 16 files, +800/-23 |

Fix wave after the Opus review (see "Reviews"):

| Commit | What |
|---|---|
| `dfb17c9` | `wet-push` matches state files in any case; `idp stacks` refuses empty or duplicate names (exit 2); a wet Component ARN naming no account exits 1 (the placeholder account only when the environment has no Component stack); the 256-stack boundary is pinned; `plan-summary --plan-dir` accepts only reconcilable names |
| `ec2b685` | ADR-0031 and A14: artifacts a later job trusts are downloaded by ID; the wet account rules |
| `63b62df` | Each reduced plan is read from its own artifact directory; one account per wet environment (disagreement exits 1) |
| `b324755` | ADR-0031: each plan artifact keeps its own directory |

Five mutations of the fix wave were caught (on committed code). CI on `b324755`: all nine checks SUCCESS.

### PR 2 (#40, merged as `fb24c40`): Tasks 5 to 9

| Task | What | Commit | Outcome |
|---|---|---|---|
| 5 | `internal/awsstack`: replay, plan, apply | `97a500a` | Complete. RED: undefined `Config`, `Plan`, `Apply` (observed by moving the implementation aside). GREEN: 20 tests. 4 of 4 mutations caught |
| 6 | `idp aws-stack plan\|apply` | `1febc6c` | Complete. RED: undefined `newTofu`. GREEN. 2 of 2 mutations caught |
| 7 | `TestReplay`, the floci rehearsal | `c7b9378` | Complete. Offline it skips as stated; its RED and GREEN are the floci job of PR 2 |
| 8 | `idp wet-root` | `c573077` | Complete. RED: unknown command (6 tests). GREEN. 3 of 3 mutations caught |
| 9 | `docs/cli.md` | `0b09039` | Complete: the new blocks, aligned with PR 1's fix wave |

Carried from PR 1's Gentle AI advisories, done in this PR:

| Commit | What |
|---|---|
| `fc11355` | `wet-root` leaves state files out of the renders and names them on stderr, so the `github` state still commits; `wetpush.IsState` is exported and reused. Mutation caught |
| `0d5e456` | `TestReconcilable` pinned to the claims schemas (1, 40 and 41 characters for a component; 10 and 11 for an environment). Mutation `{0,39}` caught |
| `8f2de8f` | `downloadedPlanIn` reuses `planFileIn` |

Fix wave after the Opus review:

| Commit | What |
|---|---|
| `c4f8729` | Writer A: `wet-root` keeps an orphan baseline while its Components stay in `wet`, leaves foreign markers out; `--wet` required. 5 mutations caught |
| `b40d1ca` | Writer B: `awsstack` failure hooks, `ErrApplyFailed` and the partial-apply wording, absolute `Work`, `copyTree` copies regular files only. 6 mutations caught |
| `0b51f26` | Parent: the `github` render is never deleted from `wet`; `wetpush.GitHubState` is shared. Mutation caught |
| `ae8a29d` | `docs/cli.md` for the wet-root keeps and leave-outs and a partial AWS apply |
| `ea586a8` | Follow-up from the re-review and the Gentle AI advisories: an applied orphan baseline that `--wet` does not hold is kept and named; non-directories among wet's components are skipped; tests for a baseline replay failure and the `!inNew` and `hasRender` branches of `wet-root`. 5 mutations caught |

CI: all nine checks SUCCESS on `0b09039`, `ae8a29d` and `ea586a8`. PR 2 was one PR of 8 commits at open (17 files, +1824/-54), with no cut.

### PR 3 (#41, merged as `531d63b`): Tasks 10 to 14

| Task | What | Commit | Outcome |
|---|---|---|---|
| 12 | `recover` pushes the paths the reconcile listed | `8ce25f8` | Complete. RED: 4 "run lacks" lines as planned. GREEN. actionlint v1.7.12 ok. 2 of 2 mutations caught (the wet-paths branch, the empty-paths guard) |
| 10 | `pr.yaml`: `plan-aws` and the summarizing `comment` job | `61a4a6c` | Complete (writer, sonnet). RED as planned plus 2 new tests. GREEN. actionlint ok |
| 11 | `reconcile.yaml`: `plan-aws`, `gate`, `apply-aws`, `apply-github` | `de6cb39` | Complete (writer, sonnet). RED: 16 tests, 3 of them new. GREEN. actionlint ok. 3 of 3 mutations caught (the gated-ID guard, the widened AWS pattern, `--wet` dropped) |
| 13 | `docs/pipelines.md`, the runbook | `8205a0b` | Complete: the trust-rule bullet lists the by-ID artifacts; the permissions table matches the YAML |
| 14 | Ship: verification, PR #41, CI, reviews, fix waves, merge, this log and the README | PR #41, merge `531d63b`; this commit | See "Ship" |

Task 14 Step 1 (local verification): `go test`, `go vet` and `gofmt` green; naming clean; `main..HEAD` was exactly the nine files of Tasks 10 to 13 (+1106/-149), with no `ci`, `drift`, action, example, module, lock file or render change.

Fix wave after the final whole-phase review:

| Commit | What |
|---|---|
| `527beda` | Parent: `readMarker` bounded (`LimitReader`, `maxMarkerSize` 128; mutation caught); plan-dir wording in `docs/cli.md`, the flag help and comments |
| `a6de728` | Writer (sonnet): the plan layout no longer depends on the artifact count; the issue body names the `idp-wet-root` artifact ID and `recover` downloads by ID (refusing without one); job-level `!cancelled()`; a 5-minute timeout on the applied download; floci jobs have no outputs, container or actions cache; the wet-paths loops keep a last line. RED: 21 tests. GREEN. actionlint ok on all three workflows. 8 of 8 mutations caught |
| `0bea58d` | The keep upload sets `overwrite: true`; pinned in `TestWetRootIsSavedBeforeThePush` (RED observed, mutation caught) |
| `0bf9bb2` | Docs only: ADR-0031 records the shared AWS plan directory; the runbook gains the step "recover before bumping the engine" and its rationale |

CI: all nine checks SUCCESS on `8205a0b`, `a6de728`, `0bea58d` and `0bf9bb2`.

### The fix after PR 3 (#42, merged as `a577c34`)

The harness run after PR 3's merge (run 44723100136) failed at its update step: reconcile run 44724810412 of `idp-claims-e2e` gated `approval` for the topics delete, yet `approve` and `apply-github` were skipped, so nothing was approved or applied, and the delete step left `e2e-r44723100136-app` and its team behind. Cause: Task 11 made `approve` need the `gate`, which needs `plan-aws` and runs past it with `!cancelled()`. With no AWS stack affected, `plan-aws` is skipped, and `approve` kept the implicit `success()`, which is false when any job before it was skipped, not only one it needs. The plan's text specified that condition, and the workflowspec test pinned it.

| Commit | Content |
|---|---|
| `404a306` | `approve` runs on `!cancelled() && needs.gate.result == 'success' && needs.gate.outputs.decision == 'approval'`. A rule over every workflow (`TestAJobAfterOneThatRunsPastASkipChecksItsOwnStatus`): a job that needs one whose `if` starts with `!cancelled()` or `always()` starts its own `if` with one too; it flagged only `approve`. RED observed on both tests |

CI: all nine checks SUCCESS on `404a306`. The owner approved the cleanup: `idp-claims-e2e` PR #269 pinned `a577c34`; its reconcile (run 44731267609) waited in `approve`, the first live proof of the fix, and after the approval destroyed the ten leftover addresses, all `e2e-r44723100136-*`.

## Rulings

A ruling is a controller or owner decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong. The owner approved the plan and D-1 to D-18 (three PRs) on 2026-11-18 and 2026-11-19 ("dale"). Phase 2a's owner decisions still hold: Phase 2 is split (2a D-1) and v1 is floci only (2a D-2). The plan holds each decision's full text; this is the summary.

- **D-1. Scope:** the PR plans every affected AWS stack; the reconcile plans, gates and applies them. An orphan is destroyed with its old configuration from `wet`. `apply-github` commits an AWS stack's render only after it applied, and deletes an orphan's render only after its destroy applied. *Cost if wrong:* committing the renders of failed stacks would hide failures from the next reconcile; reverting is one condition in `idp wet-root` and its test.
- **D-2. No secret in a job that runs floci** (ADR-0031): floci mounts the runner's Docker socket, so `plan-aws` and `apply-aws` hold no `secrets.*`, no App token, no environment and no `TF_ENCRYPTION`; `GITHUB_TOKEN` is `contents: read` and never in a step's environment; `idp aws-stack` refuses `TF_ENCRYPTION` and `IDP_STATE_PASSPHRASE`. The plan job's summary and gate move to secret-less jobs. `verdict` stays the only required check (ADR-0022). *Cost if wrong:* a job that needs a secret beside floci would have to split again.
- **D-3. The per-stack sequence:** a Component stack's job applies its environment's baseline first (`wet`'s version, or the new render's for a new environment), then the stack's `wet` version, then plans the new version against the state that replay wrote, in the same scratch directory with the configuration swapped. An orphan gets a destroy plan with its `wet` configuration. The apply re-plans and requires equality with the gated fingerprint. Planning findings, verified offline with OpenTofu 1.12.6: a swapped configuration plans `update` against the replayed state (not `create`); `init` after the swap accepts a changed module source; a `-destroy` plan of the old configuration deletes every resource. *Cost if wrong:* replaying the new baseline instead of `wet`'s changes one loop's order and one test line.
- **D-4. Testable:** the sequence is Go code (`internal/awsstack`) behind `idp aws-stack`, with a fake runner in unit tests and `TestReplay` in CI's floci job. *Cost if wrong:* none in production; a floci gap shows in 2f's live run.
- **D-5. The plan pipeline:** the AWS stacks join `plan-summary`, the comment, the marker and the gate through reduced plans and `--plan-dir`; the `jq` guard becomes the `idp stacks` allowlist (`github`, `aws/<env>/_baseline`, `aws/<env>/components/<name>`). *Cost if wrong:* one regular expression in `internal/wetdiff`.
- **D-6. Artifacts:** one escaping, `wetdiff.Escape` (each `/` becomes `.`); names `idp-plan-<stack>`, `idp-renders`, `idp-gated`, `idp-applied-<stack>`, plus `idp-reconcile` and `idp-wet-root`; one-day retention (seven for `idp-wet-root`). Only reduced plans upload, the `github` stack's included, because `tofu show -json` holds every value a plan sets in plaintext and artifacts of public repos are public. *Cost if wrong:* `reduced` gains fields; nothing stored depends on its shape.
- **D-7. The tfstate guard:** `wet-push` refuses any `*.tfstate` or copy except `tfstate/github.tfstate`, before any write. `idp diff` does not refuse. *Cost if wrong:* a state file in a render would show as a changed stack until removed; the push still refuses it.
- **D-8. Drift stays GitHub-only:** an AWS stack's state lives only in its job (ADR-0030). `recover` pushes the paths in `wet-paths`. *Cost if wrong:* none until a real account.
- **D-9. Time, matrix and limits:** one job per stack, `fail-fast: false`, `max-parallel: 10`, at most 256 AWS stacks, and no AWS job when none is affected. Baselines measured on 2026-11-18: the floci job on `19e65bd` took 2m34s; the harness's `pr / validate` 32 s, `pr / plan` 45 s, `pr / comment` 34 s, `reconcile / plan` 40 s, `reconcile / apply-github` 66 s; `setup-idp` 20 to 27 s per job; spec §11 risk 3 about 26 s of OpenTofu per stack. Forecast: a `plan-aws` leg about 1.5 min, an `apply-aws` leg about 1.7 min. *Cost if wrong:* per-environment legs would cut setup time; the change is `idp stacks`'s output and two job blocks.
- **D-10. Live proof:** `idp-claims` pins `v0.1.1` (four callers), holds three environments with `aws` blocks and no claims, and its `wet` holds only `rendered/github`; `idp-claims-e2e` has no `aws` block. No live AWS smoke run was proposed; the live run is 2f's. *Cost if wrong:* a workflow-level fault in the AWS jobs shows first in 2f.
- **D-11. Docs:** A14 and ADR-0031 in Task 1; `docs/cli.md` in Task 9; `docs/pipelines.md` and the runbook in Task 13; the README at ship.
- **D-12. Delivery:** three PRs, each green and shippable alone. Forecast about 820, 1,770 and 900 lines. *Cost if wrong:* a single PR is the same commits.
- **D-13. Renovate:** the weekly run of Monday 2026-11-23 is on the watch list. The floci image is pinned in three files and `TestFlociJobsHoldNoSecret` requires them equal.
- **D-14. The account of each AWS job:** from the claims' platform config; for a removed environment, from the ARN in `wet`'s render of one of its Component stacks; `000000000000` for an orphan baseline with no Component stack. *Cost if wrong:* a wrong account plans a create instead of the change.
- **D-15. `apply-github` runs after every `apply-aws` job** and collects their markers, whatever they concluded. *Cost if wrong:* splitting the wet commit into its own job would need the state and passphrase there too.
- **D-16. The wet commit's path list:** `idp wet-root` writes `wet-paths`; the push and `recover` read it. A wet root with nothing to commit pushes nothing and opens no issue.
- **D-17. The `github` plan's artifacts:** `reconcile / plan` uploads `idp-reconcile` whenever any stack is affected; only its reduced plan uploads.
- **D-18. Local tools:** OpenTofu 1.12.6 locally (run from a `mktemp -d` copy), CI's 1.13.2 authoritative; no local Docker, so `TestReplay`, actionlint v1.7.12 (run through `go run`), zizmor and shellcheck results come from CI or the local `go run` of actionlint.

Execution rulings:

- **Execution inline, with writers for named reasons.** The parent ran the work, delegating to bounded sonnet writers: PR 2's fix wave as two parallel writers with disjoint surfaces (independent units); PR 3's Task 10, Task 11 and fix wave one at a time (context backstop; Task 11 consumes Task 10's helpers, so not parallel). Tasks 12 and 13 ran inline. *Cost if wrong:* writer diffs needed a surface check, which the parent did each time.
- **A15 numbered by approval order, not merge order:** the roadmap amendment landed on `main` before A14. *Cost if wrong:* none.
- **The by-ID artifact trust rule (PR 1 review, Important):** every artifact a later job trusts comes from a job without floci and is downloaded by the artifact IDs its producer outputs; only floci-job artifacts (reduced AWS plans, applied markers) are downloaded by pattern, and a forged AWS plan is bounded by the apply-time equality. The apply-github fingerprint re-check was not adopted: the by-ID chain binds plan, gate and apply. *Cost if wrong:* one more check.
- **No plan download merges artifacts across stacks (PR 1 re-review):** `plan-summary --plan-dir` reads `<dir>/idp-plan-<s>/<s>.json`; the writers keep `<out>/<s>.json`. *Cost if wrong:* a layout change.
- **Plan layout after the layout finding (PR 3 final review):** the `github` plan by ID is merged into `plans/idp-plan-github`; the AWS plans by pattern are merged into a separate `aws-plans` directory, and a trusted step copies each regular, non-symlink `aws-plans/A.json` to `plans/idp-plan-A/A.json` for each A in the plan job's `aws` output. *Cost if wrong:* one step.
- **Recover by artifact ID:** the issue body carries the `idp-wet-root` artifact ID and `recover` downloads by ID, refusing without one (an issue from before 2e is recovered by hand). *Cost if wrong:* a stranded legacy issue, handled by runbook step 6.
- **Keep with `overwrite: true`:** a floci job could upload `idp-wet-root` first and make the keep fail. *Cost if wrong:* none.
- **Job-level `always()` became `!cancelled()`,** so a cancelled run starts no job. The applied download has a 5-minute timeout and `wet-root` reads at most 129 bytes per marker.
- **The harness's failure fixed as a fourth PR of 2e (#42):** the harness is the phase's ship gate, so its fix and the cleanup of what the failed run left (approved by the owner) belong to the phase. *Cost if wrong:* one more PR in this log.
- **Won't fix and accepted rulings** are listed under "Final disposition of every finding".

## Review Focus

| # | Case | Pinned by | Result |
|---|---|---|---|
| 1 | A job that runs floci reaching a secret | `TestFlociJobsHoldNoSecret`; `TestAWSStackRefusesTheStatePassphrase` | Tested; the floci jobs were hardened further in `a6de728` (no outputs, container or actions cache). The ledger does not record the mutation count for the secret rule itself |
| 2 | An AWS apply of a plan nobody gated | `TestApplyRefusesAPlanThatDiffersFromTheGatedOne`, `TestApplyRefusesAStackTheGateNeverSaw`, `TestApplyAppliesTheGatedPlan`, `TestAWSStackApplyRefusal`, `TestApplyAWSAppliesWhatTheGateDecided`, `TestReplay`'s refusal subtest | Tested; Task 5 (4 mutations) and Task 11 (3) caught. `TestReplay` passed 5 of 5 in CI. Residual accepted: a floci artifact can forge an AWS plan, bounded by this equality |
| 3 | A replay that plans against the wrong state | `TestPlanAChangedComponentReplaysWet`, `TestPlanABaseline`, `TestPlanAnOrphanDestroysWithItsWetConfig`, `TestPlanANewComponent`, `TestPlanKeepsTheRenderUntouched`, `TestReplay` | Tested; Task 5's 4 mutations caught; `TestReplay` passed against floci |
| 4 | `wet` getting AWS state, a render for a stack that did not apply, or an orphan's render removed early | `TestSyncCommitsNoStateButGitHubs`, the `TestWetRoot*` tests, `TestApplyCommitsTheRenderOnlyAfterASuccessfulApply`, `TestWetRootIsSavedBeforeThePush`, `TestRecoverChecksTheRunAndTheWetBase` | Tested; mutations caught in Tasks 2, 8 and 12 and in the fix waves. Review found three gaps (orphan baseline, foreign marker, keep overwrite), all fixed |
| 5 | A stack this version cannot reconcile slipping through, or a plan missing from the comment and the gate | `TestReconcilable`, `TestEscape`, `TestStacksRefusesWhatThisVersionDoesNotReconcile`, `TestStacksRefusesMoreThanAMatrixRuns`, `TestPlanSummaryReadsAPlanDir`, `TestPlanSummaryTakesPlanOrPlanDir`, `TestPRPlanSummaryCoversTheAffectedStacksAtTheHeadCommit`, `TestReconcileGatesWithTheWorkflowToken` | Tested; mutations caught in Tasks 3 and 4. Review found the plan-dir layout defect (below), fixed |

## Reviews

| PR | Whole-PR review (Opus) | Gentle AI review (owner granted) |
|---|---|---|
| #39 | Ready after fixes: 1 Important, 5 Minor; a sonnet scoped re-review resolved all six and found one new Minor (ruled in `63b62df`) | Lineage `review-533e73aabf323948` (risk, readability, reliability): approved, acknowledged; risk lens 0 findings |
| #40 | Ready after fixes: 3 Important, 3 Minor; a sonnet scoped re-review resolved all and found two test gaps (added in `ea586a8`) | `0b09039` candidate (4 lenses): approved, acknowledged (lineage ID not recorded in the ledger). Fix wave: `review-fd62d34471747e4f` (resilience, reliability, readability). Follow-up `ea586a8`: `review-865ab14c60e3b562` (3 lenses). All approved and acknowledged |
| #41 | Ready after fixes: 1 Critical, 1 Important, Minors; a sonnet scoped re-review resolved findings 1 to 5 and found one new Minor (`0bea58d`) | `8205a0b` (4 lenses): `review-88e0bc52eb8613c7`. Fix wave `8205a0b..0bea58d` (4 lenses): `review-66107ee1435b9bc0`. Both approved and acknowledged. `0bf9bb2` was docs only: assessed passive, no review due |
| #42 | None: a one-condition fix, pinned by the new rule and proven by the harness | `404a306` (reliability): `review-4074468011e5787f`, approved and acknowledged; two suggestions, won't fix |

## Ship (Task 14)

1. PR #41 was opened with the by-ID trust rule in its body; the CI of every pushed head (`8205a0b`, `a6de728`, `0bea58d`, `0bf9bb2`) passed all nine checks.
2. The final whole-phase review (Opus) and its fix waves are in "Tasks" and "Reviews".
3. PR #41 merged as `531d63b` (merge commit, as #39 and #40).
4. The release run after the merge left release PR #28 titled `chore(main): release 0.2.0`; it stays open.
5. The harness after the merge failed at its update step (see "The fix after PR 3"). After #42 and the owner-approved cleanup, run 44731848233 on `a577c34` passed.
6. This log and the README update are one `docs` commit on `main`.

## Watch list

| Item | Result |
|---|---|
| Each PR's check list: `go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke`, `floci`, `release-snapshot` | PR #39: all 9 SUCCESS on `b10da82` and `b324755`. PR #40: all 9 SUCCESS on `0b09039`, `ae8a29d`, `ea586a8`. PR #41: all 9 SUCCESS on `8205a0b`, `a6de728`, `0bea58d`, `0bf9bb2` (CLEAN at merge) |
| `TestReplay`'s first run and the floci job's duration | PR #40's CI on `0b09039`: 5 of 5 subtests PASS, about 170 s for `TestReplay`. The floci job took 5m11s (CI run 44499089595), against `19e65bd`'s 2m34s: `TestReplay` accounts for most of the difference |
| Release PR number and version after each merge | `#28 chore(main): release 0.2.0` after PR 3's merge, left open until 2f |
| Harness run after PR 1's merge | Run 44498284721: success |
| Harness run after PR 2's merge | Run 44524048004 on `fb24c40`: success |
| Job durations of the harness after PR 3 (`pr / plan`, `pr / comment`, `reconcile / plan`, `reconcile / gate`, `reconcile / apply-github`) and how GitHub names a skipped matrix job | Run 44731848233 (after #42): `pr / validate` 32 s, `pr / plan` 45 s, `pr / comment` 28 s (PR #271's run); `reconcile / plan` 60 s, `reconcile / gate` 34 s, `reconcile / approve` 5 s, `reconcile / apply-github` 42 s (run 44732997160). A skipped matrix job keeps its unexpanded name: `pr / plan-aws ${{ matrix.stack }}`, `reconcile / apply-aws ${{ matrix.stack }}` |
| An AWS leg in a real pipeline | Pending: 2f's live run |
| `download-artifact` with a `pattern` that matches nothing (every `apply-aws` leg failed) | Pending: not exercised before 2f's live run |
| The AWS provider downloaded twice per Component leg (a plugin cache in `awsstack.Env` if it dominates) | Pending: not measured |
| The single-artifact by-ID download layout in a live run | Pending: verified only in the action's source (below); confirm live in 2f |
| The weekly Renovate run of Monday 2026-11-23, 04:17 UTC (`hashicorp/aws` 6.72.0 as one "aws provider" PR labelled `needs-lockfile`; floci 2.5.0 eligible from 2026-11-29, which must move `ci.yaml`, `pr.yaml` and `reconcile.yaml` together) | Not yet run |

Harness after PR 3: run 44723100136 on `531d63b` failed (the skipped approval, fixed by #42). Run 44730969659, dispatched by #42's merge, refused in its preflight because the leftovers still existed. Run 44731848233 on `a577c34`, after the cleanup, passed in 1,400 s with both approvals, no drift and the delete.

## Notable findings caught by review

| Finding | Where caught | Disposition |
|---|---|---|
| Floci-job artifacts downloaded by pattern could shadow `github.json` in the gate (the artifact-trust rule) | PR 1 Opus review, Important | Fixed: every trusted download is by artifact ID (`ec2b685`, `63b62df`, `b324755`; applied in PR 3) |
| A pattern download could still shadow `github.json` if it shared the by-ID directory | PR 1 scoped re-review | Fixed: no plan download merges artifacts across stacks (`63b62df`) |
| `wet-push`'s refusal aborts the whole sync, the `github` state included | PR 1 Gentle AI R3 | Kept fail-closed: a partial commit would silently split `wet` from the applied state |
| `wet-root` deleted an orphan baseline while an orphan Component of the environment failed and stayed in `wet`, so every later plan failed | PR 2 Opus review, Important 1 | Fixed: kept orphan baseline (`c4f8729`, `ea586a8`) |
| One foreign marker made `wet-root` exit 1 before writing the `github` state | PR 2 Opus review, Important 2 | Fixed: foreign markers left out (`c4f8729`) |
| The fakes never failed: mutants ignoring the final apply error, the replay apply error and `checkWork`'s wet arm survived | PR 2 Opus review, Important 3 | Fixed: failure hooks (`b40d1ca`); the baseline replay failure test (`ea586a8`) |
| A failed final apply was summarized as "Nothing was applied" (the partial-apply wording) | PR 2 Opus review and Gentle AI R4/R3 | Fixed: `ErrApplyFailed` names what applied (`b40d1ca`) |
| `tofu` receives the whole `os.Environ` | PR 2 Gentle AI R1-001 | Carried: an environment allowlist in `awsstack.Exec`; ADR-0031 guarantees no secret at job level |
| `download-artifact` extracts a single artifact flat into `path`, not into `path/<name>/` | PR 3 final review, Critical | Fixed (`a6de728`). Verified in `download-artifact@9000827`'s source (`src/download-artifact.ts`): the artifact is extracted flat when `name` is set, when `merge-multiple` is set, or when exactly one artifact is found (so a pattern matching one artifact is flat too). An earlier ledger claim that a by-ID download unmerged lands in `path/<name>/` was wrong for one artifact, and would have made `plan-summary --plan-dir` fail for every `github` change |
| `recover` downloaded `idp-wet-root` by name, so a floci job could pre-upload it | PR 3 final review, Important | Fixed: by ID from the issue body (`a6de728`) |
| The keep upload had no `overwrite`, so a floci job uploading `idp-wet-root` first made the keep fail | PR 3 scoped re-review | Fixed: `overwrite: true` (`0bea58d`) |
| An artifact ID read from a mutable issue body | PR 3 Gentle AI R1 | Carried to 2f |
| The layout step skips silently, and a `jq` failure in the process substitution does not fail the step | PR 3 Gentle AI R4 | Carried to 2f (it fails closed at `plan-summary` one step later) |
| AWS plans merged into one directory lose per-stack isolation: one floci leg can forge another AWS stack's plan | PR 3 Gentle AI R1/R4/R3 | Accepted: bounded by the apply-time equality; ADR-0031 says so (`0bf9bb2`) |
| A legacy `wet-push` issue without an artifact ID is stranded | PR 3 Gentle AI R4/R3 | Runbook step 6: recover before bumping the engine (`0bf9bb2`) |
| Mutation-check incident: a mutation run restored files with `git checkout` before the fixes were committed and erased them | PR 1 fix wave | Re-applied from a script. Rule: mutate only committed code |
| False survivors from mutations that did not compile | PR 2 follow-up (`ea586a8`) | The mutation helper now detects build failures |
| `approve` kept the implicit `success()` while the gate runs past a skipped `plan-aws`, so a destructive change to the `github` stack alone was never approved or applied | The harness after PR 3's merge; no review caught it | Fixed in #42 (`404a306`), with a rule over every workflow |

No review found an open Critical issue after its fix wave.

## Final disposition of every finding

Statuses: **Fixed** (done, with the commits), **Carried** to a later sub-phase, **Won't fix** (consciously rejected, with the reason), **Accepted**. No finding is left open without a destination.

### Fixed

Phase 2d's "Carried into 2e" items:

- The AWS plan and apply against floci: **Fixed**, Tasks 5, 6, 10 and 11 (`97a500a`, `1febc6c`, `61a4a6c`, `de6cb39`).
- The scratch copies and no AWS state in `wet`: **Fixed**, Tasks 2, 5 and 8 (`614c33e`, `97a500a`, `c573077`).
- The tooling guard against a `terraform.tfstate` reaching `wet`: **Fixed**, Task 3 (`750996f`, the `idp stacks` allowlist); the `wet-push` state refusal is Task 2 (`614c33e`).
- `plan-summary`, `gate` and `comment` over the AWS stacks: **Fixed**, Tasks 4, 10 and 11 (`b10da82`, `61a4a6c`, `de6cb39`).
- The floci time in spec §11 risk 3: **Fixed**, Task 1 and D-9 (`880beb8`).

This phase's findings marked Fixed above, with their commits, and the PR 1 advisories carried to PR 2 (`fc11355`, `0d5e456`, `8f2de8f`). The skipped approval the harness caught: **Fixed**, #42 (`404a306`).

### Won't fix

- **PR 1:** reduced type name in `Parse` (R2): its doc comment states it is the subset `Parse` reads; cost if wrong: a rename.
- **PR 2:** `wet-root --out` stat error wording (the workflow passes a fresh `RUNNER_TEMP` path); new components not in `--wet` (a Component in `--new` implies its baseline in `--new`, so the baseline is no orphan); `copyTree` mode 0644 (renders hold JSON and lock files only); kept-message rewording (the wording is new in the same unreleased PR); the kept verb when `--wet` lacks the render (accurate for the real branch); a symlinked Component directory skipped (a hand-committed one fails visibly with no baseline); shared reason constants (style).
- **PR 3:** the marker-bound wording, literal limits in a test, the layout step duplicated in two workflows, the gate layout checked twice (all pinned by tests); the floci preamble duplicated three times (pinned to `ci.yaml`'s text by a test).
- Not bugs, by review: `plan-dir` empty when nothing is affected; `wet-root` with missing directories on a `github`-only run; `apply-github` on an AWS-only run (the `github` arguments are guarded by `GITHUB_STACK`).

### Accepted

- The forged AWS plan class (a floci artifact can plant another leg's `A.json`): bounded by the apply-time equality; that stack applies nothing until the next reconcile.
- Applied markers merged: accepted by ADR-0031.
- `wet-push`'s fail-closed refusal.

## Carried into later sub-phases

- **2f.**
  - Re-pin `idp-claims` to `0.2.0`: its first reconcile plans and applies three new baselines (D-10).
  - The live end-to-end AWS run and the pending watch items: a real AWS leg, a `pattern` download that matching nothing, the provider downloads per leg, and confirming the single-artifact download layout live.
  - The E2E env-variable checks and the release, as the [Phase 2b log](phase-2b.md) lists.
  - A tofu subprocess environment allowlist in `awsstack.Exec` (R1-001).
  - Check the `idp-wet-root` artifact's name and run through the API in `recover` before the download (R1, artifact ID from a mutable issue body).
  - In the plan layout step: log the files it skips and fail on a `jq` error.
  - The Renovate observations of Monday 2026-11-23.
- **Phase 3.** Workspaces (`aws/<env>/workspaces/<name>`), which `idp stacks` refuses until then; `observe` replays.
- **Real AWS (beyond v1).** A persistent, encrypted backend (ADR-0030), OIDC credentials, ordering between stacks, and AWS drift.
- **Phase 4.** As the [Phase 2c log](phase-2c.md) lists.

## Owner notes

- The release PR #28, proposing `0.2.0`, stays open until 2f. Every push to `main` re-runs the harness on it while it is open (about 20 minutes each, ADR-0028); this log is a `docs` commit, so its push does that once more.
- A `wet-push` issue created before this engine has no artifact ID: recover it by hand, and recover before bumping the engine (runbook step 6).
- The earlier owner items stay as recorded in the [Phase 1e log](phase-1e.md); they are not new asks.

## Process notes

- **Execution:** inline by the parent, with bounded sonnet writers for named units (PR 2's fix wave, two in parallel; PR 3's Tasks 10 and 11 and fix wave), Opus whole-PR reviews, sonnet scoped re-reviews, and Gentle AI receipt-driven reviews on every candidate with the owner's consent (lineages in "Reviews").
- **TDD:** strict; RED observed for each behavior change. Exceptions: Task 7's RED and GREEN are CI's floci job; some RED results on Windows were not observable (an `ENOTDIR` case, observable only on Linux).
- **Mutation checks:** run on committed code only, after the incident in PR 1's fix wave. Counts recorded per task and wave above (for example Task 5: 4; PR 2 fix wave: 5, 6 and 1; `ea586a8`: 5; Task 11: 3; `a6de728`: 8). The two false survivors of `ea586a8`'s first run were compile errors.
- **Local OpenTofu 1.12.6 against CI's 1.13.2:** the local version validated the one-state swap offline; CI's 1.13.2 is the source of truth.
- **actionlint v1.7.12** was run locally through `go run`; zizmor, shellcheck and `TestReplay` ran in CI only.
- **Branch hygiene:** after PR 1's merge, the old local branch was deleted and PR 2's Task 5 files were briefly written on `main` uncommitted after a failed `switch -c`, then moved intact.
- **Naming rule:** the naming checks ran on each task's work and on this log.
- **What only the harness caught:** the skipped approval passed every check. The workflowspec test pinned the `if` the plan specified, and `TestReplay` does not run the workflows. GitHub's job-skip rules show only in a live run; the new rule test encodes the one learned here.
- **Ledger note:** the ledger lived in `.superpowers/`, which is excluded from git; this log is its durable copy.

## Status

**Phase 2e is complete.** The pipelines plan and apply the AWS stacks against the floci emulator. Each AWS stack runs in jobs of its own that hold no secret; a job replays the version in `wet` before it plans the new one; an apply must equal the gated plan; a stack's render reaches `wet` only after it applied, and its state stays in the job. Nothing reaches a real AWS account. This is shown by the green CI on PRs #39, #40 and #41 (merged as `434633d`, `fb24c40` and `531d63b`), by `TestReplay`'s 5 of 5 passing against floci in PR #40's CI, and by the release PR #28, which still proposes `0.2.0`. The harness run after PR 3's merge is pending (see the watch list). Every finding has a final disposition above.

Next: Phase 2f (the live AWS run, the E2E env-variable checks and `0.2.0`).
