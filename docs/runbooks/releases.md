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

The repository ruleset `idp-engine-tags` forbids deleting, force-moving or updating `refs/tags/v*`, with no bypass; creating a tag stays allowed, so release-please can tag. To recreate it, save this as `idp-engine-tags.json` and run `gh api -X POST repos/jellalshadows-idp/idp-engine/rulesets --input idp-engine-tags.json`:

```json
{
  "name": "idp-engine-tags",
  "target": "tag",
  "enforcement": "active",
  "conditions": {
    "ref_name": {
      "include": ["refs/tags/v*"],
      "exclude": []
    }
  },
  "rules": [
    {"type": "deletion"},
    {"type": "non_fast_forward"},
    {"type": "update"}
  ],
  "bypass_actors": []
}
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
