# noc portal (Network Operations Center)

> [scenario-a](../../../README.md) › [frontend](../../README.md) › noc

The **NOC portal** is a real-time operational monitoring dashboard for network engineers and operations staff. It visualises the platform's infrastructure topology — Besu nodes, services, and inter-spoke relay connections — and provides live health metrics, alerts, and message tracing.

---

## Architecture Placement

```
Browser (NOC Operator)
         │
         ▼
  [noc portal]
         │
         ▼
  [noc-backend]   REST :8000
         │
         ▼
  [noc-agent(s)] ──► Docker metrics, Besu RPC
```

---

## Users

Network operations engineers, infrastructure administrators, and on-call engineers.

---

## Key Features

- **Network topology graph** — Interactive visualization of the platform's nodes and connections, built with [Xyflow](https://xyflow.com/) (React Flow). Shows Besu nodes, services, and relay links per spoke.
- **Real-time health panel** — Container status, block heights, and service availability updated live.
- **Alert management** — Active and resolved alerts with severity levels and resolution timestamps.
- **Message tracing** — Track inter-spoke relay messages through their lifecycle (pending → settled / failed).
- **Latency & throughput** — Per-service and per-spoke performance metrics.
- **Log viewer** — Recent log output from monitored containers.

---

## Key Details

| Property | Value |
|----------|-------|
| Framework | React 19 + TypeScript |
| Build tool | Vite 7 |
| Styling | Tailwind CSS v4 + `@cbweb3/ui` |
| State | Zustand + React Query |
| Routing | react-router-dom |
| Graph | Xyflow (React Flow) |
| API | Axios → noc-backend REST |

### Dev

```bash
cd frontend
npm run dev:noc
```

---

## Related

- [frontend workspace](../../README.md) — monorepo setup and npm scripts
- [noc-backend](../../../backend/services/noc-backend/README.md) — the API this portal consumes
- [noc-agent](../../../backend/services/noc-agent/README.md) — collects the metrics displayed here
