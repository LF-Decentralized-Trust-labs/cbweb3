# Architecture Overview

A simplified view of the CBWeb3 system architecture and how the Toolbox artifacts map to each component.

> This overview is based on Deliverable 4 (Architecture) and Deliverable 7v2 (Design Document). It focuses on the concepts relevant to Toolbox users and contributors.

---

## System topology: Spoke and Hub

CBWeb3 uses a **dual-layer architecture**:

```
┌─────────────────────────────────────────────────────────────────┐
│                    TRANSNATIONAL HUB                            │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐  │
│  │  CCIP / Cacti │  │  Governance  │  │ Relayer Registry      │  │
│  │  (bridging)   │  │  Contracts   │  │ (cross-chain proofs)  │  │
│  └──────────────┘  └──────────────┘  └───────────────────────┘  │
│                                                                 │
│         Hyperledger Besu (QBFT consensus, gasless)              │
└──────────────────────────┬──────────────────────────────────────┘
                           │
              Hyperledger Cacti (interoperability relay)
                           │
          ┌────────────────┼────────────────┐
          │                                 │
┌─────────┴─────────┐           ┌───────────┴───────────┐
│   SPOKE A          │           │   SPOKE B              │
│   (Country A)      │           │   (Country B)          │
│                    │           │                        │
│ ┌────────────────┐ │           │ ┌────────────────────┐ │
│ │ Central Bank A │ │           │ │ Central Bank B     │ │
│ │ issues tCeBM-A │ │           │ │ issues tCeBM-B     │ │
│ └────────────────┘ │           │ └────────────────────┘ │
│                    │           │                        │
│ ┌────────────────┐ │           │ ┌────────────────────┐ │
│ │ Commercial     │ │           │ │ Commercial         │ │
│ │ Banks          │ │           │ │ Banks              │ │
│ └────────────────┘ │           │ └────────────────────┘ │
│                    │           │                        │
│ HTLC + Zeto/       │           │ HTLC + Zeto/           │
│ Paladin (privacy)  │           │ Paladin (privacy)      │
│                    │           │                        │
│ Hyperledger Besu   │           │ Hyperledger Besu       │
└────────────────────┘           └────────────────────────┘
```

### Domestic networks (Spokes)

Each participating country operates its own **private Hyperledger Besu** network:
- **Central Bank** issues and governs tokenized central bank money (tCeBM)
- **Commercial Banks** hold, transfer, and use tCeBM
- **Privacy:** Hyperledger Paladin with Zeto tokens (ZK-SNARKs) encrypts balances and transaction values
- **Consensus:** QBFT (deterministic finality, no gas fees)

### Transnational network (Hub)

A shared settlement layer for cross-border operations:
- Hosts **cross-chain bridging** contracts (CCIP adapters, Cacti relay plugins)
- Hosts **governance contracts** (circuit breaker, participant registry)
- Operated by a neutral multilateral entity
- All central banks participate on equal footing

### Interoperability (Cacti)

**Hyperledger Cacti** relays cryptographic proofs between Spokes and the Hub:
- Captures block headers and transaction receipts
- Verifies events occurred on the source chain before triggering actions on the destination
- Uses M-of-N threshold signatures from authorized relayer nodes

---

## Two settlement scenarios

### Scenario A: Enhanced Correspondent Banking (HTLC)

Bilateral PvP settlement using **Hash Time-Lock Contracts**. This is the scenario fully covered by the Toolbox today.

**4-corner model:**
- CommA (originator) locks tCeBM-A on Spoke A
- Bank C (correspondent in Country B) locks tCeBM-B on Spoke B
- Secret revelation triggers atomic settlement on both sides
- If timeout expires, funds are automatically refunded

> The Toolbox simplifies this to a 2-party model (Central Bank A <-> Central Bank B) for clarity.

### Scenario B: Cross-Chain Interoperability (CCIP / Fabric-X)

Hub-mediated bridging for multi-network settlement:
- CCIP adapters relay messages and token transfers between Spokes and the Hub
- Fabric-X corridor kit enables settlement across heterogeneous DLTs (Besu ↔ Fabric)
- Cryptographic proofs verify cross-chain events before releasing funds

> Not yet covered in Toolbox artifacts. See backlog issues #28 (CCIP Adapter) and #29 (Fabric-X Corridor Kit).

---

## Technology stack

| Layer | Technology | Language |
|-------|-----------|----------|
| **Backend** | Go microservices (Auth Service, Payment Orchestrator, Compliance Orchestrator, Liquidity Monitor) | Go 1.25+ |
| **Smart contracts** | HTLC.sol, Bridge.sol | Solidity |
| **Blockchain** | Hyperledger Besu | EVM-compatible |
| **Consensus** | QBFT (Byzantine Fault Tolerant) | — |
| **Privacy** | Hyperledger Paladin + Zeto tokens (ZK-SNARKs) | — |
| **Interoperability** | Hyperledger Cacti + Business Logic Plugins | TypeScript |
| **Frontend** | Bank Portal, Treasury Portal, Supervisor Portal, NOC Portal, Governance Portal | React 18+ / TypeScript |
| **Auth** | OAuth 2.0 / JWT (RSA256) | — |
| **API** | REST / OpenAPI 3.0.3 / JSON | — |

---

## How the Toolbox maps to this architecture

The Toolbox does NOT contain the implementation. It provides **integration artifacts** that describe the interfaces between components:

```
┌────────────────────────────┐     ┌──────────────────────────┐
│ REAL SYSTEM                │     │ TOOLBOX                  │
│                            │     │                          │
│ Payment Orchestrator (Go)  │ ──> │ contracts/pvp/           │
│   exposes REST API         │     │   openapi_pvp_v0.1.0.yaml│
│                            │     │                          │
│ API responses              │ ──> │ mocks/pvp/               │
│   (what you get back)      │     │   happy-path/*.json      │
│                            │     │                          │
│ Business rules             │ ──> │ test-vectors/pvp/        │
│   (what MUST happen)       │     │   pvp_htlc_vectors.json  │
│                            │     │                          │
│ Quality gates              │ ──> │ conformance/             │
│   (does it work?)          │     │   tests/pvp/test_*.py    │
│                            │     │                          │
│ Getting started            │ ──> │ sandbox/                 │
│   (how do I begin?)        │     │   tutorials/, devnet-guide│
└────────────────────────────┘     └──────────────────────────┘
```

| Toolbox artifact | Maps to | Real-world component |
|-----------------|---------|---------------------|
| OpenAPI contract (`contracts/pvp/`) | REST API spec | Payment Orchestrator endpoints (`/fx/*`, `/htlc/*`) |
| Reference mocks (`mocks/pvp/`) | Simulated responses | What the backend returns for each operation |
| Test vectors (`test-vectors/pvp/`) | Business rules | HTLC state machine, FX agreement lifecycle |
| Conformance tests (`conformance/`) | Validation suite | Executable checks against any implementation |
| Sandbox (`sandbox/`) | Developer onboarding | How to start without real infrastructure |

---

## Further reading

- [PvP Settlement Flow Walkthrough](flow-walkthrough.md) — Step-by-step explanation of the HTLC settlement
- [PvP Interface Contract](../../contracts/pvp/README.md) — Full endpoint documentation
- [Conformance Requirements](../../conformance/spec/conformance_requirements.md) — What "pass" means
