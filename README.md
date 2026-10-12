# idp-engine

The engine of an internal developer platform (IDP) that reconciles on **GitHub Actions** instead of a Kubernetes operator. Desired state lives in claims (YAML in Git), with no Kubernetes involved.

> **Status:** Phase 1b is complete: every pipeline subcommand (`diff`, `plan-summary`, `gate`, `comment`, `issue`, `wet-push`, `encryption-env`) and a drift-ready `bootstrap check` ship with tests; see the [phase 1b log](docs/phases/phase-1b.md). Phase 1c (the reusable workflows and a live smoke run) is next, so nothing is reconciled automatically yet.

**What exists today:** `idp-claims` and `idp-claims-e2e` are bootstrapped and protected. Phase 1b (pipeline commands) is complete; Phase 1c (reusable workflows) is next.

- Design: [docs/superpowers/specs/2026-10-08-idp-on-actions-design.md](docs/superpowers/specs/2026-10-08-idp-on-actions-design.md)
- Decisions: [docs/adr/](docs/adr/)
- Claims reference: [docs/claims.md](docs/claims.md)
- Pipelines: [docs/pipelines.md](docs/pipelines.md)
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
