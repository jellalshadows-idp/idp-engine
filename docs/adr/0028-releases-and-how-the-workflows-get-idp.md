# 0028. Releases, and how the workflows get idp

- Status: Accepted
- Date: 2026-10-18
- Spec: [§7.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#74-rulesets), [§7.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#75-supply-chain), [§8.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#85-level-4-e2e), [§9.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#91-releases)

## Context

Spec §9.1 releases `idp-engine` with release-please, publishes in the same workflow gated on `release_created`, uploads the CLI and its checksums with GoReleaser and attests the build provenance. Spec §7.5 has the workflows verify that attestation before they run the binary. Until the first release, every reusable job built `idp` from the reusable workflow's own commit (ADR-0021).

Callers pin the reusable workflows to a commit SHA, so a job knows `job.workflow_sha`, never a tag. The binary's version is what rendered module sources pin (spec §5.3), and GoReleaser's `{{.Version}}` drops the `v` that the engine's tags carry. Pull requests opened with a workflow's `GITHUB_TOKEN` trigger no workflows, and the org setting that lets Actions open them stays off (spec §9.2). The end-to-end harness must run on release PRs (spec §8.5), but its environment `idp-e2e` is limited to `main` (ADR-0027).

## Decision

- **Release PRs.** `.github/workflows/release.yaml` runs on every push to `main` and on dispatch, one run at a time, with no run dropped (`queue: max`). Its `release-please` job runs in environment `idp-release` (deployment branch `main`) and mints a bot token (ADR-0026) for `idp-engine` only, with contents, issues and pull requests write, so the release PR triggers CI. One package covers the repo: release type `go`, tags `vX.Y.Z`, breaking changes bump the minor while the version is `0.x`, the first version is `0.1.0`, and every push to `main` refreshes the release PR.
- **The harness on release PRs.** When release-please creates or updates the release PR, the `e2e-release-pr` job dispatches `e2e.yaml` on `main`, the release PR's base (the same code without the changelog), with the PR's head commit. The harness run posts the commit status `idp-e2e` on that commit: `pending` when it starts, then `success` or `failure`, also when it fails early. A release PR is merged only when `idp-e2e` is green. `idp-e2e` stays limited to `main`.
- **Publishing**, in the same run and only when release-please created a release:
  1. refuse a tag that is not `vX.Y.Z`, and a release whose commit is not the run's commit (the attestation names the run's commit);
  2. build with GoReleaser: `./cmd/idp` only, `CGO_ENABLED=0`, `-trimpath`, the version ldflag set to `{{.Tag}}` so the `v` survives into `?ref=`, for linux, darwin and windows on amd64 and arm64;
  3. upload the six archives and `checksums.txt` to the release with the workflow's own token, keeping release-please's notes;
  4. check that the linux/amd64 binary prints the tag with `idp version`;
  5. attest the provenance of the six archives and `checksums.txt` with `actions/attest-build-provenance`.
- **How a job gets idp.** `setup-idp` keeps its two inputs. It lists the engine's tags and keeps the lightweight `vX.Y.Z` tags that point at the pinned commit.
  - **No tag:** Go from the engine's `go.mod` and a source build at the commit (ADR-0021). Module sources pin the commit.
  - **One tag:** download the runner's linux archive and `checksums.txt` from that release, check the checksum, run `gh attestation verify` (signer workflow `release.yaml`, source ref `refs/heads/main`, source digest the pinned commit), require `idp version` to print the tag, and install. Module sources pin the tag.
  - **Anything else fails the job:** a failed tag listing, several tags, or any failure after a tag was found. A tagged commit never falls back to a build.
- **Tags never move** (spec §7.4). The repository ruleset `idp-engine-tags` forbids deleting, force-moving or updating `refs/tags/v*`, with no bypass. Creating a tag stays allowed, so release-please can tag.
- `idp version` prints the build's version: the tag of a release binary, the commit of a source build, `dev` for a local build.

## Consequences

### Positive

- A released pipeline runs only a binary whose provenance ties it to the pinned commit, and spends no Go build.
- Rendered module sources pin a tag that never moves.
- Any commit can still be pinned: the harness's commit under test and unreleased pins build from source.
- Every release is proven by the harness against the real org before it merges.

### Negative / costs

- `setup-idp`'s first step calls the GitHub API; when the tag listing fails, the job fails instead of guessing.
- A broken release (missing assets, a failed attestation) fails every pipeline pinned to it until its publish job is re-run or the callers pin another release. Between the tag and the end of the publish job, a few minutes, a pin of the release commit fails the same way.
- An annotated tag is not recognized as a release. release-please creates lightweight tags.
- The harness tests `main`, and its status sits on the release PR's head. A waiting harness run can be replaced by a newer one in the `idp-e2e` concurrency group; the release PR then shows no `idp-e2e` status until someone dispatches the harness again ([releases runbook](../runbooks/releases.md)).
- "Merge only with `idp-e2e` green" is a documented rule, not an enforced one: `idp-engine` has no branch ruleset.
- One harness run, about 20 minutes, per push to `main` while a release PR is open.

## Alternatives considered

- **Fall back to a source build for a tagged commit.** Rejected: the render would pin the commit instead of the tag, and a broken release would go unnoticed.
- **GitHub immutable releases.** Rejected for now: once a release is published its assets cannot change, and release-please publishes the release before GoReleaser uploads to it. Cost: someone with write access can replace a published asset; the attestation check then fails every pipeline pinned to that release instead of running the replacement.
- **Pass the binary between jobs as an artifact.** Rejected again (ADR-0021): a trust hop to save under a minute per job.
- **Stamp the version into the workflow files at release time** (release-please extra files). Rejected: the tagged commit would differ from the merged release PR, and every release would need a second commit.
- **A `pull_request` trigger for the harness on release PRs.** Rejected: `idp-e2e`, which holds the bot key and the approver token, is limited to `main`, and widening it would hand those secrets to a branch.
- **A personal access token to publish.** Ruled out by spec §9.1.
