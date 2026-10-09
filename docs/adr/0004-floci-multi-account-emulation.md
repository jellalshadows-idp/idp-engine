# 0004. floci multi-account emulation

- Status: Accepted
- Date: 2026-10-08
- Spec: [§5.6](../superpowers/specs/2026-10-08-idp-on-actions-design.md#56-emulator-specifics-belong-to-ci-not-to-the-render)

## Context

AWS is emulated; envs map to accounts.

## Decision

One floci per job; the 12-digit access key ID selects the account.

## Consequences

- Real-looking ARNs.
- Nothing persists between jobs, so stacks are replayed from `wet`.

## Alternatives considered

- One floci per env on separate ports: the original design, replaced once floci's multi-account support was verified.
