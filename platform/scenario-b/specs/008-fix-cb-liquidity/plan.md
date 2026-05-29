# Implementation Plan: Fix CB Liquidity (Frontend Alignment Only)

**Branch**: `008-fix-cb-liquidity` | **Date**: 2026-05-21 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/008-fix-cb-liquidity/spec.md`

## Summary

Alinhar exclusivamente os frontends de `bank` e `governance` ao comportamento canônico do Scenario B definido no runbook v6.0 (seções 16 e 19), removendo dependências de UX legada G5-cross no fluxo de CB, padronizando mapeamento de estados/erros oficiais e preservando integralmente o gating de Scenario A sem regressões. O plano reaproveita os alicerces de `004-scenario-b-frontend-integration` (estrutura e gating), `005-cooperative-liquidity` (wizard/commit-reveal e tipos) e `007-bridge-based-cb-liquidity` (fluxo soberano bridge->commit->watcher->pool ACTIVE).

## Technical Context

**Language/Version**: TypeScript 5.9.x, React 19, Node 20 (frontend monorepo)  
**Primary Dependencies**: Vite, React Router, Zustand, Axios, Tailwind + `@cbweb3/ui`, Lucide React  
**Storage**: N/A (estado em memória via Zustand; sem persistência local)  
**Validation**: `eslint`, `tsc --noEmit`, validação manual E2E com tryouts  
**Target Platform**: SPA web (desktop/mobile) via Vite/Nginx no ambiente Linux Docker  
**Project Type**: Web application (somente frontend apps)  
**Performance Goals**:
- Polling `bridge_state=ACTIVE`: 5s, timeout 120s
- Polling `commit.status=EXECUTED`: 3s, timeout de UI 60s com aviso em 30s
- Refresh de quote: 10-15s
**Constraints**:
- Escopo somente frontend (`frontend/apps/bank`, `frontend/apps/governance`) com extensões mínimas de API backend para observabilidade (FR-028: LP positions query, FR-029: simplified API payload)
- Sem mudanças em lógica de negócio core, contratos Solidity, infra ou fluxos funcionais existentes
- Preservar compatibilidade de Scenario A (rotas/menus/fluxos bloqueados quando `VITE_SCENARIO!=b`)
- Fonte de verdade: runbook v6.0 + tryouts oficiais + specs 004/005/007
**Scale/Scope**: Ajustes de alinhamento em páginas/stores/services/types e rotas/sidebar dos dois apps, mantendo interfaces existentes. Extensões backend limitadas a novos endpoints de leitura (LP positions, commits query) sem alteração de fluxo core.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`/.specify/memory/constitution.md` está em formato template sem princípios mandatórios preenchidos. Gates aplicados por escopo desta feature:

| Gate | Status | Observação |
|---|---|---|
| Frontend-only (sem backend changes) | PASS | Escopo restringido a apps `bank` e `governance` |
| Scenario A regression zero | PASS | Gating existente por `isScenarioB` permanece obrigatório |
| Source-of-truth operacional | PASS | Runbook v6.0 + tryouts + specs 004/005/007 usados como referência |
| Sem breaking de API | PASS | Apenas consumo e mapeamento de APIs existentes |

**Re-check pós-design**: PASS. Nenhuma violação identificada.

## Project Structure

### Documentation (this feature)

```text
specs/008-fix-cb-liquidity/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── frontend-scenario-b-alignment.md
└── tasks.md             # Phase 2 (/speckit.tasks), não criado por /speckit.plan
```

### Source Code (repository root)

```text
frontend/apps/bank/src/
├── config/scenario.ts
├── routes/index.tsx
├── components/layout/Sidebar.tsx
├── pages/AMMTradingPage.tsx
├── features/bridge/BridgePage.tsx
├── features/bridge/bridge.store.ts
├── features/amm/amm-v2.store.ts
├── services/api/amm-v2.api.ts
├── services/api/bridge.api.ts
├── services/api/circuit-breaker-status.api.ts
└── types/
    ├── amm-v2.types.ts
    └── bridge.types.ts

frontend/apps/governance/src/
├── config/scenario.ts
├── routes/index.tsx
├── components/layout/Sidebar.tsx
├── pages/CircuitBreakerPage.tsx
├── features/liquidity/LiquidityManagementPage.tsx
├── features/liquidity/CooperativeLiquidityWizard.tsx
├── features/liquidity/liquidity.store.ts
├── services/api/liquidity.api.ts
├── services/api/circuit-breaker-v2.api.ts
└── types/
    ├── liquidity.types.ts
    └── circuit-breaker-v2.types.ts
```

**Structure Decision**: Manter a estrutura atual dos apps e concentrar o alinhamento em páginas/stores/services/types já existentes de Scenario B, sem introduzir novo app ou camada compartilhada adicional.

## Phase 0: Research

**Status**: Complete — ver [research.md](./research.md)

Principais decisões:
1. Fluxo soberano canônico de CB em 4 fases (runbook seção 19) será a base única da UX de governança.
2. Padrão G5-cross no frontend é tratado como bloqueado/depreciado com handling explícito de `CROSS_CB_MINT_PROHIBITED`.
3. Pré-condição de bridge para commit usa bloqueio de progresso + polling orientado (`BRIDGE_POSITION_NOT_ACTIVE`).
4. App `bank` mantém jornada comercial (quote/swap/transfer) e nunca expõe controles de governança.
5. App `governance` mantém jornada soberana CB + circuit breaker e não expõe swap comercial.

## Phase 1: Design

### Artifacts gerados

| Artifact | Status | Link |
|---|---|---|
| research.md | Complete | [research.md](./research.md) |
| data-model.md | Complete | [data-model.md](./data-model.md) |
| quickstart.md | Complete | [quickstart.md](./quickstart.md) |
| contracts/frontend-scenario-b-alignment.md | Complete | [contracts/frontend-scenario-b-alignment.md](./contracts/frontend-scenario-b-alignment.md) |

### Impacted Modules (bank)

1. Navegação e gating: `config/scenario.ts`, `routes/index.tsx`, `components/layout/Sidebar.tsx`
2. Fluxo comercial Scenario B: `pages/AMMTradingPage.tsx`, `features/amm/amm-v2.store.ts`, `services/api/amm-v2.api.ts`, `types/amm-v2.types.ts`
3. Fluxo de bridge: `features/bridge/BridgePage.tsx`, `features/bridge/bridge.store.ts`, `services/api/bridge.api.ts`, `types/bridge.types.ts`
4. Estado regulatório para swap: `services/api/circuit-breaker-status.api.ts`

### Impacted Modules (governance)

1. Navegação e gating: `config/scenario.ts`, `routes/index.tsx`, `components/layout/Sidebar.tsx`
2. Fluxo de liquidez soberana: `features/liquidity/CooperativeLiquidityWizard.tsx`, `features/liquidity/LiquidityManagementPage.tsx`, `features/liquidity/liquidity.store.ts`, `services/api/liquidity.api.ts`, `types/liquidity.types.ts`
3. Governança de circuito: `pages/CircuitBreakerPage.tsx`, `services/api/circuit-breaker-v2.api.ts`, `types/circuit-breaker-v2.types.ts`

### Impacted Modules (backend - minimal read-only extensions)

1. LP positions query: `backend/services/api-gateway/internal/app/lp_position_repository.go` (FindByPoolPair, FindByProviderAndPoolPair methods)
2. Liquidity handler: `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go` (ListPositions handler)
3. Router registration: `backend/services/api-gateway/internal/http/router/v2/router.go` (GET /liquidity/positions route)
4. OpenAPI spec: `apis/openapi/amm.yaml` (LP positions endpoint documentation)
5. Environment templates: `backend/config/.env.infra.*.example` (6 files synchronized with production config)

### Scenario B UX Alignment Notes (Implementers)

1. Fluxo soberano de CB sempre em quatro fases: lock-mint, bridge ACTIVE, commit/watcher, pool ACTIVE.
2. `CROSS_CB_MINT_PROHIBITED` deve interromper fluxo legado sem retry automático.
3. `BRIDGE_POSITION_NOT_ACTIVE` deve manter a UI em espera com polling de 5s até 120s.
4. Swap comercial só habilita quando `pool_status=ACTIVE` e circuit breaker não está `HALTED`.
5. Bridge comercial deve refletir lifecycle lock-mint -> ACTIVE -> burn-unlock -> BURNED/UNLOCKED.

## Verification Strategy

### Static checks

```bash
cd frontend
npm run lint --workspace=bank
npm run lint --workspace=governance
npm run type-check --workspace=bank
npm run type-check --workspace=governance
```

### Scenario A non-regression checks

1. Build apps sem `VITE_SCENARIO=b` e validar que rotas/menus de Scenario B permanecem ocultos.
2. Verificar que páginas Scenario A de `bank` e `governance` continuam acessíveis e sem alteração funcional.

### Manual E2E alignment checks (Scenario B)

1. Executar `tryouts/tryout-sovereign-cb-liquidity.sh` para validar fase soberana (bridge->commit->executed->pool active).
2. Executar `tryouts/tryout-scenario-b-e2e.sh` para validar fluxo comercial e mensagens de erro de swap.
3. No app `governance`, validar UX para:
   - `CROSS_CB_MINT_PROHIBITED` (bloqueio definitivo, sem retry automático)
   - `BRIDGE_POSITION_NOT_ACTIVE` (estado de espera + polling 5s/120s)
   - `COMMIT_PENDING` com aviso >30s e timeout de UI em 60s
4. No app `bank`, validar:
   - pre-check `pool_status != ACTIVE` bloqueando swap
   - mapeamento de `POOL_NOT_ACTIVE`, `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY`, `CIRCUIT_BREAKER_HALTED`
   - refresh de quote em 10-15s

## Complexity Tracking

Sem violações de constitution ou exceções de escopo nesta fase de planejamento.
