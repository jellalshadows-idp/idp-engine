# 0022. The required check lives in the caller

- Status: Accepted
- Date: 2026-10-11
- Spec: [§6.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#61-pr-pull_request--main), [§7.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#74-rulesets)

## Context

The `main` ruleset of a claims repo requires one status check, `idp-gate` (spec §6.1, §7.4). A job inside a reusable workflow reports a check named `<caller job> / <job>`, so no engine job can report exactly `idp-gate`. A required check that is **skipped** counts as passing, and pull requests from forks skip the plan on purpose.

## Decision

- The reusable PR workflow ends with a `verdict` job that always runs. It fails pull requests from forks with "a maintainer must push this branch to the repo", and it fails unless `validate`, `plan` and `comment` all succeeded.
- The caller template has a job named `idp-gate` with `needs: pr`, `if: always()` and no permissions. It passes only when the reusable workflow's result is `success`.
- `if: always()`, never `!cancelled()`, keeps `idp-gate` from ever being skipped: a cancelled run reports a failing check, not a skipped one.
- The PR template has no path filters, so every pull request reports `idp-gate`. A filter would leave the required check pending forever on a PR that touches no claims.

## Consequences

### Positive

- The check name is stable, and the logic behind it is versioned with the engine.

### Negative / costs

- The mirror job lives in a workflow file of the claims repo, so a pull request can change it. This is the trust boundary ADR-0018 already accepts: members who can push branches can run workflows as `github-actions[bot]`, and while `required_approving_review_count` is 0 they can merge. The owner decides whether to require a review on `.github/` (CODEOWNERS plus one approval) and to pin `idp-gate` to the GitHub Actions integration in the ruleset (phase 1b owner note).

## Alternatives considered

- **A job named `idp-gate` inside the reusable workflow.** Rejected: its check would be `pr / idp-gate`.
- **Every reusable job as a required check.** Rejected: the names depend on the caller's job id, and skipped jobs pass.
- **Ruleset-enforced workflows that run a workflow from another repository.** They would take the check out of the PR's reach. Not evaluated in this phase; recorded for Phase 5.
