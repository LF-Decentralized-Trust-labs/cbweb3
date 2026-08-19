# Documentation Index — CBWeb3 Platform (Scenario A)

> Master navigable index for all documentation produced under RG-T4567 · ATN/KS-21330-RG.
> Scenario B extensions are noted where applicable.

---

## Quick Navigation

| I need to… | Go to |
|------------|-------|
| Understand the system architecture | [Architecture Overview](#deliverable-11--architecture-documentation) |
| Deploy the platform locally | [Deployment Runbook](#deliverable-9--cbdc-system-deployment) |
| Configure services and contracts | [Configuration Reference](#deliverable-10--cbdc-system-configuration) |
| Consult the REST API | [OpenAPI Specification](#deliverable-10--cbdc-system-configuration) |
| Run a specific portal | [Portal Documentation](#portal-documentation) |
| Run the tests | [Test Execution Plan](#deliverable-12--cbdc-system-testing) |
| Find a specific test case | [Test Catalog](#deliverable-12--cbdc-system-testing) |
| Understand a flow (sequence diagram) | [Flow Diagrams](#flow-diagrams) |

---

## Deliverable 9 — CBDC System Deployment

| Document | Description |
|----------|-------------|
| [runbooks/deployment-runbook.md](runbooks/deployment-runbook.md) | Complete deployment guide: `make spoke-all`, phase-by-phase walkthrough, health checks, HTLC smoke test, and teardown procedure |
| [runbooks/environment-setup.md](runbooks/environment-setup.md) | Prerequisites, tool installation (Go, Foundry, Docker, k6), and full port reference for all 6 entities across both spokes |
| [`docs/TOOLCHAIN.md`](../../docs/TOOLCHAIN.md) | **Authoritative** tool versions for the whole platform (Go 1.26, Node 22 LTS, …) and where each floor is enforced |

**Quick start:**

```bash
# 1. Prepare environment
# See: runbooks/environment-setup.md

# 2. Deploy full stack
cd scenario-a && make spoke-all

# 3. Verify
curl http://localhost:18080/healthz   # bank-a
curl http://localhost:28080/healthz   # bank-b
curl http://localhost:58080/healthz   # central-bank-a
```

---

## Deliverable 10 — CBDC System Configuration

| Document | Description |
|----------|-------------|
| [runbooks/configuration-reference.md](runbooks/configuration-reference.md) | Complete environment variable reference by component: infra, api-gateway, auth, compliance, payment-orchestrator, contracts, frontend, NOC, Paladin — across all 6 entities |
| [runbooks/contract-configuration.md](runbooks/contract-configuration.md) | Smart contract deployment parameters, on-chain RBAC roles (IdentityRegistry, tCeBM, HTLC), Keycloak realm/client/role setup, PKI certificate structure, and post-deployment checklist |
| [../apis/openapi/api-gateway.yaml](../apis/openapi/api-gateway.yaml) | OpenAPI 3.0 specification — v2.3.0, 83 operations, all public and internal api-gateway endpoints |
| [../apis/README.md](../apis/README.md) | API index with endpoint summary table and usage guide |

---

## Deliverable 11 — Architecture Documentation

| Document | Description |
|----------|-------------|
| [architecture/architecture-overview.md](architecture/architecture-overview.md) | **Comprehensive architecture narrative** — network topology, all 6 layers (frontend → contracts → Paladin → Cacti), key flows, state machines, data flow boundaries, and design decisions |
| [architecture/CBWeb3-ComponentDiagram.png](architecture/CBWeb3-ComponentDiagram.png) | Full component diagram (PNG export) |
| [architecture/cbweb3-component-diagram-with-paladin.excalidraw](architecture/cbweb3-component-diagram-with-paladin.excalidraw) | Editable Excalidraw source for the component diagram |
| [architecture/fx-agreement-hybrid-design.md](architecture/fx-agreement-hybrid-design.md) | FX Agreement hybrid design — backend PostgreSQL state machine + Pente bilateral context |
| [architecture/fx-agreement-production-hardening.md](architecture/fx-agreement-production-hardening.md) | Production hardening notes for the FX Agreement service |

### Flow Diagrams

| Diagram | Description |
|---------|-------------|
| [charts/scenario-a/architecture.md](charts/scenario-a/architecture.md) | Full Mermaid component diagram (all services, networks, contracts, Paladin nodes, and Cacti relay) |
| [charts/scenario-a/escrow.md](charts/scenario-a/escrow.md) | Deposit → Escrow → Redeem sequence diagram (fCeBM → Zeto tCeBM → fCeBM lifecycle) |
| [charts/scenario-a/transfer.md](charts/scenario-a/transfer.md) | Cross-spoke FX Agreement + HTLC PvP settlement sequence diagram (4 phases + contingency refund path) |

---

## Deliverable 12 — CBDC System Testing

| Document | Description |
|----------|-------------|
| [test-execution-plan.md](test-execution-plan.md) | Test execution plan: strategy, scope, 9-phase timeline with LNET/Banks RACI matrix, API-First E2E flows, performance thresholds, security controls, CI/CD quality gates (pending), and evidence bundle format |
| [../tests/TEST-CATALOG.md](../tests/TEST-CATALOG.md) | Test case catalog: 159 cases (120 implemented, 39 planned) across smart contract (Foundry), API unit (Go), integration, and E2E levels — mapped to existing tryout scripts |

**Run tests manually:**

```bash
# Smart contract unit tests
cd scenario-a/contracts && forge test -v

# Go unit tests
cd scenario-a/backend && go test ./...

# E2E flows
./tryouts/tryout-fx-agreement-e2e.sh
./tryouts/tryout-escrow-flow.sh
./tryouts/tryout-cacti-interop.sh
```

---

## Portal Documentation

Operational guides for each web portal, including screenshots of the key user flows.

| Document | Portal | Primary Users |
|----------|--------|--------------|
| [runbooks/BANK-PORTAL-DOCS.md](runbooks/BANK-PORTAL-DOCS.md) | Bank Portal | Commercial bank operators — deposits, escrows, redeems, FX agreements, HTLC |
| [runbooks/TREASURY-PORTAL-DOCS.md](runbooks/TREASURY-PORTAL-DOCS.md) | Treasury Portal | Central bank treasury officers — deposit/escrow/redeem approvals |
| [runbooks/SUPERVISOR-PORTAL-DOCS.md](runbooks/SUPERVISOR-PORTAL-DOCS.md) | Supervisor Portal | Regulators — audit log, compliance screening, account freeze/unfreeze |
| [runbooks/NOC-PORTAL-DOCS.md](runbooks/NOC-PORTAL-DOCS.md) | NOC Dashboard | Network operators — node health, block monitoring, relay status |
| [runbooks/ONBOARDING-DOCS.md](runbooks/ONBOARDING-DOCS.md) | Onboarding Flow | Governance officers + commercial banks — participant onboarding wizard |

---

## Component READMEs

Per-module documentation maintained alongside source code.

| Module | README | Description |
|--------|--------|-------------|
| Scenario A root | [../README.md](../README.md) | Platform overview, quick start, directory structure |
| Backend | [../backend/README.md](../backend/README.md) | Go microservices overview |
| — api-gateway | [../backend/services/api-gateway/README.md](../backend/services/api-gateway/README.md) | REST entry point, routing, middleware |
| — auth | [../backend/services/auth/README.md](../backend/services/auth/README.md) | Identity, login, wallet binding, PKI |
| — compliance | [../backend/services/compliance/README.md](../backend/services/compliance/README.md) | Onboarding, AML screening, participant registry |
| — payment-orchestrator | [../backend/services/payment-orchestrator/README.md](../backend/services/payment-orchestrator/README.md) | HTLC, FX Agreement, Zeto escrow, Cacti integration |
| — noc-agent | [../backend/services/noc-agent/README.md](../backend/services/noc-agent/README.md) | Monitoring daemon |
| — noc-backend | [../backend/services/noc-backend/README.md](../backend/services/noc-backend/README.md) | NOC dashboard API |
| Contracts | [../contracts/README.md](../contracts/README.md) | Solidity smart contracts (Foundry) — HTLC, tCeBM, IdentityRegistry, FXAgreement, AMM |
| Interoperability | [../interop/README.md](../interop/README.md) | Cacti relay and cross-spoke bridge |
| Infrastructure | [../deploy/README.md](../deploy/README.md) | Docker Compose deployment profiles |
| PKI | [../backend/config/pki/README.md](../backend/config/pki/README.md) | X.509 certificate generation and management |
| APIs | [../apis/README.md](../apis/README.md) | OpenAPI specs, proto definitions, SDKs |

---

## Runbooks Index

| Document | Description |
|----------|-------------|
| [runbooks/README.md](runbooks/README.md) | Runbooks index — links to all D9 and D10 documents |
| [runbooks/fx-agreement-reconciliation.md](runbooks/fx-agreement-reconciliation.md) | FX Agreement state reconciliation procedure |

---

## Scenario B — International Hub with AMM

> **Status: Implemented — work in progress.** Not 100% complete; adjustments ongoing.

Scenario B has its own complete documentation set under `scenario-b/docs/`.

| Document | Description |
|----------|-------------|
| [`scenario-b/docs/INDEX.md`](../../../scenario-b/docs/INDEX.md) | Master documentation index for Scenario B |
| [`scenario-b/docs/architecture/architecture-overview.md`](../../../scenario-b/docs/architecture/architecture-overview.md) | Architecture narrative — hub topology, AMM, commit-reveal, SpokeBridge, Cacti relay |
| [`scenario-b/docs/runbooks/deployment-runbook.md`](../../../scenario-b/docs/runbooks/deployment-runbook.md) | Step-by-step deployment guide (`make scenario-b.up`) |
| [`scenario-b/docs/runbooks/environment-setup.md`](../../../scenario-b/docs/runbooks/environment-setup.md) | Prerequisites and port reference |
| [`scenario-b/docs/runbooks/configuration-reference.md`](../../../scenario-b/docs/runbooks/configuration-reference.md) | Full environment variable reference including hub/AMM vars |
| [`scenario-b/docs/runbooks/contract-configuration.md`](../../../scenario-b/docs/runbooks/contract-configuration.md) | Hub contract parameters, on-chain roles, PairRegistry, circuit breaker |
| [`scenario-b/docs/test-execution-plan.md`](../../../scenario-b/docs/test-execution-plan.md) | Test execution plan — AMM/bridge flows, RACI, performance targets |
| [`scenario-b/tests/TEST-CATALOG.md`](../../../scenario-b/tests/TEST-CATALOG.md) | Test catalog — 22 Foundry test files, Go unit tests, E2E (US1–US6) |

---

## Document Status

| Deliverable | Status | Documents |
|-------------|--------|-----------|
| D9 — Deployment | Complete | deployment-runbook.md, environment-setup.md |
| D10 — Configuration | Complete | configuration-reference.md, contract-configuration.md, OpenAPI v2.3.0 |
| D11 — Architecture | Complete | architecture-overview.md, component diagrams, flow charts, portal docs |
| D12 — Testing | Complete | test-execution-plan.md, TEST-CATALOG.md (159 cases) |
