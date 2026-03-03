# cbweb3-platform

**Status:** _Suggested structure (baseline)_.  
This repository proposes a **reference structure** for the CBWeb3 platform codebase delivered by the external development firm, covering **smart contracts, backend services, frontend apps, public APIs, and interoperability components** (hub‑and‑spoke and single‑ledger). It is designed as a pragmatic monorepo, ready to be adapted to LACNet/LACChain operational policies (security, CI/CD, naming, environments).

> Objective: enable **modular development**, **clean separations of concern**, and **reproducible deployments** across dev/staging/testnet, with strong governance (CODEOWNERS, PR checks, SBOM/SAST) and DPG‑aligned openness.

---

## Proposed structure

```
/contracts                 (Solidity smart contracts, tests, scripts)
  src/
  test/
  scripts/

/backend                   (domain services & shared libs)
  services/
    payments/              (wCBDC payments, P2P/PvP/DvP flows)
    fx/                    (FX pricing & settlement adapters)
    ledger-gateway/        (RPC/WS client, event subscribers)
    compliance/            (KYC/AML hooks, audit trails)
    identity/              (PKI/OIDC/SSI abstractions)
  shared/                  
  config/

/frontend                  (operator consoles, dashboards)
  apps/console/
  packages/ui/

/apis
  openapi/                 (YAML contracts per domain)
  sdk/                     (auto‑generated clients)
    ts/
    python/
    java/

/interop                   (hybrid interop layer)
  hub-and-spoke/
    cacti/                 (Hyperledger Cacti connectors)
    ccip/                  (Chainlink CCIP adapters)
  single-ledger/           (intra‑network flows)

/deploy                    (infra as code & release manifests)
  terraform/
  helm-k8s/
  cloud-run/

/tests                     (test harnesses)
  unit/
  integration/
  e2e/
  performance/

/docs                      (architecture, design, operations)
  architecture/
  design/
  governance/
  runbooks/

/.github/workflows         (CI/CD pipelines)
SECURITY.md | CONTRIBUTING.md | CODEOWNERS | LICENSE | .gitignore
```

---

## Component descriptions

### `/contracts`
- Solidity sources, unit tests (e.g., Foundry/Hardhat), and deployment **scripts**.  
- Keep **ABI & addresses** versioned per environment under `/deploy/` or `/apis/openapi/` for consumers.

### `/backend`
- Domain‑oriented services (payments, FX, gateway, compliance, identity).  
- Include adapters and **anti‑corruption layers** to isolate external systems (oracles, KMS, identity providers).  
- Shared libraries under `/backend/shared` and centralized configuration under `/backend/config` (12‑factor).

### `/frontend`
- Operator/participant UIs. Keep shared UI elements in `packages/ui`.  
- Serve only non‑sensitive configuration via `public/` and environment variables.

### `/apis`
- **OpenAPI** specs as the **source of truth** for external consumption.  
- Auto‑generate **SDKs** (TS/Python/Java) under `/apis/sdk/` and publish via GitHub Packages when tagged.

### `/interop`
- Hybrid model split:  
  - **hub-and-spoke/** → cross‑network connectors (**Cacti**, **CCIP**).  
  - **single-ledger/** → intra‑network orchestration & sequences.  
- Document assumptions (security, failure domains, retries, idempotency).

### `/deploy`
- IaC for cloud/K8s and deployment manifests. Maintain **per‑environment** variables and remote state.  
- Include Helm charts for services and validators (if applicable).

### `/tests`
- Unit/integration/E2E/performance harnesses.  
- Include **synthetic workloads** and golden scenarios aligned with `/apis/openapi/` contracts.

### `/docs`
- Architecture (ISO‑42010 views), design decisions (ADRs), runbooks (ops, DR, rollback), and governance (RACI, release process).

### `/.github/workflows`
- CI jobs for lint/tests, **SAST/SBOM**, and artifact publication (SDKs, container images, charts).

### Root files
- **`SECURITY.md`**: how to report vulnerabilities and response expectations.  
- **`CONTRIBUTING.md`**: branching model, commit conventions, DCO/CLA (if any), code review policy.  
- **`CODEOWNERS`**: ownership per domain (contracts, backend, interop, frontend, deploy).  
- **`LICENSE`**: recommended **Apache‑2.0** (unless specified otherwise).  
- **`.gitignore`**: ignore build artifacts, secrets, caches (see sample below).

---

## Sample `.gitignore` (recommendation)

```
# Node / Frontend
node_modules/
dist/
.next/
coverage/
*.log

# Python / Go caches
__pycache__/
*.pyc

# Environment
.env
.env.*

# Terraform
.terraform/
terraform.tfstate*
*.tfvars

# Generated SDKs / Artifacts
apis/sdk/*/dist/
build/
artifacts/

# OS
.DS_Store
```

---

## Workflow (recommended)

1. Develop on `feature/*` branches. Keep PRs scoped to a **single domain** when possible.  
2. CI runs unit/integration tests, SAST, and SBOM generation.  
3. CODEOWNERS review by domain; merge to `main`.  
4. Tag releases (semver). Publish SDKs/images/charts and attach artifacts.  
5. Promote to `staging/testnet` via `/deploy` runbooks and environment approvals.

---

## Prerequisites (suggested)

- Node.js + package manager (pnpm/yarn/npm) for frontend/SDK tooling.  
- A smart‑contract framework (Foundry/Hardhat) for `/contracts`.  
- Docker/Compose and optional Kubernetes toolchain for `/deploy`.  
- Organization secrets manager for credentials—**never** commit secrets.

---

## Notice

This is a **suggested structure** for the CBWeb3 platform. Teams may adapt folders, tooling, and pipelines to organizational policies while preserving the principles of **modularity, reproducibility, and open governance**.
