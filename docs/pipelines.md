# Pipelines

A claims repo reconciles through four thin workflows that call the engine's reusable workflows (spec §6). This page shows how to onboard a claims repo and what each pipeline does. When something goes wrong, see the [pipelines runbook](runbooks/pipelines.md).

## Onboard a claims repo

1. Bootstrap the repo first ([bootstrap runbook](runbooks/bootstrap.md)). The pipelines need what `idp bootstrap apply` creates: the `wet` branch, the rulesets, the `idp-approval` and `idp-write` environments, the secrets `IDP_READER_PRIVATE_KEY` and `IDP_STATE_PASSPHRASE`, the variables `IDP_READER_CLIENT_ID` and `IDP_BOOTSTRAP`, and, in `idp-write`, the secret `IDP_WRITER_PRIVATE_KEY` and the variable `IDP_WRITER_CLIENT_ID`. The reconcile, recover and drift templates pass the writer key by name; keep that mapping.
2. Copy the four files of [`examples/claims-repo/.github/workflows/`](../examples/claims-repo/.github/workflows/) into the claims repo's `.github/workflows/`. They pin the latest release (its commit, with the tag as the comment); keep all four on the same one ([Upgrading the engine](#upgrading-the-engine)).
3. Add `config/platform.yaml` ([claims reference](claims.md)), and claims when you want them.
4. Open a pull request. The `idp-gate` check that the `main` ruleset requires comes from the PR workflow.

## The four workflows

| Workflow | Trigger | Checks | What it does |
|---|---|---|---|
| `pr.yaml` | pull request to `main` | `pr / validate`, `pr / plan`, `pr / plan-aws <stack>` (one per affected AWS stack), `pr / comment`, `pr / verdict`, `idp-gate` | Validates and renders the claims, diffs the render against `wet`, plans the `github` stack with the reader App without refreshing and each affected AWS stack against its own floci, and keeps one sticky plan comment on the PR. |
| `reconcile.yaml` | push to `main`, manual dispatch | `reconcile / plan`, `reconcile / plan-aws <stack>`, `reconcile / gate`, `reconcile / approve`, `reconcile / apply-aws <stack>`, `reconcile / apply-github` | Re-renders and diffs against the current `wet` (every stack on a dispatch), plans the `github` stack with the writer App in `idp-write` and each AWS stack against its own floci, gates, waits for approval when the gate asks for it, applies the saved plans, and commits the state and the renders that applied to `wet`. |
| `drift.yaml` | daily at 05:23 UTC, manual dispatch | `drift / drift` | Plans the `wet` render with the writer App in `idp-write`, checks the bootstrap protections with the reader App, and keeps one `drift` issue open while anything drifted. |
| `recover.yaml` | manual dispatch with a run id | `recover / recover` | Commits the tree a reconcile run saved when its wet push failed. |

### Pull requests

- Pull requests from forks are validated and rendered, never planned: they get no secrets, and `pr / verdict` fails with "a maintainer must push this branch to the repo" (spec §6.1).
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
- A cancelled run starts no new job (the `comment`, `gate` and apply jobs use `!cancelled()`, not `always()`). Cancelling does not interrupt an apply that has started: the apply step runs to completion, and the state is committed. A force-cancel kills OpenTofu mid-apply and can lose state; never force-cancel while `reconcile / apply-github` runs.

### Drift

- A run fails only when the check cannot run. Drift opens (or updates) the issue labelled `drift`, titled "Drift detected", and adds a warning annotation; a clean run closes it. Nothing is remediated automatically.
- It checks the `github` stack only: an AWS stack's state lives only in its job's floci, so there is nothing to compare (spec §6.4).
- Ruleset bypass lists are invisible to the reader token, so `bootstrap check` reports them as notices ([ADR-0017](adr/0017-drift-check-token.md)).

### Recover

- See the [runbook](runbooks/pipelines.md#a-wet-push-failed). The recovery refuses a run that is not a failed or cancelled reconcile of `main`, a run the open "Wet push failed" issue does not name, and any run after `wet` moved since that run's plan.

## Permissions and credentials

| Job | `GITHUB_TOKEN` scopes | App token | Environment |
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
- Artifacts: `idp-plan-<stack>` (each stack's reduced plan, its addresses and actions only, 1 day; `<stack>` has each `/` as `.`), `idp-renders` (the new and the `wet` AWS renders, 1 day), `idp-gated` (the gated fingerprint, 1 day), `idp-applied-<stack>` (the AWS stacks that applied, 1 day), `idp-reconcile` (the encrypted `github` plan and the render, 1 day), `idp-wet-root` (the wet commit with the encrypted state and its list of paths, `wet-paths`, 7 days; the open "Wet push failed" issue names its artifact ID, and the recovery downloads that ID). Artifacts of public repos are public; the `github` plan and state are encrypted ([ADR-0002](adr/0002-encrypted-opentofu-state-in-git.md)), and no full plan leaves its job ([ADR-0031](adr/0031-floci-jobs-hold-no-secret.md)).
- A job that trusts an artifact made by a job without floci downloads it by the ID its producer outputs, never by name or pattern: `idp-renders`, `idp-plan-github`, `idp-reconcile` and `idp-gated`. `idp-wet-root` is recovered by the ID its issue names; it is uploaded with `overwrite`, so an artifact a floci job uploaded first under that name is replaced, not kept. Only the floci jobs' artifacts (`idp-plan-aws.*`, `idp-applied-*`) go by pattern. The `comment` and `gate` jobs lay the plans out the same way whatever the artifact count: the `github` plan merges into `plans/idp-plan-github/`, the AWS plans into `aws-plans/`, a directory apart, and a step copies each plan the trusted matrix names to `plans/idp-plan-<stack>/<stack>.json`, where `idp plan-summary` reads it. So no floci job's artifact can stand in for the `github` plan, a render or the gated fingerprint ([ADR-0031](adr/0031-floci-jobs-hold-no-secret.md)).
- AWS jobs: one per affected AWS stack, ten at a time (`max-parallel`), and every job finishes even when another fails. `idp stacks` refuses more than 256 AWS stacks in one run, the jobs one matrix runs: split such a change.

## How the workflows get idp

Each job checks out the engine at the reusable workflow's own commit (`job.workflow_sha`, the commit the caller pinned) and runs the `setup-idp` composite action. The action gets `idp` one of two ways and installs OpenTofu in either case, so the workflow, the CLI and the rendered module refs are always one version ([ADR-0021](adr/0021-reusable-workflows-build-idp-from-their-commit.md), [ADR-0028](adr/0028-releases-and-how-the-workflows-get-idp.md)):

- **The commit is a release** (a `vX.Y.Z` tag points at it). It downloads the release's linux archive for the runner and `checksums.txt`, checks the checksum, verifies the build provenance with `gh attestation verify` (built by `release.yaml` on `main` from that very commit), checks that `idp version` prints the tag, and installs it. Module sources pin the tag. If any of this fails, the job fails: a release commit never falls back to a build, because the render would then pin the commit and a broken release would go unnoticed.
- **Any other commit** (the end-to-end harness's commit under test, or an unreleased pin). It installs Go from the engine's `go.mod` and builds `idp` from the checkout. Module sources pin the commit.

The tag lookup, the download and the provenance check use the job's own `GITHUB_TOKEN`; the engine is public, so `contents: read` is enough. Nothing is cached.

## Upgrading the engine

Engine releases are `vX.Y.Z` tags ([releases runbook](runbooks/releases.md)). A claims repo upgrades by re-pinning its four caller workflows:

1. Re-pin all four together, to the commit the new tag points at, with the tag as the comment: `@<commit> # vX.Y.Z`. Renovate opens this PR (group "idp engine", [dependencies runbook](runbooks/dependencies.md)); by hand, the commit is `gh api repos/jellalshadows-idp/idp-engine/git/ref/tags/vX.Y.Z --jq .object.sha`.
2. The PR's plan comment shows only the resource changes the new engine makes, usually `No infrastructure changes.`; the version jump shows in the PR diff (`# vA` to `# vB`, all four callers) and in the release notes. A plan that does change resources is reviewed like any other plan, and a delete or replace waits for approval. Merge it when the plan is what you expect.
3. Re-run `idp bootstrap apply` for the claims repo when the release notes say so, after the re-pin merges, never before.

**From an engine older than commit `3b9beed` (the end-to-end harness) to `v0.1.0` or later:** the newer engine writes `IDP_BOOTSTRAP` with `approverIds`, which the older engine's parser rejects, while the newer parser still reads the older `approverId` form. So merge the re-pin first, then run `idp bootstrap apply` and `check` for that claims repo ([bootstrap runbook](runbooks/bootstrap.md)). Until `apply` runs, the drift check reports `IDP_BOOTSTRAP` as drift.

## Limitations

- The AWS stacks are planned and applied against floci only (ADR-0004): nothing reaches a real AWS account, and their state ends with the job. Workspaces are not reconciled yet (Phase 3): a change that affects one fails `idp stacks`. A claims repo with an `aws` block stays on an engine release from before Phase 2d (`idp-claims` pins the `v0.1.1` tag) until `v0.2.0`.
- The required check comes from a workflow file in the claims repo, so a pull request can change it. Requiring a review on `.github/` closes that gap; it is the owner's decision ([ADR-0022](adr/0022-required-check-lives-in-the-caller.md)).
- GitHub disables scheduled workflows in public repos after 60 days without activity. Re-enable the drift check with `gh workflow enable drift.yaml`.
- Every job lists the engine's tags through the GitHub API before it gets `idp`; when that call fails, the job fails rather than guess whether its commit is a release.
