# Test Execution Plan — CBWeb3 Platform

> **Deliverable 12** · CBDC System Test Execution Plan
>
> Project: RG-T4567 · Suboperation: ATN/KS-21330-RG
> Authors: Lucas Campelo, Samuel Venzi
> Date: 2026-05-29

---

## Table of Contents

- [Introduction](#introduction)
- [Scope](#scope)
- [Test Goals](#test-goals)
- [Test Execution Timeline and Responsibilities](#test-execution-timeline-and-responsibilities)
- [Testing Methodology](#testing-methodology)
- [Test Levels](#test-levels)
- [API-First E2E Execution Strategy](#api-first-e2e-execution-strategy)
- [Traceability Map](#traceability-map)
- [Test Environments and Reproducible Topology](#test-environments-and-reproducible-topology)
- [Performance Testing Strategy](#performance-testing-strategy)
- [Security and Abuse Testing Strategy](#security-and-abuse-testing-strategy)
- [Time Budgets, Finality, and Flakiness Controls](#time-budgets-finality-and-flakiness-controls)
- [Interoperability Resilience Scenarios](#interoperability-resilience-scenarios)
- [CI/CD Quality Gates and Pipeline Strategy](#cicd-quality-gates-and-pipeline-strategy)
- [Evidence Bundle Format](#evidence-bundle-format)
- [Scenario B — Planned Extensions](#scenario-b--planned-extensions)

---

## Introduction

This Test Execution Plan establishes the formal quality assurance strategy for the CBWeb3 platform — a regional prototype for wholesale CBDC settlement between jurisdictions in Latin America and the Caribbean. The plan validates functional and non-functional integrity across the three-tier testing hierarchy (unit, integration, end-to-end), covering the hub-and-spoke DLT architecture, atomic cross-spoke settlement via HTLC, the Paladin/Zeto privacy layer, and the Hyperledger Cacti relay.

The complete test case catalog is maintained in [`scenario-a/tests/TEST-CATALOG.md`](../../tests/TEST-CATALOG.md).

---

## Scope

The following components and layers are explicitly **in scope**:

- **Smart Contracts**: HTLC escrow state machine, tCeBM/fCeBM token logic, IdentityRegistry on-chain RBAC, FXAgreement lifecycle, SpokeBridge, AutomatedMarketMaker
- **Backend API Services**: api-gateway (REST), auth (gRPC), compliance (gRPC), payment-orchestrator (gRPC) — all 6 entities
- **Escrow Flow**: full deposit → fCeBM mint → escrow (fCeBM burn + tCeBM Zeto mint) → redeem (tCeBM Zeto transfer + fCeBM mint) lifecycle
- **FX Agreement Lifecycle**: PROPOSED → ACCEPTED → SETTLED / REJECTED / CANCELLED
- **HTLC Cross-Spoke Settlement**: bilateral atomic swap between Spoke-A (chain 1338) and Spoke-B (chain 1339) via Cacti relay
- **Cacti Relay**: automatic secret extraction and cross-spoke submission
- **Paladin / Zeto**: ZKP-shielded token transfers, Pente bilateral context for FX Agreement
- **Compliance Layer**: AML/CFT screening, participant onboarding (KYC), account freeze/unfreeze
- **Test Environments**: Devnet (local Docker Compose) and Testnet (staging)

The following are explicitly **out of scope**:

- Hyperledger Besu core QBFT consensus protocol (validated by upstream maintainers)
- Frontend UI pixel-level testing (covered by API-First E2E which validates backend state consistency)
- Paladin node internals beyond the API surface exposed to the platform

---

## Test Goals

| Goal | Metric |
|------|--------|
| **Logic Validation** | ≥ 80% code coverage on core smart contract business logic (HTLC state machine, tCeBM mint/burn, FXAgreement state transitions) |
| **Data Integrity** | Integration tests verify that data flows correctly across API → gRPC → Besu/Paladin; exceptions (insufficient balance, AML block, expired timeLock) handled without state inconsistency |
| **Cross-Spoke Atomicity** | E2E HTLC flow completes fully (both legs settled) or reverts fully (both legs refunded) with no orphaned escrow |
| **Compliance Enforcement** | Sanctioned/frozen accounts are blocked at the API layer before any on-chain interaction |
| **Relay Resilience** | Cacti relay recovers from downtime, sweeping missed events and completing pending cross-spoke locks |
| **Performance Baseline** | System meets pilot-grade thresholds under steady-state and burst load |

---

## Test Execution Timeline and Responsibilities

### Stakeholders

| Stakeholder | Role |
|-------------|------|
| **LNET** | Platform developer and integrator — responsible for smart contract, API, infrastructure, tooling delivery, and test execution |
| **Banks** | Participant institutions (bank-a, bank-b, bank-c, bank-d, central-bank-a, central-bank-b) — responsible for UAT validation and operational sign-off |

### Responsibility Matrix (RACI)

| Phase | LNET | Banks | Notes |
|-------|------|-------|-------|
| Test Infrastructure Setup | **R / A** | C | LNET deploys devnet and testnet; Banks provide test account data |
| Smart Contract Unit Tests | **R / A** | I | Foundry suite; no bank involvement required |
| Backend API Unit Tests | **R / A** | I | Go unit suite; all dependencies mocked |
| Integration Tests | **R / A** | C | Live Besu/Paladin; Banks consulted for acceptance criteria |
| E2E Core Flows | **R / A** | **R** | LNET executes flows; Banks verify outcomes via portal and API |
| Performance & Security | **R / A** | I | k6/Caliper/ZAP; Banks informed of final results |
| User Acceptance Testing (UAT) | C / **A** | **R** | Banks lead UAT; LNET provides support and resolves defects within 24 h |
| Issue Resolution & Retest | **R / A** | **R** | LNET fixes defects; Banks confirm retest |
| Evidence Bundle & Sign-off | **R / A** | **R** | Joint sign-off required for Deliverable 12 submission |

> **R** = Responsible (executes the work) · **A** = Accountable (owns the outcome) · **C** = Consulted (provides input) · **I** = Informed (notified of results)

### Execution Timeline

| Phase | Duration | Window | Responsible | Key Output |
|-------|----------|--------|-------------|------------|
| **Phase 0 — Mobilization** | 1 week | Week 1 | LNET | Devnet up, test data seeded, accounts provisioned |
| **Phase 1 — Unit Testing** | 1 week | Week 2 | LNET | Forge + Go test results; ≥ 80 % coverage report |
| **Phase 2 — Integration Testing** | 1 week | Week 3 | LNET | Integration test results; blockchain connectivity confirmed |
| **Phase 3 — E2E Core Flows** | 1 week | Week 4 | LNET + Banks | E2E evidence bundle (HTLC, escrow, compliance) |
| **Phase 4 — Performance & Security** | 1 week | Week 5 | LNET | k6/Caliper report; ZAP scan; acceptance thresholds validated |
| **Phase 5 — User Acceptance Testing** | 2 weeks | Weeks 6–7 | Banks (LNET support) | UAT sign-off report; defect log |
| **Phase 6 — Regression & Retest** | 1 week | Week 8 | LNET + Banks | Closed defect list; final evidence bundle |
| **Phase 7 — Sign-off & Submission** | 3 days | Week 9 | LNET + Banks | Signed test report; Deliverable 12 submitted |

> Total: **~9 weeks** from mobilization to submission. Actual calendar dates are to be agreed with the IDB Technical Committee.

### Phase Checklists

#### Phase 0 — Mobilization (Week 1)

**LNET:** deploy test environment · **Banks:** confirm UAT participants and schedule

- [ ] Devnet deployed (`make spoke-all`)
- [ ] All 6 API gateways healthy (`/healthz` 200 OK)
- [ ] Test participant accounts provisioned and registered in IdentityRegistry with `Verified` status
- [ ] tCeBM initial balances seeded per entity
- [ ] Test tooling installed (Foundry, Go ≥ 1.26, k6, OWASP ZAP)
- [ ] Banks identify UAT testers per portal role

#### Phase 1 — Unit Testing (Week 2)

**LNET:** execute · **Banks:** informed

- [ ] `cd scenario-a/contracts && forge test -v` — all Foundry tests pass
- [ ] `cd scenario-a/backend && go test ./...` — all Go unit tests pass
- [ ] Coverage report generated; ≥ 80 % on HTLC, tCeBM, FXAgreement, IdentityRegistry
- [ ] Foundry fuzz tests executed (≥ 10 000 runs per invariant)
- [ ] Results documented in evidence bundle

#### Phase 2 — Integration Testing (Week 3)

**LNET:** execute · **Banks:** consulted on acceptance criteria

- [ ] API ↔ Besu integration confirmed (real transaction broadcast + receipt)
- [ ] Paladin/Zeto ZKP generation and verification confirmed
- [ ] gRPC chain (api-gateway → payment-orchestrator → besu client) validated end-to-end
- [ ] AML blocking at API layer confirmed (sanctioned address rejected before on-chain call)
- [ ] Results documented

#### Phase 3 — E2E Core Flows (Week 4)

**LNET:** execute flows · **Banks:** verify results via API responses and portal

- [ ] `tryout-spoke-a-bank-a.sh` — Spoke-A onboarding + lifecycle
- [ ] `tryout-spoke-b-bank-b.sh` — Spoke-B onboarding + lifecycle
- [ ] `tryout-fx-agreement-e2e.sh` — Full HTLC cross-spoke settlement (happy path + timeout refund)
- [ ] `tryout-escrow-flow.sh` — Full deposit → escrow → redeem
- [ ] `tryout-cacti-interop.sh` — Cacti relay health and event propagation
- [ ] `tryout-compliance-participants.sh` — Compliance and AML screening
- [ ] Interoperability resilience scenarios: relay downtime, network partition, duplicate messages
- [ ] Evidence bundle generated per flow

#### Phase 4 — Performance & Security (Week 5)

**LNET:** execute · **Banks:** informed of results

- [ ] k6 steady-state load: ≥ 50 TPS token transfers confirmed
- [ ] k6 burst/spike profile executed
- [ ] Caliper smart contract benchmarks: TTF < 5 s, error rate < 1 %
- [ ] OWASP ZAP baseline scan: zero critical or high-severity findings
- [ ] Foundry fuzz security invariants executed (HTLC, AMM)
- [ ] Performance report (k6 HTML + Caliper JSON) archived

#### Phase 5 — User Acceptance Testing (Weeks 6–7)

**Banks:** execute · **LNET:** support with 24 h defect resolution SLA

| Entity | Portal | UAT Focus |
|--------|--------|-----------|
| bank-a | Bank Portal | Onboarding, FX agreement initiation, balance view |
| bank-b | Bank Portal | FX agreement acceptance, HTLC settlement |
| bank-c | Bank Portal | Onboarding, token transfers |
| bank-d | Bank Portal | Onboarding, token transfers |
| central-bank-a | Treasury Portal | Deposit approval, escrow approval, redeem approval |
| central-bank-b | Treasury Portal | Equivalent flows to central-bank-a |
| Governance Officer | Governance Portal | Participant approval, policy configuration |
| Supervisor | Supervisor Portal | Regulatory oversight views, audit log |
| NOC | NOC Dashboard | Network monitoring, health alerts |

- [ ] Each entity validates its own flows end-to-end
- [ ] Banks submit defects with reproduction steps; LNET acknowledges within 4 h and resolves within 24 h
- [ ] UAT defect log completed; critical/major issues escalated immediately

#### Phase 6 — Regression & Retest (Week 8)

**LNET:** fix defects · **Banks:** confirm resolution

- [ ] All critical and major defects from UAT closed
- [ ] Regression E2E suite re-executed after each fix batch
- [ ] Banks confirm fixed defects via targeted retest
- [ ] Final evidence bundle assembled and validated

#### Phase 7 — Sign-off & Submission (Week 9)

**LNET + Banks:** joint

- [ ] LNET produces final Test Report with complete evidence bundle
- [ ] Banks sign UAT acceptance form
- [ ] Deliverable 12 package submitted to IDB Technical Committee

---

## Testing Methodology

| Layer | Framework | Scope |
|-------|-----------|-------|
| **Smart Contracts** | [Foundry](https://getfoundry.sh/) (`forge test`) | Solidity unit tests with full EVM isolation; fuzz tests for HTLC and AMM invariants |
| **Backend API** | Go standard `testing` package | Unit tests with mocked gRPC and blockchain dependencies; isolated per-service |
| **Integration** | Go `testing` + local Besu/Paladin testnet | Live infrastructure; tests sign real transactions and verify on-chain state |
| **E2E** | Bash scripts (`tryout-*.sh`) + API-First runner | Full lifecycle execution via REST API; on-chain finality confirmed via `eth_getTransactionReceipt` |
| **Performance** | `k6` (API) + `Hyperledger Caliper` (smart contracts) | Load, burst, stress, and soak profiles |
| **Security** | OWASP ZAP + Foundry fuzz + manual abuse testing | RBAC escalation, input fuzzing, idempotency, rate limiting |

**Evaluation criteria (binary):**

- **Pass**: actual result matches expected result exactly; no side effects on ledger or privacy layer; execution within time budget
- **Fail**: result differs, system crashes, test times out, or execution cannot reach a terminal state

---

## Test Levels

### Unit Testing

Validates discrete, isolated functions with all external dependencies mocked.

- **Smart Contracts**: function-level tests via Foundry (EVM state isolated per test); covers HTLC lock/settle/refund, tCeBM mint/burn, FXAgreement lifecycle, IdentityRegistry RBAC, AMM constant-product formula
- **Backend API**: Go unit tests; database connections, blockchain clients, and gRPC channels are substituted with mocks; covers request handlers, middleware (JWT validation, RBAC), routing, and service-layer logic

### Integration Testing

Validates coordination between components with live local infrastructure.

- **Blockchain Connectivity**: API signs real transactions against a local Besu node; verifies transaction broadcast, event log parsing, and async state updates
- **Paladin Privacy Layer**: compliance service connects to a local Paladin node; validates ZKP generation/verification for Zeto transfers
- **gRPC Orchestration**: api-gateway → payment-orchestrator → besu client chain validated end-to-end with real gRPC calls

### End-to-End (E2E) Testing

Validates complete user journeys via the REST API against a full running stack (`make spoke-all`).

All E2E tests follow the **API-First** approach: no UI interaction; assertions are made against both the API response and direct on-chain state via `eth_getLogs` and contract read functions.

Existing E2E scripts (in `scenario-a/tryouts/`):

| Script | Flow |
|--------|------|
| `tryout-spoke-a-bank-a.sh` | Onboarding + token lifecycle (bank-a, Spoke-A) |
| `tryout-spoke-a-bank-c.sh` | Onboarding + token lifecycle (bank-c, Spoke-A) |
| `tryout-spoke-b-bank-b.sh` | Onboarding + token lifecycle (bank-b, Spoke-B) |
| `tryout-spoke-b-bank-d.sh` | Onboarding + token lifecycle (bank-d, Spoke-B) |
| `tryout-fx-agreement-e2e.sh` | Full FX Agreement + HTLC cross-spoke settlement |
| `tryout-escrow-flow.sh` | Full deposit → escrow → redeem lifecycle |
| `tryout-cacti-interop.sh` | Cacti relay health + PluginLedgerConnectorBesu validation |
| `tryout-compliance-participants.sh` | Participant compliance screening |
| `tryout-internal-relay-auth.sh` | Internal relay authentication |
| `tryout-my-onboarding-status.sh` | Onboarding status polling |

---

## API-First E2E Execution Strategy

### API-Driven E2E Flows

E2E test runners programmatically authenticate, invoke lifecycle endpoints, and assert both API responses and on-chain state:

**Scenario A (Bilateral HTLC cross-spoke):**

1. Authenticate as bank-a and bank-b via `/api/v1/auth/login`
2. Propose FX Agreement: `POST /api/v1/payments/fx/agreements` (bank-a → bank-b)
3. Accept: `POST /api/v1/payments/fx/agreements/{tradeId}/accept` (bank-b)
4. Lock funds (initiator): `POST /api/v1/htlc/lock` (bank-a, Spoke-A)
5. Wait for Cacti relay to detect lock event and propagate hash
6. Lock with hash (responder): `POST /api/v1/htlc/lock-with-hash` (bank-b, Spoke-B)
7. Settle: `POST /api/v1/htlc/settle` — relay extracts secret and submits to Spoke-A automatically
8. Assert FX Agreement status transitions to `SETTLED`
9. Assert both spoke balances via contract read functions

**Scenario A (Escrow flow):**

1. Authenticate as bank-a and central-bank-a
2. Register deposit: `POST /internal/v1/payments/deposits`
3. CB approves deposit + mints fCeBM
4. Request escrow: `POST /internal/v1/payments/escrows` (burns fCeBM, mints tCeBM via Zeto)
5. CB approves escrow
6. Request redeem: `POST /internal/v1/payments/redeems` (Zeto transfer + fCeBM mint)
7. CB approves redeem
8. Assert final balances (fCeBM restored, tCeBM zero)

### On-Chain Verification and Finality Rules

After each API call that broadcasts a transaction:

1. **Event Parsing**: query Besu RPC (`eth_getLogs`) to verify expected contract events were emitted with correct parameters
2. **Finality Confirmation**: poll `eth_getTransactionReceipt` every 1 second; assert finality once `receipt.blockNumber + 1` is reached (QBFT = 1-block finality)
3. **State Verification**: direct contract read calls confirm state transitions match expected output

---

## Traceability Map

| Use Case | Test ID(s) | Type | Requirement | Module | Status |
|----------|-----------|------|------------|--------|--------|
| Participant Onboarding | UT-SC-A-24 to A-29, UT-API-A-10 | Unit | REQ-COM-001 | Compliance | Implemented |
| tCeBM Issuance | UT-SC-A-19 to A-23, UT-API-A-14 | Unit | REQ-CAP-001 | Capital | Implemented |
| HTLC Cross-Spoke Settlement | UT-SC-A-01 to A-18, INT-API-A-04, E2E-A-03 | Unit/Integration/E2E | REQ-PAY-007 | Payments/Interop | Implemented |
| HTLC Timeout Refund | UT-SC-A-11, UT-SC-A-12, E2E-A-03 | Unit/E2E | REQ-PAY-008 | Interop/Capital | Implemented |
| FX Agreement Lifecycle | UT-SC-A-42 to A-71, UT-API-A-15, INT-API-A-10, E2E-A-04 | Unit/Integration/E2E | REQ-PAY-007 | Payments | Implemented |
| Escrow Flow | E2E-A-05 | E2E | REQ-CAP-005 | Capital/Paladin | Implemented |
| Compliance AML Screening | UT-API-A-10 to A-12, INT-API-A-07 | Unit/Integration | REQ-COM-001, REQ-COM-003 | Compliance/Privacy | Implemented |
| Governance Emergency Freeze | UT-SC-A-25, INT-API-A-10 | Unit/Integration | REQ-COM-003 | Compliance | Implemented |
| AMM Swap (Exact Output) | UT-SC-B-07 to B-10, E2E-B-01 | Unit/E2E | REQ-PAY-007, REQ-FX-008 | FX/Payments | Planned (Scenario B) |
| AMM Pool Maintenance | UT-SC-B-03 to B-06, E2E-B-04 | Unit/E2E | REQ-FX-008 | FX/Governance | Planned (Scenario B) |

---

## Test Environments and Reproducible Topology

### Hub-and-Spoke Topology Assumptions (Scenario A)

| Network | Chain ID | Entities | Validator Nodes |
|---------|---------|---------|----------------|
| Spoke-A | 1338 | central-bank-a, bank-a, bank-c | 3 (QBFT) |
| Spoke-B | 1339 | central-bank-b, bank-b, bank-d | 3 (QBFT) |

The Hyperledger Cacti relay operates between Spoke-A and Spoke-B, subscribing to HTLC events and executing automatic cross-spoke settlement. There is no hub network in Scenario A.

### Component Versions (Validated SBOM)

| Component | Version |
|-----------|---------|
| Hyperledger Besu | v25.8.0 (QBFT) |
| Hyperledger Cacti | v2.1.0 |
| Paladin | v0.15 |
| Zeto | v0.2.2 |
| Go backend APIs | 1.26.0 |
| Solidity | ^0.8.20 |
| OpenZeppelin | v5.6.0 |

### Key Configuration for Tests

- **Block time**: 2s intervals; QBFT absolute finality in 1 block
- **Chain IDs**: Spoke-A (1338), Spoke-B (1339), Hub/Scenario-B (1337 — planned)
- **API Gateway timeouts**: 30 seconds
- **Relay cross-chain propagation timeout**: 15 seconds
- **HTLC time-locks in tests**: shortened to 600 seconds (10 min) for timeout/refund scenarios

### Devnet (Local Docker Compose)

- **Purpose**: CI/CD pipeline execution, unit/integration tests, rapid iteration
- **Persistence**: Ephemeral — spun up fresh per test run via `make spoke-all`
- **Seeding**: initialization scripts register test participants, mint initial tCeBM balances
- **Teardown**: `make spoke-all-down` destroys all state; `make deploy.down-infra` removes volumes
- **Readiness criteria**: all 6 API gateways return 200 on `/healthz`; Besu RPC reports blocks advancing; Cacti relay health endpoint 200 OK

### Testnet (Staging)

- **Purpose**: persistent E2E, performance benchmarking, stakeholder demonstration
- **Persistence**: long-lived multi-node deployment
- **Reset**: snapshot-restore to known golden state after destructive tests
- **Seeding**: predefined subset of static test accounts

### Readiness Checklist ("Environment Up" Criteria)

Before any integration, E2E, or performance suite may execute:

1. Besu RPC endpoints return `eth_syncing = false` and block heights are advancing on both spokes
2. Cacti relay returns 200 OK health with active WebSocket subscriptions to both spoke networks
3. `GET /healthz` returns 200 OK on all 6 API gateways
4. Contract addresses are present in `deploy/local/paladin/spoke-{a,b}/.deployed-addrs.env`
5. All 6 entities are registered in the IdentityRegistry with `Verified` status

---

## Performance Testing Strategy

### Workload Models

| Profile | Description |
|---------|-------------|
| **Steady State (Load)** | Expected average traffic over a standard operational window; establishes baseline |
| **Burst (Spike)** | Sharp transaction volume increase; validates rate limiting and queue resilience |
| **Stress** | Gradually increases load until degradation; identifies maximum TPS |
| **Soak** | Moderate continuous load over 12 hours; identifies memory leaks and state bloat |

### Target Metrics

- **Throughput (TPS)**: successfully finalized on-chain transactions per second
- **Time-to-Finality (TTF)**: broadcast to irreversible block confirmation (Besu QBFT)
- **API Latency**: p50 and p95 for both read (query) and state-changing operations
- **Error Rate**: failed HTTP requests (5xx, timeouts, contract reverts) vs. total
- **Resource Utilization**: CPU, Memory, Disk I/O across API pods, Cacti relay, and Besu validator nodes

### Acceptance Thresholds (Pilot-Grade)

| Metric | Threshold |
|--------|-----------|
| Throughput — token transfers | ≥ 50 TPS sustained |
| Throughput — Zeto shielded transactions | ≥ 15 TPS sustained |
| Time-to-Finality | < 5 seconds per spoke |
| API Latency — read operations | p50 < 200ms, p95 < 500ms |
| API Latency — state-changing operations | p50 < 800ms, p95 < 1500ms (before on-chain finality) |
| Error Rate (steady state) | < 1% |
| Resource Stability | No memory leaks or node crashes during 12-hour soak |

### Tooling

- **API Load Generation**: `k6` — scripted HTTP load with JWT authentication and dynamic payload generation
- **Smart Contract Benchmarking**: `Hyperledger Caliper` — direct RPC-level load against Besu nodes
- **Monitoring**: `Prometheus` (metrics scraping) + `Grafana` (dashboards and snapshot exports)

---

## Security and Abuse Testing Strategy

### API Security (OWASP Controls)

All endpoints defined in the OpenAPI spec (`scenario-a/apis/openapi/api-gateway.yaml`) are subject to:

| Control | Test |
|---------|------|
| **AuthN / AuthZ / RBAC** | Privilege escalation attempts: `ROLE_COMMERCIAL_BANK` token accessing `/api/v1/governance/approve-kyc` or relay endpoints; missing/expired/malformed JWTs must return 401; insufficient roles must return 403 |
| **Input Validation** | Fuzzing with malformed JSON, boundary-exceeding numerics, SQL/NoSQL injection vectors; must return 400 rather than crashing |
| **Replay / Idempotency** | State-changing calls (`/api/v1/htlc/lock`, `/api/v1/payments/fx/agreements`) submitted with duplicate `Idempotency-Key`; first call accepted, subsequent calls return 409 |
| **Rate Limiting** | High-frequency blasts to verify 429 triggers before backend saturation |
| **DAST** | OWASP ZAP baseline scan against Testnet API gateways on each nightly pipeline run |

### Smart Contract Security and Invariants

- **Access Control**: administrative functions restricted to `GOVERNANCE_ROLE` / `CENTRAL_BANK_ROLE` (OpenZeppelin `AccessControl`)
- **HTLC Invariants (Foundry Fuzz)**: funds cannot be claimed without the exact pre-image; refund cannot execute before `timeLock` expiry; `INVALID → LOCKED → SETTLED/REFUNDED` FSM cannot be violated
- **AMM Invariants (Foundry Fuzz)**: constant-product formula `x · y = k` is maintained after every swap; `swapExactOutput` reverts when calculated input exceeds `maxInput`; circuit breaker blocks all state changes when paused
- **Pause / Circuit Breaker**: `GOVERNANCE_ROLE` can pause; unauthorized callers receive 403; all state-changing operations revert with `"Contract Paused"` while paused

### Resilience and "Fail Closed" Behavior

When Paladin ZKP verifier or external compliance oracle is unresponsive, the payment-orchestrator must default to Fail Closed: return 503 or 500 to the initiator; no state changes or locked funds are orphaned on-chain.

---

## Time Budgets, Finality, and Flakiness Controls

### Time Budgets per Critical Segment

| Segment | Budget |
|---------|--------|
| API Synchronous Timeout | 30 seconds |
| On-Chain Transaction Mining (local node) | 10 seconds |
| Cacti Relay Cross-Chain Propagation (lock → counter-lock) | 15 seconds |
| Full HTLC Settlement Lifecycle (FX Agreement → final settlement) | 60 seconds |

### Deterministic Finality Confirmation

E2E and integration test runners must not use arbitrary `sleep`. Instead:

1. Receive transaction hash from the API response
2. Poll `eth_getTransactionReceipt` every 1 second (maximum: time budget ÷ 1s attempts)
3. Assert finality once current block height ≥ `receipt.blockNumber + 1`
4. Proceed to next assertion step

QBFT consensus provides absolute single-block finality; no chain reorganizations occur under normal conditions.

### Flakiness Controls

- **Deterministic Seeding**: test accounts are logically partitioned; concurrent test threads use disjoint wallet sets to avoid EVM nonce collisions
- **Dynamic Identifiers**: every test run generates unique UUIDs for `agreementId`, `Idempotency-Key`, and `X-Correlation-Id`; hardcoded IDs are prohibited
- **Idempotent Cleanup**: on persistent Testnet, tests must gracefully conclude their lifecycle (trigger `/api/v1/htlc/refund` if a lock times out) before the runner exits

---

## Interoperability Resilience Scenarios

| Scenario | Description | Expected Outcome |
|----------|-------------|-----------------|
| **Relay Downtime & Recovery** | Cacti relay is terminated after HTLC lock is confirmed on Spoke-A but before cross-chain submission to Spoke-B; relay is restarted after 60 seconds | Relay sweeps blockchain for missed events, recovers state, and successfully executes the delayed counter-lock |
| **Delayed / Duplicated Messages** | Same cross-chain proof submitted multiple times concurrently to the destination contract | First valid transaction accepted; subsequent duplicates revert with `"Already Settled"` |
| **Network Partitioning** | Connection between Spoke-A and Spoke-B simulated as severed; in-flight HTLC nears `timeLock` expiry | Initiator can call `refund` after expiry; funds return to sender; no orphaned escrow |
| **Spoke Node Restart** | One Besu validator node restarted mid-HTLC lifecycle | QBFT consensus continues with remaining validators; transaction is re-broadcast and mined after reconnection |

---

## CI/CD Quality Gates and Pipeline Strategy

> **Status: Pending.** CI/CD pipelines are not yet configured for this repository. The specifications below define the target architecture to be implemented. Tests can currently be executed manually following the instructions in each section of this plan.

### Pull Request (PR) Pipeline — Planned

To be triggered on every commit to an open PR targeting `main` or `develop`.

- **Scope**: static analysis (SonarQube), Foundry unit tests (`forge test`), Go unit tests (`go test ./...`), lightweight integration tests against local devnet
- **Time Budget**: ≤ 10 minutes
- **Pass Criteria**:
  - 100% pass rate on unit tests
  - Minimum 80% code coverage on core business logic
  - Zero critical or high-severity vulnerabilities in smart contract bytecode
  - Failure blocks the PR from being merged

**Manual equivalent (current):**

```bash
# Smart contract unit tests
cd scenario-a/contracts && forge test -v

# Go unit tests (all services)
cd scenario-a/backend && go test ./...
```

### Nightly E2E & Resilience Pipeline — Planned

To be triggered every night against the persistent Testnet (staging) environment.

- **Scope**: full API-driven E2E suite (Scenario A), Interoperability Resilience tests, Security/Abuse controls (OWASP ZAP)
- **Time Budget**: ≤ 2 hours
- **Pass Criteria**:
  - 100% pass rate on core functional E2E flows
  - Successful completion of relay recovery resilience scenario
  - Automated generation and storage of the Standard Test Evidence Bundle

**Manual equivalent (current):**

```bash
# Full backend stack
cd scenario-a && make spoke-all

# E2E flows
./tryouts/tryout-fx-agreement-e2e.sh
./tryouts/tryout-escrow-flow.sh
./tryouts/tryout-cacti-interop.sh
./tryouts/tryout-spoke-a-bank-a.sh
./tryouts/tryout-spoke-b-bank-b.sh
```

---

## Evidence Bundle Format

Every E2E and integration pipeline run generates a standard evidence archive:

```
evidence-bundle-<run-id>/
├── execution_summary.json       # Machine-readable pass/fail per step
├── aggregated_traces.log        # X-Correlation-Id → txHash → blockNumber mapping
└── performance_report.html      # k6 HTML output (nightly performance runs only)
```

**`execution_summary.json` schema:**

```json
{
  "passed": true,
  "total_duration_ms": 42000,
  "run_id": "uuid-v4",
  "results": [
    {
      "scenario_id": "E2E-A-04",
      "step": "lock-initiator",
      "http_status": 202,
      "tx_hash": "0x...",
      "block_number": 1042,
      "latency_ms": 310,
      "passed": true
    }
  ]
}
```

**`aggregated_traces.log` format:**

```
<X-Correlation-Id (UUIDv4)> | <txHash (bytes32)> | <blockNumber (uint256)> | <network (spoke-a|spoke-b)>
```

This bundle is stored as a CI artifact and retained for technical validation by the LNET Technical Committee.

---

## Scenario B — Planned Extensions

> **Status:** Smart contracts implemented (`AutomatedMarketMaker.sol`, `ManualOracle.sol`, `FXAgreement.sol`); hub deployment and end-to-end flows pending.

When Scenario B (AMM hub, chain 1337) is deployed, this plan will be extended with:

| Addition | Description |
|----------|-------------|
| Hub topology assumptions | International Hub with 4+ validator nodes (consortium of Central Banks) |
| AMM unit tests | Constant-product pricing, slippage protection, LP token mechanics, circuit breaker (tests already exist in `contracts/test/AutomatedMarketMaker.t.sol`) |
| AMM integration tests | Quote service, liquidity addition approval (multi-step ERC-20 allowance), swap execution with ZK-Pointers, pool imbalance detection |
| E2E-B flows | Exact-Output Swap (happy path), Slippage Protection (volatility test), Governance Circuit Breaker (emergency stop), Liquidity Imbalance Alert |
| Performance Scenario B | Additional k6/Caliper profiles for AMM swap throughput and LP operations |
| Hub readiness checklist | Liquidity provisioned (AMM pool > 0 for both assets), ManualOracle price feed active |
