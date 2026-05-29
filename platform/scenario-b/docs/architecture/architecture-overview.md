# Architecture Overview — CBWeb3 Platform (Scenario B)

> **Deliverable 11** · CBDC System Architecture Documentation — Scenario B
>
> Project: RG-T4567 · Suboperation: ATN/KS-21330-RG
> Authors: Lucas Campelo, Samuel Venzi
> Date: 2026-05-29

---

> **STATUS — IMPLEMENTATION IN PROGRESS**
>
> Scenario B is implemented but not 100% complete. Several components are functional and have been tested end-to-end at the governance level; others are partially implemented and undergoing active adjustment. Known gaps are explicitly enumerated in the [Known Gaps](#known-gaps) section and flagged inline throughout this document wherever they apply. Do not treat any part of this document as a description of a finalized, production-ready system.

---

## Table of Contents

- [Purpose and Scope](#purpose-and-scope)
- [Scenario B — International Hub with AMM](#scenario-b--international-hub-with-amm)
  - [Network Topology](#network-topology)
  - [Participants and Roles](#participants-and-roles)
  - [Architectural Layers](#architectural-layers)
    - [Frontend Layer](#frontend-layer)
    - [Shared Infrastructure](#shared-infrastructure)
    - [Backend Services (per Entity)](#backend-services-per-entity)
    - [Hub Smart Contract Layer](#hub-smart-contract-layer)
    - [Spoke Contract Layer](#spoke-contract-layer)
    - [SpokeBridge — Lock-and-Mint / Burn-and-Unlock](#spokebridge--lock-and-mint--burn-and-unlock)
    - [Privacy Layer — Paladin / Zeto (Caveat)](#privacy-layer--paladin--zeto-caveat)
    - [Interoperability Relay — Hyperledger Cacti](#interoperability-relay--hyperledger-cacti)
  - [Key Flows](#key-flows)
    - [Flow 1 — Cooperative Liquidity (Commit-Reveal)](#flow-1--cooperative-liquidity-commit-reveal)
    - [Flow 2 — Commercial Bank AMM Swap](#flow-2--commercial-bank-amm-swap)
    - [Flow 3 — FX Agreement Lifecycle](#flow-3--fx-agreement-lifecycle)
  - [State Machines](#state-machines)
    - [AMM Circuit Breaker](#amm-circuit-breaker)
    - [FX Agreement](#fx-agreement)
    - [PairRegistry — Currency Pair Lifecycle](#pairregistry--currency-pair-lifecycle)
    - [LiquidityCommitRegistry — Commit Lifecycle](#liquiditycommitregistry--commit-lifecycle)
    - [Participant (IdentityRegistry)](#participant-identityregistry)
  - [Data Flows and Network Boundaries](#data-flows-and-network-boundaries)
  - [Key Design Decisions](#key-design-decisions)
- [Known Gaps](#known-gaps)
- [Technology Stack Summary](#technology-stack-summary)
- [Related Documents](#related-documents)

---

## Purpose and Scope

This document provides the architectural narrative for Scenario B of the CBWeb3 platform — the **International Hub with AMM** variant of the regional wholesale CBDC settlement prototype. Scenario B extends the bilateral correspondent banking model of Scenario A by introducing a central hub network hosting an Automated Market Maker (AMM) that enables multi-currency FX settlement without requiring pre-negotiated bilateral FX agreements between each pair of jurisdictions.

This document covers: hub-and-spoke network topology, smart contract architecture on the hub and spokes, the SpokeBridge lock-and-mint / burn-and-unlock mechanism, the cooperative liquidity commit-reveal protocol, the commercial bank AMM swap flow, backend service components specific to Scenario B, the Cacti relay's Scenario B role, and all relevant state machines.

Detailed sequence diagrams and flow charts are maintained in [`../charts/scenario-b/`](../charts/scenario-b/) and should be read alongside this document.

For Scenario A (Enhanced Correspondent Banking — bilateral HTLC), refer to the Scenario A architecture overview.

---

## Scenario B — International Hub with AMM

Scenario B models a multi-jurisdiction wholesale CBDC settlement arrangement mediated by an international hub network. Two central banks each govern a spoke network. A central hub — operated as a consortium of central banks — hosts AMM liquidity pools denominated in hub-wrapped CBDC tokens. Commercial banks access cross-border liquidity by bridging their spoke tokens to the hub, swapping through the AMM, and bridging back to the destination spoke.

### Network Topology

The platform uses a three-network hub-and-spoke model: Hub, Spoke-A, and Spoke-B.

| Network | Chain ID | Consensus | Entities | Notes |
|---------|----------|-----------|----------|-------|
| **Hub** | 1338 | QBFT | central-bank-a, central-bank-b, mlp (opt-in) | **Local dev: Hub shares Spoke-A's node and RPC (:8645). No dedicated hub network yet — known limitation. Production target: 4+ validator consortium.** |
| **Spoke-A** | 1338 | QBFT | central-bank-a, bank-a, bank-c | 3 validator nodes |
| **Spoke-B** | 1339 | QBFT | central-bank-b, bank-b, bank-d | 3 validator nodes |

**Important local development caveat**: In the current local development environment there is **no dedicated hub network**. The hub smart contracts are deployed on the same Besu node and chain (1338, RPC :8645) used by Spoke-A. This is a deliberate simplification to enable end-to-end testing without operating a separate 4-validator consortium. In a production deployment, the hub would be an independent permissioned network with one validator per sovereign central bank participant.

The MLP (Multilateral Liquidity Provider) entity — for example, an institution such as IDB — is an optional hub participant. It is **not started by default**; it requires `ENABLE_MLP=true` in the deployment configuration (see known gap #4).

The full component diagram is available as:
- Interactive: [`CBWeb3-ComponentDiagram.png`](CBWeb3-ComponentDiagram.png)
- Mermaid source: [`../charts/scenario-b/architecture.md`](../charts/scenario-b/architecture.md)

### Participants and Roles

| Entity | Functional Role | Network | API Gateway | gRPC auth | gRPC payment-orch |
|--------|----------------|---------|-------------|-----------|-------------------|
| central-bank-a | CB governance; hub token issuance; hub liquidity provider | Hub / Spoke-A | :38080 | :39091 | :39094 |
| bank-a | Commercial bank | Spoke-A | :18080 | :19091 | :19094 |
| bank-b | Commercial bank | Spoke-B | :28080 | :29091 | :29094 |
| bank-c | Commercial bank | Spoke-A | :48080 | :49091 | :49094 |
| bank-d | Commercial bank | Spoke-B | :58080 | :59091 | :59094 |
| central-bank-b | CB governance; hub token issuance; hub liquidity provider | Hub / Spoke-B | :60080 | :60091 | :60094 |
| mlp (opt-in) | Multilateral Liquidity Provider; optional hub pool participant | Hub | :68080 | :68091 | :68094 |

Roles are enforced at three distinct layers:

1. **Keycloak (OIDC)** — JWT roles (`ROLE_COMMERCIAL_BANK`, `ROLE_TREASURY`, `ROLE_GOVERNANCE`, etc.) gate REST endpoint access at the API gateway
2. **IdentityRegistry (on-chain RBAC)** — roles `GOVERNANCE`, `CENTRAL_BANK`, `COMMERCIAL_BANK`, `LIQUIDITY_PROVIDER` control which addresses may call smart contract functions; `canTransact()`, `canGovern()`, `isLiquidityProvider()`, and `getCentralBankOf()` are the primary authorization predicates
3. **PKI (X.509)** — Each entity has a CA-signed certificate; fingerprints are registered on-chain and verified during participant onboarding

---

### Architectural Layers

#### Frontend Layer

The same five React web portals used in Scenario A serve distinct operational roles. Scenario B introduces AMM-specific operations — liquidity management, pair proposals, and swap execution — primarily in the Bank Portal and Treasury Portal.

| Portal | User | Primary Scenario B Operations |
|--------|------|-------------------------------|
| **Bank Portal** | Commercial bank operators | AMM swap quote and execution, bridge lock-mint / burn-unlock, FX agreement flows |
| **Treasury Portal** | Central bank treasury officers | Liquidity commit registration, pair proposal/confirmation, hub token minting |
| **Governance Portal** | Governance officers | Participant onboarding, hub IdentityRegistry management, circuit breaker governance |
| **Supervisor Portal** | Regulators | Audit log views, compliance screening results, account freeze/unfreeze |
| **NOC Dashboard** | Network operators | Node health, block height monitoring, Cacti liquidity watcher status |

All portals communicate exclusively via REST with the API gateway of their respective entity. There is no direct frontend-to-blockchain or frontend-to-contract communication.

#### Shared Infrastructure

A single Docker network (`cbweb3_network`) hosts three shared infrastructure services accessed by all entity backend stacks:

| Service | Port | Purpose |
|---------|------|---------|
| **Keycloak** | :8081 | OIDC identity provider — one realm per entity |
| **PostgreSQL** | :5432 | Relational persistence — application databases per entity |
| **Redis** | :6379 | Nonce cache for JWT revocation; **in-memory only — not production-grade (known gap #6)** |

#### Backend Services (per Entity)

Each of the seven entities (six core + MLP opt-in) runs the same four-service backend stack as Scenario A:

```
api-gateway  (Go, REST/HTTP)
     │
     ├──► auth              (Go, gRPC) ──► Keycloak JWKS / Redis
     ├──► compliance        (Go, gRPC) ──► PostgreSQL / IdentityRegistry
     └──► payment-orchestrator (Go, gRPC) ──► Besu RPC / PostgreSQL
```

| Service | Protocol | Responsibility |
|---------|----------|---------------|
| **api-gateway** | REST (HTTP/HTTPS) | Request routing, JWT validation, RBAC enforcement, idempotency key enforcement, `X-Correlation-Id` injection |
| **auth** | gRPC | Login, token issuance and refresh, wallet binding, PKI certificate signing, JWT revocation (Redis) |
| **compliance** | gRPC | Participant onboarding (AML/CFT screening), status management (Pending → Verified → Suspended), account freeze/unfreeze |
| **payment-orchestrator** | gRPC | AMM swap orchestration, bridge operations, FX Agreement state machine, deposit/escrow flows, Cacti relay integration |

Key Scenario B-specific backend components within the api-gateway service:

| Component | Source Path | Responsibility |
|-----------|------------|---------------|
| **AMM client** | `backend/shared/blockchain/scenariob/amm/client.go` | Embedded ABI; exposes `Reserves()`, `IsPaused()`, `QuoteExactOutput()`, `SwapExactOutput()`, `AddLiquidity()`, `RemoveLiquidity()`, `PauseCircuitBreaker()`, `ProposeResume()`, `SignResume()` |
| **LiquidityCommitRegistry adapter** | `backend/services/api-gateway/internal/app/liquidity_commit_registry.go` | `RegisterCommit()`, `CancelCommit()`, `GetCommit()` — interfaces with the hub `LiquidityCommitRegistry` smart contract |
| **SovereignLiquidityService** | `backend/services/api-gateway/internal/services/sovereign_liquidity_service.go` | Executes matched sovereign CB commits triggered by Cacti `CommitMatched` events via `ExecuteMatchedCommit()`; **error recovery / reconciliation is incomplete (known gap #3)** |
| **BridgeHandler** | `backend/services/api-gateway/internal/http/handlers/bridge_handler.go` | `POST /api/v2/bridge/lock-mint`, `POST /api/v2/bridge/burn-unlock`; `bank_code` derived server-side from JWT + config to prevent spoofing |

**v2 REST API surface — key Scenario B endpoints:**

```
GET  /api/v2/amm/quote/exact-output?pair=BRL-USD&amount_out=1000
POST /api/v2/amm/swap/execute                               # commercial bank AMM swap
POST /api/v2/amm/liquidity/add                              # CB adds dual-sided liquidity
POST /api/v2/amm/liquidity/remove                           # CB removes liquidity
POST /api/v2/amm/liquidity/commit                           # CB registers sovereign commit
POST /api/v2/amm/liquidity/sovereign-execute-matched-commit # Cacti webhook trigger
GET  /api/v2/amm/pairs                                      # list active currency pairs
POST /api/v2/amm/pairs/propose                              # CB proposes new pair
POST /api/v2/amm/pairs/{pair_id}/confirm                    # counterparty CB confirms pair
POST /api/v2/bridge/lock-mint                               # bank locks spoke tokens → hub
POST /api/v2/bridge/burn-unlock                             # bank returns hub tokens → spoke
POST /api/v2/amm/token/mint-and-approve                     # CB mints hub tokens for pool seeding
```

#### Hub Smart Contract Layer

The following contracts are deployed on the Hub (chain 1338 in local dev, co-located with Spoke-A):

| Contract | Purpose | Key Functions / Authorization |
|----------|---------|-------------------------------|
| **AutomatedMarketMaker** | Constant-product AMM (`x · y = k`) | `swapTokensForExactTokens()`, `addLiquidity()`, `removeLiquidity()`, `addSingleSidedLiquidity()`; circuit breaker: pause (1-of-N CB), resume (2-of-N CBs via `proposeResume()` + `signResume()`) |
| **ManualOracle** | FX rate oracle | `setRate(token0, token1, rate)` — requires `GOVERNANCE_ROLE`; read by AMM for price reference |
| **LiquidityCommitRegistry** | Sovereign CB commit-reveal protocol | `registerCommit(poolPair, side, amount, wTokenAddress)`, `cancelCommit()`, `expireCommit()`; 72h TTL; emits `CommitMatched` events consumed by Cacti relay |
| **PairRegistry** | Bilateral CB approval for new currency pairs | `proposePair()`, `confirmPair()`, `getAllActivePairs()`; requires both CBs to approve before a pair becomes active |
| **CurrencyRegistry** | Hub currency discovery | `registerCurrency()`, `removeCurrency()`, `getAllCurrencies()` |
| **FXAgreement** | Bilateral FX agreement lifecycle on hub | `propose()`, `accept()`, `reject()`, `cancel()`, `settle()`; `proposeOnBehalf()`, `acceptOnBehalf()` for relay-mediated flows |
| **IdentityRegistry** | On-chain RBAC | Roles: `GOVERNANCE`, `CENTRAL_BANK`, `COMMERCIAL_BANK`, `LIQUIDITY_PROVIDER`; `canTransact()`, `canGovern()`, `isLiquidityProvider()`, `getCentralBankOf()` |
| **TokenizedCentralBankMoney (tCeBM)** | ERC-20 CBDC | One instance per currency on the hub (e.g., tCeBM-BRL, tCeBM-EUR); `CENTRAL_BANK_ROLE` required for `mint()` and `burn()` |

Contracts are deployed via Foundry scripts. Addresses are propagated to service `.env` files via `make contracts.sync-addresses`.

#### Spoke Contract Layer

Each spoke (Spoke-A, Spoke-B) retains its own local contract set, consistent with the Scenario A deployment:

| Contract | Purpose |
|----------|---------|
| `TokenizedCentralBankMoney` (tCeBM, spoke-local) | Spoke-native CBDC token |
| `FiatCentralBankMoney` (fCeBM) | Collateralized fiat token |
| `HTLC` | Cross-spoke coordination anchor (used primarily in Scenario A; retained) |
| `SpokeBridge` | Coordinates Lock&Mint and Burn&Unlock operations with the hub |
| `IdentityRegistry` (spoke-local) | On-chain RBAC for spoke participants |

#### SpokeBridge — Lock-and-Mint / Burn-and-Unlock

The `SpokeBridge` contract implements the two-way peg mechanism that moves liquidity between a spoke and the hub. It ensures that the total supply of any tCeBM is conserved across the hub and all spokes: tokens locked on the spoke are precisely accounted for by the wrapped tokens minted on the hub.

**Lock-and-Mint (spoke → hub):**

```
1. Bank calls POST /api/v2/bridge/lock-mint
2. payment-orchestrator locks bank's spoke tCeBM inside SpokeBridge
3. Hub mints wrapped hub token (W-tCeBM) to bank's hub address
4. Bank now holds W-tCeBM on hub and may interact with AMM
```

**Burn-and-Unlock (hub → spoke):**

```
1. Bank calls POST /api/v2/bridge/burn-unlock
2. Hub burns bank's W-tCeBM
3. SpokeBridge releases previously locked spoke tCeBM back to bank's spoke address
```

#### Privacy Layer — Paladin / Zeto (Caveat)

In Scenario A, each entity runs a **Paladin sidecar node** managing private state via the Zeto domain (`Zeto_AnonNullifier`, nullifier-based double-spend prevention with ZK proofs). Spoke-level Paladin nodes are retained in Scenario B for spoke-side private token operations.

**However, the Hub does not have a Paladin node.** Hub-level transactions — AMM swaps, liquidity additions, bridge operations — are executed as public Besu transactions visible to all hub validators. Pool sizes, swap amounts, and LP positions are therefore transparent on the hub ledger.

The decision on whether to introduce Paladin-level privacy on the hub is **pending** and not yet scoped. See known gap #5.

#### Interoperability Relay — Hyperledger Cacti

In Scenario B, the Cacti relay plays a different role than in Scenario A. It does **not** relay HTLC secrets. Instead, it acts as a **liquidity event watcher and sovereign commit executor**.

**Cacti Scenario B configuration:**

| Property | Value |
|----------|-------|
| Service | `LiquidityCommitWatcher` |
| Source | `interop/hub-and-spoke/cacti/src/liquidity-commit-watcher.ts` |
| Port | :4000 |
| Health check | `GET /api/v1/health` → `{"status":"ok","mode":"scenario-b-liquidity","watcher_active":true}` |

**Responsibilities:**

1. Subscribes to `LiquidityCommitRegistry` `CommitMatched` events on the Hub via `eth_getLogs`
2. When both CB commits for a pair are matched, calls `POST /api/v2/amm/liquidity/sovereign-execute-matched-commit` on **both** CB API gateways using the `X-Relay-Auth` authentication header
3. Each CB gateway then independently executes the bridge-and-add-liquidity sequence for its side of the pool

The relay holds no keys and signs no transactions. It is a stateless event processor; all state is on-chain or in the CB backend databases. Event subscription is log-based (`eth_getLogs`) rather than WebSocket-only, providing resilience after relay restarts.

---

### Key Flows

#### Flow 1 — Cooperative Liquidity (Commit-Reveal)

This flow (user story US1) allows two central banks to cooperatively seed or extend an AMM liquidity pool without requiring either CB to act unilaterally. A commit-reveal protocol enforced by the `LiquidityCommitRegistry` smart contract ensures neither CB bridges and adds liquidity without a binding counterpart commitment.

```
Step 1:  CB-A proposes new currency pair
         POST /api/v2/amm/pairs/propose
         → PairRegistry.proposePair() on hub

Step 2:  CB-B confirms the pair
         POST /api/v2/amm/pairs/{pair_id}/confirm
         → PairRegistry.confirmPair() → pair becomes ACTIVE

Step 3:  CB-A registers sovereign commit (side A, BRL amount, W-tCeBM address)
         POST /api/v2/amm/liquidity/commit
         → LiquidityCommitRegistry records CB-A's commit (72h TTL)

Step 4:  CB-B registers sovereign commit (side B, EUR amount, W-tCeBM address)
         POST /api/v2/amm/liquidity/commit
         → LiquidityCommitRegistry detects match → emits CommitMatched event

Step 5:  Cacti LiquidityCommitWatcher detects CommitMatched event
         → calls sovereign-execute-matched-commit on CB-A gateway
         → calls sovereign-execute-matched-commit on CB-B gateway

Step 6:  Each CB gateway (independently) executes:
         a. Bridge spoke tCeBM → hub (Lock&Mint via SpokeBridge)
         b. Add liquidity to AMM pool (addLiquidity())
         c. Record commit execution result in PostgreSQL
```

If either CB fails to register a matching commit within 72 hours, the commit expires and can be cancelled via `cancelCommit()` or expired via `expireCommit()`. Failed execution after a `CommitMatched` event does not auto-retry — see known gap #3.

Full design: [`../design/cooperative-liquidity.md`](../design/cooperative-liquidity.md)

#### Flow 2 — Commercial Bank AMM Swap

This flow (user story US3) allows a commercial bank to execute a cross-currency payment through the hub AMM.

**Note: the commercial bank swap handler backend routing is partially implemented. The governance-level execution path has been tested. The full commercial bank path — where the bank's payment-orchestrator orchestrates the complete bridge → swap → bridge sequence — needs completion and end-to-end testing. See known gap #2.**

```
Step 1:  Bank-A requests a quote (exact-output):
         GET /api/v2/amm/quote/exact-output?pair=BRL-USD&amount_out=500
         → AMM returns: {amount_in: 502, price_impact: "0.2%", pool_fee: 2}

Step 2:  Bank-A approves and submits swap:
         POST /api/v2/amm/swap/execute
         {pair: "BRL-USD", amount_out: 500, max_amount_in: 502}

Step 3:  Payment orchestrator triggers Lock&Mint for Bank-A:
         Bank-A spoke-A tCeBM-BRL → locked in SpokeBridge → W-tCeBM-BRL minted on hub

Step 4:  AMM executes swap on hub:
         W-tCeBM-BRL in → AutomatedMarketMaker.swapTokensForExactTokens() → W-tCeBM-EUR out

Step 5:  Bank-B side receives W-tCeBM-EUR:
         Burn&Unlock → spoke-B tCeBM-EUR credited to Bank-B's spoke address
```

Price impact and slippage are computed on-chain using AMM reserves at the time of execution. The `QuoteExactOutput()` call in Step 1 reads live reserve data; the actual execution in Step 4 may differ if reserves change between quote and execution.

Full sequence diagram: [`../charts/scenario-b/swap.md`](../charts/scenario-b/swap.md)

#### Flow 3 — FX Agreement Lifecycle

The FX Agreement lifecycle in Scenario B uses the same state machine as Scenario A. The key difference is that in Scenario B the `FXAgreement` contract is deployed **on the Hub** rather than per-spoke independently. The Cacti relay mediates `proposeOnBehalf()` and `acceptOnBehalf()` calls when proposing and accepting parties are on different spokes.

Full design: [`fx-agreement-hybrid-design.md`](fx-agreement-hybrid-design.md)

---

### State Machines

#### AMM Circuit Breaker

The AMM circuit breaker allows central banks to halt trading in exceptional circumstances (extreme rate dislocation, technical incident). The pause is asymmetric: a single CB can pause, but resumption requires multi-CB consensus.

```
ACTIVE ──── any CB calls PauseCircuitBreaker() ──► PAUSED
              (1-of-N CB threshold)                    │
                                                       │
                                              CB-X calls proposeResume()
                                                       │
                                                       ▼
                                              RESUME_PROPOSED
                                                       │
                                              CB-Y calls signResume()
                                              (2-of-N CB threshold)
                                                       │
                                                       ▼
                                                    ACTIVE
```

While `PAUSED`, all `swapTokensForExactTokens()` calls revert. Liquidity additions and removals remain available to prevent LP lockout during a pause.

#### FX Agreement

```
PROPOSED ──► ACCEPTED ──► SETTLED   (terminal)
         └──► REJECTED              (terminal)
         └──► CANCELLED             (terminal)
ACCEPTED ──► EXPIRED                (terminal)
```

State transitions are enforced by the `payment-orchestrator` service and mirrored to the counterparty by the Cacti relay. The `FXAgreement` contract on the Hub is the authoritative on-chain record.

#### PairRegistry — Currency Pair Lifecycle

```
(unpaired)
     │
     ▼
  PROPOSED ──── CB-B calls confirmPair() ──► ACTIVE
     │
     └──── timeout / no action ──────────► (remains PROPOSED — can be cancelled)
```

A pair becomes `ACTIVE` only when both sovereign central banks have explicitly approved it. `getAllActivePairs()` returns only confirmed pairs. No AMM pool may be seeded for a pair until it is `ACTIVE`.

#### LiquidityCommitRegistry — Commit Lifecycle

```
(no commit)
     │
     ▼
  PENDING ──── counterpart commit arrives ──► MATCHED ──► CommitMatched event emitted
     │                                                       (consumed by Cacti relay)
     ├──── 72h TTL expires ──────────────────► EXPIRED  (terminal)
     └──── CB calls cancelCommit() ──────────► CANCELLED (terminal)
```

Once a `CommitMatched` event is emitted, the Cacti relay triggers execution on both CBs. A matched commit that fails during backend execution does not automatically retry or transition to a safe failure state — see known gap #3.

#### Participant (IdentityRegistry)

```
NONE ──► Pending ──► Verified ──► Suspended
```

Applies to both hub and spoke `IdentityRegistry` instances. Only `Verified` participants may satisfy `canTransact()`, which gates all token and AMM operations.

---

### Data Flows and Network Boundaries

| Source | Destination | Protocol | What crosses |
|--------|-------------|----------|--------------|
| Frontend | api-gateway | HTTPS REST | JWT, request payload |
| api-gateway | auth / compliance / payment-orchestrator | gRPC (Docker-internal) | Authenticated request context |
| payment-orchestrator | Besu node (spoke-local) | Besu RPC (HTTP) | Signed spoke transactions, `eth_getLogs` |
| payment-orchestrator | Besu node (hub / chain 1338) | Besu RPC (HTTP) | Signed hub transactions, AMM calls, bridge operations |
| Cacti relay | Hub Besu node | Besu RPC (HTTP) | `eth_getLogs` subscription for `CommitMatched` events |
| Cacti relay | CB api-gateways (both) | HTTPS REST | `POST sovereign-execute-matched-commit` with `X-Relay-Auth` |
| api-gateway | PostgreSQL | GORM/TCP | Commit records, agreement state, audit logs |
| auth | Redis | Redis protocol | Nonce cache, token revocation |
| auth | Keycloak | OIDC/JWKS (HTTP) | Public key retrieval for JWT verification |

**Cross-spoke communication in Scenario B** is mediated by the Cacti relay (for liquidity events) and by the hub itself (through shared AMM contracts). Commercial banks on Spoke-A and Spoke-B interact indirectly via hub AMM operations — there is no direct Spoke-A to Spoke-B RPC communication.

No component communicates directly with a Besu node or backend service belonging to a different entity, with the exception of the Cacti relay, which has controlled, authenticated access to CB API gateways via `X-Relay-Auth`.

---

### Key Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | **Hub co-located with Spoke-A in local dev** | Operating a dedicated 4+ validator hub consortium requires significant infrastructure. For local development and testing, the hub contracts are deployed on the same Besu node as Spoke-A (chain 1338, :8645). This enables full end-to-end flow testing without the operational overhead of a multi-validator consortium. A dedicated hub is the production target. |
| 2 | **Commit-reveal protocol for sovereign liquidity** | Neither central bank should be required to add liquidity to a pool without a binding commitment from the counterpart CB. The `LiquidityCommitRegistry` contract enforces mutual commitment before either side executes, preventing a situation where one CB bridges and adds liquidity but the counterpart does not. |
| 3 | **Bridge-based liquidity (Lock&Mint / Burn&Unlock)** | Spoke tCeBM tokens remain sovereign on their respective chains. The hub operates exclusively with wrapped representations (W-tCeBM). This preserves each central bank's monetary sovereignty: hub liquidity events are always mirrored by corresponding locked spoke tokens, maintaining full reserve accountability. |
| 4 | **Asymmetric AMM circuit breaker (1-of-N pause, 2-of-N resume)** | Pause requires a single CB (fast response to emergencies); resume requires multi-CB consensus (prevents a single actor from unilaterally restarting trading after a halt). This reflects the asymmetric risk profile: blocking trading is a conservative protective action; resuming trading is a consequential operational decision requiring broader agreement. |
| 5 | **Constant-product AMM (`x · y = k`)** | Provides predictable price discovery, transparent on-chain liquidity, and eliminates the need for pre-agreed bilateral FX rates for each transaction. The `ManualOracle` provides a GOVERNANCE-gated reference rate for calibration purposes, but the effective swap price is determined entirely by pool reserves. |
| 6 | **No Paladin on the hub (decision pending)** | Hub transactions are currently public Besu transactions. Adding Paladin-level privacy to hub AMM operations introduces significant complexity — ZK proof generation for constant-product AMM swaps is non-trivial. The decision has been deferred pending further architectural analysis. |
| 7 | **Cacti relay as stateless event processor** | The relay holds no keys, signs no transactions, and custodies no funds. It is a pure event bridge between the `LiquidityCommitRegistry` contract and the CB API gateways. All state and business logic remain within the CB backend services and on-chain contracts. |
| 8 | **MLP as opt-in participant** | The Multilateral Liquidity Provider role is designed for institutions such as development banks that may wish to provide additional hub liquidity without a CB mandate. Making MLP opt-in (`ENABLE_MLP=true`) ensures the core protocol operates correctly without any dependency on a third-party liquidity provider. |
| 9 | **`bank_code` derived server-side in BridgeHandler** | The `bank_code` used to identify the caller in bridge operations is derived from the authenticated JWT and server-side configuration, not from client-supplied parameters. This prevents a commercial bank from spoofing another entity's identity when initiating bridge operations. |

---

## Known Gaps

The following limitations are known and documented. Each represents a planned improvement.

| # | Area | Description | Severity |
|---|------|-------------|----------|
| 1 | **No dedicated hub network in local dev** | The hub shares Spoke-A's Besu node (chain 1338, :8645). There is no separate hub network or consortium of hub validators. A 4+ validator consortium hub deployment is planned but not yet implemented. | High |
| 2 | **Commercial bank swap handler (US3) incomplete** | Backend routing for bank-issued AMM swap requests is partially implemented. The governance-level execution path has been tested. The commercial bank path — where the bank's payment-orchestrator orchestrates the full bridge → swap → bridge sequence — needs completion and end-to-end testing. | High |
| 3 | **Sovereign liquidity reconciliation incomplete** | `SovereignLiquidityService.ExecuteMatchedCommit()` lacks error recovery for failed commits. If the bridge or liquidity addition fails after a `CommitMatched` event, the commit does not automatically transition to a `RECONCILIATION_REQUIRED` state. Manual intervention is required to resolve failed matched commits. | Medium |
| 4 | **MLP not auto-started** | The MLP entity and its service stack are not started by default. `ENABLE_MLP=true` must be explicitly set. MLP integration flows have not been tested end-to-end. | Low |
| 5 | **No Paladin / Zeto privacy on hub** | Hub-level AMM transactions are public Besu transactions. Pool sizes, swap amounts, and liquidity positions are visible to all hub validators. Privacy architecture for the hub has not been designed. | Medium |
| 6 | **Rate limiting is in-memory only** | The api-gateway rate limiter uses an in-memory store. This is not suitable for production deployments (no persistence across restarts, no sharing across multiple api-gateway replicas). Redis-backed persistent rate limiting is a planned improvement. | Low |
| 7 | **CCIP integration is empty** | The `interop/hub-and-spoke/ccip/` directory exists in the repository but contains no implementation. CCIP (Cross-Chain Interoperability Protocol) integration is a future item with no committed timeline. | Low |

---

## Technology Stack Summary

| Layer | Technology | Version / Notes |
|-------|-----------|-----------------|
| Blockchain consensus | Hyperledger Besu QBFT | v25.8.0 |
| Cross-spoke relay | Hyperledger Cacti | v2.1.0 — Scenario B: `LiquidityCommitWatcher` mode |
| Privacy / ZKP | Paladin + Zeto (AnonNullifier) | Spoke-level only; hub privacy pending (gap #5) |
| Smart contracts | Solidity + OpenZeppelin + Foundry | Solidity ^0.8.20, OZ v5.6.0 |
| AMM model | Constant-product (`x · y = k`) | Hub-deployed `AutomatedMarketMaker.sol` |
| Backend services | Go | 1.26.0 |
| Identity provider | Keycloak | (bundled via Docker) |
| Database | PostgreSQL | (bundled via Docker) |
| Session cache | Redis | (bundled via Docker; in-memory only — gap #6) |
| Frontend | React (Turborepo monorepo) | — |
| API specification | OpenAPI 3.0 | `/api/v2/` route surface |

---

## Related Documents

| Document | Description |
|----------|-------------|
| [`../INDEX.md`](../INDEX.md) | Master documentation index for Scenario B |
| [`../charts/scenario-b/architecture.md`](../charts/scenario-b/architecture.md) | Full Mermaid component diagram (all services, networks, contracts, and Cacti relay) |
| [`../charts/scenario-b/swap.md`](../charts/scenario-b/swap.md) | Cross-chain AMM swap sequence diagram |
| [`../design/cooperative-liquidity.md`](../design/cooperative-liquidity.md) | Cooperative liquidity commit-reveal protocol design |
| [`fx-agreement-hybrid-design.md`](fx-agreement-hybrid-design.md) | FX Agreement hybrid design — backend state machine + hub contract |
| [`fx-agreement-production-hardening.md`](fx-agreement-production-hardening.md) | Production hardening notes for the FX Agreement service |
| [`../runbooks/deployment-runbook.md`](../runbooks/deployment-runbook.md) | Step-by-step deployment guide for Scenario B |
| [`../runbooks/environment-setup.md`](../runbooks/environment-setup.md) | Prerequisites and full port reference for all entities |
| [`../runbooks/configuration-reference.md`](../runbooks/configuration-reference.md) | Full environment variable reference |
| [`../runbooks/contract-configuration.md`](../runbooks/contract-configuration.md) | Contract parameters, on-chain roles, Keycloak, PKI |
| [`../runbooks/README.md`](../runbooks/README.md) | Runbooks index |
| [`../test-execution-plan.md`](../test-execution-plan.md) | Test execution plan and strategy |
| [`../../tests/TEST-CATALOG.md`](../../tests/TEST-CATALOG.md) | Test case catalog |
