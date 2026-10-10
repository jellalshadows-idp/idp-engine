# 0019. Wet commits through the Git Data API

- Status: Accepted
- Date: 2026-10-10
- Spec: [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main)

## Context

Reconcile ends with **one** commit to `wet` (spec §6.2 step 4.3):

- the GitHub state, always;
- the render of each stack that applied, and only those;
- a deletion of anything a stack no longer renders.

Only the writer App may update `wet` (ruleset `idp-wet`).

## Decision

`idp wet-push` makes that commit with the writer token, through the Git Data API:

1. Read the branch head and its tree. A truncated tree is an error.
2. For each synced path, compare the local files with the remote blobs. Unchanged files are skipped by their git blob hash; `.terraform/` directories are never synced.
3. Upload only changed files as blobs, and delete remote files that are no longer present locally.
4. Create one tree on top of the old one, one commit, and update the ref **without force**.
5. Nothing changed → no commit.

Safeguards:

- `--root` must exist and be a directory.
- A synced path that is a single file on the branch is never deleted; if it is missing locally that is an error.
- `.terraform` paths are rejected.

## Consequences

### Positive

- Commits are made by the App through the API, so GitHub signs them, shows them as **Verified**, and attributes them to the writer App.
- No git credentials are written to the runner's disk or process arguments.
- The behaviour is unit-tested against the fake GitHub.

### Negative / costs

- A concurrent update of `wet` makes the push fail. Reconcile and drift share the `idp-wet` concurrency group, so this should never happen; if it does, the failure is loud and the runbook covers it.

## Alternatives considered

- **`git push` from the runner with the token.** Rejected:
  - the token ends up in git config or command arguments;
  - the commits are unsigned;
  - the logic would be shell that is not tested.
- **The contents API.** Rejected: it makes one commit per file, which breaks "one commit".
