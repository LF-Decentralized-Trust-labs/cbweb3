# Implementation Plan: Manifest Schema and Validation

**Branch**: `023-manifest-schema-validation` | **Date**: 2026-06-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/023-manifest-schema-validation/spec.md`

## Summary

Create two co-located deliverables for the `ParticipantDeployment` manifest:

1. A JSON Schema draft-07 file (`scenario-a/provisioning/schema/v1/participant-deployment.schema.yaml`) that editors (VS Code YAML Language Server) use for autocompletion and inline validation.
2. A Go package (`scenario-a/toolkit/engine/manifest/`) with `Load` + `Validate` functions that the engine calls at runtime — no JSON Schema library dependency; struct-based validation with explicit per-field error messages.

Research resolved: runtime validation uses Go structs (not a JSON Schema library), `gopkg.in/yaml.v3` is the only new dependency, and multi-error reporting uses `errors.Join` (Go 1.20+).

## Technical Context

**Language/Version**: Go 1.26+; YAML 1.2 (schema and manifest files)
**Primary Dependencies**: `gopkg.in/yaml.v3` (new; only addition to `toolkit/go.mod`)
**Storage**: None — stateless; reads one file, validates, returns result
**Testing**: `go test` with table-driven cases covering all FR items
**Target Platform**: Linux/macOS developer workstations and CI pipelines
**Project Type**: Go library package (consumed by the provisioning engine)
**Performance Goals**: < 100ms per validation on a standard workstation (SC-001)
**Constraints**: No external deps beyond `yaml.v3` for the validator; JSON Schema file must be valid draft-07 per RFC
**Scale/Scope**: Single manifest file per invocation; no concurrency requirements

## Constitution Check

*GATE: Must pass before implementation. Re-checked after Phase 1 design: ✅ no changes.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Scenario-Scoped Independence | ✅ Pass | All deliverables live under `scenario-a/`; no Scenario B paths touched |
| II. Privacy by Design | ✅ Pass | FR-007 explicitly forbids secret fields in the schema; no on-chain code |
| III. Atomic Settlement Guarantee | N/A | No settlement code in this task |
| IV. Compliance Gate Before Participation | N/A | Schema is pre-deployment infrastructure, not payment-path code |
| V. Test-First at Every Layer | ✅ Pass | Table-driven tests written before implementation; failing test first |
| VI. Observability and Auditability | N/A | Library function; not a microservice; no log requirements |

No violations. Complexity Tracking table omitted (no deviations).

## Project Structure

### Documentation (this feature)

```text
specs/023-manifest-schema-validation/
├── plan.md              ← this file
├── spec.md
├── research.md
├── data-model.md
├── contracts/
│   ├── manifest-yaml-format.md
│   └── go-package-api.md
├── checklists/
│   └── requirements.md
└── tasks.md             ← created by /speckit.tasks (not this command)
```

### Source Code

```text
scenario-a/
├── provisioning/
│   └── schema/
│       └── v1/
│           └── participant-deployment.schema.yaml   ← NEW: JSON Schema draft-07
└── toolkit/
    ├── go.mod                                        ← ADD: gopkg.in/yaml.v3
    └── engine/
        ├── genesis/          (existing — untouched)
        ├── pki/              (existing — untouched)
        └── manifest/         ← NEW package
            ├── types.go          struct definitions
            ├── validate.go       Load() + Validate()
            └── validate_test.go  table-driven tests
```

**Structure Decision**: New `engine/manifest` package follows the established pattern of `engine/genesis` and `engine/pki`. Schema file goes under `provisioning/schema/v1/` per the spec (TK-1 item). No new top-level directories; no changes outside `scenario-a/`.
