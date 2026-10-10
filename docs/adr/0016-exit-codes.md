# 0016. One exit-code convention for every command

- Status: Accepted
- Date: 2026-10-10
- Spec: [§3.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#33-units), [§6.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#64-drift-daily-cron)

## Context

Phase 0 finding M1: `idp bootstrap check` exits 1 both when it finds drift and when it cannot check at all, for example a network error or a bad token. The drift workflow must tell them apart. Drift opens or updates an issue; a failed check must fail the run loudly. `idp validate` has the same ambiguity: a broken claim and an unreadable directory both exit 1.

OpenTofu's `plan -detailed-exitcode` already uses 2 for "changes present", and Go's `flag` package uses 2 for usage errors. Both conventions appear in the same workflow logs.

## Decision

Every `idp` command uses four exit codes:

| Code | Meaning | Examples |
|---|---|---|
| 0 | The command did its job, and nothing needs attention | valid claims, no drift, a gate decision was made |
| 1 | The command could not do its job | unreadable file, API error, missing secret |
| 2 | The command line is wrong | unknown flag, missing required flag |
| 3 | The command did its job, and the answer needs attention | invalid claims (`validate`, `render`), drift (`bootstrap check`) |

The pipeline commands (`diff`, `plan-summary`, `gate`, `comment`, `issue`, `wet-push`, `encryption-env`) report their results through step outputs, such as `affected`, `decision` and `commit`, and exit 0 when they produced them. Whether a run continues is decided by the workflow from those outputs, not from exit codes.

## Consequences

### Positive

- The drift workflow can branch on 3 (open the issue) versus 1 (fail the job).
- Scripts that treat any non-zero code as failure keep working.

### Negative / costs

- `validate` and `render` with invalid claims, and `bootstrap check` with drift, move from exit 1 to exit 3, which is a visible behaviour change. The runbook and the claims reference document it.

## Alternatives considered

- **Keep 1 for both.** Rejected: that is finding M1.
- **Use OpenTofu's 2 for "found something".** Rejected: 2 already means a usage error in every Go CLI, so the two meanings would collide.
- **A distinct code per kind of finding.** Rejected as YAGNI: the step output carries the details.
