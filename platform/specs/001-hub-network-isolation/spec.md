# Feature Specification: International Hub Network Isolation

**Feature Branch**: `001-hub-network-isolation`
**Created**: 2026-06-08
**Status**: Draft
**Input**: User description: "Provision an independent Besu network for the International Hub at chain 1337. Currently all Scenario B services resolve HUB_CHAIN_ID to 1338, which points at Spoke A's own chain, so the AMM runs on a domestic ledger instead of the neutral settlement layer described in D2/D7. The Hub must be topologically separate from Spoke A and Spoke B, with services connecting exclusively to it and AMM contracts deployed there, plus a deployment runbook and removal of all hardcoded chain ID references."

## Clarifications

### Session 2026-06-08

- Q: Should this feature design and implement a migration/cutover procedure that carries forward existing on-chain state (deployed contract data, balances, liquidity positions, HTLC records) from Spoke A's current "Hub" deployment to the new independent Hub, or is it scoped as a clean rebuild where that state is decommissioned and the environment is recreated fresh? → A: Clean rebuild — no migration of on-chain state. The existing "Hub" deployment hosted on Spoke A is decommissioned/abandoned; the environment is recreated from genesis on the new Hub network, and the runbook documents this explicitly as a fresh start rather than a migration.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Operator stands up an independent Hub network (Priority: P1)

An infrastructure operator (LNet or a project engineer) needs to bring up the International Hub as its own network — separate from any participating jurisdiction's domestic ledger — so that the shared settlement layer is demonstrably neutral rather than hosted inside one spoke's infrastructure.

**Why this priority**: Without an independently running Hub network, no other Scenario B claim about a "neutral settlement layer" can be true or tested. This is the foundational capability that everything else in this spec depends on.

**Independent Test**: Start only the Hub network stack (no Spoke A, no Spoke B, no backend services running). Confirm a validator node is producing blocks and an RPC endpoint answers with chain identity `1337`. This delivers value on its own: it proves the Hub exists as a standalone settlement layer, which is the core architectural claim being validated.

**Acceptance Scenarios**:

1. **Given** no other network stack is running, **When** the operator starts the Hub network stack, **Then** a validator node starts, begins producing blocks under the agreed consensus mechanism, and an RPC endpoint responds with chain identity `1337`.
2. **Given** the Hub network is running, **When** the operator stops Spoke A and Spoke B (or never starts them), **Then** the Hub continues to run and produce blocks without interruption or error.
3. **Given** the Hub network is running, **When** the operator inspects its network identity, **Then** it is confirmed to be a network distinct from Spoke A (chain `1338`) and Spoke B (chain `1339`) — sharing no nodes, validators, or genesis state with either.

---

### User Story 2 - Engineer deploys shared contracts to the neutral Hub (Priority: P1)

A platform engineer needs to deploy the AMM, Liquidity Pool, and shared governance contracts onto the newly independent Hub network, so the shared settlement logic actually executes on the neutral layer rather than inside Spoke A's domestic chain.

**Why this priority**: Standing up the network is necessary but not sufficient — the entire point of the Hub is to host shared contracts neutrally. Until contracts are deployed and addressable on chain `1337`, the Hub is an empty shell and Scenario B cannot run end to end.

**Independent Test**: With the Hub network running (User Story 1 satisfied), run the contract deployment procedure against it and confirm each shared contract returns a valid, queryable on-chain address on chain `1337`. This can be verified independently of whether any spoke or backend service is configured yet.

**Acceptance Scenarios**:

1. **Given** the Hub network is running and reachable, **When** the engineer runs the deployment procedure for the shared contract suite (AMM, Liquidity Pool / pair and currency registries, identity registry, FX agreement, shared tokens), **Then** each contract deploys successfully and returns a distinct, valid contract address on chain `1337`.
2. **Given** the contracts are deployed to the Hub, **When** the engineer queries the AMM contract address on chain `1337`, **Then** the contract responds and its on-chain code matches the deployed artifact (i.e., it is live and operable, not merely an address record).
3. **Given** a fresh deployment with no bridge activity yet, **When** the engineer inspects Hub-minted token balances, **Then** the total reflects zero value locked until the first cross-chain lock event has been processed — establishing the baseline reconciliation point documented in the runbook.

---

### User Story 3 - Engineer reconfigures all services to point exclusively at the Hub (Priority: P1)

A platform engineer needs every Scenario B service (API gateway, payment orchestrator, and all four entity backend stacks) to resolve the Hub's identity from configuration alone — with chain `1337` as the default — and never from a baked-in reference to Spoke A's chain.

**Why this priority**: This is the change that actually closes the defect described in the input: today, every service silently resolves to `1338` (Spoke A) regardless of what the deployment intends. Without this, standing up an independent Hub network changes nothing for the running system — services would still talk to Spoke A.

**Independent Test**: Search the codebase and configuration for the old hardcoded value; confirm none remain as defaults pointing at Spoke A's chain, and confirm every service correctly resolves `1337` when started with standard configuration, with a documented fallback when the value is altogether absent.

**Acceptance Scenarios**:

1. **Given** a service is started with the standard environment configuration (no overrides), **When** it resolves the Hub's network identity, **Then** it resolves to chain `1337`, sourced from configuration rather than a value embedded in the program.
2. **Given** all environment templates and service-startup configuration files in the repository, **When** the engineer inspects their default values for the Hub's network identity, **Then** every one of them defaults to `1337` and none default to `1338`.
3. **Given** a service is started with the Hub's network identity entirely absent from its environment, **When** it starts up, **Then** it does not crash — it falls back to the documented default (`1337`), emits a clearly visible warning that the value was not explicitly configured, and continues operating. No binary may silently assume `1338`.
4. **Given** the full Scenario B stack is configured per the updated defaults, **When** an end-to-end flow that depends on the Hub (e.g., a cross-currency swap through the AMM) is exercised, **Then** the relevant on-chain activity is observed on chain `1337`, not `1338`.

---

### User Story 4 - Operator follows a runbook to reproduce the full environment (Priority: P2)

An operator at a partner organization (LNet) who was not involved in building the Hub needs to bring up the entire environment — Hub, contracts, both spokes, and the cross-chain relay — in the correct order, using only the written runbook, without assistance from the team that built it.

**Why this priority**: The Hub is only useful to the broader Scenario B effort if partners can reproduce it independently. This is what converts "it works on the builder's machine" into a shared, validated reference environment — and it's the basis for the partner sign-off this feature depends on (see Assumptions).

**Independent Test**: Hand the runbook to someone unfamiliar with the recent changes and have them bring up the full stack from a clean environment using only the document. Success is a fully running environment with no undocumented steps or tribal knowledge required.

**Acceptance Scenarios**:

1. **Given** a clean environment and only the runbook as a guide, **When** the operator follows it step by step, **Then** they can bring up the Hub, deploy contracts to it, bring up both spokes, and configure the cross-chain relay — in that order — ending with a fully operational environment.
2. **Given** the runbook, **When** the operator reaches the section on validator configuration, **Then** they find an explicit statement that the sandbox environment uses a minimal single-validator setup, along with a clear description of what a multi-validator production configuration would additionally require.
3. **Given** the runbook, **When** the operator reaches the section on reconciliation, **Then** they find a clear explanation of the "zero until first lock" baseline for Hub-minted value, and how to confirm it holds immediately after a fresh startup.

---

### User Story 5 - Maintainer confirms no regressions to the existing system (Priority: P3)

A maintainer needs assurance that introducing the independent Hub network and reconfiguring services has not broken Scenario A or any previously passing automated checks.

**Why this priority**: This is a safety net rather than new capability — it confirms the change is additive/corrective and doesn't destabilize parts of the system that already worked.

**Independent Test**: Run the full automated check suite on the branch and confirm Scenario A behaves exactly as it did before this change, with no new failures attributable to this work.

**Acceptance Scenarios**:

1. **Given** the changes in this feature are applied, **When** the full automated check suite runs, **Then** it passes with no new failures, and Scenario A's behavior is unchanged.

---

### Edge Cases

- What happens if an operator starts Spoke A or Spoke B *before* the Hub is up? The Hub must not be a hidden dependency of either spoke at startup — spokes should start (or fail) independently of Hub availability, and only Hub-dependent operations (e.g., cross-chain swaps) should be affected if the Hub isn't reachable yet.
- What happens if a service is configured with a Hub network identity that doesn't match the network it can actually reach (misconfiguration, e.g., pointed at Spoke A by mistake)? The system should surface this clearly (e.g., a mismatch between expected and actual chain identity) rather than silently operating against the wrong ledger.
- What happens to existing deployments or environments that still have the old default (`1338`) cached in their local configuration? The runbook must explain how to detect and correct a stale configuration value (distinct from the on-chain cutover, which is a clean rebuild — see below).
- What happens to the shared contracts and any on-chain state currently hosted on Spoke A's chain as a byproduct of the prior architecture (the "Hub-on-Spoke-A" setup)? Per the clean-rebuild decision (see Clarifications), that deployment is decommissioned outright — no state is carried forward. The runbook must say so explicitly, list the old addresses as retired/non-authoritative, and instruct operators to treat the new Hub deployment as the sole source of truth going forward.
- How is the "zero TVL until first lock" invariant verified if the Hub is restarted after having processed activity (i.e., not a fresh deployment)? The reconciliation guidance must distinguish "fresh start" from "restart with existing state."

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a way to start an International Hub network that runs independently — producing blocks and serving RPC requests — without requiring Spoke A or Spoke B to be running.
- **FR-002**: The Hub network MUST identify itself with network/chain identity `1337`, distinct from Spoke A (`1338`) and Spoke B (`1339`).
- **FR-003**: The Hub network MUST use the same consensus mechanism, block-time, and gas-limit characteristics already established for Spoke A and Spoke B, so the three networks remain operationally consistent within the prototype.
- **FR-004**: The Hub network MUST be topologically separate from Spoke A and Spoke B — sharing no validator nodes, peer connections, or genesis state with either.
- **FR-005**: The system MUST provide a deployment procedure that places the full shared contract suite (AMM, Liquidity Pool / pair and currency registries, identity registry, FX agreement, shared tokens, and any other shared-governance contracts) onto the Hub network, with each contract returning a valid, distinct, queryable on-chain address.
- **FR-006**: Every Scenario B service that needs to reach the Hub MUST resolve the Hub's network identity and endpoint exclusively from its runtime configuration, never from a value embedded in source code or compiled into a binary.
- **FR-007**: The default value for the Hub's network identity, wherever configured (service startup configuration, environment templates, or any other default-supplying mechanism), MUST be `1337`.
- **FR-008**: No part of the system may assume the Hub's network identity is `1338` (Spoke A's identity) by default or as a fallback.
- **FR-009**: If a service starts without the Hub's network identity present in its configuration, it MUST NOT fail to start; it MUST fall back to `1337`, and MUST emit a clearly visible warning indicating the value was not explicitly provided.
- **FR-010**: The system MUST provide a written, self-contained deployment runbook covering the full startup sequence in dependency order: Hub network up → shared contracts deployed to the Hub → spoke networks up → cross-chain relay configured — sufficient for an independent operator to reproduce the environment without assistance from the team that built it.
- **FR-011**: The runbook MUST explicitly document that the sandbox/prototype validator configuration is minimal (a single validator is acceptable), and MUST separately describe what a multi-validator production-grade configuration would require.
- **FR-012**: The runbook MUST document the Total Value Locked reconciliation baseline: immediately after a fresh Hub startup, Hub-minted token value must be zero, and this remains the expected state until the first cross-chain lock event has been processed by the relay.
- **FR-013**: The change MUST NOT alter the behavior, configuration defaults, or automated-check results of Scenario A.
- **FR-014**: The system MUST provide a way to verify, after configuration changes are applied, that Scenario B's on-chain activity (e.g., AMM operations, cross-currency swaps) actually occurs on the Hub's network (`1337`) rather than on Spoke A's network (`1338`).
- **FR-015**: The cutover MUST be a clean rebuild — no on-chain state (contract data, balances, liquidity positions, HTLC records) is migrated from the existing "Hub-on-Spoke-A" deployment to the new Hub network. The runbook MUST explicitly document that the prior deployment is decommissioned/retired, list its addresses as non-authoritative, and designate the new Hub deployment as the sole source of truth going forward.

### Key Entities

- **International Hub Network**: The independent settlement-layer network (chain identity `1337`) that hosts shared, neutral infrastructure for Scenario B. Distinguished from Spoke A and Spoke B by having its own validator(s), genesis configuration, and network topology.
- **Shared Contract Suite**: The set of smart contracts that must live on the Hub because they represent neutral, multilaterally-governed functionality — including the Automated Market Maker, Liquidity Pool registries, identity registry, FX agreement, and shared tokens. Each is identified by a deployed on-chain address that must be addressable on chain `1337`.
- **Hub Network Identity (configuration value)**: The configuration value that tells a service which network to treat as "the Hub" — must default to `1337`, must be sourced from configuration (not embedded in code), and must have a documented, non-silent fallback behavior when absent.
- **Deployment Runbook**: The authoritative written procedure for bringing up the full environment in the correct dependency order, including validator-configuration caveats and the TVL reconciliation baseline — the artifact that allows an external partner to reproduce the environment independently.
- **TVL Reconciliation Baseline**: The expected state of Hub-minted token value at a given point in the system's lifecycle — specifically, zero at fresh startup, remaining zero until the first cross-chain lock event is processed by the relay. Used as a sanity check that the bridge and Hub are wired correctly.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can bring up a fully functioning International Hub network from a stopped state in under 10 minutes using only the documented procedure, with no manual intervention beyond what the runbook describes.
- **SC-002**: 100% of configuration locations that determine the Hub's network identity (service defaults, environment templates, and any other default-supplying location) default to `1337`; zero locations default to `1338`.
- **SC-003**: An operator unfamiliar with the recent changes can reproduce the full environment (Hub, contracts, both spokes, relay) end-to-end on a clean machine using only the runbook, without needing to ask the implementing team a single clarifying question.
- **SC-004**: 100% of the shared contract suite is independently verifiable as live and addressable on the Hub's network after deployment (each contract returns a valid address and responds to a basic on-chain query).
- **SC-005**: After the change, an end-to-end Scenario B flow that exercises the Hub (e.g., a cross-currency swap) shows 100% of the relevant on-chain activity occurring on chain `1337`, with zero such activity observed on chain `1338`.
- **SC-006**: Scenario A's automated checks pass at the same rate after this change as immediately before it (no regressions introduced).
- **SC-007**: Immediately following a fresh Hub deployment and before any cross-chain activity occurs, Hub-minted token value is verifiably zero — confirming the documented TVL reconciliation baseline holds in practice.

## Assumptions

- "LNet confirms alignment on genesis parameters prior to final merge" (stated in the input as a goal condition) is treated as an external coordination/sign-off step that gates the merge of this work, not as a system capability to be built. This spec defines what must be true about the Hub's parameters (FR-003, FR-011) so that such alignment can be sought; obtaining LNet's confirmation itself sits outside the system's functional scope.
- "CI passes with no regressions on Scenario A" (stated in the input as a goal condition) is captured as a quality gate (FR-013, SC-006) rather than a new capability — the existing automated-check suite is assumed to remain the mechanism for verifying this.
- The "shared contract suite" is assumed to include, at minimum, the contracts named in the input (AMM, Liquidity Pool) plus the other shared/governance contracts already known to live on what is currently treated as "the Hub" (pair/currency registries, identity registry, FX agreement, shared tokens) — moving only some of them would leave the Hub partially neutral and partially domestic, defeating the purpose.
- A single-validator configuration is assumed acceptable for the sandbox/prototype environment, per the input's explicit guidance — with the documentation requirement (FR-011) serving as the safeguard against this being mistaken for a production-ready setup.
- "Topologically separate" is assumed to mean: no shared validator nodes, no shared peer/network membership, and no shared genesis state between the Hub and either spoke — i.e., an outage or compromise of a spoke's network cannot directly affect the Hub's operation, and vice versa.
- The fallback behavior described in the input ("a sensible default of 1337 with a logged warning is acceptable... but the value must never be silently baked into a binary") is interpreted as: the default may exist in configuration templates and as a last-resort runtime fallback, but must always be visibly surfaced (via a warning) when relied upon rather than explicitly set — distinguishing "configurable with a documented default" from "hardcoded."
