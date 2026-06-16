# Feature Specification: Supervisor Portal (R2-CR-8)

**Feature Branch**: `014-supervisor-portal`
**Created**: 2026-06-11
**Status**: Draft
**Scope**: Scenarios A and B — each scenario's portal is implemented independently under its own frontend and backend directories.

---

## Overview

The Supervisor Portal is the regulatory oversight interface for central bank supervisors. It provides
three core capabilities across both scenarios:

1. **Audit & Supervision** — paginated, filterable read-only access to the immutable audit log via
   a dedicated route `GET /api/v1/compliance/audit/logs` registered under a new `SupervisorHandler`.
   This decouples supervisor access from `GovernanceHandler` (which already owns governance-facing
   endpoints) and makes RBAC enforcement explicit at the handler level rather than via inline role
   checks inside a shared handler.

2. **ZK Pointer Compliance Verification** — a tool for supervisors to validate that a ZK-Pointer
   commitment hash registered for a given bank is VALID and non-expired in the Trust Registry. This
   is unique to Scenario B (which uses `ZetoToken` + ZKP-based compliance gates). Scenario A's
   equivalent is a credential/KYC status lookup against the `IdentityRegistry`.

3. **Investigation Module (Deanonymisation)** — an interface to initiate, co-sign, track, and close
   Master Viewing Key disclosure requests for specific shielded transactions that have triggered
   AML/CFT thresholds. The multi-party quorum model (2-of-N central bank signatures required) prevents
   unilateral deanonymisation. In Scenario B the backend exists; in Scenario A it must be built.

Both scenarios share the same React scaffold under `frontend/apps/supervisor/` but differ in:
- Which API routes they call (v1 for Scenario A, v1 + v2 for Scenario B).
- Whether ZK Pointer Verification applies (Scenario B only; Scenario A shows credential status instead).
- Whether the Investigation Module calls existing oversight endpoints (Scenario B) or newly built ones
  (Scenario A).

---

## Current State (as of 2026-06-11)

### What already exists

| Layer | Scenario A | Scenario B |
|---|---|---|
| Frontend scaffold | Full (mock-only) | Full (mock-only) |
| `GET /api/v1/governance/audit/logs` | ✅ Implemented | ✅ Implemented |
| ZK Pointer validation (backend) | ❌ N/A (no ZK domain) | ✅ In swap flow only; no HTTP route |
| Oversight/Disclosure (backend) | ❌ Missing entirely | ✅ `/api/v2/oversight/*` exists |
| Frontend → real API wiring | ❌ All mocks | ❌ All mocks |

### What is missing

- A supervisor-accessible audit logs HTTP route (`/compliance/audit/logs`) in both scenarios,
  requiring `ROLE_SUPERVISOR` rather than `ROLE_GOVERNANCE`.
- Scenario B: `GET /api/v1/compliance/zk-pointer/verify` HTTP endpoint exposing the ZK compliance
  gate for supervisor queries.
- Scenario A: Oversight handler + routes (`/api/v1/oversight/*`) mirroring Scenario B's v2 endpoints.
- Both scenarios: frontend services wired to real HTTP calls instead of `mockDb`.

---

## User Scenarios & Testing

### User Story 1 — Audit Log Review (Priority: P1)

A supervisor at the Central Bank opens the **Audit Vault** page, sets a date range and severity
filter, and reviews the paginated list of governance and compliance events to verify no unauthorised
actions occurred during a settlement window.

**Why this priority**: Audit access is the baseline supervisory function and the lowest-risk
integration (read-only). It unblocks regulatory demos before the heavier investigation module is
ready.

**Independent Test**: With a live backend, a supervisor can log in, navigate to Audit Vault, apply a
`severity=CRITICAL` filter, and see real log entries returned from `GET /api/v1/compliance/audit/logs`.
The page must NOT show mock data.

**Acceptance Scenarios**:

1. **Given** the supervisor is authenticated with `ROLE_SUPERVISOR`, **When** they open Audit Vault,
   **Then** the page fetches `GET /api/v1/compliance/audit/logs` (not mock) and renders entries with
   actor, action, target, timestamp, and status.
2. **Given** filters for `severity`, `category`, `from_date`, `to_date` are applied, **When** the
   user submits the filter form, **Then** the request includes the corresponding query parameters
   and results are refreshed.
3. **Given** pagination is set (default page 1, limit 50), **When** the user navigates to page 2,
   **Then** `?page=2&limit=50` is appended and the next page of results is displayed.
4. **Given** the backend returns a 403 for a non-supervisor token, **When** the frontend receives it,
   **Then** the page shows "Insufficient privileges" and does not attempt retry.
5. **Given** the backend is unreachable, **When** the fetch fails, **Then** a structured error
   message is shown and the last-known data (if any) remains visible.

---

### User Story 2 — ZK Pointer Compliance Verification (Priority: P2, Scenario B only)

A supervisor enters a bank ID and commitment hash into the **Compliance Verification** panel and
clicks Verify. The portal checks whether the ZK-Pointer is VALID in the Trust Registry and shows the
pointer state, expiry, and the on-chain commitment hash for cross-reference.

**Why this priority**: ZK pointer verification is the on-chain authenticity check that distinguishes
a valid privacy-preserving transaction from a spoofed one. P2 because it requires a new backend
endpoint but no new backend service.

**Independent Test**: With a live Scenario B backend, a supervisor can enter the bank ID and
commitment hash from a known swap and receive `{ "state": "VALID", "expires_at": "..." }` from
`GET /api/v1/compliance/zk-pointer/verify`.

**Acceptance Scenarios**:

1. **Given** the supervisor enters a valid `bank_id` and `commitment_hash`, **When** they click
   Verify, **Then** the portal calls `GET /api/v1/compliance/zk-pointer/verify` and displays state
   (`VALID` / `INVALID` / `EXPIRED`), expiry date, and the resolved commitment hash.
2. **Given** the bank has no matching ZK-Pointer, **When** the API returns 404, **Then** a "No valid
   ZK-Pointer found for this bank and commitment" message is shown.
3. **Given** the ZK-Pointer has expired, **When** the API returns `{ "state": "EXPIRED" }`, **Then**
   the UI shows a destructive badge and the expiry timestamp.
4. **Given** a bank ID without the correct role, **When** the call returns 403, **Then** "Access
   denied" is shown inline without crashing the page.

---

### User Story 3 — Investigation Request (Deanonymisation) (Priority: P2)

A supervisor identifies a shielded transaction that has breached an AML/CFT threshold. They open the
**Investigation Module**, enter the transaction reference and a legal reason code, and submit an
opening request. A second authorised supervisor then co-signs the request. Once quorum is reached the
portal shows the request as `QUORUM_REACHED` and the supervisor can initiate the Paladin view-key
disclosure workflow.

**Why this priority**: The quorum model prevents unilateral deanonymisation, a critical regulatory
safeguard. This is the R2-CR-8 primary use case.

**Independent Test**: With a live backend, two authenticated supervisors can open and co-sign a
disclosure request through the UI and see the final state change to `QUORUM_REACHED`.

**Acceptance Scenarios**:

1. **Given** the supervisor enters `tx_ref`, `requestor_id`, and `reason_code`, **When** they submit
   Open Request, **Then** the portal calls `POST /oversight/disclosure-request` and displays the
   returned `request_id`, `state: PENDING`, and `expires_at`.
2. **Given** a request in `PENDING` state, **When** a second supervisor enters `request_id` and their
   `signer_id` and clicks Sign, **Then** `POST /oversight/disclosure-sign` is called and the
   `quorum_reached` counter increments.
3. **Given** `quorum_reached >= quorum_required`, **When** the status is polled, **Then** the portal
   shows `state: QUORUM_REACHED` and a prominent call-to-action for the Paladin disclosure step
   (informational — the actual Paladin call is out of scope).
4. **Given** the disclosure request has expired (past `expires_at`), **When** the supervisor tries to
   sign, **Then** the portal shows "Request expired — a new request must be opened" and the Sign
   button is disabled.
5. **Given** a signer attempts to sign a request they have already signed, **When** the API returns
   400 with "already signed", **Then** the portal shows "You have already co-signed this request".

---

### User Story 4 — Scenario A: Credential/KYC Status Lookup (Priority: P3, Scenario A only)

In Scenario A there are no ZK pointers. Instead of ZK verification, the Compliance Verification
panel lets the supervisor look up the current KYC/credential status of a participant by subject ID,
including any freeze or revocation flags recorded in the IdentityRegistry.

**Why this priority**: Scenario A supervisors still need compliance visibility; they just use KYC
status rather than ZK pointers.

**Independent Test**: A Scenario A supervisor can enter a subject ID and see the returned KYC status
(`ACTIVE`, `FROZEN`, `REVOKED`, etc.) from `GET /api/v1/compliance/kyc-status/:subject`.

**Acceptance Scenarios**:

1. **Given** a valid subject ID, **When** the supervisor submits the lookup, **Then** the portal
   displays `status`, `last_updated_at`, and any freeze/revocation reason from the API response.
2. **Given** an unknown subject ID, **When** the API returns 404, **Then** "Subject not found in
   Identity Registry" is shown.
3. **Given** the subject is `FROZEN`, **When** the result is displayed, **Then** a destructive badge
   and the freeze reason are shown prominently.

---

### Edge Cases

- What happens when a supervisor token expires mid-session? → Auth store must intercept 401 responses
  and redirect to login without losing filter/form state.
- What happens when the disclosure request quorum is already met before the second sign attempt? →
  The sign form should be disabled and status shown as `QUORUM_REACHED`.
- What if the same user tries to sign the same disclosure request twice? → Backend returns 400
  "already signed"; frontend must surface this clearly.
- What if `GET /api/v1/compliance/audit/logs` returns 0 results? → Show "No audit events match the
  current filter" empty state, not a loading spinner.
- What if filters produce a very large result set? → The API enforces a max `limit` of 100; the
  frontend must not allow requesting more than that.

---

## Requirements

### Functional Requirements

**Shared (both scenarios):**

- **FR-SUP-001**: The portal MUST authenticate using the existing Keycloak OIDC session and require
  `ROLE_SUPERVISOR` for all supervisor-specific endpoints; calls with other roles MUST receive 403.
- **FR-SUP-002**: A `SupervisorHandler` MUST be added to both scenarios' API Gateways. It MUST expose
  `GET /api/v1/compliance/audit/logs`, backed by the same `GetAuditLogs` compliance adapter call used
  by `GovernanceHandler`, but guarded exclusively by `ROLE_SUPERVISOR` middleware. The
  `GovernanceHandler` MUST NOT gain a `ROLE_SUPERVISOR` branch; the two handlers remain independent.
- **FR-SUP-003**: The audit logs response MUST include `id`, `actor`, `action`, `target`,
  `timestamp`, `status` fields. The frontend MUST render them in a sortable, paginated table.
- **FR-SUP-004**: Filter parameters `category`, `severity`, `from_date`, `to_date`, `page`, `limit`
  MUST be supported as query params on the audit logs endpoint and passed through by the frontend.
- **FR-SUP-005**: All frontend API clients MUST replace `mockDb` calls with real HTTP fetch calls
  through a shared `apiClient` that includes the session cookie and a correlation-id header.
- **FR-SUP-006**: The Investigation Module MUST require a `reason_code` from a predefined enum
  (`AML_ALERT`, `CFT_INVESTIGATION`, `COURT_ORDER`, `REGULATORY_EXAM`) before a disclosure request
  can be opened.
- **FR-SUP-007**: The disclosure request status MUST be polled every 10 seconds while a request is
  in `PENDING` state; polling MUST stop when the request reaches `QUORUM_REACHED` or `EXPIRED`.
- **FR-SUP-008**: Sensitive decrypted payloads from the Investigation Module MUST NOT be persisted
  to `localStorage` or `sessionStorage`. They are held only in component state for the session.

**Scenario B only:**

- **FR-SUP-009**: A new HTTP endpoint `GET /api/v1/compliance/zk-pointer/verify` MUST be added to
  the Scenario B API Gateway under `SupervisorHandler`. It MUST accept `bank_id` and
  `commitment_hash` as query parameters and delegate to the existing `ZKComplianceGate.ValidateZKPointer`
  directly (same process, same DB connection — no new gRPC hop). This is acceptable for MVP because
  the coupling already exists; a future refactor may route this through the compliance service gRPC
  interface once the service owns all compliance state exclusively.
- **FR-SUP-010**: The ZK Pointer Verification panel MUST display `state` (VALID / INVALID /
  EXPIRED), `pointer_id`, `expires_at`, and the `commitment_hash` that was validated.

**Scenario A only:**

- **FR-SUP-011**: The Scenario A API Gateway MUST add oversight endpoints under the `/api/v1` namespace
  (consistent with Scenario A's existing routing convention): `POST /api/v1/oversight/disclosure-request`,
  `POST /api/v1/oversight/disclosure-sign`, `GET /api/v1/oversight/disclosure-status/:requestID`.
  These are implemented under a new `OversightHandler`; they MUST NOT share code with Scenario B's
  handler despite identical signatures.
- **FR-SUP-012**: The Scenario A compliance service MUST gain a `DisclosureRequest` domain model
  and `OversightService` equivalent to Scenario B's, stored in Postgres. No cross-scenario code
  sharing; it is an independent reimplementation.

### Key Entities

- **AuditLogEntry**: `id`, `actor`, `action`, `target`, `timestamp`, `status` (SUCCESS | FAILED),
  `severity` (INFO | WARNING | CRITICAL), `category` (GOVERNANCE | COMPLIANCE | SETTLEMENT).
- **ZKPointerVerification** (Scenario B): `bank_id`, `pointer_id`, `commitment_hash`, `state`
  (VALID | INVALID | EXPIRED), `expires_at`.
- **DisclosureRequest**: `request_id`, `tx_ref`, `requestor_id`, `reason_code`, `state`
  (PENDING | QUORUM_REACHED | EXPIRED), `quorum_required`, `quorum_reached`, `opened_at`,
  `expires_at`.
- **DisclosureSignature**: `sig_id`, `request_id`, `signer_bank_id`, `signed_at`.

---

## Success Criteria

### Measurable Outcomes

- **SC-SUP-001**: Supervisor can view real audit log entries (not mocks) within 2 seconds of
  navigating to Audit Vault, in both Scenario A and B environments.
- **SC-SUP-002**: Scenario B supervisor can verify a ZK-Pointer validity via the portal and receive
  a `VALID` or `INVALID` result with no console errors.
- **SC-SUP-003**: Two supervisors in separate browser sessions can open and co-sign a disclosure
  request, transitioning it to `QUORUM_REACHED`, within a single demo walkthrough.
- **SC-SUP-004**: Zero `mockDb` calls remain in the supervisor frontend services after
  implementation; all API clients target real HTTP endpoints.
- **SC-SUP-005**: All new backend endpoints return structured JSON errors with a `message` field for
  4xx/5xx responses.
- **SC-SUP-006**: The `GET /api/v1/compliance/audit/logs` endpoint returns HTTP 403 when called with
  a `ROLE_GOVERNANCE` token (separation of duties).
- **SC-SUP-007**: Backend unit tests cover `OversightService` (Scenario A) and the new ZK-Pointer
  HTTP handler (Scenario B); Go test coverage ≥ 80% on new service/handler files.
- **SC-SUP-008**: Playwright E2E test exists for User Story 1 (Audit Vault) in both scenarios'
  test suites, covering the happy path and the 403 error case.

---

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Scenario Isolation | ✅ Compliant | Scenario A and B implementations are in separate directories. No cross-scenario imports. |
| II. Privacy by Design | ✅ Compliant | Decrypted payloads are in-memory only (FR-SUP-008). ZK-Pointer verification queries the compliance DB, not raw transaction data. |
| III. Atomic Settlement | ✅ N/A | This is a supervisory read + disclosure-request feature; no settlement path is touched. |
| IV. Compliance Gate | ✅ Compliant | All supervisor endpoints enforce `ROLE_SUPERVISOR` via existing auth middleware. No bypass. |
| V. Test-First | ✅ Compliant | Unit tests for backend (SC-SUP-007) and Playwright E2E (SC-SUP-008) are mandatory deliverables. |
| VI. Observability | ✅ Compliant | All disclosure lifecycle actions MUST be appended to the `audit_log` table via the `audit.Append` helper. |

---

## Assumptions

- The Keycloak realm already has a `ROLE_SUPERVISOR` client role; the frontend and backend only need
  to enforce it, not create it.
- `OversightService` in Scenario B is the reference implementation; Scenario A replicates it
  independently, with the same quorum model (2-of-N).
- The Paladin view-key disclosure step (actual decryption of the shielded transaction on the spoke)
  is **out of scope** for this feature. The Investigation Module surfaces `QUORUM_REACHED` as the
  terminal state visible in the portal; the CB's Paladin node handles the actual key release
  off-portal.
- The SSE websocket service already wired in the supervisor scaffold does not need changes; it can
  remain as-is since no real-time event stream is required for audit logs or disclosure requests
  (polling suffices).
- Scenario B's ZK Pointer verification endpoint is read-only and does not modify the compliance DB;
  it only queries `compliance_zk_pointers` via the existing `ZKComplianceGate`.
- The existing `audit.Append` helper in the Scenario B api-gateway can be reused to log supervisor
  actions; Scenario A must implement an equivalent append-only mechanism.
- Scenario A's `reason_code` enum for disclosure requests is the same as Scenario B's:
  `AML_ALERT`, `CFT_INVESTIGATION`, `COURT_ORDER`, `REGULATORY_EXAM`.

---

## Out of Scope

- Paladin view-key decryption on the spoke (the actual key release after quorum is reached).
- HTLC-level transaction deanonymisation in Scenario A (the HTLC secret is already public; this
  feature concerns privacy-token transactions only).
- WebSocket-based real-time audit log streaming (polling is sufficient for the supervisor use case).
- Supervisor ability to write, update, or delete audit log entries (read-only by design).
- Mobile/responsive layout optimisation beyond what Tailwind/shadcn provides by default.
- Multi-tenant supervisor support (each supervisor acts on behalf of their own institution only).
- `LiquidityMonitorPage` and `StabilityControlsPage` in the supervisor scaffold: these are AMM pool
  health monitors, not regulatory oversight tools. They are Scenario B–specific (the
  `LiquidityMonitorPage` title reads "Scenario B · Liquidity Stability Monitor") and are deferred
  to a separate AMM observability ticket.

---

## Design Decisions (Resolved)

| Decision | Choice | Rationale |
|---|---|---|
| Audit logs handler | New `SupervisorHandler` (not `ComplianceHandler` or `GovernanceHandler`) | `ComplianceHandler` is for KYC/AML participant management. `GovernanceHandler` already accumulates too many responsibilities. A dedicated handler makes RBAC explicit and avoids inline role branching inside shared handlers. |
| Scenario A oversight routes | `/api/v1/oversight/*` | Consistent with Scenario A's existing v1 namespace. Avoids misleading version parity with Scenario B's `/api/v2` prefix. |
| ZK Pointer verification implementation | Direct `ZKComplianceGate` call (no new gRPC hop) | The coupling already exists; adding a gRPC round-trip for MVP adds ~80 lines and a network hop with no new safety. Post-MVP refactor to gRPC is noted in FR-SUP-009 for when compliance service becomes the exclusive owner of ZK state. |
