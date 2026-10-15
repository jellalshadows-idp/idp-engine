# End-to-end harness

The harness drives `idp-claims-e2e` through the real pipelines at an engine commit and checks the results through the GitHub API ([ADR-0027](adr/0027-end-to-end-harness.md), spec §8.5). It is `cmd/idp-e2e`, run by the workflow `.github/workflows/e2e.yaml`.

## What a run does

Before step 1 it refuses to start while claims, repositories or teams named `e2e-r<id>-app` or `e2e-r<id>-team` from an earlier run exist, while a reconcile, drift or recover run of `idp-claims-e2e` is unfinished (they share the `idp-wet` queue), or while a "Wet push failed" issue is open.

1. Pins the four caller workflows of `idp-claims-e2e` to the commit under test (a PR, when they differ). That PR must plan `No infrastructure changes.`: anything else means claims the platform has not applied are on `main`.
2. Creates `e2e-r<run>-team` and `e2e-r<run>-app` through a PR, merges it after `idp-gate`, waits for the automatic reconcile, and checks the repository, team, memberships and environment.
3. Updates the description and removes every topic.
4. Changes only the case of a member login and expects `No infrastructure changes.`.
5. Runs the drift check and expects no drift issue.
6. Deletes the claims through a PR, approves that reconcile's `idp-approval` deployment, and checks the repository and team are gone.
7. Runs the drift check again.

Only steps 6 and the cleanup approve anything, and only the reconcile of their own merge commit. A reconcile of steps 1 to 4 that waits for `idp-approval` is rejected: nothing is applied, the queue frees, and the step fails.

A partial apply failure, the `recover` workflow and pull requests from forks are not covered; the Phase 1c smoke proved them (docs/phases/phase-1c.md).

## When it runs

Nightly at 03:41 UTC and on demand: `gh workflow run e2e.yaml --repo jellalshadows-idp/idp-engine --ref main`. One run at a time. Do not commit to `idp-claims-e2e` while a run is in progress.

## Credentials

Both live only in the `idp-engine` environment `idp-e2e` (deployment branch `main`):

| Name | What | Scope |
|---|---|---|
| `IDP_BOT_PRIVATE_KEY` (secret), `IDP_BOT_CLIENT_ID` (variable) | The bot App ([ADR-0026](adr/0026-automation-app.md)) | A token per run for `idp-claims-e2e` only: actions write, checks read, contents write, issues read, members read, pull requests write, workflows write |
| `IDP_E2E_APPROVER_TOKEN` (secret) | A fine-grained token of the test account `adrian-da-silva`, an org member and a second `idp-approval` reviewer in `idp-claims-e2e` only | `idp-claims-e2e` only: Actions read, Deployments read and write |

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

The approver account must be an org member and listed as an approver of `idp-claims-e2e`: `idp bootstrap apply --org jellalshadows-idp --claims-repo idp-claims-e2e --approver jellalshadows --approver adrian-da-silva …` ([bootstrap runbook](runbooks/bootstrap.md)).

### Rotating the approver token

Fine-grained tokens expire. Before the expiry date, sign in as `adrian-da-silva`, create a new token with the same scope, approve it in the org settings if asked, run the `gh secret set IDP_E2E_APPROVER_TOKEN …` line above, then delete the old token. A run with an expired token fails at its first approval.

## When a run fails

- The log of the `run the harness` step names the step and the cause.
- After a failure past the creation step, the harness deletes its claims through the platform (a delete PR, approved only if its reconcile asks). If that also fails, the run reports both errors and leaves the resources; finish the cleanup **through a PR in `idp-claims-e2e`** that deletes the `e2e-r<run>-*` claim files, and approve its `idp-approval` deployment if the plan lists only those resources. Never delete repositories or teams by hand.
- `an earlier run left its claims behind`: an `e2e-r<id>-*` claim file is on `main`. Delete it through a PR as above.
- `an earlier run left them behind`: a repository or team named `e2e-r<id>-*` exists. If its claim file is still on `main`, delete the claim through a PR. If not, the platform's delete did not finish: run the drift check and follow its issue, or [A wet push failed](runbooks/pipelines.md#a-wet-push-failed).
- `pipeline run(s) of the claims repo are still running or waiting`: let them finish. A run waiting on `idp-approval` needs a person: approve it if its PR plan is expected, otherwise reject it.
- `unexpectedly waits for approval; the harness rejected it`: the step's plan was destructive although the step expected it not to be. Read the plan comment of that step's PR; the cleanup that follows deletes the claims through the platform.
- `the pin PR must plan no infrastructure change`: claims on `main` are not applied. Find why in the latest reconcile run before running the harness again.
- The PR branches `e2e/<run>/<step>` stay in `idp-claims-e2e` as evidence; GitHub deletes merged branches automatically.
- An open "Wet push failed" issue in `idp-claims-e2e` stops the harness before it starts: follow [A wet push failed](runbooks/pipelines.md#a-wet-push-failed).
