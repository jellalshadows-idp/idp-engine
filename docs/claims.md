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
| `environments.<env>.aws.accountId` | yes, in `aws` | | 12 digits, quoted (`"000000000001"`), so YAML keeps it a string with its leading zeros. Each environment needs its own account: its baseline creates the account's GitHub OIDC provider, and an account holds one |
| `environments.<env>.aws.region` | yes, in `aws` | | An AWS region of the standard `aws` partition, such as `eu-west-1`. A region of another partition, such as GovCloud (`us-gov-*`) or China (`cn-*`), fails validation: the identities are computed for the standard partition (`arn:aws:` ARNs and the `amazonaws.com` registry host), and floci would create the resources in the other one |
| `modules.allowedSources` | no | | Allowed Workspace module prefixes; used from Phase 3 |

An `aws` block is shaped like real AWS: an account ID and a region, never an emulator endpoint (the schema rejects any other key). In v1, AWS is the floci emulator in CI (spec §5.6), so the IDs can be fake ones like those above. An environment needs an `aws` block only when a Component with `aws.registry: true` lists it. Every environment with one gets its baseline stack, which creates the account's GitHub OIDC provider, whether or not a Component uses it (see "Rendering").

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
- **The variables come first.** Since Phase 2b the github stack carries them, and GitHub creates them when the pipelines apply that stack, after the merge (see "Rendering"). Since Phase 2d the render also writes the AWS stack that creates the ECR repository and the role they name; the pipelines apply it, to the floci emulator, from Phase 2e.
- Turning `aws.registry` off, or removing one of the Component's environments, deletes those variables; that reconcile waits for approval in `idp-approval`, like every delete.

The `features` field arrives in Phase 4. Until then it fails validation.

## Validation

`idp validate --dir <claims repo>` reports every problem at once as `file:line: message` (`file: message` when a problem has no line, such as a missing file). Inside GitHub Actions (`GITHUB_ACTIONS=true`) it also prints GitHub annotations, so errors show inline on the PR. It exits 0 when the claims are valid, 3 when it found problems, 1 when it could not run (for example an unreadable directory), and 2 on a usage error ([ADR-0016](adr/0016-exit-codes.md)).

Beyond the schemas, the checks include: each name matches its file name; the owner Group exists; every environment exists in `config/platform.yaml`; a Component with `aws.registry: true` lists at least one environment, and each of them has an `aws` block; no two environments share an `accountId`; and every region is in the standard `aws` partition. For the Component above, in a platform whose `pro` has no `aws` block:

```text
claims/components/api.yaml:6: environment "pro" needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true
```

For the platform above with `pro` set to `aws: { accountId: "000000000001", region: us-gov-west-1 }`, both problems are reported at the later environment:

```text
config/platform.yaml:11: environment "pro" has region "us-gov-west-1", in the aws-us-gov partition: only the standard aws partition is supported, because the AWS identities are computed with arn:aws: ARNs and the amazonaws.com host (spec §5.5)
config/platform.yaml:11: environment "pro" uses accountId "000000000001", like environment "dev": each environment needs its own AWS account, because each environment's baseline creates that account's GitHub OIDC provider (spec §5.4)
```

## Rendering

`idp render --dir <claims repo> --out rendered` writes:

- `rendered/github/main.tf.json`: the provider, the local backend at `../../tfstate/github.tfstate`, enforced state and plan encryption, and one module call per claim, pinned to the engine's release tag;
- `rendered/github/.terraform.lock.hcl`: the pinned provider checksums;
- `rendered/aws/<env>/_baseline/`: for every environment with an `aws` block, a stack that calls `modules/aws/baseline`, the account's GitHub OIDC provider;
- `rendered/aws/<env>/components/<name>/`: for every Component with `aws.registry: true`, in each of its environments, a stack that calls `modules/aws/component` with the repository, the role and the OIDC provider that `internal/naming` computes, the same names the github stack's variables carry;
- `rendered/.idp-rendered`: a marker. `--out` must not exist, be empty, or hold a previous render, so a mistyped path can never wipe unrelated data.

A Component with `aws.registry: true` passes `variables` to its module for each of its environments: `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION`, computed from the names and the environment's `aws` block (spec §5.3). A Component without it renders exactly as before, so its stack plans no change. Rendering creates nothing: GitHub creates the variables only when the pipelines apply the github stack (reconcile, after the merge), like every other GitHub resource.

Each AWS stack holds the pinned OpenTofu and `hashicorp/aws` versions, the provider with the environment's region and nothing else, the local backend at `terraform.tfstate` in its own directory, no encryption block, and the engine's AWS lock file. Its state stays in the job that runs it and is never committed to `wet` ([ADR-0030](adr/0030-aws-stack-state-stays-in-the-job.md)). The stacks never mention floci: CI adds `floci_override.tf.json` (spec §5.6). A claims repo without `aws` blocks renders exactly as before. The pipelines apply the AWS stacks only from Phase 2e; until then, a claims repo with an `aws` block stays on an engine release from before Phase 2d ([pipelines](pipelines.md#limitations)).

`--out` is relative to the current directory, not to `--dir`. The github stack's local backend writes its state to `<out>/../tfstate/github.tfstate`, which is where the `wet` branch keeps it (spec §5.4).

Claim text is data: the render escapes `${` and `%{` (as `$${` and `%%{`) in every free-text value that comes from a claim, so OpenTofu never evaluates a template in it. The other claim values (names, logins, roles, topics, environment names) cannot contain those characters: the schemas forbid them.

Development builds must pass `--module-ref <ref>`, or `--modules-dir <path>` to validate against local modules.
