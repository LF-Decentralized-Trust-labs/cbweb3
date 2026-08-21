# Test Execution Plan — CBWeb3 Platform (Scenario B)

> **Deliverable 12** · CBDC System Test Execution Plan — Scenario B: AMM Hub and Cross-Currency Liquidity
>
> Project: RG-T4567 · Suboperation: ATN/KS-21330-RG
> Authors: Lucas Campelo, Samuel Venzi
> Date: 2026-05-29

---

> **Work in Progress.** Scenario B smart contracts and core backend services are implemented. Several test areas — including the commercial-bank cross-currency swap E2E path, full integration tests against live infrastructure, and CI/CD pipelines — are Partial or Planned. This plan documents the target state and accurately marks the current completion status of each test area.

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
- [CI/CD Quality Gates and Pipeline Strategy](#cicd-quality-gates-and-pipeline-strategy)
- [Evidence Bundle Format](#evidence-bundle-format)
- [Known Test Coverage Gaps](#known-test-coverage-gaps)

---

## Introduction

This Test Execution Plan establishes the formal quality assurance strategy for Scenario B of the CBWeb3 platform — the AMM-based hub network extension that enables cross-currency wholesale CBDC settlement between jurisdictions. Scenario B introduces a shared International Hub network (chain 1337) operated by a consortium of Central Banks, an Automated Market Maker (AMM) for FX liquidity provisioning, a Multilateral Liquidity Provider (MLP), a PairRegistry and LiquidityCommitRegistry for cooperative pair formation, and a SpokeBridge for locking and minting hub-wrapped tokens (W-tCeBM).

This plan validates functional and non-functional integrity across the three-tier testing hierarchy (unit, integration, end-to-end), covering hub smart contracts, backend API services for all entities (bank-a, bank-b, central-bank-a, central-bank-b, MLP), the Hyperledger Cacti relay (CommitMatched event detection and cross-spoke relay), and the AMM FX swap lifecycle.

Scenario B runs after and extends Scenario A. The Scenario A Test Execution Plan (`scenario-a/docs/test-execution-plan.md`) and the Scenario A Test Catalog (`scenario-a/tests/TEST-CATALOG.md`) remain valid for shared components (base IdentityRegistry, tCeBM, fCeBM, HTLC, baseline Cacti relay).

The complete Scenario B test case catalog is maintained in [`scenario-b/tests/TEST-CATALOG.md`](../tests/TEST-CATALOG.md).

---

## Scope

The following components and layers are explicitly **in scope**:

- **Hub Smart Contracts**: AutomatedMarketMaker, PairRegistry, LiquidityCommitRegistry, ManualOracle, FXAgreement (hub), SpokeBridge, CurrencyRegistry, IdentityRegistry (hub), TokenizedCentralBankMoney (hub tCeBM and W-tCeBM), CBWeb3Hub deployment validation, HashTimeLockedContract (hub-side)
- **Backend API Services**: api-gateway (REST v2 AMM, bridge, and liquidity endpoints), payment-orchestrator (FX agreement, AMM client adapter), compliance (KYC/AML for hub participants) — all 5 entities (bank-a, bank-b, central-bank-a, central-bank-b, MLP)
- **Cooperative Liquidity Flow**: PairRegistry bilateral pair proposal/confirmation → LiquidityCommitRegistry commit-reveal → CommitMatched event detection → AMM pool seeding
- **Commercial Bank FX Swap**: hub AMM quote → exact-output swap → bridge round-trip (SpokeBridge lockAndMint / burnAndUnlock)
- **MLP Bilateral Liquidity Provisioning**: MLP registers matched commits on both sides; Cacti detects CommitMatched event and executes
- **FX Agreement Lifecycle on Hub**: PROPOSED → ACCEPTED → SETTLED / REJECTED / CANCELLED via hub FXAgreement contract
- **SpokeBridge**: Lock-and-mint (spoke tCeBM → hub W-tCeBM) and burn-and-unlock (hub W-tCeBM → spoke tCeBM)
- **Cacti Relay**: CommitMatched event subscription, health validation, cross-spoke relay
- **Compliance Layer**: KYC/AML screening, participant onboarding, status management for hub entities
- **Performance Testing**: AMM quote latency, swap latency, pool status monitoring
- **Test Environments**: Devnet (local Docker Compose via `make scenario-b.up-infra`) and Testnet (staging)

The following are explicitly **out of scope**:

- Hyperledger Besu core QBFT consensus protocol (validated by upstream maintainers)
- Hub network validator node isolation and Byzantine fault tolerance (infrastructure concern; not application-layer)
- Cross-Chain Interoperability Protocol (CCIP) — not used in this implementation
- Frontend UI pixel-level testing (covered by API-First E2E which validates backend and on-chain state)
- Paladin node internals beyond the API surface exposed to the platform
- Scenario A HTLC cross-spoke bilateral settlement (covered in the Scenario A plan)

---

## Test Goals

| Goal | Metric |
|------|--------|
| **Logic Validation** | >= 80% code coverage on core hub smart contract business logic (AMM constant-product formula, FXAgreement state machine, LiquidityCommitRegistry TTL, PairRegistry bilateral approval) |
| **Quote Accuracy** | AMM `getAmountIn` returns a value consistent with the constant-product formula `x * y = k` across all valid reserve states; assertion tolerance = 0 wei (exact math) |
| **Swap Atomicity** | Exact-output swap either completes fully (exact token output delivered, correct input deducted, pool reserves updated) or reverts fully (no tokens transferred, no reserve change) |
| **AMM Invariant Preservation** | After every swap and every liquidity operation, the product `reserveA * reserveB >= k_before` holds; enforced via Foundry fuzz tests |
| **Bridge Round-Trip Integrity** | lockAndMint and burnAndUnlock are symmetric: locked spoke balance equals minted hub W-tCeBM; burned hub W-tCeBM equals unlocked spoke balance; no token creation or destruction |
| **Compliance Enforcement** | Sanctioned or unverified participants are blocked at the API layer before any on-chain hub interaction |
| **Relay Event Detection** | Cacti relay successfully subscribes to CommitMatched events on the hub; detected events trigger correct cross-spoke action within the relay propagation time budget |
| **Performance Baseline** | AMM API meets pilot-grade thresholds: quote p95 <= 300ms, swap p95 <= 6000ms, pool status p95 <= 15000ms, error rate < 1% |

---

## Test Execution Timeline and Responsibilities

### Stakeholders

| Stakeholder | Role |
|-------------|------|
| **LNET** | Platform developer and integrator — responsible for hub smart contract, API, infrastructure, tooling delivery, and test execution |
| **Banks** | Participant institutions (bank-a, bank-b, central-bank-a, central-bank-b, MLP if enabled) — responsible for UAT validation and operational sign-off |

### Responsibility Matrix (RACI)

| Phase | LNET | Banks | Notes |
|-------|------|-------|-------|
| Test Infrastructure Setup | **R / A** | C | LNET deploys hub devnet; Banks provide test account data |
| Smart Contract Unit Tests | **R / A** | I | Foundry suite on hub contracts; no bank involvement required |
| Backend API Unit Tests | **R / A** | I | Go unit suite; all dependencies mocked |
| Integration Tests | **R / A** | C | Live Hub Besu + Paladin + AMM; Banks consulted for acceptance criteria |
| E2E Core Flows | **R / A** | **R** | LNET executes flows; Banks verify outcomes via portal and API |
| Performance & Security | **R / A** | I | k6 / Foundry fuzz; Banks informed of final results |
| User Acceptance Testing (UAT) | C / **A** | **R** | Banks lead UAT; LNET provides support and resolves defects within 24 h |
| Issue Resolution & Retest | **R / A** | **R** | LNET fixes defects; Banks confirm retest |
| Evidence Bundle & Sign-off | **R / A** | **R** | Joint sign-off required for Deliverable 12 submission |

> **R** = Responsible (executes the work) · **A** = Accountable (owns the outcome) · **C** = Consulted (provides input) · **I** = Informed (notified of results)

### Execution Timeline

| Phase | Duration | Window | Responsible | Key Output |
|-------|----------|--------|-------------|------------|
| **Phase 0 — Mobilization** | 1 week | Week 1 | LNET | Hub devnet up, test data seeded, hub accounts provisioned |
| **Phase 1 — Unit Testing** | 1 week | Week 2 | LNET | Forge + Go test results; >= 80% coverage report |
| **Phase 2 — Integration Testing** | 1 week | Week 3 | LNET | Integration test results; hub blockchain connectivity confirmed |
| **Phase 3 — E2E Core Flows** | 1 week | Week 4 | LNET + Banks | E2E evidence bundle (cooperative liquidity, commercial swap, bridge round-trip) |
| **Phase 4 — Performance & Security** | 1 week | Week 5 | LNET | k6 report; AMM fuzz invariant results; acceptance thresholds validated |
| **Phase 5 — User Acceptance Testing** | 2 weeks | Weeks 6–7 | Banks (LNET support) | UAT sign-off report; defect log |
| **Phase 6 — Regression & Retest** | 1 week | Week 8 | LNET + Banks | Closed defect list; final evidence bundle |
| **Phase 7 — Sign-off & Submission** | 3 days | Week 9 | LNET + Banks | Signed test report; Deliverable 12 submitted |

> Total: **~9 weeks** from mobilization to submission. Scenario B execution begins after Scenario A baseline has been established. Actual calendar dates are to be agreed with the IDB Technical Committee.

### Phase Checklists

#### Phase 0 — Mobilization (Week 1)

**LNET:** deploy hub test environment · **Banks:** confirm UAT participants and schedule

- [ ] Hub devnet deployed (`make scenario-b.up-infra` + `make scenario-b.deploy-contracts`)
- [ ] Hub Besu RPC reports blocks advancing (`:8645`)
- [ ] Spoke-B Besu RPC reports blocks advancing (`:8745`)
- [ ] All 6+ API gateways healthy (`/healthz` 200 OK)
- [ ] Hub contract addresses non-empty in all entity `.env.infra.*` files
- [ ] Hub IdentityRegistry has all entities registered with `Verified` status
- [ ] AMM pool seeded with initial liquidity for BRL-USD pair
- [ ] ManualOracle price feed active and rate set for all registered pairs
- [ ] Cacti relay health endpoint returns `{"status":"ok","watcher_active":true}`
- [ ] Test participant accounts provisioned (bank-a, bank-b, central-bank-a, central-bank-b, MLP)
- [ ] Test tooling installed (Foundry, Go >= 1.26, k6)
- [ ] Banks identify UAT testers per portal role

#### Phase 1 — Unit Testing (Week 2)

**LNET:** execute · **Banks:** informed

- [ ] `make scenario-b.test-contracts` — all Foundry hub contract tests pass
- [ ] `make scenario-b.test-backend` — all Go unit tests pass
- [ ] Coverage report generated; >= 80% on AMM, FXAgreement, LiquidityCommitRegistry, PairRegistry
- [ ] Foundry fuzz tests executed (>= 10 000 runs per AMM invariant)
- [ ] Results documented in evidence bundle

#### Phase 2 — Integration Testing (Week 3)

**LNET:** execute · **Banks:** consulted on acceptance criteria

- [ ] AMM quote endpoint responds against live hub contract
- [ ] AMM swap execution confirmed on-chain (real transaction broadcast + receipt)
- [ ] LiquidityCommitRegistry commit-reveal flow confirmed against live hub Besu node
- [ ] Cacti watcher successfully detects CommitMatched event on hub
- [ ] SpokeBridge lockAndMint confirmed on-chain (W-tCeBM minted on hub)
- [ ] FX Agreement hub settlement confirmed on-chain
- [ ] AML blocking at API layer confirmed (blocked participant rejected before on-chain call)
- [ ] Results documented

#### Phase 3 — E2E Core Flows (Week 4)

**LNET:** execute flows · **Banks:** verify results via API responses and portal

- [ ] `tryout-scenario-b-e2e.sh us1` — Cooperative liquidity pair formation (PairRegistry + LCR)
- [ ] `tryout-scenario-b-e2e.sh us2` — MLP bilateral liquidity provisioning
- [ ] `tryout-scenario-b-e2e.sh us3` — Commercial bank FX swap via AMM (governance path; note: commercial bank path is partial)
- [ ] `tryout-scenario-b-e2e.sh us4` — FX Agreement lifecycle on hub
- [ ] `tryout-scenario-b-e2e.sh us5` — PairRegistry bilateral pair approval
- [ ] `tryout-scenario-b-e2e.sh us6` — SpokeBridge Lock&Mint / Burn&Unlock
- [ ] `tryout-cacti-interop.sh` — Cacti relay health + CommitMatched event detection
- [ ] Evidence bundle generated per flow

#### Phase 4 — Performance & Security (Week 5)

**LNET:** execute · **Banks:** informed of results

- [ ] `make scenario-b.perf-baseline` — k6 performance baseline executed
- [ ] AMM quote p95 <= 300ms confirmed
- [ ] AMM swap p95 <= 6000ms confirmed (excluding on-chain finality)
- [ ] Pool status p95 <= 15000ms confirmed
- [ ] Error rate < 1% at steady-state load
- [ ] Foundry fuzz AMM invariants executed (>= 10 000 runs; constant-product `x*y=k` never violated)
- [ ] RBAC escalation tests executed (COMMERCIAL_BANK token blocked on governance endpoints)
- [ ] Circuit breaker pause/resume tested under load
- [ ] Performance report (k6 JSON + HTML) archived

#### Phase 5 — User Acceptance Testing (Weeks 6–7)

**Banks:** execute · **LNET:** support with 24 h defect resolution SLA

| Entity | Portal | UAT Focus |
|--------|--------|-----------|
| bank-a | Bank Portal | Onboarding, FX swap initiation via AMM, bridge deposit, balance view |
| bank-b | Bank Portal | FX swap acceptance, bridge withdrawal, balance view |
| central-bank-a | Treasury Portal | Pair approval, liquidity provisioning, hub oversight |
| central-bank-b | Treasury Portal | Equivalent flows to central-bank-a |
| MLP | MLP Portal | Bilateral commit registration, LP share management, rebalancing |

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
| **Smart Contracts** | [Foundry](https://getfoundry.sh/) (`forge test`) | Solidity unit tests with full EVM isolation; fuzz tests for AMM constant-product invariant and circuit breaker |
| **Backend API** | Go standard `testing` package | Unit tests with mocked gRPC and blockchain dependencies; isolated per-service |
| **Integration** | Go `testing` + local Hub Besu node + live AMM | Live hub infrastructure; tests sign real transactions and verify on-chain state |
| **E2E** | Bash scripts (`tryout-*.sh`) + API-First runner | Full lifecycle execution via REST API; on-chain finality confirmed via `eth_getTransactionReceipt` |
| **Performance** | `k6` | AMM quote, swap, and pool-status load profiles; thresholds enforced as merge gates per Decision 13 |
| **Security** | Foundry fuzz + manual RBAC abuse testing | AMM invariant fuzzing, circuit breaker RBAC, input boundary testing |

**Evaluation criteria (binary):**

- **Pass**: actual result matches expected result exactly; no side effects on ledger state; execution within time budget; AMM invariant holds
- **Fail**: result differs, system crashes, test times out, AMM invariant violated, or execution cannot reach a terminal state

---

## Test Levels

### Unit Testing

Validates discrete, isolated functions with all external dependencies mocked.

- **Smart Contracts**: function-level tests via Foundry (EVM state isolated per test); covers AMM addLiquidity/removeLiquidity/swapExactOutput/circuit-breaker, LiquidityCommitRegistry FSM, PairRegistry bilateral approval, FXAgreement hub FSM, SpokeBridge authorization, ManualOracle RBAC, CurrencyRegistry uniqueness, IdentityRegistry hub RBAC, CBWeb3Hub deployment validation
- **Backend API**: Go unit tests; database connections, blockchain clients, and gRPC channels are substituted with mocks; covers AMM handler, bridge handler, liquidity handler, quote endpoint, FX agreement handler, auth/RBAC middleware for v2 endpoints

Run commands:

```bash
# Smart contract unit tests
make scenario-b.test-contracts
# expands to: cd scenario-b/contracts && forge test --match-contract "AutomatedMarketMakerTest|..." -vv

# Go unit tests
make scenario-b.test-backend
# expands to: cd scenario-b/backend && go test ./...
```

### Integration Testing

Validates coordination between components with live local hub infrastructure.

- **AMM Connectivity**: API signs real swap transactions against a live hub Besu node; verifies transaction broadcast, event log parsing (Swap, Sync events), and pool reserve updates
- **LiquidityCommitRegistry**: commit-reveal flow tested against live hub; CommitMatched event verified via `eth_getLogs`
- **Cacti Relay**: watcher subscribes to hub events; CommitMatched event detected and cross-spoke action triggered
- **SpokeBridge**: lockAndMint and burnAndUnlock tested against live spoke and hub nodes; W-tCeBM balance verified
- **FX Agreement Hub**: full lifecycle executed against live hub FXAgreement contract

> **Current Status**: Integration tests are partially implemented. AMM quote and FX agreement integration paths are further along than the bridge and LCR event-detection paths.

### End-to-End (E2E) Testing

Validates complete user journeys via the REST API against a full running hub stack (`make scenario-b.up`).

All E2E tests follow the **API-First** approach: no UI interaction; assertions are made against both the API response and direct on-chain state via `eth_getLogs` and contract read functions.

E2E scripts are located in `scenario-b/tryouts/`. See the [API-First E2E Execution Strategy](#api-first-e2e-execution-strategy) section for flow details.

| Script | User Story | Status |
|--------|-----------|--------|
| `tryout-scenario-b-e2e.sh us1` | US1: Cooperative liquidity pair formation | Implemented |
| `tryout-scenario-b-e2e.sh us2` | US2: MLP bilateral liquidity provisioning | Implemented |
| `tryout-scenario-b-e2e.sh us3` | US3: Commercial bank FX swap via AMM | Partial |
| `tryout-scenario-b-e2e.sh us4` | US4: FX Agreement lifecycle on hub | Implemented |
| `tryout-scenario-b-e2e.sh us5` | US5: PairRegistry bilateral pair approval | Implemented |
| `tryout-scenario-b-e2e.sh us6` | US6: SpokeBridge Lock&Mint / Burn&Unlock | Implemented |
| `tryout-cacti-interop.sh` | Cacti relay health + CommitMatched event detection | Implemented |
| `tryout-lp-bilateral-ratio.sh` | LP share ratio calculation | Implemented |
| `tryout-payment-routes.sh` | Payment routing validation | Implemented |

---

## API-First E2E Execution Strategy

### Three Main E2E Flows

#### Flow 1 — Cooperative Liquidity Pair Formation (US1 / US2 / US5)

1. Authenticate as central-bank-a and central-bank-b via `/api/v1/auth/login`
2. CB-A proposes pair: `POST /api/v2/hub/pair-registry/propose` (CB-A → CB-B, BRL-USD)
3. CB-B confirms pair: `POST /api/v2/hub/pair-registry/confirm/{pairId}` → pair status = `ACTIVE`
4. Verify via `GET /api/v2/hub/pair-registry/pairs` — pair appears in active list
5. MLP (or CB-A and CB-B) registers commit: `POST /api/v2/hub/liquidity/commit` (both sides)
6. LiquidityCommitRegistry emits `CommitMatched` when both-sided commits are registered within 72h TTL
7. Cacti relay detects `CommitMatched` event on hub → triggers AMM pool seeding via Cacti endpoint
8. AMM pool reserves updated; verify via `GET /api/v2/amm/pool/BRL-USD/status`
9. Assert: pair active, AMM pool non-zero reserves, LP shares credited to providers

- **Scripts**: `tryout-scenario-b-e2e.sh us1`, `tryout-scenario-b-e2e.sh us2`, `tryout-scenario-b-e2e.sh us5`
- **Status**: Implemented

#### Flow 2 — Commercial Bank FX Swap via AMM (US3)

1. Authenticate as bank-a (commercial bank) via `/api/v1/auth/login`
2. Request FX quote: `GET /api/v2/amm/quote/exact-output?pair=BRL-USD&amount_out=1000`
3. Assert: quote returns `amount_in`, `price_impact`, `fee`; values consistent with constant-product formula
4. Submit exact-output swap: `POST /api/v2/amm/swap/exact-output` with `max_amount_in` from quote
5. Assert: API returns `txHash`; on-chain state updated
6. Poll `eth_getTransactionReceipt` — confirm finality
7. Verify pool reserves via `eth_getLogs` (Sync event) — `reserveA * reserveB >= k_before`
8. Bridge out: bank-a calls `POST /api/v2/hub/bridge/burn-and-unlock` to convert W-tCeBMb back to spoke tCeBMb
9. Assert: W-tCeBM burned on hub; spoke tCeBM balance increased by corresponding amount
10. Assert: FX swap fully settled; no orphaned tokens on hub or spoke

- **Scripts**: `tryout-scenario-b-e2e.sh us3`, `samples/sample-tryout.sh`
- **Status**: Partial — governance path works; commercial bank direct swap path is in progress

#### Flow 3 — SpokeBridge Round-Trip (US6)

1. Authenticate as bank-a (commercial bank)
2. Bridge in: `POST /api/v2/hub/bridge/lock-and-mint` — lock spoke tCeBMa, receive hub W-tCeBMa
3. Poll hub `eth_getTransactionReceipt` — confirm W-tCeBMa minted
4. Assert: spoke tCeBMa balance decreased; hub W-tCeBMa balance increased by same amount
5. Bridge out: `POST /api/v2/hub/bridge/burn-and-unlock` — burn hub W-tCeBMa, receive spoke tCeBMa
6. Poll spoke `eth_getTransactionReceipt` — confirm spoke tCeBMa unlocked
7. Assert: hub W-tCeBMa balance = 0; spoke tCeBMa balance restored to original amount
8. Verify symmetric accounting: no tokens created or destroyed across the round-trip

- **Script**: `tryout-scenario-b-e2e.sh us6`
- **Status**: Implemented

### On-Chain Verification and Finality Rules

After each API call that broadcasts a transaction:

1. **Event Parsing**: query Besu RPC (`eth_getLogs`) to verify expected contract events were emitted with correct parameters (Swap, CommitRegistered, CommitMatched, PairProposed, PairConfirmed, Locked, Minted, Burned, Unlocked)
2. **Finality Confirmation**: poll `eth_getTransactionReceipt` every 1 second; assert finality once `receipt.blockNumber + 1` is reached (QBFT = 1-block finality)
3. **State Verification**: direct contract read calls confirm state transitions match expected output (pool reserves, pair status, commit status, bridge balances)

---

## Traceability Map

| Use Case | Test ID(s) | Type | Requirement | Module | Status |
|----------|-----------|------|------------|--------|--------|
| Cooperative Liquidity Pair Formation | UT-SC-B-AMM-01 to AMM-09, UT-SC-B-PAR-01 to PAR-06, E2E-B-01 | Unit/E2E | REQ-FX-008, REQ-LIQ-001 | AMM/PairRegistry | Implemented |
| MLP Bilateral Liquidity Provisioning | UT-SC-B-LCR-01 to LCR-08, INT-API-B-03, E2E-B-02 | Unit/Integration/E2E | REQ-LIQ-002 | LCR/MLP | Implemented |
| Commercial Bank FX Swap via AMM | UT-SC-B-AMM-05 to AMM-10, INT-API-B-01, INT-API-B-02, E2E-B-03 | Unit/Integration/E2E | REQ-PAY-007, REQ-FX-008 | AMM/Payments | Partial |
| FX Agreement Lifecycle on Hub | UT-SC-B-FXA-01 to FXA-09, INT-API-B-05, E2E-B-04 | Unit/Integration/E2E | REQ-PAY-007 | FXAgreement/Hub | Implemented |
| PairRegistry Bilateral Pair Approval | UT-SC-B-PAR-01 to PAR-06, E2E-B-05 | Unit/E2E | REQ-FX-008, REQ-GOV-003 | PairRegistry | Implemented |
| SpokeBridge Lock&Mint / Burn&Unlock | UT-SC-B-BRG-01 to BRG-06, INT-API-B-04, E2E-B-06 | Unit/Integration/E2E | REQ-PAY-009 | SpokeBridge | Implemented |
| AMM Quote Accuracy | UT-SC-B-AMM-03, UT-SC-B-AMM-04, INT-API-B-01 | Unit/Integration | REQ-FX-008 | AMM | Implemented |
| AMM Constant-Product Invariant (Fuzz) | UT-SC-B-AMM-11 to AMM-15 | Unit (Fuzz) | REQ-FX-008 | AMM | Implemented |
| AMM Circuit Breaker | UT-SC-B-AMM-08 to AMM-10, INT-API-B-07 | Unit/Integration | REQ-GOV-001 | AMM | Implemented |
| ManualOracle RBAC | UT-SC-B-ORC-01 to ORC-04 | Unit | REQ-GOV-002 | Oracle | Implemented |
| CurrencyRegistry Management | UT-SC-B-CUR-01 to CUR-04 | Unit | REQ-GOV-003 | CurrencyRegistry | Implemented |
| IdentityRegistry Hub RBAC | UT-SC-B-IDR-01 to IDR-06 | Unit | REQ-COM-001 | IdentityRegistry | Implemented |
| Hub Deployment Validation | UT-SC-B-HUB-01 to HUB-03 | Unit | REQ-INF-001 | CBWeb3Hub | Implemented |
| Compliance AML Screening (Hub) | UT-API-B-07, INT-API-B-08 | Unit/Integration | REQ-COM-001, REQ-COM-003 | Compliance | Implemented |
| AMM Performance Baseline | PERF-B-01 to PERF-B-03 | Performance | REQ-PERF-002 | AMM/API | Partial (targets draft) |
| Full Cross-Currency Payment Lifecycle | E2E-B-03, E2E-B-06 | E2E | REQ-PAY-007, REQ-PAY-009 | AMM/Bridge/Payments | Partial |

---

## Test Environments and Reproducible Topology

### Hub-and-Spoke Topology (Scenario B)

| Network | Chain ID | Entities | Validator Nodes |
|---------|---------|---------|----------------|
| Hub | 1337 | central-bank-a, central-bank-b, MLP | 4 (QBFT — consortium of Central Banks) |
| Spoke-A | 1338 | central-bank-a, bank-a | 3 (QBFT) |
| Spoke-B | 1339 | central-bank-b, bank-b | 3 (QBFT) |

The Hyperledger Cacti relay subscribes to hub events (CommitMatched) and spoke events, executing cross-chain actions. The hub AMM contract (`AutomatedMarketMaker.sol`) is deployed on chain 1337. SpokeBridge contracts are deployed on each spoke network and reference the hub for W-tCeBM minting.

### Component Versions (Validated SBOM)

| Component | Version |
|-----------|---------|
| Hyperledger Besu | v25.8.0 (QBFT) |
| Hyperledger Cacti | v2.1.0 |
| Go backend APIs | 1.26.0 |
| Solidity | ^0.8.20 |
| OpenZeppelin | v5.6.0 |
| k6 | latest stable |

### Key Configuration for Tests

- **Block time**: 2s intervals; QBFT absolute finality in 1 block
- **Chain IDs**: Hub (1337), Spoke-A (1338), Spoke-B (1339)
- **Hub RPC port**: 8645
- **Spoke-B RPC port**: 8745
- **API Gateway timeouts**: 30 seconds
- **Relay cross-chain propagation timeout**: 15 seconds
- **LiquidityCommitRegistry TTL**: 72 hours (production); shortened in tests for expiry scenarios
- **AMM slippage tolerance in tests**: 1% for happy-path; 0% for slippage-revert tests

### Devnet (Local Docker Compose)

- **Purpose**: unit/integration tests, rapid iteration, E2E flow development
- **Persistence**: Ephemeral — spun up fresh per test run
- **Start**: `make scenario-b.up-infra` then `make scenario-b.deploy-contracts`
- **Seeding**: initialization scripts register test participants, seed AMM pool with BRL-USD liquidity, activate ManualOracle rate
- **Teardown**: `make scenario-b.down-infra` destroys all state
- **Readiness criteria**: see readiness checklist below

### Testnet (Staging)

- **Purpose**: persistent E2E, performance benchmarking, stakeholder demonstration
- **Persistence**: long-lived multi-node hub + spoke deployment
- **Reset**: snapshot-restore to known golden state after destructive tests
- **Status**: Planned

### Readiness Checklist ("Environment Up" Criteria)

Before any integration, E2E, or performance suite may execute:

1. Hub Besu RPC (`:8645`) returns `eth_syncing = false` and block heights are advancing
2. Spoke-B Besu RPC (`:8745`) returns `eth_syncing = false` and block heights are advancing
3. `GET /api/v1/health` on Cacti relay returns `{"status":"ok","watcher_active":true}`
4. `GET /healthz` returns 200 OK on all 6+ API gateways
5. Hub contract addresses are non-empty in all entity `.env.infra.*` files
6. Hub IdentityRegistry has all entities registered with `Verified` status
7. AMM pool for the target pair has non-zero reserves (verified via `GET /api/v2/amm/pool/{pair}/status`)
8. ManualOracle has an active rate set for all registered currency pairs

---

## Performance Testing Strategy

### Workload Models

| Profile | Description |
|---------|-------------|
| **Baseline (Steady State)** | Constant VU load over 1 minute; establishes reference latency and throughput for AMM endpoints |
| **Burst (Spike)** | Sharp increase in quote and swap requests; validates AMM handler queue resilience and rate limiting |
| **Stress** | Gradually increasing VU count until p95 degrades past gate; identifies maximum sustainable TPS |
| **Soak** | Moderate continuous load over 12 hours; identifies memory leaks and pool state drift |

### Target Metrics

- **AMM Quote Latency**: p95 for `GET /api/v2/amm/quote/exact-output`
- **AMM Swap Latency**: p95 for `POST /api/v2/amm/swap/exact-output` (excluding on-chain finality)
- **Pool Status Latency**: p95 for `GET /api/v2/amm/pool/{pair}/status`
- **Error Rate**: failed HTTP requests (5xx, timeouts, contract reverts) vs. total
- **Throughput (TPS)**: successfully finalized on-chain swap transactions per second
- **AMM Invariant Stability**: constant-product value must not decrease across performance runs

### Acceptance Thresholds (Pilot-Grade)

| Metric | Threshold | Gate Source |
|--------|-----------|-------------|
| AMM Quote p95 | <= 300ms | SC-021 / `scenario-b-perf.js` |
| AMM Swap p95 | <= 6000ms | SC-022 / `scenario-b-perf.js` |
| Pool Status p95 | <= 15000ms | SC-023 / `scenario-b-perf.js` |
| Error Rate (steady state) | < 1% | `scenario-b-perf.js` |
| AMM Swap Throughput | >= 30 TPS sustained | Draft — not yet validated against measured devnet results |
| Resource Stability | No memory leaks or node crashes during 12-hour soak | — |

> **Note**: The AMM swap throughput target of 30 TPS is a draft value. It is set lower than the Scenario A token transfer target (50 TPS) due to AMM constant-product math overhead on each swap. This threshold must be validated against measured devnet results before being treated as a formal gate.

### k6 Baseline Execution

```bash
# Run performance baseline (requires hub devnet up + AUTH_TOKEN set)
make scenario-b.perf-baseline

# Manual equivalent:
API_GW_URL=http://localhost:3000 \
AUTH_TOKEN=<jwt> \
PAIR=BRL-USD \
DURATION=1m \
VUS=20 \
k6 run scenario-b/tests/performance/scenario-b-perf.js
```

The k6 script (`scenario-b/tests/performance/scenario-b-perf.js`) runs three concurrent scenarios: `quoteScenario` (20 VUs), `swapScenario` (5 VUs), and `poolScenario` (5 VUs). A delta > 20% above the gate thresholds must block merge per Decision 13.

### Tooling

- **API Load Generation**: `k6` — scripted HTTP load with JWT authentication and dynamic payload generation
- **Monitoring**: Prometheus (metrics scraping) + Grafana (dashboards and snapshot exports) — Planned

---

## Security and Abuse Testing Strategy

### API Security (OWASP Controls)

All endpoints defined in the Scenario B OpenAPI spec (`scenario-b/backend/services/api-gateway/docs/openapi.yaml` — the spec the gateway serves at `GET /openapi.yaml`) are subject to:

| Control | Test |
|---------|------|
| **AuthN / AuthZ / RBAC** | `COMMERCIAL_BANK` token accessing governance endpoints (`/api/v2/hub/pair-registry/propose`, AMM pause) must return 403; missing/expired/malformed JWTs must return 401 |
| **Input Validation** | Fuzzing AMM swap endpoint with negative amounts, amounts exceeding reserve, zero `max_amount_in`, non-existent pair identifiers; must return 400 rather than crashing |
| **Replay / Idempotency** | State-changing calls (`/api/v2/amm/swap/exact-output`, `/api/v2/hub/liquidity/commit`) submitted with duplicate `Idempotency-Key`; first call accepted, subsequent calls return 409 |
| **Rate Limiting** | High-frequency quote blasts to verify 429 triggers before backend saturation |

### Smart Contract Security and Invariants

- **AMM Access Control**: `setPause` restricted to `CENTRAL_BANK_ROLE` or `GOVERNANCE_ROLE`; unauthorized callers revert
- **AMM Constant-Product Invariant (Foundry Fuzz)**: after every swap, `reserveA * reserveB >= k_before`; `swapExactOutput` reverts when calculated `amountIn` exceeds `maxAmountIn`; circuit breaker blocks all state changes when paused
- **LiquidityCommitRegistry TTL**: commits that have not been matched within 72 hours must not be executable; expiry revert tested
- **PairRegistry Bilateral Approval**: a pair cannot transition to `ACTIVE` with only one-party confirmation; unilateral confirm reverts
- **SpokeBridge Authorization**: `lockAndMint` and `burnAndUnlock` require caller to be a verified participant; authorization checks tested via Foundry
- **ManualOracle RBAC**: `setRate` restricted to authorized oracle operator; unauthorized callers revert

### Resilience and "Fail Closed" Behavior

When the hub Besu node or ManualOracle is unresponsive, the payment-orchestrator must default to Fail Closed: return 503 or 500 to the initiator; no tokens are locked on spokes or minted on the hub without a corresponding confirmed receipt.

---

## CI/CD Quality Gates and Pipeline Strategy

> **Status: Pending.** CI/CD pipelines are not yet configured for this repository. The specifications below define the target architecture to be implemented. Tests can currently be executed manually following the instructions in each section of this plan.

### Pull Request (PR) Pipeline — Planned

To be triggered on every commit to an open PR targeting `main` or `develop`.

- **Scope**: static analysis, Foundry hub contract unit tests (`forge test`), Go unit tests (`go test ./...`), lightweight integration tests against local hub devnet
- **Time Budget**: <= 10 minutes
- **Pass Criteria**:
  - 100% pass rate on unit tests
  - Minimum 80% code coverage on core AMM, FXAgreement, LiquidityCommitRegistry business logic
  - Zero critical or high-severity vulnerabilities in hub contract bytecode
  - k6 performance gate: p95 <= gate threshold + 20% delta (Decision 13)
  - Failure blocks the PR from being merged

**Manual equivalent (current):**

```bash
# Smart contract unit tests
cd scenario-b/contracts && forge test -vv

# Go unit tests (all services)
cd scenario-b/backend && go test ./...
```

### Nightly E2E & Performance Pipeline — Planned

To be triggered every night against the persistent hub Testnet (staging) environment.

- **Scope**: full API-driven E2E suite (US1–US6), Cacti relay validation, performance baseline
- **Time Budget**: <= 2 hours
- **Pass Criteria**:
  - 100% pass rate on core functional E2E flows
  - AMM performance gates met (p95 within thresholds)
  - Automated generation and storage of the Standard Test Evidence Bundle

**Manual equivalent (current):**

```bash
# Full hub stack
cd scenario-b && make scenario-b.up

# E2E flows
bash tryouts/tryout-scenario-b-e2e.sh all
bash tryouts/tryout-cacti-interop.sh

# Performance baseline
make scenario-b.perf-baseline
```

---

## Evidence Bundle Format

Every E2E and integration pipeline run generates a standard evidence archive:

```
evidence-bundle-<run-id>/
├── execution_summary.json       # Machine-readable pass/fail per step
├── aggregated_traces.log        # X-Correlation-Id → txHash → blockNumber → network mapping
└── performance_report.html      # k6 HTML output (performance runs only)
```

**`execution_summary.json` schema:**

```json
{
  "passed": true,
  "total_duration_ms": 38000,
  "run_id": "uuid-v4",
  "scenario": "scenario-b",
  "results": [
    {
      "scenario_id": "E2E-B-03",
      "step": "amm-swap-exact-output",
      "http_status": 202,
      "tx_hash": "0x...",
      "block_number": 2051,
      "network": "hub-1337",
      "latency_ms": 420,
      "passed": true
    }
  ]
}
```

**`aggregated_traces.log` format:**

```
<X-Correlation-Id (UUIDv4)> | <txHash (bytes32)> | <blockNumber (uint256)> | <network (hub|spoke-a|spoke-b)>
```

This bundle is stored as a CI artifact and retained for technical validation by the LNET Technical Committee.

---

## Known Test Coverage Gaps

The following areas are identified as having incomplete or missing test coverage as of 2026-05-29. These gaps are tracked and will be addressed in subsequent implementation sprints.

| Gap Area | Contract / Service | Missing Tests | Priority |
|----------|--------------------|---------------|----------|
| AMM extreme fee values | `AutomatedMarketMaker.sol` | Edge cases with fee set to 0 or to the maximum allowed value; fee math correctness at boundaries | Medium |
| AMM reserve overflow protection | `AutomatedMarketMaker.sol` | Swap attempts where calculated `amountIn` would cause reserve overflow; must revert gracefully | High |
| PairRegistry pair removal / archival | `PairRegistry.sol` | Removing an active pair; archiving deactivated pairs; re-proposing an archived pair | Medium |
| FXAgreement concurrent settlement | `FXAgreement.sol` (hub) | Two concurrent `settle` calls on the same agreement; only one should succeed | High |
| LiquidityCommitRegistry garbage collection | `LiquidityCommitRegistry.sol` | Batch expiry of multiple expired commits; gas cost of cleanup; state consistency after cleanup | Medium |
| Integration: Cacti event detection | Cacti relay + hub | End-to-end test that Cacti watcher detects `CommitMatched` on hub and triggers spoke action | High |
| Integration: SpokeBridge on-chain | SpokeBridge API | Full integration test: API call → on-chain `Locked` event → W-tCeBM minted on hub | Medium |
| AMM throughput validation | k6 performance | 30 TPS target is draft; must be validated with measured devnet results under realistic load | Medium |
| Testnet environment | All services | No persistent staging environment exists yet; all tests run against ephemeral devnet | High |
| CI/CD pipelines | All | No automated pipeline; all tests executed manually | Medium |
