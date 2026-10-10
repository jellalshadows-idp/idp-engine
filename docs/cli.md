# CLI reference

Every command shares one exit-code convention ([ADR-0016](adr/0016-exit-codes.md)):

| Code | Meaning |
|---|---|
| 0 | done; nothing needs attention |
| 1 | the command could not do its job |
| 2 | usage error |
| 3 | done, and the answer needs attention (invalid claims, drift) |

Commands that talk to GitHub read a token from `GH_TOKEN`, else `GITHUB_TOKEN`. Inside GitHub Actions, commands write step outputs to `GITHUB_OUTPUT`, and `encryption-env` writes to `GITHUB_ENV`.

Other environment variables:

- `IDP_GITHUB_API` overrides the GitHub API base URL (used by tests).
- `GITHUB_ACTIONS=true` turns on error annotations; `GITHUB_WORKSPACE` is the directory annotation paths are made relative to.
- When `GITHUB_OUTPUT` or `GITHUB_STEP_SUMMARY` is empty or unset (outside Actions), writing to it is skipped silently. `GITHUB_ENV` is the exception: `encryption-env` requires it and exits 2 without it.

Commands are listed in the order of `idp help`. Flags are written as `--name VALUE`; flags in `[brackets]` are optional.

## validate

Validates a claims repo: schema and semantic checks.

```text
idp validate [--dir DIR]
```

- `--dir`: claims repo root (default `.`).

Reads: the claims under `--dir`, and `GITHUB_ACTIONS` and `GITHUB_WORKSPACE` for annotations.

Writes: `validate: ok (N group(s), M component(s))` on success. Every problem goes to stderr; inside GitHub Actions each problem is also printed to stdout as an `::error` annotation, with its file path made relative to the workspace. See [claims.md](claims.md) for what is checked.

Exit codes: 0 valid; 1 the claims could not be loaded; 2 usage error; 3 invalid claims.

## render

Renders a claims repo into OpenTofu stacks.

```text
idp render [--dir DIR] [--out DIR] [--module-ref REF] [--modules-dir DIR]
```

- `--dir`: claims repo root (default `.`).
- `--out`: output directory (default `rendered`). It is created, or replaced if it holds a previous render.
- `--module-ref`: the idp-engine git ref that module sources pin. Default: this build's version.
- `--modules-dir`: use local module sources from this directory instead (validation only). It must be on the same volume as `--out`.

A development build has no version to pin, so it needs `--module-ref` or `--modules-dir`.

Reads: the claims under `--dir` (validated first, like `validate`).

Writes: the rendered tree under `--out`, including the `.idp-rendered` marker file, and one `render: wrote PATH` line per file. `idp diff` requires that marker.

Exit codes: 0 rendered; 1 the render or the write failed; 2 usage error (including a development build with neither flag); 3 invalid claims, nothing rendered.

## diff

Lists the stacks whose render differs from the wet branch.

```text
idp diff --new DIR --wet DIR [--all]
```

- `--new`: a fresh render, the `--out` of `idp render`. It must hold the `.idp-rendered` marker, otherwise the command fails.
- `--wet`: the `rendered/` directory of the wet branch checkout. It may not exist yet (the first reconcile).
- `--all`: treat every stack as affected, for a manual reconcile.

Each stack is reported with one status:

| Status | Meaning |
|---|---|
| `new` | only in the fresh render |
| `changed` | in both, with different files |
| `unchanged` | in both, byte for byte |
| `orphan` | only in wet: its claims are gone |

A stack is a directory that holds `main.tf.json`. `.terraform` directories are skipped.

Reads: both trees.

Writes: one `PATH: STATUS` line per stack and a `diff: N affected stack(s)` line. Step output `affected`: a JSON array of the stack paths that are `new`, `changed` or `orphan` (every stack of the fresh render with `--all`, plus the orphans).

Exit codes: 0 done; 1 the trees could not be read or `--new` is not a render; 2 usage error.

## plan-summary

Summarizes OpenTofu plans as a pull request comment and a fingerprint.

```text
idp plan-summary --stacks JSON --head-sha SHA --comment-out FILE --fingerprint-out FILE [--run-url URL] [--plan STACK=FILE]...
```

- `--stacks`: required. The `affected` JSON array written by `idp diff` (for example `["github"]`, or `[]` when nothing is affected). The set of `--plan` stacks must equal this set exactly. A missing plan must fail here, never read as "no changes": the gate and the reconcile would otherwise decide on a partial view and apply a plan nobody saw.
- `--head-sha`: the commit the plans are for, 40 hex characters.
- `--comment-out`: where to write the comment Markdown.
- `--fingerprint-out`: where to write the fingerprint JSON.
- `--run-url`: link to the workflow run, shown in the comment.
- `--plan STACK=FILE`: the `tofu show -json` output of one stack. Repeatable; a stack may be given only once.

Reads: each plan file. A plan with an action the fingerprint does not know is a failure, never treated as harmless ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).

Writes:

- The comment file. It carries the marker `<!-- idp-plan -->` and a hidden fingerprint marker, `<!-- idp-fingerprint:v1 sha=<head SHA> data=<base64(gzip(JSON))> -->` ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).
- The fingerprint JSON file, which `idp gate` reads.
- One `STACK: +create ~update -delete ±replace` line per stack, and a `plan-summary: N change(s), destructive: BOOL` line.
- Step outputs `changes` (the number of changes) and `destructive` (`true` or `false`: any delete or replace).

Exit codes: 0 done; 1 a plan could not be read or parsed, or a file could not be written; 2 usage error (a missing flag, a `--head-sha` that is not a full SHA, a stack given twice, `--stacks` that is not a JSON array of unique names, an affected stack with no `--plan`, a `--plan` for a stack that is not in `--stacks`).

## gate

Decides whether a reconcile applies automatically or waits for approval.

```text
idp gate --repo OWNER/REPO --sha SHA --fingerprint FILE
```

- `--repo`: the claims repository.
- `--sha`: the commit being reconciled, 40 hex characters.
- `--fingerprint`: the fingerprint JSON written by `idp plan-summary` for this reconcile.

Reads: the fingerprint file. When there are changes and none is destructive, it also reads GitHub (token required) to find the pull request merged as `--sha` and its plan comment. In the other cases the answer is already known and GitHub is not called.

Writes: a `gate: DECISION (REASON)` line, the step output `decision`, which is `auto` or `approval`, and a line in the job summary (`GITHUB_STEP_SUMMARY`).

The rules, in order ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)):

1. No changes: `auto`.
2. Any delete or replace: `approval`.
3. No trusted plan fingerprint: `approval`.
4. A change missing from the same stack of the trusted fingerprint: `approval`.
5. Otherwise: `auto`.

A fingerprint is trusted only when it is in the newest comment written by `github-actions[bot]` (type `Bot`) that contains `<!-- idp-plan -->`, on the pull request whose merge commit is `--sha`, and only when the marker's SHA equals that pull request's head SHA. Comments by anyone else are ignored, so a stranger cannot even force an approval prompt.

The gate reports its answer through the output. It exits 0 for both decisions.

Exit codes: 0 decided; 1 the fingerprint or GitHub could not be read; 2 usage error (including a missing token when GitHub is needed).

## comment

Creates or updates the sticky plan comment on a pull request.

```text
idp comment --repo OWNER/REPO --pr NUMBER --body-file FILE [--marker MARKER]
```

- `--repo`: the repository.
- `--pr`: the pull request number.
- `--body-file`: the comment Markdown. It must contain the marker.
- `--marker`: the marker that identifies the sticky comment (default `<!-- idp-plan -->`).

Reads: the body file; the pull request's comments through the GitHub API.

Writes: it edits the newest comment that carries the marker and was authored by `github-actions[bot]`, or creates one. It never edits a comment by anyone else, so a pull request keeps a single plan comment. Prints `comment: created|updated ID on #PR`.

Run it with the workflow's `GITHUB_TOKEN`, so the comment is authored by `github-actions[bot]`. The gate trusts only that author ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)); a comment made with any other token is created, but never trusted, and the next run would add a second comment instead of editing it. `GH_TOKEN` takes precedence over `GITHUB_TOKEN` when both are set, so do not set `GH_TOKEN` on this step.

Exit codes: 0 done; 1 the body is missing the marker, or GitHub failed; 2 usage error (including no token).

## issue

Opens or closes a labelled issue (drift, failed wet pushes).

```text
idp issue open  --repo OWNER/REPO --label LABEL --title TITLE --body-file FILE
idp issue close --repo OWNER/REPO --label LABEL --title TITLE [--comment TEXT]
```

- `--repo`, `--label`, `--title`: identify the issue. There is one open issue per label and title; pull requests are never matched.
- `--body-file`: the issue body (`open` only, required).
- `--comment`: a comment to leave before closing (`close` only).

Reads: the body file; the repository's open issues through the GitHub API.

Writes:

- `open` creates the label when it does not exist, then replaces the body of the open issue with that label and title, or creates it. Prints `issue: opened #N` or `issue: updated #N`.
- `close` adds the comment (when given) and closes the issue as completed. When no such issue is open it does nothing and prints `issue: none open`.

Like `comment`, run it with the workflow's `GITHUB_TOKEN`, so the issue is authored by `github-actions[bot]`; `GH_TOKEN` takes precedence over `GITHUB_TOKEN` when both are set.

Exit codes: 0 done (including nothing to close); 1 GitHub failed; 2 usage error (including no token).

## wet-push

Commits files to the wet branch in one commit.

```text
idp wet-push --repo OWNER/REPO --root DIR --message TEXT --path PATH... [--branch BRANCH]
```

- `--repo`: the claims repository.
- `--root`: a local directory laid out like the branch. It must exist.
- `--message`: the commit message.
- `--path`: a clean, relative slash path under `--root`, a file or a directory. Repeatable, at least one.
- `--branch`: the branch to commit to (default `wet`).

Reads: the files under `--root`; the branch's current tree through the GitHub API.

Writes: it makes the branch's files under each `--path` equal to the files under `--root`/`PATH`, in one commit on top of the current head ([ADR-0019](adr/0019-wet-commits-through-the-git-data-api.md)). Files that exist on the branch but not locally are deleted in that commit. The commit is made through the Git Data API, so GitHub signs it and it is verified. The branch is updated without force.

Safeguards:

- A path that is a single file on the branch (the state file, for example) is never deleted: if the local copy is missing, the command fails.
- `.terraform` directories are never synced, and a path that contains one is rejected.
- A truncated tree listing is refused, because deletions cannot be computed from a partial listing.

Prints `wet-push: SHA (N changed, M deleted)`, or `wet-push: no changes`. Step output `commit`: the new commit SHA, or empty when nothing changed (no commit is made then).

Run it with the writer token, since the wet ruleset admits only the writer App.

Exit codes: 0 done (including no changes); 1 the sync or the branch update failed; 2 usage error (including no token).

## encryption-env

Exports `TF_ENCRYPTION` for OpenTofu.

```text
idp encryption-env
```

It takes no flags.

Reads: `IDP_STATE_PASSPHRASE` (valid UTF-8, a single line, at least 16 characters) and `GITHUB_ENV`. Run it as a workflow step, with the repo secret passed as `IDP_STATE_PASSPHRASE`.

Writes: `TF_ENCRYPTION` to `GITHUB_ENV`, for the next steps. It is a `pbkdf2` key provider and an `aes_gcm` method, both named `idp`, used for state and plan. The name `idp` is frozen: OpenTofu records it in the encrypted state ([ADR-0020](adr/0020-tf-encryption-from-idp.md)). If the passphrase is missing or invalid, it prints an error that names the secret (as an annotation inside GitHub Actions) and writes nothing.

Exit codes: 0 `TF_ENCRYPTION` set; 1 the passphrase is missing or invalid; 2 usage error (including `GITHUB_ENV` not set, that is, not running in GitHub Actions).

## bootstrap

Creates or verifies the protections an org needs before the IDP runs. These run outside the IDP, on purpose ([ADR-0011](adr/0011-bootstrap-outside-the-idp.md)).

```text
idp bootstrap app   --org ORG --role reader|writer [--out-dir DIR] [--listen ADDR]
idp bootstrap apply --org ORG --claims-repo REPO --approver LOGIN --reader FILE --writer FILE --passphrase-file FILE
idp bootstrap check --org ORG --claims-repo REPO (--approver LOGIN --reader FILE --writer FILE | --params-env NAME) [--allow-hidden-bypass]
```

`FILE` for `--reader` and `--writer` is the `<slug>.json` written by `bootstrap app`.

### bootstrap app

Creates a GitHub App (reader or writer) for the org through the App manifest flow.

- `--org`: the organization that will own the App.
- `--role`: `reader` or `writer`.
- `--out-dir`: where to write `<slug>.json` and `<slug>.pem` (default `~/.idp/apps`).
- `--listen`: the local address of the callback server (default `127.0.0.1:0`). It must be a loopback address, because the server receives the App credentials.

Writes: the two credential files, and a link to install the App on the org with access to all repositories. It does not call GitHub with a token.

Exit codes: 0 created; 1 the flow failed; 2 usage error.

### bootstrap apply

Makes the org match the desired state. It is safe to run any number of times: a second run against an unchanged org performs no writes.

- `--org`, `--claims-repo`: the organization and the claims repository name.
- `--approver`: the login of the platform admin who approves runs.
- `--reader`, `--writer`: the App files from `bootstrap app`. `apply` also reads the private keys they point to.
- `--passphrase-file`: a file with the OpenTofu state passphrase.

Reads: the App files and the passphrase file; the token (`GH_TOKEN`, else `GITHUB_TOKEN`) needs the `admin:org` scope.

Writes, through the GitHub API: the org's workflow permissions (read-only), the claims repository (public), the orphan `wet` branch, the rulesets, the environments, the secrets (existing secrets are kept; delete one in GitHub to rotate it) and the variables, including `IDP_BOOTSTRAP`, the non-secret identities that `check --params-env` reads ([ADR-0017](adr/0017-drift-check-token.md)). One line per change; ends with `bootstrap apply: done`.

Exit codes: 0 done; 1 invalid input or GitHub failed; 2 usage error (including `--params-env` or `--allow-hidden-bypass`, which are check-only).

### bootstrap check

Reports drift without changing anything.

- `--org`, `--claims-repo`: as in `apply`.
- `--approver`, `--reader`, `--writer`: the identities, as in `apply` (key material is not needed).
- `--params-env NAME`: read the identities from the environment variable `NAME` (`IDP_BOOTSTRAP` in workflows) instead. It cannot be combined with `--approver`, `--reader` or `--writer`.
- `--allow-hidden-bypass`: report ruleset bypass actors that the token cannot see as notices instead of drift. The reader token cannot see them ([ADR-0017](adr/0017-drift-check-token.md)).

Reads: the identities and the org's live configuration through the GitHub API.

Writes: one line per finding. Notices do not count as drift. It ends with `bootstrap check: no drift`, `bootstrap check: no drift (N notice(s))`, or `bootstrap check: N finding(s)`.

The scheduled drift run uses `--params-env IDP_BOOTSTRAP --allow-hidden-bypass` with the reader token. The owner-run check, with an owner token and without `--allow-hidden-bypass`, still verifies bypass lists.

Exit codes: 0 no drift (notices allowed); 1 the check could not run; 2 usage error; 3 drift found.
