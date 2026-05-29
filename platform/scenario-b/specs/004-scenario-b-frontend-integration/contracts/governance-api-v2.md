# Governance App — API v2 Contracts

**Feature**: `scenario-b-frontend-integration` | **App**: `frontend/apps/governance` | Date: 2026-05-07

All endpoints require `Authorization: Bearer <JWT>` (injected by `attachAuthInterceptor`).
All amount fields are integer strings. All requests/responses use `Content-Type: application/json`.

---

## Liquidity API

### GET /api/v2/amm/pool/:pair/status

Get real-time pool reserves and imbalance flag (same endpoint as bank-side, central bank role).

**Path param**: `pair=BRL-USD`

**Response 200** (`PoolStatus`):
```json
{
  "pool_pair":      "BRL-USD",
  "reserve_a":      "10000000",
  "reserve_b":      "9500000",
  "current_ratio":  "0.95",
  "imbalance_flag": true,
  "updated_at":     "2026-05-07T12:00:00Z"
}
```

When `imbalance_flag = true`: governance Liquidity page shows a warning banner.

---

### POST /api/v2/amm/liquidity/add

Add a new LP position to the pool.

**Request body** (`AddLiquidityRequest`):
```json
{
  "pool_pair":        "BRL-USD",
  "token_a_amount":   "5000000",
  "token_b_amount":   "5000000",
  "provider_bank_id": "central-bank-a"
}
```

**Response 201** (`LiquidityPosition`):
```json
{
  "lp_id":            "lp-uuid-1",
  "pool_pair":        "BRL-USD",
  "provider_bank_id": "central-bank-a",
  "token_a_amount":   "5000000",
  "token_b_amount":   "5000000",
  "lp_shares":        "4975000",
  "status":           "ACTIVE",
  "added_at":         "2026-05-07T12:00:00Z"
}
```

The returned `LiquidityPosition` is appended to the session-only LP Positions table. No subsequent polling of positions occurs.

---

### POST /api/v2/amm/liquidity/remove

Remove an LP position.

**Request body** (`RemoveLiquidityRequest`):
```json
{
  "lp_id":            "lp-uuid-1",
  "pool_pair":        "BRL-USD",
  "provider_bank_id": "central-bank-a"
}
```

**Response 200**:
```json
{ "status": "ok" }
```

**Error response 404**: Position not found or already withdrawn.
UI shows: "Position not found or already withdrawn"

---

### POST /api/v2/amm/token/mint-and-approve

Mint Hub tokens and approve AMM to spend them (one-time setup).

**Request body** (`MintAndApproveRequest`):
```json
{
  "amount_a":  "10000000",
  "amount_b":  "10000000",
  "recipient": "central-bank-a"  // optional
}
```

**Response 200**:
```json
{ "status": "ok" }
```

---

## Circuit Breaker v2 API

### GET /api/v2/governance/circuit-breaker/status

Get the current circuit-breaker state. Polled every ≤ 15 seconds by the governance Circuit Breaker page.

**Query params**: `pair=BRL-USD`

**Response 200** (`CircuitBreakerV2Status`):
```json
{
  "pair":              "BRL-USD",
  "state":             "LIVE",
  "pause_initiator":   null,
  "pause_reason":      null,
  "resume_request_id": null
}
```

When `state = "RESUME_PENDING"`:
```json
{
  "pair":              "BRL-USD",
  "state":             "RESUME_PENDING",
  "pause_initiator":   "central-bank-a",
  "pause_reason":      "LIQUIDITY_CONCERN",
  "resume_request_id": "req-uuid-1"
}
```

---

### POST /api/v2/governance/circuit-breaker/pause

Immediately pause the AMM.

**Request body** (`PauseRequest`):
```json
{
  "pair":        "BRL-USD",
  "bank_id":     "central-bank-a",
  "reason_code": "LIQUIDITY_CONCERN",
  "signature":   "AA="
}
```

**Response 200** (`CircuitBreakerV2Status`):
```json
{
  "pair":              "BRL-USD",
  "state":             "HALTED",
  "pause_initiator":   "central-bank-a",
  "pause_reason":      "LIQUIDITY_CONCERN",
  "resume_request_id": null
}
```

---

### POST /api/v2/governance/circuit-breaker/resume-request

Propose a resume (initiates multi-party signing flow).

**Request body** (`ProposeResumeRequest`):
```json
{
  "pair":      "BRL-USD",
  "bank_id":   "central-bank-a",
  "signature": "AA="
}
```

**Response 200** (`ProposeResumeResponse`):
```json
{
  "request_id": "req-uuid-1",
  "state":      "RESUME_PENDING"
}
```

The returned `request_id` MUST be displayed prominently so the operator can share it for co-signing.

---

### POST /api/v2/governance/circuit-breaker/resume-sign

Co-sign a resume proposal.

**Request body** (`SignResumeRequest`):
```json
{
  "pair":       "BRL-USD",
  "request_id": "req-uuid-1",
  "bank_id":    "central-bank-b",
  "signature":  "AA="
}
```

**Response 200 — quorum met** (`CircuitBreakerV2Status`):
```json
{
  "pair":              "BRL-USD",
  "state":             "LIVE",
  "pause_initiator":   null,
  "pause_reason":      null,
  "resume_request_id": null
}
```

**Response 200 — quorum NOT yet met**:
```json
{
  "pair":              "BRL-USD",
  "state":             "RESUME_PENDING",
  "pause_initiator":   "central-bank-a",
  "pause_reason":      "LIQUIDITY_CONCERN",
  "resume_request_id": "req-uuid-1"
}
```

UI shows "Waiting for additional co-signatures" when state remains `RESUME_PENDING`.

---

## Oversight API

### POST /api/v2/oversight/disclosure-request

Open an AML/CFT disclosure request.

**Request body** (`OpenDisclosureRequest`):
```json
{
  "tx_ref":       "tx-ref-abc123",
  "requestor_id": "central-bank-a",
  "reason_code":  "AML_INVESTIGATION"
}
```

**Response 201** (`DisclosureRequest`):
```json
{
  "request_id":      "disc-uuid-1",
  "tx_ref":          "tx-ref-abc123",
  "requestor_id":    "central-bank-a",
  "reason_code":     "AML_INVESTIGATION",
  "state":           "PENDING",
  "quorum_reached":  0,
  "quorum_required": 2,
  "expires_at":      "2026-05-08T12:00:00Z",
  "closed_at":       null
}
```

---

### POST /api/v2/oversight/disclosure-sign

Co-sign a disclosure request.

**Request body** (`SignDisclosureRequest`):
```json
{
  "request_id": "disc-uuid-1",
  "signer_id":  "central-bank-b"
}
```

**Response 200**:
```json
{ "status": "signed" }
```

**Error 422** — duplicate signature:
```json
{ "error": "Already signed by this signer or request is closed" }
```

UI shows error inline without crashing.

---

### GET /api/v2/oversight/disclosure-status/:requestID

Fetch the current state of a disclosure request.

**Path param**: `requestID=disc-uuid-1`

**Response 200** (`DisclosureRequest`):
```json
{
  "request_id":      "disc-uuid-1",
  "tx_ref":          "tx-ref-abc123",
  "requestor_id":    "central-bank-a",
  "reason_code":     "AML_INVESTIGATION",
  "state":           "QUORUM_REACHED",
  "quorum_reached":  2,
  "quorum_required": 2,
  "expires_at":      "2026-05-08T12:00:00Z",
  "closed_at":       "2026-05-07T14:00:00Z"
}
```

**Error 404**: Request ID not found.
