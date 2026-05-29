# Quickstart: CB Liquidity Frontend Refactor (009)

How to run, verify, and manually test the governance wizard after the 009 refactor.

---

## Prerequisites

- Running Scenario B backend (008-fix-cb-liquidity or later)
- `VITE_SCENARIO=b` environment variable set for the governance app
- A CB operator JWT (bank-central-a or bank-central-b)
- Docker Compose stack: `backend/docker-compose-backend.central-bank-a.yaml`

---

## Run the governance app

```bash
cd frontend
npm run dev --workspace=governance
```

Open `http://localhost:5174` (or the port shown in terminal).

---

## Static verification

```bash
cd frontend

# TypeScript type check (zero errors expected)
npm run type-check --workspace=governance

# Lint
npm run lint --workspace=governance
```

---

## Dead code verification (SC-005)

```bash
cd frontend/apps/governance/src
# Should return zero results in each search:
grep -r "mintAndApprove"     types/ services/api/ features/liquidity/
grep -r "addLiquidity"       types/ services/api/ features/liquidity/
grep -r "AddLiquidityRequest"   types/ services/api/ features/liquidity/
grep -r "MintAndApproveRequest" types/ services/api/ features/liquidity/
```

---

## Wizard happy-path flow (manual E2E)

1. Navigate to **Governance → Liquidity** (requires `VITE_SCENARIO=b`)
2. **Step 1 — Bridge Lock-Mint**: Enter an amount (e.g. `100000000000000000000000`), click Submit.  
   - Network tab: `POST /api/v2/bridge/lock-mint` payload = `{ "amount": "..." }` only (no `recipient`, `token`).
   - Wizard advances to Step 2 once bridge position reaches `ACTIVE`.
3. **Step 2 — Commit**: Pool pair defaults to `W-BRL-ARS`, enter amount, click Submit.  
   - Network tab: `POST /api/v2/amm/liquidity/commit` payload = `{ "pool_pair": "W-BRL-ARS", "amount": "..." }` only (no `provider_id`, `side`).
   - Step 2 success panel shows `commit_id` and `on_chain_commit_id` (or "pending on-chain confirmation").
4. **Step 3 — Monitor**: Observe polling every 3 s. After 30 s with status `PENDING`, a warning banner appears. At 180 s, a timeout message appears.
5. **Step 4 — Positions**: Loads `GET /api/v2/amm/liquidity/positions?pool_pair=W-BRL-ARS&provider_id=<bankId>`.  
   - Displays `token_a_contributed`, `token_b_contributed`, `deposit_side`, `commit_id` for each position.
   - If empty array: displays "No positions found" message.

---

## Cancel flow (manual)

1. With a commit in Step 3 (PENDING), click **Cancel Commit**.
2. Network tab: `DELETE /api/v2/amm/liquidity/commits/<id>?provider_id=<bankId>`.
3. UI updates to show `CANCELLED` status badge (not "unknown status").
4. Wizard returns to Step 1.

---

## Tryout scripts

```bash
# Sovereign CB liquidity end-to-end
bash tryouts/tryout-sovereign-cb-liquidity.sh

# Full Scenario B E2E
bash tryouts/tryout-scenario-b-e2e.sh
```
