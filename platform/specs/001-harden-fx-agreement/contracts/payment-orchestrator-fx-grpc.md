# gRPC Contract Notes: Payment Orchestrator FX Agreement

## Scope

This document captures contract-level deltas required to support production-grade FX Agreement lifecycle.

## Existing RPC Surface (kept)

- `ProposeFXAgreement`
- `AcceptFXAgreement`
- `RejectFXAgreement`
- `CancelFXAgreement`
- `SettleFXAgreement`
- `GetFXAgreement`
- `ListFXAgreements`

No breaking rename is planned.

## Message Evolution

### FXAgreement

Add optional fields for bilateral private context metadata:
- `group_id` (string)
- `contract_address` (string)

Behavior:
- Nullable/empty while Pente flow is disabled or during migration.
- Required by policy when private lifecycle mode is enabled.

### Error Semantics (non-breaking)

Standardize failed-precondition responses for lock validation:
- Agreement not accepted
- Agreement expired
- Agreement terms mismatch for caller spoke
- Missing linkage when production strict mode is enabled

## Relay and Internal Synchronization Expectations

- Cross-spoke forwarding remains idempotent at business transition level.
- `on_behalf` flow remains supported for relayed transitions.
- Terminal states `CANCELLED` and `SETTLED` must be accepted and mirrored by counterpart service.

## Compatibility Rules

- Backward compatible wire format: new fields are additive only.
- Existing clients that ignore unknown fields remain functional.
- During rollout, services must tolerate records with missing private-context fields.

## Validation Policy Alignment

`LockHTLC` and `LockHTLCWithHashLock` must enforce:
- `trade_id` exists.
- agreement state is `ACCEPTED`.
- agreement is not expired.
- receiver/amount match agreement leg for caller spoke.

These checks remain enforced even if on-chain gate is still in transition.
