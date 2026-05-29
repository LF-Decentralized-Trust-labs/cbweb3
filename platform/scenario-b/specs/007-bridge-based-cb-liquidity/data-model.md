# Data Model: Bridge-Based CB Liquidity (spec-007)

**Feature**: `007-bridge-based-cb-liquidity`  
**Branch**: `007-bridge-based-cb-liquidity`  
**Date**: 2026-05-20

---

## Entidades Existentes (sem alteração de schema)

As tabelas abaixo já existem (spec-002, spec-005) e são reutilizadas sem modificação de schema:

### BridgedAssetPosition (payment_orchestrator DB)

| Campo | Tipo | Descrição |
|---|---|---|
| `position_id` | UUID PK | Identificador único da posição |
| `owner_bank_id` | string | `client_id` do JWT do CB |
| `spoke_network` | string | `"spoke-a"` \| `"spoke-b"` |
| `native_asset` | string | Endereço do tCeBM no spoke |
| `mirrored_asset` | string | Endereço do W-tCeBM no Hub **← campo-chave para spec-007** |
| `mirrored_amount` | string | Quantidade em wei |
| `bridge_state` | enum | `LOCKING → ACTIVE → BURNING → BURNED → RELEASED \| RECONCILIATION_REQUIRED` |
| `first_attempt_at` | timestamp | |
| `last_attempt_at` | timestamp | |

**Uso em spec-007**: O handler de `/commit` verifica `BridgedAssetPosition` com `bridge_state = ACTIVE` e `mirrored_asset = w_token_address` do commit antes de aceitar.

---

### PoolCommit (api_gateway DB)

| Campo | Tipo | Descrição |
|---|---|---|
| `commit_id` | UUID PK | Identificador local |
| `provider_id` | string | `client_id` do JWT do CB (validado contra JWT — FR-009) |
| `pool_pair` | string | `"W-BRL-ARS"` (novo par soberano) |
| `side` | string | `"A"` \| `"B"` |
| `amount` | string | Quantidade em wei |
| `status` | enum | `PENDING → MATCHED → EXECUTED \| RECONCILIATION_REQUIRED \| EXPIRED` |
| `on_chain_commit_id` | bytes32 | `commit_id` retornado pelo `LiquidityCommitRegistry.registerCommit` |
| `expires_at` | timestamp | `now + 72h` (herdado de spec-005) |

**Novo campo**: `on_chain_commit_id` (bytes32) — referência ao commit on-chain no `LiquidityCommitRegistry`. Pode ser adicionado como migração não-breaking (nullable, backfilled como null para commits existentes do par antigo).

---

### LiquidityPosition (api_gateway DB)

| Campo | Tipo | Descrição |
|---|---|---|
| `position_id` | UUID PK | |
| `provider_id` | string | `client_id` do CB (validado contra JWT em removeLiquidity — FR-007) |
| `pool_pair` | string | `"W-BRL-ARS"` para posições soberanas |
| `token_address` | string | W-tCeBM depositado |
| `amount` | string | Quantidade depositada |
| `shares` | string | LP shares recebidos |
| `status` | enum | `ACTIVE → WITHDRAWN` |

Sem alteração de schema.

---

## Nova Entidade On-Chain: LiquidityCommitRegistry

**Tipo**: Contrato Solidity deployado no Hub  
**Endereço**: Determinado após deploy (variável de ambiente `LIQUIDITY_COMMIT_REGISTRY_ADDRESS`)

### Estado on-chain (storage)

```solidity
struct Commit {
    address signer;          // msg.sender que registrou o commit
    address wTokenAddress;   // W-tCeBM correspondente
    uint256 amount;          // em wei
    uint256 expiresAt;       // block.timestamp + 72h
    CommitStatus status;     // PENDING | MATCHED | EXPIRED | CANCELLED
}

enum CommitStatus { PENDING, MATCHED, EXPIRED, CANCELLED }

// pool_pair → side ("A"|"B") → Commit
mapping(bytes32 => mapping(uint8 => Commit)) public commits;
// commit_id → Commit
mapping(bytes32 => Commit) public commitById;
```

### Transições de estado on-chain

```
registerCommit()  → PENDING
cancelCommit()    → CANCELLED     (somente msg.sender == commit.signer)
(auto no match)   → MATCHED       (ambos os lados PENDING → CommitMatched emitido)
expireCommit()    → EXPIRED       (anyone, após expiresAt)
```

---

## Agregação Lógica: SovereignLiquidityDeposit

Esta não é uma entidade persistida — é uma visão de estados rastreada no DB do gateway para reconciliação e auditoria.

### Máquina de estados

```
BRIDGE_PENDING
    ↓ (bridge_state = ACTIVE na BridgedAssetPosition)
BRIDGE_ACTIVE
    ↓ (commit registrado localmente + on-chain)
COMMIT_PENDING
    ↓ (CommitMatched emitido pelo LiquidityCommitRegistry)
COMMIT_MATCHED
    ↓ (addSingleSidedLiquidity confirmado on-chain)
EXECUTED
    ↓ (timeout: 300s sem execução após COMMIT_MATCHED)
RECONCILIATION_REQUIRED
```

### Campos de rastreamento (não persistidos; inferidos do estado do gateway)

| Campo | Fonte |
|---|---|
| `bridge_state` | `BridgedAssetPosition.bridge_state` |
| `commit_status` | `PoolCommit.status` |
| `position_status` | `LiquidityPosition.status` |
| `on_chain_commit_id` | `PoolCommit.on_chain_commit_id` |

---

## Novo Serviço: CommitMatchedWatcher

**Tipo**: Módulo TypeScript em `interop/hub-and-spoke/cacti/src/liquidity-commit-watcher.ts`  
**Responsabilidade**: Assistir eventos `CommitMatched` no Hub e notificar o gateway local via HTTP interno.

### Contrato de notificação (POST interno)

```
POST /internal/amm/execute-matched-commit
Authorization: INTERNAL_RELAY_AUTH_SECRET (header existente)
Body:
{
  "pool_pair": "W-BRL-ARS",
  "commit_id_a": "0x...",
  "signer_a": "0x...",
  "amount_a": "100000000000000000000",
  "commit_id_b": "0x...",
  "signer_b": "0x...",
  "amount_b": "100000000000000000000"
}
```

O gateway verifica se `signer_a == local_cb_signer` (para executar side A) ou `signer_b == local_cb_signer` (para executar side B). Se nem A nem B pertencer ao CB local, o evento é ignorado.

---

## Entidades de Deploy (não persistidas no DB)

### W-tCeBM_BRL (Hub)
- **Contrato**: `TokenizedCentralBankMoney` (novo deploy)
- **Símbolo**: `W-tCeBM_BRL`
- **CENTRAL_BANK_ROLE**: Bridge Relayer address
- **setCentralBankOf(addr, CB_A_hub_signer)**: necessário para `PairRegistry.proposePair`

### W-tCeBM_ARS (Hub)
- **Contrato**: `TokenizedCentralBankMoney` (novo deploy)
- **Símbolo**: `W-tCeBM_ARS`
- **CENTRAL_BANK_ROLE**: Bridge Relayer address
- **setCentralBankOf(addr, CB_B_hub_signer)**: necessário para `PairRegistry.confirmPair`

### AMM Soberano (Hub)
- **Contrato**: `AutomatedMarketMaker` (novo deploy)
- **tokenA**: `W-tCeBM_BRL_address`
- **tokenB**: `W-tCeBM_ARS_address`
- **pairId**: `"W-BRL-ARS"` (registrado via `PairRegistry.proposePair` + `confirmPair`)

---

## Variáveis de Ambiente (novos)

| Variável | Valor | Serviço |
|---|---|---|
| `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` | `0x...` (após deploy) | api-gateway, watcher |
| `SOVEREIGN_PAIR_AMM_MAP` | JSON `{"W-BRL-ARS":"0x...","W-BRL-CLP":"0x..."}` | api-gateway, watcher |
| `SOVEREIGN_PAIR_IDS` | lista separada por vírgula: `W-BRL-ARS,W-BRL-CLP` | api-gateway |
| `W_TOKEN_BRL_ADDRESS` | `0x...` (após deploy) | api-gateway, watcher |
| `W_TOKEN_ARS_ADDRESS` | `0x...` (após deploy) | api-gateway, watcher |
| `HUB_RPC_URL` | já existente | watcher |

> **Nota (C1 — remediação 2026-05-20)**: As env vars `SOVEREIGN_AMM_ADDRESS` (endereço único) e `SOVEREIGN_POOL_PAIR_ID` (par único) foram removidas e substituídas por `SOVEREIGN_PAIR_AMM_MAP` (mapa JSON) e `SOVEREIGN_PAIR_IDS` (lista). Isso permite que um gateway participe de múltiplos pares soberanos simultaneamente sem refatoração de código.
