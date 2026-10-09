# IDP on GitHub Actions — Design Spec (v1)

| | |
|---|---|
| Status | Draft — awaiting owner review |
| Date | 2026-10-08 |
| Owner | Adrian-Manuel |
| Reference system | [Firestartr](https://github.com/firestartr-pro/firestartr) (Prefapp), read at `a9dd5d3` |

## 1. Intent

**What.** An internal developer platform (IDP) inspired by Firestartr. Teams declare
**claims** (YAML) in a Git repo, and the platform turns them into real GitHub
resources (teams, repositories, rulesets, environments, files) and AWS resources.
Firestartr runs its reconciliation as a Kubernetes operator; this one runs on
**GitHub Actions only**.

**Why.** It's a public portfolio project. It demonstrates platform-engineering
judgment: a claims model, rendering, gated plan/apply, state handling, least
privilege, supply-chain hygiene, and honest documentation of tradeoffs.

**Who it is for.** Reviewers and interviewers reading the repos, and anyone who wants
to bootstrap it on their own GitHub organization.

**Success criteria for v1 (`1.0.0`):**
1. Claims for Group, Component (GitHub + AWS) and Workspace go from PR to applied
   resources through the pipelines described here.
2. The end-to-end harness passes nightly against the `idp-claims-e2e` repo.
3. Someone external can bootstrap a new org by following only the README.
4. Every major decision has an ADR, and the limitations are stated up front.

## 2. Scope and constraints

### 2.1 In scope (v1)
- Claim kinds: `Group`, `Component` (a GitHub part plus an optional AWS part), and
  `Workspace` (a generic OpenTofu module).
- **Features**: versioned file bundles installed into Component repos. The v1
  catalog has `container-ci`, `release-please` and `codeowners`.
- Pipelines: validate/plan on PR, gated apply on merge, manual reconcile, and a
  daily drift check.
- AWS only, **emulated with floci**.

### 2.2 Out of scope (v1), deliberately
- Kubernetes, and level-triggered continuous reconciliation.
- Multi-cloud, and real AWS accounts. The claims and config are already shaped
  for real AWS, so going live only changes the target (see §5.6).
- Secrets management. Claims never carry secrets (see §7.6).
- Private repositories, which need the GitHub Team plan (see §2.3).
- A `User` kind, nested teams, renames, and cross-workspace references.
- Switching an existing file between `managed` and `userManaged` (rejected by
  validation).
- Per-file target branches (Firestartr's `target_branch`).
- AWS drift detection. Emulated AWS is rebuilt from `wet` on every job, so it
  cannot drift.

### 2.3 Hard constraints
- **The GitHub org is on the Free plan.** On Free, rulesets, environment required
  reviewers, wait timers and environment secrets are available **only on public
  repositories**. So every repo in this system is public, including Component
  repos created by the platform.
- **The owner has no local Docker.** Anything that needs a container runtime
  (floci, e2e) runs in CI only. Go and `tofu test` with mocks run locally.
- **Everything is readable by anyone**: code, `wet`, Actions logs, artifacts and PR
  comments (see §7.1).

## 3. Architecture

### 3.1 Repositories

Everything lives in **one** org, `<org>` (see §12). Production and tests are separated
by claims repo, not by org: each claims repo has its own `wet` branch, its own
encrypted state and its own passphrase (owner decision, 2026-10-09; see §8.5).

| Repo | Role | Versioning |
|---|---|---|
| `idp-engine` | Product: Go CLI `idp`, OpenTofu modules (`modules/github/*`, `modules/aws/*`), reusable workflows, JSON Schemas, provider lock files, bootstrap, docs | release-please, **one version for everything** (`vX.Y.Z`) |
| `idp-features` | Catalog of features (`features/<name>/`) | release-please multi-component, tags `<name>-v<semver>` |
| `idp-modules` | Catalog of Workspace modules (`modules/<name>/`). v1 ships `s3-bucket` | release-please multi-component, tags `<name>-v<semver>` |
| `idp-claims` | `main` = desired state (DRY). `wet` = rendered output + encrypted GitHub state, written only by `idp-writer` | None. Renovate bumps the engine version |
| `idp-claims-e2e` | E2E harness target (see §8.5): same shape as `idp-claims`, but only `e2e-`-prefixed resources | None |

### 3.2 Flow

```
 developer                idp-claims (main)                 idp-claims (wet)
 ─────────                ─────────────────                 ────────────────
 edit claims ──PR──▶  validate → render → diff vs wet ◀──── rendered/ + tfstate
                      plan github (state from wet)
                      plan aws (floci per job, replay from wet)
                      sticky comment + fingerprint
              merge ─▶ reconcile (serialized):
                      re-render → plan → gate (auto | approval)
                      apply aws (floci) → apply github
                      ONE commit to wet ───────────────────▶ rendered/ + tfstate
 cron daily ────────▶ drift plan (github) + bootstrap --check → issue
```

### 3.3 Units

Each unit has one purpose and a narrow interface.

| Unit | Purpose | Depends on |
|---|---|---|
| `internal/claims` | Load, parse and validate claims (schema + semantics) | `internal/config`, JSON Schemas |
| `internal/config` | Load `config/platform.yaml` | — |
| `internal/naming` | Deterministic identities: ARNs, ECR URLs, role names, Terraform addresses | `internal/config` |
| `internal/features` | Fetch features at a tag, resolve inputs, render templates | GitHub API (fetch only) |
| `internal/render` | Claims → stacks (`.tf.json` + files). Pure | `claims`, `naming`, `features` (cache) |
| `internal/wetdiff` | New render vs `wet` → changed and orphaned stacks, as a matrix | filesystem |
| `internal/plan` | `tofu show -json` → policy verdict, fingerprint, Markdown summary, gate decision | — |
| `modules/github/{group,component}` | GitHub resources for each claim kind | `integrations/github` provider |
| `modules/aws/{baseline,component}` | Per-env OIDC provider, and a Component's ECR + CI role | `hashicorp/aws` provider |
| reusable workflows | `pr.yaml`, `reconcile.yaml`, `drift.yaml` (`on: workflow_call`) | the `idp` CLI, `tofu`, floci |

| `internal/ghapi` | Minimal GitHub REST client with typed errors | GitHub API |
| `internal/bootstrap` | Org bootstrap: create Apps from manifests, apply and check the protections (§9.2) | `internal/ghapi` |

CLI subcommands: `idp validate`, `idp fetch`, `idp adopt`, `idp render`,
`idp diff`, `idp plan-summary`, `idp gate`, `idp feature test`, and
`idp bootstrap app|apply|check`.

## 4. Claims model

### 4.1 Layout of `idp-claims` (`main`)
```
claims/
  groups/<name>.yaml
  components/<name>.yaml
  workspaces/<env>/<name>.yaml     # the environment comes from the folder
config/
  platform.yaml
```

### 4.2 Common envelope
Every claim has `apiVersion: idp/v1`, `kind` and `name`. Every kind except `Group`
also has `owner`, and `description` is optional.
- `name` must equal the file name (without `.yaml`) and match
  `^[a-z][a-z0-9-]{0,38}[a-z0-9]$`, so it is at most 40 characters.
- `owner` is `group:<group-name>` and must reference an existing Group.

### 4.3 Group
```yaml
apiVersion: idp/v1
kind: Group
name: platform
members:
  - user: jellalshadows
    role: maintainer      # maintainer | member
  - user: test-account-1
    role: member
```
It renders a GitHub team (slug = `name`, privacy `closed`) plus its memberships.

### 4.4 Component
```yaml
apiVersion: idp/v1
kind: Component
name: api                         # = repo name = file name
owner: group:platform
environments: [dev, staging, pro] # must exist in platform.yaml
github:
  topics: [java, orders]          # optional
aws:
  registry: true                  # optional; v1's only AWS capability
features:
  - name: container-ci
    version: 0.1.0                # exact semver, no ranges
    args: { registry: ghcr }
```

**GitHub part**, always rendered:
- The repository: public, `auto_init: true`, `delete_branch_on_merge: true`,
  vulnerability alerts on, and `archive_on_destroy` from platform config (default
  `true`).
- The owner team gets `maintain` permission. `maintain` is enough to work, and repo
  settings belong to the platform.
- A ruleset on the default branch:
  - Requires a PR, with approvals taken from `github.requiredApprovals` in platform
    config (default 1).
  - Blocks force pushes and deletion.
  - Has `idp-writer` in the bypass list, so it can commit managed files.
- One GitHub environment per entry in `environments`. If the env is
  `protected: true` in platform config, the owner team is a required reviewer.

**AWS part**, rendered only when `aws.registry: true`. For each environment:
- An ECR repository named `<component>`.
- An IAM role `<component>-<env>-ci` that is allowed to push to that ECR. Its trust
  policy is restricted to the GitHub OIDC provider with
  `sub = repo:<org>/<component>:environment:<env>` and `aud = sts.amazonaws.com`.
- GitHub environment variables `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION`.
  Their values are computed deterministically (see §5.5).

### 4.5 Workspace
```yaml
# claims/workspaces/dev/logs-bucket.yaml
apiVersion: idp/v1
kind: Workspace
name: logs-bucket
owner: group:platform
policy: full-control              # full-control | apply | observe — REQUIRED, no default
module:
  source: git::https://github.com/<org>/idp-modules.git//modules/s3-bucket?ref=s3-bucket-v1.0.0
values:
  bucket_name: logs-dev
```
- **Identity.** A Workspace is identified by `<env>/<name>`, so the same name can
  exist in several envs. There is no `env` field, which keeps a single source of
  truth.
- **Module source.** It must start with one of `modules.allowedSources`, and `ref`
  must be a 40-hex SHA or a tag matching `^([a-z0-9-]+-)?v\d+\.\d+\.\d+$`. A module
  is code that runs in CI, so unpinned or foreign modules are rejected.
- **Values.** They are passed as module variables, and OpenTofu validates them at
  plan time. v1 has no `valuesSchema`.

Policies are adapted from Firestartr. Unlike Firestartr, which defaults to
`observe`, here the policy is required.

| Policy | Create/update | Delete or replace | Applied? |
|---|---|---|---|
| `full-control` | yes | yes, with approval (§6.3) | yes |
| `apply` | yes | **PR fails** | yes |
| `observe` | — | — | never; it is plan-only |

### 4.6 Platform config
```yaml
apiVersion: idp/v1
kind: Platform
github:
  org: <org>
  archiveOnDestroy: true          # false in idp-claims-e2e
  requiredApprovals: 1
environments:                     # names match ^[a-z][a-z0-9]{0,9}$
  dev:     { aws: { accountId: "000000000001", region: eu-west-1 } }
  staging: { aws: { accountId: "000000000002", region: eu-west-1 } }
  pro:     { aws: { accountId: "000000000003", region: eu-west-1 }, protected: true }
modules:
  allowedSources: ["git::https://github.com/<org>/"]
```
The config is shaped like real AWS: account IDs and regions, never emulator ports.
How CI maps these values to floci is defined in §5.6.

### 4.7 Validation
Validation runs in two layers, and **all errors are reported together** as GitHub
annotations with file and line.
1. **JSON Schema per kind** (draft 2020-12), published by the engine at each tag.
   This also gives editor completion through
   `# yaml-language-server: $schema=https://raw.githubusercontent.com/<org>/idp-engine/vX.Y.Z/schemas/<kind>.json`.
2. **Semantic checks:**
   - Name and file match, and names are unique within their kind and env.
   - The owner Group exists.
   - Every env exists in the platform config.
   - Each feature tag exists (checked by `idp fetch`) and its args validate against
     the feature's schema.
   - The module source is allowed and pinned.
   - A file's mode does not change between `wet` and the new render.

A rename shows up as a delete plus a create. It is not blocked, but it always goes
through approval (§6.3).

## 5. Rendering

### 5.1 Two steps; render is pure
- `idp fetch` uses the network. It downloads each referenced feature
  (`/repos/<org>/idp-features/tarball/<name>-v<version>`) into
  `.idp/cache/features/<name>@<version>/`.
- `idp adopt` uses the network with a read-only token. For each `userManaged` file
  that is not installed yet, if the file already exists in the target repo, it is
  marked as installed in the working copy of the manifest without being touched
  (§5.7).
- `idp render` runs **offline**. It reads the claims, `config/`, the cache and the
  manifest, and writes `rendered/`. **The same input always produces the same
  bytes.**

### 5.2 Output format: `.tf.json`
Stacks are generated as JSON, as Firestartr does. Its provisioners synthesize JSON
(`JSON.stringify(this.document)` in `gh_provisioner`, and
`firestartr-providers.tf.json` in `terraform_provisioner`). Go's `encoding/json`
sorts map keys, so the output is deterministic. Humans review the **plan** in the
PR comment, not the JSON.

### 5.3 Thin renderer, thick modules
The renderer emits only provider/backend blocks, `module` calls and `encryption`
settings. Module sources point to `idp-engine` **at the renderer's own version**:
```json
{ "module": { "component_api": {
  "source": "git::https://github.com/<org>/idp-engine.git//modules/github/component?ref=v0.3.0",
  "name": "api",
  "owner_team": "platform",
  "environments": { "pro": { "protected": true, "variables": {
    "AWS_ROLE_ARN":   "arn:aws:iam::000000000003:role/api-pro-ci",
    "ECR_REPOSITORY": "000000000003.dkr.ecr.eu-west-1.amazonaws.com/api",
    "AWS_REGION":     "eu-west-1"
  }}}
}}}
```
All resource logic lives in the modules and is tested with `tofu test` (§8.3). When
Renovate bumps the engine, the re-render changes every `ref`, so the upgrade shows up
as a plan in a PR.

Each stack also gets the engine's `.terraform.lock.hcl` copied in, so `tofu init`
verifies provider checksums.

### 5.4 Layout of the `wet` branch
```
rendered/
  github/
    main.tf.json                      # provider (App token), local backend, one module per claim
    .terraform.lock.hcl
    files/<component>/<path>          # real rendered feature content
  aws/
    <env>/
      _baseline/main.tf.json          # GitHub OIDC provider for the env account
      components/<name>/main.tf.json  # ECR + CI role
      workspaces/<name>/main.tf.json  # Workspace module call
tfstate/github.tfstate                # encrypted (§7.2)
.idp/manifest.json                    # engine version, source main SHA, installed userManaged files
```
Putting each kind in its own folder lets a Component and a Workspace share a name.

### 5.5 Deterministic identities, with no shared state between stacks
The GitHub stack needs AWS values (the role ARN and the ECR URL). Those values are
**computed by `internal/naming`** from the platform config, never read from AWS
state. The AWS state lives in an ephemeral emulator, so it cannot be read later.
- Role: `arn:aws:iam::<accountId>:role/<component>-<env>-ci`. It is at most 54
  characters, within IAM's 64-character limit, given the name rules in §4.2 and §4.6.
- ECR: `<accountId>.dkr.ecr.<region>.amazonaws.com/<component>`
- OIDC provider: `arn:aws:iam::<accountId>:oidc-provider/token.actions.githubusercontent.com`

Stacks have no data dependencies on each other, so they plan in parallel and their
order does not matter.

### 5.6 Emulator specifics belong to CI, not to the render
Rendered stacks do not know floci exists. In CI, the workflow:
- Starts floci as a service container (image pinned by digest, port 4566).
- Sets `AWS_ACCESS_KEY_ID=<accountId>`. floci uses a 12-digit access key ID directly
  as the account ID, and isolates resources per account. One floci therefore serves
  every env with correct ARNs.
- Writes `floci_override.tf.json` (endpoint and provider flags). Override files are
  native to OpenTofu.

Going to real AWS means replacing this step with OIDC credentials. Claims and
renders do not change.

### 5.7 Features
**Format** (owned by `idp-engine`, `apiVersion: idp/v1`):
```yaml
apiVersion: idp/v1
name: codeowners
inputs:
  owner_team: { from: claim.owner }     # claim.name | claim.owner | claim.environments
  extra_paths: { arg: { type: array, items: { type: string }, default: [] } }
files:
  - { template: CODEOWNERS.tmpl, path: .github/CODEOWNERS, mode: managed }
```
- `arg` is a JSON Schema fragment, and `default` makes it optional. `claim.owner`
  resolves to the Group name.
- Templates use Go `text/template` with delimiters `{{|` and `|}}`, so they never
  clash with `${{ }}` in workflows.
- Template data is `{ org, component: { name, environments }, inputs }`.
- The function set is the `text/template` builtins plus `join` and `toJson`.

**`idp-features` repo layout:**
```
features/<name>/{feature.yaml, templates/, tests/<case>/{inputs.yaml, expected/...}, README.md, CHANGELOG.md}
release-please-config.json, .release-please-manifest.json
.github/workflows/{verify.yaml, release-please.yaml}
```

**Materialization.** Rendered files are written to
`rendered/github/files/<component>/<path>`, and the component module creates
`github_repository_file` resources from them. So reviewing a `wet` diff means seeing
exactly what will land in the repo.

**Modes.** The resource names match Firestartr's, and the files are always written
to the default branch.
- **`managed`**: `github_repository_file.managed[...]` with
  `overwrite_on_create = true`. Manual edits are reverted on the next apply.
- **`userManaged`**: `github_repository_file.user_managed[...]`. The mechanism is
  the one Firestartr uses (`ghfeature/helpers/managed_files.ts:76-81`):
  1. The file is created **once**.
  2. Right after the apply, `tofu state rm` removes it from state, and
     `<component>:<path>` is recorded in `.idp/manifest.json`.
  3. Later renders skip it. If the file already existed, `idp adopt` records it
     without writing anything.
  4. Manifest entries are removed when their Component is deleted.
- Why not `lifecycle { ignore_changes = [content] }`? Because removing the feature
  would **delete** the user's file, and deleting the file by hand would bring it back.
- Why not a `removed` block? Because its `from` "cannot include instance keys",
  and these files use `for_each`.

**v1 features:**

| Feature | Files | Notes |
|---|---|---|
| `codeowners` | `.github/CODEOWNERS` (managed) | `* @<org>/<owner_team>`, plus the `extra_paths` arg |
| `release-please` | the workflow (managed), `release-please-config.json` (managed), `.release-please-manifest.json` (**userManaged**, because release-please bumps it) | Needs the org setting that lets Actions create PRs (bootstrap, §9.2) |
| `container-ci` | `.github/workflows/container-ci.yaml` (managed) | arg `registry: ghcr \| ecr`, default `ghcr`. The `ecr` path uses the env variables from §4.4. In v1 it is only checked statically (actionlint): the ECR lives in an ephemeral emulator in another repo's CI, so a component's CI cannot push to it |

## 6. Pipelines

The guiding rule: **nothing is ever applied that nobody saw.**

`idp-claims` contains three thin workflows. Each one is
`uses: <org>/idp-engine/.github/workflows/<x>.yaml@vX.Y.Z`, and Renovate bumps the
ref. Each workflow sets `permissions: {}`, and each job asks for what it needs.

### 6.1 PR (`pull_request` → `main`)
Concurrency: `idp-plan-<pr-number>`, `cancel-in-progress: true`.
1. **validate**: runs §4.7 and emits annotations.
2. **render + diff**: `fetch` → `adopt` (the manifest is not committed) → `render`
   → `diff` against `wet`. This produces a matrix of affected stacks, orphans
   included.
3. **plan-github**: decrypts the state from `wet` and uses the `idp-reader` token.
4. **plan-aws** runs as a matrix, with one floci per job:
   1. Apply the env's `_baseline`.
   2. Apply the `wet` version of the stack, if it existed and is not `observe`.
   3. Plan the new version. An orphan is planned as a destroy, using its **old**
      config from `wet`.
5. **policy** (`idp plan-summary` over `tofu show -json`) enforces the §4.5 table.
6. **sticky comment** on the PR, posted with `GITHUB_TOKEN` and
   `pull-requests: write`:
   - A summary per stack (`+N ~M -D`), with destructive actions flagged.
   - A hidden **fingerprint**: the sorted set of `(address, action)` pairs per
     stack, excluding `no-op` and `read`, stored as `<!-- idp-fingerprint:v1 … -->`.
7. **`idp-gate`** is the only required status check, because matrix job names vary.

**PRs from forks** only run validate and render. Plan jobs require
`head.repo.full_name == github.repository`, and `idp-gate` fails with "a maintainer
must push this branch to the repo". `pull_request_target` is never used.

### 6.2 Reconcile (`push` → `main`, and `workflow_dispatch` on `main`)
This is one workflow with two triggers. The dispatch is the universal recovery
button. Concurrency: group `idp-wet`, `cancel-in-progress: false`, `queue: max`.

1. **plan** (no environment; reader token and passphrase as repo secrets):
   1. Re-render and diff against the **current** `wet`. Truth is `wet` now, not
      what the PR saw.
   2. Plan every affected stack, and save the GitHub plan as an artifact. The plan
      is encrypted by OpenTofu, and its retention is 1 day.
   3. Upload the render as an artifact, including the working manifest that came
      out of `adopt`. Apply jobs consume this artifact and **never re-render**.
   4. Compute the fingerprints.
   5. `idp gate` decides **`auto`** when:
      - there are no delete or replace actions (a repo archive counts as a delete),
        **and**
      - every stack's fingerprint is a subset of that stack's fingerprint in the PR
        comment. The PR is found with `GET /repos/{o}/{r}/commits/{sha}/pulls`. A
        stack missing from the PR comment is not a subset.

      In any other case, including when no PR is found, it decides **`approval`**.
      Example: A and B merge back to back. B's PR plan showed A+B. After A applies,
      B's plan shows only B, which is a subset, so it runs automatically.
2. **approve** runs only when `gate == approval`. It references environment
   `idp-approval` (required reviewers, set by bootstrap to the platform admins;
   deployment branch `main`; **no secrets**). Required reviewers gate each job that
   references the environment, so keeping approval in this single, secret-less job
   gives exactly one approval per run.
3. **apply-aws** is a matrix with `fail-fast: false` and no environment. It runs
   only when `plan` succeeded and either `gate == auto` or `approve` succeeded.
   Each job replays and re-plans in its own floci. It requires the fingerprint to
   **equal** the one that was gated, then applies and writes evidence to the job
   summary. `observe` Workspaces are never applied: for them, a successful plan
   counts as success.
4. **apply-github** references environment `idp-write` (deployment branch `main`;
   no reviewers; it holds the `idp-writer` key). It runs after `apply-aws`
   (`if: always()` plus the same gate condition). Stacks are independent (§5.5), so
   an AWS failure does not block GitHub.
   1. It applies the **saved plan**, or skips this step when the GitHub stack has
      no changes. OpenTofu refuses a plan whose state changed in the meantime
      ("Saved plan is stale"), even when the plan is encrypted.
   2. It runs `state rm` for `userManaged` files created in this apply, and updates
      the manifest.
   3. It makes **one commit to `wet`**, with `if: always()`:
      - the GitHub tfstate, **always**, even if the apply failed, because real
        resources may have changed;
      - `rendered/` for the stacks that applied successfully;
      - the manifest.

      A stack that failed keeps its old render in `wet`, so the next run sees the
      difference and retries it. **The system self-heals.**
   4. If the push fails, it uploads the encrypted state as an artifact, opens an
      issue that links the runbook, and fails.

### 6.3 Deletions
Any delete or replace goes through `idp-approval`.
- **Workspace.** When the claim file is gone, the policy is read from the **last
  render in `wet`**.
  - `full-control`: the destroy is planned and goes through approval.
  - `apply`: the PR fails. Deleting it takes two PRs: change the policy, then
    delete the claim.
  - `observe`: it was never applied, so its render is removed from `wet` without
    any destroy.
- **Component.**
  - The repo is archived (`archive_on_destroy`), and its ECR and CI roles are
    destroyed.
  - OpenTofu destroys dependents first. So the last commit of the archived repo
    deletes the `managed` files, while `userManaged` files stay.
  - This is accepted and documented for v1. Avoiding it would need a two-step
    deletion (YAGNI for now).
- **Group.** The team is deleted.

### 6.4 Drift (daily cron)
- It runs on `cron: '23 5 * * *'`, avoiding the top of the hour, and in concurrency
  group `idp-wet`, so it never reads state in the middle of an apply.
- It plans the GitHub stack with `idp-reader` and runs `idp bootstrap check` (§9.2).
- If it finds drift, it creates or updates **one** issue labeled `drift` with the
  plan. If there is no drift, it closes that issue. Issues are handled with
  `GITHUB_TOKEN` and `issues: write`.
- **There is no auto-remediation.** A human either fixes the claims or runs
  reconcile.
- Scheduled workflows in public repos are disabled after 60 days without activity.
  Renovate PRs keep the repo active, and this is documented.

## 7. State and security

### 7.1 Threat model
Everything is public: code, `wet`, logs, artifacts and comments. The design protects
**who can write**, and limits the blast radius of a leaked credential.
- **Fork contributors** get nothing (§6.1).
- **Members who can push branches** can propose claims, and a plan executes module
  code. That is why modules are allowlisted and pinned, plan-time tokens are
  read-only, and the write key is out of reach of PR runs.

### 7.2 Encrypted state
- The render emits `encryption { state { enforced = true } plan { enforced = true } }`.
  The key material (a PBKDF2 passphrase of at least 16 characters, with AES-GCM)
  comes from `TF_ENCRYPTION`, filled from a secret. If the variable is missing,
  OpenTofu refuses to write plaintext.
- AES-GCM is authenticated encryption, so tampered state fails to decrypt instead
  of being read.
- **Honest note:** today the GitHub state is almost all public information. The
  encryption buys **integrity**, and readiness for private repos.
- Each `wet` commit is a version of the state, which gives an audit trail and a
  backup. Restoring state is **not** a `git revert`; there is a runbook for it.
- Rotation uses OpenTofu's `fallback` block (runbook).

### 7.3 Two GitHub Apps and three secret scopes

| Credential | Permissions | Where it lives |
|---|---|---|
| `idp-reader` App key | Read only | Repo secret: PR plans, reconcile plan, drift |
| `idp-writer` App key | Write, **including `Workflows`** (features write `.github/workflows/`; `Contents` alone is rejected) | **Only** in environment `idp-write` (deployment branch `main`, no reviewers) |
| State passphrase | — | Repo secret, needed to read state in plans |
| Approval | — | Environment `idp-approval` (required reviewers, `main`, no secrets) |

- `idp-reader` and `idp-writer` are **roles**. GitHub App names are unique across
  GitHub and limited to 34 characters, so the real Apps are named
  `<org>-reader` and `<org>-writer`. For example, `jellalshadows-idp-writer` is 24
  characters. Both claims repos (`idp-claims` and `idp-claims-e2e`) share the same
  two Apps, because Apps are installed per org.
- Tokens are minted per job with `actions/create-github-app-token` and expire after
  1 hour. A longer apply is a known limitation, and Phase 0 measures how long
  applies take.
- **Accepted risk:** the passphrase reaches PR plans. If it leaks, someone can read
  near-public state, but they still cannot write to `wet`.

### 7.4 Rulesets
- **`idp-claims` `main`:** PR required, `idp-gate` required, branch up to date, no
  force push, no deletion, **no bypass**. Recovery goes through reconcile, not
  through skipping rules.
- **`idp-claims` `wet`:** only `idp-writer` may update it; no force push, no
  deletion.
- **Tags in `idp-engine`, `idp-features` and `idp-modules`:** a published tag never
  moves.

### 7.5 Supply chain
- Actions are pinned by SHA, and Renovate updates them along with the version
  comment.
- `actionlint` and `zizmor` run on the engine's workflows.
- Provider versions are exact, and lock files are shipped (§5.3).
- The OpenTofu version is pinned exactly (≥ 1.12, chosen in Phase 0).
- Every pinned version (actions, tools, providers, images, Go modules) must be at
  least 14 days old when it is adopted. Renovate enforces this with
  `minimumReleaseAge`.
- The CLI is released with checksums and a provenance attestation
  (`actions/attest-build-provenance`). The workflows run `gh attestation verify`
  before they execute it.

### 7.6 No secrets flow through the IDP in v1
- Claims have no secret fields. Firestartr's `secrets.actions` is left out on
  purpose.
- floci uses fake 12-digit credentials. **There is no real cloud credential anywhere
  in v1.**
- The path to real AWS is OIDC from `idp-claims`, never static keys.

### 7.7 Runbooks
Runbooks live in the repo and are written in English:
- Rotate the passphrase.
- Rotate the App keys.
- Recover lost or corrupt state with `import` blocks.
- Unblock a failed apply with reconcile.
- Recover from a failed `wet` push.
- Bootstrap a new org.

## 8. Testing

| Level | What | Local | CI (PR) | Nightly |
|---|---|---|---|---|
| 1. Go unit + golden | CLI logic, render | ✅ | ✅ | |
| 2. `tofu test` + `mock_provider` | Module logic | ✅ (needs the `tofu` binary only) | ✅ | |
| 3. AWS integration | `modules/aws/*`, `idp-modules` against real floci | ❌ | ✅ | |
| 4. E2E | Whole flow, PR → merge → apply → cleanup, in `idp-claims-e2e` | ❌ | engine release PR | ✅ |

### 8.1 Practice
Strict TDD (red → green → refactor) for Go and for module assertions. Go tests follow
the owner's `go-testing` skill: table-driven tests, golden files with `-update`,
`t.TempDir()`, interfaces at exec boundaries, and `-short` to skip integration.

### 8.2 Level 1: Go
- Table-driven tests for validation, naming, policy verdicts, fingerprint subset,
  gate decisions and diff (changes and orphans).
- Golden render cases live in `testdata/<case>/{claims/, config/, expected/}`. Golden
  diffs are reviewed in PRs.
- **Determinism test:** rendering twice, and with a shuffled file-read order, gives
  identical bytes.
- `plan-summary` fixtures are real `tofu show -json` outputs, captured once in CI
  and committed.

### 8.3 Level 2: modules
`tofu test` with `mock_provider` and `command = plan` asserts the decisions in this
spec. For example:
- `idp-writer` is in the ruleset bypass.
- A protected env has the owner team as a reviewer.
- The trust policy `sub` is `repo:<org>/<name>:environment:<env>`.
- `archive_on_destroy` follows the platform config.

### 8.4 Level 3: floci
`tofu test` with `command = apply` against a floci service container: apply, assert,
destroy. It also includes a **replay test**: apply the old version, plan the new one,
and check that the diff is exactly the expected one.

### 8.5 Level 4: E2E
The harness targets `<org>/idp-claims-e2e`, using the reusable workflows at the
candidate SHA. This mirrors Firestartr's `smoke-tests/scripts/smoke.sh`. The
harness:
1. Creates a branch with fixture claims and opens a PR.
2. Waits for `idp-gate` and checks the plan comment.
3. Merges and waits for reconcile.
4. Asserts with `gh api` that the repo, team, ruleset, environments and files exist.
5. Opens a PR that deletes the claims and verifies the cleanup.

`idp-claims-e2e` uses `archiveOnDestroy: false`. The harness runs nightly and on
engine release PRs, not on every PR, to protect the rate limit and CI minutes.

**One org, isolated by repo and prefix (owner decision, 2026-10-09).** There is no
separate sandbox org, so the isolation is enforced in three ways:
- `idp-claims-e2e` has its own `wet` branch, encrypted state and passphrase, so
  production state and test state never mix.
- Every resource the harness or a spike creates is prefixed `e2e-` or `spike-`, and
  the harness refuses to delete anything without that prefix.
- `idp-claims` validation rejects claim names that start with `e2e-` or `spike-`.

What a sandbox org would have added, and is accepted as lost:
- The Apps and their keys are shared between production and tests.
- Org-level settings (Actions permissions, invitations) are shared.
- The rate limit is shared.

### 8.6 Features and workflows
- Feature golden tests run with `idp feature test`.
- `actionlint` runs on the **rendered** workflows, not just the templates.
- There is no coverage threshold (coverage is published, not gated), and no fuzzing
  in v1.

## 9. Releases, bootstrap, docs

### 9.1 Releases
- **`idp-engine`** uses release-please.
  - Publishing runs in the **same workflow**, gated on `release_created`, with no
    `workflow_run` cascade and no PAT.
  - GoReleaser uploads binaries and checksums to that release, and the build
    provenance is attested.
- **Versioning** is `0.x` while v1 is being built: breaking changes bump the minor.
  `apiVersion: idp/v1` freezes at `1.0.0`.
- **`idp-features` and `idp-modules`** are released multi-component.
- **`idp-claims`** is never released.

### 9.2 Bootstrap (outside the IDP on purpose)
The IDP must not manage its own protections. Otherwise a PR could disable the
checks that guard it.
The bootstrap is the `idp bootstrap` subcommand, written in Go. This was amended
from a bash script: a Go implementation can be TDD'd locally, and the
desired-vs-actual JSON comparison that `check` needs is natural in Go.
- **`idp bootstrap app`:** runs GitHub's manifest flow through a local callback
  server. A person still confirms in the browser, as GitHub requires. The manifests
  are versioned in `idp-engine/bootstrap/apps/{reader,writer}.json`, so the
  permissions are reviewable code.
- **`idp bootstrap apply`** (idempotent) creates:
  - the claims repo and its `wet` branch;
  - the rulesets;
  - the environments `idp-approval` and `idp-write`;
  - the secrets and variables;
  - the org-level Actions workflow defaults (read-only).
- **Deferred to Phase 4:** allowing Actions to create PRs, which the
  `release-please` feature needs. GitHub exposes that permission and approving PRs
  as one setting (`can_approve_pull_request_reviews`), so enabling it weakens
  required reviews in Component repos. It is turned on only when the feature
  lands, and its ADR records the risk.
- **`idp bootstrap check`** reports drift in these protections. Drift runs it
  (§6.4).
- Why not an OpenTofu stack? It would need its own state somewhere: turtles all the
  way down.

### 9.3 Docs
All repos are licensed Apache-2.0. Docs are in English and live in each repo:
- **A README per repo**, with the limitations up front.
- **`docs/architecture.md`**, with Mermaid diagrams.
- **ADRs from day one:**
  1. Edge-triggered Actions instead of an operator.
  2. Encrypted state in Git.
  3. `wet` branch plus one PR.
  4. floci multi-account.
  5. `.tf.json` output.
  6. Deterministic identities.
  7. `userManaged` via `state rm`.
  8. No dflook.
  9. Two Apps and split environments.
  10. Public repos because of the Free plan.
  11. Bootstrap outside the IDP.
  12. Required policy with no default.
- **The claims reference**, the feature-authoring guide and the runbooks.
- **`docs/firestartr-comparison.md`:**
  - What was kept: claims, policies, `userManaged`, the feature format.
  - What changed: edge vs level triggering, Git state, a single PR.
  - What is lost: continuous reconciliation, multi-cloud, scale.

## 10. Roadmap

This spec is the umbrella design for v1. **Implementation plans are written one
phase at a time.** Each plan starts once the previous phase has shipped, so it can
use what that phase measured. Each phase ships an engine minor release.

| Phase | Delivers | Exit criteria |
|---|---|---|
| 0 | Bootstrap + spike | Orgs, Apps and repos exist. An ADR with measurements: encrypted state round-trip via Git, the GitHub provider with an App token, apply duration vs the 1-hour token, floci replay time per stack, and pending-invite behavior |
| 1 → `0.1.0` | Group + Component (GitHub part), all pipelines (§6), **E2E harness born** | E2E creates, modifies and deletes an `e2e-` repo and team through `idp-claims-e2e` |
| 2 → `0.2.0` | Component AWS part: baseline, ECR + role, deterministic env variables | E2E checks the env variables, and floci integration is green |
| 3 → `0.3.0` | Workspace: allowlist, policies, orphans, `idp-modules/s3-bucket` | E2E covers the two-step delete and the `apply`-policy rejection |
| 4 → `0.4.0` | Features: `idp-features` + the 3 features, managed and userManaged | Golden tests, plus E2E with one managed and one userManaged file |
| 5 → `1.0.0` | Polish: complete docs, Firestartr comparison, tested runbooks | An external person bootstraps a new org using only the README |

## 11. Risks (verified in Phase 0 or by E2E)

| # | Risk | Mitigation, or fallback if it materializes | Phase 0 result |
|---|---|---|---|
| 1 | A Group member who is not yet in the org gets an invitation, and the membership stays "pending", which could leave a permanent plan diff | Fallback: validation requires members to already be org members (checked with the reader token) | Closed: no fallback needed. The plan exits 0 both with the invitation pending and after acceptance. See [ADR-0013](../../adr/0013-phase-0-spike-findings.md) |
| 2 | A GitHub apply takes longer than the 1-hour installation token | Measure in Phase 0. Fallback: the provider's `app_auth` with the key file inside `idp-write` | Closed: 32 s for 10 resources is 3.2 s per resource; 3.2 s x 450 is about 24 min, under the 30 min threshold. Keep minted installation tokens, no `app_auth`. See [ADR-0013](../../adr/0013-phase-0-spike-findings.md) |
| 3 | floci replay time multiplied by the number of stacks drives CI minutes | Measure in Phase 0. Only affected stacks are planned | Measured in S3: about 26 s per stack (init 7 + apply 13 + plan 6) plus 6 s floci readiness per job. Acceptable |
| 4 | floci fidelity gaps (IAM/ECR) | The OIDC provider is already confirmed to exist in floci. Gaps get documented and skipped in level 3 | No gaps found for IAM, OIDC and ECR in S3. ECR URL uses the `localhost:4566` host form (cosmetic). Real `AssumeRoleWithWebIdentity` not exercised |
| 5 | `userManaged` adoption edge cases (file deleted between adopt and apply) | `overwrite_on_create = false` on `user_managed`, so the apply fails loudly and the next reconcile retries | Not in Phase 0 scope: covered in Phase 4 (Features) |
| 6 | Cron disabled after 60 days of inactivity | Renovate activity. Documented | Not in Phase 0 scope: covered in Phase 1 (drift pipeline) and Phase 5 (docs) |

## 12. Inputs (resolved 2026-10-08, amended 2026-10-09)
- A single org, `jellalshadows-idp`, created by hand because Free orgs cannot be
  created through the API. The 2026-10-08 plan for a separate sandbox org was
  dropped by the owner on 2026-10-09; tests are isolated by repo and prefix (§8.5).
- Test account for Group members and the pending-invite spike: `adrian-da-silva`.
  The approver and platform admin is `jellalshadows`.
- Timing: this runs in parallel with `chart-base`.

## 13. Firestartr mapping (quick reference)

| Firestartr | Here |
|---|---|
| Claims repo → cdk8s renderer → CRs in a "state" repo → Argo CD → operator | Claims (`main`) → `idp render` → `wet` branch → reconcile workflow |
| Two PRs (claims PR, then hydrate PR) | One PR; `wet` is written by the reconcile |
| `TFWorkspaceClaim` with `lifecycle` / `context.providers` / `context.backend` | `Workspace`; the env comes from the folder, and providers come from the platform config |
| `.firestartr` platforms with `envs` + `allowedClaims` | `config/platform.yaml` with `environments` + `modules.allowedSources` |
| Policies `full-control` / `apply` / `observe` / `create-only`; default `observe` | `full-control` / `apply` / `observe`; **required**, no default; no `create-only` in v1 |
| Features from `prefapp/features`, ref `<name>-v<version>`, rendered to `github_repository_file` | Same model in `idp-features`, with `{{\| \|}}` delimiters |
| `userManaged` via state rm + `installed_managed_files` output | `userManaged` via state rm + `.idp/manifest.json` |
| Level-triggered operator with periodic sync | Edge-triggered: PR, merge and dispatch; daily drift is report-only |
