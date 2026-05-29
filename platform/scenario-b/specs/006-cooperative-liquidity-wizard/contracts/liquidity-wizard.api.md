# API Contracts: Wizard Frontend para Criação Cooperativa de Liquidez

**Feature**: `006-cooperative-liquidity-wizard`
**Date**: 2026-05-15
**Scope**: New API methods added to `frontend/apps/governance/src/services/api/liquidity.api.ts`

These contracts document what the frontend calls. The backend implements them.
No backend changes are part of this feature.

---

## Existing endpoints used (no change)

| Method | Path | Used by |
|---|---|---|
| `POST` | `/api/v2/amm/token/mint-and-approve` | Step 1 (`mintAndApprove` store action) |
| `GET` | `/api/v2/amm/pool/{pair}/status` | Page polling + Step 3 polling |
| `POST` | `/api/v2/amm/liquidity/remove` | Remove Liquidity form (preserved) |

---

## New endpoints called by the wizard

### POST /api/v2/amm/liquidity/commit

Register a liquidity commitment for one side of the pool.

**Request body**:
```json
{
  "pool_pair":   "BRL-USD",
  "provider_id": "central-bank-a",
  "side":        "A",
  "amount":      "100000"
}
```

**Response 201 Created**:
```json
{
  "commit_id":  "a1b2c3d4-...",
  "pool_pair":  "BRL-USD",
  "side":       "A",
  "amount":     "100000",
  "status":     "PENDING",
  "expires_at": "2026-05-18T14:00:00Z",
  "lp_ids":     null
}
```

**Response 201 Created (instant match)**:
```json
{
  "commit_id":  "a1b2c3d4-...",
  "pool_pair":  "BRL-USD",
  "side":       "A",
  "amount":     "100000",
  "status":     "EXECUTED",
  "expires_at": "2026-05-18T14:00:00Z",
  "lp_ids":     ["lp-001-brl", "lp-002-usd"]
}
```

**Error responses**:
| HTTP | body `error` | Frontend action |
|---|---|---|
| 409 | `"SAME_PROVIDER_BOTH_SIDES"` | Show inline error in Step 2 |
| 409 | `"COMMIT_ALREADY_EXISTS"` | Show inline error with link to Step 3 |
| 422 | `"INVALID_AMOUNT"` | Show inline field error |
| 403 | `"NOT_AUTHORIZED_PROVIDER"` | Show inline error |

**Frontend method**:
```typescript
commitLiquidity(payload: CommitRequest): Promise<CommitResult>
// POST /amm/liquidity/commit
```

---

### DELETE /api/v2/amm/liquidity/commits/:commit_id

Cancel a pending commit.

**URL params**: `commit_id` (UUID)  
**Query params**: `provider_id` (string)

**Example**: `DELETE /api/v2/amm/liquidity/commits/a1b2c3d4-...?provider_id=central-bank-a`

**Response 204 No Content**: commit cancelled successfully.

**Error responses**:
| HTTP | Meaning | Frontend action |
|---|---|---|
| 404 | Commit not found | Show inline error in Step 3 |
| 403 | Not the commit owner | Show inline error in Step 3 |
| 409 | Commit already executed | Show inline error in Step 3 |

**Frontend method**:
```typescript
cancelCommit(commitId: string, providerId: string): Promise<void>
// DELETE /amm/liquidity/commits/:commitId?provider_id=:providerId
```

---

### GET /api/v2/amm/liquidity/commits (utility — not wired to wizard)

List commits for a pool pair, optionally filtered by status.

**Query params**:
- `pool_pair` (string, required) — e.g., `"BRL-USD"`
- `status` (string, optional) — `"PENDING"` or `"EXECUTED"`

**Example**: `GET /api/v2/amm/liquidity/commits?pool_pair=BRL-USD&status=PENDING`

**Response 200 OK**:
```json
[
  {
    "commit_id":  "a1b2c3d4-...",
    "pool_pair":  "BRL-USD",
    "side":       "A",
    "amount":     "100000",
    "status":     "PENDING",
    "expires_at": "2026-05-18T14:00:00Z",
    "lp_ids":     null
  }
]
```

**Frontend method**:
```typescript
listCommits(poolPair: string, status?: string): Promise<CommitResult[]>
// GET /amm/liquidity/commits?pool_pair=:poolPair[&status=:status]
```

> **Note**: This method is implemented but **not wired** to any wizard component in
> this feature. The pending-commit banner in `LiquidityManagementPage` uses
> `poolStatus.pending_commits` (returned by the existing pool status endpoint) instead.

---

## Extended pool status response

The existing `GET /api/v2/amm/pool/{pair}/status` endpoint is expected to return
additional fields used by the banner and Step 4. These fields may be absent in older
API versions — the frontend handles them as optional:

```json
{
  "pool_pair":       "BRL-USD",
  "reserve_a":       "100000",
  "reserve_b":       "100000",
  "current_ratio":   "1.00",
  "imbalance_flag":  false,
  "updated_at":      "2026-05-15T12:00:00Z",
  "pool_status":     "ACTIVE",
  "fee_rate_bps":    30,
  "total_lp_count":  2,
  "pending_commits": []
}
```

When `pool_status` is `"PENDING_COUNTERPART"`, `pending_commits` contains summary
entries for each pending commit.
