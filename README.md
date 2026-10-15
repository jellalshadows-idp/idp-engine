# idp-engine

The engine of an internal developer platform (IDP) that reconciles on **GitHub Actions** instead of a Kubernetes operator. Desired state lives in claims (YAML in Git), with no Kubernetes involved.

> **Status:** Phase 1c is complete: the reusable `pr`, `reconcile`, `drift` and `recover` workflows reconcile a claims repo end to end, proven by a live smoke run in `idp-claims-e2e`; see the [phase 1c log](docs/phases/phase-1c.md) and [docs/pipelines.md](docs/pipelines.md). Phase 1d (the E2E harness and the first release) is next.

**What exists today:** `idp-claims-e2e` runs the pipelines against the org; `idp-claims` is bootstrapped and protected, and gets its workflows with the first release.

- Design: [docs/superpowers/specs/2026-10-08-idp-on-actions-design.md](docs/superpowers/specs/2026-10-08-idp-on-actions-design.md)
- Decisions: [docs/adr/](docs/adr/)
- Claims reference: [docs/claims.md](docs/claims.md)
- Pipelines: [docs/pipelines.md](docs/pipelines.md)
- End-to-end harness: [docs/e2e.md](docs/e2e.md)
- CLI reference: [docs/cli.md](docs/cli.md)
- Runbooks: [docs/runbooks/](docs/runbooks/)
- Phase log: [docs/phases/](docs/phases/)
- Plans: [docs/superpowers/plans/](docs/superpowers/plans/)

## Honest limitations (v1)

- AWS is **emulated** with [floci](https://floci.io). No real cloud account is used.
- Every repository is **public**: on the GitHub Free plan, rulesets and environment reviewers exist only for public repos.
- **No secrets** flow through the platform.
- Phase 1 reconciles **GitHub only** (teams, repositories, rulesets, environments); AWS arrives in Phase 2.
- A reconcile that needs approval holds the claims repo's pipelines until someone decides.

## License

[Apache-2.0](LICENSE)
