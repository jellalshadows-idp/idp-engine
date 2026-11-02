# Phase 1e execution log: releases, Renovate, `v0.1.0` and onboarding `idp-claims`

- Period: 2026-10-18 to 2026-11-03
- Plan: [2026-10-18-phase-1e-releases.md](../superpowers/plans/2026-10-18-phase-1e-releases.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md), amended by A9
- Decisions: ADR-0028 (releases, and how the workflows get `idp`) and ADR-0029 (self-hosted Renovate with the bot App)
- Previous phase: [Phase 1d log](phase-1d.md)
- Status: **complete**. The phase merged through [PR #10](https://github.com/jellalshadows-idp/idp-engine/pull/10) (merge commit `6178fac`). The release PR [#11](https://github.com/jellalshadows-idp/idp-engine/pull/11) merged as `08ec386`, which is the commit of `v0.1.0`. The template re-pin merged through [PR #12](https://github.com/jellalshadows-idp/idp-engine/pull/12) (merge commit `3e3ab47`). The onboarding of `idp-claims` merged through [idp-claims#1](https://github.com/jellalshadows-idp/idp-claims/pull/1) (`2c3b199`).

This file is the durable copy of the execution ledger that was kept while the plan ran. The ledger lived in a git-ignored scratch folder that is deleted afterwards, so what it recorded is reproduced here: the rulings, each task's review outcome, the live runs, the disposition of every finding and what is carried forward.

## Summary

Phase 1e ships the engine as a release and puts production on it.

- **Releases.** One version for everything (the CLI, the reusable workflows, the modules and the schemas), tagged `vX.Y.Z`. release-please keeps a release PR on `main`; merging it tags the release and the same run builds the CLI with GoReleaser, uploads six archives and `checksums.txt` into that release and attests their build provenance (ADR-0028). `idp version` prints the build's version.
- **Download, verify, else build.** `setup-idp` keeps its two inputs. It looks for a `vX.Y.Z` tag on the pinned commit: with none it builds from source as before; with one it downloads the runner's archive, checks the checksum, runs `gh attestation verify`, requires `idp version` to print the tag, and installs. A tagged commit never falls back to a build.
- **The harness on release PRs.** When release-please opens or updates the release PR, `release.yaml` dispatches the end-to-end harness on `main`; the run posts the commit status `idp-e2e` on the PR head.
- **Renovate**, self-hosted with the bot App, weekly over `idp-engine` and `idp-claims`, with one global config, no automerge, 14 days for third-party releases and no wait for the engine's own (ADR-0029).
- **`v0.1.0`** was released and verified, the four caller templates were re-pinned to it, and the production `idp-claims` was onboarded at that release.

How it was executed: subagent-driven, as in Phases 1c and 1d. Each task had an implementer and a separate reviewer, with fix rounds on findings. Tasks 9 to 12 (verification, the push and merge, the environments and ruleset, the release, the re-pin and the onboarding) were run by the controller, and the PR for the re-pin was reviewed by a separate reviewer. A whole-branch review by the most capable model followed the push, and one fix wave closed it.

CI evidence on `idp-engine`, all green:

- run 41529570094 at `0389835` (`go`, `workflows`, `modules (github/group)`, `modules (github/component)`, `render-smoke`, `release-snapshot`), before the final review;
- run 41532368302 at `61b2b6b`, after the final fix wave, the last commit before the merge of PR #10;
- run 41546509457 on PR #12 (the template re-pin) before its merge.

## Tasks

"Review" is the review done after the task by a separate reviewer. Commit ranges are `base..tip` on `feat/phase-1e-releases`, created from `main` at `3730bed` (the plan commit).

| Task | What | Commits | Review |
|---|---|---|---|
| 1 | ADR-0028, ADR-0029, spec amendment A9 and the ADR amendments | `3730bed..eff9073` | Needs fixes (1 Important, 1 Minor), clean after 2 fix rounds |
| 2 | `idp version` | `eff9073..76e94e9` | Clean |
| 3 | GoReleaser config, its contract test and the CI snapshot | `76e94e9..6a513bd` | Clean |
| 4 | The release workflow, release-please and the releases runbook | `6a513bd..20ea2bd` (`476fabd`, fix round `a382cad..20ea2bd`) | Needs fixes (1 Important, 7 Minor), clean after 1 fix round |
| 5 | The harness on release PRs and the `idp-e2e` status | `20ea2bd..2e8ce25` (`bd10020`, fix round `2e8ce25`) | Needs fixes (2 Important, 2 Minor), clean after 1 fix round |
| 6 | `setup-idp` installs the verified release of a release commit | `2e8ce25..39fefc7` (`66ff161`, fix round `39fefc7`) | Needs fixes (1 Important, 5 Minor, 2 spec nits), clean after 1 fix round |
| 7 | The harness pins the commit under test as `# unreleased` | `39fefc7..ad61adb` | Clean |
| 8 | Self-hosted Renovate | `ad61adb..0389835` (`79a8ffe`, fix round `0389835`) | Needs fixes (2 Important, 4 Minor), clean after 1 fix round |
| 9 | Ship I: verification, PR #10, whole-branch review, fix wave, environments, tag ruleset, merge | PR #10; wave `37ccb2e`, `61b2b6b`; merge `6178fac` | Whole branch: with fixes (0 Critical, 1 Important, 5 Minor), all addressed on re-review |
| 10 | Release `v0.1.0` | release PR #11 (`8ad6b93`), merge `08ec386`; changelog tidy `4f1395c` | Not applicable (controller; verification below) |
| 11 | Re-pin the templates to `v0.1.0`, drop `# unreleased` | `cde4df1`, PR #12, merge `3e3ab47` | Approved |
| 12 | Onboard `idp-claims`, first Renovate run | idp-claims#1, merge `2c3b199` | Not applicable (controller; verification below) |
| 13 | This log and the README | this commit | Not applicable |

Task details:

- **Task 1.** `e915c6e` records ADR-0028 and ADR-0029, spec amendment A9 and the amended sentences of ADR-0018, ADR-0021, ADR-0026 and ADR-0027. Review found the spec §7.3 bot row had lost a colon and that the ADR-0009 note was missing (`590400e`); the note was then moved after the credentials table (`eff9073`) and a re-review found both addressed.
- **Task 2.** `76e94e9`. Three tests; the actual RED set included `TestVersionRejectsArguments` on stderr.
- **Task 3.** `6a513bd`. The contract test for `.goreleaser.yaml`, `bashMatch`, and the `release-snapshot` CI job.
- **Task 4.** `476fabd`. `release.yaml`, `release-please-config.json`, the manifest and `docs/runbooks/releases.md`. Review (the most capable model) found that the tests did not pin that the publish guards stop the job (`exit 1`, no `if`, no `continue-on-error`); fixed in `a382cad..20ea2bd` along with the minors and comments on the publish permissions.
- **Task 5.** `bd10020`. `e2e.yaml` gets `release-pr-sha` and posts `idp-e2e`. Review found the validate-before-post order and the step-scoped token unpinned; fixed in `2e8ce25`.
- **Task 6.** `66ff161`. The download, verification and install steps of `setup-idp`. Review found no path that runs an unverified binary. Findings: the `Limitations` wording the plan mandated, and the gate (see rulings); fixed in `39fefc7`.
- **Task 7.** `ad61adb`. The harness rewrites the callers' pin to `@<engineSHA> # unreleased`.
- **Task 8.** `79a8ffe`, `.github/renovate-global.json`, `renovate.yaml`, `docs/runbooks/dependencies.md`. Review found two Important issues (see "Notable findings caught by review"); fixed in `0389835`, which also changed the Renovate pin (ruling below).
- **Task 9.** See "Ship I".
- **Task 10.** See "The release".
- **Task 11.** `cde4df1`. The `# unreleased` cases of `TestCheckUses` flip to failures, the four templates pin `08ec386b36f80d338d5cbf3927848af018ca725b # v0.1.0`, and the exception is removed from `checkUses`. RED before the change: `TestCheckUses` failed on "reusable workflow marked unreleased".

## Rulings

A ruling is a controller or owner decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong. D-1 to D-4 are the owner's, D-5 and D-6 are the owner's Phase 1d decisions this plan carried out, and the owner approved the plan and chose subagent-driven execution on 2026-10-18.

Planning rulings (all recorded in the plan):

- **D-1 (owner). The harness runs on release PRs through a dispatch.** `release.yaml` dispatches `e2e.yaml` on `main` (the release PR's base) with `GITHUB_TOKEN`; `e2e.yaml` takes an optional validated input `release-pr-sha` and posts `idp-e2e` on that commit: `pending` first, then `success` or `failure`, even on failure. `idp-e2e` stays limited to `main`. *Why:* the harness could not run on a PR head. *Cost if wrong:* the harness tests `main` while the merge adds the changelog commit; the changelog cannot break the pipelines.
- **D-2 (owner). No automerge.** The owner merges every Renovate PR. *Cost if wrong:* the owner's time.
- **D-3 (owner). Renovate runs weekly over `idp-engine` and `idp-claims`,** not `idp-claims-e2e`, whose callers the harness pins on every run. *Cost if wrong:* a week of delay; a dispatch runs it at once.
- **D-4 (owner). The 14-day `minimumReleaseAge` applies to third-party dependencies only.** The engine's own releases have no minimum age. *Cost if wrong:* a broken engine release reaches `idp-claims` sooner; the harness ran on its release PR and the owner merges by hand.
- **D-5 (owner). Renovate is self-hosted with the bot App.** *Cost if wrong:* a different update bot later.
- **D-6 (owner). The workflows download and verify the release binary,** building from source only when the pinned commit has no release. *Cost if wrong:* the source build stays.
- **D-7. How a job gets `idp`.** `setup-idp` keeps exactly its two inputs. A `download` step runs first; `go` and `build` run only when it did not install. The lookup lists the engine's `v` tags, keeps refs matching `^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$` whose object is a commit equal to the pinned commit. An API error fails; no tag means a source build; several tags fail; one tag means checksum, `gh attestation verify` (repo, signer workflow, source digest, source ref) and an `idp version` check. *Why:* a fallback for a tagged commit would change the rendered `?ref=` from the tag to the SHA and hide a broken release. *Cost if wrong:* a broken release blocks its pins until it is republished.
- **D-8. `idp version`** prints `version.Version` and a newline and exits 0; an argument is a usage error (ADR-0016). *Cost:* none.
- **D-9. GoReleaser and the `{{.Tag}}` guard.** `.goreleaser.yaml` builds only `./cmd/idp` for linux, darwin and windows on amd64 and arm64, with the version injected from `{{.Tag}}`, `changelog.disable: true` and `release.mode: keep-existing`. The guard has two layers: a contract test on the config, and the publish job refusing a non-`vX.Y.Z` tag and checking the built binary. CI proves the config on every PR (`goreleaser check` and a snapshot). GoReleaser never runs locally. *Cost if wrong:* one more CI job.
- **D-10. `release.yaml` and release-please.** One package `.`, `release-type: go`, `bump-minor-pre-major`, `include-component-in-tag: false`, `include-v-in-tag: true`, changelog sections feat, fix, perf, refactor, docs and revert, with chore and test hidden. Jobs `release-please` (environment `idp-release`, bot token), `publish` (gated on `release_created`) and `e2e-release-pr`. Release PRs come from the bot token, so CI runs on them, and they merge with a merge commit. *Cost if wrong:* release-please is replaceable without touching the callers.
- **D-11. Two environments, `idp-release` and `idp-renovate`,** deployment branch `main`, each with the secret `IDP_BOT_PRIVATE_KEY` and the variable `IDP_BOT_CLIENT_ID`. The bootstrap runbook's bot key rotation covers all three environments. *Cost:* one more place per rotation.
- **D-12. Renovate.** Workflow `renovate.yaml` (weekly and on dispatch, environment `idp-renovate`), the action pinned by SHA and the Renovate version pinned exactly; one global config (`onboarding: false`, `requireConfig: "ignored"`, the bot's author, the dashboard, `automerge: false`, `minimumReleaseAge: "14 days"`, `internalChecksFilter: "strict"`); groups "idp engine" (no wait), the OpenTofu pins and the provider pins labelled `needs-lockfile`. The lock file is regenerated by hand (`docs/runbooks/dependencies.md`, with the `-platform=` flags). *Cost if wrong:* config only.
- **D-13. Tags never move.** The repository ruleset `idp-engine-tags` (target tag, `refs/tags/v*`, rules deletion, non_fast_forward, update, no bypass) is created in Task 9; creating a tag stays allowed. **GitHub immutable releases stay off:** they forbid adding assets to a published release, which the two-step publish needs. *Cost:* someone with write access could replace a published asset; the attestation check then fails every pipeline pinned to it instead of running the replacement.
- **D-14. Templates re-pin after the release.** The release commit's SHA is unknown inside the release, so a separate PR re-pins the four templates to `v0.1.0`'s commit and drops the `# unreleased` exception from `checkUses`. The harness meanwhile pins the commit under test as `# unreleased`. *Cost:* one PR.
- **D-15. Onboarding `idp-claims`** by a PR: the four callers copied from the re-pinned templates and `config/platform.yaml` with production values, no claims. `bootstrap check` runs first. *Cost:* production starts reconciling; it has no claims yet.
- **D-16. Docs and amendments** fold into the task whose change needs them. *Cost:* none.
- **D-17. `initial-version: "0.1.0"` in `release-please-config.json`.** The manifest `0.0.0` plus `bump-minor-pre-major` was expected to give `0.1.0`. It does not: release-please 17.6.0 skips a manifest version of `0.0.0` when it backfills the latest release, and with no latest release the version is `initial-version` or `1.0.0`. *Cost if wrong:* none; the `Release-As: 0.1.0` fallback remained available. Proven live (see "The release").
- **D-18. `always-update: true`.** Without it release-please leaves the release PR untouched when its body does not change, so a push of only hidden types would leave a green `idp-e2e` on a PR whose merge carries untested code. With it, every push to `main` rebuilds the release PR and re-dispatches the harness. *Cost:* one harness run (about 20 minutes) per push to `main` while a release PR is open.
- **D-19. `release.yaml` concurrency adds `queue: max`.** GitHub otherwise cancels an older waiting run, and a later push's run could create the release for a commit that is not its own. `.github/actionlint.yaml` ignores actionlint's "unexpected key queue" message for workflows. *Cost if wrong:* one ignore line.
- **D-20. The publish guard also requires the release's commit to be the run's commit,** because the attestation's source digest is the run's commit and `setup-idp` verifies it against the tagged commit. *Cost if wrong:* a release created by another run must be cut again as the next patch (releases runbook).
- **D-21. `release-please-action` v5.0.0 output names** were verified in its source: unprefixed `release_created`, `tag_name`, `sha` for the root path; `prs_created` is true for a created or updated release PR; `pr` has no reliable head SHA, so `e2e-release-pr` reads `headRefOid` with `gh pr view` and gets `pull-requests: read`. *Cost if wrong:* a live fix in Task 10.
- **D-22. Renovate writes no commit statuses.** Renovate 44's GitHub platform aborts the repository's run when a status write fails, and the bot App has `statuses: read`. With `internalChecksFilter: strict` the dashboard lists pending updates instead. Changing the App's permissions is an owner browser step the plan avoided. *Cost:* no Renovate branch statuses; artifact errors still appear as PR comments.
- **D-23. The Renovate config is strict JSON at `.github/renovate-global.json`** (the brief said `.github/renovate.json5`), so the Go contract test decodes it with `DisallowUnknownFields` and no new dependency. *Cost if wrong:* a rename.
- **D-24. CI validates the config with the Renovate image** (`renovate-config-validator --strict` in `ghcr.io/renovatebot/renovate`), not `npx`: the pinned Renovate needs a Node version the `ubuntu-24.04` image did not ship. *Cost:* a docker pull in CI.
- **D-25. Native Renovate managers where they exist; regex managers only where none does.** `setup-opentofu`'s `tofu_version` and `goreleaser-action`'s `version` are known actions of the github-actions manager; the provider pins come from the `terraform` manager. Regex managers cover actionlint, zizmor, the Renovate version, and the renderer's pins (`render.TofuVersion` is rendered as an exact `required_version`). A contract test keeps `render.TofuVersion` equal to CI's OpenTofu. *Cost if wrong:* config only.
- **D-26. `goreleaser check` fails CI on deprecated properties,** so the config uses `formats`, not `format`. *Cost:* none.
- **D-27. The publish job checks the version from the linux/amd64 archive,** the artifact callers download. *Cost:* none.
- **D-28. The `# unreleased` cases of `TestCheckUses` flip to failures in Task 11** instead of being deleted, so the RED step is a real failing test. *Cost:* none.
- **D-29. Production `config/platform.yaml`** uses spec §4.6's values: `writerAppId`, `archiveOnDestroy: true`, `requiredApprovals: 1`, reserved prefixes `e2e-` and `spike-`, environments `dev`, `staging` and `pro` (fake account ids, `pro` protected), and `modules.allowedSources`. The renderer ignores AWS until Phase 2. *Cost if wrong:* a config PR.
- **D-30. The release is checked locally with the released `windows/amd64` archive** (checksum, `gh attestation verify`, `idp version`), and that binary runs the `idp-claims` bootstrap check: no local build. *Cost:* none.
- **D-31. Spec amendment A9 also covers §3.1, §3.3, §6 and §7.4,** besides the §7.3, §7.5, §8.5 and §10 items, and ADR-0027's "Release PRs join in Phase 1e" is amended. *Why:* each states what Phase 1e changes. *Cost:* none.
- **D-32. The `renovate` job has `contents: read`** (the brief said `permissions: {}`): it checks out the repo to read its config file. *Cost:* none.
- **D-33. Not in Phase 1e:** the optional narrower plan token stays an owner note (ADR-0025); reporting the provider's documentation and code mismatch upstream stays optional.

Execution rulings:

- **Scan rulings.** A pre-flight scan of the plan (67 rows, 0 Critical, 2 Important, 10 Minor) produced these rulings. Scan #1: the provider-version regex in `TestOneOpenTofuAndProviderVersion` is anchored to the start of the line, because it first matched `required_version`. Scan #2: the Task 11 check becomes "no `# unreleased` under `examples`, exactly the three D-28 rows in `internal/workflowspec`", because the plan's check could never pass. Scan #3 to #12: the old ADR anchor is checked instead of the phrase; the branch already existed; the tag ruleset is shown as JSON "to recreate it"; two wording fixes; the plan's approximate RED lists are replaced by the actual RED sets; assertions that cannot fail alone are dropped; shared helpers (`wantBotToken`, `secretReaders`) are reused; `releaseAssets` and `ciGoreleaserVersion` move to Task 4; Task 4's guard test asserts the exact test and exit; forward links to later runbooks are accepted between commits. *Cost if wrong:* none.
- **Old anchor in historical plans.** The old `#73-two-github-apps-and-three-secret-scopes` anchor stays in the historical 1c and 1d plans and in the 1e plan text; plans are historical records, so the anchor check excludes `docs/superpowers`. *Cost if wrong:* two dangling links in old plans.
- **Task 4, build and sign share the publish job.** The `id-token` permission is visible to the GoReleaser build and to running the fresh `idp version`. Accepted: all code comes from `main` at the tagged commit, and GoReleaser OSS cannot publish a dist it did not build. *Cost if wrong:* weaker than SLSA-L3 build isolation.
- **Task 6, no `ifBody`.** The plan's `ifBody` helper and `TestIfBody` were not added: exact `wantLines` blocks replace them, so the helper would be dead code. *Cost:* none.
- **Task 6, fail-closed gate.** The `go` and `build` steps run when `steps.download.outputs.installed == 'false'` instead of `!= 'true'`. Only the explicit not-a-release decision builds; a download step that never wrote the output fails loudly at the first `idp` call. *Cost if wrong:* none functionally.
- **Renovate pin 44.132.5 instead of 44.133.0.** 44.132.5 was released on 2026-10-03, so it was past the 14-day age on 2026-10-18; 44.133.0 would have been 14 days old only at 18:42Z that day. The pin is 44.132.5, folded into Task 8's fix round; this supersedes an earlier ruling to delay the push. *Cost if wrong:* one Renovate patch older; Renovate bumps itself (it opened a PR for 44.171.1 in the first run).
- **Task 8, one owner per pin and unprefixed OpenTofu versions.** `extractVersion` on the OpenTofu rule so `tofu_version` is not written `v`-prefixed, and the `docker` dependency of the Renovate action disabled so the Renovate pin has one owner. *Cost if wrong:* none.
- **Final wave.** Fixed I1, M1, M2, M4 and M5 and the release-without-assets note (R1). M3 (template pins may get a digest-only Renovate PR before Task 11) needed no code: Task 11 landed before the first scheduled run (Monday 2026-11-09 04:17Z). *Cost if wrong:* one stray PR to close.
- **Declined-to-judge items of the final review,** each accepted and listed with its phase: the docs as a visible changelog section (each docs push updates the release PR and runs the harness, per D-10 and D-18); `chore(deps)` hidden, so dependency updates ship with the next feat or fix release; the Renovate image and GoReleaser pinned by exact version tag, not digest (spec §7.5 asks for exact versions); `--deny-self-hosted-runners` not added (the org has no self-hosted runners; Phase 5 hardening list); merging with a green `idp-e2e` not enforced by a ruleset (ADR-0028); fork-commit pins and the hard-coded engine repo predate this branch (Phase 5); the `idp-claims` `$schema` hint pinned to a tag is not bumped by Renovate (see disposition); the 60-day schedule rule against Renovate activity is unverifiable offline. *Cost if wrong:* each is listed with its phase.
- **Publish needs the owner's go.** Before merging the `v0.1.0` release PR, the controller showed the owner the generated notes (the first public release text) and waited for a go, because a publish is a stop-class action even though the plan authorized it. *Cost if wrong:* one short wait.
- **Pause and resume.** The owner paused the run on 2026-10-18 after Task 8's first fix round and resumed it on 2026-11-02. `origin/main` was unchanged since `3730bed` at the resume.
- **Changelog line removed after publication.** One changelog line was removed at the owner's request, from the `v0.1.0` release notes and from `CHANGELOG.md` (commit `4f1395c`, a `chore:` commit). Because `chore` is a hidden type, that commit also showed that hidden commit types open no release PR: the release run for it succeeded and opened none.

## Credentials and environments

- **Bot App.** The existing bot App (created in Phase 1d) is the identity of the release and Renovate jobs. Its installation covers `idp-engine`, `idp-claims` and `idp-claims-e2e`.
- **Environments on `idp-engine`.** `idp-release` and `idp-renovate` were created in Task 9, each with the secret `IDP_BOT_PRIVATE_KEY` (taken from the local key file, never printed) and the variable `IDP_BOT_CLIENT_ID`. Their deployment branch policy lists `main` only, verified after creation (the check Task 4's review asked for, since it is the only thing stopping a branch dispatch from using the bot key). `idp-e2e` from Phase 1d is unchanged.
- **Tag ruleset.** `idp-engine-tags` (id 26401195): target tag, `refs/tags/v*`, rules deletion, non_fast_forward and update, no bypass. It held after the release.
- **Tokens in the workflows.** `release.yaml` and `renovate.yaml` use bot tokens minted per job; `publish` uses the workflow token with contents, id-token and attestations write; the harness posts `idp-e2e` with `GITHUB_TOKEN` and `statuses: write` (step-scoped).
- **Production `idp-claims`.** `bootstrap check` with the released `v0.1.0` binary found no drift before onboarding (Task 12 Step 1).

## Ship I (Task 9)

1. Local verification passed: tests, vet, gofmt, actionlint (both invocations), zizmor with no findings, the naming checks and a clean tree.
2. The branch was pushed as PR #10. CI at `0389835` (run 41529570094) was green, with two watch items proven from the logs: the `release-snapshot` step printed `snapshot idp version: v0.0.0`, and the pinned Renovate 44.132.5 image validated the config (`Config validated successfully against 1 file(s)`).
3. The final whole-branch review (most capable model) ran in parallel on `3730bed..0389835`: **Ready to merge: with fixes**, 0 Critical, 1 Important, 5 Minor. The trust chain, the fail-closed gate, cross-task names and first-release mechanics had no gap. Triage of the deferred minors: none had to be fixed before the merge.
4. The fix wave (`37ccb2e`, `61b2b6b`) closed the findings; a scoped re-review found 6 addressed, 0 open, and no new breakage.
5. The environments and the tag ruleset were created, CI was green at `61b2b6b` (run 41532368302) and PR #10 merged as `6178fac`.

## The release (Task 10)

- **The release PR.** The release run for the merge of PR #10 (run 41532679709) finished with `release-please` success, `publish` skipped and the harness dispatch success. It opened the release PR [#11](https://github.com/jellalshadows-idp/idp-engine/pull/11), which proposed **0.1.0** (manifest `0.1.0`, changelog heading `## 0.1.0`): the `initial-version` ruling (D-17) was proven.
- **Its CI and harness.** CI on the release PR was green (the six checks). The harness ran on the release PR head `8ad6b93` (run 41532946346) and passed, and the commit status `idp-e2e` was `success`: D-1 was proven. In that run the `idp-claims-e2e` pin run built `idp` from source, because the pinned commit (`6178fac`) was not a release; the caller token listed the engine's tags and got an empty list (the watch item for the tag lookup, closed).
- **The decision.** On 2026-11-02 the owner decided to publish `v0.1.0` after seeing the release notes.
- **The merge and the publish.** PR #11 merged as `08ec386`. The release run (run 41542891997) finished with `release-please` success and `publish` success: both publish guards passed, the built binary printed `idp version: v0.1.0`, and the provenance attestation covered 7 subjects. The e2e dispatch job was skipped.
- **The release.** `v0.1.0` is not a draft and not a prerelease. The tag is lightweight and points at `08ec386`. The release carries the release-please notes (so `keep-existing` kept them), six archives (Linux, macOS and Windows, amd64 and arm64) and `checksums.txt`. The ruleset held.
- **Local verification (D-30).** The checksums matched for the windows/amd64 and linux/amd64 archives, `gh attestation verify` (gh 2.89.0) succeeded for both (source ref `refs/heads/main`, source digest `08ec386`, signer workflow `release.yaml`), and the Windows binary printed `v0.1.0` for `idp version`. Note: without a terminal, `gh attestation verify` prints nothing on success and exits 0; `--format json` shows the evidence.
- **The changelog line.** One changelog line was removed at the owner's request (see "Rulings"): from the published release body with `gh release edit` (the 7 assets intact) and from `CHANGELOG.md` in `4f1395c`.

## Live evidence after the release

**The download path (Task 10 Step 10).** The harness was dispatched on `main` at `08ec386` (run 41543231306) and passed: its pin PR #97 moved the `idp-claims-e2e` callers to `08ec386` (further PRs #98 to #101). The `idp-claims-e2e` pin run (41543344058) logged `setup-idp: installed idp v0.1.0, verified against 08ec386...`, so the runner's `gh attestation` flags and the caller token's reads of the engine's releases and attestations work live. That run's pin plan was `No infrastructure changes.`. The create run (41543677844) ran `tofu init` and downloaded `git::https://github.com/jellalshadows-idp/idp-engine.git?ref=v0.1.0` for both modules, which closes the Phase 1a Task 11 minor.

**Re-pin (Task 11).** PR #12 (`cde4df1`, merge `3e3ab47`), CI run 41546509457 green. The `# unreleased` check: no match under `examples`, and exactly the three expected rows in `internal/workflowspec`.

**Onboarding `idp-claims` (Task 12).**

- The `chore/onboard-idp-pipelines` branch carries the four callers copied from the re-pinned templates and `config/platform.yaml` (D-29); the released binary validated it locally (`validate: ok`, 0 groups, 0 components).
- [idp-claims#1](https://github.com/jellalshadows-idp/idp-claims/pull/1): all checks passed (`pr` run 41547010435). `idp-gate` was reported by the PR itself, `setup-idp` installed `idp v0.1.0` verified against `08ec386` in the production caller, and the plan comment said `No infrastructure changes.`. It merged as `2c3b199`.
- **First production reconcile** (run 41547253418): `plan` success, `approve` skipped, `apply-github` success, `gate: auto` with no changes to apply; the `wet` branch holds `rendered/github/main.tf.json` and its lock file. The first reconcile of a repo with no claims and no state works.
- **Drift** (run 41547416165): `drift / drift` success and 0 open drift issues.

**The first Renovate run** (run 41547420457, dispatched): success, `REPOSITORY_CHANGED` 0, one warning (cannot access vulnerability alerts; expected, the App has no Dependabot alerts permission).

- `idp-engine`: Dependency Dashboard #24 by the bot, and 11 PRs (#13 to #23) waiting for the owner (D-2): `actions/download-artifact` 8.0.2, `actions/upload-artifact` 7.0.2, GoReleaser 2.18.3, `renovatebot/github-action` 46.3.9, `actions/create-github-app-token` 3.3.0, OpenTofu 1.13.1, Renovate 44.171.1, zizmor 1.31.1, `goreleaser-action` 7.3.0, `golang.org/x/crypto` 0.58.0 and `golang.org/x/text` 0.43.0.
- The OpenTofu PR (#18) moves all five pins to 1.13.1 unprefixed in one PR (`setup-idp`, `ci.yaml` twice, `render.TofuVersion` and the golden file): the Task 8 Important fix, proven live.
- `idp-claims`: Dependency Dashboard #2 and no PR. The dashboard lists the four engine pins as `v0.1.0@08ec386`, so Renovate reads reusable-workflow pins that carry version comments. Updating them is proven only by the first release after `v0.1.0`.

## Review Focus

| # | Case | Pinned by | Result |
|---|---|---|---|
| 1 | The tag lookup fails | `TestSetupIDPFailsWhenTheTagListingFails` | Tested; the gate is fail-closed (ruling) |
| 2 | A tagged commit whose release lacks its archive or checksums | `TestSetupIDPNeverBuildsATaggedCommit` | Tested |
| 3 | A non-semver tag | `TestReleasePublishRefusesANonSemverTag`, `TestSetupIDPIgnoresNonReleaseTags` | Tested |
| 4 | The harness fails early with `release-pr-sha` set | `TestE2EReportsEveryOutcomeOnTheReleasePR` | Tested; mutation-checked in Task 5 |
| 5 | The four engine pins bumped separately | `TestRenovateMovesTheEnginePinsTogether` and `TestRenovateRules` (the no-wait half) | Tested; the "idp engine" group is on the `idp-claims` dashboard |

Also tested, outside the five: `TestSetupIDPPicksTheRunnerArchive` and `TestReleasePleaseNamesAMissingCredential`.

## Watch list

| Item | Result |
|---|---|
| The runner's `gh` supports `--signer-workflow`, `--source-digest` and `--source-ref` | Seen live: run 41543344058 and the `idp-claims` PR checks |
| The caller job's `GITHUB_TOKEN` reads the engine's tags, release assets and attestations | Seen live in `idp-claims-e2e` and `idp-claims` |
| `release-please-action` output names (D-21) | Seen live: the release PR run and the release run |
| GoReleaser `keep-existing` uploads into the release and keeps its notes | Proven (run 41542891997) |
| The first release is `0.1.0` (D-17) | Proven (release PR #11) |
| The snapshot `{{.Tag}}` is `v0.0.0` without tags | Proven (CI run 41529570094) |
| The Renovate image exists and validates the config | Proven (44.132.5) |
| The harness at the release commit plans nothing and `?ref=v0.1.0` resolves | Proven (runs 41543344058 and 41543677844) |
| `idp-gate` on the onboarding PR; first reconcile with no claims and no state | Proven (idp-claims#1, run 41547253418) |
| `matching-refs` returns an empty list (not 404) when there are no tags | Proven: the pre-release harness run (41532946346) built from source |
| Renovate finishes both repos and lists the reusable-workflow pins | Proven (run 41547420457); updating such a pin awaits the next release |
| `idp-release` deployment branch policy lists only `main` | Verified at creation |
| The `idp-e2e` result step runs after a job timeout (Task 5 review) | Not recorded as observed |
| `job.status` in the harness `always()` step reports `failure` or `cancelled` after a failed step | Not recorded as observed (the release PR run passed) |
| Release-please still updates its PR when a change to `.github/workflows` lands on `main` (the bot token has no `workflows: write`) | Not recorded as observed |

## Notable findings caught by review

| Finding | Where caught | Fix |
|---|---|---|
| The spec §7.3 bot row lost a colon; the ADR-0009 note was missing | Task 1 review | `590400e`, `eff9073` |
| Tests did not pin that the publish guards stop the job (`exit 1`, `if`, `continue-on-error`) | Task 4 review (Important) | `a382cad..20ea2bd` |
| The validate-before-post order and the step-scoped token were unpinned in the harness status | Task 5 review (2 Important) | `2e8ce25` |
| `Limitations` wording the plan mandated; `!= 'true'` gate would build if the download step wrote nothing | Task 6 review (Important) and ruling | `39fefc7` |
| The OpenTofu group wrote a `v`-prefixed `tofu_version`; the Renovate pin had two owners | Task 8 review (2 Important) | `0389835` |
| The new Task 8 test assertions were weak in RED (masked by a strict-decode failure) | Task 8 fix round | The re-review checked that they fail on their own |
| The upgrade docs promised a plan comment that lists module source moves; the comment shows only resource changes | Final review (Important) | `61b2b6b` |
| Renovate's default concurrent PR limit could hold the engine bump behind third-party PRs | Final review M1 | `37ccb2e`, `prConcurrentLimit: 0`, pinned by a test |
| The lock-file recipe hard-coded the OpenTofu version; the release-PR merge rule missed "up to date with `main`"; the `e2e-release-pr` permissions were uncommented | Final review M2, M4, M5 | `37ccb2e`, `61b2b6b` |
| Renovate can propose a release whose publish failed | Final review (R1) | A bullet in the releases runbook |

Findings checked and found not to be problems: the final review found no path that runs an unverified binary; the Task 6 review likewise.

## Final disposition of every finding

Statuses: **Fixed** (done, with the commits), **Phase 2**, **Phase 5**, **Won't fix** (consciously rejected, with the reason), **Owner note**. No finding is left open without a destination.

### Fixed

- Task 1 review findings: **Fixed**, `590400e` and `eff9073`.
- Task 4 Important finding and its Minors 1 to 6: **Fixed**, `a382cad..20ea2bd`; Minor 7 was ruled (see rulings).
- Task 5 Important findings and the two documentation Minors: **Fixed**, `2e8ce25`.
- Task 6 Important finding, Minors 1, 2, 4, 5 and the spec nits: **Fixed**, `39fefc7`; Minor 3 became the fail-closed gate ruling.
- Task 8 Important findings and Minors (test name, `prHourlyLimit`, bootstrap wording, derivation) and the Renovate pin: **Fixed**, `0389835`.
- Final review I1, M1, M2, M4, M5 and R1: **Fixed**, `37ccb2e` and `61b2b6b`; M3 needed no code.
- Phase 1a final review minor #8 (`{{.Tag}}` into the version): **Fixed**, Tasks 3 and 4, proven by the snapshot (`v0.0.0`) and by the release (`v0.1.0`).
- Phase 1a Task 11 minor (git `?ref=` sources after the first release): **Fixed**, Task 10 Step 10 (the create run downloaded `?ref=v0.1.0` for both modules).
- Phase 1a Task 8 minor (the `-platform=` flags): **Fixed**, `docs/runbooks/dependencies.md`.
- Phase 0 M9 (the Go patch pin through Renovate): **Fixed**, the `go` directive rule in the Renovate config.
- Phase 1d Task 1 doc touch-ups and the re-pin upgrade note: **Fixed**, Tasks 1 and 6.
- The harness on release PRs: **Fixed**, Task 5, proven live on PR #11.
- The release machinery, Renovate, `v0.1.0`, the templates re-pin and onboarding `idp-claims` (the Phase 1d "Carried into Phase 1e" list): **Fixed**, Tasks 4 to 12.

### Phase 2

- Nothing from this phase is a prerequisite of Phase 2 other than the onboarded `idp-claims` at a release. The AWS part of the Component is Phase 2's scope.

### Phase 5

- The provider documentation and code mismatch for `github_repository_topics` (an empty array does not clear topics): the upstream report stays **optional** (D-33).
- Deferred Minors of Tasks 2, 3, 4, 5, 7, 8 and 11, which the ledger recorded without naming a phase. They are treated as in Phase 1d (low risk, fail closed or cosmetic): `TestVersionRejectsArguments` does not cover an unknown flag, and the usage-line test pins exact text (Task 2); the no-token check in the GoReleaser test covers step env only, `bashMatch` ends at the first space-`]]`, and the goos/goarch lists are order-sensitive (Task 3); `wantLines` accepts a block anywhere in the script, and the GoReleaser step is not in the `if`/`continue-on-error` loop (Task 4); the step-scoped token scan matches only the literal `github.token` and `secrets.github_token` forms (Task 5); the pin regex owns the whole line tail after the SHA, so a future trailing comment would be dropped, and there is no multi-ref test (Task 7); the plan text still says Renovate 44.133.0 (historical; this log records 44.132.5) (Task 8); the `pr.yaml` header comment wraps unevenly (Task 11).
- The declined-to-judge items listed in the rulings: `--deny-self-hosted-runners`, fork-commit pins and the hard-coded engine repo, and the Phase 1d items already assigned to Phase 5.

### Won't fix

- The Phase 0 note on a pinned actionlint binary: the `go run` pin is exact and Renovate keeps it current.
- A merge-with-green-`idp-e2e` ruleset: not enforced, per ADR-0028 (the runbook states the rule).
- GitHub immutable releases: off on purpose (D-13).

### Owner note

- The optional narrower plan token stays an owner note (ADR-0025).
- The `$schema` hint in `idp-claims`'s `config/platform.yaml` is pinned to a tag and Renovate does not bump it; update it by hand at a release.

## Carried into Phase 2

- Phase 2 is the Component's AWS part (README status). The onboarded production `idp-claims` has no claims yet, and `config/platform.yaml` carries fake AWS account ids for `dev`, `staging` and `pro`; the renderer ignores AWS until then (D-29).
- Updating an engine pin through Renovate in `idp-claims` is proven only by the first PR after the next release.
- Carried to Phase 5: the deferred Minors and hardening items above, and the earlier Phase 1d and 1c items recorded in their logs.

## Owner notes

- Merge Renovate PRs by hand (D-2). The first run left 11 PRs in `idp-engine` (#13 to #23); the Dependency Dashboards are `idp-engine` #24 and `idp-claims` #2.
- Merge a release PR only with `idp-e2e` green on its current head and with the PR up to date with `main` (the releases runbook).
- The bot key rotation now covers three environments in `idp-engine`: `idp-e2e`, `idp-release` and `idp-renovate` ([docs/runbooks/bootstrap.md](../runbooks/bootstrap.md)).
- This log is a `docs` commit, which is a user-facing type, so release-please is expected to open the next release PR (0.1.1). It stays open until the owner decides to release it.
- The earlier owner items stay as recorded in the [Phase 1d log](phase-1d.md) ("Owner notes") and earlier; they are not new asks.

## Process notes

- **Execution:** subagent-driven. Fix rounds used: Task 1 (2), Task 4 (1), Task 5 (1), Task 6 (1), Task 8 (1), the final wave (1).
- **Naming rule:** the naming checks ran on each task's work and on this log, and were clean.
- **Local verification constraint:** no local container tooling; GoReleaser, the Renovate validator and the snapshot run only in CI.
- **Process tool:** the workspace tooling of the process changed paths between the pause and the resume (the scripts moved to a newer version of the tool). It had no effect on the repository.
- **Ledger note:** the ledger and the task briefs and reports lived in `.superpowers/`, which is excluded from git; this log is their durable copy.

## Status

**Phase 1e is complete.** `v0.1.0` is published with verified binaries at `08ec386`; the reusable workflows install that binary for a release commit and build from source otherwise; the harness reports `idp-e2e` on release PRs; Renovate ran over both repositories; the four templates and the production `idp-claims` pin the release. This is shown by green CI runs (41529570094, 41532368302, 41546509457), the release run 41542891997, the post-release harness run 41543231306, the onboarding PR checks, the first production reconcile (41547253418) and drift (41547416165) runs, and the first Renovate run (41547420457). Every finding has a final disposition above.

Next: Phase 2 (the Component's AWS part).
