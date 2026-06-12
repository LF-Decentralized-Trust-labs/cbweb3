# API Contracts: Supervisor Portal (014)

All endpoints require a valid Keycloak session cookie (`SESSION` or `access_token`).  
All endpoints return `Content-Type: application/json`.  
All error responses use `{ "error": "<message>" }`.

---

## Shared (Scenarios A and B)

### GET /api/v1/compliance/audit/logs

**Role required**: `ROLE_SUPERVISOR`  
**Handler**: `SupervisorHandler.GetAuditLogs`  
**Backed by**: same `ComplianceGRPCAdapter.GetAuditLogs` used by `GovernanceHandler`

**Query Parameters**:
| Param | Type | Required | Default | Notes |
|---|---|---|---|---|
| `category` | string | No | (all) | `GOVERNANCE`, `COMPLIANCE`, `SETTLEMENT` |
| `severity` | string | No | (all) | `INFO`, `WARNING`, `CRITICAL` |
| `from_date` | string | No | (none) | ISO-8601 date |
| `to_date` | string | No | (none) | ISO-8601 date |
| `page` | int | No | 1 | 1-based |
| `limit` | int | No | 50 | Max 100 |

**200 OK**:
```json
{
  "logs": [
    {
      "id": "1",
      "actor": "supervisor@centralbank.gov",
      "action": "POOL_STATUS_VIEW",
      "target": "BRL/ARS",
      "timestamp": "2026-06-11T14:30:00Z",
      "status": "SUCCESS",
      "severity": "INFO",
      "category": "GOVERNANCE"
    }
  ],
  "page": 1,
  "limit": 50,
  "total": 1
}
```

**403 Forbidden**: caller does not have `ROLE_SUPERVISOR`  
**500 Internal Server Error**: compliance service unavailable

---

### POST /oversight/disclosure-request

**Scenario A path**: `/api/v1/oversight/disclosure-request`  
**Scenario B path**: `/api/v2/oversight/disclosure-request` (existing)  
**Role required**: `ROLE_SUPERVISOR` (Scenario A); `ROLE_CENTRAL_BANK` (Scenario B — existing)  
**Handler**: `OversightHandler.OpenDisclosure` (both scenarios)

**Request Body**:
```json
{
  "tx_ref": "0x8f41b290d90e10a89c90f6d20d9eae1288a8ff1f9d",
  "requestor_id": "cb_lnet",
  "reason_code": "AML_ALERT"
}
```
`reason_code` must be one of: `AML_ALERT`, `CFT_INVESTIGATION`, `COURT_ORDER`, `REGULATORY_EXAM`.

**201 Created**:
```json
{
  "request_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "state": "PENDING",
  "quorum_required": 2,
  "quorum_reached": 0,
  "opened_at": "2026-06-11T14:30:00Z",
  "expires_at": "2026-06-14T14:30:00Z"
}
```

**400 Bad Request**: missing required fields or invalid `reason_code`  
**422 Unprocessable Entity**: domain error (e.g. tx_ref already has an open request)

---

### POST /oversight/disclosure-sign

**Scenario A path**: `/api/v1/oversight/disclosure-sign`  
**Scenario B path**: `/api/v2/oversight/disclosure-sign` (existing)  
**Role required**: `ROLE_SUPERVISOR` (Scenario A); `ROLE_CENTRAL_BANK` (Scenario B — existing)

**Request Body**:
```json
{
  "request_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "signer_id": "cb_spoke_a"
}
```

**200 OK**:
```json
{ "status": "signed" }
```

**400 Bad Request**: `signer_id` has already signed this request  
**404 Not Found**: `request_id` does not exist  
**422 Unprocessable Entity**: request expired

---

### GET /oversight/disclosure-status/:requestID

**Scenario A path**: `/api/v1/oversight/disclosure-status/:requestID`  
**Scenario B path**: `/api/v2/oversight/disclosure-status/:requestID` (existing)  
**Role required**: `ROLE_SUPERVISOR` (Scenario A); open (Scenario B — existing, no role guard)

**200 OK**:
```json
{
  "request_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "tx_ref": "0x8f41b290d90e10a89c90f6d20d9eae1288a8ff1f9d",
  "reason_code": "AML_ALERT",
  "state": "QUORUM_REACHED",
  "quorum_required": 2,
  "quorum_reached": 2,
  "opened_at": "2026-06-11T14:30:00Z",
  "expires_at": "2026-06-14T14:30:00Z"
}
```

**404 Not Found**: `requestID` does not exist

---

## Scenario B Only

### GET /api/v1/compliance/zk-pointer/verify

**Role required**: `ROLE_SUPERVISOR`  
**Handler**: `SupervisorHandler.VerifyZKPointer`  
**Backed by**: `ZKComplianceGate.ValidateZKPointer` (direct DB call — see research D-02)

**Query Parameters**:
| Param | Type | Required | Notes |
|---|---|---|---|
| `bank_id` | string | Yes | Bank identifier |
| `commitment_hash` | string | Yes | ZK commitment hash to validate |

**200 OK** (pointer found and valid):
```json
{
  "bank_id": "cb_spoke_a",
  "pointer_id": "ptr_abc123",
  "commitment_hash": "0xabc123...",
  "state": "VALID",
  "expires_at": "2026-12-31T00:00:00Z"
}
```

**200 OK** (pointer found but expired):
```json
{
  "bank_id": "cb_spoke_a",
  "pointer_id": "ptr_abc123",
  "commitment_hash": "0xabc123...",
  "state": "EXPIRED",
  "expires_at": "2026-01-01T00:00:00Z"
}
```

**404 Not Found** (no matching pointer):
```json
{ "error": "no valid ZK-Pointer found for this bank and commitment" }
```

**400 Bad Request**: missing `bank_id` or `commitment_hash`
