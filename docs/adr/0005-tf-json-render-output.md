# 0005. `.tf.json` render output

- Status: Accepted
- Date: 2026-10-08
- Spec: [§5.2](../superpowers/specs/2026-10-08-idp-on-actions-design.md#52-output-format-tfjson)

## Context

`idp render` turns claims into OpenTofu stacks. It must be **pure and offline**: the same input always produces the same bytes (spec §5.1). That property is what makes the diff against `wet` meaningful, lets golden tests catch accidental changes, and lets a determinism test render twice, with a shuffled file-read order, and compare bytes (§8.2).

The renderer is written in Go. It could emit HCL text or JSON. Firestartr already made this choice: its provisioners synthesize JSON (`JSON.stringify(this.document)` in `gh_provisioner`, and `firestartr-providers.tf.json` in `terraform_provisioner`), so there is a reference for the approach (§5.2).

A second force is who reads what. The renderer is deliberately thin: it emits provider and backend blocks, `module` calls and `encryption` settings, while resource logic lives in tested modules (§5.3). So the generated files are an artifact of the machine, and humans review the **plan** in the PR comment, not the generated files.

## Decision

Stacks are generated as **`.tf.json`**, written with Go's `encoding/json` (spec §5.2).

- Go's `encoding/json` sorts map keys, so the output is deterministic without a custom writer.
- Files are named `main.tf.json` per stack (and `floci_override.tf.json` in CI), under `rendered/` in `wet` (§5.4).
- Humans review the plan in the PR comment, not the JSON.
- The JSON uses the same module calls shown in §5.3, with module sources pinned to `idp-engine` at the renderer's own version.

## Consequences

### Positive

- Deterministic output from the standard library, with no HCL formatter or template whitespace to keep stable.
- Structured data is built from Go types, so the renderer cannot produce syntactically invalid configuration by string concatenation.
- Golden-file tests and the determinism test compare bytes directly (§8.2).
- Same approach as Firestartr, so the comparison document can say what was kept (§9.3).

### Negative / costs

- JSON is harder to read than HCL for a person who opens a `wet` diff. The spec accepts this because the plan, not the render, is the review surface.
- Anything that does not exist in the JSON syntax of OpenTofu (for example comments) cannot be emitted.
- Reviewers who expect `.tf` files will not find them; the README and architecture docs must explain it.

### Follow-ups

- Phase 1 includes golden render cases under `testdata/<case>/{claims/, config/, expected/}` and the determinism test (§8.2, §10).
- Documentation should show an example of the generated JSON, as in §5.3.

## Alternatives considered

- **HCL text via templates.** Rejected: whitespace and ordering would have to be controlled by hand to keep output byte-stable, and string templating invites invalid output. JSON from sorted maps removes both problems.
- **HCL via a writer library.** Not recorded in the spec as evaluated. JSON from the standard library needs no extra dependency, which also fits the supply-chain stance (§7.5).
- **Letting humans review the rendered files instead of the plan.** Rejected: the plan shows the actual resource effects (`+N ~M -D`, destructive actions flagged), which is what a reviewer needs (§6.1).

## References

- Spec §5.1-§5.4, §7.5, §8.2, §9.3.
- Firestartr: `gh_provisioner` (`JSON.stringify(this.document)`), `terraform_provisioner` (`firestartr-providers.tf.json`), as named in spec §5.2.
