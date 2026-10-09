# 0013. Phase 0 spike findings

- Status: Accepted (S1, S3); S2 pending
- Date: 2026-10-09
- Spec: [§10](../superpowers/specs/2026-10-08-idp-on-actions-design.md#10-roadmap), [§11](../superpowers/specs/2026-10-08-idp-on-actions-design.md#11-risks-verified-in-phase-0-or-by-e2e)

## Context

Phase 0 exit criteria (spec §10) require an ADR with measurements: the encrypted state round-trip through Git (S1), the GitHub provider with an App token including apply duration versus the 1-hour installation token and pending-invite behavior (S2), and floci replay time per stack (S3). Spec §11 lists six risks that these spikes verify or that later phases cover.

All spikes ran in a throwaway public repo, [`jellalshadows-idp/idp-spike`](https://github.com/jellalshadows-idp/idp-spike), which is archived when Phase 0 closes. This ADR is the only durable place the results live. The execution log is in [docs/phases/phase-0.md](../phases/phase-0.md).

This ADR is honest about what is measured and what is not. **S1 and S3 are measured. S2 has not run**: every S2 step needs a GitHub App token, and creating and installing a GitHub App requires browser actions that were not available in the session (ruling R15 in the Phase 0 log). The ADR is therefore only partly accepted, and the rows that depend on S2 are marked pending rather than guessed.

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

**Status: pending. Not run. No S2 number below exists yet.**

Why: S2 needs the writer App's installation token (and, for Step 8, the reader App's). Creating an App goes through GitHub's manifest flow, which needs a human to click "Create" in a browser, and installing it needs another browser click. `gh auth refresh -s admin:org` (needed by `idp bootstrap apply/check`) is a device flow that also needs a browser, and the org invitation for the test account must be accepted in a browser. None of this has an API, by GitHub's design. Both manifest flows are already running locally and waiting for the owner (ruling R17).

What is staged: `github/main.tf`, `.github/workflows/spike-github.yaml` and `.github/workflows/spike-reader-probe.yaml` are written as in the plan and pushed to the spike repo's main branch. Only the secret `SPIKE_TF_ENCRYPTION` exists in the repo; no `SPIKE_WRITER_*` or `SPIKE_READER_*` variables or secrets are set.

What will be measured (plan Tasks 13 and 15):

| Measure | Source step | Value |
|---|---|---|
| apply seconds for 10 resources, and `resources_in_state=10` | Task 15 Step 4 | pending |
| plan seconds (refresh of 10 resources) | Task 15 Step 5 | pending |
| destroy seconds | Task 15 Step 7 | pending |
| Ruleset bypass for the writer App (files committed after the ruleset exists) | Task 15 Step 4 | pending |
| Writing `.github/workflows/*` with the writer token (needs `workflows: write`) | Task 15 Step 4 | pending |
| `plan_exit` while the invite is pending (0 no diff, 2 diff), then after acceptance | Task 15 Steps 5 and 6 | pending |
| Extrapolation: seconds per resource x 450 resources (50 components) versus the 1-hour token | derived | pending |
| Reader probe: HTTP status of ten endpoints with a reader token | Task 15 Step 8 | pending |
| Reader probe: whether `bypass_actors` is present in each ruleset response | Task 15 Step 8 | pending |
| Live ruleset JSON with an owner token, and whether an empty bypass list comes back as `"bypass_actors": []` or is omitted | Task 13 Step 7 | pending |

The reader probe covers these endpoints on `idp-claims-e2e`: the repo, its rulesets, `environments/idp-approval`, its `deployment-branch-policies`, its `secrets`, `environments/idp-write/variables/IDP_WRITER_CLIENT_ID`, `actions/variables/IDP_READER_CLIENT_ID`, `actions/secrets/IDP_STATE_PASSPHRASE`, and the org's `actions/permissions/workflow` and `installations`.

Decision rules already fixed in the plan (they apply when the numbers arrive):

- Pending invite: if the pending plan exits 2 and the accepted plan exits 0, the diff is temporary and the fallback from spec §11 is adopted (members must already be org members, checked by validation). If both exit 0, record "no fallback needed".
- Token expiry: see the decision list below.
- Any 403/404 in the probe table means the reader manifest lacks a permission: fix `bootstrap/apps/reader.json` by PR, update the App's permissions in its settings, and accept the change on the installation before the production bootstrap (Task 17).
- `bypass_actors present = false` confirms finding I3 (below).
- A failure writing workflow files with `refusing to allow a GitHub App to create or update workflow` means the writer manifest lacks `workflows: write`; that is a Task 11 bug to fix by PR. A ruleset violation means the bypass does not work, which ADR-0009 depends on.

### Open question I3: `bypass_actors` and non-write tokens

The final whole-branch review (finding I3) noted that GitHub omits `bypass_actors` from ruleset responses for tokens without write access. A reader token running `idp bootstrap check` in the drift workflow may therefore see a ruleset with no bypass list and report it as drifted, or, depending on how absent fields are compared, miss a real bypass entry (check fails open on `idp-main` with a reader token).

The finding was parked (ruling R12), not fixed, because fixing it without data risks making `apply` non-idempotent if GitHub also omits an empty `bypass_actors` for admin tokens. It is routed to two places:

1. **Data:** Task 13 Step 7 (owner-token ruleset JSON, empty bypass list shape) and Task 15 Step 8 (reader probe, `bypass_actors` present or not).
2. **Decision:** a Phase 1 ADR decides the drift-check token strategy (for example, a token that can see bypass lists, or a documented limit of what the reader verifies) **before** `drift.yaml` exists.

Until then, `check` run with a reader token must not be trusted for ruleset bypass lists.

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

The plan wrote metrics only to `$GITHUB_STEP_SUMMARY`, which is a separate file per step and cannot be retrieved with `gh run view --log` or the jobs API. Every `>> "$GITHUB_STEP_SUMMARY"` in `spike-state.yaml` and `spike-floci.yaml` became `| tee -a "$GITHUB_STEP_SUMMARY"` so the values also reach the step log. This changed no behavior. An attempt to print the summary file in a final step printed nothing and was reverted. For S1 this cost two extra re-runs (three green runs in total). The same `tee` change is advisable for the two S2 workflows before they run; they were left exactly as written in the plan.

## Decisions taken from these numbers

- **Risk 1 (pending invite): pending S2.** The rule is fixed (above); no decision is taken until `plan_exit` is measured before and after acceptance.
- **Risk 2 (token expiry): pending S2.** The rule from the plan stands: if the extrapolated apply time (seconds per resource x 450 resources) is under 30 minutes, keep minted installation tokens; otherwise Phase 1 adopts the provider's `app_auth`, with the key file inside `idp-write`.
- **Risk 3 (CI minutes): decided from S3.** Replay cost per stack = init cached + apply v1 + plan v2 = 7 + 13 + 6 = **26 seconds**, plus 6 seconds of floci readiness once per job. Since the plugin cache showed no gain, using the cold init (6 s) would give 25 seconds; the difference is below the timer resolution. Only affected stacks are planned (spec §11), so the cost scales with changed stacks, not with the number of stacks in the org. No change to the design is needed; the figure is the baseline Phase 2 compares against.
- **Risk 4 (fidelity): no gaps found** for the IAM role and policy, the OIDC provider and the ECR repository. Nothing needs documenting and skipping in level 3 for these resources. The only observed difference is the `localhost:4566` ECR URL host (cosmetic, see above). Unverified: a real `AssumeRoleWithWebIdentity`, which Phase 2 covers.
- **Reader App permissions for running `check` in drift** are validated in Phase 1 and by the S2 reader probe. The REST permission schema exposes no `variables` permission, so a gap there is fixed by updating `reader.json` then. The final-review fix wave already added `actions:read` and `actions_variables:read` to the reader manifest after checking GitHub's server-to-server permissions table.
- **Finding I3 (bypass_actors visibility): open**, routed to the reader probe and to a Phase 1 ADR (above).

## Consequences

- Positive: the two riskiest storage and emulation assumptions (encrypted state in Git, multi-account floci with IAM/OIDC/ECR) hold, with measured numbers.
- Negative: Phase 0 cannot fully close until the App-dependent steps run. The exit criterion "an ADR with measurements ... the GitHub provider with an App token ... pending-invite behavior" is not yet met.
- Follow-up: when S2 runs, fill the S2 table, update the Status line to plain `Accepted`, and update the "Phase 0 result" cells for risks 1 and 2 in spec §11.

## Alternatives considered

- **Writing guessed S2 numbers or marking the ADR fully accepted.** Rejected: the owner wants the repo to be an honest record, and an invented number would be worse than a visible gap.
- **Simulating the GitHub provider against a fake API.** Rejected: the point of S2 is the real behavior of GitHub (ruleset bypass, workflow-file permission, pending invites, token lifetime), which a fake cannot show.

## References

- Spec §10, §11. Plan Tasks 13 to 17.
- Spike repo: https://github.com/jellalshadows-idp/idp-spike
- Runs: S1 37986095179, S3 37986181743.
