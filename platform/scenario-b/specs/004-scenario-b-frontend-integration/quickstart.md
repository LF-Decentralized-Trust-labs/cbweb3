# Quickstart: Scenario B Frontend Integration

**Feature**: `scenario-b-frontend-integration` | Date: 2026-05-07

---

## Prerequisites

- Node 20 + `pnpm` (or `npm`) installed
- Monorepo dependencies installed: `pnpm install` from `frontend/`
- A running Scenario B backend with `/api/v2` endpoints reachable at `VITE_API_URL`
- (Optional) Only TypeScript validation: no backend required

---

## Running the Bank App (Scenario B)

```bash
cd frontend/apps/bank

# Development server — Scenario B
VITE_SCENARIO=b VITE_API_URL=http://localhost:8080 npm run dev

# Production build — Scenario B
VITE_SCENARIO=b VITE_API_URL=http://localhost:8080 npm run build
```

Expected: Browser opens at `http://localhost:5173`. Sidebar shows **Bridge** and **Automated FX Trading** (not HTLC, Deposits, Escrows, etc.).

---

## Running the Governance App (Scenario B)

```bash
cd frontend/apps/governance

# Development server — Scenario B
VITE_SCENARIO=b VITE_API_URL=http://localhost:8081 npm run dev

# Production build — Scenario B
VITE_SCENARIO=b VITE_API_URL=http://localhost:8081 npm run build
```

Expected: Browser opens at `http://localhost:5174`. Sidebar shows **Liquidity Management**, **Circuit Breaker**, and **Oversight** (not HTLCMonitor, DepositsApproval, etc.).

---

## Verifying Scenario A Is Unaffected

```bash
# No VITE_SCENARIO set = Scenario A
cd frontend/apps/bank && npm run dev
cd frontend/apps/governance && npm run dev
```

Expected: Both apps behave identically to before this feature. No Scenario B routes reachable.

---

## TypeScript Validation (No Backend Required)

```bash
cd frontend/apps/bank && npx tsc --noEmit
cd frontend/apps/governance && npx tsc --noEmit
```

Both commands must exit with code 0.

---

## Key New Routes

### Bank App (Scenario B)

| Path | Page | Description |
|------|------|-------------|
| `/bridge` | `BridgePage` | Lock & Mint, Burn & Unlock, Bridge Positions table |
| `/amm` | `AMMTradingPage` (v2 branch) | Quote, Swap, Approve AMM, circuit-breaker banner |

### Governance App (Scenario B)

| Path | Page | Description |
|------|------|-------------|
| `/liquidity` | `LiquidityManagementPage` | Pool Status, Add/Remove LP, MintAndApprove |
| `/circuit-breaker` | `CircuitBreakerPage` (v2 branch) | Pause, Propose Resume, Sign Resume, state display |
| `/oversight` | `OversightPage` | Open/Sign/Status disclosure requests |

---

## End-to-End Payment Flow (Developer Walkthrough)

### Step 1 — Governance: Fund the Pool

1. Open governance app at `/liquidity`
2. MintAndApprove panel: enter `amount_a` and `amount_b`, submit → `{ status: "ok" }`
3. Add Liquidity form: enter `pool_pair=BRL-USD`, amounts, `provider_bank_id`, submit → LP position appears in session table

### Step 2 — Bank: Bridge Native CBDC onto Hub

1. Open bank app at `/bridge`
2. Lock & Mint form: enter `owner_bank_id`, `spoke_network=spoke-a`, `native_asset=BRL`, `amount`, submit → position appears in table with state `LOCKING`
3. Wait for polling (5s interval) to show state `ACTIVE`

### Step 3 — Bank: AMM Swap

1. Navigate to bank app `/amm`
2. Quote form: enter `pair=BRL-USD`, `amount_out`, submit → review `required_input` and `price_impact`
3. Swap form: set `max_amount_in` (slippage buffer), enter `payer_id`, `beneficiary_id`, submit → swap result shows `order_id`, `tx_hash`, `confirmed_at`

### Step 4 — Bank: Burn & Unlock

1. Return to `/bridge`
2. Burn & Unlock form: enter the `position_id` from Step 2, submit → position state transitions to `BURNED` then polls to `UNLOCKED`

### Step 5 — Governance: Emergency Halt (optional)

1. Open governance app at `/circuit-breaker`
2. Pause form: enter `pair`, `bank_id`, `reason_code`, leave `signature` as `"AA="`, submit → state shows `HALTED`
3. Bank app AMM page: within ≤15s poll cycle, halted banner appears and Swap button is disabled
4. Propose Resume: enter `bank_id`, submit → `request_id` displayed
5. Sign Resume: enter `pair`, `request_id`, `bank_id`, submit → if quorum met, state returns to `LIVE`

---

## Environment Variables Reference

| Variable | App | Purpose |
|----------|-----|---------|
| `VITE_SCENARIO` | bank, governance | Set to `"b"` to activate Scenario B; any other value = Scenario A |
| `VITE_API_URL` | bank, governance | Backend base URL (default: `http://localhost:8080`) |

---

## Common Issues

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Bridge/Liquidity/Oversight routes not found (404) | `VITE_SCENARIO` not set to `"b"` at build time | Rebuild with `VITE_SCENARIO=b` |
| LP Positions table empty after page reload | Expected — table is session-only | Add Liquidity again or accept session-only behaviour |
| Quote stale warning immediately | System clock skew or backend returning old timestamp | Refresh quote |
| Circuit breaker polling shows last known state with stale warning | Network error during poll | Normal degraded-mode behaviour; check backend connectivity |
| TypeScript errors after adding new files | Missing imports or type mismatches | Run `tsc --noEmit` and fix reported errors |
