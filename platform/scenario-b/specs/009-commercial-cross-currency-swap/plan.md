# Implementation Plan: Commercial Cross-Currency Swap

**Branch**: `009-commercial-cross-currency-swap` | **Date**: 2026-05-27 (atualizado) | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/009-commercial-cross-currency-swap/spec.md`

## Summary

Implementar fluxo end-to-end de swap cross-currency para bancos comerciais, permitindo troca BRL→ARS via pool de liquidez CB no Hub. Orquestra 3 operações: (1) Bridge Spoke-A→Hub (lock BRL, mint W-BRL), (2) Swap Hub (W-BRL→W-ARS via AMM), (3) Bridge Hub→Spoke-B (burn W-ARS, unlock ARS). Reutiliza infraestrutura existente de bridge (007/008) e swap (POST /api/v2/amm/swap/exact-output), adicionando novo endpoint orquestrador `POST /api/v2/amm/swap/cross-currency` com validação de pré-condições (pool ACTIVE, circuit breaker OK), quote com expiry (15s), proteção slippage e rollback parcial em caso de falha.

**Atualização 2026-05-27 (FR-012)**: Gateways comerciais **não** leem o Sovereign AMM no Hub diretamente para `pool_status`/reservas/quote. Com `CENTRAL_BANK_API_URL` configurado, delegam via `CentralBankPoolClient` ao api-gateway do **BC do mesmo spoke** (`GET {CB}/api/v2/amm/pool/{pair}/status`). O BC mantém `SOVEREIGN_AMM_ADDRESS` e é fonte canônica on-chain. Mapeamento: Spoke-A (`bank-a`, `bank-c`) → CB-A; Spoke-B (`bank-b`, `bank-d`) → CB-B. *Pendente*: execução on-chain do swap no Hub a partir do gateway comercial ainda pode usar `AMM_CONTRACT_ADDRESS` legado — ver T054.

## Technical Context

**Language/Version**: Go 1.25.5 (backend api-gateway), TypeScript 5.4 + Node 20 (frontend bank app), Solidity 0.8.20 (contratos AMM/Bridge)  
**Primary Dependencies**: Fiber v2.52.9 (HTTP), GORM + PostgreSQL, go-ethereum v1.17.1, ethers v6 (frontend), React 19 + Zustand (frontend state)  
**Storage**: PostgreSQL (cross_currency_swap_operations table para tracking end-to-end, swap_quotes table para quotes com expiry)  
**Testing**: Go test + testify (unit/integration backend), React Testing Library + Vitest (frontend), bash tryouts (E2E)  
**Target Platform**: Linux server (backend), SPA web browser (frontend), Besu EVM (contratos)  
**Project Type**: Web service (backend REST API) + Web application (frontend React)  
**Performance Goals**: Latência end-to-end p50 ≤60s / p95 ≤90s (SC-002), quote generation <500ms, swap tx confirmation <10s on-chain  
**Constraints**: Pool reserves validation pre-swap (comercial → proxy BC; CB → Sovereign AMM Hub), circuit breaker status check, quote expiry enforcement (15s), rate limiting 10 swaps/min per bank  
**Integration**: `CENTRAL_BANK_API_URL` (já usado em onboarding/payments) para leitura de pool no gateway comercial  
**Scale/Scope**: Suporta dezenas de bancos comerciais com centenas de swaps/hora, pool reserves ~100k-1M tokens por lado

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution file está em formato template (sem princípios preenchidos). Gates aplicados por escopo desta feature:

| Gate | Status | Observação |
|---|---|---|
| Reutilização de infraestrutura existente | PASS | Reutiliza swap service, bridge service, pool status via BC (FR-012) |
| Separação Hub vs spoke para pool status | PASS | Sovereign AMM no Hub; comercial consulta BC do spoke |
| Sem breaking changes em APIs existentes | PASS | Novo endpoint orquestrador, APIs existentes inalteradas |
| Test coverage para fluxos críticos | PENDING | Requer unit tests (service layer) + tryout E2E |
| Backward compatibility | PASS | Novos endpoints não afetam fluxos CB existentes (spec 007/008) |

**Re-check pós-design**: PASS após Phase 1 completa.

## Project Structure

### Documentation (this feature)

```text
specs/009-commercial-cross-currency-swap/
├── plan.md              # Este arquivo (gerado por /speckit.plan)
├── research.md          # Phase 0 output - decisões de arquitetura orquestrador
├── data-model.md        # Phase 1 output - entidades CrossCurrencySwapOp, SwapQuote
├── quickstart.md        # Phase 1 output - tryout manual E2E
├── contracts/           # Phase 1 output - contrato de API orquestrador
│   └── cross-currency-swap-api.md
└── tasks.md             # Phase 2 output (/speckit.tasks - NÃO criado por /speckit.plan)
```

### Source Code (repository root)

```text
backend/services/api-gateway/
├── internal/
│   ├── domain/
│   │   ├── cross_currency_swap.go         # Nova entidade CrossCurrencySwapOperation
│   │   └── swap_quote.go                  # Nova entidade SwapQuote com expiry
│   ├── app/
│   │   ├── cross_currency_swap_repository.go  # Persistence para tracking end-to-end
│   │   └── swap_quote_repository.go           # Persistence quotes com TTL
│   ├── services/
│   │   ├── central_bank_pool_client.go          # FR-012: proxy pool status → BC do spoke
│   │   ├── central_bank_pool_client_test.go
│   │   ├── cross_currency_swap_orchestrator.go  # orquestra bridge+swap+bridge
│   │   ├── swap_quote_generator.go              # quote com timestamp validation
│   │   └── swap_rollback_coordinator.go         # rollback parcial em caso de falha
│   ├── http/
│   │   ├── handlers/
│   │   │   ├── cross_currency_swap_handler.go   # NOVO: POST /cross-currency
│   │   │   └── swap_quote_handler.go            # ATUALIZADO: GET /quote com valid_until
│   │   └── router/v2/
│   │       └── router.go                        # ATUALIZADO: registrar novos endpoints
│   └── migrations/
│       ├── 010_create_cross_currency_swap_operations.up.sql
│       └── 011_create_swap_quotes.up.sql

frontend/apps/bank/src/
├── features/
│   └── cross-currency-swap/                 # NOVO módulo
│       ├── CrossCurrencySwapPage.tsx        # UI principal 3-etapas
│       ├── QuoteStep.tsx                    # Step 1: Quote & Approve
│       ├── ExecutionStep.tsx                # Step 2: Bridge & Swap monitoring
│       ├── ConfirmationStep.tsx             # Step 3: Unlock final verification
│       ├── cross-currency-swap.store.ts     # Zustand state
│       └── cross-currency-swap.api.ts       # API client
├── services/api/
│   └── cross-currency-swap.api.ts           # HTTP wrapper para endpoints
└── types/
    └── cross-currency-swap.types.ts         # TypeScript types

tryouts/
├── tryout-commercial-swap-e2e.sh            # NOVO: E2E completo BRL→ARS
└── tryout-cross-currency-diagnostic.sh      # NOVO: diagnostic step-by-step

contracts/src/                                # Sem mudanças (reutiliza AMM + Bridge existentes)
apis/openapi/amm.yaml                         # ATUALIZADO: documentar novos endpoints
```

**Structure Decision**: Backend segue padrão existente (domain → app → services → handlers → router). Frontend adiciona novo módulo `cross-currency-swap` no app `bank` (comercial banks only). Sem mudanças em contratos Solidity — reutiliza SovereignAMM e SpokeBridge já deployados.

## Phase 0: Research

**Status**: Completo — ver [research.md](./research.md)

Decisões registradas (incl. spec Session 2026-05-27):

1. **Orquestração**: Transaction coordinator simples (bridge-in → swap → bridge-out) com rollback parcial.
2. **Quote expiry**: Server-side em `swap_quotes` (15s TTL).
3. **Rollback**: Bridge reverso automático até 3 tentativas.
4. **Rate limiting**: Database-backed counters (MVP).
5. **Correlation ID**: UUID no orchestrador, propagado explicitamente.
6. **Pool status (FR-012)**: Gateway comercial → HTTP proxy ao BC do spoke (`CENTRAL_BANK_API_URL`); BC lê `SOVEREIGN_AMM_ADDRESS` no Hub. Sem replicação de `SOVEREIGN_AMM` em `.env` de bancos comerciais.
7. **Par soberano**: Normalização `W-BRL-W-ARS` → `W-BRL-ARS` na chamada ao BC.

## Phase 1: Design

### Artifacts a serem gerados

| Artifact | Status | Link |
|---|---|---|
| research.md | Done | [research.md](./research.md) |
| data-model.md | Done | [data-model.md](./data-model.md) |
| quickstart.md | Done (atualizar nota FR-012) | [quickstart.md](./quickstart.md) |
| contracts/cross-currency-swap-api.md | Done | [contracts/cross-currency-swap-api.md](./contracts/cross-currency-swap-api.md) |
| spec.md FR-012 | Done 2026-05-27 | [spec.md](./spec.md) |

### Impacted Modules

#### Backend (api-gateway)

1. **Domain Layer**:
   - `cross_currency_swap.go`: Entidade CrossCurrencySwapOperation com estados (QUOTING, BRIDGE_IN_PROGRESS, SWAP_IN_PROGRESS, BRIDGE_OUT_PROGRESS, COMPLETED, FAILED)
   - `swap_quote.go`: Entidade SwapQuote com timestamp, valid_until, expired computed field

2. **Persistence Layer (app/)**:
   - `cross_currency_swap_repository.go`: CRUD + FindByCorrelationID para tracking
   - `swap_quote_repository.go`: Create + FindValidQuote (valida timestamp) + ExpireOld (cleanup job)

3. **Service Layer**:
   - `central_bank_pool_client.go` (FR-012): Quando `CENTRAL_BANK_API_URL` setado, implementa leitura de pool/quote reserves via BC do spoke; usado por `PoolStatusService`, `PoolStatusGate`, `SwapQuoteGenerator` no gateway comercial
   - `cross_currency_swap_orchestrator.go`: Coordena 3-step flow, aplica pre-checks (pool ACTIVE via CB no comercial, circuit breaker OK), chama bridge/swap services
   - `swap_quote_generator.go`: Calcula amount_in via fórmula x·y=k, adiciona timestamp + 15s TTL
   - `swap_rollback_coordinator.go`: Executa bridge reverso se swap falhar
   - `app.go` wiring: `if cfg.CentralBankAPIURL != "" { cbPoolClient ... }` — sem fallback para AMM legado local

4. **HTTP Layer (handlers/)**:
   - `cross_currency_swap_handler.go`: POST /cross-currency, valida JWT commercial_bank role
   - `swap_quote_handler.go`: ATUALIZADO para retornar valid_until field

5. **Router**:
   - Registrar POST /api/v2/amm/swap/cross-currency
   - Atualizar GET /api/v2/amm/quote/exact-output response

6. **Migrations**:
   - SQL para cross_currency_swap_operations table
   - SQL para swap_quotes table

#### Frontend (bank app)

1. **Features**:
   - `CrossCurrencySwapPage.tsx`: Container com stepper 3 etapas
   - `QuoteStep.tsx`: Form + countdown timer 15s
   - `ExecutionStep.tsx`: Polling tx_hash + bridge states
   - `ConfirmationStep.tsx`: Mostrar resultado final + tx links

2. **State Management**:
   - `cross-currency-swap.store.ts`: Zustand store com current_step, quote, swap_operation, error_state

3. **API Client**:
   - `cross-currency-swap.api.ts`: Wrappers para POST /cross-currency, GET /quote

4. **Types**:
   - TypeScript interfaces para CrossCurrencySwapRequest/Response, SwapQuote

#### Tryouts

1. **tryout-commercial-swap-e2e.sh**: Setup CB-A/CB-B liquidity, executar swap BRL→ARS de banco comercial, validar unlock final no Spoke-B
2. **tryout-cross-currency-diagnostic.sh**: Step-by-step com verbose output para troubleshooting

#### OpenAPI

1. **amm.yaml**: Adicionar schemas para /cross-currency endpoint (request/response) e atualizar /quote response com valid_until

### Cross-Cutting Concerns

- **Logging**: Correlation ID em todos os logs das 3 operações
- **Monitoring**: Métricas de latência por etapa (bridge-in, swap, bridge-out)
- **Error Handling**: Mapear erros on-chain (SLIPPAGE_EXCEEDED, INSUFFICIENT_LIQUIDITY) para HTTP 422 com error_code
- **Security**: Rate limiter anti-abuse + validação de commercial_bank role no JWT

## Verification Strategy

### Static Checks

```bash
cd backend/services/api-gateway
go fmt ./...
go vet ./...
staticcheck ./...
go test ./internal/services/... -v
go test ./internal/http/handlers/... -v
```

### Contract Tests (Go)

- `cross_currency_swap_orchestrator_test.go`: Unit tests para happy path + failure scenarios (swap fail, bridge-out fail)
- `swap_quote_generator_test.go`: Validar cálculo amount_in, expiry logic
- `cross_currency_swap_handler_test.go`: HTTP contract tests para 200/422/500 responses

### Integration Tests (Bash Tryouts)

- `tryout-commercial-swap-e2e.sh`: E2E via gateway comercial (`API_GATEWAY_URL=http://localhost:18080` bank-a); passos 1–2 validam pool ACTIVE e quote via proxy CB
- `tryout-sovereign-cb-liquidity.sh` / `debug-cb-liquidity-injection.sh`: Pré-requisito — pool ACTIVE no CB antes do tryout comercial
- Cenários de erro: pool EMPTY (se CB sem liquidez), CB indisponível, circuit breaker HALTED, quote expired, slippage exceeded

### Frontend Tests

```bash
cd frontend/apps/bank
npm run test
```

- `CrossCurrencySwapPage.test.tsx`: Render 3 steps, countdown timer behavior
- `cross-currency-swap.store.test.ts`: State transitions QUOTING → COMPLETED

## Complexity Tracking

Nenhuma violação de constitution identificada — feature reutiliza padrões existentes (service layer, handler pattern, Zustand state) e não introduz complexidade arquitetural nova.

## Phase 2: Task Generation

**Note**: Phase 2 é executada via comando `/speckit.tasks` separadamente após Phase 1 estar completa.

Ver [tasks.md](./tasks.md) — destaques pós FR-012:
- T050-T053: CB pool proxy (implementado)
- T054: Swap Hub no gateway comercial com Sovereign AMM (pendente)
- T037-T049, T040-T046: pendentes (frontend, rate limit, diagnostic)
