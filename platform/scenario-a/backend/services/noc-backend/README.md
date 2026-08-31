# noc-backend

> [scenario-a](../../../README.md) › [backend](../../README.md) › noc-backend

The **noc-backend** is the data hub for the Network Operations Center. It receives telemetry from [noc-agents](../noc-agent/README.md), stores metrics and alerts, and exposes a REST API consumed by the [NOC frontend dashboard](../../../frontend/apps/noc/README.md).

---

## Architecture Placement

```
[noc-agent] ──► HTTP push ──► [noc-backend] :8000
                                     │
                        ┌────────────┼────────────┐
                        ▼            ▼             ▼
                  PostgreSQL     Keycloak     Admin API
                 (metrics/alerts) (auth)   (manage spokes/agents)
                        │
                        ▼
               [NOC frontend] :5xxx
```

---

## Responsibilities

- **Telemetry ingestion** — Receives metrics, block heights, and container states pushed by noc-agents.
- **Alert management** — Evaluates incoming telemetry against thresholds and creates/resolves alerts.
- **Admin API** — CRUD operations for managing registered agents, spoke configurations, and NOC keys.
- **Dashboard API** — Serves aggregated metrics and active alerts to the NOC frontend.
- **Relay metrics** — Tracks inter-spoke relay activity (messages sent, settled, pending).

---

## Key Details

| Property | Value |
|----------|-------|
| Protocol | REST (HTTP/1.1) |
| Port | `8000` |
| Framework | Fiber |
| Language | Go |
| Auth | Keycloak OIDC |

### Roles

| Role | Access |
|------|--------|
| `ROLE_NOC_ADMIN` | Full access: read dashboard, manage agents/spokes/keys |
| `noc-user` | Read-only: dashboard metrics and alerts |

### Key Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/dashboard` | Aggregated spoke health and active alerts |
| `POST` | `/ingest` | Telemetry push from noc-agents |
| `GET` | `/admin/agents` | List registered agents |
| `POST` | `/admin/agents` | Register a new agent |
| `GET` | `/admin/spokes` | List spoke configurations |
| `GET` | `/alerts` | Active and resolved alerts |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `NOC_HTTP_PORT` | HTTP listen port (default: 8000) |
| `POSTGRES_DSN` | PostgreSQL connection string |
| `KEYCLOAK_URL` | Keycloak base URL for token validation |
| `KEYCLOAK_REALM` | Keycloak realm for the NOC |

---

## Internal Structure

```
noc-backend/
├── cmd/main.go            Entrypoint
├── internal/
│   ├── http/              Fiber router and handlers
│   ├── ingest/            Telemetry ingestion pipeline
│   ├── alert/             Alert evaluation and management
│   ├── admin/             Admin API handlers
│   └── repository/        PostgreSQL repositories (metrics, alerts, agents)
└── Dockerfile
```

---

## Related

- [noc-agent](../noc-agent/README.md) — produces the telemetry this service ingests
- [frontend › noc app](../../../frontend/apps/noc/README.md) — the dashboard that consumes this API
