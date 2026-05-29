# Data Model: Commercial Cross-Currency Swap

**Feature**: 009-commercial-cross-currency-swap  
**Phase**: Phase 1 - Data & Entity Design  
**Date**: 2026-05-26

## Overview

Este documento define entidades, relacionamentos e regras de validação para o fluxo de swap cross-currency. Todas as entidades são agnósticas de implementação (não especificam SQL/NoSQL/ORM).

---

## Entity: CrossCurrencySwapOperation

**Purpose**: Rastrear operação end-to-end de swap cross-currency, vinculando as 3 sub-operações (bridge-in, swap Hub, bridge-out) via correlation_id para auditoria completa.

### Attributes

| Attribute | Type | Constraints | Description |
|-----------|------|-------------|-------------|
| `swap_id` | UUID | Primary key, generated | Identificador único da operação |
| `correlation_id` | UUID | Unique, indexed | Vincula bridge_in + swap + bridge_out |
| `payer_bank_id` | String | Not null, indexed | Bank ID do pagador (comercial bank no Spoke-A) |
| `beneficiary_bank_id` | String | Not null | Bank ID do beneficiário (no Spoke-B) |
| `source_currency` | String | Not null | Ex: "BRL" (ativo no Spoke-A) |
| `target_currency` | String | Not null | Ex: "ARS" (ativo no Spoke-B) |
| `pool_pair` | String | Not null | Ex: "W-BRL-ARS" (pool usado no Hub) |
| `amount_in` | Decimal(78,0) | Not null, >0 | Montante pago em source_currency (wei) |
| `amount_out` | Decimal(78,0) | Not null, >0 | Montante recebido em target_currency (wei) |
| `max_amount_in` | Decimal(78,0) | Not null | Limite de slippage (máximo payer aceita pagar) |
| `effective_rate` | Float | Computed | amount_out / amount_in (taxa efetiva do swap) |
| `status` | Enum | Not null | QUOTING \| BRIDGE_IN_PROGRESS \| SWAP_IN_PROGRESS \| BRIDGE_OUT_PROGRESS \| COMPLETED \| FAILED |
| `bridge_in_position_id` | UUID | Nullable | Referência a BridgePosition (Spoke-A → Hub) |
| `swap_tx_hash` | String | Nullable | Hash da transação de swap no Hub AMM |
| `bridge_out_position_id` | UUID | Nullable | Referência a BridgePosition (Hub → Spoke-B) |
| `quote_id` | UUID | Nullable | Referência a SwapQuote usada (validação expiry) |
| `failure_reason` | String | Nullable | Error message se status=FAILED |
| `created_at` | Timestamp | Not null, indexed | Momento de início da operação |
| `completed_at` | Timestamp | Nullable | Momento de conclusão (success ou failure) |

### State Transitions

```
QUOTING (inicial)
  ↓ (quote válida obtida + pré-condições OK)
BRIDGE_IN_PROGRESS (lock BRL no Spoke-A, mint W-BRL no Hub)
  ↓ (bridge_state=ACTIVE)
SWAP_IN_PROGRESS (swap W-BRL → W-ARS no Hub AMM)
  ↓ (swap tx confirmed on-chain)
BRIDGE_OUT_PROGRESS (burn W-ARS no Hub, unlock ARS no Spoke-B)
  ↓ (bridge_state=UNLOCKED)
COMPLETED (sucesso)

Qualquer etapa pode transicionar para:
FAILED (erro não recuperável: slippage, circuit breaker, pool not active, rollback failure)
```

### Validation Rules

- `amount_in` MUST be ≤ `max_amount_in` (slippage protection)
- `effective_rate` MUST be computed como `amount_out / amount_in` (valor exato post-swap)
- `status` transitions são unidirecionais: não pode voltar de FAILED para COMPLETED
- `bridge_in_position_id`, `swap_tx_hash`, `bridge_out_position_id` devem ser populados conforme operação progride (tracking de sub-operações)
- Se `status=FAILED`, `failure_reason` MUST NOT be null

### Relationships

- **1:1 com SwapQuote**: Uma operação usa exatamente uma quote (foreign key `quote_id`)
- **1:1 com BridgePosition (in)**: Referência a posição de bridge Spoke-A → Hub
- **1:1 com BridgePosition (out)**: Referência a posição de bridge Hub → Spoke-B
- **N:1 com CommercialBank (payer)**: Muitas operações por banco pagador (indexed por `payer_bank_id`)

---

## Entity: SwapQuote

**Purpose**: Armazenar cotação de swap com timestamp de validade (15s TTL) para validação server-side de expiry e proteção contra replay attacks com quotes antigas.

### Attributes

| Attribute | Type | Constraints | Description |
|-----------|------|-------------|-------------|
| `quote_id` | UUID | Primary key, generated | Identificador único da quote |
| `pool_pair` | String | Not null | Ex: "W-BRL-ARS" |
| `amount_out` | Decimal(78,0) | Not null, >0 | Montante desejado pelo usuário (target currency) |
| `amount_in` | Decimal(78,0) | Not null, >0 | Montante requerido calculado via x·y=k + fee |
| `effective_rate` | Float | Computed | amount_out / amount_in |
| `fee_bps` | Int | Not null | Taxa do pool em basis points (ex: 30 = 0.3%) |
| `max_slippage_pct` | Float | Not null | Tolerância de slippage do usuário (ex: 0.01 = 1%) |
| `reserve_a_snapshot` | Decimal(78,0) | Not null | Reserve A no momento da quote (auditoria) |
| `reserve_b_snapshot` | Decimal(78,0) | Not null | Reserve B no momento da quote (auditoria) |
| `created_at` | Timestamp | Not null | Timestamp de geração da quote (server UTC) |
| `valid_until` | Timestamp | Not null, indexed | created_at + 15 seconds (expiry enforcement) |
| `expired` | Bool | Computed | `NOW() > valid_until` (propriedade computed para queries) |

### Validation Rules

- `amount_in` MUST be calculado usando fórmula constant-product AMM:  
  ```
  amount_in_no_fee = (reserve_in × amount_out) / (reserve_out - amount_out)
  amount_in = amount_in_no_fee / (1 - fee_bps/10000)
  ```
- `valid_until` MUST be exactly `created_at + 15 seconds` (não configurável)
- Quote CANNOT be used if `NOW() > valid_until` (server-side validation obrigatória)
- `max_slippage_pct` MUST be ≥0 e ≤1.0 (0% a 100%)

### Lifecycle

- **Creation**: Gerada via `GET /api/v2/amm/quote/exact-output` endpoint
- **Usage**: Referenciada por `CrossCurrencySwapOperation.quote_id` durante execução
- **Expiry**: Quote expirada não pode ser usada (validação em CrossCurrencySwapOrchestrator)
- **Cleanup**: Background job deleta quotes com `expired=true AND created_at < NOW() - INTERVAL '1 hour'` (evitar acúmulo de quotes antigas)

### Relationships

- **1:N com CrossCurrencySwapOperation**: Uma quote pode ser tentada múltiplas vezes (retries) mas apenas uma operação COMPLETED por quote

---

## Entity: SwapRollbackLog

**Purpose**: Registrar tentativas de rollback (bridge reverso Hub→Spoke-A) quando swap falha após bridge-in bem-sucedido. Usado para auditoria e retry manual se rollback automático falhar.

### Attributes

| Attribute | Type | Constraints | Description |
|-----------|------|-------------|-------------|
| `rollback_id` | UUID | Primary key, generated | Identificador único do rollback |
| `swap_operation_id` | UUID | Not null, foreign key | Referência a CrossCurrencySwapOperation que falhou |
| `bridge_in_position_id` | UUID | Not null | Posição de bridge que precisa ser revertida |
| `rollback_status` | Enum | Not null | PENDING \| IN_PROGRESS \| COMPLETED \| FAILED |
| `burn_tx_hash` | String | Nullable | Hash da transação de burn no Hub (bridge reverso) |
| `unlock_tx_hash` | String | Nullable | Hash da transação de unlock no Spoke-A (via Relayer) |
| `failure_reason` | String | Nullable | Erro se rollback_status=FAILED |
| `retry_count` | Int | Not null, default 0 | Número de tentativas de retry (máximo 3) |
| `created_at` | Timestamp | Not null | Momento da primeira tentativa de rollback |
| `completed_at` | Timestamp | Nullable | Momento de conclusão do rollback (success/failure definitivo) |

### Validation Rules

- `rollback_status` transitions: PENDING → IN_PROGRESS → (COMPLETED | FAILED)
- Se `rollback_status=FAILED`, `failure_reason` MUST NOT be null
- `retry_count` MUST be ≤3 (máximo 3 tentativas automáticas)
- Se `retry_count=3 AND rollback_status=FAILED`, marcar para intervenção manual

### Relationships

- **N:1 com CrossCurrencySwapOperation**: Uma operação pode ter zero ou um rollback (0..1 cardinality)

---

## Entity: SwapRateLimitCounter

**Purpose**: Rastrear contadores de rate limiting por banco comercial (10 swaps/min, 100 swaps/hora) para prevenir abuso.

**Note**: Esta é uma solução MVP baseada em database. Produção deve migrar para Redis (ver research.md Q4).

### Attributes

| Attribute | Type | Constraints | Description |
|-----------|------|-------------|-------------|
| `id` | UUID | Primary key | Identificador único do registro |
| `bank_id` | String | Not null, indexed | Bank ID do comercial bank |
| `window_type` | Enum | Not null | MINUTE \| HOUR (tipo de janela de tempo) |
| `window_start` | Timestamp | Not null, indexed | Início da janela (truncated to minute/hour) |
| `swap_count` | Int | Not null, default 0 | Contador de swaps na janela |
| `last_updated` | Timestamp | Not null | Última atualização do contador |

### Validation Rules

- `swap_count` MUST be ≤10 para `window_type=MINUTE`
- `swap_count` MUST be ≤100 para `window_type=HOUR`
- Se limite excedido, retornar HTTP 429 com `Retry-After` header

### Lifecycle

- **Increment**: A cada swap iniciado, incrementar contador para janelas MINUTE e HOUR correntes
- **Cleanup**: Background job deleta registros com `window_start < NOW() - INTERVAL '2 hours'` (janelas expiradas)
- **Query**: Antes de aceitar swap, validar se `swap_count < limit` para ambas as janelas

### Composite Unique Constraint

`UNIQUE(bank_id, window_type, window_start)` — garante um registro por janela de tempo

---

## Derived/Computed Properties

Estas propriedades são calculadas em runtime, não armazenadas:

### CrossCurrencySwapOperation

- **duration_seconds**: `completed_at - created_at` (latência total da operação, usado para SC-002)
- **is_within_latency_target**: `duration_seconds ≤ 90` (validação de SC-002)

### SwapQuote

- **time_remaining_seconds**: `valid_until - NOW()` (countdown para frontend)
- **expired**: `NOW() > valid_until` (usado em queries de validação)

---

## Database Indexes (Recommendations)

Para otimizar queries frequentes:

### CrossCurrencySwapOperation
- `CREATE INDEX idx_correlation_id ON cross_currency_swap_operations(correlation_id)`
- `CREATE INDEX idx_payer_bank_created ON cross_currency_swap_operations(payer_bank_id, created_at DESC)`
- `CREATE INDEX idx_status ON cross_currency_swap_operations(status) WHERE status IN ('FAILED', 'BRIDGE_OUT_PROGRESS')`

### SwapQuote
- `CREATE INDEX idx_valid_until ON swap_quotes(valid_until) WHERE expired=false`
- `CREATE INDEX idx_created_at ON swap_quotes(created_at)` (para cleanup job)

### SwapRateLimitCounter
- `CREATE INDEX idx_bank_window ON swap_rate_limit_counters(bank_id, window_type, window_start)`

---

## Audit Trail Queries

### Query 1: Trace completo de uma operação por correlation_id
```sql
SELECT 
    s.swap_id, s.status, s.amount_in, s.amount_out,
    s.bridge_in_position_id, s.swap_tx_hash, s.bridge_out_position_id,
    q.amount_in AS quote_amount_in, q.valid_until,
    r.rollback_status, r.failure_reason AS rollback_failure
FROM cross_currency_swap_operations s
LEFT JOIN swap_quotes q ON s.quote_id = q.quote_id
LEFT JOIN swap_rollback_logs r ON s.swap_id = r.swap_operation_id
WHERE s.correlation_id = '<uuid>';
```

### Query 2: Operações de um banco em janela de tempo (rate limit check)
```sql
SELECT COUNT(*) 
FROM cross_currency_swap_operations
WHERE payer_bank_id = 'bank-a' 
  AND created_at > NOW() - INTERVAL '1 minute';
```

### Query 3: Operações falhadas que requerem intervenção manual
```sql
SELECT s.swap_id, s.correlation_id, s.failure_reason,
       r.rollback_status, r.retry_count
FROM cross_currency_swap_operations s
LEFT JOIN swap_rollback_logs r ON s.swap_id = r.swap_operation_id
WHERE s.status = 'FAILED'
  AND (r.rollback_status = 'FAILED' OR r.retry_count >= 3);
```

---

## Next Steps

Com data model completo, prosseguir para:
- **contracts/cross-currency-swap-api.md**: Definir contratos HTTP (request/response schemas)
- **quickstart.md**: Tryout manual E2E
