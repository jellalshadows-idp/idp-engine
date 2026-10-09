# 0006. Deterministic identities

- Status: Accepted
- Date: 2026-10-08
- Spec: [§5.5](../superpowers/specs/2026-10-08-idp-on-actions-design.md#55-deterministic-identities-with-no-shared-state-between-stacks)

## Context

The GitHub stack needs AWS values. For a Component with `aws.registry: true`, each GitHub environment gets the variables `AWS_ROLE_ARN`, `ECR_REPOSITORY` and `AWS_REGION` (spec §4.4), and the trust policy of the CI role must match `repo:<org>/<component>:environment:<env>`. The natural Terraform answer is to read these values from the AWS stack's outputs or state.

That does not work here. The AWS stacks run against an **ephemeral emulator** (ADR-0004): its state is rebuilt from `wet` on every job and is gone afterwards, so nothing can be read later. Coupling the two stacks through state would also create ordering dependencies, make the GitHub plan depend on AWS availability, and force a failed AWS apply to block GitHub, which the reconcile deliberately avoids (§6.2).

The values are, however, pure functions of data the platform already owns: the account ID and region per environment in `config/platform.yaml` (§4.6), and the component name and environment from the claim, whose name rules are strict (§4.2).

## Decision

Cross-stack values are **computed by `internal/naming`** from the platform config and the claim, and never read from AWS state (spec §3.3, §5.5):

- Role: `arn:aws:iam::<accountId>:role/<component>-<env>-ci`
- ECR: `<accountId>.dkr.ecr.<region>.amazonaws.com/<component>`
- OIDC provider: `arn:aws:iam::<accountId>:oidc-provider/token.actions.githubusercontent.com`

Stacks have no data dependencies on each other, so they plan in parallel and their order does not matter. The same package produces Terraform addresses. The name rules guarantee the role name is at most 54 characters, within IAM's 64-character limit (component names are at most 40 characters, §4.2, and env names at most 10, §4.6).

## Consequences

### Positive

- The GitHub and AWS stacks are independent: they plan in parallel, and an AWS failure does not block the GitHub apply (§6.2).
- Rendering stays pure and offline (ADR-0005): the identity is a function of the input.
- Naming is table-driven testable in Go, and the module tests can assert the trust policy `sub` (§8.2, §8.3).
- The same code works for real AWS, because the values are shaped like real ARNs and URLs (§4.6).

### Negative / costs

- The names are a **contract**. If the AWS module and `internal/naming` disagree, the GitHub variables point at a role or repository that does not exist, and no state would reveal it. The level-3 floci tests exist to catch this (§8.4).
- Renaming is not supported in v1 (a rename is a delete plus a create, §4.7), because identities derive from the name.
- Changing a naming rule changes every dependent value, so it is a breaking change to the platform.

### Follow-ups

- Phase 2 delivers the deterministic env variables, and its E2E checks them (§10).
- Floci tests should assert that the created role and repository match what `internal/naming` returns (§8.4).

## Alternatives considered

- **Remote state data sources between stacks.** Rejected: the AWS state lives in an ephemeral emulator and cannot be read later.
- **Stack outputs passed as inputs at apply time.** Rejected: it would force ordering (AWS before GitHub) and make a failed AWS apply block GitHub, whereas independent stacks self-heal (§6.2).
- **Querying AWS at render time.** Rejected: render must be offline and pure (§5.1).

## References

- Spec §3.3, §4.2, §4.4, §4.6, §5.1, §5.5, §6.2, §8.3, §8.4.
