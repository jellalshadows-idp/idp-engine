# 0009. Two GitHub Apps and split environments

- Status: Accepted
- Date: 2026-10-08
- Spec: [§7.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#73-two-github-apps-and-three-secret-scopes)

## Context

Required reviewers gate every job that references an environment.

## Decision

Reader App for plans; writer App key only in `idp-write`; approval in a secret-less `idp-approval`.

## Consequences

- PR code never sees a write token.
- One approval per run.

## Alternatives considered

- One App with the key in the approval env (several approvals per run).
