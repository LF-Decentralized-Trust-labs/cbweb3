# Research: DB Migration — Spoke-Keyed Leg Fields

**Phase 0 output** | Branch: `021-db-migration-spoke-keyed`

---

## Decisão 1: Estratégia de idempotência — transação vs. IF EXISTS

**Decision**: Usar `db.Transaction()` envolvendo o UPDATE de backfill e os DROPs, combinado com verificação explícita de `HasColumn` antes de executar qualquer operação.

**Rationale**:
- A abordagem anterior (inferir estado a partir do erro do UPDATE) silencia erros reais e não recupera estados parciais.
- PostgreSQL suporta DDL transacional completo: `ALTER TABLE … DROP COLUMN` dentro de um `BEGIN/COMMIT` é seguro e atômico.
- SQLite (usado nos testes) também suporta DDL transacional desde versão 3.26 — disponível nas imagens de CI do projeto.
- A verificação explícita via `HasColumn` separa claramente "qual é o estado atual?" de "o que executar?", tornando a lógica legível e testável por caso.

**Alternatives considered**:
- `DROP COLUMN IF EXISTS` puro (sem `HasColumn`): funcionaria para o DROP, mas não resolveria o caso de backfill parcial em que uma coluna já foi removida — o UPDATE ainda referenciaria a coluna ausente e retornaria erro.
- Manter a lógica atual com apenas `IF EXISTS` no DROP: melhor que o estado atual, mas não trata o caso `hasA=false, hasB=true` (backfill parcial + primeira coluna já removida).

---

## Decisão 2: Lógica de backfill em estado parcial

**Decision**: Executar o UPDATE de backfill apenas quando AMBAS as colunas legadas estiverem presentes (`hasA && hasB`). Quando apenas uma remanescente, pular o backfill (dados já foram migrados em execução anterior) e apenas remover a coluna residual.

**Rationale**:
O código original dropa `spoke_a_receiver` antes de `spoke_b_receiver`. O único estado parcial possível é `hasA=false, hasB=true` — o que significa que o backfill já completou (ele ocorre antes de qualquer DROP) e `dest_receiver` está populado. Tentar re-executar o backfill nesse estado falha porque a coluna `spoke_a_receiver` não existe mais.

**Alternatives considered**:
- Backfill com SQL condicional (`COALESCE(spoke_b_receiver, '')` quando A ausente): mais complexo, coluna B pode já estar populada em `dest_receiver`, resultando em dupla sobrescrita desnecessária.

---

## Decisão 3: Logging de lifecycle da migração

**Decision**: Adicionar entradas de log estruturado via `log/slog` (stdlib Go 1.21+, disponível no Go 1.26 do projeto) para os eventos: início da migração, número de rows backfilladas, cada coluna removida, e conclusão.

**Rationale**:
A Constituição (Princípio VI) proíbe falhas silenciosas. A migração atual não loga nada — um operador que reinicia o serviço não tem como confirmar se a migração ocorreu ou não. O `log/slog` é stdlib e não requer dependência nova. A função `RunSpokeKeyedMigration` recebe `*gorm.DB` mas não um logger — passar `context.Context` ou um `*slog.Logger` como segundo parâmetro mantém a testabilidade sem acoplar ao logger global.

**Alternatives considered**:
- Logger global de pacote: cria estado compartilhado difícil de testar em paralelo.
- Adicionar logging ao caller (`NewGormFXAgreementRepository`): esconde a granularidade dos eventos de migração.

---

## Decisão 4: Compatibilidade SQLite nos testes

**Decision**: Usar `db.Migrator().HasColumn()` para a guarda pré-DROP (sem `IF EXISTS` no SQL raw), garantindo compatibilidade com SQLite e PostgreSQL.

**Rationale**:
Os testes do `payment-orchestrator` usam SQLite (`gorm.io/driver/sqlite`). Embora SQLite 3.35.0+ suporte `DROP COLUMN IF EXISTS`, não há controle sobre a versão exata instalada no CI. O método `db.Migrator().HasColumn()` do GORM é database-agnostic e resolve esse risco sem adicionar complexidade.

**Alternatives considered**:
- `DROP COLUMN IF EXISTS` no raw SQL: depende da versão do SQLite; risco de falha silenciosa em CI.
- Dois conjuntos de SQL (postgres vs. sqlite): complexidade injustificada para um caso que o GORM já abstrai.
