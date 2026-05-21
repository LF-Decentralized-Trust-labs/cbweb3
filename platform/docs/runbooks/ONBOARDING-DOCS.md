# Commercial Bank Onboarding Guide

This guide walks through the complete onboarding process from both sides: the **Commercial Bank Portal** (where the bank submits its registration request) and the **Central Bank Governance Portal** (where the Central Bank reviews and approves the request).

---

## Table of Contents

1. [Overview](#1-overview)
2. [Prerequisites](#2-prerequisites)
3. [Part A — Commercial Bank: Submitting the Onboarding Request](#3-part-a--commercial-bank-submitting-the-onboarding-request)
   - [Step 1: Access the Bank Portal and Sign In](#step-1-access-the-bank-portal-and-sign-in)
   - [Step 2: Navigate to Onboarding](#step-2-navigate-to-onboarding)
   - [Step 3: Operator Confirmation](#step-3-operator-confirmation)
   - [Step 4: Fill in Institution Data](#step-4-fill-in-institution-data)
   - [Step 5: Wait for KYC Approval](#step-5-wait-for-kyc-approval)
4. [Part B — Central Bank: Reviewing and Approving the Request](#4-part-b--central-bank-reviewing-and-approving-the-request)
   - [Step 6: Access the Governance Portal and Sign In](#step-6-access-the-governance-portal-and-sign-in)
   - [Step 7: Navigate to the Registry](#step-7-navigate-to-the-registry)
   - [Step 8: Approve the Pending KYC Request](#step-8-approve-the-pending-kyc-request)
5. [Part C — Commercial Bank: Completing Onboarding](#5-part-c--commercial-bank-completing-onboarding)
   - [Step 9: Onboarding Completion](#step-9-onboarding-completion)
   - [Step 10: Access the Dashboard](#step-10-access-the-dashboard)
6. [Status Reference](#6-status-reference)
7. [Troubleshooting](#7-troubleshooting)

---

## 1. Overview

The onboarding process registers a new commercial bank as a participant in a CBWeb3 spoke. It involves two actors:

| Actor | Portal | Role |
|---|---|---|
| Commercial Bank Operator | Bank Portal | Submits institution data and initiates the onboarding request |
| Central Bank Governance Operator | Governance Portal | Reviews, validates, and approves the KYC request |

The process follows a linear state machine:

```
NONE → PENDING → CREDENTIAL_REQUESTED → KYC_APPROVED → ACTIVE
                                                      ↘ REJECTED / REVOKED / FROZEN
```

Once the Central Bank approves the request, the platform automatically provisions an EVM wallet for the institution, issues the PKI credential, and activates the account on-chain.

---

## 2. Prerequisites

Before starting, ensure the following are in place:

- The spoke infrastructure is running (Central Bank node + Bank node containers are up).
- The commercial bank operator has received their initial **Client ID** and **Client Secret** from the Central Bank (required for the first login to the Bank Portal).
- The Central Bank governance operator has access credentials to the Governance Portal.
- Both portals are accessible via their respective URLs in the spoke deployment.

---

## 3. Part A — Commercial Bank: Submitting the Onboarding Request

### Step 1: Access the Bank Portal and Sign In

Open the **Bank Portal** URL in your browser. The address depends on your spoke deployment (e.g., `http://bank-a.spoke-a.local` or the configured hostname).

You will land on the sign-in screen.

> ![Bank Portal Login Screen](./screenshots/onboarding/01-bank-login.png)

Fill in your credentials:

| Field | Description |
|---|---|
| **Client ID** | The institutional client identifier provided by the Central Bank (UUID format). |
| **Client Secret** | The corresponding secret password for the Client ID. |

Click **Sign in**. On success, you are redirected to the Dashboard.

---

### Step 2: Navigate to Onboarding

From the left-hand navigation menu, click **Onboarding**.

> ![Bank Portal Navigation Menu](./screenshots/onboarding/02-bank-nav-onboarding.png)

The Onboarding Wizard opens. A progress bar at the top shows the four stages:

```
1. Operator  →  2. Institution  →  3. KYC  →  4. Complete
```

> ![Onboarding Wizard — Progress Bar](./screenshots/onboarding/03-onboarding-wizard-progress.png)

---

### Step 3: Operator Confirmation

The first step asks you to confirm your identity before proceeding.

The screen displays the **Operator ID** that is currently signed in. Verify it corresponds to your institution's operator account.

Click **Start Onboarding** to proceed to Step 2.

---

### Step 4: Fill in Institution Data

Complete the institution registration form with your bank's details.

> ![Step 2 — Institution Data Form](./screenshots/onboarding/05-step2-institution-form.png)

| Field | Description | Constraints |
|---|---|---|
| **Institution Name** | Full official name of the commercial bank. | Minimum 2 characters. |
| **Bank Code** | Short unique code identifying the bank in the system. | 1–8 characters; only lowercase letters (`a–z`), digits (`0–9`), or hyphens (`-`). |
| **Country** | ISO 3166-1 alpha-2 country code (e.g., `BR` for Brazil). | Exactly 2 characters. |
| **Role** | The institutional role of this bank. | Select `Commercial Bank` or `Treasury Bank` from the dropdown. |
| **Username** | The login username that will be associated with this institution. | Minimum 3 characters; only lowercase letters, digits, underscores (`_`), or hyphens (`-`). |
| **Email** | Official contact email for this institution. | Must be a valid email address. |

After filling all fields, click **Submit Onboarding Request**.

> If any field fails validation, an inline error message will appear below it. Correct the input and resubmit.

On successful submission, the wizard automatically advances to Step 3.

---

### Step 5: Wait for KYC Approval

After submission, the request enters the **PENDING** state. The wizard transitions to the KYC Approval waiting screen.

> ![Step 3 — KYC Pending](./screenshots/onboarding/07-step3-kyc-pending.png)

The screen displays:

| Element | Description |
|---|---|
| **Current status badge** | Reflects the latest known state (`PENDING`, `CREDENTIAL_REQUESTED`, or `APPROVED`). |
| **Status progress bar** | Visual indicator: `PENDING → APPROVED → ACTIVE`. |
| **Request ID** | Unique identifier for this onboarding request. Keep this for reference. |
| **Wallet** | The EVM wallet address provisioned for the institution (may show `—` until the Central Bank processes the request). |
| **Elapsed** | Time in seconds since the request was submitted. |

The portal automatically polls the backend every **5 seconds**. You can also click **Refresh status** at any time to manually check.

> You must keep this browser tab open (or return to the Onboarding page) while waiting for Central Bank approval. The process continues to the next step once the Central Bank acts on the request.

---

## 4. Part B — Central Bank: Reviewing and Approving the Request

> This section is performed by a **Central Bank Governance operator** using the Governance Portal.

### Step 6: Access the Governance Portal and Sign In

Open the **Governance Portal** URL in your browser (e.g., `http://governance.spoke-a.local` or the configured hostname).

> ![Governance Portal Login Screen](./screenshots/onboarding/08-governance-login.png)

Enter the governance operator's **username** (email format) and **password**, then click **Sign in**. On success, you land on the Governance Dashboard.

---

### Step 7: Navigate to the Registry

From the left-hand navigation menu, click **Registry**.

> ![Governance Portal Navigation — Registry](./screenshots/onboarding/09-governance-nav-registry.png)

The Registry page loads with two sections:

1. **Compliance Registry** — a searchable table of all registered participants, their statuses, credentials, and expiry dates.
2. **Pending KYC Approvals** — a live-updated table of commercial banks awaiting Central Bank validation.

> ![Registry Page — Overview](./screenshots/onboarding/10-registry-overview.png)

The **Pending KYC Approvals** table refreshes automatically every **15 seconds**. You can also click **Refresh** manually.

---

### Step 8: Approve the Pending KYC Request

In the **Pending KYC Approvals** table, locate the commercial bank that submitted the onboarding request. Each row shows:

| Column | Description |
|---|---|
| **Subject** | Unique user identifier of the bank's operator account. |
| **Bank Code** | The bank code submitted by the bank in Step 4. |
| **Institution** | The institution name submitted in Step 4. |
| **Wallet** | The EVM wallet address provisioned for this institution. |
| **Reason** | Text input field — you must provide a reason before approving. |
| **Action** | The **Approve KYC** button. |

> ![Pending KYC Approvals Table](./screenshots/onboarding/11-pending-kyc-table.png)

To approve:

1. In the **Reason** field of the target bank's row, type the approval justification. The reason must be **at least 10 characters** long.
2. Click **Approve KYC**.

> ![KYC Approved — Success Toast](./screenshots/onboarding/13-kyc-approved-toast.png)

On successful approval, the entry disappears from the pending list and the participant's status in the Compliance Registry transitions from `PENDING` to `ACTIVE`.

---

## 5. Part C — Commercial Bank: Completing Onboarding

> Switch back to the **Bank Portal** browser tab.

### Step 9: Onboarding Completion

Once the Central Bank has approved the KYC request, the bank's portal automatically detects the status change (via polling) and advances to **Step 4**.

The wizard initiates the final completion sequence: it exchanges the approval for a PKI credential and activates the institution account on-chain.

> ![Step 4 — Finalizing Onboarding](./screenshots/onboarding/14-step4-finalizing.png)

Once finalization succeeds, the screen displays:

| Element | Description |
|---|---|
| **Wallet Address** | The EVM wallet permanently associated with this institution. |
| **Transaction Hash** | The on-chain transaction hash of the activation transaction. |
| **PKI login badge** | Indicates whether the PKI credential login validation succeeded (`PKI login validated`) or is still pending (`PKI login pending`). |

> ![Step 4 — Completion Details](./screenshots/onboarding/15-step4-completion-details.png)

---

## 6. Status Reference

The onboarding request progresses through the following states:

| Status | Meaning | Where Visible |
|---|---|---|
| `NONE` | No onboarding request exists for this account. | Bank Portal — wizard starts at Step 1. |
| `PENDING` | The onboarding request was submitted and is awaiting Central Bank review. | Bank Portal Step 3; Governance Portal Registry (Pending KYC table). |
| `CREDENTIAL_REQUESTED` | The system is internally requesting the PKI credential. | Bank Portal Step 3 (shown as "PENDING" to the user). |
| `KYC_APPROVED` | The Central Bank has approved the KYC; final activation is in progress. | Bank Portal Step 4 (finalizing). |
| `ACTIVE` | The institution is fully onboarded and can operate. | Bank Portal Dashboard; Governance Registry table. |
| `REJECTED` | The Central Bank rejected the onboarding request. | Bank Portal — wizard shows rejection notice. |
| `REVOKED` | The institution's access was revoked after activation. | Bank Portal — wizard shows revocation notice. |
| `FROZEN` | The institution's account was frozen by Central Bank governance. | Bank Portal — wizard shows frozen notice; Governance Accounts page. |

> If the request reaches `REJECTED`, `REVOKED`, or `FROZEN`, the Bank Portal displays a descriptive message. The operator should contact Central Bank governance support for further instructions.

---

## 7. Troubleshooting

### Bank Portal login fails with "unauthorized"

- Verify the **Client ID** and **Client Secret** are correct and have not expired.
- Confirm the spoke backend services are running (`make spoke-up` or check `docker compose ps`).

### Onboarding form returns an error after submission

- Check that all fields pass their validation constraints (see table in Step 4).
- Verify backend connectivity: the Bank Portal must be able to reach the API Gateway.

### Pending KYC table is empty in the Governance Portal

- Click **Refresh** to force a manual poll.
- Confirm the commercial bank completed Step 4 and the wizard shows "Step 3: KYC Approval" with a valid Request ID.

### The bank is `ACTIVE` in the Governance Portal but the Bank Portal wizard still shows Step 3

- Click **Refresh status** on the Step 3 screen to force an immediate status check.
- Confirm the browser tab has not been idle long enough for the session cookie to expire; if so, log out, log back in, and navigate to Onboarding.
