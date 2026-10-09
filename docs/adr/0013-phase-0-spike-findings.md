# 0013. Phase 0 spike findings

- Status: Accepted
- Date: 2026-10-09
- Spec: [§10](../superpowers/specs/2026-10-08-idp-on-actions-design.md#10-roadmap), [§11](../superpowers/specs/2026-10-08-idp-on-actions-design.md#11-risks-verified-in-phase-0-or-by-e2e)

## Context

Phase 0 exit criteria (spec §10) require an ADR with measurements: the encrypted state round-trip through Git (S1), the GitHub provider with an App token including apply duration versus the 1-hour installation token and pending-invite behavior (S2), and floci replay time per stack (S3). Spec §11 lists six risks that these spikes verify or that later phases cover.

All spikes ran in a throwaway public repo, [`jellalshadows-idp/idp-spike`](https://github.com/jellalshadows-idp/idp-spike), which is archived when Phase 0 closes. This ADR is the only durable place the results live. The execution log is in [docs/phases/phase-0.md](../phases/phase-0.md).

All three spikes were measured against real systems (GitHub for S1 and S2, the floci emulator for S3). S2 could not run at first because it needs GitHub Apps, whose creation and installation need a browser; once the owner connected a browser session (see the phase log) the Apps were created and S2 ran. The real runs also exposed two defects in the bootstrap command that no unit test had caught; they are recorded below.

## Pins adopted (all at least 14 days old on 2026-10-08)

OpenTofu 1.12.6 · integrations/github 6.13.0 · hashicorp/aws 6.66.0 · floci 2.1.0 (digest in the Phase 0 plan) · actions as pinned in `.github/workflows/ci.yaml`. The spike runner was `ubuntu-24.04` with OpenTofu 1.12.6.

## S1 — encrypted state through Git

Final run: [37986095179](https://github.com/jellalshadows-idp/idp-spike/actions/runs/37986095179) (green). Two earlier green runs (37985906772, 37985993221) produced no readable numbers; see "Spike fixes" below.

| Measure | Value |
|---|---|
| apply_seconds (init + apply of one `terraform_data`) | 1 |
| state_bytes (1 resource) | 1057 |
| state file keys | `encrypted_data`, `encryption_version`, `lineage`, `meta`, `serial` |
| round-trip | ok (a fresh job planned with exit 0) |
| tamper | rejected |
| plaintext | refused |

Verdicts:

- **Round-trip: works.** State committed to Git and read back in a fresh job gave a clean plan. `encrypted_data` is the correct ciphertext field in 1.12.6; the warning in the plan about a possible `KeyError` did not trigger.
- **Tamper: detected.** Modifying the stored ciphertext fails with `failed to write backup file: decryption failed for all provided methods ... cipher: message authentication failed`. That is the AES-GCM authentication failure ADR-0002 relies on.
- **Plaintext: refused, but with a cryptic message.** With `TF_ENCRYPTION=""` and `enforced = true`, apply fails closed. The error is `Invalid expression ... A single static variable reference is required` on the `state {}` and `plan {}` blocks. The unresolved key-provider method makes the configuration invalid, so nothing is written in plaintext. The message does not say "the passphrase is missing".
- **Implication for Phase 1 error messaging.** The most likely real-world cause of this error is a missing or empty `TF_ENCRYPTION` / passphrase (for example a mis-wired secret). Phase 1 workflows must check for it **before** invoking `tofu` and fail with a self-explanatory message that names the missing secret and links the runbook. Relying on OpenTofu's own message would send users to debug HCL that is correct.
- The passphrase never left the temp file or the GitHub secret, and the temp file was deleted.

## S2 — GitHub provider with an App token (10 resources)

Runs (all in `idp-spike`, 2026-10-09): apply [37988071031](https://github.com/jellalshadows-idp/idp-spike/actions/runs/37988071031), plan with the invitation pending [37988175963](https://github.com/jellalshadows-idp/idp-spike/actions/runs/37988175963), plan after acceptance [37988242468](https://github.com/jellalshadows-idp/idp-spike/actions/runs/37988242468), destroy [37988298453](https://github.com/jellalshadows-idp/idp-spike/actions/runs/37988298453), reader probe [37988407675](https://github.com/jellalshadows-idp/idp-spike/actions/runs/37988407675).

| Measure | Value |
|---|---|
| apply (writer App token), 10 resources | 32 s, exit 0, `resources_in_state=10`; first attempt green, no spike-file fixes |
| plan with the invitation pending | 5 s, `plan_exit=0` ("No changes"), `resources_in_state=10` |
| plan after the invitation was accepted | 3 s, `plan_exit=0` |
| destroy | 18 s, exit 0, 10 destroyed, `resources_in_state=0` |
| Ruleset bypass for the writer App | works: `.github/CODEOWNERS` and `hello.yaml` were committed to main after a PR-required ruleset existed |
| Writing `.github/workflows/*` | works: `hello.yaml` was written (verified through the contents API); no "refusing to allow a GitHub App to create or update workflow" error. The writer manifest carries `workflows: write` |

Verdicts:

- **Writer bypass works.** The ruleset required a PR, and the writer App still committed to main. ADR-0009 depends on this.
- **The Workflows permission works.** The writer token can create workflow files, which Phase 1 needs for rendering `.github/workflows/*` into `wet`.
- **A pending invitation is harmless.** The plan exits 0 both with the invitation pending and after acceptance, so there is no permanent diff. The fallback of spec §11 risk 1 (validation requiring members to already be org members) is **not needed**. Caveat: the test account was invited through `github_team_membership`, and the plan refresh sees the membership as present in both states.
- **API detail for accepting an invitation.** The invited account accepted through `PATCH user/memberships/orgs/jellalshadows-idp -f state=active` (state became `active`). Under Git Bash on Windows the endpoint must be written **without** a leading slash, otherwise the path is rewritten to a local filesystem path.
- **Cleanup was verified.** The test account was removed from the org (`DELETE orgs/jellalshadows-idp/members/adrian-da-silva`; it still listed for about five seconds, then disappeared; its membership returns 404), and the `spike-component` repo and `spike-team` team return 404. A separate, pre-existing pending invitation for `Adrian-Manuel` (the owner's other account) was not created by the spike and was left untouched.
- Observation: `tofu` emitted a deprecation warning ("will be removed in a future version") in plan output. It was not investigated.

### Reader probe

Run [37988407675](https://github.com/jellalshadows-idp/idp-spike/actions/runs/37988407675) (green). A reader App token was minted and used against the endpoints that `idp bootstrap check` reads on `idp-claims-e2e`.

| Endpoint | Status |
|---|---|
| `repos/jellalshadows-idp/idp-claims-e2e` | 200 |
| `.../rulesets` | 200 |
| `.../environments/idp-approval` | 200 |
| `.../environments/idp-approval/deployment-branch-policies` | 200 |
| `.../environments/idp-approval/secrets` | 200 |
| `.../environments/idp-write/variables/IDP_WRITER_CLIENT_ID` | 200 |
| `.../actions/variables/IDP_READER_CLIENT_ID` | 200 |
| `.../actions/secrets/IDP_STATE_PASSPHRASE` | 200 |
| `orgs/jellalshadows-idp/actions/permissions/workflow` | 200 |
| `orgs/jellalshadows-idp/installations` | 200 |

- **10 of 10 endpoints returned 200.** No 403 or 404, so the reader manifest needs no further permission fix. This **validates the final-review I2 fix** (`actions:read` and `actions_variables:read` on the reader), made after checking GitHub's server-to-server permission table.
- **`bypass_actors` was not present** in the ruleset responses for `idp-main` or `idp-wet` when read with the reader token. The reader token cannot see bypass actors (see defect b below).

## Defects found by the real runs

The bootstrap command passed its unit tests against a fake GitHub. Running it against the real org found two defects in assumptions the fake could not check.

### (a) R19: the `update` rule shape made apply non-idempotent

On the first real bootstrap of `idp-claims-e2e`, the second `apply` printed `updated ruleset idp-wet (drift at [$.rules.update.parameters])`. GitHub returns the `update` rule **without** `parameters` when `update_allows_fetch_and_merge` is false (the default), while the engine sent `{"update_allows_fetch_and_merge": false}`. The drift comparison therefore always saw a difference, and every apply rewrote the ruleset.

The fix (PR #2, merge `c042915`, code commit `e7b1959`) makes the desired `wet` ruleset emit GitHub's canonical shape, `{type: update}` with no parameters, and pins the two live rulesets captured from GitHub as fixtures (`internal/bootstrap/testdata/live-ruleset-idp-main.json` and `live-ruleset-idp-wet.json`) with `TestDesiredRulesetsMatchLiveGitHub`. The test was red on `idp-wet` at `[$.rules.update.parameters]` before the fix and green after. The alternative of loosening the subset comparison was rejected because it would hide real drift. After the fix the second apply printed only `kept existing secret` lines and `check` reported no drift.

Side result of the same capture: extra parameters that GitHub adds to the pull-request rule (`dismissal_restriction`, `require_extra_approval_for_unattributed_changes`, `required_reviewers`) are ignored by the subset comparison, as intended.

### (b) I3: `bypass_actors` visibility

The final review (finding I3) suspected that GitHub hides `bypass_actors` from tokens without write access. The real captures settled it: **owner tokens always return `bypass_actors`, even when it is empty (`[]`), and the reader token never does.** An absent key can therefore only mean "hidden from this token", never "empty".

The fix (PR #3, merge `c062837`; commits `2a34e5b`, `0316e09`, `8063f05`, `1cb8cbd`) makes `check` fail closed: when the key is absent or `null` it reports one explicit finding per ruleset (`$.bypass_actors (not returned to this token; GitHub only shows bypass actors to callers with write access)`) instead of silently passing. The generic mismatch for that path is dropped so each ruleset yields exactly one message, and the runbook now says to run `check` with a write-capable token. After merging, a real `check` with the owner token reported no drift for `idp-claims` and `idp-claims-e2e`.

## S3 — floci

Run: [37986181743](https://github.com/jellalshadows-idp/idp-spike/actions/runs/37986181743) (green on the first attempt, no resource removed).

| Measure | Value |
|---|---|
| floci_ready_seconds | 6 |
| init cold (seconds) | 6 |
| init cached (seconds) | 7 |
| apply v1 (seconds; OIDC provider, ECR repo, IAM role, role policy) | 13 |
| plan v2 (seconds; exit code 2, one in-place update as intended) | 6 |
| Account isolation | ok (account `000000000003` cannot get the role of the dev account) |
| role_arn | `arn:aws:iam::000000000001:role/api-dev-ci` |
| repository_url | `000000000001.dkr.ecr.eu-west-1.localhost:4566/api` |
| Fidelity gaps | none |

Notes:

- **The ECR URL format differs from AWS.** floci returns `<account>.dkr.ecr.<region>.localhost:4566/<name>`, not `<account>.dkr.ecr.<region>.amazonaws.com/<name>`. This is a cosmetic emulator difference. **Deterministic identities (ADR-0006) are unaffected**: they are computed from the platform config by convention, not read back from the emulator, so the value the GitHub stack receives is the conventional one. Docs and tests must remember that anything compared against emulator output uses the `localhost:4566` host form.
- **The plugin cache gave no gain at 1-second resolution.** Cached init (7 s) was not faster than cold init (6 s). With whole-second timing and a single sample this is noise-level; the honest reading is "no benefit observed", so the replay estimate below does not assume one.
- **Not verified by the spike:** that the apply actually exercised `sts:AssumeRoleWithWebIdentity`. Only resource creation and policy storage were tested. The OIDC provider existing and the role carrying its trust policy is confirmed; a real assume-role is left to the Phase 2 floci integration tests.
- Multi-account selection by the 12-digit access key ID (ADR-0004) works: the account with ID `000000000003` could not read the role created in `000000000001`.

### Spike fixes (measurement only)

The plan wrote metrics only to `$GITHUB_STEP_SUMMARY`, which is a separate file per step and cannot be retrieved with `gh run view --log` or the jobs API. Every `>> "$GITHUB_STEP_SUMMARY"` in `spike-state.yaml` and `spike-floci.yaml` became `| tee -a "$GITHUB_STEP_SUMMARY"` so the values also reach the step log. This changed no behavior. An attempt to print the summary file in a final step printed nothing and was reverted. For S1 this cost two extra re-runs (three green runs in total). The same `tee` change was applied to the S2 workflows.

## Decisions taken from these numbers

- **Risk 1 (pending invite): closed, no fallback.** The plan exits 0 with the invite pending and after acceptance, so no permanent diff exists. Group members need not already be org members for the plan to stay clean.
- **Risk 2 (token expiry): closed.** 32 s for 10 resources is 3.2 s per resource; 3.2 s × 450 resources (50 components) is about 24 minutes, under the 30-minute threshold set against the 1-hour installation token. Keep minted installation tokens; do **not** adopt the provider's `app_auth`.
- **Risk 3 (CI minutes): decided from S3.** Replay cost per stack = init cached + apply v1 + plan v2 = 7 + 13 + 6 = **26 seconds**, plus 6 seconds of floci readiness once per job. Since the plugin cache showed no gain, using the cold init (6 s) would give 25 seconds; the difference is below the timer resolution. Only affected stacks are planned (spec §11), so the cost scales with changed stacks, not with the number of stacks in the org. No change to the design is needed; the figure is the baseline Phase 2 compares against.
- **Risk 4 (fidelity): no gaps found** for the IAM role and policy, the OIDC provider and the ECR repository. Nothing needs documenting and skipping in level 3 for these resources. The only observed difference is the `localhost:4566` ECR URL host (cosmetic, see above). Unverified: a real `AssumeRoleWithWebIdentity`, which Phase 2 covers.
- **Reader App permissions: validated** by the reader probe (10 of 10 endpoints return 200).
- **Finding I3: closed in Phase 0** by failing closed (PR #3). What remains is the strategy decision below.

### Phase 1 follow-ups

- **ADR on the drift-workflow token strategy for `check`.** The reader token cannot see bypass actors, so a read-only drift run reports one explicit finding per ruleset. Phase 1 must decide, before `drift.yaml` exists, between a token that can see bypass lists and a documented limit of what the reader verifies.
- **M1: exit codes for drift versus error.** `check` should distinguish "drift found" from "could not check" (Phase 1 ADR).
- **Plaintext-state error handling (S1).** Check `TF_ENCRYPTION` / the passphrase before invoking `tofu` and fail with a message that names the missing secret.
- **Single-org mitigations (ADR-0014, spec §8.5).** The prefix rules and the harness refusal of unprefixed deletes land in Phase 1.
- **Other parked minors that matter for Phase 1:** M3 (`apply` on a private or empty repo returns a raw error), M5 (manifest without `hook_attributes`; GitHub accepted the manifests in the real run, so no change was needed), M8 (`"main"` literal, no default-branch constant), M9 (CI pins an exact Go patch version; Renovate later), unpaginated installation and ruleset listings, and installations matched by slug rather than App ID. The full list with status is in the phase log.

## Consequences

- Positive: the riskiest storage, GitHub and emulation assumptions (encrypted state in Git, App-token apply with ruleset bypass, multi-account floci with IAM/OIDC/ECR) hold, with measured numbers. Risks 1 and 2 are closed without fallbacks.
- Positive: running against the real systems found and fixed two defects that the fake could not show, and pinned live fixtures now guard against a repeat.
- Negative: a drift check run with a read-only token cannot verify ruleset bypass lists and reports that explicitly until Phase 1 decides the token strategy.
- Follow-up: the Phase 1 items above.

## Alternatives considered

- **Writing guessed S2 numbers or marking the ADR fully accepted before S2 ran.** Rejected at the time: the owner wants the repo to be an honest record, and an invented number would be worse than a visible gap. The numbers above are measured.
- **Simulating the GitHub provider against a fake API.** Rejected: the point of S2 is the real behavior of GitHub (ruleset bypass, workflow-file permission, pending invites, token lifetime), which a fake cannot show. The two defects above are the proof.
- **Loosening the drift comparison to hide the `update` rule difference (R19).** Rejected: it would hide real drift; sending the canonical shape fixes the cause.
- **Deferring the `bypass_actors` fail-open to Phase 1 (R12).** Superseded by R20 once the data showed that an absent key can only mean "hidden".

## References

- Spec §10, §11. Plan Tasks 13 to 17.
- Spike repo: https://github.com/jellalshadows-idp/idp-spike
- Runs: S1 37986095179, S3 37986181743, S2 37988071031 / 37988175963 / 37988242468 / 37988298453, reader probe 37988407675.
- Pull requests: PR #2 (R19 fix), PR #3 (I3 fix).
