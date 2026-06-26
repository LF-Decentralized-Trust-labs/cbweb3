# Implementation Plan: Genesis Existence Guard in Provisioning Toolkit

**Branch**: `017-fix-genesis-guard-toolkit` | **Date**: 2026-06-25 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `specs/017-fix-genesis-guard-toolkit/spec.md`

---

## Summary

Add a `GenesisGuard` to the Scenario A provisioning toolkit's orchestration engine that detects whether a spoke's genesis file already exists before any initialization step and refuses destructive execution (regenerating genesis on a running chain). The guard is a standalone Go package under `scenario-a/toolkit/engine/genesis/` following the same contract-first, test-first pattern established by `engine/pki/`.

---

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Go stdlib only (`encoding/json`, `os`, `io`, `log/slog`, `time`)  
**Storage**: Filesystem — `{workspacePath}/genesis/genesis.json` (read-only from the guard's perspective)  
**Testing**: `go test ./engine/genesis/...` (RED-phase tests first, implementation turns them GREEN)  
**Target Platform**: Linux (local dev) and staging/prod (same guard, environment is a parameter)  
**Project Type**: Library package within the standalone provisioning toolkit CLI  
**Performance Goals**: Guard must complete in < 100 ms (filesystem stat + JSON parse of a small file)  
**Constraints**: Zero new external dependencies; must not modify any file (read-only); must emit exactly one structured log event per call

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Verified 2026-06-25.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Scenario-Scoped Independence | ✅ PASS | Entirely within `scenario-a/toolkit/`. No cross-scenario code. |
| II. Privacy by Design | ✅ PASS (N/A) | No value transfers or on-chain data. Infrastructure layer only. |
| III. Atomic Settlement Guarantee | ✅ PASS (N/A) | No settlement protocol involvement. Pre-provisioning guard. |
| IV. Compliance Gate Before Participation | ✅ PASS (N/A) | Operates below the identity/compliance layer. |
| V. Test-First at Every Layer | ✅ PASS | `guard_test.go` with failing tests is Phase 1, Step 1. Implementation is Step 2. |
| VI. Observability and Auditability | ✅ PASS | Spec requires structured JSON log event (`genesis_skipped`, `genesis_proceed`, `genesis_error`) per call; the engine emits `genesis_created` after generation completes. |

**All gates pass. No Complexity Tracking entries required.**

*Post-design re-check (Phase 1 complete)*: Same result. The `engine/genesis` package introduces no new technology and no cross-principle concerns.

---

## Project Structure

### Documentation (this feature)

```text
specs/017-fix-genesis-guard-toolkit/
├── plan.md              ← this file
├── research.md          ← Phase 0 output
├── data-model.md        ← Phase 1 output
└── tasks.md             ← Phase 2 output (/speckit.tasks — not yet created)
```

### Source Code

```text
scenario-a/toolkit/
├── go.mod                          (existing — no changes)
└── engine/
    ├── pki/                        (existing — untouched)
    │   ├── csr.go
    │   ├── csr_test.go
    │   └── helpers_test.go
    └── genesis/                    (NEW — this feature)
        ├── guard.go                contract types + guard implementation
        └── guard_test.go           RED-phase contract tests
```

No other directories are touched. The existing `deploy/local/spoke-besu-*/startBesu.sh` sample scripts are **not modified** (per design principle in task-notions.md §4).

**Structure Decision**: Single package `engine/genesis` within the existing Go module. Mirrors the `engine/pki` pattern exactly.

---

## Implementation Phases

### Phase 1 — Contract Tests (RED)

**Files**: `scenario-a/toolkit/engine/genesis/guard_test.go`

Write all contract tests **before** any implementation code. Tests must fail with "not implemented" at this step.

Tests to implement (following `csr_test.go` style):

1. **`TestGuardGenesis_AbsentGenesis_Proceed`** — Given a workspace with no `genesis/genesis.json`, `GuardGenesis` returns `DecisionProceed`.
2. **`TestGuardGenesis_PresentGenesis_Skip`** — Given a workspace with a valid `genesis/genesis.json`, `GuardGenesis` returns `DecisionSkip` and does not modify the file.
3. **`TestGuardGenesis_PresentGenesis_EmitsSkipEvent`** — Given a valid genesis, the log output to `w` contains a JSON object with `event=genesis_skipped`, correct `spoke_id`, `genesis_path`, and an ISO-8601 `timestamp`.
4. **`TestGuardGenesis_CorruptGenesis_Abort`** — Given an empty `genesis/genesis.json`, `GuardGenesis` returns `DecisionAbort` (not an error) and emits `genesis_error`.
5. **`TestGuardGenesis_CorruptGenesis_ForceReinit_Proceed`** — Given a corrupt genesis and `forceReinit=true`, returns `DecisionProceed`.
6. **`TestGuardGenesis_PresentGenesis_IsReadOnly`** — After `GuardGenesis` runs on a present genesis, the file content is byte-for-byte identical to the original (guard must not modify files).
7. **`TestCheckGenesis_AbsentReturnsAbsent`** — `CheckGenesis` returns `GenesisAbsent` when file does not exist.
8. **`TestCheckGenesis_PresentReturnsPresent`** — `CheckGenesis` returns `GenesisPresent` for a valid JSON file.
9. **`TestCheckGenesis_EmptyReturnsCorrupt`** — `CheckGenesis` returns `GenesisCorrupt` for a zero-byte file.
10. **`TestCheckGenesis_InvalidJSONReturnsCorrupt`** — `CheckGenesis` returns `GenesisCorrupt` for a non-JSON file.

Verify tests fail: `go test ./engine/genesis/... 2>&1` should show "not implemented" failures.

### Phase 2 — Contract Definition (stub)

**File**: `scenario-a/toolkit/engine/genesis/guard.go`

Define exported types (`GenesisState`, `GuardDecision`, `GuardResult`) and function signatures with stub bodies that return `errors.New("not implemented: ...")`. This mirrors `engine/pki/csr.go` exactly.

```go
// Package genesis provides the genesis-existence guard for the Scenario A
// provisioning toolkit. The guard prevents destructive regeneration of
// genesis/genesis.json on a spoke that has already been initialized.
package genesis

// GenesisState ...
// GuardDecision ...
// GuardResult ...

func CheckGenesis(workspacePath string) (GenesisState, error) {
    return 0, errors.New("not implemented: CheckGenesis")
}

func GuardGenesis(spokeID, workspacePath string, forceReinit bool, w io.Writer) (GuardResult, error) {
    return GuardResult{}, errors.New("not implemented: GuardGenesis")
}
```

Tests remain RED. This step separates contract definition from implementation.

### Phase 3 — Implementation (GREEN)

**File**: `scenario-a/toolkit/engine/genesis/guard.go` (expand stubs)

Implement `CheckGenesis`:
1. `os.Stat(filepath.Join(workspacePath, "genesis", "genesis.json"))` — if `os.IsNotExist` → `GenesisAbsent`
2. Read file; if zero bytes → `GenesisCorrupt`
3. `json.Valid(data)` — if false → `GenesisCorrupt`
4. Otherwise → `GenesisPresent`

Implement `GuardGenesis`:
1. Call `CheckGenesis` to get state.
2. Determine `GuardDecision` from state and `forceReinit` (per state machine in data-model.md).
3. Map decision to log event name and severity.
4. Emit one `GenesisLogEvent` JSON line to `w` using `slog` or `json.Marshal`.
5. Return `GuardResult` with decision, reason, path, and spokeID.

Run tests: `go test ./engine/genesis/... -v` — all 10 tests must pass GREEN.

### Phase 4 — Integration Point Documentation

**File**: `scenario-a/toolkit/engine/genesis/guard.go` (doc comment update)

Add a package-level comment that documents exactly where in the orchestration sequence `GuardGenesis` must be called:

> The engine's `apply` command MUST call `GuardGenesis` as its first action, before any of:
> rendering compose templates, invoking `besu operator generate-blockchain-config`,
> writing node key files, or starting containers.
> Only `DecisionProceed` allows the engine to continue to genesis generation.

This is not implementation code — it is a contract comment that guides TK-4/TK-5 implementers. No engine code is written in this PR (the engine entrypoints are a separate feature).

---

## Out of Scope

- The manifest schema (TK-4) — separate PR.
- The orchestration engine entrypoints that call `GuardGenesis` (TK-5) — those PRs import this package.
- The `--force-reinit` CLI flag wiring — belongs in the engine CLI PR (TK-5).
- Cleanup of `tmpFiles/` on first-time genesis generation — that is orchestration logic, not the guard's concern.
- The existing `deploy/local/spoke-besu-*/startBesu.sh` scripts — must not be modified (design principle §4 in task-notions.md).
