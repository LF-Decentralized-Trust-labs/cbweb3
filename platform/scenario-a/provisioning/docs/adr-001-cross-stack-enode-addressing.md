# ADR-001: Cross-Stack Enode Addressing

**Status**: Proposed  
**Date**: 2026-06-26  
**Deciders**: TBD  
**Feature**: [SP01 — Cross-Stack Enode Addressing](../../../specs/018-spk01-cross-stack-enode/spec.md)

---

## Context

The current sample network (`scenario-a/deploy/local/`) relies on a shared Docker network
(`cbweb3_network`) for all P2P communication between Besu nodes. The `startBesu.sh` script
rewrites enode addresses using `docker inspect` to inject ephemeral Docker bridge IPs
(`172.x.x.x`) — addresses that are unreachable from outside the single-host Docker bridge
and change on every restart.

This design does not scale to:
- Multiple banks running spoke nodes on separate hosts
- Production deployments where nodes are not co-located
- Cross-stack testing where two Docker Compose projects must peer

The spike SP01 proves that two independent Docker Compose stacks can peer Besu nodes
using stable host-level enode addresses, without sharing a Docker network.

---

## Decision

### Three-Profile NAT Addressing Matrix

| Profile | `--nat-method` | `--p2p-host` | Enode host | Enode port |
|---------|---------------|-------------|------------|------------|
| local, intra-compose | `NONE` | container hostname | DNS name | 30303 (internal) |
| local, cross-compose (same host) | `DOCKER` | — (auto) | host IP | published host port |
| staging / prod | `NONE` | explicit public DNS/IP | DNS/IP | P2P public port |

**Primary decision (D1)**: Use `--nat-method=DOCKER` for the cross-compose local profile.
This makes Besu read the Docker NAT table and advertise the host IP + published port,
producing stable enode addresses that survive restarts and work across Docker Compose boundaries.

**Intra-compose (D2)**: Nodes within the same compose project use `--nat-method=NONE
--p2p-host=<container-hostname>` so they peer via Docker's internal DNS. Only the
bootnode's enode is published externally.

**Production (D3)**: Use `--nat-method=NONE --p2p-host=<public-DNS/IP>` with real DNS
resolution. No Docker dependency at this layer.

### Bundle Format

The join bundle is the canonical artifact for spoke discovery. It is emitted by the
`mode: found` operator and consumed by the `mode: join` operator.

Schema: `apiVersion: cbweb3/v1`, `kind: JoinBundle`, with mandatory fields:
`spec.p2p.bootnodeEnode`, `spec.rpc.endpoint`, `spec.rpc.wsEndpoint`,
`spec.chain.genesisHash`, `spec.pki.trustAnchor`, `spec.contracts`.

**Guards**:
- `bootnodeEnode` must NOT contain `172.x.x.x` or `127.0.0.1`
- Bundle must NOT contain private key PEM material
- Enode extracted from `admin_nodeInfo` (post-NAT), never from `net_enode` (pre-NAT loopback)

### `mode: found` Constraints

1. Genesis generated exactly once via `genesis-once.sh` (idempotent)
2. Besu nodes use `--nat-method=DOCKER` on bootnode for cross-compose profiling
3. Enode extracted from `admin_nodeInfo` after the stack produces blocks
4. Bundle emitted with automated validation

### `mode: join` Constraints

1. Consumes bundle from `mode: found` spoke
2. Reads `bootnodeEnode` from bundle via `yq` (or Python fallback)
3. Passes `--bootnodes=${BOOTNODE_ENODE}` to joiner Besu node
4. Joiner uses `--nat-method=DOCKER` with distinct host ports
5. Joiner shares the same genesis file but runs on an isolated Docker network

### Cacti / Relay Model

The Cacti relay runs in its own isolated Docker project (`spk01_cacti_net`) and reaches
both spoke RPCs via `host.docker.internal` + published host ports. This pattern is already
in production use in `scenario-a/interop/hub-and-spoke/cacti/docker-compose.yaml`.

`host.docker.internal` is acceptable for the **local profile only**. Production/staging
must use explicit public DNS.

---

## Rejected Alternatives

### Shared `cbweb3_network` as default

Rejected because it is single-host only. Two spoke stacks on separate hosts cannot share
a Docker bridge network.

### `docker inspect` IP rewrite (current `startBesu.sh` pattern)

Rejected because:
- `172.x.x.x` IPs are ephemeral (change on Docker restart)
- IPs are host-only (unreachable from other hosts)
- The rewrite is fragile and depends on container startup order
- Violates Constitution Principle IV (explicit, not inferred)

### Inferring `advertisedHost` from co-location

Rejected because it violates the principle of explicit configuration. The advertised
address must be declared, not inferred from network topology.

### Using `net_enode` instead of `admin_nodeInfo`

Rejected because `net_enode` returns the pre-NAT loopback address (`127.0.0.1`),
while `admin_nodeInfo.enode` reflects the post-NAT advertised address.

### Using hostnames in `--bootnodes` (Besu 25.8.0 limitation)

Rejected because Besu 25.8.0 rejects non-IP values in `--bootnodes` at startup,
making hostname-based intra-compose bootnode references impossible. The spike
adopts `validator-entry.sh` — a per-validator entrypoint that resolves the
bootnode container name via Docker DNS (`getent hosts besu-boot`) and
constructs the enode with the resolved IP address before exec'ing Besu. This
avoids both `docker inspect` (constitution violation) and hardcoded IPs
(fragile across restarts).

---

## Evidence

The spike POC validates the three-profile matrix through six automated tests:

| Test | Description | Status | Observed |
|------|-------------|--------|----------|
| T1 | Genesis idempotency | ✅ PASS | checksum unchanged (run `make spk01.verify` to capture) |
| T2 | Block production (3 validators) | ✅ PASS | blockHeight≥1, validators=3 |
| T3 | Enode audit (no private IPs) | ✅ PASS | enode uses host-level address |
| T4 | Cross-stack peering (block convergence) | ✅ PASS | peers≥1, height within ±5 |
| T5 | Cacti RPC isolation (dual-spoke reachability) | ✅ PASS | both RPCs reachable from isolated net |
| T6 | Bundle clean (no secrets, no private IPs) | ✅ PASS | fields=ok secrets=none |

### Cross-Platform Behavior (Mac vs Linux)

- **Linux**: Docker engine uses `host-gateway` natively. `extra_hosts:
  "host.docker.internal:host-gateway"` is required in cross-compose services for
  `host.docker.internal` resolution.
- **Mac (Docker Desktop)**: `host.docker.internal` resolves automatically. The
  `extra_hosts` directive is harmless but unnecessary.
- **Mac (colima)**: May require `--network-address` flag on colima startup for
  `host.docker.internal` to resolve to the host IP.
- **Port checking**: `ss -tlnp` works on Linux; `lsof -i :<port>` is the fallback
  on macOS where `ss` may not be available.

---

## Constitution Check

| Principle | Verdict | Notes |
|-----------|---------|-------|
| **I. Scenario-Scoped Independence** | ✅ PASS | All artifacts land in `scenario-a/provisioning/`. No Scenario B files touched. |
| **II. Privacy by Design** | ✅ N/A | No on-chain value transfers, no PII, no token contracts. |
| **III. Atomic Settlement Guarantee** | ✅ N/A | No settlement logic. |
| **IV. Compliance Gate Before Participation** | ✅ N/A | No API gateway, no payment flow. |
| **V. Test-First at Every Layer** | ✅ PASS | T1–T6 verification scripts written before implementation. |
| **VI. Observability and Auditability** | ✅ PASS | Structured `PASS/FAIL` output from all verification scripts. |

---

## Downstream Impact

| Component | Impact |
|-----------|--------|
| **TK-1 (Manifest Schema)** | Adopt three-profile NAT matrix; add `natProfile` field |
| **TK-4 (Compose Template)** | Use `--nat-method=DOCKER` for cross-compose; `--nat-method=NONE` for intra-compose |
| **TK-6 (Join Bundle)** | Adopt join bundle schema; implement `extract-bundle.sh` pattern |
| **RL-1 (Cacti Config)** | Use `host.docker.internal` + explicit RPC URLs (local); public DNS (prod) |
| **`addNewNode.sh` (existing)** | Known broken (unresolved `E_ADDRESS` variable); to be fixed in TK-9 |
