# CBWeb3

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Toolbox CI](https://github.com/LF-Decentralized-Trust-labs/cbweb3/actions/workflows/toolbox-ci.yml/badge.svg)](https://github.com/LF-Decentralized-Trust-labs/cbweb3/actions/workflows/toolbox-ci.yml)

CBWeb3 is an IDB Lab project that seeks to enable a regional test-network (**Test Net**) for Latin America and the Caribbean that will allow for the issuance of **tCeBM (tokenized Central Bank Money)** and the tokenization of financial assets, with a focus on cross-regional interoperability between central banks and financial institutions.

The **Test Net** will build upon the technological capabilities and infrastructure of **IDB Lab's initiative [LNet](https://lnet.global/)** — the Alliance for the Development of the Blockchain Ecosystem in Latin America and the Caribbean.

---

## How it works

CBWeb3 connects domestic central bank networks through a shared transnational hub, enabling three types of transactions:

![CBWeb3 Transaction Types](docs/images/CBWeb3%20transactions.png)

| Transaction | Description |
|-------------|-------------|
| **Domestic issuance & redemption** | A central bank issues or redeems tCeBM within its own network |
| **Enhanced Correspondent Banking** | Two parties settle a cross-border payment atomically using Hash Time-Lock Contracts (HTLC) |
| **Automated Market Maker (AMM)** | Commercial banks exchange currencies through a shared liquidity pool on the transnational hub |

### Enhanced Correspondent Banking (PvP with HTLC)

The primary settlement flow — a bilateral Payment-versus-Payment using HTLC across two domestic networks:

![Enhanced Correspondent Banking flow](docs/images/CBWeb3%20Enh.png)

1. Commercial Bank A proposes an FX agreement to Correspondent Bank D
2. Correspondent Bank D accepts the agreement
3. Central Bank A issues tCeBM-A to Commercial Bank A
4. Central Bank B issues tCeBM-B to Commercial Bank B
5. HTLC locks tCeBM-A on Network A
6. tCeBM-A is transferred to Correspondent Bank C
7. tCeBM-B is transferred to Commercial Bank A — settlement complete

### Automated Market Maker (AMM)

A hub-based liquidity pool for multi-currency exchange:

![AMM flow](docs/images/CBWeb3%20AMM.png)

Commercial banks bridge tCeBM to the transnational network, where an AMM contract determines exchange rates algorithmically and executes swaps atomically.

---

## Architecture

The platform uses a **Hub & Spoke topology**: each country operates a domestic Besu network (Spoke), connected through a shared transnational settlement layer (Hub) via Hyperledger Cacti.

![CBWeb3 System Component Architecture](docs/images/CBWeb3-ComponentDiagramV3.png)

| Layer | Key components |
|-------|---------------|
| **Frontend** | Treasury Portal, NOC Portal, Supervisor Portal, Governance Portal, Bank Integration Portal |
| **API** | REST Gateway (OpenAPI 3.0.3), WebSocket Gateway |
| **Microservices** | Policy Engine, Identity Service, Privacy Service, Time Service, Observability Hub |
| **Blockchain** | Hyperledger Besu (QBFT consensus, gasless), Hyperledger Paladin (ZK-SNARKs privacy via Zeto tokens), Cacti (cross-chain relay) |
| **Smart contracts** | HTLC, LP/AMM Pool, FX Oracle, Atomicity Coordinator |

---

## What's in this repository

This repository is the **coordination hub** for the CBWeb3 DPG Working Group. It contains documentation, governance artifacts, and the **Toolbox** — a shared integration kit for implementers and contributors.

```
cbweb3/
├── Toolbox/              Integration kit: contracts, mocks, test vectors,
│                         conformance tests, sandbox tutorials
├── docs/                 DPG assessment, research papers, public documentation
├── CONTRIBUTING.md       Governance model, roles, decision-making process
└── README.md             This file
```

> The full CBWeb3 platform (microservices, smart contracts, frontend portals) is developed by **AguilaHub / GoLedger** under contract with IDB Lab, delivered as milestones D1–D7. This repository documents the interfaces and provides tooling for the community to build on top of that platform.

### The Toolbox

The [**Toolbox**](Toolbox/README.md) is a curated set of integration-ready artifacts that help implementers build, test, and validate CBWeb3-compatible systems:

| Artifact | What it provides | Status |
|----------|-----------------|--------|
| [Interface contracts](Toolbox/contracts/pvp/) | OpenAPI 3.0.3 specs for PvP settlement (7 endpoints) | Available |
| [Reference mocks](Toolbox/mocks/pvp/) | Canonical API responses for the full PvP flow (8 fixtures) | Available |
| [Test vectors](Toolbox/test-vectors/pvp/) | Deterministic input/output fixtures (13 vectors) | Available |
| [Conformance tests](Toolbox/conformance/) | Executable compliance checks (16 test methods) | Available |
| [Sandbox & tutorials](Toolbox/sandbox/) | Quick-start guides, mock server setup, step-by-step tutorials | Available |

### Community Backlog

The [**Toolbox Backlog**](https://github.com/orgs/LF-Decentralized-Trust-labs/projects/5) tracks modules and tools that the **community** is invited to develop:

| Issue | Module | Category |
|-------|--------|----------|
| [#28](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/28) | CCIP Integration Adapter | Interoperability |
| [#29](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/29) | Fabric-X Corridor Kit | Interoperability |
| [#30](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/30) | Paladin Privacy Conformance Suite | Privacy & Compliance |
| [#31](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/31) | Policy-as-Code Starter Pack | Privacy & Compliance |
| [#32](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/32) | Gasless Anti-Spam Controller | Ops & Reliability |
| [#33](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/33) | Observability Pack (NOC-in-a-box) | Ops & Reliability |
| [#34](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/34) | Secure Dev & Supply Chain Templates | Security |
| [#35](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/35) | Permissioning / Allowlisting Module | Security |
| [#36](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/36) | Community-built SDK | Interoperability |

---

## Getting started

**Run the PvP tutorial** (15 min) — Execute a full cross-border settlement against a mock server:
> [Toolbox/sandbox/tutorials/01-pvp-settlement-mock.md](Toolbox/sandbox/tutorials/01-pvp-settlement-mock.md)

**Validate an implementation** — Run conformance tests against your own API:
> [Toolbox/sandbox/tutorials/02-validate-implementation.md](Toolbox/sandbox/tutorials/02-validate-implementation.md)

**Contribute to the backlog** — Pick an issue from the [Toolbox Backlog](https://github.com/orgs/LF-Decentralized-Trust-labs/projects/5) and follow the [contribution guide](Toolbox/CONTRIBUTING.md).

**Explore the research** — Read the [DPG assessment](docs/dpg-assesment/CBWeb3_as_a_DPG.md), the [Privacy vs Confidentiality analysis](docs/dpg-assesment/Privacy%20vs%20Confidentiality.md), or the governance, privacy, and interoperability research papers in [docs/research-papers/](docs/research-papers/).

---

## CBWeb3 as a DPG

CBWeb3 is explicitly designed to meet the [DPGA criteria](https://digitalpublicgoods.net/):

- **Open-source** — Apache-2.0 license, all artifacts public and reusable
- **Open standards** — Hyperledger Besu (EVM), OpenAPI 3.0.3, ISO 20022, ERC-20/ERC-1400
- **Privacy-by-design** — Hyperledger Paladin with Zeto tokens (ZK-SNARKs) for on-chain privacy
- **Vendor neutrality** — No mandatory proprietary dependencies
- **SDG alignment** — Targets SDGs 8 (Decent Work), 9 (Industry & Innovation), 10 (Reduced Inequalities), 17 (Partnerships)

We are building an **open DPG Working Group (DPG WG)** to:
- Steward design decisions
- Accept community RFCs/PRs
- Publish materials so any competent team can deploy, operate, and extend the system.

Once the defined milestones are met, we will:
1. Complete the **DPGA self-assessment**
2. Apply for official **DPG registration**
3. Keep all artifacts **public and reusable** so the ecosystem can benefit beyond this project

See the full [DPG self-assessment](docs/dpg-assesment/CBWeb3_as_a_DPG.md) for details.

---

## Governance

CBWeb3 is governed by a **DPG Working Group (DPG WG)** with open participation:

| Role | Who | Responsibilities |
|------|-----|-----------------|
| **Maintainer Core** | LNet technical lead + LFDT approvers | Merge PRs, enforce workflow |
| **Cochair** | CEMLA / FLAR nominees | Facilitate meetings, curate backlog |
| **Contributor** | Any individual or organization | File issues, submit PRs, review code |

**Decisions** follow a consensus-first model with a GitVote fallback (50%+1 quorum, simple majority).

See the full [participation guidelines](CONTRIBUTING.md) for details on roles, voting, and the decision-making process.

### Join us

- **Bi-weekly tech sync** — Thursdays at 9 AM Pacific via [Zoom LFX](https://zoom-lfx.platform.linuxfoundation.org/meetings/lf-decentralized-trust-labs?view=week)
- **Discord** — [#cbweb3 channel](https://discord.lfdecentralizedtrust.org)
- **GitHub** — [Issues](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues) and [Discussions](https://github.com/LF-Decentralized-Trust-labs/cbweb3/discussions)

---

## Partners

| Organization | Role |
|-------------|------|
| [IDB Lab](https://bidlab.org/) | Project sponsor |
| [LNet](https://lnet.global/) | Infrastructure and technical coordination |
| [CEMLA](https://www.cemla.org/) | Regional monetary advisory, WG co-chair |
| [FLAR](https://www.flar.net/) | Regional financial advisory, WG co-chair |
| [LACChain](https://www.lacchain.net/) | Blockchain infrastructure (LACNet) |
| [LFDT](https://www.lfdecentralizedtrust.org/) | Open-source governance (Linux Foundation) |
| [AguilaHub / GoLedger](https://goledger.com.br/) | Core platform development (D1–D7) |

---

## License

[Apache-2.0](LICENSE)

---

## Glossary

| Term | Definition |
|------|-----------|
| **tCeBM** | Tokenized Central Bank Money — digital representation of central bank currency on a blockchain |
| **PvP** | Payment-versus-Payment — simultaneous exchange of two currencies to eliminate settlement risk |
| **HTLC** | Hash Time-Lock Contract — smart contract that enables atomic swaps using cryptographic hash locks and time limits |
| **AMM** | Automated Market Maker — algorithm that provides liquidity and determines exchange rates via a mathematical formula |
| **DPG** | Digital Public Good — open-source software that contributes to the UN Sustainable Development Goals |
| **Spoke** | A domestic blockchain network operated by a single country's central bank |
| **Hub** | The shared transnational settlement network connecting all Spokes |
| **CEMLA** | Center for Latin American Monetary Studies |
| **FLAR** | Latin American Reserve Fund |
| **LNet** | Alliance for the Development of the Blockchain Ecosystem in Latin America and the Caribbean |
| **LFDT** | Linux Foundation Decentralized Trust (formerly Hyperledger Foundation) |
