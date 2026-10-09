# Claims reference

A claims repo describes the desired state of a GitHub org. `idp validate` checks it, and `idp render` turns it into OpenTofu stacks. The JSON Schemas in [`schemas/`](../schemas) are the source of truth for every field ([ADR-0015](adr/0015-claim-parsing-and-validation.md)).

## Layout

```
config/platform.yaml          # one per claims repo
claims/groups/<name>.yaml     # kind: Group
claims/components/<name>.yaml # kind: Component
```

- The directory decides the kind. A Group file under `claims/components/` fails validation.
- Each file holds exactly one YAML document, its `name` must equal the file name, and the file must end in `.yaml`. Dotfiles such as `.gitkeep` are ignored.
- Because `name` equals the file name and names are lowercase, names are unique within their kind (spec §4.7). A Group and a Component may share a name.
- Workspace claims (`claims/workspaces/<env>/`) arrive in Phase 3. Until then that directory is an error.

## Editor completion

Put this first line in a claim file to get completion and inline errors in editors with the YAML language server:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jellalshadows-idp/idp-engine/main/schemas/component.json
```

Use `group.json` or `platform.json` for the other kinds. Pin a release tag instead of `main` once one exists.

## Platform (`config/platform.yaml`)

| Field | Required | Default | Meaning |
|---|---|---|---|
| `github.org` | yes | | The GitHub org |
| `github.writerAppId` | yes | | The writer App's id; the only bypass actor of every Component's default-branch ruleset |
| `github.archiveOnDestroy` | no | `true` | Archive (not delete) a repo when its Component claim is removed |
| `github.requiredApprovals` | no | `1` | PR approvals on Component repos (0–10) |
| `naming.requiredPrefix` | no | | Every claim name must start with it (e.g. `e2e-` in `idp-claims-e2e`) |
| `naming.reservedPrefixes` | no | | No claim name may start with these (e.g. `[e2e-, spike-]` in `idp-claims`) |
| `environments.<env>` | yes (≥ 1) | | Env names match `^[a-z][a-z0-9]{0,9}$` |
| `environments.<env>.protected` | no | `false` | Protected envs require the owner team's approval |
| `environments.<env>.aws` | no | | `accountId` and `region`; used from Phase 2 |
| `modules.allowedSources` | no | | Allowed Workspace module prefixes; used from Phase 3 |

## Group (`claims/groups/<name>.yaml`)

```yaml
apiVersion: idp/v1
kind: Group
name: platform
description: Platform team   # optional
members:
  - user: alice            # GitHub login; each login at most once (case-insensitive)
    role: maintainer       # maintainer | member
```

A Group renders a closed GitHub team with these memberships. Adding someone who is not yet an org member sends them an org invitation; Phase 0 measured that a pending invitation causes no plan drift.

## Component (`claims/components/<name>.yaml`)

```yaml
apiVersion: idp/v1
kind: Component
name: api                  # = repository name
description: Orders API    # optional
owner: group:platform      # must name an existing Group
environments: [dev, pro]   # must exist in config/platform.yaml
github:
  topics: [java, orders]   # optional; at most 20, unique, each lowercase letters, digits or hyphens, ≤ 50 characters
```

A Component renders:
- a public repository (auto-init, delete branch on merge, vulnerability alerts on);
- `maintain` access for the owner team;
- a default-branch ruleset that requires PRs, blocks force pushes and deletion, and lets only the writer App bypass;
- one environment per entry, each deploying only from `main`. Protected environments require the owner team.

The `aws` and `features` fields arrive in Phases 2 and 4. Until then they fail validation.

## Validation

`idp validate --dir <claims repo>` reports every problem at once as `file:line: message` (`file: message` when a problem has no line, such as a missing file). Inside GitHub Actions (`GITHUB_ACTIONS=true`) it also prints GitHub annotations, so errors show inline on the PR. It exits 0 when valid, 1 with problems, and 2 on a usage error.

## Rendering

`idp render --dir <claims repo> --out rendered` writes:

- `rendered/github/main.tf.json`: the provider, the local backend at `../../tfstate/github.tfstate`, enforced state and plan encryption, and one module call per claim, pinned to the engine's release tag;
- `rendered/github/.terraform.lock.hcl`: the pinned provider checksums;
- `rendered/.idp-rendered`: a marker. `--out` must not exist, be empty, or hold a previous render, so a mistyped path can never wipe unrelated data.

Development builds must pass `--module-ref <ref>`, or `--modules-dir <path>` to validate against local modules.
