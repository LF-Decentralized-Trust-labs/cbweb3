# Scenario B — Bank Portal User Manual

**Audience:** Commercial bank operators participating in the International Hub.
**Data source:** Live backend (real API Gateway).

---

## Table of Contents

1. [Overview](#1-overview)
2. [Who uses it](#2-who-uses-it)
3. [Login and access](#3-login-and-access)
4. [Dashboard](#4-dashboard)
5. [Onboarding](#5-onboarding)
6. [Issuance Requests](#6-issuance-requests)
7. [Reserve Tokenisation](#7-reserve-tokenisation)
8. [Redeems](#8-redeems)
9. [Approve AMM](#9-approve-amm)
10. [Transfer](#10-transfer)
11. [Bridge](#11-bridge)
12. [Compliance](#12-compliance)
13. [Settings](#13-settings)
14. [Status reference](#14-status-reference)
15. [Troubleshooting](#15-troubleshooting)

---

## 1. Overview

The Bank Portal is the operational interface for a commercial bank operator in
**Scenario B — International Hub**. Where Scenario A settles bilaterally between two
spokes, Scenario B routes cross-border value through a shared **Hub** with an
**Automated Market Maker (AMM)** for FX and a **bridge** that locks native CBDC on a
spoke and mints a mirrored position on the Hub.

The day-to-day lifecycle is: obtain tCeBM (issuance / tokenisation), approve the AMM
to spend it, then **Transfer** value cross-border (quote → execute against the AMM)
or use the **Bridge** to move positions between a spoke and the Hub.

| Actor | Portal | Role |
|---|---|---|
| Commercial Bank Operator | Bank Portal | Manages balances, issuance/redemption, AMM transfers, and bridge operations |

---

## 2. Who uses it

Commercial bank operators at a participating institution. Access requires an
`ACTIVE` institution and credentials issued by the Central Bank.

---

## 3. Login and access

Open the portal URL provided by your administrator (see the **Portal Ports** table
in [`scenario-b/frontend/README.md`](../../../scenario-b/frontend/README.md)), enter
your **Client ID** and **Client Secret**, and sign in.

![Screenshot: Scenario B — Bank Portal login](../img/scenario-b/bank-login.png) <!-- TODO: capture screenshot -->

> If sign-in fails immediately with correct credentials, confirm your institution's
> status is `ACTIVE` — new institutions must finish onboarding first.

---

## 4. Dashboard

**Route:** `/` · **Sidebar label:** Dashboard

A real-time overview of your institution: tCeBM balance, pending request counts, and
recent transfer / bridge activity.

![Screenshot: Scenario B — Bank Dashboard](../img/scenario-b/bank-dashboard.png) <!-- TODO: capture screenshot -->

---

## 5. Onboarding

**Route:** `/onboarding` · **Sidebar label:** Onboarding

Registers a new commercial bank with the Central Bank and provisions its wallet.
Until onboarding completes and the institution is `ACTIVE`, the operational screens
are unavailable. See the shared
[ONBOARDING-DOCS](../../../scenario-a/docs/runbooks/ONBOARDING-DOCS.md) for the
step-by-step flow.

![Screenshot: Scenario B — Onboarding](../img/scenario-b/bank-onboarding.png) <!-- TODO: capture screenshot -->

---

## 6. Issuance Requests

**Route:** `/deposits` · **Sidebar label:** Issuance Requests

Request the Central Bank to **issue tCeBM** against your fiat collateral. Enter the
amount, **Review**, then **Confirm**. The request appears in the table as `PENDING`
until the Central Bank approves it from the Governance Portal.

![Screenshot: Scenario B — Issuance Requests](../img/scenario-b/bank-deposits.png) <!-- TODO: capture screenshot -->

| Column | Description |
|---|---|
| **ID** | Unique request identifier. |
| **Fiat Amount** | Amount requested. |
| **Status** | Request status (see [Status reference](#14-status-reference)). |
| **Created At** | Submission time. |

> Tokens are minted only after approval. The page does not auto-refresh.

---

## 7. Reserve Tokenisation

**Route:** `/escrows` · **Sidebar label:** Reserve Tokenisation

After an issuance request is approved, convert the approved reserve into tCeBM
tokens. Enter the tCeBM amount, **Review**, **Confirm**. On approval the system
burns the old position and mints tCeBM to your wallet.

![Screenshot: Scenario B — Reserve Tokenisation](../img/scenario-b/bank-escrows.png) <!-- TODO: capture screenshot -->

---

## 8. Redeems

**Route:** `/redeems` · **Sidebar label:** Redeems

Convert tCeBM back to fiat reserves. The system performs a privacy-preserving token
transfer and submits the request to the Central Bank for fiat-release approval.

![Screenshot: Scenario B — Redeems](../img/scenario-b/bank-redeems.png) <!-- TODO: capture screenshot -->

---

## 9. Approve AMM

**Route:** `/approve-amm` · **Sidebar label:** Approve AMM

Before the AMM can use your tCeBM for a transfer/swap, you must **approve a spending
allowance** for it. This is a one-time (or top-up) authorisation per amount.

![Screenshot: Scenario B — Approve AMM](../img/scenario-b/bank-approve-amm.png) <!-- TODO: capture screenshot -->

1. Enter the **amount** of tCeBM to approve for AMM usage.
2. Click **Prepare**, then confirm in the **Confirm AMM Approval** dialog.
3. The dialog states *"You are about to approve N tCeBM for AMM usage."* — verify and
   **Approve**.

> If a transfer or swap fails with an approval/allowance error, return here and
> approve a sufficient amount first.

---

## 10. Transfer

**Route:** `/transfer` · **Sidebar label:** Transfer

The core cross-border action in Scenario B: send value to a counterparty by swapping
through the Hub AMM. It is a two-step, quote-then-execute flow.

![Screenshot: Scenario B — Transfer](../img/scenario-b/bank-transfer.png) <!-- TODO: capture screenshot -->

**Step 1 — Get Quote.** Enter the transfer amount and request a quote. The quote
shows the required input and price impact. Quotes can expire — if so, request a new
one before executing.

**Step 2 — Execute Transfer.** Provide a **maximum input amount** (your slippage
ceiling) and the **beneficiary**, then **Confirm**. A *Suggested Max* helper fills a
recommended ceiling from the quote. Progress is shown through to **Transfer
Completed**; **Reset** clears the form for the next one.

> The **Execute** button is disabled while the pool is inactive or a quote is
> expired. If you see *"Quote expired — click Get Quote to refresh"*, re-quote.

---

## 11. Bridge

**Route:** `/bridge` · **Sidebar label:** Bridge

Moves value between a spoke and the Hub using a lock/mint and burn/unlock model.

![Screenshot: Scenario B — Bridge](../img/scenario-b/bank-bridge.png) <!-- TODO: capture screenshot -->

| Action | What it does |
|---|---|
| **Lock & Mint** | Locks native CBDC on the spoke and mints a mirrored position on the Hub. |
| **Burn & Unlock** | Burns the mirrored Hub position and unlocks the native CBDC back on the spoke. |
| **Bridge Positions** | Tracks the lifecycle of each bridge operation and relayer progress; filter by state and **Refresh** to update. |

> Bridge operations depend on the relay carrying events between the spoke and the
> Hub. If a position is stuck, check its state in **Bridge Positions** and the relay
> status (NOC Portal) before assuming a contract problem.

---

## 12. Compliance

**Route:** `/compliance` · **Sidebar label:** Compliance

Your institution's compliance status and credential information — used to confirm
you are cleared to transact and to surface any compliance blocks.

![Screenshot: Scenario B — Compliance Center](../img/scenario-b/bank-compliance.png) <!-- TODO: capture screenshot -->

---

## 13. Settings

**Route:** `/settings` · **Sidebar label:** Settings

Portal preferences for the current session.

---

## 14. Status reference

### Issuance / Tokenisation / Redeem request statuses

| Status | Meaning |
|---|---|
| `PENDING` | Submitted; awaiting Central Bank approval. |
| `APPROVED` | Approved; the on-chain operation is complete. |
| `REJECTED` | Declined by the Central Bank; check the reason. |
| `MINT_FAILED` | Approved but the on-chain mint failed — contact your Central Bank. |

### Transfer / swap states

| State | Meaning |
|---|---|
| **Quote ready** | A valid quote is available; you may execute. |
| **Quote expired** | The quote is stale; re-quote before executing. |
| **Executing** | The swap is being submitted on-chain. |
| **Completed** | The transfer settled. |
| **Error** | The swap failed (e.g. slippage exceeded, pool halted). |

### Bridge position states

| State | Meaning |
|---|---|
| **Locked / Minting** | Spoke funds locked; mirrored position being minted on the Hub. |
| **Minted** | Mirrored position is available on the Hub. |
| **Burning / Unlocking** | Mirrored position burned; native CBDC being unlocked on the spoke. |
| **Completed** | The bridge round-trip finished. |

> The exact state labels are shown in the **Bridge Positions** filter; use them to
> track where a position currently sits.

---

## 15. Troubleshooting

### Sign-in fails with "unauthorized"
- Confirm Client ID / Secret are correct and the institution is `ACTIVE`.
- Confirm the backend services are running and reachable.

### A transfer/swap fails with an approval or allowance error
- Go to [Approve AMM](#9-approve-amm) and approve enough tCeBM for the AMM first.

### "Quote expired" / Execute is disabled
- Re-request the quote. If the pool is inactive or halted (circuit breaker), wait
  until governance resumes it.

### A bridge position is stuck
- Check its state in **Bridge Positions** and **Refresh**.
- The relay must carry events between the spoke and the Hub — check relay status in
  the NOC Portal before assuming a contract bug.

### Issuance / tokenisation / redeem stuck in `PENDING`
- The Central Bank must approve it from the Governance Portal (Issuance / Tokenisation
  / Redeem Approvals).
