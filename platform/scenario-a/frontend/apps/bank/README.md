# bank portal

> [scenario-a](../../../README.md) › [frontend](../../README.md) › bank

The **bank portal** is the primary web interface for commercial bank operators. It provides the full day-to-day banking workflow: managing balances, initiating intra-spoke payments, participating in cross-spoke HTLC swaps, and engaging in FX trades with counterparty banks.

---

## Architecture Placement

```
Browser (Commercial Bank Operator)
         │
         ▼
  [bank portal]   :5173 / :5174 / :5175 / :5176
         │
         ▼
  [api-gateway]   REST :18080 / :28080 / :48080 / :58080
```

One instance is deployed per commercial bank entity (Bank-A, Bank-B, Bank-C, Bank-D), each pointing to its own api-gateway.

---

## Users

Commercial bank operators — traders, payment officers, and settlement teams.

---

## Key Features

- **Balance dashboard** — Real-time tCeBM balance overview for the bank's wallet.
- **Intra-spoke transfers** — Send tCeBM to other banks on the same spoke (standard ERC-20 or privacy-preserving Zeto transfer).
- **HTLC management** — Initiate and respond to cross-spoke atomic swaps: lock funds, monitor counterparty lock, claim on settlement.
- **FX agreements** — Propose, accept, and settle bilateral FX trades with counterparty banks on other spokes.
- **Transaction history** — Full audit trail of payments, locks, and settlements.

---

## Key Details

| Property | Value |
|----------|-------|
| Framework | React 19 + TypeScript |
| Build tool | Vite 7 |
| Styling | Tailwind CSS v4 + `@cbweb3/ui` |
| State | Zustand + React Query |
| Routing | react-router-dom |
| API | Axios → api-gateway REST |

### Port Assignments

| Entity | URL |
|--------|-----|
| Bank-A | http://localhost:5173 |
| Bank-B | http://localhost:5174 |
| Bank-C | http://localhost:5175 |
| Bank-D | http://localhost:5176 |

### Dev

```bash
cd frontend
npm run dev:bank -- --port 5173
```

---

## Related

- [frontend workspace](../../README.md) — monorepo setup, npm scripts, Docker stacks
- [api-gateway](../../../backend/services/api-gateway/README.md) — backend this portal calls
- [governance portal](../governance/README.md) — central bank counterpart
