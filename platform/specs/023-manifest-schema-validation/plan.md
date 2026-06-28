# Implementation Plan: TK-1 — Manifest Schema and Validation for Participant Deployment

**Branch**: `023-manifest-schema-validation` | **Date**: 2026-06-27 | **Spec**: `specs/023-manifest-schema-validation/spec.md`
**Input**: Feature specification from `/specs/023-manifest-schema-validation/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: [e.g., Python 3.11, Swift 5.9, Rust 1.75 or NEEDS CLARIFICATION]  
**Primary Dependencies**: [e.g., FastAPI, UIKit, LLVM or NEEDS CLARIFICATION]  
**Storage**: [if applicable, e.g., PostgreSQL, CoreData, files or N/A]  
**Testing**: [e.g., pytest, XCTest, cargo test or NEEDS CLARIFICATION]  
**Target Platform**: [e.g., Linux server, iOS 15+, WASM or NEEDS CLARIFICATION]
**Project Type**: [e.g., library/cli/web-service/mobile-app/compiler/desktop-app or NEEDS CLARIFICATION]  
**Performance Goals**: [domain-specific, e.g., 1000 req/s, 10k lines/sec, 60 fps or NEEDS CLARIFICATION]  
**Constraints**: [domain-specific, e.g., <200ms p95, <100MB memory, offline-capable or NEEDS CLARIFICATION]  
**Scale/Scope**: [domain-specific, e.g., 10k users, 1M LOC, 50 screens or NEEDS CLARIFICATION]

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

This feature delivers the ParticipantDeployment manifest JSON Schema (draft-07) and its
runtime validator for the Scenario A provisioning toolkit. Per-principle assessment:

- **I. Scenario-Scoped Independence — PASS.** Schema and validator live entirely under
  `scenario-a/toolkit/`. The schema rejects `spec.scenario: b` (FR-002), structurally enforcing
  that this artifact only describes Scenario A participants. No Scenario B files are read or written.
- **II. Privacy by Design — PASS (security-relevant).** The schema MUST NOT accommodate secrets:
  private keys, passphrases, and credentials cannot be expressed as manifest fields (FR-007).
  `spec.keyProvider` / `spec.certSource` carry only URI references to a backend, never key material.
  This keeps key handling delegated to TK-2/TK-3, consistent with the constitution's prohibition on
  plaintext sensitive data in declarative artifacts.
- **III. Atomic Settlement Guarantee — N/A.** This task introduces no settlement, HTLC, or
  cross-network transfer logic; it is a static schema + validator over a deployment manifest.
- **IV. Compliance Gate Before Participation — N/A.** No payment path, IdentityRegistry, or
  Keycloak interaction is introduced; validation runs at provisioning time, before any runtime entity exists.
- **V. Test-First at Every Layer — PASS.** Validator is a Go toolkit component covered by `go test`.
  The four user stories (valid manifest, missing field, invalid enum, editor tooling) map directly to
  acceptance tests written before implementation, per the Red-Green-Refactor discipline. SC-005
  pins the canonical minimal manifest (concat.md §5.1) as a passing fixture.
- **VI. Observability and Auditability — PASS.** The validator fails fast and reports every
  violation in a single pass (FR-004, SC-003), naming each field by full path (FR-002, SC-002). No
  error is swallowed silently; missing-field and enum-violation cases produce distinct messages (FR-005).

**Gate result: PASS.** No deviations to record in Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
# [REMOVE IF UNUSED] Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# [REMOVE IF UNUSED] Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# [REMOVE IF UNUSED] Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure: feature modules, UI flows, platform tests]
```

**Structure Decision**: [Document the selected structure and reference the real
directories captured above]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
