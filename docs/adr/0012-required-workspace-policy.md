# 0012. Required Workspace policy

- Status: Accepted
- Date: 2026-10-08
- Spec: [§4.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#45-workspace)

## Context

A `Workspace` claim runs a generic OpenTofu module (v1 ships `s3-bucket`) in a given environment. Unlike the Group and Component kinds, whose resources the platform fully understands, a Workspace is arbitrary module code, so the platform cannot know in advance which changes are safe. The question is how much the pipeline is allowed to do with each one: only plan, create and update, or also destroy.

Firestartr models this with policies (`full-control`, `apply`, `observe`, `create-only`) and defaults to `observe` (spec §13). A default is attractive because it is safe, but it hides the decision: a claim author who forgets the field silently gets a Workspace that is planned and never applied, and the claim looks done. The opposite default, `full-control`, would silently allow destroy.

The deletion path matters too. When a claim file is deleted, there is no claim left to read the policy from, so the policy has to come from the **last render in `wet`** (§6.3). The policy therefore also decides what a deletion means.

## Decision

Workspace claims carry a **required** `policy` field, with no default. Validation fails if it is missing (spec §4.5). Three policies are supported; `create-only` is not in v1 (§13).

| Policy | Create/update | Delete or replace | Applied? |
|---|---|---|---|
| `full-control` | yes | yes, with approval (`idp-approval`, §6.3) | yes |
| `apply` | yes | **PR fails** | yes |
| `observe` | none | none | never; plan-only |

- `idp plan-summary` enforces the table on `tofu show -json` (§6.1).
- On deletion of the claim file, the policy comes from the last render in `wet`: `full-control` plans a destroy and requires approval; `apply` fails the PR, so deleting takes two PRs (change the policy, then delete the claim); `observe` removes its render from `wet` with no destroy (§6.3).
- Any delete or replace goes through `idp-approval` (§6.3), and `observe` Workspaces are never applied: a successful plan counts as success (§6.2).

## Consequences

### Positive

- The claim author makes the destructive-power decision explicitly; it is visible in the PR diff and in the claim.
- `apply` gives a safe middle: the platform can create and update but can never destroy by accident.
- `observe` lets existing infrastructure be tracked without being touched.
- One table drives validation, the PR check and the deletion behavior, and it is table-driven testable (§8.2).

### Negative / costs

- Slightly more friction: every Workspace needs a policy, even trivial ones.
- Deleting an `apply` Workspace takes two PRs. That is intended friction, but it is friction.
- Differs from Firestartr (which defaults to `observe`), so users coming from it must be told (§13, §9.3).
- No `create-only` policy in v1.

### Follow-ups

- Phase 3 delivers the allowlist, policies, orphans and `idp-modules/s3-bucket`; its E2E covers the two-step delete and the `apply`-policy rejection (§10).
- The claims reference should document the table and the two-PR deletion flow.

## Alternatives considered

- **Default to `observe`, as Firestartr does.** Rejected: a forgotten field silently yields a Workspace that is never applied, which hides a decision that should be explicit.
- **Default to `full-control`.** Rejected: destroy power would be granted by omission.
- **Only a global setting, not per claim.** Not chosen: different Workspaces in one repo need different safety levels.
- **Include `create-only`.** Deferred: not in v1 (§13).

## References

- Spec §4.5, §6.1-§6.3, §8.2, §9.3, §10, §13.
