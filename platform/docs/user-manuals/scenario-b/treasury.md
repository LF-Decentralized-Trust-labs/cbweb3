# Scenario B — Treasury Portal User Manual

**Audience:** Central bank treasury / issuance operators.
**Data source:** ⚠️ **Mock (sample) data in the current build.**

---

> ## ⚠️ Read first — current status
>
> In Scenario B the Treasury Portal currently runs on **mock (sample) data**. The
> screens show the intended treasury workflow — funding, issuance, redemption,
> reconciliation, KYC and audit — but the figures and records are **illustrative,
> not live**, and the sign-in is a demonstration login. Do not use any value here
> for a real treasury decision yet. Live backend wiring is planned and **not yet
> available** for this portal in Scenario B.
>
> (The Scenario A Treasury Portal *is* wired to the live backend for the approvals
> workflow — see the [Scenario A Treasury manual](../scenario-a/treasury.md).)

---

## Table of Contents

1. [Overview](#1-overview)
2. [Who uses it](#2-who-uses-it)
3. [Login and access](#3-login-and-access)
4. [Dashboard](#4-dashboard)
5. [Funding Requests](#5-funding-requests)
6. [Issuance](#6-issuance)
7. [Redemption](#7-redemption)
8. [Reconciliation](#8-reconciliation)
9. [KYC](#9-kyc)
10. [Audit](#10-audit)
11. [Settings](#11-settings)
12. [Status reference](#12-status-reference)
13. [Troubleshooting](#13-troubleshooting)

---

## 1. Overview

The Treasury Portal is the Central Bank's treasury workbench for managing the
issuance and redemption lifecycle, reconciling reserves against tokens in
circulation, and reviewing KYC. In the current Scenario B build it is a **UI preview
on sample data**.

| Actor | Portal | Role (intended) |
|---|---|---|
| Central Bank Treasury Operator | Treasury Portal | Funding, issuance, redemption, reconciliation, KYC review |

---

## 2. Who uses it

Central bank treasury / issuance operators.

---

## 3. Login and access

Open the Treasury Portal URL and sign in with the demonstration credentials.

![Screenshot: Scenario B — Treasury login](../img/scenario-b/treasury-login.png) <!-- TODO: capture screenshot -->

> **Demo login.** Sign-in is a placeholder in this build, not production
> authentication.

---

## 4. Dashboard

**Route:** `/` · **Sidebar label:** Dashboard

A treasury overview: reserve position, tokens in circulation, and outstanding
requests.

![Screenshot: Scenario B — Treasury Dashboard](../img/scenario-b/treasury-dashboard.png) <!-- TODO: capture screenshot -->

> **Sample data.** All figures are illustrative.

---

## 5. Funding Requests

**Route:** `/funding-requests` · **Sidebar label:** Funding Requests

Lists requests for treasury funding for review and action.

![Screenshot: Scenario B — Funding Requests](../img/scenario-b/treasury-funding-requests.png) <!-- TODO: capture screenshot -->

> **Sample data.** Requests shown are illustrative.

---

## 6. Issuance

**Route:** `/issuance` · **Sidebar label:** Issuance

The treasury view of tCeBM issuance against reserves.

![Screenshot: Scenario B — Issuance](../img/scenario-b/treasury-issuance.png) <!-- TODO: capture screenshot -->

> **Sample data.**

---

## 7. Redemption

**Route:** `/redemption` · **Sidebar label:** Redemption

The treasury view of tCeBM redemption back to fiat reserves (burn).

![Screenshot: Scenario B — Redemption](../img/scenario-b/treasury-redemption.png) <!-- TODO: capture screenshot -->

> **Sample data.**

---

## 8. Reconciliation

**Route:** `/reconciliation` · **Sidebar label:** Reconciliation

Reconciles fiat reserves against tokens in circulation to confirm full backing.

![Screenshot: Scenario B — Reconciliation](../img/scenario-b/treasury-reconciliation.png) <!-- TODO: capture screenshot -->

> **Sample data.** Reconciliation figures are illustrative.

---

## 9. KYC

**Route:** `/kyc` · **Sidebar label:** KYC

Review of participant KYC status from the treasury perspective.

![Screenshot: Scenario B — Treasury KYC](../img/scenario-b/treasury-kyc.png) <!-- TODO: capture screenshot -->

> **Sample data.**

---

## 10. Audit

**Route:** `/audit` · **Sidebar label:** Audit

A record of treasury actions for review.

> **Sample data.**

---

## 11. Settings

**Route:** `/settings` · **Sidebar label:** Settings

Portal preferences for the current session.

---

## 12. Status reference

Because the portal runs on sample data, status values are illustrative. The intended
issuance / redemption statuses follow the platform-wide pattern:

| Status | Intended meaning |
|---|---|
| `PENDING` | Awaiting a treasury decision. |
| `APPROVED` | Approved; on-chain mint / burn complete. |
| `REJECTED` | Declined; reason recorded. |
| `MINT_FAILED` | Approved but the on-chain operation failed. |

---

## 13. Troubleshooting

### Figures look static / unrealistic
- Expected — the portal is on **sample data**; there is no live feed to refresh.

### I cannot find a real request or reserve figure
- Expected — records are illustrative, not the live data set.

### Login behaves differently from the Bank or Governance portals
- Expected — this is a demonstration login, not production authentication.

> When this portal is wired to the live backend, this manual will be updated to
> remove the sample-data warnings and document real behaviour.
