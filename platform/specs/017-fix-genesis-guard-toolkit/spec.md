# Feature Specification: Genesis Existence Guard in Provisioning Toolkit

**Feature Branch**: `017-fix-genesis-guard-toolkit`  
**Created**: 2026-06-25  
**Status**: Draft  
**Input**: FIX-2 — NÃO REPLICAR: regeneração de genesis no toolkit. The deploy/local/spoke-besu-a/startBesu.sh line 93 runs `besu operator generate-blockchain-config` and regenerates the genesis on every execution, destroying chain state. The provisioning toolkit (TK-4/TK-5 engine) must check whether genesis already exists before any step, refuse destructive execution, and never replicate this behavior.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Idempotent Apply on Existing Spoke (Priority: P1)

An operator runs `apply` on a spoke that has already been initialized. The toolkit detects that the genesis file is already present and skips genesis generation entirely — leaving the running chain's state intact.

**Why this priority**: Without this guard, a second `apply` call — whether accidental, automated, or part of a CI reconciliation loop — would destroy the blockchain's entire history. This is the most critical safety invariant for the provisioning toolkit and is a direct blocker for pilot testing.

**Independent Test**: Can be tested by running the toolkit `apply` command twice against the same spoke directory; the second run must produce zero changes to the genesis file and zero disruption to any running Besu node.

**Acceptance Scenarios**:

1. **Given** a spoke directory with `genesis/genesis.json` already present, **When** the operator runs the toolkit `apply` command, **Then** the toolkit skips genesis generation, emits a clear log message stating genesis already exists, and does not modify any node key or genesis file.
2. **Given** a spoke with a running Besu node and an existing genesis file, **When** `apply` is run again, **Then** no container is stopped, no key is overwritten, and the node continues producing blocks without interruption.
3. **Given** a spoke directory that contains partial genesis artifacts (e.g., `genesis/genesis.json` exists but `tmpFiles/` does not), **When** `apply` is run, **Then** the toolkit treats genesis as complete and does not attempt to regenerate it.

---

### User Story 2 - Clear Refusal on Destructive Command (Priority: P1)

An operator explicitly or accidentally triggers a toolkit command that would attempt to regenerate genesis on an existing spoke. The toolkit refuses the operation with a clear, actionable error message.

**Why this priority**: Silent destruction is worse than noisy refusal. The operator must know exactly why the toolkit stopped and what to do next (e.g., use a `--nuke` or `--force-reinit` flag to explicitly wipe state).

**Independent Test**: Can be tested by calling a hypothetical `toolkit init --force` or any code path that reaches the genesis-generation step on a spoke that already has `genesis/genesis.json`; the toolkit must exit with a non-zero status and a human-readable error.

**Acceptance Scenarios**:

1. **Given** a spoke with an existing genesis file, **When** any toolkit command reaches the genesis-generation step, **Then** the toolkit exits with a non-zero status code and an error message that names the existing genesis file path and instructs the operator on how to proceed.
2. **Given** a spoke with no existing genesis, **When** `apply` runs for the first time, **Then** genesis generation proceeds normally without any refusal.
3. **Given** a operator who wants to intentionally wipe and reinitialize a spoke, **When** they provide an explicit destructive flag (`--force-reinit` or equivalent), **Then** the toolkit warns once, requires confirmation, and only then regenerates genesis.

---

### User Story 3 - Audit Trail for Skipped Generation (Priority: P2)

Every time the toolkit skips genesis generation because a genesis file already exists, it records a structured log entry that captures the spoke identifier, the path of the detected genesis file, and the timestamp of the skip.

**Why this priority**: Operators and CI systems need to distinguish between "genesis was created" and "genesis was found and skipped" to understand the state of a deployment without inspecting files directly.

**Independent Test**: Can be tested by running `apply` on an existing spoke and asserting that the structured log output contains a `genesis_skipped` event with the expected fields.

**Acceptance Scenarios**:

1. **Given** a spoke with an existing genesis, **When** `apply` runs and skips generation, **Then** the log output contains a structured entry with at minimum: event type `genesis_skipped`, spoke ID, genesis file path, and ISO-8601 timestamp.
2. **Given** a spoke with no existing genesis, **When** `apply` runs and generates genesis, **Then** the log output contains a `genesis_created` event (not `genesis_skipped`).

---

### Edge Cases

- What happens when the genesis file exists but is empty or corrupt (zero bytes / invalid JSON)? The toolkit must treat this as "not a valid genesis" and either fail with a clear error or — if a `--force-reinit` flag is present — overwrite after explicit confirmation.
- What happens when the filesystem permissions prevent the toolkit from reading the genesis path? The toolkit must fail fast with a permission error, not silently skip or proceed.
- What happens when `tmpFiles/` exists but `genesis/genesis.json` does not (partial previous run)? The toolkit must clean up `tmpFiles/` before regenerating genesis to avoid stale key contamination.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The toolkit orchestration engine MUST check for the existence of `genesis/genesis.json` (or the configured genesis output path) as the very first step of any spoke initialization sequence, before any other action is taken.
- **FR-002**: If the genesis file exists and is valid, the toolkit MUST skip the entire genesis-generation step and proceed to subsequent idempotent steps (e.g., node registration, Paladin config rendering).
- **FR-003**: If any code path in the toolkit would invoke `besu operator generate-blockchain-config` or an equivalent genesis-generation command on a spoke where genesis already exists, the toolkit MUST abort with a non-zero exit status and a descriptive error message.
- **FR-004**: The toolkit MUST emit a structured log entry (JSON, to stdout) whenever genesis generation is skipped, including spoke ID, detected genesis path, and timestamp.
- **FR-005**: The guard MUST emit a `genesis_proceed` event when it decides generation is permitted (genesis absent, or `--force-reinit` set). Because the guard is read-only and cannot observe the outcome of generation, the engine entry point that actually runs the generation command MUST emit the `genesis_created` event after it completes successfully.
- **FR-006**: A `--force-reinit` (or equivalent) flag MUST be the sole mechanism to allow genesis regeneration on an existing spoke; its use MUST require explicit operator confirmation before proceeding.
- **FR-007**: The genesis check MUST be implemented as a reusable guard that can be invoked from any entry point of the toolkit engine (apply, init, converge) without duplicating the check logic.
- **FR-008**: If the genesis file exists but is empty or contains invalid content, the toolkit MUST report the path and the detected problem, and refuse to proceed without the explicit `--force-reinit` flag.
- **FR-009**: The guard logic MUST NOT be present in or depend on the existing `deploy/local/spoke-besu-*/startBesu.sh` scripts. The sample scripts are a reference network and must remain untouched.

### Key Entities

- **GenesisGuard**: A decision component that determines whether genesis generation is permitted. Inputs: spoke workspace path, existence/validity of genesis file, presence of `--force-reinit` flag. Outputs: `proceed`, `skip`, or `abort` decision with a reason code.
- **Spoke Workspace**: The directory tree representing one spoke's on-disk state, including `genesis/`, `nodes/`, and `tmpFiles/`. The guard treats the presence of a valid `genesis/genesis.json` as the canonical "genesis complete" signal.
- **Structured Log Event**: A JSON object emitted to stdout containing at minimum: `event` (`genesis_skipped` | `genesis_created` | `genesis_error`), `spoke_id`, `genesis_path`, `timestamp` (ISO-8601).

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Running the toolkit `apply` command twice on the same initialized spoke completes the second run without modifying any genesis or node-key file — verified by file checksum comparison before and after.
- **SC-002**: Every toolkit execution that reaches the genesis-generation decision point produces exactly one structured guard log event (`genesis_skipped`, `genesis_proceed`, or `genesis_error`) within the first 5 seconds of startup.
- **SC-003**: Attempting to regenerate genesis without the `--force-reinit` flag results in a non-zero exit code 100% of the time across all tested entry points (apply, init, converge).
- **SC-004**: The existing sample network (`deploy/local/spoke-besu-a/startBesu.sh`) passes its own green-state check unchanged after this feature is delivered — confirmed by running `make scenario-b.test-contracts` or equivalent without modification.
- **SC-005**: An operator who receives the refusal error can, without consulting documentation beyond the error message itself, understand what path was detected and what flag to use for an intentional reinitialzation.

---

## Assumptions

- The provisioning toolkit is being built as a new, standalone component within Scenario A; it does not modify the existing `deploy/local/` sample scripts (per the design principle in task-notions.md §4).
- The canonical "genesis complete" signal is the presence of a non-empty, parseable `genesis/genesis.json` at the spoke workspace root; this is consistent with how `startBesu.sh` uses that file path today.
- The toolkit's orchestration engine is implemented in Go (consistent with the project stack), so the `GenesisGuard` will be a Go function or package.
- The `--force-reinit` flag is intended for development/test workflows only; it is not expected to be used in staging or production.
- Structured log output follows the project convention: JSON to stdout with fields `event`, `spoke_id`, `timestamp`, `severity` (INFO/ERROR), and `service` = `provisioning-toolkit`.
- TK-4 (manifest schema + compose template) and TK-5 (orchestration engine) are the implementation targets for this guard; no other toolkit layers are in scope for this fix.
