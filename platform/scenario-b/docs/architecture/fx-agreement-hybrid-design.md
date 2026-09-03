# FX Agreement Architecture — Hybrid Besu/Pente Design

## Overview

The FX Agreement system uses a **hybrid approach** for bilateral contract management:
- **Private Pente Contracts** for confidential agreement lifecycle (one per bilateral pair)
- **Public Besu Registries** for identity verification and HTLC gating coordination
- **Service-Layer Defense-in-Depth** for agreement enforcement

## Architecture Diagram

```
┌────────────────────────────────────────────────────────────────────┐
│                     Payment Orchestrator (Backend)                │
│                                                                    │
│  ┌──────────────────────────────────────────────────────────────┐│
│  │ FX Agreement Service                                         ││
│  │                                                              ││
│  │  ProposeFXAgreement()                                        ││
│  │    ├─ Validate originator in DB                             ││
│  │    ├─ Persist to PostgreSQL (fx_agreements table)           ││
│  │    └─ [Future] Call Pente contract (private context)        ││
│  │                                                              ││
│  │  AcceptFXAgreement()                                         ││
│  │    ├─ Load agreement from PostgreSQL                        ││
│  │    ├─ Create/ensure Pente bilateral context (PenteClient)   ││
│  │    ├─ Call Pente FXAgreement.accept() (private)             ││
│  │    ├─ Persist GroupID + ContractAddress                     ││
│  │    └─ [Future] Register commitment hash to public registry   ││
│  │                                                              ││
│  │  LockHTLC() — Defense-in-Depth Validation                   ││
│  │    ├─ GATE 1: Service-layer checks agreement state          ││
│  │    │           (ACCEPTED, not expired, amount match)        ││
│  │    ├─ GATE 2: On-chain FXAgreement.accept() check           ││
│  │    │           (if callable from Pente)                     ││
│  │    └─ GATE 3: CommitmentHashRegistry.isAccepted()           ││
│  │               (fallback if Pente unavailable)               ││
│  │                                                              ││
│  └──────────────────────────────────────────────────────────────┘│
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
         │                              │
         │                              │
         v                              v
┌──────────────────┐       ┌───────────────────────────────────┐
│  PostgreSQL DB   │       │   Public Besu Blockchain         │
│                  │       │                                   │
│ fx_agreements    │       │ ┌─────────────────────────────┐   │
│ fx_agreement_    │       │ │  IdentityRegistry           │   │
│   events         │       │ │  (Shared verification)      │   │
│ relay_delivery_  │       │ │                             │   │
│   records        │       │ │  canTransact(address)       │   │
│                  │       │ │  canGovern(address)         │   │
│ Stores:          │       │ └─────────────────────────────┘   │
│ - Agreement      │       │                                   │
│   state          │       │ ┌─────────────────────────────┐   │
│ - GroupID        │       │ │ CommitmentHashRegistry      │   │
│ - ContractAddr   │       │ │ (Fallback HTLC gating)      │   │
│ - Bilateral      │       │ │                             │   │
│   event log      │       │ │ registerCommitment()        │   │
│ - Relay retry    │       │ │ acceptCommitment()          │   │
│   state          │       │ │ settleCommitment()          │   │
│                  │       │ │ isAccepted()                │   │
│                  │       │ │                             │   │
│                  │       │ │ Storage:                    │   │
│                  │       │ │ - Commitment hashes         │   │
│                  │       │ │ - State transitions         │   │
│                  │       │ └─────────────────────────────┘   │
│                  │       │                                   │
│                  │       │ ┌─────────────────────────────┐   │
│                  │       │ │ HashTimeLockedContract      │   │
│                  │       │ │ (Cross-spoke coordination)  │   │
│                  │       │ │                             │   │
│                  │       │ │ lock(agreementId)           │   │
│                  │       │ │ settle(secret)              │   │
│                  │       │ │ refund()                    │   │
│                  │       │ └─────────────────────────────┘   │
│                  │       │                                   │
└──────────────────┘       └───────────────────────────────────┘
         │
         │
         v
    ┌──────────────────────────────────┐
    │ Paladin / Pente (Private)         │
    │                                  │
    │ ┌──────────────────────────────┐│
    │ │ FXAgreement Contract         ││
    │ │ (Private Pente Context)      ││
    │ │ - One per bilateral pair     ││
    │ │ - Only parties visible       ││
    │ │                              ││
    │ │ propose()                    ││
    │ │ accept()  → [Future]         ││
    │ │ reject()    atomic lock()    ││
    │ │ cancel()    externalCall()   ││
    │ │ settle()                     ││
    │ │ getAgreement()               ││
    │ └──────────────────────────────┘│
    │                                  │
    │ GroupID: Bilateral context       │
    │ ContractAddress: FXAgreement addr│
    └──────────────────────────────────┘
```

## Component Details

### 1. **Public Besu — IdentityRegistry**

**Purpose**: Shared participant verification across all agreements and HTLC flows.

**Key Methods**:
- `canTransact(address)` — Returns true if participant is verified and active
- `canGovern(address)` — Returns true if participant has governance role (Central Bank)

**Used By**:
- CommitmentHashRegistry (governance gates)
- HashTimeLockedContract (participant clearance)
- Service layer (identity validation)

### 2. **Public Besu — CommitmentHashRegistry**

**Purpose**: Fallback FX agreement gating when Pente contract is unavailable or as additional validation layer.

**Key Data Structure**:
```solidity
struct Commitment {
    bytes32 tradeId;
    bytes32 commitmentHash;         // keccak256(tradeId || amount || rate)
    CommitmentState state;          // PENDING | ACCEPTED | SETTLED | CANCELLED
    // ... other fields
}
```

**Key Methods**:
- `registerCommitment(tradeId, originator, counterpartyB, amounts, rate)` — Governance registers a commitment
- `acceptCommitment(commitmentHash)` — Governance marks as accepted
- `isAccepted(commitmentHash)` — Returns true if in ACCEPTED state
- `settleCommitment(commitmentHash)` — Marks as SETTLED, no further locks allowed

**Used By**:
- Backend service (commitment persistence)
- HashTimeLockedContract.lock() (fallback gate)

### 3. **Public Besu — HashTimeLockedContract**

**Purpose**: Coordinate atomic swap across spokes via hashlock/timelock mechanism.

**Enforcement Logic** (Defense-in-Depth):
```
lock(agreementId) called:
  └─ Gate 1: Service-layer state validation (PRIMARY)
     │   Agreement is ACCEPTED && not expired
     │
  └─ Gate 2: On-chain FXAgreement check (if deployed in Pente)
     │   Calls agreement contract to verify state
     │
  └─ Gate 3: CommitmentHashRegistry check (FALLBACK)
     │   Agreement commitment hash is ACCEPTED in registry
     │
     └─ All gates must pass (fail-closed)
```

**Updated Dependencies**:
- `CommitmentHashRegistry` — For fallback commitment checking
- `IFXAgreement` — For future on-chain agreement state checking

### 4. **PostgreSQL — FX Agreement Storage**

**Tables**:
- `fx_agreements` — Current agreement state, includes GroupID + ContractAddress from Pente
- `fx_agreement_events` — Audit trail of all state transitions
- `relay_delivery_records` — Cross-spoke event delivery with retry tracking

**Key Fields** (addition):
```sql
ALTER TABLE fx_agreements ADD COLUMN pente_group_id VARCHAR(255);
ALTER TABLE fx_agreements ADD COLUMN pente_contract_address VARCHAR(42);
```

### 5. **Private Pente — FXAgreement Contract**

**Purpose**: Bilateral agreement lifecycle with privacy-preserving terms.

**Deployment Pattern**:
- Deployed per bilateral pair via Paladin HTTP API
- Same Solidity code as public version, but in isolated Pente context
- Only counterparties and designated governance can access

**Key Methods** (Same as public):
- `propose()` / `proposeOnBehalf()`
- `accept()` / `acceptOnBehalf()`
- `reject()` / `rejectOnBehalf()`
- `cancel()`
- `settle()`
- `getAgreement(tradeId)`

**Future Enhancement** (Phase B):
```
FXAgreement.accept() calls:
  └─ Pente externalCall() to:
     └─ HashTimeLockedContract.lock() on public Besu
        (atomic within Pente transaction)
```

## Enforcement Strategy

### **Phase A: Current (Implemented)**

```
HTLC Lock Request:
  ├─ Service-Layer Check (PRIMARY)
  │  └─ Load agreement from PostgreSQL
  │     ├─ State == ACCEPTED
  │     ├─ Not expired
  │     └─ Receiver authorized for this trade
  │
  └─ On-Chain Fallback (SECONDARY)
     ├─ Check: CommitmentHashRegistry.isAccepted(commitmentHash)?
     └─ Result: Pass/Fail on-chain
```

**Advantages**:
- ✅ Service layer provides immediate enforcement
- ✅ No cross-network latency (service is local)
- ✅ CommitmentHashRegistry is lightweight (no full contract state)
- ✅ Defense-in-depth: both gates must pass

**Limitations**:
- Service compromise could bypass on-chain gate
- No atomicity guarantee between service decision and on-chain lock

### **Phase B: Future (When Pente Ready)**

```
FXAgreement.accept() in Pente:
  └─ Executes atomically with:
     └─ HTLC.lock() externalCall() on public Besu
        (same Pente transaction)
```

**Advantages**:
- ✅ Atomic accept + lock (no bypass window)
- ✅ Private execution preserves confidentiality
- ✅ On-chain verified

**Requirements**:
- Pente `externalCalls` feature stable and tested
- Cross-network call mechanism proven in production

## Deployment Sequence

**Order** (Critical for dependencies):

1. **IdentityRegistry** (public Besu)
   ```bash
   forge script script/IdentityRegistry.s.sol \
     --broadcast \
     --rpc-url $BESU_RPC \
     -vvv
   ```

2. **CommitmentHashRegistry** (public Besu)
   ```bash
   forge script script/CommitmentHashRegistry.s.sol \
     --broadcast \
     --rpc-url $BESU_RPC \
     -vvv
   ```
   **Env vars**:
   - `DEPLOYER_PRIVATE_KEY`
   - `IDENTITY_REGISTRY_ADDRESS` ← From step 1

3. **HashTimeLockedContract** (public Besu, updated)
   ```bash
   forge script script/HashTimeLockedContract.s.sol \
     --broadcast \
     --rpc-url $BESU_RPC \
     -vvv
   ```
   **Env vars**:
   - `DEPLOYER_PRIVATE_KEY`
   - `IDENTITY_REGISTRY_ADDRESS` ← From step 1
   - `COMMITMENT_HASH_REGISTRY_ADDRESS` ← From step 2

4. **FXAgreement** (private Pente contexts)
   ```bash
   # Via Paladin API (manifest-based deployment)
   # Handled by backend service on-demand per bilateral pair
   ```

## Testing Strategy

### **Unit Tests**:
- ✅ CommitmentHashRegistry.t.sol — State machine, governance gates
- ✅ HashTimeLockedContract updated tests — Fallback gate validation
- ✅ FXAgreement.t.sol — Agreement lifecycle (can run against Pente or mock)

### **Integration Tests**:
- Service-layer + PostgreSQL + CommitmentHashRegistry coordination
- Cross-spoke relay with bilateral context creation
- Pente context creation and FXAgreement deployment (when Pente available)

## Configuration & Environment

**Backend Service** (payment-orchestrator):
```env
# FX Agreement Enforcement
FX_AGREEMENT_HTLC_STRICT=true               # Enable service-layer gate
PENTE_ENABLED=true|false                    # Enable Pente integration
PENTE_BASE_URL=http://paladin:8080          # Paladin HTTP API

# Database
POSTGRES_DSN=postgresql://user:pass@host/db # FX agreement persistence

# Blockchain
BESU_RPC=http://besu:8545
COMMITMENT_HASH_REGISTRY_ADDRESS=0x...      # From deployment
```

**Docker Compose** (backend/docker-compose.yml):
```yaml
services:
  payment-orchestrator:
    environment:
      - FX_AGREEMENT_HTLC_STRICT=true
      - PENTE_ENABLED=${PENTE_ENABLED:-false}
      - PENTE_BASE_URL=${PENTE_BASE_URL}
      - COMMITMENT_HASH_REGISTRY_ADDRESS=${COMMITMENT_HASH_REGISTRY_ADDRESS}
```

## Future Enhancements

1. **Pente ExternalCalls Atomicity** (Phase B)
   - Enable atomic accept() → lock() coupling
   - Eliminate service-layer bypass surface

2. **Zeto Privacy Integration**
   - Link Zeto token transfers to FX agreement on Pente
   - Enable confidential settlement tracking

3. **Cross-Chain Commitment Verification**
   - Multi-signature commitment from both spokes
   - Enhanced security for bilateral context

4. **Performance Optimization**
   - Batch commitment registration
   - Caching strategies for CommitmentHashRegistry checks

## References

- [research.md](../research.md) — Architecture decisions and rationale
- [spec.md](../spec.md) — Functional requirements and acceptance criteria
- [CommitmentHashRegistry.sol](../../../contracts/src/CommitmentHashRegistry.sol) — Source code
- [HashTimeLockedContract.sol](../../../contracts/src/HashTimeLockedContract.sol) — Updated HTLC
