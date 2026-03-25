# Local Deployment (CBWeb3 Platform — Spoke-A)

This folder contains the local runtime setup for the CBWeb3 platform. All
entities — **Bank-A**, **Bank-B** and **Central Bank** — live in a single
**spoke-a** environment, sharing one Besu network, one Keycloak instance,
one PostgreSQL instance and one Redis instance, with logical isolation per
entity.

## Architecture overview

```
spoke-a
├── Besu (chain 1338)          ← single network shared by all 3 entities
├── Keycloak                   ← 3 realms: bank-a, bank-b, central-bank
├── PostgreSQL                 ← 3 databases: cbweb3_bank_a / cbweb3_bank_b / cbweb3_central_bank
├── Redis                      ← 3 logical DBs: 0 (bank-a) / 1 (bank-b) / 2 (central-bank)
├── bank-a services            ← api-gateway :18080 | auth :19091 | compliance :19093
├── bank-b services            ← api-gateway :28080 | auth :29091 | compliance :29093
└── central-bank services      ← api-gateway :38080 | auth :39091 | compliance :39093
```

## Components

### 1) Besu network — spoke-besu-a

A single Besu QBFT network for spoke-a with **3 fixed named nodes**, one per entity:

- Directory: `spoke-besu-a/`
- Scripts: `startBesu.sh` / `stopBesu.sh` / `addNewNode.sh`
- Chain ID: `1338`
- Docker network: `spoke_a_besu_network`
- Container prefix: `cbweb3-spoke-a-besu`

| Node | Container | Host RPC | Host P2P |
|------|-----------|----------|----------|
| central-bank (bootnode) | `cbweb3-spoke-a-besu.central-bank` | `8645` | `31303` |
| bank-a | `cbweb3-spoke-a-besu.bank-a` | `8646` | `31304` |
| bank-b | `cbweb3-spoke-a-besu.bank-b` | `8647` | `31305` |

Each backend service connects to its own node's RPC endpoint via `BESU_RPC_URL` in `.env.infra.*`.
When `BLOCKCHAIN_CLIENT=besu`, the backends use the container name (Docker DNS) to reach the node on the internal `spoke_a_besu_network`.

Start/stop with `make deploy.up-spoke-a` / `make deploy.down-spoke-a`.

### 2) Keycloak

Keycloak is started by Docker Compose and initialized by
[keycloak/init.sh](keycloak/init.sh).

On startup, the script:

1. Waits for Keycloak readiness.
2. Authenticates with admin credentials.
3. Creates realm **`bank-a`** with client `bank-a-client`.
4. Creates realm **`bank-b`** with client `bank-b-client`.
5. Creates realm **`central-bank`** with client `central-bank-client`.
6. Fetches each client secret and writes runtime infra env files:
   - `backend/config/.env.infra.bank-a`
   - `backend/config/.env.infra.bank-b`
   - `backend/config/.env.infra.central-bank`

When you run `make deploy.up-infra` or `make dev.up` from the repository root,
[`make/10-deploy.mk`](../../make/10-deploy.mk) waits in two phases:

1. **HTTP readiness**: polls `http://localhost:${KEYCLOAK_PORT}/realms/master`.
2. **Init completion**: polls container logs until `KEYCLOAK_INIT_DONE` appears.

Override waits (defaults: 40×3s for HTTP, 600s for init):

- `KEYCLOAK_READY_ATTEMPTS`, `KEYCLOAK_WAIT_SLEEP_SEC`

### 3) PostgreSQL

PostgreSQL runs as a container managed by Compose with a persistent volume.
[postgres/init-multi-db.sh](postgres/init-multi-db.sh) creates three logical
databases at first start:

- `cbweb3_bank_a`
- `cbweb3_bank_b`
- `cbweb3_central_bank`
- `cbweb3_keycloak` (for Keycloak)

### 4) Redis

Redis runs as a Compose service in `cbweb3_network`. Each entity uses a
dedicated Redis logical DB for nonce isolation:

| Entity       | Redis DB |
|--------------|----------|
| bank-a       | 0        |
| bank-b       | 1        |
| central-bank | 2        |

## How components communicate

- Besu nodes communicate inside `spoke_a_besu_network` (Docker bridge created
  by `startBesu.sh`).
- Keycloak, PostgreSQL and Redis communicate on `cbweb3_network` (external
  Docker network created by `deploy.create-shared-network`).
- Backend services within each entity communicate on a dedicated internal
  network (`cbweb3_backend_bank_a`, etc.) and reach infra via `cbweb3_network`.
- Local access:
  - Keycloak: `http://localhost:${KEYCLOAK_PORT}` (default `8081`)
  - PostgreSQL: `localhost:${POSTGRES_PORT}` (default `5432`)
  - Redis: `localhost:${REDIS_PORT}` (default `6379`)
  - Besu RPC (central-bank): `http://localhost:8645`
  - Besu RPC (bank-a):       `http://localhost:8646`
  - Besu RPC (bank-b):       `http://localhost:8647`

## Configuration model

- `backend/config/.env.infra.bank-a`, `.env.infra.bank-b` and
  `.env.infra.central-bank` contain environment settings split by entity.
- These files are generated/updated from `*.example` templates with
  Keycloak-specific values and secrets.
- **`*.example` files are templates only**; runtime values are written to
  `.env.infra.*` by `keycloak/init.sh`.

### Recommended setup

1. Copy and adjust the example files:
   - `backend/config/.env.infra.bank-a.example` → `backend/config/.env.infra.bank-a`
   - `backend/config/.env.infra.bank-b.example` → `backend/config/.env.infra.bank-b`
   - `backend/config/.env.infra.central-bank.example` → `backend/config/.env.infra.central-bank`
2. Start infra (Keycloak will regenerate the files automatically):
   ```bash
   make deploy.up-infra
   ```
3. Start the Besu network:
   ```bash
   make deploy.up-spoke-a
   ```
4. Start backend services per entity (or all at once):
   ```bash
   make deploy.up-backend-entities
   ```

### Backend compose by entity

After starting shared infra, you can start backend services with one compose
file per entity:

- `backend/docker-compose-backend.bank-a.yaml`
- `backend/docker-compose-backend.bank-b.yaml`
- `backend/docker-compose-backend.central-bank.yaml`

All three backend files:

- Load entity settings from `backend/config/.env.infra.*`.
- Keep service-to-service traffic in a dedicated backend network per entity.
- Connect to shared infra through external network `cbweb3_network`.
- Use non-overlapping host ports to run all entities simultaneously.

Commands (from repository root):

- Validate compose syntax:
  ```bash
  docker compose -f backend/docker-compose-backend.bank-a.yaml config
  docker compose -f backend/docker-compose-backend.bank-b.yaml config
  docker compose -f backend/docker-compose-backend.central-bank.yaml config
  ```
- Start backend per entity:
  ```bash
  docker compose -f backend/docker-compose-backend.bank-a.yaml up -d
  docker compose -f backend/docker-compose-backend.bank-b.yaml up -d
  docker compose -f backend/docker-compose-backend.central-bank.yaml up -d
  ```
- Stop backend per entity:
  ```bash
  docker compose -f backend/docker-compose-backend.bank-a.yaml down
  docker compose -f backend/docker-compose-backend.bank-b.yaml down
  docker compose -f backend/docker-compose-backend.central-bank.yaml down
  ```

Make targets (equivalent shortcuts):

- `make deploy.validate-backend-entities`
- `make deploy.up-backend-bank-a`
- `make deploy.up-backend-bank-b`
- `make deploy.up-backend-central-bank`
- `make deploy.up-backend-entities`
- `make deploy.down-backend-bank-a`
- `make deploy.down-backend-bank-b`
- `make deploy.down-backend-central-bank`
- `make deploy.down-backend-entities`

Default host ports:

- Bank-A:       API `18080`, Auth gRPC `19091`, Compliance gRPC `19093`
- Bank-B:       API `28080`, Auth gRPC `29091`, Compliance gRPC `29093`
- Central Bank: API `38080`, Auth gRPC `39091`, Compliance gRPC `39093`

## Start and stop

From repository root:

- `make dev.up` — full environment (PKI + Besu + infra + contracts + all backends).
- `make dev.down` — stop full environment in reverse order.
- `make dev.up-bank-a` — PKI + infra + Besu + bank-a backend only.
- `make dev.up-bank-b` — PKI + infra + Besu + bank-b backend only.
- `make dev.up-central-bank` — PKI + infra + Besu + central-bank backend only.
- `make deploy.up-infra` — shared infra only (Keycloak + Postgres + Redis).
- `make deploy.up-spoke-a` — Besu spoke-a only.
- `make deploy.down-infra` — stop shared infra.
- `make deploy.down-spoke-a` — stop Besu spoke-a.

## Manual Keycloak credentials refresh

If needed, refresh Keycloak credentials manually with:

- [keycloak/get_credentials_direct.sh](keycloak/get_credentials_direct.sh)

Examples:

```bash
./deploy/local/keycloak/get_credentials_direct.sh --entity bank-a
./deploy/local/keycloak/get_credentials_direct.sh --entity bank-b
./deploy/local/keycloak/get_credentials_direct.sh --entity central-bank
```

## Compose variable fallbacks

Compose uses shell-style defaults (e.g. `${POSTGRES_PORT:-5432}`), so local
startup still works when some variables are not explicitly set.

## Security note

Do not commit real secrets. Treat all runtime files in `backend/config/.env.*`
as local sensitive files.
