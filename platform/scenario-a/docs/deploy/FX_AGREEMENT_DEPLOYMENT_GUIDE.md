# FX Agreement Deployment Guide

> **Stale bring-up commands below.** This guide still instructs `make deploy.up-backend*`
> and `make deploy.down-backend`, which were removed with the legacy `deploy/local` path.
> A stack is stood up with the toolkit — `cd scenario-a/samples && ./deploy-all.sh` — which
> provisions and starts each entity's backend as a step of `apply`. The FX-agreement steps
> that follow are still correct; only the container lifecycle commands are not.

## Overview

This guide covers deploying FX Agreement infrastructure across both Besu spokes,
optionally with Pente (Paladin private EVM) for bilateral private contexts.

## Prerequisites

- ✅ Foundry (forge) installed
- ✅ Both Besu networks running (Spoke A & B)
- ✅ Paladin sidecars running (if enabling Pente integration)
- ✅ Deployer account funded on both networks
- ✅ Environment variables configured

## Environment Setup

```bash
# Core deployment vars — use the host-mapped Besu RPC ports
export DEPLOYER_PRIVATE_KEY=0x...
export BESU_RPC_URL=http://localhost:8645          # Spoke A — central-bank-a node
export BESU_RPC_URL_SPOKE_B=http://localhost:8745  # Spoke B — central-bank-b node

# Pente/Paladin integration (optional — disabled by default)
export PENTE_ENABLED=true|false
export PENTE_BASE_URL=http://localhost:31648        # Paladin node (CB-A: 31648, CB-B: 31748)

# Contract addresses (filled during deployment)
export IDENTITY_REGISTRY_ADDRESS=""
export COMMITMENT_HASH_REGISTRY_ADDRESS=""
export HTLC_ADDRESS=""
```

> **Note on Besu RPC ports:** Each Besu node has its own host-mapped port.
> See the [Port Reference](../../README.md#port-reference) for the full list.
> `BESU_RPC_URL` is the variable name used by all make targets and backend services.

## Deployment Steps

The preferred approach is to use the existing `make` targets, which handle
env loading, `forge script` invocation, and address syncing automatically.

### **Option A — Make targets (recommended)**

```bash
# Deploy all contracts to both spokes and sync addresses into .env files
make contracts.deploy-all-with-sync

# Or per-spoke:
BESU_RPC_URL=http://localhost:8645 make contracts.deploy-spoke-a
BESU_RPC_URL=http://localhost:8745 make contracts.deploy-spoke-b

# Register bank participants on-chain after deployment
make contracts.register-participants
```

Address sync writes the deployed addresses back into
`backend/config/.env.infra.<entity>` files automatically.

### **Option B — Manual forge scripts**

Use this only when deploying individual contracts out of the normal sequence.

#### Step 1: Deploy IdentityRegistry (Public Besu)

```bash
cd contracts

forge script script/IdentityRegistry.s.sol:DeployIdentityRegistry \
  --broadcast \
  --rpc-url $BESU_RPC_URL \
  --private-key $DEPLOYER_PRIVATE_KEY \
  -vvv

export IDENTITY_REGISTRY_ADDRESS=0x...  # save from output
```

**Verify:**
```bash
cast call $IDENTITY_REGISTRY_ADDRESS "GOVERNANCE_ROLE()" --rpc-url $BESU_RPC_URL
```

#### Step 2: Deploy CommitmentHashRegistry (Public Besu)

```bash
forge script script/CommitmentHashRegistry.s.sol \
  --broadcast \
  --rpc-url $BESU_RPC_URL \
  --private-key $DEPLOYER_PRIVATE_KEY \
  -vvv \
  --sig "run()" \
  --constructor-args $IDENTITY_REGISTRY_ADDRESS

export COMMITMENT_HASH_REGISTRY_ADDRESS=0x...  # save from output
```

**Verify:**
```bash
cast call $COMMITMENT_HASH_REGISTRY_ADDRESS "IDENTITY_REGISTRY()" --rpc-url $BESU_RPC_URL
# Should return: $IDENTITY_REGISTRY_ADDRESS
```

#### Step 3: Deploy HashTimeLockedContract (Public Besu)

```bash
forge script script/HashTimeLockedContract.s.sol:DeployHTLC \
  --broadcast \
  --rpc-url $BESU_RPC_URL \
  --private-key $DEPLOYER_PRIVATE_KEY \
  -vvv

export HTLC_ADDRESS=0x...  # save from output
```

The deployment script passes three constructor arguments:
1. `IDENTITY_REGISTRY_ADDRESS`
2. `FX_AGREEMENT_ADDRESS` (address(0) if not yet deployed)
3. `COMMITMENT_HASH_REGISTRY_ADDRESS`

**Verify:**
```bash
cast call $HTLC_ADDRESS "COMMITMENT_HASH_REGISTRY()" --rpc-url $BESU_RPC_URL
# Should return: $COMMITMENT_HASH_REGISTRY_ADDRESS
```

#### Step 4: Deploy to Spoke B (repeat Steps 1–3)

```bash
export BESU_RPC_URL=$BESU_RPC_URL_SPOKE_B

# Repeat steps 1–3, saving addresses with _SPOKE_B suffix:
export IDENTITY_REGISTRY_ADDRESS_SPOKE_B=0x...
export COMMITMENT_HASH_REGISTRY_ADDRESS_SPOKE_B=0x...
export HTLC_ADDRESS_SPOKE_B=0x...
```

### **Step 5: Configure Backend Services**

Contract addresses and connection strings live in per-entity env files at
`backend/config/.env.infra.<entity>`. Update the relevant entries:

```env
# Example: backend/config/.env.infra.central-bank-a
BESU_RPC_URL=http://cbweb3-spoke-a-besu.central-bank-a:8545   # container-internal URL
HTLC_ADDRESS=0x...                                             # from step 3
FX_AGREEMENT_ADDRESS=0x...                                     # if deployed
DATABASE_URL=postgres://default:default@cbweb3-postgres:5432/cbweb3_central_bank_a?sslmode=disable

# Pente (disabled by default)
PENTE_ENABLED=false
PENTE_BASE_URL=http://host.docker.internal:31648               # CB-A Paladin port
```

### **Step 6: Configure the Cacti Relay**

Update `interop/hub-and-spoke/cacti/env-sample` (or `.env`):

```env
SPOKE_A_BESU_RPC=http://host.docker.internal:8645
SPOKE_A_HTLC_ADDRESS=0x...

SPOKE_B_BESU_RPC=http://host.docker.internal:8745
SPOKE_B_HTLC_ADDRESS=0x...
SPOKE_B_INTERNAL_API=http://host.docker.internal:28080
SPOKE_B_PAYMENT_GRPC=host.docker.internal:29094
```

### **Step 7: Start Backend Services**

```bash
# Start all backends for both spokes
make deploy.up-backend

# Or per-spoke:
make deploy.up-backend-spoke-a
make deploy.up-backend-spoke-b
```

**Verify startup (example for Central Bank A, API gateway port 38080):**
```bash
curl http://localhost:38080/api/v1/health
# {"status":"healthy"}
```

See the [Port Reference](../../README.md#port-reference) for all entity ports.

### **Step 8: Register Initial Participants**

```bash
make contracts.register-participants-spoke-a
make contracts.register-participants-spoke-b

# Or manually via cast:
cast send $IDENTITY_REGISTRY_ADDRESS \
  "registerParticipant(address,string,uint8,bytes32)" \
  0x<bank_address> "Bank A" 2 0x0 \
  --rpc-url $BESU_RPC_URL \
  --private-key $DEPLOYER_PRIVATE_KEY
```

## Verification Checklist

### On-Chain Verification

```bash
# 1. IdentityRegistry deployed
cast call $IDENTITY_REGISTRY_ADDRESS "GOVERNANCE_ROLE()" --rpc-url $BESU_RPC_URL

# 2. CommitmentHashRegistry linked
cast call $COMMITMENT_HASH_REGISTRY_ADDRESS "IDENTITY_REGISTRY()" --rpc-url $BESU_RPC_URL
# Should return: $IDENTITY_REGISTRY_ADDRESS

# 3. HTLC linked
cast call $HTLC_ADDRESS "COMMITMENT_HASH_REGISTRY()" --rpc-url $BESU_RPC_URL
# Should return: $COMMITMENT_HASH_REGISTRY_ADDRESS

# 4. Test CommitmentHashRegistry gate
cast send $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "registerCommitment(bytes32,address,address,uint256,uint256,uint256)" \
  0x<trade_id> 0x<originator> 0x<counterparty> 1000 5500 5500000000000000 \
  --rpc-url $BESU_RPC_URL \
  --private-key $DEPLOYER_PRIVATE_KEY

cast send $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "acceptCommitment(bytes32)" \
  0x<commitment_hash> \
  --rpc-url $BESU_RPC_URL \
  --private-key $DEPLOYER_PRIVATE_KEY

cast call $COMMITMENT_HASH_REGISTRY_ADDRESS "isAccepted(bytes32)" 0x<commitment_hash> --rpc-url $BESU_RPC_URL
# Should return: true
```

### Backend Service Verification

```bash
# Propose FX agreement (Central Bank A, payment-orchestrator gRPC port 39094)
grpcurl -plaintext \
  -d '{"trade_id":"TEST-001","counterparty_b":"0x...","origin_amount":"1000000","counter_amount":"5500000","origin_currency":"BRL","counter_currency":"EUR","rate":"5500000000000000","expiry_date":"1713110400"}' \
  localhost:39094 \
  payment_orchestrator.v1.PaymentOrchestratorService/ProposeFXAgreement

# Accept FX agreement
grpcurl -plaintext \
  -d '{"trade_id":"TEST-001"}' \
  localhost:39094 \
  payment_orchestrator.v1.PaymentOrchestratorService/AcceptFXAgreement

# Check database
psql "postgres://default:default@localhost:5432/cbweb3_central_bank_a?sslmode=disable" \
  -c "SELECT trade_id, state FROM fx_agreements LIMIT 5;"
```

See the [Port Reference](../../README.md#port-reference) for gRPC ports of all entities.

### Pente Integration Verification (if enabled)

```bash
# Check service logs for Pente calls
docker logs backend-payment-orchestrator-central-bank-a | grep "Pente\|pente"
# Should show: "EnsureFXContext succeeded" when a bilateral context is established
```

## Testing Flow

### End-to-End Test

```bash
#!/bin/bash
TRADE_ID="TEST-$(date +%s)"
CB_A_GRPC="localhost:39094"
BESU_RPC="http://localhost:8645"

# 1. Propose agreement
grpcurl -plaintext \
  -d "{\"trade_id\":\"$TRADE_ID\",\"counterparty_b\":\"0x...\",...}" \
  $CB_A_GRPC \
  payment_orchestrator.v1.PaymentOrchestratorService/ProposeFXAgreement

# 2. Accept agreement
grpcurl -plaintext \
  -d "{\"trade_id\":\"$TRADE_ID\"}" \
  $CB_A_GRPC \
  payment_orchestrator.v1.PaymentOrchestratorService/AcceptFXAgreement

# 3. Register commitment on Besu (if Pente not available)
COMMITMENT_HASH=$(cast keccak $(echo -n "$TRADE_ID" | tr -d '\n'))
cast send $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "registerCommitment(bytes32,address,address,uint256,uint256,uint256)" \
  $COMMITMENT_HASH 0x<originator> 0x<counterparty> 1000 5500 5500000000000000 \
  --rpc-url $BESU_RPC --private-key $DEPLOYER_PRIVATE_KEY

# 4. Accept commitment
cast send $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "acceptCommitment(bytes32)" $COMMITMENT_HASH \
  --rpc-url $BESU_RPC --private-key $DEPLOYER_PRIVATE_KEY

# 5. Lock HTLC (should succeed now)
grpcurl -plaintext \
  -d "{\"contract_id\":\"HTL-$TRADE_ID\",\"agreement_id\":\"$TRADE_ID\",...}" \
  $CB_A_GRPC \
  payment_orchestrator.v1.PaymentOrchestratorService/LockHTLC

# 6. Verify on-chain state
cast call $HTLC_ADDRESS \
  "getLockDetails(bytes32)" $COMMITMENT_HASH \
  --rpc-url $BESU_RPC
# Should show: state=LOCKED
```

## Troubleshooting

### Issue: CommitmentHashRegistry deployment fails with "Unauthorized"

Ensure the deployer address has the governance role in IdentityRegistry:
```bash
cast send $IDENTITY_REGISTRY_ADDRESS \
  "registerParticipant(address,string,uint8,bytes32)" \
  $DEPLOYER_ADDRESS "Deployer" 1 0x0 \
  --rpc-url $BESU_RPC_URL --private-key $DEPLOYER_PRIVATE_KEY
```

### Issue: HTLC.lock() fails with "CommitmentNotAccepted"

Verify the commitment was registered and accepted:
```bash
cast call $COMMITMENT_HASH_REGISTRY_ADDRESS "isAccepted(bytes32)" $COMMITMENT_HASH --rpc-url $BESU_RPC_URL
# Should return: true
```

### Issue: Pente context creation times out

Verify the Paladin sidecar is healthy:
```bash
curl http://localhost:31648/api/v1/health   # CB-A Paladin (port 31748 for CB-B)
```

### Issue: Backend logs show "FX_AGREEMENT_ADDRESS not set"

Add the deployed FX agreement address to the entity's env file and restart:
```bash
# In backend/config/.env.infra.<entity>
FX_AGREEMENT_ADDRESS=0x...

make deploy.up-backend-<entity>
```

## Rollback Procedure

1. **Stop services:**
   ```bash
   make deploy.down-backend
   ```

2. **Reset database (if needed):**
   ```bash
   psql "postgres://default:default@localhost:5432/cbweb3_central_bank_a?sslmode=disable" \
     -c "DROP TABLE fx_agreements, fx_agreement_events, relay_delivery_records CASCADE;"
   ```

3. **Redeploy** from the affected step.

## Production Deployment Checklist

- [ ] All 3 contracts deployed on both spokes (IdentityRegistry, CommitmentHashRegistry, HTLC)
- [ ] Contract addresses synced to `backend/config/.env.infra.*` files
- [ ] Participants registered in IdentityRegistry on both spokes
- [ ] E2E test passed (propose → accept → lock)
- [ ] Cross-spoke relay tested
- [ ] Pente integration verified (if `PENTE_ENABLED=true`)
- [ ] Monitoring/alerting configured
- [ ] Rollback procedure tested
- [ ] Security audit completed (if required)

## References

- [FX Agreement Architecture](../architecture/fx-agreement-hybrid-design.md)
- [FX Agreement Production Hardening](../architecture/fx-agreement-production-hardening.md)
- [CommitmentHashRegistry.sol](../../contracts/src/CommitmentHashRegistry.sol)
- [HashTimeLockedContract.sol](../../contracts/src/HashTimeLockedContract.sol)
- [Port Reference](../../README.md#port-reference)
- [Hyperledger Besu](https://besu.hyperledger.org/)
