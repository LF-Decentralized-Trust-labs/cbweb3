# FX Agreement Reconciliation Runbook

## Purpose

Detect and reconcile cross-spoke divergence for FX agreement lifecycle and relay delivery.

## Preconditions

- Payment Orchestrator and API Gateway are running on both spokes.
- Relay is running with persistent store enabled.
- Internal relay auth secret is aligned between relay and API Gateways.

## Baseline Checks

1. Validate service health:
- `GET /api/v1/health` on API Gateway.
- Relay process health logs (`retry-stats` lines).

2. Validate DB persistence:
- Confirm `fx_agreements` and `fx_agreement_events` contain the target `trade_id`.

3. Validate audit consistency:
- Call `GET /api/v1/payments/fx/agreements/{tradeId}/audit` on both spokes.
- Compare event sequence (`from_state`, `to_state`, `occurred_at_unix`, `source`).

## Divergence Diagnosis

1. State mismatch between spokes:
- Query `GET /api/v1/payments/fx/agreements/{tradeId}` on both sides.
- Compare terminal/non-terminal state and `updated_at` equivalent timeline from audit.

2. Missing relay propagation:
- Check relay logs for `forward` failures and retry entries.
- Inspect relay store JSON for pending retries for the same `trade_id`.

3. HTLC lock refusal in strict mode:
- Verify agreement is `ACCEPTED` and unexpired.
- Verify receiver/amount match spoke leg.
- If fallback gate is used, confirm commitment registration happened after acceptance.

## Recovery Actions

1. If retry queue is pending:
- Keep relay running until `pending=0`.
- Do not clear relay store manually unless corruption is confirmed.

2. If one spoke missed transition permanently:
- Replay transition via orchestrator command (`accept/reject/cancel/settle`) using idempotent flow.
- Confirm new audit event appears on both sides.

3. If strict mode blocks valid lock:
- Revalidate `agreement_id` and agreement record terms.
- Re-run acceptance path to refresh private context and commitment registration when applicable.

## Post-Recovery Verification

1. Agreement state converged on both spokes.
2. Audit sequence converged and ordered.
3. Relay retry queue drained (`pending=0`).
4. No new divergence in the next polling window.
