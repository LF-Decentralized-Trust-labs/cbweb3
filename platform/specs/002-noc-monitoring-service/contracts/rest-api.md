# API Contracts: NOC Backend REST API

**Service**: `noc-backend` | **Base URL**: `http://noc-backend:8090`  
**Auth**: Bearer JWT (Keycloak) for frontend-facing routes; `X-Agent-Key` header for agent push routes.  
**Format**: JSON request/response bodies. Errors follow `{"error": "<message>", "code": "<code>"}`.

---

## Authentication & Roles

| Role | Description |
|------|-------------|
| `noc-viewer` | Read-only access to all dashboards, alerts, SLO, logs, audit |
| `noc-operator` | `noc-viewer` + can acknowledge/resolve alerts |
| `noc-admin` | `noc-operator` + can register/deregister spokes |
| `agent` | Internal — authenticated via `X-Agent-Key` header only |

---

## Service Health

### `GET /api/v1/health`
Liveness probe. No auth required.

**Response 200**:
```json
{ "status": "ok", "uptime": 3600 }
```

---

## Network Overview

### `GET /api/v1/overview`
Full network overview: all spokes and their component statuses.  
**Auth**: `noc-viewer`+

**Response 200**:
```json
{
  "spokes": [
    {
      "id": "uuid",
      "name": "spoke-a",
      "currency_code": "BRL",
      "jurisdiction": "Brazil",
      "overall_status": "DEGRADED",
      "agent_status": "REACHABLE",
      "components": [
        {
          "id": "uuid",
          "name": "besu-node",
          "type": "BESU",
          "health_status": "HEALTHY",
          "last_checked_at": "2026-05-22T14:00:00Z"
        },
        {
          "id": "uuid",
          "name": "cacti-relay",
          "type": "CACTI_RELAY",
          "health_status": "OFFLINE",
          "last_checked_at": "2026-05-22T13:58:00Z"
        }
      ],
      "active_alert_count": 1
    }
  ]
}
```

`overall_status`: worst status among all components (`HEALTHY` < `DEGRADED` < `OFFLINE` < `UNKNOWN`).

---

## Spokes

### `GET /api/v1/spokes`
List all registered spokes.  
**Auth**: `noc-viewer`+

### `GET /api/v1/spokes/{spoke_id}`
Spoke detail with component list.  
**Auth**: `noc-viewer`+

### `GET /api/v1/spokes/{spoke_id}/components`
All components for a spoke with current health.  
**Auth**: `noc-viewer`+

---

## Components

### `GET /api/v1/components/{component_id}`
Component detail + last 20 health events.  
**Auth**: `noc-viewer`+

**Response 200**:
```json
{
  "id": "uuid",
  "name": "besu-node",
  "type": "BESU",
  "spoke_id": "uuid",
  "health_status": "HEALTHY",
  "last_checked_at": "2026-05-22T14:00:00Z",
  "recent_health_events": [
    {
      "status": "OFFLINE",
      "occurred_at": "2026-05-22T13:55:00Z",
      "diagnostic": "connection refused"
    },
    {
      "status": "HEALTHY",
      "occurred_at": "2026-05-22T13:58:00Z",
      "diagnostic": null
    }
  ]
}
```

### `GET /api/v1/components/{component_id}/logs`
Recent container log lines.  
**Auth**: `noc-viewer`+

**Query params**:
- `limit` (int, default 100, max 1000)
- `since` (ISO8601 timestamp, optional)
- `keyword` (string, optional — substring filter)
- `stream` (`stdout` | `stderr` | `all`, default `all`)

**Response 200**:
```json
{
  "component_id": "uuid",
  "lines": [
    { "stream": "stdout", "log_line": "[relay] AcceptFXAgreement...", "occurred_at": "2026-05-22T14:00:01Z" }
  ],
  "total": 87
}
```

### `GET /api/v1/components/{component_id}/log-snapshots`
List OFFLINE/UNKNOWN snapshots.  
**Auth**: `noc-viewer`+

**Response 200**:
```json
{
  "snapshots": [
    {
      "id": "uuid",
      "trigger_status": "OFFLINE",
      "snapshot_at": "2026-05-22T13:55:00Z",
      "line_count": 500
    }
  ]
}
```

### `GET /api/v1/components/{component_id}/log-snapshots/{snapshot_id}`
Full snapshot content.  
**Auth**: `noc-viewer`+

---

## Alerts

### `GET /api/v1/alerts`
List alerts.  
**Auth**: `noc-viewer`+

**Query params**:
- `state` (`ACTIVE` | `RESOLVED` | `all`, default `ACTIVE`)
- `severity` (`INFO` | `WARNING` | `HIGH` | `CRITICAL`, optional)
- `spoke_id` (UUID, optional)
- `limit` (int, default 50)
- `offset` (int, default 0)

**Response 200**:
```json
{
  "alerts": [
    {
      "id": "uuid",
      "severity": "HIGH",
      "state": "ACTIVE",
      "title": "cacti-relay OFFLINE on spoke-a",
      "component_id": "uuid",
      "incident_id": "uuid",
      "created_at": "2026-05-22T13:55:00Z",
      "resolved_at": null
    }
  ],
  "total": 3
}
```

### `GET /api/v1/alerts/{alert_id}`
Alert detail.  
**Auth**: `noc-viewer`+

### `PATCH /api/v1/alerts/{alert_id}/resolve`
Manually resolve an alert.  
**Auth**: `noc-operator`+

**Response 200**:
```json
{ "id": "uuid", "state": "RESOLVED", "resolved_at": "2026-05-22T14:05:00Z" }
```

---

## Incidents

### `GET /api/v1/incidents`
List incidents.  
**Auth**: `noc-viewer`+

**Query params**: `state`, `spoke_id`, `limit`, `offset`

---

## SLO Metrics

### `GET /api/v1/slo`
Pre-computed SLO metrics.  
**Auth**: `noc-viewer`+

**Query params**:
- `spoke_id` (UUID, optional — omit for all)
- `component_id` (UUID, optional)
- `window` (`24h` | `7d` | `30d`, default `24h`)

**Response 200**:
```json
{
  "metrics": [
    {
      "component_id": "uuid",
      "component_name": "besu-node",
      "spoke_id": "uuid",
      "window": "24h",
      "availability_pct": 99.72,
      "p50_ms": 120,
      "p95_ms": 450,
      "p99_ms": 890,
      "slo_breached": false,
      "breach_duration_seconds": 0,
      "computed_at": "2026-05-22T14:00:00Z"
    }
  ]
}
```

---

## Audit Log (Transaction Events)

### `GET /api/v1/transactions`
Search transaction events.  
**Auth**: `noc-viewer`+

**Query params**:
- `tx_id` (string, optional)
- `spoke_id` (UUID, optional)
- `participant_id` (string, optional)
- `event_type` (string, optional)
- `from` (ISO8601, optional)
- `to` (ISO8601, optional)
- `limit` (int, default 50)
- `offset` (int, default 0)

### `GET /api/v1/transactions/{tx_id}/events`
All events for a single transaction in chronological order.  
**Auth**: `noc-viewer`+

**Response 200**:
```json
{
  "tx_id": "33581ea8-55c2-4a6d-b8ae-4f6357363e38",
  "events": [
    {
      "id": "uuid",
      "event_type": "INITIATED",
      "occurred_at": "2026-05-22T14:00:00Z",
      "spoke_id": "uuid",
      "participant_id": "bank-a",
      "contract_id": null,
      "error_code": null
    }
  ]
}
```

### `GET /api/v1/transactions/export`
Export transactions as JSON for regulatory reporting.  
**Auth**: `noc-viewer`+

**Query params**: `from` (required), `to` (required), `spoke_id` (optional), `format` (`json` | `csv`, default `json`)

---

## Spoke Registry Admin

### `GET /api/v1/admin/spokes`
List all spokes including inactive.  
**Auth**: `noc-admin`

### `POST /api/v1/admin/spokes`
Register a new spoke.  
**Auth**: `noc-admin`

**Body**:
```json
{
  "name": "spoke-c",
  "currency_code": "CLP",
  "jurisdiction": "Chile"
}
```

**Response 201**:
```json
{ "id": "uuid", "name": "spoke-c", "registered_at": "..." }
```

### `DELETE /api/v1/admin/spokes/{spoke_id}`
Deregister (soft-delete) a spoke.  
**Auth**: `noc-admin`

**Response 204** (no body)

---

## Agent Key Provisioning (Admin)

### `POST /api/v1/admin/agents/provision-key`
Pre-provision a raw API key for an agent, binding it to a specific spoke before its first push. Backend stores only the SHA-256 hash. On the agent's first push, the backend validates that the push payload's `spoke_id` matches the provisioned binding; if they differ, the push is rejected with HTTP 403.  
**Auth**: `noc-admin`

**Body**:
```json
{
  "raw_key": "sk-live-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "spoke_id": "uuid-of-spoke-a",
  "hint": "agent-spoke-a"
}
```

**Response 201**:
```json
{ "key_hash": "sha256:...", "spoke_id": "uuid-of-spoke-a", "hint": "agent-spoke-a", "created_at": "..." }
```

**Response 409** (key hash already exists):
```json
{ "error": "key already provisioned", "code": "KEY_EXISTS" }
```

---

## Agent Push (Internal)

### `POST /internal/v1/push`
Receive health metrics and log lines from a NOC Agent.  
**Auth**: `X-Agent-Key: <raw-api-key>` (no Bearer token)

**Body**:
```json
{
  "agent_id": "uuid",
  "spoke_id": "spoke-a",
  "pushed_at": "2026-05-22T14:00:00Z",
  "components": [
    {
      "name": "besu-node",
      "type": "BESU",
      "health_status": "HEALTHY",
      "checked_at": "2026-05-22T14:00:00Z",
      "block_number": 18432,
      "diagnostic": null
    }
  ],
  "logs": [
    {
      "component_name": "cacti-relay",
      "lines": [
        { "stream": "stdout", "line": "[relay] poll ok", "ts": "2026-05-22T14:00:00Z" }
      ]
    }
  ],
  "transaction_events": [
    {
      "tx_id": "33581ea8-55c2-4a6d-b8ae-4f6357363e38",
      "event_type": "HTLC_LOCKED",
      "occurred_at": "2026-05-22T14:00:01Z",
      "participant_id": "bank-a",
      "contract_id": "0xabc...",
      "error_code": null
    }
  ]
}
```

**Response 200**:
```json
{ "accepted": true }
```

**Response 401** (missing/unknown key):
```json
{ "error": "unauthorized", "code": "INVALID_AGENT_KEY" }
```
