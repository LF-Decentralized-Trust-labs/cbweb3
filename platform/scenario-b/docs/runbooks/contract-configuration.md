# Contract Configuration Guide — CBWeb3 Platform · Scenario B

> **Project:** RG-T4567 · Suboperation ATN/KS-21330-RG
> **Authors:** Lucas Campelo, Samuel Venzi
> **Date:** 2026-05-29
>
> **Deliverable 10** · Smart contract parameterization, on-chain permissions, and Keycloak

> **Status: Work in progress.** Scenario B is implemented but not 100% complete. Hub contracts (AMM, PairRegistry, LiquidityCommitRegistry) are deployed and functional. The commercial bank swap flow (US3) is partially implemented. Sovereign liquidity auto-recovery and Paladin/Zeto on the hub are not yet implemented.

This guide covers Solidity contract deployment parameters, hub and spoke contract inventories, on-chain role models, PairRegistry bilateral approval, AMM circuit breaker, LiquidityCommitRegistry commit TTL, Keycloak realms, PKI, and the post-deployment checklist. For service environment variables, see [`configuration-reference.md`](configuration-reference.md).

---

## Table of Contents

- [Required variables before deployment](#required-variables-before-deployment)
- [Contracts deployed — hub network](#contracts-deployed--hub-network)
- [Contracts deployed — spokes](#contracts-deployed--spokes)
- [On-chain roles](#on-chain-roles)
  - [Hub IdentityRegistry](#hub-identityregistry)
  - [Spoke IdentityRegistry](#spoke-identityregistry)
  - [TokenizedCentralBankMoney (tCeBM)](#tokenizedcentralbankmoney-tcebm)
  - [HashTimeLockedContract (HTLC)](#hashtimelockedcontract-htlc)
- [PairRegistry — bilateral pair approval flow](#pairregistry--bilateral-pair-approval-flow)
- [AMM circuit breaker (pause / resume quorum)](#amm-circuit-breaker-pause--resume-quorum)
- [LiquidityCommitRegistry — commit TTL](#liquiditycommitregistry--commit-ttl)
- [Keycloak — realms and permissions](#keycloak--realms-and-permissions)
- [PKI — X.509 certificates](#pki--x509-certificates)
- [Post-deployment checklist](#post-deployment-checklist)

---

## Required variables before deployment

Set these variables in `scenario-b/contracts/.env` before running `make scenario-b.deploy-contracts`:

| Variable | Type | Description |
|----------|------|-------------|
| `HUB_RPC_URL` | URL **required** | Hub Besu RPC endpoint. In local dev: `http://127.0.0.1:8645` (same as Spoke-A) |
| `SPOKE_A_RPC_URL` | URL **required** | Spoke-A RPC endpoint. Local dev: `http://127.0.0.1:8645` |
| `SPOKE_B_RPC_URL` | URL **required** | Spoke-B RPC endpoint. Local dev: `http://127.0.0.1:8745` |
| `DEPLOYER_PRIVATE_KEY` | hex secret **required** | Deployer account private key (Besu genesis account in local dev) |
| `ADMIN_PRIVATE_KEY` | hex secret **required** | Governance admin private key (receives `DEFAULT_ADMIN_ROLE` + `GOVERNANCE_ROLE`) |
| `ADMIN_ADDRESS` | address **required** | Ethereum address of the governance admin |
| `CENTRAL_BANK_ADDRESS` | address **required** | CB-A Ethereum address (receives hub governance + `CENTRAL_BANK_ROLE` on tCeBM-BRL) |
| `CENTRAL_BANK_B_ADDRESS` | address **required** | CB-B Ethereum address (receives hub governance + `CENTRAL_BANK_ROLE` on tCeBM-EUR) |
| `CENTRAL_BANK_PRIVATE_KEY` | hex secret **required** | CB-A private key (used for post-deployment role grants) |
| `CENTRAL_BANK_B_PRIVATE_KEY` | hex secret **required** | CB-B private key (used for post-deployment role grants) |

> In local development, use Besu genesis accounts. In staging or production, use dedicated accounts managed by an HSM or secret manager.

---

## Contracts deployed — hub network

All hub contracts are deployed to chain 1338 in local dev (same node as Spoke-A). The deployment script is invoked by `make scenario-b.deploy-contracts`.

| Contract | Purpose | Constructor authorization |
|---------|---------|--------------------------|
| `IdentityRegistry` (hub) | Participant registry for the hub; RBAC for all hub contracts | `GOVERNANCE_ROLE`: admin, CB-A, CB-B |
| `CurrencyRegistry` | Tracks currencies registered on the hub | `GOVERNANCE_ROLE` required to register |
| `PairRegistry` | Bilateral CB pair approval lifecycle | `GOVERNANCE_ROLE` required for `proposePair()` / `confirmPair()` |
| `TokenizedCentralBankMoney` (tCeBM-BRL) | ERC-20 hub BRL CBDC token | `CENTRAL_BANK_ROLE`: CB-A |
| `TokenizedCentralBankMoney` (tCeBM-EUR) | ERC-20 hub EUR CBDC token | `CENTRAL_BANK_ROLE`: CB-B |
| `ManualOracle` | FX rate oracle | `GOVERNANCE_ROLE` required for `setRate()` |
| `AutomatedMarketMaker` | Constant-product AMM; requires both tCeBM tokens + IdentityRegistry + ManualOracle | Pauseable by CENTRAL\_BANK or GOVERNANCE |
| `LiquidityCommitRegistry` | Commit-reveal for sovereign CB liquidity; 72-hour TTL per commit | `GOVERNANCE_ROLE` required for admin ops |
| `FXAgreement` (hub) | Hub-side FX agreement lifecycle management | Reads hub IdentityRegistry |
| `SpokeBridge` (spoke-a) | Lock&Mint / Burn&Unlock bridge connecting Spoke-A to the hub | Uses spoke-a IdentityRegistry |
| `SpokeBridge` (spoke-b) | Lock&Mint / Burn&Unlock bridge connecting Spoke-B to the hub | Uses spoke-b IdentityRegistry |

After deployment, all addresses are automatically written to the backend config files via `contracts.sync-addresses`. Do not set these addresses manually.

---

## Contracts deployed — spokes

Spoke contracts are re-deployed fresh in Scenario B (separate instances from Scenario A). Deployed to chain 1338 (Spoke-A) and 1339 (Spoke-B).

| Contract | Purpose |
|---------|---------|
| `IdentityRegistry` (per spoke) | Participant registry for spoke-local authorization |
| `TokenizedCentralBankMoney` (per spoke) | ERC-20 spoke CBDC token |
| `FiatCentralBankMoney` (per spoke) | Collateralized fiat token |
| `HashTimeLockedContract` (per spoke) | Atomic cross-spoke escrow (HTLC) |
| `SpokeBridge` (per spoke) | Connects the spoke to the hub (Lock&Mint / Burn&Unlock) |

---

## On-chain roles

### Hub IdentityRegistry

The hub `IdentityRegistry` uses OpenZeppelin `AccessControl` with both Solidity roles (for contract administration) and `ParticipantRole` enum entries (for functional access control).

#### Solidity roles

| Role | Assigned to | Permissions |
|------|------------|------------|
| `DEFAULT_ADMIN_ROLE` (`0x00`) | `admin` (constructor) | Manage all roles |
| `GOVERNANCE_ROLE` | `admin`, `central-bank-a`, `central-bank-b` | Register/update participants, set certificate fingerprints, manage PairRegistry, set oracle rates |

#### ParticipantRole enum (hub)

| Role | Value | Assigned to | Capabilities |
|------|-------|------------|-------------|
| `NONE` | 0 | Unregistered | No permission to transact on the hub |
| `GOVERNANCE` | 1 | Governance admin | Full hub administration |
| `CENTRAL_BANK` | 2 | central-bank-a, central-bank-b | AMM liquidity management, pair proposal/confirmation, circuit breaker |
| `COMMERCIAL_BANK` | 3 | bank-a, bank-b, bank-c, bank-d | Execute swaps, query pool status |
| `LIQUIDITY_PROVIDER` | 4 | mlp (optional) | Bilateral liquidity deposit on both sides of a pair |

#### setCentralBankOf mapping

The hub `IdentityRegistry` also stores a token→central bank mapping used by `PairRegistry` to enforce bilateral approval:

```bash
# Set CB-A as the central bank for tCeBM-BRL (required after deployment)
cast send $HUB_IDENTITY_REGISTRY_ADDRESS \
  "setCentralBankOf(address,address)" $HUB_TOKEN_A_ADDRESS $CENTRAL_BANK_ADDRESS \
  --private-key $ADMIN_PRIVATE_KEY --rpc-url http://localhost:8645

# Set CB-B as the central bank for tCeBM-EUR
cast send $HUB_IDENTITY_REGISTRY_ADDRESS \
  "setCentralBankOf(address,address)" $HUB_TOKEN_B_ADDRESS $CENTRAL_BANK_B_ADDRESS \
  --private-key $ADMIN_PRIVATE_KEY --rpc-url http://localhost:8645
```

> These calls are executed automatically by `make scenario-b.deploy-contracts`. They are documented here for manual recovery.

---

### Spoke IdentityRegistry

Spoke IdentityRegistries follow the same access control model as Scenario A:

| Role | Assigned to |
|------|------------|
| `DEFAULT_ADMIN_ROLE` | `admin` |
| `GOVERNANCE_ROLE` | `admin` |
| `ParticipantRole.CENTRAL_BANK` | spoke's central bank |
| `ParticipantRole.COMMERCIAL_BANK` | commercial banks on that spoke |

To check a participant's status:

```bash
cast call $SPOKE_A_IDENTITY_REGISTRY \
  "getParticipant(address)" $BANK_A_ADDRESS \
  --rpc-url http://localhost:8645
```

---

### TokenizedCentralBankMoney (tCeBM)

Each tCeBM instance (hub and spokes) uses OpenZeppelin `AccessControl`:

| Role | Capabilities |
|------|-------------|
| `DEFAULT_ADMIN_ROLE` (`0x00`) | Manage roles |
| `CENTRAL_BANK_ROLE` | `mint()` and `burn()` |

Only the central bank that issued the token receives `CENTRAL_BANK_ROLE`. No other entity can create or destroy supply.

- Hub tCeBM-BRL → `CENTRAL_BANK_ROLE`: `central-bank-a`
- Hub tCeBM-EUR → `CENTRAL_BANK_ROLE`: `central-bank-b`
- Spoke-A tCeBM → `CENTRAL_BANK_ROLE`: `central-bank-a`
- Spoke-B tCeBM → `CENTRAL_BANK_ROLE`: `central-bank-b`

---

### HashTimeLockedContract (HTLC)

The HTLC uses the spoke `IdentityRegistry` as a gatekeeper. It has no roles of its own. Operations (`lock`, `settle`, `refund`) require the participant to be `Verified` in the spoke registry.

**HTLC state machine:**

```
INVALID → LOCKED → SETTLED
                 → REFUNDED
```

**Lock parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `hashLock` | `bytes32` | SHA-256 of the secret preimage |
| `timeLock` | `uint256` | Unix expiry timestamp (minimum: `block.timestamp + 1 hour`) |
| `amount` | `uint256` | Amount locked in tCeBM |
| `tokenAddress` | `address` | tCeBM contract address |
| `recipient` | `address` | Recipient verified in the spoke IdentityRegistry |

---

## PairRegistry — bilateral pair approval flow

A new currency pair on the hub must be approved by both central banks via the `PairRegistry` contract. The flow enforces that:
- The proposing CB is the issuer of `tokenA` (verified via `getCentralBankOf(tokenA)`)
- The confirming CB is the issuer of `tokenB` (verified via `getCentralBankOf(tokenB)`)

**API flow:**

```
CB-A (BRL issuer)                    CB-B (EUR issuer)
POST /api/v2/amm/pairs/propose       POST /api/v2/amm/pairs/confirm
  pair_id    = "BRL-EUR"               pair_id    = "BRL-EUR"
  token_a    = tCeBM-BRL               (CB-B's signer verified on-chain)
  token_b    = tCeBM-EUR               → PairEntry status: PROPOSED → ACTIVE
  amm_addr   = 0x...
  → on-chain: proposePair()
  → local DB: PROPOSED
```

**API endpoints:**

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v2/amm/pairs/propose` | CB JWT | Propose a new currency pair (tokenA issuer only) |
| `POST` | `/api/v2/amm/pairs/confirm` | CB JWT | Confirm a proposed pair (tokenB issuer only) |
| `GET` | `/api/v2/amm/pairs` | Any JWT | List active pairs (includes lazy on-chain sync) |

**Error codes:**

| Code | Description |
|------|-------------|
| HTTP 409 `SAME_PROVIDER_BOTH_SIDES` | One CB attempted to both propose and confirm the same pair |
| HTTP 403 | Caller is not the registered issuer of the respective token |
| HTTP 404 | Pair ID not found (confirm called before propose) |

**Makefile tryout:**

```bash
SKIP_UP=1 bash tryouts/tryout-scenario-b-e2e.sh us5
```

---

## AMM circuit breaker (pause / resume quorum)

The `AutomatedMarketMaker` implements a circuit breaker that can be triggered by either central bank or the governance admin.

### Pause

Any participant with `CENTRAL_BANK` or `GOVERNANCE` role can pause the AMM immediately:

```bash
# Pause via CB-A governance account
cast send $AMM_CONTRACT_ADDRESS "pause()" \
  --private-key $CENTRAL_BANK_PRIVATE_KEY \
  --rpc-url http://localhost:8645
```

While paused, all `swap()` and `addLiquidity()` calls revert with `Pausable: paused`.

### Resume (2-of-2 quorum)

Resuming the AMM requires both registered central banks to sign. The flow uses a two-step proposal + quorum:

1. First CB calls `proposeResume()` — proposal recorded on-chain
2. Second CB calls `signResume()` — if quorum reached (2 signatures), AMM unpauses

```bash
# Step 1 — CB-A proposes resume
cast send $AMM_CONTRACT_ADDRESS "proposeResume()" \
  --private-key $CENTRAL_BANK_PRIVATE_KEY \
  --rpc-url http://localhost:8645

# Step 2 — CB-B signs (triggers unpausing if quorum met)
cast send $AMM_CONTRACT_ADDRESS "signResume()" \
  --private-key $CENTRAL_BANK_B_PRIVATE_KEY \
  --rpc-url http://localhost:8645
```

A single CB cannot unilaterally resume the AMM. Governance admin alone also cannot resume — both CBs must agree.

**Makefile tryout:**

```bash
SKIP_UP=1 bash tryouts/tryout-scenario-b-e2e.sh us4
```

---

## LiquidityCommitRegistry — commit TTL

The `LiquidityCommitRegistry` implements the commit-reveal protocol for cooperative sovereign liquidity:

- **Phase 1 (commit):** CB-A posts a one-sided commit (`side = "A"`, `amount`, `pool_pair`). The commit is stored with status `PENDING` and an expiry of `created_at + 72 hours`. No funds move.
- **Phase 2 (match + reveal):** CB-B posts a matching commit (`side = "B"`). The backend auto-matches the two commits, calls `addSingleSided(A)` and `addSingleSided(B)` on the AMM, and transitions both commits to `EXECUTED`. The pool transitions to `ACTIVE`.

**Commit states:**

| Status | Description |
|--------|-------------|
| `PENDING` | Commit submitted; waiting for counterpart |
| `MATCHED` | Counterpart found; on-chain reveal in progress |
| `EXECUTED` | Both sides deposited; pool is ACTIVE |
| `EXPIRED` | 72-hour TTL elapsed without a counterpart; pool returns to `EMPTY` |

**Enforced constraints:**

| Error | Condition |
|-------|-----------|
| `SAME_PROVIDER_BOTH_SIDES` (HTTP 409) | One CB attempted to commit both sides of the same pair |
| `POOL_NOT_ACTIVE` | Swap attempted before both sides are committed and matched |

**API endpoint:**

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v2/amm/liquidity/commit` | CB or MLP JWT | Submit a one-sided liquidity intent |
| `GET` | `/api/v2/amm/pool/{pair}/status` | Any JWT | Pool reserves, status, and fee rate |
| `GET` | `/api/v2/amm/liquidity/positions` | Auth required | List LP positions for the caller |
| `POST` | `/api/v2/amm/liquidity/remove` | CB or MLP JWT | Remove LP position (proportional) |

**LP fee distribution:** Each swap distributes 30 bps proportionally to all active LP positions at the time of execution. Accumulated fees are recorded in `liquidity_positions.fee_claim_accumulated` and paid out on withdrawal.

---

## Keycloak — realms and permissions

Scenario B uses 7 realms (6 entity realms + 1 MLP realm). The Keycloak `init.sh` script creates all realms, clients, and writes secrets automatically during `make scenario-b.up-infra`.

### Realms and clients

| Entity | Realm | Client ID |
|--------|-------|-----------|
| bank-a | `bank-a` | `bank-a-client` |
| bank-b | `bank-b` | `bank-b-client` |
| bank-c | `bank-c` | `bank-c-client` |
| bank-d | `bank-d` | `bank-d-client` |
| central-bank-a | `central-bank-a` | `central-bank-a-client` |
| central-bank-b | `central-bank-b` | `central-bank-b-client` |
| mlp | `mlp` | `mlp-client` |

Each client is **confidential** with **service accounts enabled**. The generated client secret is written automatically to `backend/config/.env.infra.<entity>`.

### Platform roles (per realm)

| Keycloak role | Used by | Description |
|--------------|---------|-------------|
| `ROLE_COMMERCIAL_BANK` | Bank Portal | Commercial bank operator |
| `ROLE_TREASURY` | Treasury Portal | Treasury management |
| `ROLE_SUPERVISOR` | Supervisor Portal | Regulatory oversight |
| `ROLE_GOVERNANCE_OFFICER` | Governance Portal | Governance officer |
| `ROLE_GOVERNANCE` | Governance Portal + service accounts | Governance admin; also granted to service accounts |

### Service account roles

Each client's service account automatically receives `ROLE_GOVERNANCE` for authenticated backend service-to-service calls.

### OIDC configuration

Each realm is configured with:

- **Access token lifespan:** 1 hour
- **Protocol:** `openid-connect`
- **Enabled grant types:** `authorization_code`, `client_credentials`
- **Attribute mappers:** roles mapped to the JWT as the `roles` claim

To verify login for an entity:

```bash
curl -s -X POST \
  http://localhost:8081/realms/central-bank-a/protocol/openid-connect/token \
  -d "grant_type=client_credentials&client_id=central-bank-a-client&client_secret=<secret>" \
  | jq .access_token
```

---

## PKI — X.509 certificates

Certificates are generated by `make scenario-b.prepare-pki` in `scenario-b/backend/config/pki/`.

### Structure per entity

| File | Type | Algorithm | Validity |
|------|------|-----------|---------|
| `<entity>-ca.key` | CA private key | EC prime256v1 | — |
| `<entity>-ca.crt` | Root CA certificate | X.509 self-signed | 10 years |
| `<entity>.key` | Participant private key | EC prime256v1 | — |
| `<entity>.csr` | Certificate Signing Request | — | — |
| `<entity>.crt` | Participant certificate (signed by CA) | X.509 | 5 years |

### Subject DN

| Entity | CN | O | OU |
|--------|----|----|-----|
| central-bank-a | `CBWeb3-CentralBankA-CA` | `CentralBankA` | — |
| central-bank-b | `CBWeb3-CentralBankB-CA` | `CentralBankB` | — |
| bank-a | `bank-a` | `bank-a` | `ROLE_COMMERCIAL_BANK` |
| bank-b | `bank-b` | `bank-b` | `ROLE_COMMERCIAL_BANK` |
| bank-c | `bank-c` | `bank-c` | `ROLE_COMMERCIAL_BANK` |
| bank-d | `bank-d` | `bank-d` | `ROLE_COMMERCIAL_BANK` |
| mlp | `mlp` | `mlp` | `ROLE_LIQUIDITY_PROVIDER` |

### PKI commands

```bash
# Generate all certificates (first time or idempotent re-run)
make scenario-b.prepare-pki

# Force regeneration (overwrites existing files)
make scenario-b.prepare-pki FORCE=1
```

The `auth` service uses `CA_CERT_FILE` and `CA_KEY_FILE` to sign participant certificates during onboarding and to verify certificate fingerprints registered in the spoke `IdentityRegistry` via `setCertFingerprint()`.

---

## Post-deployment checklist

Run after `make scenario-b.up` to validate that all components are correctly configured.

### Contracts

- [ ] `contracts/.env` has all required variables set (`HUB_RPC_URL`, `DEPLOYER_PRIVATE_KEY`, `ADMIN_ADDRESS`, `CENTRAL_BANK_ADDRESS`, `CENTRAL_BANK_B_ADDRESS`)
- [ ] `make scenario-b.deploy-contracts` completed without errors
- [ ] `AMM_CONTRACT_ADDRESS` is not empty in `backend/config/.env.infra.central-bank-a`
- [ ] `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` is not empty in `backend/config/.env.infra.central-bank-a`
- [ ] `SPOKE_BRIDGE_ADDRESS` is set for both spokes
- [ ] `setCentralBankOf(tCeBM-BRL, CB-A)` confirmed on-chain (run by deploy script)
- [ ] `setCentralBankOf(tCeBM-EUR, CB-B)` confirmed on-chain (run by deploy script)
- [ ] Commercial banks registered in hub `IdentityRegistry` with `COMMERCIAL_BANK` role

### Hub IdentityRegistry roles

- [ ] `admin` has `GOVERNANCE_ROLE` + `DEFAULT_ADMIN_ROLE`
- [ ] `central-bank-a` has `GOVERNANCE_ROLE` + `ParticipantRole.CENTRAL_BANK` on hub
- [ ] `central-bank-b` has `GOVERNANCE_ROLE` + `ParticipantRole.CENTRAL_BANK` on hub
- [ ] `bank-a`, `bank-b`, `bank-c`, `bank-d` have `ParticipantRole.COMMERCIAL_BANK` on hub
- [ ] `mlp` has `ParticipantRole.LIQUIDITY_PROVIDER` on hub (if MLP enabled)
- [ ] All participants in `Verified` status (required to transact)

### Spoke contracts

- [ ] Spoke-A IdentityRegistry has CB-A and Bank-A, Bank-C registered and `Verified`
- [ ] Spoke-B IdentityRegistry has CB-B and Bank-B, Bank-D registered and `Verified`
- [ ] HTLC contracts deployed on both spokes
- [ ] SpokeBridge contracts deployed on both spokes and connected to hub

### PairRegistry

- [ ] CB-A has proposed `BRL-EUR` pair via `POST /api/v2/amm/pairs/propose`
- [ ] CB-B has confirmed `BRL-EUR` pair via `POST /api/v2/amm/pairs/confirm`
- [ ] `GET /api/v2/amm/pairs` returns `BRL-EUR` with status `ACTIVE`

### Liquidity

- [ ] CB-A has committed BRL liquidity (`POST /api/v2/amm/liquidity/commit` with `side: "A"`)
- [ ] CB-B has committed EUR liquidity (`POST /api/v2/amm/liquidity/commit` with `side: "B"`)
- [ ] Pool `BRL-EUR` status is `ACTIVE` (`GET /api/v2/amm/pool/BRL-EUR/status`)

### Keycloak

- [ ] All 7 realms created and accessible at `http://localhost:8081`
- [ ] `central-bank-a-client` token endpoint responds correctly:
  ```bash
  curl -s -X POST http://localhost:8081/realms/central-bank-a/protocol/openid-connect/token \
    -d "grant_type=client_credentials&client_id=central-bank-a-client&client_secret=<secret>" \
    | jq .access_token
  ```
- [ ] `mlp-client` realm accessible if MLP stack is enabled

### PKI

- [ ] `make scenario-b.prepare-pki` shows no errors
- [ ] `CA_CERT_FILE` and `CA_KEY_FILE` point to existing files in all `.env.infra.*` files

### Health checks

- [ ] `curl http://localhost:18080/healthz` → HTTP 200 (bank-a)
- [ ] `curl http://localhost:28080/healthz` → HTTP 200 (bank-b)
- [ ] `curl http://localhost:38080/healthz` → HTTP 200 (central-bank-a)
- [ ] `curl http://localhost:48080/healthz` → HTTP 200 (bank-c)
- [ ] `curl http://localhost:58080/healthz` → HTTP 200 (bank-d)
- [ ] `curl http://localhost:60080/healthz` → HTTP 200 (central-bank-b)
- [ ] `curl http://localhost:68080/healthz` → HTTP 200 (mlp, if enabled)
- [ ] `curl http://localhost:4000/api/v1/health` → HTTP 200 (Cacti relay)
