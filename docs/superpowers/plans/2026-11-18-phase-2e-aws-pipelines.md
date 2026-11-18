# Phase 2e — The Pipelines' AWS Plan and Apply Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** the reusable workflows plan and apply the rendered AWS stacks against floci (spec §6.1 step 4, §6.2 steps 1 to 4).
- `pr.yaml` plans every affected AWS stack in a `plan-aws` job of its own, and the sticky comment summarizes them with the `github` stack.
- `reconcile.yaml` plans them the same way, gates every stack's plan in a secret-less `gate` job, applies each in an `apply-aws` job that re-plans and refuses a plan not equal to the gated one, and commits a stack's render to `wet` only after it applied. An orphan is destroyed with its old configuration from `wet`, and its render leaves `wet` only after the destroy applied.
- A job that runs floci holds no secret (ADR-0031, new); the AWS state never leaves its job (ADR-0030).

The `github` stack's plan and apply are unchanged apart from the restructuring. `drift` stays GitHub-only. No Workspaces, no features, no release.

**Architecture:**
- **`internal/awsstack`** (new) holds the per-stack sequence: `Plan(cfg, runner)` stages the environment's baseline and the stack's `wet` version in a scratch copy laid out like a render, applies them, swaps in the new version against the same state and plans it, or plans an orphan's destroy with its `wet` configuration; `Apply(cfg, runner, gated)` re-plans and requires equality with the gated fingerprint before it applies the saved plan. `WriteOverride` moves here from the floci tests, so CI and the pipelines write one override.
- **New `idp` commands:** `stacks` (the allowlist guard, the AWS matrix and each stack's account), `plan-reduce` (a plan's addresses and actions only), `aws-stack plan|apply` (one stack against floci, refusing any secret) and `wet-root` (the wet commit of a reconcile). `plan-summary` reads reduced plans with `--plan-dir`; `wet-push` refuses any state file but `tfstate/github.tfstate`.
- **`internal/floci`** gains `TestReplay`, which CI's floci job runs: the rehearsal of `plan-aws` and `apply-aws` through `awsstack` (new, changed, unchanged, orphan, refusal).
- **Workflows:** `pr.yaml` gets `plan-aws` and a `comment` job that summarizes; `reconcile.yaml` gets `plan-aws`, `gate`, `apply-aws`, and an `apply-github` that commits what applied; `recover.yaml` reads the path list. `internal/workflowspec` pins all of it, including a rule that every job that runs floci holds no secret.

**Tech Stack:** Go 1.26 (standard library; no new dependency). No new pin: the floci image and the actions are the ones `ci.yaml` and the workflows already pin. Local OpenTofu 1.12.6; CI's 1.13.2 is authoritative.

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §5.4–§5.6, §6 (intro, §6.1, §6.2, §6.4), §7.2, §7.5, §8.4, §10, §11 and amendments A10–A13. This plan adds A14.

Context:
- the phase logs `docs/phases/phase-2d.md` ("Carried into later sub-phases", 2e) and `docs/phases/phase-2c.md`;
- ADR-0004, ADR-0018, ADR-0019, ADR-0021 to ADR-0025 and ADR-0030;
- `docs/pipelines.md`, `docs/cli.md`, `docs/runbooks/pipelines.md`.

## Phase 2 split (owner decision, 2026-11-03)

Phase 2 is delivered as six plans, each written after the previous one ships (Phase 2a ruling D-1, spec §10 under A10).

| Plan | Delivers |
|---|---|
| 2a (shipped, PR #29, `fca5c0b`) | The AWS model of the Platform and the Component, `internal/naming`, the schema and semantic validation, spec amendment A10 |
| 2b (shipped, PR #35, `c6aa267`) | The deterministic env variables in `modules/github/component`, rendered from `internal/naming`; spec amendment A11 |
| 2c (shipped, PR #37, `368e5a3`) | `modules/aws/{baseline,component}`, the AWS lock file and the floci CI job; one account per environment; spec amendment A12 |
| 2d (shipped, `19e65bd`) | Rendering the AWS stacks; `render.AWSProviderVersion`; level 3 on the real render; spec amendment A13, ADR-0030 |
| **2e (this plan)** | The workflows' AWS plan and apply against floci: `idp stacks`, `plan-reduce`, `aws-stack`, `wet-root`; `internal/awsstack` and its floci rehearsal; `plan-aws`, `gate`, `apply-aws`; spec amendment A14, ADR-0031 |
| 2f | The E2E env-variable checks and the live AWS end-to-end run, the `0.2.0` release and the Phase 2 log |

## Decisions taken while planning

The executor copies these into the ledger as rulings before Task 1. Phase 2a's owner decisions still hold: Phase 2 is split (2a D-1), and v1 is floci only, with no real AWS account (2a D-2).
- D-1 to D-13 record the controller's thirteen rulings and what was decided under them.
- D-14 to D-18 were decided while planning, from the real code and from facts read live on 2026-11-18.
- No ruling conflicts with the spec or the code. Where the spec placed work in a job that would now run floci beside secrets (the gate inside `plan`, spec §6.2 step 1), A14 records the split instead of changing the rule (D-2).

- **D-1. Scope (ruling 1).**
  - The PR plans every affected AWS stack; the reconcile plans, gates and applies them. An orphan is planned and applied as a destroy with its **old** configuration from `wet` (spec §6.1 step 4.3).
  - `apply-github` commits `rendered/` only for the AWS stacks whose `apply-aws` job succeeded, and deletes an orphan's render only after its destroy applied (`idp wet-root`, Task 8). A failed stack keeps its old render, so the next reconcile retries it (spec §6.2 step 4).
  - The `github` stack's plan, apply and state are unchanged apart from the restructuring: its plan moves its summary and gate to secret-less jobs (D-2), and its full plan never leaves its job (D-6).
  - No Workspaces (`aws/<env>/workspaces/<name>`, Phase 3), no `observe`, no features (Phase 4).

  *Cost if wrong:* committing the renders of failed stacks too would hide failures from the next reconcile; reverting is one condition in `idp wet-root` and its test.
- **D-2. No secret in a job that runs floci (ruling 2; ADR-0031, new).**
  - floci mounts the runner's Docker socket (ADR-0004), so `plan-aws` and `apply-aws` hold no `secrets.*`, no App token, no environment and no `TF_ENCRYPTION`; `GITHUB_TOKEN` gets `contents: read` and is never put in a step's environment. `idp aws-stack` refuses to run when `TF_ENCRYPTION` or `IDP_STATE_PASSPHRASE` is set.
  - Jobs:
    - PR: `validate`, `plan` (the `github` stack, with the reader key and the passphrase), the `plan-aws` matrix, `comment` (secret-less: `plan-summary` over the reduced plans, then the comment), `verdict`;
    - reconcile: `plan` (the `github` stack, in `idp-write`), the `plan-aws` matrix, `gate` (secret-less: `plan-summary`, then `idp gate`), `approve`, the `apply-aws` matrix, `apply-github` (in `idp-write`).
  - `verdict`, mirrored by the caller's `idp-gate`, stays the only required check (ADR-0022); `approve` stays one secret-less job (spec §6.2 step 2). The harness's job names, `reconcile / apply-github` and `reconcile / approve`, are unchanged.
  - Residual risk, recorded in ADR-0031: a floci job still holds its run's runtime token, so a misbehaving floci image could upload a forged artifact. The image is pinned by digest, and no job restores a cache.

  *Cost if wrong:* a job that needs a secret beside floci would have to split again; the rule is one test (`TestFlociJobsHoldNoSecret`) and one CLI check.
- **D-3. The per-stack sequence (ruling 3).**
  - **The baseline first:** a Component stack's job applies its environment's `_baseline` before anything else: **`wet`'s version**, because the applied Component stacks were planned against it, or the new render's when `wet` has none yet (a new environment). A baseline stack needs nothing first.
  - **Then the stack's `wet` version**, when `wet` has one, applied; **then** the new version planned against the state that replay wrote, or, for an orphan, a destroy planned with the `wet` configuration.
  - **One state (ADR-0030):** the new version is planned in the same scratch directory, with the configuration swapped: everything but `.terraform`, `terraform.tfstate` and `terraform.tfstate.backup` is removed, the new render is copied in, and `init -lockfile=readonly` runs again. Verified while planning with OpenTofu 1.12.6, offline: a swapped configuration plans `update` against the replayed state (not `create`), an `init` after the swap accepts a changed module source, and a `-destroy` plan of the old configuration deletes every resource.
  - **Where:** in `--work/rendered/<stack>`, a scratch directory outside both renders, laid out like a render so the rendered relative module sources resolve (the floci test's `../../modules`). The renders are never written; `awsstack` refuses a `--work` inside either.
  - **The reconcile apply:** replay, re-plan, require the stack's changes to **equal** the gated fingerprint's, both ways, then apply the saved plan and write the evidence to the job summary. A stack the gated fingerprint does not name applies nothing.
  - **The replays** apply with `-auto-approve`: they only rebuild floci's state. The final apply uses the saved plan.
  - **The override code** lives in `internal/awsstack` (`WriteOverride`), used by `idp aws-stack` and by the floci tests; `internal/floci`'s copy goes.

  *Cost if wrong:* replaying the new baseline instead of `wet`'s changes one loop's order (`baselineSource`) and one test line.
- **D-4. Testable (ruling 4).** The sequence is Go code (`internal/awsstack`, Task 5), driven by a new subcommand, `idp aws-stack plan|apply` (Task 6, documented in `docs/cli.md`). Unit tests drive it with a fake runner that records which configuration ran against which state. CI's floci job rehearses it against floci (`TestReplay`, Task 7): a new stack, a changed one (planned against its replayed state, applied), an unchanged one, an orphan destroyed with its `wet` configuration, and the refusal of a plan that differs from the gated one. `ci.yaml` needs no change: its floci job already runs `go test ./internal/floci/`.

  *Cost if wrong:* none in production; a rehearsal that misses a floci gap shows in 2f's live run.
- **D-5. The plan pipeline (ruling 5).**
  - The AWS stacks join `plan-summary`, the comment, the marker and the gate through reduced plans (D-6) and `plan-summary --plan-dir`. Slash paths are pinned in the marker and the comment table (`TestPlanSummaryReadsAPlanDir`, Task 4).
  - The `jq` guard becomes an allowlist in `idp stacks` (Task 3): `github`, `aws/<env>/_baseline` and `aws/<env>/components/<name>`, with the claims schemas' name patterns. Anything else fails with `idp stacks: this engine version reconciles only github, aws/<env>/_baseline and aws/<env>/components/<name>; also affected: ...` until Phase 3.

  *Cost if wrong:* the allowlist is one regular expression in `internal/wetdiff` and its table test.
- **D-6. Artifacts (ruling 6).**
  - One escaping, `wetdiff.Escape`: each `/` becomes `.`. No reconcilable stack name holds a dot, so two stacks never share an escaped name, and the result holds none of the characters artifact names refuse.
  - Names: `idp-plan-<stack>`, `idp-renders`, `idp-gated`, `idp-applied-<stack>`, plus the existing `idp-reconcile` and `idp-wet-root`. `idp-pr-comment` goes: the comment job now writes the comment.
  - Retention: 1 day for every plan, render, fingerprint and marker artifact; `idp-wet-root` keeps its 7 days.
  - **Only reduced plans upload**, the `github` stack's included: `tofu show -json` holds every value a plan sets in plaintext, and artifacts of public repos are public. A reduced plan holds the format version and each change's address and actions, exactly what `plan.Parse` reads (`plan.Reduce`, Task 4).

  *Cost if wrong:* if a later reader needs more of the plan, `reduced` gains fields; nothing stored depends on its shape (1-day artifacts).
- **D-7. The tfstate guard (ruling 7).** `idp wet-push` refuses any file named `*.tfstate`, or a copy such as `terraform.tfstate.backup`, except `tfstate/github.tfstate`, before any write (Task 2). `idp diff` does not refuse: it reads renders, which never hold state because OpenTofu runs only in scratch copies (D-3), and `wet-push` is the one place anything reaches `wet`.

  *Cost if wrong:* a state file in a render would show as a changed stack in `idp diff` until removed; the push still refuses it.
- **D-8. Drift stays GitHub-only (ruling 8; A14 in §6.4).** An AWS stack's state lives only in its job's floci (ADR-0030), so there is nothing to compare. `recover` works with AWS paths: it pushes the paths the reconcile listed in `wet-paths`, and falls back to the `github` paths for a wet root saved before `wet-paths` existed (Task 12).

  *Cost if wrong:* none until a real account; an AWS drift check then needs a persistent backend first (ADR-0030).
- **D-9. Time, matrix and limits (ruling 9).**
  - **Granularity: one job per stack** (spec §6.1 step 4, "one floci per job"): stacks are independent (§5.5), each leg gets its own evidence, and `fail-fast: false` lets every other stack finish. `max-parallel: 10`.
  - **Limits:** `idp stacks` refuses more than 256 AWS stacks, the jobs one matrix runs (spec §11 risk 3). An empty matrix fails a run, so `plan-aws` and `apply-aws` skip on `needs.plan.outputs.aws == '[]'`: a PR without AWS stacks starts no floci.
  - **Measured on 2026-11-18:** CI's floci job on `19e65bd` took 2m34s (test step 2m05s, container start 8 s); the harness's runs took `pr / validate` 32 s, `pr / plan` 45 s, `pr / comment` 34 s, `reconcile / plan` 40 s, `reconcile / apply-github` 66 s, and `setup-idp` 20 to 27 s per job. Spec §11 risk 3 measured about 26 s of OpenTofu per stack (init 7, apply 13, plan 6).
  - **Forecast:** a `plan-aws` leg of a Component stack about 1.5 min (setup about 25 s, floci about 8 s, two replays about 40 s, swap, plan and show about 10 s); an `apply-aws` leg about 1.7 min. A PR that changes one registry Component: about 3.7 min end to end (about 2 min today). A reconcile of it: about 6 min (about 2 min today). Up to ten stacks run in about the same wall time; billed minutes grow by about 3.5 min per AWS stack and reconcile. The watch list records the real numbers.

  *Cost if wrong:* per-environment legs would cut setup time for many stacks; the matrix is built by `idp stacks`, so the change is its output and two job blocks.
- **D-10. Live proof (ruling 10).**
  - The harness runs `main`'s head on release PR #28 at every push, so each merge must keep the `github` path green (watch list). The harness's claims repo has no `aws` block, so its runs skip every AWS job.
  - Verified live on 2026-11-18: `idp-claims`'s four callers pin `@150b7d35dc03831046d732a642e5ff03e33ce8bf # v0.1.1`, and the tag `v0.1.1` is that commit. `idp-claims` holds three environments with `aws` blocks (dev `000000000001`, staging `000000000002`, pro `000000000003`) and no claims; its `wet` holds only `rendered/github`. So no live repo reaches this phase's AWS jobs until 2f re-pins it.
  - **No live AWS smoke run is proposed.** It would need a change to `idp-claims` or `idp-claims-e2e` (not authorized), and CI's rehearsal (D-4) runs the same code against the same floci image. The live end-to-end AWS run is 2f's.

  *Cost if wrong:* a workflow-level fault in the AWS jobs (an expression, an artifact name) shows first in 2f's live run; `internal/workflowspec` pins every such value offline.
- **D-11. Docs (ruling 11).** Spec amendment A14 (§6 intro, §6.1 step 4, §6.2 steps 1, 3 and 4, §6.4, §11 risk 3) and ADR-0031 in Task 1; `docs/cli.md` in Task 9; `docs/pipelines.md` and the runbook (a failed AWS apply, recover with AWS paths) in Task 13; the README at ship.
- **D-12. Delivery (ruling 12).** Three PRs, each green and shippable on its own, each cut from `main` after the previous one merged, on the branch `feat/phase-2e-aws-pipelines` (re-created each time; `--delete-branch` removes it at each merge). Forecast authored lines, measured on the validated scratch branch:
  - **PR 1, Tasks 1 to 4** (decisions, the state guard, `idp stacks`, reduced plans): about 820 lines. Nothing calls the new commands yet.
  - **PR 2, Tasks 5 to 9** (`internal/awsstack`, `idp aws-stack`, the floci rehearsal, `idp wet-root`, `docs/cli.md`): about 1,770 lines, 1,010 of them tests. If review asks for less, cut after Task 7 (about 1,350, then about 425).
  - **PR 3, Tasks 10 to 13** (the three workflows, their tests and docs): about 900 lines. It is the only PR that changes what the pipelines run.

  No release: 0.2.0 ships in 2f.

  *Cost if wrong:* a single PR instead is the same commits; the cut points only decide when review happens.
- **D-13. Renovate (ruling 13).** The weekly run of Monday 2026-11-23, 04:17 UTC, is on the watch list; nothing is dispatched or merged. The floci image is now pinned in three files (`ci.yaml`, `pr.yaml`, `reconcile.yaml`), and `TestFlociJobsHoldNoSecret` requires them to be equal, so a bump that misses one fails the `go` job.
- **D-14. The account of each AWS job (from the code).** floci keys an account by the access key ID (ADR-0004), and the Component stack's render names its account in the OIDC provider ARN, so each job runs as its environment's account: from the claims' platform config; for an environment the claims removed, from the ARN in the `wet` render of one of its Component stacks; and `000000000000` for an orphan baseline with no Component stack, whose render names no account (`idp stacks`, Task 3).

  *Cost if wrong:* a wrong account plans a create instead of the real change; `TestStacksCommand` pins each source.
- **D-15. `apply-github` runs after every `apply-aws` job (spec §6.2 step 4).** It collects the applied-stack markers, so it waits for the AWS legs, whatever they concluded (`always()`). The `github` apply therefore starts about 1.7 min later than today. In floci, AWS legs that run in parallel cannot conflict: each has its own emulator. Ordering between stacks in a real account is a later phase's concern.

  *Cost if wrong:* splitting the wet commit into its own job would need the `github` state and the passphrase there too.
- **D-16. The wet commit's path list.** `idp wet-root` (Task 8) writes `wet-paths`, and the push and `recover` read it, instead of each workflow rebuilding the list in shell. A wet root with nothing to commit (every AWS leg failed and no `github` stack) pushes nothing and opens no issue.

  *Cost if wrong:* none stored: the artifact lives 7 days.
- **D-17. The `github` plan's artifacts.** `reconcile / plan` uploads `idp-reconcile` (the encrypted saved plan and the render) whenever any stack is affected, since `apply-github` commits the AWS renders from it too. The `github` plan JSON stays in the job; only its reduced form uploads.
- **D-18. Local tools (from the real setup).** OpenTofu 1.12.6 at `~/.idp/tools/tofu-1.12.6/tofu.exe` runs only from a copy in a `mktemp -d` directory; CI's 1.13.2 is authoritative. There is no local Docker: `TestReplay`, actionlint, zizmor and shellcheck run only in CI, so every workflow value goes through `env:`.

## Global Constraints

**Names** (copy verbatim)
- Branch `feat/phase-2e-aws-pipelines`, once per PR (D-12); each PR merged with a merge commit.
- `internal/wetpush`: `githubState`, `isState`; test `TestSyncCommitsNoStateButGitHubs`.
- `internal/wetdiff`: `Reconcilable(stack)`, `Escape(stack)`; tests `TestReconcilable`, `TestEscape`.
- `internal/plan`: `Reduce(stack, r)`, the types `reduced` and `reducedChange`; tests `TestReduceKeepsOnlyWhatTheFingerprintReads`, `TestReduceFormat`, `TestReduceRejectsWhatParseRejects`.
- `internal/awsstack`:
  - code: `Runner`, `Config{Stack, New, Wet, Work, Endpoint}`, `Result{Plan, Reduced}`, `Plan(cfg, run)`, `Apply(cfg, run, gated)`, `WriteOverride(dir, endpoint)`, `Exec{Path, Env, Log}`, `Env(base, account)`, `fakeSecret`;
  - tests: `TestPlanANewComponent`, `TestPlanAChangedComponentReplaysWet`, `TestPlanAnOrphanDestroysWithItsWetConfig`, `TestPlanABaseline`, `TestApplyAppliesTheGatedPlan`, `TestApplyRefusesAPlanThatDiffersFromTheGatedOne`, `TestApplyRefusesAStackTheGateNeverSaw`, `TestPlanRefuses`, `TestPlanKeepsTheRenderUntouched`, `TestExec`, `TestEnv`, `TestWriteOverride`, `TestMain` (`IDP_AWSSTACK_FAKE_TOFU`).
- `internal/cli`:
  - commands `stacks`, `plan-reduce`, `aws-stack` (`plan`, `apply`), `wet-root`; `plan-summary --plan-dir`;
  - code: `runStacks`, `awsJob`, `maxMatrix`, `placeholderAccount`, `wetAccount`, `runPlanReduce`, `planFileIn`, `countsLine`, `runAWSStack`, `newTofu`, `runWetRoot`, `readMarkers`;
  - tests: `TestStacksCommand`, `TestStacksPlaceholderAccount`, `TestStacksWithoutAWS`, `TestStacksRefusesWhatThisVersionDoesNotReconcile`, `TestStacksRefusesMoreThanAMatrixRuns`, `TestStacksUsageErrors`, `TestPlanReduceCommand`, `TestPlanReduceUsageErrors`, `TestPlanSummaryReadsAPlanDir`, `TestPlanSummaryTakesPlanOrPlanDir`, `TestAWSStackPlan`, `TestAWSStackApply`, `TestAWSStackApplyRefusal`, `TestAWSStackRefusesTheStatePassphrase`, `TestAWSStackUsageErrors`, `TestWetRootCommand`, `TestWetRootAfterAFailedGitHubApply`, `TestWetRootWithNothingToCommit`, `TestWetRootWithoutGitHubStateYet`, `TestWetRootRefusesAForeignMarker`, `TestWetRootUsageErrors`.
- `internal/floci`: `TestReplay` and its helpers `rehearsal`, `renderInto`, `newRehearsal`, `inAccount`, `renamedOrg`, `withStaging`, `wantPlan`, `wantTrust`, `testLog`.
- `internal/workflowspec`: `TestFlociJobsHoldNoSecret`, `checkAWSMatrix`, `TestPRPlansEachAWSStackInItsOwnJob`, `TestPRPlansCrossJobsReduced`, `TestReconcilePlansEachAWSStackInItsOwnJob`, `TestApplyAWSAppliesWhatTheGateDecided`.
- Workflow jobs `plan-aws`, `gate`, `apply-aws` (named `plan-aws ${{ matrix.stack }}` and `apply-aws ${{ matrix.stack }}`); matrix keys `stack`, `artifact`, `account`; artifacts `idp-plan-<artifact>`, `idp-renders`, `idp-gated`, `idp-applied-<artifact>`; the wet root's `wet-paths`.
- Spec amendment **A14**; `docs/adr/0031-floci-jobs-hold-no-secret.md`; phase log `docs/phases/phase-2e.md`.

**Process**
- Strict TDD: failing test, run it to see it fail, implement, run it to see it pass, commit. Tests that pin existing behavior say so; a mutation check shows what each new test catches.
- No `go build` and no GoReleaser run locally (no `go run` either). Use `go test ./...`, `go vet ./...` and `gofmt -l .` (must print nothing); `-race` runs in CI.
- There is no local Docker: `TestReplay` against floci, actionlint, zizmor and shellcheck run only in CI. Values reach run scripts only through `env:` (`TestRunScriptsTakeValuesFromEnv`).
- Shell rule: never use `cat`, `grep`, `find`, `sed` or `ls`, never write files with shell heredocs, never `perl`. Use the Read/Write/Edit tools, `rg`, `fd`, `eza`, `bat`, `sd`, `gh`, `git`, `python -I`.
- Conventional commits, never with Co-Authored-By or any AI attribution.
- Docs in English. Owner rule: docs describe the platform on its own terms, never naming another product as inspiration or comparison, and never use the owner's work account. The controller gives each dispatch the naming check that enforces both (it must print nothing).
- Doc-only changes after a merge go straight to `main`; code goes through PRs.
- Never write into `~/.idp/`. Running `~/.idp/tools/tofu-1.12.6/tofu.exe` only reads it, from a copy in a `mktemp -d` directory (D-18).
- Scratch state lives in `mktemp -d` directories outside the worktree, removed after use.
- Every pin is at least 14 days old when adopted (spec §7.5). This plan adopts none.
- Risk lines:
  - Tasks 1, 3, 9 and 13: `Risk: none`.
  - Task 2: `Risk: item 3 (wet-push is shared by reconcile and recover; the new refusal is pinned by its test)`.
  - Task 4: `Risk: item 3 (plan.Parse's decoding is shared with plan-summary and gate; plan-summary gains --plan-dir)`.
  - Task 5: `Risk: item 6 (the real OpenTofu sequence is proven only by CI's floci job, Task 7)`.
  - Task 6: `Risk: item 2 (the command refuses the state passphrase, ADR-0031)`.
  - Task 7: `Risk: item 5 (CI: the floci job runs longer)`.
  - Task 8: `Risk: item 1 (it decides what reaches wet once Task 11 wires it)`.
  - Tasks 10 and 11: `Risk: items 1, 2, 3 and 5 (the reusable workflows callers consume: what reaches wet, where secrets go, CI)`.
  - Task 12: `Risk: item 1 (recover commits to wet)`.

  Each PR's CI run and its whole-PR review check them.

**Spec rules**
- Nothing is ever applied that nobody saw (spec §6): every AWS apply equals the gated plan.
- No real cloud credential anywhere (spec §7.6, 2a's D-2); the floci jobs hold no secret at all (ADR-0031).
- The AWS state never leaves its job, and `wet` holds no state but the `github` stack's (ADR-0030).
- Emulator specifics belong to CI and its tests, never to the render (spec §5.6, ADR-0004).

**Owner actions:** none beyond approving this plan and its side effects.

**Side effects this plan authorizes** (controller, after the owner approves the plan)
1. Push `feat/phase-2e-aws-pipelines`, open its PR, and merge it after CI and the review, three times (D-12).
2. Doc-only commits straight to `main`: this plan, the phase log, the README.
3. Delete the local, git-ignored SDD workspace `.superpowers/sdd/` after the phase log is committed.

Not authorized, stop and ask:
- merging any release PR, including #28, which proposes `0.2.0`;
- merging or dispatching Renovate, or merging a Renovate PR;
- changing any repository or org setting, App, ruleset, environment, variable or secret;
- any change to `idp-claims` or `idp-claims-e2e`, including re-pinning their callers, and dispatching any of their workflows;
- a live AWS smoke run of any kind (D-10), and creating any cloud resource;
- changing `ci.yaml`, `setup-idp`, the caller templates or the floci image.

## Review Focus

1. **A job that runs floci reaching a secret.**
   - **Expected:** `plan-aws` and `apply-aws` hold no `secrets.*`, no `github.token`, no `TF_ENCRYPTION`, no environment and no permission beyond `contents: read`; their floci is CI's, digest included; `idp aws-stack` refuses to run with `TF_ENCRYPTION` or `IDP_STATE_PASSPHRASE` set.
   - **Pinned by:**
     - `TestFlociJobsHoldNoSecret` (Tasks 10, 11), over every reusable workflow, with the exact list of jobs that run floci;
     - `TestAWSStackRefusesTheStatePassphrase` (Task 6).

     Mutation-checked in Task 10 (a token in `plan-aws`) and Task 6 (the refusal off).
2. **An AWS apply of a plan nobody gated.**
   - **Expected:** `apply-aws` re-plans and applies only when the stack's changes equal the gated fingerprint's, both ways; a stack missing from the fingerprint runs no OpenTofu at all; the applied marker uploads only after a successful apply.
   - **Pinned by:**
     - `TestApplyRefusesAPlanThatDiffersFromTheGatedOne`, `TestApplyRefusesAStackTheGateNeverSaw`, `TestApplyAppliesTheGatedPlan` (Task 5);
     - `TestAWSStackApplyRefusal` (Task 6);
     - `TestApplyAWSAppliesWhatTheGateDecided` (Task 11);
     - `TestReplay`'s refusal subtest, against floci (CI, Task 7).

     Mutation-checked in Tasks 5, 6 and 11.
3. **A replay that plans against the wrong state.**
   - **Expected:** a changed stack is planned in the directory where its `wet` version was applied, so its plan shows the change, not a full create; a Component's job applies `wet`'s baseline first; an orphan is planned with `-destroy` and its `wet` configuration; nothing is written into the renders.
   - **Pinned by:**
     - `TestPlanAChangedComponentReplaysWet`, `TestPlanABaseline`, `TestPlanAnOrphanDestroysWithItsWetConfig`, `TestPlanANewComponent`, `TestPlanKeepsTheRenderUntouched` (Task 5);
     - `TestReplay` (CI, Task 7).

     Mutation-checked in Task 5 (the state dropped at the swap, no `-destroy`, the new baseline first).
4. **`wet` getting AWS state, a render for a stack that did not apply, or an orphan's render removed before its destroy.**
   - **Expected:** `wet-push` refuses any state file but `tfstate/github.tfstate`; the wet commit lists the `github` state, the `github` render only after its apply succeeded or had nothing to apply, and an AWS stack's path only when it was marked applied; `recover` pushes the same list.
   - **Pinned by:**
     - `TestSyncCommitsNoStateButGitHubs` (Task 2);
     - the `TestWetRoot*` tests (Task 8);
     - `TestApplyCommitsTheRenderOnlyAfterASuccessfulApply`, `TestWetRootIsSavedBeforeThePush` (Task 11);
     - `TestRecoverChecksTheRunAndTheWetBase` (Task 12).

     Mutation-checked in Tasks 2, 8 and 12.
5. **A stack this version cannot reconcile slipping through, or a plan missing from the comment and the gate.**
   - **Expected:** `idp stacks` refuses a Workspace, a malformed name and more than 256 AWS stacks; escaped names never collide; the comment and the gate read one reduced plan per affected stack, and a missing one fails `plan-summary`, never reads as "no changes".
   - **Pinned by:**
     - `TestReconcilable`, `TestEscape`, `TestStacksRefusesWhatThisVersionDoesNotReconcile`, `TestStacksRefusesMoreThanAMatrixRuns` (Task 3);
     - `TestPlanSummaryReadsAPlanDir`, `TestPlanSummaryTakesPlanOrPlanDir` (Task 4);
     - `TestPRPlanSummaryCoversTheAffectedStacksAtTheHeadCommit` (Task 10), `TestReconcileGatesWithTheWorkflowToken` (Task 11).

     Mutation-checked in Tasks 3 and 4.

Also tested, outside the five:
- a reduced plan holds no value, no state and no configuration (`TestReduceFormat`, Task 4, mutation-checked);
- no AWS job, so no floci, when no AWS stack is affected, and the matrix's exact strategy (`checkAWSMatrix`, Tasks 10 and 11, mutation-checked);
- OpenTofu's environment and output handling (`TestEnv`, `TestExec`, Task 5);
- one floci override for CI and the pipelines (`TestWriteOverride`, moved in Task 5);
- the account of each AWS job (`TestStacksCommand`, `TestStacksPlaceholderAccount`, Task 3, mutation-checked);
- the harness's job names and conditions (`TestApplyRunsOnlyAfterTheGate`, `TestApprovalIsOneSecretlessJob`, Task 11, mutation-checked).

## Watch list for the live steps

Record each result in the ledger:
- **Each PR's check list:** `go`, `workflows` (actionlint, zizmor and shellcheck; in PR 3 they see the matrix expressions in job names and artifact names, the `while IFS= read -r` loops and `"${args[@]}"`), `modules (github/group)`, `modules (github/component)`, `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke`, `floci` and `release-snapshot`.
- **The `floci` job of PR 2** runs `TestReplay` for the first time. Predicted: the subtests `a new stack is planned as a create`, `a changed stack is planned against its replayed state and applied` (exactly `update module.component.aws_iam_role.ci`, and floci's trust policy then names `acme-renamed/api`), `an unchanged stack plans no change`, `an orphan is destroyed with its wet configuration` (three deletes, then a baseline destroy plan) and `a plan that differs from the gated one is refused` pass. A floci gap (for example in `UpdateAssumeRolePolicy`) is handled as spec §11 risk 4 says, with the owner's ruling: stop and ask, never loosen an assertion. Record the job's duration against `19e65bd`'s 2m34s.
- **After each merge:** release PR #28 still proposes `0.2.0`, and the harness run dispatched for it passes. After PR 3, record its job durations against D-9's baselines: `pr / plan`, `pr / comment` (it now summarizes, about 10 s more), `reconcile / plan`, the new `reconcile / gate` (about 30 s), `reconcile / apply-github`. Its claims repo has no `aws` block, so `plan-aws` and `apply-aws` show as skipped; record how GitHub names a skipped matrix job.
- **Not exercised before 2f's live run** (record as pending): an AWS leg in a real pipeline; `download-artifact` with a `pattern` that matches nothing (every `apply-aws` leg failed; `prepare` runs with `always()` either way); an `init` that downloads the AWS provider twice per Component leg (if it dominates the leg's time, a plugin cache in `awsstack.Env` is the follow-up).
- **The weekly Renovate run of Monday 2026-11-23, 04:17 UTC** (scheduled runs can start hours late; not dispatched by this plan). Read the Dependency Dashboard and the PRs it opens:
  - `hashicorp/aws` 6.72.0 arrives as one "aws provider" PR labelled `needs-lockfile`, moving the module pins, `AWSProviderVersion` and the golden AWS stacks together (2d's D-5); its checks fail until the lock file is regenerated (runbook).
  - floci 2.5.0 is eligible only from 2026-11-29. When it comes, its PR must move `ci.yaml`, `pr.yaml` and `reconcile.yaml` together; `TestFlociJobsHoldNoSecret` fails the `go` job otherwise (D-13).

  Merge nothing (not authorized). Record what happened before the phase log is written, and carry the rest into 2f.

## File Structure

```
docs/superpowers/specs/2026-10-08-idp-on-actions-design.md (A14), docs/adr/0031-floci-jobs-hold-no-secret.md (new),
docs/adr/README.md                                                                                                 # Task 1
internal/wetpush/wetpush.go, internal/wetpush/wetpush_test.go                                                       # Task 2
internal/wetdiff/stacks.go (new), internal/wetdiff/stacks_test.go (new), internal/cli/stacks.go (new),
internal/cli/stacks_test.go (new), internal/cli/cli.go                                                             # Task 3
internal/plan/reduce.go (new), internal/plan/reduce_test.go (new), internal/plan/plan.go, internal/cli/planreduce.go (new),
internal/cli/planreduce_test.go (new), internal/cli/plansummary.go, internal/cli/cli.go                            # Task 4
internal/awsstack/{awsstack.go,exec.go,awsstack_test.go,exec_test.go} (new),
internal/floci/{fixture_test.go,floci_test.go,stack_test.go}                                                       # Task 5
internal/cli/awsstack.go (new), internal/cli/awsstack_test.go (new), internal/cli/cli.go                           # Task 6
internal/floci/replay_test.go (new)                                                                                # Task 7
internal/cli/wetroot.go (new), internal/cli/wetroot_test.go (new), internal/cli/cli.go                             # Task 8
docs/cli.md                                                                                                        # Task 9
.github/workflows/pr.yaml, internal/workflowspec/pr_test.go, internal/workflowspec/floci_test.go (new)             # Task 10
.github/workflows/reconcile.yaml, internal/workflowspec/reconcile_test.go, internal/workflowspec/floci_test.go     # Task 11
.github/workflows/recover.yaml, internal/workflowspec/recover_test.go                                             # Task 12
docs/pipelines.md, docs/runbooks/pipelines.md                                                                      # Task 13
docs/phases/phase-2e.md (new), README.md                                                                           # Task 14
```

Tasks 1 to 13 touch 38 distinct files: 16 in PR 1, 14 in PR 2 and 9 in PR 3 (`internal/cli/cli.go` is in PRs 1 and 2); 19 are new. Task 14 adds the phase log and edits the README.

## Pre-flight (controller, before Task 1)

- [ ] `gh api user --jq .login` prints `jellalshadows` (else `gh auth switch -u jellalshadows`).
- [ ] `git switch main && git pull --ff-only`, then `git log -1 --format=%h` prints `19e65bd` or a later commit, and `git status --short` prints nothing.
- [ ] `go version` prints `go1.26` or a later 1.26 patch, and `go test ./...` passes on `main`, so any later failure is this plan's.
- [ ] `TOFU_DIR="$(mktemp -d)" && cp "$HOME/.idp/tools/tofu-1.12.6/tofu.exe" "$TOFU_DIR/" && "$TOFU_DIR/tofu.exe" version | head -1 && rm -rf "$TOFU_DIR"` prints `OpenTofu v1.12.6` (D-18).
- [ ] `gh pr view 28 --repo jellalshadows-idp/idp-engine --json state,title --jq '"\(.state) \(.title)"'` prints `OPEN chore(main): release 0.2.0`.
- [ ] The production pin and the harness config (D-10), read-only:
  - `gh api -H "Accept: application/vnd.github.raw" repos/jellalshadows-idp/idp-claims/contents/.github/workflows/pr.yaml | rg -o '@[0-9a-f]{40} # \S+'` prints `@150b7d35dc03831046d732a642e5ff03e33ce8bf # v0.1.1`, and so do `reconcile.yaml`, `drift.yaml` and `recover.yaml`;
  - `gh api -H "Accept: application/vnd.github.raw" repos/jellalshadows-idp/idp-claims-e2e/contents/config/platform.yaml | rg -c aws` prints nothing.

  If `idp-claims` pins anything after `v0.1.1`, or the harness config has an `aws` block, stop and ask: a live repo would reach this phase's AWS jobs before 2f.
- [ ] `rg -n 'image: floci/floci' .github/workflows/ci.yaml` prints `146:        image: floci/floci:2.4.0@sha256:5ee85db604c4aa74ac76e07ea5cc83086f37bb472bf3dbb97e34019e48efae9c`, the image Tasks 10 and 11 copy.
- [ ] The owner approves this plan, its side effects and the rulings D-1 to D-18; ledger them. Commit this plan to `main` as `docs: add the phase 2e plan` and push. The push refreshes release PR #28 and re-runs the harness on it; leave the PR open.

---

### Task 1: Decisions first — spec amendment A14 and ADR-0031

**Files:**
- Modify: `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, `docs/adr/README.md`
- Create: `docs/adr/0031-floci-jobs-hold-no-secret.md`

**Interfaces:**
- Consumes: nothing.
- Produces: A14 and ADR-0031, which Tasks 2 to 13 cite.

- [ ] **Step 1: Create the branch** — `git switch -c feat/phase-2e-aws-pipelines`.

- [ ] **Step 2: Apply amendment A14 to the spec** (exact replacements with Edit)

`docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` (Edit):

1. old:

```markdown
job asks for what it needs. Every reusable job gets `idp` for the reusable
workflow's own commit, so the workflow, the CLI and the module refs are one version
(ADR-0021): a release commit installs its verified release binary, and any other
commit is built from source (ADR-0028; *Amendment A9, 2026-10-18*). Until Phase 2e
plans and applies the AWS stacks (§10), the workflows reconcile only the `github`
stack and fail if any other stack is affected. The render writes the AWS stacks of
every env with an `aws` block since Phase 2d (§5.4), so until then a claims repo with
one stays on an engine release from before Phase 2d. Until Phase 4 there are no
features to fetch or adopt.
*(Amendment A6, 2026-10-11; Amendment A10, 2026-11-03; Amendment A13, 2026-11-17.)*

### 6.1 PR (`pull_request` → `main`)
Concurrency: `idp-plan-<pr-number>`, `cancel-in-progress: true`.
```

new:

```markdown
job asks for what it needs. Every reusable job gets `idp` for the reusable
workflow's own commit, so the workflow, the CLI and the module refs are one version
(ADR-0021): a release commit installs its verified release binary, and any other
commit is built from source (ADR-0028; *Amendment A9, 2026-10-18*). Since Phase 2e
the workflows plan and apply the `github` stack and the AWS stacks the render writes
(§5.4): each env's `_baseline` and each registry Component's stack, against floci.
Any other affected stack, such as a Workspace, fails the run until Phase 3. A job
that runs floci holds no secret (ADR-0031). Until Phase 4 there are no features to
fetch or adopt.
*(Amendment A6, 2026-10-11; Amendment A10, 2026-11-03; Amendment A13, 2026-11-17;
Amendment A14, 2026-11-18.)*

### 6.1 PR (`pull_request` → `main`)
Concurrency: `idp-plan-<pr-number>`, `cancel-in-progress: true`.
```

2. old:

```markdown
   2. Apply the `wet` version of the stack, if it existed and is not `observe`.
   3. Plan the new version. An orphan is planned as a destroy, using its **old**
      config from `wet`.
5. **policy** (`idp plan-summary` over `tofu show -json`) enforces the §4.5 table.
6. **sticky comment** on the PR, posted with `GITHUB_TOKEN` and
   `pull-requests: write`:
```

new:

```markdown
   2. Apply the `wet` version of the stack, if it existed and is not `observe`.
   3. Plan the new version. An orphan is planned as a destroy, using its **old**
      config from `wet`.

   Both versions run against one state, in one scratch copy with the configuration
   swapped (ADR-0030). A job runs as the env's account from the claims, or, for an
   env the claims removed, from its stacks in `wet`. floci reaches the runner's
   Docker socket, so the job holds no secret: it gets the renders as an artifact and
   hands back only its reduced plan, the addresses and actions that the fingerprint
   reads (ADR-0031). `idp plan-summary` runs over the reduced plans of every stack in
   the secret-less `comment` job. *(Amendment A14, 2026-11-18.)*
5. **policy** (`idp plan-summary` over `tofu show -json`) enforces the §4.5 table.
6. **sticky comment** on the PR, posted with `GITHUB_TOKEN` and
   `pull-requests: write`:
```

3. old:

```markdown
      *(Amendment A8, 2026-10-13.)*
      Example: A and B merge back to back. B's PR plan showed A+B. After A applies,
      B's plan shows only B, which is a subset, so it runs automatically.
2. **approve** runs only when `gate == approval`. It references environment
   `idp-approval` (required reviewers, set by bootstrap to the platform admins;
   deployment branch `main`; **no secrets**). Required reviewers gate each job that
```

new:

```markdown
      *(Amendment A8, 2026-10-13.)*
      Example: A and B merge back to back. B's PR plan showed A+B. After A applies,
      B's plan shows only B, which is a subset, so it runs automatically.

   Since Phase 2e this step runs as three jobs: `plan` (1.1 to 1.3 for the `github`
   stack, in `idp-write`), `plan-aws` (one job per AWS stack, as in §6.1 step 4), and
   `gate` (1.4 and 1.5 over the reduced plans of every stack, with read scopes of
   `GITHUB_TOKEN` and no secret), which saves the gated fingerprint for `apply-aws`
   (ADR-0031). *(Amendment A14, 2026-11-18.)*
2. **approve** runs only when `gate == approval`. It references environment
   `idp-approval` (required reviewers, set by bootstrap to the platform admins;
   deployment branch `main`; **no secrets**). Required reviewers gate each job that
```

4. old:

```markdown
   Each job replays and re-plans in its own floci. It requires the fingerprint to
   **equal** the one that was gated, then applies and writes evidence to the job
   summary. `observe` Workspaces are never applied: for them, a successful plan
   counts as success.
4. **apply-github** references environment `idp-write` (deployment branch `main`;
   no reviewers; it holds the `idp-writer` key). It runs after `apply-aws`
   (`if: always()` plus the same gate condition). Stacks are independent (§5.5), so
```

new:

```markdown
   Each job replays and re-plans in its own floci. It requires the fingerprint to
   **equal** the one that was gated, then applies and writes evidence to the job
   summary. `observe` Workspaces are never applied: for them, a successful plan
   counts as success. A job holds no secret (ADR-0031), its state never leaves it
   (ADR-0030), and it marks its stack applied, as an artifact, only after a
   successful apply. *(Amendment A14, 2026-11-18.)*
4. **apply-github** references environment `idp-write` (deployment branch `main`;
   no reviewers; it holds the `idp-writer` key). It runs after `apply-aws`
   (`if: always()` plus the same gate condition). Stacks are independent (§5.5), so
```

5. old:

```markdown
        signed by GitHub and attributed to the writer App (ADR-0019).
      - the GitHub tfstate, **always**, even if the apply failed, because real
        resources may have changed;
      - `rendered/` for the stacks that applied successfully;
      - the manifest.

      A stack that failed keeps its old render in `wet`, so the next run sees the
```

new:

```markdown
        signed by GitHub and attributed to the writer App (ADR-0019).
      - the GitHub tfstate, **always**, even if the apply failed, because real
        resources may have changed;
      - `rendered/` for the stacks that applied successfully: the `github` stack's
        when its apply succeeded or had nothing to apply, and each AWS stack that
        `apply-aws` marked applied. An orphan's render is deleted only when its
        destroy applied. `wet` holds no state but the `github` stack's, and
        `idp wet-push` refuses any other state file (ADR-0030). *(Amendment A14,
        2026-11-18.)*
      - the manifest.

      A stack that failed keeps its old render in `wet`, so the next run sees the
```

6. old:

```markdown
- It also runs on `workflow_dispatch`. A run fails only when the check cannot run;
  drift itself is reported by the issue and a warning annotation.
  *(Amendment A6, 2026-10-11.)*

## 7. State and security
```

new:

```markdown
- It also runs on `workflow_dispatch`. A run fails only when the check cannot run;
  drift itself is reported by the issue and a warning annotation.
  *(Amendment A6, 2026-10-11.)*
- It checks the `github` stack only. An AWS stack's state lives only in its job's
  floci (ADR-0030), so there is nothing to compare until the platform targets a real
  account (§2.2). *(Amendment A14, 2026-11-18.)*

## 7. State and security
```

7. old:

```markdown
|---|---|---|---|
| 1 | A Group member who is not yet in the org gets an invitation, and the membership stays "pending", which could leave a permanent plan diff | Fallback: validation requires members to already be org members (checked with the reader token) | Closed: no fallback needed. The plan exits 0 both with the invitation pending and after acceptance. See [ADR-0013](../../adr/0013-phase-0-spike-findings.md) |
| 2 | A GitHub apply takes longer than the 1-hour installation token | Measure in Phase 0. Fallback: the provider's `app_auth` with the key file inside `idp-write` | Closed: 32 s for 10 resources is 3.2 s per resource; 3.2 s x 450 is about 24 min, under the 30 min threshold. Keep minted installation tokens, no `app_auth`. See [ADR-0013](../../adr/0013-phase-0-spike-findings.md) |
| 3 | floci replay time multiplied by the number of stacks drives CI minutes | Measure in Phase 0. Only affected stacks are planned | Measured in S3: about 26 s per stack (init 7 + apply 13 + plan 6) plus 6 s floci readiness per job. Acceptable |
| 4 | floci fidelity gaps (IAM/ECR) | The OIDC provider is already confirmed to exist in floci. Gaps get documented and skipped in level 3 | No gaps found for IAM, OIDC and ECR in S3. ECR URL uses the `localhost:4566` host form (cosmetic). Real `AssumeRoleWithWebIdentity` not exercised; out of scope for v1, which has no real AWS account (§2.2, §7.6): the AWS module tests assert the trust policy's `sub` and `aud` (§8.3), and the floci test asserts them on the created role (§8.4) (owner decision; *Amendment A10, 2026-11-03; reworded by Amendment A11, 2026-11-03, and Amendment A12, 2026-11-17*) |
| 5 | `userManaged` adoption edge cases (file deleted between adopt and apply) | `overwrite_on_create = false` on `user_managed`, so the apply fails loudly and the next reconcile retries | Not in Phase 0 scope: covered in Phase 4 (Features) |
| 6 | Cron disabled after 60 days of inactivity | Renovate activity. Documented | Not in Phase 0 scope: covered in Phase 1 (drift pipeline) and Phase 5 (docs) |
```

new:

```markdown
|---|---|---|---|
| 1 | A Group member who is not yet in the org gets an invitation, and the membership stays "pending", which could leave a permanent plan diff | Fallback: validation requires members to already be org members (checked with the reader token) | Closed: no fallback needed. The plan exits 0 both with the invitation pending and after acceptance. See [ADR-0013](../../adr/0013-phase-0-spike-findings.md) |
| 2 | A GitHub apply takes longer than the 1-hour installation token | Measure in Phase 0. Fallback: the provider's `app_auth` with the key file inside `idp-write` | Closed: 32 s for 10 resources is 3.2 s per resource; 3.2 s x 450 is about 24 min, under the 30 min threshold. Keep minted installation tokens, no `app_auth`. See [ADR-0013](../../adr/0013-phase-0-spike-findings.md) |
| 3 | floci replay time multiplied by the number of stacks drives CI minutes | Measure in Phase 0. Only affected stacks are planned. Phase 2e runs one job per affected AWS stack, ten at a time, and `idp stacks` refuses more than the 256 jobs a matrix runs (*Amendment A14, 2026-11-18*) | Measured in S3: about 26 s per stack (init 7 + apply 13 + plan 6) plus 6 s floci readiness per job. Acceptable |
| 4 | floci fidelity gaps (IAM/ECR) | The OIDC provider is already confirmed to exist in floci. Gaps get documented and skipped in level 3 | No gaps found for IAM, OIDC and ECR in S3. ECR URL uses the `localhost:4566` host form (cosmetic). Real `AssumeRoleWithWebIdentity` not exercised; out of scope for v1, which has no real AWS account (§2.2, §7.6): the AWS module tests assert the trust policy's `sub` and `aud` (§8.3), and the floci test asserts them on the created role (§8.4) (owner decision; *Amendment A10, 2026-11-03; reworded by Amendment A11, 2026-11-03, and Amendment A12, 2026-11-17*) |
| 5 | `userManaged` adoption edge cases (file deleted between adopt and apply) | `overwrite_on_create = false` on `user_managed`, so the apply fails loudly and the next reconcile retries | Not in Phase 0 scope: covered in Phase 4 (Features) |
| 6 | Cron disabled after 60 days of inactivity | Renovate activity. Documented | Not in Phase 0 scope: covered in Phase 1 (drift pipeline) and Phase 5 (docs) |
```


- [ ] **Step 3: Add ADR-0031 and its index row**

`docs/adr/0031-floci-jobs-hold-no-secret.md` (Create):

```markdown
# 0031. Jobs that run floci hold no secret

- Status: Accepted
- Date: 2026-11-18
- Spec: [§6.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#61-pr-pull_request--main), [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main) (amendment A14)

## Context

Phase 2e plans and applies the AWS stacks in the pipelines, each in a job with a floci of its own (spec §6.1 step 4, §6.2 step 3). floci's ECR runs a registry container, so the floci service mounts the runner's Docker socket (ADR-0004). Whoever holds that socket is root on the runner: a floci image that misbehaved could read every file and every environment variable of its job. The `github` stack's jobs hold the state passphrase, the reader or writer App's key, and tokens minted from them. `tofu show -json` prints every value a plan sets, in plaintext, so a full plan of the `github` stack is a secret too.

## Decision

- A job that runs floci (`plan-aws`, `apply-aws`) holds no secret: no `secrets.*`, no App token, no `GITHUB_TOKEN` beyond `contents: read`, no environment, and no `TF_ENCRYPTION` (`idp aws-stack` refuses to run when it or `IDP_STATE_PASSPHRASE` is set). Its floci is CI's, pinned by digest. It saves no cache, so nothing it writes reaches another job.
- The jobs that hold secrets never run floci. The `github` stack is planned and applied in `plan` and `apply-github`, as before.
- What crosses jobs is public or reduced: the AWS renders (`idp-renders`), each stack's reduced plan (`idp-plan-<stack>`, from `idp plan-reduce` and `idp aws-stack plan`: the format version and each change's address and actions, which is all the fingerprint reads), the gated fingerprint (`idp-gated`), and the applied-stack markers (`idp-applied-<stack>`). The `github` stack's full plan never leaves its job. Plan artifacts are kept for 1 day.
- `idp plan-summary`, the PR comment and `idp gate` run in secret-less jobs (`comment`, `gate`) over the reduced plans. `verdict` and the caller's `idp-gate` stay the only required check (ADR-0022); approval stays one secret-less job (spec §6.2 step 2).

## Consequences

### Positive

- A compromised or buggy floci image reaches nothing but public renders and an emulator.
- The PR comment and the gate read one format for every stack, and an artifact of a public repo exposes no plan value.

### Negative / costs

- More jobs: each AWS stack costs a job with its own `idp` setup and floci start (about 30 s before the first OpenTofu command), and the gate moves out of `plan` into its own job. The reconcile's `apply-github` waits for every `apply-aws` job.
- What a job that runs floci still holds is its run's runtime token, which every job has: with it, a floci image that misbehaved could upload a forged artifact for the jobs after it, such as a reduced plan or an applied-stack marker. The image is pinned by digest and moves only through a reviewed Renovate PR (spec §7.5), and no job restores a cache.
- An `apply-aws` job cannot open an issue or write anything to GitHub; a failed one shows as a failed run, and its render stays old in `wet`, so the next reconcile retries it (spec §6.2 step 4).
- Going to real AWS changes this: a job then needs cloud credentials, through OIDC, and a real account's apply can no longer share a runner with an emulator.

## Alternatives considered

- **Plan the AWS stacks in the `plan` job, beside the `github` stack.** Rejected: it would run floci, and so give the Docker socket, in a job that holds the passphrase and an App token.
- **Upload the full `tofu show -json` and reduce it in the collector.** Rejected: the `github` stack's full plan holds values in plaintext, and artifacts of public repos are public.
- **Run floci without the Docker socket.** Rejected: floci's ECR needs it (ADR-0004).

## References

- Spec §6.1 step 4, §6.2 steps 1 to 4 (amendment A14), §11 risk 3.
- ADR-0004 (one floci per job), ADR-0018 (the fingerprint), ADR-0022 (the required check), ADR-0023 (tokens per step), ADR-0030 (the AWS state stays in the job).
```


`docs/adr/README.md` (Edit):

1. old:

```markdown
| 0028 | [Releases, and how the workflows get idp](0028-releases-and-how-the-workflows-get-idp.md) | Accepted |
| 0029 | [Self-hosted Renovate with the bot App](0029-self-hosted-renovate.md) | Accepted |
| 0030 | [The AWS stacks keep their state in the job, unencrypted](0030-aws-stack-state-stays-in-the-job.md) | Accepted |
```

new:

```markdown
| 0028 | [Releases, and how the workflows get idp](0028-releases-and-how-the-workflows-get-idp.md) | Accepted |
| 0029 | [Self-hosted Renovate with the bot App](0029-self-hosted-renovate.md) | Accepted |
| 0030 | [The AWS stacks keep their state in the job, unencrypted](0030-aws-stack-state-stays-in-the-job.md) | Accepted |
| 0031 | [Jobs that run floci hold no secret](0031-floci-jobs-hold-no-secret.md) | Accepted |
```


- [ ] **Step 4: Check and commit** — `rg -c 'Amendment A14' docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` prints `7`; `go test ./... && go vet ./... && gofmt -l .` pass; the naming check prints nothing.

```bash
git add docs/superpowers/specs/2026-10-08-idp-on-actions-design.md docs/adr/README.md docs/adr/0031-floci-jobs-hold-no-secret.md
git commit -m "docs: record the aws pipelines in amendment a14 and adr-0031"
```

`Risk: none`.

---

### Task 2: wet-push commits no state but the github stack's

**Files:**
- Modify: `internal/wetpush/wetpush.go`, `internal/wetpush/wetpush_test.go`

**Interfaces:**
- Consumes: `wetpush.Sync`, the test helpers `setup`, `writeLocal`.
- Produces: the refusal `refusing to commit <path>: wet holds no state but tfstate/github.tfstate (ADR-0030)`, which Tasks 8 and 11 rely on.

- [ ] **Step 1: Write the failing test** — the new test goes before `func TestBlobSHAMatchesGit`.

`internal/wetpush/wetpush_test.go` (Edit):

1. old:

```go
	}
}

func TestBlobSHAMatchesGit(t *testing.T) {
	// echo -n "hello" | git hash-object --stdin
	if got := blobSHA([]byte("hello")); got != "b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0" {
```

new:

```go
	}
}

// TestSyncCommitsNoStateButGitHubs: wet is public, and the only state it holds
// is the github stack's, encrypted. An AWS stack's state stays in its job
// (ADR-0030), so a state file anywhere else under a synced path is refused
// before any write, even when a workflow names its directory by mistake.
func TestSyncCommitsNoStateButGitHubs(t *testing.T) {
	for _, c := range []struct{ path, file string }{
		{"rendered/aws/dev/_baseline", "rendered/aws/dev/_baseline/terraform.tfstate"},
		{"rendered/aws/dev/components/api", "rendered/aws/dev/components/api/terraform.tfstate.backup"},
		{"rendered/github", "rendered/github/github.tfstate"},
		{"tfstate/aws.tfstate", "tfstate/aws.tfstate"},
	} {
		fake, p := setup(t, nil)
		root := writeLocal(t, map[string]string{c.file: "{}"})
		_, err := p.Sync(context.Background(), root, []string{c.path}, "m")
		want := "refusing to commit " + c.file + ": wet holds no state but tfstate/github.tfstate (ADR-0030)"
		if err == nil || err.Error() != want || len(fake.Writes) != 0 {
			t.Errorf("%s: err = %v, writes = %v; want %q and no writes", c.file, err, fake.Writes, want)
		}
	}
	fake, p := setup(t, nil)
	root := writeLocal(t, map[string]string{"tfstate/github.tfstate": "state"})
	if _, err := p.Sync(context.Background(), root, []string{"tfstate/github.tfstate"}, "m"); err != nil || len(fake.Writes) == 0 {
		t.Errorf("github's state: err = %v, writes = %v; want it committed", err, fake.Writes)
	}
}

func TestBlobSHAMatchesGit(t *testing.T) {
	// echo -n "hello" | git hash-object --stdin
	if got := blobSHA([]byte("hello")); got != "b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0" {
```


- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/wetpush/ -count=1 -run TestSyncCommitsNoStateButGitHubs`
Expected: `FAIL` with four lines, one per state file, each `<file>: err = <nil>, writes = [POST /repos/acme/idp-claims/git/blobs ...]; want "refusing to commit <file>: wet holds no state but tfstate/github.tfstate (ADR-0030)" and no writes`.

- [ ] **Step 3: Implement**

`internal/wetpush/wetpush.go` (Edit):

1. old:

```go
// branch without force. A path is a file or a directory; .terraform
// directories are never synced. Nothing changed means no commit. A path that
// is a single file on the branch is never deleted: a missing local copy is an
// error, because deleting the state is never part of a reconcile.
func (p Pusher) Sync(ctx context.Context, root string, paths []string, message string) (Result, error) {
	if err := validateBranch(p.Branch); err != nil {
		return Result{}, err
```

new:

```go
// branch without force. A path is a file or a directory; .terraform
// directories are never synced. Nothing changed means no commit. A path that
// is a single file on the branch is never deleted: a missing local copy is an
// error, because deleting the state is never part of a reconcile. No state
// file but tfstate/github.tfstate is ever committed (ADR-0030).
func (p Pusher) Sync(ctx context.Context, root string, paths []string, message string) (Result, error) {
	if err := validateBranch(p.Branch); err != nil {
		return Result{}, err
```

2. old:

```go
	if err != nil {
		return Result{}, err
	}
	for _, p := range paths {
		if _, onBranch := remote[p]; onBranch {
			if _, ok := local[p]; !ok {
```

new:

```go
	if err != nil {
		return Result{}, err
	}
	for _, rel := range sortedKeys(local) {
		if isState(rel) && rel != githubState {
			return Result{}, fmt.Errorf("refusing to commit %s: wet holds no state but %s (ADR-0030)", rel, githubState)
		}
	}
	for _, p := range paths {
		if _, onBranch := remote[p]; onBranch {
			if _, ok := local[p]; !ok {
```

3. old:

```go
	return nil
}

func underAny(rel string, paths []string) bool {
	for _, p := range paths {
		if rel == p || strings.HasPrefix(rel, p+"/") {
```

new:

```go
	return nil
}

// githubState is the one state file wet holds: the github stack's, encrypted
// (spec §5.4, §7.2). An AWS stack's state stays in its job (ADR-0030).
const githubState = "tfstate/github.tfstate"

// isState reports whether a file holds OpenTofu state: a name ending in
// .tfstate, or a copy of one such as terraform.tfstate.backup.
func isState(rel string) bool {
	name := path.Base(rel)
	return strings.HasSuffix(name, ".tfstate") || strings.Contains(name, ".tfstate.")
}

func underAny(rel string, paths []string) bool {
	for _, p := range paths {
		if rel == p || strings.HasPrefix(rel, p+"/") {
```


- [ ] **Step 4: Run it to see it pass, and commit**

Run: `go test ./internal/wetpush/ -count=1`
Expected: `ok`. Then `go test ./... && go vet ./... && gofmt -l .` pass.

```bash
git add internal/wetpush/wetpush.go internal/wetpush/wetpush_test.go
git commit -m "feat(wetpush): refuse to commit any state but the github stack's"
```

`Risk: item 3 (wet-push is shared by reconcile and recover; the new refusal is pinned by its test)`.

- [ ] **Step 5: Mutation checks** (nothing is committed; restore each with `git checkout internal/wetpush/wetpush.go`, then `git status --short` prints nothing)
1. Replace `	return strings.HasSuffix(name, ".tfstate") || strings.Contains(name, ".tfstate.")` with `	return strings.HasSuffix(name, ".tfstate")`. Run the test of Step 2. Expected: `FAIL` with one line, for `rendered/aws/dev/components/api/terraform.tfstate.backup`.
2. Replace `		if isState(rel) && rel != githubState {` with `		if isState(rel) && false {`. Expected: `FAIL` with the four lines of Step 2.

---

### Task 3: Stack names and `idp stacks`

**Files:**
- Create: `internal/wetdiff/stacks.go`, `internal/wetdiff/stacks_test.go`, `internal/cli/stacks.go`, `internal/cli/stacks_test.go`
- Modify: `internal/cli/cli.go`

**Interfaces:**
- Consumes: `loadClaims(dir, stdout, stderr, env)`, `actions.SetOutput`, the claims model's `Platform.Environments[env].AWS.AccountID`, the test helpers `writeFileAll` and `envOf`.
- Produces: `wetdiff.Reconcilable(stack) bool`, `wetdiff.Escape(stack) string` (Tasks 4 to 8); `idp stacks --affected JSON --wet DIR [--dir DIR]` with the outputs `github` and `aws`, a JSON array of `{"stack","artifact","account"}` (Tasks 10, 11).

- [ ] **Step 1: Write the failing tests**

`internal/wetdiff/stacks_test.go` (Create):

```go
package wetdiff

import (
	"strings"
	"testing"
)

// TestReconcilable pins the stacks this engine version plans and applies:
// github and the AWS stacks of spec §5.4 without Workspaces (Phase 3), with the
// claims schemas' name patterns.
func TestReconcilable(t *testing.T) {
	for _, s := range []string{
		"github",
		"aws/dev/_baseline",
		"aws/pro/components/api",
		"aws/staging2/components/my-api-2",
	} {
		if !Reconcilable(s) {
			t.Errorf("Reconcilable(%q) = false, want true", s)
		}
	}
	for _, s := range []string{
		"",
		"aws/dev/workspaces/db",
		"aws/dev",
		"aws/dev/components",
		"aws/dev/components/api/extra",
		"aws/Dev/_baseline",
		"aws/abcdefghijk/_baseline",
		"aws/dev/components/-api",
		"aws/dev/components/api-",
		"aws/dev/_baseline/",
		"github/x",
		"rendered/github",
	} {
		if Reconcilable(s) {
			t.Errorf("Reconcilable(%q) = true, want false", s)
		}
	}
}

// TestEscape: a stack's escaped name holds no character that an artifact name
// refuses, and distinct stacks never share one.
func TestEscape(t *testing.T) {
	for stack, want := range map[string]string{
		"github":                 "github",
		"aws/dev/_baseline":      "aws.dev._baseline",
		"aws/dev/components/api": "aws.dev.components.api",
	} {
		if got := Escape(stack); got != want {
			t.Errorf("Escape(%q) = %q, want %q", stack, got, want)
		}
	}
	seen := map[string]string{}
	for _, s := range []string{"github", "aws/dev/_baseline", "aws/dev/components/api", "aws/dev/components/baseline", "aws/devcomponents/_baseline", "aws/dev/components/dev-api"} {
		e := Escape(s)
		if strings.ContainsAny(e, "\":<>|*?\r\n\\/") {
			t.Errorf("Escape(%q) = %q holds a character artifact names refuse", s, e)
		}
		if other, ok := seen[e]; ok {
			t.Errorf("Escape(%q) = Escape(%q) = %q", s, other, e)
		}
		seen[e] = s
	}
}
```


`internal/cli/stacks_test.go` (Create):

```go
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// awsFixture is a claims repo with dev (000000000001) and pro (000000000003).
var awsFixture = filepath.Join("..", "render", "testdata", "aws-registry")

// runStacksCmd runs idp stacks and returns its exit code, stdout, stderr and
// GITHUB_OUTPUT.
func runStacksCmd(t *testing.T, args ...string) (int, string, string, string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "github-output")
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"stacks"}, args...), &stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	got, err := os.ReadFile(out)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String(), string(got)
}

// TestStacksCommand: the AWS stacks become the plan-aws and apply-aws matrix,
// each with its escaped name and its environment's account: from the claims,
// or, for an environment the claims removed, from its wet Component stacks.
func TestStacksCommand(t *testing.T) {
	wet := t.TempDir()
	writeFileAll(t, filepath.Join(wet, "aws", "staging", "components", "api", "main.tf.json"),
		`{"module":{"component":{"oidc_provider_arn":"arn:aws:iam::000000000002:oidc-provider/token.actions.githubusercontent.com"}}}`)
	affected := `["aws/dev/_baseline","aws/dev/components/api","aws/staging/_baseline","aws/staging/components/api","github"]`
	code, stdout, stderr, output := runStacksCmd(t, "--affected", affected, "--dir", awsFixture, "--wet", wet)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	wantOut := "aws/dev/_baseline: account 000000000001\n" +
		"aws/dev/components/api: account 000000000001\n" +
		"aws/staging/_baseline: account 000000000002\n" +
		"aws/staging/components/api: account 000000000002\n" +
		"stacks: github true, 4 AWS stack(s)\n"
	if stdout != wantOut {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout, wantOut)
	}
	wantOutput := "github=true\n" + `aws=[{"stack":"aws/dev/_baseline","artifact":"aws.dev._baseline","account":"000000000001"},` +
		`{"stack":"aws/dev/components/api","artifact":"aws.dev.components.api","account":"000000000001"},` +
		`{"stack":"aws/staging/_baseline","artifact":"aws.staging._baseline","account":"000000000002"},` +
		`{"stack":"aws/staging/components/api","artifact":"aws.staging.components.api","account":"000000000002"}]` + "\n"
	if output != wantOutput {
		t.Errorf("GITHUB_OUTPUT =\n%s\nwant\n%s", output, wantOutput)
	}
}

// TestStacksPlaceholderAccount: a removed environment that had only its
// baseline names no account anywhere; its baseline's plan is the same in any
// account, so it runs as 000000000000.
func TestStacksPlaceholderAccount(t *testing.T) {
	code, stdout, stderr, _ := runStacksCmd(t, "--affected", `["aws/qa/_baseline"]`, "--dir", awsFixture, "--wet", t.TempDir())
	if code != 0 || stdout != "aws/qa/_baseline: account 000000000000\nstacks: github false, 1 AWS stack(s)\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// TestStacksWithoutAWS: no AWS stack means an empty matrix, which the
// workflows skip, and no claims to read.
func TestStacksWithoutAWS(t *testing.T) {
	for affected, github := range map[string]string{`[]`: "false", `["github"]`: "true"} {
		code, _, stderr, output := runStacksCmd(t, "--affected", affected, "--dir", filepath.Join(t.TempDir(), "none"), "--wet", t.TempDir())
		if want := "github=" + github + "\naws=[]\n"; code != 0 || output != want {
			t.Errorf("%s: exit %d, GITHUB_OUTPUT %q, stderr %q; want %q", affected, code, output, stderr, want)
		}
	}
}

// TestStacksRefusesWhatThisVersionDoesNotReconcile: any other stack fails the
// run, by name, until Phase 3 reconciles Workspaces.
func TestStacksRefusesWhatThisVersionDoesNotReconcile(t *testing.T) {
	code, _, stderr, output := runStacksCmd(t, "--affected", `["aws/dev/workspaces/db","github","rendered/x"]`, "--dir", awsFixture, "--wet", t.TempDir())
	want := "idp stacks: this engine version reconciles only github, aws/<env>/_baseline and aws/<env>/components/<name>; also affected: aws/dev/workspaces/db, rendered/x\n"
	if code != 1 || stderr != want || output != "" {
		t.Errorf("exit %d, stderr %q, GITHUB_OUTPUT %q; want 1, %q and no output", code, stderr, output, want)
	}
}

// TestStacksRefusesMoreThanAMatrixRuns: GitHub runs at most 256 jobs from one
// matrix (spec §11 risk 3).
func TestStacksRefusesMoreThanAMatrixRuns(t *testing.T) {
	var stacks []string
	for i := range 257 {
		stacks = append(stacks, fmt.Sprintf("aws/dev/components/c%03d", i))
	}
	affected, err := json.Marshal(stacks)
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr, _ := runStacksCmd(t, "--affected", string(affected), "--dir", awsFixture, "--wet", t.TempDir())
	if want := "idp stacks: 257 AWS stacks are affected; a matrix runs at most 256 jobs (spec §11 risk 3): split the change\n"; code != 1 || stderr != want {
		t.Errorf("exit %d, stderr %q; want 1 and %q", code, stderr, want)
	}
}

func TestStacksUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--wet", "w"},
		{"--affected", "[]"},
		{"--affected", "github", "--wet", "w"},
		{"--affected", "[]", "--wet", "w", "extra"},
	} {
		if code, _, stderr, _ := runStacksCmd(t, args...); code != 2 || !strings.HasPrefix(stderr, "idp stacks: ") {
			t.Errorf("%v: exit %d, stderr %q; want 2 and an idp stacks message", args, code, stderr)
		}
	}
}
```


- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/wetdiff/ ./internal/cli/ -count=1 -run 'TestReconcilable|TestEscape|TestStacks'`
Expected: `internal/wetdiff` fails to build with `undefined: Reconcilable` and `undefined: Escape`; `internal/cli` fails the six `TestStacks*` tests, and their messages show `idp: unknown command "stacks"`.

- [ ] **Step 3: Implement**

`internal/wetdiff/stacks.go` (Create):

```go
package wetdiff

import (
	"regexp"
	"strings"
)

// reconcilable matches the stacks this engine version plans and applies (spec
// §5.4): github, each environment's AWS baseline and each registry Component's
// AWS stack. Workspaces (aws/<env>/workspaces/<name>) come in Phase 3. The name
// patterns are the claims schemas' (schemas/platform.json, component.json).
var reconcilable = regexp.MustCompile(`^(?:github|aws/[a-z][a-z0-9]{0,9}/(?:_baseline|components/[a-z][a-z0-9-]{0,38}[a-z0-9]))$`)

// Reconcilable reports whether this engine version plans and applies stack.
func Reconcilable(stack string) bool { return reconcilable.MatchString(stack) }

// Escape names a stack where a slash is not allowed, such as an artifact or a
// file name: each slash becomes a dot. No reconcilable stack name holds a dot,
// so two stacks never share an escaped name.
func Escape(stack string) string { return strings.ReplaceAll(stack, "/", ".") }
```


`internal/cli/stacks.go` (Create):

```go
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

// maxMatrix is the number of jobs GitHub Actions runs from one matrix.
const maxMatrix = 256

// placeholderAccount runs the orphan baseline of an environment that the claims
// removed and that had no Component stack: nothing names its account, and a
// baseline's plan is the same in any account (its configuration names none).
const placeholderAccount = "000000000000"

// awsJob is one entry of the plan-aws and apply-aws matrix (spec §6.1 step 4).
type awsJob struct {
	Stack    string `json:"stack"`
	Artifact string `json:"artifact"` // wetdiff.Escape(Stack), for artifact names
	Account  string `json:"account"`  // the floci account the job runs as (ADR-0004)
}

// runStacks splits idp diff's affected stacks into the github stack and the AWS
// matrix, and fails on any stack this engine version does not reconcile.
func runStacks(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp stacks", flag.ContinueOnError)
	fs.SetOutput(stderr)
	affectedJSON := fs.String("affected", "", "JSON array of the affected stacks, as written by idp diff (required)")
	dir := fs.String("dir", ".", "claims repo root, whose config/platform.yaml gives each environment's account")
	wet := fs.String("wet", "", "the wet branch's rendered/ tree, whose Component stacks name the account of an environment the claims removed (required)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp stacks: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	if *affectedJSON == "" || *wet == "" {
		fmt.Fprintln(stderr, "idp stacks: --affected and --wet are required")
		return exitUsage
	}
	var affected []string
	if err := json.Unmarshal([]byte(*affectedJSON), &affected); err != nil || affected == nil {
		fmt.Fprintf(stderr, "idp stacks: --affected must be a JSON array of stack names, got %q\n", *affectedJSON)
		return exitUsage
	}
	github := false
	var aws, other []string
	for _, s := range affected {
		switch {
		case s == "github":
			github = true
		case wetdiff.Reconcilable(s):
			aws = append(aws, s)
		default:
			other = append(other, s)
		}
	}
	if len(other) > 0 {
		fmt.Fprintf(stderr, "idp stacks: this engine version reconciles only github, aws/<env>/_baseline and aws/<env>/components/<name>; also affected: %s\n", strings.Join(other, ", "))
		return exitError
	}
	if len(aws) > maxMatrix {
		fmt.Fprintf(stderr, "idp stacks: %d AWS stacks are affected; a matrix runs at most %d jobs (spec §11 risk 3): split the change\n", len(aws), maxMatrix)
		return exitError
	}
	jobs := []awsJob{}
	if len(aws) > 0 {
		m, code := loadClaims(*dir, stdout, stderr, env)
		if code != exitOK {
			return code
		}
		sort.Strings(aws)
		for _, s := range aws {
			envName := strings.Split(s, "/")[1]
			account := ""
			if e, ok := m.Platform.Environments[envName]; ok && e.AWS != nil {
				account = e.AWS.AccountID
			} else {
				a, err := wetAccount(*wet, envName)
				if err != nil {
					return fail(stderr, err)
				}
				account = a
			}
			jobs = append(jobs, awsJob{Stack: s, Artifact: wetdiff.Escape(s), Account: account})
			fmt.Fprintf(stdout, "%s: account %s\n", s, account)
		}
	}
	matrix, err := json.Marshal(jobs)
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "stacks: github %t, %d AWS stack(s)\n", github, len(jobs))
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "github", fmt.Sprint(github)); err != nil {
		return fail(stderr, err)
	}
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "aws", string(matrix)); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

// oidcAccount reads the account out of a Component stack's oidc_provider_arn,
// naming.OIDCProviderARN(accountId) (spec §5.5).
var oidcAccount = regexp.MustCompile(`^arn:aws:iam::([0-9]{12}):oidc-provider/`)

// wetAccount is the account of an environment the claims no longer give one:
// the account its wet Component stacks name, or placeholderAccount when it has
// none.
func wetAccount(wet, envName string) (string, error) {
	stacks, err := filepath.Glob(filepath.Join(wet, "aws", envName, "components", "*", wetdiff.StackFile))
	if err != nil {
		return "", err
	}
	sort.Strings(stacks)
	for _, p := range stacks {
		raw, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		var doc struct {
			Module struct {
				Component struct {
					OIDCProviderARN string `json:"oidc_provider_arn"`
				} `json:"component"`
			} `json:"module"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return "", fmt.Errorf("%s: %w", p, err)
		}
		if m := oidcAccount.FindStringSubmatch(doc.Module.Component.OIDCProviderARN); m != nil {
			return m[1], nil
		}
	}
	return placeholderAccount, nil
}
```


`internal/cli/cli.go` (Edit):

1. old:

```go
  validate        Validate a claims repo (schema + semantic checks)
  render          Render a claims repo into OpenTofu stacks
  diff            List the stacks whose render differs from the wet branch
  plan-summary    Summarize OpenTofu plans as a PR comment and a fingerprint
  gate            Decide whether a reconcile applies automatically or waits for approval
  comment         Create or update the sticky plan comment on a pull request
```

new:

```go
  validate        Validate a claims repo (schema + semantic checks)
  render          Render a claims repo into OpenTofu stacks
  diff            List the stacks whose render differs from the wet branch
  stacks          Split the affected stacks into the github stack and the AWS matrix
  plan-summary    Summarize OpenTofu plans as a PR comment and a fingerprint
  gate            Decide whether a reconcile applies automatically or waits for approval
  comment         Create or update the sticky plan comment on a pull request
```

2. old:

```go
		return runRender(args[1:], stdout, stderr, env)
	case "diff":
		return runDiff(args[1:], stdout, stderr, env)
	case "plan-summary":
		return runPlanSummary(args[1:], stdout, stderr, env)
	case "gate":
```

new:

```go
		return runRender(args[1:], stdout, stderr, env)
	case "diff":
		return runDiff(args[1:], stdout, stderr, env)
	case "stacks":
		return runStacks(args[1:], stdout, stderr, env)
	case "plan-summary":
		return runPlanSummary(args[1:], stdout, stderr, env)
	case "gate":
```


- [ ] **Step 4: Run them to see them pass, and commit**

Run: the command of Step 2. Expected: `ok` twice. Then `go test ./... && go vet ./... && gofmt -l .` pass (`TestUsageColumnsAlign` checks the new usage line).

```bash
git add internal/wetdiff/stacks.go internal/wetdiff/stacks_test.go internal/cli/stacks.go internal/cli/stacks_test.go internal/cli/cli.go
git commit -m "feat(cli): add idp stacks, the allowlist and matrix of the aws stacks"
```

`Risk: none`.

- [ ] **Step 5: Mutation checks** (nothing is committed; restore each with `git checkout internal/cli/stacks.go`, then `git status --short` prints nothing)
1. Replace `		case wetdiff.Reconcilable(s):` with `		case strings.HasPrefix(s, "aws/"):`. Run `go test ./internal/cli/ -count=1 -run TestStacks`. Expected: `FAIL: TestStacksRefusesWhatThisVersionDoesNotReconcile`.
2. Replace `ok && e.AWS != nil {` with `ok && e.AWS != nil && false {`. Expected: `FAIL: TestStacksCommand`: dev's account no longer comes from the claims.

---

### Task 4: Reduced plans — `idp plan-reduce` and `plan-summary --plan-dir`

**Files:**
- Create: `internal/plan/reduce.go`, `internal/plan/reduce_test.go`, `internal/cli/planreduce.go`, `internal/cli/planreduce_test.go`
- Modify: `internal/plan/plan.go`, `internal/cli/plansummary.go`, `internal/cli/cli.go`

**Interfaces:**
- Consumes: `plan.Parse`, `plan.FingerprintOf`, `wetdiff.Escape`, `wetdiff.Reconcilable`, the fixtures `internal/plan/testdata/{mixed,creates}.json`, `DecodeMarker`.
- Produces: `plan.Reduce(stack, r) (*Plan, []byte, error)`; `planFileIn(dir, stack)` and `countsLine(p)` (Task 6); `idp plan-reduce --stack --plan --out-dir` with the output `changes`; `idp plan-summary --plan-dir DIR` (Tasks 10, 11).

- [ ] **Step 1: Write the failing tests**

`internal/plan/reduce_test.go` (Create):

```go
package plan

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestReduceKeepsOnlyWhatTheFingerprintReads: a reduced plan holds the format
// version and each resource change's address and actions, nothing of the
// values, the state or the configuration, and parses to the same plan as the
// full one (ADR-0031).
func TestReduceKeepsOnlyWhatTheFingerprintReads(t *testing.T) {
	for _, name := range []string{"mixed.json", "creates.json"} {
		raw, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		full, err := Parse("aws/dev/components/api", bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		p, reduced, err := Reduce("aws/dev/components/api", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		again, err := Parse("aws/dev/components/api", bytes.NewReader(reduced))
		if err != nil || !reflect.DeepEqual(p, full) || !reflect.DeepEqual(again, full) {
			t.Errorf("%s: Reduce's plan %+v and its reparse %+v (err %v), want %+v", name, p, again, err, full)
		}
		var doc struct {
			Top     map[string]json.RawMessage
			Changes []map[string]json.RawMessage `json:"resource_changes"`
		}
		if err := json.Unmarshal(reduced, &doc.Top); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(reduced, &doc); err != nil {
			t.Fatal(err)
		}
		if got := slices.Sorted(maps.Keys(doc.Top)); !slices.Equal(got, []string{"format_version", "resource_changes"}) {
			t.Errorf("%s: reduced keys %v", name, got)
		}
		for _, rc := range doc.Changes {
			var change map[string]json.RawMessage
			if err := json.Unmarshal(rc["change"], &change); err != nil {
				t.Fatal(err)
			}
			if got := slices.Sorted(maps.Keys(rc)); !slices.Equal(got, []string{"address", "change"}) || len(change) != 1 || change["actions"] == nil {
				t.Errorf("%s: reduced resource change %s", name, rc)
			}
		}
	}
}

// TestReduceFormat pins the bytes: one line of JSON, no-op changes kept, and an
// empty list rather than null.
func TestReduceFormat(t *testing.T) {
	for in, want := range map[string]string{
		`{"format_version":"1.2","terraform_version":"1.13.2","resource_changes":[{"address":"a","mode":"managed","change":{"actions":["no-op"],"before":{"secret":"x"}}}],"prior_state":{}}`: `{"format_version":"1.2","resource_changes":[{"address":"a","change":{"actions":["no-op"]}}]}` + "\n",
		`{"format_version":"1.2"}`: `{"format_version":"1.2","resource_changes":[]}` + "\n",
	} {
		_, got, err := Reduce("github", strings.NewReader(in))
		if err != nil || string(got) != want {
			t.Errorf("Reduce(%s) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// TestReduceRejectsWhatParseRejects: an unknown action or format never crosses
// jobs as an artifact.
func TestReduceRejectsWhatParseRejects(t *testing.T) {
	for in, want := range map[string]string{
		`{"format_version":"2.0"}`: `plan github: unsupported format_version "2.0" (want 1.x)`,
		`{"format_version":"1.2","resource_changes":[{"address":"x","change":{"actions":["frobnicate"]}}]}`: "plan github: x: unsupported actions [frobnicate]",
		`not json`: "plan github: invalid character 'o' in literal null (expecting 'u')",
	} {
		if _, _, err := Reduce("github", strings.NewReader(in)); err == nil || err.Error() != want {
			t.Errorf("Reduce(%s) err = %v, want %q", in, err, want)
		}
	}
}
```


`internal/cli/planreduce_test.go` (Create):

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

// planFixture is a tofu show -json fixture of internal/plan.
func planFixture(name string) string { return filepath.Join("..", "plan", "testdata", name) }

// TestPlanReduceCommand: idp plan-reduce writes the stack's reduced plan under
// its escaped name, prints its counts and sets the changes output.
func TestPlanReduceCommand(t *testing.T) {
	dir := t.TempDir()
	plans, out := filepath.Join(dir, "plans"), filepath.Join(dir, "out")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plan-reduce", "--stack", "aws/dev/components/api", "--plan", planFixture("mixed.json"), "--out-dir", plans},
		&stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if got := stdout.String(); got != "aws/dev/components/api: +1 ~1 -1 ±1\n" {
		t.Errorf("stdout = %q", got)
	}
	f, err := os.Open(filepath.Join(plans, "aws.dev.components.api.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if p, err := plan.Parse("aws/dev/components/api", f); err != nil || len(p.Changes) != 4 {
		t.Errorf("reduced plan = %+v, %v; want mixed.json's 4 changes", p, err)
	}
	if got, _ := os.ReadFile(out); string(got) != "changes=4\n" {
		t.Errorf("GITHUB_OUTPUT = %q", got)
	}
}

func TestPlanReduceUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--plan", "p.json", "--out-dir", "d"},
		{"--stack", "github", "--out-dir", "d"},
		{"--stack", "github", "--plan", "p.json"},
		{"--stack", "aws/dev/workspaces/db", "--plan", "p.json", "--out-dir", "d"},
		{"--stack", "github", "--plan", "p.json", "--out-dir", "d", "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"plan-reduce"}, args...), &stdout, &stderr, noEnv); code != 2 || !strings.HasPrefix(stderr.String(), "idp plan-reduce: ") {
			t.Errorf("%v: exit %d, stderr %q; want 2 and an idp plan-reduce message", args, code, stderr.String())
		}
	}
}

// TestPlanSummaryReadsAPlanDir: the collector job's path. Reduced plans of a
// slash stack and of github, read by escaped name, give a comment whose table
// and fingerprint marker name both stacks by their slash paths.
func TestPlanSummaryReadsAPlanDir(t *testing.T) {
	dir := t.TempDir()
	plans := filepath.Join(dir, "plans")
	for stack, fixture := range map[string]string{"github": "mixed.json", "aws/dev/components/api": "creates.json"} {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"plan-reduce", "--stack", stack, "--plan", planFixture(fixture), "--out-dir", plans}, &stdout, &stderr, noEnv); code != 0 {
			t.Fatalf("plan-reduce %s: exit %d, stderr %q", stack, code, stderr.String())
		}
	}
	comment, fp := filepath.Join(dir, "c.md"), filepath.Join(dir, "fp.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plan-summary", "--stacks", `["aws/dev/components/api","github"]`, "--plan-dir", plans,
		"--head-sha", headSHA, "--comment-out", comment, "--fingerprint-out", fp}, &stdout, &stderr, noEnv)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	want := "aws/dev/components/api: +4 ~0 -0 ±0\ngithub: +1 ~1 -1 ±1\nplan-summary: 8 change(s), destructive: true\n"
	if stdout.String() != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout.String(), want)
	}
	body, err := os.ReadFile(comment)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "| <code>aws/dev/components/api</code> |") {
		t.Errorf("comment has no table row for the slash stack:\n%s", body)
	}
	sha, f, ok, err := plan.DecodeMarker(string(body))
	if err != nil || !ok || sha != headSHA || len(f["aws/dev/components/api"]) != 4 || len(f["github"]) != 4 || len(f) != 2 {
		t.Errorf("marker: sha %q, fingerprint %v, ok %t, err %v", sha, f, ok, err)
	}
}

// TestPlanSummaryTakesPlanOrPlanDir: the two ways to name the plans exclude each
// other, and a stack missing from the directory fails.
func TestPlanSummaryTakesPlanOrPlanDir(t *testing.T) {
	dir := t.TempDir()
	common := []string{"plan-summary", "--stacks", `["github"]`, "--head-sha", headSHA, "--comment-out", filepath.Join(dir, "c.md"), "--fingerprint-out", filepath.Join(dir, "fp.json")}
	var stdout, stderr bytes.Buffer
	if code := Run(append(common, "--plan-dir", dir, "--plan", "github="+planFixture("mixed.json")), &stdout, &stderr, noEnv); code != 2 || stderr.String() != "idp plan-summary: give --plan or --plan-dir, not both\n" {
		t.Errorf("both: exit %d, stderr %q", code, stderr.String())
	}
	stderr.Reset()
	if code := Run(append(common, "--plan-dir", dir), &stdout, &stderr, noEnv); code != 1 || !strings.Contains(stderr.String(), "github.json") {
		t.Errorf("missing: exit %d, stderr %q; want 1 naming github.json", code, stderr.String())
	}
}
```


- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/plan/ ./internal/cli/ -count=1 -run 'TestReduce|TestPlanReduce|TestPlanSummaryReadsAPlanDir|TestPlanSummaryTakesPlanOrPlanDir'`
Expected: `internal/plan` fails to build with `undefined: Reduce`; `internal/cli` fails `TestPlanReduceCommand` and `TestPlanReduceUsageErrors` with `idp: unknown command "plan-reduce"`, and the two `TestPlanSummary*` tests with `flag provided but not defined: -plan-dir`.

- [ ] **Step 3: Implement**

`internal/plan/reduce.go` (Create):

```go
package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// reduced is the part of tofu show -json that Parse reads: the format version
// and each resource change's address and actions. It holds no value, no state
// and no configuration, so it is what crosses jobs as an artifact (ADR-0031).
type reduced struct {
	FormatVersion   string          `json:"format_version"`
	ResourceChanges []reducedChange `json:"resource_changes"`
}

type reducedChange struct {
	Address string `json:"address"`
	Change  struct {
		Actions []string `json:"actions"`
	} `json:"change"`
}

// Reduce reads the JSON of tofu show -json for one stack and returns its plan
// and its reduced form, one line of JSON that Parse reads back to the same
// plan. It rejects what Parse rejects.
func Reduce(stack string, r io.Reader) (*Plan, []byte, error) {
	var raw reduced
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("plan %s: %w", stack, err)
	}
	if raw.ResourceChanges == nil {
		raw.ResourceChanges = []reducedChange{}
	}
	out, err := json.Marshal(raw)
	if err != nil {
		return nil, nil, err
	}
	p, err := Parse(stack, bytes.NewReader(out))
	if err != nil {
		return nil, nil, err
	}
	return p, append(out, '\n'), nil
}
```


`internal/plan/plan.go` (Edit): `Parse` decodes into the same type, so the two never disagree.

1. old:

```go
// Parse reads the JSON of tofu show -json for one stack. An action it does not
// know fails the parse: an unknown action is never treated as harmless.
func Parse(stack string, r io.Reader) (*Plan, error) {
	var raw struct {
		FormatVersion   string `json:"format_version"`
		ResourceChanges []struct {
			Address string `json:"address"`
			Change  struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("plan %s: %w", stack, err)
	}
```

new:

```go
// Parse reads the JSON of tofu show -json for one stack. An action it does not
// know fails the parse: an unknown action is never treated as harmless.
func Parse(stack string, r io.Reader) (*Plan, error) {
	var raw reduced
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("plan %s: %w", stack, err)
	}
```


`internal/cli/planreduce.go` (Create):

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

// planFileIn is where a stack's reduced plan lives in a plan directory: idp
// plan-reduce and idp aws-stack plan write it there, and idp plan-summary
// --plan-dir reads it (spec §6.1 step 5).
func planFileIn(dir, stack string) string { return filepath.Join(dir, wetdiff.Escape(stack)+".json") }

// countsLine is a stack's one-line summary: +create ~update -delete ±replace.
func countsLine(p *plan.Plan) string {
	c := p.Counts()
	return fmt.Sprintf("%s: +%d ~%d -%d ±%d", p.Stack, c.Create, c.Update, c.Delete, c.Replace)
}

func runPlanReduce(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp plan-reduce", flag.ContinueOnError)
	fs.SetOutput(stderr)
	stack := fs.String("stack", "", "the stack the plan is for, as idp diff names it (required)")
	planFile := fs.String("plan", "", "the tofu show -json output of the stack's plan (required)")
	outDir := fs.String("out-dir", "", "write the reduced plan into this directory, as <stack with each / as .>.json (required)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp plan-reduce: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	if *stack == "" || *planFile == "" || *outDir == "" {
		fmt.Fprintln(stderr, "idp plan-reduce: --stack, --plan and --out-dir are required")
		return exitUsage
	}
	if !wetdiff.Reconcilable(*stack) {
		fmt.Fprintf(stderr, "idp plan-reduce: --stack %q is not a stack this engine version reconciles\n", *stack)
		return exitUsage
	}
	f, err := os.Open(*planFile)
	if err != nil {
		return fail(stderr, err)
	}
	defer f.Close()
	p, reduced, err := plan.Reduce(*stack, f)
	if err != nil {
		return fail(stderr, err)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(planFileIn(*outDir, *stack), reduced, 0o644); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, countsLine(p))
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "changes", strconv.Itoa(p.Counts().Total())); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}
```


`internal/cli/plansummary.go` (Edit):

1. old:

```go
	runURL := fs.String("run-url", "", "link to the workflow run, shown in the comment")
	commentOut := fs.String("comment-out", "", "write the PR comment Markdown here")
	fpOut := fs.String("fingerprint-out", "", "write the fingerprint JSON here")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
```

new:

```go
	runURL := fs.String("run-url", "", "link to the workflow run, shown in the comment")
	commentOut := fs.String("comment-out", "", "write the PR comment Markdown here")
	fpOut := fs.String("fingerprint-out", "", "write the fingerprint JSON here")
	planDir := fs.String("plan-dir", "", "read each affected stack's plan from this directory, as <stack with each / as .>.json, as idp plan-reduce and idp aws-stack plan write them (instead of --plan)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
```

2. old:

```go
		}
		listed[s] = true
	}
	if !plan.ValidSHA(*headSHA) {
		fmt.Fprintf(stderr, "idp plan-summary: --head-sha %q is not a full commit SHA\n", *headSHA)
		return exitUsage
```

new:

```go
		}
		listed[s] = true
	}
	if *planDir != "" {
		if len(plans) > 0 {
			fmt.Fprintln(stderr, "idp plan-summary: give --plan or --plan-dir, not both")
			return exitUsage
		}
		for _, s := range affected {
			plans = append(plans, s+"="+planFileIn(*planDir, s))
		}
	}
	if !plan.ValidSHA(*headSHA) {
		fmt.Fprintf(stderr, "idp plan-summary: --head-sha %q is not a full commit SHA\n", *headSHA)
		return exitUsage
```

3. old:

```go
	}
	total := 0
	for _, p := range parsed {
		c := p.Counts()
		total += c.Total()
		fmt.Fprintf(stdout, "%s: +%d ~%d -%d ±%d\n", p.Stack, c.Create, c.Update, c.Delete, c.Replace)
	}
	destructive := len(fp.Destructive()) > 0
	fmt.Fprintf(stdout, "plan-summary: %d change(s), destructive: %t\n", total, destructive)
```

new:

```go
	}
	total := 0
	for _, p := range parsed {
		total += p.Counts().Total()
		fmt.Fprintln(stdout, countsLine(p))
	}
	destructive := len(fp.Destructive()) > 0
	fmt.Fprintf(stdout, "plan-summary: %d change(s), destructive: %t\n", total, destructive)
```


`internal/cli/cli.go` (Edit):

1. old:

```go
  render          Render a claims repo into OpenTofu stacks
  diff            List the stacks whose render differs from the wet branch
  stacks          Split the affected stacks into the github stack and the AWS matrix
  plan-summary    Summarize OpenTofu plans as a PR comment and a fingerprint
  gate            Decide whether a reconcile applies automatically or waits for approval
  comment         Create or update the sticky plan comment on a pull request
```

new:

```go
  render          Render a claims repo into OpenTofu stacks
  diff            List the stacks whose render differs from the wet branch
  stacks          Split the affected stacks into the github stack and the AWS matrix
  plan-reduce     Keep of a stack's plan only the addresses and actions, for an artifact
  plan-summary    Summarize OpenTofu plans as a PR comment and a fingerprint
  gate            Decide whether a reconcile applies automatically or waits for approval
  comment         Create or update the sticky plan comment on a pull request
```

2. old:

```go
		return runDiff(args[1:], stdout, stderr, env)
	case "stacks":
		return runStacks(args[1:], stdout, stderr, env)
	case "plan-summary":
		return runPlanSummary(args[1:], stdout, stderr, env)
	case "gate":
```

new:

```go
		return runDiff(args[1:], stdout, stderr, env)
	case "stacks":
		return runStacks(args[1:], stdout, stderr, env)
	case "plan-reduce":
		return runPlanReduce(args[1:], stdout, stderr, env)
	case "plan-summary":
		return runPlanSummary(args[1:], stdout, stderr, env)
	case "gate":
```


- [ ] **Step 4: Run them to see them pass, and commit**

Run: the command of Step 2. Expected: `ok` twice. Then `go test ./... && go vet ./... && gofmt -l .` pass: the existing `plan-summary` and `gate` tests pin the unchanged `--plan` path.

```bash
git add internal/plan/reduce.go internal/plan/reduce_test.go internal/plan/plan.go internal/cli/planreduce.go internal/cli/planreduce_test.go internal/cli/plansummary.go internal/cli/cli.go
git commit -m "feat(plan): reduce plans to addresses and actions, and summarize a plan dir"
```

`Risk: item 3 (plan.Parse's decoding is shared with plan-summary and gate; plan-summary gains --plan-dir)`.

- [ ] **Step 5: Mutation checks** (nothing is committed; restore with `git checkout internal/cli/planreduce.go internal/plan/reduce.go`, then `git status --short` prints nothing)
1. In `internal/cli/planreduce.go`, replace `return filepath.Join(dir, wetdiff.Escape(stack)+".json")` with `return filepath.Join(dir, stack+".json")`. Run `go test ./internal/cli/ -count=1 -run 'TestPlanReduce|TestPlanSummaryReadsAPlanDir'`. Expected: `FAIL: TestPlanReduceCommand` and `FAIL: TestPlanSummaryReadsAPlanDir`.
2. In `internal/plan/reduce.go`, replace `	if raw.ResourceChanges == nil {` with `	if raw.ResourceChanges == nil && false {`. Run `go test ./internal/plan/ -count=1 -run TestReduceFormat`. Expected: `FAIL`: `{"format_version":"1.2"}` reduces to `"resource_changes":null`.

- [ ] **Step 6: PR 1** (D-12; side effect 1)
  - Local verification: `go test ./... && go vet ./... && gofmt -l .` pass; the naming check over `main..HEAD` prints nothing; `git diff --stat main..HEAD` lists the 16 files of Tasks 1 to 4.
  - `git push -u origin feat/phase-2e-aws-pipelines`, then `gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-2e-aws-pipelines --title "feat: phase 2e stack names, reduced plans and the wet state guard" --body "Part 1 of 3 of docs/superpowers/plans/2026-11-18-phase-2e-aws-pipelines.md (spec amendment A14, ADR-0031): idp stacks (the allowlist of reconcilable stacks, the AWS matrix and each stack's account), idp plan-reduce and plan-summary --plan-dir (only addresses and actions cross jobs), and wet-push refusing any state but tfstate/github.tfstate. Nothing calls the new commands yet; the workflows change in part 3. No release."`.
  - CI: `gh pr checks <number> --repo jellalshadows-idp/idp-engine --watch`; every check of the watch list passes.
  - Whole-PR review (most capable model): `main..HEAD`, spec §6 and §11, ADR-0018, ADR-0030, ADR-0031, this plan's D-1 to D-7. One fix wave, one scoped re-review, rulings in the ledger, CI green again.
  - Merge: `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`. Record the merge commit, release PR #28's title (still `0.2.0`) and the harness run it dispatched (`success` expected).

---

### Task 5: `internal/awsstack` — replay, plan and apply one AWS stack

**Files:**
- Create: `internal/awsstack/awsstack.go`, `internal/awsstack/exec.go`, `internal/awsstack/awsstack_test.go`, `internal/awsstack/exec_test.go`
- Modify: `internal/floci/fixture_test.go`, `internal/floci/floci_test.go`, `internal/floci/stack_test.go`

**Interfaces:**
- Consumes: `plan.Reduce`, `plan.Fingerprint`, `plan.FingerprintOf`, `Fingerprint.Missing`, `wetdiff.Reconcilable`, `wetdiff.Escape`, `wetdiff.StackFile`.
- Produces: `awsstack.Runner`, `Config`, `Result`, `Plan`, `Apply`, `WriteOverride`, `Exec`, `Env` (Tasks 6 and 7). The refusal `<stack>: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)`, followed by `  not gated: <change>` and `  gated, not planned: <change>` lines.

- [ ] **Step 1: Branch again from `main`** — `git switch -c feat/phase-2e-aws-pipelines` (D-12).

- [ ] **Step 2: Write the failing tests.** The fake runner records `<dir under Work> [<config>, <state>]: <args>`, so each line shows which configuration ran against which state.

`internal/awsstack/awsstack_test.go` (Create):

```go
package awsstack

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

// fakeTofu records every tofu run as "<dir under Work> [<config>, <state>]:
// <args>", with Work shown as $WORK. <config> is the version in the directory's
// main.tf.json, and <state> the content of its terraform.tfstate, so a line
// shows which configuration ran against which state. A replay (apply
// -auto-approve) writes "state of <config>"; show prints the plan JSON set in
// show. Every run must find floci_override.tf.json beside the stack.
type fakeTofu struct {
	t     *testing.T
	work  string
	show  string
	calls []string
}

func (f *fakeTofu) Tofu(dir string, args ...string) ([]byte, error) {
	f.t.Helper()
	rel, err := filepath.Rel(f.work, dir)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "floci_override.tf.json")); err != nil {
		f.t.Errorf("tofu %v in %s: no floci_override.tf.json", args, rel)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "main.tf.json"))
	if err != nil {
		f.t.Fatalf("tofu %v in %s: %v", args, rel, err)
	}
	var doc struct{ V string }
	if err := json.Unmarshal(raw, &doc); err != nil {
		f.t.Fatal(err)
	}
	state := "no state"
	if b, err := os.ReadFile(filepath.Join(dir, "terraform.tfstate")); err == nil {
		state = string(b)
	}
	line := fmt.Sprintf("%s [%s, %s]: %s", rel, doc.V, state, strings.ReplaceAll(strings.Join(args, " "), f.work, "$WORK"))
	f.calls = append(f.calls, filepath.ToSlash(line))
	switch {
	case args[0] == "apply" && slices.Contains(args, "-auto-approve"):
		if err := os.WriteFile(filepath.Join(dir, "terraform.tfstate"), []byte("state of "+doc.V), 0o644); err != nil {
			f.t.Fatal(err)
		}
	case args[0] == "show":
		return []byte(f.show), nil
	}
	return nil, nil
}

// renders writes stacks into two trees, new and wet, each stack a directory
// with a main.tf.json naming its version and a lock file; a nil map leaves the
// tree out, as on a first reconcile.
func renders(t *testing.T, fresh, wet map[string]string) Config {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{New: filepath.Join(dir, "new"), Wet: filepath.Join(dir, "wet"), Work: filepath.Join(dir, "work"), Endpoint: "http://localhost:4566"}
	for root, stacks := range map[string]map[string]string{cfg.New: fresh, cfg.Wet: wet} {
		for stack, v := range stacks {
			d := filepath.Join(root, filepath.FromSlash(stack))
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, content := range map[string]string{"main.tf.json": `{"v":"` + v + `"}`, ".terraform.lock.hcl": "# lock " + v} {
				if err := os.WriteFile(filepath.Join(d, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	return cfg
}

// noChanges is the plan JSON of a plan that changes nothing.
const noChanges = `{"format_version":"1.2","resource_changes":[]}`

// roleUpdate is the plan JSON of a changed Component stack: its role's trust
// policy changes in place.
const roleUpdate = `{"format_version":"1.2","resource_changes":[{"address":"module.component.aws_iam_role.ci","change":{"actions":["update"],"before":{"name":"api-dev-ci"}}}]}`

func wantCalls(t *testing.T, f *fakeTofu, want ...string) {
	t.Helper()
	if !slices.Equal(f.calls, want) {
		t.Errorf("tofu runs:\n%s\nwant:\n%s", strings.Join(f.calls, "\n"), strings.Join(want, "\n"))
	}
}

// TestPlanANewComponent: no wet version, so nothing to replay. The
// environment's baseline is applied first, from the new render since wet has
// none, and the new version is planned in its own scratch copy.
func TestPlanANewComponent(t *testing.T) {
	cfg := renders(t, map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"}, nil)
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
	if _, err := Plan(cfg, f); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [new base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [new base, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [new, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/components/api [new, no state]: plan -input=false -lock=false -out=$WORK/aws.dev.components.api.tfplan",
		"rendered/aws/dev/components/api [new, no state]: show -json $WORK/aws.dev.components.api.tfplan",
	)
}

// TestPlanAChangedComponentReplaysWet: the baseline comes from wet, the wet
// version is applied, and the new version is planned in the same directory,
// against the state the replay wrote (ADR-0030), never against an empty one.
func TestPlanAChangedComponentReplaysWet(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: roleUpdate}
	res, err := Plan(cfg, f)
	if err != nil {
		t.Fatal(err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [wet base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [wet base, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [wet, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/components/api [wet, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [new, state of wet]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/components/api [new, state of wet]: plan -input=false -lock=false -out=$WORK/aws.dev.components.api.tfplan",
		"rendered/aws/dev/components/api [new, state of wet]: show -json $WORK/aws.dev.components.api.tfplan",
	)
	want := &plan.Plan{Stack: cfg.Stack, Changes: []plan.Change{{Address: "module.component.aws_iam_role.ci", Action: plan.Update}}}
	if !slices.Equal(res.Plan.Changes, want.Changes) || res.Plan.Stack != want.Stack {
		t.Errorf("plan = %+v, want %+v", res.Plan, want)
	}
	if got := string(res.Reduced); got != `{"format_version":"1.2","resource_changes":[{"address":"module.component.aws_iam_role.ci","change":{"actions":["update"]}}]}`+"\n" {
		t.Errorf("reduced = %s", got)
	}
	// The swap replaced the configuration: nothing of wet's version is left
	// beside the state, the lock file included.
	lock, err := os.ReadFile(filepath.Join(cfg.Work, "rendered", "aws", "dev", "components", "api", ".terraform.lock.hcl"))
	if err != nil || string(lock) != "# lock new" {
		t.Errorf("lock file after the swap = %q, %v; want the new version's", lock, err)
	}
}

// TestPlanAnOrphanDestroysWithItsWetConfig: a stack only wet has is replayed
// and planned as a destroy with its old configuration (spec §6.1 step 4.3).
func TestPlanAnOrphanDestroysWithItsWetConfig(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
	if _, err := Plan(cfg, f); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [wet base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [wet base, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [wet, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/components/api [wet, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/components/api [wet, state of wet]: plan -input=false -lock=false -out=$WORK/aws.dev.components.api.tfplan -destroy",
		"rendered/aws/dev/components/api [wet, state of wet]: show -json $WORK/aws.dev.components.api.tfplan",
	)
}

// TestPlanABaseline: a baseline needs no other stack first.
func TestPlanABaseline(t *testing.T) {
	cfg := renders(t, map[string]string{"aws/dev/_baseline": "new base"}, map[string]string{"aws/dev/_baseline": "wet base"})
	cfg.Stack = "aws/dev/_baseline"
	f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
	if _, err := Plan(cfg, f); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, f,
		"rendered/aws/dev/_baseline [wet base, no state]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [wet base, no state]: apply -input=false -auto-approve",
		"rendered/aws/dev/_baseline [new base, state of wet base]: init -input=false -lockfile=readonly",
		"rendered/aws/dev/_baseline [new base, state of wet base]: plan -input=false -lock=false -out=$WORK/aws.dev._baseline.tfplan",
		"rendered/aws/dev/_baseline [new base, state of wet base]: show -json $WORK/aws.dev._baseline.tfplan",
	)
}

// TestApplyAppliesTheGatedPlan: a re-plan equal to the gated fingerprint is
// applied from the saved plan, never with -auto-approve.
func TestApplyAppliesTheGatedPlan(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	f := &fakeTofu{t: t, work: cfg.Work, show: roleUpdate}
	gated := plan.Fingerprint{cfg.Stack: {{Address: "module.component.aws_iam_role.ci", Action: plan.Update}}, "github": {}}
	if _, err := Apply(cfg, f, gated); err != nil {
		t.Fatal(err)
	}
	if got, want := f.calls[len(f.calls)-1], "rendered/aws/dev/components/api [new, state of wet]: apply -input=false $WORK/aws.dev.components.api.tfplan"; got != want {
		t.Errorf("last tofu run = %q, want %q", got, want)
	}
}

// TestApplyRefusesAPlanThatDiffersFromTheGatedOne: the re-plan must equal the
// gated fingerprint, both ways (spec §6.2 step 3); otherwise nothing is
// applied.
func TestApplyRefusesAPlanThatDiffersFromTheGatedOne(t *testing.T) {
	for name, gated := range map[string]plan.Fingerprint{
		"less was gated": {"aws/dev/components/api": {}},
		"more was gated": {"aws/dev/components/api": {
			{Address: "module.component.aws_iam_role.ci", Action: plan.Update},
			{Address: "module.component.aws_ecr_repository.this", Action: plan.Delete},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := renders(t,
				map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
				map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
			cfg.Stack = "aws/dev/components/api"
			f := &fakeTofu{t: t, work: cfg.Work, show: roleUpdate}
			_, err := Apply(cfg, f, gated)
			want := map[string]string{
				"less was gated": "aws/dev/components/api: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)\n  not gated: aws/dev/components/api: update module.component.aws_iam_role.ci",
				"more was gated": "aws/dev/components/api: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)\n  gated, not planned: aws/dev/components/api: delete module.component.aws_ecr_repository.this",
			}[name]
			if err == nil || err.Error() != want {
				t.Errorf("err = %v, want %q", err, want)
			}
			if last := f.calls[len(f.calls)-1]; !strings.Contains(last, ": show -json ") {
				t.Errorf("a tofu run followed the refused plan: %q", last)
			}
		})
	}
}

// TestApplyRefusesAStackTheGateNeverSaw: no entry in the gated fingerprint
// means no gate decision covers the stack; nothing runs.
func TestApplyRefusesAStackTheGateNeverSaw(t *testing.T) {
	cfg := renders(t, map[string]string{"aws/dev/_baseline": "new base"}, nil)
	cfg.Stack = "aws/dev/_baseline"
	f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
	_, err := Apply(cfg, f, plan.Fingerprint{"github": {}})
	if want := "aws/dev/_baseline: the gated fingerprint has no entry for this stack, so nothing was applied"; err == nil || err.Error() != want || len(f.calls) != 0 {
		t.Errorf("err = %v, tofu runs %v; want %q and none", err, f.calls, want)
	}
}

func TestPlanRefuses(t *testing.T) {
	base := map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"}
	for name, c := range map[string]struct {
		stack string
		fresh map[string]string
		edit  func(*Config)
		want  string
	}{
		"github":           {stack: "github", fresh: base, want: `"github" is not an AWS stack this engine version reconciles (aws/<env>/_baseline, aws/<env>/components/<name>)`},
		"a workspace":      {stack: "aws/dev/workspaces/db", fresh: base, want: `"aws/dev/workspaces/db" is not an AWS stack this engine version reconciles (aws/<env>/_baseline, aws/<env>/components/<name>)`},
		"in neither":       {stack: "aws/dev/components/web", fresh: base, want: "aws/dev/components/web: in neither the new render nor wet"},
		"without baseline": {stack: "aws/dev/components/api", fresh: map[string]string{"aws/dev/components/api": "new"}, want: "aws/dev/components/api: no baseline aws/dev/_baseline in wet or in the new render"},
		"work in a render": {stack: "aws/dev/components/api", fresh: base, edit: func(c *Config) { c.Work = filepath.Join(c.New, "aws", "work") }, want: "is inside the render"},
		"used work":        {stack: "aws/dev/components/api", fresh: base, edit: func(c *Config) { _ = os.MkdirAll(filepath.Join(c.Work, "rendered"), 0o755) }, want: "already holds a rendered tree"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := renders(t, c.fresh, nil)
			cfg.Stack = c.stack
			if c.edit != nil {
				c.edit(&cfg)
			}
			f := &fakeTofu{t: t, work: cfg.Work, show: noChanges}
			_, err := Plan(cfg, f)
			if err == nil || !strings.Contains(err.Error(), c.want) || len(f.calls) != 0 {
				t.Errorf("err = %v, tofu runs %v; want %q and none", err, f.calls, c.want)
			}
		})
	}
}

// TestPlanKeepsTheRenderUntouched: the scratch copy is the only place tofu
// writes; the render stays as rendered (ADR-0030).
func TestPlanKeepsTheRenderUntouched(t *testing.T) {
	cfg := renders(t,
		map[string]string{"aws/dev/_baseline": "new base", "aws/dev/components/api": "new"},
		map[string]string{"aws/dev/_baseline": "wet base", "aws/dev/components/api": "wet"})
	cfg.Stack = "aws/dev/components/api"
	if _, err := Plan(cfg, &fakeTofu{t: t, work: cfg.Work, show: noChanges}); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{cfg.New, cfg.Wet} {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && d.Name() != "main.tf.json" && d.Name() != ".terraform.lock.hcl" {
				err = errors.New(p + " was written into the render")
			}
			return err
		})
		if err != nil {
			t.Error(err)
		}
	}
}
```


`internal/awsstack/exec_test.go` (Create): `TestWriteOverride` moves here from `internal/floci/stack_test.go`, unchanged but for the function's name.

```go
package awsstack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestMain turns the test binary into a stub tofu when IDP_AWSSTACK_FAKE_TOFU
// is set: it prints its arguments to stdout and "stderr" to stderr, and exits
// with the variable's value, so TestExec runs offline.
func TestMain(m *testing.M) {
	if code := os.Getenv("IDP_AWSSTACK_FAKE_TOFU"); code != "" {
		fmt.Println(strings.Join(os.Args[1:], " "))
		fmt.Fprintln(os.Stderr, "stderr")
		if code != "0" {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestExec: a show's stdout is the plan JSON, returned and not logged; any
// other run logs both streams; a failure names the command and the directory.
func TestExec(t *testing.T) {
	dir := t.TempDir()
	var log bytes.Buffer
	e := Exec{Path: os.Args[0], Env: append(os.Environ(), "IDP_AWSSTACK_FAKE_TOFU=0"), Log: &log}
	out, err := e.Tofu(dir, "show", "-json", "x.tfplan")
	if err != nil || string(out) != "show -json x.tfplan\n" || log.String() != "stderr\n" {
		t.Errorf("show: out %q, log %q, err %v", out, log.String(), err)
	}
	log.Reset()
	out, err = e.Tofu(dir, "init", "-input=false")
	if err != nil || len(out) != 0 || log.String() != "init -input=false\nstderr\n" {
		t.Errorf("init: out %q, log %q, err %v", out, log.String(), err)
	}
	e.Env = append(os.Environ(), "IDP_AWSSTACK_FAKE_TOFU=1")
	if _, err := e.Tofu(dir, "apply", "x.tfplan"); err == nil || err.Error() != "tofu apply x.tfplan in "+dir+": exit status 1" {
		t.Errorf("failure: err = %v", err)
	}
}

// TestEnv: a run against floci is the account's, by its access key ID
// (ADR-0004), and never prompts.
func TestEnv(t *testing.T) {
	got := Env([]string{"PATH=/bin"}, "000000000001")
	want := []string{"PATH=/bin", "AWS_ACCESS_KEY_ID=000000000001", "AWS_SECRET_ACCESS_KEY=floci-does-not-check-it", "TF_IN_AUTOMATION=1", "TF_INPUT=0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Env = %v, want %v", got, want)
	}
}

// TestWriteOverride pins what CI adds to a rendered AWS stack for floci (spec
// §5.6): the endpoint and the provider flags that OpenTofu merges into the
// stack's provider block. The pipelines and the floci tests write it with this
// one function.
func TestWriteOverride(t *testing.T) {
	dir := t.TempDir()
	if err := WriteOverride(dir, "http://localhost:4566"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "floci_override.tf.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"provider": map[string]any{"aws": map[string]any{
		"endpoints":                   map[string]any{"ecr": "http://localhost:4566", "iam": "http://localhost:4566", "sts": "http://localhost:4566"},
		"skip_credentials_validation": true,
		"skip_metadata_api_check":     "true",
		"skip_requesting_account_id":  true,
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("floci_override.tf.json = %v, want %v", got, want)
	}
}
```


- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/awsstack/ -count=1`
Expected: build failure, `undefined: Config` and `undefined: Plan`.

- [ ] **Step 4: Implement**

`internal/awsstack/awsstack.go` (Create):

```go
// Package awsstack replays, plans and applies one rendered AWS stack against
// floci, the way the pipelines' plan-aws and apply-aws jobs do (spec §6.1 step
// 4, §6.2 step 3). Each job starts an empty floci (ADR-0004) and rebuilds what
// wet records: the environment's baseline, then the wet version of the stack,
// whose new version is planned against the state that replay wrote, in the same
// scratch directory with the configuration swapped (ADR-0030). The stack's
// state never leaves the job, and a job that runs floci holds no secret
// (ADR-0031).
package awsstack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

// Runner runs OpenTofu in a directory and returns what it prints on stdout.
type Runner interface {
	Tofu(dir string, args ...string) ([]byte, error)
}

// Config is one stack's job.
type Config struct {
	Stack    string // aws/<env>/_baseline or aws/<env>/components/<name>
	New      string // the fresh render's root; a stack missing from it is an orphan
	Wet      string // wet's rendered/ root; it may be missing, before the first reconcile
	Work     string // a scratch directory outside both renders; Work/rendered must not exist yet
	Endpoint string // floci's endpoint, written into floci_override.tf.json
}

// Result is a stack's plan.
type Result struct {
	Plan    *plan.Plan // the stack's changes
	Reduced []byte     // the plan as plan.Reduce writes it, for the plan artifact
	dir     string     // where it was planned
	saved   string     // the saved plan, which Apply applies
}

// stateFiles survive the swap from the wet version to the new one: the state
// the replay wrote, and the providers and modules init installed.
var stateFiles = map[string]bool{".terraform": true, "terraform.tfstate": true, "terraform.tfstate.backup": true}

// Plan rebuilds what wet records and plans the stack (spec §6.1 step 4). In
// Work/rendered, laid out like a render so relative module sources resolve the
// same way:
//  1. a Component stack's environment baseline is applied first: wet's
//     version, which the applied Component stacks were planned with, or the new
//     one when wet has none yet;
//  2. the wet version of the stack, when it has one, is applied;
//  3. the new version is planned in the same directory, against that state, or,
//     for an orphan, a destroy with the wet configuration.
//
// The replays apply with -auto-approve: they only rebuild the emulator's state.
func Plan(cfg Config, run Runner) (*Result, error) {
	env, baseline, err := parse(cfg.Stack)
	if err != nil {
		return nil, err
	}
	if err := checkWork(cfg); err != nil {
		return nil, err
	}
	fresh, old := stackDir(cfg.New, cfg.Stack), stackDir(cfg.Wet, cfg.Stack)
	hasNew, err := isStack(fresh)
	if err != nil {
		return nil, err
	}
	hasWet, err := isStack(old)
	if err != nil {
		return nil, err
	}
	if !hasNew && !hasWet {
		return nil, fmt.Errorf("%s: in neither the new render nor wet", cfg.Stack)
	}
	rendered := filepath.Join(cfg.Work, "rendered")
	if !baseline {
		base := "aws/" + env + "/_baseline"
		src, err := baselineSource(cfg, base)
		if err != nil {
			return nil, err
		}
		dir := stackDir(rendered, base)
		if err := stage(src, dir, cfg.Endpoint); err != nil {
			return nil, err
		}
		if err := replay(run, dir); err != nil {
			return nil, err
		}
	}
	dir := stackDir(rendered, cfg.Stack)
	destroy := false
	switch {
	case hasWet:
		if err := stage(old, dir, cfg.Endpoint); err != nil {
			return nil, err
		}
		if err := replay(run, dir); err != nil {
			return nil, err
		}
		if !hasNew {
			destroy = true
			break
		}
		if err := swap(fresh, dir, cfg.Endpoint); err != nil {
			return nil, err
		}
		if err := initStack(run, dir); err != nil {
			return nil, err
		}
	default:
		if err := stage(fresh, dir, cfg.Endpoint); err != nil {
			return nil, err
		}
		if err := initStack(run, dir); err != nil {
			return nil, err
		}
	}
	saved := filepath.Join(cfg.Work, wetdiff.Escape(cfg.Stack)+".tfplan")
	args := []string{"plan", "-input=false", "-lock=false", "-out=" + saved}
	if destroy {
		args = append(args, "-destroy")
	}
	if _, err := run.Tofu(dir, args...); err != nil {
		return nil, err
	}
	js, err := run.Tofu(dir, "show", "-json", saved)
	if err != nil {
		return nil, err
	}
	p, reduced, err := plan.Reduce(cfg.Stack, bytes.NewReader(js))
	if err != nil {
		return nil, err
	}
	return &Result{Plan: p, Reduced: reduced, dir: dir, saved: saved}, nil
}

// Apply re-plans the stack as Plan does, requires its changes to equal the
// gated fingerprint's for the stack, both ways, and applies the saved plan
// (spec §6.2 step 3). Anything else applies nothing.
func Apply(cfg Config, run Runner, gated plan.Fingerprint) (*Result, error) {
	want, ok := gated[cfg.Stack]
	if !ok {
		return nil, fmt.Errorf("%s: the gated fingerprint has no entry for this stack, so nothing was applied", cfg.Stack)
	}
	res, err := Plan(cfg, run)
	if err != nil {
		return nil, err
	}
	if err := sameChanges(cfg.Stack, res.Plan, want); err != nil {
		return nil, err
	}
	if _, err := run.Tofu(res.dir, "apply", "-input=false", res.saved); err != nil {
		return nil, err
	}
	return res, nil
}

// sameChanges compares a plan's changes with the gated ones for its stack.
func sameChanges(stack string, p *plan.Plan, gated []plan.Change) error {
	got, want := plan.FingerprintOf(p), plan.Fingerprint{stack: gated}
	extra, lost := got.Missing(want), want.Missing(got)
	if len(extra)+len(lost) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)", stack)
	for _, e := range extra {
		fmt.Fprintf(&b, "\n  not gated: %s", e)
	}
	for _, l := range lost {
		fmt.Fprintf(&b, "\n  gated, not planned: %s", l)
	}
	return errors.New(b.String())
}

// parse splits an AWS stack's slash path into its environment and whether it is
// the environment's baseline.
func parse(stack string) (env string, baseline bool, err error) {
	if !strings.HasPrefix(stack, "aws/") || !wetdiff.Reconcilable(stack) {
		return "", false, fmt.Errorf("%q is not an AWS stack this engine version reconciles (aws/<env>/_baseline, aws/<env>/components/<name>)", stack)
	}
	parts := strings.Split(stack, "/")
	return parts[1], parts[2] == "_baseline", nil
}

// checkWork keeps tofu out of the renders: Work is a fresh scratch directory
// outside both (ADR-0030).
func checkWork(cfg Config) error {
	work, err := filepath.Abs(cfg.Work)
	if err != nil {
		return err
	}
	for _, root := range []string{cfg.Wet, cfg.New} {
		abs, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		if rel, err := filepath.Rel(abs, work); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("work %s is inside the render %s: a stack runs in a scratch copy, never in a rendered tree (ADR-0030)", cfg.Work, root)
		}
	}
	if _, err := os.Stat(filepath.Join(work, "rendered")); err == nil {
		return fmt.Errorf("work %s already holds a rendered tree: give each stack a fresh scratch directory", cfg.Work)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// baselineSource is the baseline a Component stack's job applies first: wet's
// if it has one, else the new render's.
func baselineSource(cfg Config, base string) (string, error) {
	for _, root := range []string{cfg.Wet, cfg.New} {
		dir := stackDir(root, base)
		ok, err := isStack(dir)
		if err != nil {
			return "", err
		}
		if ok {
			return dir, nil
		}
	}
	return "", fmt.Errorf("%s: no baseline %s in wet or in the new render", cfg.Stack, base)
}

func stackDir(root, stack string) string { return filepath.Join(root, filepath.FromSlash(stack)) }

// isStack reports whether dir holds a stack; a missing directory, or a missing
// root, holds none.
func isStack(dir string) (bool, error) {
	_, err := os.Stat(filepath.Join(dir, wetdiff.StackFile))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	}
	return false, err
}

// stage copies a rendered stack into dir and adds floci's override.
func stage(src, dir, endpoint string) error {
	if err := os.CopyFS(dir, os.DirFS(src)); err != nil {
		return err
	}
	return WriteOverride(dir, endpoint)
}

// swap replaces the configuration in dir with src's and keeps the state, the
// providers and the modules (ADR-0030: one state for both versions).
func swap(src, dir, endpoint string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !stateFiles[e.Name()] {
			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return stage(src, dir, endpoint)
}

func initStack(run Runner, dir string) error {
	_, err := run.Tofu(dir, "init", "-input=false", "-lockfile=readonly")
	return err
}

// replay inits and applies a stack version, to rebuild its state in floci.
func replay(run Runner, dir string) error {
	if err := initStack(run, dir); err != nil {
		return err
	}
	_, err := run.Tofu(dir, "apply", "-input=false", "-auto-approve")
	return err
}

// WriteOverride writes into a staged AWS stack what CI adds for floci (spec
// §5.6): floci_override.tf.json, with the endpoint and the provider flags that
// OpenTofu merges into the stack's provider block. The pipelines and the floci
// tests both write it with this function.
func WriteOverride(dir, endpoint string) error {
	b, err := json.MarshalIndent(map[string]any{
		"provider": map[string]any{"aws": map[string]any{
			"endpoints":                   map[string]any{"ecr": endpoint, "iam": endpoint, "sts": endpoint},
			"skip_credentials_validation": true,
			"skip_metadata_api_check":     "true",
			"skip_requesting_account_id":  true,
		}},
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "floci_override.tf.json"), append(b, '\n'), 0o644)
}
```


`internal/awsstack/exec.go` (Create):

```go
package awsstack

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
)

// fakeSecret is the secret access key floci receives: floci checks no
// signature, and its account is the 12-digit access key ID (ADR-0004).
const fakeSecret = "floci-does-not-check-it"

// Env is base plus what a tofu run against floci needs: the account, as the
// access key ID (ADR-0004), and no prompt.
func Env(base []string, account string) []string {
	return append(slices.Clip(base),
		"AWS_ACCESS_KEY_ID="+account,
		"AWS_SECRET_ACCESS_KEY="+fakeSecret,
		"TF_IN_AUTOMATION=1",
		"TF_INPUT=0",
	)
}

// Exec runs the tofu binary at Path with the environment Env. Its output goes
// to Log, except a show's stdout, which is the plan's JSON and is returned.
type Exec struct {
	Path string
	Env  []string
	Log  io.Writer
}

// Tofu runs tofu with args in dir.
func (e Exec) Tofu(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(e.Path, args...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, e.Env, e.Log, e.Log
	var out bytes.Buffer
	if len(args) > 0 && args[0] == "show" {
		cmd.Stdout = &out
	}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tofu %s in %s: %w", strings.Join(args, " "), dir, err)
	}
	return out.Bytes(), nil
}
```


- [ ] **Step 5: Run them to see them pass**

Run: `go test ./internal/awsstack/ -count=1 -v`
Expected: `PASS` for each test, the two `TestApplyRefusesAPlanThatDiffersFromTheGatedOne` subtests and the six `TestPlanRefuses` subtests included, then `ok`.

- [ ] **Step 6: Move the floci tests to the one override** — the floci tests now write the override with `awsstack.WriteOverride`, and their own copy and its test go.

`internal/floci/fixture_test.go` (Edit):

1. old:

```go
package floci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
```

new:

```go
package floci

import (
	"fmt"
	"path/filepath"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
```

2. old:

```go
	}
	return render.WriteTree(filepath.Join(root, "rendered"), files)
}

// writeOverride writes into a rendered AWS stack what CI adds for floci (spec
// §5.6): floci_override.tf.json, with the endpoint and the provider flags that
// OpenTofu merges into the stack's provider block.
func writeOverride(dir, endpoint string) error {
	b, err := json.MarshalIndent(map[string]any{
		"provider": map[string]any{"aws": map[string]any{
			"endpoints":                   map[string]any{"ecr": endpoint, "iam": endpoint, "sts": endpoint},
			"skip_credentials_validation": true,
			"skip_metadata_api_check":     "true",
			"skip_requesting_account_id":  true,
		}},
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "floci_override.tf.json"), append(b, '\n'), 0o644)
}
```

new:

```go
	}
	return render.WriteTree(filepath.Join(root, "rendered"), files)
}
```


`internal/floci/floci_test.go` (Edit):

1. old:

```go
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
)
```

new:

```go
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/awsstack"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
)
```

2. old:

```go
}

// apply creates one rendered stack of tg's environment, a slash path under
// rendered/, the way the pipelines will (spec §5.6, §6.1): it adds
// floci_override.tf.json, inits with the rendered lock file, applies, then
// replays, a plan of the same version that must find nothing to change (spec
// §8.4). It registers the stack's destroy before applying, so whatever an apply
```

new:

```go
}

// apply creates one rendered stack of tg's environment, a slash path under
// rendered/, the way the pipelines do (spec §5.6, §6.1): it adds
// floci_override.tf.json, inits with the rendered lock file, applies, then
// replays, a plan of the same version that must find nothing to change (spec
// §8.4). It registers the stack's destroy before applying, so whatever an apply
```

3. old:

```go
func (f *floci) apply(t *testing.T, tg target, stack string) {
	t.Helper()
	dir := filepath.Join(f.root, "rendered", filepath.FromSlash(stack))
	if err := writeOverride(dir, f.endpoint); err != nil {
		t.Fatal(err)
	}
	f.tofuOK(t, tg, dir, "init", "-input=false", "-lockfile=readonly", "-no-color")
```

new:

```go
func (f *floci) apply(t *testing.T, tg target, stack string) {
	t.Helper()
	dir := filepath.Join(f.root, "rendered", filepath.FromSlash(stack))
	if err := awsstack.WriteOverride(dir, f.endpoint); err != nil {
		t.Fatal(err)
	}
	f.tofuOK(t, tg, dir, "init", "-input=false", "-lockfile=readonly", "-no-color")
```


`internal/floci/stack_test.go` (Edit):

1. old:

```go
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
```

new:

```go
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
```

2. old:

```go

// TestRenderFixture: the render TestFloci applies holds exactly the AWS stacks
// it applies, each with the engine's AWS lock file, a module source in the
// modules it copies, and nothing of floci's, which writeOverride adds.
func TestRenderFixture(t *testing.T) {
	root := t.TempDir()
	if err := renderFixture(root); err != nil {
```

new:

```go

// TestRenderFixture: the render TestFloci applies holds exactly the AWS stacks
// it applies, each with the engine's AWS lock file, a module source in the
// modules it copies, and nothing of floci's, which awsstack.WriteOverride adds.
func TestRenderFixture(t *testing.T) {
	root := t.TempDir()
	if err := renderFixture(root); err != nil {
```

3. old:

```go
	}
}

func TestWriteOverride(t *testing.T) {
	dir := t.TempDir()
	if err := writeOverride(dir, "http://localhost:4566"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "floci_override.tf.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"provider": map[string]any{"aws": map[string]any{
		"endpoints":                   map[string]any{"ecr": "http://localhost:4566", "iam": "http://localhost:4566", "sts": "http://localhost:4566"},
		"skip_credentials_validation": true,
		"skip_metadata_api_check":     "true",
		"skip_requesting_account_id":  true,
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("floci_override.tf.json = %v, want %v", got, want)
	}
}

// TestModulesUseNamingsIssuer pins the one identity the modules write
// themselves: GitHub Actions' issuer, whose host is in every OIDC provider ARN
// that internal/naming computes and in the trust policy's condition keys.
```

new:

```go
	}
}

// TestModulesUseNamingsIssuer pins the one identity the modules write
// themselves: GitHub Actions' issuer, whose host is in every OIDC provider ARN
// that internal/naming computes and in the trust policy's condition keys.
```


Run: `go test ./internal/floci/ ./internal/awsstack/ -count=1`
Expected: `ok` twice (`TestFloci` skips without `IDP_FLOCI_ENDPOINT`). `rg -n 'writeOverride' internal/floci` prints nothing. Then `go test ./... && go vet ./... && gofmt -l .` pass.

- [ ] **Step 7: Commit**

```bash
git add internal/awsstack internal/floci/fixture_test.go internal/floci/floci_test.go internal/floci/stack_test.go
git commit -m "feat(awsstack): replay, plan and apply one aws stack against floci"
```

`Risk: item 6 (the real OpenTofu sequence is proven only by CI's floci job, Task 7)`.

- [ ] **Step 8: Mutation checks** (nothing is committed; restore each with `git checkout internal/awsstack/awsstack.go`, then `git status --short` prints nothing). Run `go test ./internal/awsstack/ -count=1` after each.
1. Replace `".terraform": true, "terraform.tfstate": true,` with `".terraform": true,`. Expected: `FAIL: TestPlanAChangedComponentReplaysWet`, `TestPlanABaseline` and `TestApplyAppliesTheGatedPlan`: the new version runs against `no state`.
2. Replace `			destroy = true` with `			destroy = false`. Expected: `FAIL: TestPlanAnOrphanDestroysWithItsWetConfig`.
3. Replace `	if len(extra)+len(lost) == 0 {` with `	if len(extra) == 0 {`. Expected: `FAIL: TestApplyRefusesAPlanThatDiffersFromTheGatedOne/more_was_gated`.
4. In `baselineSource`, replace the two lines `	for _, root := range []string{cfg.Wet, cfg.New} {` and `		dir := stackDir(root, base)` with `	for _, root := range []string{cfg.New, cfg.Wet} {` and the same second line. Expected: `FAIL: TestPlanAChangedComponentReplaysWet` and `TestPlanAnOrphanDestroysWithItsWetConfig`: the baseline comes from the new render.

---

### Task 6: `idp aws-stack plan|apply`

**Files:**
- Create: `internal/cli/awsstack.go`, `internal/cli/awsstack_test.go`
- Modify: `internal/cli/cli.go`

**Interfaces:**
- Consumes: `awsstack.Plan`, `awsstack.Apply`, `awsstack.Exec`, `awsstack.Env`, `plan.ReadFingerprint`, `planFileIn`, `countsLine`, `fail`, `actions.SetOutput`, `actions.AddSummary`.
- Produces: `idp aws-stack plan` (the reduced plan in `--out-dir`, the output `changes`) and `idp aws-stack apply` (the marker `<--applied-dir>/<escaped stack>` holding the stack and a newline, the job summary), which Tasks 10 and 11 run; `newTofu`, the seam its tests replace.

- [ ] **Step 1: Write the failing tests**

`internal/cli/awsstack_test.go` (Create):

````go
package cli

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/awsstack"
)

// stubTofu answers every tofu run of idp aws-stack: show prints its plan JSON.
type stubTofu struct {
	show string
	env  []string
	runs []string
}

func (s *stubTofu) Tofu(dir string, args ...string) ([]byte, error) {
	s.runs = append(s.runs, strings.Join(args, " "))
	if args[0] == "show" {
		return []byte(s.show), nil
	}
	return nil, nil
}

// useStubTofu makes idp aws-stack run s instead of the tofu binary.
func useStubTofu(t *testing.T, show string) *stubTofu {
	t.Helper()
	s := &stubTofu{show: show}
	old := newTofu
	newTofu = func(_ string, env []string, _ io.Writer) awsstack.Runner {
		s.env = env
		return s
	}
	t.Cleanup(func() { newTofu = old })
	return s
}

// awsStackDirs renders a wet and a new version of aws/dev/components/api and
// its baseline, and returns the flags that point idp aws-stack at them.
func awsStackDirs(t *testing.T) (dir string, flags []string) {
	t.Helper()
	dir = t.TempDir()
	for _, root := range []string{"new", "wet"} {
		for _, stack := range []string{"aws/dev/_baseline", "aws/dev/components/api"} {
			writeFileAll(t, filepath.Join(dir, root, filepath.FromSlash(stack), "main.tf.json"), `{"v":"`+root+`"}`)
		}
	}
	return dir, []string{"--stack", "aws/dev/components/api", "--account", "000000000001",
		"--new", filepath.Join(dir, "new"), "--wet", filepath.Join(dir, "wet"), "--work", filepath.Join(dir, "work"),
		"--endpoint", "http://localhost:4566"}
}

const updateRole = `{"format_version":"1.2","resource_changes":[{"address":"module.component.aws_iam_role.ci","change":{"actions":["update"]}}]}`

// TestAWSStackPlan: plan writes the stack's reduced plan for the plan-aws
// artifact, prints its counts, and runs tofu as the account.
func TestAWSStackPlan(t *testing.T) {
	s := useStubTofu(t, updateRole)
	dir, flags := awsStackDirs(t)
	plans, out := filepath.Join(dir, "plans"), filepath.Join(dir, "out")
	var stdout, stderr bytes.Buffer
	code := Run(append(append([]string{"aws-stack", "plan"}, flags...), "--out-dir", plans), &stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if got := stdout.String(); got != "aws/dev/components/api: +0 ~1 -0 ±0\n" {
		t.Errorf("stdout = %q", got)
	}
	got, err := os.ReadFile(filepath.Join(plans, "aws.dev.components.api.json"))
	if err != nil || string(got) != updateRole+"\n" {
		t.Errorf("reduced plan = %q, %v", got, err)
	}
	if got, _ := os.ReadFile(out); string(got) != "changes=1\n" {
		t.Errorf("GITHUB_OUTPUT = %q", got)
	}
	if !slices.Contains(s.env, "AWS_ACCESS_KEY_ID=000000000001") {
		t.Errorf("tofu env lacks the account's access key ID")
	}
}

// TestAWSStackApply: apply re-plans, applies the gated plan, marks the stack
// applied for the wet commit and writes the evidence to the job summary.
func TestAWSStackApply(t *testing.T) {
	s := useStubTofu(t, updateRole)
	dir, flags := awsStackDirs(t)
	gated, applied, summary := filepath.Join(dir, "gated.json"), filepath.Join(dir, "applied"), filepath.Join(dir, "summary.md")
	writeFileAll(t, gated, `{"aws/dev/components/api":[{"address":"module.component.aws_iam_role.ci","action":"update"}],"github":[]}`)
	var stdout, stderr bytes.Buffer
	code := Run(append(append([]string{"aws-stack", "apply"}, flags...), "--fingerprint", gated, "--applied-dir", applied),
		&stdout, &stderr, envOf(map[string]string{"GITHUB_STEP_SUMMARY": summary}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if last := s.runs[len(s.runs)-1]; !strings.HasPrefix(last, "apply -input=false ") || !strings.HasSuffix(last, "aws.dev.components.api.tfplan") {
		t.Errorf("last tofu run = %q, want the saved plan's apply", last)
	}
	if got := stdout.String(); got != "aws/dev/components/api: +0 ~1 -0 ±0: applied\n" {
		t.Errorf("stdout = %q", got)
	}
	if got, err := os.ReadFile(filepath.Join(applied, "aws.dev.components.api")); err != nil || string(got) != "aws/dev/components/api\n" {
		t.Errorf("applied marker = %q, %v", got, err)
	}
	want := "### `aws/dev/components/api`\n\nThe re-plan equals the gated fingerprint, and it applied: +0 ~1 -0 ±0.\n\n- update `module.component.aws_iam_role.ci`\n\n"
	if got, _ := os.ReadFile(summary); string(got) != want {
		t.Errorf("summary =\n%s\nwant\n%s", got, want)
	}
}

// TestAWSStackApplyRefusal: a re-plan that differs from the gated one applies
// nothing and marks nothing, so the stack's render stays old in wet and the
// next reconcile retries it (spec §6.2 step 4).
func TestAWSStackApplyRefusal(t *testing.T) {
	useStubTofu(t, updateRole)
	dir, flags := awsStackDirs(t)
	gated, applied, summary := filepath.Join(dir, "gated.json"), filepath.Join(dir, "applied"), filepath.Join(dir, "summary.md")
	writeFileAll(t, gated, `{"aws/dev/components/api":[]}`)
	var stdout, stderr bytes.Buffer
	code := Run(append(append([]string{"aws-stack", "apply"}, flags...), "--fingerprint", gated, "--applied-dir", applied),
		&stdout, &stderr, envOf(map[string]string{"GITHUB_STEP_SUMMARY": summary}))
	refusal := "aws/dev/components/api: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)\n  not gated: aws/dev/components/api: update module.component.aws_iam_role.ci"
	if code != 1 || stderr.String() != "idp: "+refusal+"\n" {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
	if _, err := os.Stat(applied); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("applied dir: %v, want none", err)
	}
	if got, _ := os.ReadFile(summary); string(got) != "### `aws/dev/components/api`\n\nNothing was applied:\n\n```\n"+refusal+"\n```\n\n" {
		t.Errorf("summary = %q", got)
	}
}

// TestAWSStackRefusesTheStatePassphrase: a job that runs floci shares the
// runner's Docker socket, so it never holds the state passphrase (ADR-0031).
func TestAWSStackRefusesTheStatePassphrase(t *testing.T) {
	for _, name := range []string{"TF_ENCRYPTION", "IDP_STATE_PASSPHRASE"} {
		s := useStubTofu(t, updateRole)
		_, flags := awsStackDirs(t)
		var stdout, stderr bytes.Buffer
		code := Run(append(append([]string{"aws-stack", "plan"}, flags...), "--out-dir", t.TempDir()), &stdout, &stderr, envOf(map[string]string{name: "x"}))
		want := "idp aws-stack: " + name + " is set: a job that runs floci shares the runner's Docker socket, so it holds no secret (ADR-0031)\n"
		if code != 1 || stderr.String() != want || len(s.runs) != 0 {
			t.Errorf("%s: exit %d, stderr %q, tofu runs %v; want 1, %q and none", name, code, stderr.String(), s.runs, want)
		}
	}
}

func TestAWSStackUsageErrors(t *testing.T) {
	_, flags := awsStackDirs(t)
	without := func(flag string) []string {
		i := slices.Index(flags, flag)
		return slices.Concat(flags[:i], flags[i+2:])
	}
	for name, args := range map[string][]string{
		"no mode":              nil,
		"unknown mode":         {"destroy"},
		"plan without out-dir": append([]string{"plan"}, flags...),
		"apply without gate":   append(append([]string{"apply"}, flags...), "--applied-dir", "a"),
		"apply without marker": append(append([]string{"apply"}, flags...), "--fingerprint", "f"),
		"no stack":             append(append([]string{"plan"}, without("--stack")...), "--out-dir", "o"),
		"no endpoint":          append(append([]string{"plan"}, without("--endpoint")...), "--out-dir", "o"),
		"bad account":          append(append([]string{"plan"}, flags...), "--out-dir", "o", "--account", "1"),
		"extra argument":       append(append([]string{"plan"}, flags...), "--out-dir", "o", "extra"),
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"aws-stack"}, args...), &stdout, &stderr, noEnv); code != 2 || !strings.HasPrefix(stderr.String(), "idp aws-stack") {
			t.Errorf("%s: exit %d, stderr %q; want 2 and an idp aws-stack message", name, code, stderr.String())
		}
	}
}
````


- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/cli/ -count=1 -run TestAWSStack`
Expected: build failure, `undefined: newTofu`.

- [ ] **Step 3: Implement**

`internal/cli/awsstack.go` (Create):

````go
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/awsstack"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

// newTofu runs the tofu binary; tests replace it.
var newTofu = func(path string, env []string, log io.Writer) awsstack.Runner {
	return awsstack.Exec{Path: path, Env: env, Log: log}
}

var accountID = regexp.MustCompile(`^[0-9]{12}$`)

// secretNames are what a job that runs floci must never hold (ADR-0031).
var secretNames = []string{"TF_ENCRYPTION", "IDP_STATE_PASSPHRASE"}

// runAWSStack plans one AWS stack against floci for plan-aws, or re-plans and
// applies it for apply-aws (spec §6.1 step 4, §6.2 step 3).
func runAWSStack(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 || (args[0] != "plan" && args[0] != "apply") {
		fmt.Fprintln(stderr, "idp aws-stack: want plan or apply")
		return exitUsage
	}
	mode := args[0]
	fs := flag.NewFlagSet("idp aws-stack "+mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var cfg awsstack.Config
	fs.StringVar(&cfg.Stack, "stack", "", "the AWS stack, as idp diff names it (required)")
	account := fs.String("account", "", "the 12-digit floci account the stack runs as, from idp stacks (required)")
	fs.StringVar(&cfg.New, "new", "", "the fresh render's root (required)")
	fs.StringVar(&cfg.Wet, "wet", "", "wet's rendered/ root; it may be missing (required)")
	fs.StringVar(&cfg.Work, "work", "", "a fresh scratch directory outside both renders (required)")
	fs.StringVar(&cfg.Endpoint, "endpoint", "", "floci's endpoint (required)")
	tofu := fs.String("tofu", "tofu", "the OpenTofu binary")
	outDir := fs.String("out-dir", "", "plan: write the reduced plan into this directory, as idp plan-reduce does (required)")
	gatedFile := fs.String("fingerprint", "", "apply: the fingerprint the gate decided on (required)")
	appliedDir := fs.String("applied-dir", "", "apply: after a successful apply, mark the stack applied in this directory (required)")
	if err := fs.Parse(args[1:]); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp aws-stack %s: unexpected argument %q\n", mode, fs.Arg(0))
		return exitUsage
	}
	if cfg.Stack == "" || *account == "" || cfg.New == "" || cfg.Wet == "" || cfg.Work == "" || cfg.Endpoint == "" {
		fmt.Fprintf(stderr, "idp aws-stack %s: --stack, --account, --new, --wet, --work and --endpoint are required\n", mode)
		return exitUsage
	}
	if !accountID.MatchString(*account) {
		fmt.Fprintf(stderr, "idp aws-stack %s: --account %q is not a 12-digit account ID\n", mode, *account)
		return exitUsage
	}
	if mode == "plan" && *outDir == "" {
		fmt.Fprintln(stderr, "idp aws-stack plan: --out-dir is required")
		return exitUsage
	}
	if mode == "apply" && (*gatedFile == "" || *appliedDir == "") {
		fmt.Fprintln(stderr, "idp aws-stack apply: --fingerprint and --applied-dir are required")
		return exitUsage
	}
	for _, name := range secretNames {
		if env(name) != "" {
			fmt.Fprintf(stderr, "idp aws-stack: %s is set: a job that runs floci shares the runner's Docker socket, so it holds no secret (ADR-0031)\n", name)
			return exitError
		}
	}
	run := newTofu(*tofu, awsstack.Env(os.Environ(), *account), stderr)
	if mode == "plan" {
		return awsStackPlan(cfg, run, *outDir, stdout, stderr, env)
	}
	return awsStackApply(cfg, run, *gatedFile, *appliedDir, stdout, stderr, env)
}

func awsStackPlan(cfg awsstack.Config, run awsstack.Runner, outDir string, stdout, stderr io.Writer, env Env) int {
	res, err := awsstack.Plan(cfg, run)
	if err != nil {
		return fail(stderr, err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(planFileIn(outDir, cfg.Stack), res.Reduced, 0o644); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, countsLine(res.Plan))
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "changes", strconv.Itoa(res.Plan.Counts().Total())); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

func awsStackApply(cfg awsstack.Config, run awsstack.Runner, gatedFile, appliedDir string, stdout, stderr io.Writer, env Env) int {
	f, err := os.Open(gatedFile)
	if err != nil {
		return fail(stderr, err)
	}
	gated, err := plan.ReadFingerprint(f)
	f.Close()
	if err != nil {
		return fail(stderr, err)
	}
	heading := "### `" + cfg.Stack + "`\n\n"
	res, err := awsstack.Apply(cfg, run, gated)
	if err != nil {
		if err := actions.AddSummary(env("GITHUB_STEP_SUMMARY"), heading+"Nothing was applied:\n\n```\n"+err.Error()+"\n```\n"); err != nil {
			fmt.Fprintln(stderr, "idp:", err)
		}
		return fail(stderr, err)
	}
	if err := os.MkdirAll(appliedDir, 0o755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(appliedDir, wetdiff.Escape(cfg.Stack)), []byte(cfg.Stack+"\n"), 0o644); err != nil {
		return fail(stderr, err)
	}
	line := countsLine(res.Plan)
	fmt.Fprintln(stdout, line+": applied")
	var b strings.Builder
	b.WriteString(heading + "The re-plan equals the gated fingerprint, and it applied: " + strings.TrimPrefix(line, cfg.Stack+": ") + ".\n")
	if len(res.Plan.Changes) > 0 {
		b.WriteString("\n")
	}
	for _, c := range res.Plan.Changes {
		fmt.Fprintf(&b, "- %s `%s`\n", c.Action, c.Address)
	}
	if err := actions.AddSummary(env("GITHUB_STEP_SUMMARY"), b.String()); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}
````


`internal/cli/cli.go` (Edit):

1. old:

```go
  render          Render a claims repo into OpenTofu stacks
  diff            List the stacks whose render differs from the wet branch
  stacks          Split the affected stacks into the github stack and the AWS matrix
  plan-reduce     Keep of a stack's plan only the addresses and actions, for an artifact
  plan-summary    Summarize OpenTofu plans as a PR comment and a fingerprint
  gate            Decide whether a reconcile applies automatically or waits for approval
```

new:

```go
  render          Render a claims repo into OpenTofu stacks
  diff            List the stacks whose render differs from the wet branch
  stacks          Split the affected stacks into the github stack and the AWS matrix
  aws-stack       Plan, or re-plan and apply, one AWS stack against floci
  plan-reduce     Keep of a stack's plan only the addresses and actions, for an artifact
  plan-summary    Summarize OpenTofu plans as a PR comment and a fingerprint
  gate            Decide whether a reconcile applies automatically or waits for approval
```

2. old:

```go
		return runDiff(args[1:], stdout, stderr, env)
	case "stacks":
		return runStacks(args[1:], stdout, stderr, env)
	case "plan-reduce":
		return runPlanReduce(args[1:], stdout, stderr, env)
	case "plan-summary":
```

new:

```go
		return runDiff(args[1:], stdout, stderr, env)
	case "stacks":
		return runStacks(args[1:], stdout, stderr, env)
	case "aws-stack":
		return runAWSStack(args[1:], stdout, stderr, env)
	case "plan-reduce":
		return runPlanReduce(args[1:], stdout, stderr, env)
	case "plan-summary":
```


- [ ] **Step 4: Run them to see them pass, and commit**

Run: the command of Step 2. Expected: `ok`. Then `go test ./... && go vet ./... && gofmt -l .` pass.

```bash
git add internal/cli/awsstack.go internal/cli/awsstack_test.go internal/cli/cli.go
git commit -m "feat(cli): add idp aws-stack to plan and apply one aws stack"
```

`Risk: item 2 (the command refuses the state passphrase, ADR-0031)`.

- [ ] **Step 5: Mutation checks** (nothing is committed; restore with `git checkout internal/cli/awsstack.go internal/awsstack/awsstack.go`, then `git status --short` prints nothing). Run the command of Step 2 after each.
1. In `internal/cli/awsstack.go`, replace `		if env(name) != "" {` with `		if false && env(name) != "" {`. Expected: `FAIL: TestAWSStackRefusesTheStatePassphrase`.
2. In `internal/awsstack/awsstack.go`, replace `	if err := sameChanges(cfg.Stack, res.Plan, want); err != nil {` with `	if err := sameChanges(cfg.Stack, res.Plan, want); err != nil && false {`. Expected: `FAIL: TestAWSStackApplyRefusal`.

---

### Task 7: The floci rehearsal — `TestReplay`

**Files:**
- Create: `internal/floci/replay_test.go`

**Interfaces:**
- Consumes: `awsstack.Plan`, `awsstack.Apply`, `awsstack.Exec`, `awsstack.Env`; the floci test's types `floci` (its methods `awsJSON`, `wantMissing`, `wantPolicy`) and `target`, its constants `fixtureDir`, `modulesDir`, `endpointEnv`, `component`, and `trustPolicy`; `claims.Load`, `render.Render`, `render.WriteTree`, `naming.RoleName`, `naming.OIDCProviderARN`.
- Produces: `TestReplay`, which CI's floci job runs with `IDP_FLOCI_ENDPOINT` set; `ci.yaml` is unchanged (D-4).

Each scenario renders its own wet and new trees with the fixture changed, and runs in accounts of its own, so the scenarios share one floci the way the pipelines' jobs each have their own.

- [ ] **Step 1: Write the rehearsal**

`internal/floci/replay_test.go` (Create):

```go
package floci

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/awsstack"
	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
)

// rehearsal is a wet and a new render of the fixture for one scenario of
// TestReplay, in an account of its own, so the scenarios share one floci the
// way the pipelines' jobs each have a floci of their own.
type rehearsal struct {
	root, endpoint string
	runs           int
}

// renderInto renders the fixture, changed by edit, into dir with module sources
// in the modules each scratch directory copies.
func renderInto(t *testing.T, dir string, edit func(*claims.Model)) {
	t.Helper()
	m, diags, err := claims.Load(fixtureDir)
	if err != nil || len(diags) > 0 {
		t.Fatalf("%s: %v %v", fixtureDir, err, diags)
	}
	edit(m)
	files, err := render.Render(m, render.Options{ModulesDir: "../../modules"})
	if err != nil {
		t.Fatal(err)
	}
	if err := render.WriteTree(dir, files); err != nil {
		t.Fatal(err)
	}
}

// newRehearsal renders the new claims, and the wet ones unless wet is nil, as
// the first reconcile has no wet render.
func newRehearsal(t *testing.T, endpoint string, wet, fresh func(*claims.Model)) *rehearsal {
	r := &rehearsal{root: t.TempDir(), endpoint: endpoint}
	if wet != nil {
		renderInto(t, filepath.Join(r.root, "wet"), wet)
	}
	renderInto(t, filepath.Join(r.root, "new"), fresh)
	return r
}

// config is a job's view of stack: a fresh scratch directory holding the
// engine's AWS modules where the rendered relative sources point.
func (r *rehearsal) config(t *testing.T, stack string) awsstack.Config {
	t.Helper()
	r.runs++
	work := filepath.Join(r.root, fmt.Sprintf("work%d", r.runs))
	if err := os.CopyFS(filepath.Join(work, "modules", "aws"), os.DirFS(modulesDir)); err != nil {
		t.Fatal(err)
	}
	return awsstack.Config{Stack: stack, New: filepath.Join(r.root, "new"), Wet: filepath.Join(r.root, "wet"), Work: work, Endpoint: r.endpoint}
}

// inAccount gives dev the account of a scenario.
func inAccount(account string, more ...func(*claims.Model)) func(*claims.Model) {
	return func(m *claims.Model) {
		m.Platform.Environments["dev"].AWS.AccountID = account
		for _, f := range more {
			f(m)
		}
	}
}

// renamedOrg changes what every Component's trust policy names: the org.
func renamedOrg(m *claims.Model) { m.Platform.GitHub.Org = "acme-renamed" }

// withStaging adds an environment, staging, in its own account, with api in it.
func withStaging(account string) func(*claims.Model) {
	return func(m *claims.Model) {
		m.Platform.Environments["staging"] = claims.Environment{AWS: &claims.AWS{AccountID: account, Region: "eu-west-1"}}
		m.Components[0].Environments = append(m.Components[0].Environments, "staging")
	}
}

func wantPlan(t *testing.T, got *plan.Plan, want ...plan.Change) {
	t.Helper()
	if want == nil {
		want = []plan.Change{}
	}
	if !reflect.DeepEqual(got.Changes, want) {
		t.Errorf("%s: changes %v, want %v", got.Stack, got.Changes, want)
	}
}

// The Component stack's resources and the baseline's (modules/aws/*/main.tf).
var (
	repoAddr = "module.component.aws_ecr_repository.this"
	roleAddr = "module.component.aws_iam_role.ci"
	pushAddr = "module.component.aws_iam_role_policy.push"
	oidcAddr = "module.baseline.aws_iam_openid_connect_provider.this"
)

// TestReplay rehearses the pipelines' AWS jobs against floci with awsstack, the
// code that plan-aws and apply-aws run (spec §6.1 step 4, §6.2 step 3;
// ADR-0030): a new stack, a changed one, an unchanged one, an orphan destroyed
// with its wet configuration, and a plan refused because it differs from the
// gated one. It checks the results through the AWS API too.
func TestReplay(t *testing.T) {
	endpoint := os.Getenv(endpointEnv)
	if endpoint == "" {
		t.Skipf("set %s to rehearse the pipelines' AWS jobs against floci (spec §8.4); CI's floci job does", endpointEnv)
	}
	f := &floci{endpoint: endpoint, root: t.TempDir()}
	for _, tool := range []struct {
		name string
		path *string
	}{{"tofu", &f.tofu}, {"aws", &f.aws}} {
		p, err := exec.LookPath(tool.name)
		if err != nil {
			t.Fatalf("%s is not on PATH: %v", tool.name, err)
		}
		*tool.path = p
	}
	runner := func(account string) awsstack.Runner {
		return awsstack.Exec{Path: f.tofu, Env: awsstack.Env(os.Environ(), account), Log: testLog{t}}
	}
	const api = "aws/dev/components/api"

	t.Run("a new stack is planned as a create", func(t *testing.T) {
		const account = "000000000101"
		r := newRehearsal(t, endpoint, nil, inAccount(account))
		// The baseline's job first: it plans and applies nothing, so the
		// Component's job, which applies the baseline, still finds the account
		// as empty as its own floci would be.
		res, err := awsstack.Plan(r.config(t, "aws/dev/_baseline"), runner(account))
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan, plan.Change{Address: oidcAddr, Action: plan.Create})
		res, err = awsstack.Plan(r.config(t, api), runner(account))
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan, plan.Change{Address: repoAddr, Action: plan.Create}, plan.Change{Address: roleAddr, Action: plan.Create}, plan.Change{Address: pushAddr, Action: plan.Create})
	})

	t.Run("a changed stack is planned against its replayed state and applied", func(t *testing.T) {
		const account = "000000000102"
		tg := target{env: "dev", accountID: account, region: "eu-west-1"}
		r := newRehearsal(t, endpoint, inAccount(account), inAccount(account, renamedOrg))
		update := plan.Change{Address: roleAddr, Action: plan.Update}
		res, err := awsstack.Apply(r.config(t, api), runner(account), plan.Fingerprint{api: {update}})
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan, update)
		f.wantTrust(t, tg, "acme-renamed/api")
	})

	t.Run("an unchanged stack plans no change", func(t *testing.T) {
		const account = "000000000103"
		r := newRehearsal(t, endpoint, inAccount(account), inAccount(account))
		res, err := awsstack.Plan(r.config(t, api), runner(account))
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan)
	})

	t.Run("an orphan is destroyed with its wet configuration", func(t *testing.T) {
		const account, staging = "000000000104", "000000000114"
		tg := target{env: "staging", accountID: staging, region: "eu-west-1"}
		r := newRehearsal(t, endpoint, inAccount(account, withStaging(staging)), inAccount(account))
		destroyed := []plan.Change{{Address: repoAddr, Action: plan.Delete}, {Address: roleAddr, Action: plan.Delete}, {Address: pushAddr, Action: plan.Delete}}
		const orphan = "aws/staging/components/api"
		res, err := awsstack.Apply(r.config(t, orphan), runner(staging), plan.Fingerprint{orphan: destroyed})
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan, destroyed...)
		f.wantMissing(t, tg, "the orphan's role after its destroy", "iam", "get-role", "--role-name", naming.RoleName(component, "staging"))
		// The orphan baseline's job runs in a floci of its own: here, an
		// account no other job used.
		const staging2 = "000000000124"
		r = newRehearsal(t, endpoint, inAccount(account, withStaging(staging2)), inAccount(account))
		res, err = awsstack.Plan(r.config(t, "aws/staging/_baseline"), runner(staging2))
		if err != nil {
			t.Fatal(err)
		}
		wantPlan(t, res.Plan, plan.Change{Address: oidcAddr, Action: plan.Delete})
	})

	t.Run("a plan that differs from the gated one is refused", func(t *testing.T) {
		const account = "000000000105"
		tg := target{env: "dev", accountID: account, region: "eu-west-1"}
		r := newRehearsal(t, endpoint, inAccount(account), inAccount(account, renamedOrg))
		_, err := awsstack.Apply(r.config(t, api), runner(account), plan.Fingerprint{api: {}})
		want := api + ": the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)\n  not gated: " + api + ": update " + roleAddr
		if err == nil || err.Error() != want {
			t.Fatalf("err = %v, want %q", err, want)
		}
		f.wantTrust(t, tg, "acme/api")
	})
}

// wantTrust checks that the dev CI role of tg's account trusts repo's
// environment, as the trust policy that modules/aws/component writes.
func (f *floci) wantTrust(t *testing.T, tg target, repo string) {
	t.Helper()
	roleName := naming.RoleName(component, tg.env)
	var role struct {
		Role struct{ AssumeRolePolicyDocument json.RawMessage }
	}
	if !f.awsJSON(t, tg, &role, "iam", "get-role", "--role-name", roleName) {
		t.Fatalf("get-role %s failed", roleName)
	}
	f.wantPolicy(t, "trust policy", role.Role.AssumeRolePolicyDocument, trustPolicy(naming.OIDCProviderARN(tg.accountID), repo, tg.env))
}

// testLog writes tofu's output to the test log.
type testLog struct{ t *testing.T }

func (l testLog) Write(p []byte) (int, error) {
	l.t.Log(string(p))
	return len(p), nil
}
```


- [ ] **Step 2: Run it offline**

Run: `go test ./internal/floci/ -count=1 -v -run TestReplay`
Expected: `--- SKIP: TestReplay` with `set IDP_FLOCI_ENDPOINT to rehearse the pipelines' AWS jobs against floci (spec §8.4); CI's floci job does`, then `ok`. Then `go test ./... && go vet ./... && gofmt -l .` pass. Its RED and GREEN are CI's: PR 2's floci job (watch list).

- [ ] **Step 3: Commit**

```bash
git add internal/floci/replay_test.go
git commit -m "test(floci): rehearse the pipelines' aws jobs against floci"
```

`Risk: item 5 (CI: the floci job runs longer)`.

---

### Task 8: `idp wet-root` — the wet commit of a reconcile

**Files:**
- Create: `internal/cli/wetroot.go`, `internal/cli/wetroot_test.go`
- Modify: `internal/cli/cli.go`

**Interfaces:**
- Consumes: the markers of `idp aws-stack apply` (Task 6), `wetdiff.Reconcilable`, `wetdiff.Escape`, `wetdiff.StackFile`, `actions.SetOutput`.
- Produces: `idp wet-root --out --wet-base --new [--applied-dir] [--github-state [--github-render]]`: under `--out`, `wet-base`, `tfstate/github.tfstate`, the applied renders and `wet-paths`; the output `paths` (Tasks 11, 12).

- [ ] **Step 1: Write the failing tests**

`internal/cli/wetroot_test.go` (Create):

```go
package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wetRootInputs writes what apply-github has when it prepares the wet commit:
// the new render, the github stack's state and the applied-stack markers of
// apply-aws, one per stack that applied.
func wetRootInputs(t *testing.T, applied ...string) (dir string, flags []string) {
	t.Helper()
	dir = t.TempDir()
	for _, stack := range []string{"github", "aws/dev/_baseline", "aws/dev/components/api"} {
		writeFileAll(t, filepath.Join(dir, "new", filepath.FromSlash(stack), "main.tf.json"), `{"stack":"`+stack+`"}`)
		writeFileAll(t, filepath.Join(dir, "new", filepath.FromSlash(stack), ".terraform.lock.hcl"), "# lock")
	}
	writeFileAll(t, filepath.Join(dir, "work", "github.tfstate"), "encrypted state")
	for _, stack := range applied {
		writeFileAll(t, filepath.Join(dir, "applied", strings.ReplaceAll(stack, "/", ".")), stack+"\n")
	}
	return dir, []string{"--out", filepath.Join(dir, "wet-root"), "--wet-base", "0123456789abcdef0123456789abcdef01234567",
		"--new", filepath.Join(dir, "new"), "--applied-dir", filepath.Join(dir, "applied")}
}

func wetRoot(t *testing.T, args ...string) (code int, stdout, stderr, output string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "output")
	var o, e bytes.Buffer
	code = Run(append([]string{"wet-root"}, args...), &o, &e, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	b, _ := os.ReadFile(out)
	return code, o.String(), e.String(), string(b)
}

func wantFile(t *testing.T, path, want string) {
	t.Helper()
	if got, err := os.ReadFile(path); err != nil || string(got) != want {
		t.Errorf("%s = %q, %v; want %q", path, got, err, want)
	}
}

func wantNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s: %v, want it missing", path, err)
	}
}

// TestWetRootCommand: the wet commit holds the github stack's state, its
// render after a successful apply, and the render of each AWS stack that
// applied; an applied orphan's path is listed with no render, so wet-push
// deletes it (spec §6.2 step 4). A stack that did not apply keeps its old
// render in wet.
func TestWetRootCommand(t *testing.T) {
	dir, flags := wetRootInputs(t, "aws/dev/components/api", "aws/staging/components/api")
	code, stdout, stderr, output := wetRoot(t, append(flags, "--github-state", filepath.Join(dir, "work", "github.tfstate"), "--github-render")...)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	root := filepath.Join(dir, "wet-root")
	wantFile(t, filepath.Join(root, "wet-base"), "0123456789abcdef0123456789abcdef01234567\n")
	wantFile(t, filepath.Join(root, "tfstate", "github.tfstate"), "encrypted state")
	wantFile(t, filepath.Join(root, "rendered", "github", "main.tf.json"), `{"stack":"github"}`)
	wantFile(t, filepath.Join(root, "rendered", "aws", "dev", "components", "api", ".terraform.lock.hcl"), "# lock")
	wantNoFile(t, filepath.Join(root, "rendered", "aws", "dev", "_baseline"))
	wantNoFile(t, filepath.Join(root, "rendered", "aws", "staging"))
	wantFile(t, filepath.Join(root, "wet-paths"), "rendered/aws/dev/components/api\nrendered/aws/staging/components/api\nrendered/github\ntfstate/github.tfstate\n")
	want := "rendered/aws/dev/components/api: the applied render\n" +
		"rendered/aws/staging/components/api: deleted, the orphan's destroy applied\n" +
		"rendered/github: the applied render\n" +
		"tfstate/github.tfstate: the github stack's state\n" +
		"wet-root: 4 path(s) to commit\n"
	if stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout, want)
	}
	if output != "paths=4\n" {
		t.Errorf("GITHUB_OUTPUT = %q", output)
	}
}

// TestWetRootAfterAFailedGitHubApply: the state is committed always, since real
// resources may have changed; the github render is not, so the next reconcile
// retries it (spec §6.2).
func TestWetRootAfterAFailedGitHubApply(t *testing.T) {
	dir, flags := wetRootInputs(t)
	code, stdout, stderr, output := wetRoot(t, append(flags, "--github-state", filepath.Join(dir, "work", "github.tfstate"))...)
	if code != 0 || stdout != "tfstate/github.tfstate: the github stack's state\nwet-root: 1 path(s) to commit\n" || output != "paths=1\n" {
		t.Errorf("exit %d, stdout %q, stderr %q, output %q", code, stdout, stderr, output)
	}
	wantNoFile(t, filepath.Join(dir, "wet-root", "rendered"))
}

// TestWetRootWithNothingToCommit: no github stack and no AWS stack that
// applied, as when every apply-aws leg failed, leaves nothing to push.
func TestWetRootWithNothingToCommit(t *testing.T) {
	dir, flags := wetRootInputs(t)
	code, stdout, stderr, output := wetRoot(t, flags...)
	if code != 0 || stdout != "wet-root: 0 path(s) to commit\n" || output != "paths=0\n" {
		t.Errorf("exit %d, stdout %q, stderr %q, output %q", code, stdout, stderr, output)
	}
	wantFile(t, filepath.Join(dir, "wet-root", "wet-paths"), "")
}

// TestWetRootWithoutGitHubStateYet: a first reconcile that changed nothing in
// GitHub has no state to commit, and names no state path, so wet-push never
// deletes one.
func TestWetRootWithoutGitHubStateYet(t *testing.T) {
	dir, flags := wetRootInputs(t)
	code, stdout, _, _ := wetRoot(t, append(flags, "--github-state", filepath.Join(dir, "work", "missing.tfstate"), "--github-render")...)
	if code != 0 || stdout != "rendered/github: the applied render\nwet-root: 1 path(s) to commit\n" {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

func TestWetRootRefusesAForeignMarker(t *testing.T) {
	for name, marker := range map[string][2]string{
		"github":        {"github", "github\n"},
		"misnamed":      {"aws.dev._baseline", "aws/dev/components/api\n"},
		"not a stack":   {"aws.dev.workspaces.db", "aws/dev/workspaces/db\n"},
		"no final line": {"aws.dev._baseline", "aws/dev/_baseline"},
	} {
		t.Run(name, func(t *testing.T) {
			dir, flags := wetRootInputs(t)
			writeFileAll(t, filepath.Join(dir, "applied", marker[0]), marker[1])
			code, stdout, stderr, _ := wetRoot(t, flags...)
			want := "idp wet-root: " + filepath.Join(dir, "applied", marker[0]) + ": not an applied-stack marker: want a file named after an AWS stack, each / as ., that holds the stack and a newline\n"
			if code != 1 || stderr != want || stdout != "" {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1 and %q", code, stdout, stderr, want)
			}
			wantNoFile(t, filepath.Join(dir, "wet-root"))
		})
	}
}

func TestWetRootUsageErrors(t *testing.T) {
	dir, flags := wetRootInputs(t)
	if err := os.MkdirAll(filepath.Join(dir, "wet-root"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		args []string
		code int
	}{
		"no flags":                {nil, exitUsage},
		"render without state":    {append(flags[2:], "--out", filepath.Join(dir, "x"), "--github-render"), exitUsage},
		"extra argument":          {append(flags, "extra"), exitUsage},
		"an existing output root": {flags, exitError},
	} {
		code, _, stderr, _ := wetRoot(t, c.args...)
		if code != c.code || !strings.HasPrefix(stderr, "idp wet-root") {
			t.Errorf("%s: exit %d, stderr %q; want %d and an idp wet-root message", name, code, stderr, c.code)
		}
	}
}
```


- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/cli/ -count=1 -run TestWetRoot`
Expected: `FAIL` for the six `TestWetRoot*` tests, each with `idp: unknown command "wet-root"`.

- [ ] **Step 3: Implement**

`internal/cli/wetroot.go` (Create):

```go
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

// runWetRoot prepares apply-github's wet commit (spec §6.2 step 4): a root
// holding wet-base, the github stack's state, its render when it applied, and
// the render of each AWS stack apply-aws marked applied, plus wet-paths, the
// paths idp wet-push commits, one per line. An applied orphan's path is listed
// with no render under the root, so wet-push deletes it. The root is saved as
// the idp-wet-root artifact, which the recover workflow pushes again.
func runWetRoot(args []string, stdout, stderr io.Writer, env Env) int {
	flags := flag.NewFlagSet("idp wet-root", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "", "the wet root to write; it must not exist yet (required)")
	base := flags.String("wet-base", "", "the wet commit the reconcile planned against, written to wet-base (required)")
	fresh := flags.String("new", "", "the new render's root (required)")
	appliedDir := flags.String("applied-dir", "", "apply-aws's markers, one per AWS stack that applied; it may be missing")
	state := flags.String("github-state", "", "the github stack's state; committed when the file exists")
	githubRender := flags.Bool("github-render", false, "commit the github stack's render: its apply succeeded or had nothing to apply (needs --github-state)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "idp wet-root: unexpected argument %q\n", flags.Arg(0))
		return exitUsage
	}
	if *out == "" || *base == "" || *fresh == "" {
		fmt.Fprintln(stderr, "idp wet-root: --out, --wet-base and --new are required")
		return exitUsage
	}
	if *githubRender && *state == "" {
		fmt.Fprintln(stderr, "idp wet-root: --github-render needs --github-state: the github stack's render is never committed without its state")
		return exitUsage
	}
	applied, err := readMarkers(*appliedDir)
	if err != nil {
		fmt.Fprintln(stderr, "idp wet-root:", err)
		return exitError
	}
	if _, err := os.Stat(*out); !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "idp wet-root: --out %s already exists: give a fresh directory\n", *out)
		return exitError
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(*out, "wet-base"), []byte(*base+"\n"), 0o644); err != nil {
		return fail(stderr, err)
	}
	notes := map[string]string{}
	if *state != "" {
		b, err := os.ReadFile(*state)
		switch {
		case err == nil:
			if err := writeFileIn(*out, githubStatePath, b); err != nil {
				return fail(stderr, err)
			}
			notes[githubStatePath] = "the github stack's state"
		case !errors.Is(err, fs.ErrNotExist):
			return fail(stderr, err)
		}
	}
	if *githubRender {
		applied = append(applied, "github")
	}
	for _, stack := range applied {
		rel := path.Join("rendered", stack)
		src := filepath.Join(*fresh, filepath.FromSlash(stack))
		if _, err := os.Stat(filepath.Join(src, wetdiff.StackFile)); errors.Is(err, fs.ErrNotExist) {
			notes[rel] = "deleted, the orphan's destroy applied"
			continue
		} else if err != nil {
			return fail(stderr, err)
		}
		if err := os.CopyFS(filepath.Join(*out, filepath.FromSlash(rel)), os.DirFS(src)); err != nil {
			return fail(stderr, err)
		}
		notes[rel] = "the applied render"
	}
	paths := slices.Sorted(maps.Keys(notes))
	var list strings.Builder
	for _, p := range paths {
		fmt.Fprintf(stdout, "%s: %s\n", p, notes[p])
		list.WriteString(p + "\n")
	}
	if err := os.WriteFile(filepath.Join(*out, "wet-paths"), []byte(list.String()), 0o644); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "wet-root: %d path(s) to commit\n", len(paths))
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "paths", strconv.Itoa(len(paths))); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

// githubStatePath is the one state file wet holds (spec §5.4).
const githubStatePath = "tfstate/github.tfstate"

// readMarkers reads apply-aws's applied-stack markers: each a file named after
// an AWS stack, each / as ., holding the stack and a newline. A missing
// directory holds none.
func readMarkers(dir string) ([]string, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var stacks []string
	for _, e := range entries {
		file := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		stack, ok := strings.CutSuffix(string(b), "\n")
		if !ok || !strings.HasPrefix(stack, "aws/") || !wetdiff.Reconcilable(stack) || wetdiff.Escape(stack) != e.Name() {
			return nil, fmt.Errorf("%s: not an applied-stack marker: want a file named after an AWS stack, each / as ., that holds the stack and a newline", file)
		}
		stacks = append(stacks, stack)
	}
	slices.Sort(stacks)
	return stacks, nil
}

// writeFileIn writes data at the slash path rel under root.
func writeFileIn(root, rel string, data []byte) error {
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
```


`internal/cli/cli.go` (Edit):

1. old:

```go
  gate            Decide whether a reconcile applies automatically or waits for approval
  comment         Create or update the sticky plan comment on a pull request
  issue           Open or close a labelled issue (drift, failed wet pushes)
  wet-push        Commit files to the wet branch in one commit
  encryption-env  Export TF_ENCRYPTION for OpenTofu from IDP_STATE_PASSPHRASE
  bootstrap       Create or verify the protections an org needs before the IDP runs
```

new:

```go
  gate            Decide whether a reconcile applies automatically or waits for approval
  comment         Create or update the sticky plan comment on a pull request
  issue           Open or close a labelled issue (drift, failed wet pushes)
  wet-root        Prepare the wet commit of a reconcile: the applied renders and the state
  wet-push        Commit files to the wet branch in one commit
  encryption-env  Export TF_ENCRYPTION for OpenTofu from IDP_STATE_PASSPHRASE
  bootstrap       Create or verify the protections an org needs before the IDP runs
```

2. old:

```go
		return runComment(args[1:], stdout, stderr, env)
	case "issue":
		return runIssue(args[1:], stdout, stderr, env)
	case "wet-push":
		return runWetPush(args[1:], stdout, stderr, env)
	case "encryption-env":
```

new:

```go
		return runComment(args[1:], stdout, stderr, env)
	case "issue":
		return runIssue(args[1:], stdout, stderr, env)
	case "wet-root":
		return runWetRoot(args[1:], stdout, stderr, env)
	case "wet-push":
		return runWetPush(args[1:], stdout, stderr, env)
	case "encryption-env":
```


- [ ] **Step 4: Run them to see them pass, and commit**

Run: the command of Step 2. Expected: `ok`. Then `go test ./... && go vet ./... && gofmt -l .` pass.

```bash
git add internal/cli/wetroot.go internal/cli/wetroot_test.go internal/cli/cli.go
git commit -m "feat(cli): add idp wet-root to prepare the wet commit of a reconcile"
```

`Risk: item 1 (it decides what reaches wet once Task 11 wires it)`.

- [ ] **Step 5: Mutation checks** (nothing is committed; restore each with `git checkout internal/cli/wetroot.go`, then `git status --short` prints nothing). Run the command of Step 2 after each.
1. Replace `			notes[rel] = "deleted, the orphan's destroy applied"` with `			notes["x"] = "deleted, the orphan's destroy applied"`. Expected: `FAIL: TestWetRootCommand`.
2. Replace ` || wetdiff.Escape(stack) != e.Name() {` with ` {`. Expected: `FAIL: TestWetRootRefusesAForeignMarker/misnamed`.
3. Replace `	if *githubRender {` with `	if true {`. Expected: `FAIL: TestWetRootAfterAFailedGitHubApply` and `TestWetRootWithNothingToCommit`.

---

### Task 9: `docs/cli.md`

**Files:**
- Modify: `docs/cli.md`

**Interfaces:**
- Consumes: the commands of Tasks 2 to 8, as `idp help` lists them.
- Produces: the reference that `docs/pipelines.md` links (Task 13).

- [ ] **Step 1: Document the commands** (exact replacements with Edit)

`docs/cli.md` (Edit):

1. old:

````markdown

Exit codes: 0 done; 1 the trees could not be read or `--new` is not a render; 2 usage error.

## plan-summary

Summarizes OpenTofu plans as a pull request comment and a fingerprint.

```text
idp plan-summary --stacks JSON --head-sha SHA --comment-out FILE --fingerprint-out FILE [--run-url URL] [--plan STACK=FILE]...
```

- `--stacks`: required. The `affected` JSON array written by `idp diff` (for example `["github"]`, or `[]` when nothing is affected). The set of `--plan` stacks must equal this set exactly. A missing plan must fail here, never read as "no changes": the gate and the reconcile would otherwise decide on a partial view and apply a plan nobody saw.
````

new:

````markdown

Exit codes: 0 done; 1 the trees could not be read or `--new` is not a render; 2 usage error.

## stacks

Splits the affected stacks into the `github` stack and the AWS matrix.

```text
idp stacks --affected JSON --wet DIR [--dir DIR]
```

- `--affected`: required. The `affected` JSON array of `idp diff`.
- `--wet`: required. The `rendered/` directory of the wet branch checkout; it may not exist yet.
- `--dir`: the claims repo (default `.`). It is read only when an AWS stack is affected.

This engine version reconciles `github`, `aws/<env>/_baseline` and `aws/<env>/components/<name>`, the names the claims schemas allow; any other affected stack, such as a Workspace (`aws/<env>/workspaces/<name>`, Phase 3), fails the command and lists what it does not reconcile. More than 256 AWS stacks, the jobs one matrix runs, fail it too (spec §11 risk 3).

Each AWS stack runs as its environment's account (ADR-0004): from the claims' platform config, or, for an environment the claims removed, from the OIDC provider ARN in the wet render of one of its Component stacks. An orphan baseline whose environment had no Component stack runs as `000000000000`: its configuration names no account.

For example, after `staging` (account `000000000002`) was removed and `api` changed in `dev`:

```text
aws/dev/components/api: account 000000000001
aws/staging/_baseline: account 000000000002
aws/staging/components/api: account 000000000002
stacks: github true, 3 AWS stack(s)
```

Writes: one `STACK: account ID` line per AWS stack and a summary line. Step outputs `github` (`true` when the `github` stack is affected) and `aws`, the matrix: a JSON array of `{"stack", "artifact", "account"}`, sorted by stack, where `artifact` is the stack with each `/` as `.`, for artifact names (`aws/dev/_baseline` is `aws.dev._baseline`). `[]` when no AWS stack is affected.

Exit codes: 0 done; 1 a stack this version does not reconcile, too many AWS stacks, or invalid claims; 2 usage error.

## aws-stack

Plans one AWS stack against floci, or re-plans and applies it, as the pipelines' `plan-aws` and `apply-aws` jobs do (spec §6.1 step 4, §6.2 step 3).

```text
idp aws-stack plan  --stack STACK --account ID --new DIR --wet DIR --work DIR --endpoint URL --out-dir DIR [--tofu PATH]
idp aws-stack apply --stack STACK --account ID --new DIR --wet DIR --work DIR --endpoint URL --fingerprint FILE --applied-dir DIR [--tofu PATH]
```

- `--stack`: an AWS stack, `aws/<env>/_baseline` or `aws/<env>/components/<name>`.
- `--account`: the 12-digit account the stack runs as, from `idp stacks`.
- `--new`: the fresh render's root. A stack missing from it is an orphan.
- `--wet`: the wet branch's `rendered/` directory. It may not exist yet.
- `--work`: a scratch directory outside both renders, with no `rendered/` in it yet. OpenTofu runs only there; the renders are never written.
- `--endpoint`: floci's endpoint, written into each stack's `floci_override.tf.json` (spec §5.6).
- `--out-dir` (plan): where to write the stack's reduced plan, as `idp plan-reduce` does.
- `--fingerprint` (apply): the fingerprint the gate decided on.
- `--applied-dir` (apply): where to mark the stack applied, after a successful apply.
- `--tofu`: the OpenTofu binary (default `tofu`).

In `--work/rendered`, laid out like a render so relative module sources resolve, it:

1. for a Component stack, applies its environment's `_baseline`: the wet version, or the new one when wet has none;
2. applies the wet version of the stack, when wet has one;
3. plans the new version in the same directory, against the state step 2 wrote (ADR-0030), or, for an orphan, plans a destroy with the wet configuration.

The replays of steps 1 and 2 apply with `-auto-approve`: they only rebuild floci's state. `apply` then requires the plan's changes to equal the gated fingerprint's for the stack, both ways, and applies the saved plan; a stack with no entry in the fingerprint, or any difference, applies nothing:

```text
idp: aws/dev/components/api: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)
  not gated: aws/dev/components/api: update module.component.aws_iam_role.ci
```

OpenTofu runs with `AWS_ACCESS_KEY_ID` set to the account (ADR-0004), a fixed secret access key that floci does not check, `TF_IN_AUTOMATION=1` and `TF_INPUT=0`; its output goes to stderr. The command refuses to run when `TF_ENCRYPTION` or `IDP_STATE_PASSPHRASE` is set: a job that runs floci shares the runner's Docker socket, so it holds no secret (ADR-0031).

Writes: `plan` writes the reduced plan, prints `STACK: +create ~update -delete ±replace`, and sets the step output `changes`. `apply` prints the same line with `: applied`, writes the file `STACK` (each `/` as `.`) into `--applied-dir`, holding the stack and a newline, and appends the evidence to the job summary: the changes it applied, or why nothing was applied.

Exit codes: 0 done; 1 OpenTofu failed, the plan differs from the gated one, a secret is set, or a stack, render or work directory is not usable; 2 usage error.

## plan-reduce

Keeps of a stack's plan only the addresses and actions, for an artifact.

```text
idp plan-reduce --stack STACK --plan FILE --out-dir DIR
```

- `--stack`: a stack this engine version reconciles (see `idp stacks`).
- `--plan`: the `tofu show -json` output of the stack.
- `--out-dir`: where to write the reduced plan, as `STACK.json` with each `/` as `.` (`aws.dev._baseline.json`).

`tofu show -json` holds every value the plan sets, in plaintext. The reduced plan keeps the format version and each resource change's address and actions, which is all the fingerprint reads, so it is what crosses jobs as an artifact ([ADR-0031](adr/0031-floci-jobs-hold-no-secret.md)). `idp plan-summary` reads it back to the same plan.

Writes: the reduced plan, one line of JSON; a `STACK: +create ~update -delete ±replace` line; step output `changes`.

Exit codes: 0 done; 1 the plan could not be read or parsed (the same checks as `plan-summary`), or the file could not be written; 2 usage error.

## plan-summary

Summarizes OpenTofu plans as a pull request comment and a fingerprint.

```text
idp plan-summary --stacks JSON --head-sha SHA --comment-out FILE --fingerprint-out FILE [--run-url URL] [--plan STACK=FILE]...
idp plan-summary --stacks JSON --head-sha SHA --comment-out FILE --fingerprint-out FILE [--run-url URL] --plan-dir DIR
```

- `--stacks`: required. The `affected` JSON array written by `idp diff` (for example `["github"]`, or `[]` when nothing is affected). The set of `--plan` stacks must equal this set exactly. A missing plan must fail here, never read as "no changes": the gate and the reconcile would otherwise decide on a partial view and apply a plan nobody saw.
````

2. old:

```markdown
- `--fingerprint-out`: where to write the fingerprint JSON.
- `--run-url`: link to the workflow run, shown in the comment.
- `--plan STACK=FILE`: the `tofu show -json` output of one stack. Repeatable; a stack may be given only once.

Reads: each plan file. A plan with an action the fingerprint does not know is a failure, never treated as harmless ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).
```

new:

```markdown
- `--fingerprint-out`: where to write the fingerprint JSON.
- `--run-url`: link to the workflow run, shown in the comment.
- `--plan STACK=FILE`: the `tofu show -json` output of one stack. Repeatable; a stack may be given only once.
- `--plan-dir DIR`: instead of `--plan`, read each affected stack's plan from `DIR/STACK.json`, with each `/` as `.`, as `idp plan-reduce` and `idp aws-stack plan` write them. A missing file fails the command.

Reads: each plan file. A plan with an action the fingerprint does not know is a failure, never treated as harmless ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).
```

3. old:

```markdown
- One `STACK: +create ~update -delete ±replace` line per stack, and a `plan-summary: N change(s), destructive: BOOL` line.
- Step outputs `changes` (the number of changes) and `destructive` (`true` or `false`: any delete or replace).

Exit codes: 0 done; 1 a plan could not be read or parsed, or a file could not be written; 2 usage error (a missing flag, a `--head-sha` that is not a full SHA, a stack given twice, `--stacks` that is not a JSON array of unique names, an affected stack with no `--plan`, a `--plan` for a stack that is not in `--stacks`).

## gate
```

new:

```markdown
- One `STACK: +create ~update -delete ±replace` line per stack, and a `plan-summary: N change(s), destructive: BOOL` line.
- Step outputs `changes` (the number of changes) and `destructive` (`true` or `false`: any delete or replace).

Exit codes: 0 done; 1 a plan could not be read or parsed, or a file could not be written; 2 usage error (a missing flag, a `--head-sha` that is not a full SHA, a stack given twice, `--stacks` that is not a JSON array of unique names, an affected stack with no `--plan`, a `--plan` for a stack that is not in `--stacks`, `--plan` and `--plan-dir` together).

## gate
```

4. old:

```markdown

Exit codes: 0 done (including nothing to close); 1 GitHub failed; 2 usage error (including no token).

## wet-push

Commits files to the wet branch in one commit.
```

new:

````markdown

Exit codes: 0 done (including nothing to close); 1 GitHub failed; 2 usage error (including no token).

## wet-root

Prepares the wet commit of a reconcile (spec §6.2 step 4).

```text
idp wet-root --out DIR --wet-base SHA --new DIR [--applied-dir DIR] [--github-state FILE [--github-render]]
```

- `--out`: the wet root to write. It must not exist yet.
- `--wet-base`: the wet commit the reconcile planned against, written to `wet-base` for the `recover` workflow.
- `--new`: the reconcile's render.
- `--applied-dir`: the markers of `idp aws-stack apply`, one per AWS stack that applied. It may be missing.
- `--github-state`: the `github` stack's state. Committed when the file exists, even when the apply failed.
- `--github-render`: commit the `github` stack's render: its apply succeeded or had nothing to apply. It needs `--github-state`.

Writes, under `--out`: `wet-base`, `tfstate/github.tfstate`, `rendered/github` when asked, `rendered/STACK` for each AWS stack that applied, and `wet-paths`, the paths for `idp wet-push`, one per line. An applied orphan's path is listed with no render, so `wet-push` deletes it; a stack that did not apply keeps its old render in wet. Prints one `PATH: why` line per path and `wet-root: N path(s) to commit`; step output `paths`, the count.

Exit codes: 0 done; 1 a marker is not one `idp aws-stack apply` writes, `--out` exists, or a file could not be read or written; 2 usage error.

## wet-push

Commits files to the wet branch in one commit.
````

5. old:

```markdown

- A path that is a single file on the branch (the state file, for example) is never deleted: if the local copy is missing, the command fails.
- `.terraform` directories are never synced, and a path that contains one is rejected.
- A truncated tree listing is refused, because deletions cannot be computed from a partial listing.

Prints `wet-push: SHA (N changed, M deleted)`, or `wet-push: no changes`. Step output `commit`: the new commit SHA, or empty when nothing changed (no commit is made then).
```

new:

```markdown

- A path that is a single file on the branch (the state file, for example) is never deleted: if the local copy is missing, the command fails.
- `.terraform` directories are never synced, and a path that contains one is rejected.
- No state file is committed but `tfstate/github.tfstate`: a file named `*.tfstate`, or a copy such as `terraform.tfstate.backup`, anywhere else under a path fails the command before any write. An AWS stack's state stays in its job ([ADR-0030](adr/0030-aws-stack-state-stays-in-the-job.md)).
- A truncated tree listing is refused, because deletions cannot be computed from a partial listing.

Prints `wet-push: SHA (N changed, M deleted)`, or `wet-push: no changes`. Step output `commit`: the new commit SHA, or empty when nothing changed (no commit is made then).
```


- [ ] **Step 2: Check and commit** — every command in `idp help` has a section, in its order: `rg -n '^## ' docs/cli.md` lists `validate`, `render`, `diff`, `stacks`, `aws-stack`, `plan-reduce`, `plan-summary`, `gate`, `comment`, `issue`, `wet-root`, `wet-push`, `encryption-env`, `bootstrap`, `version`. The naming check prints nothing.

```bash
git add docs/cli.md
git commit -m "docs(cli): document stacks, aws-stack, plan-reduce and wet-root"
```

`Risk: none`.

- [ ] **Step 3: PR 2** (D-12; side effect 1) — as Task 4 Step 6, with:
  - `git diff --stat main..HEAD` listing the 14 files of Tasks 5 to 9;
  - the title `feat: phase 2e plan and apply one aws stack against floci` and the body "Part 2 of 3 of docs/superpowers/plans/2026-11-18-phase-2e-aws-pipelines.md: internal/awsstack replays wet's baseline and the stack's wet version in a scratch copy, plans the new version against that state (or an orphan's destroy with its wet config), and applies only a re-plan equal to the gated fingerprint; idp aws-stack runs it and refuses any secret (ADR-0031); idp wet-root prepares the wet commit; the floci job rehearses the sequence (TestReplay). The workflows change in part 3. No release.";
  - the floci job's log showing `--- PASS: TestReplay` and its five subtests (watch list); the review package adds ADR-0004 and D-3, D-4, D-14 to D-16.

---

### Task 10: `pr.yaml` — `plan-aws` and the reduced-plan comment

**Files:**
- Modify: `.github/workflows/pr.yaml`, `internal/workflowspec/pr_test.go`
- Create: `internal/workflowspec/floci_test.go`

**Interfaces:**
- Consumes: `idp stacks` (Task 3), `idp plan-reduce` and `plan-summary --plan-dir` (Task 4), `idp aws-stack plan` (Task 6); `ci.yaml`'s floci job; the helpers `loadWorkflow`, `mustJob`, `mustStep`, `jobNode`, `mapValue`, `scalars`, `reusableWorkflows`, `wantPerms`, `wantEnv`, `wantWith`, `wantRun`.
- Produces: the PR jobs `plan` (outputs `affected`, `aws`), `plan-aws`, `comment`, `verdict`; `TestFlociJobsHoldNoSecret` and `checkAWSMatrix` (Task 11).

- [ ] **Step 1: Branch again from `main`** — `git switch -c feat/phase-2e-aws-pipelines` (D-12).

- [ ] **Step 2: Write the failing tests**

`internal/workflowspec/floci_test.go` (Create): the list of jobs that run floci starts with `pr.yaml`'s; Task 11 adds the reconcile's.

```go
package workflowspec

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// TestFlociJobsHoldNoSecret: floci's ECR runs a registry container, so floci
// reaches the runner's Docker socket (ADR-0004), and with it every file and
// environment variable of its job. A job that runs floci therefore holds no
// secret, no token beyond contents: read, no environment, and saves no cache
// (ADR-0031). Its floci is CI's, pinned by digest, which TestCIFlociJob pins.
func TestFlociJobsHoldNoSecret(t *testing.T) {
	ci := mustJob(t, loadWorkflow(t, ciPath), "floci")
	ciFloci := ci.Services["floci"]
	_, ciReady := mustStep(t, "ci floci", ci.Steps, "floci-ready")
	var found []string
	for _, path := range reusableWorkflows(t) {
		w := loadWorkflow(t, path)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			j := w.Jobs[id]
			s, ok := j.Services["floci"]
			if !ok {
				if len(j.Services) > 0 {
					t.Errorf("%s %s: a service other than floci", path, id)
				}
				continue
			}
			where := path + " " + id
			found = append(found, where)
			if s.Image != ciFloci.Image || !slices.Equal(s.Ports, ciFloci.Ports) || !slices.Equal(s.Volumes, ciFloci.Volumes) || len(s.Env) > 0 || s.Options != "" {
				t.Errorf("%s: floci = %+v, want CI's %+v", where, s, ciFloci)
			}
			if len(j.Services) != 1 {
				t.Errorf("%s: services besides floci", where)
			}
			if j.Environment != "" || len(j.Env) > 0 {
				t.Errorf("%s: environment %q, env %v; want none (ADR-0031)", where, j.Environment, j.Env)
			}
			wantPerms(t, where, j.Permissions, map[string]string{"contents": "read"})
			for _, v := range scalars(jobNode(w, id)) {
				for _, banned := range []string{"secrets.", "github.token", "steps.encryption", "IDP_STATE_PASSPHRASE", "TF_ENCRYPTION"} {
					if strings.Contains(v, banned) {
						t.Errorf("%s: %q names %s: a job that runs floci holds no secret (ADR-0031)", where, v, banned)
					}
				}
			}
			_, ready := mustStep(t, where, j.Steps, "floci-ready")
			if ready.Run != ciReady.Run {
				t.Errorf("%s: floci-ready differs from CI's floci-ready", where)
			}
		}
	}
	want := []string{
		".github/workflows/pr.yaml plan-aws",
	}
	if slices.Sort(found); !slices.Equal(found, want) {
		t.Errorf("jobs that run floci = %v, want %v", found, want)
	}
}

// checkAWSMatrix pins the AWS stacks' matrix (spec §11 risk 3): one job per
// stack from idp stacks, which refuses more than a matrix runs; every leg
// finishes even when another fails; at most ten floci at a time; and no job,
// so no floci, when no AWS stack is affected.
func checkAWSMatrix(t *testing.T, w *workflow, id string) {
	t.Helper()
	j := mustJob(t, w, id)
	if want := id + " ${{ matrix.stack }}"; j.Name != want {
		t.Errorf("%s name = %q, want %q", id, j.Name, want)
	}
	if !strings.Contains(j.If, "needs.plan.outputs.aws != '[]'") {
		t.Errorf("%s if = %q: it must skip the job when no AWS stack is affected, since an empty matrix fails", id, j.If)
	}
	strategy := mapValue(jobNode(w, id), "strategy")
	got := map[string]string{}
	for _, key := range []string{"fail-fast", "max-parallel"} {
		if v := mapValue(strategy, key); v != nil {
			got[key] = v.Value
		}
	}
	if v := mapValue(mapValue(strategy, "matrix"), "include"); v != nil {
		got["matrix.include"] = v.Value
	}
	want := map[string]string{"fail-fast": "false", "max-parallel": "10", "matrix.include": "${{ fromJSON(needs.plan.outputs.aws) }}"}
	if !maps.Equal(got, want) || len(mapValue(strategy, "matrix").Content) != 2 {
		t.Errorf("%s strategy = %v, want %v and nothing else in the matrix", id, got, want)
	}
	_, dl := mustStep(t, id, j.Steps, "download")
	wantWith(t, id+" download", dl, map[string]string{"name": "idp-renders", "path": "${{ runner.temp }}/idp/renders"})
}
```


`internal/workflowspec/pr_test.go` (Edit):

1. old:

```go
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	wantOptionalSecrets(t, prPath, w.call(t), "IDP_READER_PRIVATE_KEY", "IDP_STATE_PASSPHRASE")
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"comment", "plan", "validate", "verdict"}) {
		t.Errorf("jobs = %v", got)
	}
}
```

new:

```go
		t.Errorf("triggers = %v, want only workflow_call", got)
	}
	wantOptionalSecrets(t, prPath, w.call(t), "IDP_READER_PRIVATE_KEY", "IDP_STATE_PASSPHRASE")
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"comment", "plan", "plan-aws", "validate", "verdict"}) {
		t.Errorf("jobs = %v", got)
	}
}
```

2. old:

```go
	w := loadWorkflow(t, prPath)
	wantPerms(t, "validate", mustJob(t, w, "validate").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "plan", mustJob(t, w, "plan").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "comment", mustJob(t, w, "comment").Permissions, map[string]string{"contents": "read", "pull-requests": "write"})
	wantPerms(t, "verdict", mustJob(t, w, "verdict").Permissions, map[string]string{})
}
```

new:

```go
	w := loadWorkflow(t, prPath)
	wantPerms(t, "validate", mustJob(t, w, "validate").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "plan", mustJob(t, w, "plan").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "plan-aws", mustJob(t, w, "plan-aws").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "comment", mustJob(t, w, "comment").Permissions, map[string]string{"contents": "read", "pull-requests": "write"})
	wantPerms(t, "verdict", mustJob(t, w, "verdict").Permissions, map[string]string{})
}
```

3. old:

```go
	if !slices.Equal(plan.Needs, strlist{"validate"}) {
		t.Errorf("plan needs = %v, want [validate]", plan.Needs)
	}
	if got := mustJob(t, w, "comment").Needs; !slices.Equal(got, strlist{"plan"}) {
		t.Errorf("comment needs = %v, want [plan]", got)
	}
}
```

new:

```go
	if !slices.Equal(plan.Needs, strlist{"validate"}) {
		t.Errorf("plan needs = %v, want [validate]", plan.Needs)
	}
	if got := mustJob(t, w, "plan-aws").Needs; !slices.Equal(got, strlist{"plan"}) {
		t.Errorf("plan-aws needs = %v, want [plan]: it runs only after the same-repo plan job", got)
	}
	c := mustJob(t, w, "comment")
	if got := slices.Sorted(slices.Values(c.Needs)); !slices.Equal(got, []string{"plan", "plan-aws"}) {
		t.Errorf("comment needs = %v, want [plan plan-aws]", got)
	}
	// A PR without AWS stacks skips plan-aws; the comment still runs.
	if want := "always() && needs.plan.result == 'success' && (needs.plan-aws.result == 'success' || needs.plan-aws.result == 'skipped')"; c.If != want {
		t.Errorf("comment if = %q, want %q", c.If, want)
	}
}
```

4. old:

```go
	if v.If != "always()" {
		t.Errorf("verdict if = %q, want always(): it must report even when a job failed or was skipped", v.If)
	}
	if got := slices.Sorted(slices.Values(v.Needs)); !slices.Equal(got, []string{"comment", "plan", "validate"}) {
		t.Errorf("verdict needs = %v", got)
	}
	_, s := mustStep(t, "verdict", v.Steps, "decide")
```

new:

```go
	if v.If != "always()" {
		t.Errorf("verdict if = %q, want always(): it must report even when a job failed or was skipped", v.If)
	}
	if got := slices.Sorted(slices.Values(v.Needs)); !slices.Equal(got, []string{"comment", "plan", "plan-aws", "validate"}) {
		t.Errorf("verdict needs = %v", got)
	}
	_, s := mustStep(t, "verdict", v.Steps, "decide")
```

5. old:

```go
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
```

new:

```go
		"SAME_REPO": "${{ github.event.pull_request.head.repo.full_name == github.repository }}",
		"VALIDATE":  "${{ needs.validate.result }}",
		"PLAN":      "${{ needs.plan.result }}",
		"PLAN_AWS":  "${{ needs.plan-aws.result }}",
		"COMMENT":   "${{ needs.comment.result }}",
	})
	wantRun(t, "verdict decide", s,
		`if [ "$SAME_REPO" != true ]; then`,
		"a maintainer must push this branch to the repo",
		`if [ "$VALIDATE" != success ] || [ "$PLAN" != success ] || [ "$COMMENT" != success ]; then`,
		`if [ "$PLAN_AWS" != success ] && [ "$PLAN_AWS" != skipped ]; then`,
	)
}
```

6. old:

```go
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

new:

```go
	_, d := mustStep(t, "plan", plan.Steps, "diff")
	wantRun(t, "plan diff", d, `idp diff --new "$RUNNER_TEMP/idp/work/rendered" --wet .idp/wet/rendered`)
	_, st := mustStep(t, "plan", plan.Steps, "stacks")
	wantEnv(t, "plan stacks", st, map[string]string{"AFFECTED": "${{ steps.diff.outputs.affected }}"})
	wantRun(t, "plan stacks", st, `idp stacks --affected "$AFFECTED" --wet .idp/wet/rendered`)
	want := map[string]string{
		"affected": "${{ steps.diff.outputs.affected }}",
		"aws":      "${{ steps.stacks.outputs.aws }}",
	}
	if !maps.Equal(map[string]string(plan.Outputs), want) {
		t.Errorf("plan outputs = %v, want %v", plan.Outputs, want)
	}
	_, s := mustStep(t, "comment", mustJob(t, w, "comment").Steps, "summary")
	if s.If != "" {
		t.Errorf("summary if = %q: it must run whatever is affected, so a PR with no changes still gets its comment", s.If)
	}
	wantEnv(t, "comment summary", s, map[string]string{
		"AFFECTED": "${{ needs.plan.outputs.affected }}",
		"HEAD_SHA": "${{ github.event.pull_request.head.sha }}",
	})
	wantRun(t, "comment summary", s, `--stacks "$AFFECTED"`, `--head-sha "$HEAD_SHA"`, `--plan-dir "$RUNNER_TEMP/idp/plans"`)
}

func TestPRCommentPostsTheSavedComment(t *testing.T) {
	w := loadWorkflow(t, prPath)
	c := mustJob(t, w, "comment")
	iDl, dl := mustStep(t, "comment", c.Steps, "download")
	if want := "needs.plan.outputs.affected != '[]'"; dl.If != want {
		t.Errorf("comment download if = %q, want %q: with nothing affected, no plan was uploaded", dl.If, want)
	}
	wantWith(t, "comment download", dl, map[string]string{"pattern": "idp-plan-*", "path": "${{ runner.temp }}/idp/plans", "merge-multiple": "true"})
	iSum, _ := mustStep(t, "comment", c.Steps, "summary")
	iPost, post := mustStep(t, "comment", c.Steps, "post")
	if !(iDl < iSum && iSum < iPost) {
		t.Errorf("comment must download the plans, summarize them, then post (got indexes %d, %d, %d)", iDl, iSum, iPost)
	}
	wantEnv(t, "comment post", post, map[string]string{"PR_NUMBER": "${{ github.event.pull_request.number }}"})
	wantRun(t, "comment post", post, `idp comment --repo "$GITHUB_REPOSITORY" --pr "$PR_NUMBER" --body-file "$RUNNER_TEMP/idp/out/comment.md"`)
}

func TestPRPlansEachAWSStackInItsOwnJob(t *testing.T) {
	w := loadWorkflow(t, prPath)
	plan := mustJob(t, w, "plan")
	_, r := mustStep(t, "plan", plan.Steps, "upload-renders")
	if want := "steps.stacks.outputs.aws != '[]'"; r.If != want {
		t.Errorf("upload-renders if = %q, want %q", r.If, want)
	}
	wantWith(t, "plan upload-renders", r, map[string]string{
		"name":                 "idp-renders",
		"path":                 "${{ runner.temp }}/idp/renders",
		"include-hidden-files": "true",
		"retention-days":       "1",
	})
	checkAWSMatrix(t, w, "plan-aws")
	_, p := mustStep(t, "plan-aws", mustJob(t, w, "plan-aws").Steps, "plan")
	wantEnv(t, "plan-aws plan", p, map[string]string{"STACK": "${{ matrix.stack }}", "ACCOUNT": "${{ matrix.account }}"})
	wantRun(t, "plan-aws plan", p, `idp aws-stack plan --stack "$STACK" --account "$ACCOUNT"`, `--endpoint http://localhost:4566 --out-dir "$RUNNER_TEMP/idp/reduced"`)
}

// TestPRPlansCrossJobsReduced: tofu show -json holds the values a plan sets,
// in plaintext, so only reduced plans leave a job (ADR-0031).
func TestPRPlansCrossJobsReduced(t *testing.T) {
	w := loadWorkflow(t, prPath)
	plan := mustJob(t, w, "plan")
	_, red := mustStep(t, "plan", plan.Steps, "reduce")
	wantRun(t, "plan reduce", red, `idp plan-reduce --stack github --plan "$RUNNER_TEMP/idp/plans/github.json" --out-dir "$RUNNER_TEMP/idp/reduced"`)
	_, up := mustStep(t, "plan", plan.Steps, "upload")
	wantWith(t, "plan upload", up, map[string]string{
		"name":              "idp-plan-github",
		"path":              "${{ runner.temp }}/idp/reduced/github.json",
		"retention-days":    "1",
		"if-no-files-found": "error",
	})
	_, aws := mustStep(t, "plan-aws", mustJob(t, w, "plan-aws").Steps, "upload")
	wantWith(t, "plan-aws upload", aws, map[string]string{
		"name":              "idp-plan-${{ matrix.artifact }}",
		"path":              "${{ runner.temp }}/idp/reduced/${{ matrix.artifact }}.json",
		"retention-days":    "1",
		"if-no-files-found": "error",
	})
}
```


- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/workflowspec/ -count=1`
Expected: `FAIL` for exactly these, with these lines:
- `TestFlociJobsHoldNoSecret`: `jobs that run floci = [], want [.github/workflows/pr.yaml plan-aws]`;
- `TestPRInterface`: `jobs = [comment plan validate verdict]`;
- `TestPRJobPermissions`, `TestPRPlansOnlySameRepoBranches`: `.github/workflows/pr.yaml: no job "plan-aws"`;
- `TestPRVerdictFailsForksAndFailures`: `verdict needs = [comment plan validate]`, `env PLAN_AWS = ""`, and the run lacking the `PLAN_AWS` test;
- `TestPRPlanSummaryCoversTheAffectedStacksAtTheHeadCommit`: the run lacking `idp stacks --affected "$AFFECTED" --wet .idp/wet/rendered`, `plan outputs = map[]`, `comment: no step with id "summary"`;
- `TestPRCommentPostsTheSavedComment`: the download's `if`, `pattern`, `path` and `merge-multiple`, and `comment: no step with id "summary"`;
- `TestPRPlansEachAWSStackInItsOwnJob`: `plan: no step with id "upload-renders"`;
- `TestPRPlansCrossJobsReduced`: `plan: no step with id "reduce"`.

- [ ] **Step 4: Implement**

`.github/workflows/pr.yaml` (Edit):

1. old:

```yaml
name: idp-pr

# Reusable PR pipeline (spec §6.1): validate and render, diff against wet, plan
# the affected stacks with the reader App, keep one sticky plan comment, and end
# in a verdict that the caller's idp-gate job mirrors (ADR-0022).
# The PR plan does not refresh: it compares the claims with the state the last
# apply wrote, because the read-only reader token cannot see admin-only
# repository fields (ADR-0025).

on:
  workflow_call:
```

new:

```yaml
name: idp-pr

# Reusable PR pipeline (spec §6.1): validate and render, diff against wet, plan
# the affected stacks, keep one sticky plan comment, and end in a verdict that
# the caller's idp-gate job mirrors (ADR-0022).
# The plan job plans the github stack with the reader App; it does not refresh:
# it compares the claims with the state the last apply wrote, because the
# read-only reader token cannot see admin-only repository fields (ADR-0025).
# Each affected AWS stack is planned in a plan-aws job against a floci of its
# own, which holds no secret (ADR-0031). Only reduced plans, addresses and
# actions, cross jobs; the comment job summarizes them.

on:
  workflow_call:
```

2. old:

```yaml
    timeout-minutes: 30
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
```

new:

```yaml
    timeout-minutes: 30
    permissions:
      contents: read
    outputs:
      affected: ${{ steps.diff.outputs.affected }}
      aws: ${{ steps.stacks.outputs.aws }}
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
```

3. old:

```yaml
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
```

new:

```yaml
      - name: diff against wet
        id: diff
        run: idp diff --new "$RUNNER_TEMP/idp/work/rendered" --wet .idp/wet/rendered
      - name: split the affected stacks
        id: stacks
        env:
          AFFECTED: ${{ steps.diff.outputs.affected }}
        run: idp stacks --affected "$AFFECTED" --wet .idp/wet/rendered
      - name: keep the AWS renders for plan-aws
        id: renders
        if: steps.stacks.outputs.aws != '[]'
        run: |
          mkdir -p "$RUNNER_TEMP/idp/renders/new" "$RUNNER_TEMP/idp/renders/wet"
          if [ -d "$RUNNER_TEMP/idp/work/rendered/aws" ]; then cp -R "$RUNNER_TEMP/idp/work/rendered/aws" "$RUNNER_TEMP/idp/renders/new/aws"; fi
          if [ -d .idp/wet/rendered/aws ]; then cp -R .idp/wet/rendered/aws "$RUNNER_TEMP/idp/renders/wet/aws"; fi
      - name: upload the AWS renders
        id: upload-renders
        if: steps.stacks.outputs.aws != '[]'
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: idp-renders
          path: ${{ runner.temp }}/idp/renders
          include-hidden-files: true
          retention-days: 1
          overwrite: true
          if-no-files-found: error
      - name: mint the reader token
        id: reader
        if: steps.stacks.outputs.github == 'true'
```

4. old:

```yaml
          tofu init -input=false -lockfile=readonly
          tofu plan -input=false -lock=false -refresh=false -out="$RUNNER_TEMP/idp/plans/github.tfplan"
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
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
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
```

new:

```yaml
          tofu init -input=false -lockfile=readonly
          tofu plan -input=false -lock=false -refresh=false -out="$RUNNER_TEMP/idp/plans/github.tfplan"
          tofu show -json "$RUNNER_TEMP/idp/plans/github.tfplan" > "$RUNNER_TEMP/idp/plans/github.json"
      - name: reduce the github plan
        id: reduce
        if: steps.stacks.outputs.github == 'true'
        run: idp plan-reduce --stack github --plan "$RUNNER_TEMP/idp/plans/github.json" --out-dir "$RUNNER_TEMP/idp/reduced"
      - name: upload the reduced github plan
        id: upload
        if: steps.stacks.outputs.github == 'true'
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: idp-plan-github
          path: ${{ runner.temp }}/idp/reduced/github.json
          retention-days: 1
          overwrite: true
          if-no-files-found: error

  plan-aws:
    name: plan-aws ${{ matrix.stack }}
    needs: plan
    if: needs.plan.outputs.aws != '[]'
    runs-on: ubuntu-24.04
    timeout-minutes: 20
    permissions:
      contents: read
    strategy:
      fail-fast: false
      max-parallel: 10
      matrix:
        include: ${{ fromJSON(needs.plan.outputs.aws) }}
    # floci reaches the Docker socket because its ECR runs a registry container
    # (ADR-0004), so this job holds no secret and saves no cache (ADR-0031).
    services:
      floci:
        image: floci/floci:2.4.0@sha256:5ee85db604c4aa74ac76e07ea5cc83086f37bb472bf3dbb97e34019e48efae9c
        ports:
          - "4566:4566"
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
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
      - name: wait until floci is ready
        id: floci-ready
        run: |
          until (( SECONDS >= 60 )); do
            if [[ "$(curl -fsS --max-time 2 http://localhost:4566/_floci/init 2>/dev/null)" == *'"ready":true'* ]]; then
              exit 0
            fi
            sleep 1
          done
          echo "::error::floci did not report ready on localhost:4566 within 60 seconds"
          exit 1
      - name: fetch the AWS renders
        id: download
        uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2
        with:
          name: idp-renders
          path: ${{ runner.temp }}/idp/renders
      - name: plan the stack against floci
        id: plan
        env:
          STACK: ${{ matrix.stack }}
          ACCOUNT: ${{ matrix.account }}
        run: |
          idp aws-stack plan --stack "$STACK" --account "$ACCOUNT" \
            --new "$RUNNER_TEMP/idp/renders/new" --wet "$RUNNER_TEMP/idp/renders/wet" \
            --work "$RUNNER_TEMP/idp/aws" --endpoint http://localhost:4566 --out-dir "$RUNNER_TEMP/idp/reduced"
      - name: upload the reduced plan
        id: upload
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: idp-plan-${{ matrix.artifact }}
          path: ${{ runner.temp }}/idp/reduced/${{ matrix.artifact }}.json
          retention-days: 1
          overwrite: true
          if-no-files-found: error

  comment:
    name: comment
    needs: [plan, plan-aws]
    if: always() && needs.plan.result == 'success' && (needs.plan-aws.result == 'success' || needs.plan-aws.result == 'skipped')
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    permissions:
```

5. old:

```yaml
        with:
          engine-dir: .idp/engine
          version: ${{ job.workflow_sha }}
      - name: fetch the comment
        id: download
        uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2
        with:
          name: idp-pr-comment
          path: ${{ runner.temp }}/idp/out
      - name: post the sticky plan comment
        id: post
        env:
```

new:

```yaml
        with:
          engine-dir: .idp/engine
          version: ${{ job.workflow_sha }}
      - name: fetch the reduced plans
        id: download
        if: needs.plan.outputs.affected != '[]'
        uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2
        with:
          pattern: idp-plan-*
          path: ${{ runner.temp }}/idp/plans
          merge-multiple: true
      - name: summarize the plans
        id: summary
        env:
          AFFECTED: ${{ needs.plan.outputs.affected }}
          HEAD_SHA: ${{ github.event.pull_request.head.sha }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
        run: |
          mkdir -p "$RUNNER_TEMP/idp/out"
          idp plan-summary --stacks "$AFFECTED" --head-sha "$HEAD_SHA" --run-url "$RUN_URL" \
            --plan-dir "$RUNNER_TEMP/idp/plans" \
            --comment-out "$RUNNER_TEMP/idp/out/comment.md" \
            --fingerprint-out "$RUNNER_TEMP/idp/out/fingerprint.json"
      - name: post the sticky plan comment
        id: post
        env:
```

6. old:

```yaml

  verdict:
    name: verdict
    needs: [validate, plan, comment]
    if: always()
    runs-on: ubuntu-24.04
    timeout-minutes: 5
```

new:

```yaml

  verdict:
    name: verdict
    needs: [validate, plan, plan-aws, comment]
    if: always()
    runs-on: ubuntu-24.04
    timeout-minutes: 5
```

7. old:

```yaml
          SAME_REPO: ${{ github.event.pull_request.head.repo.full_name == github.repository }}
          VALIDATE: ${{ needs.validate.result }}
          PLAN: ${{ needs.plan.result }}
          COMMENT: ${{ needs.comment.result }}
        run: |
          if [ "$SAME_REPO" != true ]; then
```

new:

```yaml
          SAME_REPO: ${{ github.event.pull_request.head.repo.full_name == github.repository }}
          VALIDATE: ${{ needs.validate.result }}
          PLAN: ${{ needs.plan.result }}
          PLAN_AWS: ${{ needs.plan-aws.result }}
          COMMENT: ${{ needs.comment.result }}
        run: |
          if [ "$SAME_REPO" != true ]; then
```

8. old:

```yaml
            exit 1
          fi
          if [ "$VALIDATE" != success ] || [ "$PLAN" != success ] || [ "$COMMENT" != success ]; then
            echo "::error::the idp pipeline did not succeed (validate: $VALIDATE, plan: $PLAN, comment: $COMMENT)"
            exit 1
          fi
          echo "idp: validated, planned and commented"
```

new:

```yaml
            exit 1
          fi
          if [ "$VALIDATE" != success ] || [ "$PLAN" != success ] || [ "$COMMENT" != success ]; then
            echo "::error::the idp pipeline did not succeed (validate: $VALIDATE, plan: $PLAN, plan-aws: $PLAN_AWS, comment: $COMMENT)"
            exit 1
          fi
          if [ "$PLAN_AWS" != success ] && [ "$PLAN_AWS" != skipped ]; then
            echo "::error::an AWS stack's plan did not succeed (plan-aws: $PLAN_AWS)"
            exit 1
          fi
          echo "idp: validated, planned and commented"
```


- [ ] **Step 5: Run them to see them pass, and commit**

Run: `go test ./internal/workflowspec/ -count=1`
Expected: `ok`: the new tests and every existing rule (values only through `env:`, pinned actions, `persist-credentials: false`, `idp` only after `setup-idp` from the workflow's commit, the caller templates' permission union). Then `go test ./... && go vet ./... && gofmt -l .` pass.

```bash
git add .github/workflows/pr.yaml internal/workflowspec/pr_test.go internal/workflowspec/floci_test.go
git commit -m "feat(pr): plan each affected aws stack against its own floci"
```

`Risk: items 1, 2, 3 and 5 (the reusable workflows callers consume: what reaches wet, where secrets go, CI)`.

- [ ] **Step 6: Mutation checks** (nothing is committed; restore each with `git checkout .github/workflows/pr.yaml`, then `git status --short` prints nothing). Run `go test ./internal/workflowspec/ -count=1` after each.
1. In `plan-aws`'s step `plan`, add the line `          GITHUB_TOKEN: ${{ github.token }}` after `          ACCOUNT: ${{ matrix.account }}`. Expected: `FAIL: TestFlociJobsHoldNoSecret` with `.github/workflows/pr.yaml plan-aws: "${{ github.token }}" names github.token: a job that runs floci holds no secret (ADR-0031)`.
2. Remove the line `      max-parallel: 10`. Expected: `FAIL: TestPRPlansEachAWSStackInItsOwnJob` with `plan-aws strategy = map[fail-fast:false matrix.include:${{ fromJSON(needs.plan.outputs.aws) }}], want map[...max-parallel:10] and nothing else in the matrix`.

---

### Task 11: `reconcile.yaml` — `plan-aws`, `gate`, `apply-aws` and the wet commit

**Files:**
- Modify: `.github/workflows/reconcile.yaml`, `internal/workflowspec/reconcile_test.go`, `internal/workflowspec/floci_test.go`

**Interfaces:**
- Consumes: Task 10's `checkAWSMatrix` and `TestFlociJobsHoldNoSecret`; `idp stacks`, `plan-reduce`, `plan-summary --plan-dir`, `gate`, `aws-stack apply`, `wet-root`, `wet-push`; the existing `checkManifestToken`, `checkPushToken`.
- Produces: the reconcile jobs `plan` (outputs `wet-sha`, `affected`, `github`, `aws`, `changes`), `plan-aws`, `gate` (output `decision`), `approve`, `apply-aws`, `apply-github`; the wet root's `wet-paths` (Task 12).

- [ ] **Step 1: Write the failing tests**

`internal/workflowspec/floci_test.go` (Edit):

1. old:

```go
	}
	want := []string{
		".github/workflows/pr.yaml plan-aws",
	}
	if slices.Sort(found); !slices.Equal(found, want) {
		t.Errorf("jobs that run floci = %v, want %v", found, want)
```

new:

```go
	}
	want := []string{
		".github/workflows/pr.yaml plan-aws",
		".github/workflows/reconcile.yaml apply-aws",
		".github/workflows/reconcile.yaml plan-aws",
	}
	if slices.Sort(found); !slices.Equal(found, want) {
		t.Errorf("jobs that run floci = %v, want %v", found, want)
```


`internal/workflowspec/reconcile_test.go` (Edit):

1. old:

```go
		"IDP_STATE_PASSPHRASE":   false,
		"IDP_WRITER_PRIVATE_KEY": true,
	})
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
```

new:

```go
		"IDP_STATE_PASSPHRASE":   false,
		"IDP_WRITER_PRIVATE_KEY": true,
	})
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"apply-aws", "apply-github", "approve", "gate", "plan", "plan-aws"}) {
		t.Errorf("jobs = %v", got)
	}
	wantPerms(t, "plan", mustJob(t, w, "plan").Permissions, map[string]string{"contents": "read", "issues": "read"})
	wantPerms(t, "plan-aws", mustJob(t, w, "plan-aws").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "gate", mustJob(t, w, "gate").Permissions, map[string]string{"contents": "read", "issues": "read", "pull-requests": "read"})
	wantPerms(t, "approve", mustJob(t, w, "approve").Permissions, map[string]string{})
	wantPerms(t, "apply-aws", mustJob(t, w, "apply-aws").Permissions, map[string]string{"contents": "read"})
	wantPerms(t, "apply-github", mustJob(t, w, "apply-github").Permissions, map[string]string{"contents": "read", "issues": "write"})
}

func TestReconcileIsNeverReRun(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	for _, id := range []string{"plan", "apply-aws", "apply-github"} {
		j := mustJob(t, w, id)
		if len(j.Steps) == 0 || j.Steps[0].ID != "guard" {
			t.Errorf("%s: the first step must be the guard", id)
```

2. old:

```go
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
```

new:

```go
		`if [ "$EVENT" = workflow_dispatch ]; then all=(--all); fi`,
		`idp diff --new "$RUNNER_TEMP/idp/out/rendered" --wet .idp/wet/rendered "${all[@]}"`,
	)
	_, st := mustStep(t, "plan", plan.Steps, "stacks")
	wantRun(t, "plan stacks", st, `idp stacks --affected "$AFFECTED" --wet .idp/wet/rendered`)
	want := map[string]string{
		"wet-sha":  "${{ steps.wet.outputs.sha }}",
		"affected": "${{ steps.diff.outputs.affected }}",
		"github":   "${{ steps.stacks.outputs.github }}",
		"aws":      "${{ steps.stacks.outputs.aws }}",
		"changes":  "${{ steps.reduce.outputs.changes }}",
	}
	if !maps.Equal(map[string]string(plan.Outputs), want) {
		t.Errorf("plan outputs = %v, want %v", plan.Outputs, want)
	}
}

// TestReconcileGatesWithTheWorkflowToken: the gate decides on every affected
// stack's reduced plan, in a job that holds no secret (spec §6.2 step 2).
func TestReconcileGatesWithTheWorkflowToken(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	gate := mustJob(t, w, "gate")
	if got := slices.Sorted(slices.Values(gate.Needs)); !slices.Equal(got, []string{"plan", "plan-aws"}) {
		t.Errorf("gate needs = %v, want [plan plan-aws]", got)
	}
	if want := "always() && needs.plan.result == 'success' && (needs.plan-aws.result == 'success' || needs.plan-aws.result == 'skipped')"; gate.If != want {
		t.Errorf("gate if = %q, want %q", gate.If, want)
	}
	if gate.Environment != "" {
		t.Errorf("gate environment = %q, want none: the gate holds no secret", gate.Environment)
	}
	if want := (strmap{"decision": "${{ steps.gate.outputs.decision }}"}); !maps.Equal(gate.Outputs, want) {
		t.Errorf("gate outputs = %v, want %v", gate.Outputs, want)
	}
	_, dl := mustStep(t, "gate", gate.Steps, "download")
	if want := "needs.plan.outputs.affected != '[]'"; dl.If != want {
		t.Errorf("gate download if = %q, want %q", dl.If, want)
	}
	wantWith(t, "gate download", dl, map[string]string{"pattern": "idp-plan-*", "path": "${{ runner.temp }}/idp/plans", "merge-multiple": "true"})
	_, s := mustStep(t, "gate", gate.Steps, "summary")
	wantEnv(t, "gate summary", s, map[string]string{"AFFECTED": "${{ needs.plan.outputs.affected }}"})
	wantRun(t, "gate summary", s, `--stacks "$AFFECTED"`, `--head-sha "$GITHUB_SHA"`, `--plan-dir "$RUNNER_TEMP/idp/plans"`)
	_, g := mustStep(t, "gate", gate.Steps, "gate")
	wantEnv(t, "gate gate", g, map[string]string{"GITHUB_TOKEN": "${{ github.token }}"})
	wantRun(t, "gate gate", g, `idp gate --repo "$GITHUB_REPOSITORY" --sha "$GITHUB_SHA" --fingerprint "$RUNNER_TEMP/idp/out/fingerprint.json"`)
	_, up := mustStep(t, "gate", gate.Steps, "upload")
	wantWith(t, "gate upload", up, map[string]string{
		"name":              "idp-gated",
		"path":              "${{ runner.temp }}/idp/out/fingerprint.json",
		"retention-days":    "1",
		"if-no-files-found": "error",
	})
}

// TestReconcilePlansEachAWSStackInItsOwnJob: plan-aws plans each stack as the
// PR did; only reduced plans leave a job (ADR-0031).
func TestReconcilePlansEachAWSStackInItsOwnJob(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	plan := mustJob(t, w, "plan")
	_, red := mustStep(t, "plan", plan.Steps, "reduce")
	wantRun(t, "plan reduce", red, `idp plan-reduce --stack github --plan "$RUNNER_TEMP/idp/plans/github.json" --out-dir "$RUNNER_TEMP/idp/reduced"`)
	_, up := mustStep(t, "plan", plan.Steps, "upload-plan")
	wantWith(t, "plan upload-plan", up, map[string]string{"name": "idp-plan-github", "path": "${{ runner.temp }}/idp/reduced/github.json", "retention-days": "1"})
	_, r := mustStep(t, "plan", plan.Steps, "upload-renders")
	wantWith(t, "plan upload-renders", r, map[string]string{"name": "idp-renders", "path": "${{ runner.temp }}/idp/renders", "include-hidden-files": "true", "retention-days": "1"})
	checkAWSMatrix(t, w, "plan-aws")
	pa := mustJob(t, w, "plan-aws")
	if !slices.Equal(pa.Needs, strlist{"plan"}) {
		t.Errorf("plan-aws needs = %v, want [plan]", pa.Needs)
	}
	_, p := mustStep(t, "plan-aws", pa.Steps, "plan")
	wantRun(t, "plan-aws plan", p, `idp aws-stack plan --stack "$STACK" --account "$ACCOUNT"`)
	_, pu := mustStep(t, "plan-aws", pa.Steps, "upload")
	wantWith(t, "plan-aws upload", pu, map[string]string{"name": "idp-plan-${{ matrix.artifact }}", "path": "${{ runner.temp }}/idp/reduced/${{ matrix.artifact }}.json"})
}

// TestApplyAWSAppliesWhatTheGateDecided: each AWS stack re-plans in its own
// job, refuses a plan that differs from the gated one, applies, and marks the
// stack applied for the wet commit (spec §6.2 step 3).
func TestApplyAWSAppliesWhatTheGateDecided(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	checkAWSMatrix(t, w, "apply-aws")
	a := mustJob(t, w, "apply-aws")
	want := "always() && needs.gate.result == 'success' && needs.plan.outputs.aws != '[]' && (needs.gate.outputs.decision == 'auto' || needs.approve.result == 'success')"
	if a.If != want {
		t.Errorf("apply-aws if = %q, want %q", a.If, want)
	}
	if got := slices.Sorted(slices.Values(a.Needs)); !slices.Equal(got, []string{"approve", "gate", "plan"}) {
		t.Errorf("apply-aws needs = %v", got)
	}
	_, g := mustStep(t, "apply-aws", a.Steps, "gated")
	wantWith(t, "apply-aws gated", g, map[string]string{"name": "idp-gated", "path": "${{ runner.temp }}/idp/gated"})
	_, ap := mustStep(t, "apply-aws", a.Steps, "apply")
	wantEnv(t, "apply-aws apply", ap, map[string]string{"STACK": "${{ matrix.stack }}", "ACCOUNT": "${{ matrix.account }}"})
	wantRun(t, "apply-aws apply", ap,
		`idp aws-stack apply --stack "$STACK" --account "$ACCOUNT"`,
		`--fingerprint "$RUNNER_TEMP/idp/gated/fingerprint.json" --applied-dir "$RUNNER_TEMP/idp/applied"`)
	_, up := mustStep(t, "apply-aws", a.Steps, "upload")
	if up.If != "" {
		t.Errorf("apply-aws upload if = %q: it must keep the implicit success(), so a refused or failed apply marks nothing", up.If)
	}
	wantWith(t, "apply-aws upload", up, map[string]string{
		"name":              "idp-applied-${{ matrix.artifact }}",
		"path":              "${{ runner.temp }}/idp/applied/${{ matrix.artifact }}",
		"retention-days":    "1",
		"if-no-files-found": "error",
	})
}

func TestApprovalIsOneSecretlessJob(t *testing.T) {
```

3. old:

```go
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
```

new:

```go
	if a.Environment != "idp-approval" {
		t.Errorf("approve environment = %q, want idp-approval", a.Environment)
	}
	if want := "needs.gate.outputs.decision == 'approval'"; a.If != want {
		t.Errorf("approve if = %q, want %q", a.If, want)
	}
	if !slices.Equal(a.Needs, strlist{"gate"}) {
		t.Errorf("approve needs = %v, want [gate]", a.Needs)
	}
}

func TestApplyRunsOnlyAfterTheGate(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	a := mustJob(t, w, "apply-github")
	// apply-github runs after apply-aws, whatever its legs concluded, so it
	// commits the github stack and every AWS stack that applied.
	want := "always() && needs.gate.result == 'success' && needs.plan.outputs.affected != '[]' && (needs.gate.outputs.decision == 'auto' || needs.approve.result == 'success')"
	if a.If != want {
		t.Errorf("apply-github if = %q, want %q", a.If, want)
	}
	if a.Environment != "idp-write" {
		t.Errorf("apply-github environment = %q, want idp-write", a.Environment)
	}
	if got := slices.Sorted(slices.Values(a.Needs)); !slices.Equal(got, []string{"apply-aws", "approve", "gate", "plan"}) {
		t.Errorf("apply-github needs = %v", got)
	}
}
```

4. old:

```go
func TestApplyTokenHasTheWriterManifestPermissions(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	_, s := mustStep(t, "apply-github", mustJob(t, w, "apply-github").Steps, "apply-token")
	if want := "needs.plan.outputs.changes != '0'"; s.If != want {
		t.Errorf("apply-token if = %q, want %q", s.If, want)
	}
	checkManifestToken(t, "apply-github apply-token", s)
```

new:

```go
func TestApplyTokenHasTheWriterManifestPermissions(t *testing.T) {
	w := loadWorkflow(t, reconcilePath)
	_, s := mustStep(t, "apply-github", mustJob(t, w, "apply-github").Steps, "apply-token")
	if want := "needs.plan.outputs.github == 'true' && needs.plan.outputs.changes != '0'"; s.If != want {
		t.Errorf("apply-token if = %q, want %q", s.If, want)
	}
	checkManifestToken(t, "apply-github apply-token", s)
```

5. old:

```go
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
	stateCopy := strings.Index(prep.Run, `cp "$RUNNER_TEMP/idp/work/tfstate/github.tfstate" "$root/tfstate/github.tfstate"`)
	renderBranch := strings.Index(prep.Run, `if [ "$CHANGES" = 0 ] || [ "$APPLY_OUTCOME" = success ]; then`)
	if stateCopy < 0 || renderBranch < 0 || stateCopy > renderBranch {
		t.Errorf("prepare must copy the state before, and outside, the render branch: the state is committed even when the apply failed")
	}
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
```

new:

```go
		t.Errorf("prepare if = %q, want %q", prep.If, want)
	}
	wantEnv(t, "apply-github prepare", prep, map[string]string{
		"GITHUB_STACK":  "${{ needs.plan.outputs.github }}",
		"CHANGES":       "${{ needs.plan.outputs.changes }}",
		"APPLY_OUTCOME": "${{ steps.apply.outcome }}",
		"WET_SHA":       "${{ needs.plan.outputs.wet-sha }}",
	})
	wantRun(t, "apply-github prepare", prep,
		`if [ "$CHANGES" = 0 ] || [ "$APPLY_OUTCOME" = success ]; then`,
		`args+=(--github-render)`,
		`idp wet-root --out "$RUNNER_TEMP/idp/wet-root" --wet-base "$WET_SHA"`,
		`--new "$RUNNER_TEMP/idp/in/rendered" --applied-dir "$RUNNER_TEMP/idp/applied" "${args[@]}"`,
	)
	stateArg := strings.Index(prep.Run, `args+=(--github-state "$RUNNER_TEMP/idp/work/tfstate/github.tfstate")`)
	renderBranch := strings.Index(prep.Run, `if [ "$CHANGES" = 0 ] || [ "$APPLY_OUTCOME" = success ]; then`)
	if stateArg < 0 || renderBranch < 0 || stateArg > renderBranch {
		t.Errorf("prepare must name the state before, and outside, the render branch: the state is committed even when the apply failed")
	}
	iApplied, applied := mustStep(t, "apply-github", a.Steps, "applied")
	iPrep, _ := mustStep(t, "apply-github", a.Steps, "prepare")
	if want := "always() && needs.plan.outputs.aws != '[]'"; applied.If != want || iApplied > iPrep {
		t.Errorf("applied if = %q, want %q, before prepare", applied.If, want)
	}
	wantWith(t, "apply-github applied", applied, map[string]string{"pattern": "idp-applied-*", "path": "${{ runner.temp }}/idp/applied", "merge-multiple": "true"})
	_, push := mustStep(t, "apply-github", a.Steps, "push")
	wantEnv(t, "apply-github push", push, map[string]string{"GH_TOKEN": "${{ steps.push-token.outputs.token }}"})
	wantRun(t, "apply-github push", push,
		`while IFS= read -r name; do paths+=(--path "$name"); done < "$RUNNER_TEMP/idp/wet-root/wet-paths"`,
		`idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root"`,
	)
}
```

6. old:

```go
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
```

new:

```go
		"retention-days":       "7",
		"if-no-files-found":    "error",
	})
	_, tok := mustStep(t, "apply-github", a.Steps, "push-token")
	if want := "always() && steps.prepare.outcome == 'success' && steps.prepare.outputs.paths != '0'"; tok.If != want {
		t.Errorf("push-token if = %q, want %q: with nothing to commit, nothing is pushed", tok.If, want)
	}
	if want := "always() && steps.push-token.outcome == 'success'"; push.If != want {
		t.Errorf("push if = %q, want %q", push.If, want)
	}
	if want := "always() && steps.prepare.outcome == 'success' && steps.prepare.outputs.paths != '0' && steps.push.outcome != 'success'"; issue.If != want {
		t.Errorf("issue if = %q, want %q", issue.If, want)
	}
	wantRun(t, "apply-github issue", issue, `idp issue open --repo "$GITHUB_REPOSITORY" --label idp-wet-push --title "Wet push failed"`)
```

7. old:

```go
	// drops by default; without the lock file the apply's tofu init fails.
	w := loadWorkflow(t, reconcilePath)
	_, up := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "upload")
	if want := "steps.stacks.outputs.github == 'true'"; up.If != want {
		t.Errorf("upload if = %q, want %q", up.If, want)
	}
	wantWith(t, "plan upload", up, map[string]string{
```

new:

```go
	// drops by default; without the lock file the apply's tofu init fails.
	w := loadWorkflow(t, reconcilePath)
	_, up := mustStep(t, "plan", mustJob(t, w, "plan").Steps, "upload")
	if want := "steps.diff.outputs.affected != '[]'"; up.If != want {
		t.Errorf("upload if = %q, want %q", up.If, want)
	}
	wantWith(t, "plan upload", up, map[string]string{
```


- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/ -count=1`
Expected: `FAIL` for exactly `TestFlociJobsHoldNoSecret`, `TestReconcileInterface`, `TestReconcileIsNeverReRun`, `TestReconcileDiffsAgainstTheCurrentWet`, `TestReconcileGatesWithTheWorkflowToken`, `TestReconcilePlansEachAWSStackInItsOwnJob`, `TestApplyAWSAppliesWhatTheGateDecided`, `TestApprovalIsOneSecretlessJob`, `TestApplyRunsOnlyAfterTheGate`, `TestApplyTokenHasTheWriterManifestPermissions`, `TestApplyCommitsTheRenderOnlyAfterASuccessfulApply`, `TestWetRootIsSavedBeforeThePush` and `TestRenderArtifactsKeepHiddenFiles`; among their lines, `jobs = [apply-github approve plan]` and `.github/workflows/reconcile.yaml: no job "gate"`.

- [ ] **Step 3: Implement**

`.github/workflows/reconcile.yaml` (Edit):

1. old:

```yaml

# Reusable reconcile pipeline (spec §6.2): re-render and diff against the current
# wet, plan, gate, wait for approval when the gate asks for it, apply the saved
# plan, and commit the state and the render to wet in one commit (ADR-0019).
# The plan runs in idp-write and refreshes with a writer token, which can see
# admin-only repository fields (ADR-0025).
# A run is never re-run; a failed run is retried with a new dispatch (ADR-0024).

on:
```

new:

```yaml

# Reusable reconcile pipeline (spec §6.2): re-render and diff against the current
# wet, plan, gate, wait for approval when the gate asks for it, apply the saved
# plans, and commit the state and the renders that applied to wet in one commit
# (ADR-0019).
# The plan job plans the github stack in idp-write and refreshes with a writer
# token, which can see admin-only repository fields (ADR-0025). Each affected AWS
# stack is planned in a plan-aws job and applied in an apply-aws job, each
# against a floci of its own and holding no secret (ADR-0031); the gate job
# decides on the reduced plans of every stack.
# A run is never re-run; a failed run is retried with a new dispatch (ADR-0024).

on:
```

2. old:

```yaml
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
```

new:

```yaml
    permissions:
      contents: read
      issues: read
    outputs:
      wet-sha: ${{ steps.wet.outputs.sha }}
      affected: ${{ steps.diff.outputs.affected }}
      github: ${{ steps.stacks.outputs.github }}
      aws: ${{ steps.stacks.outputs.aws }}
      changes: ${{ steps.reduce.outputs.changes }}
    steps:
      - name: reconcile main, once
        id: guard
```

3. old:

```yaml
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
      - name: mint the writer token for the plan
        id: plan-token
        if: steps.stacks.outputs.github == 'true'
```

new:

```yaml
          all=()
          if [ "$EVENT" = workflow_dispatch ]; then all=(--all); fi
          idp diff --new "$RUNNER_TEMP/idp/out/rendered" --wet .idp/wet/rendered "${all[@]}"
      - name: split the affected stacks
        id: stacks
        env:
          AFFECTED: ${{ steps.diff.outputs.affected }}
        run: idp stacks --affected "$AFFECTED" --wet .idp/wet/rendered
      - name: keep the AWS renders for plan-aws and apply-aws
        id: renders
        if: steps.stacks.outputs.aws != '[]'
        run: |
          mkdir -p "$RUNNER_TEMP/idp/renders/new" "$RUNNER_TEMP/idp/renders/wet"
          if [ -d "$RUNNER_TEMP/idp/out/rendered/aws" ]; then cp -R "$RUNNER_TEMP/idp/out/rendered/aws" "$RUNNER_TEMP/idp/renders/new/aws"; fi
          if [ -d .idp/wet/rendered/aws ]; then cp -R .idp/wet/rendered/aws "$RUNNER_TEMP/idp/renders/wet/aws"; fi
      - name: upload the AWS renders
        id: upload-renders
        if: steps.stacks.outputs.aws != '[]'
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: idp-renders
          path: ${{ runner.temp }}/idp/renders
          include-hidden-files: true
          retention-days: 1
          if-no-files-found: error
      - name: mint the writer token for the plan
        id: plan-token
        if: steps.stacks.outputs.github == 'true'
```

4. old:

```yaml
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
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
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
```

new:

```yaml
          tofu init -input=false -lockfile=readonly
          tofu plan -input=false -lock=false -out="$RUNNER_TEMP/idp/out/github.tfplan"
          tofu show -json "$RUNNER_TEMP/idp/out/github.tfplan" > "$RUNNER_TEMP/idp/plans/github.json"
      - name: reduce the github plan
        id: reduce
        if: steps.stacks.outputs.github == 'true'
        run: idp plan-reduce --stack github --plan "$RUNNER_TEMP/idp/plans/github.json" --out-dir "$RUNNER_TEMP/idp/reduced"
      - name: upload the reduced github plan
        id: upload-plan
        if: steps.stacks.outputs.github == 'true'
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: idp-plan-github
          path: ${{ runner.temp }}/idp/reduced/github.json
          retention-days: 1
          if-no-files-found: error
      - name: keep the plan and the render for the apply
        id: upload
        if: steps.diff.outputs.affected != '[]'
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: idp-reconcile
          path: ${{ runner.temp }}/idp/out
          include-hidden-files: true
          retention-days: 1
          if-no-files-found: error

  plan-aws:
    name: plan-aws ${{ matrix.stack }}
    needs: plan
    if: needs.plan.outputs.aws != '[]'
    runs-on: ubuntu-24.04
    timeout-minutes: 20
    permissions:
      contents: read
    strategy:
      fail-fast: false
      max-parallel: 10
      matrix:
        include: ${{ fromJSON(needs.plan.outputs.aws) }}
    # floci reaches the Docker socket because its ECR runs a registry container
    # (ADR-0004), so this job holds no secret and saves no cache (ADR-0031).
    services:
      floci:
        image: floci/floci:2.4.0@sha256:5ee85db604c4aa74ac76e07ea5cc83086f37bb472bf3dbb97e34019e48efae9c
        ports:
          - "4566:4566"
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
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
      - name: wait until floci is ready
        id: floci-ready
        run: |
          until (( SECONDS >= 60 )); do
            if [[ "$(curl -fsS --max-time 2 http://localhost:4566/_floci/init 2>/dev/null)" == *'"ready":true'* ]]; then
              exit 0
            fi
            sleep 1
          done
          echo "::error::floci did not report ready on localhost:4566 within 60 seconds"
          exit 1
      - name: fetch the AWS renders
        id: download
        uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2
        with:
          name: idp-renders
          path: ${{ runner.temp }}/idp/renders
      - name: plan the stack against floci
        id: plan
        env:
          STACK: ${{ matrix.stack }}
          ACCOUNT: ${{ matrix.account }}
        run: |
          idp aws-stack plan --stack "$STACK" --account "$ACCOUNT" \
            --new "$RUNNER_TEMP/idp/renders/new" --wet "$RUNNER_TEMP/idp/renders/wet" \
            --work "$RUNNER_TEMP/idp/aws" --endpoint http://localhost:4566 --out-dir "$RUNNER_TEMP/idp/reduced"
      - name: upload the reduced plan
        id: upload
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: idp-plan-${{ matrix.artifact }}
          path: ${{ runner.temp }}/idp/reduced/${{ matrix.artifact }}.json
          retention-days: 1
          if-no-files-found: error

  gate:
    name: gate
    needs: [plan, plan-aws]
    if: always() && needs.plan.result == 'success' && (needs.plan-aws.result == 'success' || needs.plan-aws.result == 'skipped')
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    permissions:
      contents: read
      issues: read
      pull-requests: read
    outputs:
      decision: ${{ steps.gate.outputs.decision }}
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
      - name: fetch the reduced plans
        id: download
        if: needs.plan.outputs.affected != '[]'
        uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2
        with:
          pattern: idp-plan-*
          path: ${{ runner.temp }}/idp/plans
          merge-multiple: true
      - name: summarize the plans
        id: summary
        env:
          AFFECTED: ${{ needs.plan.outputs.affected }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
        run: |
          mkdir -p "$RUNNER_TEMP/idp/out"
          idp plan-summary --stacks "$AFFECTED" --head-sha "$GITHUB_SHA" --run-url "$RUN_URL" \
            --plan-dir "$RUNNER_TEMP/idp/plans" \
            --comment-out "$RUNNER_TEMP/idp/out/comment.md" \
            --fingerprint-out "$RUNNER_TEMP/idp/out/fingerprint.json"
          cat "$RUNNER_TEMP/idp/out/comment.md" >> "$GITHUB_STEP_SUMMARY"
      - name: gate
        id: gate
        env:
          GITHUB_TOKEN: ${{ github.token }}
        run: idp gate --repo "$GITHUB_REPOSITORY" --sha "$GITHUB_SHA" --fingerprint "$RUNNER_TEMP/idp/out/fingerprint.json"
      - name: keep the gated fingerprint for apply-aws
        id: upload
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: idp-gated
          path: ${{ runner.temp }}/idp/out/fingerprint.json
          retention-days: 1
          if-no-files-found: error

  approve:
    name: approve
    needs: gate
    if: needs.gate.outputs.decision == 'approval'
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    environment: idp-approval
```

5. old:

```yaml
        id: approved
        run: echo "approved in the idp-approval environment; the apply continues"

  apply-github:
    name: apply-github
    needs: [plan, approve]
    if: always() && needs.plan.result == 'success' && needs.plan.outputs.github == 'true' && (needs.plan.outputs.decision == 'auto' || needs.approve.result == 'success')
    runs-on: ubuntu-24.04
    timeout-minutes: 50
    environment: idp-write
```

new:

```yaml
        id: approved
        run: echo "approved in the idp-approval environment; the apply continues"

  apply-aws:
    name: apply-aws ${{ matrix.stack }}
    needs: [plan, gate, approve]
    if: always() && needs.gate.result == 'success' && needs.plan.outputs.aws != '[]' && (needs.gate.outputs.decision == 'auto' || needs.approve.result == 'success')
    runs-on: ubuntu-24.04
    timeout-minutes: 20
    permissions:
      contents: read
    strategy:
      fail-fast: false
      max-parallel: 10
      matrix:
        include: ${{ fromJSON(needs.plan.outputs.aws) }}
    # floci reaches the Docker socket because its ECR runs a registry container
    # (ADR-0004), so this job holds no secret and saves no cache (ADR-0031).
    services:
      floci:
        image: floci/floci:2.4.0@sha256:5ee85db604c4aa74ac76e07ea5cc83086f37bb472bf3dbb97e34019e48efae9c
        ports:
          - "4566:4566"
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
    steps:
      - name: apply once
        id: guard
        run: |
          if [ "$GITHUB_RUN_ATTEMPT" != 1 ]; then
            echo "::error::a reconcile run is never re-run, because its plan may be stale: dispatch the reconcile workflow instead (docs/runbooks/pipelines.md)"
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
      - name: wait until floci is ready
        id: floci-ready
        run: |
          until (( SECONDS >= 60 )); do
            if [[ "$(curl -fsS --max-time 2 http://localhost:4566/_floci/init 2>/dev/null)" == *'"ready":true'* ]]; then
              exit 0
            fi
            sleep 1
          done
          echo "::error::floci did not report ready on localhost:4566 within 60 seconds"
          exit 1
      - name: fetch the AWS renders
        id: download
        uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2
        with:
          name: idp-renders
          path: ${{ runner.temp }}/idp/renders
      - name: fetch the gated fingerprint
        id: gated
        uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2
        with:
          name: idp-gated
          path: ${{ runner.temp }}/idp/gated
      - name: re-plan, compare with the gate and apply against floci
        id: apply
        env:
          STACK: ${{ matrix.stack }}
          ACCOUNT: ${{ matrix.account }}
        run: |
          idp aws-stack apply --stack "$STACK" --account "$ACCOUNT" \
            --new "$RUNNER_TEMP/idp/renders/new" --wet "$RUNNER_TEMP/idp/renders/wet" \
            --work "$RUNNER_TEMP/idp/aws" --endpoint http://localhost:4566 \
            --fingerprint "$RUNNER_TEMP/idp/gated/fingerprint.json" --applied-dir "$RUNNER_TEMP/idp/applied"
      - name: mark the stack applied for the wet commit
        id: upload
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: idp-applied-${{ matrix.artifact }}
          path: ${{ runner.temp }}/idp/applied/${{ matrix.artifact }}
          retention-days: 1
          if-no-files-found: error

  apply-github:
    name: apply-github
    needs: [plan, gate, approve, apply-aws]
    if: always() && needs.gate.result == 'success' && needs.plan.outputs.affected != '[]' && (needs.gate.outputs.decision == 'auto' || needs.approve.result == 'success')
    runs-on: ubuntu-24.04
    timeout-minutes: 50
    environment: idp-write
```

6. old:

```yaml
          path: ${{ runner.temp }}/idp/in
      - name: export TF_ENCRYPTION
        id: encryption
        env:
          IDP_STATE_PASSPHRASE: ${{ secrets.IDP_STATE_PASSPHRASE }}
        run: idp encryption-env
```

new:

```yaml
          path: ${{ runner.temp }}/idp/in
      - name: export TF_ENCRYPTION
        id: encryption
        if: needs.plan.outputs.github == 'true'
        env:
          IDP_STATE_PASSPHRASE: ${{ secrets.IDP_STATE_PASSPHRASE }}
        run: idp encryption-env
```

7. old:

```yaml
          fi
      - name: mint the writer token for the apply
        id: apply-token
        if: needs.plan.outputs.changes != '0'
        uses: actions/create-github-app-token@64ab729a6294000f5aa4a4bc0b73d1f32bc45510 # v3.3.0
        with:
          client-id: ${{ vars.IDP_WRITER_CLIENT_ID }}
```

new:

```yaml
          fi
      - name: mint the writer token for the apply
        id: apply-token
        if: needs.plan.outputs.github == 'true' && needs.plan.outputs.changes != '0'
        uses: actions/create-github-app-token@64ab729a6294000f5aa4a4bc0b73d1f32bc45510 # v3.3.0
        with:
          client-id: ${{ vars.IDP_WRITER_CLIENT_ID }}
```

8. old:

```yaml
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
```

new:

```yaml
        run: |
          tofu init -input=false -lockfile=readonly
          tofu apply -input=false "$RUNNER_TEMP/idp/in/github.tfplan"
      - name: fetch the AWS stacks that applied
        id: applied
        if: always() && needs.plan.outputs.aws != '[]'
        uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2
        with:
          pattern: idp-applied-*
          path: ${{ runner.temp }}/idp/applied
          merge-multiple: true
      - name: prepare the wet commit
        id: prepare
        if: always() && steps.stage.outcome == 'success'
        env:
          GITHUB_STACK: ${{ needs.plan.outputs.github }}
          CHANGES: ${{ needs.plan.outputs.changes }}
          APPLY_OUTCOME: ${{ steps.apply.outcome }}
          WET_SHA: ${{ needs.plan.outputs.wet-sha }}
        run: |
          args=()
          if [ "$GITHUB_STACK" = true ]; then
            args+=(--github-state "$RUNNER_TEMP/idp/work/tfstate/github.tfstate")
            # The state is committed always: real resources may have changed. The render
            # only when the apply succeeded or had nothing to apply; a failed stack keeps
            # its old render in wet, so the next reconcile retries it (spec §6.2).
            if [ "$CHANGES" = 0 ] || [ "$APPLY_OUTCOME" = success ]; then
              args+=(--github-render)
            else
              echo "::warning::the github apply did not succeed: only its state is committed, and its old render stays in wet so the next reconcile retries"
            fi
          fi
          # An AWS stack's render is committed only when its apply-aws job marked it
          # applied (spec §6.2 step 4); its state stays in that job (ADR-0030).
          idp wet-root --out "$RUNNER_TEMP/idp/wet-root" --wet-base "$WET_SHA" \
            --new "$RUNNER_TEMP/idp/in/rendered" --applied-dir "$RUNNER_TEMP/idp/applied" "${args[@]}"
      - name: keep the wet commit as an artifact
        id: keep
        if: always() && steps.prepare.outcome == 'success'
```

9. old:

```yaml
          if-no-files-found: error
      - name: mint the wet-commit token
        id: push-token
        if: always() && steps.prepare.outcome == 'success'
        uses: actions/create-github-app-token@64ab729a6294000f5aa4a4bc0b73d1f32bc45510 # v3.3.0
        with:
          client-id: ${{ vars.IDP_WRITER_CLIENT_ID }}
```

new:

```yaml
          if-no-files-found: error
      - name: mint the wet-commit token
        id: push-token
        if: always() && steps.prepare.outcome == 'success' && steps.prepare.outputs.paths != '0'
        uses: actions/create-github-app-token@64ab729a6294000f5aa4a4bc0b73d1f32bc45510 # v3.3.0
        with:
          client-id: ${{ vars.IDP_WRITER_CLIENT_ID }}
```

10. old:

```yaml
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
```

new:

```yaml
        if: always() && steps.push-token.outcome == 'success'
        env:
          GH_TOKEN: ${{ steps.push-token.outputs.token }}
        run: |
          paths=()
          while IFS= read -r name; do paths+=(--path "$name"); done < "$RUNNER_TEMP/idp/wet-root/wet-paths"
          idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root" \
            --message "idp: reconcile $GITHUB_SHA (run $GITHUB_RUN_ID)" "${paths[@]}"
      - name: open the wet-push issue
        id: issue
        if: always() && steps.prepare.outcome == 'success' && steps.prepare.outputs.paths != '0' && steps.push.outcome != 'success'
        env:
          GITHUB_TOKEN: ${{ github.token }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
```


- [ ] **Step 4: Run them to see them pass, and commit**

Run: `go test ./internal/workflowspec/ -count=1`
Expected: `ok`. `TestTemplatesGrantExactlyWhatTheReusableJobsNeed` passes unchanged: `gate` keeps the `pull-requests: read` that `plan` drops, so the callers' union does not move. Then `go test ./... && go vet ./... && gofmt -l .` pass.

```bash
git add .github/workflows/reconcile.yaml internal/workflowspec/reconcile_test.go internal/workflowspec/floci_test.go
git commit -m "feat(reconcile): gate, apply and commit the aws stacks"
```

`Risk: items 1, 2, 3 and 5 (the reusable workflows callers consume: what reaches wet, where secrets go, CI)`.

- [ ] **Step 5: Mutation checks** (nothing is committed; restore each with `git checkout .github/workflows/reconcile.yaml`, then `git status --short` prints nothing). Run `go test ./internal/workflowspec/ -count=1` after each.
1. Under `      - name: mark the stack applied for the wet commit` and `        id: upload`, add the line `        if: always()`. Expected: `FAIL: TestApplyAWSAppliesWhatTheGateDecided` with `apply-aws upload if = "always()": it must keep the implicit success(), so a refused or failed apply marks nothing`.
2. Replace `    needs: [plan, gate, approve, apply-aws]` with `    needs: [plan, gate, approve]`. Expected: `FAIL: TestApplyRunsOnlyAfterTheGate` with `apply-github needs = [approve gate plan]`.

---

### Task 12: `recover.yaml` pushes the reconcile's path list

**Files:**
- Modify: `.github/workflows/recover.yaml`, `internal/workflowspec/recover_test.go`

**Interfaces:**
- Consumes: the wet root's `wet-paths` (Tasks 8, 11).
- Produces: a recovery that commits the AWS paths too, and still recovers a wet root saved before `wet-paths` existed (D-8).

- [ ] **Step 1: Write the failing test**

`internal/workflowspec/recover_test.go` (Edit):

1. old:

```go
		"GH_TOKEN": "${{ steps.push-token.outputs.token }}",
		"RENDER":   "${{ steps.base.outputs.render }}",
	})
	wantRun(t, "recover push", push, `paths=(--path tfstate/github.tfstate)`, `idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root"`)
	if !(iReq < iRun && iRun < iIssue && iIssue < iDL && iDL < iBase && iBase < iPush) {
		t.Errorf("recover must check the request, the run and the issue, then download, compare the base, and only then push (got %d %d %d %d %d %d)", iReq, iRun, iIssue, iDL, iBase, iPush)
	}
```

new:

```go
		"GH_TOKEN": "${{ steps.push-token.outputs.token }}",
		"RENDER":   "${{ steps.base.outputs.render }}",
	})
	// A reconcile names the paths it commits in wet-paths, AWS renders
	// included (spec §6.2 step 4); a wet root saved before wet-paths existed
	// holds only the github stack.
	wantRun(t, "recover push", push,
		`if [ -f "$root/wet-paths" ]; then`,
		`while IFS= read -r name; do paths+=(--path "$name"); done < "$root/wet-paths"`,
		`paths=(--path tfstate/github.tfstate)`,
		`if [ "${#paths[@]}" = 0 ]; then`,
		`idp wet-push --repo "$GITHUB_REPOSITORY" --root "$root"`,
	)
	if !(iReq < iRun && iRun < iIssue && iIssue < iDL && iDL < iBase && iBase < iPush) {
		t.Errorf("recover must check the request, the run and the issue, then download, compare the base, and only then push (got %d %d %d %d %d %d)", iReq, iRun, iIssue, iDL, iBase, iPush)
	}
```


- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/workflowspec/ -count=1 -run TestRecover`
Expected: `FAIL: TestRecoverChecksTheRunAndTheWetBase` with four `recover push: run lacks` lines: `if [ -f "$root/wet-paths" ]; then`, the `while IFS= read -r name` loop, `if [ "${#paths[@]}" = 0 ]; then` and `idp wet-push --repo "$GITHUB_REPOSITORY" --root "$root"`.

- [ ] **Step 3: Implement**

`.github/workflows/recover.yaml` (Edit):

1. old:

```yaml
          RENDER: ${{ steps.base.outputs.render }}
          RUN_ID: ${{ inputs.run-id }}
        run: |
          paths=(--path tfstate/github.tfstate)
          if [ "$RENDER" = true ]; then paths+=(--path rendered/github); fi
          idp wet-push --repo "$GITHUB_REPOSITORY" --root "$RUNNER_TEMP/idp/wet-root" \
            --message "idp: recover the wet commit of run $RUN_ID" "${paths[@]}"
      - name: close the wet-push issue
        id: close
```

new:

```yaml
          RENDER: ${{ steps.base.outputs.render }}
          RUN_ID: ${{ inputs.run-id }}
        run: |
          root="$RUNNER_TEMP/idp/wet-root"
          paths=()
          if [ -f "$root/wet-paths" ]; then
            # What the reconcile was about to commit: the github stack and the AWS
            # stacks that applied (spec §6.2 step 4).
            while IFS= read -r name; do paths+=(--path "$name"); done < "$root/wet-paths"
          else
            # A wet root saved before wet-paths existed holds only the github stack.
            paths=(--path tfstate/github.tfstate)
            if [ "$RENDER" = true ]; then paths+=(--path rendered/github); fi
          fi
          if [ "${#paths[@]}" = 0 ]; then
            echo "::error::the wet root of run $RUN_ID lists nothing to commit"
            exit 1
          fi
          idp wet-push --repo "$GITHUB_REPOSITORY" --root "$root" \
            --message "idp: recover the wet commit of run $RUN_ID" "${paths[@]}"
      - name: close the wet-push issue
        id: close
```


- [ ] **Step 4: Run it to see it pass, and commit**

Run: the command of Step 2. Expected: `ok`. Then `go test ./... && go vet ./... && gofmt -l .` pass.

```bash
git add .github/workflows/recover.yaml internal/workflowspec/recover_test.go
git commit -m "feat(recover): push the paths the reconcile listed"
```

`Risk: item 1 (recover commits to wet)`.

- [ ] **Step 5: Mutation check** (nothing is committed) — replace `          if [ -f "$root/wet-paths" ]; then` with `          if false; then`. Run the command of Step 2. Expected: `FAIL` with `recover push: run lacks "if [ -f \"$root/wet-paths\" ]; then"`. Restore with `git checkout .github/workflows/recover.yaml`; `git status --short` prints nothing.

---

### Task 13: `docs/pipelines.md` and the runbook

**Files:**
- Modify: `docs/pipelines.md`, `docs/runbooks/pipelines.md`

**Interfaces:**
- Consumes: Tasks 10 to 12's jobs, permissions and artifacts; `docs/cli.md#aws-stack` (Task 9).
- Produces: the operator docs for the AWS jobs: a failed AWS apply, and recover with AWS paths.

- [ ] **Step 1: Update the docs** (exact replacements with Edit)

`docs/pipelines.md` (Edit):

1. old:

```markdown

| Workflow | Trigger | Checks | What it does |
|---|---|---|---|
| `pr.yaml` | pull request to `main` | `pr / validate`, `pr / plan`, `pr / comment`, `pr / verdict`, `idp-gate` | Validates and renders the claims, diffs the render against `wet`, plans the affected stacks with the reader App without refreshing, and keeps one sticky plan comment on the PR. |
| `reconcile.yaml` | push to `main`, manual dispatch | `reconcile / plan`, `reconcile / approve`, `reconcile / apply-github` | Re-renders and diffs against the current `wet` (every stack on a dispatch), plans with the writer App in `idp-write`, gates, waits for approval when the gate asks for it, applies the saved plan with the writer App, and commits the state and the render to `wet`. |
| `drift.yaml` | daily at 05:23 UTC, manual dispatch | `drift / drift` | Plans the `wet` render with the writer App in `idp-write`, checks the bootstrap protections with the reader App, and keeps one `drift` issue open while anything drifted. |
| `recover.yaml` | manual dispatch with a run id | `recover / recover` | Commits the tree a reconcile run saved when its wet push failed. |
```

new:

```markdown

| Workflow | Trigger | Checks | What it does |
|---|---|---|---|
| `pr.yaml` | pull request to `main` | `pr / validate`, `pr / plan`, `pr / plan-aws <stack>` (one per affected AWS stack), `pr / comment`, `pr / verdict`, `idp-gate` | Validates and renders the claims, diffs the render against `wet`, plans the `github` stack with the reader App without refreshing and each affected AWS stack against its own floci, and keeps one sticky plan comment on the PR. |
| `reconcile.yaml` | push to `main`, manual dispatch | `reconcile / plan`, `reconcile / plan-aws <stack>`, `reconcile / gate`, `reconcile / approve`, `reconcile / apply-aws <stack>`, `reconcile / apply-github` | Re-renders and diffs against the current `wet` (every stack on a dispatch), plans the `github` stack with the writer App in `idp-write` and each AWS stack against its own floci, gates, waits for approval when the gate asks for it, applies the saved plans, and commits the state and the renders that applied to `wet`. |
| `drift.yaml` | daily at 05:23 UTC, manual dispatch | `drift / drift` | Plans the `wet` render with the writer App in `idp-write`, checks the bootstrap protections with the reader App, and keeps one `drift` issue open while anything drifted. |
| `recover.yaml` | manual dispatch with a run id | `recover / recover` | Commits the tree a reconcile run saved when its wet push failed. |
```

2. old:

```markdown
- The PR plan does not refresh: it shows what the PR changes against the state the last apply wrote. Live drift is reported by the drift check, and the reconcile plan sees it ([ADR-0025](adr/0025-plans-refresh-with-the-writer-on-main.md)).
- The plan comment carries a hidden fingerprint of the plan for the PR's head commit. The reconcile gate trusts only that comment, written by `github-actions[bot]` ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).
- `idp-gate` always runs and passes only when the reusable workflow succeeded, so a cancelled or failed run never counts as passing ([ADR-0022](adr/0022-required-check-lives-in-the-caller.md)).

### Reconcile

- The gate applies automatically when nothing is deleted or replaced and every change was in the merged PR's plan comment; otherwise the `idp-approval` environment asks a platform admin first. The rules are in [docs/cli.md](cli.md#gate).
- The apply uses the plan saved by `reconcile / plan` and the render from the same run; it never re-renders.
- The state is always committed to `wet`, even when the apply failed, because real resources may have changed. The render is committed only when the apply succeeded or had nothing to apply, so a failed stack is retried by the next reconcile.
- A reconcile run is never re-run: its jobs refuse a second attempt, and the apply refuses a `wet` that moved since its plan. Retry with a new dispatch ([ADR-0024](adr/0024-failed-wet-push-recovery.md)).
- While there is an open "Wet push failed" issue opened by `github-actions[bot]` (label `idp-wet-push`), every reconcile refuses to plan: an earlier run applied changes it could not commit, so `wet` is older than the real resources until the `recover` workflow commits the saved tree and closes the issue ([ADR-0024](adr/0024-failed-wet-push-recovery.md)).
- Cancelling a run does not interrupt an apply that has started: the apply step runs to completion, and the state is committed. A force-cancel kills OpenTofu mid-apply and can lose state; never force-cancel while `reconcile / apply-github` runs.
```

new:

```markdown
- The PR plan does not refresh: it shows what the PR changes against the state the last apply wrote. Live drift is reported by the drift check, and the reconcile plan sees it ([ADR-0025](adr/0025-plans-refresh-with-the-writer-on-main.md)).
- The plan comment carries a hidden fingerprint of the plan for the PR's head commit. The reconcile gate trusts only that comment, written by `github-actions[bot]` ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).
- `idp-gate` always runs and passes only when the reusable workflow succeeded, so a cancelled or failed run never counts as passing ([ADR-0022](adr/0022-required-check-lives-in-the-caller.md)).
- Each affected AWS stack is planned in a `plan-aws` job of its own, against an empty floci: it applies the environment's `_baseline` and the stack's `wet` version, then plans the new version against that state, or a destroy with the `wet` configuration for an orphan (spec §6.1 step 4, [`idp aws-stack`](cli.md#aws-stack)). A PR that affects no AWS stack starts no floci. The comment job summarizes every stack's reduced plan.

### Reconcile

- The gate applies automatically when nothing is deleted or replaced and every change was in the merged PR's plan comment; otherwise the `idp-approval` environment asks a platform admin first. The rules are in [docs/cli.md](cli.md#gate).
- The apply uses the plan saved by `reconcile / plan` and the render from the same run; it never re-renders.
- `reconcile / gate` decides on every stack's reduced plan. Each `apply-aws` job re-plans its stack as `plan-aws` did, refuses a plan that is not equal to the gated one, applies it to its floci, and writes the evidence to its job summary. `apply-github` runs after every `apply-aws` job, whatever they concluded.
- The `github` state is always committed to `wet`, even when the apply failed, because real resources may have changed. A render is committed only when its stack applied: the `github` stack's when its apply succeeded or had nothing to apply, an AWS stack's when its `apply-aws` job succeeded. A failed stack keeps its old render, so the next reconcile retries it. An orphan's render is removed only after its destroy applied.
- An AWS stack's state stays in its job: `wet` holds no AWS state, and `idp wet-push` refuses any state file but `tfstate/github.tfstate` ([ADR-0030](adr/0030-aws-stack-state-stays-in-the-job.md)).
- A reconcile run is never re-run: its jobs refuse a second attempt, and the apply refuses a `wet` that moved since its plan. Retry with a new dispatch ([ADR-0024](adr/0024-failed-wet-push-recovery.md)).
- While there is an open "Wet push failed" issue opened by `github-actions[bot]` (label `idp-wet-push`), every reconcile refuses to plan: an earlier run applied changes it could not commit, so `wet` is older than the real resources until the `recover` workflow commits the saved tree and closes the issue ([ADR-0024](adr/0024-failed-wet-push-recovery.md)).
- Cancelling a run does not interrupt an apply that has started: the apply step runs to completion, and the state is committed. A force-cancel kills OpenTofu mid-apply and can lose state; never force-cancel while `reconcile / apply-github` runs.
```

3. old:

```markdown
### Drift

- A run fails only when the check cannot run. Drift opens (or updates) the issue labelled `drift`, titled "Drift detected", and adds a warning annotation; a clean run closes it. Nothing is remediated automatically.
- Ruleset bypass lists are invisible to the reader token, so `bootstrap check` reports them as notices ([ADR-0017](adr/0017-drift-check-token.md)).

### Recover
```

new:

```markdown
### Drift

- A run fails only when the check cannot run. Drift opens (or updates) the issue labelled `drift`, titled "Drift detected", and adds a warning annotation; a clean run closes it. Nothing is remediated automatically.
- It checks the `github` stack only: an AWS stack's state lives only in its job's floci, so there is nothing to compare (spec §6.4).
- Ruleset bypass lists are invisible to the reader token, so `bootstrap check` reports them as notices ([ADR-0017](adr/0017-drift-check-token.md)).

### Recover
```

4. old:

```markdown
|---|---|---|---|
| `pr / validate` | `contents: read` | none | none |
| `pr / plan` | `contents: read` | reader (no refresh), whole installation | none |
| `pr / comment` | `contents: read`, `pull-requests: write` | none | none |
| `pr / verdict` | none | none | none |
| `idp-gate` (caller) | none | none | none |
| `reconcile / plan` | `contents`, `issues`, `pull-requests`: read | writer (plan; the writer manifest's permissions, whole installation) | `idp-write` |
| `reconcile / approve` | none | none | `idp-approval` |
| `reconcile / apply-github` | `contents: read`, `issues: write` | writer for the apply (the writer manifest's permissions, whole installation); writer for the wet commit (`contents: write`, the claims repo only) | `idp-write` |
| `drift / drift` | `contents: read`, `issues: write` | writer for the plan (the writer manifest's permissions, whole installation), reader for the bootstrap check | `idp-write` |
| `recover / recover` | `actions: read`, `contents: read`, `issues: write` | writer for the wet commit | `idp-write` |

Each token is set only on the step that uses it ([ADR-0023](adr/0023-pipeline-tokens-per-step.md)).

## Concurrency and artifacts

- `idp-plan-<PR number>` cancels older runs of the same PR.
- `idp-wet` (with `queue: max`) runs reconcile, drift and recover one at a time. A reconcile waiting for approval holds it.
- Artifacts: `idp-pr-comment` (the plan comment, 1 day), `idp-reconcile` (the encrypted plan and the render, 1 day), `idp-wet-root` (the wet commit with the encrypted state, 7 days). Artifacts of public repos are public; the plan and the state are encrypted ([ADR-0002](adr/0002-encrypted-opentofu-state-in-git.md)).

## How the workflows get idp
```

new:

```markdown
|---|---|---|---|
| `pr / validate` | `contents: read` | none | none |
| `pr / plan` | `contents: read` | reader (no refresh), whole installation | none |
| `pr / plan-aws` | `contents: read` | none | none |
| `pr / comment` | `contents: read`, `pull-requests: write` | none | none |
| `pr / verdict` | none | none | none |
| `idp-gate` (caller) | none | none | none |
| `reconcile / plan` | `contents`, `issues`: read | writer (plan; the writer manifest's permissions, whole installation) | `idp-write` |
| `reconcile / plan-aws` | `contents: read` | none | none |
| `reconcile / gate` | `contents`, `issues`, `pull-requests`: read | none | none |
| `reconcile / approve` | none | none | `idp-approval` |
| `reconcile / apply-aws` | `contents: read` | none | none |
| `reconcile / apply-github` | `contents: read`, `issues: write` | writer for the apply (the writer manifest's permissions, whole installation); writer for the wet commit (`contents: write`, the claims repo only) | `idp-write` |
| `drift / drift` | `contents: read`, `issues: write` | writer for the plan (the writer manifest's permissions, whole installation), reader for the bootstrap check | `idp-write` |
| `recover / recover` | `actions: read`, `contents: read`, `issues: write` | writer for the wet commit | `idp-write` |

Each token is set only on the step that uses it ([ADR-0023](adr/0023-pipeline-tokens-per-step.md)).

The jobs that run floci (`plan-aws`, `apply-aws`) hold no secret, no App token and no environment: floci reaches the runner's Docker socket, and with it everything in its job ([ADR-0031](adr/0031-floci-jobs-hold-no-secret.md)).

## Concurrency and artifacts

- `idp-plan-<PR number>` cancels older runs of the same PR.
- `idp-wet` (with `queue: max`) runs reconcile, drift and recover one at a time. A reconcile waiting for approval holds it.
- Artifacts: `idp-plan-<stack>` (each stack's reduced plan, its addresses and actions only, 1 day; `<stack>` has each `/` as `.`), `idp-renders` (the new and the `wet` AWS renders, 1 day), `idp-gated` (the gated fingerprint, 1 day), `idp-applied-<stack>` (the AWS stacks that applied, 1 day), `idp-reconcile` (the encrypted `github` plan and the render, 1 day), `idp-wet-root` (the wet commit with the encrypted state and its list of paths, `wet-paths`, 7 days). Artifacts of public repos are public; the `github` plan and state are encrypted ([ADR-0002](adr/0002-encrypted-opentofu-state-in-git.md)), and no full plan leaves its job ([ADR-0031](adr/0031-floci-jobs-hold-no-secret.md)).
- AWS jobs: one per affected AWS stack, ten at a time (`max-parallel`), and every job finishes even when another fails. `idp stacks` refuses more than 256 AWS stacks in one run, the jobs one matrix runs: split such a change.

## How the workflows get idp
```

5. old:

```markdown

## Limitations

- Only the `github` stack is reconciled until Phase 2e; the workflows fail when any other stack is affected. The render writes AWS stacks for every environment with an `aws` block since Phase 2d, so a claims repo with one stays on an engine release from before Phase 2d (`idp-claims` pins the `v0.1.1` tag) until the release that applies them, `v0.2.0`.
- The required check comes from a workflow file in the claims repo, so a pull request can change it. Requiring a review on `.github/` closes that gap; it is the owner's decision ([ADR-0022](adr/0022-required-check-lives-in-the-caller.md)).
- GitHub disables scheduled workflows in public repos after 60 days without activity. Re-enable the drift check with `gh workflow enable drift.yaml`.
- Every job lists the engine's tags through the GitHub API before it gets `idp`; when that call fails, the job fails rather than guess whether its commit is a release.
```

new:

```markdown

## Limitations

- The AWS stacks are planned and applied against floci only (ADR-0004): nothing reaches a real AWS account, and their state ends with the job. Workspaces are not reconciled yet (Phase 3): a change that affects one fails `idp stacks`. A claims repo with an `aws` block stays on an engine release from before Phase 2d (`idp-claims` pins the `v0.1.1` tag) until `v0.2.0`.
- The required check comes from a workflow file in the claims repo, so a pull request can change it. Requiring a review on `.github/` closes that gap; it is the owner's decision ([ADR-0022](adr/0022-required-check-lives-in-the-caller.md)).
- GitHub disables scheduled workflows in public repos after 60 days without activity. Re-enable the drift check with `gh workflow enable drift.yaml`.
- Every job lists the engine's tags through the GitHub API before it gets `idp`; when that call fails, the job fails rather than guess whether its commit is a release.
```


`docs/runbooks/pipelines.md` (Edit):

1. old:

```markdown

- **`apply-github` failed in "apply the saved plan"**: the state was still committed to `wet`, because real resources may have changed, and the old render stayed, so the next reconcile retries the stack. Fix the cause (a claims PR, or wait out a transient error), then dispatch. The new plan shows only what is left; the gate asks for approval when that is not in the last merged PR's plan.
- **`apply-github` failed in "refuse when wet moved since the plan"**: something committed to `wet` after this run planned. Dispatch a new reconcile.
- **The run opened a "Wet push failed" issue** (its wet-commit token or its commit to wet failed): do not dispatch. Follow [A wet push failed](#a-wet-push-failed); until the recovery runs, every reconcile refuses to plan.
- **The run was cancelled during `apply-github`**: if it opened a "Wet push failed" issue, follow [A wet push failed](#a-wet-push-failed); the recovery accepts cancelled runs. **The runner was lost** (the run opened no "Wet push failed" issue, whether or not it has an `idp-wet-root` artifact): compare the newest `wet` commit with the run's log before dispatching; if resources were applied but never committed, the state has to be rebuilt with `import` blocks (spec §7.7), so open an issue in the engine repo.
```

new:

```markdown

- **`apply-github` failed in "apply the saved plan"**: the state was still committed to `wet`, because real resources may have changed, and the old render stayed, so the next reconcile retries the stack. Fix the cause (a claims PR, or wait out a transient error), then dispatch. The new plan shows only what is left; the gate asks for approval when that is not in the last merged PR's plan.
- **`apply-github` failed in "refuse when wet moved since the plan"**: something committed to `wet` after this run planned. Dispatch a new reconcile.
- **An `apply-aws <stack>` job failed**: the other stacks still applied, and `apply-github` committed their renders; the failed stack kept its old render in `wet`, so the next reconcile retries it. Its job summary says why. "the plan differs from the gated one, so nothing was applied" means the re-plan was not the plan the gate decided on, so nothing was applied: dispatch a new reconcile, which plans and gates it again. An OpenTofu error in a replay or the plan means the stack does not apply even to an empty floci: fix the claims, or the engine if the render is wrong, then dispatch. Nothing real was changed: AWS stacks run only against floci, and their state ends with the job.
- **A `plan-aws <stack>` job failed**: the gate did not run and nothing was applied. Read its log, fix the cause, then dispatch.
- **`plan` failed in "split the affected stacks"**: the change affects a stack this engine version does not reconcile (a Workspace, until Phase 3), or more than 256 AWS stacks. Revert the claims change, or split it into smaller PRs.
- **The run opened a "Wet push failed" issue** (its wet-commit token or its commit to wet failed): do not dispatch. Follow [A wet push failed](#a-wet-push-failed); until the recovery runs, every reconcile refuses to plan.
- **The run was cancelled during `apply-github`**: if it opened a "Wet push failed" issue, follow [A wet push failed](#a-wet-push-failed); the recovery accepts cancelled runs. **The runner was lost** (the run opened no "Wet push failed" issue, whether or not it has an `idp-wet-root` artifact): compare the newest `wet` commit with the run's log before dispatching; if resources were applied but never committed, the state has to be rebuilt with `import` blocks (spec §7.7), so open an issue in the engine repo.
```

2. old:

````markdown
   gh workflow run recover.yaml --repo <repo> --ref main -f run-id=<run id>
   ```

2. It checks that the run is a failed or cancelled reconcile of `main` and that the open issue names it, downloads the artifact, and refuses if `wet` moved since that run's plan. Otherwise it commits the saved state, and the render if the apply had succeeded, and closes the issue.
3. If it refuses because `wet` moved, or the 7 days have passed, do not force anything: the saved state is older than `wet`'s. The state has to be rebuilt with `import` blocks (spec §7.7); that runbook is not written yet, so open an issue in the engine repo.
4. If the recovery committed to `wet` but failed to close the issue, a second recovery refuses (wet moved). Check that the newest `wet` commit is "idp: recover the wet commit of run <id>", then close the issue by hand; that unblocks the reconcile.
5. After a successful recovery, dispatch a reconcile: claims merged while the reconcile was held are not in `wet` yet, and the drift check cannot see them.
````

new:

````markdown
   gh workflow run recover.yaml --repo <repo> --ref main -f run-id=<run id>
   ```

2. It checks that the run is a failed or cancelled reconcile of `main` and that the open issue names it, downloads the artifact, and refuses if `wet` moved since that run's plan. Otherwise it commits the paths the run listed in `wet-paths`: the saved state, the `github` render if its apply had succeeded, and the render of each AWS stack that applied, or the removal of an orphan's render; then it closes the issue. A wet root saved before `wet-paths` existed commits the `github` stack only, as before.
3. If it refuses because `wet` moved, or the 7 days have passed, do not force anything: the saved state is older than `wet`'s. The state has to be rebuilt with `import` blocks (spec §7.7); that runbook is not written yet, so open an issue in the engine repo.
4. If the recovery committed to `wet` but failed to close the issue, a second recovery refuses (wet moved). Check that the newest `wet` commit is "idp: recover the wet commit of run <id>", then close the issue by hand; that unblocks the reconcile.
5. After a successful recovery, dispatch a reconcile: claims merged while the reconcile was held are not in `wet` yet, and the drift check cannot see them.
````


- [ ] **Step 2: Check and commit** — `rg -n 'idp-pr-comment|only the .github. stack is reconciled' docs` prints nothing outside `docs/phases` and `docs/superpowers/plans`; the naming check prints nothing.

```bash
git add docs/pipelines.md docs/runbooks/pipelines.md
git commit -m "docs(pipelines): document the aws jobs, their artifacts and runbook"
```

`Risk: none`.

---

### Task 14: Ship — PR 3, review, merge, phase log, README

Controller task (tests are delegated to a subagent).

- [ ] **Step 1: Local verification**
  - `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).
  - The naming check over the branch's changes prints nothing.
  - `git diff --stat main..HEAD` lists exactly the 9 files of Tasks 10 to 13.
  - `git diff main..HEAD -- .github/workflows/ci.yaml .github/workflows/drift.yaml .github/actions examples modules lockfiles internal/render` prints nothing: no CI, drift, action, caller template, module or render change.

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feat/phase-2e-aws-pipelines
gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-2e-aws-pipelines --title "feat: phase 2e aws plan and apply in the pipelines" --body "Part 3 of 3 of docs/superpowers/plans/2026-11-18-phase-2e-aws-pipelines.md (spec amendment A14, ADR-0031): pr.yaml plans each affected AWS stack in a plan-aws job with its own floci and summarizes every stack's reduced plan in the comment job; reconcile.yaml adds plan-aws, a secret-less gate, apply-aws (re-plan, require the gated fingerprint, apply, mark applied) and an apply-github that commits the github state and the renders that applied; recover pushes the reconcile's path list. Jobs that run floci hold no secret. Drift stays GitHub-only. No release: 0.2.0 ships in Phase 2f."
```

- [ ] **Step 3: CI** — `gh pr checks <number> --repo jellalshadows-idp/idp-engine --watch`.
  - Expected: every check of the watch list passes; `workflows` runs actionlint, zizmor and shellcheck on the new jobs.
  - A failure goes through the fix loop: fix with a test, scoped review, push.

- [ ] **Step 4: Final whole-phase review** (most capable model).
  - The package: PR 3's `main..HEAD`, and PRs 1 and 2 as merged; the spec (§5.4–§5.6, §6, §7.2, §11) with A14; ADR-0004, ADR-0018, ADR-0022 to ADR-0025, ADR-0030 and ADR-0031; this plan.
  - One fix wave, one scoped re-review, rulings in the ledger, and CI green again after the wave.

- [ ] **Step 5: Merge** — confirm the PR's latest CI is green, then `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`.

- [ ] **Step 6: The release PR and the harness** (watch list) — the merge's push runs `release.yaml`.
  - Read the release PR with `gh pr list --repo jellalshadows-idp/idp-engine --head release-please--branches--main --json number,title --jq '.[] | "\(.number) \(.title)"'`. Expected: one line, `28 chore(main): release 0.2.0`. Record it and leave the PR open.
  - Record the harness run that `e2e-release-pr` dispatched (`gh run list --repo jellalshadows-idp/idp-engine --workflow e2e.yaml --limit 1 --json databaseId,status,conclusion`) and, when it ends, its conclusion. Expected: `success`, with `reconcile / gate` succeeding and `plan-aws` and `apply-aws` skipped. A failure is a regression to fix before 2f. Record the job durations of the watch list.

- [ ] **Step 7: Write `docs/phases/phase-2e.md`** in the style of `docs/phases/phase-2d.md`. Include:
  - the header list: period, plan, the spec amended by A14, ADR-0031 added, the previous phase (the Phase 2d log), and the status with the three PRs and their merge commits;
  - the summary, and the tasks with their commits and reviews;
  - every ruling (D-1 to D-18 and the execution rulings) with what, why and the cost if wrong. Include the planning findings:
    - OpenTofu plans a swapped configuration against the replayed state, accepts a changed module source on the next `init`, and plans an old configuration's destroy (D-3);
    - `tofu show -json` holds plaintext values, so only reduced plans cross jobs (D-6);
    - the measured baselines of D-9;
  - the Review Focus table with each result, and the watch list with each result: each PR's checks; `TestReplay`'s first run and the floci job's duration; the release PR's number and version; the harness runs and their job durations; the pending items; the Renovate observations, or "not yet run";
  - the notable findings caught by review, and the final disposition of every finding. 2d's "Carried into 2e" items are **Fixed**: the AWS plan and apply against floci (Tasks 5, 6, 10, 11), the scratch copies and no AWS state in `wet` (Tasks 2, 5, 8), the guard (Task 3), `plan-summary`, `gate` and `comment` over the AWS stacks (Tasks 4, 10, 11), the floci time in spec §11 risk 3 (Task 1, D-9).

  Then "Carried into later sub-phases":
  - **2f:**
    - re-pin `idp-claims` to `0.2.0`: its first reconcile plans and applies three new baselines (D-10);
    - the live end-to-end AWS run, and the pending watch items: a real AWS leg, a `pattern` download that matches nothing, the provider downloads per leg;
    - the E2E env-variable checks and the release, as Phase 2b's log lists.
  - **Phase 3:** Workspaces (`aws/<env>/workspaces/<name>`), which `idp stacks` refuses until then; `observe` replays.
  - **Real AWS (beyond v1):** a persistent, encrypted backend (ADR-0030), OIDC credentials, ordering between stacks, and AWS drift.

  Then "Owner notes": release PR #28 stays open until 2f, and every push to `main` re-runs the harness on it. Close with the process notes (inline or delegated execution, TDD evidence, the mutation checks, local OpenTofu 1.12.6 against CI's 1.13.2) and the status.

- [ ] **Step 8: Update `README.md`** (exact replacements with Edit)

1. The status line, old:

```markdown
> **Status:** Phase 2d is complete: `idp render` writes the AWS stacks, a baseline for every environment with an `aws` block and a stack for every Component with `aws.registry` in each of its environments, with the identities `internal/naming` computes and their state kept in the job, never in `wet` ([ADR-0030](docs/adr/0030-aws-stack-state-stays-in-the-job.md)). CI validates every rendered stack, applies the rendered AWS stacks to two accounts of the floci emulator on every PR, and `idp diff` reports them by their nested paths. The pipelines do not apply them yet. See the [phase 2d log](docs/phases/phase-2d.md) and [docs/claims.md](docs/claims.md). Phase 2e (the workflows' AWS plan and apply) is next.
```

new:

```markdown
> **Status:** Phase 2e is complete: the pipelines plan and apply the AWS stacks against the floci emulator, each stack in a job of its own that holds no secret ([ADR-0031](docs/adr/0031-floci-jobs-hold-no-secret.md)). A job replays the version in `wet` before it plans the new one, an apply must equal the gated plan, and a stack's render reaches `wet` only after it applied; its state stays in the job ([ADR-0030](docs/adr/0030-aws-stack-state-stays-in-the-job.md)). Nothing reaches a real AWS account. See the [phase 2e log](docs/phases/phase-2e.md) and [docs/pipelines.md](docs/pipelines.md). Phase 2f (the live AWS run, the E2E env-variable checks and `0.2.0`) is next.
```

2. "Honest limitations", the pipelines bullet, old:

```markdown
- The pipelines reconcile **GitHub only** (teams, repositories, rulesets, environments and, since Phase 2b, the environment variables of a Component with `aws.registry`). The AWS stacks that create the OIDC provider, the ECR repository and the CI role are rendered since Phase 2d and tested against floci in CI; the pipelines apply them from Phase 2e, so a claims repo with an `aws` block stays on `v0.1.1` until `v0.2.0`.
```

new:

```markdown
- The pipelines reconcile GitHub (teams, repositories, rulesets, environments and, since Phase 2b, the environment variables of a Component with `aws.registry`) and, since Phase 2e, the AWS stacks that create the OIDC provider, the ECR repository and the CI role, **against the floci emulator only**: their state ends with the job, and there is no AWS drift check. A claims repo with an `aws` block stays on `v0.1.1` until `v0.2.0`.
```

- [ ] **Step 9: Commit doc-only to `main`** (side effect 2) — the naming check prints nothing; then:

```bash
git add docs/phases/phase-2e.md README.md
git commit -m "docs: add the phase 2e execution log"
git push
```

The push runs `release.yaml`, which refreshes the open release PR and re-runs the harness on it. Leave the PR open.

- [ ] **Step 10: Delete the SDD workspace** (side effect 3) — after checking that the phase log reproduces the ledger: `rm -rf .superpowers/sdd`, then `git status --short` prints nothing.

---

## Self-review

- **Spec coverage.**
  - §6.1 step 4: `plan-aws` per stack, one floci per job, the baseline, the `wet` replay, the new plan or the orphan's destroy with its old config (D-3; Tasks 5, 6, 10).
  - §6.1 steps 5 to 7: `plan-summary`, the comment and the marker over every stack, `verdict` and `idp-gate` unchanged as the required check (D-2, D-5; Tasks 4, 10).
  - §6.2 step 1: the plan split into `plan`, `plan-aws` and `gate`, recorded by A14 (D-2; Task 11).
  - §6.2 step 2: `approve` stays one secret-less job (Task 11).
  - §6.2 step 3: `apply-aws` with `fail-fast: false`, no environment, the equality with the gated fingerprint and the evidence in the job summary (D-3; Tasks 5, 6, 11).
  - §6.2 step 4: `apply-github` after `apply-aws`, the state always, the renders of what applied, the orphan removed after its destroy, `idp-wet-root` before the push (D-1, D-16; Tasks 8, 11).
  - §6.4 and recover: drift GitHub-only, recover with AWS paths (D-8; Tasks 1, 12).
  - §11 risk 3: one job per stack, ten at a time, the 256-job limit, measured and forecast times (D-9; Tasks 1, 3).
  - ADR-0030: scratch copies, one state per stack, no AWS state in `wet` (Tasks 2, 5, 8).
- **Rulings 1 to 13** map to D-1 to D-13; D-14 to D-18 were decided while planning. No ruling conflicted with the spec or the code.
- **Every carried item has a task:** 2d's "Carried into 2e" list (Tasks 2 to 13; Task 14 Step 7 lists them); the Renovate observations (watch list).
- **No placeholders.** Every Edit step has exact old blocks, checked to occur once in the file at that point of the plan, and new blocks. Every new file is given whole.
- **Validated.** Every block was applied verbatim, in task order, by a script to an archive of `19e65bd`, and each touched file was compared with the validated scratch branch after its task. The result passed `go test ./...`, `go vet ./...` and `gofmt -l .`, and RED, GREEN and the mutation checks were observed as each task states. OpenTofu 1.12.6 confirmed the one-state swap offline (D-3).
- **Names are consistent across tasks:**
  - `wetdiff.Reconcilable` and `wetdiff.Escape`, from Task 3 to Tasks 4, 5, 8;
  - `planFileIn` and `countsLine`, from Task 4 to Task 6;
  - `awsstack.Plan`, `Apply`, `Exec`, `Env` and `WriteOverride`, from Task 5 to Tasks 6 and 7;
  - the marker file named by `wetdiff.Escape`, from Task 6 to Task 8;
  - the outputs `affected`, `github`, `aws`, `changes`, `decision`, `paths` and the artifact names, from Tasks 3, 4, 8 to Tasks 10 and 11;
  - `wet-paths`, from Task 8 to Tasks 11 and 12.
- **Left to CI:** OpenTofu 1.13.2; `TestReplay` against floci; actionlint, zizmor and shellcheck on the new jobs and scripts; Renovate's handling of the floci image in three files (watch list). **Left to 2f's live run:** an AWS leg in a real pipeline (D-10).
- **The skill named by the brief, `superpowers:writing-plans`, was not installed on the planning machine.** This plan follows the Phase 2d plan's structure instead.
