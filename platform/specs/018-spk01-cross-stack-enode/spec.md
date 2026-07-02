# Feature Specification: SP01 — Cross-Stack Enode Addressing

**Feature Branch**: `018-spk01-cross-stack-enode`  
**Created**: 2026-06-26  
**Status**: Draft  
**Type**: Spike — Research + POC (no production code)  
**Input**: User description: "crie uma spec para esse spike"

---

## Context

Today every node in Scenario A shares a single Docker network (`cbweb3_network`). The existing startup script resolves the Besu P2P address by calling `docker inspect` to obtain the container bridge IP (`172.x.x.x`) and rewrites the enode with it. This address is ephemeral (changes on restart) and only routable within the same host.

In the target model, each central bank runs its own independent infrastructure stack. A commercial bank in Colombia must be able to join the Colombian spoke network even though it runs on a completely separate server from the Colombian Central Bank — and neither has any connection to Brazil's infrastructure except through the settlement relay.

This spike answers the question: **how do two independent Docker Compose stacks establish Besu P2P peering using a stable, routable enode address?** The answer becomes the architectural foundation for the provisioning toolkit (Phases 1–4 of the deployment-scalability roadmap).

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Operator founds a spoke with a stable P2P address (Priority: P1)

A central bank operator starts a new spoke network. Every Besu node in the stack announces a P2P address that is stable across restarts, free of container-internal IPs, and reachable from outside the stack's own Docker network.

**Why this priority**: This is the prerequisite for every other scenario. If the enode is not stable and routable, the join bundle cannot work, and the relay cannot be configured reliably. All downstream toolkit components (manifest engine, join bundle emitter) depend on this design decision.

**Independent Test**: The operator runs the "found" stack and the verification script confirms that the announced enode contains no `172.x` or `127.0.0.1` addresses. Block production is confirmed across all nodes. This scenario has value on its own because it proves the Besu configuration pattern the toolkit must use.

**Acceptance Scenarios**:

1. **Given** the found stack is not yet running, **When** the operator starts it for the first time, **Then** all nodes are producing blocks and each node's announced enode contains a host-level address (not a Docker bridge IP)
2. **Given** the found stack is already running, **When** the operator restarts it, **Then** the enode address is identical to what it was before the restart
3. **Given** the found stack is running, **When** the automated enode verification script is executed, **Then** the script exits successfully and prints the confirmed stable enode address

---

### User Story 2 — Operator publishes a join bundle with no secrets (Priority: P1)

After founding a spoke, the central bank operator generates a join bundle artifact. This artifact is the public "invitation" that any new participant needs to join the network. It is safe to commit to a repository and contains no private keys.

**Why this priority**: The join bundle is the contract between the founding spoke and future participants. Its format and contents must be validated before any joining flow can be designed.

**Independent Test**: The operator runs the bundle extraction script. The resulting YAML file is inspected: it contains the bootnode enode, genesis hash, RPC/WS endpoints, and the spoke CA certificate. An automated check confirms no private keys or internal Docker IPs are present.

**Acceptance Scenarios**:

1. **Given** the found stack is running, **When** the bundle extraction script is executed, **Then** a `bundles/<spoke-id>.bundle.yaml` file is produced containing: `bootnodeEnode`, `rpcEndpoint`, `wsEndpoint`, `genesisHash`, and `trustAnchor`
2. **Given** a produced join bundle, **When** it is scanned for secrets, **Then** no private key material and no `172.x` / `127.0.0.1` addresses are found
3. **Given** a produced join bundle, **When** it is added to git staging, **Then** the commit succeeds with no secrets-detection warning

---

### User Story 3 — Participant joins an existing spoke from a separate stack (Priority: P1)

A commercial bank operator, running their own completely separate Docker Compose stack, joins an existing spoke by consuming only the public join bundle. The two stacks share no Docker network. After joining, the new node synchronizes to the same block height as the founder's nodes.

**Why this priority**: This is the core technical unknown of the spike. If this scenario fails, the entire `mode: join` design in the provisioning toolkit must be reconsidered before any implementation begins.

**Independent Test**: The join stack is started in a Docker network with no connection to the found stack's network. The verification script confirms that the joiner has at least one peer and its block height converges with the found stack's block height. This proves cross-stack peering works without a shared Docker network.

**Acceptance Scenarios**:

1. **Given** the found stack is running and a join bundle has been produced, **When** the join stack is started consuming that bundle, **Then** the joining node connects to the bootnode and is added to `admin_peers` of the found stack nodes
2. **Given** the joiner has connected, **When** the block height is checked on both stacks, **Then** the joiner's block height converges with the founder's block height within a reasonable wait period
3. **Given** the join stack is started, **When** the Docker network configuration is inspected, **Then** the join stack's containers are not attached to any network shared with the found stack

---

### User Story 4 — Relay reaches both spokes via explicit configuration (Priority: P2)

The settlement relay operator configures the relay to monitor two independent spokes using only explicitly declared RPC endpoints — without joining either spoke's Docker network.

**Why this priority**: The relay is operated centrally by LNET; it will never share a Docker network with individual bank stacks. Confirming that the relay can reach both spokes via public endpoints validates the relay integration model for the toolkit.

**Independent Test**: A smoke-test relay container is started in its own isolated network. It calls `eth_blockNumber` on both spoke RPC endpoints. Both calls succeed, confirming that the relay model works without shared networks.

**Acceptance Scenarios**:

1. **Given** both the found and join stacks are running, **When** the relay smoke-test container is started with explicit RPC URLs in environment variables, **Then** it successfully retrieves the latest block number from both spokes
2. **Given** the relay smoke test is running, **When** its Docker network configuration is inspected, **Then** it is not attached to any network belonging to either spoke stack

---

### User Story 5 — Genesis is generated exactly once (Priority: P1)

The genesis file for a spoke is created on first run and never overwritten by subsequent invocations of the startup tooling.

**Why this priority**: The existing sample network regenerates genesis on every run, which destroys chain state. The provisioning toolkit must enforce the opposite behavior from day one. This story validates the "genesis-once" pattern that all toolkit templates will follow.

**Independent Test**: The genesis generation script is executed twice. After the second execution, the genesis file is byte-for-byte identical to what was produced by the first execution and the script explicitly signals that generation was skipped.

**Acceptance Scenarios**:

1. **Given** no genesis file exists, **When** the genesis script is executed, **Then** a genesis file is created and the script exits successfully
2. **Given** a genesis file already exists, **When** the genesis script is executed again, **Then** the script exits without modifying the file and prints a clear "skip" message
3. **Given** the genesis file was produced by the first run, **When** a checksum is compared before and after the second run, **Then** the checksums are identical

---

### Edge Cases

- What happens when the host port for P2P is already bound by another process? The stack startup should fail with a clear error message, not silently bind to a different port.
- What happens when the join bundle references an enode that is currently offline? The joiner should wait for connectivity and log the retry state — it must not silently proceed with zero peers.
- What happens on Docker Desktop for Mac vs. Linux? The `DOCKER` NAT behavior differs; the spike must document the exact behavior difference and whether `extra_hosts: host-gateway` is required on Linux.
- What happens if the enode audit script is run against a node started without the correct NAT flags? The script must fail with a clear message identifying the offending IP.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The enode announced by a Besu bootnode MUST contain a host-level address that remains stable across container restarts and is reachable from outside the stack's own Docker network
- **FR-002**: The genesis file for a spoke MUST be created exactly once; the startup tooling MUST refuse to overwrite an existing genesis file
- **FR-003**: The join bundle MUST contain: stable bootnode enode, genesis hash, RPC endpoint, WS endpoint, spoke CA certificate, and any deployed contract addresses needed for onboarding
- **FR-004**: The join bundle MUST NOT contain any private key material, password, or container-internal IP addresses (`172.x`, `127.0.0.1`)
- **FR-005**: A participant running in an independent Docker Compose stack MUST be able to peer with the founding spoke using only the join bundle — no shared Docker network required
- **FR-006**: The relay integration MUST be configurable via explicit RPC URL environment variables; the relay MUST NOT require attachment to any spoke's Docker network
- **FR-007**: An automated verification suite MUST confirm: enode stability (T3), block production (T2), cross-stack peering (T4), relay RPC reachability (T5), and genesis idempotency (T1)
- **FR-008**: The spike artifacts MUST NOT modify any file under `scenario-a/deploy/local/` or `scenario-a/make/`

### Key Entities

- **Spoke**: A blockchain network instance operated by a central bank, identified by a spoke ID (e.g., `spoke-brl`, `spoke-cop`), with its own chain ID, genesis, and set of validator nodes
- **Bootnode**: The first Besu node in a spoke, operated by the founding central bank, whose announced P2P address is the sole entry point in the join bundle
- **Join Bundle**: A public, committable YAML artifact emitted by the founding central bank after a spoke is initialized; contains everything a new participant needs to connect — no secrets
- **Enode**: A Besu P2P peer address in the format `enode://<public-key>@<host>:<port>`; must use a stable host-level address, never a Docker bridge IP
- **Verification Suite**: A set of scripts that check specific post-conditions (enode validity, peering, block convergence, relay reach, genesis idempotency) and exit non-zero on failure

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All T1–T6 verification checks pass with exit code 0 in a single `make verify` invocation on both Linux and Docker Desktop (Mac)
- **SC-002**: A new participant running in an isolated Docker Compose stack achieves block-height convergence with the founding stack within 60 seconds of startup
- **SC-003**: Zero `172.x` or `127.0.0.1` addresses appear in any produced join bundle — confirmed by automated scan
- **SC-004**: The genesis file checksum is identical before and after a second invocation of the genesis script
- **SC-005**: The relay smoke-test container reaches both spoke RPC endpoints from a network with zero shared containers — confirmed by automated check
- **SC-006**: An engineer who has never run the sample network can execute the spike end-to-end following only the spike README, with no manual steps beyond running `make spk01.up && make spk01.verify`
- **SC-007**: The ADR is accepted by the team with the design decision recorded, rejected alternatives documented, and a clear mapping from Besu NAT flags to each deployment profile (local intra-compose, local cross-compose, staging, prod)

---

## Assumptions

- The spike targets Besu version 25.8.0 only (pinned per project constitution; no `latest` tag)
- Both local same-host cross-compose and separate-host prod addressing are in scope for the ADR, but the POC only runs same-host (separate-host requires physical machines not available in the dev environment)
- QBFT validator voting for dynamically adding a new validator is out of scope — the joining node in the POC participates as a non-validator or uses a genesis with all validators pre-declared; validator voting is covered by SP02
- Paladin, deployed contracts, and settlement E2E are out of scope for this spike; the POC exercises only Besu P2P peering and relay RPC connectivity
- The existing `scenario-a/deploy/local/` sample network is the reference network and must remain untouched and fully functional throughout the spike
- `host.docker.internal` resolving on Linux requires `extra_hosts: host-gateway` in the Compose file; the spike documents this as a local-profile constraint, not a production model
- The spike is scoped to Scenario A only; Scenario B infrastructure is not affected
