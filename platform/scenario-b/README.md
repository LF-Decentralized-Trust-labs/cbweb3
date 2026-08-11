# Scenario B — International Hub with FX Liquidity

A wholesale CBDC interoperability model built around a dedicated **international hub** network. The hub runs an AMM-based foreign-exchange liquidity pool; two sovereign spoke networks, each operated by a central bank and a commercial bank, settle cross-currency transactions through it. Value moves between a spoke and the hub via a lock-and-mint / burn-and-unlock bridge, is exchanged on the hub AMM, and bridges back. A Cacti-based relay observes events across all three networks and a governance-controlled circuit breaker can pause and resume the pool.

> This README describes **Scenario B only**. Repository-wide context and the comparison between Scenario A and Scenario B live in the [root README](../README.md).

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Running the Scenario B Walkthrough](#running-the-scenario-b-walkthrough)
- [Frontend](#frontend)
- [Directory Structure](#directory-structure)
- [Toolkit (Provisioning)](#toolkit-provisioning)
- [Smart Contracts](#smart-contracts)
- [Backend Services](#backend-services)
- [Infrastructure](#infrastructure)
- [Port Reference](#port-reference)
- [Testing](#testing)
- [Teardown](#teardown)
- [Additional Make Targets](#additional-make-targets)

---

## Overview

Scenario B models a hub-and-spoke wholesale CBDC ecosystem centred on an **international hub**:

- **Hub** (chain 1337) — an independent Besu QBFT network operating the FX liquidity pool. It hosts the AMM, the FX agreement contract, the liquidity/pair/currency registries, and hub-wrapped tokens (W-tCeBM). The `SpokeBridge` that moves value between a spoke and the hub is deployed spoke-side, not on the hub.
- **Spoke-A** (chain 1338) — operated by **Central-Bank-A** with commercial bank **Bank-A**.
- **Spoke-B** (chain 1339) — operated by **Central-Bank-B** with commercial bank **Bank-B**.

Each spoke runs its own Besu QBFT network and a full backend service stack; the four entities also peer on the hub network. Cross-currency settlement works as **lock → mint (bridge in) → swap on the AMM → burn → unlock (bridge out)**, coordinated by the backend orchestrator and observed by a Cacti relay.

### Key Capabilities

| Capability | Status |
|------------|--------|
| **Hub AMM (FX liquidity pool)** — constant-product AMM with slippage bounds, fee math, and a governance circuit breaker (pause/resume) | Implemented |
| **FX Agreement** — bilateral OTC FX settlement lifecycle on the hub (`PROPOSED → ACCEPTED → SETTLED`, plus reject/cancel and on-behalf variants) | Implemented |
| **Cooperative liquidity** — `PairRegistry` bilateral currency-pair activation + `LiquidityCommitRegistry` commit-reveal provisioning | Implemented |
| **Spoke bridge** — `SpokeBridge` lock-and-mint / burn-and-unlock between a spoke tCeBM and hub W-tCeBM | Implemented |
| **Cross-currency swap orchestration** — backend orchestrator wiring bridge-in → AMM swap → bridge-out, with a rollback coordinator | Implemented |
| **Stuck-position handling** — a failed bridge position transitions to a terminal `RECONCILIATION_REQUIRED` state that aborts the request; there is no automatic reconciliation worker, so recovery from this state is manual | Implemented (terminal state; manual recovery) |
| **Commercial-bank direct swap** — end-to-end swap driven directly by a commercial bank | Partial — the governance-driven path works; the commercial-bank direct-authentication path is in progress |
| **Manual FX oracle** — `ManualOracle` rate feed, fed locally by a mock rate feeder | Implemented |
| **Identity & compliance** — on-chain `IdentityRegistry`, PKI certificates, Keycloak OIDC, KYC/AML gate at the API gateway | Implemented |
| **NOC** — Network Operations Center portal + backend + monitoring agents for the hub and both spokes | Implemented |
| **Privacy on the spokes** — Paladin/Zeto ZKP execution for confidential intra-spoke tCeBM transfers | Implemented (spokes only) |

**Not part of Scenario B today:**

- **No privacy on the hub.** The hub is a plaintext Besu QBFT network; hub-side value is held as wrapped tCeBM (W-tCeBM) ERC-20. Paladin/Zeto runs on the spokes only.
- **No staging/production environment.** Scenario B ships a local Docker Compose deployment only; the provisioning toolkit (below) targets self-managed multi-host deployments, not a hosted environment.

---

## Architecture

The scenario is organized in layers:

| Layer | Components |
|-------|------------|
| **Frontend** | Bank Portal, Governance Portal, Treasury, Supervisor, NOC Portal |
| **API** | REST API Gateway (per entity) |
| **Microservices** | Auth (gRPC), Compliance (gRPC), Payment Orchestrator (gRPC), FX, Ledger Gateway |
| **Interoperability** | Cacti HTLC/event relay, SpokeBridge, cross-currency swap orchestrator + rollback coordinator |
| **FX** | AutomatedMarketMaker, ManualOracle, FXAgreement, PairRegistry, LiquidityCommitRegistry, CurrencyRegistry, circuit breaker |
| **Privacy (spokes only)** | Paladin Core, Zeto Domain (ZKP tokens) |
| **Blockchain** | Besu QBFT networks (hub: chain 1337, spoke-a: chain 1338, spoke-b: chain 1339) |

---

## Prerequisites

| Tool | Purpose |
|------|---------|
| **Docker & Docker Compose** | Container orchestration for all services |
| **GNU Make** | Build automation (`make` targets) |
| **Go** (1.26+) | Backend services and Paladin tooling |
| **Node.js** (22+) & npm | Frontend applications and the Cacti relay |
| **Foundry** (forge, cast) | Solidity contract compilation, testing, deployment |
| **jq** | JSON processing in shell scripts |
| **openssl** | PKI certificate generation |

> **Note:** Ensure Docker has sufficient resources allocated (recommended: 8+ GB RAM, 4+ CPUs) since the full stack runs the hub, two spokes, Paladin nodes, shared infra, and per-entity backend stacks.

All commands below are run from the `scenario-b/` directory.

---

## Quick Start

Bring up the entire Scenario B stack — PKI, shared infrastructure, all three Besu networks, contracts, relay, backend services, the FX rate feeder, and the NOC:

```bash
make scenario-b.up
```

This target runs, in order:

1. **PKI** (`prepare-pki`) — generates X.509 certificate material for all entities (idempotent).
2. **Infrastructure + Besu** (`up-infra`) — starts Keycloak, PostgreSQL, and Redis, then the hub and both spoke Besu QBFT networks.
3. **Contracts** (`deploy-contracts`) — builds and deploys the hub and spoke contracts, syncs addresses to backends/frontends, registers participants, seeds the sovereign currency pair, and grants liquidity-provider roles.
4. **Relay** (`up-relayer`) — starts the Cacti cross-network relay.
5. **Backend** (`up-backend`) — starts api-gateway, auth, compliance, and payment-orchestrator per entity.
6. **FX feeder** — starts the mock BRL/ARS rate feeder that writes into the hub `ManualOracle`.
7. **NOC** — configures Keycloak, starts the NOC portal/backend, and starts the monitoring agents.

A perf-lean variant without the NOC portal is available as `make scenario-b.up-perf`.

Once complete, the four entities (central-bank-a + bank-a on spoke-a, central-bank-b + bank-b on spoke-b) are running with their full service stacks.

---

## Running the Scenario B Walkthrough

After `make scenario-b.up` completes, run the end-to-end walkthrough. It covers five user stories; the script accepts `us1`, `us2`, `us3`, `us5`, `us6`, or `all` (the numbering skips `us4` — there is no such story):

```bash
bash tryouts/tryout-scenario-b-e2e.sh all      # every story
bash tryouts/tryout-scenario-b-e2e.sh us1      # a single story
```

Convenience targets wrap the first three stories:

```bash
make scenario-b.tryout          # all stories
make scenario-b.tryout-us1      # quote + swap + pool status
make scenario-b.tryout-us2      # spoke↔hub bridging + cooperative liquidity remove
make scenario-b.tryout-us3      # governance circuit breaker + master viewing key
make scenario-b.tryout-us2-mlp  # US2 with the MLP dual-sided provisioning step (ENABLE_MLP=true)
```

The table below is derived from the script dispatcher (`tryouts/tryout-scenario-b-e2e.sh`). Each story is cumulative: `us2` also runs the `us1` steps, and `us3` runs `us1` + `us2`.

| Story | id | What it exercises | Status |
|-------|----|-------------------|--------|
| **US1** | `us1` | Quote + swap on the hub AMM + pool status | Implemented |
| **US2** | `us2` | Spoke↔hub bridging (lock-and-mint + burn-and-unlock) + cooperative liquidity remove; optional MLP dual-sided provisioning under `ENABLE_MLP=true` (`scenario-b.tryout-us2-mlp`) | Implemented |
| **US3** | `us3` | Governance circuit breaker (asymmetric: 1-of-N pause, 2-of-N resume) + master viewing key | Implemented |
| **US5** | `us5` | Bilateral PairRegistry propose + rejection tests + confirm | Implemented |
| **US6** | `us6` | CurrencyRegistry register / list / reject duplicates / remove (skipped when `CURRENCY_REGISTRY_CONTRACT_ADDRESS` is unset) | Implemented |

The commercial-bank cross-currency swap is not one of these numbered stories; it has its own script, `tryouts/tryout-commercial-swap-e2e.sh` (see also `tryout-cross-currency-full-lifecycle.sh`). See [`tests/TEST-CATALOG.md`](tests/TEST-CATALOG.md) for the full, per-test status inventory.

---

## Frontend

Build and start the Scenario B frontend containers:

```bash
make frontend-scenario-b        # build + start
make frontend-scenario-b-down   # stop
make frontend-scenario-b-logs   # tail logs
```

### Frontend Apps

| App | Path | Description | Status |
|-----|------|-------------|--------|
| `bank` | `frontend/apps/bank/` | Commercial bank operator portal | Implemented |
| `governance` | `frontend/apps/governance/` | Central bank governance console | Implemented |
| `treasury` | `frontend/apps/treasury/` | Central bank treasury management | Implemented |
| `supervisor` | `frontend/apps/supervisor/` | Operations supervisor dashboard | In progress |
| `noc` | `frontend/apps/noc/` | Network Operations Center portal | Implemented |

### Frontend Tech Stack

- **React 19** + TypeScript
- **Vite 7** build tool
- **Tailwind CSS v4**
- npm workspaces monorepo (`apps/*` and `packages/*`)

Container host ports are defined in `frontend/.env` (see `frontend/.env.example`). Defaults: Bank-A `5173`, Bank-B `5174`, Central-Bank-A governance `5177`, Central-Bank-B governance `5178`, Supervisor Spoke-A `5179`, Supervisor Spoke-B `5180`, Central-Bank-A treasury `5181`, Central-Bank-B treasury `5182`, NOC portal `5910`.

---

## Directory Structure

```
scenario-b/
├── contracts/              Solidity smart contracts (Foundry)
│   ├── src/                Contract sources
│   ├── test/               Foundry tests
│   └── script/             Deployment scripts
├── backend/
│   ├── services/           Go microservices
│   │   ├── api-gateway/    REST gateway (per entity), AMM/FX/bridge/liquidity APIs
│   │   ├── auth/           gRPC auth service (Keycloak OIDC)
│   │   ├── compliance/     gRPC compliance (KYC/AML)
│   │   ├── payment-orchestrator/  gRPC payment, bridge, and swap orchestration
│   │   ├── payments/       Payment domain logic
│   │   ├── fx/             FX pricing helpers
│   │   ├── ledger-gateway/ Blockchain RPC/WS client
│   │   ├── noc-agent/      Per-network monitoring agent
│   │   └── noc-backend/    NOC portal backend
│   ├── shared/             Shared Go libraries
│   └── config/             PKI certs and per-entity .env files
├── frontend/
│   ├── apps/               React apps (bank, governance, treasury, supervisor, noc)
│   └── packages/           Shared UI components and config
├── apis/                   OpenAPI specs, protobuf/gRPC definitions, generated SDKs
├── interop/
│   └── hub-and-spoke/      Cross-network interop
│       ├── cacti/          Cacti-based event/HTLC relay
│       ├── ccip/           CCIP experiments
│       └── noc/            NOC agent configs (hub, spoke-a, spoke-b)
├── deploy/
│   └── local/              Docker Compose infrastructure
│       ├── hub-besu/       Besu nodes for the hub (chain 1337)
│       ├── spoke-besu-a/   Besu nodes for spoke-a (chain 1338)
│       ├── spoke-besu-b/   Besu nodes for spoke-b (chain 1339)
│       ├── paladin/        Privacy nodes (Zeto) — spokes only
│       ├── keycloak/       OIDC identity provider
│       ├── postgres/       Multi-database init
│       ├── compose.yml     Shared infrastructure compose
│       └── compose.noc.yml NOC stack compose
├── make/                   Modular Makefile includes
├── toolkit/                Declarative provisioning toolkit (Go: cmd/cbweb3b + engine/*)
├── provisioning/           Provisioning assets (schema/v1 + compose templates)
├── tests/                  Unit, integration, E2E, and performance harnesses
├── tryouts/                E2E walkthrough and per-flow scripts
└── docs/                   Architecture, design, governance, runbooks
```

---

## Toolkit (Provisioning)

Declarative provisioning toolkit for the Scenario B hub-and-spoke topology, used to stand up and join hub and spoke networks across self-managed hosts. The single-host `make scenario-b.up` stack above remains the local path. The Go toolkit lives in `toolkit/` (`cmd/cbweb3b` + `engine/*`); its compose templates and the `ParticipantDeployment` JSON-Schema live in `provisioning/`.

| Component | Status |
|-----------|--------|
| Manifest schema & validation (`ParticipantDeployment`, `cbweb3b/v1`) — TK-B1 | Implemented |
| Custody boundaries: `KeyProvider` (`kms://`) + `CertSource` (`self-signed`/`ca://`) — TK-B2/B3 | Implemented |
| Parametrized compose templates (hub, entity-besu, entity-*, relay, NOC) under `provisioning/templates/` + validation — TK-B4 | Implemented |
| Generalized relay (dynamic N-spokes, runtime registration, `isPaused` gate) + `RelayRegistrar` (`relay://`) — TK-B5 | Implemented |
| Orchestration engine + `found-hub` steps + hub bundle + `apply` CLI — TK-B6 | Implemented |
| `found-spoke` mode (register-cb + spoke contracts + Keycloak + register-relay-spoke + soft add-noc-agent) + spoke bundle emitter — TK-B7 | Implemented |
| `join` mode (non-validating full node: write-genesis + wait-sync + gen-csr; canonical flow, no relay/noc) — TK-B8 | Implemented |
| Sovereign-pair tail (open-sovereign-pair + commit-liquidity + seed-oracle; soft, driven by `spec.pair`, strict sovereignty) — TK-B9 | Implemented |
| Full-pipeline E2E (swap + breaker + SpokeBridge) + toolkit-native perf baseline + E2E-STATUS — TK-B10 | Implemented |
| NOC observability integration (`observe` mode) — TK-B11 | Implemented |
| Production `CertSource`/`KeyProvider` (KMS/CA), auth-per-CB relay, threshold-gated baseline | Planned |

The manifest model and validation are the toolkit's entry point: parse + validate + report only, no execution. The custody boundaries provide per-entity blockchain keys and the CB-as-CA leaf issuance, with local in-memory implementations and production stubs behind URI factories; no private key material ever enters a manifest, state file, or bundle. See [`toolkit/README.md`](toolkit/README.md) and [`toolkit/E2E-STATUS.md`](toolkit/E2E-STATUS.md) for usage and pipeline status.

**Runtime dependency justification (Technology Stack Constraints):** the toolkit module adds `github.com/ethereum/go-ethereum` (v1.17.1, already standard across the repository). It is required by the `KeyProvider` for **secp256k1** key handling and EVM address derivation — the curve used to sign Besu/QBFT transactions, which is outside the Go standard library's `crypto/ecdsa`.

The **api-gateway** module adds `github.com/redis/go-redis/v9` (v9.18.0, the version the `auth`
service already depends on). No new infrastructure: every entity already runs a Redis
(`entity-infra.compose.yaml`), and `auth` already keeps its PKI login nonces there. The gateway uses
it for one thing — the relay-auth replay guard, which admits each verified signature once. Held in
process memory alone, that guard is lost on restart and absent across replicas, and the control it
enforces is a compliance one (a replayed `transfer-limits/restore` credits a bank's daily allowance
back). `REDIS_ADDR` unset falls back to per-process memory and the gateway says so at boot; an
unreachable Redis degrades to the same fallback rather than refusing traffic, since every internal
route rides that middleware.

---

## Smart Contracts

Built with **Foundry**. Foundry unit tests exist for every contract below (see [`tests/TEST-CATALOG.md`](tests/TEST-CATALOG.md), Section 1).

| Contract | Description |
|----------|-------------|
| `TokenizedCentralBankMoney.sol` | ERC-20 tCeBM with role-based minting (central bank only); wrapped as W-tCeBM on the hub |
| `FiatCentralBankMoney.sol` | ERC-20 fCeBM (fiat central bank money on the spoke ledger); escrowed/redeemed against tCeBM by the central bank |
| `HashTimeLockedContract.sol` | HTLC deployed on the hub but labelled **Scenario A** (Correspondent Banking) in the deploy script; cross-scenario residue, not part of the Scenario B settlement path |
| `AutomatedMarketMaker.sol` | Constant-product AMM for FX liquidity pools, with circuit breaker and fee math |
| `SpokeBridge.sol` | Lock-and-mint / burn-and-unlock bridge between a spoke tCeBM and hub W-tCeBM |
| `FXAgreement.sol` | Bilateral OTC FX settlement agreement lifecycle |
| `PairRegistry.sol` | Bilateral currency-pair proposal and activation |
| `LiquidityCommitRegistry.sol` | Commit-reveal liquidity provisioning with matching and TTL expiry |
| `CurrencyRegistry.sol` | Registry of currencies and their issuing central banks |
| `CommitmentHashRegistry.sol` | On-chain commitment hash registry |
| `ManualOracle.sol` | Manual FX price oracle |
| `IdentityRegistry.sol` | On-chain participant registry with governance RBAC |

### Contract Commands

```bash
make contracts.setup       # Install Solidity dependencies (forge soldeer)
make contracts.build       # Compile contracts
make contracts.test        # Run Foundry unit tests
make contracts.coverage    # Coverage report
make contracts.fmt         # Format Solidity sources
make contracts.lint        # Lint checks
make contracts.gen-doc     # Generate Forge documentation
```

---

## Backend Services

All backend services are written in **Go** and communicate via **gRPC** internally, with a REST API Gateway as the external entry point.

| Service | Protocol | Description |
|---------|----------|-------------|
| **api-gateway** | REST | External entry point; hosts the AMM, FX agreement, bridge, and liquidity endpoints and the cross-currency swap orchestrator + rollback coordinator |
| **auth** | gRPC | OIDC/JWT validation, Keycloak integration, RBAC, nonce management |
| **compliance** | gRPC | KYC/AML checks, on-chain IdentityRegistry queries, audit logging |
| **payment-orchestrator** | gRPC | Payment coordination, bridge position tracking (including `RECONCILIATION_REQUIRED`), relayer worker |
| **payments** | — | Payment domain logic |
| **fx** | — | FX pricing helpers |
| **ledger-gateway** | — | Blockchain RPC/WS client |
| **noc-agent** | — | Per-network monitoring agent (hub, spoke-a, spoke-b) |
| **noc-backend** | REST | NOC portal backend |

Each entity (bank-a, bank-b, central-bank-a, central-bank-b) runs its own isolated instance of the core services with dedicated Docker networks.

---

## Infrastructure

The local deployment uses Docker Compose for all infrastructure components. See [`deploy/local/README.md`](deploy/local/README.md) for the authoritative topology.

### Besu Networks

| Network | Chain ID | Nodes | Docker Network |
|---------|----------|-------|----------------|
| hub | 1337 | hub-validator + one peer per entity (central-bank-a, bank-a, central-bank-b, bank-b) | `hub_besu_network` |
| spoke-a | 1338 | central-bank-a, bank-a | `spoke_a_besu_network` |
| spoke-b | 1339 | central-bank-b, bank-b | `spoke_b_besu_network` |

### Shared Services

| Service | Container | Port | Purpose |
|---------|-----------|------|---------|
| Keycloak | `cbweb3-keycloak` | 8081 | OIDC identity provider (one realm per entity + a NOC realm; an optional `mlp` realm under `ENABLE_MLP=true`) |
| PostgreSQL | `cbweb3-postgres` | 5432 | 6 databases (four entities + MLP + Keycloak) |
| Redis | `cbweb3-redis` | 6379 | Cache/session store (4 logical DBs) |

### Paladin Privacy Nodes (spokes only)

Each spoke runs Paladin nodes providing Zeto ZKP support for privacy-preserving intra-spoke tCeBM transfers. The hub network does not run Paladin. (The spoke compose files currently define more Paladin nodes than the spoke has Besu validators — e.g. `spoke-a` defines `cb`, `bank-a`, and a residual `bank-c` node; the extra node is legacy topology and is not required by the two-entity spoke model.)

### NOC Stack

The NOC stack (`deploy/local/compose.noc.yml`) runs a dedicated Postgres, the NOC backend, the NOC portal, and one monitoring agent per network (hub, spoke-a, spoke-b).

---

## Port Reference

### API Gateways (REST) and gRPC services

| Entity | Spoke | API Gateway | Auth gRPC | Compliance gRPC |
|--------|-------|-------------|-----------|-----------------|
| bank-a | spoke-a | 18080 | 19091 | 19093 |
| bank-b | spoke-b | 28080 | 29091 | 29093 |
| central-bank-a | spoke-a | 38080 | 39091 | 39093 |
| central-bank-b | spoke-b | 60080 | 60091 | 60093 |

### Besu RPC

| Node | Network | Host RPC Port |
|------|---------|---------------|
| hub-validator | hub | 8845 |
| central-bank-a (peer) | hub | 8846 |
| bank-a (peer) | hub | 8847 |
| central-bank-b (peer) | hub | 8848 |
| bank-b (peer) | hub | 8849 |
| central-bank-a | spoke-a | 8645 |
| bank-a | spoke-a | 8646 |
| central-bank-b | spoke-b | 8745 |
| bank-b | spoke-b | 8746 |

---

## Testing

```bash
make scenario-b.test             # contracts + backend
make scenario-b.test-contracts   # Foundry unit tests (AMM, Hub, SpokeBridge, ...)
make scenario-b.test-backend     # Go tests (api-gateway, payment-orchestrator, compliance)
make scenario-b.test-integration # full happy-path API integration test against a live stack
```

Foundry unit tests are fully implemented across all Scenario B contracts. Backend unit tests and integration coverage are partially implemented; several integration and E2E areas are still Planned — see [`tests/TEST-CATALOG.md`](tests/TEST-CATALOG.md) and [`docs/test-execution-plan.md`](docs/test-execution-plan.md).

### Tryout / E2E Scripts

E2E walkthroughs and per-flow demos live under `tryouts/`, including:

```bash
bash tryouts/tryout-scenario-b-e2e.sh all          # US1/US2/US3/US5/US6 walkthrough
bash tryouts/tryout-commercial-swap-e2e.sh         # commercial-bank swap flow
bash tryouts/tryout-cross-currency-full-lifecycle.sh
bash tryouts/tryout-cacti-interop.sh               # relay interop
```

---

## Teardown

```bash
make scenario-b.down            # Stop backend, relay, NOC, and infra
make scenario-b.nuke            # Full wipe (containers + Postgres volume + chain data)
make frontend-scenario-b-down   # Stop all frontends
```

---

## Additional Make Targets

### PKI

```bash
make scenario-b.prepare-pki    # Generate all certificate material (pki.gen-all)
make pki.check                 # Verify certificate status
make pki.clean                 # Remove generated certificates
```

### Infrastructure

```bash
make deploy.up-infra           # Start Keycloak + PostgreSQL + Redis
make deploy.up-hub-besu        # Start the hub Besu network only (chain 1337)
make deploy.up-besu            # Start hub + both spoke Besu networks
make scenario-b.up-infra       # Infrastructure + all Besu networks
```

### Relay

```bash
make scenario-b.up-relayer     # Start the Cacti cross-network relay
make scenario-b.down-relayer   # Stop the relay
```

### NOC

```bash
make noc.setup-keycloak        # Configure NOC Keycloak client
make noc.up                    # Start the NOC portal + backend + agents
make noc.setup-agents          # Register the monitoring agents
make noc.down                  # Stop the NOC stack
make noc.logs                  # Tail NOC logs
```

### FX Rate Feeder

```bash
make scenario-b.up-fx-feeder   # Start the mock BRL/ARS rate feeder (Hub ManualOracle)
make scenario-b.down-fx-feeder # Stop the feeder
```

### Protobuf

```bash
make proto-gen                 # Generate protobuf code
make proto-lint                # Lint proto files
make proto-breaking            # Check for breaking changes
```
