# Research: NOC Monitoring Service (002)

**Date**: 2026-05-22 | **Branch**: `002-noc-monitoring-service`

---

## R-001: HTTP Framework for NOC Backend

**Decision**: Fiber (`github.com/gofiber/fiber/v2`)

**Rationale**: The `api-gateway` service already uses Fiber v2.52.9. Reusing it ensures consistency in middleware patterns (JWT auth, CORS, error handling) and allows direct code reuse. All other backend services (compliance, auth, payment-orchestrator) are gRPC-only; Fiber is the established HTTP choice in this codebase.

**Alternatives considered**:
- `gin`: Popular but not used anywhere in the codebase — adds inconsistency
- `echo`: Same situation; no existing patterns to reuse
- `net/http` stdlib: Sufficient but verbose for route definition and middleware

**Reference**: `backend/services/api-gateway/go.mod` line 7, `main.go`

---

## R-002: Keycloak JWT Validation in Go

**Decision**: Reuse `backend/services/auth/internal/keycloak/client.go` directly. Library: `github.com/golang-jwt/jwt/v5` (already in go.sum).

**Rationale**: The platform already has a complete, production-ready Keycloak client implementation with:
- JWKS endpoint: `{keycloak}/realms/{realm}/protocol/openid-connect/certs`
- RS256 signature validation
- JWKS cache with 5-minute TTL
- Role extraction from `realm_access.roles` claim
- Token validation with expiry checking

No new library needed. The NOC Backend can import the shared keycloak client pattern or copy-adapt it for the `noc` realm client.

**Role claim path**: `realm_access.roles` → array of strings (e.g., `["noc-viewer", "noc-operator"]`)

**Middleware pattern**: Extract Bearer token from `Authorization` header → validate via keycloak client → store claims in Fiber locals → role guard on protected routes.

**Reference**: `backend/services/auth/internal/keycloak/client.go` (lines 96–320), `backend/services/api-gateway/internal/http/middleware/auth.go`

---

## R-003: Paladin Health Check Strategy

**Decision**: `POST http://{host}:8548/` with JSON-RPC `ptx_getTransaction dummy` payload. A response containing error code `PD020704` ("transaction not found") indicates a healthy, ready Paladin instance.

**Rationale**: Paladin does not expose a dedicated `/health` REST endpoint. The standard readiness check used in Makefile targets (`make/40-paladin.mk`) is a dummy JSON-RPC call that intentionally returns a known "not found" error, proving the JSON-RPC server is accepting requests.

**Healthy response pattern**:
```json
{"jsonrpc":"2.0","id":1,"error":{"code":20704,"message":"..."}}
```
Check: response body contains `"PD020704"` string.

**Unhealthy states**: connection refused, timeout, or response body does not contain `PD020704`.

**Reference**: `make/40-paladin.mk` lines 169–172 (`paladin.wait-spoke-a` target)

---

## R-004: Container Log Collection via Docker Socket

**Decision**: NOC Agent reads container logs via Docker Engine HTTP API over Unix socket (`/var/run/docker.sock`). Use `GET /containers/{id}/logs?stdout=1&stderr=1&tail=200&timestamps=1`.

**Rationale**: Docker SDK for Go (`github.com/docker/docker/client`) wraps the socket API cleanly. Agent needs to be deployed with `docker.sock` volume mount. Container names/IDs are derived from the `agent.yaml` component config — the agent resolves container by name using `GET /containers/json?filters={"name":["<component_name>"]}`.

**Alternatives considered**:
- Parsing `/var/log/containers/*.log` files on host: requires host filesystem access, path is non-portable across container runtimes
- Log shipping agents (Fluentd, Promtail): overkill for v1, adds dependency

**Log buffer**: Backend stores last 1,000 lines per component in `noc_container_logs` table (rolling — oldest deleted when limit exceeded). Log snapshots (`noc_log_snapshots`) capture 500 lines at OFFLINE/UNKNOWN transition for post-mortem analysis, retained 90 days.

**Libraries**:
- `github.com/docker/docker v28+` (Docker SDK for Go)

---

## R-005: SLO Metric Computation Strategy

**Decision**: Compute SLO metrics on-demand from `noc_health_events` table using PostgreSQL window aggregation. Pre-compute and cache for 24h/7d/30d windows every 5 minutes into `noc_slo_metrics`.

**Rationale**: Computing p95/p99 latency over 30 days from raw events on every request would be expensive. A background worker recomputes rolling windows every 5 minutes and writes results to `noc_slo_metrics`. API reads from the pre-computed table — always < 50ms.

**Availability computation**: `(healthy_seconds / total_window_seconds) * 100`. Gaps filled as UNKNOWN (not counted as downtime, flagged separately).

**Latency**: Transaction event timestamps from `noc_transaction_events` (HTLC initiation → settlement). p50/p95/p99 computed via PostgreSQL `percentile_disc`.

---

## R-006: Alert Deduplication / Root-Cause Grouping

**Decision**: Root-cause signature = `sha256(component_id + alert_type + error_fingerprint)`. Alerts with the same signature within a 10-minute window are grouped under the same incident.

**Rationale**: Simple hash-based grouping is sufficient for v1. Prevents alert storms when the same node repeatedly fails health checks. Error fingerprint is derived from the first 64 chars of the diagnostic detail (normalized, lowercased, whitespace-collapsed).

---

## R-007: Project Layout in Monorepo

**Decision**: New services under `backend/services/noc-agent/` and `backend/services/noc-backend/`. Each is a standalone Go module (`go.mod`) following the same structure as `payment-orchestrator`.

**Structure**:
```
backend/services/noc-agent/      ← Go module, static binary
backend/services/noc-backend/    ← Go module, Fiber HTTP service
```

Both services integrated into `interop/hub-and-spoke/cacti/docker-compose.yaml` area? No — they get their own `docker-compose.yaml` under `interop/hub-and-spoke/noc/` to keep NOC concerns separate from the Cacti relay.

**Deployment**: One `noc-backend` container (centralized). One `noc-agent` container per spoke/hub, co-deployed in that spoke's docker-compose.

---

## R-008: Database Migrations

**Decision**: GORM `AutoMigrate` — consistent with `payment-orchestrator` which uses `db.AutoMigrate(...)` at service startup (confirmed: `backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go` line 26). No separate SQL migration files are used in this codebase.

**Implication**: NOC Backend runs `db.AutoMigrate(...)` on all 9 `noc_*` GORM models at startup. Schema diffs are applied automatically. For production rollbacks, GORM AutoMigrate is additive only (safe for adds; column removals require manual DDL).
