# Feature Specification: NOC Monitoring Service

**Feature Branch**: `feature/noc-monitoring-service`  
**Created**: 2025-07-15  
**Status**: Draft  
**Directory**: `specs/002-noc-monitoring-service`

## Overview

The NOC Monitoring Service is a spoke-agnostic backend engine that powers the Network Operations Center (NOC) portal for the cbweb3-platform. It centralizes real-time health monitoring, alert management, SLO tracking, and operational tooling for all participants — central banks, commercial banks, and platform operators — across all connected spoke networks.

The service must be deployable independently of any specific spoke configuration, meaning it adapts dynamically to whichever spokes (e.g., Spoke-A/BRL, Spoke-B/ARS) and hub topology are registered, without code changes.

The system follows a **two-component architecture**:
- **NOC Agent**: A lightweight collector process deployed co-located with each spoke/hub server. It monitors local Besu nodes, Cacti relayers, and Paladin via their local APIs (Docker socket, Besu RPC, Cacti REST), and **pushes** metrics and events to the NOC Backend. One agent per spoke/hub.
- **NOC Backend**: A central aggregation service that receives data from all agents, runs the alert engine, computes SLO metrics, stores audit logs, and exposes a REST API consumed by the NOC Frontend.

---

## Clarifications

### Session 2026-05-22

- Q: How does the NOC Agent communicate with the NOC Backend? → A: Push — the agent sends metrics and events periodically to the NOC Backend endpoint.
- Q: When a NOC Agent stops sending data, what status do its spoke components get in the backend? → A: Grace period + UNKNOWN — components retain last known status for a configurable grace period (default: 3× push interval), then transition to `UNKNOWN`; a separate "agent unreachable" alert is raised.
- Q: How does the NOC Backend authenticate incoming pushes from NOC Agents? → A: API key per agent — each agent is configured with a unique static API key sent as `X-Agent-Key` header; the backend rejects pushes with unknown or missing keys.
- Q: What language/runtime should the NOC Agent be implemented in? → A: Go — produces a lightweight static binary suitable for co-location with Besu and Cacti; consistent with the `payment-orchestrator` stack.
- Q: What is the NOC Backend's database strategy? → A: Dedicated PostgreSQL instance (`noc-db`) — isolated container to prevent metric write load from interfering with the platform's transactional banking databases; uses GORM consistent with the platform standard.
- Q: What is the NOC portal operator UX navigation model? → A: Global overview first, drill-down on demand — operator lands on a network-wide dashboard showing all spokes/hub with traffic-light status; clicks into a spoke for component detail; clicks into a component for history, alerts, and logs. No upfront spoke selection required.
- Q: How is the NOC Agent configured to know which components to monitor? → A: Static `agent.yaml` file co-deployed with each spoke — declares `spoke_id`, `noc_backend_url`, `api_key`, and a list of components each with a `name`, `type` (BESU / CACTI_RELAY / PALADIN), and `endpoint`. Each type uses a distinct health-check strategy (Besu: `eth_blockNumber` JSON-RPC; Cacti: `GET /api/v1/health`; Paladin: `GET /health`).
- Q: How are NOC portal users authenticated and managed? → A: Keycloak integration with NOC-specific roles — the NOC portal is a Keycloak client; users are managed in Keycloak (not in the NOC service itself); three roles: `noc-viewer` (read-only), `noc-operator` (can acknowledge/resolve alerts), `noc-admin` (can register/remove spokes).

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Real-Time Network Health Dashboard (Priority: P1)

A platform operator or NOC engineer opens the NOC portal and immediately sees a live health dashboard showing the current status of every node and relayer across all registered spokes and the hub. Without navigating away, they can confirm which components are healthy, degraded, or offline.

**Why this priority**: This is the foundational capability. Without real-time health visibility, all other monitoring and support functions are blind. It delivers the highest immediate operational value and is the prerequisite for all alert-based and SLO workflows.

**Independent Test**: Can be fully tested by provisioning a multi-spoke environment with at least two spokes, deliberately stopping one node, and verifying the dashboard reflects the failure within the defined refresh interval — delivering an independently valuable health overview with no other features required.

**Acceptance Scenarios**:

1. **Given** the NOC portal is open, **When** a Besu node on any registered spoke loses connectivity, **Then** the dashboard marks that node as `OFFLINE` within 30 seconds and displays its last-seen timestamp.
2. **Given** all nodes and relayers are operating normally, **When** an operator views the health dashboard, **Then** every component shows a `HEALTHY` status with no manual refresh required.
3. **Given** a Cacti relayer process restarts, **When** it re-establishes its gRPC connections, **Then** the dashboard transitions that relayer from `DEGRADED` to `HEALTHY` automatically.
4. **Given** the monitoring service has never seen a newly registered spoke, **When** the spoke is added to the platform configuration, **Then** its nodes and relayers appear in the dashboard without requiring a service restart.

---

### User Story 2 — Alert and Incident Centralization (Priority: P2)

A support engineer receives a notification that a cross-border transaction has failed. They open the NOC portal's alert panel and see a consolidated view of all active incidents — API errors, synchronization failures between spokes, and Paladin privacy service interruptions — with enough context to begin triage without switching tools.

**Why this priority**: Alert centralization directly reduces mean time to detect (MTTD) and mean time to respond (MTTR) for incidents. It transforms isolated failure signals into actionable incidents for L1/L2 support teams.

**Independent Test**: Can be fully tested by injecting a synthetic API error and a simulated synchronization failure, then verifying both appear as distinct alerts in the alert panel with correct severity, timestamps, and affected component labels.

**Acceptance Scenarios**:

1. **Given** a Cacti relayer reports a failed HTLC synchronization event, **When** the monitoring service processes this event, **Then** an incident of severity `HIGH` is created, linked to the affected spoke pair, and visible in the alert panel within 60 seconds.
2. **Given** the Paladin privacy service on a spoke becomes unresponsive, **When** the health check detects the failure, **Then** an alert is raised with severity `CRITICAL` and a human-readable description of the affected capability.
3. **Given** an alert is resolved (the underlying condition clears), **When** the monitoring service detects recovery, **Then** the alert transitions to `RESOLVED` state with a resolution timestamp and remains visible in the incident history.
4. **Given** multiple simultaneous alerts from the same component, **When** they share the same root cause signature, **Then** they are grouped under a single incident rather than creating duplicate noise.

---

### User Story 3 — SLO Metrics and Performance Tracking (Priority: P3)

A central bank technical team reviews the weekly performance of their spoke's infrastructure. They access the SLO dashboard and see current availability percentages, transaction latency distributions, and whether the system is operating within agreed service limits — enabling evidence-based conversations with the platform operator.

**Why this priority**: SLO visibility is required for regulatory accountability and contractual obligations between central banks and the platform. It is lower priority than alerting because it is retrospective and not time-critical for immediate incident response.

**Independent Test**: Can be fully tested by feeding 7 days of synthetic metric samples into the service and verifying that the SLO dashboard correctly computes availability percentage and highlights any SLO breach periods.

**Acceptance Scenarios**:

1. **Given** a spoke network has been operating for at least 24 hours, **When** a user views the SLO dashboard for that spoke, **Then** availability percentage and p95/p99 latency metrics are displayed for configurable time windows (last 24h, 7d, 30d).
2. **Given** the system's transaction processing latency exceeds the configured SLO threshold, **When** the breach is detected, **Then** the SLO panel visually highlights the breach period and the duration it was out of compliance.
3. **Given** a user selects a specific time window with an SLO breach, **When** they expand that breach event, **Then** they can see which component (node, relayer, or Paladin service) contributed most to the degradation.

---

### User Story 4 — Transaction Log Search and Audit (Priority: P4)

An L2 support engineer is investigating a disputed cross-border transaction. They search the audit log by transaction ID and retrieve a chronological sequence of all events — from initiation through HTLC locking to final settlement — with enough detail to identify exactly where and why the transaction deviated from the expected flow.

**Why this priority**: Audit log access is essential for post-incident analysis and compliance, but it does not affect live operations and can be delivered after the core monitoring and alerting capabilities.

**Independent Test**: Can be fully tested by querying for a known transaction ID and verifying that all associated events are returned in correct chronological order with relevant component labels and timestamps.

**Acceptance Scenarios**:

1. **Given** a support engineer searches for a transaction by its unique identifier, **When** the query executes, **Then** all events associated with that transaction across all participating spokes are returned in ascending chronological order within 5 seconds.
2. **Given** a search query includes a time range filter, **When** results are returned, **Then** only events within the specified time range are included.
3. **Given** an audit event contains an error condition, **When** the event is displayed, **Then** the error code, affected component, and a human-readable description are all visible.
4. **Given** a user needs to export audit records for a regulatory report, **When** they request an export for a given time range, **Then** the records are available for download in a structured, machine-readable format.

---

### User Story 5 — L1/L2 Support Interface *(Deferred — out of scope for v1)*

> **Deferred**: Initial L1/L2 support coordination will be handled via email. This user story and its associated functional requirements (FR-021, FR-011) are explicitly out of scope for v1. The participant-scoped query API (FR-021) may be revisited in a future iteration once the core monitoring, alerting, SLO, and audit features are stable.

~~An L1 support agent at a commercial bank contacts the platform support team about a failed transaction. The L2 engineer uses the NOC portal's support interface to quickly look up the bank's node status, recent alerts, and transaction history in a single view — without needing direct access to the underlying infrastructure or multiple separate tools.~~

---

### Edge Cases

- What happens when the monitoring service cannot reach a node's health endpoint — does it distinguish between a transient network blip and a persistent outage before raising an alert?
- How does the system behave when a new spoke is registered mid-operation — are existing monitors unaffected and new spoke monitors started without a restart?
- What happens if the monitoring service itself loses connectivity to its data store — does it buffer events locally and replay them upon reconnection, or drop them?
- How are alerts deduplicated when the same failure triggers multiple independent detection paths (e.g., a node failure detected via both a health check and a missing block event)?
- What happens when clock drift between nodes causes event timestamps to appear out of order in the audit log?
- How does the service behave when a spoke is decommissioned — are its historical records preserved and its live monitors gracefully shut down?
- **When a NOC Agent becomes unreachable, component statuses remain at their last known value for the grace period (3× push interval), then transition to `UNKNOWN`. A distinct "agent unreachable" alert is raised — separate from component-level `OFFLINE` alerts — so operators can distinguish a monitoring gap from a genuine node failure.**

---

## Requirements *(mandatory)*

### Functional Requirements

#### Network Health Monitoring

- **FR-001**: Each **NOC Agent** MUST continuously collect the health status of all local spoke components (Besu nodes, Cacti relayer, Paladin service) at a configurable interval (default: 15 seconds) and push the results to the NOC Backend.
- **FR-002**: The NOC Backend MUST expose a unified health status model for any registered spoke, hub, node, or relay component — without requiring spoke-specific code.
- **FR-003**: The NOC Backend MUST detect when a NOC Agent stops sending data. After a configurable grace period (default: 3× the agent's push interval), all components associated with that agent MUST transition to `UNKNOWN` status and a `HIGH`-severity "agent unreachable" alert MUST be generated.
- **FR-004**: The service MUST automatically re-evaluate component health and update status when a previously offline or UNKNOWN component becomes reachable again.
- **FR-005**: The service MUST support dynamic registration of new spokes and their components without requiring a service restart.

#### Alert and Incident Management

- **FR-006**: The service MUST generate an alert when any monitored component transitions to a degraded or offline state, including: spoke nodes, Cacti relayers, and the Paladin privacy service.
- **FR-007**: The service MUST classify alerts by severity: `INFO`, `WARNING`, `HIGH`, and `CRITICAL`, based on configurable rules tied to the type and duration of the failure.
- **FR-008**: The service MUST group alerts sharing the same root cause signature into a single incident to prevent duplicate notifications.
- **FR-009**: The service MUST automatically resolve an alert and record a resolution timestamp when the underlying failure condition clears.
- **FR-010**: The service MUST retain a historical record of all alerts and incidents, including state transitions and resolution notes, for a minimum of 90 days.
- ~~**FR-011**: Support engineers MUST be able to add notes to active incidents via the API, with each note persisted alongside author identity and timestamp.~~ *(Deferred — out of scope for v1; support handled via email)*

#### SLO Tracking

- **FR-012**: The service MUST compute and expose availability percentage metrics per spoke component for configurable rolling windows: 24 hours, 7 days, and 30 days.
- **FR-013**: The service MUST compute and expose transaction latency percentiles (p50, p95, p99) per spoke for the same configurable time windows.
- **FR-014**: The service MUST detect and record SLO breach periods when availability or latency metrics fall outside configured thresholds.
- **FR-015**: The service MUST attribute SLO breaches to the specific component responsible for the degradation.

#### Audit Log and Transaction Visibility

- **FR-016**: The service MUST ingest and store transaction lifecycle events from all registered spoke networks, including initiation, HTLC locking, cross-spoke relay, and settlement events.
- **FR-017**: Users MUST be able to search transaction events by transaction identifier, returning all associated events in ascending chronological order.
- **FR-018**: Users MUST be able to filter transaction events by time range, participant, spoke, and event type.
- **FR-019**: The service MUST expose audit records in a structured, exportable format suitable for regulatory reporting.
- **FR-020**: Audit log queries MUST return results within 5 seconds for time ranges up to 30 days.

#### Support Interface *(Deferred — out of scope for v1)*

- ~~**FR-021**: The service MUST expose a participant-scoped query endpoint that returns the current health status, active alerts, and recent transaction events for a given participant (bank or central bank).~~ *(Deferred)*
- **FR-022**: The service API MUST be usable by the existing NOC frontend portal at `frontend/apps/noc/` without additional middleware.
- **FR-023**: The service MUST enforce access control so that participant-scoped queries only return data relevant to the requesting participant's spoke.
- **FR-026**: Each NOC Agent MUST authenticate all push requests to the NOC Backend using a unique, per-agent API key transmitted via the `X-Agent-Key` HTTP header. The NOC Backend MUST reject any push request with an unknown or missing key with HTTP 401.

#### Access Control & Authentication

- **FR-027**: The NOC Backend MUST validate a Keycloak-issued JWT on every API request from the NOC Frontend. Requests without a valid token MUST be rejected with HTTP 401.
- **FR-028**: The NOC Backend MUST enforce role-based access: `noc-viewer` has read-only access; `noc-operator` may additionally acknowledge and resolve alerts; `noc-admin` may additionally register and deregister spokes in the spoke registry.

#### NOC Agent Configuration

- **FR-029**: Each NOC Agent MUST be configured via a static `agent.yaml` file that declares: `spoke_id`, `noc_backend_url`, `api_key`, `push_interval_seconds`, and a list of components each with a `name`, `type`, and `endpoint`.
- **FR-030**: The NOC Agent MUST support the following component types, each with a distinct health-check strategy: `BESU` (calls `eth_blockNumber` via JSON-RPC and validates block progression), `CACTI_RELAY` (calls `GET /api/v1/health`, validates HTTP 200 and `{"status":"ok"}` in response body), `PALADIN` (posts `{"jsonrpc":"2.0","id":1,"method":"ptx_getTransaction","params":["dummy"]}` to HTTP RPC endpoint on port 8548, validates that response contains error code `PD020704` indicating JSON-RPC server is ready).

#### Container Log Visibility

- **FR-031**: The NOC Agent MUST collect the most recent log lines from each monitored Docker container via the Docker socket (`/var/run/docker.sock`) and include them in each push payload to the NOC Backend (default: last 200 lines per component per push cycle).
- **FR-032**: The NOC Backend MUST persist received container logs in a rolling buffer per component (default retention: last 1,000 lines per component), overwriting the oldest entries when the buffer is full.
- **FR-033**: NOC operators (role `noc-viewer` or above) MUST be able to retrieve the most recent log lines for any monitored component via the API, with optional filtering by keyword and time range.
- **FR-034**: When a component transitions to `OFFLINE` or `UNKNOWN`, the NOC Backend MUST preserve a snapshot of the last 500 log lines from that component at the time of the state change, retained for 90 days independently of the rolling buffer.

#### Spoke-Agnostic Architecture

- **FR-024**: The service MUST be configurable via a spoke registry (e.g., a configuration file or dynamic registration API) that lists all active spokes, their components, and their connection endpoints.
- **FR-025**: The service MUST apply the same monitoring, alerting, and SLO logic uniformly to any spoke registered in the spoke registry, regardless of the spoke's currency or jurisdiction.

---

### Key Entities

- **NOCAgent**: A collector process co-located with a spoke/hub server. Has an identifier, assigned spoke, last-seen timestamp, push interval, current reachability status as observed by the NOC Backend, and a unique API key used to authenticate push requests.
- **Spoke**: A registered blockchain network instance (e.g., Spoke-A/BRL, Spoke-B/ARS) with an identifier, currency code, jurisdiction, and a list of associated components.
- **Component**: A monitorable infrastructure element within a spoke or the hub — such as a Besu node, a Cacti relayer, or the Paladin privacy service. Has a type, endpoint, current health status, and last-checked timestamp.
- **HealthStatus**: A point-in-time record of a component's state (`HEALTHY`, `DEGRADED`, `OFFLINE`, `UNKNOWN`), captured timestamp, and optional diagnostic detail. `UNKNOWN` indicates the agent responsible for the component has not sent data within the grace period.
- **Alert**: A record of a detected failure condition, including severity, affected component, creation time, current state (`ACTIVE`, `RESOLVED`), and optional resolution notes.
- **Incident**: A grouping of one or more related alerts sharing a common root cause, with a lifecycle (open, resolved) and an associated timeline of state changes.
- **SLOMetric**: A computed metric covering a rolling time window — includes availability percentage, latency percentiles, whether the SLO was breached, and the breach duration if applicable.
- **TransactionEvent**: A single event in the lifecycle of a cross-border transaction, with a transaction identifier, event type, timestamp, originating spoke, and any error information.
- **Participant**: An institution (commercial bank or central bank) registered on the platform, associated with one or more spokes and used to scope health and audit queries.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A component failure on any registered spoke is reflected in the NOC dashboard within 30 seconds of occurrence, without any manual operator action.
- **SC-002**: 100% of cross-border transaction lifecycle events from all registered spokes are captured and searchable within the audit log.
- **SC-003**: Support engineers can retrieve a complete transaction event history for any transaction within 5 seconds using the transaction identifier search.
- **SC-004**: Adding a new spoke to the platform requires only a configuration change — no code modifications, container rebuilds, or service restarts are needed for the monitoring service to begin tracking the new spoke.
- **SC-005**: SLO dashboards accurately reflect availability and latency for any 30-day window, with breach periods identified and attributed to specific components.
- **SC-006**: Mean time to detect (MTTD) for critical component failures is under 1 minute from the moment the failure occurs.
- **SC-007**: The service handles at least 5 registered spokes simultaneously with no degradation in health polling accuracy or alert latency.
- **SC-008**: Alert deduplication ensures that a single root-cause failure does not generate more than one active incident per affected component.

---

## Assumptions

- The existing NOC frontend portal at `frontend/apps/noc/` will consume this service's API directly and is already capable of rendering health, alert, SLO, and audit data — UI development is out of scope for this service.
- Each spoke exposes a health-check endpoint or equivalent mechanism that the monitoring service can poll; the service is not responsible for instrumenting the spoke nodes themselves.
- The Cacti relayer and Paladin privacy service expose observable signals (e.g., health endpoints, event streams, or logs) that the monitoring service can consume.
- Authentication and authorization for NOC portal users are handled by **Keycloak** — the NOC portal is a Keycloak client; user management is performed in Keycloak, not in the NOC service. Three roles are defined: `noc-viewer` (read-only access to all dashboards), `noc-operator` (can acknowledge and resolve alerts), `noc-admin` (can register/remove spokes in the spoke registry). The NOC Backend validates the Keycloak-issued JWT on every request.
- "Spoke-agnostic" means the service's core logic applies uniformly to all spokes registered in the spoke registry — it does not mean the service is unaware of spoke topology, only that spoke-specific code changes are never required.
- Spoke registration (adding/removing spokes) is performed by platform operators, not end users or banks.
- Data retention for audit logs defaults to 90 days; longer retention periods are out of scope for v1 but the data model should not preclude future extension.
- Mobile browser support for the NOC portal is out of scope for v1.
- **L1/L2 support coordination (incident notes, participant-scoped support queries) is out of scope for v1.** Initial support workflows will be handled via email. User Story 5 and FR-011/FR-021 are explicitly deferred.
- The service operates in a trusted internal network; transport-level encryption (e.g., TLS between service and spokes) is assumed to be handled by the platform's existing network layer.
- The **NOC Agent** is implemented in **Go**, producing a lightweight static binary deployable as a Docker container co-located with each spoke/hub server. The **NOC Backend** is also implemented in Go, consistent with the `payment-orchestrator` stack.
- The NOC Backend uses a **dedicated PostgreSQL instance** (`noc-db`), isolated from the platform's transactional banking databases, accessed via GORM consistent with the platform standard.
- Real-time push notifications (e.g., webhooks, WebSockets for alerts) are desirable but not required for v1; polling-based delivery to the frontend is acceptable initially.
