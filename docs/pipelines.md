# Pipelines

A claims repo reconciles through four thin workflows that call the engine's reusable workflows (spec §6). This page shows how to onboard a claims repo and what each pipeline does. When something goes wrong, see the [pipelines runbook](runbooks/pipelines.md).

## Onboard a claims repo

1. Bootstrap the repo first ([bootstrap runbook](runbooks/bootstrap.md)). The pipelines need what `idp bootstrap apply` creates: the `wet` branch, the rulesets, the `idp-approval` and `idp-write` environments, the secrets `IDP_READER_PRIVATE_KEY` and `IDP_STATE_PASSPHRASE`, the variables `IDP_READER_CLIENT_ID` and `IDP_BOOTSTRAP`, and, in `idp-write`, the secret `IDP_WRITER_PRIVATE_KEY` and the variable `IDP_WRITER_CLIENT_ID`. The reconcile, recover and drift templates pass the writer key by name; keep that mapping.
2. Copy the four files of [`examples/claims-repo/.github/workflows/`](../examples/claims-repo/.github/workflows/) into the claims repo's `.github/workflows/`. Keep the pinned engine commit; all four must pin the same one.
3. Add `config/platform.yaml` ([claims reference](claims.md)), and claims when you want them.
4. Open a pull request. The `idp-gate` check that the `main` ruleset requires comes from the PR workflow.

## The four workflows

| Workflow | Trigger | Checks | What it does |
|---|---|---|---|
| `pr.yaml` | pull request to `main` | `pr / validate`, `pr / plan`, `pr / comment`, `pr / verdict`, `idp-gate` | Validates and renders the claims, diffs the render against `wet`, plans the affected stacks with the reader App without refreshing, and keeps one sticky plan comment on the PR. |
| `reconcile.yaml` | push to `main`, manual dispatch | `reconcile / plan`, `reconcile / approve`, `reconcile / apply-github` | Re-renders and diffs against the current `wet` (every stack on a dispatch), plans with the writer App in `idp-write`, gates, waits for approval when the gate asks for it, applies the saved plan with the writer App, and commits the state and the render to `wet`. |
| `drift.yaml` | daily at 05:23 UTC, manual dispatch | `drift / drift` | Plans the `wet` render with the writer App in `idp-write`, checks the bootstrap protections with the reader App, and keeps one `drift` issue open while anything drifted. |
| `recover.yaml` | manual dispatch with a run id | `recover / recover` | Commits the tree a reconcile run saved when its wet push failed. |

### Pull requests

- Pull requests from forks are validated and rendered, never planned: they get no secrets, and `pr / verdict` fails with "a maintainer must push this branch to the repo" (spec §6.1).
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

### Drift

- A run fails only when the check cannot run. Drift opens (or updates) the issue labelled `drift`, titled "Drift detected", and adds a warning annotation; a clean run closes it. Nothing is remediated automatically.
- Ruleset bypass lists are invisible to the reader token, so `bootstrap check` reports them as notices ([ADR-0017](adr/0017-drift-check-token.md)).

### Recover

- See the [runbook](runbooks/pipelines.md#a-wet-push-failed). The recovery refuses a run that is not a failed or cancelled reconcile of `main`, a run the open "Wet push failed" issue does not name, and any run after `wet` moved since that run's plan.

## Permissions and credentials

| Job | `GITHUB_TOKEN` scopes | App token | Environment |
|---|---|---|---|
| `pr / validate` | `contents: read` | none | none |
| `pr / plan` | `contents: read` | reader (no refresh), whole installation | none |
| `pr / comment` | `contents: read`, `pull-requests: write` | none | none |
| `pr / verdict` | none | none | none |
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

Each job checks out the engine at the reusable workflow's own commit and builds `idp` from it with the `setup-idp` composite action, so the workflow, the CLI and the rendered module refs are one version ([ADR-0021](adr/0021-reusable-workflows-build-idp-from-their-commit.md)).

## Limitations

- Only the `github` stack is reconciled for now; the workflows fail when any other stack is affected.
- The required check comes from a workflow file in the claims repo, so a pull request can change it. Requiring a review on `.github/` closes that gap; it is the owner's decision ([ADR-0022](adr/0022-required-check-lives-in-the-caller.md)).
- GitHub disables scheduled workflows in public repos after 60 days without activity. Re-enable the drift check with `gh workflow enable drift.yaml`.
