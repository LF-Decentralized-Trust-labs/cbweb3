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

Scenario B adds an independent International Hub network on top of the two domestic spokes from Scenario A. The Hub runs on its own Besu/QBFT network (chain **1337**, RPC port **8845**) that is topologically isolated from both spokes. Spoke-A runs on chain 1338 (port 8645) and Spoke-B on chain 1339 (port 8745). All shared contracts (AMM, registries, FX agreement, tokens) are deployed exclusively to the Hub.

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
| **Besu Hub** | 8845 | Chain **1337** — independent International Hub network (see Phase 3) |
| **Besu Spoke-A** | 8645 | Chain 1338 — domestic spoke only |
| **Besu Spoke-B** | 8745 | Chain 1339 |

> This phase waits for the Keycloak initialization script to complete (up to 10 minutes). Keycloak creates all realms, clients, and writes `KC_CLIENT_SECRET` values to `backend/config/.env.infra.*` files automatically.

---

### Phase 3 — Besu networks (Hub, Spoke-A, and Spoke-B)

All three Besu networks start as part of `make scenario-b.up-infra`. The Hub network **must be running before contracts are deployed** (Phase 4 deploys the shared contract suite to it).

**Important: the Hub must start first.** `make scenario-b.up-infra` starts networks in the order Hub → Spoke-A → Spoke-B automatically (see `deploy.up-besu` in `make/10-deploy.mk`).

To verify blocks are being produced on all three networks:

```bash
# International Hub — chain 1337 (independent neutral network)
curl -s -X POST http://localhost:8845 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' | jq .result
# Expect: "0x539"  (1337 in hex)

curl -s -X POST http://localhost:8845 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result
# Re-run after a few seconds — block number must advance (2-second block period)

# Spoke-A — chain 1338
curl -s -X POST http://localhost:8645 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result

# Spoke-B — chain 1339
curl -s -X POST http://localhost:8745 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .result
```

To start only the Hub (without the full stack) for independent verification:

```bash
make deploy.up-hub-besu
# Brings up ONLY the Hub network — no spokes, no backend services
```

#### Validator Configuration

The sandbox uses a **single-validator** Hub topology (`NODES=1` in `deploy/local/hub-besu/.env.network`, see [research.md Decision 3](../../specs/001-hub-network-isolation/research.md)). A single validator is sufficient for prototype and integration testing: the sole node acts as both bootnode and validator, producing 2-second QBFT blocks.

**For production deployments**, the single-validator topology is explicitly not recommended. Production requires a Byzantine-fault-tolerant validator set with **n ≥ 3f+1** nodes, where `f` is the maximum number of faulty nodes you wish to tolerate. Minimum practical configuration for 1 fault tolerance: **4 validators**. Additional steps for a multi-validator Hub:

1. Increase `NODES=4` (or your target count) in `hub-besu/.env.network`
2. The `startBesu.sh` script calls `besu operator generate-blockchain-config` with the updated count — rerun it to regenerate `genesis/genesis.json` with all validator keys in the QBFT `extraData` field
3. Each additional node needs its own `nodes/<name>/data/key` and `key.pub` provisioned and started with `--bootnodes=<hub-validator-enode>`
4. Ensure all validator nodes are accessible to each other via their P2P port (default 31503; add ports for subsequent nodes)

Until multi-validator is configured, the Hub is a convenience prototype that cannot tolerate node failure.

---

### Phase 4 — Smart contracts (hub + spokes)

```bash
make scenario-b.deploy-contracts
```

This target compiles all Solidity contracts via Foundry and deploys them in the following order:

1. **Hub contracts** (deployed to chain **1337** / `HUB_RPC_URL` = `http://localhost:8845` — the independent Hub network):
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

#### TVL Reconciliation Baseline

After a fresh deployment (Phase 4 just run, no cross-chain activity yet), the Hub-minted token supply **must be exactly zero**. This is the TVL reconciliation baseline — it confirms no orphaned mints from a prior deployment are present and the Hub is starting clean.

Verify immediately after `contracts.deploy-hub` (or `scenario-b.deploy-contracts`):

```bash
# Read tCeBM_BRL address from synced config
TOKEN_BRL=$(grep '^HUB_TOKEN_A_ADDRESS=' backend/config/.env.infra.bank-a | cut -d= -f2-)
TOKEN_EUR=$(grep '^HUB_TOKEN_B_ADDRESS=' backend/config/.env.infra.bank-a | cut -d= -f2-)

# Verify zero supply on the independent Hub (chain 1337, port 8845)
cast call $TOKEN_BRL "totalSupply()(uint256)" --rpc-url http://localhost:8845
# Expect: 0

cast call $TOKEN_EUR "totalSupply()(uint256)" --rpc-url http://localhost:8845
# Expect: 0
```

**Distinguishing fresh start from restart with existing state:**

| Checkpoint | Expected Hub token supply | Explanation |
|---|---|---|
| `fresh-startup` (just after deploy) | **0** | No cross-chain activity has occurred; Hub has never minted |
| `post-lock-event` (after first spoke lock) | **> 0** | Hub has minted in response to a spoke lock event |
| `restart-with-state` (containers restarted, data preserved) | **unchanged from pre-restart** | Chain state persists in `nodes/hub-validator/data/` across restarts |

If supply is non-zero immediately after a deploy (fresh start), a stale container data directory may contain chain state from a previous run. Run `make deploy.down-hub-besu` (which removes the old node data) and re-run `make deploy.up-hub-besu` to start from a clean genesis, then redeploy contracts.

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
- [ ] Besu Hub (chain **1337**, port 8845) reports `eth_chainId` = `0x539` and blocks are advancing
- [ ] Besu Spoke-A (chain 1338, port 8645) is producing blocks
- [ ] Besu Spoke-B (chain 1339, port 8745) is producing blocks
- [ ] Hub has zero peers with either spoke (`net_peerCount` on port 8845 = `0x0`)
- [ ] Contract addresses synced (verify `backend/config/.env.infra.bank-a` has `AMM_CONTRACT_ADDRESS` set)
- [ ] `HUB_CHAIN_ID` in all backend `.env.infra.*` files = `1337` (not `1338`)
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

---

## Decommissioning the Legacy Hub-on-Spoke-A Deployment

Prior to this change (feature branch `001-hub-network-isolation`), Scenario B ran hub contracts on Spoke-A's Besu node (chain 1338, port 8645). This configuration was a placeholder — the hub and Spoke-A shared a network identity, making cross-chain relay verification degenerate.

**The prior Hub-on-Spoke-A deployment is now retired and non-authoritative.** Do not use chain-1338 addresses as Hub contract references. The new sole source of truth for all Hub contracts is the independent Hub network (chain 1337, port 8845).

### Legacy contract addresses (for reference only — retired)

These addresses were deployed to chain 1338 as the "Hub" before this change. They are listed here only to support decommissioning workflows (e.g., confirming that on-chain state was not migrated). Do not call these addresses for any new operation:

```
# Legacy Hub-on-Spoke-A addresses (chain 1338) — NON-AUTHORITATIVE
# See contracts/broadcast/CBWeb3Hub.s.sol/1338/ for historic broadcast records
# (if any were saved — these are NOT valid Hub addresses post-cutover)
```

The current authoritative Hub addresses are found in `contracts/broadcast/CBWeb3Hub.s.sol/1337/run-latest.json` and propagated to `backend/config/.env.infra.*.` via `contracts.sync-addresses`.

### Cutover procedure (clean rebuild — no state migration)

There is no on-chain state to migrate. The prototype carries no production data, and the clarification decision (`/speckit.clarify`) explicitly chose a clean rebuild. Proceed as follows:

1. **Stop everything**: `make scenario-b.down`
2. **Bring up only the Hub**: `make deploy.up-hub-besu`
3. **Verify chain 1337**: `eth_chainId` must return `0x539`
4. **Deploy contracts to Hub**: `make contracts.deploy-hub`
5. **Bring up spokes and relay**: continue from Phase 3 of this runbook
6. **Re-sync addresses**: `make contracts.sync-addresses` (already called by `scenario-b.deploy-contracts`)
7. **Verify zero stale defaults**: `grep -rn "HUB_CHAIN_ID" backend contracts make | grep 1338` must return 0 results

### Detecting and correcting a stale locally cached 1338 value

If a service logs `HUB_CHAIN_ID not set; defaulting to 1337` at startup, the `HUB_CHAIN_ID` env var is absent from the entity's `.env.infra.*` file. After running `contracts.sync-addresses`, this variable should be populated with `1337`. If it shows `1338`, you have a stale cached file:

```bash
# Check for stale 1338 defaults
grep -rn "HUB_CHAIN_ID=1338" backend/config/

# Fix: re-run address sync (overwrites stale values)
make contracts.sync-addresses

# Verify all entity configs now show 1337
grep -rn "HUB_CHAIN_ID" backend/config/.env.infra.* | grep -v "1338"
```

After fixing, restart the affected backend services.

---

## Known limitations

| Limitation | Impact | Workaround |
|-----------|--------|-----------|
| Hub sandbox uses single validator | Hub cannot tolerate node failure in prototype | For production, provision a multi-validator QBFT set (n ≥ 3f+1) — see Phase 3 Validator Configuration above |
| MLP stack requires explicit opt-in | MLP backend does not start automatically | Pass `ENABLE_MLP=true` and run `make scenario-b.up-backend-mlp` |
| US3 (commercial bank swap) partially implemented | Full cross-currency swap lifecycle requires governance account for some steps | Use CB-A governance account for testing; see tryout scripts |
| Sovereign liquidity auto-recovery not implemented | Failed matched commits leave the pool in a stuck state | Manual re-commit by both CBs |
| Paladin/Zeto privacy not available on hub | Hub AMM transactions are not privacy-preserving | Privacy is enforced per-spoke only |
