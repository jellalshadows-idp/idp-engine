# 0006. Deterministic identities

- Status: Accepted
- Date: 2026-10-08
- Spec: [§5.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#55-deterministic-identities-with-no-shared-state-between-stacks)

## Context

The GitHub stack needs AWS identifiers, but AWS state is ephemeral.

## Decision

Compute ARNs and URLs from the platform config by convention.

## Consequences

- Stacks are independent and plan in parallel.
- Names are constrained (≤ 40-character claims, ≤ 10-character envs).

## Alternatives considered

- `terraform_remote_state` between stacks.
