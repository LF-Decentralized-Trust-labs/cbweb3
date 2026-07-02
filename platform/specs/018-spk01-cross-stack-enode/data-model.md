# Data Model: SP01 — Cross-Stack Enode Addressing

**Phase 1 output** | Branch: `018-spk01-cross-stack-enode`

---

## Key Entity: Join Bundle

The join bundle is the central artifact of the spike. It is the output of `extract-bundle.sh` after
a `mode: found` spoke is initialized, and the input consumed by `start-join.sh` for `mode: join`.

### Schema (`bundles/<spoke-id>.bundle.yaml`)

```yaml
apiVersion: cbweb3/v1
kind: JoinBundle
metadata:
  spokeId: string          # e.g. "spoke-brl" — unique identifier for the spoke
  createdAt: string        # ISO-8601 timestamp

spec:
  p2p:
    bootnodeEnode: string  # enode://<pubkey>@<host>:<port> — MUST NOT contain 172.x/127.0.0.1
                           # MUST use host-level address (from admin_nodeInfo, not net_enode)

  rpc:
    endpoint: string       # http://<host>:<port> — explicit, never inferred
    wsEndpoint: string     # ws://<host>:<port>   — explicit, never inferred

  chain:
    genesisHash: string    # sha256 of genesis.json — used for integrity check on join

  pki:
    trustAnchor: string    # PEM-encoded CA certificate of the founding central bank
                           # This is the spoke's trust root; no private keys

  contracts:               # Deployed contract addresses (empty in POC; populated in TK-6)
    identityRegistry: string   # 0x...
    fxAgreement: string        # 0x...
    zetoFactory: string        # 0x...

  relay:
    endpoint: string       # http://<host>:<port> of the LNET Cacti relay (optional in POC)
```

### Invariants

- `bootnodeEnode` MUST match the pattern `enode://[0-9a-f]{128}@[^:]+:[0-9]+`
- `bootnodeEnode` MUST NOT match `172\.(1[6-9]|2[0-9]|3[01])\.` or `127\.0\.0\.1`
- `trustAnchor` MUST contain `-----BEGIN CERTIFICATE-----` and no `-----BEGIN.*PRIVATE KEY-----`
- The file MUST NOT contain any field whose value matches a private key PEM pattern
- `genesisHash` MUST be the SHA-256 hex digest of the genesis.json as-written (reproducible)

---

## Key Entity: Spoke Node Config (runtime, not persisted)

Transient configuration derived by `start-found.sh` and `start-join.sh` for each Besu container.
Not stored as a file — it is the set of environment variables and Docker flags passed at startup.

| Field | Source | Example |
|-------|--------|---------|
| `HOST_P2P` | Manifest / env | `31303` |
| `HOST_RPC` | Manifest / env | `8645` |
| `HOST_WS` | Manifest / env | `8655` |
| `NAT_METHOD` | Profile | `DOCKER` (cross-compose local) |
| `P2P_HOST` | Manifest `advertisedHost` | (only when `NAT_METHOD=NONE`) |
| `BOOTNODES` | Join bundle `p2p.bootnodeEnode` | enode://...@host:31303 |
| `GENESIS_VOLUME` | Script output | `./data/genesis.json` |

---

## Key Entity: Verification Result (test harness output)

Each `verify-*.sh` script writes a structured one-line result to stdout on exit:

```
[PASS|FAIL] <test-id> <description> [<detail>]
```

Examples:
```
PASS T1 genesis-idempotency checksum=sha256:abc123 unchanged
PASS T3 enode-audit enode=enode://abc@192.168.1.5:31303
FAIL T3 enode-audit enode=enode://abc@172.20.0.5:30303 CONTAINS_PRIVATE_IP
PASS T4 cross-stack-peering peers=1 blockHeight=42
```

The Makefile target `spk01.verify` aggregates these and exits non-zero if any line starts with `FAIL`.

---

## Key Entity: ADR (Architecture Decision Record)

File: `scenario-a/provisioning/docs/adr-001-cross-stack-enode-addressing.md`

This is not a data entity but a structured document. Its "data" is the set of decisions from
research.md, enriched with POC evidence (script outputs, cross-platform test results).

Mandatory fields:
- `status`: Proposed → Accepted (after team review)
- `deciders`: list of reviewers
- `date`: ISO-8601
- `context`: why `cbweb3_network` doesn't scale
- `decision`: the three-profile addressing matrix (D1–D3 from research.md)
- `consequences`: what changes in TK-1, TK-4, TK-6, RL-1
- `rejected-alternatives`: shared Docker network, `docker inspect` IP rewrite, co-location inference
- `evidence`: links to T1–T6 script outputs
