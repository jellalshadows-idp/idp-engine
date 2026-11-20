# Pipelines runbook

Every procedure here uses only GitHub: the Actions tab, `gh`, and the claims repo. None needs the state passphrase or an App key on your machine. In the commands, `<repo>` is the claims repo, for example `jellalshadows-idp/idp-claims`.

## idp-gate failed

Open the run's `pr / verdict` job: it says what failed.

- **"a maintainer must push this branch to the repo"**: the PR comes from a fork, so nothing is planned (spec §6.1). A maintainer pushes the branch to the claims repo and opens the PR from there.
- **`pr / validate` failed**: the annotations on the PR point at each claim error ([claims reference](../claims.md)).
- **`pr / plan` failed in "check the repository configuration"**: a secret or variable is missing. Re-run the bootstrap ([bootstrap runbook](bootstrap.md)), and check that the caller passes both secrets.
- **`pr / plan` failed in `tofu`**: read the log. An error while reading a resource usually means the reader App lacks a permission.
- A transient failure: re-run the failed jobs of the PR run, or push again.

## A reconcile waits for approval

The gate asked for approval: the plan deletes or replaces something, or has a change that the merged PR's plan comment did not show ([ADR-0018](../adr/0018-plan-fingerprint-and-gate-trust.md)). The plan is in the summary of the `reconcile / plan` job.

1. Read the plan.
2. Approve or reject the `idp-approval` deployment in the run.
3. Rejecting applies nothing now, but the change stays merged: the next reconcile plans it again, and the next PR's plan shows it, so the gate can apply it without approval once that PR merges (unless it is destructive). To drop the change, revert the claim in a PR.

While it waits, the run holds the `idp-wet` concurrency group: drift and other reconciles queue behind it. Decide promptly.

The saved plan and render live in the run's `idp-reconcile` artifact for one day. An approval given later fails when the apply downloads them; nothing is applied. Dispatch a new reconcile instead.

If you rejected the deployment and then use "Re-run failed jobs", GitHub asks for the approval again and the apply then refuses the second attempt. Dispatch a new reconcile instead.

## A reconcile failed

Never re-run a reconcile run: its jobs refuse a second attempt, because the plan may be stale. Dispatch a new one, which plans every stack against the current `wet`:

```bash
gh workflow run reconcile.yaml --repo <repo> --ref main
```

- **`apply-github` failed in "apply the saved plan"**: the state was still committed to `wet`, because real resources may have changed, and the old render stayed, so the next reconcile retries the stack. Fix the cause (a claims PR, or wait out a transient error), then dispatch. The new plan shows only what is left; the gate asks for approval when that is not in the last merged PR's plan.
- **`apply-github` failed in "refuse when wet moved since the plan"**: something committed to `wet` after this run planned. Dispatch a new reconcile.
- **An `apply-aws <stack>` job failed**: the other stacks still applied, and `apply-github` committed their renders; the failed stack kept its old render in `wet`, so the next reconcile retries it. Its job summary says why. "the plan differs from the gated one, so nothing was applied" means the re-plan was not the plan the gate decided on, so nothing was applied: dispatch a new reconcile, which plans and gates it again. An OpenTofu error in a replay or the plan means the stack does not apply even to an empty floci: fix the claims, or the engine if the render is wrong, then dispatch. Nothing real was changed: AWS stacks run only against floci, and their state ends with the job.
- **A `plan-aws <stack>` job failed**: the gate did not run and nothing was applied. Read its log, fix the cause, then dispatch.
- **`plan` failed in "split the affected stacks"**: the change affects a stack this engine version does not reconcile (a Workspace, until Phase 3), or more than 256 AWS stacks. Revert the claims change, or split it into smaller PRs.
- **The run opened a "Wet push failed" issue** (its wet-commit token or its commit to wet failed): do not dispatch. Follow [A wet push failed](#a-wet-push-failed); until the recovery runs, every reconcile refuses to plan.
- **The run was cancelled**: a cancelled run starts no new job, so the jobs after the one running do not start; an `apply-aws` leg that had not started applies nothing, and the next reconcile plans it again.
- **The run was cancelled during `apply-github`**: if it opened a "Wet push failed" issue, follow [A wet push failed](#a-wet-push-failed); the recovery accepts cancelled runs. **The runner was lost** (the run opened no "Wet push failed" issue, whether or not it has an `idp-wet-root` artifact): compare the newest `wet` commit with the run's log before dispatching; if resources were applied but never committed, the state has to be rebuilt with `import` blocks (spec §7.7), so open an issue in the engine repo.

## A wet push failed

The apply job opened an issue labelled `idp-wet-push`, titled "Wet push failed". What the job was about to commit is saved as the artifact `idp-wet-root` of that run, for 7 days, and the issue names its artifact ID on a line `idp-wet-root artifact id: <ID>`. An issue without that line (the wet commit was not saved as an artifact, or the run predates this engine version) cannot be recovered by the workflow: it refuses, and step 3 applies.

1. Run the recovery with the id of the run the issue names (the number in its URL):

   ```bash
   gh workflow run recover.yaml --repo <repo> --ref main -f run-id=<run id>
   ```

2. It checks that the run is a failed or cancelled reconcile of `main` and that the open issue names it, downloads the artifact by the ID the issue names (never by name), and refuses if `wet` moved since that run's plan. Otherwise it commits the paths the run listed in `wet-paths`: the saved state, the `github` render if its apply had succeeded, and the render of each AWS stack that applied, or the removal of an orphan's render; then it closes the issue. A wet root saved before `wet-paths` existed commits the `github` stack only, as before.
3. If it refuses because `wet` moved, the issue names no artifact ID, or the 7 days have passed, do not force anything: the saved state is older than `wet`'s, or there is no saved state the workflow can trust. The state has to be rebuilt with `import` blocks (spec §7.7); that runbook is not written yet, so open an issue in the engine repo.
4. If the recovery committed to `wet` but failed to close the issue, a second recovery refuses (wet moved). Check that the newest `wet` commit is "idp: recover the wet commit of run <id>", then close the issue by hand; that unblocks the reconcile.
5. After a successful recovery, dispatch a reconcile: claims merged while the reconcile was held are not in `wet` yet, and the drift check cannot see them.
6. Recover before you move the claims repo to a new engine version: a recovery runs the engine version the claims repo pins, and an issue opened before `0.2.0` names no artifact ID, so the `0.2.0` recovery refuses it.

## A drift issue is open

The drift check found that GitHub differs from the `wet` render, or that a bootstrap protection changed. The issue holds the plan summary and the bootstrap findings.

- Fix the claims in a PR, or dispatch the reconcile to put GitHub back to the claims. A remediation usually waits for approval: the gate applies automatically only changes whose address and action were in the last merged PR's plan.
- For bootstrap findings, re-run `idp bootstrap apply` ([bootstrap runbook](bootstrap.md)).
- The issue closes on the next clean drift run. To run one now:

  ```bash
  gh workflow run drift.yaml --repo <repo> --ref main
  ```

## The drift schedule stopped

GitHub disables scheduled workflows in public repos after 60 days without activity. Re-enable it:

```bash
gh workflow enable drift.yaml --repo <repo>
```
