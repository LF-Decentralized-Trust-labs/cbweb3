# Data Model: Genesis Existence Guard

**Feature**: `017-fix-genesis-guard-toolkit` | Date: 2026-06-25

---

## Entities

### GenesisState (enumeration)

Represents what the guard found at the genesis path on disk.

| Value | Meaning |
|-------|---------|
| `GenesisAbsent` | `genesis/genesis.json` does not exist; generation is allowed |
| `GenesisPresent` | `genesis/genesis.json` exists and is valid JSON with non-zero size; generation must be skipped |
| `GenesisCorrupt` | `genesis/genesis.json` exists but is empty or not parseable as JSON; abort unless `--force-reinit` |

---

### GuardDecision (enumeration)

The action the orchestration engine must take based on `GenesisState` and the `forceReinit` flag.

| Value | Meaning |
|-------|---------|
| `DecisionProceed` | Genesis is absent; run `besu operator generate-blockchain-config` and subsequent steps |
| `DecisionSkip` | Genesis is present and valid; skip generation entirely and proceed to idempotent steps |
| `DecisionAbort` | Genesis is corrupt, or a destructive operation was attempted without `forceReinit`; exit non-zero |

**State machine:**

```
GenesisAbsent  → DecisionProceed
GenesisPresent + forceReinit=false → DecisionSkip
GenesisPresent + forceReinit=true  → (require confirmation) → DecisionProceed
GenesisCorrupt + forceReinit=false → DecisionAbort
GenesisCorrupt + forceReinit=true  → (require confirmation) → DecisionProceed
```

---

### GuardResult (value object)

Returned by `GuardGenesis`. Carries the decision and enough information to log a structured event.

| Field | Type | Description |
|-------|------|-------------|
| `Decision` | `GuardDecision` | What the engine should do next |
| `Reason` | string | Human-readable reason code (e.g., `"genesis already present"`, `"genesis absent"`, `"genesis corrupt: empty file"`) |
| `GenesisPath` | string | Absolute path that was checked |
| `SpokeID` | string | Spoke identifier (passed in by the caller for log context) |

---

### GenesisLogEvent (structured log record)

Emitted to stdout as a single-line JSON object on every `GuardGenesis` call.

| Field | Type | Example |
|-------|------|---------|
| `event` | string | `"genesis_skipped"` / `"genesis_created"` / `"genesis_error"` |
| `spoke_id` | string | `"spoke-brl"` |
| `genesis_path` | string | `"/workspace/spoke-brl/genesis/genesis.json"` |
| `timestamp` | string (ISO-8601) | `"2026-06-25T14:30:00Z"` |
| `severity` | string | `"INFO"` / `"ERROR"` |
| `service` | string | `"provisioning-toolkit"` |
| `reason` | string | `"genesis already present"` |

Event mapping:
- `DecisionSkip` → `genesis_skipped` (severity: INFO)
- `DecisionProceed` (after first-time generation) → `genesis_created` (severity: INFO)
- `DecisionAbort` → `genesis_error` (severity: ERROR)

---

## Package Contract

Package: `github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/genesis`

```go
// CheckGenesis inspects workspacePath/genesis/genesis.json and returns its state.
// It does NOT log; callers use the returned GenesisState to build log events.
func CheckGenesis(workspacePath string) (GenesisState, error)

// GuardGenesis is the main entry point for the engine.
// It checks genesis state, applies the forceReinit flag, emits a structured log
// event to w (pass os.Stdout in production), and returns a GuardResult.
// It returns a non-nil error only for filesystem I/O failures unrelated to
// genesis state (e.g., permission denied reading the directory).
func GuardGenesis(spokeID, workspacePath string, forceReinit bool, w io.Writer) (GuardResult, error)
```

**Key invariants**:
- `GuardGenesis` MUST emit exactly one log event per call.
- `GuardGenesis` MUST return `DecisionAbort` (not an error) when genesis is corrupt and `forceReinit` is false.
- `GuardGenesis` MUST NOT modify any file; it is read-only.
- `CheckGenesis` is exported for use in CLI flag validation before `GuardGenesis` is called.

---

## Files Produced

```
scenario-a/toolkit/
└── engine/
    └── genesis/
        ├── guard.go        — GenesisState, GuardDecision, GuardResult, CheckGenesis, GuardGenesis
        └── guard_test.go   — Contract tests (RED phase); turn GREEN when guard.go is implemented
```

No new external dependencies. Uses only Go stdlib: `encoding/json`, `os`, `io`, `log/slog`, `time`.
