# Implementation Plan: TK-2 — Interface keyProvider

**Branch**: `024-tk2-keyprovider-interface` | **Date**: 2026-06-27 | **Spec**: `specs/024-tk2-keyprovider-interface/spec.md`
**Input**: Feature specification from `/specs/024-tk2-keyprovider-interface/spec.md`

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

This feature defines the `KeyProvider` interface, a local in-memory KMS emulator
(`kms://local-emulator`), a production stub, and a URI-driven factory for the Scenario A
provisioning toolkit. Production KMS implementation is deferred to Phase 4. Per-principle assessment:

- **I. Scenario-Scoped Independence — PASS.** The interface, emulator, stub, and factory live
  entirely under `scenario-a/toolkit/`. No code is shared with Scenario B and no Scenario B
  artifacts are read or written.
- **II. Privacy by Design — PASS (security-relevant: keys never in files/env/manifest).** The
  interface never returns private key material to callers — only public keys (EVM address) and
  signatures are exposed (FR-002). The local emulator generates and holds keys exclusively in memory,
  writing no key material to file, environment variable, or log (FR-003), and the manifest carries
  only a `keyProvider` URI reference, never key bytes (KeyProviderURI entity). This directly upholds
  the constitution's prohibition on plaintext sensitive data outside the privacy-preserving layer.
- **III. Atomic Settlement Guarantee — N/A.** No settlement or cross-network transfer logic is
  introduced; signing is an isolated cryptographic primitive consumed later by the orchestration engine.
- **IV. Compliance Gate Before Participation — N/A.** No payment path, IdentityRegistry, or
  Keycloak interaction is introduced; the provider operates at provisioning time.
- **V. Test-First at Every Layer — PASS.** The package is covered by `go test`. The three user
  stories (no keys in files, local-only operation, prod extensibility) map to acceptance tests
  written before implementation, including idempotent `GenerateKey`, wrong-size payload rejection,
  `key not found` errors, and concurrent-access safety, per Red-Green-Refactor (SC-002).
- **VI. Observability and Auditability — PASS.** The factory returns a readable error (no raw
  stack trace) for malformed or unrecognized URIs (FR-008, SC-005); all operations support context
  cancellation (FR-010). Errors are surfaced descriptively rather than swallowed; logging key
  material is explicitly prohibited (FR-003), consistent with structured-logging discipline.

**Gate result: PASS.** No deviations to record in Complexity Tracking. Production KMS deferral to
Phase 4 is a scope boundary, not an architectural deviation; the production stub preserves the
interface contract so the orchestration engine needs no code change at migration (SC-003).

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
