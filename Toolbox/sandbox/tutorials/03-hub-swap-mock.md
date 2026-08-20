# Tutorial 3 — Scenario B: cross-currency hub swap against a mock

**Time:** ~25 minutes · **Prerequisites:** Node.js 18+, `curl`, `jq` · **Scenario:** B
(hub-and-spoke, "International Hub")

Scenario B settles a cross-border payment through a Hub AMM instead of a pair of HTLCs.
You will walk the whole thing: unauthenticated discovery, the mandatory reserve prelude,
corridor provisioning, quote → swap, and — the part that turns an API test into a
settlement test — residue and reconciliation verification.

Every path, field name and status code below is taken from
[`Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml`](../../contracts/amm/openapi_amm_v2.3.0.yaml).

> **Scenario B has no HTLC endpoints and no FX agreement.** Not one of the Scenario B
> gateway's paths contains `htlc`. The `LockHTLC*` schemas that appear in the Scenario B
> platform document are unreferenced copy-paste residue and are deliberately not published
> by the Toolbox. If you are integrating with Scenario A, go to
> [tutorial 01](01-pvp-settlement-mock.md).

---

## Setup

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml --port 4011
```

```bash
export AMM=http://localhost:4011
export COOKIE='Cookie: access_token=SYNTHETIC_COOKIE_CI'
export JSON='Content-Type: application/json'
```

---

## Who does what

The commercial bank makes **one** settlement call. It fans out into four relay-only steps
across two Central Bank gateways.

```
  Bank (Spoke BRL)        Its Central Bank        Hub AMM        Beneficiary's Central Bank
        │                        │                   │                      │
        │ POST /api/v2/amm/swap/cross-currency       │                      │
        ├───────────>│           │                   │                      │
        │            │ bridge-in: lock spoke tCeBM,  │                      │
        │            │ mint Hub W-token              │                      │
        │            ├──────────────────────────────>│                      │
        │            │           │  Hub swap, signed by that same CB        │
        │            │           │                   │                      │
        │            │           │  bridge-out: the BENEFICIARY's CB burns  │
        │            │           │  the W-token and delivers on its spoke   │
        │            │           │                   ├─────────────────────>│
        │            │           │                   │                      │
        │            │  residue return: the unspent slippage buffer comes   │
        │            │  back to the payer as a SEPARATE bridge position     │
        │<───────────┤           │                   │                      │
```

The commercial bank never holds a W-token and never signs on the Hub. CB-A burning CB-B's
tokens would be a sovereignty breach, which is why the *beneficiary's* Central Bank
performs the bridge-out. That is also why one `POST` produces asynchronous state on
gateways you never called, and why **partial failure is expressible**.

---

## Step 1 — Discovery, with no credentials at all

Twelve Scenario B operations are public. Note the absence of `-H "$COOKIE"` throughout this
step.

```bash
curl -s "$AMM/api/v2/amm/hub-config" | jq .
```
```json
{
  "amm_address": "0x1234abcd...",
  "pair_registry": "0x5678ef01...",
  "currency_registry": "0x9abc2345..."
}
```

```bash
curl -s "$AMM/api/v2/hub/currencies" | jq .
```
```json
{
  "currencies": [
    { "symbol": "W-tCeBM_BRL", "country_name": "Brazil",    "token_address": "0x1234abcd...", "proposer_cb": "central_bank_a" },
    { "symbol": "W-tCeBM_ARS", "country_name": "Argentina", "token_address": "0x5678ef01...", "proposer_cb": "central_bank_b" }
  ]
}
```

```bash
curl -s "$AMM/api/v2/amm/pairs" | jq .
```
```json
{
  "pairs": [
    {
      "pair_id": "W-tCeBM_BRL/W-tCeBM_ARS",
      "status": "ACTIVE",
      "token_a_address": "0x1234abcd...",
      "token_b_address": "0x5678ef01...",
      "amm_address": "0x9abc2345...",
      "proposer_cb": "central_bank_a",
      "confirmer_cb": "central_bank_b",
      "proposed_at": "2026-06-17T14:00:00Z",
      "confirmed_at": "2026-06-17T14:20:00Z"
    }
  ]
}
```

> **Never construct a pool-pair identifier.** Five incompatible spellings appear across
> the delivered artefacts — `tCeBM_BRL-tCeBM_ARS`, `W-tCeBM_BRL/W-tCeBM_ARS`, `W-BRL-ARS`,
> the quote-style `W-BRL-W-ARS` and the circuit breaker's `BRL-USD` fallback — and **none is
> documented as canonical**. All five are enumerated with source citations in
> [`../../DIVERGENCES.md`](../../DIVERGENCES.md). Treat `pair_id` as an opaque string taken
> verbatim from this call and passed through unmodified.
>
> The mock fixtures under `mocks/amm/` use the sovereign form (`W-CRC-CLP`) because the pool
> route is a single path segment; the examples in this tutorial use the slash form with
> `%2F`. Both are real; neither is canonical.

> **Spec divergence, mirrored.** `hub/currencies`, `amm/pairs` and `bridge/positions` are
> documented upstream as bare JSON arrays but actually return wrapped objects. The Toolbox
> models the wrapped form, which is what a client receives. This is annotated in the
> contract, not silently corrected.

```bash
PAIR=$(curl -s "$AMM/api/v2/amm/pairs" | jq -r '.pairs[0].pair_id')
curl -s --get "$AMM/api/v2/amm/pool/$PAIR/status" | jq .
```
```json
{
  "pool_pair": "W-tCeBM_BRL/W-tCeBM_ARS",
  "pool_status": "ACTIVE",
  "reserve_a": "50000000000000000000000",
  "reserve_b": "42500000000000000000000",
  "current_ratio": 0.85,
  "imbalance_flag": false,
  "fee_rate_bps": 30,
  "total_lp_count": 2,
  "pending_commits": [],
  "updated_at": "2026-06-18T09:58:00Z"
}
```

`pool_status` must be `ACTIVE` before you swap. The other two values are `EMPTY` and
`PENDING_COUNTERPART` — a pool with liquidity on only one side.

```bash
curl -s --get "$AMM/api/v2/governance/circuit-breaker/status" \
  --data-urlencode "pair=$PAIR" | jq .
# → {"pair":"W-tCeBM_BRL/W-tCeBM_ARS","state":"LIVE"}
```

> **Always pass `pair`.** Omitting it does not report "the network". The handler
> substitutes the literal `BRL-USD`, which on most deployments is not a registered pair —
> so an omitted parameter can return a healthy-looking `LIVE` for a pair nobody trades.
> The parameter is undeclared upstream; the Toolbox publishes it as
> `x-cbweb3-source: implementation-observed`.

---

## Step 2 — The reserve prelude (mandatory, and enforced)

A swap cannot bridge in tokens the bank does not hold. The bridge-in handler refuses with
*"insufficient tokenized reserves: bank %s holds %s tCeBM, need %s — complete Reserve
Tokenisation first"*.

```bash
curl -s -X POST "$AMM/api/v1/payments/deposits" \
  -H "$JSON" -H "$COOKIE" -d '{"amount":"10000000"}' | jq .
# → {"deposit_id":"dep-a1b2c3d4"}

curl -s -X POST "$AMM/api/v1/payments/deposits/approve" \
  -H "$JSON" -H "$COOKIE" -d '{"deposit_id":"dep-a1b2c3d4"}' | jq .
# → 201 {"status":"approved"}

curl -s -X POST "$AMM/api/v1/payments/deposits/exchange" \
  -H "$JSON" -H "$COOKIE" -d '{"deposit_id":"dep-a1b2c3d4"}' | jq .
# → 201 {"mint_tx_hash":"0x1234..."}

curl -s -X POST "$AMM/api/v1/payments/escrows" \
  -H "$JSON" -H "$COOKIE" -d '{"amount":"5000000","deposit_id":"dep-a1b2c3d4"}' | jq .
# → {"escrow_id":"esc-a1b2c3d4"}

curl -s -X POST "$AMM/api/v1/payments/escrows/approve" \
  -H "$JSON" -H "$COOKIE" -d '{"escrow_id":"esc-a1b2c3d4"}' | jq .
# → 201 {"burn_tx_hash":"0x1234...","mint_tx_hash":"0x5678..."}

curl -s "$AMM/api/v1/token/balance" -H "$COOKIE" | jq .
# → {"balance":"9000000"}
```

> **Three Scenario A/B divergences hide in that block. All are the platform's, not ours.**
> 1. The fiat exchange path is `deposits/`**`exchange`** here and
>    `deposits/`**`fiat-exchange`** in Scenario A. Neither gateway serves both.
> 2. Scenario B's escrow approval returns `mint_tx_hash`; Scenario A's returns
>    `zeto_mint_tx_hash`. Records differ too: `requester_id` here,
>    `requester_paladin_identity` there.
> 3. Scenario B's `POST /api/v1/payments/escrows` accepts a `deposit_id`; Scenario A's does
>    not.
>
> This is exactly why the reserve lifecycle is **duplicated** across the two contracts
> rather than extracted into a shared one. A shared document would have to declare both
> spellings and would therefore describe a gateway that does not exist.

---

## Step 3 — Corridor provisioning (Central Bank work, once per corridor)

You will not do this as a commercial bank, but a client that reads pool state needs to know
what produced it. Every call in this step requires a Central Bank session.

```bash
# 1. Register the wrapped currency on the Hub
curl -s -X POST "$AMM/api/v2/hub/currencies" -H "$JSON" -H "$COOKIE" \
  -d '{"symbol":"W-tCeBM_BRL","country_name":"Brazil","token_address":"0x1234abcd...","proposer_cb":"central_bank_a"}' | jq .

# 2. The CB of token A proposes the pair
curl -s -X POST "$AMM/api/v2/amm/pairs/propose" -H "$JSON" -H "$COOKIE" \
  -d '{"pair_id":"W-tCeBM_BRL/W-tCeBM_ARS","token_a_address":"0x1234abcd...","token_b_address":"0x5678ef01...","proposer_cb":"central_bank_a"}' | jq .
# → 201 {"pair_id":"...","status":"PROPOSED", ...}

# 3. The CB of token B confirms it — this is what makes the pair ACTIVE
curl -s -X POST "$AMM/api/v2/amm/pairs/confirm" -H "$JSON" -H "$COOKIE" \
  -d '{"pair_id":"W-tCeBM_BRL/W-tCeBM_ARS","confirmer_cb":"central_bank_b"}' | jq .
# → 200 {"pair_id":"...","status":"ACTIVE", ...}

# 4. Bridge spoke tCeBM in, minting the Hub W-token
curl -s -X POST "$AMM/api/v2/bridge/lock-mint" -H "$JSON" -H "$COOKIE" \
  -d '{"amount":"1000000000000000000000"}' | jq .
# → 201 {"position_id":"9e1c4b77-...","bridge_state":"LOCKING", ...}

# 5. Mint and approve the AMM to spend
curl -s -X POST "$AMM/api/v2/amm/token/mint-and-approve" -H "$JSON" -H "$COOKIE" \
  -d '{"amount":"1000000000000000000000","pool_pair":"W-tCeBM_BRL/W-tCeBM_ARS","side":"A"}' | jq .
# → 200 {"status":"ok","amount":"1000000000000000000000"}

# 6. Commit liquidity, one side at a time
curl -s -X POST "$AMM/api/v2/amm/liquidity/commit" -H "$JSON" -H "$COOKIE" \
  -d '{"pool_pair":"W-tCeBM_BRL/W-tCeBM_ARS","amount":"1000000000000000000000"}' | jq .
# → {"commit_id":"cmt-3f8a2b19","status":"PENDING"}

# 7. Poll until EXECUTED
curl -s "$AMM/api/v2/amm/liquidity/commits/cmt-3f8a2b19" -H "$COOKIE" | jq .
# → {"commit_id":"cmt-3f8a2b19", ..., "status":"EXECUTED"}
```

> **Two `sovereignty` gates that return `403`, not `401`:** `pairs/propose` rejects a caller
> that is not the central bank of token A (`NOT_CENTRAL_BANK_OF_TOKEN_A`), and
> `pairs/confirm` rejects a caller that is not the central bank of token B
> (`NOT_CENTRAL_BANK_OF_TOKEN_B`). Neither is a permission bug — it is the design.

> **`provider_id` and `side` on `liquidity/commit` are deprecated backward-compatibility
> fields.** The handler requires only `pool_pair` and `amount`; the provider comes from the
> session and the side resolves on-chain. The upstream document marks them required, which
> is wrong, and the Toolbox contract corrects the required-set while keeping the fields
> declared and flagged `deprecated`.

---

## Step 4 — Quote, then swap. In one step.

```bash
curl -s --get "$AMM/api/v2/amm/quote/cross-currency" \
  --data-urlencode "source_currency=BRL" \
  --data-urlencode "target_currency=ARS" \
  --data-urlencode "amount_out=1000000000000000000000" | jq .
```
```json
{
  "quote_id": "q-7f3a91c2",
  "pool_pair": "W-tCeBM_BRL/W-tCeBM_ARS",
  "amount_out": "1000000000000000000000",
  "amount_in": "1176470588235294117647",
  "effective_rate": 0.85,
  "fee_bps": 30,
  "max_slippage_pct": 0.01,
  "reserve_a_snapshot": "50000000000000000000000",
  "reserve_b_snapshot": "42500000000000000000000",
  "created_at": 1711500000,
  "valid_until": 1711500015,
  "time_remaining_seconds": 15
}
```

> ### ⏱ The quote lives 15 seconds
>
> `valid_until − created_at` is **15**. A human walkthrough that pauses between the quote
> and the swap will simply expire. Issue both **programmatically, in one step**. That is
> why the conformance suite fetches a quote and swaps in the same test method rather than
> across two.

> **Spec divergence (blocking, mirrored not fixed).** The platform document declares
> **zero parameters** for this operation while the handler requires three. A client written
> strictly against the upstream document always gets `400`. The same class of defect
> affects `GET /api/v2/amm/quote/exact-output`, documented as taking `amountOut` while the
> handler reads `amount_out`. The Toolbox publishes what the handler reads — the platform
> surface wins — and footnotes each deviation so nobody mistakes it for invention.

Now the swap:

```bash
curl -s -X POST "$AMM/api/v2/amm/swap/cross-currency" \
  -H "$JSON" -H "$COOKIE" \
  -d '{
        "source_currency": "BRL",
        "target_currency": "ARS",
        "pool_pair": "W-tCeBM_BRL/W-tCeBM_ARS",
        "amount_out": "1000000000000000000000",
        "max_amount_in": "1200000000000000000000",
        "beneficiary_bank_id": "bank-galicia",
        "quote_id": "q-7f3a91c2"
      }' | jq .
```
```json
{
  "swap_id": "swp-1a2b3c4d",
  "correlation_id": "5f2a1c88-9d34-4f6a-b0e1-7c3d9a2b5e10",
  "status": "BRIDGE_IN_PROGRESS",
  "amount_in": "1176470588235294117647",
  "amount_out": "1000000000000000000000",
  "effective_rate": 0.85,
  "bridge_in_position_id": "9e1c4b77-2f10-4a8d-9c31-6b4e7d2a1f05",
  "created_at": "2026-06-18T10:00:00Z"
}
```

Two things that will bite you against a real gateway:

- **This endpoint is cookie-only** (`RequireCookieAuth`). The bearer token that works on
  twelve other `/api/v2` routes will **not** work here, nor on
  `GET /api/v2/amm/swap/cross-currency`.
- **Do not send `payer_bank_id`.** Caller identity is deprecated in the body and taken from
  the session.

> **Amounts on `/api/v2` are 18-decimal base units** and say so. Amounts on `/api/v1` are
> 7-digit strings with **no unit statement anywhere**. Moving a figure from a deposit into
> a swap without a conversion you have derived yourself is not safe. A compounding trap:
> `max_amount` on a transfer limit is a **human decimal on input** (`"100000.00"`) and
> **wei on output** (`"100000000000000000000000"`) — under the same field name.

---

## Step 5 — Poll to completion

```bash
curl -s "$AMM/api/v2/amm/swap/cross-currency/swp-1a2b3c4d" -H "$COOKIE" | jq .
```
```json
{
  "swap_id": "swp-1a2b3c4d",
  "correlation_id": "5f2a1c88-9d34-4f6a-b0e1-7c3d9a2b5e10",
  "status": "COMPLETED",
  "amount_in": "1176470588235294117647",
  "amount_out": "1000000000000000000000",
  "effective_rate": 0.85,
  "bridge_in_position_id": "9e1c4b77-2f10-4a8d-9c31-6b4e7d2a1f05",
  "swap_tx_hash": "0xabcd1234...",
  "bridge_out_position_id": "c4a9e7b1-5d2f-4c80-91ab-2e6f8d3a7c11",
  "residue_amount": "23529411764705882353",
  "residue_position_id": "b1d3f6a2-44c8-4e91-8f2a-3c7e5b1d90aa",
  "residue_status": "RETURNED",
  "created_at": "2026-06-18T10:00:00Z",
  "completed_at": "2026-06-18T10:00:41Z"
}
```

The intermediate vocabulary — `QUOTING`, `BRIDGE_IN_PROGRESS`, `SWAP_IN_PROGRESS`,
`BRIDGE_OUT_PROGRESS`, `COMPLETED`, `FAILED` — is **not in the platform's OpenAPI at all**.
It is recovered from Go domain code and published as documented strings, **not** as an enum
the platform has committed to. The same applies to bridge position state (`LOCKING`,
`ACTIVE`, `BURNING`, `BURNED`, `RELEASED`, `RECONCILIATION_REQUIRED`) — and the upstream
example values `PENDING` and `CLOSED` do not exist in the code at all.

---

## Step 6 — Settlement verification

**`status == COMPLETED` alone is not enough.** This is the step that separates a settlement
test from an API test.

| Assertion | Why it matters |
|---|---|
| The payer was debited **`amount_in`**, not `max_amount_in` | The full `max_amount_in` is bridged in *before* the swap runs, because the realized input is unknown until execution |
| `residue_status == "RETURNED"` | The unspent slippage buffer (here `23529411764705882353`) is burned on the Hub and returned to the payer on its source spoke as a **separate** bridge position with `leg=RESIDUE`. **`RETURN_FAILED` means the payer has not been made whole** |
| The bridge-out position reached `BURNED` / `RELEASED` | Delivery actually happened on the beneficiary's spoke |
| The beneficiary balance is non-zero | The payment arrived |
| `hub-reconciliation` reports `balanced == true` | Nothing is stranded on the Hub |

```bash
curl -s "$AMM/api/v2/bridge/positions" -H "$COOKIE" | jq .
```
```json
{
  "positions": [
    {
      "position_id": "9e1c4b77-2f10-4a8d-9c31-6b4e7d2a1f05",
      "owner_bank_id": "central_bank_a",
      "spoke_network": "spoke-brl",
      "native_asset": "tCeBM_BRL",
      "mirrored_asset": "W-tCeBM_BRL",
      "mirrored_amount": "1000000000000000000000",
      "bridge_state": "ACTIVE",
      "relayer_retries": 0
    }
  ]
}
```

```bash
curl -s "$AMM/api/v2/amm/hub-reconciliation" -H "$COOKIE" | jq .
```
```json
{
  "w_token": "0x1234abcd...",
  "holder_address": "0x5678ef01...",
  "on_chain_balance": "1000000000000000000000",
  "expected_in_flight": "1000000000000000000000",
  "unexplained": "0",
  "stranded_total": "0",
  "per_bank": [ { "owner_bank_id": "bank-itau", "amount": "1000000000000000000000", "positions": 1 } ],
  "stranded": [],
  "unattributed": [],
  "balanced": true
}
```

`GET /api/v2/bridge/positions` and `GET /api/v2/amm/hub-reconciliation` are two of the
twelve routes that accept **either** the cookie **or** a bearer token.

---

## Errors on `/api/v2`

Unlike `/api/v1`, the v2 handlers do emit a machine-readable code — under three different
spellings across the surface (`code`, `error_code`, and Scenario A's undocumented
`required_role` array). None of the 422 codes below is declared in the platform document;
they are observed in the delivered handlers and marked
`x-cbweb3-source: implementation-observed` in the contract.

| Situation | Status | Code |
|---|---|---|
| Daily transfer limit exhausted | `422` | `TRANSFER_LIMIT_EXCEEDED` |
| Pool has no liquidity on one side | `422` | `POOL_NOT_ACTIVE` |
| A Central Bank has paused the pair | `422` | `CIRCUIT_BREAKER_HALTED` |
| Not enough depth for the requested output | `422` | `INSUFFICIENT_POOL_LIQUIDITY` |
| No session cookie | `401` | `UNAUTHENTICATED` |
| **Hub swap succeeded, delivery did not** | `500` | `BRIDGE_OUT_FAILED` |

```bash
curl -s -H 'Prefer: code=422' -H "$COOKIE" -H "$JSON" \
  -X POST "$AMM/api/v2/amm/swap/cross-currency" \
  -d '{"source_currency":"BRL","target_currency":"ARS","pool_pair":"W-tCeBM_BRL/W-tCeBM_ARS","amount_out":"1000000000000000000000","max_amount_in":"1200000000000000000000","beneficiary_bank_id":"bank-galicia"}' | jq .
```
```json
{
  "error": "daily transfer limit exceeded",
  "error_code": "TRANSFER_LIMIT_EXCEEDED",
  "recommended_action": "retry after the daily window resets or request a higher limit from your central bank"
}
```

> **`BRIDGE_OUT_FAILED` is the one error you must not retry.** The Hub swap succeeded and
> delivery did not. Retrying double-spends. Reconcile the bridge-out position with the
> beneficiary Central Bank instead. That failure mode exists precisely *because* the
> beneficiary's CB, not yours, performs the bridge-out.

---

## What the mock cannot show you

- **State and asynchrony.** Prism answers from examples, so the swap never actually
  progresses through `BRIDGE_IN_PROGRESS → COMPLETED`; you get the documented terminal
  example immediately.
- **Quote expiry.** A mock quote never expires. A real one dies in 15 seconds.
- **Residue.** The residue arithmetic is real settlement behaviour, not a response field a
  mock can compute for you.
- **Sovereignty gates.** Prism does not check that you are the central bank of token A.

Those are exactly the assertions the conformance suite marks `live_only` and skips in mock
mode. See [tutorial 02](02-validate-implementation.md).

---

## Next steps

- [Tutorial 2 — validate an implementation](02-validate-implementation.md)
- [Tutorial 1 — Scenario A PvP settlement](01-pvp-settlement-mock.md), the other mechanism
- [Settlement flow walkthrough](../devnet-guide/flow-walkthrough.md)
- [The contract itself](../../contracts/amm/openapi_amm_v2.3.0.yaml) — 52 paths,
  59 operations, 12 of them public
