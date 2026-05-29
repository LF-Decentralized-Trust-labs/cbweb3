# Data Model: 010-cross-currency-swap-frontend

**Source**: `frontend/apps/bank/src/types/cross-currency-swap.types.ts`

---

## Entity Hierarchy

```
CROSS_CURRENCY_SWAP_ERROR (const object / discriminated union source)
  └── CrossCurrencySwapErrorCode (derived type)

CrossCurrencySwapStatus (union type — lifecycle states)

CrossCurrencyQuote (GET /amm/quote/cross-currency response)

CrossCurrencySwapRequest (POST /amm/swap/cross-currency request body)

CrossCurrencySwapResult (POST response + GET /:id polling response)

CrossCurrencySwapStoreState (Zustand store shape — runtime only, not persisted)
  ├── uses: PoolStatus (from amm-v2.types.ts — reused, not duplicated)
  ├── uses: CrossCurrencyQuote
  ├── uses: CrossCurrencySwapResult
  └── uses: CrossCurrencySwapErrorCode
```

---

## Types

### `CROSS_CURRENCY_SWAP_ERROR`

Const object used as both a runtime value map and the source for the `CrossCurrencySwapErrorCode` union.

| Key | Value |
|-----|-------|
| `POOL_NOT_ACTIVE` | `"POOL_NOT_ACTIVE"` |
| `CIRCUIT_BREAKER_HALTED` | `"CIRCUIT_BREAKER_HALTED"` |
| `SLIPPAGE_LIMIT_EXCEEDED` | `"SLIPPAGE_LIMIT_EXCEEDED"` |
| `INSUFFICIENT_POOL_LIQUIDITY` | `"INSUFFICIENT_POOL_LIQUIDITY"` |
| `QUOTE_EXPIRED` | `"QUOTE_EXPIRED"` |
| `BRIDGE_IN_FAILED` | `"BRIDGE_IN_FAILED"` |
| `SWAP_FAILED` | `"SWAP_FAILED"` |
| `BRIDGE_OUT_FAILED` | `"BRIDGE_OUT_FAILED"` |
| `SWAP_NOT_FOUND` | `"SWAP_NOT_FOUND"` |
| `INVALID_REQUEST` | `"INVALID_REQUEST"` |

### `CrossCurrencySwapErrorCode`

```typescript
type CrossCurrencySwapErrorCode = (typeof CROSS_CURRENCY_SWAP_ERROR)[keyof typeof CROSS_CURRENCY_SWAP_ERROR];
```

---

### `CrossCurrencySwapStatus`

Union type representing the full lifecycle of a swap operation. `COMPLETED` and `FAILED` are terminal states.

```typescript
type CrossCurrencySwapStatus =
  | "QUOTING"
  | "BRIDGE_IN_PROGRESS"
  | "SWAP_IN_PROGRESS"
  | "BRIDGE_OUT_PROGRESS"
  | "COMPLETED"
  | "FAILED";
```

| Value | Terminal | Stepper Mapping |
|-------|----------|-----------------|
| `QUOTING` | No | Pre-step (not shown in stepper) |
| `BRIDGE_IN_PROGRESS` | No | Step 1 active |
| `SWAP_IN_PROGRESS` | No | Step 2 active |
| `BRIDGE_OUT_PROGRESS` | No | Step 3 active |
| `COMPLETED` | **Yes** | All steps complete |
| `FAILED` | **Yes** | Current step error state |

---

### `CrossCurrencyQuote`

Response from `GET /api/v2/amm/quote/cross-currency`.

| Field | Type | Notes |
|-------|------|-------|
| `quote_id` | `string` | Opaque ID, passed as `quote_id` in swap request |
| `amount_in` | `string` | Wei string — source tokens required |
| `amount_out` | `string` | Wei string — target tokens to receive |
| `effective_rate` | `string` | Decimal string (e.g. `"0.510204"`) |
| `fee_bps` | `number` | Fee in basis points |
| `max_slippage_pct` | `number` | Backend-computed max slippage |
| `pool_pair` | `string` | e.g. `"W-BRL-ARS"` |
| `time_remaining_seconds` | `number` | TTL remaining at fetch time (≤ 15s) |
| `valid_until` | `number` | Unix timestamp (ms or s) — expiry |
| `created_at` | `number` | Unix timestamp of quote creation |

**Derived client-side state** (not in the API response):
- `quoteExpiresAt: number | null` — stored in Zustand as `valid_until * 1000` (ms) for `Date.now()` comparison

---

### `CrossCurrencySwapRequest`

Request body for `POST /api/v2/amm/swap/cross-currency`.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `source_currency` | `string` | Yes | e.g. `"BRL"` |
| `target_currency` | `string` | Yes | e.g. `"ARS"` |
| `pool_pair` | `string` | Yes | e.g. `"W-BRL-ARS"` |
| `amount_out` | `string` | Yes | Wei string |
| `max_amount_in` | `string` | Yes | Wei string — computed via `calcMaxAmountIn` |
| `payer_bank_id` | `string` | Yes | Operator-entered |
| `beneficiary_bank_id` | `string` | Yes | Operator-entered |
| `quote_id` | `string` | No | From `CrossCurrencyQuote.quote_id` |

---

### `CrossCurrencySwapResult`

Response from both `POST /api/v2/amm/swap/cross-currency` and `GET /api/v2/amm/swap/cross-currency/:id`.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `swap_id` | `string` | Yes | Stable ID for polling and reconciliation |
| `correlation_id` | `string` | No | Internal tracing ID |
| `status` | `CrossCurrencySwapStatus` | Yes | Current lifecycle state |
| `amount_in` | `string` | No | Wei string — populated on COMPLETED |
| `amount_out` | `string` | No | Wei string — populated on COMPLETED |
| `effective_rate` | `string` | No | Final executed rate |
| `bridge_in_position_id` | `string` | No | Position ID from step 1 |
| `swap_tx_hash` | `string` | No | On-chain hash from step 2 |
| `bridge_out_position_id` | `string` | No | Position ID from step 3 |
| `error_code` | `string` | No | Set when `status === "FAILED"` |
| `created_at` | `string` | No | ISO timestamp |

---

### `CrossCurrencySwapStoreState`

Zustand store shape (runtime only — no persistence). Defined in `cross-currency-swap.store.ts`.

| Field | Type | Initial |
|-------|------|---------|
| `poolStatus` | `PoolStatus \| null` | `null` |
| `quote` | `CrossCurrencyQuote \| null` | `null` |
| `quoteExpiresAt` | `number \| null` | `null` |
| `swapResult` | `CrossCurrencySwapResult \| null` | `null` |
| `step` | `"idle" \| "quoting" \| "submitting" \| "polling" \| "completed" \| "failed"` | `"idle"` |
| `error` | `string \| null` | `null` |
| `errorCode` | `CrossCurrencySwapErrorCode \| null` | `null` |

**Store actions**:

| Action | Signature | Side effects |
|--------|-----------|-------------|
| `fetchPoolStatus` | `(pair: string) => Promise<void>` | Sets `poolStatus` |
| `fetchQuote` | `(source: string, target: string, amountOut: string) => Promise<void>` | Sets `quote`, `quoteExpiresAt`, `step: idle` |
| `executeSwap` | `(req: CrossCurrencySwapRequest) => Promise<void>` | Drives full flow through terminal state |
| `pollSwapStatus` | `(swapId: string) => Promise<void>` | Loops until COMPLETED/FAILED or 90 attempts |
| `reset` | `() => void` | Clears quote, quoteExpiresAt, swapResult, step, error, errorCode — preserves poolStatus |

---

## Utility Functions

### `calcMaxAmountIn` (exported from `cross-currency-swap.api.ts`)

```
Input:  amountInWei: string  — quote's amount_in
        slippageBps: number  — default 1100 (= 10% buffer)
Output: string               — wei string with slippage added

Formula: floor(amountInWei × slippageBps / 1000)
Example: calcMaxAmountIn("1000000000000000000", 1100) → "1100000000000000000"
```

### `weiToDisplay` (exported from `cross-currency-swap.api.ts`)

```
Input:  wei: string      — 18-decimal wei string
        decimals: number — display decimal places, default 6
Output: string           — formatted display string or "—" on invalid input

Guard: returns "—" if input is empty, non-numeric, or does not match /^\d+$/
Example: weiToDisplay("1000000000000000000", 6) → "1.000000"
Example: weiToDisplay("", 6) → "—"
```

---

## Type reuse map

| Type | Source file | Used in |
|------|-------------|---------|
| `PoolStatus` | `amm-v2.types.ts` | `crossCurrencySwapApi.getPoolStatus`, store state |
| `CrossCurrencyQuote` | `cross-currency-swap.types.ts` | store state, UI display |
| `CrossCurrencySwapRequest` | `cross-currency-swap.types.ts` | `executeSwap` API call, store action param |
| `CrossCurrencySwapResult` | `cross-currency-swap.types.ts` | store state, polling |
| `CrossCurrencySwapStatus` | `cross-currency-swap.types.ts` | result field, stepper mapping |
| `CrossCurrencySwapErrorCode` | `cross-currency-swap.types.ts` | store state, error message map |
