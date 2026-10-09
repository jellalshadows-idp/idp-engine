# 0001. Edge-triggered reconciliation on GitHub Actions

- Status: Accepted
- Date: 2026-10-08
- Spec: [§1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#1-intent), [§6](../superpowers/specs/2026-10-08-idp-on-actions-design.md#6-pipelines)

## Context

The platform turns claims (YAML in a Git repo) into real GitHub and AWS resources. The usual way to do that is a Kubernetes operator: claims are rendered into custom resources, synced by a GitOps tool such as Argo CD, and a **level-triggered** operator with a periodic sync keeps reality converged on them. Level-triggered means the system keeps comparing desired and actual state on a timer, so it also notices changes nobody committed.

This project has a different set of forces. It is a public portfolio project owned by a single person (bus factor 1), and its purpose is to show platform-engineering judgment: a claims model, rendering, gated plan/apply, state handling and least privilege (§1). Running a cluster just to host a reconciler would add operating cost and a permanent attack surface that has nothing to do with what the project wants to demonstrate. GitHub Actions is already present in every repo, is available on public repos, and already provides the primitives a reconciler needs: events, concurrency groups, environments with approvals, and a job log.

The trade is explicit: Actions only runs when something happens. There is no process watching the world between events. The design has to decide how much of the level-triggered behavior it gives up, and what it replaces it with.

## Decision

Reconciliation is **edge-triggered** and runs on GitHub Actions only, with no cluster and no long-running process (spec §2.2, §6). The triggers are:

- `pull_request` to `main`: validate, render, diff against `wet`, plan, and post a sticky comment (§6.1).
- `push` to `main` and `workflow_dispatch` on `main`: the single `reconcile` workflow, serialized in concurrency group `idp-wet` with `cancel-in-progress: false` (§6.2). The manual dispatch is the universal recovery button.
- A daily `cron: '23 5 * * *'` drift check, in the same `idp-wet` group, that plans the GitHub stack and opens or updates one issue labeled `drift` (§6.4).

Drift is **report-only**: the daily run never remediates. A human either fixes the claims or runs reconcile. Kubernetes and level-triggered continuous reconciliation are out of scope for v1 (§2.2). The guiding rule of the pipelines is that nothing is applied that nobody saw (§6).

## Consequences

### Positive

- Zero infrastructure to run: no cluster, no operator image, no Argo CD to patch.
- Every change has an event, a log and, when it matters, an approval. The audit trail is the Actions history plus the `wet` commits.
- Reconcile is serialized and self-healing: a failed stack keeps its old render in `wet`, so the next run sees the difference and retries (§6.2).
- Easy to reproduce for a reviewer: bootstrapping needs a GitHub org, not a cluster.

### Negative / costs

- No continuous reconciliation. A manual change in GitHub between events is only noticed by the daily drift run, and then only reported.
- AWS drift is not detected at all in v1, because emulated AWS is rebuilt from `wet` on every job and cannot drift (§2.2).
- Scheduled workflows in public repos are disabled after 60 days without activity. Renovate PRs keep the repo active; this is documented, not enforced (§6.4, risk 6 in §11).
- It does not scale like an operator. Continuous reconciliation, multi-cloud and scale are what is lost (§2.2).

### Follow-ups

- Phase 1 builds the three pipelines (`pr`, `reconcile`, `drift`) and the E2E harness that exercises them (§10).
- The drift issue and the cron limitation must be stated in the README limitations (§9.3).

## Alternatives considered

- **A Kubernetes operator that reconciles continuously.** Rejected: level-triggered is the stronger model, but it needs a cluster to host it. That is infrastructure and operating cost unrelated to the platform ideas the project wants to show, and a single maintainer would have to keep it alive.
- **Actions with auto-remediation on drift.** Rejected: it would apply changes no human reviewed, breaking the rule that nothing is applied that nobody saw (§6). Drift opens an issue instead.
- **A scheduled full reconcile instead of report-only drift.** Rejected for the same reason, and because every scheduled apply would spend CI minutes and API rate limit without a reviewed change.

## References

- Spec §1, §2.2, §6.1, §6.2, §6.4, §9.3, §11 (risk 6).
