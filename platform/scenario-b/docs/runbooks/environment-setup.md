# Environment Setup Guide — CBWeb3 Platform · Scenario B

> **Project:** RG-T4567 · Suboperation ATN/KS-21330-RG
> **Authors:** Lucas Campelo, Samuel Venzi
> **Date:** 2026-05-29
>
> **Deliverable 9** · Prerequisites and port reference

> **Status: Work in progress.** Scenario B is implemented but not 100% complete. Port assignments and tooling requirements are stable. The MLP stack ports are optional and only active when `ENABLE_MLP=true`. See [Known limitations in deployment-runbook.md](deployment-runbook.md#known-limitations) for current gaps.

This guide covers local environment preparation before running the Scenario B deployment. For the deployment procedure itself, see [`deployment-runbook.md`](deployment-runbook.md).

---

## Table of Contents

- [Required tools](#required-tools)
- [Tool installation](#tool-installation)
- [Port reference — Scenario B](#port-reference--scenario-b)
  - [Shared infrastructure](#shared-infrastructure)
  - [API Gateways (REST)](#api-gateways-rest)
  - [Internal gRPC services](#internal-grpc-services)
  - [Besu QBFT — RPC and WebSocket nodes](#besu-qbft--rpc-and-websocket-nodes)
  - [Paladin (privacy nodes)](#paladin-privacy-nodes)
  - [Cacti relay](#cacti-relay)
- [Configuration files per entity](#configuration-files-per-entity)
- [Docker network](#docker-network)

---

## Required tools

| Tool | Minimum version | Purpose |
|------|----------------|---------|
| **Docker** | 24.x | Container orchestration for all services |
| **Docker Compose** | v2 (plugin) | Multi-container stack management |
| **GNU Make** | 3.81 | Build automation (`make scenario-b.up`) |
| **Go** | 1.26 | Backend services compilation |
| **Node.js** | 20 LTS | Cacti relay (TypeScript) |
| **npm** | 10 | Package management for Cacti relay |
| **Foundry** (`forge`, `cast`) | nightly | Solidity contract compilation and deployment |
| **k6** | 0.50 | Load and performance tests |
| **jq** | 1.6 | JSON processing in shell scripts |
| **openssl** | 3.x | PKI certificate generation (EC prime256v1) |

**Recommended Docker resources:**

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| RAM | 12 GB | 16 GB |
| CPUs | 4 | 8 |
| Disk | 25 GB free | 40 GB free |

The full Scenario B stack runs approximately 35 containers (6 entity backends × 4 services each, shared infra, 2 Besu nodes, Cacti relay, optional MLP).

---

## Tool installation

### macOS

```bash
# Homebrew
brew install go node jq openssl make

# Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup

# k6
brew install k6

# Docker Desktop — https://www.docker.com/products/docker-desktop/
# npm comes with Node.js
```

### Linux (Ubuntu/Debian)

```bash
# Go 1.26
wget https://go.dev/dl/go1.26.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.26.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Node.js 20 via nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash
nvm install 20 && nvm use 20

# Tools
sudo apt-get install -y jq openssl make

# Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup

# k6
sudo gpg -k
sudo gpg --no-default-keyring \
  --keyring /usr/share/keyrings/k6-archive-keyring.gpg \
  --keyserver hkp://keyserver.ubuntu.com:80 \
  --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] \
  https://dl.k6.io/deb stable main" \
  | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update && sudo apt-get install k6

# Docker Engine — https://docs.docker.com/engine/install/ubuntu/
```

### Verification

```bash
docker --version          # Docker version 24.x or later
docker compose version    # Docker Compose version v2.x
go version                # go1.26.x
node --version            # v20.x or later
npm --version             # 10.x
forge --version           # forge x.x.x (nightly)
k6 version                # k6 v0.50.x
jq --version              # jq-1.6
openssl version           # OpenSSL 3.x
```

---

## Port reference — Scenario B

### Shared infrastructure

| Service | Port | Notes |
|---------|------|-------|
| **Keycloak** | 8081 | OIDC / realm management — `http://localhost:8081` |
| **PostgreSQL** | 5432 | Shared across all 7 entity databases |
| **Redis** | 6379 | Shared (logical DBs 0–6, one per entity) |

### API Gateways (REST)

| Entity | API Gateway port | Spoke/Role |
|--------|-----------------|-----------|
| bank-a | **18080** | Spoke-A — commercial bank |
| bank-b | **28080** | Spoke-B — commercial bank |
| central-bank-a | **38080** | Spoke-A — hub authority (CB-A) |
| bank-c | **48080** | Spoke-A — commercial bank |
| bank-d | **58080** | Spoke-B — commercial bank |
| central-bank-b | **60080** | Spoke-B — hub authority (CB-B) |
| mlp | **68080** | Optional MLP stack (requires `ENABLE_MLP=true`) |

### Internal gRPC services

| Entity | Auth gRPC | Compliance gRPC | Payment-Orch gRPC |
|--------|-----------|-----------------|-------------------|
| bank-a | 19091 | 19093 | 19094 |
| bank-b | 29091 | 29093 | 29094 |
| central-bank-a | 39091 | 39093 | 39094 |
| bank-c | 49091 | 49093 | 49094 |
| bank-d | 59091 | 59093 | 59094 |
| central-bank-b | 60091 | 60093 | 60094 |
| mlp | 68091 | 68093 | 68094 |

### Besu QBFT — RPC and WebSocket nodes

| Node | Network | Chain ID | RPC port | WS port |
|------|---------|---------|----------|---------|
| central-bank-a | Spoke-A (= Hub in local dev) | 1338 | **8645** | **8655** |
| bank-a | Spoke-A | 1338 | **8646** | **8656** |
| bank-c | Spoke-A | 1338 | **8647** | **8657** |
| central-bank-b | Spoke-B | 1339 | **8745** | **8755** |
| bank-b | Spoke-B | 1339 | **8746** | **8756** |
| bank-d | Spoke-B | 1339 | **8747** | **8757** |

> **Note:** In local development, the Hub Besu network is co-located with Spoke-A. Both share chain ID 1338 and RPC port 8645. Hub contracts are deployed to the same node. A separate hub node (planned chain ID 1337) will be introduced in a future update.

### Paladin (privacy nodes)

Paladin nodes provide Zeto (ZKP) and Pente (bilateral context) privacy on the spokes. Privacy is **not** available on the hub in the current implementation.

| Node | Spoke | HTTP port | WS port |
|------|-------|-----------|---------|
| CB-A (central-bank-a) | A | 31648 | 31649 |
| Bank-A | A | 31658 | 31659 |
| Bank-C | A | 31668 | 31669 |
| CB-B (central-bank-b) | B | 31748 | 31749 |
| Bank-B | B | 31758 | 31759 |
| Bank-D | B | 31768 | 31769 |

### Cacti relay

| Service | Port | Protocol | Notes |
|---------|------|----------|-------|
| **Cacti relay API** | 4000 | REST (HTTP) | LiquidityCommitWatcher + HTLC event relay |

The Cacti relay watches both Spoke-A and Spoke-B for HTLC and bridge events. It connects to `GATEWAY_INTERNAL_URLS` (central-bank-a and central-bank-b API gateways) to forward settlement instructions.

---

## Configuration files per entity

Each entity has separate configuration files. Copy the `.example` files before first use:

### Infrastructure (per entity)

```
scenario-b/backend/config/
├── .env.infra.bank-a.example          → .env.infra.bank-a
├── .env.infra.bank-b.example          → .env.infra.bank-b
├── .env.infra.bank-c.example          → .env.infra.bank-c
├── .env.infra.bank-d.example          → .env.infra.bank-d
├── .env.infra.central-bank-a.example  → .env.infra.central-bank-a
├── .env.infra.central-bank-b.example  → .env.infra.central-bank-b
└── .env.infra.mlp.example             → .env.infra.mlp
```

> Fields in these files marked as `auto` are populated automatically by the Keycloak initialization script during `make scenario-b.up-infra`. Contract addresses are written by `make scenario-b.deploy-contracts`. Only `contracts/.env` needs to be filled in manually before the first deployment.

### Contracts

```
scenario-b/contracts/.env.example → contracts/.env
```

This file must be populated manually with deployer/admin private keys and central bank addresses before running `make scenario-b.deploy-contracts`. See [contract-configuration.md](contract-configuration.md) for the complete variable list.

### Cacti relay

```
scenario-b/interop/hub-and-spoke/cacti/.env
```

This file is written automatically by `make scenario-b.deploy-contracts` (address sync). Manually managed variables (`SPOKE_A_BESU_RPC`, `SPOKE_B_BESU_RPC`, etc.) should be set from the example file:

```
scenario-b/interop/hub-and-spoke/cacti/.env.example → .env
```

---

## Docker network

All Scenario B containers share the `cbweb3_network` Docker network, created automatically on the first `make scenario-b.up-infra`.

To create it manually:

```bash
docker network create cbweb3_network
```

To inspect:

```bash
docker network inspect cbweb3_network
```

The Cacti relay uses `host.docker.internal` to reach Besu RPC and WS endpoints from inside Docker. On Linux, you may need to add `--add-host=host.docker.internal:host-gateway` to the compose file or configure the network accordingly.
