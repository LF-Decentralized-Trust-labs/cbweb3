# Implementation Plan: Scenario B Frontend Integration

**Branch**: `feature/scenario-b-frontend` | **Date**: 2026-05-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `.agent/specs/scenario-b-frontend-integration/spec.md`

## Summary

Add Scenario B (AMM v2 / cross-border liquidity pool) UI to the `bank` and `governance` frontend apps, gated entirely by the build-time flag `VITE_SCENARIO=b`. When the flag is absent or `a`, both apps behave exactly as today. When set to `b`, a distinct set of pages, routes, sidebar items, API service modules, Zustand stores, and domain types are activated. All new code is strictly additive; only `AMMTradingPage.tsx` and `CircuitBreakerPage.tsx` are modified in-place to add an `isScenarioB` branch. All Scenario B network calls target real `/api/v2/*` endpoints — no mocks.

---

## Technical Context

**Language/Version**: TypeScript 5.4, React 19, Node 20
**Primary Dependencies**: Vite (build + env), Zustand 5 (state), Axios (HTTP), React Router v6 (routing), Tailwind CSS + shadcn/ui via `@cbweb3/ui` (UI primitives), Lucide React (icons)
**Storage**: Browser session state only (Zustand, no persistence). LP Positions table is session-only — no `GET /api/v2/amm/liquidity/positions` endpoint exists.
**Testing**: `tsc --noEmit` (TypeScript validation). No unit/integration tests in this feature (separate task per spec).
**Target Platform**: Web browser (Vite SPA, served via Nginx in Docker). Flag set at Docker Compose build time via `args: VITE_SCENARIO=b`.
**Project Type**: Monorepo frontend (React SPAs): `frontend/apps/bank`, `frontend/apps/governance`, `frontend/packages/ui`
**Performance Goals**: Bridge position polling ≤ 5s interval; circuit-breaker polling ≤ 15s interval. Polling stops at 120s timeout for bridge positions.
**Constraints**:
  - `VITE_SCENARIO` is build-time only (no runtime toggle)
  - No TypeScript enums — use `const` object + union type alias pattern
  - No mock API calls in Scenario B code paths
  - Amount fields are integer strings (no decimals)
  - Signature fields: text input labeled "Institutional Signature (base64)", pre-populated `"AA="`
  - `usePolling` hook lives in each app's own `hooks/` — NOT in `@cbweb3/ui`
  - LP Positions table must NOT poll `GET /api/v2/amm/liquidity/positions` (endpoint does not exist)
**Scale/Scope**: ~25 new files across 2 apps (pages, stores, services, types, hooks, config). 2 files modified in-place (AMMTradingPage, CircuitBreakerPage). Routing and sidebar files updated in both apps.

---

## Constitution Check

The project constitution (`constitution.md`) is a blank template — no organisation-specific gates are ratified. Standard engineering gates apply:

| Gate | Status | Notes |
|------|--------|-------|
| No TypeScript enums | PASS | All state values use `const` obj + union type. Enforced in spec FR-042. |
| No mock APIs in Scenario B | PASS | All Scenario B services import `httpClient` directly. FR-039. |
| No modification of Scenario A files | PASS | Only `AMMTradingPage.tsx` and `CircuitBreakerPage.tsx` modified, with `isScenarioB` guard. FR-003. |
| No shared `usePolling` in `@cbweb3/ui` | PASS | Each app implements its own hook. FR-043, Assumption 5. |
| LP Positions table does not poll backend | PASS | Session-only table from Add Liquidity responses. FR-024, Clarification note. |
| Build-time flag only | PASS | `isScenarioB = import.meta.env.VITE_SCENARIO === "b"`. FR-001. |
| Integer-string amounts | PASS | Enforced in all forms and displays. FR-040. |

**Result**: All gates PASS. No violations to justify.

---

## Project Structure

### Documentation (this feature)

```text
.agent/specs/scenario-b-frontend-integration/
├── plan.md              ← This file
├── research.md          ← Phase 0 output
├── data-model.md        ← Phase 1 output
├── quickstart.md        ← Phase 1 output
├── contracts/           ← Phase 1 output (API contracts)
│   ├── bank-api-v2.md
│   └── governance-api-v2.md
└── tasks.md             ← Phase 2 output (NOT created by /speckit.plan)
```

### Source Code — bank app

```text
frontend/apps/bank/src/
  config/
    scenario.ts                       ← NEW: export const isScenarioB
  types/
    bridge.types.ts                   ← NEW: BridgedAssetPosition, BridgeState consts
    amm-v2.types.ts                   ← NEW: AMMQuote, SwapOrder, PoolStatus, ApproveAmmRequest
  hooks/
    usePolling.ts                     ← NEW: setInterval+cleanup hook (app-local)
  services/api/
    bridge.api.ts                     ← NEW: /api/v2/bridge/* calls
    amm-v2.api.ts                     ← NEW: /api/v2/amm/* real calls (quotes, swap, pool, approve)
    circuit-breaker-status.api.ts     ← NEW: /api/v2/governance/circuit-breaker/status (read-only)
  features/
    bridge/
      bridge.store.ts                 ← NEW: Zustand store for positions
      BridgePage.tsx                  ← NEW: Lock&Mint + Burn&Unlock + positions table
    amm/
      amm-v2.store.ts                 ← NEW: Zustand store for v2 quotes/swaps/pool/circuit-breaker
      AMMTradingPage.tsx              ← MODIFIED: isScenarioB branch (v2 when true, Scenario A when false)
  components/layout/
    Sidebar.tsx                       ← MODIFIED: isScenarioB → Scenario B nav items only
  routes/
    index.tsx                         ← MODIFIED: isScenarioB → Scenario B route tree
```

### Source Code — governance app

```text
frontend/apps/governance/src/
  config/
    scenario.ts                       ← NEW: export const isScenarioB
  types/
    liquidity.types.ts                ← NEW: LiquidityPosition, PoolStatus consts
    circuit-breaker-v2.types.ts       ← NEW: CircuitBreakerV2Status, resume/pause consts
    oversight.types.ts                ← NEW: DisclosureRequest, DisclosureState consts
  hooks/
    usePolling.ts                     ← NEW: setInterval+cleanup hook (app-local)
  services/api/
    liquidity.api.ts                  ← NEW: /api/v2/amm/liquidity/* and pool status
    circuit-breaker-v2.api.ts         ← NEW: /api/v2/governance/circuit-breaker/*
    oversight.api.ts                  ← NEW: /api/v2/oversight/*
  features/
    liquidity/
      liquidity.store.ts              ← NEW: Zustand store for pool status + LP positions (session)
      LiquidityManagementPage.tsx     ← NEW: Pool status card + add/remove LP + MintAndApprove
    circuit-breaker/
      circuit-breaker-v2.store.ts     ← NEW: Zustand store for v2 CB state
      CircuitBreakerPage.tsx          ← MODIFIED: isScenarioB branch (v2 multi-party when true, v1 when false)
    oversight/
      oversight.store.ts              ← NEW: Zustand store for disclosure requests
      OversightPage.tsx               ← NEW: Open/Sign/Status disclosure request
  components/layout/
    Sidebar.tsx                       ← MODIFIED: isScenarioB → Scenario B nav items only
  routes/
    index.tsx                         ← MODIFIED: isScenarioB → Scenario B route tree
```

**Structure Decision**: Files co-located in `features/<domain>/` for pages and stores (matching spec layout). API services in `services/api/` and types in `types/` (matching existing Scenario A pattern). This mirrors the Scenario A layout exactly to minimise cognitive overhead.

---

## Milestones

### M1 — Shared Infrastructure (scenario flag, types, polling hook)

**Goal**: Both apps have the build-time scenario flag, all domain types, and the `usePolling` hook. No runtime code yet.

**Files — bank app**:
- `src/config/scenario.ts` — `export const isScenarioB = import.meta.env.VITE_SCENARIO === "b";`
- `src/types/bridge.types.ts` — `BRIDGE_STATE` const obj + `BridgeState` union; `BridgedAssetPosition` interface; `LockMintRequest`; `BurnUnlockRequest`
- `src/types/amm-v2.types.ts` — `AMMQuote`, `SwapOrder`, `PoolStatus`, `SWAP_ERROR` const obj + union, `ApproveAmmRequest`, `SwapRequest`
- `src/hooks/usePolling.ts` — `usePolling(callback: () => void, intervalMs: number, enabled: boolean, maxDurationMs?: number): void`; `setInterval` + `clearInterval` + `setTimeout` stop after `maxDurationMs`; cleanup on unmount

**Files — governance app**:
- `src/config/scenario.ts` — same flag
- `src/types/liquidity.types.ts` — `LiquidityPosition`, `PoolStatus`, `LP_STATUS` const obj + union; `AddLiquidityRequest`; `RemoveLiquidityRequest`; `MintAndApproveRequest`
- `src/types/circuit-breaker-v2.types.ts` — `CB_STATE` const obj + union (`LIVE`, `HALTED`, `RESUME_PENDING`); `CircuitBreakerV2Status`; `PauseRequest`; `ProposeResumeRequest`; `ProposeResumeResponse`; `SignResumeRequest`
- `src/types/oversight.types.ts` — `DISCLOSURE_STATE` const obj + union (`PENDING`, `QUORUM_REACHED`, `EXPIRED`, `REJECTED`); `DisclosureRequest`; `OpenDisclosureRequest`; `SignDisclosureRequest`
- `src/hooks/usePolling.ts` — identical implementation (app-local copy)

**Type pattern** (FR-042):
```ts
export const BRIDGE_STATE = {
  LOCKING: "LOCKING",
  ACTIVE: "ACTIVE",
  BURNED: "BURNED",
  UNLOCKED: "UNLOCKED",
  RECONCILIATION_REQUIRED: "RECONCILIATION_REQUIRED",
  FAILED: "FAILED",
} as const;
export type BridgeState = (typeof BRIDGE_STATE)[keyof typeof BRIDGE_STATE];
```

**Verification**: `tsc --noEmit` passes in both apps. No import errors.

---

### M2 — Bank App: API Services

**Goal**: All bank-side Scenario B API modules call real `/api/v2` endpoints via the existing `httpClient`.

**Files**:

`src/services/api/bridge.api.ts`
```ts
import { httpClient } from "./http-client";
export const bridgeApi = {
  lockMint: (payload: LockMintRequest) =>
    httpClient.post<BridgedAssetPosition>("/api/v2/bridge/lock-mint", payload).then(r => r.data),
  burnUnlock: (payload: BurnUnlockRequest) =>
    httpClient.post<{ position_id: string; bridge_state: BridgeState }>("/api/v2/bridge/burn-unlock", payload).then(r => r.data),
  listPositions: () =>
    httpClient.get<BridgedAssetPosition[]>("/api/v2/bridge/positions").then(r => r.data),
};
```

`src/services/api/amm-v2.api.ts`
```ts
export const ammV2Api = {
  getQuote: (pair: string, amount_out: string) =>
    httpClient.get<AMMQuote>("/api/v2/amm/quote/exact-output", { params: { pair, amount_out } }).then(r => r.data),
  executeSwap: (payload: SwapRequest) =>
    httpClient.post<SwapOrder>("/api/v2/amm/swap/exact-output", payload).then(r => r.data),
  getPoolStatus: (pair: string) =>
    httpClient.get<PoolStatus>(`/api/v2/amm/pool/${pair}/status`).then(r => r.data),
  approveAmm: (payload: ApproveAmmRequest) =>
    httpClient.post<{ status: string }>("/api/v2/amm/token/approve-amm", payload).then(r => r.data),
};
```

`src/services/api/circuit-breaker-status.api.ts` *(read-only, bank-side halted banner)*
```ts
export const cbStatusApi = {
  getStatus: (pair: string) =>
    httpClient.get<{ state: string; resume_request_id?: string }>("/api/v2/governance/circuit-breaker/status", { params: { pair } }).then(r => r.data),
};
```

**Verification**: TypeScript compiles. Import paths resolve.

---

### M3 — Bank App: Stores

**Goal**: Zustand stores for bridge positions and AMM v2 flows exist and wire to the API services.

`src/features/bridge/bridge.store.ts`
- State: `positions: BridgedAssetPosition[]`, `status: "idle" | "loading" | "error"`, `error: string | null`
- Actions: `loadPositions()`, `submitLockMint(payload: LockMintRequest)`, `submitBurnUnlock(payload: BurnUnlockRequest)`
- `submitLockMint`: calls `bridgeApi.lockMint`, prepends returned position to `positions`
- `submitBurnUnlock`: calls `bridgeApi.burnUnlock`, updates matching position in `positions` by `position_id`
- `loadPositions`: calls `bridgeApi.listPositions`, sets `positions`

`src/features/amm/amm-v2.store.ts`
- State: `quote: AMMQuote | null`, `quoteTimestamp: number | null`, `swapResult: SwapOrder | null`, `poolStatus: PoolStatus | null`, `circuitBreakerState: string | null`, `status: "idle" | "loading" | "error"`, `error: string | null`
- Actions: `fetchQuote(pair: string, amount_out: string)`, `executeSwap(payload: SwapRequest)`, `fetchPoolStatus(pair: string)`, `fetchCircuitBreakerState(pair: string)`, `clearQuote()`
- `fetchQuote`: sets `quote` and `quoteTimestamp = Date.now()`
- Quote staleness is derived in the component: `isQuoteStale = Date.now() - quoteTimestamp > 10_000`
- Swap error: store sets `error` to the API error code string (e.g. `"SLIPPAGE_LIMIT_EXCEEDED"`); page maps to user message

**Verification**: `tsc --noEmit` passes.

---

### M4 — Bank App: Pages, Sidebar & Routes

**Goal**: Scenario B reachable in the `bank` app. Bridge page and modified AMM page render. Scenario A routes absent when `VITE_SCENARIO=b`.

#### `src/components/layout/Sidebar.tsx` (MODIFIED)
- Add `import { isScenarioB } from "../../config/scenario"` at top
- Add `scenarioBLinks` array: `[{ to: "/", Dashboard }, { to: "/bridge", Bridge, icon: ArrowLeftRight }, { to: "/amm", "Automated FX Trading", icon: Scale }]`
- Render `isScenarioB ? scenarioBLinks : links` (original `links` array preserved)

#### `src/routes/index.tsx` (MODIFIED)
- Import `isScenarioB` and `BridgePage`
- Define `scenarioBChildren` array: `[DashboardPage, BridgePage at "/bridge", AMMTradingPage at "/amm", ComplianceCenterPage, OnboardingPage, SettingsPage]`
- Assign `children: isScenarioB ? scenarioBChildren : scenarioAChildren` inside `AppLayout` route
- Scenario A routes (HTLC, FX Agreements, Deposits, Escrows, Redeems, Liquidity) NOT registered when `isScenarioB`

#### `src/features/bridge/BridgePage.tsx` (NEW)
- Lock & Mint form: `owner_bank_id`, `spoke_network`, `native_asset`, `mirrored_asset` (optional), `amount` (integer string)
- Burn & Unlock form: `position_id` input
- Bridge Positions table: `position_id`, `owner_bank_id`, `spoke_network`, `native_asset`, `bridge_state` badge, `mirrored_amount`, `relayer_retries`
- State filter dropdown: client-side filter, does NOT change API call
- Polling: `usePolling(store.loadPositions, 5000, hasNonTerminal, 120_000)` where `hasNonTerminal = positions.some(p => ["LOCKING","ACTIVE","BURNED"].includes(p.bridge_state))`
- Polling timeout notice on affected rows (from 120s `maxDurationMs`)
- State badge variants: `LOCKING` → default/neutral, `ACTIVE` → success (green), `BURNED` → secondary (blue), `UNLOCKED` → outline/muted, `RECONCILIATION_REQUIRED` → destructive (red), `FAILED` → destructive (red)

#### `src/features/amm/AMMTradingPage.tsx` (MODIFIED)
- Add `import { isScenarioB } from "../../config/scenario"` at top of file
- Wrap existing return JSX: `return isScenarioB ? <AMMTradingV2 /> : <existing JSX>`
- `AMMTradingV2` inline component (same file, no new file):
  - Circuit-breaker halted banner (polls via `ammV2Store.fetchCircuitBreakerState`, `usePolling(..., 15000, true)`)
  - Pool imbalance banner (from `poolStatus?.imbalance_flag`)
  - Quote form: `pair` (default `BRL-USD`), `amount_out` → displays `required_input`, `price_impact` (as `${(price_impact * 100).toFixed(2)}%`), `quote_timestamp`; stale warning `> 10s`
  - Swap form: pre-fills `pair`+`amount_out` from quote; `max_amount_in`, `payer_id`, `beneficiary_id`
  - Swap error messages per error code:
    - `SLIPPAGE_LIMIT_EXCEEDED` → "Market moved — retry with updated quote" + Refresh Quote button
    - `INSUFFICIENT_POOL_LIQUIDITY` → "Insufficient pool liquidity — contact Central Bank"
    - `ZK_VALIDATION_FAILED` → "ZK validation failed — check ZK pointer field"
    - `CIRCUIT_BREAKER_HALTED` → "Swaps temporarily suspended by Central Bank" (also disables Submit)
  - Approve AMM collapsible panel: `amount_a`, `amount_b` → `POST /api/v2/amm/token/approve-amm`

**Verification**: `tsc --noEmit`. Build with `VITE_SCENARIO=b` and without. Scenario A renders unchanged.

---

### M5 — Governance App: API Services

**Goal**: All governance-side Scenario B API modules call real `/api/v2` endpoints.

`src/services/api/liquidity.api.ts`
```ts
export const liquidityApi = {
  getPoolStatus: (pair: string) =>
    httpClient.get<PoolStatus>(`/api/v2/amm/pool/${pair}/status`).then(r => r.data),
  addLiquidity: (payload: AddLiquidityRequest) =>
    httpClient.post<LiquidityPosition>("/api/v2/amm/liquidity/add", payload).then(r => r.data),
  removeLiquidity: (payload: RemoveLiquidityRequest) =>
    httpClient.post<{ status: string }>("/api/v2/amm/liquidity/remove", payload).then(r => r.data),
  mintAndApprove: (payload: MintAndApproveRequest) =>
    httpClient.post<{ status: string }>("/api/v2/amm/token/mint-and-approve", payload).then(r => r.data),
};
```

`src/services/api/circuit-breaker-v2.api.ts`
```ts
export const circuitBreakerV2Api = {
  getStatus: (pair: string) =>
    httpClient.get<CircuitBreakerV2Status>("/api/v2/governance/circuit-breaker/status", { params: { pair } }).then(r => r.data),
  pause: (payload: PauseRequest) =>
    httpClient.post<CircuitBreakerV2Status>("/api/v2/governance/circuit-breaker/pause", payload).then(r => r.data),
  proposeResume: (payload: ProposeResumeRequest) =>
    httpClient.post<ProposeResumeResponse>("/api/v2/governance/circuit-breaker/resume-request", payload).then(r => r.data),
  signResume: (payload: SignResumeRequest) =>
    httpClient.post<CircuitBreakerV2Status>("/api/v2/governance/circuit-breaker/resume-sign", payload).then(r => r.data),
};
```

`src/services/api/oversight.api.ts`
```ts
export const oversightApi = {
  openDisclosure: (payload: OpenDisclosureRequest) =>
    httpClient.post<DisclosureRequest>("/api/v2/oversight/disclosure-request", payload).then(r => r.data),
  signDisclosure: (payload: SignDisclosureRequest) =>
    httpClient.post<{ status: string }>("/api/v2/oversight/disclosure-sign", payload).then(r => r.data),
  getDisclosureStatus: (requestId: string) =>
    httpClient.get<DisclosureRequest>(`/api/v2/oversight/disclosure-status/${requestId}`).then(r => r.data),
};
```

**Verification**: `tsc --noEmit` passes in governance app.

---

### M6 — Governance App: Stores

**Goal**: Zustand stores for liquidity, circuit breaker v2, and oversight wire to the new API services.

`src/features/liquidity/liquidity.store.ts`
- State: `poolStatus: PoolStatus | null`, `lpPositions: LiquidityPosition[]`, `status: "idle" | "loading" | "error"`, `error: string | null`
- Actions: `fetchPoolStatus(pair)`, `addLiquidity(payload)` — appends returned position to session-only `lpPositions`, `removeLiquidity(payload)` — updates matching item's `status` to `"WITHDRAWN"` on success, `mintAndApprove(payload)`
- LP positions are NOT fetched from backend; `lpPositions` populated only from `addLiquidity` responses

`src/features/circuit-breaker/circuit-breaker-v2.store.ts`
- State: `cbStatus: CircuitBreakerV2Status | null`, `resumeRequestId: string | null`, `status: "idle" | "loading" | "error"`, `error: string | null`
- Actions: `fetchStatus(pair)`, `pause(payload)` — updates `cbStatus` from response, `proposeResume(payload)` — stores `request_id` in `resumeRequestId`, `signResume(payload)` — updates `cbStatus` from response

`src/features/oversight/oversight.store.ts`
- State: `disclosures: DisclosureRequest[]`, `currentDisclosure: DisclosureRequest | null`, `status: "idle" | "loading" | "error"`, `error: string | null`
- Actions: `openDisclosure(payload)` — prepends returned request to `disclosures`, `signDisclosure(payload)`, `fetchDisclosureStatus(requestId)` — sets `currentDisclosure`

**Verification**: `tsc --noEmit` passes.

---

### M7 — Governance App: Pages, Sidebar & Routes

**Goal**: Scenario B reachable in `governance` app. New pages render. v2 Circuit Breaker flow works. Scenario A routes absent when `VITE_SCENARIO=b`.

#### `src/components/layout/Sidebar.tsx` (MODIFIED)
- Import `isScenarioB` from `../../config/scenario`
- Add `scenarioBNavItems` array: `[{ to: "/", Dashboard }, { to: "/liquidity", "Liquidity Management" }, { to: "/circuit-breaker", "Circuit Breaker" }, { to: "/oversight", Oversight }]`
- Render `isScenarioB ? scenarioBNavItems : navItems` (existing `navItems` preserved)
- Circuit-breaker badge: when `isScenarioB`, derive `isHalted` from `circuitBreakerV2Store.cbStatus?.state === CB_STATE.HALTED`; when `!isScenarioB`, existing v1 `useCircuitBreaker()` hook unchanged

#### `src/routes/index.tsx` (MODIFIED)
- Import `isScenarioB`, `LiquidityManagementPage`, `OversightPage`
- Define `scenarioBChildren` array: `[DashboardPage, LiquidityManagementPage at "/liquidity", CircuitBreakerPage at "/circuit-breaker", OversightPage at "/oversight", SettingsPage]`
- Assign `children: isScenarioB ? scenarioBChildren : scenarioAChildren`
- Scenario A routes (HTLCMonitor, DepositsApproval, EscrowsApproval, RedeemsApproval, Registry, Accounts, Parameters, Audit) NOT registered when `isScenarioB`

#### `src/features/liquidity/LiquidityManagementPage.tsx` (NEW)
- Pool Status card: `reserve_a`, `reserve_b`, `current_ratio`, `updated_at`; imbalance banner when `imbalance_flag = true`
- Polling: `usePolling(store.fetchPoolStatus.bind(null, "BRL-USD"), 15000, true)`
- Add Liquidity form: `pool_pair` (default `BRL-USD`), `token_a_amount`, `token_b_amount`, `provider_bank_id` (all integer strings)
- Remove Liquidity form: `lp_id`, `pool_pair`, `provider_bank_id`; error "Position not found or already withdrawn" on 404
- LP Positions table (session-only): `lp_id`, `pool_pair`, `provider_bank_id`, `token_a_amount`, `token_b_amount`, `lp_shares`, `status`, `added_at`
- MintAndApprove collapsible panel: `amount_a`, `amount_b`, optional `recipient`

#### `src/features/circuit-breaker/CircuitBreakerPage.tsx` (MODIFIED)
- Add `import { isScenarioB } from "../../config/scenario"` at top
- Wrap existing return JSX: `return isScenarioB ? <CircuitBreakerV2 /> : <existing JSX>`
- `CircuitBreakerV2` inline component (same file):
  - State card: `cbStatus.state` badge (`LIVE` → default/success, `HALTED` → destructive, `RESUME_PENDING` → warning/yellow)
  - When `RESUME_PENDING`: show `cbStatus.resume_request_id` prominently for co-signing
  - Polling: `usePolling(store.fetchStatus.bind(null, "BRL-USD"), 15000, true)`
  - Pause form: `pair` (default `BRL-USD`), `bank_id`, `reason_code`, `signature` (text input, label "Institutional Signature (base64)", default value `"AA="`)
  - Propose Resume form: `pair`, `bank_id`, `signature` (same label/default); on success, display `request_id` prominently
  - Sign Resume form: `pair`, `request_id`, `bank_id`, `signature` (same label/default)
  - `RESUME_PENDING` + additional signatures note: "Waiting for additional co-signatures"

#### `src/features/oversight/OversightPage.tsx` (NEW)
- Open Disclosure Request form: `tx_ref`, `requestor_id`, `reason_code`; on success display full `DisclosureRequest` object
- Sign Disclosure form: `request_id`, `signer_id`; 422 error shown inline ("Already signed or invalid request")
- Disclosure Status lookup: `request_id` input → shows state badge, quorum progress (`${quorum_reached}/${quorum_required}`), `expires_at`, `closed_at`
- State badge variants: `PENDING` → default/neutral, `QUORUM_REACHED` → default/success, `EXPIRED` → outline/muted, `REJECTED` → destructive/red

**Verification**: `tsc --noEmit` passes in governance app. Build with and without `VITE_SCENARIO=b`. Scenario A governance routes unreachable when flag set.

---

## Verification Plan

Run after each milestone and at completion of all milestones:

```bash
# TypeScript checks
cd frontend/apps/bank && npx tsc --noEmit
cd frontend/apps/governance && npx tsc --noEmit

# Scenario B build smoke tests
cd frontend/apps/bank && VITE_SCENARIO=b npx vite build 2>&1 | grep -E "^.*(error|Error)"
cd frontend/apps/governance && VITE_SCENARIO=b npx vite build 2>&1 | grep -E "^.*(error|Error)"

# Scenario A builds remain unaffected
cd frontend/apps/bank && npx vite build 2>&1 | grep -E "^.*(error|Error)"
cd frontend/apps/governance && npx vite build 2>&1 | grep -E "^.*(error|Error)"
```

**Passing looks like**: Zero TypeScript errors. Vite builds complete. No `TS2307`, `TS2345`, `TS18046`, or `TS2304` errors.

**Manual verification for Scenario B** (with `VITE_SCENARIO=b` build):
- Bank: `/bridge` renders Lock & Mint + Burn & Unlock forms + positions table
- Bank: `/amm` renders v2 quote form + circuit-breaker banner area + approve panel
- Bank: `/deposits`, `/escrows`, `/htlc`, `/agreements` return 404/redirect (not registered)
- Governance: `/liquidity` renders pool status card + LP forms + session table
- Governance: `/circuit-breaker` renders v2 state machine (pause/resume forms)
- Governance: `/oversight` renders disclosure request + sign + status panels
- Governance: `/deposits-approval`, `/escrows-approval`, `/htlc-monitor` return 404/redirect

**Manual verification for Scenario A** (no flag / `VITE_SCENARIO=a`):
- All existing Scenario A routes render unchanged in both apps
- No runtime import errors for Scenario B modules

---

## Rollback / Safety

All Scenario B code is additive. Rollback options:
1. Revert the branch: `git checkout main`
2. Set `VITE_SCENARIO=a` (or unset) at Docker Compose build time — Scenario B is completely unreachable without the flag; tree-shaking eliminates its bundle

The `!isScenarioB` branches in `AMMTradingPage.tsx` and `CircuitBreakerPage.tsx` render the original content unchanged. Zero Scenario A behaviour is altered.

---

## Decision Log

- **2026-05-07**: Scenario flag in `src/config/scenario.ts` per app (not shared) — ensures each app's bundle tree-shakes independently and keeps the shared `@cbweb3/ui` package free of build-time env assumptions.
- **2026-05-07**: `usePolling` duplicated between apps (not shared) — per spec clarification, must NOT be added to `@cbweb3/ui`. Cost of duplication is low (< 20 lines).
- **2026-05-07**: LP Positions table is session-only from Add Liquidity responses — endpoint `GET /api/v2/amm/liquidity/positions` does not exist; confirmed in spec clarification.
- **2026-05-07**: `AMMTradingPage.tsx` and `CircuitBreakerPage.tsx` modified in-place — per spec clarifications (no `*V2.tsx` files). The `isScenarioB` guard ensures zero Scenario A regression.
- **2026-05-07**: Bank app needs a `circuit-breaker-status.api.ts` (read-only GET) to display the halted banner on the AMM page. This is separate from governance's `circuit-breaker-v2.api.ts` which includes mutation operations.
- **2026-05-07**: Routing strategy uses a ternary on `isScenarioB` to select the entire `children` array, rather than filtering individual routes, to avoid accidental partial registration.
- **2026-05-07**: Governance Sidebar circuit-breaker badge uses v2 store when `isScenarioB` (not the v1 `useCircuitBreaker` hook) so the badge reflects v2 multi-party state (`LIVE`/`HALTED`/`RESUME_PENDING`).

---

## Surprises & Discoveries

- **2026-05-07**: Bank app's `amm.api.ts` is entirely mock-backed — Scenario B `amm-v2.api.ts` is a wholly new file; existing `useAmmStore` / `ammApi` remain untouched.
- **2026-05-07**: Governance app already has `useCircuitBreaker` hook wrapping the v1 store. The v2 store (`circuit-breaker-v2.store.ts`) is completely independent — no coupling to v1.
- **2026-05-07**: Setup script requires numeric-prefixed branch names (`001-*`); current branch `feature/scenario-b-frontend` bypassed via `SPECIFY_FEATURE_DIRECTORY` + `SPECIFY_FEATURE=005-scenario-b-frontend` env overrides.

---

## Progress

- [ ] M1: Shared infrastructure — scenario flag, types, usePolling hook (both apps)
- [ ] M2: Bank app — API services (bridge.api.ts, amm-v2.api.ts, circuit-breaker-status.api.ts)
- [ ] M3: Bank app — Stores (bridge.store.ts, amm-v2.store.ts)
- [ ] M4: Bank app — Pages, sidebar, routes (BridgePage new, AMMTradingPage modified)
- [ ] M5: Governance app — API services (liquidity.api.ts, circuit-breaker-v2.api.ts, oversight.api.ts)
- [ ] M6: Governance app — Stores (liquidity.store.ts, circuit-breaker-v2.store.ts, oversight.store.ts)
- [ ] M7: Governance app — Pages, sidebar, routes (LiquidityManagementPage + OversightPage new, CircuitBreakerPage modified)
