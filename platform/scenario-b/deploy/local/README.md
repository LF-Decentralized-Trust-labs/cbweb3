# Local Deployment (CBWeb3 Platform — Two Spokes)

This folder contains the local runtime setup for the CBWeb3 platform, focused on five main infrastructure components:

- Besu private networks (hub, spoke A, spoke B)
- Keycloak (identity and access management)
- PostgreSQL (relational storage for local services)
- Redis (cache and message-oriented local support)
- Paladin nodes (privacy-preserving smart contract execution, spoke A and spoke B)

```
spoke-a (chain 1338)
├── Besu (chain 1338)          ← 3 nodes: central-bank-a, bank-a, bank-c
spoke-b (chain 1339)
├── Besu (chain 1339)          ← 3 nodes: central-bank-b, bank-b, bank-d
shared infra
├── Keycloak                   ← 6 realms
├── PostgreSQL                 ← 7 databases
├── Redis                      ← 6 logical DBs (0-5)
```

Per-entity backend stacks (api-gateway / auth / compliance) use dedicated host
ports and internal Docker networks; see **Port allocations** below.

## Components

### 1) Besu networks — spoke-besu-a and spoke-besu-b

Two separate Besu QBFT networks: **spoke-a** (chain **1338**) and **spoke-b**
(chain **1339**), each with **three fixed named nodes**, one per entity.

| Spoke   | Directory      | Chain ID | Docker network           | Container prefix        |
|---------|----------------|----------|--------------------------|-------------------------|
| spoke-a | `spoke-besu-a/` | `1338`   | `spoke_a_besu_network`   | `cbweb3-spoke-a-besu`   |
| spoke-b | `spoke-besu-b/` | `1339`   | `spoke_b_besu_network`   | `cbweb3-spoke-b-besu`   |

Scripts: `startBesu.sh` / `stopBesu.sh` (and `addNewNode.sh` where present).

| Spoke   | Node (entity)    | Container name                         | Host RPC | Host P2P |
|---------|------------------|----------------------------------------|----------|----------|
| spoke-a | central-bank-a   | `cbweb3-spoke-a-besu.central-bank-a`   | `8645`   | `31303`  |
| spoke-a | bank-a           | `cbweb3-spoke-a-besu.bank-a`           | `8646`   | `31304`  |
| spoke-a | bank-c           | `cbweb3-spoke-a-besu.bank-c`           | `8647`   | `31305`  |
| spoke-b | central-bank-b   | `cbweb3-spoke-b-besu.central-bank-b`   | `8745`   | `31403`  |
| spoke-b | bank-b           | `cbweb3-spoke-b-besu.bank-b`           | `8746`   | `31404`  |
| spoke-b | bank-d           | `cbweb3-spoke-b-besu.bank-d`           | `8747`   | `31405`  |

Each backend service connects to its own node’s RPC endpoint via `BESU_RPC_URL`
in `.env.infra.*`. When `BLOCKCHAIN_CLIENT=besu`, backends use the container
name (Docker DNS) on the spoke’s Besu network.

**Besu lifecycle (repository root):**

- `make deploy.up-spoke-a` / `make deploy.down-spoke-a` — spoke-a only  
- `make deploy.up-spoke-b` / `make deploy.down-spoke-b` — spoke-b only  
- `make deploy.up-besu` — both spokes (spoke-a then spoke-b)  
- `make deploy.down-besu` — both spokes (spoke-b then spoke-a)  

### 2) Keycloak

Keycloak is started by Docker Compose and initialized by
[keycloak/init.sh](keycloak/init.sh).

On startup, the script:

1. Waits for Keycloak readiness.
2. Authenticates with admin credentials.
3. Creates **six** realms, each with one OIDC client:
   - **`bank-a`** / `bank-a-client`
   - **`bank-b`** / `bank-b-client`
   - **`central-bank-a`** / `central-bank-a-client`
   - **`bank-c`** / `bank-c-client`
   - **`bank-d`** / `bank-d-client`
   - **`central-bank-b`** / `central-bank-b-client`
4. Fetches each client secret and writes runtime infra env files:
   - `backend/config/.env.infra.bank-a`
   - `backend/config/.env.infra.bank-b`
   - `backend/config/.env.infra.central-bank-a`
   - `backend/config/.env.infra.bank-c`
   - `backend/config/.env.infra.bank-d`
   - `backend/config/.env.infra.central-bank-b`

When you run `make deploy.up-infra` or `make dev.up` from the repository root,
[`make/10-deploy.mk`](../../make/10-deploy.mk) waits in two phases:

1. **HTTP readiness**: polls `http://localhost:${KEYCLOAK_PORT}/realms/master`.
2. **Init completion**: polls container logs until `KEYCLOAK_INIT_DONE` appears.

Override waits (defaults: 40×3s for HTTP, 600s for init):

- `KEYCLOAK_READY_ATTEMPTS`, `KEYCLOAK_WAIT_SLEEP_SEC`

### 3) PostgreSQL

PostgreSQL runs as a container managed by Compose with a persistent volume.
[postgres/init-multi-db.sh](postgres/init-multi-db.sh) creates **seven**
databases at first start (six application DBs + Keycloak):

- `cbweb3_bank_a`
- `cbweb3_bank_b`
- `cbweb3_bank_c`
- `cbweb3_bank_d`
- `cbweb3_central_bank_a`
- `cbweb3_central_bank_b`
- `cbweb3_keycloak`

### 4) Redis

Redis runs as a Compose service on `cbweb3_network`. Each entity uses a
dedicated Redis logical DB for nonce isolation:

| Entity         | Redis DB |
|----------------|----------|
| bank-a         | 0        |
| bank-b         | 1        |
| central-bank-a | 2        |
| bank-c         | 3        |
| bank-d         | 4        |
| central-bank-b | 5        |

## Port allocations (backend host ports)

| Entity         | Spoke   | API (api-gateway) | Auth gRPC | Compliance gRPC |
|----------------|---------|-------------------|-----------|-----------------|
| bank-a         | spoke-a | `18080`           | `19091`   | `19093`         |
| bank-b         | spoke-b | `28080`           | `29091`   | `29093`         |
| central-bank-a | spoke-a | `38080`           | `39091`   | `39093`         |
| bank-c         | spoke-a | `48080`           | `49091`   | `49093`         |
| bank-d         | spoke-b | `58080`           | `59091`   | `59093`         |
| central-bank-b | spoke-b | `60080`           | `60091`   | `60093`         |

### 5) Paladin nodes

Paladin provides privacy-preserving smart contract execution using zero-knowledge proofs (Zeto domain). Each spoke runs three Paladin nodes, one per Besu validator, in a 1-to-1 mapping:

| Paladin container | Besu node | RPC port | gRPC port | Metrics port |
|---|---|---|---|---|
| `paladin-spoke-a-cb` | bootnode (8645) | 31648 | 31649 | 9011 |
| `paladin-spoke-a-bank-a` | node-1 (8647) | 31658 | 31659 | 9012 |
| `paladin-spoke-a-bank-b` | node-2 (8648) | 31668 | 31669 | 9013 |
| `paladin-spoke-b-cb` | bootnode (8745) | 31748 | 31749 | 9021 |
| `paladin-spoke-b-bank-a` | node-1 (8747) | 31758 | 31759 | 9022 |
| `paladin-spoke-b-bank-b` | node-2 (8748) | 31768 | 31769 | 9023 |

Paladin nodes run on the same Docker network as Besu (`spoke_a_besu_network` / `spoke_b_besu_network`).

#### Setup steps (automated by `make setup-spoke-a` / `make setup-spoke-b`)

1. **Deploy contracts** — deploys a Paladin `IdentityRegistry` and a `ZetoFactory` (with `Zeto_Anon` implementation) to the spoke's Besu network. Contract addresses are written to `paladin/spoke-{a,b}/.deployed-addrs.env`.
2. **Generate TLS certificates** — creates self-signed P-256 certificates for each node's gRPC transport (`paladin/spoke-{a,b}/config/<node>/tls.{crt,key}`).
3. **Render configs** — substitutes contract addresses from `.deployed-addrs.env` into each node's `config.yaml.tmpl`, producing `config.yaml`.
4. **Register nodes** — calls `registerIdentity` and `setIdentityProperty(transport.grpc)` on the `IdentityRegistry` for all three nodes.
5. **Start containers** — brings up the three Paladin containers via Docker Compose.

#### Directory structure

```
paladin/
  artifacts/          # Paladin K8s artifact YAMLs (contract bytecode + link refs)
  generate-certs.sh   # TLS cert generation (SPOKE=spoke-a|spoke-b)
  render-configs.sh   # Config template rendering (SPOKE=spoke-a|spoke-b)
  scripts/            # Go programs for contract deployment and node registration
    cmd/
      deploy-registry/       # Deploys IdentityRegistry
      deploy-zeto-factory/   # Deploys ZetoFactory + Zeto_Anon impl
      register-nodes/        # Registers nodes in IdentityRegistry
    helpers.go               # Shared helpers (artifact reading, bytecode linking)
  spoke-a/
    docker-compose.yml
    config/{central-bank,bank-a,bank-b}/
      config.yaml.tmpl        # Template (placeholders for contract addresses)
      config.yaml             # Rendered at setup time
      tls.{crt,key}           # Generated at setup time
    .deployed-addrs.env       # Written by deploy scripts
  spoke-b/
    (same structure)
```

## How components communicate

- Besu nodes on a spoke communicate inside that spoke’s Docker network
  (`spoke_a_besu_network` / `spoke_b_besu_network`, created by the respective
  `startBesu.sh`).
- Keycloak, PostgreSQL and Redis communicate on `cbweb3_network` (external
  Docker network created by `deploy.create-shared-network`).
- Backend services within each entity communicate on a dedicated internal
  network per entity and reach infra via `cbweb3_network`.
- Local access:
  - Keycloak: `http://localhost:${KEYCLOAK_PORT}` (default `8081`)
  - PostgreSQL: `localhost:${POSTGRES_PORT}` (default `5432`)
  - Redis: `localhost:${REDIS_PORT}` (default `6379`)
  - Besu RPC (spoke-a): central-bank-a `http://localhost:8645`, bank-a `8646`, bank-c `8647`
  - Besu RPC (spoke-b): central-bank-b `http://localhost:8745`, bank-b `8746`, bank-d `8747`

## Configuration model

Runtime infra files (six entities):

- `backend/config/.env.infra.bank-a`
- `backend/config/.env.infra.bank-b`
- `backend/config/.env.infra.central-bank-a`
- `backend/config/.env.infra.bank-c`
- `backend/config/.env.infra.bank-d`
- `backend/config/.env.infra.central-bank-b`

These are generated/updated from `*.example` templates with Keycloak-specific
values and secrets. **`*.example` files are templates only**; runtime values are
written to `.env.infra.*` by `keycloak/init.sh`.

### Recommended setup

1. Copy and adjust the example files (one per entity), e.g.  
   `backend/config/.env.infra.bank-a.example` → `backend/config/.env.infra.bank-a`  
   (repeat for `bank-b`, `central-bank-a`, `bank-c`, `bank-d`, `central-bank-b`).
2. Start infra (Keycloak will regenerate the files automatically):
   ```bash
   make deploy.up-infra
   ```
3. Start the Besu networks (one or both spokes):
   ```bash
   make deploy.up-spoke-a
   make deploy.up-spoke-b
   # or: make deploy.up-besu
   ```
4. Start backend services per entity or all at once:
   ```bash
   make deploy.up-backend-entities
   ```

### Backend compose by entity

After starting shared infra, start backends with one compose file per entity:

- `backend/docker-compose-backend.bank-a.yaml`
- `backend/docker-compose-backend.bank-b.yaml`
- `backend/docker-compose-backend.central-bank-a.yaml`
- `backend/docker-compose-backend.bank-c.yaml`
- `backend/docker-compose-backend.bank-d.yaml`
- `backend/docker-compose-backend.central-bank-b.yaml`

All six backend files:

- Load entity settings from `backend/config/.env.infra.*`.
- Keep service-to-service traffic in a dedicated backend network per entity.
- Connect to shared infra through external network `cbweb3_network`.
- Use non-overlapping host ports so all entities can run simultaneously.

Commands (from repository root):

- Validate compose syntax:
  ```bash
  make deploy.validate-backend-entities
  ```
  (or `docker compose --env-file backend/config/.env.infra.<entity> -f backend/docker-compose-backend.<entity>.yaml config` per file.)
- Start / stop backends:
  ```bash
  make deploy.up-backend-entities
  make deploy.down-backend-entities
  ```

Per-entity Make shortcuts:

- `make deploy.validate-backend-bank-a` … `deploy.validate-backend-central-bank-b`
- `make deploy.up-backend-bank-a` … `deploy.up-backend-central-bank-b`
- `make deploy.down-backend-bank-a` … `deploy.down-backend-central-bank-b`

## Start and stop

From repository root:

| Target | Description |
|--------|-------------|
| `make dev.up` | Full environment: PKI + both Besu spokes + infra + contracts + all six backends. |
| `make dev.down` | Stop full environment in reverse order (PKI files kept; `deploy.down` tears down both Besu spokes). |
| `make dev.up-bank-a` | PKI (bank-a CA + commercial banks) + infra + **spoke-a** Besu + bank-a backend. |
| `make dev.down-bank-a` | Stop bank-a backend only. |
| `make dev.up-bank-b` | PKI (bank-b CA + commercial banks) + infra + **spoke-b** Besu + bank-b backend. |
| `make dev.down-bank-b` | Stop bank-b backend only. |
| `make dev.up-central-bank-a` | PKI (central-bank-a CA) + infra + **spoke-a** Besu + central-bank-a backend. |
| `make dev.down-central-bank-a` | Stop central-bank-a backend only. |
| `make dev.up-bank-c` | PKI (bank-c CA + commercial banks) + infra + **spoke-a** Besu + bank-c backend. |
| `make dev.down-bank-c` | Stop bank-c backend only. |
| `make dev.up-bank-d` | PKI (bank-d CA + commercial banks) + infra + **spoke-b** Besu + bank-d backend. |
| `make dev.down-bank-d` | Stop bank-d backend only. |
| `make dev.up-central-bank-b` | PKI (central-bank-b CA) + infra + **spoke-b** Besu + central-bank-b backend. |
| `make dev.down-central-bank-b` | Stop central-bank-b backend only. |
| `make deploy.up-infra` | Shared infra only (Keycloak + Postgres + Redis). |
| `make deploy.down-infra` | Stop shared infra. |
| `make deploy.up-spoke-a` / `make deploy.down-spoke-a` | Besu spoke-a only. |
| `make deploy.up-spoke-b` / `make deploy.down-spoke-b` | Besu spoke-b only. |
| `make deploy.up-besu` / `make deploy.down-besu` | Both Besu spokes. |
| `make deploy.up-backend` | Same as `deploy.up-backend-entities` (all six backends). |
| `make deploy.down-backend` | Same as `deploy.down-backend-entities`. |
| `make deploy.build-backend` | Build images for all six entity compose files. |

### Paladin setup and lifecycle

Full spoke setup (starts Besu, deploys contracts, registers nodes, starts Paladin):

```bash
make setup-spoke-a      # set up spoke A end-to-end
make setup-spoke-b      # set up spoke B end-to-end
make setup-scope-uc     # set up both spokes sequentially
```

Granular Paladin targets:

```bash
# Contract deployment
make paladin.deploy-contracts-spoke-a   # deploy IdentityRegistry + ZetoFactory on spoke A
make paladin.deploy-contracts-spoke-b

# TLS certificates
make paladin.generate-certs-spoke-a     # generate node TLS certs
make paladin.generate-certs-spoke-b

# Config rendering (requires .deployed-addrs.env)
make paladin.render-configs-spoke-a     # render config.yaml from templates
make paladin.render-configs-spoke-b

# Node registration
make paladin.register-nodes-spoke-a     # register nodes in IdentityRegistry
make paladin.register-nodes-spoke-b

# Container lifecycle
make paladin.start-spoke-a
make paladin.stop-spoke-a
make paladin.start-spoke-b
make paladin.stop-spoke-b
```

> **Note:** `setup-spoke-a` / `setup-spoke-b` expect the corresponding Besu network to not already be running — they call `deploy.up-spoke-{a,b}` themselves. If Besu is already up, run the granular targets directly.

## Manual Keycloak credentials refresh

If needed, refresh Keycloak credentials manually with:

- [keycloak/get_credentials_direct.sh](keycloak/get_credentials_direct.sh)

Examples:

```bash
./deploy/local/keycloak/get_credentials_direct.sh --entity bank-a
./deploy/local/keycloak/get_credentials_direct.sh --entity bank-b
./deploy/local/keycloak/get_credentials_direct.sh --entity central-bank-a
./deploy/local/keycloak/get_credentials_direct.sh --entity bank-c
./deploy/local/keycloak/get_credentials_direct.sh --entity bank-d
./deploy/local/keycloak/get_credentials_direct.sh --entity central-bank-b
```

## Compose variable fallbacks

Compose uses shell-style defaults (e.g. `${POSTGRES_PORT:-5432}`), so local
startup still works when some variables are not explicitly set.

## Environment variables — 007-bridge-based-cb-liquidity (Sovereign CB Liquidity)

These variables are required in each Central Bank's gateway compose file after deploying
the sovereign contracts via `make contracts.seed-sovereign-pair`.

| Variable | Required by | Description |
|---|---|---|
| `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` | api-gateway, cacti watcher | On-chain address of `LiquidityCommitRegistry` on the Hub |
| `SOVEREIGN_PAIR_AMM_MAP` | api-gateway | JSON map of pool_pair → AMM address, e.g. `{"W-BRL-ARS":"0xAMM"}` |
| `SOVEREIGN_PAIR_IDS` | api-gateway | Comma-separated list of sovereign pair IDs, e.g. `W-BRL-ARS` |
| `W_TOKEN_BRL_ADDRESS` | api-gateway | W-tCeBM_BRL contract address on Hub (output of SeedNewSovereignPair) |
| `W_TOKEN_ARS_ADDRESS` | api-gateway | W-tCeBM_ARS contract address on Hub (output of SeedNewSovereignPair) |
| `LOCAL_CB_HUB_SIGNER` | api-gateway | Ethereum address of this gateway's sovereign signer on the Hub |
| `CB_A_HUB_PRIVATE_KEY` | deploy scripts | Private key for CB-A Hub signer (seed scripts only, not runtime) |
| `CB_B_HUB_PRIVATE_KEY` | deploy scripts | Private key for CB-B Hub signer (seed scripts only, not runtime) |

**Obtaining values**: run `make contracts.seed-sovereign-pair` and read the console.log output.

**Adding a new CB**: copy `bank-a.yaml`, update `SOVEREIGN_PAIR_AMM_MAP` and `SOVEREIGN_PAIR_IDS`
with the new pair entries, and run a new `make contracts.seed-sovereign-pair` invocation with the
new CB's keys.

## Security note

Do not commit real secrets. Treat all runtime files in `backend/config/.env.*`
as local sensitive files.
