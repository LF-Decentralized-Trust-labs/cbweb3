# Feature Specification: Bank Swap Wizard Refactor

**Feature Branch**: `011-bank-swap-wizard-refactor`
**Created**: 2026-05-28
**Status**: Draft

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Commercial Bank Operator Completes Cross-Currency Swap in One Guided Workflow (Priority: P1)

A commercial bank operator needs to exchange fiat reserve tokens across currencies (e.g., BRL → ARS) on behalf of the bank. Today, they must navigate four separate pages in sequence — Deposits, Escrows, AMM Trading, Redeems — with no guidance on the order, no automatic handoff between steps, and no single view showing overall progress. A single mistake (e.g., entering the wrong payer bank identity) can corrupt the operation.

The operator should be able to complete the entire flow — from requesting token issuance through redemption of the exchanged fiat — in a single guided wizard, where each step is pre-filled from the previous one and the system advances automatically upon CB approval.

**Why this priority**: This is the primary business workflow for Scenario B banks. All audit findings that create security or data correctness risks are surfaced through this flow.

**Independent Test**: Operator logs in, navigates to the swap wizard, enters a fiat amount, and follows all five guided steps to completion. At the end, a redemption confirmation is shown with no manual re-entry of IDs between steps.

**Acceptance Scenarios**:

1. **Given** an authenticated commercial bank operator, **When** they open the swap wizard, **Then** Step 1 (token issuance request) is shown with only a fiat amount input — no bank identity or side fields.
2. **Given** a completed Step 1 with an `APPROVED` deposit, **When** the system detects the status change automatically, **Then** the wizard advances to Step 2 (reserve tokenisation) with the deposit ID pre-filled and read-only.
3. **Given** a completed Step 2 with an `APPROVED` escrow, **When** the system detects the status change, **Then** the wizard advances to Step 3 (AMM spending approval) with the approve amount pre-filled from the full escrow amount but editable; the operator's current tCeBM balance is displayed alongside the input; entering an amount exceeding the balance triggers a non-blocking warning without blocking submission.
4. **Given** Step 3 submitted successfully, **When** the operator is on Step 4 (cross-currency swap), **Then** `payer_bank_id` is shown as read-only and sourced from the operator's authenticated session — the operator cannot modify it.
5. **Given** the operator has obtained a quote in Step 4, **When** they execute the swap, **Then** the interface remains responsive and shows progress stages (bridge-in → swap → bridge-out) for up to 5 minutes without timing out.
6. **Given** the swap completes successfully, **When** Step 5 (redemption) is shown, **Then** the token amount is pre-filled from the swap output and the operator only needs to confirm.
7. **Given** the bridge-out step fails during swap execution, **When** the error is returned, **Then** a persistent critical alert is shown containing the swap ID and a reconciliation message; the operator is not left in an ambiguous state.
8. **Given** the bank app is running in Scenario B mode (`VITE_SCENARIO=b`), **When** the operator views the navigation menu, **Then** the Deposits, Escrows, and AMM Trading entries are not listed; the Swap Wizard (`/swap`) is the primary navigation entry point. The individual page routes remain accessible via direct URL.

---

### User Story 2 — Power User Accesses Corrected AMM Trading Page Without Data Corruption (Priority: P2)

An advanced operator who wants direct access to AMM operations (e.g., approve-AMM, get quote, execute cross-currency swap individually) relies on the existing AMM Trading page. Currently, this page displays corrupted values (e.g., price impact rendered as a raw decimal string, quote timestamps shown as raw integers), exposes an editable `payer_bank_id` field that should be locked to the session identity, and shows a currency pair default (`BRL-USD`) that does not match the active deployment.

After this feature, the page must render all values correctly, lock sensitive fields to the session, and have no dead code paths that could cause confusion or errors.

**Why this priority**: The page is used by operators who want finer control. Data display bugs and the security gap with editable `payer_bank_id` must be fixed regardless of the wizard.

**Independent Test**: Power user navigates to the AMM Trading page, requests a quote, and sees: price impact as a percentage (e.g., "0.25%"), quote timestamp as a human-readable date, the default pair as BRL-ARS, and `payer_bank_id` as a non-editable field matching their session. Executing a swap does not time out in under 5 minutes.

**Acceptance Scenarios**:

1. **Given** an authenticated operator on the AMM Trading page, **When** a quote is returned, **Then** the price impact is displayed as a percentage derived from a decimal value (not a raw multiplication of a string), and the quote timestamp is shown as a human-readable date-time.
2. **Given** the AMM Trading page loads, **When** the default currency pair is rendered, **Then** it shows `BRL-ARS` (not `BRL-USD`).
3. **Given** the operator views the cross-currency swap form, **When** the form is rendered, **Then** the `payer_bank_id` field is read-only and populated from the operator's authenticated session; no text input is presented for it.
4. **Given** the operator submits an AMM approval without selecting a side, **When** the request is sent, **Then** no `side` field is included in the request — the bank's side is resolved server-side.
5. **Given** the page loads in Scenario B mode, **When** the component tree is rendered, **Then** no legacy Scenario A code paths are executed or rendered.

---

### User Story 3 — Central Bank Operator Monitors Scenario B Swap Operations (Priority: P3)

A Central Bank (CB) operator uses the governance app to approve deposits, escrows, and redeems. For Scenario B, they also need visibility into in-progress and completed cross-currency swap operations — to correlate swap requests with the token flows they are approving, and to detect stuck or failed swaps.

Currently, there is no monitoring view for swap operations in the governance app. CB operators must consult raw logs or contact bank operators directly.

**Why this priority**: This improves operational observability for CB operators but does not block bank flows from working. It is additive.

**Independent Test**: CB operator logs into the governance app under Scenario B mode, navigates to the swap monitor page, and sees a list of recent swap operations with their statuses. The page also links to the existing deposit, escrow, and redeem approval pages.

**Acceptance Scenarios**:

1. **Given** a CB operator in the governance app under Scenario B, **When** they navigate to `/swap-monitor`, **Then** a dashboard is shown with: (1) counts of pending deposits, escrows, and redeems each linking to the corresponding existing approval page, and (2) a swap history section that fetches individual swap records via `GET /api/v2/amm/swap/cross-currency/{id}` for each swap ID stored in the current browser session (Zustand); if no swap IDs are tracked in the session, the swap history section shows "No swap records in this session".
2. **Given** the swap monitor page, **When** the operator clicks a link to Deposits/Escrows/Redeems approvals, **Then** they are taken to the corresponding existing approval page.
3. **Given** the governance app is running in Scenario A mode, **When** the operator views the navigation, **Then** the swap monitor link is not shown.
4. **Given** the swap monitor page has no swap IDs in session state, **When** the swap history section renders, **Then** "No swap records in this session" is displayed without error.
5. **Given** the swap monitor page is rendered, **When** the operator views it, **Then** no approval actions (approve/reject) are available — the page is read-only; all approval actions remain on the existing approval pages.

---

### Edge Cases

- What happens if the CB does not approve a deposit within the polling window? The wizard shows the pending state persistently with a "still waiting" indicator; the operator can leave and return without losing progress.
- What happens if the bank's swap side is not configured on the gateway? Step 3 (AMM approval) shows an actionable error: "bank swap side not configured on this gateway — contact your administrator."
- What happens if a quote expires before the operator executes the swap? The quote expiry countdown reaches zero and the system prevents execution, prompting the operator to request a new quote.
- What happens if the swap orchestration exceeds 5 minutes? A timeout error is shown with the swap ID for support reference; the operator is not left in an ambiguous state.
- What happens if the operator refreshes the browser mid-wizard? The wizard restarts from Step 1 (session-only state, no persistence).
- What happens if a swap ID stored in session state cannot be fetched on the governance monitoring page? The corresponding record shows an inline error state; all other tracked swap records are still displayed. If no swap IDs are tracked in the current session, the swap history section shows "No swap records in this session" without error — no list endpoint exists for cross-currency swap operations.

---

## Requirements *(mandatory)*

### Functional Requirements

**Wizard — Commercial Bank App**

- **FR-001**: The system MUST provide a single-page guided workflow covering token issuance request, reserve tokenisation request, AMM spending approval, cross-currency swap, and redemption request — in that order.
- **FR-002**: The wizard MUST automatically advance to the next step when the current step reaches its approval or completion status, without operator intervention.
- **FR-003**: Identifiers produced in each step (deposit ID, escrow ID, swap ID) MUST be automatically carried forward as pre-filled, read-only inputs to subsequent steps.
- **FR-004**: The operator's bank identity (`payer_bank_id`) MUST be sourced exclusively from their authenticated session and MUST NOT be modifiable by the operator.
- **FR-005**: The swap side (A or B) MUST NOT be selectable by the operator; it MUST be resolved server-side based on the bank's configuration.
- **FR-006**: The wizard MUST poll for status changes every 5 seconds on steps awaiting CB approval (token issuance, reserve tokenisation, redemption).
- **FR-007**: The swap execution step MUST remain active and show progress indicators for up to 5 minutes to accommodate synchronous backend processing.
- **FR-008**: If the bridge-out stage of the swap fails, the wizard MUST display a persistent critical alert with the swap ID and a reconciliation instruction; this alert MUST NOT be dismissible.
- **FR-009**: The currency pair active in the wizard MUST default to BRL-ARS.
- **FR-010**: The pool status for the active pair MUST be shown in the swap step, refreshed at a low frequency to avoid unnecessary load.
- **FR-011**: The wizard MUST show a quote with effective rate and input amount before the operator can execute a swap; execution MUST be blocked if no valid quote is held.

**AMM Trading Page — Fixes**

- **FR-012**: The price impact value returned from the quote endpoint MUST be rendered as a human-readable percentage without silent arithmetic errors on non-numeric types.
- **FR-013**: The quote timestamp MUST be rendered as a human-readable date-time, not as a raw numeric value.
- **FR-014**: All legacy code paths belonging to the deprecated Scenario A AMM flow MUST be removed from the bank app.
- **FR-015**: The `payer_bank_id` field in the cross-currency swap form MUST be read-only and populated from the authenticated session.
- **FR-016**: The AMM spending approval form MUST NOT include a swap side selector; the side MUST be omitted from the request payload.
- **FR-017**: The default currency pair displayed on the AMM Trading page MUST be BRL-ARS.
- **FR-018**: The `crossCurrencyExecuteClient` MUST use a 300_000ms HTTP timeout. The backend orchestrator is synchronous and holds the connection until the operation completes or fails; no polling pattern is required. The 300s timeout serves as a client-side safety net.

**Governance App — Swap Monitor**

- **FR-019**: The governance app MUST provide a monitoring view at `/swap-monitor` (Scenario B only) showing: (1) counts of pending deposits, escrows, and redeems with links to the existing approval pages, and (2) a swap history section. No list endpoint exists; the page fetches individual swap records via `GET /api/v2/amm/swap/cross-currency/{id}` using swap IDs stored in browser session state (Zustand). If no swap IDs are tracked in the current session, the swap history section MUST display "No swap records in this session". The page MUST be read-only.
- **FR-020**: The swap monitoring view MUST link to the existing deposit, escrow, and redeem approval pages.
- **FR-021**: The swap monitoring view MUST only be visible when the governance app is running in Scenario B mode.
- **FR-022**: *(Wizard — Bank App)* When `VITE_SCENARIO=b`, the bank app navigation menu MUST hide entries for `DepositsPage`, `EscrowsPage`, and `AMMTradingPage`; their routes MUST remain accessible via direct URL for power users and debugging.
- **FR-023**: *(Wizard — Step 3)* The approve-AMM amount field in Step 3 MUST be pre-filled with the full escrow amount from the approved escrow record but MUST remain editable. The operator's current tCeBM balance MUST be displayed alongside the input. If the entered amount exceeds the balance, a non-blocking warning MUST be shown; form submission MUST NOT be prevented (backend validation applies).

### Key Entities *(include if feature involves data)*

- **SwapWizardSession**: Represents the in-progress state for a single operator's guided swap flow. Holds references to deposit ID, escrow ID, swap ID, redeem ID, and amounts at each step. Session-scoped (not persisted across page reloads).
- **SwapOperation**: A completed or in-progress cross-currency swap record. Key attributes: swap ID, payer bank ID, beneficiary bank ID, bridge-in position ID, swap transaction reference, bridge-out position ID, status (PENDING / BRIDGE_IN_PROGRESS / SWAP_IN_PROGRESS / BRIDGE_OUT_PROGRESS / COMPLETED / BRIDGE_OUT_FAILED).
- **AMMQuote**: A point-in-time price quote for a cross-currency swap. Key attributes: effective rate (decimal), required input amount, price impact (decimal fraction), quote timestamp (Unix epoch), 15-second TTL.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Commercial bank operators can complete the full fiat-to-fiat cross-currency swap flow — from token issuance request to redemption confirmation — without navigating to more than one page.
- **SC-002**: Zero cases where a commercial bank operator can submit a swap with a `payer_bank_id` different from their authenticated session identity.
- **SC-003**: Zero cases where the swap side (A or B) is selected by the operator in the commercial bank workflow.
- **SC-004**: Quote price impact and timestamp values are displayed accurately (no `undefined`, no raw integers shown as timestamps, no string multiplication errors) in 100% of quote responses received from the backend.
- **SC-005**: Swap execution remains in progress for up to 5 minutes (300_000ms `crossCurrencyExecuteClient` timeout) without the interface reporting a client-side timeout. No polling is used — the synchronous backend connection is held until the orchestrator returns COMPLETED or an error.
- **SC-006**: When bridge-out fails, 100% of operators are shown the swap ID and reconciliation instructions without needing to query backend logs.
- **SC-007**: CB operators can view the status of recent cross-currency swap operations from the governance app without accessing backend logs.
- **SC-008**: The bank app contains no unreachable code paths from the deprecated Scenario A AMM flow.

---

## Assumptions

- All backend API endpoints referenced in this feature (deposits, escrows, redeems, AMM approve, cross-currency swap, quote, pool status, swap list) are already implemented, stable, and require no changes.
- The `usePolling` hook already exists in the bank app and can be reused without modification.
- No new external packages or libraries need to be added to implement this feature.
- The wizard (`/swap`) is a new page alongside the existing standalone pages (Deposits, Escrows, AMM Trading). In Scenario B, the bank app navigation menu hides entries for those standalone pages; their routes remain accessible via direct URL for power users and debugging. Power users can still access the AMM Trading page directly.
- Scenario A flows (pages, components, stores) are entirely out of scope and must not be changed.
- The wizard's in-progress state is session-only and is intentionally lost on page reload; no durable wizard session persistence is required.
- No list endpoint for cross-currency swap operations exists. The governance monitoring page (`SwapMonitorPage`) tracks swap IDs in browser session state (Zustand) and fetches individual records via `GET /api/v2/amm/swap/cross-currency/{id}`. Swap history visibility is limited to operations initiated in the current browser session.
- The governance app's existing deposit, escrow, and redeem approval pages require no changes for this feature.
- The `BANK_CODE` environment variable is the mechanism by which the server side determines swap side; this configuration is an operator/infrastructure concern, not a user-facing input.
- The active deployment pair is BRL-ARS; BRL-USD is deprecated and no longer in use.
- The cross-currency swap backend orchestrator is synchronous; it holds the HTTP connection open until the operation completes or fails. A 300_000ms client-side timeout is the correct safeguard. No fire-and-forget or polling pattern is needed for the swap execution call.

---

## Clarifications

### Session 2026-05-28

- Q: Wizard routing and standalone page visibility in Scenario B → A: Remove `DepositsPage`, `EscrowsPage`, and `AMMTradingPage` from the bank app navigation menu when `VITE_SCENARIO=b`; keep routes accessible via direct URL for power users and debugging. `CommercialSwapWizard` at `/swap` is the primary Scenario B navigation entry point; existing standalone pages serve as fallback reference views.
- Q: Approve-AMM amount in Step 3 of the wizard → A: Pre-fill with the full escrow amount from the approved escrow record but keep the field editable. Display the operator's current tCeBM balance alongside the input. If the entered amount exceeds the balance, show a non-blocking warning; do not prevent submission — backend validates.
- Q: Cross-currency swap HTTP timeout (300s vs polling) → A: Use 300_000ms on `crossCurrencyExecuteClient`. Backend orchestrator is synchronous and holds the HTTP connection until COMPLETED or failure; polling is not the right pattern. 300s client-side timeout is a safety net. No fire-and-forget or polling pattern change needed.
- Q: SwapMonitorPage vs enhancement of existing approval pages → A: `SwapMonitorPage` is a new aggregated read-only view in the governance app at `/swap-monitor` (Scenario B only). Does NOT replace or modify existing approval pages. Shows: (1) counts of pending deposits/escrows/redeems with links to existing approval pages, and (2) recent swap activity fetched via `GET /api/v2/amm/swap/cross-currency/{id}` for swap IDs stored in Zustand session state. No list endpoint exists. If no swap IDs are tracked in the current session, shows "No swap records in this session".
