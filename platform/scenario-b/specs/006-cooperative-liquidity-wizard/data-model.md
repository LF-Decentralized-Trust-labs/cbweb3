# Data Model: Wizard Frontend para Criação Cooperativa de Liquidez

**Feature**: `006-cooperative-liquidity-wizard`
**Date**: 2026-05-15
**File**: `frontend/apps/governance/src/types/liquidity.types.ts`

---

## New Types

### `PoolLifecycleStatus`

```typescript
export type PoolLifecycleStatus = "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE";
```

State machine for the AMM pool's commit-reveal lifecycle.

| Value | Meaning |
|---|---|
| `"EMPTY"` | No commits. Pool is idle (or a commit expired/was cancelled). |
| `"PENDING_COUNTERPART"` | One side has committed; waiting for the other side. |
| `"ACTIVE"` | Both sides committed; pool is operational. |

---

### `CommitSide`

```typescript
export type CommitSide = "A" | "B";
```

Identifies which side of the currency pair the provider is depositing.
`"A"` = BRL side (CB-A); `"B"` = USD side (CB-B).

---

### `CommitStatus`

```typescript
export type CommitStatus = "PENDING" | "EXECUTED";
```

| Value | Meaning |
|---|---|
| `"PENDING"` | Commit registered; awaiting counterpart. `lp_ids` will be `null`. |
| `"EXECUTED"` | Both sides committed; LP shares minted. `lp_ids` will be populated. |

---

### `CommitRequest`

```typescript
export interface CommitRequest {
  pool_pair: string;    // e.g., "BRL-USD"
  provider_id: string;  // e.g., "central-bank-a"
  side: CommitSide;     // "A" or "B"
  amount: string;       // integer as string, e.g., "100000"
}
```

Payload for `POST /api/v2/amm/liquidity/commit`.

---

### `CommitResult`

```typescript
export interface CommitResult {
  commit_id: string;     // UUID
  pool_pair: string;     // e.g., "BRL-USD"
  side: CommitSide;
  amount: string;
  status: CommitStatus;
  expires_at: string;    // ISO 8601, e.g., "2026-05-18T14:00:00Z"
  lp_ids: string[] | null;  // null when status=PENDING; [lpA, lpB] when EXECUTED
}
```

Response from `POST /api/v2/amm/liquidity/commit`. Also used for `GET /commits`.

---

### `PendingCommitSummary`

```typescript
export interface PendingCommitSummary {
  commit_id: string;
  provider_id: string;
  side: CommitSide;
  amount: string;
  expires_at: string;  // ISO 8601
}
```

Embedded inside `PoolStatus.pending_commits`. Used by `LiquidityManagementPage` to
determine whether the pending-commit banner should be shown for the current operator.

---

## Modified Types

### `PoolStatus` (extended)

```typescript
export interface PoolStatus {
  // --- existing fields (unchanged) ---
  pool_pair: string;
  reserve_a: string;
  reserve_b: string;
  current_ratio: string;
  imbalance_flag: boolean;
  updated_at: string;

  // --- new fields ---
  pool_status?: PoolLifecycleStatus;       // lifecycle state
  fee_rate_bps?: number;                   // e.g., 30 = 0.30%
  total_lp_count?: number;                 // number of active LP positions
  pending_commits?: PendingCommitSummary[]; // populated when pool_status=PENDING_COUNTERPART
}
```

All new fields are optional (`?`) to maintain backward compatibility with API
responses that may not yet include them.

---

## Wizard Internal State

Not persisted — lives in `CooperativeLiquidityWizard` component local state.

```typescript
enum WizardStep {
  MINT_APPROVE = 1,
  COMMIT       = 2,
  MONITOR      = 3,
  SUCCESS      = 4,
}
```

---

## Store Extensions

Added to `LiquidityStore` type in `liquidity.store.ts`:

```typescript
// State
activeCommit: CommitResult | null;
commitStatus: "idle" | "loading" | "error";
commitError: string | null;

// Actions
submitCommit(payload: CommitRequest): Promise<void>;
cancelActiveCommit(commitId: string, providerId: string): Promise<void>;
clearCommit(): void;
fetchPendingCommits(poolPair: string): Promise<PendingCommitSummary[]>;
```

`activeCommit` is set by `submitCommit` on success and cleared by
`cancelActiveCommit` or `clearCommit`. It is the source of truth for LP IDs shown
in Step 4.

---

## Entity Relationships

```
PoolStatus (1) ──────────── (0..*) PendingCommitSummary
    │
    └─ pool_status: PoolLifecycleStatus

CommitRequest ──→ [POST /amm/liquidity/commit] ──→ CommitResult
    │                                                   │
    └─ provider_id, side, amount                        └─ stored as activeCommit in store
                                                            lp_ids null when PENDING
                                                            lp_ids [lpA, lpB] when EXECUTED
```

---

## Validation Rules

| Field | Rule |
|---|---|
| `CommitRequest.amount` | Non-empty string; must match `/^[0-9]+$/`; value > 0 |
| `CommitRequest.pool_pair` | Non-empty; default `"BRL-USD"` |
| `CommitRequest.provider_id` | Non-empty |
| `CommitRequest.side` | Must be `"A"` or `"B"` |
| `MintAndApproveRequest.amount_b` | Required; must be positive integer string |
| `MintAndApproveRequest.amount_a` | Optional for CB-B side; must be positive integer string if provided |

All validation is frontend-side only (pre-submit form validation). Backend is
authoritative for business rules.
