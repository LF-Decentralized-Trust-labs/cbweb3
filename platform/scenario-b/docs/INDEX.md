# Documentation Index — CBWeb3 Platform (Scenario B)

> Master navigable index for all Scenario B documentation produced under RG-T4567 · ATN/KS-21330-RG.
>
> Project: RG-T4567 · Suboperation: ATN/KS-21330-RG
> Authors: Lucas Campelo, Samuel Venzi
> Date: 2026-05-29

---

> **STATUS — IMPLEMENTATION IN PROGRESS**
>
> Scenario B is implemented but not 100% complete. Several components are functional and have been tested end-to-end at the governance level; others are partially implemented and undergoing active adjustment. Some documents listed in this index are forthcoming — they will be created as part of the current documentation effort or the next implementation phase. Links marked **[FORTHCOMING]** do not yet exist on disk. All known gaps are documented in the [Known Gaps](#known-gaps) section of the architecture overview.

---

## Quick Navigation

| I need to… | Go to |
|------------|-------|
| Understand Scenario B architecture | [Architecture Overview](#deliverable-11--architecture-documentation) |
| Deploy Scenario B locally | [Deployment Runbook](#deliverable-9--cbdc-system-deployment) |
| Configure services and hub contracts | [Configuration Reference](#deliverable-10--cbdc-system-configuration) |
| Understand the AMM and hub contracts | [Architecture Overview — Hub Smart Contract Layer](architecture/architecture-overview.md#hub-smart-contract-layer) |
| Understand the cooperative liquidity flow | [Cooperative Liquidity Design](#flow-diagrams) |
| Understand the AMM swap flow | [Swap Sequence Diagram](#flow-diagrams) |
| Run tests for Scenario B | [Test Execution Plan](#deliverable-12--cbdc-system-testing) |
| Find a specific test case | [Test Catalog](#deliverable-12--cbdc-system-testing) |
| Understand participant onboarding | [Onboarding Runbook](#portal-documentation) |
| Know what is not yet finished | [Known Gaps](architecture/architecture-overview.md#known-gaps) |
| Compare with Scenario A | [Scenario A Documentation](#scenario-a-reference) |

---

## Deliverable 9 — CBDC System Deployment

| Document | Status | Description |
|----------|--------|-------------|
| [runbooks/deployment-runbook.md](runbooks/deployment-runbook.md) | **[FORTHCOMING]** | Complete deployment guide: hub and spoke bring-up, phase-by-phase walkthrough, health checks, AMM smoke test, and teardown procedure |
| [runbooks/environment-setup.md](runbooks/environment-setup.md) | **[FORTHCOMING]** | Prerequisites, tool installation (Go, Foundry, Docker, k6), and full port reference for all 7 entities (6 core + MLP opt-in) |

**Quick start (when runbook is available):**

```bash
# 1. Prepare environment
# See: runbooks/environment-setup.md

# 2. Deploy hub and spokes
make hub-all
make spoke-all

# 3. Verify
curl http://localhost:18080/healthz   # bank-a (Spoke-A)
curl http://localhost:28080/healthz   # bank-b (Spoke-B)
curl http://localhost:38080/healthz   # central-bank-a (Hub / Spoke-A)
curl http://localhost:60080/healthz   # central-bank-b (Hub / Spoke-B)
curl http://localhost:4000/api/v1/health  # Cacti relay (scenario-b-liquidity mode)
```

---

## Deliverable 10 — CBDC System Configuration

| Document | Status | Description |
|----------|--------|-------------|
| [runbooks/configuration-reference.md](runbooks/configuration-reference.md) | **[FORTHCOMING]** | Complete environment variable reference by component: infra, api-gateway, auth, compliance, payment-orchestrator, contracts, frontend, NOC — across all entities; includes Scenario B-specific variables (`ENABLE_MLP`, AMM contract addresses, hub RPC, bridge parameters) |
| [runbooks/contract-configuration.md](runbooks/contract-configuration.md) | **[FORTHCOMING]** | Hub and spoke smart contract deployment parameters, on-chain RBAC roles (hub IdentityRegistry, tCeBM, AMM, PairRegistry, LiquidityCommitRegistry), Keycloak realm/client/role setup, PKI certificate structure, and post-deployment checklist |
| [runbooks/README.md](runbooks/README.md) | **[FORTHCOMING]** | Runbooks index — links to all D9 and D10 documents |

---

## Deliverable 11 — Architecture Documentation

| Document | Status | Description |
|----------|--------|-------------|
| [architecture/architecture-overview.md](architecture/architecture-overview.md) | **Available** | **Comprehensive architecture narrative** — network topology, all layers (frontend → contracts → Cacti), cooperative liquidity and AMM swap flows, all state machines, data flow boundaries, key design decisions, and known gaps |
| [architecture/CBWeb3-ComponentDiagram.png](architecture/CBWeb3-ComponentDiagram.png) | Available | Full component diagram (PNG export) |
| [architecture/cbweb3-component-diagram-with-paladin.excalidraw](architecture/cbweb3-component-diagram-with-paladin.excalidraw) | Available | Editable Excalidraw source for the component diagram |
| [architecture/fx-agreement-hybrid-design.md](architecture/fx-agreement-hybrid-design.md) | Available | FX Agreement hybrid design — backend state machine + hub `FXAgreement` contract |
| [architecture/fx-agreement-production-hardening.md](architecture/fx-agreement-production-hardening.md) | Available | Production hardening notes for the FX Agreement service |

### Flow Diagrams

| Diagram | Status | Description |
|---------|--------|-------------|
| [charts/scenario-b/architecture.md](charts/scenario-b/architecture.md) | Available | Full Mermaid component diagram (hub, spokes, all services, contracts, and Cacti relay) |
| [charts/scenario-b/swap.md](charts/scenario-b/swap.md) | Available | Cross-chain AMM swap sequence diagram (quote → lock-mint → swap → burn-unlock) |
| [design/cooperative-liquidity.md](design/cooperative-liquidity.md) | Available | Cooperative liquidity commit-reveal protocol design (pair proposal, commit matching, Cacti-triggered execution) |

---

## Deliverable 12 — CBDC System Testing

| Document | Status | Description |
|----------|--------|-------------|
| [test-execution-plan.md](test-execution-plan.md) | **[FORTHCOMING]** | Test execution plan: strategy, scope, phase timeline with LNET/Banks RACI matrix, AMM E2E flows, circuit breaker and bridge tests, performance thresholds, security controls, CI/CD quality gates, and evidence bundle format |
| [../tests/TEST-CATALOG.md](../tests/TEST-CATALOG.md) | **[FORTHCOMING]** | Test case catalog — smart contract tests (Foundry), API unit tests (Go), integration tests, and E2E tests mapped to Scenario B tryout scripts |

**Run tests manually (when available):**

```bash
# Smart contract unit tests
cd contracts && forge test -v

# Go unit tests
cd backend && go test ./...

# E2E: cooperative liquidity flow
./tryouts/tryout-cooperative-liquidity.sh

# E2E: AMM swap flow
./tryouts/tryout-amm-swap.sh
```

---

## Portal Documentation

Operational guides for each web portal, including Scenario B-specific AMM and bridge operations.

| Document | Status | Portal | Primary Users |
|----------|--------|--------|--------------|
| [runbooks/ONBOARDING-DOCS.md](runbooks/ONBOARDING-DOCS.md) | Available | Onboarding Flow | Governance officers + commercial banks — participant onboarding wizard |
| Bank Portal docs | **[FORTHCOMING]** | Bank Portal | Commercial bank operators — AMM swap, bridge lock-mint / burn-unlock, FX agreements |
| Treasury Portal docs | **[FORTHCOMING]** | Treasury Portal | CB treasury officers — liquidity commits, pair proposals, hub token minting |
| Supervisor Portal docs | **[FORTHCOMING]** | Supervisor Portal | Regulators — audit log, compliance screening, account freeze/unfreeze |
| NOC Dashboard docs | **[FORTHCOMING]** | NOC Dashboard | Network operators — node health, block monitoring, Cacti watcher status |

---

## Architecture and Design Documents

| Document | Status | Description |
|----------|--------|-------------|
| [architecture/architecture-overview.md](architecture/architecture-overview.md) | Available | Full Scenario B architecture narrative |
| [architecture/fx-agreement-hybrid-design.md](architecture/fx-agreement-hybrid-design.md) | Available | FX Agreement design — hub contract + backend state machine |
| [architecture/fx-agreement-production-hardening.md](architecture/fx-agreement-production-hardening.md) | Available | Production hardening notes |
| [design/cooperative-liquidity.md](design/cooperative-liquidity.md) | Available | Cooperative liquidity commit-reveal design |

---

## Runbooks and Operational Guides

| Document | Status | Description |
|----------|--------|-------------|
| [runbooks/README.md](runbooks/README.md) | **[FORTHCOMING]** | Runbooks index |
| [runbooks/deployment-runbook.md](runbooks/deployment-runbook.md) | **[FORTHCOMING]** | Full deployment guide for Scenario B |
| [runbooks/environment-setup.md](runbooks/environment-setup.md) | **[FORTHCOMING]** | Prerequisites and port reference |
| [runbooks/configuration-reference.md](runbooks/configuration-reference.md) | **[FORTHCOMING]** | Environment variable reference |
| [runbooks/contract-configuration.md](runbooks/contract-configuration.md) | **[FORTHCOMING]** | Smart contract deployment parameters and on-chain role configuration |
| [runbooks/fx-agreement-reconciliation.md](runbooks/fx-agreement-reconciliation.md) | Available | FX Agreement state reconciliation procedure |
| [runbooks/onboarding-novo-cb-soberano.md](runbooks/onboarding-novo-cb-soberano.md) | Available | Procedure for onboarding a new sovereign central bank to the hub |
| [runbooks/ONBOARDING-DOCS.md](runbooks/ONBOARDING-DOCS.md) | Available | Participant onboarding wizard documentation |

---

## Component READMEs

Per-module documentation maintained alongside source code.

| Module | README | Description |
|--------|--------|-------------|
| Scenario B root | [../README.md](../README.md) | Platform overview, quick start, directory structure |
| Backend | [../backend/README.md](../backend/README.md) | Go microservices overview |
| — api-gateway | [../backend/services/api-gateway/README.md](../backend/services/api-gateway/README.md) | REST entry point, routing, middleware, Scenario B AMM and bridge endpoints |
| Contracts | [../contracts/README.md](../contracts/README.md) | Solidity smart contracts (Foundry) — AMM, PairRegistry, LiquidityCommitRegistry, FXAgreement, tCeBM, IdentityRegistry |
| Interoperability | [../interop/README.md](../interop/README.md) | Cacti relay — Scenario B LiquidityCommitWatcher mode |
| Infrastructure | [../deploy/README.md](../deploy/README.md) | Docker Compose deployment profiles (hub + spokes) |
| PKI | [../backend/config/pki/README.md](../backend/config/pki/README.md) | X.509 certificate generation and management |

---

## External and Reference Documents

| Document | Description |
|----------|-------------|
| [external/scenario-b/scenario-b.md](external/scenario-b/scenario-b.md) | External Scenario B specification and requirements |
| [external/scenario-b/glossary.md](external/scenario-b/glossary.md) | Glossary of terms for Scenario B |

---

## Scenario A Reference

Scenario A (Enhanced Correspondent Banking — bilateral HTLC, no hub) is documented separately. Many foundational design decisions (three-layer authorization, Cacti relay resilience, backend service stack) are shared between scenarios.

| Resource | Description |
|----------|-------------|
| Scenario A docs | `scenario-a/docs/` in the main repository |
| Scenario A architecture overview | Full bilateral correspondent banking architecture with Paladin/Zeto, HTLC, and Cacti relay |

---

## Document Status Summary

| Deliverable | Documents | Status |
|-------------|-----------|--------|
| D9 — Deployment | deployment-runbook.md, environment-setup.md | Forthcoming |
| D10 — Configuration | configuration-reference.md, contract-configuration.md, runbooks/README.md | Forthcoming |
| D11 — Architecture | architecture-overview.md, component diagrams, flow charts, design documents | **Available** (this index + architecture-overview.md created in current sprint) |
| D12 — Testing | test-execution-plan.md, TEST-CATALOG.md | Forthcoming |
| Portal docs | Bank, Treasury, Supervisor, NOC portals | Forthcoming (onboarding docs available) |
| Runbooks | fx-agreement-reconciliation.md, onboarding-novo-cb-soberano.md | Available |
| Design | cooperative-liquidity.md, fx-agreement-hybrid-design.md | Available |
| External | scenario-b.md, glossary.md | Available |
