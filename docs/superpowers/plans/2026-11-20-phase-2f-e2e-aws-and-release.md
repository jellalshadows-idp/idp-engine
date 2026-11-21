# Phase 2f — The E2E AWS Checks and the 0.2.0 Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** close Phase 2 (spec §10: "E2E checks the env variables, and floci integration is green").
- The end-to-end harness gives its Component `aws.registry: true` in `dev` and checks, live, through `idp-claims-e2e`: the PR plans the Component's AWS stack against floci, the reconcile applies it, its render reaches `wet`, the GitHub environment `dev` carries `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION` with spec §5.3's values, and the delete destroys the stack and removes its render.
- The harness reads those variables with a token of the reader App, scoped to the Component repository and `environments: read` (owner decision, 2026-11-20; spec amendment A16).
- Three hardening items carried from Phase 2e: OpenTofu gets only an allowlist of the job's environment (`awsstack.Env`), `recover` checks the artifact the issue names through the API before downloading it, and no run script reads a process substitution (the plan layout step logs what it skips and fails on a `jq` error).
- The `0.2.0` release (release PR #28), the re-pin of the templates and of `idp-claims` to it, `idp-claims`'s first AWS reconcile, and the phase log that closes Phase 2.

**Architecture:**
- **`internal/awsstack`:** `Env(base, account)` keeps from `base` only the variables `passEnv` names, then adds the floci account, the fixed secret access key and the no-prompt flags.
- **Workflows:** `pr.yaml` and `reconcile.yaml` read the trusted AWS matrix into a variable before the layout loop and warn for each plan they skip; `recover.yaml` reads the run through a variable and gains an `artifact-check` step whose verified ID the download uses. A new rule test refuses `<(` in any run script.
- **`internal/e2e`:** `GitHub` gains a `Reader` client and `EnvVariables`; `Main` mints reader tokens for `e2e-r<run>-app` only. The scenario reads `dev`'s `aws` block from `config/platform.yaml` in preflight, renders the Component with `aws.registry: true`, and checks the AWS stack's plan, apply, render and destroy, and the three variables.
- **Live, owner-confirmed:** an `aws` block for `dev` in `idp-claims-e2e`, the reader credentials in the `idp-engine` environment `idp-e2e`, the merge of release PR #28, the re-pins and the approvals.

**Tech Stack:** Go 1.26 (standard library and the existing `go.yaml.in/yaml/v3`; no new dependency). No new pin. Local OpenTofu 1.12.6; CI's 1.13.2 is authoritative.

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §4.4, §4.6, §5.3, §5.4, §6.1, §6.2, §7.3, §8.5, §9.1, §10 and amendments A10 to A15. This plan adds A16.

Context:
- the phase logs `docs/phases/phase-2e.md` ("Carried into later sub-phases", 2f; the watch list's pending items; "Owner notes"), `phase-2d.md` (D-10), `phase-2c.md` and `phase-2b.md` (the "2f" item);
- ADR-0009, ADR-0013, ADR-0023, ADR-0026 to ADR-0028, ADR-0030 and ADR-0031;
- `docs/e2e.md`, `docs/pipelines.md`, `docs/runbooks/{pipelines,releases,dependencies}.md`.

## Open questions for the owner

Answered by the owner on 2026-11-21: both recommendations are taken (a 403 on `environments: read` alone allows `actions_variables: read`; two PRs). Execution: native, inline in one session, with the final whole-branch review on the most capable model.

1. **The reader token's permission.** GitHub's permission tables list the environment-variable endpoints under `environments: read`, and the harness asks for that alone (D-2). Phase 0 read an environment variable with the reader App's full permission set (ADR-0013), never with `environments: read` alone. If the first live read answers 403, may the harness also ask for `actions_variables: read`? **Recommendation: yes.** Both are read-only on one repository the harness created. With a yes, a 403 in Task 9 becomes a one-line fix PR (`readerPermissions` and its test) instead of a stop.
2. **Two PRs, one of them under the 400-line band.** PR 1 (Tasks 1 to 3, the 2e carries) forecasts about 230 authored lines; PR 2 (Tasks 5 to 8, the harness) about 580. **Recommendation: two PRs** (D-12). The harness is the ship gate, and the owner's `aws` block lands between the two merges (D-9), so each live run tests one change. The alternative is one PR of about 810 lines, the same commits, merged after both owner steps.

## Phase 2 split (owner decision, 2026-11-03)

Phase 2 is delivered as six plans, each written after the previous one ships (Phase 2a ruling D-1, spec §10 under A10).

| Plan | Delivers |
|---|---|
| 2a (shipped, PR #29, `fca5c0b`) | The AWS model of the Platform and the Component, `internal/naming`, the schema and semantic validation, spec amendment A10 |
| 2b (shipped, PR #35, `c6aa267`) | The deterministic env variables in `modules/github/component`, rendered from `internal/naming`; spec amendment A11 |
| 2c (shipped, PR #37, `368e5a3`) | `modules/aws/{baseline,component}`, the AWS lock file and the floci CI job; spec amendment A12 |
| 2d (shipped, PR #38, `76e23ab`) | Rendering the AWS stacks; spec amendment A13, ADR-0030 |
| 2e (shipped, PRs #39 to #42, last merge `a577c34`; log `b0fb772`) | The workflows' AWS plan and apply against floci; spec amendment A14, ADR-0031 |
| **2f (this plan)** | The harness's AWS and env-variable checks with the reader App, the three 2e carries, `0.2.0`, the re-pins and the phase log; spec amendment A16 |

## Decisions taken while planning

The executor copies these into the ledger as rulings before Task 1. Phase 2a's owner decisions still hold: Phase 2 is split (2a D-1), and v1 is floci only, with no real AWS account (2a D-2).
- D-1 to D-3 and D-6 to D-14 record the owner's choices in the brief of 2026-11-20 and what was decided under them.
- D-4, D-5 and D-15 to D-18 were decided while planning, from the real code and from facts read live on 2026-11-20 and 2026-11-21.
- No ruling conflicts with the spec or the code. Where a decision changes a documented placement (where the reader key lives, spec §7.3), A16 records it (D-2).

- **D-1. Scope.**
  - The harness checks the AWS part of a Component end to end: plan, apply, render on `wet`, env variables, destroy (D-3, D-4).
  - The three carries from the Phase 2e log: R1-001 (D-6), the `recover` artifact check (D-7), the layout step (D-8).
  - The `0.2.0` release, the template and `idp-claims` re-pins, `idp-claims`'s first AWS reconcile (D-10, D-11), and the phase log (D-16).
  - Not in scope: Workspaces and `observe` (Phase 3); a plugin cache for OpenTofu (D-5); merging any Renovate PR but the engine re-pins (D-13); a real AWS account.

  *Cost if wrong:* none stored; a missed carry shows in the phase log's disposition list.
- **D-2. The harness reads the env variables with the reader App (owner's choice, 2026-11-20).**
  - The owner copies the reader App's private key and client ID into the `idp-engine` environment `idp-e2e`, as the secret `IDP_READER_PRIVATE_KEY` and the variable `IDP_READER_CLIENT_ID` (the names the claims repos use). The harness mints installation tokens from it with `repositories: [e2e-r<run>-app]` and `permissions: {environments: read}` (`readerPermissions`), lazily, at the first read, when the repository exists.
  - Why: the bot's tokens cover only `idp-claims-e2e` and have no environments permission (ADR-0027). The reader App is installed on all repositories (`docs/runbooks/bootstrap.md`, step 2), and the Phase 0 probe read an environment variable with it and got 200 (ADR-0013).
  - Rejected: a new read-only App (made by hand through the manifest flow, ADR-0011, for a read the reader already does) and a fine-grained PAT (it expires and is tied to a user).
  - Like the bot key, the reader key reaches only the `run the harness` step; the binary is built in an earlier step that sees no secret (`TestE2ESecretsReachOnlyTheHarness`, `TestE2EBuildsTheHarnessWithoutSecrets`).
  - **Docs:** spec amendment **A16** (§7.3's `idp-reader` row, §8.5 steps 4 and 5). The next free number is A16: A15 is the roadmap amendment of 2026-11-19. **No new ADR:** ADR-0027 holds the harness's credentials and records changes as dated in-place amendments ("*Amended in execution, …*"), as ADR-0026 does; it is amended in place (Task 5), with the two rejected alternatives under "Alternatives considered". ADR-0009 gets one sentence that points to it.

  - The reader's first token is minted at the first read, after the create step, so a wrong reader credential costs one run and its cleanup through the platform. A preflight probe would need a reader token for `idp-claims-e2e` too, which the owner's design does not grant; not built.

  *Cost if wrong:* a leaked copy reads what the reader reads across the org; the harness's tokens are scoped to one repository and one permission. Moving to a dedicated App later changes `Main`, two names in `e2e.yaml` and ADR-0027.
- **D-3. The harness's own Component gets `aws.registry: true`; `idp-claims-e2e` gets an `aws` block for `dev` (owner's choice, 2026-11-20).**
  - `ComponentClaim` always renders `aws:\n  registry: true\n`, so the create and the update keep it. No second Component: the existing one already lives in `dev`, and a second claim would double the AWS legs for no new check.
  - The owner's block in `idp-claims-e2e`'s `config/platform.yaml`: `dev: { aws: { accountId: "000000000004", region: eu-west-1 } }`. floci starts empty in every job, so `idp-claims`'s `dev` account (`000000000001`) would work too; a distinct ID keeps the shape of real AWS, where an account holds one GitHub OIDC provider and so one baseline (`docs/claims.md`).
  - The expected values come from that block, read on `main` in preflight; the formulas are spec §5.3's, written out in the harness (`arn:aws:iam::<account>:role/<component>-dev-ci`, `<account>.dkr.ecr.<region>.amazonaws.com/<component>`, `<region>`), not computed by `internal/naming`, so a naming regression cannot pass its own check.
  - Preflight refuses a config without that block: the check would otherwise silently test nothing.

  *Cost if wrong:* a second Component later is one claim and one more name in `Names`.
- **D-4. What the harness checks of the AWS part (from the code).**
  - **Create:** the plan comment's fingerprint (for the PR head) has a change for `aws/dev/components/e2e-r<run>-app`, and only creates; the reconcile's `reconcile / apply-aws aws/dev/components/e2e-r<run>-app` and `reconcile / apply-github` succeeded; the three variables match; `rendered/aws/dev/components/e2e-r<run>-app/main.tf.json` is on `wet`.
  - **Update and case-only:** the AWS stack is not affected (description, topics and memberships are not in it), so `plan-aws` and `apply-aws` are skipped, and the update still approves exactly the topics delete; the variables are checked again after the update.
  - **Delete:** a destroy is allowed only for this run's modules **in the `github` stack**, or anywhere in the Component's own AWS stack; never `aws/dev/_baseline`, which stays while the platform config has the block. Before, a matching module address was accepted in any stack; requiring the `github` stack is a deliberate tightening, pinned by a test. The main path requires `apply-aws` to succeed; both paths (main and cleanup) require the render to have left `wet`.
  - **Pin:** once `wet` holds `aws/dev/_baseline`, every pin PR re-renders its module `ref`, so it plans that baseline against floci too; it must still plan `No infrastructure changes.`, as today.

  *Cost if wrong:* a false failure costs one harness run and a cleanup through the platform; every check is one function and its test.
- **D-5. The live run closes Phase 2e's pending watch items.**
  - **A real AWS leg:** first in the reconcile of the owner's `aws` block PR (Task 4: a baseline created), then in every harness run (pin, create, delete).
  - **The provider downloads per leg:** counted from the logs of the first Component leg (`Installing hashicorp/aws`). A plugin cache is a follow-up only if `init` takes more than half of a leg; it would add `TF_PLUGIN_CACHE_DIR` to `passEnv` (D-6) and is carried, not built here.
  - **The single-artifact download layout:** the `gate` and `comment` jobs of a run with one AWS stack download one AWS plan by pattern; it lands flat in `aws-plans/`, and the layout step then warns for nothing (D-8 adds the warning).
  - **A `pattern` download that matches nothing:** not arranged live. It needs every `apply-aws` leg of a live reconcile to fail, which leaves `wet` behind the claims and needs a cleanup. Verified instead in `actions/download-artifact@9000827`'s `src/download-artifact.ts` (read on 2026-11-21): a pattern filters the run's artifact list, zero matches download nothing, raise no error and log `Total of 0 artifact(s) downloaded`; and `idp wet-root` reads a missing `--applied-dir` as no marker (`readMarkers`, pinned by `TestWetRootWithNothingToCommit`, whose inputs have no `applied` directory).

  *Cost if wrong:* none in production: `prepare the wet commit` runs on `always()` after that download either way.
- **D-6. OpenTofu gets an allowlist of the job's environment (carry R1-001).**
  - `passEnv`: `PATH`, `HOME` (git fetches the module sources), `TMPDIR`, `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` and their lowercase forms, `SSL_CERT_FILE` and `SSL_CERT_DIR`. Exact names: `PATHEXT` does not pass.
  - Everything else of the job (`GITHUB_TOKEN`, `ACTIONS_RUNTIME_TOKEN`, `ACTIONS_*`, any `AWS_*`, `TF_*`) stays out, so a secret added to a floci job by mistake never reaches OpenTofu. ADR-0031 still guarantees the job holds none; this is the second wall.
  - The CI floci job sets no `TF_*` variable for `TestReplay` (only `render-smoke` sets `TF_PLUGIN_CACHE_DIR`), so the rehearsal runs as before.

  *Cost if wrong:* a variable OpenTofu turns out to need on the runner fails `TestReplay` in CI; adding it is one list entry.
- **D-7. `recover` checks the artifact through the API before the download (carry R1).**
  - The issue body is mutable, so a new `artifact-check` step reads `GET /repos/<repo>/actions/artifacts/<id>` and requires `name == idp-wet-root`, `workflow_run.id == <run-id>` and `expired == false`, then outputs the ID; the download uses that output only. The API path is scoped to the repository, so another repository's artifact answers 404 and fails the step.
  - Within one run there is one `idp-wet-root`: the keep uploads with `overwrite: true` after every floci job ended (Phase 2e).
  - The runbook says how to find the run's own artifact ID when the issue was edited.

  *Cost if wrong:* a refused recovery costs one runbook step; the check is one step and its test.
- **D-8. No run script reads a process substitution (carry R4, generalized).**
  - `done < <(jq …)` hides a failed `jq`: bash ignores the exit status of `<(...)`. The layout step now assigns `jq`'s output first (`artifacts="$(jq …)"`), which fails the step under the runner's `bash -eo pipefail`, skips empty lines, and warns for each plan it does not find (`::warning::no regular file <A>.json among the downloaded AWS plans: plan-summary will fail for its stack`).
  - The same defect sits in `recover`'s run check (`read … < <(gh api …)`); it is fixed the same way.
  - A rule over every workflow and action step, `TestRunScriptsReadNoProcessSubstitution`, refuses `<(` from now on.

  *Cost if wrong:* none: the normal path is byte-identical in what it lays out and reads.
- **D-9. The order of the owner steps.** After PR 2 merges, every harness run requires the reader credentials and `dev`'s `aws` block, and a failing run turns release PR #28's `idp-e2e` red. So:
  1. PR 1 merges; the harness passes on the GitHub-only path (Task 4).
  2. **Owner:** the `aws` block PR in `idp-claims-e2e`; its reconcile applies `aws/dev/_baseline` against floci, the first live AWS leg (Task 4).
  3. **Owner:** the reader credentials into `idp-e2e` (Task 4). Unused until PR 2.
  4. PR 2 merges only after Task 9's read-only checks of steps 2 and 3; its merge dispatches the harness, the live AWS run (Task 9).

  *Cost if wrong:* a red harness on #28 until the missing step is done; nothing is applied.
- **D-10. Release `0.2.0` (Task 10).** Release PR #28 ("chore(main): release 0.2.0") merges only when its `idp-e2e` status is green on its current head, CI passed, it is up to date with `main`, **and the owner confirms**. Then: the release run tags `v0.2.0` and publishes six archives and `checksums.txt`; the controller verifies one archive locally (checksum, attestation, `idp version`), as Phase 1e did; the harness runs on `main` at the release commit to prove the release binary installs (`setup-idp: installed idp v0.2.0`). No release happens before Task 10.

  *Cost if wrong:* a bad release is fixed by a patch release; tags never move (ADR-0028).
- **D-11. The re-pins and `idp-claims`'s first AWS reconcile (Task 10).**
  - Both go through Renovate's "idp engine" PRs (no waiting period for engine releases), as `v0.1.1` did (`idp-claims#3`, `f38582c`): one in `idp-engine` for the four templates, one in `idp-claims` for the four callers. They come with the first Renovate run after the release: Monday's 04:17 UTC schedule, or a dispatch the owner confirms. The owner merges both. A hand-made PR with the same four lines is the fallback.
  - Before `idp-claims`'s PR merges: no open "Wet push failed" issue and no running pipeline there (runbook step 6: recover before moving the engine).
  - Expected plan (verified live on 2026-11-21: three environments with `aws` blocks, no claims, `wet` holds only `rendered/github`): `aws/dev/_baseline`, `aws/staging/_baseline` and `aws/pro/_baseline` each `+1 ~0 -0 ±0`, the `github` stack no change. Its reconcile runs three `plan-aws` and three `apply-aws` legs against floci and commits the three renders. Then the daily drift (05:23 UTC) must find nothing.

  *Cost if wrong:* a failed baseline leg keeps its render out of `wet`, and the next reconcile retries it (spec §6.2 step 4).
- **D-12. Delivery.** Two PRs, each green and shippable alone, each cut from `main` after the previous one merged, with merge commits (as 2e). Forecast authored lines:
  - **PR 1, Tasks 1 to 3** (branch `fix/phase-2f-aws-hardening`): about 230 lines, under the 400 band (open question 2). It changes no harness and needs no owner step.
  - **PR 2, Tasks 5 to 8** (branch `feat/phase-2f-e2e-aws`): about 580 lines, about 330 of them tests.
  - Tasks 4, 9, 10 and 11 are controller tasks: ship, owner steps, release, log.

  *Cost if wrong:* one PR is the same commits; the cut only decides when review and the live run happen.
- **D-13. Renovate and floci stay on the watch list.** The weekly run of Monday 2026-11-23, 04:17 UTC, is expected to open `hashicorp/aws` 6.72.0 as one "aws provider" PR labelled `needs-lockfile`; floci 2.5.0 becomes eligible on 2026-11-29 (the run of 2026-11-30) and must move `ci.yaml`, `pr.yaml` and `reconcile.yaml` together, which `TestFlociJobsHoldNoSecret` enforces. Neither is merged in 2f: a provider bump before the release would ride into `0.2.0` untested by the harness. The engine re-pins (D-11) are the only Renovate PRs 2f merges. While planning, the dependencies runbook's floci row was found stale (it names `ci.yaml` only); Task 8 fixes it.
- **D-14. The Phase 2e lessons, encoded.**
  - (a) GitHub skips a job when any job before it in its chain was skipped. 2f adds no job; `TestAJobAfterOneThatRunsPastASkipChecksItsOwnStatus` stays the guard for any job added later.
  - (b) A single artifact lands flat in `path`. `recover` downloads one artifact by ID into `wet-root/`, flat, as today; the new check does not change the download's layout.
  - (c) Mutation checks run only on committed code: each task commits, then mutates with Edit, runs, and restores with `git checkout -- <file>`; `git status --short` prints nothing after.
  - (d) The harness is the ship gate, and only a live run shows GitHub's job-skip and environment behavior. Every merge waits for its harness run before the next step (Tasks 4, 9, 10); a failure is fixed forward with a fix PR, as #42 did.
- **D-15. Time.** The harness run on `b0fb772` (run 44735548455) succeeded in 48 minutes (12:24:40 to 13:13:05 UTC). Forecast with the AWS checks, from 2e's D-9 leg times (`plan-aws` about 1.5 min, `apply-aws` about 1.7 min): one baseline leg in the pin PR and its reconcile, one Component leg in the create PR, its reconcile, the delete PR and its reconcile, about +14 minutes, so about 62 minutes. `DefaultTimeouts` are unchanged: a PR pipeline with one AWS leg takes about 4 minutes against the 15-minute `Checks` bound, and a reconcile about 7 against `Reconcile`'s 30.

  *Cost if wrong:* a bound that is too short fails a run at a wait and cleans up; the numbers are recorded in the watch list.
- **D-16. The phase log closes Phase 2.** `docs/phases/phase-2f.md` in the style of the 2e log, with a closing section "Phase 2 exit criterion" that names the evidence (the harness run with the AWS checks, CI's green `floci` job, the `v0.2.0` release). No separate `phase-2.md`: Phase 1 has none, and Phase 1e's log closed Phase 1.
- **D-17. Docs.** A16, ADR-0027 and ADR-0009 in Task 5; `docs/cli.md` with Task 1; `docs/pipelines.md` and the pipelines runbook with Task 3; `docs/e2e.md` and the dependencies runbook in Task 8; the README, `docs/pipelines.md`'s limitations and the phase log in Task 11.
- **D-18. Local tools (from the real setup).** OpenTofu 1.12.6 at `~/.idp/tools/tofu-1.12.6/tofu.exe` runs only from a copy in a `mktemp -d` directory; CI's 1.13.2 is authoritative. There is no local Docker: `TestReplay`, zizmor and shellcheck run only in CI; actionlint runs locally through `go run`.

## Global Constraints

**Names** (copy verbatim)
- Branches `fix/phase-2f-aws-hardening` (PR 1) and `feat/phase-2f-e2e-aws` (PR 2); each PR merged with a merge commit.
- `internal/awsstack`: `passEnv`; `Env(base, account)` (signature unchanged); test `TestEnv` (rewritten).
- `internal/cli`: test `TestAWSStackKeepsTheJobEnvironmentFromTofu`.
- `internal/workflowspec`: `TestRunScriptsReadNoProcessSubstitution` (new); `checkPlanLayout`, `TestRecoverChecksTheRunAndTheWetBase`, `TestE2EHarnessMintsTheBotToken`, `TestE2ESecretsReachOnlyTheHarness`, `e2eSecrets` (changed).
- Workflow step `artifact-check` in `recover.yaml` (output `artifact-id`); the shell variables `artifacts`, `info`.
- `internal/e2e`:
  - code: `readerPermissions`, `GitHub.Reader`, `Forge.EnvVariables(ctx, repo, env)`, `Names.AWSStack()`, `Names.AWSRender()`, `Scenario.aws`, `platformAWS`, `createsTheAWSStack`, `requireJobs`, `wantAWSRender`;
  - tests: `testKeyPEM`, `TestReaderPermissions`, `TestMainRefusesABadRunAsBefore`, `TestGitHubReadsEnvVariablesWithTheReaderToken`, `TestScenarioChecksTheEnvVariables`, `TestPreflightRequiresAnAWSBlockForDev`, `TestCreateRequiresTheAWSStackPlannedAndApplied`, `TestCreateRequiresTheAWSRenderOnWet`, `TestDeleteDestroysTheAWSStack`, `TestDeleteAllowsOnlyItsOwnAWSStack`; fake fields `vars`, `wet`, `jobs`.
- Harness environment variables `E2E_READER_CLIENT_ID`, `E2E_READER_PRIVATE_KEY`; `idp-e2e` environment secret `IDP_READER_PRIVATE_KEY` and variable `IDP_READER_CLIENT_ID`.
- Spec amendment **A16**; ADR-0027 and ADR-0009 amended in place; phase log `docs/phases/phase-2f.md`.

**Process**
- Strict TDD: failing test, run it to see it fail (RED observed), implement, run it to see it pass, commit. A test that pins existing behavior says so and is run before the change to see it pass.
- Mutation checks run only on committed code (D-14 c).
- Never build the binary: no `go build`, no `go run` of the CLI, no GoReleaser. Use `go test ./...`, `go vet ./...`, `gofmt -l .` (must print nothing) and `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`; `-race` runs in CI.
- Shell rule for every command: never `cat`, `grep`, `find`, `sed` or `ls`, never write files with shell heredocs, never `perl`. Use the Read/Write/Edit tools, `bat`, `rg`, `fd`, `sd`, `eza`, `jq`, `gh`, `git`.
- Conventional commits, never with Co-Authored-By or any AI attribution.
- Docs in English. Owner rule: docs describe the platform on its own terms, never naming another product as inspiration or comparison, and never use the owner's work account. The controller gives each dispatch the naming check that enforces both; it must print nothing over every changed file. The words it searches for are never written in this repository.
- Docs-only changes after a merge go straight to `main`; code goes through a PR.
- Never write into `~/.idp/`. Only the tofu binary there may be executed, from a copy in a `mktemp -d` directory. Reading `~/.idp/apps/*` is the owner's, in the owner-only steps.
- Scratch state lives in `mktemp -d` directories outside the worktree, removed after use. A downloaded release archive goes into its own new, empty directory.
- Every pin is at least 14 days old when adopted (spec §7.5); the engine's own releases are exempt. This plan adopts none.
- Nothing touches a real AWS account: every AWS call in this plan goes to a floci container in a job.
- Risk lines:
  - Tasks 5 and 8: `Risk: none`.
  - Task 1: `Risk: items 2 and 6 (what reaches OpenTofu; only CI's floci job runs the real tofu with the allowlist)`.
  - Task 2: `Risk: items 3 and 5 (the reusable workflows callers consume; CI)`.
  - Task 3: `Risk: items 1 and 2 (recover commits to wet; the guard on what it trusts)`.
  - Task 6: `Risk: item 2 (a credential reaches the harness)`.
  - Task 7: `Risk: item 5 (the harness gates every release)`.
  - Tasks 4, 9, 10 and 11: the live steps carry item 5 (release, the claims repos' pipelines); each is owner-confirmed where marked.

  Each PR's CI run and its whole-PR review check them.

**Spec rules**
- Nothing is ever applied that nobody saw (spec §6): the harness approves only what its checks bound.
- No real cloud credential anywhere (spec §7.6, 2a's D-2); the floci jobs hold no secret (ADR-0031).
- Tokens are minted per step with the narrowest scope that works (ADR-0023).
- The harness touches only `e2e-r<run>-*` names and never deletes a resource directly (ADR-0027).

**Owner-only steps** (marked **OWNER** where they occur; the controller prepares and reads, the owner acts or confirms)
1. Adding the `aws` block to `idp-claims-e2e` (Task 4 Step 6).
2. Pasting the reader credentials into `idp-e2e` (Task 4 Step 8).
3. Merging release PR #28 (Task 10 Step 2).
4. Merging the template re-pin in `idp-engine` and the re-pin in `idp-claims`, and dispatching Renovate if the scheduled run is not awaited (Task 10 Steps 5 and 6).
5. Approving any `idp-approval` deployment by hand, in any claims repo (the harness's own approvals use its approver token as designed).
6. Any cleanup of harness leftovers through a PR in `idp-claims-e2e`, and dispatching any workflow of `idp-claims` or `idp-claims-e2e`.

**Side effects this plan authorizes** (controller, after the owner approves the plan)
1. Push the two branches, open their PRs, and merge each after CI and the review (D-12).
2. Doc-only commits straight to `main`: this plan, the phase log, the README and `docs/pipelines.md`'s limitations.
3. Dispatch `e2e.yaml` on `idp-engine` `main` (the harness, which opens, merges and deletes `e2e-` claims in `idp-claims-e2e` as designed): once after the owner's `aws` block (Task 4 Step 7), once after the release (Task 10 Step 4), and once after each fix PR (Task 9 Step 6). The runs `release.yaml` dispatches after each merge are automatic.
4. Read-only `gh` calls against every repository of the org.
5. Delete the local, git-ignored SDD workspace `.superpowers/sdd/` after the phase log is committed.

Not authorized, stop and ask:
- anything in the owner-only list without the owner's go;
- merging any Renovate PR (the owner merges the two engine re-pins, D-11; nothing else merges in 2f, D-13);
- changing any repository or org setting, App, ruleset, environment, variable or secret beyond owner-only step 2;
- changing `ci.yaml`, `setup-idp`, the caller templates (beyond the re-pin), the modules or the floci image;
- creating any cloud resource outside floci.

## Review Focus

1. **A credential reaching more than it should.**
   - **Expected:** the reader key reaches only the harness step; each reader token is for `e2e-r<run>-app` only with `environments: read`; the variables are read with the reader token, never the bot's; no error repeats a key; OpenTofu receives no variable outside `passEnv`.
   - **Pinned by:** `TestE2ESecretsReachOnlyTheHarness`, `TestE2EHarnessMintsTheBotToken` (Task 6); `TestReaderPermissions`, `TestGitHubReadsEnvVariablesWithTheReaderToken`, `TestMainUsage`'s reader-key rows (Task 6); `TestEnv`, `TestAWSStackKeepsTheJobEnvironmentFromTofu` (Task 1).

     Mutation-checked in Tasks 1 and 6.
2. **The harness approving a destroy it does not own, now with AWS stacks.**
   - **Expected:** a delete may destroy only this run's modules in the `github` stack and its Component's own AWS stack; the `dev` baseline and any other stack are refused before the merge.
   - **Pinned by:** `TestDeleteAllowsOnlyItsOwnAWSStack`, `TestDeleteRefusesForeignDestroys` (Task 7).

     Mutation-checked in Task 7.
3. **A false pass on the AWS part.**
   - **Expected:** wrong or missing variables, a create plan without the AWS stack or with anything but creates, a failed `apply-aws`, a render missing from `wet` after the create or still there after the delete, and a config without `dev`'s `aws` block each fail the run.
   - **Pinned by:** `TestScenarioChecksTheEnvVariables`, `TestPreflightRequiresAnAWSBlockForDev`, `TestCreateRequiresTheAWSStackPlannedAndApplied`, `TestCreateRequiresTheAWSRenderOnWet`, `TestDeleteDestroysTheAWSStack` (Task 7); live in Task 9.

     Mutation-checked in Task 7.
4. **`recover` trusting an edited issue.**
   - **Expected:** the download uses only an ID the API names as that run's unexpired `idp-wet-root`.
   - **Pinned by:** `TestRecoverChecksTheRunAndTheWetBase` (Task 3).

     Mutation-checked in Task 3.
5. **A run step that hides a failure.**
   - **Expected:** no run script reads `<(`; the layout step fails on a `jq` error and warns for each plan it skips.
   - **Pinned by:** `TestRunScriptsReadNoProcessSubstitution`, `checkPlanLayout` (Task 2).

     Mutation-checked in Task 2.

Also tested, outside the five:
- a bad `--run` keeps exit 1 and its message (`TestMainRefusesABadRunAsBefore`, Task 6, run before the change);
- the harness's names for the AWS stack and its render (`TestNewNames`, Task 7);
- the Component claim always carries the registry (`TestComponentClaim`, Task 7).

## Watch list for the live steps

Record each result in the ledger:
- **Each PR's check list:** `go`, `workflows` (actionlint, zizmor and shellcheck; they see the new `artifact-check` step, the herestrings and the `::warning::` line), `modules (github/group)`, `modules (github/component)`, `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke`, `floci` and `release-snapshot`. The `floci` job of PR 1 runs `TestReplay` with the allowlist for the first time: 5 of 5 subtests pass, as in 2e.
- **After PR 1's merge (Task 4):** the harness run dispatched for #28 passes on the GitHub-only path.
- **The owner's `aws` block PR (Task 4):** `pr / plan-aws aws/dev/_baseline` and `reconcile / apply-aws aws/dev/_baseline` succeed (the first AWS legs in a real pipeline). Record each leg's duration against D-15's forecast, the `Installing hashicorp/aws` count in each leg's log, and whether the `gate` and `comment` logs carry any `no regular file` warning (expected: none; the single-pattern download landed flat).
- **The first live reader read (Task 9):** `GET …/environments/dev/variables` answers 200 with `environments: read` alone. A 403 follows open question 1.
- **After PR 2's merge (Task 9):** the harness passes with the AWS checks. In `idp-claims-e2e`'s runs record: the pin PR's baseline leg (no change), the create PR's `plan-aws` and its reconcile's `apply-aws` for `aws/dev/components/e2e-r<run>-app`, the update's skipped AWS jobs with its approval, the delete PR's orphan plan and its reconcile's destroy; the run's total time against D-15's 62 minutes; the provider downloads of a Component leg (two inits expected: the baseline's and the stack's).
- **Release PR number and version after each merge:** `#28 chore(main): release 0.2.0` until Task 10.
- **The release (Task 10):** the release run's `release-please` and `publish` succeed; six archives and `checksums.txt`; the post-release harness logs `setup-idp: installed idp v0.2.0`.
- **`idp-claims`'s first AWS reconcile (Task 10):** three `plan-aws` and three `apply-aws` legs succeed, `wet` holds the three baseline renders, and the next drift run is clean.
- **The weekly Renovate run of Monday 2026-11-23, 04:17 UTC** (scheduled runs can start hours late): read the Dependency Dashboard and the PRs it opens. `hashicorp/aws` 6.72.0 arrives as one "aws provider" PR labelled `needs-lockfile` (its checks fail until the lock file is regenerated); if `v0.2.0` exists by then, the two "idp engine" PRs arrive too (D-11). floci 2.5.0 is eligible from 2026-11-29 and must move `ci.yaml`, `pr.yaml` and `reconcile.yaml` together. Merge only the engine re-pins (D-13).
- **After the phase log's commit:** release-please may open a release PR for the `docs` commits after `v0.2.0`; leave it open (the owner decides).

## File Structure

```
internal/awsstack/exec.go, internal/awsstack/exec_test.go, internal/cli/awsstack_test.go, docs/cli.md        # Task 1
.github/workflows/pr.yaml, .github/workflows/reconcile.yaml, .github/workflows/recover.yaml,
internal/workflowspec/common_test.go, internal/workflowspec/floci_test.go, internal/workflowspec/recover_test.go  # Task 2
.github/workflows/recover.yaml, internal/workflowspec/recover_test.go, docs/pipelines.md,
docs/runbooks/pipelines.md                                                                                       # Task 3
docs/superpowers/specs/2026-10-08-idp-on-actions-design.md (A16), docs/adr/0027-end-to-end-harness.md,
docs/adr/0009-two-apps-and-split-environments.md                                                                 # Task 5
internal/e2e/{main.go,main_test.go,forge.go,github.go,github_test.go,fake_test.go}, .github/workflows/e2e.yaml,
internal/workflowspec/e2e_test.go                                                                                # Task 6
internal/e2e/{names.go,names_test.go,claims.go,claims_test.go,scenario.go,scenario_test.go,fake_test.go}       # Task 7
docs/e2e.md, docs/runbooks/dependencies.md                                                                       # Task 8
docs/phases/phase-2f.md (new), README.md, docs/pipelines.md                                                      # Task 11
```

Tasks 1 to 3 touch 12 distinct files (PR 1); Tasks 5 to 8 touch 19 (PR 2), none of them new. Task 11 adds the phase log.

## Pre-flight (controller, before Task 1)

- [ ] `gh api user --jq .login` prints `jellalshadows` (else `gh auth switch -u jellalshadows`).
- [ ] `git switch main && git pull --ff-only`, then `git log -1 --format=%h` prints `b0fb772` or a later commit, and `git status --short` prints nothing.
- [ ] `go version` prints `go1.26` or a later 1.26 patch, and `go test ./...` passes on `main`, so any later failure is this plan's.
- [ ] `TOFU_DIR="$(mktemp -d)" && cp "$HOME/.idp/tools/tofu-1.12.6/tofu.exe" "$TOFU_DIR/" && "$TOFU_DIR/tofu.exe" version | head -1 && rm -rf "$TOFU_DIR"` prints `OpenTofu v1.12.6` (D-18).
- [ ] `gh pr view 28 --repo jellalshadows-idp/idp-engine --json state,title --jq '"\(.state) \(.title)"'` prints `OPEN chore(main): release 0.2.0`.
- [ ] `gh run view 44735548455 --repo jellalshadows-idp/idp-engine --json conclusion,headSha --jq '"\(.conclusion) \(.headSha)"'` prints `success b0fb772e579dc55993e890f8984202f5b00a4fd1` (the harness on 2e's log commit, D-15).
- [ ] The live state, read-only (verified on 2026-11-21; stop and ask if any differs):
  - `gh api -H "Accept: application/vnd.github.raw" repos/jellalshadows-idp/idp-claims/contents/.github/workflows/pr.yaml | rg -o '@[0-9a-f]{40} # \S+'` prints `@150b7d35dc03831046d732a642e5ff03e33ce8bf # v0.1.1`, and so do `reconcile.yaml`, `drift.yaml` and `recover.yaml`;
  - `gh api -H "Accept: application/vnd.github.raw" repos/jellalshadows-idp/idp-claims-e2e/contents/config/platform.yaml | rg -n 'dev:'` prints `10:  dev: {}`;
  - `gh api "repos/jellalshadows-idp/idp-claims-e2e/git/trees/wet?recursive=1" --jq '[.tree[].path | select(startswith("rendered/aws"))] | length'` prints `0`, and the same for `idp-claims`;
  - `gh api orgs/jellalshadows-idp/installations --jq '.installations[] | select(.app_slug == "jellalshadows-idp-reader") | "\(.repository_selection) \(.permissions.environments)"'` prints `all read` (D-2: installed on all repositories, with environments read).
- [ ] `rg -n 'image: floci/floci' .github/workflows/ci.yaml .github/workflows/pr.yaml .github/workflows/reconcile.yaml` prints the same `floci/floci:2.4.0@sha256:5ee85db604c4aa74ac76e07ea5cc83086f37bb472bf3dbb97e34019e48efae9c` image four times (ci once, pr once, reconcile twice).
- [ ] The owner answers the two open questions and approves this plan, its side effects and the rulings D-1 to D-18; ledger them. Commit this plan to `main` as `docs: add the phase 2f plan` and push. The push refreshes release PR #28 and re-runs the harness on it; leave the PR open.

---

## PR 1: the Phase 2e carries (branch `fix/phase-2f-aws-hardening`)

- [ ] `git switch -c fix/phase-2f-aws-hardening main`

### Task 1: OpenTofu gets an allowlist of the job's environment (R1-001)

**Files:**
- Modify: `internal/awsstack/exec.go`, `internal/awsstack/exec_test.go`, `internal/cli/awsstack_test.go`, `docs/cli.md`

**Interfaces:**
- `var passEnv map[string]bool`, the names OpenTofu keeps.
- `func Env(base []string, account string) []string`: signature unchanged. It now keeps only the `passEnv` entries of `base`, in their order, then appends the same four values as today. Its two callers (`internal/cli/awsstack.go:80`, `internal/floci/replay_test.go:129`) are unchanged.

- [ ] **Step 1: Write the failing tests.** In `internal/awsstack/exec_test.go`, replace the whole `TestEnv` function with:

```go
// TestEnv: OpenTofu gets only the passEnv variables of the job's environment,
// then the account, the fixed secret access key and no prompt. A token, a
// state key or a real AWS credential in the job never reaches it (R1-001).
func TestEnv(t *testing.T) {
	base := []string{
		"PATH=/bin", "GITHUB_TOKEN=ghs_must_not_reach_tofu", "HOME=/home/runner", "TMPDIR=/tmp",
		"ACTIONS_RUNTIME_TOKEN=runtime", "ACTIONS_ID_TOKEN_REQUEST_URL=https://token.example",
		"HTTPS_PROXY=http://proxy:3128", "no_proxy=localhost", "SSL_CERT_FILE=/etc/ssl/cert.pem",
		"TF_ENCRYPTION=key_provider", "TF_PLUGIN_CACHE_DIR=/cache", "TF_CLI_CONFIG_FILE=/x",
		"AWS_ACCESS_KEY_ID=AKIAREAL", "AWS_SECRET_ACCESS_KEY=real", "AWS_SESSION_TOKEN=real", "AWS_PROFILE=prod",
		"PATHEXT=.EXE", "HOMEDRIVE=C:", "malformed",
	}
	kept := slices.Clone(base)
	tail := []string{"AWS_ACCESS_KEY_ID=000000000001", "AWS_SECRET_ACCESS_KEY=floci-does-not-check-it", "TF_IN_AUTOMATION=1", "TF_INPUT=0"}
	want := append([]string{"PATH=/bin", "HOME=/home/runner", "TMPDIR=/tmp", "HTTPS_PROXY=http://proxy:3128", "no_proxy=localhost", "SSL_CERT_FILE=/etc/ssl/cert.pem"}, tail...)
	if got := Env(base, "000000000001"); !reflect.DeepEqual(got, want) {
		t.Errorf("Env =\n%v\nwant\n%v", got, want)
	}
	if !reflect.DeepEqual(base, kept) {
		t.Errorf("Env changed its base: %v", base)
	}
	if got := Env(nil, "000000000001"); !reflect.DeepEqual(got, tail) {
		t.Errorf("Env(nil) = %v, want %v", got, tail)
	}
}
```

  Add `"slices"` to that file's imports. In `internal/cli/awsstack_test.go`, add after `TestAWSStackPlan`:

```go
// TestAWSStackKeepsTheJobEnvironmentFromTofu: idp aws-stack hands tofu only
// the allowlisted variables of its environment, so a token of the job never
// reaches OpenTofu (R1-001). TF_ENCRYPTION and IDP_STATE_PASSPHRASE are not
// set here: the command refuses to run with them (ADR-0031).
func TestAWSStackKeepsTheJobEnvironmentFromTofu(t *testing.T) {
	for _, k := range []string{"GITHUB_TOKEN", "ACTIONS_RUNTIME_TOKEN", "AWS_SESSION_TOKEN"} {
		t.Setenv(k, "must-not-reach-tofu")
	}
	s := useStubTofu(t, updateRole)
	dir, flags := awsStackDirs(t)
	var stdout, stderr bytes.Buffer
	args := append(append([]string{"aws-stack", "plan"}, flags...), "--out-dir", filepath.Join(dir, "plans"))
	if code := Run(args, &stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": filepath.Join(dir, "out")})); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	for _, kv := range s.env {
		if strings.Contains(kv, "must-not-reach-tofu") {
			t.Errorf("tofu env holds %s", kv)
		}
	}
	if !slices.Contains(s.env, "AWS_ACCESS_KEY_ID=000000000001") {
		t.Error("tofu env lacks the account's access key ID")
	}
}
```

- [ ] **Step 2: Run them to see them fail.**

```bash
go test ./internal/awsstack/ ./internal/cli/ -count=1 -run 'TestEnv$|TestAWSStackKeepsTheJobEnvironmentFromTofu'
```

  Expected: FAIL. `TestEnv` prints a `got` that still holds `GITHUB_TOKEN=ghs_must_not_reach_tofu` and the real AWS keys; the cli test prints `tofu env holds GITHUB_TOKEN=must-not-reach-tofu` (and the other two).

- [ ] **Step 3: Implement.** In `internal/awsstack/exec.go`, replace the `Env` doc comment and function with:

```go
// passEnv names the variables of the job's environment that a tofu run keeps:
// what OpenTofu, git (which fetches the module sources) and the AWS provider
// need on a Linux runner. Everything else stays out, a token, a state key or a
// real AWS credential included, so a secret added to a floci job by mistake
// never reaches OpenTofu (ADR-0031). The names are exact.
var passEnv = map[string]bool{
	"PATH": true, "HOME": true, "TMPDIR": true,
	"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true,
	"http_proxy": true, "https_proxy": true, "no_proxy": true,
	"SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
}

// Env is the environment of a tofu run against floci: the passEnv variables of
// base, in their order, then the account, as the access key ID (ADR-0004), and
// no prompt. It never changes base.
func Env(base []string, account string) []string {
	var env []string
	for _, kv := range base {
		if name, _, ok := strings.Cut(kv, "="); ok && passEnv[name] {
			env = append(env, kv)
		}
	}
	return append(env,
		"AWS_ACCESS_KEY_ID="+account,
		"AWS_SECRET_ACCESS_KEY="+fakeSecret,
		"TF_IN_AUTOMATION=1",
		"TF_INPUT=0",
	)
}
```

  Remove `"slices"` from that file's imports: `Env` was its only user (`rg -n 'slices\.' internal/awsstack/exec.go` prints nothing). In `docs/cli.md` line 160, replace `OpenTofu runs with `AWS_ACCESS_KEY_ID` set to the account (ADR-0004), a fixed secret access key that floci does not check, `TF_IN_AUTOMATION=1` and `TF_INPUT=0`; its output goes to stderr.` with:

```markdown
OpenTofu keeps only these variables of the job's environment: `PATH`, `HOME`, `TMPDIR`, `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` (and their lowercase forms), `SSL_CERT_FILE` and `SSL_CERT_DIR`. Every other variable stays out, a token or an `AWS_*` credential of the job included. It adds `AWS_ACCESS_KEY_ID` set to the account (ADR-0004), a fixed secret access key that floci does not check, `TF_IN_AUTOMATION=1` and `TF_INPUT=0`; its output goes to stderr.
```

  The sentence after it (`The command refuses to run when `TF_ENCRYPTION` …`) stays: the refusal is the first wall, the allowlist the second.

- [ ] **Step 4: Run them to see them pass, then commit.**

```bash
go test ./internal/awsstack/ ./internal/cli/ -count=1 -run 'TestEnv$|TestAWSStackKeepsTheJobEnvironmentFromTofu'
go test ./... -count=1
go vet ./...
gofmt -l .
git add internal/awsstack/exec.go internal/awsstack/exec_test.go internal/cli/awsstack_test.go docs/cli.md
git commit -m "fix(awsstack): pass tofu only an allowlist of the job environment"
```

  Expected: PASS; `gofmt -l .` prints nothing. `TestReplay` skips locally (no floci); the PR's `floci` job runs it with the allowlist (watch list).

- [ ] **Step 5: Mutation check (committed code).** With Edit, add `"GITHUB_TOKEN": true,` to `passEnv`; run Step 2's command: expected FAIL in both tests. Restore with `git checkout -- internal/awsstack/exec.go`. Then with Edit replace `var env []string` and its loop with `env := slices.Clip(base)` (adding the import back); expected FAIL again. Restore the same way; `git status --short` prints nothing.

**Risk:** items 2 and 6 (what reaches OpenTofu; only CI's `floci` job runs the real tofu with the allowlist).

### Task 2: no run script reads a process substitution (R4, generalized)

**Files:**
- Modify: `.github/workflows/pr.yaml`, `.github/workflows/reconcile.yaml`, `.github/workflows/recover.yaml`, `internal/workflowspec/common_test.go`, `internal/workflowspec/floci_test.go`, `internal/workflowspec/recover_test.go`

**Interfaces:** none in Go. Workflow scripts: the `layout` step of the `comment` job (`pr.yaml`) and the `gate` job (`reconcile.yaml`) read the matrix into `artifacts` first; `recover`'s `run-check` reads the run into `info` first. What they lay out and check is unchanged.

- [ ] **Step 1: Write the failing tests.** In `internal/workflowspec/common_test.go`, add after `TestRunScriptsTakeValuesFromEnv`:

```go
// TestRunScriptsReadNoProcessSubstitution: bash ignores the exit status of a
// process substitution, so `done < <(cmd)` or `read … < <(cmd)` carries on
// with empty input when cmd fails. A script assigns the output first, which
// fails the step under bash -e, and reads it from a here-string.
func TestRunScriptsReadNoProcessSubstitution(t *testing.T) {
	for _, l := range everyStep(t) {
		if strings.Contains(l.step.Run, "<(") {
			t.Errorf("%s: step %q reads a process substitution, which hides its command's failure; assign the output to a variable first", l.where, l.step.label())
		}
	}
}
```

  In `internal/workflowspec/floci_test.go`, in `checkPlanLayout`, replace the `wantRun(t, id+" layout", lay, …)` call with:

```go
	wantRun(t, id+" layout", lay,
		`artifacts="$(jq -r '.[].artifact' <<<"$AWS_STACKS")"`,
		`[ -n "$artifact" ] || continue`,
		`f="$RUNNER_TEMP/idp/aws-plans/$artifact.json"`,
		`if [ -f "$f" ] && [ ! -L "$f" ]; then`,
		`mkdir -p "$RUNNER_TEMP/idp/plans/idp-plan-$artifact"`,
		`cp "$f" "$RUNNER_TEMP/idp/plans/idp-plan-$artifact/$artifact.json"`,
		// A skipped plan is named: plan-summary then fails on that stack.
		`echo "::warning::no regular file $artifact.json among the downloaded AWS plans; plan-summary fails on the missing plan of that stack"`,
		`done <<<"$artifacts"`,
	)
```

  In `internal/workflowspec/recover_test.go`, in `TestRecoverChecksTheRunAndTheWetBase`, add as the first two strings of the `wantRun(t, "recover run-check", run, …)` call:

```go
		`info="$(gh api "repos/$GITHUB_REPOSITORY/actions/runs/$RUN_ID" --jq '[.event, .head_branch, (.conclusion // "")] | @tsv')"`,
		`read -r event branch conclusion <<<"$info"`,
```

- [ ] **Step 2: Run them to see them fail.**

```bash
go test ./internal/workflowspec/ -count=1 -run 'TestRunScriptsReadNoProcessSubstitution|TestPRPlanLayoutIsIndependentOfTheArtifactCount|TestReconcileGatePlanLayoutIsIndependentOfTheArtifactCount|TestRecoverChecksTheRunAndTheWetBase'
```

  Expected: FAIL. The new rule names three steps (`pr.yaml` job `comment` step `layout`, `reconcile.yaml` job `gate` step `layout`, `recover.yaml` step `run-check`), and the layout and run-check checks report the strings they lack.

- [ ] **Step 3: Implement.** In both `.github/workflows/pr.yaml` (lines 302 to 308) and `.github/workflows/reconcile.yaml` (lines 336 to 342), replace the loop of the `layout` step:

```bash
          while IFS= read -r artifact; do
            f="$RUNNER_TEMP/idp/aws-plans/$artifact.json"
            if [ -f "$f" ] && [ ! -L "$f" ]; then
              mkdir -p "$RUNNER_TEMP/idp/plans/idp-plan-$artifact"
              cp "$f" "$RUNNER_TEMP/idp/plans/idp-plan-$artifact/$artifact.json"
            fi
          done < <(jq -r '.[].artifact' <<<"$AWS_STACKS")
```

  with:

```bash
          # A failed jq fails the step here; a process substitution would hide it.
          artifacts="$(jq -r '.[].artifact' <<<"$AWS_STACKS")"
          while IFS= read -r artifact; do
            [ -n "$artifact" ] || continue
            f="$RUNNER_TEMP/idp/aws-plans/$artifact.json"
            if [ -f "$f" ] && [ ! -L "$f" ]; then
              mkdir -p "$RUNNER_TEMP/idp/plans/idp-plan-$artifact"
              cp "$f" "$RUNNER_TEMP/idp/plans/idp-plan-$artifact/$artifact.json"
            else
              echo "::warning::no regular file $artifact.json among the downloaded AWS plans; plan-summary fails on the missing plan of that stack"
            fi
          done <<<"$artifacts"
```

  Read the lines first (`bat -r 296:310 .github/workflows/pr.yaml`, `bat -r 330:344 .github/workflows/reconcile.yaml`) and keep the comment lines above the loop. In `.github/workflows/recover.yaml`, step `run-check`, replace:

```bash
          read -r event branch conclusion < <(gh api "repos/$GITHUB_REPOSITORY/actions/runs/$RUN_ID" --jq '[.event, .head_branch, (.conclusion // "")] | @tsv')
```

  with:

```bash
          info="$(gh api "repos/$GITHUB_REPOSITORY/actions/runs/$RUN_ID" --jq '[.event, .head_branch, (.conclusion // "")] | @tsv')"
          read -r event branch conclusion <<<"$info"
```

  Every workflow sets `defaults.run.shell: bash` (`bash --noprofile --norc -eo pipefail`), so a failed `gh` or `jq` in an assignment fails the step.

- [ ] **Step 4: Run them to see them pass, then commit.**

```bash
go test ./internal/workflowspec/ -count=1
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
rg -n '<\(' .github examples
git add .github/workflows/pr.yaml .github/workflows/reconcile.yaml .github/workflows/recover.yaml internal/workflowspec/common_test.go internal/workflowspec/floci_test.go internal/workflowspec/recover_test.go
git commit -m "fix(workflows): fail a run step whose jq or gh fails, and name each skipped AWS plan"
```

  Expected: PASS, actionlint prints nothing, `rg` prints nothing.

- [ ] **Step 5: Mutation check (committed code).** With Edit, put `done < <(jq -r '.[].artifact' <<<"$AWS_STACKS")` back as the last loop line in `reconcile.yaml` only; run `go test ./internal/workflowspec/ -count=1`: expected FAIL in the new rule and in the gate's layout check. Restore with `git checkout -- .github/workflows/reconcile.yaml`. Then delete the `echo "::warning::…"` line in `pr.yaml`: expected FAIL in the comment job's layout check. Restore; `git status --short` prints nothing.

**Risk:** items 3 and 5 (the reusable workflows callers consume; their CI behavior). The layout is unchanged when every plan is there.

### Task 3: `recover` checks the artifact the issue names through the API (R1)

**Files:**
- Modify: `.github/workflows/recover.yaml`, `internal/workflowspec/recover_test.go`, `docs/runbooks/pipelines.md`, `docs/pipelines.md`

**Interfaces:** a step `artifact-check` between `issue-check` and the engine checkout, with the output `artifact-id`; the `download` step reads `steps.artifact-check.outputs.artifact-id`. No permission changes: the job already has `actions: read`.

- [ ] **Step 1: Write the failing test.** In `internal/workflowspec/recover_test.go`, in `TestRecoverChecksTheRunAndTheWetBase`:
  - Replace the comment `// The ID the issue body names is the only artifact the recovery trusts.` with `// The issue body names the only artifact the recovery considers; artifact-check trusts it only once the API confirms it.`
  - Add after the `issue-check` block (after its `wantRun` call):

```go
	// The issue body can be edited by anyone with issue write access, so the
	// API must name the artifact as that run's unexpired idp-wet-root (D-7).
	iArt, art := mustStep(t, "recover", r.Steps, "artifact-check")
	wantEnv(t, "recover artifact-check", art, map[string]string{
		"GH_TOKEN":    "${{ github.token }}",
		"RUN_ID":      "${{ inputs.run-id }}",
		"ARTIFACT_ID": "${{ steps.issue-check.outputs.artifact-id }}",
	})
	wantRun(t, "recover artifact-check", art,
		`info="$(gh api "repos/$GITHUB_REPOSITORY/actions/artifacts/$ARTIFACT_ID" --jq '[.name, (.workflow_run.id | tostring), (.expired | tostring)] | @tsv')"`,
		`read -r name run expired <<<"$info"`,
		`if [ "$name" != idp-wet-root ] || [ "$run" != "$RUN_ID" ]; then`,
		`if [ "$expired" != false ]; then`,
		`echo "artifact-id=$ARTIFACT_ID" >> "$GITHUB_OUTPUT"`,
	)
```

  - In the `wantWith(t, "recover download", …)` map, change `"artifact-ids"` to `"${{ steps.artifact-check.outputs.artifact-id }}"`.
  - Replace the order check with:

```go
	if !(iReq < iRun && iRun < iIssue && iIssue < iArt && iArt < iDL && iDL < iBase && iBase < iPush) {
		t.Errorf("recover must check the request, the run, the issue and the artifact, then download, compare the base, and only then push (got %d %d %d %d %d %d %d)", iReq, iRun, iIssue, iArt, iDL, iBase, iPush)
	}
```

- [ ] **Step 2: Run it to see it fail.**

```bash
go test ./internal/workflowspec/ -count=1 -run TestRecoverChecksTheRunAndTheWetBase
```

  Expected: FAIL with `recover: no step with id "artifact-check"`.

- [ ] **Step 3: Implement.** In `.github/workflows/recover.yaml`, insert after the `issue-check` step (after its `echo "artifact-id=$artifact_id" >> "$GITHUB_OUTPUT"` line) and before the `actions/checkout` step:

```yaml
      - name: check that the artifact the issue names is that run's wet root
        id: artifact-check
        env:
          GH_TOKEN: ${{ github.token }}
          RUN_ID: ${{ inputs.run-id }}
          ARTIFACT_ID: ${{ steps.issue-check.outputs.artifact-id }}
        run: |
          # The issue body can be edited: its ID counts only once the API names
          # it as this run's unexpired idp-wet-root. The path is scoped to this
          # repository, so another repository's artifact answers 404.
          info="$(gh api "repos/$GITHUB_REPOSITORY/actions/artifacts/$ARTIFACT_ID" --jq '[.name, (.workflow_run.id | tostring), (.expired | tostring)] | @tsv')"
          read -r name run expired <<<"$info"
          if [ "$name" != idp-wet-root ] || [ "$run" != "$RUN_ID" ]; then
            echo "::error::artifact $ARTIFACT_ID, which the issue names, is not the idp-wet-root of run $RUN_ID (name: $name, run: $run): someone edited the issue. Follow the runbook (docs/runbooks/pipelines.md#a-wet-push-failed)."
            exit 1
          fi
          if [ "$expired" != false ]; then
            echo "::error::artifact $ARTIFACT_ID of run $RUN_ID has expired: the 7 days have passed. Follow the runbook (docs/runbooks/pipelines.md#a-wet-push-failed)."
            exit 1
          fi
          echo "artifact-id=$ARTIFACT_ID" >> "$GITHUB_OUTPUT"
```

  In the `download` step, change `artifact-ids: ${{ steps.issue-check.outputs.artifact-id }}` to `artifact-ids: ${{ steps.artifact-check.outputs.artifact-id }}`.

  Docs, each an exact replacement (old text first, then new):
  - `docs/runbooks/pipelines.md` step 2 (line 56):

```text
downloads the artifact by the ID the issue names (never by name),
```

```text
checks through the API that the artifact the issue names is that run's unexpired `idp-wet-root`, downloads it by that ID (never by name),
```

  - Same file, step 3 (line 57): the text after `3. ` starts with `If it refuses because `wet` moved`. Insert before it, on the same line and followed by one space:

```text
If it refuses because the artifact the issue names is not that run's `idp-wet-root`, someone edited the issue: find who in the issue's edit history, which also shows the ID the job wrote, put that line `idp-wet-root artifact id: <ID>` back only if the run's own listing names the same ID (`gh api repos/<owner>/<repo>/actions/runs/<run-id>/artifacts --jq '.artifacts[] | select(.name == "idp-wet-root") | .id'`), and run the workflow again.
```

  - `docs/pipelines.md` line 48:

```text
a run the open "Wet push failed" issue does not name, and any run after
```

```text
a run the open "Wet push failed" issue does not name, an artifact ID the API does not name as that run's unexpired `idp-wet-root`, and any run after
```

  - Same file, line 78:

```text
`idp-wet-root` is recovered by the ID its issue names;
```

```text
`idp-wet-root` is recovered by the ID its issue names, once the API confirms it is that run's `idp-wet-root`;
```

- [ ] **Step 4: Run it to see it pass, then commit.**

```bash
go test ./internal/workflowspec/ -count=1
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
git add .github/workflows/recover.yaml internal/workflowspec/recover_test.go docs/runbooks/pipelines.md docs/pipelines.md
git commit -m "fix(recover): check the artifact the issue names through the API before the download"
```

  Expected: PASS; actionlint prints nothing.

- [ ] **Step 5: Mutation check (committed code).** With Edit, point the download back at `steps.issue-check.outputs.artifact-id`; run Step 2's command: expected FAIL. Restore with `git checkout -- .github/workflows/recover.yaml`. Then delete the `|| [ "$run" != "$RUN_ID" ]` condition: expected FAIL. Restore; `git status --short` prints nothing.

**Risk:** items 1 and 2 (recover commits to `wet`; the guard on what it trusts). `TestReplay` and the live harness do not exercise `recover` (ADR-0027); its check is pinned by the rule test and by zizmor and shellcheck in CI.

### Task 4: Ship PR 1, then the owner steps (controller)

- [ ] **Step 1: The whole-branch checks.**

```bash
go test ./... -count=1
go vet ./...
gofmt -l .
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
git diff --stat main...HEAD
```

  Expected: all pass, `gofmt` prints nothing, the stat shows the 12 files of Tasks 1 to 3 and about 230 changed lines. The controller runs the naming check over `git diff --name-only main...HEAD`; it prints nothing.

- [ ] **Step 2: Review.** One whole-branch review against Review Focus 1, 4 and 5 (the reviewer reads this plan's D-6 to D-8 and the diff). Fix blockers in one batch, recheck only those (the correction bounds of the 2e plan).
- [ ] **Step 3: Push and open the PR.**

```bash
git push -u origin fix/phase-2f-aws-hardening
gh pr create --repo jellalshadows-idp/idp-engine --base main --head fix/phase-2f-aws-hardening \
  --title "fix: Phase 2f AWS hardening (tofu env allowlist, recover artifact check, no process substitution)" \
  --body-file "$PR_BODY"
```

  `PR_BODY` is a file the controller writes with the Write tool in a `mktemp -d` directory: the summary, the three carries with their Phase 2e log references, the checks run, and `Risk: items 1, 2, 3, 5 and 6` with one line each. No AI attribution.

- [ ] **Step 4: CI.** `gh pr checks <number> --repo jellalshadows-idp/idp-engine --watch` until every check of the watch list passes; the `floci` job's log shows `TestReplay` with 5 of 5 subtests passing. A failure is fixed on the branch with a new commit (no force-push).
- [ ] **Step 5: Merge and the harness.** `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge`. The merge updates release PR #28, and `release.yaml` dispatches the harness for it. Find the run with `gh run list --repo jellalshadows-idp/idp-engine --workflow e2e.yaml --limit 3` and watch it: `gh run watch <id> --repo jellalshadows-idp/idp-engine --exit-status`. Expected: success on the GitHub-only path (about 48 minutes), and `idp-e2e` green on #28's head. A failure stops here: read the harness log and fix forward before Step 6.
- [ ] **Step 6: OWNER, the `aws` block of `idp-claims-e2e`.** The owner opens a PR in `idp-claims-e2e` that changes `config/platform.yaml` line 10 from `  dev: {}` to:

```yaml
  dev:
    aws:
      accountId: "000000000004"
      region: eu-west-1
```

  The account ID is not `idp-claims`'s `dev` one (D-3). Before opening it: no harness run in progress (`gh run list --repo jellalshadows-idp/idp-engine --workflow e2e.yaml --status in_progress` prints nothing) and no pipeline running in `idp-claims-e2e`. The controller reads the PR's checks and plan comment: expected `aws/dev/_baseline: +1 ~0 -0 ±0` and no `github` change, `pr / plan-aws aws/dev/_baseline` green. **OWNER** merges it. Its reconcile runs `reconcile / plan-aws aws/dev/_baseline` and `reconcile / apply-aws aws/dev/_baseline` against floci, without approval (a create the PR showed). The controller checks:

```bash
gh run list --repo jellalshadows-idp/idp-claims-e2e --workflow reconcile.yaml --limit 1 --json databaseId,conclusion
gh run view <id> --repo jellalshadows-idp/idp-claims-e2e --json jobs --jq '.jobs[] | "\(.name) \(.conclusion)"'
gh api "repos/jellalshadows-idp/idp-claims-e2e/git/trees/wet?recursive=1" --jq '[.tree[].path | select(startswith("rendered/aws"))]'
```

  Expected: the run succeeded, its `apply-aws aws/dev/_baseline` leg succeeded, and `wet` lists `rendered/aws/dev/_baseline/main.tf.json`. Record the leg durations and the `Installing hashicorp/aws` count in each leg's log (`gh run view --log --job <job-id> … | rg -c 'Installing hashicorp/aws'`), and that the `gate` log has no `no regular file` warning. This is the first live AWS leg (D-5).
- [ ] **Step 7: The harness on the new config.** Dispatch the harness once: `gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main`. Expected: success. The harness of PR 1 does not read the `aws` block yet, but its pin PR now plans `aws/dev/_baseline` too (with no change), so the config change is proven before PR 2.
- [ ] **Step 8: OWNER, the reader credentials.** The owner copies the reader App's key and client ID into the `idp-engine` environment `idp-e2e` (D-2). A secret cannot be read back from GitHub, and the bootstrap runbook has the local `.pem` files deleted after the bootstrap, so the key comes from the owner's password manager (or `~/.idp/apps/jellalshadows-idp-reader.pem` if it is still there). If neither holds it, the owner generates a second private key in the reader App's settings: the existing key stays valid, so the claims repos keep working. The client ID is a variable and can be copied:

```bash
# Prompts for the key; paste the PEM. It is never written to disk.
gh secret set IDP_READER_PRIVATE_KEY --env idp-e2e --repo jellalshadows-idp/idp-engine
gh variable set IDP_READER_CLIENT_ID --env idp-e2e --repo jellalshadows-idp/idp-engine \
  --body "$(gh variable get IDP_READER_CLIENT_ID --repo jellalshadows-idp/idp-claims-e2e)"
```

  The controller checks, read-only: `gh secret list --env idp-e2e --repo jellalshadows-idp/idp-engine` lists `IDP_READER_PRIVATE_KEY`, and `gh variable list --env idp-e2e --repo jellalshadows-idp/idp-engine` lists `IDP_READER_CLIENT_ID` (with a value that starts with `Iv`). Nothing reads them until PR 2 merges.
- [ ] **Step 9: Ledger.** Record the PR number, merge commit, both harness runs, the baseline legs and the owner steps.

---

## PR 2: the harness checks the AWS part (branch `feat/phase-2f-e2e-aws`)

Start only after Task 4 is complete: PR 1 merged, the `aws` block applied in `idp-claims-e2e`, the reader credentials in `idp-e2e` (D-9).

- [ ] `git switch main && git pull --ff-only && git switch -c feat/phase-2f-e2e-aws`

### Task 5: spec amendment A16, ADR-0027 and ADR-0009

**Files:**
- Modify: `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, `docs/adr/0027-end-to-end-harness.md`, `docs/adr/0009-two-apps-and-split-environments.md`

**Interfaces:** none. Passive documentation: no RED; the checks are a structural readback.

- [ ] **Step 1: The spec (A16).** In §7.3, replace the `idp-reader` row:

```markdown
| `idp-reader` App key | Read only | Repo secret: PR plans, drift bootstrap check *(Amendment A7, 2026-10-12.)* |
```

  with:

```markdown
| `idp-reader` App key | Read only | Repo secret: PR plans, drift bootstrap check *(Amendment A7, 2026-10-12.)* A copy in the `idp-engine` environment `idp-e2e`, read only by the harness step, which mints tokens with `environments: read` for the repository its run created, to check that repository's env variables (ADR-0027) *(Amendment A16, 2026-11-20.)* |
```

  In §8.5, replace steps 4 and 5:

```markdown
4. Asserts with `gh api` that the repo, team, ruleset, environments and files exist.
5. Opens a PR that deletes the claims and verifies the cleanup.
```

  with:

```markdown
4. Asserts with `gh api` that the repo, team, ruleset, environments and files exist.
   Its Component has `aws.registry: true` in `dev`, which `idp-claims-e2e`'s platform
   config gives an `aws` block, so the harness also checks that the PR planned only
   creates for the Component's AWS stack, that the reconcile applied it and committed
   its render to `wet`, and that `dev` holds `AWS_ROLE_ARN`, `ECR_REPOSITORY` and
   `AWS_REGION` with the §5.3 values. It reads them with tokens of the `idp-reader`
   App, scoped to that repository and `environments: read` (§7.3).
5. Opens a PR that deletes the claims and verifies the cleanup, the AWS stack's
   destroy and its render's removal from `wet` included. *(Amendment A16, 2026-11-20.)*
```

- [ ] **Step 2: ADR-0027, amended in place.**
  - Decision, step 2: replace `and asserts the repository, team, memberships and environment through the API;` with `and asserts the repository, team, memberships and environment through the API, and, since the Component has `aws.registry: true`, that its AWS stack was planned with creates only, applied and rendered to `wet`, and that `dev` holds the three AWS variables of spec §5.3; *(Amended in Phase 2f, 2026-11-20.)*` (the marker after the `;`, as step 3 has it).
  - Decision, step 6: replace `and asserts the repository and team are gone;` with `and asserts the repository and team are gone, the AWS stack destroyed and its render gone from `wet`; the delete may destroy only this run's modules in the `github` stack and its Component's own AWS stack; *(Amended in Phase 2f, 2026-11-20.)*`.
  - Replace `- Credentials, both only in `idp-e2e`:` with `- Credentials, all three only in `idp-e2e`:`, and add after the approver token's bullet:

```markdown
  - the reader App token (ADR-0009), minted from a copy of the reader App's key for the run's Component repository only, with environments read, to read that repository's env variables: the bot's tokens cover only `idp-claims-e2e`. The harness mints it at the first read, after the create step, and renews it like the bot's. *(Amended in Phase 2f, 2026-11-20.)*
```

  - Consequences, under "Negative / costs", add:

```markdown
- A second copy of the reader App's key, in `idp-e2e`. A leaked copy reads what the reader App reads across the org; the harness's own tokens reach one repository with one read permission. A wrong reader credential fails a run only after the create step, and the run cleans up through the platform. *(Amended in Phase 2f, 2026-11-20.)*
```

  - Alternatives considered, add at the end:

```markdown
- **A new read-only App for the harness's reads.** Rejected in Phase 2f: one more App to create by hand through the manifest flow (ADR-0011) and to rotate, for a read the reader App already does.
- **A fine-grained token to read the env variables.** Rejected in Phase 2f: it expires, belongs to a user, and would need every repository of the org, since each run creates a new one.
```

- [ ] **Step 3: ADR-0009.** After the line `*(Amended by ADR-0025, 2026-10-12: …)*`, add a blank line and:

```markdown
*(Amended by ADR-0027 in Phase 2f, 2026-11-20: a copy of the `idp-reader` key also lives in the `idp-engine` environment `idp-e2e`, where only the harness step reads it.)*
```

- [ ] **Step 4: Readback and commit.**

```bash
rg -n 'Amendment A16' docs/superpowers/specs/2026-10-08-idp-on-actions-design.md
rg -c 'Amended in Phase 2f' docs/adr/0027-end-to-end-harness.md
rg -n 'ADR-0027 in Phase 2f' docs/adr/0009-two-apps-and-split-environments.md
git add docs/superpowers/specs/2026-10-08-idp-on-actions-design.md docs/adr/0027-end-to-end-harness.md docs/adr/0009-two-apps-and-split-environments.md
git commit -m "docs: amend the spec (A16) and ADR-0027 for the harness's reader token"
```

  Expected: two `Amendment A16` lines (§7.3 and §8.5), `4` for ADR-0027 (steps 2 and 6, the reader bullet, the cost), one line for ADR-0009. Read each changed paragraph back with `bat -r`; the controller runs the naming check over the three files.

**Risk:** none (documentation).

### Task 6: the harness mints reader tokens and reads env variables

**Files:**
- Modify: `internal/e2e/main.go`, `internal/e2e/main_test.go`, `internal/e2e/forge.go`, `internal/e2e/github.go`, `internal/e2e/github_test.go`, `internal/e2e/fake_test.go`, `.github/workflows/e2e.yaml`, `internal/workflowspec/e2e_test.go`

**Interfaces:**
- `var readerPermissions = map[string]string{"environments": "read"}`.
- `GitHub.Reader *ghapi.Client`: reader App tokens for the run's Component repository.
- `Forge.EnvVariables(ctx context.Context, repo, env string) (map[string]string, error)`: the variables of environment `env` of `<Owner>/<repo>`, by name.
- `Main` requires `E2E_READER_CLIENT_ID` and `E2E_READER_PRIVATE_KEY`, and calls `NewNames(*run)` after reading the templates, to scope the reader's source to `names.Component`.

  One deliberate ordering change, recorded here: a bad `--run` given together with a bad `--owner` or `--member` now reports the run first. Both still exit 1, and a bad `--run` alone keeps its exit code and message (pinned by `TestMainRefusesABadRunAsBefore`). `e2e.yaml` passes `$GITHUB_RUN_ID` and the default logins, so no run sees the difference.

- [ ] **Step 1: Pin the behavior that must not change.** In `internal/e2e/main_test.go`, add the imports `crypto/rand`, `crypto/rsa`, `crypto/x509`, `encoding/pem`, `os` and `path/filepath`, and add:

```go
// testKeyPEM is a fresh RSA key in PKCS#1 PEM, which ghapp.ParseKey accepts.
func testKeyPEM(t *testing.T) string {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}))
}

// TestMainRefusesABadRunAsBefore pins what a bad --run gets: exit 1 and the
// message NewNames gives, before any call to GitHub. Main derives the names
// itself once the reader's tokens are scoped to the run's repository.
func TestMainRefusesABadRunAsBefore(t *testing.T) {
	dir := t.TempDir()
	for n, b := range testTemplates() {
		if err := os.WriteFile(filepath.Join(dir, n), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	key := testKeyPEM(t)
	vars := map[string]string{
		"E2E_BOT_CLIENT_ID": "Iv23li", "E2E_BOT_PRIVATE_KEY": key,
		"E2E_READER_CLIENT_ID": "Iv23lr", "E2E_READER_PRIVATE_KEY": key,
		"E2E_APPROVER_TOKEN": "a", "IDP_GITHUB_API": "http://127.0.0.1:1",
	}
	var out, errb bytes.Buffer
	args := []string{"--repo", "acme/x", "--engine-sha", strings.Repeat("a", 40), "--run", "Bad_Run", "--templates", dir}
	code := Main(args, &out, &errb, func(k string) string { return vars[k] })
	if want := "idp-e2e: run \"Bad_Run\" must be 1 to 12 lowercase letters or digits\n"; code != 1 || errb.String() != want || out.Len() != 0 {
		t.Errorf("exit %d, stderr %q, stdout %q; want 1 and %q", code, errb.String(), out.String(), want)
	}
}
```

  Run it on today's code: `go test ./internal/e2e/ -count=1 -run TestMainRefusesABadRunAsBefore`. Expected: PASS (it pins existing behavior; `IDP_GITHUB_API` points at a closed port, so a call to GitHub would fail the test rather than reach it).

- [ ] **Step 2: Write the failing tests.**
  - In `internal/e2e/main_test.go`, `TestMainUsage`:
    - Add `"E2E_READER_CLIENT_ID": "Iv23lr", "E2E_READER_PRIVATE_KEY": "pem"` to the map in `with`.
    - Set `const missing = "idp-e2e: E2E_BOT_CLIENT_ID, E2E_BOT_PRIVATE_KEY, E2E_READER_CLIENT_ID, E2E_READER_PRIVATE_KEY and E2E_APPROVER_TOKEN must be set"`.
    - Replace `badKey` with:

```go
	badKey := func(name, pemText string) string {
		_, err := ghapp.ParseKey([]byte(pemText))
		if err == nil {
			t.Fatalf("ParseKey accepted %q", pemText)
		}
		return "idp-e2e: " + name + ": " + err.Error()
	}
	good := testKeyPEM(t)
	withGoodBot := func(k, v string) map[string]string {
		vars := with(k, v)
		vars["E2E_BOT_PRIVATE_KEY"] = good
		return vars
	}
```

    - Change the two existing key rows to `badKey("E2E_BOT_PRIVATE_KEY", garbage)` and `badKey("E2E_BOT_PRIVATE_KEY", corrupt)`, and add these rows after them:

```go
		{"no reader client id", args, with("E2E_READER_CLIENT_ID", ""), missing, ""},
		{"no reader key", args, with("E2E_READER_PRIVATE_KEY", ""), missing, ""},
		{"garbage reader key", args, withGoodBot("E2E_READER_PRIVATE_KEY", garbage), badKey("E2E_READER_PRIVATE_KEY", garbage), garbage},
		{"corrupt reader key", args, withGoodBot("E2E_READER_PRIVATE_KEY", corrupt), badKey("E2E_READER_PRIVATE_KEY", corrupt), "c2VjcmV0IGtleSBtYXRlcmlhbCBuZXZlciBwcmludGVk"},
```

  - In the same file, add:

```go
func TestReaderPermissions(t *testing.T) {
	if want := map[string]string{"environments": "read"}; !maps.Equal(readerPermissions, want) {
		t.Errorf("readerPermissions = %v, want %v: a reader token only reads the env variables of the run's repository (ADR-0027)", readerPermissions, want)
	}
}
```

  - In `internal/e2e/github_test.go`, give `forgeAgainst`'s `GitHub` the field `Reader: ghapi.New(srv.URL, "reader-token")` (and say "bot, approver and reader clients" in its comment), add `"maps"` to the imports, and add before `var _ Forge = GitHub{}`:

```go
func TestGitHubReadsEnvVariablesWithTheReaderToken(t *testing.T) {
	var auth string
	total := 2
	g := forgeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/acme/e2e-r42-app/environments/dev/variables" {
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			http.NotFound(w, r)
			return
		}
		auth = r.Header.Get("Authorization")
		fmt.Fprintf(w, `{"total_count":%d,"variables":[{"name":"AWS_REGION","value":"eu-west-1"},{"name":"ECR_REPOSITORY","value":"x"}]}`, total)
	})
	got, err := g.EnvVariables(context.Background(), "e2e-r42-app", "dev")
	if err != nil || !maps.Equal(got, map[string]string{"AWS_REGION": "eu-west-1", "ECR_REPOSITORY": "x"}) {
		t.Fatalf("EnvVariables = %v, %v", got, err)
	}
	if !strings.HasSuffix(auth, "reader-token") {
		t.Errorf("Authorization = %q: env variables are read with the reader token, never the bot's", auth)
	}
	total = 31
	if _, err := g.EnvVariables(context.Background(), "e2e-r42-app", "dev"); err == nil || !strings.Contains(err.Error(), "more than one page") {
		t.Errorf("31 variables: err = %v, want a refusal to check a partial list", err)
	}
}
```

  - In `internal/e2e/fake_test.go`, add the field `vars map[string]map[string]string // "repo/env" → its variables`, initialize it in `newFakeForge` (`vars: map[string]map[string]string{}`), add `"maps"` to the imports, and add:

```go
func (f *fakeForge) EnvVariables(_ context.Context, repo, env string) (map[string]string, error) {
	v, ok := f.vars[repo+"/"+env]
	if !ok {
		return nil, fmt.Errorf("GET the variables of %s's environment %s: 404", repo, env)
	}
	return maps.Clone(v), nil
}
```

  - In `internal/workflowspec/e2e_test.go`:
    - Set `var e2eSecrets = []string{"IDP_BOT_PRIVATE_KEY", "IDP_READER_PRIVATE_KEY", "IDP_E2E_APPROVER_TOKEN"}` and say "the bot and reader keys and the approver token" in `TestE2ESecretsReachOnlyTheHarness`'s comment.
    - Add to the `want` map of `TestE2EHarnessMintsTheBotToken`:

```go
		"E2E_READER_CLIENT_ID":   "${{ vars.IDP_READER_CLIENT_ID }}",
		"E2E_READER_PRIVATE_KEY": "${{ secrets.IDP_READER_PRIVATE_KEY }}",
```

    - Add to `wantConfig`:

```go
		"READER_CLIENT_ID": "${{ vars.IDP_READER_CLIENT_ID }}",
		"READER_KEY_SET":   "${{ secrets.IDP_READER_PRIVATE_KEY != '' }}",
```

    - Add after the `maps.Equal` check of `wantConfig`:

```go
	wantRun(t, "e2e config", c,
		`[ -n "$READER_CLIENT_ID" ] || missing="$missing variable IDP_READER_CLIENT_ID,"`,
		`[ "$READER_KEY_SET" = true ] || missing="$missing secret IDP_READER_PRIVATE_KEY,"`,
	)
```

- [ ] **Step 3: Run them to see them fail.**

```bash
go test ./internal/e2e/ ./internal/workflowspec/ -count=1
```

  Expected: FAIL. `internal/e2e`'s tests do not compile (`undefined: readerPermissions`, `unknown field Reader in struct literal`, `g.EnvVariables undefined`; the fake's extra method compiles). `internal/workflowspec` fails `TestE2EHarnessMintsTheBotToken`, `TestE2ESecretsReachOnlyTheHarness` (`IDP_READER_PRIVATE_KEY is read by []`) and the config's two missing run lines.

- [ ] **Step 4: Implement.**
  - `internal/e2e/forge.go`: add to the `Forge` interface, after `ActiveRuns`:

```go
	// EnvVariables returns the variables of environment env of the org's
	// repository repo, by name. GitHub reads them with the reader App's token.
	EnvVariables(ctx context.Context, repo, env string) (map[string]string, error)
```

  - `internal/e2e/github.go`: replace the `GitHub` comment and struct with:

```go
// GitHub implements Forge over the REST API with the three tokens of ADR-0027:
// Bot for everything except approvals and env variables, Approver for
// idp-approval only, and Reader for the env variables of the run's Component
// repository.
type GitHub struct {
	Bot      *ghapi.Client
	Approver *ghapi.Client
	Reader   *ghapi.Client
	Owner    string
	Repo     string
}
```

    and add after `ActiveRuns`:

```go
// EnvVariables reads one page of environment variables, the most GitHub
// returns (30), and refuses a longer list rather than check part of it.
func (g GitHub) EnvVariables(ctx context.Context, repo, env string) (map[string]string, error) {
	if g.Reader == nil {
		return nil, errors.New("no reader client: env variables are read with the reader App's token (ADR-0027)")
	}
	var resp struct {
		TotalCount int `json:"total_count"`
		Variables  []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"variables"`
	}
	p := "/repos/" + g.Owner + "/" + url.PathEscape(repo) + "/environments/" + url.PathEscape(env) + "/variables?per_page=30"
	if err := g.read(ctx, func() error { return g.Reader.Get(ctx, p, &resp) }); err != nil {
		return nil, fmt.Errorf("read the variables of %s's environment %s: %w", repo, env, err)
	}
	if resp.TotalCount > len(resp.Variables) {
		return nil, fmt.Errorf("%s's environment %s has %d variables, more than one page", repo, env, resp.TotalCount)
	}
	out := map[string]string{}
	for _, v := range resp.Variables {
		out[v.Name] = v.Value
	}
	return out, nil
}
```

  - `internal/e2e/main.go`:
    - Add after `botPermissions`:

```go
// readerPermissions scope every reader token to reading environment variables
// (ADR-0027); each token is also limited to the run's Component repository.
var readerPermissions = map[string]string{"environments": "read"}
```

    - In the `Main` doc comment, replace `and the approver token in E2E_APPROVER_TOKEN.` with `the reader App's in E2E_READER_CLIENT_ID and E2E_READER_PRIVATE_KEY, and the approver token in E2E_APPROVER_TOKEN.`, and add after the sentence that ends `a run can last longer.`: `It mints the reader's tokens the same way, for the repository the run creates only, with readerPermissions.`
    - Replace the block from `clientID, keyPEM, approver := …` through the `E2E_BOT_PRIVATE_KEY` parse error's `return 2` and its `}` with:

```go
	clientID, keyPEM, approver := getenv("E2E_BOT_CLIENT_ID"), getenv("E2E_BOT_PRIVATE_KEY"), getenv("E2E_APPROVER_TOKEN")
	readerID, readerPEM := getenv("E2E_READER_CLIENT_ID"), getenv("E2E_READER_PRIVATE_KEY")
	if clientID == "" || keyPEM == "" || readerID == "" || readerPEM == "" || approver == "" {
		fmt.Fprintln(stderr, "idp-e2e: E2E_BOT_CLIENT_ID, E2E_BOT_PRIVATE_KEY, E2E_READER_CLIENT_ID, E2E_READER_PRIVATE_KEY and E2E_APPROVER_TOKEN must be set")
		return 2
	}
	key, err := ghapp.ParseKey([]byte(keyPEM))
	if err != nil {
		fmt.Fprintln(stderr, "idp-e2e: E2E_BOT_PRIVATE_KEY:", err) // the error never repeats the key
		return 2
	}
	readerKey, err := ghapp.ParseKey([]byte(readerPEM))
	if err != nil {
		fmt.Fprintln(stderr, "idp-e2e: E2E_READER_PRIVATE_KEY:", err) // the error never repeats the key
		return 2
	}
```

    - After the `readTemplates` block, add:

```go
	names, err := NewNames(*run)
	if err != nil {
		fmt.Fprintln(stderr, "idp-e2e:", err)
		return 1
	}
```

    - Replace the `src := …` line and the `F:` line with:

```go
	src := &ghapp.Source{BaseURL: base, ClientID: clientID, Key: key, Owner: org, Repo: name, Permissions: botPermissions}
	reader := &ghapp.Source{BaseURL: base, ClientID: readerID, Key: readerKey, Owner: org, Repo: names.Component, Permissions: readerPermissions}
	s := &Scenario{
		F: GitHub{
			Bot: ghapi.NewWithTokens(base, src.Token), Approver: ghapi.New(base, approver),
			Reader: ghapi.NewWithTokens(base, reader.Token), Owner: org, Repo: name,
		},
```

      keeping the two lines after `F:` unchanged. The reader source mints lazily, at the first read, when the repository exists.
  - `.github/workflows/e2e.yaml`, step `config`: add to its `env` after `BOT_KEY_SET`:

```yaml
          READER_CLIENT_ID: ${{ vars.IDP_READER_CLIENT_ID }}
          READER_KEY_SET: ${{ secrets.IDP_READER_PRIVATE_KEY != '' }}
```

    and to its script after the `IDP_BOT_PRIVATE_KEY` line:

```bash
          [ -n "$READER_CLIENT_ID" ] || missing="$missing variable IDP_READER_CLIENT_ID,"
          [ "$READER_KEY_SET" = true ] || missing="$missing secret IDP_READER_PRIVATE_KEY,"
```

    Step `harness`: replace the three comment lines above it with:

```yaml
      # The harness mints the bot's installation tokens from the App key and
      # renews them before the hour runs out, each scoped to idp-claims-e2e and
      # the permissions the forge calls. It mints the reader's the same way, for
      # the repository the run creates and environments read only. The keys
      # reach only this step.
```

    and add to its `env` after `E2E_BOT_PRIVATE_KEY`:

```yaml
          E2E_READER_CLIENT_ID: ${{ vars.IDP_READER_CLIENT_ID }}
          E2E_READER_PRIVATE_KEY: ${{ secrets.IDP_READER_PRIVATE_KEY }}
```

- [ ] **Step 5: Run them to see them pass, then commit.**

```bash
go test ./internal/e2e/ ./internal/workflowspec/ -count=1
go test ./... -count=1
go vet ./...
gofmt -l .
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
git add internal/e2e/main.go internal/e2e/main_test.go internal/e2e/forge.go internal/e2e/github.go internal/e2e/github_test.go internal/e2e/fake_test.go .github/workflows/e2e.yaml internal/workflowspec/e2e_test.go
git commit -m "feat(e2e): mint reader tokens for the run's repository and read its env variables"
```

  Expected: PASS, `gofmt` and actionlint print nothing. `TestMainRefusesABadRunAsBefore` still passes unchanged.

- [ ] **Step 6: Mutation check (committed code).** With Edit, change `readerPermissions` to `{"environments": "read", "actions_variables": "read"}`: `TestReaderPermissions` FAILS. Restore with `git checkout -- internal/e2e/main.go`. In `EnvVariables`, use `g.Bot.Get` instead of `g.Reader.Get`: `TestGitHubReadsEnvVariablesWithTheReaderToken` FAILS. Restore. In `e2e.yaml`, move `E2E_READER_PRIVATE_KEY` from the harness step's env to the build step's: `TestE2ESecretsReachOnlyTheHarness` FAILS. Restore; `git status --short` prints nothing.

**Risk:** item 2 (a credential reaches the harness).

### Task 7: the harness checks the Component's AWS part

**Files:**
- Modify: `internal/e2e/names.go`, `internal/e2e/names_test.go`, `internal/e2e/claims.go`, `internal/e2e/claims_test.go`, `internal/e2e/scenario.go`, `internal/e2e/scenario_test.go`, `internal/e2e/fake_test.go`

**Interfaces:**
- `func (n Names) AWSStack() string` (`aws/dev/components/<Component>`), `func (n Names) AWSRender() string` (`rendered/<AWSStack>/main.tf.json`).
- `ComponentClaim` always renders `aws:\n  registry: true\n` last.
- `Scenario.aws claims.AWS`, set in preflight by `platformAWS`.
- `createsTheAWSStack(comment string, pr PR) error`, `requireJobs(jobs map[string]string, names ...string) error`, `wantAWSRender(ctx, want bool) error`.
- `destroysOnlyOwnNames`: allows this run's modules in the `github` stack and anything in `AWSStack()`; refuses everything else, the `dev` baseline included. Its error keeps the format `the delete would destroy something the harness does not own: <stack>: <action> <address>`.

#### 7a: the names and the claim

- [ ] **Step 1: Write the failing tests.** In `internal/e2e/names_test.go`, `TestNewNames`, add after the paths check:

```go
	if n.AWSStack() != "aws/dev/components/e2e-r38314547633-app" || n.AWSRender() != "rendered/aws/dev/components/e2e-r38314547633-app/main.tf.json" {
		t.Errorf("AWS stack %s, render %s", n.AWSStack(), n.AWSRender())
	}
```

  In `internal/e2e/claims_test.go`, `TestComponentClaim`, append `aws:\n  registry: true\n` to both expected strings: the first becomes `"…github:\n  topics: [e2e, harness]\naws:\n  registry: true\n"`, the second `"…environments: [dev]\naws:\n  registry: true\n"`.
- [ ] **Step 2: Run them to see them fail.** `go test ./internal/e2e/ -count=1 -run 'TestNewNames|TestComponentClaim'`. Expected: FAIL to compile (`n.AWSStack undefined`).
- [ ] **Step 3: Implement.** In `internal/e2e/names.go`, add at the end:

```go
// AWSStack is the Component's AWS stack: it has aws.registry in dev, the only
// environment the harness's Component lives in (spec §5.4).
func (n Names) AWSStack() string { return "aws/dev/components/" + n.Component }

// AWSRender is the path of that stack's render on wet.
func (n Names) AWSRender() string { return "rendered/" + n.AWSStack() + "/main.tf.json" }
```

  In `internal/e2e/claims.go`, replace the `ComponentClaim` comment with `// ComponentClaim renders a Component claim in environment dev with an ECR registry; no topics means no github block.` and add before `return []byte(b.String())`:

```go
	b.WriteString("aws:\n  registry: true\n")
```

- [ ] **Step 4: Run them to see them pass, then commit.**

```bash
go test ./internal/e2e/ -count=1
git add internal/e2e/names.go internal/e2e/names_test.go internal/e2e/claims.go internal/e2e/claims_test.go
git commit -m "feat(e2e): give the harness's Component an ECR registry in dev"
```

#### 7b: the checks

- [ ] **Step 5: Write the failing tests.**
  - `internal/e2e/fake_test.go`:
    - Add the fields `wet map[string][]byte // files on wet` and `jobs map[string]map[string]string // slug → reconcile jobs added to or overriding the defaults`, initialized in `newFakeForge` (`wet: map[string][]byte{}, jobs: map[string]map[string]string{}`).
    - Replace `File` with:

```go
func (f *fakeForge) File(_ context.Context, ref, path string) ([]byte, bool, error) {
	files, ok := map[string]map[string][]byte{"main": f.main, "wet": f.wet}[ref]
	if !ok {
		return nil, false, fmt.Errorf("unexpected ref %s", ref)
	}
	b, ok := files[path]
	return b, ok, nil
}
```

    - In `Jobs`, replace its `return` with:

```go
	jobs := map[string]string{"reconcile / plan": "success", "reconcile / approve": approve, "reconcile / apply-github": "success"}
	maps.Copy(jobs, f.jobs[f.merged[fakeSHA(int(id-500))]])
	return jobs, nil
```

  - `internal/e2e/scenario_test.go`, in `world`, add `"maps"` to the file's imports and, after the `repo, team := …` line:

```go
	f.main["config/platform.yaml"] = []byte("apiVersion: idp/v1\nkind: Platform\nenvironments:\n  dev:\n    aws:\n      accountId: \"000000000004\"\n      region: eu-west-1\n")
	stack := "aws/dev/components/e2e-r42-app"
	render := "rendered/" + stack + "/main.tf.json"
	// Spec §5.3's values for e2e-r42-app in dev, written out.
	awsVars := map[string]string{
		"AWS_ROLE_ARN":   "arn:aws:iam::000000000004:role/e2e-r42-app-dev-ci",
		"ECR_REPOSITORY": "000000000004.dkr.ecr.eu-west-1.amazonaws.com/e2e-r42-app",
		"AWS_REGION":     "eu-west-1",
	}
	f.fps["create"] = plan.Fingerprint{
		"github": {{Address: "module.component_e2e-r42-app.github_repository.this", Action: plan.Create}},
		stack: {
			{Address: "module.component.aws_ecr_repository.this", Action: plan.Create},
			{Address: "module.component.aws_iam_role.ci", Action: plan.Create},
			{Address: "module.component.aws_iam_role_policy.push", Action: plan.Create},
		},
	}
	f.jobs["create"] = map[string]string{"reconcile / apply-aws " + stack: "success"}
	f.jobs["delete"] = map[string]string{"reconcile / apply-aws " + stack: "success"}
```

    In the create effect add `f.vars["e2e-r42-app/dev"] = maps.Clone(awsVars)` and `f.wet[render] = []byte("{}")`. Add to `f.fps["delete"]` the entry:

```go
		stack: {
			{Address: "module.component.aws_ecr_repository.this", Action: plan.Delete},
			{Address: "module.component.aws_iam_role.ci", Action: plan.Delete},
			{Address: "module.component.aws_iam_role_policy.push", Action: plan.Delete},
		},
```

    In the delete effect add `delete(f.wet, render)` and `delete(f.vars, "e2e-r42-app/dev")`.
  - At the end of `internal/e2e/scenario_test.go`, add:

```go
const testStack = "aws/dev/components/e2e-r42-app"

// TestScenarioChecksTheEnvVariables: after the create and the update, dev must
// hold exactly spec §5.3's three AWS variables, for the platform's dev account
// and region.
func TestScenarioChecksTheEnvVariables(t *testing.T) {
	for name, edit := range map[string]func(map[string]string){
		"wrong role":     func(v map[string]string) { v["AWS_ROLE_ARN"] = "arn:aws:iam::000000000004:role/e2e-r42-app-ci" },
		"no registry":    func(v map[string]string) { delete(v, "ECR_REPOSITORY") },
		"other region":   func(v map[string]string) { v["AWS_REGION"] = "us-east-1" },
		"extra variable": func(v map[string]string) { v["AWS_ACCOUNT_ID"] = "000000000004" },
	} {
		f, s := world(t)
		create := f.effects["create"]
		f.effects["create"] = func(f *fakeForge) { create(f); edit(f.vars["e2e-r42-app/dev"]) }
		err := s.Execute(context.Background())
		if err == nil || !strings.Contains(err.Error(), "create: e2e-r42-app's environment dev variables = ") || slices.Contains(f.log, "pr update") {
			t.Errorf("%s: err = %v: the create must fail before the update", name, err)
		}
	}
	f, s := world(t)
	update := f.effects["update"]
	f.effects["update"] = func(f *fakeForge) { update(f); delete(f.vars["e2e-r42-app/dev"], "AWS_REGION") }
	if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "update: e2e-r42-app's environment dev variables = ") {
		t.Errorf("update: err = %v: the variables are checked after the update too", err)
	}
}

func TestPreflightRequiresAnAWSBlockForDev(t *testing.T) {
	for name, cfg := range map[string]string{
		"no aws block": "environments:\n  dev: {}\n",
		"no region":    "environments:\n  dev:\n    aws:\n      accountId: \"000000000004\"\n",
		"no dev":       "environments:\n  staging:\n    aws:\n      accountId: \"000000000004\"\n      region: eu-west-1\n",
	} {
		f, s := world(t)
		f.main["config/platform.yaml"] = []byte(cfg)
		err := s.Execute(context.Background())
		if err == nil || !strings.Contains(err.Error(), "preflight: config/platform.yaml gives dev no aws block") || len(f.log) > 0 {
			t.Errorf("%s: err = %v, steps %v: the run must stop before it creates anything", name, err, f.log)
		}
	}
	f, s := world(t)
	delete(f.main, "config/platform.yaml")
	if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "preflight: config/platform.yaml is not on main") {
		t.Errorf("no config: err = %v", err)
	}
}

func TestCreateRequiresTheAWSStackPlannedAndApplied(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*fakeForge)
		want   string
		merged bool
	}{
		{"not planned", func(f *fakeForge) { delete(f.fps["create"], testStack) }, "create: the plan has no change for " + testStack, false},
		{"an update", func(f *fakeForge) { f.fps["create"][testStack][0].Action = plan.Update }, "create: the plan would update module.component.aws_ecr_repository.this in " + testStack, false},
		// GitHub skips a job after a skipped one (Phase 2e lesson a): the
		// run succeeds, and the leg keeps its unexpanded name.
		{"apply skipped", func(f *fakeForge) { delete(f.jobs["create"], "reconcile / apply-aws "+testStack) }, "create: apply-aws " + testStack + ": ", true},
	} {
		f, s := world(t)
		tt.mutate(f)
		err := s.Execute(context.Background())
		if err == nil || !strings.Contains(err.Error(), tt.want) || slices.Contains(f.log, "merge create") != tt.merged {
			t.Errorf("%s: err = %v, merged %v, want %q", tt.name, err, slices.Contains(f.log, "merge create"), tt.want)
		}
	}
}

func TestCreateRequiresTheAWSRenderOnWet(t *testing.T) {
	f, s := world(t)
	create := f.effects["create"]
	f.effects["create"] = func(f *fakeForge) { create(f); delete(f.wet, "rendered/"+testStack+"/main.tf.json") }
	err := s.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "create: wet lacks rendered/"+testStack+"/main.tf.json after the reconcile") {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteDestroysTheAWSStack(t *testing.T) {
	render := "rendered/" + testStack + "/main.tf.json"
	t.Run("the destroy was skipped", func(t *testing.T) {
		f, s := world(t)
		delete(f.jobs["delete"], "reconcile / apply-aws "+testStack)
		if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "delete: apply-aws "+testStack+": ") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("the render stayed on wet", func(t *testing.T) {
		f, s := world(t)
		del := f.effects["delete"]
		f.effects["delete"] = func(f *fakeForge) { del(f); f.wet[render] = []byte("{}") }
		if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "delete: "+render+" is still on wet after the delete") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("the cleanup checks the render too", func(t *testing.T) {
		f, s := world(t)
		f.gate["update"] = "failure"
		del := f.effects["delete"]
		f.effects["delete"] = func(f *fakeForge) { del(f); f.wet[render] = []byte("{}") }
		if err := s.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "cleanup: "+render+" is still on wet after the delete") {
			t.Errorf("err = %v", err)
		}
	})
}

// TestDeleteAllowsOnlyItsOwnAWSStack: the delete may destroy its Component's
// AWS stack, never the dev baseline, another Component's stack or an own name
// in another environment. A module address of this run counts only in the
// github stack (a deliberate tightening, D-4).
func TestDeleteAllowsOnlyItsOwnAWSStack(t *testing.T) {
	for _, foreign := range []struct{ stack, address string }{
		{"aws/dev/_baseline", "module.baseline.aws_iam_openid_connect_provider.this"},
		{"aws/dev/components/e2e-r43-app", "module.component.aws_ecr_repository.this"},
		{"aws/staging/components/e2e-r42-app", "module.component.aws_ecr_repository.this"},
		{"aws/dev/components/api", "module.component_e2e-r42-app.github_repository.this"},
	} {
		f, s := world(t)
		f.fps["delete"][foreign.stack] = []plan.Change{{Address: foreign.address, Action: plan.Delete}}
		err := s.Execute(context.Background())
		want := "does not own: " + foreign.stack + ": delete " + foreign.address
		if err == nil || !strings.Contains(err.Error(), want) || slices.Contains(f.log, "merge delete") {
			t.Errorf("%s: err = %v, want %q and no merge", foreign.stack, err, want)
		}
	}
}
```

- [ ] **Step 6: Run them to see them fail.**

```bash
go test ./internal/e2e/ -count=1
```

  Expected: FAIL. The new tests fail because no AWS check exists yet. Every test whose run reaches the delete fails too, the happy path included, with `the delete would destroy something the harness does not own: aws/dev/components/e2e-r42-app: delete module.component.aws_ecr_repository.this`: today's check refuses the Component's own AWS stack, which the world now plans. Record both failures as the RED.

- [ ] **Step 7: Implement.** In `internal/e2e/scenario.go`:
  - Imports: add `"github.com/jellalshadows-idp/idp-engine/internal/claims"` and `"go.yaml.in/yaml/v3"`.
  - `Scenario`: add after `names     Names` the field `aws       claims.AWS     // dev's aws block in the claims repo's platform config`; run `gofmt -w internal/e2e/scenario.go` after editing to realign the block.
  - `preflight`: its comment ends `anything an earlier run left behind.`; append ` It also reads dev's aws block, which the AWS checks need.` Replace its final lines:

```go
	if len(mine) > 0 {
		return fmt.Errorf("%v exist: an earlier run left them behind (docs/e2e.md)", mine)
	}
	return nil
}
```

    with:

```go
	if len(mine) > 0 {
		return fmt.Errorf("%v exist: an earlier run left them behind (docs/e2e.md)", mine)
	}
	a, err := s.platformAWS(ctx)
	if err != nil {
		return err
	}
	s.aws = a
	return nil
}

// platformAWS returns dev's aws block from config/platform.yaml on main: the
// account and region the Component's env variables name (spec §5.3).
func (s *Scenario) platformAWS(ctx context.Context) (claims.AWS, error) {
	b, ok, err := s.F.File(ctx, "main", "config/platform.yaml")
	if err != nil {
		return claims.AWS{}, err
	}
	if !ok {
		return claims.AWS{}, errors.New("config/platform.yaml is not on main")
	}
	var p claims.Platform
	if err := yaml.Unmarshal(b, &p); err != nil {
		return claims.AWS{}, fmt.Errorf("config/platform.yaml: %w", err)
	}
	dev := p.Environments["dev"]
	if dev.AWS == nil || dev.AWS.AccountID == "" || dev.AWS.Region == "" {
		return claims.AWS{}, errors.New("config/platform.yaml gives dev no aws block with an accountId and a region: the harness checks its Component's AWS part there (docs/e2e.md)")
	}
	return *dev.AWS, nil
}
```

  - `create`: between the `s.change(…)` block and `return s.assertResources(…)`, insert:

```go
	if err := s.wantAWSRender(ctx, true); err != nil {
		return err
	}
```

  - `change`: replace its comment with `// change opens a pull request with put, waits for idp-gate, refuses a destructive plan or one that does not create the Component's AWS stack, merges, and requires the reconcile to apply both stacks without approval. The create is its only caller.`. After the `if strings.Contains(comment, warning) { … }` block, insert:

```go
	if err := s.createsTheAWSStack(comment, pr); err != nil {
		return err
	}
```

    and replace the end of `change` (the same `apply-github` check also ends `removeTopics`, which keeps it, so the edit anchors on `change`'s own `waitReconcile` line, the only `jobs, err := s.waitReconcile(ctx, sha, rejectApproval, "")`):

```go
	jobs, err := s.waitReconcile(ctx, sha, rejectApproval, "")
	if err != nil {
		return err
	}
	if jobs["reconcile / apply-github"] != "success" {
		return fmt.Errorf("apply-github: %s", jobs["reconcile / apply-github"])
	}
	return nil
}
```

    with:

```go
	jobs, err := s.waitReconcile(ctx, sha, rejectApproval, "")
	if err != nil {
		return err
	}
	return requireJobs(jobs, "reconcile / apply-aws "+s.names.AWSStack(), "reconcile / apply-github")
}

// createsTheAWSStack checks the plan comment's fingerprint, planned for the
// PR's head, has changes for the Component's AWS stack, and only creates: a
// new Component has nothing to change or destroy in AWS.
func (s *Scenario) createsTheAWSStack(comment string, pr PR) error {
	fp, err := fingerprintOf(comment, pr)
	if err != nil {
		return err
	}
	stack := s.names.AWSStack()
	if len(fp[stack]) == 0 {
		return fmt.Errorf("the plan has no change for %s: the Component's registry is not planned", stack)
	}
	for _, c := range fp[stack] {
		if c.Action != plan.Create {
			return fmt.Errorf("the plan would %s %s in %s; a new Component only creates", c.Action, c.Address, stack)
		}
	}
	return nil
}

// requireJobs checks each named job of a reconcile succeeded. A matrix leg is
// named "reconcile / apply-aws <stack>"; a skipped one keeps its unexpanded
// name, so it is missing here and fails.
func requireJobs(jobs map[string]string, names ...string) error {
	for _, n := range names {
		if jobs[n] != "success" {
			return fmt.Errorf("%s: %s", strings.TrimPrefix(n, "reconcile / "), jobs[n])
		}
	}
	return nil
}

// wantAWSRender checks whether wet holds the Component's AWS render: the
// reconcile commits it once its apply-aws leg succeeded, and removes it once
// the destroy did (spec §6.2 step 4).
func (s *Scenario) wantAWSRender(ctx context.Context, want bool) error {
	_, ok, err := s.F.File(ctx, "wet", s.names.AWSRender())
	if err != nil {
		return err
	}
	switch {
	case want && !ok:
		return fmt.Errorf("wet lacks %s after the reconcile", s.names.AWSRender())
	case !want && ok:
		return fmt.Errorf("%s is still on wet after the delete", s.names.AWSRender())
	}
	return nil
}
```

  - `deleteClaims`: replace:

```go
	if _, err := s.waitReconcile(ctx, sha, mode, "delete "+s.names.Component+" and "+s.names.Group); err != nil {
		return err
	}
```

    with:

```go
	jobs, err := s.waitReconcile(ctx, sha, mode, "delete "+s.names.Component+" and "+s.names.Group)
	if err != nil {
		return err
	}
	// A cleanup can run before the AWS stack was ever applied; the render
	// check below holds on both paths.
	if !cleanup {
		if err := requireJobs(jobs, "reconcile / apply-aws "+s.names.AWSStack()); err != nil {
			return err
		}
	}
```

    and insert before `s.logf("e2e: %s and %s are gone", …)`:

```go
	if err := s.wantAWSRender(ctx, false); err != nil {
		return err
	}
```

  - `destroysOnlyOwnNames`: replace its comment's last line `// the rendered rows are truncated) belongs to one of this run's modules.` with the two lines:

```go
// the rendered rows are truncated) belongs to one of this run's modules in the
// github stack, or to its Component's AWS stack.
```

    and replace its body after the `fingerprintOf` block with:

```go
	own := []string{"module.group_" + s.names.Group + ".", "module.component_" + s.names.Component + "."}
	for _, entry := range fp.Destructive() {
		stack, rest, _ := strings.Cut(entry, ": ") // "stack: action address"
		_, address, _ := strings.Cut(rest, " ")
		switch {
		case stack == s.names.AWSStack():
		case stack == "github" && (strings.HasPrefix(address, own[0]) || strings.HasPrefix(address, own[1])):
		default:
			return fmt.Errorf("the delete would destroy something the harness does not own: %s", entry)
		}
	}
	return nil
```

  - `assertResources`: insert before `s.logf("e2e: %s and %s match the claims", …)`:

```go
	got, err := s.F.EnvVariables(ctx, s.names.Component, "dev")
	if err != nil {
		return err
	}
	// Spec §5.3's formulas, written out here rather than computed by
	// internal/naming, so a naming regression cannot pass its own check.
	want := map[string]string{
		"AWS_ROLE_ARN":   "arn:aws:iam::" + s.aws.AccountID + ":role/" + s.names.Component + "-dev-ci",
		"ECR_REPOSITORY": s.aws.AccountID + ".dkr.ecr." + s.aws.Region + ".amazonaws.com/" + s.names.Component,
		"AWS_REGION":     s.aws.Region,
	}
	if !maps.Equal(got, want) {
		return fmt.Errorf("%s's environment dev variables = %v, want exactly %v (spec §5.3)", s.names.Component, got, want)
	}
```

- [ ] **Step 8: Run them to see them pass, then commit.**

```bash
go test ./internal/e2e/ -count=1
go test ./... -count=1
go vet ./...
gofmt -l .
git add internal/e2e/scenario.go internal/e2e/scenario_test.go internal/e2e/fake_test.go
git commit -m "feat(e2e): check the Component's AWS stack, its render on wet and its env variables"
```

  Expected: PASS, including every earlier scenario test (`TestScenarioPassesAgainstWorkingPipelines` keeps its exact step log: the reads add no step), and `TestDefaultTimeoutsFitTheJob` unchanged (D-15).

- [ ] **Step 9: Mutation check (committed code).** One at a time with Edit, each restored with `git checkout -- internal/e2e/scenario.go`, `go test ./internal/e2e/ -count=1` FAILS in the test named:
  - `case stack == s.names.AWSStack():` → `case strings.HasPrefix(stack, "aws/"):` (`TestDeleteAllowsOnlyItsOwnAWSStack`);
  - drop `stack == "github" && ` (`TestDeleteAllowsOnlyItsOwnAWSStack`, its last case);
  - `"-dev-ci"` → `"-ci"` (`TestScenarioPassesAgainstWorkingPipelines`: the literal formula is what the fake's spec values check);
  - delete the `wantAWSRender(ctx, false)` block (`TestDeleteDestroysTheAWSStack`);
  - delete the `c.Action != plan.Create` loop (`TestCreateRequiresTheAWSStackPlannedAndApplied`, "an update");
  - delete the `platformAWS` call (every test: `s.aws` is empty, so the variables differ).

  `git status --short` prints nothing after.

**Risk:** item 5 (the harness gates every release); its new checks run live only in Task 9.

### Task 8: docs for the harness's AWS checks

**Files:**
- Modify: `docs/e2e.md`, `docs/runbooks/dependencies.md`

**Interfaces:** none. Passive documentation.

- [ ] **Step 1: `docs/e2e.md`.**
  - "What a run does", the paragraph before step 1: replace `or while a "Wet push failed" issue is open.` with `while a "Wet push failed" issue is open, or while `config/platform.yaml` of `idp-claims-e2e` gives `dev` no `aws` block.`
  - Step 2: after `and the environment.` insert:

```markdown
 The Component has `aws.registry: true`, so the harness also requires the PR's plan to create the Component's AWS stack `aws/dev/components/e2e-r<run>-app` (creates only), the reconcile's `apply-aws` leg for it to succeed, its render `rendered/aws/dev/components/e2e-r<run>-app/main.tf.json` to be on `wet`, and `dev` to hold exactly `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION`, with spec §5.3's values for the account and region of `dev`'s `aws` block. It reads the variables with the reader App (see Credentials). The AWS resources live in the floci of each job and vanish with it (ADR-0030).
```

  - Step 3: append ` The update leaves the AWS stack alone, and the harness checks the variables again.`
  - Step 6: replace `and checks the repository and team are gone.` with `and checks the repository and team are gone, the `apply-aws` leg destroyed the AWS stack, and its render left `wet`.`
  - The trust list: replace `and a delete's fingerprint may destroy only `e2e-r<run>-*` resources.` with `and a delete's fingerprint may destroy only this run's `e2e-r<run>-*` modules in the `github` stack and the Component's own AWS stack, never the `dev` baseline.`
  - Credentials: replace `Both live only in` with `All three live only in`, and add this row after the bot's:

```markdown
| `IDP_READER_PRIVATE_KEY` (secret), `IDP_READER_CLIENT_ID` (variable) | A copy of the reader App's credentials ([ADR-0009](adr/0009-two-apps-and-split-environments.md)), the ones the claims repos hold | Installation tokens the harness mints itself, each for the repository the run created (`e2e-r<run>-app`) only: environments read. The first is minted at the first read, after the create step |
```

  - The paragraph after the table: replace `so the harness mints the bot's tokens from the App key rather than receiving one. The key is passed only to` with `so the harness mints the bot's and the reader's tokens from their App keys rather than receiving them. The keys are passed only to`.
  - Setting up: add before the approver token's two lines:

```bash
# The reader App credentials: the same App the claims repos use. The key comes
# from the password manager (the bootstrap deletes the local .pem files); paste it.
gh secret set IDP_READER_PRIVATE_KEY --env idp-e2e --repo jellalshadows-idp/idp-engine
gh variable set IDP_READER_CLIENT_ID --env idp-e2e --repo jellalshadows-idp/idp-engine \
  --body "$(gh variable get IDP_READER_CLIENT_ID --repo jellalshadows-idp/idp-claims-e2e)"
```

    and after the block, the sentence: `` `idp-claims-e2e`'s `config/platform.yaml` gives `dev` an `aws` block (`accountId: "000000000004"`, `region: eu-west-1`); the harness refuses to start without it. ``
  - When a run fails: change the bot entry's start to `` `mint a token for <owner>/idp-claims-e2e: …` ``, and add after it:

```markdown
- `mint a token for <owner>/e2e-r<run>-app: …` or `read the variables of e2e-r<run>-app's environment dev: …`: the harness could not read the env variables with the reader App: a wrong `IDP_READER_CLIENT_ID`, a key in `IDP_READER_PRIVATE_KEY` that is not the reader's, the reader App not installed on all repositories, or a permission it lacks. The first reader token is minted after the create step, so the harness cleans up through the platform as above. Fix the credential ([Setting up](#setting-up)) and dispatch a new run.
- `config/platform.yaml gives dev no aws block`: someone removed `dev`'s `aws` block from `idp-claims-e2e`. Nothing was created. Put it back through a PR.
- `e2e-r<run>-app's environment dev variables = …`: the variables of the run's repository differ from spec §5.3. Compare the two maps in the message; the cause is in `modules/github/component` or `internal/naming`.
- `the plan has no change for aws/dev/components/…`, `the plan would …` or `apply-aws aws/dev/components/…:`: the AWS stack was not planned, planned something besides creates, or its leg did not succeed. Read the PR's `plan-aws` and the reconcile's `apply-aws` logs. An empty conclusion after the colon means the leg was skipped.
- `wet lacks rendered/aws/…` or `… is still on wet after the delete`: the reconcile did not commit or remove the AWS render; read its `prepare the wet commit` step. A render still on `wet` means the next drift run checks a stack whose claim is gone; delete it through the platform, never by hand.
```

- [ ] **Step 2: `docs/runbooks/dependencies.md`.** Replace the row `| floci | the `floci` service image of `ci.yaml`, tag and digest | one |` with:

```markdown
| floci | the `floci` service image of `ci.yaml`, `pr.yaml` and `reconcile.yaml` (four jobs), tag and digest | one: the three files move together, and `TestFlociJobsHoldNoSecret` fails otherwise |
```

- [ ] **Step 3: Readback and commit.**

```bash
rg -n 'aws block|reader App|IDP_READER' docs/e2e.md
rg -n '^\| floci' docs/runbooks/dependencies.md
git add docs/e2e.md docs/runbooks/dependencies.md
git commit -m "docs(e2e): describe the AWS checks and the reader credentials"
```

  Read each changed paragraph back with `bat -r`; the controller runs the naming check over both files.

**Risk:** none (documentation).

### Task 9: Ship PR 2, then the live AWS run (controller)

- [ ] **Step 1: The whole-branch checks.** As Task 4 Step 1. Expected: the stat shows 19 files and about 580 changed lines; the naming check prints nothing.
- [ ] **Step 2: Review.** One whole-branch review against Review Focus 1 to 3. The reviewer gets this whole plan, the diff from `main`, and the instruction to run the Task 7 mutation list itself. Fix blockers in one batch, recheck only those.
- [ ] **Step 3: The owner steps are done (read-only).**

```bash
gh api -H "Accept: application/vnd.github.raw" repos/jellalshadows-idp/idp-claims-e2e/contents/config/platform.yaml | rg -n -A3 '^  dev:'
gh secret list --env idp-e2e --repo jellalshadows-idp/idp-engine --json name --jq '.[].name'
gh variable list --env idp-e2e --repo jellalshadows-idp/idp-engine --json name --jq '.[].name'
gh run list --repo jellalshadows-idp/idp-engine --workflow e2e.yaml --status in_progress --json databaseId --jq length
```

  Expected: `dev` with `accountId: "000000000004"` and `region: eu-west-1`; the secrets list `IDP_BOT_PRIVATE_KEY`, `IDP_E2E_APPROVER_TOKEN` and `IDP_READER_PRIVATE_KEY`; the variables `IDP_BOT_CLIENT_ID` and `IDP_READER_CLIENT_ID`; `0` runs in progress. Anything missing stops here: the merge would turn every harness run red (D-9).
- [ ] **Step 4: Push, PR and CI.** As Task 4 Steps 3 and 4, with the title `feat(e2e): check a Component's AWS stack and env variables end to end (Phase 2f)` and `Risk: items 2 and 5` in the body.
- [ ] **Step 5: Merge and the live run.** `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge`. `release.yaml` dispatches the harness for #28; watch it with `gh run watch <id> --repo jellalshadows-idp/idp-engine --exit-status`. Expected: success in about 62 minutes (D-15), `idp-e2e` green on #28's head. Then read `idp-claims-e2e`'s runs of that window:

```bash
gh run list --repo jellalshadows-idp/idp-claims-e2e --limit 20 --json databaseId,workflowName,event,headBranch,conclusion,createdAt
gh run view <id> --repo jellalshadows-idp/idp-claims-e2e --json jobs --jq '.jobs[] | "\(.name) \(.conclusion) \(.startedAt) \(.completedAt)"'
```

  Record per the watch list: the pin PR's `plan-aws aws/dev/_baseline` (no change); the create PR's `plan-aws aws/dev/components/e2e-r<run>-app` and its reconcile's `apply-aws` leg; the update's and the case-only edit's skipped AWS jobs; the delete PR's orphan plan and its reconcile's destroy; the total time; and `Installing hashicorp/aws` per Component leg (`gh run view --log --job <job-id> --repo jellalshadows-idp/idp-claims-e2e | rg -c 'Installing hashicorp/aws'`). Then check nothing is left: `gh api "orgs/jellalshadows-idp/repos?per_page=100" --paginate --jq '.[].name' | rg '^e2e-r'` and `gh api "repos/jellalshadows-idp/idp-claims-e2e/git/trees/wet?recursive=1" --jq '[.tree[].path | select(startswith("rendered/aws/dev/components"))] | length'` print nothing and `0`.
- [ ] **Step 6: If the run fails.** Read the `run the harness` step's log; the harness has already cleaned up through the platform or named the leftovers (`docs/e2e.md`, "When a run fails").
  - `read the variables of e2e-r<run>-app's environment dev: … 403 …`: open question 1. With the owner's yes, a fix PR `fix/phase-2f-reader-permission` from `main`: RED first (`TestReaderPermissions` wants `{"environments": "read", "actions_variables": "read"}`), then `readerPermissions`, the A16 row and ADR-0027's reader bullet ("environments and actions variables read"), `docs/e2e.md`'s row; CI, merge, and one new harness dispatch. Without a yes, stop: **Needs your decision**.
  - Anything else: reproduce it in a scenario test first (the fake can model any job conclusion, file and variable), fix on a branch from `main`, CI, merge, one new harness dispatch. One correction round; a second failure of the same cause becomes one **Needs your decision** result with the run IDs.
  - Never change `idp-claims-e2e` directly: leftovers are deleted through a PR there by the **OWNER** (owner-only 6).
- [ ] **Step 7: Ledger.** Record the PR, the merge commit, the run IDs, every watch-list value, and #28's `idp-e2e` state: `gh api "repos/jellalshadows-idp/idp-engine/commits/$(gh pr view 28 --repo jellalshadows-idp/idp-engine --json headRefOid --jq .headRefOid)/status" --jq '.statuses[] | select(.context == "idp-e2e") | .state'` prints `success`.

**Risk:** item 5 (the harness gates the release); the run touches only `e2e-r<run>-*` names and floci.

### Task 10: Release `0.2.0`, the re-pins and `idp-claims`'s first AWS reconcile (controller and OWNER)

- [ ] **Step 1: #28 is mergeable by the runbook's rule (read-only).**

```bash
gh pr view 28 --repo jellalshadows-idp/idp-engine --json title,headRefOid,mergeStateStatus,statusCheckRollup \
  --jq '"\(.title) \(.mergeStateStatus)", (.statusCheckRollup[] | "\(.name // .context) \(.conclusion // .state)")'
gh run list --repo jellalshadows-idp/idp-engine --workflow release.yaml --limit 3 --json status,conclusion,headSha
gh pr view 28 --repo jellalshadows-idp/idp-engine --json body --jq .body | rg -n '^#|0\.2\.0'
```

  Expected: `chore(main): release 0.2.0 CLEAN`; every check `SUCCESS` and `idp-e2e` `success` on the current head; no `release.yaml` run queued or in progress; the changelog lists the Phase 2a to 2f `feat` and `fix` entries under `0.2.0`. Anything else: wait for the running pipeline, or dispatch the harness for the current head (`docs/e2e.md`, "On release pull requests").
- [ ] **Step 2: OWNER merges #28.** The controller shows the owner Step 1's output and asks for the go. The owner runs `gh pr merge 28 --repo jellalshadows-idp/idp-engine --merge --delete-branch` (releases runbook step 3).
- [ ] **Step 3: The release.** Watch the merge's `release.yaml` run: `gh run watch <id> --repo jellalshadows-idp/idp-engine --exit-status`. Expected: `release-please` and `publish` succeed. Then verify, as the releases runbook does, in a new empty directory:

```bash
REL="$(mktemp -d)"
gh release view v0.2.0 --repo jellalshadows-idp/idp-engine --json tagName,assets --jq '.tagName, ([.assets[].name] | length)'
gh release download v0.2.0 --repo jellalshadows-idp/idp-engine --dir "$REL" --pattern 'idp_0.2.0_linux_amd64.tar.gz' --pattern 'idp_0.2.0_windows_amd64.zip' --pattern checksums.txt
(cd "$REL" && sha256sum --check --ignore-missing checksums.txt)
for a in idp_0.2.0_linux_amd64.tar.gz idp_0.2.0_windows_amd64.zip; do
  gh attestation verify "$REL/$a" --repo jellalshadows-idp/idp-engine \
    --signer-workflow jellalshadows-idp/idp-engine/.github/workflows/release.yaml \
    --source-ref refs/heads/main \
    --source-digest "$(gh api repos/jellalshadows-idp/idp-engine/git/ref/tags/v0.2.0 --jq .object.sha)"
done
rm -rf "$REL"
```

  Expected: `v0.2.0` and `7` assets; both checksums `OK`; both attestations verified. No downloaded binary is run locally: `publish` checked that the linux/amd64 binary prints the tag, and Step 4 installs it in a real job. A failure follows the releases runbook ("The tag and the release exist, but assets or the attestation are missing").
- [ ] **Step 4: The harness on the release commit.** Before any other commit lands on `main`: `gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main`, then watch it. Expected: success, and the `idp-claims-e2e` pipelines' `setup-idp` steps log `setup-idp: installed idp v0.2.0, verified against <the release commit>` (the callers pin the release commit, so `setup-idp` downloads the release instead of building). `gh run view --log --job <job-id> --repo jellalshadows-idp/idp-claims-e2e | rg 'setup-idp: installed idp v0.2.0'` prints the line.
- [ ] **Step 5: The template re-pin (OWNER merges).** Renovate's next run (Monday 2026-11-23, 04:17 UTC, or a dispatch the **OWNER** confirms: `gh workflow run renovate.yaml --repo jellalshadows-idp/idp-engine --ref main`) opens the "idp engine" PR in `idp-engine`, re-pinning the four templates in `examples/claims-repo/.github/workflows/` to `v0.2.0`'s commit with `# v0.2.0`. The controller checks the diff is exactly those four `uses:` lines (`gh pr diff <number> --repo jellalshadows-idp/idp-engine`) and that CI passed; the **OWNER** merges it. If Renovate opens none by Tuesday, the fallback is a controller PR with the same four lines (each pin from `gh api repos/jellalshadows-idp/idp-engine/git/ref/tags/v0.2.0 --jq .object.sha`), merged by the owner.
- [ ] **Step 6: `idp-claims` moves to `v0.2.0` (OWNER).**
  - Pre-checks, read-only: `gh issue list --repo jellalshadows-idp/idp-claims --label idp-wet-push --state open` prints nothing, and `gh run list --repo jellalshadows-idp/idp-claims --limit 5 --json status --jq '[.[] | select(.status != "completed")] | length'` prints `0` (runbook step 6: recover before moving to a new engine).
  - The same Renovate run opens the "idp engine" PR in `idp-claims`, moving its four callers from `150b7d35… # v0.1.1` to the `v0.2.0` commit. The controller reads its plan comment. Expected: `aws/dev/_baseline`, `aws/staging/_baseline` and `aws/pro/_baseline` each `+1 ~0 -0 ±0`, the `github` stack no change, no destructive warning, and its three `plan-aws` legs green (D-11).
  - The **OWNER** merges it. Its reconcile runs three `plan-aws` and three `apply-aws` legs against floci, without approval (creates the PR showed). If it waits for `idp-approval`, the **OWNER** decides (owner-only 5); the controller approves nothing.
  - Checks after it: the run succeeded, its six AWS legs succeeded, and

```bash
gh api "repos/jellalshadows-idp/idp-claims/git/trees/wet?recursive=1" --jq '[.tree[].path | select(startswith("rendered/aws"))]'
```

    lists `rendered/aws/dev/_baseline/main.tf.json`, `rendered/aws/pro/_baseline/main.tf.json` and `rendered/aws/staging/_baseline/main.tf.json`.
  - The next scheduled drift run (05:23 UTC) succeeds and opens no drift issue: `gh run list --repo jellalshadows-idp/idp-claims --workflow drift.yaml --limit 1 --json conclusion` and `gh issue list --repo jellalshadows-idp/idp-claims --state open`. A dispatch instead of the schedule is the **OWNER**'s (owner-only 6).
  - The fallback, if Renovate opens no PR: the **OWNER** opens one with the same four lines.
- [ ] **Step 7: Ledger.** The release run, the tag's commit, the verification output, both re-pin PRs and their merge commits, the reconcile and drift run IDs, the leg durations.

**Risk:** item 5 (the release, and the claims repos' pipelines). Every merge in this task is the owner's.

### Task 11: The phase log, closing Phase 2 (controller, docs straight to `main`)

**Files:**
- Create: `docs/phases/phase-2f.md`
- Modify: `README.md`, `docs/pipelines.md`

- [ ] **Step 1: Write `docs/phases/phase-2f.md`** with the Write tool, in the structure of `docs/phases/phase-2e.md`:
  - Header: status **complete**, the two PRs and their merge commits, any fix PR, the release `v0.2.0` and its commit.
  - "What 2f delivers": the harness's AWS and env-variable checks with the reader App (A16, ADR-0027), the three carries, the release and the re-pins.
  - Rulings D-1 to D-18 as recorded in the ledger (with any change the execution made, dated).
  - The watch list with each live result from Tasks 4, 9 and 10, and the 2e pending items closed: a real AWS leg (Task 4 Step 6, Task 9), the provider downloads per leg (counts), the single-artifact layout seen live (no `no regular file` warning), and the `pattern`-matches-nothing ruling (D-5).
  - Reviews and their findings, with "Final disposition of every finding" (Fixed, Carried, Won't fix, Accepted), as the 2e log does.
  - The disposition of every carried item, from the Self-review's map below.
  - "Carried into later phases": Phase 3 (Workspaces, `observe` replays), real AWS beyond v1 (a persistent, encrypted backend, OIDC credentials, ordering between stacks, AWS drift), Phase 4 (as the Phase 2c log lists), and the plugin cache if D-5's measure called for it.
  - "Phase 2 exit criterion" (spec §10: "E2E checks the env variables, and floci integration is green"): the harness run ID of Task 9 and of Task 10 Step 4, CI's green `floci` job on both PRs, and the `v0.2.0` release.
  - Owner notes: the reader key now has a copy in `idp-e2e` (rotate both when the reader key is rotated); release-please may open a release PR for the `docs` commits after `v0.2.0` (leave it open); the Renovate PRs of D-13 wait.
  - Process notes: execution, TDD exceptions (Tasks 5 and 8 are documentation; Task 9's checks are live), mutation counts per task, the tools (D-18), the naming check.
  - Status: **Phase 2 is complete.** Next: Phase 3.
- [ ] **Step 2: README and `docs/pipelines.md`.**
  - `README.md` line 5: replace the whole status line with:

```markdown
> **Status:** Phase 2 is complete and released as `v0.2.0`: the pipelines plan and apply the AWS stacks against the floci emulator, each stack in a job of its own that holds no secret ([ADR-0031](docs/adr/0031-floci-jobs-hold-no-secret.md)), and the end-to-end harness checks a Component's AWS stack and environment variables through `idp-claims-e2e` ([docs/e2e.md](docs/e2e.md)). Nothing reaches a real AWS account. See the [phase 2f log](docs/phases/phase-2f.md). Phase 3 (Workspaces) is next.
```

  - `README.md` line 36: delete the sentence ` A claims repo with an `aws` block stays on `v0.1.1` until `v0.2.0`.`
  - `docs/pipelines.md` line 102: delete the sentence ` A claims repo with an `aws` block stays on an engine release from before Phase 2d (`idp-claims` pins the `v0.1.1` tag) until `v0.2.0`.`
- [ ] **Step 3: Readback, naming check and commit.**

```bash
rg -n 'v0\.1\.1' README.md docs/pipelines.md
git add docs/phases/phase-2f.md README.md docs/pipelines.md
git commit -m "docs: add the phase 2f log and close Phase 2"
git push origin main
```

  Expected: `rg` prints nothing; the naming check over the three files prints nothing. Then delete the local, git-ignored `.superpowers/sdd/` (side effect 5) and check `git status --short` prints nothing.

**Risk:** none (documentation).

## Self-review

**The skill this plan follows.** The brief named `superpowers:writing-plans` at `C:\Users\Usuario\.claude\plugins\cache\claude-plugins-official\superpowers\6.4.1\skills\writing-plans\SKILL.md`. It is not installed: that path does not exist, and `fd` over `~/.claude` finds no `writing-plans` skill. The plan follows the 2e plan's structure instead: header, Global Constraints, Review Focus, File Structure, tasks with exact code and commands, Self-review, and no "Execution Handoff" section (the brief asked to skip it).

**Spec coverage (§10, Phase 2 → `0.2.0`, 2f = "the E2E env-variable checks, the `0.2.0` release and the phase log").**
- The E2E env-variable checks: Tasks 6 and 7 (code), Task 5 (A16, ADR-0027), Task 8 (docs), Task 9 (live).
- "floci integration is green": CI's `floci` job on both PRs (Tasks 4 and 9), and the live AWS legs (Task 4 Step 6, Task 9, Task 10 Step 6).
- The `0.2.0` release: Task 10. The phase log: Task 11.

**Every item carried into 2f, and where it lands.**

| Carried item | From | Lands in |
|---|---|---|
| The harness's token to read environment variables | 2b, 2c, 2d logs | D-2, Tasks 5 and 6 |
| The `aws` block for `dev` in `idp-claims-e2e` | 2b, 2c, 2d logs | D-3, Task 4 Step 6 (**OWNER**) |
| An `aws.registry` harness claim whose variables the E2E checks | 2b, 2c, 2d logs | D-3, Task 7 |
| `idp-claims` re-pins to `v0.2.0`; its three baselines plan and apply | 2d D-10, 2e log | D-11, Task 10 Step 6 (**OWNER**) |
| The `0.2.0` release | 2b to 2e logs | D-10, Task 10 (**OWNER** merges #28) |
| The Phase 2 log | 2c, 2d logs | D-16, Task 11 |
| A real AWS leg in a live pipeline | 2e watch list | D-5, Task 4 Step 6, Task 9 Step 5 |
| The provider downloads per leg | 2e watch list | D-5, Task 4 Step 6, Task 9 Step 5 |
| The single-artifact download layout, live | 2e watch list | D-5, D-8 (the warning), Task 4 Step 6, Task 9 Step 5 |
| A `pattern` download that matches nothing | 2e watch list | D-5: ruled out of the live run (unsafe to arrange), verified in the action's source and pinned by `TestWetRootWithNothingToCommit` |
| A tofu environment allowlist (R1-001) | 2e PR 2 | D-6, Task 1 |
| `recover` checks the artifact's name and run through the API (R1) | 2e PR 3 | D-7, Task 3 |
| The layout step logs what it skips and fails on a `jq` error (R4) | 2e PR 3 | D-8, Task 2 (generalized to every run script) |
| The Renovate observations of Monday 2026-11-23 | 2e log | D-13, the watch list |
| Recover before moving a claims repo to a new engine | 2e owner notes | Task 10 Step 6's pre-check |
| floci 2.5.0 moves three workflows together | brief | D-13, the watch list, Task 8 (the stale runbook row) |

Moved out of 2f, with a reason: the plugin cache (D-5: built only if the measured `init` time calls for it, then carried to Phase 3); the Renovate PRs other than the engine re-pins (D-13: a provider bump would ride into `0.2.0` untested by the harness); Workspaces and `observe` (Phase 3, spec §10).

**The owner's decisions, kept.** The reader App with its env variables, never a new App or a PAT (D-2); the `aws` block and `aws.registry: true` (D-3); the three carries (D-6 to D-8); #28 merged only on the owner's go after green `idp-e2e`, CI and up to date (D-10, Task 10 Steps 1 and 2); the re-pin and the three baselines (D-11); PRs cut from `main` after each merge, merge commits, no release before Task 10 (D-12); Renovate and floci as watch items (D-13); the 2e lessons (D-14). Every owner-only step is marked **OWNER**.

**Placeholder scan.** Angle-bracket values in commands are run-time values the step itself prints first (`<number>` from `gh pr create`, `<id>` from `gh run list`, `<job-id>` from `gh run view --json jobs`), never content to invent. The phase log's prose (Task 11) depends on live results, so its sections and sources are fixed and its text is written then.

**Type and name consistency.** `Forge.EnvVariables(ctx, repo, env)` is added to the interface, `GitHub` and the fake in the same task (Task 6). `requireJobs` keeps `change`'s `apply-github: <conclusion>` text. `destroysOnlyOwnNames` keeps its error format, so `TestDeleteRefusesForeignDestroys` holds. `ComponentClaim`'s two expectations change together with it (Task 7a). `TestScenarioPassesAgainstWorkingPipelines` keeps its step log: no new forge call logs. `TestDefaultTimeoutsFitTheJob` is unchanged (D-15).

**Consistency with the code read on 2026-11-21.** `awsstack.Env`'s two callers, `change`'s single caller (`create`), the identical `apply-github` check in `removeTopics` (left alone; Task 7's edit anchors on `change`'s own line), `NewNames`'s message, `ghapp.ParseKey`'s key-free errors, `plan.Fingerprint.Destructive`'s `stack: action address` format, the three `<(` in run scripts, and the live state of the Pre-flight list.

**Risks I could not close while planning.** Whether `environments: read` alone reads environment variables (open question 1; Task 9 Step 6 has the fix path). Whether the AWS legs' time stays inside the harness's bounds (D-15's forecast; recorded live). Neither changes a task's code before the live run.

