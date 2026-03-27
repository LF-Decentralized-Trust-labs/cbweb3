# Tryouts

## Purpose

The `tryouts/` directory contains **end-to-end integration shell scripts** that exercise real HTTP flows against the locally running platform stack. They sit between unit/integration tests (which run in isolation) and full regression suites, serving three distinct purposes:

1. **Developer smoke-testing** — quickly validate that an entire API path works after a code change, without having to manually craft curl commands or configure a Postman collection.
2. **Living documentation** — each script narrates the exact sequence of API calls, actor roles, and expected outcomes for a business flow, making it the authoritative human-readable reference for how that flow works end-to-end.
3. **Onboarding reference** — new contributors can run a script against a local stack and observe the full request/response cycle before diving into service code.

> Tryouts are **not** automated test suite entries and are **not** executed by CI/CD pipelines. They depend on a live local environment with all services up and secrets in place.

---

## Architecture Context

The platform is organized around a **hub-and-spoke model**:

- **Spoke-A** — Besu chain `1338`, central bank `central-bank-a` (`localhost:38080`), commercial banks `bank-a` (`18080`) and `bank-c` (`48080`).
- **Spoke-B** — Besu chain `1339`, central bank `central-bank-b` (`localhost:60080`), commercial banks `bank-b` (`28080`) and `bank-d` (`58080`).

Each entity runs its own API Gateway backed by its own Keycloak realm, PostgreSQL schema, and Redis database. The tryout scripts simulate real operator interactions crossing entity boundaries (bank → central bank and back), which is the most complex class of flows in the system.

---

## Scripts

### Bank Onboarding Tryouts

All four onboarding scripts share the same **dual-entity 3-phase PKI + Blockchain flow** (7 steps). The entity-specific parameters (URLs, env files, client IDs) are the only things that differ between them.

| Script | Bank | Central Bank | Bank Port | CB Port |
|---|---|---|---|---|
| `tryout-spoke-a-bank-a.sh` | bank-a | central-bank-a | 18080 | 38080 |
| `tryout-spoke-a-bank-c.sh` | bank-c | central-bank-a | 48080 | 38080 |
| `tryout-spoke-b-bank-b.sh` | bank-b | central-bank-b | 28080 | 60080 |
| `tryout-spoke-b-bank-d.sh` | bank-d | central-bank-b | 58080 | 60080 |

#### Onboarding Flow (7 Steps)

```
Step 1  Bank operator login          → Keycloak (bank realm)        → operator access token
Step 2  Credential Request (Phase 1) → Bank proxy → CB public API   → request_id, user_id, wallet_address
Step 3  CB governance login          → Keycloak (CB realm)          → governance access token
Step 4  Approve KYC (Phase 2)        → CB API /onboarding/approve   → KYC approved, pop_nonce issued
Step 5  Polling (bank side)          → Bank proxy /onboarding/status → discovers pop_nonce
Step 6  Complete Onboarding (Phase 3)→ Bank proxy (PoP + wallet bind)→ blockchain TX hash, Keycloak user provisioned
Step 7  PKI Login (validation)       → CB API /auth/pki-login        → verifies certificate-based auth works
```

**Actors simulated within the same script:**
- **Bank operator** — authenticates via the bank's own Keycloak and calls the bank's API Gateway, which proxies sensitive requests to the central bank.
- **CB governance** — authenticates via the central bank's Keycloak and performs KYC approval through the CB's API Gateway directly.

This dual-actor simulation means a single script exercises both sides of the trust boundary, making it easy to reproduce the full handshake without requiring two separate terminal sessions or human coordination.

**Key security mechanisms exercised:**
- **Proof-of-Possession (PoP)**: the bank proves ownership of its private key during Phase 3.
- **Wallet binding**: an Ethereum wallet address is cryptographically linked to the bank's identity on-chain.
- **PKI login**: the final validation step authenticates with an X.509 certificate, confirming the on-chain identity is usable for subsequent authenticated calls.

---

### Compliance Tryout

**`tryout-compliance-participants.sh`** — Exercises the `GET /compliance/participants` endpoint under four scenarios to validate role-based access control (RBAC) and filter behaviour:

| Scenario | Actor | Expected HTTP |
|---|---|---|
| 1 | bank-a (`ROLE_COMMERCIAL_BANK`) | `403 Forbidden` |
| 2 | central-bank-a (`ROLE_GOVERNANCE`) | `200` + participant list |
| 3 | central-bank-a with invalid status filter | `200` + empty list |
| 4 | central-bank-a with valid status filter | `200` + filtered list |

This script is **assertion-based**: each scenario prints `PASS` or `FAIL` with the actual and expected HTTP status code, and a summary at the end counts total passes and failures.

**Prerequisite**: at least one participant must already be registered (i.e. run one of the onboarding tryouts first).

---

## Prerequisites

For all scripts:

- Local stack is running: `make dev.up` (or the individual `dev.up-<entity>` targets).
- `curl` and `jq` are installed and on `$PATH`.
- The entity `env` files exist at `backend/config/.env.infra.<entity>` and contain a valid `KC_CLIENT_SECRET`.
- PKI material is present at `backend/config/pki/` (generated by `make pki.generate`).

---

## Environment Variable Overrides

Every script exposes environment variables to override defaults, enabling them to run against non-standard deployments (staging, custom ports, etc.):

| Variable | Purpose | Default example |
|---|---|---|
| `BANK_URL` | Bank API Gateway base URL | `http://localhost:18080/api/v1` |
| `CB_URL` | Central Bank API Gateway base URL | `http://localhost:38080/api/v1` |
| `BANK_ENV` | Path to the bank's `.env.infra` file | `backend/config/.env.infra.bank-a` |
| `CB_ENV` | Path to the CB's `.env.infra` file | `backend/config/.env.infra.central-bank-a` |
| `PKI_DIR` | Directory containing PKI certificates/keys | `backend/config/pki` |

Example override:

```bash
BANK_URL=http://staging-bank-a:18080/api/v1 CB_URL=http://staging-cb:38080/api/v1 ./tryout-spoke-a-bank-a.sh
```

---

## Adding New Tryouts

When implementing a new business flow, follow these conventions:

1. **Name the script** after the flow and the entities involved: `tryout-<flow>-<entity>.sh`.
2. **Open with a comment block** describing actors, numbered steps, prerequisites, usage, and overridable environment variables — this is the primary documentation surface.
3. **Use `set -euo pipefail`** so the script aborts on any unhandled error or unset variable.
4. **Isolate each step in a function** named after its intent (`bank_operator_login`, `submit_credential_request`, etc.) and document its actor and purpose in a comment header above the function.
5. **Print progress lines** to stdout so the operator can follow execution without grepping logs.
6. For assertion-based tryouts (like the compliance one), **track `PASS`/`FAIL` counters** and print a summary at the end rather than exiting on the first failure — this gives a more complete picture of regressions.
