# NOC Monitoring Portal — Operator Guide

This guide covers the operation of the **Network Operations Center (NOC) Portal**: monitoring infrastructure health, managing alerts, viewing container logs, and reviewing operator audit trails.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Prerequisites](#2-prerequisites)
3. [Access and Login](#3-access-and-login)
4. [Dashboard](#4-dashboard)
5. [Infrastructure](#5-infrastructure)
6. [Relays](#6-relays)
7. [Topology](#7-topology)
8. [Log Viewer](#8-log-viewer)
9. [Audit](#9-audit)
10. [Settings](#10-settings)
11. [Alert Management](#11-alert-management)
12. [Status Reference](#12-status-reference)
13. [Troubleshooting](#13-troubleshooting)

---

## 1. Overview

The NOC Portal is a **read-only infrastructure monitoring interface** restricted to system administrators (`SYS_ADMIN` role). It provides live visibility into the health of all spoke components, active alert feeds, container logs, and a network topology diagram.

> The NOC Portal does not expose any destructive controls. NOC operators cannot issue tokens, approve transactions, or modify system configuration. The only write operations available are alert acknowledgement and dismissal.

| Actor | Role | Access |
|---|---|---|
| NOC System Administrator | Infrastructure monitoring, alert triage, log inspection | SYS_ADMIN credentials required |

### Key Capabilities

| Capability | Section |
|---|---|
| Real-time component health overview | [Section 4](#4-dashboard) and [Section 5](#5-infrastructure) |
| Monitor interoperability relay health | [Section 6](#6-relays) |
| Visualise network topology | [Section 7](#7-topology) |
| Inspect container logs | [Section 8](#8-log-viewer) |
| Review operator action history | [Section 9](#9-audit) |
| Acknowledge and dismiss alerts | [Section 11](#11-alert-management) |

---

## 2. Prerequisites

> **Initial test environment:** For the initial testing phase, credentials and access URLs will be provided by the system administrator. The test setup consists of **2 spokes**, each containing three institutions: one Central Bank, one Commercial Bank, and one Financial Correspondent.

Before accessing the portal:

- The NOC backend service is running and reachable.
- The operator has received a **username** and **password** with the `SYS_ADMIN` role from the system administrator.
- The operator has been provided with the **dispatcher URL** — the single entry point that automatically routes each user to the correct portal based on their credentials.

---

## 3. Access and Login

Open the **dispatcher URL** in your browser. Enter your credentials and the dispatcher will automatically redirect you to the NOC Portal.

> ![NOC Login Screen](./screenshots/noc/01-noc-login.png)

The login page displays an information panel (on desktop) describing the portal's key features and access restrictions.

| Field | Description |
|---|---|
| **Username** | Your `SYS_ADMIN` username. |
| **Password** | Your `SYS_ADMIN` password. |

Click **Sign in**. On success, you are redirected to the Dashboard.

> Access is restricted to `SYS_ADMIN` accounts only. Standard bank or governance operator credentials will not work.

---

## 4. Dashboard

**Route:** `/`

The Dashboard is the primary monitoring view. It provides an at-a-glance status of the entire platform — across all spokes — with live alerts and a component health summary.

> ![NOC Dashboard](./screenshots/noc/02-noc-dashboard.png)

### Spoke Selector

A dropdown at the top of the page lets you filter the Dashboard view to a specific spoke or view data across **all spokes** simultaneously.

### Summary Cards

Four cards provide an instant platform-wide health snapshot:

| Card | Colour When Non-Zero | Description |
|---|---|---|
| **Critical / High Alerts** | Red | Number of active alerts with severity HIGH or CRITICAL. |
| **Degraded Components** | Orange | Number of components currently in DEGRADED state. |
| **Offline Components** | Red | Number of components currently OFFLINE. |
| **Total Spokes** | — | Total number of spokes registered in the system. |

A non-zero count on Critical/High Alerts or Offline Components requires immediate attention.

### Component Health Table

Displays the first 8 components across the selected scope with their **name**, **type**, and a **health status badge**. For a full list, navigate to the [Infrastructure page](#5-infrastructure).

### Active Alert Feed

Lists up to 8 active alerts in real time:

| Element | Description |
|---|---|
| **Title** | Short description of the alert condition. |
| **Severity badge** | `INFO`, `WARNING`, `HIGH`, or `CRITICAL`. |
| **ACK badge** | Shown if the alert has been acknowledged by an operator. |
| **Dismiss button** | Click to dismiss the alert directly from the feed (logged in audit). |

Click any alert row to open the **Alert Detail Modal**.

> ![Alert Detail Modal](./screenshots/noc/03-alert-detail-modal.png)

The Alert Detail Modal displays the full alert context: title, severity, affected component, description, and first-detected timestamp. From this modal you can **Acknowledge** or **Dismiss** the alert (see [Section 11](#11-alert-management)).

### Polling

The Dashboard data refreshes automatically at the interval configured in [Settings](#10-settings) (minimum 5 seconds). The current polling state is shown in the top status bar.

---

## 5. Infrastructure

**Route:** `/infrastructure`

The Infrastructure page shows the health status of all non-relay components across spokes: blockchain nodes, Paladin nodes, and payment orchestrators.

> ![Infrastructure Page](./screenshots/noc/04-infrastructure-page.png)

### Controls

| Element | Description |
|---|---|
| **Spoke Selector** | Filter the component list to a specific spoke. |
| **Refresh button** | Force an immediate data refresh with a loading indicator. |

### Component Health Table

Each row represents one infrastructure component:

| Column | Description |
|---|---|
| **Name** | The component's display name. |
| **Type** | Component category: `BESU`, `PALADIN`, or `PAYMENT_ORCHESTRATOR`. |
| **Endpoint URL** | The component's API or RPC endpoint (truncated). |
| **Block #** | The latest block number (populated for BESU blockchain nodes only). |
| **Last Check** | Timestamp of the most recent health poll. |
| **Status** | Current health status badge (see [Status Reference](#12-status-reference)). |
| **Action** | **View Logs** button — opens the [Log Viewer](#8-log-viewer) for this component. |

> CACTI relay components are excluded from this table. To monitor relays, use the [Relays page](#6-relays).

---

## 6. Relays

**Route:** `/relays`

The Relays page monitors the health of **CACTI interoperability relay containers** — the components responsible for cross-spoke communication.

> ![Relays Page](./screenshots/noc/05-relays-page.png)

### Controls

| Element | Description |
|---|---|
| **Spoke Selector** | You must select a specific spoke to display relay data. The table is empty until a spoke is selected. |

### Relay Components Table

The table shows only `CACTI_RELAY` type components for the selected spoke. Columns are identical to the [Infrastructure page](#5-infrastructure) table:

| Column | Description |
|---|---|
| **Name** | Relay container name. |
| **Type** | Always `CACTI_RELAY`. |
| **Endpoint URL** | The relay container's endpoint. |
| **Last Check** | Timestamp of the last health poll. |
| **Status** | Current health status badge. |
| **Action** | **View Logs** button — opens the Log Viewer for this relay. |

---

## 7. Topology

**Route:** `/topology`

The Topology page renders an **interactive network diagram** visualising the relationships between all nodes and links across the platform.

> ![Topology Page](./screenshots/noc/06-topology-page.png)

### Diagram Elements

| Element | Colour | Description |
|---|---|---|
| **BESU node** | Blue | A blockchain node (Hyperledger Besu). |
| **PALADIN node** | Purple | A Paladin privacy node. |
| **CACTI relay node** | Orange | An interoperability relay container. |
| **Green edge** | Green | A healthy link between two nodes. |
| **Red edge** | Red | An unhealthy link between two nodes. |

Each node displays a **health status badge** and the name of its spoke.

### Diagram Controls

| Control | Action |
|---|---|
| Scroll | Zoom in / out. |
| Click and drag (canvas) | Pan the diagram. |
| Mini-map (bottom-right) | Navigate quickly across a large topology. |

### Topology Health Summary Card

Located alongside the diagram, this card shows:

| Metric | Description |
|---|---|
| **Total Nodes** | Number of nodes in the topology. |
| **Total Links** | Number of edges between nodes. |
| **Healthy Links** | Number of links with a green (healthy) state. |
| **Unhealthy Links** | Number of links with a red (unhealthy) state. |

A colour legend identifies each node type.

---

## 8. Log Viewer

**Route:** `/logs/:componentId`

The Log Viewer displays **real-time container logs** for any infrastructure or relay component. It is accessed by clicking the **View Logs** button on the Infrastructure or Relays pages.

> ![Log Viewer Page](./screenshots/noc/07-log-viewer.png)

### Page Elements

| Element | Description |
|---|---|
| **Component Name** | Displayed at the top — identifies which component's logs are being shown. |
| **Refresh button** | Force an immediate log fetch; shows a loading spinner during the request. |
| **Log panel** | Terminal-style viewer (black background, green text). |

### Log Panel

| Column | Description |
|---|---|
| **Timestamp** | ISO timestamp of the log line. |
| **Stream badge** | `stdout` (normal output) or `stderr` (error output). `stderr` lines are highlighted in red. |
| **Log line** | The raw log text from the container. |

The panel displays the **last 500 lines** and auto-scrolls to the bottom on load and on each refresh. Logs auto-refresh every **15 seconds** automatically.

---

## 9. Audit

**Route:** `/audit`

The Audit page provides an immutable record of all operator actions performed within the NOC Portal, plus a history of resolved alerts.

> ![Audit Page](./screenshots/noc/08-audit-page.png)

### Operator Actions Table

Every alert acknowledgement and dismissal performed by any NOC operator is recorded here.

| Column | Description |
|---|---|
| **Timestamp** | When the action was performed. |
| **Actor** | The username of the NOC operator who performed the action. |
| **Action** | The type of action: `ACKNOWLEDGE_ALERT` or `DISMISS_ALERT`. |
| **Target ID** | The identifier of the alert that was acted upon. |
| **Detail** | Additional context provided at the time of the action. |

Use the **search input** at the top to filter by actor, action type, target, or detail text.

### Resolved Alerts Table

A searchable history of alerts that have been automatically or manually resolved:

| Column | Description |
|---|---|
| **Resolved At** | Timestamp when the alert was resolved. |
| **Title** | The alert title. |
| **Severity** | The alert's severity badge at the time of resolution. |
| **Root Cause Signature** | A system-generated identifier for the root cause pattern. |

---

## 10. Settings

**Route:** `/settings`

The Settings page allows each NOC operator to configure their personal monitoring preferences.

> ![Settings Page](./screenshots/noc/09-settings-page.png)

| Setting | Description |
|---|---|
| **Alert Email** | Email address to receive alert notifications. Stored locally per operator session. |
| **Critical Alerts Only** | When checked, `INFO` and `WARNING` severity alerts are muted from the Dashboard feed. Only `HIGH` and `CRITICAL` alerts are shown. |
| **Polling Interval** | How frequently the Dashboard auto-refreshes (in seconds). Minimum value: **5 seconds**. |

Click **Save** to apply the settings.

> Settings are stored in the browser session and persist across page navigation within the same session. They are reset when you log out.

---

## 11. Alert Management

NOC operators can take two actions on active alerts: **Acknowledge** and **Dismiss**.

### Difference Between Acknowledge and Dismiss

| Action | Meaning | Effect on Alert |
|---|---|---|
| **Acknowledge** | An operator has seen the alert and is investigating. | Alert remains active in the feed; an ACK badge appears on the alert row. Action logged in Audit. |
| **Dismiss** | The alert is not actionable or has been resolved externally. | Alert is removed from the active feed. Action logged in Audit. |

### How to Acknowledge an Alert

1. On the Dashboard, locate the alert in the **Active Alert Feed**.
2. Click the alert row to open the **Alert Detail Modal**.
3. Review the alert title, severity, affected component, and description.
4. Click **Acknowledge**.

The ACK badge appears on the alert row in the feed, and the action is recorded in the [Audit page](#9-audit).

### How to Dismiss an Alert

**Option A — From the Dashboard feed:**

1. Click the **dismiss button** (X icon) on the right side of the alert row.
2. Confirm the dismissal in the dialog.

**Option B — From the Alert Detail Modal:**

1. Click the alert row to open the **Alert Detail Modal**.
2. Click **Dismiss**.

In both cases, the alert is removed from the active feed and the action is logged in Audit.

---

## 12. Status Reference

### Component Health Statuses

| Status | Meaning |
|---|---|
| `HEALTHY` | The component is responding correctly to health checks. |
| `DEGRADED` | The component is responding but with errors or elevated latency. Investigate promptly. |
| `OFFLINE` | The component is not responding to health checks. Requires immediate attention. |
| `UNKNOWN` | Health check has not been performed yet or the last result could not be determined. |

### Alert Severities

| Severity | Meaning |
|---|---|
| `INFO` | Informational event; no action required. |
| `WARNING` | Potential issue detected; monitor the situation. |
| `HIGH` | Significant problem affecting the platform; investigate as soon as possible. |
| `CRITICAL` | Severe failure requiring immediate action. |

---

## 13. Troubleshooting

### Login fails

- Verify the username and password are correct for a `SYS_ADMIN` account.
- Confirm the NOC backend service is running and reachable.
- Standard bank or governance portal credentials cannot be used here.

### Dashboard shows no data or all components show `UNKNOWN`

- Check the NOC backend service logs for connectivity issues.
- Verify the spoke services are running (`docker compose ps`).
- Try clicking the **Refresh** button on the Infrastructure page to force a manual poll.

### Relay page shows an empty table

- A spoke must be selected in the **Spoke Selector** dropdown before relay data is displayed. Select a specific spoke and the table will populate.

### Log Viewer shows no logs or an error

- Verify the component is running and its endpoint is reachable from the NOC backend.
- Try clicking the **Refresh** button to force a new log fetch.
- Some components may produce minimal stdout output — check the `stderr` stream for error messages.

### Alerts are not updating

- Check the **Polling Interval** setting in [Settings](#10-settings). If set too high, new alerts will appear infrequently.
- Verify the WebSocket connection status indicator in the top status bar. If disconnected, the portal falls back to HTTP polling automatically, but with a higher latency.
- Click any **Refresh** button to trigger an immediate data pull.

### Alert dismissed by mistake

- Dismissed alerts are moved to the **Resolved Alerts** table on the [Audit page](#9-audit) and cannot be re-activated from the portal.
- Contact the system administrator if the alert needs to be re-raised.
