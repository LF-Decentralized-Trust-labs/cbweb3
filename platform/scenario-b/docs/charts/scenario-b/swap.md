# CBWeb3 Scenario B — Cross-Chain Swap Operation (Detailed)

> End-to-end flow for a Hub-and-Spoke AMM swap with bridge lock/mint and burn/unlock lifecycle,
> including Cacti relay coordination, compliance gates, and circuit breaker governance.

---

## Phase 1: Pool Provisioning (Pre-condition)

Before any swap can execute, Central Banks must provision the AMM pool with initial liquidity.

```mermaid
sequenceDiagram
    autonumber
    participant CB_A as Central Bank A<br/>(API Gateway :38080)
    participant AUTH as Auth Service<br/>(gRPC :9091)
    participant KC as Keycloak<br/>(:8081)
    participant PO as Payment Orchestrator<br/>(gRPC :9094)
    participant HUB_AMM as Hub AMM<br/>(chain 1338)
    participant HUB_TOKEN_A as tCeBM_BRL<br/>(Hub Token A)
    participant HUB_TOKEN_B as tCeBM_EUR<br/>(Hub Token B)

    Note over CB_A,KC: Authentication
    CB_A->>AUTH: client_credentials grant
    AUTH->>KC: POST /token (central-bank-a realm)
    KC-->>AUTH: access_token (CENTRAL_BANK role)
    AUTH-->>CB_A: JWT bearer token

    Note over CB_A,HUB_TOKEN_B: Mint Hub Tokens
    CB_A->>PO: POST /api/v2/amm/token/mint-and-approve<br/>{amount_a: 200000, amount_b: 200000}
    PO->>HUB_TOKEN_A: mint(central_bank_a_signer, 200000)
    HUB_TOKEN_A-->>PO: tx_hash_1
    PO->>HUB_TOKEN_B: mint(central_bank_a_signer, 200000)
    HUB_TOKEN_B-->>PO: tx_hash_2

    Note over PO,HUB_AMM: Approve AMM Spending
    PO->>HUB_TOKEN_A: approve(AMM_ADDRESS, 200000)
    PO->>HUB_TOKEN_B: approve(AMM_ADDRESS, 200000)
    PO-->>CB_A: {status: "ok"}

    Note over CB_A,HUB_AMM: Seed Initial Liquidity
    CB_A->>PO: POST /api/v2/amm/liquidity/add<br/>{pool_pair: "BRL-USD", token_a_amount: 100000,<br/>token_b_amount: 100000}
    PO->>HUB_AMM: addLiquidity(100000, 100000)
    HUB_AMM->>HUB_TOKEN_A: transferFrom(CB_A, AMM, 100000)
    HUB_AMM->>HUB_TOKEN_B: transferFrom(CB_A, AMM, 100000)
    HUB_AMM-->>PO: lp_id, shares
    PO-->>CB_A: {lp_id: "...", reserve_a: 100000, reserve_b: 100000}
```

---

## Phase 2: AMM Swap Execution (US1 — Exact Output)

A commercial bank requests an exact-output swap through the compliance-gated pipeline.

```mermaid
sequenceDiagram
    autonumber
    participant BANK_A as Bank A<br/>(API Gateway :18080)
    participant AUTH as Auth Service
    participant COMP as Compliance Service<br/>(gRPC :9093)
    participant IR as IdentityRegistry<br/>(Spoke-A)
    participant SWAP_SVC as Swap Service<br/>(API Gateway)
    participant PO as Payment Orchestrator<br/>(gRPC :9094)
    participant ORACLE as ManualOracle<br/>(Hub)
    participant HUB_AMM as Hub AMM<br/>(chain 1338)
    participant HUB_TOKEN_A as tCeBM_BRL
    participant HUB_TOKEN_B as tCeBM_EUR

    Note over BANK_A,AUTH: 1. Authentication & Authorization
    BANK_A->>AUTH: client_credentials grant
    AUTH-->>BANK_A: JWT (COMMERCIAL_BANK role)

    Note over BANK_A,ORACLE: 2. Quote (read-only)
    BANK_A->>SWAP_SVC: GET /api/v2/amm/quote/exact-output<br/>?pair=BRL-USD&amount_out=1000
    SWAP_SVC->>PO: QuoteExactOutput(BRL-USD, 1000)
    PO->>HUB_AMM: getAmountIn(1000, reserve_a, reserve_b)
    HUB_AMM->>ORACLE: getLatestRate(BRL-USD)
    ORACLE-->>HUB_AMM: rate = 5.25
    HUB_AMM-->>PO: required_input = 1012 (with fee)
    PO-->>SWAP_SVC: {required_input: "1012", price_impact: "0.12%"}
    SWAP_SVC-->>BANK_A: {required_input: "1012", price_impact: "0.12%"}

    Note over BANK_A,HUB_TOKEN_B: 3. Swap Execution Pipeline
    BANK_A->>SWAP_SVC: POST /api/v2/amm/swap/exact-output<br/>{pair: "BRL-USD", amount_out: "1000",<br/>max_amount_in: "1050", payer_id: "bank-a",<br/>beneficiary_id: "bank-b"}

    rect rgb(255, 245, 238)
        Note over SWAP_SVC,COMP: Gate 1: Circuit Breaker (FR-030)
        SWAP_SVC->>HUB_AMM: paused() → false
    end

    rect rgb(243, 229, 245)
        Note over SWAP_SVC,IR: Gate 2: ZK-Pointer Compliance (FR-058)
        SWAP_SVC->>COMP: ValidateZKPointer(bank-a, zk_payer)
        COMP->>IR: canTransact(bank-a_address)
        IR-->>COMP: true (registered COMMERCIAL_BANK)
        COMP-->>SWAP_SVC: ok
        SWAP_SVC->>COMP: ValidateZKPointer(bank-b, zk_beneficiary)
        COMP-->>SWAP_SVC: ok
    end

    rect rgb(232, 245, 233)
        Note over SWAP_SVC,HUB_TOKEN_B: Gate 3: Slippage Check + Submit
        SWAP_SVC->>PO: SwapExactOutput(BRL-USD, 1000, 1050, bank-a, bank-b)
        PO->>HUB_AMM: swapExactOutput(1000, 1050)
        HUB_AMM->>HUB_TOKEN_A: transferFrom(bank-a, AMM, 1012)
        HUB_AMM->>HUB_TOKEN_B: transfer(bank-b, 1000)
        HUB_AMM-->>PO: tx_hash, amount_in=1012
        PO-->>SWAP_SVC: {order_id, tx_hash, amount_in: "1012"}
    end

    SWAP_SVC-->>BANK_A: {order_id: "...", tx_hash: "0x...",<br/>amount_in: "1012", state: "COMPLETED"}
```

---

## Phase 3: Bridge Lock & Mint (US2 — Cross-Spoke Transfer)

A commercial bank locks native tokens on its spoke; the Cacti relay mints mirrored tokens on the Hub.

```mermaid
sequenceDiagram
    autonumber
    participant BANK_A as Bank A<br/>(API Gateway :18080)
    participant PO_A as Payment Orchestrator<br/>(Spoke-A)
    participant BRIDGE_A as SpokeBridge<br/>(Spoke-A chain 1338)
    participant TOKEN_A as tCeBM_BRL<br/>(Spoke-A Token)
    participant DB as PostgreSQL<br/>(positions table)
    participant RELAYER as Relayer Worker<br/>(background goroutine)
    participant CACTI as Cacti HTLC Relay<br/>(:4000)
    participant BESU_A as Besu Spoke-A<br/>(WS :8655)
    participant HUB_TOKEN as tCeBM_BRL<br/>(Hub Token)

    Note over BANK_A,DB: 1. Lock Request
    BANK_A->>PO_A: POST /api/v2/bridge/lock-mint<br/>{owner_bank_id: "bank_a",<br/>spoke_network: "spoke-a",<br/>native_asset: "BRL-CBDC",<br/>mirrored_asset: "mBRL-CBDC",<br/>amount: "5000"}

    PO_A->>BRIDGE_A: lock(bank_a, 5000)
    BRIDGE_A->>TOKEN_A: transferFrom(bank_a, bridge, 5000)
    TOKEN_A-->>BRIDGE_A: success
    BRIDGE_A-->>PO_A: LockResult{tx_hash: "0xabc..."}

    Note over PO_A,DB: 2. Persist Position (state: LOCKING)
    PO_A->>DB: INSERT BridgedAssetPosition<br/>{position_id, state=LOCKING}
    PO_A->>DB: INSERT RelayerQueueItem<br/>{event_type=LOCK_MINT, state=PENDING}
    PO_A-->>BANK_A: {position_id: "pos-123",<br/>bridge_state: "LOCKING"}

    Note over RELAYER,HUB_TOKEN: 3. Relayer Worker Processes Queue
    RELAYER->>DB: SELECT * FROM relayer_queue<br/>WHERE state=PENDING AND next_attempt_at <= now()
    DB-->>RELAYER: [item: LOCK_MINT for pos-123]

    RELAYER->>CACTI: SubmitLockEvent(idempotency_key, pos-123)
    CACTI->>BESU_A: getPastLogs(SpokeBridge, LogLocked)
    BESU_A-->>CACTI: [LogLocked event confirmed]

    Note over CACTI,HUB_TOKEN: 4. Cacti Mints on Hub
    CACTI->>HUB_TOKEN: mint(bank_a_hub_address, 5000)
    HUB_TOKEN-->>CACTI: tx_hash_mint

    CACTI-->>RELAYER: {status: "minted", tx_hash}
    RELAYER->>DB: UPDATE position SET state=ACTIVE
    RELAYER->>DB: UPDATE queue_item SET state=DONE

    Note over BANK_A,DB: 5. Bank Polls Position Status
    BANK_A->>PO_A: GET /api/v2/bridge/positions
    PO_A->>DB: SELECT * FROM bridged_asset_positions
    DB-->>PO_A: [{position_id: "pos-123", state: "ACTIVE"}]
    PO_A-->>BANK_A: {positions: [{position_id: "pos-123",<br/>bridge_state: "ACTIVE"}]}
```

---

## Phase 4: Bridge Burn & Unlock (US2 — Return to Native)

After the position is ACTIVE on the Hub, the bank requests a burn which unlocks native tokens back on the spoke.

```mermaid
sequenceDiagram
    autonumber
    participant BANK_A as Bank A<br/>(API Gateway :18080)
    participant PO_A as Payment Orchestrator<br/>(Spoke-A)
    participant DB as PostgreSQL
    participant BRIDGE_A as SpokeBridge<br/>(Spoke-A)
    participant TOKEN_A as tCeBM_BRL<br/>(Spoke-A Token)
    participant RELAYER as Relayer Worker
    participant CACTI as Cacti HTLC Relay<br/>(:4000)
    participant HUB_TOKEN as tCeBM_BRL<br/>(Hub Token)

    Note over BANK_A,DB: 1. Burn Request (position must be ACTIVE)
    BANK_A->>PO_A: POST /api/v2/bridge/burn-unlock<br/>{position_id: "pos-123"}

    PO_A->>DB: SELECT WHERE position_id="pos-123"<br/>AND state=ACTIVE
    DB-->>PO_A: BridgedAssetPosition (ACTIVE)

    PO_A->>BRIDGE_A: unlock(pos-123, spoke-a, mBRL-CBDC, 5000)
    BRIDGE_A-->>PO_A: UnlockResult{tx_hash: "0xdef..."}

    Note over PO_A,DB: 2. Transition to BURNING
    PO_A->>DB: UPDATE position SET state=BURNING
    PO_A->>DB: INSERT RelayerQueueItem<br/>{event_type=BURN_UNLOCK, state=PENDING}
    PO_A-->>BANK_A: {position_id: "pos-123",<br/>bridge_state: "BURNING"}

    Note over RELAYER,HUB_TOKEN: 3. Relayer Worker Processes Burn
    RELAYER->>DB: SELECT pending BURN_UNLOCK items
    RELAYER->>CACTI: SubmitBurnEvent(idempotency_key, pos-123)

    CACTI->>HUB_TOKEN: burn(bank_a_hub_address, 5000)
    HUB_TOKEN-->>CACTI: tx_hash_burn

    Note over CACTI,TOKEN_A: 4. Cacti Confirms Unlock on Spoke
    CACTI->>BRIDGE_A: confirmUnlock(pos-123)
    BRIDGE_A->>TOKEN_A: transfer(bridge, bank_a, 5000)
    TOKEN_A-->>BRIDGE_A: success
    BRIDGE_A-->>CACTI: unlocked

    CACTI-->>RELAYER: {status: "burned"}
    RELAYER->>DB: UPDATE position SET state=BURNED
    RELAYER->>DB: UPDATE queue_item SET state=DONE

    Note over BANK_A,DB: 5. Bank Confirms Final State
    BANK_A->>PO_A: GET /api/v2/bridge/positions
    PO_A-->>BANK_A: {positions: [{position_id: "pos-123",<br/>bridge_state: "BURNED"}]}
```

---

## Phase 5: Circuit Breaker & Governance (US3)

Central Banks can halt/resume the AMM via asymmetric multi-sig circuit breaker.

```mermaid
sequenceDiagram
    autonumber
    participant CB_A as Central Bank A<br/>(API Gateway :38080)
    participant CB_B as Central Bank B<br/>(API Gateway :48080)
    participant PO as Payment Orchestrator
    participant HUB_AMM as Hub AMM<br/>(chain 1338)
    participant IR as IdentityRegistry<br/>(Hub Governance)

    Note over CB_A,HUB_AMM: Emergency Pause (single CB, immediate)
    CB_A->>PO: POST /api/v2/governance/circuit-breaker/pause<br/>{pair: "BRL-USD", reason_code: "INCIDENT_001"}
    PO->>IR: canGovern(CB_A_address) → true
    PO->>HUB_AMM: pause()
    HUB_AMM-->>PO: tx_hash
    PO-->>CB_A: {status: "paused", tx_hash}

    Note over CB_A,IR: Resume Proposal (requires 2-of-N)
    CB_A->>PO: POST /api/v2/governance/circuit-breaker/resume-request<br/>{pair: "BRL-USD"}
    PO-->>CB_A: {request_id: "req-456", signatures: 1/2}

    Note over CB_B,HUB_AMM: Second Signature (different CB)
    CB_B->>PO: POST /api/v2/governance/circuit-breaker/resume-sign<br/>{request_id: "req-456"}
    PO->>IR: canGovern(CB_B_address) → true
    PO->>HUB_AMM: unpause()
    HUB_AMM-->>PO: tx_hash
    PO-->>CB_B: {status: "resumed", signatures: 2/2}
```

---

## Complete End-to-End State Machine

```mermaid
stateDiagram-v2
    [*] --> LOCKING: POST /bridge/lock-mint
    LOCKING --> ACTIVE: Cacti confirms mint on Hub
    ACTIVE --> SWAPPABLE: tokens available on Hub AMM
    SWAPPABLE --> SWAPPED: POST /amm/swap/exact-output
    ACTIVE --> BURNING: POST /bridge/burn-unlock
    BURNING --> BURNED: Cacti confirms burn + spoke unlock
    BURNED --> [*]: Native tokens returned

    LOCKING --> FAILED: Lock reverted / timeout
    BURNING --> FAILED: Burn reverted / timeout
    FAILED --> [*]

    state ACTIVE {
        [*] --> HubBalance
        HubBalance --> AddedToPool: addLiquidity
        AddedToPool --> HubBalance: removeLiquidity
    }
```

---

## API Endpoints Summary

| Method | Endpoint | Actor | Description |
|--------|----------|-------|-------------|
| GET | `/api/v2/amm/quote/exact-output` | Commercial Bank | Get required input for desired output |
| POST | `/api/v2/amm/swap/exact-output` | Commercial Bank | Execute swap with slippage protection |
| POST | `/api/v2/amm/liquidity/add` | Central Bank | Seed/add liquidity to pool |
| POST | `/api/v2/amm/liquidity/remove` | Central Bank | Remove liquidity from pool |
| GET | `/api/v2/amm/pool/:pair/status` | Any | Pool reserves and ratio |
| POST | `/api/v2/amm/token/mint-and-approve` | Central Bank | Mint Hub tokens + approve AMM |
| POST | `/api/v2/amm/token/approve-amm` | Commercial Bank | Approve AMM to spend tokens |
| POST | `/api/v2/bridge/lock-mint` | Commercial Bank | Lock spoke tokens, mint on Hub |
| POST | `/api/v2/bridge/burn-unlock` | Commercial Bank | Burn Hub tokens, unlock on spoke |
| GET | `/api/v2/bridge/positions` | Commercial Bank | List bridge positions and states |
| POST | `/api/v2/governance/circuit-breaker/pause` | Central Bank | Emergency halt (1-of-N) |
| POST | `/api/v2/governance/circuit-breaker/resume-request` | Central Bank | Propose resume (1st sig) |
| POST | `/api/v2/governance/circuit-breaker/resume-sign` | Central Bank | Co-sign resume (2-of-N) |

---

## Position State Transitions

| Current State | Event | Next State | Trigger |
|---------------|-------|------------|---------|
| — | `lock-mint` request | `LOCKING` | API call |
| `LOCKING` | Cacti confirms mint | `ACTIVE` | Relayer worker |
| `ACTIVE` | `burn-unlock` request | `BURNING` | API call |
| `BURNING` | Cacti confirms burn | `BURNED` | Relayer worker |
| `LOCKING` | Timeout / revert | `FAILED` | Relayer retry exhausted |
| `BURNING` | Timeout / revert | `FAILED` | Relayer retry exhausted |

---

## Key Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 19 | Pool must have liquidity before swap | Zero-reserve swap is an env config error, not a runtime edge case |
| 20 | Wait for ACTIVE before Burn&Unlock | Position must be minted on Hub before it can be burned back |
| 21 | `lp_id` returned on add liquidity | Enables targeted removal without race conditions |
| 24 | Relayer worker is idempotent | DB queue + idempotency key prevents double-processing on crash recovery |
| FR-030 | Circuit breaker is asymmetric | Pause = 1 CB (immediate); Resume = 2 CBs (multi-sig) |
| SC-022 | Swap p95 ≤ 6 seconds | Performance SLA for the swap pipeline end-to-end |
