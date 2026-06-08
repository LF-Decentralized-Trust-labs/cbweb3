---

description: "Task list for International Hub Network Isolation"
---

# Tasks: International Hub Network Isolation

**Input**: Design documents from `/specs/001-hub-network-isolation/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md (all present)

**Tests**: Included for User Story 3 only — Constitution Principle V ("Test-First at Every Layer") makes backend-service unit tests for new fallback/warning behavior NON-NEGOTIABLE ("The test MUST fail before the implementation is written"). No other story introduces new backend logic requiring new tests; existing Foundry/E2E suites are reused as validation gates, not rewritten.

**Organization**: Tasks are grouped by user story (priorities from spec.md: US1/US2/US3 = P1, US4 = P2, US5 = P3) to enable independent implementation and testing of each.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Maps task to a user story (US1–US5) for traceability
- All file paths are relative to the repository root

---

## Phase 1: Setup

**Purpose**: Decisions and inventories that every later phase depends on, but that aren't story-specific implementation work

- [ ] T001 Allocate and record dedicated `hub-besu` network parameters — `NETWORK_NAME=hub_besu_network`, `CONTAINER_PREFIX=cbweb3-hub-besu`, and an RPC/P2P/WS port range distinct from Spoke A's `86xx` and Spoke B's `87xx` ranges (inspect `scenario-b/deploy/local/spoke-besu-a/.env.network` and `scenario-b/deploy/local/spoke-besu-b/.env.network` for the existing ranges before choosing); note the chosen values for use in T005
- [ ] T002 [P] Re-verify the Shared Contract Suite list and the three Go call sites against current source — confirm `scenario-b/contracts/script/CBWeb3Hub.s.sol:DeployCBWeb3Hub` still deploys exactly the 9 contracts listed in `data-model.md` §2, and that `payment-orchestrator/cmd/payment-orchestrator/main.go:205`, `api-gateway/internal/app/app.go:234`, and `api-gateway/internal/app/identity_bootstrap.go` are still the only `HUB_CHAIN_ID`/Hub-chain-identity resolution points; record any drift from `research.md`/`contracts/service-configuration-contract.md` before proceeding

**Checkpoint**: Network parameters and the change inventory are confirmed accurate — implementation can begin

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Stand up the one artifact every other phase references — the `hub-besu` deployment directory and its RPC endpoint. US2, US3, and US4 all need this directory (and the endpoint it exposes) to exist before their own changes are meaningful.

**⚠️ CRITICAL**: No user story phase can be functionally completed (run + verified) until this phase is done — though US3's code-level edits can be written in parallel (see Dependencies below)

- [ ] T003 Create `scenario-b/deploy/local/hub-besu/` by copying the full directory structure of `scenario-b/deploy/local/spoke-besu-a/` (`.env.network`, `startBesu.sh`, `stopBesu.sh`, `addNewNode.sh`, `besu.autocomplete.sh`, `bin/`, `lib/`, `LICENSE`, `config/configTemplate.json`, `config/qbftConfigFile.json`, `genesis/`, `nodes/` templates) as the unmodified base to adapt in US1

**Checkpoint**: `hub-besu/` directory skeleton exists — User Story 1 can now adapt it into a running, independent network

---

## Phase 3: User Story 1 - Operator stands up an independent Hub network (Priority: P1) 🎯 MVP

**Goal**: Bring up the International Hub as its own Besu/QBFT network — distinct chain identity (`1337`), distinct topology, startable and runnable without Spoke A or Spoke B.

**Independent Test**: Start only the Hub network stack (no Spoke A, no Spoke B, no backend services). Confirm a validator node produces blocks and the RPC endpoint reports chain identity `1337`, with zero shared peers/genesis state with either spoke (quickstart.md Step 1).

### Implementation for User Story 1

- [ ] T004 [US1] Edit `scenario-b/deploy/local/hub-besu/config/configTemplate.json`: set `chainId: 1337` and mirror Spoke A/B's QBFT genesis parameters exactly — `blockperiodseconds: 2`, `epochlength: 30000`, `requesttimeoutseconds: 4`, `gasLimit: "0x1c9c380"`, `zeroBaseFee: true` (per `research.md` Decision 2 / `contracts/hub-network-rpc-contract.md` §3 parity table)
- [ ] T005 [US1] Edit `scenario-b/deploy/local/hub-besu/.env.network`: set `NETWORK_NAME`, `CONTAINER_PREFIX`, `NODES=1` (single-validator sandbox per `research.md` Decision 3), and the port assignments decided in T001
- [ ] T006 [US1] Adapt `scenario-b/deploy/local/hub-besu/startBesu.sh` and `stopBesu.sh` for a single-validator/bootnode topology (mirroring the `central-bank-a` bootnode role in `spoke-besu-a/startBesu.sh`) — remove or no-op the multi-node peer-bootstrap steps that assume 3 nodes
- [ ] T007 [US1] Add `deploy.up-hub-besu` and `deploy.down-hub-besu` targets to `scenario-b/make/10-deploy.mk` (mirroring `deploy.up-spoke-a`/`deploy.down-spoke-a`, `cd`-ing into `hub-besu/` and invoking its `startBesu.sh`/`stopBesu.sh`); update `deploy.up-besu` to depend on `deploy.up-hub-besu` alongside the existing spoke targets
- [ ] T008 [US1] Update `scenario-b/make/60-scenario-b.mk` so `scenario-b.up-infra` brings up the Hub network as part of `deploy.up-besu` (verify the dependency chain reaches the new target — no separate edit needed if T007's `deploy.up-besu` change is sufficient; otherwise add an explicit dependency)
- [ ] T009 [US1] Validate independently: run `make deploy.up-hub-besu` with Spoke A/B stopped; confirm via `eth_chainId` (expect `0x539`) and repeated `eth_blockNumber` calls that the Hub produces blocks; confirm via `admin_peers`/genesis-hash comparison that it shares no peers or genesis state with Spoke A or Spoke B (quickstart.md Step 1 / `contracts/hub-network-rpc-contract.md` §1–2)

**Checkpoint**: The Hub runs as a fully independent network reporting chain `1337` — this is the MVP. It can be demoed on its own.

---

## Phase 4: User Story 2 - Engineer deploys shared contracts to the neutral Hub (Priority: P1)

**Goal**: Deploy the full shared contract suite (AMM, Liquidity Pool/pair/currency registries, identity registry, FX agreement, shared tokens, oracle) onto the now-independent Hub network at chain `1337`.

**Independent Test**: With User Story 1's network running, run the contract deployment procedure against it and confirm each shared contract returns a valid, queryable, distinct on-chain address on chain `1337`, with Hub-minted token supply at zero (quickstart.md Step 2).

### Implementation for User Story 2

- [ ] T010 [P] [US2] Update `scenario-b/contracts/.env.example`: point `HUB_RPC_URL` at the new `hub-besu` RPC endpoint (from T005) and set `HUB_CHAIN_ID=1337`
- [ ] T011 [US2] Edit the `contracts.deploy-hub` target in `scenario-b/make/30-contracts.mk`: remove the `BESU_HUB_RPC` → `SPOKE_A_RPC_URL` fallback and its accompanying warning message (currently around line 59), and instead default `BESU_HUB_RPC`/`HUB_CHAIN_ID` to the new `hub-besu` endpoint and `1337` (per `research.md` Decision 5 / `contracts/service-configuration-contract.md` §2 — this is the literal mechanism that bakes "Hub == Spoke A" into the deployment layer, and FR-008 requires it gone)
- [ ] T012 [US2] Validate independently: run `make contracts.deploy-hub` against the running Hub network; for each of the 9 contracts in the Shared Contract Suite (`IdentityRegistry`, `CurrencyRegistry`, `PairRegistry`, `TokenizedCentralBankMoney`×2, `ManualOracle`, `AutomatedMarketMaker`, `LiquidityCommitRegistry`, `FXAgreement`), confirm via `cast code <addr> --rpc-url <hub-rpc>` that bytecode is present and matches the compiled artifact, and via `cast call <tCeBM_addr> "totalSupply()(uint256)"` that Hub-minted supply is `0` (quickstart.md Step 2 / `data-model.md` §2 and §5 — the fresh-startup TVL baseline)

**Checkpoint**: The shared contract suite is live and addressable on chain `1337`. Combined with US1, the Hub is no longer an empty shell.

---

## Phase 5: User Story 3 - Engineer reconfigures all services to point exclusively at the Hub (Priority: P1)

**Goal**: Every Scenario B service resolves the Hub's identity from configuration alone, defaulting to `1337`, with a non-silent (warn-and-continue) fallback when the value is absent — and zero remaining defaults of `1338`.

**Independent Test**: Search the codebase/config for the old hardcoded value and confirm none remain as Hub defaults; confirm every service resolves `1337` under standard configuration and emits a visible warning (without crashing) when the value is absent (quickstart.md Step 3).

### Tests for User Story 3 ⚠️

> Constitution Principle V requires these to be written FIRST and confirmed FAILING before the implementation tasks (T016–T018) below.

- [ ] T013 [P] [US3] Write a failing test asserting `getEnv("HUB_CHAIN_ID", ...)` resolution in `payment-orchestrator/cmd/payment-orchestrator/main.go` defaults to `1337` (not `1338`) and emits a structured warning log naming the variable when the env var is unset, in `scenario-b/backend/services/payment-orchestrator/cmd/payment-orchestrator/main_test.go` (new file)
- [ ] T014 [P] [US3] Write a failing test asserting `api-gateway/internal/app/app.go`'s `HUB_CHAIN_ID` resolution defaults to `1337` and emits the same warning-on-absence behavior, extending `scenario-b/backend/services/api-gateway/internal/app/app_test.go` (existing file)
- [ ] T015 [P] [US3] Write a failing test asserting `identity_bootstrap.go` resolves the Hub chain ID from configuration (not the hardcoded `int64(1338)`), defaults to `1337`, and warns when unset, in `scenario-b/backend/services/api-gateway/internal/app/identity_bootstrap_test.go` (new file)

### Implementation for User Story 3

- [ ] T016 [US3] In `scenario-b/backend/services/payment-orchestrator/cmd/payment-orchestrator/main.go:205`, change the `getEnv("HUB_CHAIN_ID", "1338")` default to `"1337"` and add a structured-JSON warning log (matching the existing logger's shape — service name, severity `warn`, ISO-8601 timestamp) emitted whenever the default is applied; confirm T013 now passes
- [ ] T017 [US3] In `scenario-b/backend/services/api-gateway/internal/app/app.go:234`, replace the bare `os.Getenv("HUB_CHAIN_ID")` read with a default-plus-warning resolution identical in shape to T016 (default `1337`, warning on absence); confirm T014 now passes
- [ ] T018 [US3] In `scenario-b/backend/services/api-gateway/internal/app/identity_bootstrap.go`, replace the hardcoded `chainID := int64(1338) // Hub default` with configuration-sourced resolution defaulting to `1337` plus the same warning-on-absence behavior; confirm T015 now passes
- [ ] T019 [P] [US3] In `scenario-b/backend/docker-compose-backend.central-bank-a.yaml`, change `HUB_CHAIN_ID: "${HUB_CHAIN_ID:-1338}"` to default `1337` and update `HUB_BESU_RPC_URL` to point at the new `hub-besu` endpoint (from T005)
- [ ] T020 [P] [US3] Apply the same two edits to `scenario-b/backend/docker-compose-backend.central-bank-b.yaml`
- [ ] T021 [P] [US3] Apply the same two edits to `scenario-b/backend/docker-compose-backend.bank-a.yaml`
- [ ] T022 [P] [US3] Apply the same two edits to `scenario-b/backend/docker-compose-backend.bank-b.yaml`
- [ ] T023 [P] [US3] In `scenario-b/backend/config/.env.infra.central-bank-a.example`, change `HUB_CHAIN_ID=1338` (and any `BESU_CHAIN_ID`/comment block referring to the Hub) to `1337` and update the Hub RPC URL reference
- [ ] T024 [P] [US3] Apply the same edits to `scenario-b/backend/config/.env.infra.central-bank-b.example`
- [ ] T025 [P] [US3] Apply the same edits to `scenario-b/backend/config/.env.infra.bank-a.example`
- [ ] T026 [P] [US3] Apply the same edits to `scenario-b/backend/config/.env.infra.bank-b.example`
- [ ] T027 [US3] Run a repository-wide search confirming zero remaining locations default the Hub's network identity to `1338` and that every default-supplying location (Go call sites, `.env.infra.*.example`, `docker-compose-backend.*.yaml`, `contracts/.env.example`) defaults to `1337` (quickstart.md Step 3a / SC-002 — `grep -rn "HUB_CHAIN_ID" ... | grep -c 1338` must return `0`); fix any stragglers found
- [ ] T028 [US3] Validate independently: start the full stack with standard configuration and confirm `HUB_CHAIN_ID` resolves to `1337` everywhere; start a service with the variable unset and confirm it logs the visible warning and still starts; run the existing AMM cross-currency-swap E2E flow and confirm via relay/event logs that 100% of Hub-side on-chain activity lands on chain `1337` and 0% on `1338` (quickstart.md Steps 3b/3c/4 — FR-014, SC-005)

**Checkpoint**: All Scenario B services exclusively and visibly resolve the Hub's identity to `1337`. Combined with US1 + US2, the system runs end-to-end against the new Hub.

---

## Phase 6: User Story 4 - Operator follows a runbook to reproduce the full environment (Priority: P2)

**Goal**: Rewrite the deployment runbook so an LNet partner unfamiliar with this change can bring up the entire environment — Hub, contracts, both spokes, relay — from a clean machine using only the document.

**Independent Test**: Hand the rewritten runbook to someone unfamiliar with the change; they bring up the full stack from clean using only the document, with no undocumented steps (quickstart.md Step 6).

### Implementation for User Story 4

- [ ] T029 [US4] Rewrite the "Besu networks" phase of `scenario-b/docs/runbooks/deployment-runbook.md` to replace the existing "the hub shares the same Besu node as Spoke-A (chain 1338, RPC port 8645)" / "Local dev note: The Hub Besu network is co-located with Spoke-A" language with a description of the new independent `hub-besu` network and its startup steps (referencing the `deploy.up-hub-besu` target from T007)
- [ ] T030 [US4] Rewrite the contract-deployment phase to show the shared contract suite deploying to chain `1337` via the retargeted `contracts.deploy-hub` (T011), replacing any remaining references to contracts being "deployed to the Spoke-A node"
- [ ] T031 [US4] Add a "Validator Configuration" subsection stating explicitly that the sandbox uses a single-validator setup (per T005/T006) and separately describing what a multi-validator (`n ≥ 3f+1` QBFT) production configuration would additionally require (FR-011, US4 Acceptance Scenario 2)
- [ ] T032 [US4] Add a "TVL Reconciliation Baseline" section explaining the "zero Hub-minted value until first cross-chain lock event" invariant, how to confirm it immediately after a fresh startup (referencing the `cast call ... totalSupply()` check from T012), and how this differs from a restart with existing state (FR-012, US4 Acceptance Scenario 3, `data-model.md` §5)
- [ ] T033 [US4] Add a "Decommissioning the Legacy Hub-on-Spoke-A Deployment" section that explicitly states the prior chain-`1338` "hub" deployment is retired/non-authoritative, lists its addresses for reference only, designates the new Hub deployment as sole source of truth, and explains how to detect/correct a stale locally cached `1338` configuration value (FR-015, Edge Cases)
- [ ] T034 [US4] Validate independently: have someone unfamiliar with this change execute quickstart.md Steps 1–4 using only the rewritten runbook; confirm they reach a fully running environment without needing to ask a clarifying question (SC-003)

**Checkpoint**: The runbook fully and accurately describes the new independent topology; an external operator can reproduce the environment unaided — this is the basis for LNet sign-off.

---

## Phase 7: User Story 5 - Maintainer confirms no regressions to the existing system (Priority: P3)

**Goal**: Confirm that introducing the independent Hub and reconfiguring services has not destabilized Scenario A or any previously passing automated check.

**Independent Test**: Run the full automated check suite on the branch and confirm Scenario A behaves exactly as before, with no new failures attributable to this work.

### Implementation for User Story 5

- [ ] T035 [US5] Run Scenario A's full automated check suite and confirm it passes at the same rate as on `main`, with no new failures attributable to this branch (FR-013, SC-006)
- [ ] T036 [US5] Run the full Scenario B automated/E2E/smoke suite (re-pointed at chain `1337` per US3's changes) and confirm no new failures beyond what US1–US4's own validation steps already covered

**Checkpoint**: No regressions found — the change is confirmed additive/corrective, not destabilizing.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final, repo-wide checks and documentation currency that span all stories

- [ ] T037 [P] Update `scenario-b/README.md` to reflect the Hub's new independent-network status, per the constitution's "Documentation currency" requirement (implementation status MUST accurately read "Fully implemented" for this capability at merge time)
- [ ] T038 Re-run the full `quickstart.md` sequence (Steps 1–6) end-to-end as the final acceptance gate before merge
- [ ] T039 Re-run the repository-wide `1338`/`1337` Hub-identity grep from T027 as a final pre-merge gate (SC-002) — zero `1338` Hub-identity defaults must remain anywhere in `scenario-b/`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on T001 (needs the chosen network parameters to scaffold correctly) — BLOCKS the *functional validation* of every later phase (US2 needs a reachable Hub RPC endpoint; US3's compose/env edits need to know the endpoint to point at; US4's runbook describes the directory T003 creates)
- **User Story 1 (Phase 3)**: Depends on Foundational (T003). Delivers the MVP — an independently running Hub.
- **User Story 2 (Phase 4)**: T010–T011 (file edits) can be written in parallel with US1's implementation, but T012 (validation) requires US1's network to be running (T009 complete)
- **User Story 3 (Phase 5)**: T013–T026 (tests + file edits) have NO code dependency on US1/US2 — they can be written and unit-tested in complete isolation. T028 (E2E validation) requires US1 + US2 to be functionally complete (the full stack must be up and contracts deployed)
- **User Story 4 (Phase 6)**: T029–T033 (runbook prose) can be drafted in parallel with US1–US3, but should reference their final target/file/section names; T034 (validation) requires US1+US2+US3 to be functionally complete (the runbook must describe a working system)
- **User Story 5 (Phase 7)**: Depends on US1–US3 being functionally complete (there must be a changed system to regression-test)
- **Polish (Phase 8)**: Depends on all desired user stories being complete

### Within Each User Story

- US3 follows strict test-first ordering: T013–T015 (tests, parallel) → confirm failing → T016–T018 (implementation, sequential per file) → confirm passing → T019–T026 (config edits, parallel) → T027 (repo-wide check) → T028 (E2E validation)
- All other stories: implementation edits → independent validation (the story's "Independent Test" from spec.md)

### Parallel Opportunities

- T001 and T002 (Setup) can run in parallel
- T010 (contracts `.env.example`) can be edited in parallel with US1's Phase 3 implementation tasks (different files, no shared state until validation)
- T013, T014, T015 (US3 tests) can be written in parallel — three different files
- T019–T022 (four docker-compose files) can be edited in parallel — different files, identical edit shape
- T023–T026 (four `.env.infra.*.example` files) can be edited in parallel — different files, identical edit shape
- T029–T033 (five runbook sections) can be drafted in parallel by different writers, then assembled

---

## Parallel Example: User Story 3

```bash
# Write all three failing tests together (different files):
Task: "Write failing test for payment-orchestrator HUB_CHAIN_ID default+warning in scenario-b/backend/services/payment-orchestrator/cmd/payment-orchestrator/main_test.go"
Task: "Write failing test for api-gateway app.go HUB_CHAIN_ID default+warning in scenario-b/backend/services/api-gateway/internal/app/app_test.go"
Task: "Write failing test for identity_bootstrap.go HUB_CHAIN_ID default+warning in scenario-b/backend/services/api-gateway/internal/app/identity_bootstrap_test.go"

# Then, after T016-T018 make them pass, edit all four compose files together:
Task: "Update HUB_CHAIN_ID/HUB_BESU_RPC_URL defaults in scenario-b/backend/docker-compose-backend.central-bank-a.yaml"
Task: "Update HUB_CHAIN_ID/HUB_BESU_RPC_URL defaults in scenario-b/backend/docker-compose-backend.central-bank-b.yaml"
Task: "Update HUB_CHAIN_ID/HUB_BESU_RPC_URL defaults in scenario-b/backend/docker-compose-backend.bank-a.yaml"
Task: "Update HUB_CHAIN_ID/HUB_BESU_RPC_URL defaults in scenario-b/backend/docker-compose-backend.bank-b.yaml"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T002)
2. Complete Phase 2: Foundational (T003)
3. Complete Phase 3: User Story 1 (T004–T009)
4. **STOP and VALIDATE**: Confirm the Hub runs independently and reports chain `1337` (T009)
5. Demo: "The International Hub now exists as its own network" — the foundational architectural claim is proven

### Incremental Delivery

1. Setup + Foundational → directory and parameters ready
2. **US1** → independent Hub network running → demo the MVP
3. **US2** → shared contracts live on the Hub → demo "the neutral layer is no longer an empty shell"
4. **US3** → all services exclusively resolve `1337`, defect closed → demo a full E2E swap on the Hub
5. **US4** → runbook rewritten and partner-validated → ready for LNet sign-off
6. **US5** → regression-free confirmed → ready to merge
7. **Polish** → documentation currency + final SC-002/quickstart gates → merge

### Parallel Team Strategy

With multiple contributors, after Setup + Foundational (T001–T003):
- Contributor A: User Story 1 (network) — unblocks US2/US3/US5's validation steps fastest
- Contributor B: User Story 3's tests + Go edits (T013–T018) — no dependency on US1/US2's runtime, can proceed entirely in parallel
- Contributor C: User Story 4's runbook prose (T029–T033) — can be drafted against the planned target state, finalized once US1–US3 land

---

## Notes

- [P] tasks touch different files with no inter-task dependency
- [Story] labels map every implementation task to its user story for traceability back to spec.md
- US3's tests (T013–T015) MUST be confirmed failing before T016–T018 are written, per Constitution Principle V (red→green, non-negotiable)
- T027 and T039 both re-run the SC-002 grep — T027 is the "did I miss anything" check inside US3; T039 is the final pre-merge gate after US4/US5 may have touched documentation that also references chain IDs
- Commit after each task or logical group; stop at any checkpoint to validate a story independently before continuing
