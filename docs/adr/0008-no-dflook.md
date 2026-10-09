# 0008. No dflook actions

- Status: Accepted
- Date: 2026-10-08
- Spec: [§9.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#93-docs), plus [§3.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#33-units) and §6.1 (where `idp plan-summary` lives)

## Context

Plan and apply on a PR is a solved problem in the Terraform world, and the dflook `tofu-plan` and `tofu-apply` actions are the popular answer: they post the plan as a PR comment and refuse to apply if the plan changed relative to that comment. That is close to the gate this platform needs.

The platform's requirements go further, though. The pipeline must (spec §6): compute a **fingerprint** (the sorted `(address, action)` pairs per stack, excluding `no-op` and `read`) and embed it in a sticky comment; decide `auto` versus `approval` by checking that each stack's fingerprint is a **subset** of the one in the PR comment; enforce the Workspace policy table (ADR-0012) from `tofu show -json`; and require apply to match the gated fingerprint exactly. It also runs plans against a **local backend with encrypted state** (`TF_ENCRYPTION`, ADR-0002) and, for AWS, inside a floci service container with replayed state. The behavior of dflook with the local backend plus `TF_ENCRYPTION` was undocumented, and an undocumented interaction at the center of the security gate is not something to depend on.

The project is also supply-chain conscious: actions are pinned by SHA, `zizmor` and `actionlint` run on workflows, and every dependency must be at least 14 days old (§7.5). Each third-party action in the gate path widens that surface.

## Decision

The plan, gate and apply logic is implemented in the project's own Go CLI, `idp`, and plain `tofu` commands, with **no dflook actions** (spec §3.3, §6).

- `idp plan-summary` reads `tofu show -json` and produces the policy verdict, the fingerprint, the Markdown summary and the gate decision input (`internal/plan`, §3.3). `idp gate` makes the `auto`/`approval` decision (§6.2).
- The sticky comment is posted with `GITHUB_TOKEN` and `pull-requests: write` (§6.1).
- GitHub apply uses the saved, encrypted plan; OpenTofu itself refuses a stale plan ("Saved plan is stale"), also when encrypted (§6.2).
- The CLI is released with checksums and a build provenance attestation, and the workflows run `gh attestation verify` before executing it (§7.5).

## Consequences

### Positive

- The gate logic (fingerprints, subset check, policy table) is unit-tested Go with fixtures from real `tofu show -json` output (§8.2).
- No dependence on an action's undocumented behavior with the local backend plus `TF_ENCRYPTION`.
- One less third-party action in the path that decides what gets applied.
- The same code serves PR, reconcile and drift.

### Negative / costs

- The project owns code that a popular action already provides in part: comment formatting, plan parsing, and their edge cases.
- The stale-plan protection comes from OpenTofu's own check, not from dflook's comparison to the PR comment; the fingerprint comparison replaces that role.
- Parsing `tofu show -json` couples the CLI to OpenTofu's plan JSON format, and the OpenTofu version is pinned exactly (≥ 1.12, chosen in Phase 0, §7.5).

### Follow-ups

- Plan-summary fixtures are real `tofu show -json` outputs captured once in CI and committed (§8.2).
- The ADR on spike findings (ADR-0013) records the Phase 0 choice of OpenTofu version.

## Alternatives considered

- **dflook `tofu-plan` / `tofu-apply`.** Rejected: it does not compute the subset-of-fingerprint gate or enforce the Workspace policy table, and its behavior with the local backend and `TF_ENCRYPTION` was undocumented. It does refuse to apply if the plan changed against the PR comment, which is close to, but not the same as, the gate required here.
- **Wrapping dflook and adding the fingerprint on top.** Rejected: it would still sit on the unverified path (encryption plus local backend) and add a dependency while leaving the real logic to our own code.
- **Plain `tofu plan` text pasted into the comment.** Rejected: no structured verdict, no stable fingerprint, and no way to flag destructive actions.

## References

- Spec §3.3, §6.1, §6.2, §7.5, §8.2, §9.3.
- Verified: dflook refuses to apply if the plan changed vs the PR comment; its behavior with local backend + `TF_ENCRYPTION` is undocumented. OpenTofu stale saved plans are refused, also when encrypted.
