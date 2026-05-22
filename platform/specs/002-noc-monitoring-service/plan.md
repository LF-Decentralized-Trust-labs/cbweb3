# Implementation Plan: NOC Monitoring Service

**Branch**: `002-noc-monitoring-service` | **Date**: 2026-05-22 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `specs/002-noc-monitoring-service/spec.md`

## Summary

Two new Go microservices implement the NOC monitoring infrastructure:
- **`noc-agent`** (Go 1.25.5): lightweight collector co-deployed per spoke/hub; reads Docker container logs + health-checks Besu/Cacti/Paladin; pushes data to `noc-backend` via HTTP POST authenticated with a per-agent API key.
- **`noc-backend`** (Go 1.25.5, Fiber): central aggregation service; validates Keycloak JWTs for frontend requests; runs alert engine + SLO background worker; stores data in a dedicated PostgreSQL instance (`noc-db`); exposes REST API consumed by the existing NOC frontend at `frontend/apps/noc/`.

Scope: P1 (Health), P2 (Alerts), P3 (SLO), P4 (Audit Log), Container Logs. Support interface (P5) deferred to post-v1.

## Technical Context

**Language/Version**: Go 1.25.5 (both services)  
**Primary Dependencies**:
- `noc-backend`: `github.com/gofiber/fiber/v2` (HTTP), `gorm.io/gorm` + `gorm.io/driver/postgres` (DB), `github.com/golang-jwt/jwt/v5` (Keycloak auth — reuse from `backend/services/auth/`)
- `noc-agent`: `github.com/docker/docker/client` (Docker socket log tailing), `net/http` stdlib (health checks + push), `gopkg.in/yaml.v3` (agent.yaml parsing)

**Storage**: Dedicated PostgreSQL 15 container (`noc-db`). Tables prefixed `noc_`. See `data-model.md`.  
**Testing**: Go standard `testing` package + `github.com/stretchr/testify`  
**Target Platform**: Linux/Docker (Docker Compose deployment)  
**Project Type**: Two microservices + docker-compose integration  
**Performance Goals**: Health push cycle ≤ 15s; API responses < 200ms p95; audit log queries < 5s for 30-day range (SC-003); MTTD < 1 minute (SC-006)  
**Constraints**: Agent binary < 30MB; no TimescaleDB; Keycloak client reused from `auth` service  
**Scale/Scope**: ≥ 5 spokes (SC-007); 90-day retention; 1,000 log lines per component rolling buffer

## Constitution Check

Constitution file contains only placeholder template — no specific gates defined. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/002-noc-monitoring-service/
├── plan.md              ← this file
├── spec.md
├── research.md          ← Phase 0 findings
├── data-model.md        ← DB schema + entities
├── contracts/
│   └── rest-api.md      ← REST API contract
└── tasks.md             ← Phase 2 (speckit.tasks)
```

### Source Code Layout

```text
backend/services/noc-agent/
├── cmd/agent/main.go                 ← entrypoint: load config, start push loop
├── internal/
│   ├── config/config.go              ← agent.yaml loader
│   ├── collector/
│   │   ├── collector.go              ← orchestrates health checks
│   │   ├── besu.go                   ← eth_blockNumber JSON-RPC
│   │   ├── cacti.go                  ← GET /api/v1/health
│   │   └── paladin.go                ← ptx_getTransaction dummy
│   ├── logs/docker.go                ← Docker socket log tailing
│   └── pusher/pusher.go              ← HTTP POST + retry to /internal/v1/push
├── go.mod
└── Dockerfile

backend/services/noc-backend/
├── cmd/server/main.go                ← entrypoint: DB, Fiber, workers
├── internal/
│   ├── config/config.go              ← env vars
│   ├── domain/models.go              ← GORM models
│   ├── middleware/
│   │   ├── auth.go                   ← Keycloak JWT (reuse auth service)
│   │   └── agent_auth.go             ← X-Agent-Key validation
│   ├── api/
│   │   ├── overview.go
│   │   ├── spokes.go
│   │   ├── components.go
│   │   ├── alerts.go
│   │   ├── incidents.go
│   │   ├── slo.go
│   │   ├── transactions.go
│   │   ├── logs.go
│   │   └── push.go                   ← POST /internal/v1/push
│   ├── repository/                   ← GORM-based data access
│   ├── service/
│   │   ├── alert_engine.go           ← dedup + severity
│   │   ├── slo_worker.go             ← background SLO recompute (5 min)
│   │   ├── log_buffer.go             ← rolling trim + snapshot capture
│   │   └── agent_watchdog.go         ← stale agent → UNKNOWN transition
│   └── keycloak/client.go            ← adapted from backend/services/auth/
├── migrations/001_initial_schema.sql
├── go.mod
└── Dockerfile

interop/hub-and-spoke/noc/
├── docker-compose.yaml               ← noc-backend + noc-db
├── .env.example
└── agent-configs/
    ├── spoke-a/agent.yaml
    ├── spoke-b/agent.yaml
    └── hub/agent.yaml

# noc-agent added to each spoke's docker-compose (e.g., cacti/docker-compose.yaml)
```

## Complexity Tracking

| Decision | Rationale |
|----------|-----------|
| Two separate Go modules | Agent and backend have incompatible deps (Docker SDK vs Fiber/GORM); keeps agent binary lean |
| Dedicated `noc-db` | Metric write load must not affect transactional banking DBs |
| Fiber REST API | NOC Frontend consumes REST; Fiber is the established HTTP framework (api-gateway) |
| Rolling log buffer in PostgreSQL | Avoids Elasticsearch/Loki for v1; 1,000 lines per component sufficient for diagnosis |
| Reuse `auth` keycloak client | JWKS+RS256+role extraction already battle-tested in the platform |
