# Scenario B — Governance Portal User Manual

**Audience:** Central bank governance operators.
**Data source:** Live backend (real API Gateway), with a development mock toggle.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Who uses it](#2-who-uses-it)
3. [Login and access](#3-login-and-access)
4. [Dashboard](#4-dashboard)
5. [Registry](#5-registry)
6. [Liquidity Management](#6-liquidity-management)
7. [Issuance Approvals](#7-issuance-approvals)
8. [Tokenisation Approvals](#8-tokenisation-approvals)
9. [Redeem Approvals](#9-redeem-approvals)
10. [Swap Monitor](#10-swap-monitor)
11. [Circuit Breaker](#11-circuit-breaker)
12. [Oversight](#12-oversight)
13. [Audit](#13-audit)
14. [Settings](#14-settings)
15. [Status reference](#15-status-reference)
16. [Troubleshooting](#16-troubleshooting)

---

## 1. Overview

The Governance Portal is the Central Bank's full control surface for **Scenario B —
International Hub**. Beyond the participant registry and issuance approvals it adds
the Scenario B-specific governance tools: **Hub liquidity management**, an **AMM swap
monitor**, the **circuit breaker** that can pause and resume the AMM, and broader
**oversight**.

| Actor | Portal | Role |
|---|---|---|
| Central Bank Governance Operator | Governance Portal | Approves issuance/redemption, manages Hub liquidity, monitors swaps, operates the circuit breaker, oversees the network |

> **Circuit breaker and atomicity.** The Scenario B settlement model requires the
> circuit breaker to be validated before swaps proceed: it pauses on a **1-of-N**
> signal and resumes on **2-of-N**. The [Circuit Breaker](#11-circuit-breaker) screen
> is where governance exercises this control.

---

## 2. Who uses it

Central bank governance / oversight operators. Access is provisioned by the network
administrator; sign in with a Central Bank operator account.

---

## 3. Login and access

Open the Governance Portal URL (the Central Bank entry in the **Portal Ports** table
of [`scenario-b/frontend/README.md`](../../../scenario-b/frontend/README.md)) and
sign in.

![Screenshot: Scenario B — Governance login](../img/scenario-b/governance-login.png) <!-- TODO: capture screenshot -->

---

## 4. Dashboard

**Route:** `/` · **Sidebar label:** Dashboard

Network-level overview for the Hub: supply, active institutions, pool/AMM
indicators, and recent governance activity.

![Screenshot: Scenario B — Governance Dashboard](../img/scenario-b/governance-dashboard.png) <!-- TODO: capture screenshot -->

---

## 5. Registry

**Route:** `/registry` · **Sidebar label:** Registry

The participant registry: every institution known to the network with its identity
and compliance status. This is also where pending KYC / credential requests are
reviewed and approved.

![Screenshot: Scenario B — Registry](../img/scenario-b/governance-registry.png) <!-- TODO: capture screenshot -->

| Column | Description |
|---|---|
| **Institution** | Participant display name. |
| **Identity / Wallet** | On-chain or Paladin identity. |
| **Status** | Registry / compliance status (see [Status reference](#15-status-reference)). |
| **Country** | Registered jurisdiction. |

Pending credential requests appear with status `CREDENTIAL_REQUESTED`; approve them
to clear the participant to transact.

---

## 6. Liquidity Management

**Route:** `/liquidity` · **Sidebar label:** Liquidity Management

Manage the Hub's AMM liquidity — the reserves that back cross-border transfers. Use
it to review pool balances and the institution's committed liquidity.

![Screenshot: Scenario B — Liquidity Management](../img/scenario-b/governance-liquidity.png) <!-- TODO: capture screenshot -->

> Keeping pools adequately funded prevents transfers from failing on price impact /
> slippage. Watch this alongside the [Swap Monitor](#10-swap-monitor).

---

## 7. Issuance Approvals

**Route:** `/deposits-approval` · **Sidebar label:** Issuance Approvals

Commercial-bank requests to **issue tCeBM**. Open a request, review the institution
and amount, then **Approve** (mints on-chain) or **Reject** (records a reason).

![Screenshot: Scenario B — Issuance Approvals](../img/scenario-b/governance-issuance-approvals.png) <!-- TODO: capture screenshot -->

---

## 8. Tokenisation Approvals

**Route:** `/escrows-approval` · **Sidebar label:** Tokenisation Approvals

Requests to **convert approved reserves into tCeBM**. Approving executes the
burn-and-mint to the bank's wallet. Flow mirrors [Issuance Approvals](#7-issuance-approvals).

![Screenshot: Scenario B — Tokenisation Approvals](../img/scenario-b/governance-tokenisation-approvals.png) <!-- TODO: capture screenshot -->

---

## 9. Redeem Approvals

**Route:** `/redeems-approval` · **Sidebar label:** Redeem Approvals

Requests to **redeem tCeBM back to fiat**. Approving authorises the fiat release.
Flow mirrors [Issuance Approvals](#7-issuance-approvals).

![Screenshot: Scenario B — Redeem Approvals](../img/scenario-b/governance-redeem-approvals.png) <!-- TODO: capture screenshot -->

---

## 10. Swap Monitor

**Route:** `/swap-monitor` · **Sidebar label:** Swap Monitor

A read-only monitor of AMM swap activity on the Hub: which swaps executed, sizes, and
price impact. Use it to spot unusual flow or stressed pools.

![Screenshot: Scenario B — Swap Monitor](../img/scenario-b/governance-swap-monitor.png) <!-- TODO: capture screenshot -->

---

## 11. Circuit Breaker

**Route:** `/circuit-breaker` · **Sidebar label:** Circuit Breaker

The safety control for the AMM. Governance can **pause** swaps (halt) and **resume**
them. Pausing takes effect on a single authorised signal (1-of-N); resuming requires
a second authorisation (2-of-N) to prevent a single operator from re-opening the
market unilaterally.

![Screenshot: Scenario B — Circuit Breaker](../img/scenario-b/governance-circuit-breaker.png) <!-- TODO: capture screenshot -->

| Control | Effect |
|---|---|
| **Pause / Halt** | Stops AMM swaps immediately. Transfers fail closed until resumed. |
| **Resume** | Re-enables swaps once the required approvals are present. |

> While the breaker is engaged, bank operators will see transfers disabled with a
> halted-pool message. Communicate planned pauses to participants.

---

## 12. Oversight

**Route:** `/oversight` · **Sidebar label:** Oversight

A consolidated oversight view of network activity and compliance posture for the
Central Bank, complementing the registry and swap monitor.

![Screenshot: Scenario B — Oversight](../img/scenario-b/governance-oversight.png) <!-- TODO: capture screenshot -->

---

## 13. Audit

**Route:** `/audit` · **Sidebar label:** Audit

A record of governance and compliance actions for review.

![Screenshot: Scenario B — Governance Audit](../img/scenario-b/governance-audit.png) <!-- TODO: capture screenshot -->

---

## 14. Settings

**Route:** `/settings` · **Sidebar label:** Settings

Portal preferences for the current session.

---

## 15. Status reference

### Participant / registry status

| Status | Meaning |
|---|---|
| `ACTIVE` | Onboarded and cleared to transact. |
| `PENDING` | Onboarding / credential issuance in progress. |
| `CREDENTIAL_REQUESTED` | Awaiting credential approval in the Registry. |
| `SUSPENDED` / `FROZEN` | Blocked from transacting. |

### Approval request statuses

| Status | Meaning |
|---|---|
| `PENDING` | Awaiting a governance decision. |
| `APPROVED` | Approved; on-chain operation complete. |
| `REJECTED` | Declined; a reason is recorded. |
| `MINT_FAILED` | Approved but the on-chain mint failed — investigate. |

### Circuit breaker state

| State | Meaning |
|---|---|
| **Active / Open** | AMM swaps are enabled. |
| **Halted / Paused** | AMM swaps are stopped; transfers fail closed until resumed. |

---

## 16. Troubleshooting

### A request is stuck in `PENDING`
- It awaits a governance decision — open it and Approve or Reject.

### Banks report transfers are failing / disabled
- Check the [Circuit Breaker](#11-circuit-breaker) — if halted, transfers are
  intentionally blocked until resumed (resume needs 2-of-N).
- Check [Liquidity Management](#6-liquidity-management) — an underfunded pool causes
  high price impact and failed swaps.

### A credential request never clears
- Approve the `CREDENTIAL_REQUESTED` entry in the Registry; confirm the compliance
  service is reachable.

### Resume does not re-enable the AMM
- Resuming requires a second authorisation (2-of-N). Confirm a second governance
  operator has approved the resume.
