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
3. Rejecting applies nothing; the next reconcile plans again.

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
- **`apply-github` failed in "commit to wet"**: do not dispatch. Follow [A wet push failed](#a-wet-push-failed); until the recovery runs, every reconcile refuses to plan.
- **The run was cancelled or its runner was lost during the apply**: compare the newest `wet` commit with the run before dispatching. If `wet` has no commit from that run but the run's log shows applied resources, treat it as a failed wet push.

## A wet push failed

The apply job opened an issue labelled `idp-wet-push`, titled "Wet push failed". What the job was about to commit is saved as the artifact `idp-wet-root` of that run, for 7 days.

1. Run the recovery with the id of the run the issue names (the number in its URL):

   ```bash
   gh workflow run recover.yaml --repo <repo> --ref main -f run-id=<run id>
   ```

2. It checks that the run is a failed reconcile of `main`, downloads the artifact, and refuses if `wet` moved since that run's plan. Otherwise it commits the saved state, and the render if the apply had succeeded, and closes the issue.
3. If it refuses because `wet` moved, or the 7 days have passed, do not force anything: the saved state is older than `wet`'s. The state has to be rebuilt with `import` blocks (spec §7.7); that runbook is not written yet, so open an issue in the engine repo.
4. If the recovery committed to `wet` but failed to close the issue, a second recovery refuses (wet moved). Check that the newest `wet` commit is "idp: recover the wet commit of run <id>", then close the issue by hand; that unblocks the reconcile.
5. A run that was cancelled (not failed) is refused by the recovery even if its push failed. Treat it like step 3.

## A drift issue is open

The drift check found that GitHub differs from the `wet` render, or that a bootstrap protection changed. The issue holds the plan summary and the bootstrap findings.

- Fix the claims in a PR, or dispatch the reconcile to put GitHub back to the claims. A remediation is not in the last merged PR's plan, so it waits for approval.
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
