# 0002. Encrypted OpenTofu state in Git

- Status: Accepted
- Date: 2026-10-08
- Spec: [§7.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#72-encrypted-state)

## Context

The GitHub stack needs durable state, with no cloud backend available.

## Decision

Keep the GitHub stack's state in `wet`, encrypted with OpenTofu (PBKDF2 + AES-GCM, `enforced = true`).

## Consequences

- Authenticated encryption detects tampering.
- Every commit is a state version.
- The passphrase must never be lost.

## Alternatives considered

- An S3 backend (no real AWS in v1).
- Plaintext state in a public repo.
