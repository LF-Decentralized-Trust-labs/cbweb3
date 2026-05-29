# Quickstart: CooperativeLiquidityWizard

**Feature**: `006-cooperative-liquidity-wizard`
**Date**: 2026-05-15

---

## What this feature does

Replaces the broken "Add Liquidity" form in the governance frontend with a 4-step
wizard that correctly implements the Commit-Reveal cooperative liquidity protocol.

After this feature:
- CB-A and CB-B operators each run through their own wizard to contribute liquidity
- The wizard handles PENDING → ACTIVE transitions automatically (polling)
- If a match is instant, the wizard skips straight to the Success screen

---

## Prerequisites

A running local stack with cooperative liquidity enabled:

```bash
# Start the full scenario-B backend stack
make dev-up
# or the specific spoke setup from the interop scripts
```

Governance frontend must be accessible at `http://localhost:5173` (or configured
`VITE_API_GW_URL`).

---

## Running the wizard (happy path — two operators)

### Operator 1 (CB-A, BRL side)

1. Open governance app → **Liquidity Management**
2. Click **"Add Cooperative Liquidity"**
3. **Step 1 — Mint & Approve**: Enter `amount_a = 200000`, `amount_b = 200000`, click submit
4. **Step 2 — Commit**: `pool_pair = BRL-USD`, `provider_id = central-bank-a`, `side = A`, `amount = 100000`, click submit
5. Response is `status: PENDING` → wizard shows **Step 3 (Monitor)**
6. Countdown shows ~72h remaining

### Operator 2 (CB-B, USD side)

1. Open governance app (second browser/tab) → **Liquidity Management**
2. Repeat Steps 1–4 with `provider_id = central-bank-b`, `side = B`
3. Response is `status: EXECUTED` → wizard skips Step 3, shows **Step 4 (Success)**

### Back to Operator 1

- Polling detects `pool_status: ACTIVE` within 5–10 seconds
- Wizard advances automatically to **Step 4 (Success)**
- LP IDs are shown; pool stats (reserve_a, reserve_b, ratio) are displayed

---

## Resuming a pending commit after page reload

If Operator 1 closes the tab and returns:

1. The 15s page-level polling detects `pool_status: PENDING_COUNTERPART`
2. A banner appears: **"Pending commit detected — Monitor Pending Commit"**
3. Clicking the banner opens the wizard at **Step 3** with the commit details pre-populated
4. When CB-B commits, the wizard auto-advances to Step 4

---

## Cancelling a commit

In Step 3, click **"Cancel Commit"**. The system calls
`DELETE /api/v2/amm/liquidity/commits/:commit_id?provider_id=...` and returns the wizard
to Step 1.

---

## Running tryout script (automated smoke test)

```bash
bash tryouts/tryout-scenario-b-e2e.sh
```

This script exercises the full cooperative liquidity flow via the API and can be used
to verify that the wizard's underlying endpoints work correctly.

---

## Development verification

```bash
# Type-check
pnpm --filter @cbweb3/governance tsc --noEmit

# Lint
pnpm --filter @cbweb3/governance lint

# Unit tests
pnpm --filter @cbweb3/governance test

# Build
pnpm --filter @cbweb3/governance build
```
