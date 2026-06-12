# Scenario B — NOC (Network Operations) Portal User Manual

**Audience:** Network operations engineers.
**Data source:** ⚠️ **Mock (sample) data — monitoring views are illustrative.**

---

> ## ⚠️ Read first — current status
>
> The NOC Portal is currently a **monitoring UI populated with sample data**. The
> screens show the intended layout for infrastructure health, relay status, pool
> stability, and network topology, but the figures are **not a live feed**. Do not
> use them for real incident response yet. Live telemetry wiring is planned and
> **not yet available**.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Who uses it](#2-who-uses-it)
3. [Login and access](#3-login-and-access)
4. [Dashboard](#4-dashboard)
5. [Infrastructure](#5-infrastructure)
6. [Relays](#6-relays)
7. [Pool Stability](#7-pool-stability)
8. [Topology](#8-topology)
9. [Audit](#9-audit)
10. [Settings](#10-settings)
11. [Status reference](#11-status-reference)
12. [Troubleshooting](#12-troubleshooting)

---

## 1. Overview

The NOC (Network Operations Center) Portal is the monitoring surface for engineers
keeping the **International Hub** healthy: node and service health, cross-network relay
status, **AMM pool stability**, and the overall topology of spokes, Hub, and
connections.

| Actor | Portal | Role |
|---|---|---|
| Network Operations Engineer | NOC Portal | Infrastructure health, relay monitoring, pool stability, topology |

---

## 2. Who uses it

Network operations / SRE engineers operating the platform. Access is provisioned by
the network administrator.

---

## 3. Login and access

Open the NOC Portal URL provided by your administrator and sign in.

![Screenshot: Scenario B — NOC login](../img/scenario-b/noc-login.png) <!-- TODO: capture screenshot -->

---

## 4. Dashboard

**Route:** `/` · **Sidebar label:** Dashboard

A single-pane overview of network health: healthy vs. degraded components, relay
status, pool stability at a glance, and recent events.

![Screenshot: Scenario B — NOC Dashboard](../img/scenario-b/noc-dashboard.png) <!-- TODO: capture screenshot -->

> **Sample data.** Health counts are illustrative.

---

## 5. Infrastructure

**Route:** `/infrastructure` · **Sidebar label:** Infrastructure

Lists infrastructure components (nodes, services, databases) with their health
state.

![Screenshot: Scenario B — NOC Infrastructure](../img/scenario-b/noc-infrastructure.png) <!-- TODO: capture screenshot -->

| Column | Description |
|---|---|
| **Component** | The node or service name. |
| **Type** | Node / service / database. |
| **Health** | Current health state (see [Status reference](#11-status-reference)). |

> **Sample data.** Component health is illustrative.

---

## 6. Relays

**Route:** `/relays` · **Sidebar label:** Relays

Status of the cross-network relays that carry events between spokes and the Hub — the
component to check first when a transfer or bridge operation appears stuck.

![Screenshot: Scenario B — NOC Relays](../img/scenario-b/noc-relays.png) <!-- TODO: capture screenshot -->

> **Sample data.** Relay states are illustrative.

---

## 7. Pool Stability

**Route:** `/pool-stability` · **Sidebar label:** Pool Stability

Scenario B-specific view of AMM pool health: reserves, imbalance, and stability
alerts. Use it to spot a pool drifting out of tolerance before transfers start
failing on price impact.

![Screenshot: Scenario B — NOC Pool Stability](../img/scenario-b/noc-pool-stability.png) <!-- TODO: capture screenshot -->

> **Sample data.** Pool figures and alerts are illustrative.

---

## 8. Topology

**Route:** `/topology` · **Sidebar label:** Topology

A visual map of spokes, the Hub, and the connections between them.

![Screenshot: Scenario B — NOC Topology](../img/scenario-b/noc-topology.png) <!-- TODO: capture screenshot -->

> **Sample data.** The topology shown is illustrative.

---

## 9. Audit

**Route:** `/audit` · **Sidebar label:** Audit

A record of operational and administrative events for review.

> **Sample data.**

---

## 10. Settings

**Route:** `/settings` · **Sidebar label:** Settings

Portal preferences for the current session.

---

## 11. Status reference

| Health state | Intended meaning |
|---|---|
| **Healthy** | Component is up and responding normally. |
| **Degraded** | Component is responding but impaired. |
| **Down / Unreachable** | Component is not responding. |

| Relay state | Intended meaning |
|---|---|
| **Active** | Relay is connected and forwarding events. |
| **Stalled / Error** | Relay is not making progress; investigate before assuming a contract bug. |

| Pool stability | Intended meaning |
|---|---|
| **Stable** | Pool reserves are within tolerance. |
| **At risk / Imbalanced** | Pool is drifting out of tolerance; transfers may fail on price impact. |

---

## 12. Troubleshooting

### Health, relay, and pool figures never change
- Expected — the portal is on **sample data**. There is no live telemetry feed yet.

### A transfer or bridge operation is stuck — what do I check here?
- Once live, **Relays** is the first place to look: a stalled relay, not a contract
  bug, is the usual cause of a frozen cross-spoke / bridge operation.
- Check **Pool Stability** if transfers are failing on price impact rather than
  freezing.

### Pool Stability shows everything as healthy during a known incident
- Expected in the current build — the data is illustrative, not live.
