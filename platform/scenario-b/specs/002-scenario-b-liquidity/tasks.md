---
description: "Task list for Scenario B rebuild: external docs + backend v2 + Solidity alignment + E2E tryout"
---

# Tasks: Scenario B Liquidity (Docs + Backend + Contracts + E2E)

**Input**: Design documents from `/specs/002-scenario-b-liquidity/`
**Prerequisites**: plan.md, spec.md (P1/P2/P3 user stories + FR-001 a FR-059 + SC-001 a SC-030 + clarificacoes 2026-05-04), research.md (Decisions 1-23), data-model.md, contracts/scenario-b-backend-api.md, quickstart.md

**Tests**: `forge test -vv` para Solidity; testes de contrato Go (`go test ./...`) para handlers; tryout E2E em `tryouts/` sao requisito explicito do spec (FR-019/FR-020).

**Explicit exclusions** (absorvidos das clarificacoes):
- Sem rate limiting nesta iteracao (FR-053/FR-054) — NENHUMA task instala middleware de throttling.
- Sem observabilidade estruturada (Prom/OTel) nesta iteracao (FR-051/FR-052) — logs ad-hoc em stdout; audit helper e somente event append, sem metricas/tracing.
- Frontend (`frontend/apps/*`) intocado (Decision 18 / SC-025) — NENHUMA task edita arquivos sob `frontend/`.
- Retencao indefinida (FR-045/FR-047) — NENHUMA task de purge/TTL; particionamento por tempo substitui purge.
- Sem arquivos `.sql` de migration (FR-055) — schema via GORM `AutoMigrate` + `db.Exec()` em `internal/db/init/`.

**Organization**: Tasks agrupadas por user story (US1 P1, US2 P2, US3 P3) para entrega e validacao independentes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependencias pendentes).
- **[Story]**: Mapeia a user story da spec (US1/US2/US3).
- Caminhos de arquivo explicitos em cada task.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Inicializacao de diretorios, esqueletos de arquivos e baseline de ferramentas. Sem logica funcional. Completa antes que qualquer user story comece.

- [X] T001 Criar estrutura de diretorios de documentacao externa: `docs/external/scenario-b/` com arquivo `scenario-b.md` (stub vazio) e `glossary.md` (stub vazio). Esta documentacao e a entrega principal de US1.
- [X] T002 [P] Criar diretorios Go para inicializadores de schema GORM (FR-055): `backend/services/api-gateway/internal/db/init/` com stub `migrate.go` e `backend/services/api-gateway/internal/db/seeds/` com stub `endpoint_contract_seed.go`. Repetir para `backend/services/payment-orchestrator/internal/db/init/` e `backend/services/compliance/internal/db/init/`. **Nenhum arquivo `.sql`** deve ser criado nesta fase nem em qualquer outra.
- [X] T003 [P] Criar stub de OpenAPI v2 em `backend/services/api-gateway/openapi/v2/scenario-b.yaml` com info, servidor e componentes de seguranca basicos (bearerAuth Keycloak). Todos os endpoints serao populados em tasks posteriores.
- [X] T004 [P] Criar pacote Go `backend/shared/blockchain/scenariob/` com stubs: `amm/client.go`, `spokebridge/client.go`, `tcebm/client.go`, `identity/client.go`, `relayer/client.go`. Cada arquivo define a struct do cliente e seus metodos como `// TODO: implement`.
- [X] T005 [P] Criar stub de tryout E2E em `tryouts/tryout-scenario-b-e2e.sh`: cabecalho Bash com `set -euo pipefail`, comentario de pre-requisitos (docker, forge, cast, jq, openssl), e secoes vazias por etapa (1-Infraestrutura, 2-Contratos, 3-Participantes, 4-Pool, 5-US1, 6-US2, 7-US3).
- [X] T006 [P] Criar ou atualizar `tryouts/README.md` documentando o tryout E2E do Cenario B: pre-requisitos, como executar (`bash tryouts/tryout-scenario-b-e2e.sh`), e variaveis de ambiente necessarias (BESU_HUB_RPC, SPOKE_A_RPC, SPOKE_B_RPC, PALADIN_RPC_URL, KEYCLOAK_URL).
- [X] T007 Rodar `.specify/scripts/bash/check-prerequisites.sh` para confirmar que todos os artefatos de spec estao em ordem antes de avancar para a Phase 2.

---

## Phase 2: Foundational (Blocking Prerequisites — MUST complete before user stories)

**Purpose**: Inventario de artefatos legados, registros de reuso de infra, GORM foundation, remocao do Cenario A, configuracao de Keycloak e scaffolding de DI. Bloqueia todos as user stories.

### Inventario e Cutover Artifacts

- [X] T008 Inventariar todos os artefatos de backend do Cenario A em `specs/002-scenario-b-liquidity/cutover/legacy-artifacts.yaml`: routes, handlers, services, jobs, testes, docs e schemas de DB. Cada entrada deve ter `artifact_id`, `artifact_type`, `source_path`, `migration_action` (`REMOVE` ou `REPLACE`) e `replacement_ref` quando aplicavel (FR-011/FR-014/FR-015).
- [X] T009 [P] Registrar componentes de infraestrutura transversal reaproveitaveis em `specs/002-scenario-b-liquidity/cutover/infrastructure-reuse.yaml`: Keycloak (IDENTITY), Postgres (DATASTORE), Redis (CACHE), Docker Compose (RUNTIME). Cada entrada com `component_id`, `layer`, `reuse_mode` e `legacy_dependency_check: true` (FR-017/Decision 2).
- [X] T010 [P] Criar `specs/002-scenario-b-liquidity/cutover/cutover-plan.yaml` com status inicial `PLANNED`, `scheduled_at`, totalizadores de artefatos legados vindos de T008, e lista de componentes reaproveitados de T009 (FR-016/SC-008).

### GORM Foundation e Remocao Cenario A

- [X] T011 [P] Adicionar dependencias GORM ao `backend/services/api-gateway/go.mod`: `gorm.io/gorm v1.31.1` e `gorm.io/driver/postgres v1.6.0`; rodar `go mod tidy` e confirmar compilacao (FR-055/C1).
- [X] T012 [P] Criar `backend/services/api-gateway/internal/db/init/migrate.go` com funcao `RunAutoMigrate(db *gorm.DB) error` que registra todos os modelos do Cenario B via `db.AutoMigrate(...)` — inicialmente vazio, modelos serao adicionados em T029-T033, T058, T066-T069, T085-T090. Nenhum arquivo `.sql` (FR-055).
- [X] T013 Remover toda a implementacao funcional do Cenario A do `backend/services/api-gateway`: rotas v1, handlers, services e qualquer import de dominio A. Garantir que o servico ainda compila apos remocao. Registrar em `legacy-artifacts.yaml` cada artefato removido (FR-010/FR-012/FR-013/SC-010).
- [X] T014 [P] Remover toda a implementacao funcional do Cenario A do `backend/services/payment-orchestrator`: routes, handlers, services, domain models e jobs. Servico deve compilar apos remocao (FR-010/FR-013/SC-010).
- [X] T015 [P] Remover toda a implementacao funcional do Cenario A do `backend/services/compliance`: routes, handlers e domain models (FR-010/FR-013/SC-010).
- [X] T016 Criar `backend/services/api-gateway/internal/db/init/cleanup_scenarioa.go` com funcao `DropScenarioATables(db *gorm.DB) error` que executa via `db.Exec()` os `DROP TABLE IF EXISTS` dos schemas e tabelas do Cenario A. Chamar antes de `RunAutoMigrate` no startup (FR-016/FR-055/SC-009).

### Keycloak e Domain Role Constants

- [X] T017 [P] Reconfigurar realm Keycloak do Cenario B em `deploy/local/keycloak/realms/scenario-b-realm.json`: adicionar realm roles `central_bank`, `commercial_bank` e `hub_operator`. Adicionar constantes `RoleCentralBankScenarioB = "central_bank"` e `RoleCommercialBankScenarioB = "commercial_bank"` em `backend/services/api-gateway/internal/domain/auth.go` (FR-030/FR-056/M1/M2).

### Router v2 e DI Scaffolding

- [X] T018 Criar `backend/services/api-gateway/internal/http/router/v2/router.go` com struct `Dependencies` que aceita `QuoteService`, `SwapService`, `PoolStatusService`, `BridgeService`, `ComplianceService`, `GovernanceService` e `OversightService` como dependencias injetadas. Registrar funcao `Register(app *fiber.App, deps Dependencies)` sem rotas (rotas adicionadas nas tasks das user stories). Seguir padrao existente de `AuthHandler`, `ComplianceHandler`, `GovernanceHandler`.
- [X] T019 [P] Criar `backend/services/api-gateway/internal/audit/audit.go` com funcao `Append(db *gorm.DB, tableName, eventKind, refID string, meta map[string]string) error` que insere via GORM Create em tabela generica de audit log append-only. Nao emite UPDATE ou DELETE (FR-048/FR-049).
- [X] T020 Atualizar `backend/services/api-gateway/internal/app/app.go` (funcao `New`): instanciar `QuoteService`, `SwapService`, `PoolStatusService` (stubs vazios neste momento; implementacao em T037-T039), construir `router.Dependencies` com esses services e chamar `v2router.Register(app, deps)`. Seguir exatamente o padrao de wiring dos handlers ja existentes (FR-056 / H4).
- [X] T021 [P] Criar `backend/services/api-gateway/internal/db/init/triggers.go` com funcao `CreateAppendOnlyTriggers(db *gorm.DB) error` que executa via `db.Exec()` os triggers PL/pgSQL `BEFORE UPDATE` e `BEFORE DELETE` que lancam excecao em tabelas de audit log. Aplicar a: `circuit_breaker_signatures`, `disclosure_signatures`, `liquidity_alerts`, tabelas `*_event_history`. Criar correspondente `triggers_test.go` validando que UPDATE/DELETE falham com excecao (FR-048/FR-049/FR-055).
- [X] T022 Rodar `forge build` e `forge test -vv` para confirmar baseline Foundry; corrigir erros de compilacao pre-existentes nos contratos existentes antes de evoluir (T002 de Foundry baseline).
- [X] T023 [P] Criar `specs/002-scenario-b-liquidity/cutover/risk-register.md` documentando: ausencia de rate limiting (FR-053), ausencia de observabilidade estruturada (FR-051), incompatibilidade de frontend (SC-025), retencao indefinida sem purge (FR-045), e hardening contra DBA (FR-050) — todos como riscos aceitos (FR-054/FR-047).

---

## Phase 3: User Story 1 — Documentacao Externa + AMM Quote/Swap/Pool (Priority P1)

**Story Goal**: Como responsavel por documentacao externa, publicar resumo oficial do Cenario B e entregar API v2 de quote/swap/pool com protecao de slippage, RBAC e ZK compliance gate.

**Independent Test Criteria**: `GET /api/v2/amm/quote/exact-output` retorna input correto; `POST /api/v2/amm/swap/exact-output` retorna 422 com `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY` ou `ZK_VALIDATION_FAILED` conforme cenario; docs externas sem referencias ao Cenario A.

### Documentacao Externa

- [X] T024 [P] [US1] Redigir secao de resumo executivo em `docs/external/scenario-b/scenario-b.md`: objetivo do projeto, atores principais (Liquidity Taker, Liquidity Provider/Issuer, Network Operator, Hub, Relayer), e valor de negocio do modelo de liquidity pool. Linguagem orientada a stakeholder nao tecnico (FR-001/FR-009/FR-021).
- [X] T025 [P] [US1] Redigir secao "Fluxo Operacional Cenario B" em `docs/external/scenario-b/scenario-b.md`: topologia Hub-and-Spoke, ciclo Lock&Mint/Burn&Unlock, Exact-Output com `maxAmountIn`, ZK-Pointers e threshold de desequilibrio 70/30. Incluir diagrama de sequencia Mermaid (FR-003/FR-021/FR-022/FR-023/FR-024/FR-025).
- [X] T026 [P] [US1] Redigir secao "Capacidades Reaproveitaveis" em `docs/external/scenario-b/scenario-b.md`: listar capacidades com status (disponivel, parcial, nao implementado), separando reaproveitaveis das pendentes. Mapear rastreabilidade com fontes de referencia (FR-004/FR-005/FR-008).
- [X] T027 [P] [US1] Redigir secao "Proposta de Nova Implementacao" e "Controles de Risco e Governanca" em `docs/external/scenario-b/scenario-b.md`: ciclo de liquidez, fluxo hub-and-spoke E2E, Circuit Breaker, Master Viewing Key. Cada proposta com precondicao de negocio e criterio de pronto verificavel (FR-006/FR-007/FR-024/FR-026).
- [X] T028 [P] [US1] Redigir secao "Governanca Multi-Assinatura" em `docs/external/scenario-b/scenario-b.md`: Circuit Breaker assimetrico (pause 1-of-N / resume 2-of-N), Master Viewing Key 2-of-3 com timeout 72h, descricao em linguagem de negocio para stakeholder nao tecnico (FR-026/FR-030/FR-034/FR-035).
- [X] T029 [P] [US1] Realizar auditoria de remocao do Cenario A no documento externo e criar `specs/002-scenario-b-liquidity/cutover/docs-editorial-review.md`: listar todas as referencias ao Cenario A encontradas/removidas, confirmar 0 referencias ativas apos revisao (FR-002/SC-001).
- [X] T030 [P] [US1] Criar `specs/002-scenario-b-liquidity/cutover/cross-consistency-report.md`: mapear cada secao do documento externo a sua FR/SC de origem, registrar zero contradicoes criticas (FR-008/SC-004).

### Models GORM — Cenario B Core

- [X] T031 [US1] Criar model `SwapOrderScenarioB` em `backend/services/payment-orchestrator/internal/domain/swap_order.go` com maquina de 5 estados canonicos (FR-057): `PENDING` (criado, aguardando submissao on-chain) → `SUBMITTED` (transacao enviada ao Hub; registrar `submitted_at *time.Time`) → `CONFIRMING` (aguardando confirmacao de bloco) → `COMPLETED` (bloco confirmado; registrar `confirmed_at *time.Time`) / `FAILED` (erro em qualquer etapa: `maxAmountIn` violado, ZK-Pointer rejeitado, revert on-chain, timeout). Adicionar campo `error_code string` para persistir o codigo canonico da falha (FR-057/FR-059).
- [X] T032 [P] [US1] Criar model `ScenarioBRiskControlState` em `backend/services/api-gateway/internal/domain/risk_control.go` com campos de pool pair, threshold 70/30, estado do circuit breaker (`LIVE`, `HALTED`, `RESUME_PENDING`), `resume_request_id`, `resume_signatures_required` (default 2) e `updated_at` (FR-030/data-model.md §5).
- [X] T033 [P] [US1] Criar modelos de inventario em `backend/services/api-gateway/internal/domain/cutover.go`: `ScenarioBApiCutoverPlan`, `ScenarioBEndpointContract`, `LegacyArtifactInventory`, `InfrastructureReuseRegister`. GORM tags completas (FR-055/data-model.md §1-4).
- [X] T034 [US1] Registrar `SwapOrderScenarioB` e `ScenarioBRiskControlState` em `migrate.go` (T012) via `db.AutoMigrate(...)`. Criar `db/init/partitions.go` com funcao `CreatePartitions(db *gorm.DB) error` que executa `db.Exec()` para particao mensal da tabela `swap_order_scenario_b` (FR-046/FR-055).

### AMM EVM Client e Services

- [X] T035 [US1] Implementar cliente EVM do AMM Hub em `backend/shared/blockchain/scenariob/amm/client.go`: metodos `QuoteExactOutput(ctx, pair, amountOut) (*QuoteResult, error)` e `SwapExactOutput(ctx, req SwapRequest) (*SwapResult, error)` via `go-ethereum/ethclient` + ABI embutida (pacote compartilhado `backend/shared/blockchain/scenariob/evm`). Usar constante de endereco do contrato via env `AMM_CONTRACT_ADDRESS` (FR-027/FR-037). **Decisao de implementacao**: adotada ABI JSON embutida ao inves de bindings abigen para simplificar pipeline de build e eliminar dependencia externa de toolchain (ver `backend/shared/blockchain/scenariob/evm/evm.go`).
- [X] T036 [P] [US1] **Substituido**: ao inves de gerar bindings via `abigen`, o cliente AMM usa ABI JSON embutida (`ABIJSON` constante em `client.go`) via `go-ethereum/accounts/abi`. Manutencao sincronizada com `contracts/src/interfaces/IAutomatedMarketMaker.sol` (FR-035/Decision 7). Zero dependencia de binario externo `abigen` no build.
- [X] T037 [US1] Criar `backend/services/api-gateway/internal/services/quote_service.go` com `QuoteService.GetExactOutputQuote(ctx, pair, amountOut) (*QuoteResponse, error)`: verificar pool ativo, invocar `amm.Client.QuoteExactOutput`, retornar `required_input`, `price_impact`, `quote_timestamp`. Performance gate: p95 <= 300ms (FR-037/SC-021).
- [X] T038 [US1] Criar `backend/services/api-gateway/internal/services/pool_status_service.go` com `PoolStatusService.GetPoolStatus(ctx, pair) (*PoolStatusResponse, error)`: ler reservas do AMM, calcular razao, retornar flag de desequilibrio 70/30 e `updated_at` (FR-028/REQ-FX-008).
- [X] T039 [US1] Criar `backend/services/api-gateway/internal/services/swap_service.go` com `SwapService` e `SwapGates` struct contendo `ComplianceGate` e `CircuitBreakerGate` (interfaces). Implementar `Execute(ctx, req) (*SwapResult, error)`: verificar breaker ativo, executar `ComplianceGate.ValidateZKPointer`, calcular input via AMM, verificar `maxAmountIn`, enviar transacao, atualizar `SwapOrderScenarioB` para `SUBMITTED/CONFIRMING/COMPLETED/FAILED`. Persistir `submitted_at` e `confirmed_at`. Performance gate: p95 <= 6s (FR-027/FR-037/FR-057/SC-022).

### RBAC Middleware e ZK Failure Handling

- [X] T040 [P] [US1] Implementar middleware RBAC em `backend/services/api-gateway/internal/http/middleware/scenariob_rbac.go`: `RequireCentralBankRole()` que inspeciona `realm_access.roles` do JWT e exige presenca de `RoleCentralBankScenarioB` (`central_bank`); `RequireCommercialBankRole()` que exige `RoleCommercialBankScenarioB` (`commercial_bank`). Rejeitar com HTTP 403 + `error_code: INSUFFICIENT_ROLE`. Usar constantes de `domain/auth.go` (T017). Aplicar em endpoints conforme FR-056 (FR-056/SC-016).
- [X] T041 [US1] Implementar tratamento de ZK_VALIDATION_FAILED no `SwapService` (T039): quando `ComplianceGate` retornar erro de validacao, retornar HTTP 422 com `error_code: ZK_VALIDATION_FAILED`, transicionar `SwapOrderScenarioB` para `FAILED` (persistir `error_code: ZK_VALIDATION_FAILED`) e manter `BridgedAssetPosition.bridge_state = ACTIVE` — nenhum Unbridging automatico deve ser disparado. O cliente pode retentar o swap com novo ZK-Pointer valido sem iniciar novo Bridging (FR-058/SC-016).

### Handlers HTTP US1

- [X] T042 [P] [US1] Criar handler `GET /api/v2/amm/quote/exact-output` em `backend/services/api-gateway/internal/http/handlers/quote_handler.go`: validar `pair` e `amount_out`, invocar `QuoteService`, retornar JSON com `required_input`, `price_impact`, `quote_timestamp`. Sem autenticacao obrigatoria (endpoint de consulta publico) (FR-027/SC-013).
- [X] T043 [US1] Criar handler `POST /api/v2/amm/swap/exact-output` em `backend/services/api-gateway/internal/http/handlers/swap_handler.go`: validar campos obrigatorios (pair, amount_out, max_amount_in, payer_id, beneficiary_id, zk_pointer_payer, zk_pointer_beneficiary), aplicar `RequireCommercialBankRole()`, invocar `SwapService.Execute`. Retornar HTTP 422 com `error_code` dos dois codigos canonicos (FR-059): `SLIPPAGE_LIMIT_EXCEEDED` (input calculado > max_amount_in) e `INSUFFICIENT_POOL_LIQUIDITY` (reserva zerada/abaixo do minimo). Tambem retornar `ZK_VALIDATION_FAILED` (FR-058). Performance gate: p95 <= 6s (FR-027/FR-037/FR-057/FR-058/FR-059/SC-013).
- [X] T044 [P] [US1] Criar handler `GET /api/v2/amm/pool/{pair}/status` em `backend/services/api-gateway/internal/http/handlers/pool_handler.go`: invocar `PoolStatusService.GetPoolStatus`, retornar `reserves`, `current_ratio`, `imbalance_flag`, `updated_at` (FR-028/SC-013).
- [X] T045 [US1] Registrar rotas US1 em `backend/services/api-gateway/internal/http/router/v2/router.go` (`Register` function): `GET /api/v2/amm/quote/exact-output` → `QuoteHandler`, `POST /api/v2/amm/swap/exact-output` → `SwapHandler` (com `RequireCommercialBankRole()`), `GET /api/v2/amm/pool/{pair}/status` → `PoolHandler`. Confirmar que `router.Dependencies` ja possui os services injetados (T020).

### OpenAPI e Testes US1

- [X] T046 [P] [US1] Preencher `backend/services/api-gateway/openapi/v2/scenario-b.yaml` com os endpoints de US1: `GET /api/v2/amm/quote/exact-output`, `POST /api/v2/amm/swap/exact-output` (incluir schemas de erro `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY`, `ZK_VALIDATION_FAILED` — FR-059), `GET /api/v2/amm/pool/{pair}/status`. Incluir componentes de seguranca bearerAuth (FR-027/FR-059/SC-013).
- [X] T047 [P] [US1] Escrever testes de contrato do quote handler em `backend/services/api-gateway/internal/http/handlers/quote_handler_test.go`: (a) cotacao bem-sucedida retorna `required_input`; (b) par invalido retorna HTTP 400; (c) pool sem liquidez retorna HTTP 422 `INSUFFICIENT_POOL_LIQUIDITY` (FR-027).
- [X] T048 [US1] Escrever testes de contrato do swap handler em `backend/services/api-gateway/internal/http/handlers/swap_handler_test.go`: (a) swap bem-sucedido → `COMPLETED`; (b) `maxAmountIn` violado → HTTP 422 `error_code: SLIPPAGE_LIMIT_EXCEEDED`, `SwapOrderScenarioB.state = FAILED`; (c) pool sem liquidez → HTTP 422 `error_code: INSUFFICIENT_POOL_LIQUIDITY`, `SwapOrderScenarioB.state = FAILED`; (d) ZK-Pointer invalido → HTTP 422 `error_code: ZK_VALIDATION_FAILED`, `BridgedAssetPosition.bridge_state = ACTIVE` (FR-058/FR-059/SC-013).
- [X] T049 [P] [US1] Escrever testes do pool handler em `backend/services/api-gateway/internal/http/handlers/pool_handler_test.go`: (a) status normal retorna ratio e imbalance_flag=false; (b) ratio 72/28 retorna imbalance_flag=true (FR-028).

### E2E Tryout US1

- [X] T050 [US1] Implementar etapas 1-5 do `tryouts/tryout-scenario-b-e2e.sh` para US1: (1) subir infraestrutura local via docker compose (Hub + Spoke A + Spoke B + Keycloak + Postgres + Redis), (2) deploy dos contratos via `forge script`, (3) registrar participantes no IdentityRegistry, (4) provisionar pool com reservas iniciais, (5) executar cotacao + swap feliz + swap com `max_amount_in` baixo (esperando HTTP 422 `SLIPPAGE_LIMIT_EXCEEDED`) + swap com pool vazio (esperando HTTP 422 `INSUFFICIENT_POOL_LIQUIDITY`) + consultar `pool/{pair}/status` (FR-019/FR-020/SC-012/SC-013).

---

## Phase 4: User Story 2 — Reaproveitamento, Bridging, ZK Compliance e Liquidity Monitor (Priority P2)

**Story Goal**: Como lider tecnico, entregar fluxo completo de Lock&Mint/Burn&Unlock com Relayer idempotente, compliance ZK-Pointer, monitoramento de desequilibrio 70/30 e endpoints de reconciliacao.

**Independent Test Criteria**: `POST /api/v2/bridge/lock-mint` e `POST /api/v2/bridge/burn-unlock` refletem estados corretos; Relayer executa 5 retentativas com backoff e escala para `RECONCILIATION_REQUIRED`; Liquidity Monitor detecta desequilibrio 70/30 em p95 <= 15s.

### Docs US2

- [X] T051 [P] [US2] Criar `specs/002-scenario-b-liquidity/cutover/reuse-matrix.md`: tabela de capacidades reaproveitaveis x pendentes com status (disponivel, parcial, nao implementado), mapeando cada entrada a FR correspondente e evidencia de implementacao (FR-004/FR-005/SC-002).

### Models GORM — Bridging e Compliance

- [X] T052 [US2] Criar model `BridgedAssetPosition` em `backend/services/payment-orchestrator/internal/domain/bridged_position.go` com campos: `position_id`, `owner_bank_id`, `spoke_network`, `native_asset`, `mirrored_asset`, `mirrored_amount`, `bridge_state` (enum: `LOCKING`, `ACTIVE`, `BURNING`, `RELEASED`, `RECONCILIATION_REQUIRED`), `relayer_retries`, `last_attempt_at`, `first_attempt_at`, `relayer_error_log` (JSONB). GORM tags completas (FR-029/FR-031/FR-032/data-model.md §6).
- [X] T053 [P] [US2] Criar model `ComplianceZKPointer` em `backend/services/compliance/internal/domain/zk_pointer.go`: `pointer_id`, `bank_id`, `tx_ref`, `commitment_hash`, `proof_cid`, `validated_at`, `expires_at`, `state` (enum: `VALID`, `EXPIRED`, `REVOKED`) (FR-025/FR-058/data-model.md §7).
- [X] T054 [P] [US2] Criar model `RelayerQueueItem` em `backend/services/payment-orchestrator/internal/domain/relayer_queue.go`: `item_id`, `idempotency_key` (hash derivado do evento origem), `event_type` (LOCK_MINT / BURN_UNLOCK), `position_id` (FK), `state` (PENDING/IN_FLIGHT/COMPLETED/FAILED/ESCALATED), `attempt_count`, `next_attempt_at`, `last_error` (FR-031/FR-039/Decision 11).
- [X] T055 [P] [US2] Criar models de monitoramento: `PoolStateReading` em `backend/services/api-gateway/internal/domain/pool_monitoring.go` com `reading_id`, `pool_pair`, `reserve_a`, `reserve_b`, `current_ratio`, `recorded_at`; e `LiquidityAlert` com `alert_id`, `pool_pair`, `ratio_observed`, `threshold`, `alerted_at`, `resolved_at` (FR-028/data-model.md §10-11).
- [X] T056 [P] [US2] Criar model `LiquidityPosition` em `backend/services/api-gateway/internal/domain/liquidity_position.go`: `lp_id`, `provider_bank_id`, `pool_pair`, `token_a_contributed`, `token_b_contributed`, `lp_shares`, `added_at`, `withdrawn_at` (data-model.md §9).
- [X] T057 [US2] Registrar novos modelos de US2 em `migrate.go` (T012): `BridgedAssetPosition`, `ComplianceZKPointer`, `RelayerQueueItem`, `LiquidityPosition`, `PoolStateReading`, `LiquidityAlert`. Adicionar particao mensal para `pool_state_readings` via `db.Exec()` em `partitions.go` (T034). Adicionar triggers append-only para `liquidity_alerts` em `triggers.go` (T021) (FR-046/FR-048/FR-055).

### Clientes EVM US2

- [X] T058 [P] [US2] Implementar cliente SpokeBridge em `backend/shared/blockchain/scenariob/spokebridge/client.go`: metodos `LockAsset(ctx, req) (*LockResult, error)` e `UnlockAsset(ctx, req) (*UnlockResult, error)` via ethclient + ABI embutida de `SpokeBridge.sol`. Endereco via env `SPOKE_BRIDGE_ADDRESS` por rede (FR-029/Decision 6). Relayer Cacti client em `backend/shared/blockchain/scenariob/relayer/client.go` usa HTTP JSON com payloads canonicos (lock-mint / burn-unlock) e idempotency key.
- [X] T059 [P] [US2] **Substituido**: ABI JSON embutida em `backend/shared/blockchain/scenariob/spokebridge/client.go` (constante `ABIJSON`). Bindings `TokenizedCentralBankMoney.sol` e `CommitmentHashRegistry.sol` acessados via ABI embutida quando necessarios pelos services de compliance/liquidity. Zero dependencia de `abigen` no build (FR-019).

### Services e Workers US2

- [X] T060 [US2] Criar `backend/services/payment-orchestrator/internal/services/bridge_lock_mint_service.go`: aceita evento de trava confirmado no Spoke A, cria `BridgedAssetPosition` em `LOCKING`, enfileira `RelayerQueueItem` com chave de idempotencia, atualiza para `ACTIVE` apos confirmacao no Hub (FR-029/FR-031/SC-015).
- [X] T061 [P] [US2] Criar `backend/services/payment-orchestrator/internal/services/bridge_burn_unlock_service.go`: aceita resultado de swap do Hub, cria item de unbridging, transiciona `BridgedAssetPosition` para `BURNING` → `RELEASED`; em caso de falha, enfileira no Relayer com idempotency_key do burn (FR-029/FR-032/SC-015).
- [X] T062 [US2] Criar `backend/services/payment-orchestrator/internal/workers/relayer_worker.go`: poll da tabela `relayer_queue_item` com estados `PENDING`/`IN_FLIGHT`; executar cada item com no maximo 5 tentativas e backoff exponencial (2s, 4s, 8s, 16s, 32s; cap 60s). Idempotencia garantida por `idempotency_key` (verifica se efeito on-chain ja foi aplicado antes de retentar). Apos 5 falhas, transicionar `BridgedAssetPosition` para `RECONCILIATION_REQUIRED` e gerar `LiquidityAlert` com motivo `RELAYER_EXHAUSTED` (FR-031/FR-032/FR-039/SC-018/Decision 11).
- [X] T063 [P] [US2] Criar `backend/services/compliance/internal/services/zk_compliance_gate.go` implementando interface `ComplianceGate`: `ValidateZKPointer(ctx, bankID, txRef, commitmentHash string) error`. Verifica `ComplianceZKPointer` no DB (e futuramente on-chain via `CommitmentHashRegistry`). Retorna erro tipado `ZKValidationError` quando falha (FR-025/FR-058/SC-016).
- [X] T064 [US2] Criar `backend/services/api-gateway/internal/services/liquidity_monitor_service.go`: poll periodico (cadencia alvo p95 <= 15s — FR-037/SC-023) consultando `PoolStatusService.GetPoolStatus`, persistindo `PoolStateReading` via GORM Create, e criando `LiquidityAlert` via GORM Create quando `current_ratio > 0.70`. Log em stdout por cada leitura (qualitativo para esta feature — FR-051/FR-052/SC-014).
- [X] T065 [P] [US2] Criar `backend/services/api-gateway/internal/services/liquidity_provision_service.go`: metodos `AddLiquidity(ctx, req) (*LPResult, error)` e `RemoveLiquidity(ctx, req) (*LPResult, error)` que persistem `LiquidityPosition` via GORM e invocam AMM client (FR-027/data-model.md §9).

### Handlers HTTP US2

- [X] T066 [P] [US2] Criar handler `POST /api/v2/bridge/lock-mint` em `backend/services/api-gateway/internal/http/handlers/bridge_handler.go`: aplicar `RequireCommercialBankRole()`, validar campos, invocar `BridgeLockMintService`, retornar estado da posicao e referencia do Relayer (FR-029/SC-015).
- [X] T067 [P] [US2] Criar handler `POST /api/v2/bridge/burn-unlock` em `bridge_handler.go`: aplicar `RequireCommercialBankRole()`, validar posicao espelhada ativa, invocar `BridgeBurnUnlockService`, retornar estado e referencia do Relayer (FR-029/SC-015).
- [X] T068 [P] [US2] Criar handler `GET /api/v2/bridge/positions` em `bridge_handler.go`: aceitar filtro por `state` (URL query param), retornar lista de `BridgedAssetPosition` com campos de reconciliacao (ids on-chain, retries, erros, timestamps). `RequireCommercialBankRole()` para acesso (FR-033).
- [X] T069 [P] [US2] Criar handler `POST /api/v2/amm/liquidity/add` e `POST /api/v2/amm/liquidity/remove` em `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go`: aplicar `RequireCentralBankRole()` (Bancos Centrais como LPs experimentais), invocar `LiquidityProvisionService` (FR-027/contracts §5).
- [X] T070 [P] [US2] Registrar rotas US2 em `router.go` (T018/T045): bridge, posicoes e liquidity provision routes com middleware de role correto (T040).

### OpenAPI e Testes US2

- [X] T071 [P] [US2] Adicionar ao `scenario-b.yaml` os endpoints de US2: `POST /api/v2/bridge/lock-mint`, `POST /api/v2/bridge/burn-unlock`, `GET /api/v2/bridge/positions`, `POST /api/v2/amm/liquidity/add`, `POST /api/v2/amm/liquidity/remove` com schemas completos (FR-033/SC-015).
- [X] T072 [P] [US2] Testes de contrato dos handlers de bridge em `bridge_handler_test.go`: (a) lock-mint bem-sucedido retorna `LOCKING`; (b) posicao nao encontrada retorna 404; (c) bridge position com state filter correto.
- [X] T073 [P] [US2] Testes unitarios do `RelayerWorker`: (a) item concluido em 1a tentativa; (b) item falha em 3 tentativas, backoff crescente conforme valores canonicos; (c) item exaurido apos 5 falhas → `RECONCILIATION_REQUIRED`; (d) idempotencia: re-envio com mesma chave nao gera efeito duplicado (FR-031/FR-039/SC-018).

### E2E Tryout US2

- [X] T074 [US2] Implementar etapas de US2 no `tryouts/tryout-scenario-b-e2e.sh`: (1) executar Lock&Mint no Spoke A, verificar `BridgedAssetPosition = ACTIVE`; (2) executar swap com ZK-Pointer invalido e verificar HTTP 422 `ZK_VALIDATION_FAILED` + `BridgedAssetPosition = ACTIVE` (FR-058); (3) executar swap bem-sucedido; (4) verificar alerta 70/30 do Liquidity Monitor em stdout; (5) executar Burn&Unlock no Spoke B, verificar `RELEASED`; (6) simular falha do Relayer e verificar escalada para `RECONCILIATION_REQUIRED` (FR-019/FR-020/SC-012/SC-014/SC-015/SC-018).

---

## Phase 5: User Story 3 — Governanca, Circuit Breaker Assimetrico e Master Viewing Key (Priority P3)

**Story Goal**: Como gestor de governanca e compliance, expor Circuit Breaker assimetrico (pause 1-of-N / resume 2-of-N) e fluxo de disclosure do Master Viewing Key (quorum 2-of-3, timeout 72h) com auditabilidade completa.

**Independent Test Criteria**: `POST /api/v2/governance/circuit-breaker/pause` com 1 assinatura de BC pausa o AMM; `POST .../resume-sign` sem quorum 2-of-N e rejeitado com evento `CircuitBreakerResumeDisputed`; disclosure expira automaticamente apos 72h.

### Docs US3

- [X] T075 [P] [US3] Criar `specs/002-scenario-b-liquidity/cutover/governance-runbook.md`: documentar Circuit Breaker assimetrico (pause fail-safe 1-of-N, resume quorum 2-of-N), Master Viewing Key (quorum 2-of-3, timeout 72h), retencao indefinida sem purge jobs, ausencia de rate limiting e hardening contra DBA como riscos aceitos (FR-030/FR-034/FR-047/FR-053/FR-054).

### Contratos Solidity — Circuit Breaker Assimetrico

- [X] T076 [US3] Evoluir `contracts/src/interfaces/IAutomatedMarketMaker.sol` adicionando assinaturas do Circuit Breaker assimetrico (FR-030/FR-044): `pauseCircuitBreaker(bytes calldata signature)`, `proposeResume(bytes calldata signature) returns (bytes32 requestID)`, `signResume(bytes32 requestID, bytes calldata signature)`, `executeResume(bytes32 requestID)`. Adicionar eventos: `CircuitBreakerPaused(address indexed bankID, string reason)`, `CircuitBreakerResumeProposed(bytes32 indexed requestID, address indexed proposer)`, `CircuitBreakerResumeDisputed(bytes32 indexed requestID, address[] collectedSigners)`, `CircuitBreakerResumed(bytes32 indexed requestID)`. Remover assinatura generica `setPause(bool)` da interface.
- [X] T077 [US3] Implementar Circuit Breaker assimetrico em `contracts/src/AutomatedMarketMaker.sol` (FR-030/FR-044/SC-026): (a) `pauseCircuitBreaker`: aceita 1-of-N assinatura de `authorizedCentralBanks`, valida a assinatura, transiciona estado para HALTED, emite `CircuitBreakerPaused`; (b) `proposeResume`: BC autorizado registra solicitacao com `requestID`, emite `CircuitBreakerResumeProposed`; (c) `signResume`: BC distinto adiciona assinatura ao `requestID`; (d) `executeResume`: requer `signatures >= 2` com signatarios distintos — caso contrario emite `CircuitBreakerResumeDisputed` e reverte; sucesso emite `CircuitBreakerResumed` e transiciona para LIVE. Mapping persistente `authorizedCentralBanks` + funcoes de registro/revogacao.
- [X] T078 [P] [US3] Escrever testes Foundry para Circuit Breaker em `contracts/test/AutomatedMarketMaker.t.sol`: (a) pause com 1-of-N → HALTED em 1 bloco (SC-017); (b) resume com 1 assinatura → rejeitado, `CircuitBreakerResumeDisputed`; (c) resume com 2 assinaturas distintas → LIVE; (d) swap rejeitado enquanto HALTED; (e) nao-autorizado nao pode pausar (FR-030/FR-044/SC-017/SC-026). Rodar `forge test --match-contract AutomatedMarketMaker -vv`.
- [X] T079 [P] [US3] **Substituido**: apos evolucao do contrato em T077, a ABI JSON embutida em `backend/shared/blockchain/scenariob/amm/client.go` foi atualizada para incluir `pause(string)`, `proposeResume()`, `signResume(bytes32)`, `isPaused()`, `resumeQuorum()`, `resumeSignatures(bytes32)`. Nenhum `abigen` executado (ver Decision 7 revisada).

### Models GORM — Governance

- [X] T080 [US3] Criar model `CircuitBreakerSignature` em `backend/services/api-gateway/internal/domain/circuit_breaker.go`: `signature_id`, `control_id` (FK), `event_kind` (PAUSE/RESUME), `request_id`, `signer_bank_id`, `signer_wallet`, `signature_payload` (bytes), `on_chain_tx_ref`, `signed_at`. Regras: PAUSE requer 1 registro; RESUME requer >= 2 com `signer_bank_id` unico por `request_id`. Append-only (FR-030/FR-048/data-model.md §5a).
- [X] T081 [P] [US3] Criar models de oversight em `backend/services/compliance/internal/domain/disclosure.go`: `DisclosureRequest` (`request_id`, `tx_ref`, `requestor_bank_id`, `state` enum PENDING/APPROVED/DENIED/EXPIRED, `created_at`, `expires_at`, `quorum_required: 2`, `quorum_reached: int`) e `DisclosureSignature` (`sig_id`, `request_id` FK, `signer_bank_id`, `signed_at`, `signature_payload`). Append-only (FR-034/FR-035/FR-036/data-model.md §12-13).
- [X] T082 [US3] Registrar novos modelos de US3 em `migrate.go` (T012): `CircuitBreakerSignature`, `DisclosureRequest`, `DisclosureSignature`. Adicionar triggers append-only para `circuit_breaker_signatures` e `disclosure_signatures` em `triggers.go` (T021) (FR-048/FR-055).

### Paladin Client e OversightService

- [X] T083 [P] [US3] Implementar cliente Paladin JSON-RPC em `backend/shared/blockchain/scenariob/paladin/client.go`: struct `PaladinClient` com metodo `RequestMasterViewingKeyDisclosure(ctx, txID, requestID string) error` e `GetDisclosureResult(ctx, requestID string) (*DisclosureResult, error)` via HTTP calls ao endpoint Paladin (`ethclient`-style). Constante de URL configuravel via env `PALADIN_RPC_URL`. **Nenhuma dependencia gRPC** introduzida — exclusivamente JSON-RPC over HTTP (FR-034/M4). Testes unitarios em `paladin/client_test.go` com servidor HTTP mockado.
- [X] T084 [US3] Criar `backend/services/compliance/internal/services/oversight_service.go` com `OversightService`: `OpenDisclosure(ctx, txRef, requestorID string) (*DisclosureRequest, error)` — cria request com `expires_at = now + 72h`; `SignDisclosure(ctx, requestID, signerID string) error` — adiciona `DisclosureSignature` via GORM Create, verifica quorum 2-of-3 e chama `PaladinClient.RequestMasterViewingKeyDisclosure` quando atingido; `GetDisclosureStatus(ctx, requestID string) (*DisclosureRequest, error)`. Persistir todas as operacoes como append-only (FR-034/FR-035/FR-036/SC-027).
- [X] T085 [P] [US3] Criar job `backend/services/compliance/internal/workers/disclosure_expiry_worker.go`: poll periodico (a cada minuto) buscando `DisclosureRequest` com `state = PENDING` e `expires_at < now`, transicionando para `EXPIRED` via GORM + gerando entrada de audit via `audit.Append`. Idempotente (FR-035/SC-027).

### CircuitBreakerService e BreakerGate

- [X] T086 [US3] Criar `backend/services/api-gateway/internal/services/circuit_breaker_service.go`: metodos `Pause(ctx, bankID, reasonCode string, signature []byte) error` — valida role central_bank, chama `amm.Client.PauseCircuitBreaker`, persiste `CircuitBreakerSignature` via GORM Create, atualiza `ScenarioBRiskControlState.circuit_breaker_state = HALTED`; `ProposeResume(ctx, bankID string, sig []byte) (requestID string, error)` — chama `proposeResume` no AMM, persiste signature; `SignResume(ctx, requestID, bankID string, sig []byte) error` — chama `signResume` no AMM, persiste signature adicional; `ExecuteResume(ctx, requestID string) error` — chama `executeResume` no AMM, retorna disputa ou sucesso (FR-030/FR-044/SC-017/SC-026).
- [X] T087 [US3] Criar `backend/services/api-gateway/internal/services/circuit_breaker_gate.go` implementando interface `CircuitBreakerGate`: `IsHalted(ctx, pair string) (bool, error)` — consultando `ScenarioBRiskControlState.circuit_breaker_state`. Wired em `app.go` (T020) via `SwapGates.CircuitBreakerGate` (FR-030/H4).

### Handlers HTTP US3

- [X] T088 [P] [US3] Criar handler `POST /api/v2/governance/circuit-breaker/pause` em `backend/services/api-gateway/internal/http/handlers/governance_handler.go`: aplicar `RequireCentralBankRole()`, validar `reason_code` e `signature`, invocar `CircuitBreakerService.Pause`, retornar estado atualizado e `control_id` (FR-030/SC-017).
- [X] T089 [P] [US3] Criar handlers `POST /api/v2/governance/circuit-breaker/resume-request`, `POST /api/v2/governance/circuit-breaker/resume-sign` e `GET /api/v2/governance/circuit-breaker/status` em `governance_handler.go`: aplicar `RequireCentralBankRole()`, invocar `CircuitBreakerService`, retornar estado intermedio (`RESUME_PENDING`) ou final (`LIVE`) apos quorum (FR-030/FR-044/SC-026).
- [X] T090 [P] [US3] Criar handlers de oversight em `backend/services/api-gateway/internal/http/handlers/oversight_handler.go`: `POST /api/v2/oversight/disclosure-request`, `POST /api/v2/oversight/disclosure-sign`, `GET /api/v2/oversight/disclosure-status/{requestID}`. Aplicar `RequireCentralBankRole()` em todos (FR-034/FR-035/FR-036/SC-027).
- [X] T091 [US3] Registrar rotas US3 em `router.go`: governance circuit-breaker routes, oversight routes. Todos com `RequireCentralBankRole()` (T040). Verificar que `router.Dependencies` inclui `GovernanceService` e `OversightService` via `app.go` (T020).

### OpenAPI e Testes US3

- [X] T092 [P] [US3] Adicionar ao `scenario-b.yaml` os endpoints de US3: circuit-breaker pause/resume-request/resume-sign/status, oversight disclosure-request/disclosure-sign/disclosure-status. Incluir schemas de erro especificos (FR-030/FR-034).
- [X] T093 [P] [US3] Testes de contrato dos handlers de governance em `governance_handler_test.go`: (a) pause com role `central_bank` → HTTP 200 + HALTED; (b) pause sem role `central_bank` → HTTP 403; (c) resume-sign com quorum insuficiente → HTTP 422 + `CircuitBreakerResumeDisputed`; (d) resume-sign com quorum completo → HTTP 200 + LIVE (FR-030/FR-044/SC-026).
- [X] T094 [P] [US3] Testes de contrato dos handlers de oversight em `oversight_handler_test.go`: (a) abrir disclosure cria request com `expires_at = now+72h`; (b) assinar sem quorum retorna `PENDING`; (c) assinar com quorum aciona Paladin mock; (d) request expirado retorna 422 (FR-034/FR-035/SC-027).
- [X] T095 [P] [US3] Verificar que triggers append-only bloqueiam UPDATE/DELETE em `circuit_breaker_signatures` e `disclosure_signatures`: executar testes de integracao Postgres em `triggers_test.go` (T021) confirmando que excecao e lancada (FR-048/FR-049/SC-030).

### E2E Tryout US3

- [X] T096 [US3] Implementar etapas de US3 no `tryouts/tryout-scenario-b-e2e.sh`: (1) pause do Circuit Breaker com 1 BC (verificar HALTED em 1 bloco — SC-017); (2) tentar swap com AMM pausado (verificar falha controlada); (3) resume-request + resume-sign com 1 assinatura (verificar `CircuitBreakerResumeDisputed`); (4) resume-sign com 2a assinatura distinta (verificar LIVE); (5) abrir disclosure request; (6) assinar com 1 BC; (7) assinar com 2o BC (verificar chamada ao Paladin mock); (8) simular expiracao de disclosure pelo worker (FR-019/FR-020/SC-012/SC-026/SC-027).

---

## Phase 6: Polish, Cutover Validation e Performance Baseline

**Purpose**: Fechar o inventario de Cenario A, validar zero referencias legadas, garantir imutabilidade dos audit logs, benchmark de performance, seed de dados iniciais e preparar o pacote de cutover.

- [X] T097 Atualizar `specs/002-scenario-b-liquidity/cutover/cutover-plan.yaml` status para `IN_PROGRESS` ou `COMPLETED`: preencher `executed_at`, `legacy_artifacts_removed` (deve ser igual a `legacy_artifacts_total` — SC-008), e listar os componentes de infra reaproveitados (FR-016/SC-006/SC-007/SC-008/SC-009/SC-010).
- [X] T098 [P] Realizar sweep final de referencias ao Cenario A no repositorio: `git grep -rn "scenario.a\|scenarioA\|scenario_a\|CenarioA" -- backend/ contracts/` deve retornar zero matches funcionais. Registrar evidencia em `docs-editorial-review.md` (SC-010/SC-001).
- [X] T099 [P] Verificar que `frontend/apps/` tem zero arquivos modificados nesta feature: `git diff --name-only HEAD~1 -- frontend/` deve retornar vazio (SC-025).
- [X] T100 [P] Validar que nenhum arquivo `.sql` de migration foi criado: `find backend/ -name "*.sql" | grep -v "_test\|fixture"` deve retornar vazio (FR-055).
- [X] T101 [P] Rodar `forge build && forge test -vv` completo: zero failures antes de cutover. Documentar saida em `specs/002-scenario-b-liquidity/cutover/forge-test-evidence.md` (FR-022/SC-012).
- [X] T102 [P] Rodar `go test ./... -v` em `backend/services/api-gateway/`, `backend/services/payment-orchestrator/` e `backend/services/compliance/`: zero failures. Documentar saida (FR-019/FR-020/SC-012).
- [X] T103 Criar seed inicial de `ScenarioBEndpointContract` em `backend/services/api-gateway/internal/db/seeds/endpoint_contract_seed.go`: inserir via GORM Create os endpoints ativos da API v2 (quote, swap, pool, bridge, governance, oversight). Sem arquivos `.sql` (FR-055/data-model.md §2).
- [X] T104 [P] Executar tryout E2E completo `bash tryouts/tryout-scenario-b-e2e.sh`: todas as 7 etapas (US1 + US2 + US3) devem passar com exit code 0. Capturar log de saida como evidencia (FR-019/FR-020/SC-012).
- [X] T105 [P] Validacao de performance com `k6` ou `hey` em ambiente local: coletar p95 de `GET /api/v2/amm/quote/exact-output` (gate: <= 300ms — SC-021), `POST /api/v2/amm/swap/exact-output` (gate: <= 6s — SC-022) e cadencia do Liquidity Monitor via polling de `GET /api/v2/amm/pool/{pair}/status` (gate: p95 <= 15s — SC-023). Registrar script em `tests/performance/scenario-b-perf.js` e resultados em `specs/002-scenario-b-liquidity/cutover/performance-baseline.md` com timestamp, ambiente e percentis p50/p95/p99. Desvio > 20% dos alvos bloqueia merge (FR-037/FR-038/Decision 13/SC-021/SC-022/SC-023).
- [X] T106 [P] Validar imutabilidade dos audit logs via integracao: tentar UPDATE e DELETE em `circuit_breaker_signatures`, `disclosure_signatures`, `liquidity_alerts` e confirmar excecao lancada pelos triggers (FR-048/FR-049/SC-030).
- [X] T107 [P] Rodar validacao de schema GORM apos startup: `db.AutoMigrate` deve ser idempotente (segunda execucao nao altera schema). Verificar existencia das particoes em `swap_order_scenario_b` e `pool_state_readings` apos `CreatePartitions` (FR-046/FR-055).
- [X] T108 [P] Finalizar `backend/services/api-gateway/openapi/v2/scenario-b.yaml`: validar com `swagger-cli validate` ou `redocly lint`, confirmar que todos os 59 endpoints estao documentados com schemas de erro corretos (incluindo `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY`, `ZK_VALIDATION_FAILED`, `INSUFFICIENT_ROLE`) e que zero referencias ao Cenario A existem (FR-059/SC-004).
- [X] T109 [P] Atualizar `specs/002-scenario-b-liquidity/quickstart.md` com passo-a-passo final do rebuild: como rodar setup, como executar tryout E2E, como verificar performance baseline e como confirmar o cutover no `cutover-plan.yaml`.
- [X] T110 Revisao editorial final do `docs/external/scenario-b/scenario-b.md`: verificar 0 referencias ao Cenario A, linguagem adequada para stakeholders nao tecnicos, tabela de status de capacidades completa, rastreabilidade para FRs. Gerar `docs/external/scenario-b/glossary.md` com termos canonicos (FR-001 a FR-009/SC-001/SC-003/SC-004/SC-005).

---

## Phase Dependencies (Story Completion Order)

```
Phase 1 (Setup T001-T007)
  └─> Phase 2 (Foundational T008-T023)
        ├─> Phase 3 (US1 T024-T050)  ← MVP: entrega isolada de docs + quote/swap/pool
        │     └─> Phase 4 (US2 T051-T074)  ← depende de T031/T032/T039/T040/T063
        │           └─> Phase 5 (US3 T075-T096)  ← depende de T031/T057/T063/T083
        │                 └─> Phase 6 (Polish T097-T110)
        └─> [T022 Foundry baseline pode correr em paralelo com Phase 3]
```

**Parallel Execution — Phase 3 (US1)**:

```
T024,T025,T026,T027,T028,T029,T030 podem rodar em paralelo (docs)
T031 → T034 (sequencial: model → AutoMigrate)
T035,T036,T037,T038,T039 (clients/services)
T040,T041 (middleware, T041 depende de T039)
T042,T043,T044 podem rodar em paralelo apos T037-T040
T045 depende de T042-T044
T046,T047,T048,T049 podem rodar em paralelo apos handlers
T050 depende de T045-T049 (tryout requer handlers funcionais)
```

**Parallel Execution — Phase 4 (US2)**:

```
T051 (docs) paralelo
T052,T053,T054,T055,T056 (models) paralelos entre si
T057 → depende de T052-T056
T058,T059 (clients) paralelos
T060,T061 sequenciais (bridge lock antes de burn)
T062 depende de T060,T061 (worker precisa dos services)
T063,T064,T065 paralelos entre si
T066,T067,T068,T069 paralelos (handlers)
T070 depende de T066-T069
T071,T072,T073 paralelos (openapi e testes)
T074 depende de T070-T073
```

**Parallel Execution — Phase 5 (US3)**:

```
T075 (docs) paralelo
T076 → T077 → T078 (contrato interface → implementacao → testes)
T079 (bindings) paralelo apos T077
T080,T081 (models) paralelos entre si
T082 depende de T080,T081
T083,T084 paralelos entre si (Paladin client + OversightService)
T085 depende de T084
T086 → T087 sequenciais (service → gate)
T088,T089,T090 paralelos (handlers)
T091 depende de T088-T090
T092,T093,T094,T095 paralelos (openapi e testes)
T096 depende de T091-T095
```

---

## Implementation Strategy

**MVP (Phase 3 / US1 completo)**:
- docs-external editadas e sem Cenario A
- `GET /api/v2/amm/quote/exact-output` funcional (p95 <= 300ms)
- `POST /api/v2/amm/swap/exact-output` funcional com `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY` e `ZK_VALIDATION_FAILED`
- `GET /api/v2/amm/pool/{pair}/status` funcional
- RBAC middleware aplicado
- tryout etapas 1-5 passando

**Full Delivery (Phases 4-6)**:
- Bridging Lock&Mint / Burn&Unlock com Relayer idempotente (5 retries/backoff)
- Liquidity Monitor 70/30 (p95 <= 15s)
- Circuit Breaker assimetrico (pause 1-of-N / resume 2-of-N) on-chain
- Master Viewing Key multi-sig 2-of-3 com expiry 72h
- Performance baseline documentado (k6/hey)
- Tryout E2E 7 etapas com exit code 0
- Zero references Cenario A; Zero .sql migrations; Zero frontend changes

---

## Phase 7: Tryout Hotfix — Bugs Detectados em 2026-05-04 (Clarificacoes Sessions 2026-05-04)

**Purpose**: Corrigir as 5 falhas confirmadas pelo output real do tryout E2E de 2026-05-04. Todas as tasks desta fase operam exclusivamente no script `tryouts/tryout-scenario-b-e2e.sh`, em handlers e no OpenAPI — nenhuma alteracao de schema ou logica de dominio nova e necessaria.

**Story Goal**: Garantir que `bash tryouts/tryout-scenario-b-e2e.sh all` executa com exit code 0 em todas as 7 etapas, atendendo SC-012 (tryout sem deps Cenario A), SC-015 (bridging round-trip) e FR-019/FR-020.

**Independent Test Criteria**: `bash tryouts/tryout-scenario-b-e2e.sh all` deve completar sem `[ERROR]` em nenhuma etapa. Pool deve ter reservas > 0 antes de US1; Burn&Unlock deve encontrar posicao `ACTIVE`; Remove Liquidity deve encontrar posicao por `lp_id`; pool status deve exibir `reserve_a`/`reserve_b`; resume-sign com 2a assinatura deve retornar `state: LIVE`.

**Evidencia de falha (2026-05-04)**:
- Pool: `reserve_a: "0"` em step4 → US1 falha com `INSUFFICIENT_POOL_LIQUIDITY` em 100% dos casos
- Burn&Unlock: `active bridged position not found: record not found` (posicao ainda em `LOCKING`)
- Remove Liquidity: `active liquidity position not found: record not found` (`lp_shares` hardcoded incorreto)
- Pool status jq: `(warn)` silencioso porque selector `.reserves` nao existe no schema plano
- Resume-sign: ja funciona corretamente (retorna `LIVE`) — task e de documentacao/OpenAPI apenas

### C1 — Pool zerado invalida US1 (Decision 19 / FR-019)

- [X] T111 [US1] Implementar `step4b_seed_liquidity()` em `tryouts/tryout-scenario-b-e2e.sh`: chamar `POST /api/v2/amm/liquidity/add` usando `CENTRAL_BANK_A_TOKEN` (role `central_bank`) com `pool_pair=BRL-USD`, `token_a_amount=100000`, `token_b_amount=100000`, `provider_bank_id=central_bank_a`; verificar que `reserve_a` e `reserve_b` sao `> 0` apos provisao via `GET /api/v2/amm/pool/BRL-USD/status`; falhar com `fail "step4b: pool still empty after seed"` se reservas permanecerem zeradas. Funcao deve ser chamada entre `step4_pool` e `step5_us1` no dispatcher ao final do script (FR-019/Decision 19).
- [X] T112 [P] [US1] Atualizar `step4_pool()` em `tryouts/tryout-scenario-b-e2e.sh`: substituir selector jq `.reserves` por `.reserve_a` na verificacao de disponibilidade do endpoint (Decision 23 / D2). Log de `(warn)` deve exibir `reserve_a` e `reserve_b` em vez de um objeto `.reserves` inexistente.

### C2 — Burn&Unlock prematuro sem aguardar ACTIVE (Decision 20 / FR-029)

- [X] T113 [US2] Implementar `wait_for_position_state()` em `tryouts/tryout-scenario-b-e2e.sh`: funcao Bash `wait_for_position_state <position_id> <target_state> [timeout_secs] [interval_secs]` que faz poll em `GET /api/v2/bridge/positions` (filtrando pelo `position_id` via jq) a cada 5s ate o `bridge_state` igualar `<target_state>` ou o timeout de 120s ser atingido; ao esgotar, chamar `fail "Timeout waiting for position $1 to reach $2"`. Inserir chamada `wait_for_position_state "$POSITION_ID" "ACTIVE" 120 5` em `step6_us2()` imediatamente apos `POST /api/v2/bridge/lock-mint` e antes de `POST /api/v2/bridge/burn-unlock` (Decision 20 / FR-029 / SC-015).

### C3 — lp_id nao capturado para Remove Liquidity (Decision 21 / FR-027)

- [X] T114 [US2] Corrigir captura de `lp_id` em `step6_us2()` em `tryouts/tryout-scenario-b-e2e.sh`: (a) capturar resposta completa de Add Liquidity em variavel `ADD_RESP`; (b) extrair `LP_ID=$(echo "$ADD_RESP" | jq -r '.lp_id // empty')`; (c) falhar com `fail "Add Liquidity did not return lp_id"` se vazio; (d) construir body de Remove Liquidity usando `$LP_ID`: `jq -cn --arg id "$LP_ID" --arg bank "bank_a" '{lp_id:$id, pool_pair:"BRL-USD", provider_bank_id:$bank}'`; remover o `lp_shares: "500"` hardcoded (Decision 21 / SC-015 / contracts/scenario-b-backend-api.md §5).

### I1 — Confirmar e documentar resume automatico (Decision 22 / FR-030)

- [X] T115 [P] [US3] Atualizar `backend/services/api-gateway/openapi/v2/scenario-b.yaml`: no schema de response de `POST /api/v2/governance/circuit-breaker/resume-sign`, documentar explicitamente que quando `signatures_collected >= 2` o campo `state` retorna `"LIVE"` (transicao automatica — Decision 22); quando `signatures_collected < 2`, retorna `"RESUME_PENDING"`; remover qualquer referencia a endpoint `executeResume` do contrato OpenAPI se presente (FR-030/Decision 22).

### Validacao pos-hotfix

- [X] T116 [P] Executar `bash tryouts/tryout-scenario-b-e2e.sh all` apos T111-T115 e confirmar: (a) step4b retorna `reserve_a > 0`; (b) step5/US1 retorna cotacao e swap com sucesso; (c) step6/US2 aguarda `ACTIVE` antes de Burn&Unlock e Remove usa `lp_id`; (d) step7/US3 retorna `state: LIVE` apos 2a assinatura. Capturar saida completa como evidencia de SC-012 (FR-019/FR-020/SC-012/SC-015).

---

## Phase 7 Dependencies

```
T111 (seed liquidity — pre-condicao US1)
T112 [P] (fix jq selector — independente)
T113 (wait_for_position_state — depende de T111 para contexto)
T114 (lp_id capture — depende de T111 para contexto)
T115 [P] (OpenAPI resume doc — independente)
T116 depende de T111-T115 (validacao final)
```

**Parallel Execution — Phase 7**:
```
T111 → T113 (sequencial: seed antes de validar polling de posicao)
T112 paralelo (arquivo mesmo, secao diferente de step4_pool)
T114 paralelo com T113 (secoes independentes de step6_us2)
T115 paralelo (arquivo OpenAPI diferente)
T116 depende de todos acima
```

---

## Updated Implementation Strategy

**Hotfix Delivery (Phase 7)**:
- `step4b_seed_liquidity()` provisionando 100k BRL + 100k USD antes de US1
- `wait_for_position_state()` com timeout 120s e intervalo 5s para LOCKING → ACTIVE
- Remove Liquidity usando `lp_id` (UUID) capturado do Add Liquidity
- Pool status jq selector `.reserve_a` (schema plano confirmado)
- OpenAPI de `resume-sign` documentando transicao automatica ao quorum
- `bash tryouts/tryout-scenario-b-e2e.sh all` passando com exit code 0 (SC-012)

