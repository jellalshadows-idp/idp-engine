# 0024. A failed wet push is recovered from the run's own artifact

- Status: Accepted
- Date: 2026-10-11
- Spec: [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main), [§7.7](../superpowers/specs/2026-10-08-idp-on-actions-design.md#77-runbooks)

## Context

After an apply, the runner holds the only copy of the new state until `idp wet-push` commits it (spec §6.2). If the push fails (an API error, an expired token, a lost runner), the next reconcile would plan against the old state in `wet` while the real resources already changed. Re-running the apply job has the same problem: it would apply its saved plan onto the state that plan read, even when `wet` moved since.

## Decision

- Before every push, `apply-github` writes the exact tree it commits (`tfstate/github.tfstate`, plus `rendered/github` when the apply succeeded or had nothing to apply) and a `wet-base` file with the `wet` commit its plan read. It uploads that tree as the artifact `idp-wet-root`, kept 7 days, hidden files included.
- If the push fails, the job opens one issue labelled `idp-wet-push`, titled "Wet push failed", that links the runbook, and the run fails.
- The `recover` workflow (a dispatch with the run id, in environment `idp-write`) checks that the run is a failed reconcile of `main`, downloads its `idp-wet-root`, refuses when `wet` no longer points at `wet-base`, then commits the tree with the wet-commit token (ADR-0023) and closes the issue.
- A reconcile run is never re-run: `plan` and `apply-github` refuse a second attempt, and `apply-github` refuses when `wet` moved since its plan. A failed reconcile is retried with a new dispatch.

## Consequences

### Positive

- Recovering is one dispatch: no local tools, no passphrase, no key.
- Neither the recovery nor a re-run can overwrite a newer state.

### Negative / costs

- When `wet` moved, or after 7 days, the recovery refuses, and the state must be rebuilt with `import` blocks (spec §7.7).
- Artifacts of a public repo are public. The state in them is encrypted, like the state in `wet` (ADR-0002).
- A whole-run re-run after a transient plan failure is refused too; the runbook says to dispatch instead.

## Alternatives considered

- **Retry the push inside the job.** Rejected as the only answer: it does not cover an expired token or a lost runner, so the recovery path is needed anyway.
- **Recover by hand with the passphrase.** Kept only as the fallback when `wet` moved.
