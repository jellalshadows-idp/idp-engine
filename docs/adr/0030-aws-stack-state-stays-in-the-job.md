# 0030. The AWS stacks keep their state in the job, unencrypted

- Status: Accepted
- Date: 2026-11-17
- Spec: [§5.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#54-layout-of-the-wet-branch), [§5.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#55-deterministic-identities-with-no-shared-state-between-stacks), [§5.6](../superpowers/specs/2026-10-08-idp-on-actions-design.md#56-emulator-specifics-belong-to-ci-not-to-the-render), [§7.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#72-encrypted-state)

## Context

Phase 2d renders the AWS stacks (spec §5.4): the `_baseline` of every environment with an `aws` block, and the stack of each Component with `aws.registry: true` in each of its environments. Their resources live in floci, which every job starts empty (ADR-0004), so a job rebuilds the AWS side by applying the stacks again (spec §6.1). The `github` stack is different: GitHub is real, so its state is the only record of what the pipelines created, and `wet` keeps it, encrypted (ADR-0002, spec §7.2). Spec §5.4 lists no AWS state in `wet`, and spec §5.5 says the AWS state cannot be read later. The render must say where an AWS stack keeps its state, and whether that state is encrypted.

## Decision

- Each AWS stack declares the local backend at `terraform.tfstate`, in the stack's own directory. The state lives in the job's working copy, as long as the emulator it describes. Nothing AWS is committed to `wet` but the rendered stacks.
- The AWS stacks emit no `encryption` block. The `github` stack keeps its enforced state and plan encryption.
- A job that sets `TF_ENCRYPTION` anyway, as `idp encryption-env` does for the whole job, encrypts the AWS state with it, and must keep it set for every command on that stack; a job without it writes plain state. Both work, because the state never leaves the job.

## Consequences

### Positive

- The AWS jobs need no passphrase, so `IDP_STATE_PASSPHRASE` never has to reach the floci jobs, which hold the runner's Docker socket (ADR-0004).
- `wet` keeps one state file, and no AWS state can go stale against an emulator that no longer exists.
- The render stays thin (spec §5.3): the backend is one constant path, pinned by the golden files.

### Negative / costs

- Going to real AWS changes the render: the AWS stacks then need a persistent backend and encryption, and the replay of spec §6.1 changes with them (spec §5.6, amendment A13).
- The state file is written inside the stack's directory. Phase 2e must run the AWS stacks in a scratch copy, as the `github` stack already runs in `$RUNNER_TEMP/idp/work`, and must never push AWS state to `wet`; `idp diff` would read a `terraform.tfstate` left in a rendered tree as a file of its stack. Because the path is relative to the stack's directory, the replay of spec §6.1 (apply the `wet` version, then plan the new one) must run both versions against one state: in the same scratch directory, swapping the configuration, or with `init -backend-config=path=<absolute path>`; otherwise the plan is a full create instead of the real change.

## Alternatives considered

- **AWS state in `wet`, encrypted, like the `github` stack.** Rejected: it would describe resources that are gone when the job ends, so the next job would plan against state that matches nothing, and the floci jobs would need the passphrase.
- **An `encryption` block, enforced.** Rejected: every AWS job would need `TF_ENCRYPTION`, which puts the passphrase in the jobs that hold the Docker socket, to protect state that never leaves the job and holds only names that the renders and the environment variables already publish.
- **No backend block, OpenTofu's default local state.** The same behavior. Rejected for clarity: the explicit block states the decision in every stack, and the golden files pin it.

## References

- Spec §5.4, §5.5, §5.6 (amendment A13), §6.1, §7.2 (amendment A13).
- ADR-0002 (the `github` stack's encrypted state), ADR-0004 (one floci per job, amended in Phase 2d), ADR-0020 (`TF_ENCRYPTION`).
- Verified while planning Phase 2d with OpenTofu 1.12.6: a stack without an `encryption` block writes plain state without `TF_ENCRYPTION`, and encrypted state with it; that encrypted state cannot be read once the variable is unset.
