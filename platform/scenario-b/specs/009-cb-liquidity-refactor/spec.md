# Feature Specification: CB Liquidity Frontend Refactor (Scenario B)

**Feature Branch**: `009-cb-liquidity-refactor`
**Created**: 2026-05-26
**Status**: Draft
**Input**: Adapt the governance frontend to the simplified CB sovereign liquidity API introduced by 008-fix-cb-liquidity. The backend now derives `provider_id`, `side`, and `w_token_address` from JWT + server config; the client payloads are smaller, dead endpoints are removed, and field names changed on several response shapes.

---

## Context and Motivation

The `008-fix-cb-liquidity` backend work introduced a "Simplified API" for the Central Bank sovereign liquidity flow in Scenario B. Key consequences for the frontend:

1. **Reduced payloads**: `POST /api/v2/amm/liquidity/commit` now only accepts `pool_pair` and `amount`; `provider_id` and `side` are derived server-side from the JWT and environment config.
2. **Bridge lock-mint is now a real first step**: The governance wizard Step 1 currently calls `/amm/token/mint-and-approve` (which is now HTTP 403 `CROSS_CB_MINT_PROHIBITED`). It must instead call `POST /api/v2/bridge/lock-mint`.
3. **Dead endpoints**: `/amm/liquidity/add` and `/amm/token/mint-and-approve` are no longer functional for CB sovereign flows and must be removed from the frontend.
4. **New endpoint**: `GET /api/v2/amm/liquidity/positions` now exists and should be called in Step 4 of the wizard to show actual LP position details.
5. **Type changes**: `LiquidityPosition` response field names changed from `token_a_amount`/`token_b_amount` to `token_a_contributed`/`token_b_contributed`, and new fields `deposit_side` and `commit_id` were added. `CommitResult` is missing `on_chain_commit_id`. `CommitStatus` is missing `"CANCELLED"`.
6. **Timeout correction**: `SOVEREIGN_FLOW_POLICY.commitTimeoutMs` is 60,000 ms but the guide specifies 180 s.

Without this refactor, the governance app wizard is broken at Step 1 (calls a blocked endpoint), sends over-specified payloads in Step 2, displays stale type structures, and fails to show LP position data in Step 4.

---

## Clarifications

### Session 2026-05-26

- Q: Should `cancelCommit` still send `provider_id` as a query param? → A: No. The backend derives provider identity from the JWT. The `provider_id` query param must be removed from the `DELETE /api/v2/amm/liquidity/commits/:id` call.
- Q: What should happen to the `AddLiquidityRequest` type and `addLiquidity` API call? → A: Both are dead code — the `/amm/liquidity/add` endpoint is no longer active. Remove the type and the API function entirely. Any wizard step referencing them should be removed or replaced.
- Q: Should the phase machine label `LOCK_MINT` be kept or renamed to avoid confusion with the old `mintAndApprove` flow? → A: Keep the `LOCK_MINT` phase label in the state machine — it now correctly maps to `POST /api/v2/bridge/lock-mint`. The previous confusion was that the wizard called `mintAndApprove` while labeling it "Lock-Mint". After this refactor, the label and the action are aligned.
- Q: What field casing does `GET /api/v2/amm/liquidity/commits` return for commit list items? → A: PascalCase. `domain.PoolCommit` has only `gorm:` struct tags and no `json:` tags; Go's JSON encoder outputs struct field names verbatim: `CommitID`, `PoolPair`, `ProviderID`, `Side`, `Amount`, `Status`, `OnChainCommitID`, `CounterpartCommitID`, `CreatedAt`, `ExpiresAt`. `OnChainCommitID` is `*[]byte` and serializes as a base64 string or `null`. Frontend types and accessors for `ListCommits` response items MUST use PascalCase.
- Q: Does `CancelCommit` (`DELETE /api/v2/amm/liquidity/commits/:id`) still require `provider_id` as a query param in the current backend? → A: Yes. The `CancelCommit` handler in `liquidity_handler.go` still reads `c.Query("provider_id")` and returns HTTP 400 if absent. This was NOT updated in 008-fix-cb-liquidity. Since backend changes are out of scope for this refactor, the frontend MUST continue sending `provider_id` as a query parameter. The earlier clarification in this session ("remove provider_id") was incorrect and is superseded by this answer. FR-015 and SC-007 are updated accordingly.
- Q: Does `GET /api/v2/amm/liquidity/positions` auto-filter by the authenticated CB's identity, or does it require an explicit `provider_id` query param? → A: It does NOT auto-filter by JWT. The `ListPositions` handler accepts an optional `provider_id` query param: when provided it calls `FindByProviderAndPoolPair`; otherwise it calls `FindByPoolPair` and returns all positions for that pool. For wizard Step 4, the frontend MUST pass the authenticated CB's `bankId` as `provider_id` to show only that CB's positions. The response uses snake_case DTO fields (`lp_id`, `provider_bank_id`, `token_a_contributed`, `token_b_contributed`, etc.) and includes a top-level `pool_pair` field alongside `positions` and `count`.

---

## User Scenarios

### User Story 1 — CB Operator: Complete Sovereign Liquidity Provision (Priority: P1)

As a Central Bank operator using the governance app, I want to complete the sovereign liquidity provision wizard from start to finish using the new simplified API, so that the wizard reflects the actual backend contract and I am not blocked by deprecated endpoints.

**Why this priority**: The entire CB sovereign flow is broken at Step 1 today due to the blocked endpoint. This is the most critical fix.

**Independent Validation**: A CB operator can open the governance wizard, complete all four steps (lock-mint → bridge active → commit → positions), and arrive at a success state showing LP position data without encountering HTTP 403 errors or submitting stale payload fields.

**Acceptance Scenarios**:

1. **Given** a CB operator starts Step 1 of the wizard, **When** they enter an amount and submit, **Then** the app calls `POST /api/v2/bridge/lock-mint` with `{ "amount": "<value>" }` only — no `recipient`, `token`, or other fields — and transitions to bridge-active polling.
2. **Given** Step 1 completes successfully, **When** the bridge position reaches `ACTIVE`, **Then** the wizard advances to Step 2 with no `provider_id` or `side` inputs shown.
3. **Given** a CB operator is on Step 2, **When** they select a pool pair and enter an amount and submit, **Then** the app calls `POST /api/v2/amm/liquidity/commit` with only `{ "pool_pair": "W-BRL-ARS", "amount": "<value>" }` — no `provider_id` or `side` in the payload.
4. **Given** Step 2 completes and the commit is accepted, **When** the monitor polls for status, **Then** the commit status transitions from `PENDING` → `EXECUTED` (or shows `CANCELLED` if the commit was cancelled), and the displayed `commit_id` includes both `commit_id` and `on_chain_commit_id`.
5. **Given** all steps succeed, **When** the wizard reaches Step 4, **Then** the app calls `GET /api/v2/amm/liquidity/positions?pool_pair=W-BRL-ARS` and displays the returned position details including `token_a_contributed`, `token_b_contributed`, `deposit_side`, and `commit_id`.

---

### User Story 2 — CB Operator: Cancel an Active Commit (Priority: P2)

As a CB operator monitoring a pending commit, I want to cancel it without providing my provider ID, since the backend now derives it from my JWT, so that the cancel action uses the current API contract.

**Why this priority**: Cancel is a secondary flow but currently sends a stale `provider_id` param that may cause unexpected behavior.

**Independent Validation**: A CB operator can cancel an active commit using the cancel button, and the app sends `DELETE /api/v2/amm/liquidity/commits/:id?provider_id=<bankId>` with the authenticated bank's ID — no other query parameters.

**Acceptance Scenarios**:

1. **Given** a CB operator has an active commit in monitoring state, **When** they click Cancel, **Then** the app calls `DELETE /api/v2/amm/liquidity/commits/:id?provider_id=<bankId>` with the authenticated CB's bank ID, and on success the commit status updates to `CANCELLED` in the UI.
2. **Given** the cancel call succeeds, **When** the status is updated, **Then** `CANCELLED` is a recognized and displayed status (not an unhandled enum value).

---

### User Story 3 — Developer: Dead Code Removal (Priority: P2)

As a developer maintaining the governance app, I want dead types and API calls (for `addLiquidity`, `mintAndApprove`) removed from the codebase, so that the type system accurately reflects the supported API surface and future maintainers are not confused.

**Why this priority**: Dead code increases cognitive load and risks accidental regression where removed endpoints are re-introduced by mistake.

**Independent Validation**: After the refactor, a search of the codebase finds no references to `addLiquidity`, `mintAndApprove`, `AddLiquidityRequest`, or `MintAndApproveRequest` in the governance app's API, types, or store files.

**Acceptance Scenarios**:

1. **Given** the refactor is complete, **When** a developer searches the governance app source for `mintAndApprove`, **Then** there are zero results in `types/`, `services/api/`, and `features/liquidity/` directories.
2. **Given** the refactor is complete, **When** a developer searches for `addLiquidity` or `AddLiquidityRequest`, **Then** there are zero results in the same directories.
3. **Given** the refactor is complete, **When** TypeScript compilation runs on the governance app, **Then** there are no type errors in `liquidity.types.ts`, `liquidity.api.ts`, `liquidity.store.ts`, or `CooperativeLiquidityWizard.tsx`.

---

### Edge Cases

- `GET /api/v2/amm/liquidity/positions` returns an empty `positions` array — wizard Step 4 should display a "no positions yet" message rather than crash.
- Backend returns `"CANCELLED"` as commit status — must be handled gracefully as a terminal state (do not keep polling).
- Bridge lock-mint Step 1 fails with a non-200 response — wizard must display error and allow retry without losing the amount field value.
- `on_chain_commit_id` is null or empty in the commit response — UI must handle this gracefully (display as pending/unavailable rather than crash).
- Commit timeout (180 s) elapses with status still `PENDING` — show timeout warning and allow operator to navigate away without data loss.

---

## Requirements

### Functional Requirements

#### Types (`liquidity.types.ts`)

- **FR-000**: A new `CommitListItem` type MUST be added with PascalCase fields matching the `GET /api/v2/amm/liquidity/commits` response: `{ CommitID: string; PoolPair: string; ProviderID: string; Side: string; Amount: string; Status: string; OnChainCommitID: string | null; CounterpartCommitID: string | null; CreatedAt: string; ExpiresAt: string }`. This PascalCase shape is dictated by `domain.PoolCommit` having no `json:` struct tags.
- **FR-001**: `CommitRequest` MUST be reduced to `{ pool_pair: string; amount: string }`. Fields `provider_id`, `side`, and any other server-derived fields MUST be removed.
- **FR-002**: `CommitStatus` union type MUST include `"CANCELLED"`. Current definition is missing this value.
- **FR-003**: `CommitResult` MUST include an `on_chain_commit_id: string` field alongside the existing `commit_id`.
- **FR-004**: `LiquidityPosition` fields `token_a_amount` and `token_b_amount` MUST be renamed to `token_a_contributed` and `token_b_contributed` respectively. New fields `deposit_side: string` and `commit_id: string` MUST be added.
- **FR-005**: `AddLiquidityRequest` type MUST be deleted entirely.
- **FR-006**: `MintAndApproveRequest` type MUST be deleted entirely.
- **FR-007**: A new `BridgeLockMintRequest` type MUST be added: `{ amount: string }`.
- **FR-008**: A new `BridgeLockMintResponse` type MUST be added with fields: `position_id: string`, `bridge_state: string`, `spoke_network: string`, `native_asset: string`, `mirrored_asset: string`, `amount: string`.
- **FR-009**: A new `LpPositionsResponse` type MUST be added: `{ pool_pair: string; positions: LiquidityPosition[]; count: number }`. The top-level `pool_pair` field is included in the actual backend DTO response.
- **FR-010**: `SOVEREIGN_FLOW_POLICY.commitTimeoutMs` MUST be changed from `60000` to `180000` (180 seconds).

#### API Client (`liquidity.api.ts`)

- **FR-011**: A new `lockMint(req: BridgeLockMintRequest): Promise<BridgeLockMintResponse>` function MUST be added, calling `POST /api/v2/bridge/lock-mint` with `{ amount: req.amount }` only.
- **FR-012**: The existing `mintAndApprove` function MUST be deleted entirely.
- **FR-013**: The existing `addLiquidity` function MUST be deleted entirely.
- **FR-014**: `commitLiquidity` MUST send only `{ pool_pair, amount }` — remove `provider_id` and `side` from the request payload. The function signature MUST accept `CommitRequest` which now only has those two fields.
- **FR-015**: `cancelCommit` MUST send `provider_id` as a query parameter. The backend `CancelCommit` handler (`liquidity_handler.go`) still reads `c.Query("provider_id")` and returns HTTP 400 if absent (this handler was NOT updated in 008-fix-cb-liquidity). The call MUST be `DELETE /api/v2/amm/liquidity/commits/:id?provider_id=<bankId>`. The `bankId` value is the authenticated CB's bank identifier, available from the store's auth state or a dedicated `currentBankId` state field.
- **FR-016**: A new `listPositions(poolPair: string, providerBankId: string): Promise<LpPositionsResponse>` function MUST be added, calling `GET /api/v2/amm/liquidity/positions?pool_pair=<poolPair>&provider_id=<providerBankId>`. The `provider_id` param is required to filter results to the authenticated CB; omitting it returns all positions for the pool.

#### Store (`liquidity.store.ts`)

- **FR-017**: The `mintAndApprove` store action MUST be deleted.
- **FR-018**: The `addLiquidity` store action MUST be deleted.
- **FR-019**: A new `lockMint(amount: string): Promise<void>` store action MUST be added. It calls `api.lockMint({ amount })`, stores the response in store state (at minimum `bridgePositionId` and `bridgeState`), and transitions `sovereignPhase` to `BRIDGE_ACTIVE` on success.
- **FR-020**: `submitCommit` MUST send only `{ pool_pair, amount }`. Any reference to `provider_id` or `side` in the action payload MUST be removed.
- **FR-021**: `cancelActiveCommit` MUST remove the `providerId` parameter from its public signature. The action signature MUST be `cancelActiveCommit(commitId: string)` — no second argument. Internally, the store derives the `provider_id` value from store state (e.g., `get().currentBankId` or equivalent auth state) and passes it to `api.cancelCommit(commitId, bankId)` as required by the backend.
- **FR-022**: A new `fetchLpPositions(poolPair: string, providerBankId: string): Promise<void>` store action MUST be added. It calls `api.listPositions(poolPair, providerBankId)` and stores the result in the existing `lpPositions` state field.
- **FR-023**: The phase machine `sovereignPhase` transitions MUST follow: `IDLE` → `LOCK_MINT` (user initiates Step 1) → `BRIDGE_ACTIVE` (lock-mint response received, polling confirms `bridge_state=ACTIVE`) → `COMMIT_PENDING` (commit submitted) → `COMMIT_EXECUTED` (polling confirms `status=EXECUTED`) → `POOL_ACTIVE` (Step 4 validation complete). `CANCELLED` MUST be a reachable terminal state from `COMMIT_PENDING`.

#### Wizard UI (`CooperativeLiquidityWizard.tsx`)

- **FR-024**: Step 1 MUST call the `lockMint` store action (not `mintAndApprove`). The form MUST send only the `amount` field.
- **FR-025**: Step 2 form MUST remove the "Provider ID" input and the "Commit Side" radio button group. Only `pool_pair` (selector or read-only display) and `amount` input MUST remain.
- **FR-026**: The default value for `commitPoolPair` MUST be changed from `"BRL-USD"` to `"W-BRL-ARS"`.
- **FR-027**: The `toActiveCommitFromPending` function (or equivalent) MUST use `pool_pair: "W-BRL-ARS"` (not `"BRL-USD"`).
- **FR-028**: Step 3 monitor MUST display latency tracking: warn the operator after 30 s of polling with status still `PENDING`, and show a timeout message after 180 s (aligned with FR-010).
- **FR-029**: Step 4 MUST call `fetchLpPositions("W-BRL-ARS", bankId)` on mount, where `bankId` is the authenticated CB's bank identifier from store state. Display the returned position details. At minimum: `token_a_contributed`, `token_b_contributed`, `deposit_side`, `commit_id`. If `positions` is empty, display a "No positions found" message.
- **FR-030**: `cancelActiveCommit` calls in the wizard MUST pass only `commitId` — the second `monitoredProviderId` argument MUST be removed.
- **FR-031**: The wizard MUST display `on_chain_commit_id` (from `CommitResult`) in the Step 2 success state and Step 3 monitor alongside `commit_id`. If `on_chain_commit_id` is empty/null, display it as "pending on-chain confirmation".

### Non-Functional Requirements

- **NFR-001**: TypeScript compilation MUST succeed with zero errors in the four changed files after the refactor.
- **NFR-002**: No new browser console errors at runtime for the wizard happy-path flow.
- **NFR-003**: Removal of `mintAndApprove` and `addLiquidity` MUST leave no dangling imports or unreferenced types across the governance app.
- **NFR-004**: The change in commit timeout (60 s → 180 s) MUST NOT break existing polling logic — only the timeout threshold changes.

---

## Key Entities

- **BridgeLockMintRequest**: Simplified request to `POST /api/v2/bridge/lock-mint` — contains only `amount: string`. All other fields (`recipient`, `token`, `network`) are derived server-side.
- **BridgeLockMintResponse**: Response from the lock-mint call — `position_id`, `bridge_state`, `spoke_network`, `native_asset`, `mirrored_asset`, `amount`.
- **CommitRequest (simplified)**: Request to `POST /api/v2/amm/liquidity/commit` — only `pool_pair` and `amount`. `provider_id` and `side` are no longer client-supplied.
- **CommitResult (updated)**: Now includes `on_chain_commit_id` alongside `commit_id`, `status`, `side`, `pool_pair`, `expires_at`.
- **CommitStatus (updated)**: Enum including `"PENDING"`, `"EXECUTED"`, `"CANCELLED"`.
- **LiquidityPosition (updated)**: `token_a_contributed` / `token_b_contributed` (renamed from `token_a_amount`/`token_b_amount`), plus new fields `deposit_side` and `commit_id`.
- **LpPositionsResponse**: Response shape from `GET /api/v2/amm/liquidity/positions` — `{ pool_pair: string; positions: LiquidityPosition[]; count: number }`. All fields are snake_case (handler uses an explicit DTO with `json:` tags).
- **SovereignPhase**: Phase machine state in the store — `IDLE | LOCK_MINT | BRIDGE_ACTIVE | COMMIT_PENDING | COMMIT_EXECUTED | POOL_ACTIVE | CANCELLED`.

---

## API Contract Reference

### POST /api/v2/bridge/lock-mint

**Request** (simplified):
```json
{ "amount": "100000000000000000000000" }
```

**Response**:
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

*All fields other than `amount` are derived by the backend from the JWT and server environment configuration.*

---

### POST /api/v2/amm/liquidity/commit

**Request** (simplified):
```json
{ "pool_pair": "W-BRL-ARS", "amount": "100000000000000000000000" }
```
*Fields `provider_id`, `side`, and `w_token_address` are no longer accepted from the client.*

**Response**:
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

---

### GET /api/v2/amm/liquidity/commits?pool_pair=W-BRL-ARS&status=EXECUTED

**Response** (note: backend may return capitalized field names — verify against actual handler):
```json
{
  "commits": [
    {
      "CommitID": "commit-abc",
      "OnChainCommitID": "0xdeadbeef...",
      "Status": "EXECUTED",
      "Side": "A",
      "ProviderID": "bank-central-a",
      "Amount": "100000000000000000000000",
      "CreatedAt": "2026-05-26T12:00:00Z",
      "ExecutedAt": "2026-05-26T12:02:30Z"
    }
  ]
}
```

*Implementer note*: **PascalCase confirmed.** `domain.PoolCommit` has only `gorm:` struct tags and no `json:` tags; Go's JSON encoder outputs struct field names verbatim. Frontend MUST use PascalCase accessors (`commit.CommitID`, `commit.OnChainCommitID`, `commit.ProviderID`, etc.) or normalize via the `CommitListItem` type defined in FR-000. `OnChainCommitID` is `*[]byte` — null when not set, base64-encoded bytes32 when present.

---

### GET /api/v2/amm/liquidity/positions?pool_pair=W-BRL-ARS&provider_id=bank-central-a

**Query parameters**: `pool_pair` (required), `provider_id` (optional — filters to a specific provider). Without `provider_id`, all positions for the pool are returned. For wizard Step 4, always pass `provider_id` to filter to the authenticated CB's positions.

**Response** (snake_case confirmed — handler maps `domain.LiquidityPosition` to an explicit DTO with `json:` tags):
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

*Note*: The top-level response includes `pool_pair` alongside `positions` and `count`. `commit_id` is omitted from the DTO when null (`omitempty`).

---

### DELETE /api/v2/amm/liquidity/commits/:id

**Request**: Query parameter `provider_id=<bankId>` is **required** (backend `CancelCommit` handler was not updated in 008 — still reads `c.Query("provider_id")` and returns HTTP 400 if absent). No body. No other query parameters.

**Response**: `200 OK` with `{ "message": "commit cancelled", "commit_id": "<id>" }` on success.

---

## Before / After Summary

| Surface | Before (broken) | After (correct) |
|---|---|---|
| `CommitRequest` type | `{ pool_pair, provider_id, side, amount }` | `{ pool_pair, amount }` |
| `CommitStatus` | `"PENDING" \| "MATCHED" \| "EXECUTED"` | `"PENDING" \| "EXECUTED" \| "CANCELLED"` |
| `CommitResult` | no `on_chain_commit_id` field | includes `on_chain_commit_id: string` |
| `LiquidityPosition` | `token_a_amount`, `token_b_amount` | `token_a_contributed`, `token_b_contributed`, `deposit_side`, `commit_id` |
| `AddLiquidityRequest` | exists | deleted |
| `MintAndApproveRequest` | exists | deleted |
| `BridgeLockMintRequest` | missing | `{ amount: string }` |
| `BridgeLockMintResponse` | missing | added (see API ref above) |
| `LpPositionsResponse` | missing | `{ positions: LiquidityPosition[]; count: number }` |
| `commitTimeoutMs` | 60,000 ms | 180,000 ms |
| `lockMint` API fn | missing | `POST /api/v2/bridge/lock-mint` with `{ amount }` |
| `mintAndApprove` API fn | calls blocked endpoint | deleted |
| `addLiquidity` API fn | calls dead endpoint | deleted |
| `commitLiquidity` payload | sends `provider_id`, `side` | sends only `pool_pair`, `amount` |
| `cancelCommit` query param | sends `provider_id` | still sends `provider_id=<bankId>` (backend requires it; derived from store state, not passed by caller) |
| `listPositions` API fn | missing | `GET /api/v2/amm/liquidity/positions?pool_pair=&provider_id=` |
| Store `lockMint` action | missing | added, transitions phase to `BRIDGE_ACTIVE` |
| Store `mintAndApprove` action | exists (blocked) | deleted |
| Store `addLiquidity` action | exists (dead) | deleted |
| Store `submitCommit` | sends `provider_id`, `side` | sends only `pool_pair`, `amount` |
| Store `cancelActiveCommit` | `(commitId, providerId)` | `(commitId)` |
| Store `fetchLpPositions` | missing | added, stores into `lpPositions` |
| Wizard Step 1 | calls `mintAndApprove` (HTTP 403) | calls `lockMint` |
| Wizard Step 2 | shows Provider ID + Commit Side fields | fields removed |
| Wizard `commitPoolPair` default | `"BRL-USD"` | `"W-BRL-ARS"` |
| Wizard Step 3 latency | no timeout warning | warn at 30 s, timeout at 180 s |
| Wizard Step 4 positions | no API call | calls `fetchLpPositions("W-BRL-ARS", bankId)` |
| Wizard cancel call | `cancelActiveCommit(id, providerId)` | `cancelActiveCommit(id)` — store internally derives `bankId` from state and sends `?provider_id=<bankId>` (backend still requires it) |

---

## Success Criteria

### Measurable Outcomes

- **SC-001**: After the refactor, `POST /api/v2/bridge/lock-mint` is called with a payload containing only `amount`. Verified by inspecting network traffic during wizard Step 1.
- **SC-002**: After the refactor, `POST /api/v2/amm/liquidity/commit` payload contains exactly two fields: `pool_pair` and `amount`. No `provider_id` or `side` present. Verified by network inspection.
- **SC-003**: The wizard Step 3 monitor displays a visible latency warning when commit status remains `PENDING` beyond 30 seconds, and a timeout message at 180 seconds. Verified by UI testing with a slow commit scenario.
- **SC-004**: Wizard Step 4 renders LP position data (`token_a_contributed`, `token_b_contributed`, `deposit_side`, `commit_id`) returned from `GET /api/v2/amm/liquidity/positions`. Verified by end-to-end test.
- **SC-005**: Zero references to `mintAndApprove`, `addLiquidity`, `AddLiquidityRequest`, or `MintAndApproveRequest` remain in `liquidity.types.ts`, `liquidity.api.ts`, `liquidity.store.ts`, or `CooperativeLiquidityWizard.tsx` after the refactor. Verified by `grep` or TypeScript compilation.
- **SC-006**: TypeScript compilation of the governance app produces zero errors after the refactor.
- **SC-007**: `DELETE /api/v2/amm/liquidity/commits/:id?provider_id=<bankId>` is called with the authenticated CB's bank ID as the `provider_id` query parameter when cancel is triggered. No other query parameters are sent. Verified by network inspection.
- **SC-008**: The `CANCELLED` status is displayed correctly in the commit monitor UI when returned by the backend — no "unknown status" fallback message shown.
- **SC-009**: `on_chain_commit_id` is displayed in the wizard Step 2 success panel and Step 3 monitor. When it is empty/null, the UI shows "pending on-chain confirmation" rather than a blank or crashed render.

---

## Assumptions

- The backend API contracts described in this spec (008-fix-cb-liquidity) are stable and deployed in the target environment. No backend changes are required to deliver this spec. Exception noted: the `CancelCommit` handler still requires `provider_id` as a query param (not updated in 008). The frontend works around this by deriving the value from store state rather than requiring the caller to pass it explicitly.
- The `GET /api/v2/amm/liquidity/commits` response **does** use PascalCase field names: `CommitID`, `PoolPair`, `ProviderID`, `Side`, `Amount`, `Status`, `OnChainCommitID`, `CounterpartCommitID`, `CreatedAt`, `ExpiresAt`. This is confirmed by `domain.PoolCommit` having only `gorm:` struct tags (no `json:` tags). Frontend code MUST use PascalCase accessors or define a `CommitListItem` type (FR-000) and normalize at the API client layer. `OnChainCommitID` is `*[]byte` — serialized as a base64 string or `null`.
- The `W-BRL-ARS` pool pair string is the correct canonical value for the deployed Scenario B environment. It is used as the default everywhere in the wizard.
- No UI framework or styling changes are required beyond the specific field additions/removals described in this spec.
- The governance app is the only frontend application affected. The bank app (swap/transfer flow) is out of scope.
- The Zustand store structure (state shape, action naming conventions) follows the existing pattern in `liquidity.store.ts` — no store architecture changes are needed.
- Bridge polling behavior (interval, retry logic) from spec-008 / FR-003 of spec-008 is already in place and does not need to change — only the timeout threshold (FR-010) is updated.

---

## Out of Scope

- Backend changes: no modifications to `api-gateway`, `compliance`, or any other service.
- Solidity contract changes.
- Bank app (`frontend/apps/bank`) changes.
- Redesign of the wizard layout beyond the specific field removals and additions described.
- Changes to the Scenario A gating behavior (already implemented in spec-008).
- Authentication or JWT handling changes.
- New error codes beyond those already handled by spec-008.
- Performance optimization of polling logic.
