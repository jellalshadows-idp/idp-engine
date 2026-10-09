# 0014. Single org, isolated by repo and prefix

- Status: Accepted
- Date: 2026-10-09
- Spec: [§3.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#31-repositories), [§8.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#85-level-4-e2e)

## Context

The plan called for a separate sandbox org for E2E and spikes.

## Decision

Owner decision (2026-10-09): one org; tests are isolated by claims repo (`idp-claims-e2e`, own `wet`, state and passphrase) and by the `e2e-`/`spike-` prefix.

## Consequences

- Simpler: two Apps instead of four, one bootstrap of the org settings.
- Apps, keys, org settings and rate limit are shared between production and tests.
- The harness must refuse to delete unprefixed resources.
- `idp-claims` must reject prefixed names.

## Alternatives considered

- A sandbox org per environment (stronger isolation, double the Apps and bootstrap).
