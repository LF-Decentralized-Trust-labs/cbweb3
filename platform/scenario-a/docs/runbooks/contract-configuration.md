# Contract Configuration Guide — CBWeb3 Platform

> **Deliverable 10** · Smart contract parameterization, on-chain permissions, and Keycloak

This guide covers Solidity contract deployment parameters, the on-chain role model, Keycloak permissions per entity, and PKI certificates. For service environment variables, see [`configuration-reference.md`](configuration-reference.md).

---

## Table of Contents

- [Contract deployment parameters](#contract-deployment-parameters)
  - [Scenario A — spoke-a and spoke-b](#scenario-a--spoke-a-and-spoke-b)
  - [Scenario B — hub](#scenario-b--hub)
- [On-chain roles and permissions](#on-chain-roles-and-permissions)
  - [IdentityRegistry (on-chain RBAC)](#identityregistry-on-chain-rbac)
  - [TokenizedCentralBankMoney (tCeBM)](#tokenizedcentralbankmoney-tcebm)
  - [HashTimeLockedContract (HTLC)](#hashtimelockedcontract-htlc)
  - [SpokeBridge](#spokebridge)
- [Keycloak permissions (OIDC)](#keycloak-permissions-oidc)
  - [Realms and clients](#realms-and-clients)
  - [Platform roles](#platform-roles)
  - [Service account roles](#service-account-roles)
- [PKI — X.509 Certificates](#pki--x509-certificates)
- [Post-deployment configuration checklist](#post-deployment-configuration-checklist)
- [Scenario B — contract configuration](#scenario-b--contract-configuration)

---

## Contract deployment parameters

### Scenario A — spoke-a and spoke-b

Contracts are deployed via Foundry scripts. Environment parameters must be in `scenario-a/contracts/.env`.

#### Required variables before deployment

| Variable | Type | Description |
|----------|------|-------------|
| `DEPLOYER_PRIVATE_KEY` | hex ⚠️ | Deployer private key (Besu genesis account in local dev) |
| `ADMIN_PRIVATE_KEY` | hex ⚠️ | IdentityRegistry admin key |
| `ADMIN_ADDRESS` | address | Ethereum admin address (receives `GOVERNANCE_ROLE` and `DEFAULT_ADMIN_ROLE`) |
| `CENTRAL_BANK_ADDRESS` | address | Ethereum central bank address (receives `CENTRAL_BANK_ROLE` on tCeBM) |
| `SPOKE_A_RPC_URL` | URL | Spoke-A RPC — default: `http://127.0.0.1:8645` |
| `SPOKE_B_RPC_URL` | URL | Spoke-B RPC — default: `http://127.0.0.1:8745` |

> **Note:** In local dev, use Besu genesis accounts. In staging/production, use dedicated accounts managed by HSM or secret manager.

#### tCeBM token parameters (hardcoded in the Makefile per spoke)

| Parameter | Spoke-A | Spoke-B | Description |
|-----------|---------|---------|-------------|
| `TOKEN_NAME` | `"Tokenized BRL"` | `"Tokenized BRL"` | ERC-20 token name |
| `TOKEN_SYMBOL` | `"tCeBM_BRL"` | `"tCeBM_BRL"` | Token symbol |
| `FIAT_TOKEN_NAME` | `"Fiat BRL"` | `"Fiat BRL"` | Collateralized fiat token name |
| `FIAT_TOKEN_SYMBOL` | `"fCeBM_BRL"` | `"fCeBM_BRL"` | Fiat symbol |

To use different currencies (e.g. BRL/ARS for spoke-a/spoke-b), edit the values in `scenario-a/make/30-contracts.mk`:

```makefile
contracts.deploy-spoke-a:
    @cd contracts && TOKEN_NAME="Tokenized BRL" TOKEN_SYMBOL="tCeBM_BRL" \
        FIAT_TOKEN_NAME="Fiat BRL" FIAT_TOKEN_SYMBOL="fCeBM_BRL" \
        CENTRAL_BANK_ADDRESS=$(CENTRAL_BANK_ADDRESS) \
        FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke \
        --rpc-url ${SPOKE_A_RPC_URL} --broadcast
```

#### Contracts deployed by `make contracts.deploy-spoke-{a,b}`

| Contract | Constructor args | Description |
|---------|-----------------|-------------|
| `TokenizedCentralBankMoney` | `name, symbol, admin, centralBank` | ERC-20 CBDC token |
| `FiatCentralBankMoney` | `name, symbol, admin, centralBank` | Collateralized fiat token |
| `HashTimeLockedContract` | `identityRegistry` | Cross-spoke escrow |
| `IdentityRegistry` | `admin` | On-chain participant registry |
| `SpokeBridge` | `identityRegistry` | Cross-spoke bridge |
| `AutomatedMarketMaker` | `tokenA, tokenB, identityRegistry` | AMM pool (used in Scenario B) |
| `FXAgreement` | `identityRegistry, oracle` | FX agreement lifecycle |

After deployment, run `make contracts.sync-addresses` to propagate addresses to the service `.env` files.

#### On-chain participant registration

```bash
make contracts.register-participants-spoke-a
make contracts.register-participants-spoke-b
```

The `RegisterParticipants.s.sol` script registers all entities in the IdentityRegistry with their respective roles. Uses `ADMIN_PRIVATE_KEY` for governance calls.

---

### Scenario B — hub

Implemented, and deployed from Scenario B's own contract tree by the `cbweb3b`
toolkit's `found-hub` mode — not by any script documented here. See
[`scenario-b/docs/runbooks/contract-configuration.md`](../../../scenario-b/docs/runbooks/contract-configuration.md).

---

## On-chain roles and permissions

### IdentityRegistry (on-chain RBAC)

The `IdentityRegistry` contract uses OpenZeppelin `AccessControl` with two Solidity roles:

| Role | Identifier | Assigned to | Permissions |
|------|-----------|------------|------------|
| `DEFAULT_ADMIN_ROLE` | `0x00` (bytes32) | `admin` (constructor) | Manage all roles |
| `GOVERNANCE_ROLE` | `keccak256("GOVERNANCE_ROLE")` | `admin` (constructor) | Register/update participants, set certificate fingerprints |

#### `ParticipantRole` enum (participant roles)

Registered participants receive one of the following functional roles:

| Role | Value | Assigned to | Capabilities |
|------|-------|------------|-------------|
| `NONE` | 0 | Unregistered | No permission to transact |
| `GOVERNANCE` | 1 | Network administrators | Transaction authorization + pause AMM |
| `CENTRAL_BANK` | 2 | Central banks | Transaction authorization + pause AMM |
| `COMMERCIAL_BANK` | 3 | Commercial banks | Transaction authorization |

> `canTransact()` returns `true` only for participants with `Verified` status. `canPause()` is reserved for `CENTRAL_BANK` and `GOVERNANCE`.

#### Participant status

| Status | Description |
|--------|-------------|
| `NONE` | Not registered |
| `Pending` | Registration initiated, awaiting verification |
| `Verified` | Authorized to transact |
| `Suspended` | Operations suspended by the central bank |

To check and update status via Foundry:

```bash
# Check participant
cast call $IDENTITY_REGISTRY_ADDRESS \
  "getParticipant(address)" $BANK_ADDRESS \
  --rpc-url http://localhost:8645

# Update status (requires GOVERNANCE_ROLE)
cast send $IDENTITY_REGISTRY_ADDRESS \
  "updateParticipantStatus(address,uint8)" $BANK_ADDRESS 2 \
  --private-key $ADMIN_PRIVATE_KEY \
  --rpc-url http://localhost:8645
```

---

### TokenizedCentralBankMoney (tCeBM)

tCeBM uses OpenZeppelin `AccessControl`:

| Role | Identifier | Capabilities |
|------|-----------|-------------|
| `DEFAULT_ADMIN_ROLE` | `0x00` | Manage roles |
| `CENTRAL_BANK_ROLE` | `keccak256("CENTRAL_BANK_ROLE")` | `mint()` and `burn()` |

Only the address passed as `centralBank` in the constructor receives `CENTRAL_BANK_ROLE`. No other entity can create or destroy supply.

---

### HashTimeLockedContract (HTLC)

The HTLC uses the `IdentityRegistry` as a gatekeeper — it has no roles of its own. Any operation (`lock`, `settle`, `refund`) requires the participant to be `Verified` in the registry.

**HTLC states (FSM):**

```
INVALID → LOCKED → SETTLED
                 → REFUNDED
```

**Lock parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `hashLock` | `bytes32` | SHA-256 of the secret preimage |
| `timeLock` | `uint256` | Unix expiry timestamp (minimum: `block.timestamp + 1h`) |
| `amount` | `uint256` | Amount locked in tCeBM |
| `tokenAddress` | `address` | tCeBM contract address |
| `recipient` | `address` | Recipient verified in the IdentityRegistry |

---

### SpokeBridge

SpokeBridge coordinates cross-spoke messages. Uses the IdentityRegistry to verify senders and recipients. No additional roles of its own.

---

## Keycloak permissions (OIDC)

### Realms and clients

The toolkit's Keycloak provisioning step automatically creates one realm and one client per entity when `cbweb3 apply` runs the `provision-keycloak` step:

| Entity | Realm | Client ID |
|--------|-------|-----------|
| bank-a | `bank-a` | `bank-a-client` |
| bank-b | `bank-b` | `bank-b-client` |
| bank-c | `bank-c` | `bank-c-client` |
| bank-d | `bank-d` | `bank-d-client` |
| central-bank-a | `central-bank-a` | `central-bank-a-client` |
| central-bank-b | `central-bank-b` | `central-bank-b-client` |
| NOC | `cbweb3` | `noc-portal` |

Each client is **confidential** with **service accounts enabled**. The generated client secret is automatically written to the entity's `.env.infra.<entity>` files.

### Platform roles

The following roles are created in each entity realm:

| Keycloak role | Used by portal | Description |
|--------------|---------------|-------------|
| `ROLE_COMMERCIAL_BANK` | Bank Portal | Commercial bank operator |
| `ROLE_TREASURY` | Treasury Portal | Treasury management |
| `ROLE_SUPERVISOR` | Supervisor Portal | Regulatory oversight |
| `ROLE_NOC` | NOC Dashboard | Network operator |
| `ROLE_GOVERNANCE_OFFICER` | Governance Portal | Governance officer |
| `ROLE_GOVERNANCE` | Governance Portal | Governance administrator |

### Service account roles

Each client's service account automatically receives `ROLE_GOVERNANCE` for authenticated backend operations (service-to-service calls).

To inspect assigned roles via kcadm:

```bash
# List roles for the bank-a realm
/opt/keycloak/bin/kcadm.sh get roles -r bank-a \
  --server http://localhost:8081 \
  --realm master \
  --user admin --password admin

# Inspect service account
/opt/keycloak/bin/kcadm.sh get clients -r bank-a \
  --fields clientId,serviceAccountsEnabled
```

### OIDC configuration (per realm)

Each realm is configured by init.sh with:

- **Access token lifespan:** 5 minutes (300s, Keycloak's own default; override with
  `KC_ACCESS_TOKEN_LIFESPAN`). The portals renew silently, so a short token costs no
  operator session — it was 6 hours until finding R1-10.7.
- **Protocol:** `openid-connect`
- **Enabled grant types:** `authorization_code`, `client_credentials`
- **Attribute mappers:** roles mapped to the JWT as the `roles` claim

To change the token TTL via the Keycloak API (keep it short — the lifespan is the window
in which a leaked bearer stays replayable, and `TestDeployLocalAccessTokenLifespanIsShort`
rejects anything above 900s on the scripted path):

```bash
/opt/keycloak/bin/kcadm.sh update realms/bank-a \
  -s accessTokenLifespan=900 \
  --server http://localhost:8081 \
  --realm master --user admin --password admin
```

---

## PKI — X.509 Certificates

Certificates are generated by `make pki.gen-all` in `scenario-a/backend/config/pki/`.

### Structure per entity

| File | Type | Algorithm | Validity |
|------|------|-----------|---------|
| `<entity>-ca.key` | CA private key | EC prime256v1 | — |
| `<entity>-ca.crt` | Root CA certificate | X.509 self-signed | 10 years |
| `<entity>.key` | Participant private key | EC prime256v1 | — |
| `<entity>.csr` | Certificate Signing Request | — | — |
| `<entity>.crt` | Participant certificate | X.509, signed by CA | 5 years |

### Subject DN per entity

| Entity | CN | O | OU |
|--------|----|----|-----|
| central-bank-a | `CBWeb3-CentralBankA-CA` | `CentralBankA` | — |
| bank-a | `bank-a` | `bank-a` | `ROLE_COMMERCIAL_BANK` |
| bank-b | `bank-b` | `bank-b` | `ROLE_COMMERCIAL_BANK` |

### PKI commands

```bash
# Generate all certificates (first time)
make pki.gen-all

# Check status
make pki.check
make pki.check-commercial-banks

# Regenerate all (overwrites)
make pki.gen-all FORCE=1

# Clean and regenerate
make pki.clean && make pki.gen-all
```

### Service integration

The `auth` service uses `CA_CERT_FILE` and `CA_KEY_FILE` (defined in `.env.infra.<entity>`) to:
- Sign participant certificates during onboarding
- Verify certificate fingerprints registered in the IdentityRegistry via `setCertFingerprint()`

---

## Post-deployment configuration checklist

Run after `cd samples && ./deploy-all.sh` to validate everything is correctly configured:

### Contracts

- [ ] `contracts/.env` has `DEPLOYER_PRIVATE_KEY`, `ADMIN_ADDRESS`, `CENTRAL_BANK_ADDRESS` set
- [ ] `make contracts.deploy-spoke-a` completed without errors
- [ ] `make contracts.deploy-spoke-b` completed without errors
- [ ] `make contracts.sync-addresses` propagated addresses to all `.env.infra.*`
- [ ] `PARTICIPANT_REGISTRY_ADDRESS` is not empty in `.env.infra.bank-a`
- [ ] `TOKEN_ADDRESS` is not empty in `.env.infra.bank-a`
- [ ] `make contracts.register-participants-spoke-a` registered all entities
- [ ] `make contracts.register-participants-spoke-b` registered all entities

### IdentityRegistry

- [ ] Central bank registered with `ParticipantRole.CENTRAL_BANK`
- [ ] Commercial banks registered with `ParticipantRole.COMMERCIAL_BANK`
- [ ] All participant statuses = `Verified` (required to transact)
- [ ] PKI certificate fingerprints set via `setCertFingerprint()`

### Keycloak

- [ ] Realm created for each entity (`bank-a`, `bank-b`, `central-bank-a`, etc.)
- [ ] Confidential client created with service accounts enabled
- [ ] `KC_CLIENT_SECRET` propagated to `.env.infra.<entity>` (done automatically)
- [ ] Platform roles created in each realm
- [ ] `cbweb3` realm for NOC configured via `make noc.setup-keycloak`
- [ ] Login working: `curl -s -X POST http://localhost:8081/realms/bank-a/protocol/openid-connect/token -d 'grant_type=client_credentials&client_id=bank-a-client&client_secret=<secret>'`

### PKI

- [ ] `make pki.check` shows `[OK]` for all files
- [ ] `make pki.check-commercial-banks` shows `[OK]` for all commercial banks
- [ ] `CA_CERT_FILE` and `CA_KEY_FILE` point to existing files in the `.env.infra.*`

---

## Scenario B — contract configuration

Scenario B is implemented. Its AMM, oracle, registries and `SpokeBridge` live in
[`scenario-b/contracts/src/`](../../../scenario-b/contracts/src/) — a tree
independent of Scenario A's, deliberately not shared (scenario isolation). Their
configuration is documented with them, not here:

- [`scenario-b/docs/runbooks/contract-configuration.md`](../../../scenario-b/docs/runbooks/contract-configuration.md) — AMM, `ManualOracle`, `PairRegistry`, `LiquidityCommitRegistry`, `CurrencyRegistry`, `SpokeBridge`
- [`scenario-b/README.md`](../../../scenario-b/README.md) — deployment and the circuit-breaker model (1-of-N pause, 2-of-N resume)

Nothing in this guide configures a Scenario B contract, and the `AutomatedMarketMaker`
row in the constructor table above refers to Scenario A's own copy.
