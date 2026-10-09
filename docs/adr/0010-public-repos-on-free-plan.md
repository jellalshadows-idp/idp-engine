# 0010. Public repos on the Free plan

- Status: Accepted
- Date: 2026-10-08
- Spec: [§2.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#23-hard-constraints)

## Context

The security design depends on GitHub features: rulesets that protect `main` and `wet` (spec §7.4), environments with required reviewers for approval (§6.2), wait timers, and environment secrets that keep the write key out of PR runs (ADR-0009). The GitHub org is on the **Free plan**, and on Free those features are available only on **public repositories** (verified against GitHub docs, repo rules and environments pages).

Private repositories would need the GitHub Team plan, which is a recurring cost for a personal portfolio project. The alternative of running with private repos but without these features would mean no enforced review, no approval gate and no secret scoping, which would defeat the point of a platform whose core promise is that nothing is applied that nobody saw (§6).

The consequence is not limited to the platform's own repos. The platform also creates Component repos, and the rulesets and environments it configures for them (§4.4) need the same features.

## Decision

**Every repository in this system is public, including the Component repos the platform creates** (spec §2.3). Private repositories are out of scope for v1 and tied to the GitHub Team plan (§2.2).

- The Component module always creates `public` repos and attaches the ruleset and environments (§4.4).
- The design assumes everything is readable by anyone: code, `wet`, Actions logs, artifacts and PR comments, and its threat model protects **who can write**, not who can read (§7.1).
- No secrets flow through the IDP in v1: claims have no secret fields, and floci uses fake credentials (§7.6).
- The README states the limitations up front (§9.3).

## Consequences

### Positive

- Rulesets, required reviewers, wait timers and environment secrets work at no cost.
- A reviewer can read everything: code, rendered output, pipeline logs. That helps the portfolio goal (§1).
- Forces a clear security posture: assume public, protect writes, limit blast radius of leaked credentials (§7.1).

### Negative / costs

- Nothing private can be managed by the platform in v1. A user cannot create a private Component repo.
- State, comments and logs are public, so the encrypted state buys integrity and readiness rather than confidentiality (ADR-0002).
- Public repos accept PRs from forks, so forks get only validate and render, and plan jobs require `head.repo.full_name == github.repository` (§6.1).
- Scheduled workflows in public repos are disabled after 60 days of inactivity (§6.4, §11 risk 6).

### Follow-ups

- Private repo support is a future change tied to the Team plan; the encrypted state is already prepared for it (§7.2).
- The claims reference and README must state that Component repos are always public.

## Alternatives considered

- **Upgrade to GitHub Team and use private repos.** Rejected: recurring cost not justified for a portfolio project, and public repos serve the audience (§1).
- **Private repos on Free without rulesets, approvals and secret scoping.** Rejected: the security gates would not be enforceable, and the central rule of the pipelines could not be guaranteed.
- **Public platform repos, but private Component repos.** Not possible on Free: the same plan limits apply to every repo the platform creates.

## References

- Spec §1, §2.2, §2.3, §4.4, §6.1, §6.2, §6.4, §7.1, §7.2, §7.4, §7.6, §9.3, §11.
- Verified: GitHub Free limits for rulesets, environment reviewers, wait timers and environment secrets (GitHub docs, repo rules and environments pages).
