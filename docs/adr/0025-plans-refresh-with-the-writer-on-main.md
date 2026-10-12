# 0025. Plans refresh with the writer token on main

- Status: Accepted
- Date: 2026-10-12
- Spec: [§6.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#61-pr-pull_request--main), [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main), [§6.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#64-drift-daily-cron), [§7.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#73-two-github-apps-and-three-secret-scopes), [§7.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#71-threat-model)

## Context

The GitHub API shows some repository fields only to a caller with admin rights on the repository. To any other token it returns them as `null` or leaves them out:

- `GET /repos/{owner}/{repo}`: `allow_merge_commit`, `allow_squash_merge`, `allow_rebase_merge`, `delete_branch_on_merge`, `merge_commit_title`, `merge_commit_message`, `squash_merge_commit_title` and `squash_merge_commit_message`;
- `GET /repos/{owner}/{repo}/rulesets/{id}`: `bypass_actors` (finding I3, ADR-0013).

The read-only reader App is in that position. Every plan used to refresh with the reader token (ADR-0023), so a refresh read those fields as false or absent, and OpenTofu planned an update for every Component to set them again. The PR comment showed changes the PR did not make, every reconcile re-applied no-ops, and the drift issue never closed.

The live smoke of Phase 1c showed it. Right after a clean apply, drift run [38302684806](https://github.com/jellalshadows-idp/idp-claims-e2e/actions/runs/38302684806) planned `github: +0 ~2 -0 ±0` (the smoke Component's repository and its default-branch ruleset) and opened a drift issue.

ADR-0017 rejected a write-capable token in the daily job when the only goal was to read one field for `bootstrap check`. The plan is a different case: it cannot be correct without those fields, for every Component.

## Decision

- **PR plan.** It keeps the reader token, but plans with `-refresh=false`. It compares the claims with the state the last apply wrote, and the writer refreshed that state, so the plan shows exactly what the PR changes. Live drift is the drift check's job, and the reconcile plan sees it.
- **Reconcile plan.** The `plan` job runs in environment `idp-write` (deployment branch `main`) and refreshes with a writer token minted with the writer manifest's permissions, the same set as the apply token. It sees every field, so a dispatch can remediate drift (spec §6.4). The reconcile no longer uses the reader key at all.
- **Drift plan.** The `drift` job runs in environment `idp-write` and refreshes with a writer token minted the same way. `idp bootstrap check --params-env IDP_BOOTSTRAP --allow-hidden-bypass` keeps the reader token, because it reads org settings that the writer App has no permission for; ADR-0017 stays valid for the check.
- **The writer key and PR runs.** The writer key never reaches a PR run (spec §7.1, amendment A7: PR plan tokens stay read-only). Two more jobs hold a write-capable token while `tofu plan` runs. Both run only on `main` and execute the same pinned engine modules that the apply already runs with that token.

The reconcile, recover and drift callers pass `IDP_WRITER_PRIVATE_KEY` by name, and the pr caller never does; a contract test enforces both (ADR-0023). Contract tests also pin the plan tokens to the writer manifest's permissions and the PR plan to `-refresh=false`.

## Consequences

### Positive

- Plans and drift are exact: a clean apply is followed by a clean drift run and an empty reconcile plan.
- A dispatched reconcile can remediate drift, as spec §6.4 says.
- The PR comment shows only the PR's change.

### Negative / costs

- The reconcile `plan` job and the `drift` job hold a write-capable token while `tofu plan` runs, on `main` only.
- The PR plan cannot see live drift. A reconcile that remediates drift therefore asks for approval, unless the drifted resource also appears in the PR's plan with the same action; then the revert to the claims is applied with it (ADR-0018).
- The drift job needs the writer key, which its caller must pass by name.

## Alternatives considered

- **`lifecycle { ignore_changes }` on those fields.** Rejected: `bypass_actors` holds the writer-only ruleset bypass, a security control, which would never be verified or corrected again.
- **No refresh anywhere, with only the drift plan on the writer.** Rejected: a dispatched reconcile could no longer remediate drift, contrary to spec §6.4.
- **A reader App with `administration: write`.** Rejected: it would no longer be read-only.
- **A narrower plan token** (administration write, the rest read). Not adopted for now: `administration: write`, the permission that reveals the hidden fields, is already the most powerful of the set, so the gain is small; it can be pinned after a live run proves the narrower set suffices.
