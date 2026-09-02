# Architecture Overview

A simplified view of the CBWeb3 system architecture and how the Toolbox artifacts map to each component.

> This overview describes the system as **delivered**: CBWeb3 API Gateway **v2.3.0**, in the
> two scenarios the platform actually runs. It focuses on the concepts a Toolbox user or
> contributor needs; the normative detail lives in the three interface contracts under
> `Toolbox/contracts/`.

---

## System topology: Spoke and Hub

CBWeb3 uses a **dual-layer architecture**:

```
┌─────────────────────────────────────────────────────────────────┐
│                    TRANSNATIONAL HUB                            │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐  │
│  │ Hub AMM +    │  │  Governance  │  │ Bridge (lock-mint /   │  │
│  │ pair &       │  │  Contracts   │  │ burn-unlock) + relay  │  │
│  │ currency reg.│  │ (breaker)    │  │ (cross-chain proofs)  │  │
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
│ │ issues tCeBM   │ │           │ │ issues tCeBM       │ │
│ └────────────────┘ │           │ └────────────────────┘ │
│                    │           │                        │
│ ┌────────────────┐ │           │ ┌────────────────────┐ │
│ │ Commercial     │ │           │ │ Commercial         │ │
│ │ Banks          │ │           │ │ Banks              │ │
│ └────────────────┘ │           │ └────────────────────┘ │
│                    │           │                        │
│ Zeto / Paladin     │           │ Zeto / Paladin         │
│ (privacy)          │           │ (privacy)              │
│ HTLC — scenario A  │           │ HTLC — scenario A      │
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
- Hosts the **Hub AMM**, the pair registry and the currency registry (Scenario B)
- Hosts the **bridge** that mints and burns Hub W-tokens against locked spoke reserves
- Hosts **governance contracts** (circuit breaker, transfer limits, participant registry)
- Operated by a neutral multilateral entity
- All central banks participate on equal footing

### Interoperability (Cacti)

**Hyperledger Cacti** relays cryptographic proofs between Spokes and the Hub:
- Captures block headers and transaction receipts
- Verifies events occurred on the source chain before triggering actions on the destination
- Uses M-of-N threshold signatures from authorized relayer nodes

---

## Two settlement scenarios

The platform ships **two complete stacks**. They are not variants of one API — they share the
identity, token and reserve surfaces and diverge completely at the settlement layer.

### Scenario A: single-ledger, spoke-to-spoke (FX agreement + HTLC)

Bilateral PvP settlement using a **bilateral FX agreement** plus a pair of dual-layer **Hash
Time-Locked Contracts**.

- The originating bank proposes an FX agreement; the counterparty accepts it
- The initiator locks tCeBM — **the gateway generates the secret internally** and returns the
  derived `hash_lock`
- The hash lock crosses to the other spoke **out of band; there is no API endpoint for it**
- The responder locks with that hash, on a **shorter** timelock
- The initiator settles, revealing the secret; the Cacti relay broadcasts it to the other
  spoke, which settles the mirror leg with no call from anyone
- If the secret is never revealed, each side refunds its own leg after its own timelock

Covered by [`contracts/pvp/`](../../contracts/pvp/README.md) — 28 paths / 32 operations.

### Scenario B: hub-and-spoke, "International Hub" (bridge + AMM)

Hub-mediated settlement through a shared automated market maker. **Scenario B has no HTLC
endpoints and no FX agreement surface whatsoever.**

- Both Central Banks provision the corridor: register currencies, propose and confirm the
  pair, bridge reserves in, and commit liquidity to each side of the pool
- A commercial bank quotes (15-second TTL) and issues **one** swap call
- That call fans out into bridge-in (payer's CB) → Hub AMM swap (same CB) → bridge-out
  (**beneficiary's** CB, because one CB burning another's tokens would breach sovereignty) →
  residue return
- The unspent slippage buffer comes back to the payer as a separate `RESIDUE` bridge position

Covered by [`contracts/amm/`](../../contracts/amm/README.md) — 52 paths / 59 operations.

Both scenarios bootstrap their session through the same authentication surface,
[`contracts/auth/`](../../contracts/auth/README.md) — 8 paths / 8 operations, verified
byte-identical between the two delivered gateway specifications.

---

## Technology stack

| Layer | Technology | Language |
|-------|-----------|----------|
| **Backend** | Go microservices (Auth Service, Payment Orchestrator, Compliance Orchestrator, Liquidity Monitor) | Go 1.25+ |
| **Smart contracts** | HTLC + FX coordination (Scenario A); Hub AMM, pair/currency registries, bridge, circuit breaker (Scenario B) | Solidity |
| **Blockchain** | Hyperledger Besu | EVM-compatible |
| **Consensus** | QBFT (Byzantine Fault Tolerant) | — |
| **Privacy** | Hyperledger Paladin + Zeto tokens (ZK-SNARKs) | — |
| **Interoperability** | Hyperledger Cacti + Business Logic Plugins | TypeScript |
| **Frontend** | Bank Portal, Treasury Portal, Supervisor Portal, NOC Portal, Governance Portal | React 18+ / TypeScript |
| **Auth** | Keycloak-issued JWT delivered as an `access_token` **HttpOnly cookie**; commercial banks bind a session with an X.509 + P-256 nonce signature | — |
| **API** | REST / OpenAPI 3.0.3 / JSON — `/api/v1` (core) and `/api/v2` (Hub AMM) served side by side; `/api/v2` is a subsystem prefix, **not** a newer generation | — |

---

## How the Toolbox maps to this architecture

The Toolbox does **not** contain the implementation. It provides **integration artifacts**
that describe the delivered interfaces, so a third party can build against them.

```
┌─────────────────────────────┐     ┌───────────────────────────────┐
│ DELIVERED PLATFORM          │     │ TOOLBOX                       │
│                             │     │                               │
│ API Gateway v2.3.0 (Go)     │ ──> │ contracts/auth/               │
│   scenario-a + scenario-b   │     │   openapi_auth_v2.3.0.yaml    │
│   REST surface              │     │ contracts/pvp/                │
│                             │     │   openapi_pvp_v2.3.0.yaml     │
│                             │     │ contracts/amm/                │
│                             │     │   openapi_amm_v2.3.0.yaml     │
│                             │     │                               │
│ API responses               │ ──> │ mocks/{auth,pvp,amm}/         │
│   (what you get back)       │     │   *.json                      │
│                             │     │                               │
│ Business rules              │ ──> │ test-vectors/{auth,pvp,amm}/  │
│   (what MUST happen)        │     │   *_vectors.json              │
│                             │     │                               │
│ Quality gates               │ ──> │ conformance/                  │
│   (does it work?)           │     │   tests/{auth,pvp,amm}/       │
│                             │     │                               │
│ Getting started             │ ──> │ sandbox/                      │
│   (how do I begin?)         │     │   tutorials/, devnet-guide/   │
└─────────────────────────────┘     └───────────────────────────────┘
```

| Toolbox artifact | Maps to | Delivered component |
|-----------------|---------|---------------------|
| `contracts/auth/` | REST API spec | Gateway authentication surface — `/healthz`, `/api/v1/auth/*` (shared by both scenarios) |
| `contracts/pvp/` | REST API spec | Scenario A gateway — `/api/v1/token/*`, `/api/v1/payments/*`, `/api/v1/htlc/*` |
| `contracts/amm/` | REST API spec | Scenario B gateway — `/api/v1/token/*`, `/api/v1/payments/*`, `/api/v2/{amm,bridge,hub,governance,oversight}/*` |
| `mocks/` | Simulated responses | What each gateway returns for a given call |
| `test-vectors/` | Business rules | HTLC and FX agreement lifecycles; reserve tokenisation; swap and residue reconciliation |
| `conformance/` | Validation suite | Executable checks against any implementation, per gateway profile |
| `sandbox/` | Developer onboarding | How to start without real infrastructure |

### What the Toolbox deliberately does not describe

- **`/internal/*` on either scenario.** Those routes are authenticated by a relay credential,
  are only ever called by the Cacti relay between Central Bank gateways, and are never
  client-callable. Publishing them with a placeholder credential in a public kit would invite
  misuse.
- **The cross-spoke choreography itself.** Moving a hash lock from initiator to responder, and
  broadcasting a revealed secret between spokes, both happen out of band. Neither has an HTTP
  endpoint in the delivered gateway, and the Toolbox does not invent one.
- **The compliance, governance, supervisor, onboarding and PKI-administration surface of
  Scenario A** — roughly 42 of that gateway's 69 `/api/v1` paths. Deferred, not forgotten:
  they serve supervisors and regulators rather than the settlement integrator this kit
  targets.

---

## Further reading

- [Settlement Flow Walkthrough](flow-walkthrough.md) — both scenarios, call by call
- [PvP Interface Contract (Scenario A)](../../contracts/pvp/README.md)
- [Hub-and-Spoke Interface Contract (Scenario B)](../../contracts/amm/README.md)
- [Authentication Contract (shared)](../../contracts/auth/README.md)
- [Conformance Requirements](../../conformance/spec/conformance_requirements.md) — what "pass" means
