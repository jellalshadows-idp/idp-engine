# idp-engine

The engine of an internal developer platform (IDP) that reconciles on **GitHub Actions** instead of a Kubernetes operator. Desired state lives in claims (YAML in Git), with no Kubernetes involved.

> **Status:** Phase 2 is complete and released as `v0.2.0`: the pipelines plan and apply the AWS stacks against the floci emulator, each stack in a job of its own that holds no secret ([ADR-0031](docs/adr/0031-floci-jobs-hold-no-secret.md)), and the end-to-end harness checks a Component's AWS stack and environment variables through `idp-claims-e2e` ([docs/e2e.md](docs/e2e.md)). Nothing reaches a real AWS account. See the [phase 2f log](docs/phases/phase-2f.md). Phase 3 (Workspaces) is next.

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
- The pipelines reconcile GitHub (teams, repositories, rulesets, environments and, since Phase 2b, the environment variables of a Component with `aws.registry`) and, since Phase 2e, the AWS stacks that create the OIDC provider, the ECR repository and the CI role, **against the floci emulator only**: their state ends with the job, and there is no AWS drift check.
- A reconcile that needs approval holds the claims repo's pipelines until someone decides.

## License

[Apache-2.0](LICENSE)
