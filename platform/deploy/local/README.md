# Local Deployment (CBWeb3 Platform)

This folder contains the local runtime setup for the CBWeb3 platform, focused on four main infrastructure components:

- Besu private networks (hub, spoke A, spoke B)
- Keycloak (identity and access management)
- PostgreSQL (relational storage for local services)
- Redis (cache and message-oriented local support)

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
6. recreates runtime credential files under `backend/config/`:
   - `.env.keycloak.spoke-a`
   - `.env.keycloak.spoke-b`
   - compatibility aliases:
     - `.env.keycloack.spoke-a`
     - `.env.keycloack.spoke-b`

This keeps Keycloak client credentials always synchronized with the running container.

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
- `backend/config/.env.keycloak.spoke-a` and `backend/config/.env.keycloak.spoke-b` are generated at runtime by Keycloak initialization.
- Compose still supports standard environment variables (with fallbacks), and you should provide one domain env file from `backend/config`.

In other words:

- **Infra/domain configuration goes to `backend/config/.env.infra.*`.**
- **Runtime Keycloak credentials stay in `backend/config` as generated files.**

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
3. Keycloak will regenerate runtime files in `backend/config/`.
4. Use the domain-specific files in backend services (`spoke-a`, `spoke-b`, `hub`).

## Start and stop

From repository root:

- `make up-besu` starts only Besu stacks (hub + spoke A + spoke B).

- `make up-infra` starts Compose services (Keycloak + PostgreSQL + Redis).

- `make up` starts full local stack (Besu + infra).

- `make down-infra` stops Compose services.

- `make down-besu` stops only Besu stacks.

- `make down` stops full local stack.

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
