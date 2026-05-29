# Deployment Runbook — CBWeb3 Platform · Scenario B

> **Project:** RG-T4567 · Suboperation ATN/KS-21330-RG
> **Authors:** Lucas Campelo, Samuel Venzi
> **Date:** 2026-05-29
>
> **Deliverable 9** · CBDC System Deployment

> **Status: Work in progress.** Scenario B is implemented but not 100% complete. The commercial bank swap flow (US3) is partially implemented — use a governance account for full end-to-end testing. Sovereign liquidity reconciliation lacks automatic recovery for failed matched commits. Paladin/Zeto privacy is available on spokes but not on the hub. See [Known limitations](#known-limitations) below.

This runbook describes the complete procedure for bringing up the CBWeb3 Scenario B environment (International Hub with AMM/FX), from prerequisites through operational endpoint verification and smoke tests.

---

## Table of Contents

- [Scope](#scope)
- [Prerequisites](#prerequisites)
- [Command overview](#command-overview)
- [Step-by-step deployment](#step-by-step-deployment)
  - [Phase 1 — PKI certificate generation](#phase-1--pki-certificate-generation)
  - [Phase 2 — Shared infrastructure](#phase-2--shared-infrastructure)
  - [Phase 3 — Besu networks (Spoke-A and Spoke-B)](#phase-3--besu-networks-spoke-a-and-spoke-b)
  - [Phase 4 — Smart contracts (hub + spokes)](#phase-4--smart-contracts-hub--spokes)
  - [Phase 5 — Sovereign pair seed (optional)](#phase-5--sovereign-pair-seed-optional)
  - [Phase 6 — Backend services](#phase-6--backend-services)
  - [Phase 7 — MLP stack (optional)](#phase-7--mlp-stack-optional)
  - [Phase 8 — Cacti relay](#phase-8--cacti-relay)
- [Health verification](#health-verification)
- [Smoke tests](#smoke-tests)
  - [HTLC cross-spoke smoke test](#htlc-cross-spoke-smoke-test)
  - [AMM smoke test (quote + swap)](#amm-smoke-test-quote--swap)
- [End-to-end tryouts](#end-to-end-tryouts)
- [Teardown](#teardown)
- [Troubleshooting](#troubleshooting)
- [Known limitations](#known-limitations)

---

## Scope

| Scenario | Status | Description |
|----------|--------|-------------|
| **Scenario B** | Implemented (partially) | International Hub with AMM/FX, cooperative sovereign liquidity, PairRegistry, cross-spoke bridge via Cacti |

Scenario B adds a hub network on top of the two spokes from Scenario A. In local development, the hub shares the same Besu node as Spoke-A (chain 1338, RPC port 8645). Spoke-B runs separately (chain 1339, RPC port 8745).

---

## Prerequisites

Install all tools before proceeding. See the detailed guide in [`environment-setup.md`](environment-setup.md).

| Tool | Minimum version | Check |
|------|----------------|-------|
| Docker + Docker Compose | Docker 24+ | `docker --version` |
| GNU Make | 3.81+ | `make --version` |
| Go | 1.26+ | `go version` |
| Node.js | 20+ | `node --version` |
| npm | 10+ | `npm --version` |
| Foundry (`forge`, `cast`) | nightly | `forge --version` |
| k6 | 0.50+ | `k6 version` |
| `jq` | 1.6+ | `jq --version` |
| `openssl` | 3.x | `openssl version` |

**Recommended Docker resources:** 16 GB RAM and 8 CPUs (the full stack runs approximately 35 containers, including hub + both spokes + relay + MLP).

All Makefile targets below must be run from the `scenario-b/` directory:

```bash
cd scenario-b
```

---

## Command overview

```bash
# One-shot full stack
make scenario-b.up      # infra + contracts + seed + relayer + all backends
make scenario-b.down    # teardown everything
make scenario-b.restart # down + up

# Step-by-step
make scenario-b.prepare-pki      # Phase 1 — X.509 certificates
make scenario-b.up-infra         # Phase 2 — Keycloak + Postgres + Redis
make scenario-b.deploy-contracts # Phase 4 — hub + spoke contracts + sync addresses
make scenario-b.seed-sovereign-pair  # Phase 5 — LiquidityCommitRegistry + wrapped tokens
make scenario-b.build-backend-images
make scenario-b.up-backend       # Phase 6 — all 6 entity backends
make scenario-b.up-backend-mlp   # Phase 7 — MLP stack (requires ENABLE_MLP=true)
make scenario-b.up-relayer       # Phase 8 — Cacti LiquidityCommitWatcher
```

---

## Step-by-step deployment

### Phase 1 — PKI certificate generation

```bash
make scenario-b.prepare-pki
```

Generates X.509 certificates (EC prime256v1) for all seven entities in `backend/config/pki/`. This phase is **idempotent**: if certificate files already exist, the phase is skipped automatically.

Files generated per entity: `<entity>-ca.key`, `<entity>-ca.crt`, `<entity>.key`, `<entity>.csr`, `<entity>.crt`.

To force regeneration:

```bash
make scenario-b.prepare-pki FORCE=1
```

---

### Phase 2 — Shared infrastructure

```bash
make scenario-b.up-infra
```

Brings up shared services via Docker Compose:

| Service | Port | Notes |
|---------|------|-------|
| **Keycloak** | 8081 | 7 realms created: bank-a, bank-b, bank-c, bank-d, central-bank-a, central-bank-b, mlp |
| **PostgreSQL** | 5432 | 7 databases created: cbweb3\_bank\_a, cbweb3\_bank\_b, cbweb3\_bank\_c, cbweb3\_bank\_d, cbweb3\_central\_bank\_a, cbweb3\_central\_bank\_b, cbweb3\_mlp |
| **Redis** | 6379 | 7 logical DBs (one per entity) |
| **Besu Spoke-A** | 8645 | Chain 1338 — also serves as Hub in local dev |
| **Besu Spoke-B** | 8745 | Chain 1339 |

> This phase waits for the Keycloak initialization script to complete (up to 10 minutes). Keycloak creates all realms, clients, and writes `KC_CLIENT_SECRET` values to `backend/config/.env.infra.*` files automatically.

---

### Phase 3 — Besu networks (Spoke-A and Spoke-B)

Besu nodes start as part of `make scenario-b.up-infra` above. To verify blocks are being produced:

```bash
# Spoke-A (also Hub in local dev) — chain 1338
curl -s -X POST http://localhost:8645 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result

# Spoke-B — chain 1339
curl -s -X POST http://localhost:8745 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result
```

> **Local dev note:** The Hub Besu network is co-located with Spoke-A (same RPC endpoint, port 8645, chain 1338). No separate hub Besu node exists in local development. Hub contracts are deployed to the Spoke-A node. In production, the hub would run on a separate network.

---

### Phase 4 — Smart contracts (hub + spokes)

```bash
make scenario-b.deploy-contracts
```

This target compiles all Solidity contracts via Foundry and deploys them in the following order:

1. **Hub contracts** (deployed to chain 1338 / `HUB_RPC_URL`):
   - `IdentityRegistry` — hub participant registry; admin, CB-A, and CB-B receive `GOVERNANCE_ROLE`
   - `CurrencyRegistry` — tracks registered hub currencies
   - `PairRegistry` — bilateral CB pair approval lifecycle
   - `TokenizedCentralBankMoney` (tCeBM-BRL) — hub BRL token
   - `TokenizedCentralBankMoney` (tCeBM-EUR) — hub EUR token
   - `ManualOracle` — FX rate oracle; `GOVERNANCE_ROLE` required for `setRate()`
   - `AutomatedMarketMaker` — constant-product AMM using both hub tokens
   - `LiquidityCommitRegistry` — commit-reveal for sovereign CB liquidity; 72-hour TTL
   - `FXAgreement` (hub) — hub-side FX agreement lifecycle
   - `SpokeBridge` (per spoke) — Lock&Mint / Burn&Unlock bridge

2. **Spoke contracts** (deployed per spoke, same as Scenario A):
   - `IdentityRegistry` (per spoke)
   - `TokenizedCentralBankMoney` (per spoke)
   - `FiatCentralBankMoney` (per spoke)
   - `HashTimeLockedContract` (per spoke)
   - `SpokeBridge` (per spoke)

3. Address sync — `make scenario-b.deploy-contracts` internally runs `contracts.sync-addresses`, which propagates all deployed addresses into the backend `.env.infra.*` files and `interop/hub-and-spoke/cacti/.env`.

Required variables in `scenario-b/contracts/.env` before running this target — see [contract-configuration.md](contract-configuration.md).

---

### Phase 5 — Sovereign pair seed (optional)

```bash
make scenario-b.seed-sovereign-pair
```

Deploys the `LiquidityCommitRegistry` with the first sovereign pair (BRL-EUR) and the wrapped token configurations required for the cooperative liquidity flows (US1, US2). This step is optional if you only want to test spoke-to-spoke HTLC without AMM liquidity.

---

### Phase 6 — Backend services

```bash
make scenario-b.build-backend-images
make scenario-b.up-backend
```

Starts four microservices per entity (6 entities = 24 containers total):

| Service | Protocol | Responsibility |
|---------|----------|---------------|
| `api-gateway` | REST (HTTP) | External entry point; routes to internal gRPC services |
| `auth` | gRPC | Identity, login, wallets, PKI |
| `compliance` | gRPC | Onboarding, AML, participant registry |
| `payment-orchestrator` | gRPC | HTLC, FX, AMM swap execution, escrow |

Docker Compose files per entity:

```
backend/docker-compose-backend.bank-a.yaml
backend/docker-compose-backend.bank-b.yaml
backend/docker-compose-backend.central-bank-a.yaml
backend/docker-compose-backend.central-bank-b.yaml
backend/docker-compose-backend.bank-c.yaml  (optional)
backend/docker-compose-backend.bank-d.yaml  (optional)
```

---

### Phase 7 — MLP stack (optional)

```bash
ENABLE_MLP=true make scenario-b.up-backend-mlp
```

Starts the Multilateral Liquidity Provider (MLP) backend stack. This includes the `api-gateway`, `auth`, `compliance`, and `payment-orchestrator` services for the MLP entity on port 68080.

The MLP stack requires `ENABLE_MLP=true` and uses a separate Keycloak realm (`mlp`), PostgreSQL database (`cbweb3_mlp`), and Redis logical DB.

To stop the MLP stack independently:

```bash
make scenario-b.down-backend-mlp
```

---

### Phase 8 — Cacti relay

```bash
make scenario-b.up-relayer
```

Starts the Hyperledger Cacti `LiquidityCommitWatcher` relay (TypeScript, Node.js 20) that:

- Monitors `LogHTLCLocked` and `LogHTLCClaimed` events on both Spoke-A and Spoke-B via `PluginLedgerConnectorBesu`
- Calls `PaymentOrchestratorService.SettleHTLC` gRPC on the target spoke's payment-orchestrator to propagate the secret and complete the cross-spoke bridge cycle
- Exposes a REST API on port 4000 consumed by the Go payment-orchestrator adapter

The relay configuration lives in `interop/hub-and-spoke/cacti/.env`. Addresses are synced automatically by `make scenario-b.deploy-contracts`.

To stop the relay independently:

```bash
make scenario-b.down-relayer
```

---

## Health verification

After `make scenario-b.up`, verify the main endpoints:

```bash
# Keycloak
curl -s http://localhost:8081/realms/master | jq .realm

# API Gateways — all entities
curl -s http://localhost:18080/healthz   # bank-a
curl -s http://localhost:28080/healthz   # bank-b
curl -s http://localhost:38080/healthz   # central-bank-a
curl -s http://localhost:48080/healthz   # bank-c
curl -s http://localhost:58080/healthz   # bank-d
curl -s http://localhost:60080/healthz   # central-bank-b
curl -s http://localhost:68080/healthz   # mlp (if enabled)

# Besu RPC — block production
curl -s -X POST http://localhost:8645 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result

curl -s -X POST http://localhost:8745 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result

# Cacti relay
curl -s http://localhost:4000/api/v1/health
```

**Operational checklist:**

- [ ] Keycloak responds at `http://localhost:8081/realms/master`
- [ ] All 6 entity API gateways return 200 on `/healthz`
- [ ] Besu Spoke-A/Hub (chain 1338) is producing blocks
- [ ] Besu Spoke-B (chain 1339) is producing blocks
- [ ] Contract addresses synced (verify `backend/config/.env.infra.bank-a` has `AMM_CONTRACT_ADDRESS` set)
- [ ] Cacti relay healthy at port 4000
- [ ] `docker ps` shows all containers in `Up` state

---

## Smoke tests

### HTLC cross-spoke smoke test

Verify cross-spoke atomic settlement (same as Scenario A, no hub involvement):

```bash
cd scenario-b
bash tryouts/tryout-scenario-b-e2e.sh us3
```

This exercises the HTLC lock → Cacti relay → HTLC claim cycle between Spoke-A and Spoke-B.

---

### AMM smoke test (quote + swap)

Verify hub AMM quote and swap execution using the CB-A governance account:

```bash
# 1. Authenticate as CB-A
CB_A_TOKEN=$(curl -s -X POST \
  http://localhost:8081/realms/central-bank-a/protocol/openid-connect/token \
  -d "grant_type=client_credentials&client_id=central-bank-a-client&client_secret=<secret>" \
  | jq -r .access_token)

# 2. Check pool status
curl -s http://localhost:38080/api/v2/amm/pool/BRL-EUR/status \
  -H "Authorization: Bearer $CB_A_TOKEN" | jq .

# 3. Get a swap quote (exact-output)
curl -s "http://localhost:38080/api/v2/amm/quote/exact-output?pool_pair=BRL-EUR&amount_out=1000" \
  -H "Authorization: Bearer $CB_A_TOKEN" | jq .

# 4. List active pairs
curl -s http://localhost:38080/api/v2/amm/pairs \
  -H "Authorization: Bearer $CB_A_TOKEN" | jq .
```

Expected responses: pool status shows `ACTIVE` after both CBs have committed liquidity; quote returns `amount_in` and `price_impact`; pairs list returns `BRL-EUR` with status `ACTIVE`.

---

## End-to-end tryouts

Full scenario tryouts are in `scenario-b/tryouts/`:

```bash
# All user stories
bash tryouts/tryout-scenario-b-e2e.sh all

# Individual user stories
bash tryouts/tryout-scenario-b-e2e.sh us1   # cooperative sovereign liquidity
bash tryouts/tryout-scenario-b-e2e.sh us2   # MLP bilateral liquidity
bash tryouts/tryout-scenario-b-e2e.sh us3   # commercial bank cross-currency swap
bash tryouts/tryout-scenario-b-e2e.sh us5   # PairRegistry bilateral approval

# Additional scenario scripts
bash tryouts/tryout-commercial-swap-e2e.sh
bash tryouts/tryout-cross-currency-full-lifecycle.sh
```

To skip the `make scenario-b.up` step if the stack is already running:

```bash
SKIP_UP=1 bash tryouts/tryout-scenario-b-e2e.sh all
```

---

## Teardown

```bash
# Full teardown
make scenario-b.down

# Partial teardown
make scenario-b.down-backend      # stop all 6 entity backends
make scenario-b.down-backend-mlp  # stop MLP stack only
make scenario-b.down-relayer      # stop Cacti relay
make scenario-b.down-infra        # stop Keycloak, Postgres, Redis, Besu nodes
```

> **Warning:** `make scenario-b.down-infra` removes Docker volumes (`-v`). All database data (Keycloak realms, Postgres records) will be lost. Re-run the full `make scenario-b.up` to restore.

---

## Troubleshooting

### Keycloak does not initialize

```bash
docker logs cbweb3-keycloak --tail 50
docker ps | grep postgres
```

If PostgreSQL takes too long to become ready, increase the retry count:

```bash
KEYCLOAK_READY_ATTEMPTS=60 make scenario-b.up-infra
```

### Contract deployment fails

```bash
# Verify Spoke-A/Hub is producing blocks
curl -s -X POST http://localhost:8645 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Check contracts/.env variables
cat scenario-b/contracts/.env
```

Ensure `HUB_RPC_URL`, `DEPLOYER_PRIVATE_KEY`, `ADMIN_ADDRESS`, `CENTRAL_BANK_ADDRESS`, and `CENTRAL_BANK_B_ADDRESS` are all set in `contracts/.env`.

### Backend service crashlooping

```bash
# View logs for a specific service
docker logs cbweb3-api-gateway-bank-a --tail 50
docker logs cbweb3-payment-orchestrator-central-bank-a --tail 50
```

Check that contract addresses were synced: look for `AMM_CONTRACT_ADDRESS` in `backend/config/.env.infra.central-bank-a`.

### AMM pool stuck in PENDING state

A pool enters `PENDING_COUNTERPART` when only one CB has committed. If the counterpart commit does not arrive within 72 hours, the commit expires and the pool returns to `EMPTY`. To re-seed:

1. Have both CBs submit a fresh `/api/v2/amm/liquidity/commit` with matching `pool_pair`
2. The second commit auto-matches and transitions the pool to `ACTIVE`

If a commit is in the `MATCHED` state but the pool did not activate, sovereign liquidity auto-recovery is not yet implemented. Manual intervention is required — contact the platform operator.

### Cacti relay not forwarding events

```bash
docker logs cbweb3-cacti-relay --tail 50
curl -s http://localhost:4000/api/v1/health
```

Verify `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` is set correctly in `interop/hub-and-spoke/cacti/.env`. Re-run `make scenario-b.deploy-contracts` to re-sync addresses.

### Port already in use

```bash
lsof -i :8081   # Keycloak
lsof -i :5432   # PostgreSQL
lsof -i :8645   # Besu Spoke-A / Hub
lsof -i :8745   # Besu Spoke-B
lsof -i :18080  # API bank-a
lsof -i :38080  # API central-bank-a
lsof -i :4000   # Cacti relay
```

---

## Known limitations

| Limitation | Impact | Workaround |
|-----------|--------|-----------|
| Hub shares Spoke-A Besu node in local dev | Hub and Spoke-A cannot be isolated; chain 1338 is used for both | Deploy to separate chains in staging/production |
| MLP stack requires explicit opt-in | MLP backend does not start automatically | Pass `ENABLE_MLP=true` and run `make scenario-b.up-backend-mlp` |
| US3 (commercial bank swap) partially implemented | Full cross-currency swap lifecycle requires governance account for some steps | Use CB-A governance account for testing; see tryout scripts |
| Sovereign liquidity auto-recovery not implemented | Failed matched commits leave the pool in a stuck state | Manual re-commit by both CBs |
| Paladin/Zeto privacy not available on hub | Hub AMM transactions are not privacy-preserving | Privacy is enforced per-spoke only |
