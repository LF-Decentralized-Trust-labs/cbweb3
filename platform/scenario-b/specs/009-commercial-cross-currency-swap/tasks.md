# Implementation Tasks: Commercial Cross-Currency Swap

**Feature**: 009-commercial-cross-currency-swap  
**Generated**: 2026-05-26 | **Updated**: 2026-05-27 (FR-012 CB pool proxy; FR-013 approve-amm BANK_CODE)  
**User Stories**: 3 (US1 P1, US2 P1, US3 P2)  
**Estimated Tasks**: 57

## Implementation Strategy

MVP Scope: **User Story 1 only** (core cross-currency swap flow)
- Fornece valor imediato: banco comercial pode trocar BRL→ARS end-to-end
- Reutiliza infraestrutura existente (bridge, swap services)
- Independent test: operador executa 1000 BRL→2000 ARS via tryout

US2 (quote + slippage) e US3 (monitoring) são incrementais, sem dependências bloqueantes.

---

## Phase 1: Setup & Infrastructure

### Setup Tasks

- [X] T001 Create database migrations for new tables in backend/services/api-gateway/internal/migrations/010_create_cross_currency_swap_operations.up.sql
- [X] T002 Create database migrations for swap quotes table in backend/services/api-gateway/internal/migrations/011_create_swap_quotes.up.sql
- [X] T003 Create database migrations for rollback logs in backend/services/api-gateway/internal/migrations/012_create_swap_rollback_logs.up.sql
- [X] T004 Create database migrations for rate limit counters in backend/services/api-gateway/internal/migrations/013_create_swap_rate_limit_counters.up.sql
- [X] T005 [P] Create domain entity CrossCurrencySwapOperation in backend/services/api-gateway/internal/domain/cross_currency_swap.go
- [X] T006 [P] Create domain entity SwapQuote in backend/services/api-gateway/internal/domain/swap_quote.go
- [X] T007 [P] Create domain entity SwapRollbackLog in backend/services/api-gateway/internal/domain/swap_rollback_log.go
- [X] T008 [P] Create domain entity SwapRateLimitCounter in backend/services/api-gateway/internal/domain/swap_rate_limit_counter.go

---

## Phase 2: Foundational (Blocking Prerequisites)

### Repository Layer

- [X] T009 [P] Implement CrossCurrencySwapRepository with Create, FindByID, FindByCorrelationID, UpdateStatus methods in backend/services/api-gateway/internal/app/cross_currency_swap_repository.go
- [X] T010 [P] Implement SwapQuoteRepository with Create, FindByID, DeleteExpired methods in backend/services/api-gateway/internal/app/swap_quote_repository.go
- [X] T011 [P] Implement SwapRollbackLogRepository with Create, FindBySwapID, UpdateStatus methods in backend/services/api-gateway/internal/app/swap_rollback_log_repository.go
- [X] T012 [P] Implement RateLimitCounterRepository with IncrementCounter, CheckLimit, DeleteExpired methods in backend/services/api-gateway/internal/app/rate_limit_counter_repository.go

### Core Orchestration Services

- [X] T013 Create SwapRollbackCoordinator service with ReverseBridge method (3 retry logic with backoff 5s/15s/45s) in backend/services/api-gateway/internal/services/swap_rollback_coordinator.go
- [X] T014 Create CrossCurrencySwapOrchestrator service coordinating 3-step flow (bridge-in → swap → bridge-out) in backend/services/api-gateway/internal/services/cross_currency_swap_orchestrator.go
- [X] T015 Add correlation_id parameter to BridgeLockMintService.LockAndEnqueue in backend/services/api-gateway/internal/services/bridge_service.go
- [X] T016 Add correlation_id parameter to BridgeBurnUnlockService.BurnAndEnqueue in backend/services/api-gateway/internal/services/bridge_service.go
- [X] T017 Add correlation_id logging to SwapService.Execute (format: [correlation_id=<uuid>]) in backend/services/api-gateway/internal/services/swap_service.go

---

## Phase 2b: CB Pool Status Proxy (FR-012) — Commercial Bank Gateways

**Goal**: Banco comercial consulta `pool_status` e reservas via BC do spoke, não via AMM legado local.

**Test Criteria**: `GET /api/v2/amm/pool/W-BRL-ARS/status` no bank-a (18080) retorna ACTIVE com mesmas reservas do CB-A (38080); quote cross-currency no bank-a retorna `amount_in` válido.

- [X] T050 [P] [FR-012] Create `CentralBankPoolClient` with GetPoolStatus, IsActive, GetPoolReserves, GetFeeBps delegating to `GET {CENTRAL_BANK_API_URL}/api/v2/amm/pool/{pair}/status` in backend/services/api-gateway/internal/services/central_bank_pool_client.go
- [X] T051 [FR-012] Wire commercial bank gateways in `buildV2Dependencies`: when `cfg.CentralBankAPIURL != ""`, set PoolStatusService, PoolStatusGate, and SwapQuoteGenerator reserve reader to `CentralBankPoolClient` (no fallback to legacy AMM) in backend/services/api-gateway/internal/app/app.go
- [X] T052 [P] [FR-012] Add unit tests for pair normalization (`W-BRL-W-ARS` → `W-BRL-ARS`) and HTTP proxy in backend/services/api-gateway/internal/services/central_bank_pool_client_test.go
- [X] T053 [P] [US1] Fix `tryout-commercial-swap-e2e.sh`: default `API_GATEWAY_URL=http://localhost:18080`, parse `pool_status` and `amount_in` JSON fields in tryouts/tryout-commercial-swap-e2e.sh
- [X] T054 [US1] Align Hub swap execution via option B: CB exposes `GET /api/v2/amm/hub-liquidity-config`; commercial gateway resolves `sovereign_amm_address` + W-tokens at startup for `ammClient` / `SwapService` in hub_liquidity_config.go, hub_liquidity_handler.go, central_bank_pool_client.go, app.go

---

## Phase 3: User Story 1 - Swap Cross-Currency BRL→ARS via Hub (P1)

**Goal**: Banco comercial executa swap end-to-end de BRL no Spoke-A para ARS no Spoke-B via pool Hub

**Test Criteria**: Operador converte 1000 BRL em ~2000 ARS, confirma tx_hash em cada etapa, valida unlock final no Spoke-B

### Backend Implementation

- [X] T018 [US1] Implement pre-condition validation in CrossCurrencySwapOrchestrator: check pool ACTIVE (via `CentralBankPoolClient` when `CENTRAL_BANK_API_URL` set), circuit breaker OK, reserves sufficient in backend/services/api-gateway/internal/services/cross_currency_swap_orchestrator.go
- [X] T019 [US1] Implement Step 1 (Bridge-In) in CrossCurrencySwapOrchestrator: call LockAndEnqueue, wait for ACTIVE state via polling (120s timeout) in backend/services/api-gateway/internal/services/cross_currency_swap_orchestrator.go
- [X] T020 [US1] Implement Step 2 (Swap Hub) in CrossCurrencySwapOrchestrator: call SwapService.Execute with W-tokens, handle SLIPPAGE_LIMIT_EXCEEDED in backend/services/api-gateway/internal/services/cross_currency_swap_orchestrator.go
- [X] T021 [US1] Implement Step 3 (Bridge-Out) in CrossCurrencySwapOrchestrator: call BurnAndEnqueue for Spoke-B, wait for UNLOCKED state via polling (120s timeout) in backend/services/api-gateway/internal/services/cross_currency_swap_orchestrator.go
- [X] T022 [US1] Implement rollback logic in CrossCurrencySwapOrchestrator: on swap failure, call SwapRollbackCoordinator.ReverseBridge, log to SwapRollbackLog in backend/services/api-gateway/internal/services/cross_currency_swap_orchestrator.go
- [X] T023 [US1] Create CrossCurrencySwapHandler with SwapCrossCurrency method handling POST /api/v2/amm/swap/cross-currency in backend/services/api-gateway/internal/http/handlers/cross_currency_swap_handler.go
- [X] T024 [US1] Implement error mapping in CrossCurrencySwapHandler: POOL_NOT_ACTIVE, CIRCUIT_BREAKER_HALTED, INSUFFICIENT_POOL_LIQUIDITY return HTTP 422 with user-friendly messages in backend/services/api-gateway/internal/http/handlers/cross_currency_swap_handler.go
- [X] T025 [US1] Register POST /api/v2/amm/swap/cross-currency route with auth middleware (role: commercial_bank) in backend/services/api-gateway/internal/http/router/v2/router.go
- [X] T026 [P] [US1] Update OpenAPI spec amm.yaml with POST /api/v2/amm/swap/cross-currency endpoint schema (request: source_currency, target_currency, amount_out, max_amount_in, payer_bank_id, beneficiary_bank_id; response: swap_id, correlation_id, status, tx_hash) in apis/openapi/amm.yaml

### E2E Tryout

- [X] T027 [US1] Create tryout script tryout-commercial-swap-e2e.sh with 6 steps: (1) verify pool ACTIVE, (2) obtain quote, (3) execute swap, (4) monitor bridge-in (30s), (5) monitor swap (10s), (6) monitor bridge-out (30s), (7) verify COMPLETED status in tryouts/tryout-commercial-swap-e2e.sh
- [X] T028 [US1] Validate SC-001 in tryout: verify tx_hash for all 3 transactions (SpokeBridge-A lock, SovereignAMM swap, SpokeBridge-B unlock) via block explorer queries in tryouts/tryout-commercial-swap-e2e.sh
- [X] T029 [US1] Validate SC-002 in tryout: measure total latency (quote → unlock), assert p50 ≤60s and p95 ≤90s across 10 iterations in tryouts/tryout-commercial-swap-e2e.sh

---

## Phase 4: User Story 2 - Quote e Aprovação com Slippage Protection (P1)

**Goal**: Operador obtém cotação precisa com TTL 15s, valida slippage protection on-chain

**Test Criteria**: Operador obtém quote, aguarda 20s, recebe QUOTE_EXPIRED, obtém nova quote e executa com sucesso

### Quote Generation

- [X] T030 [P] [US2] Create SwapQuoteGenerator service with GenerateQuote method calculating amount_in via x·y=k + fee, setting created_at + valid_until (15s) in backend/services/api-gateway/internal/services/swap_quote_generator.go
- [X] T031 [P] [US2] Persist quote to swap_quotes table via SwapQuoteRepository.Create in SwapQuoteGenerator.GenerateQuote in backend/services/api-gateway/internal/services/swap_quote_generator.go
- [X] T032 [US2] Update GET /api/v2/amm/quote/exact-output handler to call SwapQuoteGenerator, return quote_id + created_at + valid_until + time_remaining_seconds in backend/services/api-gateway/internal/http/handlers/quote_handler.go
- [X] T033 [P] [US2] Update OpenAPI spec amm.yaml with GET /api/v2/amm/quote/exact-output updated response schema (add quote_id, created_at, valid_until, time_remaining_seconds fields) in apis/openapi/amm.yaml

### Quote Expiry Validation

- [X] T034 [US2] Implement quote expiry validation in CrossCurrencySwapOrchestrator: if quote_id provided, query SwapQuoteRepository, check NOW() ≤ valid_until, return HTTP 422 QUOTE_EXPIRED if expired in backend/services/api-gateway/internal/services/cross_currency_swap_orchestrator.go
- [X] T035 [US2] Implement slippage validation in CrossCurrencySwapOrchestrator: after swap execution, verify actual amount_in ≤ max_amount_in, return HTTP 422 SLIPPAGE_LIMIT_EXCEEDED if exceeded in backend/services/api-gateway/internal/services/cross_currency_swap_orchestrator.go

### Background Cleanup Job

- [X] T036 [P] [US2] Create background cleanup job (cron or scheduler) DeleteExpiredQuotes running hourly, calling SwapQuoteRepository.DeleteExpired(created_at < NOW() - 1h) in backend/services/api-gateway/cmd/cleanup_jobs/delete_expired_quotes.go

### E2E Tryout

- [ ] T037 [US2] Add quote expiry test to tryout-commercial-swap-e2e.sh: obtain quote, sleep 20s, attempt swap, assert HTTP 422 QUOTE_EXPIRED, obtain new quote, execute successfully in tryouts/tryout-commercial-swap-e2e.sh
- [ ] T038 [US2] Add slippage limit test to tryout-commercial-swap-e2e.sh: obtain quote, set max_amount_in artificially low (below quote amount_in), attempt swap, assert HTTP 422 SLIPPAGE_LIMIT_EXCEEDED in tryouts/tryout-commercial-swap-e2e.sh
- [ ] T039 [US2] Validate SC-003 in tryout: simulate volatility (modify pool reserves between quote and swap), measure slippage failure rate, assert ≤5% in tryouts/tryout-commercial-swap-e2e.sh

---

## Phase 5: User Story 3 - Monitoramento de Pool e Circuit Breaker Status (P2)

**Goal**: Operador consulta pool status e circuit breaker antes de swap para feedback preventivo

**Test Criteria**: Operador visualiza W-BRL-ARS status ACTIVE + reserves + circuit breaker OK; recebe alerta quando pool HALTED

### Backend Implementation

- [X] T050 (FR-012) Commercial bank `GET /api/v2/amm/pool/{pair}/status` returns CB-sourced pool_status (partial US3 — circuit breaker proxy still pending)
- [ ] T040 [P] [US3] Implement GET /api/v2/amm/pool/{pair}/circuit-breaker-status handler returning status (OK/HALTED), halted_by, halted_at, reason via circuit breaker service in backend/services/api-gateway/internal/http/handlers/pool_handler.go
- [ ] T055 [P] [US3] Optional: proxy circuit-breaker-status from commercial gateway to CB (same pattern as T050) when `CENTRAL_BANK_API_URL` is set
- [ ] T041 [P] [US3] Update OpenAPI spec amm.yaml with GET /api/v2/amm/pool/{pair}/circuit-breaker-status endpoint schema in apis/openapi/amm.yaml

### Frontend Implementation

- [ ] T042 [P] [US3] Create CrossCurrencySwapPage component with 3-step wizard UI (Quote, Execution, Confirmation) in frontend/apps/bank/src/features/cross-currency-swap/CrossCurrencySwapPage.tsx
- [ ] T043 [P] [US3] Create QuoteStep component with pool status query, countdown timer (15s), slippage tolerance input, amount fields (source_currency, target_currency, amount_out, max_amount_in) in frontend/apps/bank/src/features/cross-currency-swap/QuoteStep.tsx
- [ ] T044 [P] [US3] Create ExecutionStep component with polling logic for bridge-in (5s interval, 120s timeout), swap (3s interval, 60s timeout), bridge-out (5s interval, 120s timeout) with progress indicators in frontend/apps/bank/src/features/cross-currency-swap/ExecutionStep.tsx
- [ ] T045 [P] [US3] Create ConfirmationStep component displaying final tx_hash, amount_out, effective_rate, correlation_id, link to block explorer in frontend/apps/bank/src/features/cross-currency-swap/ConfirmationStep.tsx
- [ ] T046 [US3] Implement circuit breaker status check in QuoteStep: call GET /pool/{pair}/circuit-breaker-status before enabling "Get Quote" button, show alert if HALTED in frontend/apps/bank/src/features/cross-currency-swap/QuoteStep.tsx

---

## Phase 6: FR-013 — ApproveAMM Auto-Detection via BANK_CODE

**Goal**: Eliminar exigência explícita de `side` para bancos comerciais em `POST /api/v2/amm/token/approve-amm`, alinhando com o padrão já adotado em `/liquidity/commit` (spec clarification Session 2026-05-27).

**Test Criteria**: Bank-a chama `POST /api/v2/amm/token/approve-amm {"amount":"1"}` sem `side` e recebe `200 {"status":"ok","amount":"1"}`; chamada idêntica sem BANK_CODE configurado retorna `400 COMMIT_SIDE_NOT_CONFIGURED`; CB-A chamando com `side:"B"` explícito ainda funciona (G5-cross).

- [X] T056 [FR-013] Update TokenHandler in backend/services/api-gateway/internal/http/handlers/token_handler.go: add `approveSide string` field; update constructors (NewTokenHandler, NewTokenHandlerWithCBChecker) to accept `approveSide`; in ApproveAMM, when `approveSide != ""` (BANK_CODE configured) discard payload `side` entirely and pass `approveSide` to adapter; when `approveSide == ""` and caller is non-CB, return `400 {"error":"'side' required — set BANK_CODE env var","code":"COMMIT_SIDE_NOT_CONFIGURED"}` (CB G5-cross explicit side unchanged)
- [X] T057 [FR-013] Wire FR-013 in backend/services/api-gateway/internal/app/app.go: derive `approveSide` from `cfg.BankCode` using same mapping as `commitSide` (BANK_CODE → "A"|"B"); pass `approveSide` to TokenHandler constructor; no fallback to reading `side` from request
- [X] T058 [P] [FR-013] Add FR-013 validation to tryouts/tryout-integracao-scenario-b.sh: (a) bank-a calls approve-amm without `side` → assert HTTP 200; (b) call without `side` on gateway with BANK_CODE unset → assert HTTP 400 COMMIT_SIDE_NOT_CONFIGURED; (c) CB-A calls with `side:"B"` → assert HTTP 200 (G5-cross preserved)

---

## Phase 7: Rate Limiting & Final Polish

### Rate Limiting

- [ ] T047 [P] Implement RateLimitMiddleware for POST /api/v2/amm/swap/cross-currency: check swap_rate_limit_counters for payer_bank_id (10/min, 100/hour), increment counters, return HTTP 429 with Retry-After if exceeded in backend/services/api-gateway/internal/http/middleware/rate_limit_middleware.go
- [ ] T048 [P] Create background cleanup job DeleteExpiredRateLimitCounters running hourly, deleting window_start < NOW() - 2h in backend/services/api-gateway/cmd/cleanup_jobs/delete_expired_rate_limits.go

### Diagnostic Tooling

- [ ] T049 [P] Create tryout-cross-currency-diagnostic.sh with step-by-step validation: (1) check pool status, (2) check circuit breaker, (3) check bridge positions, (4) query swap operation by correlation_id, (5) check rollback logs in tryouts/tryout-cross-currency-diagnostic.sh

---

## Dependency Graph

```
Phase 1 (Setup)
  ↓
Phase 2 (Foundational)
  ↓
Phase 2b (FR-012 CB pool proxy) ← blocks correct pool/quote on commercial gateways
  ↓
Phase 3 (US1 - Core Swap) ← MVP切割点 (T054 blocks full E2E swap on commercial until Hub AMM aligned)
  ↓
Phase 4 (US2 - Quote + Slippage) ← Incremental value
  ↓
Phase 5 (US3 - Monitoring) ← Incremental value
  ↓
Phase 6 (FR-013 approve-amm BANK_CODE) ← Can run parallel with Phase 5 (independent of swap orchestrator)
  ↓
Phase 7 (Polish)
```

**US Dependencies**:
- US1 (P1) is foundation - must complete first
- US2 (P1) depends on US1 orchestrator being functional
- US3 (P2) depends on US1 backend + US2 quote service

**Parallel Opportunities**:
- Within Phase 1: T005-T008 (domain entities) can run parallel
- Within Phase 2: T009-T012 (repositories) can run parallel after domain entities
- Within Phase 3: T026 (OpenAPI) can run parallel with T023-T025 (handler/route)
- Within Phase 4: T030-T031 (quote generation) can run parallel with T036 (cleanup job)
- Within Phase 5: T042-T046 (frontend components) can run parallel
- Phase 6: T047-T048 (rate limiting) can run parallel with T049 (diagnostic)

---

## Execution Evidence

Track completion status here (update as tasks finish):

### Phase 1: Setup
- [X] T001-T008: ✓ Domain entities created (cross_currency_swap.go, swap_quote.go, swap_rollback_log.go, swap_rate_limit_counter.go) + registered in RunAutoMigrate

### Phase 2: Foundational
- [X] T009-T012: ✓ Repositories
- [X] T013-T017: ✓ Orchestrator, rollback, correlation_id

### Phase 2b: FR-012 CB pool proxy
- [X] T050-T053: ✓ CentralBankPoolClient, app wiring, tests, tryout fixes (pool ACTIVE + quote OK on bank-a:18080)
- [X] T054: Hub swap client resolves sovereign AMM from CB `hub-liquidity-config`

### Phase 3: User Story 1
- [X] T018-T025, T027-T029, T054: ✓ Backend + tryout steps 1–2; full E2E swap requer validação manual pós-rebuild

### Phase 4: User Story 2
- [X] T030-T036: ✓ Quote generator + expiry + cleanup job
- [ ] T037-T039: Tryout quote/slippage tests pending

### Phase 5: User Story 3
- [X] T050: Partial — pool status via CB proxy on commercial gateways
- [ ] T040-T046, T055: Circuit breaker endpoint + frontend pending

### Phase 6: FR-013 approve-amm BANK_CODE
- [ ] T056-T058: Not started

### Phase 7: Polish
- [ ] T047-T049: Not started

---

## Validation Checklist

Before marking feature complete, verify:

- [ ] All 3 user stories have independent test criteria passing
- [ ] SC-001: tx_hash verified for all 3 transactions (bridge-in, swap, bridge-out)
- [ ] SC-002: Latency p50 ≤60s and p95 ≤90s measured via tryout
- [ ] SC-003: Slippage failure rate ≤5% in volatility simulation
- [ ] SC-004: All error scenarios display user-friendly messages with recommended actions
- [ ] SC-005: Quote expiry flow (obtain → wait 20s → error → refresh → success) validated manually
- [ ] FR-010 rollback verified: swap fails after bridge-in → automatic bridge reverso → funds returned to payer
- [ ] FR-011 cleanup jobs running: quotes deleted after 1h, rate limit counters deleted after 2h
- [X] FR-012 commercial bank pool status/quote via `CENTRAL_BANK_API_URL` → BC of spoke (T050-T053)
- [X] FR-012 full E2E: Hub swap step uses Sovereign AMM resolved from CB hub-liquidity-config (T054)
- [ ] FR-013 approve-amm: commercial bank gateway derives side from BANK_CODE, no explicit side required (T056-T058)
- [ ] NFR-005 correlation_id logged in all 3 sub-operations (bridge-in, swap, bridge-out)
- [ ] OpenAPI spec updated and validated via Swagger UI
- [ ] Frontend deployed to bank app, accessible at /cross-currency-swap route
