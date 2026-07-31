# Slippage residue handling — decision record

**Status:** decided and implemented
**Scope:** Scenario B cross-currency swap (`api-gateway` orchestrator + issuing-CB endpoint)
**Depends on:** realized `amount_in` decoded from `LogSwap` (PR #68). Before it, the stored
`amount_in` echoed `MaxAmountIn` and the residue could not even be computed.

## The problem

Step 1 (bridge-in) moves `MaxAmountIn` — the desired amount **plus the slippage buffer** — from the
source spoke to the Hub, because the true cost is unknown until the AMM swap runs. Step 2 consumes
only the realized `amount_in`. The difference stayed on the initiating gateway's Hub swap signer,
with no step returning it. The payer's burned/locked tCeBM therefore exceeded what the payment used,
on **every** swap.

## Options considered

**(a) Automatic bridge-back.** After the swap, burn the unspent wrapped source on the Hub and
deliver the native token back to the payer on the source spoke.

**(b) Credit / tracked balance.** Leave the residue on the hub signer as a tracked, reusable balance
credited toward the bank's next bridge-in.

## Decision: (a), executed as an asynchronous, retryable leg

Three reasons, in order of weight:

1. **(b) is already the de-facto behaviour, and it is worse than plain stranding.** The hub signer
   is a single per-gateway address (`SIGNER_PRIVATE_KEY`), shared by every bank the CB serves. Because
   the AMM pulls the input from that address, one bank's residue is silently spendable by another
   bank's swap. The value is not idle; it is **fungibly mutualised with no attribution**. Adopting
   (b) properly would require building the per-bank bookkeeping that does not exist, so (b) is not
   the cheaper option — it only looks cheaper while the accounting is missing.

2. **(b) keeps reserves encumbered indefinitely.** The payer's native tCeBM stays burned on the
   source spoke to back a Hub balance it never used. For a central-bank platform that is a
   prudential problem, not merely an accounting one.

3. **Constitution, §Atomicity:** partial settlement is forbidden in production paths. A debit that
   exceeds the value delivered is partial settlement under another name.

### Why asynchronous

The return must not sit in the payment's critical path. When it executes, the payment has already
settled (Step 3 dispatched), so a failure in the return must never fail the swap. It is enqueued as
an independent bridge position with its own lifecycle, driven by the existing relayer.

### Why it also runs when Step 3 fails

The unspent input was never owed to anyone. If the bridge-out fails, skipping the return would leave
the payer debited for the full cap *on top of* an undelivered payment — the worst of both. So the
residue return runs on the bridge-out failure path too, while the swap's `FAILED` verdict stands.

## How it works

```
Step 1  bridge-in   MaxAmountIn → Hub (wrapped source on the gateway's hub signer)
Step 2  swap AMM    consumes the realized amount_in (LogSwap)
Step 3  bridge-out  AmountOut → beneficiary spoke via the beneficiary CB
Step 4  residue     residue = MaxAmountIn − realized amount_in
          ├─ sovereign: signed POST → issuing CB of the source currency
          └─ local:     enqueued directly (this gateway *is* the issuing CB)
                        → burn wrapped source on Hub + mint native to the payer
```

Step 4 mirrors Step 1's split: only the issuing CB of the source currency holds
`CENTRAL_BANK_ROLE` on the wrapped source token, so the return is delegated over the same direct CB
channel the bridge-in already uses (no Cacti hop — same jurisdiction).

### The amount is derived, never accepted

`POST /internal/amm/cross-currency-residue-return` carries **no amount field**. The issuing CB
computes it from two sources the caller cannot forge:

- the amount bridged in, read from the bridge-in position **the CB itself created**;
- the realized `amount_in`, decoded from the `LogSwap` of the trusted AMM on the Hub.

`residue = position.mirrored_amount − LogSwap.amountIn`

Additional gates: the swap's *input* token must be this CB's wrapped token; the swap's `msg.sender`
must match the address the CB minted to; the payer's spoke wallet is resolved from the CB's own
participants registry; consumption exceeding the bridged amount is refused as a reconciliation event
rather than paid out; and each swap is refundable at most once. This is the same trust model as the
R2-CR-6 bridge-out endpoint.

### Replay protection required a schema change

Replay protection was a unique index on `swap_tx_hash` alone. A swap now legitimately produces two
positions — the settlement and the residue return — and in a single-CB deployment both land in the
same table, so the old index would have made the residue leg look like a replay of the settlement
and silently dropped it.

The index is now unique on `(swap_tx_hash, leg)` with `leg ∈ {SETTLEMENT, RESIDUE}`. The replacement
is created by `AutoMigrate` **before** the legacy index is dropped, and the drop is refused if the
replacement is absent — replay protection is never absent, not even mid-migration.

> **Deploy constraint:** this version has no rollback. The previous binary queries
> `swap_tx_hash` without a leg filter, so once a `RESIDUE` row exists it can be mistaken for an
> already-consumed settlement, causing legitimate bridge-outs to be rejected as replays.

## Deliberate deviation from the ticket wording

The ticket asks to *"reconcile the `BridgedAssetPosition` records so they reflect the true consumed
amount, not the bridged max."*

The bridge-in position's `mirrored_amount` is **not** rewritten. It records what the chain actually
moved and is audit evidence; overwriting it with a number that was never on-chain destroys the
trail. Instead, net consumption is **derived** from the pair of positions, linked by
`parent_position_id`:

```
net_consumed = bridge_in.mirrored_amount − Σ residue_legs.mirrored_amount
```

If the intent was literally to update the field, that remains open for confirmation with the ticket
author — the change is small.

## Side effect fixed along the way

The daily transfer limit reserved the worst case (`MaxAmountIn`) before Step 1 and only restored it
when the swap ended `FAILED` — so a successful swap permanently consumed the buffer out of the bank's
quota. The unused part is now released once the real cost is known, and `quotaReleased` ensures the
failure path restores only the remainder instead of the buffer twice.

## Verification

**Unit:** residue-specific cases across orchestrator, CB handler, repository/idempotency and
migration; suites green in `api-gateway` and `payment-orchestrator`.

**End-to-end** (`tests/e2e/residue-return`, build tag `e2e`, skips without a live stack) — run
against a clean stack:

| check | result |
|---|---|
| `residue_amount` == `max_amount_in` − realized | `10254832589857970570` = `15382248884786955855` − `5127416294928985285` |
| payer's net on-chain debit == realized `amount_in` | `5127416294928985285`, cap was `15382248884786955855` |
| hub signer wrapped-source balance | `0` before, `0` after — no accumulation |
| residue leg reaches `RELEASED` | yes, linked to its bridge-in parent |
| replay of the return | `duplicate`, no second burn, balance unmoved |

Known limitations and follow-up work are tracked as separate tickets rather than in this record.
