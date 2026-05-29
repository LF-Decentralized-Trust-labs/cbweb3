# Architecture Overview — CBWeb3 Platform

> **Deliverable 11** · CBDC System Architecture Documentation
>
> Project: RG-T4567 · Suboperation: ATN/KS-21330-RG
> Authors: Lucas Campelo, Samuel Venzi
> Date: 2026-05-29

---

## Table of Contents

- [Purpose and Scope](#purpose-and-scope)
- [Scenario A — Enhanced Correspondent Banking](#scenario-a--enhanced-correspondent-banking)
  - [Network Topology](#network-topology)
  - [Participants and Roles](#participants-and-roles)
  - [Architectural Layers](#architectural-layers)
    - [Frontend Layer](#frontend-layer)
    - [Shared Infrastructure](#shared-infrastructure)
    - [Backend Services (per Entity)](#backend-services-per-entity)
    - [Smart Contract Layer](#smart-contract-layer)
    - [Privacy Layer — Paladin / Zeto](#privacy-layer--paladin--zeto)
    - [Interoperability Relay — Hyperledger Cacti](#interoperability-relay--hyperledger-cacti)
  - [Key Flows](#key-flows)
    - [Liquidity Injection — Deposit, Escrow, and Redeem](#liquidity-injection--deposit-escrow-and-redeem)
    - [Cross-Spoke PvP Settlement — FX Agreement and HTLC](#cross-spoke-pvp-settlement--fx-agreement-and-htlc)
  - [State Machines](#state-machines)
  - [Data Flows and Network Boundaries](#data-flows-and-network-boundaries)
  - [Key Design Decisions](#key-design-decisions)
- [Scenario B — International Hub with AMM (Planned)](#scenario-b--international-hub-with-amm-planned)
- [Technology Stack Summary](#technology-stack-summary)
- [Related Documents](#related-documents)

---

## Purpose and Scope

This document provides the architectural narrative for the CBWeb3 platform — a regional prototype for wholesale CBDC settlement between jurisdictions in Latin America and the Caribbean. It consolidates the topology, component relationships, protocol decisions, and key design rationale into a single reference intended for technical reviewers, integrators, and auditors.

Detailed sequence diagrams and flow charts are maintained in [`docs/charts/scenario-a/`](../charts/scenario-a/) and should be read alongside this document.

---

## Scenario A — Enhanced Correspondent Banking

Scenario A models a bilateral correspondent banking arrangement between two sovereign jurisdictions. Two central banks each operate an independent blockchain network (spoke). Commercial banks are registered participants on their respective spoke. Cross-border settlement between spokes is performed atomically using a dual-leg HTLC protocol coordinated by a Hyperledger Cacti relay.

### Network Topology

The platform uses a **hub-and-spoke** deployment model where, in Scenario A, there is **no central hub network**. The two spokes connect directly through the Cacti relay.

| Network | Chain ID | Consensus | Entities | Validator Nodes |
|---------|---------|-----------|---------|----------------|
| **Spoke-A** | 1338 | QBFT | central-bank-a, bank-a, bank-c | 3 |
| **Spoke-B** | 1339 | QBFT | central-bank-b, bank-b, bank-d | 3 |

Each validator node corresponds to one entity: the central bank runs the bootnode; each commercial bank runs a validator. Block time is 2 seconds with absolute single-block finality (no reorganizations under normal QBFT conditions).

The full component diagram is available as:
- Interactive: [`CBWeb3-ComponentDiagram.png`](CBWeb3-ComponentDiagram.png)
- Mermaid source: [`../charts/scenario-a/architecture.md`](../charts/scenario-a/architecture.md)

### Participants and Roles

| Entity | Functional Role | Spoke | Besu RPC | API Gateway | Paladin gRPC |
|--------|----------------|-------|----------|-------------|-------------|
| central-bank-a | Governance, tCeBM Mint/Burn | Spoke-A | :8645 | :58080 | :31649 |
| bank-a | Commercial Bank | Spoke-A | :8646 | :18080 | :31659 |
| bank-c | Commercial Bank | Spoke-A | :8647 | :38080 | :31669 |
| central-bank-b | Governance, tCeBM Mint/Burn | Spoke-B | :8745 | :60080 | :31749 |
| bank-b | Commercial Bank | Spoke-B | :8746 | :28080 | :31759 |
| bank-d | Commercial Bank | Spoke-B | :8747 | :48080 | :31769 |

Roles are enforced at three distinct layers:
1. **Keycloak (OIDC)** — JWT roles (`ROLE_COMMERCIAL_BANK`, `ROLE_TREASURY`, `ROLE_GOVERNANCE`, etc.) gate REST endpoint access at the API gateway
2. **IdentityRegistry (on-chain RBAC)** — `ParticipantRole` enum (`CENTRAL_BANK`, `COMMERCIAL_BANK`, `GOVERNANCE`) controls which addresses may call smart contract functions
3. **PKI (X.509)** — Each entity has a CA-signed certificate; fingerprints are registered on-chain and verified during the participant onboarding flow

---

### Architectural Layers

#### Frontend Layer

Five React web applications serve distinct operational roles:

| Portal | User | Primary Operations |
|--------|------|--------------------|
| **Bank Portal** | Commercial bank operators | FX agreement initiation/acceptance, HTLC lock/settle/refund, deposit and escrow requests |
| **Treasury Portal** | Central bank treasury officers | Deposit/escrow/redeem approvals, tCeBM issuance oversight |
| **Governance Portal** | Governance officers | Participant onboarding approval, participant registry, policy configuration |
| **Supervisor Portal** | Regulators | Audit log views, compliance screening results, account freeze/unfreeze |
| **NOC Dashboard** | Network operators | Node health, block height monitoring, relay status, alert management |

All portals communicate exclusively via REST with the API gateway of their respective entity. There is no direct frontend-to-blockchain or frontend-to-Paladin communication.

#### Shared Infrastructure

A single Docker network (`cbweb3_network`) hosts three shared infrastructure services accessed by all six entity backend stacks:

| Service | Port | Purpose |
|---------|------|---------|
| **Keycloak** | :8081 | OIDC identity provider — one realm per entity (7 total: 6 entities + 1 NOC `cbweb3` realm) |
| **PostgreSQL** | :5432 | Relational persistence — 7 databases (6 application + 1 Keycloak) |
| **Redis** | :6379 | Nonce cache for JWT revocation and session management — 6 logical databases (one per entity) |

#### Backend Services (per Entity)

Each of the six entities runs an identical four-service stack:

```
api-gateway  (Go, REST/HTTP)
     │
     ├──► auth              (Go, gRPC) ──► Keycloak JWKS / Redis
     ├──► compliance        (Go, gRPC) ──► PostgreSQL / IdentityRegistry
     └──► payment-orchestrator (Go, gRPC) ──► Paladin / Besu RPC / PostgreSQL
```

| Service | Protocol | Responsibility |
|---------|----------|---------------|
| **api-gateway** | REST (HTTP/HTTPS) | Request routing, JWT validation, RBAC enforcement, idempotency key enforcement, `X-Correlation-Id` injection |
| **auth** | gRPC | Login, token issuance and refresh, wallet binding, PKI certificate signing, JWT revocation (Redis) |
| **compliance** | gRPC | Participant onboarding (AML/CFT screening), status management (Pending → Verified → Suspended), account freeze/unfreeze |
| **payment-orchestrator** | gRPC | HTLC lifecycle (lock/settle/refund), FX Agreement state machine, Zeto private token operations, deposit/escrow/redeem flows, Cacti relay integration |

The full OpenAPI specification for the api-gateway is maintained at [`../../apis/openapi/api-gateway.yaml`](../../apis/openapi/api-gateway.yaml) (v2.3.0, 75 operations).

#### Smart Contract Layer

The same set of contracts is deployed independently on each spoke:

| Contract | Purpose | Authorization |
|----------|---------|--------------|
| `TokenizedCentralBankMoney` (tCeBM) | ERC-20 CBDC — publicly visible on Besu | `CENTRAL_BANK_ROLE` required for `mint()` and `burn()` |
| `FiatCentralBankMoney` (fCeBM) | Collateralized fiat token — represents off-chain fiat deposits | `CENTRAL_BANK_ROLE` required for `mint()` and `burn()` |
| `HashTimeLockedContract` (HTLC) | Cross-spoke coordination anchor — holds no token value; stores only the `hashLock` and `timeLock` | `canTransact()` on `IdentityRegistry` — all parties must be `Verified` |
| `IdentityRegistry` | On-chain participant registry and RBAC | `GOVERNANCE_ROLE` for registration and status changes |
| `FXAgreement` | Bilateral FX trade lifecycle state machine | `IdentityRegistry` gating |
| `SpokeBridge` | Cross-spoke message coordination | `IdentityRegistry` gating |
| `AutomatedMarketMaker` | Constant-product AMM liquidity pool (Scenario B) | `GOVERNANCE_ROLE` for circuit breaker |

Contracts are deployed via Foundry scripts (`make contracts.deploy-spoke-{a,b}`). Addresses are propagated to service `.env` files via `make contracts.sync-addresses`.

#### Privacy Layer — Paladin / Zeto

Each entity runs a **Paladin sidecar node** connected to its local Besu validator. Paladin manages private state using the **Zeto domain** (ZKP-based UTXO model):

- **`Zeto_AnonNullifier`**: private token with nullifier-based double-spend prevention and ZK proofs of transfer validity
- **Pente domain**: private bilateral execution context used for FX Agreement state management (bilaterally confidential)

Key invariant: **the HTLC contract holds no token value**. Tokens are locked inside Paladin as Zeto UTXO states. The HTLC records only the coordination metadata (`hashLock`, `timeLock`, `contractID`) so that the relay can perform cross-spoke settlement without exposing amounts on the public ledger.

External observers on the public Besu chain see only ZK commitment hashes — amounts and counterparties are hidden.

#### Interoperability Relay — Hyperledger Cacti

The Cacti relay (`HtlcRelay` service, TypeScript/Node.js, port :4000) operates between the two spokes:

**Responsibilities:**
1. **FX Agreement propagation** — polls each spoke's internal `/internal/v1/payments/fx/agreements` endpoint at a configurable interval; detects `PROPOSED` agreements and mirrors them cross-spoke via `gRPC ProposeFXAgreement(..., on_behalf=true)`; detects `ACCEPTED` state and mirrors back
2. **HTLC secret extraction and relay** — subscribes to `LogHTLCClaimed` events on Spoke-A via `eth_getLogs`; extracts the pre-image secret; calls `gRPC SettleHTLC` on Spoke-B's payment-orchestrator to complete the counterparty leg
3. **Resilience** — polling is log-based (not subscription-only), so the relay sweeps missed events after a restart

The relay does not hold keys, sign transactions, or custody funds. It is a stateless event processor; all state is on-chain or in the payment-orchestrators' databases.

---

### Key Flows

#### Liquidity Injection — Deposit, Escrow, and Redeem

The escrow flow converts off-chain fiat liquidity into private on-chain tCeBM (Zeto):

```
Phase 1 — Deposit:   fiat → fCeBM (public ERC-20, Besu)
Phase 2 — Escrow:    fCeBM burn  → tCeBM Zeto mint (private UTXO, Paladin)
Phase 3 — Redeem:    tCeBM Zeto transfer to CB → fCeBM remint (back to public)
```

Each phase is **governance-gated**: the central bank must explicitly approve each state transition. Idempotency guards (`mint_tx_hash == ""` before executing) prevent double-minting. Rejection at Phase 2 (before burn) requires no compensation; rejection at Phase 3 triggers a return ZK transfer.

Full sequence diagram: [`../charts/scenario-a/escrow.md`](../charts/scenario-a/escrow.md)

#### Cross-Spoke PvP Settlement — FX Agreement and HTLC

The settlement flow delivers Payment-vs-Payment atomicity across two independent blockchains without a shared ledger:

```
Phase 0 — FX Negotiation:   PROPOSED → ACCEPTED (Cacti mirrors agreement cross-spoke)
Phase 1 — Initiator Lock:   Bank-A locks tCeBM on Spoke-A via Zeto; registers HTLC on-chain
Phase 2 — Counterparty Lock: Bank-B locks tCeBM on Spoke-B with same hashLock (shorter timeLock)
Phase 3 — Settlement:       Bank-A reveals secret → Cacti detects → relays to Spoke-B → both legs settle
Phase 4 — Post-Trade:       Central Bank marks FX Agreement → SETTLED
Contingency:                timeLock expiry → refund unlocks Zeto UTXO on the initiating spoke
```

**Asymmetric timeLocks** are mandatory for atomicity: the counterparty's timeLock (1800s) must be shorter than the initiator's (3600s). This prevents the scenario where the initiator settles Spoke-B but the counterparty refunds Spoke-A.

Full sequence diagram: [`../charts/scenario-a/transfer.md`](../charts/scenario-a/transfer.md)

---

### State Machines

#### FX Agreement

```
PROPOSED ──► ACCEPTED ──► SETTLED   (terminal)
         └──► REJECTED              (terminal)
         └──► CANCELLED             (terminal)
ACCEPTED ──► EXPIRED                (terminal)
```

State transitions are enforced by the `payment-orchestrator` service and mirrored to the counterparty spoke by the Cacti relay. There is no on-chain FX Agreement state in the current implementation — state is held in PostgreSQL with the Pente bilateral context as a future cryptographic anchor.

#### HTLC Position

```
INVALID ──► LOCKED ──► SETTLED   (terminal — secret revealed)
                   └──► REFUNDED (terminal — timeLock expired)
```

Each HTLC position is identified by a `contractID = SHA-256(agreement_id + timestamp)`. The `hashLock = SHA-256(secret)` is the common anchor between the two spoke positions.

#### Participant (IdentityRegistry)

```
NONE ──► Pending ──► Verified ──► Suspended
```

Only `Verified` participants may call `canTransact()` on the IdentityRegistry, which gates all HTLC and token operations.

---

### Data Flows and Network Boundaries

| Source | Destination | Protocol | What crosses |
|--------|-------------|----------|--------------|
| Frontend | api-gateway | HTTPS REST | JWT, request payload |
| api-gateway | auth / compliance / payment-orchestrator | gRPC (Docker-internal) | Authenticated request context |
| payment-orchestrator | Paladin sidecar | JSON-RPC / gRPC | Private transaction requests, ZK proof generation |
| Paladin sidecar | Besu node | Besu RPC (HTTP) | Signed transactions, `eth_getLogs` queries |
| Cacti relay | Besu nodes (both spokes) | Besu RPC (HTTP/WS) | `eth_getLogs` subscriptions |
| Cacti relay | payment-orchestrators | gRPC | `SettleHTLC`, `ProposeFXAgreement` calls |
| payment-orchestrator | PostgreSQL | GORM/TCP | Agreement state, deposit/escrow/redeem records |
| auth | Redis | Redis protocol | Nonce cache, token revocation |
| auth | Keycloak | OIDC/JWKS (HTTP) | Public key retrieval for JWT verification |

No component communicates directly with a Besu node or Paladin on a different spoke. Cross-spoke communication is exclusively mediated by the Cacti relay.

---

### Key Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | **No hub in Scenario A** | Scenario A uses direct Cacti relay between two spokes; the hub network (chain 1337) is reserved for Scenario B (AMM). This reduces operational complexity and latency for bilateral settlements. |
| 2 | **HTLC holds no token value** | All token custody is managed inside Paladin/Zeto as private UTXO states. The HTLC contract serves only as a coordination anchor, preventing public exposure of settlement amounts. |
| 3 | **Asymmetric timeLocks** | Counterparty timeLock < Initiator timeLock (1800s < 3600s). Prevents the case where the initiator settles one spoke and the counterparty subsequently refunds the other. |
| 4 | **Polling-based Cacti relay** | Resilience: the relay reads `eth_getLogs` rather than relying solely on WebSocket push events. After a relay restart, missed events are recovered by replaying the log window. |
| 5 | **FX Agreement state in PostgreSQL, not on-chain** | Current implementation uses a service-layer state machine persisted in PostgreSQL. A Pente bilateral context is created at acceptance time as a future cryptographic anchor. Full on-chain commitment registry is a roadmap item. |
| 6 | **Three-layer authorization** | Keycloak (OIDC roles), IdentityRegistry (on-chain RBAC), and PKI (X.509) provide defense-in-depth. A compromised JWT alone is insufficient to execute on-chain operations if the corresponding address is not `Verified` in the registry. |
| 7 | **One backend stack per entity** | Each of the six entities runs its own isolated api-gateway, auth, compliance, and payment-orchestrator. This matches the institutional separation model and allows each entity to manage its own secrets, keys, and Keycloak realm independently. |

---

## Scenario B — International Hub with AMM (Planned)

Scenario B extends the platform with an international settlement hub (chain 1337) that introduces automated market-making (AMM) liquidity pools for multi-currency FX.

| Component | Status | Description |
|-----------|--------|-------------|
| `AutomatedMarketMaker.sol` | Smart contract implemented | Constant-product AMM (`x · y = k`), `swapExactOutput`, slippage protection, circuit breaker |
| `ManualOracle.sol` | Smart contract implemented | GOVERNANCE-gated FX price oracle |
| `FXAgreement.sol` (hub) | Smart contract implemented | Hub-side FX agreement lifecycle |
| Hub network deployment | Pending | 4+ validator consortium (central banks); Docker Compose profile reserved |
| Backend hub services | Pending | Hub api-gateway and payment-orchestrator stacks |
| AMM E2E flows | Planned | Exact-output swap, slippage protection, governance circuit breaker, LP operations |

When Scenario B is deployed, the Cacti relay will be extended to bridge Spoke-A and Spoke-B through the Hub, replacing direct bilateral spoke-to-spoke coordination with hub-mediated multi-currency settlement.

---

## Technology Stack Summary

| Layer | Technology | Version |
|-------|-----------|---------|
| Blockchain consensus | Hyperledger Besu QBFT | v25.8.0 |
| Cross-spoke relay | Hyperledger Cacti | v2.1.0 |
| Privacy / ZKP | Paladin + Zeto (AnonNullifier) | Paladin v0.15, Zeto v0.2.2 |
| Smart contracts | Solidity + OpenZeppelin + Foundry | Solidity ^0.8.20, OZ v5.6.0 |
| Backend services | Go | 1.26.0 |
| Identity provider | Keycloak | (bundled via Docker) |
| Database | PostgreSQL | (bundled via Docker) |
| Session cache | Redis | (bundled via Docker) |
| Frontend | React (Turborepo monorepo) | — |
| API specification | OpenAPI 3.0 | v2.3.0 |

---

## Related Documents

| Document | Deliverable | Description |
|----------|-------------|-------------|
| [`../runbooks/deployment-runbook.md`](../runbooks/deployment-runbook.md) | D9 | Step-by-step deployment guide |
| [`../runbooks/environment-setup.md`](../runbooks/environment-setup.md) | D9 | Prerequisites and port reference |
| [`../runbooks/configuration-reference.md`](../runbooks/configuration-reference.md) | D10 | Full environment variable reference |
| [`../runbooks/contract-configuration.md`](../runbooks/contract-configuration.md) | D10 | Contract parameters, on-chain roles, Keycloak, PKI |
| [`../../apis/openapi/api-gateway.yaml`](../../apis/openapi/api-gateway.yaml) | D10 | OpenAPI 3.0 specification (v2.3.0) |
| [`../charts/scenario-a/architecture.md`](../charts/scenario-a/architecture.md) | D11 | Full Mermaid component diagram |
| [`../charts/scenario-a/escrow.md`](../charts/scenario-a/escrow.md) | D11 | Deposit / escrow / redeem sequence diagram |
| [`../charts/scenario-a/transfer.md`](../charts/scenario-a/transfer.md) | D11 | Cross-spoke FX Agreement + HTLC sequence diagram |
| [`fx-agreement-hybrid-design.md`](fx-agreement-hybrid-design.md) | D11 | FX Agreement backend / Pente hybrid design |
| [`../test-execution-plan.md`](../test-execution-plan.md) | D12 | Test execution plan and strategy |
| [`../../tests/TEST-CATALOG.md`](../../tests/TEST-CATALOG.md) | D12 | Test case catalog (159 cases) |
| [`../INDEX.md`](../INDEX.md) | D11 | Master documentation index |
