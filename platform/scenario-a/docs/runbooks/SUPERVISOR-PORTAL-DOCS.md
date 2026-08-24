# Supervisor Portal Guide

The Supervisor Portal is the **read-and-intervene** interface for regulatory authorities and internal auditors in the CBWeb3 ecosystem. It provides oversight of cross-border settlement flows, AMM liquidity health, and participant compliance without granting transactional powers (mint, burn, transfer).

---

## Table of Contents

1. [Overview](#1-overview)
2. [Prerequisites](#2-prerequisites)
3. [Access and Login](#3-access-and-login)
4. [Module: Dashboard](#4-module-dashboard)
5. [Module: Liquidity Monitor](#5-module-liquidity-monitor)
6. [Module: Compliance Registry](#6-module-compliance-registry)
7. [Module: Audit Vault](#7-module-audit-vault)
8. [Module: Stability Insights](#8-module-stability-insights)
9. [API Reference](#9-api-reference)
10. [Security Notes](#10-security-notes)
11. [Troubleshooting](#11-troubleshooting)

---

## 1. Overview

The Supervisor Portal is scoped to **read-only ledger access** plus **investigative actions** (decrypt, report). It does not allow minting, burning, or any transfer of tokens.

| Actor | Role | Portal |
|---|---|---|
| Regional Supervisor | Cross-spoke oversight, AML/CFT enforcement | Supervisor Portal |
| Domestic Auditor | Spoke-level audit, forensic review | Supervisor Portal |

### Access Control

Access requires one of the following Keycloak roles:

| Role | Capabilities |
|---|---|
| `REGIONAL_SUPERVISOR` | All modules; cross-spoke visibility |
| `DOMESTIC_AUDITOR` | All modules; single-spoke visibility |

> Supervisors **cannot** mint, burn, or initiate swaps. Write access is strictly limited to audit commentary, report generation, and transaction decryption requests.

---

## 2. Prerequisites

Before accessing the portal:

- The spoke infrastructure must be running (`cd samples && ./deploy-all.sh`; there is no per-spoke bring-up target any more).
- The supervisor has received Keycloak credentials with `REGIONAL_SUPERVISOR` or `DOMESTIC_AUDITOR` role.
- The Supervisor Portal container is up (included in `make frontend-spoke-all`).

| Entity | Default URL |
|---|---|
| Central-Bank-A Supervisor | http://localhost:5178 |
| Central-Bank-B Supervisor | http://localhost:5180 |

---

## 3. Access and Login

Open the Supervisor Portal URL in your browser. You will land on the sign-in screen.

Enter your **username** (email format) and **password**, then click **Sign in**.

On success, you are redirected to the Dashboard.

> **Session note:** The portal stores session credentials in memory only. There is no `localStorage` or `sessionStorage` persistence for sensitive data.

---

## 4. Module: Dashboard

**Route:** `/` (default landing page)

The Dashboard provides a real-time summary of the entire network posture at a glance.

### Summary Cards

| Card | Description |
|---|---|
| **Total tCeBM Supply** | Aggregate token supply across all registered participants |
| **Active Institutions** | Count of participants with `ACTIVE` credential status |
| **Cross-border Agreements** | Active FX agreements between central banks |
| **Healthy Pools** | AMM pools with ratio within the 70/30 threshold |
| **Imbalanced Pools** | AMM pools that have breached the 70/30 threshold |

### Liquidity Health Table

Displays all AMM pairs with their current reserve ratio and health status.

| Column | Description |
|---|---|
| Pair | Token pair identifier (e.g., `BRL-USD`) |
| Ratio | Current A/B reserve ratio |
| Status | `HEALTHY` (green) or `IMBALANCED` (red) |

Imbalance threshold: **70/30** — any pool where one side exceeds 70% of total reserves is flagged.

### Real-time Alerts (SSE)

The last 8 backend telemetry events streamed via Server-Sent Events (SSE) from the compliance service. Each alert shows:

- **Type** — event category (e.g., `AMM_IMBALANCE`, `HTLC_TIMEOUT`)
- **Severity** — `CRITICAL` (red badge) or `WARNING` (yellow badge)
- **Message** — human-readable description

> Alerts refresh automatically without page reload. If no events have been received yet, the panel shows "Waiting for realtime events...".

---

## 5. Module: Liquidity Monitor

**Route:** `/liquidity`

Full detail view of Scenario B (AMM) pool health.

### Pool Status Table

| Column | Description |
|---|---|
| Pair | Token pair (e.g., `BRL-USD`) |
| Reserve A | Current balance of token A in the pool |
| Reserve B | Current balance of token B in the pool |
| Ratio | Displayed as `A/B` percentage split |
| Status | `HEALTHY` or `IMBALANCED` badge |

### Active Alerts Panel

Lists all imbalance alerts currently active, with pair, severity, and description.

- `CRITICAL` — pool ratio has exceeded 70/30 significantly; immediate review recommended.
- `WARNING` — pool ratio is approaching the threshold.

> Data is fetched from `GET /api/v2/amm/pool/{pair}/status` on page load. Use browser refresh to manually re-fetch if needed.

---

## 6. Module: Compliance Registry

**Route:** `/participants`

Read-only view of the compliance registry for all onboarded participants.

### Summary Cards

| Card | Description |
|---|---|
| **Total Participants** | All registered institutions in the registry |
| **Active Credentials** | Institutions whose PKI credential is `ACTIVE` |

### Compliance Registry Table

| Column | Description |
|---|---|
| Institution | Full name of the registered bank |
| EVM Address | On-chain wallet address (`0x…`) |
| Jurisdiction | ISO 3166-1 alpha-2 country code |
| Credential | `ACTIVE` (green) or non-active status (red) |

### Search

Use the search box to filter by institution name, EVM address, or jurisdiction code. Filtering is performed client-side in real time.

> This view calls `GET /api/v1/compliance/registry`. No write actions are available in this module.

---

## 7. Module: Audit Vault

**Route:** `/audit`

Two-panel module for forensic investigation: transaction decryption and immutable audit log review.

### Transaction Decryption Panel

Used to reveal shielded (ZKP-protected) transaction details when legally authorized.

| Field | Description |
|---|---|
| **Transaction hash** | The `0x…` hash of the shielded transaction to investigate |
| **Regulatory view key** | The Paladin/Zeto view key issued to the supervisor; held in memory only |
| **Reason** | Mandatory justification string (minimum 5 characters) for the audit trail |

Click **Decrypt Transaction** to submit. The decrypted payload appears in the result panel on the right showing: tx hash, amount, currency, sender address, and receiver address.

> Decrypted values are **not** written to `localStorage` or `sessionStorage`. They exist only for the duration of the active browser session.

The button is disabled until all three fields meet minimum length requirements.

### Immutable Audit Logs Table

Read-only log of all governance actions taken within the system.

| Column | Description |
|---|---|
| Timestamp | Local datetime of the event |
| Actor | Supervisor or system identifier that performed the action |
| Action | Action type (e.g., `DECRYPT_TX`, `VIEW_REGISTRY`) |
| Target | Address or resource affected (`0x…` or resource ID) |
| Status | `SUCCESS` (green) or `FAILED` (red) |

> Every action taken in the Supervisor Portal — including viewing transactions and decrypting records — is logged here and is non-repudiable (REQ-COM-005 / NFR-RES-002).

---

## 8. Module: Stability Insights

**Route:** `/stability`

Read-only risk posture view for Scenario B (AMM) network. Intended for senior supervisors monitoring systemic risk.

### Summary Cards

| Card | Description |
|---|---|
| **Imbalanced Pools** | Count of pools currently beyond the 70/30 threshold |
| **Active Alerts** | Count of unresolved stability alerts |

### Pool Ratio Risk Bars

Each AMM pair is displayed with a horizontal progress bar showing how far the current ratio has drifted from 50/50. The target is `50` (balanced); escalation starts at `70` (imbalanced).

| Element | Description |
|---|---|
| Pair label | Token pair identifier |
| Ratio label | Current split shown as `A/B` |
| Progress bar | Filled to `ratioA` value; visually indicates drift toward imbalance |

> This module is read-only. Circuit breaker controls (pause/resume) are owned by the Governance Portal (central bank operator role).

---

## 9. API Reference

The Supervisor Portal consumes the following backend endpoints:

| Module | Method | Endpoint | Description |
|---|---|---|---|
| Dashboard | `GET` | `/api/v2/amm/pool/{pair}/status` | Pool reserves and ratio |
| Dashboard | `GET` | `/api/v1/supervisor/network/overview` | Aggregated network KPIs |
| Dashboard | `SSE` | `/api/v1/supervisor/events` | Real-time alert stream |
| Liquidity Monitor | `GET` | `/api/v2/amm/pool/{pair}/status` | Per-pair pool health |
| Compliance Registry | `GET` | `/api/v1/compliance/registry` | Registered participant list |
| Audit Vault | `GET` | `/api/v1/compliance/audit/logs` | Immutable governance log |
| Audit Vault | `POST` | `/api/v1/compliance/audit/decrypt` | Decrypt shielded transaction |

> All endpoints are consumed via the entity's API Gateway. The portal does not call blockchain nodes directly.

---

## 10. Security Notes

| Concern | Mitigation |
|---|---|
| **Access control** | `REGIONAL_SUPERVISOR` / `DOMESTIC_AUDITOR` Keycloak roles enforced on every protected route |
| **No token issuance** | Supervisor Portal has no mint, burn, or transfer capabilities by design |
| **View key handling** | Regulatory view keys are never persisted; in-memory only for the active session |
| **Input sanitization** | All search and hash inputs are sanitized before being sent to the Go-Fiber backend (OWASP A03) |
| **Audit trail** | Every supervisor action is logged in the immutable audit log (OWASP A09) |
| **Least privilege** | Read-only ledger access; write access scoped strictly to decrypt requests and audit comments |

---

## 11. Troubleshooting

| Symptom | Likely Cause | Resolution |
|---|---|---|
| Login fails with 401 | Missing or incorrect `REGIONAL_SUPERVISOR` / `DOMESTIC_AUDITOR` Keycloak role | Ask the governance operator to assign the correct role in Keycloak |
| Dashboard shows all `—` | Network overview service unreachable or no data yet | Confirm `cd samples && ./deploy-all.sh` completed and api-gateway containers are running |
| Pool table is empty | AMM pool not seeded yet | Run `make contracts.seed-hub` or execute step 4b of the scenario-b tryout |
| Decrypt button stays disabled | One or more input fields below minimum length | Ensure tx hash ≥ 8 chars, view key ≥ 8 chars, reason ≥ 5 chars |
| Audit log is empty | No governance actions have been performed yet | Perform any compliance action (e.g., view a participant) to generate the first log entry |
| No real-time alerts | SSE connection dropped or backend events service down | Refresh the page; check `docker logs` for the api-gateway container |
