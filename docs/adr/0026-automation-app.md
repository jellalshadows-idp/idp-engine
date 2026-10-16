# 0026. An automation App for releases, dependency updates and the end-to-end harness

- Status: Accepted
- Date: 2026-10-13
- Spec: [§7.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#73-two-github-apps-and-three-secret-scopes), [§8.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#85-level-4-e2e), [§9.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#91-releases)

## Context

Three jobs need to push branches and open or merge pull requests: release-please in `idp-engine`, Renovate in the engine and claims repos, and the end-to-end harness in `idp-claims-e2e`. Pull requests opened with a workflow's `GITHUB_TOKEN` do not trigger workflows, so a release PR would get no CI, and letting Actions open pull requests needs the org setting that also lets Actions approve them, which spec §9.2 keeps off because it weakens required reviews. The reader App is read-only by design; the writer App's key is the apply key and lives only in the claims repos' `idp-write` environments.

## Decision

- A third App, `<org>-bot`, is created with `idp bootstrap app --role bot` from the manifest `bootstrap/apps/bot.json`. Repository permissions: actions write, checks read, contents write, issues write, metadata read, pull requests write, statuses read, workflows write; organization permission: members read; no events.
- It is installed on **only** `idp-engine`, `idp-claims` and `idp-claims-e2e`.
- Its key lives in `idp-engine` environments with deployment branch `main` (`idp-e2e` now; the release and Renovate environments in Phase 1e). Each job mints a token for the repositories and permissions it needs (ADR-0023).
- It is not a bypass actor anywhere: its pull requests in claims repos pass `idp-gate` like anyone's.

## Consequences

### Positive

- Its pull requests trigger CI, and no org setting is weakened.
- One identity for automation, limited to three repositories, with its permissions reviewable in the repo.

### Negative / costs

- One more key to rotate (bootstrap runbook).
- It can merge pull requests in `idp-claims` once Renovate uses it (Phase 1e). The `idp-gate` check and the approval of destructive changes still apply; requiring a human review on claims repos remains the owner's decision (ADR-0022).

## Alternatives considered

- **`GITHUB_TOKEN` with "Allow GitHub Actions to create and approve pull requests".** Rejected: release PRs would not trigger CI, and the setting weakens required reviews (spec §9.2).
- **A personal access token.** Rejected: personal, long-lived, and spec §9.1 rules out a PAT for releases.
- **The hosted Renovate App.** Rejected by the owner: a third party with write access to the repos.
