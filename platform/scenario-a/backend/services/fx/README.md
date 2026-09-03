# fx

> [scenario-a](../../../README.md) › [backend](../../README.md) › fx

> **Status: Planned — directory stub only, not yet implemented.**

The **fx** service will handle FX rate computation and price feed management as a dedicated microservice, decoupling pricing logic from the payment orchestrator.

---

## Planned Responsibilities

- Aggregate FX rates from on-chain oracles (`ManualOracle.sol`) and off-chain price feeds.
- Expose gRPC RPCs for quote requests and rate subscriptions.
- Drive the `ManualOracle` contract with rate updates authorized by the Central Bank.

---

## Current State

The FX pricing logic for Scenario A is currently handled inline by the [payment-orchestrator](../payment-orchestrator/README.md) via direct contract calls to `FXAgreement.sol` and `ManualOracle.sol`. This service will extract and centralize that logic in a future phase.

---

## Related

- [payment-orchestrator](../payment-orchestrator/README.md) — currently owns FX logic
- [contracts › ManualOracle](../../../contracts/README.md) — on-chain price oracle
- [contracts › AutomatedMarketMaker](../../../contracts/README.md) — AMM pool for Scenario B
