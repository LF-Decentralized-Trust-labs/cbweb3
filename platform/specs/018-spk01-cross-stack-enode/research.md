# Research: SP01 — Cross-Stack Enode Addressing

**Phase 0 output** | Branch: `018-spk01-cross-stack-enode`

---

## Finding 1 — The root cause is three lines in `startBesu.sh`

**Source**: `scenario-a/deploy/local/spoke-besu-a/startBesu.sh:194-198`

```bash
CENTRAL_BANK_IP=$(docker inspect \
    -f "{{(index .NetworkSettings.Networks \"${NETWORK_NAME}\").IPAddress}}" \
    "${NODE_CENTRAL_BANK}")
ENODE_INTERNAL=$(echo "$ENODE" | sed -e "s/127.0.0.1/${CENTRAL_BANK_IP}/g")
```

Besu's `net_enode` returns `127.0.0.1` when no `--nat-method` is configured. The script then replaces that with the Docker bridge IP (`172.x.x.x`) via `docker inspect`. This IP:
- Is only reachable from containers on the same Docker network
- Changes on Docker restart
- Is unreachable from any other host or stack

**Decision**: Do not copy this pattern. The toolkit must never call `docker inspect` to derive an enode address.

---

## Finding 2 — No `--nat-method` flag in the sample network

**Source**: `startBesu.sh:159,221,247` — all three `docker create` calls

The three Besu nodes are started with no `--nat-method` and no `--p2p-host`. This is why Besu reports `127.0.0.1` and the IP-rewrite hack is needed.

**Decision**: The POC must add `--nat-method=DOCKER` to the bootnode and validators. With this flag, Besu reads the Docker NAT table and advertises the *host* IP + the published port (e.g., `200.10.1.5:31303`), not the container IP.

---

## Finding 3 — `hyperledger/besu:latest` in sample network (constitution violation to avoid)

**Source**: `startBesu.sh:172,215,240`

All three containers use `hyperledger/besu:latest`. Constitution § Technology Stack Constraints mandates `25.8.0` pinned. The spike must use `hyperledger/besu:25.8.0` in all compose files.

**Decision**: Pin `hyperledger/besu:25.8.0` everywhere in the spike.

---

## Finding 4 — Genesis is regenerated on every `startBesu.sh` run

**Source**: `startBesu.sh:86-116` — generates `qbftConfigFile.json`, calls `operator generate-blockchain-config`, copies keys and genesis unconditionally on every invocation.

This destroys the chain on restart. The toolkit must implement the opposite.

**Decision**: `genesis-once.sh` checks for the genesis file first, and exits 0 with a "skip" message if it already exists. The compose `command` for the found stack references this idempotent genesis. The verification script T1 invokes `genesis-once.sh` twice and asserts the file is unchanged.

---

## Finding 5 — Cacti relay already proves the cross-network RPC model

**Source**: `scenario-a/interop/hub-and-spoke/cacti/docker-compose.yaml`

The Cacti relay:
1. Runs in its **own** isolated Docker project (`cacti_default` network)
2. Reaches both spokes via `host.docker.internal:8645` / `host.docker.internal:8745`
3. Already uses `extra_hosts: "host.docker.internal:host-gateway"` for Linux compatibility

This confirms that `host.docker.internal` + published host ports is the validated RPC pattern for cross-network access. The spike can adopt this pattern for the relay smoke test directly.

**Decision**: The `cacti-isolated.yml` smoke test is straightforward — use the same `extra_hosts` + env-var pattern already in production use. RPC access is proven; the spike's novel part is P2P peering.

---

## Finding 6 — `--nat-method=DOCKER` behavior and port mismatch constraint

**Source**: Besu 25.x documentation + code analysis

With `--nat-method=DOCKER`, Besu reads the container's published port mapping from the Docker daemon. It advertises `enode://<pubkey>@<host-ip>:<host-port>` where `<host-port>` is whatever port the container's P2P port is published as on the host.

**Critical constraint**: When using `--nat-method=DOCKER`, the advertised port = the host's published port. If the container uses `--p2p-port=30303` published as `-p 31303:30303`, the enode announces port `31303` (the host port). A second stack trying to peer must use that host port, not `30303`.

**For intra-compose** (all containers in the same Docker network): use `--nat-method=NONE --p2p-host=<container-hostname>` so the enode is `enode://<pubkey>@<container-hostname>:30303` — only valid within the same Docker network.

**Three-profile matrix** (validated decision):

| Profile | `--nat-method` | `--p2p-host` | Enode host | Enode port |
|---------|---------------|-------------|------------|------------|
| local, intra-compose | `NONE` | container hostname | DNS name | 30303 (internal) |
| local, cross-compose (same host) | `DOCKER` | — (auto) | host IP | published host port |
| staging / prod | `NONE` | explicit public DNS/IP | DNS/IP | P2P public port |

---

## Finding 7 — `addNewNode.sh` broken — join path was never validated

**Source**: `scenario-a/deploy/local/spoke-besu-a/addNewNode.sh:66` — `E_ADDRESS` is referenced but never defined

This confirms the `mode: join` path has never been exercised in the existing setup. The spike's T4 is the first validated proof that cross-stack joining works. This is not a blocker for the POC (the POC uses `--bootnodes` flag directly, not `addNewNode.sh`), but it must be noted in the ADR.

**Decision**: The spike documents this as a known gap. `addNewNode.sh` is out of scope; it belongs to the existing sample network (untouched) and will be addressed in the toolkit's `mode: join` engine (TK-9).

---

## Finding 8 — Paladin uses `host.docker.internal` for Besu even on the same network

**Source**: `scenario-a/deploy/local/paladin/spoke-a/config/central-bank/config.yaml.tmpl` — uses `host.docker.internal:8645` for Besu RPC even though Paladin and Besu share `spoke_a_besu_network`

This is a known inconsistency in the sample network (Paladin avoids internal DNS for Besu). It does not affect the spike, which excludes Paladin. It is noted here because TK-5 (orchestration engine) will need to address it.

---

## Summary of Decisions

| # | Decision | Rationale |
|---|---------|-----------|
| D1 | `--nat-method=DOCKER` for cross-compose local profile | Advertises host IP + published port; stable across restarts |
| D2 | `--nat-method=NONE --p2p-host=<hostname>` for intra-compose | DNS name resolves within stack; no host-port dependency |
| D3 | `--nat-method=NONE --p2p-host=<DNS/IP>` for prod/staging | Real DNS; no Docker dependency |
| D4 | `genesis-once.sh` idempotency guard | Never regenerate genesis on a running spoke |
| D5 | Bundle extracts enode from `admin_nodeInfo` (not `net_enode`) | `admin_nodeInfo.enode` reflects the advertised address after NAT; `net_enode` returns pre-NAT loopback |
| D6 | `extra_hosts: host-gateway` in all cross-compose services | Required for `host.docker.internal` resolution on Linux |
| D7 | Bundle never contains `172.x` or `127.0.0.1` | Automated guard (`verify-enode.sh`) fails the pipeline if found |
| D8 | Pin `hyperledger/besu:25.8.0` in all spike compose files | Constitution mandate; no `latest` tag |
