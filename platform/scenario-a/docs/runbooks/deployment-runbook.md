# Deployment Runbook — CBWeb3 Platform

> **Deliverable 9** · CBDC System Deployment

This runbook describes the complete procedure for bringing up the CBWeb3 environment, from prerequisites through operational endpoint verification.

---

## Table of Contents

- [Scope](#scope)
- [Prerequisites](#prerequisites)
- [Scenario A — Enhanced Correspondent Banking](#scenario-a--enhanced-correspondent-banking)
  - [Command overview](#command-overview)
  - [Full deployment: make spoke-all](#full-deployment-make-spoke-all)
  - [Detailed phases](#detailed-phases)
  - [Frontend: make frontend-spoke-all](#frontend-make-frontend-spoke-all)
  - [Health verification](#health-verification)
  - [End-to-end demo (HTLC cross-spoke)](#end-to-end-demo-htlc-cross-spoke)
  - [Teardown](#teardown)
- [Scenario B — International Hub](#scenario-b--international-hub)
- [Troubleshooting](#troubleshooting)

---

## Scope

| Scenario | Status | Description |
|----------|--------|-------------|
| **Scenario A** | Implemented and runnable | Enhanced Correspondent Banking (two Besu spokes, HTLC, Paladin/Zeto) |
| **Scenario B** | Implemented and runnable | International Hub with AMM/FX — deployed from its own tree; see [`scenario-b/docs/runbooks/deployment-runbook.md`](../../../scenario-b/docs/runbooks/deployment-runbook.md) |

---

## Prerequisites

Install all tools before proceeding. See the detailed guide in [`environment-setup.md`](environment-setup.md).

| Tool | Minimum version | Check |
|------|----------------|-------|
| Docker + Docker Compose | Docker 24+ | `docker --version` |
| GNU Make | 3.81+ | `make --version` |
| Go | 1.26+ | `go version` |
| Node.js | 22 LTS | `node --version` |
| npm | 10+ | `npm --version` |
| Foundry (`forge`, `cast`) | nightly | `forge --version` |
| `jq` | 1.6+ | `jq --version` |
| `openssl` | 3.x | `openssl version` |

**Recommended Docker resources:** 8 GB RAM and 4 CPUs (the full stack runs ~30 containers).

---

## Scenario A — Enhanced Correspondent Banking

### Command overview

```bash
cd scenario-a
make spoke-all          # Brings up the entire backend stack (both spokes + relay)
make frontend-spoke-all # Brings up all frontends
```

These two commands are sufficient to have the complete environment operational. The sections below detail what each one does internally.

---

### Full deployment: `make spoke-all`

```bash
cd scenario-a
make spoke-all
```

The command runs **Spoke-A** followed by **Spoke-B** in sequence, then starts the Cacti relay. Estimated time: **10–20 minutes** on first run (depending on Docker image build speed).

**Resulting components:**

| Component | Instances |
|-----------|-----------|
| Besu QBFT nodes | 6 (3 per spoke) |
| Keycloak | 1 (shared) |
| PostgreSQL | 1 (7 databases) |
| Redis | 1 (shared) |
| Paladin nodes | 6 (1 per entity) |
| Backend services | 24 (api-gateway, auth, compliance, payment-orchestrator × 6 entities) |
| HTLC relay (Cacti) | 1 |

---

### Detailed phases

Each spoke goes through the same phases. The numbers below correspond to the internal `make` calls.

#### Phase 1 — PKI (idempotent)

```bash
make pki.gen-central-bank-a pki.gen-bank-a pki.gen-bank-c pki.gen-commercial-banks
# Spoke-B: pki.gen-central-bank-b pki.gen-bank-b pki.gen-bank-d
```

Generates X.509 certificates (EC prime256v1) for each entity in `backend/config/pki/`. Generation is **idempotent**: if files already exist, the phase is skipped. To regenerate: `make pki.gen-all FORCE=1`.

Files generated per entity: `<entity>-ca.key`, `<entity>-ca.crt`, `<entity>.key`, `<entity>.csr`, `<entity>.crt`.

#### Phase 2 — Shared infrastructure

```bash
make deploy.up-infra
```

Brings up the shared services between both spokes via `docker compose`:

- **Keycloak** (port 8081) — waits for automatic readiness (up to 40 attempts × 3s)
- **PostgreSQL** (port 5432) — 7 databases created on init
- **Redis** (port 6379)

> This phase waits for the Keycloak initialization script (`KEYCLOAK_INIT_DONE`) to complete, up to 10 minutes.

#### Phase 3 — Besu networks

```bash
make deploy.up-spoke-a   # chain 1338
make deploy.up-spoke-b   # chain 1339
```

Starts QBFT validators via `startBesu.sh` in `deploy/local/spoke-besu-{a,b}/`. Each network has 3 nodes (central bank + 2 commercial banks).

#### Phase 4 — Smart Contracts

```bash
make contracts.setup
make contracts.deploy-spoke-a   # chain 1338
make contracts.deploy-spoke-b   # chain 1339
make contracts.sync-addresses
make contracts.register-participants-spoke-a
make contracts.register-participants-spoke-b
```

Compiles via Foundry and deploys the contracts. The resulting addresses are automatically propagated to the service environment variables via `contracts.sync-addresses`.

Contracts deployed per spoke:

| Contract | Purpose |
|---------|---------|
| `TokenizedCentralBankMoney` (tCeBM) | ERC-20 CBDC token |
| `FiatCentralBankMoney` (fCeBM) | Collateralized fiat token |
| `HashTimeLockedContract` | Atomic cross-spoke escrow |
| `IdentityRegistry` | On-chain participant registry |
| `SpokeBridge` | Cross-spoke bridge |
| `AutomatedMarketMaker` | AMM pool (used in Scenario B) |
| `FXAgreement` | FX agreement lifecycle |

#### Phase 5 — Paladin (privacy layer)

```bash
make paladin.deploy-contracts-spoke-a    # IdentityRegistry, ZetoFactory, PenteFactory
make paladin.generate-certs-spoke-a
make paladin.render-configs-spoke-a
make paladin.register-nodes-spoke-a
make paladin.stop-spoke-a
make paladin.clean-volumes-spoke-a
make paladin.start-spoke-a
make paladin.wait-spoke-a                # Waits for Paladin node readiness
make paladin.create-zeto-token-spoke-a   # ZKP tCeBM instance
make paladin.create-pente-context-spoke-a
make paladin.deploy-fxagreement-pente-spoke-a
make paladin.verify-fxagreement-pente-spoke-a
```

Configures Paladin privacy nodes with Zeto (ZKP) and Pente (bilateral context) domains. Configurations live in `deploy/local/paladin/spoke-{a,b}/config/<entity>/config.yaml`.

#### Phase 6 — Backend services

```bash
make deploy.up-backend-spoke-a
make deploy.up-backend-spoke-b
```

Starts four services per entity via `docker compose`:

| Service | Protocol | Responsibility |
|---------|----------|---------------|
| `api-gateway` | REST (HTTP) | External entry point |
| `auth` | gRPC | Identity, login, wallets, PKI |
| `compliance` | gRPC | Onboarding, AML, participant registry |
| `payment-orchestrator` | gRPC | HTLC, FX, Zeto, escrow |

#### Phase 7 — Relay (Cacti)

```bash
make cacti-up
```

Starts the Hyperledger Cacti relay that monitors HTLC events on both spokes and performs automatic settlement.

---

### Frontend: `make frontend-spoke-all`

```bash
cd scenario-a
make frontend-spoke-all
```

Builds and starts React containers for all entities. Requires `make spoke-all` to have completed first.

**URLs after deployment:**

| Entity | Application | URL |
|--------|------------|-----|
| Bank-A | Bank Portal | http://localhost:5173 |
| Bank-B | Bank Portal | http://localhost:5174 |
| Bank-C | Bank Portal | http://localhost:5175 |
| Bank-D | Bank Portal | http://localhost:5176 |
| Central-Bank-A | Governance Portal | http://localhost:5177 |
| Central-Bank-B | Governance Portal | http://localhost:5178 |

**Individual commands:**

```bash
make frontend-spoke-a          # Spoke-A only (Bank-A, Bank-C, CB-A)
make frontend-spoke-b          # Spoke-B only (Bank-B, Bank-D, CB-B)
make frontend-spoke-all        # Both spokes

make frontend-spoke-all-down   # Stop all frontends
make frontend-spoke-all-logs   # Tail frontend logs
```

---

### Health verification

After `make spoke-all`, verify the main endpoints:

```bash
# Keycloak
curl -s http://localhost:8081/realms/master | jq .realm

# API Gateways (healthcheck per entity)
curl -s http://localhost:18080/healthz   # bank-a
curl -s http://localhost:28080/healthz   # bank-b
curl -s http://localhost:38080/healthz   # central-bank-a
curl -s http://localhost:48080/healthz   # bank-c
curl -s http://localhost:58080/healthz   # bank-d
curl -s http://localhost:60080/healthz   # central-bank-b

# Besu RPC
curl -s -X POST http://localhost:8645 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result
# chain 1339 (spoke-b):
curl -s -X POST http://localhost:8745 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result

# PKI — verify generated certificates
make pki.check
make pki.check-commercial-banks
```

**Operational checklist:**

- [ ] Keycloak responds at `http://localhost:8081/realms/master`
- [ ] All 6 API gateways return 200 on `/healthz`
- [ ] Besu spoke-a (chain 1338) is producing blocks
- [ ] Besu spoke-b (chain 1339) is producing blocks
- [ ] Contracts deployed (addresses in `deploy/local/paladin/spoke-a/.deployed-addrs.env`)
- [ ] Paladin nodes responding (check logs via `docker logs`)
- [ ] Cacti relay running

---

### End-to-end demo (HTLC cross-spoke)

With the full stack operational, run the cross-spoke atomic settlement demo:

```bash
cd scenario-a
./tryout-htlc-cross-spoke.sh
```

The script executes 16 steps across 6 phases: authentication → mint → lock (spoke-a) → lock (spoke-b) → settlement → verification.

**Optional environment variables:**

| Variable | Default | Description |
|----------|---------|-------------|
| `BANK_A_URL` | `http://localhost:18080/api/v1` | Bank-A API URL |
| `BANK_B_URL` | `http://localhost:28080/api/v1` | Bank-B API URL |
| `CB_A_URL` | `http://localhost:38080/api/v1` | Central-Bank-A API URL |
| `MINT_AMOUNT` | `10000000` | Amount minted per bank |
| `LOCK_AMOUNT` | `1000000` | Amount locked per HTLC |
| `RELAY_SETTLE_TIMEOUT` | `30` | Seconds to wait for automatic relay settlement |

Per-entity scripts:

```bash
./tryouts/tryout-spoke-a-bank-a.sh   # Onboarding + payment Bank-A
./tryouts/tryout-spoke-a-bank-c.sh   # Onboarding + payment Bank-C
./tryouts/tryout-spoke-b-bank-b.sh   # Onboarding + payment Bank-B
./tryouts/tryout-spoke-b-bank-d.sh   # Onboarding + payment Bank-D
```

---

### Teardown

```bash
# Stop both spokes + relay (shared infrastructure kept running)
make spoke-all-down

# Stop frontends
make frontend-spoke-all-down

# Stop shared infrastructure (Keycloak, PostgreSQL, Redis)
make deploy.down-infra

# Stop a single spoke
make spoke-a-down    # backend + Paladin + Besu spoke-a
make spoke-b-down    # backend + Paladin + Besu spoke-b
```

> **Warning:** `deploy.down-infra` removes PostgreSQL volumes (`-v`). Database data will be lost.

---

## Scenario B — International Hub

> **Status:** implemented and runnable. Deployed from its own tree and documented in its own runbook — this document covers Scenario A only.

Scenario B adds a neutral hub network (chain 1337) with an AMM pool for FX settlement between sovereign spokes. It is a separate product under [`scenario-b/`](../../../scenario-b/README.md), with its own contracts, services, provisioning toolkit and runbooks. Scenario isolation is a constitutional rule of this repository: the two trees are deliberately not shared, and nothing in this runbook applies to Scenario B.

For Scenario B deployment, see:

- [`scenario-b/docs/runbooks/deployment-runbook.md`](../../../scenario-b/docs/runbooks/deployment-runbook.md) — the deployment procedure
- [`scenario-b/docs/runbooks/environment-setup.md`](../../../scenario-b/docs/runbooks/environment-setup.md) — prerequisites and port reference
- [`scenario-b/samples/deploy-all.sh`](../../../scenario-b/samples/deploy-all.sh) — the sample deployment (hub + two spokes + four commercial banks) driven by the `cbweb3b` toolkit

Scenario B's contracts live in [`scenario-b/contracts/src/`](../../../scenario-b/contracts/src/), **not** in Scenario A's tree. Alongside `AutomatedMarketMaker.sol`, `ManualOracle.sol` and `FXAgreement.sol` it carries the hub registries — `PairRegistry.sol`, `LiquidityCommitRegistry.sol`, `CurrencyRegistry.sol` and `SpokeBridge.sol` — which have no Scenario A counterpart.

---

## Troubleshooting

### Keycloak does not initialize

```bash
docker logs cbweb3-keycloak --tail 50
# Verify PostgreSQL is healthy
docker ps | grep postgres
```

If PostgreSQL takes too long to become ready, increase `KEYCLOAK_READY_ATTEMPTS`:
```bash
KEYCLOAK_READY_ATTEMPTS=60 make deploy.up-infra
```

### Contract deployment fails

```bash
# Verify Besu is producing blocks
curl -s -X POST http://localhost:8645 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Verify variables are populated
cat contracts/.env
```

Make sure `DEPLOYER_PRIVATE_KEY`, `ADMIN_ADDRESS`, and `CENTRAL_BANK_ADDRESS` are set in `contracts/.env`.

### Paladin fails to start

```bash
docker logs <paladin-container-name> --tail 100
# Clean volumes and retry
make paladin.clean-volumes-spoke-a
make paladin.start-spoke-a
```

### Backend service crashlooping

```bash
# View logs for a specific service
docker logs cbweb3-api-gateway-bank-a --tail 50

# Validate the compose file before starting
make deploy.validate-backend-bank-a
```

Check that contract addresses were synced: `deploy/local/paladin/spoke-a/.deployed-addrs.env`.

### Regenerate PKI

```bash
make pki.clean
make pki.gen-all FORCE=1
```

### Port already in use

Check that no other process is occupying the critical ports:

```bash
lsof -i :8081   # Keycloak
lsof -i :5432   # PostgreSQL
lsof -i :8645   # Besu spoke-a
lsof -i :18080  # API bank-a
```
