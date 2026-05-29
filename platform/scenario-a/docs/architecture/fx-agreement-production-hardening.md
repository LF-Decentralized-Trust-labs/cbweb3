# FX Agreement Production Hardening Architecture

## Objective

Establish a production-ready FX agreement lifecycle with durable persistence, auditable state transitions, authenticated relay synchronization, bilateral private context support, and strict HTLC linkage.

## Components

- Payment Orchestrator:
  - Owns lifecycle commands (`propose`, `accept`, `reject`, `cancel`, `settle`).
  - Persists current agreement state in `fx_agreements`.
  - Persists append-only transitions in `fx_agreement_events`.
  - Enforces strict HTLC preconditions in service layer.
- PostgreSQL:
  - Stores current state (`fx_agreements`) and immutable audit events (`fx_agreement_events`).
  - Stores relay delivery durability (`relay_delivery_records`).
- API Gateway:
  - Exposes public FX endpoints and audit endpoint (`/api/v1/payments/fx/agreements/:tradeId/audit`).
  - Protects internal relay polling route using `X-Relay-Auth`.
- Cacti Relay:
  - Polls internal FX state and forwards lifecycle transitions cross-spoke.
  - Uses persistent dedup/retry store (`relay-store.ts`) for restart-safe synchronization.
- HTLC Contract:
  - Keeps existing agreement gate when `FX_AGREEMENT` is configured.
  - Adds fallback commitment gate (`registerAgreementCommitment`) for strict mode without FX agreement contract.
- Pente Adapter:
  - Optional HTTP integration for bilateral private context (`group_id`, `contract_address`) during acceptance flow.

## Data Model and State

- Agreement terminal states: `REJECTED`, `CANCELLED`, `SETTLED`.
- Allowed key transitions:
  - `PROPOSED -> ACCEPTED`
  - `PROPOSED -> REJECTED`
  - `PROPOSED -> CANCELLED`
  - `ACCEPTED -> CANCELLED`
  - `ACCEPTED -> SETTLED`
- Each transition emits an immutable audit event with source (`LOCAL_API`, `RELAY`, `SYSTEM_JOB`, `ON_BEHALF`).

## Strict HTLC Enforcement

- `FX_AGREEMENT_HTLC_STRICT=true` enforces:
  - `agreement_id` required.
  - Agreement must exist, be `ACCEPTED`, and not expired.
  - Receiver and amount must match the spoke leg.
- On-chain gate behavior:
  - If `FX_AGREEMENT` adapter is active, use trade ID gate.
  - Otherwise, use fallback commitment `keccak256(tradeId|originAmount|counterAmount|rate)`.

## Operational Notes

- Expiry worker auto-cancels expired non-terminal agreements.
- Relay emits retry metrics (`pending`, `max_lag_ms`) for reconciliation visibility.
- Pente integration is optional and controlled by env (`PENTE_ENABLED`).
