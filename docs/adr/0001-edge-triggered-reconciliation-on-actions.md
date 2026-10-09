# 0001. Edge-triggered reconciliation on GitHub Actions

- Status: Accepted
- Date: 2026-10-08
- Spec: [§1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#1-intent), [§6](../superpowers/specs/2026-10-08-idp-on-actions-design.md#6-pipelines)

## Context

Firestartr reconciles with a Kubernetes operator; this project must not require a cluster.

## Decision

Reconcile on GitHub Actions events: PR, merge, manual dispatch, and a daily drift report.

## Consequences

- No continuous reconciliation.
- Drift is reported, never auto-fixed.
- Zero infrastructure to run.

## Alternatives considered

- Kubernetes operator (Firestartr): level-triggered, but needs a cluster.
