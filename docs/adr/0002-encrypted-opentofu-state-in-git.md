# 0002. Encrypted OpenTofu state in Git

- Status: Accepted
- Date: 2026-10-08
- Spec: [§7.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#72-encrypted-state)

## Context

The GitHub stack manages real resources (teams, repos, rulesets, environments, files), so OpenTofu needs somewhere durable to keep its state between runs. Actions runners are ephemeral, and the platform deliberately has no cloud account or bucket: there is no real cloud credential anywhere in v1 (spec §7.6), and the AWS side is an emulator whose state is rebuilt on every job (§5.5, §5.6). A remote backend would mean introducing a new service, with its own credentials, only to hold a small file.

Git is already the system of record for the rendered output. The `wet` branch of the claims repo holds `rendered/` and the state, written only by `idp-writer` (§3.1, §7.4). Keeping the state next to the render means one `wet` commit describes both what was rendered and what was applied. The catch is that everything in this system is public: code, `wet`, logs, artifacts and comments (§7.1). A plaintext state file in a public branch is a bad default even if its content is mostly public today, because state can contain values nobody planned to expose.

OpenTofu supports native state and plan encryption. Per the facts verified during design: `TF_ENCRYPTION` environment configuration overrides what is in code, `enforced = true` refuses to write plaintext, a `fallback` block supports key rotation, and a PBKDF2 passphrase needs at least 16 characters.

## Decision

The GitHub state is stored as `tfstate/github.tfstate` in the `wet` branch of the claims repo, using the local backend, **encrypted by OpenTofu** (spec §5.4, §7.2).

- The render emits `encryption { state { enforced = true } plan { enforced = true } }`. The key material is a PBKDF2 passphrase (at least 16 characters) with AES-GCM, supplied through `TF_ENCRYPTION` and filled from a repo secret. If the variable is missing, OpenTofu refuses to write plaintext.
- Saved plans are encrypted too, and kept as artifacts with a 1-day retention (§6.2).
- Each claims repo has its own passphrase, `wet` branch and state, so `idp-claims` and `idp-claims-e2e` never share state (§8.5, ADR-0014).
- Rotation uses the `fallback` block, and restoring state is **not** a `git revert`; both are runbooks (§7.2, §7.7).
- The passphrase is a repo secret because plans must read the state. That it reaches PR plans is an accepted risk (§7.3).

## Consequences

### Positive

- No backend service and no extra credential to operate; the state lives with the render it belongs to.
- Every `wet` commit is a version of the state, which gives an audit trail and a backup (§7.2).
- AES-GCM is authenticated encryption, so tampered state fails to decrypt instead of being read (§7.2).
- The same setup is ready for private repos.

### Negative / costs

- **Honest note from the spec:** today the GitHub state is almost entirely public information, so the encryption buys **integrity** and readiness, not confidentiality (§7.2).
- If the passphrase leaks, someone can read near-public state, but still cannot write to `wet` (§7.3).
- Git is not a lock service. Concurrent writers are prevented by the `idp-wet` concurrency group, not by the backend (§6.2).
- Restoring state needs a runbook (recover with `import` blocks), because reverting a commit does not restore consistent state (§7.2, §7.7).

### Follow-ups

- Phase 0 validates the encrypted state round-trip through Git (ADR-0013, spec §10).
- Runbooks to write: rotate the passphrase, recover lost or corrupt state, recover from a failed `wet` push (§7.7). Reconcile writes the state to `wet` even when an apply fails (§6.2).
- The saved-plan flow relies on OpenTofu refusing a stale plan, also when encrypted (§6.2).

## Alternatives considered

- **Plaintext state in Git.** Rejected: the branch is public, and `enforced = true` exists precisely to stop plaintext being written by mistake.
- **A remote backend (object storage or a hosted service).** Rejected: it adds infrastructure and a real credential, against the no-real-credentials rule (§7.6) and the zero-infrastructure goal (ADR-0001).
- **Actions artifacts or cache as the state store.** Rejected: they expire and are not versioned or auditable the way commits are. Artifacts are used only for 1-day saved plans.
- **Encrypting the file outside OpenTofu.** Rejected: native encryption covers state and plans, supports `enforced` and `fallback`, and needs no extra tooling.

## References

- Spec §5.4, §6.2, §7.1-§7.3, §7.6, §7.7, §8.5, §10.
- Verified: OpenTofu `TF_ENCRYPTION`, `enforced`, `fallback`, PBKDF2 minimum, stale saved plans refused also when encrypted (opentofu e2e `encryption_test.go`).
