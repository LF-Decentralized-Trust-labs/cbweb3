# Tasks: NOC Monitoring Service (002)

**Input**: Design documents from `specs/002-noc-monitoring-service/`
**Branch**: `002-noc-monitoring-service` | **Date**: 2026-05-22
**Prerequisites**: plan.md ✅ | spec.md ✅ | data-model.md ✅ | contracts/ ✅ | research.md ✅

**Tests**: Not requested — test tasks omitted per spec.

**Organization**: Tasks grouped by user story for independent implementation and delivery.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no blocking dependencies)
- **[Story]**: Maps to user story from spec.md (US1–US4)

## Path Conventions

- `backend/services/noc-agent/` — Go module for the lightweight collector
- `backend/services/noc-backend/` — Go module for the central aggregation service
- `interop/hub-and-spoke/noc/` — Docker Compose stack for the NOC services

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Go module scaffolding, Docker artifacts, and agent config templates.

- [ ] T001 Create Go module for noc-agent with required dependencies (github.com/docker/docker/client, gopkg.in/yaml.v3, net/http stdlib) in `backend/services/noc-agent/go.mod`
- [ ] T002 Create Go module for noc-backend with required dependencies (github.com/gofiber/fiber/v2, gorm.io/gorm, gorm.io/driver/postgres, github.com/golang-jwt/jwt/v5) in `backend/services/noc-backend/go.mod`
- [ ] T003 [P] Create multi-stage Dockerfile for noc-agent (build stage: go build; final stage: distroless/scratch, binary < 30 MB) in `backend/services/noc-agent/Dockerfile`
- [ ] T004 [P] Create multi-stage Dockerfile for noc-backend (build stage: go build; final stage: distroless/static) in `backend/services/noc-backend/Dockerfile`
- [ ] T005 [P] Create NOC Docker Compose file (noc-backend on port 8090, noc-db PostgreSQL 15) in `interop/hub-and-spoke/noc/docker-compose.yaml`
- [ ] T006 [P] Create `.env.example` for NOC stack documenting all required environment variables for noc-backend and noc-db in `interop/hub-and-spoke/noc/.env.example`
- [ ] T007 [P] Create sample `agent.yaml` config files for spoke-a, spoke-b, and hub with placeholder values in `interop/hub-and-spoke/noc/agent-configs/spoke-a/agent.yaml`, `spoke-b/agent.yaml`, `hub/agent.yaml`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure shared by all user stories. No user story work can begin until this phase is complete.

**⚠️ CRITICAL**: This phase must be fully complete before Phase 3+.

- [ ] T008 Define all 9 GORM models (`NocSpoke`, `NocAgent`, `NocComponent`, `NocHealthEvent`, `NocAlert`, `NocIncident`, `NocSloMetric`, `NocTransactionEvent`, `NocContainerLog`, `NocLogSnapshot`) with correct field types, constraints, and indexes in `backend/services/noc-backend/internal/domain/models.go`
- [ ] T009 [P] Implement noc-backend config loader (read env vars: `DATABASE_URL`, `NOC_BACKEND_PORT`, `KEYCLOAK_URL`, `KEYCLOAK_REALM`, `AGENT_GRACE_MULTIPLIER`) in `backend/services/noc-backend/internal/config/config.go`
- [ ] T010 [P] Implement noc-agent config loader (parse `agent.yaml`: `spoke_id`, `noc_backend_url`, `api_key`, `push_interval_seconds`, `components[]` with `name`, `type`, `endpoint`, `container_name`) in `backend/services/noc-agent/internal/config/config.go`
- [ ] T011 Adapt Keycloak JWT client from `backend/services/auth/internal/keycloak/client.go` for NOC realm: JWKS fetch with 5-min cache, RS256 validation, `realm_access.roles` extraction in `backend/services/noc-backend/internal/keycloak/client.go`
- [ ] T012 Implement Keycloak JWT middleware for Fiber (extract Bearer token from `Authorization` header, validate via keycloak client, store `sub` + `roles` in Fiber locals, reject with 401 on failure) in `backend/services/noc-backend/internal/middleware/auth.go`
- [ ] T013 [P] Implement agent key auth middleware for Fiber (read `X-Agent-Key` header, compute SHA-256, look up in DB `noc_agents.api_key_hash` or provisioned keys table, reject unknown with 401) in `backend/services/noc-backend/internal/middleware/agent_auth.go`
- [ ] T014 Initialize GORM PostgreSQL connection with `db.AutoMigrate(...)` for all 9 `noc_*` models at service startup in `backend/services/noc-backend/internal/repository/db.go`
- [ ] T015 [P] Implement Spoke registry GORM repository (Create, List, GetByID, SoftDelete via `deregistered_at`) in `backend/services/noc-backend/internal/repository/spokes.go`
- [ ] T016 [P] Implement Agent + Component GORM repository (LookupByKeyHash for auth, UpsertAgent, UpsertComponent by `(agent_id, name)`, UpdateLastSeen, BulkTransitionToUnknown for watchdog) in `backend/services/noc-backend/internal/repository/agents.go`
- [ ] T017 Initialize Fiber app: configure CORS middleware (allow NOC frontend origin), register `/api/v1`, `/internal/v1`, route groups; set up structured error handler in `backend/services/noc-backend/cmd/server/main.go`
- [ ] T018 Implement `POST /api/v1/admin/agents/provision-key` endpoint (validate `raw_key` + `spoke_id` + `hint`, store `sha256(raw_key)` + spoke_id binding, return 201 or 409 on duplicate) in `backend/services/noc-backend/internal/api/admin_keys.go`
- [ ] T019 [P] Implement `POST /api/v1/admin/spokes` (register new spoke: name, currency_code, jurisdiction) and `DELETE /api/v1/admin/spokes/{spoke_id}` (soft-delete via `deregistered_at`) in `backend/services/noc-backend/internal/api/admin_spokes.go`

**Checkpoint**: Foundation ready — all user story phases can now begin.

---

## Phase 3: User Story 1 — Real-Time Network Health Dashboard (Priority: P1) 🎯 MVP

**Goal**: NOC operators see live health for all spokes/components within 30 s of any failure, and can drill down to component logs.

**Independent Test**: Start noc-backend + noc-db + two noc-agents (spoke-a, spoke-b). Kill the Besu node on spoke-a. Within 30 s the `GET /api/v1/overview` response must show spoke-a/besu-node as `OFFLINE` and spoke-b as `HEALTHY`.

### noc-agent: Health Collectors

- [ ] T020 [P] [US1] Implement BESU health checker: call `eth_blockNumber` JSON-RPC via HTTP, compare result to `NocComponent.last_block_number`; emit `DEGRADED` if block unchanged for ≥2 consecutive cycles, `OFFLINE` if connection refused/timeout in `backend/services/noc-agent/internal/collector/besu.go`
- [ ] T021 [P] [US1] Implement CACTI_RELAY health checker: `GET /api/v1/health` with 10 s timeout; `HEALTHY` if HTTP 200 + `{"status":"ok"}` within 5 s; `DEGRADED` if HTTP 200 but response latency > 5 s; `OFFLINE` otherwise in `backend/services/noc-agent/internal/collector/cacti.go`
- [ ] T022 [P] [US1] Implement PALADIN health checker: `POST :8548/` with `ptx_getTransaction dummy` JSON-RPC payload; `HEALTHY` if response body contains `PD020704`; `DEGRADED` if connection ok but response missing `PD020704`; `OFFLINE` if connection refused/timeout in `backend/services/noc-agent/internal/collector/paladin.go`

### noc-agent: Log Collection

- [ ] T023 [US1] Implement Docker socket log tailer (generic): `GET /containers/{name}/logs?stdout=1&stderr=1&tail=200&timestamps=1` via `/var/run/docker.sock`; return `[]LogLine{Stream, Line, Ts}`; resolve container name to ID via `GET /containers/json?filters={"name":["<name>"]}` in `backend/services/noc-agent/internal/logs/docker.go`

### noc-agent: Push Loop

- [ ] T024 [US1] Implement collector orchestrator: on each push cycle, fan-out all health checkers + Docker log tails for all configured components; assemble push payload (`components[]`, `logs[]`, `transaction_events[]`); update `last_block_number` in in-memory state for BESU stall detection in `backend/services/noc-agent/internal/collector/collector.go`
- [ ] T025 [US1] Implement pusher: serialize push payload as JSON, `POST /internal/v1/push` with `X-Agent-Key` header; on non-2xx: exponential backoff retry (max 3 attempts, 2/4/8 s); log errors on permanent failure in `backend/services/noc-agent/internal/pusher/pusher.go`
- [ ] T026 [US1] Implement agent main loop: load `agent.yaml`, initialize collector + pusher, start push ticker at `push_interval_seconds`, handle OS signals for graceful shutdown in `backend/services/noc-agent/cmd/agent/main.go`

### noc-backend: Ingest & Health State

- [ ] T027 [US1] Implement `POST /internal/v1/push` endpoint: validate `X-Agent-Key` (middleware); on first push — verify payload `spoke_id` matches provisioned key binding, auto-create `noc_agents` + `noc_components` records; on subsequent pushes — upsert component `health_status` + `last_block_number` + `last_checked_at`, append `noc_health_events` row per component in `backend/services/noc-backend/internal/api/push.go`
- [ ] T028 [US1] Implement agent watchdog worker: tick every 15 s; find `noc_agents` where `last_seen_at < now() - 3×push_interval_seconds`; bulk-update their components to `UNKNOWN`; update agent `status = UNREACHABLE`; reverse on next push in `backend/services/noc-backend/internal/service/agent_watchdog.go`

### noc-backend: Container Log Buffer

- [ ] T029 [US1] Implement log buffer service: on push ingest, insert received `logs[]` lines into `noc_container_logs`; after insert, delete oldest rows to enforce 1,000-line cap per component (`DELETE ... WHERE id < (SELECT id ... OFFSET 999)`) in `backend/services/noc-backend/internal/service/log_buffer.go`
- [ ] T030 [US1] Extend log buffer service: on component `OFFLINE` or `UNKNOWN` transition in push handler, capture last 500 lines from `noc_container_logs` into `noc_log_snapshots` with `trigger_status` and `snapshot_at` in `backend/services/noc-backend/internal/service/log_buffer.go`

### noc-backend: Health & Log API Endpoints

- [ ] T031 [P] [US1] Implement `GET /api/v1/overview`: query all active `noc_spokes` with their agents + components; compute `overall_status` per spoke as worst child status (`HEALTHY` < `DEGRADED` < `OFFLINE` < `UNKNOWN`); include `active_alert_count` and `agent_status` in `backend/services/noc-backend/internal/api/overview.go`
- [ ] T032 [P] [US1] Implement `GET /api/v1/spokes`, `GET /api/v1/spokes/{spoke_id}`, and `GET /api/v1/spokes/{spoke_id}/components` endpoints in `backend/services/noc-backend/internal/api/spokes.go`
- [ ] T033 [P] [US1] Implement `GET /api/v1/components/{component_id}` (detail + last 20 health events from `noc_health_events`) in `backend/services/noc-backend/internal/api/components.go`
- [ ] T034 [P] [US1] Implement `GET /api/v1/components/{component_id}/logs` (query `noc_container_logs` with `limit`, `since`, `keyword` substring filter, `stream` filter) in `backend/services/noc-backend/internal/api/components.go`
- [ ] T035 [P] [US1] Implement `GET /api/v1/components/{component_id}/log-snapshots` (list snapshots) and `GET /api/v1/components/{component_id}/log-snapshots/{snapshot_id}` (full snapshot content) in `backend/services/noc-backend/internal/api/components.go`

### Wiring

- [ ] T036 [US1] Wire agent watchdog + log buffer service into noc-backend `main.go`: start both as background goroutines after Fiber app init in `backend/services/noc-backend/cmd/server/main.go`
- [ ] T037 [P] [US1] Add `noc-agent-spoke-a` service entry (with `agent.yaml` volume mount + `docker.sock` mount) to spoke-a Docker Compose for local integration testing in `interop/hub-and-spoke/cacti/docker-compose.yaml`

**Checkpoint**: US1 complete. `GET /api/v1/overview` returns live health for all spokes. Container logs visible per component.

---

## Phase 4: User Story 2 — Alert and Incident Centralization (Priority: P2)

**Goal**: Every component failure generates a correctly-classified alert; repeated failures are grouped; recovery auto-resolves.

**Independent Test**: Push a payload with PALADIN `OFFLINE` — verify `GET /api/v1/alerts` returns a `CRITICAL` alert. Push a recovery with PALADIN `HEALTHY` — verify alert transitions to `RESOLVED`.

- [ ] T038 [P] [US2] Implement alerts GORM repository: Insert, ListByFilter (state/severity/spoke), GetByID, UpdateState (acknowledge, resolve), DeduplicateCheck (same `root_cause_sig` within 10-min window) in `backend/services/noc-backend/internal/repository/alerts.go`
- [ ] T039 [P] [US2] Implement incidents GORM repository: UpsertByRootCauseSig (create or reuse open incident), AddAlertToIncident, ResolveIncident, ListByFilter in `backend/services/noc-backend/internal/repository/incidents.go`
- [ ] T040 [US2] Implement alert engine service: given a `(component, previous_status, new_status)` tuple, compute severity using fixed rules (`CRITICAL` = BESU/PALADIN→OFFLINE; `HIGH` = CACTI→OFFLINE; `WARNING` = any→DEGRADED; `INFO` = any→HEALTHY); compute `root_cause_sig = sha256(component_id + alert_type + error_fingerprint)`; dedup within 10-min window; group into incident; auto-resolve on INFO in `backend/services/noc-backend/internal/service/alert_engine.go`
- [ ] T041 [US2] Integrate alert engine into push endpoint: after each component `health_status` upsert, if status changed from previous value, invoke alert engine with `(component, old_status, new_status)` in `backend/services/noc-backend/internal/api/push.go`
- [ ] T042 [US2] Integrate agent-unreachable alert into watchdog: when agent transitions to `UNREACHABLE`, raise `HIGH` severity alert titled "Agent unreachable"; auto-resolve when agent sends next push in `backend/services/noc-backend/internal/service/agent_watchdog.go`
- [ ] T043 [P] [US2] Implement `GET /api/v1/alerts` endpoint (filter by `state`, `severity`, `spoke_id`; paginate with `limit`/`offset`) in `backend/services/noc-backend/internal/api/alerts.go`
- [ ] T044 [P] [US2] Implement `GET /api/v1/incidents` and `GET /api/v1/incidents/{incident_id}` endpoints (incident detail includes linked alerts list) in `backend/services/noc-backend/internal/api/incidents.go`
- [ ] T045 [US2] Implement `POST /api/v1/alerts/{id}/acknowledge` (require `noc-operator` role; set `acknowledged_by = sub claim`; 403 for `noc-viewer`) in `backend/services/noc-backend/internal/api/alerts.go`
- [ ] T046 [US2] Implement `POST /api/v1/alerts/{id}/resolve` (require `noc-operator` role; set `state = RESOLVED`, `resolved_at = now()`; also resolve parent incident if all its alerts are resolved) in `backend/services/noc-backend/internal/api/alerts.go`

**Checkpoint**: US2 complete. Alerts fire within one push cycle of a failure; dedup works; resolution is automatic on recovery.

---

## Phase 5: User Story 3 — SLO Metrics and Performance Tracking (Priority: P3)

**Goal**: Availability % and latency percentiles are visible per spoke component for 24h/7d/30d windows; breaches are flagged.

**Independent Test**: Seed `noc_health_events` with 7 days of synthetic HEALTHY/OFFLINE records representing 99% uptime. Call `GET /api/v1/spokes/{id}/slo?window=7d` — verify `availability_pct = 99.xx` and `slo_breached = false`.

- [ ] T047 [P] [US3] Implement SLO GORM repository (Upsert by `(component_id, window)`, QueryBySpoke) in `backend/services/noc-backend/internal/repository/slo.go`
- [ ] T048 [US3] Implement SLO background worker: every 5 minutes, for each active component × each window (`24h`, `7d`, `30d`): compute `availability_pct = healthy_seconds / window_seconds * 100` from `noc_health_events`; compute `p50_ms/p95_ms/p99_ms` from `noc_transaction_events` using PostgreSQL `percentile_disc(0.5/0.95/0.99)`; detect breach vs configured threshold; upsert `noc_slo_metrics` in `backend/services/noc-backend/internal/service/slo_worker.go`
- [ ] T049 [P] [US3] Implement `GET /api/v1/spokes/{spoke_id}/slo` endpoint (query `noc_slo_metrics` for all components of the spoke; accept `window` query param; return pre-computed values) in `backend/services/noc-backend/internal/api/slo.go`
- [ ] T050 [US3] Wire SLO worker into noc-backend `main.go` startup (start as goroutine after DB init, trigger first compute on startup) in `backend/services/noc-backend/cmd/server/main.go`

**Checkpoint**: US3 complete. SLO dashboard can display real availability data.

---

## Phase 6: User Story 4 — Transaction Log Search and Audit (Priority: P4)

**Goal**: Support engineers can search transaction events by `tx_id`, filter by spoke/time, and export for regulatory reporting.

**Independent Test**: Configure noc-agent to monitor a spoke with a running payment-orchestrator container that emits structured transaction log lines. After a full push cycle, call `GET /api/v1/transactions/{tx_id}/events` — verify all lifecycle events are present in chronological order.

### noc-agent: Transaction Event Extraction

- [ ] T051 [US4] Implement payment-orchestrator log parser: given a slice of Docker log lines, extract structured transaction events matching known log patterns (JSON log lines with fields: `tx_id`, `event_type` in {INITIATED, HTLC_LOCKED, RELAYED, HTLC_SETTLED, HTLC_REFUNDED, FAILED}, `timestamp`, `participant_id`, `contract_id`, `error_code`); return `[]TransactionEvent` in `backend/services/noc-agent/internal/logs/txparser.go`
- [ ] T052 [US4] Extend collector orchestrator: after collecting Docker logs for each component whose `container_name` matches `payment-orchestrator*`, pass log lines through txparser; append extracted events to push payload `transaction_events[]` field in `backend/services/noc-agent/internal/collector/collector.go`

### noc-backend: Ingest & Query

- [ ] T053 [US4] Extend push endpoint handler: persist `transaction_events[]` from push payload into `noc_transaction_events` (ignore duplicates via `ON CONFLICT DO NOTHING` on `(tx_id, event_type, occurred_at, spoke_id)`) in `backend/services/noc-backend/internal/api/push.go`
- [ ] T054 [P] [US4] Implement transactions GORM repository (SearchByTxID, ListByFilter with spoke_id/time range/event_type, ExportByDateRange returning paginated rows) in `backend/services/noc-backend/internal/repository/transactions.go`
- [ ] T055 [P] [US4] Implement `GET /api/v1/transactions` (filter params: `tx_id`, `spoke_id`, `event_type`, `from`, `to`, `limit`, `offset`) and `GET /api/v1/transactions/{tx_id}/events` (all events in ascending `occurred_at` order) in `backend/services/noc-backend/internal/api/transactions.go`
- [ ] T056 [US4] Implement `GET /api/v1/transactions/export` endpoint (require `from` + `to` params; stream response as JSON array or CSV; include all fields required for regulatory reporting) in `backend/services/noc-backend/internal/api/transactions.go`

**Checkpoint**: US4 complete. Full audit trail searchable and exportable.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Operational readiness, retention, Keycloak setup, and integration completeness.

- [ ] T057 Implement retention worker: daily goroutine that deletes rows older than 90 days from `noc_health_events`, `noc_transaction_events`, and `noc_container_logs`; deletes `noc_log_snapshots` older than 90 days; log summary of deleted rows in `backend/services/noc-backend/internal/service/retention_worker.go`
- [ ] T058 [P] Wire retention worker into noc-backend `main.go` startup in `backend/services/noc-backend/cmd/server/main.go`
- [ ] T059 [P] Implement `GET /api/v1/health` liveness probe (no auth; return `{"status":"ok","uptime":<seconds>}`) in `backend/services/noc-backend/internal/api/health.go`
- [ ] T060 [P] Create Keycloak setup scripts: create NOC Keycloak client (`noc-portal`), create roles `noc-viewer`/`noc-operator`/`noc-admin`, create a default NOC admin user in `deploy/local/keycloak/setup-noc-client.sh` and `deploy/local/keycloak/create-noc-user.sh`
- [ ] T061 Add noc-agent service entries (with `docker.sock` mount and `agent.yaml` config volume) to the Docker Compose files of remaining spokes (spoke-b, hub) in their respective `docker-compose.yaml` files under `interop/hub-and-spoke/`
- [ ] T062 [P] Validate `specs/002-noc-monitoring-service/quickstart.md`: run through all setup steps locally and correct any outdated commands, env var names, or missing steps

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (Setup)        → no dependencies, start immediately
Phase 2 (Foundational) → depends on Phase 1 ⚠️ BLOCKS all user stories
Phase 3 (US1)          → depends on Phase 2
Phase 4 (US2)          → depends on Phase 2 + Phase 3 (needs health events as alert triggers)
Phase 5 (US3)          → depends on Phase 2 + Phase 3 (needs noc_health_events data)
Phase 6 (US4)          → depends on Phase 2 + Phase 3 (needs push pipeline for tx events)
Phase 7 (Polish)       → depends on all user story phases
```

### Inter-Story Dependencies

| Story | Depends On | Reason |
|-------|-----------|--------|
| US2 (Alerts) | US1 | Alert engine is triggered from health status transitions in the push handler |
| US3 (SLO) | US1 | SLO worker reads `noc_health_events` written by US1 push handler |
| US4 (Audit) | US1 | Tx events ingested via same push endpoint; requires push pipeline from US1 |
| US2, US3, US4 | US1 | All share the push endpoint (`T027`) — US1 must implement it first |

### Within Each Phase

- Health checkers (T020–T022) can be built in parallel → then integrated by collector (T024)
- API endpoints within a phase marked `[P]` have no cross-file dependencies
- Log buffer (T029–T030) must complete before log API endpoints (T034–T035)
- Alert engine (T040) must complete before push integration (T041) and watchdog integration (T042)

---

## Parallel Execution Examples

### Phase 2 — Foundational (all `[P]` tasks can run simultaneously)

```
T009  noc-agent config loader          ─┐
T011  Keycloak client adaptation        ├─ Parallel (different files)
T012  Agent key auth middleware         │
T013  Fiber app skeleton                │
T015  Spoke repository                  │
T016  Agent+Component repository        │
T019  Admin spokes endpoint            ─┘
T008  GORM models (required first — blocks T014)
T014  DB init + AutoMigrate (depends on T008)
```

### Phase 3 — US1 Health Collectors (noc-agent side)

```
T020  BESU checker    ─┐
T021  CACTI checker    ├─ Parallel → T024 Collector orchestrator
T022  PALADIN checker ─┘
T023  Docker log tailer → used by T024
```

### Phase 3 — US1 API Endpoints (noc-backend side, after T027)

```
T031  GET /overview          ─┐
T032  GET /spokes             ├─ Parallel (different handler functions)
T033  GET /components/{id}    │
T034  GET /components/logs    │
T035  GET /components/snaps  ─┘
```

### Phase 4 — US2

```
T038  Alerts repo   ─┐
T039  Incidents repo ├─ Parallel → T040 Alert engine → T041 Push integration
```

---

## Implementation Strategy

### MVP (User Story 1 only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational ⚠️
3. Complete Phase 3: US1 (T020–T037)
4. **STOP and VALIDATE**: Run two noc-agents against noc-backend, kill a component, confirm `GET /api/v1/overview` shows `OFFLINE` within 30 s
5. Optionally demo to stakeholders

### Incremental Delivery

```
Phase 1 + 2  → Foundation ready
+ Phase 3    → US1: Live health dashboard + container logs (MVP)
+ Phase 4    → US2: Alerts and incidents
+ Phase 5    → US3: SLO metrics
+ Phase 6    → US4: Audit log + regulatory export
+ Phase 7    → Production-ready (retention, Keycloak, docs)
```

### Parallel Team Strategy (if 2+ developers available)

After Phase 2 completes:
- **Developer A**: Phase 3 (US1) — noc-agent + push endpoint + health API
- **Developer B**: Phase 4 (US2) — alert engine + alert/incident API *(can start data models in parallel with Dev A)*

---

## Summary

| Phase | Tasks | Story |
|-------|-------|-------|
| Phase 1: Setup | T001–T007 | — |
| Phase 2: Foundational | T008–T019 | — |
| Phase 3: US1 Health Dashboard | T020–T037 | US1 (P1) |
| Phase 4: US2 Alerts | T038–T046 | US2 (P2) |
| Phase 5: US3 SLO | T047–T050 | US3 (P3) |
| Phase 6: US4 Audit | T051–T056 | US4 (P4) |
| Phase 7: Polish | T057–T062 | — |
| **Total** | **62 tasks** | |

**Parallel opportunities**: 28 tasks marked `[P]`
**MVP scope**: T001–T037 (37 tasks, US1 complete)
