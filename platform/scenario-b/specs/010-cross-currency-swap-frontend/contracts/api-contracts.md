# API Contracts: 010-cross-currency-swap-frontend

**Base URL**: `{VITE_API_URL}/api/v2` (same root as `httpClientV2`)  
**Authentication**: Cookie-based (`withCredentials: true`) — transparent via existing auth interceptor  
**Backend**: Implemented in `009-commercial-cross-currency-swap` — these contracts are consumption-only (frontend read spec)

---

## Endpoint 1 — Get Cross-Currency Quote

```
GET /api/v2/amm/quote/cross-currency
```

### Query Parameters

| Parameter | Type | Required | Example |
|-----------|------|----------|---------|
| `source_currency` | string | Yes | `BRL` |
| `target_currency` | string | Yes | `ARS` |
| `amount_out` | string | Yes | `2000000000000000000000` (wei) |

### Success Response — 200 OK

```json
{
  "quote_id": "q-abc123",
  "amount_in": "1020000000000000000000",
  "amount_out": "2000000000000000000000",
  "effective_rate": "0.510204",
  "fee_bps": 30,
  "max_slippage_pct": 1,
  "pool_pair": "W-BRL-ARS",
  "time_remaining_seconds": 15,
  "valid_until": 1748386815,
  "created_at": 1748386800
}
```

### Error Responses

| HTTP | `error` field | Trigger |
|------|---------------|---------|
| 400 | `POOL_NOT_ACTIVE` | Pool is EMPTY, PENDING_COUNTERPART, or HALTED |
| 400 | `INVALID_REQUEST` | Missing or malformed parameters |
| 503 | `CIRCUIT_BREAKER_HALTED` | Circuit breaker active |

### Client usage

```typescript
crossCurrencySwapApi.getQuote(sourceCurrency, targetCurrency, amountOut)
// Uses: httpClientV2 (no custom timeout)
// Called by: fetchQuote store action, usePolling auto-refresh (10s interval)
```

---

## Endpoint 2 — Execute Cross-Currency Swap

```
POST /api/v2/amm/swap/cross-currency
```

**Important**: This is a synchronous orchestrator call that drives 3 sequential on-chain steps (bridge-in → swap Hub → bridge-out). Expected latency: 10–30 seconds. Client MUST use 60-second timeout.

### Request Body

```json
{
  "source_currency": "BRL",
  "target_currency": "ARS",
  "pool_pair": "W-BRL-ARS",
  "amount_out": "2000000000000000000000",
  "max_amount_in": "1122000000000000000000",
  "payer_bank_id": "bank-a",
  "beneficiary_bank_id": "bank-b",
  "quote_id": "q-abc123"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `source_currency` | string | Yes | ISO currency code |
| `target_currency` | string | Yes | ISO currency code |
| `pool_pair` | string | Yes | Hub pool pair name |
| `amount_out` | string | Yes | Wei string — exact output desired |
| `max_amount_in` | string | Yes | Wei string — computed via `calcMaxAmountIn(quote.amount_in, 1100)` |
| `payer_bank_id` | string | Yes | Bank identifier for source funds |
| `beneficiary_bank_id` | string | Yes | Bank identifier for destination |
| `quote_id` | string | No | Ties request to a pre-fetched quote |

### Success Response — 200 OK (synchronous COMPLETED)

```json
{
  "swap_id": "swap-xyz789",
  "correlation_id": "corr-001",
  "status": "COMPLETED",
  "amount_in": "1018000000000000000000",
  "amount_out": "2000000000000000000000",
  "effective_rate": "0.511248",
  "bridge_in_position_id": "pos-in-001",
  "swap_tx_hash": "0xabc...def",
  "bridge_out_position_id": "pos-out-002",
  "created_at": "2026-05-27T14:00:00Z"
}
```

### Intermediate Response — 200 OK (non-terminal, triggers client polling)

```json
{
  "swap_id": "swap-xyz789",
  "status": "BRIDGE_IN_PROGRESS",
  "created_at": "2026-05-27T14:00:00Z"
}
```

### Error Responses

| HTTP | `error` field | Behaviour |
|------|---------------|-----------|
| 408 / 504 | timeout | Store sets `step: failed`, error = timeout message, does NOT poll (no swap_id) |
| 400 | `QUOTE_EXPIRED` | Store auto-retries: calls `fetchQuote`, then re-submits with new `quote_id` |
| 400 | `SLIPPAGE_LIMIT_EXCEEDED` | Store sets `step: failed`, `errorCode: SLIPPAGE_LIMIT_EXCEEDED` |
| 400 | `INSUFFICIENT_POOL_LIQUIDITY` | Store sets `step: failed`, `errorCode: INSUFFICIENT_POOL_LIQUIDITY` |
| 503 | `CIRCUIT_BREAKER_HALTED` | Store sets `step: failed`, `errorCode: CIRCUIT_BREAKER_HALTED` |
| 500 | `BRIDGE_IN_FAILED` | Store sets `step: failed`, `errorCode: BRIDGE_IN_FAILED` |
| 500 | `BRIDGE_OUT_FAILED` | **Critical**: Store sets `step: failed`, `errorCode: BRIDGE_OUT_FAILED`, `swap_id` MUST be displayed |

### Client usage

```typescript
crossCurrencySwapApi.executeSwap(payload)
// Uses: dedicated axios instance with timeout: 60_000
// Called by: executeSwap store action only
```

---

## Endpoint 3 — Get Swap Status (Polling)

```
GET /api/v2/amm/swap/cross-currency/:id
```

### Path Parameter

| Parameter | Type | Example |
|-----------|------|---------|
| `id` | string | `swap-xyz789` |

### Success Response — 200 OK

Same shape as `CrossCurrencySwapResult` — returns current status at any lifecycle stage:

```json
{
  "swap_id": "swap-xyz789",
  "correlation_id": "corr-001",
  "status": "SWAP_IN_PROGRESS",
  "bridge_in_position_id": "pos-in-001",
  "created_at": "2026-05-27T14:00:00Z"
}
```

When terminal (`COMPLETED`):

```json
{
  "swap_id": "swap-xyz789",
  "status": "COMPLETED",
  "amount_in": "1018000000000000000000",
  "amount_out": "2000000000000000000000",
  "effective_rate": "0.511248",
  "bridge_in_position_id": "pos-in-001",
  "swap_tx_hash": "0xabc...def",
  "bridge_out_position_id": "pos-out-002"
}
```

When terminal (`FAILED`):

```json
{
  "swap_id": "swap-xyz789",
  "status": "FAILED",
  "error_code": "BRIDGE_OUT_FAILED",
  "bridge_in_position_id": "pos-in-001",
  "swap_tx_hash": "0xabc...def"
}
```

### Error Responses

| HTTP | Condition | Client behaviour |
|------|-----------|-----------------|
| 404 | `SWAP_NOT_FOUND` | Stop polling, set `step: failed`, `errorCode: SWAP_NOT_FOUND` |
| 500 | Server error | Retry on next interval; count toward 90-attempt limit |

### Polling contract

- **Interval**: 2 seconds between polls
- **Max attempts**: 90 (= 3 minutes total)
- **Stop conditions**: `status === "COMPLETED"` OR `status === "FAILED"` OR attempts exhausted
- **On exhaustion**: `step: failed`, `error: "Swap status unknown after 3 minutes. Please contact your Central Bank with correlation_id."`

### Client usage

```typescript
crossCurrencySwapApi.getSwapStatus(swapId)
// Uses: httpClientV2 (no custom timeout)
// Called by: pollSwapStatus store action (2s loop, max 90 iterations)
```

---

## Endpoint 4 — Get Pool Status (reused from amm-v2)

```
GET /api/v2/amm/pool/{pair}/status
```

This endpoint is already consumed by `ammV2Api.getPoolStatus` in `amm-v2.api.ts`. The cross-currency API client exposes a `getPoolStatus(pair)` method that calls the **same endpoint** via `httpClientV2`, returning the existing `PoolStatus` type from `amm-v2.types.ts`. No new type or endpoint contract — this section documents its usage in the cross-currency context.

**Default pair for Cross-Currency tab**: `"W-BRL-ARS"` (called on mount, refreshed every 30s)

**Success Response**: `PoolStatus` (see `amm-v2.types.ts`)

**Guard**: [Get Quote] and [Execute Swap] disabled when `poolStatus?.pool_status !== "ACTIVE"`.
