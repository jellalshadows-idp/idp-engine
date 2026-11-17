# idp-engine

The engine of an internal developer platform (IDP) that reconciles on **GitHub Actions** instead of a Kubernetes operator. Desired state lives in claims (YAML in Git), with no Kubernetes involved.

> **Status:** Phase 2d is complete: `idp render` writes the AWS stacks, a baseline for every environment with an `aws` block and a stack for every Component with `aws.registry` in each of its environments, with the identities `internal/naming` computes and their state kept in the job, never in `wet` ([ADR-0030](docs/adr/0030-aws-stack-state-stays-in-the-job.md)). CI validates every rendered stack, applies the rendered AWS stacks to two accounts of the floci emulator on every PR, and `idp diff` reports them by their nested paths. The pipelines do not apply them yet. See the [phase 2d log](docs/phases/phase-2d.md) and [docs/claims.md](docs/claims.md). Phase 2e (the workflows' AWS plan and apply) is next.

**What exists today:** `idp-claims` runs the pipelines at an engine release; `idp-claims-e2e` is checked by the end-to-end harness nightly and on every release PR.

- Design: [docs/superpowers/specs/2026-10-08-idp-on-actions-design.md](docs/superpowers/specs/2026-10-08-idp-on-actions-design.md)
- Decisions: [docs/adr/](docs/adr/)
- Claims reference: [docs/claims.md](docs/claims.md)
- Pipelines: [docs/pipelines.md](docs/pipelines.md)
- End-to-end harness: [docs/e2e.md](docs/e2e.md)
- CLI reference: [docs/cli.md](docs/cli.md)
- Runbooks: [docs/runbooks/](docs/runbooks/)
- Phase log: [docs/phases/](docs/phases/)
- Plans: [docs/superpowers/plans/](docs/superpowers/plans/)

## Releases

The engine ships as one version for everything (the CLI, the reusable workflows, the modules and the schemas), tagged `vX.Y.Z` and `0.x` until v1. release-please keeps a release PR on `main`; the end-to-end harness runs on it, and merging it tags the release. Each release carries `idp` for Linux, macOS and Windows (amd64 and arm64), `checksums.txt` and a build provenance attestation. Claims repos pin a release commit in their four caller workflows, the workflows run that release's verified binary, and Renovate moves the pins when a new release ships.

To install the CLI, download your platform's archive from the release page and verify it before you run it:

```bash
gh attestation verify <archive> --repo jellalshadows-idp/idp-engine   --signer-workflow jellalshadows-idp/idp-engine/.github/workflows/release.yaml
```

Runbooks: [releases](docs/runbooks/releases.md), [dependency updates](docs/runbooks/dependencies.md).

## Honest limitations (v1)

- AWS is **emulated** with [floci](https://floci.io). No real cloud account is used.
- Every repository is **public**: on the GitHub Free plan, rulesets and environment reviewers exist only for public repos.
- **No secrets** flow through the platform.
- The pipelines reconcile **GitHub only** (teams, repositories, rulesets, environments and, since Phase 2b, the environment variables of a Component with `aws.registry`). The AWS stacks that create the OIDC provider, the ECR repository and the CI role are rendered since Phase 2d and tested against floci in CI; the pipelines apply them from Phase 2e, so a claims repo with an `aws` block stays on `v0.1.1` until `v0.2.0`.
- A reconcile that needs approval holds the claims repo's pipelines until someone decides.

## License

[Apache-2.0](LICENSE)
