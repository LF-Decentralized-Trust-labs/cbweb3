# payments

> [scenario-a](../../../README.md) › [backend](../../README.md) › payments

> **Status: Planned — directory stub only, not yet implemented.**

The **payments** service will extract core payment domain logic from the payment orchestrator into a focused domain service, separating orchestration concerns from business rules.

---

## Planned Responsibilities

- Validate payment preconditions (limits, compliance clearance, balance checks).
- Maintain a payment event ledger for audit and reconciliation.
- Expose gRPC RPCs for payment initiation and status queries.
- Implement idempotency for payment operations.

---

## Current State

Payment logic is currently embedded in the [payment-orchestrator](../payment-orchestrator/README.md). This service will refactor the domain layer out of the orchestrator as the platform matures.

---

## Related

- [payment-orchestrator](../payment-orchestrator/README.md) — currently owns the payment logic
- [compliance](../compliance/README.md) — will provide clearance checks to this service
