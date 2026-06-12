# Scenario A — Treasury Portal User Manual

**Audience:** Central bank treasury / issuance operators.
**Data source:** Live backend (real API Gateway).

---

## Table of Contents

1. [Overview](#1-overview)
2. [Who uses it](#2-who-uses-it)
3. [Login and access](#3-login-and-access)
4. [Dashboard](#4-dashboard)
5. [Issuance Approvals](#5-issuance-approvals)
6. [Tokenisation Approvals](#6-tokenisation-approvals)
7. [Redeem Approvals](#7-redeem-approvals)
8. [PvP Settlement Monitor](#8-pvp-settlement-monitor)
9. [Planned / not yet available](#9-planned--not-yet-available)
10. [Status reference](#10-status-reference)
11. [Troubleshooting](#11-troubleshooting)

---

## 1. Overview

The Treasury Portal is the Central Bank's **approvals desk** for Scenario A. When a
commercial bank requests issuance, reserve tokenisation, or redemption from the
Bank Portal, those requests land here for a treasury operator to approve or reject.
The portal also provides a read-only monitor of cross-border PvP settlements.

| Actor | Portal | Role |
|---|---|---|
| Central Bank Treasury Operator | Treasury Portal | Reviews and approves issuance / tokenisation / redemption requests; monitors PvP settlement |

Every approval here is the counterpart to an action a commercial bank took in the
[Bank Portal](./bank.md): the bank requests, the Central Bank approves, and only
then does the on-chain mint / burn happen.

---

## 2. Who uses it

Central bank treasury and issuance operators. Access is provisioned by the network
administrator; sign in with a Central Bank operator account.

---

## 3. Login and access

Open the Treasury Portal URL provided by your administrator and sign in.

![Screenshot: Scenario A — Treasury login](../img/scenario-a/treasury-login.png) <!-- TODO: capture screenshot -->

---

## 4. Dashboard

**Route:** `/` · **Sidebar label:** Dashboard

A summary of outstanding work and reserve position: how many issuance,
tokenisation, and redemption requests await your decision, and recent activity.

![Screenshot: Scenario A — Treasury Dashboard](../img/scenario-a/treasury-dashboard.png) <!-- TODO: capture screenshot -->

---

## 5. Issuance Approvals

**Route:** `/deposits-approval` · **Sidebar label:** Issuance Approvals

Lists commercial-bank requests to **issue tCeBM** against fiat collateral. Each row
is a request submitted from a Bank Portal's *Issuance Requests* page.

![Screenshot: Scenario A — Issuance Approvals](../img/scenario-a/treasury-issuance-approvals.png) <!-- TODO: capture screenshot -->

| Column | Description |
|---|---|
| **Institution** | The requesting commercial bank. |
| **Fiat Amount** | The amount requested for issuance. |
| **Status** | Current request status (see [Status reference](#10-status-reference)). |
| **Created At** | When the bank submitted the request. |

**To act on a request:** open it, review the amount and requesting institution,
then **Approve** (triggers the on-chain mint) or **Reject** (records a reason, no
tokens are minted).

> Approving is an on-chain action. Confirm the institution and amount before
> approving — a confirmation dialog appears first.

---

## 6. Tokenisation Approvals

**Route:** `/escrows-approval` · **Sidebar label:** Tokenisation Approvals

Lists requests to **convert approved reserves into tCeBM tokens** (the pledge /
tokenisation step that follows an approved issuance). Approving executes the
burn-and-mint that puts tCeBM in the bank's wallet.

![Screenshot: Scenario A — Tokenisation Approvals](../img/scenario-a/treasury-tokenisation-approvals.png) <!-- TODO: capture screenshot -->

Columns and the Approve / Reject flow mirror [Issuance Approvals](#5-issuance-approvals).

---

## 7. Redeem Approvals

**Route:** `/redeems-approval` · **Sidebar label:** Redeem Approvals

Lists requests to **redeem tCeBM back to fiat reserves**. Approving authorises the
fiat release after the privacy-preserving token transfer has completed.

![Screenshot: Scenario A — Redeem Approvals](../img/scenario-a/treasury-redeem-approvals.png) <!-- TODO: capture screenshot -->

Columns and the Approve / Reject flow mirror [Issuance Approvals](#5-issuance-approvals).

---

## 8. PvP Settlement Monitor

**Route:** `/htlc-monitor` · **Sidebar label:** PvP Settlement

A **read-only** monitor of cross-border PvP (HTLC) settlements visible to the
Central Bank. Use it to confirm that atomic settlements are progressing and to spot
contracts that are stuck or have entered revocation.

![Screenshot: Scenario A — PvP Settlement Monitor](../img/scenario-a/treasury-htlc-monitor.png) <!-- TODO: capture screenshot -->

| Column | Description |
|---|---|
| **Contract ID** | Unique HTLC contract identifier. |
| **Sender / Receiver** | The participating institutions. |
| **State** | Current contract state (see [Status reference](#10-status-reference)). |
| **Expiry** | The settlement window expiry. |

> This screen is for oversight only. Completing or revoking a settlement is done by
> the participating banks in the Bank Portal, not here.

---

## 9. Planned / not yet available

The following Treasury modules exist in the codebase but are **not exposed** in the
current Scenario A build (hidden from the sidebar). Do not present them as working:

- **Funding Requests**
- **Issuance** (direct issuance screen — issuance is handled via *Issuance Approvals*)
- **Redemption** (direct redemption screen — handled via *Redeem Approvals*)
- **Reconciliation**
- **Audit**
- **Settings**

---

## 10. Status reference

### Issuance / Tokenisation / Redeem request statuses

| Status | Meaning |
|---|---|
| `PENDING` | Awaiting your decision. |
| `APPROVED` | You approved it; the on-chain operation is complete. |
| `REJECTED` | You declined it; a reason is recorded and no tokens move. |
| `MINT_FAILED` | Approval succeeded but the on-chain mint failed — investigate and retry. |

### PvP (HTLC) contract states

| State | Meaning |
|---|---|
| `PENDING_SETTLEMENT` | Locked; waiting for the receiver to complete. |
| `PROCESSING` | Settlement or revocation is executing on-chain. |
| `SETTLED` | Atomic swap completed; funds delivered. |
| `REVOKING` | Settlement window expired; revocation in progress. |
| `REVOKED` | Funds returned to the sender. |

---

## 11. Troubleshooting

### A request is stuck in `PENDING`
- It is waiting for a treasury operator to act — open it and Approve or Reject.

### Approval succeeded but the bank did not receive tokens (`MINT_FAILED`)
- The on-chain mint transaction failed after approval. Check backend / node logs
  and confirm the Central Bank signer has gas and is reachable.

### The approvals list is empty when a bank says they submitted a request
- Reload the page (it does not auto-refresh).
- Confirm you are on the correct Central Bank backend for that bank's spoke.

### A PvP settlement is stuck in the monitor
- This is informational. Check whether the settlement window has expired; if so,
  the counterparty bank must revoke from the Bank Portal. Check relay logs if a
  cross-spoke settlement appears frozen.
