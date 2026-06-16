# NOC Portal — User Manual (Scenario B)

**Audience:** Network operations / SRE engineers.
**Portal:** Network Operations Center (NOC) — International Hub (Scenario B).

---

> ## Read first — current data status
>
> All data displayed in the NOC Portal is currently **mock (sample) data** served
> from an in-memory fixture. The screens show the intended layout and interaction
> model for infrastructure health, relay status, pool stability, and network
> topology, but no figures are drawn from a live telemetry feed. Telemetry frames
> displayed on the Dashboard are randomly generated in-browser on a 3-second
> timer and are not real measurements.
>
> **Do not use these figures for incident response until live API wiring is
> confirmed.**

---

## Table of Contents

1. [Overview](#1-overview)
2. [Access and login](#2-access-and-login)
3. [Navigation](#3-navigation)
4. [Screens](#4-screens)
   - 4.1 [Dashboard](#41-dashboard)
   - 4.2 [Infrastructure](#42-infrastructure)
   - 4.3 [Relay Status](#43-relay-status)
   - 4.4 [Pool Stability](#44-pool-stability)
   - 4.5 [Topology](#45-topology)
   - 4.6 [Audit](#46-audit)
   - 4.7 [Settings](#47-settings)
5. [Typical workflows](#5-typical-workflows)
6. [Alert reference](#6-alert-reference)
7. [Troubleshooting](#7-troubleshooting)

---

## 1. Overview

The NOC Portal is the monitoring surface for the **International Hub** scenario
(Scenario B). Scenario B uses a **hub-and-spoke** architecture: multiple spoke
networks (one per central bank) connect to a shared Regional Hub network. The
Cacti-based relay bridges events between spoke networks and the Hub.

The NOC Portal gives network operations engineers a unified view of:

- Node and service health across all spoke networks and the Hub
- Cross-network relay latency, proof success rate, and status
- AMM liquidity pool reserve ratios and 70/30 breach alerts
- Network topology: nodes, links, and redundancy flags
- Immutable audit trail of control-plane events

### How Scenario B differs from Scenario A

| Dimension | Scenario A | Scenario B |
|---|---|---|
| Architecture | Peer-to-peer corridors (HTLC) | Hub-and-spoke (Regional Hub + spoke networks) |
| Relay component | Not present | Cacti relay (spoke ↔ Hub) |
| Pool Stability page | Not present | Present — monitors AMM 70/30 reserve ratio |
| Log Viewer page | Present | Not present |
| Privacy tokens | ZetoToken / NotoToken | ZetoToken / NotoToken via Paladin |

---

## 2. Access and Login

**Route:** `/login`

Open the NOC Portal URL provided by your administrator and sign in with your
operational credentials. The login form requires a username (minimum 3
characters) and a password (minimum 6 characters).

<!-- TODO: screenshot — NOC login page -->

Access is RBAC-constrained to the `SYS_ADMIN` role. The portal exposes no
destructive infrastructure controls — it is read-only from an operations plane
perspective.

> **Demo credentials** (local structural testing only): username `noc.admin`,
> password `NOCAdmin2026!`. These are pre-filled for local environments and
> must not be used in any non-local deployment.

After a successful login you are redirected to the Dashboard. If the portal
detects an existing session it will restore it automatically; you will not be
asked to sign in again unless the session has expired or the portal has been
reloaded after a logout.

---

## 3. Navigation

The portal uses a persistent sidebar for primary navigation. All protected
screens require authentication. An unauthenticated request to any protected
route is silently redirected to `/login`.

| Sidebar label | Route | Description |
|---|---|---|
| Dashboard | `/` | Aggregated health overview and live telemetry feed |
| Infrastructure | `/infrastructure` | Node and service health per network |
| Relays | `/relays` | Cross-network relay latency and proof success |
| Pool Stability | `/pool-stability` | AMM pool reserve ratios and 70/30 breach status |
| Topology | `/topology` | Hub-and-spoke topology snapshot |
| Audit | `/audit` | Immutable control-plane audit trail |
| Settings | `/settings` | Notification and polling preferences |

All routes in the table above are active. There is no Log Viewer route in
Scenario B.

---

## 4. Screens

### 4.1 Dashboard

**Route:** `/` — **Sidebar label:** Dashboard

<!-- TODO: screenshot — NOC Dashboard -->

The Dashboard is a single-pane overview that aggregates key signals from all
monitored subsystems. It loads data from Infrastructure, Relay, Pool, and
Telemetry stores on mount.

#### Summary cards

Four summary cards appear at the top of the page:

| Card | What it shows |
|---|---|
| Critical Alerts | Count of active alerts with severity `CRITICAL` |
| Degraded Nodes | Count of infrastructure nodes whose status is not `HEALTHY` |
| Worst Relay p95 | Highest p95 latency (ms) across all monitored relays |
| Pool Breaches (70/30) | Count of AMM pools that have breached the 70/30 reserve threshold |

A value of zero in "Critical Alerts" and "Pool Breaches" and a "Worst Relay p95"
within acceptable bounds indicates normal operating state. Elevated counts
require investigation on the respective detail pages.

#### Recent Telemetry Frames

A table of the six most recent telemetry snapshots, updated on a 3-second
interval by a simulated WebSocket stream. Columns: Component, CPU (%), Memory
(%), Latency (ms).

> **Note:** Telemetry frames are generated in-browser from the mock data layer.
> Frames where CPU or memory exceeds 90% automatically produce a `CRITICAL`
> alert in the Active Alert Feed; frames with `healthy = false` produce a
> `WARNING` alert.

#### Active Alert Feed

Up to eight most recent alerts from all sources (INFRASTRUCTURE, RELAY, POOL,
TELEMETRY). Each entry shows the source, severity badge, and message. Alert
severity values are `INFO`, `WARNING`, and `CRITICAL`.

---

### 4.2 Infrastructure

**Route:** `/infrastructure` — **Sidebar label:** Infrastructure

<!-- TODO: screenshot — NOC Infrastructure page -->

Lists all monitored infrastructure nodes across spoke networks and the Hub, with
their current health state. A **Refresh** button triggers a manual data reload.

> **Sample data.** Node health is illustrative; no live data source is wired.

#### Column reference

| Column | Description |
|---|---|
| Node | Unique node identifier (e.g. `besu-a-1`) |
| Component | Runtime type: `BESU` (Hyperledger Besu node) or `PALADIN` (Paladin Core) |
| Network | The spoke or hub network this node belongs to |
| Region | Geographic region code (e.g. `BR-SP`) |
| Uptime | Rolling uptime percentage |
| Sync Lag | Number of blocks by which this node lags behind the chain head |
| Status | Health badge — see [Status reference](#6-alert-reference) |

A `syncLagBlocks` value greater than zero on a `BESU` node indicates that the
node is not fully synchronised. Combined with a `DEGRADED` status this warrants
investigation of the node's peer connectivity and disk I/O.

---

### 4.3 Relay Status

**Route:** `/relays` — **Sidebar label:** Relays

<!-- TODO: screenshot — NOC Relay Status page -->

Displays the health and performance metrics of each Cacti relay route. Relays
carry events between spoke networks and the Regional Hub — a stalled relay is
the most common cause of a frozen cross-spoke transfer or bridge operation. A
**Refresh** button triggers a manual data reload.

> **Sample data.** Relay states are illustrative.

#### Column reference

| Column | Description |
|---|---|
| Relay | Route identifier in the form `Network X ↔ Hub` or `Network X ↔ Network Y` |
| Status | Health badge: `HEALTHY`, `DEGRADED`, or `DOWN` |
| p50 Latency | Median round-trip latency for relay proof submissions (ms) |
| p95 Latency | 95th-percentile latency — the tail value used by the Dashboard "Worst Relay p95" card |
| Proof Success | Percentage of relay proof submissions that completed successfully |
| Updated At | Timestamp of the last data refresh |

A relay showing `DEGRADED` or `DOWN` with a low proof success rate means events
are not being delivered reliably between networks. Investigate the relay
container logs (`cacti`) before assuming a smart contract issue.

---

### 4.4 Pool Stability

**Route:** `/pool-stability` — **Sidebar label:** Pool Stability

**This page is specific to Scenario B.** It is not present in the Scenario A
NOC Portal.

<!-- TODO: screenshot — NOC Pool Stability page -->

Displays the reserve ratios of each AMM (AutomatedMarketMaker) liquidity pool
and flags pools that have breached the 70/30 threshold. A **Refresh** button
triggers a manual data reload.

> **Sample data.** Pool figures and breach flags are illustrative.

#### Understanding the 70/30 rule

The AMM maintains two-sided reserve pools for each currency pair (e.g.
`BRL-tCeBM/ARS-tCeBM`). The platform enforces a maximum reserve imbalance of
70% / 30% — if either side of the pool drops below 30% of total reserves, the
circuit breaker may halt swaps for that corridor to prevent excessive price
impact on settlement transactions.

#### Column reference

| Column | Description |
|---|---|
| Pool | Currency pair identifier |
| Reserve A | Total units held for token A |
| Reserve B | Total units held for token B |
| Split | Visual progress bar showing `ratioA% / ratioB%` — the current reserve split |
| Status | `STABLE` (within 70/30) or `BREACHED` (threshold exceeded) |
| Updated At | Timestamp of the last pool snapshot |

#### Responding to a BREACHED pool

1. Note which pool pair is flagged and whether it is a single corridor or
   multiple.
2. Cross-reference with Relay Status — if the relay carrying that corridor is
   also degraded, the imbalance may be caused by undelivered settlement events
   rather than organic trading pressure.
3. Escalate to the liquidity operations team with the pool name, current ratios,
   and the time the breach appeared.
4. Do not attempt to rebalance the pool directly from this portal; the portal is
   read-only.

---

### 4.5 Topology

**Route:** `/topology` — **Sidebar label:** Topology

<!-- TODO: screenshot — NOC Topology page -->

Displays a tabular snapshot of the hub-and-spoke network graph from the latest
synchronisation cycle. The page is split into two panels.

> **Sample data.** The topology shown is illustrative.

#### Topology Snapshot panel (left)

Lists all nodes in the network graph.

| Column | Description |
|---|---|
| Node | Human-readable node label (e.g. `Regional Hub`, `Besu A`) |
| Kind | Node type: `HUB`, `BESU`, `PALADIN`, or `CACTI` |
| Redundant | Whether a redundant peer exists for this node (`YES` / `NO`) |
| Status | Health badge: `HEALTHY`, `DEGRADED`, or `DOWN` |

Nodes flagged `redundant: NO` are single points of failure. A `DEGRADED` or
`DOWN` status on a non-redundant node requires immediate escalation.

#### Topology Health panel (right)

A summary sidebar showing:

| Metric | Description |
|---|---|
| Total Nodes | Count of all nodes in the snapshot |
| Total Links | Count of all edges (connections) between nodes |
| Healthy Links | Count of edges flagged as `healthy = true` |

A gap between Total Links and Healthy Links indicates one or more broken
connections in the topology. Cross-reference with the Relay Status page to
identify which relay corresponds to the unhealthy edge.

---

### 4.6 Audit

**Route:** `/audit` — **Sidebar label:** Audit

<!-- TODO: screenshot — NOC Audit page -->

An immutable trail of operator actions and control-plane decisions emitted by
the monitored components. Records are loaded on mount and can be filtered
in-browser using the search field.

> **Sample data.** Audit records are illustrative.

#### Filtering

Type any term into the search field to filter records by component name,
message text, or severity. The filter is applied client-side against the
currently loaded dataset.

#### Column reference

| Column | Description |
|---|---|
| Timestamp | Local datetime when the event was recorded |
| Component | Emitting component: `BESU`, `PALADIN`, `CACTI`, or `SYSTEM` |
| Severity | `INFO`, `WARNING`, or `CRITICAL` |
| Message | Free-text description of the event |

Use the Audit page to confirm that a relay restart, node configuration change,
or other control-plane action was recorded correctly. The audit trail cannot be
edited or deleted from the portal.

---

### 4.7 Settings

**Route:** `/settings` — **Sidebar label:** Settings

<!-- TODO: screenshot — NOC Settings page -->

Configures notification and polling preferences for the current session.
Settings are applied immediately on save and are local to the browser session.

| Setting | Description |
|---|---|
| Alert Email | Email address to which alert notifications will be sent |
| Critical Alerts Only | When checked, notifications are sent only for `CRITICAL` severity events |
| Polling Interval (seconds) | How frequently the portal polls for updated data (minimum 1 second) |

Click **Save Settings** to apply. A toast confirmation appears on success.

> Settings persistence across sessions depends on backend wiring that is not yet
> implemented. Changes made here may not survive a page reload in the current
> build.

---

## 5. Typical Workflows

### Investigating a stuck cross-spoke transfer

1. Open **Relay Status** and identify the relay route for the affected corridor
   (e.g. `Network A ↔ Hub` or `Network B ↔ Hub`).
2. Check the relay's **Status**, **p95 Latency**, and **Proof Success** rate. A
   degraded proof success rate or high p95 latency confirms the relay is the
   bottleneck.
3. If the relay appears healthy, open **Pool Stability** for the affected currency
   pair. A `BREACHED` pool may cause settlement to fail on price impact rather
   than freezing.
4. Check the **Topology** panel for unhealthy links on the affected corridor.
5. Review the **Audit** trail filtered on `CACTI` or the affected network for
   recent control-plane events.
6. Escalate with: relay route, proof success rate, p95 latency, pool status, and
   any relevant audit entries.

### Checking node sync health after a deployment

1. Open **Infrastructure** and click **Refresh**.
2. Sort mentally by **Sync Lag** — nodes with non-zero sync lag are not at chain
   head.
3. Nodes that are `DEGRADED` and have a sync lag of more than a few blocks should
   be investigated for peer connectivity or disk issues.
4. Confirm in **Topology** that the affected node's links are still healthy.

### Monitoring pool stability before a high-volume settlement window

1. Open **Pool Stability** and click **Refresh**.
2. Review the **Split** column for each pool that will be used in the settlement
   window.
3. Alert the liquidity operations team if any pool's ratio is approaching 70/30
   (e.g. A ratio above 65% or below 35%) even if not yet flagged `BREACHED`.
4. Monitor the **Dashboard** "Pool Breaches (70/30)" card during the window for
   any breach that occurs mid-flight.

---

## 6. Alert Reference

### Infrastructure node status

| Status | Meaning |
|---|---|
| `HEALTHY` | Node is up, synced, and responding normally |
| `DEGRADED` | Node is responding but impaired — elevated sync lag or reduced uptime |
| `DOWN` | Node is not responding |

### Relay status

| Status | Meaning |
|---|---|
| `HEALTHY` | Relay is connected and forwarding events with acceptable latency and proof success |
| `DEGRADED` | Relay is operational but showing elevated latency or reduced proof success rate |
| `DOWN` | Relay is not forwarding events |

### Pool stability status

| Status | Meaning |
|---|---|
| `STABLE` | Pool reserves are within the 70/30 tolerance |
| `BREACHED` | One side of the pool has dropped below 30% of total reserves; transfers on this corridor may fail on price impact |

### Topology node and link status

| Value | Meaning |
|---|---|
| `HEALTHY` | Node or link is operating normally |
| `DEGRADED` | Node or link is impaired but not fully down |
| `DOWN` | Node or link is not operational |
| `redundant: NO` | No failover peer exists — this node is a single point of failure |

### Alert severity

| Severity | Meaning |
|---|---|
| `INFO` | Informational event; no action required |
| `WARNING` | Condition warrants monitoring; may escalate |
| `CRITICAL` | Immediate investigation required |

Alert sources: `INFRASTRUCTURE`, `RELAY`, `POOL`, `TELEMETRY`.

---

## 7. Troubleshooting

### All figures are static and never update

Expected in the current build. All data is served from an in-memory mock. There
is no live telemetry backend wired. The only live-style data is the Dashboard
telemetry feed, which generates frames in-browser every three seconds using
random values.

### The telemetry feed on the Dashboard stops updating

The simulated telemetry service randomly drops its connection (approximately 3%
chance per tick). The portal automatically reconnects with exponential backoff
starting at two seconds and capping at 15 seconds. If the feed does not resume
within a minute, reload the page.

### Relay Status shows a relay as DEGRADED but transfers appear to be working

In the current build the relay state is mock data and does not reflect actual
relay health. Once live telemetry is wired, a `DEGRADED` relay with a proof
success rate above 95% typically means elevated latency rather than message
loss, and transfers may still complete with added delay.

### Pool Stability shows STABLE during a known AMM incident

Expected in the current build — pool data is illustrative, not live. Once live
data is connected, confirm the pool pair, note the reserve ratios, and escalate
to the liquidity operations team if the ratio is outside tolerance.

### Login fails with "Invalid credentials"

Verify that the username is at least 3 characters and the password is at least
6 characters. In local environments the form is pre-filled with demo credentials
(`noc.admin` / `NOCAdmin2026!`). If credentials are correct and login still
fails, check that the auth service is reachable and that the Keycloak OIDC
provider is running (`make deploy.up-infra` from `scenario-b/`).

### A page shows no data after navigating to it

Each page fetches data on mount. If the fetch is in a loading state the table
will appear empty. Click **Refresh** (where available) or reload the page. If
the problem persists, check browser console logs for network errors.
