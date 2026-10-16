# 0027. The end-to-end harness

- Status: Accepted
- Date: 2026-10-13
- Spec: [§8.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#85-level-4-e2e), [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main)

## Context

Phase 1c proved the pipelines once, by hand, in `idp-claims-e2e`. A repeatable check has to run without a person, including the approval of a destructive change. GitHub lets only a required reviewer approve a deployment, and reviewers are users or teams, never Apps.

## Decision

- `cmd/idp-e2e` (over `internal/e2e`, not part of the released CLI) drives `idp-claims-e2e` through the real pipelines at the engine commit under test:
  1. pins the four caller workflows to that commit, through a PR;
  2. creates a Group `e2e-r<run>-team` and a Component `e2e-r<run>-app` through a PR, checks the plan comment, merges, waits for the automatic reconcile and asserts the repository, team, memberships and environment through the API;
  3. updates the description and removes every topic, approves the update's reconcile for exactly the delete of the topics resource, and asserts them; *(Amended in Phase 1d execution, 2026-10-16.)*
  4. changes only the case of a member login, and expects `No infrastructure changes.`;
  5. runs the drift check and expects no drift issue;
  6. deletes the claims through a PR, expects the destructive warning, approves the `idp-approval` deployment of that reconcile run only, and asserts the repository and team are gone;
  7. runs the drift check again.
- Names are unique per run and always start with `e2e-`. The harness refuses any other name, touches only its own claim files, and never deletes a GitHub resource directly: after a failure it deletes its claims through the platform and reports both errors.
- Before a run, the harness refuses to start while any `e2e-` claim is on `main`, any `e2e-r` repository or team exists, or any reconcile, drift or recover run of the claims repo is unfinished. A reconcile of the harness's own merge that unexpectedly waits for approval is rejected (nothing is applied and the queue frees), never left waiting. *(Amended in execution, 2026-10-15.)*
- It runs nightly at 03:41 UTC and on demand from `idp-engine` `main`, in environment `idp-e2e` (deployment branch `main`), one run at a time (concurrency group `idp-e2e`). Release PRs join in Phase 1e.
- Credentials, both only in `idp-e2e`:
  - the bot App token (ADR-0026), minted for `idp-claims-e2e` only with actions write, checks read, contents write, issues read, members read, pull requests write and workflows write. Installation tokens expire after an hour and a run can last longer, so the harness mints and refreshes the bot's tokens from the App key, each scoped to the claims repo and the permissions the forge calls. *(Amended in execution, 2026-10-15.)*
  - the approver token: a fine-grained token of the org owner `jellalshadows`, limited to `idp-claims-e2e` with Actions read and Deployments write. *(Amended in execution, 2026-10-16.)*
- Not covered, and proven by the Phase 1c smoke: a partial apply failure, the `recover` workflow, pull requests from forks.

## Consequences

### Positive

- Every engine change can be proven against the real org, approval path included.
- A leaked approver token can approve deployments of the test repo and nothing else.

### Negative / costs

- A token that expires; the runbook covers rotation.
- A run takes tens of minutes of CI, creates and deletes real `e2e-` resources, and shares the org's rate limit.
- `idp-claims-e2e` belongs to the harness: people should not commit there while it runs.

## Alternatives considered

- **The owner's token.** Rejected: an org owner's token in CI is a takeover of the org if it leaks. *(Amended in execution, 2026-10-16.)* The rejection holds for a classic or broadly scoped token. A fine-grained token limited to the test repo and these two permissions has the same exposure as a second account's token, so the decision uses it.
- **A second account's fine-grained token.** The first choice; dropped in execution because no dedicated test account exists. Same exposure; it would tell the harness's approvals apart from the owner's in the audit log.
- **Approving by hand.** Rejected by the owner: a nightly run must finish on its own.
- **A custom deployment protection App.** Rejected: more machinery than a scoped token for one approval.
