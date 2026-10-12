# 0021. The reusable workflows build idp from their own commit

- Status: Accepted
- Date: 2026-10-11
- Spec: [§6](../superpowers/specs/2026-10-08-idp-on-actions-design.md#6-pipelines), [§7.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#75-supply-chain)

## Context

A claims repo calls the engine's reusable workflows by commit. Each job needs the `idp` CLI at the same version as the workflow file, and the render pins module sources to the CLI's own version (spec §5.3). No release exists yet, so there is no binary to download and verify (spec §7.5).

Inside a called workflow, `github.workflow_sha` is the **caller's** commit. The job context has `job.workflow_repository` and `job.workflow_sha`, which name the reusable workflow's own repository and commit.

## Decision

Every reusable job that runs `idp`:

1. checks out `${{ job.workflow_repository }}` at `${{ job.workflow_sha }}` into `.idp/engine`, without persisted credentials;
2. runs the local composite action `.idp/engine/.github/actions/setup-idp`, which installs Go from the engine's `go.mod` without a cache, installs OpenTofu 1.12.6, and builds `idp` with `version.Version` set to that commit;
3. renders with that default module ref, so the workflow, the CLI and the modules are one commit.

Callers pin the reusable workflows to a full commit SHA. The templates mark the pin `# unreleased` until releases exist.

## Consequences

### Positive

- One version for the workflow, the CLI and the modules, with no release machinery yet.
- No binary travels between jobs or runs, so there is no artifact to trust.

### Negative / costs

- Each job spends a Go build. There is no module or build cache, on purpose: a cache restored into the job that holds the writer key is a poisoning path.
- actionlint v1.7.12 does not know `job.workflow_repository` and `job.workflow_sha`. `.github/actionlint.yaml` ignores exactly that message, only for `.github/workflows/`.
- zizmor's `self-repository` audit flags `uses: ./…` in a reusable workflow, because `./` resolves in the caller's workspace. That is why the engine is checked out there first; each such line carries an inline ignore that says so.
- Module sources pin a commit, not a tag, until releases. Phase 1d decides how releases change this (spec §7.5).

## Alternatives considered

- **`github.workflow_sha`.** Rejected: in a called workflow it is the caller's commit.
- **Download a released binary and verify its attestation.** Not possible before the first release; Phase 1d revisits it.
- **Build once and pass the binary between jobs as an artifact.** Rejected: it saves under a minute per job and adds a trust hop.
- **The `$/` prefix for local actions.** Rejected for now: it needs a recent runner (2.336.0 or later), and how it resolves inside a called workflow is not settled. The explicit checkout is plain and testable.
