# Phase 2b execution log: the deterministic env variables

- Period: 2026-11-03 to 2026-11-16 (the session was interrupted between those dates)
- Plan: [2026-11-03-phase-2b-env-variables.md](../superpowers/plans/2026-11-03-phase-2b-env-variables.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md), amended by A10 and A11
- Decisions: [ADR-0013](../adr/0013-phase-0-spike-findings.md), amended (the trust policy tests are future tense)
- Previous phase: [Phase 2a log](phase-2a.md)
- Status: **complete**. The phase merged through [PR #35](https://github.com/jellalshadows-idp/idp-engine/pull/35) (merge commit `c6aa267`). It ships no release: the release PR [#28](https://github.com/jellalshadows-idp/idp-engine/pull/28) "chore(main): release 0.2.0" stays open until Phase 2f.

This file is the durable copy of the execution ledger that was kept while the plan ran. The ledger lived in a git-ignored scratch folder that is deleted afterwards, so what it recorded is reproduced here: the rulings, each task's outcome, the reviews, the live results, the disposition of every finding and what is carried forward.

## Summary

Phase 2b renders and creates the GitHub environment variables of a Component with `aws.registry: true`. It is the second of the six plans of Phase 2 (2a to 2f).

- **`modules/github/component`** takes `variables` inside each entry of `environments` (an optional `map(string)`, default `{}`) and creates one `github_actions_environment_variable` per environment and name, keyed `"<env>/<name>"`. Each variable depends on its environment. With no variables the resource has no instances.
- **The renderer** passes `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION` for a Component with `aws.registry: true`, for each environment it lists, computed by `internal/naming` from the names and the environment's `aws` block. A Component without `aws.registry` renders byte for byte as before: `basic`'s golden file is unchanged. A registry Component whose environment has no `aws` block makes `Render` return an error instead of rendering less.
- **`TestRenderIgnoresTheAWSPart` became the env-variables golden** (`TestRenderEnvVariablesGolden`), with the `aws-registry` fixture's own golden file. Phase 2a's guard that the `aws` blocks alone change nothing survives as `TestRenderWithoutRegistryPassesNoVariables`.
- **CI's `render-smoke`** now validates both fixtures against the local modules.
- **Spec amendment A11** and one amended sentence in ADR-0013 close the Phase 2a review items that were spec text. The other carried items were fixed in `internal/naming` and `docs/claims.md`.
- No lock file, provider pin or release changed. Only the github stack is rendered; the AWS stacks are Phase 2d.

The plan corrected the controller's brief, verified against spec §5.3: the variables live inside each environment entry (D-1, D-2), and the existing reconcile applies them together with the github stack (D-12).

How it was executed: inline in the controller session, with no per-task subagents, across a session break (2026-11-03 to 2026-11-16) during which the session scratchpad was cleaned and the helper scripts were recreated. Every task followed TDD with an observed RED and GREEN, and three mutation checks were run (see "Process notes"). A whole-branch review by the most capable model followed the push, and a Gentle AI receipt-driven review ran on the candidate.

CI evidence on `idp-engine`: the CI run 43964802143 on PR #35 was green, and `render-smoke` validated both fixtures on OpenTofu 1.13.1.

## Tasks

Commits are on `feat/phase-2b-env-variables`, created from `main` after the plan commit `d792548`. The branch merged as `c6aa267`.

| Task | What | Commit | Outcome |
|---|---|---|---|
| 1 | Spec amendment A11, ADR-0013 amended | `005e298` | Complete: 4 replacements applied from the plan; anchors resolve; A11 appears 4 times, A10 6 times; ADR-0013 amended once; the stale-text and naming checks print nothing |
| 2 | `internal/naming`: uppercase after the first position; the partition | `4fc6313`, `acaf514` | Complete. The gap was shown: the widened rule passed the old tests. RED: `aBc`. The rule was restored. GREEN: 13 subtests. The partition limit is documented. Gates clean |
| 3 | The module creates the environments' variables | `e33e834` | Complete. Module baseline 9/9. RED: 8 passed, 1 failed, 3 skipped. GREEN: 12/12. Mutation check, reverted. Go gates clean |
| 4 | The render passes the env variables | `2c23e0b` | Complete. The Renovate guard failed on the new golden file and then passed. RED: exactly 3 render tests. GREEN: 6. `basic`'s golden file unchanged. Docs updated. Mutation check, reverted. Gates clean |
| 5 | CI validates the env-variables render | `ae4269e` | Complete. `render-smoke` renders and validates `basic` and `aws-registry`; `workflowspec` passes; actionlint, zizmor and the job itself run in CI |
| 6 | Ship: verification, PR #35, CI, reviews, merge, release PR, this log and the README | PR #35, merge `c6aa267`; this commit | See "Ship" |

No task had a separate reviewer; the reviews are the two whole-branch reviews below. Task details:

- **Task 1.** A11 covers spec §3.3 (the `internal/claims` and `internal/config` rows), §8.3 (the trust policy's `aud` added next to `sub`) and §11 risk 4. ADR-0013's note on the trust policy tests is now future tense.
- **Task 2.** Adds the carried `aBc`, `dEv` and `deV` cases to the uppercase check, and documents the standard `aws` partition limit in naming's package doc and in `docs/claims.md` (D-8).
- **Task 3.** `variables.tf` gets the optional `variables` attribute; `main.tf` gets `local.environment_variables` and `github_actions_environment_variable.this`; three new runs: `environments_have_no_variables_by_default`, `variables_for_two_environments` and `only_environments_with_variables_get_them`.
- **Task 4.** `render.go` gains `envVariables`; the tests gain `checkGolden`, `TestRenderEnvVariablesGolden`, `TestRenderWithoutRegistryPassesNoVariables`, `TestRenderEnvVariablesFollowEachEnvironment` and `TestRenderRejectsARegistryEnvironmentWithoutAWS`. `TestRenovateCustomManagersFindThePins` lists the new golden file. `docs/claims.md` rewrites the R3-001 bullet and says when the variables are created.
- **Task 5.** `render-smoke` loops over the two fixtures, so the check's name stays `render-smoke`.

## Rulings

A ruling is a controller or owner decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong. The owner approved the plan, D-1 to D-17 and inline execution on 2026-11-03. Phase 2a's owner decisions still hold: Phase 2 is split (2a D-1) and v1 is floci only (2a D-2).

- **D-1. The input shape: `variables` inside each entry of `environments`.** The type becomes `map(object({ protected = bool, variables = optional(map(string), {}) }))`. This is spec §5.3's own rendered example, so §5.3 needs no amendment. A separate `variables` input keyed by environment would contradict §5.3 and need a validation across two inputs. Existing callers keep working. *Cost if wrong:* moving to a separate input changes the module's `variables.tf` and `main.tf`, its three new runs, six lines of the render and one golden file; nothing is applied from a release before `0.2.0`.
- **D-2. No module validation, so no validation message.** A variable can be declared only inside an environment entry, and that entry is the environment it belongs to, so a variable for an environment the module does not create cannot be expressed. The run `only_environments_with_variables_get_them` replaces the "unknown environment" test a separate input would have needed. On the render side, `TestRenderEnvVariablesFollowEachEnvironment` shows that an environment the Component does not list gets nothing, even when the platform gives it an `aws` block. *Cost if wrong:* a validation can be added later without rejecting any input that is valid today.
- **D-3. The resource address is `github_actions_environment_variable.this["<env>/<name>"]`.** Environment names never contain a slash (spec §4.6), so each key names exactly one variable, and OpenTofu orders `for_each` keys lexically. Each instance takes its `environment` from `github_repository_environment.this[...]`, so it is created after that environment and destroyed before it. *Cost if wrong:* renaming the keys later moves state addresses; before any registry Component is applied there is no state to move.
- **D-4. Only what the render passes (YAGNI).** The type is a plain `map(string)`; the module does not restrict the names. Its only caller is the renderer, and the golden file pins the three names. *Cost if wrong:* none; a later caller passes other names, or a validation restricts them then.
- **D-5. Existing state is untouched.** For a Component without `aws.registry` the new resource has zero instances, input types are not stored in state, and no existing resource is renamed. Live facts read on 2026-11-03: neither `idp-claims` nor `idp-claims-e2e` has any file under `claims/` on `main`; production `idp-claims` pins the engine at `v0.1.1`; the harness's claims carry no `aws` block. Proof: `basic`'s golden file unchanged byte for byte, the run `environments_have_no_variables_by_default`, and the harness on the release PR, whose pin PR and case-only step both require `No infrastructure changes.`. *Cost if wrong:* an unexpected create would fail the harness's pin or case-only step before any release.
- **D-6. The renderer passes `variables` when, and only when, the Component has `aws.registry: true`,** for each environment it lists. The values come from `naming.RoleARN`, `naming.RegistryURL` and the region, through `literal`. A registry Component in an environment without an `aws` block makes `Render` return `render: component "<name>" has aws.registry, but environment "<env>" has no aws block`. It never panics and never renders the Component without its variables, because that render would plan their deletion. `claims.Load` already rejects that case (2a's D-8), so only a caller that bypassed `Load` sees the error. *Cost if wrong:* none.
- **D-7. The render tests.** `TestRenderIgnoresTheAWSPart` becomes `TestRenderEnvVariablesGolden` with its own golden file, and it checks `pro`'s variables against spec §5.3's values verbatim, outside the golden file, so that `-update` cannot change them unnoticed. 2a's interim guard becomes `TestRenderWithoutRegistryPassesNoVariables`. `TestRenderIsDeterministic` moves to the `aws-registry` fixture and also shuffles each Component's environments. `TestRenovateCustomManagersFindThePins` must list the new golden file; the existing Renovate patterns already move its pins. *Cost if wrong:* none.
- **D-8. Carried Minor 3: document "standard `aws` partition only"; do not narrow the region pattern.** Naming's identities use `arn:aws:` and `amazonaws.com`, while the platform schema checks only a region's shape, so it also accepts GovCloud and China regions. The validators use Go's `regexp`, which has no lookahead, so narrowing would need an allowlist of prefixes that a future standard region could break. In v1 every region is the floci emulator. The limit is documented in naming's package doc and in `docs/claims.md`; the `schemas_test.go` row "aws region with three parts" stays on purpose. *Cost if wrong:* going live in another partition needs naming to derive the partition and host suffix from the region.
- **D-9. Carried Minor 2: future tense, and `aud` in §8.3 now.** ADR-0013's note and spec §11 risk 4 now say the trust policy "will be asserted by the AWS module tests (Phase 2c)". A11 adds `aud` to §8.3, which listed only `sub`, so that 2c implements one complete list. *Cost if wrong:* one line of spec text.
- **D-10. A11 covers only text that 2b makes stale or that a carried item fixes:** §3.3's rows (Minor 4), §8.3 (D-9) and §11 risk 4 (Minor 2). §4.4, §5.3, §5.5, §6 and §10 were not stale; `docs/pipelines.md` stays true until 2e. *Cost if wrong:* none.
- **D-11. Carried R3-001 (`docs/claims.md:101`): a real wording issue.** "nothing renders from it until Phases 2b to 2d" did not say which part arrives when and stops being true once 2b ships. Task 4 rewrote the bullet. *Cost if wrong:* wording only.
- **D-12. The variables are created when the pipelines apply the github stack, which they already do.** Rendering creates nothing. Reconcile's `apply-github` applies the whole github stack, so once a claims repo runs an engine with 2b, a merged registry Component gets its variables. No App permission changes: GitHub's documentation lists the environment variable endpoints under the "Environments" repository permission, which the writer App has for writes and the reader App for reads. This corrects the controller's brief ("their application is still Phase 2e"): what waits for 2d and 2e is the ECR repository and the CI role that the variables name. No claims repo runs an engine with 2b before `0.2.0`. A delete (turning `aws.registry` off) waits for approval like every delete. *Cost if wrong:* documentation wording.
- **D-13. `render-smoke` also validates the `aws-registry` render** by looping over the two fixtures. With OpenTofu 1.12.6, `tofu validate` rejects a `variables` value of the wrong type but silently drops an attribute the module does not declare, so it cannot catch a rename. The attribute's name is pinned on both sides instead: the module runs count instances created from `variables`, and the render test reads `environments.pro.variables`. *Cost if wrong:* a few seconds more of `render-smoke`.
- **D-14. Local `tofu test` uses OpenTofu 1.12.6; CI's 1.13.1 is the source of truth.** The binary is executed from `~/.idp/tools/tofu-1.12.6/`, never written to, and the module is copied to a scratch directory for `init` and `test`. *Cost if wrong:* one CI round trip if 1.13.1 differs. It did not: CI passed.
- **D-15. The lock file and the provider pin are unchanged.** `github_actions_environment_variable` exists in `integrations/github` 6.13.0. *Cost if wrong:* none.
- **D-16. No release in 2b.** After the merge, release PR #28 still proposes `0.2.0` and stays open until 2f (2a's D-15). Every push to `main` re-runs the harness on it, about 20 minutes each. *Cost if wrong:* if it proposed another version, the rule is the same: do not merge it.
- **D-17. No new ADR.** ADR-0006 (identities computed by `internal/naming`, never read from AWS) decides 2b. A11 records the spec edits and ADR-0013 gets one amended sentence. *Cost if wrong:* one ADR later.

Execution rulings:

- **Execution inline.** The controller ran every task itself, with no per-task subagents. *Cost if wrong:* no separate reviewer per task; the whole-branch reviews cover the branch.
- **Carry the review Minor and both Gentle AI advisories into the Phase 2c plan** instead of reopening the approved candidate before the merge. The partition advisory is D-8's documented behavior. *Cost if wrong:* three small items wait one sub-phase.

## Review Focus

| # | Case | Pinned by | Result |
|---|---|---|---|
| 1 | A registry Component whose environments are in different accounts and regions | `TestRenderEnvVariablesFollowEachEnvironment` (`dev` in `000000000001`/`eu-west-1`, `pro` in `000000000003`/`us-east-1`); the module run `variables_for_two_environments` | Tested; mutation-checked in Task 4 |
| 2 | A Component without `aws.registry`, including one whose environments have `aws` blocks | `TestRenderGolden` against `basic`'s unchanged golden file; `TestRenderWithoutRegistryPassesNoVariables`; the module run `environments_have_no_variables_by_default` | Tested; the module run was mutation-checked in Task 3 |
| 3 | `aws.registry` set but an environment has no `aws` block (a model `Load` rejects) | `TestRenderRejectsARegistryEnvironmentWithoutAWS` | Tested |
| 4 | Variables for an environment the Component does not list | The module run `only_environments_with_variables_get_them`; `TestRenderEnvVariablesFollowEachEnvironment`, whose `staging` has an `aws` block and gets nothing | Tested |
| 5 | Deterministic ordering and addresses | `variables_for_two_environments` (the exact sorted key list); `TestRenderIsDeterministic` on the `aws-registry` fixture with shuffled environments, and the `aws-registry` golden file | Tested |

Also tested, outside the five: spec §5.3's values, verbatim; Renovate moves the pins of both golden files; the rendered `aws-registry` stack validates against the local modules (CI, `render-smoke`, twice `The configuration is valid.`); the carried naming cases `aBc`, `dEv` and `deV`.

## Ship (Task 6)

1. Local verification was clean before the push: 14 files changed; the lock file, the provider pins and `basic`'s golden file unchanged; the module runs 12/12.
2. The branch was pushed as PR #35. CI run 43964802143 was green, and `render-smoke` validated both fixtures on OpenTofu 1.13.1.
3. **Final whole-branch review** (most capable model): **Ready to merge: Yes**, with 0 Critical, 0 Important and 1 Minor: `internal/render/render_test.go:232`, where an inner loop variable `i` shadows the outer one.
4. **Gentle AI receipt-driven review** (the owner granted consent; risk high because of the shell in `ci.yaml`; four lenses): approved and acknowledged (lineage `review-2e1df840e2dc7f5b`), with two advisories: R2-ci-fixture-list-duplicated (readability, `.github/workflows/ci.yaml:99`) and R3-partition-silent (reliability warning, `internal/render/render.go:135-139`, which is D-8's documented behavior).
5. The ruling was to carry the Minor and both advisories into the Phase 2c plan rather than reopen the approved candidate.
6. PR #35 merged as `c6aa267`.

## Watch list

| Item | Result |
|---|---|
| After the merge, release PR #28 still proposes `0.2.0` (D-16) | Seen live: the merge's release run (43974764962) succeeded and release PR #28 still proposes "chore(main): release 0.2.0". It stays open until 2f |
| The harness run dispatched for that release PR passes (D-5) | Seen live: run 43974870260 (`workflow_dispatch` on `c6aa267`) ended with conclusion `success` |

## Notable findings caught by review

| Finding | Where caught | Disposition |
|---|---|---|
| `render_test.go:232`: an inner loop variable `i` shadows the outer one | Final review, Minor 1 | Carried into Phase 2c |
| R2-ci-fixture-list-duplicated: a fixture list duplicated in `ci.yaml` (readability, line 99) | Gentle AI review, advisory | Carried into Phase 2c |
| R3-partition-silent: the partition limit is silent (reliability warning, `render.go:135-139`) | Gentle AI review, advisory | Carried into Phase 2c; the behavior is documented in D-8 |

Neither review found a Critical or Important issue.

## Final disposition of every finding

Statuses: **Fixed** (done, with the commits), **Phase 2c** and later sub-phases, **Won't fix** (consciously rejected, with the reason). No finding is left open without a destination.

### Fixed

The five items that Phase 2a carried into this phase are all fixed:

- Phase 2a final review Minor 1 (uppercase letters after the first position): **Fixed**, Task 2 (`4fc6313`).
- Minor 2 (ADR-0013 and spec §11 risk 4 in present tense): **Fixed**, Task 1 (`005e298`), with `aud` added to §8.3.
- Minor 3 (the region pattern accepts other partitions): **Fixed** by documenting it, Task 2 (`acaf514`, D-8).
- Minor 4 (spec §3.3's units table): **Fixed**, Task 1 (`005e298`).
- R3-001 (`docs/claims.md:101`): **Fixed**, Task 4 (`2c23e0b`, D-11).

### Phase 2c

- Opus Minor: rename the inner loop variable at `internal/render/render_test.go:232`.
- R2-ci-fixture-list-duplicated: the fixture list at `.github/workflows/ci.yaml:99`.
- R3-partition-silent: the warning at `internal/render/render.go:135-139` (documented behavior, D-8).

## Carried into later sub-phases

- **2c.** The three items above. The AWS modules create exactly naming's identities and their tests assert the trust policy's `sub` and `aud` (§8.3 under A11); the baseline's lifecycle and the shared-account question (2a's D-11); the AWS lock file and the provider pin (`hashicorp/aws` 6.66.0 was proven on OpenTofu 1.12.6, and the repo pins 1.13.1); the floci ECR host form (ADR-0013).
- **2d.** Rendering the AWS stacks; the nested-stack discovery items of [Phase 1b](phase-1b.md).
- **2e.** The AWS reconcile design.
- **2f.** The harness's token to read environment variables (the bot token is scoped to the claims repo and has no environments permission), the `aws` block for `dev` in `idp-claims-e2e`'s `config/platform.yaml`, and an `aws.registry` harness claim whose variables the E2E checks.

## Owner notes

- The release PR #28, proposing `0.2.0`, stays open until 2f (D-16). Every push to `main` re-runs the harness on it while it is open (about 20 minutes each, ADR-0028). This log is a `docs` commit, so its push does that once more.
- The earlier owner items stay as recorded in the [Phase 1e log](phase-1e.md); they are not new asks.

## Process notes

- **Execution:** inline in the controller session, across a session break during which the scratchpad was cleaned and the helper scripts were recreated. Every task followed TDD with an observed RED and GREEN. Fix rounds: none.
- **Mutation checks (three, each reverted):** in Task 2, the widened naming rule made `aBc` fail; in Task 3, a module `MUTANT` made 3 runs fail; in Task 4, rendering only the first environment made 4 tests fail.
- **Local OpenTofu 1.12.6 against CI's 1.13.1:** the module runs and `render-smoke` passed on 1.13.1 in CI.
- **Naming rule:** the naming checks ran on each task's work and on this log, and were clean.
- **Ledger note:** the ledger lived in `.superpowers/`, which is excluded from git; this log is its durable copy.

## Status

**Phase 2b is complete.** A Component with `aws.registry` renders `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION` for each environment it lists, computed by `internal/naming`, and `modules/github/component` creates them when the github stack is applied. A Component without `aws.registry` renders as before. The AWS resources the variables name are not rendered or applied yet. This is shown by the green CI on PR #35, merged as `c6aa267`, by the release run 43974764962, after which the release PR #28 still proposes `0.2.0`, and by the harness run 43974870260, which ended with `success`. Every finding has a final disposition above.

Next: Phase 2c (the AWS modules and their floci tests).
