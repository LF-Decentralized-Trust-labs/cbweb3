# Data Model: Supervisor Portal (014)

## Entities

### AuditLogEntry (read-only, both scenarios)

Source: `audit_log` table (Scenario B: api-gateway DB; Scenario A: compliance service DB via gRPC).  
The frontend only reads; no writes originate from the supervisor portal.

| Field | Type | Notes |
|---|---|---|
| `id` | uint | Auto-increment PK |
| `actor` | string | Username or service identity that triggered the event |
| `action` | string | Machine-readable event kind (e.g. `POOL_STATUS_VIEW`, `KYC_APPROVED`) |
| `target` | string | Affected resource reference (tx hash, bank ID, pair name) |
| `timestamp` | ISO-8601 string | `created_at` from DB |
| `status` | `SUCCESS \| FAILED` | Outcome of the action |
| `severity` | `INFO \| WARNING \| CRITICAL` | Mapped from `event_kind` pattern at query time |
| `category` | `GOVERNANCE \| COMPLIANCE \| SETTLEMENT` | Mapped from `table_name` at query time |

Query filters: `category`, `severity`, `from_date`, `to_date`, `page` (default 1), `limit` (default 50, max 100).

---

### ZKPointerVerification (Scenario B only, read-only)

Resolved from `compliance_zk_pointers` table via `ZKComplianceGate.ValidateZKPointer`.  
The supervisor never writes ZK pointers; this is a point-in-time lookup.

| Field | Type | Notes |
|---|---|---|
| `bank_id` | string | The queried bank identifier |
| `pointer_id` | string | `PointerID` from the DB record |
| `commitment_hash` | string | The validated commitment hash |
| `state` | `VALID \| INVALID \| EXPIRED` | Derived: VALID = found + non-expired; EXPIRED = found + past `expires_at`; INVALID = not found |
| `expires_at` | ISO-8601 string \| null | From DB record; null if no expiry set |

HTTP query params: `bank_id` (required), `commitment_hash` (required).  
HTTP response: `200 OK` with body, or `404` if no matching record.

---

### DisclosureRequest (both scenarios — Scenario B: existing; Scenario A: new)

Scenario B table: `disclosure_requests` (compliance service DB, accessed via `OversightService`).  
Scenario A table: `disclosure_requests` (new, same schema, same service in Scenario A compliance).

| Field | Type | Notes |
|---|---|---|
| `request_id` | UUID string | PK |
| `requested_by_bank_id` | string | `requestor_id` from open request |
| `target_transaction_ref` | string | `tx_ref` from open request |
| `reason_code` | `AML_ALERT \| CFT_INVESTIGATION \| COURT_ORDER \| REGULATORY_EXAM` | Validated enum |
| `state` | `PENDING \| QUORUM_REACHED \| EXPIRED` | State machine |
| `quorum_required` | int | Hardcoded 2 |
| `quorum_reached` | int | Incremented on each valid signature |
| `opened_at` | ISO-8601 | Set on creation |
| `expires_at` | ISO-8601 | `opened_at + 72h` |

#### State Transitions

```
PENDING ──(second CB sign)──► QUORUM_REACHED  [terminal: requires Paladin action off-portal]
PENDING ──(72h elapsed)──────► EXPIRED         [terminal: new request must be opened]
```

No transition from QUORUM_REACHED or EXPIRED back to PENDING.

---

### DisclosureSignature (both scenarios — Scenario B: existing; Scenario A: new)

| Field | Type | Notes |
|---|---|---|
| `sig_id` | UUID string | PK |
| `request_id` | UUID string | FK → DisclosureRequest |
| `signer_bank_id` | string | Identity of the signing CB |
| `signed_at` | ISO-8601 | Timestamp of signature |

Constraint: `(request_id, signer_bank_id)` is unique — one signature per signer per request.

---

## Frontend Type Additions

Both supervisor apps need the following new TypeScript types added to `src/types/`:

```typescript
// audit.types.ts — extend existing
export interface AuditLogEntry {
  id: string;
  actor: string;
  action: string;
  target: string;
  timestamp: string;
  status: "SUCCESS" | "FAILED";
  severity?: "INFO" | "WARNING" | "CRITICAL";
  category?: "GOVERNANCE" | "COMPLIANCE" | "SETTLEMENT";
}

export interface AuditLogFilters {
  category?: string;
  severity?: string;
  fromDate?: string;
  toDate?: string;
  page: number;
  limit: number;
}

// investigation.types.ts — new file
export type ReasonCode = "AML_ALERT" | "CFT_INVESTIGATION" | "COURT_ORDER" | "REGULATORY_EXAM";
export type DisclosureState = "PENDING" | "QUORUM_REACHED" | "EXPIRED";

export interface DisclosureRequest {
  requestId: string;
  txRef: string;
  requestorId: string;
  reasonCode: ReasonCode;
  state: DisclosureState;
  quorumRequired: number;
  quorumReached: number;
  openedAt: string;
  expiresAt: string;
}

export interface OpenDisclosurePayload {
  txRef: string;
  requestorId: string;
  reasonCode: ReasonCode;
}

export interface SignDisclosurePayload {
  requestId: string;
  signerId: string;
}

// zk-pointer.types.ts — new file, Scenario B only
export type ZKPointerState = "VALID" | "INVALID" | "EXPIRED";

export interface ZKPointerVerification {
  bankId: string;
  pointerId: string;
  commitmentHash: string;
  state: ZKPointerState;
  expiresAt: string | null;
}
```
