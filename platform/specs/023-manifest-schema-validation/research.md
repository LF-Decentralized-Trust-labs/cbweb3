# Research: Manifest Schema and Validation

**Feature**: 023-manifest-schema-validation
**Date**: 2026-06-27

## Decision 1 — Runtime validation approach: Go structs vs. JSON Schema library

**Decision**: The runtime validator uses Go struct unmarshaling plus explicit programmatic checks. The JSON Schema (draft-07) file is used exclusively by editor tooling (YAML Language Server); it is **not** loaded at runtime.

**Rationale**: A JSON Schema runtime library (e.g., `github.com/santhosh-tekuri/jsonschema/v5`) would add an external dependency to the toolkit module for a task the editor integration already handles for free. Explicit Go checks produce error messages tailored to operator UX (e.g., naming `spec.node.advertisedHost` and adding "never inferred") whereas JSON Schema validation errors are generic pointer strings. The existing toolkit packages (`genesis/guard.go`, `pki/csr.go`) use zero external deps beyond stdlib; the manifest package follows the same pattern.

**Alternatives considered**:
- `github.com/santhosh-tekuri/jsonschema/v5` (runtime schema validation) — supports draft-07+, well-maintained, but adds a dependency for richer errors we can produce manually at lower cost.
- `github.com/xeipuuv/gojsonschema` — older library; same objection.
- Both alternatives would make runtime errors mirror the JSON Schema pointer format (`/spec/node/advertisedHost`) rather than the operator-readable field path used in the spec (`spec.node.advertisedHost is required and must never be inferred`).

## Decision 2 — YAML parsing library

**Decision**: `gopkg.in/yaml.v3` — the only YAML dependency added to the toolkit module.

**Rationale**: Standard Go YAML library (v3 API). Needed regardless of validation approach to parse the manifest file into a Go struct. No alternative offers a meaningful advantage at this scope.

**Alternatives considered**:
- `gopkg.in/yaml.v2` — older API, struct tag semantics differ slightly. v3 is the correct choice for new code.

## Decision 3 — Multi-error accumulation pattern

**Decision**: Collect validation errors into a `[]error` slice; return them joined via `errors.Join()` (available since Go 1.20, present in Go 1.26+). Callers can unwrap individual errors if needed.

**Rationale**: `errors.Join` is stdlib (no import), produces a multi-line error string from multiple errors, and integrates cleanly with `errors.Is`/`errors.As` for test assertions. Each individual error in the slice carries a distinct field-path message, satisfying SC-002 and SC-003.

**Alternatives considered**:
- Custom `ValidationError` struct with a `[]string Fields` slice — adds a type for no behavioral gain; `errors.Join` covers the same use case.
- Return on first error — violates FR-004 and SC-003 (operator must re-run to find additional errors).

## Decision 4 — JSON Schema draft-07 YAML format for YAML Language Server

**Decision**: Use `$schema: "http://json-schema.org/draft-07/schema#"` at root. Use `definitions` (not `$defs`, which is 2019-09+). Use `if/then/else` (draft-07 compliant) to document the conditional `joinBundleRef` requirement in a comment only — not enforced at schema level per FR-011.

**Rationale**: The Red Hat YAML extension (v1.14+) fully supports JSON Schema draft-07 in YAML file format. Operator manifest files gain autocompletion and inline errors by adding one header line: `# yaml-language-server: $schema: ../../provisioning/schema/v1/participant-deployment.schema.yaml`. No plugin installation or workspace configuration is required beyond the YAML extension itself.

**Alternatives considered**:
- JSON Schema 2019-09 or 2020-12 — use `$defs`; rejected because YAML LS support for post-draft-07 versions is less uniform across editor versions.
- OpenAPI 3.x Schema Object — not applicable; not a REST API resource.

## Decision 5 — Package location within the toolkit module

**Decision**: `scenario-a/toolkit/engine/manifest/` — consistent with `engine/genesis/` and `engine/pki/`.

**Rationale**: The manifest package is foundational to the engine: every `apply` invocation loads and validates a manifest first, then hands the parsed struct to downstream engine packages. Placing it under `engine/` makes the dependency direction explicit. Keeping it as a subpackage of `engine/` (not a top-level package) prevents it from being imported by external tools without going through the engine.

## Decision 6 — go.mod dependency addition

**Decision**: Add `gopkg.in/yaml.v3` to `scenario-a/toolkit/go.mod` via `go get`.

**Impact**: Single new dependency. No other toolkit package uses YAML today; this PR introduces the first YAML-aware package. The `go.sum` file will be updated accordingly.
