# 0011. Bootstrap outside the IDP

- Status: Accepted
- Date: 2026-10-08
- Spec: [§9.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#92-bootstrap-outside-the-idp-on-purpose)

## Context

The IDP must not manage the protections that guard it.

## Decision

`idp bootstrap apply/check`, run by an org owner; `check` runs in drift.

## Consequences

- Apps are created via the browser manifest flow.
- There is a small manual runbook.

## Alternatives considered

- Managing the claims repo through claims.
- A bootstrap OpenTofu stack (state chicken-and-egg).
