# 0011. Bootstrap outside the IDP

- Status: Accepted
- Date: 2026-10-08
- Spec: [§9.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#92-bootstrap-outside-the-idp-on-purpose)

## Context

The platform's safety relies on protections: the rulesets on `idp-claims` `main` and `wet`, the `idp-approval` and `idp-write` environments, the secrets and variables, and the two GitHub Apps (spec §7.3, §7.4). Someone has to create them before the pipelines can run, and someone has to be able to check that they are still in place.

The obvious tool is the platform itself: declare them as claims and let the pipeline apply them. That is circular and unsafe. If the IDP manages its own protections, a PR could disable the very checks that guard it. It would also need a way to start from nothing, since the claims repo, `wet` branch and Apps do not exist yet. Managing them with a separate OpenTofu stack just moves the problem: that stack needs its own state somewhere, "turtles all the way down" (§9.2).

There is a further GitHub constraint: creating an App through the manifest flow requires a person to confirm in the browser, and App names are unique across GitHub and limited to 34 characters (verified). Bootstrap can automate around that but not remove the human step.

## Decision

Bootstrap is **outside the IDP**, and is implemented as the Go subcommand `idp bootstrap`, not as a bash script (spec §9.2, amended from the original script design so it can be developed test-first locally and so the desired-versus-actual JSON comparison that `check` needs is natural in Go). The code lives in `internal/bootstrap` on top of `internal/ghapi` (§3.3).

- **`idp bootstrap app`** runs GitHub's manifest flow through a local callback server. A person still confirms in the browser. The manifests are versioned in `idp-engine/bootstrap/apps/{reader,writer}.json`, so permissions are reviewable code.
- **`idp bootstrap apply`** (idempotent) creates the claims repo and its `wet` branch, the rulesets, the environments `idp-approval` and `idp-write`, the secrets and variables, and the org-level Actions workflow defaults (read-only).
- **`idp bootstrap check`** reports drift in those protections. The daily drift workflow runs it (§6.4).
- Allowing Actions to create PRs is **deferred to Phase 4**: GitHub exposes it together with approving PRs as a single setting (`can_approve_pull_request_reviews`), so enabling it weakens required reviews in Component repos. It is turned on only when the `release-please` feature lands, and that ADR records the risk.

## Consequences

### Positive

- A PR cannot disable the protections that guard the platform, because nothing in `main` manages them.
- No second state to store. The protections are described in code and compared against reality.
- Go makes bootstrap unit-testable locally, which fits the TDD practice and the no-local-Docker constraint (§8.1, §2.3).
- Reviewable App manifests, and drift reporting through `check`.

### Negative / costs

- Bootstrap is a different mechanism from the rest of the platform, so there are two ways of managing GitHub resources.
- A person must confirm App creation in the browser. Bootstrapping a new org is therefore never fully unattended.
- Reader permissions have limits: `bypass_actors` is only returned to callers with write access, so `check` with a read token cannot fully verify a ruleset (verified fact).

### Follow-ups

- Phase 0 produces the Apps, orgs and repos by hand (spec §10, §12) before `idp bootstrap` exists to automate it.
- The runbook "bootstrap a new org" and the success criterion that an external person can bootstrap using only the README are Phase 5 exit criteria (§7.7, §10).
- The Phase 4 ADR for the Actions-create-PRs setting must record the weakened-review risk.

## Alternatives considered

- **Manage the protections with claims in the IDP.** Rejected: a PR could remove the checks that guard it, and the platform could not start from nothing.
- **A separate OpenTofu stack for bootstrap.** Rejected: it needs its own state somewhere, which is circular (§9.2).
- **A bash script.** Replaced (spec amendment): a Go implementation can be TDD'd locally and suits the desired-versus-actual comparison.
- **Manual click-ops with a checklist.** Not chosen: not repeatable or checkable, and the drift check needs the desired state in code.

## References

- Spec §2.3, §3.3, §6.4, §7.3, §7.4, §7.7, §8.1, §9.2, §10, §12.
- Verified: App manifest flow needs a person in the browser; App name limits; `bypass_actors` visibility; reader permission mapping.
