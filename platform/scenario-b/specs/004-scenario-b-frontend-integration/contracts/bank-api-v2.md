# Bank App — API v2 Contracts

**Feature**: `scenario-b-frontend-integration` | **App**: `frontend/apps/bank` | Date: 2026-05-07

All endpoints require `Authorization: Bearer <JWT>` (injected by `attachAuthInterceptor`).
All amount fields are integer strings. All requests/responses use `Content-Type: application/json`.

---

## Bridge API

### POST /api/v2/bridge/lock-mint

Initiate a Lock & Mint operation to move native CBDC onto the Hub as mirrored tokens.

**Request body** (`LockMintRequest`):
```json
{
  "owner_bank_id":  "bank-a",
  "spoke_network":  "spoke-a",
  "native_asset":   "BRL",
  "mirrored_asset": "mBRL",       // optional
  "amount":         "1000000"
}
```

**Response 201** (`BridgedAssetPosition`):
```json
{
  "position_id":     "pos-uuid-1",
  "owner_bank_id":   "bank-a",
  "spoke_network":   "spoke-a",
  "native_asset":    "BRL",
  "mirrored_asset":  "mBRL",
  "mirrored_amount": "1000000",
  "bridge_state":    "LOCKING",
  "relayer_retries": 0,
  "relayer_error_log": null,
  "created_at":      "2026-05-07T12:00:00Z",
  "updated_at":      "2026-05-07T12:00:00Z"
}
```

**Error responses**: `400` invalid input, `422` validation error

---

### POST /api/v2/bridge/burn-unlock

Initiate Burn & Unlock for an active bridge position.

**Request body** (`BurnUnlockRequest`):
```json
{
  "position_id": "pos-uuid-1"
}
```

**Response 200**:
```json
{
  "position_id":  "pos-uuid-1",
  "bridge_state": "BURNED"
}
```

**Error responses**: `404` position not found, `409` position not in `ACTIVE` state

---

### GET /api/v2/bridge/positions

List bridge positions. **No `state` filter in the Scenario B UI request** (filter is client-side only).

**Query params** (API supports but UI does not send): `state=LOCKING` (optional)

**Response 200**:
```json
[
  {
    "position_id":     "pos-uuid-1",
    "owner_bank_id":   "bank-a",
    "spoke_network":   "spoke-a",
    "native_asset":    "BRL",
    "mirrored_asset":  "mBRL",
    "mirrored_amount": "1000000",
    "bridge_state":    "ACTIVE",
    "relayer_retries": 0,
    "relayer_error_log": null,
    "created_at":      "2026-05-07T12:00:00Z",
    "updated_at":      "2026-05-07T12:01:00Z"
  }
]
```

---

## AMM v2 API (Bank)

### GET /api/v2/amm/quote/exact-output

Get a price quote for a given output amount.

**Query params**: `pair=BRL-USD&amount_out=500000`

**Response 200** (`AMMQuote`):
```json
{
  "pair":            "BRL-USD",
  "amount_out":      "500000",
  "required_input":  "550000",
  "price_impact":    0.0125,
  "quote_timestamp": "2026-05-07T12:00:00Z"
}
```

**Staleness**: UI treats quote as stale if `Date.now() - Date.parse(quote_timestamp) > 10_000 ms`.

---

### POST /api/v2/amm/swap/exact-output

Execute an AMM swap.

**Request body** (`SwapRequest`):
```json
{
  "pair":           "BRL-USD",
  "amount_out":     "500000",
  "max_amount_in":  "561250",
  "payer_id":       "bank-a",
  "beneficiary_id": "bank-b"
}
```

**Response 200** (`SwapOrder`):
```json
{
  "order_id":     "order-uuid-1",
  "tx_hash":      "0xabc123",
  "amount_in":    "548000",
  "amount_out":   "500000",
  "state":        "COMPLETED",
  "confirmed_at": "2026-05-07T12:00:01Z"
}
```

**Error responses** (HTTP 422 or 400 with error code in body):
```json
{ "error": "SLIPPAGE_LIMIT_EXCEEDED" }
{ "error": "INSUFFICIENT_POOL_LIQUIDITY" }
{ "error": "ZK_VALIDATION_FAILED" }
{ "error": "CIRCUIT_BREAKER_HALTED" }
```

UI maps error codes → user messages per data-model.md.

---

### GET /api/v2/amm/pool/:pair/status

Get real-time pool reserves and imbalance flag.

**Path param**: `pair=BRL-USD`

**Response 200** (`PoolStatus`):
```json
{
  "pool_pair":      "BRL-USD",
  "reserve_a":      "10000000",
  "reserve_b":      "9500000",
  "current_ratio":  "0.95",
  "imbalance_flag": false,
  "updated_at":     "2026-05-07T12:00:00Z"
}
```

---

### POST /api/v2/amm/token/approve-amm

Approve AMM contract to spend bank's Hub tokens.

**Request body** (`ApproveAmmRequest`):
```json
{
  "amount_a": "1000000",
  "amount_b": "1000000"
}
```

**Response 200**:
```json
{ "status": "ok" }
```

---

## Circuit Breaker Status (Bank — Read Only)

### GET /api/v2/governance/circuit-breaker/status

Read-only endpoint used by the bank app to display the halted banner on the AMM page.

**Query params**: `pair=BRL-USD`

**Response 200**:
```json
{
  "pair":              "BRL-USD",
  "state":             "LIVE",
  "pause_initiator":   null,
  "pause_reason":      null,
  "resume_request_id": null
}
```

When `state = "HALTED"`, the bank AMM page disables the Swap Submit button and shows: "Swaps temporarily suspended by Central Bank".
