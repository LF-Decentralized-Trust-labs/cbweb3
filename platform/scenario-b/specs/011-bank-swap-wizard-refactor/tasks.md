# Tasks: Bank Swap Wizard Refactor

**Feature**: `011-bank-swap-wizard-refactor`
**Input**: `specs/011-bank-swap-wizard-refactor/` (spec.md, plan.md, data-model.md)
**Branch**: `011-bank-swap-wizard-refactor`
**Verification**: `tsc --noEmit` + `npm run lint` in `frontend/apps/bank` and `frontend/apps/governance`

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no shared dependencies)
- **[US1]**: User Story 1 — Guided Swap Wizard (bank app, new `/swap` page)
- **[US2]**: User Story 2 — AMM Trading Page fixes (bank app, existing page)
- **[US3]**: User Story 3 — Governance Swap Monitor (governance app, new `/swap-monitor` page)

---

## Phase 1: Foundational — Type & Store Fixes

**Purpose**: Correct four broken/unsafe field definitions and eliminate the mutable `payerBankId` security gap. These changes unblock both US1 and US2 — no user story work may begin until this phase passes `tsc --noEmit`.

**All four tasks are independent and can run in parallel.**

- [X] T001 [P] Edit `frontend/apps/bank/src/types/amm-v2.types.ts`: change `AMMQuote.price_impact` from `number` to `string`; change `AMMQuote.quote_timestamp` from `string` to `number`; remove `SwapOrder.amount_out` field entirely; make `ApproveAmmRequest.side` optional (`side?: "A" | "B"`)
- [X] T002 [P] Edit `frontend/apps/bank/src/features/amm/amm-v2.store.ts`: update `approveAmm` signature to accept `side` as an optional second parameter; omit the `side` key from the request payload entirely when `side` is `undefined`
- [X] T003 [P] Edit `frontend/apps/bank/src/services/api/cross-currency-swap.api.ts`: change the `crossCurrencyExecuteClient` Axios instance `timeout` from `60_000` to `300_000` (backend orchestrator is synchronous and holds the connection up to 5 min)
- [X] T004 [P] Edit `frontend/apps/bank/src/features/amm/cross-currency-swap.store.ts`: remove `payerBankId` field from store state and from any `set(...)` calls; at the `executeSwap` call site, read `payer_bank_id` from `useAuthStore.getState().profile.bankId` instead of store state

**Checkpoint**: Run `tsc --noEmit` in `frontend/apps/bank` — must pass with zero errors before proceeding to Phase 2 or Phase 3.

---

## Phase 2: User Story 1 — Guided Swap Wizard (Priority: P1) 🎯 MVP

**Goal**: Commercial bank operator completes the full BRL-ARS cross-currency swap flow — token issuance → reserve tokenisation → AMM approval → swap → redemption — in a single guided five-step wizard with automatic step advancement, pre-filled IDs, and a session-locked `payer_bank_id`.

**Independent Test**: Operator logs in, navigates to `/swap`, enters a fiat amount, and follows all five steps to a redemption confirmation with no manual ID re-entry between steps and no client-side timeout during swap execution.

### Implementation

- [X] T005 [P] [US1] Create `frontend/apps/bank/src/features/swap-wizard/swap-wizard.store.ts`: Zustand store (no `persist` middleware) implementing `SwapWizardSession` from data-model.md — state fields: `currentStep: WizardStep` (1–5), `depositFiatAmount: string`, `depositId: string | null`, `depositStatus: string | null`, `escrowId: string | null`, `escrowAmount: string | null`, `escrowStatus: string | null`, `approveAmount: string`, `amountOut: string`, `swapId: string | null`, `swapStatus: CrossCurrencySwapStep`, `redeemAmount: string | null`, `redeemId: string | null`, `redeemStatus: string | null`, `stepError: Record<WizardStep, string | null>`, `isPolling: boolean`; actions: `setStep`, `setDepositId(id, status?)`, `setDepositStatus`, `setEscrowId(id, amount?)`, `setEscrowStatus`, `setApproveAmount`, `setSwapId`, `setSwapStatus`, `setRedeemId`, `setRedeemStatus`, `setStepError(step, msg)`, `clearStepError(step)`, `reset`
- [X] T006 [P] [US1] Create `frontend/apps/bank/src/features/swap-wizard/useSwapWizardPolling.ts`: hook signature `useSwapWizardPolling(type: "deposit" | "escrow" | "redeem", id: string | null, onApproved: () => void)` — uses existing `usePolling` hook at 5 000 ms interval; each tick calls `usePaymentStore.fetchAll()` to refresh; reads the relevant status from payment store by `id`; invokes `onApproved` callback when status transitions to `"APPROVED"`; stops polling when `id` is `null` or when 60 attempts (5 min) are exhausted
- [X] T007 [US1] Create `frontend/apps/bank/src/pages/CommercialSwapWizardPage.tsx`: five-step wizard following the `CooperativeLiquidityWizard` component pattern from `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx` — Step indicator grid (1–5); **Step 1**: fiat amount `<Input>`, "Register Deposit" `<Button>` → `usePaymentStore.registerDeposit(fiatAmount)`, `depositId` shown read-only after registration, `useSwapWizardPolling("deposit", depositId, advanceToStep2)` → auto-advance; **Step 2**: `depositId` read-only pre-filled, "Request Escrow" `<Button>` → `usePaymentStore.requestEscrow(depositId)`, `escrowId` shown read-only, `useSwapWizardPolling("escrow", escrowId, advanceToStep3)` → auto-advance; **Step 3**: `approveAmount` `<Input>` pre-filled from `escrowAmount` (editable), tCeBM balance display from `usePaymentStore.balance`, non-blocking `<Badge>` warning if `approveAmount > balance` (do not block submission), "Approve AMM" `<Button>` → `useAmmV2Store.approveAmm(approveAmount)` with no `side` argument, manual "Next" button to advance; **Step 4**: `payer_bank_id` `<Input readOnly>` from `useAuthStore.profile.bankId` (not editable), `beneficiaryId` `<Input>`, pair BRL-ARS fixed/read-only, quote panel via `useCrossCurrencySwapStore.fetchQuote()` with 15 s TTL countdown and refresh button, `maxAmountIn` auto-computed via `calcMaxAmountIn(quote.required_input)`, progress stepper (bridge-in → swap → bridge-out mirroring `useCrossCurrencySwapStore.step`), "Execute Swap" `<Button>` blocked when no valid quote held, persistent non-dismissible critical `<Alert>` on `BRIDGE_OUT_FAILED` containing `swapId` and reconciliation instruction, auto-advance to Step 5 on `COMPLETED`; **Step 5**: `redeemAmount` read-only from swap output, `swapId` display, "Request Redeem" `<Button>` → `usePaymentStore.requestRedeem()`, `useSwapWizardPolling("redeem", redeemId, showCompletionSummary)`, completion summary card showing all IDs and amounts; per-step error display using `stepError` from wizard store
- [X] T008 [US1] Edit bank app routing file (`frontend/apps/bank/src/routes/index.tsx` or `frontend/apps/bank/src/App.tsx`): add `/swap` route rendering `CommercialSwapWizardPage`; when `VITE_SCENARIO=b` remove `DepositsPage`, `EscrowsPage`, and `AMMTradingPage` from the navigation sidebar entries (keep their routes registered and accessible via direct URL — do not remove the `<Route>` definitions)

**Checkpoint**: Operator can navigate to `/swap` and complete all five wizard steps — deposits, escrows, and AMM nav entries are hidden in Scenario B but their URLs still work.

---

## Phase 3: User Story 2 — AMM Trading Page Fixes (Priority: P2)

**Goal**: Power user accesses the existing AMM Trading page with correct value rendering, a session-locked `payer_bank_id`, the correct default pair, and no Scenario A dead code.

**Independent Test**: Power user opens AMM Trading page directly, requests a quote, and observes: price impact as a percentage string (e.g., `"0.25%"`), timestamp as a human-readable date-time, default pair `BRL-ARS`, `payer_bank_id` as a non-editable field matching their session identity, no `AMMTradingScenarioA` rendering, no console errors.

**Note**: This phase is independent of US1 (Phase 2) and can proceed in parallel with it after Phase 1 completes.

### Implementation

- [X] T009 [US2] Edit `frontend/apps/bank/src/pages/AMMTradingPage.tsx` with the following changes in a single edit pass: **(1)** Remove the `AMMTradingScenarioA` function entirely (~lines 40–146); **(2)** Simplify the `AMMTradingPage` root function to unconditionally return `<AMMTradingV2 />` — remove the `isScenarioB` conditional branch; **(3)** Change `useState("BRL-USD")` to `useState("BRL-ARS")` and update the `CardDescription` label to match; **(4)** Fix `price_impact` display: replace `(quote.price_impact * 100).toFixed(2)` with `(parseFloat(quote.price_impact) * 100).toFixed(2) + "%"`; **(5)** Fix `quote_timestamp` display: replace raw value with `new Date(quote.quote_timestamp * 1000).toLocaleString()`; **(6)** Remove any `amount_out` reference from the swap result display section; **(7)** Replace the `payer_bank_id` `<Input onChange>` with `<Input readOnly value={useAuthStore.getState().profile?.bankId ?? ""} />` — remove the `payerId` local `useState`, add `useAuthStore` import; **(8)** Remove `approveSide` local state and the side selector `<Input>` (or radio buttons) from the approve-AMM panel — call `approveAmm(approveAmount)` with no second argument; **(9)** In `CrossCurrencySwapPanel` (if inline or child component): replace `payerBankId` `useState` with a read from `useAuthStore`

**Checkpoint**: `tsc --noEmit` passes; AMM Trading page renders with no console errors; all nine changes verified.

---

## Phase 4: User Story 3 — Governance Swap Monitor (Priority: P3)

**Goal**: CB operator views pending deposit/escrow/redeem counts and recent cross-currency swap activity from the governance app at `/swap-monitor`, visible only in Scenario B. Page is read-only.

**Independent Test**: CB operator opens governance app under Scenario B, navigates to `/swap-monitor`, sees count cards linking to existing approval pages, and either sees swap records (fetched by ID from session state) or the message "No swap records in this session". No approval actions are available on the page.

**Note**: This phase is independent of US1 and US2 — governance app changes do not depend on bank app changes. T010 is independent of T008/T009 and can run in parallel with Phase 2 or Phase 3.

### Implementation

- [X] T010 [P] [US3] Create `frontend/apps/governance/src/features/swap-monitor/swap-monitor.store.ts`: Zustand store (no `persist` middleware) implementing `SwapMonitorStore` from data-model.md — state: `swapIds: string[]`, `swapRecords: Record<string, SwapMonitorEntry>`, `fetchStatus: Record<string, "loading" | "error" | "done">`; `SwapMonitorEntry` interface with fields: `swap_id`, `payer_bank_id`, `beneficiary_bank_id`, `status` (union of `"PENDING" | "BRIDGE_IN_PROGRESS" | "SWAP_IN_PROGRESS" | "BRIDGE_OUT_PROGRESS" | "COMPLETED" | "BRIDGE_OUT_FAILED"`), optional `bridge_in_position_id`, `swap_tx_ref`, `bridge_out_position_id`, `created_at`, `updated_at`; actions: `fetchSwapRecord(swapId)` — calls `GET /api/v2/amm/swap/cross-currency/{id}`, updates `swapRecords[swapId]` and `fetchStatus[swapId]`; `addSwapId(swapId)` — appends to `swapIds`
- [X] T011 [US3] Create `frontend/apps/governance/src/pages/SwapMonitorPage.tsx`: read-only dashboard with two sections — **PendingCountsSection**: three `<Card>` components showing pending deposits count (link to `/deposits-approval`), pending escrows count (link to `/escrows-approval`), pending redeems count (link to `/redeems-approval`), with counts fetched from existing payment store hooks; **SwapHistorySection**: reads `swapIds` from `useSwapMonitorStore`; if `swapIds.length === 0` render `"No swap records in this session"` text; for each `swapId` call `fetchSwapRecord(swapId)` on mount and render a `SwapRecordCard` showing status `<Badge>`, bridge stage fields, and `created_at`/`updated_at`; if `fetchStatus[swapId] === "error"` show an inline error state for that card only (other cards still displayed); no approve/reject buttons anywhere on the page; page renders only when `VITE_SCENARIO=b` (guard at component or route level)
- [X] T012 [US3] Edit governance app routing file (`frontend/apps/governance/src/routes/index.tsx` or `frontend/apps/governance/src/App.tsx`): add `/swap-monitor` route rendering `SwapMonitorPage` inside the Scenario B navigation/children block; do not add the route or nav link in Scenario A mode

**Checkpoint**: CB operator can navigate to `/swap-monitor` in Scenario B governance app and see the read-only dashboard. Route is absent in Scenario A.

---

## Phase 5: Polish & Verification

- [X] T013 Run `tsc --noEmit` in both `frontend/apps/bank` and `frontend/apps/governance`; fix all type errors introduced by T001–T012
- [X] T014 [P] Run `npm run lint` in both `frontend/apps/bank` and `frontend/apps/governance`; fix all lint warnings and errors surfaced by T001–T012

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (Foundational — T001–T004, all [P])
        ↓
Phase 2 (US1 — T005–T008) ──── can start in parallel with Phase 3
Phase 3 (US2 — T009)       ──── can start in parallel with Phase 2
Phase 4 (US3 — T010–T012)  ──── independent of Phases 2 & 3; can start after Phase 1
        ↓
Phase 5 (Polish — T013–T014)
```

### User Story Dependency Table

| Story | Phase | Depends On | Internal Order |
|-------|-------|-----------|----------------|
| US1 (Wizard) | 2 | Phase 1 complete | T005 ∥ T006 → T007 → T008 |
| US2 (AMM fixes) | 3 | Phase 1 complete | T009 (single task) |
| US3 (Gov monitor) | 4 | None (different app) | T010 → T011 → T012 |

### Parallel Execution Examples

**Phase 1 — all four tasks at once**:
```
T001 (types)     ║
T002 (amm store) ║ → tsc --noEmit → proceed
T003 (API)       ║
T004 (swap store)║
```

**After Phase 1 — three stories in parallel**:
```
US1: T005 ─┐
           ├─ T007 ─ T008   (bank app — wizard)
US1: T006 ─┘

US2: T009                   (bank app — AMM page, independent of US1)

US3: T010 ─ T011 ─ T012     (governance app — independent of bank app)
```

---

## Implementation Strategy

1. **Phase 1 first** — parallelise all four type/store fixes; run `tsc --noEmit` to confirm zero regressions before any UI work.
2. **MVP = Phase 2 (US1)** — wizard store (T005) and polling hook (T006) in parallel, then the wizard page (T007), then routing (T008). At this point Scenario B bank operators have the complete guided flow.
3. **Phase 3 (US2)** — all changes are in one file (`AMMTradingPage.tsx`). Can be done concurrently with Phase 2 by a second developer.
4. **Phase 4 (US3)** — entirely in the governance app. Fully independent; can proceed in parallel with Phases 2 and 3.
5. **Phase 5** — final `tsc --noEmit` and lint pass after all stories are complete.
