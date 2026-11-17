# Phase 2d — Rendering the AWS Stacks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `idp render` writes the AWS stacks of spec §5.4.
- `aws/<env>/_baseline` for every environment with an `aws` block.
- `aws/<env>/components/<name>` for every Component with `aws.registry: true`, in each environment it lists.

Each stack is thin: the pinned versions, a local backend, the environment's region and one call of the engine's module, with the identities `internal/naming` computes and the engine's AWS lock file. Its state stays in the job (ADR-0030, new). Around the render:
- the level-3 floci test applies the real render instead of hand-written stacks;
- CI validates every rendered stack;
- `idp diff` is pinned on the nested AWS paths and their orphans;
- Renovate moves the new `render.AWSProviderVersion` together with the module pins and the golden files.

The pipelines are unchanged: they still fail when an AWS stack is affected, until Phase 2e. No release.

**Architecture:**
- **`render.Render`** adds the AWS stacks, built by `awsStacks(m, opts)`, keyed by stack directory. `awsStack` builds one document, and every stack goes through `marshal` like the github stack. Each AWS stack also gets `lockfiles.AWS` as `.terraform.lock.hcl`.
- **`moduleSource(opts, stackDir, module)`** serves every stack. The git form is `//modules/<provider>/<name>?ref=<ModuleRef>`. `ModulesDir` keeps its contract (relative to the github stack), and a deeper stack resolves it from its own directory. The github stack's bytes do not change.
- **Golden tests** cover every rendered stack of a fixture and the set of stacks; four golden AWS stacks join `aws-registry`. A test outside the golden files pins spec §5.5's identities.
- **`internal/floci`** renders the `aws-registry` fixture with local module sources, adds only `floci_override.tf.json` to each AWS stack, and keeps every Phase 2c assertion.
- **`internal/wetdiff`** is unchanged in behavior. New tests pin nested stacks, the root, symbolic links, real renders with orphans and `idp diff`'s output.
- **CI** validates every rendered stack in `render-smoke`. Renovate's new `aws` regex manager moves the renderer's pin with the modules.

**Tech Stack:** Go 1.26 (standard library; no new dependency). No new pin: `hashicorp/aws` 6.71.0 is Phase 2c's (D-5). Local OpenTofu 1.12.6; CI's 1.13.2 is authoritative (D-14).

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §3.3, §4.4, §4.7, §5.1–§5.6, §6 (intro and §6.1), §7.2, §7.5, §8.2–§8.4, §10, §11 and amendments A10–A12. This plan adds A13.

Context:
- the phase logs: `docs/phases/phase-2c.md` and `docs/phases/phase-2b.md` ("Carried into later sub-phases"), and `docs/phases/phase-1b.md` (DEFER-1b, and the Phase 4 item M2);
- ADR-0002, ADR-0004, ADR-0006, ADR-0013 and ADR-0020;
- `docs/runbooks/dependencies.md`.

## Phase 2 split (owner decision, 2026-11-03)

Phase 2 is delivered as six plans, each written after the previous one ships (Phase 2a ruling D-1, spec §10 under A10).

| Plan | Delivers |
|---|---|
| 2a (shipped, PR #29, `fca5c0b`) | The AWS model of the Platform and the Component, `internal/naming`, the schema and semantic validation, spec amendment A10 |
| 2b (shipped, PR #35, `c6aa267`) | The deterministic env variables in `modules/github/component`, rendered from `internal/naming`; spec amendment A11 |
| 2c (shipped, PR #37, `368e5a3`) | `modules/aws/{baseline,component}`, the AWS lock file and the floci CI job; one account per environment; spec amendment A12 |
| **2d (this plan)** | Rendering the AWS stacks (`aws/<env>/_baseline`, `aws/<env>/components/<name>`); `render.AWSProviderVersion` with its Renovate manager; level 3 on the real render; CI validation of every stack; nested stack discovery tests; the 2c review items; spec amendment A13, ADR-0030 |
| 2e | The workflows' AWS plan and apply against floci |
| 2f | The E2E env-variable checks, the `0.2.0` release and the Phase 2 log |

## Decisions taken while planning

The executor copies these into the ledger as rulings before Task 1. Phase 2a's owner decisions still hold: Phase 2 is split (2a D-1), and v1 is floci only, with no real AWS account (2a D-2).
- D-1 to D-13 record the controller's twelve rulings and what was decided under them.
- D-14 and D-15 were decided while planning, from facts read live on 2026-11-17.

- **D-1. Layout (ruling 1, spec §5.4).**
  - Every environment with an `aws` block gets `aws/<env>/_baseline/main.tf.json`, whether or not a registry Component uses it (2c's D-1: each account gets its OIDC provider).
  - Every Component with `aws.registry: true` gets `aws/<env>/components/<name>/main.tf.json` in each environment it lists, and nowhere else.
  - Each AWS stack gets `.terraform.lock.hcl` = `lockfiles.AWS`. No `workspaces/` (Phase 3).
  - A registry Component listing an environment without an `aws` block keeps failing with the existing error, `render: component "api" has aws.registry, but environment "pro" has no aws block`. `githubStack` reports it first, and `awsStacks` reports the same error on its own.
  - The `basic` fixture (no `aws` block) renders exactly `github/main.tf.json` and `github/.terraform.lock.hcl`, byte for byte as before. `TestRenderGolden` now asserts the file list too.

  *Cost if wrong:* rendering a baseline only for environments that a registry Component uses is a filter in `awsStacks` and two fewer golden files. No state moves (D-3).
- **D-2. Thin stacks (ruling 2, spec §5.3, §5.6).** A stack's document is:
  - `terraform`: `required_version` = `TofuVersion`, `required_providers.aws` = `{source: hashicorp/aws, version: AWSProviderVersion}`, and `backend.local.path` (D-3);
  - `provider.aws`: only `region`, the environment's;
  - one module call: `module.baseline` with only `source`, or `module.component` with `source` and five inputs.

  More about the module calls:
  - The call names are 2c's `writeStack` names, so the resource addresses (`module.component.aws_iam_role.ci`) are the ones level 3 has applied since 2c.
  - The inputs are exactly 2c's `componentInputs`: `github_repository` = `<org>/<name>`, `environment`, and from `internal/naming` `repository_name` = `RepositoryName`, `role_name` = `RoleName` and `oidc_provider_arn` = `OIDCProviderARN(accountId)`.
  - Every string goes through `literal()`, like `envVariables`. The schemas forbid `${` and `%{` in these values, so no byte changes.

  Nothing about floci appears: no endpoint, no `skip_*` flag, no credentials, no `assume_role`.

  *Cost if wrong:* renaming a module call changes addresses in state that lives only in a job (D-3). That is a render and golden change, with no migration.
- **D-3. The AWS stacks' state and encryption (ruling 3; new ADR-0030).**
  - Each AWS stack declares the local backend at `terraform.tfstate`, in its own directory, in the job's working copy.
  - It emits **no `encryption` block**.
  - Nothing AWS is persisted in `wet` in v1 except the rendered stacks.

  Why:
  - spec §5.4 lists only `tfstate/github.tfstate`;
  - §5.5 says the AWS state lives in an ephemeral emulator and cannot be read later;
  - ADR-0004 rebuilds the AWS side in every job, by applying the stacks again (§6.1 step 4);
  - an `encryption` block would force `TF_ENCRYPTION`, and so the passphrase, into every floci job, and those jobs hold the runner's Docker socket.

  Checked:
  - `wet-push` (`internal/wetpush`) syncs only the paths the workflow names, today `rendered/github` and `tfstate/github.tfstate`, and rejects `.terraform`. AWS state could reach `wet` only if 2e named it.
  - `idp encryption-env` writes `TF_ENCRYPTION` to `GITHUB_ENV` for the whole job. Verified locally with OpenTofu 1.12.6 on a stack without an `encryption` block:
    - without the variable, the state is plain JSON;
    - with it, the state is encrypted (top-level keys `encrypted_data`, `encryption_version`, `lineage`, `meta`, `serial`), and the replay plan exits 0;
    - with the variable unset afterwards, the encrypted state is unreadable (`Unsupported state file format`).

    So a 2e job keeps one setting per stack, as a job naturally does.

  The explicit `backend "local"` block is chosen over OpenTofu's default (the same behavior) because it states the decision in every stack, and the golden files pin it. Spec §5.6 and §7.2 (A13) and ADR-0004 are amended.

  *Cost if wrong:* a persisted backend later changes one constant and the golden files, plus 2e's workflow paths. Nothing migrates, because no AWS state exists anywhere yet.
- **D-4. Module sources (ruling 2).** `moduleSource(opts, stackDir, module string)` replaces `moduleSource(opts, module)`.
  - The git form is `git::https://github.com/jellalshadows-idp/idp-engine.git//modules/<provider>/<name>?ref=<ModuleRef>`, so the github stack's sources are unchanged.
  - `ModulesDir` keeps its contract: relative to the github stack, so `idp render`'s `relativeModulesDir` is unchanged.
  - A deeper stack climbs to the output root and descends into `github` first, and `path.Join` cleans the result. `../../modules` gives `../../../../modules/aws/baseline` for `aws/dev/_baseline` and `../../../../../modules/aws/component` for `aws/dev/components/api`.
  - The github stack takes `ModulesDir` as given, so `./mods` stays `./mods`. `TestModuleSource` pins both forms, and a CLI test resolves every source on disk.

  *Cost if wrong:* a `ModulesDir` relative to the output root would change the CLI flag's contract and the github stack's sources; this keeps both.
- **D-5. `render.AWSProviderVersion` (ruling 4, 2c's D-7).**
  - `AWSProviderVersion = "6.71.0"`, equal to `modules/aws/*/versions.tf` and the lock file. No new pin: 6.71.0 was released 2026-10-28 (2c's D-5).
  - `TestOneOpenTofuAndProviderVersion` now walks every `modules/*/*/versions.tf` (`moduleDirs`). It compares each module's pin with the renderer's version for that module's provider, and fails for a provider the renderer does not pin.
  - `TestRenderCopiesTheLockFile` checks that `lockfiles.AWS` pins it.
  - Renovate gets a regex manager `aws` over `render.go` and `internal/render/testdata/*/expected/aws/**/main.tf.json`. Its package name is `hashicorp/aws`, so the existing "aws provider" rule groups it with the `terraform` manager's module pins, in one PR labelled `needs-lockfile`.
  - The `opentofu` manager's golden pattern widens from `expected/github/` to `expected/.+/`, so the AWS golden files' `required_version` moves too.
  - `TestRenovateCustomManagers` gains the row, and `TestRenovateCustomManagersFindThePins` lists the exact files of each pin.
  - The lock file stays a manual step (runbook).

  *Cost if wrong:* a bump that misses a file fails the `go` job instead of passing silently.
- **D-6. Golden files, determinism and identities (ruling 5).**
  - `checkGolden(t, fixture, rel, got)` takes any rendered file.
  - `checkGoldens` compares every `main.tf.json` of a render with the fixture's golden files, and the set of stacks too, so a stack that appears or disappears fails. `-update` writes the rendered ones; a golden file of a stack that is no longer rendered is deleted by hand.
  - Four new golden files: `aws/{dev,pro}/_baseline` and `aws/{dev,pro}/components/api`.
  - `TestRenderIsDeterministic` compares every file (`maps.EqualFunc`).
  - `TestRenderAWSStacksUseNamingsIdentities` pins spec §5.5's identities outside the golden files (2b's D-7 pattern): pro's module call verbatim (`role_name` `api-pro-ci`, `oidc_provider_arn` `arn:aws:iam::000000000003:oidc-provider/token.actions.githubusercontent.com`, `repository_name` `api`, `github_repository` `acme/api`). In every environment, the role and the repository the stack creates, and its region, must equal the github stack's `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION`.
  - `awsStacks` ranges over the environments in sorted order. The result is a map, so the order cannot change the bytes, but an error names the same environment on every run.

  *Cost if wrong:* test code only.
- **D-7. Level 3 on the real render (ruling 6).** `TestFloci`:
  - copies `modules/aws` to `<root>/modules/aws`;
  - `renderFixture(root)`: `claims.Load("../render/testdata/aws-registry")`, `render.Render` with `ModulesDir: "../../modules"`, and `render.WriteTree` into `<root>/rendered`;
  - for each target, applies `stacks(tg)`: the baseline, then the Component's stack. `apply` adds only `floci_override.tf.json` (`writeOverride`, 2c's override verbatim) and runs `init -lockfile=readonly` with the rendered lock file. It registers the destroy before applying, applies, and requires a replay plan that exits 0.

  Every 2c assertion stays: `wantCreated`, `wantPolicy`, account isolation and `wantNothingLeft`.
  - **Removed:** `writeStack`, `componentInputs`, `TestWriteStack`.
  - **Kept:** `target`, `targets`, `org`, `component`, `githubRepository`, `repositoryARN`, `decodePolicy`, `trustPolicy`, `pushPolicy`, the `floci` methods, `TestModulesUseNamingsIssuer`, `TestExpectedPoliciesAreTheModulesOwn`, `TestDecodePolicy`.
  - **New:**
    - the helpers `fixtureDir`, `stacks`, `renderFixture` and `writeOverride`;
    - `TestTheFixtureIsTheTestPlatform`: the written-out targets, org and Component equal the fixture;
    - `TestRenderFixture`: the render holds exactly the stacks `TestFloci` applies, each with the AWS lock file, sources that resolve to the copied modules, and no emulator words;
    - `TestWriteOverride`.

  The expectations stay written out rather than derived from the fixture, so the floci checks never take their expected values from the render they check.

  *Cost if wrong:* deriving the targets from the fixture is a few lines.
- **D-8. CI (ruling 7).**
  - `render-validate` loops over every `main.tf.json` of each render with bash `globstar` (`"$RUNNER_TEMP/rendered/$fixture"/**/main.tf.json`). Per stack it runs `init -backend=false -input=false -lockfile=readonly`, then `validate`.
  - It sets `TF_PLUGIN_CACHE_DIR: ${{ runner.temp }}/tofu-plugin-cache`, which the script creates, so the AWS provider downloads once for the four AWS stacks.
  - `-lockfile=readonly` is new for the github stack too. Verified locally: a stack carrying the wrong lock file fails `init` with `Provider dependency changes detected`.
  - `globstar` rather than `find`: shellcheck, run by actionlint, flags loops over `find` output (SC2044), and the shell rule avoids `find`.
  - `TF_ENCRYPTION` stays on the step for the github stack's enforced encryption; the AWS stacks validate with it set.
  - `TestCIRenderSmokeCoversEveryFixture` pins the variable and the lines.
  - The `floci` job's steps are unchanged; only its comment and the test step's name follow the render. `TestCIFlociJob` keeps pinning the steps and now requires exactly one `setup-go` step (D-11).

  *Cost if wrong:* a CI-only failure, caught on the PR.
- **D-9. Stack discovery (ruling 8).** `internal/wetdiff` already finds nested stacks and gives each file to the deepest stack that holds it. 2d pins that behavior and documents it in `readStacks`.

  The new tests:
  - `TestDiffNestedStackOwnsItsFiles`.
  - `TestDiffRootIsNeverAStack`.
  - `TestDiffRejectsSymlinks`. It skips where the OS refuses to create a symbolic link; it ran on the planning machine.
  - `TestDiffSeesTheRenderedAWSStacks`: two real renders, with the orphans left after an environment and a registry Component were removed.
  - In `internal/cli`, `TestDiffNamesTheNestedAWSStacks`: the exact stdout and `GITHUB_OUTPUT`.

  These close Phase 1b's M2 items (nested discovery, nested ownership, the symlink branch, the root `main.tf.json`), which DEFER-1b had sent to Phase 4.
  - A root `main.tf.json` stays ignored, not an error: a render never writes one, and the render marker is ignored the same way.
  - The output format is unchanged: `PATH: STATUS` lines with slash paths, and `affected` a JSON array of slash paths.

  **What 2e must handle (recorded, not changed):**
  - `pr.yaml` and `reconcile.yaml`'s step "check the affected stacks" (`jq -e 'all(.[]; . == "github")'`) fails any run with an AWS stack affected. That is intended until 2e.
  - `plan-summary` takes `--plan STACK=FILE` and splits at the first `=`, so slash paths pass.
  - `wet-push` pushes only the paths it is given.
  - `gate`, `comment` and `issue` read `plan-summary`'s per-stack output.

  2d changes none of those packages or workflows, and the github stack's bytes are unchanged (golden files).

  *Cost if wrong:* making a root `main.tf.json` an error is a few lines.
- **D-10. Safety of the render change (ruling 9).** Verified live on 2026-11-17 with read-only `gh api`:
  - **`idp-claims`.** `config/platform.yaml` has three `aws` blocks: `dev` `000000000001`, `staging` `000000000002`, `pro` `000000000003`, all in `eu-west-1`. Its four callers pin `@150b7d35dc03831046d732a642e5ff03e33ce8bf # v0.1.1`, the commit tag `v0.1.1` points at. So it renders with `v0.1.1` until it re-pins to `0.2.0` in 2f, after 2e.
  - **Renovate.** It covers `idp-engine` and `idp-claims` only, and its "idp engine" group moves the callers to new release tags only. No Renovate PR can move `idp-claims` to a 2d commit.
  - **`idp-claims-e2e`.** Its `config/platform.yaml` has no `aws` block (`dev: {}`), and the harness (`internal/e2e`, `cmd/idp-e2e`) never writes one. The harness runs that every push to `main` dispatches on release PR #28 therefore render no AWS stack.
  - **PR #28** (`chore(main): release 0.2.0`) is open. It must not be merged before 2f.

  *Cost if wrong:* a claims repo with an `aws` block on an engine between 2d and 2e fails every pipeline run at "check the affected stacks", and changes nothing.
- **D-11. The items carried from 2c (ruling 10).**
  - **R3-001:**
    - `awsCLI` and `wantMissing` take `testing.TB`.
    - A `TestMain` turns the test binary into a stub AWS CLI when `IDP_FLOCI_FAKE_AWS_EXIT` is set. The stub writes `IDP_FLOCI_FAKE_AWS_STDERR` to stderr and exits with that code, before any test runs.
    - `TestWantMissing` runs the three outcomes, offline in every `go test ./...`, through a `recorder` that keeps `Errorf`: missing passes, found fails, and an unreachable floci fails.
  - **R3-002:** `TestCIFlociJob` counts the `setup-go` steps and requires exactly one.
  - **The Renovate observations** are a watch-list item, not a task. Live on 2026-11-17:
    - the latest runs were on 2026-11-16, both successful: the scheduled one, which started at 12:53 UTC, hours after its 04:17 cron, and a dispatch at 23:52 UTC;
    - no Renovate PR is open;
    - `hashicorp/aws` 6.72.0 becomes eligible on 2026-11-18.

  *Cost if wrong:* test code only.
- **D-12. Docs (ruling 11).** Amendment A13 touches only text that 2d makes stale:
  - §5.6: going live now also changes the AWS stacks' backend, so "renders do not change" no longer holds;
  - §6's introduction: the render writes AWS stacks since 2d, and the workflows reconcile only `github` until 2e;
  - §7.2: the `encryption` block is the github stack's;
  - §8.4: level 3 applies the render.

  §5.4 is unchanged: its comments match the render (D-1), and ADR-0030 records that the AWS state is not in `wet`. §5.3 is still true.

  Beyond the spec:
  - ADR-0030 is new (D-3), and ADR-0004 is amended in two places;
  - `docs/cli.md` (`render`, `diff`), `docs/claims.md` (the `aws` block, `aws.registry`, "Rendering") and `docs/pipelines.md` ("Limitations") are updated;
  - the runbook gets its row and section (Task 4), and the README is updated at ship.

  *Cost if wrong:* wording.
- **D-13. No workflow change and no release (ruling 12).**
  - The reusable workflows, the caller templates and `setup-idp` are unchanged.
  - `ci.yaml` changes only `render-smoke`'s step and the `floci` job's comment and step name.
  - After the merge, release PR #28 still proposes `0.2.0` and stays open until 2f.

  *Cost if wrong:* if it proposes another version, the rule is the same: do not merge it.
- **D-14. Local checks use OpenTofu 1.12.6; CI's 1.13.2 is the source of truth.** Verified while planning, in scratch copies outside the repository:
  - **Go:** every code and doc block of this plan was applied verbatim by a script to an archive of `e521466`, in task order. `go test ./...`, `go vet ./...` and `gofmt -l .` are clean. Each task's RED and GREEN and its mutation checks were observed as written.
  - **Rendering:** the `basic` and `aws-registry` fixtures were rendered through `cli.Run` with `--modules-dir`, from a test that was never committed (no `go run`).
  - **OpenTofu, on all six rendered stacks:** `init -backend=false -input=false -lockfile=readonly` and `validate` print `Success! The configuration is valid.` for each.
  - **OpenTofu, other checks:**
    - a wrong lock file fails `init`;
    - a rendered Component stack with `floci_override.tf.json` added validates;
    - `init` with the rendered local backend succeeds;
    - the module tests pass: `aws/baseline` 2/2 and `aws/component` 8/8.
  - **A limit of the local checks:** OpenTofu 1.12.6's `validate` does not check `required_version` (a stack requiring `9.0.0` validated), so the local validation does not prove the `1.13.2` pin; CI does.

  Not verifiable locally: actionlint, zizmor and shellcheck on `ci.yaml`; the floci apply of the rendered stacks; Renovate's behavior.

  *Cost if wrong:* one CI round trip.
- **D-15. Delivery: one PR, eight work-unit commits (Tasks 1 to 8).** It is about 1,170 added and 190 removed lines over 27 files; the four golden files make up 114 of the added lines, and most of the rest is tests. 2a to 2c each shipped as one PR. The Risk lines below single out the commits that need attention.

  *Cost if wrong:* the controller splits it into two PRs: Tasks 1 to 4 (the render and Renovate), then Tasks 5 to 8.

## Global Constraints

**Names** (copy verbatim)
- Branch `feat/phase-2d-aws-render`; one PR, merged with a merge commit.
- `internal/render`:
  - code: `AWSProviderVersion`, `awsStateFile`, `awsStacks`, `awsStack`, `moduleSource(opts, stackDir, module)`; the module calls `baseline` and `component`; the stack directories `aws/<env>/_baseline` and `aws/<env>/components/<name>`;
  - tests: `TestModuleSource`, `TestRenderAWSStacksGolden`, `TestRenderAWSStacksFollowTheAWSBlocks`, `TestRenderAWSStacksUseNamingsIdentities`, `TestRenderAWSStacksAreThin`;
  - test helpers: `checkGolden(t, fixture, rel, got)`, `checkGoldens`, `renderedAWSStack`, `awsStackOf`, `stackDirs`.
- `internal/cli`: `TestRenderLocalModulesResolveFromEveryStack`, `TestDiffNamesTheNestedAWSStacks`.
- `internal/wetdiff`: `TestDiffNestedStackOwnsItsFiles`, `TestDiffRootIsNeverAStack`, `TestDiffRejectsSymlinks`, `TestDiffSeesTheRenderedAWSStacks`.
- `internal/floci`:
  - helpers: `fixtureDir`, `stacks`, `renderFixture`, `writeOverride`, `recorder`;
  - tests: `TestTheFixtureIsTheTestPlatform`, `TestRenderFixture`, `TestWriteOverride`, `TestWantMissing`, `TestMain`;
  - environment variables: `IDP_FLOCI_FAKE_AWS_EXIT`, `IDP_FLOCI_FAKE_AWS_STDERR`.
- Renovate custom manager `aws` (`depNameTemplate`); `ci.yaml` step `render-validate` with `TF_PLUGIN_CACHE_DIR`.
- Spec amendment **A13**; `docs/adr/0030-aws-stack-state-stays-in-the-job.md`; phase log `docs/phases/phase-2d.md`.

**Process**
- Strict TDD: failing test, run it to see it fail, implement, run it to see it pass, commit. Tests that pin existing behavior say so and pass before the change; a mutation check shows what they catch.
- No `go build` and no GoReleaser run locally (no `go run` either). Use `go test ./...`, `go vet ./...` and `gofmt -l .` (must print nothing); `-race` runs in CI.
- There is no local Docker: `TestFloci` against floci, actionlint and zizmor run only in CI.
- Shell rule: never use `cat`, `grep`, `find`, `sed` or `ls`, never write files with shell heredocs, never `perl`. Use the Read/Write/Edit tools, `rg`, `fd`, `eza`, `bat`, `sd`, `gh`, `git`, `python -I`.
- Conventional commits, never with Co-Authored-By or any AI attribution.
- Docs in English. Owner rule: docs describe the platform on its own terms, never naming another product as inspiration or comparison, and never use the owner's work account. The controller gives each dispatch the naming check that enforces both (it must print nothing).
- Doc-only changes after the merge go straight to `main`; code goes through PRs.
- Never write into `~/.idp/`. Running `~/.idp/tools/tofu-1.12.6/tofu.exe` only reads it (D-14).
- Scratch state lives in `mktemp -d` directories outside the worktree, removed after use.
- Every pin is at least 14 days old when adopted (spec §7.5). This plan adopts none (D-5).
- Risk lines:
  - Tasks 1, 5, 6 and 8: `Risk: none`.
  - Task 2: `Risk: item 3 (moduleSource is shared with the github stack; its golden files pin the bytes)`.
  - Task 3: `Risk: item 3 (the render's output, which the workflows consume: new stacks, the github stack unchanged; D-10)`.
  - Task 4: `Risk: item 5 (dependency automation)`.
  - Task 7: `Risk: item 5 (CI)`.

  The PR's CI run and the final whole-branch review check them.

**Module test snippet** (Git Bash, from the repository root; D-14). Set `MODULE` first, for example `MODULE=aws/component`:

```bash
TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"
"$TOFU" fmt -check -recursive "modules/$MODULE"
MOD="$(mktemp -d)"
cp -r "modules/$MODULE/." "$MOD"
cp "lockfiles/${MODULE%%/*}.terraform.lock.hcl" "$MOD/.terraform.lock.hcl"
(cd "$MOD" && "$TOFU" init -input=false -lockfile=readonly >/dev/null && "$TOFU" test -no-color)
rm -rf "$MOD"
```

**Render validation snippet** (Git Bash, from the repository root; D-14). It renders both fixtures through `idp render`'s own code from a throwaway test (no `go run`), then initializes and validates every stack, as `render-smoke` does. Write the throwaway test with the Write tool to `internal/cli/zz_scratch_render_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestScratchRender(t *testing.T) {
	out := os.Getenv("SCRATCH_OUT")
	if out == "" {
		t.Skip("set SCRATCH_OUT")
	}
	for _, f := range []string{"basic", "aws-registry"} {
		var stdout, stderr bytes.Buffer
		args := []string{"render", "--dir", filepath.Join("..", "render", "testdata", f), "--out", filepath.Join(out, "rendered", f), "--modules-dir", filepath.Join(out, "modules")}
		if code := Run(args, &stdout, &stderr, noEnv); code != 0 {
			t.Fatalf("%s: exit %d %s", f, code, stderr.String())
		}
	}
}
```

Then run:

```bash
OUT="$(mktemp -d)"
SCRATCH_OUT="$(cygpath -w "$OUT")" go test ./internal/cli/ -count=1 -run TestScratchRender
rm internal/cli/zz_scratch_render_test.go
cp -r modules "$OUT/modules"
export TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe" TF_PLUGIN_CACHE_DIR="$OUT/plugin-cache"
mkdir -p "$TF_PLUGIN_CACHE_DIR"
export TF_ENCRYPTION='key_provider "pbkdf2" "ci" {
  passphrase = "render-smoke-only-not-a-secret"
}
method "aes_gcm" "ci" {
  keys = key_provider.pbkdf2.ci
}
state {
  method = method.aes_gcm.ci
}
plan {
  method = method.aes_gcm.ci
}'
(cd "$OUT/rendered" && bash -ec 'shopt -s globstar
for stack in */**/main.tf.json; do
  stack="${stack%/main.tf.json}"
  "$TOFU" -chdir="$stack" init -backend=false -input=false -lockfile=readonly -no-color >/dev/null
  printf "%s: " "$stack"; "$TOFU" -chdir="$stack" validate -no-color
done')
rm -rf "$OUT"
```

Expected, six lines in this order:
```text
aws-registry/aws/dev/_baseline: Success! The configuration is valid.
aws-registry/aws/dev/components/api: Success! The configuration is valid.
aws-registry/aws/pro/_baseline: Success! The configuration is valid.
aws-registry/aws/pro/components/api: Success! The configuration is valid.
aws-registry/github: Success! The configuration is valid.
basic/github: Success! The configuration is valid.
```

`git status --short` then prints nothing beyond the task's own changes. The AWS provider is a large download (about 40 s cold); the plugin cache downloads it once.

**Spec rules**
- No real cloud credential anywhere (spec §7.6, 2a's D-2).
- The AWS identities come only from `internal/naming`, never from AWS state (spec §5.5, ADR-0006).
- Emulator specifics belong to CI and its tests, never to the render (spec §5.6, ADR-0004).
- Rendering stays pure (spec §5.1): the same model and options give the same bytes.

**Owner actions:** none beyond approving this plan and its side effects.

**Side effects this plan authorizes** (controller, after the owner approves the plan)
1. Push `feat/phase-2d-aws-render`, open its PR, merge it after CI and the final review.
2. Doc-only commits straight to `main`: this plan, the phase log, the README.
3. Delete the local, git-ignored SDD workspace `.superpowers/sdd/` after the phase log is committed.

Not authorized, stop and ask:
- merging any release PR, including #28, which proposes `0.2.0` (D-13);
- merging or dispatching Renovate, or merging a Renovate PR;
- changing any repository or org setting, App, ruleset, environment, variable or secret;
- any change to `idp-claims` or `idp-claims-e2e`;
- any change to the reusable workflows, the caller templates or `setup-idp` (D-13);
- creating any cloud resource.

## Review Focus

1. **A render that drifts from `internal/naming`.**
   - **Expected:** in every environment, the Component stack's `role_name`, `repository_name`, region and `oidc_provider_arn` name the role, the repository, the region and the provider that the github stack's `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION` carry. pro's module call is spec §5.5's, verbatim.
   - **Pinned by:**
     - `TestRenderAWSStacksUseNamingsIdentities` (Task 3), outside the golden files;
     - `TestFloci`'s identity checks on the applied render (CI, Task 6).

     Mutation-checked in Task 3 (`naming.RoleName(c.Name, "dev")`).
2. **An AWS stack in the wrong place: a baseline for an environment without an `aws` block, or a registry Component's stack in an environment it does not list.**
   - **Expected:**
     - a baseline for every environment with an `aws` block, used or not, and for no other environment;
     - a Component stack only in the registry Component's own environments;
     - no AWS stack for a Component without `aws.registry`.
   - **Pinned by:**
     - `TestRenderAWSStacksFollowTheAWSBlocks` (`staging` with an `aws` block and no Component, `qa` without one, `web` without `aws.registry`);
     - `TestRenderWithoutRegistryPassesNoVariables`;
     - `TestRenderAWSStacksGolden`'s stack list (Task 3).

     Mutation-checked in Task 3 (every environment for the Component).
3. **`basic`, or the github stack of any claims repo, changing bytes.**
   - **Expected:** `basic` renders exactly its two github files, byte for byte as before. The github stack's sources and documents are unchanged in both fixtures.
   - **Pinned by:**
     - in Task 3, the existing golden files with `TestRenderGolden`'s new file list, and `TestRenderWithoutRegistryPassesNoVariables`;
     - in Task 2, `TestModuleSource`'s github rows and `TestRenderLocalModules`.

     Task 2 Step 4 checks that `git status --short internal/render/testdata` prints nothing. Mutation-checked in Task 2 (the github guard).
4. **A Renovate bump that moves the modules but not `AWSProviderVersion` or the golden AWS stacks.**
   - **Expected:** one "aws provider" group over the module pins, `AWSProviderVersion` and the four golden AWS stacks. Any pair that disagrees fails the `go` job.
   - **Pinned by** (Task 4):
     - `TestRenovateCustomManagersFindThePins`, the exact files of each pin;
     - `TestRenovateCustomManagers`;
     - `TestOneOpenTofuAndProviderVersion`, every module against the renderer.

     Also `TestRenderCopiesTheLockFile` (Task 3). Mutation-checked in Task 4 (`AWSProviderVersion = "6.70.0"`).
5. **`idp diff` missing or misnaming a nested AWS stack, or not reporting an orphan `aws/<env>` after an environment is removed.**
   - **Expected:**
     - every stack at its slash path under `rendered/`;
     - a file counted in the deepest stack that holds it;
     - orphans for a removed environment's baseline and Component stacks, and for a removed Component's stack;
     - the exact `PATH: STATUS` lines and the `affected` output.
   - **Pinned by** (Task 5): `TestDiffSeesTheRenderedAWSStacks`, `TestDiffNamesTheNestedAWSStacks`, `TestDiffNestedStackOwnsItsFiles`, `TestDiffRootIsNeverAStack`, `TestDiffRejectsSymlinks`. Mutation-checked in Task 5 (three mutations).

Also tested, outside the five:
- no floci, endpoint, `skip_*` or `encryption` text in any AWS stack (`TestRenderAWSStacksAreThin`, Task 3, mutation-checked; `TestRenderFixture`, Task 6);
- non-deterministic output (`TestRenderIsDeterministic` over every file, Task 3);
- every source resolving on disk (`TestRenderLocalModulesResolveFromEveryStack`, Task 3);
- `wantMissing`'s two failure branches (`TestWantMissing`, Task 6, mutation-checked);
- exactly one `setup-go` step in the floci job (`TestCIFlociJob`, Task 7, mutation-checked);
- every rendered stack validated in CI (`TestCIRenderSmokeCoversEveryFixture`, Task 7).

## Watch list for the live steps

Record each result in the ledger:
- **The PR's check list:** `go`, `workflows` (actionlint and zizmor on the new `render-validate` script), `modules (github/group)`, `modules (github/component)`, `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke`, `floci` and `release-snapshot`.
- **`render-smoke`** validates six stacks: one for `basic`, five for `aws-registry`. The log shows `Success! The configuration is valid.` six times. The first AWS stack's `init` prints `Installing hashicorp/aws v6.71.0 to the shared cache directory...`, and the other three print `Detected previously-installed hashicorp/aws v6.71.0 in the shared cache directory` (the wording of OpenTofu 1.12.6, observed while planning). Record the step's duration against Phase 2c's.
- **The first `floci` job on the real render.** It passes:
  - its log shows `TestRenderFixture`, `TestTheFixtureIsTheTestPlatform`, `TestWriteOverride` and `TestWantMissing` passing;
  - it shows `--- PASS: TestFloci` with the subtests `dev`, `pro` and `accounts are isolated`;
  - it shows two `floci's repository URI for api:` lines.

  Record its duration against Phase 2c's.
- **After the merge** (Task 9 Step 6): release PR #28 still proposes `0.2.0` (D-13), and the harness run dispatched for it passes. The harness's `config/platform.yaml` has no `aws` block, so its render has no AWS stack (D-10).
- **The weekly Renovate run of Monday 2026-11-23, 04:17 UTC** (scheduled runs can start hours late: 2026-11-16's started at 12:53 UTC; not dispatched by this plan). Read the Dependency Dashboard of `idp-engine` and the PRs it opens:
  - `hashicorp/aws` 6.72.0 (eligible from 2026-11-18) arrives as one "aws provider" PR labelled `needs-lockfile`.
  - If 2d is on `main` by then, that PR moves `modules/aws/*/versions.tf`, `AWSProviderVersion` in `render.go` and the four golden AWS stacks together (D-5). Its `go` job fails (`TestRenderCopiesTheLockFile`, `TestModulesPinTheirLockedProvider`), and so do `modules (aws/*)`, `render-smoke` and `floci` (`init -lockfile=readonly`), until the lock file is regenerated (runbook).
  - If 2d is not on `main` yet, the PR moves only the module pins. After the merge, check that a later Renovate run rebases it to include the new manager's files.
  - The Dashboard still lists `floci/floci` under `github-actions` and `hashicorp/aws` under `terraform`. floci 2.5.0 is eligible only from 2026-11-29.

  Merge nothing (not authorized). Record what happened before the phase log is written, and carry the rest into 2e.

## File Structure

```
docs/superpowers/specs/2026-10-08-idp-on-actions-design.md (A13), docs/adr/0030-aws-stack-state-stays-in-the-job.md (new),
docs/adr/0004-floci-multi-account-emulation.md, docs/adr/README.md                                              # Task 1
internal/render/render.go, internal/render/render_test.go                                                        # Task 2
internal/render/render.go, internal/render/render_test.go, internal/cli/claims_test.go,
internal/render/testdata/aws-registry/expected/aws/{dev,pro}/{_baseline,components/api}/main.tf.json (4 new)     # Task 3
.github/renovate-global.json, internal/workflowspec/renovate_test.go, docs/runbooks/dependencies.md               # Task 4
internal/wetdiff/wetdiff.go, internal/wetdiff/wetdiff_test.go, internal/cli/diff_test.go                          # Task 5
internal/floci/{doc.go,fixture_test.go,floci_test.go,stack_test.go}, internal/floci/wantmissing_test.go (new)     # Task 6
.github/workflows/ci.yaml, internal/workflowspec/ci_test.go                                                      # Task 7
docs/cli.md, docs/claims.md, docs/pipelines.md                                                                   # Task 8
docs/phases/phase-2d.md (new), README.md                                                                         # Task 9
```

Tasks 1 to 8 touch 27 files.

## Pre-flight (controller, before Task 1)

- [ ] `gh api user --jq .login` prints `jellalshadows` (else `gh auth switch -u jellalshadows`).
- [ ] `git switch main && git pull --ff-only`, then `git log -1 --format=%h` prints `e521466` or a later commit, and `git status --short` prints nothing.
- [ ] `go version` prints `go1.26` or a later 1.26 patch, and `go test ./...` passes on `main`, so any later failure is this plan's.
- [ ] `"$HOME/.idp/tools/tofu-1.12.6/tofu.exe" version` prints `OpenTofu v1.12.6`, and the module test snippet ends with `Success! 8 passed, 0 failed.` for `MODULE=aws/component` and `Success! 2 passed, 0 failed.` for `MODULE=aws/baseline`.
- [ ] `gh pr view 28 --repo jellalshadows-idp/idp-engine --json state,title --jq '"\(.state) \(.title)"'` prints `OPEN chore(main): release 0.2.0`.
- [ ] The production pin and the harness config (D-10), read-only:
  - `gh api -H "Accept: application/vnd.github.raw" repos/jellalshadows-idp/idp-claims/contents/.github/workflows/pr.yaml | rg -o '@[0-9a-f]{40} # \S+'` prints `@150b7d35dc03831046d732a642e5ff03e33ce8bf # v0.1.1`, and so do `reconcile.yaml`, `drift.yaml` and `recover.yaml`;
  - `gh api -H "Accept: application/vnd.github.raw" repos/jellalshadows-idp/idp-claims-e2e/contents/config/platform.yaml | rg -c aws` prints nothing.

  If `idp-claims` pins anything after `v0.1.1`, or the harness config has an `aws` block, stop and ask: 2d's render would reach a pipeline that cannot apply it yet.
- [ ] The owner approves this plan, its side effects and the rulings D-1 to D-15; ledger them. Commit this plan to `main` as `docs: add the phase 2d plan` and push. The push refreshes release PR #28 and re-runs the harness on it; leave the PR open (D-13).

---

### Task 1: Decisions first — spec amendment A13, ADR-0030 and ADR-0004

**Files:**
- Modify: `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, `docs/adr/0004-floci-multi-account-emulation.md`, `docs/adr/README.md`
- Create: `docs/adr/0030-aws-stack-state-stays-in-the-job.md`

**Interfaces:**
- Produces: ADR-0030 (D-3), which the render's comments (Task 3) and the docs (Task 8) cite; A13 on §5.6, §6, §7.2 and §8.4 (D-12).

- [ ] **Step 1: Create the branch**

Run: `git switch main && git pull --ff-only && git switch -c feat/phase-2d-aws-render`

- [ ] **Step 2: Apply amendment A13 to the spec** (exact replacements with Edit)

1. §5.6, the last paragraph, old:

```markdown
Going to real AWS means replacing this step with OIDC credentials. Claims and
renders do not change.
```

new:

```markdown
Going to real AWS means replacing this step with OIDC credentials, and giving the AWS
stacks a persistent, encrypted backend, which their render does not have in v1
(ADR-0030). Claims do not change. *(Amendment A13, 2026-11-17.)*
```

2. §6, the introduction's end, old:

```markdown
commit is built from source (ADR-0028; *Amendment A9, 2026-10-18*). Until Phase 2
renders and reconciles the AWS stacks (Phases 2d and 2e, §10), the only stack is
`github`, and until Phase 4 there are no
features to fetch or adopt: the workflows fail if any other stack is affected.
*(Amendment A6, 2026-10-11; Amendment A10, 2026-11-03.)*
```

new:

```markdown
commit is built from source (ADR-0028; *Amendment A9, 2026-10-18*). Until Phase 2e
plans and applies the AWS stacks (§10), the workflows reconcile only the `github`
stack and fail if any other stack is affected. The render writes the AWS stacks of
every env with an `aws` block since Phase 2d (§5.4), so until then a claims repo with
one stays on an engine release from before Phase 2d. Until Phase 4 there are no
features to fetch or adopt.
*(Amendment A6, 2026-10-11; Amendment A10, 2026-11-03; Amendment A13, 2026-11-17.)*
```

3. §7.2, the first bullet, old:

```markdown
- The render emits `encryption { state { enforced = true } plan { enforced = true } }`.
  The key material (a PBKDF2 passphrase of at least 16 characters, with AES-GCM)
  comes from `TF_ENCRYPTION`, filled from a secret. If the variable is missing,
  OpenTofu refuses to write plaintext.
```

new:

```markdown
- The render emits `encryption { state { enforced = true } plan { enforced = true } }`
  in the `github` stack, whose state `wet` keeps (§5.4).
  The key material (a PBKDF2 passphrase of at least 16 characters, with AES-GCM)
  comes from `TF_ENCRYPTION`, filled from a secret. If the variable is missing,
  OpenTofu refuses to write plaintext.
- The AWS stacks emit no `encryption` block: their state stays in the job's working
  copy and is gone with the emulator it describes (§5.5, ADR-0030). *(Amendment A13,
  2026-11-17.)*
```

4. §8.4, the first sentences, old:

```markdown
A Go test (`internal/floci`, run by CI's `floci` job) applies `modules/aws/baseline`
and then `modules/aws/component` to two accounts of one floci service container, with
the inputs `internal/naming` computes, as rendered stacks will pass them. It checks
```

new:

```markdown
A Go test (`internal/floci`, run by CI's `floci` job) renders the renderer's
`aws-registry` fixture and applies its AWS stacks, each env's `_baseline` and then its
Component stack, to two accounts of one floci service container, adding only
`floci_override.tf.json`, as CI does (§5.6). *(Amendment A13, 2026-11-17.)* It checks
```

- [ ] **Step 3: Amend ADR-0004** (exact replacements with Edit)

1. "Decision", the replay bullet, old:

```markdown
- Each job brings up its own floci and replays state from `wet` (§6.1, §6.2). Going to real AWS means replacing this CI step with OIDC credentials; claims and renders do not change.
```

new:

```markdown
- Each job brings up its own floci and replays state from `wet` (§6.1, §6.2). Going to real AWS means replacing this CI step with OIDC credentials; claims and renders do not change. *(Amended in Phase 2d, 2026-11-17: what a job replays from `wet` is the rendered AWS stacks, never their state, which stays in the job (ADR-0030); going to real AWS also gives those stacks a persistent, encrypted backend, a render change.)*
```

2. "Consequences", the first positive bullet, old:

```markdown
- The same rendered stacks run unchanged against real AWS later, so the emulation is a CI detail rather than a design fork.
```

new:

```markdown
- The same rendered stacks run unchanged against real AWS later, so the emulation is a CI detail rather than a design fork. *(Amended in Phase 2d, 2026-11-17: except their backend, ADR-0030.)*
```

- [ ] **Step 4: Add ADR-0030**

1. `docs/adr/README.md` (Edit), old:

```markdown
| 0029 | [Self-hosted Renovate with the bot App](0029-self-hosted-renovate.md) | Accepted |
```

new:

```markdown
| 0029 | [Self-hosted Renovate with the bot App](0029-self-hosted-renovate.md) | Accepted |
| 0030 | [The AWS stacks keep their state in the job, unencrypted](0030-aws-stack-state-stays-in-the-job.md) | Accepted |
```

2. Create `docs/adr/0030-aws-stack-state-stays-in-the-job.md` (Write):

```markdown
# 0030. The AWS stacks keep their state in the job, unencrypted

- Status: Accepted
- Date: 2026-11-17
- Spec: [§5.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#54-layout-of-the-wet-branch), [§5.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#55-deterministic-identities-with-no-shared-state-between-stacks), [§5.6](../superpowers/specs/2026-10-08-idp-on-actions-design.md#56-emulator-specifics-belong-to-ci-not-to-the-render), [§7.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#72-encrypted-state)

## Context

Phase 2d renders the AWS stacks (spec §5.4): the `_baseline` of every environment with an `aws` block, and the stack of each Component with `aws.registry: true` in each of its environments. Their resources live in floci, which every job starts empty (ADR-0004), so a job rebuilds the AWS side by applying the stacks again (spec §6.1). The `github` stack is different: GitHub is real, so its state is the only record of what the pipelines created, and `wet` keeps it, encrypted (ADR-0002, spec §7.2). Spec §5.4 lists no AWS state in `wet`, and spec §5.5 says the AWS state cannot be read later. The render must say where an AWS stack keeps its state, and whether that state is encrypted.

## Decision

- Each AWS stack declares the local backend at `terraform.tfstate`, in the stack's own directory. The state lives in the job's working copy, as long as the emulator it describes. Nothing AWS is committed to `wet` but the rendered stacks.
- The AWS stacks emit no `encryption` block. The `github` stack keeps its enforced state and plan encryption.
- A job that sets `TF_ENCRYPTION` anyway, as `idp encryption-env` does for the whole job, encrypts the AWS state with it, and must keep it set for every command on that stack; a job without it writes plain state. Both work, because the state never leaves the job.

## Consequences

### Positive

- The AWS jobs need no passphrase, so `IDP_STATE_PASSPHRASE` never has to reach the floci jobs, which hold the runner's Docker socket (ADR-0004).
- `wet` keeps one state file, and no AWS state can go stale against an emulator that no longer exists.
- The render stays thin (spec §5.3): the backend is one constant path, pinned by the golden files.

### Negative / costs

- Going to real AWS changes the render: the AWS stacks then need a persistent backend and encryption, and the replay of spec §6.1 changes with them (spec §5.6, amendment A13).
- The state file is written inside the stack's directory. Phase 2e must run the AWS stacks in a scratch copy, as the `github` stack already runs in `$RUNNER_TEMP/idp/work`, and must never push AWS state to `wet`; `idp diff` would read a `terraform.tfstate` left in a rendered tree as a file of its stack.

## Alternatives considered

- **AWS state in `wet`, encrypted, like the `github` stack.** Rejected: it would describe resources that are gone when the job ends, so the next job would plan against state that matches nothing, and the floci jobs would need the passphrase.
- **An `encryption` block, enforced.** Rejected: every AWS job would need `TF_ENCRYPTION`, which puts the passphrase in the jobs that hold the Docker socket, to protect state that never leaves the job and holds only names that the renders and the environment variables already publish.
- **No backend block, OpenTofu's default local state.** The same behavior. Rejected for clarity: the explicit block states the decision in every stack, and the golden files pin it.

## References

- Spec §5.4, §5.5, §5.6 (amendment A13), §6.1, §7.2 (amendment A13).
- ADR-0002 (the `github` stack's encrypted state), ADR-0004 (one floci per job, amended in Phase 2d), ADR-0020 (`TF_ENCRYPTION`).
- Verified while planning Phase 2d with OpenTofu 1.12.6: a stack without an `encryption` block writes plain state without `TF_ENCRYPTION`, and encrypted state with it; that encrypted state cannot be read once the variable is unset.
```

- [ ] **Step 5: Check and commit**

Run:

```bash
python -I -c "
import re,pathlib
spec=pathlib.Path('docs/superpowers/specs/2026-10-08-idp-on-actions-design.md').read_text(encoding='utf-8')
def slug(h):
    h=h.strip().lower(); h=re.sub(r'[^\w\- ]','',h); return h.replace(' ','-')
anchors={slug(m.group(1)) for m in re.finditer(r'^#+ (.+)$', spec, re.M)}
bad=[(str(p),a) for p in pathlib.Path('docs').rglob('*.md') if 'superpowers' not in p.parts for a in re.findall(r'2026-10-08-idp-on-actions-design\.md#([\w\-]+)', p.read_text(encoding='utf-8')) if a not in anchors]
print(bad or 'all spec anchors resolve')
"
```

Expected: `all spec anchors resolve` (ADR-0030's four spec links included). Then:
- `rg -c "Amendment A13" docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` prints `4`;
- `rg -c "Amended in Phase 2d" docs/adr/0004-floci-multi-account-emulation.md` prints `2`;
- `rg -n 'renders do not change|the only stack is' docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` prints nothing;
- the naming check prints nothing.

```bash
git add docs/superpowers/specs/2026-10-08-idp-on-actions-design.md docs/adr/0004-floci-multi-account-emulation.md docs/adr/README.md docs/adr/0030-aws-stack-state-stays-in-the-job.md
git commit -m "docs: record where the aws stacks keep their state (spec amendment a13, adr-0030)"
```

---

### Task 2: Module sources for a stack at any depth

**Files:**
- Modify: `internal/render/render_test.go`, `internal/render/render.go`

**Interfaces:**
- Consumes: `Options`, `version.Repo`.
- Produces: `moduleSource(opts Options, stackDir, module string) string`, where `stackDir` is a slash path under the output root and `module` is `<provider>/<name>` (Task 3 uses it for `aws/baseline` and `aws/component`).

- [ ] **Step 1: Write the failing test** — `internal/render/render_test.go` (Edit), old:

```go
func TestRenderLocalModules(t *testing.T) {
```

new:

```go
// TestModuleSource: ModulesDir is relative to the github stack, so a deeper
// stack climbs to the output root and descends into github first. Every form
// resolves to the same directory from its own stack, and the github stack's
// sources stay ModulesDir as given, byte for byte.
func TestModuleSource(t *testing.T) {
	tests := []struct{ modulesDir, stackDir, module, want string }{
		{"../../modules", "github", "github/group", "../../modules/github/group"},
		{"../../modules/", "github", "github/group", "../../modules/github/group"},
		{"./mods", "github", "github/component", "./mods/github/component"},
		{"../../modules", "aws/dev/_baseline", "aws/baseline", "../../../../modules/aws/baseline"},
		{"../../modules/", "aws/dev/_baseline", "aws/baseline", "../../../../modules/aws/baseline"},
		{"./mods", "aws/dev/_baseline", "aws/baseline", "../../../github/mods/aws/baseline"},
		{"../../modules", "aws/dev/components/api", "aws/component", "../../../../../modules/aws/component"},
		{"./mods", "aws/dev/components/api", "aws/component", "../../../../github/mods/aws/component"},
	}
	for _, tt := range tests {
		if got := moduleSource(Options{ModulesDir: tt.modulesDir}, tt.stackDir, tt.module); got != tt.want {
			t.Errorf("moduleSource(%q, %q, %q) = %q, want %q", tt.modulesDir, tt.stackDir, tt.module, got, tt.want)
		}
	}
	if got, want := moduleSource(Options{ModuleRef: "v1"}, "aws/dev/_baseline", "aws/baseline"), "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/aws/baseline?ref=v1"; got != want {
		t.Errorf("git source = %q, want %q", got, want)
	}
}

func TestRenderLocalModules(t *testing.T) {
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/render/ -count=1 -run TestModuleSource`
Expected: build failure, `too many arguments in call to moduleSource` with `have (Options, string, string)` and `want (Options, string)`.

- [ ] **Step 3: Implement** — `internal/render/render.go` (Edit).

1. Imports, old:

```go
	"errors"
	"fmt"
	"strings"
```

new:

```go
	"errors"
	"fmt"
	"path"
	"strings"
```

2. The group call in `githubStack`, old:

```go
			"source":      moduleSource(opts, "group"),
```

new:

```go
			"source":      moduleSource(opts, "github", "github/group"),
```

3. The component call in `githubStack`, old:

```go
			"source":             moduleSource(opts, "component"),
```

new:

```go
			"source":             moduleSource(opts, "github", "github/component"),
```

4. `moduleSource`, old:

```go
func moduleSource(opts Options, module string) string {
	if opts.ModulesDir != "" {
		return strings.TrimSuffix(opts.ModulesDir, "/") + "/github/" + module
	}
	return fmt.Sprintf("git::https://%s.git//modules/github/%s?ref=%s", version.Repo, module, opts.ModuleRef)
}
```

new:

```go
// moduleSource is where the stack in stackDir, a slash path relative to the
// output root, finds the engine's module <provider>/<name>: the engine's git
// repository at ModuleRef, or ModulesDir. ModulesDir is relative to the github
// stack, so a deeper stack climbs to the output root and descends into github
// first. The github stack's sources are ModulesDir as given.
func moduleSource(opts Options, stackDir, module string) string {
	if opts.ModulesDir == "" {
		return fmt.Sprintf("git::https://%s.git//modules/%s?ref=%s", version.Repo, module, opts.ModuleRef)
	}
	dir := strings.TrimSuffix(opts.ModulesDir, "/")
	if stackDir != "github" {
		dir = path.Join(strings.Repeat("../", strings.Count(stackDir, "/")+1)+"github", dir)
	}
	return dir + "/" + module
}
```

5. The `Options.ModulesDir` comment, old:

```go
	// ModulesDir replaces git sources with local paths, relative to the github stack
	// directory and starting with ./ or ../. CI uses it to validate a render before
	// its ref exists.
	ModulesDir string
```

new:

```go
	// ModulesDir replaces git sources with local paths, relative to the github stack
	// directory and starting with ./ or ../. CI uses it to validate a render before
	// its ref exists. The AWS stacks, deeper in the tree, get the same directory
	// expressed from their own.
	ModulesDir string
```

- [ ] **Step 4: Run it to see it pass, and commit**

Run: `go test ./internal/render/ ./internal/cli/ -count=1 -v -run 'TestModuleSource|TestRenderGolden|TestRenderEnvVariablesGolden|TestRenderLocalModules'`
Expected: `PASS` for each, and `ok` twice: the github golden files and `idp render --modules-dir` are unchanged. `git status --short internal/render/testdata` prints nothing. Then `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

```bash
git add internal/render/render.go internal/render/render_test.go
git commit -m "refactor(render): resolve module sources for a stack at any depth"
```

- [ ] **Step 5: Mutation check** (nothing is committed) — in `internal/render/render.go`, replace `	if stackDir != "github" {` with `	if true {`. Run `go test ./internal/render/ -count=1 -run TestModuleSource`. Expected: `FAIL` with `moduleSource("./mods", "github", "github/component") = "../github/mods/github/component", want "./mods/github/component"`. Restore with `git checkout internal/render/render.go`; `git status --short` prints nothing.

---

### Task 3: The AWS stacks, their golden files and their identities

**Files:**
- Modify: `internal/render/render_test.go`, `internal/render/render.go`, `internal/cli/claims_test.go`
- Create (by `-update`): `internal/render/testdata/aws-registry/expected/aws/dev/_baseline/main.tf.json`, `…/aws/pro/_baseline/main.tf.json`, `…/aws/dev/components/api/main.tf.json`, `…/aws/pro/components/api/main.tf.json`

**Interfaces:**
- Consumes:
  - `moduleSource` (Task 2), `literal`, `marshal`;
  - `naming.RepositoryName`, `naming.RoleName`, `naming.OIDCProviderARN`;
  - `lockfiles.AWS`;
  - `claims.Model`, `claims.AWS`.
- Produces:
  - `AWSProviderVersion` (Task 4) and `awsStateFile`;
  - `awsStacks(m *claims.Model, opts Options) (map[string]map[string]any, error)` and `awsStack(aws *claims.AWS, name string, call map[string]any) map[string]any`;
  - `Render`'s AWS files, which Tasks 5 and 6 read;
  - the test helpers `checkGolden(t, fixture, rel, got)`, `checkGoldens`, `renderedAWSStack`, `awsStackOf` and `stackDirs`;
  - the four golden files (Task 4 lists them).

- [ ] **Step 1: Write the failing render tests** — `internal/render/render_test.go` (Edit).

1. Imports, old:

```go
	"flag"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)
```

new:

```go
	"flag"
	"io/fs"
	"maps"
	"math/rand/v2"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)
```

2. `checkGolden`, and the new helpers after it, old:

```go
// checkGolden compares a rendered github stack with the golden file of fixture,
// and rewrites that file first when the test runs with -update.
func checkGolden(t *testing.T, fixture string, got []byte) {
	t.Helper()
	golden := filepath.Join("testdata", fixture, "expected", "github", "main.tf.json")
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s: main.tf.json differs from its golden file:\n%s", fixture, got)
	}
}
```

new:

```go
// checkGolden compares one rendered file, rel, with its golden file under
// testdata/<fixture>/expected, and rewrites that file first when the test runs
// with -update.
func checkGolden(t *testing.T, fixture, rel string, got []byte) {
	t.Helper()
	golden := filepath.Join("testdata", fixture, "expected", filepath.FromSlash(rel))
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s: %s differs from its golden file:\n%s", fixture, rel, got)
	}
}

// checkGoldens compares every stack of a render with fixture's golden files:
// each main.tf.json byte for byte, and the set of stacks, so a stack that
// appears or disappears fails too. -update rewrites the golden files of the
// rendered stacks; a golden file of a stack that is no longer rendered must be
// deleted by hand.
func checkGoldens(t *testing.T, fixture string, files map[string][]byte) {
	t.Helper()
	var rendered []string
	for p := range files {
		if path.Base(p) == "main.tf.json" {
			rendered = append(rendered, p)
		}
	}
	slices.Sort(rendered)
	for _, p := range rendered {
		checkGolden(t, fixture, p, files[p])
	}
	expected := filepath.Join("testdata", fixture, "expected")
	var golden []string
	err := filepath.WalkDir(expected, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "main.tf.json" {
			return err
		}
		rel, err := filepath.Rel(expected, p)
		golden = append(golden, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(golden)
	if !slices.Equal(rendered, golden) {
		t.Errorf("%s: rendered stacks %v, golden stacks %v", fixture, rendered, golden)
	}
}

// renderedAWSStack is what the tests read of a rendered AWS stack.
type renderedAWSStack struct {
	Provider struct {
		AWS struct {
			Region string `json:"region"`
		} `json:"aws"`
	} `json:"provider"`
	Module map[string]map[string]any `json:"module"`
}

// awsStackOf decodes the AWS stack rendered in dir.
func awsStackOf(t *testing.T, files map[string][]byte, dir string) renderedAWSStack {
	t.Helper()
	b, ok := files[dir+"/main.tf.json"]
	if !ok {
		t.Fatalf("no stack %s in %v", dir, slices.Sorted(maps.Keys(files)))
	}
	var s renderedAWSStack
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// stackDirs lists the stacks of a render: the directories of its main.tf.json files.
func stackDirs(files map[string][]byte) []string {
	var out []string
	for p := range files {
		if path.Base(p) == "main.tf.json" {
			out = append(out, path.Dir(p))
		}
	}
	slices.Sort(out)
	return out
}
```

3. `TestRenderGolden`, old:

```go
// TestRenderGolden pins the render of a claims repo without any aws part. Phase 2b
// left this golden file unchanged, byte for byte: a Component without
// aws.registry passes no variables, so existing stacks plan no change.
func TestRenderGolden(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "basic", files["github/main.tf.json"])
}
```

new:

```go
// TestRenderGolden pins the render of a claims repo without any aws part:
// only the github stack, whose golden file Phases 2b and 2d left unchanged,
// byte for byte. A Component without aws.registry passes no variables, and an
// environment without an aws block has no AWS stack, so existing claims repos
// plan no change.
func TestRenderGolden(t *testing.T) {
	files, err := Render(loadFixture(t, "basic"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(files)); !slices.Equal(got, []string{"github/.terraform.lock.hcl", "github/main.tf.json"}) {
		t.Errorf("rendered files = %v, want only the github stack", got)
	}
	checkGoldens(t, "basic", files)
}
```

4. `TestRenderEnvVariablesGolden`, its comment and its first lines, old:

```go
// pro's are spec §5.3's example, checked verbatim here so that -update cannot
// change them unnoticed. Only the github stack is rendered until Phase 2d adds
// the AWS stacks.
func TestRenderEnvVariablesGolden(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	if !m.Components[0].AWS.Registry || m.Platform.Environments["dev"].AWS == nil || m.Platform.Environments["pro"].AWS == nil {
		t.Fatal("the fixture must set aws.registry and give every environment an aws block")
	}
	files, err := Render(m, Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(files)); !slices.Equal(got, []string{"github/.terraform.lock.hcl", "github/main.tf.json"}) {
		t.Errorf("rendered files = %v, want only the github stack", got)
	}
	checkGolden(t, "aws-registry", files["github/main.tf.json"])
```

new:

```go
// pro's are spec §5.3's example, checked verbatim here so that -update cannot
// change them unnoticed. TestRenderAWSStacksGolden checks the fixture's AWS
// stacks.
func TestRenderEnvVariablesGolden(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	if !m.Components[0].AWS.Registry || m.Platform.Environments["dev"].AWS == nil || m.Platform.Environments["pro"].AWS == nil {
		t.Fatal("the fixture must set aws.registry and give every environment an aws block")
	}
	files, err := Render(m, Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "aws-registry", "github/main.tf.json", files["github/main.tf.json"])
```

5. `TestRenderWithoutRegistryPassesNoVariables`, and the four new tests after it, old:

```go
// TestRenderWithoutRegistryPassesNoVariables keeps Phase 2a's guard for every
// Component without aws.registry: the environments' aws blocks alone change
// nothing, so the render is basic's golden file, byte for byte.
func TestRenderWithoutRegistryPassesNoVariables(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Components[0].AWS.Registry = false
	files, err := Render(m, Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "basic", files["github/main.tf.json"])
}
```

new:

```go
// TestRenderWithoutRegistryPassesNoVariables keeps Phase 2a's guard for every
// Component without aws.registry: the environments' aws blocks alone leave the
// github stack basic's golden file, byte for byte, and add only each
// environment's baseline, never a Component stack.
func TestRenderWithoutRegistryPassesNoVariables(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Components[0].AWS.Registry = false
	files, err := Render(m, Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "basic", "github/main.tf.json", files["github/main.tf.json"])
	if got, want := stackDirs(files), []string{"aws/dev/_baseline", "aws/pro/_baseline", "github"}; !slices.Equal(got, want) {
		t.Errorf("stacks = %v, want %v", got, want)
	}
}

// TestRenderAWSStacksGolden pins the AWS stacks of the aws-registry fixture
// (spec §5.4): each environment's baseline and the Component's stack in each
// of its environments, each with the engine's AWS lock file.
func TestRenderAWSStacksGolden(t *testing.T) {
	files, err := Render(loadFixture(t, "aws-registry"), Options{ModuleRef: "v0.0.0-test"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"aws/dev/_baseline", "aws/dev/components/api", "aws/pro/_baseline", "aws/pro/components/api", "github"}
	if got := stackDirs(files); !slices.Equal(got, want) {
		t.Errorf("stacks = %v, want %v", got, want)
	}
	if len(files) != 2*len(want) {
		t.Errorf("rendered %d files, want a main.tf.json and a lock file for each of the %d stacks: %v", len(files), len(want), slices.Sorted(maps.Keys(files)))
	}
	for _, dir := range want[:4] {
		if !bytes.Equal(files[dir+"/.terraform.lock.hcl"], lockfiles.AWS) {
			t.Errorf("%s must carry the engine's AWS lock file", dir)
		}
	}
	checkGoldens(t, "aws-registry", files)
}

// TestRenderAWSStacksFollowTheAWSBlocks: every environment with an aws block
// gets its baseline, used by a Component or not, and an environment without one
// gets no AWS stack; a registry Component gets a stack in each environment it
// lists, and only there; a Component without aws.registry gets none.
func TestRenderAWSStacksFollowTheAWSBlocks(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	m.Platform.Environments["staging"] = claims.Environment{AWS: &claims.AWS{AccountID: "000000000002", Region: "eu-central-1"}}
	m.Platform.Environments["qa"] = claims.Environment{}
	web := m.Components[0]
	web.Name, web.Environments = "web", []string{"dev", "qa", "staging"}
	web.AWS.Registry = false
	m.Components = append(m.Components, web)
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"aws/dev/_baseline", "aws/dev/components/api", "aws/pro/_baseline", "aws/pro/components/api", "aws/staging/_baseline", "github"}
	if got := stackDirs(files); !slices.Equal(got, want) {
		t.Errorf("stacks = %v, want %v", got, want)
	}
	if got := awsStackOf(t, files, "aws/staging/_baseline").Provider.AWS.Region; got != "eu-central-1" {
		t.Errorf("staging baseline region = %q, want its environment's eu-central-1", got)
	}
}

// TestRenderAWSStacksUseNamingsIdentities pins spec §5.5's identities in the
// rendered Component stacks, outside the golden files, so that -update cannot
// change them unnoticed: pro's module call verbatim, and in every environment
// the role, the repository and the region that the github stack's variables
// name, and the account's GitHub OIDC provider.
func TestRenderAWSStacksUseNamingsIdentities(t *testing.T) {
	m := loadFixture(t, "aws-registry")
	files, err := Render(m, Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"source":            "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/aws/component?ref=v1",
		"github_repository": "acme/api",
		"environment":       "pro",
		"repository_name":   "api",
		"role_name":         "api-pro-ci",
		"oidc_provider_arn": "arn:aws:iam::000000000003:oidc-provider/token.actions.githubusercontent.com",
	}
	if got := awsStackOf(t, files, "aws/pro/components/api").Module; !reflect.DeepEqual(got, map[string]map[string]any{"component": want}) {
		t.Errorf("pro module = %v, want only component = %v", got, want)
	}
	vars := componentEnvironments(t, files["github/main.tf.json"], "api")
	for _, env := range m.Components[0].Environments {
		aws := m.Platform.Environments[env].AWS
		s := awsStackOf(t, files, "aws/"+env+"/components/api")
		call := s.Module["component"]
		v := vars[env].Variables
		if got, want := "arn:aws:iam::"+aws.AccountID+":role/"+call["role_name"].(string), v["AWS_ROLE_ARN"]; got != want {
			t.Errorf("%s: the stack creates the role %s, the github stack passes AWS_ROLE_ARN %s", env, got, want)
		}
		if got, want := aws.AccountID+".dkr.ecr."+aws.Region+".amazonaws.com/"+call["repository_name"].(string), v["ECR_REPOSITORY"]; got != want {
			t.Errorf("%s: the stack creates the repository %s, the github stack passes ECR_REPOSITORY %s", env, got, want)
		}
		if s.Provider.AWS.Region != v["AWS_REGION"] {
			t.Errorf("%s: the stack's region %s, the github stack passes AWS_REGION %s", env, s.Provider.AWS.Region, v["AWS_REGION"])
		}
		if got, want := call["oidc_provider_arn"], naming.OIDCProviderARN(aws.AccountID); got != want {
			t.Errorf("%s: oidc_provider_arn = %v, want the account's %s (naming.OIDCProviderARN)", env, got, want)
		}
		if got := awsStackOf(t, files, "aws/"+env+"/_baseline").Module; !reflect.DeepEqual(got, map[string]map[string]any{"baseline": {"source": "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/aws/baseline?ref=v1"}}) {
			t.Errorf("%s baseline module = %v, want only the baseline call, with no inputs", env, got)
		}
	}
}

// TestRenderAWSStacksAreThin: an AWS stack holds the pinned versions, a local
// backend in its own directory, no encryption block, the region and one module
// call (spec §5.3, ADR-0030), and nothing about floci, which CI adds in
// floci_override.tf.json (spec §5.6).
func TestRenderAWSStacksAreThin(t *testing.T) {
	files, err := Render(loadFixture(t, "aws-registry"), Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range stackDirs(files) {
		if !strings.HasPrefix(dir, "aws/") {
			continue
		}
		b := files[dir+"/main.tf.json"]
		var doc struct {
			Terraform map[string]any            `json:"terraform"`
			Provider  map[string]any            `json:"provider"`
			Module    map[string]map[string]any `json:"module"`
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&doc); err != nil {
			t.Errorf("%s: %v: a stack holds only terraform, provider and module", dir, err)
			continue
		}
		wantTerraform := map[string]any{
			"required_version":   TofuVersion,
			"required_providers": map[string]any{"aws": map[string]any{"source": "hashicorp/aws", "version": AWSProviderVersion}},
			"backend":            map[string]any{"local": map[string]any{"path": "terraform.tfstate"}},
		}
		if !reflect.DeepEqual(doc.Terraform, wantTerraform) {
			t.Errorf("%s: terraform = %v, want %v", dir, doc.Terraform, wantTerraform)
		}
		if want := map[string]any{"aws": map[string]any{"region": "eu-west-1"}}; !reflect.DeepEqual(doc.Provider, want) {
			t.Errorf("%s: provider = %v, want only the region, %v", dir, doc.Provider, want)
		}
		if len(doc.Module) != 1 {
			t.Errorf("%s: %d module calls, want one", dir, len(doc.Module))
		}
		for _, word := range []string{"floci", "localhost", "4566", "endpoint", "skip_", "encryption"} {
			if bytes.Contains(bytes.ToLower(b), []byte(word)) {
				t.Errorf("%s mentions %q: emulator settings belong to CI (spec §5.6), and the AWS stacks keep no encrypted state (ADR-0030):\n%s", dir, word, b)
			}
		}
	}
}
```

6. `TestRenderRejectsARegistryEnvironmentWithoutAWS`, its end, old:

```go
	want := `render: component "api" has aws.registry, but environment "pro" has no aws block`
	if err == nil || err.Error() != want || files != nil {
		t.Errorf("Render = %d files, error %v; want no files and the error %q", len(files), err, want)
	}
}
```

new:

```go
	want := `render: component "api" has aws.registry, but environment "pro" has no aws block`
	if err == nil || err.Error() != want || files != nil {
		t.Errorf("Render = %d files, error %v; want no files and the error %q", len(files), err, want)
	}
	if stacks, err := awsStacks(m, Options{ModuleRef: "v1"}); err == nil || err.Error() != want || stacks != nil {
		t.Errorf("awsStacks = %d stacks, error %v; want no stacks and the error %q", len(stacks), err, want)
	}
}
```

7. `TestRenderCopiesTheLockFile`, its end, old:

```go
	if !regexp.MustCompile(`version\s*=\s*"` + regexp.QuoteMeta(GitHubProviderVersion) + `"`).Match(lockfiles.GitHub) {
		t.Errorf("lock file does not pin GitHubProviderVersion %s", GitHubProviderVersion)
	}
}
```

new:

```go
	if !regexp.MustCompile(`version\s*=\s*"` + regexp.QuoteMeta(GitHubProviderVersion) + `"`).Match(lockfiles.GitHub) {
		t.Errorf("lock file does not pin GitHubProviderVersion %s", GitHubProviderVersion)
	}
	if !regexp.MustCompile(`version\s*=\s*"` + regexp.QuoteMeta(AWSProviderVersion) + `"`).Match(lockfiles.AWS) {
		t.Errorf("the AWS lock file does not pin AWSProviderVersion %s", AWSProviderVersion)
	}
}
```

8. `TestRenderLocalModules`, its end, old:

```go
	s := string(files["github/main.tf.json"])
	for _, want := range []string{`"source": "../../modules/github/group"`, `"source": "../../modules/github/component"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
}
```

new:

```go
	s := string(files["github/main.tf.json"])
	for _, want := range []string{`"source": "../../modules/github/group"`, `"source": "../../modules/github/component"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
	files, err = Render(loadFixture(t, "aws-registry"), Options{ModulesDir: "../../modules"})
	if err != nil {
		t.Fatal(err)
	}
	for dir, want := range map[string]string{
		"aws/dev/_baseline":      `"source": "../../../../modules/aws/baseline"`,
		"aws/dev/components/api": `"source": "../../../../../modules/aws/component"`,
	} {
		if !strings.Contains(string(files[dir+"/main.tf.json"]), want) {
			t.Errorf("%s: missing %s, the same directory seen from the stack", dir, want)
		}
	}
}
```

9. `TestRenderIsDeterministic`, its comment, old:

```go
// TestRenderIsDeterministic is spec §8.2's determinism test at the boundary Render
// consumes: any claim order, and any order of a Component's environments, gives
// identical bytes, env variables included. (Load's half, sorted output whatever
// the read order, is TestLoadSortsClaimsByName in internal/claims.)
```

new:

```go
// TestRenderIsDeterministic is spec §8.2's determinism test at the boundary Render
// consumes: any claim order, and any order of a Component's environments, gives
// identical files, env variables and AWS stacks included. The platform's
// environments are a map, which Go ranges over in a new order on every render.
// (Load's half, sorted output whatever the read order, is
// TestLoadSortsClaimsByName in internal/claims.)
```

10. `TestRenderIsDeterministic`, the comparison, old:

```go
		got, err := Render(&shuffled, Options{ModuleRef: "v1"})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got["github/main.tf.json"], want["github/main.tf.json"]) {
			t.Fatalf("iteration %d: render output depends on claim order", i)
		}
```

new:

```go
		got, err := Render(&shuffled, Options{ModuleRef: "v1"})
		if err != nil {
			t.Fatal(err)
		}
		if !maps.EqualFunc(got, want, bytes.Equal) {
			t.Fatalf("iteration %d: render output depends on claim order", i)
		}
```

- [ ] **Step 2: Write the failing CLI test** — `internal/cli/claims_test.go` (Edit).

1. Imports, old:

```go
import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)
```

new:

```go
import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)
```

2. Before `TestRenderRefusesForeignOutDir`, old:

```go
func TestRenderRefusesForeignOutDir(t *testing.T) {
```

new:

```go
// TestRenderLocalModulesResolveFromEveryStack: every rendered stack, the AWS
// stacks deeper in the tree included, finds --modules-dir from its own
// directory, which is what render-smoke validates in CI.
func TestRenderLocalModulesResolveFromEveryStack(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")
	modules := filepath.Join(root, "engine", "modules")
	fixture := filepath.Join("..", "render", "testdata", "aws-registry")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"render", "--dir", fixture, "--out", out, "--modules-dir", modules}, &stdout, &stderr, noEnv); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	var stacks, resolved []string
	err := filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Name() != "main.tf.json" {
			return err
		}
		dir := filepath.Dir(p)
		rel, err := filepath.Rel(out, dir)
		if err != nil {
			return err
		}
		stacks = append(stacks, filepath.ToSlash(rel))
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var doc struct {
			Module map[string]struct {
				Source string `json:"source"`
			} `json:"module"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			return err
		}
		for _, call := range doc.Module {
			mod, err := filepath.Rel(modules, filepath.Join(dir, filepath.FromSlash(call.Source)))
			if err != nil {
				return err
			}
			if mod = filepath.ToSlash(mod); !slices.Contains(resolved, mod) {
				resolved = append(resolved, mod)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(stacks)
	slices.Sort(resolved)
	if want := []string{"aws/dev/_baseline", "aws/dev/components/api", "aws/pro/_baseline", "aws/pro/components/api", "github"}; !slices.Equal(stacks, want) {
		t.Errorf("stacks = %v, want %v", stacks, want)
	}
	if want := []string{"aws/baseline", "aws/component", "github/component", "github/group"}; !slices.Equal(resolved, want) {
		t.Errorf("module sources resolve to %v under %s, want %v", resolved, modules, want)
	}
}

func TestRenderRefusesForeignOutDir(t *testing.T) {
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/render/ ./internal/cli/ -count=1`
Expected:
- `internal/render`: build failure, `undefined: AWSProviderVersion` and `undefined: awsStacks`;
- `internal/cli`: `--- FAIL: TestRenderLocalModulesResolveFromEveryStack` with `stacks = [github], want [aws/dev/_baseline aws/dev/components/api aws/pro/_baseline aws/pro/components/api github]` and `module sources resolve to [github/component github/group]`.

- [ ] **Step 4: Implement** — `internal/render/render.go` (Edit).

1. Imports, old:

```go
	"fmt"
	"path"
	"strings"
```

new:

```go
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
```

2. The pinned versions and the AWS backend path, old:

```go
// Pinned versions the rendered stacks require (spec §7.5).
const (
	TofuVersion           = "1.13.2"
	GitHubProviderVersion = "6.13.0"
)

// stateFile is the GitHub stack's local backend path, relative to rendered/github
// in a checkout of the wet branch (spec §5.4).
const stateFile = "../../tfstate/github.tfstate"
```

new:

```go
// Pinned versions the rendered stacks require (spec §7.5). AWSProviderVersion
// equals the pin in modules/aws/*/versions.tf and the AWS lock file.
const (
	TofuVersion           = "1.13.2"
	GitHubProviderVersion = "6.13.0"
	AWSProviderVersion    = "6.71.0"
)

// stateFile is the GitHub stack's local backend path, relative to rendered/github
// in a checkout of the wet branch (spec §5.4).
const stateFile = "../../tfstate/github.tfstate"

// awsStateFile is every AWS stack's local backend path: in the stack's own
// directory, in the job's working copy, and never committed to wet. AWS state
// lives as long as the emulator it describes (spec §5.5, ADR-0030).
const awsStateFile = "terraform.tfstate"
```

3. `Render`, and `awsStacks` and `awsStack` after it, old:

```go
// Render returns the rendered files, keyed by slash-separated path relative to the
// output root. It fails, rather than render less, on a model that Load rejects in
// a way the render depends on, such as a registry Component in an environment
// without an aws block.
func Render(m *claims.Model, opts Options) (map[string][]byte, error) {
	if opts.ModuleRef == "" && opts.ModulesDir == "" {
		return nil, errors.New("render: set ModuleRef or ModulesDir")
	}
	doc, err := githubStack(m, opts)
	if err != nil {
		return nil, err
	}
	stack, err := marshal(doc)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"github/main.tf.json":        stack,
		"github/.terraform.lock.hcl": lockfiles.GitHub,
	}, nil
}
```

new:

```go
// Render returns the rendered files, keyed by slash-separated path relative to the
// output root: the github stack, and the AWS stacks of spec §5.4. It fails, rather
// than render less, on a model that Load rejects in a way the render depends on,
// such as a registry Component in an environment without an aws block.
func Render(m *claims.Model, opts Options) (map[string][]byte, error) {
	if opts.ModuleRef == "" && opts.ModulesDir == "" {
		return nil, errors.New("render: set ModuleRef or ModulesDir")
	}
	doc, err := githubStack(m, opts)
	if err != nil {
		return nil, err
	}
	stack, err := marshal(doc)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		"github/main.tf.json":        stack,
		"github/.terraform.lock.hcl": lockfiles.GitHub,
	}
	stacks, err := awsStacks(m, opts)
	if err != nil {
		return nil, err
	}
	for dir, doc := range stacks {
		b, err := marshal(doc)
		if err != nil {
			return nil, err
		}
		files[dir+"/main.tf.json"] = b
		files[dir+"/.terraform.lock.hcl"] = lockfiles.AWS
	}
	return files, nil
}

// awsStacks are the AWS stacks of spec §5.4, keyed by directory: the baseline of
// every environment with an aws block, which creates the account's GitHub OIDC
// provider whether or not a Component uses it, and the stack of each Component
// with aws.registry in each environment it lists. An environment without an aws
// block has no AWS stack.
func awsStacks(m *claims.Model, opts Options) (map[string]map[string]any, error) {
	p := m.Platform
	stacks := map[string]map[string]any{}
	for _, env := range slices.Sorted(maps.Keys(p.Environments)) {
		aws := p.Environments[env].AWS
		if aws == nil {
			continue
		}
		dir := "aws/" + env + "/_baseline"
		stacks[dir] = awsStack(aws, "baseline", map[string]any{
			"source": moduleSource(opts, dir, "aws/baseline"),
		})
	}
	for _, c := range m.Components {
		if !c.AWS.Registry {
			continue
		}
		for _, env := range c.Environments {
			aws := p.Environments[env].AWS
			if aws == nil {
				// Load reports this (spec §4.7), and githubStack fails on it first.
				return nil, fmt.Errorf("render: component %q has aws.registry, but environment %q has no aws block", c.Name, env)
			}
			dir := "aws/" + env + "/components/" + c.Name
			// The inputs come from internal/naming, as the github stack's
			// variables do (spec §5.5), so both stacks name the same role and
			// repository.
			stacks[dir] = awsStack(aws, "component", map[string]any{
				"source":            moduleSource(opts, dir, "aws/component"),
				"github_repository": literal(p.GitHub.Org + "/" + c.Name),
				"environment":       literal(env),
				"repository_name":   literal(naming.RepositoryName(c.Name)),
				"role_name":         literal(naming.RoleName(c.Name, env)),
				"oidc_provider_arn": literal(naming.OIDCProviderARN(aws.AccountID)),
			})
		}
	}
	return stacks, nil
}

// awsStack is one thin AWS stack (spec §5.3): the pinned versions, a local
// backend with no encryption block (ADR-0030), the environment's region and one
// call of the engine's module. The emulator's endpoint and provider flags are
// not here: CI adds them in floci_override.tf.json (spec §5.6).
func awsStack(aws *claims.AWS, name string, call map[string]any) map[string]any {
	return map[string]any{
		"terraform": map[string]any{
			"required_version": TofuVersion,
			"required_providers": map[string]any{
				"aws": map[string]any{"source": "hashicorp/aws", "version": AWSProviderVersion},
			},
			"backend": map[string]any{"local": map[string]any{"path": awsStateFile}},
		},
		"provider": map[string]any{"aws": map[string]any{"region": literal(aws.Region)}},
		"module":   map[string]any{name: call},
	}
}
```

- [ ] **Step 5: Generate the golden files**

Run: `go test ./internal/render/ -count=1 -run TestRenderAWSStacksGolden -update`
Expected: `ok`. `git status --short internal/render/testdata` prints only `?? internal/render/testdata/aws-registry/expected/aws/`, and the existing golden files are unchanged. Review the four new files against these, byte for byte:
- `aws/dev/_baseline/main.tf.json` and `aws/pro/_baseline/main.tf.json`, identical (the same region and no input):

```json
{
  "module": {
    "baseline": {
      "source": "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/aws/baseline?ref=v0.0.0-test"
    }
  },
  "provider": {
    "aws": {
      "region": "eu-west-1"
    }
  },
  "terraform": {
    "backend": {
      "local": {
        "path": "terraform.tfstate"
      }
    },
    "required_providers": {
      "aws": {
        "source": "hashicorp/aws",
        "version": "6.71.0"
      }
    },
    "required_version": "1.13.2"
  }
}
```

- `aws/dev/components/api/main.tf.json`:

```json
{
  "module": {
    "component": {
      "environment": "dev",
      "github_repository": "acme/api",
      "oidc_provider_arn": "arn:aws:iam::000000000001:oidc-provider/token.actions.githubusercontent.com",
      "repository_name": "api",
      "role_name": "api-dev-ci",
      "source": "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/aws/component?ref=v0.0.0-test"
    }
  },
  "provider": {
    "aws": {
      "region": "eu-west-1"
    }
  },
  "terraform": {
    "backend": {
      "local": {
        "path": "terraform.tfstate"
      }
    },
    "required_providers": {
      "aws": {
        "source": "hashicorp/aws",
        "version": "6.71.0"
      }
    },
    "required_version": "1.13.2"
  }
}
```

- `aws/pro/components/api/main.tf.json`:

```json
{
  "module": {
    "component": {
      "environment": "pro",
      "github_repository": "acme/api",
      "oidc_provider_arn": "arn:aws:iam::000000000003:oidc-provider/token.actions.githubusercontent.com",
      "repository_name": "api",
      "role_name": "api-pro-ci",
      "source": "git::https://github.com/jellalshadows-idp/idp-engine.git//modules/aws/component?ref=v0.0.0-test"
    }
  },
  "provider": {
    "aws": {
      "region": "eu-west-1"
    }
  },
  "terraform": {
    "backend": {
      "local": {
        "path": "terraform.tfstate"
      }
    },
    "required_providers": {
      "aws": {
        "source": "hashicorp/aws",
        "version": "6.71.0"
      }
    },
    "required_version": "1.13.2"
  }
}
```

- [ ] **Step 6: Run them to see them pass, and commit**

Run: `go test ./internal/render/ ./internal/cli/ -count=1`
Expected: `ok` twice. Then:
- `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing);
- the render validation snippet prints its six `Success!` lines;
- the naming check prints nothing.

```bash
git add internal/render/render.go internal/render/render_test.go internal/cli/claims_test.go internal/render/testdata/aws-registry/expected/aws
git commit -m "feat(render): render the aws stacks of every environment and registry component"
```

- [ ] **Step 7: Mutation checks** (nothing is committed; restore each with `git checkout internal/render/render.go`, then `git status --short` prints nothing)
1. In `awsStacks`, replace `naming.RoleName(c.Name, env)` with `naming.RoleName(c.Name, "dev")`. Run `go test ./internal/render/ -count=1 -run TestRenderAWSStacksUseNamingsIdentities`. Expected: `FAIL` with `pro module = map[component:map[...role_name:api-dev-ci...]], want only component = ...` and `pro: the stack creates the role arn:aws:iam::000000000003:role/api-dev-ci, the github stack passes AWS_ROLE_ARN arn:aws:iam::000000000003:role/api-pro-ci`.
2. Replace `		for _, env := range c.Environments {` with `		for _, env := range []string{"dev", "pro", "staging"} {`. Run `go test ./internal/render/ -count=1 -run TestRenderAWSStacksFollowTheAWSBlocks`. Expected: `FAIL` with `stacks = [... aws/staging/_baseline aws/staging/components/api github], want [... aws/staging/_baseline github]`.
3. In `awsStack`, replace `"provider": map[string]any{"aws": map[string]any{"region": literal(aws.Region)}},` with `"provider": map[string]any{"aws": map[string]any{"region": literal(aws.Region), "skip_region_validation": true}},`. Run `go test ./internal/render/ -count=1 -run TestRenderAWSStacksAreThin`. Expected: `FAIL` with `aws/dev/_baseline: provider = map[aws:map[region:eu-west-1 skip_region_validation:true]], want only the region` and `aws/dev/_baseline mentions "skip_"`.

---

### Task 4: Renovate and the AWS provider version contract

**Files:**
- Modify: `internal/workflowspec/renovate_test.go`, `.github/renovate-global.json`, `docs/runbooks/dependencies.md`

**Interfaces:**
- Consumes: `render.AWSProviderVersion` and the four golden AWS stacks (Task 3); `moduleDirs`, `lockedVersion`, `repoFiles`, `loadRenovateConfig` (`internal/workflowspec`).
- Produces: the custom manager `aws`; the `opentofu` manager over every golden stack (D-5).

- [ ] **Step 1: Write the failing tests** — `internal/workflowspec/renovate_test.go` (Edit).

1. `TestRenovateCustomManagers`, its `want` list, old:

```go
		{"opentofu", "github-releases", "opentofu/opentofu", ""},
		{"github", "terraform-provider", "integrations/github", "https://registry.opentofu.org"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("custom managers = %v, want %v", got, want)
	}
}
```

new:

```go
		{"opentofu", "github-releases", "opentofu/opentofu", ""},
		{"github", "terraform-provider", "integrations/github", "https://registry.opentofu.org"},
		{"aws", "terraform-provider", "hashicorp/aws", "https://registry.opentofu.org"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("custom managers = %v, want %v", got, want)
	}
}
```

2. `TestRenovateCustomManagersFindThePins`, the golden files, old:

```go
	ci, rn := ".github/workflows/ci.yaml", renovatePath
	// Every golden render carries both pins, so Renovate must move them all.
	awsGolden := "internal/render/testdata/aws-registry/expected/github/main.tf.json"
	golden := "internal/render/testdata/basic/expected/github/main.tf.json"
```

new:

```go
	ci, rn := ".github/workflows/ci.yaml", renovatePath
	// Every golden github stack carries the OpenTofu and GitHub provider pins,
	// and every golden AWS stack the OpenTofu and AWS provider pins, so Renovate
	// must move them all. The list is exact: a new golden stack is added here.
	awsGolden := "internal/render/testdata/aws-registry/expected/github/main.tf.json"
	golden := "internal/render/testdata/basic/expected/github/main.tf.json"
	var awsStacks []string
	for _, s := range []string{"dev/_baseline", "dev/components/api", "pro/_baseline", "pro/components/api"} {
		awsStacks = append(awsStacks, "internal/render/testdata/aws-registry/expected/aws/"+s+"/main.tf.json")
	}
	pins := func(version string, files ...string) []string {
		var out []string
		for _, f := range files {
			out = append(out, f+"="+version)
		}
		return slices.Sorted(slices.Values(out))
	}
```

3. `TestRenovateCustomManagersFindThePins`, the expected pins, old:

```go
		"opentofu":                    {"internal/render/render.go=" + render.TofuVersion, awsGolden + "=" + render.TofuVersion, golden + "=" + render.TofuVersion},
		"github":                      {"internal/render/render.go=" + render.GitHubProviderVersion, awsGolden + "=" + render.GitHubProviderVersion, golden + "=" + render.GitHubProviderVersion},
	}
```

new:

```go
		"opentofu":                    pins(render.TofuVersion, append([]string{"internal/render/render.go", awsGolden, golden}, awsStacks...)...),
		"github":                      pins(render.GitHubProviderVersion, "internal/render/render.go", awsGolden, golden),
		"aws":                         pins(render.AWSProviderVersion, append([]string{"internal/render/render.go"}, awsStacks...)...),
	}
```

4. `TestOneOpenTofuAndProviderVersion`, old:

```go
func TestOneOpenTofuAndProviderVersion(t *testing.T) {
	if got := ciTofuVersion(t); got != render.TofuVersion {
		t.Errorf("CI tests with OpenTofu %s, the renderer requires %s: the opentofu group moves them together, so they must start equal", got, render.TofuVersion)
	}
	for _, m := range []string{"group", "component"} {
		b, err := os.ReadFile(filepath.Join(repoRoot, "modules", "github", m, "versions.tf"))
		if err != nil {
			t.Fatal(err)
		}
		// Anchored to the line start, so required_version = ">= 1.12.0" does not match.
		sub := regexp.MustCompile(`(?m)^[ \t]*version[ \t]*=[ \t]*"([^"]+)"`).FindSubmatch(b)
		if sub == nil || string(sub[1]) != render.GitHubProviderVersion {
			t.Errorf("modules/github/%s pins the provider at %q, the renderer at %s", m, sub, render.GitHubProviderVersion)
		}
	}
}
```

new:

```go
// TestOneOpenTofuAndProviderVersion: CI's OpenTofu is the renderer's, and every
// module pins its provider at the version the renderer writes into the stacks
// that call it, so one Renovate group moves them together.
func TestOneOpenTofuAndProviderVersion(t *testing.T) {
	if got := ciTofuVersion(t); got != render.TofuVersion {
		t.Errorf("CI tests with OpenTofu %s, the renderer requires %s: the opentofu group moves them together, so they must start equal", got, render.TofuVersion)
	}
	rendered := map[string]string{"github": render.GitHubProviderVersion, "aws": render.AWSProviderVersion}
	modules := moduleDirs(t)
	if len(modules) == 0 {
		t.Fatal("no modules/*/*/versions.tf")
	}
	for _, m := range modules {
		provider := strings.SplitN(m, "/", 2)[0]
		want, ok := rendered[provider]
		if !ok {
			t.Errorf("modules/%s: the renderer pins no %s provider version", m, provider)
			continue
		}
		b, err := os.ReadFile(filepath.Join(repoRoot, "modules", filepath.FromSlash(m), "versions.tf"))
		if err != nil {
			t.Fatal(err)
		}
		// Anchored to the line start, so required_version = ">= 1.12.0" does not match.
		if sub := lockedVersion.FindSubmatch(b); sub == nil || string(sub[1]) != want {
			t.Errorf("modules/%s pins the provider at %q, the renderer at %s", m, sub, want)
		}
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/ -count=1 -run 'TestRenovate|TestOneOpenTofu'`
Expected: `--- FAIL: TestRenovateCustomManagers` (the `want` list ends with `{aws terraform-provider hashicorp/aws https://registry.opentofu.org}`) and `--- FAIL: TestRenovateCustomManagersFindThePins` (no `aws` entry, and `opentofu` lacks the four golden AWS stacks). `TestOneOpenTofuAndProviderVersion` passes: the pins already agree, and Step 5 shows what it catches.

- [ ] **Step 3: Implement** — `.github/renovate-global.json` (Edit).

1. The "opentofu" rule's description, old:

```json
      "description": "One OpenTofu version: setup-idp, ci.yaml, the renderer's required_version and its golden render move together. The release tags carry a v that every pin leaves out.",
```

new:

```json
      "description": "One OpenTofu version: setup-idp, ci.yaml, the renderer's required_version and its golden renders move together. The release tags carry a v that every pin leaves out.",
```

2. The "aws provider" rule's description, old:

```json
      "description": "The AWS provider: the pins of modules/aws/* move together; the lock file is regenerated by hand (docs/runbooks/dependencies.md).",
```

new:

```json
      "description": "The AWS provider: the pins of modules/aws/*, the renderer's pin and its golden AWS stacks move together; the lock file is regenerated by hand (docs/runbooks/dependencies.md).",
```

3. The `opentofu` manager's files, old:

```json
      "description": "OpenTofu in the renderer's required_version and its golden render; setup-opentofu's tofu_version is found by the github-actions manager.",
      "managerFilePatterns": [
        "/^internal/render/render\\.go$/",
        "/^internal/render/testdata/[^/]+/expected/github/main\\.tf\\.json$/"
      ],
```

new:

```json
      "description": "OpenTofu in the renderer's required_version and its golden renders, every stack of them; setup-opentofu's tofu_version is found by the github-actions manager.",
      "managerFilePatterns": [
        "/^internal/render/render\\.go$/",
        "/^internal/render/testdata/[^/]+/expected/.+/main\\.tf\\.json$/"
      ],
```

4. The new `aws` manager, after the `github` one, old:

```json
      "datasourceTemplate": "terraform-provider",
      "depNameTemplate": "github",
      "packageNameTemplate": "integrations/github",
      "registryUrlTemplate": "https://registry.opentofu.org"
    }
  ]
}
```

new:

```json
      "datasourceTemplate": "terraform-provider",
      "depNameTemplate": "github",
      "packageNameTemplate": "integrations/github",
      "registryUrlTemplate": "https://registry.opentofu.org"
    },
    {
      "customType": "regex",
      "description": "The AWS provider in the renderer and its golden AWS stacks; the module pins are found by the terraform manager.",
      "managerFilePatterns": [
        "/^internal/render/render\\.go$/",
        "/^internal/render/testdata/[^/]+/expected/aws/.+/main\\.tf\\.json$/"
      ],
      "matchStrings": [
        "AWSProviderVersion\\s+=\\s+\"(?<currentValue>\\d+\\.\\d+\\.\\d+)\"",
        "\"source\": \"hashicorp/aws\",\\s+\"version\": \"(?<currentValue>\\d+\\.\\d+\\.\\d+)\""
      ],
      "datasourceTemplate": "terraform-provider",
      "depNameTemplate": "aws",
      "packageNameTemplate": "hashicorp/aws",
      "registryUrlTemplate": "https://registry.opentofu.org"
    }
  ]
}
```

Run: `go test ./internal/workflowspec/ -count=1`
Expected: `ok`.

- [ ] **Step 4: Update the runbook** — `docs/runbooks/dependencies.md` (Edit).

1. The table row, old:

```markdown
| The AWS provider | `modules/aws/*/versions.tf` | group "aws provider", label `needs-lockfile` |
```

new:

```markdown
| The AWS provider | `modules/aws/*/versions.tf`, `AWSProviderVersion` in `render.go` and the golden AWS stacks | group "aws provider", label `needs-lockfile` |
```

2. "An AWS provider update", the first paragraph, old:

```markdown
Renovate moves the pins in `modules/aws/*/versions.tf` but not `lockfiles/aws.terraform.lock.hcl`, so the PR's `go` job fails (`TestModulesPinTheirLockedProvider`), and so do `modules (aws/baseline)`, `modules (aws/component)` and `floci` (`tofu init -lockfile=readonly`). Regenerate the lock the same way, from the component module:
```

new:

```markdown
Renovate moves the pins in `modules/aws/*/versions.tf`, `AWSProviderVersion` and the golden AWS stacks, but not `lockfiles/aws.terraform.lock.hcl`, so the PR's `go` job fails (`TestModulesPinTheirLockedProvider`, `TestRenderCopiesTheLockFile`), and so do `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke` and `floci` (`tofu init -lockfile=readonly`). Regenerate the lock the same way, from the component module:
```

3. "An AWS provider update", the last paragraph, old:

```markdown
Then set the new version in `lockfiles/lockfiles_test.go` (`TestAWSLockPinsTheProvider`), run `go test ./lockfiles/ ./internal/workflowspec/ ./internal/floci/`, commit as `chore(deps): regenerate the aws provider lock file` and push. The registry signs the checksums of every package of a release, and OpenTofu records them all whatever the `-platform` flags say; the flags keep one command for both providers.
```

new:

```markdown
Then set the new version in `lockfiles/lockfiles_test.go` (`TestAWSLockPinsTheProvider`), run `go test ./lockfiles/ ./internal/render/ ./internal/workflowspec/ ./internal/floci/`, commit as `chore(deps): regenerate the aws provider lock file` and push. The registry signs the checksums of every package of a release, and OpenTofu records them all whatever the `-platform` flags say; the flags keep one command for both providers.
```

- [ ] **Step 5: Verify and commit** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing); `python -I -c "import json;json.load(open('.github/renovate-global.json'))"` prints nothing; the naming check prints nothing.

```bash
git add internal/workflowspec/renovate_test.go .github/renovate-global.json docs/runbooks/dependencies.md
git commit -m "chore(renovate): move the renderer's aws provider pin and golden stacks with the modules"
```

- [ ] **Step 6: Mutation check** (nothing is committed) — in `internal/render/render.go`, replace `AWSProviderVersion    = "6.71.0"` with `AWSProviderVersion    = "6.70.0"`. Run `go test ./internal/workflowspec/ ./internal/render/ -count=1`. Expected: `FAIL` in both packages.
- `internal/workflowspec`: `TestRenovateCustomManagersFindThePins` and `TestOneOpenTofuAndProviderVersion` (`modules/aws/baseline pins the provider at ... the renderer at 6.70.0`).
- `internal/render`: `TestRenderAWSStacksGolden` and `TestRenderCopiesTheLockFile` (`the AWS lock file does not pin AWSProviderVersion 6.70.0`).

Restore with `git checkout internal/render/render.go`; `git status --short` prints nothing.

---

### Task 5: Stack discovery — `idp diff` on the nested AWS stacks

**Files:**
- Modify: `internal/wetdiff/wetdiff_test.go`, `internal/wetdiff/wetdiff.go`, `internal/cli/diff_test.go`

**Interfaces:**
- Consumes: `wetdiff.Diff`, `render.Render`, `render.WriteTree`, `claims.Load` (test-only imports in `internal/wetdiff`; `render` does not import `wetdiff`, so no cycle); the CLI helpers `Run`, `writeFileAll`, `envOf`, `noEnv`.
- Produces: tests only, and `readStacks`'s documented rules (D-9). No behavior change.

- [ ] **Step 1: Write the tests** — `internal/wetdiff/wetdiff_test.go` (Edit).

1. Imports, old:

```go
import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)
```

new:

```go
import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
)
```

2. Before `TestDiffNewRootMissingIsAnError`, old:

```go
func TestDiffNewRootMissingIsAnError(t *testing.T) {
```

new:

```go
// TestDiffNestedStackOwnsItsFiles: a file belongs to the deepest stack that
// contains it, so a change inside a stack nested in another changes only the
// nested one.
func TestDiffNestedStackOwnsItsFiles(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, wet, map[string]string{"aws/dev/main.tf.json": "{}", "aws/dev/_baseline/main.tf.json": `{"v":1}`})
	write(t, fresh, map[string]string{"aws/dev/main.tf.json": "{}", "aws/dev/_baseline/main.tf.json": `{"v":2}`})
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{
		Stacks:   []Stack{{Path: "aws/dev", Status: Unchanged}, {Path: "aws/dev/_baseline", Status: Changed}},
		Affected: []string{"aws/dev/_baseline"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff =\n%+v\nwant\n%+v", got, want)
	}
}

// TestDiffRootIsNeverAStack: a main.tf.json directly under a root is ignored,
// like the render marker; only directories below the root are stacks.
func TestDiffRootIsNeverAStack(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, wet, map[string]string{"main.tf.json": `{"v":1}`, "github/main.tf.json": "{}"})
	write(t, fresh, map[string]string{"main.tf.json": `{"v":2}`, "github/main.tf.json": "{}"})
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{Stacks: []Stack{{Path: "github", Status: Unchanged}}, Affected: []string{}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff = %+v, want %+v", got, want)
	}
}

// TestDiffRejectsSymlinks: a render holds only directories and regular files,
// so a symbolic link in either tree is an error, never followed.
func TestDiffRejectsSymlinks(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, fresh, map[string]string{"github/main.tf.json": "{}"})
	write(t, wet, map[string]string{"github/main.tf.json": "{}"})
	if err := os.Symlink(filepath.Join(fresh, "github", "main.tf.json"), filepath.Join(wet, "github", "link.json")); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
	_, err := Diff(fresh, wet, false)
	if want := "wet render: github/link.json: not a regular file or directory"; err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

// TestDiffSeesTheRenderedAWSStacks diffs two real renders (spec §5.4): the
// claims after an environment, staging, and a registry Component, web, were
// removed, against the render that still had them. Every AWS stack is found
// at its nested path, and the removed ones are orphans.
func TestDiffSeesTheRenderedAWSStacks(t *testing.T) {
	m, diags, err := claims.Load("../render/testdata/aws-registry")
	if err != nil || len(diags) > 0 {
		t.Fatalf("load: %v %v", err, diags)
	}
	current, err := render.Render(m, render.Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	old := *m
	old.Platform.Environments = map[string]claims.Environment{"staging": {AWS: &claims.AWS{AccountID: "000000000002", Region: "eu-west-1"}}}
	for name, env := range m.Platform.Environments {
		old.Platform.Environments[name] = env
	}
	api, web := m.Components[0], m.Components[0]
	api.Environments = []string{"dev", "pro", "staging"}
	web.Name, web.Environments = "web", []string{"dev"}
	old.Components = []claims.Component{api, web}
	previous, err := render.Render(&old, render.Options{ModuleRef: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	if err := render.WriteTree(fresh, current); err != nil {
		t.Fatal(err)
	}
	if err := render.WriteTree(wet, previous); err != nil {
		t.Fatal(err)
	}
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{
		Stacks: []Stack{
			{Path: "aws/dev/_baseline", Status: Unchanged},
			{Path: "aws/dev/components/api", Status: Unchanged},
			{Path: "aws/dev/components/web", Status: Orphan},
			{Path: "aws/pro/_baseline", Status: Unchanged},
			{Path: "aws/pro/components/api", Status: Unchanged},
			{Path: "aws/staging/_baseline", Status: Orphan},
			{Path: "aws/staging/components/api", Status: Orphan},
			{Path: "github", Status: Changed},
		},
		Affected: []string{"aws/dev/components/web", "aws/staging/_baseline", "aws/staging/components/api", "github"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDiffNewRootMissingIsAnError(t *testing.T) {
```

Then `internal/cli/diff_test.go` (Edit), before `TestDiffRequiresARenderMarker`, old:

```go
func TestDiffRequiresARenderMarker(t *testing.T) {
```

new:

```go
// TestDiffNamesTheNestedAWSStacks: idp diff prints each AWS stack by its slash
// path under rendered/ and writes the affected ones to GITHUB_OUTPUT, an
// orphan environment included.
func TestDiffNamesTheNestedAWSStacks(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join("..", "render", "testdata", "aws-registry")
	for _, out := range []string{"new", "wet"} {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"render", "--dir", fixture, "--out", filepath.Join(dir, out), "--module-ref", "v1"}, &stdout, &stderr, noEnv); code != 0 {
			t.Fatalf("render %s: exit %d, stderr %q", out, code, stderr.String())
		}
	}
	writeFileAll(t, filepath.Join(dir, "wet", "aws", "staging", "_baseline", "main.tf.json"), "{}")
	writeFileAll(t, filepath.Join(dir, "new", "aws", "dev", "components", "api", "extra.tf.json"), "{}")
	ghOut := filepath.Join(dir, "github-output")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"diff", "--new", filepath.Join(dir, "new"), "--wet", filepath.Join(dir, "wet")}, &stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": ghOut}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	want := "aws/dev/_baseline: unchanged\n" +
		"aws/dev/components/api: changed\n" +
		"aws/pro/_baseline: unchanged\n" +
		"aws/pro/components/api: unchanged\n" +
		"aws/staging/_baseline: orphan\n" +
		"github: unchanged\n" +
		"diff: 2 affected stack(s)\n"
	if stdout.String() != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout.String(), want)
	}
	got, err := os.ReadFile(ghOut)
	if err != nil {
		t.Fatal(err)
	}
	if want := "affected=[\"aws/dev/components/api\",\"aws/staging/_baseline\"]\n"; string(got) != want {
		t.Errorf("GITHUB_OUTPUT = %q, want %q", got, want)
	}
}

func TestDiffRequiresARenderMarker(t *testing.T) {
```

- [ ] **Step 2: Run them** — they pin existing behavior, which Task 3's render now exercises, so they pass before any change.

Run: `go test ./internal/wetdiff/ ./internal/cli/ -count=1 -v -run 'TestDiff'`
Expected: `PASS` for every `TestDiff*`, `TestDiffRejectsSymlinks` included. A `SKIP` with `cannot create a symbolic link here` is acceptable on a machine that refuses links; CI's Linux runner runs it. Then `ok` twice.

- [ ] **Step 3: Document the rules** — `internal/wetdiff/wetdiff.go` (Edit), old:

```go
// readStacks maps every stack directory under root (slash path) to its files.
// .terraform directories are skipped; files outside any stack (the render
// marker) are ignored. A missing root returns an error wrapping fs.ErrNotExist.
```

new:

```go
// readStacks maps every stack directory under root (slash path) to its files.
// A stack is a directory below root that holds StackFile, at any depth: github,
// or aws/dev/components/api. root itself is never a stack, so a StackFile
// directly under root is ignored, like every file outside a stack (the render
// marker). A file belongs to the deepest stack that contains it, so a stack
// nested in another owns its own files. .terraform directories are skipped, and
// anything but a directory or a regular file, a symbolic link included, is an
// error. A missing root returns an error wrapping fs.ErrNotExist.
```

- [ ] **Step 4: Verify and commit** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

```bash
git add internal/wetdiff/wetdiff.go internal/wetdiff/wetdiff_test.go internal/cli/diff_test.go
git commit -m "test(wetdiff): pin nested stack discovery on the rendered aws stacks"
```

- [ ] **Step 5: Mutation checks** (nothing is committed; restore each with `git checkout internal/wetdiff/wetdiff.go`, then `git status --short` prints nothing). Run `go test ./internal/wetdiff/ -count=1` after each.
1. In `ownerStack`, replace `len(d) > len(owner)` with `owner == ""`. Expected: `--- FAIL: TestDiffNestedStackOwnsItsFiles`.
2. In `readStacks`, replace ` && rel != "."` with nothing. Expected: `--- FAIL: TestDiffRootIsNeverAStack`.
3. In `readStacks`, replace `return fmt.Errorf("%s: not a regular file or directory", rel)` with `return nil`. Expected: `--- FAIL: TestDiffRejectsSymlinks` (where links can be created).

---

### Task 6: Level 3 on the real render, and `wantMissing`'s offline test

**Files:**
- Modify: `internal/floci/stack_test.go`, `internal/floci/fixture_test.go` (rewritten), `internal/floci/floci_test.go`, `internal/floci/doc.go`
- Create: `internal/floci/wantmissing_test.go`

**Interfaces:**
- Consumes:
  - `claims.Load`, `render.Render`, `render.Options`, `render.WriteTree`;
  - the `aws-registry` fixture and its rendered AWS stacks (Task 3);
  - `modules/aws` and `lockfiles.AWS`.
- Produces:
  - `fixtureDir`, `stacks(tg target) []string`, `renderFixture(root string) error`, `writeOverride(dir, endpoint string) error`;
  - `(*floci).apply(t, tg, stack string)`;
  - `awsCLI` and `wantMissing` on `testing.TB`;
  - `TestMain`, `recorder`, and the tests of D-7 and D-11.
- Removes: `writeStack`, `componentInputs`, `TestWriteStack` (D-7).

- [ ] **Step 1: Write the failing tests**

1. `internal/floci/stack_test.go` (Edit): the imports and `TestWriteStack` become the imports and three new tests, old:

```go
import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

func TestWriteStack(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dev", "component")
	tg := targets[0]
	if err := writeStack(dir, "component", tg, "http://localhost:4566", componentInputs(tg)); err != nil {
		t.Fatal(err)
	}
	read := func(name string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return v
	}
	wantMain := map[string]any{
		"module": map[string]any{"component": map[string]any{
			"source":            "../../modules/aws/component",
			"github_repository": "acme/api",
			"environment":       "dev",
			"repository_name":   "api",
			"role_name":         "api-dev-ci",
			"oidc_provider_arn": "arn:aws:iam::000000000001:oidc-provider/token.actions.githubusercontent.com",
		}},
		"provider": map[string]any{"aws": map[string]any{"region": "eu-west-1"}},
	}
	if got := read("main.tf.json"); !reflect.DeepEqual(got, wantMain) {
		t.Errorf("main.tf.json = %v, want %v", got, wantMain)
	}
	wantOverride := map[string]any{"provider": map[string]any{"aws": map[string]any{
		"endpoints":                   map[string]any{"ecr": "http://localhost:4566", "iam": "http://localhost:4566", "sts": "http://localhost:4566"},
		"skip_credentials_validation": true,
		"skip_metadata_api_check":     "true",
		"skip_requesting_account_id":  true,
	}}}
	if got := read("floci_override.tf.json"); !reflect.DeepEqual(got, wantOverride) {
		t.Errorf("floci_override.tf.json = %v, want %v", got, wantOverride)
	}
	lock, err := os.ReadFile(filepath.Join(dir, ".terraform.lock.hcl"))
	if err != nil || !bytes.Equal(lock, lockfiles.AWS) {
		t.Errorf("the stack must carry the engine's AWS lock file (err %v)", err)
	}
}
```

new:

```go
import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

// TestTheFixtureIsTheTestPlatform keeps TestFloci's written-out expectations
// equal to the fixture it renders: the org, the one registry Component, and
// each environment's account and region.
func TestTheFixtureIsTheTestPlatform(t *testing.T) {
	m, diags, err := claims.Load(fixtureDir)
	if err != nil || len(diags) > 0 {
		t.Fatalf("load %s: %v %v", fixtureDir, err, diags)
	}
	if m.Platform.GitHub.Org != org {
		t.Errorf("org = %q, want %q", m.Platform.GitHub.Org, org)
	}
	var envs []string
	for _, tg := range targets {
		envs = append(envs, tg.env)
		if got := m.Platform.Environments[tg.env].AWS; got == nil || *got != (claims.AWS{AccountID: tg.accountID, Region: tg.region}) {
			t.Errorf("environment %s: aws = %+v, want account %s in %s", tg.env, got, tg.accountID, tg.region)
		}
	}
	if len(m.Platform.Environments) != len(targets) {
		t.Errorf("the fixture has %d environments, TestFloci applies %d", len(m.Platform.Environments), len(targets))
	}
	if len(m.Components) != 1 || m.Components[0].Name != component || !m.Components[0].AWS.Registry || !slices.Equal(m.Components[0].Environments, envs) {
		t.Errorf("components = %+v, want only %s, with aws.registry, in %v", m.Components, component, envs)
	}
}

// TestRenderFixture: the render TestFloci applies holds exactly the AWS stacks
// it applies, each with the engine's AWS lock file, a module source in the
// modules it copies, and nothing of floci's, which writeOverride adds.
func TestRenderFixture(t *testing.T) {
	root := t.TempDir()
	if err := renderFixture(root); err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, tg := range targets {
		want = append(want, stacks(tg)...)
	}
	slices.Sort(want)
	rendered := filepath.Join(root, "rendered")
	var got []string
	err := filepath.WalkDir(filepath.Join(rendered, "aws"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Name() != "main.tf.json" {
			return err
		}
		dir := filepath.Dir(p)
		rel, err := filepath.Rel(rendered, dir)
		if err != nil {
			return err
		}
		got = append(got, filepath.ToSlash(rel))
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if bytes.Contains(b, []byte("floci")) || bytes.Contains(b, []byte("endpoint")) {
			t.Errorf("%s: the render mentions the emulator (spec §5.6)", rel)
		}
		var doc struct {
			Module map[string]struct {
				Source string `json:"source"`
			} `json:"module"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			return err
		}
		for name, call := range doc.Module {
			want := filepath.Join(root, "modules", "aws", name)
			if src := filepath.Join(dir, filepath.FromSlash(call.Source)); src != want {
				t.Errorf("%s: module %s resolves to %s, want the copied modules' %s", rel, name, src, want)
			}
		}
		lock, err := os.ReadFile(filepath.Join(dir, ".terraform.lock.hcl"))
		if err != nil || !bytes.Equal(lock, lockfiles.AWS) {
			t.Errorf("%s must carry the engine's AWS lock file (err %v)", rel, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("rendered AWS stacks = %v, TestFloci applies %v", got, want)
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
```

2. Create `internal/floci/wantmissing_test.go` (Write):

```go
package floci

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// fakeAWSExit makes this test binary act as the AWS CLI: when it is set, the
// binary writes fakeAWSStderr to stderr and exits with fakeAWSExit, before any
// test runs.
const (
	fakeAWSExit   = "IDP_FLOCI_FAKE_AWS_EXIT"
	fakeAWSStderr = "IDP_FLOCI_FAKE_AWS_STDERR"
)

func TestMain(m *testing.M) {
	if code, ok := os.LookupEnv(fakeAWSExit); ok {
		fmt.Fprint(os.Stderr, os.Getenv(fakeAWSStderr))
		n, err := strconv.Atoi(code)
		if err != nil {
			n = 125
		}
		os.Exit(n)
	}
	os.Exit(m.Run())
}

// recorder keeps the errors a check reports instead of failing the test.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

// TestWantMissing runs wantMissing against a stub AWS CLI, offline: only a
// failure that names NoSuchEntity passes, so a found resource and an
// unreachable or broken floci both fail (Phase 2c, R3-001).
func TestWantMissing(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f := &floci{endpoint: "http://localhost:4566", aws: exe, root: t.TempDir()}
	for _, tt := range []struct{ name, exit, stderr, want string }{
		{"missing", "254", "An error occurred (NoSuchEntity) when calling the GetRole operation: Role not found", ""},
		{"found", "0", "", "role: found, want NoSuchEntity"},
		{"unreachable", "255", "Could not connect to the endpoint URL", "role: aws exited 255 without NoSuchEntity:\nCould not connect to the endpoint URL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(fakeAWSExit, tt.exit)
			t.Setenv(fakeAWSStderr, tt.stderr)
			r := &recorder{TB: t}
			f.wantMissing(r, targets[0], "role", "iam", "get-role", "--role-name", "api-dev-ci")
			if got := strings.Join(r.errors, "\n"); got != tt.want {
				t.Errorf("wantMissing reported %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/floci/ -count=1`
Expected: build failure:
- `undefined: fixtureDir`, `undefined: renderFixture`, `undefined: stacks` and `undefined: writeOverride`;
- `cannot use r (variable of type *recorder) as *testing.T value in argument to f.wantMissing`.

- [ ] **Step 3: Implement**

1. Replace `internal/floci/fixture_test.go` (Write) with:

```go
package floci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
)

// fixtureDir is the claims repo whose render TestFloci applies: the renderer's
// aws-registry fixture, whose golden files pin the same stacks.
const fixtureDir = "../render/testdata/aws-registry"

// modulesDir is the engine's AWS modules, seen from this package's directory.
const modulesDir = "../../modules/aws"

// target is one environment of the test platform: its account and region.
type target struct {
	env, accountID, region string
}

// The test platform, written out so that TestFloci's checks never take their
// expectations from the render they check: one registry Component, api of the
// GitHub org acme, in two environments in two accounts of one floci
// (ADR-0004). TestTheFixtureIsTheTestPlatform keeps it equal to the fixture.
const (
	org       = "acme"
	component = "api"
)

var targets = []target{
	{env: "dev", accountID: "000000000001", region: "eu-west-1"},
	{env: "pro", accountID: "000000000003", region: "eu-west-1"},
}

// githubRepository is the Component's repository, <org>/<name>.
func githubRepository() string { return org + "/" + component }

// repositoryARN is the ARN AWS gives the Component's repository. naming has no
// function for it: the push policy reads it from the repository resource.
func repositoryARN(tg target) string {
	return "arn:aws:ecr:" + tg.region + ":" + tg.accountID + ":repository/" + naming.RepositoryName(component)
}

// stacks are the rendered AWS stacks of tg's environment, as slash paths under
// rendered/, in apply order: the baseline, whose OIDC provider the Component's
// role trusts, then the Component's stack.
func stacks(tg target) []string {
	return []string{"aws/" + tg.env + "/_baseline", "aws/" + tg.env + "/components/" + component}
}

// renderFixture renders the fixture into root/rendered the way CI's
// render-smoke does, with module sources in root/modules, which the caller
// fills with the engine's modules.
func renderFixture(root string) error {
	m, diags, err := claims.Load(fixtureDir)
	if err != nil {
		return err
	}
	if len(diags) > 0 {
		return fmt.Errorf("%s: %v", fixtureDir, diags)
	}
	files, err := render.Render(m, render.Options{ModulesDir: "../../modules"})
	if err != nil {
		return err
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

2. `internal/floci/floci_test.go` (Edit). `awsCLI`, old:

```go
// awsCLI runs an AWS CLI command against floci as the account of tg and
// returns its stdout and stderr apart, so a warning on stderr never reaches
// the JSON decoder, and its exit code. A command that cannot start fails the
// test.
func (f *floci) awsCLI(t *testing.T, tg target, args ...string) (stdout, stderr string, code int) {
```

new:

```go
// awsCLI runs an AWS CLI command against floci as the account of tg and
// returns its stdout and stderr apart, so a warning on stderr never reaches
// the JSON decoder, and its exit code. A command that cannot start fails the
// test.
func (f *floci) awsCLI(t testing.TB, tg target, args ...string) (stdout, stderr string, code int) {
```

3. `wantMissing`, old:

```go
// wantMissing runs an AWS CLI read that must fail with NoSuchEntity. A success,
// or a failure for any other reason, fails the test, so an unreachable or
// broken floci never passes for a missing resource.
func (f *floci) wantMissing(t *testing.T, tg target, what string, args ...string) {
```

new:

```go
// wantMissing runs an AWS CLI read that must fail with NoSuchEntity. A success,
// or a failure for any other reason, fails the test, so an unreachable or
// broken floci never passes for a missing resource (TestWantMissing).
func (f *floci) wantMissing(t testing.TB, tg target, what string, args ...string) {
```

4. `apply`, and `TestFloci`'s comment, old:

```go
// apply creates one stack of tg's environment the way the pipelines will (spec
// §6.1): init with the shipped lock file, apply, then replay, a plan of the
// same version that must find nothing to change (spec §8.4). It registers the
// stack's destroy before applying, so whatever an apply created is destroyed,
// even when the test fails.
func (f *floci) apply(t *testing.T, tg target, module string, inputs map[string]any) {
	t.Helper()
	dir := filepath.Join(f.root, tg.env, module)
	if err := writeStack(dir, module, tg, f.endpoint, inputs); err != nil {
		t.Fatal(err)
	}
	f.tofuOK(t, tg, dir, "init", "-input=false", "-lockfile=readonly", "-no-color")
	t.Cleanup(func() {
		if out, code := f.run(t, tg, dir, f.tofu, "destroy", "-input=false", "-auto-approve", "-no-color"); code != 0 {
			t.Errorf("%s %s: destroy exited %d:\n%s", tg.env, module, code, out)
		}
	})
	f.tofuOK(t, tg, dir, "apply", "-input=false", "-auto-approve", "-no-color")
	if out, code := f.run(t, tg, dir, f.tofu, "plan", "-input=false", "-detailed-exitcode", "-no-color"); code != 0 {
		t.Errorf("%s %s: the replay plan exited %d, want 0 (no changes):\n%s", tg.env, module, code, out)
	}
}

// TestFloci is the level-3 test of the AWS modules (spec §8.4). Against one
// floci it applies, as each account, the environment's baseline and then its
// Component stack, with the inputs internal/naming computes. It checks through
// the AWS API that the created identities are naming's (ADR-0006), that the
// trust and push policies are exactly spec §4.4's, and that one account cannot
// see another's role (ADR-0004). It destroys everything and checks that
// nothing is left (spec §6.3).
```

new:

```go
// apply creates one rendered stack of tg's environment, a slash path under
// rendered/, the way the pipelines will (spec §5.6, §6.1): it adds
// floci_override.tf.json, inits with the rendered lock file, applies, then
// replays, a plan of the same version that must find nothing to change (spec
// §8.4). It registers the stack's destroy before applying, so whatever an apply
// created is destroyed, even when the test fails.
func (f *floci) apply(t *testing.T, tg target, stack string) {
	t.Helper()
	dir := filepath.Join(f.root, "rendered", filepath.FromSlash(stack))
	if err := writeOverride(dir, f.endpoint); err != nil {
		t.Fatal(err)
	}
	f.tofuOK(t, tg, dir, "init", "-input=false", "-lockfile=readonly", "-no-color")
	t.Cleanup(func() {
		if out, code := f.run(t, tg, dir, f.tofu, "destroy", "-input=false", "-auto-approve", "-no-color"); code != 0 {
			t.Errorf("%s: destroy exited %d:\n%s", stack, code, out)
		}
	})
	f.tofuOK(t, tg, dir, "apply", "-input=false", "-auto-approve", "-no-color")
	if out, code := f.run(t, tg, dir, f.tofu, "plan", "-input=false", "-detailed-exitcode", "-no-color"); code != 0 {
		t.Errorf("%s: the replay plan exited %d, want 0 (no changes):\n%s", stack, code, out)
	}
}

// TestFloci is the level-3 test of the AWS modules and the AWS stacks that
// render them (spec §8.4). It renders the aws-registry fixture with local
// module sources and, against one floci, applies as each account the
// environment's rendered baseline and then its rendered Component stack, each
// with only floci_override.tf.json added. It checks through the AWS API that
// the created identities are naming's (ADR-0006), that the trust and push
// policies are exactly spec §4.4's, and that one account cannot see another's
// role (ADR-0004). It destroys everything and checks that nothing is left
// (spec §6.3).
```

5. `TestFloci`, the render and the applies, old:

```go
	if err := os.CopyFS(filepath.Join(f.root, "modules", "aws"), os.DirFS(modulesDir)); err != nil {
		t.Fatal(err)
	}

	// Registered first, so it runs last, after every destroy.
	t.Cleanup(func() {
		for _, tg := range targets {
			f.wantNothingLeft(t, tg)
		}
	})
	for _, tg := range targets {
		f.apply(t, tg, "baseline", map[string]any{})
		f.apply(t, tg, "component", componentInputs(tg))
	}
```

new:

```go
	if err := os.CopyFS(filepath.Join(f.root, "modules", "aws"), os.DirFS(modulesDir)); err != nil {
		t.Fatal(err)
	}
	if err := renderFixture(f.root); err != nil {
		t.Fatal(err)
	}

	// Registered first, so it runs last, after every destroy.
	t.Cleanup(func() {
		for _, tg := range targets {
			f.wantNothingLeft(t, tg)
		}
	})
	for _, tg := range targets {
		for _, stack := range stacks(tg) {
			f.apply(t, tg, stack)
		}
	}
```

6. `internal/floci/doc.go` (Edit), old:

```go
// Package floci holds the level-3 tests of the AWS modules (spec §8.4). CI's
// floci job applies modules/aws/baseline and modules/aws/component to two
// accounts of one floci emulator, with the inputs that internal/naming
// computes, replays them, checks the created identities and policies through
// the AWS API, and destroys them. The package has no code outside its tests,
// and TestFloci skips unless IDP_FLOCI_ENDPOINT is set, so go test ./... stays
// offline.
```

new:

```go
// Package floci holds the level-3 tests of the AWS modules and the AWS stacks
// that render them (spec §8.4). CI's floci job renders the renderer's
// aws-registry fixture, adds floci_override.tf.json to each AWS stack, applies
// the stacks to two accounts of one floci emulator, replays them, checks the
// created identities and policies through the AWS API against internal/naming,
// and destroys them. The package has no code outside its tests, and TestFloci
// skips unless IDP_FLOCI_ENDPOINT is set, so go test ./... stays offline.
```

- [ ] **Step 4: Run them to see them pass, and commit**

Run: `go test ./internal/floci/ -count=1 -v`
Expected:
- `--- SKIP: TestFloci` (no endpoint);
- `PASS` for `TestExpectedPoliciesAreTheModulesOwn`, `TestDecodePolicy`, `TestTheFixtureIsTheTestPlatform`, `TestRenderFixture`, `TestWriteOverride`, `TestModulesUseNamingsIssuer`, and `TestWantMissing` with its subtests `missing`, `found` and `unreachable`;
- `ok`.

`rg -n 'writeStack|componentInputs' internal` prints nothing. Then `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

```bash
git add internal/floci
git commit -m "test(floci): apply the rendered aws stacks and test wantMissing offline"
```

- [ ] **Step 5: Mutation check** (nothing is committed) — in `internal/floci/floci_test.go`, replace `	case !strings.Contains(stderr, "NoSuchEntity"):` with `	case false:`. Run `go test ./internal/floci/ -count=1 -run TestWantMissing`. Expected: `--- FAIL: TestWantMissing/unreachable` with `wantMissing reported "", want "role: aws exited 255 without NoSuchEntity:\nCould not connect to the endpoint URL"`. Restore with `git checkout internal/floci/floci_test.go`; `git status --short` prints nothing.

- [ ] **Step 6: The floci test fails, not skips, without an emulator** — `IDP_FLOCI_ENDPOINT=http://localhost:1 go test ./internal/floci/ -count=1 -run TestFloci`. Expected: `--- FAIL: TestFloci`, not `SKIP`. Without the tools on `PATH` (the planning machine) the message is `tofu is not on PATH`. With them, the fixture renders and the first `apply` fails against the closed port.

---

### Task 7: CI — validate every rendered stack; one `setup-go` in the floci job

**Files:**
- Modify: `internal/workflowspec/ci_test.go`, `.github/workflows/ci.yaml`

**Interfaces:**
- Consumes: `mustJob`, `mustStep`, `wantLines`, `loadWorkflow`, `ciPath` (`internal/workflowspec`); the AWS stacks under `aws/` (Task 3).
- Produces: `render-validate` over every stack with `TF_PLUGIN_CACHE_DIR` (D-8); `TestCIFlociJob`'s single `setup-go` (D-11).

- [ ] **Step 1: Write the failing tests** — `internal/workflowspec/ci_test.go` (Edit).

1. `TestCIRenderSmokeCoversEveryFixture`, its end, old:

```go
	_, s := mustStep(t, "ci render-smoke", mustJob(t, loadWorkflow(t, ciPath), "render-smoke").Steps, "render-validate")
	wantLines(t, "ci render-smoke", s.Run, []string{
		`go run ./cmd/idp render --dir "internal/render/testdata/$fixture" --out "$RUNNER_TEMP/rendered/$fixture" --modules-dir modules`,
		`tofu -chdir="$RUNNER_TEMP/rendered/$fixture/github" init -backend=false -input=false`,
		`tofu -chdir="$RUNNER_TEMP/rendered/$fixture/github" validate`,
	})
}
```

new:

```go
	// Every stack of every render is validated, the AWS stacks under aws/
	// included: each directory with a main.tf.json, found at any depth. The
	// shipped lock file must already hold the stack's provider, and the plugin
	// cache downloads each provider once for all the stacks.
	_, s := mustStep(t, "ci render-smoke", mustJob(t, loadWorkflow(t, ciPath), "render-smoke").Steps, "render-validate")
	if got, want := s.Env["TF_PLUGIN_CACHE_DIR"], "${{ runner.temp }}/tofu-plugin-cache"; got != want {
		t.Errorf("ci render-smoke TF_PLUGIN_CACHE_DIR = %q, want %q", got, want)
	}
	wantLines(t, "ci render-smoke", s.Run, []string{
		`mkdir -p "$TF_PLUGIN_CACHE_DIR"`,
		"shopt -s globstar",
	})
	wantLines(t, "ci render-smoke", s.Run, []string{
		`go run ./cmd/idp render --dir "internal/render/testdata/$fixture" --out "$RUNNER_TEMP/rendered/$fixture" --modules-dir modules`,
		`for stack in "$RUNNER_TEMP/rendered/$fixture"/**/main.tf.json; do`,
		`stack="${stack%/main.tf.json}"`,
		`tofu -chdir="$stack" init -backend=false -input=false -lockfile=readonly`,
		`tofu -chdir="$stack" validate`,
		"done",
		"done",
	})
}
```

2. `TestCIFlociJob`, the step loop, old:

```go
	var tofu []string
	for _, s := range j.Steps {
		if strings.HasPrefix(s.Uses, "opentofu/setup-opentofu@") {
			tofu = append(tofu, s.With["tofu_version"])
		}
		// floci holds the Docker socket, which reaches the runner: its job must
		// not save a Go module cache that other jobs restore.
		if strings.HasPrefix(s.Uses, "actions/setup-go@") && s.With["cache"] != "false" {
			t.Errorf("ci floci setup-go cache = %q, want false", s.With["cache"])
		}
	}
```

new:

```go
	var tofu []string
	setupGo := 0
	for _, s := range j.Steps {
		if strings.HasPrefix(s.Uses, "opentofu/setup-opentofu@") {
			tofu = append(tofu, s.With["tofu_version"])
		}
		// floci holds the Docker socket, which reaches the runner: its job must
		// not save a Go module cache that other jobs restore.
		if strings.HasPrefix(s.Uses, "actions/setup-go@") {
			setupGo++
			if s.With["cache"] != "false" {
				t.Errorf("ci floci setup-go cache = %q, want false", s.With["cache"])
			}
		}
	}
	// Exactly one: without it the job would test with the runner's own Go, and
	// the cache check above would pass with nothing to check (Phase 2c, R3-002).
	if setupGo != 1 {
		t.Errorf("ci floci has %d setup-go steps, want exactly one", setupGo)
	}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/ -count=1 -run 'TestCIRenderSmoke|TestCIFlociJob'`
Expected: `--- FAIL: TestCIRenderSmokeCoversEveryFixture` with `ci render-smoke TF_PLUGIN_CACHE_DIR = "", want "${{ runner.temp }}/tofu-plugin-cache"` and two `the script lacks the consecutive lines`. `TestCIFlociJob` passes: the job has one `setup-go` step, and Step 4 shows what the count catches.

- [ ] **Step 3: Implement** — `.github/workflows/ci.yaml` (Edit).

1. `render-validate`, its name, comment and environment, old:

```yaml
      - name: render each fixture with local module sources and tofu validate it (proves the JSON syntax, lock file and modules)
        id: render-validate
        # basic has no aws part; aws-registry passes each environment's variables.
        env:
```

new:

```yaml
      - name: render each fixture with local module sources and tofu validate every stack (proves the JSON syntax, lock files and modules)
        id: render-validate
        # basic renders only the github stack; aws-registry adds each
        # environment's variables and the AWS stacks under aws/. Every directory
        # with a main.tf.json is a stack. -lockfile=readonly fails a stack whose
        # shipped lock file lacks its provider, and the plugin cache downloads
        # each provider once.
        env:
          TF_PLUGIN_CACHE_DIR: ${{ runner.temp }}/tofu-plugin-cache
```

2. `render-validate`, its script, old:

```yaml
        run: |
          for fixture in basic aws-registry; do
            go run ./cmd/idp render --dir "internal/render/testdata/$fixture" --out "$RUNNER_TEMP/rendered/$fixture" --modules-dir modules
            tofu -chdir="$RUNNER_TEMP/rendered/$fixture/github" init -backend=false -input=false
            tofu -chdir="$RUNNER_TEMP/rendered/$fixture/github" validate
          done
```

new:

```yaml
        run: |
          mkdir -p "$TF_PLUGIN_CACHE_DIR"
          shopt -s globstar
          for fixture in basic aws-registry; do
            go run ./cmd/idp render --dir "internal/render/testdata/$fixture" --out "$RUNNER_TEMP/rendered/$fixture" --modules-dir modules
            for stack in "$RUNNER_TEMP/rendered/$fixture"/**/main.tf.json; do
              stack="${stack%/main.tf.json}"
              tofu -chdir="$stack" init -backend=false -input=false -lockfile=readonly
              tofu -chdir="$stack" validate
            done
          done
```

3. The `floci` job's comment, old:

```yaml
  floci:
    # Level 3 (spec §8.4): the AWS modules applied to two accounts of one floci,
    # checked against internal/naming, replayed and destroyed by internal/floci.
```

new:

```yaml
  floci:
    # Level 3 (spec §8.4): the rendered AWS stacks of the aws-registry fixture
    # applied to two accounts of one floci, checked against internal/naming,
    # replayed and destroyed by internal/floci.
```

4. The `floci-test` step's name, old:

```yaml
      - name: apply, check, replay and destroy the AWS modules on floci
```

new:

```yaml
      - name: apply, check, replay and destroy the rendered AWS stacks on floci
```

Run: `go test ./internal/workflowspec/ -count=1`
Expected: `ok`. Then `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing). actionlint and zizmor run in CI's `workflows` job (watch list).

```bash
git add internal/workflowspec/ci_test.go .github/workflows/ci.yaml
git commit -m "ci: validate every rendered stack and require one setup-go in the floci job"
```

- [ ] **Step 4: Mutation check** (nothing is committed) — in `.github/workflows/ci.yaml`, job `floci`, insert after the line `          cache: false` that follows `# jobs restore.` a second step:

```yaml
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: false
```

Run `go test ./internal/workflowspec/ -count=1 -run TestCIFlociJob`. Expected: `FAIL` with `ci floci has 2 setup-go steps, want exactly one`. Restore with `git checkout .github/workflows/ci.yaml`; `git status --short` prints nothing.

---

### Task 8: Docs — `cli.md`, `claims.md`, `pipelines.md`

**Files:**
- Modify: `docs/cli.md`, `docs/claims.md`, `docs/pipelines.md`

**Interfaces:**
- Consumes: the render (Task 3), `idp diff`'s pinned output (Task 5), ADR-0030 (Task 1).
- Produces: user docs for what an `aws` block and `aws.registry` render now (D-12).

- [ ] **Step 1: `docs/cli.md`** (Edit)

1. `render`, `--modules-dir`, old:

```markdown
- `--modules-dir`: use local module sources from this directory instead (validation only). It must be on the same volume as `--out`.
```

new:

```markdown
- `--modules-dir`: use local module sources from this directory instead (validation only). It must be on the same volume as `--out`. Every stack, the AWS stacks deeper in the tree included, points at it from its own directory.
```

2. `render`, "Writes", old:

```markdown
Writes: the rendered tree under `--out`, including the `.idp-rendered` marker file, and one `render: wrote PATH` line per file. `idp diff` requires that marker.
```

new:

```markdown
Writes: the rendered tree under `--out`, including the `.idp-rendered` marker file, and one `render: wrote PATH` line per file. `idp diff` requires that marker. The tree holds the `github` stack and, since Phase 2d, an AWS stack for every environment with an `aws` block (`aws/<env>/_baseline`) and for every Component with `aws.registry: true` in each of its environments (`aws/<env>/components/<name>`), each with its lock file ([claims reference](claims.md#rendering)).
```

3. `diff`, what a stack is, with an example, old:

```markdown
A stack is a directory that holds `main.tf.json`. `.terraform` directories are skipped.
```

new:

````markdown
A stack is a directory below `--new` or `--wet` that holds `main.tf.json`, at any depth: `github`, or `aws/dev/components/api`. A file belongs to the deepest stack that holds it. The root itself is never a stack, and files outside every stack, such as the marker, are ignored. `.terraform` directories are skipped; anything but a directory or a regular file, a symbolic link included, fails the command.

For example, after the environment `staging` was removed from the platform config and one file of `api`'s `dev` stack changed:

```text
aws/dev/_baseline: unchanged
aws/dev/components/api: changed
aws/staging/_baseline: orphan
github: unchanged
diff: 2 affected stack(s)
```
````

- [ ] **Step 2: `docs/claims.md`** (Edit)

1. The `aws` block, old:

```markdown
An `aws` block is shaped like real AWS: an account ID and a region, never an emulator endpoint (the schema rejects any other key). In v1, AWS is the floci emulator in CI (spec §5.6), so the IDs can be fake ones like those above. An environment needs an `aws` block only when a Component with `aws.registry: true` lists it.
```

new:

```markdown
An `aws` block is shaped like real AWS: an account ID and a region, never an emulator endpoint (the schema rejects any other key). In v1, AWS is the floci emulator in CI (spec §5.6), so the IDs can be fake ones like those above. An environment needs an `aws` block only when a Component with `aws.registry: true` lists it. Every environment with one gets its baseline stack, which creates the account's GitHub OIDC provider, whether or not a Component uses it (see "Rendering").
```

2. Component, the variables bullet, old:

```markdown
- **The variables come first.** Since Phase 2b the github stack carries them, and GitHub creates them when the pipelines apply that stack, after the merge (see "Rendering"). The ECR repository and the role they name are rendered from Phase 2d and applied, to the floci emulator, from Phase 2e.
```

new:

```markdown
- **The variables come first.** Since Phase 2b the github stack carries them, and GitHub creates them when the pipelines apply that stack, after the merge (see "Rendering"). Since Phase 2d the render also writes the AWS stack that creates the ECR repository and the role they name; the pipelines apply it, to the floci emulator, from Phase 2e.
```

3. "Rendering", the list, old:

```markdown
- `rendered/github/.terraform.lock.hcl`: the pinned provider checksums;
```

new:

```markdown
- `rendered/github/.terraform.lock.hcl`: the pinned provider checksums;
- `rendered/aws/<env>/_baseline/`: for every environment with an `aws` block, a stack that calls `modules/aws/baseline`, the account's GitHub OIDC provider;
- `rendered/aws/<env>/components/<name>/`: for every Component with `aws.registry: true`, in each of its environments, a stack that calls `modules/aws/component` with the repository, the role and the OIDC provider that `internal/naming` computes, the same names the github stack's variables carry;
```

4. "Rendering", the AWS stacks, old:

```markdown
A Component with `aws.registry: true` passes `variables` to its module for each of its environments: `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION`, computed from the names and the environment's `aws` block (spec §5.3). A Component without it renders exactly as before, so its stack plans no change. Rendering creates nothing: GitHub creates the variables only when the pipelines apply the github stack (reconcile, after the merge), like every other GitHub resource. The AWS stacks, with the ECR repository and the CI role, render from Phase 2d.
```

new:

```markdown
A Component with `aws.registry: true` passes `variables` to its module for each of its environments: `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION`, computed from the names and the environment's `aws` block (spec §5.3). A Component without it renders exactly as before, so its stack plans no change. Rendering creates nothing: GitHub creates the variables only when the pipelines apply the github stack (reconcile, after the merge), like every other GitHub resource.

Each AWS stack holds the pinned OpenTofu and `hashicorp/aws` versions, the provider with the environment's region and nothing else, the local backend at `terraform.tfstate` in its own directory, no encryption block, and the engine's AWS lock file. Its state stays in the job that runs it and is never committed to `wet` ([ADR-0030](adr/0030-aws-stack-state-stays-in-the-job.md)). The stacks never mention floci: CI adds `floci_override.tf.json` (spec §5.6). A claims repo without `aws` blocks renders exactly as before. The pipelines apply the AWS stacks only from Phase 2e; until then, a claims repo with an `aws` block stays on an engine release from before Phase 2d ([pipelines](pipelines.md#limitations)).
```

5. "Rendering", the github stack's state, old:

```markdown
`--out` is relative to the current directory, not to `--dir`. The stack's local backend writes its state to `<out>/../tfstate/github.tfstate`, which is where the `wet` branch keeps it (spec §5.4).
```

new:

```markdown
`--out` is relative to the current directory, not to `--dir`. The github stack's local backend writes its state to `<out>/../tfstate/github.tfstate`, which is where the `wet` branch keeps it (spec §5.4).
```

- [ ] **Step 3: `docs/pipelines.md`** (Edit), "Limitations", old:

```markdown
- Only the `github` stack is reconciled for now; the workflows fail when any other stack is affected.
```

new:

```markdown
- Only the `github` stack is reconciled until Phase 2e; the workflows fail when any other stack is affected. The render writes AWS stacks for every environment with an `aws` block since Phase 2d, so a claims repo with one stays on an engine release from before Phase 2d (`idp-claims` pins `v0.1.1`) until the release that applies them, `0.2.0`.
```

- [ ] **Step 4: Check and commit**
- Task 1's anchor check prints `all spec anchors resolve`.
- Every relative link in the three files resolves: `python -I -c "import re,pathlib; print([(str(p),l) for p in map(pathlib.Path,['docs/cli.md','docs/claims.md','docs/pipelines.md']) for l in re.findall(r'\]\(([^)#\s]+)(?:#[^)]*)?\)', p.read_text(encoding='utf-8')) if not l.startswith('http') and not (p.parent/l).exists()] or 'all links resolve')"` prints `all links resolve`.
- `rg -n 'render from Phase 2d|rendered from Phase 2d and applied' docs/claims.md` prints nothing.
- The naming check prints nothing.

```bash
git add docs/cli.md docs/claims.md docs/pipelines.md
git commit -m "docs: describe the rendered aws stacks and nested stack discovery"
```

---

### Task 9: Ship — verification, PR, review, merge, phase log, README

Controller task (tests are delegated to a subagent).

- [ ] **Step 1: Local verification**
  - `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).
  - The render validation snippet prints its six `Success!` lines.
  - The module test snippet ends with `Success! 2 passed, 0 failed.` (`aws/baseline`) and `Success! 8 passed, 0 failed.` (`aws/component`); the modules are unchanged.
  - The naming check over the branch's changes prints nothing.
  - `git diff --stat main..HEAD` lists exactly the 27 files of the File Structure (Tasks 1 to 8).
  - `git diff main..HEAD -- modules lockfiles internal/render/testdata/basic internal/render/testdata/aws-registry/expected/github .github/workflows/pr.yaml .github/workflows/reconcile.yaml .github/workflows/drift.yaml .github/workflows/recover.yaml .github/actions examples` prints nothing: no module, lock file, github golden, reusable workflow, action or caller template change (D-1, D-13).

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feat/phase-2d-aws-render
gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-2d-aws-render --title "feat: phase 2d render the aws stacks" --body "Implements docs/superpowers/plans/2026-11-17-phase-2d-aws-render.md (spec amendment A13, ADR-0030): idp render writes aws/<env>/_baseline for every environment with an aws block and aws/<env>/components/<name> for every registry Component in its environments, thin stacks with the identities internal/naming computes, the AWS lock file and a local backend whose state stays in the job. render.AWSProviderVersion moves with the module pins through a new Renovate manager. The floci test applies the real render; render-smoke validates every stack; idp diff is pinned on nested stacks and orphans. The workflows still reconcile only the github stack (Phase 2e); no release: 0.2.0 ships in Phase 2f."
```

- [ ] **Step 3: CI** — `gh pr checks <number> --repo jellalshadows-idp/idp-engine --watch`.
  - Expected: `go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke`, `floci` and `release-snapshot` pass.
  - `render-smoke`'s log shows six `Success! The configuration is valid.`
  - `floci`'s log shows the offline tests passing and `--- PASS: TestFloci`, with the subtests `dev`, `pro` and `accounts are isolated`, and two `floci's repository URI for api:` lines.
  - Record the watch-list items: the durations of `render-smoke` and `floci` against Phase 2c's, and the plugin cache use.
  - A failure goes through the fix loop: fix with a test, scoped review, push. A floci fidelity gap is handled as spec §11 risk 4 says, with the owner's ruling, never by loosening an assertion silently.

- [ ] **Step 4: Final whole-branch review** (most capable model).
  - The package: `main..HEAD`; the spec (§4.7, §5.3–§5.6, §6, §7.2, §8.4); ADR-0002, ADR-0004, ADR-0006 and ADR-0030; this plan.
  - One fix wave, one scoped re-review, rulings in the ledger, and CI green again after the wave.

- [ ] **Step 5: Merge** — confirm the PR's latest CI is green, then `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`.

- [ ] **Step 6: The release PR and the harness** (watch list) — the merge's push runs `release.yaml`.
  - Read the release PR with `gh pr list --repo jellalshadows-idp/idp-engine --head release-please--branches--main --json number,title --jq '.[] | "\(.number) \(.title)"'`. Expected: one line, `28 chore(main): release 0.2.0`. Record it and leave the PR open (D-13).
  - Record the harness run that `e2e-release-pr` dispatched (`gh run list --repo jellalshadows-idp/idp-engine --workflow e2e.yaml --limit 1 --json databaseId,status,conclusion`) and, when it ends, its conclusion. Expected: `success`. A failure is a regression to fix before 2e.

- [ ] **Step 7: Write `docs/phases/phase-2d.md`** in the style of `docs/phases/phase-2c.md`. Include:
  - the header list: period, plan, the spec amended by A13, ADR-0030 added and ADR-0004 amended, the previous phase (the Phase 2c log), and the status with the PR and its merge commit;
  - the summary, and the tasks with their commits and reviews;
  - every ruling (D-1 to D-15 and the execution rulings) with what, why and the cost if wrong. Include the planning findings:
    - OpenTofu applies `TF_ENCRYPTION` to a stack without an `encryption` block, and its state is then unreadable without it (D-3);
    - OpenTofu 1.12.6's `validate` ignores `required_version` (D-14);
    - `-lockfile=readonly` catches a stack with the wrong lock file (D-8);
  - the Review Focus table with each result, and the watch list with each result: the CI checks; the `render-smoke` and `floci` durations; the release PR's number and version; the harness run; the Renovate observations, or "not yet run";
  - the notable findings caught by review, and the final disposition of every finding. The items Phase 2c carried are **Fixed**: R3-001 and R3-002 in Tasks 6 and 7, the AWS render in Task 3, the backend and encryption in Tasks 1 and 3 (D-3), `AWSProviderVersion` in Tasks 3 and 4, `TestFloci` on the real render in Task 6. Phase 1b's M2 items are **Fixed** in Task 5 (D-9).

  Then "Carried into later sub-phases":
  - **2e:**
    - the workflows plan and apply the AWS stacks against floci: they start floci per job (the `floci` job's service block is the model), write `floci_override.tf.json` (`writeOverride` is the model), and replay from `wet` (spec §6.1 step 4);
    - they run each AWS stack in a scratch copy, keep one `TF_ENCRYPTION` setting per stack, and never push AWS state (ADR-0030);
    - they replace the "check the affected stacks" guard, and `wet-push` gains `rendered/aws`;
    - `plan-summary`, `gate`, `comment` and `issue` take the AWS stacks (D-9);
    - the floci job's measured time feeds spec §11 risk 3;
    - any Renovate observation still pending.
  - **2f:** as Phase 2b's log lists; `idp-claims` re-pins to `0.2.0`, and its three `aws` blocks then render (D-10).
  - **Phase 3:** `aws/<env>/workspaces/<name>`.

  Then "Owner notes": release PR #28 stays open until 2f, and every push to `main` re-runs the harness on it; the earlier owner items stay as recorded. Close with the process notes (inline or delegated execution, TDD evidence, the mutation checks, local OpenTofu 1.12.6 against CI's 1.13.2) and the status.

- [ ] **Step 8: Update `README.md`** (exact replacements with Edit)

1. The status line, old:

```markdown
> **Status:** Phase 2c is complete: `modules/aws/baseline` (an environment account's GitHub OIDC provider) and `modules/aws/component` (a Component's ECR repository and its push-only CI role) exist, and CI applies them to two accounts of the floci emulator on every PR, checking their identities against `internal/naming`. Each environment needs its own AWS account in the standard partition. No render calls the modules yet. See the [phase 2c log](docs/phases/phase-2c.md) and [docs/claims.md](docs/claims.md). Phase 2d (rendering the AWS stacks) is next.
```

new:

```markdown
> **Status:** Phase 2d is complete: `idp render` writes the AWS stacks, a baseline for every environment with an `aws` block and a stack for every Component with `aws.registry` in each of its environments, with the identities `internal/naming` computes and their state kept in the job, never in `wet` ([ADR-0030](docs/adr/0030-aws-stack-state-stays-in-the-job.md)). CI validates every rendered stack, applies the rendered AWS stacks to two accounts of the floci emulator on every PR, and `idp diff` reports them by their nested paths. The pipelines do not apply them yet. See the [phase 2d log](docs/phases/phase-2d.md) and [docs/claims.md](docs/claims.md). Phase 2e (the workflows' AWS plan and apply) is next.
```

2. "Honest limitations", the pipelines bullet, old:

```markdown
- The pipelines reconcile **GitHub only** (teams, repositories, rulesets, environments and, since Phase 2b, the environment variables of a Component with `aws.registry`). The AWS modules that create the ECR repository and the CI role exist since Phase 2c and are tested against floci in CI; they are rendered from Phase 2d and applied from Phase 2e.
```

new:

```markdown
- The pipelines reconcile **GitHub only** (teams, repositories, rulesets, environments and, since Phase 2b, the environment variables of a Component with `aws.registry`). The AWS stacks that create the OIDC provider, the ECR repository and the CI role are rendered since Phase 2d and tested against floci in CI; the pipelines apply them from Phase 2e, so a claims repo with an `aws` block stays on `v0.1.1` until `0.2.0`.
```

- [ ] **Step 9: Commit doc-only to `main`** (side effect 2) — the naming check prints nothing; then:

```bash
git add docs/phases/phase-2d.md README.md
git commit -m "docs: add the phase 2d execution log"
git push
```

The push runs `release.yaml`, which refreshes the open release PR and re-runs the harness on it. Leave the PR open.

- [ ] **Step 10: Delete the SDD workspace** (side effect 3) — after checking that the phase log reproduces the ledger: `rm -rf .superpowers/sdd`, then `git status --short` prints nothing.

---

## Self-review

- **Spec coverage.**
  - §5.4's layout: `_baseline` per environment with an `aws` block, `components/<name>` per registry Component and environment, no `workspaces/` (D-1, Task 3).
  - §5.3's thin stacks and lock files (D-2, Task 3).
  - §5.5's identities from `internal/naming`, pinned outside the golden files (D-6, Task 3).
  - §5.6: nothing of floci in the render; level 3 adds the override (D-2, D-7; Tasks 3 and 6).
  - §7.2 and §5.6 amended for the AWS state (D-3, Task 1).
  - §7.5's pins: no new pin, and Renovate moves the renderer's (D-5, Task 4).
  - §8.2's determinism over every file (Task 3).
  - §8.4's level 3 on the render (Task 6, A13).
  - §6's workflows unchanged (D-13); what 2e must change is recorded (D-9).
- **Rulings 1 to 12** map to D-1 to D-13.
- **Every carried item has a task:**
  - 2c's 2d list: the render (Task 3), the backend and encryption (D-3), `AWSProviderVersion` (Tasks 3, 4), `TestFloci` on the render (Task 6), the Phase 1b M2 items (Task 5), R3-001 (Task 6), R3-002 (Task 7);
  - the Renovate observations: the watch list.
- **No placeholders.** Every Edit step has an exact old block, checked to occur once in the file at that point of the plan, and a new block. Every new file is given whole; the golden files are generated, and their expected bytes are given.
- **Validated.** Every block was applied verbatim, in task order, by a script to an archive of `e521466`. The result passed `go test ./...`, `go vet ./...` and `gofmt -l .`, and RED, GREEN and the mutation checks were observed as each task states. The render validation snippet and the module tests passed with OpenTofu 1.12.6 (D-14).
- **Names are consistent across tasks:**
  - `moduleSource(opts, stackDir, module)`, from Task 2 to Task 3;
  - `AWSProviderVersion`, from Task 3 to Task 4;
  - the golden paths, from Task 3 to Task 4;
  - `render.Render`, `render.WriteTree` and `claims.Load`, in Tasks 5 and 6;
  - `stacks`, `renderFixture` and `writeOverride`, in Task 6.
- **Left to CI:** OpenTofu 1.13.2; actionlint, zizmor and shellcheck on the new script; the floci apply of the rendered stacks; the plugin cache; Renovate's grouping of the new manager with the module pins (watch list).
- **The skill named by the brief, `superpowers:writing-plans`, was not installed on the planning machine.** This plan follows the Phase 2c plan's structure instead.
