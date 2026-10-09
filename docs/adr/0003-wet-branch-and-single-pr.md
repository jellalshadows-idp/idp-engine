# 0003. `wet` branch and a single PR

- Status: Accepted
- Date: 2026-10-08
- Spec: [§3.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#32-flow)

## Context

Claims are DRY: a few lines of YAML expand into repos, rulesets, environments, files and cloud resources. Someone has to be able to see what a change will really do, and the platform needs a place to keep the rendered output and the state. One common answer is two PRs: a claims PR, then a "hydrate" PR with the rendered output in a separate state repo. That gives a reviewable rendered diff, but the developer follows two PRs, and the second only exists after the first is merged.

The forces here: a single maintainer, a public portfolio where a reviewer should be able to follow a change end to end, and a strong rule that nothing is applied that nobody saw (§6). The rendered output is deterministic (ADR-0005), so it can be recomputed at any time and compared with what was last applied.

There is also a trust problem. The branch that records "what was applied" must not be writable by the people proposing changes, otherwise a PR author could forge the record the pipeline trusts.

## Decision

The claims repo has two long-lived branches (spec §3.1, §3.2):

- `main` is the desired state, the DRY claims. Changes arrive by **one PR**.
- `wet` holds `rendered/`, the encrypted GitHub state (`tfstate/github.tfstate`) and `.idp/manifest.json` (§5.4). It is written **only by `idp-writer`**, with no force push and no deletion (§7.4).

On the PR, the pipeline renders and diffs the result against `wet`, plans, and posts a sticky comment (§6.1). On merge, `reconcile` re-renders against the **current** `wet`, plans, gates, applies, and makes **one commit to `wet`** (§6.2). There is no second PR. A stack that failed keeps its old render in `wet`, so the next run sees the difference and retries.

## Consequences

### Positive

- One PR for the developer; the rendered diff and the plan appear as a sticky comment on that same PR.
- `wet` is a faithful record of what was applied, because it is written only after an apply and only by `idp-writer`.
- `internal/wetdiff` compares the new render with `wet` to find changed and orphaned stacks (§3.3), which also gives the self-healing retry.
- Reviewing a `wet` diff shows exactly what will land in a repo, including feature files (§5.7).

### Negative / costs

- `wet` is machine-written: no human reviews its commits before they land, only the plan on the PR. The safeguards are the gate (§6.2) and the ruleset that restricts who can write (§7.4).
- The PR plan can be stale by merge time. Reconcile therefore re-plans against current `wet` and compares fingerprints with the PR comment to choose `auto` or `approval` (§6.2).
- The history of `wet` mixes rendered files and state, so restoring state is not a plain revert (ADR-0002).

### Follow-ups

- Phase 1 delivers the diff/orphan logic and the pipelines; the `wet` ruleset comes from bootstrap (§10, ADR-0011).
- The architecture docs must record "a single PR" as a deliberate choice (§9.3).

## Alternatives considered

- **Two PRs: claims, then hydrate.** Rejected: more ceremony for a single maintainer, and the reviewer of the claim never sees the rendered result before merging. Showing it on the first PR does the same job.
- **Render in CI and never store it.** Rejected: nothing would record what was last applied, so orphans (claims deleted from `main`) and failed stacks could not be detected.
- **Commit rendered output to `main`.** Rejected: it mixes human and machine files in the protected branch, and `main` has no bypass at all (§7.4).
- **A separate state repo.** Rejected for v1: more repos to wire up for no extra protection, since a branch with a writer-only ruleset gives the same guarantee.

## References

- Spec §3.1-§3.3, §5.4, §5.7, §6.1, §6.2, §7.4.
