# SP01 — Cross-Stack Enode Addressing

Research spike proving that two independent Docker Compose stacks can peer Besu nodes
without sharing a Docker network, using stable host-level enode addresses.

## Prerequisites

- **Docker** (20.10+) and **Docker Compose v2**
- **jq** (JSON processing)
- **yq** (YAML processing) — optional; Python3 fallback with PyYAML is supported
- **sha256sum** — standard on Linux; `shasum -a 256` on macOS
- **ss** or **lsof** — for port availability checks

## Quick Start

```bash
# From the spike root directory:
# Full lifecycle: found → bundle → join → verify
make spk01.up && make spk01.verify

# Or run phases individually:
make spk01.found    # Start found stack (bootnode + 2 validators)
make spk01.bundle   # Extract join bundle
make spk01.join     # Start joiner node on isolated stack
make spk01.verify   # Run all verification tests (T1–T6)

# Teardown
make spk01.down     # Stop all stacks (preserves data/)
make spk01.clean    # Stop + remove data/ and bundles/
```

## Ports Used

| Port | Service | Host Mapping |
|------|---------|-------------|
| 31303 | Bootnode P2P (TCP+UDP) | HOST_P2P_BOOT |
| 8645 | Bootnode RPC HTTP | HOST_RPC_BOOT |
| 8655 | Bootnode WS | HOST_WS_BOOT |
| 31304 | Validator 1 P2P (TCP+UDP) | HOST_P2P_V1 |
| 31305 | Validator 2 P2P (TCP+UDP) | HOST_P2P_V2 |
| 31306 | Joiner P2P (TCP+UDP) | HOST_P2P_JOIN |
| 8646 | Joiner RPC HTTP | HOST_RPC_JOIN |
| 8656 | Joiner WS | HOST_WS_JOIN |

Override via environment variables (see `scripts/env-defaults.sh`).

## Test Criteria

| Test | Script | What it verifies |
|------|--------|-----------------|
| T1 | `verify-genesis-idempotency.sh` | Genesis is generated once; second run idempotent |
| T2 | `verify-peering.sh found` | Found stack produces blocks with 3 validators |
| T3 | `verify-enode.sh` | Enode does not contain private/Docker IPs |
| T4 | `verify-peering.sh join` | Joiner peers with found stack, block convergence |
| T5 | `verify-cacti-rpc.sh` | Isolated container reaches both spoke RPCs |
| T6 | `verify-bundle.sh` | Bundle contains no secrets or private IPs |

All tests emit `PASS <id> <description>` or `FAIL <id> <description>`.

## Known Platform Differences

### Linux
- `host.docker.internal` requires `extra_hosts: "host.docker.internal:host-gateway"` in
  compose files
- `ss -tlnp` available for port checks

### macOS (Docker Desktop)
- `host.docker.internal` resolves automatically
- `ss` may not be available; port checks fall back to `lsof -i :<port>`
- Host IP for the join bundle is resolved via `scripts/resolve-host-ip.sh` (default-route
  interface). Override with `export HOST_IP=<your-lan-ip>` if auto-detection fails.

### macOS (colima)
- May require `colima start --network-address` for `host.docker.internal` to resolve
  to the host IP

## Architecture

```
spk01_found_net          spk01_join_net           spk01_cacti_net
┌─────────────────┐      ┌────────────────┐       ┌──────────────────┐
│ besu-boot       │      │ besu-joiner    │       │ cacti-smoke      │
│ besu-v1         │      │                │       │ (curl container) │
│ besu-v2         │      └────────────────┘       └──────────────────┘
└─────────────────┘
        ↑ P2P via host port 31303      ↑ host.docker.internal + extra_hosts
        └──────── host.docker.internal ──────┘
                         ↑ eth_blockNumber via host port 8645/8646
```

All three stacks communicate only through ports published to the host. No inter-stack
Docker networks are shared.

## Cleanup

```bash
make spk01.clean
```

Removes all data directories, generated bundles, and Docker volumes.

## Flow Execution

```bash
make spk01.up     → found → bundle → join
make spk01.verify → T1 (idempotency) → T2 (block production) → T3 (enode audit)
                     → T4 (cross-stack peering + network isolation)
                     → T5 (cacti RPC isolation) → T6 (bundle clean)
make spk01.down   → cacti ↓ → join ↓ → found ↓
make spk01.clean  → down + remove data/ e bundles/
```

## See Also

- [ADR-001: Cross-Stack Enode Addressing](../../docs/adr-001-cross-stack-enode-addressing.md)
- [Feature Spec](../../../specs/018-spk01-cross-stack-enode/spec.md)
- [Research](../../../specs/018-spk01-cross-stack-enode/research.md)
