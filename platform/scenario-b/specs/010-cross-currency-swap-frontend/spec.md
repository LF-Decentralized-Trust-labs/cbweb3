# Feature Specification: Cross-Currency Swap Frontend

**Feature Branch**: `010-cross-currency-swap-frontend`
**Created**: 2026-05-27
**Status**: Draft
**Input**: Implement cross-currency swap flow in the bank frontend app, matching the new backend orchestrator introduced in `009-commercial-cross-currency-swap`.

## Context and Motivation

The `009-commercial-cross-currency-swap` spec introduced a backend orchestrator that executes a complete three-step cross-currency swap: bridge Spoke→Hub, swap W-tokens on the Sovereign AMM, and bridge Hub→Spoke. The existing `AMMTradingPage` only covers exact-output swaps (direct Hub swaps for Central Bank use). Commercial bank operators need a dedicated UI to:

1. **Get a time-bounded quote** for a cross-currency swap (BRL→ARS or any active pair), with a visible 15-second countdown.
2. **Execute the swap** through a single button that drives the orchestrator's synchronous 3-step flow.
3. **Track progress** in real-time via a stepper showing `BRIDGE_IN_PROGRESS → SWAP_IN_PROGRESS → BRIDGE_OUT_PROGRESS → COMPLETED`.
4. **Handle errors gracefully** with user-friendly messages per error code (`QUOTE_EXPIRED`, `SLIPPAGE_LIMIT_EXCEEDED`, `BRIDGE_IN_FAILED`, etc.).

This is a frontend-only feature. Backend endpoints are already implemented and stable. The new UI lives alongside the existing Exact Output tab without modifying or removing any existing functionality.

## Clarifications

### Session 2026-05-27

- Q: Should the Cross-Currency tab share the same Zustand store as the existing exact-output swap? → A: **No** — a dedicated store `useCrossCurrencySwapStore` is created. The existing `amm-v2.store.ts` is untouched. This isolates state, avoids regressions, and keeps the stores single-purpose.
- Q: How should the 60-second HTTP timeout be handled without breaking all other API calls? → A: **Dedicated axios instance** — `cross-currency-swap.api.ts` creates its own axios instance with `timeout: 60_000` only for the `POST /amm/swap/cross-currency` call. The shared `httpClientV2` remains unchanged (no timeout) so other endpoints are unaffected.
- Q: What happens if the backend returns COMPLETED synchronously vs. requires polling? → A: **Dual handling** — if `executeSwap` response has `status === "COMPLETED"`, store transitions directly to `step: completed`. If status is non-terminal (e.g., network cut, intermediate state), store launches polling every 2s for up to 90 attempts (3 minutes) before declaring `step: failed`.
- Q: Should the quote auto-refresh while the user is looking at it, or only on demand? → A: **Auto-refresh every 10s** via `usePolling` hook while the Cross-Currency tab is open and a quote exists. The user can also manually click [Get Quote] at any time. Auto-refresh stops when the swap is submitted (`step !== idle`).
- Q: How should the `max_amount_in` be computed client-side? → A: `calcMaxAmountIn(amountInWei, slippageBps = 1100)` = `(BigInt(amountInWei) * BigInt(slippageBps) / 1000n).toString()` — a 10% default slippage buffer on top of the quote's `amount_in`. Exported from `cross-currency-swap.api.ts` for use in both store and UI.
- Q: What is the `weiToDisplay` implementation contract? → A: **BigInt-only** — divides by `10n ** 18n` and formats to up to 6 decimal places. Never uses `parseFloat` on wei strings. Returns `"—"` for non-numeric or empty input. No floating-point arithmetic at any stage.
- Q: What should the UI show when the backend returns `BRIDGE_OUT_FAILED`? → A: **Critical persistent error alert** displaying the `swap_id` explicitly. Message: source funds were already debited and manual reconciliation with the Central Bank is required. No auto-retry. The alert persists and cannot be cleared by [Try Again] — only [Reset Form] after operator acknowledges.
- Q: Does switching to the Cross-Currency tab trigger automatic data fetches? → A: **Yes** — mounting the Cross-Currency tab automatically calls `fetchPoolStatus("W-BRL-ARS")`. The cross-currency store is fully independent from `amm-v2.store.ts`; no state leaks in either direction.
- Q: Should the Cross-Currency UI be a new page/route or colocated with the existing AMM page? → A: **New tab** inside the existing `Tabs` component in `AMMTradingV2` (`src/pages/AMMTradingPage.tsx`). Not a new route or page. Coexists with "Exact Output" tab without any routing change.
- Q: What are the default values for `source_currency`, `target_currency`, and `pool_pair`? → A: `source_currency = "BRL"`, `target_currency = "ARS"`, `pool_pair = "W-BRL-ARS"`. Pre-filled as defaults; operator may edit. Whether these are hardcoded constants or editable inputs is an implementation detail deferred to plan.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Get Quote and Execute Cross-Currency Swap (Priority: P1)

As a commercial bank operator, I want to get a real-time quote for swapping BRL to ARS and execute the swap through a single confirmation flow, so that I can initiate international payments without manually tracking each bridge and swap step.

**Why this priority**: This is the entire reason for the feature. Without a working quote-and-execute flow, no value is delivered.

**Independent Test**: A bank-a operator opens the AMM Trading page, selects the Cross-Currency tab, enters `source_currency=BRL`, `target_currency=ARS`, and an `amount_out` in wei. They click [Get Quote], see `effective_rate` and `amount_in` displayed in human-readable form with a 15s countdown, fill in `payer_bank_id` and `beneficiary_bank_id`, click [Execute Swap], and watch the 3-step stepper reach COMPLETED with `swap_tx_hash` and bridge position IDs displayed.

**Acceptance Scenarios**:

1. **Given** the pool `W-BRL-ARS` is ACTIVE, **When** the operator clicks [Get Quote] with valid inputs, **Then** the UI displays `effective_rate`, `amount_in` in human-readable units (e.g., "1 020.00 BRL"), `amount_out` in human-readable units (e.g., "2 000.00 ARS"), and a countdown timer starting at 15 seconds.

2. **Given** a valid unexpired quote is displayed, **When** the operator enters `payer_bank_id`, `beneficiary_bank_id` and clicks [Execute Swap], **Then** the step indicator transitions: `submitting → BRIDGE_IN_PROGRESS → SWAP_IN_PROGRESS → BRIDGE_OUT_PROGRESS → COMPLETED`.

3. **Given** the swap completes synchronously (backend returns `status: COMPLETED`), **Then** the result panel shows `swap_tx_hash`, `bridge_in_position_id`, `bridge_out_position_id`, `amount_in` and `amount_out` in human-readable form.

4. **Given** the [Execute Swap] button is clicked, **Then** the button is immediately disabled and remains disabled until the step reaches `completed` or `failed` (prevents duplicate submissions).

---

### User Story 2 — Quote Expiry and Auto-Refresh (Priority: P1)

As a commercial bank operator, I want the quote to automatically refresh before it expires and to receive a clear error if I try to submit an expired quote, so that I always see an accurate price and am never surprised by silent rejections.

**Why this priority**: Quote TTL is 15 seconds. Without auto-refresh, the operator would constantly see stale data and get QUOTE_EXPIRED errors on submission.

**Independent Test**: Operator opens Cross-Currency tab, gets a quote, waits without submitting. The countdown reaches zero and a new quote auto-loads. Operator then submits; the swap succeeds. In a second test, operator disables JS timers (or uses a paused network), waits 16s, and submits — the UI shows a "Quote expired — fetching new quote" message and auto-retries once before re-enabling the Execute button.

**Acceptance Scenarios**:

1. **Given** a quote is active and 10 seconds have passed since it was fetched, **Then** the store automatically calls `fetchQuote` again with the same inputs and replaces the displayed quote with updated values and a fresh 15s countdown.

2. **Given** a quote has expired (`quoteExpiresAt < Date.now()`), **When** the operator clicks [Execute Swap], **Then** the [Execute Swap] button is disabled and shows tooltip "Quote expired — click Get Quote to refresh".

3. **Given** the backend returns `QUOTE_EXPIRED` on `executeSwap`, **Then** the store automatically calls `fetchQuote` once (auto-retry), updates the quote, and re-enables the [Execute Swap] button — displaying a warning "Quote was refreshed. Review and confirm again." rather than a hard error.

4. **Given** auto-refresh is active (`step === idle`) and the user starts typing in the amount_out field, **Then** the existing auto-refresh timer is cancelled and a new quote fetch is triggered after 600ms debounce.

---

### User Story 3 — Pool Status Banner and Pre-flight Checks (Priority: P2)

As a commercial bank operator, I want to see the pool's current status at the top of the Cross-Currency tab, so that I know before filling in forms whether the pool is available and can avoid wasted effort when it is not ACTIVE.

**Why this priority**: Prevents frustration from filling in forms only to get a backend rejection. Improves UX but does not gate the core swap flow (P1 already validates at execution).

**Independent Test**: Operator opens Cross-Currency tab while pool is HALTED. A red banner reads "Pool W-BRL-ARS is currently paused. Contact your Central Bank for status." [Get Quote] and [Execute Swap] buttons are disabled. After pool is restored to ACTIVE (simulated by refreshing pool status), the banner turns green and buttons re-enable.

**Acceptance Scenarios**:

1. **Given** pool status is ACTIVE, **Then** a green banner at the top of the Cross-Currency tab shows "Pool W-BRL-ARS — ACTIVE" with the current ratio.

2. **Given** pool status is not ACTIVE (EMPTY, PENDING_COUNTERPART, or HALTED), **Then** a red/amber banner shows the pool state and a descriptive message; [Get Quote] and [Execute Swap] buttons are disabled.

3. **Given** the Cross-Currency tab is mounted, **Then** `fetchPoolStatus("W-BRL-ARS")` is called automatically on mount and the banner reflects the result within one render cycle.

4. **Given** the operator is on the Cross-Currency tab, **Then** pool status auto-refreshes every 30 seconds in the background.

---

### User Story 4 — Error Display with Actionable Messages (Priority: P2)

As a commercial bank operator, I want every swap failure to display a clear, non-technical message with a suggested action, so that I know what to do without escalating to a developer.

**Why this priority**: Operators are non-technical. Generic error messages create support load. Per-code messages are table stakes for production UX.

**Independent Test**: Developer triggers each error code by mocking the API (or using tryout scripts) and verifies that the correct human-readable message is displayed in the error panel.

**Acceptance Scenarios**:

1. **Given** the backend returns `CIRCUIT_BREAKER_HALTED`, **Then** the error panel shows: "Pool is temporarily paused by the Central Bank. Please try again later or contact your Central Bank."

2. **Given** the backend returns `SLIPPAGE_LIMIT_EXCEEDED`, **Then** the error panel shows: "Price moved beyond your slippage tolerance. Please refresh the quote and try again."

3. **Given** the backend returns `BRIDGE_IN_FAILED`, **Then** the error panel shows: "Could not lock your funds on the source network. The operation has been rolled back. No funds were lost."

4. **Given** the backend returns `INSUFFICIENT_POOL_LIQUIDITY`, **Then** the error panel shows: "Not enough liquidity in the pool for this amount. Try a smaller amount or wait for the Central Bank to add liquidity."

5. **Given** an unrecognised error code is returned, **Then** the error panel shows a generic fallback message and the raw `error_code` in a collapsed details section.

6. **Given** a step reaches `failed`, **Then** a [Try Again] button is shown that calls `reset()` and returns the form to the `idle` state, preserving the previously entered `source_currency`, `target_currency`, and `amount_out` inputs.

7. **Given** the backend returns `BRIDGE_OUT_FAILED`, **Then** a **critical persistent error alert** is shown (not a transient error panel) displaying the `swap_id` and the message: "Source funds were already debited. Manual reconciliation is required — contact your Central Bank with Swap ID: {swap_id}." No auto-retry is offered. Only [Dismiss] and [Reset Form] actions are available, and the operator must explicitly acknowledge before the form resets.

---

### Edge Cases

- **Quote auto-refresh triggers while step is submitting**: auto-refresh is suspended when `step !== idle`; no new quote fetch fires during execution.
- **BigInt overflow on wei display**: `weiToDisplay` must guard against non-numeric or empty strings and return `"—"` instead of crashing the render.
- **Backend times out after 60s**: the dedicated axios instance cancels the request; store sets `step: failed` with error `"Request timed out. The swap may still be in progress — check with your Central Bank."` and `pollSwapStatus` is NOT called (no swap_id available).
- **Poll reaches 90 attempts without terminal state**: store sets `step: failed` with error `"Swap status unknown after 3 minutes. Please contact your Central Bank with correlation_id."` and displays the last known non-terminal `swapResult.status`.
- **Tab switch while quote countdown is running**: countdown and auto-refresh timers must be cleaned up via `useEffect` return function to prevent memory leaks and ghost fetches.
- **User clears `amount_out` field**: existing quote is cleared (`clearQuote` equivalent), auto-refresh timer is cancelled.
- **Concurrent tab open in two browser windows**: each window has independent Zustand store (in-memory); no cross-tab synchronisation is needed.
- **`BRIDGE_OUT_FAILED` — partial execution state**: source funds are debited on the Hub but target bridge-out failed. The error alert MUST NOT be cleared by `reset()` alone — it requires explicit operator acknowledgement. `reset()` only re-enables the form after acknowledgement so the operator cannot accidentally re-submit without reading the reconciliation warning.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The frontend MUST add a "Cross-Currency" tab inside `AMMTradingV2` component in `src/pages/AMMTradingPage.tsx`, coexisting with the existing "Exact Output" tab. All existing exact-output functionality MUST remain unchanged.

- **FR-002**: A new TypeScript file `src/types/cross-currency-swap.types.ts` MUST define `CrossCurrencyQuote`, `CrossCurrencySwapRequest`, `CrossCurrencySwapStatus` (union type), `CrossCurrencySwapResult`, `CROSS_CURRENCY_SWAP_ERROR` const object, and `CrossCurrencySwapErrorCode` type, covering all fields documented in the backend integration guide.

- **FR-003**: A new API client file `src/services/api/cross-currency-swap.api.ts` MUST expose `crossCurrencySwapApi` with methods: `getPoolStatus(pair)`, `getQuote(sourceCurrency, targetCurrency, amountOut)`, `executeSwap(payload)`, and `getSwapStatus(swapId)`. The `executeSwap` method MUST use a dedicated axios instance configured with `timeout: 60_000`; all other methods MUST use the standard `httpClientV2`. The shared `httpClientV2` in `http-client.ts` MUST NOT be modified.

- **FR-004**: The file MUST also export utility functions `calcMaxAmountIn(amountInWei: string, slippageBps?: number): string` (default `slippageBps = 1100`, computed via BigInt arithmetic) and `weiToDisplay(wei: string, decimals?: number): string` (default `decimals = 6`). Both functions MUST use BigInt (not `parseFloat`) for all arithmetic to avoid precision loss on 18-decimal wei values.

- **FR-005**: A new Zustand store `src/features/amm/cross-currency-swap.store.ts` MUST manage state: `poolStatus`, `quote`, `quoteExpiresAt`, `swapResult`, `step` (`idle | quoting | submitting | polling | completed | failed`), `error`, `errorCode`. Actions MUST include `fetchPoolStatus`, `fetchQuote`, `executeSwap`, `pollSwapStatus`, and `reset`.

- **FR-006**: The `executeSwap` action MUST: (a) set `step: submitting`; (b) call `crossCurrencySwapApi.executeSwap`; (c) if response `status === "COMPLETED"`, set `step: completed` and `swapResult`; (d) if response is non-terminal, call `pollSwapStatus(swapId)` with 2-second intervals, up to 90 attempts; (e) if backend returns `QUOTE_EXPIRED`, auto-retry once by calling `fetchQuote` then re-submitting with the new `quote_id`; (f) on any terminal failure, set `step: failed`, `error`, and `errorCode`.

- **FR-007**: The Cross-Currency tab UI MUST contain: (1) a pool status banner driven by `poolStatus.pool_status`; (2) a quote form with `source_currency`, `target_currency`, `amount_out` inputs and a [Get Quote] button; (3) a quote display panel showing `effective_rate`, `amount_in` in human-readable form (`weiToDisplay`), and a TTL countdown in seconds; (4) a swap confirmation section with `payer_bank_id`, `beneficiary_bank_id` inputs and a [Execute Swap] button; (5) a 3-step progress stepper showing BRIDGE_IN → SWAP → BRIDGE_OUT with the current `swapResult.status` highlighted; (6) a result panel (when `step === completed`) showing `swap_tx_hash`, `bridge_in_position_id`, `bridge_out_position_id`, `amount_in`, `amount_out` in human-readable form; (7) an error panel (when `step === failed`) with per-error-code messages and a [Try Again] button.

- **FR-008**: Quote auto-refresh MUST be implemented via a `useEffect`-based interval (or a `usePolling` hook) that calls `fetchQuote` every 10 seconds while `step === idle` and a quote exists. The interval MUST be cleared when `step` changes to a non-idle value or when the component unmounts.

- **FR-009**: The [Execute Swap] button MUST be disabled when any of the following conditions are true: `poolStatus?.pool_status !== "ACTIVE"`, `step !== "idle"`, `quote === null`, or `quoteExpiresAt !== null && quoteExpiresAt < Date.now()`.

- **FR-010**: The 3-step progress stepper MUST map `swapResult.status` to a visual state: `BRIDGE_IN_PROGRESS` → step 1 active; `SWAP_IN_PROGRESS` → step 2 active; `BRIDGE_OUT_PROGRESS` → step 3 active; `COMPLETED` → all steps complete; `FAILED` → current step shows error state.

- **FR-011**: Per-error-code user-facing messages MUST be defined in a static map within the Cross-Currency tab component (or a co-located `errorMessages.ts`), covering at minimum: `POOL_NOT_ACTIVE`, `CIRCUIT_BREAKER_HALTED`, `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY`, `QUOTE_EXPIRED`, `BRIDGE_IN_FAILED`, `SWAP_FAILED`, `BRIDGE_OUT_FAILED`, `SWAP_NOT_FOUND`, `INVALID_REQUEST`, and a fallback for unknown codes. The `BRIDGE_OUT_FAILED` message MUST be rendered as a **critical persistent alert** (visually distinct from the standard error panel) that displays the `swap_id` and explicitly states that source funds were debited and manual reconciliation is required. It MUST NOT be dismissable via the standard [Try Again] reset path without explicit operator acknowledgement.

- **FR-012**: The `reset()` store action MUST clear `quote`, `quoteExpiresAt`, `swapResult`, `step` (back to `idle`), `error`, and `errorCode`, without clearing `poolStatus` (pool status should persist across resets to avoid an extra network call).

### Non-Functional Requirements

- **NFR-001**: No existing tests or behaviours in `amm-v2.store.ts`, `amm-v2.api.ts`, `amm-v2.types.ts`, or the existing Exact Output tab UI MUST be modified or broken by this feature.

- **NFR-002**: All BigInt arithmetic in `calcMaxAmountIn` and `weiToDisplay` MUST be covered by unit tests that assert correct output for representative wei values (e.g., `1000000000000000000` = `"1.000000"`).

- **NFR-003**: The 60-second axios timeout MUST be isolated to the dedicated `executeSwap` axios instance. A unit test MUST assert that `httpClientV2` does not have a `timeout` configured and that the cross-currency execute instance has `timeout === 60000`.

- **NFR-004**: The `pollSwapStatus` loop MUST NOT run indefinitely. It MUST stop at 90 iterations (3 minutes at 2s interval) and set `step: failed` with an appropriate message if no terminal state is reached.

- **NFR-005**: All new components and store actions MUST be written in TypeScript with strict types. No `any` types in files introduced by this feature.

- **NFR-006**: The cross-currency UI MUST be responsive and follow the existing Tailwind CSS + `@cbweb3/ui` design conventions used in the rest of `frontend/apps/bank/`.

## Key Entities / Types

- **`CrossCurrencyQuote`**: Represents a time-bounded price quote returned by `GET /amm/quote/cross-currency`. Key fields: `quote_id`, `amount_in` (wei string), `amount_out` (wei string), `effective_rate` (decimal string), `fee_bps`, `max_slippage_pct`, `pool_pair`, `time_remaining_seconds`, `valid_until` (unix timestamp), `created_at`.

- **`CrossCurrencySwapRequest`**: Request body for `POST /amm/swap/cross-currency`. Fields: `source_currency`, `target_currency`, `pool_pair`, `amount_out` (wei string), `max_amount_in` (wei string), `payer_bank_id`, `beneficiary_bank_id`, `quote_id` (optional).

- **`CrossCurrencySwapStatus`**: Union type `"QUOTING" | "BRIDGE_IN_PROGRESS" | "SWAP_IN_PROGRESS" | "BRIDGE_OUT_PROGRESS" | "COMPLETED" | "FAILED"`. Terminal states are `COMPLETED` and `FAILED`.

- **`CrossCurrencySwapResult`**: Response from both `POST /amm/swap/cross-currency` and `GET /amm/swap/cross-currency/:id`. Fields: `swap_id`, `correlation_id`, `status` (CrossCurrencySwapStatus), `amount_in` (wei string, optional), `amount_out` (wei string, optional), `effective_rate` (optional), `bridge_in_position_id` (optional), `swap_tx_hash` (optional), `bridge_out_position_id` (optional), `created_at` (optional).

- **`CrossCurrencySwapErrorCode`**: Type derived from `CROSS_CURRENCY_SWAP_ERROR` const: `"POOL_NOT_ACTIVE" | "CIRCUIT_BREAKER_HALTED" | "SLIPPAGE_LIMIT_EXCEEDED" | "INSUFFICIENT_POOL_LIQUIDITY" | "QUOTE_EXPIRED" | "BRIDGE_IN_FAILED" | "SWAP_FAILED" | "BRIDGE_OUT_FAILED" | "SWAP_NOT_FOUND" | "INVALID_REQUEST"`.

- **`CrossCurrencySwapStoreState`**: Zustand state shape. Fields: `poolStatus: PoolStatus | null`, `quote: CrossCurrencyQuote | null`, `quoteExpiresAt: number | null`, `swapResult: CrossCurrencySwapResult | null`, `step: "idle" | "quoting" | "submitting" | "polling" | "completed" | "failed"`, `error: string | null`, `errorCode: CrossCurrencySwapErrorCode | null`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A commercial bank operator can complete the full quote → execute → confirmed flow in the UI within 60 seconds of receiving a successful COMPLETED response from the backend, measured manually against a running local environment.

- **SC-002**: All 10 defined `CROSS_CURRENCY_SWAP_ERROR` codes produce distinct, non-empty, user-readable messages in the error panel — verified by a unit/integration test that exercises the error message map with each code.

- **SC-003**: The [Execute Swap] button is never enabled when `step !== "idle"`, verified by a component test that checks the disabled attribute across all non-idle step values.

- **SC-004**: `weiToDisplay("1000000000000000000", 6)` returns `"1.000000"` and `calcMaxAmountIn("1000000000000000000", 1100)` returns `"1100000000000000000"`, verified by unit tests.

- **SC-005**: Switching to the Exact Output tab and back to the Cross-Currency tab does not cause any visible regressions in the Exact Output tab (quote, swap, approve AMM panel all function as before), verified manually.

- **SC-006**: No TypeScript compilation errors are introduced by the new files (`cross-currency-swap.types.ts`, `cross-currency-swap.api.ts`, `cross-currency-swap.store.ts`, and changes to `AMMTradingPage.tsx`), verified by `tsc --noEmit` passing cleanly.

## Assumptions

- The backend endpoints (`GET /amm/quote/cross-currency`, `POST /amm/swap/cross-currency`, `GET /amm/swap/cross-currency/:id`) are deployed and reachable at the same `baseURL` used by `httpClientV2` (i.e., no separate host or port).
- The `httpClientV2` axios instance in `src/services/api/http-client.ts` currently has no `timeout` set; the cross-currency API client will create its own instance inheriting `baseURL` and `withCredentials: true` from `httpClientV2` defaults.
- The existing `PoolStatus` interface in `amm-v2.types.ts` already has the same shape as the pool status response from `GET /amm/pool/{pair}/status`; `crossCurrencySwapApi.getPoolStatus` reuses it without introducing a duplicate type.
- The frontend monorepo uses `@cbweb3/ui` for shared UI primitives (buttons, tabs, badges, cards). Tabs and stepper-style components are available or can be composed from existing primitives without adding new external dependencies.
- `usePolling` is either already defined in `frontend/apps/bank/src/hooks/` or can be implemented as a simple `useEffect`-based interval within the store action — no new library is required.
- The `amount_out` field in the quote form is entered by the operator in wei (raw 18-decimal integer string), consistent with the backend contract. A helper display below the input converts the entered value to human units via `weiToDisplay` for confirmation before submission.
- Cookie-based authentication is already handled transparently by `httpClientV2`; the cross-currency API client inherits `withCredentials: true` and requires no additional auth logic.
- The pair `W-BRL-ARS` is the primary development/test pair. The UI hardcodes this as the default `pool_pair` value; it is pre-filled but editable by the operator.

## Out of Scope

- Backend changes of any kind — this spec is frontend-only.
- Changes to `amm-v2.types.ts`, `amm-v2.api.ts`, `amm-v2.store.ts`, or the Exact Output tab UI.
- A pair-discovery dropdown populated from `GET /api/v2/amm/pairs` — the pair field is a free-text input pre-filled with `W-BRL-ARS` for the MVP.
- Mobile-specific layout adjustments beyond what Tailwind responsive utilities provide automatically.
- End-to-end (Playwright/Cypress) tests — unit and integration tests for utilities and store actions are in scope; full browser automation tests are not.
- Internationalisation (i18n) of error messages — English strings only for MVP.
- Accessibility (WCAG) audit — standard HTML semantics are expected but a formal audit is out of scope.

## Before / After Summary

| Surface | Before (010) | After (010) |
|---------|-------------|-------------|
| `src/types/` | Only `amm-v2.types.ts` (exact-output types) | + `cross-currency-swap.types.ts` with 5 new types/consts |
| `src/services/api/` | `amm-v2.api.ts` (exact-output endpoints only) | + `cross-currency-swap.api.ts` (4 methods + 2 utilities + 2 constants) |
| `src/features/amm/` | `amm-v2.store.ts` (exact-output store only) | + `cross-currency-swap.store.ts` (5 actions, 7-field state) |
| `src/pages/AMMTradingPage.tsx` | Single "Exact Output" form | + "Cross-Currency" tab with quote form, stepper, result panel, error panel |
| `src/services/api/http-client.ts` | Unchanged | Unchanged (timeout isolation done in new file) |
| Axios timeout | No timeout on `httpClientV2` | 60s timeout on dedicated cross-currency execute instance only |
