# Implementation Plan: International Hub Network Isolation

**Branch**: `001-hub-network-isolation` | **Date**: 2026-06-08 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-hub-network-isolation/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

The International Hub currently has no independent identity: every Scenario B service resolves `HUB_CHAIN_ID=1338`, which is Spoke A's own chain — confirmed in code by `make/30-contracts.mk:59`, which silently falls back to `SPOKE_A_RPC_URL` whenever `BESU_HUB_RPC` is unset, and by the deployment runbook's own admission that "the hub shares the same Besu node as Spoke-A." This plan stands up a genuinely independent Besu/QBFT network at chain `1337` — mirroring the proven `spoke-besu-a`/`spoke-besu-b` layout exactly except for chain identity and topology — redeploys the full shared contract suite onto it via the existing (unchanged) Foundry scripts, removes every hardcoded/defaulted reference to `1338` as "the Hub" across 4 docker-compose files, 4 `.env.example` files, and 3 Go call sites (replacing them with a `1337` default plus a non-silent warning when absent), and rewrites the deployment runbook's Hub-related phases to describe a clean rebuild — explicitly retiring the old chain-1338 "hub" addresses rather than migrating their state (per the `/speckit.clarify` decision, FR-015).

## Technical Context

**Language/Version**: Bash (deployment/genesis scripts), Go 1.26+ (`api-gateway`, `payment-orchestrator`), Solidity via Foundry/`forge` (contract redeploy targets — no contract source changes)
**Primary Dependencies**: Hyperledger Besu 25.8.0 (QBFT consensus), Docker Compose, GNU Make, Foundry (`forge`/`cast`)
**Storage**: N/A — this is a network/configuration feature; no new persistent storage, schemas, or migrations are introduced
**Testing**: Foundry (`forge test` — contract suite is unchanged and already covered), `go test` (the two touched Go call sites get a config-resolution unit test), the existing deployment-runbook smoke tests (HTLC cross-spoke, AMM quote+swap) re-run against chain `1337`
**Target Platform**: Local Docker Compose development environments (operators: project engineers and LNet partners), reproducing the existing `spoke-besu-a`/`spoke-besu-b` operational model
**Project Type**: Infrastructure/deployment correction within an existing Go + Solidity + Docker Compose monorepo scenario (`scenario-b/`)
**Performance Goals**: Hub network reaches block production and answers RPC within the SC-001 target (<10 minutes from a stopped state, matching the existing spoke startup experience); no new latency/throughput targets — block period (`2s`) and gas limit (`0x1c9c380`) are inherited unchanged from Spoke A/B genesis configs
**Constraints**: MUST NOT alter Scenario A behavior or defaults (FR-013 / Constitution Principle I — Scenario-Scoped Independence confines all changes to `scenario-b/`); MUST NOT introduce a new runtime dependency outside the fixed stack (Besu/QBFT, Foundry, Go, Docker Compose); cutover is a clean rebuild — no on-chain state migration tooling (FR-015)
**Scale/Scope**: One new Besu/QBFT network (1 validator for the sandbox, documented multi-validator path for production); ~10 shared contracts redeployed; 4 docker-compose files + 4 `.env.example` files + 3 Go call sites + 3 Makefile targets edited; 1 runbook rewritten (Hub-related phases)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Assessment | Notes |
|---|---|---|
| **I. Scenario-Scoped Independence** | **PASS** | Every touched path is under `scenario-b/` (new `deploy/local/hub-besu/`, `contracts/`, `backend/`, `make/`, `docs/runbooks/`). No Scenario A file is modified (FR-013); no new cross-scenario shared library is introduced. The new Hub network is, by definition, topologically self-contained — it must be startable without Spoke A or Spoke B running (US1), which is the same independence test the constitution applies to scenarios themselves. |
| **II. Privacy by Design** | **PASS / N/A** | No token or privacy-mechanism code changes. `tCeBM` (ERC-20, reserve-layer) contracts are redeployed as-is to a new chain; `ZetoToken`/`NotoToken` usage is unaffected — this feature only relocates *where* the existing, already-compliant contract suite executes. |
| **III. Atomic Settlement Guarantee** | **PASS — and corrective** | Today the relay's Scenario B verification clause ("Spoke lock event → Hub mint, Hub burn event → Spoke unlock") is *degenerate*: Spoke A and "Hub" are the same chain, so the cross-chain check it's supposed to perform cannot meaningfully execute. Making the Hub independent is what makes this constitutional requirement *testable* for the first time. The plan must confirm the Cacti relay's circuit-breaker validation and lock/mint/burn/unlock event verification operate correctly across two genuinely distinct chains (1337 ↔ 1338/1339) before sign-off — this becomes part of Phase 1 quickstart verification. |
| **IV. Compliance Gate Before Participation** | **PASS** | `IdentityRegistry` is part of the redeployed shared contract suite (FR-005); `contracts.register-participants` (already chained into `scenario-b.deploy-contracts`) re-establishes registration/compliance gating against the new Hub addresses. No gate logic changes — only the network it runs against. |
| **V. Test-First at Every Layer** | **PASS — gate enforced** | Contract code is unchanged, so existing Foundry tests remain the safety net (re-run against the new chain ID as part of deployment, not rewritten). The two Go call sites that gain new fallback/warning behavior (FR-009) get a unit test written first (red→green) per Principle V. The existing E2E/smoke suite (HTLC cross-spoke, AMM quote+swap) is the acceptance gate for "activity occurs on 1337, not 1338" (FR-014, SC-005) — it is re-pointed at the new chain, not replaced. |
| **VI. Observability and Auditability** | **PASS** | The relay logging requirements already specify the Scenario B lifecycle events (commit registered → pair matched → swap executed → circuit-breaker transitions, lock→mint, burn→unlock). This plan adds exactly one new observable: a structured warning log when `HUB_CHAIN_ID` falls back to its default (FR-009) — consistent with the "no silent failures" rule, sourced through the same structured-JSON logging already in use. |

**Gate result**: All six principles PASS. No entries required in Complexity Tracking — this work follows established patterns (mirrors `spoke-besu-a`/`spoke-besu-b`; reuses existing Foundry deploy scripts, the `getEnv` helper, and `contracts.sync-addresses`) rather than introducing new ones.

## Project Structure

### Documentation (this feature)

```text
specs/001-hub-network-isolation/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command) — interface contracts
│   ├── hub-network-rpc-contract.md
│   └── service-configuration-contract.md
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

This is an infrastructure-correction feature inside the existing `scenario-b/` monorepo scenario — there is no new application/service to scaffold. The structure below lists only the real paths touched, organized by what already exists vs. what is newly created.

```text
scenario-b/
├── deploy/local/
│   ├── hub-besu/                                          # NEW — mirrors spoke-besu-a/ exactly
│   │   ├── .env.network                                   #   NETWORK_NAME=hub_besu_network, CONTAINER_PREFIX=cbweb3-hub-besu,
│   │   │                                                   #   dedicated RPC/P2P/WS ports (no collision with 86xx/87xx ranges)
│   │   ├── config/configTemplate.json                     #   chainId: 1337; blockperiodseconds/gasLimit/etc. copied verbatim
│   │   │                                                   #   from spoke-besu-a (Decision 2, research.md)
│   │   ├── startBesu.sh / stopBesu.sh                     #   copied & adapted from spoke-besu-a (single validator for sandbox)
│   │   ├── genesis/ , nodes/                              #   generated at startup, same as spoke-besu-a/-b
│   │   └── addNewNode.sh, besu.autocomplete.sh, bin/, lib/, LICENSE   # copied utilities (match spoke-besu-a)
│   ├── spoke-besu-a/                                      # UNCHANGED (chain 1338 — remains a spoke, not the Hub)
│   └── spoke-besu-b/                                      # UNCHANGED (chain 1339)
│
├── contracts/
│   ├── .env.example                                       # EDIT: HUB_RPC_URL → new hub-besu endpoint; HUB_CHAIN_ID=1337
│   └── script/CBWeb3Hub.s.sol                             # UNCHANGED — redeployed via new --rpc-url target only
│
├── backend/
│   ├── docker-compose-backend.central-bank-a.yaml         # EDIT: HUB_CHAIN_ID default 1338→1337, HUB_BESU_RPC_URL → hub-besu endpoint
│   ├── docker-compose-backend.central-bank-b.yaml         # EDIT: same two defaults
│   ├── docker-compose-backend.bank-a.yaml                 # EDIT: same two defaults
│   ├── docker-compose-backend.bank-b.yaml                 # EDIT: same two defaults
│   ├── config/.env.infra.central-bank-a.example           # EDIT: HUB_CHAIN_ID=1337, BESU comment block updated
│   ├── config/.env.infra.central-bank-b.example           # EDIT: same
│   ├── config/.env.infra.bank-a.example                   # EDIT: same
│   ├── config/.env.infra.bank-b.example                   # EDIT: same
│   └── services/
│       ├── payment-orchestrator/cmd/payment-orchestrator/main.go        # EDIT line 205: getEnv default 1338→1337 + warning log (FR-009)
│       ├── payment-orchestrator/.../main_test.go (new/extended)         # NEW unit test for the fallback+warning behavior (Principle V, red→green)
│       └── api-gateway/internal/app/
│           ├── app.go                                                    # EDIT line 234: same default+warning treatment
│           ├── identity_bootstrap.go                                     # EDIT: align its inline "1338 // Hub default" fallback to 1337 + warning
│           └── app_test.go / identity_bootstrap_test.go (new/extended)   # NEW unit tests
│
├── make/
│   ├── 10-deploy.mk                                       # EDIT: add deploy.up-hub-besu / deploy.down-hub-besu; deploy.up-besu depends on it
│   ├── 30-contracts.mk                                    # EDIT: contracts.deploy-hub — remove the SPOKE_A_RPC_URL fallback (line 59),
│   │                                                       #   default BESU_HUB_RPC/HUB_CHAIN_ID to the new hub-besu endpoint/1337
│   └── 60-scenario-b.mk                                   # EDIT: scenario-b.up-infra depends on the new hub-besu target
│
└── docs/runbooks/
    └── deployment-runbook.md                              # REWRITE Phase 3 ("Besu networks") to add a Hub Besu phase;
                                                            # REWRITE Phase 4 to show hub contracts deploying to chain 1337;
                                                            # ADD: single-validator-sandbox note + production multi-validator
                                                            #   requirements (FR-011); TVL reconciliation baseline section (FR-012);
                                                            # ADD: "Decommissioning the legacy Hub-on-Spoke-A deployment" section
                                                            #   documenting the clean-rebuild cutover (FR-015) and retired addresses
```

**Structure Decision**: Extend the existing, proven `scenario-b/deploy/local/` layout by adding a `hub-besu/` directory that is a structural mirror of `spoke-besu-a/` (same script names, config shape, genesis/QBFT parameters — differing only in `chainId`, network name, container prefix, and port ranges). This keeps the Hub "topologically separate" (FR-004) while staying "operationally consistent" (FR-003) and immediately familiar to anyone — including LNet — who already knows how to operate a spoke. All other changes are narrow, mechanical edits that propagate the new `1337` default through the Makefile → contract-deploy scripts → backend compose/env files → the two Go services that read `HUB_CHAIN_ID`, finishing with a runbook rewrite of the Hub-specific phases. Every touched path sits under `scenario-b/`, satisfying Constitution Principle I.

## Complexity Tracking

> No entries — the Constitution Check above resulted in a clean pass with no violations requiring justification. This plan introduces no new architectural patterns, no fourth project layer, and no shared database; it replicates an existing, sanctioned pattern (`spoke-besu-a`/`spoke-besu-b`) and corrects configuration defaults using mechanisms (the `getEnv` helper, `contracts.sync-addresses`) that already exist in the codebase.
