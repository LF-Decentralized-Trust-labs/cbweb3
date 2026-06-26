# Tasks: SP02 — Live Spoke Join (QBFT + Paladin)

**Input**: Design documents from `specs/019-spk02-live-join/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅
**Branch**: `019-spk02-live-join`

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1–US4)
- All paths relative to repo root

---

## Phase 1: Setup (Project Scaffold)

**Purpose**: Create the spike directory structure and shared infrastructure. Nothing runs yet.

- [x] T001 Create spike directory tree: `scenario-a/provisioning/spikes/spk-02-live-join/{compose,scripts,data,bundles,docs}` and add `.gitignore` for `data/` and `bundles/*.yaml`
- [x] T002 Create `scenario-a/provisioning/spikes/spk-02-live-join/Makefile` with declared targets: `spk02.up` (found+bundle+join), `spk02.found`, `spk02.bundle`, `spk02.join`, `spk02.vote`, `spk02.paladin-join`, `spk02.verify`, `spk02.down`, `spk02.clean` — mirror SP01's Makefile pattern
- [x] T003 [P] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/env-defaults.sh` — define port constants in the 9xxx range (P2P: 32303–32306, RPC: 9645–9646, WS: 9655–9656, Paladin RPC: 9648–9650, Paladin gRPC: 9700–9702); include container name constants for all 7 containers

**Checkpoint**: Directory scaffolded. `make spk02.up` exists but all targets are stubs.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Genesis, Besu keys, Paladin contract deployment, TLS certs, and Paladin configs.
Must complete before any user story phase can begin.

**⚠️ CRITICAL**: US1 and US2 both depend on this phase completing.

- [x] T004 Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/genesis-once.sh` — adapt SP01's genesis-once.sh to add `"qbft": {"epochlength": 30}` in the QBFT config block; include key generation for 4 nodes (key0–key3); generate genesis only if not already present; exit 0 if already exists with a skip message
- [x] T005 [P] Create `scenario-a/provisioning/spikes/spk-02-live-join/compose/stack-found.yml` — 3 Besu validators (besu-boot, besu-v1, besu-v2) on `spk02_found_net`; use `--nat-method=DOCKER`; publish host ports from env-defaults.sh; mount key0–key2; include Paladin CB (`spk02-paladin-cb`) and Paladin Bank-A (`spk02-paladin-bank-a`) services on the same network; add `extra_hosts: host.docker.internal:host-gateway` to Paladin containers
- [x] T006 [P] Create `scenario-a/provisioning/spikes/spk-02-live-join/compose/stack-join.yml` — Besu joiner (besu-joiner) on isolated `spk02_join_net`; `--nat-method=DOCKER`; `--bootnodes=${BOOTNODE_ENODE}` from env; publish RPC on 9646; include Paladin Bank-X (`spk02-paladin-bank-x`) on `spk02_join_net`; add `extra_hosts: host.docker.internal:host-gateway`
- [x] T007 Create Paladin config templates `scenario-a/provisioning/spikes/spk-02-live-join/data/paladin/{cb,bank-a,bank-x}/config.yaml.tmpl` — adapt from `scenario-a/deploy/local/paladin/spoke-a/config/` templates; set `nodeName` to `spoke-spk02-cb`, `spoke-spk02-bank-a`, `spoke-spk02-bank-x`; blockchain URLs via `host.docker.internal` + spike RPC ports; gRPC `externalHostname` matching compose service names; `contractAddress: "${REGISTRY_CONTRACT_ADDRESS}"`
- [x] T008 [P] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/generate-paladin-certs.sh` — adapt SP01's `generate-certs.sh`; generate self-signed P-256 certs for cb (`spk02-paladin-cb`), bank-a (`spk02-paladin-bank-a`), bank-x (`spk02-paladin-bank-x`); output to `data/paladin/{cb,bank-a,bank-x}/{tls.crt,tls.key}`; idempotent (skip if cert already exists)
- [x] T009 Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/render-paladin-configs.sh` — read `REGISTRY_CONTRACT_ADDRESS` from `data/.deployed-addrs.env`; use `envsubst` to produce `config.yaml` from each `.tmpl`; accept `NODE=cb|bank-a|bank-x` argument; write rendered configs to `data/paladin/<node>/config.yaml`
- [x] T010 Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/start-found.sh` — orchestrates the full found-stack boot: (1) call `genesis-once.sh`, (2) `docker compose -f compose/stack-found.yml up -d besu-boot besu-v1 besu-v2`, (3) wait for RPC on port 9645, (4) invoke `paladin.deploy-registry-spoke-a`-equivalent Go test pointing to `BESU_RPC_URL=http://localhost:9645` and write `REGISTRY_CONTRACT_ADDRESS` to `data/.deployed-addrs.env`, (5) call `generate-paladin-certs.sh`, (6) call `render-paladin-configs.sh` for cb and bank-a, (7) register CB + Bank-A on-chain via Go test, (8) bring up Paladin CB and Bank-A via `docker compose up -d`, (9) wait for Paladin CB RPC on 9648
- [x] T011 Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/extract-bundle.sh` — adapt SP01's extract-bundle.sh; call `admin_nodeInfo` on bootnode RPC 9645; write `bundles/spoke-spk02.bundle.yaml` including `bootnodeEnode`, `rpcEndpoint`, `wsEndpoint`, `genesisHash`, `trustAnchor` (empty for spike), `contracts.identityRegistry`; fail if enode contains `172.x` or `127.0.0.1`

**Checkpoint**: `make spk02.found && make spk02.bundle` completes without error; Besu produces blocks; Paladin CB + Bank-A are running; bundle file exists.

---

## Phase 3: User Story 1 — QBFT Validator Promotion (Priority: P1)

**Goal**: Promote a synced non-validator to active QBFT validator while the network continues producing blocks without interruption.

**Independent Test**: Start found stack; start joiner; vote from 2 of 3 validators; run `make spk02.verify` and confirm T1 + T2 pass.

### Implementation for User Story 1

- [x] T012 [US1] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/join-besu.sh` — read `BOOTNODE_ENODE` from bundle via `yq`; export it; run `docker compose -f compose/stack-join.yml up -d besu-joiner`; wait for RPC on 9646; print current block height of joiner and found-stack bootnode; loop until heights are within ±5 blocks (max 60s); exit 1 if not converged
- [x] T013 [US1] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/vote-in-validator.sh` — (1) extract joiner EVM address from `eth_coinbase` on RPC 9646, (2) call `qbft_proposeValidatorVote(joinerAddr, true)` on all 3 existing validator RPCs (9645, and the V1/V2 ports configured in env-defaults.sh), (3) print each vote response, (4) poll `qbft_getValidatorsByBlockNumber("latest")` every 2s for up to 120s until the joiner address appears, (5) print the block number at which activation occurred and calculate which epoch boundary it corresponds to
- [x] T014 [US1] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/verify-qbft.sh` — implements T1 and T2:
  - **T1**: call `qbft_getValidatorsByBlockNumber("latest")` on bootnode RPC; assert exactly 4 addresses returned; assert joiner address is in the list; emit `PASS T1 qbft-validator-set validators=4 joiner=<addr>` or `FAIL T1 <reason>`
  - **T2**: read 20 consecutive block headers from `eth_getBlockByNumber` on bootnode; compute max gap between consecutive block timestamps; assert max gap ≤ 2 × `blockperiodseconds` (default 2s → max gap ≤ 4s); emit `PASS T2 block-continuity max_gap=<N>s` or `FAIL T2 max_gap=<N>s threshold=4s`

**Checkpoint**: `make spk02.vote` succeeds; `verify-qbft.sh` exits 0; T1 + T2 emit PASS.

---

## Phase 4: User Story 2 — Paladin Live Join (Priority: P1)

**Goal**: Determine whether adding a new Paladin node to a running spoke requires existing nodes to restart. Produce verified evidence for the verdict.

**Independent Test**: With found stack running (Paladin CB + Bank-A active), run `join-paladin.sh` and then `verify-paladin-join.sh`; observe T3 (mTLS success or x509 failure) and T4 (no restart of existing containers).

### Implementation for User Story 2

- [x] T015 [US2] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/record-container-start-times.sh` — call `docker inspect --format '{{.State.StartedAt}}' <container>` for each of the 5 existing containers (besu-boot, besu-v1, besu-v2, spk02-paladin-cb, spk02-paladin-bank-a); write results to `data/baseline-start-times.json` as `{"<container>": "<ISO timestamp>"}` ; also write current timestamp to `data/baseline-captured-at.json`
- [x] T016 [US2] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/join-paladin.sh` — (1) call `generate-paladin-certs.sh bank-x`, (2) call `render-paladin-configs.sh bank-x`, (3) run `docker compose -f compose/stack-join.yml up -d paladin-bank-x`, (4) wait for Paladin Bank-X RPC on 9650, (5) invoke Go test `TestRegisterPaladinNodes` pointing to `BESU_RPC_URL=http://localhost:9646` and `PALADIN_URL=http://localhost:9650` to register Bank-X on-chain, (6) wait 15s for event propagation, (7) print Bank-X nodeName and registered endpoint
- [x] T017 [US2] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/verify-paladin-join.sh` — implements T3 and T4:
  - **T3 (mTLS probe)**: issue a JSON-RPC call from CB Paladin (9648) that requires routing to Bank-X (e.g., `ptx_resolveVerifier` with Bank-X nodeName as target); check for success vs `x509` / `transport` error in response; emit `PASS T3 paladin-mtls-no-restart` or `FAIL T3 paladin-mtls-x509-restart-required error=<msg>`
  - **T4 (no restart check for Paladin containers)**: compare current `docker inspect StartedAt` for `spk02-paladin-cb` and `spk02-paladin-bank-a` against values in `data/baseline-start-times.json`; emit `PASS T4 no-existing-paladin-restart` or `FAIL T4 restart-detected container=<name> before=<T> after=<T>`
- [x] T018 [US2] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/verify-restart-scope.sh` — implements T5:
  - Compare `docker inspect StartedAt` for all 3 Besu validator containers against baseline; emit `PASS T5 no-besu-validator-restart` or `FAIL T5 restart-detected container=<name>`
  - Compare `docker inspect StartedAt` for all 5 existing containers (both Besu + Paladin) to confirm none restarted during the entire join sequence

**Checkpoint**: `make spk02.paladin-join` runs; T3 resolves the TLS verdict; T4 + T5 confirm no unwanted restarts. The verdict (restart required / not required) is now known and evidenced in logs.

---

## Phase 5: User Story 3 — Restart Path (Priority: P1, conditional)

**Goal**: If T3 in Phase 4 produced a `FAIL` (x509 error), produce a documented rolling-restart procedure and re-validate that after the restart the cross-node operation succeeds.

**Independent Test**: This phase is only executed if T3 failed. After `rolling-restart.sh` runs, T3 is re-run; it must now emit PASS. T4 must emit FAIL (restart detected — expected and documented).

### Implementation for User Story 3 (execute only if T3 failed)

- [x] T019 [US3] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/rolling-restart.sh` — (1) append Bank-X TLS cert to the `tls.caFile` config or `trustedCerts` list for CB and Bank-A Paladin configs (determine exact config field from Paladin docs during POC); (2) restart `spk02-paladin-cb`: `docker compose -f compose/stack-found.yml restart paladin-cb`; wait for Paladin CB RPC health on 9648; (3) restart `spk02-paladin-bank-a`: same pattern; wait for health on 9649; (4) record restart duration per node; (5) emit structured output: `RESTART cb start=<T> end=<T> duration=<N>s healthy=true`
- [x] T020 [US3] Update `scenario-a/provisioning/spikes/spk-02-live-join/scripts/join-paladin.sh` to call `rolling-restart.sh` as a conditional step: check the T3 verdict from `verify-paladin-join.sh`; if T3 failed with x509, run `rolling-restart.sh` automatically and re-run T3 probe; print a clear operator notice: `[SP02] TLS restart required: rolling restart of CB + Bank-A Paladin completed (<N>s total)`
- [x] T021 [US3] Add `spk02.rolling-restart` target to Makefile that runs `rolling-restart.sh` standalone (for operator reference in toolkit documentation)

**Checkpoint** (if this phase ran): T3 re-probe passes after rolling restart; T4 shows expected restart of CB + Bank-A; T5 still shows no Besu restarts. Rolling restart duration is recorded.

---

## Phase 6: User Story 4 — ADR-002 and Verification Suite (Priority: P1)

**Goal**: Produce a complete ADR-002 with all evidence fields populated and a verification script that confirms the ADR table is filled.

**Independent Test**: After the full POC runs (`make spk02.up`), run `verify-adr-evidence.sh`; it must exit 0.

### Implementation for User Story 4

- [x] T022 [P] [US4] Create `scenario-a/provisioning/spikes/spk-02-live-join/docs/adr-002-live-validator-join.md` — initial ADR structure with all sections: Context (what SP01 left open), Decisions (D1–D5 as stubs), Evidence Table (T1–T6 rows with `[PENDING]` status), Rejected Alternatives (placeholder), Downstream Impact (TK-x items), Constitution Check; mark as **Status: Draft — pending POC execution**
- [x] T023 [US4] Create `scenario-a/provisioning/spikes/spk-02-live-join/scripts/verify-adr-evidence.sh` — implements T6: scan `docs/adr-002-live-validator-join.md` for the evidence table; assert all 6 rows (T1–T5 + ADR-completeness) contain a non-`[PENDING]` status string; emit `PASS T6 adr-evidence-complete rows=6` or `FAIL T6 adr-evidence-incomplete missing=<ids>`
- [ ] T024 [US4] After the full POC runs end-to-end, populate ADR-002 with actual evidence: fill D1 (epoch length + activation block observed), D2 (TLS verdict + log excerpt), D3 (discovery mode + log excerpt), D4 (validated step sequence from Makefile), D5 (restart procedure if applicable); update Status to **Accepted**; fill all T1–T6 rows with observed values

**Checkpoint**: `verify-adr-evidence.sh` exits 0. ADR-002 Status is **Accepted**. All PENDING stubs are replaced with real evidence.

---

## Phase 7: Polish & Integration

**Purpose**: Wire everything together into `make spk02.up && make spk02.verify`, write the README, and confirm the single-command experience.

- [x] T025 Fill in all Makefile stub targets with their actual script calls and dependencies: `spk02.up` → `spk02.found + spk02.bundle + spk02.join`; `spk02.verify` → calls all 6 verify scripts in sequence; `spk02.down` → `docker compose down` both stacks + Paladin; `spk02.clean` → down + remove `data/` + `bundles/*.yaml`
- [x] T026 Create `scenario-a/provisioning/spikes/spk-02-live-join/README.md` — include: prerequisites, quick start (`make spk02.up && make spk02.verify`), port table (all 7 containers with host ports), test criteria table (T1–T6 with script, description, and pass condition), known platform differences (Linux vs Mac TLS + port check tools), TLS verdict summary box (filled after POC runs), link to ADR-002
- [ ] T027 Run `make spk02.up && make spk02.verify` end-to-end on Linux; confirm all T1–T6 emit PASS; fix any script bugs found during the run; record the actual epoch boundary block number and TLS verdict in `data/poc-evidence.txt` for reference

---

## Dependencies (User Story Completion Order)

```
Phase 1 (Setup)
    └── Phase 2 (Foundational)
          ├── Phase 3 (US1 — QBFT)
          │     └── Phase 6 (US4 — ADR, partial: T1+T2 evidence)
          └── Phase 4 (US2 — Paladin)
                ├── Phase 5 (US3 — Restart path, conditional on T3 verdict)
                │     └── Phase 6 (US4 — ADR, partial: T3+T4 evidence + D2/D3)
                └── Phase 6 (US4 — ADR, T5 evidence)
                      └── Phase 7 (Polish)
```

Phase 3 and Phase 4 can start independently after Phase 2 completes.
Phase 5 only executes if Phase 4's T3 check fails (TLS x509 error detected).
Phase 6 (T022 draft ADR + T023 verify script) can be written in parallel with Phases 3–4.
Phase 6 (T024 fill evidence) must wait for Phases 3 + 4 + 5 to complete.

---

## Parallel Execution

**After Phase 2 completes**, these can run in parallel:
- `[P]` T005 (stack-found.yml) ← already done in Phase 2
- Phase 3 implementation (T012–T014) and Phase 4 implementation (T015–T018) can proceed simultaneously if two engineers are working
- T022 (ADR draft) and T023 (verify-adr-evidence.sh) can be written at any time after Phase 1

---

## Implementation Strategy

**MVP scope**: Phases 1–3 (T001–T014 + T022–T023 stubs). This delivers the QBFT voting
answer — the more tractable of the two unknowns — and produces T1+T2 evidence for ADR-002.
Phase 4 (Paladin TLS investigation) is the riskier path and should follow immediately.

**Total tasks**: 27
- Phase 1 (Setup): 3 tasks
- Phase 2 (Foundational): 8 tasks
- Phase 3 (US1): 3 tasks
- Phase 4 (US2): 4 tasks
- Phase 5 (US3, conditional): 3 tasks
- Phase 6 (US4): 3 tasks
- Phase 7 (Polish): 3 tasks

**Parallelizable tasks**: T003, T005, T006, T007, T008, T022

---

## Phase 8: Convergence

- [x] T028 CRITICAL: Expose RPC ports for besu-v1 and besu-v2 in `scenario-a/provisioning/spikes/spk-02-live-join/compose/stack-found.yml` (add `HOST_RPC_V1:8545` and `HOST_RPC_V2:8545` port mappings) and add `HOST_RPC_V1` / `HOST_RPC_V2` variables to `scripts/env-defaults.sh` (e.g., 9647 and 9648-offset); then rewrite `scripts/vote-in-validator.sh` to call `qbft_proposeValidatorVote` on V1's RPC and V2's RPC independently so that 2 distinct validator votes are cast — the current implementation submits 3 votes from the single bootnode RPC, which counts as 1 vote and will never reach the floor(3/2)+1=2 threshold per FR-001, US1/AC1 (missing)
- [x] T029 Implement actual on-chain Paladin node registration in `scripts/start-found.sh` (for spoke-spk02-cb and spoke-spk02-bank-a) and `scripts/join-paladin.sh` (for spoke-spk02-bank-x) — either by parameterizing the existing `TestRegisterPaladinNodes` Go test to accept custom nodeName/endpoint values, or by calling the Paladin REST API `POST /api/v1/registries/evm-registry/entries` directly with the spike node names; without registration Bank-X has no on-chain entry for existing nodes to discover, making T3 invalid per FR-004, US2/AC1 (missing)
- [x] T030 Fix `scripts/verify-qbft.sh` T2: replace the hardcoded `seq 1 20` block loop (which reads genesis blocks 1–20 before any voting occurred) with a dynamic range reading the 20 most recent blocks at call time — use `eth_blockNumber` to get the current head and then iterate `[head-19 .. head]`; this ensures T2 measures block continuity during and after the voting window, not before it per FR-003, US1/AC3 (partial)
- [x] T031 Fix `scripts/verify-paladin-join.sh` T3: replace the `ptx_getTransaction` probe (which resolves locally on CB without cross-node routing) with a call that forces mTLS transport between CB and Bank-X — use `POST /api/v1/registries/evm-registry/entries?nodeName=spoke-spk02-bank-x` on the CB Paladin REST API (port 9648), which requires the CB to query the registry and then resolve the transport endpoint of Bank-X via gRPC; if the response contains Bank-X's endpoint, mTLS succeeded; if it contains `x509` or `connection refused`, restart is required per US2/AC1, T3 (partial)

---

## Phase 9: Convergence

- [x] T032 CRITICAL: Add `../data/key3:/opt/besu/data/key:ro` volume mount and `--node-private-key-file=/opt/besu/data/key` flag to the `besu-joiner` service in `scenario-a/provisioning/spikes/spk-02-live-join/compose/stack-join.yml` — without this, Besu auto-generates a random ephemeral key in `besu_joiner_data` volume; `vote-in-validator.sh` votes for the address derived from `data/key3` which never matches the joiner's actual running address, making T1 (and therefore US1) permanently fail per FR-001, US1/AC1 (contradicts)
- [x] T033 Extend `scenario-a/provisioning/spikes/spk-02-live-join/Makefile` `spk02.up` target to include `spk02.vote` and `spk02.paladin-join` after `spk02.join` so that `make spk02.up && make spk02.verify` is the single complete command SC-007 requires — current definition `spk02.up: spk02.found spk02.bundle spk02.join` leaves QBFT voting and Paladin join as manual steps; T1–T3 will fail if omitted per SC-007, FR-007 (partial)
- [x] T034 Wire `scripts/record-container-start-times.sh` into the orchestration sequence: call it from `scripts/join-besu.sh` (or as a separate `spk02.baseline` Makefile target invoked between `spk02.join` and `spk02.vote`) after the Besu joiner starts but before any vote or Paladin join — without this call `data/baseline-start-times.json` is never written and `verify-paladin-join.sh --t4` (T4) and `verify-restart-scope.sh` (T5) always exit with "baseline not found" per FR-007, US2/AC1 (missing)
- [x] T035 Add `export PATH="${HOME}/.foundry/bin:${PATH}"` to `scripts/env-defaults.sh` (or at the top of `scripts/register-paladin-nodes.sh` before the first `cast` call) so that Foundry's `cast` binary is discoverable when scripts are invoked via `make`; on Linux systems where Foundry is user-local, the Make-inherited PATH typically does not include `~/.foundry/bin` per FR-004 (partial)
- [x] T036 Update `scenario-a/provisioning/spikes/spk-02-live-join/README.md` port table: set `spk02-besu-v1` Host RPC to `9643` and `spk02-besu-v2` Host RPC to `9644` (currently shown as `—`; T028 added these ports in `stack-found.yml` and `env-defaults.sh`); also add `cast` (Foundry 1.5+) to the Prerequisites section per SC-007 (partial)
