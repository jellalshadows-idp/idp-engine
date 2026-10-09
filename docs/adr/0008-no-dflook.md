# 0008. No dflook actions

- Status: Accepted
- Date: 2026-10-08
- Spec: [§6](../superpowers/specs/2026-10-08-idp-on-actions-design.md#6-pipelines)

## Context

The gate needs fingerprint subsets, policies and one commit per run.

## Decision

Own steps plus `idp plan-summary` instead of the dflook actions.

## Consequences

- More code, all of it testable.
- Removes the undocumented-encryption risk.

## Alternatives considered

- dflook tofu-plan/apply.
