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

### FX Agreement End-to-End Tryout

**`tryout-fx-agreement-e2e.sh`** — Complete validation of feature 001-harden-fx-agreement (FX Agreement hardening for production). Tests the full lifecycle from onboarding through HTLC settlement, exercising all five design decisions:

1. **Hybrid Besu/Pente Architecture** — FX Agreement stored in private Pente context, with commitment validation on public Besu chain
2. **Two-Phase Enforcement** — Phase A: CommitmentHashRegistry for immediate deployment; Phase B: Pente externalCalls (future)
3. **Idempotent Relay** — Cross-spoke deduplication and persistent tracking with exponential backoff retry
4. **Service-Layer Gate** — HTLC validation at application level before on-chain attempt (fail-closed)
5. **Append-Only Audit Trail** — Complete event history for each FX Agreement with actor, timestamp, state changes

#### Test Flow (12 Steps)

```
Step 1  [Optional] Bank-A onboarding          → Keycloak + PKI phases
Step 2  [Optional] Bank-B onboarding          → Keycloak + PKI phases
Step 3  [Optional] Central Bank governance approvals → KYC approval
Step 4  Get Bank-A & Bank-B operator tokens  → Keycloak authentication
Step 5  Bank-A proposes FX agreement          → POST /fx/agreements {tradeId, originator, counterparty, amounts, rate}
Step 6  Validate persistence                  → GET /fx/agreements/{trade_id} → state=PROPOSED
Step 7  Validate audit trail                  → GET /fx/agreements/{trade_id}/events → append-only events
Step 8  Bank-B accepts FX agreement           → POST /fx/agreements/{trade_id}/accept (cross-spoke)
Step 9  Validate cross-spoke synchronization  → Confirm acceptance propagated via relay within timeout
Step 10 Lock HTLC with commitment             → POST /htlc/lock-with-hash (CommitmentHashRegistry validation)
Step 11 Settle HTLC                           → POST /htlc/settle {contract_id, secret}
Step 12 Verify automatic expiration           → Create stale proposal → background worker expires it
```

**Key validations:**
- **Durability (SC-001)**: FX Agreement state persists after service restart
- **Auditability (SC-002)**: Every state transition recorded with actor, timestamp, change details
- **Relay reliability (SC-003)**: Cross-spoke events delivered in ≤2 minutes with idempotent deduplication
- **Expiration (SC-007)**: Stale proposals automatically marked EXPIRED by background job
- **On-chain enforcement (SC-006)**: CommitmentHashRegistry hash present and verified before HTLC lock
- **Private bilateral context (SC-010)**: Pente GroupID + ContractAddress persisted (optional if --no-pente)

**Actors simulated within the same script:**
- **Bank-A operator** — initiates FX proposal and monitors acceptance
- **Bank-B operator** — accepts the FX agreement on counterparty side
- **Cacti relay daemon** — automatically propagates acceptance across spokes

#### Usage

```bash
# Full end-to-end test with onboarding:
./tryout-fx-agreement-e2e.sh

# Skip onboarding (assuming banks already onboarded):
./tryout-fx-agreement-e2e.sh --skip-onboarding

# Run without Pente (CommitmentHashRegistry only):
./tryout-fx-agreement-e2e.sh --skip-onboarding --no-pente

# Enable verbose output for debugging:
./tryout-fx-agreement-e2e.sh --skip-onboarding --verbose
```

#### Environment Variables

| Variable | Purpose | Default |
|---|---|---|
| `BANK_A_URL` | Bank-A API Gateway base URL | `http://localhost:18080/api/v1` |
| `BANK_B_URL` | Bank-B API Gateway base URL | `http://localhost:28080/api/v1` |
| `CB_A_URL` | Central Bank-A API Gateway base URL | `http://localhost:38080/api/v1` |
| `CB_B_URL` | Central Bank-B API Gateway base URL | `http://localhost:60080/api/v1` |
| `PORCH_URL` | Paladin Porch HTTP endpoint | `http://localhost:6969/api/v1` |
| `BANK_A_ENV` | Path to bank-a's `.env.infra` file | `backend/config/.env.infra.bank-a` |
| `BANK_B_ENV` | Path to bank-b's `.env.infra` file | `backend/config/.env.infra.bank-b` |
| `CB_A_ENV` | Path to central-bank-a's `.env.infra` file | `backend/config/.env.infra.central-bank-a` |
| `CB_B_ENV` | Path to central-bank-b's `.env.infra` file | `backend/config/.env.infra.central-bank-b` |
| `RELAY_SYNC_TIMEOUT` | Timeout for cross-spoke relay sync (seconds) | `30` |
| `HTTP_TIMEOUT` | HTTP request timeout (seconds) | `120` |
| `SKIP_ONBOARDING` | Skip onboarding phase (boolean) | `false` |
| `NO_PENTE` | Disable Pente validation (CommitmentHashRegistry only) | `false` |
| `VERBOSE` | Enable debug output | `false` |

#### Expected Output

```
════════════════════════════════════════════════════════════
  STEP: Step 5: Propose FX Agreement
════════════════════════════════════════════════════════════
✅ FX Agreement proposed: TRADE-1713180846123

════════════════════════════════════════════════════════════
  STEP: Step 6: Validate Persistence
════════════════════════════════════════════════════════════
✅ Agreement persisted with state: PROPOSED

════════════════════════════════════════════════════════════
  STEP: Step 7: Validate Audit Trail
════════════════════════════════════════════════════════════
✅ Audit trail contains 1 events

════════════════════════════════════════════════════════════
  STEP: Step 9: Validate Cross-Spoke Synchronization
════════════════════════════════════════════════════════════
✅ Cross-spoke synchronization complete

════════════════════════════════════════════════════════════
  STEP: SUMMARY
════════════════════════════════════════════════════════════
✅ End-to-End FX Agreement + HTLC Test Completed Successfully ✅

Key Results:
  Trade ID:        TRADE-1713180846123
  Commitment Hash: 0x4d967...
  Contract ID:     0xabcd...

All validation steps passed:
  ✅ FX Agreement proposal
  ✅ Persistent storage
  ✅ Audit trail
  ✅ Cross-spoke relay sync
  ✅ HTLC locking with commitment
  ✅ HTLC settlement
  ✅ Expiration handling
```

**Prerequisites for this tryout:**
- All four banks onboarded (or use `--skip-onboarding` if already done)
- Cacti relay running (daemon) for cross-spoke event propagation
- PostgreSQL with FX Agreement tables created (migrations applied)
- Besu nodes running with CommitmentHashRegistry deployed

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
