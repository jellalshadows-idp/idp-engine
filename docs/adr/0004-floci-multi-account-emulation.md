# 0004. floci multi-account emulation

- Status: Accepted
- Date: 2026-10-08
- Spec: [§5.6](../superpowers/specs/2026-10-08-idp-on-actions-design.md#56-emulator-specifics-belong-to-ci-not-to-the-render)

## Context

v1 supports AWS only as an **emulated** target (spec §2.1, §2.2): there are no real AWS accounts and no real cloud credential anywhere (§7.6). The claims and config are shaped for real AWS, with 12-digit account IDs and regions per environment (§4.6), so that going live changes only the target.

The platform needs an AWS emulator that runs in CI. The owner has no local Docker, so anything that needs a container runtime runs in CI only (§2.3). The environments (`dev`, `staging`, `pro`) are different AWS accounts in the config, and the ARNs that the GitHub side consumes embed the account ID (ADR-0006). If the emulator collapsed all environments into one account, ARNs would be wrong and cross-environment mistakes would be invisible in tests.

floci has a documented behavior that fits: a 12-digit access key ID is used directly as the account ID, with per-account isolation (floci docs, `multi-account.md`). floci also implements `CreateOpenIDConnectProvider`, which the per-env baseline needs (§5.3, §11 risk 4). Its ECR needs the Docker socket, which is another reason it runs only in CI.

## Decision

AWS is emulated with **floci, one instance serving every environment**, and emulator specifics belong to CI, never to the render (spec §5.6).

- The workflow starts floci as a service container, image **pinned by digest**, on port 4566.
- It sets `AWS_ACCESS_KEY_ID=<accountId>` per job, using the account ID from `config/platform.yaml`. floci isolates resources per account, so ARNs come out right for each env.
- It writes `floci_override.tf.json` with the endpoint and provider flags. Override files are native to OpenTofu.
- Rendered stacks do not know floci exists. The platform config holds account IDs and regions, never emulator ports (§4.6).
- Each job brings up its own floci and replays state from `wet` (§6.1, §6.2). Going to real AWS means replacing this CI step with OIDC credentials; claims and renders do not change.

## Consequences

### Positive

- One emulator for all envs, with correct per-env ARNs and account isolation.
- The same rendered stacks run unchanged against real AWS later, so the emulation is a CI detail rather than a design fork.
- No real credentials: floci uses fake 12-digit keys (§7.6).

### Negative / costs

- Emulated AWS cannot drift and keeps no state of its own, so every job must replay the stack from `wet`. The cost grows with the number of stacks, and this is measured in Phase 0 (spec §11 risk 3, ADR-0013).
- Fidelity gaps in IAM and ECR are possible. The OIDC provider is confirmed to exist; other gaps are documented and skipped in level-3 tests (§11 risk 4).
- Nothing about floci can run locally without Docker, so levels 3 and 4 of the test pyramid are CI-only (§8).
- A component's own CI cannot push to the ephemeral ECR, so the `container-ci` `ecr` path is only checked statically (§5.7).

### Follow-ups

- Phase 0 measures floci replay time per stack (ADR-0013); Phase 2 delivers the AWS part and floci integration tests (§10).
- Level 3 includes a replay test: apply the old version, plan the new one, assert the exact diff (§8.4).

## Alternatives considered

- **Real AWS accounts.** Rejected for v1: out of scope (§2.2); it would require real credentials and cost, against the portfolio constraints.
- **One floci instance per environment.** Rejected: unnecessary, since floci already isolates per account via the access key ID.
- **Putting emulator endpoints into the rendered stacks or config.** Rejected: it would tie renders and claims to the emulator and force a re-render to go live. Keeping it in `floci_override.tf.json` keeps the render real-AWS shaped.
- **Another emulator.** Not evaluated beyond the facts verified for floci (multi-account, OIDC provider); the spec records no comparison.

## References

- Spec §2.1-§2.3, §4.6, §5.6, §7.6, §8, §11 (risks 3 and 4).
- Verified: floci `multi-account.md`; `CreateOpenIDConnectProvider` implemented; ECR needs the Docker socket.
