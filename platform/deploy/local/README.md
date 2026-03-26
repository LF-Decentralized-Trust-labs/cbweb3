# Local Deployment (CBWeb3 Platform)

This folder contains the local runtime setup for the CBWeb3 platform, focused on five main infrastructure components:

- Besu private networks (hub, spoke A, spoke B)
- Keycloak (identity and access management)
- PostgreSQL (relational storage for local services)
- Redis (cache and message-oriented local support)
- Paladin nodes (privacy-preserving smart contract execution, spoke A and spoke B)

It follows the same modular architecture described in the project root documentation, but tailored for local development and testing.

## Components

### 1) Besu networks

There are three independent Besu environments:

- `hub-besu`
- `spoke-besu-a`
- `spoke-besu-b`

Each environment has its own scripts and artifacts:

- `startBesu.sh` / `stopBesu.sh`
- `config/`, `genesis/`, `nodes/`
- optional node expansion with `addNewNode.sh`

These networks simulate distinct ledgers for domestic and cross-border scenarios.

Each network now uses:

- dedicated Docker network name,
- dedicated container prefix,
- dedicated host RPC/P2P port range.

This avoids collisions when all three Besu stacks run at the same time.

### 2) Keycloak

Keycloak is started by Docker Compose and initialized by [keycloak/init.sh](keycloak/init.sh).

On startup, the script:

1. waits for Keycloak readiness,
2. authenticates with admin credentials,
3. ensures realm `cbweb3-spoke-a` with client `cbweb3-spoke-a-client`,
4. ensures realm `cbweb3-spoke-b` with client `cbweb3-spoke-b-client`,
5. fetches both client secrets,
6. recreates runtime infra env files under `backend/config/` from `.example` templates:
   - `.env.infra.spoke-a`
   - `.env.infra.spoke-b`
   - `.env.infra.hub`

This keeps Keycloak settings and client credentials synchronized in the domain infra files.

### 3) PostgreSQL

PostgreSQL runs as a container managed by Compose, with:

- configurable image tag,
- configurable host port,
- configurable user/password,
- persistent volume (`postgres_data`).

It initializes multiple logical databases in the same instance via [postgres/init-multi-db.sh](postgres/init-multi-db.sh):

- `cbweb3_spoke_a`
- `cbweb3_spoke_b`
- `cbweb3_hub`

This supports the MVP split for spoke A, spoke B and international hub while keeping a single local Postgres container.

### 4) Redis

Redis runs as a Compose service in the same `cbweb3_network`, with configurable image tag and host port.

Default local access:

- `localhost:${REDIS_PORT}` (default `6379`)

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

1. **Deploy contracts** — deploys a Paladin `IdentityRegistry` and a `ZetoFactory` (with `Zeto_AnonNullifier` implementation) to the spoke's Besu network. Contract addresses are written to `paladin/spoke-{a,b}/.deployed-addrs.env`.
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
      deploy-zeto-factory/   # Deploys ZetoFactory + Zeto_AnonNullifier impl
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

- Besu nodes communicate inside their own Docker bridge network (one per stack, created by each `startBesu.sh`).
- Keycloak, PostgreSQL and Redis communicate on `cbweb3_network` (external Docker network used by Compose).
- Local applications can access:
  - Keycloak on `localhost:${KEYCLOAK_PORT}` (default `8081`)
  - PostgreSQL on `localhost:${POSTGRES_PORT}` (default `5432`)
  - Redis on `localhost:${REDIS_PORT}` (default `6379`)

## Configuration model

Important separation:

- `backend/config/.env.infra.spoke-a`, `.env.infra.spoke-b`, and `.env.infra.hub` contain environment settings split by domain.
- these files are generated/updated from `*.example` templates with Keycloak-specific values and secrets.
- Compose still supports standard environment variables (with fallbacks), and you should provide one domain env file from `backend/config`.

In other words:

- **Infra/domain configuration goes to `backend/config/.env.infra.*`.**
- **`*.example` files remain templates only; runtime values are written to `.env.infra.*`.**

Use these templates:

- [../backend/config/.env.infra.spoke-a.example](../backend/config/.env.infra.spoke-a.example)
- [../backend/config/.env.infra.spoke-b.example](../backend/config/.env.infra.spoke-b.example)
- [../backend/config/.env.infra.hub.example](../backend/config/.env.infra.hub.example)

### Recommended setup

1. Copy and adjust one env file per domain:
   - `backend/config/.env.infra.spoke-a.example` -> `backend/config/.env.infra.spoke-a`
   - `backend/config/.env.infra.spoke-b.example` -> `backend/config/.env.infra.spoke-b`
   - `backend/config/.env.infra.hub.example` -> `backend/config/.env.infra.hub`
2. Start local stack with one selected domain env file, for example:
   - `docker compose --env-file backend/config/.env.infra.spoke-a -f deploy/local/compose.yml up -d`
3. Keycloak will regenerate `.env.infra.spoke-a`, `.env.infra.spoke-b`, and `.env.infra.hub` in `backend/config/`.
4. Use the domain-specific files in backend services (`spoke-a`, `spoke-b`, `hub`).

### Backend compose by domain (hub/spokes)

After starting shared infra (`keycloak`, `postgres`, `redis`) with `deploy/local/compose.yml`,
you can start backend services with one compose file per domain:

- `backend/docker-compose-backend.spoke-a.yaml`
- `backend/docker-compose-backend.spoke-b.yaml`
- `backend/docker-compose-backend.hub.yaml`

All three backend files:

- load domain settings from `backend/config/.env.infra.*`,
- keep service-to-service traffic in a dedicated backend network per domain,
- connect to shared infra through external network `cbweb3_network`,
- use non-overlapping host ports to run all domains at once.

Commands (from repository root):

- Validate compose syntax:
  - `docker compose -f backend/docker-compose-backend.spoke-a.yaml config`
  - `docker compose -f backend/docker-compose-backend.spoke-b.yaml config`
  - `docker compose -f backend/docker-compose-backend.hub.yaml config`
- Start backend per domain:
  - `docker compose -f backend/docker-compose-backend.spoke-a.yaml up -d`
  - `docker compose -f backend/docker-compose-backend.spoke-b.yaml up -d`
  - `docker compose -f backend/docker-compose-backend.hub.yaml up -d`
- Stop backend per domain:
  - `docker compose -f backend/docker-compose-backend.spoke-a.yaml down`
  - `docker compose -f backend/docker-compose-backend.spoke-b.yaml down`
  - `docker compose -f backend/docker-compose-backend.hub.yaml down`

Make targets (equivalent shortcuts):

- `make validate-backend-domains`
- `make up-backend-spoke-a`
- `make up-backend-spoke-b`
- `make up-backend-hub`
- `make up-backend-domains`
- `make down-backend-spoke-a`
- `make down-backend-spoke-b`
- `make down-backend-hub`
- `make down-backend-domains`

Default host ports:

- Spoke A: API `18080`, Auth gRPC `19091`, Compliance gRPC `19093`
- Spoke B: API `28080`, Auth gRPC `29091`, Compliance gRPC `29093`
- Hub: API `38080`, Auth gRPC `39091`, Compliance gRPC `39093`

## Start and stop

From repository root:

- `make up-besu` starts only Besu stacks (hub + spoke A + spoke B).

- `make up-infra` starts Compose services (Keycloak + PostgreSQL + Redis).

- `make up` starts full local stack (Besu + infra).

- `make down-infra` stops Compose services.

- `make down-besu` stops only Besu stacks.

- `make down` stops full local stack.

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

- `./deploy/local/keycloak/get_credentials_direct.sh --spoke a`
- `./deploy/local/keycloak/get_credentials_direct.sh --spoke b`

The script reads domain infra settings from `backend/config/.env.infra.spoke-a|spoke-b` and recreates files in `backend/config/`.

## Compose variable fallbacks

Compose uses shell-style defaults (e.g. `${POSTGRES_PORT:-5432}`), so local startup still works when some variables are not explicitly set.

Compose currently uses Keycloak, PostgreSQL and Redis variables in local mode.

## Security note

Do not commit real secrets. Treat all runtime files in `backend/config/.env.*` as local sensitive files.
