# Scenario A — NOC (Network Operations) Portal User Manual

**Audience:** Network operations engineers.
**Data source:** ⚠️ **Mock (sample) data — monitoring views are illustrative.**

---

> ## ⚠️ Read first — current status
>
> The NOC Portal is currently a **monitoring UI populated with sample data**. The
> screens show the intended layout for infrastructure health, relay status, and
> network topology, but the figures are **not a live feed**. Do not use them for
> real incident response yet. Live telemetry wiring is planned and **not yet
> available**.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Who uses it](#2-who-uses-it)
3. [Login and access](#3-login-and-access)
4. [Dashboard](#4-dashboard)
5. [Infrastructure](#5-infrastructure)
6. [Relays](#6-relays)
7. [Topology](#7-topology)
8. [Log Viewer](#8-log-viewer)
9. [Audit](#9-audit)
10. [Settings](#10-settings)
11. [Status reference](#11-status-reference)
12. [Troubleshooting](#12-troubleshooting)

---

## 1. Overview

The NOC (Network Operations Center) Portal is the monitoring surface for engineers
who keep the network healthy: node and service health, cross-network relay status,
and the overall topology of spokes and connections.

| Actor | Portal | Role |
|---|---|---|
| Network Operations Engineer | NOC Portal | Infrastructure health, relay monitoring, topology, log review |

---

## 2. Who uses it

Network operations / SRE engineers operating the platform. Access is provisioned by
the network administrator.

---

## 3. Login and access

Open the NOC Portal URL provided by your administrator and sign in.

![Screenshot: Scenario A — NOC login](../img/scenario-a/noc-login.png) <!-- TODO: capture screenshot -->

---

## 4. Dashboard

**Route:** `/` · **Sidebar label:** Dashboard

A single-pane overview of network health: how many components are healthy vs.
degraded, relay status at a glance, and recent events.

![Screenshot: Scenario A — NOC Dashboard](../img/scenario-a/noc-dashboard.png) <!-- TODO: capture screenshot -->

> **Sample data.** Health counts are illustrative.

---

## 5. Infrastructure

**Route:** `/infrastructure` · **Sidebar label:** Infrastructure

Lists infrastructure components (nodes, services, databases) with their health
state. Each component links to its logs (see [Log Viewer](#8-log-viewer)).

![Screenshot: Scenario A — NOC Infrastructure](../img/scenario-a/noc-infrastructure.png) <!-- TODO: capture screenshot -->

| Column | Description |
|---|---|
| **Component** | The node or service name. |
| **Type** | Node / service / database. |
| **Health** | Current health state (see [Status reference](#11-status-reference)). |
| **Action** | Open the component's logs. |

> **Sample data.** Component health is illustrative.

---

## 6. Relays

**Route:** `/relays` · **Sidebar label:** Relays

Shows the status of the cross-network relays that carry events between spokes —
the component to check first when a cross-border settlement appears stuck.

![Screenshot: Scenario A — NOC Relays](../img/scenario-a/noc-relays.png) <!-- TODO: capture screenshot -->

> **Sample data.** Relay states are illustrative.

---

## 7. Topology

**Route:** `/topology` · **Sidebar label:** Topology

A visual map of spokes and the connections between them, for understanding the
network shape and where a fault sits.

![Screenshot: Scenario A — NOC Topology](../img/scenario-a/noc-topology.png) <!-- TODO: capture screenshot -->

> **Sample data.** The topology shown is illustrative.

---

## 8. Log Viewer

**Route:** `/logs/:componentId`

Opened from a component on the [Infrastructure](#5-infrastructure) page. Streams the
selected component's recent log lines for inspection. There is no direct sidebar
entry — you reach it by clicking a component.

![Screenshot: Scenario A — NOC Log Viewer](../img/scenario-a/noc-log-viewer.png) <!-- TODO: capture screenshot -->

> **Sample data.** Log lines shown are illustrative.

---

## 9. Audit

**Route:** `/audit` · **Sidebar label:** Audit

A record of operational and administrative events for review.

> **Sample data.** Audit entries are illustrative.

---

## 10. Settings

**Route:** `/settings` · **Sidebar label:** Settings

Portal preferences for the current session.

---

## 11. Status reference

| Health state | Intended meaning |
|---|---|
| **Healthy** | Component is up and responding normally. |
| **Degraded** | Component is responding but impaired (latency, partial failures). |
| **Down / Unreachable** | Component is not responding. |

| Relay state | Intended meaning |
|---|---|
| **Active** | Relay is connected and forwarding events. |
| **Stalled / Error** | Relay is not making progress; investigate before assuming a contract bug. |

---

## 12. Troubleshooting

### Health and relay figures never change
- Expected — the portal is on **sample data**. There is no live telemetry feed yet.

### A cross-border settlement is stuck — what do I check here?
- Once live, the **Relays** page is the first place to look: a stalled relay, not a
  contract bug, is the usual cause of a frozen cross-spoke settlement.

### Log Viewer shows nothing
- Open it from a component on the **Infrastructure** page rather than navigating
  directly; in the current build the logs are sample data.
