# Scenario A — Bank Portal User Manual

**Audience:** Commercial bank operators participating in a CBWeb3 spoke.
**Data source:** Live backend (real API Gateway).

---

## 1. Overview

The Bank Portal is the operational interface for a commercial bank operator in
**Scenario A — Enhanced Correspondent Banking**. Through it you manage the full
digital-currency lifecycle: obtain tCeBM (tokenised Central Bank Money), agree FX
trades with a counterparty bank, and settle them atomically across spokes using
**PvP (Payment vs. Payment)** Hash Time Lock Contracts.

> **This portal already has a complete, reference-quality operator guide.**
> The detailed step-by-step walkthrough — with screenshots, every field, and full
> troubleshooting — lives at:
>
> **[`scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md`](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md)**
>
> That document is the model these manuals follow. This page is a short index into
> it so the Bank Portal sits alongside the other portal manuals.

| Actor | Portal | Role |
|---|---|---|
| Commercial Bank Operator | Bank Portal | Manages balances, requests issuance/redemption, runs FX trades and PvP settlements |

---

## 2. Who uses it

Commercial bank operators at a participating institution. Access requires an
`ACTIVE` institution and a Client ID / Client Secret issued by the Central Bank.

---

## 3. Login / access

Open the **dispatcher URL** provided by your administrator, enter your **Client ID**
and **Client Secret**, and you are routed to the Bank Portal.

![Screenshot: Scenario A — Bank Portal login](../img/scenario-a/bank-login.png) <!-- TODO: capture screenshot -->

See [BANK-PORTAL-DOCS § 3](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md#3-access-and-login)
for the full login walkthrough.

---

## 4. Screens available in this build

These are the modules exposed in the Scenario A Bank Portal left navigation:

| Sidebar label | Route | What it does | Full guide |
|---|---|---|---|
| **Dashboard** | `/` | Balances, pending counts, recent PvP activity | [§ 4](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md#4-dashboard) |
| **Issuance Requests** | `/deposits` | Request the Central Bank to issue tCeBM | [§ 6](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md#6-token-issuance--deposits) |
| **Reserve Tokenisation** | `/escrows` | Convert approved reserves into tCeBM | [§ 7](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md#7-reserve-tokenisation--escrows) |
| **Redeems** | `/redeems` | Redeem tCeBM back to fiat reserves | [§ 8](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md#8-token-redemption--redeems) |
| **Trade Agreements** | `/agreements` | Propose, accept and manage FX agreements | [§ 9](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md#9-trade-agreements) |
| **PvP Settlement** | `/htlc` | Initiate / continue / complete atomic settlements | [§ 10](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md#10-cross-border-pvp-settlement--htlc) |
| **Onboarding** | `/onboarding` | Register a new institution | [§ 5](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md#5-onboarding) |

### Planned / not yet available

The following items exist in the codebase but are **not exposed** in the current
Scenario A build (hidden from the sidebar) and should not be presented to banks as
working features:

- **Liquidity & Transfers**
- **Automated FX Trading (AMM)**
- **Compliance Center**
- **Settings**

These are active in Scenario B (see the [Scenario B Bank manual](../scenario-b/bank.md)).

---

## 5. Status reference

See [BANK-PORTAL-DOCS § 11](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md#11-status-reference)
for the full status tables (Deposit / Escrow / Redeem statuses, Trade Agreement
states, and HTLC contract states).

---

## 6. Troubleshooting

See [BANK-PORTAL-DOCS § 12](../../../scenario-a/docs/runbooks/BANK-PORTAL-DOCS.md#12-troubleshooting)
for the full troubleshooting guide (login failures, stuck requests, stalled HTLC
settlements, and counterparty agreement issues).
