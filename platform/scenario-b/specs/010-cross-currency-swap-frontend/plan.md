# Implementation Plan: Cross-Currency Swap Frontend

**Branch**: `010-cross-currency-swap-frontend` | **Date**: 2026-05-27 | **Spec**: [spec.md](spec.md)  
**Input**: Feature spec from `specs/010-cross-currency-swap-frontend/spec.md`

## Summary

Add a "Cross-Currency" tab to the existing `AMMTradingV2` component in `src/pages/AMMTradingPage.tsx`. The tab drives a quote-and-execute flow backed by the `009-commercial-cross-currency-swap` orchestrator endpoints. New files: one types file, one API client with two utility functions, one Zustand store, one UI component (inline tab), plus unit tests. No existing files other than `AMMTradingPage.tsx` are modified.

## Technical Context

**Language/Version**: TypeScript 5.4, React 19, Node 20  
**Primary Dependencies**: Zustand 5, Axios, React Router v6, `@cbweb3/ui` (Tabs, Button, Badge, Card, Input, Label already imported), Tailwind CSS  
**Storage**: Browser session only — Zustand in-memory, no persistence  
**Testing**: Vitest (existing test runner); unit tests for utilities and store actions  
**Target Platform**: Browser (bank frontend app — Scenario B mode)  
**Project Type**: React SPA — new tab feature inside existing page  
**Performance Goals**: Quote auto-refresh every 10s; polling loop at 2s intervals, max 90 iterations  
**Constraints**: 60s timeout isolated to execute axios instance only; no modification to `httpClientV2`; no `any` types; BigInt-only arithmetic for wei values  
**Scale/Scope**: 4 new source files + 1 modified page + 1 test file

## Constitution Check

*Constitution is project-template placeholder — no project-specific gates defined. Applying general engineering gates:*

| Gate | Status | Notes |
|------|--------|-------|
| Existing functionality preserved | PASS | `amm-v2.store.ts`, `amm-v2.api.ts`, `amm-v2.types.ts`, Exact Output tab — all untouched |
| No `any` types in new files | REQUIRED | Enforced by NFR-005 |
| `httpClientV2` timeout unchanged | REQUIRED | Dedicated axios instance per FR-003 / NFR-003 |
| BigInt arithmetic for wei | REQUIRED | `calcMaxAmountIn` and `weiToDisplay` — no `parseFloat` / `Number()` on wei |
| `BRIDGE_OUT_FAILED` non-dismissable | REQUIRED | Critical persistent alert with explicit acknowledgement gate |
| `tsc --noEmit` passes | REQUIRED | SC-006 |

## Project Structure

### Documentation (this feature)

```text
specs/010-cross-currency-swap-frontend/
├── plan.md              ← this file
├── research.md          ← Phase 0 (complete)
├── data-model.md        ← Phase 1 (complete)
└── contracts/
    └── api-contracts.md ← Phase 1 (complete)
```

### Source Code

```text
frontend/apps/bank/src/
├── types/
│   └── cross-currency-swap.types.ts          ← NEW (M1)
├── services/api/
│   ├── cross-currency-swap.api.ts             ← NEW (M2)
│   └── __tests__/
│       └── cross-currency-swap.api.test.ts    ← NEW (M5, partial)
├── features/amm/
│   ├── cross-currency-swap.store.ts           ← NEW (M3)
│   └── __tests__/
│       └── cross-currency-swap.store.test.ts  ← NEW (M5)
└── pages/
    └── AMMTradingPage.tsx                     ← MODIFIED (M4) — add Cross-Currency tab to AMMTradingV2
```

---

## Milestones

- [ ] **M1** — Types: `cross-currency-swap.types.ts`
- [ ] **M2** — API client + utilities: `cross-currency-swap.api.ts`
- [ ] **M3** — Store: `cross-currency-swap.store.ts`
- [ ] **M4** — UI: Cross-Currency tab added to `AMMTradingV2` in `AMMTradingPage.tsx`
- [ ] **M5** — Tests: unit tests for `calcMaxAmountIn`, `weiToDisplay`, store actions

---

## M1: Types — `src/types/cross-currency-swap.types.ts`

**File**: `frontend/apps/bank/src/types/cross-currency-swap.types.ts`  
**Exports**: 6 items — `CROSS_CURRENCY_SWAP_ERROR`, `CrossCurrencySwapErrorCode`, `CrossCurrencySwapStatus`, `CrossCurrencyQuote`, `CrossCurrencySwapRequest`, `CrossCurrencySwapResult`  
**Dependencies**: none (pure types)

### Content

```typescript
export const CROSS_CURRENCY_SWAP_ERROR = {
  POOL_NOT_ACTIVE: "POOL_NOT_ACTIVE",
  CIRCUIT_BREAKER_HALTED: "CIRCUIT_BREAKER_HALTED",
  SLIPPAGE_LIMIT_EXCEEDED: "SLIPPAGE_LIMIT_EXCEEDED",
  INSUFFICIENT_POOL_LIQUIDITY: "INSUFFICIENT_POOL_LIQUIDITY",
  QUOTE_EXPIRED: "QUOTE_EXPIRED",
  BRIDGE_IN_FAILED: "BRIDGE_IN_FAILED",
  SWAP_FAILED: "SWAP_FAILED",
  BRIDGE_OUT_FAILED: "BRIDGE_OUT_FAILED",
  SWAP_NOT_FOUND: "SWAP_NOT_FOUND",
  INVALID_REQUEST: "INVALID_REQUEST",
} as const;

export type CrossCurrencySwapErrorCode =
  (typeof CROSS_CURRENCY_SWAP_ERROR)[keyof typeof CROSS_CURRENCY_SWAP_ERROR];

export type CrossCurrencySwapStatus =
  | "QUOTING"
  | "BRIDGE_IN_PROGRESS"
  | "SWAP_IN_PROGRESS"
  | "BRIDGE_OUT_PROGRESS"
  | "COMPLETED"
  | "FAILED";

export interface CrossCurrencyQuote {
  quote_id: string;
  amount_in: string;
  amount_out: string;
  effective_rate: string;
  fee_bps: number;
  max_slippage_pct: number;
  pool_pair: string;
  time_remaining_seconds: number;
  valid_until: number;
  created_at: number;
}

export interface CrossCurrencySwapRequest {
  source_currency: string;
  target_currency: string;
  pool_pair: string;
  amount_out: string;
  max_amount_in: string;
  payer_bank_id: string;
  beneficiary_bank_id: string;
  quote_id?: string;
}

export interface CrossCurrencySwapResult {
  swap_id: string;
  correlation_id?: string;
  status: CrossCurrencySwapStatus;
  amount_in?: string;
  amount_out?: string;
  effective_rate?: string;
  bridge_in_position_id?: string;
  swap_tx_hash?: string;
  bridge_out_position_id?: string;
  error_code?: string;
  created_at?: string;
}
```

**Acceptance**: `tsc --noEmit` passes; all types importable by M2/M3/M4.

---

## M2: API Client + Utilities — `src/services/api/cross-currency-swap.api.ts`

**File**: `frontend/apps/bank/src/services/api/cross-currency-swap.api.ts`  
**Exports**: `crossCurrencySwapApi` object, `calcMaxAmountIn` function, `weiToDisplay` function  
**Dependencies**: `axios`, `http-client.ts` (for `httpClientV2`; re-derive `apiBaseV2` from same `VITE_API_URL`), `auth.interceptor.ts`, `amm-v2.types.ts` (`PoolStatus`), `cross-currency-swap.types.ts`

### Key constraints

1. `httpClientV2` in `http-client.ts` MUST NOT be touched.
2. A dedicated `crossCurrencyExecuteClient` is created:
   - `baseURL`: `apiBaseV2` (re-derived from `VITE_API_URL` using same logic as `http-client.ts`)
   - `withCredentials: true`
   - `timeout: 60_000`
3. `attachAuthInterceptor(crossCurrencyExecuteClient)` MUST be called.
4. `getPoolStatus`, `getQuote`, `getSwapStatus` use `httpClientV2` (no timeout).

### `calcMaxAmountIn`

```
Input:  amountInWei: string, slippageBps: number = 1100
Output: string
Formula: (BigInt(amountInWei) * BigInt(slippageBps) / 1000n).toString()
SC-004:  calcMaxAmountIn("1000000000000000000", 1100) → "1100000000000000000"
```

### `weiToDisplay`

```
Input:  wei: string, decimals: number = 6
Output: string | "—"
Guard:  return "—" if !wei || !/^\d+$/.test(wei)
Formula: whole = BigInt(wei) / 10n**18n
         frac  = (BigInt(wei) % 10n**18n).toString().padStart(18, "0").slice(0, decimals)
         return `${whole}.${frac}`
SC-004:  weiToDisplay("1000000000000000000", 6) → "1.000000"
```

### API methods

| Method | HTTP | Path | Client | Notes |
|--------|------|------|--------|-------|
| `getPoolStatus(pair)` | GET | `/amm/pool/{pair}/status` | `httpClientV2` | Returns `PoolStatus` |
| `getQuote(src, tgt, amtOut)` | GET | `/amm/quote/cross-currency` | `httpClientV2` | Params: `source_currency`, `target_currency`, `amount_out` |
| `executeSwap(payload)` | POST | `/amm/swap/cross-currency` | `crossCurrencyExecuteClient` | Body: `CrossCurrencySwapRequest`; 60s timeout |
| `getSwapStatus(swapId)` | GET | `/amm/swap/cross-currency/${swapId}` | `httpClientV2` | Returns `CrossCurrencySwapResult` |

**Acceptance (NFR-003)**: `httpClientV2.defaults.timeout === undefined`; `crossCurrencyExecuteClient.defaults.timeout === 60000`.

---

## M3: Store — `src/features/amm/cross-currency-swap.store.ts`

**File**: `frontend/apps/bank/src/features/amm/cross-currency-swap.store.ts`  
**Export**: `useCrossCurrencySwapStore`  
**Dependencies**: `zustand`, `axios`, `cross-currency-swap.api.ts`, `cross-currency-swap.types.ts`, `amm-v2.types.ts`

### State shape

```typescript
type CrossCurrencySwapStore = {
  poolStatus: PoolStatus | null;
  quote: CrossCurrencyQuote | null;
  quoteExpiresAt: number | null;
  swapResult: CrossCurrencySwapResult | null;
  step: "idle" | "quoting" | "submitting" | "polling" | "completed" | "failed";
  error: string | null;
  errorCode: CrossCurrencySwapErrorCode | null;
  fetchPoolStatus: (pair: string) => Promise<void>;
  fetchQuote: (sourceCurrency: string, targetCurrency: string, amountOut: string) => Promise<void>;
  executeSwap: (req: CrossCurrencySwapRequest) => Promise<void>;
  pollSwapStatus: (swapId: string) => Promise<void>;
  reset: () => void;
};
```

### `fetchPoolStatus`

```
1. Call crossCurrencySwapApi.getPoolStatus(pair)
2. set({ poolStatus: result })
3. On error: swallow (pool status is advisory — no step change)
```

### `fetchQuote`

```
1. set({ step: "quoting", error: null, errorCode: null })
2. Call crossCurrencySwapApi.getQuote(source, target, amountOut)
3. set({ quote: result, quoteExpiresAt: result.valid_until * 1000, step: "idle" })
4. On error: set({ step: "idle", error: extractApiError(err, "Failed to fetch quote") })
```

### `executeSwap` (critical path)

```
1.  set({ step: "submitting", error: null, errorCode: null })
2.  Expiry check: if (Date.now() > (get().quoteExpiresAt ?? 0)):
    a. await get().fetchQuote(src, tgt, amtOut)
    b. If step still not "idle": set failed QUOTE_EXPIRED; return
    c. Attach new quote_id to req
3.  Call crossCurrencySwapApi.executeSwap(req)
4.  On SUCCESS:
    a. status === "COMPLETED" → set({ step: "completed", swapResult })
    b. non-terminal → set({ swapResult }); await get().pollSwapStatus(result.swap_id)
5.  On ECONNABORTED (60s timeout):
    → set({ step: "failed", error: "Request timed out. The swap may still be in progress — check with your Central Bank." })
    → No pollSwapStatus (no swap_id)
6.  On API error (error.response.data.error):
    QUOTE_EXPIRED     → fetchQuote once, re-submit with new quote_id; if fails: set failed
    BRIDGE_OUT_FAILED → set({ step: "failed", errorCode: "BRIDGE_OUT_FAILED", swapResult: from error body if available })
    known errorCode   → set({ step: "failed", errorCode })
    unknown           → set({ step: "failed", error: extractApiError(err, "Swap failed") })
```

### `pollSwapStatus`

```
MAX_POLLS = 90, POLL_INTERVAL_MS = 2000
1. set({ step: "polling" })
2. for i in 0..MAX_POLLS:
   a. await sleep(POLL_INTERVAL_MS)
   b. result = await crossCurrencySwapApi.getSwapStatus(swapId)
   c. set({ swapResult: result })
   d. COMPLETED → set({ step: "completed" }); return
   e. FAILED    → set({ step: "failed", errorCode: result.error_code }); return
3. Exhausted → set({ step: "failed", error: "Swap status unknown after 3 minutes. Please contact your Central Bank with correlation_id." })
```

### `reset`

```
set({ quote: null, quoteExpiresAt: null, swapResult: null, step: "idle", error: null, errorCode: null })
// poolStatus intentionally preserved (FR-012)
```

**Acceptance**: Unit tests cover COMPLETED sync path, timeout path, BRIDGE_OUT_FAILED, poll exhaustion, reset preserves poolStatus.

---

## M4: UI — Cross-Currency Tab in `AMMTradingPage.tsx`

**File**: `frontend/apps/bank/src/pages/AMMTradingPage.tsx`  
**Change type**: Additive — wrap existing `AMMTradingV2` body in `<Tabs>`, add `CrossCurrencyTab` component.

### Structural change to `AMMTradingV2`

Wrap the existing flat-grid content as `TabsContent value="exact-output"` and add:

```tsx
<Tabs defaultValue="exact-output">
  <TabsList>
    <TabsTrigger value="exact-output">Exact Output</TabsTrigger>
    <TabsTrigger value="cross-currency">Cross-Currency</TabsTrigger>
  </TabsList>
  <TabsContent value="exact-output">
    {/* existing grid — zero internal changes */}
  </TabsContent>
  <TabsContent value="cross-currency">
    <CrossCurrencyTab />
  </TabsContent>
</Tabs>
```

`defaultValue="exact-output"` preserves current default behavior (SC-005).

### `CrossCurrencyTab` — local state

| State | Type | Default |
|-------|------|---------|
| `sourceCurrency` | `string` | `"BRL"` |
| `targetCurrency` | `string` | `"ARS"` |
| `amountOut` | `string` | `""` |
| `payerBankId` | `string` | `""` |
| `beneficiaryBankId` | `string` | `""` |
| `bridgeOutAcknowledged` | `boolean` | `false` |

### `CrossCurrencyTab` — effects and polling

```typescript
// Mount: fetch pool status
useEffect(() => { void fetchPoolStatus("W-BRL-ARS"); }, [fetchPoolStatus]);

// Pool status auto-refresh (30s)
usePolling(() => { void fetchPoolStatus("W-BRL-ARS"); }, 30_000, true);

// Quote auto-refresh (10s, only when idle with a live quote)
usePolling(
  () => { void fetchQuote(sourceCurrency, targetCurrency, amountOut); },
  10_000,
  step === "idle" && Boolean(quote && amountOut),
);
```

### Execute Swap disabled condition (FR-009, SC-003)

```typescript
const isSwapDisabled =
  poolStatus?.pool_status !== "ACTIVE" ||
  step !== "idle" ||
  quote === null ||
  (quoteExpiresAt !== null && quoteExpiresAt < Date.now());
```

### UI sections (FR-007)

1. **Pool status banner** — variant based on `poolStatus.pool_status`:
   - `ACTIVE`: green success badge + pool pair + current ratio
   - `PENDING_COUNTERPART` / `EMPTY`: amber warning message
   - `HALTED` / missing: red destructive alert; buttons disabled
2. **Quote form** — inputs for `source_currency`, `target_currency`, `amount_out`; [Get Quote] button (disabled when pool inactive)
3. **Quote display** — `effective_rate`, `weiToDisplay(quote.amount_in)` BRL, `weiToDisplay(quote.amount_out)` ARS, countdown timer (`time_remaining_seconds` decremented live via `setInterval`)
4. **Swap confirmation** — `payer_bank_id`, `beneficiary_bank_id`; [Execute Swap] button guarded by `isSwapDisabled`
5. **3-step stepper** — horizontal or vertical, steps: "Bridge In" / "Swap Hub" / "Bridge Out"; active step from `swapResult.status` per FR-010 mapping
6. **Result panel** (`step === "completed"`) — `swap_tx_hash`, `bridge_in_position_id`, `bridge_out_position_id`, `weiToDisplay(amount_in)`, `weiToDisplay(amount_out)`
7. **Error panel** (`step === "failed" && errorCode !== "BRIDGE_OUT_FAILED"`) — message from `ERROR_MESSAGES[errorCode]`; [Try Again] → `reset()`
8. **Critical alert** (`errorCode === "BRIDGE_OUT_FAILED"`) — red border, `swap_id` displayed, reconciliation text; checkbox or explicit [I Understand] button sets `bridgeOutAcknowledged = true`; [Reset Form] only enabled after acknowledgement

### Error message map (FR-011)

```typescript
const ERROR_MESSAGES: Record<string, string> = {
  POOL_NOT_ACTIVE: "Pool is not active. Contact your Central Bank.",
  CIRCUIT_BREAKER_HALTED: "Pool is temporarily paused by the Central Bank. Please try again later or contact your Central Bank.",
  SLIPPAGE_LIMIT_EXCEEDED: "Price moved beyond your slippage tolerance. Please refresh the quote and try again.",
  INSUFFICIENT_POOL_LIQUIDITY: "Not enough liquidity in the pool for this amount. Try a smaller amount or wait for the Central Bank to add liquidity.",
  QUOTE_EXPIRED: "Quote expired — a new quote has been fetched. Review and confirm again.",
  BRIDGE_IN_FAILED: "Could not lock your funds on the source network. The operation has been rolled back. No funds were lost.",
  SWAP_FAILED: "The AMM swap step failed. The operation has been rolled back. No funds were lost.",
  BRIDGE_OUT_FAILED: "Source funds were already debited. Manual reconciliation is required — contact your Central Bank with Swap ID:",
  SWAP_NOT_FOUND: "Swap record not found. Contact your Central Bank with the operation reference.",
  INVALID_REQUEST: "Invalid request. Check all inputs and try again.",
};
// Unknown code → generic fallback + collapsed <details> with raw errorCode value
```

**Acceptance**: SC-002, SC-003, SC-005 all pass.

---

## M5: Tests

### `src/services/api/__tests__/cross-currency-swap.api.test.ts`

| Test | Expected |
|------|----------|
| `calcMaxAmountIn("1000000000000000000", 1100)` | `"1100000000000000000"` (SC-004) |
| `calcMaxAmountIn` with default slippage | `slippageBps = 1100` applied |
| `weiToDisplay("1000000000000000000", 6)` | `"1.000000"` (SC-004) |
| `weiToDisplay("500000000000000000", 6)` | `"0.500000"` |
| `weiToDisplay("", 6)` | `"—"` |
| `weiToDisplay("not-a-number", 6)` | `"—"` |
| `httpClientV2.defaults.timeout` | `undefined` (NFR-003) |
| `crossCurrencyExecuteClient.defaults.timeout` | `60000` (NFR-003) — export instance or constant for test access |

### `src/features/amm/__tests__/cross-currency-swap.store.test.ts`

| Test | Expected |
|------|----------|
| `fetchQuote` success | `quote` populated, `step: "idle"`, `quoteExpiresAt` set |
| `fetchQuote` API error | `step: "idle"`, `error` non-null, `quote` unchanged |
| `executeSwap` returns COMPLETED | `step: "completed"`, `swapResult.status === "COMPLETED"` |
| `executeSwap` returns non-terminal → pollSwapStatus called | `step` transitions to `"polling"` then `"completed"` |
| `executeSwap` ECONNABORTED | `step: "failed"`, `error` contains "timed out", `pollSwapStatus` not called |
| `executeSwap` BRIDGE_OUT_FAILED | `step: "failed"`, `errorCode: "BRIDGE_OUT_FAILED"` |
| `pollSwapStatus` reaches COMPLETED on poll N | `step: "completed"` |
| `pollSwapStatus` reaches FAILED on poll N | `step: "failed"`, `errorCode` from `result.error_code` |
| `pollSwapStatus` exhausts 90 attempts | `step: "failed"`, error contains "3 minutes" |
| `reset()` preserves `poolStatus` | `poolStatus` unchanged |
| `reset()` clears transient fields | `quote`, `quoteExpiresAt`, `swapResult`, `step`, `error`, `errorCode` all null/idle |

---

## Verification Plan

```bash
# TypeScript typecheck (SC-006)
cd frontend && npx tsc --noEmit

# Build
make frontend-scenario-b

# Unit tests
cd frontend/apps/bank && npx vitest run src/services/api/__tests__/cross-currency-swap.api.test.ts
cd frontend/apps/bank && npx vitest run src/features/amm/__tests__/cross-currency-swap.store.test.ts
```

Passing criteria: `tsc --noEmit` exits 0; build succeeds; all unit test cases pass.

---

## Rollback / Safety

- Only `AMMTradingPage.tsx` is modified (additive wrapper + new tab). Revert = remove `<Tabs>` wrapper and `CrossCurrencyTab` usage.
- 4 new source files — net-new, no existing consumers.
- `defaultValue="exact-output"` ensures Exact Output tab is the landing tab — zero regression risk for existing flows.

---

## Decision Log

- **2026-05-27**: Dedicated axios instance (`crossCurrencyExecuteClient`) with `timeout: 60_000` for `executeSwap` only — preserves `httpClientV2` as timeout-free.
- **2026-05-27**: `PoolStatus` reused from `amm-v2.types.ts` — same backend response shape, no duplicate type.
- **2026-05-27**: `AMMTradingV2` wrapped in `<Tabs defaultValue="exact-output">` — minimal structural change; existing content zero-edit inside `TabsContent`.
- **2026-05-27**: `BRIDGE_OUT_FAILED` requires `bridgeOutAcknowledged` boolean before `reset()` — prevents accidental re-use after partial fund debit.
- **2026-05-27**: `pollSwapStatus` is a plain async loop inside the store action (`await sleep(2000)`) — avoids component-level `usePolling` for polling phase; polling is entirely store-managed.

## Surprises & Discoveries

- **2026-05-27**: `AMMTradingV2` uses a flat grid (not tabs) — introducing `<Tabs>` is required. `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` already imported in the file.
- **2026-05-27**: `httpClientV2.defaults.timeout` is `undefined` (confirmed in `http-client.ts`). NFR-003 test assertion: `=== undefined`.
- **2026-05-27**: `__tests__/` directories in `features/amm/` and `services/api/` are empty — test files are fully net-new.
- **2026-05-27**: `usePolling` has optional `maxDurationMs` param — not needed for quote auto-refresh (manual stop via `enabled` flag suffices); not used for polling (store-managed loop).
