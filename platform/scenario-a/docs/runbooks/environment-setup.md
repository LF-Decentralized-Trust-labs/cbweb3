# Environment Setup Guide — CBWeb3 Platform

> **Deliverable 9** · Prerequisites and port reference

This guide covers local environment preparation before running the deployment. For the deployment procedure itself, see [`deployment-runbook.md`](deployment-runbook.md).

---

## Table of Contents

- [Required tools](#required-tools)
- [Tool installation](#tool-installation)
- [Port reference — Scenario A](#port-reference--scenario-a)
- [Port reference — Scenario B (planned)](#port-reference--scenario-b-planned)
- [Configuration files per entity](#configuration-files-per-entity)
- [Docker network](#docker-network)

---

## Required tools

| Tool | Minimum version | Purpose |
|------|----------------|---------|
| **Docker** | 24.x | Container orchestration for all services |
| **Docker Compose** | v2 (plugin) | Multi-container stack management |
| **GNU Make** | 3.81 | Build automation (`make spoke-all`) |
| **Go** | 1.22 | Backend services and Paladin scripts |
| **Node.js** | 22 LTS | Frontend applications (React/Vite) |
| **npm** | 10 | Frontend package management |
| **Foundry** (`forge`, `cast`) | nightly | Solidity contract compilation and deployment |
| **jq** | 1.6 | JSON processing in shell scripts |
| **openssl** | 3.x | PKI certificate generation (EC prime256v1) |

**Recommended Docker resources:**

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| RAM | 8 GB | 16 GB |
| CPUs | 4 | 8 |
| Disk | 20 GB free | 40 GB free |

---

## Tool installation

### macOS

```bash
# Homebrew
brew install go node jq openssl make

# Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Docker Desktop — https://www.docker.com/products/docker-desktop/
# npm comes with Node.js
```

### Linux (Ubuntu/Debian)

```bash
# Go
wget https://go.dev/dl/go1.22.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Node.js 22 via nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash
nvm install 22 && nvm use 22

# Tools
sudo apt-get install -y jq openssl make

# Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Docker Engine — https://docs.docker.com/engine/install/ubuntu/
```

### Verification

```bash
docker --version          # Docker version 24.x
docker compose version    # Docker Compose version v2.x
go version                # go1.22.x
node --version            # v22.x
npm --version             # 10.x
forge --version           # forge x.x.x
jq --version              # jq-1.6
openssl version           # OpenSSL 3.x
```

---

## Port reference — Scenario A

### Shared infrastructure

| Service | Port | Notes |
|---------|------|-------|
| **Keycloak** | 8081 | OIDC / realm management — `http://localhost:8081` |
| **PostgreSQL** | 5432 | Shared across all services |
| **Redis** | 6379 | Shared (DB 0–5 per entity) |

### API Gateways (REST)

| Entity | API Gateway | Spoke |
|--------|------------|-------|
| bank-a | **18080** | A |
| bank-b | **28080** | B |
| central-bank-a | **38080** | A |
| bank-c | **48080** | A |
| bank-d | **58080** | B |
| central-bank-b | **60080** | B |

### Internal gRPC services

| Entity | Auth gRPC | Compliance gRPC | Payment gRPC |
|--------|-----------|-----------------|--------------|
| bank-a | 19091 | 19093 | 19094 |
| bank-b | 29091 | 29093 | 29094 |
| central-bank-a | 39091 | 39093 | 39094 |
| bank-c | 49091 | 49093 | 49094 |
| bank-d | 59091 | 59093 | 59094 |
| central-bank-b | 60091 | 60093 | 60094 |

### Besu QBFT — RPC nodes

| Node | Spoke | Chain ID | RPC port |
|------|-------|---------|----------|
| central-bank-a | A | 1338 | **8645** |
| bank-a | A | 1338 | **8646** |
| bank-c | A | 1338 | **8647** |
| central-bank-b | B | 1339 | **8745** |
| bank-b | B | 1339 | **8746** |
| bank-d | B | 1339 | **8747** |

### Paladin (privacy nodes)

| Node | Spoke | API port |
|------|-------|---------|
| central-bank-a | A | 31648 |
| bank-a | A | 31658 |
| bank-c | A | 31668 |
| central-bank-b | B | 31748 |
| bank-b | B | 31758 |
| bank-d | B | 31768 |

### Frontends

| Entity | Application | Port |
|--------|------------|------|
| Bank-A | Bank Portal | **5173** |
| Bank-B | Bank Portal | **5174** |
| Bank-C | Bank Portal | **5175** |
| Bank-D | Bank Portal | **5176** |
| Central-Bank-A | Governance Portal | **5177** |
| Central-Bank-B | Governance Portal | **5178** |
| Central-Bank-A | Treasury Portal | **5179** |
| Central-Bank-B | Treasury Portal | **5180** |
| — | Dispatcher | **5181** |

---

## Port reference — Scenario B (planned)

> When implemented, Scenario B will add an intermediate hub network. The ports below are a planned reservation.

| Component | Port (planned) | Notes |
|-----------|---------------|-------|
| Besu hub (chain 1337) | 8545 | AMM/FX hub network |
| API Gateway hub | — | To be defined |
| AMM / ManualOracle | — | Deployed via existing contracts |

---

## Configuration files per entity

Each entity has separate configuration files. Copy the `.example` files before first use:

### Infrastructure (per entity)

```
scenario-a/backend/config/
├── .env.infra.bank-a.example          → .env.infra.bank-a
├── .env.infra.bank-b.example          → .env.infra.bank-b
├── .env.infra.bank-c.example          → .env.infra.bank-c
├── .env.infra.bank-d.example          → .env.infra.bank-d
├── .env.infra.central-bank-a.example  → .env.infra.central-bank-a
└── .env.infra.central-bank-b.example  → .env.infra.central-bank-b
```

### Backend services

```
scenario-a/backend/services/
├── api-gateway/.env.example  → .env
└── auth/.env.example         → .env
```

### Contracts

```
scenario-a/contracts/.env.example → contracts/.env
```

### Frontend

```
scenario-a/frontend/.env.example  → frontend/.env
scenario-a/frontend/apps/bank/.env.example → apps/bank/.env
```

> The actual `.env` files are generated automatically by Keycloak during `make deploy.up-infra` and placed in `backend/config/`. Only `contracts/.env` needs to be filled in manually before the first deployment.

---

## Docker network

All containers share the `cbweb3_network` network, created automatically on the first `make deploy.up-infra`. To create it manually:

```bash
docker network create cbweb3_network
```

To inspect:

```bash
docker network inspect cbweb3_network
```
