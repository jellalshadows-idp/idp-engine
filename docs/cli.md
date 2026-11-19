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
- `--modules-dir`: use local module sources from this directory instead (validation only). It must be on the same volume as `--out`. Every stack, the AWS stacks deeper in the tree included, points at it from its own directory.

A development build has no version to pin, so it needs `--module-ref` or `--modules-dir`.

Reads: the claims under `--dir` (validated first, like `validate`).

Writes: the rendered tree under `--out`, including the `.idp-rendered` marker file, and one `render: wrote PATH` line per file. `idp diff` requires that marker. The tree holds the `github` stack and, since Phase 2d, an AWS stack for every environment with an `aws` block (`aws/<env>/_baseline`) and for every Component with `aws.registry: true` in each of its environments (`aws/<env>/components/<name>`), each with its lock file ([claims reference](claims.md#rendering)).

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

A stack is a directory below `--new` or `--wet` that holds `main.tf.json`, at any depth: `github`, or `aws/dev/components/api`. A file belongs to the deepest stack that holds it. The root itself is never a stack, and files outside every stack, such as the marker, are ignored. `.terraform` directories are skipped; anything but a directory or a regular file, a symbolic link included, fails the command.

For example, after the environment `staging` was removed from the platform config and one file of `api`'s `dev` stack changed:

```text
aws/dev/_baseline: unchanged
aws/dev/components/api: changed
aws/staging/_baseline: orphan
github: unchanged
diff: 2 affected stack(s)
```

Reads: both trees.

Writes: one `PATH: STATUS` line per stack and a `diff: N affected stack(s)` line. Step output `affected`: a JSON array of the stack paths that are `new`, `changed` or `orphan` (every stack of the fresh render with `--all`, plus the orphans).

Exit codes: 0 done; 1 the trees could not be read or `--new` is not a render; 2 usage error.

## stacks

Splits the affected stacks into the `github` stack and the AWS matrix.

```text
idp stacks --affected JSON --wet DIR [--dir DIR]
```

- `--affected`: required. The `affected` JSON array of `idp diff`.
- `--wet`: required. The `rendered/` directory of the wet branch checkout; it may not exist yet.
- `--dir`: the claims repo (default `.`). It is read only when an AWS stack is affected.

This engine version reconciles `github`, `aws/<env>/_baseline` and `aws/<env>/components/<name>`, the names the claims schemas allow; any other affected stack, such as a Workspace (`aws/<env>/workspaces/<name>`, Phase 3), fails the command and lists what it does not reconcile. More than 256 AWS stacks, the jobs one matrix runs, fail it too (spec §11 risk 3).

Each AWS stack runs as its environment's account (ADR-0004): from the claims' platform config, or, for an environment the claims removed or kept without its `aws` block, from the OIDC provider ARN in the wet renders of its Component stacks, which must all name one account. An orphan baseline whose environment had no Component stack runs as `000000000000`: its configuration names no account.

For example, after `staging` (account `000000000002`) was removed and `api` changed in `dev`:

```text
aws/dev/components/api: account 000000000001
aws/staging/_baseline: account 000000000002
aws/staging/components/api: account 000000000002
stacks: github true, 3 AWS stack(s)
```

Writes: one `STACK: account ID` line per AWS stack and a summary line. Step outputs `github` (`true` when the `github` stack is affected) and `aws`, the matrix: a JSON array of `{"stack", "artifact", "account"}`, sorted by stack, where `artifact` is the stack with each `/` as `.`, for artifact names (`aws/dev/_baseline` is `aws.dev._baseline`). `[]` when no AWS stack is affected.

Exit codes: 0 done; 1 a stack this version does not reconcile, too many AWS stacks, or a wet Component stack whose OIDC provider ARN names no account, or another account than its environment's other Component stacks; 2 usage error (including an empty or duplicate stack name); 3 invalid claims.

## aws-stack

Plans one AWS stack against floci, or re-plans and applies it, as the pipelines' `plan-aws` and `apply-aws` jobs do (spec §6.1 step 4, §6.2 step 3).

```text
idp aws-stack plan  --stack STACK --account ID --new DIR --wet DIR --work DIR --endpoint URL --out-dir DIR [--tofu PATH]
idp aws-stack apply --stack STACK --account ID --new DIR --wet DIR --work DIR --endpoint URL --fingerprint FILE --applied-dir DIR [--tofu PATH]
```

- `--stack`: an AWS stack, `aws/<env>/_baseline` or `aws/<env>/components/<name>`.
- `--account`: the 12-digit account the stack runs as, from `idp stacks`.
- `--new`: the fresh render's root. A stack missing from it is an orphan.
- `--wet`: the wet branch's `rendered/` directory. It may not exist yet.
- `--work`: a scratch directory outside both renders, with no `rendered/` in it yet. OpenTofu runs only there; the renders are never written.
- `--endpoint`: floci's endpoint, written into each stack's `floci_override.tf.json` (spec §5.6).
- `--out-dir` (plan): where to write the stack's reduced plan, as `idp plan-reduce` does.
- `--fingerprint` (apply): the fingerprint the gate decided on.
- `--applied-dir` (apply): where to mark the stack applied, after a successful apply.
- `--tofu`: the OpenTofu binary (default `tofu`).

In `--work/rendered`, laid out like a render so relative module sources resolve, it:

1. for a Component stack, applies its environment's `_baseline`: the wet version, or the new one when wet has none;
2. applies the wet version of the stack, when wet has one;
3. plans the new version in the same directory, against the state step 2 wrote (ADR-0030), or, for an orphan, plans a destroy with the wet configuration.

The replays of steps 1 and 2 apply with `-auto-approve`: they only rebuild floci's state. `apply` then requires the plan's changes to equal the gated fingerprint's for the stack, both ways, and applies the saved plan; a stack with no entry in the fingerprint, or any difference, applies nothing:

```text
idp: aws/dev/components/api: the plan differs from the gated one, so nothing was applied: dispatch the reconcile workflow again (spec §6.2)
  not gated: aws/dev/components/api: update module.component.aws_iam_role.ci
```

OpenTofu runs with `AWS_ACCESS_KEY_ID` set to the account (ADR-0004), a fixed secret access key that floci does not check, `TF_IN_AUTOMATION=1` and `TF_INPUT=0`; its output goes to stderr. The command refuses to run when `TF_ENCRYPTION` or `IDP_STATE_PASSPHRASE` is set: a job that runs floci shares the runner's Docker socket, so it holds no secret (ADR-0031).

Writes: `plan` writes the reduced plan, prints `STACK: +create ~update -delete ±replace`, and sets the step output `changes`. `apply` prints the same line with `: applied`, writes the file `STACK` (each `/` as `.`) into `--applied-dir`, holding the stack and a newline, and appends the evidence to the job summary: the changes it applied, or why nothing was applied.

Exit codes: 0 done; 1 OpenTofu failed, the plan differs from the gated one, a secret is set, or a stack, render or work directory is not usable; 2 usage error.

## plan-reduce

Keeps of a stack's plan only the addresses and actions, for an artifact.

```text
idp plan-reduce --stack STACK --plan FILE --out-dir DIR
```

- `--stack`: a stack this engine version reconciles (see `idp stacks`).
- `--plan`: the `tofu show -json` output of the stack.
- `--out-dir`: where to write the reduced plan, as `STACK.json` with each `/` as `.` (`aws.dev._baseline.json`).

`tofu show -json` holds every value the plan sets, in plaintext. The reduced plan keeps the format version and each resource change's address and actions, which is all the fingerprint reads, so it is what crosses jobs as an artifact ([ADR-0031](adr/0031-floci-jobs-hold-no-secret.md)). `idp plan-summary` reads it back to the same plan.

Writes: the reduced plan, one line of JSON; a `STACK: +create ~update -delete ±replace` line; step output `changes`.

Exit codes: 0 done; 1 the plan could not be read or parsed (the same checks as `plan-summary`), or the file could not be written; 2 usage error.

## plan-summary

Summarizes OpenTofu plans as a pull request comment and a fingerprint.

```text
idp plan-summary --stacks JSON --head-sha SHA --comment-out FILE --fingerprint-out FILE [--run-url URL] [--plan STACK=FILE]...
idp plan-summary --stacks JSON --head-sha SHA --comment-out FILE --fingerprint-out FILE [--run-url URL] --plan-dir DIR
```

- `--stacks`: required. The `affected` JSON array written by `idp diff` (for example `["github"]`, or `[]` when nothing is affected). The set of `--plan` stacks must equal this set exactly. A missing plan must fail here, never read as "no changes": the gate and the reconcile would otherwise decide on a partial view and apply a plan nobody saw.
- `--head-sha`: the commit the plans are for, 40 hex characters.
- `--comment-out`: where to write the comment Markdown.
- `--fingerprint-out`: where to write the fingerprint JSON.
- `--run-url`: link to the workflow run, shown in the comment.
- `--plan STACK=FILE`: the `tofu show -json` output of one stack. Repeatable; a stack may be given only once.
- `--plan-dir DIR`: instead of `--plan`, read each affected stack's plan from `DIR/idp-plan-S/S.json`, where `S` is the stack with each `/` as `.`: the layout `actions/download-artifact` gives the `idp-plan-S` artifacts when it does not merge them, so a file that another artifact holds is never read (ADR-0031). Only `github`, `aws/<env>/_baseline` and `aws/<env>/components/<name>` can be read this way. A missing file fails the command.

Reads: each plan file. A plan with an action the fingerprint does not know is a failure, never treated as harmless ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).

Writes:

- The comment file. It carries the marker `<!-- idp-plan -->` and a hidden fingerprint marker, `<!-- idp-fingerprint:v1 sha=<head SHA> data=<base64(gzip(JSON))> -->` ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).
- The fingerprint JSON file, which `idp gate` reads.
- One `STACK: +create ~update -delete ±replace` line per stack, and a `plan-summary: N change(s), destructive: BOOL` line.
- Step outputs `changes` (the number of changes) and `destructive` (`true` or `false`: any delete or replace).

Exit codes: 0 done; 1 a plan could not be read or parsed, or a file could not be written; 2 usage error (a missing flag, a `--head-sha` that is not a full SHA, a stack given twice, `--stacks` that is not a JSON array of unique names, an affected stack with no `--plan`, a `--plan` for a stack that is not in `--stacks`, `--plan` and `--plan-dir` together, a stack `--plan-dir` cannot read).

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

A fingerprint is trusted only when it is in the newest comment written by `github-actions[bot]` (type `Bot`) that contains `<!-- idp-plan -->`, on the pull request whose merge commit is `--sha`, and only when the marker's SHA equals that pull request's head SHA. Comments by anyone else are ignored, so a stranger cannot even force an approval prompt. A comment that anyone but the Actions bot last edited is not trusted either; the gate reads its last editor through the GraphQL API ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).

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

## wet-root

Prepares the wet commit of a reconcile (spec §6.2 step 4).

```text
idp wet-root --out DIR --wet-base SHA --new DIR [--applied-dir DIR] [--github-state FILE [--github-render]]
```

- `--out`: the wet root to write. It must not exist yet.
- `--wet-base`: the wet commit the reconcile planned against, written to `wet-base` for the `recover` workflow.
- `--new`: the reconcile's render.
- `--applied-dir`: the markers of `idp aws-stack apply`, one per AWS stack that applied. It may be missing.
- `--github-state`: the `github` stack's state. Committed when the file exists, even when the apply failed.
- `--github-render`: commit the `github` stack's render: its apply succeeded or had nothing to apply. It needs `--github-state`.

Writes, under `--out`: `wet-base`, `tfstate/github.tfstate`, `rendered/github` when asked, `rendered/STACK` for each AWS stack that applied, and `wet-paths`, the paths for `idp wet-push`, one per line. An applied orphan's path is listed with no render, so `wet-push` deletes it; a stack that did not apply keeps its old render in wet. A state file inside a render is left out and named on stderr (`idp wet-root: left out PATH: ...`): `wet-push` refuses one, and its refusal would drop the `github` state's commit too (ADR-0030). Prints one `PATH: why` line per path and `wet-root: N path(s) to commit`; step output `paths`, the count.

Exit codes: 0 done; 1 a marker is not one `idp aws-stack apply` writes, `--out` exists, or a file could not be read or written; 2 usage error.

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
- No state file is committed but `tfstate/github.tfstate`: a file named `*.tfstate`, or a copy such as `terraform.tfstate.backup`, anywhere else under a path fails the command before any write. An AWS stack's state stays in its job ([ADR-0030](adr/0030-aws-stack-state-stays-in-the-job.md)).
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
idp bootstrap app   --org ORG --role reader|writer|bot [--out-dir DIR] [--listen ADDR]
idp bootstrap apply --org ORG --claims-repo REPO --approver LOGIN [--approver LOGIN]... --reader FILE --writer FILE --passphrase-file FILE
idp bootstrap check --org ORG --claims-repo REPO (--approver LOGIN [--approver LOGIN]... --reader FILE --writer FILE | --params-env NAME) [--allow-hidden-bypass]
```

`FILE` for `--reader` and `--writer` is the `<slug>.json` written by `bootstrap app`.

### bootstrap app

Creates a GitHub App (reader, writer or bot) for the org through the App manifest flow.

- `--org`: the organization that will own the App.
- `--role`: `reader`, `writer` or `bot`. `bot` creates the automation App of [ADR-0026](adr/0026-automation-app.md); install it on only the engine and claims repos.
- `--out-dir`: where to write `<slug>.json` and `<slug>.pem` (default `~/.idp/apps`).
- `--listen`: the local address of the callback server (default `127.0.0.1:0`). It must be a loopback address, because the server receives the App credentials.

Writes: the two credential files, and a link to install the App on the org (all repositories for the reader and the writer; only the engine and claims repos for the bot). It does not call GitHub with a token.

Exit codes: 0 created; 1 the flow failed; 2 usage error.

### bootstrap apply

Makes the org match the desired state. It is safe to run any number of times: a second run against an unchanged org performs no writes.

- `--org`, `--claims-repo`: the organization and the claims repository name.
- `--approver`: a person who approves runs; repeat it for several (each becomes an `idp-approval` reviewer).
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

## version

Prints this build's version.

```text
idp version
```

It takes no flags.

Writes: one line with the version: the release tag of a released binary (for example `v0.1.0`), the engine commit of the source build the pipelines make for a commit that is not a release, or `dev` for a local build. Rendered module sources pin the same value. The workflows compare it with the release tag after they install a release ([ADR-0028](adr/0028-releases-and-how-the-workflows-get-idp.md)).

Exit codes: 0 printed; 2 usage error (an argument).
