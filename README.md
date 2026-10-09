# idp-engine

The engine of an internal developer platform (IDP) that reconciles on **GitHub Actions** instead of a Kubernetes operator. It is inspired by [Firestartr](https://github.com/firestartr-pro/firestartr).

> **Status:** Phase 0 (bootstrap + spike) is in progress. The `idp bootstrap` command has shipped and spikes S1 and S3 are done; the steps that need a GitHub App (S2, bootstrapping the claims repos) are pending. Nothing else is usable yet.

- Design: [docs/superpowers/specs/2026-10-08-idp-on-actions-design.md](docs/superpowers/specs/2026-10-08-idp-on-actions-design.md)
- Decisions: [docs/adr/](docs/adr/)
- Phase log: [docs/phases/](docs/phases/)
- Plans: [docs/superpowers/plans/](docs/superpowers/plans/)

## Honest limitations (v1)

- AWS is **emulated** with [floci](https://floci.io). No real cloud account is used.
- Every repository is **public**: on the GitHub Free plan, rulesets and environment reviewers exist only for public repos.
- **No secrets** flow through the platform.

## License

[Apache-2.0](LICENSE)
