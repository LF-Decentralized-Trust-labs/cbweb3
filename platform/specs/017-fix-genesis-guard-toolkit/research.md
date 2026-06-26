# Research: Genesis Existence Guard in Provisioning Toolkit

**Phase 0 Output** | Feature: `017-fix-genesis-guard-toolkit` | Date: 2026-06-25

---

## Finding 1 — Toolkit structure and pattern

**Decision**: The `GenesisGuard` is implemented as a new Go package `engine/genesis` inside `scenario-a/toolkit/`, following the exact same pattern as `engine/pki/`.

**Rationale**: The PKI package (`engine/pki/csr.go`) establishes the toolkit's convention:
1. A contract file defines exported function signatures with "not implemented" stubs.
2. A separate `_test.go` file contains RED-phase tests that document required behavior.
3. Implementation is delivered in a subsequent PR that turns tests GREEN.

This feature follows the same pattern: `guard.go` (contract + implementation) and `guard_test.go` (RED-phase tests first, then GREEN). The module path is already established: `github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit`.

**Alternatives considered**:
- Shell script guard (early exit in a wrapper around `startBesu.sh`): rejected — the toolkit is Go, and the design principle explicitly states the sample scripts must stay untouched.
- Guard embedded in the manifest `apply` command directly: rejected — the guard must be reusable from any toolkit entry point (FR-007), which requires a dedicated, importable package.

---

## Finding 2 — Genesis path convention

**Decision**: The canonical genesis-complete signal is the presence of a non-empty, parseable file at `{workspacePath}/genesis/genesis.json`. This is consistent with how `startBesu.sh` uses the path.

**Evidence**:
- `startBesu.sh` line 119–121: `mkdir -p genesis && cp tmpFiles/networkFiles/genesis.json genesis/genesis.json`
- `startBesu.sh` lines 146, 207, 233: all Besu container start commands use `--genesis-file=genesis/genesis.json`

**Rationale**: The spoke workspace directory is the directory from which `startBesu.sh` is run (all paths are relative). The toolkit accepts `workspacePath` as a parameter and constructs the full path internally — never hardcodes it.

**Alternatives considered**:
- Using `tmpFiles/networkFiles/genesis.json` as the signal: rejected — `tmpFiles/` is a scratch directory that `startBesu.sh` creates and the toolkit should clean up. The stable output is `genesis/genesis.json`.
- Using a `.genesis-complete` sentinel file: rejected — unnecessary indirection; the actual genesis file's presence is the authoritative signal.

---

## Finding 3 — Destructive operation identification

**Decision**: The single line to guard against is `besu operator generate-blockchain-config` (line 93 of `startBesu.sh`). In the toolkit, any invocation of this command or equivalent logic is the "destructive operation" that the guard must block.

**Evidence**:
- `startBesu.sh` line 93–97 (verified):
  ```bash
  ../$BESU operator generate-blockchain-config \
      --config-file=../config/qbftConfigFile.json \
      --to=networkFiles \
      --private-key-file-name=key
  ```
  This command (a) generates new QBFT validator keys, (b) generates a new genesis block, and (c) if existing node keys are present, overwrites them — destroying any previously mined chain state.
- It runs unconditionally every time `startBesu.sh` is called.

**Rationale**: The toolkit must never invoke this command when genesis already exists. The guard function returns a `GuardDecision` before any orchestration step; the engine must check the decision before invoking this command.

---

## Finding 4 — No existing genesis guard

**Decision**: This feature implements the guard from scratch. There is no existing genesis check in the toolkit or its dependencies.

**Evidence**: `find scenario-a/toolkit -type f` returns only:
- `engine/pki/csr.go`
- `engine/pki/csr_test.go`
- `engine/pki/helpers_test.go`
- `go.mod`

No files reference genesis, `generate-blockchain-config`, or idempotency logic.

---

## Finding 5 — Structured log format

**Decision**: Log events follow the project's structured JSON convention: `{"event":"genesis_skipped","spoke_id":"...","genesis_path":"...","timestamp":"...","severity":"INFO","service":"provisioning-toolkit"}`.

**Rationale**: Consistent with Constitution Principle VI — all services must emit structured JSON logs to stdout with service name, severity, ISO-8601 timestamp. The toolkit follows this even though it is not a long-running microservice.

**Alternatives considered**:
- Plain text log: rejected — unstructured logs cannot be parsed by automated CI assertions (SC-002).
- slog (Go 1.21+ stdlib): selected as the logging mechanism (no new dependency, fits Go 1.26+ constraint).
