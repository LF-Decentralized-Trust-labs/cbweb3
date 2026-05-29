# Data Model: Provisão Cooperativa de Liquidez no AMM

**Feature**: `005-cooperative-liquidity`
**Date**: 2026-05-14
**Derived from**: [research.md](research.md) — Decisões D1–D8

---

## 1. Entidades Novas

### 1.1 `PoolCommit`

Registra a intenção de depósito de um provedor de liquidez antes da transferência de fundos. Criada no momento em que o CB chama `POST /api/v2/amm/liquidity/commit`. Expirada automaticamente após 72h se não houver contraparte.

```
PoolCommit
├── commit_id       : UUID (PK, gerado pelo backend)
├── pool_pair       : VARCHAR(20) — ex.: "BRL-USD" (índice)
├── provider_id     : VARCHAR(100) — identificador do CB ou MLP (índice)
├── side            : CHAR(1) — "A" (token_a) ou "B" (token_b)
├── amount          : NUMERIC(78, 0) — quantidade em menor unidade do token
├── status          : VARCHAR(20) — PENDING | MATCHED | EXECUTED | EXPIRED
├── counterpart_commit_id : UUID (FK → PoolCommit, nullable)
├── created_at      : TIMESTAMPTZ NOT NULL DEFAULT now()
└── expires_at      : TIMESTAMPTZ NOT NULL — DEFAULT created_at + INTERVAL '72 hours'
```

**Transições de status**:
```
PENDING → MATCHED   (quando contraparte registra commit para o mesmo pool_pair)
MATCHED → EXECUTED  (quando ambas as chamadas on-chain são confirmadas)
PENDING → EXPIRED   (worker de expiração: expires_at < now() AND status = PENDING)
MATCHED → EXPIRED   (caso raro: contraparte expirou antes da execução on-chain)
```

**Invariantes**:
- Para cada `pool_pair` só pode existir no máximo **um** commit `PENDING` ou `MATCHED` por `side`.
- Um commit `EXECUTED` ou `EXPIRED` é imutável (append-only).

---

### 1.2 `LPFeeEvent`

Registro append-only de cada taxa de swap gerada, com a distribuição percentual por LP ativo no momento do swap. Usado para auditoria e para cálculo de `fee_claim_accumulated` na retirada.

```
LPFeeEvent
├── event_id        : UUID (PK)
├── pool_pair       : VARCHAR(20)
├── swap_order_id   : VARCHAR(100) (FK → swap_orders.order_id — referência ao swap gerador)
├── fee_amount_a    : NUMERIC(78, 0) — taxa em unidades de token_a
├── fee_amount_b    : NUMERIC(78, 0) — taxa em unidades de token_b
├── distribution    : JSONB — { "lp_id_1": "60.00", "lp_id_2": "40.00", ... }
│                     (percentual com 4 casas decimais; soma = 100)
└── created_at      : TIMESTAMPTZ NOT NULL DEFAULT now()
```

**Semântica**:
- `distribution` é um snapshot imutável das participações no momento do swap.
- Tabela particionada por mês (`created_at`) seguindo padrão de `PoolStateReading`.
- Sem UPDATE ou DELETE — triggers de auditoria idênticos ao padrão existente.

---

## 2. Entidades Modificadas

### 2.1 `LiquidityPosition` (modificações)

Colunas adicionadas à tabela `liquidity_positions`:

```diff
  LiquidityPosition
  ├── lp_id                    : UUID (PK) — sem alteração
  ├── provider_bank_id         : VARCHAR — sem alteração
  ├── pool_pair                : VARCHAR — sem alteração
  ├── token_a_contributed      : NUMERIC — sem alteração
  ├── token_b_contributed      : NUMERIC — sem alteração
  ├── lp_shares                : VARCHAR — mantido (informacional, legado)
  ├── status                   : VARCHAR — sem alteração (ACTIVE | WITHDRAWN)
  ├── added_at                 : TIMESTAMPTZ — sem alteração
  ├── withdrawn_at             : TIMESTAMPTZ (nullable) — sem alteração
+ ├── deposit_side             : VARCHAR(4) NOT NULL DEFAULT 'BOTH'
+ │                              CHECK (deposit_side IN ('A', 'B', 'BOTH'))
+ │                              — 'BOTH': legado (dual-sided); 'A' ou 'B': cooperativo
+ ├── shares_percentage        : DECIMAL(18,4) nullable
+ │                              — participação percentual no pool no momento da adição
+ │                              — NULL para posições legadas (BOTH)
+ ├── fee_claim_accumulated    : NUMERIC(78, 0) NOT NULL DEFAULT 0
+ │                              — crédito total de taxas acumuladas para este LP
+ │                              — atualizado a cada LPFeeEvent que inclua este lp_id
+ └── commit_id                : UUID (FK → PoolCommit, nullable)
+                                — referência ao commit que originou esta posição cooperativa
```

**Regras de retirada por `deposit_side`**:
- `BOTH` → usa caminho legado: `removeLiquidity(token_a_contributed, token_b_contributed)`
- `A` ou `B` → usa caminho proporcional: retorna `reserveA * shares_percentage / 100` e `reserveB * shares_percentage / 100`, mais `fee_claim_accumulated`

---

### 2.2 `PoolState` / `PoolStateReading` (modificações)

Colunas adicionadas para rastrear estado do fee pool:

```diff
  PoolStateReading
  ├── id            : UUID — sem alteração
  ├── pool_pair     : VARCHAR — sem alteração
  ├── reserve_a     : NUMERIC — sem alteração
  ├── reserve_b     : NUMERIC — sem alteração
  ├── current_ratio : DECIMAL — sem alteração
  ├── imbalance_flag: BOOL — sem alteração
  ├── updated_at    : TIMESTAMPTZ — sem alteração
+ ├── fee_rate_bps  : INT NOT NULL DEFAULT 30
+ │                   — taxa atual em basis points (1 bps = 0,01%)
+ ├── total_lp_count: INT — número de LPs ACTIVE no momento da leitura
+ └── pool_status   : VARCHAR(25) NOT NULL DEFAULT 'EMPTY'
+                     CHECK (pool_status IN ('EMPTY', 'PENDING_COUNTERPART', 'ACTIVE'))
```

---

## 3. Contrato Solidity — Mudanças de Estado

### 3.1 `AutomatedMarketMaker.sol`

**Novas variáveis de estado**:
```solidity
uint256 public feeBps;          // taxa em basis points (padrão: 30)
IIdentityRegistry public immutable REGISTRY; // já existe, mas usado no novo modifier
```

**Novo evento**:
```solidity
event LogSingleSidedLiquidityAdded(address indexed provider, bool isTokenA, uint256 amount);
event LogFeeRateUpdated(uint256 oldFeeBps, uint256 newFeeBps);
```

**Nova função**:
```solidity
function addSingleSidedLiquidity(bool isTokenA, uint256 amount)
    external nonReentrant whenNotPaused onlyLiquidityProvider(msg.sender);
```

**Função modificada — `swapExactOutput`**: adiciona dedução de `feeBps` do `amountIn` antes do cálculo da fórmula de produto constante.

**Novo modifier**:
```solidity
modifier onlyLiquidityProvider(address account) {
    if (!REGISTRY.isLiquidityProvider(account)) revert AMM__NotLiquidityProvider();
    _;
}
```

**Função de governança**:
```solidity
function setFeeBps(uint256 newFeeBps) external onlyPauser; // reutiliza papel de governança
```

### 3.2 `IdentityRegistry.sol`

**Novo storage**:
```solidity
mapping(address => bool) private _liquidityProviders;
```

**Novas funções**:
```solidity
function isLiquidityProvider(address account) external view returns (bool);
function grantLiquidityProvider(address account) external onlyAdmin;
function revokeLiquidityProvider(address account) external onlyAdmin;
```

**Novos eventos**:
```solidity
event LogLiquidityProviderGranted(address indexed account);
event LogLiquidityProviderRevoked(address indexed account);
```

---

## 4. Relacionamentos entre Entidades

```
PoolCommit (side=A, pool_pair=BRL-USD)  ←──────────────────┐
PoolCommit (side=B, pool_pair=BRL-USD)  ←── counterpart_commit_id (auto-referencial)
        │
        │ (quando ambos MATCHED → EXECUTED)
        ▼
LiquidityPosition (deposit_side=A, provider=BCB)  ─┐
LiquidityPosition (deposit_side=B, provider=Fed)   ─┴─ mesma pool_pair = BRL-USD
        │
        │ (a cada swap no pool)
        ▼
LPFeeEvent { distribution: { lp_id_bcb: 50, lp_id_fed: 50 } }
        │
        │ (acumulado em)
        ▼
LiquidityPosition.fee_claim_accumulated (atualizado por trigger ou worker)
```

---

## 5. Migrações de Schema (SQL)

```sql
-- Migration: 005_cooperative_liquidity.sql

-- 1. PoolCommit
CREATE TABLE pool_commits (
    commit_id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_pair            VARCHAR(20) NOT NULL,
    provider_id          VARCHAR(100) NOT NULL,
    side                 CHAR(1) NOT NULL CHECK (side IN ('A', 'B')),
    amount               NUMERIC(78, 0) NOT NULL,
    status               VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                           CHECK (status IN ('PENDING', 'MATCHED', 'EXECUTED', 'EXPIRED')),
    counterpart_commit_id UUID REFERENCES pool_commits(commit_id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at           TIMESTAMPTZ NOT NULL DEFAULT (now() + INTERVAL '72 hours')
);
CREATE INDEX idx_pool_commits_pair_side_status
    ON pool_commits(pool_pair, side, status);

-- Constraint: máx 1 commit PENDING/MATCHED por (pool_pair, side)
CREATE UNIQUE INDEX uniq_pool_commits_active
    ON pool_commits(pool_pair, side)
    WHERE status IN ('PENDING', 'MATCHED');

-- 2. LPFeeEvent (particionada por mês)
CREATE TABLE lp_fee_events (
    event_id       UUID NOT NULL DEFAULT gen_random_uuid(),
    pool_pair      VARCHAR(20) NOT NULL,
    swap_order_id  VARCHAR(100),
    fee_amount_a   NUMERIC(78, 0) NOT NULL DEFAULT 0,
    fee_amount_b   NUMERIC(78, 0) NOT NULL DEFAULT 0,
    distribution   JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
) PARTITION BY RANGE (created_at);
CREATE INDEX idx_lp_fee_events_pool_pair ON lp_fee_events(pool_pair);

-- 3. Alterações em liquidity_positions
ALTER TABLE liquidity_positions
    ADD COLUMN deposit_side          VARCHAR(4) NOT NULL DEFAULT 'BOTH'
        CHECK (deposit_side IN ('A', 'B', 'BOTH')),
    ADD COLUMN shares_percentage     DECIMAL(18, 4),
    ADD COLUMN fee_claim_accumulated NUMERIC(78, 0) NOT NULL DEFAULT 0,
    ADD COLUMN commit_id             UUID REFERENCES pool_commits(commit_id);

-- 4. Alterações em pool_state_readings
ALTER TABLE pool_state_readings
    ADD COLUMN fee_rate_bps  INT NOT NULL DEFAULT 30,
    ADD COLUMN total_lp_count INT,
    ADD COLUMN pool_status   VARCHAR(25) NOT NULL DEFAULT 'EMPTY'
        CHECK (pool_status IN ('EMPTY', 'PENDING_COUNTERPART', 'ACTIVE'));
```

---

## 6. Regras de Validação

| Entidade | Campo | Regra |
|----------|-------|-------|
| `PoolCommit` | `side` | Apenas 'A' ou 'B' — não 'BOTH' |
| `PoolCommit` | `amount` | > 0 |
| `PoolCommit` | `pool_pair` | Deve existir como par configurado no AMM |
| `PoolCommit` | unicidade | Max 1 PENDING/MATCHED por (pool_pair, side) via unique partial index |
| `LiquidityPosition` | `deposit_side = A` | `token_b_contributed = 0` obrigatório |
| `LiquidityPosition` | `deposit_side = B` | `token_a_contributed = 0` obrigatório |
| `LiquidityPosition` | `deposit_side = BOTH` | `token_a_contributed > 0 AND token_b_contributed > 0` |
| `LPFeeEvent` | `distribution` | Soma dos valores percentuais = 100 (com tolerância de 0.01%) |
| AMM Contract | `addSingleSidedLiquidity` | `onlyLiquidityProvider(msg.sender)` |
| AMM Contract | `swapExactOutput` | `reserveA > 0 AND reserveB > 0` antes de executar |

---

## 7. Máquinas de Estado

### PoolCommit
```
PENDING ──────────────────────────────────────────► EXPIRED
   │ (contraparte registra commit para mesmo par)        ▲
   ▼                                                     │ (expires_at < now)
MATCHED ────────────────────────────────────────────────►│
   │ (ambas tx on-chain confirmadas)
   ▼
EXECUTED (terminal)
```

### LiquidityPosition (estendida)
```
             [commit MATCHED]
PoolCommit ──────────────────► ACTIVE
                                  │ (provider chama remove)
                                  ▼
                              WITHDRAWN (terminal)
```

### Pool Status (derivado de reserveA/reserveB)
```
EMPTY ─────────────────────────────────────────────────────────► PENDING_COUNTERPART
  (quando addSingleSidedLiquidity para lado A ou B é chamado)         │
                                                                       │ (outro lado depositado)
                                                                       ▼
                                                                    ACTIVE
                                                                       │ (todos LPs removem)
                                                                       ▼
                                                                    EMPTY
```

---

## 4. Entidades Novas (Sessão 2026-05-18 — Multi-Par PairRegistry)

### 4.1 `PairProposal`

Registra o estado bilateral de criação de um novo par de moedas. Persistida no DB local (D11) para auditabilidade e para servir `GET /api/v2/amm/pairs` sem RPC call. O `PairRegistry` on-chain é a fonte autoritativa; este registro é atualizado pelo event watcher ao receber `PairRegistered`.

```
PairProposal
├── pair_id         : VARCHAR(20) PRIMARY KEY — ex.: "BRL-ARS" (único)
├── proposer_cb     : VARCHAR(100) NOT NULL — provider_id do CB que propôs (emissor de tokenA)
├── confirmer_cb    : VARCHAR(100) — provider_id do CB que confirmou (emissor de tokenB); NULL até confirmação
├── token_a_address : VARCHAR(42) NOT NULL — endereço do contrato TokenizedCentralBankMoney do tokenA
├── token_b_address : VARCHAR(42) NOT NULL — endereço do contrato TokenizedCentralBankMoney do tokenB
├── amm_address     : VARCHAR(42) NOT NULL — endereço do contrato AutomatedMarketMaker para este par
├── status          : VARCHAR(10) NOT NULL DEFAULT 'PROPOSED'
│                     CHECK (status IN ('PROPOSED', 'ACTIVE'))
├── proposed_at     : TIMESTAMPTZ NOT NULL DEFAULT now()
└── confirmed_at    : TIMESTAMPTZ — NULL até confirmação on-chain
```

**Transições de status**:
```
PROPOSED → ACTIVE  (quando CB_B chama confirmPair on-chain; evento PairRegistered recebido pelo watcher)
```

**Invariantes**:
- `pair_id` é único no DB e único no `PairRegistry` on-chain.
- Uma vez `ACTIVE`, a entrada é imutável (append-only para fins de auditoria).
- `amm_address` aponta para um contrato `AutomatedMarketMaker` independente, não compartilhado com outros pares.

---

## 5. Contrato Solidity — PairRegistry.sol (Novo)

### 5.1 Estruturas de Estado

```solidity
enum PairStatus { PROPOSED, ACTIVE }

struct PairEntry {
    string   pairId;       // ex.: "BRL-ARS"
    address  ammAddress;   // AutomatedMarketMaker para este par
    address  tokenA;       // tCeBM do CB proponente
    address  tokenB;       // tCeBM do CB confirmante
    PairStatus status;
    address  proposer;     // endereço on-chain do CB_A
    address  confirmer;    // endereço on-chain do CB_B (address(0) até confirmação)
}

mapping(string => PairEntry) public pairs;
string[] public pairIds;              // índice iterável para getAllActivePairs()
```

### 5.2 Fluxo de Estado On-Chain

```
proposePair(pairId, tokenA, tokenB, ammAddress)
  ├─ requer: pairs[pairId].status não existe (novo par)
  ├─ requer: IIdentityRegistry.getCentralBankOf(tokenA) == msg.sender
  ├─ define: pairs[pairId] = PairEntry { status: PROPOSED, proposer: msg.sender, ... }
  └─ emite:  PairProposed(pairId, msg.sender, tokenA, tokenB)

confirmPair(pairId)
  ├─ requer: pairs[pairId].status == PROPOSED
  ├─ requer: IIdentityRegistry.getCentralBankOf(tokenB) == msg.sender
  ├─ define: pairs[pairId].status = ACTIVE; confirmer = msg.sender
  └─ emite:  PairRegistered(pairId, ammAddress, tokenA, tokenB)
```

### 5.3 Nova View no IdentityRegistry

```solidity
/// @notice Retorna o endereço do CB que é emissor autorizado de um token.
/// @dev    Usado pelo PairRegistry para verificar autoridade bilateral.
function getCentralBankOf(address token) external view returns (address centralBank);
```

---

## 5. MLP Path B — Sem Novas Tabelas

**Data**: 2026-05-18

O MLP opera como mais um provedor de liquidez — sem novas entidades no banco de dados.

### Tabelas Reutilizadas

| Tabela | Campo chave | Valor para MLP |
|--------|-------------|----------------|
| `liquidity_positions` | `provider_bank_id` | `"mlp"` |
| `pool_commits` | `provider_id` | `"mlp"` |
| `lp_fee_events` | `provider_id` | `"mlp"` |

### Nova Database (isolamento de serviço)

O compliance-mlp utiliza a database `cbweb3_mlp` (criada por `init-multi-db.sh`). Ela contém as mesmas tabelas que `cbweb3_central_bank_a` e `cbweb3_central_bank_b` — é apenas isolamento de instância de serviço, não novo schema.

**Invariante**: `SUM(shares_percentage) = 100%` para todos os `LiquidityPosition` com `status=ACTIVE` no mesmo `pool_pair` — inclui o MLP quando ativo.

### Redis DB

Redis DB 6 dedicado ao noncestore do MLP (isolamento de DB já aplicado nos outros serviços).

```
Redis DB 0: (reservado)
Redis DB 2: central-bank-a noncestore
Redis DB 5: central-bank-b noncestore
Redis DB 6: mlp noncestore (novo)
```
