# noc-agent

> [scenario-a](../../../README.md) › [backend](../../README.md) › noc-agent

The **noc-agent** is a lightweight monitoring daemon that runs alongside the platform's services. It collects operational metrics — Docker container health, Besu block heights, and service logs — and pushes them to the [noc-backend](../noc-backend/README.md) for aggregation and display on the NOC dashboard.

---

## Architecture Placement

```
Docker socket  ──► [noc-agent]  ──► HTTP push ──► [noc-backend] :8000
Besu RPC       ──►     │
Service logs   ──►
```

The agent is stateless and outbound-only: it has no inbound port. It is deployed as a sidecar process per spoke environment, configured via a YAML/JSON manifest that describes the components to watch.

---

## Responsibilities

- **Container monitoring** — Polls the Docker socket to check the running state and resource usage of platform containers.
- **Besu block height** — Queries Besu JSON-RPC to track block progression per spoke node.
- **Log collection** — Gathers recent log output from configured containers.
- **Telemetry push** — Periodically POSTs collected metrics to the noc-backend ingestion endpoint.

---

## Key Details

| Property | Value |
|----------|-------|
| Protocol | Outbound HTTP push (no inbound port) |
| Language | Go |
| Config | YAML/JSON component manifest |

### Configuration File (`noc-agent.yaml`)

The agent reads a component manifest that declares which Docker containers and Besu nodes to monitor:

```yaml
noc_backend_url: "http://noc-backend:8000"
push_interval_seconds: 10
components:
  - name: bank-a-api-gateway
    container_id: cbweb3-bank-a-api-gateway
  - name: spoke-a-besu-node1
    container_id: cbweb3-besu-a-node1
    besu_rpc_url: "http://besu-a-node1:8645"
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `NOC_AGENT_CONFIG` | Path to the YAML component manifest |
| `DOCKER_HOST` | Docker socket path (default: `unix:///var/run/docker.sock`) |

---

## Related

- [noc-backend](../noc-backend/README.md) — receives and stores the telemetry this agent produces
- [frontend › noc app](../../../frontend/apps/noc/README.md) — the NOC dashboard that visualises the data
