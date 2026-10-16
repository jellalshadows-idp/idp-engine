# End-to-end harness

The harness drives `idp-claims-e2e` through the real pipelines at an engine commit and checks the results through the GitHub API ([ADR-0027](adr/0027-end-to-end-harness.md), spec §8.5). It is `cmd/idp-e2e`, run by the workflow `.github/workflows/e2e.yaml`.

## What a run does

Before step 1 it checks that the approver token can still read the runs of `idp-claims-e2e`, and refuses to start while claims, repositories or teams named `e2e-r<id>-app` or `e2e-r<id>-team` from an earlier run exist, while a reconcile, drift or recover run of `idp-claims-e2e` is unfinished (they share the `idp-wet` queue), or while a "Wet push failed" issue is open.

1. Pins the four caller workflows of `idp-claims-e2e` to the commit under test (a PR, when they differ). That PR must plan `No infrastructure changes.`: anything else means claims the platform has not applied are on `main`.
2. Creates `e2e-r<run>-team` and `e2e-r<run>-app` through a PR, merges it after `idp-gate`, waits for the automatic reconcile, and checks the repository, its active `idp-default-branch` ruleset, the team, its memberships, its `maintain` (not `admin`) grant on the repository, and the environment. The component's files are checked from Phase 4, when component templates exist.
3. Updates the description and removes every topic.
4. Changes only the case of a member login and expects `No infrastructure changes.`.
5. Runs the drift check and expects no drift issue.
6. Deletes the claims through a PR, approves that reconcile's `idp-approval` deployment, and checks the repository and team are gone.
7. Runs the drift check again.

Only steps 6 and the cleanup approve anything, and only the reconcile of their own merge commit. A reconcile of steps 1 to 4 that waits for `idp-approval` is rejected: nothing is applied, the queue frees, and the step fails.

The harness trusts what it approves only as far as it checked it:

- The plan comment must be the bot's own: a comment anyone else edited is refused, as the gate refuses it ([ADR-0018](adr/0018-plan-fingerprint-and-gate-trust.md)).
- The comment's fingerprint must be for the PR's head, and a delete's fingerprint may destroy only `e2e-r<run>-*` resources.
- Every merge is pinned to that head: GitHub refuses it if someone pushed to the branch.
- Step 6 starts only from the `main` its last merge left, which reconciled and then showed no drift.
- After a delete merges, its merge commit's first parent must be the `main` head the PR branched from. Otherwise someone else's commit reconciles with it: the harness rejects that reconcile if it waits for approval, and fails.

Each PR branch `e2e/<run>/<step>` is deleted by the harness right after its merge. After a failure, the harness closes the PRs it opened and did not merge; their branches stay as evidence.

A partial apply failure, the `recover` workflow and pull requests from forks are not covered; the Phase 1c smoke proved them (docs/phases/phase-1c.md).

## When it runs

Nightly at 03:41 UTC and on demand: `gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main`. One run at a time. Do not commit to `idp-claims-e2e` while a run is in progress.

Never re-run a run ("Re-run all jobs" or "Re-run failed jobs"): the new attempt reuses the run id, so its branches `e2e/<run>/<step>` collide with the first attempt's. The workflow refuses any attempt but the first; dispatch a new run instead.

The job's timeout (330 minutes) outlasts the harness's own bounded waits (about 300 minutes in the worst case, cleanup included), so GitHub never kills a run before those waits end.

## Credentials

Both live only in the `idp-engine` environment `idp-e2e` (deployment branch `main`):

| Name | What | Scope |
|---|---|---|
| `IDP_BOT_PRIVATE_KEY` (secret), `IDP_BOT_CLIENT_ID` (variable) | The bot App ([ADR-0026](adr/0026-automation-app.md)) | Installation tokens the harness mints itself and renews before the hour runs out, each for `idp-claims-e2e` only: actions write, checks read, contents write, issues read, members read, pull requests write, workflows write |
| `IDP_E2E_APPROVER_TOKEN` (secret) | A fine-grained token of `jellalshadows`, the org owner and the only `idp-approval` reviewer of `idp-claims-e2e` | `idp-claims-e2e` only: Actions read, Deployments read and write |

An installation token lives one hour and a run can last longer, so the harness mints the bot's tokens from the App key rather than receiving one. The key is passed only to the `run the harness` step, and the binary is built in an earlier step that sees no secret.

### Setting up

```bash
# The environment, with deployment branch main.
gh api -X PUT repos/jellalshadows-idp/idp-engine/environments/idp-e2e \
  -F 'deployment_branch_policy[protected_branches]=false' -F 'deployment_branch_policy[custom_branch_policies]=true'
gh api -X POST repos/jellalshadows-idp/idp-engine/environments/idp-e2e/deployment-branch-policies -f name=main -f type=branch
# The bot App credentials, from the files `idp bootstrap app --role bot` wrote.
gh secret set IDP_BOT_PRIVATE_KEY --env idp-e2e --repo jellalshadows-idp/idp-engine < ~/.idp/apps/jellalshadows-idp-bot.pem
gh variable set IDP_BOT_CLIENT_ID --env idp-e2e --repo jellalshadows-idp/idp-engine --body "<client id from jellalshadows-idp-bot.json>"
# The approver token: the owner pastes it; it is never written to disk.
gh secret set IDP_E2E_APPROVER_TOKEN --env idp-e2e --repo jellalshadows-idp/idp-engine
```

The approver must be listed as an approver of `idp-claims-e2e`, which `jellalshadows` already is: `idp bootstrap apply --org jellalshadows-idp --claims-repo idp-claims-e2e --approver jellalshadows …` ([bootstrap runbook](runbooks/bootstrap.md)).

A fine-grained token never exceeds the permissions it is granted, so a leaked approver token can approve or reject deployments of `idp-claims-e2e` and read its runs, and nothing else.

The harness team's member is `Adrian-Manuel`, the owner's second personal account and an org member.

### Rotating the approver token

Fine-grained tokens expire. Before the expiry date, sign in as `jellalshadows`, create a new token with the same scope, run the `gh secret set IDP_E2E_APPROVER_TOKEN …` line above, then delete the old token. A run with an expired token fails at preflight, before it creates anything: `the approver token cannot read the claims repo's runs (expired or missing Actions read?)`.

## When a run fails

- The log of the `run the harness` step names the step and the cause.
- After a failure past the creation step, the harness deletes its claims through the platform (a delete PR, approved only if its reconcile asks). If that also fails, the run reports both errors and leaves the resources; finish the cleanup **through a PR in `idp-claims-e2e`** that deletes the `e2e-r<run>-*` claim files, and approve its `idp-approval` deployment if the plan lists only those resources. Never delete repositories or teams by hand.
- A failure in step 6, the delete itself, gets no automatic cleanup. Finish the delete through a PR as above; if the delete PR already merged, see the `main moved` entries below, or [A reconcile waits for approval](runbooks/pipelines.md#a-reconcile-waits-for-approval) when its reconcile is still waiting.
- `re-running idp-e2e reuses its run id`: someone re-ran a run. Dispatch a new run.
- `the approver token cannot read the claims repo's runs`: the token expired or lost its Actions read permission. Nothing was created. Rotate it ([Rotating the approver token](#rotating-the-approver-token)).
- `mint a token for <owner>/<repo>: …`: the harness could not mint the bot's installation token. The cause is a wrong `IDP_BOT_CLIENT_ID` variable, a rotated or revoked key in `IDP_BOT_PRIVATE_KEY`, the bot App uninstalled from `idp-claims-e2e`, or a permission the App was not granted. The first token is minted in preflight, so the run fails before any claim is created and nothing is left behind. Fix the credential or the installation ([Setting up](#setting-up), [the bot App](runbooks/bootstrap.md#the-bot-app)) and dispatch a new run. If the message appears after preflight, a refresh failed mid-run once the cached token had a minute or less left; follow the cleanup advice above.
- `the plan comment on PR #<n> was edited by <login>`: someone edited the bot's plan comment on a harness PR, so the harness stopped before merging it. Find out who and why before the next run.
- `main moved since the harness's last merge`: someone committed to `main` of `idp-claims-e2e` during the run. The harness opened no delete PR, so the `e2e-r<run>-*` claims and resources stay. Read the foreign commit, then delete the claims through a PR as above.
- `main moved while PR #<n> was open: nothing was approved`: a commit landed on `main` while the delete PR was open, so the delete's reconcile also carried someone else's change. The harness rejected that reconcile if it waited for approval: the claim files are gone from `main`, but the resources may remain. Read the foreign commit, then dispatch a reconcile (`gh workflow run reconcile.yaml --repo jellalshadows-idp/idp-claims-e2e --ref main`) and approve its `idp-approval` deployment only if its plan destroys nothing but `e2e-r<run>-*` resources ([A reconcile waits for approval](runbooks/pipelines.md#a-reconcile-waits-for-approval)).
- `an earlier run left its claims behind`: an `e2e-r<id>-*` claim file is on `main`. Delete it through a PR as above.
- `an earlier run left them behind`: a repository or team named `e2e-r<id>-*` exists. If its claim file is still on `main`, delete the claim through a PR. If not, the platform's delete did not finish: run the drift check and follow its issue, or [A wet push failed](runbooks/pipelines.md#a-wet-push-failed).
- `pipeline run(s) of the claims repo are still running or waiting`: let them finish. A run waiting on `idp-approval` needs a person: approve it if its PR plan is expected, otherwise reject it.
- `unexpectedly waits for approval; the harness rejected it`: the step's plan was destructive although the step expected it not to be. Read the plan comment of that step's PR; the cleanup that follows deletes the claims through the platform.
- `the pin PR must plan no infrastructure change`: claims on `main` are not applied. Find why in the latest reconcile run before running the harness again.
- The harness deletes each PR branch `e2e/<run>/<step>` right after merging it; a branch it cannot delete is logged (`could not delete branch`), never fails the run, and stays until someone deletes it. It closes the PRs it opened and did not merge, and leaves their branches in `idp-claims-e2e` as evidence.
- An open "Wet push failed" issue in `idp-claims-e2e` stops the harness before it starts: follow [A wet push failed](runbooks/pipelines.md#a-wet-push-failed).
