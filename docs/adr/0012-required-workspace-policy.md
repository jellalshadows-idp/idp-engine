# 0012. Required Workspace policy

- Status: Accepted
- Date: 2026-10-08
- Spec: [§4.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#45-workspace)

## Context

Firestartr defaults to `observe`, so a forgotten field silently does nothing.

## Decision

`policy` is required; only `full-control` may delete.

## Consequences

- Explicit intent in every Workspace.
- Deleting a non-`full-control` workspace takes two PRs.

## Alternatives considered

- Defaulting to `observe`.
- Defaulting to `full-control`.
