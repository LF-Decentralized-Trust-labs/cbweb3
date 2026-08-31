# Interop Layer — Cacti Integration Analysis

> **Date:** April 2026  
> **Scope:** `interop/hub-and-spoke/cacti/` and related backend/contracts components  
> **Purpose:** Audit of the current interop implementation, comparison against Hyperledger Cacti, and a concrete plan to properly integrate it.

---

## Table of Contents

1. [Current State](#1-current-state)
2. [How Hyperledger Cacti Works](#2-how-hyperledger-cacti-works)
3. [Gap Analysis — What Is Missing](#3-gap-analysis--what-is-missing)
4. [Trust Model Issues](#4-trust-model-issues)
5. [Recommended Integration Approach](#5-recommended-integration-approach)
6. [Implementation Plan](#6-implementation-plan)

---

## 1. Current State

### 1.1 What Has Been Built

The `interop/hub-and-spoke/cacti/` directory contains a fully custom relay service written in TypeScript. Despite being named after and referencing Hyperledger Cacti, it does **not use the Hyperledger Cacti codebase**. The `@hyperledger/cactus-*` npm packages appear in `package.json` but are never imported anywhere in the source code (an explicit `TODO` comment in `index.ts` acknowledges this).

The custom relay is **operational** for Scenario A (cross-spoke HTLC atomic swaps) and implements the following:

| Component | Location | Status |
|---|---|---|
| TypeScript relay service | `interop/hub-and-spoke/cacti/src/` | ✅ Operational |
| HTLC event polling + forwarding | `src/htlc-relay.ts` | ✅ Operational |
| FX Agreement polling + forwarding | `src/htlc-relay.ts` | ✅ Operational |
| REST API + proof store | `src/index.ts` | ✅ Operational |
| Go adapter (`CactiRelay`) | `backend/.../adapters/cacti/relay.go` | ✅ Operational |
| Port interface (`InteroperabilityPort`) | `backend/.../ports/interoperability.go` | ✅ Designed |
| `HashTimeLockedContract.sol` | `contracts/src/` | ✅ Deployed |
| `FXAgreement.sol` | `contracts/src/` | ✅ Deployed |
| `SpokeBridge.sol` (Scenario B) | `contracts/src/` | ⚠️ Deployed, no orchestration |
| CCIP connector | `interop/hub-and-spoke/ccip/` | ❌ Empty |
| Single-ledger interop | `interop/single-ledger/` | ❌ Empty |

### 1.2 Cross-Spoke Transaction Flow (Scenario A — HTLC)

The flow below covers the full Spoke-A → Spoke-B atomic swap via dual-layer HTLC:

```
Spoke-A (chain 1338)                   Relay (:4000)             Spoke-B (chain 1339)
──────────────────────────             ────────────              ────────────────────────────

[Optional: FX Agreement Phase]
Bank-A → POST /payments/fx/agreements
 └─ FX_STATE_PROPOSED saved
                                  polls /internal/v1/payments/fx/agreements
                                  → gRPC: ProposeFXAgreement to Bank-D
                                                             Bank-D → POST /agreements/{id}/accept
                                                              └─ FX_STATE_ACCEPTED saved
                                  polls Spoke-B agreements
                                  → gRPC: AcceptFXAgreement to Bank-A
Bank-A sees FX_STATE_ACCEPTED ✓

[HTLC Phase]
Bank-A → POST /htlc/lock
 ├─ Generates secret + SHA-256 hashLock
 ├─ Zeto.Lock(amount, Bank-C) → zetoLockRef
 ├─ HTLC.lock(...) emits LogHTLCLocked on chain 1338
 └─ Returns: contractId, hashLock, secret

Bank-D → POST /htlc/lock-with-hash (same hashLock)
 ├─ Zeto.Lock(amount, Bank-B) → zetoLockRef
 └─ HTLC.lock(...) emits LogHTLCLocked on chain 1339

Bank-A → POST /htlc/settle (reveals secret)
 ├─ HTLC.settle(contractId, secret) → emits LogHTLCClaimed on chain 1338
 └─ Zeto.TransferLocked(zetoLockRef, Bank-C, amount)

                                  polls LogHTLCClaimed on chain 1338
                                  resolves counterpart by hashLock match
                                  → gRPC: SettleHTLC(counterpartId, secret)
                                                             HTLC.settle(contractId, secret)
                                                              └─ emits LogHTLCClaimed on chain 1339
                                                             Zeto.TransferLocked(zetoLockRef, Bank-B)

Both spokes: HTLC_STATE_SETTLED ✓
```

**Important:** A manual fallback exists in the tryout scripts — if the relay does not auto-settle within `RELAY_SETTLE_TIMEOUT` seconds (default 30s), the script manually calls settle on Spoke-B. This indicates occasional relay timing issues.

### 1.3 Architecture Diagram

```
┌─────────────────────┐          ┌───────────────────────┐        ┌─────────────────────┐
│      Spoke-A        │          │   Custom Cacti Relay  │        │      Spoke-B        │
│  (Besu, chain 1338) │          │  (TypeScript, :4000)  │        │  (Besu, chain 1339) │
│                     │          │                       │        │                     │
│  payment-           │◄─gRPC───►│  htlc-relay.ts        │◄─gRPC─►│  payment-           │
│  orchestrator       │          │  (ethers.js polling)  │        │  orchestrator       │
│                     │          │                       │        │                     │
│  HashTimeLocked     │◄─RPC────►│  JsonRpcProvider      │◄─RPC──►│  HashTimeLocked     │
│  Contract           │          │  (direct ethers.js)   │        │  Contract           │
│                     │          │                       │        │                     │
│  Paladin/Zeto (ZK)  │          │  In-memory proof store│        │  Paladin/Zeto (ZK)  │
└─────────────────────┘          └───────────────────────┘        └─────────────────────┘
```

### 1.4 Known Gaps in the Current Implementation (Before Cacti Comparison)

These are issues independent of the Cacti comparison — they affect production readiness regardless:

| Gap | Detail | Severity |
|---|---|---|
| No HTLC state persistence | Go in-memory map — lost on restart | High |
| No FX Agreement state persistence | Also in-memory — lost on restart | High |
| No relay persistence or HA | In-memory ring buffers (10K event cap per category) | High |
| Relay as polling (not event-driven) | Besu events and FX states are polled periodically | Medium |
| Best-effort contractId resolution | If ring buffer hasn't seen both lock events, falls back to forwarding source contractId | Medium |
| Secret exposed in lock response | `LockHTLC` response includes the pre-image in plain text | Medium |
| Scenario B has no E2E orchestration | `SpokeBridge.sol` deployed but no relay or orchestration | Medium |
| Cacti npm packages declared but unused | `@hyperledger/cactus-*` in `package.json` — never imported | Low (technical debt) |

---

## 2. How Hyperledger Cacti Works

### 2.1 Overview

**Hyperledger Cacti** is a graduated Hyperledger Foundation project created in November 2022 by merging two previously separate initiatives:

- **Hyperledger Cactus** — a pluggable Node.js framework with per-chain connector plugins, orchestrated at the application layer
- **Weaver Lab** — a relay + proof-based framework using cryptographic state verification

The merge produced a single repository with both codebases intact. As of v2.1.0-alpha.1 (November 2025), both tracks coexist and a third track — **SATP-Hermes** (an implementation of the IETF Secure Asset Transfer Protocol) — is under active development.

**Design principles (no compromises):**
- No intermediary chain
- Networks retain sovereignty and governance
- No changes to the underlying DLT platform
- Privacy-preserving interactions
- Trust based on native DLT consensus — not on a third-party relay

### 2.2 Two Primary Architecture Tracks

#### Track A — Cactus Connector Model

Designed for application-layer orchestration across chains. An API server loads connector plugins, and the DApp coordinates cross-chain calls sequentially.

```
DApp / Client
     ↓ REST or gRPC
cactus-cmd-api-server (Node.js)
     ↓ Plugin Registry
cactus-plugin-ledger-connector-besu  (one instance per Besu network)
     ↓ JSON-RPC
Besu Node
```

The connector implements these core methods:
- `deployContract(req)` — deploy Solidity bytecode
- `invokeContract(req)` — call or send to a contract function
- `transactSigned(rawTx)` — submit pre-signed transaction
- `transactPrivateKey(req)` — sign and send with a raw private key
- `transactCactusKeychainRef(req)` — sign with key stored in Cacti keychain vault (recommended for production)

REST route template:
```
POST /api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/invoke-contract
POST /api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/run-transaction
POST /api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/deploy-contract-solidity-bytecode
```

**Cross-chain coordination** with this model is application-level: call connector A, then connector B. There is no relay or proof layer. Atomicity is provided by the DApp logic (HTLC, 2-phase commit, etc.).

#### Track B — Weaver Relay + Proof Model

Designed for trust-minimized cross-chain interactions. Each network runs a relay and optionally a driver. Cross-chain state is transferred as **cryptographic proofs** verified on-chain.

```
DApp
  ↓ gRPC (App Service)
Local Relay (Rust gRPC server)  ←──gRPC──→  Remote Relay
  ↓ gRPC                                         ↓ gRPC
Driver (per DLT)                               Remote Driver
  ↓ JSON-RPC / SDK                               ↓
DLT Network + Interop Contracts             DLT Network + Interop Contracts
```

**Proof flow:**
1. DApp on Network A queries state from Network B
2. Local relay routes request to Remote relay B
3. Remote relay B calls its driver, which queries the on-chain Interop Contract
4. State + validator signatures (state proof) are collected and returned
5. DApp on Network A submits the proof to its local Interop Contract
6. The Interop Contract verifies the signatures against stored Network B membership info
7. If valid, executes the cross-chain logic on-chain

**Key Weaver components:**
| Component | Language | Role |
|---|---|---|
| Relay | Rust | gRPC message router — one per network |
| Driver | Node.js (Fabric), JVM (Corda) | Relay ↔ DLT adapter |
| Interop Contract (Solidity) | Solidity | On-chain proof verifier + membership store |
| IIN Agents | Various | Optional: decentralized identity exchange |
| `besu-cli` | Node.js CLI | Orchestrates Besu-Besu HTLC flows |

**Critical note for Besu-Besu HTLC:** For asset exchange (atomic swap), Weaver **does not require a relay**. The shared secret/hash is the coordination mechanism. Relays are only needed when one network needs to read and cryptographically verify state from another (data sharing or burn-and-mint asset transfer).

### 2.3 Besu-Besu Standard HTLC Flow (Weaver)

Weaver's reference implementation uses `weaver/samples/besu/simpleasset` contracts:

```
1. Generate: secret + SHA-256(secret) → hashBase64

2. Alice locks her asset on Network-1 for Bob, timeout=1h:
   besu-cli asset lock --network=network1 --sender=1 --recipient=2
     --asset_type=ERC721 --token_id=0 --timeout=3600 --hash_base64=<hash>

3. Bob locks his asset on Network-2 for Alice, timeout=30min:
   besu-cli asset lock --network=network2 --sender=2 --recipient=1
     --amount=10 --timeout=1800 --hash_base64=<hash>

4. Alice claims Bob's lock on Network-2 (reveals preimage):
   besu-cli asset claim --network=network2 --recipient=1 --preimage=secret

5. Bob observes preimage, claims Alice's lock on Network-1:
   besu-cli asset claim --network=network1 --recipient=2 --preimage=secret
```

No relay is involved. The shared secret provides atomicity. Weaver's HTLC contracts (`HashTimeLock.sol`) hold ERC20/721/1155 tokens directly.

### 2.4 `cactus-plugin-htlc-eth-besu`

This package also exists (`@hyperledger/cactus-plugin-htlc-eth-besu` v2.0.0) and wraps HTLC management as a Cactus plugin loaded into the API server. It uses a `PrivateHashTimeLock` Solidity contract and integrates with the Besu connector for transaction submission. It is the Cactus-track equivalent of the Weaver HTLC approach — experimental/research grade (≈1 weekly download).

### 2.5 SATP-Hermes (Emerging Third Track)

The most recent Cacti commits (2025–2026) focus on **SATP-Hermes**, an implementation of the IETF Secure Asset Transfer Protocol. This is the standardized path for cross-chain asset transfer and is DLT-agnostic. It is not yet production-ready but represents the long-term direction of the project.

---

## 3. Gap Analysis — What Is Missing

### 3.1 Summary Table

| Capability | CBWeb3 Has It? | Cacti Has It? | Gap Severity |
|---|---|---|---|
| Operational HTLC cross-spoke swap | ✅ Custom relay | ✅ Weaver `besu-cli` | Low — functional, different tech |
| Actual use of Cacti npm packages | ❌ Listed, not imported | — | Medium — technical debt / misleading |
| Cacti API Server as Besu gateway | ❌ Direct ethers.js | ✅ `cactus-cmd-api-server` | High — core integration gap |
| Cacti keychain vault for key management | ❌ None | ✅ `CactusKeychainRef` | High — security |
| On-chain cryptographic proof verification | ❌ None | ✅ Weaver interop contracts | **Critical — trust model** |
| Foreign network membership on-chain | ❌ None | ✅ Weaver membership registry | **Critical — trust model** |
| Access control policies on-chain | ❌ None | ✅ Weaver interop contracts | **Critical — trust model** |
| ZK privacy layer (Paladin/Zeto) | ✅ Novel feature | ❌ Not in Cacti | N/A — your differentiator |
| HTLC state persistence | ❌ In-memory only | Not prescribed | High — production readiness |
| Relay HA / persistence | ❌ In-memory | Relay is stateless | Medium |
| gRPC push model (Weaver) vs polling | ❌ Polling | ✅ Weaver push | Medium — performance/latency |
| Scenario B (SpokeBridge / burn+mint E2E) | ❌ Incomplete | ✅ Weaver asset transfer | Medium — if needed |
| SATP-Hermes compliance | ❌ None | In active development | Low — future standard |

### 3.2 Technology Stack Comparison

| Dimension | CBWeb3 Custom Relay | Cacti Standard |
|---|---|---|
| Relay language | TypeScript (Node.js) | Rust (Weaver relay) or TS (Cactus API server) |
| Besu RPC client | `ethers.js JsonRpcProvider` directly | `cactus-plugin-ledger-connector-besu` via Cacti API server |
| Transport to payment-orchestrator | gRPC (caller from relay) | gRPC App Service (Weaver) or REST from DApp (Cactus) |
| Event model | Polling loop every N seconds | Event subscription (Weaver push) or periodic query |
| Proof model | None — in-memory ring buffer | Validator signatures over state, on-chain verified |
| Key management | None in relay; keys held per service | Cacti keychain vault (optional hardware-backed) |
| Persistence | None (in-memory) | Not prescribed; relay is designed stateless |

---

## 4. Trust Model Issues

### 4.1 The Core Problem: Relay as Trusted Intermediary

The most significant architectural gap is the **trust model**. The current system works as follows:

```
Spoke-A (chain 1338)                Custom Relay                  Spoke-B (chain 1339)
       │                                  │                                │
       │  LogHTLCClaimed emitted          │                                │
       │─────────────────────────────────►│                                │
       │                                  │  gRPC: SettleHTLC(id, secret)  │
       │                                  │───────────────────────────────►│
       │                                  │                                │ accepts without
       │                                  │                                │ any on-chain proof
```

**Spoke-B blindly trusts the relay's gRPC message.** There is no on-chain mechanism for Spoke-B to independently verify that:

1. A `LogHTLCClaimed` event actually occurred on Spoke-A's chain
2. The event was produced by the correct contract with the correct parameters
3. The relay has not been tampered with or compromised

If the relay is compromised, an attacker can trigger arbitrary settlements on Spoke-B without any corresponding HTLC claim on Spoke-A.

### 4.2 Weaver's Solution

Weaver eliminates relay trust by introducing **on-chain proof verification**:

```
Spoke-A (chain 1338)                Custom Relay                  Spoke-B (chain 1339)
       │                                  │                                │
       │  Event + Validator Signatures    │                                │
       │─────────────────────────────────►│                                │
       │                                  │  State Proof (event + sigs)    │
       │                                  │───────────────────────────────►│
       │                                  │                         ┌──────┴───────┐
       │                                  │                         │ Interop      │
       │                                  │                         │ Contract     │
       │                                  │                         │              │
       │                                  │                         │ Verifies:    │
       │                                  │                         │ sigs ∈ known │
       │                                  │                         │ Spoke-A      │
       │                                  │                         │ validators   │
       │                                  │                         └──────────────┘
       │                                  │                                │
       │                                  │              SettleHTLC only if proof valid
```

**Weaver requires three on-chain components per network:**

1. **Interop Contract** (`InteropContract.sol`) — stores foreign network membership info; verifies incoming state proofs; enforces access control policies
2. **Membership Registry** — records the validator/member public keys of each foreign network as a one-time governance setup
3. **Access Control Policies** — defines which remote networks and which query types are authorized to trigger state changes

### 4.3 Implications for a CBDC Platform

For a regulated Central Bank Digital Currency platform, the trust model is especially important:

- **Regulatory audits** may require that cross-network settlements are verifiable on-chain with no trust in a third-party relay
- **Non-repudiation** requires that Spoke-B can independently prove that Spoke-A's claim occurred, using Spoke-A's validators' signatures — not just the relay's word
- **Operational security** demands that a compromised relay (infrastructure attack, misconfigured credentials, man-in-the-middle) cannot unilaterally trigger settlements on any spoke

The current relay-as-trusted-intermediary design is appropriate for a development/demo environment but does not meet the security bar for production CBDC infrastructure.

### 4.4 The Privacy Dimension

The Paladin/Zeto ZK privacy layer is a genuine differentiator that does not exist in Cacti. However, it creates an additional challenge for the Weaver proof model: the state being proved (a Zeto-locked token transfer) involves zero-knowledge proofs that are not easily verified by a generic Weaver interop contract. The proof model would need to be extended to encompass ZK proof verification, or the HTLC coordination layer (which is public) would act as the proof anchor point while the ZK layer remains internal to each spoke.

---

## 5. Recommended Integration Approach

Given the project's requirements (CBDC, regulated, cross-spoke Besu HTLC with ZK privacy, on-chain proof verification required), the recommended path is a **two-phase integration**:

### 5.1 Phase 1 — Cactus Connector Integration (Replace Raw ethers.js)

**Goal:** Replace direct `JsonRpcProvider` ethers.js calls in the relay with proper Cacti API server interactions. This activates the declared `@hyperledger/cactus-*` dependencies and gives proper key management.

**Architecture after Phase 1:**

```
┌─────────────────────┐     ┌───────────────────────┐     ┌─────────────────────┐
│      Spoke-A        │     │   Cacti Relay Service  │     │      Spoke-B        │
│                     │     │   (TypeScript, :4000)  │     │                     │
│  payment-           │◄────│  htlc-relay.ts         │────►│  payment-           │
│  orchestrator (gRPC)│     │  (Cacti REST calls)    │     │  orchestrator (gRPC)│
│                     │     │           │            │     │                     │
└─────────────────────┘     │    ┌──────┴──────┐     │     └─────────────────────┘
                            │    │ Cacti HTTP  │     │
                            │    │ client      │     │
                            └────┴──────┬──────┘─────┘
                                        │
                    ┌───────────────────┴────────────────────┐
                    │                                        │
          ┌─────────▼──────────┐              ┌─────────────▼──────────┐
          │ cactus-cmd-api-    │              │  cactus-cmd-api-       │
          │ server (Spoke-A)   │              │  server (Spoke-B)      │
          │ :3001              │              │  :3002                 │
          │                    │              │                        │
          │ connector-besu     │              │  connector-besu        │
          │ → Besu RPC :8645   │              │  → Besu RPC :8745      │
          └────────────────────┘              └────────────────────────┘
```

**What changes:**
- Add two `cactus-cmd-api-server` Docker containers (one per spoke) to the docker-compose stacks
- Each container loads `cactus-plugin-ledger-connector-besu` pointing at the spoke's Besu RPC
- Modify `htlc-relay.ts` to call `POST /api/v1/plugins/.../invoke-contract` for HTLC event reads instead of `JsonRpcProvider.getLogs()`
- Wire Cacti keychain vault for signing credentials to replace any raw private key usage
- Actually use `@hyperledger/cactus-plugin-ledger-connector-besu` TypeScript types in the relay source

**What does NOT change:**
- The relay's gRPC calling pattern to payment-orchestrators
- The HTLC and FXAgreement Solidity contracts
- The Paladin/Zeto ZK layer
- The backend Go `CactiRelay` adapter interface

### 5.2 Phase 2 — Weaver On-Chain Proof Verification (Trust Model Fix)

**Goal:** Eliminate the relay as a trusted intermediary by deploying Weaver interop contracts on each spoke. Settlements on Spoke-B must be gated by an on-chain-verified state proof from Spoke-A.

**Architecture after Phase 2:**

```
Spoke-A                           Relay                            Spoke-B
   │                                │                                 │
   │  LogHTLCClaimed + validator    │                                 │
   │  signatures (state proof)      │                                 │
   │──────────────────────────────► │                                 │
   │                                │  SubmitStateProof(proof)        │
   │                                │────────────────────────────────►│
   │                                │                         ┌───────┴──────┐
   │                                │                         │ InteropContract│
   │                                │                         │ verifies proof │
   │                                │                         │ against known │
   │                                │                         │ Spoke-A member│
   │                                │                         │ ships          │
   │                                │                         └───────┬──────┘
   │                                │                                 │ proof valid
   │                                │                   SettleHTLC authorized
```

**What changes:**
- Add Weaver `InteropContract.sol` and membership registry to `contracts/src/`
- Add deployment scripts for interop contracts to both spokes
- One-time governance: register each spoke's validator member keys in the other spoke's interop contract
- Modify relay's settlement forwarding: collect `LogHTLCClaimed` event + validator signatures; package as a Weaver-format state proof
- Add `VerifyAndSettleHTLC` logic in `HashTimeLockedContract.sol` (or a wrapper) that calls the interop contract before executing settlement
- Modify Go `CactiRelay` adapter to support proof submission, not just gRPC forwarding

**Note on ZK interaction:** The state proof anchors on the public HTLC coordination event (`LogHTLCClaimed`). The Zeto ZK proofs remain internal to each spoke — they are not part of the cross-chain proof. This keeps the Weaver integration tractable while preserving privacy guarantees.

### 5.3 Phase 3 — Relay Hardening (Production Readiness)

**Goal:** Address non-Cacti-related production gaps.

- Replace in-memory HTLC and FX state maps with a persistent store (PostgreSQL or Redis)
- Replace in-memory event ring buffers with a persistent event log
- Add relay clustering support / health endpoint improvements
- Address the plain-text secret in `LockHTLC` response (use asymmetric encryption for delivery to initiator, or remove from response and rely on local secret storage)
- Implement Scenario B (SpokeBridge) orchestration with Weaver's asset transfer pattern

### 5.4 Long-Term — SATP-Hermes Alignment

The IETF Secure Asset Transfer Protocol (SATP) is the emerging standard for cross-ledger asset transfer. Cacti's SATP-Hermes track is under active development. Once stable, aligning the relay protocol with SATP-Hermes would provide standards compliance and future interoperability with other SATP-compliant systems.

---

## 6. Implementation Plan

### Phase 1 — Cactus Connector Integration

**Estimated scope:** Medium (relay refactor + docker-compose additions)

#### Step 1.1 — Add Cacti API Server Containers

- Add `cactus-api-server-spoke-a` container to `docker-compose-backend.bank-a.yaml` (or a shared interop compose file)
- Add `cactus-api-server-spoke-b` container for Spoke-B
- Use official image: `ghcr.io/hyperledger/cactus-cmd-api-server`
- Configure each with `--plugins` JSON loading `cactus-plugin-ledger-connector-besu` pointing at the respective Besu RPC endpoint
- Expose ports (e.g., `:3001` for Spoke-A, `:3002` for Spoke-B) and add to relay config

#### Step 1.2 — Update `config.ts`

- Add `CACTI_SPOKE_A_API_URL` and `CACTI_SPOKE_B_API_URL` environment variables
- Remove or repurpose `SPOKE_A_BESU_RPC` / `SPOKE_B_BESU_RPC` (the connector owns those now)

#### Step 1.3 — Refactor `htlc-relay.ts`

- Replace `new ethers.JsonRpcProvider(...)` with an HTTP client calling Cacti's `invoke-contract` endpoint
- Use `@hyperledger/cactus-plugin-ledger-connector-besu` TypeScript types (`InvokeContractV1Request`, `InvokeContractV1Response`) for request/response objects
- Replace raw `getLogs()` calls with Cacti connector's contract event query

#### Step 1.4 — Key Management

- Register signing credentials in Cacti keychain vault during initialization
- Replace any raw private key usage in Besu calls with `Web3SigningCredentialType.CACTUS_KEYCHAIN_REF`

#### Step 1.5 — Tests and Tryouts

- Validate that `tryout-htlc-cross-spoke-minimal.sh` and `tryout-htlc-cross-spoke-a-c-d-b-full.sh` still pass end-to-end
- Add integration tests covering Cacti connector invocation

---

### Phase 2 — Weaver On-Chain Proof Verification

**Estimated scope:** Large (new contracts + relay protocol change)

#### Step 2.1 — Deploy Weaver Interop Contracts

- Copy `weaver/core/network/besu/contracts/interop/InteropContract.sol` (from Cacti repo) into `contracts/src/interop/`
- Write deployment scripts in `contracts/script/InteropContract.s.sol` for both Spoke-A and Spoke-B
- Add to CI deployment pipeline

#### Step 2.2 — Register Cross-Spoke Membership

- Create a governance script (`contracts/script/RegisterSpokeMembership.s.sol`) that:
  - Reads Spoke-A validator public keys and writes them to Spoke-B's interop contract
  - Reads Spoke-B validator public keys and writes them to Spoke-A's interop contract
- Run as a one-time setup per deployment environment
- Store validator key material in `backend/config/pki/`

#### Step 2.3 — Define Cross-Spoke Access Control Policies

- Write policies in each spoke's interop contract specifying:
  - Which remote network ID is trusted (Spoke-A ↔ Spoke-B)
  - Which contract and event type are authorized to trigger settlement

#### Step 2.4 — State Proof Collection in Relay

- Extend `htlc-relay.ts` to collect validator signatures over `LogHTLCClaimed` events from the source spoke after a claim
- Package a state proof per Weaver's protobuf schema
- Submit proof via the Cacti API server (or direct gRPC) to the destination spoke's interop contract

#### Step 2.5 — `HashTimeLockedContract.sol` Proof Gate

- Add `settleWithProof(contractId, secret, stateProof)` function
- This function calls the local `InteropContract` to verify the proof before executing settlement
- Keep the existing `settle(contractId, secret)` for intra-spoke same-party flows (non-cross-chain)

#### Step 2.6 — Update Go `CactiRelay` Adapter

- Extend `InteroperabilityPort` interface with `SubmitStateProof(proof InteroperabilityProof) error`
- Implement in `adapters/cacti/relay.go`
- Wire in `payment-orchestrator` gRPC server to use proof-gated settlement path

---

### Phase 3 — Relay Hardening

**Estimated scope:** Medium

#### Step 3.1 — Persistent State Store

- Add PostgreSQL or Redis service to relay docker-compose
- Replace in-memory HTLC state map (Go) with DB-backed repository
- Replace in-memory FX agreement state with DB-backed repository

#### Step 3.2 — Persistent Event Log

- Replace in-memory ring buffers in `htlc-relay.ts` with an append-only event log (PostgreSQL table or Redis Streams)
- Remove the 10K event cap; add TTL-based expiry instead

#### Step 3.3 — Secret Handling

- Remove `secret` from the `LockHTLC` API response body
- Instead, deliver via an encrypted channel to the initiator (or store locally and return a correlation token)

#### Step 3.4 — Scenario B Orchestration
- Implement relay-side orchestration for `SpokeBridge.sol` lock-and-mint flows
- Add E2E tryout script for Scenario B

---

### Summary Timeline

```
Phase 1 — Cactus Connector   ████████░░░░░░░░░░░░░░░░  (activates real Cacti integration)
Phase 2 — Weaver Proof Model ░░░░░░░░████████████░░░░  (fixes trust model — critical)
Phase 3 — Hardening          ░░░░░░░░░░░░░░░░████████  (production readiness)
```

Phase 1 is a dependency for Phase 2 (Cacti API server is the submission channel for state proofs). Phase 3 can be parallelized with Phase 2.

---

## References

- [Hyperledger Cacti Documentation](https://hyperledger.github.io/cacti/)
- [Cacti GitHub Repository](https://github.com/hyperledger/cacti)
- [Weaver Introduction](https://hyperledger.github.io/cacti/weaver/introduction/)
- [cactus-plugin-ledger-connector-besu (npm)](https://www.npmjs.com/package/@hyperledger/cactus-plugin-ledger-connector-besu)
- [cactus-plugin-htlc-eth-besu (npm)](https://www.npmjs.com/package/@hyperledger/cactus-plugin-htlc-eth-besu)
- [Weaver Besu Samples](https://github.com/hyperledger/cacti/tree/main/weaver/samples/besu)
- [IETF SATP Specification](https://datatracker.ietf.org/doc/draft-ietf-satp-core/)
