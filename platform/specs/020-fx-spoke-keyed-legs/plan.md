# Implementation Plan: FX Agreement Spoke-Keyed Legs

**Branch**: `020-fx-spoke-keyed-legs` | **Date**: 2026-06-26 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `specs/020-fx-spoke-keyed-legs/spec.md`

---

## Summary

Replace the positional `spoke_a_receiver`/`spoke_b_receiver` fields in the `FXAgreement` proto with four spoke-keyed fields — `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` — and propagate the change through the full stack: proto, generated Go code, domain struct, GORM model, database migration (with backfill), gRPC server routing logic, Cacti relay leg parsing, and frontend type definitions. All changes are scoped to Scenario A. Test-first: a failing test precedes every production change.

---

## Technical Context

**Language/Version**: Go 1.26+ (payment-orchestrator), TypeScript (relay, frontend)  
**Primary Dependencies**: protoc + protoc-gen-go (code gen), GORM v2 (ORM + AutoMigrate), gRPC  
**Storage**: PostgreSQL — `fx_agreements` table; column additions + backfill + column drops via startup SQL  
**Testing**: `go test` (unit + integration), existing `fx_onchain_test.go` and `relay_test.go`  
**Target Platform**: Linux container (Docker Compose, Scenario A stack)  
**Project Type**: gRPC microservice + interop relay + React frontend  
**Performance Goals**: No regression in agreement proposal latency (change is additive at the DB layer)  
**Constraints**: Proto3 wire compatibility — removed field numbers must be reserved; backfill must be idempotent  
**Scale/Scope**: Scenario A only; affects one microservice, one relay, one frontend type file

---

## Constitution Check

*GATE: All six Core Principles evaluated before Phase 0. Re-checked after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| **I. Scenario-Scoped Independence** | ✅ PASS | Change is entirely within `scenario-a/`. Scenario B's proto file is untouched. Spec FR-009 explicitly calls this out. |
| **II. Privacy by Design** | ✅ PASS | `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` are routing/coordination metadata — no PII, no amounts, no on-chain value. The Zeto/Noto privacy layer is unchanged. |
| **III. Atomic Settlement Guarantee** | ✅ PASS | HTLC lock+secret-reveal semantics are preserved. The relay routing logic (FR-006) replaces a positional field read with a spoke-ID field read — the atomicity guarantee is not weakened. The new routing logic must be verified against the existing `relay_test.go` suite. |
| **IV. Compliance Gate Before Participation** | ✅ PASS | No changes to IdentityRegistry, Compliance service, Keycloak, or API gateway. |
| **V. Test-First at Every Layer** | ✅ PASS | FR-008 mandates a failing test before any production code change. Tests required: (a) new `fx_onchain_test.go` case for the new proto fields, (b) updated `relay_test.go` routing test, (c) `gorm_repos_test.go` column persistence test. |
| **VI. Observability and Auditability** | ✅ PASS | Relay must log `dest_spoke_id` in the lock-detected event to maintain the full settlement lifecycle audit trail (constitution §VI). Verify log fields are updated alongside the field rename. |

**No violations. No Complexity Tracking required.**

---

## Project Structure

### Documentation (this feature)

```text
specs/020-fx-spoke-keyed-legs/
├── plan.md              ← this file
├── spec.md
├── research.md          ← Phase 0 output
├── data-model.md        ← Phase 1 output
├── contracts/
│   └── grpc-fx-agreement.md  ← Phase 1 output
└── tasks.md             ← Phase 2 output (from /speckit.tasks)
```

### Source Code (Scenario A — files touched by this change)

```text
scenario-a/
├── apis/proto/payment_orchestrator/v1/
│   └── payment_orchestrator.proto          ← proto change (BREAKING)
│
├── backend/shared/proto/payment_orchestrator/v1/
│   └── payment_orchestrator.pb.go          ← regenerated (make proto-gen)
│
├── backend/services/payment-orchestrator/
│   ├── internal/domain/
│   │   └── fx.go                           ← domain struct field rename
│   ├── internal/repository/
│   │   ├── fx_agreement_models.go          ← GORM model field rename
│   │   ├── fx_agreement_gorm.go            ← startup SQL migration added
│   │   └── gorm_repos_test.go              ← new column persistence test
│   └── internal/grpc/server/
│       ├── server.go                       ← 14 field references updated + routing logic
│       ├── fx_onchain_test.go              ← failing test added FIRST
│       └── relay_test.go                  ← spoke-routing test updated
│
├── interop/hub-and-spoke/cacti/src/
│   └── htlc-relay.ts                       ← interface + parsing + payload updated
│
└── frontend/apps/bank/src/types/
    └── fx-agreement.types.ts               ← TypeScript type updated
```

---

## Implementation Phases

### Phase 1A — Test-First (write failing tests before any production code)

1. Add a test case to `fx_onchain_test.go` that constructs a `ProposeFXAgreementRequest` using `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`.
   - **Expected**: compile error (fields do not exist yet). This is the required "red" state.
2. Add a test case to `relay_test.go` verifying spoke-ID routing via `DestSpokeID == spokePrefix`.
   - **Expected**: compile error.
3. Add a test to `gorm_repos_test.go` asserting that `SourceSpokeID`, `DestSpokeID`, `SourceReceiver`, `DestReceiver` are persisted and retrieved correctly.
   - **Expected**: compile error.

### Phase 1B — Proto change

1. Edit `payment_orchestrator.proto`:
   - Add `reserved` declarations for fields 17, 18 in `FXAgreement` and fields 14, 15 in `ProposeFXAgreementRequest`.
   - Add `source_spoke_id=21`, `dest_spoke_id=22`, `source_receiver=23`, `dest_receiver=24` to `FXAgreement`.
   - Add `source_spoke_id=16`, `dest_spoke_id=17`, `source_receiver=18`, `dest_receiver=19` to `ProposeFXAgreementRequest`.
2. Regenerate Go code: `make proto-gen` (from `scenario-a/`).
   - The generated `.pb.go` is committed alongside the proto source.

### Phase 1C — Go backend changes

Order matters (domain → model → repository → server):

1. **`domain/fx.go`**: Rename `SpokeAReceiver`→`SourceSpokeID`, `SpokeBReceiver`→`SpokeBReceiver`; add `DestSpokeID`, `SourceReceiver`, `DestReceiver`.
2. **`repository/fx_agreement_models.go`**: Rename GORM struct fields and column tags to match.
3. **`repository/fx_agreement_gorm.go`**: Add startup SQL migration (4 ADD COLUMN + backfill + 2 DROP COLUMN) before the AutoMigrate call.
4. **`server/server.go`**: Update all 14 field references; replace the spoke-leg routing logic at lines 1666–1676 with the `DestSpokeID == s.spokePrefix` comparison; add `source_spoke_id`/`dest_spoke_id` validation (reject empty or equal values).

After each file: `go build ./...` to verify no compilation errors.

### Phase 1D — Relay changes (`htlc-relay.ts`)

1. Update the `FXAgreement` TypeScript interface at lines 148–149.
2. Update parsing at lines 607–608 (add `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`).
3. Update payload construction at lines 747–748.
4. Verify `tsc --noEmit` passes.

### Phase 1E — Frontend type

1. Update `fx-agreement.types.ts` — remove `spoke_a_receiver`/`spoke_b_receiver`, add the four new fields.

### Phase 1F — Test validation

All three failing tests (Phase 1A) now compile and pass. Run full suite:
```bash
cd scenario-a && go test ./backend/services/payment-orchestrator/...
```

---

## Verification Checklist

- [ ] `grep -r "spoke_a_receiver\|spoke_b_receiver" scenario-a/` returns zero results outside of `reserved` declarations and this plan.
- [ ] `go test ./backend/services/payment-orchestrator/...` passes.
- [ ] `make proto-gen` produces no diff beyond the expected field additions/removals.
- [ ] `tsc --noEmit` in the relay passes.
- [ ] A fresh `ProposeFXAgreement` with two distinct spoke IDs completes the propose–accept–lock–settle flow in the local stack.
- [ ] The database backfill migration is idempotent (run twice: second run is a no-op).
