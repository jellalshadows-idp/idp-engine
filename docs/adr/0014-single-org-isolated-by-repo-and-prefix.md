# 0014. Single org, isolated by repo and prefix

- Status: Accepted
- Date: 2026-10-09
- Spec: [§3.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#31-repositories), [§8.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#85-level-4-e2e)

## Context

The platform needs an end-to-end harness that creates, modifies and deletes real GitHub resources: a repo, a team, a ruleset, environments and files (spec §8.5). Phase 0 spikes also create real resources. Running those against production claims would mix test and production state, so some isolation is needed.

The original plan (2026-10-08) was a separate sandbox org. The owner dropped it on 2026-10-09 (§12). One reason that follows from the constraints: Free orgs cannot be created through the API, so every org is a manual step (§12), and a second org also needs its own Apps and secrets. With a bus factor of one, a second org doubles the manual setup and the surface to maintain.

The question is therefore what isolation is achievable with one org, and what is honestly lost.

## Decision

Everything lives in **one org**, `jellalshadows-idp` (spec §3.1, §12). Production and tests are separated **by claims repo, not by org**, with isolation enforced three ways (owner decision, 2026-10-09; §8.5):

- `idp-claims-e2e` has its own `wet` branch, encrypted state and passphrase, so production and test state never mix (ADR-0002). It has the same shape as `idp-claims` but only `e2e-`-prefixed resources, and uses `archiveOnDestroy: false`.
- Every resource created by the harness or a spike is prefixed `e2e-` or `spike-`, and the harness **refuses to delete anything without that prefix**.
- `idp-claims` validation **rejects claim names that start with `e2e-` or `spike-`**.

The two claims repos share the same two GitHub Apps, because Apps are installed per org (§7.3). The harness runs nightly and on engine release PRs, not on every PR (§8.5).

## Consequences

### Positive

- One org to create and maintain by hand, instead of two.
- Test state and production state are separate per repo, with separate passphrases.
- Prefix rules fail safe in both directions: production cannot claim test names, and the harness cannot delete production resources.
- The nightly E2E exercises the same Apps and reusable workflows that production uses.

### Negative / costs

What a sandbox org would have added is accepted as lost (spec §8.5):

- The Apps and their keys are shared between production and tests, so a compromise of one affects both.
- Org-level settings (Actions permissions, invitations) are shared.
- The rate limit is shared, which is why the E2E runs nightly and on release PRs rather than every PR.
- Safety rests on prefixes and validation code, not on a hard boundary. A bug in the prefix guard could touch production resources.

### Follow-ups

- Phase 1 creates the E2E harness; its exit criterion is creating, modifying and deleting an `e2e-` repo and team through `idp-claims-e2e` (§10).
- Bootstrap must create both claims repos and wire their passphrases (§9.2, ADR-0011).
- Phase 0 records pending-invite behavior using the test account `adrian-da-silva` (§10, §12).

## Alternatives considered

- **A separate sandbox org.** Rejected by the owner on 2026-10-09: Free orgs cannot be created through the API, and it would double Apps, secrets and settings to maintain. Its extra isolation (separate Apps, org settings, rate limit) is the accepted loss above.
- **No separation: run tests against `idp-claims`.** Rejected: test state and production state would mix, and a failed test could leave production claims dirty.
- **Separate branch only, same repo.** Not chosen: a repo gives its own `wet`, state, passphrase and ruleset, which a branch would share.

## References

- Spec §3.1, §7.3, §8.5, §9.2, §10, §12.
