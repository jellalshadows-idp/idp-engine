# 0023. Pipeline tokens are minted per step with the narrowest scope that works

- Status: Accepted
- Date: 2026-10-11
- Spec: [§7.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#73-two-github-apps-and-three-secret-scopes)

## Context

The pipelines use the reader App, the writer App and the workflow's `GITHUB_TOKEN` (spec §7.3). `actions/create-github-app-token` mints a token for the whole installation unless `repositories` narrows it, with all of the App's permissions unless `permission-*` inputs narrow them. zizmor's `github-app` audit flags both defaults.

## Decision

| Token | Used by | Repositories | Permissions |
|---|---|---|---|
| Reader | PR plan (no refresh), drift `bootstrap check` | the whole installation | all of the reader App's, which are read-only by design |
| Writer, plan and apply | `tofu plan` and `tofu apply` in reconcile; `tofu plan` in drift (ADR-0025) | the whole installation | exactly the writer manifest's: administration, contents, environments, members and workflows, all write |
| Writer, wet commit | `idp wet-push` in `apply-github` and `recover` | the claims repo only | `contents: write` |
| `GITHUB_TOKEN` | PR comment, issues, gate reads, artifact download | the claims repo | per job, as listed in [docs/pipelines.md](../pipelines.md) |

- Every token is set on the one step that uses it, never at job or workflow level.
- `idp comment` and `idp issue` run with `GITHUB_TOKEN` and never with `GH_TOKEN` set, so the bot authors what the gate trusts (ADR-0018).
- The apply token is minted right before the apply, and only when the plan has changes.
- The reconcile, recover and drift callers pass `IDP_WRITER_PRIVATE_KEY` by name (`${{ secrets.IDP_WRITER_PRIVATE_KEY }}`), as GitHub requires for an environment secret used in a reusable workflow; the value resolves only in the jobs that reference `idp-write`, whose deployment branch is `main`. The reusable workflows declare it required, so a caller that forgets it fails at the call. The pr caller never passes it, and a contract test enforces both.

## Consequences

### Positive

- The token that commits to `wet` cannot touch any other repository.
- A contract test pins the permissions of the writer plan and apply tokens to `bootstrap/apps/writer.json`, so they cannot drift apart.

### Negative / costs

- The reader token is not narrowed. It serves the PR plan, which does not refresh (ADR-0025), and the drift `bootstrap check`, which reads repository variables; those need the App's `actions_variables` permission, and `create-github-app-token` v3.2.0 declares no input for it, so a narrowed token would break the check. The App itself is read-only. Each reader step carries a zizmor inline ignore that says so.
- The writer plan and apply tokens span every repository, because the claims may declare any repository of the org. Each of those steps carries a zizmor inline ignore.

## Alternatives considered

- **One writer token for the apply and the commit.** Rejected: the commit needs one repository and one permission.
- **The provider's `app_auth` with the key file.** Rejected in ADR-0013: minted tokens fit the measured apply times.
