# Tasks: 010-cross-currency-swap-frontend

**Feature**: Cross-Currency Swap Frontend Tab  
**Branch**: `010-cross-currency-swap-frontend`  
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)  
**Generated**: 2026-05-27

---

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Parallelizable — operates on a different file with no dependency on an in-progress task
- **[US1–US4]**: User story this task implements (from spec.md)
- No story label = infrastructure or cross-cutting task

---

## Phase 1: Setup

**Purpose**: Expose the `apiBaseV2` constant that the new API client requires without touching `httpClientV2`.

| # | Task | Status |
|---|------|--------|
| T005 | Export `apiBaseV2` from `http-client.ts` | ☑ |

### Tasks

- [X] T005 Export `apiBaseV2` from `frontend/apps/bank/src/services/api/http-client.ts`

**Checkpoint**: `apiBaseV2` is importable by other modules; `tsc --noEmit` passes; no circular imports.

---

## Phase 2: Foundational (Blocking Prerequisite)

**Purpose**: Define all TypeScript types used by the API client, store, and UI. Must be complete before any other milestone.

**⚠️ CRITICAL**: T002, T003, and T004 cannot start until T001 is done.

| # | Task | Status |
|---|------|--------|
| T001 | Define cross-currency swap types | ☑ |

### Tasks

- [X] T001 Create `frontend/apps/bank/src/types/cross-currency-swap.types.ts`

**Checkpoint**: `tsc --noEmit` passes; all 6 exports (`CROSS_CURRENCY_SWAP_ERROR`, `CrossCurrencySwapErrorCode`, `CrossCurrencySwapStatus`, `CrossCurrencyQuote`, `CrossCurrencySwapRequest`, `CrossCurrencySwapResult`) are importable.

---

## Phase 3: P1 User Stories — Quote & Execute (US1 + US2)

**Goal**: Implement the API client and Zustand store that drive the core quote-and-execute flow.

**Independent Test**: Vitest unit tests against `calcMaxAmountIn` and `weiToDisplay` pass (see T006). Store action tests cover `fetchQuote → executeSwap → pollSwapStatus` state transitions.

### US1 — Get Quote and Execute Cross-Currency Swap

**Why P1**: This is the entire reason for the feature; no value is delivered without it.

| # | Task | Status |
|---|------|--------|
| T002 | API client + utility functions | ☑ |
| T003 | Zustand store with state machine | ☑ |

### Tasks

- [X] T002 [US1] Create `frontend/apps/bank/src/services/api/cross-currency-swap.api.ts`
- [X] T003 [US1] Create `frontend/apps/bank/src/features/amm/cross-currency-swap.store.ts`

**Checkpoint**: TypeScript compiles; `crossCurrencySwapApi.getQuote`, `executeSwap`, `getSwapStatus`, `getPoolStatus` are callable; `httpClientV2.defaults.timeout` is NOT set after import; `useCrossCurrencySwapStore` state machine transitions are correct (idle → submitting → completed/failed).

---

## Phase 4: P1 + P2 User Stories — UI (US1, US2, US3, US4)

**Goal**: Render the Cross-Currency tab inside the existing `AMMTradingV2` component. This single task implements the visible surface for all four user stories.

**Independent Test**: Open `AMMTradingPage` — Exact Output tab renders unchanged. Cross-Currency tab shows pool status banner, quote form, swap form, stepper, result panel, and error panel. TypeScript compiles with `tsc --noEmit`.

### US1/US2/US3/US4 — Tab UI

| # | Task | Status |
|---|------|--------|
| T004 | Add Cross-Currency tab to `AMMTradingPage.tsx` | ☑ |

### Tasks

- [X] T004 [US1] Modify `frontend/apps/bank/src/pages/AMMTradingPage.tsx` to add Cross-Currency tab and `CrossCurrencySwapPanel` component

**Checkpoint**: Exact Output tab renders unchanged. Cross-Currency tab renders correctly for each state: pool status banner (ACTIVE = green, else = amber/red), quote form with TTL countdown, swap form with `max_amount_in`, stepper (BRIDGE_IN → SWAP_IN_PROGRESS → BRIDGE_OUT → COMPLETED), result panel (`swap_tx_hash`, position IDs, amounts), error panel with per-code messages, and critical `BRIDGE_OUT_FAILED` persistent alert with explicit acknowledge gate.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Goal**: Unit tests for the two pure utility functions exported from the API client.

| # | Task | Status |
|---|------|--------|
| T006 | Unit tests for `calcMaxAmountIn` and `weiToDisplay` | ☑ |

### Tasks

- [X] T006 [P] Create `frontend/apps/bank/src/services/api/__tests__/cross-currency-swap.utils.test.ts`

**Checkpoint**: All test cases pass when run with Vitest; TypeScript compiles.

---

## Task Reference

### T005 — Export `apiBaseV2` from `http-client.ts`

| Field | Value |
|-------|-------|
| **Milestone** | M4 (prerequisite) |
| **Target file** | `frontend/apps/bank/src/services/api/http-client.ts` |
| **Effort** | S |
| **Depends on** | — |
| **Blocked by** | — |

**What to do**: Add `export` to the existing `const apiBaseV2` declaration. All other exports must remain unchanged.

```typescript
// Change:
const apiBaseV2 = `${apiBaseRoot}/api/v2`;
// To:
export const apiBaseV2 = `${apiBaseRoot}/api/v2`;
```

**Acceptance**:
- `apiBaseV2` is importable in `cross-currency-swap.api.ts`
- `tsc --noEmit` passes
- No new imports or changes to `httpClient`, `httpClientV2`, or `attachAuthInterceptor` calls

---

### T001 — Define Cross-Currency Swap Types

| Field | Value |
|-------|-------|
| **Milestone** | M1 |
| **Target file** | `frontend/apps/bank/src/types/cross-currency-swap.types.ts` |
| **Effort** | S |
| **Depends on** | T005 |
| **Blocked by** | — |

**What to do**: Create the types file with exactly these 6 exports:

1. `CROSS_CURRENCY_SWAP_ERROR` — `as const` object with 10 error code keys:
   `POOL_NOT_ACTIVE`, `CIRCUIT_BREAKER_HALTED`, `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY`, `QUOTE_EXPIRED`, `BRIDGE_IN_FAILED`, `SWAP_FAILED`, `BRIDGE_OUT_FAILED`, `SWAP_NOT_FOUND`, `INVALID_REQUEST`

2. `CrossCurrencySwapErrorCode` — derived type: `(typeof CROSS_CURRENCY_SWAP_ERROR)[keyof typeof CROSS_CURRENCY_SWAP_ERROR]`

3. `CrossCurrencySwapStatus` — union type:
   `"QUOTING" | "BRIDGE_IN_PROGRESS" | "SWAP_IN_PROGRESS" | "BRIDGE_OUT_PROGRESS" | "COMPLETED" | "FAILED"`

4. `CrossCurrencyQuote` — interface with fields:
   `quote_id: string`, `amount_in: string`, `amount_out: string`, `effective_rate: string`, `fee_bps: number`, `max_slippage_pct: number`, `pool_pair: string`, `time_remaining_seconds: number`, `valid_until: number`, `created_at: number`

5. `CrossCurrencySwapRequest` — interface with fields:
   `source_currency: string`, `target_currency: string`, `pool_pair: string`, `amount_out: string`, `max_amount_in: string`, `payer_bank_id: string`, `beneficiary_bank_id: string`, `quote_id?: string`

6. `CrossCurrencySwapResult` — interface with fields:
   `swap_id: string`, `status: CrossCurrencySwapStatus`, `correlation_id?: string`, `amount_in?: string`, `amount_out?: string`, `effective_rate?: string`, `bridge_in_position_id?: string`, `swap_tx_hash?: string`, `bridge_out_position_id?: string`, `error_code?: string`, `created_at?: string`

**Acceptance**:
- `tsc --noEmit` passes
- All 6 symbols are named exports
- No `any` types

---

### T002 — API Client + Utility Functions

| Field | Value |
|-------|-------|
| **Milestone** | M2 |
| **Target file** | `frontend/apps/bank/src/services/api/cross-currency-swap.api.ts` |
| **Effort** | M |
| **Depends on** | T001, T005 |
| **Blocked by** | T001, T005 |

**What to do**:

1. Import `apiBaseV2` from `./http-client` and `httpClientV2` from `./http-client`
2. Import `attachAuthInterceptor` from `./interceptors/auth.interceptor`
3. Import `CrossCurrencyQuote`, `CrossCurrencySwapRequest`, `CrossCurrencySwapResult` from `../../types/cross-currency-swap.types`
4. Import `PoolStatus` from `../../features/amm/amm-v2.types` (reuse existing type — do not duplicate)

5. **Create `crossCurrencyExecuteClient`** — dedicated axios instance:
   - `baseURL: apiBaseV2`
   - `withCredentials: true`
   - `timeout: 60_000`
   - Call `attachAuthInterceptor(crossCurrencyExecuteClient)` immediately after

6. **Export `crossCurrencySwapApi`** object with 4 methods:
   - `getQuote(sourceCurrency: string, targetCurrency: string, amountOut: string): Promise<CrossCurrencyQuote>`
     → `GET /amm/quote/cross-currency` via `httpClientV2` with params `{ source_currency, target_currency, amount_out }`
   - `executeSwap(payload: CrossCurrencySwapRequest): Promise<CrossCurrencySwapResult>`
     → `POST /amm/swap/cross-currency` via `crossCurrencyExecuteClient` with JSON body
   - `getSwapStatus(swapId: string): Promise<CrossCurrencySwapResult>`
     → `GET /amm/swap/cross-currency/${swapId}` via `httpClientV2`
   - `getPoolStatus(pair: string): Promise<PoolStatus>`
     → `GET /amm/pool/${pair}/status` via `httpClientV2`

7. **Export `calcMaxAmountIn`**:
   ```typescript
   export function calcMaxAmountIn(amountInWei: string, slippageBps = 1100): string {
     return (BigInt(amountInWei) * BigInt(slippageBps) / 1000n).toString();
   }
   ```

8. **Export `weiToDisplay`**:
   ```typescript
   export function weiToDisplay(wei: string, decimals = 6): string {
     if (!wei || !/^\d+$/.test(wei)) return "—";
     const whole = BigInt(wei) / 10n ** 18n;
     const frac = (BigInt(wei) % 10n ** 18n).toString().padStart(18, "0").slice(0, decimals);
     return `${whole}.${frac}`;
   }
   ```

9. **Export constants**:
   ```typescript
   export const CROSS_CURRENCY_QUOTE_TTL_MS = 15_000;
   export const CROSS_CURRENCY_QUOTE_REFRESH_MS = 10_000;
   ```

**Acceptance**:
- `tsc --noEmit` passes; no `any` types
- `httpClientV2.defaults.timeout` is `undefined` after importing this module
- `crossCurrencyExecuteClient.defaults.timeout === 60000`
- `calcMaxAmountIn("1000000000000000000", 1100)` → `"1100000000000000000"`
- `weiToDisplay("1000000000000000000", 6)` → `"1.000000"`
- `weiToDisplay("")` → `"—"`

---

### T003 — Zustand Store with State Machine

| Field | Value |
|-------|-------|
| **Milestone** | M3 |
| **Target file** | `frontend/apps/bank/src/features/amm/cross-currency-swap.store.ts` |
| **Effort** | M |
| **Depends on** | T001, T002 |
| **Blocked by** | T001, T002 |

**What to do**: Create a Zustand store `useCrossCurrencySwapStore` with the following state type and actions. Do NOT import from or modify `amm-v2.store.ts`.

**State type `CrossCurrencySwapStore`**:

```typescript
interface CrossCurrencySwapState {
  poolStatus: PoolStatus | null;
  quote: CrossCurrencyQuote | null;
  quoteExpiresAt: number | null;           // Date.now() + CROSS_CURRENCY_QUOTE_TTL_MS
  swapResult: CrossCurrencySwapResult | null;
  step: "idle" | "submitting" | "polling" | "completed" | "failed";
  error: string | null;
  errorCode: CrossCurrencySwapErrorCode | null;
  bridgeOutAcknowledged: boolean;
}
```

**Actions**:

1. **`fetchPoolStatus(pair: string)`** — calls `crossCurrencySwapApi.getPoolStatus(pair)`, stores result in `poolStatus`. On error, sets `error`.

2. **`fetchQuote(sourceCurrency: string, targetCurrency: string, amountOut: string)`** — calls `crossCurrencySwapApi.getQuote(...)`, sets `quote`, sets `quoteExpiresAt = Date.now() + CROSS_CURRENCY_QUOTE_TTL_MS`. On error, sets `error` and clears `quote`.

3. **`executeSwap(req: CrossCurrencySwapRequest)`** — orchestration:
   1. Set `step = "submitting"`
   2. If `quoteExpiresAt !== null && Date.now() > quoteExpiresAt`: call `fetchQuote` once; if it fails, set `step = "failed"`, `errorCode = QUOTE_EXPIRED`, return
   3. Call `crossCurrencySwapApi.executeSwap(req)`
   4. If response `status === "COMPLETED"`: set `swapResult`, `step = "completed"`, return
   5. If response has `swap_id`: set `swapResult` with partial data, call `pollSwapStatus(result.swap_id)`
   6. On HTTP error with `error_code === "QUOTE_EXPIRED"`: call `fetchQuote` once, then re-submit (retry only once); if retry fails set `step = "failed"`, `errorCode = QUOTE_EXPIRED`
   7. On HTTP error with `error_code === "BRIDGE_OUT_FAILED"`: set `step = "failed"`, `errorCode = BRIDGE_OUT_FAILED`, `bridgeOutAcknowledged = false`, preserve `swapResult` with `swap_id`
   8. On other HTTP error: set `step = "failed"`, `errorCode` from response `error_code`, `error` from message

4. **`pollSwapStatus(swapId: string)`** — polling loop:
   - Max 90 iterations × 2000ms delay (3 minutes total)
   - Set `step = "polling"` on each iteration
   - Update `swapResult` with each response
   - On `status === "COMPLETED"`: set `step = "completed"`, stop
   - On `status === "FAILED"`: set `step = "failed"`, `errorCode = result.error_code ?? null`, stop
   - On 404 (`SWAP_NOT_FOUND`): set `step = "failed"`, `errorCode = SWAP_NOT_FOUND`, stop
   - On exhaustion: set `step = "failed"`, `error = "Swap status unknown after 3 minutes."`, stop

5. **`acknowledgeSwapError()`** — sets `bridgeOutAcknowledged = true`

6. **`reset()`** — guard: only allowed if `errorCode !== BRIDGE_OUT_FAILED || bridgeOutAcknowledged === true`; if guard fails, do nothing (do not throw). If allowed, reset all state to initial values (`step = "idle"`, clear `quote`, `swapResult`, `error`, `errorCode`, `quoteExpiresAt`, `bridgeOutAcknowledged = false`). `poolStatus` is preserved.

**Acceptance**:
- `tsc --noEmit` passes; no `any` types
- `amm-v2.store.ts` is not imported or modified
- State transitions: idle → submitting → completed/failed; submitting → polling → completed/failed
- `reset()` is a no-op when `errorCode === BRIDGE_OUT_FAILED && bridgeOutAcknowledged === false`

---

### T004 — Cross-Currency Tab UI in `AMMTradingPage.tsx`

| Field | Value |
|-------|-------|
| **Milestone** | M4 |
| **Target file** | `frontend/apps/bank/src/pages/AMMTradingPage.tsx` |
| **Effort** | L |
| **Depends on** | T001, T002, T003 |
| **Blocked by** | T003 |

**What to do**: Modify `AMMTradingPage.tsx` to add a "cross-currency" tab alongside the existing "exact-output" tab. All changes are additive — the existing layout must not change.

**Structural changes to `AMMTradingV2` (or the component that owns the current content)**:
1. Wrap the existing content in `<Tabs defaultValue="exact-output">` with two tab triggers: "Exact Output" and "Cross-Currency"
2. Move the existing grid (halted banner, imbalance banner, Quote/Swap/Pool/Approve cards) into `<TabsContent value="exact-output">`
3. Add `<TabsContent value="cross-currency"><CrossCurrencySwapPanel /></TabsContent>`

**`CrossCurrencySwapPanel` component** (new function component in the same file or a co-located file):

**On mount**: calls `fetchPoolStatus("W-BRL-ARS")`

**Pool status banner** (always visible at top of tab):
- `poolStatus.status === "ACTIVE"` → green `Badge`/banner: "Pool W-BRL-ARS — ACTIVE"
- otherwise → amber/red banner: "Pool W-BRL-ARS is currently {status}. Contact your Central Bank for status."
- Pool status auto-refreshes every 30s via `usePolling` or `setInterval` (implement in whatever pattern is consistent with the codebase)

**Quote form** (visible when `step === "idle"`):
- `source_currency` text input, default `"BRL"`
- `target_currency` text input, default `"ARS"`
- `amount_out` text input (wei string); on change: debounce 600ms, then trigger `fetchQuote`
- [Get Quote] `Button` — calls `fetchQuote(sourceCurrency, targetCurrency, amountOut)`; disabled when `poolStatus?.status !== "ACTIVE"`

**Quote display** (visible when `quote !== null`):
- `effective_rate` label
- `amount_in` → `weiToDisplay(quote.amount_in)` + " BRL" (or source currency)
- `amount_out` → `weiToDisplay(quote.amount_out)` + " ARS" (or target currency)
- TTL countdown: computed from `quoteExpiresAt - Date.now()` in seconds, updated every second via `useInterval` or similar
- Auto-refresh: `usePolling` or `setInterval` at `CROSS_CURRENCY_QUOTE_REFRESH_MS` (10s) while `step === "idle"` and `quote !== null`

**Swap form** (visible when `quote !== null && step === "idle"`):
- `payer_bank_id` text input
- `beneficiary_bank_id` text input
- `max_amount_in` read-only display: `calcMaxAmountIn(quote.amount_in)` in human-readable form
- [Execute Swap] `Button`:
  - Disabled when: `step !== "idle"` OR `poolStatus?.status !== "ACTIVE"` OR `quoteExpiresAt !== null && Date.now() > quoteExpiresAt`
  - Tooltip on disabled-due-to-expired: "Quote expired — click Get Quote to refresh"
  - On click: calls `executeSwap({ source_currency, target_currency, pool_pair: "W-BRL-ARS", amount_out, max_amount_in: calcMaxAmountIn(quote.amount_in), payer_bank_id, beneficiary_bank_id, quote_id: quote.quote_id })`

**Progress stepper** (visible when `step === "submitting"` or `step === "polling"`):
4 steps in order: BRIDGE_IN → SWAP_IN_PROGRESS → BRIDGE_OUT → COMPLETED
- Highlight the step matching `swapResult?.status`
- Use `Badge` or a simple ordered list with Tailwind to indicate active/done/pending state

**Result panel** (visible when `step === "completed"`):
- `swap_tx_hash`
- `amount_in` → `weiToDisplay(swapResult.amount_in)`
- `amount_out` → `weiToDisplay(swapResult.amount_out)`
- `bridge_in_position_id`
- `bridge_out_position_id`
- `correlation_id`
- [Reset] `Button` → calls `reset()`

**Error panel** (visible when `step === "failed"` and `errorCode !== "BRIDGE_OUT_FAILED"`):
Per-code user-friendly messages:
| `errorCode` | Message |
|-------------|---------|
| `CIRCUIT_BREAKER_HALTED` | "Pool is temporarily paused by the Central Bank. Please try again later or contact your Central Bank." |
| `SLIPPAGE_LIMIT_EXCEEDED` | "Price moved beyond your slippage tolerance. Please refresh the quote and try again." |
| `BRIDGE_IN_FAILED` | "Could not lock your funds on the source network. The operation has been rolled back. No funds were lost." |
| `INSUFFICIENT_POOL_LIQUIDITY` | "Not enough liquidity in the pool for this amount. Try a smaller amount or wait for the Central Bank to add liquidity." |
| `QUOTE_EXPIRED` | "The quote expired during submission. A new quote has been fetched — please review and confirm again." |
| `SWAP_FAILED` | "The swap transaction failed on the Hub. No funds were moved. Please try again." |
| `SWAP_NOT_FOUND` | "The swap operation could not be found. Please contact support with the operation reference." |
| `INVALID_REQUEST` | "The request was invalid. Please verify your inputs and try again." |
| unknown | Generic: "An unexpected error occurred." + collapsed `<details>` showing raw `error` and `errorCode` |
- [Try Again] `Button` → calls `reset()` (preserves form inputs in local component state)

**BRIDGE_OUT_FAILED critical alert** (visible when `step === "failed"` and `errorCode === "BRIDGE_OUT_FAILED"`):
- Prominent/destructive `Card` or alert section (distinct visual style from normal error panel)
- Title: "Critical: Partial Swap — Manual Action Required"
- Body: "Source funds were already debited. Manual reconciliation is required — contact your Central Bank with Swap ID: **{swapResult.swap_id}**."
- Two buttons:
  - [Acknowledge] → calls `acknowledgeSwapError()` (reveals [Reset Form] button)
  - [Reset Form] → only visible after `bridgeOutAcknowledged === true` → calls `reset()`
- No auto-retry. No [Try Again] button.

**QUOTE_EXPIRED auto-retry warning** (inline warning, not an error state):
- After the store auto-retries on `QUOTE_EXPIRED`, if the store re-enables the form, show a `Badge`/banner: "Quote was refreshed. Review and confirm again." until user edits amount or submits

**Acceptance**:
- `tsc --noEmit` passes; no `any` types
- Existing "Exact Output" tab renders exactly as before
- Cross-Currency tab: pool banner, quote form, TTL countdown, swap form, stepper, result panel, error panel, and BRIDGE_OUT_FAILED critical alert all render correctly
- [Execute Swap] is disabled while `step !== "idle"` (no duplicate submissions)
- `reset()` on BRIDGE_OUT_FAILED is gated behind explicit [Acknowledge] click
- No imports from `amm-v2.store.ts`

---

### T006 — Unit Tests: `calcMaxAmountIn` and `weiToDisplay`

| Field | Value |
|-------|-------|
| **Milestone** | M5 |
| **Target file** | `frontend/apps/bank/src/services/api/__tests__/cross-currency-swap.utils.test.ts` |
| **Effort** | S |
| **Depends on** | T002 |
| **Blocked by** | T002 |
| **Parallelizable** | Yes (after T002; independent of T003 and T004) |

**What to do**: Create a Vitest test file for the two exported pure functions. Do not mock axios or the API object — import only `calcMaxAmountIn` and `weiToDisplay` from the api module.

**`calcMaxAmountIn` test cases**:
```typescript
// Default slippage (1100 bps = 10%)
calcMaxAmountIn("1000000000000000000") → "1100000000000000000"
// Custom slippage
calcMaxAmountIn("1000000000000000000", 500) → "500000000000000000"
// Large wei value
calcMaxAmountIn("2000000000000000000000", 1100) → "2200000000000000000000"
// Zero input
calcMaxAmountIn("0") → "0"
```

**`weiToDisplay` test cases**:
```typescript
weiToDisplay("1000000000000000000")     → "1.000000"    // 1 ether
weiToDisplay("500000000000000000")      → "0.500000"    // 0.5 ether
weiToDisplay("1000000000000")           → "0.000001"    // 1 microether
weiToDisplay("0")                       → "0.000000"
weiToDisplay("")                        → "—"
weiToDisplay("not-a-number")            → "—"
// Custom decimals
weiToDisplay("1000000000000000000", 2)  → "1.00"
```

**Acceptance**:
- All test cases pass when run with `pnpm test` or `vitest run` from the `bank` app directory
- `tsc --noEmit` passes
- No axios calls, no mocking of HTTP clients

---

## Dependency Graph

```
T005 (export apiBaseV2)
  └─→ T001 (types)
        └─→ T002 (API client)           T006 [P] (tests — only needs T002)
              └─→ T003 (store)
                    └─→ T004 (UI)
```

**Execution order**: `T005 → T001 → T002 → T003 → T004`  
**Parallel opportunity**: `T006` can start as soon as `T002` is done, independently of `T003` and `T004`

---

## Summary

| Task | Milestone | Story | Effort | Depends On | Status |
|------|-----------|-------|--------|------------|--------|
| T005 | M4 (prereq) | — | S | — | ☐ |
| T001 | M1 | — | S | T005 | ☐ |
| T002 | M2 | US1, US2 | M | T001, T005 | ☐ |
| T003 | M3 | US1, US2, US4 | M | T001, T002 | ☐ |
| T004 | M4 | US1–US4 | L | T001, T002, T003 | ☐ |
| T006 | M5 | — | S | T002 | ☐ |

**Total tasks**: 6  
**P1 story tasks**: T002, T003, T004 (US1 + US2)  
**P2 story tasks**: T004 (US3 + US4 — same component)  
**Parallel opportunities**: T006 ∥ {T003, T004}  
**MVP scope**: T005 → T001 → T002 → T003 → T004 (all 5 tasks — this is a single-feature implementation; T006 is optional polish)

---

## Implementation Notes

1. **`http-client.ts` modification is minimal**: only add `export` keyword to existing `apiBaseV2` const. Touch nothing else.
2. **No `any` types**: enforced by NFR-005; use proper generics for axios response types (`axios.get<CrossCurrencyQuote>(...)`).
3. **BigInt arithmetic is mandatory**: `calcMaxAmountIn` and `weiToDisplay` must never use `Number()` or `parseFloat()` on wei strings.
4. **`BRIDGE_OUT_FAILED` is non-dismissable without acknowledgement**: `reset()` is a silent no-op until `acknowledgeSwapError()` is called. The UI must enforce this via the store guard.
5. **`amm-v2.store.ts` is untouched**: the new store is fully isolated. `PoolStatus` type is imported from `amm-v2.types.ts` (the types file, not the store).
6. **60s timeout is isolated**: only `crossCurrencyExecuteClient` has `timeout: 60_000`. `httpClientV2` must have `timeout === undefined` after this module loads.
