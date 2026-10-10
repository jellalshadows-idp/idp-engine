# 0018. Plan fingerprints and what the gate trusts

- Status: Accepted
- Date: 2026-10-10
- Spec: [§6.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#61-pr-pull_request--main), [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main)

## Context

The guiding rule of spec §6 is "nothing is ever applied that nobody saw". Reconcile applies without approval only when its plan is a subset of the plan that was shown on the pull request. That plan lives in a PR comment, and PR comments are public. Anyone with a GitHub account can add comments to a public repository's pull requests, so the gate must decide exactly which comment to believe.

## Decision

- **Fingerprint.** A fingerprint is, per stack, the sorted set of `(address, action)` pairs.
  - Actions are normalised to `create`, `update`, `delete`, `replace` (`delete,create` in either order) and `forget`.
  - `no-op` and `read` are excluded.
  - Any other action makes parsing **fail**. An unknown action is never treated as harmless.
- **Comment marker.** The plan comment carries `<!-- idp-plan -->` and one hidden marker: `<!-- idp-fingerprint:v1 sha=<head SHA> data=<base64(gzip(JSON))> -->`. Gzip keeps a plan of hundreds of resources within GitHub's comment size limit.
- **Which comment the gate trusts.** The gate (`idp gate`) trusts a fingerprint only when all of these hold:
  1. It is in the **newest** comment **written by `github-actions[bot]` (type `Bot`)** that contains `<!-- idp-plan -->`. That is the identity of the plan job's `GITHUB_TOKEN`. Comments by anyone else are ignored, so a stranger cannot even force an approval prompt.
  2. The comment is on the merged pull request whose `merge_commit_sha` is the pushed commit.
  3. The marker's `sha` equals that pull request's **head SHA**, so the comment describes the code that was merged.
- **Decision order:**
  1. No changes → `auto`.
  2. Any `delete` or `replace` → `approval`.
  3. No trusted fingerprint → `approval`.
  4. A change missing from the same stack of the trusted fingerprint → `approval`.
  5. Otherwise → `auto`.

## Consequences

### Positive

- Comments by anyone else and stale bot comments (whose `sha` is not the merged head) are ignored. When no trusted fingerprint is left, the worst case is an approval prompt, which fails safe.

### Negative / costs

- A pull request whose last plan run failed or was cancelled ends in an approval prompt after merge. This is intended.
- **Accepted risk**, alongside the passphrase risk of spec §7.3: the author check proves who wrote the comment, not that nobody edited it. (1) People with write access can edit the bot's comment, and the author stays `github-actions[bot]`. (2) Any workflow's `GITHUB_TOKEN` writes as `github-actions[bot]`, including a workflow added on a same-repo pull request branch. Both routes are open only to members who can already push branches (spec §7.1), and destructive changes always require approval regardless.

## Follow-up

- Phase 1c decides whether to reject comments edited by anyone other than the Actions bot (GraphQL `editor` / `lastEditedAt`).

## Alternatives considered

- **Keep the fingerprint in a workflow artifact.** Rejected: artifacts expire, and finding the right run of another workflow is fragile. The comment is literally what the reviewer saw.
- **Commit statuses or check runs.** Rejected: they have no room for the payload.
- **Plain JSON in the marker.** Rejected: hundreds of addresses exceed the comment limit.
