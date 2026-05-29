# Research: Wizard Frontend para Criação Cooperativa de Liquidez

**Feature**: `006-cooperative-liquidity-wizard`
**Date**: 2026-05-15
**Status**: Complete — all NEEDS CLARIFICATION resolved

---

## Decision: Multi-step wizard as a single embedded component

- **Decision**: `CooperativeLiquidityWizard` is a single React component with internal
  step state managed via `useState<WizardStep>`, embedded directly in
  `LiquidityManagementPage`. No new routes.
- **Rationale**: The governance SPA uses page-embedded panels (see existing
  `MintAndApprove` collapsible panel pattern). Adding wizard routes would require router
  changes and a new URL pattern — unnecessary complexity for a sub-page flow.
- **Alternatives considered**:
  - Route-based wizard (rejected: requires router config changes, breaks
    direct-URL-access assumption)
  - Modal dialog wizard (rejected: insufficient screen real estate for 4-step flows
    with polling + countdown in Step 3)

## Decision: Provider ID source

- **Decision**: `useAuthStore((s) => s.profile?.bankId ?? "")` — reads from the
  existing auth store, falls back to empty string when undefined. Field remains editable
  in Step 2.
- **Rationale**: Confirmed in spec clarification Q7. `UserProfile.bankId` is the CB
  identifier used throughout the governance app. The `?? ""` fallback allows tryout
  operators to type the ID manually.
- **Alternatives considered**:
  - `VITE_PROVIDER_ID` env variable (rejected: per clarification Q7, env vars configure
    API URLs, not user identity)

## Decision: Polling strategy for Step 3

- **Decision**: Use existing `usePolling(cb, 5000, enabled)` hook with
  `enabled = currentStep === WizardStep.MONITOR`. The page-level 15s polling continues
  running in parallel (same `fetchPoolStatus` action, same `poolStatus` state slice).
- **Rationale**: Both polls write to the same `poolStatus` key in Zustand — no state
  conflict. The Step 3 poll just runs more frequently while it's active. Disabling the
  15s poll while in Step 3 would require threading `currentStep` state outside the
  wizard, which leaks internal state unnecessarily.
- **Alternatives considered**:
  - Pausing the 15s poll while in Step 3 (rejected: adds coupling between wizard and
    page-level polling logic)
  - WebSocket subscription (rejected: out of scope, no WS infrastructure on the
    governance API)

## Decision: Countdown implementation

- **Decision**: `setInterval` (1s) inside Step 3 component, using `useEffect` with
  cleanup. Compute remaining time as `Math.max(0, expires_at_ms - Date.now())`, format
  as `Xh Ym` or "Expirado" when ≤ 0.
- **Rationale**: Native browser timer, no library dependency. The 1s granularity is
  sufficient for a 72h window (SC-003 allows 2s visual lag).
- **Alternatives considered**:
  - `react-countdown` library (rejected: no new packages constraint)
  - Polling the server for expiry (rejected: unnecessary — `expires_at` is a fixed
    timestamp returned by the commit response)

## Decision: Error display strategy

- **Decision**: Inline error below the form (not toast/alert overlay). A `commitError`
  state field in the store drives the error text. Error is cleared on the next submit
  attempt.
- **Rationale**: FR-013 specifies inline errors. Consistent with existing pattern in
  `LiquidityManagementPage` (shows `{error && <p className="text-sm text-red-500">}`).
- **Alternatives considered**:
  - Toast (rejected: FR-013 explicitly requires inline)

## Decision: `listCommits` wiring

- **Decision**: Implement `listCommits` in `liquidity.api.ts` and
  `fetchPendingCommits` in `liquidity.store.ts` but wire neither to any wizard
  component.
- **Rationale**: Spec clarification Q5 states the method is a utility for future use.
  The banner detection uses `poolStatus.pending_commits` already fetched by the 15s
  polling — no extra request needed.
- **Alternatives considered**:
  - Skip implementing `listCommits` entirely (rejected: FR-009 explicitly requires it)

## Decision: `addLiquidity` legacy action preservation

- **Decision**: Keep `addLiquidity` action in store and `liquidityApi.addLiquidity` in
  API module. Remove only the Add Liquidity _form_ card from the page.
- **Rationale**: Spec Assumptions state "the legacy endpoint is maintained for
  compatibility with tryout scripts". Other pages or scripts may call `addLiquidity`
  directly.
- **Alternatives considered**:
  - Remove the action (rejected: spec explicitly says keep it)

## Decision: `commitStatus` as independent status field

- **Decision**: Add `commitStatus: "idle" | "loading" | "error"` as a separate field
  from the existing `status` field in the store.
- **Rationale**: The existing `status` field is shared by `fetchPoolStatus`,
  `addLiquidity`, `removeLiquidity`, and `mintAndApprove`. If `commitStatus` reused
  `status`, a commit in progress would disable the Remove Liquidity submit button.
  Independent fields prevent cross-action interference.
- **Alternatives considered**:
  - Reuse `status` (rejected: would disable existing Remove Liquidity form during commit
    loading)

---

## All NEEDS CLARIFICATION resolved

| Unknown | Resolution |
|---|---|
| Step navigation mechanism | Internal `useState<WizardStep>` in single component |
| Provider ID source | `useAuthStore(s => s.profile?.bankId ?? "")` |
| Step 3 polling vs page polling | Co-exist, same state key, different intervals |
| Countdown mechanism | `setInterval` 1s in `useEffect`, pure computation from `expires_at` |
| Error display | Inline, driven by `commitError` store field |
| `listCommits` usage | Implemented but not wired (utility for future) |
| `addLiquidity` legacy | Keep action and API method; remove only the form card |
| `commitStatus` isolation | Separate field from `status` to avoid cross-action disabling |
