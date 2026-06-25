# Implementation Plan: Fix PKI Local CA Model

**Branch**: `016-fix-pki-local-ca` | **Date**: 2026-06-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/016-fix-pki-local-ca/spec.md`

## Summary

Two-track fix for Scenario A's PKI model:

1. **Track 1 (Makefile documentation)** — Add explicit `[DEV ONLY]` warning comments and terminal output to `scenario-a/make/05-pki.mk`'s commercial-bank cert targets. No functional change; backward-compatible. Updates scenario-a README to document both paths.

2. **Track 2 (Toolkit enforcement)** — Define the contract that the new provisioning toolkit (TK-3 orchestration engine, TK-9 commercial-bank join flow) MUST follow: use `shared/identity.GenerateCSR` for keypair+CSR generation, then POST to CB `credential-request` endpoint for signing. A commercial bank MUST never generate a CA key. The existing `shared/identity/ca.go` (`SignCSR`, `GenerateCSR`, `GenerateSelfSignedCA`) and compliance gRPC `SignParticipantCSR` are the authoritative implementations to reuse.

Research findings confirm zero NEEDS CLARIFICATION items — all decisions resolved by code inspection. See `research.md`.

---

## Technical Context

**Language/Version**: Go 1.26+ (toolkit PKI step), GNU Make + Bash (Makefile fix)
**Primary Dependencies**:
- `scenario-a/backend/shared/identity/ca.go` — `GenerateCSR`, `SignCSR`, `GenerateSelfSignedCA` (no new deps)
- `scenario-a/backend/services/compliance/internal/pki/ca.go` — `CA.SignCSR`, `CA.NewCAFromEnv`
- CB public API `POST /api/v1/onboarding/credential-request` (existing endpoint)
- `ComplianceService.SignParticipantCSR` gRPC (existing, called internally by CB API)

**Storage**: Filesystem only — `{bankCode}.key`, `{bankCode}.csr`, `{bankCode}.crt` (leaf cert from CB)

**Testing**: `go test` for toolkit PKI unit tests; manual `openssl verify` check for integration

**Target Platform**: Linux (local dev + staging/prod; same code path per task-notions §4)

**Project Type**: Tooling fix (documentation) + provisioning toolkit component contract

**Performance Goals**: N/A — PKI cert issuance is a one-time onboarding operation, not in the hot path

**Constraints**: Makefile fix must not break existing `make pki.gen-all` flow; toolkit must not introduce a CA key for commercial banks under any code path

**Scale/Scope**: Scenario A only; affects `make/05-pki.mk`, `scenario-a/README.md`, and the to-be-built provisioning toolkit (TK-3/TK-9)

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-evaluated post-research — all findings consistent.*

| Principle | Status | Notes |
|-----------|--------|-------|
| **I. Scenario-Scoped Independence** | ✅ PASS | Changes confined to `scenario-a/`; no cross-scenario touches. Toolkit artifacts self-contained within Scenario A per task-notions §10. |
| **II. Privacy by Design** | ✅ N/A | PKI cert management only; no on-chain value transfers, no ZetoToken/NotoToken changes, no PII on-chain. |
| **III. Atomic Settlement Guarantee** | ✅ N/A | No changes to settlement paths, HTLC, or relay. |
| **IV. Compliance Gate Before Participation** | ✅ STRENGTHENED | The fix enforces that commercial bank certs are issued by the CB (the compliance authority), not self-signed. This directly tightens the trust boundary that gate IV relies on. The toolkit join flow must pass through `credential-request` → `onboarding/complete` (PoP + IdentityRegistry), per spec FR-005. |
| **V. Test-First at Every Layer** | ✅ PASS (with obligation) | Makefile doc change: no tests needed. Toolkit PKI step (TK-9): failing test MUST be written before implementing the CSR submission step. `ca_signature_test.go` already covers the shared library; new tests needed for toolkit-level behavior (no bank CA generated, fail-fast on CB unreachable). |
| **VI. Observability and Auditability** | ✅ PASS (with obligation) | Toolkit CSR submission and CB response MUST emit structured JSON log entries (request ID, service, severity, ISO-8601 ts). CB `credential-request` handler already logs. Toolkit must log: CSR generated for `{bankCode}`, CSR submitted to `{cb_url}`, cert received and stored. |

**Constitution Check result**: PASS. No violations. No Complexity Tracking entries required.

---

## Project Structure

### Documentation (this feature)

```text
specs/016-fix-pki-local-ca/
├── plan.md              ← this file
├── research.md          ← Phase 0 output (complete)
├── data-model.md        ← Phase 1 output (minimal — PKI file contract only)
└── tasks.md             ← Phase 2 output (/speckit.tasks — not yet created)
```

No `contracts/` or `quickstart.md` needed: Track 1 is documentation-only; Track 2 defines behavior for toolkit components that do not yet exist, so no external API contract is exposed by this PR.

### Source Code (repository root)

Track 1 — Makefile documentation (immediate changes):

```text
scenario-a/
├── make/
│   └── 05-pki.mk                         ← add [DEV ONLY] comments + echo warnings
└── README.md (or docs/pki.md)            ← add PKI trust model section
```

Track 2 — Provisioning toolkit PKI contract (defines what TK-3/TK-9 must implement):

```text
scenario-a/
└── toolkit/                              ← new (created by TK-3/TK-9 work, not this PR)
    ├── engine/
    │   └── pki/
    │       ├── csr.go                    ← GenerateCSR wrapper (reuses shared/identity)
    │       └── csr_test.go              ← failing test: no bank CA generated
    └── join/
        └── pki_step.go                   ← CSR submission to CB credential-request endpoint
```

> **Note**: Track 2 source files do not exist yet — this plan defines their contract. The failing tests for TK-9 PKI behavior will be the first artifact created in the toolkit implementation PR. This PR (016) delivers only the Makefile + README documentation (Track 1) and the specification/design artifacts (this plan + data-model.md).

**Structure Decision**: Single scenario-a project layout. No new top-level directories created by this PR. Toolkit directory is planned but built in subsequent PRs (TK-3, TK-9).

---

## Phase 0 Research Summary

Research complete. See `research.md` for full findings. Key decisions:

| Decision | Rationale |
|----------|-----------|
| Toolkit uses `shared/identity.GenerateCSR` for CSR creation | Already implemented, tested, P-256/ECDSA compliant |
| Toolkit submits via `POST /api/v1/onboarding/credential-request` | Same public endpoint used by smart proxy; correct boundary for external operator tool |
| CB CA signing uses `CA.SignCSR` via ComplianceService gRPC | Existing implementation; toolkit never calls this directly |
| Makefile fix is comments-only | CA generation for local dev is acceptable shortcut; changing it would require CB CA key in commercial bank env |
| TK-3 CB-spoke `mode: found` uses `GenerateSelfSignedCA` | Matches current `pki.gen-central-bank-*` — CB self-signs its own root, which is correct |

---

## Phase 1 Design

### Data Model

See `data-model.md` for the PKI file contract (keypair, CSR, leaf cert) that the toolkit must produce and consume.

### Interface Contracts

No new external interfaces introduced by this PR. The existing CB `credential-request` endpoint contract is documented in `research.md` (Finding 1).

### Agent Context

Run `.specify/scripts/bash/update-agent-context.sh claude` after plan is complete to sync.

---

## Complexity Tracking

> No violations to justify — all gates pass.

---

## Implementation Phases (for /speckit.tasks)

> **This section seeds task generation. Do not implement here.**

### Phase A — Track 1: Makefile + README documentation (no code change)

**Goal**: Deliver FR-001, FR-002, FR-003 from spec. Independently testable (SC-001, SC-004).

1. Add `[DEV ONLY]` block comment above `gen_commercial_bank_cert` macro in `make/05-pki.mk`
2. Add `echo "[DEV ONLY] ..."` warning to the `pki.gen-commercial-bank-%` terminal output
3. Verify existing `make pki.gen-all` still completes cleanly (SC-004 regression check)
4. Add PKI trust model section to `scenario-a/README.md`:
   - Document local bootstrap path (Makefile shortcut) with explicit "dev only" label
   - Document production path: `GenerateCSR` → `credential-request` → CB signs → leaf cert stored
   - Reference `onboarding_proxy.go` smart mode as authoritative implementation
   - Reference `shared/identity/ca.go` as the canonical PKI library

### Phase B — Track 2: Define and write failing tests for toolkit PKI behavior (TK-3 / TK-9 pre-work)

**Goal**: Deliver constitution Principle V obligation — failing tests exist before TK-9 implementation. Seeds FR-004, FR-005, FR-006.

5. Write `scenario-a/toolkit/engine/pki/csr_test.go`:
   - Test: CSR generation produces `{bankCode}.key` + `{bankCode}.csr` and NO `{bankCode}-ca.key` or `{bankCode}-ca.crt`
   - Test: Toolkit fails fast (non-zero exit / error return) when CB `credential-request` endpoint returns error
   - Test: Toolkit fails fast when CB endpoint is unreachable (timeout / connection refused)
   - Test: Issued cert issuer matches CB CA (not bank self-CA) — use `NewCAFromPEM` test CA

6. Write stub `scenario-a/toolkit/engine/pki/csr.go` (function signatures only, returning `errors.New("not implemented")`) so tests compile but fail

**Deliverable**: `go test ./scenario-a/toolkit/...` fails with "not implemented" — red phase complete.

### Phase C — Track 2 implementation (separate PR, TK-9 scope)

> Out of scope for this PR (016). Defined here for sequencing.

7. Implement `csr.go` using `shared/identity.GenerateCSR` + HTTP POST to CB `credential-request`
8. Implement `pki_step.go` in toolkit join flow: generate → submit → poll status → complete → store cert
9. Add structured log entries (JSON, ISO-8601, request ID) for each step per constitution Principle VI
10. Verify `go test` passes; run `openssl verify` integration check (SC-002, SC-003, SC-005)
