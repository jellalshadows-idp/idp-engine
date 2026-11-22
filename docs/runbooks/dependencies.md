# Runbook: dependency updates

Renovate runs self-hosted from `idp-engine`, as the bot App, over `idp-engine` and `idp-claims` ([ADR-0029](../adr/0029-self-hosted-renovate.md)). `idp-claims-e2e` is left out: the end-to-end harness pins its callers on every run.

## When it runs

Weekly, on Mondays at 04:17 UTC, and on demand:

```bash
gh workflow run renovate.yaml --repo jellalshadows-idp/idp-engine --ref main
```

`.github/workflows/renovate.yaml` runs in the `idp-renovate` environment (deployment branch `main`) and mints a bot token for the two repositories only. Its whole configuration is `.github/renovate-global.json`; the repositories carry no Renovate file, and one added to a repository is ignored.

## What it updates

| Dependency | Where | Pull request |
|---|---|---|
| Actions, pinned by SHA with `# vX.Y.Z` | `.github/workflows/`, `.github/actions/setup-idp/` | one per action |
| The engine's reusable workflows | the four caller workflows of `idp-claims`; the templates in `examples/claims-repo/` | group "idp engine", no waiting period |
| OpenTofu | `setup-idp`, `ci.yaml`, `TofuVersion` in `internal/render/render.go` and the golden renders | group "opentofu" |
| The GitHub provider | `modules/github/*/versions.tf`, `GitHubProviderVersion` in `render.go` and the golden renders | group "github provider", label `needs-lockfile` |
| The AWS provider | `modules/aws/*/versions.tf`, `AWSProviderVersion` in `render.go` and the golden AWS stacks | group "aws provider", label `needs-lockfile` |
| Go modules | `go.mod` and `go.sum` (`go mod tidy`) | one per module |
| Go | the `go` directive in `go.mod`, which CI and `setup-idp` install | patch updates only |
| GoReleaser | the GoReleaser action's `version` in `ci.yaml` and `release.yaml` | one |
| actionlint, zizmor | `ci.yaml` | one each |
| floci | the `floci` service image of `ci.yaml`, `pr.yaml` and `reconcile.yaml` (four jobs), tag and digest | one: the three files move together, and `TestFlociJobsHoldNoSecret` fails otherwise |
| Renovate | `renovate.yaml` and the validator image in `ci.yaml` | one |

Runner images (`runs-on`) are not updated. Every update waits 14 days after its release; engine releases do not wait. Renovate opens every pending update in the run, with no limit on pull requests per hour or open at once (`prHourlyLimit: 0`, `prConcurrentLimit: 0`), so the engine bump never waits behind third-party PRs. Pending updates are listed in the "Dependency Dashboard" issue of each repository.

## Reviewing and merging

Renovate never merges. Wait for the PR's checks, read it, and merge it with a merge commit: `gh pr merge <number> --repo <owner>/<repo> --merge --delete-branch`. In `idp-claims` a PR merges only after `idp-gate`; for an engine upgrade the plan comment shows only the resource changes the new engine makes, usually `No infrastructure changes.`, and the version jump shows in the PR diff and the release notes ([Upgrading the engine](../pipelines.md#upgrading-the-engine)).

Renovate sets no commit statuses (the bot can read statuses, not write them), so there is no `renovate/stability-days` check on its branches.

## Setting up the environment

```bash
gh api -X PUT repos/jellalshadows-idp/idp-engine/environments/idp-renovate \
  -F 'deployment_branch_policy[protected_branches]=false' -F 'deployment_branch_policy[custom_branch_policies]=true'
gh api -X POST repos/jellalshadows-idp/idp-engine/environments/idp-renovate/deployment-branch-policies -f name=main -f type=branch
gh secret set IDP_BOT_PRIVATE_KEY --env idp-renovate --repo jellalshadows-idp/idp-engine < ~/.idp/apps/jellalshadows-idp-bot.pem
gh variable set IDP_BOT_CLIENT_ID --env idp-renovate --repo jellalshadows-idp/idp-engine --body "<client_id from jellalshadows-idp-bot.json>"
```

## A GitHub provider update (`needs-lockfile`)

Renovate moves the provider pins but not the shipped lock file, so the PR's `go` and `modules` jobs fail (`TestRenderCopiesTheLockFile`, `TestModulesPinTheirLockedProvider`, `tofu init -lockfile=readonly`). Regenerate the lock on the PR branch, in Git Bash, with OpenTofu at the version pinned in `render.TofuVersion` (and `ci.yaml`), written `<version>` below:

```bash
gh pr checkout <number> --repo jellalshadows-idp/idp-engine
cd modules/github/group
rm -f .terraform.lock.hcl
"$HOME/.idp/tools/tofu-<version>/tofu.exe" providers lock -platform=linux_amd64 -platform=windows_amd64 -platform=darwin_amd64 -platform=darwin_arm64
mv .terraform.lock.hcl ../../../lockfiles/github.terraform.lock.hcl
cd ../../..
```

The four platforms are the runners (linux amd64) and the owner's machines. Then set the new version in `lockfiles/lockfiles_test.go` (`TestGitHubLockPinsTheProvider`), run `go test ./lockfiles/ ./internal/render/`, commit as `chore(deps): regenerate the github provider lock file` and push. The checks run again.

## An AWS provider update (`needs-lockfile`)

Renovate moves the pins in `modules/aws/*/versions.tf`, `AWSProviderVersion` and the golden AWS stacks, but not `lockfiles/aws.terraform.lock.hcl`, so the PR's `go` job fails (`TestModulesPinTheirLockedProvider`, `TestRenderCopiesTheLockFile`), and so do `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke` and `floci` (`tofu init -lockfile=readonly`). Regenerate the lock the same way, from the component module:

```bash
gh pr checkout <number> --repo jellalshadows-idp/idp-engine
cd modules/aws/component
rm -f .terraform.lock.hcl
"$HOME/.idp/tools/tofu-<version>/tofu.exe" providers lock -platform=linux_amd64 -platform=windows_amd64 -platform=darwin_amd64 -platform=darwin_arm64
mv .terraform.lock.hcl ../../../lockfiles/aws.terraform.lock.hcl
cd ../../..
```

Then set the new version in `lockfiles/lockfiles_test.go` (`TestAWSLockPinsTheProvider`), run `go test ./lockfiles/ ./internal/render/ ./internal/workflowspec/ ./internal/floci/`, commit as `chore(deps): regenerate the aws provider lock file` and push. The registry signs the checksums of every package of a release, and OpenTofu records them all whatever the `-platform` flags say; the flags keep one command for both providers.

## A floci update

Renovate moves the image's tag and digest together, and the `floci` job is the check. Read floci's release notes for IAM and ECR changes before merging: the floci tests compare names, ARNs and policies, never the registry URL, whose host floci chooses (ADR-0013).

Review a floci update as a security change too: the `floci` job mounts the runner's Docker socket into the floci container, which gives that image root-equivalent control of the runner for the job's duration. The job holds no secret (`contents: read`, `persist-credentials: false`, fake AWS keys) and saves no Go cache. floci's ECR also pulls its backing registry image, `registry:2`, by tag through that socket; this repository does not pin it (accepted in Phase 2c, D-11).

## An OpenTofu update

One PR moves every pin. Update the local tool under `$HOME/.idp/tools/` to the same version before running module tests locally.

## When a run fails

- `the idp-renovate environment is missing: …`: set it up as above.
- `mint a token …` or a 401: a wrong `IDP_BOT_CLIENT_ID`, a rotated key, or the bot App uninstalled from one of the two repositories ([the bot App](bootstrap.md#the-bot-app)).
- A repository is skipped with an error in the log: read the Dependency Dashboard of that repository; Renovate lists there what it could not do.
- GitHub disables a scheduled workflow after 60 days without activity in a public repository; Renovate PRs keep `idp-claims` active. Re-enable a disabled schedule with `gh workflow enable <workflow> --repo <owner>/<repo>`.
