# Feature Specification: Scenario B Frontend Integration

**Feature Branch**: `feature/scenario-b-frontend`
**Created**: 2026-05-07
**Status**: Draft
**Spec Directory**: `.agent/specs/scenario-b-frontend-integration/`

---

## Overview

Add Scenario B (AMM v2 / cross-border liquidity pool) UI to the existing `bank` and `governance` frontend applications in the `cbweb3-platform` monorepo. Scenario B is gated entirely by the build-time environment variable `VITE_SCENARIO=b`. When this flag is absent or set to `a`, the applications behave exactly as today (Scenario A: HTLC / FX-Agreements / Deposits / Escrows). When set to `b`, a distinct set of pages, routes, sidebar items, API service modules, Zustand stores, and domain types are activated instead.

No Scenario A pages, routes, stores, or services are modified. All new code is additive.

---

## Clarifications

### Session 2026-05-07

- Q: LP Positions Table Data Source → A: Session-only. The LP Positions table is populated exclusively from Add Liquidity API responses in the current browser session. There is no `GET /api/v2/amm/liquidity/positions` endpoint in the backend router; the table MUST NOT poll against it.
- Q: Signature field UX in governance → A: Render a text input labeled **"Institutional Signature (base64)"**, pre-populated with `"AA="` as the placeholder for dev environments. The user can override it. This applies to the Pause, Propose Resume, and Sign Resume forms.
- Q: Where does the `usePolling` hook live? → A: Each app implements its own polling logic under `frontend/apps/{bank,governance}/src/hooks/usePolling.ts`. The hook MUST NOT be added to the shared `@cbweb3/ui` package.
- Q: Bridge positions table — filter scope → A: Initial load lists ALL positions (no `state` filter on the API request). A `state` filter dropdown is provided as a UI control for user-driven filtering only, applied client-side. After a Lock&Mint submission, polling identifies the relevant position by matching `position_id` client-side.
- Q: Scenario B navigation — what happens to existing Scenario A pages when `VITE_SCENARIO=b`? → A: Scenario-A-only routes (HTLC, FX Agreements, Deposits, Escrows, Redeems, Agreements) are NOT registered in the route tree when `VITE_SCENARIO=b`. Sidebar items for those routes are NOT rendered. Page files remain on disk but are unreachable.
- Q: AMM Trading page in Scenario B — new file or modify existing? → A: Modify the existing `AMMTradingPage.tsx` in-place using the `isScenarioB` flag. When `true`, render the full v2 flow (real `/api/v2` calls, bridge awareness, slippage input, pool status from v2 endpoint). When `false`, render the existing mock-backed Scenario A content. No separate `AMMTradingPageV2.tsx` file is created.
- Q: Circuit Breaker page in governance Scenario B — same pattern? → A: Yes. Modify the existing `CircuitBreakerPage.tsx` in-place using `isScenarioB`. When `true`, render the v2 multi-party flow (pause form, propose-resume, sign-resume, state machine display). When `false`, render the existing v1 toggle logic. No separate `CircuitBreakerPageV2.tsx` file is created.

---

## Goals

1. Commercial banks (`bank` app) can initiate cross-border payments via the AMM — Lock & Mint on the source spoke, Swap on the Hub, Burn & Unlock on the destination spoke — through a guided UI.
2. Commercial banks can view and manage their bridge positions with live state polling.
3. Central banks (`governance` app) can manage Hub AMM liquidity (add/remove LP positions) and monitor pool health.
4. Central banks can pause and propose/co-sign resume of the Circuit Breaker through a multi-party governance flow.
5. Central banks can open, co-sign, and monitor AML/CFT disclosure requests (Oversight / Master Viewing Key).
6. Both apps show a dedicated Scenario B sidebar navigation that replaces Scenario A nav items when the build flag is set.
7. All network calls in Scenario B hit real `/api/v2/*` endpoints — no mocks.

---

## Non-Goals

- Modifying or replacing any Scenario A page, route, store, or service file — with the exception of `AMMTradingPage.tsx` and `CircuitBreakerPage.tsx`, which are modified solely to add an `isScenarioB` branch; the `false` branch MUST render their existing Scenario A content unchanged.
- Implementing a runtime toggle between scenarios (the flag is build-time only).
- Changing authentication flows, Keycloak configuration, or token refresh logic.
- Implementing ZK-Pointer generation UI (the field accepts raw hex input as a developer aid).
- Building WebSocket-based real-time updates; polling via `setInterval` is sufficient.
- Adding unit or integration tests in this feature (test coverage is a separate task).
- Multi-pool support beyond the `BRL-USD` default pair displayed initially.
- Mobile/responsive layout optimisation beyond what Tailwind / shadcn/ui provides by default.

---

## User Scenarios & Testing

### User Story 1 — Commercial Bank: Cross-Border AMM Payment (Priority: P1)

A commercial bank operator opens the **AMM Trading** page (Scenario B build), enters the destination amount they need to deliver, reviews the quoted cost including price impact and slippage tolerance, approves the spend allowance if needed, and executes the swap. The page shows live circuit-breaker and pool-status context before and after submission.

**Why this priority**: This is the primary revenue-generating flow for commercial banks in Scenario B. All other bank-side stories are setup/support for this flow.

**Independent Test**: With `VITE_SCENARIO=b` built and a live `/api/v2` backend, a bank operator can navigate to `/amm`, get a valid quote, set slippage, and submit a swap that returns `COMPLETED` state.

**Acceptance Scenarios**:

1. **Given** the AMM is `LIVE`, **When** the user enters `amount_out` and selects a pool pair, **Then** the page fetches a quote and displays `required_input`, `price_impact` (as %), and `quote_timestamp`.
2. **Given** a fresh quote older than 10 seconds, **When** the user reviews it, **Then** a "quote stale — refresh" warning is shown.
3. **Given** the user sets a slippage tolerance and submits the swap, **When** the API returns `COMPLETED`, **Then** `order_id`, `tx_hash`, `amount_in`, and `confirmed_at` are displayed.
4. **Given** the API returns `SLIPPAGE_LIMIT_EXCEEDED`, **When** the error is received, **Then** the form shows "Market moved — retry with updated quote" and a Refresh Quote button.
5. **Given** the API returns `CIRCUIT_BREAKER_HALTED`, **When** the error is received, **Then** a banner "Swaps temporarily suspended by Central Bank" is shown and the Submit button is disabled.
6. **Given** `imbalance_flag = true` on pool status, **When** the page loads or polls, **Then** a pool imbalance warning banner is shown.

---

### User Story 2 — Commercial Bank: Bridge Lock & Mint / Burn & Unlock (Priority: P1)

A commercial bank operator opens the **Bridge** page, initiates a Lock & Mint to move native CBDC assets onto the Hub as mirrored tokens, then monitors the bridge position until it reaches `ACTIVE`. After swapping, they return to the Bridge page to initiate Burn & Unlock and watch the position progress to `UNLOCKED`.

**Why this priority**: Bridge positions are the prerequisite for cross-spoke swaps. Without working Bridge UI, end-to-end payment flows cannot be completed.

**Independent Test**: With `VITE_SCENARIO=b` built, an operator can submit a Lock & Mint, see the new position appear in the table at `LOCKING`, and watch it poll to `ACTIVE` within the polling window.

**Acceptance Scenarios**:

1. **Given** a user submits Lock & Mint with valid inputs, **When** the API returns `201` with `bridge_state: LOCKING`, **Then** the new position appears in the Bridge Positions table immediately.
2. **Given** a position is at `LOCKING`, **When** polling runs every 5 seconds, **Then** the table row updates state badges in real time until `ACTIVE` or a terminal error state.
3. **Given** a position reaches `ACTIVE`, **When** the user initiates Burn & Unlock for that position, **Then** the position transitions to `BURNED` and polling continues to `UNLOCKED`.
4. **Given** a position reaches `RECONCILIATION_REQUIRED`, **When** the table renders it, **Then** the row shows a distinctive error badge and an operator escalation note.
5. **Given** polling has been running 120 seconds without a terminal state, **When** the timeout elapses, **Then** polling stops and a "polling timeout" notice is shown on the relevant row.

---

### User Story 3 — Commercial Bank: ApproveAMM Token Allowance (Priority: P2)

A commercial bank operator needs to grant the AMM contract permission to spend their Hub tokens before swapping. They use the **Approve AMM** panel on the AMM Trading page to submit the approve transaction.

**Why this priority**: Required one-time setup before a bank can execute any swap. P2 because it is typically done once at environment setup, not per payment.

**Independent Test**: An operator can open the AMM page, expand the Approve AMM panel, enter amounts, submit, and see `status: ok` confirmation.

**Acceptance Scenarios**:

1. **Given** the user enters `amount_a` and `amount_b` and submits Approve AMM, **When** the API returns `{ status: "ok" }`, **Then** a success toast is shown.
2. **Given** the API returns an error, **When** the error is received, **Then** the error message is displayed inline on the panel.

---

### User Story 4 — Central Bank: Liquidity Management (Priority: P1)

A central bank operator opens the **Liquidity Management** page, reviews current pool reserves and status, adds a new LP position by specifying token amounts and provider ID, and can later remove an existing LP position using its `lp_id`.

**Why this priority**: Without liquidity in the pool, all commercial bank swaps fail. This is the foundational governance action for Scenario B.

**Independent Test**: With `VITE_SCENARIO=b` built, a central bank operator can navigate to `/liquidity`, see current pool status, submit Add Liquidity, receive an `lp_id`, see it in the LP Positions table, and then remove it.

**Acceptance Scenarios**:

1. **Given** the page loads, **When** a pool pair is selected, **Then** a Pool Status card shows `reserve_a`, `reserve_b`, `current_ratio`, and an imbalance banner when `imbalance_flag = true`.
2. **Given** a central bank operator fills Add Liquidity fields (`pool_pair`, `token_a_amount`, `token_b_amount`, `provider_bank_id`) and submits, **When** the API returns `201`, **Then** the new LP position (with `lp_id`) appears in the LP Positions table.
3. **Given** an LP position in `ACTIVE` status, **When** the operator enters its `lp_id` and submits Remove Liquidity, **Then** the position's status changes to `WITHDRAWN` in the table.
4. **Given** the operator tries to remove an already `WITHDRAWN` position, **When** the API returns `404`, **Then** an error "Position not found or already withdrawn" is shown.

---

### User Story 5 — Central Bank: MintAndApprove Token Operation (Priority: P2)

A central bank operator uses the **Token Operations** panel (on or near the Liquidity Management page) to mint Hub tokens and approve the AMM to spend them, funding the pool setup.

**Why this priority**: One-time setup per environment; needed before Add Liquidity but not part of the regular flow.

**Independent Test**: An operator can fill `amount_a`, `amount_b`, optionally `recipient`, submit, and see `status: ok`.

**Acceptance Scenarios**:

1. **Given** valid amounts and optional recipient are entered and submitted, **When** the API returns `{ status: "ok" }`, **Then** a success confirmation is shown.
2. **Given** the API returns an error, **Then** the error is displayed inline.

---

### User Story 6 — Central Bank: Circuit Breaker Multi-Party Governance (Priority: P1)

A central bank operator can immediately pause the AMM (Pause action), propose a resume (Propose Resume, which returns a `request_id`), and co-sign a resume initiated by another central bank (Sign Resume, using the `request_id`). The page shows the current `state` (`LIVE` / `HALTED` / `RESUME_PENDING`) and the `resume_request_id` when applicable.

**Why this priority**: Emergency halt capability is a regulatory requirement and must be accurate and accessible.

**Independent Test**: An operator can see current `LIVE` state, click Pause, see state change to `HALTED`, then use Propose Resume to obtain a `request_id`, then use Sign Resume in the same session to bring the state to `LIVE`.

**Acceptance Scenarios**:

1. **Given** the state is `LIVE`, **When** the operator provides `bank_id`, `pair`, `reason_code`, and submits Pause, **Then** the displayed state updates to `HALTED`.
2. **Given** the state is `HALTED`, **When** the operator provides `bank_id` and submits Propose Resume, **Then** the state updates to `RESUME_PENDING` and the `request_id` is displayed.
3. **Given** the state is `RESUME_PENDING`, **When** the operator provides `request_id`, `bank_id`, and submits Sign Resume, **Then** upon quorum the state transitions to `LIVE`.
4. **Given** Sign Resume returns `RESUME_PENDING` (quorum not yet met), **Then** the page continues to show `RESUME_PENDING` with a "waiting for additional signatures" note.
5. **Given** the state is `HALTED`, **When** the bank app is also open, **Then** the bank AMM page shows a halted banner (bank app polls circuit-breaker status independently).

---

### User Story 7 — Central Bank: Oversight / Disclosure Request (Priority: P2)

A central bank operator opens the **Oversight** page, creates an AML/CFT disclosure request for a specific transaction reference, co-signs an existing request using its `request_id`, and monitors the quorum progress (`0/2`, `1/2`, `2/2`) until the state reaches `QUORUM_REACHED` or `EXPIRED`.

**Why this priority**: Regulatory compliance feature; important but less time-sensitive than liquidity and circuit-breaker.

**Independent Test**: An operator can navigate to `/oversight`, open a disclosure request, see `PENDING` state with `quorum_reached: 0`, sign it to see `quorum_reached: 1`, and open disclosure status by `request_id`.

**Acceptance Scenarios**:

1. **Given** valid `tx_ref`, `requestor_id`, and `reason_code` are entered, **When** submitted, **Then** the new request appears with state `PENDING`, `quorum_reached: 0`, and an `expires_at` countdown.
2. **Given** the operator enters a `request_id` and `signer_id` and submits Sign Disclosure, **When** the API returns `{ status: "signed" }`, **Then** fetching the disclosure status shows incremented `quorum_reached`.
3. **Given** `quorum_reached = quorum_required`, **When** the status is displayed, **Then** the state badge shows `QUORUM_REACHED`.
4. **Given** a request has `state: EXPIRED`, **When** it is displayed, **Then** the state badge is shown in a muted/warning colour.
5. **Given** a duplicate signature is submitted, **When** the API returns a 422, **Then** the error is shown inline without crashing.

---

### Edge Cases

- What happens when `VITE_SCENARIO` is absent or set to an unknown value? → Treat as Scenario A (default).
- What happens if circuit-breaker polling receives a network error? → Show last known state with a stale-data warning; do not crash.
- What happens if a bridge position polling times out (120s) without reaching a terminal state? → Stop polling and show a manual-refresh prompt.
- What happens if the quote timestamp is older than 10 seconds when the user submits? → Re-fetch quote automatically before submitting or warn the user.
- What happens if `lp_id` is not found on Remove Liquidity? → Show "Position not found or already withdrawn" error inline.
- What happens if Sign Resume is called when quorum is already met? → Show the API error inline; status will reflect `LIVE`.
- What happens when `imbalance_flag = true` on the pool? → Show an imbalance banner on both the bank AMM page and the governance Liquidity page.

---

## Requirements

### Functional Requirements

#### Build-Time Gating

- **FR-001**: The build-time variable `VITE_SCENARIO` (values: `a`, `b`) MUST gate all Scenario B routes, pages, and sidebar items. When `VITE_SCENARIO != "b"`, none of the Scenario B code paths are reachable.
- **FR-002**: When `VITE_SCENARIO` is absent or any value other than `"b"`, the application MUST behave identically to the current Scenario A baseline.
- **FR-003**: Scenario A pages, routes, stores, and service files MUST NOT be modified, with the following exceptions: `AMMTradingPage.tsx` and `CircuitBreakerPage.tsx` MUST be modified solely to add an `isScenarioB` branch; the `false` branch MUST render their existing Scenario A content unchanged and all existing Scenario A tests MUST continue to pass.

#### bank App — Sidebar & Navigation (Scenario B)

- **FR-004**: When `VITE_SCENARIO=b`, the `bank` sidebar MUST show: Bridge, AMM Trading. Scenario A routes (HTLC, FX Agreements, Deposits, Escrows, Redeems) MUST NOT be registered in the route tree and their sidebar items MUST NOT be rendered; the corresponding page files remain on disk but are unreachable.
- **FR-005**: All Scenario B routes in the `bank` app MUST be nested under `ProtectedRoute` and `AppLayout` exactly as Scenario A routes are.

#### bank App — Bridge Page (`/bridge`)

- **FR-006**: The Bridge page MUST display a Lock & Mint form with fields: `owner_bank_id`, `spoke_network`, `native_asset`, `mirrored_asset` (optional), `amount`.
- **FR-007**: On successful Lock & Mint submission, the returned `position_id` MUST be retained in component or store state and the new position MUST appear in the Bridge Positions table immediately.
- **FR-008**: The Bridge page MUST display a Burn & Unlock form that accepts a `position_id` input.
- **FR-009**: After Burn & Unlock is initiated, the associated position row in the Bridge Positions table MUST reflect the updated state.
- **FR-010**: The Bridge Positions table MUST load all positions on initial render by calling `GET /api/v2/bridge/positions` with no `state` filter parameter. A state filter UI dropdown MUST be available for user-driven filtering; this filter is applied client-side and MUST NOT alter the API request. The table MUST poll the same endpoint (no state filter) every 5 seconds while at least one position is in a non-terminal state (`LOCKING`, `ACTIVE`, `BURNED`). After a Lock&Mint submission, the newly created position MUST be identified in polling responses by matching `position_id` client-side.
- **FR-011**: Polling MUST stop after 120 seconds if no position has reached a terminal state during that window; a timeout notice MUST be shown on affected rows.
- **FR-012**: Bridge state badges MUST visually distinguish: `LOCKING` (neutral), `ACTIVE` (success/green), `BURNED` (info/blue), `UNLOCKED` (muted/complete), `RECONCILIATION_REQUIRED` (error/red), `FAILED` (error/red).

#### bank App — AMM Trading Page (`/amm`, Scenario B)

- **FR-013**: The existing `AMMTradingPage.tsx` MUST be modified to branch on `isScenarioB`. When `true`, render the full v2 flow (real `/api/v2` calls, bridge awareness, slippage input, pool status from the v2 endpoint). When `false`, render the existing mock-backed Scenario A content unchanged. No separate `AMMTradingPageV2.tsx` file is created.
- **FR-014**: The page MUST include a Quote form with inputs `pair` and `amount_out`, displaying `required_input`, `price_impact` (formatted as a percentage), and `quote_timestamp`. If the quote is older than 10 seconds, a staleness warning MUST be shown.
- **FR-015**: The page MUST include a Swap form that pre-fills `pair` and `amount_out` from the latest quote, and adds a `max_amount_in` field (slippage-computed or manually entered) and `payer_id`, `beneficiary_id`.
- **FR-016**: Swap error codes `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY`, `ZK_VALIDATION_FAILED`, `CIRCUIT_BREAKER_HALTED` MUST each show a distinct, user-readable error message as specified in the runbook.
- **FR-017**: When `GET /api/v2/governance/circuit-breaker/status` returns `state: HALTED`, a banner "Swaps temporarily suspended by Central Bank" MUST be shown and the Swap Submit button MUST be disabled.
- **FR-018**: When `GET /api/v2/amm/pool/:pair/status` returns `imbalance_flag: true`, a pool imbalance warning banner MUST be shown.
- **FR-019**: ~~The AMM page MUST include an Approve AMM panel (collapsible or section) with inputs `amount_a` and `amount_b`, calling `POST /api/v2/amm/token/approve-amm`.~~ ⚠️ **Superseded by spec `006` FR-022 (FR-018 breaking change — 2026-05-19)**: payload changed to `{amount: string, side: "A" | "B"}`. Fields `amount_a` and `amount_b` are deprecated; backend returns HTTP 400 `DEPRECATED_FIELDS` for old payloads.

#### governance App — Sidebar & Navigation (Scenario B)

- **FR-020**: When `VITE_SCENARIO=b`, the `governance` sidebar MUST show: Dashboard, Liquidity Management, Circuit Breaker, Oversight. Scenario A routes (HTLC Monitor, Deposits Approval, Escrows Approval, Redeems Approval) MUST NOT be registered in the route tree and their sidebar items MUST NOT be rendered; the corresponding page files remain on disk but are unreachable.
- **FR-021**: Existing Scenario A governance pages MUST NOT be modified.

#### governance App — Liquidity Management Page (`/liquidity`)

- **FR-022**: The Liquidity Management page MUST include a Pool Status card showing `reserve_a`, `reserve_b`, `current_ratio`, `updated_at`, and an imbalance banner when `imbalance_flag = true`.
- **FR-023**: The page MUST include an Add Liquidity form with fields: `pool_pair`, `token_a_amount`, `token_b_amount`, `provider_bank_id`.
- **FR-024**: On successful Add Liquidity, the returned LP position (including `lp_id`) MUST appear in an LP Positions table. The table is populated exclusively from Add Liquidity API responses in the current browser session; there is no `GET /api/v2/amm/liquidity/positions` endpoint and the table MUST NOT poll one.
- **FR-025**: The page MUST include a Remove Liquidity form accepting `lp_id`, `pool_pair`, `provider_bank_id`.
- **FR-026**: The LP Positions table MUST display at minimum: `lp_id`, `pool_pair`, `provider_bank_id`, `token_a_amount`, `token_b_amount`, `lp_shares`, `status`, `added_at`.
- **FR-027**: ~~The page MUST include a MintAndApprove panel with fields: `amount_a`, `amount_b`, optional `recipient`, calling `POST /api/v2/amm/token/mint-and-approve`.~~ ⚠️ **Superseded by spec `006` FR-016/FR-017/FR-019 (FR-018 breaking change — 2026-05-19)**: payload changed to `{amount: string, recipient?: string}`. Fields `amount_a` and `amount_b` are deprecated; backend returns HTTP 400 `DEPRECATED_FIELDS` for old payloads.

#### governance App — Circuit Breaker Page (`/circuit-breaker`, Scenario B)

- **FR-028**: The existing `CircuitBreakerPage.tsx` MUST be modified to branch on `isScenarioB`. When `true`, render the v2 multi-party flow (pause form, propose-resume, sign-resume, state machine display). When `false`, render the existing v1 toggle logic unchanged. No separate `CircuitBreakerPageV2.tsx` file is created.
- **FR-029**: The page MUST display the current state (`LIVE` / `HALTED` / `RESUME_PENDING`) fetched from `GET /api/v2/governance/circuit-breaker/status`, polling at ≤ 15 second intervals.
- **FR-030**: The Pause action form MUST accept `pair`, `bank_id`, `reason_code`, and a text input labeled **"Institutional Signature (base64)"** pre-populated with `"AA="` as the dev placeholder; the user can override it with any valid base64 string.
- **FR-031**: The Propose Resume action form MUST accept `pair`, `bank_id`, and a text input labeled **"Institutional Signature (base64)"** pre-populated with `"AA="`. On success, the returned `request_id` MUST be displayed prominently for sharing.
- **FR-032**: The Sign Resume action form MUST accept `pair`, `request_id`, `bank_id`, and a text input labeled **"Institutional Signature (base64)"** pre-populated with `"AA="`. When the API returns `state: LIVE`, the status card MUST update accordingly.
- **FR-033**: When `state = RESUME_PENDING`, the `resume_request_id` from the status response MUST be shown so operators can share it for co-signing.

#### governance App — Oversight / Disclosure Page (`/oversight`)

- **FR-034**: The Oversight page MUST include an Open Disclosure Request form with fields: `tx_ref`, `requestor_id`, `reason_code`.
- **FR-035**: On successful submission, the returned disclosure object (including `request_id`, `state`, `quorum_reached`, `quorum_required`, `expires_at`) MUST be displayed.
- **FR-036**: The page MUST include a Sign Disclosure form with fields: `request_id`, `signer_id`.
- **FR-037**: The page MUST include a Disclosure Status lookup by `request_id`, showing state badge, quorum progress (e.g., `1/2`), and timestamps.
- **FR-038**: State badges MUST visually map: `PENDING` (neutral), `QUORUM_REACHED` (success), `EXPIRED` (warning/muted), `REJECTED` (error).

#### Shared Constraints

- **FR-039**: All Scenario B API service modules MUST use the real HTTP client (Axios). No mock fallback is permitted in Scenario B code paths.
- **FR-040**: Token amounts in all forms and displays MUST be treated as integer strings (no decimal input; no decimal display).
- **FR-041**: All `signature` fields in forms MUST be rendered as a text input labeled **"Institutional Signature (base64)"**, pre-populated with `"AA="` as the default value. The user can override it with any valid base64 string. This applies to all governance and oversight forms (Pause, Propose Resume, Sign Resume).
- **FR-042**: TypeScript enums MUST NOT be used. State values MUST be represented as const-object + union type alias (e.g., `const BRIDGE_STATE = { LOCKING: "LOCKING", ... } as const; type BridgeState = typeof BRIDGE_STATE[keyof typeof BRIDGE_STATE];`).
- **FR-043**: Polling implementations MUST use `setInterval` + `clearInterval` cleanup inside `useEffect`, or a dedicated `usePolling` hook placed at `frontend/apps/{bank,governance}/src/hooks/usePolling.ts` (one per app). The hook MUST NOT be added to the shared `@cbweb3/ui` package. All polling logic MUST clean up on component unmount.

### Key Entities

- **BridgedAssetPosition**: Represents a cross-spoke bridging operation. Key fields: `position_id`, `owner_bank_id`, `spoke_network`, `native_asset`, `mirrored_asset`, `mirrored_amount`, `bridge_state`, `relayer_retries`, `relayer_error_log`.
- **SwapOrder**: Represents an AMM swap execution. Key fields: `order_id`, `tx_hash`, `amount_in`, `state`, `confirmed_at`.
- **AMMQuote**: Represents a quoted price before committing to a swap. Key fields: `required_input`, `price_impact`, `quote_timestamp`.
- **PoolStatus**: Real-time state of an AMM pool. Key fields: `pool_pair`, `reserve_a`, `reserve_b`, `current_ratio`, `imbalance_flag`, `updated_at`.
- **LiquidityPosition**: An LP deposit by a central bank. Key fields: `lp_id`, `pool_pair`, `provider_bank_id`, `token_a_amount`, `token_b_amount`, `lp_shares`, `status`, `added_at`.
- **CircuitBreakerStatus**: Governance state of the AMM. Key fields: `pair`, `state`, `pause_initiator`, `pause_reason`, `resume_request_id`.
- **DisclosureRequest**: AML/CFT oversight request. Key fields: `request_id`, `state`, `quorum_reached`, `quorum_required`, `expires_at`, `closed_at`.

---

## Technical Design

### Architecture: Build-Time Scenario Flag

The flag `VITE_SCENARIO` is read as `import.meta.env.VITE_SCENARIO` at build time. Conditional imports are resolved via a dedicated `scenarioConfig.ts` helper that exports:

```ts
export const isScenarioB = import.meta.env.VITE_SCENARIO === "b";
```

Routes in `routes/index.tsx` (both apps) branch on `isScenarioB` to register either Scenario A or Scenario B page components. Since this is a build-time constant, tree-shaking eliminates the unused scenario's bundle.

### File Layout

#### bank app additions

```
src/
  features/
    bridge/
      BridgePage.tsx              ← new (Scenario B)
      bridge.store.ts             ← Zustand store for positions
    amm/
      AMMTradingPage.tsx          ← MODIFIED: isScenarioB branch added (v2 flow when true, Scenario A when false)
      amm-v2.store.ts             ← new: Zustand store for v2 quotes/swaps/pool status
  services/
    api/
      bridge.api.ts               ← new: calls /api/v2/bridge/*
      amm-v2.api.ts               ← new: calls /api/v2/amm/* (real, no mocks)
  types/
    bridge.types.ts               ← BridgedAssetPosition, BridgeState, etc.
    amm-v2.types.ts               ← AMMQuote, SwapOrder, PoolStatus, etc.
  hooks/
    usePolling.ts                 ← new, app-local (NOT in @cbweb3/ui); setInterval + cleanup
  routes/
    index.tsx                     ← add Scenario B route branches; Scenario A routes NOT registered when VITE_SCENARIO=b
```

#### governance app additions

```
src/
  features/
    liquidity/
      LiquidityManagementPage.tsx ← new (Scenario B)
      liquidity.store.ts
    circuit-breaker/
      CircuitBreakerPage.tsx      ← MODIFIED: isScenarioB branch added (v2 multi-party flow when true, v1 when false)
      circuit-breaker-v2.store.ts ← new: Zustand store for v2 circuit-breaker state (Scenario B only)
    oversight/
      OversightPage.tsx           ← new (Scenario B)
      oversight.store.ts
  services/
    api/
      liquidity.api.ts            ← new: /api/v2/amm/liquidity/*
      circuit-breaker-v2.api.ts   ← new: /api/v2/governance/circuit-breaker/*
      oversight.api.ts            ← new: /api/v2/oversight/*
  types/
    liquidity.types.ts
    circuit-breaker-v2.types.ts
    oversight.types.ts
  hooks/
    usePolling.ts                 ← new, app-local (NOT in @cbweb3/ui); setInterval + cleanup
  routes/
    index.tsx                     ← add Scenario B route branches; Scenario A routes NOT registered when VITE_SCENARIO=b
```

### Routing Strategy

Both apps use `VITE_SCENARIO` to select which page component is mounted per route. The route paths remain the same where applicable (e.g., `/amm`, `/circuit-breaker`) so bookmarks and auth guards do not need changes. New-only paths (`/bridge`, `/liquidity`, `/oversight`) are added to the Scenario B branch only.

### Polling

Each app implements its own `usePolling(callback, intervalMs, enabled)` hook at `frontend/apps/{bank,governance}/src/hooks/usePolling.ts`. The hook encapsulates `setInterval` + `clearInterval` cleanup and is NOT placed in the shared `@cbweb3/ui` package. Components pass `enabled = positions.some(p => isNonTerminal(p.bridge_state))` for bridge polling and `enabled = true` for circuit-breaker and pool-status polling.

### API Services

All Scenario B API files import `httpClient` from their app's `services/api/http-client.ts` directly — the same authenticated Axios instance used by Scenario A. No dual mock/real branching is needed in Scenario B files.

### State Management

Each Scenario B feature has a dedicated Zustand store. Stores are not imported or registered in any Scenario A file. Store files are imported only from their corresponding Scenario B page components and route files.

---

## API Contract Summary

| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | `/api/v2/amm/quote/exact-output?pair&amount_out` | — (public) | Get a price quote |
| POST | `/api/v2/amm/swap/exact-output` | commercial_bank | Execute a swap |
| GET | `/api/v2/amm/pool/:pair/status` | — (public) | Pool reserves and imbalance flag |
| POST | `/api/v2/amm/token/approve-amm` | commercial_bank | Approve AMM allowance — ⚠️ payload `{amount, side}` (ver spec 006 FR-022; `amount_a`/`amount_b` obsoletos) |
| POST | `/api/v2/bridge/lock-mint` | commercial_bank | Initiate Lock & Mint |
| POST | `/api/v2/bridge/burn-unlock` | commercial_bank | Initiate Burn & Unlock |
| GET | `/api/v2/bridge/positions?state=` | commercial_bank | List bridge positions (optional state filter) |
| POST | `/api/v2/amm/liquidity/add` | central_bank | Add LP position |
| POST | `/api/v2/amm/liquidity/remove` | central_bank | Remove LP position |
| POST | `/api/v2/amm/token/mint-and-approve` | central_bank | Mint Hub tokens and approve AMM — ⚠️ payload `{amount, recipient?}` (ver spec 006 FR-016; `amount_a`/`amount_b` obsoletos) |
| GET | `/api/v2/governance/circuit-breaker/status?pair=` | — (public) | Circuit breaker state |
| POST | `/api/v2/governance/circuit-breaker/pause` | central_bank | Pause AMM immediately |
| POST | `/api/v2/governance/circuit-breaker/resume-request` | central_bank | Propose resume (returns `request_id`) |
| POST | `/api/v2/governance/circuit-breaker/resume-sign` | central_bank | Co-sign resume proposal |
| POST | `/api/v2/oversight/disclosure-request` | central_bank | Open AML disclosure request |
| POST | `/api/v2/oversight/disclosure-sign` | central_bank | Co-sign disclosure request |
| GET | `/api/v2/oversight/disclosure-status/:requestID` | central_bank | Get disclosure status and quorum |

All requests use `Content-Type: application/json` and `Authorization: Bearer <JWT>` (except public endpoints). All amount fields are integer strings. Signature fields accept base64 strings; `"AA=="` is the dev placeholder.

---

## Success Criteria

### Measurable Outcomes

- **SC-001**: A commercial bank operator can complete the end-to-end cross-border payment flow (Lock & Mint → poll to ACTIVE → Swap → Burn & Unlock → poll to UNLOCKED) without leaving the `bank` app or encountering unhandled UI errors.
- **SC-002**: Bridge position state transitions are reflected in the UI within 10 seconds of the backend state change (5s poll interval + render).
- **SC-003**: A central bank operator can pause the AMM, propose a resume, and co-sign the resume — completing the full circuit-breaker governance cycle — without page reload or manual navigation.
- **SC-004**: A central bank operator can open and progress a disclosure request from `PENDING` → `QUORUM_REACHED` entirely within the Oversight page.
- **SC-005**: All Scenario B network calls target real `/api/v2` endpoints. Zero mock API calls are reachable when `VITE_SCENARIO=b`.
- **SC-006**: Scenario A apps (`VITE_SCENARIO=a` or unset) are unaffected: all existing Scenario A tests pass without modification after merging this feature.
- **SC-007**: The circuit-breaker halted banner appears on the bank AMM page within one poll cycle (≤ 15 seconds) of the backend state changing to `HALTED`.
- **SC-008**: No TypeScript compilation errors (`tsc --noEmit`) are introduced by any Scenario B file.

---

## Assumptions

1. The existing `httpClient` (Axios instance) in each app already handles Bearer JWT injection and token refresh. No changes to auth interceptors are required for Scenario B API calls.
2. The `@cbweb3/ui` shared package's existing components (`Badge`, `Button`, `Card`, `Input`, `Label`, `Textarea`, `toast`) are sufficient for all new Scenario B UI. No new component primitives need to be added to the shared package.
3. The pool pair used in all initial UI defaults is `BRL-USD`. Other pairs can be entered by the user as free text.
4. `VITE_SCENARIO` is set at Docker Compose build time via `args:` in the frontend service definition. The build tooling already supports this.
5. The `usePolling` hook is new (does not yet exist in either app). It MUST be placed in each app's own `hooks/` directory (`frontend/apps/{bank,governance}/src/hooks/usePolling.ts`). It MUST NOT be extracted into the shared `frontend/packages/ui/src/hooks/` directory.
6. The `signature` field in governance and oversight forms is rendered as a text input labeled **"Institutional Signature (base64)"**, pre-populated with `"AA="` for dev environments. In production, an external signing tool provides the base64 value. The UI accepts any non-empty base64 string as an override.
7. LP positions are not persisted in local state beyond the current browser session; the table is populated exclusively from Add Liquidity API responses. There is no `GET /api/v2/amm/liquidity/positions` endpoint and the table MUST NOT poll one.
8. The governance app's existing `CircuitBreakerPage.tsx` uses the v1 API. It is modified in-place to add an `isScenarioB` branch: when `true`, it renders the v2 multi-party flow; when `false`, it renders the existing v1 toggle logic unchanged. No `CircuitBreakerPageV2.tsx` file is created.
