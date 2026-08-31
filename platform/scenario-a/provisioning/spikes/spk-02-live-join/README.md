# SP02 — Live Spoke Join (QBFT + Paladin)

Research spike that answers two questions left open by SP01:

1. **QBFT validator promotion**: What is the exact sequence of votes required to promote
   a synced non-validator to an active QBFT validator on a live network?
2. **Paladin live join**: Does adding a new Paladin node to a running spoke require a
   restart of existing Paladin nodes?

## Prerequisites

- Docker 24+ with Compose v2
- `jq`, `curl`, `openssl`, `sha256sum`
- `cast` (Foundry 1.5+) — used by `register-paladin-nodes.sh` for on-chain IdentityRegistry calls (typically installed in `~/.foundry/bin`; `env-defaults.sh` adds this to PATH automatically)
- `go` 1.25+ (for Paladin registry deployment Go tests)
- Python 3 (for JSON parsing in verification scripts)
- SP01 should be in a passing state (validates the enode addressing pattern)

## Quick Start

```bash
# Full POC: found stack + bundle + join + verify
make spk02.up && make spk02.verify

# Step-by-step
make spk02.found          # Start 3-validator Besu + Paladin CB + Bank-A
make spk02.bundle         # Extract join bundle
make spk02.join           # Start Besu joiner (non-validator, synced)
make spk02.vote           # Cast QBFT votes to promote joiner
make spk02.paladin-join   # Start Paladin Bank-X node
make spk02.verify         # Run T1-T6 verification suite
```

## Port Table

| Container | Role | Host P2P | Host RPC | Host WS | Host Paladin RPC | Host gRPC |
|-----------|------|----------|----------|---------|-----------------|-----------|
| `spk02-besu-boot` | Besu validator 0 (bootnode) | 32303 | 9645 | 9655 | — | — |
| `spk02-besu-v1` | Besu validator 1 | 32304 | 9643 | — | — | — |
| `spk02-besu-v2` | Besu validator 2 | 32305 | 9644 | — | — | — |
| `spk02-besu-joiner` | Besu joiner (non-validator → validator) | 32306 | 9646 | 9656 | — | — |
| `spk02-paladin-cb` | Paladin central bank | — | — | — | 9648 | 9700 |
| `spk02-paladin-bank-a` | Paladin bank A | — | — | — | 9649 | 9701 |
| `spk02-paladin-bank-x` | Paladin bank X (joiner) | — | — | — | 9650 | 9702 |

Ports are in the 9xxx range to avoid collisions with SP01 (8xxx) and Scenario A deploy ports (8x45).

## Test Criteria

| Test | Script | Description | Pass Condition |
|------|--------|-------------|---------------|
| T1 | `verify-qbft.sh` | QBFT validator set after voting | 4 addresses returned; joiner present |
| T2 | `verify-qbft.sh --t2` | Block continuity during voting | No gap > 2× blockperiodseconds (4s) |
| T3 | `verify-paladin-join.sh --t3` | Paladin mTLS cross-node op | No x509/transport error |
| T4 | `verify-paladin-join.sh --t4` | Existing Paladin no restart | Start times unchanged for CB + Bank-A |
| T5 | `verify-restart-scope.sh` | Besu validators no restart | All container start times unchanged |
| T6 | `verify-adr-evidence.sh` | ADR-002 evidence table complete | All 6 rows populated (no PENDING) |

## Known Platform Differences

- **Linux**: `host.docker.internal` requires `extra_hosts: host.docker.internal:host-gateway` in compose files
- **Mac**: Docker Desktop provides `host.docker.internal` natively; the `extra_hosts` directive is benign
- **Port checking**: `ss -tlnp` on Linux, `lsof` fallback on macOS

## TLS Verdict Summary

[FILLED AFTER POC RUNS]

The TLS trust model verdict (T3) determines whether `mode: join` requires a restart window:

| Hypothesis | Description | POC Result |
|-----------|-------------|------------|
| H-A | Registry-fetched cert (TOFU) | [PENDING] |
| H-B | Startup-only trust store | [PENDING] |
| H-C | InsecureSkipVerify=true | [PENDING] |

## ADR-002

The Architecture Decision Record is the primary deliverable of this spike.
See [docs/adr-002-live-validator-join.md](docs/adr-002-live-validator-join.md).

## Cleanup

```bash
make spk02.down    # Stop all containers
make spk02.clean   # Stop + remove data/ and bundles/
```
