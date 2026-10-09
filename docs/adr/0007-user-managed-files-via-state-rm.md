# 0007. User-managed files via `state rm`

- Status: Accepted
- Date: 2026-10-08
- Spec: [§5.7](../superpowers/specs/2026-10-08-idp-on-actions-design.md#57-features)

## Context

Features install file bundles into Component repos as `github_repository_file` resources (spec §5.7). Most files should be **managed**: the platform owns them and reverts manual edits on the next apply. Some files, however, must be created by the platform once and then belong to the user or to another tool. The v1 example is `.release-please-manifest.json`, which release-please bumps itself.

OpenTofu has no native "create once, then let go". Firestartr solves it with `userManaged` files: the file is created, then removed from state so it is no longer tracked (`packages/gh_provisioner/src/entities/ghfeature/helpers/managed_files.ts:76-81`). This project keeps the same resource names and the same mechanism, replacing Firestartr's `installed_managed_files` output with `.idp/manifest.json` (§13).

The obvious OpenTofu-only tools both fail, which is why the decision needs a record. `lifecycle { ignore_changes = [content] }` keeps the resource in state, so removing the feature would **delete** the user's file, and deleting the file by hand would bring it back. A `removed` block would forget the resource without destroying it, but its `from` "cannot include instance keys" (verified), and these files use `for_each`.

## Decision

Feature files have two modes, and both are written to the default branch (spec §5.7):

- **`managed`**: `github_repository_file.managed[...]` with `overwrite_on_create = true`. Manual edits are reverted on the next apply.
- **`userManaged`**: `github_repository_file.user_managed[...]`, handled in four steps:
  1. The file is created **once**.
  2. Right after the apply, `tofu state rm` removes it from state, and `<component>:<path>` is recorded in `.idp/manifest.json`.
  3. Later renders skip it. If the file already existed, `idp adopt` records it as installed without writing anything.
  4. Manifest entries are removed when their Component is deleted.

`apply-github` runs the `state rm` and updates the manifest, then the manifest goes into the single `wet` commit (§6.2). `user_managed` uses `overwrite_on_create = false`, so if a file was deleted between adopt and apply the apply fails loudly and the next reconcile retries (§11 risk 5). Switching an existing file between modes is rejected by validation (§2.2, §4.7).

## Consequences

### Positive

- The user's file survives feature removal, manual deletion and later renders: the platform has truly let go of it.
- Same mechanism and names as Firestartr, so the comparison document can list `userManaged` as kept (§9.3).
- The manifest in `wet` is an auditable list of what the platform installed and then released.

### Negative / costs

- State and manifest must stay in step. The step is done by the pipeline after apply, so a failure between apply and `state rm` is a recovery case (runbook "unblock a failed apply with reconcile", §7.7).
- When a Component is deleted, the repo is archived and OpenTofu destroys dependents first, so the last commit deletes the `managed` files while `userManaged` files stay. Accepted for v1; avoiding it needs a two-step deletion (§6.3).
- The manifest is part of the render, so `adopt` needs network access with a read-only token (§5.1).

### Follow-ups

- Phase 4 delivers features, with E2E covering one managed and one userManaged file (§10).
- Phase 0 or later must confirm the adoption edge cases in risk 5 (§11).

## Alternatives considered

- **`lifecycle { ignore_changes = [content] }`.** Rejected: removing the feature would delete the user's file, and deleting the file by hand would recreate it.
- **A `removed` block.** Rejected: `from` cannot include instance keys, and these files use `for_each`.
- **Treat every file as managed.** Rejected: release-please must be able to bump its own manifest.
- **Per-file target branches (Firestartr's `target_branch`).** Out of scope for v1 (§2.2); files always go to the default branch.

## References

- Spec §2.2, §4.7, §5.1, §5.7, §6.2, §6.3, §7.7, §9.3, §11 (risk 5), §13.
- Firestartr: `packages/gh_provisioner/src/entities/ghfeature/helpers/managed_files.ts:76-81`.
- Verified: OpenTofu `removed` block `from` "cannot include instance keys".
