# 0003. `wet` branch and a single PR

- Status: Accepted
- Date: 2026-10-08
- Spec: [§3.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#32-flow)

## Context

Firestartr needs two PRs (claims, then hydrate).

## Decision

Render into a `wet` branch that only the reconcile writes; humans review one PR.

## Consequences

- Smaller loop.
- `wet` history shows exactly what was applied.

## Alternatives considered

- Two PRs as in Firestartr.
- Rendering into `main`.
