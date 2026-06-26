# Tasks: SP01 — Cross-Stack Enode Addressing

**Input**: Design documents from `specs/018-spk01-cross-stack-enode/`  
**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅ data-model.md ✅

**Format**: `[ID] [P?] [Story?] Description — file path`  
- **[P]**: Parallelizable (different files, no dependency conflicts)  
- **[US#]**: User Story (maps to spec.md)  
- Verification scripts are the tests — written before the implementation they validate (test-first)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Scaffold the directory tree and Makefile skeleton. Nothing executes yet.

- [X] T001 Create directory structure: `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/{compose,scripts,bundles}` and `scenario-a/provisioning/docs/`
- [X] T002 [P] Create `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/bundles/.gitkeep`
- [X] T003 [P] Create `.gitignore` in spike root to exclude `data/` and `bundles/*.yaml` (generated; never committed) — `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/.gitignore`
- [X] T004 Create `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/Makefile` with all seven targets as stubs: `spk01.up`, `spk01.found`, `spk01.bundle`, `spk01.join`, `spk01.verify`, `spk01.down`, `spk01.clean`
- [X] T005 Create `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/env-defaults.sh` exporting all port variables with defaults: `HOST_P2P_BOOT=31303`, `HOST_RPC_BOOT=8645`, `HOST_WS_BOOT=8655`, `HOST_P2P_V1=31304`, `HOST_P2P_V2=31305`, `HOST_P2P_JOIN=31306`, `HOST_RPC_JOIN=8646`, `HOST_WS_JOIN=8656`, `SPOKE_ID=spoke-spk01`

**Checkpoint**: Directory tree exists; `make spk01.up` runs but exits with "not implemented" on all targets.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: `wait-for-blocks.sh` helper used by multiple user stories, plus the QBFT genesis config template that all stacks share.

- [X] T006 Create `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/wait-for-blocks.sh` — polls `eth_blockNumber` on a given RPC URL until block > 0 or timeout; exits non-zero on timeout. Used by `start-found.sh` and `start-join.sh`.
- [X] T007 Create `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/compose/genesis-config.json` — QBFT config template for 3 validators (chainId 1337, blockperiodseconds 2, requesttimeoutseconds 4, epoch 30000). This is the input to `besu operator generate-blockchain-config`.

**Checkpoint**: Helper scripts exist; both are independently testable (`bash scripts/wait-for-blocks.sh http://localhost:9999` exits non-zero on unreachable host).

---

## Phase 3: User Story 5 — Genesis Generated Exactly Once (Priority: P1) 🎯 Prerequisite

**Goal**: `genesis-once.sh` creates genesis on first run and skips silently on subsequent runs. This is the foundational idempotency pattern for the whole toolkit.

**Why P1 before US1**: The found stack compose file mounts the genesis produced by `genesis-once.sh`. US1 cannot be started without it.

**Independent Test (T1)**: `bash scripts/genesis-once.sh ./data/genesis.json` twice → second run prints "SKIP", `sha256sum` unchanged.

### Verification for US5 (write first — must FAIL before implementation)

- [X] T008 [US5] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/verify-genesis-idempotency.sh` — runs `genesis-once.sh` twice, computes SHA-256 before and after second run, fails with `FAIL T1` if checksums differ or if "SKIP" not in second run output; passes with `PASS T1 genesis-idempotency checksum=<hash> unchanged`

### Implementation for US5

- [X] T009 [US5] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/genesis-once.sh` — checks if `$1` (genesis file path) exists; if yes prints `[genesis-once] SKIP: genesis already exists at <path>` and exits 0; if no: calls `docker run --rm -v $(pwd):/work hyperledger/besu:25.8.0 operator generate-blockchain-config --config-file=/work/compose/genesis-config.json --to=/work/data/networkFiles --private-key-file-name=key`, copies genesis and node keys to `data/`, removes temp dir.
- [X] T010 [US5] Wire T1 into Makefile: `spk01.verify` target calls `bash scripts/verify-genesis-idempotency.sh`; `spk01.found` target calls `genesis-once.sh ./data/genesis.json` as first step.

**Checkpoint (US5)**: `bash scripts/verify-genesis-idempotency.sh` prints `PASS T1` → User Story 5 complete.

---

## Phase 4: User Story 1 — Operator Founds a Spoke with Stable P2P Address (Priority: P1) 🎯 MVP

**Goal**: `stack-found.yml` starts bootnode + 2 validators. Every node announces an enode with a host-level address (not `172.x.x.x`). Block production confirmed. Verification scripts T2 and T3 pass.

**Independent Test**: `make spk01.found && bash scripts/verify-enode.sh && bash scripts/verify-peering.sh found` → T2 + T3 PASS.

### Verification for US1 (write first — T3 MUST FAIL before correct Besu flags are in compose)

- [X] T011 [P] [US1] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/verify-enode.sh` — calls `admin_nodeInfo` on `${HOST_RPC:-http://localhost:8645}`, extracts `.result.enode` via `jq`; fails with `FAIL T3 enode-audit enode=<enode> CONTAINS_PRIVATE_IP` if enode matches `172\.(1[6-9]|2[0-9]|3[01])\.` or `127\.0\.0\.1`; passes with `PASS T3 enode-audit enode=<enode>`.
- [X] T012 [P] [US1] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/verify-peering.sh` — accepts mode arg (`found` or `join`); for `found`: calls `eth_blockNumber` on all three found-stack RPC ports and `qbft_getValidators` on bootnode; fails if block = 0 or validator count ≠ 3; prints `PASS T2 block-production blockHeight=<n>`. For `join`: calls `admin_peers` on joiner RPC; fails if peers < 1; prints `PASS T4 cross-stack-peering peers=<n>`.

### Implementation for US1

- [X] T013 [US1] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/compose/stack-found.yml` — three services (`besu-boot`, `besu-v1`, `besu-v2`) on network `spk01_found_net`; image `hyperledger/besu:25.8.0`; bootnode flags: `--nat-method=DOCKER --p2p-port=30303 --rpc-http-api=ETH,NET,QBFT,ADMIN`; ports `${HOST_P2P_BOOT:-31303}:30303/tcp+udp`, `${HOST_RPC_BOOT:-8645}:8545`, `${HOST_WS_BOOT:-8655}:8546`; validators bootnodes set to `enode://<pubkey>@besu-boot:30303` (intra-stack DNS); genesis and node-key volumes mounted from `./data/`; NO connection to `cbweb3_network`.
- [X] T014 [US1] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/start-found.sh` — sources `env-defaults.sh`; calls `genesis-once.sh ./data/genesis.json`; extracts node pubkeys from `data/networkFiles/keys/` to derive each node's `--node-private-key-file`; writes per-validator bootnodes static-nodes file; runs `docker compose -f compose/stack-found.yml up -d`; calls `wait-for-blocks.sh http://localhost:${HOST_RPC_BOOT}` with 60s timeout.
- [X] T015 [US1] Wire T2+T3 into Makefile: `spk01.verify` calls `verify-enode.sh` then `verify-peering.sh found`; `spk01.found` calls `start-found.sh`.

**Checkpoint (US1)**: `make spk01.found && make spk01.verify` → T2 `PASS` (blocks > 0, 3 validators) and T3 `PASS` (no `172.x` in enode). User Story 1 complete.

---

## Phase 5: User Story 2 — Operator Publishes Join Bundle with No Secrets (Priority: P1)

**Goal**: `extract-bundle.sh` emits `bundles/spoke-spk01.bundle.yaml` with stable enode, genesis hash, RPC/WS endpoints, and CA cert. Bundle passes automated secrets scan. Verification T6 passes.

**Independent Test**: `make spk01.bundle` + `bash scripts/verify-bundle.sh bundles/spoke-spk01.bundle.yaml` → T6 `PASS`.

### Verification for US2 (write first)

- [X] T016 [P] [US2] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/verify-bundle.sh` — reads `$1` (bundle file); fails with `FAIL T6 bundle-clean PRIVATE_IP` if any value matches `172\.(1[6-9]|2[0-9]|3[01])\.` or `127\.0\.0\.1`; fails with `FAIL T6 bundle-clean PRIVATE_KEY` if any line matches `-----BEGIN.*(PRIVATE|ENCRYPTED).*KEY`; asserts required fields present (`spec.p2p.bootnodeEnode`, `spec.rpc.endpoint`, `spec.rpc.wsEndpoint`, `spec.chain.genesisHash`); passes with `PASS T6 bundle-clean fields=ok secrets=none`.

### Implementation for US2

- [X] T017 [US2] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/extract-bundle.sh` — sources `env-defaults.sh`; calls `admin_nodeInfo` on `http://localhost:${HOST_RPC_BOOT}`; extracts `.result.enode`; runs `verify-enode.sh` inline (fails fast if enode is private); computes `sha256sum` of `./data/genesis.json`; reads trust anchor from `./data/ca.crt` if present (else empty); writes `bundles/${SPOKE_ID}.bundle.yaml` following the schema in `data-model.md` (apiVersion, kind, metadata.spokeId, spec.p2p.bootnodeEnode, spec.rpc.endpoint, spec.rpc.wsEndpoint, spec.chain.genesisHash, spec.pki.trustAnchor, spec.contracts empty); calls `verify-bundle.sh` at the end.
- [X] T018 [US2] Wire into Makefile: `spk01.bundle` calls `extract-bundle.sh`; `spk01.verify` calls `verify-bundle.sh bundles/${SPOKE_ID}.bundle.yaml`; `spk01.up` chains `spk01.found → spk01.bundle`.

**Checkpoint (US2)**: `make spk01.bundle` emits bundle, `bash scripts/verify-bundle.sh bundles/spoke-spk01.bundle.yaml` prints `PASS T6`. Bundle can be `git add`ed safely. User Story 2 complete.

---

## Phase 6: User Story 3 — Participant Joins Spoke from Separate Stack (Priority: P1)

**Goal**: `stack-join.yml` starts a joiner Besu node on a network with ZERO shared containers with `spk01_found_net`. Joiner connects via host-published enode from bundle and reaches block-height parity. T4 passes.

**Independent Test**: `make spk01.join` + `bash scripts/verify-peering.sh join` → T4 `PASS` (peers ≥ 1, block convergence).

### Implementation for US3

- [X] T019 [US3] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/compose/stack-join.yml` — one service (`besu-joiner`) on network `spk01_join_net` (distinct from `spk01_found_net`); image `hyperledger/besu:25.8.0`; flags: `--nat-method=DOCKER --bootnodes=${BOOTNODE_ENODE} --p2p-port=30303`; ports `${HOST_P2P_JOIN:-31306}:30303/tcp+udp`, `${HOST_RPC_JOIN:-8646}:8545`, `${HOST_WS_JOIN:-8656}:8546`; `extra_hosts: ["host.docker.internal:host-gateway"]` (Linux compatibility); genesis volume mounted from `./data/genesis.json` (same genesis, read-only); NO connection to `spk01_found_net` or `cbweb3_network`.
- [X] T020 [US3] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/start-join.sh` — sources `env-defaults.sh`; reads `BOOTNODE_ENODE` from bundle file using `yq e '.spec.p2p.bootnodeEnode' bundles/${SPOKE_ID}.bundle.yaml` (or `python3 -c "import yaml,sys; print(yaml.safe_load(sys.stdin)['spec']['p2p']['bootnodeEnode'])"` as fallback if `yq` absent); exports `BOOTNODE_ENODE`; runs `docker compose -f compose/stack-join.yml up -d`; calls `wait-for-blocks.sh http://localhost:${HOST_RPC_JOIN}` with 90s timeout.
- [X] T021 [US3] Extend `verify-peering.sh` join mode (see T012) to also compare block height within ±2 of found stack; fails with `FAIL T4 cross-stack-peering BLOCK_LAG height_found=<n> height_join=<m>` if difference > 5 after 60s.
- [X] T022 [US3] Wire T4 into Makefile: `spk01.verify` calls `verify-peering.sh join`; `spk01.up` chains `spk01.found → spk01.bundle → spk01.join`; `spk01.down` tears down join then found.

**Checkpoint (US3)**: `make spk01.join && bash scripts/verify-peering.sh join` → T4 `PASS`. Docker network inspection confirms joiner has no interface on `spk01_found_net`. User Story 3 complete.

---

## Phase 7: User Story 4 — Relay Reaches Both Spokes via Explicit Configuration (Priority: P2)

**Goal**: A smoke-test container in an isolated network (`spk01_cacti_net`) reaches both found-stack and join-stack RPC endpoints using only explicit env-var URLs. T5 passes.

**Independent Test**: `docker compose -f compose/cacti-isolated.yml run --rm cacti-smoke bash -c 'bash /scripts/verify-cacti-rpc.sh'` → T5 `PASS`.

### Verification for US4 (write first)

- [X] T023 [P] [US4] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/verify-cacti-rpc.sh` — reads `SPOKE_FOUND_RPC` and `SPOKE_JOIN_RPC` from env; calls `eth_blockNumber` on each; fails with `FAIL T5 cacti-rpc-isolation UNREACHABLE url=<url>` if either call fails or returns block 0; passes with `PASS T5 cacti-rpc-isolation found=<n> join=<m> network=isolated`.

### Implementation for US4

- [X] T024 [US4] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/compose/cacti-isolated.yml` — one service (`cacti-smoke`) using `curlimages/curl:latest`; network `spk01_cacti_net` (no connection to found or join nets); `extra_hosts: ["host.docker.internal:host-gateway"]`; env vars: `SPOKE_FOUND_RPC=http://host.docker.internal:${HOST_RPC_BOOT:-8645}`, `SPOKE_JOIN_RPC=http://host.docker.internal:${HOST_RPC_JOIN:-8646}`; mounts `./scripts` read-only so the container can run `verify-cacti-rpc.sh`.
- [X] T025 [US4] Wire T5 into Makefile: `spk01.verify` calls `docker compose -f compose/cacti-isolated.yml run --rm cacti-smoke bash /scripts/verify-cacti-rpc.sh`; `spk01.down` includes `docker compose -f compose/cacti-isolated.yml down -v`.

**Checkpoint (US4)**: `docker compose -f compose/cacti-isolated.yml run --rm cacti-smoke` → T5 `PASS`. Network inspection confirms smoke container has no interface on `spk01_found_net` or `spk01_join_net`. User Story 4 complete.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: ADR writing, README, cross-platform validation, full end-to-end run, spike acceptance.

- [X] T026 [P] Write `scenario-a/provisioning/docs/adr-001-cross-stack-enode-addressing.md` — all sections from plan.md §1.8: Context, Decision (3-profile matrix from research.md D1–D3), Bundle format (data-model.md schema), `mode: found` constraints, `mode: join` constraints, Cacti/relay model, Rejected alternatives (shared network, `docker inspect` rewrite, co-location inference), Evidence (T1–T6 outputs), Constitution Check, Downstream impact (TK-1, TK-4, TK-6, RL-1)
- [X] T027 [P] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/README.md` — prerequisites (`docker`, `docker compose v2`, `jq`, `yq` or Python fallback), ports used (31303-31306, 8645-8646), how to run (`make spk01.up && make spk01.verify`), test criteria T1–T6, cleanup (`make spk01.clean`), known differences Mac vs Linux
- [X] T028 Run full end-to-end: `make spk01.up && make spk01.verify` → all six tests T1–T6 pass; capture output as evidence for ADR §Evidence
- [X] T029 Document Mac vs Linux NAT behavior differences in ADR — add section comparing `--nat-method=DOCKER` host IP on `colima`/Docker Desktop vs Linux Docker engine; add `extra_hosts: host-gateway` requirement note
- [X] T030 [P] Verify no files under `scenario-a/deploy/local/` or `scenario-a/make/` were modified: `git diff --name-only HEAD | grep -E 'deploy/local|make/' | wc -l` must output `0`

**Checkpoint (Final)**: `make spk01.up && make spk01.verify` exits 0 on Linux. ADR is reviewable. PR is ready.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1; blocks US5 and US1
- **US5 (Phase 3)**: Depends on Phase 2; produces `genesis-once.sh` which blocks US1
- **US1 (Phase 4)**: Depends on US5 complete + `genesis-once.sh` working; T011+T012 can be written in parallel before `start-found.sh`
- **US2 (Phase 5)**: Depends on US1 (found stack must be running for `admin_nodeInfo`)
- **US3 (Phase 6)**: Depends on US2 (needs bundle with `bootnodeEnode`)
- **US4 (Phase 7)**: Depends on US1 + US3 (needs both RPC ports available)
- **Polish (Phase 8)**: Depends on all user stories complete

### User Story Dependencies

```
Setup → Foundational → US5 → US1 → US2 → US3 → US4 → Polish
                                        ↗
                              (US4 also depends on US1)
```

### Parallel Opportunities (within phases)

- T011 + T012 (US1): `verify-enode.sh` and `stack-found.yml` touch different files — write in parallel
- T016 + T017 (US2): `verify-bundle.sh` can be written before `extract-bundle.sh`
- T023 + T024 (US4): `verify-cacti-rpc.sh` and `cacti-isolated.yml` are independent
- T026 + T027 (Polish): ADR and README are independent documents

---

## Parallel Example: User Story 1

```
Parallel — write verification before implementation:
  Task T011: Write verify-enode.sh          ← verify T3 FAILS without correct Besu flags
  Task T012: Write verify-peering.sh found  ← verify T2 FAILS before stack is up

Sequential — then implement:
  Task T013: Write stack-found.yml          ← correct --nat-method=DOCKER flags
  Task T014: Write start-found.sh           ← starts stack, waits for blocks
  Task T015: Wire Makefile                  ← connects all pieces

Validate:
  make spk01.found && make spk01.verify → T2+T3 GREEN
```

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. Complete Phase 1 (Setup) + Phase 2 (Foundational)
2. Complete Phase 3 (US5 — genesis-once)
3. Complete Phase 4 (US1 — found stack + stable enode)
4. **STOP and VALIDATE**: T2 + T3 passing proves the core design decision (D1 from research.md)
5. This is sufficient to unblock TK-4 (compose template design) in the toolkit

### Incremental Delivery

1. Setup + Foundational → skeleton exists
2. US5 → genesis idempotency proven (T1)
3. US1 → stable enode proven (T2 + T3) → **MVP: answers the spike's core question**
4. US2 → bundle format proven (T6) → enables `mode: join` design
5. US3 → cross-stack join proven (T4) → completes `mode: join` validation
6. US4 → relay isolation proven (T5) → confirms relay model
7. Polish → ADR + cross-platform → spike acceptance

---

## Notes

- Verification scripts are the tests for this spike; they follow the test-first principle: write them to FAIL before the implementation, then implement until they PASS
- `hyperledger/besu:latest` is explicitly forbidden in all compose files; always use `:25.8.0`
- `genesis-once.sh` must never call `docker inspect`; the enode rewrite pattern from `startBesu.sh` must not appear anywhere in the spike
- `yq` is preferred for bundle parsing but requires a `python3` fallback since it may not be available; document in README
- Bundle files in `bundles/*.yaml` are excluded from git via `.gitignore` except `.gitkeep`; the ADR references the schema only, not real bundle files

---

## Phase 9: Convergence

- [X] T031 [US3] Extend `verify-peering.sh` join mode to ALSO call `admin_peers` on the found-stack bootnode (`http://localhost:${HOST_RPC_BOOT}`) and assert the joiner's enode appears in the result; emit `FAIL T4 cross-stack-peering FOUND_SIDE_MISSING` if the joiner is absent — `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/verify-peering.sh` per US3/AC1 (partial)
- [X] T032 [US3] Write `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/verify-network-isolation.sh` — runs `docker network inspect spk01_found_net --format '{{json .Containers}}'`; fails with `FAIL T4 network-isolation JOINER_ON_FOUND_NET` if any container name contains `joiner`; passes with `PASS T4 network-isolation join_net_isolated`; exit non-zero if `spk01_found_net` does not exist per US3/AC3 (missing)
- [X] T033 [US3] Wire `verify-network-isolation.sh` into Makefile: `spk01.verify` calls it after `verify-peering.sh join`; `spk01.up` does not gate on it (it is a post-hoc check, not a startup step) — `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/Makefile` per US3/AC3 (missing)
- [X] T034 Add port-availability preflight to `start-found.sh` and `start-join.sh`: before `docker compose up`, check each host port (`HOST_P2P_BOOT`, `HOST_RPC_BOOT`, `HOST_WS_BOOT` / `HOST_P2P_JOIN`, `HOST_RPC_JOIN`) with `ss -tlnp | grep :<port>` (or `lsof -i :<port>` on Mac); if any port is bound, emit clear error `[start-found] ERROR: port <n> already in use — cannot start` and exit 1 — `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/start-found.sh` and `start-join.sh` per spec edge-case "port already bound" (missing)
- [X] T035 Add per-attempt log line to `wait-for-blocks.sh`: each poll iteration MUST emit `[wait-for-blocks] attempt <n>/<max> RPC=<url> blockHeight=<h>` to stdout so the caller can see retry state; on timeout emit `[wait-for-blocks] TIMEOUT after <n> attempts` before exiting non-zero — `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/scripts/wait-for-blocks.sh` per spec edge-case "offline enode / log retry state" (partial)
