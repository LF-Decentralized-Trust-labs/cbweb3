# Scenario A — Supervisor Portal User Manual

**Audience:** Supervisors / regulators.
**Data source:** ⚠️ **Mock (sample) data only — not a live regulatory tool.**

---

> ## ⚠️ Read first — current status (R2-CR-8)
>
> The Supervisor Portal is currently a **UI preview running entirely on mock
> (sample) data**, including a **demonstration login that is not connected to the
> platform's real authentication**. None of the figures, participants, pools, or
> audit entries shown are live.
>
> - **Do not** use any number, balance, alert, or audit record in this portal for a
>   real supervisory or regulatory decision.
> - **Do not** treat the login as a security boundary — it is a demo sign-in.
> - This manual documents only the screens that exist today and labels each as
>   **sample data**. Live data wiring and production authentication are tracked
>   under finding **R2-CR-8** and are **not yet available**.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Who uses it](#2-who-uses-it)
3. [Login and access](#3-login-and-access)
4. [Dashboard](#4-dashboard)
5. [Liquidity Monitor](#5-liquidity-monitor)
6. [Compliance Registry](#6-compliance-registry)
7. [Audit Vault](#7-audit-vault)
8. [Stability Insights](#8-stability-insights)
9. [Settings](#9-settings)
10. [Status reference](#10-status-reference)
11. [Troubleshooting](#11-troubleshooting)

---

## 1. Overview

The Supervisor Portal is designed as a **read-only oversight surface** for
supervisors and regulators: monitoring liquidity, reviewing the participant
registry, inspecting audit records, and watching stability indicators. In the
current build every screen is populated with **sample data** so the intended
layout and workflow can be reviewed ahead of live integration.

| Actor | Portal | Role (intended) |
|---|---|---|
| Supervisor / Regulator | Supervisor Portal | Read-only monitoring, compliance oversight, audit review |

---

## 2. Who uses it

Supervisors and regulators. In the current preview, anyone with the demo login can
view the sample screens; there is no per-institution access control yet.

---

## 3. Login and access

Open the Supervisor Portal URL and sign in with the demonstration credentials
provided by your administrator.

![Screenshot: Scenario A — Supervisor login](../img/scenario-a/supervisor-login.png) <!-- TODO: capture screenshot -->

> **Sample data / demo login.** This sign-in is a placeholder and does not perform
> real authentication. It will be replaced by the platform's production identity
> provider as part of the R2-CR-8 work.

---

## 4. Dashboard

**Route:** `/` · **Sidebar label:** Dashboard

A network-level overview: total tCeBM supply, active institutions, cross-border
agreement counts, and pool indicators.

![Screenshot: Scenario A — Supervisor Dashboard](../img/scenario-a/supervisor-dashboard.png) <!-- TODO: capture screenshot -->

> **Sample data.** All figures on this screen are illustrative.

---

## 5. Liquidity Monitor

**Route:** `/liquidity` · **Sidebar label:** Liquidity Monitor

Shows liquidity positions and pool balances across the network, intended to help a
supervisor spot imbalances.

![Screenshot: Scenario A — Liquidity Monitor](../img/scenario-a/supervisor-liquidity.png) <!-- TODO: capture screenshot -->

> **Sample data.** Balances and imbalance indicators are illustrative.

---

## 6. Compliance Registry

**Route:** `/participants` · **Sidebar label:** Compliance Registry

A searchable list of participating institutions with their compliance status and
jurisdiction. A search box filters by institution, address, or jurisdiction.

![Screenshot: Scenario A — Compliance Registry](../img/scenario-a/supervisor-participants.png) <!-- TODO: capture screenshot -->

> **Sample data.** The participants shown are not the live registry.

---

## 7. Audit Vault

**Route:** `/audit` · **Sidebar label:** Audit Vault

Intended for reviewing audit records and, where authorised, decrypting transaction
details for investigation.

![Screenshot: Scenario A — Audit Vault](../img/scenario-a/supervisor-audit.png) <!-- TODO: capture screenshot -->

> **Sample data.** Audit entries and any decryption results are illustrative and
> must not be used as evidence.

---

## 8. Stability Insights

**Route:** `/stability` · **Sidebar label:** Stability Insights

Surfaces pool stability indicators and alerts intended to flag stressed pools.

![Screenshot: Scenario A — Stability Insights](../img/scenario-a/supervisor-stability.png) <!-- TODO: capture screenshot -->

> **Sample data.** Stability alerts are illustrative.

---

## 9. Settings

**Route:** `/settings` · **Sidebar label:** Settings

Portal preferences for the current session.

---

## 10. Status reference

Because the portal runs on sample data, status values are illustrative. The
intended indicators are:

| Indicator | Intended meaning |
|---|---|
| Pool **balanced** | Pool reserves are within tolerance. |
| Pool **imbalanced** | Pool reserves are outside tolerance and may need attention. |
| Participant **active / suspended** | Whether a participant is cleared to transact. |

---

## 11. Troubleshooting

### The numbers look static or unrealistic
- Expected — the portal is on **sample data**. There is no live feed to refresh.

### I cannot find a real institution / transaction
- Expected — the registry and audit views are illustrative, not the live data set.

### Login behaves differently from the Bank or Governance portals
- Expected — this is a demonstration login, not production authentication
  (tracked under R2-CR-8).

> When the portal is wired to live data and production authentication, this manual
> will be updated to remove the sample-data warnings and document the real
> behaviour of each screen.
