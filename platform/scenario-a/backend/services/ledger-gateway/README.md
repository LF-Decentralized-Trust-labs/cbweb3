# ledger-gateway

> [scenario-a](../../../README.md) › [backend](../../README.md) › ledger-gateway

> **Status: Planned — directory stub only, not yet implemented.**

The **ledger-gateway** will provide a unified abstraction over the Besu blockchain RPC/WebSocket interface, decoupling services from direct JSON-RPC calls.

---

## Planned Responsibilities

- Wrap Besu JSON-RPC and WebSocket subscriptions behind a gRPC interface.
- Manage connection pooling and retry logic for blockchain calls.
- Provide event subscription streams (block headers, contract events) to consuming services.
- Abstract contract ABI encoding/decoding so services work with typed structs rather than raw hex.

---

## Current State

Direct Besu interactions (contract calls, event polling) are currently embedded within the [auth](../auth/README.md), [compliance](../compliance/README.md), and [payment-orchestrator](../payment-orchestrator/README.md) services via the shared `backend/shared/blockchain` library. The ledger-gateway will centralize and standardize these interactions.

---

## Related

- [payment-orchestrator](../payment-orchestrator/README.md) — primary consumer of blockchain reads/writes
- [compliance](../compliance/README.md) — writes to IdentityRegistry via Besu
- [backend › shared](../../README.md) — current blockchain client shared library
