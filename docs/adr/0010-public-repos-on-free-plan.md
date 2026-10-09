# 0010. Public repos on the Free plan

- Status: Accepted
- Date: 2026-10-08
- Spec: [§2.3](../superpowers/specs/2026-10-08-idp-on-actions-design.md#23-hard-constraints)

## Context

On Free, rulesets and environment reviewers exist only for public repos.

## Decision

Every repo, including Component repos, is public.

## Consequences

- Nothing secret may live in repos, logs or state.
- Private repos need the Team plan.

## Alternatives considered

- Team plan.
- Private repos without protections.
