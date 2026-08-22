# Runbooks — CBWeb3 Platform · Scenario B

> **Project:** RG-T4567 · Suboperation ATN/KS-21330-RG
> **Authors:** Lucas Campelo, Samuel Venzi
> **Date:** 2026-05-29

> **Status: Work in progress.** Scenario B is implemented but not 100% complete. Some flows (US3 commercial bank swap, sovereign liquidity reconciliation auto-recovery) are partially implemented. Refer to the known limitations section in each document.

---

## Deliverable 9 — CBDC System Deployment

| Document | Description |
|----------|-------------|
| [deployment-runbook.md](deployment-runbook.md) | Complete deployment guide: `cd samples && ./deploy-all.sh`, step-by-step phases (PKI → infra → contracts → seed → backend → relayer), health checks, AMM smoke test, teardown |
| [environment-setup.md](environment-setup.md) | Prerequisites, tool installation, complete port reference for all 7 entities, Besu RPC/WS ports, Paladin ports, shared infra ports, Cacti relay port |

**Quick start:**

```bash
# 1. Prepare environment (see environment-setup.md)
# 2. Deploy full stack
cd scenario-b/samples && ./deploy-all.sh
# 3. Verify endpoints
curl http://localhost:38080/healthz   # central-bank-a (hub authority)
curl http://localhost:18080/healthz   # bank-a
curl http://localhost:8081/realms/master | jq .realm
```

---

## Deliverable 10 — CBDC System Configuration

| Document | Description |
|----------|-------------|
| [configuration-reference.md](configuration-reference.md) | Complete environment variable reference by component: per-entity common vars, hub/AMM-specific vars, MLP vars, Cacti relay vars |
| [contract-configuration.md](contract-configuration.md) | Contract deployment parameters, hub and spoke contracts, on-chain roles, PairRegistry bilateral approval, AMM circuit breaker, LiquidityCommitRegistry TTL, Keycloak realms, PKI, post-deployment checklist |
| [identity-registry-role-separation.md](identity-registry-role-separation.md) | Two-step IdentityRegistry onboarding (Pending→Verified), GOVERNANCE_ROLE vs VERIFIER_ROLE separation of duties, and the production role-split runbook (R1-10.6 / R2-10.6) |

---

## Migration notes

| Document | Description |
|----------|-------------|
| [besu-chainid-migration-notes.md](besu-chainid-migration-notes.md) | Network-parameter delta D6 v2 → D12: Besu `24.x → 25.8.0` (pinned) and chain IDs `80000/80001/80002 → 1337/1338/1339`, with compatibility verification, LNET coordination, and a flagged Besu version-skew risk in the Scenario B bring-up scripts |

---

## Portal documentation

| Document | Description |
|----------|-------------|
| [ONBOARDING-DOCS.md](ONBOARDING-DOCS.md) | Commercial bank onboarding flow (Bank Portal + Governance Portal) |
| [onboarding-novo-cb-soberano.md](onboarding-novo-cb-soberano.md) | New sovereign CB onboarding procedure |
| [fx-agreement-reconciliation.md](fx-agreement-reconciliation.md) | FX agreement reconciliation flows |

---

## Scenario B — User Stories

| User Story | Description | Status |
|-----------|-------------|--------|
| **US1** — Cooperative Sovereign Liquidity | Central banks (CB-A and CB-B) each commit their own currency to seed the AMM pool via commit-reveal protocol | Implemented |
| **US2** — MLP Bilateral Liquidity | Multilateral Liquidity Provider deposits both sides of a pair when a CB lacks capacity | Implemented (opt-in) |
| **US3** — Commercial Bank Cross-Currency Swap | Bank-A quotes and executes a BRL→EUR swap through the hub AMM; Cacti relay bridges tokens cross-spoke | Partially implemented |
| **US4** — AMM Circuit Breaker | Central banks can pause/resume the AMM via 2-of-2 quorum | Implemented |
| **US5** — PairRegistry Bilateral Approval | CB-A proposes a new currency pair; CB-B confirms it on-chain | Implemented |

---

## Status by scenario component

| Component | Deployment | Configuration | Notes |
|-----------|-----------|---------------|-------|
| **Infra** (Keycloak, Postgres, Redis) | Runnable | Documented | Shared topology with Scenario A |
| **Hub Besu network** | Runnable (co-located with Spoke-A in local dev) | Documented | No isolated hub node yet — known limitation |
| **Spoke-A** (chain 1338) | Runnable | Documented | — |
| **Spoke-B** (chain 1339) | Runnable | Documented | — |
| **Hub contracts** (AMM, PairRegistry, LCR) | Deployed via `make scenario-b.deploy-contracts` | Documented | — |
| **Spoke contracts** | Deployed via `make scenario-b.deploy-contracts` | Documented | — |
| **Backend services** (6 entities) | Provisioned and started by `cd samples && ./deploy-all.sh` | Documented | — |
| **MLP stack** | Runnable via `make scenario-b.up-backend-mlp` | Documented | Requires `ENABLE_MLP=true` |
| **Cacti relay** (LiquidityCommitWatcher) | Runnable via `make scenario-b.up-relayer` | Documented | — |
| **US3 commercial bank swap** | Partial | Documented | Use governance account for full testing |
| **Sovereign liquidity auto-recovery** | Not implemented | — | Manual recovery required on failed matched commits |
| **Paladin/Zeto on hub** | Not implemented | — | Privacy available on spokes only |
