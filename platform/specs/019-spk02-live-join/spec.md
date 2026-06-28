# Feature Specification: SP02 — Live Spoke Join (QBFT + Paladin)

**Feature Branch**: `019-spk02-live-join`
**Created**: 2026-06-26
**Status**: Draft
**Type**: Spike — Research + POC (no production code)
**Input**: SP-2 — Investigar: join de spoke ao vivo (QBFT + Paladin)

---

## Context

SP01 proved that two independent Docker Compose stacks can establish Besu P2P peering using stable
enode addresses. In that spike the joining node was a non-validator: it synced to the chain but
did not participate in QBFT block production, and no Paladin node was started for it.

The provisioning toolkit's `mode: join` must eventually turn a newly connected node into a
**QBFT validator** (so it can propose and vote on blocks) and start a **Paladin node** for it
(so it can issue and receive privacy tokens). Both transitions must happen to a spoke that is
**already running** — block production must not be interrupted, and existing participants must
not need to coordinate a maintenance window.

This spike answers two independent questions that together define the design of `mode: join`:

1. **QBFT promotion**: What is the exact sequence of validator votes required to promote a
   synced non-validator node into a QBFT validator on a live network?
2. **Paladin live join**: Does adding a new Paladin node to a running spoke require a restart
   of existing Paladin nodes? The answer depends on whether the on-chain transport registry
   is discovered reactively (no restart) or whether the TLS trust model forces a config
   reload (restart required for existing nodes).

The answers produce a validated step sequence that becomes the design contract for the
`mode: join` path in the provisioning toolkit. If a restart is unavoidable, its scope,
duration, and operator UX must be documented explicitly.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Operator promotes a synced node to QBFT validator without interrupting block production (Priority: P1)

A central bank operator adds a new commercial bank node to its spoke network. The node is
already synced and peered (established in SP01's `mode: join`). The operator wants to promote
it to a validator so it can participate in block production. Existing validators must not
restart, and block production must continue without a gap exceeding the normal QBFT block time.

**Why this priority**: Validator promotion is the critical path for the `mode: join` toolkit
action. If validator voting causes a liveness disruption, `mode: join` becomes a maintenance-
window operation — a showstopper for multi-bank onboarding.

**Independent Test**: A 3-validator spoke is running and producing blocks. A 4th node joins as
a non-validator and syncs. The test calls `qbft_proposeValidatorVote` on all 3 existing
validators and waits for block production to continue. A verification script confirms that the
4th node appears in `qbft_getValidatorsByBlockNumber("latest")` and that block height increased
monotonically (no reorgs or pauses) throughout the voting window.

**Acceptance Scenarios**:

1. **Given** a 3-validator spoke is running with a 4th non-validator node synced, **When** the
   operator calls `qbft_proposeValidatorVote(newAddress, true)` on each existing validator,
   **Then** the voting succeeds without error and blocks continue to be produced throughout
   the voting period
2. **Given** all 3 votes have been cast, **When** the next QBFT epoch boundary is reached,
   **Then** the new validator's address appears in `qbft_getValidatorsByBlockNumber("latest")`
3. **Given** the new validator is active, **When** the block log is inspected, **Then** no
   gap larger than 2× the configured block time appears in the sequence of block timestamps
4. **Given** the validator set has changed, **When** the existing validators are checked,
   **Then** no existing validator container was restarted during the process

---

### User Story 2 — New Paladin node discovers existing peers via on-chain registry without requiring existing nodes to restart (Priority: P1)

A commercial bank's Paladin node starts for the first time and registers itself on the spoke's
on-chain transport registry. The existing Paladin nodes (central bank + other commercial banks)
must discover the new node and establish mTLS transport with it — without any of the existing
Paladin containers being restarted.

**Why this priority**: If existing Paladin nodes must restart to accept a new peer, `mode: join`
imposes downtime on the entire spoke every time a bank onboards. This is operationally
unacceptable in a production CBDC network.

**Independent Test**: A spoke is running with central bank + bank-A Paladin nodes active. A
new bank-X Paladin node starts, generates its TLS certificate, and registers on-chain. A
verification script checks that the existing nodes' containers have not been restarted (by
comparing start times before and after) and that a cross-node `ptx` call succeeds between the
new bank-X and the existing central bank node.

**Acceptance Scenarios**:

1. **Given** central-bank and bank-A Paladin nodes are running, **When** bank-X Paladin starts
   and registers on-chain, **Then** the existing nodes' containers remain running with the same
   start time (no restart occurred)
2. **Given** bank-X has registered on-chain, **When** a privacy-token operation is attempted
   between bank-X and central-bank, **Then** the mTLS handshake succeeds and the operation
   completes
3. **Given** the Paladin transport logs are inspected, **Then** the peer discovery event for
   bank-X appears in both central-bank and bank-A logs, confirming reactive (event-driven)
   discovery

---

### User Story 3 — If restart is unavoidable, operator receives a clear UX contract for mode: join (Priority: P1)

If the investigation concludes that existing Paladin nodes **must** restart after a new peer
registers (due to TLS trust-store loading at startup), the spike must produce a concrete
operator sequence that:
- Identifies exactly which containers restart (Paladin only, not Besu)
- Bounds the restart window (rolling vs. simultaneous)
- Documents the observable impact (which operations are unavailable during restart)

**Why this priority**: "Restart required" is a valid outcome — but only if its UX cost is fully
specified. The toolkit cannot hide a restart from the operator; it must present it as an
explicit, timed step.

**Acceptance Scenarios** (applicable only if restart is required):

1. **Given** restart is required, **When** the documentation is reviewed, **Then** it explicitly
   states: which containers restart, estimated restart duration, and which capabilities are
   unavailable during restart
2. **Given** restart is required, **When** a rolling restart is possible (one node at a time),
   **Then** the spoke remains available (other validators continue producing blocks) during
   the entire rolling window
3. **Given** restart is required, **When** a restart script is executed by the operator,
   **Then** the script produces structured output logging start time, end time, and whether
   each container reached healthy state

---

### User Story 4 — Spike results are captured in ADR-002 with rejected alternatives (Priority: P1)

The spike produces an Architecture Decision Record that records both questions' outcomes,
the tested evidence, rejected alternatives, and the exact step sequence the toolkit will
implement for `mode: join`. Engineers who read the ADR must be able to reproduce the
behavior without running the spike again.

**Why this priority**: The ADR is the deliverable. The POC is evidence. Without the ADR the
research has no durable artifact and the toolkit cannot be implemented correctly.

**Acceptance Scenarios**:

1. **Given** the spike POC has been run, **When** ADR-002 is reviewed, **Then** it contains
   the complete test evidence table (test ID, description, pass/fail, observed behavior) for
   all T1–T6 checks
2. **Given** the ADR describes the QBFT voting sequence, **When** an engineer follows the
   sequence using only the ADR, **Then** a new validator is successfully promoted on a local
   spoke with no additional reference material
3. **Given** the ADR describes the Paladin join behavior, **When** the toolkit engineer reads
   the restart verdict (restart required / not required), **Then** the verdict includes the
   evidence that led to the conclusion (log excerpts, container start-time comparison)

---

### Edge Cases

- What happens if an existing validator is temporarily offline when votes are being cast?
  The voting must still succeed using the remaining online validators (provided majority is
  maintained). Document the minimum quorum for voting.
- What happens if the new Paladin node's TLS certificate is revoked or expired? The existing
  Paladin nodes should reject mTLS and log a clear error, not fail silently.
- What happens if the new Paladin node's `nodeName` collides with an existing one in the
  registry? The registry call should fail with a clear error at registration time, not
  silently overwrite.
- What happens during the epoch boundary when a newly voted validator is being activated?
  Block production must not stall. Confirm that QBFT handles the validator-set change
  atomically at epoch boundaries.
- What happens on Docker Desktop for Mac vs. Linux for the TLS self-signed cert path?
  Document any OS-level difference in cert loading behavior that affects the restart verdict.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A non-validator Besu node that is already peered and synced MUST be promotable
  to QBFT validator status by having existing validators call `qbft_proposeValidatorVote`
  without restarting any existing validator
- **FR-002**: After the vote threshold (floor(N/2)+1 of current validators) is reached, the
  new validator's address MUST appear in `qbft_getValidatorsByBlockNumber("latest")` within
  the next QBFT epoch
- **FR-003**: Block height MUST increase monotonically throughout the QBFT voting window — no
  block timestamp gap exceeding 2× the configured block time is permitted
- **FR-004**: The new Paladin node MUST start, generate a TLS certificate, and register itself
  in the on-chain transport registry without modifying the configuration of existing Paladin nodes
- **FR-005**: The spike MUST determine and document whether existing Paladin nodes discover the
  new peer reactively (via on-chain event subscription) or require a config reload and restart
- **FR-006**: If a restart is required for existing Paladin nodes, the restart MUST be bounded
  to the Paladin layer only — Besu validators MUST NOT restart
- **FR-007**: An automated verification suite MUST confirm all post-conditions: validator
  promotion (T1–T2), block continuity (T3), no existing container restart (T4), mTLS
  cross-node operation (T5), and ADR evidence table completeness (T6)
- **FR-008**: The spike artifacts MUST NOT modify any file under `scenario-a/deploy/local/`
  or `scenario-a/make/`

### Key Entities

- **Spoke**: A QBFT blockchain network instance operated by a central bank with N validators
  and M associated Paladin nodes; the subject of the live-join operation
- **Non-validator Node**: A Besu node that is peered and synced but not included in the active
  QBFT validator set; the starting state for the joining node in this spike
- **Validator Vote**: A call to `qbft_proposeValidatorVote(address, true)` from an existing
  validator node; floor(N/2)+1 votes promote a candidate to active validator status
- **QBFT Epoch**: The fixed-block-count interval at which QBFT processes accumulated validator
  votes; the epoch boundary is when the new validator becomes effective
- **Paladin Node**: A privacy middleware instance running alongside a Besu node, identified by
  a `nodeName`; registers its gRPC transport endpoint + TLS certificate in the on-chain
  transport registry
- **Transport Registry**: The on-chain registration mechanism where each Paladin node publishes
  its `nodeName`, gRPC endpoint, and TLS certificate; used by other Paladin nodes for peer
  discovery. Hypothesis to validate: this transport registry may or may not be the
  `IdentityRegistry` contract (`PARTICIPANT_REGISTRY_ADDRESS`), which is the approved-participant
  whitelist for blockchain identity — a distinct concern from Paladin node/transport
  registration. The spike must determine the actual Paladin node/transport registration mechanism.
- **TLS Trust Model**: The mechanism by which Paladin nodes verify each other's TLS certificates
  during mTLS transport establishment; the critical unknown of this spike (pre-loaded CA vs.
  per-peer cert vs. TOFU vs. registry-fetched)
- **ADR-002**: The Architecture Decision Record that captures the spike's findings, the
  validated step sequence, and the UX contract for `mode: join`

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All T1–T6 verification checks pass with exit code 0 in a single
  `make spk02.verify` invocation on both Linux and Docker Desktop (Mac)
- **SC-002**: A new node is promoted from non-validator to active QBFT validator within
  one QBFT epoch of the last vote being cast, with block production uninterrupted
- **SC-003**: No existing Paladin or Besu container is restarted during the join process —
  confirmed by comparing container start times before and after the join sequence
- **SC-004** *(if no restart required)*: A cross-node privacy-token operation between the
  new Paladin node and an existing one completes successfully within 30 seconds of registration
- **SC-005** *(if restart required)*: The restart script completes within 60 seconds per node
  and produces structured logs confirming each node returned to healthy state
- **SC-006**: ADR-002 is accepted by the team with all evidence fields populated and the
  step sequence reproducible by an engineer who has not run the spike
- **SC-007**: An engineer who has never run the spike can execute it end-to-end following
  only the spike README, with no manual steps beyond `make spk02.up && make spk02.verify`

---

## Assumptions

- The spike targets Besu version 25.8.0 only (pinned per project constitution)
- The spike builds on SP01's `stack-found.yml` (3-validator network with stable enode
  addresses); the SP01 POC must be in a passing state before SP02 runs
- QBFT uses block-header voting (not smart-contract-based validator selection); the voting
  API is `qbft_proposeValidatorVote` / `qbft_getValidatorsByBlockNumber`
- Paladin nodes use self-signed TLS certificates (as established in SP01's
  `generate-certs.sh`) — there is no shared CA; the TLS trust behavior is the core unknown
- No new contracts are deployed for this spike. Hypothesis to validate: the Paladin transport
  registry may or may not be the `IdentityRegistry` contract already deployed in the existing
  spoke setup; `IdentityRegistry` (`PARTICIPANT_REGISTRY_ADDRESS`) is the approved-participant
  whitelist for blockchain identity, which is distinct from a Paladin node/transport registry.
  The spike must determine the actual Paladin node/transport registration mechanism.
- The spike is scoped to Scenario A only; Scenario B infrastructure is not affected
- The spike artifacts land exclusively in `scenario-a/provisioning/spikes/spk-02-live-join/`;
  no existing files are modified
- "Live" means the founding stack (Besu + Paladin) is running and producing blocks throughout
  the entire join sequence; there is no planned maintenance window
- The Paladin version used is whatever is currently pinned in the project's Paladin Docker
  Compose files; no version changes are introduced by this spike
