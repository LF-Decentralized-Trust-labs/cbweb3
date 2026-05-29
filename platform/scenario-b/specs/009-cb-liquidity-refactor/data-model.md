# Data Model: CB Liquidity Frontend Refactor (009)

**Phase 1 output** — TypeScript type changes for the governance app.

---

## Types to Add

### `BridgeLockMintRequest`
```ts
interface BridgeLockMintRequest {
  amount: string;
}
```
*Request payload for `POST /api/v2/bridge/lock-mint`. All other fields (`recipient`, `token`, `network`) are derived server-side from JWT + env config.*

---

### `BridgeLockMintResponse`
```ts
interface BridgeLockMintResponse {
  position_id: string;
  bridge_state: string;
  spoke_network: string;
  native_asset: string;
  mirrored_asset: string;
  amount: string;
}
```

---

### `LpPositionsResponse`
```ts
interface LpPositionsResponse {
  pool_pair: string;
  positions: LiquidityPosition[];
  count: number;
}
```
*Top-level `pool_pair` field is included in the actual backend DTO alongside `positions` and `count`.*

---

### `CommitListItem`
```ts
interface CommitListItem {
  CommitID: string;
  PoolPair: string;
  ProviderID: string;
  Side: string;
  Amount: string;
  Status: string;
  OnChainCommitID: string | null;   // *[]byte — base64 or null
  CounterpartCommitID: string | null;
  CreatedAt: string;
  ExpiresAt: string;
}
```
*PascalCase: `domain.PoolCommit` has only `gorm:` struct tags — Go JSON encoder outputs field names verbatim.*

---

## Types to Update

### `CommitRequest` (simplified)
```ts
// Before
interface CommitRequest {
  pool_pair: string;
  provider_id: string;   // ← REMOVE
  side: CommitSide;      // ← REMOVE
  amount: string;
}

// After
interface CommitRequest {
  pool_pair: string;
  amount: string;
}
```

---

### `CommitResult` (add `on_chain_commit_id`)
```ts
// Before
interface CommitResult {
  commit_id: string;
  pool_pair: string;
  side: CommitSide;
  amount: string;
  status: CommitStatus;
  expires_at: string;
  lp_ids: string[] | null;
}

// After
interface CommitResult {
  commit_id: string;
  on_chain_commit_id: string | null;   // ← ADD
  pool_pair: string;
  side: CommitSide;
  amount: string;
  status: CommitStatus;
  expires_at: string;
  lp_ids: string[] | null;
}
```

---

### `CommitStatus` (replace `"MATCHED"` with `"CANCELLED"`)
```ts
// Before
type CommitStatus = "PENDING" | "EXECUTED" | "EXPIRED" | "MATCHED";

// After
type CommitStatus = "PENDING" | "EXECUTED" | "EXPIRED" | "CANCELLED";
```

---

### `LiquidityPosition` (field renames + new fields)
```ts
// Before
interface LiquidityPosition {
  lp_id: string;
  pool_pair: string;
  provider_bank_id: string;
  token_a_amount: string;    // ← RENAME
  token_b_amount: string;    // ← RENAME
  lp_shares: string;
  status: LpStatus;
  added_at: string;
}

// After
interface LiquidityPosition {
  lp_id: string;
  pool_pair: string;
  provider_bank_id: string;
  token_a_contributed: string;   // ← renamed from token_a_amount
  token_b_contributed: string;   // ← renamed from token_b_amount
  lp_shares: string;
  status: LpStatus;
  added_at: string;
  deposit_side: string;          // ← ADD
  commit_id: string;             // ← ADD (omitempty when null)
}
```

---

### `SOVEREIGN_FLOW_PHASE` (add `CANCELLED`)
```ts
// Before
const SOVEREIGN_FLOW_PHASE = {
  LOCK_MINT, BRIDGE_ACTIVE_WAIT, COMMIT_PENDING,
  COMMIT_EXECUTED, POOL_ACTIVE, FAILED, TIMEOUT,
} as const;

// After — add CANCELLED
const SOVEREIGN_FLOW_PHASE = {
  LOCK_MINT, BRIDGE_ACTIVE_WAIT, COMMIT_PENDING,
  COMMIT_EXECUTED, POOL_ACTIVE, CANCELLED, FAILED, TIMEOUT,
} as const;
```

---

### `SOVEREIGN_FLOW_POLICY` (commitTimeoutMs)
```ts
// Before
commitTimeoutMs: 60_000,

// After
commitTimeoutMs: 180_000,
```

---

## Types to Delete

| Type | Reason |
|---|---|
| `AddLiquidityRequest` | Dead code — `/amm/liquidity/add` endpoint removed |
| `MintAndApproveRequest` | Dead code — `/amm/token/mint-and-approve` is HTTP 403 for CB flow |

---

## State Shape Changes (Zustand Store)

| Field | Change |
|---|---|
| `bridgeLockMintResult` | Add: stores `BridgeLockMintResponse` after Step 1 success |
| `lpPositions` | Already exists; now populated by `fetchLpPositions` action |

New store actions:

| Action | Signature | Replaces |
|---|---|---|
| `lockMint` | `(amount: string) => Promise<void>` | `mintAndApprove` |
| `fetchLpPositions` | `(poolPair: string, providerBankId: string) => Promise<void>` | (new) |

Deleted store actions:

| Action | Reason |
|---|---|
| `mintAndApprove` | Blocked endpoint |
| `addLiquidity` | Dead endpoint |

Updated store action signatures:

| Action | Before | After |
|---|---|---|
| `cancelActiveCommit` | `(commitId: string, providerId: string)` | `(commitId: string)` — derives `bankId` from store state |
| `submitCommit` | accepts `CommitRequest` with `provider_id`+`side` | accepts simplified `CommitRequest` (pool_pair + amount only) |
