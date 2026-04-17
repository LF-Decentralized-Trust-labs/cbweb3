# Tasks: Harden FX Agreement for Production

**Input**: Design documents from `/specs/001-harden-fx-agreement/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Nao foram adicionadas tarefas de testes automatizados dedicadas, pois a especificacao nao exige abordagem TDD explicita. A validacao independente por historia segue os criterios de teste da spec.

**Organization**: Tasks grouped by user story to enable independent implementation and validation.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Preparar base de arquivos, configuracao e contratos para iniciar implementacao sem bloqueios de estrutura.

- [X] T001 Criar arquivo de migracao inicial para acordos FX em `deploy/local/postgres/migrations/payment_orchestrator/001_fx_agreements.sql`
- [X] T002 Criar arquivo de migracao de eventos de auditoria em `deploy/local/postgres/migrations/payment_orchestrator/002_fx_agreement_events.sql`
- [X] T003 [P] Criar arquivo de migracao de confiabilidade do relay em `deploy/local/postgres/migrations/payment_orchestrator/003_relay_delivery_records.sql`
- [X] T004 [P] Definir variaveis de ambiente de FX persistente e relay auth em `backend/config/.env.infra.bank-a.example` e `backend/config/.env.infra.bank-b.example`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Infraestrutura de codigo que bloqueia todas as historias ate estar concluida.

**⚠️ CRITICAL**: Nenhuma historia deve iniciar antes de concluir esta fase.

- [X] T005 Criar porta de repositorio de FX em `backend/services/payment-orchestrator/internal/ports/fx_agreement_repository.go`
- [X] T006 [P] Criar modelos GORM para acordo e evento em `backend/services/payment-orchestrator/internal/repository/fx_agreement_models.go`
- [X] T007 Implementar repositorio PostgreSQL base em `backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go`
- [X] T008 Integrar conexao de banco e repositorio FX no bootstrap em `backend/services/payment-orchestrator/cmd/payment-orchestrator/main.go`
- [X] T009 Refatorar configuracao do servidor para injetar repositorio FX em `backend/services/payment-orchestrator/internal/grpc/server/server.go`
- [X] T010 [P] Criar middleware de autenticacao de endpoint interno do relay em `backend/services/api-gateway/internal/http/middleware/internal_relay_auth.go`
- [X] T011 Aplicar middleware de relay auth nas rotas internas de FX em `backend/services/api-gateway/internal/http/router/router.go`
- [X] T012 [P] Adicionar parsing de configuracao de relay auth no processo TS em `interop/hub-and-spoke/cacti/src/index.ts`

**Checkpoint**: Foundation ready - historias podem ser implementadas.

---

## Phase 3: User Story 1 - Persistir e auditar acordos FX (Priority: P1) 🎯 MVP

**Goal**: Garantir durabilidade dos acordos e trilha de auditoria completa por transicao de estado.

**Independent Test**: Criar acordo, transicionar estados, reiniciar payment-orchestrator e verificar estado + historico preservados para o mesmo `trade_id`.

### Implementation for User Story 1

- [X] T013 [US1] Implementar operacoes `CreateAgreement` e `GetAgreement` no repositorio em `backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go`
- [X] T014 [P] [US1] Implementar operacao `ListAgreements` com filtros de estado/contraparte em `backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go`
- [X] T015 [P] [US1] Implementar append-only de eventos (`CreateAuditEvent`) em `backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go`
- [X] T016 [US1] Persistir proposta de acordo e validacoes de `expiry_date`/`rate` em `backend/services/payment-orchestrator/internal/grpc/server/server.go`
- [X] T017 [US1] Persistir transicoes `accept/reject/cancel/settle` com evento auditor em `backend/services/payment-orchestrator/internal/grpc/server/server.go` (incluindo a transicao `ACCEPTED -> CANCELLED` — valida que apenas o originador pode cancelar um acordo ja aceito e que o relay propaga o estado terminal para o spoke contraparte)
- [X] T018 [US1] Refatorar handlers `GetFXAgreement` e `ListFXAgreements` para leitura via repositorio em `backend/services/payment-orchestrator/internal/grpc/server/server.go`
- [X] T019 [US1] Criar job de expiracao automatica de acordos `PROPOSED` em `backend/services/payment-orchestrator/internal/workers/fx_expiration_worker.go`
- [X] T020 [US1] Registrar e iniciar worker de expiracao no bootstrap do servico em `backend/services/payment-orchestrator/cmd/payment-orchestrator/main.go`
- [X] T021 [US1] Expor consulta de eventos de auditoria por `trade_id` no gRPC em `apis/proto/payment_orchestrator/v1/payment_orchestrator.proto`
- [X] T022 [US1] Implementar handler gRPC da consulta de eventos em `backend/services/payment-orchestrator/internal/grpc/server/server.go`
- [X] T023 [US1] Expor endpoint HTTP de auditoria de acordos em `backend/services/api-gateway/internal/http/router/router.go`

**Checkpoint**: US1 funcional, com estado duravel e auditoria consultavel.

---

## Phase 4: User Story 2 - Sincronizar acordos entre spokes com confiabilidade (Priority: P2)

**Goal**: Garantir propagacao cross-spoke com deduplicacao duravel, retry robusto e cobertura de estados terminais.

**Independent Test**: Simular indisponibilidade da contraparte, confirmar reentrega com backoff e ausencia de duplicacao apos restart do relay.

### Implementation for User Story 2

- [X] T024 [P] [US2] Criar armazenamento de deduplicacao/pendencias do relay em `interop/hub-and-spoke/cacti/src/relay-store.ts`
- [X] T025 [US2] Integrar deduplicacao persistente por idempotency key no loop de FX em `interop/hub-and-spoke/cacti/src/htlc-relay.ts`
- [X] T026 [US2] Implementar fila de retry com backoff exponencial para falhas de forward em `interop/hub-and-spoke/cacti/src/htlc-relay.ts`
- [X] T027 [P] [US2] Adicionar suporte de propagacao para estados `CANCELLED` e `SETTLED` em `interop/hub-and-spoke/cacti/src/htlc-relay.ts`
- [X] T028 [US2] Injetar cabecalho de autenticacao no polling interno de FX em `interop/hub-and-spoke/cacti/src/htlc-relay.ts`
- [X] T029 [US2] Validar cabecalho `X-Relay-Auth` no endpoint interno de FX em `backend/services/api-gateway/internal/http/middleware/internal_relay_auth.go`
- [X] T030 [US2] Instrumentar logs operacionais de retry/lag para reconciliacao em `interop/hub-and-spoke/cacti/src/htlc-relay.ts`

**Checkpoint**: US2 funcional com entrega resiliente e autenticada entre spokes.

---

## Phase 5: User Story 3 - Garantir acordo bilateral privado com execucao HTLC segura (Priority: P3)

**Goal**: Integrar contexto privado bilateral (Pente) e reforcar vinculo Agreement-HTLC sem bypass.

**Independent Test**: Criar acordo com contexto privado, aceitar acordo e validar lock HTLC apenas para acordo aceito, nao expirado e com termos consistentes.

### Implementation for User Story 3

- [X] T031 [P] [US3] Adicionar campos `group_id` e `contract_address` no contrato proto em `apis/proto/payment_orchestrator/v1/payment_orchestrator.proto`
- [X] T032 [US3] Regenerar artefatos protobuf compartilhados em `backend/shared/proto/payment_orchestrator/v1/payment_orchestrator.pb.go`
- [X] T033 [P] [US3] Criar porta de cliente Pente em `backend/services/payment-orchestrator/internal/ports/pente.go`
- [X] T034 [US3] Implementar adapter HTTP do Pente em `backend/services/payment-orchestrator/internal/adapters/paladin/pente_client.go`
- [X] T035 [US3] Integrar criacao/reuso de contexto privado no fluxo de aceite em `backend/services/payment-orchestrator/internal/grpc/server/server.go`
- [X] T036 [US3] Persistir `group_id` e `contract_address` por `trade_id` no repositorio em `backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go`
- [X] T037 [US3] Reforcar validacao de lock HTLC (estado, expiração, receiver, amount e fail-closed) em `backend/services/payment-orchestrator/internal/grpc/server/server.go`
- [X] T038 [US3] Configurar flags/envs de modo estrito para Agreement-HTLC e Pente em `backend/services/payment-orchestrator/cmd/payment-orchestrator/main.go`
- [X] T039 [US3] Ativar gate de enforcement on-chain do acordo FX seguindo Decision 4 de research.md: se Pente externalCalls disponivel, integrar `accept()` com `HashTimeLockedContract.lock()` via Pente na mesma transacao EVM; caso contrario, implementar registro de compromisso verificável (`keccak256(tradeId | originAmount | counterAmount | rate)`) e validar presenca do hash no `lock()` em `contracts/src/HashTimeLockedContract.sol`

**Checkpoint**: US3 funcional com contexto bilateral privado e lock HTLC fortemente vinculado ao acordo.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Consolidar governanca operacional, documentacao e validacao final.

- [X] T040 [P] Atualizar guia de arquitetura da evolucao FX Agreement em `docs/architecture/fx-agreement-production-hardening.md`
- [X] T041 [P] Criar runbook de reconciliacao e divergencia cross-spoke em `docs/runbooks/fx-agreement-reconciliation.md`
- [X] T042 Executar e registrar validacao de quickstart com resultado final em `specs/001-harden-fx-agreement/quickstart.md`

---

## Phase 7: Paladin Local Real Operation (Zeto + Pente) — REQUIRED DELIVERABLE

**Purpose**: Garantir operacionalização local OBRIGATÓRIA de Zeto E Pente simultaneamente no Paladin para validação de produção. Este é um deliverable crítico para demonstrar que o ambiente suporta ambos os subsistemas de forma funcional em operação real.

**Scope**: Fechar o gap entre integração de aplicação e operacionalização local de Pente no Paladin (Zeto já automatizado), garantindo que ambos funcionar lado a lado com suporte operacional completo.

- [X] T043 Criar manifest YAML de deploy obrigatório do FXAgreement para contexto privado no Paladin em `deploy/local/paladin/contracts/core_v1alpha1_smartcontractdeployment_fx_agreement.yaml` (REQUIRED)
- [X] T044 [P] Criar script de bootstrap obrigatório para contexto bilateral Pente (create/reuse) em `deploy/local/paladin/scripts/create_pente_context_test.go` (REQUIRED)
- [X] T045 [P] Criar script de deploy obrigatório do FXAgreement no contexto Pente em `deploy/local/paladin/scripts/deploy_fxagreement_pente_test.go` (REQUIRED)
- [X] T046 Adicionar targets de make OBRIGATÓRIOS para setup Pente por spoke em `make/40-paladin.mk` (`paladin.deploy-pente-spoke-a`, `paladin.deploy-pente-spoke-b`) — deve ser invocado automaticamente por pipeline principal
- [X] T047 Atualizar pipeline `setup-spoke-a` e `setup-spoke-b` para INCLUIR OBRIGATORIAMENTE bootstrap Pente (sem flag opcional) em `make/40-paladin.mk` — Zeto + Pente são ambos REQUIRED, não optional
- [X] T048 [P] Atualizar templates/configs de nó Paladin para parâmetros necessários do fluxo Pente em `deploy/local/paladin/spoke-a/config/*/config.yaml.tmpl` e `deploy/local/paladin/spoke-b/config/*/config.yaml.tmpl` (REQUIRED)
- [X] T049 Ajustar envs de backend para operação local Pente obrigatória (PENTE_ENABLED=true ALWAYS) em `backend/config/.env.infra.bank-a` e `backend/config/.env.infra.bank-b` — Pente NÃO é feature-flagged em produção local
- [X] T050 Executar validação integrada local (Zeto + Pente AMBOS operacionais) e registrar evidências em `specs/001-harden-fx-agreement/quickstart.md` — deve demonstrar execução bem-sucedida de ambos os subsistemas

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: sem dependencias.
- **Phase 2 (Foundational)**: depende da Phase 1 e bloqueia todas as historias.
- **Phase 3 (US1)**: depende da Phase 2.
- **Phase 4 (US2)**: depende da Phase 2; pode ocorrer em paralelo com US1 se houver equipe.
- **Phase 5 (US3)**: depende da Phase 2; recomenda-se iniciar apos estabilizar US1.
- **Phase 6 (Polish)**: depende da conclusao das historias alvo.
- **Phase 7 (Paladin Local Real Operation)**: depende das Phases 4 e 6; pode iniciar após estabilidade dos fluxos de FX/HTLC e relay.

### User Story Dependencies

- **US1 (P1)**: sem dependencia em outras historias; define o MVP.
- **US2 (P2)**: depende apenas da base; integra com estados de US1.
- **US3 (P3)**: depende da base e reutiliza persistencia de US1 para armazenar referencias privadas.
- **Operação local Zeto+Pente (Phase 7)**: depende de US3 e da infraestrutura Paladin local ativa por spoke.

### Parallel Opportunities

- Setup paralelo: `T003`, `T004`.
- Foundational paralelo: `T006`, `T010`, `T012`.
- US1 paralelo: `T014`, `T015`.
- US2 paralelo: `T024`, `T027`.
- US3 paralelo: `T031`, `T033`.
- Polish paralelo: `T040`, `T041`.
- Paladin local paralelo: `T044`, `T045`, `T048`.

---

## Parallel Example: User Story 1

```bash
# Executar em paralelo:
Task: T014 [US1] ListAgreements com filtros em backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go
Task: T015 [US1] CreateAuditEvent append-only em backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go
```

## Parallel Example: User Story 2

```bash
# Executar em paralelo:
Task: T024 [US2] relay-store persistente em interop/hub-and-spoke/cacti/src/relay-store.ts
Task: T027 [US2] suporte CANCELLED/SETTLED em interop/hub-and-spoke/cacti/src/htlc-relay.ts
```

## Parallel Example: User Story 3

```bash
# Executar em paralelo:
Task: T031 [US3] campos proto group_id/contract_address em apis/proto/payment_orchestrator/v1/payment_orchestrator.proto
Task: T033 [US3] porta Pente em backend/services/payment-orchestrator/internal/ports/pente.go
```

---

## Implementation Strategy

### MVP First (US1)

1. Concluir Phase 1 e Phase 2.
2. Entregar US1 completa (T013-T023).
3. Validar reinicio sem perda de dados e trilha de auditoria completa.

### Incremental Delivery

1. Base pronta (Phase 1 + 2).
2. Entregar US1 e validar.
3. Entregar US2 e validar resiliencia cross-spoke.
4. Entregar US3 e validar privacidade + enforcement HTLC.
5. Finalizar com Phase 6.

### Team Parallel Strategy

1. Time A: US1 (persistencia/auditoria).
2. Time B: US2 (relay confiavel).
3. Time C: US3 (Pente + HTLC), iniciando apos contratos/flags base estarem estaveis.

---

## Notes

- Todos os itens seguem formato checklist exigido: `- [ ] T### [P?] [US?] descricao com caminho de arquivo`.
- Tarefas marcadas com `[P]` evitam dependencia de tarefa incompleta e priorizam arquivos independentes.
- O escopo de MVP recomendado e US1 (persistencia + auditoria + expiracao).