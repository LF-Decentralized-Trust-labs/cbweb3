# Cooperative Liquidity — Technical Overview

**Feature branch**: `005-cooperative-liquidity`  
**Last updated**: 2026-05-19

---

## 1. Context and Motivation

The previous AMM liquidity model required a **single provider to deposit both currencies of a pair simultaneously**. For example, Central Bank A would need to hold and control both `tCeBM-BRL` and `tCeBM-USD` to seed the BRL-USD pool. This violates the monetary sovereignty principle: a Central Bank should only need to control tokens it issued.

**Cooperative Liquidity** solves this by allowing each Central Bank to contribute only its own currency. The pool is formed collectively via a **commit-reveal** protocol, ensuring atomicity and preventing partial-state traps.

---

## 2. Actors Involved

### Central Banks

| Actor | Identifier | API Gateway | Signer Address | Role |
|-------|-----------|-------------|----------------|------|
| **CB-A** (e.g., Banco Central do Brasil — BCB) | `central_bank_a` | `http://localhost:38080` | `0x627306090abaB3A6e1400e9345bC60c78a8BEf57` | Issues `tCeBM-BRL` (Token A); proposes pairs involving BRL |
| **CB-B** (e.g., U.S. Federal Reserve — Fed) | `central_bank_b` | `http://localhost:60080` | `0xf17f52151EbEF6C7334FAD080c5704D77216b732` | Issues `tCeBM-EUR` (Token B); confirms pairs involving EUR |
| **MLP** (opt-in, e.g., IDB) | `mlp` | `http://localhost:68080` | Configurable via `.env.infra.mlp` | Multilateral Liquidity Provider — dual-sided deposits when a CB lacks capacity |

### Commercial Banks

| Actor | Identifier | API Gateway | Role |
|-------|-----------|-------------|------|
| **Bank-A** | `bank_a` | `http://localhost:18080` | Executes swaps against the AMM pool |

---

## 3. Network Topology (Hub-and-Spoke)

```
┌──────────────────────────────────────────────────────────────┐
│                    HUB BESU NETWORK                          │
│                  (local: RPC :8645, ChainID 1338)            │
│                                                              │
│  Contracts:                                                  │
│    IdentityRegistry   0x4269...b210b  (CB registry + auth)  │
│    PairRegistry       0xfeae...438fd  (pair lifecycle)       │
│    tCeBM-BRL          0xa50a...8e77   (Token A — BCB)        │
│    tCeBM-EUR          0x9a3d...c17e   (Token B — Fed)        │
│    AMM BRL-USD        0x05d9...b711   (pool BRL-USD)         │
│                                                              │
│  Hub plays the role of settlement layer.                     │
│  In local dev, Spoke-A node also serves as Hub (same RPC).  │
└──────────────────────────────────────────────────────────────┘
         │                              │
         ▼                              ▼
┌─────────────────┐           ┌─────────────────┐
│  SPOKE-A        │           │  SPOKE-B        │
│  (RPC :8645)    │           │  (RPC :8745)    │
│  CB-A gateway   │           │  CB-B gateway   │
│  Bank-A gateway │           │                 │
│  Paladin spoke-a│           │                 │
│  Lock & Mint    │           │  Burn & Unlock  │
└─────────────────┘           └─────────────────┘
```

**Key**: In local development, Spoke-A shares the same Besu node as the Hub (port 8645). Spoke-B runs on a separate node (port 8745). The bridging (Lock&Mint / Burn&Unlock) uses the Cacti relayer at port 4000.

---

## 4. Core Changes

### 4.1 Commit-Reveal Protocol for Pool Formation (US1)

**Problem**: When a single CB deposits one side of a pair, the pool enters an indefinite `PENDING_COUNTERPART` state with no atomicity guarantee.

**Solution**: A commit-reveal two-phase mechanism:

```
Phase 1 — Commit (CB-A)                Phase 2 — Commit (CB-B triggers reveal)
────────────────────────────           ────────────────────────────────────────────
CB-A POSTs /amm/liquidity/commit       CB-B POSTs /amm/liquidity/commit
  pool_pair = "BRL-USD"                  pool_pair = "BRL-USD"
  side      = "A"                        side      = "B"
  amount    = 100000                     amount    = 100000
  → status  = PENDING                    → status  = EXECUTED  ← auto-matched!
  → pool    = EMPTY (no funds moved)     → pool    = ACTIVE
                                         → two LiquidityPosition records created
                                         → LP shares calculated proportionally
```

**Constraints enforced**:
- `SAME_PROVIDER_BOTH_SIDES` (HTTP 409) — one CB cannot commit both sides
- Commits expire after 72h; pool returns to `EMPTY` if no counterpart arrives
- Swaps blocked (`POOL_NOT_ACTIVE`) until pool is `ACTIVE`

**New DB table**: `pool_commits`

```sql
pool_commits (
  commit_id             UUID PRIMARY KEY,
  pool_pair             VARCHAR(20),
  provider_id           VARCHAR(100),
  side                  CHAR(1),          -- 'A' or 'B'
  amount                NUMERIC(78,0),
  status                VARCHAR(20),      -- PENDING | MATCHED | EXECUTED | EXPIRED
  counterpart_commit_id UUID,
  created_at            TIMESTAMPTZ,
  expires_at            TIMESTAMPTZ       -- created_at + 72h
)
```

---

### 4.2 PairRegistry — Bilateral Pair Approval (US5)

**Problem**: No on-chain registry existed for multi-pair routing. The gateway had no way to discover which AMM contract to use for a given `pool_pair` string, and no governance for adding new pairs.

**Solution**: A new `PairRegistry.sol` contract with bilateral Central Bank approval:

```
CB-A (issuer of tokenA)         CB-B (issuer of tokenB)
POST /amm/pairs/propose   ──►   POST /amm/pairs/confirm
  pair_id    = "BRL-EUR"          pair_id    = "BRL-EUR"
  token_a    = tCeBM-BRL          (signed on-chain by CB-B signer)
  token_b    = tCeBM-EUR          → PairEntry status: PROPOSED → ACTIVE
  amm_addr   = 0x...
  → on-chain: proposePair()
  → local DB: PROPOSED
```

**Authorization model** (enforced on-chain):
- `proposePair()` — requires `msg.sender == IdentityRegistry.getCentralBankOf(tokenA)`
- `confirmPair()` — requires `msg.sender == IdentityRegistry.getCentralBankOf(tokenB)`

**IdentityRegistry extension** (`setCentralBankOf`):
```solidity
// New function — only DEFAULT_ADMIN_ROLE
function setCentralBankOf(address token, address centralBank) external;
function getCentralBankOf(address token) external view returns (address);
```

**New DB table**: `pair_proposals`

```sql
pair_proposals (
  pair_id        VARCHAR(20) PRIMARY KEY,  -- e.g., "BRL-EUR"
  proposer_cb    VARCHAR(100),
  confirmer_cb   VARCHAR(100),
  token_a_address VARCHAR(42),
  token_b_address VARCHAR(42),
  amm_address    VARCHAR(42),
  status         VARCHAR(10),             -- PROPOSED | ACTIVE
  proposed_at    TIMESTAMPTZ,
  confirmed_at   TIMESTAMPTZ
)
```

**New API endpoints**:

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v2/amm/pairs/propose` | CB JWT | CB-A proposes a new pair |
| `POST` | `/api/v2/amm/pairs/confirm` | CB JWT | CB-B confirms the pair |
| `GET`  | `/api/v2/amm/pairs` | Any JWT | List all ACTIVE pairs |

---

### 4.3 PairRouter — Dynamic AMM Routing (US5)

**Problem**: The gateway was hardcoded to a single AMM contract address.

**Solution**: A `PairRouter` goroutine that:
1. Loads all active pairs from `PairRegistry.getAllActivePairs()` on startup
2. Subscribes to `PairRegistered` on-chain events (live cache invalidation)
3. Routes swap/quote/pool-status requests to the correct AMM contract based on `pool_pair` string

---

### 4.4 LP Fee Distribution (US1 / US2)

Each swap now records a `LPFeeEvent`, distributing the 30 bps fee proportionally to all active LP positions at the time of the swap:

```sql
lp_fee_events (
  event_id        UUID PRIMARY KEY,
  pool_pair       VARCHAR(20),
  swap_order_id   VARCHAR(100),
  total_fee_units NUMERIC(78,0),
  distributions   JSONB   -- [{lp_id, share_pct, fee_units}, ...]
)
```

The accumulated fee per LP position is stored in `liquidity_positions.fee_claim_accumulated` and paid out on withdrawal via `RemoveLiquidity`.

---

### 4.5 MLP — Multilateral Liquidity Provider (US2, opt-in)

The MLP (e.g., IDB) can deposit both sides of a pair simultaneously when a CB lacks capacity. It requires a separate stack with its own Keycloak realm, compliance service, auth service, and API gateway.

**Activation**: `ENABLE_MLP=true` in the tryout or via `make dev.up`.

**Infrastructure** (`backend/docker-compose-backend.mlp.yaml`):

```
compliance-mlp   → port 49093
auth-mlp         → port 49091
api-gateway-mlp  → port 68080 (external)
```

**Database**: `cbweb3_mlp` (separate PostgreSQL database)  
**Redis**: DB 6 (isolated nonce store)

---

### 4.6 Cacti Relay — Cross-Spoke Interoperability

O Cacti **não participa** do fluxo de provisionamento cooperativo de liquidez (commit-reveal, PairRegistry, LP fees). Ele atua exclusivamente na camada de **liquidação cross-spoke** dos swaps executados pelos bancos comerciais — a etapa em que os tokens minted no Hub precisam ser bridgeados de volta para o spoke de origem do banco comprador.

#### Onde o Cacti entra

```
Hub (AMM executa swap BRL → EUR)
         │
         ▼
  HTLC.lock() no Spoke-A   ──►  Cacti Relay observa LogHTLCLocked
                                        │
                                        ▼
                               Cacti chama SettleHTLC() via gRPC
                               no payment-orchestrator do Spoke-B
                                        │
                                        ▼
                               HTLC.claim() no Spoke-B
                               (LogHTLCClaimed com secret revelado)
                                        │
                                        ▼
                               Cacti observa LogHTLCClaimed no Spoke-B
                               e propaga secret de volta para Spoke-A
                               via SettleHTLC() gRPC (fecha ciclo)
```

#### Componente: `cbweb3-cacti-relay` (TypeScript / Node 20)

| Arquivo | Responsabilidade |
|---------|-----------------|
| `src/config.ts` | Lê env vars de Spoke-A e Spoke-B (RPC, WS, HTLC address, gRPC target) |
| `src/htlc-relay.ts` | Loop de polling via `PluginLedgerConnectorBesu.getPastLogs`; decodifica `LogHTLCLocked` / `LogHTLCClaimed` com ethers.js; chama `SettleHTLC` gRPC |
| `src/relay-store.ts` | Ring buffer in-memory de eventos + retry queue com backoff exponencial |
| `src/index.ts` | REST API na porta 4000 consumida pelo adapter Go (`CactiRelay`) |

#### API REST exposta pelo relay (porta 4000)

| Método | Path | Consumidor | Descrição |
|--------|------|-----------|-----------|
| `GET` | `/api/v1/relay/events/settle?since=<ms>` | payment-orchestrator Go | Busca eventos `LogHTLCClaimed` por timestamp |
| `GET` | `/api/v1/relay/events/lock?since=<ms>` | payment-orchestrator Go | Busca eventos `LogHTLCLocked` por timestamp |
| `POST` | `/api/v1/relay/proof` | payment-orchestrator Go | Armazena prova de relay (correlationId) |
| `GET` | `/api/v1/relay/proof/:correlationId` | payment-orchestrator Go | Recupera prova de relay |
| `GET` | `/api/v1/health` | Docker healthcheck | Liveness probe |

#### Tecnologias Cacti utilizadas

- **`@hyperledger/cactus-plugin-ledger-connector-besu`** — abstração de ledger; substitui polling direto via `ethers.JsonRpcProvider`. Fornece `getPastLogs` (HTTP) e `watchBlocksV1` (Socket.IO) para ambos os spokes.
- **`@hyperledger/cactus-core`** — `PluginRegistry` para instanciar e registrar os conectores.
- **gRPC** (`@grpc/grpc-js`) — o relay chama `PaymentOrchestratorService.SettleHTLC` diretamente no payment-orchestrator de cada banco, propagando o `secret` revelado no HTLC.

#### Relação com o fluxo cooperativo

| Etapa | Cacti envolvido? | Quem age |
|-------|-----------------|---------|
| Commit-reveal (pool formation) | **Não** | api-gateway → AMM direto |
| PairRegistry propose/confirm | **Não** | api-gateway → PairRegistry direto |
| Swap execution (AMM) | **Não** | api-gateway → AMM direto |
| HTLC lock/claim (cross-spoke bridge) | **Sim** | Cacti relay → payment-orchestrator gRPC |
| Swap settlement (token delivery ao banco) | **Sim** (via SettleHTLC) | payment-orchestrator → HTLC.claim() |

---

## 5. Sequence Diagram — Cooperative Pool Formation (BRL-USD)

```
BCB (CB-A)                  Backend (CB-A)              Besu Hub              Fed (CB-B)           Backend (CB-B)
    │                            │                          │                     │                      │
    │ POST /amm/liquidity/commit │                          │                     │                      │
    │ {side:"A", amount:100000} ─►                          │                     │                      │
    │                            │ INSERT pool_commit       │                     │                      │
    │                            │ status=PENDING           │                     │                      │
    │ ◄── {status:"PENDING"} ────│                          │                     │                      │
    │                            │                          │                     │                      │
    │                            │                          │  POST /amm/liquidity/commit                │
    │                            │                          │  {side:"B", amount:100000} ────────────────►
    │                            │                          │                     │  matching logic      │
    │                            │                          │                     │  addSingleSided(A)──►│
    │                            │                          │◄─ addSingleSided(A) │                      │
    │                            │                          │◄─ addSingleSided(B) │                      │
    │                            │                          │                     │  pool→ACTIVE         │
    │                            │                          │                     │ ◄──{status:"EXECUTED"}│
```

---

## 6. Sequence Diagram — PairRegistry (BRL-EUR)

```
CB-A (BRL issuer)               Backend (CB-A)              PairRegistry.sol    CB-B (EUR issuer)        Backend (CB-B)
     │                               │                            │                    │                       │
     │ POST /amm/pairs/propose       │                            │                    │                       │
     │ {pair_id:"BRL-EUR",          ─►                            │                    │                       │
     │  token_a, token_b, amm_addr}  │ proposePair() ────────────►│                    │                       │
     │                               │                            │ PROPOSED           │                       │
     │ ◄── {status:"PROPOSED"} ──────│                            │                    │                       │
     │                               │                            │                    │                       │
     │                               │                            │ POST /amm/pairs/confirm                    │
     │                               │                            │ {pair_id:"BRL-EUR"} ─────────────────────►│
     │                               │                            │◄── confirmPair() ──────────────────────────│
     │                               │                            │    ACTIVE          │                       │
     │                               │                            │                    │ ◄──{status:"ACTIVE"}──│
     │                               │                            │                    │                       │
     │ GET /amm/pairs                │                            │                    │                       │
     │ ─────────────────────────────►│ ListActivePairs()          │                    │                       │
     │                               │ (lazy on-chain sync) ─────►│ getAllActivePairs() │                       │
     │ ◄── [{pair_id:"BRL-EUR",      │                            │                    │                       │
     │       status:"ACTIVE"}] ──────│                            │                    │                       │
```

---

## 7. Bug Fixes Applied (This Session)

### 7.1 `pair_repo.go` — `Activate` returning `nil` on zero rows

**Root cause**: GORM's `Updates()` returns `nil` error even when 0 rows are matched. When CB-B confirmed a pair proposed via CB-A (no local record in CB-B's DB), `Activate` silently returned `nil`, the sync-from-on-chain block was never entered, and CB-B's `pair_proposals` table stayed empty.

**Fix**: Check `result.RowsAffected == 0` and return `gorm.ErrRecordNotFound`:

```go
result := r.db.WithContext(ctx).Model(...).Where(...).Updates(...)
if result.Error != nil { return result.Error }
if result.RowsAffected == 0 { return gorm.ErrRecordNotFound }
return nil
```

### 7.2 `pair_service.go` — Cross-gateway stale PROPOSED state

**Root cause**: When CB-B confirmed a pair, CB-A's local DB record remained `PROPOSED` indefinitely (no inter-gateway notification).

**Fix**: `ListActivePairs` now performs a **lazy on-chain sync**:
1. If any PROPOSED rows exist in the local DB, call `getAllActivePairs()` on-chain.
2. For each pair that is ACTIVE on-chain but PROPOSED locally, call `Activate` to update the DB.
3. Return the merged result — DB is always consistent with on-chain state after the first `GET /pairs`.

### 7.3 `tryout-scenario-b-e2e.sh` — CB-C removal and step8 rewrite

- Removed all CB-C (Argentina / ARS) references (variables, token acquisition in step3).
- Rewrote step8 (`/US5. PairRegistry`) to use the `BRL-EUR` pair with CB-A (propose) and CB-B (confirm).
- Added automatic `setCentralBankOf` calls in `step2_contracts` so the IdentityRegistry is correctly configured after any chain reset — preventing the silent-zero-address authorization failure.

---

## 8. API Summary

### Liquidity (Cooperative Commit-Reveal)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v2/amm/liquidity/commit` | Register a one-sided liquidity intent |
| `GET`  | `/api/v2/amm/pool/{pair}/status` | Pool reserves, status, fee rate |
| `POST` | `/api/v2/amm/liquidity/remove` | Remove LP position (proportional) |
| `GET`  | `/api/v2/amm/liquidity/positions` | List LP positions (session only) |

### PairRegistry

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v2/amm/pairs/propose` | CB JWT | Propose a new pair (tokenA issuer) |
| `POST` | `/api/v2/amm/pairs/confirm` | CB JWT | Confirm a pair (tokenB issuer) |
| `GET`  | `/api/v2/amm/pairs` | Any JWT | List active pairs (lazy on-chain sync) |
| `GET`  | `/api/v2/amm/quote/exact-output` | Any JWT | Quote swap via PairRouter |

---

## 9. Running the E2E Test

```bash
# Full scenario (all user stories)
cd /path/to/cbweb3-platform
SKIP_UP=1 bash tryouts/tryout-scenario-b-e2e.sh all

# PairRegistry only (US5)
SKIP_UP=1 bash tryouts/tryout-scenario-b-e2e.sh us5
```

Expected result: **8/8 PASS** in step 8, with both CB-A and CB-B showing `"status":"ACTIVE"` after confirmation.
