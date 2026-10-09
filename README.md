# idp-engine

The engine of an internal developer platform (IDP) that reconciles on **GitHub Actions** instead of a Kubernetes operator. Desired state lives in claims (YAML in Git), with no Kubernetes involved.

> **Status:** Phase 1a is complete: `idp validate` and `idp render` turn a claims repo into a validated GitHub stack, backed by the tested `github/group` and `github/component` modules; see the [phase 1a log](docs/phases/phase-1a.md). Phase 1b (pipelines) is next, so nothing is reconciled automatically yet.

**What exists today:** `idp-claims` and `idp-claims-e2e` are bootstrapped and protected. Phase 1 (pipelines) is next.

- Design: [docs/superpowers/specs/2026-10-08-idp-on-actions-design.md](docs/superpowers/specs/2026-10-08-idp-on-actions-design.md)
- Decisions: [docs/adr/](docs/adr/)
- Claims reference: [docs/claims.md](docs/claims.md)
- Phase log: [docs/phases/](docs/phases/)
- Plans: [docs/superpowers/plans/](docs/superpowers/plans/)

## Honest limitations (v1)

- AWS is **emulated** with [floci](https://floci.io). No real cloud account is used.
- Every repository is **public**: on the GitHub Free plan, rulesets and environment reviewers exist only for public repos.
- **No secrets** flow through the platform.

## License

[Apache-2.0](LICENSE)
