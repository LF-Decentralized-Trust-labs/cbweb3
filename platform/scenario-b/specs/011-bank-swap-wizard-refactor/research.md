# Research: Bank Swap Wizard Refactor

**Feature**: `011-bank-swap-wizard-refactor`  
**Date**: 2026-05-28

---

## Decision Log

### D-001: Wizard state management — Zustand store vs local component state

**Decision**: New `useSwapWizardStore` Zustand store (session-scope, no persistence).  
**Rationale**: Step data (deposit_id, escrow_id, swap_id, redeem_id, amounts) must survive React re-renders during polling and must be readable by multiple sub-components within the wizard page. Local `useState` would require prop-drilling or lifting state into the page component. A Zustand store keeps the pattern consistent with the rest of the bank app (`usePaymentStore`, `useCrossCurrencySwapStore`, `useAmmV2Store`) and is entirely session-scoped (no persistence).  
**Alternatives considered**: React Context (more boilerplate, same scope), URL params (not appropriate for sensitive in-progress IDs), `useReducer` lifted to page (simpler but inconsistent with app pattern).

### D-002: Polling implementation — reuse `usePolling` hook

**Decision**: Reuse the existing `usePolling` hook from `frontend/apps/bank/src/hooks/usePolling.ts` at 5 000 ms intervals for deposit, escrow, and redeem approval polling. No new polling infrastructure needed.  
**Rationale**: Spec assumption explicitly states `usePolling` already exists and can be reused without modification. This is confirmed in the codebase (`CooperativeLiquidityWizard.tsx` uses it).  
**Alternatives considered**: `setInterval` in the store (harder to clean up, no React lifecycle integration), React Query (not in the dependency set).

### D-003: Cross-currency swap execution — synchronous hold, no polling

**Decision**: Extend `crossCurrencyExecuteClient` timeout to 300 000 ms. Display progress stages (bridge-in → swap → bridge-out) using the `step` field already present in `useCrossCurrencySwapStore`. No fire-and-forget or polling pattern.  
**Rationale**: Backend orchestrator is synchronous; it holds the HTTP connection until COMPLETED or failure. The existing `cross-currency-swap.store.ts` already tracks `step: SwapStep` through "submitting" → "polling" → "completed" / "failed". The progress stepper is a UI overlay on a single awaited `executeSwap` call.  
**Alternatives considered**: Polling on swap status (not required; adds unnecessary complexity and round-trips given synchronous backend).

### D-004: `payer_bank_id` — derive from auth store, never from form state

**Decision**: Remove `payerBankId` from `CrossCurrencySwapStore` state. At call site in Step 4 and in `AMMTradingV2`, derive `payer_bank_id` from `useAuthStore(state => state.profile?.bankId)` and pass it directly to `executeSwap`.  
**Rationale**: SC-002 mandates zero cases where operator can submit a swap with a `payer_bank_id` different from authenticated session. Keeping it as editable form state is the root cause of the existing security gap.  
**Alternatives considered**: Hidden form field (still modifiable via dev tools), server-side rejection (adds round-trip; UX fix is preferred and complementary).

### D-005: AMM approval `side` — omit entirely from request

**Decision**: Make `ApproveAmmRequest.side` optional (`side?: "A" | "B"`). In `approveAmm` store action, omit `side` from the request payload when undefined. Remove the side selector UI from both the AMM Trading page approve panel and Step 3 of the wizard.  
**Rationale**: FR-016 / FR-005 — swap side is resolved server-side via `BANK_CODE` environment variable. The selector is an unnecessary operator input that could cause errors.  
**Alternatives considered**: Always send a hardcoded side (incorrect — the server already knows); keep selector but disable it (misleading UX).

### D-006: SwapMonitorPage swap history — session Zustand state, GET by ID

**Decision**: `SwapMonitorPage` reads a `swapIds: string[]` list from a shared session Zustand store (populated when `CommercialSwapWizardPage` completes Step 4). Fetches each swap record individually via `GET /api/v2/amm/swap/cross-currency/{id}`. Displays "No swap records in this session" when the list is empty.  
**Rationale**: No list endpoint exists (confirmed in spec assumptions and clarifications). The governance app and bank app are separate; they do not share Zustand state. `swapIds` must be stored somewhere the governance app can access. Since the governance app has its own session, it tracks only IDs for swaps initiated during its session — or rather, per clarification, the monitoring page reads from **its own** Zustand session store seeded by another mechanism. Re-reading the spec: "Zustand session IDs" in governance context means the governance app tracks swap IDs submitted via the bank app visible in the same browser session... but governance and bank are separate apps. The correct interpretation: `SwapMonitorPage` in the governance app has its own local Zustand store that is empty by default; it shows "No swap records in this session" unless the operator manually enters a swap ID or the governance app is integrated with a shared store. Since no shared store exists across apps, the simplest correct implementation is: `SwapMonitorPage` has a small local Zustand store that starts empty and shows "No swap records in this session" — satisfying FR-019 and the edge case. If swap IDs need to be tracked, the operator can note them from the bank app. This is consistent with the spec edge case: "If no swap IDs are tracked in the current session, shows 'No swap records in this session' without error."  
**Alternatives considered**: LocalStorage sharing across apps (cross-origin issues, not in scope); URL-based swap ID input (nice-to-have, not in spec).

### D-007: Navigation hiding in Scenario B — route config only

**Decision**: Remove `DepositsPage`, `EscrowsPage`, and `AMMTradingPage` from `scenarioBChildren` nav-visible array in `frontend/apps/bank/src/routes/index.tsx`, but keep the route objects with `path` so direct URL access still works. Add a `/swap` route entry pointing to `CommercialSwapWizardPage`.  
**Rationale**: FR-022 — navigation entries hidden, but routes accessible via direct URL. The existing pattern in the bank app already has `scenarioBChildren` as a separate array; simply curating it is the minimal change.  
**Alternatives considered**: Sidebar conditional rendering (brittle, duplicates route logic); route-level redirect from deposit/escrow/redeem (breaks power-user direct access).

---

## Technology Clarifications (all resolved — no unknowns)

- **`usePolling` hook**: Exists at `frontend/apps/bank/src/hooks/usePolling.ts`, confirmed via usage in `CooperativeLiquidityWizard.tsx`.
- **`useAuthStore` bankId field**: Confirmed at `frontend/apps/bank/src/stores/auth.store.ts` — `profile.bankId` available.
- **`usePaymentStore` actions**: `registerDeposit`, `requestEscrow`, `requestRedeem`, `fetchAll` confirmed via usage in `EscrowsPage.tsx`, `DepositsPage.tsx`, `RedeemsPage.tsx`.
- **`useCrossCurrencySwapStore`**: Confirmed in `frontend/apps/bank/src/features/amm/cross-currency-swap.store.ts`; has `step: SwapStep`, `swapResult`, `error`, `errorCode`, `bridgeOutAcknowledged`.
- **No new external packages required**: Confirmed — all necessary primitives exist.
- **`CooperativeLiquidityWizard` pattern**: Step grid with numbered indicators, per-step Card forms, `usePolling` for approval steps, auto-advance on status change, per-step error display — confirmed in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`.
