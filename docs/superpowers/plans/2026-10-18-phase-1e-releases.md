# Phase 1e — Releases, Renovate, `v0.1.0` and Onboarding `idp-claims` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Release `idp-engine` (release-please, GoReleaser, build provenance), make the reusable workflows install the verified release binary of a release commit, run the end-to-end harness on release PRs, keep every pin current with self-hosted Renovate, ship `v0.1.0`, re-pin the caller templates to it, and onboard the production `idp-claims`.

**Architecture:**
- **One release workflow.** `.github/workflows/release.yaml` runs release-please with a bot App token (environment `idp-release`); when release-please creates a release, the same run builds `./cmd/idp` with GoReleaser, uploads six archives and `checksums.txt` into that release with the workflow token, and attests their provenance. When release-please creates or updates the release PR, the run dispatches the harness on `main`, which reports the commit status `idp-e2e` on the PR head.
- **Download, verify, else build.** `setup-idp` keeps its two inputs. It looks up a `vX.Y.Z` tag on the pinned commit: none means the source build of ADR-0021; one means download, checksum, `gh attestation verify`, `idp version` equals the tag, install; anything else fails. A tagged commit never falls back to a build.
- **Renovate** runs weekly from `idp-engine` (environment `idp-renovate`) over `idp-engine` and `idp-claims`, with one global config, no automerge, 14 days for third-party releases and no wait for the engine's own.

**Tech Stack:** Go 1.26 (standard library plus the existing `go.yaml.in/yaml/v3`; no new dependencies), GitHub Actions, release-please 17.6.0 (through `release-please-action` v5.0.0), GoReleaser v2.18.2, `actions/attest-build-provenance` v4.2.2, `gh attestation verify`, Renovate 44.133.0, OpenTofu 1.12.6.

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §3.1, §5.3, §6, §7.3–§7.5, §8.5, §9.1 and §10. Context: `docs/phases/phase-1d.md` ("Carried into Phase 1e", "Owner notes"), ADR-0021, ADR-0026, ADR-0027.

## Decisions taken while planning

The executor copies these into the ledger as rulings before Task 1. D-1 to D-4 are the owner's (2026-10-18); D-5 and D-6 are the owner's Phase 1d decisions this plan carries out; D-7 to D-16 are the controller's design rulings; D-17 onwards were decided while planning, from verified facts.

**Owner decisions**

- **D-1 (owner). The harness runs on release PRs through a dispatch.** When release-please opens or updates the release PR, `release.yaml` dispatches `e2e.yaml` on `main` (the release PR's base: the same code without the changelog) with `GITHUB_TOKEN` and `actions: write`. `e2e.yaml` gets an optional input `release-pr-sha` (40 hex, validated); when it is set, the run posts the commit status `idp-e2e` on that commit with `GITHUB_TOKEN` and `statuses: write`: `pending` at the start, then `success` or `failure`, even on failure. A release PR is merged only with `idp-e2e` green. `idp-e2e` stays limited to `main`; the triggers stay `schedule` and `workflow_dispatch`. *Cost if wrong:* the harness tests `main` while the merge adds the changelog commit; the changelog cannot break the pipelines.
- **D-2 (owner). No automerge in Phase 1e.** The owner merges every Renovate PR in `idp-engine` and `idp-claims`. *Cost if wrong:* the owner's time.
- **D-3 (owner). Renovate runs weekly over `idp-engine` and `idp-claims`,** not `idp-claims-e2e`, whose callers the harness pins on every run. *Cost if wrong:* a week of delay; a dispatch runs it at once.
- **D-4 (owner). The 14-day `minimumReleaseAge` applies to third-party dependencies only.** The engine's own releases (the `jellalshadows-idp/idp-engine` refs in `idp-claims` and in the templates) have no minimum age. *Cost if wrong:* a broken engine release reaches `idp-claims` sooner; the harness ran on its release PR and the owner merges by hand.
- **D-5 (owner, Phase 1d D-3). Renovate is self-hosted with the bot App.**
- **D-6 (owner, Phase 1d D-4). The workflows download and verify the release binary, building from source only when the pinned commit has no release.**

**Controller rulings**

- **D-7 (R1). How a job gets `idp`.** `setup-idp` keeps exactly its two inputs and the job-level contract of `TestJobsBuildIdpFromTheWorkflowCommit`. A `download` step runs first; `go` and `build` run only when it did not install. The lookup lists `repos/jellalshadows-idp/idp-engine/git/matching-refs/tags/v` and keeps refs matching `^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$` whose `object.type` is `commit` and whose `object.sha` is the pinned commit. API error: fail. No tag: `installed=false`, source build. Several tags: fail. One tag: download the runner's linux archive (`X64`→`amd64`, `ARM64`→`arm64`, else fail) and `checksums.txt`, `sha256sum --check --ignore-missing` with the archive's line required, `gh attestation verify --repo jellalshadows-idp/idp-engine --signer-workflow jellalshadows-idp/idp-engine/.github/workflows/release.yaml --source-digest <commit> --source-ref refs/heads/main`, extract, require `idp version` to print the tag, install. **No fallback for a tagged commit:** it would change the rendered `?ref=` from the tag to the SHA and hide a broken release. *Cost if wrong:* a broken release blocks its pins until it is republished.
- **D-8 (R2). `idp version`** prints `version.Version` and a newline and exits 0; an argument is a usage error (ADR-0016). *Cost:* none.
- **D-9 (R3). GoReleaser and the `{{.Tag}}` guard.** `.goreleaser.yaml` (version 2) builds only `./cmd/idp` as `idp`, `CGO_ENABLED=0`, `-trimpath`, ldflags exactly `-s -w -X github.com/jellalshadows-idp/idp-engine/internal/version.Version={{.Tag}}`, for linux, darwin and windows on amd64 and arm64; archives `idp_{{ .Version }}_{{ .Os }}_{{ .Arch }}`, tar.gz, zip on windows; `checksums.txt`; `changelog.disable: true`; `release.mode: keep-existing`. The guard has two layers: a contract test on the config, and the publish job refusing a non-`vX.Y.Z` tag and checking the built binary's `idp version`. CI proves the config on every PR (`goreleaser check` and a snapshot, job `release-snapshot`). GoReleaser never runs locally. *Cost if wrong:* one more CI job of about two minutes.
- **D-10 (R4). `release.yaml` and release-please.** One package `.`, `release-type: go`, `bump-minor-pre-major`, `include-component-in-tag: false`, `include-v-in-tag: true`, changelog sections feat, fix, perf, refactor, docs, revert, with chore and test hidden; manifest `{".": "0.0.0"}`. Jobs `release-please` (environment `idp-release`, `permissions: {}`, bot token for `idp-engine` with contents, issues and pull requests write), `publish` (gated on `release_created`; contents write, id-token write, attestations write) and `e2e-release-pr` (dispatches the harness). Release PRs come from the bot token, so CI runs on them; they merge with a merge commit. *Cost if wrong:* release-please is replaceable without touching the callers.
- **D-11 (R5). Two environments, `idp-release` and `idp-renovate`,** deployment branch `main`, each with the secret `IDP_BOT_PRIVATE_KEY` (from `~/.idp/apps/jellalshadows-idp-bot.pem`, never printed) and the variable `IDP_BOT_CLIENT_ID`. The controller creates them in Task 9; the bootstrap runbook's bot key rotation covers all three environments; ADR-0026 and spec §7.3 name them. *Cost:* one more place per rotation.
- **D-12 (R6). Renovate.** Workflow `renovate.yaml` (cron `17 4 * * 1` and dispatch, environment `idp-renovate`, bot token for `idp-engine,idp-claims` with contents, pull requests, workflows and issues write, checks and statuses read), `renovatebot/github-action` pinned by SHA with `renovate-version` pinned exactly; one global config with `platform: github`, `onboarding: false`, `requireConfig: "ignored"`, `platformCommit: "enabled"`, the bot's `[bot]` git author, the dashboard, `automerge: false`, `minimumReleaseAge: "14 days"`, `internalChecksFilter: "strict"`; groups "idp engine" (no wait), the OpenTofu pins, and the provider pins labelled `needs-lockfile`; managers github-actions, gomod (`gomodTidy`, the `go` directive's patch), and regex managers; the lock file is regenerated by hand (`docs/runbooks/dependencies.md`, with the `-platform=` flags). CI validates the config with the pinned Renovate. *Cost if wrong:* config only.
- **D-13 (R7). Tags never move.** The controller creates the repository ruleset `idp-engine-tags` (target tag, `refs/tags/v*`, rules deletion, non_fast_forward, update, no bypass) in Task 9. Creating a tag stays allowed. **GitHub immutable releases stay off:** they forbid adding assets to a published release, which the two-step publish (release-please publishes, GoReleaser uploads) needs. *Cost:* someone with write access could replace a published asset; the attestation check then fails every pipeline pinned to it instead of running the replacement.
- **D-14 (R8). Templates re-pin after the release.** The release commit's SHA is unknown inside the release, so a separate PR re-pins the four templates to `v0.1.0`'s commit with `# v0.1.0` and drops the `# unreleased` exception from `checkUses`. The harness meanwhile rewrites `@<sha> # <comment>` to `@<engineSHA> # unreleased`, because it always pins the commit under test. *Cost:* one PR.
- **D-15 (R9). Onboarding `idp-claims`** by a PR the controller opens: the four callers copied from the re-pinned templates and `config/platform.yaml` with production values, no claims. `IDP_BOOTSTRAP` is already in the new format; `bootstrap check` runs first. *Cost:* production starts reconciling; it has no claims yet.
- **D-16 (R10). Docs and amendments** fold into the task whose change needs them; ADR-0028 and ADR-0029; spec amendment A9; ADR-0018, ADR-0021, ADR-0026 and ADR-0027 get amended sentences.

**Decided while planning (verified)**

- **D-17. `initial-version: "0.1.0"` in `release-please-config.json`.** The brief expected the manifest `0.0.0` plus `bump-minor-pre-major` to give `0.1.0`. It does not: release-please 17.6.0 (the version `release-please-action` v5.0.0 bundles) skips a manifest version of `0.0.0` when it backfills the latest release (`src/manifest.ts`, "backfill latest release tags from manifest"), and with no latest release the version is `initialReleaseVersion()`, which is `initial-version` or `1.0.0` (`src/strategies/base.ts`). Task 10 still checks the version and keeps the `Release-As: 0.1.0` fallback. *Cost if wrong:* none; the fallback remains.
- **D-18. `always-update: true` (top level).** Without it release-please leaves the release PR untouched when its body does not change (`maybeUpdateExistingPullRequest`), so a push of only hidden types (`chore`, `test`, Renovate's `chore(deps)`) would leave a green `idp-e2e` on a PR whose merge carries untested code. With it, every push to `main` rebuilds the release PR on the new `main` and re-dispatches the harness (`prs_created` is true only for a created or updated PR). *Cost:* one harness run (about 20 minutes) per push to `main` while a release PR is open.
- **D-19. `release.yaml` concurrency adds `queue: max`.** GitHub keeps one waiting run per group and cancels the older waiting one; if the run for the release PR's merge were dropped, a later push's run would create the release for a commit that is not its own, and the publish guard (D-20) would refuse it. `queue: max` is the documented key the claims-repo templates already use (spec §6.2); `.github/actionlint.yaml` ignores actionlint v1.7.12's "unexpected key queue" message for `.github/workflows/`. *Cost if wrong:* one ignore line.
- **D-20. The publish guard also requires the release's commit to be the run's commit** (`needs.release-please.outputs.sha == GITHUB_SHA`): the attestation's source digest is the run's commit, and `setup-idp` verifies it against the tagged commit. *Cost if wrong:* a release created by another run must be cut again as the next patch (releases runbook).
- **D-21. `release-please-action` v5.0.0 output names** (verified in `src/index.ts` at the tag): for the root path the outputs are unprefixed `release_created`, `tag_name`, `sha`; `prs_created` is true when a release PR was created or updated; `pr` is the first PR as JSON, without a reliable head SHA (`PullRequest.sha` is optional), so `e2e-release-pr` reads `headRefOid` with `gh pr view` and gets `pull-requests: read` besides `actions: write`. *Cost if wrong:* a live fix in Task 10.
- **D-22. Renovate writes no commit statuses** (`statusCheckNames`: all four `null`). The bot App has `statuses: read`, and Renovate 44's GitHub platform aborts the repository's run when a status write fails (`setBranchStatus` throws `REPOSITORY_CHANGED`); `renovate/stability-days` is written on every branch when `minimumReleaseAge` is set. With `internalChecksFilter: strict` the dashboard lists pending updates instead. Changing the App's permissions is an owner browser step this plan avoids. *Cost:* no Renovate branch statuses; artifact errors still appear as PR comments.
- **D-23. The Renovate config is strict JSON at `.github/renovate-global.json`** (the brief said `.github/renovate.json5`): the Go contract test decodes it with `encoding/json` and `DisallowUnknownFields`, with no new dependency, and rule `description` fields replace comments. The name is outside Renovate's repository-config file list, so nobody mistakes it for `idp-engine`'s own config (`requireConfig: "ignored"` ignores those anyway). *Cost if wrong:* a rename.
- **D-24. CI validates the config with the Renovate image** (`docker run ghcr.io/renovatebot/renovate:44.133.0 renovate-config-validator --strict`), not `npx`: Renovate 44.133.0 requires Node `^24.11.0` (`package.json` engines) and the `ubuntu-24.04` image ships Node 22.23.3 (image 20261012). The run and the validation then use one Renovate version, bumped by one regex manager. With file arguments the validator validates them as global config and, with `--strict`, fails on warnings and needed migrations (`lib/config-validator.ts`). *Cost:* a docker pull in CI.
- **D-25. Native Renovate managers where they exist; regex managers only where none does.** `opentofu/setup-opentofu`'s `tofu_version` and `goreleaser/goreleaser-action`'s `version` are known actions of the github-actions manager, which also reads composite actions; the provider pins come from the `terraform` manager (the brief listed a regex for them). Regex managers cover actionlint (`go`), zizmor (`pypi`), the Renovate version (`github-releases renovatebot/renovate`), and the renderer's pins: `render.TofuVersion` is rendered as an exact `required_version`, so an OpenTofu bump that misses it would make every rendered stack refuse the new `tofu`. A contract test keeps `render.TofuVersion` equal to CI's OpenTofu. *Cost if wrong:* config only.
- **D-26. `goreleaser check` fails CI on deprecated properties** (GoReleaser v2.18.2 `cmd/check.go` exits 2), so the config uses `formats`, not the deprecated `format`. *Cost:* none.
- **D-27. The publish job checks the version from the linux/amd64 archive**, the artifact callers download, rather than a build directory under `dist/`. *Cost:* none.
- **D-28. The `# unreleased` cases of `TestCheckUses` flip to failures** in Task 11 instead of being deleted, so the RED step is a real failing test; the "released" case stays. *Cost:* none.
- **D-29. Production `config/platform.yaml`** uses spec §4.6's values: `writerAppId: 5255579`, `archiveOnDestroy: true`, `requiredApprovals: 1`, `naming.reservedPrefixes: [e2e-, spike-]`, environments `dev`, `staging`, `pro` (fake account ids, `pro` protected), `modules.allowedSources`. The renderer ignores AWS until Phase 2. *Cost if wrong:* a config PR.
- **D-30. The release is checked locally with the released `windows/amd64` archive** (checksum, `gh attestation verify`, `idp version`), and that binary runs the `idp-claims` bootstrap check in Task 12: no local build. *Cost:* none.
- **D-31. Spec amendment A9 also covers §3.1 (the `idp-claims` row), §3.3 (`idp version`), §6 (how a job gets `idp`) and §7.4 (the tag ruleset),** besides the §7.3, §7.5, §8.5 and §10 items, because each states what Phase 1e changes; ADR-0027's "Release PRs join in Phase 1e" is amended too. *Cost:* none.
- **D-32. The `renovate` job has `contents: read`** (the brief said `permissions: {}`): it checks out the repo to read its config file. *Cost:* none.
- **D-33. Not in Phase 1e:** the optional narrower plan token stays an owner note (ADR-0025); reporting the provider's documentation and code mismatch upstream stays optional (Phase 5 disposition).

## Global Constraints

**Pinned actions and tools** (each released on or before 2026-10-04, so at least 14 days old when adopted, spec §7.5; tags dereferenced to commits on 2026-10-18):

| Action or tool | Version | Commit SHA | Released |
|---|---|---|---|
| `actions/checkout` | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` | 2026-07-20 |
| `actions/setup-go` | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` | 2026-07-16 |
| `actions/create-github-app-token` | v3.2.0 | `bcd2ba49218906704ab6c1aa796996da409d3eb1` | 2026-05-12 |
| `opentofu/setup-opentofu` | v2.0.2 | `a1320f892987e89d278cc92dc5adc984fb93aca4` | 2026-06-29 |
| `googleapis/release-please-action` | v5.0.0 (release-please 17.6.0) | `45996ed1f6d02564a971a2fa1b5860e934307cf7` | 2026-04-22 |
| `goreleaser/goreleaser-action` | v7.2.3 | `f06c13b6b1a9625abc9e6e439d9c05a8f2190e94` | 2026-06-29 |
| GoReleaser (the action's `version`) | v2.18.2 | tag object `6a0cc0c…` → commit `25a52e520f9c993711b93ea1111ed90c024e0528` | 2026-09-17 |
| `actions/attest-build-provenance` | v4.2.2 | `4d101475d8b20a2381f78447822ac1eab6504dd8` | 2026-08-06 |
| `renovatebot/github-action` | v46.3.6 | `6d26fcf0275dc65624cbc3d51fe78bc773f98321` | 2026-09-29 |
| Renovate (`renovate-version`, image `ghcr.io/renovatebot/renovate`) | 44.133.0 | `474cddd087adab35d555fd3fd3d2c8036a0eab55` | 2026-10-04 18:42 UTC (adopt no earlier than 2026-10-18 18:42 UTC) |
| actionlint, zizmor, OpenTofu | v1.7.12, 1.30.1, 1.12.6 | unchanged | — |

**Names** (copy verbatim)
- Branch `feat/phase-1e-releases`; one PR, merged with a merge commit. Task 11's branch `chore/pin-templates-v0.1.0`.
- Workflows: `.github/workflows/release.yaml` (`name: idp-release`, jobs `release-please`, `publish`, `e2e-release-pr`, concurrency group `idp-release` with `queue: max`); `.github/workflows/renovate.yaml` (`name: idp-renovate`, job `renovate`, concurrency group `idp-renovate`, cron `17 4 * * 1`).
- Environments of `idp-engine`: `idp-release` and `idp-renovate`, deployment branch `main`, secret `IDP_BOT_PRIVATE_KEY`, variable `IDP_BOT_CLIENT_ID` (value `Iv23linzv8qQJ5r6kWO7`, the `client_id` of `~/.idp/apps/jellalshadows-idp-bot.json`).
- Release bot token: `owner: jellalshadows-idp`, `repositories: idp-engine`, `permission-contents: write`, `permission-issues: write`, `permission-pull-requests: write`.
- Renovate bot token: `owner: jellalshadows-idp`, `repositories: idp-engine,idp-claims`, `permission-checks: read`, `permission-contents: write`, `permission-issues: write`, `permission-pull-requests: write`, `permission-statuses: read`, `permission-workflows: write`.
- `e2e.yaml` input `release-pr-sha`; commit status context `idp-e2e`.
- Release tags `vX.Y.Z` (lightweight, made by release-please); the first is `v0.1.0`. Assets: `idp_<X.Y.Z>_{darwin,linux}_{amd64,arm64}.tar.gz`, `idp_<X.Y.Z>_windows_{amd64,arm64}.zip`, `checksums.txt`. Release PR branch `release-please--branches--main`, label `autorelease: pending`.
- Tag ruleset `idp-engine-tags`.
- Renovate config `.github/renovate-global.json`; groups `idp engine`, `opentofu`, `github provider`; label `needs-lockfile`; git author `jellalshadows-idp-bot[bot] <343234638+jellalshadows-idp-bot[bot]@users.noreply.github.com>` (bot user id 343234638, read on 2026-10-18).
- ADRs `docs/adr/0028-releases-and-how-the-workflows-get-idp.md` and `docs/adr/0029-self-hosted-renovate.md`; spec amendment **A9**; new spec §7.3 anchor `#73-three-github-apps-and-their-secret-scopes`.
- Runbooks `docs/runbooks/releases.md`, `docs/runbooks/dependencies.md`; phase log `docs/phases/phase-1e.md`.

**Workflows**
- `permissions: {}` at the top and explicit per-job permissions; every remote `uses:` pinned by full SHA with a `# vX.Y.Z` comment; `persist-credentials: false` on every checkout; no `${{ }}` inside `run:`; tokens set only on the step that uses them.
- actionlint v1.7.12 and zizmor 1.30.1 clean. No new composite action is added, so zizmor's explicit path list (`.github/workflows .github/actions/setup-idp/action.yaml examples/claims-repo/.github/workflows/*.yaml`) stays as is.
- If zizmor flags a new finding, fix it, or carry an inline ignore with its reason on the flagged line (like the existing `# zizmor: ignore[...]` comments), and report the exact finding.

**Process**
- Strict TDD: failing test, run it to see it fail, implement, run it to see it pass, commit.
- No `go build` and no GoReleaser run locally. Use `go test ./...`, `go vet ./...`, `gofmt -l .` (must print nothing); `-race` runs in CI. Lint with `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` and `"$ZIZMOR" --offline …` (pre-flight).
- Shell rule: never use `cat`, `grep`, `find`, `sed` or `ls`, never write files with shell heredocs, never `perl`. Use the Read/Write/Edit tools, `rg` (with `--hidden` for `.github`), `gh`, `git`, `python -I`.
- Conventional commits, never with Co-Authored-By or any AI attribution.
- Docs in English. Owner rule: docs describe the platform on its own terms, never naming another product as inspiration or comparison, and never use the owner's work account; the controller gives each dispatch the naming check that enforces both (it must print nothing).
- Doc-only changes after the merge go straight to `main`; code goes through PRs.
- Never delete or overwrite `~/.idp/e2e.pass`, `~/.idp/main.pass` or `~/.idp/apps/*`.

**Spec rules**
- No personal access token anywhere (spec §9.1): release-please uses the bot App token, GoReleaser the workflow token.
- The org setting "Allow GitHub Actions to create and approve pull requests" stays off (spec §9.2, Phase 4).
- Publishing runs in the same workflow, gated on `release_created`, with no `workflow_run`.

**Owner actions:** none beyond approving this plan and the side effects below; every credential comes from files that already exist.

**Side effects this plan authorizes** (controller, after the owner approves the plan)
1. Push `feat/phase-1e-releases`, open its PR, merge it after CI, the final review and items 2–3.
2. In `idp-engine`: create the environments `idp-release` and `idp-renovate` with deployment branch policy `main`, set their secret `IDP_BOT_PRIVATE_KEY` from `~/.idp/apps/jellalshadows-idp-bot.pem` (never printed) and their variable `IDP_BOT_CLIENT_ID` from `~/.idp/apps/jellalshadows-idp-bot.json`.
3. Create the repository ruleset `idp-engine-tags` on `idp-engine`.
4. Merge the release PR, which tags and publishes `v0.1.0`; only if that PR proposes another version, push one empty commit with `Release-As: 0.1.0` to `main` first.
5. Dispatch `e2e.yaml` (a release PR whose dispatch was replaced; once on `main` after the release) and `renovate.yaml` (once, in Task 12).
6. Push `chore/pin-templates-v0.1.0`, open its PR, merge it after CI and its review.
7. In `idp-claims`: push the branch `chore/onboard-idp-pipelines`, open the onboarding PR, merge it after `idp-gate`; dispatch its `drift.yaml` once.
8. Doc-only commits straight to `main` (the phase log, the README, plan corrections).
9. Delete the local, git-ignored SDD workspace `.superpowers/sdd/` after the phase log is committed.

Not authorized, stop and ask: deleting or moving a tag, deleting a release or an asset, changing or disabling a ruleset after its creation, changing any App's permissions or installation, enabling the org setting above, merging a Renovate PR, merging any later release PR.

## Review Focus

1. **The tag lookup fails** (API error, rate limit).
   - **Expected:** `setup-idp` fails the job with an error; it never decides "unreleased" and never builds.
   - **Pinned by:** `TestSetupIDPFailsWhenTheTagListingFails` (Task 6).
2. **A tagged commit whose release lacks its archive or checksums.**
   - **Expected:** the job fails with a message that says a release commit never falls back to a build and points to the releases runbook.
   - **Pinned by:** `TestSetupIDPNeverBuildsATaggedCommit` (Task 6).
3. **A non-semver tag such as `v0.1.0-rc.1`.**
   - **Expected:** the publish guard refuses it before GoReleaser runs, and the lookup ignores it (so its commit builds from source).
   - **Pinned by:** `TestReleasePublishRefusesANonSemverTag` (Task 4), `TestSetupIDPIgnoresNonReleaseTags` (Task 6).
4. **The harness fails early (preflight) with `release-pr-sha` set.**
   - **Expected:** the `failure` status is still posted on the release PR head; a run that never posted `pending` (a re-run attempt) posts nothing.
   - **Pinned by:** `TestE2EReportsEveryOutcomeOnTheReleasePR` (Task 5).
5. **The four engine pins would be bumped separately.**
   - **Expected:** one Renovate PR ("idp engine") moves all four, with no waiting period.
   - **Pinned by:** `TestRenovateMovesTheEnginePinsTogether` (Task 8).

Also tested, outside the five: an arm64 runner picks the arm64 archive (`TestSetupIDPPicksTheRunnerArchive`, Task 6); the release-please job without its environment credentials names the missing one (`TestReleasePleaseNamesAMissingCredential`, Task 4).

## Watch list for the live steps

Prove live, and record each result in the ledger:
- The runner's `gh` supports `gh attestation verify --signer-workflow --source-digest --source-ref` (gh 2.89.0 locally has them; the `ubuntu-24.04` image 20261012 lists GitHub CLI 2.102.0). Seen in Task 10 Step 10 and Task 12.
- The caller job's `GITHUB_TOKEN` (`contents: read`, another repository) reads the public engine's tags, release assets and attestations. Seen in Task 10 Step 10 (`idp-claims-e2e`) and Task 12 (`idp-claims`). If the attestation API refuses that token, stop: the fix is a ruling (upload the attestation bundle as a release asset and verify with `--bundle`), a test change and the next patch release.
- The `release-please-action` v5.0.0 output names (D-21). Task 10 Steps 1 and 6.
- GoReleaser `keep-existing` uploads into the release release-please created and keeps its notes. Task 10 Step 7.
- The first release is `0.1.0` (D-17). Task 10 Step 2.
- GoReleaser's snapshot `{{.Tag}}` is `v0.0.0` in a repo without tags. Task 9 Step 3 (`release-snapshot`).
- The image `ghcr.io/renovatebot/renovate:44.133.0` exists and validates the config. Task 9 Step 3 (`workflows`).
- The harness run at the release commit: the pin PR plans "No infrastructure changes.", and the create step's module sources resolve `?ref=v0.1.0` (closes Phase 1a's Task 11 minor). Task 10 Step 10.
- `idp-gate` reports on the onboarding PR, and the first reconcile of a repo with no claims and no state file succeeds. Task 12.
- Renovate, with the bot's permissions (no statuses write, no administration, no Dependabot alerts), finishes both repositories and lists the engine's reusable-workflow pins (SHA with `# v0.1.0`) on the dashboard; updating such a pin is proven by the first Renovate PR after the next engine release. Task 12.
- `job.status` in the harness's `always()` step reports `failure` or `cancelled` after a failed step. Seen whenever a release-PR run fails; otherwise recorded as not observed.

## File Structure

```
docs/adr/0028-releases-and-how-the-workflows-get-idp.md, 0029-self-hosted-renovate.md, README.md   # Task 1
docs/adr/0009, 0018, 0021, 0023, 0025, 0026, 0027 (amendments, anchor)                               # Task 1
docs/superpowers/specs/2026-10-08-idp-on-actions-design.md                                          # Task 1 (A9)
internal/cli/version.go, version_test.go, cli.go; docs/cli.md                                        # Task 2
.goreleaser.yaml, .gitignore, .github/workflows/ci.yaml (release-snapshot)                           # Task 3
internal/workflowspec/goreleaser_test.go, load_test.go (bashMatch), helpers_test.go                  # Task 3
.github/workflows/release.yaml, release-please-config.json, .release-please-manifest.json           # Task 4
.github/actionlint.yaml; internal/workflowspec/release_test.go, load_test.go; docs/runbooks/releases.md  # Task 4
.github/workflows/e2e.yaml; internal/workflowspec/e2e_test.go; docs/e2e.md                          # Task 5
.github/actions/setup-idp/action.yaml; internal/workflowspec/setupidp_test.go, load_test.go (ifBody), helpers_test.go; docs/pipelines.md  # Task 6
internal/e2e/pin.go, pin_test.go, scenario_test.go; docs/e2e.md                                     # Task 7
.github/workflows/renovate.yaml, .github/renovate-global.json, .github/workflows/ci.yaml (validator)  # Task 8
internal/workflowspec/renovate_test.go, load_test.go (repoFiles); docs/runbooks/dependencies.md, docs/runbooks/bootstrap.md  # Task 8
examples/claims-repo/.github/workflows/*.yaml; internal/workflowspec/rules_test.go; docs/pipelines.md  # Task 11
idp-claims: .github/workflows/{pr,reconcile,drift,recover}.yaml, config/platform.yaml                # Task 12
docs/phases/phase-1e.md, README.md                                                                   # Task 13
```

## Pre-flight (controller, before Task 1)

- [ ] `gh api user --jq .login` prints `jellalshadows` (else `gh auth switch -u jellalshadows`).
- [ ] Recreate the zizmor venv in the session scratchpad (`python -m venv <scratchpad>/zizmor-venv` then `<scratchpad>/zizmor-venv/Scripts/pip install zizmor==1.30.1`) and give its executable to dispatches as `ZIZMOR`.
- [ ] Re-verify the pins (each line prints `commit <sha>` equal to the table):

```bash
for rt in googleapis/release-please-action:v5.0.0 goreleaser/goreleaser-action:v7.2.3 actions/attest-build-provenance:v4.2.2 renovatebot/github-action:v46.3.6; do
  gh api "repos/${rt%%:*}/git/ref/tags/${rt##*:}" --jq '"\(.object.type) \(.object.sha)"'
done
```

- [ ] `gh api "users/jellalshadows-idp-bot%5Bbot%5D" --jq .id` prints `343234638` (the git author in `.github/renovate-global.json`).
- [ ] The owner approves this plan, its side effects (including merging the release PR) and the rulings D-1 to D-33; ledger them. Commit this plan to `main` as `docs: add the phase 1e plan` and push.

---

### Task 1: Decisions first — ADRs 0028–0029, spec amendment A9, ADR amendments

**Files:**
- Create: `docs/adr/0028-releases-and-how-the-workflows-get-idp.md`, `docs/adr/0029-self-hosted-renovate.md`
- Modify: `docs/adr/README.md`, `docs/adr/0009-two-apps-and-split-environments.md`, `docs/adr/0018-plan-fingerprint-and-gate-trust.md`, `docs/adr/0021-reusable-workflows-build-idp-from-their-commit.md`, `docs/adr/0023-pipeline-tokens-per-step.md`, `docs/adr/0025-plans-refresh-with-the-writer-on-main.md`, `docs/adr/0026-automation-app.md`, `docs/adr/0027-end-to-end-harness.md`, `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`

**Interfaces:**
- Produces: the decisions Tasks 2–13 implement and cite; the spec anchor `#73-three-github-apps-and-their-secret-scopes`.

- [ ] **Step 1: Create the branch**

Run: `git switch main && git pull --ff-only && git switch -c feat/phase-1e-releases`

- [ ] **Step 2: Write `docs/adr/0028-releases-and-how-the-workflows-get-idp.md`**

````markdown
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

- Every job's first step calls the GitHub API; when the tag listing fails, the job fails instead of guessing.
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
````

- [ ] **Step 3: Write `docs/adr/0029-self-hosted-renovate.md`**

````markdown
# 0029. Self-hosted Renovate with the bot App

- Status: Accepted
- Date: 2026-10-18
- Spec: [§3.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#31-repositories), [§6.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#64-drift-daily-cron), [§7.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#75-supply-chain)

## Context

Spec §7.5 pins every action by SHA with its version as a comment, pins tools, providers and Go modules exactly, and requires each pinned version to be at least 14 days old, enforced by Renovate's `minimumReleaseAge`. Claims repos pin the engine's reusable workflows by commit, and Renovate bumps them (spec §3.1, §6). Scheduled workflows of public repos stop after 60 days without activity (spec §6.4). The owner rejected the hosted Renovate App (ADR-0026) and chose Renovate self-hosted with the bot App.

## Decision

- `.github/workflows/renovate.yaml` in `idp-engine` runs Renovate weekly (Mondays 04:17 UTC) and on dispatch, in environment `idp-renovate` (deployment branch `main`), with a bot token minted for `idp-engine` and `idp-claims` only: contents, issues, pull requests and workflows write; checks and statuses read. `renovatebot/github-action` runs a pinned Renovate version.
- It covers `idp-engine` and `idp-claims`. `idp-claims-e2e` is left out: the end-to-end harness pins its callers on every run.
- One global configuration, `.github/renovate-global.json`, reviewed in `idp-engine`. The repositories carry no Renovate file and need no onboarding; a file added to one is ignored.
- **Nothing is merged automatically** (owner decision). The owner merges every Renovate PR, in `idp-claims` after `idp-gate` and the plan comment.
- **Every third-party update waits 14 days** after its release, and Renovate proposes only versions that have waited. **The engine's own releases do not wait** (owner decision): they are this project's code, and the harness proved them on their release PR.
- Groups: the four engine pins of a repository move in one PR ("idp engine"), because the four callers must pin one commit. The OpenTofu pins (`setup-idp`, `ci.yaml`, and the renderer's `required_version` with its golden render) move together ("opentofu"). The GitHub provider pins (the modules, the renderer and its golden render) move together ("github provider", labelled `needs-lockfile`); its lock file is regenerated by hand.
- Managers: GitHub Actions (SHA pins with version comments, reusable workflow refs included, and the versions the OpenTofu and GoReleaser actions take), Go modules (with `go mod tidy`) and the `go` directive's patch, which is the Go version CI and `setup-idp` install; provider pins from `registry.opentofu.org`; and regex managers for actionlint, zizmor, the Renovate version and the renderer's pins.
- Renovate commits through the GitHub API, so GitHub signs its commits, and it writes no commit statuses: the bot can read statuses but not write them, and a status write that fails stops Renovate's run of that repository. A Dependency Dashboard issue in each repository lists pending and open updates.

## Consequences

### Positive

- Every update arrives as a PR with CI and, in `idp-claims`, a plan.
- Pins stay at least 14 days old with no manual tracking, and the version comments stay true.
- Engine releases reach `idp-claims` the week they ship, or at once with a dispatch.
- Renovate PRs keep `idp-claims` active, so its drift schedule keeps running (spec §6.4).

### Negative / costs

- A provider update fails CI until its lock file is regenerated by hand ([dependencies runbook](../runbooks/dependencies.md)).
- Weekly runs: a fix waits up to a week plus its 14 days, unless someone dispatches the workflow.
- The bot key lives in one more environment, and each rotation sets it there too.
- No `renovate/stability-days` status on branches; the dashboard shows the pending updates instead.

## Alternatives considered

- **The hosted Renovate App.** Rejected by the owner: a third party with write access to the repos (ADR-0026).
- **Dependabot.** Rejected: it has no regex managers for the tool versions that live in `run:` lines (actionlint, zizmor) and in the renderer, and the owner chose Renovate.
- **Automerge for patch and digest updates.** Rejected for now by the owner.
- **A Renovate file in each repository.** Rejected: one reviewed place for the whole policy, and no file in a claims repo that a claims PR could change.
- **Including `idp-claims-e2e`.** Rejected: the harness rewrites its callers on every run, so Renovate PRs there would only conflict.
````

- [ ] **Step 4: Add the rows to `docs/adr/README.md`** (after the `0027` row)

```markdown
| 0028 | [Releases, and how the workflows get idp](0028-releases-and-how-the-workflows-get-idp.md) | Accepted |
| 0029 | [Self-hosted Renovate with the bot App](0029-self-hosted-renovate.md) | Accepted |
```

- [ ] **Step 5: Amend the ADRs** (exact replacements with Edit)

1. `docs/adr/0021-reusable-workflows-build-idp-from-their-commit.md`, in "Context", old:

```markdown
No release exists yet, so there is no binary to download and verify (spec §7.5).
```

new:

```markdown
No release exists yet, so there is no binary to download and verify (spec §7.5). *(Amended in Phase 1e, 2026-10-18.)* Releases exist from `v0.1.0`; ADR-0028 decides how a job gets a released binary, and this decision stays for every commit that is not a release.
```

2. Same file, in "Decision", old:

```markdown
Callers pin the reusable workflows to a full commit SHA. The templates mark the pin `# unreleased` until releases exist.
```

new:

```markdown
Callers pin the reusable workflows to a full commit SHA. The templates mark the pin `# unreleased` until releases exist. *(Amended in Phase 1e, 2026-10-18.)* From `v0.1.0` the templates pin a release commit with its tag as the comment; only the end-to-end harness pins an unreleased commit, marked `# unreleased` (ADR-0028).
```

3. Same file, in "Negative / costs", old:

```markdown
- Module sources pin a commit, not a tag, until releases. Phase 1d decides how releases change this (spec §7.5).
```

new:

```markdown
- Module sources pin the commit of a source build. *(Amended in Phase 1e, 2026-10-18.)* A release commit installs the released binary instead, so its module sources pin the release tag (ADR-0028).
```

4. Same file, in "Alternatives considered", old:

```markdown
- **Download a released binary and verify its attestation.** Not possible before the first release; Phase 1d revisits it.
```

new:

```markdown
- **Download a released binary and verify its attestation.** Not possible before the first release. *(Amended in Phase 1e, 2026-10-18.)* Adopted for release commits by ADR-0028.
```

5. `docs/adr/0018-plan-fingerprint-and-gate-trust.md`, in "Negative / costs", old:

```markdown
- **Accepted risk**, alongside the passphrase risk of spec §7.3: the author check proves who wrote the comment, not that nobody edited it.
```

new:

```markdown
- **Accepted risk**, alongside the passphrase risk of spec §7.3: the author check proves who wrote the comment, and since Phase 1d the editor check proves who last edited it; neither proves which workflow of the repository wrote it. *(Amended in Phase 1e, 2026-10-18.)*
```

6. `docs/adr/0026-automation-app.md`, in "Decision", old:

```markdown
- Its key lives in `idp-engine` environments with deployment branch `main` (`idp-e2e` now; the release and Renovate environments in Phase 1e). Each job mints a token for the repositories and permissions it needs (ADR-0023).
```

new:

```markdown
- Its key lives in `idp-engine` environments with deployment branch `main`: `idp-e2e`, `idp-release` and `idp-renovate`. *(Amended in Phase 1e, 2026-10-18.)* Each job mints a token for the repositories and permissions it needs (ADR-0023).
```

7. Same file, in "Negative / costs", old:

```markdown
- It can merge pull requests in `idp-claims` once Renovate uses it (Phase 1e). The `idp-gate` check and the approval of destructive changes still apply; requiring a human review on claims repos remains the owner's decision (ADR-0022).
```

new:

```markdown
- It can merge pull requests in `idp-claims` once Renovate uses it (Phase 1e). The `idp-gate` check and the approval of destructive changes still apply; requiring a human review on claims repos remains the owner's decision (ADR-0022). *(Amended in Phase 1e, 2026-10-18.)* Renovate does not merge (ADR-0029): the owner merges every Renovate PR.
```

8. `docs/adr/0027-end-to-end-harness.md`, in "Decision", old:

```markdown
one run at a time (concurrency group `idp-e2e`). Release PRs join in Phase 1e.
```

new:

```markdown
one run at a time (concurrency group `idp-e2e`). Release PRs join in Phase 1e. *(Amended in Phase 1e, 2026-10-18.)* They reach the harness through a dispatch from `release.yaml` on `main`, and the run reports the commit status `idp-e2e` on the release PR's head (ADR-0028).
```

9. The spec §7.3 heading changes in Step 6, so its anchor changes. In each of `docs/adr/0009-two-apps-and-split-environments.md`, `docs/adr/0023-pipeline-tokens-per-step.md`, `docs/adr/0025-plans-refresh-with-the-writer-on-main.md` and `docs/adr/0026-automation-app.md`, replace `#73-two-github-apps-and-three-secret-scopes` with `#73-three-github-apps-and-their-secret-scopes` (one occurrence per file).

- [ ] **Step 6: Apply amendment A9 to the spec** (exact replacements with Edit)

1. §3.1, old:

```markdown
| `idp-claims` | `main` = desired state (DRY). `wet` = rendered output + encrypted GitHub state, written only by `idp-writer` | None. Renovate bumps the engine version |
```

new:

```markdown
| `idp-claims` | `main` = desired state (DRY). `wet` = rendered output + encrypted GitHub state, written only by `idp-writer` | None. Renovate, run from `idp-engine`, bumps the engine version (ADR-0029). *(Amendment A9.)* |
```

2. §3.3, old:

```markdown
`idp wet-push`, `idp encryption-env`, `idp feature test`, and
`idp bootstrap app|apply|check`. They share one exit-code convention
```

new:

```markdown
`idp wet-push`, `idp encryption-env`, `idp feature test`,
`idp bootstrap app|apply|check`, and `idp version`, which prints the build's version
(ADR-0028; *Amendment A9, 2026-10-18*). They share one exit-code convention
```

3. §6, old:

```markdown
job asks for what it needs. Every reusable job builds `idp` from the reusable
workflow's own commit, so the workflow, the CLI and the module refs are one version
(ADR-0021). Until Phase 2 the only stack is `github`, and until Phase 4 there are no
```

new:

```markdown
job asks for what it needs. Every reusable job gets `idp` for the reusable
workflow's own commit, so the workflow, the CLI and the module refs are one version
(ADR-0021): a release commit installs its verified release binary, and any other
commit is built from source (ADR-0028; *Amendment A9, 2026-10-18*). Until Phase 2 the only stack is `github`, and until Phase 4 there are no
```

4. §7.3 heading, old:

```markdown
### 7.3 Two GitHub Apps and three secret scopes
```

new:

```markdown
### 7.3 Three GitHub Apps and their secret scopes
```

5. §7.3 table, old:

```markdown
| `idp-bot` App key | Pull requests, contents and workflows on the engine and claims repos; members read | Only in `idp-engine` environments with deployment branch `main` (`idp-e2e`; the release and Renovate environments in Phase 1e); installed on `idp-engine`, `idp-claims` and `idp-claims-e2e` only (ADR-0026). *(Amendment A8.)* |
```

new:

```markdown
| `idp-bot` App key | Pull requests, contents and workflows on the engine and claims repos; members read | Only in `idp-engine` environments with deployment branch `main`: `idp-e2e`, `idp-release` and `idp-renovate`; installed on `idp-engine`, `idp-claims` and `idp-claims-e2e` only (ADR-0026). *(Amendment A8; environments named in A9.)* |
```

6. §7.3 roles, old:

```markdown
- `idp-reader` and `idp-writer` are **roles**. GitHub App names are unique across
  GitHub and limited to 34 characters, so the real Apps are named
  `<org>-reader` and `<org>-writer`. For example, `jellalshadows-idp-writer` is 24
  characters. Both claims repos (`idp-claims` and `idp-claims-e2e`) share the same
  two Apps, because Apps are installed per org.
```

new:

```markdown
- `idp-reader`, `idp-writer` and `idp-bot` are **roles**. GitHub App names are unique
  across GitHub and limited to 34 characters, so the real Apps are named
  `<org>-reader`, `<org>-writer` and `<org>-bot`. For example, `jellalshadows-idp-writer`
  is 24 characters. Both claims repos (`idp-claims` and `idp-claims-e2e`) share the
  reader and writer Apps, which are installed on the whole org; the bot is installed
  only on `idp-engine`, `idp-claims` and `idp-claims-e2e` (ADR-0026).
  *(Amendment A9, 2026-10-18.)*
```

7. §7.4, old:

```markdown
- **Tags in `idp-engine`, `idp-features` and `idp-modules`:** a published tag never
  moves.
```

new:

```markdown
- **Tags in `idp-engine`, `idp-features` and `idp-modules`:** a published tag never
  moves. In `idp-engine` the repository ruleset `idp-engine-tags` forbids deleting,
  force-moving or updating `refs/tags/v*`, with no bypass (ADR-0028).
  *(Amendment A9, 2026-10-18.)*
```

8. §7.5, old:

```markdown
- Every pinned version (actions, tools, providers, images, Go modules) must be at
  least 14 days old when it is adopted. Renovate enforces this with
  `minimumReleaseAge`.
- The CLI is released with checksums and a provenance attestation
  (`actions/attest-build-provenance`). The workflows run `gh attestation verify`
  before they execute it. Until the first release, the workflows build `idp` from
  source at the pinned commit instead (ADR-0021); Phase 1d decides how releases
  change that. *(Amendment A6, 2026-10-11.)*
```

new:

```markdown
- Every pinned version (actions, tools, providers, images, Go modules) must be at
  least 14 days old when it is adopted. Renovate enforces this with
  `minimumReleaseAge`. The engine's own releases, pinned by the claims repos and the
  caller templates, are exempt: they are this project's code, proven by the
  end-to-end harness on their release PR (ADR-0029). *(Amendment A9, 2026-10-18.)*
- The CLI is released with checksums and a provenance attestation
  (`actions/attest-build-provenance`). The workflows run `gh attestation verify`
  before they execute it. A pinned commit that carries a release tag installs that
  release's binary after checking it against the release's `checksums.txt` and
  verifying that `release.yaml` built it on `main` from that commit; it never falls
  back to a build. Any other commit, such as the harness's commit under test, is
  built from source at the pinned commit (ADR-0021, ADR-0028). *(Amendment A6,
  2026-10-11; A9, 2026-10-18.)*
```

9. §8.5, old:

```markdown
in `idp-claims-e2e` only, because only a person can approve a deployment (ADR-0027).
Release PRs join in Phase 1e. *(Amendment A8, 2026-10-13.)*
```

new:

```markdown
in `idp-claims-e2e` only, because only a person can approve a deployment (ADR-0027).
*(Amendment A8, 2026-10-13.)* When release-please creates or updates the engine's
release PR, `release.yaml` dispatches the harness on `main`, the release PR's base
(the same code without the changelog). The run posts the commit status `idp-e2e` on
the release PR's head, `pending` and then `success` or `failure`, and a release PR is
merged only when `idp-e2e` is green (ADR-0028). *(Amendment A9, 2026-10-18.)*
```

10. §10 table, old:

```markdown
| 1 → `0.1.0` | Group + Component (GitHub part), all pipelines (§6), **E2E harness born** | E2E creates, modifies and deletes an `e2e-` repo and team through `idp-claims-e2e` |
```

new:

```markdown
| 1 → `0.1.0` | Group + Component (GitHub part), all pipelines (§6), **E2E harness born**, releases and Renovate; `0.1.0` ships at the end of Phase 1e *(Amendment A9)* | E2E creates, modifies and deletes an `e2e-` repo and team through `idp-claims-e2e` |
```

- [ ] **Step 7: Check and commit**

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
print([pathlib.Path('docs/adr', n).exists() for n in ['0028-releases-and-how-the-workflows-get-idp.md', '0029-self-hosted-renovate.md']])
"
```

Expected: `all spec anchors resolve` and `[True, True]`. Then `rg -n "Phase 1d decides|three secret scopes" docs --glob '!docs/superpowers/plans/**' --glob '!docs/phases/**'` prints nothing, and the naming check prints nothing.

```bash
git add docs/adr docs/superpowers/specs/2026-10-08-idp-on-actions-design.md
git commit -m "docs: record the release and renovate decisions (adr 0028-0029, spec amendment a9)"
```

---

### Task 2: `idp version`

**Files:**
- Create: `internal/cli/version.go`, `internal/cli/version_test.go`
- Modify: `internal/cli/cli.go`, `docs/cli.md`

**Interfaces:**
- Consumes: `Run(args []string, stdout, stderr io.Writer, env Env) int`, `noEnv` (`internal/cli/cli_test.go`), `exitOK`, `exitUsage`, `usage`, `version.Version`.
- Produces: `func runVersion(args []string, stdout, stderr io.Writer) int`; the usage line `  version         Print this build's version (a release tag, a commit or dev)`.

- [ ] **Step 1: Write the failing tests** — `internal/cli/version_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

func TestVersionPrintsTheBuildVersion(t *testing.T) {
	old := version.Version
	version.Version = "v9.8.7"
	t.Cleanup(func() { version.Version = old })
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"version"}, &stdout, &stderr, noEnv); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if stdout.String() != "v9.8.7\n" || stderr.Len() != 0 {
		t.Errorf("stdout %q, stderr %q; want exactly the version and a newline: setup-idp compares it with the release tag", stdout.String(), stderr.String())
	}
}

func TestVersionRejectsArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"version", "extra"}, &stdout, &stderr, noEnv); code != 2 {
		t.Errorf("exit %d, want 2 (usage error, ADR-0016)", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), `idp version: unexpected argument "extra"`) {
		t.Errorf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestUsageListsVersion(t *testing.T) {
	if !strings.Contains(usage, "\n  version         Print this build's version (a release tag, a commit or dev)\n") {
		t.Error("idp help must list the version command")
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/cli/ -run 'TestVersion|TestUsageListsVersion'`
Expected: FAIL: `TestVersionPrintsTheBuildVersion` gets exit 2 (`idp: unknown command "version"`), and the usage test fails.

- [ ] **Step 3: Implement**

`internal/cli/version.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

// runVersion prints the build's version: the release tag of a released binary,
// the engine commit of a pipeline's source build, or "dev" (ADR-0028).
func runVersion(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("idp version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp version: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	fmt.Fprintln(stdout, version.Version)
	return exitOK
}
```

`internal/cli/cli.go`: in `usage`, add the line after the `bootstrap` line:

```text
  version         Print this build's version (a release tag, a commit or dev)
```

and in `Run`, add before `default:`:

```go
	case "version":
		return runVersion(args[1:], stdout, stderr)
```

`docs/cli.md`: append at the end:

````markdown
## version

Prints this build's version.

```text
idp version
```

It takes no flags.

Writes: one line with the version: the release tag of a released binary (for example `v0.1.0`), the engine commit of the source build the pipelines make for a commit that is not a release, or `dev` for a local build. Rendered module sources pin the same value. The workflows compare it with the release tag after they install a release ([ADR-0028](adr/0028-releases-and-how-the-workflows-get-idp.md)).

Exit codes: 0 printed; 2 usage error (an argument).
````

- [ ] **Step 4: Verify** — `go test ./... && go vet ./... && gofmt -l .` pass, `TestUsageColumnsAlign` included (the command column is 16 characters).

- [ ] **Step 5: Commit**

```bash
git add internal/cli docs/cli.md
git commit -m "feat(cli): add idp version"
```

---

### Task 3: GoReleaser config, its contract test and the CI snapshot

**Files:**
- Create: `.goreleaser.yaml`, `internal/workflowspec/goreleaser_test.go`
- Modify: `.gitignore`, `.github/workflows/ci.yaml`, `internal/workflowspec/load_test.go`, `internal/workflowspec/helpers_test.go`

**Interfaces:**
- Consumes: `readYAML`, `loadWorkflow`, `mustJob`, `mustStep`, `wantPerms`, `wantWith`, `wantRun`, `step`, `version.Repo`.
- Produces: `const goreleaserPath = ".goreleaser.yaml"`; `type goreleaserConfig`; `func loadGoreleaser(t *testing.T) goreleaserConfig`; `const archiveTemplate = "idp_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`; `var versionLdflag string`; `var exactVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)`; `func releaseAssets(t *testing.T, version string) []string`; `func ciGoreleaserVersion(t *testing.T) string`; in `load_test.go`, `func bashMatch(t *testing.T, where, run, variable string) *regexp.Regexp`.

- [ ] **Step 1: Write the failing tests**

`internal/workflowspec/helpers_test.go`, add:

```go
func TestBashMatch(t *testing.T) {
	run := `x=1
if ! [[ "$TAG" =~ ^v[0-9]+\.[0-9]+$ ]]; then
  exit 1
fi
`
	re := bashMatch(t, "inline", run, "TAG")
	if !re.MatchString("v1.2") || re.MatchString("v1.2.3") || re.MatchString("1.2") {
		t.Errorf("bashMatch = %q, want the pattern after =~", re)
	}
}
```

`internal/workflowspec/goreleaser_test.go`:

```go
package workflowspec

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

const (
	goreleaserPath  = ".goreleaser.yaml"
	archiveTemplate = "idp_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
)

// goreleaserConfig is the part of .goreleaser.yaml the release contract reads.
type goreleaserConfig struct {
	Version int `yaml:"version"`
	Builds  []struct {
		Main    string   `yaml:"main"`
		Binary  string   `yaml:"binary"`
		Env     []string `yaml:"env"`
		Flags   []string `yaml:"flags"`
		Ldflags []string `yaml:"ldflags"`
		Goos    []string `yaml:"goos"`
		Goarch  []string `yaml:"goarch"`
	} `yaml:"builds"`
	Archives []struct {
		Formats         []string `yaml:"formats"`
		NameTemplate    string   `yaml:"name_template"`
		FormatOverrides []struct {
			Goos    string   `yaml:"goos"`
			Formats []string `yaml:"formats"`
		} `yaml:"format_overrides"`
	} `yaml:"archives"`
	Checksum struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"checksum"`
	Changelog struct {
		Disable bool `yaml:"disable"`
	} `yaml:"changelog"`
	Release struct {
		Mode string `yaml:"mode"`
	} `yaml:"release"`
}

func loadGoreleaser(t *testing.T) goreleaserConfig {
	t.Helper()
	var c goreleaserConfig
	readYAML(t, goreleaserPath, &c)
	if len(c.Builds) != 1 || len(c.Archives) != 1 {
		t.Fatalf("%s: %d builds and %d archives, want one of each: the idp CLI only (the harness is never released, ADR-0027)", goreleaserPath, len(c.Builds), len(c.Archives))
	}
	return c
}

// versionLdflag is the -X the release build sets. {{.Tag}} keeps the v:
// rendered module sources pin ?ref=<this value>, and GoReleaser's {{.Version}}
// (0.1.0) names no ref of the engine (Phase 1a, final review minor #8).
var versionLdflag = "-X " + version.Repo + "/internal/version.Version={{.Tag}}"

var (
	versionTemplate = regexp.MustCompile(`\{\{\s*\.Version\s*\}\}`)
	exactVersion    = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
)

func TestGoreleaserBuildsOnlyTheCLI(t *testing.T) {
	c := loadGoreleaser(t)
	if c.Version != 2 {
		t.Errorf("version = %d, want 2", c.Version)
	}
	b := c.Builds[0]
	if b.Main != "./cmd/idp" || b.Binary != "idp" {
		t.Errorf("build main %q binary %q, want ./cmd/idp and idp", b.Main, b.Binary)
	}
	if !slices.Equal(b.Env, []string{"CGO_ENABLED=0"}) || !slices.Equal(b.Flags, []string{"-trimpath"}) {
		t.Errorf("env %v flags %v, want [CGO_ENABLED=0] and [-trimpath]", b.Env, b.Flags)
	}
	if want := []string{"-s -w " + versionLdflag}; !slices.Equal(b.Ldflags, want) {
		t.Errorf("ldflags = %q, want %q", b.Ldflags, want)
	}
	if !slices.Equal(b.Goos, []string{"darwin", "linux", "windows"}) || !slices.Equal(b.Goarch, []string{"amd64", "arm64"}) {
		t.Errorf("goos %v goarch %v, want darwin, linux, windows and amd64, arm64", b.Goos, b.Goarch)
	}
}

func TestGoreleaserKeepsTheVInTheVersion(t *testing.T) {
	found := false
	for _, f := range loadGoreleaser(t).Builds[0].Ldflags {
		if versionTemplate.MatchString(f) {
			t.Errorf("ldflag %q uses {{.Version}}, which drops the v: rendered module sources would pin ?ref=0.1.0, a ref the engine does not have", f)
		}
		if strings.Contains(f, versionLdflag) {
			found = true
		}
	}
	if !found {
		t.Errorf("no ldflag sets %q", versionLdflag)
	}
}

func TestGoreleaserUploadsIntoTheReleasePleaseRelease(t *testing.T) {
	c := loadGoreleaser(t)
	a := c.Archives[0]
	if a.NameTemplate != archiveTemplate {
		t.Errorf("archive name_template = %q, want %q: setup-idp downloads idp_<version>_linux_<arch>.tar.gz", a.NameTemplate, archiveTemplate)
	}
	if !slices.Equal(a.Formats, []string{"tar.gz"}) || len(a.FormatOverrides) != 1 || a.FormatOverrides[0].Goos != "windows" || !slices.Equal(a.FormatOverrides[0].Formats, []string{"zip"}) {
		t.Errorf("archive formats %v, overrides %+v; want tar.gz, and zip on windows", a.Formats, a.FormatOverrides)
	}
	if c.Checksum.NameTemplate != "checksums.txt" {
		t.Errorf("checksum name_template = %q, want checksums.txt", c.Checksum.NameTemplate)
	}
	if !c.Changelog.Disable {
		t.Error("changelog.disable must be true: release-please writes the release notes")
	}
	if c.Release.Mode != "keep-existing" {
		t.Errorf("release.mode = %q, want keep-existing: GoReleaser uploads into the release release-please created and keeps its notes", c.Release.Mode)
	}
}

// releaseAssets lists the files a release of version (without the v) carries,
// from .goreleaser.yaml: one archive per goos and goarch (zip on windows), then
// checksums.txt.
func releaseAssets(t *testing.T, version string) []string {
	t.Helper()
	c := loadGoreleaser(t)
	var out []string
	for _, goos := range c.Builds[0].Goos {
		for _, goarch := range c.Builds[0].Goarch {
			ext := ".tar.gz"
			if goos == "windows" {
				ext = ".zip"
			}
			out = append(out, "idp_"+version+"_"+goos+"_"+goarch+ext)
		}
	}
	return append(out, c.Checksum.NameTemplate)
}

// ciGoreleaserVersion is the GoReleaser version CI checks the config with.
func ciGoreleaserVersion(t *testing.T) string {
	t.Helper()
	_, s := mustStep(t, "ci release-snapshot", mustJob(t, loadWorkflow(t, ".github/workflows/ci.yaml"), "release-snapshot").Steps, "check")
	return s.With["version"]
}

func TestCIProvesTheGoreleaserConfig(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, ".github/workflows/ci.yaml"), "release-snapshot")
	where := "ci release-snapshot"
	wantPerms(t, where, j.Permissions, map[string]string{"contents": "read"})
	_, co := mustStep(t, where, j.Steps, "checkout")
	wantWith(t, where+" checkout", co, map[string]string{"fetch-depth": "0", "persist-credentials": "false"})
	_, gs := mustStep(t, where, j.Steps, "go")
	wantWith(t, where+" setup-go", gs, map[string]string{"go-version-file": "go.mod", "cache": "false"})
	ci, check := mustStep(t, where, j.Steps, "check")
	si, snap := mustStep(t, where, j.Steps, "snapshot")
	ti, tc := mustStep(t, where, j.Steps, "tag-check")
	if !(ci < si && si < ti) {
		t.Errorf("steps out of order: check %d, snapshot %d, tag-check %d", ci, si, ti)
	}
	for _, s := range []step{check, snap} {
		if !strings.HasPrefix(s.Uses, "goreleaser/goreleaser-action@") || s.With["distribution"] != "goreleaser" {
			t.Errorf("%s: want goreleaser/goreleaser-action with distribution goreleaser, got %q %v", s.ID, s.Uses, s.With)
		}
		if !exactVersion.MatchString(s.With["version"]) {
			t.Errorf("%s: GoReleaser version %q must be an exact vX.Y.Z pin (spec §7.5)", s.ID, s.With["version"])
		}
		if len(s.Env) != 0 {
			t.Errorf("%s env = %v: the snapshot publishes nothing and needs no token", s.ID, s.Env)
		}
	}
	if check.With["version"] != snap.With["version"] {
		t.Errorf("check runs GoReleaser %q, snapshot %q: keep one version", check.With["version"], snap.With["version"])
	}
	if check.With["args"] != "check" || snap.With["args"] != "release --snapshot --clean --skip=publish" {
		t.Errorf("args = %q and %q, want check and release --snapshot --clean --skip=publish", check.With["args"], snap.With["args"])
	}
	wantRun(t, where+" tag-check", tc,
		`tar -xzf "${archives[0]}" -C "$RUNNER_TEMP/idp-check"`,
		`got="$("$RUNNER_TEMP/idp-check/idp" version)"`)
	re := bashMatch(t, where+" tag-check", tc.Run, "got")
	if !re.MatchString("v0.0.0") || re.MatchString("0.0.0-SNAPSHOT-abc1234") {
		t.Errorf("tag-check pattern %q must accept a v-tag and refuse a {{.Version}} value", re)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/`
Expected: FAIL to compile (`undefined: bashMatch`).

- [ ] **Step 3: Implement**

`internal/workflowspec/load_test.go`: add `"regexp"` to the imports and, after `wantRun`:

```go
// bashMatch returns the extended regular expression a run script tests
// variable against ([[ "$variable" =~ pattern ]]), compiled with Go's regexp:
// for the anchored character-class patterns the workflows use, bash and RE2
// agree. It reads the first such test.
func bashMatch(t *testing.T, where, run, variable string) *regexp.Regexp {
	t.Helper()
	marker := `"$` + variable + `" =~ `
	i := strings.Index(run, marker)
	if i < 0 {
		t.Fatalf("%s: the run script never tests $%s with =~", where, variable)
	}
	rest := run[i+len(marker):]
	j := strings.Index(rest, " ]]")
	if j < 0 {
		t.Fatalf("%s: unterminated [[ ... =~ ... ]] for $%s", where, variable)
	}
	re, err := regexp.Compile(rest[:j])
	if err != nil {
		t.Fatalf("%s: the pattern for $%s does not compile: %v", where, variable, err)
	}
	return re
}
```

`.goreleaser.yaml`:

```yaml
# GoReleaser configuration for the idp CLI (ADR-0028). release.yaml runs it
# after release-please has tagged a release; ci.yaml checks it and builds a
# snapshot on every pull request. It never runs locally.
version: 2

builds:
  - main: ./cmd/idp
    binary: idp
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    # {{.Tag}} keeps the v: rendered module sources pin ?ref=<this version>,
    # and {{.Version}} (0.1.0) names no ref of the engine.
    ldflags:
      - -s -w -X github.com/jellalshadows-idp/idp-engine/internal/version.Version={{.Tag}}
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64

archives:
  - formats:
      - tar.gz
    name_template: "idp_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        formats:
          - zip

checksum:
  name_template: checksums.txt

# release-please writes the notes and creates the release; GoReleaser uploads
# the archives and checksums.txt into it.
changelog:
  disable: true

release:
  mode: keep-existing
```

`.gitignore`: append

```text
# GoReleaser output (CI only)
/dist/
```

`.github/workflows/ci.yaml`: append this job after `render-smoke`:

```yaml

  release-snapshot:
    runs-on: ubuntu-24.04
    permissions:
      contents: read
    steps:
      - id: checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          fetch-depth: 0
          persist-credentials: false
      - id: go
        uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: false
      - name: goreleaser check
        id: check
        uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        with:
          distribution: goreleaser
          version: v2.18.2
          args: check
      - name: goreleaser snapshot
        id: snapshot
        uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        with:
          distribution: goreleaser
          version: v2.18.2
          args: release --snapshot --clean --skip=publish
      - name: the snapshot binary reports a v-tag
        id: tag-check
        run: |
          archives=(dist/idp_*_linux_amd64.tar.gz)
          if [ "${#archives[@]}" -ne 1 ] || [ ! -f "${archives[0]}" ]; then
            echo "::error::want exactly one linux/amd64 archive in dist/, got ${archives[*]}"
            exit 1
          fi
          mkdir -p "$RUNNER_TEMP/idp-check"
          tar -xzf "${archives[0]}" -C "$RUNNER_TEMP/idp-check"
          got="$("$RUNNER_TEMP/idp-check/idp" version)"
          if ! [[ "$got" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "::error::the snapshot binary prints $got: the version ldflag must use {{.Tag}}, so rendered module sources keep the v (.goreleaser.yaml)"
            exit 1
          fi
          echo "snapshot idp version: $got"
```

- [ ] **Step 4: Verify** — `go test ./... && go vet ./... && gofmt -l .`; actionlint (no output); `"$ZIZMOR" --offline .github/workflows .github/actions/setup-idp/action.yaml examples/claims-repo/.github/workflows/*.yaml` (no unsuppressed findings; `cache: false` keeps the `cache-poisoning` audit quiet). GoReleaser itself is proven by CI in Task 9.

- [ ] **Step 5: Commit**

```bash
git add .goreleaser.yaml .gitignore .github/workflows/ci.yaml internal/workflowspec
git commit -m "build: add the goreleaser config and prove it in ci"
```

---

### Task 4: The release workflow, release-please and the releases runbook

**Files:**
- Create: `.github/workflows/release.yaml`, `release-please-config.json`, `.release-please-manifest.json`, `internal/workflowspec/release_test.go`, `docs/runbooks/releases.md`
- Modify: `.github/actionlint.yaml`, `internal/workflowspec/load_test.go`

**Interfaces:**
- Consumes: `loadWorkflow`, `mustJob`, `mustStep`, `wantPerms`, `wantEnv`, `wantWith`, `wantRun`, `bashMatch`, `strlist`, `concurrency`, `job`, `step`; Task 3's `releaseAssets`, `ciGoreleaserVersion`.
- Produces: `const releasePath = ".github/workflows/release.yaml"`; `func secretReaders(j *job, secret string) []string`; `func blockLines(s string) []string`; `func strictJSON(t *testing.T, rel string, out any)`; `internalWorkflows` gains `release.yaml`; the jobs `release-please` (outputs `release_created`, `tag_name`, `sha`, `prs_created`, `pr`), `publish`, `e2e-release-pr` (step `dispatch`, flag `-f release-pr-sha=`).

- [ ] **Step 1: Write the failing tests** — `internal/workflowspec/release_test.go`:

```go
package workflowspec

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const releasePath = ".github/workflows/release.yaml"

// secretReaders lists the steps of a job that read secret, and the job-level
// env that does; a step that only tests whether the secret is set (it compares
// the secret with the empty string) does not count. Expression contexts are
// case-insensitive, so the match is too.
func secretReaders(j *job, secret string) []string {
	ref := strings.ToLower("secrets." + secret)
	var out []string
	for _, k := range slices.Sorted(maps.Keys(j.Env)) {
		if strings.Contains(strings.ToLower(j.Env[k]), ref) {
			out = append(out, "job env "+k)
		}
	}
	for _, s := range j.Steps {
		texts := []string{s.Run}
		texts = append(texts, slices.Collect(maps.Values(s.Env))...)
		texts = append(texts, slices.Collect(maps.Values(s.With))...)
		for _, txt := range texts {
			if strings.ToLower(txt) == "${{ "+ref+" != '' }}" {
				continue
			}
			if strings.Contains(strings.ToLower(txt), ref) {
				out = append(out, s.label())
				break
			}
		}
	}
	return out
}

// blockLines splits a block scalar into its trimmed, non-empty lines.
func blockLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func TestReleaseWorkflow(t *testing.T) {
	w := loadWorkflow(t, releasePath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"push", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want push and workflow_dispatch", got)
	}
	n, ok := w.trigger("push")
	if !ok {
		t.Fatal("no push trigger")
	}
	var push struct {
		Branches []string `yaml:"branches"`
	}
	if err := n.Decode(&push); err != nil || !slices.Equal(push.Branches, []string{"main"}) {
		t.Errorf("push branches = %v (%v), want [main]", push.Branches, err)
	}
	if w.Concurrency == nil || *w.Concurrency != (concurrency{Group: "idp-release", Queue: "max"}) {
		t.Errorf("concurrency = %+v, want group idp-release, no cancellation, queue max: a dropped run of the release merge would leave its release to the run of another commit", w.Concurrency)
	}
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"e2e-release-pr", "publish", "release-please"}) {
		t.Errorf("jobs = %v, want e2e-release-pr, publish and release-please", got)
	}
}

func TestReleasePleaseRunsWithTheBotToken(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, releasePath), "release-please")
	if j.Environment != "idp-release" {
		t.Errorf("environment = %q, want idp-release: the bot key lives only in idp-engine environments limited to main (ADR-0026)", j.Environment)
	}
	wantPerms(t, "release-please", j.Permissions, map[string]string{})
	_, bot := mustStep(t, "release-please", j.Steps, "bot")
	want := map[string]string{
		"client-id":                "${{ vars.IDP_BOT_CLIENT_ID }}",
		"private-key":              "${{ secrets.IDP_BOT_PRIVATE_KEY }}",
		"owner":                    "jellalshadows-idp",
		"repositories":             "idp-engine",
		"permission-contents":      "write",
		"permission-issues":        "write",
		"permission-pull-requests": "write",
	}
	if !strings.HasPrefix(bot.Uses, "actions/create-github-app-token@") || !maps.Equal(map[string]string(bot.With), want) {
		t.Errorf("bot step = %q %v, want create-github-app-token with %v", bot.Uses, bot.With, want)
	}
	_, rp := mustStep(t, "release-please", j.Steps, "release")
	if !strings.HasPrefix(rp.Uses, "googleapis/release-please-action@") || !maps.Equal(map[string]string(rp.With), map[string]string{"token": "${{ steps.bot.outputs.token }}"}) {
		t.Errorf("release step = %q %v, want release-please-action with only the bot token: a GITHUB_TOKEN pull request would get no CI", rp.Uses, rp.With)
	}
	wantOutputs := map[string]string{
		"release_created": "${{ steps.release.outputs.release_created }}",
		"tag_name":        "${{ steps.release.outputs.tag_name }}",
		"sha":             "${{ steps.release.outputs.sha }}",
		"prs_created":     "${{ steps.release.outputs.prs_created }}",
		"pr":              "${{ steps.release.outputs.pr }}",
	}
	if !maps.Equal(map[string]string(j.Outputs), wantOutputs) {
		t.Errorf("outputs = %v, want %v", j.Outputs, wantOutputs)
	}
	if got := secretReaders(j, "IDP_BOT_PRIVATE_KEY"); !slices.Equal(got, []string{"bot"}) {
		t.Errorf("steps reading the bot key = %v, want only the token step", got)
	}
}

func TestReleasePleaseNamesAMissingCredential(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, releasePath), "release-please")
	if len(j.Steps) == 0 || j.Steps[0].ID != "config" {
		t.Fatal("the first step must check the environment, so a missing credential fails with its name instead of a token error")
	}
	c := j.Steps[0]
	wantEnv(t, "release-please config", c, map[string]string{
		"BOT_CLIENT_ID": "${{ vars.IDP_BOT_CLIENT_ID }}",
		"BOT_KEY_SET":   "${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}",
	})
	wantRun(t, "release-please config", c,
		`missing="$missing variable IDP_BOT_CLIENT_ID,"`,
		`missing="$missing secret IDP_BOT_PRIVATE_KEY,"`,
		`echo "::error::the idp-release environment is missing:${missing%,} (docs/runbooks/releases.md)"`,
		"exit 1")
}

func TestReleasePublishesOnlyACreatedRelease(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, releasePath), "publish")
	if !slices.Equal(j.Needs, strlist{"release-please"}) || j.If != "needs.release-please.outputs.release_created == 'true'" {
		t.Errorf("needs %v if %q, want release-please and release_created == 'true' (spec §9.1)", j.Needs, j.If)
	}
	if j.Environment != "" {
		t.Errorf("environment = %q: publishing holds no bot key", j.Environment)
	}
	wantPerms(t, "publish", j.Permissions, map[string]string{"attestations": "write", "contents": "write", "id-token": "write"})
	gi, guard := mustStep(t, "publish", j.Steps, "guard")
	_, co := mustStep(t, "publish", j.Steps, "checkout")
	_, gs := mustStep(t, "publish", j.Steps, "go")
	ri, rel := mustStep(t, "publish", j.Steps, "goreleaser")
	vi, check := mustStep(t, "publish", j.Steps, "version")
	ai, att := mustStep(t, "publish", j.Steps, "attest")
	if gi != 0 || !(ri < vi && vi < ai) {
		t.Errorf("steps: guard %d, goreleaser %d, version %d, attest %d; want the guard first, then GoReleaser, the version check and the attestation", gi, ri, vi, ai)
	}
	wantEnv(t, "publish guard", guard, map[string]string{
		"TAG":         "${{ needs.release-please.outputs.tag_name }}",
		"RELEASE_SHA": "${{ needs.release-please.outputs.sha }}",
	})
	wantRun(t, "publish guard", guard, `if [ "$RELEASE_SHA" != "$GITHUB_SHA" ]; then`, `echo "version=${TAG#v}" >> "$GITHUB_OUTPUT"`)
	wantWith(t, "publish checkout", co, map[string]string{"ref": "${{ needs.release-please.outputs.sha }}", "fetch-depth": "0", "persist-credentials": "false"})
	wantWith(t, "publish setup-go", gs, map[string]string{"go-version-file": "go.mod", "cache": "false"})
	if !strings.HasPrefix(rel.Uses, "goreleaser/goreleaser-action@") {
		t.Errorf("goreleaser step uses %q", rel.Uses)
	}
	wantWith(t, "publish goreleaser", rel, map[string]string{"distribution": "goreleaser", "version": ciGoreleaserVersion(t), "args": "release --clean"})
	if want := map[string]string{"GITHUB_TOKEN": "${{ github.token }}", "GORELEASER_CURRENT_TAG": "${{ needs.release-please.outputs.tag_name }}"}; !maps.Equal(map[string]string(rel.Env), want) {
		t.Errorf("goreleaser env = %v, want %v: it uploads with the workflow token, never a personal one (spec §9.1)", rel.Env, want)
	}
	wantEnv(t, "publish version", check, map[string]string{
		"TAG":     "${{ needs.release-please.outputs.tag_name }}",
		"VERSION": "${{ steps.guard.outputs.version }}",
	})
	wantRun(t, "publish version", check,
		`tar -xzf "dist/idp_${VERSION}_linux_amd64.tar.gz" -C "$RUNNER_TEMP/idp-check"`,
		`got="$("$RUNNER_TEMP/idp-check/idp" version)"`,
		`if [ "$got" != "$TAG" ]; then`)
	if !strings.HasPrefix(att.Uses, "actions/attest-build-provenance@") {
		t.Errorf("attest step uses %q", att.Uses)
	}
	var want []string
	for _, a := range releaseAssets(t, "${{ steps.guard.outputs.version }}") {
		want = append(want, "dist/"+a)
	}
	if got := blockLines(att.With["subject-path"]); !slices.Equal(got, want) {
		t.Errorf("attested subjects = %v, want every uploaded file %v", got, want)
	}
}

func TestReleasePublishRefusesANonSemverTag(t *testing.T) {
	_, guard := mustStep(t, "publish", mustJob(t, loadWorkflow(t, releasePath), "publish").Steps, "guard")
	re := bashMatch(t, "publish guard", guard.Run, "TAG")
	for _, tag := range []string{"v0.1.0", "v1.2.3", "v10.20.30"} {
		if !re.MatchString(tag) {
			t.Errorf("the guard refuses %q, a release tag", tag)
		}
	}
	for _, tag := range []string{"v0.1.0-rc.1", "0.1.0", "v0.1", "v0.1.0+build", "idp-v0.1.0", "v0.1.0 "} {
		if re.MatchString(tag) {
			t.Errorf("the guard accepts %q: rendered module sources would pin it, and only vX.Y.Z is an engine release", tag)
		}
	}
	wantRun(t, "publish guard", guard, `if ! [[ "$TAG" =~ `, "exit 1")
}

func TestReleasePRGetsTheHarness(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, releasePath), "e2e-release-pr")
	if !slices.Equal(j.Needs, strlist{"release-please"}) || j.If != "needs.release-please.outputs.prs_created == 'true'" {
		t.Errorf("needs %v if %q, want release-please and prs_created == 'true'", j.Needs, j.If)
	}
	wantPerms(t, "e2e-release-pr", j.Permissions, map[string]string{"actions": "write", "pull-requests": "read"})
	_, d := mustStep(t, "e2e-release-pr", j.Steps, "dispatch")
	if want := map[string]string{"GH_TOKEN": "${{ github.token }}", "PR_NUMBER": "${{ fromJSON(needs.release-please.outputs.pr).number }}"}; !maps.Equal(map[string]string(d.Env), want) {
		t.Errorf("dispatch env = %v, want %v", d.Env, want)
	}
	wantRun(t, "e2e-release-pr dispatch", d,
		`head="$(gh pr view "$PR_NUMBER" --repo "$GITHUB_REPOSITORY" --json headRefOid --jq .headRefOid)"`,
		`gh workflow run e2e.yaml --repo "$GITHUB_REPOSITORY" --ref main -f release-pr-sha="$head"`)
	re := bashMatch(t, "e2e-release-pr dispatch", d.Run, "head")
	if !re.MatchString(strings.Repeat("a", 40)) || re.MatchString(strings.Repeat("a", 39)) || re.MatchString("") {
		t.Errorf("head pattern %q must accept only a full commit SHA", re)
	}
}

func TestReleasePleaseConfig(t *testing.T) {
	var c struct {
		Schema       string `json:"$schema"`
		AlwaysUpdate bool   `json:"always-update"`
		Packages     map[string]struct {
			ReleaseType           string `json:"release-type"`
			InitialVersion        string `json:"initial-version"`
			BumpMinorPreMajor     bool   `json:"bump-minor-pre-major"`
			IncludeComponentInTag *bool  `json:"include-component-in-tag"`
			IncludeVInTag         *bool  `json:"include-v-in-tag"`
			ChangelogSections     []struct {
				Type    string `json:"type"`
				Section string `json:"section"`
				Hidden  bool   `json:"hidden"`
			} `json:"changelog-sections"`
		} `json:"packages"`
	}
	strictJSON(t, "release-please-config.json", &c)
	if !c.AlwaysUpdate {
		t.Error("always-update must be true: every push to main rebuilds the release PR and re-runs the harness on it")
	}
	p, ok := c.Packages["."]
	if !ok || len(c.Packages) != 1 {
		t.Fatalf("packages = %v, want exactly the root package: one version for everything (spec §3.1)", slices.Sorted(maps.Keys(c.Packages)))
	}
	if p.ReleaseType != "go" || p.InitialVersion != "0.1.0" || !p.BumpMinorPreMajor {
		t.Errorf("release-type %q initial-version %q bump-minor-pre-major %v; want go, 0.1.0 (release-please's own default is 1.0.0) and true (spec §9.1)", p.ReleaseType, p.InitialVersion, p.BumpMinorPreMajor)
	}
	if p.IncludeComponentInTag == nil || *p.IncludeComponentInTag || p.IncludeVInTag == nil || !*p.IncludeVInTag {
		t.Error("tags must be vX.Y.Z: include-component-in-tag false, include-v-in-tag true")
	}
	var visible, hidden []string
	for _, s := range p.ChangelogSections {
		if s.Hidden {
			hidden = append(hidden, s.Type)
		} else {
			visible = append(visible, s.Type)
		}
	}
	if !slices.Equal(visible, []string{"feat", "fix", "perf", "refactor", "docs", "revert"}) || !slices.Equal(hidden, []string{"chore", "test"}) {
		t.Errorf("changelog sections: visible %v hidden %v", visible, hidden)
	}
	var m map[string]string
	strictJSON(t, ".release-please-manifest.json", &m)
	if len(m) != 1 || !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(m["."]) {
		t.Errorf("manifest = %v, want only the root package at an X.Y.Z version", m)
	}
}

// strictJSON decodes a repo-relative JSON file into out, refusing unknown keys.
func strictJSON(t *testing.T, rel string, out any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		t.Fatalf("decode %s: %v (every key must be known to this test)", rel, err)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/ -run 'TestRelease'`
Expected: FAIL (`read .github/workflows/release.yaml: …`, `read release-please-config.json: …`).

- [ ] **Step 3: Implement**

`.github/workflows/release.yaml`:

```yaml
name: idp-release

# Releases idp-engine (spec §9.1, ADR-0028). release-please keeps one release PR
# up to date and, when it merges, tags the release; the same run then builds
# the CLI with GoReleaser, uploads it into that release and attests its
# provenance. Every push to main gets its own run, in order (queue: max).

on:
  push:
    branches: [main]
  workflow_dispatch:

permissions: {}

defaults:
  run:
    shell: bash

concurrency:
  group: idp-release
  cancel-in-progress: false
  queue: max

jobs:
  release-please:
    name: release-please
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    environment: idp-release
    permissions: {}
    outputs:
      release_created: ${{ steps.release.outputs.release_created }}
      tag_name: ${{ steps.release.outputs.tag_name }}
      sha: ${{ steps.release.outputs.sha }}
      prs_created: ${{ steps.release.outputs.prs_created }}
      pr: ${{ steps.release.outputs.pr }}
    steps:
      - name: check the environment
        id: config
        env:
          BOT_CLIENT_ID: ${{ vars.IDP_BOT_CLIENT_ID }}
          BOT_KEY_SET: ${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}
        run: |
          missing=""
          [ -n "$BOT_CLIENT_ID" ] || missing="$missing variable IDP_BOT_CLIENT_ID,"
          [ "$BOT_KEY_SET" = true ] || missing="$missing secret IDP_BOT_PRIVATE_KEY,"
          if [ -n "$missing" ]; then
            echo "::error::the idp-release environment is missing:${missing%,} (docs/runbooks/releases.md)"
            exit 1
          fi
      # A pull request opened with GITHUB_TOKEN would get no CI (ADR-0026).
      - name: mint the bot token
        id: bot
        uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0
        with:
          client-id: ${{ vars.IDP_BOT_CLIENT_ID }}
          private-key: ${{ secrets.IDP_BOT_PRIVATE_KEY }}
          owner: jellalshadows-idp
          repositories: idp-engine
          permission-contents: write
          permission-issues: write
          permission-pull-requests: write
      - name: release-please
        id: release
        uses: googleapis/release-please-action@45996ed1f6d02564a971a2fa1b5860e934307cf7 # v5.0.0
        with:
          token: ${{ steps.bot.outputs.token }}

  publish:
    name: publish
    needs: release-please
    if: needs.release-please.outputs.release_created == 'true'
    runs-on: ubuntu-24.04
    timeout-minutes: 30
    permissions:
      attestations: write
      contents: write
      id-token: write
    steps:
      - name: check the release
        id: guard
        env:
          TAG: ${{ needs.release-please.outputs.tag_name }}
          RELEASE_SHA: ${{ needs.release-please.outputs.sha }}
        run: |
          if ! [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "::error::refusing to publish $TAG: an engine release tag is vX.Y.Z, because rendered module sources pin it"
            exit 1
          fi
          if [ "$RELEASE_SHA" != "$GITHUB_SHA" ]; then
            echo "::error::release $TAG tags $RELEASE_SHA, but this run is for $GITHUB_SHA: its attestation would name the wrong commit (docs/runbooks/releases.md#publishing-failed)"
            exit 1
          fi
          echo "version=${TAG#v}" >> "$GITHUB_OUTPUT"
      - id: checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: ${{ needs.release-please.outputs.sha }}
          fetch-depth: 0
          persist-credentials: false
      - id: go
        uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: false
      - name: build and upload with GoReleaser
        id: goreleaser
        uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        env:
          GITHUB_TOKEN: ${{ github.token }}
          GORELEASER_CURRENT_TAG: ${{ needs.release-please.outputs.tag_name }}
        with:
          distribution: goreleaser
          version: v2.18.2
          args: release --clean
      - name: the released binary reports its tag
        id: version
        env:
          TAG: ${{ needs.release-please.outputs.tag_name }}
          VERSION: ${{ steps.guard.outputs.version }}
        run: |
          mkdir -p "$RUNNER_TEMP/idp-check"
          tar -xzf "dist/idp_${VERSION}_linux_amd64.tar.gz" -C "$RUNNER_TEMP/idp-check"
          got="$("$RUNNER_TEMP/idp-check/idp" version)"
          if [ "$got" != "$TAG" ]; then
            echo "::error::the linux/amd64 binary of $TAG prints $got: the version ldflag must use {{.Tag}} (.goreleaser.yaml)"
            exit 1
          fi
          echo "idp version: $got"
      - name: attest the build provenance
        id: attest
        uses: actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8 # v4.2.2
        with:
          subject-path: |
            dist/idp_${{ steps.guard.outputs.version }}_darwin_amd64.tar.gz
            dist/idp_${{ steps.guard.outputs.version }}_darwin_arm64.tar.gz
            dist/idp_${{ steps.guard.outputs.version }}_linux_amd64.tar.gz
            dist/idp_${{ steps.guard.outputs.version }}_linux_arm64.tar.gz
            dist/idp_${{ steps.guard.outputs.version }}_windows_amd64.zip
            dist/idp_${{ steps.guard.outputs.version }}_windows_arm64.zip
            dist/checksums.txt

  e2e-release-pr:
    name: e2e on the release PR
    needs: release-please
    if: needs.release-please.outputs.prs_created == 'true'
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    permissions:
      actions: write
      pull-requests: read
    steps:
      # A workflow_dispatch made with GITHUB_TOKEN does start a run. The harness
      # runs on main, the release PR's base, and reports idp-e2e on its head.
      - name: dispatch the harness for the release PR head
        id: dispatch
        env:
          GH_TOKEN: ${{ github.token }}
          PR_NUMBER: ${{ fromJSON(needs.release-please.outputs.pr).number }}
        run: |
          if ! [[ "$PR_NUMBER" =~ ^[0-9]+$ ]]; then
            echo "::error::release-please reported a release PR without a number"
            exit 1
          fi
          head="$(gh pr view "$PR_NUMBER" --repo "$GITHUB_REPOSITORY" --json headRefOid --jq .headRefOid)"
          if ! [[ "$head" =~ ^[0-9a-f]{40}$ ]]; then
            echo "::error::could not read the head commit of release PR #$PR_NUMBER"
            exit 1
          fi
          gh workflow run e2e.yaml --repo "$GITHUB_REPOSITORY" --ref main -f release-pr-sha="$head"
          echo "dispatched idp-e2e on main for release PR #$PR_NUMBER at $head"
```

`release-please-config.json`:

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/v17.6.0/schemas/config.json",
  "always-update": true,
  "packages": {
    ".": {
      "release-type": "go",
      "initial-version": "0.1.0",
      "bump-minor-pre-major": true,
      "include-component-in-tag": false,
      "include-v-in-tag": true,
      "changelog-sections": [
        { "type": "feat", "section": "Features" },
        { "type": "fix", "section": "Bug Fixes" },
        { "type": "perf", "section": "Performance Improvements" },
        { "type": "refactor", "section": "Code Refactoring" },
        { "type": "docs", "section": "Documentation" },
        { "type": "revert", "section": "Reverts" },
        { "type": "chore", "section": "Miscellaneous Chores", "hidden": true },
        { "type": "test", "section": "Tests", "hidden": true }
      ]
    }
  }
}
```

`.release-please-manifest.json`:

```json
{
  ".": "0.0.0"
}
```

`.github/actionlint.yaml` (whole file):

```yaml
# actionlint configuration. Every ignore is narrow and says why.
paths:
  .github/workflows/*.yaml:
    ignore:
      # job.workflow_repository and job.workflow_sha are documented job-context
      # properties (the reusable workflow's own repository and commit, ADR-0021)
      # that actionlint v1.7.12 does not know yet.
      - 'property "workflow_(repository|sha)" is not defined in object type'
      # concurrency.queue is a documented GitHub key (spec §6.2) that actionlint
      # v1.7.12 does not know yet; release.yaml uses it so that no release run
      # is dropped (ADR-0028).
      - 'unexpected key "queue" for "concurrency" section'
```

`internal/workflowspec/load_test.go`, old:

```go
	internalWorkflows = []string{"ci.yaml", "e2e.yaml"}
```

new:

```go
	internalWorkflows = []string{"ci.yaml", "e2e.yaml", "release.yaml"}
```

`docs/runbooks/releases.md`:

````markdown
# Runbook: releases

`idp-engine` is released as one version for everything: the CLI, the reusable workflows, the modules, the schemas and the lock files (spec §3.1, §9.1). Releases are `vX.Y.Z` tags; while the version is `0.x`, a breaking change bumps the minor. Why it works this way: [ADR-0028](../adr/0028-releases-and-how-the-workflows-get-idp.md).

## How a release happens

1. Every push to `main` runs `.github/workflows/release.yaml` (`idp-release`). Its `release-please` job, in the `idp-release` environment, keeps one release PR (branch `release-please--branches--main`, label `autorelease: pending`) with the next version and the changelog of the conventional commits since the last release, and rebuilds it on every push.
2. Each time it creates or updates the release PR, the `e2e-release-pr` job dispatches the end-to-end harness on `main` with the PR's head commit. The harness reports the commit status `idp-e2e` on that commit ([docs/e2e.md](../e2e.md#on-release-pull-requests)).
3. **Merge the release PR only when its `idp-e2e` status is green and CI passed,** with a merge commit: `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge --delete-branch`.
4. The push of that merge runs `release.yaml` again. release-please tags the merge commit and publishes the GitHub release with the changelog. In the same run the `publish` job refuses a tag that is not `vX.Y.Z` and a release of another commit, builds `idp` with GoReleaser, uploads six archives (linux, macOS and Windows; amd64 and arm64) and `checksums.txt` into the release, checks that the linux/amd64 binary prints the tag, and attests the build provenance.
5. Claims repos move to the release through Renovate's "idp engine" PR ([dependencies runbook](dependencies.md), [Upgrading the engine](../pipelines.md#upgrading-the-engine)).

The release PR and the tag are made by the bot App ([ADR-0026](../adr/0026-automation-app.md)), so CI runs on the release PR. GoReleaser uploads with the workflow's own token; no personal token is used anywhere (spec §9.1).

Check where a release stands:

```bash
gh pr list --repo jellalshadows-idp/idp-engine --label "autorelease: pending" --json number,title,headRefOid
gh api repos/jellalshadows-idp/idp-engine/commits/<head commit>/status --jq '.statuses[] | select(.context == "idp-e2e") | [.state, .target_url] | @tsv'
gh release view vX.Y.Z --repo jellalshadows-idp/idp-engine --json assets --jq '.assets[].name'
```

## Setting up the environment

```bash
gh api -X PUT repos/jellalshadows-idp/idp-engine/environments/idp-release \
  -F 'deployment_branch_policy[protected_branches]=false' -F 'deployment_branch_policy[custom_branch_policies]=true'
gh api -X POST repos/jellalshadows-idp/idp-engine/environments/idp-release/deployment-branch-policies -f name=main -f type=branch
gh secret set IDP_BOT_PRIVATE_KEY --env idp-release --repo jellalshadows-idp/idp-engine < ~/.idp/apps/jellalshadows-idp-bot.pem
gh variable set IDP_BOT_CLIENT_ID --env idp-release --repo jellalshadows-idp/idp-engine --body "<client_id from jellalshadows-idp-bot.json>"
```

A run without them fails at its first step, `the idp-release environment is missing: …`.

## The release PR has no green idp-e2e status

- **No status at all.** The dispatch waited behind a running harness, and a newer dispatch or the nightly run replaced it: GitHub keeps only the newest waiting run of a concurrency group. Dispatch it again for the current head:

  ```bash
  head="$(gh pr view <number> --repo jellalshadows-idp/idp-engine --json headRefOid --jq .headRefOid)"
  gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main -f release-pr-sha="$head"
  ```

- **`failure`.** Read the harness run linked from the status ([When a run fails](../e2e.md#when-a-run-fails)). Fix `main`; the next push rebuilds the release PR and runs the harness again.
- **`pending` that never ends.** The run was cancelled before its last step. Dispatch again as above.

## The first release

`release-please-config.json` sets `initial-version` to `0.1.0`: with no earlier release, release-please would otherwise propose `1.0.0`. If a release PR ever proposes a version it should not, push an empty commit to `main` whose body says the version, and the next run rebuilds the PR with it:

```bash
git commit --allow-empty -m "chore: release X.Y.Z" -m "Release-As: X.Y.Z"
git push
```

## Publishing failed

The tag and the release exist, but assets or the attestation are missing. Every pipeline pinned to that release commit fails at `setup-idp`, because a release commit never falls back to a build; nothing else is affected.

- **A step of `publish` failed** (GoReleaser, the version check, the attestation). Open the run and use **Re-run failed jobs**: the re-run keeps the run's commit and the `release-please` outputs, so it publishes the same tag. If GoReleaser had already uploaded some assets, delete those first, one per name: `gh release delete-asset vX.Y.Z <asset> --repo jellalshadows-idp/idp-engine --yes`.
- **`release vX.Y.Z tags <sha>, but this run is for <sha>`.** The release was created by the run of a later commit, so the attestation would name the wrong commit. Do not move or delete the tag (the ruleset forbids it). Cut the next patch with `Release-As: X.Y.<Z+1>` as above, and say in the broken release's notes that it has no assets.
- **The `release-please` job failed before tagging.** If `main` has not moved since the merge, dispatch `release.yaml` on `main`: `gh workflow run release.yaml --repo jellalshadows-idp/idp-engine --ref main`. If it has moved, the next run tags the merged release PR and `publish` refuses it as above; cut the next patch.

## Tags never move

The repository ruleset `idp-engine-tags` forbids deleting, force-moving or updating `refs/tags/v*`, with no bypass; creating a tag stays allowed, so release-please can tag. It was created with:

```bash
gh api -X POST repos/jellalshadows-idp/idp-engine/rulesets --input - <<'EOF'
{
  "name": "idp-engine-tags",
  "target": "tag",
  "enforcement": "active",
  "conditions": {"ref_name": {"include": ["refs/tags/v*"], "exclude": []}},
  "rules": [{"type": "deletion"}, {"type": "non_fast_forward"}, {"type": "update"}],
  "bypass_actors": []
}
EOF
```

Check it: `gh api repos/jellalshadows-idp/idp-engine/rulesets --jq '.[] | select(.name == "idp-engine-tags") | [.target, .enforcement] | @tsv'` prints `tag	active`.

GitHub's immutable releases stay off: they forbid adding assets to a published release, and GoReleaser uploads into the release that release-please has already published. Someone with write access could replace a published asset; the attestation check then fails every pipeline pinned to that release instead of running the replacement.

## Verify a release yourself

This is what `setup-idp` does before it runs a released binary (`windows_amd64.zip` for Windows):

```bash
v=X.Y.Z
gh release download "v$v" --repo jellalshadows-idp/idp-engine --pattern "idp_${v}_linux_amd64.tar.gz" --pattern checksums.txt
sha256sum --check --ignore-missing checksums.txt
gh attestation verify "idp_${v}_linux_amd64.tar.gz" --repo jellalshadows-idp/idp-engine \
  --signer-workflow jellalshadows-idp/idp-engine/.github/workflows/release.yaml \
  --source-ref refs/heads/main \
  --source-digest "$(gh api "repos/jellalshadows-idp/idp-engine/git/ref/tags/v$v" --jq .object.sha)"
```
````

- [ ] **Step 4: Verify** — `go test ./... && go vet ./... && gofmt -l .`; actionlint (no output); zizmor as in Task 3. Expected zizmor: no unsuppressed findings (the bot token names its repository and permissions, so `github-app` should not fire; `cache: false` keeps `cache-poisoning` quiet). Any other finding: fix it or carry an inline ignore with its reason, and report it.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/release.yaml .github/actionlint.yaml release-please-config.json .release-please-manifest.json internal/workflowspec docs/runbooks/releases.md
git commit -m "ci: release with release-please, goreleaser and build provenance"
```

---

### Task 5: The harness on release PRs and the `idp-e2e` status

**Files:**
- Modify: `.github/workflows/e2e.yaml`, `internal/workflowspec/e2e_test.go`, `docs/e2e.md`

**Interfaces:**
- Consumes: `e2ePath`, `e2eSecrets`, `loadWorkflow`, `mustJob`, `mustStep`, `wantPerms`, `wantEnv`, `wantRun`, `bashMatch`, `concurrency`; Task 4's `releasePath` and the `dispatch` step of `e2e-release-pr`.
- Produces: the `workflow_dispatch` input `release-pr-sha` (string, optional, default `''`); the steps `pending` (second) and `result` (last) of job `e2e`; job permissions `contents: read`, `statuses: write`; `const releasePRInput = "release-pr-sha"`.

- [ ] **Step 1: Write the failing tests** — replace `internal/workflowspec/e2e_test.go` with:

```go
package workflowspec

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

const e2ePath = ".github/workflows/e2e.yaml"

// releasePRInput is the e2e.yaml input release.yaml fills with the release PR's
// head commit (ADR-0028).
const releasePRInput = "release-pr-sha"

func TestE2EWorkflow(t *testing.T) {
	w := loadWorkflow(t, e2ePath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"schedule", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want schedule and workflow_dispatch: release PRs reach the harness through a dispatch from release.yaml (ADR-0028)", got)
	}
	n, ok := w.trigger("schedule")
	if !ok {
		t.Fatal("no schedule")
	}
	var crons []struct {
		Cron string `yaml:"cron"`
	}
	if err := n.Decode(&crons); err != nil || len(crons) != 1 || crons[0].Cron != "41 3 * * *" {
		t.Errorf("schedule = %v, %v", crons, err)
	}
	if w.Concurrency == nil || *w.Concurrency != (concurrency{Group: "idp-e2e"}) {
		t.Errorf("concurrency = %+v, want group idp-e2e without cancellation", w.Concurrency)
	}
	j := mustJob(t, w, "e2e")
	if j.Environment != "idp-e2e" {
		t.Errorf("environment = %q, want idp-e2e: the bot key and the approver token live only there (ADR-0027)", j.Environment)
	}
	wantPerms(t, "e2e", j.Permissions, map[string]string{"contents": "read", "statuses": "write"})
	// The harness's waits are bounded (DefaultTimeouts, about 300 minutes in
	// the worst case), so the job must outlast them and the cleanup.
	if j.TimeoutMinutes != 330 {
		t.Errorf("timeout-minutes = %d, want 330", j.TimeoutMinutes)
	}
}

func TestE2EIsNeverReRun(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	if len(j.Steps) == 0 || j.Steps[0].ID != "guard" {
		t.Fatal("the first step must be the guard: a re-run reuses the run id and collides with the first attempt's branches")
	}
	wantRun(t, "e2e guard", j.Steps[0],
		`if [ "$GITHUB_RUN_ATTEMPT" != 1 ]; then`,
		`echo "::error::re-running idp-e2e reuses its run id and collides with the first attempt's branches; dispatch a new run (docs/e2e.md)"`,
		"exit 1")
	if len(j.Steps[0].Env) != 0 {
		t.Errorf("guard env = %v, want none", j.Steps[0].Env)
	}
}

func TestE2EBuildsTheHarnessWithoutSecrets(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	i, b := mustStep(t, "e2e", j.Steps, "build")
	wantRun(t, "e2e build", b, `go build -o "$RUNNER_TEMP/idp-e2e" ./cmd/idp-e2e`)
	if len(b.Env) != 0 || len(b.With) != 0 {
		t.Errorf("build env = %v, with = %v: the build step sees no secret", b.Env, b.With)
	}
	if h, _ := mustStep(t, "e2e", j.Steps, "harness"); h < i {
		t.Error("the harness runs after it is built")
	}
}

// e2eSecrets are the idp-e2e environment's secrets: only the harness step reads them.
var e2eSecrets = []string{"secrets.IDP_BOT_PRIVATE_KEY", "secrets.IDP_E2E_APPROVER_TOKEN"}

func TestE2EHarnessMintsTheBotToken(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	for _, s := range j.Steps {
		if strings.Contains(s.Uses, "actions/create-github-app-token") {
			t.Errorf("step %q mints a bot token: a token lives one hour and a run can last longer, so the harness mints and refreshes its own (ADR-0027)", s.label())
		}
	}
	_, h := mustStep(t, "e2e", j.Steps, "harness")
	want := map[string]string{
		"E2E_BOT_CLIENT_ID":   "${{ vars.IDP_BOT_CLIENT_ID }}",
		"E2E_BOT_PRIVATE_KEY": "${{ secrets.IDP_BOT_PRIVATE_KEY }}",
		"E2E_APPROVER_TOKEN":  "${{ secrets.IDP_E2E_APPROVER_TOKEN }}",
	}
	if !maps.Equal(map[string]string(h.Env), want) {
		t.Errorf("harness env = %v, want exactly %v", h.Env, want)
	}
	wantRun(t, "e2e harness", h, `"$RUNNER_TEMP/idp-e2e" --repo "$GITHUB_REPOSITORY_OWNER/idp-claims-e2e" --engine-sha "$GITHUB_SHA" --run "$GITHUB_RUN_ID"`)
	if strings.Contains(h.Run, "go ") {
		t.Errorf("harness run = %q: it runs the binary built without secrets, never the go tool", h.Run)
	}
}

// TestE2ESecretsReachOnlyTheHarness: the bot key and the approver token reach
// the harness step and nothing else. The config step only checks they are set,
// and sees a boolean, never the value. Expression contexts are case-insensitive,
// so the match is too.
func TestE2ESecretsReachOnlyTheHarness(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	refs := func(text string) []string {
		var out []string
		for _, s := range e2eSecrets {
			if strings.Contains(strings.ToLower(text), strings.ToLower(s)) {
				out = append(out, s)
			}
		}
		return out
	}
	for _, k := range slices.Sorted(maps.Keys(j.Env)) {
		if r := refs(j.Env[k]); len(r) > 0 {
			t.Errorf("job env %s reads %v: every step would see it; only the harness step may", k, r)
		}
	}
	for _, s := range j.Steps {
		if s.ID == "harness" {
			continue
		}
		texts := map[string]string{"run": s.Run}
		for k, v := range s.Env {
			texts["env "+k] = v
		}
		for k, v := range s.With {
			texts["with "+k] = v
		}
		for _, where := range slices.Sorted(maps.Keys(texts)) {
			for _, r := range refs(texts[where]) {
				if s.ID == "config" && texts[where] == "${{ "+r+" != '' }}" {
					continue
				}
				t.Errorf("step %q %s reads %s: only the harness step may (the config step only as ${{ %s != '' }})", s.label(), where, r, r)
			}
		}
	}
	_, c := mustStep(t, "e2e", j.Steps, "config")
	wantEnv(t, "e2e config", c, map[string]string{
		"BOT_KEY_SET":  "${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}",
		"APPROVER_SET": "${{ secrets.IDP_E2E_APPROVER_TOKEN != '' }}",
	})
}

func TestE2ETakesTheReleasePRHead(t *testing.T) {
	n, ok := loadWorkflow(t, e2ePath).trigger("workflow_dispatch")
	if !ok {
		t.Fatal("no workflow_dispatch trigger")
	}
	var d struct {
		Inputs map[string]struct {
			Required bool   `yaml:"required"`
			Type     string `yaml:"type"`
			Default  string `yaml:"default"`
		} `yaml:"inputs"`
	}
	if err := n.Decode(&d); err != nil {
		t.Fatal(err)
	}
	in, ok := d.Inputs[releasePRInput]
	if !ok || len(d.Inputs) != 1 || in.Required || in.Type != "string" || in.Default != "" {
		t.Errorf("inputs = %+v, want exactly an optional string %s that defaults to empty, so the nightly and a plain dispatch report nothing", d.Inputs, releasePRInput)
	}
	_, disp := mustStep(t, "e2e-release-pr", mustJob(t, loadWorkflow(t, releasePath), "e2e-release-pr").Steps, "dispatch")
	wantRun(t, "release.yaml dispatch", disp, "-f "+releasePRInput+"=")
}

func TestE2EReportsEveryOutcomeOnTheReleasePR(t *testing.T) {
	j := mustJob(t, loadWorkflow(t, e2ePath), "e2e")
	if len(j.Steps) < 3 || j.Steps[1].ID != "pending" {
		t.Fatal("the step right after the guard must post pending on the release PR, before anything can fail")
	}
	p := j.Steps[1]
	if p.If != "inputs.release-pr-sha != ''" {
		t.Errorf("pending if = %q, want inputs.release-pr-sha != ''", p.If)
	}
	statusEnv := map[string]string{"GH_TOKEN": "${{ github.token }}", "RELEASE_PR_SHA": "${{ inputs.release-pr-sha }}"}
	if !maps.Equal(map[string]string(p.Env), statusEnv) {
		t.Errorf("pending env = %v, want %v", p.Env, statusEnv)
	}
	wantRun(t, "e2e pending", p, `gh api -X POST "repos/$GITHUB_REPOSITORY/statuses/$RELEASE_PR_SHA" -f state=pending -f context=idp-e2e`)
	re := bashMatch(t, "e2e pending", p.Run, "RELEASE_PR_SHA")
	if !re.MatchString(strings.Repeat("a", 40)) || re.MatchString(strings.Repeat("a", 39)) || re.MatchString(strings.Repeat("A", 40)) {
		t.Errorf("release-pr-sha pattern %q must accept only a full lowercase commit SHA", re)
	}
	r := j.Steps[len(j.Steps)-1]
	if r.ID != "result" {
		t.Fatalf("the last step must post the result, got %q", r.label())
	}
	if r.If != "always() && steps.pending.outcome == 'success'" {
		t.Errorf("result if = %q: a failure anywhere after pending, preflight included, must still post failure; a run that never posted pending posts nothing", r.If)
	}
	resultEnv := maps.Clone(statusEnv)
	resultEnv["JOB_STATUS"] = "${{ job.status }}"
	if !maps.Equal(map[string]string(r.Env), resultEnv) {
		t.Errorf("result env = %v, want %v", r.Env, resultEnv)
	}
	wantRun(t, "e2e result", r,
		"state=failure",
		`if [ "$JOB_STATUS" = success ]; then state=success; fi`,
		`gh api -X POST "repos/$GITHUB_REPOSITORY/statuses/$RELEASE_PR_SHA" -f state="$state" -f context=idp-e2e`)
	if h, _ := mustStep(t, "e2e", j.Steps, "harness"); h >= len(j.Steps)-1 {
		t.Error("the result step must come after the harness")
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/ -run TestE2E`
Expected: FAIL: `TestE2EWorkflow` (permissions lack `statuses: write`), `TestE2ETakesTheReleasePRHead` (no inputs), `TestE2EReportsEveryOutcomeOnTheReleasePR` (step 1 is not `pending`).

- [ ] **Step 3: Implement** — replace `.github/workflows/e2e.yaml` with:

```yaml
name: idp-e2e

# End-to-end harness (spec §8.5, ADR-0027): drives idp-claims-e2e through the
# real pipelines at this commit, nightly and on demand from main. release.yaml
# dispatches it for each release PR head (release-pr-sha), and the run reports
# its result on that commit as the status idp-e2e (ADR-0028).

on:
  schedule:
    - cron: '41 3 * * *'
  workflow_dispatch:
    inputs:
      release-pr-sha:
        description: Head commit of an engine release PR. The run reports its result there as the commit status idp-e2e.
        required: false
        type: string
        default: ''

permissions: {}

defaults:
  run:
    shell: bash

concurrency:
  group: idp-e2e
  cancel-in-progress: false

jobs:
  e2e:
    name: e2e
    runs-on: ubuntu-24.04
    # Outlasts the harness's bounded waits (DefaultTimeouts, about 300 minutes
    # in the worst case), so its cleanup always gets to run.
    timeout-minutes: 330
    environment: idp-e2e
    permissions:
      contents: read
      statuses: write
    steps:
      - name: run once
        id: guard
        run: |
          if [ "$GITHUB_RUN_ATTEMPT" != 1 ]; then
            echo "::error::re-running idp-e2e reuses its run id and collides with the first attempt's branches; dispatch a new run (docs/e2e.md)"
            exit 1
          fi
      - name: report pending on the release PR
        id: pending
        if: inputs.release-pr-sha != ''
        env:
          GH_TOKEN: ${{ github.token }}
          RELEASE_PR_SHA: ${{ inputs.release-pr-sha }}
        run: |
          if ! [[ "$RELEASE_PR_SHA" =~ ^[0-9a-f]{40}$ ]]; then
            echo "::error::release-pr-sha must be a full 40-character commit SHA"
            exit 1
          fi
          gh api -X POST "repos/$GITHUB_REPOSITORY/statuses/$RELEASE_PR_SHA" -f state=pending -f context=idp-e2e \
            -f description="the end-to-end harness is running on main" \
            -f target_url="$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID" > /dev/null
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: false
      - name: build the harness
        id: build
        run: go build -o "$RUNNER_TEMP/idp-e2e" ./cmd/idp-e2e
      - name: check the environment
        id: config
        env:
          BOT_CLIENT_ID: ${{ vars.IDP_BOT_CLIENT_ID }}
          BOT_KEY_SET: ${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}
          APPROVER_SET: ${{ secrets.IDP_E2E_APPROVER_TOKEN != '' }}
        run: |
          missing=""
          [ -n "$BOT_CLIENT_ID" ] || missing="$missing variable IDP_BOT_CLIENT_ID,"
          [ "$BOT_KEY_SET" = true ] || missing="$missing secret IDP_BOT_PRIVATE_KEY,"
          [ "$APPROVER_SET" = true ] || missing="$missing secret IDP_E2E_APPROVER_TOKEN,"
          if [ -n "$missing" ]; then
            echo "::error::the idp-e2e environment is missing:${missing%,} (docs/e2e.md)"
            exit 1
          fi
      # The harness mints the bot's installation tokens from the App key and
      # renews them before the hour runs out, each scoped to idp-claims-e2e and
      # the permissions the forge calls. The key reaches only this step.
      - name: run the harness
        id: harness
        env:
          E2E_BOT_CLIENT_ID: ${{ vars.IDP_BOT_CLIENT_ID }}
          E2E_BOT_PRIVATE_KEY: ${{ secrets.IDP_BOT_PRIVATE_KEY }}
          E2E_APPROVER_TOKEN: ${{ secrets.IDP_E2E_APPROVER_TOKEN }}
        run: |
          "$RUNNER_TEMP/idp-e2e" --repo "$GITHUB_REPOSITORY_OWNER/idp-claims-e2e" --engine-sha "$GITHUB_SHA" --run "$GITHUB_RUN_ID"
      # Runs whenever pending was posted, whatever failed in between.
      - name: report the result on the release PR
        id: result
        if: always() && steps.pending.outcome == 'success'
        env:
          GH_TOKEN: ${{ github.token }}
          RELEASE_PR_SHA: ${{ inputs.release-pr-sha }}
          JOB_STATUS: ${{ job.status }}
        run: |
          state=failure
          if [ "$JOB_STATUS" = success ]; then state=success; fi
          gh api -X POST "repos/$GITHUB_REPOSITORY/statuses/$RELEASE_PR_SHA" -f state="$state" -f context=idp-e2e \
            -f description="the end-to-end harness on main: $JOB_STATUS" \
            -f target_url="$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID" > /dev/null
```

`docs/e2e.md` (exact replacements with Edit):

1. In "When it runs", old:

```markdown
Nightly at 03:41 UTC and on demand: `gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main`. One run at a time. Do not commit to `idp-claims-e2e` while a run is in progress.
```

new:

```markdown
Nightly at 03:41 UTC, for every engine release PR (below), and on demand: `gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main`. One run at a time. Do not commit to `idp-claims-e2e` while a run is in progress.
```

2. Insert this section right before `## Credentials`:

````markdown
## On release pull requests

When release-please creates or updates the engine's release PR, `release.yaml` dispatches this workflow on `main` with `release-pr-sha` set to the PR's head commit ([ADR-0028](adr/0028-releases-and-how-the-workflows-get-idp.md)). `main` is the release PR's base: the same code without the changelog. Right after the re-run guard, the run posts the commit status `idp-e2e` as `pending` on that commit; its last step posts `success` or `failure`, also when an earlier step failed. Merge a release PR only when its `idp-e2e` status is green ([releases runbook](runbooks/releases.md)).

A dispatch waits in the `idp-e2e` concurrency group while another run is in progress, and GitHub keeps only the newest waiting run of a group: a nightly run or a newer dispatch can replace it. The replaced run never starts, so the release PR shows no `idp-e2e` status. Dispatch it again for the current head:

```bash
head="$(gh pr view <number> --repo jellalshadows-idp/idp-engine --json headRefOid --jq .headRefOid)"
gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main -f release-pr-sha="$head"
```
````

3. In "Credentials", after the paragraph that starts "An installation token lives one hour", add:

```markdown
The run's own `GITHUB_TOKEN` has `statuses: write`, used only by the two steps that post `idp-e2e` on a release PR head; the harness step never sees it.
```

- [ ] **Step 4: Verify** — `go test ./... && go vet ./... && gofmt -l .`; actionlint (no output); zizmor as in Task 3 (no unsuppressed findings).

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/e2e.yaml internal/workflowspec/e2e_test.go docs/e2e.md
git commit -m "ci(e2e): run the harness for release pull requests and report idp-e2e"
```

---

### Task 6: `setup-idp` installs the verified release of a release commit

**Files:**
- Modify: `.github/actions/setup-idp/action.yaml`, `internal/workflowspec/setupidp_test.go`, `internal/workflowspec/load_test.go`, `internal/workflowspec/helpers_test.go`, `docs/pipelines.md`

**Interfaces:**
- Consumes: `loadAction`, `mustStep`, `wantRun`, `bashMatch`, `ciTofuVersion`, `setupIDPPath`, `version.Repo`; Task 3's `loadGoreleaser`, `archiveTemplate`; Task 4's `releasePath`.
- Produces: the composite step `download` (output `installed`, `true` or `false`); `const notInstalled = "steps.download.outputs.installed != 'true'"` (the `if` of steps `go` and `build`); in `load_test.go`, `func ifBody(t *testing.T, where, run, prefix string) string`.

- [ ] **Step 1: Write the failing tests**

`internal/workflowspec/helpers_test.go`, add:

```go
func TestIfBody(t *testing.T) {
	run := "a\nif ! b; then\n  echo x\n  exit 1\nfi\nc\n"
	if got := ifBody(t, "inline", run, "if ! b"); got != "  echo x\n  exit 1" {
		t.Errorf("ifBody = %q", got)
	}
}
```

Replace `internal/workflowspec/setupidp_test.go` with:

```go
package workflowspec

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

// notInstalled is the condition of the steps that build idp from source: they
// run only when the download step installed no release (ADR-0028).
const notInstalled = "steps.download.outputs.installed != 'true'"

func TestSetupIDPInterface(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	if a.Runs.Using != "composite" {
		t.Fatalf("runs.using = %q, want composite", a.Runs.Using)
	}
	if len(a.Inputs) != 2 || !a.Inputs["engine-dir"].Required || !a.Inputs["version"].Required {
		t.Errorf("inputs = %v, want exactly engine-dir and version, both required", a.Inputs)
	}
}

func TestSetupIDPInstallsTheToolchain(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	_, goStep := mustStep(t, setupIDPPath, a.Runs.Steps, "go")
	_, tofuStep := mustStep(t, setupIDPPath, a.Runs.Steps, "tofu")
	if !strings.HasPrefix(goStep.Uses, "actions/setup-go@") || !strings.HasPrefix(tofuStep.Uses, "opentofu/setup-opentofu@") {
		t.Fatalf("steps go and tofu must use setup-go and setup-opentofu, got %q and %q", goStep.Uses, tofuStep.Uses)
	}
	if goStep.If != notInstalled {
		t.Errorf("setup-go if = %q, want %q: Go is needed only for a source build", goStep.If, notInstalled)
	}
	if tofuStep.If != "" {
		t.Errorf("setup-opentofu if = %q: every pipeline runs tofu, whatever installed idp", tofuStep.If)
	}
	if got := goStep.With["go-version-file"]; got != "${{ inputs.engine-dir }}/go.mod" {
		t.Errorf("setup-go go-version-file = %q", got)
	}
	if goStep.With["cache"] != "false" {
		t.Errorf("setup-go must set cache: false: a cache restored into the job that holds the writer key is a poisoning path (ADR-0021)")
	}
	if tofuStep.With["tofu_wrapper"] != "false" {
		t.Errorf("setup-opentofu must set tofu_wrapper: false")
	}
	if got, want := tofuStep.With["tofu_version"], ciTofuVersion(t); got != want {
		t.Errorf("setup-idp installs OpenTofu %q, CI tests the modules with %q: keep one version", got, want)
	}
}

// ciTofuVersion is the OpenTofu version CI tests the modules with.
func ciTofuVersion(t *testing.T) string {
	t.Helper()
	w := loadWorkflow(t, ".github/workflows/ci.yaml")
	for _, s := range mustJob(t, w, "modules").Steps {
		if strings.HasPrefix(s.Uses, "opentofu/setup-opentofu@") {
			return s.With["tofu_version"]
		}
	}
	t.Fatal("ci.yaml job modules has no setup-opentofu step")
	return ""
}

func TestSetupIDPBuildsTheGivenCommit(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	_, build := mustStep(t, setupIDPPath, a.Runs.Steps, "build")
	if build.If != notInstalled {
		t.Errorf("build if = %q, want %q", build.If, notInstalled)
	}
	if build.Shell != "bash" {
		t.Errorf("build shell = %q, want bash", build.Shell)
	}
	if build.WorkingDirectory != "${{ inputs.engine-dir }}" {
		t.Errorf("build working-directory = %q", build.WorkingDirectory)
	}
	if build.Env["IDP_VERSION"] != "${{ inputs.version }}" {
		t.Errorf("build env IDP_VERSION = %q", build.Env["IDP_VERSION"])
	}
	wantRun(t, "setup-idp build", build,
		`[[ "$IDP_VERSION" =~ ^[0-9a-f]{40}$ ]]`,
		`-X `+version.Repo+`/internal/version.Version=${IDP_VERSION}`,
		`-o "$RUNNER_TEMP/idp-bin/idp" ./cmd/idp`,
		`echo "$RUNNER_TEMP/idp-bin" >> "$GITHUB_PATH"`,
	)
}

// engineRepo is the engine's owner/name; the download looks up only its
// releases, like the module sources the renderer writes.
var engineRepo = strings.TrimPrefix(version.Repo, "github.com/")

func downloadStep(t *testing.T) (int, step) {
	t.Helper()
	return mustStep(t, setupIDPPath, loadAction(t, setupIDPPath).Runs.Steps, "download")
}

func TestSetupIDPDownloadsAndVerifiesTheRelease(t *testing.T) {
	a := loadAction(t, setupIDPPath)
	di, dl := downloadStep(t)
	gi, _ := mustStep(t, setupIDPPath, a.Runs.Steps, "go")
	bi, _ := mustStep(t, setupIDPPath, a.Runs.Steps, "build")
	if di != 0 || gi < di || bi < di {
		t.Errorf("steps: download %d, go %d, build %d; the download decides first", di, gi, bi)
	}
	if dl.Shell != "bash" || dl.WorkingDirectory != "" || dl.If != "" {
		t.Errorf("download shell %q working-directory %q if %q, want bash, none, none", dl.Shell, dl.WorkingDirectory, dl.If)
	}
	if want := map[string]string{"GH_TOKEN": "${{ github.token }}", "IDP_VERSION": "${{ inputs.version }}"}; !maps.Equal(map[string]string(dl.Env), want) {
		t.Errorf("download env = %v, want %v: the job's own token reads the public engine's releases, set on this step only (ADR-0023)", dl.Env, want)
	}
	wantRun(t, "setup-idp download", dl,
		`if ! listing="$(gh api --paginate "repos/`+engineRepo+`/git/matching-refs/tags/v" --jq '.[] | [.ref, .object.type, .object.sha] | @tsv')"; then`,
		`if [[ "$type" == commit && "$sha" == "$IDP_VERSION" && "$ref" =~ `,
		`archive="idp_${tag#v}_linux_${arch}.tar.gz"`,
		`gh release download "$tag" --repo `+engineRepo+` --pattern "$archive" --pattern checksums.txt --dir "$dl"`,
		`if ! verified="$(cd "$dl" && sha256sum --check --ignore-missing checksums.txt)" || [ "$verified" != "$archive: OK" ]; then`,
		`gh attestation verify "$dl/$archive" --repo `+engineRepo+` --signer-workflow `+engineRepo+"/"+releasePath+` --source-digest "$IDP_VERSION" --source-ref refs/heads/main`,
		`tar -xzf "$dl/$archive" -C "$RUNNER_TEMP/idp-bin" idp`,
		`got="$("$RUNNER_TEMP/idp-bin/idp" version)"`,
		`if [ "$got" != "$tag" ]; then`,
		`echo "$RUNNER_TEMP/idp-bin" >> "$GITHUB_PATH"`,
		`echo "installed=true" >> "$GITHUB_OUTPUT"`,
	)
	if got := loadGoreleaser(t).Archives[0].NameTemplate; got != archiveTemplate {
		t.Errorf("GoReleaser names archives %q; the download expects %q", got, archiveTemplate)
	}
}

func TestSetupIDPFailsWhenTheTagListingFails(t *testing.T) {
	_, dl := downloadStep(t)
	body := ifBody(t, "setup-idp download", dl.Run, `if ! listing=`)
	if !strings.Contains(body, "::error::") || !strings.Contains(body, "exit 1") || strings.Contains(body, "installed=") {
		t.Errorf("a failed tag listing must fail the step, never decide: got %q", body)
	}
	if strings.Index(dl.Run, `echo "installed=false"`) < strings.Index(dl.Run, `if ! listing=`) {
		t.Error("installed=false may be decided only after the listing succeeded")
	}
	a := loadAction(t, setupIDPPath)
	for _, id := range []string{"go", "build"} {
		_, s := mustStep(t, setupIDPPath, a.Runs.Steps, id)
		for _, f := range []string{"always()", "failure()", "cancelled()"} {
			if strings.Contains(s.If, f) {
				t.Errorf("step %s if = %q: after a failed download it must not run", id, s.If)
			}
		}
	}
}

func TestSetupIDPNeverBuildsATaggedCommit(t *testing.T) {
	_, dl := downloadStep(t)
	body := ifBody(t, "setup-idp download", dl.Run, `if ! gh release download "$tag"`)
	for _, want := range []string{"exit 1", "never falls back to a source build", "docs/runbooks/releases.md"} {
		if !strings.Contains(body, want) {
			t.Errorf("a release without its assets must fail with an actionable message containing %q, got %q", want, body)
		}
	}
	if n := strings.Count(dl.Run, `echo "installed=false" >> "$GITHUB_OUTPUT"`); n != 1 {
		t.Errorf("installed=false is written %d times, want once: only for a commit without a release tag", n)
	}
	i := strings.Index(dl.Run, `tag="${tags[0]}"`)
	if i < 0 {
		t.Fatal(`no tag="${tags[0]}" line`)
	}
	if strings.Contains(dl.Run[i:], "installed=false") {
		t.Error("once a release tag is found, the step may only install or fail")
	}
}

func TestSetupIDPIgnoresNonReleaseTags(t *testing.T) {
	_, dl := downloadStep(t)
	re := bashMatch(t, "setup-idp download", dl.Run, "ref")
	for _, ref := range []string{"refs/tags/v0.1.0", "refs/tags/v12.3.45"} {
		if !re.MatchString(ref) {
			t.Errorf("the lookup ignores %q, a release tag", ref)
		}
	}
	for _, ref := range []string{"refs/tags/v0.1.0-rc.1", "refs/tags/0.1.0", "refs/tags/v0.1", "refs/tags/idp-v0.1.0", "refs/heads/v0.1.0", "refs/tags/v0.1.0/x"} {
		if re.MatchString(ref) {
			t.Errorf("the lookup takes %q as a release: its commit would wait for archives that never exist", ref)
		}
	}
	wantRun(t, "setup-idp download", dl,
		`if [ "${#tags[@]}" -gt 1 ]; then`,
		`echo "::error::setup-idp: $IDP_VERSION carries several release tags (${tags[*]}); a release commit has exactly one"`)
}

func TestSetupIDPPicksTheRunnerArchive(t *testing.T) {
	_, dl := downloadStep(t)
	wantRun(t, "setup-idp download", dl,
		`X64) arch=amd64 ;;`,
		`ARM64) arch=arm64 ;;`,
		`*) echo "::error::setup-idp: idp releases have no linux archive for runner architecture $RUNNER_ARCH"; exit 1 ;;`)
	b := loadGoreleaser(t).Builds[0]
	if !slices.Contains(b.Goos, "linux") || !slices.Contains(b.Goarch, "amd64") || !slices.Contains(b.Goarch, "arm64") {
		t.Errorf("GoReleaser builds goos %v goarch %v; the download needs linux on amd64 and arm64", b.Goos, b.Goarch)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/`
Expected: FAIL to compile (`undefined: ifBody`); once Step 3's helper exists, `TestSetupIDP*` fail (`no step with id "download"`, `setup-go if = ""`).

- [ ] **Step 3: Implement**

`internal/workflowspec/load_test.go`, after `bashMatch`:

```go
// ifBody returns the lines of a run script between the line that starts with
// prefix and the next line that is exactly "fi": the body of a top-level if.
func ifBody(t *testing.T, where, run, prefix string) string {
	t.Helper()
	ls := strings.Split(run, "\n")
	for i, l := range ls {
		if !strings.HasPrefix(l, prefix) {
			continue
		}
		for j := i + 1; j < len(ls); j++ {
			if ls[j] == "fi" {
				return strings.Join(ls[i+1:j], "\n")
			}
		}
		t.Fatalf("%s: the block that starts with %q has no closing fi", where, prefix)
	}
	t.Fatalf("%s: no line starts with %q", where, prefix)
	return ""
}
```

Replace `.github/actions/setup-idp/action.yaml` with:

```yaml
name: setup-idp
description: Install OpenTofu and the idp CLI for an idp-engine checkout. A release commit installs its release binary after checking its checksum and build provenance; any other commit is built from source (ADR-0021, ADR-0028).

inputs:
  engine-dir:
    description: The idp-engine checkout, relative to the workspace.
    required: true
  version:
    description: The commit of that checkout, 40 hex characters. A release commit installs its release, whose binary carries the tag; any other commit is built with the commit embedded. Rendered module sources pin that version.
    required: true

runs:
  using: composite
  steps:
    - id: download
      name: download and verify the release
      shell: bash
      env:
        GH_TOKEN: ${{ github.token }}
        IDP_VERSION: ${{ inputs.version }}
      run: | # zizmor: ignore[github-env] appends a fixed directory under RUNNER_TEMP to GITHUB_PATH; no input reaches the value
        # A release commit installs its release: the archive is checked against
        # the release's checksums.txt and its provenance against this commit
        # before it runs (ADR-0028). Any other commit is built from source.
        if ! listing="$(gh api --paginate "repos/jellalshadows-idp/idp-engine/git/matching-refs/tags/v" --jq '.[] | [.ref, .object.type, .object.sha] | @tsv')"; then
          echo "::error::setup-idp: could not list the idp-engine release tags, so it cannot tell whether $IDP_VERSION is a release; re-run the job"
          exit 1
        fi
        tags=()
        while IFS=$'\t' read -r ref type sha; do
          if [[ "$type" == commit && "$sha" == "$IDP_VERSION" && "$ref" =~ ^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            tags+=("${ref#refs/tags/}")
          fi
        done <<< "$listing"
        if [ "${#tags[@]}" -eq 0 ]; then
          echo "setup-idp: $IDP_VERSION is not a release; building idp from source"
          echo "installed=false" >> "$GITHUB_OUTPUT"
          exit 0
        fi
        if [ "${#tags[@]}" -gt 1 ]; then
          echo "::error::setup-idp: $IDP_VERSION carries several release tags (${tags[*]}); a release commit has exactly one"
          exit 1
        fi
        tag="${tags[0]}"
        case "$RUNNER_ARCH" in
          X64) arch=amd64 ;;
          ARM64) arch=arm64 ;;
          *) echo "::error::setup-idp: idp releases have no linux archive for runner architecture $RUNNER_ARCH"; exit 1 ;;
        esac
        archive="idp_${tag#v}_linux_${arch}.tar.gz"
        dl="$RUNNER_TEMP/idp-dl"
        rm -rf "$dl"
        mkdir -p "$dl"
        if ! gh release download "$tag" --repo jellalshadows-idp/idp-engine --pattern "$archive" --pattern checksums.txt --dir "$dl" || [ ! -f "$dl/$archive" ] || [ ! -f "$dl/checksums.txt" ]; then
          echo "::error::setup-idp: $IDP_VERSION is release $tag, but $archive and checksums.txt could not be downloaded from it. A release commit never falls back to a source build: finish the release's publish job or pin another release (docs/runbooks/releases.md)"
          exit 1
        fi
        if ! verified="$(cd "$dl" && sha256sum --check --ignore-missing checksums.txt)" || [ "$verified" != "$archive: OK" ]; then
          echo "::error::setup-idp: $archive does not match the checksums.txt of release $tag"
          exit 1
        fi
        if ! gh attestation verify "$dl/$archive" --repo jellalshadows-idp/idp-engine --signer-workflow jellalshadows-idp/idp-engine/.github/workflows/release.yaml --source-digest "$IDP_VERSION" --source-ref refs/heads/main; then
          echo "::error::setup-idp: the provenance of $archive does not show that release.yaml built it on main from $IDP_VERSION"
          exit 1
        fi
        mkdir -p "$RUNNER_TEMP/idp-bin"
        tar -xzf "$dl/$archive" -C "$RUNNER_TEMP/idp-bin" idp
        got="$("$RUNNER_TEMP/idp-bin/idp" version)"
        if [ "$got" != "$tag" ]; then
          echo "::error::setup-idp: the idp binary of release $tag prints version $got"
          exit 1
        fi
        echo "$RUNNER_TEMP/idp-bin" >> "$GITHUB_PATH"
        echo "installed=true" >> "$GITHUB_OUTPUT"
        echo "setup-idp: installed idp $tag, verified against $IDP_VERSION"
    - id: go
      if: steps.download.outputs.installed != 'true'
      uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
      with:
        go-version-file: ${{ inputs.engine-dir }}/go.mod
        cache: false
    - id: tofu
      uses: opentofu/setup-opentofu@a1320f892987e89d278cc92dc5adc984fb93aca4 # v2.0.2
      with:
        tofu_version: 1.12.6
        tofu_wrapper: false
    - id: build
      name: build idp
      if: steps.download.outputs.installed != 'true'
      shell: bash
      working-directory: ${{ inputs.engine-dir }}
      env:
        IDP_VERSION: ${{ inputs.version }}
      run: | # zizmor: ignore[github-env] appends a fixed directory under RUNNER_TEMP to GITHUB_PATH; no input reaches the value
        if ! [[ "$IDP_VERSION" =~ ^[0-9a-f]{40}$ ]]; then
          echo "::error::setup-idp: the version input must be a full 40-character commit SHA"
          exit 1
        fi
        mkdir -p "$RUNNER_TEMP/idp-bin"
        go build -trimpath -ldflags "-X github.com/jellalshadows-idp/idp-engine/internal/version.Version=${IDP_VERSION}" -o "$RUNNER_TEMP/idp-bin/idp" ./cmd/idp
        echo "$RUNNER_TEMP/idp-bin" >> "$GITHUB_PATH"
```

`docs/pipelines.md` (exact replacements with Edit):

1. Old:

```markdown
## How the workflows get idp

Each job checks out the engine at the reusable workflow's own commit and builds `idp` from it with the `setup-idp` composite action, so the workflow, the CLI and the rendered module refs are one version ([ADR-0021](adr/0021-reusable-workflows-build-idp-from-their-commit.md)).
```

new:

````markdown
## How the workflows get idp

Each job checks out the engine at the reusable workflow's own commit (`job.workflow_sha`, the commit the caller pinned) and runs the `setup-idp` composite action. It installs OpenTofu, then gets `idp` one of two ways, so the workflow, the CLI and the rendered module refs are always one version ([ADR-0021](adr/0021-reusable-workflows-build-idp-from-their-commit.md), [ADR-0028](adr/0028-releases-and-how-the-workflows-get-idp.md)):

- **The commit is a release** (a `vX.Y.Z` tag points at it). It downloads the release's linux archive for the runner and `checksums.txt`, checks the checksum, verifies the build provenance with `gh attestation verify` (built by `release.yaml` on `main` from that very commit), checks that `idp version` prints the tag, and installs it. Module sources pin the tag. If any of this fails, the job fails: a release commit never falls back to a build, because the render would then pin the commit and a broken release would go unnoticed.
- **Any other commit** (the end-to-end harness's commit under test, or an unreleased pin). It installs Go from the engine's `go.mod` and builds `idp` from the checkout. Module sources pin the commit.

The tag lookup and the download use the job's own `GITHUB_TOKEN`; the engine is public, so `contents: read` is enough. Nothing is cached.

## Upgrading the engine

Engine releases are `vX.Y.Z` tags ([releases runbook](runbooks/releases.md)). A claims repo upgrades by re-pinning its four caller workflows:

1. Re-pin all four together, to the commit the new tag points at, with the tag as the comment: `@<commit> # vX.Y.Z`. Renovate opens this PR (group "idp engine", [dependencies runbook](runbooks/dependencies.md)); by hand, the commit is `gh api repos/jellalshadows-idp/idp-engine/git/ref/tags/vX.Y.Z --jq .object.sha`.
2. The PR's plan shows the upgrade: every module source moves to the new `?ref=`. Merge it when the plan is what you expect.
3. Re-run `idp bootstrap apply` for the claims repo when the release notes say so, after the re-pin merges, never before.

**From an engine older than commit `3b9beed` (the end-to-end harness) to `v0.1.0` or later:** the newer engine writes `IDP_BOOTSTRAP` with `approverIds`, which the older engine's parser rejects, while the newer parser still reads the older `approverId` form. So merge the re-pin first, then run `idp bootstrap apply` and `check` for that claims repo ([bootstrap runbook](runbooks/bootstrap.md)). Until `apply` runs, the drift check reports `IDP_BOOTSTRAP` as drift.
````

2. In "Limitations", add at the end:

```markdown
- A job of a release commit lists the engine's tags through the GitHub API before it can run; when that call fails, the job fails rather than guess.
```

- [ ] **Step 4: Verify** — `go test ./... && go vet ./... && gofmt -l .`; actionlint (no output); zizmor as in Task 3 (the `download` step carries the same `github-env` ignore as `build`; no other unsuppressed finding). The download path itself is proven live in Task 10.

- [ ] **Step 5: Commit**

```bash
git add .github/actions/setup-idp/action.yaml internal/workflowspec docs/pipelines.md
git commit -m "feat(setup-idp): install the verified release binary of a release commit"
```

---

### Task 7: The harness pins the commit under test as `# unreleased`

**Files:**
- Modify: `internal/e2e/pin.go`, `internal/e2e/pin_test.go`, `internal/e2e/scenario_test.go`, `docs/e2e.md`

**Interfaces:**
- Consumes: `PinTemplates(templates map[string][]byte, sha string) (map[string][]byte, error)`, `testTemplates()`, `TestScenarioPinsTheCallersFirst`.
- Produces: `PinTemplates` rewrites every `idp-engine/.github/workflows/<x>.yaml@<sha>[ # <comment>]` to `…@<sha under test> # unreleased`; same signature.

- [ ] **Step 1: Write the failing tests**

Replace `internal/e2e/pin_test.go` with:

```go
package e2e

import (
	"strings"
	"testing"
)

func TestPinTemplates(t *testing.T) {
	sha := strings.Repeat("b", 40)
	ref := "    uses: jellalshadows-idp/idp-engine/.github/workflows/pr.yaml@"
	want := ref + sha + " # unreleased\n"
	for name, in := range map[string]string{
		"a release pin":     ref + strings.Repeat("0", 40) + " # v0.1.0\n",
		"an unreleased pin": ref + strings.Repeat("0", 40) + " # unreleased\n",
		"no comment":        ref + strings.Repeat("0", 40) + "\n",
	} {
		out, err := PinTemplates(map[string][]byte{"pr.yaml": []byte(in)}, sha)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := string(out["pr.yaml"]); got != want {
			t.Errorf("%s: pinned = %q, want %q: the commit under test is usually not a release", name, got, want)
		}
	}
	in := map[string][]byte{"pr.yaml": []byte(ref + strings.Repeat("0", 40) + " # v0.1.0\n")}
	if _, err := PinTemplates(in, "abc"); err == nil {
		t.Error("a short SHA must be rejected")
	}
	if _, err := PinTemplates(map[string][]byte{"x.yaml": []byte("jobs: {}\n")}, sha); err == nil {
		t.Error("a template without an engine workflow ref must be rejected")
	}
}
```

`internal/e2e/scenario_test.go`, in `testTemplates`, old:

```go
		t[n] = []byte("jobs:\n  x:\n    uses: jellalshadows-idp/idp-engine/.github/workflows/" + n + "@" + strings.Repeat("0", 40) + " # unreleased\n")
```

new (the templates now pin a release, as they will after Task 11):

```go
		t[n] = []byte("jobs:\n  x:\n    uses: jellalshadows-idp/idp-engine/.github/workflows/" + n + "@" + strings.Repeat("0", 40) + " # v0.1.0\n")
```

`TestScenarioPinsTheCallersFirst` already requires `@bbbb… # unreleased` on `main` after the pin.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/e2e/ -run 'TestPinTemplates|TestScenarioPinsTheCallersFirst'`
Expected: FAIL: "a release pin" and "no comment" keep their comment; `TestScenarioPinsTheCallersFirst` finds `# v0.1.0`.

- [ ] **Step 3: Implement** — replace `internal/e2e/pin.go` with:

```go
package e2e

import (
	"fmt"
	"regexp"
)

var (
	shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	// enginePin is an engine workflow ref and its trailing comment, if any.
	enginePin = regexp.MustCompile(`(idp-engine/\.github/workflows/[a-z]+\.yaml@)[0-9a-f]{40}(?: #[^\n]*)?`)
)

// PinTemplates returns the caller templates with every engine workflow ref
// pinned to sha and marked "# unreleased": the harness pins the commit under
// test, which is usually not a release, whatever release the templates pin.
func PinTemplates(templates map[string][]byte, sha string) (map[string][]byte, error) {
	if !shaPattern.MatchString(sha) {
		return nil, fmt.Errorf("engine sha %q is not a 40-character commit SHA", sha)
	}
	out := make(map[string][]byte, len(templates))
	for name, b := range templates {
		if !enginePin.Match(b) {
			return nil, fmt.Errorf("template %s has no engine workflow ref", name)
		}
		out[name] = enginePin.ReplaceAll(b, []byte("${1}"+sha+" # unreleased"))
	}
	return out, nil
}
```

`docs/e2e.md`, in "What a run does", old:

```markdown
1. Pins the four caller workflows of `idp-claims-e2e` to the commit under test (a PR, when they differ).
```

new:

```markdown
1. Pins the four caller workflows of `idp-claims-e2e` to the commit under test, marked `# unreleased` because that commit is usually not a release (a PR, when they differ).
```

- [ ] **Step 4: Verify** — `go test ./internal/e2e/` passes (every scenario test), then `go test ./... && go vet ./... && gofmt -l .`.

- [ ] **Step 5: Commit**

```bash
git add internal/e2e docs/e2e.md
git commit -m "fix(e2e): pin the callers as unreleased whatever release the templates pin"
```

---

### Task 8: Self-hosted Renovate

**Files:**
- Create: `.github/workflows/renovate.yaml`, `.github/renovate-global.json`, `internal/workflowspec/renovate_test.go`, `docs/runbooks/dependencies.md`
- Modify: `.github/workflows/ci.yaml` (job `workflows`), `internal/workflowspec/load_test.go`, `docs/runbooks/bootstrap.md`

**Interfaces:**
- Consumes: `loadWorkflow`, `mustJob`, `mustStep`, `wantPerms`, `wantEnv`, `wantRun`, `usesRefs`, `readYAML`, `templateNames`, `templatesDir`, `ciTofuVersion`, `repoRoot`; Task 4's `secretReaders`, `strictJSON`; `render.TofuVersion`, `render.GitHubProviderVersion`.
- Produces: `const renovatePath = ".github/workflows/renovate.yaml"`, `const renovateConfigPath = ".github/renovate-global.json"`, `const botGitAuthor`; `type renovateConfig`, `type renovateRule`, `type customManager`; `func loadRenovateConfig(t *testing.T) renovateConfig`; `func renovateVersion(t *testing.T) string`; in `load_test.go`, `func repoFiles(t *testing.T) []string`; `internalWorkflows` gains `renovate.yaml`; the `ci.yaml` step `renovate-config` in job `workflows`.

- [ ] **Step 1: Write the failing tests** — `internal/workflowspec/renovate_test.go`:

```go
package workflowspec

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/render"
)

const (
	renovatePath       = ".github/workflows/renovate.yaml"
	renovateConfigPath = ".github/renovate-global.json"
	// botGitAuthor is the bot App's identity; GitHub authors its API commits as it.
	botGitAuthor = "jellalshadows-idp-bot[bot] <343234638+jellalshadows-idp-bot[bot]@users.noreply.github.com>"
)

// renovateConfig is .github/renovate-global.json. Decoding refuses unknown keys,
// so every option in the file is one this contract knows.
type renovateConfig struct {
	Platform             string             `json:"platform"`
	Repositories         []string           `json:"repositories"`
	Onboarding           *bool              `json:"onboarding"`
	RequireConfig        string             `json:"requireConfig"`
	PlatformCommit       string             `json:"platformCommit"`
	GitAuthor            string             `json:"gitAuthor"`
	DependencyDashboard  *bool              `json:"dependencyDashboard"`
	Automerge            *bool              `json:"automerge"`
	MinimumReleaseAge    string             `json:"minimumReleaseAge"`
	InternalChecksFilter string             `json:"internalChecksFilter"`
	StatusCheckNames     map[string]*string `json:"statusCheckNames"`
	EnabledManagers      []string           `json:"enabledManagers"`
	PackageRules         []renovateRule     `json:"packageRules"`
	CustomManagers       []customManager    `json:"customManagers"`
}

type renovateRule struct {
	Description       string          `json:"description"`
	MatchPackageNames []string        `json:"matchPackageNames"`
	MatchDepNames     []string        `json:"matchDepNames"`
	MatchDepTypes     []string        `json:"matchDepTypes"`
	MatchManagers     []string        `json:"matchManagers"`
	MatchDatasources  []string        `json:"matchDatasources"`
	MatchUpdateTypes  []string        `json:"matchUpdateTypes"`
	GroupName         string          `json:"groupName"`
	Labels            []string        `json:"labels"`
	RegistryURLs      []string        `json:"registryUrls"`
	PostUpdateOptions []string        `json:"postUpdateOptions"`
	RangeStrategy     string          `json:"rangeStrategy"`
	Enabled           *bool           `json:"enabled"`
	MinimumReleaseAge json.RawMessage `json:"minimumReleaseAge"` // absent (nil), null, or a duration
}

type customManager struct {
	CustomType             string   `json:"customType"`
	Description            string   `json:"description"`
	ManagerFilePatterns    []string `json:"managerFilePatterns"`
	MatchStrings           []string `json:"matchStrings"`
	DatasourceTemplate     string   `json:"datasourceTemplate"`
	DepNameTemplate        string   `json:"depNameTemplate"`
	PackageNameTemplate    string   `json:"packageNameTemplate"`
	VersioningTemplate     string   `json:"versioningTemplate"`
	ExtractVersionTemplate string   `json:"extractVersionTemplate"`
	RegistryURLTemplate    string   `json:"registryUrlTemplate"`
}

func loadRenovateConfig(t *testing.T) renovateConfig {
	t.Helper()
	var c renovateConfig
	strictJSON(t, renovateConfigPath, &c)
	return c
}

// renovateVersion is the Renovate version renovate.yaml runs.
func renovateVersion(t *testing.T) string {
	t.Helper()
	_, r := mustStep(t, "renovate", mustJob(t, loadWorkflow(t, renovatePath), "renovate").Steps, "renovate")
	v := r.With["renovate-version"]
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(v) {
		t.Fatalf("renovate-version = %q, want an exact X.Y.Z (spec §7.5)", v)
	}
	return v
}

func TestRenovateWorkflow(t *testing.T) {
	w := loadWorkflow(t, renovatePath)
	if got := w.triggerNames(); !slices.Equal(got, []string{"schedule", "workflow_dispatch"}) {
		t.Errorf("triggers = %v, want schedule and workflow_dispatch", got)
	}
	n, ok := w.trigger("schedule")
	if !ok {
		t.Fatal("no schedule")
	}
	var crons []struct {
		Cron string `yaml:"cron"`
	}
	if err := n.Decode(&crons); err != nil || len(crons) != 1 || crons[0].Cron != "17 4 * * 1" {
		t.Errorf("schedule = %v (%v), want weekly '17 4 * * 1' (owner decision)", crons, err)
	}
	if w.Concurrency == nil || *w.Concurrency != (concurrency{Group: "idp-renovate"}) {
		t.Errorf("concurrency = %+v, want group idp-renovate without cancellation", w.Concurrency)
	}
	if got := slices.Sorted(maps.Keys(w.Jobs)); !slices.Equal(got, []string{"renovate"}) {
		t.Errorf("jobs = %v, want renovate", got)
	}
	j := mustJob(t, w, "renovate")
	if j.Environment != "idp-renovate" {
		t.Errorf("environment = %q, want idp-renovate: the bot key lives only in idp-engine environments limited to main (ADR-0026)", j.Environment)
	}
	wantPerms(t, "renovate", j.Permissions, map[string]string{"contents": "read"})
	_, bot := mustStep(t, "renovate", j.Steps, "bot")
	want := map[string]string{
		"client-id":                "${{ vars.IDP_BOT_CLIENT_ID }}",
		"private-key":              "${{ secrets.IDP_BOT_PRIVATE_KEY }}",
		"owner":                    "jellalshadows-idp",
		"repositories":             "idp-engine,idp-claims",
		"permission-checks":        "read",
		"permission-contents":      "write",
		"permission-issues":        "write",
		"permission-pull-requests": "write",
		"permission-statuses":      "read",
		"permission-workflows":     "write",
	}
	if !strings.HasPrefix(bot.Uses, "actions/create-github-app-token@") || !maps.Equal(map[string]string(bot.With), want) {
		t.Errorf("bot step = %q %v, want create-github-app-token with %v", bot.Uses, bot.With, want)
	}
	_, r := mustStep(t, "renovate", j.Steps, "renovate")
	if !strings.HasPrefix(r.Uses, "renovatebot/github-action@") {
		t.Errorf("renovate step uses %q", r.Uses)
	}
	if want := map[string]string{"configurationFile": renovateConfigPath, "renovate-version": renovateVersion(t), "token": "${{ steps.bot.outputs.token }}"}; !maps.Equal(map[string]string(r.With), want) {
		t.Errorf("renovate with = %v, want %v", r.With, want)
	}
	if got := secretReaders(j, "IDP_BOT_PRIVATE_KEY"); !slices.Equal(got, []string{"bot"}) {
		t.Errorf("steps reading the bot key = %v, want only the token step", got)
	}
	_, c := mustStep(t, "renovate", j.Steps, "config")
	wantEnv(t, "renovate config", c, map[string]string{
		"BOT_CLIENT_ID": "${{ vars.IDP_BOT_CLIENT_ID }}",
		"BOT_KEY_SET":   "${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}",
	})
	wantRun(t, "renovate config", c, `echo "::error::the idp-renovate environment is missing:${missing%,} (docs/runbooks/dependencies.md)"`)
}

func TestRenovateCoversExactlyTheTokenRepositories(t *testing.T) {
	_, bot := mustStep(t, "renovate", mustJob(t, loadWorkflow(t, renovatePath), "renovate").Steps, "bot")
	var want []string
	for _, r := range strings.Split(bot.With["repositories"], ",") {
		want = append(want, "jellalshadows-idp/"+strings.TrimSpace(r))
	}
	got := loadRenovateConfig(t).Repositories
	if !slices.Equal(got, want) {
		t.Errorf("repositories = %v, want the token's %v", got, want)
	}
	if slices.Contains(got, "jellalshadows-idp/idp-claims-e2e") {
		t.Error("idp-claims-e2e is left out: the harness pins its callers on every run (owner decision)")
	}
}

func TestRenovateGlobalConfig(t *testing.T) {
	c := loadRenovateConfig(t)
	if c.Platform != "github" || c.RequireConfig != "ignored" || c.PlatformCommit != "enabled" {
		t.Errorf("platform %q requireConfig %q platformCommit %q, want github, ignored, enabled", c.Platform, c.RequireConfig, c.PlatformCommit)
	}
	if c.Onboarding == nil || *c.Onboarding || c.DependencyDashboard == nil || !*c.DependencyDashboard {
		t.Error("onboarding must be false and dependencyDashboard true")
	}
	if c.GitAuthor != botGitAuthor {
		t.Errorf("gitAuthor = %q, want %q", c.GitAuthor, botGitAuthor)
	}
	if !slices.Equal(c.EnabledManagers, []string{"github-actions", "gomod", "terraform", "custom.regex"}) {
		t.Errorf("enabledManagers = %v", c.EnabledManagers)
	}
	if got := slices.Sorted(maps.Keys(c.StatusCheckNames)); !slices.Equal(got, []string{"artifactError", "configValidation", "mergeConfidence", "minimumReleaseAge"}) {
		t.Errorf("statusCheckNames keys = %v, want all four", got)
	}
	for k, v := range c.StatusCheckNames {
		if v != nil {
			t.Errorf("statusCheckNames.%s = %q, want null: the bot cannot write statuses, and a failed status write stops Renovate's run of the repository", k, *v)
		}
	}
}

func TestRenovateNeverAutomerges(t *testing.T) {
	c := loadRenovateConfig(t)
	if c.Automerge == nil || *c.Automerge {
		t.Error("automerge must be false: the owner merges every Renovate PR (owner decision)")
	}
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(renovateConfigPath)))
	if err != nil {
		t.Fatal(err)
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, x := range v {
				if (k == "automerge" || k == "platformAutomerge") && x == true {
					t.Errorf("%s.%s is true", path, k)
				}
				walk(path+"."+k, x)
			}
		case []any:
			for _, x := range v {
				walk(path+"[]", x)
			}
		}
	}
	walk("$", raw)
}

func TestRenovateWaitsOnlyForThirdPartyReleases(t *testing.T) {
	c := loadRenovateConfig(t)
	if c.MinimumReleaseAge != "14 days" || c.InternalChecksFilter != "strict" {
		t.Errorf("minimumReleaseAge %q internalChecksFilter %q, want 14 days and strict (spec §7.5)", c.MinimumReleaseAge, c.InternalChecksFilter)
	}
	for _, r := range c.PackageRules {
		if r.MinimumReleaseAge != nil && !slices.Equal(r.MatchPackageNames, []string{"jellalshadows-idp/idp-engine"}) {
			t.Errorf("rule %q changes minimumReleaseAge: only the engine's own releases skip the 14 days (owner decision)", r.Description)
		}
	}
}

func TestRenovateMovesTheEnginePinsTogether(t *testing.T) {
	var engine []renovateRule
	for _, r := range loadRenovateConfig(t).PackageRules {
		if slices.Contains(r.MatchPackageNames, "jellalshadows-idp/idp-engine") {
			engine = append(engine, r)
		}
	}
	if len(engine) != 1 {
		t.Fatalf("%d rules name the engine, want 1", len(engine))
	}
	r := engine[0]
	if !slices.Equal(r.MatchPackageNames, []string{"jellalshadows-idp/idp-engine"}) || r.GroupName != "idp engine" {
		t.Errorf("engine rule = %+v, want only the engine, grouped as \"idp engine\": the four callers must move in one PR, or a claims repo would pin two engine commits", r)
	}
	if string(r.MinimumReleaseAge) != "null" {
		t.Errorf("engine minimumReleaseAge = %s, want null: engine releases do not wait (owner decision)", r.MinimumReleaseAge)
	}
	n := 0
	for _, name := range templateNames {
		for _, u := range usesRefs(readYAML(t, templatesDir+name, &struct{}{})) {
			if strings.HasPrefix(u.value, "jellalshadows-idp/idp-engine/") {
				n++
			}
		}
	}
	if n != 4 {
		t.Errorf("the templates pin the engine %d times, want 4: one group moves them all", n)
	}
}

func TestRenovateRules(t *testing.T) {
	f := false
	want := []renovateRule{
		{MatchPackageNames: []string{"jellalshadows-idp/idp-engine"}, GroupName: "idp engine", MinimumReleaseAge: json.RawMessage("null")},
		{MatchPackageNames: []string{"opentofu/opentofu"}, GroupName: "opentofu"},
		{MatchPackageNames: []string{"integrations/github"}, GroupName: "github provider", Labels: []string{"needs-lockfile"}},
		{MatchDatasources: []string{"terraform-provider"}, RegistryURLs: []string{"https://registry.opentofu.org"}},
		{MatchManagers: []string{"terraform"}, MatchDepTypes: []string{"required_version"}, Enabled: &f},
		{MatchManagers: []string{"gomod"}, PostUpdateOptions: []string{"gomodTidy"}},
		{MatchManagers: []string{"gomod"}, MatchDepTypes: []string{"golang"}, MatchDepNames: []string{"go"}, RangeStrategy: "bump"},
		{MatchManagers: []string{"gomod"}, MatchDepTypes: []string{"golang"}, MatchDepNames: []string{"go"}, MatchUpdateTypes: []string{"minor", "major"}, Enabled: &f},
		{MatchDatasources: []string{"github-runners"}, Enabled: &f},
	}
	got := slices.Clone(loadRenovateConfig(t).PackageRules)
	for i := range got {
		if got[i].Description == "" {
			t.Errorf("packageRules[%d] has no description: say why the rule exists", i)
		}
		got[i].Description = ""
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("packageRules =\n%+v\nwant\n%+v", got, want)
	}
}

func TestRenovateCustomManagers(t *testing.T) {
	type id struct{ dep, datasource, pkg, registry string }
	var got []id
	for _, m := range loadRenovateConfig(t).CustomManagers {
		if m.CustomType != "regex" || m.Description == "" {
			t.Errorf("custom manager %q: customType %q, description %q; want regex and a description", m.DepNameTemplate, m.CustomType, m.Description)
		}
		got = append(got, id{m.DepNameTemplate, m.DatasourceTemplate, m.PackageNameTemplate, m.RegistryURLTemplate})
	}
	want := []id{
		{"github.com/rhysd/actionlint", "go", "", ""},
		{"zizmor", "pypi", "", ""},
		{"renovatebot/renovate", "github-releases", "", ""},
		{"opentofu", "github-releases", "opentofu/opentofu", ""},
		{"github", "terraform-provider", "integrations/github", "https://registry.opentofu.org"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("custom managers = %v, want %v", got, want)
	}
}

// TestRenovateCustomManagersFindThePins runs each regex manager over the files
// it matches, as Renovate would, and checks it finds each pin it exists for.
func TestRenovateCustomManagersFindThePins(t *testing.T) {
	files := repoFiles(t)
	found := map[string][]string{} // depName -> "file=value", sorted
	for _, m := range loadRenovateConfig(t).CustomManagers {
		var pats []*regexp.Regexp
		for _, p := range m.ManagerFilePatterns {
			if len(p) < 2 || p[0] != '/' || p[len(p)-1] != '/' {
				t.Errorf("%s: managerFilePatterns entry %q is not a /regex/", m.DepNameTemplate, p)
				continue
			}
			pats = append(pats, regexp.MustCompile(p[1:len(p)-1]))
		}
		for _, f := range files {
			if !slices.ContainsFunc(pats, func(re *regexp.Regexp) bool { return re.MatchString(f) }) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(f)))
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range m.MatchStrings {
				re := regexp.MustCompile(s)
				i := re.SubexpIndex("currentValue")
				if i < 0 {
					t.Errorf("%q has no currentValue group", s)
					continue
				}
				for _, sub := range re.FindAllStringSubmatch(string(b), -1) {
					found[m.DepNameTemplate] = append(found[m.DepNameTemplate], f+"="+sub[i])
				}
			}
		}
	}
	for _, v := range found {
		slices.Sort(v)
	}
	ci, rn := ".github/workflows/ci.yaml", renovatePath
	golden := "internal/render/testdata/basic/expected/github/main.tf.json"
	values := func(dep string) []string {
		var out []string
		for _, e := range found[dep] {
			out = append(out, e[strings.Index(e, "=")+1:])
		}
		return slices.Compact(slices.Sorted(slices.Values(out)))
	}
	actionlint := values("github.com/rhysd/actionlint")
	zizmor := values("zizmor")
	if len(actionlint) != 1 || len(zizmor) != 1 {
		t.Fatalf("actionlint versions %v, zizmor versions %v: want one each", actionlint, zizmor)
	}
	want := map[string][]string{
		"github.com/rhysd/actionlint": {ci + "=" + actionlint[0], ci + "=" + actionlint[0]},
		"zizmor":                      {ci + "=" + zizmor[0]},
		"renovatebot/renovate":        {ci + "=" + renovateVersion(t), rn + "=" + renovateVersion(t)},
		"opentofu":                    {"internal/render/render.go=" + render.TofuVersion, golden + "=" + render.TofuVersion},
		"github":                      {"internal/render/render.go=" + render.GitHubProviderVersion, golden + "=" + render.GitHubProviderVersion},
	}
	if !reflect.DeepEqual(found, want) {
		t.Errorf("regex managers found\n%v\nwant\n%v", found, want)
	}
}

func TestOneOpenTofuAndProviderVersion(t *testing.T) {
	if got := ciTofuVersion(t); got != render.TofuVersion {
		t.Errorf("CI tests with OpenTofu %s, the renderer requires %s: the opentofu group moves them together, so they must start equal", got, render.TofuVersion)
	}
	for _, m := range []string{"group", "component"} {
		b, err := os.ReadFile(filepath.Join(repoRoot, "modules", "github", m, "versions.tf"))
		if err != nil {
			t.Fatal(err)
		}
		sub := regexp.MustCompile(`version\s*=\s*"([^"]+)"`).FindSubmatch(b)
		if sub == nil || string(sub[1]) != render.GitHubProviderVersion {
			t.Errorf("modules/github/%s pins the provider at %q, the renderer at %s", m, sub, render.GitHubProviderVersion)
		}
	}
}

func TestCIValidatesTheRenovateConfig(t *testing.T) {
	_, s := mustStep(t, "ci workflows", mustJob(t, loadWorkflow(t, ".github/workflows/ci.yaml"), "workflows").Steps, "renovate-config")
	wantRun(t, "ci renovate-config", s,
		`-v "$GITHUB_WORKSPACE/`+renovateConfigPath+`:/tmp/renovate-global.json:ro"`,
		"ghcr.io/renovatebot/renovate:"+renovateVersion(t)+" renovate-config-validator --strict /tmp/renovate-global.json")
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/workflowspec/ -run 'TestRenovate|TestOneOpenTofu|TestCIValidates'`
Expected: FAIL to compile (`undefined: repoFiles`); with the helper, FAIL on the missing files (`read .github/workflows/renovate.yaml: …`, `read .github/renovate-global.json: …`).

- [ ] **Step 3: Implement**

`internal/workflowspec/load_test.go`: add `"io/fs"` to the imports; old:

```go
	internalWorkflows = []string{"ci.yaml", "e2e.yaml", "release.yaml"}
```

new:

```go
	internalWorkflows = []string{"ci.yaml", "e2e.yaml", "release.yaml", "renovate.yaml"}
```

and after `ifBody`:

```go
// repoFiles lists every file of the repository as a slash path relative to its
// root, without .git, the local scratch folders and build output.
func repoFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".superpowers", ".terraform", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repoRoot, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}
```

`.github/workflows/renovate.yaml`:

```yaml
name: idp-renovate

# Self-hosted Renovate with the bot App (ADR-0029): weekly and on demand from
# main, over idp-engine and idp-claims. Its whole configuration is
# .github/renovate-global.json; it never merges anything.

on:
  schedule:
    - cron: '17 4 * * 1'
  workflow_dispatch:

permissions: {}

defaults:
  run:
    shell: bash

concurrency:
  group: idp-renovate
  cancel-in-progress: false

jobs:
  renovate:
    name: renovate
    runs-on: ubuntu-24.04
    # Stays within the hour the bot token lives.
    timeout-minutes: 50
    environment: idp-renovate
    permissions:
      contents: read
    steps:
      - name: check the environment
        id: config
        env:
          BOT_CLIENT_ID: ${{ vars.IDP_BOT_CLIENT_ID }}
          BOT_KEY_SET: ${{ secrets.IDP_BOT_PRIVATE_KEY != '' }}
        run: |
          missing=""
          [ -n "$BOT_CLIENT_ID" ] || missing="$missing variable IDP_BOT_CLIENT_ID,"
          [ "$BOT_KEY_SET" = true ] || missing="$missing secret IDP_BOT_PRIVATE_KEY,"
          if [ -n "$missing" ]; then
            echo "::error::the idp-renovate environment is missing:${missing%,} (docs/runbooks/dependencies.md)"
            exit 1
          fi
      - id: checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - name: mint the bot token
        id: bot
        uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0
        with:
          client-id: ${{ vars.IDP_BOT_CLIENT_ID }}
          private-key: ${{ secrets.IDP_BOT_PRIVATE_KEY }}
          owner: jellalshadows-idp
          repositories: idp-engine,idp-claims
          permission-checks: read
          permission-contents: write
          permission-issues: write
          permission-pull-requests: write
          permission-statuses: read
          permission-workflows: write
      - name: renovate
        id: renovate
        uses: renovatebot/github-action@6d26fcf0275dc65624cbc3d51fe78bc773f98321 # v46.3.6
        with:
          configurationFile: .github/renovate-global.json
          renovate-version: 44.133.0
          token: ${{ steps.bot.outputs.token }}
```

`.github/renovate-global.json`:

```json
{
  "platform": "github",
  "repositories": [
    "jellalshadows-idp/idp-engine",
    "jellalshadows-idp/idp-claims"
  ],
  "onboarding": false,
  "requireConfig": "ignored",
  "platformCommit": "enabled",
  "gitAuthor": "jellalshadows-idp-bot[bot] <343234638+jellalshadows-idp-bot[bot]@users.noreply.github.com>",
  "dependencyDashboard": true,
  "automerge": false,
  "minimumReleaseAge": "14 days",
  "internalChecksFilter": "strict",
  "statusCheckNames": {
    "artifactError": null,
    "configValidation": null,
    "mergeConfidence": null,
    "minimumReleaseAge": null
  },
  "enabledManagers": ["github-actions", "gomod", "terraform", "custom.regex"],
  "packageRules": [
    {
      "description": "Engine releases: the four caller pins of a repository move in one PR, and do not wait 14 days, because the engine is this project's own release (ADR-0029).",
      "matchPackageNames": ["jellalshadows-idp/idp-engine"],
      "groupName": "idp engine",
      "minimumReleaseAge": null
    },
    {
      "description": "One OpenTofu version: setup-idp, ci.yaml, the renderer's required_version and its golden render move together.",
      "matchPackageNames": ["opentofu/opentofu"],
      "groupName": "opentofu"
    },
    {
      "description": "The GitHub provider: the module pins, the renderer's pin and its golden render move together; the lock file is regenerated by hand (docs/runbooks/dependencies.md).",
      "matchPackageNames": ["integrations/github"],
      "groupName": "github provider",
      "labels": ["needs-lockfile"]
    },
    {
      "description": "Providers come from the OpenTofu registry, like the shipped lock file.",
      "matchDatasources": ["terraform-provider"],
      "registryUrls": ["https://registry.opentofu.org"]
    },
    {
      "description": "required_version in the modules is a lower bound for OpenTofu, not a pin.",
      "matchManagers": ["terraform"],
      "matchDepTypes": ["required_version"],
      "enabled": false
    },
    {
      "description": "Run go mod tidy after every Go module update.",
      "matchManagers": ["gomod"],
      "postUpdateOptions": ["gomodTidy"]
    },
    {
      "description": "The go directive is the exact Go version CI and setup-idp install (setup-go reads go.mod): update its patch.",
      "matchManagers": ["gomod"],
      "matchDepTypes": ["golang"],
      "matchDepNames": ["go"],
      "rangeStrategy": "bump"
    },
    {
      "description": "A new Go minor or major is a deliberate upgrade, not a dependency update.",
      "matchManagers": ["gomod"],
      "matchDepTypes": ["golang"],
      "matchDepNames": ["go"],
      "matchUpdateTypes": ["minor", "major"],
      "enabled": false
    },
    {
      "description": "Runner images (runs-on) change only on purpose.",
      "matchDatasources": ["github-runners"],
      "enabled": false
    }
  ],
  "customManagers": [
    {
      "customType": "regex",
      "description": "actionlint, run with go run in ci.yaml.",
      "managerFilePatterns": ["/^\\.github/workflows/ci\\.yaml$/"],
      "matchStrings": ["github\\.com/rhysd/actionlint/cmd/actionlint@(?<currentValue>v\\d+\\.\\d+\\.\\d+)"],
      "datasourceTemplate": "go",
      "depNameTemplate": "github.com/rhysd/actionlint"
    },
    {
      "customType": "regex",
      "description": "zizmor, run with pipx in ci.yaml.",
      "managerFilePatterns": ["/^\\.github/workflows/ci\\.yaml$/"],
      "matchStrings": ["zizmor==(?<currentValue>\\d+\\.\\d+\\.\\d+)"],
      "datasourceTemplate": "pypi",
      "depNameTemplate": "zizmor"
    },
    {
      "customType": "regex",
      "description": "The Renovate version: renovate.yaml runs it, and ci.yaml validates this file with the same image.",
      "managerFilePatterns": ["/^\\.github/workflows/(ci|renovate)\\.yaml$/"],
      "matchStrings": [
        "renovate-version: (?<currentValue>\\d+\\.\\d+\\.\\d+)",
        "ghcr\\.io/renovatebot/renovate:(?<currentValue>\\d+\\.\\d+\\.\\d+)"
      ],
      "datasourceTemplate": "github-releases",
      "depNameTemplate": "renovatebot/renovate",
      "versioningTemplate": "semver"
    },
    {
      "customType": "regex",
      "description": "OpenTofu in the renderer's required_version and its golden render; setup-opentofu's tofu_version is found by the github-actions manager.",
      "managerFilePatterns": [
        "/^internal/render/render\\.go$/",
        "/^internal/render/testdata/[^/]+/expected/github/main\\.tf\\.json$/"
      ],
      "matchStrings": [
        "TofuVersion\\s+=\\s+\"(?<currentValue>\\d+\\.\\d+\\.\\d+)\"",
        "\"required_version\": \"(?<currentValue>\\d+\\.\\d+\\.\\d+)\""
      ],
      "datasourceTemplate": "github-releases",
      "depNameTemplate": "opentofu",
      "packageNameTemplate": "opentofu/opentofu",
      "versioningTemplate": "npm",
      "extractVersionTemplate": "^v(?<version>.+)$"
    },
    {
      "customType": "regex",
      "description": "The GitHub provider in the renderer and its golden render; the module pins are found by the terraform manager.",
      "managerFilePatterns": [
        "/^internal/render/render\\.go$/",
        "/^internal/render/testdata/[^/]+/expected/github/main\\.tf\\.json$/"
      ],
      "matchStrings": [
        "GitHubProviderVersion\\s+=\\s+\"(?<currentValue>\\d+\\.\\d+\\.\\d+)\"",
        "\"source\": \"integrations/github\",\\s+\"version\": \"(?<currentValue>\\d+\\.\\d+\\.\\d+)\""
      ],
      "datasourceTemplate": "terraform-provider",
      "depNameTemplate": "github",
      "packageNameTemplate": "integrations/github",
      "registryUrlTemplate": "https://registry.opentofu.org"
    }
  ]
}
```

`.github/workflows/ci.yaml`, in job `workflows`, after the `zizmor` step:

```yaml
      - name: renovate config
        id: renovate-config
        # The Renovate image at renovate.yaml's version: Renovate needs a newer
        # Node than the runner's, and the run and the check stay one version.
        run: docker run --rm -v "$GITHUB_WORKSPACE/.github/renovate-global.json:/tmp/renovate-global.json:ro" ghcr.io/renovatebot/renovate:44.133.0 renovate-config-validator --strict /tmp/renovate-global.json
```

`docs/runbooks/dependencies.md`:

````markdown
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
| OpenTofu | `setup-idp`, `ci.yaml`, `TofuVersion` in `internal/render/render.go` and the golden render | group "opentofu" |
| The GitHub provider | `modules/github/*/versions.tf`, `GitHubProviderVersion` in `render.go` and the golden render | group "github provider", label `needs-lockfile` |
| Go modules | `go.mod` and `go.sum` (`go mod tidy`) | one per module |
| Go | the `go` directive in `go.mod`, which CI and `setup-idp` install | patch updates only |
| GoReleaser | the GoReleaser action's `version` in `ci.yaml` and `release.yaml` | one |
| actionlint, zizmor | `ci.yaml` | one each |
| Renovate | `renovate.yaml` and the validator image in `ci.yaml` | one |

Runner images (`runs-on`) are not updated. Every update waits 14 days after its release; engine releases do not wait. Pending updates are listed in the "Dependency Dashboard" issue of each repository.

## Reviewing and merging

Renovate never merges. Wait for the PR's checks, read it, and merge it with a merge commit: `gh pr merge <number> --repo <owner>/<repo> --merge --delete-branch`. In `idp-claims` a PR merges only after `idp-gate`; for an engine upgrade the plan comment shows every module source moving to the new `?ref=` ([Upgrading the engine](../pipelines.md#upgrading-the-engine)).

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

Renovate moves the provider pins but not the shipped lock file, so the PR's `go` and `modules` jobs fail (`TestGitHubLockPinsTheProvider`, `tofu init -lockfile=readonly`). Regenerate the lock on the PR branch, in Git Bash, with OpenTofu at the pinned version (here 1.12.6):

```bash
gh pr checkout <number> --repo jellalshadows-idp/idp-engine
cd modules/github/group
rm -f .terraform.lock.hcl
"$HOME/.idp/tools/tofu-1.12.6/tofu.exe" providers lock -platform=linux_amd64 -platform=windows_amd64 -platform=darwin_amd64 -platform=darwin_arm64
mv .terraform.lock.hcl ../../../lockfiles/github.terraform.lock.hcl
cd ../../..
```

The four platforms are the runners (linux amd64) and the owner's machines. Then set the new version in `lockfiles/lockfiles_test.go` (`TestGitHubLockPinsTheProvider`), run `go test ./lockfiles/ ./internal/render/`, commit as `chore(deps): regenerate the github provider lock file` and push. The checks run again.

## An OpenTofu update

One PR moves every pin. Update the local tool under `$HOME/.idp/tools/` to the same version before running module tests locally.

## When a run fails

- `the idp-renovate environment is missing: …`: set it up as above.
- `mint a token …` or a 401: a wrong `IDP_BOT_CLIENT_ID`, a rotated key, or the bot App uninstalled from one of the two repositories ([the bot App](bootstrap.md#the-bot-app)).
- A repository is skipped with an error in the log: read the Dependency Dashboard of that repository; Renovate lists there what it could not do.
- GitHub disables a scheduled workflow after 60 days without activity in a public repository; Renovate PRs keep `idp-claims` active. Re-enable a disabled schedule with `gh workflow enable <workflow> --repo <owner>/<repo>`.
````

`docs/runbooks/bootstrap.md` (exact replacements with Edit):

1. Old:

```markdown
`apply` never writes the bot's key: it lives only in the `idp-engine` environment `idp-e2e` ([e2e harness](../e2e.md#credentials)), so "Rotating a secret" below does not apply to it. To rotate it:

1. Generate a new private key in the App's settings (`https://github.com/organizations/<org>/settings/apps/<org>-bot`).
2. `gh secret set IDP_BOT_PRIVATE_KEY --env idp-e2e --repo <org>/idp-engine < <new pem>`
3. Save the new key as `~/.idp/apps/<org>-bot.pem`, replacing the old one: the [e2e setup](../e2e.md#setting-up) command reads it.
4. Run the e2e workflow once (`gh workflow run e2e.yaml --repo <org>/idp-engine --ref main`) and check it passes.
5. Delete the old key in the App's settings.
```

new:

```markdown
`apply` never writes the bot's key: it lives only in the `idp-engine` environments `idp-e2e` ([e2e harness](../e2e.md#credentials)), `idp-release` ([releases](releases.md)) and `idp-renovate` ([dependencies](dependencies.md)), so "Rotating a secret" below does not apply to it. To rotate it:

1. Generate a new private key in the App's settings (`https://github.com/organizations/<org>/settings/apps/<org>-bot`).
2. Set it in all three environments: `for env in idp-e2e idp-release idp-renovate; do gh secret set IDP_BOT_PRIVATE_KEY --env "$env" --repo <org>/idp-engine < <new pem>; done`
3. Save the new key as `~/.idp/apps/<org>-bot.pem`, replacing the old one: the setup commands of the three environments read it.
4. Run each workflow once and check it passes: `gh workflow run e2e.yaml --repo <org>/idp-engine --ref main`, the same for `release.yaml` and `renovate.yaml`.
5. Delete the old key in the App's settings.
```

2. Old:

```markdown
Keep the bot's `~/.idp/apps/<org>-bot.pem` until the `idp-e2e` environment is set up: its [setup](../e2e.md#setting-up) reads it.
```

new:

```markdown
Keep the bot's `~/.idp/apps/<org>-bot.pem` until the `idp-e2e`, `idp-release` and `idp-renovate` environments are set up: their setup commands read it ([e2e](../e2e.md#setting-up), [releases](releases.md#setting-up-the-environment), [dependencies](dependencies.md#setting-up-the-environment)).
```

- [ ] **Step 4: Verify** — `go test ./... && go vet ./... && gofmt -l .`; actionlint (no output); zizmor as in Task 3 (no unsuppressed findings). The validator itself runs in CI (Task 9); there is no local Docker.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/renovate.yaml .github/renovate-global.json .github/workflows/ci.yaml internal/workflowspec docs/runbooks/dependencies.md docs/runbooks/bootstrap.md
git commit -m "ci: run renovate weekly with the bot app"
```

---

### Task 9: Ship I — verification, PR, review, environments, ruleset, merge

Controller task (tests and linters are delegated to a subagent).

- [ ] **Step 1: Local verification** — `go test ./... && go vet ./... && gofmt -l .` (gofmt prints nothing); both actionlint invocations (`go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`, and the templates with `-ignore 'unexpected key "queue" for "concurrency" section' examples/claims-repo/.github/workflows/*.yaml`); `"$ZIZMOR" --offline .github/workflows .github/actions/setup-idp/action.yaml examples/claims-repo/.github/workflows/*.yaml`; the naming check (prints nothing). The modules are unchanged, so no `tofu test`.
- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feat/phase-1e-releases
gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-1e-releases --title "feat: phase 1e releases, renovate and verified release binaries" --body "Implements docs/superpowers/plans/2026-10-18-phase-1e-releases.md (ADR-0028, ADR-0029, spec amendment A9): idp version; GoReleaser with the {{.Tag}} guard, proven in CI by a snapshot; release.yaml (release-please with the bot App, publishing with GoReleaser and build provenance, the harness dispatched for release PRs with the idp-e2e status); setup-idp installs the verified release binary of a release commit and builds any other commit; the harness pins the commit under test as unreleased; self-hosted Renovate, weekly, with no automerge."
```

- [ ] **Step 3: CI** — `gh pr checks <number> --repo jellalshadows-idp/idp-engine --watch`. Expected: `go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `render-smoke` and `release-snapshot` pass. Record two watch items from the logs: the `release-snapshot` step `the snapshot binary reports a v-tag` prints `snapshot idp version: v0.0.0`, and the `workflows` step `renovate config` ends with `Config validated successfully against 1 file(s)`. A failure goes through the fix loop (fix with a test, scoped review, push).
- [ ] **Step 4: Final whole-branch review** (most capable model) with the package `main..HEAD`, the spec, ADRs 0021, 0026, 0027, 0028 and 0029, and this plan; one fix wave; one scoped re-review; rulings in the ledger. CI green again after the wave.
- [ ] **Step 5: The environments** (side effect 2). Run in Git Bash; the key is redirected from the file, never printed:

```bash
client_id="$(python -I -c "import json,os; print(json.load(open(os.path.expanduser('~/.idp/apps/jellalshadows-idp-bot.json')))['client_id'])")"
for env in idp-release idp-renovate; do
  gh api -X PUT "repos/jellalshadows-idp/idp-engine/environments/$env" \
    -F 'deployment_branch_policy[protected_branches]=false' -F 'deployment_branch_policy[custom_branch_policies]=true' --silent
  gh api -X POST "repos/jellalshadows-idp/idp-engine/environments/$env/deployment-branch-policies" -f name=main -f type=branch --silent
  gh secret set IDP_BOT_PRIVATE_KEY --env "$env" --repo jellalshadows-idp/idp-engine < ~/.idp/apps/jellalshadows-idp-bot.pem
  gh variable set IDP_BOT_CLIENT_ID --env "$env" --repo jellalshadows-idp/idp-engine --body "$client_id"
done
```

Verify:

```bash
gh api repos/jellalshadows-idp/idp-engine/environments --jq '[.environments[].name] | sort | join(" ")'
for env in idp-release idp-renovate; do
  gh api "repos/jellalshadows-idp/idp-engine/environments/$env/deployment-branch-policies" --jq '[.branch_policies[].name] | join(" ")'
  gh secret list --env "$env" --repo jellalshadows-idp/idp-engine --json name --jq '[.[].name] | join(" ")'
  gh variable list --env "$env" --repo jellalshadows-idp/idp-engine --json name,value --jq '.[] | [.name, .value] | @tsv'
done
```

Expected: `idp-e2e idp-release idp-renovate`; then for each environment `main`, `IDP_BOT_PRIVATE_KEY`, and `IDP_BOT_CLIENT_ID	Iv23linzv8qQJ5r6kWO7`.

- [ ] **Step 6: The tag ruleset** (side effect 3). Write `<scratchpad>/idp-engine-tags.json` with the Write tool:

```json
{
  "name": "idp-engine-tags",
  "target": "tag",
  "enforcement": "active",
  "conditions": {"ref_name": {"include": ["refs/tags/v*"], "exclude": []}},
  "rules": [{"type": "deletion"}, {"type": "non_fast_forward"}, {"type": "update"}],
  "bypass_actors": []
}
```

Run: `gh api -X POST repos/jellalshadows-idp/idp-engine/rulesets --input "<scratchpad>/idp-engine-tags.json" --jq '[.id, .name, .target, .enforcement] | @tsv'`
Expected: `<id>	idp-engine-tags	tag	active`. Then `gh api repos/jellalshadows-idp/idp-engine/rulesets/<id> --jq '[.conditions.ref_name.include[0], ([.rules[].type] | join(",")), (.bypass_actors | length)] | @tsv'` prints `refs/tags/v*	deletion,non_fast_forward,update	0`.

- [ ] **Step 7: Merge** — confirm the PR's latest CI is green, then `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`. The push of the merge starts `release.yaml`: continue with Task 10. If the weekly Renovate schedule fires before Task 12, record its run; the PRs it opens wait for the owner.

---

### Task 10: Release `v0.1.0`

- [ ] **Step 1: The release run of the merge**

```bash
run="$(gh run list --repo jellalshadows-idp/idp-engine --workflow release.yaml --event push --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$run" --repo jellalshadows-idp/idp-engine --exit-status
gh run view "$run" --repo jellalshadows-idp/idp-engine --json headSha,jobs --jq '.headSha, (.jobs[] | [.name, .conclusion] | @tsv)'
```

Expected: the head SHA is the Task 9 merge commit; `release-please	success`, `publish	skipped`, `e2e on the release PR	success` (proves `prs_created` and `pr`).

- [ ] **Step 2: The release PR proposes `0.1.0`**

```bash
gh pr list --repo jellalshadows-idp/idp-engine --label "autorelease: pending" --json number,title,headRefName,headRefOid --jq '.[] | [.number, .title, .headRefName, .headRefOid] | @tsv'
gh api "repos/jellalshadows-idp/idp-engine/contents/.release-please-manifest.json?ref=release-please--branches--main" --jq .content | python -I -c "import sys,base64,json; print(json.loads(base64.b64decode(sys.stdin.read())))"
gh api "repos/jellalshadows-idp/idp-engine/contents/CHANGELOG.md?ref=release-please--branches--main" --jq .content | python -I -c "import sys,base64; print([l for l in base64.b64decode(sys.stdin.read()).decode().splitlines() if l.startswith('## ')][0])"
```

Expected: one PR `<number>	chore(main): release 0.1.0	release-please--branches--main	<head>`; `{'.': '0.1.0'}`; a heading that starts with `## 0.1.0`. If the version is anything else, push the fallback (side effect 4) and wait for the next `release.yaml` run to rebuild the PR, then check again:

```bash
git commit --allow-empty -m "chore: release 0.1.0" -m "Release-As: 0.1.0"
git push
```

- [ ] **Step 3: CI on the release PR** — `gh pr checks <number> --repo jellalshadows-idp/idp-engine --watch`: the six checks of Task 9 Step 3 pass.
- [ ] **Step 4: The harness on the release PR**

```bash
e2e="$(gh run list --repo jellalshadows-idp/idp-engine --workflow e2e.yaml --event workflow_dispatch --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$e2e" --repo jellalshadows-idp/idp-engine --exit-status
head="$(gh pr view <number> --repo jellalshadows-idp/idp-engine --json headRefOid --jq .headRefOid)"
gh api "repos/jellalshadows-idp/idp-engine/commits/$head/status" --jq '.statuses[] | select(.context == "idp-e2e") | [.state, .target_url] | @tsv'
```

Expected: the run passes (`e2e: passed` in the `run the harness` log; its pin step moved the `idp-claims-e2e` callers to the merge commit, built from source: `setup-idp: … is not a release; building idp from source` in the `idp-claims-e2e` run logs); the status is `success` with the run's URL. If no status appears (a replaced dispatch) or the head moved, dispatch again for the current head (side effect 5) with the commands in `docs/runbooks/releases.md`, "The release PR has no green idp-e2e status".

- [ ] **Step 5: Merge the release PR** (side effect 4) — only when the PR's current head has `idp-e2e` `success` and green CI: `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge --delete-branch && git pull --ff-only`.
- [ ] **Step 6: The release run of the release merge** — the commands of Step 1 for the newest push run. Expected: the head SHA is the release merge commit (call it `M`); `release-please	success`, `publish	success`, `e2e on the release PR	skipped`. The `check the release` step passed both guards, and `the released binary reports its tag` printed `idp version: v0.1.0`.
- [ ] **Step 7: The release**

```bash
gh release view v0.1.0 --repo jellalshadows-idp/idp-engine --json tagName,isDraft,isPrerelease,body,assets --jq '[.tagName, .isDraft, .isPrerelease, (.body | contains("Features"))] | @tsv, ([.assets[].name] | sort | join(" "))'
gh api repos/jellalshadows-idp/idp-engine/git/ref/tags/v0.1.0 --jq '"\(.object.type) \(.object.sha)"'
```

Expected: `v0.1.0	false	false	true` (release-please's notes kept: `keep-existing` proven); assets `checksums.txt idp_0.1.0_darwin_amd64.tar.gz idp_0.1.0_darwin_arm64.tar.gz idp_0.1.0_linux_amd64.tar.gz idp_0.1.0_linux_arm64.tar.gz idp_0.1.0_windows_amd64.zip idp_0.1.0_windows_arm64.zip`; `commit <M>` (a lightweight tag on the release merge).

- [ ] **Step 8: Verify the release locally** (D-30), in one Git Bash command:

```bash
d="<scratchpad>/v0.1.0" && mkdir -p "$d" && \
gh release download v0.1.0 --repo jellalshadows-idp/idp-engine --dir "$d" --pattern idp_0.1.0_windows_amd64.zip --pattern idp_0.1.0_linux_amd64.tar.gz --pattern checksums.txt && \
(cd "$d" && sha256sum --check --ignore-missing checksums.txt) && \
M="$(gh api repos/jellalshadows-idp/idp-engine/git/ref/tags/v0.1.0 --jq .object.sha)" && \
for a in idp_0.1.0_windows_amd64.zip idp_0.1.0_linux_amd64.tar.gz; do
  gh attestation verify "$d/$a" --repo jellalshadows-idp/idp-engine --signer-workflow jellalshadows-idp/idp-engine/.github/workflows/release.yaml --source-digest "$M" --source-ref refs/heads/main || exit 1
done && \
python -I -m zipfile -e "$d/idp_0.1.0_windows_amd64.zip" "$d/bin" && "$d/bin/idp.exe" version
```

Expected: `idp_0.1.0_linux_amd64.tar.gz: OK` and `idp_0.1.0_windows_amd64.zip: OK`; two `Verification succeeded!` reports; `v0.1.0`. Keep `<scratchpad>/v0.1.0/bin/idp.exe` for Task 12.

- [ ] **Step 9: The ruleset holds** — the read-only check of Task 9 Step 6 prints `refs/tags/v*	deletion,non_fast_forward,update	0`. Never try to move or delete the tag to test it.
- [ ] **Step 10: The download path, live** (side effect 5). Only after Step 6's `publish` succeeded, and before anything else is pushed to `main` (so `main` is still `M`): `gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main`; find and watch the run as in Step 4. Expected: it passes, and its pin PR moved the `idp-claims-e2e` callers to `M`. Evidence in `idp-claims-e2e`:

```bash
pin="$(gh run list --repo jellalshadows-idp/idp-claims-e2e --workflow pr.yaml --limit 20 --json databaseId,headBranch --jq '[.[] | select(.headBranch | endswith("/pin"))][0].databaseId')"
gh run view "$pin" --repo jellalshadows-idp/idp-claims-e2e --log | rg "installed idp v0.1.0|is not a release"
create="$(gh run list --repo jellalshadows-idp/idp-claims-e2e --workflow pr.yaml --limit 20 --json databaseId,headBranch --jq '[.[] | select(.headBranch | endswith("/create"))][0].databaseId')"
gh run view "$create" --repo jellalshadows-idp/idp-claims-e2e --log | rg -m 3 "ref=v0.1.0"
```

Expected: only `setup-idp: installed idp v0.1.0, verified against <M>` lines (never `is not a release`): the runner's `gh` attestation flags and the caller token's read of the engine's releases and attestations both work. The pin PR's plan comment says `No infrastructure changes.`, and the create run's `tofu init` fetched `git::https://github.com/jellalshadows-idp/idp-engine.git//modules/github/…?ref=v0.1.0` (closes Phase 1a's Task 11 minor). If `main` had already moved, record it: the download path is then proven by Task 12 instead.

- [ ] **Step 11: Fix loop** — when a step fails: read the logs, rule on the cause, fix on a branch with a test that pins it, PR, CI, scoped review, merge. A fix to the release machinery or to `setup-idp` that `v0.1.0`'s own pipelines need ships as the next patch release; ask the owner before merging that release PR (it is not authorized above). Never move or delete `v0.1.0`.

---

### Task 11: Re-pin the templates to `v0.1.0` and drop `# unreleased`

**Files:**
- Modify: `examples/claims-repo/.github/workflows/{drift,pr,reconcile,recover}.yaml`, `internal/workflowspec/rules_test.go`, `docs/pipelines.md`

**Interfaces:**
- Consumes: `checkUses`, `TestCheckUses`, `TestRemoteUsesArePinned`, `TestTemplatesCallTheirReusableWorkflowAtOneCommit`.
- Produces: `checkUses` without the `# unreleased` exception; the templates pinned to `v0.1.0`'s commit with `# v0.1.0`.

- [ ] **Step 1: Create the branch** — `git switch main && git pull --ff-only && git switch -c chore/pin-templates-v0.1.0`
- [ ] **Step 2: Write the failing test** — `internal/workflowspec/rules_test.go`, in `TestCheckUses`, old:

```go
		{"reusable workflow, unreleased", "acme/idp-engine/.github/workflows/pr.yaml@" + sha, "# unreleased", true},
```

new:

```go
		{"reusable workflow marked unreleased", "acme/idp-engine/.github/workflows/pr.yaml@" + sha, "# unreleased", false},
```

- [ ] **Step 3: Run it to see it fail**

Run: `go test ./internal/workflowspec/ -run TestCheckUses`
Expected: FAIL: `checkUses("acme/idp-engine/.github/workflows/pr.yaml@aaaa…", "# unreleased") = <nil>, want ok=false`.

- [ ] **Step 4: Implement** — `internal/workflowspec/rules_test.go`, old:

```go
var (
	pinnedRef   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}$`)
	versionNote = regexp.MustCompile(`^# v\d+\.\d+\.\d+(\s|$)`)
	unreleased  = regexp.MustCompile(`^# unreleased(\s|$)`)
)

// checkUses enforces spec §7.5: actions and reusable workflows from other
// repositories are pinned to a full commit SHA, with the version as a trailing
// comment that Renovate keeps in step. An engine workflow ref of an unreleased
// engine says "# unreleased" instead. Local actions (./...) are checked by
// TestJobsBuildIdpFromTheWorkflowCommit.
func checkUses(value, comment string) error {
	if strings.HasPrefix(value, "./") {
		return nil
	}
	if !pinnedRef.MatchString(value) {
		return fmt.Errorf("%q is not pinned to a full commit SHA", value)
	}
	if strings.Contains(value, "/idp-engine/.github/workflows/") && unreleased.MatchString(comment) {
		return nil
	}
	if !versionNote.MatchString(comment) {
```

new:

```go
var (
	pinnedRef   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}$`)
	versionNote = regexp.MustCompile(`^# v\d+\.\d+\.\d+(\s|$)`)
)

// checkUses enforces spec §7.5: actions and reusable workflows from other
// repositories are pinned to a full commit SHA, with the version as a trailing
// comment that Renovate keeps in step. Engine workflow refs pin a release too
// (ADR-0028); only the end-to-end harness pins an unreleased commit, in
// idp-claims-e2e. Local actions (./...) are checked by
// TestJobsBuildIdpFromTheWorkflowCommit.
func checkUses(value, comment string) error {
	if strings.HasPrefix(value, "./") {
		return nil
	}
	if !pinnedRef.MatchString(value) {
		return fmt.Errorf("%q is not pinned to a full commit SHA", value)
	}
	if !versionNote.MatchString(comment) {
```

Run: `go test ./internal/workflowspec/`
Expected: `TestCheckUses` passes; `TestRemoteUsesArePinned` fails on the four templates (`needs a trailing '# vX.Y.Z' comment, got "# unreleased"`).

Re-pin the templates to the commit `v0.1.0` tags (each file has exactly one engine ref; the script refuses anything else and keeps LF endings):

```bash
python -I -c "
import pathlib,re,sys
sha=sys.argv[1]
assert re.fullmatch(r'[0-9a-f]{40}', sha), sha
for p in sorted(pathlib.Path('examples/claims-repo/.github/workflows').glob('*.yaml')):
    t=p.read_bytes().decode()
    new,n=re.subn(r'(idp-engine/\.github/workflows/[a-z]+\.yaml@)[0-9a-f]{40} # unreleased', lambda m: m.group(1)+sha+' # v0.1.0', t)
    assert n==1, (p, n)
    p.write_bytes(new.encode())
    print(p.name, 'pinned to', sha[:7])
" "$(gh api repos/jellalshadows-idp/idp-engine/git/ref/tags/v0.1.0 --jq 'select(.object.type == "commit") | .object.sha')"
```

Expected: `drift.yaml pinned to <short M>`, then `pr.yaml`, `reconcile.yaml`, `recover.yaml`.

`examples/claims-repo/.github/workflows/pr.yaml`, old:

```yaml
# Claims repo caller of the engine's PR pipeline (docs/pipelines.md). Keep the
# pinned engine commit identical in all four idp workflows. The idp-gate job is
```

new:

```yaml
# Claims repo caller of the engine's PR pipeline (docs/pipelines.md). Keep the
# pinned engine release identical in all four idp workflows; Renovate moves
# them together. The idp-gate job is
```

`docs/pipelines.md`, old:

```markdown
2. Copy the four files of [`examples/claims-repo/.github/workflows/`](../examples/claims-repo/.github/workflows/) into the claims repo's `.github/workflows/`. Keep the pinned engine commit; all four must pin the same one.
```

new:

```markdown
2. Copy the four files of [`examples/claims-repo/.github/workflows/`](../examples/claims-repo/.github/workflows/) into the claims repo's `.github/workflows/`. They pin the latest release (its commit, with the tag as the comment); keep all four on the same one ([Upgrading the engine](#upgrading-the-engine)).
```

- [ ] **Step 5: Verify** — `go test ./... && go vet ./... && gofmt -l .`; both actionlint invocations; zizmor; `rg -n "# unreleased" examples internal/workflowspec` prints nothing.
- [ ] **Step 6: Commit, PR, merge** (side effect 6)

```bash
git add examples internal/workflowspec/rules_test.go docs/pipelines.md
git commit -m "chore(examples): pin the caller templates to v0.1.0 and accept only released engine pins"
git push -u origin chore/pin-templates-v0.1.0
gh pr create --repo jellalshadows-idp/idp-engine --base main --head chore/pin-templates-v0.1.0 --title "chore: pin the caller templates to v0.1.0" --body "Phase 1e Task 11 (ADR-0028): the four caller templates pin v0.1.0's commit with # v0.1.0, and checkUses no longer accepts # unreleased. The harness keeps pinning its commit under test as # unreleased in idp-claims-e2e."
```

`gh pr checks <number> --repo jellalshadows-idp/idp-engine --watch` (the six checks pass); a scoped review of the PR; then `gh pr merge <number> --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`. The merge's `release.yaml` run finds no user-facing commit (`chore`), so no release PR opens; `e2e on the release PR` is skipped.

---

### Task 12: Onboard `idp-claims`, and the first Renovate run

- [ ] **Step 1: Bootstrap check** with the verified `v0.1.0` binary (D-30):

```bash
GH_TOKEN="$(gh auth token)" "<scratchpad>/v0.1.0/bin/idp.exe" bootstrap check --org jellalshadows-idp --claims-repo idp-claims --approver jellalshadows --reader ~/.idp/apps/jellalshadows-idp-reader.json --writer ~/.idp/apps/jellalshadows-idp-writer.json
```

Expected: `bootstrap check: no drift`, exit 0. Any drift: stop and ask (`bootstrap apply` is not authorized here).

- [ ] **Step 2: The onboarding branch** (side effect 7), from the engine repo root on `main` after the Task 11 merge:

```bash
gh repo clone jellalshadows-idp/idp-claims "<scratchpad>/idp-claims" && git -C "<scratchpad>/idp-claims" switch -c chore/onboard-idp-pipelines
python -I -c "
import pathlib,shutil
src=pathlib.Path('examples/claims-repo/.github/workflows')
dst=pathlib.Path(r'<scratchpad>/idp-claims/.github/workflows')
dst.mkdir(parents=True, exist_ok=True)
for p in sorted(src.glob('*.yaml')):
    shutil.copyfile(p, dst/p.name); print('copied', p.name)
"
```

Write `<scratchpad>/idp-claims/config/platform.yaml` with the Write tool (D-29):

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jellalshadows-idp/idp-engine/v0.1.0/schemas/platform.json
apiVersion: idp/v1
kind: Platform
github:
  org: jellalshadows-idp
  writerAppId: 5255579
  archiveOnDestroy: true
  requiredApprovals: 1
naming:
  reservedPrefixes: [e2e-, spike-]
environments:
  dev:     { aws: { accountId: "000000000001", region: eu-west-1 } }
  staging: { aws: { accountId: "000000000002", region: eu-west-1 } }
  pro:     { aws: { accountId: "000000000003", region: eu-west-1 }, protected: true }
modules:
  allowedSources: ["git::https://github.com/jellalshadows-idp/"]
```

Expected: four `copied …` lines; `git -C "<scratchpad>/idp-claims" status --short` lists the four workflows and `config/platform.yaml`.

- [ ] **Step 3: Commit, push, open the PR**

```bash
git -C "<scratchpad>/idp-claims" add .github/workflows config/platform.yaml
git -C "<scratchpad>/idp-claims" commit -m "chore: onboard the idp pipelines at v0.1.0"
git -C "<scratchpad>/idp-claims" push -u origin chore/onboard-idp-pipelines
gh pr create --repo jellalshadows-idp/idp-claims --base main --head chore/onboard-idp-pipelines --title "chore: onboard the idp pipelines at v0.1.0" --body "Adds the four idp caller workflows, pinned to idp-engine v0.1.0, and config/platform.yaml (docs/pipelines.md in idp-engine). No claims yet."
```

If the push is refused for a missing `workflow` scope, run `gh auth refresh -h github.com -s workflow` and push again.

- [ ] **Step 4: The PR's checks** — `gh pr checks <number> --repo jellalshadows-idp/idp-claims --watch`. Expected: `pr / validate`, `pr / plan`, `pr / comment`, `pr / verdict` and `idp-gate` pass (the PR's own `pull_request` run reports the required check). Evidence:

```bash
r="$(gh run list --repo jellalshadows-idp/idp-claims --workflow pr.yaml --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run view "$r" --repo jellalshadows-idp/idp-claims --log | rg "installed idp v0.1.0|is not a release"
gh pr view <number> --repo jellalshadows-idp/idp-claims --json comments --jq '.comments[] | select(.body | contains("<!-- idp-plan -->")) | .body' | rg "No infrastructure changes."
```

Expected: only `setup-idp: installed idp v0.1.0, verified against <M>` lines; the plan comment contains `No infrastructure changes.`.

- [ ] **Step 5: Merge** (side effect 7) — `gh pr merge <number> --repo jellalshadows-idp/idp-claims --merge --delete-branch`.
- [ ] **Step 6: The first reconcile**

```bash
rc="$(gh run list --repo jellalshadows-idp/idp-claims --workflow reconcile.yaml --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$rc" --repo jellalshadows-idp/idp-claims --exit-status
gh run view "$rc" --repo jellalshadows-idp/idp-claims --json jobs --jq '.jobs[] | [.name, .conclusion] | @tsv'
gh run view "$rc" --repo jellalshadows-idp/idp-claims --log | rg "gate: "
```

Expected: `reconcile / plan	success`, `reconcile / approve	skipped`, `reconcile / apply-github	success`; `gate: auto (…)`; nothing applied, and `wet` holds the first render. Watch item: this is the first reconcile of a repo with no claims and no state file. If it fails, stop: rule on the cause, and fix it in the engine (test, PR, next patch release, owner asked first) or onboard with a first claim on the owner's word.

- [ ] **Step 7: Drift** (side effect 7) — `gh workflow run drift.yaml --repo jellalshadows-idp/idp-claims --ref main`, then find and watch the run as in Step 6. Expected: `drift / drift	success`, and `gh issue list --repo jellalshadows-idp/idp-claims --label drift --state open --json number --jq length` prints `0`.
- [ ] **Step 8: The first Renovate run** (side effect 5)

```bash
gh workflow run renovate.yaml --repo jellalshadows-idp/idp-engine --ref main
rv="$(gh run list --repo jellalshadows-idp/idp-engine --workflow renovate.yaml --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$rv" --repo jellalshadows-idp/idp-engine --exit-status
for r in idp-engine idp-claims; do
  gh issue list --repo "jellalshadows-idp/$r" --search "Dependency Dashboard in:title" --state open --json number,title,author --jq '.[] | [.number, .title, .author.login] | @tsv'
  gh pr list --repo "jellalshadows-idp/$r" --author "app/jellalshadows-idp-bot" --json number,title --jq '.[] | [.number, .title] | @tsv'
done
gh issue view <idp-claims dashboard number> --repo jellalshadows-idp/idp-claims --json body --jq .body | rg -c "jellalshadows-idp/idp-engine"
```

Expected: the run succeeds; each repository has one open `Dependency Dashboard` issue by `app/jellalshadows-idp-bot`; `idp-claims` has no Renovate PR (its callers already pin the latest release) and its dashboard lists the engine pins (count at least 1); any `idp-engine` PR is an update at least 14 days old, labelled by its group where one applies. Nobody merges them (D-2). Record in the ledger: the run log shows no `REPOSITORY_CHANGED` abort (`gh run view "$rv" --repo jellalshadows-idp/idp-engine --log | rg -c REPOSITORY_CHANGED` prints `0`), and the bot's missing administration and Dependabot alerts permissions produced at most warnings.

---

### Task 13: Ship II — execution log, README, SDD workspace

- [ ] **Step 1: Write `docs/phases/phase-1e.md`** in the style of `docs/phases/phase-1d.md`: the header list (period, plan, spec amended by A9, ADR-0028 and ADR-0029, previous phase, status with the merge commits and `v0.1.0`'s commit); summary; tasks with commits and reviews; every ruling (D-1 to D-33 and the execution rulings) with what, why and cost if wrong; credentials and environments; the release (the release PR number, its harness run and status, the release run, the assets, the local verification); the live evidence (the post-release harness run, the onboarding PR, the first reconcile and drift of `idp-claims`, the first Renovate run with its dashboards and PRs); the watch list with each result; notable findings caught by review; the final disposition of every finding. The disposition includes these carried items: Phase 1a final review minor #8 (`{{.Tag}}`) **Fixed** (Tasks 3 and 4); Phase 1a Task 11 minor (git `?ref=` sources after the first release) **Fixed** (Task 10 Step 10, or Task 12); Phase 1a Task 8 minor (the `-platform=` flags) **Fixed** (`docs/runbooks/dependencies.md`); Phase 0 M9 (the Go patch pin through Renovate) **Fixed** (the `go` directive rule); the Phase 0 note on a pinned actionlint binary **Won't fix** (the `go run` pin is exact and Renovate keeps it current); the Phase 1d Task 1 doc touch-ups and the re-pin upgrade note **Fixed** (Tasks 1 and 6); the harness on release PRs **Fixed** (Task 5); the provider documentation mismatch report **Phase 5**, optional; the narrower plan token stays an owner note (ADR-0025). Then "Carried into Phase 2", "Owner notes" (merge Renovate PRs by hand; merge a release PR only with `idp-e2e` green; the bot key rotation now covers `idp-e2e`, `idp-release` and `idp-renovate`; earlier owner items stay as recorded), process notes and status.

- [ ] **Step 2: Update `README.md`** (exact replacements with Edit)

1. Old:

```markdown
> **Status:** Phase 1d is complete: an end-to-end harness runs the pipelines against `idp-claims-e2e` nightly and on dispatch, on top of the reusable `pr`, `reconcile`, `drift` and `recover` workflows; see the [phase 1d log](docs/phases/phase-1d.md), [docs/e2e.md](docs/e2e.md) and [docs/pipelines.md](docs/pipelines.md). Phase 1e (releases, Renovate, `v0.1.0`, onboarding `idp-claims`) is next.

**What exists today:** `idp-claims-e2e` runs the pipelines against the org, checked nightly by the harness; `idp-claims` is bootstrapped and protected, and gets its workflows with the first release.
```

new:

```markdown
> **Status:** Phase 1e is complete: the engine is released with verified binaries, Renovate keeps every pin current, and the production `idp-claims` runs the pipelines at a release; see the [phase 1e log](docs/phases/phase-1e.md), the [releases runbook](docs/runbooks/releases.md) and [docs/pipelines.md](docs/pipelines.md). Phase 2 (the Component's AWS part) is next.

**What exists today:** `idp-claims` runs the pipelines at an engine release; `idp-claims-e2e` is checked by the end-to-end harness nightly and on every release PR.
```

2. Insert before `## Honest limitations (v1)`:

````markdown
## Releases

The engine ships as one version for everything (the CLI, the reusable workflows, the modules and the schemas), tagged `vX.Y.Z` and `0.x` until v1. release-please keeps a release PR on `main`; the end-to-end harness runs on it, and merging it tags the release. Each release carries `idp` for Linux, macOS and Windows (amd64 and arm64), `checksums.txt` and a build provenance attestation. Claims repos pin a release commit in their four caller workflows, the workflows run that release's verified binary, and Renovate moves the pins when a new release ships.

To install the CLI, download your platform's archive from the release page and verify it before you run it:

```bash
gh attestation verify <archive> --repo jellalshadows-idp/idp-engine \
  --signer-workflow jellalshadows-idp/idp-engine/.github/workflows/release.yaml
```

Runbooks: [releases](docs/runbooks/releases.md), [dependency updates](docs/runbooks/dependencies.md).
````

- [ ] **Step 3: Commit doc-only to `main`** (side effect 8) — the naming check prints nothing; then:

```bash
git add docs/phases/phase-1e.md README.md
git commit -m "docs: add the phase 1e execution log"
git push
```

The push runs `release.yaml`: the `docs` commit is user-facing, so release-please opens the next release PR and the harness runs on it. Leave that PR open: releasing it is the owner's call (not authorized here).

- [ ] **Step 4: Delete the SDD workspace** (side effect 9) — after checking that the phase log reproduces the ledger: `rm -rf .superpowers/sdd`, then `git status --short` prints nothing.






