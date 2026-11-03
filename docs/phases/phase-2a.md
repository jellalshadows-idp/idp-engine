# Phase 2a execution log: the AWS model of the Platform and the Component

- Period: 2026-11-03
- Plan: [2026-11-03-phase-2a-aws-model.md](../superpowers/plans/2026-11-03-phase-2a-aws-model.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md), amended by A10
- Decisions: [ADR-0013](../adr/0013-phase-0-spike-findings.md), amended (floci only; no real `AssumeRoleWithWebIdentity` test in v1)
- Previous phase: [Phase 1e log](phase-1e.md)
- Status: **complete**. The phase merged through [PR #29](https://github.com/jellalshadows-idp/idp-engine/pull/29) (merge commit `fca5c0b`). It ships no release: the release PR [#28](https://github.com/jellalshadows-idp/idp-engine/pull/28) "chore(main): release 0.2.0" stays open until Phase 2f.

This file is the durable copy of the execution ledger that was kept while the plan ran. The ledger lived in a git-ignored scratch folder that is deleted afterwards, so what it recorded is reproduced here: the rulings, each task's outcome, the reviews, the live results, the disposition of every finding and what is carried forward.

## Summary

Phase 2a gives the claims model its AWS half without rendering any of it. Phase 2 as a whole is split into six plans (2a to 2f, D-1); this is the first.

- **`internal/naming`** computes the deterministic AWS identities of spec §5.5 from plain values: `RepositoryName`, `RoleName`, `RoleARN`, `RegistryURL`, `OIDCProviderARN` and the constant `OIDCProviderHost`. It imports nothing from the engine and returns no errors. Its tests read the name patterns from the embedded schemas and prove that the longest legal names fit IAM's and ECR's rules.
- **The Platform model** reads each environment's `aws` block (`accountId`, `region`): `Environment.AWS *AWS`, nil when the environment has no block. The account ID stays a string; the schema rejects an unquoted ID at its line.
- **The Component** accepts `aws: {registry: boolean}`. `features` is still rejected.
- **Validation** (spec §4.7 under A10) requires a registry Component to list at least one environment and each of its environments to have an `aws` block in `config/platform.yaml`.
- **The renderer is untouched.** `TestRenderIgnoresTheAWSPart` requires that an `aws` claim leaves the GitHub render equal to `basic`'s golden file, byte for byte, and that only the github stack is rendered. The test was mutation-checked.
- No workflow, action or module changed. No cloud credential or resource was created.

How it was executed: inline in the controller session, with no per-task subagents (the plan's Task 7 and the ODD routing allowed it: Go only, and the schemas gain only optional fields). Every task followed TDD with an observed RED and GREEN. A whole-branch review by the most capable model followed the push, and a Gentle AI receipt-driven review ran on the candidate.

CI evidence on `idp-engine`: PR #29 had six green checks (`go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `render-smoke`, `release-snapshot`) before the merge.

## Tasks

Commit ranges are on `feat/phase-2a-aws-model`, created from `main` at `1ccc1d7` (the plan commit). Each task is one commit.

| Task | What | Commit | Outcome |
|---|---|---|---|
| 1 | Spec amendment A10, ADR-0013 amended, the claims reference | `2e7d710` | Complete: 15 exact replacements applied from the plan; anchors resolve; A10 appears 6 times; ADR-0013 amended twice; the stale-text and naming checks print nothing |
| 2 | `internal/naming` | `4a3616a` | Complete. RED: undefined `RoleName`, `RegistryURL`, `RepositoryName` and the rest. GREEN: 20 subtests. Gates clean |
| 3 | The Platform model's `aws` block | `8fc2e11` | Complete. The boundary pins passed before the change. RED: `Environment` has no field `AWS`. GREEN. Gates clean |
| 4 | The Component schema and model accept `aws.registry` | `85b655d` | Complete. RED: 6 expected schema and validate failures, then `Component` has no field `AWS`. GREEN. Gates clean |
| 5 | The semantic check for registry Components | `c6f416d` | Complete. RED: exactly the 4 expected subtests. GREEN. Gates clean |
| 6 | The render guard | `52f48cd` | Complete. RED: the fixture was missing. GREEN. Mutation check: adding `aws_registry` to the component module made the guard fail; reverted. Gates clean |
| 7 | Ship: verification, PR #29, CI, reviews, merge, release PR, this log and the README | PR #29, merge `fca5c0b`; this commit | See "Ship" |

No task had a separate reviewer; the reviews are the two whole-branch reviews below. Task details:

- **Task 1.** The amendment A10 covers spec §3.3, §4.7, §6, §10 and §11 risk 4. ADR-0013 records that a real `sts:AssumeRoleWithWebIdentity` test is out of scope for v1 (D-2). `docs/claims.md` documents the `aws` fields, says to quote the account ID and gives the new diagnostics.
- **Task 2.** The tests read the component and environment name patterns from `schemas/*.json` instead of copying them (`TestNamesFitAWSLimits`, `TestSchemaNameRulesKeepAWSNamesValid`) and check the spec's identity examples verbatim.
- **Task 3.** Pins the account ID and region boundaries of the platform schema, the rejection of an emulator endpoint key and the rejection of an unquoted account ID.
- **Task 4.** The component schema takes `aws` with one optional boolean `registry` and `additionalProperties: false`; `features` stays rejected, pinned in each schema suite.
- **Task 5.** The two diagnostics of D-8 in `internal/claims/load.go`. An environment missing from `config/platform.yaml` reports only "is not defined".
- **Task 6.** The fixture `internal/render/testdata/aws-registry/` is `basic` plus `aws.registry: true` and the environments' `aws` blocks.

## Rulings

A ruling is a controller or owner decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong. D-1 and D-2 are the owner's (2026-11-03); D-3 to D-15 were decided while planning, from verified facts, and the owner approved the plan, its side effects and inline execution.

- **D-1 (owner). Phase 2 is split** into 2a to 2f: 2a the AWS model; 2b the deterministic env variables in `modules/github/component`; 2c `modules/aws/{baseline,component}`, the AWS lock file and the floci CI job; 2d rendering the AWS stacks; 2e the workflows' AWS plan and apply against floci; 2f the E2E env-variable checks, the `0.2.0` release and the Phase 2 log. Each plan is written after the previous one ships. 2a ships no release. *Cost if wrong:* six review cycles instead of fewer, each of them smaller.
- **D-2 (owner). floci only, no real AWS account** (spec §2.2, §7.6). A real `sts:AssumeRoleWithWebIdentity`, which ADR-0013 left to the Phase 2 floci tests, is out of scope for v1; Task 1 recorded that in A10 (§11, risk 4) and in ADR-0013. *Cost if wrong:* going live needs an account and the OIDC credentials step that spec §5.6 describes; claims and renders do not change.
- **D-3. No `internal/config`.** Phase 1a's ruling R-3.3 allows the extraction when `internal/naming` needs the Platform without the claims. It does not: naming takes plain strings, so it imports neither `internal/claims` nor a config package. A10 replaces `internal/config` in spec §3.3's dependency column. *Cost if wrong:* a later sub-phase imports `internal/claims` (A4 allows it) or extracts the package then.
- **D-4. The scope of `internal/naming` is exactly the AWS identities of spec §4.4 and §5.5,** in the `aws` partition and with the `amazonaws.com` host as the spec writes them. Not in 2a: the trust policy's subject and audience (used only by the AWS module, asserted by its `tofu test` in 2c), and Terraform addresses and AWS stack paths (they move there when 2d renders the AWS stacks). *Cost if wrong:* a few functions added in 2c or 2d.
- **D-5. AWS's name limits are guaranteed by the schemas and proven by naming's tests, not checked at run time.** IAM `RoleName` is 1 to 64 characters and ECR `repositoryName` is 2 to 256 lowercase characters; component and environment names are lowercase, so every legal name is valid for both, and the role name is at most 54 characters (spec §5.5). *Cost if wrong:* none at run time; a schema change that breaks the bound fails `TestNamesFitAWSLimits` first.
- **D-6. The Platform model:** `Environment.AWS *AWS` and `AWS{AccountID, Region string}`; nil means no `aws` block. The account ID stays a string, which keeps its leading zeros; an unquoted ID is a YAML number and the existing schema rejects it at its line, which Task 3 pins and `docs/claims.md` documents. *Cost if wrong:* none.
- **D-7. The Component schema and model:** `aws` is an object with one optional boolean `registry` (default `false`) and `additionalProperties: false`; the model is an anonymous `AWS struct { Registry bool }`. `aws: {}` and `registry: false` are valid and ask for nothing. *Cost if wrong:* rejecting `aws: {}` later would break claims that use it.
- **D-8. The semantic check and its wording** (A10 adds the rule to spec §4.7):
  - `environment "<env>" needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true`, one per such environment, at that environment's line in `environments`;
  - `aws.registry needs at least one environment: the registry and its CI role are created per environment`, at the `aws.registry` line.

  An environment missing from `config/platform.yaml` reports only "is not defined". The per-environment check runs only when the platform config decoded; the empty-list check needs no platform config. *Cost if wrong:* wording only.
- **D-9. `aws.registry: true` with no environments is an error,** not a silent no-op: every AWS resource of the part is per environment, so such a claim asks for a registry and would get none. Starting strict is the reversible choice. *Cost if wrong:* a claim lists an environment before it turns the registry on.
- **D-10. The renderer stays GitHub-only.** `TestRenderIgnoresTheAWSPart` pins that interim contract; 2b and 2d change it on purpose. *Cost if wrong:* none.
- **D-11. Not validated in 2a: two environments that share an `accountId`.** With floci every job runs its own emulator, so nothing collides in v1; on real AWS the per-account OIDC provider and the repository would. 2c, which decides the baseline's lifecycle, rules on it. *Cost if wrong:* rejecting it later breaks a config that shares an account; none does.
- **D-12. No new ADR.** ADR-0006 and ADR-0015 already decide 2a; A10 records the rule and ADR-0013 gets the amended sentences for D-2. *Cost if wrong:* one ADR later.
- **D-13. Amendment A10 covers what 2a makes stale or adds, and nothing else:** §3.3, §4.7, §6, §10 and §11 risk 4. §4.4, §4.6 and §5.5 were not stale; `docs/pipelines.md` stays true until 2e. *Cost if wrong:* none.
- **D-14. Phase 1a's Task 2 minor, AWS part.** The platform schema's account ID and region boundaries got tests in Task 3 and name length and case are pinned through naming's tests (Task 2). These tests pin existing schema behavior. *Cost if wrong:* none.
- **D-15. No release in 2a.** After the merge, release-please was expected to propose `0.2.0` and to stay open until 2f, because a release in between would accept `aws.registry: true` claims that render nothing. *Cost if wrong:* if it proposed another version the rule is the same: do not merge it. Confirmed live (see "Watch list").

Execution rulings:

- **Execution inline.** The controller ran every task itself, with no per-task subagents. *Cost if wrong:* no separate reviewer per task; the whole-branch reviews cover the branch.
- **Carry the four Minors and R3-001 into Phase 2b** instead of reopening the approved candidate before the merge. *Cost if wrong:* four small items wait one sub-phase; none affects v1 behavior.

## Review Focus

| # | Case | Pinned by | Result |
|---|---|---|---|
| 1 | A registry Component targets an environment without `aws` | `TestLoadDiagnostics` cases "aws.registry needs an aws block in every environment" and "aws.registry checks each environment" | Tested |
| 2 | An `aws` block with `registry: false`, or an empty block | `TestLoadDiagnostics` cases "registry false needs no aws block" and "an empty aws block needs no aws block"; `TestSchemas` rows "registry false" and "empty aws block" | Tested |
| 3 | Names beyond AWS's length limits or with uppercase letters | `TestNamesFitAWSLimits` and `TestSchemaNameRulesKeepAWSNamesValid` | Tested |
| 4 | `aws.registry: true` with no environments | `TestLoadDiagnostics` cases "aws.registry needs an environment" and "aws.registry needs a non-empty environment list" | Tested |
| 5 | The GitHub render with an `aws` claim present | `TestRenderIgnoresTheAWSPart` | Tested; mutation-checked in Task 6 |

Also tested, outside the five: an environment of a registry Component that is missing from `config/platform.yaml` reports only "is not defined"; an unquoted account ID is rejected at its line; the account ID and region boundaries and the rejection of an emulator endpoint key; `features` still rejected; the spec's identity examples, verbatim.

## Ship (Task 7)

1. Local verification was clean before the push.
2. The branch was pushed as PR #29. CI was green with six checks.
3. **Final whole-branch review** (most capable model): **Ready to merge: Yes**, with 4 Minor findings and no Critical or Important ones.
4. **Gentle AI receipt-driven review** (the owner granted consent; lens: reliability): approved and acknowledged (lineage `review-c2900c96235ae543`), with one informational suggestion, R3-001 at `docs/claims.md:101`.
5. The ruling was to carry the 4 Minors and R3-001 into Phase 2b rather than reopen the approved candidate before the merge.
6. PR #29 merged as `fca5c0b`.

## Watch list

| Item | Result |
|---|---|
| After the merge, release-please's release PR proposes `0.2.0` (D-15) | Seen live: the merge's release run (41626704730) succeeded and release-please opened release PR #28 "chore(main): release 0.2.0". It stays open until 2f |
| The harness run dispatched for that release PR passes; its claims carry no `aws` block, so the schema change must not affect it | Seen live: run 41626769073 (`workflow_dispatch` on `fca5c0b`) ended with conclusion `success`. It waited in a queue behind an earlier harness run before it started |

## Notable findings caught by review

| Finding | Where caught | Disposition |
|---|---|---|
| `naming_test` tests an uppercase letter only in the first position | Final review, Minor 1 | Carried into Phase 2b |
| ADR-0013 (around line 120) and spec §11 risk 4 state in present tense that module tests assert the trust policy's `sub` and `aud`; those tests arrive in 2c, and spec §8.3 lists only `sub` | Final review, Minor 2 | Carried into Phase 2b |
| The platform region pattern accepts GovCloud and China regions, while naming hardcodes the `aws` partition and `amazonaws.com` | Final review, Minor 3 | Carried into Phase 2b |
| Spec §3.3's units table still lists `internal/config` and `internal/claims` depending on it | Final review, Minor 4 | Carried into Phase 2b |
| R3-001 at `docs/claims.md:101` | Gentle AI review, informational | Carried into Phase 2b |

Neither review found a Critical or Important issue.

## Final disposition of every finding

Statuses: **Fixed** (done, with the commits), **Phase 2b** and later sub-phases, **Won't fix** (consciously rejected, with the reason), **Owner note**. No finding is left open without a destination.

### Fixed

- Phase 1a Task 2 minor, its AWS account ID and region part: **Fixed**, Task 3 (`8fc2e11`).
- Phase 1a Task 2 minor, its name length and case part: **Fixed**, Task 2 (`4a3616a`). The rest of that minor is unchanged.

### Phase 2b

- Final review Minor 1: add `aBc` and `dEv`/`deV` cases to the `naming_test` uppercase check.
- Final review Minor 2: reword ADR-0013 (around line 120) and spec §11 risk 4 so they no longer state in present tense that module tests assert the trust policy's `sub` and `aud` (they arrive in 2c; §8.3 lists only `sub`).
- Final review Minor 3: document or narrow the platform region pattern, which accepts GovCloud and China regions while naming hardcodes the `aws` partition and `amazonaws.com`.
- Final review Minor 4: update spec §3.3's units table, which still lists `internal/config` and `internal/claims` depending on it.
- R3-001 (`docs/claims.md:101`, informational): review it in the same plan.

### Won't fix

- Phase 1a's "Phase 2 candidate" to extract `internal/config`: **Won't fix** (D-3: naming takes plain values).
- ADR-0013's real `AssumeRoleWithWebIdentity` test: **Won't fix**, out of scope for v1 (D-2).

### Owner note

- See "Owner notes".

## Carried into Phase 2b

- The `variables` input and `github_actions_environment_variable` in `modules/github/component`, rendered from `internal/naming`. `TestRenderIgnoresTheAWSPart` becomes the env-variables golden.
- The four Minors and R3-001 above.

## Carried into later sub-phases

- **2c.** The baseline's lifecycle and the shared-account question (D-11).
- **2c and 2d.** The AWS stacks' backend, encryption, lock file and provider pin (`hashicorp/aws` 6.66.0 was proven on OpenTofu 1.12.6, and the repo now pins 1.13.1).
- **2d.** The nested-stack discovery items of [Phase 1b](phase-1b.md).
- **2e.** The AWS reconcile design: plan and apply matrices, required check names and wet paths.
- **2c and 2f.** The harness's token to read environment variables, the `aws` block for `dev` in `idp-claims-e2e`'s `config/platform.yaml`, and the floci ECR host form (ADR-0013).

## Owner notes

- The release PR #28, proposing `0.2.0`, stays open until 2f (D-15). Every push to `main` re-runs the harness on it while it is open (about 20 minutes each, ADR-0028). This log is a `docs` commit, so its push does that once more.
- The earlier owner items stay as recorded in the [Phase 1e log](phase-1e.md); they are not new asks.

## Process notes

- **Execution:** inline in the controller session; every task followed TDD with an observed RED and GREEN, and Task 6's mutation check is recorded above. Fix rounds: none.
- **Naming rule:** the naming checks ran on each task's work and on this log, and were clean.
- **Local verification constraint:** no workflow, action or module changed, so there was no local actionlint, zizmor or `tofu test`; CI ran them.
- **Ledger note:** the ledger lived in `.superpowers/`, which is excluded from git; this log is its durable copy.

## Status

**Phase 2a is complete.** Claims accept a Component's `aws.registry` and each environment's AWS account and region, validation requires an `aws` block for every environment of a registry Component, and `internal/naming` computes the AWS identities. Nothing AWS is rendered or applied yet. This is shown by the six green checks on PR #29, merged as `fca5c0b`, and by the release run 41626704730, which opened the release PR #28 for `0.2.0`. Every finding has a final disposition above.

Next: Phase 2b (the deterministic env variables).
