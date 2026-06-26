# Data Model: SP02 — Live Spoke Join (QBFT + Paladin)

**Branch**: `019-spk02-live-join` | **Date**: 2026-06-26
**Note**: This spike produces no persistent data structures. The data model records state
transitions, artifact schemas, and the POC's runtime entities.

---

## State Machines

### QBFT Validator State Machine

```
         start-found.sh
              │
              ▼
    ┌─────────────────────┐
    │   Active Validator  │  (nodes 0, 1, 2 — in genesis extraValidators)
    └────────┬────────────┘
             │  block production running
             │
         join-besu.sh
              │
              ▼
    ┌─────────────────────┐
    │   Non-Validator     │  (node 3 — synced, peered, not in validator set)
    │   (Synced Peer)     │
    └────────┬────────────┘
             │  vote-in-validator.sh (2 of 3 existing validators vote)
             ▼
    ┌─────────────────────┐
    │   Voted Candidate   │  (votes recorded in block headers; epoch not yet reached)
    │   (awaiting epoch)  │
    └────────┬────────────┘
             │  epoch boundary reached (blockNumber % epochlength == 0)
             ▼
    ┌─────────────────────┐
    │   Active Validator  │  (node 3 now in qbft_getValidatorsByBlockNumber("latest"))
    └─────────────────────┘
```

**Invariants**:
- Block production must be continuous across all state transitions above
- A non-zero gap between consecutive block timestamps > 2× `blockperiodseconds` = FAIL
- No existing validator container restarts during the entire state machine

---

### Paladin Node State Machine

```
         start-found.sh (CB + Bank-A)
              │
              ▼
    ┌─────────────────────┐
    │   Running + Reg'd   │  (nodeName registered on-chain in IdentityRegistry)
    │   CB + Bank-A       │
    └────────┬────────────┘
             │  join-paladin.sh starts Bank-X
             ▼
    ┌─────────────────────┐
    │   Running (Bank-X)  │  (container up, TLS cert generated, config rendered)
    │   Unregistered      │
    └────────┬────────────┘
             │  TestRegisterPaladinNodes (Bank-X registers on-chain)
             ▼
    ┌─────────────────────┐
    │   Registered        │  (Bank-X endpoint + cert published on-chain)
    │   (on-chain visible)│
    └────────┬────────────┘
             │  existing nodes discover Bank-X (reactive) OR require restart (config)
             ▼
    ┌─────────────────────┐    ┌──────────────────────────────────────┐
    │   Discoverable      │ OR │   Discovery blocked (TLS mismatch)   │
    │   (no restart)      │    │   → rolling restart of CB + Bank-A   │
    └────────┬────────────┘    └──────────────────────────────────────┘
             │                             │
             └─────────────┬───────────────┘
                           ▼
    ┌─────────────────────────────────┐
    │   mTLS Established              │
    │   (Bank-X ↔ CB cross-node op)  │
    └─────────────────────────────────┘
```

**Invariants**:
- If no restart path: CB and Bank-A container start times unchanged after join
- If restart path: only Paladin containers restart; Besu validators unchanged

---

## Artifacts

### Join Bundle (reused from SP01, schema `cbweb3/v1 JoinBundle`)

```yaml
apiVersion: cbweb3/v1
kind: JoinBundle
metadata:
  spokeId: spoke-spk02
  emittedAt: <ISO-8601>
spec:
  p2p:
    bootnodeEnode: enode://<pubkey>@<host-ip>:<port>
  rpc:
    endpoint: http://<host-ip>:<port>
    wsEndpoint: ws://<host-ip>:<port>
  chain:
    genesisHash: <0x...>
    epochlength: 30           # SP02 spike value; toolkit default is configurable
  pki:
    trustAnchor: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
  contracts:
    identityRegistry: <0x...>
```

**Guards** (same as SP01):
- `bootnodeEnode` must not contain `172.x.x.x` or `127.0.0.1`
- Bundle must not contain private key PEM material

### ADR-002 Evidence Table Schema

```
| Test | Description | Status | Observed |
|------|-------------|--------|----------|
| T1   | QBFT validator set after voting | ✅/❌ | addresses=4, joiner=<addr> |
| T2   | Block continuity during voting | ✅/❌ | max_gap_seconds=<N>, block_time=<M> |
| T3   | mTLS cross-node op (Bank-X → CB) | ✅/❌ | ptx result or error message |
| T4   | CB + Bank-A Paladin no restart | ✅/❌ | start_time_before=<T>, after=<T> |
| T5   | Besu validators no restart | ✅/❌ | start_time_before=<T>, after=<T> |
| T6   | ADR-002 evidence table complete | ✅/❌ | all 5 rows populated |
```

---

## Runtime Entities

| Entity | Container name | Network | Host ports |
|--------|---------------|---------|-----------|
| Besu bootnode (validator 0) | `spk02-besu-boot` | `spk02_found_net` | P2P: 32303, RPC: 9645, WS: 9655 |
| Besu validator 1 | `spk02-besu-v1` | `spk02_found_net` | P2P: 32304 |
| Besu validator 2 | `spk02-besu-v2` | `spk02_found_net` | P2P: 32305 |
| Besu joiner (node 3) | `spk02-besu-joiner` | `spk02_join_net` | P2P: 32306, RPC: 9646, WS: 9656 |
| Paladin CB | `spk02-paladin-cb` | `spk02_found_net` | RPC: 9648, gRPC: 9700 |
| Paladin Bank-A | `spk02-paladin-bank-a` | `spk02_found_net` | RPC: 9649, gRPC: 9701 |
| Paladin Bank-X | `spk02-paladin-bank-x` | `spk02_join_net` | RPC: 9650, gRPC: 9702 |

**Port range**: 9xxx (distinct from SP01's 8xxx range and sample network's 8x45 range;
avoids port collisions when both spikes are running).

**Network isolation**: `spk02_found_net` and `spk02_join_net` are separate Docker networks.
Besu and Paladin cross-stack communication happens via `host.docker.internal` + published
host ports (same pattern as SP01).
