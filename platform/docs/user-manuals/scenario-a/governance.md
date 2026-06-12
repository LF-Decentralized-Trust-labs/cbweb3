# Scenario A — Governance Portal User Manual

**Audience:** Central bank governance operators.
**Data source:** Live backend (real API Gateway), with a development mock toggle.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Who uses it](#2-who-uses-it)
3. [Login and access](#3-login-and-access)
4. [Dashboard](#4-dashboard)
5. [Registry](#5-registry)
6. [Planned / not yet available](#6-planned--not-yet-available)
7. [Status reference](#7-status-reference)
8. [Troubleshooting](#8-troubleshooting)

---

## 1. Overview

The Governance Portal is the Central Bank's control surface for **Scenario A —
Enhanced Correspondent Banking**. In the current build it focuses on **network
oversight** and the **participant (compliance) registry**: confirming who is in the
network, their onboarding state, and their compliance credentials.

> **Scope note.** Scenario A intentionally exposes a reduced Governance toolset.
> The richer governance controls (account freezing, parameters, circuit breaker,
> audit) are part of **Scenario B** and are listed under
> [Planned / not yet available](#6-planned--not-yet-available) below so banks are
> not told about screens they cannot reach in Scenario A.

| Actor | Portal | Role |
|---|---|---|
| Central Bank Governance Operator | Governance Portal | Monitors the network and manages the participant registry |

---

## 2. Who uses it

Central bank governance / oversight operators. Access is provisioned by the
network administrator; you sign in with a Central Bank operator account.

---

## 3. Login and access

Open the Governance Portal URL (the Central Bank entry provided by your
administrator — see the **Portal Ports** table in
[`scenario-a/frontend/README.md`](../../../scenario-a/frontend/README.md)). Enter
your credentials to reach the Dashboard.

![Screenshot: Scenario A — Governance login](../img/scenario-a/governance-login.png) <!-- TODO: capture screenshot -->

> If sign-in fails immediately with correct credentials, confirm your operator
> account is enabled and the Central Bank backend services are running.

---

## 4. Dashboard

**Route:** `/` · **Sidebar label:** Dashboard

A real-time overview of the spoke: network-level supply, active institutions, and
recent activity. Use it as your landing page each session to confirm the network
is healthy before acting on the registry.

![Screenshot: Scenario A — Governance Dashboard](../img/scenario-a/governance-dashboard.png) <!-- TODO: capture screenshot -->

---

## 5. Registry

**Route:** `/registry` · **Sidebar label:** Registry

The participant registry lists every institution known to the network together
with its identity and compliance status. This is where the Central Bank confirms
that a participant has been onboarded and holds the required credentials before it
transacts.

![Screenshot: Scenario A — Governance Registry](../img/scenario-a/governance-registry.png) <!-- TODO: capture screenshot -->

| Column | Description |
|---|---|
| **Institution** | The participant's display name. |
| **Identity / Address** | The participant's on-chain or Paladin identity. |
| **Status** | The participant's registry / compliance status (see [Status reference](#7-status-reference)). |
| **Jurisdiction / Country** | The participant's registered jurisdiction. |

Use the search field to find a participant by name, address, or jurisdiction.

---

## 6. Planned / not yet available

The following Governance modules exist in the platform design and are active in
**Scenario B**, but are **not exposed** in the Scenario A build. Do not document or
demonstrate them as Scenario A features:

- **Accounts** — account freeze / unfreeze controls
- **Circuit Breaker** — pause / resume controls
- **Parameters** — governance parameter management
- **Audit** — audit log review
- **Settings**

For these capabilities, see the [Scenario B Governance manual](../scenario-b/governance.md).

---

## 7. Status reference

### Participant / registry status

| Status | Meaning |
|---|---|
| `ACTIVE` | Participant is onboarded and cleared to transact. |
| `PENDING` | Onboarding or credential issuance is still in progress. |
| `CREDENTIAL_REQUESTED` | The participant has requested a compliance credential awaiting issuance. |
| `SUSPENDED` / `FROZEN` | Participant is blocked from transacting. |

> The exact set of values shown depends on the participant's progress through
> onboarding. If a status is not listed here, check the participant's detail and
> contact the compliance team.

---

## 8. Troubleshooting

### Sign-in fails with correct credentials
- Confirm your Central Bank operator account is enabled.
- Confirm the Central Bank backend (API Gateway, compliance service) is running.

### Registry is empty or stale
- The page does not auto-refresh — reload to pull the latest registry.
- Confirm at least one institution has completed onboarding.
- Confirm the portal can reach the compliance service through the API Gateway.

### A participant is missing from the registry
- The institution may not have finished onboarding; ask them to complete it.
- Verify you are connected to the correct spoke / Central Bank backend.
