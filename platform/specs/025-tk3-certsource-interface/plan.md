# Implementation Plan: TK-3 — Interface certSource

**Branch**: `025-tk3-certsource-interface` | **Date**: 2026-06-27 | **Spec**: `specs/025-tk3-certsource-interface/spec.md`
**Input**: Feature specification from `/specs/025-tk3-certsource-interface/spec.md`

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

This feature defines the `CertSource` interface, a local in-memory self-signed CA implementation
(`certSource: self-signed`, with `self-signed://<x>` tolerated), a production stub (`ca://...`), and
a factory for the Scenario A provisioning toolkit. Production CA implementation is deferred to
Phase 4. Per-principle assessment:

- **I. Scenario-Scoped Independence — PASS.** The interface, local implementation, stub, and
  factory live entirely under `scenario-a/toolkit/engine/certsource/`. No code is shared with
  Scenario B and no Scenario B artifacts are read or written.
- **II. Privacy by Design — PASS (security-relevant: single-tier CB-issued PKI).** The model is a
  single tier of central-bank-issued PKI: the central bank acts as the spoke CA and signs commercial
  bank leaf certificates (`OU=ROLE_COMMERCIAL_BANK`). The spoke CA private key is generated in memory
  and is never serialized to disk, log, environment variable, or manifest (FR-004, SC-003). The bank
  retains only its own private key; the trust anchor is distributed as a certificate via
  `GetTrustAnchor`. No private key material crosses the interface boundary. This aligns with the
  constitution's requirement that PKI certificates be issued by central bank CAs.
- **III. Atomic Settlement Guarantee — N/A.** No settlement or cross-network transfer logic is
  introduced; certificate issuance is a provisioning-time primitive.
- **IV. Compliance Gate Before Participation — SUPPORTING.** Leaf-cert issuance restricted to
  `OU=ROLE_COMMERCIAL_BANK` (FR-002, `ErrForbiddenRole`) provides the PKI identity material the
  gateway later uses for the nonce challenge-response; this task issues credentials but does not
  itself implement the runtime compliance gate (enforced at the API gateway in a later task).
- **V. Test-First at Every Layer — PASS.** The package is covered by `go test -race`. The three
  user stories (IssueLeafCert, GetTrustAnchor, factory + prod stub) map to acceptance tests written
  before implementation, including forbidden-role rejection, unsupported-key-algorithm rejection,
  malformed-CSR handling, idempotent trust anchor retrieval, no-key-file assertion, and concurrent
  access, per Red-Green-Refactor (SC-001).
- **VI. Observability and Auditability — PASS.** The factory returns a descriptive error before any
  cryptographic operation for unrecognized `certSource` values (US3); issuance failures return
  distinct sentinel errors rather than being swallowed. Silent creation of cryptographic material by
  a read operation is explicitly prevented — only `IssueLeafCert` initializes a spoke CA, never
  `GetTrustAnchor`.

**Gate result: PASS.** No deviations to record in Complexity Tracking. Production CA deferral to
Phase 4 is a scope boundary, not an architectural deviation; the production stub preserves the
interface contract so the orchestration engine needs no code change at migration (US3).

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
