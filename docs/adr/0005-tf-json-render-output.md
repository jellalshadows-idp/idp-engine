# 0005. `.tf.json` render output

- Status: Accepted
- Date: 2026-10-08
- Spec: [§5.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#52-output-format-tfjson)

## Context

The renderer must be deterministic and testable.

## Decision

Emit `.tf.json` via `encoding/json`, which sorts map keys.

## Consequences

- Byte-stable golden tests.
- Humans read plans, not JSON.

## Alternatives considered

- Generating HCL text with templates.
