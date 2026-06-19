

**Deliverable 12**

**Phase I (Take-Off): A Regional Prototype for Accelerating the Deployment of Central Bank Digital Currencies (tCeBMs) for Inclusion in Latin America and the Caribbean**

**Project Code: RG-T4567**  
**Suboperation: ATN/KS-21330-RG**

**Authors: Lucas Campelo, Samuel Venzi**  
**Date: May 29th, 2026**

## **Summary** {#summary}

[**Summary	2**](#summary)

[**Introduction	4**](#introduction)

[Scenario A	4](#scenario-a)

[Scope	4](#scope)

[Test Goals	5](#test-goals)

[Test Execution Timeline and Responsibilities	6](#test-execution-timeline-and-responsibilities)

[Stakeholders	6](#stakeholders)

[Responsibility Matrix (RACI)	6](#responsibility-matrix-\(raci\))

[Execution Timeline	8](#execution-timeline)

[Phase Checklists	9](#phase-checklists)

[Phase 0 — Mobilization (Week 1\)	9](#phase-0-—-mobilization-\(week-1\))

[Phase 1 — Unit Testing (Week 2\)	9](#phase-1-—-unit-testing-\(week-2\))

[Phase 2 — Integration Testing (Week 3\)	10](#phase-2-—-integration-testing-\(week-3\))

[Phase 3 — E2E Core Flows (Week 4\)	10](#phase-3-—-e2e-core-flows-\(week-4\))

[Phase 4 — Performance & Security (Week 5\)	10](#phase-4-—-performance-&-security-\(week-5\))

[Phase 5 — User Acceptance Testing (Weeks 6–7)	11](#phase-5-—-user-acceptance-testing-\(weeks-6–7\))

[Phase 6 — Regression & Retest (Week 8\)	12](#phase-6-—-regression-&-retest-\(week-8\))

[Phase 7 — Sign-off & Submission (Week 9\)	12](#phase-7-—-sign-off-&-submission-\(week-9\))

[Testing Methodology	12](#testing-methodology)

[Test Levels	14](#test-levels)

[Unit Testing	14](#unit-testing)

[Integration Testing	14](#integration-testing)

[End-to-End (E2E) Testing	14](#end-to-end-\(e2e\)-testing)

[API-First E2E Execution Strategy	15](#api-first-e2e-execution-strategy)

[API-Driven E2E Flows	15](#api-driven-e2e-flows)

[On-Chain Verification and Finality Rules	16](#on-chain-verification-and-finality-rules)

[Traceability Map	17](#traceability-map)

[Test Environments and Reproducible Topology	18](#test-environments-and-reproducible-topology)

[Hub-and-Spoke Topology Assumptions (Scenario A)	18](#hub-and-spoke-topology-assumptions-\(scenario-a\))

[Component Versions (Validated SBOM)	19](#component-versions-\(validated-sbom\))

[Key Configuration for Tests	19](#key-configuration-for-tests)

[Devnet (Local Docker Compose)	19](#devnet-\(local-docker-compose\))

[Testnet (Staging)	20](#testnet-\(staging\))

[Readiness Checklist ("Environment Up" Criteria)	20](#readiness-checklist-\("environment-up"-criteria\))

[Performance Testing Strategy	20](#performance-testing-strategy)

[Workload Models	20](#workload-models)

[Target Metrics	21](#target-metrics)

[Acceptance Thresholds (Pilot-Grade)	21](#acceptance-thresholds-\(pilot-grade\))

[Tooling	22](#tooling)

[Security and Abuse Testing Strategy	22](#security-and-abuse-testing-strategy)

[API Security (OWASP Controls)	22](#api-security-\(owasp-controls\))

[Smart Contract Security and Invariants	23](#smart-contract-security-and-invariants)

[Resilience and "Fail Closed" Behavior	24](#resilience-and-"fail-closed"-behavior)

[Time Budgets, Finality, and Flakiness Controls	24](#time-budgets,-finality,-and-flakiness-controls)

[Time Budgets per Critical Segment	24](#time-budgets-per-critical-segment)

[Deterministic Finality Confirmation	24](#deterministic-finality-confirmation)

[Flakiness Controls	25](#flakiness-controls)

[Interoperability Resilience Scenarios	25](#interoperability-resilience-scenarios)

[CI/CD Quality Gates and Pipeline Strategy	26](#ci/cd-quality-gates-and-pipeline-strategy)

[Pull Request (PR) Pipeline — Implemented	26](#pull-request-\(pr\)-pipeline-—-planned)

[Nightly E2E & Resilience Pipeline — Planned	27](#nightly-e2e-&-resilience-pipeline-—-planned)

[Evidence Bundle Format	27](#evidence-bundle-format)

[Scenario B	29](#scenario-b)

[Scope	29](#scope-1)

[Test Goals	31](#test-goals-1)

[Test Execution Timeline and Responsibilities	32](#test-execution-timeline-and-responsibilities-1)

[Stakeholders	32](#stakeholders-1)

[Responsibility Matrix (RACI)	33](#responsibility-matrix-\(raci\)-1)

[Execution Timeline	34](#execution-timeline-1)

[Phase Checklists	36](#phase-checklists-1)

[Phase 0 — Mobilization (Week 1\)	36](#phase-0-—-mobilization-\(week-1\)-1)

[Phase 1 — Unit Testing (Week 2\)	36](#phase-1-—-unit-testing-\(week-2\)-1)

[Phase 2 — Integration Testing (Week 3\)	37](#phase-2-—-integration-testing-\(week-3\)-1)

[Phase 3 — E2E Core Flows (Week 4\)	37](#phase-3-—-e2e-core-flows-\(week-4\)-1)

[Phase 4 — Performance & Security (Week 5\)	37](#phase-4-—-performance-&-security-\(week-5\)-1)

[Phase 5 — User Acceptance Testing (Weeks 6–7)	38](#phase-5-—-user-acceptance-testing-\(weeks-6–7\)-1)

[Phase 6 — Regression & Retest (Week 8\)	39](#phase-6-—-regression-&-retest-\(week-8\)-1)

[Phase 7 — Sign-off & Submission (Week 9\)	39](#phase-7-—-sign-off-&-submission-\(week-9\)-1)

[Testing Methodology	39](#testing-methodology-1)

[Test Levels	40](#test-levels-1)

[Unit Testing	40](#unit-testing-1)

[Integration Testing	41](#integration-testing-1)

[End-to-End (E2E) Testing	42](#end-to-end-\(e2e\)-testing-1)

[API-First E2E Execution Strategy	43](#api-first-e2e-execution-strategy-1)

[Three Main E2E Flows	43](#three-main-e2e-flows)

[Flow 1 — Cooperative Liquidity Pair Formation (US1 / US2 / US5)	43](#flow-1-—-cooperative-liquidity-pair-formation-\(us1-/-us2-/-us5\))

[Flow 2 — Commercial Bank FX Swap via AMM (US3)	44](#flow-2-—-commercial-bank-fx-swap-via-amm-\(us3\))

[Flow 3 — SpokeBridge Round-Trip (US6)	45](#flow-3-—-spokebridge-round-trip-\(us6\))

[On-Chain Verification and Finality Rules	45](#on-chain-verification-and-finality-rules-1)

[Traceability Map	46](#traceability-map-1)

[Test Environments and Reproducible Topology	49](#test-environments-and-reproducible-topology-1)

[Hub-and-Spoke Topology (Scenario B)	49](#hub-and-spoke-topology-\(scenario-b\))

[Component Versions (Validated SBOM)	49](#component-versions-\(validated-sbom\)-1)

[Key Configuration for Tests	50](#key-configuration-for-tests-1)

[Devnet (Local Docker Compose)	50](#devnet-\(local-docker-compose\)-1)

[Testnet (Staging)	50](#testnet-\(staging\)-1)

[Readiness Checklist ("Environment Up" Criteria)	51](#readiness-checklist-\("environment-up"-criteria\)-1)

[Performance Testing Strategy	51](#performance-testing-strategy-1)

[Workload Models	51](#workload-models-1)

[Target Metrics	52](#target-metrics-1)

[Acceptance Thresholds (Pilot-Grade)	52](#acceptance-thresholds-\(pilot-grade\)-1)

[k6 Baseline Execution	53](#k6-baseline-execution)

[Tooling	54](#tooling-1)

[Security and Abuse Testing Strategy	54](#security-and-abuse-testing-strategy-1)

[API Security (OWASP Controls)	54](#api-security-\(owasp-controls\)-1)

[Smart Contract Security and Invariants	55](#smart-contract-security-and-invariants-1)

[Resilience and "Fail Closed" Behavior	55](#resilience-and-"fail-closed"-behavior-1)

[CI/CD Quality Gates and Pipeline Strategy	56](#ci/cd-quality-gates-and-pipeline-strategy-1)

[Pull Request (PR) Pipeline — Implemented	56](#pull-request-\(pr\)-pipeline-—-planned-1)

[Nightly E2E & Performance Pipeline — Planned	56](#nightly-e2e-&-performance-pipeline-—-planned)

[Evidence Bundle Format	57](#evidence-bundle-format-1)

[Known Test Coverage Gaps	59](#known-test-coverage-gaps)

# **Introduction** {#introduction}

This document is a more formalized version of the technical documentation and runbooks available in the official GitHub repository, which can be found at: [https://github.com/LACNetNetworks/cbweb3-platform](https://github.com/LACNetNetworks/cbweb3-platform).

## **Scenario A** {#scenario-a}

This Test Execution Plan establishes the formal quality assurance strategy for the CBWeb3 platform — a regional prototype for wholesale CBDC settlement between jurisdictions in Latin America and the Caribbean. The plan validates functional and non-functional integrity across the three-tier testing hierarchy (unit, integration, end-to-end), covering the hub-and-spoke DLT architecture, atomic cross-spoke settlement via HTLC, the Paladin/Zeto privacy layer, and the Hyperledger Cacti relay.

## **Scope** {#scope}

The following components and layers are explicitly **in scope**:

- **Smart Contracts**: HTLC escrow state machine, tCeBM/fCeBM token logic, IdentityRegistry on-chain RBAC, FXAgreement lifecycle, SpokeBridge, AutomatedMarketMaker  
- **Backend API Services**: api-gateway (REST), auth (gRPC), compliance (gRPC), payment-orchestrator (gRPC) — all 6 entities  
- **Escrow Flow**: full deposit → fCeBM mint → escrow (fCeBM burn \+ tCeBM Zeto mint) → redeem (tCeBM Zeto transfer \+ fCeBM mint) lifecycle  
- **FX Agreement Lifecycle**: PROPOSED → ACCEPTED → SETTLED / REJECTED / CANCELLED  
- **HTLC Cross-Spoke Settlement**: bilateral atomic swap between Spoke-A (chain 1338\) and Spoke-B (chain 1339\) via Cacti relay  
- **Cacti Relay**: automatic secret extraction and cross-spoke submission  
- **Paladin / Zeto**: ZKP-shielded token transfers, Pente bilateral context for FX Agreement  
- **Compliance Layer**: AML/CFT screening, participant onboarding (KYC), account freeze/unfreeze  
- **Test Environments**: Devnet (local Docker Compose) and Testnet (staging)

The following are explicitly **out of scope**:

- Hyperledger Besu core QBFT consensus protocol (validated by upstream maintainers)  
- Frontend UI pixel-level testing (covered by API-First E2E which validates backend state consistency)  
- Paladin node internals beyond the API surface exposed to the platform

## **Test Goals** {#test-goals}

| Goal | Metric |
| :---- | :---- |
| **Logic Validation** | ≥ 80% code coverage on core smart contract business logic (HTLC state machine, tCeBM mint/burn, FXAgreement state transitions) |
| **Data Integrity** | Integration tests verify that data flows correctly across API → gRPC → Besu/Paladin; exceptions (insufficient balance, AML block, expired timeLock) handled without state inconsistency |
| **Cross-Spoke Atomicity** | E2E HTLC flow completes fully (both legs settled) or reverts fully (both legs refunded) with no orphaned escrow |
| **Compliance Enforcement** | Sanctioned/frozen accounts are blocked at the API layer before any on-chain interaction |
| **Relay Resilience** | Cacti relay recovers from downtime, sweeping missed events and completing pending cross-spoke locks |
| **Performance Baseline** | System meets pilot-grade thresholds under steady-state and burst load |

## **Execution Results (As-Run — 2026-06-19)** {#execution-results-scenario-a}

The following are measured outcomes from execution on branch `test/r1-12.3-perf-harness`. Full per-step evidence is in the repository: `D12_results.md`, `docs/TEST-REPORTS.md`, `scenario-a/docs/TEST-REPORT.md`, the machine-readable bundles under `evidence-bundles/`, and `scenario-a/docs/performance/`.

**Unit & coverage (D6 ≥ 80% core gate) — PASS.**

| Layer | Result |
| :---- | :---- |
| Backend (`go test`, per-service core gate) | payment-orchestrator 82.0%, compliance 90.9%, auth 85.6%, shared/identity 80.5% — all ≥ 80% |
| Smart contracts (Foundry, 256 fuzz runs) | 183 tests, 0 failures; 97.95% lines / 98.46% branches / 100% functions |

**End-to-end (live stack, `make scenario-a.test-integration` → `TestFullHappyPath`) — PASS (56.1 s).** Eight phases: readiness, login, mint (treasury), FX propose, cross-spoke sync (relay mirror → custodian accept → ACCEPTED both spokes), dual-leg HTLC lock (originator + custodian; timelock invariant), settle (secret reveal on Spoke-A + relay settles Spoke-B), verify (both legs SETTLED). Onboarding is opt-in and was skipped.

**Performance (R1-12.3, k6; local + AWS EC2 c6a.8xlarge confirmation):**

| Measurement | Target | Measured | Verdict |
| :---- | :---- | :---- | :---- |
| Full HTLC lifecycle (D6) | ≤ 60 s | p95 30.6 s (local) / 32.9 s (EC2) | **PASS** |
| Synchronous API response (D6) | ≤ 30 s | p95 7.1 s | **PASS** |
| Relayer cross-chain (D6) | ≤ 15 s | p95 3.8 s | **PASS** |
| Settlement completion | > 90% | 100% | **PASS** |
| Zeto privacy throughput | 15 TPS | 15.0 TPS, 0% err | **PASS** |
| API read latency | p95 < 500 ms | 36–81 ms | **PASS** |
| HTLC lock throughput | 50 TPS | 2.9–4.4 TPS | **FAIL** |
| API write latency | p95 < 1500 ms | p95 60 s (median 7.2 s) | **FAIL** |
| Time-to-finality | p95 < 5 s | no samples (n=0) | n/a |

*Justification — HTLC lock 50 TPS and API write latency (FAIL):* `/htlc/lock` is a synchronous, blocking operation that drives a Paladin/Zeto private lock — a zero-knowledge UTXO state transition (~5 s each) — and waits for the receipt before returning. Under concurrency the privacy domain serialises and saturates; the dominant failure is `wait lock receipt: context deadline exceeded`. Re-running the identical benchmark on a 32-vCPU AWS c6a.8xlarge produced the same ~3 TPS and 60 s write p95 with ~0 nonce errors — confirming the ceiling is the latency-bound privacy lock path, **not** CPU or signer concurrency. Remediation is architectural: asynchronous admission (return on submit; confirm via the relay/event machinery). TTF captured 0 samples because the under-load lock benchmark dropped its iterations; per-spoke finality is independently within budget (relayer p95 3.8 s; QBFT single-block finality).

## **Test Execution Timeline and Responsibilities** {#test-execution-timeline-and-responsibilities}

### **Stakeholders** {#stakeholders}

| Stakeholder | Role |
| :---- | :---- |
| **LNET** | Platform developer and integrator — responsible for smart contract, API, infrastructure, tooling delivery, and test execution |
| **Banks** | Participant institutions (bank-a, bank-b, bank-c, bank-d, central-bank-a, central-bank-b) — responsible for UAT validation and operational sign-off |

### **Responsibility Matrix (RACI)** {#responsibility-matrix-(raci)}

| Phase | LNET | Banks | Notes |
| :---- | :---- | :---- | :---- |
| Test Infrastructure Setup | **R / A** | C | LNET deploys devnet and testnet; Banks provide test account data |
| Smart Contract Unit Tests | **R / A** | I | Foundry suite; no bank involvement required |
| Backend API Unit Tests | **R / A** | I | Go unit suite; all dependencies mocked |
| Integration Tests | **R / A** | C | Live Besu/Paladin; Banks consulted for acceptance criteria |
| E2E Core Flows | **R / A** | **R** | LNET executes flows; Banks verify outcomes via portal and API |
| Performance & Security | **R / A** | I | k6/Caliper/ZAP; Banks informed of final results |
| User Acceptance Testing (UAT) | C / **A** | **R** | Banks lead UAT; LNET provides support and resolves defects within 24 h |
| Issue Resolution & Retest | **R / A** | **R** | LNET fixes defects; Banks confirm retest |
| Evidence Bundle & Sign-off | **R / A** | **R** | Joint sign-off required for Deliverable 12 submission |

**R** \= Responsible (executes the work) · **A** \= Accountable (owns the outcome) · **C** \= Consulted (provides input) · **I** \= Informed (notified of results)

### **Execution Timeline** {#execution-timeline}

| Phase | Duration | Window | Responsible | Key Output |
| :---- | :---- | :---- | :---- | :---- |
| **Phase 0 — Mobilization** | 1 week | Week 1 | LNET | Devnet up, test data seeded, accounts provisioned |
| **Phase 1 — Unit Testing** | 1 week | Week 2 | LNET | Forge \+ Go test results; ≥ 80 % coverage report |
| **Phase 2 — Integration Testing** | 1 week | Week 3 | LNET | Integration test results; blockchain connectivity confirmed |
| **Phase 3 — E2E Core Flows** | 1 week | Week 4 | LNET \+ Banks | E2E evidence bundle (HTLC, escrow, compliance) |
| **Phase 4 — Performance & Security** | 1 week | Week 5 | LNET | k6/Caliper report; ZAP scan; acceptance thresholds validated |
| **Phase 5 — User Acceptance Testing** | 2 weeks | Weeks 6–7 | Banks (LNET support) | UAT sign-off report; defect log |
| **Phase 6 — Regression & Retest** | 1 week | Week 8 | LNET \+ Banks | Closed defect list; final evidence bundle |
| **Phase 7 — Sign-off & Submission** | 3 days | Week 9 | LNET \+ Banks | Signed test report; Deliverable 12 submitted |

Total: **\~9 weeks** from mobilization to submission. Actual calendar dates are to be agreed with the IDB Technical Committee.

### **Phase Checklists** {#phase-checklists}

#### **Phase 0 — Mobilization (Week 1\)** {#phase-0-—-mobilization-(week-1)}

**LNET:** deploy test environment · **Banks:** confirm UAT participants and schedule

- [ ] Devnet deployed (`make spoke-all`)  
- [ ] All 6 API gateways healthy (`/healthz` 200 OK)  
- [ ] Test participant accounts provisioned and registered in IdentityRegistry with `Verified` status  
- [ ] tCeBM initial balances seeded per entity  
- [ ] Test tooling installed (Foundry, Go ≥ 1.26, k6, OWASP ZAP)  
- [ ] Banks identify UAT testers per portal role

#### **Phase 1 — Unit Testing (Week 2\)** {#phase-1-—-unit-testing-(week-2)}

**LNET:** execute · **Banks:** informed

- [ ] `cd scenario-a/contracts && forge test -v` — all Foundry tests pass  
- [ ] `cd scenario-a/backend && go test ./...` — all Go unit tests pass  
- [ ] Coverage report generated; ≥ 80 % on HTLC, tCeBM, FXAgreement, IdentityRegistry  
- [ ] Foundry fuzz tests executed (256 runs per invariant; `FOUNDRY_FUZZ_RUNS=256`)  
- [ ] Results documented in evidence bundle

#### **Phase 2 — Integration Testing (Week 3\)** {#phase-2-—-integration-testing-(week-3)}

**LNET:** execute · **Banks:** consulted on acceptance criteria

- [ ] API ↔ Besu integration confirmed (real transaction broadcast \+ receipt)  
- [ ] Paladin/Zeto ZKP generation and verification confirmed  
- [ ] gRPC chain (api-gateway → payment-orchestrator → besu client) validated end-to-end  
- [ ] AML blocking at API layer confirmed (sanctioned address rejected before on-chain call)  
- [ ] Results documented

#### **Phase 3 — E2E Core Flows (Week 4\)** {#phase-3-—-e2e-core-flows-(week-4)}

**LNET:** execute flows · **Banks:** verify results via API responses and portal

- [ ] `tryout-spoke-a-bank-a.sh` — Spoke-A onboarding \+ lifecycle  
- [ ] `tryout-spoke-b-bank-b.sh` — Spoke-B onboarding \+ lifecycle  
- [ ] `tryout-fx-agreement-e2e.sh` — Full HTLC cross-spoke settlement (happy path \+ timeout refund)  
- [ ] `tryout-escrow-flow.sh` — Full deposit → escrow → redeem  
- [ ] `tryout-cacti-interop.sh` — Cacti relay health and event propagation  
- [ ] `tryout-compliance-participants.sh` — Compliance and AML screening  
- [ ] Interoperability resilience scenarios: relay downtime, network partition, duplicate messages  
- [ ] Evidence bundle generated per flow

#### **Phase 4 — Performance & Security (Week 5\)** {#phase-4-—-performance-&-security-(week-5)}

**LNET:** execute · **Banks:** informed of results

- [ ] k6 steady-state load: ≥ 50 TPS token transfers confirmed  
- [ ] k6 burst/spike profile executed  
- [ ] Caliper smart contract benchmarks: TTF \< 5 s, error rate \< 1 %  
- [ ] OWASP ZAP baseline scan: zero critical or high-severity findings  
- [ ] Foundry fuzz security invariants executed (HTLC, AMM)  
- [ ] Performance report (k6 HTML \+ Caliper JSON) archived

#### **Phase 5 — User Acceptance Testing (Weeks 6–7)** {#phase-5-—-user-acceptance-testing-(weeks-6–7)}

**Banks:** execute · **LNET:** support with 24 h defect resolution SLA

| Entity | Portal | UAT Focus |
| :---- | :---- | :---- |
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

#### **Phase 6 — Regression & Retest (Week 8\)** {#phase-6-—-regression-&-retest-(week-8)}

**LNET:** fix defects · **Banks:** confirm resolution

- [ ] All critical and major defects from UAT closed  
- [ ] Regression E2E suite re-executed after each fix batch  
- [ ] Banks confirm fixed defects via targeted retest  
- [ ] Final evidence bundle assembled and validated

#### **Phase 7 — Sign-off & Submission (Week 9\)** {#phase-7-—-sign-off-&-submission-(week-9)}

**LNET \+ Banks:** joint

- [ ] LNET produces final Test Report with complete evidence bundle  
- [ ] Banks sign UAT acceptance form  
- [ ] Deliverable 12 package submitted to IDB Technical Committee

## **Testing Methodology** {#testing-methodology}

| Layer | Framework | Scope |
| :---- | :---- | :---- |
| **Smart Contracts** | [Foundry](https://getfoundry.sh/) (`forge test`) | Solidity unit tests with full EVM isolation; fuzz tests for HTLC and AMM invariants |
| **Backend API** | Go standard `testing` package | Unit tests with mocked gRPC and blockchain dependencies; isolated per-service |
| **Integration** | Go `testing` \+ local Besu/Paladin testnet | Live infrastructure; tests sign real transactions and verify on-chain state |
| **E2E** | Bash scripts (`tryout-*.sh`) \+ API-First runner | Full lifecycle execution via REST API; on-chain finality confirmed via `eth_getTransactionReceipt` |
| **Performance** | `k6` (API load) | Threshold/baseline and D6 HTLC-lifecycle profiles executed; burst, stress, soak, and `Hyperledger Caliper` smart-contract benchmarking are planned |
| **Security** | Foundry fuzz \+ manual abuse testing (OWASP ZAP — planned) | RBAC escalation, input fuzzing, idempotency, rate limiting |

**Evaluation criteria (binary):**

- **Pass**: actual result matches expected result exactly; no side effects on ledger or privacy layer; execution within time budget  
- **Fail**: result differs, system crashes, test times out, or execution cannot reach a terminal state

## **Test Levels** {#test-levels}

### **Unit Testing** {#unit-testing}

Validates discrete, isolated functions with all external dependencies mocked.

- **Smart Contracts**: function-level tests via Foundry (EVM state isolated per test); covers HTLC lock/settle/refund, tCeBM mint/burn, FXAgreement lifecycle, IdentityRegistry RBAC, AMM constant-product formula  
- **Backend API**: Go unit tests; database connections, blockchain clients, and gRPC channels are substituted with mocks; covers request handlers, middleware (JWT validation, RBAC), routing, and service-layer logic

### **Integration Testing** {#integration-testing}

Validates coordination between components with live local infrastructure.

- **Blockchain Connectivity**: API signs real transactions against a local Besu node; verifies transaction broadcast, event log parsing, and async state updates  
- **Paladin Privacy Layer**: compliance service connects to a local Paladin node; validates ZKP generation/verification for Zeto transfers  
- **gRPC Orchestration**: api-gateway → payment-orchestrator → besu client chain validated end-to-end with real gRPC calls

### **End-to-End (E2E) Testing** {#end-to-end-(e2e)-testing}

Validates complete user journeys via the REST API against a full running stack (`make spoke-all`).

All E2E tests follow the **API-First** approach: no UI interaction; assertions are made against both the API response and direct on-chain state via `eth_getLogs` and contract read functions.

Existing E2E scripts (in `scenario-a/tryouts/`):

| Script | Flow |
| :---- | :---- |
| `tryout-spoke-a-bank-a.sh` | Onboarding \+ token lifecycle (bank-a, Spoke-A) |
| `tryout-spoke-a-bank-c.sh` | Onboarding \+ token lifecycle (bank-c, Spoke-A) |
| `tryout-spoke-b-bank-b.sh` | Onboarding \+ token lifecycle (bank-b, Spoke-B) |
| `tryout-spoke-b-bank-d.sh` | Onboarding \+ token lifecycle (bank-d, Spoke-B) |
| `tryout-fx-agreement-e2e.sh` | Full FX Agreement \+ HTLC cross-spoke settlement |
| `tryout-escrow-flow.sh` | Full deposit → escrow → redeem lifecycle |
| `tryout-cacti-interop.sh` | Cacti relay health \+ PluginLedgerConnectorBesu validation |
| `tryout-compliance-participants.sh` | Participant compliance screening |
| `tryout-internal-relay-auth.sh` | Internal relay authentication |
| `tryout-my-onboarding-status.sh` | Onboarding status polling |

## **API-First E2E Execution Strategy** {#api-first-e2e-execution-strategy}

### **API-Driven E2E Flows** {#api-driven-e2e-flows}

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
3. CB approves deposit \+ mints fCeBM  
4. Request escrow: `POST /internal/v1/payments/escrows` (burns fCeBM, mints tCeBM via Zeto)  
5. CB approves escrow  
6. Request redeem: `POST /internal/v1/payments/redeems` (Zeto transfer \+ fCeBM mint)  
7. CB approves redeem  
8. Assert final balances (fCeBM restored, tCeBM zero)

### **On-Chain Verification and Finality Rules** {#on-chain-verification-and-finality-rules}

After each API call that broadcasts a transaction:

1. **Event Parsing**: query Besu RPC (`eth_getLogs`) to verify expected contract events were emitted with correct parameters  
2. **Finality Confirmation**: poll `eth_getTransactionReceipt` every 1 second; assert finality once `receipt.blockNumber + 1` is reached (QBFT \= 1-block finality)  
3. **State Verification**: direct contract read calls confirm state transitions match expected output

## **Traceability Map** {#traceability-map}

| Use Case | Test ID(s) | Type | Requirement | Module | Status |
| :---- | :---- | :---- | :---- | :---- | :---- |
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

## **Test Environments and Reproducible Topology** {#test-environments-and-reproducible-topology}

### **Hub-and-Spoke Topology Assumptions (Scenario A)** {#hub-and-spoke-topology-assumptions-(scenario-a)}

| Network | Chain ID | Entities | Validator Nodes |
| :---- | :---- | :---- | :---- |
| Spoke-A | 1338 | central-bank-a, bank-a, bank-c | 3 (QBFT) |
| Spoke-B | 1339 | central-bank-b, bank-b, bank-d | 3 (QBFT) |

The Hyperledger Cacti relay operates between Spoke-A and Spoke-B, subscribing to HTLC events and executing automatic cross-spoke settlement. There is no hub network in Scenario A.

### **Component Versions (Validated SBOM)** {#component-versions-(validated-sbom)}

| Component | Version |
| :---- | :---- |
| Hyperledger Besu | v25.8.0 (QBFT) |
| Hyperledger Cacti | v2.1.0 |
| Paladin | v0.15 |
| Zeto | v0.2.2 |
| Go backend APIs | 1.26.0 |
| Solidity | ^0.8.20 |
| OpenZeppelin | v5.6.0 |

### **Key Configuration for Tests** {#key-configuration-for-tests}

- **Block time**: 2s intervals; QBFT absolute finality in 1 block  
- **Chain IDs**: Spoke-A (1338), Spoke-B (1339), Hub/Scenario-B (1337 — planned)  
- **API Gateway timeouts**: 30 seconds  
- **Relay cross-chain propagation timeout**: 15 seconds  
- **HTLC time-locks in tests**: shortened to 600 seconds (10 min) for timeout/refund scenarios

### **Devnet (Local Docker Compose)** {#devnet-(local-docker-compose)}

- **Purpose**: CI/CD pipeline execution, unit/integration tests, rapid iteration  
- **Persistence**: Ephemeral — spun up fresh per test run via `make spoke-all`  
- **Seeding**: initialization scripts register test participants, mint initial tCeBM balances  
- **Teardown**: `make spoke-all-down` destroys all state; `make deploy.down-infra` removes volumes  
- **Readiness criteria**: all 6 API gateways return 200 on `/healthz`; Besu RPC reports blocks advancing; Cacti relay health endpoint 200 OK

### **Testnet (Staging)** {#testnet-(staging)}

- **Purpose**: persistent E2E, performance benchmarking, stakeholder demonstration  
- **Persistence**: long-lived multi-node deployment  
- **Reset**: snapshot-restore to known golden state after destructive tests  
- **Seeding**: predefined subset of static test accounts

### **Readiness Checklist ("Environment Up" Criteria)** {#readiness-checklist-("environment-up"-criteria)}

Before any integration, E2E, or performance suite may execute:

1. Besu RPC endpoints return `eth_syncing = false` and block heights are advancing on both spokes  
2. Cacti relay returns 200 OK health with active WebSocket subscriptions to both spoke networks  
3. `GET /healthz` returns 200 OK on all 6 API gateways  
4. Contract addresses are present in `deploy/local/paladin/spoke-{a,b}/.deployed-addrs.env`  
5. All 6 entities are registered in the IdentityRegistry with `Verified` status

## **Performance Testing Strategy** {#performance-testing-strategy}

### **Workload Models** {#workload-models}

| Profile | Description |
| :---- | :---- |
| **Steady State (Load)** | Expected average traffic over a standard operational window; establishes baseline |
| **Burst (Spike)** | Sharp transaction volume increase; validates rate limiting and queue resilience |
| **Stress** | Gradually increases load until degradation; identifies maximum TPS |
| **Soak** | Moderate continuous load over 12 hours; identifies memory leaks and state bloat |

### **Target Metrics** {#target-metrics}

- **Throughput (TPS)**: successfully finalized on-chain transactions per second  
- **Time-to-Finality (TTF)**: broadcast to irreversible block confirmation (Besu QBFT)  
- **API Latency**: p50 and p95 for both read (query) and state-changing operations  
- **Error Rate**: failed HTTP requests (5xx, timeouts, contract reverts) vs. total  
- **Resource Utilization**: CPU, Memory, Disk I/O across API pods, Cacti relay, and Besu validator nodes

### **Acceptance Thresholds (Pilot-Grade)** {#acceptance-thresholds-(pilot-grade)}

| Metric | Threshold |
| :---- | :---- |
| Throughput — token transfers | ≥ 50 TPS sustained |
| Throughput — Zeto shielded transactions | ≥ 15 TPS sustained |
| Time-to-Finality | \< 5 seconds per spoke |
| API Latency — read operations | p50 \< 200ms, p95 \< 500ms |
| API Latency — state-changing operations | p50 \< 800ms, p95 \< 1500ms (before on-chain finality) |
| Error Rate (steady state) | \< 1% |
| Resource Stability | No memory leaks or node crashes during 12-hour soak |

### **Tooling** {#tooling}

- **API Load Generation**: `k6` — scripted HTTP load with JWT authentication and dynamic payload generation  
- **Smart Contract Benchmarking**: `Hyperledger Caliper` — direct RPC-level load against Besu nodes  
- **Monitoring**: `Prometheus` (metrics scraping) \+ `Grafana` (dashboards and snapshot exports)

## **Security and Abuse Testing Strategy** {#security-and-abuse-testing-strategy}

### **API Security (OWASP Controls)** {#api-security-(owasp-controls)}

All endpoints defined in the OpenAPI spec (`scenario-a/apis/openapi/api-gateway.yaml`) are subject to:

| Control | Test |
| :---- | :---- |
| **AuthN / AuthZ / RBAC** | Privilege escalation attempts: `ROLE_COMMERCIAL_BANK` token accessing `/api/v1/governance/approve-kyc` or relay endpoints; missing/expired/malformed JWTs must return 401; insufficient roles must return 403 |
| **Input Validation** | Fuzzing with malformed JSON, boundary-exceeding numerics, SQL/NoSQL injection vectors; must return 400 rather than crashing |
| **Replay / Idempotency** | State-changing calls (`/api/v1/htlc/lock`, `/api/v1/payments/fx/agreements`) submitted with duplicate `Idempotency-Key`; first call accepted, subsequent calls return 409 |
| **Rate Limiting** | High-frequency blasts to verify 429 triggers before backend saturation |
| **DAST** | OWASP ZAP baseline scan against Testnet API gateways on each nightly pipeline run |

### **Smart Contract Security and Invariants** {#smart-contract-security-and-invariants}

- **Access Control**: administrative functions restricted to `GOVERNANCE_ROLE` / `CENTRAL_BANK_ROLE` (OpenZeppelin `AccessControl`)  
- **HTLC Invariants (Foundry Fuzz)**: funds cannot be claimed without the exact pre-image; refund cannot execute before `timeLock` expiry; `INVALID → LOCKED → SETTLED/REFUNDED` FSM cannot be violated  
- **AMM Invariants (Foundry Fuzz)**: constant-product formula `x · y = k` is maintained after every swap; `swapExactOutput` reverts when calculated input exceeds `maxInput`; circuit breaker blocks all state changes when paused  
- **Pause / Circuit Breaker**: `GOVERNANCE_ROLE` can pause; unauthorized callers receive 403; all state-changing operations revert with `"Contract Paused"` while paused

### **Resilience and "Fail Closed" Behavior** {#resilience-and-"fail-closed"-behavior}

When Paladin ZKP verifier or external compliance oracle is unresponsive, the payment-orchestrator must default to Fail Closed: return 503 or 500 to the initiator; no state changes or locked funds are orphaned on-chain.

## **Time Budgets, Finality, and Flakiness Controls** {#time-budgets,-finality,-and-flakiness-controls}

### **Time Budgets per Critical Segment** {#time-budgets-per-critical-segment}

| Segment | Budget |
| :---- | :---- |
| API Synchronous Timeout | 30 seconds |
| On-Chain Transaction Mining (local node) | 10 seconds |
| Cacti Relay Cross-Chain Propagation (lock → counter-lock) | 15 seconds |
| Full HTLC Settlement Lifecycle (FX Agreement → final settlement) | 60 seconds |

### **Deterministic Finality Confirmation** {#deterministic-finality-confirmation}

E2E and integration test runners must not use arbitrary `sleep`. Instead:

1. Receive transaction hash from the API response  
2. Poll `eth_getTransactionReceipt` every 1 second (maximum: time budget ÷ 1s attempts)  
3. Assert finality once current block height ≥ `receipt.blockNumber + 1`  
4. Proceed to next assertion step

QBFT consensus provides absolute single-block finality; no chain reorganizations occur under normal conditions.

### **Flakiness Controls** {#flakiness-controls}

- **Deterministic Seeding**: test accounts are logically partitioned; concurrent test threads use disjoint wallet sets to avoid EVM nonce collisions  
- **Dynamic Identifiers**: every test run generates unique UUIDs for `agreementId`, `Idempotency-Key`, and `X-Correlation-Id`; hardcoded IDs are prohibited  
- **Idempotent Cleanup**: on persistent Testnet, tests must gracefully conclude their lifecycle (trigger `/api/v1/htlc/refund` if a lock times out) before the runner exits

## **Interoperability Resilience Scenarios** {#interoperability-resilience-scenarios}

| Scenario | Description | Expected Outcome |
| :---- | :---- | :---- |
| **Relay Downtime & Recovery** | Cacti relay is terminated after HTLC lock is confirmed on Spoke-A but before cross-chain submission to Spoke-B; relay is restarted after 60 seconds | Relay sweeps blockchain for missed events, recovers state, and successfully executes the delayed counter-lock |
| **Delayed / Duplicated Messages** | Same cross-chain proof submitted multiple times concurrently to the destination contract | First valid transaction accepted; subsequent duplicates revert with `"Already Settled"` |
| **Network Partitioning** | Connection between Spoke-A and Spoke-B simulated as severed; in-flight HTLC nears `timeLock` expiry | Initiator can call `refund` after expiry; funds return to sender; no orphaned escrow |
| **Spoke Node Restart** | One Besu validator node restarted mid-HTLC lifecycle | QBFT consensus continues with remaining validators; transaction is re-broadcast and mined after reconnection |

## **CI/CD Quality Gates and Pipeline Strategy** {#ci/cd-quality-gates-and-pipeline-strategy}

**Status: PR pipeline implemented; nightly E2E pipeline planned.** GitHub Actions workflows (`.github/workflows/backend-scenario-a.yml`, `contracts-scenario-a.yml`) run on every PR and enforce the unit, coverage, and contract gates described below. The nightly E2E & resilience pipeline against a persistent testnet is not yet configured; those flows are currently executed manually (and were executed manually for this cycle — see [Execution Results](#execution-results-scenario-a)).

### **Pull Request (PR) Pipeline — Implemented** {#pull-request-(pr)-pipeline-—-planned}

Triggered on every PR via GitHub Actions (`backend-scenario-a.yml`, `contracts-scenario-a.yml`).

- **Scope**: Foundry contract suite (`forge fmt --check` → build → test → coverage); Go per-service unit tests with the D6 coverage gate (matrix per service) plus `shared/identity`; `go vet`; `gosec` security scan; the hermetic `integration_lite` tier  
- **Time Budget**: ≤ 10 minutes  
- **Pass Criteria**:  
  - 100% pass rate on unit tests  
  - ≥ 80% code coverage on core business logic (D6 gate; the job fails below threshold)  
  - `gosec` reports no critical findings  
  - Failure blocks the PR from being merged  

Static analysis via SonarQube and smart-contract-bytecode vulnerability scanning remain planned additions.

**Manual equivalent (current):**

```shell
# Smart contract unit tests
cd scenario-a/contracts && forge test -v

# Go unit tests (all services)
cd scenario-a/backend && go test ./...
```

### **Nightly E2E & Resilience Pipeline — Planned** {#nightly-e2e-&-resilience-pipeline-—-planned}

To be triggered every night against the persistent Testnet (staging) environment.

- **Scope**: full API-driven E2E suite (Scenario A), Interoperability Resilience tests, Security/Abuse controls (OWASP ZAP)  
- **Time Budget**: ≤ 2 hours  
- **Pass Criteria**:  
  - 100% pass rate on core functional E2E flows  
  - Successful completion of relay recovery resilience scenario  
  - Automated generation and storage of the Standard Test Evidence Bundle

**Manual equivalent (current):**

```shell
# Full backend stack
cd scenario-a && make spoke-all

# E2E flows
./tryouts/tryout-fx-agreement-e2e.sh
./tryouts/tryout-escrow-flow.sh
./tryouts/tryout-cacti-interop.sh
./tryouts/tryout-spoke-a-bank-a.sh
./tryouts/tryout-spoke-b-bank-b.sh
```

## **Evidence Bundle Format** {#evidence-bundle-format}

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

## **Scenario B** {#scenario-b}

This Test Execution Plan establishes the formal quality assurance strategy for Scenario B of the CBWeb3 platform — the AMM-based hub network extension that enables cross-currency wholesale CBDC settlement between jurisdictions. Scenario B introduces a shared International Hub network (chain 1337\) operated by a consortium of Central Banks, an Automated Market Maker (AMM) for FX liquidity provisioning, a Multilateral Liquidity Provider (MLP), a PairRegistry and LiquidityCommitRegistry for cooperative pair formation, and a SpokeBridge for locking and minting hub-wrapped tokens (W-tCeBM).

This plan validates functional and non-functional integrity across the three-tier testing hierarchy (unit, integration, end-to-end), covering hub smart contracts, backend API services for all entities (bank-a, bank-b, central-bank-a, central-bank-b, MLP), the Hyperledger Cacti relay (CommitMatched event detection and cross-spoke relay), and the AMM FX swap lifecycle.

## **Scope** {#scope-1}

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

## **Test Goals** {#test-goals-1}

| Goal | Metric |
| :---- | :---- |
| **Logic Validation** | \>= 80% code coverage on core hub smart contract business logic (AMM constant-product formula, FXAgreement state machine, LiquidityCommitRegistry TTL, PairRegistry bilateral approval) |
| **Quote Accuracy** | AMM `getAmountIn` returns a value consistent with the constant-product formula `x * y = k` across all valid reserve states; assertion tolerance \= 0 wei (exact math) |
| **Swap Atomicity** | Exact-output swap either completes fully (exact token output delivered, correct input deducted, pool reserves updated) or reverts fully (no tokens transferred, no reserve change) |
| **AMM Invariant Preservation** | After every swap and every liquidity operation, the product `reserveA * reserveB >= k_before` holds; enforced via Foundry fuzz tests |
| **Bridge Round-Trip Integrity** | lockAndMint and burnAndUnlock are symmetric: locked spoke balance equals minted hub W-tCeBM; burned hub W-tCeBM equals unlocked spoke balance; no token creation or destruction |
| **Compliance Enforcement** | Sanctioned or unverified participants are blocked at the API layer before any on-chain hub interaction |
| **Relay Event Detection** | Cacti relay successfully subscribes to CommitMatched events on the hub; detected events trigger correct cross-spoke action within the relay propagation time budget |
| **Performance Baseline** | AMM API meets pilot-grade thresholds: quote p95 \<= 300ms, swap p95 \<= 6000ms, pool status p95 \<= 15000ms, error rate \< 1% |

## **Execution Results (As-Run — 2026-06-18/19)** {#execution-results-scenario-b}

The following are measured outcomes from execution on branch `test/r1-12.3-perf-harness`. Full evidence: `D12_results.md`, `scenario-b/docs/TEST-REPORT.md`, the machine-readable bundles under `evidence-bundles/`, and `scenario-b/docs/performance/RESULTS.md`.

**Unit & coverage (D6 ≥ 80% core gate) — PASS.**

| Layer | Result |
| :---- | :---- |
| Backend (`go test`, per-service core gate) | api-gateway 80.0%, auth 93.2%, compliance 88.8%, payment-orchestrator 83.5%, shared/identity 88.2% — all ≥ 80% |
| Smart contracts (Foundry, 256 fuzz runs) | 286 tests passed, 0 failed, 1 skipped; 98.12% lines / 92.36% branches / 100% functions |

**End-to-end (live stack, `make scenario-b.test-integration` → `TestFullHappyPath`) — PASS (63.1 s).** Eight phases spanning E2E-B-01/02/03/06: readiness, liquidity provision (dual-sided commit-reveal → pool ACTIVE), onboarding, fiat issuance, reserve tokenisation, cross-currency transfer (bridge-in → AMM swap → bridge-out), beneficiary receipt, partial LP withdrawal.

**Performance (R1-12.3, k6; run `20260618T162301Z`):**

| Measurement | Target | Measured | Verdict |
| :---- | :---- | :---- | :---- |
| Value-transfer throughput | 50 TPS, <1% err | 9001 accepted, 0% err, admit p95 6 ms | **PASS** |
| Zeto privacy throughput | 15 TPS, <1% err | 2701 accepted, 0% err | **PASS** |
| AMM quote latency | p95 ≤ 300 ms | 8 ms | **PASS** |
| AMM swap latency (hub-only) | p95 ≤ 6000 ms | 1027 ms | **PASS** |
| Pool-status latency | p95 ≤ 15000 ms | 10 ms | **PASS** |
| AMM swap throughput (hub-only) | 30 TPS, <1% err | 88.9 TPS at 33.4% err | **REVISE** |
| Error rate (steady state) | < 1% | 2.369% | **FAIL** |
| Cross-chain finality (TTF) | p95 < 5 s | no samples (n=0) | UNKNOWN |
| Full cross-currency payment | ungated SLA | 9.4 TPS, p95 60 s | CHARACTERIZED |

*Justification — AMM swap 30 TPS (REVISE) and steady-state error rate (FAIL):* the hub-only swap reaches ~88.9 TPS but at 33.4% error when over-driven, while swap/quote/pool latency stay within gate. The limiter is single-signer EVM nonce serialisation — one signer, mined one-per-nonce at the 2 s block cadence; the steady-state 2.37% error is the same over-driven path bleeding into the aggregate baseline. Remediation: revise the target to the highest rate that holds < 1% error and p95 ≤ 6 s, and add multi-key signing (a signer pool). TTF captured 0 samples (unmeasured, not failed). The full cross-currency payment is reported as an ungated SLA (bridge/nonce-bound).

## **Test Execution Timeline and Responsibilities** {#test-execution-timeline-and-responsibilities-1}

### **Stakeholders** {#stakeholders-1}

| Stakeholder | Role |
| :---- | :---- |
| **LNET** | Platform developer and integrator — responsible for hub smart contract, API, infrastructure, tooling delivery, and test execution |
| **Banks** | Participant institutions (bank-a, bank-b, central-bank-a, central-bank-b, MLP if enabled) — responsible for UAT validation and operational sign-off |

### **Responsibility Matrix (RACI)** {#responsibility-matrix-(raci)-1}

| Phase | LNET | Banks | Notes |
| :---- | :---- | :---- | :---- |
| Test Infrastructure Setup | **R / A** | C | LNET deploys hub devnet; Banks provide test account data |
| Smart Contract Unit Tests | **R / A** | I | Foundry suite on hub contracts; no bank involvement required |
| Backend API Unit Tests | **R / A** | I | Go unit suite; all dependencies mocked |
| Integration Tests | **R / A** | C | Live Hub Besu \+ Paladin \+ AMM; Banks consulted for acceptance criteria |
| E2E Core Flows | **R / A** | **R** | LNET executes flows; Banks verify outcomes via portal and API |
| Performance & Security | **R / A** | I | k6 / Foundry fuzz; Banks informed of final results |
| User Acceptance Testing (UAT) | C / **A** | **R** | Banks lead UAT; LNET provides support and resolves defects within 24 h |
| Issue Resolution & Retest | **R / A** | **R** | LNET fixes defects; Banks confirm retest |
| Evidence Bundle & Sign-off | **R / A** | **R** | Joint sign-off required for Deliverable 12 submission |

**R** \= Responsible (executes the work) · **A** \= Accountable (owns the outcome) · **C** \= Consulted (provides input) · **I** \= Informed (notified of results)

### **Execution Timeline** {#execution-timeline-1}

| Phase | Duration | Window | Responsible | Key Output |
| :---- | :---- | :---- | :---- | :---- |
| **Phase 0 — Mobilization** | 1 week | Week 1 | LNET | Hub devnet up, test data seeded, hub accounts provisioned |
| **Phase 1 — Unit Testing** | 1 week | Week 2 | LNET | Forge \+ Go test results; \>= 80% coverage report |
| **Phase 2 — Integration Testing** | 1 week | Week 3 | LNET | Integration test results; hub blockchain connectivity confirmed |
| **Phase 3 — E2E Core Flows** | 1 week | Week 4 | LNET \+ Banks | E2E evidence bundle (cooperative liquidity, commercial swap, bridge round-trip) |
| **Phase 4 — Performance & Security** | 1 week | Week 5 | LNET | k6 report; AMM fuzz invariant results; acceptance thresholds validated |
| **Phase 5 — User Acceptance Testing** | 2 weeks | Weeks 6–7 | Banks (LNET support) | UAT sign-off report; defect log |
| **Phase 6 — Regression & Retest** | 1 week | Week 8 | LNET \+ Banks | Closed defect list; final evidence bundle |
| **Phase 7 — Sign-off & Submission** | 3 days | Week 9 | LNET \+ Banks | Signed test report; Deliverable 12 submitted |

Total: **\~9 weeks** from mobilization to submission. Scenario B execution begins after Scenario A baseline has been established. Actual calendar dates are to be agreed with the IDB Technical Committee.

### **Phase Checklists** {#phase-checklists-1}

#### **Phase 0 — Mobilization (Week 1\)** {#phase-0-—-mobilization-(week-1)-1}

**LNET:** deploy hub test environment · **Banks:** confirm UAT participants and schedule

- [ ] Hub devnet deployed (`make scenario-b.up-infra` \+ `make scenario-b.deploy-contracts`)  
- [ ] Hub Besu RPC reports blocks advancing (`:8645`)  
- [ ] Spoke-B Besu RPC reports blocks advancing (`:8745`)  
- [ ] All 6+ API gateways healthy (`/healthz` 200 OK)  
- [ ] Hub contract addresses non-empty in all entity `.env.infra.*` files  
- [ ] Hub IdentityRegistry has all entities registered with `Verified` status  
- [ ] AMM pool seeded with initial liquidity for BRL-USD pair  
- [ ] ManualOracle price feed active and rate set for all registered pairs  
- [ ] Cacti relay health endpoint returns `{"status":"ok","watcher_active":true}`  
- [ ] Test participant accounts provisioned (bank-a, bank-b, central-bank-a, central-bank-b, MLP)  
- [ ] Test tooling installed (Foundry, Go \>= 1.26, k6)  
- [ ] Banks identify UAT testers per portal role

#### **Phase 1 — Unit Testing (Week 2\)** {#phase-1-—-unit-testing-(week-2)-1}

**LNET:** execute · **Banks:** informed

- [ ] `make scenario-b.test-contracts` — all Foundry hub contract tests pass  
- [ ] `make scenario-b.test-backend` — all Go unit tests pass  
- [ ] Coverage report generated; \>= 80% on AMM, FXAgreement, LiquidityCommitRegistry, PairRegistry  
- [ ] Foundry fuzz tests executed (256 runs per AMM invariant; `FOUNDRY_FUZZ_RUNS=256`)  
- [ ] Results documented in evidence bundle

#### **Phase 2 — Integration Testing (Week 3\)** {#phase-2-—-integration-testing-(week-3)-1}

**LNET:** execute · **Banks:** consulted on acceptance criteria

- [ ] AMM quote endpoint responds against live hub contract  
- [ ] AMM swap execution confirmed on-chain (real transaction broadcast \+ receipt)  
- [ ] LiquidityCommitRegistry commit-reveal flow confirmed against live hub Besu node  
- [ ] Cacti watcher successfully detects CommitMatched event on hub  
- [ ] SpokeBridge lockAndMint confirmed on-chain (W-tCeBM minted on hub)  
- [ ] FX Agreement hub settlement confirmed on-chain  
- [ ] AML blocking at API layer confirmed (blocked participant rejected before on-chain call)  
- [ ] Results documented

#### **Phase 3 — E2E Core Flows (Week 4\)** {#phase-3-—-e2e-core-flows-(week-4)-1}

**LNET:** execute flows · **Banks:** verify results via API responses and portal

- [ ] `tryout-scenario-b-e2e.sh us1` — Cooperative liquidity pair formation (PairRegistry \+ LCR)  
- [ ] `tryout-scenario-b-e2e.sh us2` — MLP bilateral liquidity provisioning  
- [ ] `tryout-scenario-b-e2e.sh us3` — Commercial bank FX swap via AMM (governance path; note: commercial bank path is partial)  
- [ ] `tryout-scenario-b-e2e.sh us4` — FX Agreement lifecycle on hub  
- [ ] `tryout-scenario-b-e2e.sh us5` — PairRegistry bilateral pair approval  
- [ ] `tryout-scenario-b-e2e.sh us6` — SpokeBridge Lock\&Mint / Burn\&Unlock  
- [ ] `tryout-cross-currency-full-lifecycle.sh` — Full payment lifecycle including bridge  
- [ ] `tryout-cacti-interop.sh` — Cacti relay health \+ CommitMatched event detection  
- [ ] `tryout-commercial-swap-e2e.sh` — Commercial bank cross-currency swap (partial; commercial bank path in progress)  
- [ ] Evidence bundle generated per flow

#### **Phase 4 — Performance & Security (Week 5\)** {#phase-4-—-performance-&-security-(week-5)-1}

**LNET:** execute · **Banks:** informed of results

- [ ] `make scenario-b.perf-baseline` — k6 performance baseline executed  
- [ ] AMM quote p95 \<= 300ms confirmed  
- [ ] AMM swap p95 \<= 6000ms confirmed (excluding on-chain finality)  
- [ ] Pool status p95 \<= 15000ms confirmed  
- [ ] Error rate \< 1% at steady-state load  
- [ ] Foundry fuzz AMM invariants executed (256 runs; constant-product `x*y=k` never violated)  
- [ ] RBAC escalation tests executed (COMMERCIAL\_BANK token blocked on governance endpoints)  
- [ ] Circuit breaker pause/resume tested under load  
- [ ] Performance report (k6 JSON \+ HTML) archived

#### **Phase 5 — User Acceptance Testing (Weeks 6–7)** {#phase-5-—-user-acceptance-testing-(weeks-6–7)-1}

**Banks:** execute · **LNET:** support with 24 h defect resolution SLA

| Entity | Portal | UAT Focus |
| :---- | :---- | :---- |
| bank-a | Bank Portal | Onboarding, FX swap initiation via AMM, bridge deposit, balance view |
| bank-b | Bank Portal | FX swap acceptance, bridge withdrawal, balance view |
| central-bank-a | Treasury Portal | Pair approval, liquidity provisioning, hub oversight |
| central-bank-b | Treasury Portal | Equivalent flows to central-bank-a |
| MLP | MLP Portal | Bilateral commit registration, LP share management, rebalancing |

- [ ] Each entity validates its own flows end-to-end  
- [ ] Banks submit defects with reproduction steps; LNET acknowledges within 4 h and resolves within 24 h  
- [ ] UAT defect log completed; critical/major issues escalated immediately

#### **Phase 6 — Regression & Retest (Week 8\)** {#phase-6-—-regression-&-retest-(week-8)-1}

**LNET:** fix defects · **Banks:** confirm resolution

- [ ] All critical and major defects from UAT closed  
- [ ] Regression E2E suite re-executed after each fix batch  
- [ ] Banks confirm fixed defects via targeted retest  
- [ ] Final evidence bundle assembled and validated

#### **Phase 7 — Sign-off & Submission (Week 9\)** {#phase-7-—-sign-off-&-submission-(week-9)-1}

**LNET \+ Banks:** joint

- [ ] LNET produces final Test Report with complete evidence bundle  
- [ ] Banks sign UAT acceptance form  
- [ ] Deliverable 12 package submitted to IDB Technical Committee

## **Testing Methodology** {#testing-methodology-1}

| Layer | Framework | Scope |
| :---- | :---- | :---- |
| **Smart Contracts** | [Foundry](https://getfoundry.sh/) (`forge test`) | Solidity unit tests with full EVM isolation; fuzz tests for AMM constant-product invariant and circuit breaker |
| **Backend API** | Go standard `testing` package | Unit tests with mocked gRPC and blockchain dependencies; isolated per-service |
| **Integration** | Go `testing` \+ local Hub Besu node \+ live AMM | Live hub infrastructure; tests sign real transactions and verify on-chain state |
| **E2E** | Bash scripts (`tryout-*.sh`) \+ API-First runner | Full lifecycle execution via REST API; on-chain finality confirmed via `eth_getTransactionReceipt` |
| **Performance** | `k6` | AMM quote, swap, and pool-status load profiles; thresholds enforced via `make scenario-b.perf-all` per Decision 13 (in-CI merge gate planned) |
| **Security** | Foundry fuzz \+ manual RBAC abuse testing | AMM invariant fuzzing, circuit breaker RBAC, input boundary testing |

**Evaluation criteria (binary):**

- **Pass**: actual result matches expected result exactly; no side effects on ledger state; execution within time budget; AMM invariant holds  
- **Fail**: result differs, system crashes, test times out, AMM invariant violated, or execution cannot reach a terminal state

## **Test Levels** {#test-levels-1}

### **Unit Testing** {#unit-testing-1}

Validates discrete, isolated functions with all external dependencies mocked.

- **Smart Contracts**: function-level tests via Foundry (EVM state isolated per test); covers AMM addLiquidity/removeLiquidity/swapExactOutput/circuit-breaker, LiquidityCommitRegistry FSM, PairRegistry bilateral approval, FXAgreement hub FSM, SpokeBridge authorization, ManualOracle RBAC, CurrencyRegistry uniqueness, IdentityRegistry hub RBAC, CBWeb3Hub deployment validation  
- **Backend API**: Go unit tests; database connections, blockchain clients, and gRPC channels are substituted with mocks; covers AMM handler, bridge handler, liquidity handler, quote endpoint, FX agreement handler, auth/RBAC middleware for v2 endpoints

Run commands:

```shell
# Smart contract unit tests
make scenario-b.test-contracts
# expands to: cd scenario-b/contracts && forge test --match-contract "AutomatedMarketMakerTest|..." -vv

# Go unit tests
make scenario-b.test-backend
# expands to: cd scenario-b/backend && go test ./...
```

### **Integration Testing** {#integration-testing-1}

Validates coordination between components with live local hub infrastructure.

- **AMM Connectivity**: API signs real swap transactions against a live hub Besu node; verifies transaction broadcast, event log parsing (Swap, Sync events), and pool reserve updates  
- **LiquidityCommitRegistry**: commit-reveal flow tested against live hub; CommitMatched event verified via `eth_getLogs`  
- **Cacti Relay**: watcher subscribes to hub events; CommitMatched event detected and cross-spoke action triggered  
- **SpokeBridge**: lockAndMint and burnAndUnlock tested against live spoke and hub nodes; W-tCeBM balance verified  
- **FX Agreement Hub**: full lifecycle executed against live hub FXAgreement contract

**Current Status**: Integration tests are partially implemented. AMM quote and FX agreement integration paths are further along than the bridge and LCR event-detection paths.

### **End-to-End (E2E) Testing** {#end-to-end-(e2e)-testing-1}

Validates complete user journeys via the REST API against a full running hub stack (`make scenario-b.up`).

All E2E tests follow the **API-First** approach: no UI interaction; assertions are made against both the API response and direct on-chain state via `eth_getLogs` and contract read functions.

E2E scripts are located in `scenario-b/tryouts/`. See the [API-First E2E Execution Strategy](#api-first-e2e-execution-strategy) section for flow details.

| Script | User Story | Status |
| :---- | :---- | :---- |
| `tryout-scenario-b-e2e.sh us1` | US1: Cooperative liquidity pair formation | Implemented |
| `tryout-scenario-b-e2e.sh us2` | US2: MLP bilateral liquidity provisioning | Implemented |
| `tryout-scenario-b-e2e.sh us3` | US3: Commercial bank FX swap via AMM | Partial |
| `tryout-scenario-b-e2e.sh us4` | US4: FX Agreement lifecycle on hub | Implemented |
| `tryout-scenario-b-e2e.sh us5` | US5: PairRegistry bilateral pair approval | Implemented |
| `tryout-scenario-b-e2e.sh us6` | US6: SpokeBridge Lock\&Mint / Burn\&Unlock | Implemented |
| `tryout-commercial-swap-e2e.sh` | Commercial bank cross-currency swap | Partial |
| `tryout-cross-currency-full-lifecycle.sh` | Full payment lifecycle including bridge | Implemented |
| `tryout-cacti-interop.sh` | Cacti relay health \+ CommitMatched event detection | Implemented |
| `tryout-deposit-flow.sh` | Spoke deposit (fCeBM) flow | Implemented |
| `tryout-lp-bilateral-ratio.sh` | LP share ratio calculation | Implemented |
| `tryout-payment-routes.sh` | Payment routing validation | Implemented |

## **API-First E2E Execution Strategy** {#api-first-e2e-execution-strategy-1}

### **Three Main E2E Flows** {#three-main-e2e-flows}

#### **Flow 1 — Cooperative Liquidity Pair Formation (US1 / US2 / US5)** {#flow-1-—-cooperative-liquidity-pair-formation-(us1-/-us2-/-us5)}

1. Authenticate as central-bank-a and central-bank-b via `/api/v1/auth/login`  
2. CB-A proposes pair: `POST /api/v2/hub/pair-registry/propose` (CB-A → CB-B, BRL-USD)  
3. CB-B confirms pair: `POST /api/v2/hub/pair-registry/confirm/{pairId}` → pair status \= `ACTIVE`  
4. Verify via `GET /api/v2/hub/pair-registry/pairs` — pair appears in active list  
5. MLP (or CB-A and CB-B) registers commit: `POST /api/v2/hub/liquidity/commit` (both sides)  
6. LiquidityCommitRegistry emits `CommitMatched` when both-sided commits are registered within 72h TTL  
7. Cacti relay detects `CommitMatched` event on hub → triggers AMM pool seeding via Cacti endpoint  
8. AMM pool reserves updated; verify via `GET /api/v2/amm/pool/BRL-USD/status`  
9. Assert: pair active, AMM pool non-zero reserves, LP shares credited to providers  
- **Scripts**: `tryout-scenario-b-e2e.sh us1`, `tryout-scenario-b-e2e.sh us2`, `tryout-scenario-b-e2e.sh us5`  
- **Status**: Implemented

#### **Flow 2 — Commercial Bank FX Swap via AMM (US3)** {#flow-2-—-commercial-bank-fx-swap-via-amm-(us3)}

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
- **Scripts**: `tryout-scenario-b-e2e.sh us3`, `tryout-commercial-swap-e2e.sh`  
- **Status**: Partial — governance path works; commercial bank direct swap path is in progress

#### **Flow 3 — SpokeBridge Round-Trip (US6)** {#flow-3-—-spokebridge-round-trip-(us6)}

1. Authenticate as bank-a (commercial bank)  
2. Bridge in: `POST /api/v2/hub/bridge/lock-and-mint` — lock spoke tCeBMa, receive hub W-tCeBMa  
3. Poll hub `eth_getTransactionReceipt` — confirm W-tCeBMa minted  
4. Assert: spoke tCeBMa balance decreased; hub W-tCeBMa balance increased by same amount  
5. Bridge out: `POST /api/v2/hub/bridge/burn-and-unlock` — burn hub W-tCeBMa, receive spoke tCeBMa  
6. Poll spoke `eth_getTransactionReceipt` — confirm spoke tCeBMa unlocked  
7. Assert: hub W-tCeBMa balance \= 0; spoke tCeBMa balance restored to original amount  
8. Verify symmetric accounting: no tokens created or destroyed across the round-trip  
- **Script**: `tryout-scenario-b-e2e.sh us6`  
- **Status**: Implemented

### **On-Chain Verification and Finality Rules** {#on-chain-verification-and-finality-rules-1}

After each API call that broadcasts a transaction:

1. **Event Parsing**: query Besu RPC (`eth_getLogs`) to verify expected contract events were emitted with correct parameters (Swap, CommitRegistered, CommitMatched, PairProposed, PairConfirmed, Locked, Minted, Burned, Unlocked)  
2. **Finality Confirmation**: poll `eth_getTransactionReceipt` every 1 second; assert finality once `receipt.blockNumber + 1` is reached (QBFT \= 1-block finality)  
3. **State Verification**: direct contract read calls confirm state transitions match expected output (pool reserves, pair status, commit status, bridge balances)

## **Traceability Map** {#traceability-map-1}

| Use Case | Test ID(s) | Type | Requirement | Module | Status |
| :---- | :---- | :---- | :---- | :---- | :---- |
| Cooperative Liquidity Pair Formation | UT-SC-B-AMM-01 to AMM-09, UT-SC-B-PAR-01 to PAR-06, E2E-B-01 | Unit/E2E | REQ-FX-008, REQ-LIQ-001 | AMM/PairRegistry | Implemented |
| MLP Bilateral Liquidity Provisioning | UT-SC-B-LCR-01 to LCR-08, INT-API-B-03, E2E-B-02 | Unit/Integration/E2E | REQ-LIQ-002 | LCR/MLP | Implemented |
| Commercial Bank FX Swap via AMM | UT-SC-B-AMM-05 to AMM-10, INT-API-B-01, INT-API-B-02, E2E-B-03 | Unit/Integration/E2E | REQ-PAY-007, REQ-FX-008 | AMM/Payments | Partial |
| FX Agreement Lifecycle on Hub | UT-SC-B-FXA-01 to FXA-09, INT-API-B-05, E2E-B-04 | Unit/Integration/E2E | REQ-PAY-007 | FXAgreement/Hub | Implemented |
| PairRegistry Bilateral Pair Approval | UT-SC-B-PAR-01 to PAR-06, E2E-B-05 | Unit/E2E | REQ-FX-008, REQ-GOV-003 | PairRegistry | Implemented |
| SpokeBridge Lock\&Mint / Burn\&Unlock | UT-SC-B-BRG-01 to BRG-06, INT-API-B-04, E2E-B-06 | Unit/Integration/E2E | REQ-PAY-009 | SpokeBridge | Implemented |
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

## **Test Environments and Reproducible Topology** {#test-environments-and-reproducible-topology-1}

### **Hub-and-Spoke Topology (Scenario B)** {#hub-and-spoke-topology-(scenario-b)}

| Network | Chain ID | Entities | Validator Nodes |
| :---- | :---- | :---- | :---- |
| Hub | 1337 | central-bank-a, central-bank-b, MLP | 4 (QBFT — consortium of Central Banks) |
| Spoke-A | 1338 | central-bank-a, bank-a | 3 (QBFT) |
| Spoke-B | 1339 | central-bank-b, bank-b | 3 (QBFT) |

The Hyperledger Cacti relay subscribes to hub events (CommitMatched) and spoke events, executing cross-chain actions. The hub AMM contract (`AutomatedMarketMaker.sol`) is deployed on chain 1337\. SpokeBridge contracts are deployed on each spoke network and reference the hub for W-tCeBM minting.

### **Component Versions (Validated SBOM)** {#component-versions-(validated-sbom)-1}

| Component | Version |
| :---- | :---- |
| Hyperledger Besu | v25.8.0 (QBFT) |
| Hyperledger Cacti | v2.1.0 |
| Go backend APIs | 1.26.0 |
| Solidity | ^0.8.20 |
| OpenZeppelin | v5.6.0 |
| k6 | latest stable |

### **Key Configuration for Tests** {#key-configuration-for-tests-1}

- **Block time**: 2s intervals; QBFT absolute finality in 1 block  
- **Chain IDs**: Hub (1337), Spoke-A (1338), Spoke-B (1339)  
- **Hub RPC port**: 8645  
- **Spoke-B RPC port**: 8745  
- **API Gateway timeouts**: 30 seconds  
- **Relay cross-chain propagation timeout**: 15 seconds  
- **LiquidityCommitRegistry TTL**: 72 hours (production); shortened in tests for expiry scenarios  
- **AMM slippage tolerance in tests**: 1% for happy-path; 0% for slippage-revert tests

### **Devnet (Local Docker Compose)** {#devnet-(local-docker-compose)-1}

- **Purpose**: unit/integration tests, rapid iteration, E2E flow development  
- **Persistence**: Ephemeral — spun up fresh per test run  
- **Start**: `make scenario-b.up-infra` then `make scenario-b.deploy-contracts`  
- **Seeding**: initialization scripts register test participants, seed AMM pool with BRL-USD liquidity, activate ManualOracle rate  
- **Teardown**: `make scenario-b.down-infra` destroys all state  
- **Readiness criteria**: see readiness checklist below

### **Testnet (Staging)** {#testnet-(staging)-1}

- **Purpose**: persistent E2E, performance benchmarking, stakeholder demonstration  
- **Persistence**: long-lived multi-node hub \+ spoke deployment  
- **Reset**: snapshot-restore to known golden state after destructive tests  
- **Status**: Planned

### **Readiness Checklist ("Environment Up" Criteria)** {#readiness-checklist-("environment-up"-criteria)-1}

Before any integration, E2E, or performance suite may execute:

1. Hub Besu RPC (`:8645`) returns `eth_syncing = false` and block heights are advancing  
2. Spoke-B Besu RPC (`:8745`) returns `eth_syncing = false` and block heights are advancing  
3. `GET /api/v1/health` on Cacti relay returns `{"status":"ok","watcher_active":true}`  
4. `GET /healthz` returns 200 OK on all 6+ API gateways  
5. Hub contract addresses are non-empty in all entity `.env.infra.*` files  
6. Hub IdentityRegistry has all entities registered with `Verified` status  
7. AMM pool for the target pair has non-zero reserves (verified via `GET /api/v2/amm/pool/{pair}/status`)  
8. ManualOracle has an active rate set for all registered currency pairs

## **Performance Testing Strategy** {#performance-testing-strategy-1}

### **Workload Models** {#workload-models-1}

| Profile | Description |
| :---- | :---- |
| **Baseline (Steady State)** | Constant VU load over 1 minute; establishes reference latency and throughput for AMM endpoints |
| **Burst (Spike)** | Sharp increase in quote and swap requests; validates AMM handler queue resilience and rate limiting |
| **Stress** | Gradually increasing VU count until p95 degrades past gate; identifies maximum sustainable TPS |
| **Soak** | Moderate continuous load over 12 hours; identifies memory leaks and pool state drift |

### **Target Metrics** {#target-metrics-1}

- **AMM Quote Latency**: p95 for `GET /api/v2/amm/quote/exact-output`  
- **AMM Swap Latency**: p95 for `POST /api/v2/amm/swap/exact-output` (excluding on-chain finality)  
- **Pool Status Latency**: p95 for `GET /api/v2/amm/pool/{pair}/status`  
- **Error Rate**: failed HTTP requests (5xx, timeouts, contract reverts) vs. total  
- **Throughput (TPS)**: successfully finalized on-chain swap transactions per second  
- **AMM Invariant Stability**: constant-product value must not decrease across performance runs

### **Acceptance Thresholds (Pilot-Grade)** {#acceptance-thresholds-(pilot-grade)-1}

| Metric | Threshold | Gate Source |
| :---- | :---- | :---- |
| AMM Quote p95 | \<= 300ms | SC-021 / `scenario-b-perf.js` |
| AMM Swap p95 | \<= 6000ms | SC-022 / `scenario-b-perf.js` |
| Pool Status p95 | \<= 15000ms | SC-023 / `scenario-b-perf.js` |
| Error Rate (steady state) | \< 1% | `scenario-b-perf.js` |
| AMM Swap Throughput | \>= 30 TPS sustained | Draft — not yet validated against measured devnet results |
| Resource Stability | No memory leaks or node crashes during 12-hour soak | — |

**Note**: The AMM swap throughput target of 30 TPS is a draft value. It is set lower than the Scenario A token transfer target (50 TPS) due to AMM constant-product math overhead on each swap. This threshold must be validated against measured devnet results before being treated as a formal gate.

### **k6 Baseline Execution** {#k6-baseline-execution}

```shell
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

The k6 script (`scenario-b/tests/performance/scenario-b-perf.js`) runs three concurrent scenarios: `quoteScenario` (20 VUs), `swapScenario` (5 VUs), and `poolScenario` (5 VUs). A delta \> 20% above the gate thresholds must block merge per Decision 13\.

### **Tooling** {#tooling-1}

- **API Load Generation**: `k6` — scripted HTTP load with JWT authentication and dynamic payload generation  
- **Monitoring**: Prometheus (metrics scraping) \+ Grafana (dashboards and snapshot exports) — Planned

---

## **Security and Abuse Testing Strategy** {#security-and-abuse-testing-strategy-1}

### **API Security (OWASP Controls)** {#api-security-(owasp-controls)-1}

All endpoints defined in the Scenario B OpenAPI spec (`scenario-b/apis/openapi/amm.yaml`) are subject to:

| Control | Test |
| :---- | :---- |
| **AuthN / AuthZ / RBAC** | `COMMERCIAL_BANK` token accessing governance endpoints (`/api/v2/hub/pair-registry/propose`, AMM pause) must return 403; missing/expired/malformed JWTs must return 401 |
| **Input Validation** | Fuzzing AMM swap endpoint with negative amounts, amounts exceeding reserve, zero `max_amount_in`, non-existent pair identifiers; must return 400 rather than crashing |
| **Replay / Idempotency** | State-changing calls (`/api/v2/amm/swap/exact-output`, `/api/v2/hub/liquidity/commit`) submitted with duplicate `Idempotency-Key`; first call accepted, subsequent calls return 409 |
| **Rate Limiting** | High-frequency quote blasts to verify 429 triggers before backend saturation |

### **Smart Contract Security and Invariants** {#smart-contract-security-and-invariants-1}

- **AMM Access Control**: `setPause` restricted to `CENTRAL_BANK_ROLE` or `GOVERNANCE_ROLE`; unauthorized callers revert  
- **AMM Constant-Product Invariant (Foundry Fuzz)**: after every swap, `reserveA * reserveB >= k_before`; `swapExactOutput` reverts when calculated `amountIn` exceeds `maxAmountIn`; circuit breaker blocks all state changes when paused  
- **LiquidityCommitRegistry TTL**: commits that have not been matched within 72 hours must not be executable; expiry revert tested  
- **PairRegistry Bilateral Approval**: a pair cannot transition to `ACTIVE` with only one-party confirmation; unilateral confirm reverts  
- **SpokeBridge Authorization**: `lockAndMint` and `burnAndUnlock` require caller to be a verified participant; authorization checks tested via Foundry  
- **ManualOracle RBAC**: `setRate` restricted to authorized oracle operator; unauthorized callers revert

### **Resilience and "Fail Closed" Behavior** {#resilience-and-"fail-closed"-behavior-1}

When the hub Besu node or ManualOracle is unresponsive, the payment-orchestrator must default to Fail Closed: return 503 or 500 to the initiator; no tokens are locked on spokes or minted on the hub without a corresponding confirmed receipt.

## **CI/CD Quality Gates and Pipeline Strategy** {#ci/cd-quality-gates-and-pipeline-strategy-1}

**Status: PR pipeline implemented; nightly E2E & performance pipeline planned.** GitHub Actions workflows (`.github/workflows/backend-scenario-b.yml`, `contracts-scenario-b.yml`) run on every PR and enforce the unit, coverage, and contract gates described below. The nightly E2E & performance pipeline against a persistent hub testnet is not yet configured; those flows (including the k6 performance gate) are currently executed manually (and were executed manually for this cycle — see [Execution Results](#execution-results-scenario-b)).

### **Pull Request (PR) Pipeline — Implemented** {#pull-request-(pr)-pipeline-—-planned-1}

Triggered on every PR via GitHub Actions (`backend-scenario-b.yml`, `contracts-scenario-b.yml`).

- **Scope**: Foundry hub-contract suite (`forge fmt --check` → build → test → coverage); Go per-service unit tests with the D6 coverage gate (matrix per service) plus `shared/identity`; `go vet`; `gosec` security scan; the hermetic `integration_lite` tier  
- **Time Budget**: \<= 10 minutes  
- **Pass Criteria**:  
  - 100% pass rate on unit tests  
  - \>= 80% code coverage on core AMM, FXAgreement, LiquidityCommitRegistry business logic (D6 gate)  
  - `gosec` reports no critical findings  
  - Failure blocks the PR from being merged  

Static analysis (SonarQube), hub-contract-bytecode vulnerability scanning, and the in-pipeline k6 performance gate (Decision 13, p95 \<= threshold \+ 20% delta) remain planned additions; the k6 gate is currently enforced manually via `make scenario-b.perf-all`.

**Manual equivalent (current):**

```shell
# Smart contract unit tests
cd scenario-b/contracts && forge test -vv

# Go unit tests (all services)
cd scenario-b/backend && go test ./...
```

### **Nightly E2E & Performance Pipeline — Planned** {#nightly-e2e-&-performance-pipeline-—-planned}

To be triggered every night against the persistent hub Testnet (staging) environment.

- **Scope**: full API-driven E2E suite (US1–US6), Cacti relay validation, performance baseline  
- **Time Budget**: \<= 2 hours  
- **Pass Criteria**:  
  - 100% pass rate on core functional E2E flows  
  - AMM performance gates met (p95 within thresholds)  
  - Automated generation and storage of the Standard Test Evidence Bundle

**Manual equivalent (current):**

```shell
# Full hub stack
cd scenario-b && make scenario-b.up

# E2E flows
bash tryouts/tryout-scenario-b-e2e.sh all
bash tryouts/tryout-commercial-swap-e2e.sh
bash tryouts/tryout-cacti-interop.sh
bash tryouts/tryout-cross-currency-full-lifecycle.sh

# Performance baseline
make scenario-b.perf-baseline
```

## **Evidence Bundle Format** {#evidence-bundle-format-1}

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

## **Known Test Coverage Gaps** {#known-test-coverage-gaps}

The following areas are identified as having incomplete or missing test coverage as of 2026-05-29. These gaps are tracked and will be addressed in subsequent implementation sprints.

| Gap Area | Contract / Service | Missing Tests | Priority |
| :---- | :---- | :---- | :---- |
| AMM extreme fee values | `AutomatedMarketMaker.sol` | Edge cases with fee set to 0 or to the maximum allowed value; fee math correctness at boundaries | Medium |
| AMM reserve overflow protection | `AutomatedMarketMaker.sol` | Swap attempts where calculated `amountIn` would cause reserve overflow; must revert gracefully | High |
| PairRegistry pair removal / archival | `PairRegistry.sol` | Removing an active pair; archiving deactivated pairs; re-proposing an archived pair | Medium |
| FXAgreement concurrent settlement | `FXAgreement.sol` (hub) | Two concurrent `settle` calls on the same agreement; only one should succeed | High |
| LiquidityCommitRegistry garbage collection | `LiquidityCommitRegistry.sol` | Batch expiry of multiple expired commits; gas cost of cleanup; state consistency after cleanup | Medium |
| Commercial bank FX swap E2E | `tryout-commercial-swap-e2e.sh` | Full commercial bank path (not governance bypass): quote → swap → bridge round-trip | High |
| Integration: Cacti event detection | Cacti relay \+ hub | End-to-end test that Cacti watcher detects `CommitMatched` on hub and triggers spoke action | High |
| Integration: SpokeBridge on-chain | SpokeBridge API | Full integration test: API call → on-chain `Locked` event → W-tCeBM minted on hub | Medium |
| AMM throughput validation | k6 performance | 30 TPS target is draft; must be validated with measured devnet results under realistic load | Medium |
| Testnet environment | All services | No persistent staging environment exists yet; all tests run against ephemeral devnet | High |
| CI/CD — nightly pipeline | All | PR pipeline implemented (GitHub Actions: per-service coverage/unit gates, `gosec`, `integration_lite`, Foundry contract suite). Nightly E2E & performance pipeline against a persistent testnet is not yet automated; those flows run manually | Medium |

