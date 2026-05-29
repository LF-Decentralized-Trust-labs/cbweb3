# Implementation Plan: Bank Swap Wizard Refactor

**Branch**: `011-bank-swap-wizard-refactor` | **Date**: 2026-05-28 | **Spec**: [specs/011-bank-swap-wizard-refactor/spec.md](spec.md)  
**Input**: Feature specification from `specs/011-bank-swap-wizard-refactor/spec.md`

## Summary

Replace four disconnected standalone pages (Deposits, Escrows, AMM Trading, Redeems) with a single five-step guided wizard (`CommercialSwapWizardPage`) for Scenario B commercial bank operators. Simultaneously fix type bugs and security gaps on the existing AMM Trading page, and add a read-only swap monitor page to the governance app. No backend changes — purely frontend.

## Technical Context

**Language/Version**: TypeScript 5.4, React 19, Node 20  
**Primary Dependencies**: Zustand 5, React Router v6, Axios, Tailwind CSS, `@cbweb3/ui` (Button, Badge, Card, Input, Label, Tabs, Progress), Lucide React  
**Storage**: Browser session only — Zustand in-memory, no persistence  
**Testing**: `tsc --noEmit` (type check), `npm run lint` (ESLint), Vitest (unit)  
**Target Platform**: Browser (Vite SPA — `frontend/apps/bank`, `frontend/apps/governance`)  
**Project Type**: Web application — monorepo frontend  
**Performance Goals**: Swap execution UI must remain responsive for up to 300s (5 min) during synchronous backend call — FR-007, FR-018  
**Constraints**: Session-only state (no persistence); no new npm packages; no backend changes; wizard restarts from Step 1 on page reload  
**Scale/Scope**: 5 wizard steps; 2 new pages; 4 type field changes; 3 store changes; 2 route files updated

## Constitution Check

*No custom constitution defined for this repository — standard engineering gates apply.*

| Gate | Status | Notes |
|------|--------|-------|
| No new backend changes required | PASS | All APIs already implemented per spec assumptions |
| No new external packages | PASS | All UI primitives already imported in existing pages |
| Type safety maintained | PASS | Type changes narrow/correct existing types (see data-model.md) |
| Security: `payer_bank_id` locked to session | PASS | D-004 — removed from form state entirely |
| No breaking changes to standalone pages | PASS | Routes remain; pages untouched except AMM Trading fixes |
| Wizard state session-only (no `localStorage`) | PASS | Zustand store without `persist` middleware |

## Project Structure

### Documentation (this feature)

```text
specs/011-bank-swap-wizard-refactor/
├── plan.md         ← this file
├── research.md     ← Phase 0: all decisions resolved
├── data-model.md   ← Phase 1: state shapes + type changes
├── quickstart.md   ← Phase 1: how to run and verify
└── tasks.md        ← Phase 2: task breakdown (not yet created)
```

### Source Code (affected files)

```text
frontend/apps/bank/src/
│
│  ── MODIFY ─────────────────────────────────────────────────────────────────
├── types/
│   └── amm-v2.types.ts              # 4 field changes (price_impact, quote_timestamp,
│                                    #   SwapOrder.amount_out removed, ApproveAmmRequest.side optional)
│
├── features/amm/
│   ├── amm-v2.store.ts              # approveAmm: side optional, omit from payload
│   └── cross-currency-swap.store.ts # remove payerBankId state field
│
├── services/api/
│   └── cross-currency-swap.api.ts   # crossCurrencyExecuteClient timeout 60k→300k
│
├── pages/
│   └── AMMTradingPage.tsx           # remove AMMTradingScenarioA fn; AMMTradingPage
│                                    # simplified (always AMMTradingV2); fix pair default
│                                    # BRL-USD→BRL-ARS; payer_bank_id read-only from auth;
│                                    # remove approveSide selector; fix price_impact/
│                                    # quote_timestamp display
│
├── routes/
│   └── index.tsx                    # scenarioBChildren: add /swap → CommercialSwapWizardPage;
│                                    # hide deposits/escrows/amm from Scenario B nav entries
│                                    # (keep routes, remove from nav-visible list)
│
│  ── CREATE ─────────────────────────────────────────────────────────────────
├── features/swap-wizard/
│   ├── swap-wizard.store.ts         # Zustand store for wizard session state (steps 1–5)
│   └── useSwapWizardPolling.ts      # polling hook: deposit/escrow/redeem APPROVED detection
│
└── pages/
    └── CommercialSwapWizardPage.tsx # 5-step wizard (CooperativeLiquidityWizard pattern)

frontend/apps/governance/src/
│
│  ── MODIFY ─────────────────────────────────────────────────────────────────
├── routes/
│   └── index.tsx                    # scenarioBChildren: add /swap-monitor → SwapMonitorPage
│
│  ── CREATE ─────────────────────────────────────────────────────────────────
└── pages/
    └── SwapMonitorPage.tsx          # read-only dashboard: pending counts + swap history
```

## Component Architecture

```
CommercialSwapWizardPage
├── StepIndicator (1–5 grid, same pattern as CooperativeLiquidityWizard)
│
├── Step 1: Token Issuance
│   ├── <Input> fiatAmount (operator enters)
│   ├── <Button> Register Deposit → usePaymentStore.registerDeposit()
│   ├── DepositId display (read-only, shown after registration)
│   └── Polling: useSwapWizardPolling("deposit") → auto-advance on APPROVED
│
├── Step 2: Reserve Tokenisation
│   ├── depositId (pre-filled read-only from store)
│   ├── <Button> Request Escrow → usePaymentStore.requestEscrow(depositId)
│   ├── EscrowId display (read-only)
│   └── Polling: useSwapWizardPolling("escrow") → auto-advance on APPROVED
│
├── Step 3: AMM Spending Approval
│   ├── approveAmount (pre-filled from escrowAmount, editable)
│   ├── tCeBM balance display (from usePaymentStore.balance)
│   ├── Warning badge if approveAmount > balance (non-blocking)
│   ├── <Button> Approve AMM → useAmmV2Store.approveAmm(approveAmount) [no side]
│   └── Manual advance to Step 4
│
├── Step 4: Cross-Currency Swap
│   ├── payer_bank_id (read-only from useAuthStore.profile.bankId)
│   ├── beneficiaryId <Input>
│   ├── pair (BRL-ARS, fixed)
│   ├── amountOut (from Step 1 fiatAmount)
│   ├── Quote panel: useCrossCurrencySwapStore.fetchQuote() + TTL countdown
│   ├── maxAmountIn (auto-computed via calcMaxAmountIn(quote.required_input))
│   ├── Progress stepper: bridge-in → swap → bridge-out (mirrors step field)
│   ├── <Button> Execute → useCrossCurrencySwapStore.executeSwap() [300s timeout]
│   ├── Critical alert (non-dismissible) on BRIDGE_OUT_FAILED with swapId
│   └── Auto-advance to Step 5 on COMPLETED
│
└── Step 5: Redemption
    ├── redeemAmount (pre-filled from swapResult output, read-only)
    ├── swapId display (read-only)
    ├── <Button> Request Redeem → usePaymentStore.requestRedeem()
    ├── Polling: useSwapWizardPolling("redeem") → show completion summary on APPROVED
    └── Completion summary: all IDs, amounts, final status

─────────────────────────────────────────────────────────────────────────────

SwapMonitorPage (governance app)
├── PendingCountsSection
│   ├── DepositsPending count → link to /deposits-approval
│   ├── EscrowsPending count → link to /escrows-approval
│   └── RedeemsPending count → link to /redeems-approval
│
└── SwapHistorySection
    ├── if swapIds.length === 0 → "No swap records in this session"
    └── for each swapId: SwapRecordCard (fetched via GET /amm/swap/cross-currency/{id})
        ├── status badge
        ├── bridge stages
        └── inline error if fetch fails (other cards still shown)
```

## Milestones

### M1 — Type fixes + AMM store fixes (no UI changes)
**Scope**: Pure TypeScript changes, no new files. Safe to ship independently.

- [ ] `amm-v2.types.ts`: `price_impact: string`, `quote_timestamp: number`, remove `SwapOrder.amount_out`, `ApproveAmmRequest.side?`
- [ ] `amm-v2.store.ts`: `approveAmm(amount, side?)` — make side optional; omit from payload when undefined
- [ ] `cross-currency-swap.store.ts`: remove `payerBankId` state field; callers pass `bankId` from auth store
- [ ] `cross-currency-swap.api.ts`: `crossCurrencyExecuteClient` timeout `60_000` → `300_000`
- [ ] **Verify**: `tsc --noEmit` passes; `npm run lint` passes

### M2 — AMM Trading page fixes
**Scope**: Modify `AMMTradingPage.tsx` only. Depends on M1 (type changes must be in place).

- [ ] Remove `AMMTradingScenarioA` function entirely (~lines 40–146)
- [ ] Simplify `AMMTradingPage` root fn: remove `isScenarioB` branch, always render `AMMTradingV2`
- [ ] Fix `pair` default state: `"BRL-USD"` → `"BRL-ARS"`
- [ ] Fix `price_impact` display: `(quote.price_impact * 100).toFixed(2)` → `(parseFloat(quote.price_impact) * 100).toFixed(2)`
- [ ] Fix `quote_timestamp` display: raw value → `new Date(quote.quote_timestamp * 1000).toLocaleString()`
- [ ] Make `payer_bank_id` (mapped to `payerId` state) read-only: replace `<Input onChange>` with `<Input readOnly value={authStore.profile?.bankId ?? ""} />`; remove `payerId` local state; import `useAuthStore`
- [ ] Remove `approveSide` state and side selector `<Input>` from approve-AMM panel; call `approveAmm(approveAmount)` with no side argument
- [ ] **Verify**: `tsc --noEmit` passes; `npm run lint` passes; page renders in browser without console errors

### M3 — Swap wizard store + polling hook (new files)
**Scope**: New files only. No UI changes yet.

- [ ] Create `frontend/apps/bank/src/features/swap-wizard/swap-wizard.store.ts`
  - `SwapWizardSession` state shape per data-model.md
  - All actions: `setStep`, `setDepositId`, `setDepositStatus`, `setEscrowId`, `setEscrowStatus`, `setApproveAmount`, `setSwapId`, `setSwapStatus`, `setRedeemId`, `setRedeemStatus`, `setStepError`, `clearStepError`, `reset`
- [ ] Create `frontend/apps/bank/src/features/swap-wizard/useSwapWizardPolling.ts`
  - Generic hook: `useSwapWizardPolling(type: "deposit" | "escrow" | "redeem", id: string | null, onApproved: () => void)`
  - Uses `usePolling` at 5 000 ms; calls `usePaymentStore.fetchAll()` to refresh; reads status from store; calls `onApproved` when status transitions to `"APPROVED"`; stops polling when `id` is null
- [ ] **Verify**: `tsc --noEmit` passes

### M4 — `CommercialSwapWizardPage` (new page)
**Scope**: New file. Depends on M1 (types), M3 (store + hook).

- [ ] Create `frontend/apps/bank/src/pages/CommercialSwapWizardPage.tsx`
- [ ] Step indicator: 5-step grid (same markup pattern as `CooperativeLiquidityWizard`)
- [ ] Step 1: fiatAmount input, register deposit button, deposit ID read-only display, polling → auto-advance
- [ ] Step 2: depositId read-only, request escrow button, escrowId read-only display, polling → auto-advance
- [ ] Step 3: approveAmount editable input pre-filled from escrowAmount, tCeBM balance display, non-blocking warning if over balance, approve AMM button (no side), manual advance
- [ ] Step 4: payer_bank_id read-only from `useAuthStore`, beneficiaryId input, pair BRL-ARS (fixed), quote panel with TTL countdown and refresh, maxAmountIn auto from `calcMaxAmountIn`, progress stepper (bridge-in/swap/bridge-out from `useCrossCurrencySwapStore.step`), execute button (blocked without valid quote), persistent non-dismissible BRIDGE_OUT_FAILED alert with swapId, auto-advance on COMPLETED
- [ ] Step 5: redeemAmount read-only, swapId display, request redeem button, polling → completion summary
- [ ] Per-step error display (from `stepError` map in wizard store)
- [ ] **Verify**: `tsc --noEmit` passes; `npm run lint` passes; manually walk all 5 steps end-to-end

### M5 — Bank app routing update
**Scope**: Modify `frontend/apps/bank/src/routes/index.tsx`. Depends on M4.

- [ ] Import `CommercialSwapWizardPage`
- [ ] Add `{ path: "swap", element: <CommercialSwapWizardPage /> }` to `scenarioBChildren`
- [ ] Keep deposit/escrow/amm/redeem route entries in `scenarioBChildren` for direct-URL access but remove them from sidebar nav (coordinate with `AppLayout`/`Sidebar` component — conditionally hide nav items when `isScenarioB`)
- [ ] **Verify**: `/swap` loads wizard; `/deposits` still accessible via direct URL in Scenario B; nav does not show Deposits/Escrows/AMM in Scenario B

### M6 — `SwapMonitorPage` + governance routing (new page + route)
**Scope**: New governance page + governance route file update. Independent of M1–M5.

- [ ] Create `frontend/apps/governance/src/pages/SwapMonitorPage.tsx`
  - `useSwapMonitorStore` (local Zustand store with `swapIds: string[]`)
  - Pending counts section: fetch pending deposits/escrows/redeems counts; links to existing approval pages
  - Swap history: map `swapIds` → `GET /amm/swap/cross-currency/{id}`; inline error per record; empty state "No swap records in this session"
  - Read-only (no approve/reject actions)
- [ ] Add `SwapMonitorPage` to `frontend/apps/governance/src/pages/index.ts` (if barrel exists)
- [ ] Update `frontend/apps/governance/src/routes/index.tsx`: add `{ path: "swap-monitor", element: <SwapMonitorPage /> }` to `scenarioBChildren`
- [ ] Update governance sidebar: add "Swap Monitor" link (Scenario B only, same guard pattern as other Scenario B items)
- [ ] **Verify**: `/swap-monitor` loads in Scenario B; not visible in Scenario A nav; empty state shown correctly; pending counts link to approval pages

## Verification Plan

### Command sequence

```bash
cd frontend

# 1. Type check — both apps
npx tsc --noEmit

# 2. Lint — both apps
npm run lint

# 3. Unit tests (if Vitest configured)
npm run test

# 4. Build check
npm run build
```

**"Passing"** means:
- Zero TypeScript errors in `tsc --noEmit`
- Zero ESLint errors in `npm run lint`
- All Vitest tests pass (no regressions)
- Build succeeds without warnings on changed files

### Manual acceptance checks (per user story)

| User Story | Check | Expected |
|------------|-------|----------|
| US-1: Wizard end-to-end | Navigate `/swap`, complete all 5 steps | Completion summary shown; no manual re-entry of IDs |
| US-1: payer_bank_id locked | Inspect Step 4 form | Field is read-only, value matches authenticated session |
| US-1: Bridge-out failure | Simulate BRIDGE_OUT_FAILED response | Non-dismissible alert shows swap ID and reconciliation message |
| US-2: Price impact display | Get quote on AMM page | Shows `X.XX%` (not NaN or raw decimal) |
| US-2: Timestamp display | Get quote on AMM page | Shows human-readable date (not raw integer) |
| US-2: Default pair | Open AMM page | Pair field shows `BRL-ARS` |
| US-2: payer_bank_id read-only | View cross-currency swap form | Non-editable, session value |
| US-2: No side selector | Open approve AMM panel | No side A/B radio/input |
| US-3: Swap monitor loads | Governance `/swap-monitor` | Dashboard visible; "No swap records in this session" in history |
| US-3: Scenario A nav | Governance Scenario A | No "Swap Monitor" in nav |
| FR-022: Scenario B nav | Bank app Scenario B | No Deposits/Escrows/AMM in nav; `/deposits` still accessible via URL |

## Rollback Plan

All navigation changes are gated by `isScenarioB` (derived from `VITE_SCENARIO` env var). To roll back:

1. **Fastest rollback**: Set `VITE_SCENARIO=a` in `.env.local` — no wizard route visible, AMM Trading page reverts to old behaviour (though `AMMTradingScenarioA` function will be removed in M2; restore from git if needed before full M2 deploy).
2. **Full code rollback**: `git revert` the commits for this branch. No database migrations, no backend changes — frontend-only, fully reversible.
3. **Partial rollback by milestone**: Each milestone is independent enough to revert individually (M1 type changes, M2 page fixes, M3–M4 wizard, M5 routing, M6 governance are distinct commits).

The standalone pages (`DepositsPage`, `EscrowsPage`, `AMMTradingPage`, `RedeemsPage`) are not deleted — only hidden from Scenario B nav. Direct URL access persists throughout.

## Decision Log

| Date | Decision |
|------|----------|
| 2026-05-28 | D-001: Wizard state in Zustand store (session-only, no persist middleware) |
| 2026-05-28 | D-002: Reuse existing `usePolling` hook at 5 000 ms for deposit/escrow/redeem approval steps |
| 2026-05-28 | D-003: `crossCurrencyExecuteClient` 300 000 ms timeout; no polling; synchronous hold |
| 2026-05-28 | D-004: Remove `payerBankId` from `CrossCurrencySwapStore`; derive from `useAuthStore` at call site |
| 2026-05-28 | D-005: `ApproveAmmRequest.side` optional; omit from payload; remove UI selector |
| 2026-05-28 | D-006: `SwapMonitorPage` uses local empty Zustand store; shows "No swap records" by default |
| 2026-05-28 | D-007: Bank routes — keep standalone page routes, remove from Scenario B sidebar nav only |

*Full rationale for each decision in [research.md](research.md).*
