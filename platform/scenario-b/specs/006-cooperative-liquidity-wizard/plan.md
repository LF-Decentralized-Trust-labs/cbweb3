# Implementation Plan: Scenario B Full Frontend Integration

**Branch**: `006-cooperative-liquidity-wizard` | **Date**: 2026-05-20 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/006-cooperative-liquidity-wizard/spec.md`
**Plan version**: 2.0 — updated 2026-05-20 to reflect scope expansion (FR-018 breaking changes + bank app flows)

---

## Summary

### Phase A — COMPLETE (T001–T006, original plan M0–M6)

Replace the broken "Add Liquidity" single-form in `LiquidityManagementPage` with a
`CooperativeLiquidityWizard` React component implementing the 4-step Commit-Reveal
cooperative liquidity protocol. Delivered: Mint & Approve → Commit → Monitor → Success,
instant-match routing, pending-commit banner, and TypeScript/lint pass.

### Phase B — IN PROGRESS (T007–T019, 2026-05-20 additions)

The runbook `integracao-scenario-b-2.md` was updated to v4.0 introducing **breaking**
changes to the API payloads. The backend already runs these new contracts and returns
HTTP 400 `DEPRECATED_FIELDS` / `SIDE_REQUIRED` for old payloads. Phase B fixes both
apps:

**Governance app** (FR-016 to FR-020):
- `MintAndApproveRequest` → single `amount` field (drop `amount_a` / `amount_b`);
  new `ApproveAmmRequest {amount, side?}`; update API, store, standalone panel, and
  wizard Step 1; add G5-cross collapsible block for fresh-environment setup.

**Bank app AMM** (FR-021 to FR-022):
- `ApproveAmmRequest` → `{amount, side: "A" | "B"}` (drop `amount_a` / `amount_b`);
  update store action and `AMMTradingPage` approve form; add pool-status gate on swap
  submit (disabled when `pool_status !== "ACTIVE"`).

**Bank app payment flows** (FR-023 to FR-027):
- `RegisterDepositRequest` → `{requester_besu_address, requester_paladin_identity, amount}`;
  `RequestEscrowRequest` → `{deposit_id}`;
  `RequestRedeemRequest` → `{requester_besu_address, requester_paladin_identity, amount, zeto_transfer_tx_hash?}`;
  update API, store, `DepositsPage`, `EscrowsPage`, `RedeemsPage`.

No backend changes. No new npm packages.

---

## Technical Context

**Language/Version**: TypeScript 5.4, React 19, Node 20
**Primary Dependencies**: Zustand 5 (state), Axios (HTTP), React Router v6 (routing),
`@cbweb3/ui` (Card, Button, Input, Label, Badge, Table — shadcn/ui), Lucide React (icons),
Vite (build + env)
**Storage**: Browser session state only — Zustand without persistence (no localStorage,
no IndexedDB)
**Testing**: Vitest + React Testing Library (existing project test runner)
**Target Platform**: Browser (governance SPA + bank SPA, separate deployments)
**Project Type**: web-app (two SPAs)
**Constraints**: No new npm packages; TypeScript strict mode; `@cbweb3/ui` primitives
only; Zustand session-only state (no persist middleware)
**Scale/Scope (Phase B)**: ~10 modified files across two apps, ~0 new files

### Key sources for injected fields (Phase B)

- `requester_besu_address`: `wallet_address` from bank auth store (set during
  onboarding at `POST /onboarding/complete`).
- `requester_paladin_identity`: `import.meta.env.VITE_PALADIN_IDENTITY` (per-instance
  env var, e.g. `"bank-a@node1"`). Not stored in session state.
- `approveAmm.side`: explicit radio/select in `AMMTradingPage` — not inferred.
- `deposit_id` for escrow: first deposit with `status: "APPROVED"` and no escrow yet,
  from `payment.store.ts` deposits list. Field pre-filled and editable.

### G5-cross block (governance wizard Step 1)

Collapsible section gated by toggle "Is this a fresh environment?". When expanded:
- Single shared `amount` field (used by both G5.1 and G5.2).
- `cb_a_signer_address` field (free-text `0x`+40 hex, validated before enabling buttons).
- G5.1 button: calls `mintAndApprove({amount, recipient: cb_a_signer_address})`.
- G5.2 button: calls `approveAmm(amount, "B")`. Independent of G5.1.
- Deactivating toggle discards G5-cross state without blocking the main flow.

### Pool-status gate (bank app)

`AMMTradingPage` already polls pool status via `useAmmV2Store`. The swap submit button
reads `poolStatus?.pool_status` from the same store. Inline message displayed when
`pool_status !== "ACTIVE"`. Quote form remains active regardless.

---

## Constitution Check

Project constitution file is a template (no project-specific principles defined).

**Self-imposed gates**:
- [x] No new npm packages introduced
- [x] No backend changes
- [x] Zustand state is session-only (no persist middleware)
- [x] Existing Remove Liquidity, LP Positions, and Pool Status sections preserved (governance)
- [x] TypeScript strict — no `any`, no `@ts-ignore`
- [x] Breaking field changes (`amount_a`/`amount_b`) fully removed; no backwards-compat shims

---

## Project Structure

### Documentation (this feature)

```text
specs/006-cooperative-liquidity-wizard/
├── plan.md          ← this file
├── research.md      ← Phase 0 output (original)
├── data-model.md    ← Phase 1 output (original)
├── quickstart.md    ← Phase 1 output (original)
└── contracts/
    └── liquidity-wizard.api.md   ← Phase 1 output (original)
```

### Source Code — Phase B changes

```text
frontend/apps/governance/src/
├── types/
│   └── liquidity.types.ts          ← MODIFIED: MintAndApproveRequest → {amount, recipient?};
│                                       add ApproveAmmRequest {amount, side?}
├── services/api/
│   └── liquidity.api.ts            ← MODIFIED: mintAndApprove uses new type;
│                                       add approveAmm(payload)
├── features/liquidity/
│   ├── liquidity.store.ts          ← MODIFIED: mintAndApprove new sig; add approveAmm action
│   ├── LiquidityManagementPage.tsx ← MODIFIED: standalone MintAndApprove panel → single amount
│   └── CooperativeLiquidityWizard.tsx ← MODIFIED: Step 1 single amount + G5-cross block

frontend/apps/bank/src/
├── types/
│   ├── amm-v2.types.ts             ← MODIFIED: ApproveAmmRequest → {amount, side};
│   │                                   PoolStatus + {pool_status?, fee_rate_bps?, total_lp_count?}
│   └── payment.types.ts            ← MODIFIED: add RegisterDepositRequest,
│                                       RequestEscrowRequest, RequestRedeemRequest
├── services/api/
│   └── payment.api.ts              ← MODIFIED: use new request types
├── features/amm/
│   └── amm-v2.store.ts             ← MODIFIED: approveAmm(amount, side) signature
├── stores/
│   └── payment.store.ts            ← MODIFIED: registerDeposit/requestEscrow/requestRedeem sigs
└── pages/
    ├── AMMTradingPage.tsx          ← MODIFIED: approve form (amount+side); swap gate
    ├── DepositsPage.tsx            ← MODIFIED: auto-inject requester fields; missing-wallet warn
    ├── EscrowsPage.tsx             ← MODIFIED: deposit_id payload; pre-fill; post-escrow banner
    └── RedeemsPage.tsx             ← MODIFIED: auto-inject; Zeto toggle + conditional field
```

---

## Component Architecture (Phase B additions)

### Governance — CooperativeLiquidityWizard Step 1 (G5-cross block)

```
Step 1: MintApproveStep
├── Field: amount (single — replaces amount_a / amount_b)
├── Field: recipient (optional — unchanged)
├── [G5-cross Block] — collapsible, activated by toggle
│     ├── Toggle: "Is this a fresh environment?"
│     ├── Field: amount (shared with G5.1 and G5.2)
│     ├── Field: cb_a_signer_address (0x + 40 hex chars, validated)
│     ├── Button: "Mint TOKEN_B to CB-A" (G5.1)
│     │     └── calls mintAndApprove({amount, recipient: cb_a_signer_address})
│     │     └── on success: shows "Done" state
│     └── Button: "Approve TOKEN_B for AMM (side B)" (G5.2)
│           └── calls approveAmm(amount, "B")
│           └── on success: shows "Done" state, advises operator to proceed
└── Submit: calls mintAndApprove({amount, recipient?}) → advance to Step 2
```

### Bank — AMMTradingPage (updated approve section)

```
Approve AMM Section
├── Field: amount (single — replaces approveAmountA / approveAmountB)
├── Selector: side (radio or select — "A" | "B", required)
└── Submit: calls approveAmm(amount, side)

Swap Section
├── [existing quote + swap fields]
└── Submit: disabled when poolStatus?.pool_status !== "ACTIVE"
      └── Inline message: "Pool is not active. Swaps are currently unavailable."
```

### Bank — DepositsPage (updated)

```
Deposit Form
├── Field: amount (unchanged)
├── [hidden] requester_besu_address ← from auth store wallet_address
├── [hidden] requester_paladin_identity ← from import.meta.env.VITE_PALADIN_IDENTITY
├── [conditional] Warning: "Onboarding incomplete — wallet address unavailable"
│     └── shown when wallet_address is null/undefined; disables submit
└── Submit: calls registerDeposit({requester_besu_address, requester_paladin_identity, amount})
```

### Bank — EscrowsPage (updated)

```
Escrow Form
├── Field: deposit_id (pre-filled from first APPROVED deposit w/o escrow; editable)
└── Submit: calls requestEscrow({deposit_id})

Post-escrow Banner (shown when latest escrow status is "APPROVED")
└── "Your tCeBM has been issued. Next step: Approve AMM spending"
      └── Link → AMMTradingPage
```

### Bank — RedeemsPage (updated)

```
Redeem Form
├── Field: amount (unchanged)
├── [hidden] requester_besu_address ← auth store wallet_address
├── [hidden] requester_paladin_identity ← VITE_PALADIN_IDENTITY
├── Toggle: "My tokens came from a Zeto private transfer"
├── [conditional] Field: zeto_transfer_tx_hash (required when toggle on; validate 0x prefix)
│     └── Cleared when toggle turned off
└── Submit: calls requestRedeem({requester_besu_address, requester_paladin_identity,
                                  amount, zeto_transfer_tx_hash?})
```

---

## Data Flow (Phase B additions)

```
Governance wizard Step 1 submit ──→ mintAndApprove({amount, recipient?})
                                       └── POST /amm/token/mint-and-approve {amount, recipient?}

Governance G5.1 button ──────────→ mintAndApprove({amount, recipient: cb_a_signer_address})
                                       └── POST /amm/token/mint-and-approve {amount, recipient}

Governance G5.2 button ──────────→ approveAmm(amount, "B")
                                       └── POST /amm/token/approve-amm {amount, side:"B"}

Bank approve-amm submit ─────────→ approveAmm(amount, side)
                                       └── POST /amm/token/approve-amm {amount, side}

Bank swap submit ────────────────→ disabled when pool_status !== "ACTIVE"

Bank deposit submit ─────────────→ registerDeposit({requester_besu_address, requester_paladin_identity, amount})
                                       └── POST /payment/deposit/register {...}

Bank escrow submit ──────────────→ requestEscrow({deposit_id})
                                       └── POST /payment/exchange/request {deposit_id}

Bank redeem submit ──────────────→ requestRedeem({...amount, requester_*, zeto_transfer_tx_hash?})
                                       └── POST /payment/redeem/request {...}
```

---

## Milestones

### Phase A — COMPLETE (original plan)

- [x] **M0 — Infrastructure** (types + API + store extensions for Commit-Reveal)
  - Extended `liquidity.types.ts` with CommitSide, CommitStatus, PoolLifecycleStatus,
    CommitRequest, CommitResult, PendingCommitSummary; extended PoolStatus
  - Added `commitLiquidity`, `listCommits`, `cancelCommit` to `liquidity.api.ts`
  - Added `activeCommit`, `commitStatus`, `commitError`, `submitCommit`,
    `cancelActiveCommit`, `clearCommit`, `fetchPendingCommits` to `liquidity.store.ts`

- [x] **M1 — Step 1: Mint & Approve** (wizard scaffold)
  - `CooperativeLiquidityWizard.tsx` with `StepIndicator` + Step 1 form
  - Step 1 calls `mintAndApprove`; advances on success

- [x] **M2 — Step 2: Commit**
  - Step 2 form with pool_pair, provider_id, side, amount
  - Routes PENDING → Step 3, EXECUTED → Step 4; inline 409 error

- [x] **M3 — Step 3: Monitor + Cancel**
  - 5s polling, 1s countdown from `expires_at`
  - Auto-advance on ACTIVE; expiry fallback on EMPTY
  - Cancel → `cancelActiveCommit` → Step 1

- [x] **M4 — Step 4: Success**
  - Pool stats; LP IDs with null fallback
  - "Add More Liquidity" → reset; "Done" → exit

- [x] **M5 — Page integration + Pending-Commit Banner**
  - Removed broken "Add Liquidity" form from `LiquidityManagementPage`
  - Embedded `CooperativeLiquidityWizard` + pending-commit banner with provider_id filter

- [x] **M6 — TypeScript + Lint pass** (governance app)

---

### Phase B — NEW WORK (2026-05-20 additions)

- [ ] **M7 — Governance: types + API + store — FR-016 / FR-017 / FR-018**

  **Files**: `liquidity.types.ts`, `liquidity.api.ts`, `liquidity.store.ts`

  Changes:
  - `liquidity.types.ts`: rewrite `MintAndApproveRequest` to `{amount: string; recipient?: string}`;
    add `ApproveAmmRequest: {amount: string; side?: "A" | "B"}`
  - `liquidity.api.ts`: update `mintAndApprove` signature to `MintAndApproveRequest`;
    add `approveAmm(payload: ApproveAmmRequest): Promise<void>` calling
    `POST /api/v2/amm/token/approve-amm`
  - `liquidity.store.ts`: update `mintAndApprove` action to accept `MintAndApproveRequest`
    (new shape); add `approveAmm(amount: string, side?: "A" | "B"): Promise<void>` action

  _Verify_: `pnpm --filter @cbweb3/governance tsc --noEmit` passes with zero errors

- [ ] **M8 — Governance: standalone MintAndApprove panel — FR-019**

  **File**: `LiquidityManagementPage.tsx`

  Changes:
  - Remove state vars `approveAmountA`, `approveAmountB`; add single `approveAmount`
  - Update `handleMintAndApprove` to call `mintAndApprove({amount: approveAmount, recipient: ...})`
  - Replace `<Input id="approve_amount_a">` and `<Input id="approve_amount_b">` with
    single `<Input id="approve_amount">` labelled "Amount"

  _Verify_: no `amount_a` / `amount_b` references remain in `LiquidityManagementPage.tsx`;
  form submits correct payload when mocked

- [ ] **M9 — Governance: wizard Step 1 rewrite + G5-cross block — FR-016 / FR-020**

  **File**: `CooperativeLiquidityWizard.tsx`

  Changes (Step 1 main form):
  - Remove state vars `mintAmountA`, `mintAmountB`; add single `mintAmount`
  - Update `handleMintApprove` to call `mintAndApprove({amount: mintAmount, recipient: ...})`
  - Replace `<Input id="wizard_amount_a">` and `<Input id="wizard_amount_b">` with
    single `<Input id="wizard_amount">` labelled "Amount"

  Changes (G5-cross block — new section within Step 1 JSX):
  - Add boolean state `showG5Cross` (toggle, default false)
  - Add string states `g5Amount`, `g5SignerAddress`; booleans `g51Done`, `g52Done`
  - Render toggle checkbox/switch labelled "Is this a fresh environment?"
  - When `showG5Cross === true`, render collapsible block:
    - `<Input id="g5_amount">` labelled "G5 Amount" (shared between G5.1 and G5.2)
    - `<Input id="g5_signer_address">` labelled "CB-A Signer Address" with inline
      validation against `/^0x[0-9a-fA-F]{40}$/`
    - Button "Mint TOKEN_B to CB-A" (G5.1): disabled when `g5Amount` empty or
      `g5SignerAddress` invalid; calls `mintAndApprove({amount: g5Amount, recipient: g5SignerAddress})`;
      on success sets `g51Done = true`, shows "Done" badge next to button
    - Button "Approve TOKEN_B for AMM (side B)" (G5.2): disabled when `g5Amount` empty;
      calls `approveAmm(g5Amount, "B")`; on success sets `g52Done = true`, shows "Done" badge;
      independent of G5.1 (no G5.1-completion guard)
    - G5-specific error state displayed inline per button (separate from `mintError`)
  - When toggle deactivated: reset `g5Amount`, `g5SignerAddress`, `g51Done`, `g52Done`
  - Main Step 1 submit is unaffected by G5-cross state

  _Verify_: G5-cross block hidden by default; expands on toggle; G5.1/G5.2 each call
  correct store action; deactivating toggle resets state; main Step 1 submit continues
  to work regardless of G5-cross toggle state

- [ ] **M10 — Bank app: AMM types + store + AMMTradingPage — FR-021 / FR-022**

  **Files**: `amm-v2.types.ts`, `amm-v2.store.ts`, `AMMTradingPage.tsx`

  Changes:
  - `amm-v2.types.ts`: rewrite `ApproveAmmRequest` to `{amount: string; side: "A" | "B"}`;
    extend `PoolStatus` interface with `pool_status?: "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE"`,
    `fee_rate_bps?: number`, `total_lp_count?: number`
  - `amm-v2.store.ts`: update `approveAmm` action from `(amount_a: string, amount_b: string)`
    to `(amount: string, side: "A" | "B")`; update body to call
    `ammV2Api.approveAmm({ amount, side })`
  - `AMMTradingPage.tsx` (within `AMMTradingV2`):
    - Remove state `approveAmountA`, `approveAmountB`; add `approveAmount`, `approveSide`
      (`"A" | "B" | ""`, default `""`)
    - Update `handleApprove` to call `approveAmm(approveAmount, approveSide as "A" | "B")`
    - Replace approve form inputs with single `<Input id="approve_amount">` + side
      selector (select or radio, options "A" / "B")
    - Side field required: `approveSide === ""` disables approve submit button
    - Add pool-status gate on swap submit button:
      - Read `poolStatus` from `useAmmV2Store`
      - Disable swap submit when `poolStatus?.pool_status !== "ACTIVE"`
      - Render inline `<p>` "Pool is not active. Swaps are currently unavailable."
        when button is disabled for this reason
      - Quote form (getQuote) remains active regardless of pool_status

  _Verify_: `pnpm --filter @cbweb3/bank tsc --noEmit` passes; approve form has no
  `amount_a`/`amount_b` refs; swap button disabled with inline message when pool
  status is EMPTY or PENDING_COUNTERPART; swap enabled when ACTIVE

- [ ] **M11 — Bank app: payment types + API + store + pages — FR-023 to FR-027**

  **Phase B.1 — Types + API + Store (FR-023 / FR-024)**

  `payment.types.ts`:
  - Add `RegisterDepositRequest: {requester_besu_address: string; requester_paladin_identity: string; amount: string}`
  - Add `RequestEscrowRequest: {deposit_id: string}`
  - Add `RequestRedeemRequest: {requester_besu_address: string; requester_paladin_identity: string; amount: string; zeto_transfer_tx_hash?: string}`

  `payment.api.ts`:
  - Change `registerDeposit` parameter from `CreateAmountRequest` to `RegisterDepositRequest`
  - Change `requestEscrow` parameter from `CreateAmountRequest` to `RequestEscrowRequest`
  - Change `requestRedeem` parameter from `CreateAmountRequest` to `RequestRedeemRequest`

  `payment.store.ts`:
  - Change `registerDeposit(amount: string)` to
    `registerDeposit(requester_besu_address: string, requester_paladin_identity: string, amount: string)`
  - Change `requestEscrow(amount: string)` to `requestEscrow(deposit_id: string)`
  - Change `requestRedeem(amount: string)` to
    `requestRedeem(requester_besu_address: string, requester_paladin_identity: string, amount: string, zeto_transfer_tx_hash?: string)`

  **Phase B.2 — DepositsPage (FR-025)**

  `DepositsPage.tsx`:
  - Import auth store and read `walletAddress = useAuthStore(state => state.profile?.wallet_address ?? null)`
  - Read `paladinIdentity = import.meta.env.VITE_PALADIN_IDENTITY as string`
  - Update all `registerDeposit` call sites to pass
    `registerDeposit(walletAddress!, paladinIdentity, amount)` (guarded by wallet check)
  - Add conditional warning when `walletAddress` is null:
    render inline alert "Onboarding incomplete — wallet address unavailable" and
    disable submit button

  **Phase B.3 — EscrowsPage (FR-026)**

  `EscrowsPage.tsx`:
  - Replace state `[amount, setAmount]` with `[depositId, setDepositId]`
  - Derive default `depositId`: first deposit in store where
    `status === PaymentStatus.APPROVED` and no corresponding escrow entry exists
  - Update form: replace "Amount (tCeBM)" input with "Deposit ID" input pre-filled
    with derived value; field remains editable for manual correction
  - Update all `requestEscrow` call sites to pass `requestEscrow(depositId)`
  - Add post-escrow banner: shown when the most recent entry in `escrows` array has
    `status === PaymentStatus.APPROVED`; text "Your tCeBM has been issued. Next step:
    Approve AMM spending" with navigation link to AMM trading route

  **Phase B.4 — RedeemsPage (FR-027)**

  `RedeemsPage.tsx`:
  - Import auth store; read `walletAddress` and `paladinIdentity` same as DepositsPage
  - Add state `zetoToggle: boolean` (default false) and `zetoHash: string` (default `""`)
  - When `zetoToggle === false`: call
    `requestRedeem(walletAddress!, paladinIdentity, amount)` — no hash
  - When `zetoToggle === true`:
    - Render `<Input id="zeto_transfer_tx_hash">` (required; validate `zetoHash.startsWith("0x")`)
    - Submit disabled until field is non-empty and starts with `0x`
    - Call `requestRedeem(walletAddress!, paladinIdentity, amount, zetoHash)`
  - When `zetoToggle` toggled off: set `zetoHash` to `""`
  - Add missing-wallet guard same as DepositsPage

  _Verify_: all four files typecheck; `CreateAmountRequest` has zero callers remaining
  (remove from `payment.types.ts`); EscrowsPage form has no `amount` input; Zeto
  field absent from DOM when toggle is off; payload never includes `zeto_transfer_tx_hash`
  with value `""` or `undefined`

- [ ] **M12 — TypeScript + Lint pass (both apps)**

  ```bash
  pnpm --filter @cbweb3/governance tsc --noEmit
  pnpm --filter @cbweb3/governance lint
  pnpm --filter @cbweb3/bank tsc --noEmit
  pnpm --filter @cbweb3/bank lint
  ```

  Fix any errors surfaced. Zero errors / zero warnings is the exit condition.

---

## Verification Plan

```bash
# From repo root
pnpm --filter @cbweb3/governance tsc --noEmit   # strict type check (governance)
pnpm --filter @cbweb3/governance lint            # ESLint (governance)
pnpm --filter @cbweb3/bank tsc --noEmit          # strict type check (bank)
pnpm --filter @cbweb3/bank lint                  # ESLint (bank)
pnpm --filter @cbweb3/governance build           # Vite production build (governance)
pnpm --filter @cbweb3/bank build                 # Vite production build (bank)
```

**Passing = zero TypeScript errors, zero lint errors, both Vite builds succeed.**

Manual smoke test (tryout environment):
```bash
bash tryouts/tryout-scenario-b-e2e.sh
```

### Per-milestone spot checks

| Milestone | Command | Expected |
|-----------|---------|----------|
| M7 | `grep -r "amount_a\|amount_b" frontend/apps/governance/src/types frontend/apps/governance/src/services frontend/apps/governance/src/features/liquidity/liquidity.store.ts` | 0 hits |
| M8 | `grep -n "approve_amount_a\|approve_amount_b\|approveAmountA\|approveAmountB" frontend/apps/governance/src/features/liquidity/LiquidityManagementPage.tsx` | 0 hits |
| M9 | `grep -n "wizard_amount_a\|wizard_amount_b\|mintAmountA\|mintAmountB" frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx` | 0 hits |
| M10 | `grep -r "amount_a\|amount_b" frontend/apps/bank/src/types/amm-v2.types.ts frontend/apps/bank/src/features/amm frontend/apps/bank/src/pages/AMMTradingPage.tsx` | 0 hits |
| M11 | `grep -r "CreateAmountRequest" frontend/apps/bank/src/` | 0 hits |
| M12 | Both `tsc --noEmit` exits with code 0 | — |

---

## Test Strategy

### Unit tests (Vitest + React Testing Library) — Phase B additions

| File | What to test |
|---|---|
| `CooperativeLiquidityWizard.tsx` (Step 1) | Single `amount` field submits correct payload; G5-cross block hidden by default; expands on toggle; G5.1 calls `mintAndApprove` with `recipient`; G5.2 calls `approveAmm` with `side:"B"`; G5.1 and G5.2 are independent; toggle off resets G5 state; invalid `cb_a_signer_address` disables G5 buttons; main Step 1 submit unaffected by G5 state |
| `liquidity.store.ts` | `approveAmm` dispatches `POST /amm/token/approve-amm` with correct payload; `mintAndApprove` with new shape omits `amount_a`/`amount_b` |
| `AMMTradingPage.tsx` | Approve form submits `{amount, side}`; side not selected disables approve submit; swap button disabled when `pool_status === "EMPTY"`; swap button disabled when `pool_status === "PENDING_COUNTERPART"`; inline message shown when disabled; swap button enabled when `pool_status === "ACTIVE"` |
| `DepositsPage.tsx` | Submit injects `wallet_address` from auth store as `requester_besu_address`; submit injects `VITE_PALADIN_IDENTITY` as `requester_paladin_identity`; submit button disabled + warning shown when `wallet_address` is null |
| `EscrowsPage.tsx` | Form renders `deposit_id` field (not amount); pre-fills from first APPROVED deposit; `requestEscrow` called with `{deposit_id}`; post-escrow banner shown when latest escrow is APPROVED; banner hidden when not |
| `RedeemsPage.tsx` | Toggle off: payload omits `zeto_transfer_tx_hash`; toggle on: submit disabled until hash non-empty with `0x` prefix; toggle on → off: hash cleared to empty string; toggle on + filled: submit includes hash; missing wallet disables submit + shows warning |
| `payment.store.ts` | `requestEscrow` passes `deposit_id`; `registerDeposit` passes all three requester + amount fields; `requestRedeem` passes all fields; `requestRedeem` omits hash when `undefined` |

### No E2E tests (Playwright/Cypress) — explicitly out of scope per spec.

---

## Rollback Plan

Phase B changes are confined to 10 frontend files across two apps:

1. **Git revert**: `git revert HEAD~<n>` (one commit per milestone recommended) restores
   any modified file to its prior state.
2. **Zero blast radius**: no backend changes, no database migrations, no contract changes.
3. **Co-located signature changes**: store action signature changes and their callers
   (pages) are updated in the same milestone — reverting a milestone's files restores
   full consistency.

---

## Decision Log

- **2026-05-15**: Use single `CooperativeLiquidityWizard` component with internal step
  state (not separate route-based pages). Rationale: simpler navigation, no router
  changes needed, consistent with page-embedded panel pattern used by MintAndApprove.

- **2026-05-15**: `provider_id` sourced from `UserProfile.bankId` (auth store).
  Rationale: confirmed in spec clarification Q7 — wizard reads from auth store, field
  remains editable.

- **2026-05-15**: `listCommits` implemented but not wired to any wizard component.
  Rationale: spec FR-009 and clarification Q5 explicitly state it is a utility method;
  banner uses `pending_commits` from `fetchPoolStatus` response.

- **2026-05-20**: `approveAmm.side` is a required explicit selector in bank app
  `AMMTradingPage`. Rationale: bank accounts don't have `CENTRAL_BANK_ROLE`; backend
  returns HTTP 400 `SIDE_REQUIRED` if omitted. UI must not allow submit without side.

- **2026-05-20**: `zeto_transfer_tx_hash` toggled by user — not auto-detected. Rationale:
  the frontend has no way to determine whether tokens were received via a Zeto private
  transfer; the operator has the context and judgment. Toggle is informational gating.

- **2026-05-20**: `deposit_id` for escrow pre-filled from first APPROVED deposit without
  a corresponding escrow. Rationale: spec clarification Q2 (Session 2026-05-20). Field
  remains editable for manual correction.

- **2026-05-20**: G5.1 and G5.2 buttons are independent (no G5.1-completion gate on
  G5.2). Rationale: spec clarification Q3 (Session 2026-05-20) — "Os dois botões são
  independentes — G5.2 não exige que G5.1 tenha sido executado na sessão atual."

- **2026-05-20**: Pool-status gate disables only the swap submit button, not the quote
  form. Rationale: spec clarification Q4 (Session 2026-05-20) — operators should be
  able to see quotes even when pool is not ACTIVE.

---

## Surprises & Discoveries

- **2026-05-15**: `UserProfile.bankId` is optional (`bankId?: string`). The wizard
  must handle `bankId` being undefined — fall back to empty string `""`, leaving
  `provider_id` field blank for the operator to fill in manually.

- **2026-05-15**: Existing `addLiquidity` store action and `liquidityApi.addLiquidity`
  are kept in place (per spec Assumptions: "legacy endpoint maintained for compatibility")
  — the "Add Liquidity" _form_ is removed from the page, but the _action_ and _API
  method_ remain for backward compatibility.

- **2026-05-15**: `LiquidityManagementPage` binds the same `status` from the store for
  both Add and Remove Liquidity buttons. After Phase A, the store has two independent
  status fields: `status` (for remove/mint/fetch) and `commitStatus` (for commit
  operations).

- **2026-05-20**: `payment.types.ts` already defines `DepositRecord`, `EscrowRecord`,
  `RedeemRecord` with `requester_besu_address` and `requester_paladin_identity` as
  read-model (response) fields. The _request_ types (`RegisterDepositRequest`,
  `RequestEscrowRequest`, `RequestRedeemRequest`) are absent and must be added. The
  API and store currently use `CreateAmountRequest {amount}` for all three write ops.

- **2026-05-20**: `CooperativeLiquidityWizard.tsx` Step 1 currently passes
  `{amount_a: mintAmountA, amount_b: mintAmountB, recipient: ...}` — the deprecated
  two-field shape. This must be fixed in M9 alongside the G5-cross addition.

- **2026-05-20**: `LiquidityManagementPage.tsx` standalone MintAndApprove panel also
  passes `{amount_a, amount_b}` (state vars `approveAmountA`, `approveAmountB`). Must
  be fixed in M8 independently of the wizard.

---

## Progress

- [x] M0 — Commit-Reveal types + API + store extensions (governance)
- [x] M1 — CooperativeLiquidityWizard scaffold + Step 1 (original — amount_a/amount_b shape)
- [x] M2 — Step 2: Commit
- [x] M3 — Step 3: Monitor + Cancel
- [x] M4 — Step 4: Success
- [x] M5 — Page integration + PendingCommitBanner
- [x] M6 — TypeScript + lint pass (governance — Phase A)
- [ ] M7 — Governance: MintAndApproveRequest → {amount}; add ApproveAmmRequest + approveAmm action
- [ ] M8 — Governance: LiquidityManagementPage standalone panel → single amount field
- [ ] M9 — Governance: Wizard Step 1 single amount + G5-cross block
- [ ] M10 — Bank: ApproveAmmRequest → {amount, side}; PoolStatus extension; AMMTradingPage
- [ ] M11 — Bank: payment types + API + store + DepositsPage + EscrowsPage + RedeemsPage
- [ ] M12 — TypeScript + lint pass (both apps — Phase B)
