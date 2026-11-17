# Phase 2c — The AWS Modules and the floci Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the two AWS modules of spec §3.3 and prove them against floci. `modules/aws/baseline` creates an environment account's GitHub Actions OIDC provider. `modules/aws/component` creates a Component's ECR repository and its CI role, which may only push to that repository and which only a GitHub Actions job of the Component's repository, in its environment, can assume (spec §4.4). The AWS lock file ships next to the GitHub one. A new CI job, `floci`, applies both modules to two accounts of one floci, with the inputs `internal/naming` computes, checks the created identities and policies through the AWS API, replays them and destroys them (spec §8.4). Claims validation now gives every environment its own AWS account in the standard partition. No render calls the modules yet (Phase 2d), and the pipelines are unchanged (Phase 2e).

**Architecture:**
- **`modules/aws/baseline`** has no inputs and one resource, `aws_iam_openid_connect_provider.this`, for `https://token.actions.githubusercontent.com` with the client id `sts.amazonaws.com`. Its ARN is `naming.OIDCProviderARN(accountId)`.
- **`modules/aws/component`** takes the identities as inputs, which the renderer will fill from `internal/naming` in 2d: `repository_name`, `role_name` and `oidc_provider_arn`. It builds only the trust policy's subject, from `github_repository` (`<org>/<name>`) and `environment`. Its tests compare whole policy documents, so nothing can be added unseen.
- **`lockfiles/aws.terraform.lock.hcl`** is embedded as `lockfiles.AWS`. CI's `modules` job picks each module's lock file from its provider directory (`modules/<provider>/<name>`).
- **`internal/floci`** is a test-only package. Its offline tests run in every `go test ./...`; `TestFloci` runs only when `IDP_FLOCI_ENDPOINT` is set, which only CI's `floci` job does. It writes each stack's `main.tf.json` the way the render will, plus `floci_override.tf.json` and the lock file, drives `tofu`, and queries floci with the AWS CLI.
- **`claims.Load`** gains one platform check, `checkAWSAccounts`: each environment's `aws` block needs its own account and a region of the standard `aws` partition.
- The two items Phase 2b carried into CI and the render tests are closed: `render-smoke` holds its fixture list once, and a loop variable no longer shadows another.

**Tech Stack:** Go 1.26 (standard library; no new dependency), OpenTofu modules on `hashicorp/aws` 6.71.0 (new; `integrations/github` 6.13.0 unchanged), floci 2.4.0 as a GitHub Actions service container, the AWS CLI of the `ubuntu-24.04` runner image. Local `tofu test` with OpenTofu 1.12.6; CI runs 1.13.2 (D-18).

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §3.3, §4.4, §4.7, §5.4, §5.5, §5.6, §6.3, §7.5, §8.3, §8.4, §10, §11 and amendment A12. Context: `docs/phases/phase-2a.md` and `docs/phases/phase-2b.md` ("Carried into later sub-phases"), ADR-0004, ADR-0006, ADR-0013, ADR-0029, `docs/runbooks/dependencies.md`, the Phase 0 spike (`jellalshadows-idp/idp-spike`: `aws/main.tf`, `ci/floci_override.tf.json`, `.github/workflows/spike-floci.yaml`).

## Phase 2 split (owner decision, 2026-11-03)

Phase 2 is delivered as six plans, each written after the previous one ships (Phase 2a ruling D-1, spec §10 under A10).

| Plan | Delivers |
|---|---|
| 2a (shipped, PR #29, `fca5c0b`) | The AWS model of the Platform and the Component, `internal/naming`, the schema and semantic validation, spec amendment A10 |
| 2b (shipped, PR #35, `c6aa267`) | The deterministic env variables in `modules/github/component`, rendered from `internal/naming`; spec amendment A11 |
| **2c (this plan)** | `modules/aws/{baseline,component}` (the OIDC provider; the ECR repository and the CI role), the AWS lock file and the floci CI job (spec §8.4); one account per environment; the 2b review items; spec amendment A12 |
| 2d | Rendering the AWS stacks (`aws/<env>/_baseline`, `aws/<env>/components/<name>`) |
| 2e | The workflows' AWS plan and apply against floci |
| 2f | The E2E env-variable checks, the `0.2.0` release and the Phase 2 log |

## Decisions taken while planning

The executor copies these into the ledger as rulings before Task 1. Phase 2a's owner decisions still hold: Phase 2 is split (2a D-1), and v1 is floci only, with no real AWS account (2a D-2). D-1 to D-3 apply the controller's rulings; the rest were decided while planning, from facts read live on 2026-11-17.

- **D-1. Two environments may not share an `accountId` (controller ruling).** Spec §5.4 renders one `_baseline` per environment, and IAM identifies an OIDC provider by its URL, so an account holds one: a second environment in that account would create it again and fail (`EntityAlreadyExists` on AWS, and the same from floci 2.4.0, whose `docs/services/iam.md` says "Creating the same URL twice returns `EntityAlreadyExists`"). The registry Components of both environments would also collide on the repository name. `claims.Load` reports it in `config/platform.yaml`, at the `accountId` line of the later environment in file order, and names the first: `environment "pro" uses accountId "000000000001", like environment "dev": each environment needs its own AWS account, because each environment's baseline creates that account's GitHub OIDC provider (spec §5.4)`. It covers every environment with an `aws` block, used by a registry Component or not, because 2d renders a baseline per such environment. This closes 2a's D-11; strict is the reversible choice. Live facts: `idp-claims`'s `config/platform.yaml` has three `aws` blocks with three accounts (`…1`, `…2`, `…3`) in `eu-west-1`, so it still validates; `idp-claims-e2e` has none. *Cost if wrong:* lifting the check later accepts what it rejects today; nothing it rejects exists.
- **D-2. The floci job runs on every PR (controller ruling, spec §8's table)** as a new job of `ci.yaml`, so no workflow classification changes (`internalWorkflows` keeps four files). It has `timeout-minutes: 20`. Its cost, from ADR-0013's S3 numbers (about 26 s per stack and 6 s of readiness): four stacks, two per account, plus the image pull, a few minutes. *Cost if wrong:* a path filter later, if CI minutes matter.
- **D-3. The CI role may only push to its own repository (controller ruling, spec §4.4).** Its inline policy `ecr-push` has two statements: `ECRLogin` allows `ecr:GetAuthorizationToken` on `*`, because the action is account-level; `ECRPush` allows `ecr:BatchCheckLayerAvailability`, `ecr:CompleteLayerUpload`, `ecr:InitiateLayerUpload`, `ecr:PutImage` and `ecr:UploadLayerPart` on the repository's ARN (the Phase 0 spike's list). No pull or read action: `ecr:BatchGetImage`, which some clients call while pushing a multi-arch manifest, is left out, because the spec grants push only and nothing pushes in v1 (spec §5.7: a Component's CI cannot reach the ephemeral ECR). The trust policy is one statement: the principal `Federated` = `oidc_provider_arn`, the action `sts:AssumeRoleWithWebIdentity`, and `StringEquals` on `token.actions.githubusercontent.com:aud` = `sts.amazonaws.com` and `token.actions.githubusercontent.com:sub` = `repo:<org>/<name>:environment:<env>`. The module test, the offline Go test and the floci test each compare the whole document. *Cost if wrong:* one action added in the module, its test and `pushPolicy` (Phase 4's `container-ci` adds `ecr:BatchGetImage` if a real push needs it).
- **D-4. The ECR repository's settings.**
  - `image_tag_mutability = "IMMUTABLE"`: a pushed tag never moves, so a later CI run cannot replace an image that is deployed under it. Phase 4's `container-ci` must then push a unique tag per build (the commit SHA).
  - `scan_on_push = true`: AWS's basic scanning, at no cost; floci stores the flag.
  - `force_delete = true`: spec §6.3 says deleting a Component destroys its ECR, and every delete already waits for approval in `idp-approval`. Without it, an approved delete of a repository with images fails on real AWS (`RepositoryNotEmptyException`) at every reconcile until someone empties it. The images are build output of the Component's repository, which `archive_on_destroy` keeps; `archiveOnDestroy` itself is GitHub's archive, and AWS has no archive for a repository. In v1 floci never holds images.
  - Not set: `encryption_configuration` (AWS's default, AES256; the block forces a replacement when it changes) and a lifecycle policy (YAGNI; floci stores it without enforcing it).

  *Cost if wrong:* the mutability and the scanning are in-place updates; `force_delete` changes only what the provider does on delete.
- **D-5. Pins, checked live on 2026-11-17 against the 14-day rule (spec §7.5).**
  - `hashicorp/aws` **6.71.0**, released 2026-10-28T20:18Z (20 days). 6.72.0 was released 2026-11-04T21:20Z and is eligible from 2026-11-18; Renovate will propose it (watch list).
  - floci **2.4.0**: GitHub release 2026-10-31T22:10Z; the Docker Hub tag was pushed at 05:45Z the same day (16 days). It is pinned as `floci/floci:2.4.0@sha256:5ee85db604c4aa74ac76e07ea5cc83086f37bb472bf3dbb97e34019e48efae9c`, the multi-arch **index** digest. Read from `registry-1.docker.io` (manifest `2.4.0`, media type `application/vnd.oci.image.index.v1+json`): it lists linux/amd64 `sha256:7ab87e9a83909072aceb33d7026f8127a5f5dd693a974e0d30a0474eafcf91b9`, linux/arm64 `sha256:3df9677724e36251801f3828ea0bdcc2818afa96d8ec53f1741eaffdaeb990e1` and two attestation manifests. Renovate's docker datasource tracks index digests. The Phase 0 pin (`sha256:2e2343…`) was the linux/amd64 manifest of 2.1.0, not its index (`sha256:f5aa8c…`); ADR-0013 records that.
  - Phase 0 proved 6.66.0 with floci 2.1.0. floci 2.3 and 2.4 changed ECR (the advertised registry address, `PutImage` and the layer API), so the tests compare repository names and ARNs, never the registry URL (naming's own doc).

  *Cost if wrong:* a one-line pin change or a Renovate PR.
- **D-6. The lock file.** `lockfiles/aws.terraform.lock.hcl` locks `registry.opentofu.org/hashicorp/aws` 6.71.0. It was generated in a scratch copy with OpenTofu 1.12.6 and the runbook's four `-platform` flags; Task 3 writes its content verbatim. It carries 15 `h1:` and 15 `zh:` hashes. **Finding:** the same command with `-platform=linux_amd64` alone writes the identical 30 hashes, because the registry signs checksums for every package of the release and OpenTofu records them all. So a lock file cannot miss a platform because of the flags. `windows_amd64` was verified by a local `tofu init -lockfile=readonly`, and CI verifies `linux_amd64`. `TestAWSLockPinsTheProvider` mirrors the GitHub test and also requires a single provider. *Cost if wrong:* regenerate it (runbook).
- **D-7. No `render.AWSProviderVersion` in 2c.** No render uses it, so it would be a constant without a user. The pin lives in `modules/aws/*/versions.tf` and in the lock file. A new test, `TestModulesPinTheirLockedProvider`, keeps every module's pin equal to its provider's lock file. 2d adds the constant together with its Renovate regex manager and its golden files, when it renders the AWS stacks. *Cost if wrong:* 2d adds it, as planned.
- **D-8. The module inputs: identities in, subject built.** `modules/aws/component` takes `repository_name`, `role_name` and `oidc_provider_arn`, which 2d's render fills from `naming.RepositoryName`, `naming.RoleName` and `naming.OIDCProviderARN`. Computing them in HCL would duplicate `internal/naming` (ADR-0006). It builds only the trust policy's subject, from `github_repository` (`<org>/<name>`) and `environment`: 2a's D-4 left the subject and the audience to this module and its tests. Each input that reaches the trust policy is validated: `oidc_provider_arn` must be the GitHub Actions provider of an account in the `aws` partition; `github_repository` is `<org>/<name>` with no wildcard; `environment` matches spec §4.6. `modules/aws/baseline` has no inputs. *Cost if wrong:* a changed module interface before any render calls it, so no state moves.
- **D-9. No thumbprint.** `thumbprint_list` is optional and computed in provider 6.71.0 (read with `tofu providers schema -json`). IAM validates GitHub's issuer against its own trusted root CAs and ignores thumbprints for it. floci 2.4.0's `createOpenIDConnectProvider` accepts a request without one (`IamService.java` at tag `2.4.0`), and its IAM docs say thumbprints are "stored and echoed back but never validated". The Phase 0 spike created the provider without one, with floci 2.1.0. *Cost if wrong:* add GitHub's thumbprint as a literal; an in-place update.
- **D-10. floci runs as a service container, as spec §5.6 and ADR-0004 say, not with `docker run` as the spike did.**
  - floci 2.4.0's entrypoint reads the mounted socket's group and grants it to the unprivileged `floci` user (`docker/entrypoint.sh`), so the spike's `-u root` is no longer needed.
  - Its ECR starts a `registry:2` container through the socket on the first `CreateRepository` (`docs/services/ecr.md`), so the socket stays mounted.
  - The image has a `HEALTHCHECK` on `/_floci/init`, and the readiness step polls the same endpoint.
  - A service container sits on the job's own Docker network, which floci's ECR troubleshooting asks for: "Docker's default `bridge` does not resolve container names".
  - Renovate's `github-actions` manager reads `services.<id>.image`, so no regex manager is needed (D-14).

  The spike's readiness loop broke out after 90 s without failing, so it failed open. This job's loop fails after 60 s, and `TestCIFlociJob` pins that. *Cost if wrong:* a `docker run` step, plus a regex manager and its rows in `TestRenovateCustomManagers` and `TestRenovateCustomManagersFindThePins`.
- **D-11. The backing registry image is floci's default, `registry:2`, not pinned here.** floci pulls it by tag inside the ephemeral `floci` job. That job holds no credential: `contents: read` and `persist-credentials: false`. Pinning it means setting `FLOCI_SERVICES_ECR_REGISTRY_IMAGE` to a digest and adding a regex manager to track it, which is not worth it for an emulator sidecar. Recorded as an accepted gap. *Cost if wrong:* that environment variable and a regex manager.
- **D-12. Level 3 is a Go test that drives `tofu` (amends spec §8.4 by A12).** `tofu test` assertions cannot call `internal/naming`, so their expected names would be literals that duplicate it. `TestFloci` instead fills the inputs from `internal/naming`, the way the render will, and compares what floci created with naming's values. For each account it:
  - writes the stack, with `floci_override.tf.json` and the lock file;
  - runs `tofu init -lockfile=readonly`;
  - registers the stack's destroy before the apply, so a failed run still destroys what it created;
  - applies;
  - replays: a plan of the same version must exit 0.

  It then checks every identity and policy through the AWS CLI. After the destroys it checks that the accounts hold nothing (spec §6.3). Both accounts get the same Component, `api`, so a second create in one account would fail: that proves floci isolates the OIDC provider and the repository per account, and an explicit check covers the role. The test is gated by an environment variable, not a build tag, so `go vet` and the offline tests always compile it. CI cannot skip it: `TestCIFlociJob` pins `IDP_FLOCI_ENDPOINT` on the step, and a set endpoint never skips. Spec §5.6 has CI write `floci_override.tf.json`; here the test writes it, and in 2e the workflows will. The replay across module versions that §8.4 describes needs two versions, so it comes with the first change to a released module. *Cost if wrong:* rewriting the test as `tofu test` runs with `command = apply`, about one task.
- **D-13. Carried item (c), R3-partition-silent: reject regions outside the standard partition.** 2b's D-8 documented the limit only, because "in v1 every region is the floci emulator, where the partition does not matter". That premise is now false: floci 2.4.0 serves every partition and places each request in its signing region's partition (`docs/configuration/partitions.md`). With a `us-gov-west-1` environment, floci would create `arn:aws-us-gov:` resources while naming renders `arn:aws:` variables, a silent mismatch. `checkAWSAccounts` therefore rejects a region of another partition at its line, for example `environment "pro" has region "us-gov-west-1", in the aws-us-gov partition: only the standard aws partition is supported, because the AWS identities are computed with arn:aws: ARNs and the amazonaws.com host (spec §5.5)`. It uses a denylist of the region prefixes of botocore's `partitions.json`, as floci vendors it: `cn-`, `eusc-`, `eu-isoe-`, `us-gov-`, `us-iso-`, `us-isob-`, `us-isof-`. Like the AWS SDKs, it leaves any other region in `aws`, so a new standard region still validates (2b's objection to an allowlist).
  - Cost of the check: GovCloud and China configs fail validation; none exists.
  - Cost of keeping it documented only: that mismatch, found at the earliest by 2e's floci jobs, after a render.

  The `schemas_test.go` row "aws region with three parts" (`us-gov-west-1`, valid for the schema) stays: the schema checks the shape, the semantic check the partition. *Cost if wrong:* remove the check.
- **D-14. Renovate.**
  - A new rule after the GitHub provider's: `matchPackageNames: ["hashicorp/aws"]`, group "aws provider", label `needs-lockfile`. `TestRenovateRules` gains it at that position.
  - floci is tracked by the `github-actions` manager through the service image (docker datasource, tag and digest, 14-day wait), so no rule or custom manager is added. `TestRenovateCustomManagers`, `TestRenovateCustomManagersFindThePins` and `TestRenovateGlobalConfig` stay unchanged, which corrects the brief's expectation.
  - The runbook gets the AWS provider and floci rows, and an "AWS provider update" section.

  *Cost if wrong:* if Renovate does not pick up the service image (watch list), a regex manager and its test rows.
- **D-15. The carried items (a) and (b).**
  - (a) In `TestRenderIsDeterministic`, the inner loop variable `i` becomes `c`; behavior unchanged.
  - (b) R2-ci-fixture-list-duplicated: `render-smoke` renders and validates in one step and one loop. `TestCIRenderSmokeCoversEveryFixture` requires exactly one list, equal to the fixtures that have a golden render, so a new fixture cannot be left out.

  *Cost if wrong:* none.
- **D-16. CI's `modules` job takes each module's lock file from its provider directory:** `cp "$GITHUB_WORKSPACE/lockfiles/${MODULE%%/*}.terraform.lock.hcl"`. The matrix stays one list, so the check names stay `modules (<provider>/<name>)`. `TestCITestsEveryModule` requires the matrix to equal every `modules/*/*/versions.tf`. *Cost if wrong:* a matrix `include` with a `lock` key.
- **D-17. No render change, and only stale text is amended.** No render calls the new modules; 2d renders the AWS stacks with these inputs (`github_repository` = `<org>/<name>`). Amendment A12 covers §4.7 (the two new checks), §8.4 (D-12) and §11 risk 4 (now present tense). ADR-0013 gets two amended sentences: the Phase 0 floci digest kind, and the trust policy tests that now exist. No new ADR: ADR-0004 (a service container pinned by digest, the Docker socket, one floci for every account) and ADR-0006 (names are a contract that level 3 checks) decide 2c. *Cost if wrong:* one ADR later.
- **D-18. Local checks use OpenTofu 1.12.6; CI's 1.13.2 is the source of truth.** The binary is executed from `~/.idp/tools/tofu-1.12.6/`, never written to, and every module is copied to a `mktemp -d` directory. Verified while planning, in a scratch copy:
  - baseline 2/2 and component 8/8, each RED as written first;
  - the three mutation checks of Task 5;
  - `init -lockfile=readonly`, `validate` and `plan` of the stacks `writeStack` generates: 1 to add and 3 to add, offline, because the override skips every account lookup;
  - `go test ./...`, `go vet ./...` and `gofmt -l .` clean.

  *Cost if wrong:* one CI round trip.
- **D-19. Two facts the tests rely on, found while planning.**
  - Under `mock_provider`, `aws_iam_role.path` is null, while the real provider plans its default `/`. The names run therefore uses `coalesce(aws_iam_role.ci.path, "/") == "/"`, true with either value.
  - An HCL object key may be a template (`"${local.issuer}:aud" = …`), as `jsonencode` needs here.

  *Cost if wrong:* none; both are verified.
- **D-20. No release in 2c.** After the merge, release PR #28 still proposes `0.2.0` and stays open until 2f (2a's D-15). *Cost if wrong:* if it proposes another version, the rule is the same: do not merge it.

## Global Constraints

**Names** (copy verbatim)
- Branch `feat/phase-2c-aws-modules`; one PR, merged with a merge commit.
- `modules/aws/baseline`: `aws_iam_openid_connect_provider.this`; output `oidc_provider_arn`; runs `the_provider_is_github_actions_issuer`, `the_provider_accepts_only_the_sts_audience`.
- `modules/aws/component`: inputs `github_repository`, `environment`, `repository_name`, `role_name`, `oidc_provider_arn`; `local.issuer`; `aws_ecr_repository.this`, `aws_iam_role.ci`, `aws_iam_role_policy.push` (named `ecr-push`, statements `ECRLogin` and `ECRPush`); outputs `repository_arn`, `role_arn`; the eight runs of Task 5.
- `lockfiles.AWS`, `lockfiles/aws.terraform.lock.hcl`, `TestAWSLockPinsTheProvider`.
- `internal/claims`: `(*loader).checkAWSAccounts`, `partitionOf`, `nonStandardPartitions`, `TestPartitionOf`; the two diagnostics of D-1 and D-13, verbatim.
- `internal/floci`: `IDP_FLOCI_ENDPOINT`, `TestFloci`, `TestExpectedPoliciesAreTheModulesOwn`, `TestDecodePolicy`, `TestWriteStack`, `TestModulesUseNamingsIssuer`; helpers `decodePolicy`, `trustPolicy`, `pushPolicy`, `componentInputs`, `repositoryARN`, `writeStack`.
- `internal/workflowspec`: `moduleDirs`, `TestModulesPinTheirLockedProvider`, `TestCITestsEveryModule`, `TestCIRenderSmokeCoversEveryFixture`, `TestCIFlociJob`; the job field `Services` and the type `service`.
- `ci.yaml`: job `floci` with service `floci` and steps `floci-ready`, `floci-test`; the step ids `tofu-test` (job `modules`) and `render-validate` (job `render-smoke`).
- Spec amendment **A12**; phase log `docs/phases/phase-2c.md`.

**Process**
- Strict TDD: failing test, run it to see it fail, implement, run it to see it pass, commit. Tests that pin existing behavior say so and pass before the change; a mutation check shows what they catch.
- No `go build` and no GoReleaser run locally (no `go run` either). Use `go test ./...`, `go vet ./...` and `gofmt -l .` (must print nothing); `-race` runs in CI.
- Module changes run `tofu fmt -check` and `tofu test` locally with the module test snippet below, and always in CI (`modules (aws/baseline)`, `modules (aws/component)`). There is no local Docker: `TestFloci` against floci, actionlint and zizmor run only in CI.
- Shell rule: never use `cat`, `grep`, `find`, `sed` or `ls`, never write files with shell heredocs, never `perl`. Use the Read/Write/Edit tools, `rg`, `gh`, `git`, `python -I`.
- Conventional commits, never with Co-Authored-By or any AI attribution.
- Docs in English. Owner rule: docs describe the platform on its own terms, never naming another product as inspiration or comparison, and never use the owner's work account; the controller gives each dispatch the naming check that enforces both (it must print nothing).
- Doc-only changes after the merge go straight to `main`; code goes through PRs.
- Never write into `~/.idp/`. Running `~/.idp/tools/tofu-1.12.6/tofu.exe` only reads it (D-18).
- Scratch state lives in `mktemp -d` directories outside the worktree, removed after use.
- Every pin is at least 14 days old when adopted (spec §7.5, D-5).
- Risk lines:
  - Tasks 1, 7 and 9: `Risk: none`.
  - Task 2: `Risk: item 3 (claims validation rejects configs it accepted; none exists, D-1, D-13)`.
  - Tasks 3 and 6: `Risk: item 5 (dependencies)`.
  - Tasks 4 and 5: `Risk: item 2 (security: the IAM trust and permissions of the CI role)`.
  - Task 8: `Risk: item 5 (CI)`.

  The PR's CI run and the final whole-branch review check them.

**Module test snippet** (Git Bash, from the repository root; D-18). Set `MODULE` first, for example `MODULE=aws/component`:

```bash
TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"
"$TOFU" fmt -check -recursive "modules/$MODULE"
MOD="$(mktemp -d)"
cp -r "modules/$MODULE/." "$MOD"
cp "lockfiles/${MODULE%%/*}.terraform.lock.hcl" "$MOD/.terraform.lock.hcl"
(cd "$MOD" && "$TOFU" init -input=false -lockfile=readonly >/dev/null && "$TOFU" test -no-color)
rm -rf "$MOD"
```

`fmt -check` prints nothing when the files are formatted. `init` downloads the provider from `registry.opentofu.org` and checks it against the shipped lock file. The AWS provider is a large download (about 40 s cold).

**Spec rules**
- No real cloud credential anywhere (spec §7.6, 2a's D-2): floci gets the 12-digit account ID as its access key and a fake secret.
- The AWS identities come only from `internal/naming`, never from AWS state (spec §5.5, ADR-0006); the modules take them as inputs.
- Emulator specifics belong to CI and its tests, never to the modules or the render (spec §5.6, ADR-0004).
- Rendering stays pure and, in 2c, unchanged.

**Owner actions:** none beyond approving this plan and its side effects.

**Side effects this plan authorizes** (controller, after the owner approves the plan)
1. Push `feat/phase-2c-aws-modules`, open its PR, merge it after CI and the final review.
2. Doc-only commits straight to `main`: this plan, the phase log, the README.
3. Delete the local, git-ignored SDD workspace `.superpowers/sdd/` after the phase log is committed.

Not authorized, stop and ask:
- merging any release PR, including #28, which proposes `0.2.0` (D-20);
- merging or dispatching Renovate, or merging a Renovate PR;
- changing any repository or org setting, App, ruleset, environment, variable or secret;
- any change to `idp-claims` or `idp-claims-e2e`;
- creating any cloud resource.

## Review Focus

1. **Two environments that share an AWS account.**
   - **Expected:** `claims.Load` reports the later one in file order, at its `accountId` line, with D-1's message; distinct accounts pass.
   - **Pinned by:** the `TestLoadDiagnostics` cases "two environments may not share an aws account" and "the shared account is reported where the file uses it again" (Task 2); `TestLoadPlatformAWS` (distinct accounts). Mutation-checked in Task 2.
2. **The trust policy's `sub` or `aud` drifting from spec §4.4.**
   - **Expected:** exactly one statement: the account's GitHub OIDC provider, `sts:AssumeRoleWithWebIdentity`, and `StringEquals` on `aud` = `sts.amazonaws.com` and `sub` = `repo:<org>/<name>:environment:<env>`.
   - **Pinned by:** the module runs `the_trust_policy_admits_only_the_components_ci_in_its_environment` (`sub`, `aud` and the whole document) and `the_subject_follows_the_environment` (Task 5); `TestExpectedPoliciesAreTheModulesOwn` (Task 7); the trust policy check of `TestFloci` on the created role (CI). Mutation-checked in Task 5 (`StringLike`).
3. **A role policy that allows more than its own repository.**
   - **Expected:** the inline `ecr-push` only: login on `*`, the five push actions on the repository's ARN, no other statement, action, resource or element, and no managed policy.
   - **Pinned by:** the module run `the_ci_role_may_only_push_to_its_repository`, which compares the whole document (Task 5); `TestDecodePolicy`, which rejects `NotResource` and unknown elements (Task 7); `TestFloci`'s push policy, `list-role-policies` and `list-attached-role-policies` checks (CI). Mutation-checked in Task 5 (`Resource = "*"`).
4. **A module name drifting from `internal/naming`.**
   - **Expected:** the OIDC provider ARN, the role ARN, and the repository name and ARN that floci reports equal `naming.OIDCProviderARN`, `naming.RoleARN` and `naming.RepositoryName` for the inputs; the modules name naming's issuer.
   - **Pinned by:** `TestFloci` (CI); the module run `names_are_the_ones_naming_computed` (Task 5); `TestModulesUseNamingsIssuer` (Task 7). Mutation-checked in Tasks 5 (`path = "/ci/"`) and 7 (a trailing slash on the provider URL).
5. **The floci job failing open, or leaving resources behind.**
   - **Expected:**
     - the readiness wait fails the job when floci never answers;
     - no step that checks can be skipped;
     - `TestFloci` never skips when the endpoint is set;
     - every applied stack is destroyed, even after a failure, and nothing is left afterwards.
   - **Pinned by:** `TestCIFlociJob` (Task 8): `exit 1` after 60 s, no `if` or `continue-on-error`, `IDP_FLOCI_ENDPOINT` on the step, and a job timeout. Task 7 Step 9 shows `TestFloci` failing, not skipping, with the endpoint set and no emulator. In `TestFloci` (CI), each destroy is registered before its apply, and `wantNothingLeft` runs last.

Also tested, outside the five:
- `TestModulesPinTheirLockedProvider`: Renovate bumping a provider without its lock file fails the `go` job (Task 6, mutation-checked);
- the Renovate rule (`TestRenovateRules`);
- every module tested in CI with its lock file (`TestCITestsEveryModule`);
- `render-smoke` covering every fixture from one list (`TestCIRenderSmokeCoversEveryFixture`);
- the partition rule (`TestLoadDiagnostics`, `TestPartitionOf`);
- the lock file (`TestAWSLockPinsTheProvider`; D-6's finding that the platform flags do not change it);
- the replay plan with no changes and account isolation (`TestFloci`, CI).

## Watch list for the live steps

Record each result in the ledger:
- **The first `floci` job** (Task 10 Step 3). It passes. Record:
  - its duration and the duration of `floci-ready`;
  - the `floci's repository URI for api: …` lines of `TestFloci`'s log: the URL form floci 2.4.0 returns, which ADR-0013 recorded as `<account>.dkr.ecr.<region>.localhost:4566/<name>` for 2.1.0. Nothing compares it;
  - whether the runner waited for the image's health check before the steps (the "Initialize containers" log).
- **The check list of the PR**: `go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke`, `floci` and `release-snapshot`.
- **After the merge** (Task 10 Step 6), release PR #28 still proposes `0.2.0` (D-20), and the harness run dispatched for it passes. The render is unchanged, and the harness's `config/platform.yaml` has no `aws` block.
- **The next weekly Renovate run after the merge** (Mondays, 04:17 UTC; not dispatched by this plan). Read the Dependency Dashboard of `idp-engine`:
  - it lists `floci/floci` under `github-actions` (D-10, D-14) and `hashicorp/aws` under `terraform`;
  - `hashicorp/aws` 6.72.0, eligible from 2026-11-18, arrives as an "aws provider" PR labelled `needs-lockfile` whose checks fail until the lock is regenerated (the runbook's new section);
  - floci 2.5.0 (2026-11-15) is eligible from 2026-11-29.

  Merge nothing (not authorized). Record what happened before the phase log is written, and carry the rest into 2d.

## File Structure

```
docs/superpowers/specs/2026-10-08-idp-on-actions-design.md (A12), docs/adr/0013-phase-0-spike-findings.md          # Task 1
internal/claims/load_test.go, internal/claims/load.go, internal/naming/naming.go, docs/claims.md                  # Task 2
lockfiles/aws.terraform.lock.hcl (new), lockfiles/lockfiles.go, lockfiles/lockfiles_test.go                       # Task 3
modules/aws/baseline/{versions.tf,main.tf,outputs.tf,tests/baseline.tftest.hcl} (new)                             # Task 4
modules/aws/component/{versions.tf,variables.tf,main.tf,outputs.tf,tests/component.tftest.hcl} (new)              # Task 5
.github/renovate-global.json, internal/workflowspec/renovate_test.go, docs/runbooks/dependencies.md               # Task 6
internal/floci/{doc.go,policy_test.go,iam_test.go,stack_test.go,fixture_test.go,floci_test.go} (new)              # Task 7
.github/workflows/ci.yaml, internal/workflowspec/load_test.go, internal/workflowspec/ci_test.go (new),
docs/runbooks/dependencies.md                                                                                     # Task 8
internal/render/render_test.go                                                                                    # Task 9
docs/phases/phase-2c.md (new), README.md                                                                          # Task 10
```

Tasks 1 to 9 touch 31 files.

## Pre-flight (controller, before Task 1)

- [ ] `gh api user --jq .login` prints `jellalshadows` (else `gh auth switch -u jellalshadows`).
- [ ] `git switch main && git pull --ff-only`, then `git log -1 --format=%h` prints `d112509` or a later commit, and `git status --short` prints nothing.
- [ ] `go version` prints `go1.26` or a later 1.26 patch, and `go test ./...` passes on `main`, so any later failure is this plan's.
- [ ] `"$HOME/.idp/tools/tofu-1.12.6/tofu.exe" version` prints `OpenTofu v1.12.6`, and the module test snippet with `MODULE=github/component` ends with `Success! 12 passed, 0 failed.` (`github/group`: `Success! 5 passed, 0 failed.`).
- [ ] `gh pr view 28 --repo jellalshadows-idp/idp-engine --json state,title --jq '"\(.state) \(.title)"'` prints `OPEN chore(main): release 0.2.0`.
- [ ] The owner approves this plan, its side effects and the rulings D-1 to D-20; ledger them. Commit this plan to `main` as `docs: add the phase 2c plan` and push. The push refreshes release PR #28 and re-runs the harness on it; leave the PR open (D-20).

---

### Task 1: Decisions first — spec amendment A12 and ADR-0013

**Files:**
- Modify: `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, `docs/adr/0013-phase-0-spike-findings.md`

**Interfaces:**
- Produces: spec §4.7 listing the two new checks (D-1, D-13); §8.4 describing the Go test that drives `tofu` (D-12); §11 risk 4 in the present tense; ADR-0013's digest note (D-5) and its trust-policy sentence in the present tense.

- [ ] **Step 1: Create the branch**

Run: `git switch main && git pull --ff-only && git switch -c feat/phase-2c-aws-modules`

- [ ] **Step 2: Apply amendment A12 to the spec** (exact replacements with Edit)

1. §4.7, old:

```markdown
   - A Component with `aws.registry: true` lists at least one env, and each of its
     envs has an `aws` block (`accountId` and `region`) in the platform config,
     because its registry, CI role and env variables are computed per env (§5.5).
     *(Amendment A10, 2026-11-03.)*
```

new:

```markdown
   - A Component with `aws.registry: true` lists at least one env, and each of its
     envs has an `aws` block (`accountId` and `region`) in the platform config,
     because its registry, CI role and env variables are computed per env (§5.5).
     *(Amendment A10, 2026-11-03.)*
   - No two envs share an `accountId`, because each env's `_baseline` creates its
     account's GitHub OIDC provider (§5.4), and every `aws.region` is in the
     standard `aws` partition, whose ARNs and hosts `internal/naming` computes
     (§5.5). *(Amendment A12, 2026-11-17.)*
```

2. §8.4, old:

```markdown
`tofu test` with `command = apply` against a floci service container: apply, assert,
destroy. It also includes a **replay test**: apply the old version, plan the new one,
and check that the diff is exactly the expected one.
```

new:

```markdown
A Go test (`internal/floci`, run by CI's `floci` job) applies `modules/aws/baseline`
and then `modules/aws/component` to two accounts of one floci service container, with
the inputs `internal/naming` computes, as rendered stacks will pass them. It checks
through the AWS API that the created identities are naming's (ADR-0006) and that the
trust and push policies are exactly §4.4's, then destroys everything and checks that
nothing is left. Its **replay test** plans the applied version again and requires no
change. The replay across versions (apply the old version, plan the new one, and check
that the diff is exactly the expected one) comes with the first change to a released
module. *(Amendment A12, 2026-11-17.)*
```

3. §11, risk 4, old:

```markdown
the AWS module tests will assert the trust policy's `sub` and `aud` (Phase 2c, §8.3) (owner decision; *Amendment A10, 2026-11-03; reworded by Amendment A11, 2026-11-03*) |
```

new:

```markdown
the AWS module tests assert the trust policy's `sub` and `aud` (§8.3), and the floci test asserts them on the created role (§8.4) (owner decision; *Amendment A10, 2026-11-03; reworded by Amendment A11, 2026-11-03, and Amendment A12, 2026-11-17*) |
```

- [ ] **Step 3: Amend ADR-0013** (exact replacements with Edit)

1. "Versions", old:

```markdown
OpenTofu 1.12.6 · integrations/github 6.13.0 · hashicorp/aws 6.66.0 · floci 2.1.0 (digest in the Phase 0 plan) · actions as pinned in `.github/workflows/ci.yaml`. The spike runner was `ubuntu-24.04` with OpenTofu 1.12.6.
```

new:

```markdown
OpenTofu 1.12.6 · integrations/github 6.13.0 · hashicorp/aws 6.66.0 · floci 2.1.0 (digest in the Phase 0 plan) · actions as pinned in `.github/workflows/ci.yaml`. The spike runner was `ubuntu-24.04` with OpenTofu 1.12.6. *(Amended in Phase 2c, 2026-11-17: that floci digest is the linux/amd64 image's, not the multi-arch index's. CI now runs floci 2.4.0, pinned by its index digest, which Renovate tracks, and hashicorp/aws 6.71.0.)*
```

2. "S3 — floci", the notes, old:

```markdown
The trust policy's `sub` and `aud` will be asserted by the AWS module tests (Phase 2c, spec §8.3). *(Amended in Phase 2b, 2026-11-03; this said that the module tests assert them.)*
```

new:

```markdown
The trust policy's `sub` and `aud` are asserted by the AWS module tests (spec §8.3) and, on the created role, by the floci test (spec §8.4). *(Amended in Phase 2b, 2026-11-03, before those tests existed, and in Phase 2c, 2026-11-17, when they arrived.)*
```

- [ ] **Step 4: Check and commit**

Run:

```bash
python -I -c "
import re,pathlib
spec=pathlib.Path('docs/superpowers/specs/2026-10-08-idp-on-actions-design.md').read_text(encoding='utf-8')
def slug(h):
    h=h.strip().lower(); h=re.sub(r'[^\w\- ]','',h); return h.replace(' ','-')
anchors={slug(m.group(1)) for m in re.finditer(r'^#+ (.+)$', spec, re.M)}
bad=[(str(p),a) for p in pathlib.Path('docs').rglob('*.md') if 'superpowers' not in p.parts for a in re.findall(r'2026-10-08-idp-on-actions-design\.md#([\w\-]+)', p.read_text(encoding='utf-8')) if a not in anchors]
print(bad or 'all spec anchors resolve')
"
```

Expected: `all spec anchors resolve`. Then:
- `rg -c "Amendment A12" docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` prints `3`;
- `rg -c "Amendment A11" docs/superpowers/specs/2026-10-08-idp-on-actions-design.md` prints `4` (unchanged);
- `rg -c "Phase 2c, 2026-11-17" docs/adr/0013-phase-0-spike-findings.md` prints `2`;
- `rg -n 'will assert the trust|will be asserted by the AWS module|command = apply. against a floci' docs/adr docs/superpowers/specs` prints nothing;
- the naming check prints nothing.

```bash
git add docs/superpowers/specs/2026-10-08-idp-on-actions-design.md docs/adr/0013-phase-0-spike-findings.md
git commit -m "docs: record the aws account checks and the floci test (spec amendment a12)"
```

---

### Task 2: Claims — one AWS account per environment, standard partition only

**Files:**
- Modify: `internal/claims/load_test.go`, `internal/claims/load.go`, `internal/naming/naming.go`, `docs/claims.md`

**Interfaces:**
- Consumes: `(*loader).loadPlatform`, `(*document).line`, `awsPlatform`, `TestLoadDiagnostics` (`internal/claims`).
- Produces: `(*loader).checkAWSAccounts(doc *document)`; `partitionOf(region string) string`; `nonStandardPartitions`; the diagnostics of D-1 and D-13, verbatim.

- [ ] **Step 1: Write the failing account cases** — `internal/claims/load_test.go`, in `TestLoadDiagnostics` (Edit), old:

```go
		{"gitkeep ignored", func(r map[string]string) { r["claims/groups/.gitkeep"] = "" }, nil},
	}
```

new:

```go
		{"gitkeep ignored", func(r map[string]string) { r["claims/groups/.gitkeep"] = "" }, nil},
		{"two environments may not share an aws account", func(r map[string]string) {
			r["config/platform.yaml"] = strings.Replace(awsPlatform, `"000000000003"`, `"000000000001"`, 1)
		}, []string{`config/platform.yaml:14: environment "pro" uses accountId "000000000001", like environment "dev": each environment needs its own AWS account, because each environment's baseline creates that account's GitHub OIDC provider (spec §5.4)`}},
		{"the shared account is reported where the file uses it again", func(r map[string]string) {
			r["config/platform.yaml"] = "apiVersion: idp/v1\nkind: Platform\ngithub:\n  org: acme\n  writerAppId: 5255579\nenvironments:\n  pro:\n    protected: true\n    aws:\n      accountId: \"000000000001\"\n      region: eu-west-1\n  dev:\n    aws:\n      accountId: \"000000000001\"\n      region: eu-west-1\n"
		}, []string{`config/platform.yaml:14: environment "dev" uses accountId "000000000001", like environment "pro": each environment needs its own AWS account, because each environment's baseline creates that account's GitHub OIDC provider (spec §5.4)`}},
	}
```

The second case declares `pro` first, so "later" means the file's order, not the alphabet.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/claims/ -count=1 -run TestLoadDiagnostics`
Expected: `FAIL` on exactly the two new subtests, each with `diagnostics =` empty and `want` its line.

- [ ] **Step 3: Implement** — `internal/claims/load.go` (Edit), old:

```go
	l.platformOK = true
	n := l.model.Platform.Naming
	for _, p := range n.ReservedPrefixes {
		if n.RequiredPrefix != "" && p == n.RequiredPrefix {
			l.add(Diagnostic{File: doc.file, Line: doc.lines["/naming/requiredPrefix"],
				Message: fmt.Sprintf("naming.requiredPrefix %q is also listed in naming.reservedPrefixes", p)})
		}
	}
}
```

new:

```go
	l.platformOK = true
	n := l.model.Platform.Naming
	for _, p := range n.ReservedPrefixes {
		if n.RequiredPrefix != "" && p == n.RequiredPrefix {
			l.add(Diagnostic{File: doc.file, Line: doc.lines["/naming/requiredPrefix"],
				Message: fmt.Sprintf("naming.requiredPrefix %q is also listed in naming.reservedPrefixes", p)})
		}
	}
	l.checkAWSAccounts(doc)
}

// checkAWSAccounts checks, in file order, what the AWS stacks need from the
// environments' aws blocks: an account of the environment's own. Each
// environment's baseline creates its account's GitHub OIDC provider (spec
// §5.4), and an account holds one, so a second environment in the same
// account would create it again.
func (l *loader) checkAWSAccounts(doc *document) {
	var envs []string
	for name, e := range l.model.Platform.Environments {
		if e.AWS != nil {
			envs = append(envs, name)
		}
	}
	at := func(env string, path ...string) int {
		return doc.line(append([]string{"environments", env}, path...))
	}
	sort.Slice(envs, func(i, j int) bool { return at(envs[i]) < at(envs[j]) })
	first := map[string]string{} // account ID → the first environment that uses it
	for _, env := range envs {
		aws := l.model.Platform.Environments[env].AWS
		if other, taken := first[aws.AccountID]; taken {
			l.add(Diagnostic{File: doc.file, Line: at(env, "aws", "accountId"),
				Message: fmt.Sprintf("environment %q uses accountId %q, like environment %q: each environment needs its own AWS account, because each environment's baseline creates that account's GitHub OIDC provider (spec §5.4)", env, aws.AccountID, other)})
			continue
		}
		first[aws.AccountID] = env
	}
}
```

The platform is a map, so the environments are ordered by their line; `doc.line` falls back to the nearest indexed ancestor, as the schema diagnostics do.

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./internal/claims/ -count=1 -v -run 'TestLoadDiagnostics|TestLoadPlatformAWS'`
Expected: `PASS`, with 23 `TestLoadDiagnostics` subtests and `TestLoadPlatformAWS` (distinct accounts, no diagnostic).

- [ ] **Step 5: Verify and commit** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

```bash
git add internal/claims/load.go internal/claims/load_test.go
git commit -m "feat(claims): give each environment its own aws account"
```

- [ ] **Step 6: Mutation check** (nothing is committed) — in `internal/claims/load.go`, delete the line `		first[aws.AccountID] = env`. Run `go test ./internal/claims/ -count=1 -run TestLoadDiagnostics`. Expected: `FAIL` on the two account cases. Restore with `git checkout internal/claims/load.go`; `git status --short` prints nothing.

- [ ] **Step 7: Write the failing partition cases** — `internal/claims/load_test.go` (Edit). Old:

```go
		}, []string{`config/platform.yaml:14: environment "dev" uses accountId "000000000001", like environment "pro": each environment needs its own AWS account, because each environment's baseline creates that account's GitHub OIDC provider (spec §5.4)`}},
	}
```

new:

```go
		}, []string{`config/platform.yaml:14: environment "dev" uses accountId "000000000001", like environment "pro": each environment needs its own AWS account, because each environment's baseline creates that account's GitHub OIDC provider (spec §5.4)`}},
		{"regions outside the standard aws partition", func(r map[string]string) {
			p := strings.Replace(awsPlatform, "region: eu-west-1", "region: cn-north-1", 1)
			r["config/platform.yaml"] = strings.Replace(p, "\"000000000003\"\n      region: eu-west-1", "\"000000000003\"\n      region: us-gov-west-1", 1)
		}, []string{
			`config/platform.yaml:10: environment "dev" has region "cn-north-1", in the aws-cn partition: only the standard aws partition is supported, because the AWS identities are computed with arn:aws: ARNs and the amazonaws.com host (spec §5.5)`,
			`config/platform.yaml:15: environment "pro" has region "us-gov-west-1", in the aws-us-gov partition: only the standard aws partition is supported, because the AWS identities are computed with arn:aws: ARNs and the amazonaws.com host (spec §5.5)`,
		}},
	}
```

and old:

```go
func TestLoadInvalidOwnerGroupDoesNotCascade(t *testing.T) {
```

new:

```go
// TestPartitionOf follows the region rules of botocore's partitions.json, which
// the AWS SDKs and floci use to place a region in its partition.
func TestPartitionOf(t *testing.T) {
	for region, want := range map[string]string{
		"eu-west-1":       "",
		"us-east-1":       "",
		"ap-southeast-2":  "",
		"il-central-1":    "",
		"mx-central-1":    "",
		"cn-north-1":      "aws-cn",
		"cn-northwest-1":  "aws-cn",
		"us-gov-west-1":   "aws-us-gov",
		"us-gov-east-1":   "aws-us-gov",
		"us-iso-east-1":   "aws-iso",
		"us-isob-east-1":  "aws-iso-b",
		"eu-isoe-west-1":  "aws-iso-e",
		"us-isof-south-1": "aws-iso-f",
		"eusc-de-east-1":  "aws-eusc",
	} {
		if got := partitionOf(region); got != want {
			t.Errorf("partitionOf(%q) = %q, want %q", region, got, want)
		}
	}
}

func TestLoadInvalidOwnerGroupDoesNotCascade(t *testing.T) {
```

- [ ] **Step 8: Run them to see them fail**

Run: `go test ./internal/claims/ -count=1`
Expected: `FAIL` with `[build failed]` and `undefined: partitionOf`.

- [ ] **Step 9: Implement** — `internal/claims/load.go` (Edit).

1. Old:

```go
// environments' aws blocks: an account of the environment's own. Each
// environment's baseline creates its account's GitHub OIDC provider (spec
// §5.4), and an account holds one, so a second environment in the same
// account would create it again.
```

new:

```go
// environments' aws blocks: a region of the standard aws partition, and an
// account of the environment's own. Each environment's baseline creates its
// account's GitHub OIDC provider (spec §5.4), and an account holds one, so a
// second environment in the same account would create it again.
```

2. Old:

```go
		aws := l.model.Platform.Environments[env].AWS
		if other, taken := first[aws.AccountID]; taken {
```

new:

```go
		aws := l.model.Platform.Environments[env].AWS
		if p := partitionOf(aws.Region); p != "" {
			l.add(Diagnostic{File: doc.file, Line: at(env, "aws", "region"),
				Message: fmt.Sprintf("environment %q has region %q, in the %s partition: only the standard aws partition is supported, because the AWS identities are computed with arn:aws: ARNs and the amazonaws.com host (spec §5.5)", env, aws.Region, p)})
		}
		if other, taken := first[aws.AccountID]; taken {
```

3. Old:

```go
		first[aws.AccountID] = env
	}
}
```

new:

```go
		first[aws.AccountID] = env
	}
}

// nonStandardPartitions are the AWS partitions other than aws, with the region
// prefix that places a region in each (botocore's partitions.json, which the
// AWS SDKs and floci use). floci answers a request in its region's partition,
// so such a region would create ARNs that differ from the ones internal/naming
// computes.
var nonStandardPartitions = []struct{ prefix, partition string }{
	{"cn-", "aws-cn"},
	{"eusc-", "aws-eusc"},
	{"eu-isoe-", "aws-iso-e"},
	{"us-gov-", "aws-us-gov"},
	{"us-iso-", "aws-iso"},
	{"us-isob-", "aws-iso-b"},
	{"us-isof-", "aws-iso-f"},
}

// partitionOf returns the partition of a region outside the standard aws
// partition, or "" for a region in it.
func partitionOf(region string) string {
	for _, p := range nonStandardPartitions {
		if strings.HasPrefix(region, p.prefix) {
			return p.partition
		}
	}
	return ""
}
```

`us-isob-east-1` does not start with `us-iso-`, so the two ISO prefixes never overlap.

- [ ] **Step 10: Run them to see them pass**

Run: `go test ./internal/claims/ -count=1 -v -run 'TestLoadDiagnostics|TestPartitionOf'`
Expected: `PASS`, with 24 `TestLoadDiagnostics` subtests and `TestPartitionOf`.

- [ ] **Step 11: Update naming's doc and the claims reference** (exact replacements with Edit)

1. `internal/naming/naming.go`, old:

```go
// The identities are those of the standard aws partition: ARNs start with
// arn:aws: and the registry host ends in amazonaws.com. The platform schema
// checks only a region's shape, so it also accepts GovCloud (us-gov-*) and China
// (cn-*) regions, whose identities differ; they are not supported. In v1 every
// region is the floci emulator (spec §5.6), where the partition does not matter.
// Going live in another partition means deriving the partition and the host
// suffix from the region here.
```

new:

```go
// The identities are those of the standard aws partition: ARNs start with
// arn:aws: and the registry host ends in amazonaws.com. Claims validation
// rejects a region of any other partition, such as GovCloud (us-gov-*) or China
// (cn-*), whose identities differ: floci, which places each request in its
// region's partition, would create them there (spec §4.7). Supporting another
// partition means deriving the partition and the host suffix from the region
// here, and lifting that check.
```

2. `docs/claims.md`, the Platform table, old:

```markdown
| `environments.<env>.aws.accountId` | yes, in `aws` | | 12 digits, quoted (`"000000000001"`), so YAML keeps it a string with its leading zeros |
| `environments.<env>.aws.region` | yes, in `aws` | | An AWS region of the standard `aws` partition, such as `eu-west-1`. The schema checks only the shape, so GovCloud (`us-gov-*`) and China (`cn-*`) regions pass it, but they are not supported: the identities are computed for the standard partition (`arn:aws:` ARNs and the `amazonaws.com` registry host) |
```

new:

```markdown
| `environments.<env>.aws.accountId` | yes, in `aws` | | 12 digits, quoted (`"000000000001"`), so YAML keeps it a string with its leading zeros. Each environment needs its own account: its baseline creates the account's GitHub OIDC provider, and an account holds one |
| `environments.<env>.aws.region` | yes, in `aws` | | An AWS region of the standard `aws` partition, such as `eu-west-1`. A region of another partition, such as GovCloud (`us-gov-*`) or China (`cn-*`), fails validation: the identities are computed for the standard partition (`arn:aws:` ARNs and the `amazonaws.com` registry host), and floci would create the resources in the other one |
```

3. `docs/claims.md`, "Validation", old:

````markdown
Beyond the schemas, the checks include: each name matches its file name; the owner Group exists; every environment exists in `config/platform.yaml`; a Component with `aws.registry: true` lists at least one environment, and each of them has an `aws` block. For the Component above, in a platform whose `pro` has no `aws` block:

```text
claims/components/api.yaml:6: environment "pro" needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true
```
````

new:

````markdown
Beyond the schemas, the checks include: each name matches its file name; the owner Group exists; every environment exists in `config/platform.yaml`; a Component with `aws.registry: true` lists at least one environment, and each of them has an `aws` block; no two environments share an `accountId`; and every region is in the standard `aws` partition. For the Component above, in a platform whose `pro` has no `aws` block:

```text
claims/components/api.yaml:6: environment "pro" needs an aws block (accountId and region) in config/platform.yaml, because aws.registry is true
```

For the platform above with `pro` set to `aws: { accountId: "000000000001", region: us-gov-west-1 }`, both problems are reported at the later environment:

```text
config/platform.yaml:11: environment "pro" has region "us-gov-west-1", in the aws-us-gov partition: only the standard aws partition is supported, because the AWS identities are computed with arn:aws: ARNs and the amazonaws.com host (spec §5.5)
config/platform.yaml:11: environment "pro" uses accountId "000000000001", like environment "dev": each environment needs its own AWS account, because each environment's baseline creates that account's GitHub OIDC provider (spec §5.4)
```
````

Both lines were produced by `claims.Load` on that file while planning.

- [ ] **Step 12: Verify and commit** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing); `rg -n "where the partition does not matter|pass it, but they are not supported" internal docs/claims.md` prints nothing; the naming check prints nothing.

```bash
git add internal/claims/load.go internal/claims/load_test.go internal/naming/naming.go docs/claims.md
git commit -m "feat(claims): accept only regions of the standard aws partition"
```

---

### Task 3: The AWS lock file

**Files:**
- Create: `lockfiles/aws.terraform.lock.hcl`
- Modify: `lockfiles/lockfiles_test.go`, `lockfiles/lockfiles.go`

**Interfaces:**
- Produces: `lockfiles.AWS []byte` (Task 7 copies it into each floci stack; 2d's render will); the file CI's `modules` job copies for `modules/aws/*` (Task 8).

- [ ] **Step 1: Write the failing test** — `lockfiles/lockfiles_test.go` (Edit), append after `TestGitHubLockPinsTheProvider`:

```go

func TestAWSLockPinsTheProvider(t *testing.T) {
	s := string(AWS)
	if !regexp.MustCompile(`provider "registry\.opentofu\.org/hashicorp/aws"`).MatchString(s) {
		t.Fatal("lock file does not lock hashicorp/aws")
	}
	if !regexp.MustCompile(`version\s*=\s*"6\.71\.0"`).MatchString(s) {
		t.Error("lock file does not pin version 6.71.0")
	}
	if n := len(regexp.MustCompile(`"h1:`).FindAllString(s, -1)); n < 4 {
		t.Errorf("lock file has %d h1 hashes; want one per platform (linux/windows/darwin amd64 + darwin arm64)", n)
	}
	if n := len(regexp.MustCompile(`(?m)^provider `).FindAllString(s, -1)); n != 1 {
		t.Errorf("lock file locks %d providers, want only hashicorp/aws: the AWS stacks use no other", n)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./lockfiles/ -count=1`
Expected: `FAIL` with `[build failed]` and `undefined: AWS`.

- [ ] **Step 3: Write the lock file** — create `lockfiles/aws.terraform.lock.hcl` (Write tool), exactly:

```hcl
# This file is maintained automatically by "tofu init".
# Manual edits may be lost in future updates.

provider "registry.opentofu.org/hashicorp/aws" {
  version     = "6.71.0"
  constraints = "6.71.0"
  hashes = [
    "h1:0NOpfpZ0BJ+znFVEedMd2yWx4Q8SXj8H8tHEkiLrM/o=",
    "h1:1eUAMsp6IPdvQHb6Cc4U0SxxECkwvP5RYGFxXJaIGiY=",
    "h1:2Tlplnn5v5JYAIDRYtveZO7cicSJ1CleYET8ycUS1CU=",
    "h1:5SbT5whSV+Csh2KPkzYZ2VEhv0aSboFITqUDvMm/qGQ=",
    "h1:AtkTrCVMkoZzrBIm05jghLXiHPfJAnU/GTgVRdIX5bc=",
    "h1:BsoTfyxZ2ct0Q0mfdAFmWtpmeCYOIeQjsyoR804Cse4=",
    "h1:ORnxYK/YWxzFA/L49QhrRVOMWgGOvlYHlTgMNXgn2K8=",
    "h1:Q5i7ee1oBfrYHgnQsnC+DKk/1ynUFvfcIxxpqoY0btQ=",
    "h1:UsX7nnfKyOHOzKeRxkaHO0oNHsgW7RiGFe0x+JlU0gc=",
    "h1:Vcqk7kEWL0wVBGSgA/3yIeKdL7jihYEBZ5q6DJ1IW1k=",
    "h1:dwryNAhannvEPEumOIwKEPhJtNAUL1uHs1Je8JopMF4=",
    "h1:hSDn8I/j4nELdfWdP/zDIRieY/GHv+trGadEsploGeM=",
    "h1:nP2rt9Y8dNPjl/68pJ9SLsUupxlrFIt/vkylVVo3uck=",
    "h1:xhdKlv4GCsI+T6JgHiGRZnqQnxxLPRDD2L9BVSZd8ag=",
    "h1:zcXZtjJSqox2qrDFq/K8IKGpICzL6wa9Q4U+8EZU6qM=",
    "zh:26fcf302abd7112c23f44bd5c1afbdb56224e9dfbe5d3016866f3f1dba10f11a",
    "zh:399b50405fa1871f82db89e439dc53a07b6cfbd53753b86cfab0a10c42c93b8a",
    "zh:40945415f6de4e67beefb2a26c44b2d9d46981a7dae1e78ce41d4afba625b10b",
    "zh:470f3f3d83546bcfe820060969c2a7b491e84ef94b2cf8e1cc158bff30a63951",
    "zh:7804513cb9058917a3da4637a44811ddaae0c8700c1b124002531a50ab4ba8c6",
    "zh:7eb1ba4aef9834ee6d29e35737b7b53ed09c8565240702886f146df822944812",
    "zh:818953323015b5f12afd84d392ebde337c125b492d9f22b8636b3129962fb461",
    "zh:851e2beb8f9cb6f964f8512e9702ecd9b953c3c7f7548c136d91be1ef6e0a024",
    "zh:865a4b6b0aca2b3bafda820ad9fb685d1547e88b43960db90402be0db04de0a6",
    "zh:90e830f1b0025d0d442bbdc6adb1418b610b5fcd0e30733689057b769dd6474e",
    "zh:a0251e5eed0c5c74ae714ea580ad52c22166da8e9dfc4d1cdcf1ffebb9528504",
    "zh:a954bc049bd521d9dfe9996424eb16260bcf89220f6655d0e67242f99b485c08",
    "zh:c34b57e8754ec2c726c78a4c4128f468d9bee78020d8ce1ab29ff99db423b447",
    "zh:efd7385ac4b6a493ca47c4bf287d45a25a18d7a105b081d757ab6d39819f718a",
    "zh:fa881c390793ba47d0758fb5d4834dd8a0bb28bbc6eedea2ddc164f99e83186d",
  ]
}
```

Then prove it is what OpenTofu writes (D-6). In Git Bash, from the repository root:

```bash
TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"
LOCK="$(mktemp -d)"
printf 'terraform {\n  required_providers {\n    aws = {\n      source  = "hashicorp/aws"\n      version = "6.71.0"\n    }\n  }\n}\n' > "$LOCK/versions.tf"
(cd "$LOCK" && "$TOFU" providers lock -platform=linux_amd64 -platform=windows_amd64 -platform=darwin_amd64 -platform=darwin_arm64 >/dev/null)
git diff --no-index --stat "$LOCK/.terraform.lock.hcl" lockfiles/aws.terraform.lock.hcl; echo "diff exit $?"
rm -rf "$LOCK"
```

Expected: `diff exit 0` and no diff lines; on Windows Git may print line-ending warnings, and the exit code decides. This downloads four provider packages, about 40 s. If the registry is unreachable, record that and rely on CI's `-lockfile=readonly`.

- [ ] **Step 4: Embed it** — `lockfiles/lockfiles.go` (Edit).

1. Old:

```go
// arm64 whenever a provider version changes.
package lockfiles
```

new:

```go
// arm64 whenever a provider version changes (docs/runbooks/dependencies.md).
package lockfiles
```

2. Old:

```go
//go:embed github.terraform.lock.hcl
var GitHub []byte
```

new:

```go
//go:embed github.terraform.lock.hcl
var GitHub []byte

// AWS is the lock file of the AWS stacks (hashicorp/aws). The floci tests copy
// it into the stacks they apply, as the render will from Phase 2d.
//
//go:embed aws.terraform.lock.hcl
var AWS []byte
```

- [ ] **Step 5: Run it to see it pass**

Run: `go test ./lockfiles/ -count=1 -v`
Expected: `PASS`, with `TestGitHubLockPinsTheProvider` and `TestAWSLockPinsTheProvider`.

- [ ] **Step 6: Verify and commit** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).

```bash
git add lockfiles
git commit -m "feat(lockfiles): ship the aws provider lock file"
```

---

### Task 4: `modules/aws/baseline` — the account's GitHub OIDC provider

**Files:**
- Create: `modules/aws/baseline/versions.tf`, `modules/aws/baseline/tests/baseline.tftest.hcl`, `modules/aws/baseline/main.tf`, `modules/aws/baseline/outputs.tf`

**Interfaces:**
- Consumes: `lockfiles/aws.terraform.lock.hcl` (Task 3).
- Produces: `aws_iam_openid_connect_provider.this`; output `oidc_provider_arn`. No inputs. Task 7's `TestFloci` applies it; 2d renders `aws/<env>/_baseline` from it.

- [ ] **Step 1: Write the pin and the failing runs** (Write tool)

`modules/aws/baseline/versions.tf`:

```hcl
terraform {
  required_version = ">= 1.12.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.71.0"
    }
  }
}
```

`modules/aws/baseline/tests/baseline.tftest.hcl`:

```hcl
mock_provider "aws" {}

run "the_provider_is_github_actions_issuer" {
  command = plan

  assert {
    condition     = aws_iam_openid_connect_provider.this.url == "https://token.actions.githubusercontent.com"
    error_message = "the provider must be GitHub Actions' issuer, naming.OIDCProviderHost, so its ARN is naming.OIDCProviderARN (spec §5.5)"
  }
}

run "the_provider_accepts_only_the_sts_audience" {
  command = plan

  assert {
    condition     = tolist(aws_iam_openid_connect_provider.this.client_id_list) == tolist(["sts.amazonaws.com"])
    error_message = "the only client id must be sts.amazonaws.com, the aud of every CI role's trust policy (spec §4.4)"
  }
}
```

`client_id_list` is a set in the provider's schema, so the run compares lists.

- [ ] **Step 2: Run them to see them fail** — the module test snippet with `MODULE=aws/baseline`.

Expected: `fmt -check` prints nothing; `run "the_provider_is_github_actions_issuer"... fail` with `Reference to undeclared resource` and `There is no managed resource "aws_iam_openid_connect_provider" "this" definition in the root module.`; the summary is `Failure! 0 passed, 1 failed, 1 skipped.`

- [ ] **Step 3: Implement** (Write tool)

`modules/aws/baseline/main.tf`:

```hcl
# The GitHub Actions OIDC provider of an environment's AWS account (spec §5.4).
# IAM identifies a provider by its URL, so an account holds one: two
# environments in one account would create it twice, which claims validation
# rejects. Its ARN is naming.OIDCProviderARN(accountId), and the CI roles of
# modules/aws/component trust it by that computed ARN, never through this
# stack's state (spec §5.5).
resource "aws_iam_openid_connect_provider" "this" {
  url = "https://token.actions.githubusercontent.com"

  # The audience that aws-actions/configure-aws-credentials requests.
  client_id_list = ["sts.amazonaws.com"]

  # No thumbprint_list: IAM validates GitHub's issuer against its own trusted
  # root CAs and ignores thumbprints for it, and the attribute is optional.
}
```

`modules/aws/baseline/outputs.tf`:

```hcl
output "oidc_provider_arn" {
  description = "ARN of the account's GitHub Actions OIDC provider: naming.OIDCProviderARN."
  value       = aws_iam_openid_connect_provider.this.arn
}
```

- [ ] **Step 4: Run them to see them pass** — the module test snippet with `MODULE=aws/baseline`.

Expected: `fmt -check` prints nothing, both runs pass, and the summary is `Success! 2 passed, 0 failed.`

- [ ] **Step 5: Verify and commit** — `go test ./... && go vet ./... && gofmt -l .` pass; `git status --short` lists only `modules/aws/baseline/` (the snippet ran in a temporary copy, and `.gitignore` already ignores `modules/**/.terraform.lock.hcl`).

```bash
git add modules/aws/baseline
git commit -m "feat(modules): add the aws baseline module"
```

---

### Task 5: `modules/aws/component` — the ECR repository and the CI role

**Files:**
- Create: `modules/aws/component/versions.tf`, `modules/aws/component/tests/component.tftest.hcl`, `modules/aws/component/variables.tf`, `modules/aws/component/main.tf`, `modules/aws/component/outputs.tf`

**Interfaces:**
- Consumes: `lockfiles/aws.terraform.lock.hcl` (Task 3); the identities of `internal/naming` as inputs (D-8).
- Produces: the inputs `github_repository`, `environment`, `repository_name`, `role_name`, `oidc_provider_arn`; `aws_ecr_repository.this`, `aws_iam_role.ci`, `aws_iam_role_policy.push`; outputs `repository_arn`, `role_arn`. Task 7's `TestFloci` applies it with naming's values; 2d renders `aws/<env>/components/<name>` from it.

- [ ] **Step 1: Write the pin and the failing runs** (Write tool)

`modules/aws/component/versions.tf`: the same content as `modules/aws/baseline/versions.tf` (Task 4 Step 1).

`modules/aws/component/tests/component.tftest.hcl`:

```hcl
mock_provider "aws" {
  # A mock provider invents computed values; the push policy's resource is the
  # repository's ARN, so pin it to the form AWS returns.
  mock_resource "aws_ecr_repository" {
    defaults = {
      arn = "arn:aws:ecr:eu-west-1:000000000001:repository/api"
    }
  }
}

# The values the renderer passes from internal/naming for Component api in dev,
# account 000000000001 (spec §5.5; naming's TestIdentities).
variables {
  github_repository = "acme/api"
  environment       = "dev"
  repository_name   = "api"
  role_name         = "api-dev-ci"
  oidc_provider_arn = "arn:aws:iam::000000000001:oidc-provider/token.actions.githubusercontent.com"
}

run "names_are_the_ones_naming_computed" {
  command = plan

  assert {
    condition     = aws_ecr_repository.this.name == "api"
    error_message = "the repository must be named repository_name, naming.RepositoryName (spec §5.5)"
  }
  assert {
    condition     = aws_iam_role.ci.name == "api-dev-ci" && coalesce(aws_iam_role.ci.path, "/") == "/"
    error_message = "the role must be named role_name, naming.RoleName, at the default path, so its ARN is naming.RoleARN (spec §5.5)"
  }
}

run "the_trust_policy_admits_only_the_components_ci_in_its_environment" {
  command = plan

  assert {
    condition     = jsondecode(aws_iam_role.ci.assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:acme/api:environment:dev"
    error_message = "the trust policy's sub must be repo:<org>/<name>:environment:<env> (spec §4.4, §8.3)"
  }
  assert {
    condition     = jsondecode(aws_iam_role.ci.assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:aud"] == "sts.amazonaws.com"
    error_message = "the trust policy's aud must be sts.amazonaws.com (spec §4.4, §8.3)"
  }
  assert {
    condition = jsondecode(aws_iam_role.ci.assume_role_policy) == {
      Version = "2012-10-17"
      Statement = [{
        Effect    = "Allow"
        Principal = { Federated = "arn:aws:iam::000000000001:oidc-provider/token.actions.githubusercontent.com" }
        Action    = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
            "token.actions.githubusercontent.com:sub" = "repo:acme/api:environment:dev"
          }
        }
      }]
    }
    error_message = "the trust policy must be one statement: the account's GitHub OIDC provider, sts:AssumeRoleWithWebIdentity, and exact matches on aud and sub, nothing else"
  }
}

run "the_subject_follows_the_environment" {
  command = plan

  variables {
    environment = "pro"
    role_name   = "api-pro-ci"
  }

  assert {
    condition     = jsondecode(aws_iam_role.ci.assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:acme/api:environment:pro"
    error_message = "the role of one environment must not admit another environment's CI"
  }
}

run "the_ci_role_may_only_push_to_its_repository" {
  command = plan

  assert {
    condition     = aws_iam_role_policy.push.role == "api-dev-ci"
    error_message = "the push policy must belong to the CI role"
  }
  assert {
    condition = jsondecode(aws_iam_role_policy.push.policy) == {
      Version = "2012-10-17"
      Statement = [
        {
          Sid      = "ECRLogin"
          Effect   = "Allow"
          Action   = ["ecr:GetAuthorizationToken"]
          Resource = "*"
        },
        {
          Sid      = "ECRPush"
          Effect   = "Allow"
          Action   = ["ecr:BatchCheckLayerAvailability", "ecr:CompleteLayerUpload", "ecr:InitiateLayerUpload", "ecr:PutImage", "ecr:UploadLayerPart"]
          Resource = "arn:aws:ecr:eu-west-1:000000000001:repository/api"
        },
      ]
    }
    error_message = "the role may log in to the registry (an account-level action) and push to its own repository, nothing else (spec §4.4)"
  }
}

run "the_repository_keeps_its_tags_and_scans_pushes" {
  command = plan

  assert {
    condition     = aws_ecr_repository.this.image_tag_mutability == "IMMUTABLE"
    error_message = "a pushed tag must never move"
  }
  assert {
    condition     = aws_ecr_repository.this.image_scanning_configuration[0].scan_on_push
    error_message = "every push must be scanned"
  }
  assert {
    condition     = aws_ecr_repository.this.force_delete
    error_message = "deleting the Component must delete its repository even with images in it (spec §6.3)"
  }
}

run "rejects_another_issuer" {
  command = plan

  variables {
    oidc_provider_arn = "arn:aws:iam::000000000001:oidc-provider/accounts.google.com"
  }

  expect_failures = [var.oidc_provider_arn]
}

run "rejects_a_wildcard_repository" {
  command = plan

  variables {
    github_repository = "acme/*"
  }

  expect_failures = [var.github_repository]
}

run "rejects_an_environment_outside_the_name_rule" {
  command = plan

  variables {
    environment = "*"
  }

  expect_failures = [var.environment]
}
```

The policies are compared as decoded documents, so a statement, action, resource or condition added anywhere fails the run. `path` is null under the mock provider and `/` with the real one (D-19).

- [ ] **Step 2: Run them to see them fail** — the module test snippet with `MODULE=aws/component`.

Expected: `fmt -check` prints nothing; `run "names_are_the_ones_naming_computed"... fail` with `There is no managed resource "aws_ecr_repository" "this" definition in the root module.`; the summary is `Failure! 0 passed, 1 failed, 7 skipped.`

- [ ] **Step 3: Implement the inputs** — `modules/aws/component/variables.tf` (Write tool):

```hcl
variable "github_repository" {
  description = "The Component's GitHub repository, <org>/<name>: only its CI may assume the role."
  type        = string

  validation {
    condition     = can(regex("^[A-Za-z0-9-]+/[a-z0-9-]+$", var.github_repository))
    error_message = "github_repository must be <org>/<name>, letters, digits and hyphens only, so the trust policy names exactly one repository."
  }
}

variable "environment" {
  description = "The GitHub environment whose CI jobs may assume the role (spec §4.4)."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9]{0,9}$", var.environment))
    error_message = "environment must match ^[a-z][a-z0-9]{0,9}$ (spec §4.6)."
  }
}

variable "repository_name" {
  description = "ECR repository name: naming.RepositoryName (spec §5.5)."
  type        = string
}

variable "role_name" {
  description = "CI role name: naming.RoleName (spec §5.5)."
  type        = string
}

variable "oidc_provider_arn" {
  description = "ARN of the account's GitHub Actions OIDC provider, which the environment's baseline creates: naming.OIDCProviderARN (spec §5.5)."
  type        = string

  validation {
    condition     = can(regex("^arn:aws:iam::[0-9]{12}:oidc-provider/token\\.actions\\.githubusercontent\\.com$", var.oidc_provider_arn))
    error_message = "oidc_provider_arn must be the GitHub Actions OIDC provider of an account in the aws partition."
  }
}
```

- [ ] **Step 4: Implement the resources** — `modules/aws/component/main.tf` (Write tool):

```hcl
locals {
  # The issuer of GitHub Actions' OIDC tokens (naming.OIDCProviderHost): its
  # condition keys are <issuer>:aud and <issuer>:sub.
  issuer = "token.actions.githubusercontent.com"
}

# The Component's image repository in this environment's account (spec §4.4).
resource "aws_ecr_repository" "this" {
  name = var.repository_name

  # A pushed tag never moves, so a later CI run cannot replace an image that is
  # already deployed under it.
  image_tag_mutability = "IMMUTABLE"

  # Deleting the Component deletes its repository with any images in it (spec
  # §6.3). Every delete already waits for approval in idp-approval, and the
  # images are build output of the archived repository's commits.
  force_delete = true

  image_scanning_configuration {
    scan_on_push = true
  }
}

# The role the Component's CI assumes in this environment (spec §4.4): only a
# GitHub Actions job of the Component's repository, running in this
# environment, with a token for STS.
resource "aws_iam_role" "ci" {
  name = var.role_name

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Federated = var.oidc_provider_arn }
      Action    = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "${local.issuer}:aud" = "sts.amazonaws.com"
          "${local.issuer}:sub" = "repo:${var.github_repository}:environment:${var.environment}"
        }
      }
    }]
  })
}

# Push to this repository, and nothing else: no pull, no other repository.
resource "aws_iam_role_policy" "push" {
  name = "ecr-push"
  role = aws_iam_role.ci.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        # docker login. The action is account-level: IAM accepts only "*".
        Sid      = "ECRLogin"
        Effect   = "Allow"
        Action   = ["ecr:GetAuthorizationToken"]
        Resource = "*"
      },
      {
        Sid    = "ECRPush"
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:CompleteLayerUpload",
          "ecr:InitiateLayerUpload",
          "ecr:PutImage",
          "ecr:UploadLayerPart",
        ]
        Resource = aws_ecr_repository.this.arn
      },
    ]
  })
}
```

`modules/aws/component/outputs.tf` (Write tool):

```hcl
output "repository_arn" {
  description = "ARN of the ECR repository."
  value       = aws_ecr_repository.this.arn
}

output "role_arn" {
  description = "ARN of the CI role: naming.RoleARN."
  value       = aws_iam_role.ci.arn
}
```

- [ ] **Step 5: Run them to see them pass** — the module test snippet with `MODULE=aws/component`.

Expected: `fmt -check` prints nothing, every run passes, and the summary is `Success! 8 passed, 0 failed.`

- [ ] **Step 6: Verify and commit** — `go test ./... && go vet ./... && gofmt -l .` pass; `git status --short` lists only `modules/aws/component/`.

```bash
git add modules/aws/component
git commit -m "feat(modules): add the aws component module"
```

- [ ] **Step 7: Mutation checks** (one at a time, each restored with `git checkout modules/aws/component/main.tf`; nothing is committed). Run the module test snippet after each edit of `modules/aws/component/main.tf`:
  1. `Resource = aws_ecr_repository.this.arn` → `Resource = "*"`. Expected: `Failure! 7 passed, 1 failed.`, with `the_ci_role_may_only_push_to_its_repository` failing.
  2. After `  name = var.role_name` in `aws_iam_role.ci`, add the line `  path = "/ci/"`. Expected: `Failure! 7 passed, 1 failed.`, with `names_are_the_ones_naming_computed` failing.
  3. `StringEquals = {` → `StringLike = {`. Expected: `Failure! 1 passed, 1 failed, 6 skipped.`, with `the_trust_policy_admits_only_the_components_ci_in_its_environment` failing (`Unsupported attribute`).

  Afterwards `git status --short` prints nothing.

---

### Task 6: Renovate and the pin–lock contract

**Files:**
- Modify: `internal/workflowspec/renovate_test.go`, `.github/renovate-global.json`, `docs/runbooks/dependencies.md`

**Interfaces:**
- Consumes: `glob`, `repoRoot`, `loadRenovateConfig` (`internal/workflowspec`); the module pins (Tasks 4, 5) and the lock files (Task 3).
- Produces: the "aws provider" rule (D-14); `moduleDirs(t) []string` (Task 8 uses it); `TestModulesPinTheirLockedProvider`.

- [ ] **Step 1: Write the failing rule** — `internal/workflowspec/renovate_test.go`, in `TestRenovateRules` (Edit), old:

```go
		{MatchPackageNames: []string{"integrations/github"}, GroupName: "github provider", Labels: []string{"needs-lockfile"}},
```

new:

```go
		{MatchPackageNames: []string{"integrations/github"}, GroupName: "github provider", Labels: []string{"needs-lockfile"}},
		{MatchPackageNames: []string{"hashicorp/aws"}, GroupName: "aws provider", Labels: []string{"needs-lockfile"}},
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/workflowspec/ -count=1 -run TestRenovateRules`
Expected: `FAIL`: the `want` list has the `hashicorp/aws` rule fourth, and `packageRules` lacks it.

- [ ] **Step 3: Add the rule** — `.github/renovate-global.json` (Edit), old:

```json
      "groupName": "github provider",
      "labels": ["needs-lockfile"]
    },
```

new:

```json
      "groupName": "github provider",
      "labels": ["needs-lockfile"]
    },
    {
      "description": "The AWS provider: the pins of modules/aws/* move together; the lock file is regenerated by hand (docs/runbooks/dependencies.md).",
      "matchPackageNames": ["hashicorp/aws"],
      "groupName": "aws provider",
      "labels": ["needs-lockfile"]
    },
```

Run: `go test ./internal/workflowspec/ -count=1 -run 'TestRenovate'`
Expected: `ok`. The custom managers and the global config are unchanged (D-14).

- [ ] **Step 4: Add the pin–lock test** — `internal/workflowspec/renovate_test.go` (Edit).

1. Imports, old:

```go
	"os"
	"path/filepath"
```

new:

```go
	"os"
	"path"
	"path/filepath"
```

2. Old:

```go
func TestCIValidatesTheRenovateConfig(t *testing.T) {
```

new:

```go
// moduleDirs lists the engine's modules as <provider>/<name>, from their versions.tf.
func moduleDirs(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, f := range glob(t, "modules/*/*/versions.tf") {
		out = append(out, strings.TrimPrefix(path.Dir(f), "modules/"))
	}
	return out
}

// lockedVersion is the provider version a lock file pins.
var lockedVersion = regexp.MustCompile(`(?m)^[ \t]*version[ \t]*=[ \t]*"([^"]+)"`)

// TestModulesPinTheirLockedProvider: modules/<provider>/<name> pins exactly the
// provider version lockfiles/<provider>.terraform.lock.hcl locks. Renovate moves
// a module pin, and the lock file is regenerated by hand (needs-lockfile), so a
// pin without its lock fails here first (docs/runbooks/dependencies.md).
func TestModulesPinTheirLockedProvider(t *testing.T) {
	for _, m := range moduleDirs(t) {
		provider := strings.SplitN(m, "/", 2)[0]
		lock, err := os.ReadFile(filepath.Join(repoRoot, "lockfiles", provider+".terraform.lock.hcl"))
		if err != nil {
			t.Errorf("modules/%s: no lock file for its provider: %v", m, err)
			continue
		}
		pin, err := os.ReadFile(filepath.Join(repoRoot, "modules", filepath.FromSlash(m), "versions.tf"))
		if err != nil {
			t.Fatal(err)
		}
		// Anchored to the line start, so required_version = ">= 1.12.0" does not match.
		want, got := lockedVersion.FindSubmatch(lock), lockedVersion.FindSubmatch(pin)
		if want == nil || got == nil {
			t.Errorf("modules/%s or lockfiles/%s.terraform.lock.hcl names no provider version", m, provider)
			continue
		}
		if string(got[1]) != string(want[1]) {
			t.Errorf("modules/%s pins %s, lockfiles/%s.terraform.lock.hcl locks %s: regenerate the lock file (docs/runbooks/dependencies.md)", m, got[1], provider, want[1])
		}
	}
}

func TestCIValidatesTheRenovateConfig(t *testing.T) {
```

Run: `go test ./internal/workflowspec/ -count=1 -v -run TestModulesPinTheirLockedProvider`
Expected: `PASS`. It pins what Tasks 3 to 5 made; the mutation in Step 7 shows it bites.

- [ ] **Step 5: Update the runbook** — `docs/runbooks/dependencies.md` (exact replacements with Edit)

1. "What it updates", old:

```markdown
| The GitHub provider | `modules/github/*/versions.tf`, `GitHubProviderVersion` in `render.go` and the golden renders | group "github provider", label `needs-lockfile` |
```

new:

```markdown
| The GitHub provider | `modules/github/*/versions.tf`, `GitHubProviderVersion` in `render.go` and the golden renders | group "github provider", label `needs-lockfile` |
| The AWS provider | `modules/aws/*/versions.tf` | group "aws provider", label `needs-lockfile` |
```

2. Old:

```markdown
Renovate moves the provider pins but not the shipped lock file, so the PR's `go` and `modules` jobs fail (`TestRenderCopiesTheLockFile`, `tofu init -lockfile=readonly`).
```

new:

```markdown
Renovate moves the provider pins but not the shipped lock file, so the PR's `go` and `modules` jobs fail (`TestRenderCopiesTheLockFile`, `TestModulesPinTheirLockedProvider`, `tofu init -lockfile=readonly`).
```

3. Old:

```markdown
## An OpenTofu update
```

new:

````markdown
## An AWS provider update (`needs-lockfile`)

Renovate moves the pins in `modules/aws/*/versions.tf` but not `lockfiles/aws.terraform.lock.hcl`, so the PR's `go` job fails (`TestModulesPinTheirLockedProvider`), and so do `modules (aws/baseline)`, `modules (aws/component)` and `floci` (`tofu init -lockfile=readonly`). Regenerate the lock the same way, from the component module:

```bash
gh pr checkout <number> --repo jellalshadows-idp/idp-engine
cd modules/aws/component
rm -f .terraform.lock.hcl
"$HOME/.idp/tools/tofu-<version>/tofu.exe" providers lock -platform=linux_amd64 -platform=windows_amd64 -platform=darwin_amd64 -platform=darwin_arm64
mv .terraform.lock.hcl ../../../lockfiles/aws.terraform.lock.hcl
cd ../../..
```

Then set the new version in `lockfiles/lockfiles_test.go` (`TestAWSLockPinsTheProvider`), run `go test ./lockfiles/ ./internal/workflowspec/ ./internal/floci/`, commit as `chore(deps): regenerate the aws provider lock file` and push. The registry signs the checksums of every package of a release, and OpenTofu records them all whatever the `-platform` flags say; the flags keep one command for both providers.

## An OpenTofu update
````

- [ ] **Step 6: Verify and commit** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing); the naming check prints nothing.

```bash
git add .github/renovate-global.json internal/workflowspec/renovate_test.go docs/runbooks/dependencies.md
git commit -m "chore(renovate): group the aws provider and check module pins against the lock files"
```

- [ ] **Step 7: Mutation check** (nothing is committed) — in `modules/aws/component/versions.tf`, change `version = "6.71.0"` to `version = "6.72.0"`, as a Renovate PR would. Run `go test ./internal/workflowspec/ -count=1 -run TestModulesPinTheirLockedProvider`. Expected: `FAIL` with `modules/aws/component pins 6.72.0, lockfiles/aws.terraform.lock.hcl locks 6.71.0: regenerate the lock file (docs/runbooks/dependencies.md)`. Restore with `git checkout modules/aws/component/versions.tf`; `git status --short` prints nothing.

---

### Task 7: `internal/floci` — the level-3 tests

**Files:**
- Create: `internal/floci/doc.go`, `internal/floci/policy_test.go`, `internal/floci/iam_test.go`, `internal/floci/stack_test.go`, `internal/floci/fixture_test.go`, `internal/floci/floci_test.go`

**Interfaces:**
- Consumes: `naming.RepositoryName`, `naming.RoleName`, `naming.RoleARN`, `naming.OIDCProviderARN`, `naming.OIDCProviderHost`; `lockfiles.AWS`; `modules/aws/{baseline,component}` (Tasks 4, 5).
- Produces: `TestFloci`, gated by `IDP_FLOCI_ENDPOINT` (Task 8's job sets it); the offline tests `TestExpectedPoliciesAreTheModulesOwn`, `TestDecodePolicy`, `TestWriteStack` and `TestModulesUseNamingsIssuer`.

- [ ] **Step 1: The package** — create `internal/floci/doc.go` (Write tool):

```go
// Package floci holds the level-3 tests of the AWS modules (spec §8.4). CI's
// floci job applies modules/aws/baseline and modules/aws/component to two
// accounts of one floci emulator, with the inputs that internal/naming
// computes, replays them, checks the created identities and policies through
// the AWS API, and destroys them. The package has no code outside its tests,
// and TestFloci skips unless IDP_FLOCI_ENDPOINT is set, so go test ./... stays
// offline.
package floci
```

- [ ] **Step 2: Write the failing policy tests** — create `internal/floci/policy_test.go` (Write tool):

```go
package floci

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
)

// The documents that modules/aws/component plans for api in dev, account
// 000000000001, as OpenTofu's jsonencode writes them (keys sorted). Its tftest
// asserts the same documents.
const (
	moduleTrustPolicy = `{"Statement":[{"Action":"sts:AssumeRoleWithWebIdentity","Condition":{"StringEquals":{"token.actions.githubusercontent.com:aud":"sts.amazonaws.com","token.actions.githubusercontent.com:sub":"repo:acme/api:environment:dev"}},"Effect":"Allow","Principal":{"Federated":"arn:aws:iam::000000000001:oidc-provider/token.actions.githubusercontent.com"}}],"Version":"2012-10-17"}`
	modulePushPolicy  = `{"Statement":[{"Action":["ecr:GetAuthorizationToken"],"Effect":"Allow","Resource":"*","Sid":"ECRLogin"},{"Action":["ecr:BatchCheckLayerAvailability","ecr:CompleteLayerUpload","ecr:InitiateLayerUpload","ecr:PutImage","ecr:UploadLayerPart"],"Effect":"Allow","Resource":"arn:aws:ecr:eu-west-1:000000000001:repository/api","Sid":"ECRPush"}],"Version":"2012-10-17"}`
)

func TestExpectedPoliciesAreTheModulesOwn(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want policy
	}{
		{"trust policy", moduleTrustPolicy, trustPolicy(naming.OIDCProviderARN("000000000001"), "acme/api", "dev")},
		{"push policy", modulePushPolicy, pushPolicy("arn:aws:ecr:eu-west-1:000000000001:repository/api")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodePolicy(json.RawMessage(tt.doc))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decoded %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDecodePolicy(t *testing.T) {
	quoted, err := json.Marshal(url.PathEscape(moduleTrustPolicy))
	if err != nil {
		t.Fatal(err)
	}
	trust := trustPolicy(naming.OIDCProviderARN("000000000001"), "acme/api", "dev")
	tests := []struct {
		name string
		raw  string
		want policy
		bad  string // a substring of the error, when decoding must fail
	}{
		{name: "an object, as the AWS CLI prints it", raw: moduleTrustPolicy, want: trust},
		{name: "a URL-encoded string, as the IAM API returns it", raw: string(quoted), want: trust},
		{name: "a single statement and single values", raw: `{"Version":"2012-10-17","Statement":{"Effect":"Allow","Action":"ecr:PutImage","Resource":"*"}}`,
			want: policy{Version: "2012-10-17", Statement: statements{{Effect: "Allow", Action: stringList{"ecr:PutImage"}, Resource: stringList{"*"}}}}},
		{name: "NotResource, which widens a policy", raw: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"ecr:PutImage","NotResource":"x"}]}`, bad: "NotResource"},
		{name: "an unknown top-level element", raw: `{"Version":"2012-10-17","Id":"x","Statement":[]}`, bad: "Id"},
		{name: "a number for an action", raw: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":7}]}`, bad: "want a string or a list of strings"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodePolicy(json.RawMessage(tt.raw))
			if tt.bad != "" {
				if err == nil || !strings.Contains(err.Error(), tt.bad) {
					t.Fatalf("decodePolicy = %+v, %v; want an error about %s", got, err, tt.bad)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decoded %+v, want %+v", got, tt.want)
			}
		})
	}
}
```

The two constants are what `tofu console` prints for the module's `jsonencode` expressions with those inputs (checked while planning), so the Go expectations and the module are tied together offline.

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/floci/ -count=1`
Expected: `FAIL` with `[build failed]` and `undefined: policy`, `undefined: trustPolicy`, `undefined: pushPolicy`, `undefined: decodePolicy`.

- [ ] **Step 4: The IAM helpers** — create `internal/floci/iam_test.go` (Write tool):

```go
package floci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
)

// stringList is an IAM policy element written as one string or a list of them.
type stringList []string

func (l *stringList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*l = stringList{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("want a string or a list of strings, got %s", b)
	}
	*l = many
	return nil
}

// statement is one IAM policy statement. Decoding refuses any other element
// (NotAction, NotResource, NotPrincipal), so a policy cannot widen itself
// through an element this comparison does not read.
type statement struct {
	Sid       string                           `json:"Sid,omitempty"`
	Effect    string                           `json:"Effect"`
	Principal map[string]stringList            `json:"Principal,omitempty"`
	Action    stringList                       `json:"Action"`
	Resource  stringList                       `json:"Resource,omitempty"`
	Condition map[string]map[string]stringList `json:"Condition,omitempty"`
}

// statements is a policy's Statement, written as one statement or a list of them.
type statements []statement

func (s *statements) UnmarshalJSON(b []byte) error {
	if t := bytes.TrimSpace(b); len(t) > 0 && t[0] == '{' {
		var one statement
		if err := strictJSON(t, &one); err != nil {
			return err
		}
		*s = statements{one}
		return nil
	}
	var many []statement
	if err := strictJSON(b, &many); err != nil {
		return err
	}
	*s = many
	return nil
}

// policy is an IAM policy document.
type policy struct {
	Version   string     `json:"Version"`
	Statement statements `json:"Statement"`
}

func strictJSON(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// decodePolicy reads a policy document as the AWS CLI prints it: an object, or
// the URL-encoded string that the IAM API returns.
func decodePolicy(raw json.RawMessage) (policy, error) {
	doc := []byte(raw)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		unescaped, err := url.PathUnescape(s)
		if err != nil {
			return policy{}, err
		}
		doc = []byte(unescaped)
	}
	var p policy
	if err := strictJSON(doc, &p); err != nil {
		return policy{}, fmt.Errorf("policy document %s: %w", doc, err)
	}
	return p, nil
}

// trustPolicy is the only trust policy a Component's CI role may have (spec
// §4.4): GitHub Actions tokens of one repository and environment, for STS.
func trustPolicy(providerARN, githubRepository, env string) policy {
	return policy{Version: "2012-10-17", Statement: statements{{
		Effect:    "Allow",
		Principal: map[string]stringList{"Federated": {providerARN}},
		Action:    stringList{"sts:AssumeRoleWithWebIdentity"},
		Condition: map[string]map[string]stringList{"StringEquals": {
			naming.OIDCProviderHost + ":aud": {"sts.amazonaws.com"},
			naming.OIDCProviderHost + ":sub": {"repo:" + githubRepository + ":environment:" + env},
		}},
	}}}
}

// pushPolicy is the only policy a Component's CI role may have (spec §4.4):
// log in to the registry, an account-level action, and push to its repository.
func pushPolicy(repositoryARN string) policy {
	return policy{Version: "2012-10-17", Statement: statements{
		{Sid: "ECRLogin", Effect: "Allow", Action: stringList{"ecr:GetAuthorizationToken"}, Resource: stringList{"*"}},
		{Sid: "ECRPush", Effect: "Allow", Action: stringList{
			"ecr:BatchCheckLayerAvailability",
			"ecr:CompleteLayerUpload",
			"ecr:InitiateLayerUpload",
			"ecr:PutImage",
			"ecr:UploadLayerPart",
		}, Resource: stringList{repositoryARN}},
	}}
}
```

Run: `go test ./internal/floci/ -count=1 -v -run 'TestExpectedPolicies|TestDecodePolicy'`
Expected: `PASS`, with 2 and 6 subtests.

- [ ] **Step 5: Write the failing stack tests** — create `internal/floci/stack_test.go` (Write tool):

```go
package floci

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

func TestWriteStack(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dev", "component")
	tg := targets[0]
	if err := writeStack(dir, "component", tg, "http://localhost:4566", componentInputs(tg)); err != nil {
		t.Fatal(err)
	}
	read := func(name string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return v
	}
	wantMain := map[string]any{
		"module": map[string]any{"component": map[string]any{
			"source":            "../../modules/aws/component",
			"github_repository": "acme/api",
			"environment":       "dev",
			"repository_name":   "api",
			"role_name":         "api-dev-ci",
			"oidc_provider_arn": "arn:aws:iam::000000000001:oidc-provider/token.actions.githubusercontent.com",
		}},
		"provider": map[string]any{"aws": map[string]any{"region": "eu-west-1"}},
	}
	if got := read("main.tf.json"); !reflect.DeepEqual(got, wantMain) {
		t.Errorf("main.tf.json = %v, want %v", got, wantMain)
	}
	wantOverride := map[string]any{"provider": map[string]any{"aws": map[string]any{
		"endpoints":                   map[string]any{"ecr": "http://localhost:4566", "iam": "http://localhost:4566", "sts": "http://localhost:4566"},
		"skip_credentials_validation": true,
		"skip_metadata_api_check":     "true",
		"skip_requesting_account_id":  true,
	}}}
	if got := read("floci_override.tf.json"); !reflect.DeepEqual(got, wantOverride) {
		t.Errorf("floci_override.tf.json = %v, want %v", got, wantOverride)
	}
	lock, err := os.ReadFile(filepath.Join(dir, ".terraform.lock.hcl"))
	if err != nil || !bytes.Equal(lock, lockfiles.AWS) {
		t.Errorf("the stack must carry the engine's AWS lock file (err %v)", err)
	}
}

// TestModulesUseNamingsIssuer pins the one identity the modules write
// themselves: GitHub Actions' issuer, whose host is in every OIDC provider ARN
// that internal/naming computes and in the trust policy's condition keys.
func TestModulesUseNamingsIssuer(t *testing.T) {
	for file, want := range map[string]string{
		"baseline/main.tf":       `url = "https://` + naming.OIDCProviderHost + `"`,
		"component/main.tf":      `issuer = "` + naming.OIDCProviderHost + `"`,
		"component/variables.tf": strings.ReplaceAll(naming.OIDCProviderHost, ".", `\\.`) + `$"`,
	} {
		b, err := os.ReadFile(filepath.Join(modulesDir, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), want) {
			t.Errorf("modules/aws/%s does not contain %s: the modules and internal/naming must name the same issuer", file, want)
		}
	}
}
```

The override's flags are the Phase 0 spike's (`ci/floci_override.tf.json`); `skip_metadata_api_check` is a string in the provider's schema.

- [ ] **Step 6: Run them to see them fail**

Run: `go test ./internal/floci/ -count=1`
Expected: `FAIL` with `[build failed]` and `undefined: targets`, `undefined: writeStack`, `undefined: componentInputs`, `undefined: modulesDir`.

- [ ] **Step 7: The test platform and the stack writer** — create `internal/floci/fixture_test.go` (Write tool):

```go
package floci

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

// modulesDir is the engine's AWS modules, seen from this package's directory.
const modulesDir = "../../modules/aws"

// target is one environment of the test platform: its account and region.
type target struct {
	env, accountID, region string
}

// The test platform: one registry Component, api of the GitHub org acme, in two
// environments in two accounts of one floci (ADR-0004).
const (
	org       = "acme"
	component = "api"
)

var targets = []target{
	{env: "dev", accountID: "000000000001", region: "eu-west-1"},
	{env: "pro", accountID: "000000000003", region: "eu-west-1"},
}

// githubRepository is the Component's repository, <org>/<name>.
func githubRepository() string { return org + "/" + component }

// componentInputs are the inputs modules/aws/component takes in an environment.
// They come from internal/naming, as Phase 2d's render will pass them (spec
// §5.5), so a module that does not create exactly these names fails here.
func componentInputs(tg target) map[string]any {
	return map[string]any{
		"github_repository": githubRepository(),
		"environment":       tg.env,
		"repository_name":   naming.RepositoryName(component),
		"role_name":         naming.RoleName(component, tg.env),
		"oidc_provider_arn": naming.OIDCProviderARN(tg.accountID),
	}
}

// repositoryARN is the ARN AWS gives the Component's repository. naming has no
// function for it: the push policy reads it from the repository resource.
func repositoryARN(tg target) string {
	return "arn:aws:ecr:" + tg.region + ":" + tg.accountID + ":repository/" + naming.RepositoryName(component)
}

// writeStack writes the root config of one stack into dir, which sits two
// levels below the copied modules: the module call with its inputs and the
// region, as the render will write it, then what CI adds for floci (spec
// §5.6): floci_override.tf.json, with the endpoint and the provider flags, and
// the engine's lock file.
func writeStack(dir, module string, tg target, endpoint string, inputs map[string]any) error {
	call := map[string]any{"source": "../../modules/aws/" + module}
	maps.Copy(call, inputs)
	files := map[string]any{
		"main.tf.json": map[string]any{
			"module":   map[string]any{module: call},
			"provider": map[string]any{"aws": map[string]any{"region": tg.region}},
		},
		"floci_override.tf.json": map[string]any{
			"provider": map[string]any{"aws": map[string]any{
				"endpoints":                   map[string]any{"ecr": endpoint, "iam": endpoint, "sts": endpoint},
				"skip_credentials_validation": true,
				"skip_metadata_api_check":     "true",
				"skip_requesting_account_id":  true,
			}},
		},
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, doc := range files {
		b, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, ".terraform.lock.hcl"), lockfiles.AWS, 0o644)
}
```

Run: `go test ./internal/floci/ -count=1 -v`
Expected: `PASS`, with `TestExpectedPoliciesAreTheModulesOwn`, `TestDecodePolicy`, `TestWriteStack` and `TestModulesUseNamingsIssuer`.

- [ ] **Step 8: The floci test** — create `internal/floci/floci_test.go` (Write tool):

```go
package floci

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
)

// endpointEnv names floci's endpoint. CI's floci job sets it; without it
// TestFloci skips, so go test ./... needs no emulator.
const endpointEnv = "IDP_FLOCI_ENDPOINT"

// fakeSecret is the secret access key floci receives. floci does not check
// signatures, and its account is the 12-digit access key ID (spec §5.6, §7.6).
const fakeSecret = "floci-does-not-check-it"

// floci runs tofu and the AWS CLI against one floci, as one account at a time.
type floci struct {
	endpoint, tofu, aws, root string
}

// env is the environment of a command run as the account of tg.
func (f *floci) env(tg target) []string {
	return append(os.Environ(),
		"AWS_ACCESS_KEY_ID="+tg.accountID,
		"AWS_SECRET_ACCESS_KEY="+fakeSecret,
		"AWS_REGION="+tg.region,
		"AWS_DEFAULT_REGION="+tg.region,
		"AWS_PAGER=",
		"TF_IN_AUTOMATION=1",
	)
}

// run runs a command as the account of tg and returns its combined output and
// exit code. A command that cannot start fails the test.
func (f *floci) run(t *testing.T, tg target, dir, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = f.env(tg)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exit):
		return string(out), exit.ExitCode()
	}
	t.Fatalf("%s %v: %v", name, args, err)
	return "", -1
}

// tofuOK runs tofu in dir and fails the test unless it exits 0.
func (f *floci) tofuOK(t *testing.T, tg target, dir string, args ...string) {
	t.Helper()
	if out, code := f.run(t, tg, dir, f.tofu, args...); code != 0 {
		t.Fatalf("tofu %v in %s exited %d:\n%s", args, dir, code, out)
	}
}

// awsJSON runs an AWS CLI command against floci as the account of tg and
// decodes its JSON output into out. It returns false when the command fails,
// as it does for a resource that does not exist.
func (f *floci) awsJSON(t *testing.T, tg target, out any, args ...string) bool {
	t.Helper()
	args = append(args, "--endpoint-url", f.endpoint, "--output", "json")
	stdout, code := f.run(t, tg, f.root, f.aws, args...)
	if code != 0 {
		return false
	}
	if err := json.NewDecoder(bytes.NewReader([]byte(stdout))).Decode(out); err != nil {
		t.Fatalf("aws %v: %v\n%s", args, err, stdout)
	}
	return true
}

// apply creates one stack of tg's environment the way the pipelines will (spec
// §6.1): init with the shipped lock file, apply, then replay, a plan of the
// same version that must find nothing to change (spec §8.4). It registers the
// stack's destroy before applying, so whatever an apply created is destroyed,
// even when the test fails.
func (f *floci) apply(t *testing.T, tg target, module string, inputs map[string]any) {
	t.Helper()
	dir := filepath.Join(f.root, tg.env, module)
	if err := writeStack(dir, module, tg, f.endpoint, inputs); err != nil {
		t.Fatal(err)
	}
	f.tofuOK(t, tg, dir, "init", "-input=false", "-lockfile=readonly", "-no-color")
	t.Cleanup(func() {
		if out, code := f.run(t, tg, dir, f.tofu, "destroy", "-input=false", "-auto-approve", "-no-color"); code != 0 {
			t.Errorf("%s %s: destroy exited %d:\n%s", tg.env, module, code, out)
		}
	})
	f.tofuOK(t, tg, dir, "apply", "-input=false", "-auto-approve", "-no-color")
	if out, code := f.run(t, tg, dir, f.tofu, "plan", "-input=false", "-detailed-exitcode", "-no-color"); code != 0 {
		t.Errorf("%s %s: the replay plan exited %d, want 0 (no changes):\n%s", tg.env, module, code, out)
	}
}

// TestFloci is the level-3 test of the AWS modules (spec §8.4). Against one
// floci it applies, as each account, the environment's baseline and then its
// Component stack, with the inputs internal/naming computes. It checks through
// the AWS API that the created identities are naming's (ADR-0006), that the
// trust and push policies are exactly spec §4.4's, and that one account cannot
// see another's role (ADR-0004). It destroys everything and checks that
// nothing is left (spec §6.3).
func TestFloci(t *testing.T) {
	endpoint := os.Getenv(endpointEnv)
	if endpoint == "" {
		t.Skipf("set %s to run the level-3 tests against floci (spec §8.4); CI's floci job does", endpointEnv)
	}
	f := &floci{endpoint: endpoint, root: t.TempDir()}
	for _, tool := range []struct {
		name string
		path *string
	}{{"tofu", &f.tofu}, {"aws", &f.aws}} {
		p, err := exec.LookPath(tool.name)
		if err != nil {
			t.Fatalf("%s is not on PATH: %v", tool.name, err)
		}
		*tool.path = p
	}
	if err := os.CopyFS(filepath.Join(f.root, "modules", "aws"), os.DirFS(modulesDir)); err != nil {
		t.Fatal(err)
	}

	// Registered first, so it runs last, after every destroy.
	t.Cleanup(func() {
		for _, tg := range targets {
			f.wantNothingLeft(t, tg)
		}
	})
	for _, tg := range targets {
		f.apply(t, tg, "baseline", map[string]any{})
		f.apply(t, tg, "component", componentInputs(tg))
	}
	for _, tg := range targets {
		t.Run(tg.env, func(t *testing.T) { f.wantCreated(t, tg) })
	}
	t.Run("accounts are isolated", func(t *testing.T) {
		dev, pro := targets[0], targets[1]
		var role struct{}
		if f.awsJSON(t, pro, &role, "iam", "get-role", "--role-name", naming.RoleName(component, dev.env)) {
			t.Errorf("account %s can read the role of account %s (ADR-0004)", pro.accountID, dev.accountID)
		}
	})
}

// wantCreated checks the stacks of one environment through the AWS API. It
// compares names and ARNs, never the registry URL, whose host floci changes
// (naming.RegistryURL, ADR-0013).
func (f *floci) wantCreated(t *testing.T, tg target) {
	var providers struct {
		OpenIDConnectProviderList []struct{ Arn string }
	}
	if !f.awsJSON(t, tg, &providers, "iam", "list-open-id-connect-providers") {
		t.Fatal("list-open-id-connect-providers failed")
	}
	if got, want := providers.OpenIDConnectProviderList, naming.OIDCProviderARN(tg.accountID); len(got) != 1 || got[0].Arn != want {
		t.Errorf("OIDC providers = %+v, want exactly %s (naming.OIDCProviderARN)", got, want)
	}
	var provider struct {
		Url          string
		ClientIDList []string
	}
	if !f.awsJSON(t, tg, &provider, "iam", "get-open-id-connect-provider", "--open-id-connect-provider-arn", naming.OIDCProviderARN(tg.accountID)) {
		t.Fatal("get-open-id-connect-provider failed")
	}
	if provider.Url != naming.OIDCProviderHost || !slices.Equal(provider.ClientIDList, []string{"sts.amazonaws.com"}) {
		t.Errorf("OIDC provider = %+v, want URL %s and the client id sts.amazonaws.com", provider, naming.OIDCProviderHost)
	}

	var repos struct {
		Repositories []struct {
			RepositoryName             string
			RepositoryArn              string
			RepositoryUri              string
			ImageTagMutability         string
			ImageScanningConfiguration struct{ ScanOnPush bool }
		}
	}
	if !f.awsJSON(t, tg, &repos, "ecr", "describe-repositories") {
		t.Fatal("describe-repositories failed")
	}
	got := repos.Repositories
	if len(got) != 1 || got[0].RepositoryName != naming.RepositoryName(component) || got[0].RepositoryArn != repositoryARN(tg) ||
		got[0].ImageTagMutability != "IMMUTABLE" || !got[0].ImageScanningConfiguration.ScanOnPush {
		t.Errorf("repositories = %+v, want exactly %s (%s), IMMUTABLE and scanned on push", got, naming.RepositoryName(component), repositoryARN(tg))
	}
	for _, r := range got {
		// Logged for the phase log, never compared (ADR-0013).
		t.Logf("floci's repository URI for %s: %s", r.RepositoryName, r.RepositoryUri)
	}

	roleName := naming.RoleName(component, tg.env)
	var role struct {
		Role struct {
			Arn                      string
			AssumeRolePolicyDocument json.RawMessage
		}
	}
	if !f.awsJSON(t, tg, &role, "iam", "get-role", "--role-name", roleName) {
		t.Fatalf("get-role %s failed", roleName)
	}
	if want := naming.RoleARN(tg.accountID, component, tg.env); role.Role.Arn != want {
		t.Errorf("role ARN = %s, want %s (naming.RoleARN)", role.Role.Arn, want)
	}
	f.wantPolicy(t, "trust policy", role.Role.AssumeRolePolicyDocument, trustPolicy(naming.OIDCProviderARN(tg.accountID), githubRepository(), tg.env))

	var inline struct{ PolicyNames []string }
	var attached struct{ AttachedPolicies []struct{ PolicyArn string } }
	if !f.awsJSON(t, tg, &inline, "iam", "list-role-policies", "--role-name", roleName) ||
		!f.awsJSON(t, tg, &attached, "iam", "list-attached-role-policies", "--role-name", roleName) {
		t.Fatal("listing the role's policies failed")
	}
	if !slices.Equal(inline.PolicyNames, []string{"ecr-push"}) || len(attached.AttachedPolicies) != 0 {
		t.Errorf("role policies: inline %v, attached %+v; want only the inline ecr-push", inline.PolicyNames, attached.AttachedPolicies)
	}
	var push struct{ PolicyDocument json.RawMessage }
	if !f.awsJSON(t, tg, &push, "iam", "get-role-policy", "--role-name", roleName, "--policy-name", "ecr-push") {
		t.Fatal("get-role-policy ecr-push failed")
	}
	f.wantPolicy(t, "push policy", push.PolicyDocument, pushPolicy(repositoryARN(tg)))
}

func (f *floci) wantPolicy(t *testing.T, what string, raw json.RawMessage, want policy) {
	t.Helper()
	got, err := decodePolicy(raw)
	if err != nil {
		t.Errorf("%s: %v", what, err)
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %+v, want exactly %+v (spec §4.4)", what, got, want)
	}
}

// wantNothingLeft checks, after the destroys, that tg's account holds no OIDC
// provider, repository or CI role: deleting a Component destroys its
// repository and role (spec §6.3).
func (f *floci) wantNothingLeft(t *testing.T, tg target) {
	var providers struct {
		OpenIDConnectProviderList []struct{ Arn string }
	}
	if !f.awsJSON(t, tg, &providers, "iam", "list-open-id-connect-providers") || len(providers.OpenIDConnectProviderList) != 0 {
		t.Errorf("%s: OIDC providers left after destroy: %+v", tg.env, providers.OpenIDConnectProviderList)
	}
	var repos struct {
		Repositories []struct{ RepositoryName string }
	}
	if !f.awsJSON(t, tg, &repos, "ecr", "describe-repositories") || len(repos.Repositories) != 0 {
		t.Errorf("%s: repositories left after destroy: %+v", tg.env, repos.Repositories)
	}
	var role struct{}
	if f.awsJSON(t, tg, &role, "iam", "get-role", "--role-name", naming.RoleName(component, tg.env)) {
		t.Errorf("%s: role %s left after destroy", tg.env, naming.RoleName(component, tg.env))
	}
}
```

Notes for the reviewer:
- Go decodes JSON keys case-insensitively, so the ECR API's `repositoryName` fills `RepositoryName`.
- Cleanups run in reverse order: each account's component stack is destroyed before its baseline, and `wantNothingLeft` runs last.
- A later duplicate in `cmd.Env` wins, so the account's keys override any `AWS_*` variable the runner has.
- Both accounts create the same `api` repository and the same OIDC provider URL, so floci must isolate them for the applies to succeed.

Run: `go test ./internal/floci/ -count=1 -v`
Expected: `PASS`, with `--- SKIP: TestFloci` (`set IDP_FLOCI_ENDPOINT to run the level-3 tests against floci (spec §8.4); CI's floci job does`) and the four offline tests passing.

- [ ] **Step 9: Show that a set endpoint never skips** (nothing is written)

Run: `IDP_FLOCI_ENDPOINT=http://127.0.0.1:9 go test ./internal/floci/ -run '^TestFloci$' -count=1`
Expected: `FAIL`, never `ok` or `SKIP`. Without `tofu` on `PATH` (the owner runs it by its full path), the message is `tofu is not on PATH: …`. With `tofu` but no AWS CLI it is `aws is not on PATH: …`. With both, `tofu init` or `apply` fails against the closed port. Seen while planning: `--- FAIL: TestFloci` with `tofu is not on PATH`.

- [ ] **Step 10: Verify and commit** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing); the naming check prints nothing.

```bash
git add internal/floci
git commit -m "test(floci): apply the aws modules to floci and check them against naming"
```

- [ ] **Step 11: Mutation check** (nothing is committed) — in `modules/aws/baseline/main.tf`, change `url = "https://token.actions.githubusercontent.com"` to `url = "https://token.actions.githubusercontent.com/"`.
  - Run `go test ./internal/floci/ -count=1 -run TestModulesUseNamingsIssuer`. Expected: `FAIL` with `modules/aws/baseline/main.tf does not contain url = "https://token.actions.githubusercontent.com": the modules and internal/naming must name the same issuer`.
  - Run the module test snippet with `MODULE=aws/baseline`. Expected: `Failure! 1 passed, 1 failed.`

  Restore with `git checkout modules/aws/baseline/main.tf`; `git status --short` prints nothing.

---

### Task 8: CI — every module with its lock file, the floci job, one fixture list

**Files:**
- Create: `internal/workflowspec/ci_test.go`
- Modify: `internal/workflowspec/load_test.go`, `.github/workflows/ci.yaml`, `docs/runbooks/dependencies.md`

**Interfaces:**
- Consumes: `moduleDirs` (Task 6); `ciTofuVersion`, `mustJob`, `mustStep`, `jobNode`, `mapValue`, `wantPerms`, `wantLines`, `glob` (`internal/workflowspec`); `TestFloci` and `IDP_FLOCI_ENDPOINT` (Task 7).
- Produces: the job field `Services map[string]service` and the type `service`; `TestCITestsEveryModule`, `TestCIRenderSmokeCoversEveryFixture`, `TestCIFlociJob`; the `ci.yaml` job `floci`; the step ids `tofu-test` and `render-validate`.

- [ ] **Step 1: Decode service containers** — `internal/workflowspec/load_test.go` (Edit), old:

```go
type job struct {
	Name           string    `yaml:"name"`
	Needs          strlist   `yaml:"needs"`
	If             string    `yaml:"if"`
	Environment    string    `yaml:"environment"`
	TimeoutMinutes int       `yaml:"timeout-minutes"`
	Env            strmap    `yaml:"env"`
	Permissions    perms     `yaml:"permissions"`
	Outputs        strmap    `yaml:"outputs"`
	Uses           string    `yaml:"uses"`
	With           strmap    `yaml:"with"`
	Secrets        yaml.Node `yaml:"secrets"`
	Steps          []step    `yaml:"steps"`
}
```

new:

```go
type job struct {
	Name           string             `yaml:"name"`
	Needs          strlist            `yaml:"needs"`
	If             string             `yaml:"if"`
	Environment    string             `yaml:"environment"`
	TimeoutMinutes int                `yaml:"timeout-minutes"`
	Env            strmap             `yaml:"env"`
	Permissions    perms              `yaml:"permissions"`
	Outputs        strmap             `yaml:"outputs"`
	Uses           string             `yaml:"uses"`
	With           strmap             `yaml:"with"`
	Secrets        yaml.Node          `yaml:"secrets"`
	Services       map[string]service `yaml:"services"`
	Steps          []step             `yaml:"steps"`
}

// service is a job's service container.
type service struct {
	Image   string   `yaml:"image"`
	Ports   []string `yaml:"ports"`
	Volumes []string `yaml:"volumes"`
	Env     strmap   `yaml:"env"`
	Options string   `yaml:"options"`
}
```

Run: `go test ./internal/workflowspec/ -count=1`
Expected: `ok` (no workflow has services yet).

- [ ] **Step 2: Write the failing contract tests** — create `internal/workflowspec/ci_test.go` (Write tool):

```go
package workflowspec

import (
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const ciPath = ".github/workflows/ci.yaml"

// TestCITestsEveryModule: the modules job tests every module, each with the lock
// file of its provider.
func TestCITestsEveryModule(t *testing.T) {
	w := loadWorkflow(t, ciPath)
	n := mapValue(mapValue(mapValue(jobNode(w, "modules"), "strategy"), "matrix"), "module")
	if n == nil {
		t.Fatal("ci modules: no strategy.matrix.module")
	}
	var matrix []string
	if err := n.Decode(&matrix); err != nil {
		t.Fatal(err)
	}
	if got, want := slices.Sorted(slices.Values(matrix)), moduleDirs(t); !slices.Equal(got, want) {
		t.Errorf("ci modules matrix = %v, want every module: %v", got, want)
	}
	_, s := mustStep(t, "ci modules", mustJob(t, w, "modules").Steps, "tofu-test")
	if s.WorkingDirectory != "modules/${{ matrix.module }}" || s.Env["MODULE"] != "${{ matrix.module }}" {
		t.Errorf("ci modules tofu-test: working-directory %q, MODULE %q; want the matrix module", s.WorkingDirectory, s.Env["MODULE"])
	}
	wantLines(t, "ci modules tofu-test", s.Run, []string{
		"tofu fmt -check -recursive",
		`cp "$GITHUB_WORKSPACE/lockfiles/${MODULE%%/*}.terraform.lock.hcl" .terraform.lock.hcl`,
		"tofu init -input=false -lockfile=readonly",
		"tofu test",
	})
}

// TestCIRenderSmokeCoversEveryFixture: render-smoke renders and validates every
// fixture that has a golden render, from one list.
func TestCIRenderSmokeCoversEveryFixture(t *testing.T) {
	var want []string
	for _, d := range glob(t, "internal/render/testdata/*/expected") {
		want = append(want, path.Base(path.Dir(d)))
	}
	loop := regexp.MustCompile(`(?m)^\s*for fixture in ([^;]+); do$`)
	var lists [][]string
	for _, s := range mustJob(t, loadWorkflow(t, ciPath), "render-smoke").Steps {
		for _, m := range loop.FindAllStringSubmatch(s.Run, -1) {
			lists = append(lists, strings.Fields(m[1]))
		}
	}
	if len(lists) != 1 {
		t.Fatalf("render-smoke lists the fixtures %d times, want once: %v", len(lists), lists)
	}
	if got := slices.Sorted(slices.Values(lists[0])); !slices.Equal(got, want) {
		t.Errorf("render-smoke fixtures = %v, want every fixture with a golden render: %v", got, want)
	}
	_, s := mustStep(t, "ci render-smoke", mustJob(t, loadWorkflow(t, ciPath), "render-smoke").Steps, "render-validate")
	wantLines(t, "ci render-smoke", s.Run, []string{
		`go run ./cmd/idp render --dir "internal/render/testdata/$fixture" --out "$RUNNER_TEMP/rendered/$fixture" --modules-dir modules`,
		`tofu -chdir="$RUNNER_TEMP/rendered/$fixture/github" init -backend=false -input=false`,
		`tofu -chdir="$RUNNER_TEMP/rendered/$fixture/github" validate`,
	})
}

// TestCIFlociJob pins the level-3 job (spec §8.4): floci pinned by tag and
// index digest, so Renovate moves both (spec §7.5); the Docker socket for its
// ECR; a readiness wait that fails instead of continuing; and the test step
// with the endpoint set, so TestFloci runs instead of skipping.
func TestCIFlociJob(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, ciPath), "floci")
	wantPerms(t, "ci floci", j.Permissions, map[string]string{"contents": "read"})
	if j.TimeoutMinutes < 1 || j.TimeoutMinutes > 30 {
		t.Errorf("ci floci timeout-minutes = %d, want 1 to 30: a hung emulator must not hold a runner for 6 hours", j.TimeoutMinutes)
	}
	if got := slices.Sorted(maps.Keys(j.Services)); !slices.Equal(got, []string{"floci"}) {
		t.Fatalf("ci floci services = %v, want floci", got)
	}
	svc := j.Services["floci"]
	if !regexp.MustCompile(`^floci/floci:\d+\.\d+\.\d+@sha256:[0-9a-f]{64}$`).MatchString(svc.Image) {
		t.Errorf("floci image = %q, want floci/floci:X.Y.Z@sha256:<index digest> (ADR-0004)", svc.Image)
	}
	if !slices.Equal(svc.Ports, []string{"4566:4566"}) || !slices.Equal(svc.Volumes, []string{"/var/run/docker.sock:/var/run/docker.sock"}) {
		t.Errorf("floci ports %v, volumes %v; want 4566:4566 and the Docker socket", svc.Ports, svc.Volumes)
	}
	if len(svc.Env) != 0 || svc.Options != "" {
		t.Errorf("floci env %v, options %q; want floci's defaults", svc.Env, svc.Options)
	}
	var tofu []string
	for _, s := range j.Steps {
		if strings.HasPrefix(s.Uses, "opentofu/setup-opentofu@") {
			tofu = append(tofu, s.With["tofu_version"])
		}
	}
	if !slices.Equal(tofu, []string{ciTofuVersion(t)}) {
		t.Errorf("ci floci installs OpenTofu %v, want once the modules job's %s", tofu, ciTofuVersion(t))
	}
	ri, ready := mustStep(t, "ci floci", j.Steps, "floci-ready")
	ti, test := mustStep(t, "ci floci", j.Steps, "floci-test")
	if ri > ti {
		t.Error("ci floci: the tests run before floci is ready")
	}
	for _, s := range []step{ready, test} {
		if s.If != "" || s.ContinueOnError != "" {
			t.Errorf("ci floci %s: if %q, continue-on-error %q; it must run, and fail the job", s.ID, s.If, s.ContinueOnError)
		}
	}
	wantLines(t, "ci floci-ready", ready.Run, []string{
		`echo "::error::floci did not report ready on localhost:4566 within 60 seconds"`,
		"exit 1",
	})
	if test.Env["IDP_FLOCI_ENDPOINT"] != "http://localhost:4566" {
		t.Errorf("ci floci-test env IDP_FLOCI_ENDPOINT = %q, want http://localhost:4566: without it TestFloci skips", test.Env["IDP_FLOCI_ENDPOINT"])
	}
	if got, want := strings.TrimSpace(test.Run), "go test -count=1 -timeout=15m -v ./internal/floci/"; got != want {
		t.Errorf("ci floci-test run = %q, want %q", got, want)
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/workflowspec/ -count=1 -run 'TestCITestsEveryModule|TestCIRenderSmokeCoversEveryFixture|TestCIFlociJob'`
Expected: `FAIL` on exactly three tests (as seen while planning):
- `TestCITestsEveryModule`: `ci modules matrix = [github/component github/group], want every module: [aws/baseline aws/component github/component github/group]` and `ci modules: no step with id "tofu-test"`;
- `TestCIRenderSmokeCoversEveryFixture`: `render-smoke lists the fixtures 2 times, want once: [[basic aws-registry] [basic aws-registry]]`;
- `TestCIFlociJob`: `.github/workflows/ci.yaml: no job "floci"`.

- [ ] **Step 4: The modules job** — `.github/workflows/ci.yaml` (Edit), old:

```yaml
        module: [github/group, github/component]
```

new:

```yaml
        module: [github/group, github/component, aws/baseline, aws/component]
```

and old:

```yaml
      - name: tofu fmt and test
        working-directory: modules/${{ matrix.module }}
        run: |
          tofu fmt -check -recursive
          cp "$GITHUB_WORKSPACE/lockfiles/github.terraform.lock.hcl" .terraform.lock.hcl
          tofu init -input=false -lockfile=readonly
          tofu test
```

new:

```yaml
      - name: tofu fmt and test
        id: tofu-test
        working-directory: modules/${{ matrix.module }}
        # modules/<provider>/<name> takes the lock file lockfiles/<provider>.terraform.lock.hcl.
        env:
          MODULE: ${{ matrix.module }}
        run: |
          tofu fmt -check -recursive
          cp "$GITHUB_WORKSPACE/lockfiles/${MODULE%%/*}.terraform.lock.hcl" .terraform.lock.hcl
          tofu init -input=false -lockfile=readonly
          tofu test
```

- [ ] **Step 5: `render-smoke` in one loop** (carried item (b), D-15) — `.github/workflows/ci.yaml` (Edit), old:

```yaml
      - name: render the fixtures with local module sources
        # basic has no aws part; aws-registry passes each environment's variables.
        run: |
          for fixture in basic aws-registry; do
            go run ./cmd/idp render --dir "internal/render/testdata/$fixture" --out "$RUNNER_TEMP/rendered/$fixture" --modules-dir modules
          done
      - name: tofu validate the rendered stacks (proves the JSON syntax, lock file and modules)
        working-directory: ${{ runner.temp }}/rendered
        env:
```

new:

```yaml
      - name: render each fixture with local module sources and tofu validate it (proves the JSON syntax, lock file and modules)
        id: render-validate
        # basic has no aws part; aws-registry passes each environment's variables.
        env:
```

and old:

```yaml
        run: |
          for fixture in basic aws-registry; do
            tofu -chdir="$fixture/github" init -backend=false -input=false
            tofu -chdir="$fixture/github" validate
          done
```

new:

```yaml
        run: |
          for fixture in basic aws-registry; do
            go run ./cmd/idp render --dir "internal/render/testdata/$fixture" --out "$RUNNER_TEMP/rendered/$fixture" --modules-dir modules
            tofu -chdir="$RUNNER_TEMP/rendered/$fixture/github" init -backend=false -input=false
            tofu -chdir="$RUNNER_TEMP/rendered/$fixture/github" validate
          done
```

The step now runs from the repository root, so `-chdir` takes the full path. `TF_ENCRYPTION` is now set during the render too, which reads no environment.

- [ ] **Step 6: The floci job** — `.github/workflows/ci.yaml` (Edit), old:

```yaml
  release-snapshot:
    runs-on: ubuntu-24.04
```

new:

```yaml
  floci:
    # Level 3 (spec §8.4): the AWS modules applied to two accounts of one floci,
    # checked against internal/naming, replayed and destroyed by internal/floci.
    # floci reaches the Docker socket because its ECR runs a registry container
    # (ADR-0004).
    runs-on: ubuntu-24.04
    timeout-minutes: 20
    permissions:
      contents: read
    services:
      floci:
        image: floci/floci:2.4.0@sha256:5ee85db604c4aa74ac76e07ea5cc83086f37bb472bf3dbb97e34019e48efae9c
        ports:
          - "4566:4566"
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - uses: opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4 # v2.0.2
        with:
          tofu_version: 1.13.2
          tofu_wrapper: false
      - name: wait until floci is ready
        id: floci-ready
        # The same check as the image's health probe (docker/healthcheck.sh); it
        # fails, never continues, when floci does not answer.
        run: |
          for _ in $(seq 1 60); do
            if [[ "$(curl -fsS http://localhost:4566/_floci/init 2>/dev/null)" == *'"ready":true'* ]]; then
              exit 0
            fi
            sleep 1
          done
          echo "::error::floci did not report ready on localhost:4566 within 60 seconds"
          exit 1
      - name: apply, check, replay and destroy the AWS modules on floci
        id: floci-test
        env:
          IDP_FLOCI_ENDPOINT: http://localhost:4566
        run: go test -count=1 -timeout=15m -v ./internal/floci/
      - name: floci logs
        if: failure()
        env:
          FLOCI_CONTAINER: ${{ job.services.floci.id }}
        run: docker logs "$FLOCI_CONTAINER" 2>&1 | tail -n 300

  release-snapshot:
    runs-on: ubuntu-24.04
```

The job keeps the common rules that `TestWorkflowsDenyPermissionsByDefault`, `TestRemoteUsesArePinned`, `TestCheckoutNeverPersistsCredentials`, `TestRunScriptsTakeValuesFromEnv` (the container ID goes through `env`) and `TestWorkflowsUseNoAnchors` check.

- [ ] **Step 7: Run them to see them pass**

Run: `go test ./internal/workflowspec/ -count=1 -v -run 'TestCI|TestModulesPin|TestWorkflows|TestRemoteUses|TestCheckout|TestRunScripts|TestTokens'`
Expected: `PASS` for every test listed, `TestCITestsEveryModule`, `TestCIRenderSmokeCoversEveryFixture` and `TestCIFlociJob` among them. Then `go test ./internal/workflowspec/ -count=1` prints `ok`.

- [ ] **Step 8: The runbook's floci rows** — `docs/runbooks/dependencies.md` (exact replacements with Edit)

1. Old:

```markdown
| actionlint, zizmor | `ci.yaml` | one each |
```

new:

```markdown
| actionlint, zizmor | `ci.yaml` | one each |
| floci | the `floci` service image of `ci.yaml`, tag and digest | one |
```

2. Old:

```markdown
## An OpenTofu update
```

new:

```markdown
## A floci update

Renovate moves the image's tag and digest together, and the `floci` job is the check. Read floci's release notes for IAM and ECR changes before merging: the floci tests compare names, ARNs and policies, never the registry URL, whose host floci chooses (ADR-0013).

## An OpenTofu update
```

- [ ] **Step 9: Verify and commit** — `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing); the naming check prints nothing. actionlint, zizmor and the jobs run only in CI (Task 10 Step 3).

```bash
git add .github/workflows/ci.yaml internal/workflowspec/load_test.go internal/workflowspec/ci_test.go docs/runbooks/dependencies.md
git commit -m "ci: test the aws modules with their lock file and run the floci job"
```

- [ ] **Step 10: Mutation check** (nothing is committed) — in `.github/workflows/ci.yaml`, job `floci`, step `floci-ready`, delete the last line `          exit 1`. Run `go test ./internal/workflowspec/ -count=1 -run TestCIFlociJob`. Expected: `FAIL` with `ci floci-ready: the script lacks the consecutive lines`. Restore with `git checkout .github/workflows/ci.yaml`; `git status --short` prints nothing.

---

### Task 9: Carried item (a) — the shadowed loop variable

**Files:**
- Modify: `internal/render/render_test.go`

**Interfaces:**
- Consumes: `TestRenderIsDeterministic`. Produces: nothing new; behavior unchanged (D-15).

- [ ] **Step 1: Rename** — `internal/render/render_test.go` (Edit), old:

```go
		for i := range shuffled.Components {
			envs := slices.Clone(shuffled.Components[i].Environments)
			r.Shuffle(len(envs), func(a, b int) { envs[a], envs[b] = envs[b], envs[a] })
			shuffled.Components[i].Environments = envs
		}
```

new:

```go
		for c := range shuffled.Components {
			envs := slices.Clone(shuffled.Components[c].Environments)
			r.Shuffle(len(envs), func(a, b int) { envs[a], envs[b] = envs[b], envs[a] })
			shuffled.Components[c].Environments = envs
		}
```

- [ ] **Step 2: Verify and commit** — `go test ./internal/render/ -count=1 -v -run TestRenderIsDeterministic` passes, and `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing). A refactor of a test: it passes before and after, and the iteration message keeps naming the outer `i`.

```bash
git add internal/render/render_test.go
git commit -m "test(render): stop shadowing the iteration variable in the determinism test"
```

---

### Task 10: Ship — verification, PR, review, merge, phase log, README

Controller task (tests are delegated to a subagent).

- [ ] **Step 1: Local verification**
  - `go test ./... && go vet ./... && gofmt -l .` pass (gofmt prints nothing).
  - The module test snippet ends with `Success! 2 passed, 0 failed.` (`aws/baseline`), `Success! 8 passed, 0 failed.` (`aws/component`), `Success! 5 passed, 0 failed.` (`github/group`) and `Success! 12 passed, 0 failed.` (`github/component`).
  - The naming check over the branch's changes prints nothing.
  - `git diff --stat main..HEAD` lists exactly the 31 files of the File Structure (Tasks 1 to 9).
  - `git diff main..HEAD -- internal/render/render.go internal/render/testdata modules/github lockfiles/github.terraform.lock.hcl` prints nothing: no render, GitHub module or GitHub lock change (D-17).

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feat/phase-2c-aws-modules
gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-2c-aws-modules --title "feat: phase 2c aws modules and floci tests" --body "Implements docs/superpowers/plans/2026-11-17-phase-2c-aws-modules.md (spec amendment A12): modules/aws/baseline (the account's GitHub OIDC provider) and modules/aws/component (the ECR repository and the CI role, push-only, trusted only by its repository's CI in its environment), the hashicorp/aws 6.71.0 lock file, and the floci CI job, which applies both modules to two accounts with internal/naming's inputs, checks identities and policies, replays and destroys them. Claims validation gives each environment its own AWS account in the standard partition. Also closes the Phase 2b review items. No render change; no release: 0.2.0 ships in Phase 2f."
```

- [ ] **Step 3: CI** — `gh pr checks <number> --repo jellalshadows-idp/idp-engine --watch`.
  - Expected: `go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `modules (aws/baseline)`, `modules (aws/component)`, `render-smoke`, `floci` and `release-snapshot` pass.
  - `floci`'s log shows the four offline tests passing and `--- PASS: TestFloci`, with the subtests `dev`, `pro` and `accounts are isolated`, and two `floci's repository URI for api:` lines.
  - Record the watch-list items from the log: the job's and the readiness step's durations, the URI form, and the service initialization.
  - A failure goes through the fix loop: fix with a test, scoped review, push. A floci fidelity gap found here is documented and handled as spec §11 risk 4 says, with the owner's ruling, never by loosening an assertion silently.

- [ ] **Step 4: Final whole-branch review** (most capable model). The package: `main..HEAD`, the spec (§4.4, §4.7, §5.4, §5.5, §5.6, §6.3, §8.3, §8.4, §11), ADR-0004, ADR-0006, ADR-0013 and this plan. One fix wave, one scoped re-review, rulings in the ledger, and CI green again after the wave.

- [ ] **Step 5: Merge** — confirm the PR's latest CI is green, then `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`.

- [ ] **Step 6: The release PR and the harness** (watch list) — the merge's push runs `release.yaml`.
  - Read the release PR with `gh pr list --repo jellalshadows-idp/idp-engine --head release-please--branches--main --json number,title --jq '.[] | "\(.number) \(.title)"'`. Expected: one line, `28 chore(main): release 0.2.0`. Record it and leave the PR open (D-20).
  - Record the harness run that `e2e-release-pr` dispatched (`gh run list --repo jellalshadows-idp/idp-engine --workflow e2e.yaml --limit 1 --json databaseId,status,conclusion`) and, when it ends, its conclusion. Expected: `success`. A failure is a regression to fix before 2d.

- [ ] **Step 7: Write `docs/phases/phase-2c.md`** in the style of `docs/phases/phase-2b.md`. Include:
  - the header list: period, plan, the spec amended by A12, ADR-0013 amended, the previous phase (the Phase 2b log), and the status with the PR and its merge commit;
  - the summary, and the tasks with their commits and reviews;
  - every ruling (D-1 to D-20 and the execution rulings) with what, why and the cost if wrong, including the planning findings: the lock file's hashes do not depend on the platform flags (D-6), floci 2.4.0 is partition-aware (D-13), floci 2.4.0's entrypoint handles the socket group (D-10), and the brief's custom-manager expectation (D-14);
  - the Review Focus table with each result, and the watch list with each result: the CI checks; the floci job's duration and URI form; the release PR's number and version; the harness run; the Renovate observations, or "not yet run";
  - the notable findings caught by review, and the final disposition of every finding. The three items Phase 2b carried are all **Fixed**: the Opus Minor in Task 9, R2-ci-fixture-list-duplicated in Task 8, and R3-partition-silent in Task 2 (D-13).

  Then "Carried into later sub-phases":
  - **2d:**
    - render `aws/<env>/_baseline` and `aws/<env>/components/<name>` with these modules' inputs (`github_repository` = `<org>/<name>`);
    - the AWS stacks' backend and encryption;
    - `render.AWSProviderVersion` with its Renovate regex manager and golden files (D-7);
    - moving `TestFloci` from `writeStack` to the real render;
    - the nested-stack discovery items of `docs/phases/phase-1b.md`;
    - any Renovate observation still pending.
  - **2e:** the workflows write `floci_override.tf.json` and start floci per job (the `floci` job's service block is the model), and replay from `wet`; the floci job's measured time feeds spec §11 risk 3.
  - **2f:** as Phase 2b's log lists.
  - **Phase 4:**
    - `container-ci` must push unique tags (IMMUTABLE, D-4);
    - on real AWS it may need `ecr:BatchGetImage` (D-3);
    - repository-level `scan_on_push` versus AWS's registry-level scanning, if the project ever goes live.
  - **Accepted:** floci's backing `registry:2` is not pinned by this repository (D-11).

  Then "Owner notes": release PR #28 stays open until 2f, and every push to `main` re-runs the harness on it; the earlier owner items stay as recorded. Close with the process notes (inline or delegated execution, TDD evidence, the mutation checks, local OpenTofu 1.12.6 against CI's 1.13.2) and the status.

- [ ] **Step 8: Update `README.md`** (exact replacements with Edit)

1. Old:

```markdown
> **Status:** Phase 2b is complete: a Component with `aws.registry` renders its environment variables (`AWS_ROLE_ARN`, `ECR_REPOSITORY`, `AWS_REGION`), computed by `internal/naming`, and `modules/github/component` creates them; the AWS resources they name are not rendered yet. See the [phase 2b log](docs/phases/phase-2b.md) and [docs/claims.md](docs/claims.md). Phase 2c (the AWS modules and their floci tests) is next.
```

new:

```markdown
> **Status:** Phase 2c is complete: `modules/aws/baseline` (an environment account's GitHub OIDC provider) and `modules/aws/component` (a Component's ECR repository and its push-only CI role) exist, and CI applies them to two accounts of the floci emulator on every PR, checking their identities against `internal/naming`. Each environment needs its own AWS account in the standard partition. No render calls the modules yet. See the [phase 2c log](docs/phases/phase-2c.md) and [docs/claims.md](docs/claims.md). Phase 2d (rendering the AWS stacks) is next.
```

2. Old:

```markdown
- The pipelines reconcile **GitHub only** (teams, repositories, rulesets, environments and, since Phase 2b, the environment variables of a Component with `aws.registry`). The ECR repository and the CI role those variables name are rendered from Phase 2d and applied from Phase 2e.
```

new:

```markdown
- The pipelines reconcile **GitHub only** (teams, repositories, rulesets, environments and, since Phase 2b, the environment variables of a Component with `aws.registry`). The AWS modules that create the ECR repository and the CI role exist since Phase 2c and are tested against floci in CI; they are rendered from Phase 2d and applied from Phase 2e.
```

- [ ] **Step 9: Commit doc-only to `main`** (side effect 2) — the naming check prints nothing; then:

```bash
git add docs/phases/phase-2c.md README.md
git commit -m "docs: add the phase 2c execution log"
git push
```

The push runs `release.yaml`, which refreshes the open release PR and re-runs the harness on it. Leave the PR open.

- [ ] **Step 10: Delete the SDD workspace** (side effect 3) — after checking that the phase log reproduces the ledger: `rm -rf .superpowers/sdd`, then `git status --short` prints nothing.
