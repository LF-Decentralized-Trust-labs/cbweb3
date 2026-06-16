# Supervisor Portal — User Manual (Scenario A)

**Audience:** Supervisors and regulators.

---

> ## Preview status (R2-CR-8)
>
> The Supervisor Portal is currently a **UI preview**. Most data-fetching screens
> call real backend endpoints, but **the data those endpoints return in the current
> development environment is demonstration data, not a live production data set**.
> The authentication flow uses institutional SSO/OIDC credentials, but **no
> production identity-provider is wired up yet** — the login accepts any credentials
> that satisfy the minimum-length validation while the backend is running locally.
>
> - **Do not** use any figure, balance, alert, HTLC record, participant entry, or
>   audit log shown in this portal for a real supervisory or regulatory decision.
> - **Do not** treat the login as a production security boundary.
> - Live data wiring and production Keycloak authentication are tracked under
>   finding **R2-CR-8** and are **not yet available**.
>
> Every screen that displays data currently sourced from the development/demo
> environment is labelled **Mock data** in a callout below its description.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Who uses it](#2-who-uses-it)
3. [Access and login](#3-access-and-login)
4. [Navigation](#4-navigation)
5. [Screens](#5-screens)
   - 5.1 [Dashboard (`/`)](#51-dashboard-)
   - 5.2 [Audit Vault (`/audit`)](#52-audit-vault-audit)
   - 5.3 [Investigations (`/investigation`)](#53-investigations-investigation)
   - 5.4 [Participant Management (`/participants`)](#54-participant-management-participants)
   - 5.5 [Stability Controls (`/stability`)](#55-stability-controls-stability)
   - 5.6 [Settings (`/settings`)](#56-settings-settings)
6. [Typical workflows](#6-typical-workflows)
7. [Status reference](#7-status-reference)
8. [Security notes](#8-security-notes)
9. [Troubleshooting](#9-troubleshooting)

---

## 1. Overview

The Supervisor Portal is the **read-and-intervene** oversight surface for
regulatory authorities and internal auditors in Scenario A (Enhanced
Correspondent Banking). It provides visibility into HTLC settlement flows,
participant compliance status, and audit records without granting any
transactional powers (no minting, burning, or token transfers).

The portal exposes six screens:

| Screen | Route | Purpose |
|---|---|---|
| Dashboard | `/` | Network KPIs and live HTLC settlement table |
| Audit Vault | `/audit` | Credential verification, transaction decryption, audit log review |
| Investigations | `/investigation` | Open, co-sign, and look up disclosure requests |
| Participant Management | `/participants` | Compliance registry — read-only participant list |
| Stability Controls | `/stability` | HTLC expiry risk and active alert counts |
| Settings | `/settings` | Session-scoped operational preferences |

---

## 2. Who uses it

| Role | Keycloak claim | Scope |
|---|---|---|
| Regional Supervisor | `ROLE_SUPERVISOR` | Cross-spoke visibility; all modules |
| Compliance Officer / NOC | `ROLE_NOC` | All modules; operational compliance |
| Central Bank Admin | (other) | All modules; administrative oversight |

Supervisors cannot mint tokens, burn tokens, or initiate settlements. The only
write actions available are:

- Opening or co-signing a disclosure request (Investigations screen).
- Submitting a transaction decryption request (Audit Vault screen).
- Saving session-level preferences (Settings screen).

---

## 3. Access and login

<!-- TODO: screenshot -->

Open the Supervisor Portal URL in your browser. Two instances run by default:

| Entity | Default URL |
|---|---|
| Spoke A supervisor | http://localhost:5182 |
| Spoke B supervisor | http://localhost:5183 |

You will land on the sign-in screen.

Enter your **Client ID** (minimum 3 characters) and **Client Secret** (minimum
6 characters), then click **Sign in**. On success you are redirected to the
Dashboard.

> **Mock data — demo login (R2-CR-8):** The login form calls `/api/v1/auth/login`
> with the supplied credentials. In the current development environment the
> backend accepts any locally-configured credentials. Production Keycloak OIDC
> enforcement is not yet active. Do not treat this login as a security boundary.

Session credentials are held in memory only. There is no `localStorage` or
`sessionStorage` persistence for sensitive data. Closing or refreshing the
browser tab clears the session.

---

## 4. Navigation

After login, a persistent sidebar is shown on the left. It contains links to
all six screens. The current screen is highlighted. All routes are protected —
unauthenticated users are redirected to `/login`. Unknown routes redirect to
the Dashboard (`/`).

---

## 5. Screens

### 5.1 Dashboard (`/`)

<!-- TODO: screenshot -->

The landing page after login. It provides a network-level summary and a full
HTLC settlement table.

#### Summary cards

Three metric cards are shown at the top of the page.

| Card | Description |
|---|---|
| Active Institutions | Count of participants whose status is `ACTIVE` in the compliance registry |
| Active HTLCs | Count of HTLC contracts currently in the `HTLC_STATE_LOCKED` state |
| Pending Settlements | Count of contracts in `HTLC_STATE_LOCKED` or `HTLC_STATE_SETTLING` |

#### HTLC Settlement Status table

Displays all active and recent HTLC contracts retrieved from the backend.

| Column | Description |
|---|---|
| Contract ID | Unique identifier for the HTLC contract (truncated; click to copy the full value) |
| Hash Lock | The cryptographic hash commitment securing the HTLC (truncated; click to copy) |
| Zeto Ref | The Paladin/Zeto privacy-token lock reference (truncated; click to copy) |
| Expires | Timestamp at which the time-lock expires and a refund becomes claimable |
| State | Current lifecycle state of the contract (see the status reference table below) |

The table is refreshed automatically on page load. If no HTLCs exist, the table
shows "No active HTLCs".

> **Mock data:** In the current development environment, data is sourced from the
> local backend running with seeded demonstration records. The figures shown are
> not from a production network.

---

### 5.2 Audit Vault (`/audit`)

<!-- TODO: screenshot -->

The Audit Vault is a three-panel forensic screen:

1. **Credential Verification** — look up a participant's KYC status by subject
   identifier.
2. **Compliance and Audit Vault** — decrypt a shielded transaction using a
   regulatory view key.
3. **Immutable Audit Logs** — paginated, filterable log of all governance
   actions recorded by the backend.

> **Mock data:** In the development environment, audit log entries and
> decryption results come from the local backend and are demonstration records.
> They must not be used as evidence or cited in regulatory proceedings.

#### Credential Verification panel

Use this panel to look up the KYC/credential lifecycle status of any
participant by their subject identifier (a UUID or DID).

1. Enter the subject's ID in the **Subject ID** field.
2. Click **Verify** (or press Enter).
3. The result panel shows the subject identifier, status badge, last-updated
   timestamp, and — if the credential is frozen — the freeze reason.

| Status | Meaning |
|---|---|
| `ACTIVE` / `APPROVED` | Credential is current and the participant is cleared to transact |
| `PENDING` | Credential is under review |
| `FROZEN` | Credential has been temporarily suspended; freeze reason is shown |
| `REVOKED` / `REJECTED` | Credential has been permanently removed or denied |

#### Transaction Decryption panel

Cross-border value transfers in Scenario A use Paladin/Zeto ZKP-shielded
tokens. When legally authorised, a supervisor can request decryption of a
specific transaction.

| Field | Requirement | Description |
|---|---|---|
| Transaction hash | Minimum 8 characters | The `0x…` hash of the shielded transaction to investigate |
| Regulatory view key | Minimum 8 characters | The Paladin/Zeto view key issued to the supervisor; held in memory only, never persisted |
| Reason | Minimum 5 characters | Mandatory justification for the audit trail |

The **Decrypt Transaction** button is disabled until all three fields meet
their minimum length requirements. Click it to submit. On success, the
decryption result panel (to the right) shows:

- Transaction hash
- Amount and currency
- Sender address
- Receiver address

Decrypted values exist only in the active browser session and are never written
to `localStorage` or `sessionStorage`.

Submitting a decryption request automatically refreshes the audit log, adding a
new entry that records the action.

#### Immutable Audit Logs table

A paginated, read-only log (20 entries per page) of all governance actions
recorded by the compliance service.

| Column | Description |
|---|---|
| Timestamp | Local date and time of the event |
| Actor | Subject ID or system identifier that performed the action (truncated; click to copy) |
| Action | Action type (e.g., `DECRYPT_TX`, `VIEW_REGISTRY`, `KYC_LOOKUP`) |
| Category | Functional category of the action |
| Severity | `INFO`, `LOW`, `MEDIUM`, `HIGH`, or `CRITICAL` |
| Outcome | `SUCCESS` (green) or non-success (red) |

**Filtering:** Enter a severity value (e.g., `CRITICAL`) or a category string
in the filter fields above the table, then click **Filter**. Click **Clear** to
remove all filters and reload the unfiltered log.

Every supervisor action — including viewing records and submitting decryption
requests — is recorded here and is non-repudiable.

---

### 5.3 Investigations (`/investigation`)

<!-- TODO: screenshot -->

The Investigations screen manages the **disclosure request workflow** — the
multi-party sign-off process required to legally compel disclosure of shielded
transaction details under AML/CFT or court-order authority.

The screen contains three panels.

> **Mock data:** In the development environment, disclosure requests are
> persisted to the local backend database and are demonstration records only.

#### Open Disclosure Request panel

Initiates a 2-of-N disclosure workflow. The request expires automatically after
72 hours if quorum is not reached.

| Field | Description |
|---|---|
| Transaction Reference | The `0x…` reference of the transaction under investigation |
| Requestor Bank ID | The identifier of the bank initiating the request (e.g., `cb_lnet`) |
| Reason Code | Select one of the four statutory reason codes (see table below) |

**Reason codes:**

| Code | Use case |
|---|---|
| `AML_ALERT` | Anti-money laundering alert |
| `CFT_INVESTIGATION` | Counter-financing of terrorism investigation |
| `COURT_ORDER` | Legally binding court order |
| `REGULATORY_EXAM` | Scheduled regulatory examination |

Click **Open Request**. On success, a summary card appears showing the new
request ID, status, quorum progress, and expiry time. Record the request ID —
it is required for subsequent co-signing and status lookups.

#### Co-sign Disclosure Request panel

Add your institution's co-signature to an existing `PENDING` disclosure
request.

| Field | Description |
|---|---|
| Request ID | The UUID of the disclosure request to co-sign |
| Signer Bank ID | Your institution's bank identifier (e.g., `cb_spoke_a`) |

Click **Sign Request**. A success toast confirms the signature was recorded.
Once quorum is reached the request transitions to `QUORUM_REACHED`.

#### Disclosure Status Lookup panel

Look up the current state and quorum progress of any disclosure request.

1. Enter the **Request ID** (UUID).
2. Click **Look up**.
3. A summary card is displayed showing: request ID, state badge, target
   transaction reference, reason code, quorum progress
   (`quorumReached / quorumRequired`), and expiry datetime.

| State | Meaning |
|---|---|
| `PENDING` | Open; awaiting additional signatures |
| `QUORUM_REACHED` | Sufficient co-signatures received; disclosure may proceed |
| `EXPIRED` | The 72-hour window closed before quorum was reached |

---

### 5.4 Participant Management (`/participants`)

<!-- TODO: screenshot -->

A read-only view of the compliance registry for all onboarded participants.

> **Mock data:** In the development environment, the participant list is sourced
> from the local compliance service and contains demonstration records, not the
> live registry.

#### Summary cards

| Card | Description |
|---|---|
| Total Participants | All institutions registered in the compliance registry |
| Active Credentials | Institutions whose credential status is `ACTIVE` |

#### Compliance Registry table

| Column | Description |
|---|---|
| Institution | Full legal name of the registered institution |
| EVM Address | On-chain wallet address (`0x…`) |
| Jurisdiction | ISO 3166-1 alpha-2 country code |
| Credential | `ACTIVE` (green) or other status (red) |

**Search:** Type in the search box above the table to filter by institution
name, EVM address, or jurisdiction code. Filtering is performed client-side in
real time.

No write actions are available in this screen. Participant suspension or
reinstatement is a governance function handled outside this portal.

---

### 5.5 Stability Controls (`/stability`)

<!-- TODO: screenshot -->

The Stability Controls screen provides HTLC expiry risk monitoring and a count
of active stability alerts. It is intended for senior supervisors watching for
systemic settlement risk.

> **Mock data:** In the development environment, HTLC and alert data is sourced
> from the local backend with seeded demonstration records.

#### Summary cards

| Card | Description |
|---|---|
| Active HTLCs | Count of contracts currently in `HTLC_STATE_LOCKED` |
| Active Alerts | Count of unresolved stability alerts |

#### HTLC Expiry Risk list

Each active HTLC is shown as a card with:

- **Contract ID** (truncated to 14 characters)
- **Hash Lock** (truncated)
- **Zeto Ref** (truncated)
- **State badge** — colour-coded by lifecycle state (see the status reference
  table)
- **Expires** — local datetime at which the time-lock closes

Contracts approaching their expiry time require attention before the lock
window closes. Once a time-lock expires, the HTLC transitions to a refundable
state and the corresponding cross-border payment must be re-initiated.

This screen is read-only. Circuit-breaker pause and resume actions (if
applicable) are owned by the Governance Portal and are not accessible here.

---

### 5.6 Settings (`/settings`)

<!-- TODO: screenshot -->

Session-scoped operational preferences. Changes are saved for the current
browser session only and are lost on logout or tab close.

| Setting | Default | Description |
|---|---|---|
| Realtime Telemetry (SSE) | On | Receive imbalance and governance alerts instantly via Server-Sent Events |
| Strict Session Management | On | Require periodic re-authentication for privileged operations |
| Mask Sensitive Data | On | Prevent accidental exposure of decrypted payloads in UI surfaces |

Click **Save Preferences** to apply. A confirmation toast is shown. The
**Session-only preferences** label is a reminder that these settings are not
persisted to the backend.

---

## 6. Typical workflows

### Investigate a flagged transaction

1. Open the **Audit Vault** (`/audit`).
2. In the **Credential Verification** panel, verify the credential status of
   the parties involved using their subject IDs.
3. In the **Compliance and Audit Vault** panel, enter the transaction hash, your
   regulatory view key, and a documented reason. Click **Decrypt Transaction**.
4. Review the decrypted amount, currency, sender, and receiver in the result
   panel.
5. The decryption action is automatically recorded in the **Immutable Audit
   Logs** table.

### Open a multi-party AML disclosure request

1. Navigate to **Investigations** (`/investigation`).
2. In the **Open Disclosure Request** panel, enter the transaction reference,
   your requestor bank ID, and select the appropriate reason code.
3. Click **Open Request** and note the returned **Request ID**.
4. Share the Request ID with the other required co-signers.
5. Each co-signer goes to the **Co-sign Disclosure Request** panel, enters the
   Request ID and their signer bank ID, and clicks **Sign Request**.
6. Once all required signatures are collected, use the **Disclosure Status
   Lookup** panel to confirm the state has changed to `QUORUM_REACHED`.

### Monitor settlement health

1. Open the **Dashboard** (`/`) to see a summary of active HTLCs and pending
   settlements.
2. Open **Stability Controls** (`/stability`) for a detailed list of HTLC
   contracts with their expiry times.
3. Focus attention on contracts whose expiry timestamp is approaching. Coordinate
   with the relevant bank operators to ensure secrets are revealed and
   settlements are completed before the time-lock closes.

### Review the compliance registry

1. Navigate to **Participant Management** (`/participants`).
2. Note the **Total Participants** and **Active Credentials** counts.
3. Use the search field to locate a specific institution by name, wallet
   address, or jurisdiction code.
4. Review the credential status badge. A red badge indicates a non-active
   credential; escalate through your governance channel to investigate.

---

## 7. Status reference

### HTLC lifecycle states

| State | Badge colour | Meaning |
|---|---|---|
| `HTLC_STATE_PENDING` | Grey | Contract created; not yet locked |
| `HTLC_STATE_LOCKED` | Grey | Funds locked; awaiting secret reveal |
| `HTLC_STATE_SETTLING` | Yellow | Secret revealed; settlement in progress |
| `HTLC_STATE_SETTLED` | Green | Settlement complete; funds released |
| `HTLC_STATE_REFUNDING` | Yellow | Timeout or cancellation; refund in progress |
| `HTLC_STATE_REFUNDED` | Red | Funds returned to sender |
| `HTLC_STATE_INVALID` | Red | Contract is in an unrecoverable error state |

### Disclosure request states

| State | Meaning |
|---|---|
| `PENDING` | Open; waiting for co-signatures to reach quorum |
| `QUORUM_REACHED` | Sufficient co-signatures received; disclosure authorised |
| `EXPIRED` | 72-hour window elapsed before quorum; request is closed |

### Credential / KYC states

| Status | Meaning |
|---|---|
| `ACTIVE` | Credential is current; participant is cleared to transact |
| `APPROVED` | Credential has been approved (equivalent to `ACTIVE`) |
| `PENDING` | Credential is under review |
| `FROZEN` | Credential temporarily suspended; freeze reason provided |
| `REVOKED` | Credential permanently removed |
| `REJECTED` | Credential application was denied |

### Audit log severity levels

| Level | Meaning |
|---|---|
| `INFO` | Routine informational event |
| `LOW` | Minor event; no immediate action required |
| `MEDIUM` | Event warranting attention |
| `HIGH` | Significant event; prompt review recommended |
| `CRITICAL` | Urgent event; immediate attention required |

---

## 8. Security notes

| Concern | Current behaviour |
|---|---|
| Authentication | Institutional SSO/OIDC credentials submitted to `/api/v1/auth/login`. Production Keycloak enforcement is not yet active (R2-CR-8). |
| No token issuance | The Supervisor Portal has no mint, burn, or transfer capabilities by design. |
| View key handling | Regulatory view keys are never written to `localStorage` or `sessionStorage`; they exist in memory only for the active browser session. |
| Audit trail | Every action — including decryption requests and registry views — is logged in the immutable audit log. |
| Session persistence | Closing or refreshing the tab clears the in-memory session; the user must sign in again. |
| Role-based access | Three roles are supported: `SUPERVISOR_ROLE`, `COMPLIANCE_OFFICER`, and `CENTRAL_BANK_ADMIN`. All currently share the same set of portal permissions. |

---

## 9. Troubleshooting

| Symptom | Likely cause | Resolution |
|---|---|---|
| Login fails immediately | Client ID or Client Secret too short (minimum 3 / 6 characters) | Check the minimum-length error message shown below the field |
| Login fails with an authentication error from the server | Backend auth service is not running, or credentials are not recognised in the local configuration | Confirm the backend stack is up (`make scenario-a.up` or equivalent) and verify credentials |
| Dashboard cards show `—` | The compliance or HTLC backend service is unreachable | Check that all backend containers are running and the API base URL is correctly configured |
| HTLC table is empty | No HTLC records seeded or created yet | Run the scenario-a demo scripts to create sample HTLC locks |
| Decrypt button stays disabled | One or more fields are below the minimum length | Ensure tx hash ≥ 8 characters, view key ≥ 8 characters, reason ≥ 5 characters |
| Audit log table shows "No audit logs found" | No governance actions have been performed yet, or the backend audit service is unreachable | Perform any action (e.g., a KYC lookup) to generate the first log entry; verify backend connectivity |
| Disclosure request fails to open | Missing transaction reference or requestor ID | Ensure both required fields are filled before clicking Open Request |
| Status lookup returns "not found" | The request ID is incorrect, or the request has expired | Verify the request ID and check whether the 72-hour expiry has elapsed |
| Participant list is empty | The compliance participants endpoint is unreachable or returned no data | Verify backend connectivity; check if participants have been registered |
| Data looks static or unrealistic | Expected — the portal runs on demonstration data in the current development environment | No action required; see preview status notice at the top of this manual |

---

> When the portal is wired to live production data and production Keycloak
> authentication, this manual will be updated to remove the mock-data warnings
> and document the verified behaviour of each screen.
