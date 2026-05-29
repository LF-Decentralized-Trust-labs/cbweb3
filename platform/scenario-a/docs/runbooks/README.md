# Runbooks — CBWeb3 Platform

Operational documentation for deployment and configuration of the CBWeb3 platform.

---

## Deliverable 9 — CBDC System Deployment

| Document | Description |
|----------|-------------|
| [deployment-runbook.md](deployment-runbook.md) | Complete deployment guide: `make spoke-all`, detailed phases, health checks, HTLC demo, teardown |
| [environment-setup.md](environment-setup.md) | Prerequisites, tool installation, complete port reference per entity |

**Quick start:**

```bash
# 1. Prepare environment (see environment-setup.md)
# 2. Deploy backend
cd scenario-a && make spoke-all
# 3. Deploy frontends
make frontend-spoke-all
# 4. Verify endpoints
curl http://localhost:18080/healthz   # bank-a
curl http://localhost:8081/realms/master | jq .realm
```

---

## Deliverable 10 — CBDC System Configuration

| Document | Description |
|----------|-------------|
| [configuration-reference.md](configuration-reference.md) | Complete environment variable reference by component (infra, api-gateway, auth, contracts, frontend, NOC, Paladin) |
| [contract-configuration.md](contract-configuration.md) | Contract deployment parameters, on-chain roles (IdentityRegistry, tCeBM), Keycloak permissions, PKI, and post-deployment checklist |

---

## Portal documentation (frontend)

| Document | Portal |
|----------|--------|
| [BANK-PORTAL-DOCS.md](BANK-PORTAL-DOCS.md) | Bank Portal — commercial bank operations |
| [TREASURY-PORTAL-DOCS.md](TREASURY-PORTAL-DOCS.md) | Treasury Portal — treasury management |
| [SUPERVISOR-PORTAL-DOCS.md](SUPERVISOR-PORTAL-DOCS.md) | Supervisor Portal — regulatory oversight |
| [NOC-PORTAL-DOCS.md](NOC-PORTAL-DOCS.md) | NOC Dashboard — network monitoring |
| [ONBOARDING-DOCS.md](ONBOARDING-DOCS.md) | Participant onboarding flow |

---

## Status by scenario

| Scenario | Deployment | Configuration |
|----------|-----------|---------------|
| **Scenario A** (Enhanced Correspondent Banking) | Documented and runnable | Documented |
| **Scenario B** (International Hub + AMM) | Pending — section reserved in each runbook | Pending — planned variables documented |
