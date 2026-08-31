# interop

> [scenario-a](../README.md) › interop

The **interop** layer provides the cross-spoke connectivity that makes atomic settlement possible between independent blockchain networks. It bridges events, secrets, and state across spoke chains without requiring a shared ledger or trusted intermediary.

---

## Architecture Placement

```
Spoke-A (Besu chain 1338)          Spoke-B (Besu chain 1339)
        │                                    │
        │◄──── [hub-and-spoke / cacti] ──────►│
        │           (relay)                   │
        │                                    │
        └──── HTLC secret bridge ────────────┘
```

The relay listens for HTLC settlement events on both spokes. When Bank-A reveals the HTLC preimage on Spoke-A, the relay captures the secret and submits it to Spoke-B — completing the atomic swap without any manual intervention.

---

## Structure

```
interop/
├── hub-and-spoke/      Cross-spoke connectors
│   ├── cacti/          Hyperledger Cacti relay (primary cross-spoke bridge)
│   ├── ccip/           Chainlink CCIP connector (alternative bridge)
│   └── noc/            NOC agent integration for relay monitoring
└── single-ledger/      Single-ledger orchestration (alternative topology)
```

---

## hub-and-spoke / Cacti

The primary cross-spoke relay is built on **Hyperledger Cacti** — a pluggable framework for cross-chain transaction coordination.

**What it does:**
- Subscribes to HTLC contract events on both Spoke-A and Spoke-B.
- When a `SecretRevealed` event is detected on one spoke, extracts the preimage.
- Submits the preimage to the counterpart HTLC contract on the other spoke.
- Provides settlement finality: both sides of the swap complete atomically or not at all.

**Key characteristics:**
- Event-driven: no polling loops — reacts to on-chain events.
- Durable: persists relay state to survive restarts without missing events.
- Configurable spoke endpoints: each deployment points to its two spoke RPC URLs.

---

## hub-and-spoke / CCIP

An alternative relay implementation using **Chainlink CCIP** (Cross-Chain Interoperability Protocol). Provides the same secret-bridging capability via Chainlink's message-passing infrastructure. Planned for environments where Chainlink oracle infrastructure is available.

---

## single-ledger

Reserved for orchestration scenarios where both counterparties share a single ledger (no cross-chain bridge required). Used for intra-spoke settlement testing.

---

## Make Targets

```bash
make relay-up      # Start the Cacti HTLC cross-spoke relay
make relay-down    # Stop the relay
```

---

## Related

- [payment-orchestrator](../backend/services/payment-orchestrator/README.md) — initiates HTLC locks that the relay bridges
- [contracts › HashTimeLockedContract](../contracts/README.md) — the on-chain contract the relay interacts with
- [backend › noc-agent](../backend/services/noc-agent/README.md) — monitors relay health
