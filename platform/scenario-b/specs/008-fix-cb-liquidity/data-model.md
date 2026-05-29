# Data Model: Fix CB Liquidity (Frontend)

**Feature**: `008-fix-cb-liquidity`  
**Date**: 2026-05-21

## Scope

Modelo funcional de frontend para alinhamento de UX entre os apps `bank` e `governance` no Scenario B, sem alterações de schema backend.

## Entity: ScenarioMode

| Field | Type | Values | Notes |
|---|---|---|---|
| `scenario` | union | `"A" \| "B"` | Derivado de `isScenarioB` (`VITE_SCENARIO === "b"`) |
| `isScenarioB` | boolean | `true \| false` | Controla rotas, menu e telas por app |

**Rules**:
- Se `isScenarioB=false`, rotas e menus de Scenario B ficam indisponíveis.
- Se `isScenarioB=true`, superfícies de Scenario B são exibidas por papel de app.

## Entity: AppRoleSurface

| Field | Type | Values |
|---|---|---|
| `app` | enum | `BANK_APP`, `GOVERNANCE_APP` |
| `allowedJourneys` | list | Dependente do app |
| `forbiddenJourneys` | list | Dependente do app |

**Rules**:
- `BANK_APP`: permite quote/swap/transfer/bridge comercial; bloqueia controles de governança.
- `GOVERNANCE_APP`: permite fluxo soberano CB + circuit breaker + oversight; bloqueia swap comercial.

## Entity: SovereignLiquidityFlowState

| Field | Type | Values |
|---|---|---|
| `phase` | enum | `LOCK_MINT`, `BRIDGE_ACTIVE_WAIT`, `COMMIT_SUBMITTED`, `COMMIT_PENDING`, `COMMIT_EXECUTED`, `POOL_ACTIVE_VERIFIED`, `TIMEOUT`, `FAILED` |
| `startedAt` | number | timestamp ms |
| `updatedAt` | number | timestamp ms |
| `warning` | string? | aviso operacional |
| `errorCode` | string? | código canônico de erro |

### State transitions

1. `LOCK_MINT` -> `BRIDGE_ACTIVE_WAIT`
2. `BRIDGE_ACTIVE_WAIT` -> `COMMIT_SUBMITTED` quando bridge ativo
3. `COMMIT_SUBMITTED` -> `COMMIT_PENDING`
4. `COMMIT_PENDING` -> `COMMIT_EXECUTED` quando commit executado
5. `COMMIT_EXECUTED` -> `POOL_ACTIVE_VERIFIED` quando pool ativo
6. `BRIDGE_ACTIVE_WAIT` -> `TIMEOUT` após 120s
7. `COMMIT_PENDING` -> `TIMEOUT` após 60s (com aviso >30s)
8. Qualquer estado -> `FAILED` em erro não recuperável

## Entity: BridgePositionViewModel

| Field | Type | Notes |
|---|---|---|
| `position_id` | string | Chave de correlação para polling |
| `owner_bank_id` | string | Banco dono da posição |
| `spoke_network` | string | spoke-a/spoke-b |
| `native_asset` | string | ativo nativo |
| `mirrored_asset` | string | ativo espelhado no hub |
| `bridge_state` | enum | `LOCKING`, `ACTIVE`, `BURNED`, `UNLOCKED`, `RECONCILIATION_REQUIRED`, `FAILED` |
| `pollingTimedOut` | boolean | timeout operacional visual |

**Validation**:
- polling bridge: intervalo 5s, timeout 120s
- `BRIDGE_POSITION_NOT_ACTIVE`: bloqueia avanço de commit e mantém estado de espera

## Entity: CommitExecutionTracker

| Field | Type | Notes |
|---|---|---|
| `commit_id` | string | ID local |
| `on_chain_commit_id` | string? | referência on-chain |
| `status` | enum | `PENDING`, `EXECUTED`, `MATCHED`, `EXPIRED` |
| `elapsedMs` | number | tempo desde submit |
| `latencyWarning` | boolean | `true` se >30s |
| `uiTimedOut` | boolean | `true` se >60s |

**Validation**:
- polling commit executado: 3s
- warning de latência: >30s
- timeout de UI: 60s

## Entity: CommercialSwapReadiness

| Field | Type | Notes |
|---|---|---|
| `pool_status` | enum | `EMPTY`, `PENDING_COUNTERPART`, `ACTIVE` |
| `circuit_breaker_state` | enum | `LIVE`, `HALTED`, `RESUME_PENDING` |
| `quoteTimestamp` | number? | validade de quote |
| `canSubmitSwap` | boolean | true somente quando pool ativo e não halted |

**Validation**:
- bloquear swap se `pool_status != ACTIVE`
- bloquear swap se circuito `HALTED`
- refresh de quote em 10-15s (stale warning)

## Entity: OperationalErrorMapping

| Error Code | HTTP | UX Action |
|---|---|---|
| `CROSS_CB_MINT_PROHIBITED` | 403 | Bloqueio definitivo do legado G5-cross + instrução fluxo soberano |
| `BRIDGE_POSITION_NOT_ACTIVE` | 422 | Bloquear commit e ativar espera de bridge |
| `POOL_NOT_ACTIVE` | 422 | Bloquear swap e indicar estado do pool |
| `SLIPPAGE_LIMIT_EXCEEDED` | 422 | Mensagem de mercado movido + ação de refresh quote |
| `INSUFFICIENT_POOL_LIQUIDITY` | 422 | Mensagem para reduzir valor/aguardar liquidez |
| `CIRCUIT_BREAKER_HALTED` | 422 | Bloqueio de swap + aviso regulatório |

## Entity: PollingPolicy

| Flow | Interval | Timeout | UI note |
|---|---|---|---|
| Bridge ACTIVE | 5s | 120s | timeout explícito por posição |
| Commit EXECUTED | 3s | 60s | aviso >30s |
| Quote refresh | 10-15s | N/A | stale warning |
| Circuit breaker status | 15s | N/A | último estado conhecido em falha |

## Compatibility Model (Scenario A)

| Surface | Scenario A Expected |
|---|---|
| bank routes/sidebar | Sem rotas/menus Scenario B |
| governance routes/sidebar | Sem rotas/menus Scenario B |
| Scenario A pages | comportamento preservado |

**Invariant**: alterações de 008 não podem tornar rotas Scenario B acessíveis quando `isScenarioB=false`.
