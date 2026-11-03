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

```yaml
apiVersion: idp/v1
kind: Platform
github:
  org: acme
  writerAppId: 5255579      # the writer App's id
environments:
  dev:
    aws: { accountId: "000000000001", region: eu-west-1 }
  pro:
    protected: true
    aws: { accountId: "000000000003", region: eu-west-1 }
```

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
| `environments.<env>.aws` | no | | The environment's AWS account. Required for every environment of a Component with `aws.registry: true` |
| `environments.<env>.aws.accountId` | yes, in `aws` | | 12 digits, quoted (`"000000000001"`), so YAML keeps it a string with its leading zeros |
| `environments.<env>.aws.region` | yes, in `aws` | | An AWS region, such as `eu-west-1` |
| `modules.allowedSources` | no | | Allowed Workspace module prefixes; used from Phase 3 |

An `aws` block is shaped like real AWS: an account ID and a region, never an emulator endpoint (the schema rejects any other key). In v1, AWS is the floci emulator in CI (spec §5.6), so the IDs can be fake ones like those above. An environment needs an `aws` block only when a Component with `aws.registry: true` lists it.

## Group (`claims/groups/<name>.yaml`)

```yaml
apiVersion: idp/v1
kind: Group
name: platform
description: Platform team   # optional, single line
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
description: Orders API    # optional, single line
owner: group:platform      # must name an existing Group
environments: [dev, pro]   # must exist in config/platform.yaml
github:
  topics: [java, orders]   # optional; at most 20, unique, each lowercase letters, digits or hyphens, ≤ 50 characters
aws:
  registry: true           # optional; default false
```

A Component renders:
- a public repository (auto-init, delete branch on merge, vulnerability alerts on);
- the repository's topics, from `github.topics`. Removing the last topic deletes the topics resource, because the GitHub provider clears topics only when that resource is destroyed. That reconcile waits for approval in `idp-approval`, like every delete;
- `maintain` access for the owner team;
- a default-branch ruleset that requires PRs, blocks force pushes and deletion, and lets only the writer App bypass;
- one environment per entry, each deploying only from `main`. Protected environments require the owner team.

With `aws.registry: true`, a Component also gets, in each of its environments: an ECR repository named after it; the IAM role `<name>-<env>-ci`, which may push to that repository and trusts only GitHub's OIDC tokens for `repo:<org>/<name>:environment:<env>`; and the environment variables `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION` (spec §4.4). Their values are computed from the names and the environment's `aws` block, never read from AWS (spec §5.5). For `api` in `pro` above they are `arn:aws:iam::000000000003:role/api-pro-ci`, `000000000003.dkr.ecr.eu-west-1.amazonaws.com/api` and `eu-west-1`.

- `aws.registry: true` needs at least one environment, and each environment it lists needs an `aws` block in `config/platform.yaml`.
- `aws: {}` and `registry: false` are valid and ask for nothing.
- **Not rendered yet.** Since Phase 2a the claims accept and validate the `aws` part, but nothing renders from it until Phases 2b to 2d, and the pipelines apply it from Phase 2e.

The `features` field arrives in Phase 4. Until then it fails validation.

## Validation

`idp validate --dir <claims repo>` reports every problem at once as `file:line: message` (`file: message` when a problem has no line, such as a missing file). Inside GitHub Actions (`GITHUB_ACTIONS=true`) it also prints GitHub annotations, so errors show inline on the PR. It exits 0 when the claims are valid, 3 when it found problems, 1 when it could not run (for example an unreadable directory), and 2 on a usage error ([ADR-0016](adr/0016-exit-codes.md)).

Beyond the schemas, the checks include: each name matches its file name; the owner Group exists; every environment exists in `config/platform.yaml`; a Component with `aws.registry: true` lists at least one environment, and each of them has an `aws` block. For the Component above, in a platform whose `pro` has no `aws` block:

```text
claims/components/api.yaml:6: environment "pro" needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true
```

## Rendering

`idp render --dir <claims repo> --out rendered` writes:

- `rendered/github/main.tf.json`: the provider, the local backend at `../../tfstate/github.tfstate`, enforced state and plan encryption, and one module call per claim, pinned to the engine's release tag;
- `rendered/github/.terraform.lock.hcl`: the pinned provider checksums;
- `rendered/.idp-rendered`: a marker. `--out` must not exist, be empty, or hold a previous render, so a mistyped path can never wipe unrelated data.

A Component's `aws` part renders nothing yet: Phase 2b adds its environment variables to the github stack, and Phase 2d its AWS stacks.

`--out` is relative to the current directory, not to `--dir`. The stack's local backend writes its state to `<out>/../tfstate/github.tfstate`, which is where the `wet` branch keeps it (spec §5.4).

Claim text is data: the render escapes `${` and `%{` (as `$${` and `%%{`) in every free-text value that comes from a claim, so OpenTofu never evaluates a template in it. The other claim values (names, logins, roles, topics, environment names) cannot contain those characters: the schemas forbid them.

Development builds must pass `--module-ref <ref>`, or `--modules-dir <path>` to validate against local modules.
