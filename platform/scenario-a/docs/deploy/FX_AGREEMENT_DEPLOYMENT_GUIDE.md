# FX Agreement Deployment Guide

## Overview

This guide covers deploying FX Agreement infrastructure across Besu (public) and Pente (private) blockchains.

## Prerequisites

- ✅ Foundry (forge) installed
- ✅ Both Besu networks running (Spoke A & B)
- ✅ Paladin sidecars running (if enabling Pente integration)
- ✅ Deployer account funded on both networks
- ✅ Environment variables configured

## Environment Setup

```bash
# Core deployment vars
export DEPLOYER_PRIVATE_KEY=0x...              # Account with deployment funds
export BESU_RPC=http://localhost:8545          # Spoke A Besu RPC
export BESU_RPC_SPOKE_B=http://localhost:8546  # Spoke B Besu RPC

# Pente/Paladin integration (optional)
export PENTE_ENABLED=true|false
export PENTE_BASE_URL=http://localhost:8080    # Paladin HTTP API
export PALADIN_URL=http://localhost:31648      # Paladin sidecar

# Contract addresses (filled during deployment)
export IDENTITY_REGISTRY_ADDRESS=""
export COMMITMENT_HASH_REGISTRY_ADDRESS=""
export HTLC_ADDRESS=""
```

## Deployment Steps

### **Step 1: Deploy IdentityRegistry (Public Besu)**

```bash
cd contracts

# Deploy to Spoke A
forge script script/IdentityRegistry.s.sol \
  --broadcast \
  --rpc-url $BESU_RPC \
  --private-key $DEPLOYER_PRIVATE_KEY \
  -vvv

# Extract address from output
export IDENTITY_REGISTRY_ADDRESS=0x...  # Save this
```

**Output**:
```
Deploying: IdentityRegistry (0x1234...)
Block: 42
Confirmed ✓
```

**Verify**:
```bash
cast call $IDENTITY_REGISTRY_ADDRESS "GOVERNANCE_ROLE()" --rpc-url $BESU_RPC
```

### **Step 2: Deploy CommitmentHashRegistry (Public Besu)**

```bash
# Deploy to Spoke A
forge script script/CommitmentHashRegistry.s.sol \
  --broadcast \
  --rpc-url $BESU_RPC \
  --private-key $DEPLOYER_PRIVATE_KEY \
  -vvv \
  --sig "run()" \
  --constructor-args $IDENTITY_REGISTRY_ADDRESS

# Extract address from output
export COMMITMENT_HASH_REGISTRY_ADDRESS=0x...  # Save this
```

**Output**:
```
Deploying: CommitmentHashRegistry (0x5678...)
Initializing with IdentityRegistry: 0x1234...
Block: 43
Confirmed ✓
```

**Verify**:
```bash
cast call $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "IDENTITY_REGISTRY()" \
  --rpc-url $BESU_RPC

# Should return: 0x1234... (IdentityRegistry address)
```

### **Step 3: Deploy HashTimeLockedContract (Public Besu, Updated)**

```bash
# Deploy to Spoke A
forge script script/HashTimeLockedContract.s.sol \
  --broadcast \
  --rpc-url $BESU_RPC \
  --private-key $DEPLOYER_PRIVATE_KEY \
  -vvv

# Extract address from output
export HTLC_ADDRESS=0x...  # Save this
```

**Important**: The script now passes 3 parameters:
1. `IDENTITY_REGISTRY_ADDRESS`
2. `FX_AGREEMENT_ADDRESS` (set to address(0) for now)
3. `COMMITMENT_HASH_REGISTRY_ADDRESS`

**Output**:
```
Deploying: HashTimeLockedContract (0x9ABC...)
Initializing with:
  - IdentityRegistry: 0x1234...
  - FXAgreement: 0x0000...
  - CommitmentHashRegistry: 0x5678...
Block: 44
Confirmed ✓
```

**Verify**:
```bash
cast call $HTLC_ADDRESS \
  "COMMITMENT_HASH_REGISTRY()" \
  --rpc-url $BESU_RPC

# Should return: 0x5678... (CommitmentHashRegistry address)
```

### **Step 4: Deploy to Spoke B (Repeat Steps 1-3)**

```bash
# Set Spoke B RPC
export BESU_RPC=$BESU_RPC_SPOKE_B

# Repeat steps 1-3 with different env var names
export IDENTITY_REGISTRY_ADDRESS_SPOKE_B=0x...
export COMMITMENT_HASH_REGISTRY_ADDRESS_SPOKE_B=0x...
export HTLC_ADDRESS_SPOKE_B=0x...
```

### **Step 5: Configure Backend Service**

Update backend environment (`backend/services/payment-orchestrator/.env`):

```env
# Spoke A
BESU_RPC=http://localhost:8545
IDENTITY_REGISTRY_ADDRESS=0x...                    # From step 1
COMMITMENT_HASH_REGISTRY_ADDRESS=0x...             # From step 2
HTLC_ADDRESS=0x...                                 # From step 3

# Pente/Paladin
PENTE_ENABLED=true                                 # or false
PENTE_BASE_URL=http://paladin-sidecar:8080
PALADIN_URL=http://paladin-sidecar:31648

# Database
POSTGRES_DSN=postgresql://user:pass@db:5432/cbweb3

# FX Enforcement
FX_AGREEMENT_HTLC_STRICT=true
```

### **Step 6: Deploy Relayer Component**

Update relayer config for cross-spoke coordination:

**File**: `interop/hub-and-spoke/cacti/.env`

```env
# Spoke A
SPOKE_A_RPC=http://localhost:8545
SPOKE_A_HTLC_ADDRESS=0x...

# Spoke B
SPOKE_B_RPC=http://localhost:8546
SPOKE_B_HTLC_ADDRESS=0x...

# Coordination
RELAY_MODE=htlc+fx
FX_AGREEMENT_ENFORCEMENT=true
```

### **Step 7: Start Backend Services**

```bash
cd backend/services/payment-orchestrator
docker-compose up -d

# Verify startup
docker logs -f payment-orchestrator | grep "FX Agreement"
```

**Expected Output**:
```
INFO: FX Agreement service initialized
INFO: PENTE_ENABLED=true (or false)
INFO: CommitmentHashRegistry at 0x5678...
INFO: PostgreSQL connected
```

### **Step 8: Register Initial Participants**

```bash
# Call IdentityRegistry to register banks
cast send $IDENTITY_REGISTRY_ADDRESS \
  "registerParticipant(address,string,uint8,bytes32)" \
  0x<bank_a_address> "Bank A" 2 0x0 \
  --rpc-url $BESU_RPC \
  --private-key $DEPLOYER_PRIVATE_KEY

cast send $IDENTITY_REGISTRY_ADDRESS \
  "registerParticipant(address,string,uint8,bytes32)" \
  0x<bank_b_address> "Bank B" 2 0x0 \
  --rpc-url $BESU_RPC \
  --private-key $DEPLOYER_PRIVATE_KEY
```

## Verification Checklist

### **On-Chain Verification**

```bash
# 1. IdentityRegistry deployed
cast call $IDENTITY_REGISTRY_ADDRESS "GOVERNANCE_ROLE()" --rpc-url $BESU_RPC
# Should return: 0x0000... (hashed role)

# 2. CommitmentHashRegistry linked
cast call $COMMITMENT_HASH_REGISTRY_ADDRESS "IDENTITY_REGISTRY()" --rpc-url $BESU_RPC
# Should return: $IDENTITY_REGISTRY_ADDRESS

# 3. HTLC linked
cast call $HTLC_ADDRESS "COMMITMENT_HASH_REGISTRY()" --rpc-url $BESU_RPC
# Should return: $COMMITMENT_HASH_REGISTRY_ADDRESS

# 4. Test CommitmentHashRegistry gate
## Register a dummy commitment
cast send $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "registerCommitment(bytes32,address,address,uint256,uint256,uint256)" \
  0x<trade_id> 0x<originator> 0x<counterparty> 1000 5500 5500000000000000 \
  --rpc-url $BESU_RPC \
  --private-key $DEPLOYER_PRIVATE_KEY

## Accept it
cast send $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "acceptCommitment(bytes32)" \
  0x<commitment_hash> \
  --rpc-url $BESU_RPC \
  --private-key $DEPLOYER_PRIVATE_KEY

## Verify state
cast call $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "isAccepted(bytes32)" \
  0x<commitment_hash> \
  --rpc-url $BESU_RPC
# Should return: true
```

### **Backend Service Verification**

```bash
# 1. Service startup
curl http://localhost:8080/health
# Should return: {"status":"healthy"}

# 2. Propose FX agreement
grpcurl -plaintext \
  -d '{"trade_id":"TEST-001","counterparty_b":"0x...","origin_amount":"1000000","counter_amount":"5500000","origin_currency":"BRL","counter_currency":"EUR","rate":"5500000000000000","expiry_date":"1713110400"}' \
  localhost:5001 \
  payment_orchestrator.v1.PaymentOrchestratorService/ProposeFXAgreement

# 3. Accept FX agreement
grpcurl -plaintext \
  -d '{"trade_id":"TEST-001"}' \
  localhost:5001 \
  payment_orchestrator.v1.PaymentOrchestratorService/AcceptFXAgreement

# 4. Check database
psql $POSTGRES_DSN -c "SELECT trade_id, state FROM fx_agreements LIMIT 5;"
```

### **Pente Integration Verification** (if enabled)

```bash
# 1. Check Pente context creation
curl http://localhost:8080/api/v1/pente/contexts
# Should list bilateral contexts

# 2. Verify FXAgreement contract deployed
curl http://localhost:8080/api/v1/contracts?type=fx_agreement
# Should show deployed contracts

# 3. Check service logs for Pente calls
docker logs payment-orchestrator | grep "Pente"
# Should show: "EnsureFXContext succeeded"
```

## Testing Flow

### **End-to-End Test**

```bash
#!/bin/bash

# 1. Propose agreement
TRADE_ID="TEST-$(date +%s)"
grpcurl -plaintext \
  -d "{\"trade_id\":\"$TRADE_ID\",\"counterparty_b\":\"0x...\",..." \
  localhost:5001 \
  payment_orchestrator.v1.PaymentOrchestratorService/ProposeFXAgreement

# 2. Accept agreement
grpcurl -plaintext \
  -d "{\"trade_id\":\"$TRADE_ID\"}" \
  localhost:5001 \
  payment_orchestrator.v1.PaymentOrchestratorService/AcceptFXAgreement

# 3. Register commitment on Besu (if Pente not available)
COMMITMENT_HASH=$(cast keccak $(echo -n "$TRADE_ID" | tr -d '\n'))
cast send $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "registerCommitment(...)" ... # full call

# 4. Accept commitment
cast send $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "acceptCommitment(bytes32)" $COMMITMENT_HASH ...

# 5. Lock HTLC (should succeed now)
grpcurl -plaintext \
  -d "{\"contract_id\":\"HTL-$TRADE_ID\",\"agreement_id\":\"$TRADE_ID\",...}" \
  localhost:5001 \
  payment_orchestrator.v1.PaymentOrchestratorService/LockHTLC
# Should return: tx_hash

# 6. Verify on-chain state
cast call $HTLC_ADDRESS \
  "getLockDetails(bytes32)" $CONTRACT_ID \
  --rpc-url $BESU_RPC
# Should show: state=LOCKED
```

## Troubleshooting

### **Issue**: CommitmentHashRegistry deployment fails with "Unauthorized"

**Solution**: Ensure deployer is governance role in IdentityRegistry:
```bash
cast send $IDENTITY_REGISTRY_ADDRESS \
  "registerParticipant(...)" \
  $DEPLOYER_ADDRESS "Deployer" <CENTRAL_BANK_ROLE> 0x0 \
  --rpc-url $BESU_RPC
```

### **Issue**: HTLC.lock() fails with "CommitmentNotAccepted"

**Solution**: Verify commitment was registered and accepted:
```bash
cast call $COMMITMENT_HASH_REGISTRY_ADDRESS \
  "isAccepted(bytes32)" $COMMITMENT_HASH \
  --rpc-url $BESU_RPC
# Should return: true
```

### **Issue**: Pente context creation times out

**Solution**: Verify Paladin sidecar is running:
```bash
curl http://localhost:8080/health
# Should return: {"status":"ready"}
```

### **Issue**: Backend logs show "FX_AGREEMENT_HTLC_STRICT=false"

**Solution**: Update env var and restart:
```bash
export FX_AGREEMENT_HTLC_STRICT=true
docker-compose restart payment-orchestrator
```

## Rollback Procedure

If deployment fails and needs rollback:

1. **Stop services**:
   ```bash
   docker-compose down
   ```

2. **Reset PostgreSQL** (if needed):
   ```bash
   psql $POSTGRES_DSN -c "DROP TABLE fx_agreements, fx_agreement_events, relay_delivery_records CASCADE;"
   ```

3. **Redeploy** (repeat from affected step)

## Production Deployment Checklist

- [ ] All 3 contracts deployed (IdentityRegistry, CommitmentHashRegistry, HTLC)
- [ ] Contract addresses verified on-chain
- [ ] Backend env vars updated
- [ ] Participants registered in IdentityRegistry
- [ ] PostgreSQL migrated to GORM
- [ ] E2E test passed (propose → accept → lock)
- [ ] Monitoring/alerting configured
- [ ] Rollback procedure documented and tested
- [ ] Spoke B deployment completed
- [ ] Cross-spoke relay tested
- [ ] Pente integration verified (if enabled)
- [ ] Security audit completed (if required)

## References

- [FX Agreement Architecture](./fx-agreement-hybrid-design.md)
- [CommitmentHashRegistry.sol](../../contracts/src/CommitmentHashRegistry.sol)
- [HashTimeLockedContract.sol](../../contracts/src/HashTimeLockedContract.sol)
- [Besu Documentation](https://besu.hyperledger.org/)
- [Paladin/Pente Integration](../architecture/paladin-pente-integration.md)
