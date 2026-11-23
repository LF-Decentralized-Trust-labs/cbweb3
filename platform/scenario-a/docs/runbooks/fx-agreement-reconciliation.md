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

## Pending on-chain operations

On Paladin v1.0.0 a private transaction can stay pending indefinitely and cannot be cancelled
(upstream LFDT-Paladin/paladin#1356). When the orchestrator submits an FX operation (propose,
accept, reject, cancel, settle) and cannot learn its outcome, it does not report a plain
failure. It answers `409` with `trade_id=<id>` and, when known, the Paladin transaction id, and
records the operation on the agreement as pending (`pending_op`, `pending_tx_id`,
`pending_since` in `fx_agreements`).

While an operation is pending:

- Any other operation on the same agreement is refused with `409` ("still pending"), so no second
  private transaction is queued behind the first.
- A pending propose is stored with state `INVALID`. The relay forwards only `PROPOSED`
  agreements, so it is not propagated until the chain confirms it.

The pending-operation reconciler (`FX_PENDING_RECONCILE_INTERVAL_SEC`, default 30 s) resolves each
one:

| Paladin reports | Result |
| --- | --- |
| A successful receipt | The on-chain state is applied to the record (for example `INVALID` to `PROPOSED`, or `PROPOSED` to `ACCEPTED`) and the pending operation is cleared. |
| A failed receipt | The pending operation is cleared and the state is unchanged; the agreement can be operated on again. |
| No receipt yet | Nothing changes. After 10 minutes the reconciler logs `an FX operation has been pending for a long time` (at most once an hour per agreement). |
| No transaction id was returned | The reconciler waits until the chain shows the operation's target state. |

Every change is recorded as an audit event with source `SYSTEM_JOB` and actor
`fx-pending-reconciler`.

To find pending operations:

```sql
SELECT trade_id, state, pending_op, pending_tx_id, pending_since
FROM fx_agreements
WHERE pending_op IS NOT NULL AND pending_op <> ''
ORDER BY pending_since;
```

If an operation stays pending, look up `pending_tx_id` in the node's pending transactions
(`ptx_queryPendingTransactions`). Do not resubmit the operation and do not clear the columns by
hand while the transaction may still commit.

## Post-Recovery Verification

1. Agreement state converged on both spokes.
2. Audit sequence converged and ordered.
3. Relay retry queue drained (`pending=0`).
4. No new divergence in the next polling window.
