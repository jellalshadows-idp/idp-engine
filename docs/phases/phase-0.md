# Phase 0 execution log: bootstrap and spike

- Period: 2026-10-08 to 2026-10-09
- Plan: [2026-10-08-phase-0-bootstrap-and-spike.md](../superpowers/plans/2026-10-08-phase-0-bootstrap-and-spike.md)
- Spec: [2026-10-08-idp-on-actions-design.md](../superpowers/specs/2026-10-08-idp-on-actions-design.md) (§10 roadmap, §11 risks)
- Findings: [ADR-0013](../adr/0013-phase-0-spike-findings.md)
- Status: **complete** (two owner-only items remain, see "Status" at the end).

This file is the durable copy of the execution ledger that was kept while the plan ran. The ledger lived in a git-ignored scratch folder and is deleted later, so everything it recorded is reproduced here: the rulings, every task's review outcome, every deferred minor finding with its triage, the final review, the environment limits and what remains.

## Summary

Phase 0 delivers the `idp bootstrap` command (create the two GitHub Apps through the manifest flow, apply the org and claims-repo protections, check them for drift), CI, a bootstrap runbook, fourteen architecture decision records, and spike measurements. Both claims repos, `idp-claims-e2e` and `idp-claims`, are bootstrapped on the real org, idempotent and drift-free.

- Engine code (Tasks 2 to 12) was built task by task with TDD and a review after each task, then reviewed as a whole branch. It was merged through [PR #1](https://github.com/jellalshadows-idp/idp-engine/pull/1) with a merge commit, `fe78a1e`. CI was green on the first run (Go with `-race`, including the Linux-only chmod test; actionlint; zizmor clean).
- Spikes S1 (encrypted state through Git) and S3 (floci multi-account, OIDC/IAM/ECR, replay timing) ran in the throwaway repo [jellalshadows-idp/idp-spike](https://github.com/jellalshadows-idp/idp-spike): S1 run [37986095179](https://github.com/jellalshadows-idp/idp-spike/actions/runs/37986095179), S3 run [37986181743](https://github.com/jellalshadows-idp/idp-spike/actions/runs/37986181743). Numbers are in ADR-0013.
- Spike S2 (GitHub provider with the writer App token, pending-invite behavior, reader probe) ran after the Apps existed: apply 32 s for 10 resources, no fallback needed, see ADR-0013.
- The real runs found two defects that the fake GitHub could not show: R19 (the `update` rule shape made apply non-idempotent, fixed in [PR #2](https://github.com/jellalshadows-idp/idp-engine/pull/2), merge `c042915`) and I3 (`bypass_actors` hidden from the reader token, fixed in [PR #3](https://github.com/jellalshadows-idp/idp-engine/pull/3), merge `c062837`).
- ADRs 0001 to 0014 are written and accepted; 0013 holds the final S2 results.

## Timeline and tasks

"Review" is the review done after the task by a separate reviewer. "Fix rounds" counts rounds sent back to the implementer.

| Task | Outcome | Commits / PR | Review | Fix rounds |
|---|---|---|---|---|
| 0 Owner setup | Done: org `jellalshadows-idp` created by the owner on 2026-10-09. The `admin:org` scope was added later through the device flow in the browser-automation session | n/a | n/a | 0 |
| 1 Repo hygiene and publish | Steps 1 to 5 on main at `ce45402`; step 6 (public repo, push) done after the org existed | `ce45402`; repo created public | No task review (R2) | 0 |
| 2 Go module, CLI skeleton, CI | Done | `ce45402..0fd3ff9` | Clean | 0 |
| 3 GitHub REST client | Done | `0fd3ff9..60a8204` | Clean | 0 |
| 4 Subset diff (`Mismatches`) | Done | `60a8204..e66a91c` | Clean | 0 |
| 5 Config, credentials files, passphrase | Done | `e66a91c..340d5ba` | Clean | 0 |
| 6 Desired rulesets and environments (goldens) | Done | `340d5ba..8634ed3` | Clean | 0 |
| 7 Sealed secrets, fake GitHub, first apply steps | Done | `8634ed3..5468ae2` | Clean | 0 |
| 8 Rulesets and environments (apply and drift) | Done | `5468ae2..018f97c` | Needs fixes at `ec1042f` | 1 |
| 9 Secrets, variables, idempotency | Done | `018f97c..e8a72dd` | Clean | 0 |
| 10 `Check` (read-only drift report) | Done | `e8a72dd..07feb3b` | Clean | 0 |
| 11 App manifests and local manifest flow | Done | `07feb3b..2656315` | Needs fixes at `7d15d70` | 1 |
| 12 Wire `idp bootstrap`, runbook, merge | Done; merged via PR #1 | `2656315..c08ba7b`; merge `fe78a1e` | Clean | 0 |
| Final whole-branch review (Tasks 2 to 12) | Ready to merge with fixes; fix wave applied and re-reviewed | review of `ce45402..c08ba7b`; fixes `64a310b..f2178e5` | Important I1 to I4, minor M1 to M9 | 1 wave |
| 13 Create Apps, bootstrap `idp-claims-e2e` | Done. Apps created and installed in the browser session (reader id 5255573, writer id 5255579). First real apply created the repo, `wet`, 2 rulesets, 2 environments, 3 secrets and 2 variables. The second apply exposed the R19 defect; fixed, then the second apply printed only `kept existing secret` and `check` reported no drift. Live ruleset JSON captured (owner token: `bypass_actors` present even when `[]`) | fix `e7b1959`; [PR #2](https://github.com/jellalshadows-idp/idp-engine/pull/2) merge `c042915` | Task review of the fix: Approved | 0 |
| 14 Spike S1 | Done, run 37986095179 | spike repo | n/a | 2 re-runs for readable metrics |
| 15 Spike S2 and reader probe | Done: apply 32 s / 10 resources, plan 5 s pending and 3 s accepted (exit 0 both), destroy 18 s; writer bypass and Workflows permission work; reader probe 10 of 10 endpoints 200, `bypass_actors` not visible to the reader. Cleanup verified (see below) | spike repo (`745bec5` tee fix); runs in ADR-0013 | n/a | 0 |
| 16 Spike S3 | Done, run 37986181743, green on first attempt | spike repo | n/a | 0 |
| 17 Bootstrap `idp-claims` | Done from a clean main worktree at `c042915`: pre-check showed only "repo missing"; apply created everything; second apply printed only `kept existing secret`; `check` reported no drift (exit 0). Apps reused, no manifest change needed (reader probe 10 of 10) | main at `c042915` | n/a | 0 |
| PR #3 `bypass_actors` fail closed | Done: `check` reports one explicit finding per ruleset when the token does not return `bypass_actors` (ruling R20). Post-merge real `check` with the owner token: `idp-claims` and `idp-claims-e2e` no drift | `2a34e5b`, `0316e09`, `8063f05`, `1cb8cbd`; merge `c062837` | Approved; two minors closed in a follow-up | 1 follow-up |
| 18 ADRs, spec risk update, archive spike | Done: ADRs 0001 to 0012 and 0014 (`7c64600`, deepened in `0230a42`, ADR 0011 follow-up corrected in `b81c925`); ADR-0013 final, this log, spec §11 and README in the closing docs commit. The spike repo is archived at the end of Phase 0 | see Git history | n/a | n/a |

Other commits worth knowing: `64a310b` amended the spec and plan for the single-org decision and added ADR 0014 and this log to Task 18; `49c2413` added the reader probe and ruleset capture steps to the plan (ruling R12). PR #2 (task 13 fix) and PR #3 were merged with CI green.

## Rulings

A ruling is a controller decision that deviated from, or filled a gap in, the plan. Each has what, why, and the cost if it turns out wrong.

- **R1. Reorder execution.** Task 1 steps 1 to 5 ran on main (`ce45402`); Tasks 2 to 11 were implemented locally on `feat/phase-0-bootstrap`; Task 2 step 9 (push and PR) was deferred until the org existed and the owner approved Task 1 step 6. *Why:* the org did not exist yet and pushing is outward-facing. *Cost if wrong:* none beyond CI running later than planned.
- **R2. Task 1 had no task review.** License, readme and gitattributes ran in the controller session. *Why:* the owner approved manual and outward tasks in the main session, and the content is verbatim from the plan with no code. *Cost if wrong:* a meta-file typo caught later in the final review.
- **R3. Isolation is a feature branch in the main checkout, not a worktree.** *Why:* the plan mandates the branch name in Task 2 step 1 and the repo had no other in-flight work. *Cost if wrong:* low; switch to a worktree if parallel work appears.
- **R4. Every outward step stops for the owner's explicit OK.** Steps T1.6, T2.9, T12.7 and Tasks 13 to 18. *Why:* the plan's Global Constraints and the skill's stop classes. *Cost if wrong:* none. (Later superseded in practice by the owner's full-autonomy instruction of 2026-10-09; see R14 to R18.)
- **R5. Accept `go mod tidy` adding `golang.org/x/sys v0.48.0 // indirect` in Task 7.** *Why:* the plan's lone `go get` left `go.sum` incomplete (a plan defect); x/sys v0.48.0 is dated 2026-08-31 (at least 14 days old) and is a transitive requirement of the allowed x/crypto. *Cost if wrong:* none; it is exactly what minimal version selection picks.
- **R6. One extra runbook sentence under "Rotating a secret".** Rotating `IDP_STATE_PASSPHRASE` by delete plus apply makes existing encrypted state unreadable; an OpenTofu `fallback` migration is needed first (spec §7.2). *Why:* the Task 9 implementer and reviewer both flagged it and the plan's runbook omitted it. *Cost if wrong:* one extra doc line.
- **R7. Task 18 also writes the execution log (this file).** Every ruling with cost-if-wrong, every task's review outcome and fix rounds, every deferred minor with triage, and the spike numbers, committed before the SDD workspace is deleted, linked from the README. *Why:* the owner asked on 2026-10-09 to document everything; the ledger is git-ignored scratch. *Cost if wrong:* one extra doc file.
- **R8. Fix Task 11 findings I1 to I3 even though the plan's code mandated the flawed behavior.** The spec's goal (a reliable bootstrap a stranger can follow) outranks the plan's code. Fixes: graceful `srv.Shutdown(2s)` instead of `srv.Close`; an atomic once-guard before conversion (409 on duplicate callbacks); `chmod 0600` after write plus an error naming the created App and where to regenerate its key; plus `ReadHeaderTimeout` (a one-line gosec hardening). *Cost if wrong:* about 30 lines of deviation from the plan's code.
- **R9. Windows does not enforce 0600; document instead of coding ACLs.** The Task 12 runbook states that keys live under the user profile, which is ACL-restricted by default, and must never be placed in a shared folder. *Cost if wrong:* a key readable by other local admins on the owner's machine.
- **R10. Single-org mitigations are written into spec §8.5 as Phase 1 requirements.** A separate claims repo, name prefixes, a harness that refuses unprefixed deletes, and `idp-claims` rejecting prefixed names. *Why:* the owner chose one org (2026-10-09); these keep "never test against production state" true at the state level (see ADR-0014). *Cost if wrong:* the prefix rules add two validation checks in Phase 1.
- **R11. Final fix wave scope:** I1, I2 (plus `actions:read` and `actions_variables:read` on the reader), I4, M2 (check asserts the writer key is absent at repo level and that `idp-approval` has zero secrets), M4 and M6 runbook notes, M7 (local validation before any network call), and a passphrase-file creation how-to. *Why:* all cheap, and they land before Task 13 creates real Apps and secrets. The controller verified the I2 permission mapping against the github/docs `server-to-server-permissions.json`: `GET environments/{env}` and `deployment-branch-policies` need `actions:read`; repo variables need `actions_variables:read`; environment variables and secrets need `environments:read`; rulesets need `metadata:read`; org workflow permissions and installations need `organization_administration:read`. *Cost if wrong:* small doc and code churn.
- **R12. Park I3 (`bypass_actors` omitted for non-write tokens) and gather data first.** Task 15 gains a reader-probe job (mint a reader token, record status codes and whether `bypass_actors` is present for rulesets, environments, variables, secrets and org endpoints), and Task 13 step 7 captures the owner-token ruleset JSON. Phase 1 decides the drift-check token strategy in an ADR before `drift.yaml` exists. *Why:* fixing it without data risks making `apply` non-idempotent if GitHub omits an empty `bypass_actors` even for admins. *Cost if wrong:* the drift check is unreliable for ruleset bypass lists until Phase 1.
- **R13. Parked minors M1, M3, M5, M8, M9.** M1: exit codes for drift versus error (Phase 1 ADR). M3: `apply` on a private or empty repo returns a raw error (`check` reports it). M5: a manifest without `hook_attributes` (verify at Task 13 step 2, fix if GitHub rejects it). M8: the `"main"` literal. M9: CI pins an exact Go patch version (Renovate later). None block merge per the final reviewer.
- **R14. PR #1 merged with a merge commit, not a squash as the plan said.** *Why:* it keeps the per-task TDD commits and the SHAs that this log cites. *Cost if wrong:* a noisier main history; release-please in Phase 1 parses the individual conventional commits, which is arguably better.
- **R15. Proceed with every browser-independent task now.** Task 14 (S1), Task 16 (S3), the ADRs, the docs and this log ran; Tasks 13, 15 and 17 run as soon as the owner finishes the browser steps, reduced to one checklist. *Why:* no API exists for the four browser actions by GitHub's design. *Cost if wrong:* Phase 0 cannot fully close in this session.
- **R16. State passphrases generated locally.** Both (e2e and main) were generated with Python `secrets.token_urlsafe(32)` into `~/.idp/e2e.pass` and `~/.idp/main.pass` (ASCII, no BOM, never printed) instead of the owner generating them in a password manager. *Why:* the owner asked for full autonomy and the runbook's "password manager first" step cannot be done by Claude. *Cost if wrong:* if the owner never copies them into a password manager and deletes the files, the wet state becomes unreadable.
- **R17. Both `idp bootstrap app` flows started in the background** on fixed ports 8765 (reader) and 8766 (writer), so the owner's browser work is only clicking Create and Install; completion would notify the controller, which then continues Tasks 13, 15 and 17 unattended. *Cost if wrong:* two idle local listeners until the owner acts.
- **R18. Rewrite all ADRs in depth.** Context of 2 to 4 paragraphs, consequences split into positive, negative and follow-ups, each rejected alternative with its reason, and references to verified sources. *Why:* the owner wants exhaustive in-repo docs and terse ADRs fail the "repo explains itself" goal. *Cost if wrong:* longer docs.

- **R19. Fix the `update` rule shape by sending GitHub's canonical form, and pin live fixtures.** The real second apply reported `updated ruleset idp-wet (drift at [$.rules.update.parameters])`: GitHub returns the `update` rule without `parameters` when `update_allows_fetch_and_merge` is false. The desired `WetRuleset` now emits `{type: update}` with no parameters, and both live rulesets are pinned as `testdata/live-ruleset-idp-main.json` and `live-ruleset-idp-wet.json`, checked by `TestDesiredRulesetsMatchLiveGitHub` (RED then GREEN; the wet golden lost only `update.parameters`). Branch `fix/wet-ruleset-update-shape`, fix `e7b1959`, [PR #2](https://github.com/jellalshadows-idp/idp-engine/pull/2), merge `c042915`. Re-verified on real GitHub before merging. *Why:* loosening `Mismatches` would hide real drift. *Cost if wrong:* none; it matches the API schema, where `parameters` is optional for `update`.
- **R20. Close the I3 fail-open in Phase 0 instead of deferring it.** The data removed the risk that blocked the fix (R12): owner tokens always return `bypass_actors` (even `[]`), so an absent key can only mean "hidden from this token". `check` now reports that explicitly (fail closed; `null` counts as hidden; the wet ruleset yields exactly one message), and the runbook says to run `check` with a write-capable token. Branch `fix/bypass-actors-fail-closed`, [PR #3](https://github.com/jellalshadows-idp/idp-engine/pull/3), merge `c062837`. The Phase 1 ADR still decides the drift-workflow token strategy. *Cost if wrong:* a read-only drift check reports one explicit finding per ruleset until Phase 1.

Also decided by the owner (not numbered rulings):

- 2026-10-09, single org `jellalshadows-idp`, no sandbox org. Spec §3.1, §8.5 and §12 and plan Tasks 0 and 13 to 18 were amended (`64a310b`): `idp-claims-e2e` is isolated by its own `wet` branch, state and passphrase and by the `e2e-`/`spike-` prefix; the Apps are created once (Task 13) and reused (Task 17). ADR-0014 records it.
- 2026-10-09, full autonomy: continue to the end without asking permission, accept the org invitation.

## Review outcomes per task

No Critical findings were raised in any task review. Important findings:

- **Task 2 to 7, 9, 10, 12:** reviewed clean (only minors, below).
- **Task 8 (rulesets and drift), review at `ec1042f`: Important.** There were no negative drift tests: nothing proved that a removed rule or an extra bypass actor is detected, or that an extra (harmless) rule is tolerated. Fix round 1 added those tests (`ec1042f..018f97c`); review then clean.
- **Task 11 (App flow), review at `7d15d70`: needs fixes, three Important findings.** (I1) `srv.Close` can cut the callback response before it is sent. (I2) A duplicate callback re-ran the conversion despite the comment saying it would not. (I3) A save could orphan a created App's key on partial failure, and a pre-existing `.pem` kept its loose file mode. Fix round 1 (`7d15d70..2656315`, ruling R8) added the graceful shutdown, the atomic once-guard with a 409 for duplicates, `chmod 0600` after write, and an error that names the created App and where to regenerate its key. Review then clean.
- **Task 5 note:** the controller verified the commit attribution and that `ghapi` maps 404 to `ErrNotFound` (from Task 3). Task 3 also had its commit message verified for no attribution.
- **Verified by the controller during the run:** `asList` and `validConfig` exist (Task 7); the Task 10 helpers from Tasks 7 to 9 exist and the tests compile and pass; the Task 12 runbook sentence "second run prints only kept lines" matches the Task 9 log behavior.
- **Final whole-branch review** is summarized in its own section below.

## Deferred minor findings and triage

Statuses: **open** (still true, no decision), **parked** (consciously left, with the reason), **fixed** (done later). Items the final review or the fix wave did not touch remain open.

### Task 2
- `cli_test` does not assert that the other stream stays empty. Parked: the test shape is mandated by the plan.
- `ci.yaml` runs `go run actionlint@v1.7.12`, which downloads on every run. Parked: plan-mandated; Renovate or a pinned binary later.
- Verification obligation "CI (`-race`, actionlint, zizmor) unverified until first push": **fixed**, PR #1 CI was green on the first run.

### Task 3
- `http.Client` has no timeout (relies on the context). Open.
- `io.ReadAll` is unbounded. Open.
- A typed-nil `out` is accepted. Open.
- A 404 drops the response body message. Open.
- Untested paths: malformed JSON and context cancellation. Open.

### Task 4
- A nil desired value versus a missing key is not pinned by a test. Open.
- Path keys are not escaped. Open.
- `asList` is untested directly. Open.
- Arrays are compared positionally. Parked: by design, because `rulesetView` (Task 8) indexes rules by type.

### Task 5
- `LoadAppCredentials` does not validate the slug (empty or path separators) before joining the `.pem` path. Open.
- `UserID` may return `0, nil` when the id is missing. Parked: caught by `ValidateApply/Check`.
- No tests for a missing `.pem` or malformed JSON. Open.

### Task 6
- The `"main"` literal appears twice in `Environments` (no default-branch constant). Parked as M8 (ruling R13).
- `-update` golden mode is self-comparing. Parked: plan-mandated pattern; the goldens were compared against the brief by hand.
- Comment wording at `desired.go:73`. Open (cosmetic).
- GitHub's acceptance of `update_allows_fetch_and_merge` and `~DEFAULT_BRANCH`. **Fixed**: the real apply in Task 13 accepted `~DEFAULT_BRANCH`, but showed GitHub drops the `update` parameters when the flag is false (R19, PR #2).

### Task 7
- The idempotency promise is untested in Task 7. Parked: covered by Task 9's `TestApplyTwiceIsIdempotent`.
- The fake's generic PUT/PATCH route is broad and can mask a wrong HTTP method. Open.
- `ensureRepo` treats any 404 as "create". Open.

### Task 8
- Environment reviewer drift and a missing environment are untested in isolation. Open.
- The reviewer type (User versus Team) is not compared. Open.
- `rulesetIDs` may include org parent rulesets (consider `?includes_parents=false`). Open.
- No pagination. Open.
- Redundant `int64` conversion in a test. Open (cosmetic).
- A PUT followed by a policy failure leaves partial state, which self-heals on the next apply. Parked: documented behavior.
- The test name `TestRulesetDriftDetectsWeakening` also covers a tolerated case. Open (cosmetic).
- The fake versus real GitHub shape risk (Task 9 also): **fixed**, verified in Task 13. It found exactly one shape defect (R19); the rest matched.

### Task 9
- `secretSpec` and `variableSpec` helpers are near-duplicates. Parked: plan-mandated, Task 10 consumes both.
- `ensureSecrets` is long. Open.
- The "kept" log is asserted only for the repo-scoped passphrase. Open.
- Names are unescaped in paths. Parked: they are constants.
- A `PATCH` variable body that includes the name is accepted by the GitHub API. **Fixed**: confirmed by the real runs (second apply clean).

### Task 10
- Installations are not paginated, so more than 30 installs could be false-reported. Open.
- Installations are matched by slug, not App ID. Open.
- No tests for a missing `wet` branch or a missing ruleset. Open.
- A local `findings` variable shadows the type. Open (cosmetic).
- `Contains`-based assertions are loose. Open.

### Task 11
- Manifest HTML-escaping round trip is untested. Open.
- `FindStringSubmatch[1]` panics on a bad form. Open.
- A 502 echoes the API error to the browser. Open.
- `f.Log` is not nil-checked. Open.
- The org is not validated beyond its length. Open.
- A wildcard listener would give a `[::]` redirect. Open.
- State comparison is not constant-time. Open.
- After the fix round: the shutdown flush has no test; the chmod test only runs on Linux CI; there is a brief `WriteFile` to `Chmod` window; a failed conversion burns the once-flag (the flow ends anyway); an orphan `.pem` is not deleted (message-only, per ruling R8). All open or parked as stated.

### Task 12
- No happy-path CLI tests for `apply` and `check`, the exit-1-on-findings branch, the `GITHUB_TOKEN` fallback or `IDP_GITHUB_API` (could use `httptest`). Open.
- `--org` and `--role` share one error message. Open.
- A bad `--listen` returns exit 1, not 2. Open.
- `NotifyContext` is created on usage paths. Open.

### Final review minors (see the next section)
- Fixed: M2, M4, M6, M7. Parked: M1, M3, M5, M8, M9 (ruling R13). Together these cover M1 to M9. The ledger records only the short topic of each, not longer text.

### Final fix wave residuals
- The runbook's `read -rs P` should be `IFS= read -rs P` to preserve edge spaces. Open.
- The new CLI test ignores `os.WriteFile` errors. Open.
- The runbook lacks a note that existing Apps keep their old permissions after a manifest change (update them in the App settings and accept on the installation). Open; the Apps were created after the last manifest change, so it did not bite, and the reader probe (10 of 10) confirmed the permissions.
- Checked by the controller and confirmed: `GET environments/{env}/secrets` needs `environments:read`, which the reader has.

## Final whole-branch review

Reviewer: opus, over `ce45402..c08ba7b`. Verdict: ready to merge **with fixes**.

Important findings:

- **I1. A Windows-encoded passphrase (BOM or UTF-16) was accepted silently.** Fixed (`2b5631d`).
- **I2. The reader manifest lacked permissions that `check` needs.** Fixed (`6d4b10c`): added `actions:read` and `actions_variables:read` after verifying the mapping (ruling R11).
- **I3. `bypass_actors` is omitted for non-write tokens, so `check` can fail open on `idp-main` with a reader token.** Parked first (ruling R12), then confirmed by the reader probe and the owner-token capture, and **fixed** by failing closed (ruling R20, PR #3). The drift-workflow token strategy remains a Phase 1 ADR.
- **I4. The runbook was bash-only and used `~` on Windows.** Fixed (`f2178e5`).

Minor findings:

- **Fixed:** M2 (`4fcfa53`: `check` asserts the writer key is absent at repo level and `idp-approval` has zero secrets), M4 and M6 (runbook notes, `f2178e5`), M7 (`2a4012c`: local validation before network).
- **Parked (ruling R13):** M1 (drift versus error exit codes, Phase 1 ADR), M3 (apply on private or empty repo raw error), M8 (`"main"` literal), M9 (CI pins exact Go patch version). M5 (manifest without `hook_attributes`) was verified at Task 13 step 2: GitHub accepted both manifests, so it is **closed**.

Fix wave: commits `64a310b..f2178e5` (`2b5631d` I1, `6d4b10c` I2, `4fcfa53` M2, `2a4012c` M7, `f2178e5` docs I4/M4/M6). A scoped re-review found everything addressed and no new Critical or Important findings.

## Environment limits and the browser-automation session

At first the Claude in Chrome extension was not connected (no connected browsers), and four steps that GitHub only offers through a browser were blocked (ruling R15): creating the Apps through the manifest flow, installing them on all repositories, the `gh auth refresh -s admin:org` device flow, and accepting the org invitation for `Adrian-Manuel`.

On 2026-10-09 the owner connected Claude in Chrome, after first testing that the integration left no history in claude.ai (the owner checked the recents page and found nothing). The owner then confirmed the exact list of browser actions ("vale probado, adelante"). In that session Claude:

- created the reader App (id 5255573) and the writer App (id 5255579) through the manifest flows,
- installed both on `jellalshadows-idp` for all repositories (verified through `orgs/jellalshadows-idp/installations`),
- authorized the `admin:org` scope of the local gh token through the device flow.

Not possible: accepting the invitation for `Adrian-Manuel`. That account is not signed in to the browser's GitHub account switcher, and Claude cannot sign in (passwords are off limits), so the invitation stays an owner action.

Note on the two accounts: `Adrian-Manuel` is the owner's other account (its invitation is to be accepted). `adrian-da-silva` is the spike's test account; it stayed a non-member until the S2 pending-invite test, was invited by the spike, accepted through the API and then removed.

## Cleanup verification (S2)

- `DELETE orgs/jellalshadows-idp/members/adrian-da-silva` done; the members list contains only `jellalshadows` (the account still listed for about five seconds after the delete); `memberships/adrian-da-silva` returns 404.
- The `spike-component` repo and the `spike-team` team return 404 (destroy removed all 10 resources, `resources_in_state=0`).
- The pre-existing pending invitation for `Adrian-Manuel` was not created by the spike and was left untouched.

## Status

**Phase 0 is complete.** The `idp bootstrap` command shipped and ran against the real org; `idp-claims-e2e` and `idp-claims` are bootstrapped, idempotent and drift-free; spikes S1, S2 and S3 are measured and recorded in [ADR-0013](../adr/0013-phase-0-spike-findings.md); spec §11 risks 1 and 2 are closed; ADRs 0001 to 0014 are accepted. The spike repo `jellalshadows-idp/idp-spike` is archived at the end of Phase 0 (`gh repo archive jellalshadows-idp/idp-spike --yes`).

Owner-only items left:

1. Accept the org invitation for `Adrian-Manuel` at https://github.com/orgs/jellalshadows-idp/invitation. That account is not signed in to the browser, and Claude cannot sign in.
2. Copy `~/.idp/e2e.pass` and `~/.idp/main.pass` into a password manager (they were generated by Claude, ruling R16; if lost, the wet state encrypted with them cannot be read), then delete them together with `~/.idp/apps/*.pem`, following the runbook's "After bootstrapping" section.

Next: the Phase 1 plan (pipelines). It owns the drift-workflow token ADR (reader token cannot see bypass actors), the exit-code ADR (M1), the pre-`tofu` passphrase check from S1, and the single-org prefix rules from ADR-0014.
