# 0007. User-managed files via `state rm`

- Status: Accepted
- Date: 2026-10-08
- Spec: [§5.7](../superpowers/specs/2026-10-08-idp-on-actions-design.md#57-features)

## Context

Some feature files must become the user's after creation.

## Decision

Create once, `state rm`, record in `.idp/manifest.json`, as Firestartr does.

## Consequences

- User edits are never reverted or deleted.
- Mode switches are rejected in v1.

## Alternatives considered

- `ignore_changes` (deletes on removal).
- A `removed` block (no instance keys).
