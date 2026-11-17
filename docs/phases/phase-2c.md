# Phase 2c execution log: the AWS modules and the floci tests

- Period: 2026-11-17
- Plan: [2026-11-17-phase-2c-aws-modules.md](../superpowers/plans/2026-11-17-phase-2c-aws-modules.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md), amended by A12
- Decisions: [ADR-0013](../adr/0013-phase-0-spike-findings.md) amended (the Phase 0 floci digest kind; the trust policy tests now exist), [ADR-0004](../adr/0004-floci-multi-account-emulation.md) amended (the level-3 replay scope)
- Previous phase: [Phase 2b log](phase-2b.md)
- Status: **complete**. The phase merged through [PR #37](https://github.com/jellalshadows-idp/idp-engine/pull/37) (merge commit `368e5a3`). It ships no release: the release PR [#28](https://github.com/jellalshadows-idp/idp-engine/pull/28) "chore(main): release 0.2.0" stays open until Phase 2f.

This file is the durable copy of the execution ledger that was kept while the plan ran. The ledger lived in a git-ignored scratch folder that is deleted afterwards, so what it recorded is reproduced here: the rulings, each task's outcome, the reviews, the live results, the disposition of every finding and what is carried forward.

## Summary

Phase 2c adds the two AWS modules of spec §3.3 and proves them against floci. It is the third of the six plans of Phase 2 (2a to 2f).

- **`modules/aws/baseline`** creates an environment account's GitHub Actions OIDC provider (`https://token.actions.githubusercontent.com`, client id `sts.amazonaws.com`, no thumbprint). Its ARN is `naming.OIDCProviderARN(accountId)`.
- **`modules/aws/component`** creates a Component's ECR repository (immutable tags, scan on push, `force_delete`) and its CI role. The role's trust policy admits only `repo:<org>/<name>:environment:<env>` with `aud` = `sts.amazonaws.com`, and its inline policy `ecr-push` allows the registry login on `*` and five push actions on the repository's own ARN, nothing else. The identities are inputs that the render will fill from `internal/naming` in 2d.
- **`lockfiles/aws.terraform.lock.hcl`** locks `hashicorp/aws` 6.71.0 and is embedded as `lockfiles.AWS`. `TestModulesPinTheirLockedProvider` keeps every module's pin equal to its provider's lock file, and Renovate groups the AWS provider with the label `needs-lockfile`.
- **The `floci` CI job** runs on every PR. `internal/floci`'s `TestFloci` applies both modules to two accounts of one floci 2.4.0 service container with naming's inputs, checks every identity and policy through the AWS API, replays each stack (a plan with no changes), destroys everything and checks that nothing is left. Its offline tests run in every `go test ./...`.
- **Claims validation** gives each environment its own AWS account and accepts only regions of the standard `aws` partition.
- **CI's `modules` job** tests every module with its provider's lock file, and `render-smoke` holds its fixture list once.
- **Spec amendment A12** covers §4.7, §8.4 and §11 risk 4.
- No render changed: no render calls the modules yet (2d), and the pipelines are unchanged (2e).

How it was executed: inline in the controller session, with no per-task subagents. Every task followed TDD with an observed RED and GREEN, and seven mutation checks were run (see "Process notes"). After the push, a whole-branch review by the most capable model and a Gentle AI receipt-driven review ran on the branch. One fix wave addressed both. A scoped re-review and a second receipt-driven review then checked that wave.

CI evidence on `idp-engine`: the CI runs 44054303902 (the branch) and 44073825584 (after the fix wave) on PR #37 were green on every check, `floci` included, on the first try.

## Tasks

Commits are on `feat/phase-2c-aws-modules`, created from `main` after the plan commit `7d1b009`. The branch merged as `368e5a3`.

| Task | What | Commit | Outcome |
|---|---|---|---|
| 1 | Spec amendment A12, ADR-0013 amended | `859a896` | Complete: A12 appears 3 times, A11 still 4; ADR-0013 amended twice; anchors resolve; stale-text and naming checks clean |
| 2 | Claims: one AWS account per environment; the standard partition only | `f8db48f`, `153328a` | Complete. Accounts: RED on the 2 new cases, GREEN 23 subtests, and a mutation check failed both. Partition: RED build failure (`undefined: partitionOf`), GREEN 24 subtests plus `TestPartitionOf`. Naming's doc and `docs/claims.md` updated |
| 3 | The AWS lock file | `6c646c3` | Complete. RED `undefined: AWS`. The shipped file is byte-identical to `tofu providers lock` output (`diff exit 0`). GREEN |
| 4 | `modules/aws/baseline` | `bc4e9db` | Complete. RED 0 passed, 1 failed, 1 skipped; GREEN 2/2 |
| 5 | `modules/aws/component` | `d3377cc` | Complete. RED on `names_are_the_ones_naming_computed`; GREEN 8/8; three mutation checks |
| 6 | Renovate and the pin–lock contract | `db070fa` | Complete. RED/GREEN on `TestRenovateRules`. `TestModulesPinTheirLockedProvider` passes, and fails on a 6.72.0 pin (mutation). Runbook rows and section added |
| 7 | `internal/floci` | `2310264` | Complete. RED twice (build failures), GREEN on 4 offline tests with `TestFloci` skipped. With the endpoint set, `TestFloci` fails (`tofu is not on PATH`), never skips. The issuer mutation failed the Go test and the module test |
| 8 | CI: every module with its lock, the floci job, one fixture list | `1adb98c` | Complete. RED on exactly the 3 planned tests, then GREEN. Removing `exit 1` from the readiness wait failed `TestCIFlociJob` (mutation) |
| 9 | Carried item (a): the shadowed loop variable | `a865f3f` | Complete. A refactor that passes before and after |
| 10 | Ship: verification, PR #37, CI, reviews, fix wave, merge, release PR, this log and the README | PR #37, merge `368e5a3`; this commit | See "Ship" |

Fix wave, after the reviews of the branch:

| Commit | What |
|---|---|
| `31d285c` | `internal/floci`: `wantMissing` requires `NoSuchEntity` for a missing resource; `awsJSON` decodes stdout only; `strictJSON` refuses data after the document (RED: "a second document after the policy"); the offline test's comment says what it ties together |
| `89628ef` | `internal/claims`: environments written on one line are ordered by name (RED: 26 of 30 runs failed; GREEN: 30 of 30) |
| `13b3622` | `ci.yaml`: the readiness wait uses `curl --max-time 2` and a 60-second deadline; `floci-logs` runs on `failure() \|\| cancelled()`; `setup-go` saves no cache in the floci job; `TestCIFlociJob` pins all of it plus the job's `if` and `continue-on-error` (RED: 3 failures) |
| `3f91c62` | Docs: ADR-0004's follow-up on the cross-version replay; the runbook's floci section records the Docker socket trust and the unpinned `registry:2` |

## Rulings

A ruling is a controller or owner decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong. The owner approved the plan, D-1 to D-20 and its side effects on 2026-11-17. Phase 2a's owner decisions still hold: Phase 2 is split (2a D-1) and v1 is floci only (2a D-2). The plan holds each decision's full text; this is the summary.

- **D-1. Two environments may not share an `accountId`.** Each environment's baseline creates its account's GitHub OIDC provider, and an account holds one. `claims.Load` reports the later environment in file order, at its `accountId` line. This closes 2a's D-11. `idp-claims` (three accounts) still validates. *Cost if wrong:* lifting the check accepts what it rejects today; nothing it rejects exists.
- **D-2. The floci job runs on every PR**, as a job of `ci.yaml`, with `timeout-minutes: 20`. *Cost if wrong:* a path filter later.
- **D-3. The CI role may only push to its own repository:** `ECRLogin` (`ecr:GetAuthorizationToken` on `*`) and `ECRPush` (five push actions on the repository ARN); no pull. The trust policy is one statement with `StringEquals` on `aud` and `sub`. *Cost if wrong:* one action added; Phase 4 may need `ecr:BatchGetImage`.
- **D-4. ECR settings:** `IMMUTABLE` tags, `scan_on_push`, `force_delete` (an approved delete must not fail on a repository with images). *Cost if wrong:* in-place updates.
- **D-5. Pins:** `hashicorp/aws` 6.71.0 and floci 2.4.0 by its multi-arch index digest, both at least 14 days old. *Cost if wrong:* a one-line pin change.
- **D-6. The lock file**, generated with the runbook's four `-platform` flags. Finding: the registry signs every package of a release, so the flags do not change the hashes. *Cost if wrong:* regenerate it.
- **D-7. No `render.AWSProviderVersion` in 2c;** 2d adds it with its regex manager and golden files. *Cost if wrong:* 2d adds it, as planned.
- **D-8. Identities in, subject built:** the component module takes `repository_name`, `role_name` and `oidc_provider_arn` from naming, builds only the trust policy's subject, and validates every input that reaches the trust policy. *Cost if wrong:* an interface change before any render calls it.
- **D-9. No thumbprint:** optional in provider 6.71.0; IAM ignores it for GitHub; floci accepts its absence. *Cost if wrong:* an in-place update.
- **D-10. floci runs as a service container** with the Docker socket mounted, on the job's network, with a readiness wait that fails. Finding: floci 2.4.0's entrypoint grants the socket's group to its own user, so the spike's `-u root` is not needed. *Cost if wrong:* a `docker run` step and a regex manager.
- **D-11. floci's backing `registry:2` is not pinned** by this repository: accepted gap, recorded in the runbook. *Cost if wrong:* an environment variable and a regex manager.
- **D-12. Level 3 is a Go test that drives `tofu`** (A12), because `tofu test` assertions cannot call `internal/naming`. It is gated by `IDP_FLOCI_ENDPOINT`, and the cross-version replay waits for a released module change. *Cost if wrong:* about one task to rewrite it as `tofu test`.
- **D-13. Regions outside the standard partition are rejected** (carried R3-partition-silent). Finding: floci 2.4.0 places each request in its region's partition, so the 2b premise ("the partition does not matter on floci") was false. *Cost if wrong:* remove the check.
- **D-14. Renovate:** one "aws provider" rule; floci is tracked by the `github-actions` manager through the service image, so no custom manager was added (this corrected the controller's brief). *Cost if wrong:* a regex manager if Renovate misses the image.
- **D-15. The carried items (a) and (b):** the loop variable rename, and `render-smoke` in one loop pinned by `TestCIRenderSmokeCoversEveryFixture`. *Cost if wrong:* none.
- **D-16. The `modules` job takes each module's lock from its provider directory.** *Cost if wrong:* a matrix `include`.
- **D-17. No render change and no new ADR;** A12 and the ADR amendments cover the stale text. *Cost if wrong:* one ADR later.
- **D-18. Local OpenTofu 1.12.6; CI's 1.13.2 is the source of truth.** It held: CI passed. *Cost if wrong:* one CI round trip.
- **D-19. Two facts the tests rely on:** `aws_iam_role.path` is null under `mock_provider`, and an HCL object key may be a template. *Cost if wrong:* none.
- **D-20. No release in 2c.** *Cost if wrong:* none; the rule is the same.

Execution rulings:

- **Execution inline.** The controller ran every task itself, applying the plan's blocks with a helper script. *Cost if wrong:* no per-task reviewer; the whole-branch reviews cover the branch.
- **The first Gentle AI review ran on the branch before the fix wave,** because the owner granted it on that candidate; the fix wave then became a second candidate with its own review. *Cost if wrong:* one extra review of 162 lines.
- **All nine Minor findings and the five Gentle AI advisories were fixed in one wave,** instead of carried. `TestExpectedPoliciesAreTheModulesOwn` keeps its name (the plan's Global Constraints) and its comment was reworded. *Cost if wrong:* none.
- **`wantMissing`'s `NoSuchEntity` expectation could only be proven in CI.** A failure there would have been a fidelity gap to document, not an assertion to loosen. It passed on the first run.
- **The second review's two suggestions are carried into 2d** rather than reopening an approved candidate. *Cost if wrong:* two test gaps wait one sub-phase; CI's floci job covers both behaviors meanwhile.

## Review Focus

| # | Case | Pinned by | Result |
|---|---|---|---|
| 1 | Two environments that share an AWS account | `TestLoadDiagnostics` (both file orders, and the one-line case added by the fix wave); `TestLoadPlatformAWS` | Tested; mutation-checked in Task 2 |
| 2 | The trust policy's `sub` or `aud` drifting from spec §4.4 | Module runs `the_trust_policy_admits_only_the_components_ci_in_its_environment` and `the_subject_follows_the_environment`; `TestExpectedPoliciesAreTheModulesOwn`; `TestFloci` on the created roles | Tested; mutation-checked in Task 5 (`StringLike`); passed against floci |
| 3 | A role policy that allows more than its own repository | Module run `the_ci_role_may_only_push_to_its_repository` (whole document); `TestDecodePolicy`; `TestFloci`'s policy listings | Tested; mutation-checked in Task 5 (`Resource = "*"`); passed against floci |
| 4 | A module name drifting from `internal/naming` | `TestFloci`; module run `names_are_the_ones_naming_computed`; `TestModulesUseNamingsIssuer` | Tested; mutation-checked in Tasks 5 (`path = "/ci/"`) and 7 (a trailing slash on the issuer) |
| 5 | The floci job failing open, or leaving resources behind | `TestCIFlociJob` (readiness `exit 1`, deadline and `--max-time`, no `if` or `continue-on-error` on the steps or the job, the endpoint on the step, a timeout); `TestFloci`'s destroys registered before each apply and `wantNothingLeft` | Tested; mutation-checked in Task 8; seen live: nothing was left after both CI runs |

Also tested, outside the five: `TestModulesPinTheirLockedProvider` (mutation-checked), the Renovate rule, `TestCITestsEveryModule`, `TestCIRenderSmokeCoversEveryFixture`, the partition rule (`TestLoadDiagnostics`, `TestPartitionOf`), the lock file (`TestAWSLockPinsTheProvider`), the replay plan with no changes and account isolation (`TestFloci`, CI).

## Ship (Task 10)

1. Local verification was clean before the push: 31 files; the modules `aws/baseline` 2/2, `aws/component` 8/8, `github/group` 5/5 and `github/component` 12/12; no change to the render, the GitHub modules or the GitHub lock file.
2. The branch was pushed as PR #37. CI run 44054303902 passed all nine checks on the first try.
3. **Final whole-branch review** (most capable model): approve with fixes, 0 Critical, 0 Important, 9 Minor (see below). It found the IAM design tight: no input can widen the trust policy, and the push policy cannot reach another repository.
4. **Gentle AI receipt-driven review** of the branch (the owner granted consent; risk high because of the shell in `ci.yaml`; lenses risk, resilience and reliability): approved and acknowledged (lineage `review-b1cc013067d25994`), with five non-blocking advisories.
5. **Fix wave** (`31d285c`, `89628ef`, `13b3622`, `3f91c62`): every Minor and every advisory. CI run 44073825584 passed all nine checks.
6. **Scoped re-review** of the wave: all nine findings addressed, no blocking defect, one nit.
7. **Second Gentle AI review**, of the fix wave (the owner granted consent; lenses resilience and reliability): approved and acknowledged (lineage `review-bad8fe16507f17b5`), with two suggestions.
8. PR #37 merged as `368e5a3`.

## Watch list

| Item | Result |
|---|---|
| The first `floci` job passes | Seen live: run 44054303902 passed in 2m28s (2m16s after the fix wave). `TestFloci` took 113 s, with the subtests `dev`, `pro` and `accounts are isolated` |
| The readiness wait and the service health check | The runner waited for the image's health check ("floci service is healthy", 8 s of container initialization), so `floci-ready` took 1 s |
| floci 2.4.0's repository URI form | `000000000001.dkr.ecr.eu-west-1.localhost:4566/api`, the same form ADR-0013 recorded for 2.1.0. Nothing compares it |
| `NoSuchEntity` for a missing role, across accounts and after destroy | Seen live in run 44073825584: `TestFloci` passed with `wantMissing` |
| The PR's check list | `go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke`, `floci` and `release-snapshot`, all green |
| After the merge, release PR #28 still proposes `0.2.0` (D-20) | Seen live: the merge's release run (44083920206) succeeded and release PR #28 still proposes "chore(main): release 0.2.0". It stays open until 2f |
| The harness run dispatched for that release PR passes | Seen live: run 44084048822 (`workflow_dispatch`) ended with conclusion `success` |
| The next weekly Renovate run (Mondays 04:17 UTC): `floci/floci` under `github-actions`, `hashicorp/aws` under `terraform`, and `hashicorp/aws` 6.72.0 as an "aws provider" PR labelled `needs-lockfile` | Not yet run; carried into 2d |

## Notable findings caught by review

| Finding | Where caught | Disposition |
|---|---|---|
| The readiness `curl` had no `--max-time`, so a silent floci held the wait past 60 s | Final review, Minor 1 | Fixed, `13b3622` |
| `floci logs` did not run when the job timed out | Final review, Minor 2 | Fixed, `13b3622` |
| Environments on one line were reported in map-iteration order | Final review, Minor 3 (reproduced: 181 of 200 runs one way) | Fixed, `89628ef` |
| `TestCIFlociJob` did not pin the job's `if` and `continue-on-error`, so a skipped job could pass | Final review, Minor 4 | Fixed, `13b3622` |
| The floci job, which holds the Docker socket, saved a Go cache that other jobs restore | Final review, Minor 5 | Fixed, `13b3622` |
| `decodePolicy` read only the first JSON value | Final review, Minor 6 | Fixed, `31d285c`; duplicate keys documented as not detected |
| The isolation check passed on any AWS CLI failure | Final review, Minor 7; Gentle AI R3-isolation-vacuous and R4-isolation-vacuous-pass | Fixed, `31d285c` |
| `TestExpectedPoliciesAreTheModulesOwn`'s comment claimed it read the module | Final review, Minor 8 | Fixed, `31d285c` |
| ADR-0004 still promised the cross-version replay; D-11 was recorded only in the plan | Final review, Minor 9 | Fixed, `3f91c62` |
| `wantNothingLeft`'s role check passed on any failure | Gentle AI R3-nothing-left-vacuous-role | Fixed, `31d285c` |
| `awsJSON` decoded stdout and stderr together | Gentle AI R3-combined-output-json | Fixed, `31d285c` |
| A floci update should be reviewed as a security change (the Docker socket) | Gentle AI R1-floci-docker-socket | Fixed, `3f91c62` (runbook) |
| `wantMissing` has no offline test | Second Gentle AI review, R3-001 | Phase 2d |
| The `setup-go` cache check passes if the job has no `setup-go` step | Scoped re-review (nit); second Gentle AI review, R3-002 | Phase 2d |

No review found a Critical or Important issue.

## Final disposition of every finding

Statuses: **Fixed** (done, with the commits), **Phase 2d** and later sub-phases, **Won't fix** (consciously rejected, with the reason). No finding is left open without a destination.

### Fixed

The three items that Phase 2b carried into this phase:

- Opus Minor (`render_test.go`, the shadowed loop variable): **Fixed**, Task 9 (`a865f3f`).
- R2-ci-fixture-list-duplicated: **Fixed**, Task 8 (`1adb98c`).
- R3-partition-silent: **Fixed**, Task 2 (`153328a`, D-13).

Every finding of this phase's reviews in the table above marked Fixed.

### Phase 2d

- `wantMissing` offline test (second Gentle AI review, R3-001): exercise both failure branches against a stub command.
- The `setup-go` cache check (R3-002 and the re-review nit): require exactly one `setup-go` step in the floci job.

## Carried into later sub-phases

- **2d.**
  - Render `aws/<env>/_baseline` and `aws/<env>/components/<name>` with these modules' inputs (`github_repository` = `<org>/<name>`).
  - The AWS stacks' backend and encryption.
  - `render.AWSProviderVersion` with its Renovate regex manager and golden files (D-7).
  - Move `TestFloci` from `writeStack` to the real render.
  - The nested-stack discovery items of [Phase 1b](phase-1b.md).
  - The two items above, and the Renovate observations of the watch list.
- **2e.** The workflows write `floci_override.tf.json` and start floci per job (the `floci` job's service block is the model), and replay from `wet`. The floci job's measured time (2m16s to 2m28s for four stacks, including about 30 s of setup) feeds spec §11 risk 3.
- **2f.** As the [Phase 2b log](phase-2b.md) lists: the harness's token to read environment variables, the `aws` block for `dev` in `idp-claims-e2e`, and an `aws.registry` harness claim whose variables the E2E checks; the `0.2.0` release; the Phase 2 log.
- **Phase 4.**
  - `container-ci` must push unique tags, because they are `IMMUTABLE` (D-4).
  - On real AWS it may need `ecr:BatchGetImage` (D-3).
  - Repository-level `scan_on_push` versus AWS's registry-level scanning, if the project ever goes live.
- **Accepted:** floci's backing `registry:2` is not pinned by this repository (D-11).

## Owner notes

- The release PR #28, proposing `0.2.0`, stays open until 2f (D-20). Every push to `main` re-runs the harness on it while it is open (about 20 minutes each, ADR-0028). This log is a `docs` commit, so its push does that once more.
- The earlier owner items stay as recorded in the [Phase 1e log](phase-1e.md); they are not new asks.

## Process notes

- **Execution:** inline in the controller session. Every task followed TDD with an observed RED and GREEN, and so did each fix of the fix wave, except `wantMissing`, whose behavior only a live floci can show. Fix rounds: one wave after the branch reviews.
- **Mutation checks (seven, each reverted):** the shared-account check (Task 2); `Resource = "*"`, `path = "/ci/"` and `StringLike` in the component module (Task 5); a 6.72.0 pin without its lock (Task 6); a trailing slash on the baseline's issuer (Task 7, which failed both the Go test and the module test); removing the readiness wait's `exit 1` (Task 8). Each failed exactly the runs the plan predicted.
- **Local OpenTofu 1.12.6 against CI's 1.13.2:** the module runs, `render-smoke` and the floci job passed on 1.13.2 in CI.
- **Naming rule:** the naming checks ran on each task's work and on this log, and were clean.
- **Ledger note:** the ledger lived in `.superpowers/`, which is excluded from git; this log is its durable copy.

## Status

**Phase 2c is complete.** `modules/aws/baseline` and `modules/aws/component` exist. CI applies them to two accounts of the floci emulator on every PR, checks their identities against `internal/naming` and their policies against spec §4.4, replays and destroys them. Claims validation gives each environment its own AWS account in the standard partition. No render calls the modules yet. This is shown by the green CI on PR #37, merged as `368e5a3`, by the release run 44083920206, after which the release PR #28 still proposes `0.2.0`, and by the harness run 44084048822, which ended with `success`. Every finding has a final disposition above.

Next: Phase 2d (rendering the AWS stacks).
