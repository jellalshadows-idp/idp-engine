# 0015. Claim parsing and validation libraries

- Status: Accepted
- Date: 2026-10-09
- Spec: [§4.7](../superpowers/specs/2026-10-08-idp-on-actions-design.md#47-validation)

## Context

Spec §4.7 requires two validation layers (JSON Schema per kind, then semantic checks) and that **all** errors are reported together as GitHub annotations with file **and line**. That needs a YAML parser that keeps node positions, and a JSON Schema validator for draft 2020-12 whose errors carry the failing instance location, so each error can be mapped back to a line.

The usual Go YAML library, `gopkg.in/yaml.v3` (github.com/go-yaml/yaml), is archived; its README says "THIS PROJECT IS UNMAINTAINED" (verified 2026-10-09). The YAML organization maintains a fork at github.com/yaml/go-yaml, published as `go.yaml.in/yaml/v3`; its v4 line is still release candidates.

## Decision

- Parse claims with `go.yaml.in/yaml/v3` (v3.0.5). Use `yaml.Node` to build a JSON-pointer → line index per file, and accept exactly one YAML document per file.
- Validate with `github.com/santhosh-tekuri/jsonschema/v6` (v6.0.3). It passes the official 2020-12 test suite, and its `ValidationError` tree carries `InstanceLocation` for every leaf.
- The JSON Schemas in `schemas/*.json` are the **single source of truth** for claim structure. They are embedded in the binary and published at each tag, so editors get completion through `# yaml-language-server: $schema=…`.
- Schemas are strict (`additionalProperties: false`). Fields of later phases (`aws`, `features`, Workspace claims) are rejected until the phase that implements them extends the schema.
- The directory decides the kind (`claims/groups` → Group, `claims/components` → Component). A file whose `kind` disagrees fails the schema's `const`.

## Consequences

### Positive

- Every structural error has a file and line, with no hand-written structural checks duplicating the schema.
- Unknown or not-yet-supported fields fail fast instead of being ignored.

### Negative / costs

- Two new dependencies (plus `golang.org/x/text` for error messages). All are pinned ≥ 14 days old.
- Error wording comes from the validator library; tests assert locations and pointers, not its exact text.

### Follow-ups

- Phases 2–4 extend the schemas (Component `aws`, `features`; Workspace) and add their semantic checks.

## Alternatives considered

- **`gopkg.in/yaml.v3`.** Rejected: archived and unmaintained.
- **`go.yaml.in/yaml/v4`.** Rejected: only release candidates exist.
- **Go structs with hand-written validation.** Rejected: a second source of truth next to the published schemas, and no editor completion.
- **Choosing the kind from the `kind` field.** Rejected: a misfiled claim would validate against the wrong rules silently.
