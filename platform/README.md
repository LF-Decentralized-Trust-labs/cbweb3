# CBWeb3 Platform

[![Backend CI — Scenario A](https://github.com/LACNetNetworks/cbweb3-platform/actions/workflows/backend-scenario-a.yml/badge.svg?branch=develop)](https://github.com/LACNetNetworks/cbweb3-platform/actions/workflows/backend-scenario-a.yml?query=branch%3Adevelop)
[![Backend CI — Scenario B](https://github.com/LACNetNetworks/cbweb3-platform/actions/workflows/backend-scenario-b.yml/badge.svg?branch=develop)](https://github.com/LACNetNetworks/cbweb3-platform/actions/workflows/backend-scenario-b.yml?query=branch%3Adevelop)
[![Contracts CI — Scenario A](https://github.com/LACNetNetworks/cbweb3-platform/actions/workflows/contracts-scenario-a.yml/badge.svg?branch=develop)](https://github.com/LACNetNetworks/cbweb3-platform/actions/workflows/contracts-scenario-a.yml?query=branch%3Adevelop)
[![Contracts CI — Scenario B](https://github.com/LACNetNetworks/cbweb3-platform/actions/workflows/contracts-scenario-b.yml/badge.svg?branch=develop)](https://github.com/LACNetNetworks/cbweb3-platform/actions/workflows/contracts-scenario-b.yml?query=branch%3Adevelop)
[![License Headers](https://github.com/LACNetNetworks/cbweb3-platform/actions/workflows/license-headers.yml/badge.svg?branch=develop)](https://github.com/LACNetNetworks/cbweb3-platform/actions/workflows/license-headers.yml?query=branch%3Adevelop)

A research platform implementing a **hub-and-spoke wholesale CBDC ecosystem** for cross-network interoperability. The system models central banks and commercial banks operating across independent blockchain networks, with privacy-preserving payments and cross-network atomic settlement.

---

## Table of Contents

- [Purpose](#purpose)
- [Scenarios](#scenarios)
- [Architecture Overview](#architecture-overview)
- [Repository Structure](#repository-structure)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)

---

## Purpose

**CBWeb3** explores how Central Bank Digital Currencies (CBDCs) can interoperate across sovereign networks without requiring a shared ledger. Each scenario represents a distinct interoperability model, with progressive complexity in the FX and liquidity dimensions.

The platform is built to be scenario-scoped: each scenario lives in its own directory with independent infrastructure, smart contracts, services, and deployment tooling.

---

## Scenarios

### Scenario A — Enhanced Correspondent Banking

> **Status:** Fully implemented and runnable.

**Directory:** [`scenario-a/`](scenario-a/README.md)

A correspondent banking model where two independent blockchain networks (**Spoke-A** and **Spoke-B**) settle cross-network transactions via dual-layer HTLC. An initiating bank locks funds on one spoke while a responding bank locks on the other; a relay service bridges the HTLC secret to achieve atomic settlement without a shared ledger or trusted intermediary.

**Key characteristics:**
- Two Besu QBFT networks (chain 1338 and 1339), each with its own central bank and commercial banks
- Privacy-preserving transfers via Paladin/Zeto (zero-knowledge proofs)
- Cross-spoke atomic swaps with automatic relay settlement
- On-chain identity registry, Onboarding/AML compliance, and Keycloak OIDC

**Quick start:** `cd scenario-a && make spoke-all`

---

### Scenario B — International Hub with FX Liquidity Pool

> **Status:** Fully implemented and runnable.

**Directory:** [`scenario-b/`](scenario-b/README.md)

A dedicated international hub network (chain 1337) operating an AMM-based liquidity pool for foreign exchange. Two sovereign spokes, each run by a central bank and a commercial bank, settle cross-currency transactions through the hub, which provides continuous FX pricing and pooled liquidity.

**Key characteristics:**
- Hub network acting as an FX intermediary between spokes
- Automated Market Maker (constant-product) for continuous FX pricing
- Manual oracle for price feeds
- Lock-and-mint / burn-and-unlock bridge between each spoke and the hub, via `SpokeBridge`
- Governance-controlled circuit breaker gating the pool (1-of-N pause, 2-of-N resume)
- Cacti-based relay observing events across the hub and both spokes
- Declarative provisioning toolkit (`scenario-b/toolkit`) driving hub founding, spoke founding, and commercial-bank joins from a manifest

Scenario B carries its own contract tree under [`scenario-b/contracts/`](scenario-b/contracts/), independent of Scenario A's. Alongside `AutomatedMarketMaker.sol`, `ManualOracle.sol` and `FXAgreement.sol` it adds the hub registries — `PairRegistry.sol`, `LiquidityCommitRegistry.sol` and `CurrencyRegistry.sol` — which have no Scenario A counterpart. The two trees are deliberately not shared: scenario isolation is a constitutional rule of this repository.

**Quick start:** `cd scenario-b && make scenario-b.up`

---

## Architecture Overview

The platform is organized in layers common to all scenarios:

| Layer | Components |
|-------|------------|
| **Frontend** | Bank Portal, Governance Portal, Supervisor, Treasury, NOC |
| **API** | REST API Gateway (per entity) |
| **Microservices** | Auth (gRPC), Compliance (gRPC), Payment Orchestrator (gRPC) |
| **Interoperability** | HTLC relay, SpokeBridge, AMM, FX Oracle |
| **Privacy** | Paladin Core, Zeto Domain (ZKP tokens), Noto Domain |
| **Blockchain** | Besu QBFT networks (per spoke) |

Each entity (central bank or commercial bank) runs its own isolated instance of every service layer, communicating intra-entity via gRPC and cross-entity via on-chain contracts and the relay.

For each scenario's full system topology, component-level documentation, and flow diagrams, see [`scenario-a/README.md § Architecture`](scenario-a/README.md#architecture) and [`scenario-b/README.md § Architecture`](scenario-b/README.md#architecture).

---

## Repository Structure

```
cbweb3-platform/
├── scenario-a/             Scenario A — Enhanced Correspondent Banking
│   ├── backend/            Go microservices (api-gateway, auth, compliance, payment-orchestrator)
│   ├── contracts/          Solidity smart contracts (Foundry)
│   ├── frontend/           React applications (bank, governance, supervisor, treasury, noc)
│   ├── deploy/             Docker Compose infrastructure (Besu, Keycloak, Postgres, Paladin)
│   ├── interop/            Cross-spoke relay and bridge connectors
│   ├── apis/               OpenAPI specs, proto definitions, generated SDKs
│   ├── tests/              Unit, integration, e2e, and performance test harnesses
│   ├── docs/               Architecture diagrams, design docs, runbooks
│   ├── tryouts/            Per-entity demo scripts
│   └── README.md           Scenario A detailed documentation
└── scenario-b/             Scenario B — International Hub with FX Liquidity
    ├── backend/            Go microservices (api-gateway, auth, compliance, fx, ledger-gateway, payment-orchestrator, payments)
    ├── contracts/          Solidity smart contracts (Foundry) — AMM, registries, SpokeBridge
    ├── frontend/           React applications (bank, governance, supervisor, treasury, noc)
    ├── deploy/             Docker Compose infrastructure (hub + per-spoke Besu, Keycloak, Postgres, Paladin)
    ├── interop/            Cacti-based hub-and-spoke relay
    ├── provisioning/       Compose templates and manifest schema consumed by the toolkit
    ├── toolkit/            Declarative provisioning CLI (found-hub, found-spoke, join, observe)
    ├── samples/            Example deployment manifests
    ├── apis/               OpenAPI specs, proto definitions, generated SDKs
    ├── tests/              Unit, integration, e2e, and performance test harnesses
    ├── docs/               Architecture diagrams, design docs, runbooks
    ├── tryouts/            Per-entity demo scripts
    └── README.md           Scenario B detailed documentation
```

---

## Prerequisites

| Tool | Purpose |
|------|---------|
| **Docker & Docker Compose** | Container orchestration |
| **GNU Make** | Build automation |
| **Go** (1.22+) | Backend services |
| **Node.js** (22+) & npm | Frontend applications |
| **Foundry** (forge, cast) | Solidity compilation and deployment |
| **jq** | JSON processing in shell scripts |
| **openssl** | PKI certificate generation |

> **Note:** The full platform runs ~30 containers. Allocate at least 8 GB RAM and 4 CPUs to Docker.

---

## Getting Started

Each scenario is self-contained. Navigate to the scenario directory and follow its README:

```bash
# Scenario A — Enhanced Correspondent Banking
cd scenario-a
cat README.md
make spoke-all
```

For a detailed walkthrough of Scenario A, including the cross-spoke HTLC demo, architecture, port reference, and all make targets, see [`scenario-a/README.md`](scenario-a/README.md).
