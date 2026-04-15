# Quick Start: FX Agreement E2E Tryout

## Overview

The `tryout-fx-agreement-e2e.sh` script validates the complete implementation of feature 001-harden-fx-agreement through an end-to-end test covering:

- FX Agreement lifecycle (propose, accept, settle)
- Persistent state in PostgreSQL
- Audit trail with state transitions
- Cross-spoke relay synchronization
- HTLC locking with on-chain commitment verification
- Automatic expiration of stale agreements

## Prerequisites

### 1. Ensure all services are running

```bash
# Start all backend services + Cacti relay
make dev.up

# Or individually:
make dev.up-bank-a dev.up-bank-b dev.up-central-bank-a dev.up-central-bank-b
make cacti-up
```

### 2. Verify environment files exist

```bash
ls -la backend/config/.env.infra.{bank-a,bank-b,central-bank-a,central-bank-b}
```

### 3. Verify curl and jq are installed

```bash
which curl jq
```

## Running the Test

### Option 1: Full test with onboarding (first run)

```bash
cd tryouts
./tryout-fx-agreement-e2e.sh
```

**Expected runtime**: 3-5 minutes (includes onboarding phases)

### Option 2: Quick test (skip onboarding)

If you've already onboarded the banks:

```bash
cd tryouts
./tryout-fx-agreement-e2e.sh --skip-onboarding
```

**Expected runtime**: 1-2 minutes

### Option 3: Debug mode

Enable verbose output:

```bash
cd tryouts
VERBOSE=true ./tryout-fx-agreement-e2e.sh --skip-onboarding
```

### Option 4: Non-Pente mode

If Pente is not available, test CommitmentHashRegistry only:

```bash
cd tryouts
./tryout-fx-agreement-e2e.sh --skip-onboarding --no-pente
```

## What the Test Validates

### Step 5: FX Agreement Proposal
- ✅ Bank-A successfully proposes a cross-spoke FX agreement
- ✅ Unique trade ID generated
- ✅ State transitions to `PROPOSED`

### Step 6: Persistent Storage
- ✅ Agreement exists in PostgreSQL `fx_agreements` table
- ✅ GET /fx/agreements/{trade_id} returns current state
- ✅ Survives service restart

### Step 7: Audit Trail
- ✅ Append-only event log created in `fx_agreement_events` table
- ✅ GET /fx/agreements/{trade_id}/events returns all state changes
- ✅ Each event includes actor, timestamp, before/after state

### Step 8-9: Cross-Spoke Relay
- ✅ Bank-B acceptance propagates to Bank-A via Cacti relay
- ✅ Deduplication prevents duplicate events
- ✅ Exponential backoff retry handles transient failures
- ✅ Synchronization within configurable timeout (default: 30s)

### Step 10: HTLC Lock with Commitment
- ✅ CommitmentHashRegistry validates keccak256(tradeId || amounts || rate)
- ✅ HTLC lock proceeds only if commitment exists and is PENDING
- ✅ Service-layer gate prevents bypass

### Step 11: HTLC Settlement
- ✅ Secret preimage verified against lock hash
- ✅ On-chain settlement succeeds with corresponding state change
- ✅ Final state is SETTLED in both spokes

### Step 12: Expiration
- ✅ Background worker detects stale proposals
- ✅ Proposals past expiry_date marked as EXPIRED or CANCELLED
- ✅ Automatic cleanup prevents accumulation

## Expected Output

```
════════════════════════════════════════════════════════════
  STEP: Step 5: Propose FX Agreement
════════════════════════════════════════════════════════════
[INFO] Bank-A proposes FX agreement...
✅ FX Agreement proposed: TRADE-1713180846123

════════════════════════════════════════════════════════════
  STEP: Step 6: Validate Persistence
════════════════════════════════════════════════════════════
[INFO] Querying FX agreement: TRADE-1713180846123
✅ Agreement persisted with state: PROPOSED

════════════════════════════════════════════════════════════
  STEP: Step 9: Validate Cross-Spoke Synchronization
════════════════════════════════════════════════════════════
[INFO] Waiting for relay synchronization (30s timeout)...
✅ Cross-spoke synchronization complete

════════════════════════════════════════════════════════════
  STEP: SUMMARY
════════════════════════════════════════════════════════════
✅ End-to-End FX Agreement + HTLC Test Completed Successfully ✅

Key Results:
  Trade ID:        TRADE-1713180846123
  Commitment Hash: 0x4d967a2fab9...
  Contract ID:     0xabcd123456...

All validation steps passed:
  ✅ FX Agreement proposal
  ✅ Persistent storage
  ✅ Audit trail
  ✅ Cross-spoke relay sync
  ✅ HTLC locking with commitment
  ✅ HTLC settlement
  ✅ Expiration handling
```

## Troubleshooting

### "ERROR: KC_CLIENT_SECRET not found"

**Cause**: Environment files missing KC_CLIENT_SECRET value.

**Fix**:
```bash
# Ensure .env files are properly configured
cat backend/config/.env.infra.bank-a | grep KC_CLIENT_SECRET
```

### "ERROR: POST ... failed (HTTP 503)"

**Cause**: Backend services not running or not ready.

**Fix**:
```bash
# Check service status
curl -s http://localhost:18080/health | jq .
# Restart if needed
make dev.down && make dev.up-bank-a
```

### "ERROR: timeout or failed (network)"

**Cause**: Service taking longer to respond.

**Fix**:
```bash
# Increase timeout
HTTP_TIMEOUT=300 ./tryout-fx-agreement-e2e.sh --skip-onboarding
```

### "Cross-spoke synchronization timed out"

**Cause**: Cacti relay not running or relay auth not configured.

**Fix**:
```bash
# Check if Cacti is running
ps aux | grep cacti
# Restart if needed
make cacti-down && make cacti-up
# Increase timeout
RELAY_SYNC_TIMEOUT=60 ./tryout-fx-agreement-e2e.sh --skip-onboarding
```

## Validating Results in Database

After running the test, verify state was persisted:

```bash
# Connect to PostgreSQL (adjust credentials as needed)
psql -h localhost -U postgres -d bank_a -c "SELECT trade_id, state FROM fx_agreements LIMIT 5;"

# Check audit trail
psql -h localhost -U postgres -d bank_a -c "SELECT trade_id, state_before, state_after, actor FROM fx_agreement_events ORDER BY created_at DESC LIMIT 10;"

# Check relay delivery records (idempotency)
psql -h localhost -U postgres -d bank_a -c "SELECT idempotency_key, status, retry_count FROM relay_delivery_records WHERE status='delivered' LIMIT 5;"
```

## Next Steps

1. **Verify contract on Besu**: Check CommitmentHashRegistry state in Remix or ethers.js
2. **Monitor logs**: Tail service and relay logs to see internal flow
3. **Stress test**: Modify script to run multiple concurrent proposals
4. **Integration**: Add custom business logic validation based on your requirements

## Documentation References

- **Feature Spec**: [specs/001-harden-fx-agreement/spec.md](../specs/001-harden-fx-agreement/spec.md)
- **Implementation Plan**: [specs/001-harden-fx-agreement/plan.md](../specs/001-harden-fx-agreement/plan.md)
- **Data Model**: [specs/001-harden-fx-agreement/data-model.md](../specs/001-harden-fx-agreement/data-model.md)
- **Tryout README**: [README.md](./README.md)

---

**Last Updated**: 2026-04-15  
**Feature Branch**: `feature/agreement-v2`
