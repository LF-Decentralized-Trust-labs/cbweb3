# treasury portal

> [scenario-a](../../../README.md) › [frontend](../../README.md) › treasury

The **treasury portal** is the settlement and liquidity management interface for central bank treasury operators. It provides real-time visibility into cross-border settlement flows, CBDC liquidity positions, and multi-signature approval queues.

---

## Architecture Placement

```
Browser (Treasury Operator)
         │
         ▼
  [treasury portal]
         │          ┌── REST (queries/commands)
         ▼          │
  [api-gateway]  ───┤
                    └── Socket.io (real-time events)
```

Real-time settlement notifications are delivered via **Socket.io**, providing push updates without polling.

---

## Users

Treasury operators at central banks — responsible for settlement coordination, CBDC issuance approvals, and liquidity management.

---

## Key Features

- **Settlement tracking** — Real-time dashboard of cross-border settlement flows, including pending HTLCs and completed swaps.
- **CBDC balance monitoring** — Live view of tCeBM balances and liquidity positions across all entities on the spoke.
- **Cross-border settlement initiation** — Initiate large-value inter-spoke settlement operations from the treasury perspective.
- **Multi-signature approval** — Review and co-sign transactions that require treasury authorization.
- **Operational limits management** — Configure and monitor daily settlement limits and exposure thresholds.
- **Transaction history** — Full audit trail with export capability.

---

## Key Details

| Property | Value |
|----------|-------|
| Framework | React 19 + TypeScript |
| Build tool | Vite 7 |
| Styling | Tailwind CSS v4 + `@cbweb3/ui` |
| State | Zustand + React Query |
| Routing | react-router-dom |
| Real-time | Socket.io client |
| API | Axios → api-gateway REST |

### Dev

```bash
cd frontend
npm run dev:treasury
```

---

## Related

- [frontend workspace](../../README.md) — monorepo setup and npm scripts
- [payment-orchestrator](../../../backend/services/payment-orchestrator/README.md) — handles the settlement operations this portal initiates
- [governance portal](../governance/README.md) — governance and issuance counterpart
