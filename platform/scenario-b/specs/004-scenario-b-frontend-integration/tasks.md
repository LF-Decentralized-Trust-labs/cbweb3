# Tasks: Scenario B Frontend Integration

**Feature**: `scenario-b-frontend-integration`
**Branch**: `feature/scenario-b-frontend`
**Generated**: 2026-05-07
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

---

## Summary

| Phase | Milestone | Tasks | Parallelisable |
|-------|-----------|-------|----------------|
| Phase 1 | M1 — Shared Infrastructure | T001–T010 | Yes (bank + governance in parallel) |
| Phase 2 | M2 — Bank API Services | T011–T013 | Yes (all 3 files independent) |
| Phase 3 | M3 — Bank Stores | T014–T015 | Yes (bridge + amm-v2 independent) |
| Phase 4 | M4 — Bank Pages, Sidebar & Routes | T016–T019 | T017/T018 in parallel; T016 blocks T017 |
| Phase 5 | M5 — Governance API Services | T020–T022 | Yes (all 3 files independent) |
| Phase 6 | M6 — Governance Stores | T023–T025 | Yes (all 3 independent) |
| Phase 7 | M7 — Governance Pages, Sidebar & Routes | T026–T030 | T027/T028 in parallel; T029 after T027 |

**Total tasks**: 30
**Verification command (all phases)**: `cd frontend/apps/bank && npx tsc --noEmit && cd ../governance && npx tsc --noEmit`

---

## Dependency Graph

```
M1 (T001–T010)
  └── M2 (T011–T013)   — bank API services (need bank types from M1)
        └── M3 (T014–T015)  — bank stores (need API modules from M2)
              └── M4 (T016–T019)  — bank pages (need stores from M3)
  └── M5 (T020–T022)   — governance API services (need governance types from M1)
        └── M6 (T023–T025)  — governance stores (need API modules from M5)
              └── M7 (T026–T030)  — governance pages (need stores from M6)
```

---

## Phase 1 — M1: Shared Infrastructure

**Goal**: Both apps have the build-time scenario flag (`isScenarioB`), all domain types, and the `usePolling` hook. No runtime UI code yet.
**Verification**: `cd frontend/apps/bank && npx tsc --noEmit && cd ../governance && npx tsc --noEmit` — both exit 0.

---

- [ ] T001 Create scenario flag for bank app in `frontend/apps/bank/src/config/scenario.ts`

  **File to create**: `frontend/apps/bank/src/config/scenario.ts`

  **Implementation**:
  ```ts
  export const isScenarioB = import.meta.env.VITE_SCENARIO === "b";
  ```
  No other exports. The `config/` directory already exists (contains `query-client.ts`).

  **Acceptance**: File exists; `npx tsc --noEmit` in `frontend/apps/bank` passes.

---

- [ ] T002 Create bridge domain types for bank app in `frontend/apps/bank/src/types/bridge.types.ts`

  **File to create**: `frontend/apps/bank/src/types/bridge.types.ts`

  **Pattern**: const-object + union type alias (same as `frontend/apps/bank/src/types/payment.types.ts`). No TypeScript enums.

  **Implementation**:
  ```ts
  export const BRIDGE_STATE = {
    LOCKING:                 "LOCKING",
    ACTIVE:                  "ACTIVE",
    BURNED:                  "BURNED",
    UNLOCKED:                "UNLOCKED",
    RECONCILIATION_REQUIRED: "RECONCILIATION_REQUIRED",
    FAILED:                  "FAILED",
  } as const;
  export type BridgeState = (typeof BRIDGE_STATE)[keyof typeof BRIDGE_STATE];

  export interface BridgedAssetPosition {
    position_id:       string;
    owner_bank_id:     string;
    spoke_network:     string;
    native_asset:      string;
    mirrored_asset:    string;
    mirrored_amount:   string;        // integer string
    bridge_state:      BridgeState;
    relayer_retries:   number;
    relayer_error_log: string | null;
    created_at:        string;        // ISO 8601
    updated_at:        string;        // ISO 8601
  }

  export interface LockMintRequest {
    owner_bank_id:   string;
    spoke_network:   string;
    native_asset:    string;
    mirrored_asset?: string;   // optional
    amount:          string;   // integer string
  }

  export interface BurnUnlockRequest {
    position_id: string;
  }
  ```

  **Acceptance**: `npx tsc --noEmit` passes. Badge color mapping reference:
  - `LOCKING` → `default`, `ACTIVE` → `success`, `BURNED` → `secondary`, `UNLOCKED` → `outline`, `RECONCILIATION_REQUIRED` / `FAILED` → `destructive`

---

- [ ] T003 Create AMM v2 domain types for bank app in `frontend/apps/bank/src/types/amm-v2.types.ts`

  **File to create**: `frontend/apps/bank/src/types/amm-v2.types.ts`

  **Implementation**:
  ```ts
  export const SWAP_ERROR = {
    SLIPPAGE_LIMIT_EXCEEDED:     "SLIPPAGE_LIMIT_EXCEEDED",
    INSUFFICIENT_POOL_LIQUIDITY: "INSUFFICIENT_POOL_LIQUIDITY",
    ZK_VALIDATION_FAILED:        "ZK_VALIDATION_FAILED",
    CIRCUIT_BREAKER_HALTED:      "CIRCUIT_BREAKER_HALTED",
  } as const;
  export type SwapError = (typeof SWAP_ERROR)[keyof typeof SWAP_ERROR];

  export interface AMMQuote {
    pair:            string;
    amount_out:      string;   // integer string
    required_input:  string;   // integer string
    price_impact:    number;   // 0..1 decimal; display as `${(v * 100).toFixed(2)}%`
    quote_timestamp: string;   // ISO 8601
  }

  export interface SwapOrder {
    order_id:     string;
    tx_hash:      string;
    amount_in:    string;   // integer string
    amount_out:   string;   // integer string
    state:        string;
    confirmed_at: string;   // ISO 8601
  }

  export interface PoolStatus {
    pool_pair:      string;
    reserve_a:      string;    // integer string
    reserve_b:      string;    // integer string
    current_ratio:  string;    // decimal string
    imbalance_flag: boolean;
    updated_at:     string;    // ISO 8601
  }

  export interface SwapRequest {
    pair:           string;
    amount_out:     string;   // integer string
    max_amount_in:  string;   // integer string (slippage-adjusted)
    payer_id:       string;
    beneficiary_id: string;
  }

  export interface ApproveAmmRequest {
    amount_a: string;   // integer string
    amount_b: string;   // integer string
  }
  ```

  **Staleness rule** (for implementer reference): `Date.now() - Date.parse(quote_timestamp) > 10_000`
  **Acceptance**: `npx tsc --noEmit` passes.

---

- [ ] T004 Create `usePolling` hook for bank app in `frontend/apps/bank/src/hooks/usePolling.ts`

  **File to create**: `frontend/apps/bank/src/hooks/usePolling.ts`

  **Important**: This hook is app-local. It MUST NOT be placed in `@cbweb3/ui` or any shared package.

  **Signature**:
  ```ts
  export function usePolling(
    callback: () => void,
    intervalMs: number,
    enabled: boolean,
    maxDurationMs?: number,
  ): void
  ```

  **Implementation requirements**:
  - Uses `setInterval` + `clearInterval` inside a `useEffect`
  - If `enabled` is `false`, effect is a no-op (does not start the interval)
  - If `maxDurationMs` is provided, a `setTimeout` fires after that duration and calls `clearInterval` to stop polling
  - Cleanup runs on component unmount (`return () => { clearInterval(id); clearTimeout(stopId); }`)
  - `callback` is a ref to avoid re-triggering the effect on every render — wrap in `useRef` and update on each render

  **Acceptance**: `npx tsc --noEmit` passes in `frontend/apps/bank`.

---

- [ ] T005 Create scenario flag for governance app in `frontend/apps/governance/src/config/scenario.ts`

  **File to create**: `frontend/apps/governance/src/config/scenario.ts`

  **Implementation** (identical to bank app):
  ```ts
  export const isScenarioB = import.meta.env.VITE_SCENARIO === "b";
  ```

  The `config/` directory already exists. **Acceptance**: `npx tsc --noEmit` in `frontend/apps/governance` passes.

---

- [ ] T006 Create liquidity domain types for governance app in `frontend/apps/governance/src/types/liquidity.types.ts`

  **File to create**: `frontend/apps/governance/src/types/liquidity.types.ts`

  **Implementation**:
  ```ts
  export const LP_STATUS = {
    ACTIVE:    "ACTIVE",
    WITHDRAWN: "WITHDRAWN",
  } as const;
  export type LpStatus = (typeof LP_STATUS)[keyof typeof LP_STATUS];

  export interface PoolStatus {
    pool_pair:      string;
    reserve_a:      string;   // integer string
    reserve_b:      string;   // integer string
    current_ratio:  string;   // decimal string
    imbalance_flag: boolean;
    updated_at:     string;   // ISO 8601
  }

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

  **Acceptance**: `npx tsc --noEmit` passes in `frontend/apps/governance`.

---

- [ ] T007 Create circuit-breaker v2 domain types for governance app in `frontend/apps/governance/src/types/circuit-breaker-v2.types.ts`

  **File to create**: `frontend/apps/governance/src/types/circuit-breaker-v2.types.ts`

  **Implementation**:
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
    resume_request_id: string | null;
  }

  export interface PauseRequest {
    pair:        string;
    bank_id:     string;
    reason_code: string;
    signature:   string;   // base64 text, default "AA="
  }

  export interface ProposeResumeRequest {
    pair:      string;
    bank_id:   string;
    signature: string;   // base64 text, default "AA="
  }

  export interface ProposeResumeResponse {
    request_id: string;
    state:      CbState;
  }

  export interface SignResumeRequest {
    pair:       string;
    request_id: string;
    bank_id:    string;
    signature:  string;   // base64 text, default "AA="
  }
  ```

  **State badge variants** (for implementer reference):
  - `LIVE` → `default` (success/green), `HALTED` → `destructive` (red), `RESUME_PENDING` → `warning` (yellow)

  **Acceptance**: `npx tsc --noEmit` passes in `frontend/apps/governance`.

---

- [ ] T008 Create oversight domain types for governance app in `frontend/apps/governance/src/types/oversight.types.ts`

  **File to create**: `frontend/apps/governance/src/types/oversight.types.ts`

  **Implementation**:
  ```ts
  export const DISCLOSURE_STATE = {
    PENDING:        "PENDING",
    QUORUM_REACHED: "QUORUM_REACHED",
    EXPIRED:        "EXPIRED",
    REJECTED:       "REJECTED",
  } as const;
  export type DisclosureState = (typeof DISCLOSURE_STATE)[keyof typeof DISCLOSURE_STATE];

  export interface DisclosureRequest {
    request_id:      string;
    state:           DisclosureState;
    quorum_reached:  number;
    quorum_required: number;
    expires_at:      string;   // ISO 8601
    closed_at:       string | null;   // ISO 8601
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

  **State badge variants** (for implementer reference):
  - `PENDING` → `default` (neutral), `QUORUM_REACHED` → `success` (green), `EXPIRED` → `outline` (muted), `REJECTED` → `destructive` (red)

  **Acceptance**: `npx tsc --noEmit` passes in `frontend/apps/governance`.

---

- [ ] T009 [P] Create `usePolling` hook for governance app in `frontend/apps/governance/src/hooks/usePolling.ts`

  **File to create**: `frontend/apps/governance/src/hooks/usePolling.ts`

  **Implementation**: Identical to the bank app hook (T004). App-local — MUST NOT be shared via `@cbweb3/ui`.

  **Signature**:
  ```ts
  export function usePolling(
    callback: () => void,
    intervalMs: number,
    enabled: boolean,
    maxDurationMs?: number,
  ): void
  ```

  Same requirements as T004: `useRef` for callback, `setInterval` + `clearInterval`, optional `setTimeout` for max-duration stop, cleanup on unmount.

  **Acceptance**: `npx tsc --noEmit` passes in `frontend/apps/governance`.

---

- [ ] T010 [P] Verify M1 TypeScript compilation passes for both apps

  **Action**: Run `npx tsc --noEmit` in both apps and confirm exit 0.

  ```bash
  cd frontend/apps/bank && npx tsc --noEmit
  cd frontend/apps/governance && npx tsc --noEmit
  ```

  Fix any type errors before proceeding to M2/M5.

---

## Phase 2 — M2: Bank App API Services

**Goal**: All bank-side Scenario B API modules exist and call real `/api/v2` endpoints via `httpClient`. No mocks.
**Dependencies**: M1 complete (types exist).
**Verification**: `cd frontend/apps/bank && npx tsc --noEmit`

---

- [ ] T011 [P] Create bridge API service for bank app in `frontend/apps/bank/src/services/api/bridge.api.ts`

  **File to create**: `frontend/apps/bank/src/services/api/bridge.api.ts`

  **Pattern**: Follow `frontend/apps/bank/src/services/api/amm.api.ts` but import `httpClient` directly (no `mockDb`). The `httpClient` is exported from `./http-client`.

  **Implementation**:
  ```ts
  import type { BridgedAssetPosition, BurnUnlockRequest, LockMintRequest } from "../../types/bridge.types";
  import type { BridgeState } from "../../types/bridge.types";
  import { httpClient } from "./http-client";

  export const bridgeApi = {
    lockMint: (payload: LockMintRequest) =>
      httpClient.post<BridgedAssetPosition>("/api/v2/bridge/lock-mint", payload).then((r) => r.data),
    burnUnlock: (payload: BurnUnlockRequest) =>
      httpClient.post<{ position_id: string; bridge_state: BridgeState }>("/api/v2/bridge/burn-unlock", payload).then((r) => r.data),
    listPositions: () =>
      httpClient.get<BridgedAssetPosition[]>("/api/v2/bridge/positions").then((r) => r.data),
  };
  ```

  **API contract**:
  - `POST /api/v2/bridge/lock-mint` → 201 `BridgedAssetPosition` (bridge_state: `LOCKING`)
  - `POST /api/v2/bridge/burn-unlock` → 200 `{ position_id, bridge_state: "BURNED" }`
  - `GET /api/v2/bridge/positions` → 200 `BridgedAssetPosition[]` (no `state` query param sent)

  **Acceptance**: `npx tsc --noEmit` passes. No mock imports.

---

- [ ] T012 [P] Create AMM v2 API service for bank app in `frontend/apps/bank/src/services/api/amm-v2.api.ts`

  **File to create**: `frontend/apps/bank/src/services/api/amm-v2.api.ts`

  **Implementation**:
  ```ts
  import type { AMMQuote, ApproveAmmRequest, PoolStatus, SwapOrder, SwapRequest } from "../../types/amm-v2.types";
  import { httpClient } from "./http-client";

  export const ammV2Api = {
    getQuote: (pair: string, amount_out: string) =>
      httpClient
        .get<AMMQuote>("/api/v2/amm/quote/exact-output", { params: { pair, amount_out } })
        .then((r) => r.data),
    executeSwap: (payload: SwapRequest) =>
      httpClient.post<SwapOrder>("/api/v2/amm/swap/exact-output", payload).then((r) => r.data),
    getPoolStatus: (pair: string) =>
      httpClient.get<PoolStatus>(`/api/v2/amm/pool/${pair}/status`).then((r) => r.data),
    approveAmm: (payload: ApproveAmmRequest) =>
      httpClient.post<{ status: string }>("/api/v2/amm/token/approve-amm", payload).then((r) => r.data),
  };
  ```

  **API contract**:
  - `GET /api/v2/amm/quote/exact-output?pair=BRL-USD&amount_out=1000` → 200 `AMMQuote`
  - `POST /api/v2/amm/swap/exact-output` → 200 `SwapOrder`
  - `GET /api/v2/amm/pool/:pair/status` → 200 `PoolStatus`
  - `POST /api/v2/amm/token/approve-amm` → 200 `{ status: "ok" }`

  **Acceptance**: `npx tsc --noEmit` passes. No mock imports.

---

- [ ] T013 [P] Create circuit-breaker status API service (read-only) for bank app in `frontend/apps/bank/src/services/api/circuit-breaker-status.api.ts`

  **File to create**: `frontend/apps/bank/src/services/api/circuit-breaker-status.api.ts`

  **Purpose**: Bank app reads circuit-breaker state to show the "Swaps temporarily suspended" banner on the AMM page. This is read-only — bank app cannot pause or resume.

  **Implementation**:
  ```ts
  import { httpClient } from "./http-client";

  export const cbStatusApi = {
    getStatus: (pair: string) =>
      httpClient
        .get<{ state: string; resume_request_id?: string }>("/api/v2/governance/circuit-breaker/status", {
          params: { pair },
        })
        .then((r) => r.data),
  };
  ```

  **Acceptance**: `npx tsc --noEmit` passes.

---

## Phase 3 — M3: Bank App Stores

**Goal**: Zustand stores for bridge positions and AMM v2 exist and wire to the API services created in M2.
**Dependencies**: M2 complete (API services exist).
**Verification**: `cd frontend/apps/bank && npx tsc --noEmit`

---

- [ ] T014 [P] Create bridge Zustand store for bank app in `frontend/apps/bank/src/features/bridge/bridge.store.ts`

  **Files to create**:
  - `frontend/apps/bank/src/features/bridge/bridge.store.ts` (create `features/bridge/` subdirectory)

  **Pattern**: Follow `frontend/apps/bank/src/stores/amm.store.ts` — `create<State>()` from Zustand, `set()` for state updates, async actions that call API then update state.

  **Implementation**:
  ```ts
  import { create } from "zustand";
  import { bridgeApi } from "../../services/api/bridge.api";
  import type { BridgedAssetPosition, BurnUnlockRequest, LockMintRequest } from "../../types/bridge.types";

  type BridgeState = {
    positions:       BridgedAssetPosition[];
    status:          "idle" | "loading" | "error";
    error:           string | null;
    loadPositions:   () => Promise<void>;
    submitLockMint:  (payload: LockMintRequest) => Promise<void>;
    submitBurnUnlock:(payload: BurnUnlockRequest) => Promise<void>;
  };

  export const useBridgeStore = create<BridgeState>((set, get) => ({
    positions: [],
    status:    "idle",
    error:     null,

    loadPositions: async () => {
      set({ status: "loading", error: null });
      try {
        const positions = await bridgeApi.listPositions();
        set({ positions, status: "idle" });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Failed to load positions" });
      }
    },

    submitLockMint: async (payload) => {
      set({ status: "loading", error: null });
      try {
        const position = await bridgeApi.lockMint(payload);
        set({ positions: [position, ...get().positions], status: "idle" });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Lock & Mint failed" });
      }
    },

    submitBurnUnlock: async (payload) => {
      set({ status: "loading", error: null });
      try {
        const result = await bridgeApi.burnUnlock(payload);
        set({
          positions: get().positions.map((p) =>
            p.position_id === result.position_id ? { ...p, bridge_state: result.bridge_state } : p,
          ),
          status: "idle",
        });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Burn & Unlock failed" });
      }
    },
  }));
  ```

  **Acceptance**: `npx tsc --noEmit` passes.

---

- [ ] T015 [P] Create AMM v2 Zustand store for bank app in `frontend/apps/bank/src/features/amm/amm-v2.store.ts`

  **Files to create**:
  - `frontend/apps/bank/src/features/amm/amm-v2.store.ts` (create `features/amm/` subdirectory)

  **Implementation**:
  ```ts
  import { create } from "zustand";
  import { ammV2Api } from "../../services/api/amm-v2.api";
  import { cbStatusApi } from "../../services/api/circuit-breaker-status.api";
  import type { AMMQuote, ApproveAmmRequest, PoolStatus, SwapOrder, SwapRequest } from "../../types/amm-v2.types";

  type AmmV2State = {
    quote:                AMMQuote | null;
    quoteTimestamp:       number | null;
    swapResult:           SwapOrder | null;
    poolStatus:           PoolStatus | null;
    circuitBreakerState:  string | null;
    status:               "idle" | "loading" | "error";
    error:                string | null;
    fetchQuote:           (pair: string, amount_out: string) => Promise<void>;
    executeSwap:          (payload: SwapRequest) => Promise<void>;
    fetchPoolStatus:      (pair: string) => Promise<void>;
    fetchCircuitBreakerState: (pair: string) => Promise<void>;
    approveAmm:           (payload: ApproveAmmRequest) => Promise<void>;
    clearQuote:           () => void;
  };

  export const useAmmV2Store = create<AmmV2State>((set) => ({
    quote:               null,
    quoteTimestamp:      null,
    swapResult:          null,
    poolStatus:          null,
    circuitBreakerState: null,
    status:              "idle",
    error:               null,

    fetchQuote: async (pair, amount_out) => {
      set({ status: "loading", error: null });
      try {
        const quote = await ammV2Api.getQuote(pair, amount_out);
        set({ quote, quoteTimestamp: Date.now(), status: "idle" });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Quote failed" });
      }
    },

    executeSwap: async (payload) => {
      set({ status: "loading", error: null });
      try {
        const swapResult = await ammV2Api.executeSwap(payload);
        set({ swapResult, status: "idle" });
      } catch (err) {
        // Preserve the API error code string for the page to map to user message
        const message = err instanceof Error ? err.message : "Swap failed";
        set({ status: "error", error: message });
      }
    },

    fetchPoolStatus: async (pair) => {
      try {
        const poolStatus = await ammV2Api.getPoolStatus(pair);
        set({ poolStatus });
      } catch {
        // silently preserve last known value; page shows stale warning
      }
    },

    fetchCircuitBreakerState: async (pair) => {
      try {
        const data = await cbStatusApi.getStatus(pair);
        set({ circuitBreakerState: data.state });
      } catch {
        // silently preserve last known value
      }
    },

    approveAmm: async (payload) => {
      set({ status: "loading", error: null });
      try {
        await ammV2Api.approveAmm(payload);
        set({ status: "idle" });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Approve failed" });
      }
    },

    clearQuote: () => set({ quote: null, quoteTimestamp: null }),
  }));
  ```

  **Quote staleness** (component-side): `isQuoteStale = Date.now() - (quoteTimestamp ?? 0) > 10_000`
  **Swap error mapping** (component-side): map `error` string to user message per SWAP_ERROR constants.
  **Acceptance**: `npx tsc --noEmit` passes.

---

## Phase 4 — M4: Bank App Pages, Sidebar & Routes

**Goal**: Scenario B is reachable in the `bank` app. Bridge page and modified AMM page render. Scenario A routes absent when `VITE_SCENARIO=b`.
**Dependencies**: M3 complete (stores exist). T016 (scenario.ts already created in T001) — but sidebar imports it.
**Verification**: `cd frontend/apps/bank && npx tsc --noEmit`. Build with and without `VITE_SCENARIO=b` and verify Scenario A renders unchanged.

---

- [ ] T016 Modify bank app Sidebar to show Scenario B navigation when `isScenarioB` in `frontend/apps/bank/src/components/layout/Sidebar.tsx`

  **File to modify**: `frontend/apps/bank/src/components/layout/Sidebar.tsx`

  **Changes**:
  1. Add import at top: `import { isScenarioB } from "../../config/scenario";`
  2. Add import for new icon: `ArrowLeftRight` is already imported; add `Bridge` label. Actually the icon for Bridge is `ArrowLeftRight` (already in imports). Add `ArrowLeftRight` if not already present.
  3. Define `scenarioBLinks` array above the `Sidebar` function:
     ```ts
     const scenarioBLinks = [
       { to: "/",       label: "Dashboard",           icon: LayoutDashboard },
       { to: "/bridge", label: "Bridge",               icon: ArrowLeftRight },
       { to: "/amm",    label: "Automated FX Trading", icon: Scale },
       { to: "/compliance", label: "Compliance",       icon: ShieldCheck },
       { to: "/onboarding", label: "Onboarding",       icon: ClipboardList },
       { to: "/settings",   label: "Settings",         icon: Settings },
     ];
     ```
  4. In the `Sidebar` component body, replace `{links.map(...)}` with `{(isScenarioB ? scenarioBLinks : links).map(...)}`.
  5. Do NOT change the existing `links` array or any existing JSX structure. The existing Scenario A nav is preserved in `links`.

  **Acceptance**: `npx tsc --noEmit` passes. With `VITE_SCENARIO=b`, sidebar shows Bridge and AMM Trading only (no HTLC, Deposits, etc.).

---

- [ ] T017 [P] [US2] Create Bridge page for bank app in `frontend/apps/bank/src/features/bridge/BridgePage.tsx`

  **File to create**: `frontend/apps/bank/src/features/bridge/BridgePage.tsx`

  **UI components**: Import from `@cbweb3/ui`: `Button`, `Card`, `CardContent`, `CardHeader`, `CardTitle`, `CardDescription`, `Badge`, `Input`, `Label`, `Select`, `Table`, `TableBody`, `TableCell`, `TableHead`, `TableHeader`, `TableRow`.

  **Layout sections** (three stacked cards):

  **1. Lock & Mint form**:
  - Fields (all text inputs): `owner_bank_id`, `spoke_network`, `native_asset`, `mirrored_asset` (optional), `amount` (integer string — do NOT use type="number")
  - Submit button: calls `store.submitLockMint(payload)`
  - Disable submit while `store.status === "loading"`
  - Show inline error if `store.error` after submission

  **2. Burn & Unlock form**:
  - Field: `position_id` (text input)
  - Submit button: calls `store.submitBurnUnlock({ position_id })`
  - Show inline error if `store.error` after submission

  **3. Bridge Positions table**:
  - State filter dropdown (`Select` from `@cbweb3/ui`): options All / LOCKING / ACTIVE / BURNED / UNLOCKED / RECONCILIATION_REQUIRED / FAILED. Filter is **client-side only** — does NOT change the API call.
  - Columns: `position_id`, `owner_bank_id`, `spoke_network`, `native_asset`, `bridge_state` (as `Badge`), `mirrored_amount`, `relayer_retries`
  - Load on mount: `store.loadPositions()` in `useEffect`
  - Polling: `usePolling(store.loadPositions, 5000, hasNonTerminal, 120_000)` where:
    ```ts
    const NON_TERMINAL = new Set(["LOCKING", "ACTIVE", "BURNED"]);
    const hasNonTerminal = store.positions.some((p) => NON_TERMINAL.has(p.bridge_state));
    ```
  - Polling timeout: when `maxDurationMs` elapses, show "⚠ Polling timeout — refresh manually" note on rows that were non-terminal at timeout. Implement with a `timedOutPositionIds` state (`Set<string>`): pass an `onTimeout` callback (or extend `usePolling` to accept one) that captures the current non-terminal position IDs.
  - Badge variants per `bridge_state`:
    - `LOCKING` → `default`, `ACTIVE` → `success`, `BURNED` → `secondary`, `UNLOCKED` → `outline`, `RECONCILIATION_REQUIRED` → `destructive`, `FAILED` → `destructive`
  - `RECONCILIATION_REQUIRED` rows: add a muted note "Contact operations team for manual reconciliation"

  **Polling timeout implementation note**: The simplest approach is to extend `usePolling` to accept an optional `onTimeout?: () => void` parameter added after `maxDurationMs`, called when the `setTimeout` fires. Then in `BridgePage.tsx`:
  ```ts
  const [timedOut, setTimedOut] = useState(false);
  usePolling(store.loadPositions, 5000, hasNonTerminal, 120_000, () => setTimedOut(true));
  ```

  **Acceptance**: `npx tsc --noEmit` passes. Page renders all three sections. Table polling starts/stops correctly.

---

- [ ] T018 [P] [US1] [US3] Modify AMMTradingPage to add Scenario B v2 branch in `frontend/apps/bank/src/pages/AMMTradingPage.tsx`

  **File to modify**: `frontend/apps/bank/src/pages/AMMTradingPage.tsx`

  **Changes** (additive — do NOT alter the existing Scenario A JSX):
  1. Add at the top: `import { isScenarioB } from "../config/scenario";`
  2. Add imports for new store and types: `import { useAmmV2Store } from "../features/amm/amm-v2.store";` and `import { SWAP_ERROR } from "../types/amm-v2.types";`
  3. Add `import { usePolling } from "../hooks/usePolling";`
  4. Add required `@cbweb3/ui` imports if not already present: `Badge`
  5. Wrap the existing `return (...)` with:
     ```tsx
     if (isScenarioB) {
       return <AMMTradingV2 />;
     }
     return <existing JSX unchanged>;
     ```
  6. Define `function AMMTradingV2()` in the **same file** (NOT a new file):

  **AMMTradingV2 component** — three sections in a single page layout:

  **Circuit-breaker halted banner**:
  - Polls `store.fetchCircuitBreakerState("BRL-USD")` via `usePolling(..., 15_000, true)`
  - When `store.circuitBreakerState === "HALTED"`, render a `destructive` alert card: "Swaps temporarily suspended by Central Bank"

  **Pool imbalance banner**:
  - When `store.poolStatus?.imbalance_flag === true`, render a `warning` alert card: "Pool imbalance detected — liquidity may be asymmetric"

  **Quote form** (Card):
  - Inputs: `pair` (default `BRL-USD`), `amount_out` (integer string)
  - Submit button: calls `store.fetchQuote(pair, amount_out)` 
  - Display when quote present: `required_input`, `price_impact` as `${(quote.price_impact * 100).toFixed(2)}%`, `quote_timestamp`
  - Staleness check: `Date.now() - Date.parse(store.quote.quote_timestamp) > 10_000` → show warning badge "Quote stale — click Refresh Quote" + Refresh Quote button (calls `store.fetchQuote` again)
  - Refresh Quote button also available at all times below the quote display

  **Swap form** (Card):
  - Pre-filled inputs: `pair` (from quote), `amount_out` (from quote), `max_amount_in` (editable integer string), `payer_id`, `beneficiary_id`
  - Submit button: disabled when `store.circuitBreakerState === "HALTED"` or when quote is stale
  - On submit: if quote is stale, call `store.fetchQuote` first, then call `store.executeSwap(payload)`
  - On success: show `order_id`, `tx_hash`, `amount_in`, `confirmed_at`
  - Swap error message mapping (render inline below form, not as toast):
    - `SLIPPAGE_LIMIT_EXCEEDED` → "Market moved — retry with updated quote" + Refresh Quote button
    - `INSUFFICIENT_POOL_LIQUIDITY` → "Insufficient pool liquidity — contact Central Bank"
    - `ZK_VALIDATION_FAILED` → "ZK validation failed — check ZK pointer field"
    - `CIRCUIT_BREAKER_HALTED` → "Swaps temporarily suspended by Central Bank" (also disables Submit)
    - Other errors → show raw message

  **Approve AMM panel** (collapsible Card or `<details>`):
  - Label: "Approve AMM Token Allowance"
  - Inputs: `amount_a` (integer string), `amount_b` (integer string)
  - Submit: calls `store.approveAmm({ amount_a, amount_b })`
  - On success: show "Allowance approved" confirmation
  - On error: show inline error message

  **Acceptance**: `npx tsc --noEmit` passes. With `VITE_SCENARIO=a` (or unset), existing Scenario A AMM page renders unchanged.

---

- [ ] T019 Modify bank app routes to register Scenario B route tree in `frontend/apps/bank/src/routes/index.tsx`

  **File to modify**: `frontend/apps/bank/src/routes/index.tsx`

  **Changes** (additive — existing `routes` array is preserved):
  1. Add imports at top: `import { isScenarioB } from "../config/scenario";` and `import { BridgePage } from "../features/bridge/BridgePage";`
  2. Define `scenarioBChildren` array (do not remove the existing Scenario A children):
     ```ts
     const scenarioBChildren = [
       { index: true, element: <DashboardPage /> },
       { path: "bridge", element: <BridgePage /> },
       { path: "amm",    element: <AMMTradingPage /> },
       { path: "compliance", element: <ComplianceCenterPage /> },
       { path: "onboarding", element: <OnboardingPage /> },
       { path: "settings",   element: <SettingsPage /> },
     ];
     ```
  3. In the existing `routes` array, inside the `AppLayout` route object, change the `children` field:
     ```ts
     children: isScenarioB ? scenarioBChildren : scenarioAChildren,
     ```
     where `scenarioAChildren` is the existing children array (extract it into a named const to keep code readable).
  4. Do NOT remove any existing page imports — they are still needed when `isScenarioB = false`.

  **Acceptance**: `npx tsc --noEmit` passes. With `VITE_SCENARIO=b`, navigating to `/deposits` returns a not-found or redirects. `/bridge` loads `BridgePage`.

---

## Phase 5 — M5: Governance App API Services

**Goal**: All governance-side Scenario B API modules exist and call real `/api/v2` endpoints via `httpClient`.
**Dependencies**: M1 complete (governance types exist).
**Verification**: `cd frontend/apps/governance && npx tsc --noEmit`

---

- [ ] T020 [P] Create liquidity API service for governance app in `frontend/apps/governance/src/services/api/liquidity.api.ts`

  **File to create**: `frontend/apps/governance/src/services/api/liquidity.api.ts`

  **Pattern**: Follow existing `frontend/apps/governance/src/services/api/governance.api.ts` — import `httpClient` from `./http-client`, no mocks.

  **Implementation**:
  ```ts
  import type { AddLiquidityRequest, LiquidityPosition, MintAndApproveRequest, PoolStatus, RemoveLiquidityRequest } from "../../types/liquidity.types";
  import { httpClient } from "./http-client";

  export const liquidityApi = {
    getPoolStatus: (pair: string) =>
      httpClient.get<PoolStatus>(`/api/v2/amm/pool/${pair}/status`).then((r) => r.data),
    addLiquidity: (payload: AddLiquidityRequest) =>
      httpClient.post<LiquidityPosition>("/api/v2/amm/liquidity/add", payload).then((r) => r.data),
    removeLiquidity: (payload: RemoveLiquidityRequest) =>
      httpClient.post<{ status: string }>("/api/v2/amm/liquidity/remove", payload).then((r) => r.data),
    mintAndApprove: (payload: MintAndApproveRequest) =>
      httpClient.post<{ status: string }>("/api/v2/amm/token/mint-and-approve", payload).then((r) => r.data),
  };
  ```

  **API contract**:
  - `GET /api/v2/amm/pool/:pair/status` → 200 `PoolStatus`
  - `POST /api/v2/amm/liquidity/add` → 201 `LiquidityPosition`
  - `POST /api/v2/amm/liquidity/remove` → 200 `{ status: "ok" }`, 404 on not-found/withdrawn
  - `POST /api/v2/amm/token/mint-and-approve` → 200 `{ status: "ok" }`

  **Acceptance**: `npx tsc --noEmit` passes. No mock imports.

---

- [ ] T021 [P] Create circuit-breaker v2 API service for governance app in `frontend/apps/governance/src/services/api/circuit-breaker-v2.api.ts`

  **File to create**: `frontend/apps/governance/src/services/api/circuit-breaker-v2.api.ts`

  **Implementation**:
  ```ts
  import type {
    CircuitBreakerV2Status,
    PauseRequest,
    ProposeResumeRequest,
    ProposeResumeResponse,
    SignResumeRequest,
  } from "../../types/circuit-breaker-v2.types";
  import { httpClient } from "./http-client";

  export const circuitBreakerV2Api = {
    getStatus: (pair: string) =>
      httpClient
        .get<CircuitBreakerV2Status>("/api/v2/governance/circuit-breaker/status", { params: { pair } })
        .then((r) => r.data),
    pause: (payload: PauseRequest) =>
      httpClient.post<CircuitBreakerV2Status>("/api/v2/governance/circuit-breaker/pause", payload).then((r) => r.data),
    proposeResume: (payload: ProposeResumeRequest) =>
      httpClient
        .post<ProposeResumeResponse>("/api/v2/governance/circuit-breaker/resume-request", payload)
        .then((r) => r.data),
    signResume: (payload: SignResumeRequest) =>
      httpClient
        .post<CircuitBreakerV2Status>("/api/v2/governance/circuit-breaker/resume-sign", payload)
        .then((r) => r.data),
  };
  ```

  **Acceptance**: `npx tsc --noEmit` passes. No mock imports.

---

- [ ] T022 [P] Create oversight API service for governance app in `frontend/apps/governance/src/services/api/oversight.api.ts`

  **File to create**: `frontend/apps/governance/src/services/api/oversight.api.ts`

  **Implementation**:
  ```ts
  import type { DisclosureRequest, OpenDisclosureRequest, SignDisclosureRequest } from "../../types/oversight.types";
  import { httpClient } from "./http-client";

  export const oversightApi = {
    openDisclosure: (payload: OpenDisclosureRequest) =>
      httpClient.post<DisclosureRequest>("/api/v2/oversight/disclosure-request", payload).then((r) => r.data),
    signDisclosure: (payload: SignDisclosureRequest) =>
      httpClient.post<{ status: string }>("/api/v2/oversight/disclosure-sign", payload).then((r) => r.data),
    getDisclosureStatus: (requestId: string) =>
      httpClient.get<DisclosureRequest>(`/api/v2/oversight/disclosure-status/${requestId}`).then((r) => r.data),
  };
  ```

  **API contract**:
  - `POST /api/v2/oversight/disclosure-request` → 201 `DisclosureRequest`
  - `POST /api/v2/oversight/disclosure-sign` → 200 `{ status: "signed" }`, 422 on duplicate/invalid
  - `GET /api/v2/oversight/disclosure-status/:requestId` → 200 `DisclosureRequest`

  **Acceptance**: `npx tsc --noEmit` passes. No mock imports.

---

## Phase 6 — M6: Governance App Stores

**Goal**: Zustand stores for liquidity, circuit breaker v2, and oversight wire to the API services created in M5.
**Dependencies**: M5 complete (API services exist).
**Verification**: `cd frontend/apps/governance && npx tsc --noEmit`

---

- [ ] T023 [P] Create liquidity Zustand store for governance app in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`

  **Files to create**:
  - `frontend/apps/governance/src/features/liquidity/liquidity.store.ts` (create `features/liquidity/` directory)

  **Key constraint**: `lpPositions` is session-only — populated exclusively from `addLiquidity()` responses. NO polling of positions from backend. LP positions reset on page reload.

  **Implementation**:
  ```ts
  import { create } from "zustand";
  import { liquidityApi } from "../../services/api/liquidity.api";
  import type { AddLiquidityRequest, LiquidityPosition, MintAndApproveRequest, PoolStatus, RemoveLiquidityRequest } from "../../types/liquidity.types";

  type LiquidityState = {
    poolStatus:      PoolStatus | null;
    lpPositions:     LiquidityPosition[];
    status:          "idle" | "loading" | "error";
    error:           string | null;
    fetchPoolStatus: (pair: string) => Promise<void>;
    addLiquidity:    (payload: AddLiquidityRequest) => Promise<void>;
    removeLiquidity: (payload: RemoveLiquidityRequest) => Promise<void>;
    mintAndApprove:  (payload: MintAndApproveRequest) => Promise<void>;
  };

  export const useLiquidityStore = create<LiquidityState>((set, get) => ({
    poolStatus:  null,
    lpPositions: [],
    status:      "idle",
    error:       null,

    fetchPoolStatus: async (pair) => {
      try {
        const poolStatus = await liquidityApi.getPoolStatus(pair);
        set({ poolStatus });
      } catch {
        // silently preserve last known value
      }
    },

    addLiquidity: async (payload) => {
      set({ status: "loading", error: null });
      try {
        const position = await liquidityApi.addLiquidity(payload);
        set({ lpPositions: [position, ...get().lpPositions], status: "idle" });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Add liquidity failed" });
      }
    },

    removeLiquidity: async (payload) => {
      set({ status: "loading", error: null });
      try {
        await liquidityApi.removeLiquidity(payload);
        // Update the matching LP position status to WITHDRAWN in session state
        set({
          lpPositions: get().lpPositions.map((p) =>
            p.lp_id === payload.lp_id ? { ...p, status: "WITHDRAWN" as const } : p,
          ),
          status: "idle",
        });
      } catch (err) {
        const message = err instanceof Error ? err.message : "Remove liquidity failed";
        // Surface 404 as domain error for "Position not found or already withdrawn"
        set({ status: "error", error: message });
      }
    },

    mintAndApprove: async (payload) => {
      set({ status: "loading", error: null });
      try {
        await liquidityApi.mintAndApprove(payload);
        set({ status: "idle" });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Mint and approve failed" });
      }
    },
  }));
  ```

  **Acceptance**: `npx tsc --noEmit` passes. `lpPositions` is never fetched from the backend.

---

- [ ] T024 [P] Create circuit-breaker v2 Zustand store for governance app in `frontend/apps/governance/src/features/circuit-breaker/circuit-breaker-v2.store.ts`

  **Files to create**:
  - `frontend/apps/governance/src/features/circuit-breaker/circuit-breaker-v2.store.ts` (create `features/circuit-breaker/` directory)

  **Implementation**:
  ```ts
  import { create } from "zustand";
  import { circuitBreakerV2Api } from "../../services/api/circuit-breaker-v2.api";
  import type {
    CircuitBreakerV2Status,
    PauseRequest,
    ProposeResumeRequest,
    SignResumeRequest,
  } from "../../types/circuit-breaker-v2.types";

  type CbV2State = {
    cbStatus:        CircuitBreakerV2Status | null;
    resumeRequestId: string | null;
    status:          "idle" | "loading" | "error";
    error:           string | null;
    fetchStatus:     (pair: string) => Promise<void>;
    pause:           (payload: PauseRequest) => Promise<void>;
    proposeResume:   (payload: ProposeResumeRequest) => Promise<void>;
    signResume:      (payload: SignResumeRequest) => Promise<void>;
  };

  export const useCbV2Store = create<CbV2State>((set) => ({
    cbStatus:        null,
    resumeRequestId: null,
    status:          "idle",
    error:           null,

    fetchStatus: async (pair) => {
      try {
        const cbStatus = await circuitBreakerV2Api.getStatus(pair);
        set({ cbStatus });
      } catch {
        // silently preserve last known state; page shows stale warning if needed
      }
    },

    pause: async (payload) => {
      set({ status: "loading", error: null });
      try {
        const cbStatus = await circuitBreakerV2Api.pause(payload);
        set({ cbStatus, status: "idle" });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Pause failed" });
      }
    },

    proposeResume: async (payload) => {
      set({ status: "loading", error: null });
      try {
        const result = await circuitBreakerV2Api.proposeResume(payload);
        set({ resumeRequestId: result.request_id, status: "idle" });
        // Refresh full status after propose
        const cbStatus = await circuitBreakerV2Api.getStatus(payload.pair);
        set({ cbStatus });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Propose resume failed" });
      }
    },

    signResume: async (payload) => {
      set({ status: "loading", error: null });
      try {
        const cbStatus = await circuitBreakerV2Api.signResume(payload);
        set({ cbStatus, status: "idle" });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Sign resume failed" });
      }
    },
  }));
  ```

  **Acceptance**: `npx tsc --noEmit` passes.

---

- [ ] T025 [P] Create oversight Zustand store for governance app in `frontend/apps/governance/src/features/oversight/oversight.store.ts`

  **Files to create**:
  - `frontend/apps/governance/src/features/oversight/oversight.store.ts` (create `features/oversight/` directory)

  **Implementation**:
  ```ts
  import { create } from "zustand";
  import { oversightApi } from "../../services/api/oversight.api";
  import type { DisclosureRequest, OpenDisclosureRequest, SignDisclosureRequest } from "../../types/oversight.types";

  type OversightState = {
    disclosures:        DisclosureRequest[];
    currentDisclosure:  DisclosureRequest | null;
    status:             "idle" | "loading" | "error";
    error:              string | null;
    openDisclosure:     (payload: OpenDisclosureRequest) => Promise<void>;
    signDisclosure:     (payload: SignDisclosureRequest) => Promise<void>;
    fetchDisclosureStatus: (requestId: string) => Promise<void>;
  };

  export const useOversightStore = create<OversightState>((set, get) => ({
    disclosures:       [],
    currentDisclosure: null,
    status:            "idle",
    error:             null,

    openDisclosure: async (payload) => {
      set({ status: "loading", error: null });
      try {
        const disclosure = await oversightApi.openDisclosure(payload);
        set({ disclosures: [disclosure, ...get().disclosures], status: "idle" });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Open disclosure failed" });
      }
    },

    signDisclosure: async (payload) => {
      set({ status: "loading", error: null });
      try {
        await oversightApi.signDisclosure(payload);
        set({ status: "idle" });
      } catch (err) {
        // 422 "Already signed or invalid request" — surface as inline error
        set({ status: "error", error: err instanceof Error ? err.message : "Sign disclosure failed" });
      }
    },

    fetchDisclosureStatus: async (requestId) => {
      set({ status: "loading", error: null });
      try {
        const currentDisclosure = await oversightApi.getDisclosureStatus(requestId);
        set({ currentDisclosure, status: "idle" });
      } catch (err) {
        set({ status: "error", error: err instanceof Error ? err.message : "Status fetch failed" });
      }
    },
  }));
  ```

  **Acceptance**: `npx tsc --noEmit` passes.

---

## Phase 7 — M7: Governance App Pages, Sidebar & Routes

**Goal**: Scenario B reachable in `governance` app. New pages render. v2 Circuit Breaker multi-party flow works. Scenario A routes absent when `VITE_SCENARIO=b`.
**Dependencies**: M6 complete (stores exist).
**Verification**: `cd frontend/apps/governance && npx tsc --noEmit`. Build with and without `VITE_SCENARIO=b`.

---

- [ ] T026 Modify governance app Sidebar to show Scenario B navigation when `isScenarioB` in `frontend/apps/governance/src/components/layout/Sidebar.tsx`

  **File to modify**: `frontend/apps/governance/src/components/layout/Sidebar.tsx`

  **Changes**:
  1. Add import at top: `import { isScenarioB } from "../../config/scenario";`
  2. Add imports for new icons if not already imported: `Droplets` (or `Coins`) for Liquidity, `Eye` for Oversight from `lucide-react`.
  3. Add `useCbV2Store` import: `import { useCbV2Store } from "../../features/circuit-breaker/circuit-breaker-v2.store";`
  4. Define `scenarioBNavItems` array above the `Sidebar` function:
     ```ts
     const scenarioBNavItems = [
       { to: "/",               label: "Dashboard",            icon: LayoutDashboard },
       { to: "/liquidity",      label: "Liquidity Management", icon: Coins },
       { to: "/circuit-breaker",label: "Circuit Breaker",      icon: ShieldAlert },
       { to: "/oversight",      label: "Oversight",            icon: Eye },
       { to: "/settings",       label: "Settings",             icon: Settings },
     ];
     ```
  5. In the `Sidebar` function body:
     - Add: `const cbV2Status = useCbV2Store((s) => s.cbStatus);`
     - For Scenario B: derive `isHalted` from `cbV2Status?.state === "HALTED"`
     - For Scenario A: keep existing `const { circuitBreaker } = useCircuitBreaker();`
     - Circuit-breaker badge: `<Badge variant={isHalted ? "destructive" : "default"}>{(isScenarioB ? cbV2Status?.state : circuitBreaker?.state) ?? "LIVE"}</Badge>`
     - Replace `{navItems.map(...)}` with `{(isScenarioB ? scenarioBNavItems : navItems).map(...)}`
  6. Do NOT change the existing `navItems` array or any existing JSX.

  **Acceptance**: `npx tsc --noEmit` passes. With `VITE_SCENARIO=b`, sidebar shows Liquidity Management, Circuit Breaker, Oversight (no HTLCMonitor, DepositsApproval, etc.).

---

- [ ] T027 [P] [US4] [US5] Create Liquidity Management page for governance app in `frontend/apps/governance/src/features/liquidity/LiquidityManagementPage.tsx`

  **File to create**: `frontend/apps/governance/src/features/liquidity/LiquidityManagementPage.tsx`

  **UI components** from `@cbweb3/ui`: `Button`, `Card`, `CardContent`, `CardHeader`, `CardTitle`, `CardDescription`, `Badge`, `Input`, `Label`, `Table`, `TableBody`, `TableCell`, `TableHead`, `TableHeader`, `TableRow`.

  **Layout sections** (four cards):

  **1. Pool Status card**:
  - Fetches on mount: `store.fetchPoolStatus("BRL-USD")`
  - Polls: `usePolling(store.fetchPoolStatus.bind(null, "BRL-USD"), 15_000, true)` (no max duration)
  - Displays: `pool_pair`, `reserve_a`, `reserve_b`, `current_ratio`, `updated_at`
  - Imbalance banner: when `poolStatus?.imbalance_flag === true`, show a yellow warning banner "Pool imbalance detected — reserves are asymmetric"

  **2. Add Liquidity form**:
  - Inputs (all integer strings — use text inputs): `pool_pair` (default `BRL-USD`), `token_a_amount`, `token_b_amount`, `provider_bank_id`
  - Submit: calls `store.addLiquidity(payload)` — on success, new LP position appears in the session table below
  - Show inline error if `store.error`

  **3. Remove Liquidity form**:
  - Inputs: `lp_id`, `pool_pair`, `provider_bank_id`
  - Submit: calls `store.removeLiquidity(payload)`
  - On 404 error: show "Position not found or already withdrawn"
  - Show inline error otherwise

  **4. LP Positions table** (session-only — populated from Add Liquidity responses):
  - Columns: `lp_id`, `pool_pair`, `provider_bank_id`, `token_a_amount`, `token_b_amount`, `lp_shares`, `status` (as `Badge`), `added_at`
  - Status badge variants: `ACTIVE` → `success`, `WITHDRAWN` → `outline`
  - Empty state: "No LP positions added this session"
  - **No polling** — this table MUST NOT call any GET endpoint for positions

  **5. MintAndApprove panel** (collapsible Card or `<details>`):
  - Label: "Mint & Approve Token Operation"
  - Inputs: `amount_a` (integer string), `amount_b` (integer string), `recipient` (optional text)
  - Submit: calls `store.mintAndApprove(payload)`
  - On success: "Token minting and approval completed"
  - On error: show inline

  **Acceptance**: `npx tsc --noEmit` passes. LP Positions table does NOT call any backend polling endpoint.

---

- [ ] T028 [P] [US7] Create Oversight page for governance app in `frontend/apps/governance/src/features/oversight/OversightPage.tsx`

  **File to create**: `frontend/apps/governance/src/features/oversight/OversightPage.tsx`

  **UI components** from `@cbweb3/ui`: `Button`, `Card`, `CardContent`, `CardHeader`, `CardTitle`, `Badge`, `Input`, `Label`.

  **Layout sections** (three cards):

  **1. Open Disclosure Request form**:
  - Inputs: `tx_ref`, `requestor_id`, `reason_code` (all text)
  - Submit: calls `store.openDisclosure(payload)`
  - On success: display the returned `DisclosureRequest` object inline:
    - `request_id` (prominent, copyable), `state` badge, `quorum_reached`/`quorum_required` (as `${quorum_reached}/${quorum_required}`), `expires_at`
  - On error: show inline

  **2. Sign Disclosure form**:
  - Inputs: `request_id`, `signer_id`
  - Submit: calls `store.signDisclosure({ request_id, signer_id })`
  - On success: show "Signature submitted — fetch status to see updated quorum"
  - On 422 error: show "Already signed or invalid request" inline (do not throw/crash)

  **3. Disclosure Status lookup**:
  - Input: `request_id` (text)
  - Submit button: calls `store.fetchDisclosureStatus(requestId)` and sets `currentDisclosure`
  - Display `currentDisclosure` when present:
    - State badge: `PENDING` → `default`, `QUORUM_REACHED` → `success`, `EXPIRED` → `outline`, `REJECTED` → `destructive`
    - Quorum progress: `${quorum_reached}/${quorum_required}`
    - `expires_at`, `closed_at` (if non-null)

  **Acceptance**: `npx tsc --noEmit` passes. 422 error from sign does not crash page.

---

- [ ] T029 [US6] Modify CircuitBreakerPage to add Scenario B v2 multi-party branch in `frontend/apps/governance/src/pages/CircuitBreakerPage.tsx`

  **File to modify**: `frontend/apps/governance/src/pages/CircuitBreakerPage.tsx`

  **Changes** (additive — do NOT alter the existing Scenario A JSX):
  1. Add at the top: `import { isScenarioB } from "../config/scenario";`
  2. Add: `import { useCbV2Store } from "../features/circuit-breaker/circuit-breaker-v2.store";`
  3. Add: `import { usePolling } from "../hooks/usePolling";`
  4. Add: `import { CB_STATE } from "../types/circuit-breaker-v2.types";`
  5. Wrap existing `return (...)` with:
     ```tsx
     if (isScenarioB) {
       return <CircuitBreakerV2 />;
     }
     return <existing JSX unchanged>;
     ```
  6. Define `function CircuitBreakerV2()` in the **same file** (NOT a new file):

  **CircuitBreakerV2 component** — four sections:

  **State card**:
  - Polls `store.fetchStatus("BRL-USD")` via `usePolling(..., 15_000, true)` on mount
  - Shows `cbStatus?.state` as badge:
    - `LIVE` → `default` (green), `HALTED` → `destructive` (red), `RESUME_PENDING` → `warning` (yellow)
  - When `state === CB_STATE.RESUME_PENDING`: display `cbStatus.resume_request_id` prominently with label "Resume Request ID — share for co-signing"
  - When quorum not yet met (after Sign Resume returns `RESUME_PENDING`): show note "Waiting for additional co-signatures"

  **Pause form**:
  - Inputs: `pair` (default `BRL-USD`), `bank_id`, `reason_code`, `signature` (text input, label "Institutional Signature (base64)", defaultValue `"AA="`)
  - Submit: calls `store.pause({ pair, bank_id, reason_code, signature })`
  - On success: state card updates to `HALTED`
  - On error: show inline

  **Propose Resume form**:
  - Inputs: `pair` (default `BRL-USD`), `bank_id`, `signature` (text input, same label/default as above)
  - Submit: calls `store.proposeResume({ pair, bank_id, signature })`
  - On success: show `request_id` prominently — "Resume Request ID: [id] — share with co-signers"
  - State card updates to `RESUME_PENDING`
  - On error: show inline

  **Sign Resume form**:
  - Inputs: `pair` (default `BRL-USD`), `request_id`, `bank_id`, `signature` (text input, same label/default)
  - Pre-populate `request_id` from `store.resumeRequestId` if set (editable)
  - Submit: calls `store.signResume({ pair, request_id, bank_id, signature })`
  - On success with `state === CB_STATE.LIVE`: state card updates to `LIVE`
  - On success with `state === CB_STATE.RESUME_PENDING`: show "Waiting for additional co-signatures"
  - On error: show inline

  **Acceptance**: `npx tsc --noEmit` passes. With `VITE_SCENARIO=a` (or unset), existing v1 Circuit Breaker page renders unchanged.

---

- [ ] T030 Modify governance app routes to register Scenario B route tree in `frontend/apps/governance/src/routes/index.tsx`

  **File to modify**: `frontend/apps/governance/src/routes/index.tsx`

  **Changes** (additive — existing Scenario A routes preserved):
  1. Add imports: `import { isScenarioB } from "../config/scenario";`, `import { LiquidityManagementPage } from "../features/liquidity/LiquidityManagementPage";`, `import { OversightPage } from "../features/oversight/OversightPage";`
  2. Identify the existing children array inside the `AppLayout` route — extract it into a named const `scenarioAChildren`.
  3. Define `scenarioBChildren`:
     ```ts
     const scenarioBChildren = [
       { index: true, element: <DashboardPage /> },
       { path: "liquidity",       element: <LiquidityManagementPage /> },
       { path: "circuit-breaker", element: <CircuitBreakerPage /> },
       { path: "oversight",       element: <OversightPage /> },
       { path: "settings",        element: <SettingsPage /> },
     ];
     ```
  4. Change `AppLayout` route children: `children: isScenarioB ? scenarioBChildren : scenarioAChildren`
  5. Do NOT remove any existing page imports — needed when `isScenarioB = false`.

  **Acceptance**: `npx tsc --noEmit` passes. With `VITE_SCENARIO=b`, `/deposits-approval` is unreachable. `/liquidity` loads `LiquidityManagementPage`. `/circuit-breaker` loads the modified `CircuitBreakerPage` (v2 branch active).

---

## Final Verification

After all tasks complete, run:

```bash
# TypeScript validation — both apps
cd frontend/apps/bank && npx tsc --noEmit
cd frontend/apps/governance && npx tsc --noEmit

# Scenario B builds
cd frontend/apps/bank && VITE_SCENARIO=b npm run build
cd frontend/apps/governance && VITE_SCENARIO=b npm run build

# Scenario A builds (must be unchanged)
cd frontend/apps/bank && npm run build
cd frontend/apps/governance && npm run build
```

All commands must exit 0.

---

## Parallel Execution Examples

### M1 — Run in parallel (no inter-dependencies within M1):
- T001 + T002 + T003 + T004 (bank app files)
- T005 + T006 + T007 + T008 + T009 (governance app files)
- Then T010 as a gate check

### M2 + M5 — Run in parallel (different apps, no inter-dependencies):
- T011 + T012 + T013 (bank API services)
- T020 + T021 + T022 (governance API services)

### M3 + M6 — Run in parallel:
- T014 + T015 (bank stores)
- T023 + T024 + T025 (governance stores)

### M4 — T017 and T018 in parallel (different new files); T016 and T019 independently:
- T016 (sidebar), T017 (BridgePage), T018 (AMMTradingPage), T019 (routes)

### M7 — T027, T028 in parallel; T026 and T030 independently; T029 after stores (T024):
- T026 (sidebar), T027 (LiquidityPage), T028 (OversightPage), T029 (CircuitBreakerPage), T030 (routes)
