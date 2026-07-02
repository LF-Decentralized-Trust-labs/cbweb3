# Tasks: DB Migration — Spoke-Keyed Leg Fields

**Input**: Design documents em `specs/021-db-migration-spoke-keyed/`
**Branch**: `021-db-migration-spoke-keyed`
**Scope**: `scenario-a/backend/services/payment-orchestrator/`

**Organização**: Tarefas agrupadas por user story para permitir implementação e validação independente de cada cenário.

---

## Phase 1: Setup

**Purpose**: Estabelecer baseline antes de qualquer alteração.

- [X] T001 Rodar `go test ./internal/repository/... -v` em `scenario-a/backend/services/payment-orchestrator/` e registrar quais testes passam (baseline)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Não há novas dependências ou estrutura de projeto a criar — o código modificado está em arquivos existentes. Prosseguir diretamente para as user stories.

**Checkpoint**: Baseline documentado (T001) → user stories podem iniciar.

---

## Phase 3: User Story 1 — Fresh Deployment (Priority: P1) 🎯 MVP

**Goal**: Garantir que o serviço inicializa sem erro em banco vazio (sem colunas legadas), sem tentar executar migração desnecessária.

**Independent Test**: Criar banco via AutoMigrate apenas (sem colunas legadas), chamar `RunSpokeKeyedMigration`, verificar: sem erro, schema alvo correto, sem colunas `spoke_a_receiver`/`spoke_b_receiver`.

> **⚠️ CONSTITUTION V: Escrever o teste ANTES da implementação. O teste deve FALHAR com o código atual antes de prosseguir para T005.**

### Testes para User Story 1

- [X] T002 [US1] Adicionar `TestRunSpokeKeyedMigration_FreshSchema` em `scenario-a/backend/services/payment-orchestrator/internal/repository/gorm_repos_test.go` — banco criado via AutoMigrate sem colunas legadas, chamar `RunSpokeKeyedMigration`, esperar: sem erro e sem colunas legadas no schema

### Implementação para User Story 1

- [X] T003 [US1] Verificar que `TestRunSpokeKeyedMigration_FreshSchema` passa após o fix em T005–T007 (sem alteração adicional necessária para este cenário)

**Checkpoint**: Banco vazio → startup correto e schema alvo.

---

## Phase 4: User Story 2 — Migração de Deployment Existente (Priority: P1)

**Goal**: Banco com schema legado (`spoke_a_receiver`, `spoke_b_receiver`) é migrado corretamente no startup: backfill de todos os registros, colunas legadas removidas, dados intactos.

**Independent Test**: Seed de banco com schema e dados legados, chamar `RunSpokeKeyedMigration`, verificar: `source_spoke_id='spoke-a'`, `dest_spoke_id='spoke-b'`, receivers preservados, colunas legadas ausentes, zero perda de dados.

> **⚠️ CONSTITUTION V: Revisar cobertura de teste existente ANTES de implementar.**

### Testes para User Story 2

- [X] T004 [US2] Revisar `TestGormFXAgreementRepository_SpokeKeyedMigration_Idempotent` em `scenario-a/backend/services/payment-orchestrator/internal/repository/gorm_repos_test.go` — confirmar que cobre: backfill completo, DROP de ambas as colunas, e segunda execução sem erro (já existente); adicionar assertion de `result.RowsAffected > 0` se ausente

### Implementação para User Story 2

- [X] T005 [US2] Substituir guarda de estado em `RunSpokeKeyedMigration` em `scenario-a/backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go`: trocar inferência de erro do UPDATE por verificação explícita `hasA := db.Migrator().HasColumn(&FXAgreementModel{}, "spoke_a_receiver")` e `hasB := db.Migrator().HasColumn(...)` com early return quando `!hasA && !hasB`

- [X] T006 [US2] Envolver backfill + DROPs em `db.Transaction(func(tx *gorm.DB) error {...})` em `scenario-a/backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go` — eliminar possibilidade de estado parcial permanente; erros no UPDATE devem retornar `fmt.Errorf` (não mais silenciados)

- [X] T007 [US2] Implementar DROP condicional por coluna em `scenario-a/backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go`: executar backfill UPDATE apenas quando `hasA && hasB`; DROP `spoke_a_receiver` apenas se `hasA`; DROP `spoke_b_receiver` apenas se `hasB`; sem `IF EXISTS` no SQL (usar `HasColumn` como guarda — compatibilidade SQLite)

- [X] T008 [US2] Atualizar assinatura de `RunSpokeKeyedMigration` para aceitar `logger *slog.Logger` como segundo parâmetro em `scenario-a/backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go`

- [X] T009 [US2] Atualizar callers de `RunSpokeKeyedMigration` (`NewGormFXAgreementRepository` e `NewGormFXAgreementRepositoryFromDB`) em `scenario-a/backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go` para passar `slog.Default()` como logger

- [X] T010 [US2] Atualizar chamadas de `RunSpokeKeyedMigration` nos testes existentes em `scenario-a/backend/services/payment-orchestrator/internal/repository/gorm_repos_test.go` para passar `slog.New(slog.NewTextHandler(io.Discard, nil))` (logger descartado em testes)

**Checkpoint**: Banco legado com dados → migração completa, dados preservados, startup sem erro.

---

## Phase 5: User Story 3 — Migração Parcialmente Interrompida (Priority: P2)

**Goal**: Banco em estado parcial (`spoke_a_receiver` já removida, `spoke_b_receiver` ainda presente, dados previamente backfillados) é completado na próxima inicialização sem corrupção de dados e sem intervenção manual.

**Independent Test**: Simular estado parcial manualmente (backfill + DROP da primeira coluna), chamar `RunSpokeKeyedMigration`, verificar: sem erro, `spoke_b_receiver` removida, dados intactos.

> **⚠️ CONSTITUTION V: T011 DEVE ser escrito antes de T005–T007 (ou imediatamente após, verificando que FALHA com o código antigo antes do fix).**

### Testes para User Story 3

- [X] T011 [US3] Adicionar `TestRunSpokeKeyedMigration_PartialState` em `scenario-a/backend/services/payment-orchestrator/internal/repository/gorm_repos_test.go`:
  1. Criar tabela legacy com ambas as colunas
  2. Seed: `spoke_a_receiver='recv@a'`, `spoke_b_receiver='recv@b'`
  3. AutoMigrate (adiciona colunas novas)
  4. Simular backfill manual: UPDATE direto no banco
  5. DROP `spoke_a_receiver` diretamente (simular metade do DROP concluída)
  6. Chamar `RunSpokeKeyedMigration(db, logger)`
  7. Assertions: sem erro; `spoke_b_receiver` ausente; `source_receiver='recv@a'`, `dest_receiver='recv@b'`; row count = 1

### Implementação para User Story 3

- [X] T012 [US3] Verificar que o fix implementado em T005–T007 faz `TestRunSpokeKeyedMigration_PartialState` passar sem alteração adicional (o DROP condicional por `hasB` cobre exatamente este cenário)

**Checkpoint**: Estado parcial → migração completada no restart, sem perda de dados.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Observabilidade (Constituição Princípio VI) e validação end-to-end.

- [X] T013 [P] Adicionar logging estruturado via `slog` em `RunSpokeKeyedMigration` em `scenario-a/backend/services/payment-orchestrator/internal/repository/fx_agreement_gorm.go`: `logger.Info("spoke-keyed migration starting", ...)` antes da transação; `logger.Info("backfill complete", "rows_affected", n)` após UPDATE; `logger.Info("dropped column", "column", col)` após cada DROP; `logger.Info("spoke-keyed migration complete")` ao final

- [X] T014 Rodar suite completa: `go test ./... -v` em `scenario-a/backend/services/payment-orchestrator/` — todos os testes devem passar incluindo T002, T004, T011

- [X] T015 [P] Rodar `make scenario-a.test-backend-coverage` para validação da suite integrada (target `scenario-a.test-backend` não existe; `scenario-a.test-backend-coverage` usado como equivalente)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sem dependências — iniciar imediatamente
- **US1 (Phase 3)**: T002 pode ser escrito antes da implementação; T003 requer T005–T007
- **US2 (Phase 4)**: T004 (revisão de teste) antes de T005; T005→T006→T007 em sequência; T008→T009→T010 após T005
- **US3 (Phase 5)**: T011 deve ser escrito e verificado como **failing** antes de T005–T007; T012 requer T005–T007
- **Polish (Phase 6)**: T013 pode ser feito com T008; T014 requer todas as fases anteriores

### Dependências Críticas por Tarefa

| Tarefa | Depende de | Notas |
|--------|------------|-------|
| T003   | T005–T007  | US1 passa automaticamente após o fix |
| T005   | T002, T004 | Core fix — linha central da implementação |
| T006   | T005       | Encapsular na transação após reescrever a guarda |
| T007   | T005, T006 | DROP condicional; fecha a lógica da transação |
| T008   | T007       | Assinatura só muda após a lógica estar correta |
| T009   | T008       | Callers dependem da nova assinatura |
| T010   | T008       | Testes dependem da nova assinatura |
| T012   | T005–T007  | Verificação de que o fix cobre US3 |
| T013   | T008       | Logging adicionado junto com a nova assinatura |
| T014   | T002–T013  | Suite completa — validação final |

### Oportunidades de Paralelismo

- T002 e T011 podem ser escritos em paralelo (arquivos diferentes? não, mesmo arquivo, mas seções diferentes — atenção a conflitos)
- T013 pode ser desenvolvido em paralelo com T010 (mesmo arquivo, função diferente)
- T015 pode ser disparado em paralelo com T014

---

## Parallel Example: User Story 2 (implementação)

```bash
# T005, T006, T007 devem ser feitos em sequência (mesma função)
# T008 pode ser iniciado após T005 ser aprovado em review

# Rodar testes de regressão após cada tarefa de implementação:
go test ./internal/repository/... -run TestGormFXAgreementRepository -v
```

---

## Implementation Strategy

### MVP First (User Story 1 + 2)

1. T001: Baseline
2. T002: Teste US1 (fresh schema)
3. T004: Revisão cobertura US2
4. T011: Teste US3 (partial state) — VERIFICAR QUE FALHA
5. T005 → T006 → T007: Fix `RunSpokeKeyedMigration`
6. T008 → T009 → T010: Atualizar assinatura e callers
7. **STOP e VALIDAR**: `go test ./internal/repository/...` — todos devem passar
8. T013: Adicionar logging
9. T014 + T015: Validação final

### Ordem Recomendada para Desenvolvedor Único

```
T001 → T002 → T004 → T011 (verificar FAIL) →
T005 → T006 → T007 → T008 → T009 → T010 →
T003 → T012 (verificar PASS) →
T013 → T014 → T015
```

---

## Notes

- `[P]` = tarefas que podem rodar em paralelo (arquivos ou seções diferentes)
- `[USn]` = user story à qual a tarefa pertence
- Cada user story é individualmente testável via `go test -run <TestName>`
- **Commit após cada fase** — facilita bisect em caso de regressão
- Arquivos modificados: `fx_agreement_gorm.go` e `gorm_repos_test.go` — apenas `scenario-a/`
