# 0017. The drift check runs with the reader token

- Status: Accepted
- Date: 2026-10-10
- Spec: [§6.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#64-drift-daily-cron), [§9.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#92-bootstrap-outside-the-idp-on-purpose)

## Context

The daily drift workflow (spec §6.4) runs `idp bootstrap check` unattended. Two facts from Phase 0 shape how:

1. **The reader token cannot see ruleset bypass actors** (finding I3, ADR-0013). GitHub returns `bypass_actors` only to callers with write access to the ruleset. Since PR #3, `check` fails closed: a hidden list is reported as drift. With the reader token, every daily run would therefore report one finding per ruleset that nobody can act on.
2. **`check` needs identities that today come from local files.** These are the App ids, slugs and client ids, plus the approver. They live in `~/.idp/apps/*.json` on the owner's machine, which a workflow does not have.

## Decision

- **Token.** Drift runs `check` with the **reader** token and `--allow-hidden-bypass`. A ruleset whose `bypass_actors` the token cannot see is reported as a *notice* ("not verifiable with this token"), not as drift. Everything else is verified as before.
- **Identities.** `idp bootstrap apply` records them in a repository variable, `IDP_BOOTSTRAP`, as compact JSON: `{"approverId":42,"reader":{"id":1,"clientId":"Iv-…","slug":"…-reader"},"writer":{"id":2,"clientId":"Iv-…","slug":"…-writer"}}`. Nothing in it is secret. `check` verifies the variable like any other.
- **Reading the identities.** `idp bootstrap check --params-env IDP_BOOTSTRAP` reads the identities from that environment variable instead of from `--approver/--reader/--writer`.
- **Owner check.** Bypass lists stay verified by the owner-run `check`, which uses an owner token (runbook), and by every `apply`.

## Consequences

### Positive

- No write-capable key is ever present in a scheduled job.
- The daily drift run has no permanent false positive.

### Negative / costs

- **Accepted risk:** the daily run cannot see a bypass actor that an org admin adds to `idp-main`. Such a change needs admin rights, and an admin could also disable the drift workflow itself, so it is outside what drift can defend against. The owner-run `check` is the control.
- `apply` writes one more variable; existing claims repos pick it up on their next `apply`.

## Alternatives considered

- **The writer token, scoped down to `administration: write`.** Rejected. It puts the ability to *change* rulesets into a daily cron job in order to read one field.
- **A third App with `administration: write`.** Rejected: it has the same capability, plus another private key to protect.
- **The org audit log.** Rejected: its API needs GitHub Enterprise.
- **Keep failing closed.** Rejected: a permanent daily false positive trains people to ignore the drift issue.
