# Implementation Plan: SP01 — Cross-Stack Enode Addressing

**Branch**: `018-spk01-cross-stack-enode` | **Date**: 2026-06-26 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `specs/018-spk01-cross-stack-enode/spec.md`

---

## Summary

This spike proves that two independent Docker Compose stacks can peer Besu nodes without sharing a
Docker network, using stable host-level enode addresses instead of the ephemeral `172.x.x.x` bridge
IPs produced by the current `startBesu.sh`. The deliverable is an ADR (design decision document)
backed by a working POC with six automated verification tests. The decisions recorded in the ADR
directly feed the provisioning toolkit's TK-1 (manifest schema), TK-4 (compose template), TK-6
(join bundle), and RL-1 (Cacti config).

---

## Technical Context

**Language/Version**: Bash 5+ scripts, Docker Compose v2, YAML  
**Primary Dependencies**: `hyperledger/besu:25.8.0` (pinned), `curlimages/curl:latest` (smoke test), `jq`, `sha256sum`  
**Storage**: Filesystem only — genesis.json, node keypairs, bundle YAML (`bundles/<spoke-id>.bundle.yaml`)  
**Testing**: Bash verification scripts (`verify-enode.sh`, `verify-peering.sh`, `verify-cacti-rpc.sh`) invoked via `make spk01.verify`  
**Target Platform**: Linux (primary) + Docker Desktop for Mac (secondary — cross-platform validation)  
**Project Type**: Research spike — Bash scripts + Docker Compose; no Go services, no contracts, no frontend  
**Performance Goals**: N/A for spike; P2P peering convergence within 60 seconds (SC-002)  
**Constraints**: No modification to `scenario-a/deploy/local/` or `scenario-a/make/`; all artifacts self-contained in `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/`  
**Scale/Scope**: POC with 1 bootnode + 2 validators (found stack) + 1 non-validator joiner (join stack)

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Verdict | Notes |
|-----------|---------|-------|
| **I. Scenario-Scoped Independence** | ✅ PASS | All artifacts land in `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/` and `scenario-a/provisioning/docs/`. No Scenario B files touched. |
| **II. Privacy by Design** | ✅ N/A | No on-chain value transfers, no PII, no token contracts. POC is pure P2P networking. |
| **III. Atomic Settlement Guarantee** | ✅ N/A | No settlement logic, no HTLC, no relay settlement. Out of scope for this spike. |
| **IV. Compliance Gate Before Participation** | ✅ N/A | No API gateway, no payment flow, no Keycloak. Out of scope. |
| **V. Test-First at Every Layer** | ✅ PASS | Verification scripts define acceptance criteria (T1–T6) before POC implementation. The `make spk01.verify` target codifies them as executable tests. The Red-Green flow: T3 (`verify-enode.sh`) is written to FAIL on `172.x.x.x` addresses before the correct Besu flags are in place, then PASS after. |
| **VI. Observability and Auditability** | ✅ PASS | Each `verify-*.sh` script emits structured `[PASS\|FAIL] <test-id> <description>` lines. No silent failures. `genesis-once.sh` explicitly logs "skip" on idempotent runs. |

**Constitution Check: PASS** — spike proceeds.

---

## Project Structure

### Documentation (this feature)

```text
specs/018-spk01-cross-stack-enode/
├── plan.md            ← this file
├── research.md        ← Phase 0 output (decisions D1–D8)
├── data-model.md      ← Phase 1 output (bundle schema, verify output format)
└── tasks.md           ← Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code (repository root)

```text
scenario-a/
  provisioning/
    spikes/
      spk-01-cross-stack-enode/
        README.md                   # how to run, prerequisites, test criteria
        Makefile                    # spk01.up / spk01.verify / spk01.down
        compose/
          stack-found.yml           # bootnode + 2 validators, isolated network spk01_found_net
          stack-join.yml            # 1 non-validator joiner, isolated network spk01_join_net
          cacti-isolated.yml        # relay smoke test, isolated network spk01_cacti_net
        scripts/
          genesis-once.sh           # creates genesis once; refuses to overwrite (T1)
          start-found.sh            # starts stack-found with correct Besu NAT flags
          extract-bundle.sh         # calls admin_nodeInfo; emits bundles/<spoke-id>.bundle.yaml
          start-join.sh             # reads bundle; starts stack-join with --bootnodes
          verify-enode.sh           # fails if enode contains 172.x/127.0.0.1 (T3)
          verify-peering.sh         # checks admin_peers ≥ 1 and blockHeight convergence (T2, T4)
          verify-cacti-rpc.sh       # calls eth_blockNumber on both RPCs from isolated net (T5)
        bundles/
          .gitkeep                  # bundle files land here; never contain secrets
    docs/
      adr-001-cross-stack-enode-addressing.md
```

**Structure Decision**: Single flat spike directory under `scenario-a/provisioning/spikes/`. The `provisioning/` tree does not yet exist — the spike scaffolds it. The `docs/` directory is shared across spikes and future toolkit documentation.

---

## Complexity Tracking

No constitution violations. No Complexity Tracking entries required.

---

## Phase 0 — Research *(complete — see research.md)*

All unknowns resolved. Key findings:

1. **Root cause confirmed**: `startBesu.sh:194-198` rewrites enode with `docker inspect` bridge IP. No `--nat-method` flag on any node.
2. **Fix**: `--nat-method=DOCKER` makes Besu advertise host IP + published port. Use `admin_nodeInfo` (not `net_enode`) to extract the post-NAT enode.
3. **Cacti relay already proves the RPC model**: `cacti/docker-compose.yaml` uses `host.docker.internal` + `extra_hosts: host-gateway`. Adopt this pattern directly.
4. **Constitution violation in sample network**: `hyperledger/besu:latest` — spike must use `:25.8.0`.
5. **Genesis regeneration is destructive**: `startBesu.sh` regenerates unconditionally. Spike implements the opposite with `genesis-once.sh`.

Full decisions in [research.md](research.md).

---

## Phase 1 — Design

### 1.1 Compose network topology

Three stacks with zero shared networks:

```
spk01_found_net          spk01_join_net           spk01_cacti_net
┌─────────────────┐      ┌────────────────┐       ┌──────────────────┐
│ besu-boot       │      │ besu-joiner    │       │ cacti-smoke      │
│ besu-validator1 │      │                │       │ (curl container) │
│ besu-validator2 │      └────────────────┘       └──────────────────┘
└─────────────────┘
        ↑ P2P via host port 31303      ↑ host.docker.internal + extra_hosts
        └──────── host.docker.internal:31303 ──────┘
                         ↑ eth_blockNumber via host port 8645/8646
                         └──── host.docker.internal:8645/8646 ────────────────┘
```

All three stacks communicate only through ports published to the host. No inter-stack Docker networks.

### 1.2 Besu flag specifications

**Found stack — bootnode** (`nat-method=DOCKER`, port-mapped to host):

```yaml
command:
  - --nat-method=DOCKER
  - --p2p-port=30303
  - --rpc-http-enabled=true
  - --rpc-http-port=8545
  - --rpc-http-api=ETH,NET,QBFT,ADMIN
  - --rpc-ws-enabled=true
  - --rpc-ws-port=8546
  - --rpc-http-host=0.0.0.0
  - --rpc-ws-host=0.0.0.0
  - --host-allowlist=*
  - --rpc-http-cors-origins=all
  - --genesis-file=/opt/besu/genesis/genesis.json
  - --data-path=/opt/besu/data
  - --min-gas-price=0
ports:
  - "${HOST_P2P_BOOT:-31303}:30303"
  - "${HOST_P2P_BOOT:-31303}:30303/udp"
  - "${HOST_RPC_BOOT:-8645}:8545"
  - "${HOST_WS_BOOT:-8655}:8546"
```

**Critical**: `--rpc-http-api` must include `ADMIN` so `admin_nodeInfo` is accessible for bundle extraction.

**Found stack — validators 1 & 2** (same flags, different ports, `--bootnodes` set to bootnode's internal DNS address within `spk01_found_net`):

```yaml
command:
  - --nat-method=DOCKER
  - --bootnodes=enode://<bootnode-pubkey>@spk01-besu-boot:30303
  # ...same flags as bootnode...
```

Validators within the same compose project use the container hostname `spk01-besu-boot` (intra-stack DNS). Only the bootnode's enode needs host-level addressing in the bundle.

**Join stack — joiner node**:

```yaml
command:
  - --nat-method=DOCKER
  - --bootnodes=${BOOTNODE_ENODE}       # loaded from bundle by start-join.sh
  - --p2p-port=30303
  # ...same RPC flags...
extra_hosts:
  - "host.docker.internal:host-gateway" # Linux compatibility
ports:
  - "${HOST_P2P_JOIN:-31304}:30303"
  - "${HOST_P2P_JOIN:-31304}:30303/udp"
  - "${HOST_RPC_JOIN:-8646}:8545"
  - "${HOST_WS_JOIN:-8656}:8546"
```

### 1.3 `genesis-once.sh` logic

```
if [ -f "$GENESIS_FILE" ]; then
  echo "[genesis-once] SKIP: genesis already exists at $GENESIS_FILE"
  exit 0
fi
# generate genesis using Besu operator generate-blockchain-config
# write to $GENESIS_FILE
echo "[genesis-once] CREATED: $GENESIS_FILE"
```

The verification test T1 runs this script twice and asserts:
1. First run: exits 0, file exists
2. Second run: exits 0, prints "SKIP", file checksum unchanged

### 1.4 `extract-bundle.sh` — enode extraction

Uses `admin_nodeInfo` (not `net_enode`) because `admin_nodeInfo.enode` reflects the address after NAT rewriting:

```bash
ENODE=$(curl -s -X POST http://localhost:${HOST_RPC_BOOT:-8645} \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' \
  | jq -r '.result.enode')

# Guard: fail hard if enode contains private Docker IP
if echo "$ENODE" | grep -qE '172\.(1[6-9]|2[0-9]|3[01])\.|127\.0\.0\.1'; then
  echo "[FAIL] extract-bundle: enode contains private Docker IP: $ENODE"
  echo "[FAIL] Ensure bootnode is started with --nat-method=DOCKER and host port is published"
  exit 1
fi
```

The bundle is written to `bundles/spoke-spk01.bundle.yaml` following the schema in [data-model.md](data-model.md).

### 1.5 `verify-enode.sh` (T3) — the key guard

This script is the anti-hack guard. It is intentionally written to **fail before the correct Besu
flags are in place** (Red phase), then pass after (Green phase). This is the test-first pattern for
the spike.

```bash
#!/usr/bin/env bash
set -euo pipefail
RPC="${1:-http://localhost:${HOST_RPC_BOOT:-8645}}"
ENODE=$(curl -sf -X POST "$RPC" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' \
  | jq -r '.result.enode')

if echo "$ENODE" | grep -qE '172\.(1[6-9]|2[0-9]|3[01])\.|127\.0\.0\.1'; then
  echo "FAIL T3 enode-audit enode=$ENODE CONTAINS_PRIVATE_IP"
  exit 1
fi
echo "PASS T3 enode-audit enode=$ENODE"
```

### 1.6 Bundle validation guard

`extract-bundle.sh` also validates the produced YAML:

```bash
# No private key patterns
if grep -qE '\-\-\-\-\-BEGIN.*(PRIVATE|ENCRYPTED).*KEY' bundles/spoke-spk01.bundle.yaml; then
  echo "[FAIL] Bundle contains private key material"; exit 1
fi
# No Docker bridge IPs
if grep -qE '172\.(1[6-9]|2[0-9]|3[01])\.|127\.0\.0\.1' bundles/spoke-spk01.bundle.yaml; then
  echo "[FAIL] Bundle contains private Docker IP"; exit 1
fi
echo "[PASS] Bundle is clean and committable"
```

### 1.7 Makefile targets

```makefile
.PHONY: spk01.up spk01.verify spk01.down spk01.clean

spk01.up: spk01.found spk01.bundle spk01.join

spk01.found:
	bash scripts/genesis-once.sh ./data/genesis.json
	docker compose -f compose/stack-found.yml up -d
	@echo "Waiting for found stack to produce blocks..."
	@bash scripts/wait-for-blocks.sh http://localhost:8645 5

spk01.bundle:
	bash scripts/extract-bundle.sh

spk01.join:
	BOOTNODE_ENODE=$$(yq e '.spec.p2p.bootnodeEnode' bundles/spoke-spk01.bundle.yaml) \
	  docker compose -f compose/stack-join.yml up -d

spk01.verify:
	@echo "=== T1: genesis idempotency ==="
	bash scripts/genesis-once.sh ./data/genesis.json   # must print SKIP
	@echo "=== T2: block production ==="
	bash scripts/verify-peering.sh found http://localhost:8645 http://localhost:8646
	@echo "=== T3: enode audit ==="
	bash scripts/verify-enode.sh http://localhost:8645
	@echo "=== T4: cross-stack peering ==="
	bash scripts/verify-peering.sh join http://localhost:8646
	@echo "=== T5: cacti RPC isolation ==="
	docker compose -f compose/cacti-isolated.yml run --rm cacti-smoke \
	  bash scripts/verify-cacti-rpc.sh
	@echo "=== T6: bundle clean ==="
	bash scripts/verify-bundle.sh bundles/spoke-spk01.bundle.yaml
	@echo "ALL TESTS PASSED"

spk01.down:
	docker compose -f compose/stack-join.yml down -v
	docker compose -f compose/stack-found.yml down -v
	docker compose -f compose/cacti-isolated.yml down -v

spk01.clean: spk01.down
	rm -rf data/ bundles/*.yaml
```

### 1.8 ADR structure

File: `scenario-a/provisioning/docs/adr-001-cross-stack-enode-addressing.md`

Sections:
1. **Status** — Draft → Accepted
2. **Context** — `cbweb3_network` single-host limitation; `docker inspect` IP hack
3. **Decision** — three-profile addressing matrix (D1–D3 from research.md)
4. **Bundle format** — mandatory fields, guards against `172.x`/secrets
5. **`mode: found` constraints** — genesis-once, `admin_nodeInfo` after NAT, emits bundle
6. **`mode: join` constraints** — consumes bundle, `--bootnodes` from bundle, no genesis regen
7. **Cacti / relay** — `host.docker.internal` accepted in local profile only; prod uses public DNS
8. **Rejected alternatives**:
   - Shared `cbweb3_network` as default → not scalable to separate hosts
   - `docker inspect` IP rewrite → ephemeral, host-only, breaks on restart
   - Inferring `advertisedHost` from co-location → violates constitution §4 (explicit, not inferred)
9. **Evidence** — T1–T6 script outputs; cross-platform notes (Mac vs Linux)
10. **Constitution Check** — Principle I (scope), Principle V (test-first), Principle VI (no silent failures)
11. **Downstream impact** — table mapping to TK-1, TK-4, TK-6, RL-1

---

## Delivery Checklist

- [ ] `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/` scaffolded with all files
- [ ] `make spk01.up` succeeds on Linux
- [ ] `make spk01.verify` passes T1–T6 on Linux
- [ ] `make spk01.verify` passes T1–T6 on Docker Desktop (Mac)
- [ ] ADR written with all sections, evidence linked, cross-platform differences documented
- [ ] No files under `scenario-a/deploy/local/` or `scenario-a/make/` modified
- [ ] No `hyperledger/besu:latest` in any spike compose file (must be `:25.8.0`)
- [ ] All bundle files pass the secrets-scan guard
- [ ] PR includes Constitution Check section referencing this plan
