# Phase 1b execution log: pipeline commands

- Period: 2026-10-10
- Plan: [2026-10-10-phase-1b-pipeline-commands.md](../superpowers/plans/2026-10-10-phase-1b-pipeline-commands.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md) (§7 trust and gate, §10 roadmap), amended by A3, A4 and A5
- Decisions: ADR-0016 to ADR-0020 (see "Spec and ADR changes")
- Previous phase: [Phase 1a log](phase-1a.md)
- Status: **complete** (merged through [PR #6](https://github.com/jellalshadows-idp/idp-engine/pull/6), merge commit `6d83a97`).

This file is the durable copy of the execution ledger that was kept while the plan ran. The ledger lived in a git-ignored scratch folder that is deleted later, so everything it recorded is reproduced here: the rulings, every task's review outcome, every finding with its final disposition, and what is carried into the next phases.

## Summary

Phase 1b delivers every subcommand the pipelines will call, each with tests, plus the conventions they share. No workflow exists yet; the commands are the building blocks Phase 1c wires together.

- `idp diff` compares a fresh render (`--new`) with the wet tree (`--wet`) and prints the stacks to plan, as JSON an Actions job can consume.
- `idp plan-summary` parses OpenTofu plan JSON files into per-stack counts, renders the PR plan comment, and embeds a fingerprint marker (a hash of the planned changes plus the head SHA) that the gate later trusts.
- `idp gate` decides `auto` or `approval` for a merge: it finds the merged pull request, reads the newest trusted plan comment, decodes the fingerprint, and requires approval whenever anything destructive or unverifiable is involved.
- `idp comment` keeps exactly one sticky plan comment per pull request; `idp issue` keeps one labelled issue per failure.
- `idp wet-push` commits the rendered stacks and the state to the wet branch in one commit through the Git data API.
- `idp encryption-env` exports `TF_ENCRYPTION` from the state passphrase, masking it in logs.
- `idp bootstrap check` is drift-ready: it runs from the recorded identity with a read-only token, verifies the `IDP_BOOTSTRAP` variable, and reports hidden bypass actors as notices instead of drift.
- One exit-code convention across the CLI (ADR-0016): 0 success, 1 error, 2 usage, 3 findings or drift. Annotations use paths relative to `GITHUB_WORKSPACE`.
- A new `internal/actions` package wraps the Actions file commands (`GITHUB_OUTPUT`, `GITHUB_ENV`, `GITHUB_STEP_SUMMARY`, error annotations, masking).

Phase 1 was re-split in this phase (spec §10): **1a** claims to render, **1b** the pipeline subcommands, **1c** the reusable workflows and a live smoke run in `idp-claims-e2e`, **1d** the E2E harness and the `v0.1.0` release. The original 1c scope (release and end-to-end) is now 1d.

How it was executed: subagent-driven. For each of the 13 tasks, the controller dispatched an implementer, then a separate reviewer; findings went back to the implementer as fix rounds (at most five allowed per task); a whole-branch review by the most capable model followed, and one final fix wave closed it, with a scoped re-review. The work was pushed as [PR #6](https://github.com/jellalshadows-idp/idp-engine/pull/6) and merged with a merge commit (`6d83a97`), which keeps the per-task commits. The plan itself landed on `main` earlier as `27a13fe` and `0dfb599`.

CI evidence. The PR ran CI twice and both runs were fully green: at `1ab664f` (the end of Task 12, which fed the whole-branch review) and at `b8886b6` (after the final fix wave). Each run covers five jobs: `go` (tests with `-race`, vet, gofmt), `workflows`, `modules (github/group)`, `modules (github/component)` and `render-smoke`.

Scope that moved to later phases: the reusable workflows and the live smoke run are Phase 1c; the E2E harness, the release and the remaining test and polish items are Phase 1d.

## Tasks

"Review" is the review done after the task by a separate reviewer. Commit ranges are `base..tip` on `feat/phase-1b-pipeline-commands`, created from `main` at `0dfb599`.

| Task | What | Commits | Review |
|---|---|---|---|
| 1 | ADRs 0016 to 0020 and spec amendments (`97676d6`) | `0dfb599..97676d6` | Clean |
| 2 | Exit-code convention, `internal/actions`, workspace-relative annotations | `97676d6..210e77b` | Clean |
| 3 | `idp diff` (`wetdiff`) | `210e77b..6925209` | Clean after 1 fix round |
| 4 | OpenTofu plan parsing: counts, fingerprint, fixtures from real plans | `6925209..68284a5` | Clean |
| 5 | `idp plan-summary`: comment rendering and fingerprint marker | `68284a5..5676650` | Clean after 1 fix round |
| 6 | GitHub client pagination and the trusted-comment / merged-PR lookups (`ghapi`, `ghops`) | `5676650..30d0c3f` | Clean |
| 7 | `idp gate` | `30d0c3f..467a63f` | Clean |
| 8 | `idp comment` and `idp issue` (sticky comment, labelled issue) | `467a63f..ffafeac` | Clean |
| 9 | `idp wet-push` and the usage-alignment test | `ffafeac..eb48043` | Clean after 1 fix round |
| 10 | `idp encryption-env` and passphrase validation / HCL escaping | `eb48043..b706322` | Clean |
| 11 | Drift-ready `bootstrap check` (recorded identity, read-only token, `IDP_BOOTSTRAP`) | `b706322..5f10e3c` | Clean |
| 12 | CLI reference documentation (`docs/cli.md`) | `5f10e3c..1ab664f` | Clean |
| 13 | Push, PR #6, CI, whole-branch review and fix wave, merge, this log | PR #6; fix wave `1ab664f..b8886b6`; merge `6d83a97` | Whole branch: with fixes, then clean after 1 wave |

Final fix wave commits (`1ab664f..b8886b6`): `40849a9`, `8196c90`, `9d2dc3f`, `bcbc284`, `d7bef01`, `b8886b6`.

## Rulings

A ruling is a controller decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong. The labels are the ledger's.

- **P1. Execution order of the PR and the final review.** Run Tasks 1 to 12, then Task 13 steps 1 and 2 (push, open the PR, CI), then the whole-branch final review with the CI result in hand, then the single fix wave, then the merge (step 3) and the log and README (steps 4 and 5). *Why:* the final review must precede the merge, and CI results should feed it. *Cost if wrong:* one extra CI cycle.
- **P2. The merge proceeds without a further ask.** *Why:* the owner approved this plan, whose Task 13 merges, matching Phase 1a. *Cost if wrong:* a merged PR the owner wanted to inspect first (revertable).
- **P3. The owner-rule check pattern lives only in dispatch prompts and controller commands, never in repo files.** The plan says "the check the controller gave you"; the pattern itself would be a mention of exactly what the rule forbids. *Cost if wrong:* none.
- **Task 3. A mis-wired path must fail loudly, never mean "nothing affected".** The reviewer found that a regular file passed as `--new` or `--wet` was silently treated as a render with no stacks: `--new` gave `affected=[]` and exit 0 (so pipelines skip planning), `--wet` hid orphaned stacks. Ruling: `readStacks` errors when the root exists but is not a directory, and `idp diff` requires the render marker (`.idp-rendered`) in `--new`, failing with "not an idp render". The wet tree has no marker (`wet-push` syncs stack directories only), so the marker check applies to `--new` only. *Cost if wrong:* a hand-made `--new` directory without the marker is refused; only `idp render` produces `--new`.
- **Task 5. Accept all three Important findings on the plan comment.** (1) `DecodeMarker` took the first match of an unanchored regex, and addresses are rendered into the body before the real marker, so a marker-shaped address could shadow the fingerprint the gate trusts: it now uses the last match. (2) Addresses and stack names were rendered raw into Markdown (backtick, `|`, newline break tables and code spans): they now render inside `<code>` through an escaper for `& < > |` and backtick and CR/LF, so no plan text can form a marker or break a table; the golden file was updated. (3) The size guard was one pass, so the omitted-details body plus marker could still exceed 65536 bytes and lose the fingerprint: after omitting the change lists, a second check returns an error naming the size. *Why:* claims-derived addresses cannot carry these characters today (names and keys are validated), but Phase 3 Workspace modules take user `values` as `for_each` keys, so this closes a future trust hole now; failing loudly beats posting a comment without its fingerprint. *Cost if wrong:* a slightly uglier raw comment body; an absurdly large plan fails `plan-summary` instead of posting.
- **Task 7. No change for the possible `head[:7]` panic.** The review flagged a possible panic if `DecodeMarker` returned a short SHA. *Why no change:* `DecodeMarker` only matches `sha=([0-9a-f]{40})`, so a decoded marker always carries 40 hex characters. The invariant lives in the regex the gate depends on. *Cost if wrong:* none unless the regex is loosened (the whole-branch review could have added a `shortSHA` helper; it did not ask for one).
- **Task 9 (1). Usage alignment gets a test.** The bootstrap usage line in `cli.go` regressed by one column twice (in Task 5 and again in Task 8). Task 9 restores the padding and adds `TestUsageColumnsAlign`, so the 16-column layout cannot regress silently. *Cost if wrong:* one extra small test.
- **Task 9 (2). `wet-push` never deletes the state.** The reviewer found that a wrong or empty `--root` (or a failed state download) made `Sync` delete every remote blob under the synced paths, including `tfstate/github.tfstate`, as a normal commit; the next reconcile would run without state and try to recreate everything. Ruling: `Sync` requires the root to exist and be a directory; a `--path` that is a single file on the remote branch and is missing locally is refused ("refusing to delete <path>: it is missing under <root>") instead of deleted; `validatePaths` also rejects any path element `.terraform`. Directory paths may still lose all their files, which is the legitimate orphan-destroyed case (for example `rendered/aws/dev/_baseline`). *Why:* deleting the state is never part of a reconcile. *Cost if wrong:* a legitimate whole-file removal needs a directory path or a manual commit. Accepted tradeoff: an existing-but-wrong `--root` can still empty a directory path.
- **FW. One final fix wave, scoped.** After the whole-branch review ("with fixes": three Important findings and eight minors), a single wave covered:
  - F1 (I3): `plan-summary` requires `--stacks` (the affected JSON from `idp diff`) and exactly one `--plan` per listed stack; a missing input fails loudly. Cheap now, a breaking change after Phase 1c.
  - F2 (I2): `idp encryption-env` registers the raw passphrase and, when different, its HCL-escaped form with `::add-mask::` before exporting; one exported escaper (`render.EscapeHCLString`); ADR-0020 corrected.
  - F3 (I1): ADR-0018 states the real rule (newest bot-authored marker comment wins; others are ignored) and names the edit route and the PR-branch-workflow route as accepted risks.
  - F4 (triage): the `cli.md` `GITHUB_ENV` sentence, the runbook stale clause and blank line, an empty `--marker` rejected as a usage error, the `Type == "Bot"` test, the ADR-0017 params self-comparison note, ADR-0019 safeguards, the README stale "next" line.

  *Why:* all cheap now and more expensive after the workflows exist. *Cost if wrong:* a larger PR; `--stacks` is one more required flag for the Phase 1c wiring.
- **DEFER-1b. Deferred from the whole-branch review.** I1 option B (reject plan comments edited by anyone other than the Actions bot, via the GraphQL `editor` / `lastEditedAt` fields) becomes a Phase 1c decision. M1 (forget column), M3 (bound the issue body; 1c passes the bounded comment as the body), M4 (wet marker for `--wet`) and M7 (`wetpush` `PathEscape`) go to Phase 1c or 1d. M2 (nested stack discovery) goes to Phase 4 together with the stack-discovery work (T3a). The owner-log note on the `idp-main` ruleset is recorded in "Owner notes". *Cost if wrong:* rediscovery later.

Spec and ADR changes decided while planning and written in Task 1 (`97676d6`):

- **ADR-0016** exit codes: 0, 1, 2, 3 convention (closes the Phase 0 exit-code ADR item M1).
- **ADR-0017** drift check token: `bootstrap check` runs from the recorded identity with a read-only token; hidden bypass actors are notices, not drift (amendment A5; closes the Phase 0 drift-token ADR item).
- **ADR-0018** plan fingerprint and gate trust (amendment A3): the gate trusts the newest bot-authored marker comment on the merged PR's head SHA.
- **ADR-0019** wet commits through the Git data API: one commit per sync, no force pushes.
- **ADR-0020** `TF_ENCRYPTION` from `idp encryption-env`.
- **A3, A4, A5** amend the spec (§7 gate and fingerprint, §3 units, §7 drift check) and **§10** records the 1a/1b/1c/1d re-split.

## Notable findings caught by review

| Finding | Where caught | Fix |
|---|---|---|
| A regular file as `--new` or `--wet` was silently a stackless render: `--new` gave `affected=[]` and exit 0, so pipelines would skip planning | Task 3 implementer concern, confirmed in review | `6925209`: non-directory roots rejected, `--new` must carry the render marker (ruling Task 3) |
| `DecodeMarker` took the first match, so a marker-shaped address rendered before the real marker could shadow the fingerprint the gate trusts; plan text was rendered unescaped into Markdown | Task 5 review | `5676650`: last-match decode, plan text escaped inside `<code>` (ruling Task 5) |
| The comment size guard was one pass: with the details omitted, the body plus marker could still exceed 65536 bytes and lose the fingerprint | Task 5 review | `5676650`: second size check returns an error naming the size |
| The bootstrap usage line regressed by one column in Task 5 and again in Task 8 | Task 5 and Task 8 reviews | `41ca055`: padding restored and `TestUsageColumnsAlign` added (ruling Task 9 (1)) |
| A wrong `--root` or a failed state download made `wet-push` delete the remote `tfstate/github.tfstate` as a normal commit | Task 9 review | `eb48043`: root must be an existing directory, missing file paths refused, `.terraform` paths rejected (ruling Task 9 (2)) |
| I1: ADR-0018 and `cli.md` overstated the gate trust boundary. Write-access users can edit the bot comment, and any workflow's `GITHUB_TOKEN` writes as `github-actions[bot]`, including a workflow on a PR branch | Whole-branch review | `9d2dc3f`: the real rule is stated and both routes are named as accepted risks (members only, spec §7.1; destructive changes always need approval); stricter option deferred (DEFER-1b) |
| I2: an HCL-escaped passphrase inside `TF_ENCRYPTION` is not masked by GitHub, and `GITHUB_ENV` values show up in later step log headers | Whole-branch review | `8196c90`: raw and HCL-escaped forms both registered with `::add-mask::`; one exported escaper; ADR-0020 corrected |
| I3: `plan-summary` with a missing plan input produced a fingerprint without that stack, so the gate said "no changes, auto" while the reconcile applied the saved plan | Whole-branch review | `40849a9`: `--stacks` required and exactly one `--plan` per listed stack |
| An empty `--marker` passed the body check and would let `idp comment` overwrite any bot comment | Task 8 review, fixed in the wave | `bcbc284`: rejected as a usage error |
| The `Type == "Bot"` half of the trust rule had no test | Task 6 review, fixed in the wave | `d7bef01`: regression pin (passes on the current code, so there is no RED by nature) |

Findings checked and found not to be problems: the `head[:7]` panic (ruling Task 7); the Task 2 risk of the bulk literal-to-constant exit-code swap (the reviewer verified every changed return keeps its meaning except the two intended findings changes); the two edited `apply` tests in Task 11 (they only add the `IDP_BOOTSTRAP` expectation); the `ghapi.New` trailing-slash handling and the fake server's lock order (verified in code).

## Final disposition of every finding

Statuses: **Fixed** (done, with the commits), **Phase 1c**, **Phase 1d**, **Phase 4**, **Won't fix** (consciously rejected, with the reason), **Obsolete** (resolved by later work or found not to be a problem). No finding is left open. "Final review" means the whole-branch review.

### Fixed

- Task 3 Important (non-directory roots, non-render `--new`): **Fixed**, `6925209`.
- Task 5 Important 1 (first-match marker decode): **Fixed**, `5676650`.
- Task 5 Important 2 (unescaped plan text in Markdown): **Fixed**, `5676650`.
- Task 5 Important 3 (one-pass size guard): **Fixed**, `5676650`.
- Task 5 and Task 8 (bootstrap usage line one column short, twice): **Fixed**, `5676650` then `41ca055` (with `TestUsageColumnsAlign`).
- Task 9 Important (`wet-push` deleting the state on a wrong root): **Fixed**, `eb48043`.
- Task 6 minor (the Bot-type half of the trust rule untested): **Fixed**, `d7bef01`.
- Task 8 minor (empty `--marker`): **Fixed**, `bcbc284`.
- Task 2 minor (runbook "Drift is reported" sentence without a blank line): **Fixed**, `b8886b6`.
- Task 11 minor (runbook still says the token strategy is decided in Phase 1): **Fixed**, `b8886b6`.
- Task 12 minor (`cli.md` says `GITHUB_ENV` writes are skipped silently, but `encryption-env` exits 2): **Fixed**, final fix wave (`1ab664f..b8886b6`).
- Final review I1 (trust-boundary wording and accepted risks): **Fixed** in the docs, `9d2dc3f`; the stricter check is Phase 1c.
- Final review I2 (unmasked HCL-escaped passphrase): **Fixed**, `8196c90`.
- Final review I3 (missing plan input becomes "auto"): **Fixed**, `40849a9`.
- Final review minor (ADR-0017 params self-comparison note): **Fixed**, `b8886b6`.
- Final review minor (ADR-0019 safeguards): **Fixed**, `b8886b6`.
- Final review minor (README stale "next" line): **Fixed**, `b8886b6`.

### Phase 1c (workflows and live smoke run)

- Final review I1 option B: reject plan comments edited by anyone other than the Actions bot (GraphQL `editor` / `lastEditedAt`); a decision for the gate wiring.
- Final review M3: bound the issue body; 1c passes the bounded plan comment as the body.
- Task 8 minor: `idp comment` and `idp issue` must run with the workflow's `GITHUB_TOKEN` (a `GH_TOKEN` wins in `token()`), or the one-sticky-comment rule breaks; 1c workflows pin it for these steps.
- Task 10 minor: HCL escaping was verified by reading, not by a parser round-trip (no `hclsyntax` dependency allowed); the live smoke exercises the real path.
- Task 5 minor: `runURL` is interpolated unvalidated into the Markdown link; the workflow supplies it, so validate when the workflow is written.
- Task 6 minor: `MergedPullForCommit` does not check the base branch; check it when the gate is wired to the real `main`.
- Task 7 minor: some gate reasons do not say what to do (for example re-run the PR plan); reword with the workflow's real recovery steps.
- Task 11 minor: the runbook example is bash among PowerShell steps; settle it in the 1c runbook revision.
- Final review residual (found in the wave's re-review): `actions.Mask` was inserted between `SetOutput`'s doc comment and `func SetOutput`, so `SetOutput` is undocumented and its comment sits on `Mask`; parked for Phase 1c's first edit of `internal/actions` (cosmetic; no second fix wave by process).

### Phase 1d (E2E harness and release)

- Final review M1: forget column; M4: wet marker for `--wet`; M7: `wetpush` `PathEscape` (DEFER-1b, 1c may take them if it touches the same code first).
- Task 6 minor: the across-pages test does not prove newest-wins across pages; `nextPage` lacks tests for a lookalike host prefix and a `/api/v3` base path.
- Task 7 minor: no tests for API-error exit 1, unreadable or absent marker (approval), missing token, or the summary write.
- Task 5 minor: no test for an empty `--head-sha` alone.
- Task 8 minor: `CloseIssue` posts the comment before the PATCH (a retry double-posts) and closes only the first match; `EnsureLabel` returns 422 on a concurrent create; no tests for the no-token `githubRepo` or the issues-list query string; fake comment ids may collide with hand-seeded ids.
- Task 9 minor: file-to-directory flips untested; the truncated-tree test does not assert that nothing was written; an invalid `--path` exits 1, not 2.
- Task 10 minor: no CLI test with a special-character passphrase; `ReadPassphrase` maps any non-encoding error to "shorter than 16".
- Task 11 minor: the lenient-mode test does not prove other drift on the same ruleset survives; `ParseParams` accepts trailing data, `{}` and case-insensitive names; no test that `check` flags a missing or altered `IDP_BOOTSTRAP`.
- Task 4 minor: no tests for empty or three-element `actions` arrays or `ReadFingerprint` trailing data.
- Task 2 minor: no Windows-drive, symlink or delimiter-collision tests for `internal/actions`.

### Phase 4 (stack discovery)

- Final review M2 and Task 3 minor: nested stack discovery, the nested-stack ownership and symlink-branch tests, and a comment that a root `main.tf.json` is silently not a stack; all belong with the T3a discovery work.

### Won't fix

- Task 1: the spec gate paragraph was inserted without blank lines, so it merges with its neighbours when rendered. Cosmetic, plan-mandated, the source reads fine.
- Task 1: the spec's second units table fragment has no header row since Phase 0. Pre-existing and cosmetic.
- Task 1: ADR-0016 "exit 0 when they produced them" could add "or 1 if they could not run". ADR-0020 states the failing case explicitly.
- Task 2: `annotationFile` does not resolve symlinks (it falls back to the plain path). A runner checkout has no symlinked workspace, and the fallback is still a valid path.
- Task 2: `actions.appendEntry` accepts names containing `<`. Names are literals in the engine's own code, never user input.
- Task 3: `TestDiffUsageErrors` cannot tell unknown-command from usage (both 2). The behaviour is correct; only the test is coarse.
- Task 3: a missing `--new` says "is not an idp render" instead of "does not exist". It still fails loudly with the right exit code.
- Task 4: the `format_version` check accepts `"1"` and `"1.garbage"`. Only the major version matters, and OpenTofu emits well-formed values.
- Task 4: duplicate (address, action) entries are not de-duplicated. Real plans cannot produce them.
- Task 4: the generator README chain leaves the shell in the work directory on failure. A developer-only note.
- Task 5: the comment file is written before the fingerprint and outputs. A later failure fails the step anyway and the file is never trusted.
- Task 5: the 8 MiB decompression cap could be 1 to 2 MiB. Safe as is; tightening has no user-visible benefit.
- Task 6: relative `next` links are refused. Fail closed, GitHub emits absolute links.
- Task 6: the `Link` regex ignores extra params and multi-valued rels. GitHub emits the plain form.
- Task 7: a missing token exits 2, arguably 1 per ADR-0016. A missing token is a misconfiguration of the invocation, the same class as a missing flag.
- Task 9: file modes are not preserved (uploads are forced to 100644). Fine for tfstate and JSON.
- Task 9: inconsistent error wrapping (only ref read and update are wrapped). Messages are clear enough.
- Task 9: `TestUsageColumnsAlign` uses hardcoded indexes. It is a pin; a layout change should break it.
- Task 9 out-of-scope note: an existing-but-wrong `--root` can still empty a directory path. Accepted tradeoff recorded in ruling Task 9 (2).
- Task 10: HCL NFC-normalizes strings, so a non-NFC passphrase is keyed as its NFC form. Consistent across runs.
- Task 12: render output wording, passphrase wording ("single line" versus "no control characters"), one synopsis line indented differently. Cosmetic.

### Obsolete

- Task 12 minor (mention that `plan-summary` with no `--plan` yields 0 changes and the gate says auto): obsolete since F1 made `--stacks` and one `--plan` per stack required (`40849a9`).
- Task 7 Important (`head[:7]` panic): not a problem, see ruling Task 7.
- Task 2 named risk (bulk constant swap) and Task 11 named risk (edited apply tests): cleared by the reviewers.

Count: 17 Fixed, 9 Phase 1c, 10 Phase 1d, 1 Phase 4, 21 Won't fix, 4 Obsolete; 62 entries in all. Ten rulings (P1, P2, P3, Task 3, Task 5, Task 7, Task 9 twice, FW, DEFER-1b).

## Carried into Phase 1c and 1d

From this phase (DEFER-1b and the constraints the review set):

- **Phase 1c:** decide the GraphQL editor check for the gate (I1 option B); bound the issue body (1c passes the bounded comment); run `idp comment` and `idp issue` with the workflow's `GITHUB_TOKEN` and no `GH_TOKEN`; fix the misplaced doc comment on `actions.Mask` / `SetOutput` at the first edit of `internal/actions`.
- **Phase 1c or 1d:** the forget column (M1), the wet marker for `--wet` (M4), `wetpush` `PathEscape` (M7).
- **Phase 1c wet hygiene rules:** run `tofu init -lockfile=readonly`; keep plan files outside the stack directories; sync to the wet branch from a clean copy of the render artifact.

Follow-ups still open from Phase 1a:

- Annotation paths relative to `GITHUB_WORKSPACE`: **done in this phase** (Task 2, `210e77b`).
- GoReleaser `{{.Tag}}` guard (the `v` must survive in `?ref=`): Phase 1d.
- E2E steps "remove all topics" and "case-only login edit": Phase 1d.
- Go patch pin via Renovate: Phase 1d.

Closed by this phase from the Phase 0 and 1a inherited list: the drift-workflow token ADR (ADR-0017) and the exit-code ADR (ADR-0016).

## Owner notes

The whole-branch review observed that the `idp-main` ruleset (created in the Phase 0 bootstrap) has `required_approving_review_count: 0`, and the `idp-gate` required check is not pinned to an integration. It fails safe: without a trusted plan comment the gate asks for approval. It is recorded here for the owner to decide whether to tighten the ruleset (a review count of 1 and pinning `idp-gate` to the Actions integration).

## Process notes

- **Owner rule:** the docs describe the platform on its own terms. The check ran on every task, on the whole branch (including hidden directories) and on the final fix wave, and was clean each time.
- **Plan defects handled as rulings:** the plan's order of merge and final review (P1) and several plan-mandated behaviours that the reviews showed to be flawed (Tasks 3, 5 and 9) were fixed rather than kept, because the spec's goals outrank the plan's code.
- **Whole-branch review coverage:** the three Important findings were fixed in the final fix wave; the minors were either fixed in the same wave, deferred under DEFER-1b, or are recorded in the dispositions above.

## Status

**Phase 1b is complete.** Every pipeline subcommand and a drift-ready `bootstrap check` shipped with tests and are verified by two green CI runs on PR #6. Every finding has a final disposition above. Nothing is reconciled automatically yet.

Next: Phase 1c (the reusable workflows and a live smoke run in `idp-claims-e2e`), then Phase 1d (E2E harness and the `v0.1.0` release).
