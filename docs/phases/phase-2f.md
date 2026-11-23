# Phase 2f execution log: the E2E AWS checks and the 0.2.0 release

- Period: 2026-11-21 to 2026-11-23
- Plan: [2026-11-20-phase-2f-e2e-aws-and-release.md](../superpowers/plans/2026-11-20-phase-2f-e2e-aws-and-release.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md), amended by A16 (§7.3's `idp-reader` row, §8.5 steps 4 and 5)
- Decisions: no new ADR. [ADR-0027](../adr/0027-end-to-end-harness.md) is amended in place (the harness's reader token), and [ADR-0009](../adr/0009-two-apps-and-split-environments.md) gains one sentence that points to it
- Previous phase: [Phase 2e log](phase-2e.md)
- Status: **complete**. The phase merged through two PRs, both merge commits: [#43](https://github.com/jellalshadows-idp/idp-engine/pull/43) (`fbeaeca`, Tasks 1 to 3, the Phase 2e carries) and [#44](https://github.com/jellalshadows-idp/idp-engine/pull/44) (`803998e`, Tasks 5 to 8, the harness's AWS checks). It needed no fix PR. The release PR [#28](https://github.com/jellalshadows-idp/idp-engine/pull/28) merged as `7c971c2` and tagged [`v0.2.0`](https://github.com/jellalshadows-idp/idp-engine/releases/tag/v0.2.0) on that commit.

This file is the durable copy of the execution ledger kept while the plan ran. The ledger lived in a git-ignored scratch folder that is deleted afterwards, so what it recorded is reproduced here: the rulings, each task's outcome, the reviews, the live results, the disposition of every finding and what is carried forward. Anything the ledger does not state is written as unknown or pending.

## Summary

Phase 2f closes Phase 2. It is the last of the six plans of Phase 2 (2a to 2f).

- **The harness checks a Component's AWS part end to end** (spec §8.5, A16):
  - Its Component claims an ECR registry in `dev` (`aws.registry: true`), and preflight requires `dev`'s `aws` block in `idp-claims-e2e`.
  - The create requires a plan that only creates `aws/dev/components/e2e-r<run>-app`, a successful `apply-aws` leg and the stack's render on `wet`.
  - `dev` must hold exactly `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION` with spec §5.3's values. The update checks them again.
  - The delete requires the `apply-aws` destroy and the render's removal from `wet`. A delete may destroy only this run's modules in the `github` stack and the Component's own AWS stack.
- **The reader App reads the env variables.**
  - The harness mints `idp-reader` tokens for the run's Component repository only, with `environments: read` (ADR-0027, amended).
  - The reader key reaches only the harness step.
- **The three carries from Phase 2e:**
  - OpenTofu gets an allowlist of the job's environment (R1-001).
  - `recover` checks the artifact the issue names through the API before the download (R1).
  - No run script reads a process substitution, and the layout step names each AWS plan it skips (R4, generalized).
- **The release:**
  - `v0.2.0` is published with six archives and `checksums.txt`, each archive attested.
  - Renovate re-pinned the templates of `examples/` and `idp-claims`'s four callers to it.
  - `idp-claims`'s first AWS reconcile created its three baselines against floci.

How it was executed: natively, inline by the parent session, with no writer subagent. Each PR had one Opus whole-branch review, and each candidate had a Gentle AI receipt-driven review, run with the owner's consent. Every behavior change followed strict TDD with an observed RED (see "Process notes"). The work shipped as two PRs, the second cut from `main` after the first merged (D-12).

## Tasks

The plan commit is `0f11a4a` on `main`. PR 1's commits are on `fix/phase-2f-aws-hardening`, and PR 2's on `feat/phase-2f-e2e-aws`, cut from `main` after PR 1's merge.

### PR 1 (#43, merged as `fbeaeca`): Tasks 1 to 4

| Task | What | Commit | Outcome |
|---|---|---|---|
| 1 | OpenTofu gets an allowlist of the job's environment (R1-001) | `f7db005` | Complete. RED: `TestEnv` received `GITHUB_TOKEN` and real AWS keys, and three tokens reached tofu in the CLI test. GREEN. 2 of 2 mutations caught. No workflow passes a `GIT_*` or `TF_*` variable to `idp aws-stack`, so the allowlist drops nothing the jobs use |
| 2 | No run script reads a process substitution; the layout step names each skipped AWS plan (R4) | `e4f013c` | Complete. RED: the new rule named three steps. GREEN. 2 of 2 mutations caught; `rg '<('` over the workflows prints nothing |
| 3 | `recover` checks the artifact the issue names through the API (R1) | `252335d` | Complete. RED: no `artifact-check` step. GREEN. 2 of 2 mutations caught |
| 4 | Ship PR 1, the owner's `aws` block and the first live AWS leg | — | Complete. 12 files, +149/-24 before the fix batch, against a forecast of about 230 lines |

The fix batch after the reviews (see "Reviews"):

| Commit | What |
|---|---|
| `e58d6fc` | `recover` splits the artifact fields on tabs only (`IFS=$'\t'`), so `idp-wet-root ` and ` idp-wet-root` are refused. RED was observed on the pin. A gone artifact gets an `::error::` that names the runbook (RED observed). The runbook lists by `?name=idp-wet-root`. Pins require `issue-check` and `artifact-check` to be unconditional. `TestEnv` pins all 11 allowlist names. 2 of 2 mutations caught |
| `bd11605` | The pipelines runbook tells a deleted artifact (`Not Found`) from a transient `gh` error (passive, docs only) |

CI on `bd11605`: all nine checks SUCCESS, and `floci` ran `TestReplay` with the allowlist, passing 5 of 5 subtests in 175.7 s.

### PR 2 (#44, merged as `803998e`): Tasks 5 to 9

| Task | What | Commit | Outcome |
|---|---|---|---|
| 5 | Spec amendment A16, ADR-0027 (four "Amended in Phase 2f" markers), ADR-0009 (one sentence) | `e0da525` | Complete: readback and naming check clean. Documentation, so no RED |
| 6 | The harness mints reader tokens and reads env variables (`Forge.EnvVariables`, `readerPermissions`, `e2e.yaml`'s reader credentials) | `a0a7eb3` | Complete. The bad-`--run` pin passed on the old code first. RED: the build lacked the new names, and the workflowspec tests lacked the reader env. GREEN. 3 of 3 mutations caught |
| 7a | The harness's Component claims a registry; `Names.AWSStack()`, `Names.AWSRender()` | `e312d40` | Complete. RED: `n.AWSStack` undefined. GREEN |
| 7b | The AWS checks: preflight's `aws` block, the create plan and leg, the render, the variables, the delete's destroy and its ownership | `7a79a5f` | Complete. RED: 18 tests failed, including every run that reached the delete (`does not own: aws/dev/components/e2e-r42-app: …`). GREEN. 6 of 6 mutations caught |
| 8 | `docs/e2e.md` (checks, credentials, setup, failure entries) and the dependencies runbook's floci row | `b46e6aa` | Complete: readback clean. Documentation, so no RED |
| 9 | Ship PR 2 and the live AWS run | — | Complete. 19 files, +544/-68 before the review fixes, against a forecast of about 580 lines |

The review fixes:

| Commit | What |
|---|---|
| `0169557` | `readerSource()` and `TestReaderSourceIsScopedToTheRunsComponent` (RED: `readerSource` undefined). `TestCleanupNeedsNoAWSLegWhenTheCreateNeverAppliedIt` pins the `!cleanup` guard, and a row pins the create's `apply-github` (pre-existing, moved into `requireJobs`). `docs/e2e.md` names `apply-aws` and corrects the stale-render advice. Mutations X1, X4 and X8 are now caught |

CI on `0169557`: all nine checks SUCCESS, CLEAN.

### Task 10: the release, the re-pins and `idp-claims`'s first AWS reconcile

| Step | Result |
|---|---|
| #28 before the merge | CLEAN on `02952f3`: the nine checks and `idp-e2e` SUCCESS, no release run active, a changelog that covers 2a to 2f |
| The merge (the owner's "sí") | `7c971c2`, a merge commit made with `--match-head-commit` |
| The release run | Run 45086481414: `release-please` and `publish` succeeded |
| `v0.2.0` | Seven assets (six archives and `checksums.txt`), neither draft nor prerelease |
| The archives, checked locally | The `linux_amd64` and `windows_amd64` checksums are OK. Both attestations verify: signer `release.yaml`, `refs/heads/main`, digest `7c971c2`. The Windows binary prints `v0.2.0` for `idp version` |
| The harness on the release commit | Run 45086748714 passed (1,835 s). `setup-idp` logged `installed idp v0.2.0, verified against 7c971c2ebbd759d7f8b4519355afb955febcc0c3` |
| Renovate (dispatched with the owner's go, run 45090907242) | `idp-engine` #50 changed exactly the four `uses:` lines of `examples/claims-repo` to `7c971c2 # v0.2.0`. CI 9 of 9; merged as `2ae8a14` |
| `idp-claims` pre-checks | No open "Wet push failed" issue, no running pipeline, the callers at `150b7d35 # v0.1.1`, and no `rendered/aws` on `wet` |
| `idp-claims` #4 (`v0.1.1` to `v0.2.0`) | The plan was `+1 ~0 -0 ±0` for each of `aws/dev/_baseline`, `aws/staging/_baseline` and `aws/pro/_baseline` (the OIDC provider), with no change in `github`. Merged as `304d154` |
| `idp-claims` reconcile 45091904729 | Success with no approval: `plan` 32 s, `plan-aws` 36/36/44 s, `gate` 19 s, `apply-aws` 39/41/39 s, `apply-github` 26 s. `wet` holds `rendered/aws/{dev,pro,staging}/_baseline/main.tf.json` |
| `idp-claims`'s next drift run (05:23 UTC) | Pending at the time of writing |

## Rulings

A ruling is a controller or owner decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong.

- On 2026-11-21 the owner approved native execution, the plan with D-1 to D-18 and both open-question recommendations: `actions_variables: read` allowed after a 403 on `environments: read` alone, and two PRs.
- Phase 2a's owner decisions still hold: Phase 2 is split (2a D-1), and v1 is floci only (2a D-2).
- The plan holds each decision's full text; this is the summary.

- **D-1. Scope:**
  - In scope: the harness's AWS checks, the three 2e carries, the release, the re-pins and this log.
  - Not in scope: Workspaces and `observe`, a plugin cache, any Renovate PR but the engine re-pins, and a real AWS account.
- **D-2. The reader App reads the env variables** (owner's choice):
  - A copy of its key and client ID lives in `idp-e2e` as `IDP_READER_PRIVATE_KEY` and `IDP_READER_CLIENT_ID`.
  - Its tokens are for `e2e-r<run>-app` only, with `environments: read`, minted at the first read.
  - Rejected: a new App and a fine-grained PAT.
  - A16 records the new placement.
- **D-3. The harness's Component gets `aws.registry: true`, and `idp-claims-e2e` gets `dev: { aws: { accountId: "000000000004", region: eu-west-1 } }`** (owner's choice). The expected values are spec §5.3's formulas, written out in the harness rather than computed by `internal/naming`.
- **D-4. What the harness checks:**
  - **Create:** the plan creates only, both `apply` legs succeed, the variables match and the render is on `wet`.
  - **Update:** the variables are checked again.
  - **Delete:** destroys only its own `github` modules and AWS stack, with the render gone from `wet`.
  - **Pin:** still plans no change with the baseline on `wet`.
- **D-5. The live run closes 2e's pending watch items.**
  - A `pattern` download that matches nothing is not arranged live. It was verified in `download-artifact@9000827`'s source instead, and is pinned by `TestWetRootWithNothingToCommit`.
  - A plugin cache only if `init` takes more than half of a leg.
- **D-6 to D-8. The carries:**
  - D-6: the `passEnv` allowlist.
  - D-7: `artifact-check`, which reads `name`, `workflow_run.id` and `expired` through the API.
  - D-8: no `<(`, the layout step's `artifacts="$(jq …)"`, its warning, and `TestRunScriptsReadNoProcessSubstitution`.
  - *Correction made with this log (2026-11-23):* D-8's wording of the warning differed from what Task 2 specified and shipped. The plan now quotes the shipped text: `::warning::no regular file $artifact.json among the downloaded AWS plans; plan-summary fails on the missing plan of that stack`.
- **D-9. Owner-step order:** PR 1, then the `aws` block, then the reader credentials, then PR 2. Held as planned.
- **D-10. The release:** #28 merges only with a green `idp-e2e` on its head, green CI, up to date with `main` and the owner's go. Held: the owner answered "sí" after the green run.
- **D-11. The re-pins:**
  - Both went through Renovate's "idp engine" PRs.
  - The owner chose a dispatch over Monday's 04:17 UTC schedule, and had the controller merge both.
- **D-12. Delivery:** two PRs, merge commits.
- **D-13. Renovate and floci stay on the watch list:** no Renovate PR but the engine re-pins merges in 2f.
- **D-14. The 2e lessons:** the skip rule stays the guard, single artifacts land flat, mutations run on committed code, and the harness is the ship gate.
- **D-15. Time:**
  - The forecast was about 62 minutes per harness run.
  - Measured: 31.6 and 30.6 minutes with the AWS checks. `DefaultTimeouts` are unchanged.
- **D-16 to D-18. This log, the docs placement, and the local tools:**
  - OpenTofu 1.12.6 runs locally from a `mktemp -d` copy, and CI's 1.13.2 is authoritative.
  - There is no local Docker.
  - actionlint v1.7.12 runs locally through `go run`.

Execution rulings:

- **Won't fix "the issue is opened when the keep failed" in 2f:**
  - Why: it needs issue write access and a compromised floci image in the same run, and the issue then says no artifact was saved.
  - *Cost if wrong:* a forged tree reaches `wet` through an edited issue.
  - Carried to Phase 3.
- **PR 2 was developed before the owner's reader credentials, and merged only after them:**
  - Why: the credentials are read only after the merge.
  - *Cost if wrong:* none.
- **The first harness run after PR 2 failed for an environmental reason, so no code changed** (run 45078579841, 1,243 s):
  - At 22:54 UTC the runner lost its network to `api.github.com`. The reconcile-runs read got `context deadline exceeded (Client.Timeout exceeded while awaiting headers)`, so did the cleanup's claim read, and the result step's status POST got `dial tcp …:443: i/o timeout`. githubstatus showed no incident.
  - A loss of several minutes is outside the harness's read retry (2 s, then 5 s) by design.
  - *Cost if wrong:* another flaky run.
  - A longer retry on transport errors is carried as an advisory.
- **The owner approved the cleanup of that run's leftovers, done through the platform:**
  - `idp-claims-e2e` PR #308 deleted the two claims. Its plan was `-3` in `aws/dev/components/e2e-r45078579841-app` and `-13` in `github`: all of them this run's modules, and nothing of the baseline.
  - It merged as `d648521`. Its reconcile 45082745873 gated `approval` with 16 changes, and only its `idp-approval` deployment was approved.
  - The repository and team are gone, and `wet` again holds only `rendered/aws/dev/_baseline`. The re-dispatched run 45083353919 then passed.
- **AWS Emulator (owner question, 2026-11-22): stay on floci.**
  - AWS's official emulator (GA 2026-11-11, CC0) does not keep resources apart by account: one instance serves one account and Region, and ARNs use `000000000000`. That would break the per-environment ARNs (ADR-0004) and the harness's `AWS_ROLE_ARN` check.
  - It also has no published image, serves HTTPS with a self-signed certificate regenerated on every start, and its docs do not state OIDC or ECR support.
  - It was 11 days old, under the 14-day pin rule.
  - *Cost if wrong:* a later migration.
  - Carried as a candidate to re-evaluate once it isolates accounts.
- **Won't fix and accepted rulings** are listed under "Final disposition of every finding".

## Review Focus

| # | Case | Pinned by | Result |
|---|---|---|---|
| 1 | A credential reaching more than it should | `TestE2ESecretsReachOnlyTheHarness`, `TestE2EHarnessMintsTheBotToken`, `TestReaderPermissions`, `TestReaderSourceIsScopedToTheRunsComponent`, `TestGitHubReadsEnvVariablesWithTheReaderToken`, `TestMainUsage`'s reader rows, `TestEnv`, `TestAWSStackKeepsTheJobEnvironmentFromTofu` | Tested. Mutations caught in Tasks 1 and 6, the fix batch and X1. Live: the reader read the variables with `environments: read` alone, with no 403 |
| 2 | The harness approving a destroy it does not own | `TestDeleteAllowsOnlyItsOwnAWSStack`, `TestDeleteRefusesForeignDestroys` | Tested. Task 7's mutations caught. Live: run 45083353919's delete approved only its own stacks |
| 3 | A false pass on the AWS part | `TestScenarioChecksTheEnvVariables`, `TestPreflightRequiresAnAWSBlockForDev`, `TestCreateRequiresTheAWSStackPlannedAndApplied`, `TestCreateRequiresTheAWSRenderOnWet`, `TestDeleteDestroysTheAWSStack`, `TestCleanupNeedsNoAWSLegWhenTheCreateNeverAppliedIt` | Tested. Task 7's 6 mutations and X4 and X8 caught. Live: every check passed in runs 45083353919 and 45086748714 |
| 4 | `recover` trusting an edited issue | `TestRecoverChecksTheRunAndTheWetBase` | Tested. Task 3's and the fix batch's mutations caught. The tab-only split closed a bypass found by review |
| 5 | A run step that hides a failure | `TestRunScriptsReadNoProcessSubstitution`, `checkPlanLayout` | Tested. Task 2's mutations caught. Live: no `no regular file` warning in a run with one AWS plan |

## Reviews

| PR | Whole-branch review (Opus) | Gentle AI review (owner granted) |
|---|---|---|
| #43 | Ready after fixes. Important: the `read` with the default `IFS` trimmed spaces, so `idp-wet-root ` was accepted. Five Minors (below). All fixed in `e58d6fc` and `bd11605`, except one carried and one plan correction made with this log | `252335d` (risk, resilience, reliability): `review-7331ec94d353b994`, approved and acknowledged. The fix delta `e58d6fc` (risk, reliability): `review-0574f1b6ed8fe700`, approved and acknowledged. `bd11605` was docs only: passive |
| #44 | Ready. The only merge blocker was the reader credentials, an owner step. Four Minors, fixed in `0169557`, plus the pre-existing X8. The reviewer confirmed in GitHub's permission tables that `environments: read` alone covers the environment-variables read for installation tokens | `fbeaeca..0169557` (risk, reliability, resilience): `review-0adbb08c441016c1`, approved and acknowledged. Advisories only |

## Watch list

| Item | Result |
|---|---|
| Each PR's check list: `go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke`, `floci`, `release-snapshot` | #43: all 9 SUCCESS on `bd11605`, and `TestReplay` 5 of 5 with the allowlist (175.7 s). #44: all 9 SUCCESS on `0169557`. #50: all 9 SUCCESS |
| The harness after PR 1's merge | Run 44848540048 on `fbeaeca`: success (1,645 s) on the GitHub-only path |
| The owner's `aws` block PR: the first AWS legs in a real pipeline | `idp-claims-e2e` #293, merged as `df4fb9d`: `pr / plan-aws aws/dev/_baseline` green. Reconcile 44855843744: `plan` 33 s, `plan-aws` 53 s, `gate` 32 s (auto), `apply-aws` 57 s, `apply-github` 47 s. `wet` holds `rendered/aws/dev/_baseline/{main.tf.json,.terraform.lock.hcl}` |
| The harness with `dev`'s `aws` block, before PR 2 | Run 44856465837 on `fbeaeca`: success (1,411 s). The callers were already pinned, so no pin PR ran |
| The single-artifact download layout, live | Confirmed: reconcile 44855843744 downloaded one AWS plan by pattern, and its log carries no `no regular file` warning (only the script's own `echo` line) |
| The first live reader read | 200 with `environments: read` alone: the variables matched in runs 45083353919 and 45086748714. No 403, so `actions_variables: read` was not needed |
| The harness after PR 2's merge | Run 45078579841 failed, environmental (see "Rulings"). Run 45083353919 on `803998e`: success in 1,895 s, against D-15's forecast of 62 minutes |
| The create's AWS legs | `idp-claims-e2e` PR #309: `plan-aws aws/dev/components/e2e-r45083353919-app` 74 s. Reconcile 45083746842: `plan-aws` 66 s, `apply-aws` 90 s |
| The update and the case-only edit | #310 (reconcile 45084453585, approved) and #311 (reconcile 45084920951) skipped every AWS job, which kept the literal names `plan-aws ${{ matrix.stack }}` and `apply-aws ${{ matrix.stack }}` |
| The delete's destroy | PR #312: `plan-aws` 79 s (the orphan's destroy). Reconcile 45085554545: `plan-aws` 82 s, `apply-aws` 91 s. The render left `wet`, and no `e2e-r` repository, team or Component render remained |
| The pin PR's baseline leg | Run 45086748714's pin PR #313: `pr / plan-aws aws/dev/_baseline` 43 s, and the plan read "No infrastructure changes." |
| The provider downloads per leg | `Installing hashicorp/aws`: 1 per baseline leg, 2 per Component leg (the baseline's `init` and the stack's). Each install took about 8 s, so about 16 s of a 90-second leg. That is under D-5's half-of-a-leg threshold, so no plugin cache |
| A `pattern` download that matches nothing | Not arranged live (D-5): verified in the action's source and pinned by `TestWetRootWithNothingToCommit` |
| The release PR after each merge | `#28 chore(main): release 0.2.0` until Task 10 |
| The release | See Task 10: run 45086481414, seven assets, and `setup-idp: installed idp v0.2.0` in run 45086748714 |
| `idp-claims`'s first AWS reconcile | See Task 10: three `plan-aws` and three `apply-aws` legs green, and three renders on `wet`. Its next drift run: pending at the time of writing |
| Renovate | The owner had the run dispatched on 2026-11-23 (45090907242), ahead of the 04:17 UTC schedule. It opened the two engine re-pins (merged, D-11), #46 `hashicorp/aws` 6.72.0 labelled `needs-lockfile` (5 checks fail until the lock file is regenerated, as expected), #45 OpenTofu 1.13.9, #47 GoReleaser 2.20.0, #48 Renovate 44.222.5, #49 zizmor 1.32.0, and #51 `go.yaml.in/yaml/v3` to v4. All but the re-pins stay open (D-13). floci 2.5.0 becomes eligible on 2026-11-29 |

## Notable findings caught by review

| Finding | Where caught | Disposition |
|---|---|---|
| `recover`'s `read` with the default `IFS` trimmed spaces, so an artifact named `idp-wet-root ` passed the name check | PR 1 Opus review, Important | Fixed: `IFS=$'\t'` (`e58d6fc`), RED observed, probed with both padded names |
| A 404 for the artifact showed only `gh`'s raw error | PR 1 Gentle AI | Fixed: an `::error::` that names the runbook (`e58d6fc`) |
| The runbook's artifact listing paginates at 30 | PR 1 Opus review, Minor | Fixed: `?name=idp-wet-root` (`e58d6fc`) |
| Nothing pinned that `issue-check` and `artifact-check` run unconditionally | PR 1 Opus review, Minor | Fixed: `If == ""` and `ContinueOnError == ""` pinned (`e58d6fc`) |
| `TestEnv` pinned 6 of the 11 allowlist names | PR 1 Opus review, Minor | Fixed: all 11 (`e58d6fc`) |
| Any `gh` failure, a transient one included, takes the deleted-or-edited message | PR 1 fix-delta Gentle AI, Warning | Fixed in the runbook (`bd11605`): read `gh`'s error; `Not Found` means deleted, anything else is transient and needs a re-run. The shell message is kept |
| The plan's D-8 wording of the warning differed from Task 2's | PR 1 Opus review, Minor | Fixed in the plan with this log |
| The reader token's repository was not pinned | PR 2 Opus review, Minor | Fixed: `readerSource()` and its test (`0169557`) |
| The cleanup's `!cleanup` guard on `apply-aws` was not pinned | PR 2 Opus review, Minor | Fixed: `TestCleanupNeedsNoAWSLegWhenTheCreateNeverAppliedIt` (`0169557`) |
| `docs/e2e.md`'s stale-render advice assumed drift plans AWS stacks; drift plans only `github` | PR 2 Opus review, Minor | Fixed (`0169557`), after checking `drift.yaml` |
| `docs/e2e.md` named only `apply-github` for the create's reconcile | PR 2 Opus review, Minor | Fixed (`0169557`) |
| The create's `apply-github` check was unpinned (X8, pre-existing) | PR 2 Opus review | Fixed: a row in `TestCreateRequiresTheAWSStackPlannedAndApplied` (`0169557`), since the change moved that check into `requireJobs` |

No review found an open Critical or Important issue after its fix batch.

## Final disposition of every finding

Statuses: **Fixed** (done, with the commits), **Carried** to a later phase, **Won't fix** (consciously rejected, with the reason), **Accepted**. No finding is left open without a destination.

### Fixed

The items carried into 2f, from the plan's Self-review map:

- The harness's token to read environment variables: **Fixed**, D-2, Tasks 5 and 6 (`e0da525`, `a0a7eb3`), live in run 45083353919.
- The `aws` block for `dev` in `idp-claims-e2e`: **Fixed**, the owner's PR #293 (`df4fb9d`).
- An `aws.registry` harness claim whose variables the harness checks: **Fixed**, Task 7 (`e312d40`, `7a79a5f`).
- `idp-claims` re-pinned to `v0.2.0`, with its three baselines planned and applied: **Fixed**, `idp-claims` #4 (`304d154`) and reconcile 45091904729. The next drift run is pending.
- The `0.2.0` release: **Fixed**, #28 (`7c971c2`).
- The Phase 2 log: **Fixed**, this file.
- A real AWS leg in a live pipeline: **Fixed**, reconcile 44855843744, then every harness run.
- The provider downloads per leg: **Measured**: 1 per baseline leg, 2 per Component leg, about 8 s each. Under D-5's threshold, so no cache.
- The single-artifact download layout, live: **Confirmed**, reconcile 44855843744.
- A `pattern` download that matches nothing: **Closed by D-5**: verified in the action's source and pinned by a test, not arranged live.
- A tofu environment allowlist (R1-001): **Fixed**, Task 1 (`f7db005`, with `e58d6fc`'s 11-name pin).
- `recover` checks the artifact's name and run through the API (R1): **Fixed**, Task 3 (`252335d`, `e58d6fc`, `bd11605`).
- The layout step logs what it skips and fails on a `jq` error (R4): **Fixed**, Task 2 (`e4f013c`), generalized to every run script.
- The Renovate observations: **Recorded** in the watch list.
- Recover before moving a claims repo to a new engine: **Done**, Task 10's pre-check.
- floci 2.5.0 moves three workflows together: the runbook row is **Fixed** (`b46e6aa`). The bump itself is on the watch list from 2026-11-29.

This phase's findings marked Fixed in "Notable findings caught by review", with their commits.

### Won't fix

- **PR 1:** a duplicate-entry check on `len(passEnv)`, because map keys are unique. Rewording the deleted-or-edited shell message, because the runbook covers the transient case and a new wording would cost another review.
- **PR 2:** asserting `per_page` and the nil-`Reader` branch of `EnvVariables`. Both are minor, and the live run exercised the read.

### Accepted

- The allowlist drops non-secret `TF_*`, `AWS_*` and `GIT_*` configuration (D-6). No workflow passes any: the region comes from the render and the endpoint from the override.
- `TestRunScriptsReadNoProcessSubstitution` matches `<(` literally.
- The reader credential is first used after the create, so a wrong one costs one run and its cleanup through the platform (D-2).
- The `config` step stayed red until the owner added the reader credentials (D-9). The merge waited for them.
- The update leaving the AWS stack alone is described, not checked: the update's fingerprint check refuses any destroy beyond the topics, and the live run skipped every AWS job.
- A mint retry for a repository that was just created: the live runs minted on the first try.

## Carried into later phases

- **Phase 3:**
  - Workspaces (`aws/<env>/workspaces/<name>`), which `idp stacks` refuses until then.
  - `observe` replays.
  - `recover`'s issue opened when the keep failed (the won't-fix ruling above).
  - A longer retry on transport errors in the harness's reads (advisory).
- **Real AWS (beyond v1):**
  - A persistent, encrypted backend (ADR-0030).
  - OIDC credentials.
  - Ordering between stacks.
  - AWS drift.
- **Phase 4:** as the [Phase 2c log](phase-2c.md) lists.
- **The plugin cache:** not carried, since D-5's measure did not call for it.
- **AWS Emulator:** a candidate to re-evaluate once it isolates accounts (see "Rulings").

## Phase 2 exit criterion

Spec §10: "E2E checks the env variables, and floci integration is green". It is met:

- **E2E checks the env variables:**
  - Harness run 45083353919 on `803998e` (Task 9) passed with every AWS check. The reader token read `dev`'s three variables, and they matched spec §5.3.
  - So did run 45086748714 on the release commit `7c971c2` (Task 10 Step 4).
- **floci integration is green:**
  - CI's `floci` job passed on both PRs (#43 on `bd11605`, with `TestReplay` 5 of 5, and #44 on `0169557`).
  - The AWS legs ran live in `idp-claims-e2e` (reconcile 44855843744 and every harness run) and in `idp-claims` (reconcile 45091904729).
- **The release:** [`v0.2.0`](https://github.com/jellalshadows-idp/idp-engine/releases/tag/v0.2.0) on `7c971c2`, with checksums and attestations verified.

## Owner notes

- **The reader key now has two copies:** the claims repos' `IDP_READER_PRIVATE_KEY` and `idp-e2e`'s. When the reader key is rotated, rotate both, and keep `IDP_READER_CLIENT_ID` equal in both places.
- **Release-please** may open a release PR for the `docs` and `chore` commits after `v0.2.0`. Leave it open.
- **Renovate's PRs** #45, #46 (`needs-lockfile`), #47, #48, #49 and #51 wait for the owner. floci 2.5.0 (eligible from 2026-11-29) must move `ci.yaml`, `pr.yaml` and `reconcile.yaml` together.
- The earlier owner items stay as recorded in the [Phase 1e log](phase-1e.md).

## Process notes

- **Execution:**
  - Native: inline by the parent, with no writer subagent.
  - One Opus whole-branch review per PR.
  - Gentle AI receipt-driven reviews of each code candidate, with the owner's consent (lineages in "Reviews").
- **TDD:**
  - Strict; RED was observed for each behavior change.
  - Exceptions: Tasks 5 and 8 are documentation, checked by readback. Task 9's checks are live.
- **Mutation checks:**
  - Run on committed code only, each restored with `git checkout`.
  - Counts: Task 1: 2, Task 2: 2, Task 3: 2, PR 1's fix batch: 2, Task 6: 3, Task 7: 6, PR 2's review: 3 (X1, X4, X8). All were caught.
- **Tools (D-18):**
  - actionlint v1.7.12 ran locally through `go run`.
  - zizmor, shellcheck and `TestReplay` ran in CI only.
- **The `idp-claims-e2e` clone checks out CRLF** (`core.autocrlf=true`), so its edits used tools that keep line endings.
- **Naming rule:** the naming check ran on each task's work and on this log.
- **Ledger note:** the ledger lived in `.superpowers/`, which is excluded from git. This log is its durable copy.

## Status

**Phase 2 is complete.**

- The pipelines plan and apply the AWS stacks against the floci emulator, each stack in a job of its own that holds no secret.
- The end-to-end harness checks a Component's AWS stack, its render on `wet` and its environment variables through `idp-claims-e2e`.
- `v0.2.0` is released, and both claims repos run it.
- Nothing reaches a real AWS account.

The evidence is in "Phase 2 exit criterion". Every finding has a final disposition above.

Next: Phase 3 (Workspaces).
