# Tasks: Genesis Existence Guard in Provisioning Toolkit

**Input**: Design documents from `specs/017-fix-genesis-guard-toolkit/`  
**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅ data-model.md ✅

**Tests**: Included — Constitution Principle V mandates test-first at every layer; tests are part of the spec (RED before GREEN).

**Organization**: Tasks follow the RED → stub → GREEN cycle from plan.md, grouped by user story for independent delivery.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different concerns, no blocking dependency on another incomplete task)
- **[Story]**: Which user story this task belongs to ([US1], [US2], [US3])
- All paths are relative to `scenario-a/toolkit/`

---

## Phase 1: Setup

**Purpose**: Create the `engine/genesis` package so the Go module compiles.

- [x] T001 Create directory `scenario-a/toolkit/engine/genesis/` (no files yet — just the directory)
- [x] T002 Verify the toolkit module compiles from `scenario-a/toolkit/`: `go build ./...` must exit 0 (empty package OK once guard.go stub exists)

---

## Phase 2: Foundational — Contract Types + Stubs

**Purpose**: Define all exported types and stub function bodies so the test file can be written and the package compiles. All user stories depend on these types.

**⚠️ CRITICAL**: No user story tests can be written until this phase is complete — Go tests must import a compilable package.

- [x] T003 Define `GenesisState` type with constants `GenesisAbsent`, `GenesisPresent`, `GenesisCorrupt` in `scenario-a/toolkit/engine/genesis/guard.go`
- [x] T004 Define `GuardDecision` type with constants `DecisionProceed`, `DecisionSkip`, `DecisionAbort` in `scenario-a/toolkit/engine/genesis/guard.go`
- [x] T005 Define `GuardResult` struct (`Decision GuardDecision`, `Reason string`, `GenesisPath string`, `SpokeID string`) in `scenario-a/toolkit/engine/genesis/guard.go`
- [x] T006 Define `GenesisLogEvent` struct (`Event`, `SpokeID`, `GenesisPath`, `Timestamp`, `Severity`, `Service`, `Reason` fields — all string) in `scenario-a/toolkit/engine/genesis/guard.go`
- [x] T007 Add `CheckGenesis(workspacePath string) (GenesisState, error)` stub body returning `errors.New("not implemented: CheckGenesis")` in `scenario-a/toolkit/engine/genesis/guard.go`
- [x] T008 Add `GuardGenesis(spokeID, workspacePath string, forceReinit bool, w io.Writer) (GuardResult, error)` stub body returning `errors.New("not implemented: GuardGenesis")` in `scenario-a/toolkit/engine/genesis/guard.go`
- [x] T009 Verify `go build ./engine/genesis/` exits 0 and `go vet ./engine/genesis/` reports no issues

**Checkpoint**: Package compiles. Function stubs return "not implemented". No tests yet.

---

## Phase 3: User Story 1 — Idempotent Skip on Existing Genesis (Priority: P1) 🎯 MVP

**Goal**: `GuardGenesis` detects a valid `genesis/genesis.json` and returns `DecisionSkip` without modifying any file.

**Independent Test**: Run `go test ./engine/genesis/... -run "TestCheckGenesis_PresentReturnsPresent|TestGuardGenesis_PresentGenesis"` — all must pass GREEN.

### Tests for User Story 1 (RED first)

> **Write these tests FIRST, confirm they FAIL with "not implemented" before implementing**

- [x] T010 [US1] Write `TestCheckGenesis_PresentReturnsPresent` in `scenario-a/toolkit/engine/genesis/guard_test.go`: create a temp dir with a valid `genesis/genesis.json` (non-empty JSON), call `CheckGenesis`, assert `GenesisPresent` is returned
- [x] T011 [US1] Write `TestGuardGenesis_PresentGenesis_Skip` in `scenario-a/toolkit/engine/genesis/guard_test.go`: valid genesis present, `forceReinit=false` → assert `GuardResult.Decision == DecisionSkip`
- [x] T012 [US1] Write `TestGuardGenesis_PresentGenesis_IsReadOnly` in `scenario-a/toolkit/engine/genesis/guard_test.go`: record SHA-256 of genesis file before calling `GuardGenesis`, assert byte-for-byte identical after — guard must not write any file
- [x] T013 [US1] Run `go test ./engine/genesis/... -run "TestCheckGenesis_PresentReturnsPresent|TestGuardGenesis_PresentGenesis"` and confirm all 3 tests FAIL with "not implemented"

### Implementation for User Story 1

- [x] T014 [US1] Implement `CheckGenesis` (GenesisPresent path) in `scenario-a/toolkit/engine/genesis/guard.go`: `os.Stat(filepath.Join(workspacePath, "genesis", "genesis.json"))` — if file exists, non-zero size, and `json.Valid(data)` → return `GenesisPresent`
- [x] T015 [US1] Implement `GuardGenesis` (DecisionSkip path) in `scenario-a/toolkit/engine/genesis/guard.go`: if `CheckGenesis` returns `GenesisPresent` and `forceReinit=false` → populate `GuardResult{Decision: DecisionSkip, Reason: "genesis already present", GenesisPath: ..., SpokeID: spokeID}` and return it (log emission comes in US3)
- [x] T016 [US1] Run `go test ./engine/genesis/... -run "TestCheckGenesis_PresentReturnsPresent|TestGuardGenesis_PresentGenesis" -v` and confirm all 3 tests pass GREEN

**Checkpoint**: User Story 1 fully functional. An idempotent `apply` against an existing spoke skips genesis generation.

---

## Phase 4: User Story 2 — Clear Refusal on Destructive Operation (Priority: P1)

**Goal**: `GuardGenesis` refuses (returns `DecisionAbort`) when genesis is corrupt; returns `DecisionProceed` for absent genesis; honours `--force-reinit` escape hatch.

**Independent Test**: Run `go test ./engine/genesis/... -run "TestCheckGenesis_Absent|TestCheckGenesis_Empty|TestCheckGenesis_InvalidJSON|TestGuardGenesis_AbsentGenesis|TestGuardGenesis_CorruptGenesis"` — all must pass GREEN.

### Tests for User Story 2 (RED first)

> **Write these tests FIRST, confirm they FAIL with "not implemented" before implementing**

- [x] T017 [P] [US2] Write `TestCheckGenesis_AbsentReturnsAbsent` in `scenario-a/toolkit/engine/genesis/guard_test.go`: temp dir with no genesis file → assert `GenesisAbsent`
- [x] T018 [P] [US2] Write `TestCheckGenesis_EmptyReturnsCorrupt` in `scenario-a/toolkit/engine/genesis/guard_test.go`: temp dir with a zero-byte `genesis/genesis.json` → assert `GenesisCorrupt`
- [x] T019 [P] [US2] Write `TestCheckGenesis_InvalidJSONReturnsCorrupt` in `scenario-a/toolkit/engine/genesis/guard_test.go`: temp dir with `genesis/genesis.json` containing `not-json` → assert `GenesisCorrupt`
- [x] T020 [US2] Write `TestGuardGenesis_AbsentGenesis_Proceed` in `scenario-a/toolkit/engine/genesis/guard_test.go`: no genesis file, `forceReinit=false` → assert `DecisionProceed`
- [x] T021 [US2] Write `TestGuardGenesis_CorruptGenesis_Abort` in `scenario-a/toolkit/engine/genesis/guard_test.go`: zero-byte genesis, `forceReinit=false` → assert `DecisionAbort` (not an error return)
- [x] T022 [US2] Write `TestGuardGenesis_CorruptGenesis_ForceReinit_Proceed` in `scenario-a/toolkit/engine/genesis/guard_test.go`: zero-byte genesis, `forceReinit=true` → assert `DecisionProceed`
- [x] T023 [US2] Run `go test ./engine/genesis/... -run "TestCheckGenesis_Absent|TestCheckGenesis_Empty|TestCheckGenesis_InvalidJSON|TestGuardGenesis_AbsentGenesis|TestGuardGenesis_CorruptGenesis"` and confirm all 6 tests FAIL with "not implemented"

### Implementation for User Story 2

- [x] T024 [US2] Implement `CheckGenesis` (GenesisAbsent path) in `scenario-a/toolkit/engine/genesis/guard.go`: `os.IsNotExist` on stat error → return `GenesisAbsent`
- [x] T025 [US2] Implement `CheckGenesis` (GenesisCorrupt path) in `scenario-a/toolkit/engine/genesis/guard.go`: file exists but zero bytes or `!json.Valid(data)` → return `GenesisCorrupt`
- [x] T026 [US2] Implement `GuardGenesis` (DecisionAbort path) in `scenario-a/toolkit/engine/genesis/guard.go`: `GenesisCorrupt` + `forceReinit=false` → `GuardResult{Decision: DecisionAbort, Reason: "genesis corrupt: <detail>", ...}`
- [x] T027 [US2] Implement `GuardGenesis` (DecisionProceed path) in `scenario-a/toolkit/engine/genesis/guard.go`: `GenesisAbsent` → `DecisionProceed`; `GenesisPresent` or `GenesisCorrupt` + `forceReinit=true` → `DecisionProceed`
- [x] T028 [US2] Run `go test ./engine/genesis/... -run "TestCheckGenesis_Absent|TestCheckGenesis_Empty|TestCheckGenesis_InvalidJSON|TestGuardGenesis_AbsentGenesis|TestGuardGenesis_CorruptGenesis" -v` and confirm all 6 tests pass GREEN

**Checkpoint**: User Stories 1 and 2 fully functional. All guard decisions work correctly.

---

## Phase 5: User Story 3 — Audit Trail (Structured Log Events) (Priority: P2)

**Goal**: `GuardGenesis` emits exactly one `GenesisLogEvent` JSON object to `w` on every call, with the correct `event`, `spoke_id`, `genesis_path`, `timestamp`, `severity`, and `service` fields.

**Independent Test**: Run `go test ./engine/genesis/... -run "TestGuardGenesis_PresentGenesis_EmitsSkipEvent"` — must pass GREEN.

### Test for User Story 3 (RED first)

> **Write this test FIRST, confirm it FAILS before implementing**

- [x] T029 [US3] Write `TestGuardGenesis_PresentGenesis_EmitsSkipEvent` in `scenario-a/toolkit/engine/genesis/guard_test.go`: call `GuardGenesis` with valid genesis, capture output in a `bytes.Buffer` passed as `w`, unmarshal JSON, assert `event=="genesis_skipped"`, `spoke_id` matches input, `genesis_path` is non-empty, `timestamp` parses as RFC3339, `severity=="INFO"`, `service=="provisioning-toolkit"`
- [x] T030 [US3] Run `go test ./engine/genesis/... -run TestGuardGenesis_PresentGenesis_EmitsSkipEvent` and confirm it FAILS with "not implemented"

### Implementation for User Story 3

- [x] T031 [US3] Implement log event emission in `GuardGenesis` in `scenario-a/toolkit/engine/genesis/guard.go`: build a `GenesisLogEvent`, marshal to JSON with `json.Marshal`, write single line to `w` (`fmt.Fprintln(w, string(jsonBytes))`); map `DecisionSkip`→`genesis_skipped`/INFO, `DecisionProceed`→`genesis_proceed`/INFO, `DecisionAbort`→`genesis_error`/ERROR (the engine, not the guard, emits `genesis_created` after generation completes)
- [x] T032 [US3] Run `go test ./engine/genesis/... -run TestGuardGenesis_PresentGenesis_EmitsSkipEvent -v` and confirm GREEN

**Checkpoint**: All three user stories fully functional. All 10 contract tests pass GREEN.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finalize the package contract documentation and validate the full test suite.

- [x] T033 [P] Add package-level doc comment to `scenario-a/toolkit/engine/genesis/guard.go` documenting: (1) the genesis-complete signal (`genesis/genesis.json`), (2) the state machine (GenesisState → GuardDecision), (3) the integration-point instruction: "engine's `apply` MUST call GuardGenesis as its first action, before invoking `besu operator generate-blockchain-config` or any file write"
- [x] T034 Run full test suite from toolkit root: `go test ./...` — all tests pass GREEN, no regressions in `engine/pki/`
- [x] T035 Run `go vet ./...` from toolkit root — zero warnings
- [x] T036 [P] Verify zero external imports: `go list -m all` from `scenario-a/toolkit/` must show only the stdlib (no new entries in `go.mod`/`go.sum`)
- [x] T037 [P] Verify the `deploy/local/spoke-besu-a/startBesu.sh` and `deploy/local/spoke-besu-b/startBesu.sh` files are unmodified: `git diff -- scenario-a/deploy/local/` must show no changes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user story work
- **US1 (Phase 3)**: Depends on Phase 2 — can start as soon as stubs compile
- **US2 (Phase 4)**: Depends on Phase 2 — can start as soon as stubs compile (parallel with US1)
- **US3 (Phase 5)**: Depends on Phase 3 and 4 — log emission builds on the decision logic
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: Independent after Phase 2
- **US2 (P1)**: Independent after Phase 2 — can run in parallel with US1
- **US3 (P2)**: Depends on US1 and US2 (log emission requires all guard decision paths to exist)

### Within Each User Story

1. Write tests FIRST — confirm RED (not-implemented panic or compile error)
2. Implement to turn tests GREEN
3. Run targeted test command to confirm checkpoint

### Parallel Opportunities

- T017, T018, T019 within US2 are [P] — each tests a different `CheckGenesis` path
- T033, T036, T037 in Polish are [P] — different files, no shared state
- US1 and US2 can be worked in parallel by two developers after Phase 2 completes

---

## Parallel Example: User Story 2 Test Writing

```bash
# These 3 tests touch the same file (guard_test.go) but test independent CheckGenesis paths.
# With a single developer, write them sequentially.
# With pair programming, one writes absence tests while the other writes corrupt tests.

Task T017: TestCheckGenesis_AbsentReturnsAbsent  → GenesisAbsent path
Task T018: TestCheckGenesis_EmptyReturnsCorrupt   → GenesisCorrupt (zero bytes)
Task T019: TestCheckGenesis_InvalidJSONReturnsCorrupt → GenesisCorrupt (bad JSON)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T002)
2. Complete Phase 2: Foundational stubs (T003–T009)
3. Complete Phase 3: US1 — idempotent skip (T010–T016)
4. **STOP and VALIDATE**: `go test ./engine/genesis/... -run "TestCheckGenesis_PresentReturnsPresent|TestGuardGenesis_PresentGenesis" -v`
5. The guard can be imported by TK-5 for the idempotent-apply path

### Incremental Delivery

1. Setup + Foundational → package compiles
2. US1 complete → skip path works; TK-5 can begin wiring `DecisionSkip`
3. US2 complete → abort path works; TK-5 can wire `DecisionAbort` exit code
4. US3 complete → log events visible in CI and pilot runs
5. Polish → doc complete, no regressions

---

## Notes

- [P] tasks = different files or independently writable sections; no blocking dependency on another incomplete task in the same phase
- Every test must be confirmed RED (failing) before implementation — this is Constitution Principle V, not optional
- `guard.go` is the single implementation file; all phases expand it — never shrink it
- `startBesu.sh` files MUST remain untouched throughout (verified by T037)
- The `w io.Writer` parameter on `GuardGenesis` makes the log testable without capturing stdout — always pass `os.Stdout` in production callers and a `bytes.Buffer` in tests

---

## Phase 7: Convergence

**Purpose**: Close remaining gaps identified by `/speckit-converge` — silent error swallowing (Constitution VI) and incomplete log-event test coverage (plan Phase 1 test 4).

- [x] T038 [P] Fix silent error swallowing in `emitLogEvent` in `scenario-a/toolkit/engine/genesis/guard.go`: if `json.Marshal` or `fmt.Fprintln` fail, log a fallback error to `w` (e.g., `{"event":"genesis_error","severity":"ERROR","reason":"failed to marshal/write log event"}`) instead of silently returning, per Constitution VI (contradicts)
- [x] T039 [P] Add log-event assertion to `TestGuardGenesis_CorruptGenesis_Abort` in `scenario-a/toolkit/engine/genesis/guard_test.go`: verify that the output buffer contains a `GenesisLogEvent` with `event=="genesis_error"`, `severity=="ERROR"`, and a non-empty `Reason` field, per plan Phase 1 test 4 spec (partial)
- [x] T040 Run `go test ./engine/genesis/... -v` and confirm all tests pass GREEN including the updated ones
