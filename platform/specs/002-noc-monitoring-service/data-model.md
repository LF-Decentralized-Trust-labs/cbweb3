# Data Model: NOC Monitoring Service (002)

**Date**: 2026-05-22 | **Branch**: `002-noc-monitoring-service`

---

## Entities & Tables

All tables use the `noc_` prefix to avoid collisions with existing platform schemas in the dedicated `noc-db` PostgreSQL instance.

---

### `noc_spokes`

Registered spoke/hub networks that agents report on.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | UUID | PK | Generated |
| `name` | TEXT | NOT NULL, UNIQUE | e.g., `spoke-a` |
| `currency_code` | CHAR(3) | NOT NULL | e.g., `BRL`, `ARS` |
| `jurisdiction` | TEXT | NOT NULL | e.g., `Brazil` |
| `active` | BOOLEAN | NOT NULL, DEFAULT true | Soft-decommission |
| `registered_at` | TIMESTAMPTZ | NOT NULL, DEFAULT now() | |
| `deregistered_at` | TIMESTAMPTZ | NULLABLE | Set on decommission |

---

### `noc_agents`

One NOC Agent process per spoke/hub.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | UUID | PK | |
| `spoke_id` | UUID | FK → noc_spokes.id | |
| `name` | TEXT | NOT NULL | e.g., `agent-spoke-a` |
| `api_key_hash` | TEXT | NOT NULL, UNIQUE | SHA-256 of raw key (never stored in plaintext) |
| `push_interval_seconds` | INT | NOT NULL, DEFAULT 15 | |
| `status` | TEXT | NOT NULL | `REACHABLE` \| `UNREACHABLE` |
| `last_seen_at` | TIMESTAMPTZ | NULLABLE | Updated on every push |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT now() | |

**Index**: `api_key_hash` (for fast auth lookup on every push)

---

### `noc_components`

Monitorable infrastructure elements within a spoke.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | UUID | PK | |
| `agent_id` | UUID | FK → noc_agents.id | |
| `spoke_id` | UUID | FK → noc_spokes.id | Denormalized for query performance |
| `name` | TEXT | NOT NULL | e.g., `besu-node`, `cacti-relay` |
| `type` | TEXT | NOT NULL | `BESU` \| `CACTI_RELAY` \| `PALADIN` |
| `endpoint` | TEXT | NOT NULL | Health check endpoint URL |
| `health_status` | TEXT | NOT NULL | `HEALTHY` \| `DEGRADED` \| `OFFLINE` \| `UNKNOWN` |
| `last_checked_at` | TIMESTAMPTZ | NULLABLE | |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT now() | |

**Unique**: `(agent_id, name)`

---

### `noc_health_events`

Time-series of health state changes per component.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | UUID | PK | |
| `component_id` | UUID | FK → noc_components.id | |
| `status` | TEXT | NOT NULL | `HEALTHY` \| `DEGRADED` \| `OFFLINE` \| `UNKNOWN` |
| `occurred_at` | TIMESTAMPTZ | NOT NULL | Agent-reported timestamp |
| `received_at` | TIMESTAMPTZ | NOT NULL, DEFAULT now() | Backend ingest timestamp |
| `diagnostic` | TEXT | NULLABLE | Error message / block lag info |
| `block_number` | BIGINT | NULLABLE | For BESU components |

**Index**: `(component_id, occurred_at DESC)` — primary query pattern for SLO computation

**Retention**: Managed by background worker; keep last 90 days.

---

### `noc_alerts`

Individual failure condition records.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | UUID | PK | |
| `component_id` | UUID | FK → noc_components.id | |
| `incident_id` | UUID | FK → noc_incidents.id, NULLABLE | Set when grouped |
| `severity` | TEXT | NOT NULL | `INFO` \| `WARNING` \| `HIGH` \| `CRITICAL` |
| `state` | TEXT | NOT NULL | `ACTIVE` \| `RESOLVED` |
| `title` | TEXT | NOT NULL | Human-readable summary |
| `root_cause_sig` | TEXT | NOT NULL | `sha256(component_id+type+fingerprint)` |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT now() | |
| `resolved_at` | TIMESTAMPTZ | NULLABLE | |
| `acknowledged_by` | TEXT | NULLABLE | Keycloak subject (`sub` claim) |

**Index**: `(state, created_at DESC)`, `root_cause_sig`

---

### `noc_incidents`

Grouping of related alerts by root cause.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | UUID | PK | |
| `root_cause_sig` | TEXT | NOT NULL, UNIQUE | Dedup key |
| `state` | TEXT | NOT NULL | `OPEN` \| `RESOLVED` |
| `title` | TEXT | NOT NULL | Derived from first alert |
| `spoke_id` | UUID | FK → noc_spokes.id, NULLABLE | Primary affected spoke |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT now() | |
| `resolved_at` | TIMESTAMPTZ | NULLABLE | |

---

### `noc_slo_metrics`

Pre-computed SLO metrics, refreshed every 5 minutes by background worker.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | UUID | PK | |
| `component_id` | UUID | FK → noc_components.id | |
| `window` | TEXT | NOT NULL | `24h` \| `7d` \| `30d` |
| `availability_pct` | DECIMAL(5,2) | NOT NULL | 0.00–100.00 |
| `p50_ms` | INT | NULLABLE | Transaction latency p50 |
| `p95_ms` | INT | NULLABLE | Transaction latency p95 |
| `p99_ms` | INT | NULLABLE | Transaction latency p99 |
| `slo_breached` | BOOLEAN | NOT NULL, DEFAULT false | |
| `breach_duration_seconds` | INT | NULLABLE | Total seconds in breach |
| `computed_at` | TIMESTAMPTZ | NOT NULL, DEFAULT now() | |

**Unique**: `(component_id, window)` — upsert on recompute

---

### `noc_transaction_events`

Lifecycle events of cross-border transactions, ingested from spoke networks.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | UUID | PK | |
| `tx_id` | TEXT | NOT NULL | Platform transaction identifier |
| `event_type` | TEXT | NOT NULL | `INITIATED` \| `HTLC_LOCKED` \| `RELAY_FORWARDED` \| `HTLC_SETTLED` \| `HTLC_REFUNDED` \| `FAILED` |
| `occurred_at` | TIMESTAMPTZ | NOT NULL | Event timestamp |
| `spoke_id` | UUID | FK → noc_spokes.id | Originating spoke |
| `participant_id` | TEXT | NULLABLE | Bank identifier |
| `contract_id` | TEXT | NULLABLE | HTLC contract ID |
| `error_code` | TEXT | NULLABLE | |
| `error_message` | TEXT | NULLABLE | |
| `raw_payload` | JSONB | NULLABLE | Full event for export |
| `received_at` | TIMESTAMPTZ | NOT NULL, DEFAULT now() | |

**Index**: `(tx_id)`, `(spoke_id, occurred_at DESC)`, `(occurred_at DESC)`

**Retention**: 90 days.

---

### `noc_container_logs`

Rolling log buffer per component (last 1,000 lines).

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | BIGSERIAL | PK | |
| `component_id` | UUID | FK → noc_components.id | |
| `stream` | TEXT | NOT NULL | `stdout` \| `stderr` |
| `log_line` | TEXT | NOT NULL | Single log line |
| `occurred_at` | TIMESTAMPTZ | NOT NULL | Timestamp from Docker log |
| `received_at` | TIMESTAMPTZ | NOT NULL, DEFAULT now() | |

**Index**: `(component_id, occurred_at DESC)`

**Rolling buffer**: Background worker deletes rows where `id < (SELECT id FROM noc_container_logs WHERE component_id = $1 ORDER BY id DESC LIMIT 1 OFFSET 999)` per component.

---

### `noc_log_snapshots`

Frozen log snapshots captured at OFFLINE/UNKNOWN transitions.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | UUID | PK | |
| `component_id` | UUID | FK → noc_components.id | |
| `trigger_status` | TEXT | NOT NULL | `OFFLINE` \| `UNKNOWN` |
| `snapshot_at` | TIMESTAMPTZ | NOT NULL | Transition timestamp |
| `lines` | JSONB | NOT NULL | Array of `{stream, line, ts}` objects, max 500 |

**Retention**: 90 days.

---

## State Machines

### HealthStatus transitions

```
HEALTHY ──────────────────────────────→ DEGRADED
   │                                        │
   │                                        ↓
   └──────────────────────────────────→ OFFLINE
                                            │
   UNKNOWN ←── grace period elapsed ────────┘
      │
      └── agent reconnects + healthy push ──→ HEALTHY
```

### Alert state transitions

```
[created] → ACTIVE → RESOLVED
```

### Incident state transitions

```
[created] → OPEN → RESOLVED
```

---

## agent.yaml Schema

```yaml
spoke_id: spoke-a
noc_backend_url: http://noc-backend:8080
api_key: ${NOC_AGENT_KEY}
push_interval_seconds: 15
log_tail_lines: 200
grace_period_multiplier: 3   # = 3 × push_interval_seconds before UNKNOWN

components:
  - name: besu-node
    type: BESU
    endpoint: http://besu:8545
    container_name: cbweb3-besu-spoke-a   # Docker container name for log tailing

  - name: cacti-relay
    type: CACTI_RELAY
    endpoint: http://cacti-relay:4000
    container_name: cbweb3-cacti-relay

  - name: paladin
    type: PALADIN
    endpoint: http://paladin:8548
    container_name: cbweb3-paladin-spoke-a
```
