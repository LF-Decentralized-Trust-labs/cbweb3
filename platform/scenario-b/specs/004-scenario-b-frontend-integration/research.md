# Research: Scenario B Frontend Integration

**Phase 0 output** | Feature: `scenario-b-frontend-integration` | Date: 2026-05-07

All clarifications were resolved in the spec's Session 2026-05-07. No external research was required.

---

## Resolved Decisions

### R1 — LP Positions data source

**Decision**: Session-only. The LP Positions table is populated exclusively from Add Liquidity API responses in the current browser session.

**Rationale**: There is no `GET /api/v2/amm/liquidity/positions` endpoint in the backend router. The table MUST NOT poll against it.

**Impact**: `liquidity.store.ts` appends to `lpPositions[]` only from `addLiquidity()` action responses; the array resets on page reload.

---

### R2 — Signature field UX (governance)

**Decision**: Render a text `<input>` labeled **"Institutional Signature (base64)"**, pre-populated with default value `"AA="`. The user can override it with any valid base64 string.

**Rationale**: In dev environments `"AA="` is the zero-bytes base64 placeholder accepted by the backend. Production flows use an external signing tool to provide the real base64 value.

**Impact**: Applies to all three governance forms: Pause, Propose Resume, Sign Resume (FR-030, FR-031, FR-032, FR-041). Also applies to any oversight forms that accept a `signature` field.

---

### R3 — `usePolling` hook location

**Decision**: Each app implements its own `usePolling` hook under `frontend/apps/{bank,governance}/src/hooks/usePolling.ts`.

**Rationale**: The hook MUST NOT be added to the shared `@cbweb3/ui` package. Polling is an app-level concern; shared-package inclusion would add dependency coupling.

**Hook signature**:
```ts
export function usePolling(
  callback: () => void,
  intervalMs: number,
  enabled: boolean,
  maxDurationMs?: number,
): void
```
Implementation uses `setInterval` + `clearInterval` in a `useEffect`. If `maxDurationMs` is provided, a `setTimeout` cancels the interval after that duration and triggers a timeout notice (returned via a separate `onTimeout` callback or a separate `useRef`/setter passed as option). Cleanup runs on component unmount.

---

### R4 — Bridge positions table filter scope

**Decision**: Initial load calls `GET /api/v2/bridge/positions` with **no `state` filter parameter**. A `state` filter dropdown is provided as a UI control for user-driven client-side filtering only. After Lock&Mint submission, polling identifies the new position by matching `position_id` client-side.

**Rationale**: The API supports an optional `state=` query param, but the UI must start with all positions visible. Filtering is a UX convenience, not a data-scoping requirement.

---

### R5 — Scenario A pages when `VITE_SCENARIO=b`

**Decision**: Scenario-A-only routes (HTLC, FX Agreements, Deposits, Escrows, Redeems, and their governance equivalents) are NOT registered in the route tree when `VITE_SCENARIO=b`. Sidebar items for those routes are NOT rendered. Page files remain on disk but are unreachable.

**Rationale**: Build-time tree-shaking removes the code from the bundle. This is cleaner than runtime 404 handling or conditional rendering of each individual page.

---

### R6 — AMM Trading Page — in-place modification

**Decision**: Modify existing `AMMTradingPage.tsx` in-place using `isScenarioB`. No `AMMTradingPageV2.tsx` file is created.

**Rationale**: Single file with `isScenarioB` branch is simpler. The route path `/amm` stays the same in both scenarios.

---

### R7 — Circuit Breaker Page — in-place modification

**Decision**: Modify existing `CircuitBreakerPage.tsx` in-place using `isScenarioB`. No `CircuitBreakerPageV2.tsx` file is created.

**Rationale**: Same rationale as R6. The route path `/circuit-breaker` stays the same.

---

### R8 — Polling timeout for bridge positions

**Decision**: After 120 seconds of polling without a terminal state on any position, stop polling and show a "polling timeout" notice on affected rows.

**Implementation**: Pass `maxDurationMs = 120_000` to `usePolling`. On timeout, set a `timedOutPositionIds: Set<string>` state — rows with matching IDs display a manual-refresh prompt.

---

### R9 — Quote staleness threshold

**Decision**: 10 seconds. If `Date.now() - quoteTimestamp > 10_000`, show a "quote stale — refresh" warning. On swap submit, if the quote is stale, re-fetch automatically before submitting (or block submit and show the warning).

**Implementation**: Derived in the component from `ammV2Store.quoteTimestamp`. No timer needed — evaluated at render time.

---

### R10 — AMM pool pair default

**Decision**: Default pair for all initial UI inputs is `BRL-USD`. Other pairs can be entered by the user as free text.

**Rationale**: Assumption 3 from spec. Single pool in initial deployment.

---

### R11 — Amount field treatment

**Decision**: All amount inputs and displays use integer strings (no decimal input, no decimal display). `<input type="text" pattern="[0-9]+" />` is appropriate.

**Rationale**: FR-040. Backend amounts are integer strings representing smallest denomination.

---

### R12 — Governance Sidebar circuit-breaker badge in Scenario B

**Decision**: When `isScenarioB`, the governance sidebar badge reads `cbStatus.state` from `circuit-breaker-v2.store` (v2 store), not from the v1 `useCircuitBreaker()` hook.

**Rationale**: The v2 state machine has three values (`LIVE`/`HALTED`/`RESUME_PENDING`) vs v1's two. The badge should reflect the active scenario's state accurately.

---

## Technology Best Practices Applied

| Area | Practice |
|------|----------|
| Zustand stores | `create<State>()` with typed state + actions in one object. No `immer` middleware (not in existing codebase). |
| API services | Thin adapter objects (`export const fooApi = { ... }`). Import `httpClient` directly. Unwrap `.data` from Axios response in the service layer. |
| TypeScript types | `const OBJ = { ... } as const; type T = typeof OBJ[keyof typeof OBJ]` pattern. No enums. |
| Error handling | Store sets `error: string \| null`; page reads `store.error` and shows it inline. API errors surfaced via `error instanceof Error ? error.message : "fallback"`. |
| Polling | `useEffect` with `setInterval` + return cleanup `clearInterval`. `enabled` flag checked inside effect to skip setup when false. |
| Build-time flag | `import.meta.env.VITE_SCENARIO === "b"` evaluated at module load time — Vite replaces with `"b" === "b"` (true) or `"" === "b"` (false), enabling tree-shaking. |
