# Quickstart: 011-bank-swap-wizard-refactor

## Prerequisites

- Node 20, pnpm (or npm) installed
- Backend running with Scenario B configuration (`VITE_SCENARIO=b`)
- At least two bank accounts available (one for operator login, one as beneficiary)

## Run the bank app

```bash
cd frontend
npm run dev:bank
# or
pnpm --filter @cbweb3/bank dev
```

Set `VITE_SCENARIO=b` in `frontend/apps/bank/.env.local`.

## Verify the wizard

1. Log in as a commercial bank operator.
2. Navigate to `/swap` — the Swap Wizard loads at Step 1.
3. Enter a fiat amount and submit. The deposit ID is shown and polling begins.
4. Wait for CB approval in the governance app. The wizard auto-advances to Step 2.
5. Step 2 pre-fills `deposit_id` (read-only). Submit to request escrow.
6. Wait for CB approval. The wizard auto-advances to Step 3.
7. Step 3 shows approve amount (pre-filled from escrow, editable) and tCeBM balance. Submit.
8. Step 4 shows `payer_bank_id` as read-only from session. Get a quote, then execute.
9. Progress stages appear (bridge-in → swap → bridge-out). Up to 5 minutes.
10. Step 5 shows redeem amount pre-filled. Confirm redemption. Wait for CB approval.
11. Completion summary is shown.

## Verify the AMM Trading page (power user)

```
/amm
```

- Default pair: `BRL-ARS`
- `payer_bank_id` field: read-only, populated from session
- Quote: price impact as `X.XX%`, timestamp as human-readable date
- Approve AMM panel: no side selector

## Verify the governance swap monitor

```bash
cd frontend
npm run dev:governance
# or
pnpm --filter @cbweb3/governance dev
```

1. Log in as CB operator under Scenario B.
2. Navigate to `/swap-monitor`.
3. Pending counts (deposits, escrows, redeems) shown with links to approval pages.
4. Swap history section shows "No swap records in this session" (no swap IDs tracked).

## Type-check and lint

```bash
cd frontend
npx tsc --noEmit
npm run lint
```
