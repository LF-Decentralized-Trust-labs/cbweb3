# Data Model: Harden FX Agreement for Production

## Entity: FXAgreement

Represents the bilateral pre-trade agreement lifecycle.

### Fields
- `trade_id` (string, PK, immutable)
- `originator` (string, required)
- `counterparty_b` (string, required)
- `settlement_agent` (string, required)
- `custodian` (string, required)
- `beneficiary` (string, required)
- `origin_amount` (decimal string, required, > 0)
- `counter_amount` (decimal string, required, > 0)
- `origin_currency` (ISO 4217, required)
- `counter_currency` (ISO 4217, required)
- `rate` (decimal string, required, consistent with amount ratio tolerance)
- `spoke_a_receiver` (Paladin identity, required)
- `spoke_b_receiver` (Paladin identity, required)
- `expiry_date` (unix seconds, required, must be future at proposal)
- `state` (enum: `PROPOSED|ACCEPTED|REJECTED|CANCELLED|SETTLED`)
- `on_chain_tx_hash` (string, optional)
- `group_id` (string, optional, required when Pente enabled)
- `contract_address` (string, optional, required when Pente enabled)
- `created_at` (timestamp)
- `updated_at` (timestamp)

### Validation Rules
- `trade_id` unique globally per spoke domain.
- `origin_currency != counter_currency`.
- `expiry_date > now()` at proposal.
- `rate` must match `counter_amount / origin_amount` within configured tolerance.
- receivers must map to expected spoke context.

## Entity: FXAgreementEvent

Append-only audit event for lifecycle transitions.

### Fields
- `id` (bigint, PK)
- `trade_id` (string, FK -> FXAgreement.trade_id)
- `from_state` (enum)
- `to_state` (enum)
- `actor` (string; user/service identity)
- `occurred_at` (timestamp)
- `notes` (text, optional)
- `tx_hash` (string, optional)
- `source` (enum: `LOCAL_API|RELAY|SYSTEM_JOB|ON_BEHALF`)

### Invariants
- Events are immutable after insert.
- For a given `trade_id`, event order is monotonic by `occurred_at` and id.

## Entity: RelayDeliveryRecord

Tracks cross-spoke forwarding and retries.

### Fields
- `id` (bigint, PK)
- `idempotency_key` (string, unique, required)
- `trade_id` (string, required)
- `event_type` (enum: `PROPOSED|ACCEPTED|REJECTED|CANCELLED|SETTLED`)
- `source_spoke` (string)
- `target_spoke` (string)
- `status` (enum: `PENDING|RETRYING|DELIVERED|FAILED`)
- `attempt_count` (int, default 0)
- `next_retry_at` (timestamp, nullable)
- `last_error` (text, nullable)
- `created_at` (timestamp)
- `updated_at` (timestamp)

### Invariants
- `idempotency_key` guarantees at-most-once semantic at business transition level.
- `DELIVERED` is terminal for forwarding record.

## Entity: HTLCAgreementLink

Logical validation context between HTLC lock requests and agreement terms.

### Fields
- `agreement_trade_id` (string, FK)
- `lock_receiver` (string)
- `lock_amount` (decimal string)
- `caller_spoke` (string)
- `validated_at` (timestamp)
- `validation_result` (enum: `ALLOW|DENY`)
- `reason_code` (string)

### Validation Rules
- Allow only when agreement exists, is `ACCEPTED`, and not expired.
- Amount and receiver must match agreement leg for caller spoke.

## Relationships
- One `FXAgreement` has many `FXAgreementEvent`.
- One `FXAgreement` has many `RelayDeliveryRecord` entries across lifecycle.
- One `FXAgreement` may have many `HTLCAgreementLink` validation attempts.

## State Transitions

### Agreement Lifecycle
- `PROPOSED -> ACCEPTED`
- `PROPOSED -> REJECTED`
- `PROPOSED -> CANCELLED` (manually by originator)
- `PROPOSED -> CANCELLED` (automatic expiry job)
- `ACCEPTED -> SETTLED`
- `ACCEPTED -> CANCELLED` (originator cancels before HTLC lock, or HTLC lock times out without both legs completing — only `CancelFXAgreement` with originator authority is permitted; relay must propagate this transition to counterpart spoke)

Invalid transitions are rejected and audited.

### Relay Lifecycle
- `PENDING -> DELIVERED`
- `PENDING -> RETRYING`
- `RETRYING -> DELIVERED`
- `RETRYING -> FAILED` (after max attempts)

### Consistency Rules
- Terminal agreement states cannot be overridden by stale relayed transitions.
- Duplicate deliveries resolve as no-op by idempotency key.
