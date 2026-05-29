# Data Model: Scenario B Frontend Integration

**Phase 1 output** | Feature: `scenario-b-frontend-integration` | Date: 2026-05-07

---

## Entities

### 1. BridgedAssetPosition

Represents a cross-spoke bridging operation (Lock & Mint or Burn & Unlock).

**TypeScript interface** (`frontend/apps/bank/src/types/bridge.types.ts`):
```ts
export const BRIDGE_STATE = {
  LOCKING:                  "LOCKING",
  ACTIVE:                   "ACTIVE",
  BURNED:                   "BURNED",
  UNLOCKED:                 "UNLOCKED",
  RECONCILIATION_REQUIRED:  "RECONCILIATION_REQUIRED",
  FAILED:                   "FAILED",
} as const;
export type BridgeState = (typeof BRIDGE_STATE)[keyof typeof BRIDGE_STATE];

export interface BridgedAssetPosition {
  position_id:        string;
  owner_bank_id:      string;
  spoke_network:      string;
  native_asset:       string;
  mirrored_asset:     string;
  mirrored_amount:    string;        // integer string
  bridge_state:       BridgeState;
  relayer_retries:    number;
  relayer_error_log:  string | null;
  created_at:         string;        // ISO 8601
  updated_at:         string;        // ISO 8601
}

export interface LockMintRequest {
  owner_bank_id:   string;
  spoke_network:   string;
  native_asset:    string;
  mirrored_asset?: string;           // optional
  amount:          string;           // integer string
}

export interface BurnUnlockRequest {
  position_id: string;
}
```

**State machine**:
```
LOCKING → ACTIVE → BURNED → UNLOCKED
    ↓                          ↓
RECONCILIATION_REQUIRED    FAILED
```
Terminal states: `UNLOCKED`, `RECONCILIATION_REQUIRED`, `FAILED`
Non-terminal (polling continues): `LOCKING`, `ACTIVE`, `BURNED`

**Badge color mapping**:
| State | Badge variant |
|-------|--------------|
| `LOCKING` | `default` (neutral grey) |
| `ACTIVE` | `success` (green) |
| `BURNED` | `secondary` (blue/info) |
| `UNLOCKED` | `outline` (muted) |
| `RECONCILIATION_REQUIRED` | `destructive` (red) |
| `FAILED` | `destructive` (red) |

---

### 2. AMMQuote

Represents a price quote before committing to a swap.

**TypeScript interface** (`frontend/apps/bank/src/types/amm-v2.types.ts`):
```ts
export interface AMMQuote {
  pair:              string;
  amount_out:        string;   // requested output amount, integer string
  required_input:    string;   // integer string
  price_impact:      number;   // 0..1 decimal; display as `${(v * 100).toFixed(2)}%`
  quote_timestamp:   string;   // ISO 8601
}
```

**Staleness rule**: Quote is stale when `Date.now() - Date.parse(quote_timestamp) > 10_000` (10 seconds).

---

### 3. SwapOrder

Represents a completed or in-progress AMM swap execution.

**TypeScript interface** (`frontend/apps/bank/src/types/amm-v2.types.ts`):
```ts
export const SWAP_ERROR = {
  SLIPPAGE_LIMIT_EXCEEDED:      "SLIPPAGE_LIMIT_EXCEEDED",
  INSUFFICIENT_POOL_LIQUIDITY:  "INSUFFICIENT_POOL_LIQUIDITY",
  ZK_VALIDATION_FAILED:         "ZK_VALIDATION_FAILED",
  CIRCUIT_BREAKER_HALTED:       "CIRCUIT_BREAKER_HALTED",
} as const;
export type SwapError = (typeof SWAP_ERROR)[keyof typeof SWAP_ERROR];

export interface SwapOrder {
  order_id:      string;
  tx_hash:       string;
  amount_in:     string;   // integer string
  amount_out:    string;   // integer string
  state:         string;
  confirmed_at:  string;   // ISO 8601
}

export interface SwapRequest {
  pair:            string;
  amount_out:      string;   // integer string
  max_amount_in:   string;   // integer string (slippage-adjusted)
  payer_id:        string;
  beneficiary_id:  string;
}

export interface ApproveAmmRequest {
  amount_a: string;  // integer string
  amount_b: string;  // integer string
}
```

**Swap error → user message mapping** (FR-016):
| Error code | User-visible message |
|------------|---------------------|
| `SLIPPAGE_LIMIT_EXCEEDED` | "Market moved — retry with updated quote" |
| `INSUFFICIENT_POOL_LIQUIDITY` | "Insufficient pool liquidity — contact Central Bank" |
| `ZK_VALIDATION_FAILED` | "ZK validation failed — check ZK pointer field" |
| `CIRCUIT_BREAKER_HALTED` | "Swaps temporarily suspended by Central Bank" |

---

### 4. PoolStatus

Real-time state of an AMM pool.

**TypeScript interface** (used in both apps):
```ts
// bank: frontend/apps/bank/src/types/amm-v2.types.ts
// governance: frontend/apps/governance/src/types/liquidity.types.ts
export interface PoolStatus {
  pool_pair:      string;
  reserve_a:      string;   // integer string
  reserve_b:      string;   // integer string
  current_ratio:  string;   // decimal string representation
  imbalance_flag: boolean;
  updated_at:     string;   // ISO 8601
}
```

**Imbalance banner**: When `imbalance_flag = true`, both bank AMM page and governance Liquidity page show a warning banner.

---

### 5. LiquidityPosition

An LP deposit by a central bank.

**TypeScript interface** (`frontend/apps/governance/src/types/liquidity.types.ts`):
```ts
export const LP_STATUS = {
  ACTIVE:    "ACTIVE",
  WITHDRAWN: "WITHDRAWN",
} as const;
export type LpStatus = (typeof LP_STATUS)[keyof typeof LP_STATUS];

export interface LiquidityPosition {
  lp_id:            string;
  pool_pair:        string;
  provider_bank_id: string;
  token_a_amount:   string;   // integer string
  token_b_amount:   string;   // integer string
  lp_shares:        string;   // integer string
  status:           LpStatus;
  added_at:         string;   // ISO 8601
}

export interface AddLiquidityRequest {
  pool_pair:        string;
  token_a_amount:   string;   // integer string
  token_b_amount:   string;   // integer string
  provider_bank_id: string;
}

export interface RemoveLiquidityRequest {
  lp_id:            string;
  pool_pair:        string;
  provider_bank_id: string;
}

export interface MintAndApproveRequest {
  amount_a:   string;         // integer string
  amount_b:   string;         // integer string
  recipient?: string;         // optional
}
```

**Session-only constraint**: `lpPositions[]` in the store is populated exclusively from `addLiquidity()` responses. There is no backend GET endpoint for LP positions; the table resets on page reload.

---

### 6. CircuitBreakerV2Status

Governance state of the AMM (v2 multi-party).

**TypeScript interface** (`frontend/apps/governance/src/types/circuit-breaker-v2.types.ts`):
```ts
export const CB_STATE = {
  LIVE:           "LIVE",
  HALTED:         "HALTED",
  RESUME_PENDING: "RESUME_PENDING",
} as const;
export type CbState = (typeof CB_STATE)[keyof typeof CB_STATE];

export interface CircuitBreakerV2Status {
  pair:              string;
  state:             CbState;
  pause_initiator:   string | null;
  pause_reason:      string | null;
  resume_request_id: string | null;   // present when state = RESUME_PENDING
}

export interface PauseRequest {
  pair:       string;
  bank_id:    string;
  reason_code: string;
  signature:  string;   // base64, default "AA="
}

export interface ProposeResumeRequest {
  pair:      string;
  bank_id:   string;
  signature: string;   // base64, default "AA="
}

export interface ProposeResumeResponse {
  request_id: string;
  state:      CbState;
}

export interface SignResumeRequest {
  pair:       string;
  request_id: string;
  bank_id:    string;
  signature:  string;   // base64, default "AA="
}
```

**State machine**:
```
LIVE ──(pause)──▶ HALTED ──(proposeResume)──▶ RESUME_PENDING
 ▲                                                    │
 └──────────────(signResume, quorum met)──────────────┘
```

**State badge colors**:
| State | Badge variant |
|-------|--------------|
| `LIVE` | `default` (green/success) |
| `HALTED` | `destructive` (red) |
| `RESUME_PENDING` | `secondary` (yellow/warning) |

---

### 7. DisclosureRequest

AML/CFT oversight request.

**TypeScript interface** (`frontend/apps/governance/src/types/oversight.types.ts`):
```ts
export const DISCLOSURE_STATE = {
  PENDING:        "PENDING",
  QUORUM_REACHED: "QUORUM_REACHED",
  EXPIRED:        "EXPIRED",
  REJECTED:       "REJECTED",
} as const;
export type DisclosureState = (typeof DISCLOSURE_STATE)[keyof typeof DISCLOSURE_STATE];

export interface DisclosureRequest {
  request_id:       string;
  tx_ref:           string;
  requestor_id:     string;
  reason_code:      string;
  state:            DisclosureState;
  quorum_reached:   number;
  quorum_required:  number;
  expires_at:       string;   // ISO 8601
  closed_at:        string | null;
}

export interface OpenDisclosureRequest {
  tx_ref:       string;
  requestor_id: string;
  reason_code:  string;
}

export interface SignDisclosureRequest {
  request_id: string;
  signer_id:  string;
}
```

**Quorum display**: `${quorum_reached}/${quorum_required}` (e.g., `"1/2"`)

**State badge colors**:
| State | Badge variant |
|-------|--------------|
| `PENDING` | `default` (neutral) |
| `QUORUM_REACHED` | `default` (green/success) |
| `EXPIRED` | `outline` (muted/warning) |
| `REJECTED` | `destructive` (red) |

---

## Validation Rules

| Entity | Field | Rule |
|--------|-------|------|
| All forms | amount fields | Must be a non-empty string matching `[0-9]+` (integer string, no decimals) |
| All forms | signature fields | Non-empty string; default `"AA="`; any valid base64 accepted |
| LockMintRequest | `mirrored_asset` | Optional; omit if empty |
| MintAndApproveRequest | `recipient` | Optional; omit if empty |
| BurnUnlockRequest | `position_id` | Required; must match existing position |
| RemoveLiquidityRequest | `lp_id` | Required; 404 response shown as "Position not found or already withdrawn" |
| SwapRequest | `max_amount_in` | Required; must be ≥ quote's `required_input` (user responsibility) |

---

## Store Shape Summary

| Store | App | State shape |
|-------|-----|-------------|
| `useBridgeStore` | bank | `{ positions, status, error }` |
| `useAmmV2Store` | bank | `{ quote, quoteTimestamp, swapResult, poolStatus, circuitBreakerState, status, error }` |
| `useLiquidityStore` | governance | `{ poolStatus, lpPositions, status, error }` |
| `useCircuitBreakerV2Store` | governance | `{ cbStatus, resumeRequestId, status, error }` |
| `useOversightStore` | governance | `{ disclosures, currentDisclosure, status, error }` |
