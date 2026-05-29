# Tasks: CB Liquidity Frontend Refactor (Scenario B)

**Feature**: `009-cb-liquidity-refactor`
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md) | **Data Model**: [data-model.md](data-model.md)
**Contracts**: [contracts/frontend-cb-liquidity-refactor.md](contracts/frontend-cb-liquidity-refactor.md)
**Generated**: 2026-05-26
**Total Tasks**: 52 | **M1**: 13 | **M2**: 7 | **M3**: 10 | **M4**: 16 | **Integration**: 6

---

## Dependency Graph (Milestone Completion Order)

```
M1 (types) → M2 (api) → M3 (store) → M4 (wizard) → Integration
```

Each milestone MUST be TypeScript-compilable before the next begins. No circular deps.
All four changed files are in `frontend/apps/governance/src/`.

**Parallelism opportunities**:
- Within M1: T008–T011 (adding independent new types) can be done in parallel
- Within M2: T015 ∥ T016 ∥ T017 ∥ T019 (independent additions/deletions in different function scopes)
- Within M3: T022 ∥ T023 (independent dead-code removals)
- Within M4: T031–T034 (store selector bindings at top of component)

**MVP Scope**: M1 → M2 → M3 → M4 = full fix. No partial delivery possible (types → api → store → UI is a hard chain).

---

## Milestone 1: Types Layer (`liquidity.types.ts`)

**Goal**: All type definitions match the `008-fix-cb-liquidity` backend API contracts.
**Independent Validation**: `tsc --noEmit` passes for `liquidity.types.ts` in isolation; `grep -r "AddLiquidityRequest\|MintAndApproveRequest" frontend/apps/governance/src/types/` returns zero hits.

---

- [ ] T001 [US3] Remove `AddLiquidityRequest` interface in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.1 | **FR**: FR-005
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: Delete the entire `AddLiquidityRequest` interface block (type declaration + any inline comments). This type is dead code — `/amm/liquidity/add` is no longer active.
  - **Acceptance**: `grep "AddLiquidityRequest" frontend/apps/governance/src/types/liquidity.types.ts` returns zero results; file still compiles.

---

- [ ] T002 [US3] Remove `MintAndApproveRequest` interface in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.2 | **FR**: FR-006
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: Delete the entire `MintAndApproveRequest` interface block. This type is dead code — `/amm/token/mint-and-approve` returns HTTP 403 for CB flows.
  - **Acceptance**: `grep "MintAndApproveRequest" frontend/apps/governance/src/types/liquidity.types.ts` returns zero results; file still compiles.

---

- [ ] T003 [US1] Simplify `CommitRequest` to `{ pool_pair: string; amount: string }` in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.3 | **FR**: FR-001
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: Remove `provider_id` and `side` fields from the `CommitRequest` interface. The backend derives both values from the JWT and environment config; they must not be sent by the client.
  - **Acceptance**: `CommitRequest` interface has exactly two fields: `pool_pair: string` and `amount: string`. TypeScript compilation enforces this change downstream in `liquidity.api.ts` and `liquidity.store.ts`.

---

- [ ] T004 [US1] Add `on_chain_commit_id: string | null` to `CommitResult` in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.4 | **FR**: FR-003
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: Add the `on_chain_commit_id: string | null` field to the `CommitResult` interface. The Go `*[]byte` field serializes as a base64 string or JSON `null`; the frontend treats it as `string | null`.
  - **Acceptance**: `CommitResult` interface includes `on_chain_commit_id: string | null`; TypeScript compiles with no errors.

---

- [ ] T005 [US2] Replace `"MATCHED"` with `"CANCELLED"` in `CommitStatus` union in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.5 | **FR**: FR-002
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: The `CommitStatus` union currently includes `"MATCHED"` which does not exist in the backend. Replace it with `"CANCELLED"`. The final union must be `"PENDING" | "EXECUTED" | "CANCELLED"`.
  - **Acceptance**: `CommitStatus` union is exactly `"PENDING" | "EXECUTED" | "CANCELLED"`. `grep "MATCHED" frontend/apps/governance/src/types/liquidity.types.ts` returns zero results.

---

- [ ] T006 [US1] Rename `LiquidityPosition` fields `token_a_amount` → `token_a_contributed` and `token_b_amount` → `token_b_contributed` in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.6 | **FR**: FR-004
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: The backend DTO (`ListPositionsDTO`) uses `token_a_contributed` and `token_b_contributed` (snake_case, with `json:` tags). Rename both fields in `LiquidityPosition`. This is a breaking rename — all usages in store and wizard must be updated in M3/M4.
  - **Acceptance**: `LiquidityPosition` has `token_a_contributed: string` and `token_b_contributed: string`; `grep "token_a_amount\|token_b_amount" frontend/apps/governance/src/types/liquidity.types.ts` returns zero results.

---

- [ ] T007 [US1] Add `deposit_side: string` and `commit_id: string` to `LiquidityPosition` in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.7 | **FR**: FR-004
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: The backend `ListPositionsDTO` includes `deposit_side` and `commit_id` fields in each position entry. Add both to `LiquidityPosition` as required `string` fields.
  - **Acceptance**: `LiquidityPosition` has `deposit_side: string` and `commit_id: string` fields; TypeScript compiles.

---

- [ ] T008 [P] [US1] Add `BridgeLockMintRequest` type in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.8 | **FR**: FR-007
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: Add `export interface BridgeLockMintRequest { amount: string }`. This is the complete request body for `POST /api/v2/bridge/lock-mint` — all other fields are derived server-side from JWT and config.
  - **Acceptance**: `BridgeLockMintRequest` is exported from the file; `amount` is the only field; TypeScript compiles.

---

- [ ] T009 [P] [US1] Add `BridgeLockMintResponse` type with 6 fields in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.9 | **FR**: FR-008
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: Add `export interface BridgeLockMintResponse` with fields: `position_id: string`, `bridge_state: string`, `spoke_network: string`, `native_asset: string`, `mirrored_asset: string`, `amount: string`. Matches the `POST /api/v2/bridge/lock-mint` response JSON shape.
  - **Acceptance**: All 6 fields present; interface is exported; TypeScript compiles.

---

- [X] T010 [P] [US1] Add `LpPositionsResponse` type in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.10 | **FR**: FR-009
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: Add `export interface LpPositionsResponse { pool_pair: string; positions: LiquidityPosition[]; count: number }`. Matches the `GET /api/v2/amm/liquidity/positions` response DTO (all fields snake_case with explicit `json:` tags in the backend handler).
  - **Acceptance**: `LpPositionsResponse` is exported with exactly 3 fields; `positions` is typed as `LiquidityPosition[]`; TypeScript compiles.

---

- [X] T011 [P] [US1] Add `CommitListItem` type with PascalCase fields in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.11 | **FR**: FR-000
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: Add `export interface CommitListItem` with PascalCase fields: `CommitID: string`, `PoolPair: string`, `ProviderID: string`, `Side: string`, `Amount: string`, `Status: string`, `OnChainCommitID: string | null`, `CounterpartCommitID: string | null`, `CreatedAt: string`, `ExpiresAt: string`. The PascalCase shape is mandatory — `domain.PoolCommit` has no `json:` struct tags and Go's encoder outputs verbatim field names.
  - **Acceptance**: All 10 fields present with PascalCase names; `OnChainCommitID` and `CounterpartCommitID` are `string | null`; interface is exported.

---

- [X] T012 [US2] Add `CANCELLED` to `SOVEREIGN_FLOW_PHASE` constant and `SovereignFlowPhase` type in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.12
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: Add `CANCELLED: "CANCELLED"` to the `SOVEREIGN_FLOW_PHASE` const object and `"CANCELLED"` to the `SovereignFlowPhase` union type. `CANCELLED` is a reachable terminal state from `COMMIT_PENDING` (operator-initiated cancellation); it must not be modeled as `FAILED` or `TIMEOUT`.
  - **Acceptance**: `SOVEREIGN_FLOW_PHASE.CANCELLED === "CANCELLED"`; `SovereignFlowPhase` union includes `"CANCELLED"`; TypeScript compiles without widening errors.

---

- [X] T013 [US1] Change `SOVEREIGN_FLOW_POLICY.commitTimeoutMs` from `60_000` to `180_000` in `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Milestone**: M1.13 | **FR**: FR-010
  - **File**: `frontend/apps/governance/src/types/liquidity.types.ts`
  - **Description**: The commit polling timeout is specified as 180 s in the quickstart guide; the current constant of 60,000 ms (60 s) is wrong. Change to `180_000`. This constant propagates to the store polling loop via import — only the threshold value changes, not the polling logic.
  - **Acceptance**: `SOVEREIGN_FLOW_POLICY.commitTimeoutMs === 180_000`; no other timeout constants changed; NFR-004 satisfied.

---

**M1 Checkpoint**: `cd frontend && npm run type-check --workspace=governance` passes with zero errors on the types file. `grep -r "AddLiquidityRequest\|MintAndApproveRequest" frontend/apps/governance/src/types/` → zero results.

---

## Milestone 2: API Client Layer (`liquidity.api.ts`)

**Goal**: API client exposes only live endpoints with correct simplified payloads; dead functions removed.
**Independent Validation**: `tsc --noEmit` passes; `grep -r "mintAndApprove\|addLiquidity\|AddLiquidityRequest\|MintAndApproveRequest" frontend/apps/governance/src/services/api/` → zero results.

---

- [X] T014 [US3] Update imports at top of `liquidity.api.ts` — remove deleted types, add new types in `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Milestone**: M2.7
  - **File**: `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Description**: Remove `AddLiquidityRequest` and `MintAndApproveRequest` from the import statement. Add `BridgeLockMintRequest`, `BridgeLockMintResponse`, and `LpPositionsResponse` to the import from `../../types/liquidity.types`. This task must be done first to avoid TypeScript import errors blocking the rest of M2.
  - **Acceptance**: Import block references exactly the types needed by remaining functions; no `AddLiquidityRequest` or `MintAndApproveRequest` in imports; TypeScript compiles.

---

- [X] T015 [P] [US1] Add `lockMint(req: BridgeLockMintRequest): Promise<BridgeLockMintResponse>` API function in `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Milestone**: M2.1 | **FR**: FR-011
  - **File**: `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Description**: Add a new exported function that calls `POST /api/v2/bridge/lock-mint` with body `{ amount: req.amount }` only. No other fields. Use the existing Axios client pattern in the file. Returns `Promise<BridgeLockMintResponse>`.
  - **Acceptance**: Network payload for `lockMint` is `{ "amount": "<value>" }` only — no `recipient`, `token`, `network`, or other fields. Function compiles with correct types.

---

- [X] T016 [P] [US3] Remove `mintAndApprove` function from `liquidity.api.ts`
  - **Milestone**: M2.2 | **FR**: FR-012
  - **File**: `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Description**: Delete the entire `mintAndApprove` function. This function calls `/amm/token/mint-and-approve` which returns HTTP 403 `CROSS_CB_MINT_PROHIBITED` for CB sovereign flows.
  - **Acceptance**: `grep "mintAndApprove" frontend/apps/governance/src/services/api/liquidity.api.ts` → zero results; TypeScript compiles.

---

- [X] T017 [P] [US3] Remove `addLiquidity` function from `liquidity.api.ts`
  - **Milestone**: M2.3 | **FR**: FR-013
  - **File**: `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Description**: Delete the entire `addLiquidity` function. This function calls `/amm/liquidity/add` which is no longer active.
  - **Acceptance**: `grep "addLiquidity" frontend/apps/governance/src/services/api/liquidity.api.ts` → zero results; TypeScript compiles.

---

- [X] T018 [US1] Simplify `commitLiquidity` to send only `{ pool_pair, amount }` in `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Milestone**: M2.4 | **FR**: FR-014
  - **File**: `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Description**: Update `commitLiquidity` to accept the simplified `CommitRequest` (which now only has `pool_pair` and `amount` after M1.3). Ensure the Axios POST body is `{ pool_pair: req.pool_pair, amount: req.amount }` — no explicit spreading of `provider_id` or `side`. The TypeScript compiler enforces this after M1.3.
  - **Acceptance**: Network payload for `commitLiquidity` is `{ "pool_pair": "W-BRL-ARS", "amount": "<value>" }` only. Function signature accepts `CommitRequest`. No `provider_id` or `side` in the sent body.

---

- [X] T019 [P] [US1] Add `listPositions(poolPair: string, providerBankId: string): Promise<LpPositionsResponse>` API function in `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Milestone**: M2.5 | **FR**: FR-016
  - **File**: `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Description**: Add exported function calling `GET /api/v2/amm/liquidity/positions` with query params `pool_pair=<poolPair>` and `provider_id=<providerBankId>`. Both params must always be present. Returns `Promise<LpPositionsResponse>`.
  - **Acceptance**: Request URL is `/api/v2/amm/liquidity/positions?pool_pair=<poolPair>&provider_id=<providerBankId>`; both query params always sent; return type is `LpPositionsResponse`.

---

- [X] T020 [US2] Confirm `cancelCommit(commitId, providerId)` still passes `provider_id` as query param in `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Milestone**: M2.6 | **FR**: FR-015
  - **File**: `frontend/apps/governance/src/services/api/liquidity.api.ts`
  - **Description**: The `cancelCommit` function signature and implementation must NOT be changed from its current two-parameter form `(commitId: string, providerId: string)`. The backend `liquidity_handler.go` still reads `c.Query("provider_id")` and returns HTTP 400 if absent — this handler was not updated in 008. Verify after T016/T017 edits that no regression was introduced. The call must remain `DELETE /api/v2/amm/liquidity/commits/:id?provider_id=<bankId>`.
  - **Acceptance**: `cancelCommit` function exists with two params; DELETE request URL includes `?provider_id=<bankId>`; TypeScript compiles.

---

**M2 Checkpoint**: `cd frontend && npm run type-check --workspace=governance` passes. `grep -r "mintAndApprove\|addLiquidity" frontend/apps/governance/src/services/api/` → zero results.

---

## Milestone 3: Zustand Store (`liquidity.store.ts`)

**Goal**: Store actions match simplified API contracts; dead actions removed; phase machine handles `CANCELLED` terminal state.
**Independent Validation**: `tsc --noEmit` passes; `cancelActiveCommit` callers compile with the new single-argument signature.

---

- [X] T021 [US3] Update imports in `liquidity.store.ts` — remove stale types, add `BridgeLockMintResponse` in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Milestone**: M3.1
  - **File**: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Description**: Remove `AddLiquidityRequest` and `MintAndApproveRequest` from the type import. Add `BridgeLockMintResponse` to the import from `../../types/liquidity.types`. This task must be done first to avoid import errors blocking the rest of M3.
  - **Acceptance**: Import block has no `AddLiquidityRequest` or `MintAndApproveRequest`; `BridgeLockMintResponse` is imported; TypeScript compiles.

---

- [X] T022 [P] [US3] Remove `mintAndApprove` action from `LiquidityStore` type and implementation in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Milestone**: M3.2 | **FR**: FR-017
  - **File**: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Description**: Delete `mintAndApprove` from the `LiquidityStore` TypeScript interface and from the `create(...)` implementation block. This action called the now-blocked endpoint.
  - **Acceptance**: `grep "mintAndApprove" frontend/apps/governance/src/features/liquidity/liquidity.store.ts` → zero results; TypeScript compiles.

---

- [X] T023 [P] [US3] Remove `addLiquidity` action from `LiquidityStore` type and implementation in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Milestone**: M3.3 | **FR**: FR-018
  - **File**: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Description**: Delete `addLiquidity` from the `LiquidityStore` TypeScript interface and from the `create(...)` implementation block.
  - **Acceptance**: `grep "addLiquidity" frontend/apps/governance/src/features/liquidity/liquidity.store.ts` → zero results; TypeScript compiles.

---

- [X] T024 [US1] Add `bridgeLockMintResult: BridgeLockMintResponse | null` and `currentBankId: string | null` to store state in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Milestone**: M3.4 + M3.7 (state part)
  - **File**: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Description**: Add two new state fields to the `LiquidityStore` interface and the `create(...)` initial state: `bridgeLockMintResult: BridgeLockMintResponse | null` (initial: `null`) and `currentBankId: string | null` (initial: `null`). `currentBankId` is set on `lockMint` success from the auth profile and used internally by `cancelActiveCommit`.
  - **Acceptance**: Both fields present in the interface and initialized in the store; TypeScript compiles; existing state fields unchanged.

---

- [X] T025 [US1] Add `lockMint(amount: string): Promise<void>` store action in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Milestone**: M3.5 | **FR**: FR-019
  - **File**: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Description**: Implement `lockMint(amount)` action: (1) sets `status = "loading"`, (2) calls `liquidityApi.lockMint({ amount })`, (3) on success: stores response in `bridgeLockMintResult`, sets `currentBankId` from auth store if available, transitions `sovereignPhase = SOVEREIGN_FLOW_PHASE.BRIDGE_ACTIVE` (or equivalent bridge-active-wait phase), (4) polls `liquidityApi.waitForBridgeActive()` — on `true`: sets `sovereignPhase = SOVEREIGN_FLOW_PHASE.COMMIT_PENDING`; on `false` / timeout: sets `sovereignPhase = SOVEREIGN_FLOW_PHASE.TIMEOUT`, (5) on error: `status = "error"`, `error = extractApiError(...)`.
  - **Acceptance**: Action signature is `lockMint(amount: string): Promise<void>`; response is stored in `bridgeLockMintResult`; `sovereignPhase` transitions correctly; TypeScript compiles.

---

- [X] T026 [US1] Update `submitCommit` to send `{ pool_pair, amount }` only and update timeout message in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Milestone**: M3.6 | **FR**: FR-020
  - **File**: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Description**: The call to `liquidityApi.commitLiquidity` must pass only `{ pool_pair, amount }` (compiler-enforced after M1.3). Remove any remaining `provider_id` or `side` fields from the payload. Update any timeout-related string in the action (e.g., error messages referencing "60 seconds") to reference "180 seconds".
  - **Acceptance**: `commitLiquidity` call has no `provider_id` or `side` in its argument; timeout message references 180 s; TypeScript compiles.

---

- [X] T027 [US2] Update `cancelActiveCommit` to signature `(commitId: string)` with internal `bankId` derivation in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Milestone**: M3.7 | **FR**: FR-021
  - **File**: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Description**: Change `cancelActiveCommit` public signature from `(commitId: string, providerId: string)` to `(commitId: string)`. Internally, derive `bankId` via `get().currentBankId` (set during `lockMint` success). Pass `bankId` to `liquidityApi.cancelCommit(commitId, bankId)`. Update the `LiquidityStore` interface accordingly.
  - **Acceptance**: `cancelActiveCommit` has exactly one parameter `commitId: string`; internally calls `liquidityApi.cancelCommit(commitId, bankId)` where `bankId` comes from store state; TypeScript compiles with no errors on the updated interface.

---

- [X] T028 [US1] Add `fetchLpPositions(poolPair: string, providerBankId: string): Promise<void>` store action in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Milestone**: M3.8 | **FR**: FR-022
  - **File**: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Description**: Implement `fetchLpPositions(poolPair, providerBankId)` action: sets `status = "loading"`, calls `liquidityApi.listPositions(poolPair, providerBankId)`, stores result in the existing `lpPositions` state field, sets `status = "idle"` on success, `status = "error"` with `extractApiError(...)` on failure.
  - **Acceptance**: Action is in the `LiquidityStore` interface and implementation; result goes to `lpPositions`; loading/error states toggled correctly; TypeScript compiles.

---

- [X] T029 [US1] Update `clearCommit` to also reset `bridgeLockMintResult = null` in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Milestone**: M3.9
  - **File**: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Description**: Find the `clearCommit` (or equivalent reset) action and add `bridgeLockMintResult: null` to the state reset. Ensures subsequent wizard runs start clean without stale bridge data.
  - **Acceptance**: `clearCommit` sets `bridgeLockMintResult: null`; no other reset fields removed; TypeScript compiles.

---

- [X] T030 [US2] Add `CANCELLED` phase handling in commit status poll callback in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Milestone**: M3.10 | **FR**: FR-023
  - **File**: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
  - **Description**: In the commit polling logic (wherever `status === "EXECUTED"` is checked), add a branch: when poll returns `status === "CANCELLED"`, stop polling, reset `commitLatencyWarning: false`, and set `sovereignPhase = SOVEREIGN_FLOW_PHASE.CANCELLED`. `CANCELLED` must be a reachable terminal state — do not continue polling after reaching it.
  - **Acceptance**: Poll loop exits on `CANCELLED`; `sovereignPhase` is set to `"CANCELLED"`; `commitLatencyWarning` is reset to `false`; TypeScript compiles; existing `EXECUTED` / `TIMEOUT` branches are unchanged.

---

**M3 Checkpoint**: `cd frontend && npm run type-check --workspace=governance` passes. `cancelActiveCommit` has one argument in the type interface. `grep "mintAndApprove\|addLiquidity" frontend/apps/governance/src/features/liquidity/liquidity.store.ts` → zero results.

---

## Milestone 4: Wizard UI (`CooperativeLiquidityWizard.tsx`)

**Goal**: Wizard reflects simplified flow — Step 1 calls `lockMint`, Step 2 has no provider/side inputs, Step 3 shows latency warnings and `CANCELLED` state, Step 4 renders LP positions from API.
**Independent Validation**: `tsc --noEmit` passes; wizard renders without console errors; network traffic matches API contracts in `contracts/frontend-cb-liquidity-refactor.md`.

---

- [X] T031 [US3] Remove `mintAndApprove` from store selector and local binding in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.1
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Find the `useLiquidityStore` selector that extracts `mintAndApprove` and remove it. Remove the corresponding local variable binding. TypeScript will error on any remaining usages, guiding further cleanup in T034/T035.
  - **Acceptance**: `grep "mintAndApprove" frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx` → zero results; TypeScript compiles (possibly with temporary errors resolved by T034–T035).

---

- [X] T032 [US1] Add `lockMint` and `fetchLpPositions` selectors from store in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.2 + M4.3
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Add `lockMint` and `fetchLpPositions` to the `useLiquidityStore` selector. Pattern: `const lockMint = useLiquidityStore((state) => state.lockMint)` and `const fetchLpPositions = useLiquidityStore((state) => state.fetchLpPositions)`.
  - **Acceptance**: Both bindings present; TypeScript resolves types from `LiquidityStore` interface correctly; no `any` types introduced.

---

- [X] T033 [US1] Add `bridgeLockMintResult` from store state in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.4
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Add `bridgeLockMintResult` to the store state selector. This value is displayed in Step 1 success state (position ID, bridge state, etc.) and used to confirm bridge activity before advancing to Step 2.
  - **Acceptance**: `bridgeLockMintResult` binding present; TypeScript resolves to `BridgeLockMintResponse | null`; no type errors.

---

- [X] T034 [US1] Rename `handleMintAndApprove` → `handleLockMint`; call `lockMint(mintAmount)` in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.5
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Rename the Step 1 submit handler from `handleMintAndApprove` to `handleLockMint`. Change the body to call `await lockMint(mintAmount)` instead of `await mintAndApprove(...)`. The handler takes only `mintAmount` from local form state — no provider or token fields.
  - **Acceptance**: `grep "handleMintAndApprove\|mintAndApprove" frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx` → zero results; `handleLockMint` calls `lockMint(mintAmount)`; TypeScript compiles.

---

- [X] T035 [US1] Update Step 1 heading and button label in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.6
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Change Step 1 section heading from `"Step 1: Mint & Approve"` (or equivalent) to `"Step 1: Bridge Lock-Mint"`. Change the submit button label from `"Mint & Approve"` to `"Submit Bridge Lock-Mint"`. This aligns the label with the actual operation (FR-024).
  - **Acceptance**: No "Mint & Approve" labels in Step 1 UI strings; heading and button use the updated text; rendered HTML reflects changes.

---

- [X] T036 [US1] Remove `commitProviderId`, `commitSide` state, their `useEffect`, and corresponding form inputs from Step 2 in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.7 | **FR**: FR-025
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Delete: (a) `commitProviderId` and `commitSide` local state declarations, (b) any `useEffect` that initializes or syncs these state values, (c) the "Provider ID" text input in the Step 2 form, (d) the "Commit Side" radio button group in the Step 2 form. Step 2 form must only have `pool_pair` (selector or read-only) and `amount` input.
  - **Acceptance**: `grep "commitProviderId\|commitSide" frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx` → zero results; Step 2 form has no provider/side inputs; TypeScript compiles.

---

- [X] T037 [US1] Change `commitPoolPair` initial value from `"BRL-USD"` to `"W-BRL-ARS"` in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.8 | **FR**: FR-026
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Find `const [commitPoolPair, setCommitPoolPair] = useState("BRL-USD")` (or equivalent) and change the initial value to `"W-BRL-ARS"`. This is the correct pool pair for Scenario B sovereign CB liquidity.
  - **Acceptance**: `commitPoolPair` initializes to `"W-BRL-ARS"`; no hardcoded `"BRL-USD"` remains for this state; TypeScript compiles.

---

- [X] T038 [US1] Update `toActiveCommitFromPending` (or equivalent) to use `pool_pair: "W-BRL-ARS"` in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.9 | **FR**: FR-027
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Find the `toActiveCommitFromPending` function or the resumption-from-pending logic that constructs a commit object from pending state. Change any `pool_pair: "BRL-USD"` to `pool_pair: "W-BRL-ARS"`. This ensures resumed commits use the correct pool pair.
  - **Acceptance**: `toActiveCommitFromPending` (or equivalent) uses `pool_pair: "W-BRL-ARS"`; no `"BRL-USD"` string remains in this function; TypeScript compiles.

---

- [X] T039 [US1] Update `handleCommit` to submit only `{ pool_pair: commitPoolPair, amount: commitAmount }` in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.10 | **FR**: FR-025
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Find `handleCommit` (Step 2 submit handler) and ensure it passes only `{ pool_pair: commitPoolPair, amount: commitAmount }` to the store's `submitCommit` action. Remove any remaining `provider_id` or `side` fields from the argument object. TypeScript enforces this via `CommitRequest` type (simplified in M1.3).
  - **Acceptance**: `handleCommit` passes a `CommitRequest`-compatible object with exactly `pool_pair` and `amount`; no `provider_id` or `side` in the call; TypeScript compiles.

---

- [X] T040 [US1] Add `on_chain_commit_id` display in Step 2 success state in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.11 | **FR**: FR-031
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: In the Step 2 success/submitted state display (where `commit_id` is already shown), add a row for `on_chain_commit_id`: if `activeCommit?.on_chain_commit_id` is non-null and non-empty, display its value; otherwise display the string `"pending on-chain confirmation"`.
  - **Acceptance**: Step 2 success UI renders `on_chain_commit_id` or `"pending on-chain confirmation"`; null/empty `on_chain_commit_id` does not crash; TypeScript compiles with `activeCommit?.on_chain_commit_id` typed as `string | null`.

---

- [X] T041 [US1] Add latency tracking display and `CANCELLED` badge in Step 3 in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.12 | **FR**: FR-028
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: (a) Verify the existing 30 s `commitLatencyWarning` badge renders in Step 3 — no change needed if already working; (b) Update the `SOVEREIGN_FLOW_PHASE.TIMEOUT` message to reference `"180 seconds"` (was "60 seconds"); (c) Add a `SOVEREIGN_FLOW_PHASE.CANCELLED` branch in Step 3 render: show a badge with `"Commit was cancelled."` text, stop any active polling interval, and do not show the Cancel button.
  - **Acceptance**: Timeout message references "180 seconds"; `CANCELLED` phase shows badge and no Cancel button; 30 s warning badge is visible when `commitLatencyWarning === true`; TypeScript compiles.

---

- [X] T042 [US1] Add `on_chain_commit_id` display in Step 3 monitor in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.13 | **FR**: FR-031
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: In the Step 3 polling monitor (where `commit_id` is displayed), add a line below it for `on_chain_commit_id`. Apply the same null-guard as T040: display value if present, `"pending on-chain confirmation"` otherwise.
  - **Acceptance**: Step 3 monitor shows `on_chain_commit_id` row alongside `commit_id`; null case displays fallback string; no crash on null/empty value.

---

- [X] T043 [US2] Update `handleCancelCommit` to call `cancelActiveCommit(commitId)` with one argument in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.14 | **FR**: FR-030
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Find `handleCancelCommit` and change the call from `cancelActiveCommit(commitId, monitoredProviderId)` (two arguments) to `cancelActiveCommit(commitId)` (one argument). The store action (updated in M3.7 / T027) derives `bankId` internally. Remove the `monitoredProviderId` local variable if it was only used in this call.
  - **Acceptance**: `cancelActiveCommit` called with exactly one argument in the wizard; `grep "monitoredProviderId" CooperativeLiquidityWizard.tsx` → zero results (if variable is otherwise unused); TypeScript compiles with updated `LiquidityStore` interface.

---

- [X] T044 [US1] Add Step 4 `useEffect` to call `fetchLpPositions("W-BRL-ARS", bankId)` on mount in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.15 | **FR**: FR-029
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Add the following effect (or equivalent using the wizard's step-tracking mechanism):
    ```tsx
    useEffect(() => {
      if (currentStep !== 4 || !profile?.bankId) return;
      void fetchLpPositions("W-BRL-ARS", profile.bankId);
    }, [currentStep, profile?.bankId, fetchLpPositions]);
    ```
    Where `profile` is the authenticated user profile from auth store (containing `bankId`). The effect fires when entering Step 4.
  - **Acceptance**: `GET /api/v2/amm/liquidity/positions?pool_pair=W-BRL-ARS&provider_id=<bankId>` is called when wizard reaches Step 4; effect deps array is correct; no `eslint-disable` needed for deps; TypeScript compiles.

---

- [X] T045 [US1] Replace Step 4 LP display with positions table from `lpPositions` store state in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.16 | **FR**: FR-029
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: Remove the old `commitForDisplay.lp_ids` list (static data). Replace with a table/list reading from `lpPositions` store state. Columns/rows: `lp_id`, `deposit_side`, `token_a_contributed`, `token_b_contributed`, `commit_id`. If `lpPositions` array is empty, render `"No positions found for this pool."` paragraph instead of the table. Keep existing "Add More Liquidity" and "Done" buttons.
  - **Acceptance**: `lpPositions` state drives the rendered list; all 5 fields shown per position; empty state renders fallback message; `token_a_amount`/`token_b_amount` field names do not appear anywhere (renamed in M1.6); TypeScript compiles.

---

- [X] T046 [US1] Verify Step 4 handles `positions: []` (empty array) without crashing in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Milestone**: M4.16 (edge case guard)
  - **File**: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
  - **Description**: The backend returns `{ pool_pair: "W-BRL-ARS", positions: [], count: 0 }` when no positions exist for the CB. Step 4 must render `"No positions found for this pool."` message and NOT crash or show an empty table header. This is the continuation of T045 — verify the empty guard is implemented and works correctly.
  - **Acceptance**: When `lpPositions.length === 0`, wizard Step 4 renders the fallback message; no React key errors; no undefined-access crashes; browser console is clean.

---

**M4 Checkpoint**: `cd frontend && npm run type-check --workspace=governance` passes with zero errors. Wizard loads in browser with `VITE_SCENARIO=b` without console errors.

---

## Integration Validation

**Purpose**: Confirm all four milestones work end-to-end and Scenario A is not regressed.

---

- [X] T047 Run TypeScript type-check on governance app in `frontend/apps/governance/`
  - **Milestone**: Integration | **FR**: NFR-001
  - **File**: `frontend/apps/governance/tsconfig.json`
  - **Description**: Run `cd frontend && npm run type-check --workspace=governance` (or `npx tsc --noEmit` from `frontend/apps/governance/`). All four changed files (`liquidity.types.ts`, `liquidity.api.ts`, `liquidity.store.ts`, `CooperativeLiquidityWizard.tsx`) must compile cleanly.
  - **Acceptance**: Zero TypeScript errors. Exit code 0.

---

- [X] T048 Run ESLint on governance app in `frontend/apps/governance/`
  - **Milestone**: Integration | **FR**: NFR-003
  - **File**: `frontend/apps/governance/package.json`
  - **Description**: Run `cd frontend && npm run lint --workspace=governance`. Check for dangling imports of `AddLiquidityRequest`, `MintAndApproveRequest`, or any `no-unused-vars` errors introduced by the refactor.
  - **Acceptance**: Zero lint errors or warnings in the four changed files. No dangling imports.

---

- [ ] T049 Run dead-code grep verification across governance app source in `frontend/apps/governance/src/`
  - **Milestone**: Integration | **FR**: NFR-003 | **US3 validation**
  - **File**: `frontend/apps/governance/src/` (all files)
  - **Description**: Run the four grep commands from `plan.md § Verification Plan §3`:
    ```bash
    cd frontend/apps/governance/src
    grep -r "mintAndApprove"        types/ services/api/ features/liquidity/
    grep -r "addLiquidity"          types/ services/api/ features/liquidity/
    grep -r "AddLiquidityRequest"   types/ services/api/ features/liquidity/
    grep -r "MintAndApproveRequest" types/ services/api/ features/liquidity/
    ```
  - **Acceptance**: All four commands return zero results.

---

- [ ] T050 Execute manual E2E wizard happy path (Scenario B environment) using `tryouts/tryout-sovereign-cb-liquidity.sh`
  - **Milestone**: Integration | **US1 validation**
  - **File**: `tryouts/tryout-sovereign-cb-liquidity.sh`
  - **Description**: With a running Scenario B backend: (1) Start governance app with `VITE_SCENARIO=b`. (2) Step 1: enter amount, submit → network shows `POST /api/v2/bridge/lock-mint` with `{ "amount": "..." }` only (SC-001). (3) Step 2: pool pair = `W-BRL-ARS`, amount, submit → network shows `POST /api/v2/amm/liquidity/commit` with `{ "pool_pair", "amount" }` only (SC-002); `on_chain_commit_id` shown or "pending" (SC-009). (4) Step 3: 30 s warning visible; 180 s timeout message correct. (5) Step 4: `GET /api/v2/amm/liquidity/positions?pool_pair=W-BRL-ARS&provider_id=<bankId>` called; LP positions rendered (SC-004).
  - **Acceptance**: All five steps complete without HTTP errors, console errors, or broken states. Network requests match SC-001 through SC-009 from `contracts/frontend-cb-liquidity-refactor.md`.

---

- [ ] T051 Execute cancel flow and Scenario A non-regression in `frontend/apps/governance/src/`
  - **Milestone**: Integration | **US2 validation + NFR non-regression**
  - **File**: `frontend/apps/governance/src/routes/index.tsx`
  - **Description**: (A) Cancel flow: Step 3 → click Cancel → network shows `DELETE /api/v2/amm/liquidity/commits/:id?provider_id=<bankId>` (SC-007); `CANCELLED` badge appears in Step 3 (SC-008). (B) Scenario A: build governance app without `VITE_SCENARIO=b` → Scenario B routes/menus hidden; no TypeScript or console errors on Scenario A pages.
  - **Acceptance**: Cancel flow sends correct URL with `provider_id` query param; `CANCELLED` phase renders badge; Scenario A build is clean with zero new console errors.

---

- [ ] T052 Execute E2E tryout scripts for full flow validation
  - **Milestone**: Integration | **FR**: Verification Plan §7
  - **File**: `tryouts/tryout-scenario-b-e2e.sh`
  - **Description**: Run `bash tryouts/tryout-sovereign-cb-liquidity.sh` and `bash tryouts/tryout-scenario-b-e2e.sh` against a live Scenario B environment. Capture output and confirm no unexpected errors.
  - **Acceptance**: Both scripts complete without error. Any failures are documented with root cause. Frontend-induced failures (wrong payloads, missing fields, wrong URLs) must be zero.

---

## Summary

| Milestone | Tasks | User Stories | Key Acceptance |
|-----------|-------|--------------|----------------|
| M1 — Types | T001–T013 | US1, US2, US3 | `tsc --noEmit` clean; no `AddLiquidityRequest`/`MintAndApproveRequest` |
| M2 — API | T014–T020 | US1, US2, US3 | `tsc --noEmit` clean; `lockMint` + `listPositions` added; dead functions gone |
| M3 — Store | T021–T030 | US1, US2, US3 | `cancelActiveCommit(commitId)` single-arg; `lockMint` + `fetchLpPositions` actions added |
| M4 — Wizard | T031–T046 | US1, US2, US3 | Step 1 calls `lockMint`; Step 2 no provider/side; Step 4 renders LP positions |
| Integration | T047–T052 | All | Zero tsc errors; zero lint errors; zero dead-code grep hits; E2E happy path passes |

**Total**: 52 tasks across 4 milestones + integration phase.

**Parallelism**: T008–T011 (M1); T015/T016/T017/T019 (M2); T022/T023 (M3). All other tasks within a milestone are sequential due to same-file dependencies.

**Execution order**: M1 must be fully complete before M2 starts (TypeScript compilation chain). M2 before M3. M3 before M4. Integration after M4.
