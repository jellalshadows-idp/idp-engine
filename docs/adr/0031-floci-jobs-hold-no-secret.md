# 0031. Jobs that run floci hold no secret

- Status: Accepted
- Date: 2026-11-18
- Spec: [§6.1](../superpowers/specs/2026-10-08-idp-on-actions-design.md#61-pr-pull_request--main), [§6.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#62-reconcile-push--main-and-workflow_dispatch-on-main) (amendment A14)

## Context

Phase 2e plans and applies the AWS stacks in the pipelines, each in a job with a floci of its own (spec §6.1 step 4, §6.2 step 3). floci's ECR runs a registry container, so the floci service mounts the runner's Docker socket (ADR-0004). Whoever holds that socket is root on the runner: a floci image that misbehaved could read every file and every environment variable of its job. The `github` stack's jobs hold the state passphrase, the reader or writer App's key, and tokens minted from them. `tofu show -json` prints every value a plan sets, in plaintext, so a full plan of the `github` stack is a secret too.

## Decision

- A job that runs floci (`plan-aws`, `apply-aws`) holds no secret: no `secrets.*`, no App token, no `GITHUB_TOKEN` beyond `contents: read`, no environment, and no `TF_ENCRYPTION` (`idp aws-stack` refuses to run when it or `IDP_STATE_PASSPHRASE` is set). Its floci is CI's, pinned by digest. It saves no cache and restores none.
- The jobs that hold secrets never run floci. The `github` stack is planned and applied in `plan` and `apply-github`, as before.
- What crosses jobs is public or reduced: the AWS renders (`idp-renders`), each stack's reduced plan (`idp-plan-<stack>`, from `idp plan-reduce` and `idp aws-stack plan`: the format version and each change's address and actions, which is all the fingerprint reads), the gated fingerprint (`idp-gated`), and the applied-stack markers (`idp-applied-<stack>`). The `github` stack's full plan never leaves its job. Plan artifacts are kept for 1 day.
- Every artifact a later job trusts comes from a job that runs no floci, and is downloaded by the `artifact-ids` its producing job outputs, never by name or pattern: the `github` stack's reduced plan, the renders (`idp-reconcile`, `idp-renders`) and the gated fingerprint (`idp-gated`). Another job cannot set a job's outputs, and an artifact deleted and uploaded again under the same name gets a new ID. Only what the floci jobs upload is downloaded by pattern: the AWS stacks' reduced plans and the applied-stack markers. A plan download never merges artifacts (no `merge-multiple`): each lands in its own directory, and `idp plan-summary --plan-dir` reads a stack's plan only from the directory of its own `idp-plan-<stack>` artifact, so a file that another artifact holds is never read.
- `idp plan-summary`, the PR comment and `idp gate` run in secret-less jobs (`comment`, `gate`) over the reduced plans. `verdict` and the caller's `idp-gate` stay the only required check (ADR-0022); approval stays one secret-less job (spec §6.2 step 2).

## Consequences

### Positive

- A compromised or buggy floci image reads nothing but public renders and an emulator, and what it can forge is bounded. It cannot change the `github` stack's reduced plan, renders, saved plan or gated fingerprint, which go by ID. A forged AWS reduced plan can only make the gate decide on changes that are not the real ones, and `apply-aws` then refuses, because it re-plans and applies only a plan equal to the gated one. So ADR-0018's premise, that the gate's input is the plan that will apply, holds for the `github` stack through the IDs and for the AWS stacks through that equality.
- The PR comment and the gate read one format for every stack, and an artifact of a public repo exposes no plan value.

### Negative / costs

- More jobs: each AWS stack costs a job with its own `idp` setup and floci start (about 30 s before the first OpenTofu command), and the gate moves out of `plan` into its own job. The reconcile's `apply-github` waits for every `apply-aws` job.
- What a job that runs floci still holds is its run's runtime token, which every job has: with it, a floci image that misbehaved could upload a forged artifact for the jobs after it, or delete one and upload it again under the same name. That is why trusted artifacts go by ID. A forged applied-stack marker could still make `wet` record the render of an AWS stack that did not apply; in the emulator, that changes only what the next replay starts from. The image is pinned by digest and moves only through a reviewed Renovate PR (spec §7.5), and no job restores a cache.
- An `apply-aws` job cannot open an issue or write anything to GitHub; a failed one shows as a failed run, and its render stays old in `wet`, so the next reconcile retries it (spec §6.2 step 4).
- Going to real AWS changes this: a job then needs cloud credentials, through OIDC, and a real account's apply can no longer share a runner with an emulator, nor trust a marker that a job running one uploaded.

## Alternatives considered

- **Plan the AWS stacks in the `plan` job, beside the `github` stack.** Rejected: it would run floci, and so give the Docker socket, in a job that holds the passphrase and an App token.
- **Upload the full `tofu show -json` and reduce it in the collector.** Rejected: the `github` stack's full plan holds values in plaintext, and artifacts of public repos are public.
- **Run floci without the Docker socket.** Rejected: floci's ECR needs it (ADR-0004).
- **Download every reduced plan by pattern into one directory (`merge-multiple`).** Rejected: an AWS job could then upload a file named like the `github` stack's plan and shadow it, so the gate would decide the `github` stack on a plan that never ran.

## References

- Spec §6.1 step 4, §6.2 steps 1 to 4 (amendment A14), §11 risk 3.
- ADR-0004 (one floci per job), ADR-0018 (the fingerprint), ADR-0022 (the required check), ADR-0023 (tokens per step), ADR-0030 (the AWS state stays in the job).
