# Configuration Reference — CBWeb3 Platform · Scenario B

> **Project:** RG-T4567 · Suboperation ATN/KS-21330-RG
> **Authors:** Lucas Campelo, Samuel Venzi
> **Date:** 2026-05-29
>
> **Deliverable 10** · System parameterization — environment variables by component

> **Status: Work in progress.** Scenario B is implemented but not 100% complete. Hub/AMM-specific variables and MLP variables are documented here. Some contract address variables are populated automatically by `make scenario-b.deploy-contracts` and may not match values shown in examples until deployment completes.

This document is the complete reference for all Scenario B environment variables, grouped by component. For the deployment guide, see [`deployment-runbook.md`](deployment-runbook.md). For contract parameters and on-chain roles, see [`contract-configuration.md`](contract-configuration.md).

---

## Table of Contents

- [Conventions](#conventions)
- [Shared infrastructure](#shared-infrastructure)
- [Per-entity common variables](#per-entity-common-variables)
- [Hub / AMM-specific variables](#hub--amm-specific-variables)
- [MLP-specific variables](#mlp-specific-variables)
- [Cacti relay variables](#cacti-relay-variables)
- [Service: api-gateway](#service-api-gateway)
- [Service: auth](#service-auth)
- [Paladin (config.yaml per node)](#paladin-configyaml-per-node)

---

## Conventions

| Symbol | Meaning |
|--------|---------|
| **required** | Must be set before starting services |
| `default: X` | Default value applied if not set |
| `auto` | Populated automatically (Keycloak init or `make scenario-b.deploy-contracts`) — do not edit manually |
| `secret` | Never version-control; use a secret manager in production |

---

## Shared infrastructure

Files: `scenario-b/deploy/local/compose.yml` and per-entity `.env.infra.*` files.

### Keycloak

| Variable | Default | Type | Description |
|----------|---------|------|-------------|
| `KEYCLOAK_CONTAINER_NAME` | `cbweb3-keycloak` | string | Container name |
| `KEYCLOAK_PORT` | `8081` | int | Port exposed on the host |
| `KC_BOOTSTRAP_ADMIN_USERNAME` | `admin` | string | Initial admin username |
| `KC_BOOTSTRAP_ADMIN_PASSWORD` | `admin` | string secret | Initial admin password |
| `KEYCLOAK_ENV_OUTPUT_DIR` | `/opt/keycloak/host/backend/config` | path | Directory where `init.sh` writes generated `.env` files |
| `KEYCLOAK_READY_ATTEMPTS` | `40` | int | Healthcheck retry attempts before failing |
| `KEYCLOAK_WAIT_SLEEP_SEC` | `3` | int | Interval in seconds between healthcheck attempts |

Scenario B has 7 realms (vs. 6 in Scenario A): `bank-a`, `bank-b`, `bank-c`, `bank-d`, `central-bank-a`, `central-bank-b`, `mlp`.

### PostgreSQL

| Variable | Default | Type | Description |
|----------|---------|------|-------------|
| `POSTGRES_CONTAINER_NAME` | `cbweb3-postgres` | string | Container name |
| `POSTGRES_IMAGE_TAG` | `17-alpine` | string | Image version |
| `POSTGRES_PORT` | `5432` | int | Port exposed on the host |
| `POSTGRES_USER` | `postgres` | string | Main user |
| `POSTGRES_PASSWORD` | `postgres` | string secret | Main password |
| `POSTGRES_DB_BANK_A` | `cbweb3_bank_a` | string | bank-a database |
| `POSTGRES_DB_BANK_B` | `cbweb3_bank_b` | string | bank-b database |
| `POSTGRES_DB_BANK_C` | `cbweb3_bank_c` | string | bank-c database |
| `POSTGRES_DB_BANK_D` | `cbweb3_bank_d` | string | bank-d database |
| `POSTGRES_DB_CENTRAL_BANK_A` | `cbweb3_central_bank_a` | string | central-bank-a database |
| `POSTGRES_DB_CENTRAL_BANK_B` | `cbweb3_central_bank_b` | string | central-bank-b database |
| `POSTGRES_DB_MLP` | `cbweb3_mlp` | string | MLP database (optional) |

### Redis

| Variable | Default | Type | Description |
|----------|---------|------|-------------|
| `REDIS_CONTAINER_NAME` | `cbweb3-redis` | string | Container name |
| `REDIS_IMAGE_TAG` | `7-alpine` | string | Image version |
| `REDIS_PORT` | `6379` | int | Port exposed on the host |

Redis logical DB assignment:

| Entity | Redis DB |
|--------|---------|
| bank-a | 0 |
| bank-b | 1 |
| central-bank-a | 2 |
| bank-c | 3 |
| bank-d | 4 |
| central-bank-b | 5 |
| mlp | 6 |

---

## Per-entity common variables

File: `scenario-b/backend/config/.env.infra.<entity>`
Template: `scenario-b/backend/config/.env.infra.<entity>.example`

These variables are common across all 7 entities. Examples below use `bank-a` as the reference entity.

### Keycloak (per entity)

| Variable | Example (bank-a) | Type | Description |
|----------|-----------------|------|-------------|
| `KEYCLOAK_URL` | `http://keycloak:8081` | URL **required** | Keycloak base URL (internal Docker hostname) |
| `KC_REALM` | `bank-a` | string **required** | Dedicated realm for this entity |
| `KC_CLIENT_ID` | `bank-a-client` | string **required** | OIDC client ID |
| `KC_CLIENT_SECRET` | `<generated>` | string secret `auto` | Client secret generated and written by Keycloak init |

### PostgreSQL (per entity)

| Variable | Example (bank-a) | Type | Description |
|----------|-----------------|------|-------------|
| `POSTGRES_DSN` | `postgres://postgres:postgres@postgres:5432/cbweb3_bank_a` | DSN **required** | Full PostgreSQL connection string |

### Redis (per entity)

| Variable | Example (bank-a) | Type | Description |
|----------|-----------------|------|-------------|
| `REDIS_URL` | `redis://redis:6379/0` | URL **required** | Redis connection URL including logical DB index |

### Blockchain (per entity)

| Variable | Example (bank-a) | Type | Description |
|----------|-----------------|------|-------------|
| `BESU_RPC_URL` | `http://cbweb3-spoke-a-besu:8545` | URL **required** | Entity's Besu node RPC URL (internal Docker hostname) |
| `BESU_CHAIN_ID` | `1338` | int **required** | Chain ID: `1338` for Spoke-A/Hub, `1339` for Spoke-B |

### Paladin (per entity)

| Variable | Example (bank-a) | Type | Description |
|----------|-----------------|------|-------------|
| `PALADIN_RPC_URL` | `http://paladin-bank-a:9081` | URL | Paladin node HTTP RPC URL |
| `PALADIN_GRPC_URL` | `paladin-bank-a:9082` | addr | Paladin node gRPC address |

### PKI (per entity)

| Variable | Example (bank-a) | Type | Description |
|----------|-----------------|------|-------------|
| `CA_CERT_FILE` | `/workspace/backend/config/pki/bank-a-ca.crt` | path **required** | Entity CA certificate path |
| `CA_KEY_FILE` | `/workspace/backend/config/pki/bank-a-ca.key` | path secret **required** | CA private key path |

> In production: replace with Docker Secrets references (`/run/secrets/ca.crt`, `/run/secrets/ca.key`).

### Internal gRPC addresses (per entity)

| Variable | Example (bank-a) | Type | Description |
|----------|-----------------|------|-------------|
| `AUTH_GRPC_ADDR` | `auth-bank-a:19091` | addr **required** | Auth service gRPC address |
| `COMPLIANCE_GRPC_ADDR` | `compliance-bank-a:19093` | addr **required** | Compliance service gRPC address |
| `PAYMENT_ORCH_GRPC_ADDR` | `payment-orchestrator-bank-a:19094` | addr **required** | Payment orchestrator gRPC address |

### Signer

| Variable | Example | Type | Description |
|----------|---------|------|-------------|
| `SIGNER_PRIVATE_KEY` | `<hex>` | hex secret **required** | Entity's Ethereum private key used for on-chain transactions |

### Relay authentication

| Variable | Default | Type | Description |
|----------|---------|------|-------------|
| `INTERNAL_RELAY_AUTH_SECRET` | `cbweb3-relay-shared-secret` | string secret | Shared secret for the `X-Relay-Auth` header (Cacti relay → payment-orchestrator) |

---

## Hub / AMM-specific variables

These variables are additional to the per-entity common vars above and are only present in entities that interact with the hub (primarily central-bank-a, central-bank-b, and commercial banks executing swaps).

File: `scenario-b/backend/config/.env.infra.central-bank-a` (and equivalents)

| Variable | Example | Type | Description |
|----------|---------|------|-------------|
| `HUB_BESU_RPC_URL` | `http://cbweb3-spoke-a-besu:8545` | URL **required** | Hub Besu RPC URL. In local dev, same as Spoke-A (`http://cbweb3-spoke-a-besu:8545`) |
| `HUB_CHAIN_ID` | `1338` | int **required** | Hub chain ID. In local dev, same as Spoke-A (`1338`) |
| `AMM_CONTRACT_ADDRESS` | `<deployed>` | address `auto` | AutomatedMarketMaker contract address on the hub |
| `HUB_TOKEN_A_ADDRESS` | `<deployed>` | address `auto` | tCeBM-BRL token address on the hub |
| `HUB_TOKEN_B_ADDRESS` | `<deployed>` | address `auto` | tCeBM-EUR token address on the hub |
| `PAIR_REGISTRY_ADDRESS` | `<deployed>` | address `auto` | PairRegistry contract address on the hub |
| `CURRENCY_REGISTRY_ADDRESS` | `<deployed>` | address `auto` | CurrencyRegistry contract address on the hub |
| `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` | `<deployed>` | address `auto` | LiquidityCommitRegistry contract address on the hub |
| `HUB_IDENTITY_REGISTRY_ADDRESS` | `<deployed>` | address `auto` | Hub IdentityRegistry contract address |
| `FX_AGREEMENT_HUB_ADDRESS` | `<deployed>` | address `auto` | FXAgreement contract address on the hub |
| `SPOKE_BRIDGE_ADDRESS` | `<deployed>` | address `auto` | SpokeBridge contract address (per spoke) |

All addresses marked `auto` are written by `make scenario-b.deploy-contracts` → `contracts.sync-addresses`. Do not set them manually.

---

## MLP-specific variables

File: `scenario-b/backend/config/.env.infra.mlp`
Template: `scenario-b/backend/config/.env.infra.mlp.example`

The MLP (Multilateral Liquidity Provider) stack uses all per-entity common variables above, plus the hub/AMM-specific variables, with the following MLP-specific values:

| Variable | Value | Type | Description |
|----------|-------|------|-------------|
| `KC_REALM` | `mlp` | string | MLP Keycloak realm |
| `KC_CLIENT_ID` | `mlp-client` | string | MLP OIDC client ID |
| `SIGNER_PRIVATE_KEY` | `f8f8a2f43c8376ccb0871305060d7b27b0554d2cc72bccf41b2705608452f315` | hex secret | MLP signer key (local dev only — replace in production) |
| `GOVERNANCE_USER_ID` | `service-account-mlp-client` | string | Governance user ID for service account operations |

The MLP uses Redis logical DB 6 (`REDIS_URL=redis://redis:6379/6`) and PostgreSQL database `cbweb3_mlp`.

Additionally, for MLP to deposit both sides of a pair, it requires the `LIQUIDITY_PROVIDER` role in the hub `IdentityRegistry`. This role is granted during `make scenario-b.deploy-contracts`.

For dual-sided deposits into multiple pairs, the `SOVEREIGN_PAIR_AMM_MAP` variable maps pair identifiers to AMM contract addresses:

| Variable | Example | Type | Description |
|----------|---------|------|-------------|
| `SOVEREIGN_PAIR_AMM_MAP` | `{"W-BRL-ARS":"0x..."}` | JSON object | Maps pair ID to AMM address for MLP dual-sided liquidity |

---

## Cacti relay variables

File: `scenario-b/interop/hub-and-spoke/cacti/.env`

The Cacti relay (`cbweb3-cacti-relay`) uses `host.docker.internal` to reach Besu nodes from inside Docker. On Linux, ensure `host.docker.internal` resolves correctly (add `--add-host=host.docker.internal:host-gateway`).

| Variable | Example | Type | Description |
|----------|---------|------|-------------|
| `SPOKE_A_BESU_RPC` | `http://host.docker.internal:8645` | URL **required** | Spoke-A Besu HTTP RPC endpoint (also serves as Hub in local dev) |
| `SPOKE_A_BESU_WS` | `ws://host.docker.internal:8655` | URL **required** | Spoke-A Besu WebSocket endpoint |
| `SPOKE_B_BESU_RPC` | `http://host.docker.internal:8745` | URL **required** | Spoke-B Besu HTTP RPC endpoint |
| `SPOKE_B_BESU_WS` | `ws://host.docker.internal:8755` | URL **required** | Spoke-B Besu WebSocket endpoint |
| `HUB_BESU_RPC` | `http://host.docker.internal:8645` | URL **required** | Hub Besu HTTP RPC endpoint (same as Spoke-A in local dev) |
| `HUB_BESU_WS` | `ws://host.docker.internal:8655` | URL **required** | Hub Besu WebSocket endpoint (same as Spoke-A in local dev) |
| `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` | `<from contracts.sync-addresses>` | address `auto` | LiquidityCommitRegistry address on the hub |
| `GATEWAY_INTERNAL_URLS` | `http://host.docker.internal:38080,http://host.docker.internal:60080` | list **required** | Comma-separated API gateway URLs for CB-A and CB-B (used for relay-to-gateway forwarding) |
| `INTERNAL_RELAY_AUTH_SECRET` | `cbweb3-relay-shared-secret` | string secret **required** | Shared secret verified by payment-orchestrators receiving relay calls |
| `LCR_WATCHER_START_BLOCK` | `0` | int | Block number from which the LiquidityCommitWatcher starts scanning events. Set to `0` to scan from genesis. |
| `CACTI_API_PORT` | `4000` | int | REST API port exposed by the Cacti relay |

---

## Service: `api-gateway`

File: `scenario-b/backend/services/api-gateway/.env`
Template: `scenario-b/backend/services/api-gateway/.env.example`

| Variable | Default | Type | Description |
|----------|---------|------|-------------|
| `APP_PORT` | `8080` | int | Gateway HTTP port (internal; mapped to entity port on host) |
| `REQUEST_TIMEOUT_SEC` | `5` | int | Global request timeout in seconds |
| `AUTH_GRPC_ADDR` | `localhost:9091` | addr | Auth service gRPC address |
| `COMPLIANCE_GRPC_ADDR` | `localhost:9093` | addr | Compliance service gRPC address |
| `PAYMENT_ORCH_GRPC_ADDR` | `localhost:9094` | addr | Payment orchestrator gRPC address |
| `COOKIE_SECURE` | `false` | bool | `Secure` flag on cookies (set to `true` for HTTPS/production) |

---

## Service: `auth`

File: `scenario-b/backend/services/auth/.env`

### Identity provider

| Variable | Default | Type | Description |
|----------|---------|------|-------------|
| `IDENTITY_PROVIDER` | `local` | enum | Provider: `local` or `keycloak` |
| `IDENTITY_GRPC_PORT` | `9091` | int | Auth service gRPC listen port |
| `IDENTITY_HOST_URL` | `http://localhost:8081` | URL | Keycloak base URL |
| `IDENTITY_JWT_SECRET` | `local-identity-secret` | string secret | JWT secret (local mode only) |
| `IDENTITY_ACCESS_TOKEN_TTL_SEC` | `3600` | int | Access token TTL in seconds |
| `IDENTITY_REQUEST_TIMEOUT_SEC` | `5` | int | Request timeout in seconds |

### Internal JWT (service-to-service)

| Variable | Default | Type | Description |
|----------|---------|------|-------------|
| `INTERNAL_JWT_PROVIDER` | `local` | enum | Internal JWT provider: `local` or `keycloak` |
| `INTERNAL_JWT_SECRET` | `identity-internal-secret` | string secret | Internal JWT secret |
| `INTERNAL_JWT_ISSUER` | `identity-internal` | string | Internal JWT issuer |
| `INTERNAL_JWT_AUDIENCE` | `cbweb3-internal` | string | Expected audience claim |
| `INTERNAL_JWT_TTL_SEC` | `3600` | int | Internal token TTL in seconds |

### Internal JWT via Keycloak (`INTERNAL_JWT_PROVIDER=keycloak`)

| Variable | Type | Description |
|----------|------|-------------|
| `INTERNAL_KEYCLOAK_TOKEN_URL` | URL **required** | Realm token endpoint (`http://keycloak:8081/realms/<realm>/protocol/openid-connect/token`) |
| `INTERNAL_KEYCLOAK_CLIENT_ID` | string **required** | Service account client ID |
| `INTERNAL_KEYCLOAK_CLIENT_SECRET` | string secret **required** | Service account client secret |
| `INTERNAL_KEYCLOAK_TIMEOUT_SEC` | `5` | Keycloak request timeout in seconds |

---

## Paladin (`config.yaml` per node)

File: `scenario-b/deploy/local/paladin/spoke-{a,b}/config/<entity>/config.yaml`

Paladin provides Zeto (ZKP) and Pente (bilateral context) privacy on the spokes. Hub contracts do not use Paladin in the current implementation.

### Identity and logging

| Field | Example | Description |
|-------|---------|-------------|
| `nodeName` | `spoke-a-bank-a` | Unique Paladin node identifier |
| `log.level` | `debug` | Log level: `debug`, `info`, `warn`, `error` |

### Local database

| Field | Example | Description |
|-------|---------|-------------|
| `db.sqlite.dsn` | `/data/paladin.db` | SQLite database path |
| `db.sqlite.autoMigrate` | `true` | Run migrations automatically on startup |

### RPC server

| Field | Example | Description |
|-------|---------|-------------|
| `rpcServer.http.port` | `9081` | Paladin RPC HTTP port |
| `rpcServer.ws.port` | `9082` | Paladin RPC WebSocket port |

### Blockchain

| Field | Example | Description |
|-------|---------|-------------|
| `blockchain.http.url` | `http://host.docker.internal:8646` | Besu node HTTP URL for this entity |
| `blockchain.ws.url` | `ws://host.docker.internal:8656` | Besu node WebSocket URL for this entity |

### Zeto domain (ZKP)

| Field | Description |
|-------|-------------|
| `domains.zeto.registryAddress` | Deployed ZetoFactory address |
| `domains.zeto.allowSigning` | Enable signing by fixed identity |
| `domains.zeto.fixedSigningIdentity` | Identity used for signing (`funded_operator`) |
| `domains.zeto.config.snarkProver.circuitsDir` | ZKP circuits directory |

### Pente domain (bilateral context / FX)

| Field | Description |
|-------|-------------|
| `domains.pente.registryAddress` | Deployed PenteFactory address |
| `domains.pente.config.enabled` | Enable Pente domain |
| `domains.pente.config.privateMTX` | Use multilateral private transactions |

### EVM registry

| Field | Description |
|-------|-------------|
| `registries.evm-registry.config.contractAddress` | On-chain IdentityRegistry address (spoke) |
