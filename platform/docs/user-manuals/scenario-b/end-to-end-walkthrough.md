# End-to-End Walkthrough (Scenario B)

**Audience:** Anyone running or demonstrating a complete Scenario B cross-border
payment — operators, integrators, reviewers, and demo drivers.
**Scenario:** Scenario B — International Hub (FXAgreement + AMM + Bridge, with
relay and circuit breaker).
**Scope:** This document ties the five per-portal manuals together into one
"golden path": the ordered sequence of actions, across every portal, that moves
value from a payer at Bank A to a beneficiary at Bank B and settles cleanly.

Each per-portal manual documents its own screens in isolation (see the
"Typical Workflows" section in each). This walkthrough is the cross-portal view:
who does what, in which portal, in what order, and what a successful result looks
like at each hand-off.

> This is the **happy path** — the successful case with no exceptions. Timeout,
> refund, rejection, and circuit-breaker paths are covered per-screen in the
> individual manuals and in the Troubleshooting section of each.

---

## Table of Contents

1. [The cast](#1-the-cast)
2. [Prerequisites](#2-prerequisites)
3. [The golden path](#3-the-golden-path)
   - 3.1 [Stage A — Provision Hub liquidity (Central Banks)](#31-stage-a--provision-hub-liquidity-central-banks)
   - 3.2 [Stage B — Onboard the commercial banks](#32-stage-b--onboard-the-commercial-banks)
   - 3.3 [Stage C — Issue fiat reserve (fCeBM)](#33-stage-c--issue-fiat-reserve-fcebm)
   - 3.4 [Stage D — Tokenise the reserve (tCeBM)](#34-stage-d--tokenise-the-reserve-tcebm)
   - 3.5 [Stage E — Send the cross-currency payment](#35-stage-e--send-the-cross-currency-payment)
   - 3.6 [Stage F — Beneficiary receipt at Bank B](#36-stage-f--beneficiary-receipt-at-bank-b)
   - 3.7 [Stage G — Central Bank withdraws liquidity](#37-stage-g--central-bank-withdraws-liquidity)
4. [Oversight overlay (optional)](#4-oversight-overlay-optional)
5. [Success checklist](#5-success-checklist)

---

## 1. The cast

| Actor | Portal | Manual |
|---|---|---|
| Central Bank A / B treasury (liquidity, issuance, tokenisation approvals) | Treasury | [treasury.md](./treasury.md) |
| Central Bank A / B governance (registry, circuit breaker, oversight) | Governance | [governance.md](./governance.md) |
| Commercial Bank A operator (payer) | Bank | [bank.md](./bank.md) |
| Commercial Bank B operator (beneficiary) | Bank | [bank.md](./bank.md) |
| Supervisor / regulator (read-only monitoring) | Supervisor | [supervisor.md](./supervisor.md) |
| Network operations engineer (infrastructure and relay health) | NOC | [noc.md](./noc.md) |

Running example used throughout: **Spoke A settles in BRL, Spoke B settles in
ARS.** Bank A pays in its home currency (BRL) and Bank B receives the
counterpart currency (ARS). Substitute your own corridor's currencies as needed.

---

## 2. Prerequisites

Before you start, confirm:

- **Access.** You have the portal URL and sign-in credentials for each role you
  will act as, provided by your administrator or Central Bank. Each manual's
  "Access and Login" section describes the sign-in screen for its portal.
- **A healthy network.** The network and relay must be operational so the bridge
  and commit-reveal steps can settle. A network operations engineer can confirm
  this in the NOC portal: all spokes and the hub report healthy on the Dashboard,
  and the relay is running.
  See [noc.md → Relay Status](./noc.md#43-relay-status-relays).

If a gateway or the relay is unavailable, the bridge and liquidity-commit steps
stall in a pending state rather than fail loudly — check relay health first.

---

## 3. The golden path

The path has seven stages. Stages A–D are one-time set-up per corridor and per
bank; Stages E–F are the payment itself; Stage G is routine liquidity
management.

### 3.1 Stage A — Provision Hub liquidity (Central Banks)

The Hub AMM needs a funded pool for the corridor before any bank can transact.
Both central banks contribute their side, and the pool activates only once both
commits are revealed (2-of-N commit-reveal).

| # | Actor | Portal → screen | Action | Expected result |
|---|---|---|---|---|
| A1 | CB-A treasury | Treasury → [Liquidity Provisioning](./treasury.md#46-liquidity-provisioning) | Propose the pair, then seed side A (bridge tCeBM to the Hub, approve the AMM, submit the liquidity commit) | CB-A commit registered |
| A2 | CB-B treasury | Treasury → [Liquidity Provisioning](./treasury.md#46-liquidity-provisioning) | Confirm the pair, then seed side B the same way | CB-B commit registered |
| A3 | Relay | (automatic) | The relay observes both bridge locks and matches the commits | Both commits move to EXECUTED |
| A4 | CB-A / CB-B treasury | Treasury → [Liquidity Management](./treasury.md#45-liquidity-management) | Refresh the pool | Pool status is **ACTIVE**, with non-zero reserves on both sides |

<!-- SCREENSHOT-NEW: ../img/scenario-b/end-to-end-walkthrough/01-pool-active.png — Treasury Liquidity Management showing the corridor pool ACTIVE with reserves on both sides -->

### 3.2 Stage B — Onboard the commercial banks

Each commercial bank registers with its own central bank. Bank B must be
onboarded too, because the payment resolves the beneficiary by bank identity.

| # | Actor | Portal → screen | Action | Expected result |
|---|---|---|---|---|
| B1 | Bank A operator | Bank → [Onboarding](./bank.md#48-onboarding-onboarding) | Complete the registration wizard | Onboarding submitted to CB-A |
| B2 | CB-A governance | Governance → [Registry](./governance.md#52-registry) | Approve Bank A's KYC | Bank A status becomes ACTIVE |
| B3 | Bank B operator | Bank → [Onboarding](./bank.md#48-onboarding-onboarding) | Complete the registration wizard | Onboarding submitted to CB-B |
| B4 | CB-B governance | Governance → [Registry](./governance.md#52-registry) | Approve Bank B's KYC | Bank B status becomes ACTIVE |

Bank A can now sign in and reach the operational screens (see
[bank.md → Access and Login](./bank.md#2-access-and-login)).

### 3.3 Stage C — Issue fiat reserve (fCeBM)

Bank A converts a real fiat deposit into the on-chain fiat-reserve token (fCeBM).

| # | Actor | Portal → screen | Action | Expected result |
|---|---|---|---|---|
| C1 | Bank A operator | Bank → [Issuance Requests / Deposits](./bank.md#45-issuance-requests--deposits-deposits) | Submit a deposit (issuance) request with the fiat collateral proof | Request created, status PENDING |
| C2 | CB-A treasury | Treasury → [Issuance Approvals](./treasury.md#42-issuance-approvals) | Approve the request | fCeBM minted to Bank A; request APPROVED/SETTLED |
| C3 | Bank A operator | Bank → [Dashboard](./bank.md#41-dashboard) | Refresh | Fiat Reserve (fCeBM) balance is non-zero |

### 3.4 Stage D — Tokenise the reserve (tCeBM)

Bank A converts its approved fiat reserve into transferable CBDC (tCeBM). This is
an atomic burn-fCeBM / mint-tCeBM operation on the central bank's side.

| # | Actor | Portal → screen | Action | Expected result |
|---|---|---|---|---|
| D1 | Bank A operator | Bank → [Reserve Tokenisation / Escrows](./bank.md#46-reserve-tokenisation--escrows-escrows) | Request tokenisation of the fCeBM reserve | Escrow created, status PENDING |
| D2 | CB-A treasury | Treasury → [Tokenisation Approvals](./treasury.md#43-tokenisation-approvals) | Approve the escrow | fCeBM burned, tCeBM minted to Bank A (both tx hashes recorded) |
| D3 | Bank A operator | Bank → [Dashboard](./bank.md#41-dashboard) | Refresh | tCeBM balance is non-zero — Bank A is ready to transact |

### 3.5 Stage E — Send the cross-currency payment

This is the payment itself: Bank A spends its home-currency tCeBM and the Hub AMM
delivers the counterpart currency toward Bank B. Direction is fixed by the
selected pool's sovereignty — the payer does not choose currencies freely.

| # | Actor | Portal → screen | Action | Expected result |
|---|---|---|---|---|
| E1 | Bank A operator | Bank → [Bridge](./bank.md#42-bridge-bridge) | Select the corridor pool and request a quote (BRL → ARS) | Quote returned with an effective rate and a valid-for countdown |
| E2 | Bank A operator | Bank → [Bridge](./bank.md#42-bridge-bridge) | Set the max amount in and the beneficiary bank ID, then Execute Bridge | Swap initiated with a swap ID and correlation ID; status begins progressing |
| E3 | Bank A operator | Bank → [Bridge](./bank.md#42-bridge-bridge) | Wait for the progress indicator to complete | Bridge Completed card: status **COMPLETED**, with amounts, tx hash, and correlation ID |
| E4 | Bank A operator | Bank → [Bridge History](./bank.md#43-bridge-history-bridgehistory) | Open the history | The operation appears with its route, amounts, rate, and COMPLETED status |

<!-- SCREENSHOT-NEW: ../img/scenario-b/end-to-end-walkthrough/02-swap-completed.png — Bank Bridge page showing the Bridge Completed card with status COMPLETED, amounts, tx hash, and correlation ID -->

### 3.6 Stage F — Beneficiary receipt at Bank B

The Hub side of the swap mints the counterpart tCeBM toward Bank B. This is
relay-mediated: the relay burns the Hub position and the beneficiary central
bank's bridge delivers tCeBM to Bank B.

| # | Actor | Portal → screen | Action | Expected result |
|---|---|---|---|---|
| F1 | Relay | (automatic) | Relay processes the outbound leg; CB-B bridge position reaches BURNED | Counterpart tCeBM (ARS) minted to Bank B |
| F2 | Bank B operator | Bank → [Dashboard](./bank.md#41-dashboard) | Refresh | tCeBM balance is non-zero — the payment is received |
| F3 | CB governance | Governance → [Swap Monitor](./governance.md#54-swap-monitor) | Review | The swap shows as completed end-to-end |

<!-- SCREENSHOT-NEW: ../img/scenario-b/end-to-end-walkthrough/03-bank-b-receipt.png — Bank B Dashboard showing the received tCeBM balance after the cross-currency payment settles -->

### 3.7 Stage G — Central Bank withdraws liquidity

Routine liquidity management: a central bank can withdraw part of its LP position
back to its home currency. A partial withdrawal keeps the position ACTIVE.

| # | Actor | Portal → screen | Action | Expected result |
|---|---|---|---|---|
| G1 | CB-A treasury | Treasury → [Liquidity Management](./treasury.md#45-liquidity-management) | Review the on-chain LP-share position | LP shares and pool share are shown |
| G2 | CB-A treasury | Treasury → [Liquidity Provisioning](./treasury.md#46-liquidity-provisioning) | Withdraw a fraction of the position (home-currency exit) | LP shares decrease but remain greater than zero; the position stays ACTIVE |

---

## 4. Oversight overlay (optional)

Central-bank oversight runs alongside the payment flow. These actions do not
block the payment but exercise the control surface:

- **Circuit breaker.** A governance operator can pause AMM swaps for a pair
  (1-of-N) and resume them with a second signature (2-of-N). On the successful
  path the breaker stays LIVE throughout.
  See [governance.md → Circuit Breaker](./governance.md#55-circuit-breaker).
- **Master viewing key disclosure.** A supervisor opens a disclosure request and
  a second party signs it (2-of-N) to reveal the details of a specific swap for
  investigation.
  See [supervisor.md → Investigation](./supervisor.md#56-investigation) and
  [governance.md → Oversight](./governance.md#57-oversight).
- **Continuous monitoring.** Throughout the flow, the Supervisor portal's
  [Liquidity Monitor](./supervisor.md#52-liquidity-monitor) and
  [Stability Insights](./supervisor.md#55-stability-insights) reflect pool health,
  and the NOC portal's [Pool Stability](./noc.md#44-pool-stability-pool-stability)
  view tracks reserve balance.

---

## 5. Success checklist

The walkthrough is complete when all of the following hold:

- [ ] Corridor pool is **ACTIVE** with non-zero reserves on both sides (Treasury → Liquidity Management).
- [ ] Bank A and Bank B are both **ACTIVE** in the Registry (Governance → Registry).
- [ ] Bank A holds a non-zero **tCeBM** balance after tokenisation (Bank → Dashboard).
- [ ] The swap reaches **COMPLETED** with a tx hash and correlation ID (Bank → Bridge / Bridge History).
- [ ] Bank B holds a non-zero counterpart **tCeBM** balance (Bank → Dashboard).
- [ ] The central bank's LP position remains **ACTIVE** after a partial withdrawal (Treasury → Liquidity Management).
- [ ] No stuck relay events and no unexpected circuit-breaker HALTED state (NOC → Relay Status; Governance → Circuit Breaker).
