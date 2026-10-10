# Phase 1b — Pipeline Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship every `idp` subcommand the pipelines of spec §6 need: diff against `wet`, plan summary and fingerprint, the reconcile gate, the sticky PR comment, labelled issues, the one-commit push to `wet`, the `TF_ENCRYPTION` export, and drift-ready `bootstrap check`. All of them are unit-tested against files and the fake GitHub.

**Architecture:**
- **Pure core.** `internal/wetdiff` (stack diff) and `internal/plan` (parse, fingerprint, comment, gate decision) are pure, file-in/value-out packages.
- **GitHub side effects.** These live in two thin packages over `internal/ghapi`, which gains pagination:
  - `internal/ghops` covers the pull request behind a commit, the bot's sticky comment and labelled issues.
  - `internal/wetpush` makes one commit through the Git Data API.
- **Workflow plumbing.** `internal/actions` writes GitHub Actions outputs, env, summary and annotations.
- **Exit codes.** Every command follows one exit-code convention (ADR-0016).

**Tech Stack:** Go 1.26 (standard library only; no new dependencies), OpenTofu 1.12.6 (only to generate plan fixtures locally), GitHub REST API `2022-11-28`.

**Spec:** `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`, especially §3.3, §6.1–§6.4, §7.2, §8.2 and §9.2. Context: `docs/phases/phase-0.md` (findings I3, M1 and S1) and `docs/phases/phase-1a.md` (rulings DEFER and R-3.3).

## Phase 1 re-split (decided while planning)

The "1b pipelines" plan of the Phase 1a decomposition is too large for one reviewable plan. It needs seven new commands, the bootstrap changes for drift, three reusable workflows and a live smoke run. The remaining Phase 1 work is therefore split again, and each plan still ships working software:

| Plan | Delivers |
|---|---|
| 1a (done) | Claims → validated model → rendered GitHub stack, plus the modules |
| **1b (this plan)** | Every pipeline subcommand: `diff`, `plan-summary`, `gate`, `comment`, `issue`, `wet-push`, `encryption-env`; drift-ready `bootstrap check`; exit-code convention |
| 1c | Reusable workflows `pr.yaml`, `reconcile.yaml`, `drift.yaml`, the caller templates, and a live smoke run in `idp-claims-e2e` |
| 1d | E2E harness, release-please + GoReleaser + attestations, Renovate, `v0.1.0` |

The workflows plan is written after this one ships. Its jobs then call commands that already exist, so their interfaces and real behaviour are known, not guessed.

## Global Constraints

**Toolchain and dependencies**
- Go module `github.com/jellalshadows-idp/idp-engine`, `go 1.26.0`. **No new dependencies:** standard library only.
- OpenTofu 1.12.6, installed and checksum-verified at `$HOME/.idp/tools/tofu-1.12.6/tofu.exe`, is used only to generate the plan fixtures of Task 4.

**Exit codes (ADR-0016)**
- Every command exits `0` when it did its job, `1` when it failed, `2` on a usage error, and `3` when it did its job and the answer needs attention (invalid claims, drift).
- Pipeline commands report results through step outputs and exit `0`.

**Environment variables**
- Tokens: `GH_TOKEN`, else `GITHUB_TOKEN`.
- API base override for tests: `IDP_GITHUB_API`.
- Actions files: `GITHUB_OUTPUT`, `GITHUB_ENV`, `GITHUB_STEP_SUMMARY`.
- Annotations: `GITHUB_ACTIONS=true` turns them on; `GITHUB_WORKSPACE` is the root annotation paths are made relative to.
- State passphrase: `IDP_STATE_PASSPHRASE`.

**Markers**
- Plan comment marker: `<!-- idp-plan -->`.
- Fingerprint marker: `<!-- idp-fingerprint:v1 sha=<40-hex head SHA> data=<base64(gzip(fingerprint JSON))> -->`.

**GitHub conventions**
- Trusted comment author: login `github-actions[bot]` with type `Bot`.
- Drift issue: label `drift`, title `Drift detected`.
- Bootstrap identity variable: `IDP_BOOTSTRAP`.
- `TF_ENCRYPTION` key provider and method name: `idp`. It is frozen (ADR-0020).

**Process**
- Branch `feat/phase-1b-pipeline-commands`; one PR at the end.
- Conventional commits, never with Co-Authored-By or any AI attribution.
- No `go build`. Use `go test ./...`, `go vet ./...` and `gofmt -l .` (which must print nothing); `-race` runs only in CI.
- Shell rule: never use `cat`, `grep`, `find`, `sed` or `ls`, and never write files with shell heredocs. Use the Read/Write/Edit tools, `rg` and `gh`.
- Docs are in English.
- Owner rule: docs describe the platform on its own terms. They never name another product as inspiration or comparison. The controller gives each dispatch the check that enforces this.

## Review Focus

1. **A forged or stale fingerprint.**
   - **Input:** a PR comment that carries an `idp-fingerprint` marker but was written by someone other than `github-actions[bot]`, or a bot comment made for an earlier head commit.
   - **Expected:** the gate must never trust it; the answer is `approval`.
   - **Pinned by:** `TestGateIgnoresUntrustedComments` in Task 7.
2. **A list that spans several pages.**
   - **Input:** a busy PR with more than one page of comments, or a repo with many open issues.
   - **Expected:** the sticky comment and the drift issue are still found, never duplicated.
   - **Pinned by:** `TestFindBotCommentAcrossPages` (Task 6) and `TestOpenIssueUpdatesExistingAcrossPages` (Task 8), both with `MaxPerPage = 1`.
3. **The first reconcile.**
   - **Input:** `wet` is the fresh orphan branch, with no `rendered/` and no `tfstate/`.
   - **Expected:** diff reports every stack as `new` instead of failing, and wet-push creates files under prefixes that do not exist yet.
   - **Pinned by:** `TestDiffWetMissing` (Task 3) and `TestSyncCreatesFilesUnderNewPrefix` (Task 9).
4. **A plan action the engine has never seen.**
   - **Input:** a future OpenTofu action.
   - **Expected:** parsing fails closed; the action is never read as a no-op.
   - **Pinned by:** `TestParseRejectsUnknownActions` in Task 4.
5. **A passphrase containing `"`, `\` or `${`.**
   - **Expected:** `TF_ENCRYPTION` carries it literally. A mis-escaped passphrase is a *different key*: state written with it cannot be read with the real secret.
   - **Pinned by:** `TestEncryptionConfigEscapesHCL` in Task 10.

## File Structure

```
internal/actions/actions.go, actions_test.go        # outputs, env, summary, annotations (Task 2)
internal/cli/exit.go                                # exit codes (Task 2)
internal/cli/diff.go, diff_test.go                  # idp diff (Task 3)
internal/cli/plansummary.go, plansummary_test.go    # idp plan-summary (Task 5)
internal/cli/gate.go, gate_test.go                  # idp gate (Task 7)
internal/cli/conversations.go, conversations_test.go # idp comment, idp issue (Task 8)
internal/cli/wetpush.go, wetpush_test.go            # idp wet-push (Task 9)
internal/cli/encryption.go, encryption_test.go      # idp encryption-env (Task 10)
internal/wetdiff/wetdiff.go, wetdiff_test.go        # Task 3
internal/plan/plan.go, plan_test.go                 # parse, counts, fingerprint (Task 4)
internal/plan/testdata/{gen/,creates.json,mixed.json}
internal/plan/comment.go, marker.go, comment_test.go, testdata/comment-mixed.golden.md   # Task 5
internal/plan/gate.go, gate_test.go                 # Task 7
internal/ghapi/list.go, list_test.go                # Task 6
internal/ghops/ghops.go, read_test.go               # Task 6
internal/ghops/write.go, write_test.go              # Task 8
internal/wetpush/wetpush.go, wetpush_test.go        # Task 9
internal/render/encryption.go, encryption_test.go   # Task 10
internal/fakegithub/fakegithub.go                   # (modify) Tasks 6, 8, 9
internal/bootstrap/{config.go,check.go,bootstrapper.go,desired.go} # (modify) Tasks 10, 11
docs/adr/0016…0020-*.md, docs/cli.md, docs/phases/phase-1b.md
```

---

### Task 1: Decisions first — ADRs 0016–0020 and spec amendments

**Files:**
- Create: `docs/adr/0016-exit-codes.md`, `docs/adr/0017-drift-check-token.md`, `docs/adr/0018-plan-fingerprint-and-gate-trust.md`, `docs/adr/0019-wet-commits-through-the-git-data-api.md`, `docs/adr/0020-tf-encryption-from-idp.md`
- Modify: `docs/adr/README.md`, `docs/superpowers/specs/2026-10-08-idp-on-actions-design.md`

**Interfaces:**
- Produces: the decisions every later task implements: the exit codes, the trusted-comment rules, the `IDP_BOOTSTRAP` format, the wet-push semantics and the frozen `idp` name.

- [ ] **Step 1: Create the branch**

Run: `git switch main && git pull --ff-only && git switch -c feat/phase-1b-pipeline-commands`

- [ ] **Step 2: Write `docs/adr/0016-exit-codes.md`**

Match the structure and heading style of `docs/adr/0015-claim-parsing-and-validation.md` (Status, Date, Spec link, Context, Decision, Consequences with `###` sub-headings, Alternatives considered). Content:

```markdown
# 0016. One exit-code convention for every command

- Status: Accepted
- Date: 2026-10-10
- Spec: [§3.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#33-units), [§6.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#64-drift-daily-cron)

## Context

Phase 0 finding M1: `idp bootstrap check` exits 1 both when it finds drift and when it cannot check at all, for example a network error or a bad token. The drift workflow must tell them apart. Drift opens or updates an issue; a failed check must fail the run loudly. `idp validate` has the same ambiguity: a broken claim and an unreadable directory both exit 1.

OpenTofu's `plan -detailed-exitcode` already uses 2 for "changes present", and Go's `flag` package uses 2 for usage errors. Both conventions appear in the same workflow logs.

## Decision

Every `idp` command uses four exit codes:

| Code | Meaning | Examples |
|---|---|---|
| 0 | The command did its job, and nothing needs attention | valid claims, no drift, a gate decision was made |
| 1 | The command could not do its job | unreadable file, API error, missing secret |
| 2 | The command line is wrong | unknown flag, missing required flag |
| 3 | The command did its job, and the answer needs attention | invalid claims (`validate`, `render`), drift (`bootstrap check`) |

The pipeline commands (`diff`, `plan-summary`, `gate`, `comment`, `issue`, `wet-push`, `encryption-env`) report their results through step outputs, such as `affected`, `decision` and `commit`, and exit 0 when they produced them. Whether a run continues is decided by the workflow from those outputs, not from exit codes.

## Consequences

### Positive

- The drift workflow can branch on 3 (open the issue) versus 1 (fail the job).
- Scripts that treat any non-zero code as failure keep working.

### Negative / costs

- `validate` and `render` with invalid claims, and `bootstrap check` with drift, move from exit 1 to exit 3, which is a visible behaviour change. The runbook and the claims reference document it.

## Alternatives considered

- **Keep 1 for both.** Rejected: that is finding M1.
- **Use OpenTofu's 2 for "found something".** Rejected: 2 already means a usage error in every Go CLI, so the two meanings would collide.
- **A distinct code per kind of finding.** Rejected as YAGNI: the step output carries the details.
```

- [ ] **Step 3: Write `docs/adr/0017-drift-check-token.md`**

```markdown
# 0017. The drift check runs with the reader token

- Status: Accepted
- Date: 2026-10-10
- Spec: [§6.4](../superpowers/specs/2026-10-08-idp-on-actions-design.md#64-drift-daily-cron), [§9.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#92-bootstrap-outside-the-idp-on-purpose)

## Context

The daily drift workflow (spec §6.4) runs `idp bootstrap check` unattended. Two facts from Phase 0 shape how:

1. **The reader token cannot see ruleset bypass actors** (finding I3, ADR-0013). GitHub returns `bypass_actors` only to callers with write access to the ruleset. Since PR #3, `check` fails closed: a hidden list is reported as drift. With the reader token, every daily run would therefore report one finding per ruleset that nobody can act on.
2. **`check` needs identities that today come from local files.** These are the App ids, slugs and client ids, plus the approver. They live in `~/.idp/apps/*.json` on the owner's machine, which a workflow does not have.

## Decision

- **Token.** Drift runs `check` with the **reader** token and `--allow-hidden-bypass`. A ruleset whose `bypass_actors` the token cannot see is reported as a *notice* ("not verifiable with this token"), not as drift. Everything else is verified as before.
- **Identities.** `idp bootstrap apply` records them in a repository variable, `IDP_BOOTSTRAP`, as compact JSON: `{"approverId":42,"reader":{"id":1,"clientId":"Iv-…","slug":"…-reader"},"writer":{"id":2,"clientId":"Iv-…","slug":"…-writer"}}`. Nothing in it is secret. `check` verifies the variable like any other.
- **Reading the identities.** `idp bootstrap check --params-env IDP_BOOTSTRAP` reads the identities from that environment variable instead of from `--approver/--reader/--writer`.
- **Owner check.** Bypass lists stay verified by the owner-run `check`, which uses an owner token (runbook), and by every `apply`.

## Consequences

### Positive

- No write-capable key is ever present in a scheduled job.
- The daily drift run has no permanent false positive.

### Negative / costs

- **Accepted risk:** the daily run cannot see a bypass actor that an org admin adds to `idp-main`. Such a change needs admin rights, and an admin could also disable the drift workflow itself, so it is outside what drift can defend against. The owner-run `check` is the control.
- `apply` writes one more variable; existing claims repos pick it up on their next `apply`.

## Alternatives considered

- **The writer token, scoped down to `administration: write`.** Rejected. It puts the ability to *change* rulesets into a daily cron job in order to read one field.
- **A third App with `administration: write`.** Rejected: it has the same capability, plus another private key to protect.
- **The org audit log.** Rejected: its API needs GitHub Enterprise.
- **Keep failing closed.** Rejected: a permanent daily false positive trains people to ignore the drift issue.
```

- [ ] **Step 4: Write `docs/adr/0018-plan-fingerprint-and-gate-trust.md`**

```markdown
# 0018. Plan fingerprints and what the gate trusts

- Status: Accepted
- Date: 2026-10-10
- Spec: [§6.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#61-pr-pull_request--main), [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main)

## Context

The guiding rule of spec §6 is "nothing is ever applied that nobody saw". Reconcile applies without approval only when its plan is a subset of the plan that was shown on the pull request. That plan lives in a PR comment, and PR comments are public. Anyone with a GitHub account can add comments to a public repository's pull requests, so the gate must decide exactly which comment to believe.

## Decision

- **Fingerprint.** A fingerprint is, per stack, the sorted set of `(address, action)` pairs.
  - Actions are normalised to `create`, `update`, `delete`, `replace` (`delete,create` in either order) and `forget`.
  - `no-op` and `read` are excluded.
  - Any other action makes parsing **fail**. An unknown action is never treated as harmless.
- **Comment marker.** The plan comment carries `<!-- idp-plan -->` and one hidden marker: `<!-- idp-fingerprint:v1 sha=<head SHA> data=<base64(gzip(JSON))> -->`. Gzip keeps a plan of hundreds of resources within GitHub's comment size limit.
- **Which comment the gate trusts.** The gate (`idp gate`) trusts a fingerprint only when all of these hold:
  1. It is in the **newest** comment that contains `<!-- idp-plan -->`.
  2. That comment's author is `github-actions[bot]` and has type `Bot`, which is the identity of the plan job's `GITHUB_TOKEN`.
  3. The comment is on the merged pull request whose `merge_commit_sha` is the pushed commit.
  4. The marker's `sha` equals that pull request's **head SHA**, so the comment describes the code that was merged.
- **Decision order:**
  1. No changes → `auto`.
  2. Any `delete` or `replace` → `approval`.
  3. No trusted fingerprint → `approval`.
  4. A change missing from the same stack of the trusted fingerprint → `approval`.
  5. Otherwise → `auto`.

## Consequences

### Positive

- A comment by anyone else, or a bot comment left over from an earlier commit, can never widen what applies automatically. The worst case is an approval prompt, which fails safe.

### Negative / costs

- A pull request whose last plan run failed or was cancelled ends in an approval prompt after merge. This is intended.

## Alternatives considered

- **Keep the fingerprint in a workflow artifact.** Rejected: artifacts expire, and finding the right run of another workflow is fragile. The comment is literally what the reviewer saw.
- **Commit statuses or check runs.** Rejected: they have no room for the payload.
- **Plain JSON in the marker.** Rejected: hundreds of addresses exceed the comment limit.
```

- [ ] **Step 5: Write `docs/adr/0019-wet-commits-through-the-git-data-api.md`**

```markdown
# 0019. Wet commits through the Git Data API

- Status: Accepted
- Date: 2026-10-10
- Spec: [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main)

## Context

Reconcile ends with **one** commit to `wet` (spec §6.2 step 4.3):

- the GitHub state, always;
- the render of each stack that applied, and only those;
- a deletion of anything a stack no longer renders.

Only the writer App may update `wet` (ruleset `idp-wet`).

## Decision

`idp wet-push` makes that commit with the writer token, through the Git Data API:

1. Read the branch head and its tree. A truncated tree is an error.
2. For each synced path, compare the local files with the remote blobs. Unchanged files are skipped by their git blob hash; `.terraform/` directories are never synced.
3. Upload only changed files as blobs, and delete remote files that are no longer present locally.
4. Create one tree on top of the old one, one commit, and update the ref **without force**.
5. Nothing changed → no commit.

## Consequences

### Positive

- Commits are made by the App through the API, so GitHub signs them, shows them as **Verified**, and attributes them to the writer App.
- No git credentials are written to the runner's disk or process arguments.
- The behaviour is unit-tested against the fake GitHub.

### Negative / costs

- A concurrent update of `wet` makes the push fail. Reconcile and drift share the `idp-wet` concurrency group, so this should never happen; if it does, the failure is loud and the runbook covers it.

## Alternatives considered

- **`git push` from the runner with the token.** Rejected:
  - the token ends up in git config or command arguments;
  - the commits are unsigned;
  - the logic would be shell that is not tested.
- **The contents API.** Rejected: it makes one commit per file, which breaks "one commit".
```

- [ ] **Step 6: Write `docs/adr/0020-tf-encryption-from-idp.md`**

```markdown
# 0020. TF_ENCRYPTION is generated by idp

- Status: Accepted
- Date: 2026-10-10
- Spec: [§7.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#72-encrypted-state)

## Context

The render enforces state and plan encryption; the key material comes from `TF_ENCRYPTION` (spec §7.2, ADR-0002). Phase 0 spike S1 showed that a missing passphrase makes OpenTofu fail with `Invalid expression ... A single static variable reference is required`, which sends people to debug HCL that is correct (ADR-0013). Every pipeline job that runs `tofu` needs the same configuration.

## Decision

`idp encryption-env` runs as a workflow step:

1. It reads `IDP_STATE_PASSPHRASE` and validates it with the same rules as bootstrap: valid UTF-8, a single line, at least 16 characters. If the variable is missing or invalid, the step fails with a message that names the secret.
2. Otherwise it writes `TF_ENCRYPTION` to `GITHUB_ENV`. The value is a `pbkdf2` key provider and an `aes_gcm` method, both named `idp`, used for `state` and `plan`, with the passphrase escaped for HCL.

The name `idp` is **frozen**. OpenTofu records the key provider in the encrypted state, so renaming it makes existing state unreadable unless an OpenTofu `fallback` migration runs first.

## Consequences

### Positive

- One tested source of the encryption configuration for every workflow.
- A missing or empty secret is reported before `tofu` runs (finding S1).

### Negative / costs

- The passphrase passes through `GITHUB_ENV`. It is a registered secret, so GitHub masks it in logs. The workflows only use `pull_request`, `push`, `workflow_dispatch` and `schedule` triggers, never `pull_request_target` or `workflow_run`.

## Alternatives considered

- **Inline HCL in each workflow.** Rejected: it is duplicated, untested, and a missing secret still produces S1's cryptic error.
```

- [ ] **Step 7: Add the ADRs to the index**

After the 0015 row in `docs/adr/README.md`, add:

```markdown
| 0016 | [One exit-code convention for every command](0016-exit-codes.md) | Accepted |
| 0017 | [The drift check runs with the reader token](0017-drift-check-token.md) | Accepted |
| 0018 | [Plan fingerprints and what the gate trusts](0018-plan-fingerprint-and-gate-trust.md) | Accepted |
| 0019 | [Wet commits through the Git Data API](0019-wet-commits-through-the-git-data-api.md) | Accepted |
| 0020 | [TF_ENCRYPTION is generated by idp](0020-tf-encryption-from-idp.md) | Accepted |
```

- [ ] **Step 8: Amend the spec**

Make each edit at the quoted anchor text. Read the file first; the anchors are exact sentences.

1. **§3.3 units (amendment A4).** In the second table, the one whose rows start with `` | `internal/ghapi` ``, add these rows after the `internal/bootstrap` row:

```markdown
| `internal/ghops` | Pull request behind a commit, the bot's sticky plan comment, labelled issues | `internal/ghapi` |
| `internal/wetpush` | One commit to `wet` through the Git Data API (ADR-0019) | `internal/ghapi` |
| `internal/actions` | GitHub Actions step outputs, job env, job summary and annotations | — |
```

   Replace the paragraph that starts `CLI subcommands: ` with:

```markdown
CLI subcommands: `idp validate`, `idp fetch`, `idp adopt`, `idp render`,
`idp diff`, `idp plan-summary`, `idp gate`, `idp comment`, `idp issue`,
`idp wet-push`, `idp encryption-env`, `idp feature test`, and
`idp bootstrap app|apply|check`. They share one exit-code convention
(ADR-0016): 0 ok, 1 failed, 2 usage error, 3 found problems. Platform config
loading lives in `internal/claims` (Phase 1a ruling R-3.3) rather than a
separate `internal/config`. *(Amendment A4, 2026-10-10.)*
```

2. **§6.1 step 6 (amendment A3).** After the sentence that ends `stored as `<!-- idp-fingerprint:v1 … -->`.`, add this indented bullet at the same level:

```markdown
   - Marker format: `<!-- idp-fingerprint:v1 sha=<head SHA> data=<base64(gzip(JSON))> -->`.
     The comment also carries `<!-- idp-plan -->`, and the job edits that one comment
     instead of adding new ones (ADR-0018). *(Amendment A3, 2026-10-10.)*
```

3. **§6.2 step 1.5 (amendment A3).** After the line `In any other case, including when no PR is found, it decides **`approval`**.`, add a paragraph at the same indentation:

```markdown
      With no changes at all the gate decides **`auto`**: there is nothing to apply.
      The gate trusts only the newest `<!-- idp-plan -->` comment written by
      `github-actions[bot]` on the merged PR, and only when its marker `sha` equals
      the PR's head SHA (ADR-0018). *(Amendment A3, 2026-10-10.)*
```

4. **§6.2 step 4.3.** After the line `3. It makes **one commit to `wet`**, with `if: always()`:`, insert as the first sub-bullet:

```markdown
      - The commit is made with `idp wet-push` through the Git Data API, so it is
        signed by GitHub and attributed to the writer App (ADR-0019).
```

5. **§6.4 (amendment A5).** Replace the bullet `- It plans the GitHub stack with `idp-reader` and runs `idp bootstrap check` (§9.2).` with:

```markdown
- It plans the GitHub stack with `idp-reader` and runs
  `idp bootstrap check --params-env IDP_BOOTSTRAP --allow-hidden-bypass` (§9.2)
  with the same reader token. Ruleset bypass lists, which a read-only token
  cannot see, are reported as notices, not drift (ADR-0017). *(Amendment A5, 2026-10-10.)*
```

6. **§7.2.** After the first bullet, the one that ends `OpenTofu refuses to write plaintext.`, add:

```markdown
- Workflows get `TF_ENCRYPTION` from `idp encryption-env`, which validates
  `IDP_STATE_PASSPHRASE` first and fails with a message naming it. The key
  provider and method are named `idp`; the name is frozen (ADR-0020).
```

7. **§9.2.** Replace `- **`idp bootstrap check`** reports drift in these protections. Drift runs it` and the next line `  (§6.4).` with:

```markdown
- **`idp bootstrap check`** reports drift in these protections. Drift runs it
  (§6.4). `apply` also records the bootstrap identity (App ids, slugs, client
  ids, approver) in the variable `IDP_BOOTSTRAP`, so `check --params-env
  IDP_BOOTSTRAP` can run in a workflow without the local App files (ADR-0017).
```

8. **§10.** Replace the paragraph that starts `Phase 1 is delivered as three plans` with:

```markdown
Phase 1 is delivered as four plans, each shipping working software: **1a** claims → render (validation, the GitHub stack, the `github/group` and `github/component` modules); **1b** the pipeline subcommands (`diff`, `plan-summary`, `gate`, `comment`, `issue`, `wet-push`, `encryption-env`, drift-ready `bootstrap check`); **1c** the reusable workflows and a live smoke run in `idp-claims-e2e`; **1d** the E2E harness and the `v0.1.0` release. *(2026-10-09, re-split 2026-10-10.)*
```

- [ ] **Step 9: Check the docs and commit**

Run the owner-rule check the controller gave you over `docs` (expected: no output).

```bash
git add docs
git commit -m "docs: add adrs 0016-0020 and amend the spec for phase 1b"
```

---

### Task 2: Exit-code convention, the actions package, workspace-relative annotations

**Files:**
- Create: `internal/actions/actions.go`, `internal/actions/actions_test.go`, `internal/cli/exit.go`
- Modify: `internal/claims/diag.go`, `internal/cli/cli.go`, `internal/cli/claims.go`, `internal/cli/bootstrap.go`, `internal/cli/claims_test.go`, `internal/cli/bootstrap_test.go`, `docs/claims.md`, `docs/runbooks/bootstrap.md`

**Interfaces:**
- Produces (package `internal/actions`):
  - `ErrorAnnotation(file string, line int, title, message string) string`
  - `SetOutput(path, name, value string) error`
  - `SetEnv(path, name, value string) error`
  - `AddSummary(path, markdown string) error`

  Each is a no-op when `path` is empty.
- Produces (package `internal/cli`): the constants `exitOK = 0`, `exitError = 1`, `exitUsage = 2`, `exitFindings = 3`. The signature `loadClaims(dir string, stdout, stderr io.Writer, env Env) (*claims.Model, int)` returns one of those codes.

- [ ] **Step 1: Write the failing tests `internal/actions/actions_test.go`**

```go
package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestErrorAnnotation(t *testing.T) {
	tests := []struct {
		file    string
		line    int
		title   string
		message string
		want    string
	}{
		{"claims/a,b.yaml", 4, "", "100% wrong:\nsecond line", "::error file=claims/a%2Cb.yaml,line=4::100%25 wrong:%0Asecond line"},
		{"config/platform.yaml", 0, "", "file not found", "::error file=config/platform.yaml::file not found"},
		{"", 0, "State passphrase", "IDP_STATE_PASSPHRASE is empty", "::error title=State passphrase::IDP_STATE_PASSPHRASE is empty"},
		{"", 0, "", "bare", "::error::bare"},
	}
	for _, tt := range tests {
		if got := ErrorAnnotation(tt.file, tt.line, tt.title, tt.message); got != tt.want {
			t.Errorf("ErrorAnnotation(%q, %d, %q, %q) = %q, want %q", tt.file, tt.line, tt.title, tt.message, got, tt.want)
		}
	}
}

func TestSetOutputSingleLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out")
	if err := SetOutput(path, "decision", "auto"); err != nil {
		t.Fatal(err)
	}
	if err := SetOutput(path, "affected", `["github"]`); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "decision=auto\naffected=[\"github\"]\n" {
		t.Errorf("file = %q", got)
	}
}

func TestSetEnvMultiLineUsesARandomDelimiter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env")
	value := "line one\nline two"
	if err := SetEnv(path, "TF_ENCRYPTION", value); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	head, rest, ok := strings.Cut(got, "\n")
	if !ok || !strings.HasPrefix(head, "TF_ENCRYPTION<<IDP_EOF_") {
		t.Fatalf("file = %q, want a heredoc entry", got)
	}
	delim := strings.TrimPrefix(head, "TF_ENCRYPTION<<")
	if rest != value+"\n"+delim+"\n" {
		t.Errorf("file = %q, want the value followed by the delimiter %q", got, delim)
	}
}

func TestEmptyPathIsANoop(t *testing.T) {
	if err := SetOutput("", "a", "b"); err != nil {
		t.Error(err)
	}
	if err := AddSummary("", "x"); err != nil {
		t.Error(err)
	}
}

func TestInvalidNameIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out")
	for _, name := range []string{"", "a=b", "a\nb"} {
		if err := SetOutput(path, name, "v"); err == nil {
			t.Errorf("SetOutput(%q) succeeded, want an error", name)
		}
	}
}

func TestAddSummaryAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary")
	if err := AddSummary(path, "**Gate:** auto"); err != nil {
		t.Fatal(err)
	}
	if err := AddSummary(path, "done"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "**Gate:** auto\ndone\n" {
		t.Errorf("file = %q", got)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/actions/`
Expected: FAIL to compile, with `undefined: ErrorAnnotation`.

- [ ] **Step 3: Implement `internal/actions/actions.go`**

```go
// Package actions writes GitHub Actions workflow commands and environment
// files: step outputs, job environment, job summary and error annotations.
// Every writer is a no-op when its file path is empty (not running in Actions).
package actions

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

var (
	dataEscaper     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

// ErrorAnnotation formats an ::error workflow command, escaped as the
// workflow-commands docs require. file, line and title are optional.
func ErrorAnnotation(file string, line int, title, message string) string {
	var props []string
	if file != "" {
		props = append(props, "file="+propertyEscaper.Replace(file))
	}
	if line > 0 {
		props = append(props, fmt.Sprintf("line=%d", line))
	}
	if title != "" {
		props = append(props, "title="+propertyEscaper.Replace(title))
	}
	cmd := "::error"
	if len(props) > 0 {
		cmd += " " + strings.Join(props, ",")
	}
	return cmd + "::" + dataEscaper.Replace(message)
}

// SetOutput appends a step output to the GITHUB_OUTPUT file at path.
func SetOutput(path, name, value string) error { return appendEntry(path, name, value) }

// SetEnv appends a variable for later steps to the GITHUB_ENV file at path.
func SetEnv(path, name, value string) error { return appendEntry(path, name, value) }

// AddSummary appends Markdown to the GITHUB_STEP_SUMMARY file at path.
func AddSummary(path, markdown string) error {
	if path == "" {
		return nil
	}
	return appendFile(path, markdown+"\n")
}

func appendEntry(path, name, value string) error {
	if path == "" {
		return nil
	}
	if name == "" || strings.ContainsAny(name, "=\r\n") {
		return fmt.Errorf("invalid name %q", name)
	}
	if !strings.ContainsAny(value, "\r\n") {
		return appendFile(path, name+"="+value+"\n")
	}
	delim, err := delimiter(value)
	if err != nil {
		return err
	}
	return appendFile(path, name+"<<"+delim+"\n"+value+"\n"+delim+"\n")
}

// delimiter returns a random heredoc delimiter that does not occur in value,
// so a value can never end the entry early.
func delimiter(value string) (string, error) {
	for {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		if d := "IDP_EOF_" + hex.EncodeToString(b); !strings.Contains(value, d) {
			return d, nil
		}
	}
}

func appendFile(path, s string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(s); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
```

- [ ] **Step 4: Route claims annotations through the package**

In `internal/claims/diag.go`, replace the body of `Annotation` with a call to the new package, and delete `dataEscaper`, `propertyEscaper`, `escapeData` and `escapeProperty`, which are now unused:

```go
// Annotation formats d as a GitHub Actions error annotation, so the problem
// shows inline on the PR.
func (d Diagnostic) Annotation() string {
	return actions.ErrorAnnotation(d.File, d.Line, "", d.Message)
}
```

Import `github.com/jellalshadows-idp/idp-engine/internal/actions` and drop imports that become unused. `TestDiagnosticAnnotationEscapes` must still pass unchanged.

- [ ] **Step 5: Write the failing CLI tests**

In `internal/cli/claims_test.go`:

1. Change `TestValidateReportsProblemsAndAnnotations` to expect exit `3`: `if code != 3 { t.Fatalf("exit %d, want 3", code) }`.
2. Change `TestRenderStopsOnDiagnostics` to expect exit `3`.
3. Append:

```go
func TestValidateAnnotationsAreRelativeToTheWorkspace(t *testing.T) {
	workspace := t.TempDir()
	dir := filepath.Join(workspace, "claims-repo")
	bad := strings.Replace(cliComponent, "[dev]", "[staging]", 1)
	for rel, content := range map[string]string{
		"config/platform.yaml":        cliPlatform,
		"claims/groups/platform.yaml": cliGroup,
		"claims/components/api.yaml":  bad,
	} {
		writeFileAll(t, filepath.Join(dir, filepath.FromSlash(rel)), content)
	}
	var stdout, stderr bytes.Buffer
	env := envOf(map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_WORKSPACE": workspace})
	if code := Run([]string{"validate", "--dir", dir}, &stdout, &stderr, env); code != 3 {
		t.Fatalf("exit %d, want 3", code)
	}
	if !strings.Contains(stdout.String(), "::error file=claims-repo/claims/components/api.yaml,line=5::") {
		t.Errorf("stdout = %q, want an annotation relative to GITHUB_WORKSPACE", stdout.String())
	}
	if !strings.Contains(stderr.String(), "claims/components/api.yaml:5:") {
		t.Errorf("stderr = %q, want the path relative to --dir", stderr.String())
	}
}

func TestAnnotationFile(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name, workspace, dir, want string
	}{
		{"no workspace", "", filepath.Join(root, "x"), "claims/a.yaml"},
		{"dir is the workspace", root, root, "claims/a.yaml"},
		{"dir below the workspace", root, filepath.Join(root, "sub", "repo"), "sub/repo/claims/a.yaml"},
		{"dir outside the workspace", filepath.Join(root, "ws"), filepath.Join(root, "other"), "claims/a.yaml"},
	}
	for _, tt := range tests {
		if got := annotationFile(tt.workspace, tt.dir, "claims/a.yaml"); got != tt.want {
			t.Errorf("%s: annotationFile = %q, want %q", tt.name, got, tt.want)
		}
	}
}
```

In `internal/cli/bootstrap_test.go`, rename `TestCheckExitsOneOnFindings` to `TestCheckExitsThreeOnFindings` and expect `3`: `if code != 3 { t.Fatalf("exit code = %d, want 3 (stderr %q)", code, stderr) }`.

- [ ] **Step 6: Run them and see them fail**

Run: `go test ./internal/cli/`
Expected: FAIL. `annotationFile` is undefined, and validate, render and check still exit 1.

- [ ] **Step 7: Implement the convention**

Create `internal/cli/exit.go`:

```go
package cli

// Exit codes shared by every command (ADR-0016).
const (
	exitOK       = 0 // done; nothing needs attention
	exitError    = 1 // the command could not do its job
	exitUsage    = 2 // the command line is wrong
	exitFindings = 3 // done, and the answer needs attention (invalid claims, drift)
)
```

In `internal/cli/cli.go`:
- Replace the `usage` const with the text below. The command column is 16 characters wide, so later tasks can insert lines without re-aligning.
- Update `Run`'s doc comment to `// Run executes the CLI and returns the process exit code (ADR-0016).`
- Replace the literal `0` and `2` returns with `exitOK` and `exitUsage`.

```go
const usage = `idp — IDP engine CLI

Usage:
  idp <command> [flags]

Commands:
  validate        Validate a claims repo (schema + semantic checks)
  render          Render a claims repo into OpenTofu stacks
  bootstrap       Create or verify the protections an org needs before the IDP runs
  help            Show this help

Exit codes: 0 ok, 1 the command failed, 2 usage error,
3 the command found problems (invalid claims, drift).
`
```

In `internal/cli/claims.go`:

1. Change `loadClaims` to return a code:

```go
// loadClaims loads and validates a claims repo and prints every diagnostic: always
// to stderr, and also to stdout as a GitHub annotation inside GitHub Actions. The
// int is exitOK, exitError (the tool failed) or exitFindings (invalid claims).
func loadClaims(dir string, stdout, stderr io.Writer, env Env) (*claims.Model, int) {
	m, diags, err := claims.Load(dir)
	if err != nil {
		fmt.Fprintln(stderr, "idp:", err)
		return nil, exitError
	}
	inActions := env("GITHUB_ACTIONS") == "true"
	for _, d := range diags {
		if inActions {
			a := d
			a.File = annotationFile(env("GITHUB_WORKSPACE"), dir, d.File)
			fmt.Fprintln(stdout, a.Annotation())
		}
		fmt.Fprintln(stderr, d)
	}
	if len(diags) > 0 {
		fmt.Fprintf(stderr, "validate: %d problem(s)\n", len(diags))
		return nil, exitFindings
	}
	return m, exitOK
}

// annotationFile makes file (relative to the claims repo root dir) relative to
// the workspace, which is where GitHub resolves annotation paths. Outside a
// workspace, or for a dir outside it, file is returned unchanged.
func annotationFile(workspace, dir, file string) string {
	if workspace == "" {
		return file
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return file
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return file
	}
	rel, err := filepath.Rel(absWorkspace, absDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return file
	}
	return path.Join(filepath.ToSlash(rel), file)
}
```

2. In `runValidate` and `runRender`, replace `m, ok := loadClaims(...)` and `if !ok { return 1 }` with `m, code := loadClaims(*dir, stdout, stderr, env)` and `if code != exitOK { return code }`.
3. Replace the literal `0` and `2` returns with `exitOK` and `exitUsage`.

In `internal/cli/bootstrap.go`:
- `fail` returns `exitError`.
- The findings branch of check returns `exitFindings`.
- Replace the literal `0`, `1` and `2` returns with the constants, keeping their meaning.

- [ ] **Step 8: Update the docs**

In `docs/claims.md`, section "Validation", replace the sentence about exit codes with:

```markdown
It exits 0 when the claims are valid, 3 when it found problems, 1 when it could not run (for example an unreadable directory), and 2 on a usage error ([ADR-0016](adr/0016-exit-codes.md)).
```

In `docs/runbooks/bootstrap.md`, section "4. Verify", after the line `Expected: `bootstrap check: no drift` and exit code 0.`, add:

```markdown
Drift is reported line by line and exits **3**. A check that could not run (network, token, missing repo access) exits **1** ([ADR-0016](../adr/0016-exit-codes.md)).
```

- [ ] **Step 9: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok` for every package and no gofmt output.

- [ ] **Step 10: Commit**

```bash
git add internal/actions internal/claims internal/cli docs
git commit -m "feat(cli): adopt one exit-code convention and workspace-relative annotations"
```

---

### Task 3: `internal/wetdiff` and `idp diff`

**Files:**
- Create: `internal/wetdiff/wetdiff.go`, `internal/wetdiff/wetdiff_test.go`, `internal/cli/diff.go`, `internal/cli/diff_test.go`
- Modify: `internal/cli/cli.go`

**Interfaces:**
- Consumes: `actions.SetOutput` (Task 2).
- Produces (package `internal/wetdiff`):
  - the type `Status` with constants `New`, `Changed`, `Unchanged`, `Orphan`;
  - `type Stack struct { Path string; Status Status }`;
  - `type Result struct { Stacks []Stack; Affected []string }`, with JSON tags `path`, `status`, `stacks`, `affected`;
  - the const `StackFile = "main.tf.json"`;
  - `func Diff(newRoot, wetRoot string, all bool) (Result, error)`.
- Produces (CLI): `idp diff --new DIR --wet DIR [--all]`. It prints one `path: status` line per stack and writes the step output `affected=<JSON array>`.

- [ ] **Step 1: Write the failing test `internal/wetdiff/wetdiff_test.go`**

```go
package wetdiff

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiffStatuses(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, fresh, map[string]string{
		".idp-rendered":                       "marker",
		"github/main.tf.json":                 `{"v":2}`,
		"github/.terraform.lock.hcl":          "lock",
		"aws/dev/_baseline/main.tf.json":      "{}",
		"aws/dev/components/api/main.tf.json": "{}",
	})
	write(t, wet, map[string]string{
		"github/main.tf.json":                  `{"v":1}`,
		"github/.terraform.lock.hcl":           "lock",
		"aws/dev/_baseline/main.tf.json":       "{}",
		"aws/dev/workspaces/logs/main.tf.json": "{}",
	})
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{
		Stacks: []Stack{
			{Path: "aws/dev/_baseline", Status: Unchanged},
			{Path: "aws/dev/components/api", Status: New},
			{Path: "aws/dev/workspaces/logs", Status: Orphan},
			{Path: "github", Status: Changed},
		},
		Affected: []string{"aws/dev/components/api", "aws/dev/workspaces/logs", "github"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDiffAddedNestedFileIsChanged(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, wet, map[string]string{"github/main.tf.json": "{}"})
	write(t, fresh, map[string]string{"github/main.tf.json": "{}", "github/files/api/.github/CODEOWNERS": "* @acme/platform"})
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stacks[0].Status != Changed {
		t.Errorf("status = %s, want changed", got.Stacks[0].Status)
	}
}

func TestDiffWetMissing(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "new")
	write(t, fresh, map[string]string{"github/main.tf.json": "{}"})
	got, err := Diff(fresh, filepath.Join(dir, "does-not-exist"), false)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{Stacks: []Stack{{Path: "github", Status: New}}, Affected: []string{"github"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff = %+v, want %+v", got, want)
	}
}

func TestDiffAllMarksEveryStackAffected(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, fresh, map[string]string{"github/main.tf.json": "{}"})
	write(t, wet, map[string]string{"github/main.tf.json": "{}"})
	got, err := Diff(fresh, wet, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stacks[0].Status != Unchanged || !reflect.DeepEqual(got.Affected, []string{"github"}) {
		t.Errorf("Diff = %+v, want github unchanged but affected", got)
	}
}

func TestDiffIgnoresDotTerraform(t *testing.T) {
	dir := t.TempDir()
	fresh, wet := filepath.Join(dir, "new"), filepath.Join(dir, "wet")
	write(t, fresh, map[string]string{"github/main.tf.json": "{}", "github/.terraform/providers/p": "binary"})
	write(t, wet, map[string]string{"github/main.tf.json": "{}"})
	got, err := Diff(fresh, wet, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stacks[0].Status != Unchanged || len(got.Affected) != 0 {
		t.Errorf("Diff = %+v, want github unchanged and nothing affected", got)
	}
}

func TestDiffNoStacksGivesEmptyLists(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "new")
	write(t, fresh, map[string]string{".idp-rendered": "marker"})
	got, err := Diff(fresh, filepath.Join(dir, "wet"), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stacks == nil || got.Affected == nil || len(got.Stacks) != 0 || len(got.Affected) != 0 {
		t.Errorf("Diff = %#v, want empty, non-nil lists", got)
	}
}

func TestDiffNewRootMissingIsAnError(t *testing.T) {
	if _, err := Diff(filepath.Join(t.TempDir(), "nope"), t.TempDir(), false); err == nil {
		t.Error("want an error for a missing new render")
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/wetdiff/`
Expected: FAIL to compile, with `undefined: Diff`.

- [ ] **Step 3: Implement `internal/wetdiff/wetdiff.go`**

```go
// Package wetdiff compares a fresh render with the render stored on the wet
// branch and lists the stacks to plan (spec §6.1 step 2, §6.2 step 1.1).
package wetdiff

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Status is how a stack's fresh render relates to the one in wet.
type Status string

const (
	New       Status = "new"       // only in the fresh render
	Changed   Status = "changed"   // in both, with different files
	Unchanged Status = "unchanged" // in both, byte for byte
	Orphan    Status = "orphan"    // only in wet: its claims are gone
)

// Stack is one stack directory and its status.
type Stack struct {
	Path   string `json:"path"`
	Status Status `json:"status"`
}

// Result lists every stack and the ones that need a plan.
type Result struct {
	Stacks   []Stack  `json:"stacks"`   // sorted by path
	Affected []string `json:"affected"` // new, changed and orphan stacks (every stack with all)
}

// StackFile marks a directory as a stack.
const StackFile = "main.tf.json"

type stackFiles map[string][]byte // path inside the stack -> content

// Diff compares the rendered trees at newRoot and wetRoot. wetRoot may not
// exist yet (the first reconcile). With all, every stack of the fresh render
// is affected, for a manual reconcile that must re-plan everything.
func Diff(newRoot, wetRoot string, all bool) (Result, error) {
	fresh, err := readStacks(newRoot)
	if err != nil {
		return Result{}, fmt.Errorf("new render: %w", err)
	}
	stored, err := readStacks(wetRoot)
	if errors.Is(err, fs.ErrNotExist) {
		stored = map[string]stackFiles{}
	} else if err != nil {
		return Result{}, fmt.Errorf("wet render: %w", err)
	}
	res := Result{Stacks: []Stack{}, Affected: []string{}}
	for p, files := range fresh {
		status := New
		if old, ok := stored[p]; ok {
			status = Unchanged
			if !sameFiles(files, old) {
				status = Changed
			}
		}
		res.Stacks = append(res.Stacks, Stack{Path: p, Status: status})
		if all || status != Unchanged {
			res.Affected = append(res.Affected, p)
		}
	}
	for p := range stored {
		if _, ok := fresh[p]; !ok {
			res.Stacks = append(res.Stacks, Stack{Path: p, Status: Orphan})
			res.Affected = append(res.Affected, p)
		}
	}
	sort.Slice(res.Stacks, func(i, j int) bool { return res.Stacks[i].Path < res.Stacks[j].Path })
	sort.Strings(res.Affected)
	return res, nil
}

// readStacks maps every stack directory under root (slash path) to its files.
// .terraform directories are skipped; files outside any stack (the render
// marker) are ignored. A missing root returns an error wrapping fs.ErrNotExist.
func readStacks(root string) (map[string]stackFiles, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	var dirs []string
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case d.IsDir():
			if d.Name() == ".terraform" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(p, StackFile)); err == nil && rel != "." {
				dirs = append(dirs, rel)
			}
			return nil
		case d.Type().IsRegular():
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files[rel] = data
			return nil
		default:
			return fmt.Errorf("%s: not a regular file or directory", rel)
		}
	})
	if err != nil {
		return nil, err
	}
	stacks := map[string]stackFiles{}
	for _, d := range dirs {
		stacks[d] = stackFiles{}
	}
	for rel, data := range files {
		if owner := ownerStack(rel, dirs); owner != "" {
			stacks[owner][strings.TrimPrefix(rel, owner+"/")] = data
		}
	}
	return stacks, nil
}

// ownerStack is the deepest stack directory that contains rel, or "".
func ownerStack(rel string, dirs []string) string {
	owner := ""
	for _, d := range dirs {
		if strings.HasPrefix(rel, d+"/") && len(d) > len(owner) {
			owner = d
		}
	}
	return owner
}

func sameFiles(a, b stackFiles) bool {
	if len(a) != len(b) {
		return false
	}
	for p, data := range a {
		if other, ok := b[p]; !ok || !bytes.Equal(data, other) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run the package tests and see them pass**

Run: `go test ./internal/wetdiff/`
Expected: `ok`.

- [ ] **Step 5: Write the failing CLI test `internal/cli/diff_test.go`**

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffCommand(t *testing.T) {
	dir := t.TempDir()
	writeFileAll(t, filepath.Join(dir, "new", "github", "main.tf.json"), "{}")
	out := filepath.Join(dir, "github-output")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"diff", "--new", filepath.Join(dir, "new"), "--wet", filepath.Join(dir, "wet")}, &stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "github: new") || !strings.Contains(stdout.String(), "diff: 1 affected stack(s)") {
		t.Errorf("stdout = %q", stdout.String())
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "affected=[\"github\"]\n" {
		t.Errorf("GITHUB_OUTPUT = %q", got)
	}
}

func TestDiffUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"diff"}, {"diff", "--new", "x"}, {"diff", "--new", "x", "--wet", "y", "extra"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, noEnv); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
```

- [ ] **Step 6: Run it and see it fail**

Run: `go test ./internal/cli/ -run Diff`
Expected: FAIL, because `diff` is an unknown command (exit 2 where the test wants 0).

- [ ] **Step 7: Implement `internal/cli/diff.go` and wire it in**

```go
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
)

func runDiff(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	newDir := fs.String("new", "", "fresh render (the --out of idp render)")
	wetDir := fs.String("wet", "", "rendered/ of the wet branch checkout (may not exist yet)")
	all := fs.Bool("all", false, "treat every stack as affected (manual reconcile)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp diff: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	var missing []string
	if *newDir == "" {
		missing = append(missing, "--new")
	}
	if *wetDir == "" {
		missing = append(missing, "--wet")
	}
	if len(missing) > 0 {
		fmt.Fprintf(stderr, "idp diff: missing %s\n", strings.Join(missing, ", "))
		return exitUsage
	}
	res, err := wetdiff.Diff(*newDir, *wetDir, *all)
	if err != nil {
		return fail(stderr, err)
	}
	for _, s := range res.Stacks {
		fmt.Fprintf(stdout, "%s: %s\n", s.Path, s.Status)
	}
	fmt.Fprintf(stdout, "diff: %d affected stack(s)\n", len(res.Affected))
	affected, err := json.Marshal(res.Affected)
	if err != nil {
		return fail(stderr, err)
	}
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "affected", string(affected)); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}
```

In `internal/cli/cli.go`:
- Add the usage line `  diff            List the stacks whose render differs from the wet branch` after the `render` line.
- Add the dispatch `case "diff": return runDiff(args[1:], stdout, stderr, env)`.

- [ ] **Step 8: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok`, and no gofmt output.

- [ ] **Step 9: Commit**

```bash
git add internal/wetdiff internal/cli
git commit -m "feat(wetdiff): list the stacks to plan with idp diff"
```

---

### Task 4: `internal/plan` — parse, counts, fingerprint (real tofu fixtures)

**Files:**
- Create: `internal/plan/plan.go`, `internal/plan/plan_test.go`
- Create: `internal/plan/testdata/gen/mod/main.tf`, `internal/plan/testdata/gen/step1.tf.txt`, `internal/plan/testdata/gen/step2.tf.txt`, `internal/plan/testdata/gen/README.md`
- Create (generated): `internal/plan/testdata/creates.json`, `internal/plan/testdata/mixed.json`

**Interfaces:**
- Produces (package `internal/plan`):
  - the type `Action` with constants `Create`, `Update`, `Delete`, `Replace`, `Forget`, and `func (a Action) Destructive() bool`;
  - `type Change struct { Address string; Action Action }`, with JSON tags `address`, `action`;
  - `type Plan struct { Stack string; Changes []Change }`;
  - `func Parse(stack string, r io.Reader) (*Plan, error)`;
  - `type Counts struct { Create, Update, Replace, Delete, Forget int }`, with `func (c Counts) Total() int`;
  - `func (p *Plan) Counts() Counts`;
  - `type Fingerprint map[string][]Change`;
  - `func FingerprintOf(plans ...*Plan) Fingerprint`;
  - the methods `(f Fingerprint) Empty() bool`, `(f Fingerprint) Destructive() []string` and `(f Fingerprint) Missing(g Fingerprint) []string`;
  - `func ReadFingerprint(r io.Reader) (Fingerprint, error)`.

- [ ] **Step 1: Write the fixture generator sources**

`internal/plan/testdata/gen/mod/main.tf`:
```hcl
variable "roles" {
  type = map(string)
}

resource "terraform_data" "member" {
  for_each = var.roles
  input    = each.value
}
```

`internal/plan/testdata/gen/step1.tf.txt`:
```hcl
module "group_platform" {
  source = "./mod"
  roles  = { alice = "maintainer", bob = "member", carol = "member" }
}

resource "terraform_data" "replaced" {
  triggers_replace = "v1"
}
```

`internal/plan/testdata/gen/step2.tf.txt`:
```hcl
module "group_platform" {
  source = "./mod"
  roles  = { alice = "maintainer", bob = "maintainer", dave = "member" }
}

resource "terraform_data" "replaced" {
  triggers_replace = "v2"
}
```

`internal/plan/testdata/gen/README.md`:
````markdown
# Plan fixtures

`creates.json` and `mixed.json` are real `tofu show -json` outputs from OpenTofu 1.12.6 (spec §8.2: plan fixtures are real tool output, never hand-written). `terraform_data` is built into OpenTofu, so no provider and no network are needed.

- `creates.json` is step 1 planned against empty state: four creates.
- `mixed.json` is step 2 planned against step 1's state:
  - `alice` is a no-op;
  - `bob` is an update;
  - `carol` is a delete;
  - `dave` is a create;
  - `terraform_data.replaced` is a replace.

Regenerate them from the repo root in Git Bash:

```bash
TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"
W="$(mktemp -d)" && cp -r internal/plan/testdata/gen/. "$W/" && ROOT="$PWD" && cd "$W" \
  && cp step1.tf.txt main.tf && "$TOFU" init -input=false >/dev/null \
  && "$TOFU" plan -input=false -out=p1.bin >/dev/null && "$TOFU" show -json p1.bin > creates.json \
  && "$TOFU" apply -input=false p1.bin >/dev/null \
  && cp step2.tf.txt main.tf && "$TOFU" plan -input=false -out=p2.bin >/dev/null \
  && "$TOFU" show -json p2.bin > mixed.json \
  && cp creates.json mixed.json "$ROOT/internal/plan/testdata/" && cd "$ROOT"
```
````

- [ ] **Step 2: Generate the fixtures**

Run the command block from the README above. The `.tf.txt` extension keeps OpenTofu from reading both steps at once.

Then check them: `rg -o '"actions":\["[a-z-]+"(,"[a-z-]+")?\]' internal/plan/testdata/mixed.json`. Expected: `["no-op"]`, `["update"]`, `["delete"]`, `["create"]`, and `["create","delete"]` or `["delete","create"]`.

If OpenTofu plans a different action than the README describes (for example a replace for `bob`), the fixture is the truth. Change the step files until every action appears, regenerate, and record it in your report. **Never hand-edit the JSON.**

- [ ] **Step 3: Write the failing tests `internal/plan/plan_test.go`**

```go
package plan

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func parseFixture(t *testing.T, name string) *Plan {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, err := Parse("github", f)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const member = `module.group_platform.terraform_data.member`

func TestParseCreatesFixture(t *testing.T) {
	want := []Change{
		{member + `["alice"]`, Create},
		{member + `["bob"]`, Create},
		{member + `["carol"]`, Create},
		{"terraform_data.replaced", Create},
	}
	if got := parseFixture(t, "creates.json").Changes; !reflect.DeepEqual(got, want) {
		t.Errorf("changes =\n%v\nwant\n%v", got, want)
	}
}

func TestParseMixedFixture(t *testing.T) {
	p := parseFixture(t, "mixed.json")
	want := []Change{
		{member + `["bob"]`, Update},
		{member + `["carol"]`, Delete},
		{member + `["dave"]`, Create},
		{"terraform_data.replaced", Replace},
	}
	if !reflect.DeepEqual(p.Changes, want) {
		t.Errorf("changes =\n%v\nwant\n%v", p.Changes, want)
	}
	if got := p.Counts(); got != (Counts{Create: 1, Update: 1, Replace: 1, Delete: 1}) || got.Total() != 4 {
		t.Errorf("counts = %+v (total %d)", got, got.Total())
	}
}

func TestParseNormalizesActions(t *testing.T) {
	src := `{"format_version":"1.2","resource_changes":[
		{"address":"data.x.y","change":{"actions":["read"]}},
		{"address":"b","change":{"actions":["create","delete"]}},
		{"address":"a","change":{"actions":["delete","create"]}},
		{"address":"c","change":{"actions":["no-op"]}},
		{"address":"d","change":{"actions":["forget"]}}]}`
	p, err := Parse("s", strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{{"a", Replace}, {"b", Replace}, {"d", Forget}}
	if !reflect.DeepEqual(p.Changes, want) {
		t.Errorf("changes = %v, want %v", p.Changes, want)
	}
}

func TestParseRejectsUnknownActions(t *testing.T) {
	src := `{"format_version":"1.2","resource_changes":[{"address":"x","change":{"actions":["teleport"]}}]}`
	if _, err := Parse("s", strings.NewReader(src)); err == nil || !strings.Contains(err.Error(), "teleport") {
		t.Errorf("err = %v, want it to name the unsupported action", err)
	}
}

func TestParseRejectsOtherFormatVersions(t *testing.T) {
	for _, v := range []string{`"2.0"`, `""`} {
		src := `{"format_version":` + v + `,"resource_changes":[]}`
		if _, err := Parse("s", strings.NewReader(src)); err == nil {
			t.Errorf("format_version %s: want an error", v)
		}
	}
}

func TestParseEmptyPlanHasNonNilChanges(t *testing.T) {
	p, err := Parse("s", strings.NewReader(`{"format_version":"1.2"}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Changes == nil || len(p.Changes) != 0 {
		t.Errorf("changes = %#v, want empty and non-nil", p.Changes)
	}
}

func TestFingerprint(t *testing.T) {
	mixed := parseFixture(t, "mixed.json")
	f := FingerprintOf(mixed)
	if f.Empty() {
		t.Fatal("fingerprint of a plan with changes must not be empty")
	}
	wantDestructive := []string{
		"github: delete " + member + `["carol"]`,
		"github: replace terraform_data.replaced",
	}
	if got := f.Destructive(); !reflect.DeepEqual(got, wantDestructive) {
		t.Errorf("Destructive = %v, want %v", got, wantDestructive)
	}
	wider := Fingerprint{"github": append(append([]Change{}, mixed.Changes...), Change{"extra", Create})}
	if missing := f.Missing(wider); len(missing) != 0 {
		t.Errorf("Missing(wider) = %v, want none", missing)
	}
	if missing := wider.Missing(f); !reflect.DeepEqual(missing, []string{"github: create extra"}) {
		t.Errorf("Missing(narrower) = %v", missing)
	}
	if missing := f.Missing(Fingerprint{"other": mixed.Changes}); len(missing) != 4 {
		t.Errorf("a stack missing from g must make every change missing, got %v", missing)
	}
	if !(Fingerprint{"github": {}}).Empty() || !(Fingerprint{}).Empty() {
		t.Error("fingerprints without changes must be empty")
	}
}

func TestReadFingerprint(t *testing.T) {
	f, err := ReadFingerprint(strings.NewReader(`{"github":[{"address":"a","action":"create"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f, Fingerprint{"github": {{"a", Create}}}) {
		t.Errorf("fingerprint = %v", f)
	}
	for _, bad := range []string{
		`{"github":[{"address":"a","action":"teleport"}]}`,
		`{"github":[{"address":"","action":"create"}]}`,
		`{"github":[{"address":"a","action":"create","extra":1}]}`,
		`[]`,
	} {
		if _, err := ReadFingerprint(strings.NewReader(bad)); err == nil {
			t.Errorf("ReadFingerprint(%s) succeeded, want an error", bad)
		}
	}
}
```

- [ ] **Step 4: Run them and see them fail**

Run: `go test ./internal/plan/`
Expected: FAIL to compile, with `undefined: Parse`.

- [ ] **Step 5: Implement `internal/plan/plan.go`**

```go
// Package plan reads OpenTofu plans (tofu show -json) and turns them into the
// summaries, fingerprints and gate decisions of spec §6.1 and §6.2 (ADR-0018).
package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Action is a normalized change kind.
type Action string

const (
	Create  Action = "create"
	Update  Action = "update"
	Delete  Action = "delete"
	Replace Action = "replace"
	Forget  Action = "forget" // dropped from state; the real resource stays
)

// Destructive reports whether the action removes a real resource.
func (a Action) Destructive() bool { return a == Delete || a == Replace }

func (a Action) valid() bool {
	switch a {
	case Create, Update, Delete, Replace, Forget:
		return true
	}
	return false
}

// Change is one resource address and what the plan does to it.
type Change struct {
	Address string `json:"address"`
	Action  Action `json:"action"`
}

// Plan is one stack's changes, sorted by address then action. no-op and read
// are excluded: they change nothing.
type Plan struct {
	Stack   string
	Changes []Change
}

// Parse reads the JSON of tofu show -json for one stack. An action it does not
// know fails the parse: an unknown action is never treated as harmless.
func Parse(stack string, r io.Reader) (*Plan, error) {
	var raw struct {
		FormatVersion   string `json:"format_version"`
		ResourceChanges []struct {
			Address string `json:"address"`
			Change  struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("plan %s: %w", stack, err)
	}
	if major, _, _ := strings.Cut(raw.FormatVersion, "."); major != "1" {
		return nil, fmt.Errorf("plan %s: unsupported format_version %q (want 1.x)", stack, raw.FormatVersion)
	}
	p := &Plan{Stack: stack, Changes: []Change{}}
	for _, rc := range raw.ResourceChanges {
		action, isChange, err := normalize(rc.Change.Actions)
		if err != nil {
			return nil, fmt.Errorf("plan %s: %s: %w", stack, rc.Address, err)
		}
		if isChange {
			p.Changes = append(p.Changes, Change{Address: rc.Address, Action: action})
		}
	}
	sortChanges(p.Changes)
	return p, nil
}

func normalize(actions []string) (Action, bool, error) {
	switch strings.Join(actions, ",") {
	case "no-op", "read":
		return "", false, nil
	case "create":
		return Create, true, nil
	case "update":
		return Update, true, nil
	case "delete":
		return Delete, true, nil
	case "delete,create", "create,delete":
		return Replace, true, nil
	case "forget":
		return Forget, true, nil
	}
	return "", false, fmt.Errorf("unsupported actions %v", actions)
}

func sortChanges(cs []Change) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Address != cs[j].Address {
			return cs[i].Address < cs[j].Address
		}
		return cs[i].Action < cs[j].Action
	})
}

// Counts tallies a plan's changes by action.
type Counts struct {
	Create, Update, Replace, Delete, Forget int
}

// Total is the number of changes.
func (c Counts) Total() int { return c.Create + c.Update + c.Replace + c.Delete + c.Forget }

// Counts tallies p's changes.
func (p *Plan) Counts() Counts {
	var c Counts
	for _, ch := range p.Changes {
		switch ch.Action {
		case Create:
			c.Create++
		case Update:
			c.Update++
		case Replace:
			c.Replace++
		case Delete:
			c.Delete++
		case Forget:
			c.Forget++
		}
	}
	return c
}

// Fingerprint is the set of changes per stack (spec §6.1 step 6): what a
// reviewer saw on the PR, and what the reconcile gate compares against it.
type Fingerprint map[string][]Change

// FingerprintOf collects the changes of plans, one entry per stack.
func FingerprintOf(plans ...*Plan) Fingerprint {
	f := Fingerprint{}
	for _, p := range plans {
		f[p.Stack] = append([]Change{}, p.Changes...)
	}
	return f
}

// Empty reports whether no stack has a change.
func (f Fingerprint) Empty() bool {
	for _, cs := range f {
		if len(cs) > 0 {
			return false
		}
	}
	return true
}

// Destructive lists "stack: action address" for every delete or replace, sorted.
func (f Fingerprint) Destructive() []string {
	return f.entries(func(_ string, c Change) bool { return c.Action.Destructive() })
}

// Missing lists "stack: action address" for every change of f that the same
// stack of g does not have, sorted. Empty means f is a subset of g.
func (f Fingerprint) Missing(g Fingerprint) []string {
	have := map[string]map[Change]bool{}
	for stack, cs := range g {
		have[stack] = map[Change]bool{}
		for _, c := range cs {
			have[stack][c] = true
		}
	}
	return f.entries(func(stack string, c Change) bool { return !have[stack][c] })
}

func (f Fingerprint) entries(keep func(stack string, c Change) bool) []string {
	out := []string{}
	for stack, cs := range f {
		for _, c := range cs {
			if keep(stack, c) {
				out = append(out, fmt.Sprintf("%s: %s %s", stack, c.Action, c.Address))
			}
		}
	}
	sort.Strings(out)
	return out
}

// ReadFingerprint decodes a fingerprint written by idp plan-summary, rejecting
// unknown fields, empty addresses and unknown actions.
func ReadFingerprint(r io.Reader) (Fingerprint, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var f Fingerprint
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("fingerprint: %w", err)
	}
	if f == nil {
		return nil, errors.New("fingerprint: want a JSON object")
	}
	for stack, cs := range f {
		for _, c := range cs {
			if c.Address == "" || !c.Action.valid() {
				return nil, fmt.Errorf("fingerprint: stack %s: invalid change %+v", stack, c)
			}
		}
		sortChanges(cs)
	}
	return f, nil
}
```

Decoding `[]` into a map is a JSON type error, so `ReadFingerprint("[]")` fails in `Decode`. The `f == nil` guard covers `null`.

- [ ] **Step 6: Run the tests and see them pass**

Run: `go test ./internal/plan/ && go vet ./... && gofmt -l .`
Expected: `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/plan
git commit -m "feat(plan): parse opentofu plans into counts and fingerprints"
```

---

### Task 5: Plan comment, fingerprint marker and `idp plan-summary`

**Files:**
- Create: `internal/plan/marker.go`, `internal/plan/comment.go`, `internal/plan/comment_test.go`, `internal/plan/testdata/comment-mixed.golden.md`, `internal/cli/plansummary.go`, `internal/cli/plansummary_test.go`
- Modify: `internal/cli/cli.go`

**Interfaces:**
- Consumes: `Parse`, `Plan`, `Counts`, `Fingerprint`, `FingerprintOf` and `ReadFingerprint` (Task 4); `actions.SetOutput` (Task 2).
- Produces (package `internal/plan`):
  - `const CommentMarker = "<!-- idp-plan -->"`;
  - `func ValidSHA(s string) bool`;
  - `func EncodeMarker(headSHA string, f Fingerprint) (string, error)`;
  - `func DecodeMarker(body string) (headSHA string, f Fingerprint, ok bool, err error)`;
  - `func Comment(plans []*Plan, headSHA, runURL string) (string, error)`.
- Produces (CLI): `idp plan-summary [--plan STACK=FILE ...] --head-sha SHA [--run-url URL] --comment-out FILE --fingerprint-out FILE`. It writes the step outputs `changes=<n>` and `destructive=<true|false>`.

- [ ] **Step 1: Write the golden file `internal/plan/testdata/comment-mixed.golden.md`**

This is the exact expected comment for `mixed.json` with head SHA `0123456789abcdef0123456789abcdef01234567` and run URL `https://github.com/acme/idp-claims/actions/runs/1`, without the final fingerprint line. The file ends with one newline after `runs/1)`.

```
<!-- idp-plan -->
## IDP plan

| Stack | Create | Update | Replace | Delete |
|---|---:|---:|---:|---:|
| `github` | 1 | 1 | 1 ⚠️ | 1 ⚠️ |

<details><summary><code>github</code>: +1 ~1 -1 ±1</summary>

| Action | Address |
|---|---|
| update | `module.group_platform.terraform_data.member["bob"]` |
| ⚠️ delete | `module.group_platform.terraform_data.member["carol"]` |
| create | `module.group_platform.terraform_data.member["dave"]` |
| ⚠️ replace | `terraform_data.replaced` |

</details>

> [!WARNING]
> This plan deletes or replaces resources. The reconcile run after merge will wait for approval in the `idp-approval` environment.

Commit `0123456` · [workflow run](https://github.com/acme/idp-claims/actions/runs/1)
```

- [ ] **Step 2: Write the failing tests `internal/plan/comment_test.go`**

```go
package plan

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

const (
	testSHA = "0123456789abcdef0123456789abcdef01234567"
	testRun = "https://github.com/acme/idp-claims/actions/runs/1"
)

// withoutMarker drops the final fingerprint line, whose gzip bytes are not
// part of the reviewed text.
func withoutMarker(t *testing.T, body string) string {
	t.Helper()
	i := strings.LastIndex(body, "<!-- idp-fingerprint:v1 ")
	if i < 0 || !strings.HasSuffix(body, " -->\n") {
		t.Fatalf("comment does not end with a fingerprint marker:\n%s", body)
	}
	return body[:i]
}

func TestCommentGolden(t *testing.T) {
	body, err := Comment([]*Plan{parseFixture(t, "mixed.json")}, testSHA, testRun)
	if err != nil {
		t.Fatal(err)
	}
	got := withoutMarker(t, body)
	golden := filepath.Join("testdata", "comment-mixed.golden.md")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("comment differs from golden:\n%s", got)
	}
}

func TestCommentRoundTripsTheFingerprint(t *testing.T) {
	mixed := parseFixture(t, "mixed.json")
	body, err := Comment([]*Plan{mixed}, testSHA, testRun)
	if err != nil {
		t.Fatal(err)
	}
	sha, f, ok, err := DecodeMarker(body)
	if err != nil || !ok {
		t.Fatalf("DecodeMarker: ok=%v err=%v", ok, err)
	}
	if sha != testSHA || !reflect.DeepEqual(f, FingerprintOf(mixed)) {
		t.Errorf("decoded sha=%s fingerprint=%v", sha, f)
	}
}

func TestCommentWithoutChanges(t *testing.T) {
	body, err := Comment(nil, testSHA, "")
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- idp-plan -->\n## IDP plan\n\nNo infrastructure changes.\n\nCommit `0123456`\n"
	if got := withoutMarker(t, body); got != want {
		t.Errorf("comment = %q, want %q", got, want)
	}
}

func TestCommentTruncatesLongStacks(t *testing.T) {
	p := &Plan{Stack: "github"}
	for i := 0; i < 105; i++ {
		p.Changes = append(p.Changes, Change{fmt.Sprintf("module.component_c%03d.github_repository.this", i), Create})
	}
	body, err := Comment([]*Plan{p}, testSHA, testRun)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(body, "| create |"); got != 100 {
		t.Errorf("rows = %d, want 100", got)
	}
	if !strings.Contains(body, "| … | and 5 more changes |") {
		t.Error("missing the truncation row")
	}
}

func TestCommentOmitsListsWhenTooLarge(t *testing.T) {
	var plans []*Plan
	for s := 0; s < 30; s++ {
		p := &Plan{Stack: fmt.Sprintf("aws/dev/components/c%02d", s)}
		for i := 0; i < 100; i++ {
			p.Changes = append(p.Changes, Change{fmt.Sprintf("module.component_c%02d.aws_iam_role_policy_attachment.very_long_name_%03d", s, i), Create})
		}
		plans = append(plans, p)
	}
	body, err := Comment(plans, testSHA, testRun)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > 65536 {
		t.Errorf("comment is %d bytes, over GitHub's 65536 limit", len(body))
	}
	if !strings.Contains(body, "Change lists are omitted") {
		t.Error("missing the omission note")
	}
	if _, f, ok, err := DecodeMarker(body); err != nil || !ok || len(f) != 30 {
		t.Errorf("fingerprint must survive: ok=%v err=%v stacks=%d", ok, err, len(f))
	}
}

func TestMarkerErrors(t *testing.T) {
	if _, err := EncodeMarker("not-a-sha", Fingerprint{}); err == nil {
		t.Error("EncodeMarker accepted an invalid sha")
	}
	if _, _, ok, err := DecodeMarker("no marker here"); ok || err != nil {
		t.Errorf("DecodeMarker without marker: ok=%v err=%v", ok, err)
	}
	broken := "<!-- idp-fingerprint:v1 sha=" + testSHA + " data=bm90Z3ppcA== -->"
	if _, _, ok, err := DecodeMarker(broken); !ok || err == nil {
		t.Errorf("DecodeMarker with bad data: ok=%v err=%v, want ok and an error", ok, err)
	}
}

func TestValidSHA(t *testing.T) {
	if !ValidSHA(testSHA) || ValidSHA("0123456") || ValidSHA(strings.ToUpper(testSHA)) {
		t.Error("ValidSHA must accept exactly 40 lowercase hex characters")
	}
}
```

- [ ] **Step 3: Run them and see them fail**

Run: `go test ./internal/plan/`
Expected: FAIL to compile, with `undefined: Comment`.

- [ ] **Step 4: Implement `internal/plan/marker.go`**

```go
package plan

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

// CommentMarker identifies the sticky plan comment on a pull request.
const CommentMarker = "<!-- idp-plan -->"

var (
	shaRE    = regexp.MustCompile(`^[0-9a-f]{40}$`)
	markerRE = regexp.MustCompile(`<!-- idp-fingerprint:v1 sha=([0-9a-f]{40}) data=([A-Za-z0-9+/]+={0,2}) -->`)
)

// maxFingerprintBytes bounds the decompressed marker, so a hostile comment
// cannot make the gate inflate a gzip bomb.
const maxFingerprintBytes = 8 << 20

// ValidSHA reports whether s is a full lowercase commit SHA.
func ValidSHA(s string) bool { return shaRE.MatchString(s) }

// EncodeMarker returns the hidden fingerprint line of a plan comment:
// base64 of gzipped JSON, tagged with the head commit it was planned for.
func EncodeMarker(headSHA string, f Fingerprint) (string, error) {
	if !ValidSHA(headSHA) {
		return "", fmt.Errorf("head sha %q: want 40 lowercase hex characters", headSHA)
	}
	raw, err := json.Marshal(f)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("<!-- idp-fingerprint:v1 sha=%s data=%s -->", headSHA, base64.StdEncoding.EncodeToString(buf.Bytes())), nil
}

// DecodeMarker extracts the fingerprint marker from a comment body. ok is
// false when there is no marker; err reports a marker that is present but
// unreadable.
func DecodeMarker(body string) (headSHA string, f Fingerprint, ok bool, err error) {
	m := markerRE.FindStringSubmatch(body)
	if m == nil {
		return "", nil, false, nil
	}
	data, err := base64.StdEncoding.DecodeString(m[2])
	if err != nil {
		return "", nil, true, fmt.Errorf("fingerprint marker: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", nil, true, fmt.Errorf("fingerprint marker: %w", err)
	}
	raw, err := io.ReadAll(io.LimitReader(zr, maxFingerprintBytes+1))
	if err != nil {
		return "", nil, true, fmt.Errorf("fingerprint marker: %w", err)
	}
	if len(raw) > maxFingerprintBytes {
		return "", nil, true, fmt.Errorf("fingerprint marker: decompressed size exceeds %d bytes", maxFingerprintBytes)
	}
	f, err = ReadFingerprint(bytes.NewReader(raw))
	if err != nil {
		return "", nil, true, err
	}
	return m[1], f, true, nil
}
```

- [ ] **Step 5: Implement `internal/plan/comment.go`**

```go
package plan

import (
	"fmt"
	"sort"
	"strings"
)

const (
	maxRowsPerStack = 100
	// maxCommentBytes keeps the comment under GitHub's 65536-character limit,
	// with room for the fingerprint marker.
	maxCommentBytes = 60000
)

// Comment renders the sticky PR plan comment: a summary table, one collapsible
// change list per stack, a warning for destructive plans, and the fingerprint
// marker the reconcile gate trusts (ADR-0018). Change lists are dropped when
// the comment would exceed GitHub's size limit; the marker never is.
func Comment(plans []*Plan, headSHA, runURL string) (string, error) {
	marker, err := EncodeMarker(headSHA, FingerprintOf(plans...))
	if err != nil {
		return "", err
	}
	sorted := append([]*Plan{}, plans...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Stack < sorted[j].Stack })
	body := renderComment(sorted, headSHA, runURL, true)
	if len(body)+len(marker) > maxCommentBytes {
		body = renderComment(sorted, headSHA, runURL, false)
	}
	return body + marker + "\n", nil
}

func renderComment(plans []*Plan, headSHA, runURL string, details bool) string {
	var b strings.Builder
	b.WriteString(CommentMarker + "\n## IDP plan\n\n")
	total, destructive := 0, false
	for _, p := range plans {
		c := p.Counts()
		total += c.Total()
		destructive = destructive || c.Replace+c.Delete > 0
	}
	if total == 0 {
		b.WriteString("No infrastructure changes.\n\n")
	} else {
		b.WriteString("| Stack | Create | Update | Replace | Delete |\n|---|---:|---:|---:|---:|\n")
		for _, p := range plans {
			c := p.Counts()
			fmt.Fprintf(&b, "| `%s` | %d | %d | %s | %s |\n", p.Stack, c.Create, c.Update, flagged(c.Replace), flagged(c.Delete))
		}
		b.WriteString("\n")
		if details {
			for _, p := range plans {
				writeDetails(&b, p)
			}
		} else {
			b.WriteString("Change lists are omitted: this plan is too large for one comment. The workflow run has the full plan.\n\n")
		}
		if destructive {
			b.WriteString("> [!WARNING]\n> This plan deletes or replaces resources. The reconcile run after merge will wait for approval in the `idp-approval` environment.\n\n")
		}
	}
	fmt.Fprintf(&b, "Commit `%s`", headSHA[:7])
	if runURL != "" {
		fmt.Fprintf(&b, " · [workflow run](%s)", runURL)
	}
	b.WriteString("\n")
	return b.String()
}

func writeDetails(b *strings.Builder, p *Plan) {
	if len(p.Changes) == 0 {
		return
	}
	c := p.Counts()
	fmt.Fprintf(b, "<details><summary><code>%s</code>: +%d ~%d -%d ±%d</summary>\n\n", p.Stack, c.Create, c.Update, c.Delete, c.Replace)
	b.WriteString("| Action | Address |\n|---|---|\n")
	for i, ch := range p.Changes {
		if i == maxRowsPerStack {
			fmt.Fprintf(b, "| … | and %d more changes |\n", len(p.Changes)-maxRowsPerStack)
			break
		}
		action := string(ch.Action)
		if ch.Action.Destructive() {
			action = "⚠️ " + action
		}
		fmt.Fprintf(b, "| %s | `%s` |\n", action, ch.Address)
	}
	b.WriteString("\n</details>\n\n")
}

func flagged(n int) string {
	if n == 0 {
		return "0"
	}
	return fmt.Sprintf("%d ⚠️", n)
}
```

- [ ] **Step 6: Run the plan tests and see them pass**

Run: `go test ./internal/plan/`
Expected: `ok`. If the golden differs, the code is wrong; fix the code, not the golden. The only exception is a fixture regenerated in Task 4 Step 2 with different addresses; then update the golden to match and record that in your report.

- [ ] **Step 7: Write the failing CLI tests `internal/cli/plansummary_test.go`**

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

const headSHA = "0123456789abcdef0123456789abcdef01234567"

func TestPlanSummaryCommand(t *testing.T) {
	dir := t.TempDir()
	comment, fp, out := filepath.Join(dir, "c.md"), filepath.Join(dir, "fp.json"), filepath.Join(dir, "out")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plan-summary", "--plan", "github=" + filepath.Join("..", "plan", "testdata", "mixed.json"),
		"--head-sha", headSHA, "--run-url", "https://example.test/run", "--comment-out", comment, "--fingerprint-out", fp},
		&stdout, &stderr, envOf(map[string]string{"GITHUB_OUTPUT": out}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	body, err := os.ReadFile(comment)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), plan.CommentMarker) {
		t.Errorf("comment = %q", body)
	}
	f, err := os.Open(fp)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := plan.ReadFingerprint(f)
	if err != nil || len(got["github"]) != 4 {
		t.Errorf("fingerprint = %v, err %v", got, err)
	}
	outputs, _ := os.ReadFile(out)
	if string(outputs) != "changes=4\ndestructive=true\n" {
		t.Errorf("GITHUB_OUTPUT = %q", outputs)
	}
	if !strings.Contains(stdout.String(), "github: +1 ~1 -1 ±1") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestPlanSummaryWithoutPlans(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "fp.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plan-summary", "--head-sha", headSHA, "--comment-out", filepath.Join(dir, "c.md"), "--fingerprint-out", fp}, &stdout, &stderr, noEnv)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if got, _ := os.ReadFile(fp); string(got) != "{}\n" {
		t.Errorf("fingerprint = %q, want {}", got)
	}
}

func TestPlanSummaryUsageErrors(t *testing.T) {
	dir := t.TempDir()
	base := []string{"--comment-out", filepath.Join(dir, "c"), "--fingerprint-out", filepath.Join(dir, "f")}
	for name, args := range map[string][]string{
		"bad sha":         append([]string{"plan-summary", "--head-sha", "abc"}, base...),
		"plan without =":  append([]string{"plan-summary", "--head-sha", headSHA, "--plan", "github"}, base...),
		"duplicate stack": append([]string{"plan-summary", "--head-sha", headSHA, "--plan", "github=a", "--plan", "github=b"}, base...),
		"missing outputs": {"plan-summary", "--head-sha", headSHA},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, noEnv); code != 2 {
			t.Errorf("%s: exit %d, want 2 (stderr %q)", name, code, stderr.String())
		}
	}
}
```

- [ ] **Step 8: Run them and see them fail**

Run: `go test ./internal/cli/ -run PlanSummary`
Expected: FAIL, because `plan-summary` is an unknown command.

- [ ] **Step 9: Implement `internal/cli/plansummary.go` and wire it in**

```go
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

// planFlags collects repeated --plan STACK=FILE values.
type planFlags []string

func (p *planFlags) String() string { return strings.Join(*p, ",") }

func (p *planFlags) Set(v string) error {
	stack, file, ok := strings.Cut(v, "=")
	if !ok || stack == "" || file == "" {
		return fmt.Errorf("want STACK=FILE, got %q", v)
	}
	*p = append(*p, v)
	return nil
}

func runPlanSummary(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp plan-summary", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var plans planFlags
	fs.Var(&plans, "plan", "STACK=FILE: the tofu show -json output of one stack (repeatable)")
	headSHA := fs.String("head-sha", "", "commit the plans are for (40 hex characters)")
	runURL := fs.String("run-url", "", "link to the workflow run, shown in the comment")
	commentOut := fs.String("comment-out", "", "write the PR comment Markdown here")
	fpOut := fs.String("fingerprint-out", "", "write the fingerprint JSON here")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp plan-summary: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	if *commentOut == "" || *fpOut == "" || *headSHA == "" {
		fmt.Fprintln(stderr, "idp plan-summary: --head-sha, --comment-out and --fingerprint-out are required")
		return exitUsage
	}
	if !plan.ValidSHA(*headSHA) {
		fmt.Fprintf(stderr, "idp plan-summary: --head-sha %q is not a full commit SHA\n", *headSHA)
		return exitUsage
	}
	seen := map[string]bool{}
	for _, entry := range plans {
		stack, _, _ := strings.Cut(entry, "=")
		if seen[stack] {
			fmt.Fprintf(stderr, "idp plan-summary: stack %q given twice\n", stack)
			return exitUsage
		}
		seen[stack] = true
	}
	parsed := []*plan.Plan{}
	for _, entry := range plans {
		stack, file, _ := strings.Cut(entry, "=")
		p, err := parsePlanFile(stack, file)
		if err != nil {
			return fail(stderr, err)
		}
		parsed = append(parsed, p)
	}
	body, err := plan.Comment(parsed, *headSHA, *runURL)
	if err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(*commentOut, []byte(body), 0o644); err != nil {
		return fail(stderr, err)
	}
	fp := plan.FingerprintOf(parsed...)
	raw, err := json.Marshal(fp)
	if err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(*fpOut, append(raw, '\n'), 0o644); err != nil {
		return fail(stderr, err)
	}
	total := 0
	for _, p := range parsed {
		c := p.Counts()
		total += c.Total()
		fmt.Fprintf(stdout, "%s: +%d ~%d -%d ±%d\n", p.Stack, c.Create, c.Update, c.Delete, c.Replace)
	}
	destructive := len(fp.Destructive()) > 0
	fmt.Fprintf(stdout, "plan-summary: %d change(s), destructive: %t\n", total, destructive)
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "changes", strconv.Itoa(total)); err != nil {
		return fail(stderr, err)
	}
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "destructive", strconv.FormatBool(destructive)); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

func parsePlanFile(stack, file string) (*plan.Plan, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return plan.Parse(stack, f)
}
```

In `internal/cli/cli.go`:
- Add the usage line `  plan-summary    Summarize OpenTofu plans as a PR comment and a fingerprint` after `diff`.
- Add the dispatch `case "plan-summary": return runPlanSummary(args[1:], stdout, stderr, env)`.

- [ ] **Step 10: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok`.

- [ ] **Step 11: Commit**

```bash
git add internal/plan internal/cli
git commit -m "feat(plan): render the pr plan comment and fingerprint with idp plan-summary"
```

---

### Task 6: GitHub reads — `ghapi.List`, fake pagination, PR and bot-comment lookup

**Files:**
- Create: `internal/ghapi/list.go`, `internal/ghapi/list_test.go`, `internal/ghops/ghops.go`, `internal/ghops/read_test.go`
- Modify: `internal/ghapi/client.go`, `internal/fakegithub/fakegithub.go`

**Interfaces:**
- Produces (package `internal/ghapi`): `func List[T any](ctx context.Context, c *Client, path string) ([]T, error)`. It follows `Link: rel="next"` only when the link is under the client's base URL, stops after 100 pages, and asks for `per_page=100`.
- Produces (package `internal/fakegithub`):
  - the fields `Lists map[string][]any` and `MaxPerPage int`;
  - `Requests []Request`, with `type Request struct { Method, Path string; Body map[string]any }`;
  - `Actor map[string]any`, which defaults to `{"login": "github-actions[bot]", "type": "Bot"}`.
- Produces (package `internal/ghops`):
  - `type Repo struct { API *ghapi.Client; Owner, Name string }`;
  - `func ParseRepo(api *ghapi.Client, full string) (Repo, error)`;
  - `const ActionsBot = "github-actions[bot]"`;
  - `type PullRequest struct { Number int; MergedAt *string; MergeCommitSHA string; Head struct{ SHA string } }`;
  - `func (r Repo) MergedPullForCommit(ctx, sha string) (*PullRequest, error)`;
  - `type Comment struct { ID int64; Body string; User struct{ Login, Type string } }`;
  - `func (r Repo) FindBotComment(ctx, number int, marker string) (*Comment, error)`.

- [ ] **Step 1: Write the failing test `internal/ghapi/list_test.go`**

```go
package ghapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestListFollowsNextLinks(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Errorf("per_page = %q, want 100", got)
		}
		switch r.URL.Query().Get("page") {
		case "":
			w.Header().Set("Link", fmt.Sprintf(`<%s/items?per_page=100&page=2>; rel="next", <%s/items?per_page=100&page=3>; rel="last"`, srv.URL, srv.URL))
			fmt.Fprint(w, `[1,2]`)
		case "2":
			w.Header().Set("Link", fmt.Sprintf(`<%s/items?per_page=100&page=3>; rel="next"`, srv.URL))
			fmt.Fprint(w, `[3]`)
		default:
			fmt.Fprint(w, `[4]`)
		}
	}))
	defer srv.Close()
	got, err := List[int](context.Background(), New(srv.URL, "t"), "/items")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{1, 2, 3, 4}) {
		t.Errorf("items = %v", got)
	}
}

func TestListKeepsTheQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "state=open&per_page=100" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()
	got, err := List[int](context.Background(), New(srv.URL, ""), "/items?state=open")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("items = %#v, want empty and non-nil", got)
	}
}

func TestListRefusesNextLinksOffTheBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://evil.example/items?page=2>; rel="next"`)
		fmt.Fprint(w, `[1]`)
	}))
	defer srv.Close()
	if _, err := List[int](context.Background(), New(srv.URL, "secret"), "/items"); err == nil || !strings.Contains(err.Error(), "not under") {
		t.Errorf("err = %v, want a refusal to follow the foreign link", err)
	}
}

func TestListBoundsThePageCount(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", fmt.Sprintf(`<%s/items?page=next>; rel="next"`, srv.URL))
		fmt.Fprint(w, `[1]`)
	}))
	defer srv.Close()
	if _, err := List[int](context.Background(), New(srv.URL, ""), "/items"); err == nil || !strings.Contains(err.Error(), "pages") {
		t.Errorf("err = %v, want a page-limit error", err)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/ghapi/`
Expected: FAIL to compile, with `undefined: List`.

- [ ] **Step 3: Expose response headers and implement `List`**

In `internal/ghapi/client.go`, turn `Do` into a wrapper around a header-returning `do`. Keep every existing behaviour and message:

```go
// Do performs one request. A 404 returns an error wrapping ErrNotFound;
// any other status >= 300 returns *APIError.
func (c *Client) Do(ctx context.Context, method, path string, in, out any) error {
	_, err := c.do(ctx, method, path, in, out)
	return err
}
```

Then rename the old body to `func (c *Client) do(ctx context.Context, method, path string, in, out any) (http.Header, error)`:
- every `return err`-style line becomes `return nil, <same error>`;
- the success paths return `resp.Header, nil`.

Create `internal/ghapi/list.go`:

```go
package ghapi

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// maxPages bounds List, so a misbehaving server cannot loop it forever.
const maxPages = 100

var linkNext = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// List GETs every page of a list endpoint, following GitHub's Link
// rel="next" headers, and returns all items. path must not set per_page.
func List[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	next := path + sep + "per_page=100"
	all := []T{}
	for pages := 0; next != ""; pages++ {
		if pages == maxPages {
			return nil, fmt.Errorf("list %s: more than %d pages", path, maxPages)
		}
		var items []T
		hdr, err := c.do(ctx, http.MethodGet, next, nil, &items)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if next, err = c.nextPage(hdr.Get("Link")); err != nil {
			return nil, fmt.Errorf("list %s: %w", path, err)
		}
	}
	return all, nil
}

// nextPage returns the path of the rel="next" link, or "". The link must point
// under this client's base URL: the token is never sent anywhere else.
func (c *Client) nextPage(link string) (string, error) {
	m := linkNext.FindStringSubmatch(link)
	if m == nil {
		return "", nil
	}
	p, ok := strings.CutPrefix(m[1], c.baseURL)
	if !ok || !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("next page %q is not under %s", m[1], c.baseURL)
	}
	return p, nil
}
```

- [ ] **Step 4: Run the ghapi tests and see them pass**

Run: `go test ./internal/ghapi/`
Expected: `ok`, including every pre-existing ghapi test.

- [ ] **Step 5: Add list pagination, request recording and an actor to the fake**

In `internal/fakegithub/fakegithub.go`:

1. Add to the `Server` struct, after `Queries`:

```go
	// Lists maps a GET path to every item of a paginated list endpoint. The fake
	// serves pages with per_page and page, and a Link rel="next" header.
	Lists map[string][]any
	// MaxPerPage, when positive, caps per_page so tests can force several pages.
	MaxPerPage int
	// Requests holds every non-GET request with its decoded JSON body, in order.
	Requests []Request
	// Actor is the user the fake attributes the comments it creates to.
	Actor map[string]any
```

   And add the type:

```go
// Request is one recorded write.
type Request struct {
	Method string
	Path   string
	Body   map[string]any
}
```

2. In `New`, initialise `Lists: map[string][]any{}` and `Actor: map[string]any{"login": "github-actions[bot]", "type": "Bot"}`.

3. In `ServeHTTP`, right after the `Writes` append, record the request:

```go
	if r.Method != http.MethodGet {
		f.Writes = append(f.Writes, r.Method+" "+r.URL.Path)
		f.Requests = append(f.Requests, Request{Method: r.Method, Path: r.URL.Path, Body: body})
	}
	if r.Method == http.MethodGet {
		if items, ok := f.Lists[r.URL.Path]; ok || isListPath(r.URL.Path) {
			f.servePage(w, r, items)
			return
		}
	}
```

   Keep the existing single `if r.Method != http.MethodGet` block merged with the new line; do not duplicate it.

4. Add the list endpoints the commands read to the `var (...)` regexp block, and the helper. Like GitHub, the fake answers an unseeded list with `[]`, not 404:

```go
	reIssueComments  = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/\d+/comments$`)
	reIssues         = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues$`)
	rePullsForCommit = regexp.MustCompile(`^/repos/[^/]+/[^/]+/commits/[^/]+/pulls$`)
```

```go
// isListPath reports whether path is a list endpoint the idp commands read.
func isListPath(path string) bool {
	return reIssueComments.MatchString(path) || reIssues.MatchString(path) || rePullsForCommit.MatchString(path)
}
```

5. Add the page server, and add `"fmt"` to the imports:

```go
// servePage answers one page of a list endpoint the way GitHub does.
func (f *Server) servePage(w http.ResponseWriter, r *http.Request, items []any) {
	q := r.URL.Query()
	per, _ := strconv.Atoi(q.Get("per_page"))
	if per <= 0 {
		per = 30
	}
	if per > 100 {
		per = 100
	}
	if f.MaxPerPage > 0 && per > f.MaxPerPage {
		per = f.MaxPerPage
	}
	page, _ := strconv.Atoi(q.Get("page"))
	if page <= 0 {
		page = 1
	}
	start := min((page-1)*per, len(items))
	end := min(start+per, len(items))
	if end < len(items) {
		q.Set("page", strconv.Itoa(page+1))
		w.Header().Set("Link", fmt.Sprintf(`<%s%s?%s>; rel="next"`, f.URL, r.URL.Path, q.Encode()))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(append([]any{}, items[start:end]...))
}
```

6. Update the package comment's last sentence to: `It implements the endpoints the idp commands call.`

- [ ] **Step 6: Write the failing test `internal/ghops/read_test.go`**

```go
package ghops

import (
	"context"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

func newRepo(t *testing.T) (*fakegithub.Server, Repo) {
	t.Helper()
	fake := fakegithub.New(t, "acme")
	repo, err := ParseRepo(ghapi.New(fake.URL, "test-token"), "acme/idp-claims")
	if err != nil {
		t.Fatal(err)
	}
	return fake, repo
}

func user(login, kind string) map[string]any { return map[string]any{"login": login, "type": kind} }

func TestParseRepo(t *testing.T) {
	for _, bad := range []string{"", "acme", "acme/", "/repo", "a/b/c"} {
		if _, err := ParseRepo(nil, bad); err == nil {
			t.Errorf("ParseRepo(%q) succeeded, want an error", bad)
		}
	}
}

func TestMergedPullForCommit(t *testing.T) {
	fake, repo := newRepo(t)
	merged := "2026-10-10T10:00:00Z"
	fake.Lists["/repos/acme/idp-claims/commits/m/pulls"] = []any{
		map[string]any{"number": 1, "merged_at": nil, "merge_commit_sha": "m", "head": map[string]any{"sha": "h1"}},
		map[string]any{"number": 2, "merged_at": merged, "merge_commit_sha": "other", "head": map[string]any{"sha": "h2"}},
		map[string]any{"number": 3, "merged_at": merged, "merge_commit_sha": "m", "head": map[string]any{"sha": "h3"}},
	}
	pr, err := repo.MergedPullForCommit(context.Background(), "m")
	if err != nil {
		t.Fatal(err)
	}
	if pr == nil || pr.Number != 3 || pr.Head.SHA != "h3" {
		t.Errorf("pr = %+v, want #3", pr)
	}
	none, err := repo.MergedPullForCommit(context.Background(), "unknown")
	if err != nil || none != nil {
		t.Errorf("unknown commit: pr=%+v err=%v, want nil, nil", none, err)
	}
}

func TestFindBotCommentNewestTrustedOnly(t *testing.T) {
	fake, repo := newRepo(t)
	fake.Lists["/repos/acme/idp-claims/issues/7/comments"] = []any{
		map[string]any{"id": 1, "body": "<!-- idp-plan --> old", "user": user(ActionsBot, "Bot")},
		map[string]any{"id": 2, "body": "<!-- idp-plan --> newer", "user": user(ActionsBot, "Bot")},
		map[string]any{"id": 3, "body": "<!-- idp-plan --> forged", "user": user("mallory", "User")},
		map[string]any{"id": 4, "body": "unrelated bot note", "user": user(ActionsBot, "Bot")},
	}
	c, err := repo.FindBotComment(context.Background(), 7, "<!-- idp-plan -->")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.ID != 2 {
		t.Errorf("comment = %+v, want id 2 (newest by the Actions bot with the marker)", c)
	}
}

func TestFindBotCommentAcrossPages(t *testing.T) {
	fake, repo := newRepo(t)
	fake.MaxPerPage = 1
	fake.Lists["/repos/acme/idp-claims/issues/7/comments"] = []any{
		map[string]any{"id": 1, "body": "hi", "user": user("alice", "User")},
		map[string]any{"id": 2, "body": "hello", "user": user("bob", "User")},
		map[string]any{"id": 3, "body": "<!-- idp-plan --> plan", "user": user(ActionsBot, "Bot")},
	}
	c, err := repo.FindBotComment(context.Background(), 7, "<!-- idp-plan -->")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.ID != 3 {
		t.Errorf("comment = %+v, want id 3 from the third page", c)
	}
}
```

- [ ] **Step 7: Run it and see it fail**

Run: `go test ./internal/ghops/`
Expected: FAIL to compile, with `undefined: ParseRepo`.

- [ ] **Step 8: Implement `internal/ghops/ghops.go`**

```go
// Package ghops holds the pipelines' conversations with GitHub: the pull
// request behind a commit, the bot's sticky plan comment, and labelled issues.
package ghops

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// ActionsBot authors every comment made with a workflow's GITHUB_TOKEN.
const ActionsBot = "github-actions[bot]"

// Repo is one repository reached through API.
type Repo struct {
	API   *ghapi.Client
	Owner string
	Name  string
}

// ParseRepo splits "OWNER/NAME".
func ParseRepo(api *ghapi.Client, full string) (Repo, error) {
	owner, name, ok := strings.Cut(full, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return Repo{}, fmt.Errorf("repo %q: want OWNER/NAME", full)
	}
	return Repo{API: api, Owner: owner, Name: name}, nil
}

func (r Repo) base() string {
	return "/repos/" + url.PathEscape(r.Owner) + "/" + url.PathEscape(r.Name)
}

// PullRequest is the part of a pull request the gate needs.
type PullRequest struct {
	Number         int     `json:"number"`
	MergedAt       *string `json:"merged_at"`
	MergeCommitSHA string  `json:"merge_commit_sha"`
	Head           struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

// MergedPullForCommit returns the merged pull request whose merge commit is
// sha (the commit a push to main carries), or nil.
func (r Repo) MergedPullForCommit(ctx context.Context, sha string) (*PullRequest, error) {
	prs, err := ghapi.List[PullRequest](ctx, r.API, r.base()+"/commits/"+url.PathEscape(sha)+"/pulls")
	if err != nil {
		return nil, err
	}
	for i := range prs {
		if prs[i].MergedAt != nil && prs[i].MergeCommitSHA == sha {
			return &prs[i], nil
		}
	}
	return nil, nil
}

// Comment is an issue or pull request comment.
type Comment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
}

func (c Comment) byActionsBot() bool { return c.User.Login == ActionsBot && c.User.Type == "Bot" }

// FindBotComment returns the newest comment on issue or pull request number
// that the Actions bot wrote and that contains marker, or nil. Comments by
// anyone else are ignored, whatever they contain (ADR-0018).
func (r Repo) FindBotComment(ctx context.Context, number int, marker string) (*Comment, error) {
	comments, err := ghapi.List[Comment](ctx, r.API, fmt.Sprintf("%s/issues/%d/comments", r.base(), number))
	if err != nil {
		return nil, err
	}
	for i := len(comments) - 1; i >= 0; i-- {
		if comments[i].byActionsBot() && strings.Contains(comments[i].Body, marker) {
			return &comments[i], nil
		}
	}
	return nil, nil
}
```

- [ ] **Step 9: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok`. Every pre-existing bootstrap test must still pass with the extended fake.

- [ ] **Step 10: Commit**

```bash
git add internal/ghapi internal/ghops internal/fakegithub
git commit -m "feat(ghops): find the merged pull request and the trusted plan comment"
```

---

### Task 7: The gate — `plan.Decide` and `idp gate`

**Files:**
- Create: `internal/plan/gate.go`, `internal/plan/gate_test.go`, `internal/cli/gate.go`, `internal/cli/gate_test.go`
- Modify: `internal/cli/cli.go`

**Interfaces:**
- Consumes:
  - from Tasks 4–5: `Fingerprint`, `ReadFingerprint`, `DecodeMarker`, `CommentMarker`, `ValidSHA` and `EncodeMarker`;
  - from Task 6: `ghops.ParseRepo`, `MergedPullForCommit`, `FindBotComment` and `ActionsBot`;
  - from Task 2: `actions.SetOutput` and `actions.AddSummary`.
- Produces (package `internal/plan`):
  - the type `Decision` with constants `Auto` and `Approval`;
  - `type Verdict struct { Decision Decision; Reason string }`;
  - `type PRPlan struct { Number int; Fingerprint Fingerprint }`;
  - `func NeedsPR(current Fingerprint) bool`;
  - `func Decide(current Fingerprint, pr *PRPlan, whyNoPR string) Verdict`.
- Produces (CLI): `idp gate --repo OWNER/REPO --sha SHA --fingerprint FILE`. It prints `gate: <decision> (<reason>)`, writes the step output `decision=<auto|approval>`, and appends a line to the job summary.

- [ ] **Step 1: Write the failing test `internal/plan/gate_test.go`**

```go
package plan

import (
	"strings"
	"testing"
)

func TestDecide(t *testing.T) {
	create := Fingerprint{"github": {{"a", Create}}}
	update := Fingerprint{"github": {{"a", Update}}}
	destroy := Fingerprint{"github": {{"a", Delete}}}
	tests := []struct {
		name       string
		current    Fingerprint
		pr         *PRPlan
		why        string
		want       Decision
		wantReason string
	}{
		{"nothing planned is auto even without a PR", Fingerprint{"github": {}}, nil, "", Auto, "no changes"},
		{"destructive is approval even with a matching PR", destroy, &PRPlan{Number: 1, Fingerprint: destroy}, "", Approval, "destructive changes: github: delete a"},
		{"no PR is approval with its reason", create, nil, "no merged pull request", Approval, "no merged pull request"},
		{"no PR without a reason gets a default one", create, nil, "", Approval, "no pull request plan"},
		{"subset of the PR is auto", create, &PRPlan{Number: 7, Fingerprint: Fingerprint{"github": {{"a", Create}, {"b", Create}}}}, "", Auto, "PR #7"},
		{"an action the PR did not show is approval", update, &PRPlan{Number: 7, Fingerprint: create}, "", Approval, "changes not shown in PR #7: github: update a"},
		{"a stack the PR did not show is approval", Fingerprint{"aws/dev/_baseline": {{"x", Create}}}, &PRPlan{Number: 7, Fingerprint: create}, "", Approval, "aws/dev/_baseline: create x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Decide(tt.current, tt.pr, tt.why)
			if v.Decision != tt.want || !strings.Contains(v.Reason, tt.wantReason) {
				t.Errorf("Decide = %+v, want %s containing %q", v, tt.want, tt.wantReason)
			}
		})
	}
}

func TestDecideSummarizesLongLists(t *testing.T) {
	var cs []Change
	for _, a := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		cs = append(cs, Change{a, Delete})
	}
	v := Decide(Fingerprint{"github": cs}, nil, "")
	if !strings.HasSuffix(v.Reason, "and 2 more") {
		t.Errorf("reason = %q, want it to end with %q", v.Reason, "and 2 more")
	}
}

func TestNeedsPR(t *testing.T) {
	if NeedsPR(Fingerprint{}) || NeedsPR(Fingerprint{"github": {{"a", Delete}}}) || !NeedsPR(Fingerprint{"github": {{"a", Create}}}) {
		t.Error("NeedsPR must be true only for non-empty, non-destructive fingerprints")
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/plan/ -run "Decide|NeedsPR"`
Expected: FAIL to compile, with `undefined: Decide`.

- [ ] **Step 3: Implement `internal/plan/gate.go`**

```go
package plan

import (
	"fmt"
	"strings"
)

// Decision is what the reconcile gate decided.
type Decision string

const (
	Auto     Decision = "auto"     // apply without waiting
	Approval Decision = "approval" // wait for a reviewer in idp-approval
)

// Verdict is a decision and why it was made.
type Verdict struct {
	Decision Decision
	Reason   string
}

// PRPlan is the fingerprint a reviewer saw on the merged pull request.
type PRPlan struct {
	Number      int
	Fingerprint Fingerprint
}

// NeedsPR reports whether Decide needs the pull request's fingerprint, so a
// caller can skip the GitHub lookups when the answer is already known.
func NeedsPR(current Fingerprint) bool {
	return !current.Empty() && len(current.Destructive()) == 0
}

// Decide applies spec §6.2 step 1.5 and amendment A3 (ADR-0018). Nothing to
// apply is auto. Any delete or replace is approval. Otherwise it is auto only
// when every change appears in the same stack of the PR's fingerprint.
// whyNoPR explains a nil pr in the reason.
func Decide(current Fingerprint, pr *PRPlan, whyNoPR string) Verdict {
	if current.Empty() {
		return Verdict{Auto, "no changes to apply"}
	}
	if d := current.Destructive(); len(d) > 0 {
		return Verdict{Approval, "destructive changes: " + summarize(d)}
	}
	if pr == nil {
		if whyNoPR == "" {
			whyNoPR = "no pull request plan found for this commit"
		}
		return Verdict{Approval, whyNoPR}
	}
	if missing := current.Missing(pr.Fingerprint); len(missing) > 0 {
		return Verdict{Approval, fmt.Sprintf("changes not shown in PR #%d: %s", pr.Number, summarize(missing))}
	}
	return Verdict{Auto, fmt.Sprintf("every change was shown in PR #%d", pr.Number)}
}

func summarize(entries []string) string {
	const limit = 5
	if len(entries) <= limit {
		return strings.Join(entries, "; ")
	}
	return strings.Join(entries[:limit], "; ") + fmt.Sprintf(" and %d more", len(entries)-limit)
}
```

- [ ] **Step 4: Run the plan tests and see them pass**

Run: `go test ./internal/plan/`
Expected: `ok`.

- [ ] **Step 5: Write the failing CLI test `internal/cli/gate_test.go`**

```go
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

const (
	mergeSHA = "1111111111111111111111111111111111111111"
	prHead   = "2222222222222222222222222222222222222222"
	oldHead  = "3333333333333333333333333333333333333333"
)

func writeFingerprint(t *testing.T, f plan.Fingerprint) string {
	t.Helper()
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "fingerprint.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func planComment(t *testing.T, head string, f plan.Fingerprint) string {
	t.Helper()
	m, err := plan.EncodeMarker(head, f)
	if err != nil {
		t.Fatal(err)
	}
	return plan.CommentMarker + "\n## IDP plan\n" + m + "\n"
}

func seedMergedPR(fake *fakegithub.Server, comments ...any) {
	fake.Lists["/repos/acme/idp-claims/commits/"+mergeSHA+"/pulls"] = []any{map[string]any{
		"number": 7, "merged_at": "2026-10-10T10:00:00Z", "merge_commit_sha": mergeSHA, "head": map[string]any{"sha": prHead},
	}}
	fake.Lists["/repos/acme/idp-claims/issues/7/comments"] = comments
}

// gateRun runs idp gate against fake (nil when no API call is expected) and
// returns the exit code, the decision output line and everything printed.
func gateRun(t *testing.T, fake *fakegithub.Server, current plan.Fingerprint) (code int, decision, stdout string) {
	t.Helper()
	dir := t.TempDir()
	out, summary := filepath.Join(dir, "out"), filepath.Join(dir, "summary")
	vars := map[string]string{"GITHUB_OUTPUT": out, "GITHUB_STEP_SUMMARY": summary, "GH_TOKEN": "test-token"}
	if fake != nil {
		vars["IDP_GITHUB_API"] = fake.URL
	}
	var so, se bytes.Buffer
	code = Run([]string{"gate", "--repo", "acme/idp-claims", "--sha", mergeSHA, "--fingerprint", writeFingerprint(t, current)}, &so, &se, envOf(vars))
	got, _ := os.ReadFile(out)
	return code, strings.TrimSpace(string(got)), so.String() + se.String()
}

func TestGateAutoWhenNothingPlanned(t *testing.T) {
	code, decision, _ := gateRun(t, nil, plan.Fingerprint{"github": {}})
	if code != 0 || decision != "decision=auto" {
		t.Errorf("exit %d, %q", code, decision)
	}
}

func TestGateApprovalWhenDestructive(t *testing.T) {
	code, decision, out := gateRun(t, nil, plan.Fingerprint{"github": {{Address: "a", Action: plan.Delete}}})
	if code != 0 || decision != "decision=approval" || !strings.Contains(out, "destructive") {
		t.Errorf("exit %d, %q, %q", code, decision, out)
	}
}

func TestGateAutoWhenSubsetOfTrustedComment(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	shown := plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}, {Address: "b", Action: plan.Create}}}
	seedMergedPR(fake, map[string]any{"id": 1, "body": planComment(t, prHead, shown), "user": map[string]any{"login": "github-actions[bot]", "type": "Bot"}})
	code, decision, out := gateRun(t, fake, plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}}})
	if code != 0 || decision != "decision=auto" || !strings.Contains(out, "PR #7") {
		t.Errorf("exit %d, %q, %q", code, decision, out)
	}
}

func TestGateIgnoresUntrustedComments(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	wide := plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}}}
	seedMergedPR(fake,
		map[string]any{"id": 1, "body": planComment(t, oldHead, wide), "user": map[string]any{"login": "github-actions[bot]", "type": "Bot"}},
		map[string]any{"id": 2, "body": planComment(t, prHead, wide), "user": map[string]any{"login": "mallory", "type": "User"}},
	)
	code, decision, out := gateRun(t, fake, wide)
	if code != 0 || decision != "decision=approval" || !strings.Contains(out, "is for 3333333") {
		t.Errorf("exit %d, %q, %q: the bot comment is stale and the fresh one is not the bot's", code, decision, out)
	}
}

func TestGateApprovalWhenNoMergedPR(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	fake.Lists["/repos/acme/idp-claims/commits/"+mergeSHA+"/pulls"] = []any{}
	code, decision, out := gateRun(t, fake, plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}}})
	if code != 0 || decision != "decision=approval" || !strings.Contains(out, "no merged pull request") {
		t.Errorf("exit %d, %q, %q", code, decision, out)
	}
}

func TestGateApprovalWhenChangeMissingFromPR(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	seedMergedPR(fake, map[string]any{"id": 1, "body": planComment(t, prHead, plan.Fingerprint{"github": {{Address: "a", Action: plan.Create}}}),
		"user": map[string]any{"login": "github-actions[bot]", "type": "Bot"}})
	code, decision, out := gateRun(t, fake, plan.Fingerprint{"github": {{Address: "a", Action: plan.Update}}})
	if code != 0 || decision != "decision=approval" || !strings.Contains(out, "not shown in PR #7") {
		t.Errorf("exit %d, %q, %q", code, decision, out)
	}
}

func TestGateUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"gate"},
		{"gate", "--repo", "acme/idp-claims", "--sha", "short", "--fingerprint", "f"},
		{"gate", "--repo", "acme", "--sha", mergeSHA, "--fingerprint", "f"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, noEnv); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
```

- [ ] **Step 6: Run it and see it fail**

Run: `go test ./internal/cli/ -run Gate`
Expected: FAIL, because `gate` is an unknown command.

- [ ] **Step 7: Implement `internal/cli/gate.go` and wire it in**

```go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
	"github.com/jellalshadows-idp/idp-engine/internal/ghops"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

func runGate(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp gate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoFlag := fs.String("repo", "", "claims repository, OWNER/REPO")
	sha := fs.String("sha", "", "commit being reconciled (40 hex characters)")
	fpFile := fs.String("fingerprint", "", "fingerprint JSON written by idp plan-summary")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 || *repoFlag == "" || *sha == "" || *fpFile == "" {
		fmt.Fprintln(stderr, "idp gate: --repo, --sha and --fingerprint are required")
		return exitUsage
	}
	if !plan.ValidSHA(*sha) {
		fmt.Fprintf(stderr, "idp gate: --sha %q is not a full commit SHA\n", *sha)
		return exitUsage
	}
	repo, err := ghops.ParseRepo(nil, *repoFlag)
	if err != nil {
		fmt.Fprintln(stderr, "idp gate:", err)
		return exitUsage
	}
	f, err := os.Open(*fpFile)
	if err != nil {
		return fail(stderr, err)
	}
	current, err := plan.ReadFingerprint(f)
	f.Close()
	if err != nil {
		return fail(stderr, err)
	}
	var pr *plan.PRPlan
	why := ""
	if plan.NeedsPR(current) {
		tok := token(env)
		if tok == "" {
			fmt.Fprintln(stderr, "idp gate: set GH_TOKEN or GITHUB_TOKEN")
			return exitUsage
		}
		repo.API = ghapi.New(apiBase(env), tok)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if pr, why, err = trustedPRPlan(ctx, repo, *sha); err != nil {
			return fail(stderr, err)
		}
	}
	v := plan.Decide(current, pr, why)
	fmt.Fprintf(stdout, "gate: %s (%s)\n", v.Decision, v.Reason)
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "decision", string(v.Decision)); err != nil {
		return fail(stderr, err)
	}
	if err := actions.AddSummary(env("GITHUB_STEP_SUMMARY"), fmt.Sprintf("**Gate:** `%s` — %s", v.Decision, v.Reason)); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

// trustedPRPlan finds the fingerprint a reviewer saw (ADR-0018): the newest
// plan comment by the Actions bot on the PR merged as sha, made for that PR's
// final head commit. A string explains why there is none.
func trustedPRPlan(ctx context.Context, repo ghops.Repo, sha string) (*plan.PRPlan, string, error) {
	pr, err := repo.MergedPullForCommit(ctx, sha)
	if err != nil {
		return nil, "", err
	}
	if pr == nil {
		return nil, fmt.Sprintf("no merged pull request has %s as its merge commit", sha[:7]), nil
	}
	c, err := repo.FindBotComment(ctx, pr.Number, plan.CommentMarker)
	if err != nil {
		return nil, "", err
	}
	if c == nil {
		return nil, fmt.Sprintf("PR #%d has no plan comment from %s", pr.Number, ghops.ActionsBot), nil
	}
	head, fp, ok, err := plan.DecodeMarker(c.Body)
	switch {
	case err != nil:
		return nil, fmt.Sprintf("the plan comment on PR #%d has an unreadable fingerprint: %v", pr.Number, err), nil
	case !ok:
		return nil, fmt.Sprintf("the plan comment on PR #%d has no fingerprint", pr.Number), nil
	case head != pr.Head.SHA:
		return nil, fmt.Sprintf("the plan comment on PR #%d is for %s, but the PR merged %s", pr.Number, head[:7], shortSHA(pr.Head.SHA)), nil
	}
	return &plan.PRPlan{Number: pr.Number, Fingerprint: fp}, "", nil
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
```

In `internal/cli/cli.go`:
- Add the usage line `  gate            Decide whether a reconcile applies automatically or waits for approval` after `plan-summary`.
- Add the dispatch `case "gate": return runGate(args[1:], stdout, stderr, env)`.

- [ ] **Step 8: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok`.

- [ ] **Step 9: Commit**

```bash
git add internal/plan internal/cli
git commit -m "feat(gate): decide auto or approval from the trusted pr fingerprint"
```

---

### Task 8: GitHub writes — sticky comment, labelled issues, `idp comment`, `idp issue`

**Files:**
- Create: `internal/ghops/write.go`, `internal/ghops/write_test.go`, `internal/cli/conversations.go`, `internal/cli/conversations_test.go`
- Modify: `internal/fakegithub/fakegithub.go`, `internal/cli/cli.go`

**Interfaces:**
- Consumes: `Repo`, `FindBotComment` and `ActionsBot` (Task 6); `ghapi.List`; `plan.CommentMarker`; the fake's `Lists`, `Requests` and `Actor`.
- Produces (package `internal/ghops`):
  - `func (r Repo) UpsertBotComment(ctx, number int, marker, body string) (id int64, created bool, err error)`;
  - `type Issue struct { Number int; Title, State string; Labels []struct{ Name string }; PullRequest *json.RawMessage }`;
  - `func (r Repo) FindOpenIssue(ctx, label, title string) (*Issue, error)`;
  - `func (r Repo) EnsureLabel(ctx, label, color, description string) error`;
  - `func (r Repo) OpenIssue(ctx, label, title, body string) (number int, created bool, err error)`;
  - `func (r Repo) CloseIssue(ctx, label, title, comment string) (number int, closed bool, err error)`.
- Produces (CLI):
  - `idp comment --repo OWNER/REPO --pr N --body-file FILE [--marker M]`;
  - `idp issue open --repo OWNER/REPO --label L --title T --body-file FILE`;
  - `idp issue close --repo OWNER/REPO --label L --title T [--comment TEXT]`.

- [ ] **Step 1: Add the comment, issue and label routes to the fake**

In `internal/fakegithub/fakegithub.go`, add regexps to the `var (...)` block. `reIssueComments` and `reIssues` already exist from the earlier task:

```go
	reIssueComment = regexp.MustCompile(`^(/repos/[^/]+/[^/]+)/issues/comments/(\d+)$`)
	reIssue        = regexp.MustCompile(`^(/repos/[^/]+/[^/]+/issues)/(\d+)$`)
	reLabels       = regexp.MustCompile(`^/repos/[^/]+/[^/]+/labels$`)
```

Add these cases to `route`, before the generic `case method == http.MethodGet:`:

```go
	case method == http.MethodPost && reIssueComments.MatchString(path):
		f.nextID++
		comment := map[string]any{"id": f.nextID, "body": body["body"], "user": f.Actor}
		f.Lists[path] = append(f.Lists[path], comment)
		return http.StatusCreated, comment
	case method == http.MethodPatch && reIssueComment.MatchString(path):
		m := reIssueComment.FindStringSubmatch(path)
		for listPath, items := range f.Lists {
			if !strings.HasPrefix(listPath, m[1]+"/issues/") || !strings.HasSuffix(listPath, "/comments") {
				continue
			}
			for _, it := range items {
				c, _ := it.(map[string]any)
				if strconv.FormatInt(int64(toFloat(c["id"])), 10) == m[2] {
					c["body"] = body["body"]
					return http.StatusOK, c
				}
			}
		}
		return notFound()
	case method == http.MethodPost && reIssues.MatchString(path):
		labels := []any{}
		for _, l := range asList(body["labels"]) {
			labels = append(labels, map[string]any{"name": l})
		}
		issue := map[string]any{"number": len(f.Lists[path]) + 1, "title": body["title"], "body": body["body"], "state": "open", "labels": labels}
		f.Lists[path] = append(f.Lists[path], issue)
		return http.StatusCreated, issue
	case method == http.MethodPatch && reIssue.MatchString(path):
		m := reIssue.FindStringSubmatch(path)
		for _, it := range f.Lists[m[1]] {
			is, _ := it.(map[string]any)
			if strconv.Itoa(int(toFloat(is["number"]))) == m[2] {
				for k, v := range body {
					is[k] = v
				}
				return http.StatusOK, is
			}
		}
		return notFound()
	case method == http.MethodPost && reLabels.MatchString(path):
		name, ok := f.stringField(method, path, body, "name")
		if !ok {
			return http.StatusBadRequest, map[string]any{}
		}
		f.Objects[path+"/"+name] = body
		return http.StatusCreated, body
```

`toFloat` and `asList` already exist in the fake. GitHub ignores filters the fake does not implement (for example `state` or `labels` on the issues list), so `ghops` must filter on its side as well.

- [ ] **Step 2: Write the failing test `internal/ghops/write_test.go`**

```go
package ghops

import (
	"context"
	"slices"
	"testing"
)

const marker = "<!-- idp-plan -->"

func TestUpsertBotCommentCreatesThenUpdates(t *testing.T) {
	fake, repo := newRepo(t)
	ctx := context.Background()
	id, created, err := repo.UpsertBotComment(ctx, 7, marker, marker+"\nfirst")
	if err != nil || !created {
		t.Fatalf("first upsert: created=%v err=%v", created, err)
	}
	id2, created, err := repo.UpsertBotComment(ctx, 7, marker, marker+"\nsecond")
	if err != nil || created || id2 != id {
		t.Fatalf("second upsert: id=%d created=%v err=%v, want an update of %d", id2, created, err, id)
	}
	comments := fake.Lists["/repos/acme/idp-claims/issues/7/comments"]
	if len(comments) != 1 || comments[0].(map[string]any)["body"] != marker+"\nsecond" {
		t.Errorf("comments = %v, want one updated comment", comments)
	}
}

func TestUpsertBotCommentNeverEditsSomeoneElsesComment(t *testing.T) {
	fake, repo := newRepo(t)
	fake.Lists["/repos/acme/idp-claims/issues/7/comments"] = []any{
		map[string]any{"id": 1, "body": marker + " pasted by a human", "user": user("alice", "User")},
	}
	if _, created, err := repo.UpsertBotComment(context.Background(), 7, marker, marker+"\nplan"); err != nil || !created {
		t.Errorf("created=%v err=%v, want a new bot comment", created, err)
	}
	if slices.Contains(fake.Writes, "PATCH /repos/acme/idp-claims/issues/comments/1") {
		t.Error("edited a comment the bot did not write")
	}
}

func TestUpsertBotCommentRequiresTheMarker(t *testing.T) {
	_, repo := newRepo(t)
	if _, _, err := repo.UpsertBotComment(context.Background(), 7, marker, "no marker"); err == nil {
		t.Error("want an error for a body without the marker")
	}
}

func TestOpenIssueCreatesLabelAndIssue(t *testing.T) {
	fake, repo := newRepo(t)
	n, created, err := repo.OpenIssue(context.Background(), "drift", "Drift detected", "plan output")
	if err != nil || !created || n != 1 {
		t.Fatalf("n=%d created=%v err=%v", n, created, err)
	}
	if _, ok := fake.Objects["/repos/acme/idp-claims/labels/drift"]; !ok {
		t.Error("label drift was not created")
	}
}

func TestOpenIssueUpdatesExistingAcrossPages(t *testing.T) {
	fake, repo := newRepo(t)
	fake.MaxPerPage = 1
	fake.Objects["/repos/acme/idp-claims/labels/drift"] = map[string]any{"name": "drift"}
	issues := []any{}
	for i := 1; i <= 3; i++ {
		issues = append(issues, map[string]any{"number": i, "title": "other", "state": "open", "labels": []any{map[string]any{"name": "drift"}}})
	}
	issues = append(issues,
		map[string]any{"number": 4, "title": "Drift detected", "state": "open", "labels": []any{map[string]any{"name": "drift"}}, "pull_request": map[string]any{}},
		map[string]any{"number": 5, "title": "Drift detected", "state": "open", "labels": []any{map[string]any{"name": "drift"}}},
	)
	fake.Lists["/repos/acme/idp-claims/issues"] = issues
	n, created, err := repo.OpenIssue(context.Background(), "drift", "Drift detected", "new plan")
	if err != nil || created || n != 5 {
		t.Fatalf("n=%d created=%v err=%v, want an update of #5 (not the PR #4)", n, created, err)
	}
	if slices.Contains(fake.Writes, "POST /repos/acme/idp-claims/issues") {
		t.Error("created a duplicate issue")
	}
}

func TestCloseIssue(t *testing.T) {
	fake, repo := newRepo(t)
	fake.Lists["/repos/acme/idp-claims/issues"] = []any{
		map[string]any{"number": 9, "title": "Drift detected", "state": "open", "labels": []any{map[string]any{"name": "drift"}}},
	}
	n, closed, err := repo.CloseIssue(context.Background(), "drift", "Drift detected", "No drift on the last run.")
	if err != nil || !closed || n != 9 {
		t.Fatalf("n=%d closed=%v err=%v", n, closed, err)
	}
	if state := fake.Lists["/repos/acme/idp-claims/issues"][0].(map[string]any)["state"]; state != "closed" {
		t.Errorf("state = %v, want closed", state)
	}
	if len(fake.Lists["/repos/acme/idp-claims/issues/9/comments"]) != 1 {
		t.Error("closing comment missing")
	}
	if _, closed, err := repo.CloseIssue(context.Background(), "drift", "Drift detected", ""); err != nil || closed {
		t.Errorf("second close: closed=%v err=%v, want nothing to close", closed, err)
	}
}
```

- [ ] **Step 3: Run it and see it fail**

Run: `go test ./internal/ghops/`
Expected: FAIL to compile, with `undefined: repo.UpsertBotComment`.

- [ ] **Step 4: Implement `internal/ghops/write.go`**

```go
package ghops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// UpsertBotComment edits the Actions bot's comment that contains marker, or
// creates one, so a pull request keeps a single plan comment (spec §6.1).
func (r Repo) UpsertBotComment(ctx context.Context, number int, marker, body string) (int64, bool, error) {
	if !strings.Contains(body, marker) {
		return 0, false, fmt.Errorf("comment body does not contain marker %q", marker)
	}
	existing, err := r.FindBotComment(ctx, number, marker)
	if err != nil {
		return 0, false, err
	}
	payload := map[string]string{"body": body}
	if existing != nil {
		if err := r.API.Patch(ctx, fmt.Sprintf("%s/issues/comments/%d", r.base(), existing.ID), payload, nil); err != nil {
			return 0, false, err
		}
		return existing.ID, false, nil
	}
	var created Comment
	if err := r.API.Post(ctx, fmt.Sprintf("%s/issues/%d/comments", r.base(), number), payload, &created); err != nil {
		return 0, false, err
	}
	return created.ID, true, nil
}

// Issue is the part of an issue the pipelines need. The issues API also
// returns pull requests; those carry pull_request.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest *json.RawMessage `json:"pull_request"`
}

func (i Issue) hasLabel(label string) bool {
	for _, l := range i.Labels {
		if l.Name == label {
			return true
		}
	}
	return false
}

// FindOpenIssue returns the open issue (never a pull request) with label and
// title, or nil.
func (r Repo) FindOpenIssue(ctx context.Context, label, title string) (*Issue, error) {
	issues, err := ghapi.List[Issue](ctx, r.API, r.base()+"/issues?state=open&labels="+url.QueryEscape(label))
	if err != nil {
		return nil, err
	}
	for i := range issues {
		is := issues[i]
		if is.PullRequest == nil && is.State == "open" && is.Title == title && is.hasLabel(label) {
			return &is, nil
		}
	}
	return nil, nil
}

// EnsureLabel creates label unless the repository already has it.
func (r Repo) EnsureLabel(ctx context.Context, label, color, description string) error {
	err := r.API.Get(ctx, r.base()+"/labels/"+url.PathEscape(label), nil)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ghapi.ErrNotFound) {
		return err
	}
	return r.API.Post(ctx, r.base()+"/labels", map[string]string{"name": label, "color": color, "description": description}, nil)
}

// OpenIssue keeps one open issue per label and title (spec §6.4): it replaces
// the body of the existing one, or creates it.
func (r Repo) OpenIssue(ctx context.Context, label, title, body string) (int, bool, error) {
	if err := r.EnsureLabel(ctx, label, "d93f0b", "Opened by the IDP pipelines"); err != nil {
		return 0, false, err
	}
	existing, err := r.FindOpenIssue(ctx, label, title)
	if err != nil {
		return 0, false, err
	}
	if existing != nil {
		if err := r.API.Patch(ctx, fmt.Sprintf("%s/issues/%d", r.base(), existing.Number), map[string]string{"body": body}, nil); err != nil {
			return 0, false, err
		}
		return existing.Number, false, nil
	}
	var created Issue
	if err := r.API.Post(ctx, r.base()+"/issues", map[string]any{"title": title, "body": body, "labels": []string{label}}, &created); err != nil {
		return 0, false, err
	}
	return created.Number, true, nil
}

// CloseIssue closes the open issue with label and title, after adding comment
// (when not empty). It reports false when there was nothing to close.
func (r Repo) CloseIssue(ctx context.Context, label, title, comment string) (int, bool, error) {
	existing, err := r.FindOpenIssue(ctx, label, title)
	if err != nil || existing == nil {
		return 0, false, err
	}
	if comment != "" {
		if err := r.API.Post(ctx, fmt.Sprintf("%s/issues/%d/comments", r.base(), existing.Number), map[string]string{"body": comment}, nil); err != nil {
			return 0, false, err
		}
	}
	if err := r.API.Patch(ctx, fmt.Sprintf("%s/issues/%d", r.base(), existing.Number), map[string]string{"state": "closed", "state_reason": "completed"}, nil); err != nil {
		return 0, false, err
	}
	return existing.Number, true, nil
}
```

- [ ] **Step 5: Run the ghops tests and see them pass**

Run: `go test ./internal/ghops/`
Expected: `ok`.

- [ ] **Step 6: Write the failing CLI test `internal/cli/conversations_test.go`**

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
)

func ghEnv(fake *fakegithub.Server) Env {
	return envOf(map[string]string{"GH_TOKEN": "test-token", "IDP_GITHUB_API": fake.URL})
}

func TestCommentCommand(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	body := filepath.Join(t.TempDir(), "comment.md")
	if err := os.WriteFile(body, []byte("<!-- idp-plan -->\nplan"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"comment", "--repo", "acme/idp-claims", "--pr", "7", "--body-file", body}
	for i, want := range []string{"comment: created", "comment: updated"} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, ghEnv(fake)); code != 0 || !strings.Contains(stdout.String(), want) {
			t.Errorf("run %d: exit %d, stdout %q, stderr %q", i, code, stdout.String(), stderr.String())
		}
	}
}

func TestIssueOpenAndClose(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	body := filepath.Join(t.TempDir(), "issue.md")
	if err := os.WriteFile(body, []byte("drift details"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		args []string
		want string
	}{
		{[]string{"issue", "open", "--repo", "acme/idp-claims", "--label", "drift", "--title", "Drift detected", "--body-file", body}, "issue: opened #1"},
		{[]string{"issue", "open", "--repo", "acme/idp-claims", "--label", "drift", "--title", "Drift detected", "--body-file", body}, "issue: updated #1"},
		{[]string{"issue", "close", "--repo", "acme/idp-claims", "--label", "drift", "--title", "Drift detected", "--comment", "No drift."}, "issue: closed #1"},
		{[]string{"issue", "close", "--repo", "acme/idp-claims", "--label", "drift", "--title", "Drift detected"}, "issue: none open"},
	}
	for _, s := range steps {
		var stdout, stderr bytes.Buffer
		if code := Run(s.args, &stdout, &stderr, ghEnv(fake)); code != 0 || !strings.Contains(stdout.String(), s.want) {
			t.Errorf("%v: exit %d, stdout %q, stderr %q, want %q", s.args, code, stdout.String(), stderr.String(), s.want)
		}
	}
}

func TestConversationUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"comment", "--repo", "acme/idp-claims", "--pr", "x", "--body-file", "f"},
		{"comment", "--repo", "acme/idp-claims", "--pr", "7"},
		{"issue"},
		{"issue", "reopen"},
		{"issue", "open", "--repo", "acme/idp-claims", "--label", "drift", "--title", "T"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, envOf(map[string]string{"GH_TOKEN": "t"})); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
```

- [ ] **Step 7: Run it and see it fail**

Run: `go test ./internal/cli/ -run "Comment|Issue|Conversation"`
Expected: FAIL, because these are unknown commands.

- [ ] **Step 8: Implement `internal/cli/conversations.go` and wire it in**

```go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
	"github.com/jellalshadows-idp/idp-engine/internal/ghops"
	"github.com/jellalshadows-idp/idp-engine/internal/plan"
)

const issueUsage = `Usage:
  idp issue open  --repo OWNER/REPO --label LABEL --title TITLE --body-file FILE
  idp issue close --repo OWNER/REPO --label LABEL --title TITLE [--comment TEXT]
`

// githubRepo builds the repository client every conversation command needs,
// or returns a usage message.
func githubRepo(full string, env Env) (ghops.Repo, string) {
	tok := token(env)
	if tok == "" {
		return ghops.Repo{}, "set GH_TOKEN or GITHUB_TOKEN"
	}
	repo, err := ghops.ParseRepo(ghapi.New(apiBase(env), tok), full)
	if err != nil {
		return ghops.Repo{}, err.Error()
	}
	return repo, ""
}

func runComment(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp comment", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoFlag := fs.String("repo", "", "repository, OWNER/REPO")
	pr := fs.Int("pr", 0, "pull request number")
	bodyFile := fs.String("body-file", "", "comment Markdown; must contain the marker")
	marker := fs.String("marker", plan.CommentMarker, "marker that identifies the sticky comment")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 || *repoFlag == "" || *pr <= 0 || *bodyFile == "" {
		fmt.Fprintln(stderr, "idp comment: --repo, --pr and --body-file are required")
		return exitUsage
	}
	repo, msg := githubRepo(*repoFlag, env)
	if msg != "" {
		fmt.Fprintln(stderr, "idp comment:", msg)
		return exitUsage
	}
	body, err := os.ReadFile(*bodyFile)
	if err != nil {
		return fail(stderr, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	id, created, err := repo.UpsertBotComment(ctx, *pr, *marker, string(body))
	if err != nil {
		return fail(stderr, err)
	}
	verb := "updated"
	if created {
		verb = "created"
	}
	fmt.Fprintf(stdout, "comment: %s %d on #%d\n", verb, id, *pr)
	return exitOK
}

func runIssue(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 || (args[0] != "open" && args[0] != "close") {
		fmt.Fprint(stderr, issueUsage)
		return exitUsage
	}
	mode := args[0]
	fs := flag.NewFlagSet("idp issue "+mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoFlag := fs.String("repo", "", "repository, OWNER/REPO")
	label := fs.String("label", "", "label that identifies the issue")
	title := fs.String("title", "", "issue title")
	bodyFile := fs.String("body-file", "", "issue body (open only)")
	comment := fs.String("comment", "", "comment to leave when closing (close only)")
	if err := fs.Parse(args[1:]); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 || *repoFlag == "" || *label == "" || *title == "" || (mode == "open" && *bodyFile == "") {
		fmt.Fprint(stderr, issueUsage)
		return exitUsage
	}
	repo, msg := githubRepo(*repoFlag, env)
	if msg != "" {
		fmt.Fprintln(stderr, "idp issue:", msg)
		return exitUsage
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if mode == "close" {
		n, closed, err := repo.CloseIssue(ctx, *label, *title, *comment)
		if err != nil {
			return fail(stderr, err)
		}
		if closed {
			fmt.Fprintf(stdout, "issue: closed #%d\n", n)
		} else {
			fmt.Fprintln(stdout, "issue: none open")
		}
		return exitOK
	}
	body, err := os.ReadFile(*bodyFile)
	if err != nil {
		return fail(stderr, err)
	}
	n, created, err := repo.OpenIssue(ctx, *label, *title, string(body))
	if err != nil {
		return fail(stderr, err)
	}
	verb := "updated"
	if created {
		verb = "opened"
	}
	fmt.Fprintf(stdout, "issue: %s #%d\n", verb, n)
	return exitOK
}
```

In `internal/cli/cli.go`, add after `gate`:

```
  comment         Create or update the sticky plan comment on a pull request
  issue           Open or close a labelled issue (drift, failed wet pushes)
```

Then add the dispatch cases `case "comment": return runComment(args[1:], stdout, stderr, env)` and `case "issue": return runIssue(args[1:], stdout, stderr, env)`.

- [ ] **Step 9: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok`.

- [ ] **Step 10: Commit**

```bash
git add internal/ghops internal/fakegithub internal/cli
git commit -m "feat(ghops): keep one sticky plan comment and one labelled issue"
```

---

### Task 9: `internal/wetpush` and `idp wet-push`

**Files:**
- Create: `internal/wetpush/wetpush.go`, `internal/wetpush/wetpush_test.go`, `internal/cli/wetpush.go`, `internal/cli/wetpush_test.go`
- Modify: `internal/fakegithub/fakegithub.go`, `internal/cli/cli.go`

**Interfaces:**
- Consumes: `ghapi.Client`, `ghops.ParseRepo` (for flag parsing), `actions.SetOutput`, and the fake's `Requests`.
- Produces (package `internal/wetpush`):
  - `type Pusher struct { API *ghapi.Client; Owner, Repo, Branch string }`;
  - `type Result struct { Commit string; Changed, Deleted int }`;
  - `func (p Pusher) Sync(ctx context.Context, root string, paths []string, message string) (Result, error)`.
- Produces (CLI): `idp wet-push --repo OWNER/REPO [--branch wet] --root DIR --path P [--path P ...] --message MSG`. It writes the step output `commit=<sha>`, which is empty when nothing changed.

- [ ] **Step 1: Add the blob and ref-update routes to the fake**

In `internal/fakegithub/fakegithub.go`:

1. Add the field and the regexp:

```go
	// RejectRefUpdates lists ref paths whose PATCH answers 422 (not a fast forward).
	RejectRefUpdates map[string]bool
```

```go
	reRefUpdate = regexp.MustCompile(`^(/repos/[^/]+/[^/]+)/git/refs/(heads/.+)$`)
```

2. Initialise `RejectRefUpdates: map[string]bool{}` in `New`.
3. Add these cases to `route`, before the generic GET case:

```go
	case method == http.MethodPost && strings.HasSuffix(path, "/git/blobs"):
		f.nextID++
		sha := "blob-" + strconv.FormatInt(f.nextID, 10)
		f.Objects[path+"/"+sha] = body
		return http.StatusCreated, map[string]any{"sha": sha}
	case method == http.MethodPatch && reRefUpdate.MatchString(path):
		if f.RejectRefUpdates[path] {
			return http.StatusUnprocessableEntity, map[string]any{"message": "Update is not a fast forward"}
		}
		m := reRefUpdate.FindStringSubmatch(path)
		f.Objects[m[1]+"/git/ref/"+m[2]] = map[string]any{"ref": "refs/" + m[2], "object": map[string]any{"sha": body["sha"]}}
		return http.StatusOK, f.Objects[m[1]+"/git/ref/"+m[2]]
```

The existing `POST …/git/trees` route answers `tree-sha` and `POST …/git/commits` answers `commit-sha`; both stay as they are.

- [ ] **Step 2: Write the failing test `internal/wetpush/wetpush_test.go`**

```go
package wetpush

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

const repoPath = "/repos/acme/idp-claims"

func setup(t *testing.T, remote map[string]string) (*fakegithub.Server, Pusher) {
	t.Helper()
	fake := fakegithub.New(t, "acme")
	fake.Objects[repoPath+"/git/ref/heads/wet"] = map[string]any{"ref": "refs/heads/wet", "object": map[string]any{"sha": "head-sha"}}
	fake.Objects[repoPath+"/git/commits/head-sha"] = map[string]any{"sha": "head-sha", "tree": map[string]any{"sha": "base-tree"}}
	entries := []any{}
	for p, content := range remote {
		entries = append(entries, map[string]any{"path": p, "mode": "100644", "type": "blob", "sha": blobSHA([]byte(content))})
	}
	fake.Objects[repoPath+"/git/trees/base-tree"] = map[string]any{"sha": "base-tree", "truncated": false, "tree": entries}
	return fake, Pusher{API: ghapi.New(fake.URL, "writer-token"), Owner: "acme", Repo: "idp-claims", Branch: "wet"}
}

func writeLocal(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func lastRequest(t *testing.T, fake *fakegithub.Server, method, suffix string) map[string]any {
	t.Helper()
	for i := len(fake.Requests) - 1; i >= 0; i-- {
		if r := fake.Requests[i]; r.Method == method && strings.HasSuffix(r.Path, suffix) {
			return r.Body
		}
	}
	t.Fatalf("no %s ...%s request in %v", method, suffix, fake.Writes)
	return nil
}

func TestSyncCommitsChangesAndDeletionsOnly(t *testing.T) {
	fake, p := setup(t, map[string]string{
		"README.md":                    "# wet",
		"tfstate/github.tfstate":       "old state",
		"rendered/github/main.tf.json": "{}",
		"rendered/github/old.tf.json":  "gone",
	})
	root := writeLocal(t, map[string]string{
		"tfstate/github.tfstate":               "new state",
		"rendered/github/main.tf.json":         "{}",
		"rendered/github/.terraform.lock.hcl":  "lock",
		"rendered/github/.terraform/providers": "binary",
	})
	res, err := p.Sync(context.Background(), root, []string{"tfstate/github.tfstate", "rendered/github"}, "reconcile: abc")
	if err != nil {
		t.Fatal(err)
	}
	if res != (Result{Commit: "commit-sha", Changed: 2, Deleted: 1}) {
		t.Errorf("result = %+v", res)
	}
	tree := lastRequest(t, fake, "POST", "/git/trees")
	if tree["base_tree"] != "base-tree" {
		t.Errorf("base_tree = %v", tree["base_tree"])
	}
	got := map[string]any{}
	for _, e := range tree["tree"].([]any) {
		entry := e.(map[string]any)
		got[entry["path"].(string)] = entry["sha"]
	}
	if len(got) != 3 || got["rendered/github/old.tf.json"] != nil {
		t.Errorf("tree entries = %v, want 2 uploads and a null-sha deletion", got)
	}
	for _, p := range []string{"tfstate/github.tfstate", "rendered/github/.terraform.lock.hcl"} {
		if sha, _ := got[p].(string); !strings.HasPrefix(sha, "blob-") {
			t.Errorf("%s sha = %v, want an uploaded blob", p, got[p])
		}
	}
	commit := lastRequest(t, fake, "POST", "/git/commits")
	if commit["message"] != "reconcile: abc" || commit["tree"] != "tree-sha" || !reflect.DeepEqual(commit["parents"], []any{"head-sha"}) {
		t.Errorf("commit = %v", commit)
	}
	ref := lastRequest(t, fake, "PATCH", "/git/refs/heads/wet")
	if ref["sha"] != "commit-sha" || ref["force"] != false {
		t.Errorf("ref update = %v, want commit-sha without force", ref)
	}
}

func TestSyncNoChangesMakesNoCommit(t *testing.T) {
	fake, p := setup(t, map[string]string{"tfstate/github.tfstate": "same"})
	root := writeLocal(t, map[string]string{"tfstate/github.tfstate": "same"})
	res, err := p.Sync(context.Background(), root, []string{"tfstate/github.tfstate"}, "m")
	if err != nil {
		t.Fatal(err)
	}
	if res != (Result{}) || len(fake.Writes) != 0 {
		t.Errorf("result = %+v, writes = %v, want nothing", res, fake.Writes)
	}
}

func TestSyncCreatesFilesUnderNewPrefix(t *testing.T) {
	fake, p := setup(t, map[string]string{"README.md": "# wet"})
	root := writeLocal(t, map[string]string{"tfstate/github.tfstate": "state", "rendered/github/main.tf.json": "{}"})
	res, err := p.Sync(context.Background(), root, []string{"tfstate/github.tfstate", "rendered/github"}, "first reconcile")
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed != 2 || res.Deleted != 0 || res.Commit != "commit-sha" {
		t.Errorf("result = %+v", res)
	}
	if n := len(lastRequest(t, fake, "POST", "/git/trees")["tree"].([]any)); n != 2 {
		t.Errorf("tree entries = %d, want 2", n)
	}
}

func TestSyncDeletesAPrefixRemovedLocally(t *testing.T) {
	_, p := setup(t, map[string]string{"rendered/aws/dev/_baseline/main.tf.json": "{}"})
	res, err := p.Sync(context.Background(), t.TempDir(), []string{"rendered/aws/dev/_baseline"}, "orphan destroyed")
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted != 1 || res.Changed != 0 {
		t.Errorf("result = %+v", res)
	}
}

func TestSyncRefusesNonFastForward(t *testing.T) {
	fake, p := setup(t, nil)
	fake.RejectRefUpdates[repoPath+"/git/refs/heads/wet"] = true
	root := writeLocal(t, map[string]string{"tfstate/github.tfstate": "state"})
	_, err := p.Sync(context.Background(), root, []string{"tfstate/github.tfstate"}, "m")
	var apiErr *ghapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 422 {
		t.Errorf("err = %v, want the 422 from the ref update", err)
	}
}

func TestSyncRejectsTruncatedTrees(t *testing.T) {
	fake, p := setup(t, nil)
	fake.Objects[repoPath+"/git/trees/base-tree"] = map[string]any{"sha": "base-tree", "truncated": true, "tree": []any{}}
	if _, err := p.Sync(context.Background(), t.TempDir(), []string{"tfstate/github.tfstate"}, "m"); err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Errorf("err = %v, want a truncated-tree error", err)
	}
}

func TestSyncValidatesPaths(t *testing.T) {
	_, p := setup(t, nil)
	for _, bad := range [][]string{nil, {""}, {"../x"}, {"/abs"}, {"a/../b"}, {"a\\b"}} {
		if _, err := p.Sync(context.Background(), t.TempDir(), bad, "m"); err == nil {
			t.Errorf("paths %q: want an error", bad)
		}
	}
}

func TestBlobSHAMatchesGit(t *testing.T) {
	// echo -n "hello" | git hash-object --stdin
	if got := blobSHA([]byte("hello")); got != "b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0" {
		t.Errorf("blobSHA = %s", got)
	}
}
```

- [ ] **Step 3: Run it and see it fail**

Run: `go test ./internal/wetpush/`
Expected: FAIL to compile, with `undefined: Pusher`.

- [ ] **Step 4: Implement `internal/wetpush/wetpush.go`**

```go
// Package wetpush makes the one commit to the wet branch that ends a reconcile
// (spec §6.2 step 4.3), through the Git Data API (ADR-0019).
package wetpush

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// Pusher commits to one branch of one repository.
type Pusher struct {
	API    *ghapi.Client
	Owner  string
	Repo   string
	Branch string
}

// Result reports the commit made; Commit is empty when nothing changed.
type Result struct {
	Commit  string
	Changed int // files added or modified
	Deleted int
}

type treeEntry struct {
	Path string  `json:"path"`
	Mode string  `json:"mode"`
	Type string  `json:"type"`
	SHA  *string `json:"sha"` // null deletes the path
}

// Sync makes the branch's files under each path equal to the files under
// root/path, in one commit on top of the current head, and updates the
// branch without force. A path is a file or a directory; .terraform
// directories are never synced. Nothing changed means no commit.
func (p Pusher) Sync(ctx context.Context, root string, paths []string, message string) (Result, error) {
	if err := validatePaths(paths); err != nil {
		return Result{}, err
	}
	base := "/repos/" + p.Owner + "/" + p.Repo
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := p.API.Get(ctx, base+"/git/ref/heads/"+p.Branch, &ref); err != nil {
		return Result{}, fmt.Errorf("read branch %s: %w", p.Branch, err)
	}
	head := ref.Object.SHA
	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := p.API.Get(ctx, base+"/git/commits/"+head, &commit); err != nil {
		return Result{}, err
	}
	var tree struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"tree"`
	}
	if err := p.API.Get(ctx, base+"/git/trees/"+commit.Tree.SHA+"?recursive=1", &tree); err != nil {
		return Result{}, err
	}
	if tree.Truncated {
		return Result{}, errors.New("the wet tree listing is truncated; refusing to compute deletions from a partial listing")
	}
	remote := map[string]string{} // path -> blob sha, only under the synced paths
	for _, e := range tree.Tree {
		if e.Type == "blob" && underAny(e.Path, paths) {
			remote[e.Path] = e.SHA
		}
	}
	local, err := readLocal(root, paths)
	if err != nil {
		return Result{}, err
	}
	var entries []treeEntry
	res := Result{}
	for _, rel := range sortedKeys(local) {
		data := local[rel]
		if remote[rel] == blobSHA(data) {
			continue
		}
		var blob struct {
			SHA string `json:"sha"`
		}
		if err := p.API.Post(ctx, base+"/git/blobs", map[string]string{"content": base64.StdEncoding.EncodeToString(data), "encoding": "base64"}, &blob); err != nil {
			return Result{}, err
		}
		sha := blob.SHA
		entries = append(entries, treeEntry{Path: rel, Mode: "100644", Type: "blob", SHA: &sha})
		res.Changed++
	}
	for _, rel := range sortedKeys(remote) {
		if _, ok := local[rel]; !ok {
			entries = append(entries, treeEntry{Path: rel, Mode: "100644", Type: "blob", SHA: nil})
			res.Deleted++
		}
	}
	if len(entries) == 0 {
		return Result{}, nil
	}
	var newTree, newCommit struct {
		SHA string `json:"sha"`
	}
	if err := p.API.Post(ctx, base+"/git/trees", map[string]any{"base_tree": commit.Tree.SHA, "tree": entries}, &newTree); err != nil {
		return Result{}, err
	}
	if err := p.API.Post(ctx, base+"/git/commits", map[string]any{"message": message, "tree": newTree.SHA, "parents": []string{head}}, &newCommit); err != nil {
		return Result{}, err
	}
	if err := p.API.Patch(ctx, base+"/git/refs/heads/"+p.Branch, map[string]any{"sha": newCommit.SHA, "force": false}, nil); err != nil {
		return Result{}, fmt.Errorf("update branch %s: %w", p.Branch, err)
	}
	res.Commit = newCommit.SHA
	return res, nil
}

func validatePaths(paths []string) error {
	if len(paths) == 0 {
		return errors.New("no paths to sync")
	}
	for _, p := range paths {
		if p == "" || strings.Contains(p, `\`) || path.IsAbs(p) || path.Clean(p) != p || p == "." || strings.HasPrefix(p, "../") || p == ".." {
			return fmt.Errorf("path %q: want a clean relative slash path", p)
		}
	}
	return nil
}

func underAny(rel string, paths []string) bool {
	for _, p := range paths {
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

// readLocal reads the regular files under root/path for each path, keyed by
// slash path relative to root. A missing path has no files (it was removed).
func readLocal(root string, paths []string) (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, p := range paths {
		abs := filepath.Join(root, filepath.FromSlash(p))
		info, err := os.Lstat(abs)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(abs)
			if err != nil {
				return nil, err
			}
			files[p] = data
			continue
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%s: not a regular file or directory", p)
		}
		err = filepath.WalkDir(abs, func(walked string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".terraform" {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return fmt.Errorf("%s: not a regular file", walked)
			}
			rel, err := filepath.Rel(root, walked)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(walked)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(rel)] = data
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// blobSHA is git's object id for a blob with this content.
func blobSHA(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
```

- [ ] **Step 5: Run the wetpush tests and see them pass**

Run: `go test ./internal/wetpush/`
Expected: `ok`.

- [ ] **Step 6: Write the failing CLI test `internal/cli/wetpush_test.go`**

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/fakegithub"
)

func TestWetPushCommand(t *testing.T) {
	fake := fakegithub.New(t, "acme")
	fake.Objects["/repos/acme/idp-claims/git/ref/heads/wet"] = map[string]any{"object": map[string]any{"sha": "head-sha"}}
	fake.Objects["/repos/acme/idp-claims/git/commits/head-sha"] = map[string]any{"tree": map[string]any{"sha": "base-tree"}}
	fake.Objects["/repos/acme/idp-claims/git/trees/base-tree"] = map[string]any{"truncated": false, "tree": []any{}}
	root := t.TempDir()
	writeFileAll(t, filepath.Join(root, "tfstate", "github.tfstate"), "state")
	out := filepath.Join(t.TempDir(), "out")
	env := envOf(map[string]string{"GH_TOKEN": "writer-token", "IDP_GITHUB_API": fake.URL, "GITHUB_OUTPUT": out})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"wet-push", "--repo", "acme/idp-claims", "--root", root, "--path", "tfstate/github.tfstate", "--message", "reconcile"}, &stdout, &stderr, env)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wet-push: commit-sha (1 changed, 0 deleted)") {
		t.Errorf("stdout = %q", stdout.String())
	}
	if got, _ := os.ReadFile(out); string(got) != "commit=commit-sha\n" {
		t.Errorf("GITHUB_OUTPUT = %q", got)
	}
}

func TestWetPushUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"wet-push"},
		{"wet-push", "--repo", "acme/idp-claims", "--root", "r", "--message", "m"},
		{"wet-push", "--repo", "acme", "--root", "r", "--path", "p", "--message", "m"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, envOf(map[string]string{"GH_TOKEN": "t"})); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
```

- [ ] **Step 7: Run it and see it fail**

Run: `go test ./internal/cli/ -run WetPush`
Expected: FAIL, because `wet-push` is an unknown command.

- [ ] **Step 8: Implement `internal/cli/wetpush.go` and wire it in**

```go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/wetpush"
)

func runWetPush(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp wet-push", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoFlag := fs.String("repo", "", "claims repository, OWNER/REPO")
	branch := fs.String("branch", "wet", "branch to commit to")
	root := fs.String("root", "", "local directory laid out like the branch")
	message := fs.String("message", "", "commit message")
	var paths []string
	fs.Func("path", "slash path under --root to sync, file or directory (repeatable)", func(v string) error {
		if v == "" {
			return fmt.Errorf("empty --path")
		}
		paths = append(paths, v)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 || *repoFlag == "" || *root == "" || *message == "" || len(paths) == 0 {
		fmt.Fprintln(stderr, "idp wet-push: --repo, --root, --message and at least one --path are required")
		return exitUsage
	}
	repo, msg := githubRepo(*repoFlag, env)
	if msg != "" {
		fmt.Fprintln(stderr, "idp wet-push:", msg)
		return exitUsage
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	p := wetpush.Pusher{API: repo.API, Owner: repo.Owner, Repo: repo.Name, Branch: *branch}
	res, err := p.Sync(ctx, *root, paths, *message)
	if err != nil {
		return fail(stderr, err)
	}
	if res.Commit == "" {
		fmt.Fprintln(stdout, "wet-push: no changes")
	} else {
		fmt.Fprintf(stdout, "wet-push: %s (%d changed, %d deleted)\n", res.Commit, res.Changed, res.Deleted)
	}
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "commit", res.Commit); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}
```

In `internal/cli/cli.go`, add the usage line `  wet-push        Commit files to the wet branch in one commit` after `issue`, and the dispatch `case "wet-push": return runWetPush(args[1:], stdout, stderr, env)`.

- [ ] **Step 9: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok`.

- [ ] **Step 10: Commit**

```bash
git add internal/wetpush internal/fakegithub internal/cli
git commit -m "feat(wetpush): commit to the wet branch in one commit with idp wet-push"
```

---

### Task 10: `TF_ENCRYPTION` — `render.EncryptionConfig` and `idp encryption-env`

**Files:**
- Create: `internal/render/encryption.go`, `internal/render/encryption_test.go`, `internal/cli/encryption.go`, `internal/cli/encryption_test.go`
- Modify: `internal/bootstrap/config.go`, `internal/cli/cli.go`

**Interfaces:**
- Produces (package `internal/bootstrap`): `func ValidatePassphrase(p string) error`. `ReadPassphrase` keeps its exact error messages and now builds them on this function.
- Produces (package `internal/render`): `const KeyProviderName = "idp"` and `func EncryptionConfig(passphrase string) string`.
- Produces (CLI): `idp encryption-env`. It reads `IDP_STATE_PASSPHRASE` and appends `TF_ENCRYPTION` to `GITHUB_ENV`.
  - Missing or invalid passphrase → exit 1, with a message (and annotation) naming the secret.
  - No `GITHUB_ENV` → exit 2.

- [ ] **Step 1: Write the failing tests**

`internal/render/encryption_test.go`:
```go
package render

import (
	"strings"
	"testing"
)

func TestEncryptionConfig(t *testing.T) {
	got := EncryptionConfig("correct-horse-battery-staple")
	want := `key_provider "pbkdf2" "idp" {
  passphrase = "correct-horse-battery-staple"
}
method "aes_gcm" "idp" {
  keys = key_provider.pbkdf2.idp
}
state {
  method = method.aes_gcm.idp
}
plan {
  method = method.aes_gcm.idp
}
`
	if got != want {
		t.Errorf("EncryptionConfig =\n%s\nwant\n%s", got, want)
	}
}

func TestEncryptionConfigEscapesHCL(t *testing.T) {
	got := EncryptionConfig(`a"b\c${d}%{e}`)
	if !strings.Contains(got, `passphrase = "a\"b\\c$${d}%%{e}"`) {
		t.Errorf("passphrase line not escaped for HCL:\n%s", got)
	}
}
```

`internal/cli/encryption_test.go`:
```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptionEnvWritesTFEncryption(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "env")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"encryption-env"}, &stdout, &stderr, envOf(map[string]string{
		"GITHUB_ENV": envFile, "IDP_STATE_PASSPHRASE": "correct-horse-battery-staple",
	}))
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	got, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "TF_ENCRYPTION<<IDP_EOF_") || !strings.Contains(string(got), `passphrase = "correct-horse-battery-staple"`) {
		t.Errorf("GITHUB_ENV = %q", got)
	}
	if strings.Contains(stdout.String(), "correct-horse") || strings.Contains(stderr.String(), "correct-horse") {
		t.Error("the passphrase must never be printed")
	}
}

func TestEncryptionEnvNamesTheMissingSecret(t *testing.T) {
	for name, pass := range map[string]string{"empty": "", "short": "too-short", "two lines": "correct-horse-battery\nstaple"} {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"encryption-env"}, &stdout, &stderr, envOf(map[string]string{
			"GITHUB_ENV": filepath.Join(t.TempDir(), "env"), "IDP_STATE_PASSPHRASE": pass, "GITHUB_ACTIONS": "true",
		}))
		if code != 1 || !strings.Contains(stderr.String(), "IDP_STATE_PASSPHRASE") || !strings.Contains(stdout.String(), "::error title=State passphrase::") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, code, stdout.String(), stderr.String())
		}
	}
}

func TestEncryptionEnvNeedsGitHubEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"encryption-env"}, &stdout, &stderr, envOf(map[string]string{"IDP_STATE_PASSPHRASE": "correct-horse-battery-staple"})); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}
```

Append to `internal/bootstrap/config_test.go`:
```go
func TestValidatePassphrase(t *testing.T) {
	for p, ok := range map[string]bool{
		"correct-horse-battery-staple": true,
		"exactly-16-chars":             true,
		"fifteen-chars!!":              false,
		"":                             false,
		"line one is long\nline two":   false,
		"\xff\xfe-not-valid-utf8-bytes": false,
	} {
		if err := ValidatePassphrase(p); (err == nil) != ok {
			t.Errorf("ValidatePassphrase(%q) = %v, want ok=%v", p, err, ok)
		}
	}
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/render/ ./internal/cli/ ./internal/bootstrap/`
Expected: FAIL to compile, with `undefined: EncryptionConfig` and `undefined: ValidatePassphrase`.

- [ ] **Step 3: Implement**

In `internal/bootstrap/config.go`, extract the shared checks and keep `ReadPassphrase`'s messages:

```go
var (
	errPassphraseEncoding = errors.New("not valid UTF-8 or contains control characters (several lines?)")
	errPassphraseShort    = errors.New("shorter than 16 characters")
)

// ValidatePassphrase enforces the rules every state passphrase meets: valid
// UTF-8 on a single line, and OpenTofu's PBKDF2 minimum of 16 characters.
func ValidatePassphrase(p string) error {
	if !utf8.ValidString(p) || strings.ContainsFunc(p, unicode.IsControl) {
		return errPassphraseEncoding
	}
	if utf8.RuneCountInString(p) < 16 {
		return errPassphraseShort
	}
	return nil
}

// ReadPassphrase loads the state passphrase, trimming the line ending editors
// add (LF or CRLF), and validates it with ValidatePassphrase.
func ReadPassphrase(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	p := strings.TrimRight(strings.TrimPrefix(string(raw), "\xef\xbb\xbf"), "\r\n")
	switch err := ValidatePassphrase(p); {
	case errors.Is(err, errPassphraseEncoding):
		return "", fmt.Errorf("passphrase in %s is not valid UTF-8 or contains control characters (UTF-16 file? several lines?); save the file as UTF-8 without BOM, on a single line", path)
	case err != nil:
		return "", fmt.Errorf("passphrase in %s is shorter than 16 characters", path)
	}
	return p, nil
}
```

Add `"errors"` to the imports if it is not there yet.

`internal/render/encryption.go`:
```go
package render

import (
	"fmt"
	"strings"
)

// KeyProviderName names the PBKDF2 key provider and the AES-GCM method in
// TF_ENCRYPTION. OpenTofu records it in the encrypted state, so renaming it
// makes existing state unreadable without a fallback migration (ADR-0020).
const KeyProviderName = "idp"

var hclEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "${", "$${", "%{", "%%{")

// EncryptionConfig returns the TF_ENCRYPTION value for passphrase: the key
// material behind the enforced encryption the render emits (spec §7.2). The
// passphrase is escaped for an HCL quoted string, so it is used literally.
func EncryptionConfig(passphrase string) string {
	return fmt.Sprintf(`key_provider "pbkdf2" %[1]q {
  passphrase = "%[2]s"
}
method "aes_gcm" %[1]q {
  keys = key_provider.pbkdf2.%[1]s
}
state {
  method = method.aes_gcm.%[1]s
}
plan {
  method = method.aes_gcm.%[1]s
}
`, KeyProviderName, hclEscaper.Replace(passphrase))
}
```

`internal/cli/encryption.go`:
```go
package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/bootstrap"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
)

func runEncryptionEnv(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp encryption-env", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp encryption-env: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	envFile := env("GITHUB_ENV")
	if envFile == "" {
		fmt.Fprintln(stderr, "idp encryption-env: GITHUB_ENV is not set; run it as a GitHub Actions step")
		return exitUsage
	}
	pass := env("IDP_STATE_PASSPHRASE")
	if err := bootstrap.ValidatePassphrase(pass); err != nil {
		problem := fmt.Sprintf("is invalid (%v)", err)
		if pass == "" {
			problem = "is empty or missing"
		}
		msg := fmt.Sprintf("IDP_STATE_PASSPHRASE %s: pass the repo secret to the workflow as IDP_STATE_PASSPHRASE; see docs/runbooks/bootstrap.md", problem)
		if env("GITHUB_ACTIONS") == "true" {
			fmt.Fprintln(stdout, actions.ErrorAnnotation("", 0, "State passphrase", msg))
		}
		fmt.Fprintln(stderr, "idp:", msg)
		return exitError
	}
	if err := actions.SetEnv(envFile, "TF_ENCRYPTION", render.EncryptionConfig(pass)); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, "encryption-env: TF_ENCRYPTION is set for the next steps")
	return exitOK
}
```

In `internal/cli/cli.go`, add the usage line `  encryption-env  Export TF_ENCRYPTION for OpenTofu from IDP_STATE_PASSPHRASE` after `wet-push`, and the dispatch `case "encryption-env": return runEncryptionEnv(args[1:], stdout, stderr, env)`.

- [ ] **Step 4: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok`. Every pre-existing `ReadPassphrase` test keeps its expected messages.

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap internal/render internal/cli
git commit -m "feat(cli): export tf_encryption from the state passphrase with idp encryption-env"
```

---

### Task 11: Drift-ready `bootstrap check` — `IDP_BOOTSTRAP`, `--params-env`, `--allow-hidden-bypass`

**Files:**
- Modify: `internal/bootstrap/desired.go`, `internal/bootstrap/config.go`, `internal/bootstrap/bootstrapper.go`, `internal/bootstrap/check.go`, `internal/cli/bootstrap.go`, `internal/bootstrap/*_test.go`, `internal/cli/bootstrap_test.go`, `docs/runbooks/bootstrap.md`

**Interfaces:**
- Produces (package `internal/bootstrap`):
  - `const VarParams = "IDP_BOOTSTRAP"`;
  - `type AppIdentity struct { ID int64; ClientID, Slug string }`, with JSON tags `id`, `clientId`, `slug`;
  - `type Params struct { ApproverID int64; Reader, Writer AppIdentity }`, with JSON tags `approverId`, `reader`, `writer`;
  - `func (c Config) Params() Params`, `func (p Params) String() string`, `func ParseParams(s string) (Params, error)` and `func (p Params) Config(org, repo string) Config`;
  - the field `Bootstrapper.AllowHiddenBypass bool` and the field `Finding.Notice bool`.
- Produces (CLI): `idp bootstrap check --org ORG --claims-repo REPO (--approver LOGIN --reader FILE --writer FILE | --params-env NAME) [--allow-hidden-bypass]`. Notices are printed with the prefix `notice: ` and do not count as findings.

- [ ] **Step 1: Write the failing tests**

Append to `internal/bootstrap/config_test.go`:
```go
func TestParamsRoundTrip(t *testing.T) {
	p := validConfig().Params()
	want := `{"approverId":42,"reader":{"id":1,"clientId":"Iv-reader","slug":"acme-reader"},"writer":{"id":2,"clientId":"Iv-writer","slug":"acme-writer"}}`
	if p.String() != want {
		t.Errorf("String = %s, want %s", p.String(), want)
	}
	back, err := ParseParams(p.String())
	if err != nil {
		t.Fatal(err)
	}
	cfg := back.Config("acme", "idp-claims")
	if err := cfg.ValidateCheck(); err != nil {
		t.Errorf("config from params does not validate for check: %v", err)
	}
	if cfg.Writer.ID != 2 || cfg.Reader.Slug != "acme-reader" || cfg.ApproverID != 42 || cfg.Writer.PrivateKey != "" {
		t.Errorf("config = %+v", cfg)
	}
	for _, bad := range []string{"", "{", `{"approverId":1,"extra":true}`} {
		if _, err := ParseParams(bad); err == nil {
			t.Errorf("ParseParams(%q) succeeded, want an error", bad)
		}
	}
}
```

Append to `internal/bootstrap/drift_test.go`:
```go
func TestCheckAllowHiddenBypassTurnsItIntoNotices(t *testing.T) {
	fake, api := newFake(t)
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	fake.Objects["/orgs/acme/installations"] = map[string]any{"total_count": 2, "installations": []any{
		map[string]any{"app_slug": "acme-reader"}, map[string]any{"app_slug": "acme-writer"},
	}}
	for _, name := range []string{"idp-main", "idp-wet"} {
		delete(fake.RulesetByName(name), "bypass_actors")
	}

	strict, err := b.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(strict) != 2 || strict[0].Notice {
		t.Fatalf("strict findings = %v, want 2 drift findings", strict)
	}

	b.AllowHiddenBypass = true
	lenient, err := b.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lenient) != 2 || !lenient[0].Notice || !lenient[1].Notice {
		t.Fatalf("lenient findings = %v, want 2 notices", lenient)
	}
	if !strings.HasPrefix(lenient[0].String(), "notice: ruleset idp-main: bypass actors not verifiable") {
		t.Errorf("notice = %q", lenient[0].String())
	}
}

func TestApplyRecordsTheBootstrapParams(t *testing.T) {
	fake, api := newFake(t)
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	v, _ := fake.Objects["/repos/acme/idp-claims/actions/variables/IDP_BOOTSTRAP"].(map[string]any)
	if v["value"] != validConfig().Params().String() {
		t.Errorf("IDP_BOOTSTRAP = %v", v["value"])
	}
}
```

Add `"strings"` to `drift_test.go`'s imports.

Append to `internal/cli/bootstrap_test.go`:
```go
func TestCheckFromParamsEnvWithHiddenBypass(t *testing.T) {
	f := newE2E(t)
	if code, _, stderr := f.run(f.env(nil), "apply", "--passphrase-file", f.pass); code != 0 {
		t.Fatalf("apply exit %d (stderr %q)", code, stderr)
	}
	f.fake.Objects["/orgs/acme/installations"] = map[string]any{"total_count": 2, "installations": []any{
		map[string]any{"app_slug": "acme-reader"}, map[string]any{"app_slug": "acme-writer"},
	}}
	for _, name := range []string{"idp-main", "idp-wet"} {
		delete(f.fake.RulesetByName(name), "bypass_actors")
	}
	params := f.fake.Objects["/repos/acme/idp-claims/actions/variables/IDP_BOOTSTRAP"].(map[string]any)["value"].(string)
	env := f.env(map[string]string{"IDP_BOOTSTRAP": params})
	base := []string{"bootstrap", "check", "--org", "acme", "--claims-repo", "idp-claims", "--params-env", "IDP_BOOTSTRAP"}

	var stdout, stderr bytes.Buffer
	if code := Run(base, &stdout, &stderr, env); code != 3 {
		t.Errorf("strict check exit %d, want 3 (stdout %q, stderr %q)", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(append(base, "--allow-hidden-bypass"), &stdout, &stderr, env); code != 0 {
		t.Fatalf("lenient check exit %d, want 0 (stdout %q, stderr %q)", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"notice: ruleset idp-main: bypass actors not verifiable", "bootstrap check: no drift (2 notice(s))"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestCheckParamsEnvUsageErrors(t *testing.T) {
	f := newE2E(t)
	for name, args := range map[string][]string{
		"params-env with files": append([]string{"bootstrap", "check", "--params-env", "IDP_BOOTSTRAP"}, f.args...),
		"params-env on apply":   {"bootstrap", "apply", "--org", "acme", "--claims-repo", "idp-claims", "--params-env", "IDP_BOOTSTRAP", "--passphrase-file", f.pass},
		"hidden bypass on apply": append(append([]string{"bootstrap", "apply", "--allow-hidden-bypass", "--passphrase-file", f.pass}, f.args...)),
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, f.env(nil)); code != 2 {
			t.Errorf("%s: exit %d, want 2 (stderr %q)", name, code, stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bootstrap", "check", "--org", "acme", "--claims-repo", "idp-claims", "--params-env", "IDP_BOOTSTRAP"}, &stdout, &stderr, f.env(nil))
	if code != 1 || !strings.Contains(stderr.String(), "IDP_BOOTSTRAP is empty") {
		t.Errorf("empty params env: exit %d, stderr %q", code, stderr.String())
	}
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/bootstrap/ ./internal/cli/`
Expected: FAIL to compile, with `undefined: ParseParams`, `AllowHiddenBypass` and `Notice`.

- [ ] **Step 3: Implement**

`internal/bootstrap/desired.go`: add `VarParams = "IDP_BOOTSTRAP"` to the const block, next to `VarWriterClient`.

`internal/bootstrap/config.go`, appended:
```go
// AppIdentity is the public part of an App's credentials.
type AppIdentity struct {
	ID       int64  `json:"id"`
	ClientID string `json:"clientId"`
	Slug     string `json:"slug"`
}

// Params is the non-secret identity of a bootstrap. apply records it in the
// claims repo as variable IDP_BOOTSTRAP, so the drift workflow can run check
// without the local App files (ADR-0017).
type Params struct {
	ApproverID int64       `json:"approverId"`
	Reader     AppIdentity `json:"reader"`
	Writer     AppIdentity `json:"writer"`
}

// Params returns the identity part of c.
func (c Config) Params() Params {
	return Params{
		ApproverID: c.ApproverID,
		Reader:     AppIdentity{ID: c.Reader.ID, ClientID: c.Reader.ClientID, Slug: c.Reader.Slug},
		Writer:     AppIdentity{ID: c.Writer.ID, ClientID: c.Writer.ClientID, Slug: c.Writer.Slug},
	}
}

// String is the variable value: compact JSON with a fixed field order.
func (p Params) String() string {
	b, _ := json.Marshal(p) // a struct of ints and strings always marshals
	return string(b)
}

// ParseParams reads an IDP_BOOTSTRAP value; unknown fields are rejected.
func ParseParams(s string) (Params, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	var p Params
	if err := dec.Decode(&p); err != nil {
		return Params{}, fmt.Errorf("parse %s: %w", VarParams, err)
	}
	return p, nil
}

// Config returns the check configuration these params describe (no keys).
func (p Params) Config(org, repo string) Config {
	return Config{
		Org: org, ClaimsRepo: repo, ApproverID: p.ApproverID,
		Reader: AppCredentials{ID: p.Reader.ID, ClientID: p.Reader.ClientID, Slug: p.Reader.Slug},
		Writer: AppCredentials{ID: p.Writer.ID, ClientID: p.Writer.ClientID, Slug: p.Writer.Slug},
	}
}
```

`internal/bootstrap/bootstrapper.go`:

1. Add to the `Bootstrapper` struct:

```go
	// AllowHiddenBypass reports ruleset bypass actors the token cannot see as
	// notices instead of drift, for read-only tokens (drift workflow, ADR-0017).
	AllowHiddenBypass bool
```

2. In `variableSpecs`, add the entry `{scope: "", name: VarParams, value: b.Cfg.Params().String()},`.

`internal/bootstrap/check.go`:

1. Add `Notice bool` to `Finding`, with the doc comment `// Notice marks information that is not drift (for example a field this token cannot verify).`.
2. Change `String()` to:

```go
func (f Finding) String() string {
	s := f.Resource + ": " + f.Problem
	if f.Notice {
		return "notice: " + s
	}
	return s
}
```

3. Add the method:

```go
func (f *findings) notice(resource, problem string) {
	*f = append(*f, Finding{Resource: resource, Problem: problem, Notice: true})
}
```

4. In `checkRulesets`, after `drift, err := b.rulesetDrift(...)` and its error check, insert:

```go
		if b.AllowHiddenBypass && slices.Contains(drift, bypassHiddenDrift) {
			drift = slices.DeleteFunc(drift, func(p string) bool { return p == bypassHiddenDrift })
			f.notice("ruleset "+want.Name, "bypass actors not verifiable with this token (read-only); run check with an owner token to verify them")
		}
```

   Import `"slices"`.

`internal/cli/bootstrap.go`, in `runBootstrapRepo`:

1. Add the flags:

```go
	paramsEnv := fs.String("params-env", "", "check only: read the bootstrap identity from this environment variable (IDP_BOOTSTRAP in workflows) instead of --approver/--reader/--writer")
	allowHidden := fs.Bool("allow-hidden-bypass", false, "check only: report ruleset bypass actors this token cannot see as notices, not drift")
```

2. After `fs.Parse`, validate the combinations:

```go
	if mode != "check" && (*paramsEnv != "" || *allowHidden) {
		fmt.Fprintf(stderr, "idp bootstrap %s: --params-env and --allow-hidden-bypass are check-only flags\n", mode)
		return exitUsage
	}
	if *paramsEnv != "" && (*approver != "" || *readerFile != "" || *writerFile != "") {
		fmt.Fprintln(stderr, "idp bootstrap check: use either --params-env or --approver/--reader/--writer")
		return exitUsage
	}
```

3. When `*paramsEnv != ""`, the `required` list holds only `--org` and `--claims-repo`.
4. Build `cfg` from the params:

```go
	if *paramsEnv != "" {
		raw := env(*paramsEnv)
		if raw == "" {
			return fail(stderr, fmt.Errorf("environment variable %s is empty", *paramsEnv))
		}
		params, err := bootstrap.ParseParams(raw)
		if err != nil {
			return fail(stderr, err)
		}
		cfg = params.Config(*org, *repo)
	}
```

   Keep the existing file-based loading and the `UserID` lookup in an `else` branch.
5. Set `b.AllowHiddenBypass = *allowHidden` on the check `Bootstrapper`.
6. Replace the findings printing with:

```go
	notices := 0
	for _, f := range found {
		fmt.Fprintln(stdout, f)
		if f.Notice {
			notices++
		}
	}
	if drift := len(found) - notices; drift > 0 {
		fmt.Fprintf(stdout, "bootstrap check: %d finding(s)\n", drift)
		return exitFindings
	}
	if notices > 0 {
		fmt.Fprintf(stdout, "bootstrap check: no drift (%d notice(s))\n", notices)
	} else {
		fmt.Fprintln(stdout, "bootstrap check: no drift")
	}
	return exitOK
```

7. Update `bootstrapUsage`'s check line to `  idp bootstrap check --org ORG --claims-repo REPO (--approver LOGIN --reader FILE --writer FILE | --params-env NAME) [--allow-hidden-bypass]`.

Existing apply tests that assert an exact list of writes, or of variables, now also see the `IDP_BOOTSTRAP` variable. Update those expectations, and nothing else, in the same commit.

- [ ] **Step 4: Document it in the runbook**

In `docs/runbooks/bootstrap.md`, after the "4. Verify" section, add:

````markdown
## Check from a workflow

`apply` records the bootstrap identity in the repository variable `IDP_BOOTSTRAP`: the App ids, client ids and slugs, and the approver's user id. None of it is secret. A workflow can then run `check` without the local App files:

```bash
IDP_BOOTSTRAP='<the variable value>' GH_TOKEN=<reader token> go run ./cmd/idp bootstrap check \
  --org <org> --claims-repo <claims-repo> --params-env IDP_BOOTSTRAP --allow-hidden-bypass
```

`--allow-hidden-bypass` exists for read-only tokens: GitHub hides ruleset bypass actors from them, so those lists are printed as `notice:` lines instead of drift ([ADR-0017](../adr/0017-drift-check-token.md)). An owner-token `check`, as in step 4, still verifies them.
````

- [ ] **Step 5: Run all tests and see them pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/bootstrap internal/cli docs/runbooks/bootstrap.md
git commit -m "feat(bootstrap): run check from the recorded identity with a read-only token"
```

---

### Task 12: CLI reference

**Files:**
- Create: `docs/cli.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: every command, as built in Tasks 2–11.

- [ ] **Step 1: Write `docs/cli.md`**

Write one section per command, in the order of `idp help`:
- `validate`, `render`, `diff`, `plan-summary`, `gate`, `comment`, `issue open|close`, `wet-push`, `encryption-env`;
- `bootstrap app|apply|check`.

Each section needs:
- a one-line purpose;
- the synopsis, copied from the command's flags;
- what it reads (files, environment variables);
- what it writes (files, step outputs, GitHub API calls);
- its exit codes.

Start the file with:

```markdown
# CLI reference

Every command shares one exit-code convention ([ADR-0016](adr/0016-exit-codes.md)):

| Code | Meaning |
|---|---|
| 0 | done; nothing needs attention |
| 1 | the command could not do its job |
| 2 | usage error |
| 3 | done, and the answer needs attention (invalid claims, drift) |

Commands that talk to GitHub read a token from `GH_TOKEN`, else `GITHUB_TOKEN`. Inside GitHub Actions, commands write step outputs to `GITHUB_OUTPUT`, and `encryption-env` writes to `GITHUB_ENV`.
```

The facts to include, verbatim, where they apply:
- `diff`: output `affected` (a JSON array); statuses `new`, `changed`, `unchanged`, `orphan`; `--all` for a manual reconcile.
- `plan-summary`: outputs `changes` and `destructive`; the comment carries `<!-- idp-plan -->` and the fingerprint marker (ADR-0018).
- `gate`: output `decision`, which is `auto` or `approval`, plus the rules of ADR-0018.
- `comment`: edits only comments by `github-actions[bot]`.
- `issue`: one open issue per label and title; `close` is a no-op when none is open.
- `wet-push`: output `commit`, empty when nothing changed; verified commits; no force (ADR-0019).
- `encryption-env`: reads `IDP_STATE_PASSPHRASE`; the frozen name `idp` (ADR-0020).
- `bootstrap check`: `--params-env` and `--allow-hidden-bypass` (ADR-0017); notices do not count as drift.

Before committing, check every flag name against the code with `rg -n 'fs\.(String|Bool|Int|Var|Func)\(' internal/cli`.

- [ ] **Step 2: Link it from the README**

Under the `- Claims reference:` line, add: `- CLI reference: [docs/cli.md](docs/cli.md)`

- [ ] **Step 3: Check and commit**

Run the owner-rule check the controller gave you over `docs` and `README.md` (expected: no output).

```bash
git add docs/cli.md README.md
git commit -m "docs: add the cli reference"
```

---

### Task 13: Ship — PR, CI, merge, execution log

**Files:**
- Create: `docs/phases/phase-1b.md`
- Modify: `README.md` (Status line)

- [ ] **Step 1: Push and open the PR**

```bash
git push -u origin feat/phase-1b-pipeline-commands
gh pr create --repo jellalshadows-idp/idp-engine --base main --head feat/phase-1b-pipeline-commands --title "feat: phase 1b pipeline commands" --body "Implements docs/superpowers/plans/2026-10-10-phase-1b-pipeline-commands.md (ADRs 0016-0020, spec amendments A3-A5). Adds idp diff, plan-summary, gate, comment, issue, wet-push, encryption-env, and drift-ready bootstrap check."
```

- [ ] **Step 2: Wait for CI**

Run: `gh pr checks --repo jellalshadows-idp/idp-engine --watch`
Expected: `go`, `workflows`, `modules (github/group)`, `modules (github/component)` and `render-smoke` all pass.

- [ ] **Step 3: Merge**

Run: `gh pr merge --repo jellalshadows-idp/idp-engine --merge --delete-branch && git switch main && git pull --ff-only`

- [ ] **Step 4: Write `docs/phases/phase-1b.md` and update the README status**

Write the execution log in the style of `docs/phases/phase-1a.md`:
- the summary;
- a task table with commits and review outcomes;
- every ruling, with what, why and cost if wrong;
- every deferred finding with its final disposition (fixed, Phase 1c, Phase 1d, or won't fix with a reason);
- the CI evidence.

Then replace the README `> **Status:**` line with:

```markdown
> **Status:** Phase 1b is complete: every pipeline subcommand (`diff`, `plan-summary`, `gate`, `comment`, `issue`, `wet-push`, `encryption-env`) and a drift-ready `bootstrap check` ship with tests; see the [phase 1b log](docs/phases/phase-1b.md). Phase 1c (the reusable workflows and a live smoke run) is next, so nothing is reconciled automatically yet.
```

- [ ] **Step 5: Commit (doc-only, straight to main) and push**

```bash
git add docs/phases/phase-1b.md README.md
git commit -m "docs: add the phase 1b execution log"
git push
```
