# Phase 1a execution log: claims, validation and render

- Period: 2026-10-09 to 2026-10-10
- Plan: [2026-10-09-phase-1a-claims-render.md](../superpowers/plans/2026-10-09-phase-1a-claims-render.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md) (§4 claims, §5 render, §10 roadmap), amended by A1 and A2
- Decision: [ADR-0015](../adr/0015-claim-parsing-and-validation.md)
- Previous phase: [Phase 0 log](phase-0.md)
- Status: **complete** (merged through [PR #5](https://github.com/jellalshadows-idp/idp-engine/pull/5), merge commit `8af19fa`).

This file is the durable copy of the execution ledger that was kept while the plan ran. The ledger lived in a git-ignored scratch folder that is deleted later, so everything it recorded is reproduced here: the rulings, every task's review outcome, every finding with its final disposition, and what is carried into the next phases.

## Summary

Phase 1a delivers the first half of the engine's Phase 1: turning a claims repo into a validated GitHub stack, without any pipeline around it yet.

- `idp validate` loads a claims repo (`config/platform.yaml`, `claims/groups/*.yaml`, `claims/components/*.yaml`), validates every file against embedded JSON Schemas, runs the semantic checks (names match files, owners exist, environments are declared, prefix rules) and reports every problem at once, located by file and line.
- `idp render` runs the same validation and, only when there are no diagnostics, writes a deterministic OpenTofu stack (`.tf.json`) plus a pinned provider lock file. The tree writer refuses to overwrite a directory it did not render (a marker file guards it).
- Two OpenTofu modules, `github/group` (team and memberships) and `github/component` (repository, topics, rulesets, environments, access), each with `tofu test` suites that run on the mock provider.
- CI gained a job per module (`tofu test`) and a `render-smoke` job.

How it was executed: subagent-driven. For each of the 13 tasks, the controller dispatched an implementer, then a separate reviewer; findings went back to the implementer as fix rounds (at most five allowed per task); a whole-branch review by the most capable model followed, and one final fix wave closed it, with a scoped re-review. The work was pushed as [PR #5](https://github.com/jellalshadows-idp/idp-engine/pull/5) and merged with a merge commit (`8af19fa`), which keeps the per-task commits.

CI evidence. The first CI run on the PR (at `09d3f94`) passed all five jobs: `go`, `workflows`, `modules (github/group)`, `modules (github/component)` and `render-smoke`. The `render-smoke` job runs a real `tofu init -backend=false` and `tofu validate` on a rendered stack. That was the first real check of three things that had been written from documentation and never executed: the `.tf.json` shape of `backend`, `encryption` and `required_providers`, the generated lock file, and the wiring between the rendered root and the two modules (including the Component taking its owner team as a reference to the Group output, amendment A2). All three passed on the first run. After the final fix wave (ruling FW, finding F6) the module jobs also run with `-lockfile=readonly`, so they now verify the provider hashes against the shipped lock file instead of resolving the provider freely.

Scope that moved to later phases: pipelines (PR validation workflow, plan and apply workflows, drift) are Phase 1b; the release, the tagged module references and the end-to-end run against the real org are Phase 1c.

## Tasks

"Review" is the review done after the task by a separate reviewer. Commit ranges are `base..tip` on `feat/phase-1a-render`.

| Task | What | Commits | Review |
|---|---|---|---|
| 1 | Spec amendments A1 and A2, ADR-0015, spec §10 split | `7d55625..8eb730c` | Clean |
| 2 | JSON Schemas for Platform, Group, Component (`schemas/`) | `8eb730c..a302846` | Clean |
| 3 | Diagnostics and YAML parsing with line positions | `a302846..fe83eec` | Clean after 1 fix round |
| 4 | Schema validation with line positions | `fe83eec..4885e2e` | Clean after 1 fix round |
| 5 | Claims loader with semantic checks and prefix rules | `4885e2e..16311e0` | Clean after 1 fix round |
| 6 | Module `github/group` with `tofu test` | `16311e0..0444a09` | Clean |
| 7 | Module `github/component` with `tofu test` | `0444a09..d9bb2cf` | Clean |
| 8 | Pinned provider lock file (`lockfiles`) | `d9bb2cf..15cd4e3` | Clean |
| 9 | Deterministic render and safe tree writer | `15cd4e3..6604c5e` | Clean |
| 10 | `idp validate` and `idp render` in the CLI | `6604c5e..56d7efb` | Clean |
| 11 | CI: module tests and render-smoke | `56d7efb..be18aff` | Clean |
| 12 | Claims reference documentation | `be18aff..5d6561a` | Clean after 1 fix round |
| 13 | Push, PR, CI, final review and fix wave, merge, this log | PR #5; fix wave `5d6561a..2a38cd7`; merge `8af19fa` | Whole branch: with fixes, then clean after 1 wave |

The merge of `main` into the branch (`3da353b`, see "Process notes") sits inside the Task 4 range.

Final fix wave commits (`5d6561a..2a38cd7`): `3ca4de5`, `eacd114`, `b95da8d`, `22290e7`, `bac1385`, `addf3c8`, `2a38cd7`.

## Rulings

A ruling is a controller decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong. The labels are the ledger's.

- **P1. Execution order of the PR and the final review.** Run Tasks 1 to 12, then Task 13 steps 1 and 2 (push, open the PR, CI), then the whole-branch final review with the CI result in hand, then the single fix wave, then the merge and the log. *Why:* the plan put the merge before the SDD final review, but the review must precede the merge, and CI's render-smoke is the only real check of the unverified `.tf.json` syntax, so its result should feed the review. *Cost if wrong:* one extra CI cycle.
- **P2. The merge proceeds without a further ask.** *Why:* the owner approved this plan, whose Task 13 explicitly merges, and Phase 0 ran under standing autonomy for this project. *Cost if wrong:* a merged PR the owner wanted to inspect first (revertable).
- **Task 3. Do not descend into YAML alias nodes when indexing lines.** The reviewer found that `lineIndex` recursed through aliases with no cycle guard: `a: &a [*a]` caused an unrecoverable out-of-memory (reproduced), and alias fan-out is exponential. The fix indexes the alias at its use site only; errors inside aliased content then point at the use-site line through the ancestor fallback. Regression tests cover the cyclic alias and the merge key. *Why:* spec §4.7 requires every problem to be reported, and a crash reports none; the cyclic value itself is rejected later by the decoder as a diagnostic. *Cost if wrong:* an error inside aliased content points at the alias line instead of the anchor's inner line (arguably more useful).
- **Task 4 (1). Normalize the YAML tree at parse time.** The reviewer reproduced two defects: the decoder turns an unquoted `2024-01-01` into a timestamp, which the validator then saw as `"2024-01-01T00:00:00Z"` (the planning assumption that timestamps stay strings was wrong), and every `json.Marshal` failure was labeled "mapping keys must be strings" (for example `.inf`). Fix: a `normalize(file, root)` pass in `parseFile` that retags `!!timestamp` scalars (keys and values) to `!!str`, reports non-string mapping keys (the merge key `<<` is exempt) and non-finite floats as diagnostics with their line, and gives the Marshal fallback a neutral "unsupported YAML value" prefix. The "non-string keys" row moved from `TestValidate` to `TestParseFile`; aliases are not followed, consistent with the line index. *Why:* claims have no date type and the JSON model has neither non-string keys nor `.inf`/`.nan`, so parse time is the single correct place, with a line. *Cost if wrong:* a claim that used a YAML timestamp on purpose would get a string; no schema field is a date, so none does.
- **Task 4 (2). Pin the self-referential anchor behavior with a test.** Add a `TestValidate` case with source `a: &a [*a]` (kind Group) expecting a diagnostic at line 1 containing "cannot read document". *Why:* it pins the claim from the Task 3 fix that the decoder rejects self-referential anchors instead of hanging; the plan had no such test. *Cost if wrong:* none beyond one test case.
- **Task 5. A claim whose name mismatches its file is excluded from the model.** The reviewer found that such a claim stayed in the model and in the document map keyed by `Kind/<name>`; with a colliding real file (`api.yaml` declaring `name: orders` next to `orders.yaml`) its semantic diagnostics were attributed to the other file's lines. `checkFileName` now returns a bool and a mismatched claim is excluded from the model and from the semantic checks. Names then equal file stems, are unique per directory, and the keys cannot collide. A discriminating `TestLoadDiagnostics` row was added (`api.yaml` with `name: orders` plus a valid `orders.yaml` yields exactly the mismatch line). *Why:* the mismatch diagnostic already covers the file and render never runs with diagnostics. *Cost if wrong:* a mismatched claim's other errors show only after its name is fixed (one extra PR iteration).
- **Task 12. Add a complete Platform example to the claims reference.** The reviewer approved the doc with one Important finding: the Platform section had no example and omitted the required `apiVersion` and `kind`, so a reader following the table alone would write an invalid `config/platform.yaml`. A minimal example (`apiVersion: idp/v1`, `kind: Platform`, `github.org`, `github.writerAppId`, one environment) now sits above the table, like the Group and Component sections. *Why:* a reference doc must not lead to invalid files. *Cost if wrong:* none.
- **FW. One final fix wave, scoped.** After the whole-branch review ("with fixes": one Important and ten minors), a single wave covered:
  - F1: group memberships keyed by `lower(m.user)`, a validation rejecting logins equal ignoring case, and a `tofu` test (the Important finding).
  - F2: duplicate mapping keys detected in `normalize`, with the duplicate's own line instead of a line-1 decoder error.
  - F3: marker first in `WriteTree`, so a failed write no longer wedges the output directory.
  - F4: `.gitignore` adds `/tfstate/`, `*.tfstate*`, `*.tfplan`, `crash.log`.
  - F5: reject non-regular claim files and platform file (symlinks) before Phase 1b wires `validate` into fork-PR runs.
  - F6: CI module jobs use the shipped lock file (`-lockfile=readonly`), spec §7.5.
  - F7: descriptions must be single-line (schema pattern) to avoid perpetual drift.
  - F8: docs: `--out` is relative to the current directory, and state lands at `<out>/../tfstate/`.

  *Why:* all cheap now, and F1 and F5 get more expensive later (F1 because the map key becomes a state address at the first apply, F5 because of fork PRs). *Cost if wrong:* a slightly larger PR.
- **DEFER. Four minors are not fixed here; each lands in its phase plan through this log.** Minor #3 (removing the last topic may be a no-op with an optional and computed set) goes to the Phase 1c end-to-end step "remove all topics", fixed with `github_repository_topics` if confirmed. Minor #6 (annotation paths are relative to `--dir`, not to `GITHUB_WORKSPACE`) is a Phase 1b constraint. Minor #8 (GoReleaser `{{.Version}}` drops the `v`, so `?ref=0.1.0` would break) means Phase 1c must use `{{.Tag}}` plus a guard. Minor #11 (`TestLoadSortsClaimsByName` cannot fail) stays, because determinism holds by construction. *Cost if wrong:* rediscovery in 1b or 1c.
- **R-3.3. Platform config stays inside `internal/claims`.** Spec §3.3 lists a separate `internal/config` unit; the plan folded the Platform config into `internal/claims`. Keep it: one loader, one diagnostics path. Phase 2 may extract `internal/config` when `internal/naming` needs the Platform without the claims. *Cost if wrong:* a small package move in Phase 2.

Spec and ADR changes decided while planning and written in Task 1 (`8eb730c`):

- **A1 (spec §4.6).** Platform config gains `github.writerAppId` (required) and `naming.requiredPrefix` / `naming.reservedPrefixes` (optional). Rendering is pure (§5.1), so the bypass actor of every Component ruleset (§4.4) cannot be read from the environment and lives in the config; `naming` turns ADR-0014's prefix isolation into validation.
- **A2 (spec §5.3).** A Component receives its owner team as a reference to the Group module's output, not as a slug. A `data "github_team"` lookup by slug would fail on the first plan, when the team and the repository are created in the same apply; the reference also orders the apply (team first, then repository access and environment reviewers).
- **ADR-0015.** Claim parsing and validation libraries: `go.yaml.in/yaml/v3` for parsing with line positions and `santhosh-tekuri/jsonschema/v6` for validation, with the JSON Schemas as the single source of truth. Dependencies were verified to be at least 14 days old (`jsonschema/v6 v6.0.3`, 2026-06-28; `golang.org/x/text v0.42.0`, 2026-09-08; `go.yaml.in/yaml/v3 v3.0.5`, 2026-07-26).

## Notable findings caught by review

| Finding | Where caught | Fix |
|---|---|---|
| A self-referential YAML alias (`a: &a [*a]`) made the line index recurse until out-of-memory, reproduced by the reviewer; alias fan-out is also exponential | Task 3 review | `fe83eec`: the line index does not descend into aliases; regression tests for the cycle and the merge key; behavior pinned in Task 4 (ruling Task 3, Task 4 (2)) |
| The YAML decoder rewrote unquoted timestamps (`2024-01-01` became `"2024-01-01T00:00:00Z"`), so the validator saw a different value than the author wrote | Task 4 review | `3da353b..4885e2e`: `normalize` retags timestamps to strings (ruling Task 4 (1)) |
| Every JSON marshal failure was labeled "mapping keys must be strings" (for example `.inf`) | Task 4 review | Same range: non-string keys and non-finite floats reported with their line, neutral fallback message |
| A claim named differently from its file stayed in the model and borrowed another file's lines for its diagnostics | Task 5 review | `16311e0`: mismatched claims are excluded from the model (ruling Task 5) |
| The Platform reference had no example and omitted `apiVersion` and `kind` | Task 12 review | `5d6561a`: complete example added (ruling Task 12) |
| Group memberships were keyed by the login as written, so a case-only edit would plan a create and a delete of the same GitHub membership, unordered, possibly removing the user; the key becomes the state address at first apply | Whole-branch review (Important) | `3ca4de5` (F1): lowercase key, validation rejecting case-equal logins, `tofu` test |
| Duplicate mapping keys were reported at line 1 | Whole-branch review | `eacd114` (F2): reported at the duplicate's own line |
| A failed render write left a directory without its marker, so the next render refused until it was removed by hand | Task 9 review, fixed in the wave | `b95da8d` (F3): marker written first |
| Symlinked claim files could be read in a fork-PR run once Phase 1b wires `validate` in | Whole-branch review | `eacd114` (F5): non-regular files rejected |
| Module jobs resolved the provider freely instead of verifying the shipped lock file | Whole-branch review | `bac1385` (F6): `-lockfile=readonly` |
| Multi-line descriptions would cause perpetual drift | Whole-branch review | `addf3c8` (F7): single-line schema pattern |

Findings checked and found not to be problems: the numeric types of `reviewers.teams` and bypass `actor_id` (proven by the passing equality assertions); the CRLF risk on the golden file and the lock file (a tracked `.gitattributes` with `* text=auto eol=lf` overrides `core.autocrlf`); the claim that module tests run without a lock so the provider floats (each module's `versions.tf` pins `integrations/github = "6.13.0"` exactly); an excluded mismatched Group cannot cascade into "owner group does not exist" (the owner check reads the file-stem map, set before parsing).

## Final disposition of every finding

Statuses: **Fixed** (done, with the commits), **Phase 1b**, **Phase 1c**, **Won't fix** (consciously rejected, with the reason), **Obsolete** (resolved by later work or found not to be a problem). No finding is left open. "Final review" below means the whole-branch review.

### Fixed in the final fix wave (F1 to F8)

- Final review Important (case-sensitive membership keys) and Task 6 minor (module accepts duplicate or case-variant members): **Fixed**, F1, `3ca4de5`.
- Final review minor #2 and Task 3 minor (duplicate mapping keys reported at line 1): **Fixed**, F2, `eacd114`.
- Task 9 minor (`WriteTree` deletes the old render before writing, a failed write wedges the directory): **Fixed**, F3, `b95da8d`.
- Task 6 minor (`.gitignore` lacks `*.tfstate*`, `*.tfplan`, `crash.log`): **Fixed**, F4, `22290e7`.
- Final review minor #5 (non-regular claim files and symlinks): **Fixed**, F5, `eacd114`. Re-review noted a Lstat then ReadFile window, see Won't fix.
- Final review minor #4, CI module jobs not using the shipped lock file: **Fixed**, F6, `bac1385`.
- Final review minor #7 (multi-line descriptions cause drift): **Fixed**, F7, `addf3c8`.
- Final review minor #10 and Task 12 minor (`--out` default and relative base, state location not documented): **Fixed**, F8, `2a38cd7`.

### Phase 1b (pipelines)

- Final review minor #6: annotation paths are relative to `--dir`, not `GITHUB_WORKSPACE` (ruling DEFER). Phase 1b must emit workspace-relative paths.
- Task 5 minor: `read()` reports the raw `err.Error()` for non-NotExist failures, giving absolute OS-specific paths in messages; unwrap `*fs.PathError`. CI annotations need stable messages.
- Task 5 minor: test gaps (platform invalid plus bad component environments, empty claims directories, uppercase `.YAML`, duplicate names across files). Pipelines will gate PRs on these paths.
- Task 5 minor: contradictory naming is only detected when `requiredPrefix` equals a reserved prefix, not when a reserved prefix is a prefix of the required one. Fix with the pipeline validation rules.
- Task 5 minor: the mismatch regression row covers Components only; add the Group row.
- Task 2 minor: schema tests do not exercise regex boundaries (name length, case, trailing hyphen, login, prefixes, topics cap, AWS account id and region), so a pattern typo would pass. Add them once the schemas gate real PRs.
- Task 2 minor: `naming.requiredPrefix` and `reservedPrefixes` have no `maxLength`; add with the boundary tests.
- Task 3 minor: no tests for a comment-only file or a lone `---`.
- Task 9 minor: no tests for `out` being a regular file, shuffled member and topic order, or the group-description text.
- Task 10 minor: `--module-ref` and `--modules-dir` together silently prefer `--modules-dir`; make it a usage error when the workflows pass flags.
- Task 10 minor: `idp render` prints its summary as "validate: N problem(s)"; reword when the pipeline output is designed.
- Task 10 minor: test gaps (`-h`, cross-volume error, `GITHUB_ACTIONS != "true"`, render success with annotations).
- Task 12 minor: undocumented details (prefix format rules, org pattern, description limit of 350, optional Component environments, render exit codes, stdout and stderr destinations). The exit-code part lands with the exit-code ADR; the rest in the pipeline docs.

### Phase 1c (release and end-to-end)

- Final review minor #3: removing the last topic may be a no-op with an optional and computed set (ruling DEFER); add the E2E step "remove all topics" and fix with `github_repository_topics` if confirmed.
- Final review minor #8: GoReleaser `{{.Version}}` drops the `v`, so `?ref=0.1.0` would break (ruling DEFER); use `{{.Tag}}` plus a guard.
- E2E scenario "case-only login edit": confirm against the real org that renaming only the case of a member login plans no change (verifies F1 in practice).
- Task 6 minor: no test pins the outputs `team_id` and `slug` (unknown under the mock plan); needs an apply-mode run, which the E2E provides.
- Task 7 minor: the approvals assertion only runs with `required_approvals = 1` and would not catch a hard-coded 1; no tests for the non-integer and negative validation branches. Cover them in the apply-mode module tests.
- Task 8 minor: the package doc omits the exact `-platform=` flags used to regenerate the lock file; document them with the Renovate lock-file regeneration.
- Task 11 minor: render-smoke covers only local module sources; the git ref form cannot resolve before the tag exists, so verify it after the first release.

### Won't fix

- Task 1: spec §10 row "1 → 0.1.0" still lists the whole Phase 1 scope. The new split paragraph clarifies it.
- Task 1: ADR-0015 links the spec anchor `#47-validation`, which breaks silently on a rename. Headings are stable and the link is easy to repair.
- Task 2: no test asserts every `Kinds` entry has an embedded schema that compiles. `newValidator` compiles all kinds, which covers it.
- Task 3: an invalid second document is reported as "invalid YAML" rather than the multi-document message. It is still an error.
- Task 3: `yamlErrorLine` matches the first "line N" in the library text. The invalid-YAML test pins it, so a wording change fails CI.
- Task 4: `additionalProperties` with several unknown keys points the line only at the first. Fixing one reveals the next, and the message lists the key.
- Task 4: `kind.UnevaluatedProperties` is not special-cased. No schema uses it.
- Task 4: `TestValidate` passes on any matching diagnostic and there is no multi-error exact-set test. `TestLoadReportsEveryProblemAtOnce` covers "all at once" at loader level.
- Task 4: `validate` panics on an unknown kind. Callers only pass `schemas.Kinds`; it is an internal invariant.
- Task 4: an alias used as a mapping key is reported as a non-string key even when the anchor is a string. Exotic, and rejecting it is safe.
- Task 4: a float that fails decoding (for example `1e999`) is skipped by `normalize`; a later Marshal failure surfaces with the neutral message. Still reported.
- Task 6: the `name` variable is unvalidated although its description says it is the slug. The loader validates names upstream.
- Task 7: `default_branch` only feeds the deployment policy and the repository never sets its default branch. The interface is per plan and claims do not expose it.
- Task 7: `prevent_self_review` is not set on environments. The spec is silent.
- Task 8: the lock test counts `h1:` anywhere and cannot tell platforms apart; CI only exercises `linux_amd64`. Plan-mandated, and `-lockfile=readonly` (F6) now verifies hashes.
- Task 9: the marker check uses `os.Stat` (accepts a directory or symlink named `.idp-rendered`); `Lstat` plus a regular-file check would be tighter. The marker is only ever written by `render`, and a fake marker only lets a render overwrite a directory the user pointed at.
- Task 9: `ModulesDir` backslashes are not normalized. The CLI passes the slash form.
- Task 10: `modules-dir == <out>/github` yields `./.`. It works.
- Task 11: the new jobs have no `timeout-minutes`. They match the existing jobs.
- Final review minor #11: `TestLoadSortsClaimsByName` cannot fail (ruling DEFER). Determinism holds by construction.
- Final fix wave re-review: Lstat to ReadFile window (TOCTOU). A runner checkout has no concurrent writer.

### Obsolete

- Task 1: ADR-0015 says dependencies "are pinned at least 14 days old"; verified true in Task 2 (release dates above).
- Task 2: schema `default` keywords are annotations only. The loader applies defaults through Platform methods (Task 5).
- Task 3: "duplicate mapping keys silently accepted". The reviewer verified they are reported (at line 1); F2 fixed the line.
- Task 3: the fix comment claimed the decoder rejects self-referential anchors later. Pinned by a test in Task 4 (ruling Task 4 (2)).
- Task 5: an empty-string reserved prefix. The schema pattern `^[a-z][a-z0-9]*-$` forbids it, checked by the controller.
- Task 5: an excluded mismatched Group cannot cascade into "owner group does not exist". Verified, see above.
- Task 7: numeric types of `reviewers.teams` and bypass `actor_id`. Proven by the passing assertions.

Count: 8 Fixed entries (F1 to F8, covering 11 ledger findings), 13 Phase 1b, 7 Phase 1c, 21 Won't fix, 7 Obsolete; 56 entries in all.

## Carried into Phase 1b and 1c

From this phase:

- **Phase 1b:** annotation paths relative to `GITHUB_WORKSPACE` rather than `--dir` (ruling DEFER, minor #6). Rejecting symlinked claim files, which that phase depends on, is already done (F5, `eacd114`).
- **Phase 1c:** the GoReleaser `{{.Tag}}` guard (the `v` must survive in `?ref=`); E2E steps "remove all topics" and "case-only login edit".
- **Phase 2 candidate:** extract `internal/config` out of `internal/claims` when `internal/naming` needs the Platform without the claims (ruling R-3.3).

Inherited from Phase 0 (still open, see the [Phase 0 log](phase-0.md)):

- The **drift-workflow token ADR**: the reader token cannot see `bypass_actors`, so `check` fails closed on it; the ADR decides the token strategy before `drift.yaml` exists.
- The **exit-code ADR** (M1): drift versus error exit codes.
- The **pre-`tofu` passphrase check** from spike S1.
- The **Go patch pin via Renovate** (M9): CI pins an exact Go patch version.

The single-org prefix rules of ADR-0014 are implemented by A1's `naming` validation in this phase; the harness that refuses unprefixed deletes remains Phase 1 work.

## Process notes

- **Owner request, 2026-10-10: the docs describe the platform on its own terms.** In the middle of the run the owner asked that the documentation describe the platform on its own terms. The cleanup was done on `main` by commit `4da432e` (doc-only, so straight to main by policy), which also removed the comparison bullet from spec §9.3 and the ADR follow-ups tied to it, and the repository description was updated. `main` was then merged into the branch (`3da353b`, no conflict). The owner decided to leave the Git history unchanged, so no rewrite or force-push was done.
- **Process deviation recorded in the ledger.** In the final fix wave the implementer appended the F5 tests with a shell heredoc, which the owner's rules forbid. The file content was unaffected and the deviation was recorded when it was found.
- **Plan defects handled as rulings:** the plan's order of merge and final review (P1), and several plan-mandated behaviors that the reviews showed to be flawed (Tasks 3, 4, 5 and 12) were fixed rather than kept, because the spec's goals outrank the plan's code.
- **Whole-branch review coverage:** all eleven findings of the whole-branch review are dispositioned: the Important #1 and minors #2, #4, #5, #7 and #10 were fixed in the final fix wave (F1–F8, together with two deferred task minors); #3, #6, #8 and #11 are covered by ruling DEFER; #9 (the spec §3.3 `internal/config` unit folded into `internal/claims`) is ruling R-3.3.

## Status

**Phase 1a is complete.** `idp validate` and `idp render` shipped and are verified in CI, including a real `tofu init` and `tofu validate` on a rendered stack; both modules have `tofu test` suites that verify the provider against the shipped lock file. Every finding has a final disposition above. Nothing is reconciled automatically yet.

Next: Phase 1b (pipelines), which owns the drift-workflow token ADR, the exit-code ADR, the pre-`tofu` passphrase check and the workspace-relative annotation paths; then Phase 1c (release and end-to-end).
