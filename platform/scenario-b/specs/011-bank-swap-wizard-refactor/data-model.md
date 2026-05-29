# Data Model: Bank Swap Wizard Refactor

**Feature**: `011-bank-swap-wizard-refactor`  
**Date**: 2026-05-28  
**Storage**: Browser session only — Zustand in-memory, no persistence

---

## Frontend State Shape

### `SwapWizardSession` (new — `useSwapWizardStore`)

Tracks the operator's in-progress guided swap flow. Session-scoped; lost on page reload by design.

```typescript
type WizardStep = 1 | 2 | 3 | 4 | 5;

interface SwapWizardSession {
  // Navigation
  currentStep: WizardStep;

  // Step 1 — Token Issuance (Deposit)
  depositFiatAmount: string;       // operator input: fiat amount
  depositId: string | null;        // set after registerDeposit() succeeds
  depositStatus: string | null;    // polled: null → PENDING → APPROVED

  // Step 2 — Reserve Tokenisation (Escrow)
  escrowId: string | null;         // set after requestEscrow() succeeds
  escrowAmount: string | null;     // from approved escrow record
  escrowStatus: string | null;     // polled: null → PENDING → APPROVED

  // Step 3 — AMM Spending Approval
  approveAmount: string;           // pre-filled from escrowAmount, editable

  // Step 4 — Cross-Currency Swap
  amountOut: string;               // from Step 1 fiat amount
  swapId: string | null;           // set after executeSwap() returns
  swapStatus: CrossCurrencySwapStep; // mirrors cross-currency-swap.store step

  // Step 5 — Redemption
  redeemAmount: string | null;     // pre-filled from swap output
  redeemId: string | null;         // set after requestRedeem() succeeds
  redeemStatus: string | null;     // polled: null → PENDING → APPROVED

  // Global
  stepError: Record<WizardStep, string | null>;  // per-step error messages
  isPolling: boolean;
}
```

**Actions** (store methods):

| Action | Description |
|--------|-------------|
| `setStep(step)` | Navigate to a specific step (used for auto-advance) |
| `setDepositId(id, status?)` | Record deposit ID after registration |
| `setDepositStatus(status)` | Update from polling response |
| `setEscrowId(id, amount?)` | Record escrow ID + amount |
| `setEscrowStatus(status)` | Update from polling response |
| `setApproveAmount(amount)` | Sync approve amount from Step 2 or operator edit |
| `setSwapId(id)` | Record swap ID from Step 4 execution result |
| `setSwapStatus(status)` | Mirror from `useCrossCurrencySwapStore` |
| `setRedeemId(id)` | Record redeem ID after redemption request |
| `setRedeemStatus(status)` | Update from polling response |
| `setStepError(step, msg)` | Set per-step error message |
| `clearStepError(step)` | Clear per-step error |
| `reset()` | Reset entire session to initial state |

---

### `SwapMonitorStore` (new — governance app)

Tracks swap IDs for the current browser session in the governance app.

```typescript
interface SwapMonitorStore {
  swapIds: string[];                           // manually added or empty
  swapRecords: Record<string, SwapMonitorEntry>; // keyed by swap_id
  fetchStatus: Record<string, "loading" | "error" | "done">;
  fetchSwapRecord: (swapId: string) => Promise<void>;
  addSwapId: (swapId: string) => void;
}

interface SwapMonitorEntry {
  swap_id: string;
  payer_bank_id: string;
  beneficiary_bank_id: string;
  status: "PENDING" | "BRIDGE_IN_PROGRESS" | "SWAP_IN_PROGRESS" |
          "BRIDGE_OUT_PROGRESS" | "COMPLETED" | "BRIDGE_OUT_FAILED";
  bridge_in_position_id?: string;
  swap_tx_ref?: string;
  bridge_out_position_id?: string;
  created_at?: string;
  updated_at?: string;
}
```

---

## Type Changes (existing files)

### `frontend/apps/bank/src/types/amm-v2.types.ts`

| Field | Before | After | Reason |
|-------|--------|-------|--------|
| `AMMQuote.price_impact` | `number` | `string` | Backend returns decimal string; arithmetic on string was causing NaN display |
| `AMMQuote.quote_timestamp` | `string` | `number` | Backend returns Unix epoch integer; was being displayed raw |
| `SwapOrder.amount_out` | `string` (present) | removed | Field does not exist in backend response; was dead code |
| `ApproveAmmRequest.side` | `"A" \| "B"` (required) | `side?: "A" \| "B"` | Side is resolved server-side; omit from payload |

**Display fix for `price_impact`** (in `AMMTradingV2` and Step 4 of wizard):
```typescript
// Before (broken if price_impact is string):
(quote.price_impact * 100).toFixed(2)   // NaN when price_impact = "0.0025"

// After:
(parseFloat(quote.price_impact) * 100).toFixed(2)
```

**Display fix for `quote_timestamp`** (number → human-readable):
```typescript
// Before:
quote.quote_timestamp   // raw number like 1716883200

// After:
new Date(quote.quote_timestamp * 1000).toLocaleString()
```

---

## Data Flow Between Wizard Steps

```
Step 1 (Deposit)
  operator inputs: fiatAmount
  store.depositId ← registerDeposit(fiatAmount)
  polling: depositStatus → APPROVED
  auto-advance to Step 2
        ↓
Step 2 (Escrow)
  pre-filled (read-only): depositId
  store.escrowId, store.escrowAmount ← requestEscrow(depositId)
  polling: escrowStatus → APPROVED
  auto-advance to Step 3
        ↓
Step 3 (Approve AMM)
  pre-filled (editable): approveAmount ← escrowAmount
  displayed: tCeBM balance (from usePaymentStore.balance)
  warning if approveAmount > balance (non-blocking)
  on submit: approveAmm(approveAmount) — no side
  manual advance to Step 4
        ↓
Step 4 (Cross-Currency Swap)
  pre-filled (read-only): payer_bank_id ← authStore.profile.bankId
  operator inputs: beneficiaryId, maxAmountIn (or accepts quote default)
  pair: BRL-ARS (fixed default)
  quote fetch → execute → progress stepper (bridge-in → swap → bridge-out)
  store.swapId ← swapResult.swap_id
  on completion: auto-advance to Step 5
  on BRIDGE_OUT_FAILED: persistent non-dismissible critical alert with swapId
        ↓
Step 5 (Redeem)
  pre-filled (read-only): swapId, redeemAmount ← swapResult output
  on confirm: requestRedeem(escrowId, redeemAmount)
  polling: redeemStatus → APPROVED
  on APPROVED: completion summary
```

---

## Entities Not Stored (confirmed no backend changes)

All listed entities (`SwapWizardSession`, `SwapOperation`, `AMMQuote`) are frontend-only or already persisted by the existing backend. No new database tables, API endpoints, or contract changes are required.
