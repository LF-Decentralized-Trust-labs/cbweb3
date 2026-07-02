# Implementation Plan: DB Migration — Spoke-Keyed Leg Fields

**Branch**: `021-db-migration-spoke-keyed` | **Date**: 2026-06-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/021-db-migration-spoke-keyed/spec.md`

## Summary

Corrigir e completar a migração de banco de dados do `payment-orchestrator` (Scenario A) que adiciona campos spoke-keyed (`source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`) à tabela `fx_agreements`, executa backfill dos dados legados posicionais, e remove as colunas antigas. A implementação atual (`RunSpokeKeyedMigration`) tem um bug de estado parcial: se o processo for interrompido após o primeiro DROP e antes do segundo, execuções subsequentes silenciam o erro e deixam a segunda coluna legada permanentemente. A correção usa verificação explícita de `HasColumn`, transação GORM para atomicidade, e adiciona logging estruturado de lifecycle.

---

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: GORM v2 (`gorm.io/gorm`, `gorm.io/driver/postgres`), `gorm.io/driver/sqlite` (testes), `log/slog` (stdlib)
**Storage**: PostgreSQL (produção), SQLite (testes de repositório)
**Testing**: `go test` — testes de repositório com SQLite in-process
**Target Platform**: Linux (container Docker, `scenario-a/`)
**Project Type**: Microserviço backend (payment-orchestrator)
**Performance Goals**: Migração conclui durante o startup do serviço; para volumes piloto (< 10k registros) o impacto é imperceptível
**Constraints**: Idempotente; não deve bloquear o startup; deve recuperar estados parciais sem intervenção manual
**Scale/Scope**: Scenario A apenas — `scenario-a/backend/services/payment-orchestrator/`

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Princípio | Status | Notas |
|-----------|--------|-------|
| **I. Scenario-Scoped Independence** | ✅ PASS | Todas as alterações em `scenario-a/`. Nenhum arquivo de `scenario-b/` é tocado. |
| **II. Privacy by Design** | ✅ PASS | Campos de receiver são identidades Paladin (strings internas), não valores transferidos on-chain. Nenhuma mudança no modelo de privacidade. |
| **III. Atomic Settlement Guarantee** | ✅ PASS | Migração é de dados históricos de infra. Não altera lógica de liquidação HTLC nem caminhos de timeout/refund. |
| **IV. Compliance Gate Before Participation** | ✅ PASS | Nenhuma alteração em rotas de auth, compliance ou IdentityRegistry. |
| **V. Test-First at Every Layer** | ⚠️ AÇÃO NECESSÁRIA | O teste de estado parcial (`hasA=false, hasB=true`) ainda não existe e DEVE ser escrito antes da correção do código. O teste existente (`TestGormFXAgreementRepository_SpokeKeyedMigration_Idempotent`) cobre os casos "fresh" e "já migrado", mas não o estado parcial. |
| **VI. Observability and Auditability** | ⚠️ AÇÃO NECESSÁRIA | `RunSpokeKeyedMigration` não emite nenhum log. Um operador que reinicia o serviço não tem visibilidade sobre se a migração ocorreu. Logging estruturado de lifecycle é obrigatório. |

**Complexity Tracking** (apenas para violações com justificativa):

Nenhuma violação de arquitetura. Ambos os itens ⚠️ são lacunas de implementação, não violações de design.

---

## Project Structure

### Documentation (this feature)

```text
specs/021-db-migration-spoke-keyed/
├── plan.md              # Este arquivo
├── spec.md              # Especificação de feature
├── research.md          # Decisões de Phase 0
├── data-model.md        # Schema antes/depois + regras de backfill
├── checklists/
│   └── requirements.md  # Checklist de qualidade da spec
└── tasks.md             # Gerado por /speckit.tasks (próximo passo)
```

### Source Code (arquivos afetados)

```text
scenario-a/backend/services/payment-orchestrator/
└── internal/
    └── repository/
        ├── fx_agreement_gorm.go      # Correção de RunSpokeKeyedMigration
        └── gorm_repos_test.go        # Novo teste: estado parcial de migração
```

Nenhum arquivo fora de `scenario-a/` é modificado.

---

## Phase 0: Research — Decisões

Ver [research.md](research.md) para raciocínio completo.

| Decisão | Escolha |
|---------|---------|
| Estratégia de idempotência | `HasColumn` guard + `db.Transaction()` + DROP sem `IF EXISTS` |
| Backfill em estado parcial | Executar UPDATE só quando ambas as colunas legadas presentes |
| Logging | `log/slog` (stdlib) — parâmetro `logger *slog.Logger` na função |
| Compatibilidade SQLite | Usar `HasColumn` antes do DROP em vez de `IF EXISTS` no SQL raw |

---

## Phase 1: Design

### 1.1 Contrato da função `RunSpokeKeyedMigration` (revisado)

```
Assinatura: RunSpokeKeyedMigration(db *gorm.DB, logger *slog.Logger) error

Pré-condições:
  - db está aberto e conectado
  - AutoMigrate já foi executado (novas colunas existem no schema)

Pós-condições (sucesso):
  - Colunas spoke_a_receiver e spoke_b_receiver ausentes da tabela
  - Todos os registros com source_spoke_id == "" têm os campos spoke-keyed preenchidos
  - Eventos de log emitidos para cada etapa executada

Idempotência:
  - Se nenhuma coluna legada existe → return nil (sem efeito, sem log de migração)
  - Se estado parcial (apenas uma coluna legada) → completar o DROP da remanescente

Erro:
  - Qualquer falha de DB retorna fmt.Errorf wrapped — nunca swallowed
  - Em caso de erro, a transação é revertida automaticamente
```

### 1.2 Lógica revisada (pseudocódigo)

```
RunSpokeKeyedMigration(db, logger):
  se tabela não existe → return nil

  hasA = HasColumn("spoke_a_receiver")
  hasB = HasColumn("spoke_b_receiver")

  se !hasA && !hasB:
    → return nil  (schema alvo; no-op)

  log.Info("spoke-keyed migration starting", "hasA", hasA, "hasB", hasB)

  return db.Transaction(func(tx):
    se hasA && hasB:
      result = tx.Exec(UPDATE backfill WHERE source_spoke_id IS NULL OR = '')
      se erro → return wrapped error
      log.Info("backfill complete", "rows_affected", result.RowsAffected)

    se hasA:
      tx.Exec("ALTER TABLE fx_agreements DROP COLUMN spoke_a_receiver")
      se erro → return wrapped error
      log.Info("dropped column spoke_a_receiver")

    se hasB:
      tx.Exec("ALTER TABLE fx_agreements DROP COLUMN spoke_b_receiver")
      se erro → return wrapped error
      log.Info("dropped column spoke_b_receiver")

    log.Info("spoke-keyed migration complete")
    return nil
  )
```

### 1.3 Assinatura do caller (impacto em cascata)

`NewGormFXAgreementRepository` e `NewGormFXAgreementRepositoryFromDB` devem passar um `*slog.Logger` para `RunSpokeKeyedMigration`. O logger pode ser obtido via `slog.Default()` no caller ou injetado como parâmetro do construtor — a ser decidido na implementação sem afetar o contrato externo.

### 1.4 Novos testes necessários (test-first)

**Teste: `TestRunSpokeKeyedMigration_PartialState`**

Cenário: banco com `spoke_a_receiver` já removida, `spoke_b_receiver` ainda presente, e dados previamente backfillados.

```
Setup:
  1. Criar tabela legacy com ambas as colunas
  2. Seed de um row (spoke_a_receiver='recv@a', spoke_b_receiver='recv@b')
  3. AutoMigrate (adiciona novas colunas)
  4. Simular backfill manual: UPDATE SET source_spoke_id='spoke-a', dest_spoke_id='spoke-b', ...
  5. DROP spoke_a_receiver (simula primeira metade do DROP concluída)

Exercício:
  6. Chamar RunSpokeKeyedMigration(db, logger)

Assertions:
  7. Nenhum erro retornado
  8. spoke_b_receiver ausente do schema
  9. spoke_a_receiver ausente do schema (já estava)
  10. Dados do row intactos: source_receiver='recv@a', dest_receiver='recv@b'
  11. Row count = 1 (nenhum dado duplicado ou perdido)
```

---

## Artefatos gerados

| Arquivo | Status |
|---------|--------|
| [spec.md](spec.md) | ✅ |
| [research.md](research.md) | ✅ |
| [data-model.md](data-model.md) | ✅ |
| tasks.md | ⏳ próximo: `/speckit.tasks` |
