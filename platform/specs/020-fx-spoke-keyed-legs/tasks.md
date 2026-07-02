# Tasks: FX Agreement Spoke-Keyed Legs

**Input**: Design documents from `specs/020-fx-spoke-keyed-legs/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Tests**: Included per constitution Principle V (test-first is mandatory for this change).

**Organization**: Tasks are grouped by phase: foundational (proto + failing tests) → US1 (Go backend) → US2 (relay + frontend) → US3 (migration verification) → polish.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to ([US1], [US2], [US3])

---

## Phase 1: Setup

**Purpose**: Confirm test file locations and that the local stack builds cleanly before starting.

- [x] T001 Confirm `go build ./...` passes from `scenario-a/backend/services/payment-orchestrator/` (baseline green before any change)

---

## Phase 2: Foundational — Failing Tests + Proto Change

**Purpose**: Establish the RED state (failing tests) per constitution Principle V, then make the breaking proto change that unblocks all story work.

**⚠️ CRITICAL**: Tests must be written and verified to fail (compile error) BEFORE the proto is changed. Proto-gen must complete before any Go backend tasks in Phase 3.

> **NOTE: Write these tests FIRST, ensure they FAIL (compile error) before implementation**

- [x] T002 [P] Write failing test in `scenario-a/backend/services/payment-orchestrator/internal/grpc/server/fx_onchain_test.go`: add a `ProposeFXAgreement` test case that constructs the request using `SourceSpokeID`, `DestSpokeID`, `SourceReceiver`, `DestReceiver` and asserts all four fields are returned in `GetFXAgreement` — expected: compile error (fields do not exist yet)
- [x] T003 [P] Write failing test in `scenario-a/backend/services/payment-orchestrator/internal/grpc/server/relay_test.go`: add a test case for the new spoke-ID routing logic (`fx.DestSpokeID == s.spokePrefix → localReceiver = fx.DestReceiver`), replacing the current positional receiver comparison — expected: compile error
- [x] T004 [P] Write failing test in `scenario-a/backend/services/payment-orchestrator/internal/repository/gorm_repos_test.go`: assert that `SourceSpokeID`, `DestSpokeID`, `SourceReceiver`, `DestReceiver` are persisted and retrieved correctly from the `fx_agreements` table — expected: compile error
- [x] T005 Edit `scenario-a/apis/proto/payment_orchestrator/v1/payment_orchestrator.proto`: (1) in `FXAgreement` add `reserved 17, 18; reserved "spoke_a_receiver", "spoke_b_receiver";` then add fields `source_spoke_id=21`, `dest_spoke_id=22`, `source_receiver=23`, `dest_receiver=24`; (2) in `ProposeFXAgreementRequest` add `reserved 14, 15; reserved "spoke_a_receiver", "spoke_b_receiver";` then add fields `source_spoke_id=16`, `dest_spoke_id=17`, `source_receiver=18`, `dest_receiver=19` (depends on T002, T003, T004)
- [x] T006 Regenerate Go proto code: run `make proto-gen` from `scenario-a/` — confirms `scenario-a/backend/shared/proto/payment_orchestrator/v1/payment_orchestrator.pb.go` is updated; tests T002/T003/T004 should now compile but fail functionally (depends on T005)

**Checkpoint**: `go build ./...` compiles (with new proto fields), but `go test ./...` fails — expected RED state confirmed.

---

## Phase 3: User Story 1 — FX Agreement Spoke-Keyed Proposal (Priority: P1) 🎯 MVP

**Goal**: A settlement agent can propose an FX Agreement using `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`; the service validates, persists, and returns those four fields.

**Independent Test**: `go test ./internal/...` passes in `scenario-a/backend/services/payment-orchestrator/` with T002 and T004 tests green.

### Implementation for User Story 1

- [x] T007 [US1] Update `scenario-a/backend/services/payment-orchestrator/internal/domain/fx.go`: remove fields `SpokeAReceiver` and `SpokeBReceiver`; add `SourceSpokeID string`, `DestSpokeID string`, `SourceReceiver string`, `DestReceiver string` (depends on T006)
- [x] T008 [US1] Update `scenario-a/backend/services/payment-orchestrator/internal/repository/fx_agreement_models.go`: rename GORM struct fields and column tags — remove `SpokeAReceiver`/`SpokeBReceiver`, add `SourceSpokeID` (`column:source_spoke_id`), `DestSpokeID` (`column:dest_spoke_id`), `SourceReceiver` (`column:source_receiver`), `DestReceiver` (`column:dest_receiver`); update the two `toModel`/`fromModel` mapping blocks at lines 88–89 and 113–114 (depends on T007)
- [x] T009 [US1] Add startup SQL migration to `scenario-a/backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go`: before the `AutoMigrate` call, run the idempotent SQL sequence — `ADD COLUMN IF NOT EXISTS` (×4), `UPDATE ... WHERE source_spoke_id = ''` backfill, `DROP COLUMN IF EXISTS spoke_a_receiver`, `DROP COLUMN IF EXISTS spoke_b_receiver`; same steps for `fx_agreement_events` if those columns exist there (depends on T008)
- [x] T010 [US1] Update `scenario-a/backend/services/payment-orchestrator/internal/grpc/server/server.go`: replace all 14 references to `SpokeAReceiver`/`SpokeBReceiver` with the four new fields; replace the spoke-leg routing logic at lines 1666–1676 with `if fx.DestSpokeID == s.spokePrefix { localReceiver = fx.DestReceiver } else { localReceiver = fx.SourceReceiver }`; add validation rejecting requests where `source_spoke_id`, `dest_spoke_id`, `source_receiver`, or `dest_receiver` are empty, or where `source_spoke_id == dest_spoke_id` (depends on T008, T009)
- [x] T011 [US1] Run `go test ./...` from `scenario-a/backend/services/payment-orchestrator/` — T002 (fx_onchain_test) and T004 (gorm_repos_test) must now be GREEN (depends on T010)

**Checkpoint**: User Story 1 is fully functional. `ProposeFXAgreement` and `GetFXAgreement` work with spoke-keyed fields. T002 and T004 pass.

---

## Phase 4: User Story 2 — Relay Routes HTLC Legs by Destination Spoke ID (Priority: P1)

**Goal**: The Cacti relay reads `dest_spoke_id` / `source_spoke_id` from FX Agreement events to determine which spoke the counterparty HTLC lock targets.

**Independent Test**: `tsc --noEmit` passes in the relay; T003 (`relay_test.go`) passes after the Go routing logic update (T010) is in place.

### Implementation for User Story 2

- [x] T012 [P] [US2] Update `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts`: (1) in the `FXAgreement` TypeScript interface (lines 148–149) remove `spoke_a_receiver`/`spoke_b_receiver`, add `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`; (2) update chain-event parsing at lines 607–608 to read the four new keys; (3) update counterpart payload construction at lines 747–748 to emit the four new keys (depends on T006)
- [x] T013 [P] [US2] Update `scenario-a/frontend/apps/bank/src/types/fx-agreement.types.ts`: remove `spoke_a_receiver` and `spoke_b_receiver` from the `FXAgreement` TypeScript interface; add `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` (depends on T006)
- [x] T014 [US2] Run `tsc --noEmit` in both `scenario-a/interop/hub-and-spoke/cacti/` and `scenario-a/frontend/` to confirm zero type errors (depends on T012, T013)
- [x] T015 [US2] Run `go test ./internal/grpc/server/...` from `scenario-a/backend/services/payment-orchestrator/` — T003 (`relay_test.go`) must now be GREEN (depends on T010)

**Checkpoint**: Relay and frontend compile clean. T003 passes. Relay reads `dest_spoke_id` for spoke selection.

---

## Phase 5: User Story 3 — Existing Agreements Accessible After Migration (Priority: P2)

**Goal**: The startup SQL migration correctly backfills legacy rows and is idempotent when run twice.

**Independent Test**: Seed the test database with legacy rows (having `spoke_a_receiver`/`spoke_b_receiver`), restart the service, and assert all rows have non-empty `source_spoke_id`/`dest_spoke_id` and the old columns are absent.

### Implementation for User Story 3

- [x] T016 [US3] Extend `scenario-a/backend/services/payment-orchestrator/internal/repository/gorm_repos_test.go`: add a test that seeds legacy rows directly via SQL, then calls the migration function twice, and asserts: (1) all rows have non-empty `source_spoke_id` and `dest_spoke_id`; (2) the old columns do not exist; (3) the second run is a no-op (row count unchanged, no error) (depends on T009)
- [x] T017 [US3] Run `go test ./internal/repository/...` — T016 must pass; confirm `AcceptFXAgreement` and `SettleFXAgreement` still work on migrated rows by reusing existing test helpers (depends on T016)

**Checkpoint**: All three user stories are independently functional. Full `go test ./...` passes.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and documentation.

- [x] T018 [P] Run `grep -r "spoke_a_receiver\|spoke_b_receiver" scenario-a/` — must return zero hits outside of `reserved` declarations in the proto file and this tasks.md (depends on T017)
- [x] T019 [P] Update `scenario-a/README.md`: mark the spoke-keyed FX Agreement data model as "In progress" or "Fully implemented" per actual status at merge time (depends on T017)
- [x] T020 Run the full integration test suite: `cd scenario-a && go test ./...` — confirm no regressions in any unrelated test (depends on T018, T019)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all story work; tests T002–T004 written first (parallel), then T005 (proto), then T006 (proto-gen)
- **US1 (Phase 3)**: Depends on Phase 2 complete (proto-gen done)
- **US2 (Phase 4)**: T012/T013 depend only on T006 (proto-gen) — can start in parallel with US1; T015 depends on T010 (server routing)
- **US3 (Phase 5)**: Depends on T009 (migration SQL written) — can start immediately after T009
- **Polish (Phase 6)**: Depends on all story phases complete

### User Story Dependencies

- **US1 (P1)**: Starts after T006 (proto-gen) — no dependency on US2 or US3
- **US2 (P1)**: T012/T013 start after T006 (proto-gen) in parallel with US1; T015 needs T010
- **US3 (P2)**: Starts after T009 (migration written) — depends on US1 repository work

### Within Each Phase

- Tests (T002, T003, T004) MUST be written before proto change (T005)
- Domain → Model → Repository → Server (T007 → T008 → T009 → T010) — sequential within US1
- T012 and T013 are independent of each other — parallel within US2
- T016 and T017 are sequential within US3

### Parallel Opportunities

- **Phase 2**: T002, T003, T004 can all be written in parallel (different test files)
- **Phase 3+4**: After T006, T007–T010 (US1) and T012–T013 (US2) can proceed in parallel if two developers are available
- **Phase 5**: T016 can start as soon as T009 is done, overlapping with T010
- **Phase 6**: T018 and T019 are independent of each other

---

## Parallel Example: After Proto-Gen (T006 complete)

```bash
# Developer A: US1 Go backend (sequential within US1)
Task T007: Update domain/fx.go
Task T008: Update fx_agreement_models.go
Task T009: Add startup SQL migration to fx_agreement_gorm.go
Task T010: Update server.go (14 refs + routing + validation)
Task T011: go test ./... green

# Developer B: US2 relay + frontend (parallel)
Task T012: Update htlc-relay.ts
Task T013: Update fx-agreement.types.ts
Task T014: tsc --noEmit passes
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational — failing tests + proto + proto-gen (T002–T006)
3. Complete Phase 3: US1 Go backend (T007–T011)
4. **STOP and VALIDATE**: `ProposeFXAgreement` + `GetFXAgreement` work with spoke-keyed fields; T002 and T004 GREEN
5. Demo / deploy

### Incremental Delivery

1. Phase 1 + 2 → proto gate passed
2. Phase 3 (US1) → Go backend functional — **MVP** for new-agreement spoke routing
3. Phase 4 (US2) → Relay updated → cross-spoke HTLC works end-to-end
4. Phase 5 (US3) → Migration verified → safe to deploy against existing data
5. Phase 6 (Polish) → grep clean + README current

### Parallel Team Strategy

With two developers (after T006):
- **Dev A** handles US1 (T007–T011) sequentially
- **Dev B** handles T012–T013 (US2 relay+frontend) immediately, then T016 (US3 migration test) once T009 merges

---

## Notes

- [P] tasks operate on different files — no merge conflicts
- Every task in US1 is sequential (domain → model → repository → server)
- The backfill WHERE guard (`WHERE source_spoke_id = ''`) is the idempotency mechanism — do not remove it
- `spokePrefix` is already derived from `PALADIN_IDENTITY` at startup; no new env var is needed
- Proto3 `reserved` declarations are mandatory — do not skip them even if "it compiles without them"
- Commit after T006 (proto-gen), after T011 (US1 green), and after T015/T017 (US2/US3 green)

---

## Phase 7: Convergence

- [x] T021 CRITICAL: In `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts` update the `log.info` at line 614 (proposal-forwarding event) to include `dest_spoke_id` and `source_spoke_id` from the parsed event — required by Constitution VI to reconstruct the full settlement lifecycle from logs per Constitution VI (missing)
- [x] T022 In `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts` add a guard after the REST parsing block (lines 607-608 / their replacement in T012): if `sourceSpokeId` or `destSpokeId` is empty after parsing, emit `this.log.error(...)` and `continue` to skip the event without crashing — satisfies US2/AC2 acceptance scenario per US2/AC2 (missing)
- [x] T023 Add a verification step for SC-003 (manual verification checklist added to plan.md): either (a) write an integration test in `scenario-a/tests/integration/` that seeds a full FX Agreement with two distinct spoke IDs and drives propose → accept → lock → settle, or (b) add an explicit manual verification checklist to `specs/020-fx-spoke-keyed-legs/plan.md` documenting the expected commands and assertions — required before this feature is declared complete per SC-003 (missing)

---

## Phase 8: Convergence

- [x] T024 Update `scenario-a/apis/openapi/api-gateway.yaml`: in both the `FXAgreement` schema (lines ~4390/4394) and the `ProposeFXAgreementRequest` schema (lines ~4521/4525), replace `spoke_a_receiver` and `spoke_b_receiver` with `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`; update descriptions accordingly per SC-001 / FR-001 (missing)
- [x] T025 Update `scenario-a/tryouts/tryout-fx-agreement-e2e.sh`: at lines 256–257 replace `spoke_a_receiver: $sra` and `spoke_b_receiver: $srb` with `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` using appropriate variable assignments; update any variable declarations that feed these fields per SC-001 / SC-002 (missing)
- [x] T026 Update `scenario-a/docs/charts/scenario-a/transfer.md`: replace all three remaining references to `spoke_a_receiver`/`spoke_b_receiver` (lines ~38, ~73, ~108) with `source_spoke_id`/`dest_spoke_id`/`source_receiver`/`dest_receiver` and update the validation note text per SC-001 / Constitution §VI (missing)
