# Implementation Plan: CB Liquidity Frontend Refactor (Scenario B)

**Branch**: `009-cb-liquidity-refactor` | **Date**: 2026-05-26 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/009-cb-liquidity-refactor/spec.md`

## Summary

Adapt the governance frontend sovereign CB liquidity wizard (`CooperativeLiquidityWizard`) and its supporting layers (types, API client, Zustand store) to consume the simplified API introduced by `008-fix-cb-liquidity`. The backend now derives `provider_id`, `side`, and `w_token_address` from JWT + server config; dead endpoints (`/amm/liquidity/add`, `/amm/token/mint-and-approve`) are removed; field names on `LiquidityPosition` changed; and a new `/bridge/lock-mint` endpoint and `/amm/liquidity/positions` endpoint must be wired into the wizard. All changes are **frontend-only** (TypeScript/React in `frontend/apps/governance`).

## Technical Context

**Language/Version**: TypeScript 5.9.x, React 19, Node 20  
**Primary Dependencies**: Vite, React Router v6, Zustand 5, Axios, Tailwind + `@cbweb3/ui`, Lucide React  
**Storage**: N/A — Zustand in-memory state only; no persistence  
**Testing**: `tsc --noEmit` (type check), `eslint`, manual E2E via tryout scripts  
**Target Platform**: SPA web (Vite/Nginx), desktop browser, Linux Docker  
**Project Type**: Web application — governance app frontend only  
**Performance Goals**: Commit polling 3 s interval; bridge polling 5 s interval; UI warns at 30 s; hard timeout at 180 s  
**Constraints**: Frontend-only scope. No backend, Solidity, or infra changes. Governance app only (`frontend/apps/governance`); bank app is out of scope. Scenario A gating must not regress.  
**Scale/Scope**: 4 files changed; ~250–350 lines of code modified/added

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is in template state with no filled-in principles. Gates applied by scope:

| Gate | Status | Note |
|---|---|---|
| Frontend-only (no backend changes) | PASS | Scope restricted to `frontend/apps/governance` |
| Scenario A regression zero | PASS | No gating logic (`isScenarioB`) is touched |
| Source-of-truth alignment | PASS | Spec uses 008-fix-cb-liquidity API contracts + confirmed clarifications |
| No breaking API surface changes | PASS | Only consuming existing endpoints differently |
| Dead code removal is complete | PASS | All `mintAndApprove`/`addLiquidity` references removed in scope |

**Re-check post-design**: PASS. No violations identified.

## Project Structure

### Documentation (this feature)

```text
specs/009-cb-liquidity-refactor/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── frontend-cb-liquidity-refactor.md   # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
frontend/apps/governance/src/
├── types/
│   └── liquidity.types.ts          # M1: types layer
├── services/api/
│   └── liquidity.api.ts            # M2: API client layer
└── features/liquidity/
    ├── liquidity.store.ts          # M3: Zustand store
    └── CooperativeLiquidityWizard.tsx  # M4: wizard UI
```

**Structure Decision**: Modifications to 4 existing files in `frontend/apps/governance`. No new files, no new modules. The dependency order (types → api → store → UI) dictates the milestone sequence.

## Phase 0: Research

**Status**: Complete — see [research.md](./research.md)

Key decisions resolved:

1. `cancelCommit` still sends `provider_id` as a query param (backend handler not updated in 008). Store derives it from auth state; public action signature simplifies to `(commitId: string)`.
2. `GET /api/v2/amm/liquidity/commits` response fields are PascalCase (`CommitID`, `ProviderID`, etc.) — `domain.PoolCommit` has no `json:` tags.
3. `GET /api/v2/amm/liquidity/positions` requires explicit `provider_id` param for per-CB filtering; does not auto-filter by JWT.
4. `CANCELLED` is added as a distinct terminal phase (not reused as `FAILED`).
5. Initial `sovereignPhase` stays `LOCK_MINT` — no `IDLE` pre-state needed (out of scope).
6. `on_chain_commit_id` type is `string | null` (from Go `*[]byte`).
7. `waitForBridgeActive` is promoted to a first-class step invoked inside the `lockMint` store action.

## Phase 1: Design

### Artifacts generated

| Artifact | Status | Link |
|---|---|---|
| research.md | Complete | [research.md](./research.md) |
| data-model.md | Complete | [data-model.md](./data-model.md) |
| quickstart.md | Complete | [quickstart.md](./quickstart.md) |
| contracts/frontend-cb-liquidity-refactor.md | Complete | [contracts/frontend-cb-liquidity-refactor.md](./contracts/frontend-cb-liquidity-refactor.md) |

### Impacted Modules

| File | Change type | Summary |
|---|---|---|
| `types/liquidity.types.ts` | Modify | Add 4 types, update 4 types, delete 2 types, fix constant |
| `services/api/liquidity.api.ts` | Modify | Add `lockMint`, `listPositions`; delete `mintAndApprove`, `addLiquidity`; simplify `commitLiquidity` |
| `features/liquidity/liquidity.store.ts` | Modify | Add `lockMint`, `fetchLpPositions` actions; delete `mintAndApprove`, `addLiquidity`; simplify `submitCommit`, `cancelActiveCommit` |
| `features/liquidity/CooperativeLiquidityWizard.tsx` | Modify | Wire Step 1→`lockMint`; remove provider/side fields in Step 2; fix pool pair defaults; add latency display in Step 3; add positions display in Step 4 |

## Overview

The governance app's 4-step sovereign CB liquidity wizard is broken at Step 1 (calls a blocked endpoint) and sends stale payloads in Step 2. This refactor aligns the wizard to the 008-fix-cb-liquidity simplified backend API:

**Step 1 (Lock-Mint)**: Was calling `mintAndApprove` → HTTP 403. Now calls `POST /api/v2/bridge/lock-mint` with `{ amount }` only.  
**Step 2 (Commit)**: Was sending `provider_id` + `side` → backend ignores/rejects. Now sends only `pool_pair` + `amount`.  
**Step 3 (Monitor)**: Adds 30 s warning + 180 s timeout display. Recognizes `CANCELLED` as a terminal state.  
**Step 4 (Positions)**: Was showing only static pool data. Now calls `GET /api/v2/amm/liquidity/positions` and renders LP position details.

## Non-Goals

- Backend changes of any kind
- Bank app (`frontend/apps/bank`) changes
- Solidity contract changes
- Scenario A gating modifications
- Wizard layout redesign beyond the specific field removals/additions in spec
- Authentication or JWT handling changes
- New error codes beyond those already handled in spec-008

## Current Reality

### What exists now (broken state)

| Surface | Current state |
|---|---|
| `CommitRequest` | Has `provider_id` + `side` fields (server-derived; ignored or causes errors) |
| `CommitStatus` | Has `"MATCHED"` (nonexistent backend value); missing `"CANCELLED"` |
| `CommitResult` | Missing `on_chain_commit_id` |
| `LiquidityPosition` | Fields named `token_a_amount`/`token_b_amount`; missing `deposit_side`, `commit_id` |
| `AddLiquidityRequest` | Exists as dead type |
| `MintAndApproveRequest` | Exists as dead type |
| `SOVEREIGN_FLOW_POLICY.commitTimeoutMs` | 60,000 ms (should be 180,000 ms) |
| `liquidityApi.mintAndApprove()` | Calls HTTP 403 blocked endpoint |
| `liquidityApi.addLiquidity()` | Calls dead endpoint |
| `liquidityApi.lockMint()` | Does not exist |
| `liquidityApi.listPositions()` | Does not exist |
| Store `mintAndApprove` action | Exists; calls blocked endpoint |
| Store `addLiquidity` action | Exists; calls dead endpoint |
| Store `lockMint` action | Does not exist |
| Store `fetchLpPositions` action | Does not exist |
| Store `cancelActiveCommit` | Signature `(commitId, providerId)` |
| Wizard Step 1 | Label "Mint & Approve"; calls `mintAndApprove`; broken |
| Wizard Step 2 | Shows Provider ID input + Commit Side radios (not needed) |
| Wizard `commitPoolPair` default | `"BRL-USD"` (wrong pool pair) |
| Wizard Step 3 | No timeout warning display |
| Wizard Step 4 | No API call; no LP position details |

### Key files and their roles

- [frontend/apps/governance/src/types/liquidity.types.ts](../../../frontend/apps/governance/src/types/liquidity.types.ts) — shared TypeScript types; no external deps
- [frontend/apps/governance/src/services/api/liquidity.api.ts](../../../frontend/apps/governance/src/services/api/liquidity.api.ts) — Axios-based API client; depends on types
- [frontend/apps/governance/src/features/liquidity/liquidity.store.ts](../../../frontend/apps/governance/src/features/liquidity/liquidity.store.ts) — Zustand store; depends on types + api
- [frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx](../../../frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx) — React wizard UI; depends on store + types

## Proposed Approach

Apply changes in dependency order: types first (no deps), then api (depends on types), then store (depends on types + api), then wizard UI (depends on store + types). Each milestone is independently compilable after its changes are applied. No new files are created; all changes are edits to existing files.

## Milestones

### M1 — Types Layer (`liquidity.types.ts`)

**Goal**: All type definitions match the 008 backend API contracts.

- [ ] M1.1: Remove `AddLiquidityRequest` interface
- [ ] M1.2: Remove `MintAndApproveRequest` interface
- [ ] M1.3: Simplify `CommitRequest` to `{ pool_pair: string; amount: string }` (remove `provider_id`, `side`)
- [ ] M1.4: Add `on_chain_commit_id: string | null` to `CommitResult`
- [ ] M1.5: Replace `"MATCHED"` with `"CANCELLED"` in `CommitStatus` union
- [ ] M1.6: Rename `LiquidityPosition.token_a_amount` → `token_a_contributed`; `token_b_amount` → `token_b_contributed`
- [ ] M1.7: Add `deposit_side: string` and `commit_id: string` to `LiquidityPosition`
- [ ] M1.8: Add `BridgeLockMintRequest` type: `{ amount: string }`
- [ ] M1.9: Add `BridgeLockMintResponse` type with 6 fields (`position_id`, `bridge_state`, `spoke_network`, `native_asset`, `mirrored_asset`, `amount`)
- [ ] M1.10: Add `LpPositionsResponse` type: `{ pool_pair: string; positions: LiquidityPosition[]; count: number }`
- [ ] M1.11: Add `CommitListItem` type with PascalCase fields (see data-model.md)
- [ ] M1.12: Add `CANCELLED` to `SOVEREIGN_FLOW_PHASE` constant and `SovereignFlowPhase` type
- [ ] M1.13: Change `SOVEREIGN_FLOW_POLICY.commitTimeoutMs` from `60_000` to `180_000`

**Verification**: `tsc --noEmit` on types file alone; `grep` confirms no `AddLiquidityRequest`/`MintAndApproveRequest` remain.

---

### M2 — API Client Layer (`liquidity.api.ts`)

**Goal**: API client exposes only live endpoints with correct payloads.

- [ ] M2.1: Add `lockMint(req: BridgeLockMintRequest): Promise<BridgeLockMintResponse>` → `POST /api/v2/bridge/lock-mint` with `{ amount: req.amount }` body only
- [ ] M2.2: Remove `mintAndApprove` function and its `MintAndApproveRequest` import
- [ ] M2.3: Remove `addLiquidity` function and its `AddLiquidityRequest` import
- [ ] M2.4: Update `commitLiquidity` to accept simplified `CommitRequest` (compiler-enforced after M1.3); ensure payload sent is `{ pool_pair, amount }` only — remove any explicit `provider_id`/`side` spreading
- [ ] M2.5: Add `listPositions(poolPair: string, providerBankId: string): Promise<LpPositionsResponse>` → `GET /api/v2/amm/liquidity/positions?pool_pair=<poolPair>&provider_id=<providerBankId>`
- [ ] M2.6: Verify `cancelCommit(commitId, providerId)` still passes `provider_id` as query param (already correct; confirm no regression from M2.2–M2.3 edits)
- [ ] M2.7: Update import list at top of file to include new types (`BridgeLockMintRequest`, `BridgeLockMintResponse`, `LpPositionsResponse`) and remove deleted types

**Verification**: `tsc --noEmit`; `grep` confirms no `mintAndApprove`/`addLiquidity`/`AddLiquidityRequest`/`MintAndApproveRequest` remain.

---

### M3 — Zustand Store (`liquidity.store.ts`)

**Goal**: Store actions match simplified API contracts; dead actions removed; phase machine includes `CANCELLED`.

- [ ] M3.1: Update import list — remove `AddLiquidityRequest`, `MintAndApproveRequest`; add `BridgeLockMintResponse`
- [ ] M3.2: Remove `mintAndApprove` from `LiquidityStore` type and implementation
- [ ] M3.3: Remove `addLiquidity` from `LiquidityStore` type and implementation
- [ ] M3.4: Add `bridgeLockMintResult: BridgeLockMintResponse | null` to store state shape (initial value `null`)
- [ ] M3.5: Add `lockMint(amount: string): Promise<void>` action:
  - Calls `liquidityApi.lockMint({ amount })`
  - On success: stores response in `bridgeLockMintResult`, sets `sovereignPhase = BRIDGE_ACTIVE_WAIT`
  - Then polls `liquidityApi.waitForBridgeActive()`; on `true`: sets `sovereignPhase = COMMIT_PENDING` (advances wizard to Step 2 readiness); on `false`: sets `sovereignPhase = TIMEOUT`
  - On error: `status = "error"`, `error = extractApiError(...)`
- [ ] M3.6: Update `submitCommit` — payload to `liquidityApi.commitLiquidity` is `{ pool_pair, amount }` only (compiler-enforced after M1.3); update timeout message to reference 180 s (not 60 s)
- [ ] M3.7: Update `cancelActiveCommit` signature to `(commitId: string)` — derives `bankId` from store state (use `get().currentBankId ?? get().poolStatus` fallback or auth store); passes to `liquidityApi.cancelCommit(commitId, bankId)`
  - Add `currentBankId: string | null` to store state (set from auth profile `bankId` on `lockMint` or initialize from auth store import)
- [ ] M3.8: Add `fetchLpPositions(poolPair: string, providerBankId: string): Promise<void>` action:
  - Calls `liquidityApi.listPositions(poolPair, providerBankId)`
  - Stores result in `lpPositions` (existing state field)
  - Sets `status = "loading"` before call, `"idle"` on success, `"error"` on failure
- [ ] M3.9: Update `clearCommit` to also reset `bridgeLockMintResult = null`
- [ ] M3.10: Add `CANCELLED` phase handling — `commitLatencyWarning: false` reset and `sovereignPhase = CANCELLED` when commit poll returns `CANCELLED` status

**Verification**: `tsc --noEmit`; confirm `cancelActiveCommit` callers in wizard compile with new single-arg signature.

---

### M4 — Wizard UI (`CooperativeLiquidityWizard.tsx`)

**Goal**: Wizard reflects simplified flow — Step 1 calls `lockMint`, Step 2 has no provider/side inputs, Step 3 shows latency warning, Step 4 shows positions.

- [ ] M4.1: Remove `mintAndApprove` from store selector and local variable
- [ ] M4.2: Add `lockMint` from store (`useLiquidityStore((state) => state.lockMint)`)
- [ ] M4.3: Add `fetchLpPositions` from store
- [ ] M4.4: Add `bridgeLockMintResult` from store state
- [ ] M4.5: Rename `handleMintAndApprove` handler to `handleLockMint`; change it to call `lockMint(mintAmount)` instead of `mintAndApprove`
- [ ] M4.6: Change Step 1 heading from `"Step 1: Mint & Approve"` to `"Step 1: Bridge Lock-Mint"`; change submit button label to `"Submit Bridge Lock-Mint"`
- [ ] M4.7: Remove `commitProviderId` state + `commitSide` state + their `useEffect` + Provider ID input + Commit Side radio group from Step 2 form
- [ ] M4.8: Change `commitPoolPair` initial state from `"BRL-USD"` to `"W-BRL-ARS"`
- [ ] M4.9: Change `toActiveCommitFromPending` function — set `pool_pair: "W-BRL-ARS"` (was `"BRL-USD"`)
- [ ] M4.10: Update `handleCommit` to submit `{ pool_pair: commitPoolPair, amount: commitAmount }` only (remove `provider_id`, `side` fields)
- [ ] M4.11: Update Step 2 success state display to include `on_chain_commit_id`:
  - If `activeCommit?.on_chain_commit_id` is non-null/non-empty: display value
  - Else: display `"pending on-chain confirmation"`
- [ ] M4.12: Add latency tracking display in Step 3:
  - Add `commitElapsedMs` state (computed from store `commitLatencyWarning` or a local timer)
  - Show warning at 30 s: already exists (`commitLatencyWarning` badge) — verify it shows
  - Show timeout at 180 s: update existing `SOVEREIGN_FLOW_PHASE.TIMEOUT` message to reference "180 seconds"
  - Add `SOVEREIGN_FLOW_PHASE.CANCELLED` handling in Step 3: show `"Commit was cancelled."` badge and stop polling
- [ ] M4.13: Add `on_chain_commit_id` display in Step 3 monitor (below commit_id line)
- [ ] M4.14: Update `handleCancelCommit` to call `cancelActiveCommit(commitId)` with one argument only (remove `monitoredProviderId` from call; store derives `bankId` internally per M3.7)
- [ ] M4.15: Add Step 4 `useEffect` — on mount when `currentStep === 4`, call `fetchLpPositions("W-BRL-ARS", profile?.bankId ?? "")`:
  ```tsx
  useEffect(() => {
    if (currentStep !== 4 || !profile?.bankId) return;
    void fetchLpPositions("W-BRL-ARS", profile.bankId);
  }, [currentStep, profile?.bankId, fetchLpPositions]);
  ```
- [ ] M4.16: Replace Step 4 LP display — remove `commitForDisplay.lp_ids` list; add LP positions table from `lpPositions` store state:
  - Columns: `lp_id`, `deposit_side`, `token_a_contributed`, `token_b_contributed`, `commit_id`
  - If `lpPositions.length === 0`: show `"No positions found for this pool."` message
  - Keep "Add More Liquidity" and "Done" buttons

**Verification**: `tsc --noEmit`; wizard renders without console errors in browser; network traffic matches SC-001 through SC-009.

## Verification Plan

### 1. TypeScript compilation (SC-006)

```bash
cd frontend
npm run type-check --workspace=governance
# Expected: zero errors
```

### 2. Lint

```bash
cd frontend
npm run lint --workspace=governance
# Expected: zero errors/warnings in changed files
```

### 3. Dead code removal (SC-005)

```bash
cd frontend/apps/governance/src
grep -r "mintAndApprove"        types/ services/api/ features/liquidity/
grep -r "addLiquidity"          types/ services/api/ features/liquidity/
grep -r "AddLiquidityRequest"   types/ services/api/ features/liquidity/
grep -r "MintAndApproveRequest" types/ services/api/ features/liquidity/
# Expected: zero results in all four searches
```

### 4. Manual E2E — wizard happy path

1. Start governance app with `VITE_SCENARIO=b`
2. Step 1: enter amount, submit → network `POST /api/v2/bridge/lock-mint` with `{ amount }` only (SC-001)
3. Step 2: pool pair = `W-BRL-ARS`, enter amount, submit → network `POST /api/v2/amm/liquidity/commit` with `{ pool_pair, amount }` only (SC-002); `on_chain_commit_id` displayed or "pending" (SC-009)
4. Step 3: latency warn at 30 s (SC-003); `CANCELLED` status renders badge (SC-008); `on_chain_commit_id` shown
5. Step 4: `GET /api/v2/amm/liquidity/positions?pool_pair=W-BRL-ARS&provider_id=<bankId>` called on mount; LP position fields displayed (SC-004)

### 5. Cancel flow

1. Step 3 → click Cancel → network `DELETE /api/v2/amm/liquidity/commits/:id?provider_id=<bankId>` (SC-007)
2. `CANCELLED` badge shows in Step 3 (SC-008)

### 6. Scenario A non-regression

1. Build governance app without `VITE_SCENARIO=b` — Scenario B routes/menus remain hidden
2. No TypeScript errors or runtime console errors on Scenario A pages

### 7. Tryout scripts

```bash
bash tryouts/tryout-sovereign-cb-liquidity.sh
bash tryouts/tryout-scenario-b-e2e.sh
```

## Decision Log

- **2026-05-26**: `cancelCommit` keeps `provider_id` query param — backend handler not updated in 008. Store derives `bankId` from auth state internally; public action signature simplifies to `(commitId)`.
- **2026-05-26**: `CommitListItem` uses PascalCase fields — `domain.PoolCommit` has no `json:` struct tags; Go JSON encoder outputs verbatim field names.
- **2026-05-26**: `GET /api/v2/amm/liquidity/positions` requires explicit `provider_id` param — handler does not auto-filter by JWT; frontend must pass `bankId`.
- **2026-05-26**: `CANCELLED` added as distinct phase (not `FAILED`) — operator cancellation is intentional, must not show as an error state.
- **2026-05-26**: `IDLE` phase not introduced — wizard initializes to `LOCK_MINT` (existing behavior); adding `IDLE` pre-state would require a UI change beyond spec scope.
- **2026-05-26**: `waitForBridgeActive` promoted to first-class step inside `lockMint` action — previously only a fallback after `BRIDGE_POSITION_NOT_ACTIVE` error from commit.

## Data Model Note

Full type changes documented in [data-model.md](./data-model.md). No backend schema changes. No DB changes. All changes are TypeScript interface/type definitions consumed by the governance app frontend.

The four new/updated types — `BridgeLockMintRequest`, `BridgeLockMintResponse`, `LpPositionsResponse`, `CommitListItem` — live in `liquidity.types.ts` alongside the existing types and are imported by `liquidity.api.ts` as needed.
