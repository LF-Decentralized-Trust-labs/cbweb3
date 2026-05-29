# Frontend API Contracts: CB Liquidity Refactor (009)

Defines the API surface consumed by `frontend/apps/governance` after the 009 refactor.  
All endpoints are in the `api-gateway` and accessed via `httpClientV2` (base `/api/v2`).

---

## POST /api/v2/bridge/lock-mint

**Purpose**: Step 1 of sovereign CB liquidity flow — lock native tokens and mint wrapped equivalent on spoke.

**Request body**:
```json
{ "amount": "100000000000000000000000" }
```
*No other fields. `recipient`, `token`, and `network` are derived server-side from JWT + env config.*

**Response** `200 OK`:
```json
{
  "position_id": "abc-123",
  "bridge_state": "LOCKING",
  "spoke_network": "polygon-amoy",
  "native_asset": "BRL",
  "mirrored_asset": "W-BRL",
  "amount": "100000000000000000000000"
}
```

**Error cases**:
- `403 CROSS_CB_MINT_PROHIBITED` — caller is not authorized for sovereign CB lock-mint (wrong role/network)
- `400` — missing or invalid `amount`

**Frontend action**: `lockMint(amount)` → transitions phase to `BRIDGE_ACTIVE_WAIT` → polls `/bridge/positions?state=ACTIVE`.

---

## GET /api/v2/bridge/positions?state=ACTIVE

**Purpose**: Poll until the bridge position created by lock-mint reaches `ACTIVE` state.

**Query params**: `state=ACTIVE`

**Response** `200 OK` (either shape):
```json
{ "positions": [{ "position_id": "abc-123", "bridge_state": "ACTIVE" }] }
```
or bare array:
```json
[{ "position_id": "abc-123", "bridge_state": "ACTIVE" }]
```
*Frontend normalizes both shapes. Returns empty array when not yet active.*

**Polling**: 5 s interval, 120 s timeout (existing `SOVEREIGN_FLOW_POLICY` values, unchanged).

---

## POST /api/v2/amm/liquidity/commit

**Purpose**: Step 2 — submit a liquidity commit (simplified payload, no client-supplied `provider_id`/`side`).

**Request body**:
```json
{ "pool_pair": "W-BRL-ARS", "amount": "100000000000000000000000" }
```
*`provider_id`, `side`, and `w_token_address` are derived server-side.*

**Response** `200 OK`:
```json
{
  "commit_id": "commit-abc",
  "on_chain_commit_id": "0xdeadbeef...",
  "status": "PENDING",
  "side": "A",
  "pool_pair": "W-BRL-ARS",
  "expires_at": "2026-05-26T12:05:00Z"
}
```
*`on_chain_commit_id` may be `null` if on-chain confirmation is still pending.*

**Error cases**:
- `409 BRIDGE_POSITION_NOT_ACTIVE` — bridge not yet active; keep polling before retrying commit

---

## GET /api/v2/amm/liquidity/commits

**Purpose**: Poll for commit status in Step 3.

**Query params**: `pool_pair=W-BRL-ARS`, `status=EXECUTED` (optional filter)

**Response** `200 OK` — **PascalCase fields** (no `json:` tags on `domain.PoolCommit`):
```json
{
  "commits": [
    {
      "CommitID": "commit-abc",
      "PoolPair": "W-BRL-ARS",
      "ProviderID": "bank-central-a",
      "Side": "A",
      "Amount": "100000000000000000000000",
      "Status": "EXECUTED",
      "OnChainCommitID": "3q2+7w==",
      "CounterpartCommitID": null,
      "CreatedAt": "2026-05-26T12:00:00Z",
      "ExpiresAt": "2026-05-26T12:05:00Z"
    }
  ]
}
```
*`OnChainCommitID` is `*[]byte` → base64 string or `null`.*  
*Frontend MUST use `CommitListItem` type with PascalCase accessors.*

---

## GET /api/v2/amm/liquidity/positions

**Purpose**: Step 4 — fetch LP position details for the authenticated CB after commit execution.

**Query params**: `pool_pair=W-BRL-ARS` (required), `provider_id=bank-central-a` (required for per-CB filtering)

**Response** `200 OK` — snake_case (handler maps domain struct to explicit DTO with `json:` tags):
```json
{
  "pool_pair": "W-BRL-ARS",
  "positions": [
    {
      "lp_id": "lp-001",
      "provider_bank_id": "bank-central-a",
      "pool_pair": "W-BRL-ARS",
      "token_a_contributed": "100000000000000000000000",
      "token_b_contributed": "0",
      "lp_shares": "0",
      "status": "ACTIVE",
      "deposit_side": "A",
      "added_at": "2026-05-26T12:02:30Z",
      "commit_id": "commit-abc"
    }
  ],
  "count": 1
}
```
*`commit_id` is omitted (`omitempty`) when null. Frontend must handle missing `commit_id` gracefully.*

---

## DELETE /api/v2/amm/liquidity/commits/:id

**Purpose**: Cancel a pending commit.

**Query params**: `provider_id=<bankId>` — **required** (CancelCommit handler was NOT updated in 008; still returns HTTP 400 if absent).  
*Frontend derives `bankId` from store state (auth profile); the public `cancelActiveCommit` action signature no longer requires the caller to pass it.*

**Response** `200 OK`:
```json
{ "message": "commit cancelled", "commit_id": "<id>" }
```

---

## Removed/Dead Endpoints (do not call)

| Endpoint | Status | Replacement |
|---|---|---|
| `POST /api/v2/amm/token/mint-and-approve` | HTTP 403 `CROSS_CB_MINT_PROHIBITED` | `POST /api/v2/bridge/lock-mint` |
| `POST /api/v2/amm/liquidity/add` | Dead endpoint (no handler) | Removed — not needed in sovereign flow |
