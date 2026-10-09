# 0009. Two GitHub Apps and split environments

- Status: Accepted
- Date: 2026-10-08
- Spec: [§7.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#73-two-github-apps-and-three-secret-scopes)

## Context

The pipelines need GitHub credentials at two very different levels. Plans on PRs need to read the org, repos, rulesets and environments to compare them with desired state. Applies need to write teams, repos, rulesets, environments and files, including `.github/workflows/` files installed by features. All of this happens in public repos where members who can push branches can propose claims, and a plan executes module code (spec §7.1). The write credential must therefore be out of reach of PR runs.

GitHub gives two relevant mechanisms. A GitHub App yields short-lived installation tokens with explicit permissions. An Actions **environment** can hold secrets and gate jobs with required reviewers and a deployment-branch restriction, and on the Free plan those features exist only on public repos (§2.3, ADR-0010). A verified subtlety shapes the layout: required reviewers "approve workflow jobs that reference the environment", **per job**. Putting approval and the write key in one environment would mean one approval per job that touches it, and the secret would be exposed to the approval step.

Apps also need the right permission set: to write `.github/workflows/` they need the `Workflows` permission; `Contents` alone is rejected (verified).

## Decision

There are **two GitHub Apps** and **two environments**, with three secret scopes (spec §7.3):

| Credential | Permissions | Where it lives |
|---|---|---|
| `idp-reader` App key | Read only | Repo secret: PR plans, reconcile plan, drift |
| `idp-writer` App key | Write, **including `Workflows`** | **Only** in environment `idp-write` (deployment branch `main`, no reviewers) |
| State passphrase | none | Repo secret, needed to read state in plans |
| Approval | none | Environment `idp-approval` (required reviewers, `main`, no secrets) |

- `idp-reader` and `idp-writer` are roles. App names are unique across GitHub and limited to 34 characters, so the real Apps are `<org>-reader` and `<org>-writer` (for example `jellalshadows-idp-writer`, 24 characters). Both claims repos share the same two Apps, because Apps are installed per org.
- Approval lives in a single job that references `idp-approval` and holds no secrets; `apply-github` references `idp-write` (§6.2). This gives exactly one approval per run.
- Tokens are minted per job with `actions/create-github-app-token` and expire after 1 hour.
- The `wet` ruleset lets only `idp-writer` update it (§7.4).

## Consequences

### Positive

- PR runs, including plans that execute module code, only ever hold a read token.
- The write key is released only to a job on `main`, in the `idp-write` environment.
- One approval per run, from a job that holds no secrets.
- Reader permissions are narrow and were verified against GitHub's server-to-server permission data (for example, environments and deployment branch policies need `actions: read`, rulesets `metadata: read`).

### Negative / costs

- **Accepted risk:** the passphrase reaches PR plans. If it leaks, someone can read near-public state, but still cannot write to `wet` (§7.3).
- Installation tokens last 1 hour, so a longer apply is a known limitation. Phase 0 measures how long applies take; the fallback is the provider's `app_auth` with the key file inside `idp-write` (§11 risk 2).
- The Apps and keys are shared by `idp-claims` and `idp-claims-e2e` (ADR-0014).
- Creating an App through the manifest flow needs a person to confirm in the browser (ADR-0011). The `bypass_actors` list is returned only to callers with write access, so `idp-reader` cannot fully verify it (verified fact).

### Follow-ups

- Phase 0 measures apply duration against the token lifetime (ADR-0013).
- Bootstrap creates the Apps from versioned manifests and sets up both environments (§9.2, ADR-0011). Rotating App keys is a runbook (§7.7).

## Alternatives considered

- **One App with write permission for everything.** Rejected: PR plans execute module code, so a write key in their reach would let a branch pusher write.
- **A personal access token.** Not chosen: Apps give per-job short-lived tokens with explicit permissions, and are installed per org; a PAT is long-lived and tied to a person.
- **A single environment for approval and write key.** Rejected: required reviewers gate each job that references the environment, so approval would repeat per job and the secret would sit behind the approval job.
- **Repo-secret write key.** Rejected: any job, including PR runs, could read it.

## References

- Spec §2.3, §6.2, §7.1, §7.3, §7.4, §9.2, §11 (risk 2).
- Verified: required reviewers per job (GitHub docs, deployments-and-environments); App names (unique, 34 characters) and `Workflows` permission (GitHub docs "Choosing permissions for a GitHub App"); reader permission mapping from GitHub's server-to-server permission data.
