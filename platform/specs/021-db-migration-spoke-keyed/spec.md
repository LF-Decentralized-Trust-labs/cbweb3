# Feature Specification: DB Migration — Spoke-Keyed Leg Fields

**Feature Branch**: `021-db-migration-spoke-keyed`
**Created**: 2026-06-27
**Status**: Draft
**Input**: MD-2 — ALTERAR: migration de banco de dados (spoke-keyed leg fields)

## Context

O modelo de dados de FX agreements hoje identifica as pernas de um acordo por **posição** (`spoke_a_receiver`, `spoke_b_receiver`), assumindo exatamente dois spokes fixos (A e B). Para suportar N spokes com identidades arbitrárias, cada perna precisa ser identificada pelo **ID do spoke** ao qual pertence.

Esta spec cobre a migração do banco de dados que viabiliza esse modelo: adicionar colunas spoke-keyed, traduzir os dados legados via backfill, e remover as colunas posicionais.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Fresh Deployment (Priority: P1)

Um operador sobe o serviço de payment-orchestrator pela primeira vez, com banco vazio. O serviço deve inicializar corretamente com o novo schema sem erros, sem tentar migrar dados que não existem.

**Why this priority**: É o caminho mais comum em novos ambientes. Qualquer falha aqui impede o onboarding de novos spokes.

**Independent Test**: Subir o serviço com banco vazio e confirmar que a tabela `fx_agreements` é criada com as colunas spoke-keyed e SEM as colunas posicionais legadas.

**Acceptance Scenarios**:

1. **Given** um banco de dados vazio, **When** o serviço inicializa, **Then** a tabela `fx_agreements` contém `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` e NÃO contém `spoke_a_receiver` nem `spoke_b_receiver`
2. **Given** um banco de dados vazio, **When** o serviço inicializa, **Then** nenhum erro de migração é reportado nos logs
3. **Given** um banco de dados vazio já com schema novo, **When** o serviço reinicia, **Then** o startup completa sem erros (idempotência)

---

### User Story 2 — Migração de Deployment Existente (Priority: P1)

Um operador reinicia o serviço em um ambiente que já possui dados de FX agreements gravados com o schema posicional legado (`spoke_a_receiver`, `spoke_b_receiver`). Todos os dados existentes devem ser traduzidos para o novo modelo sem perda, e as colunas antigas devem ser removidas.

**Why this priority**: Garante continuidade de dados para ambientes já em uso. Sem isso, dados históricos ficam inacessíveis ou inconsistentes.

**Independent Test**: Seed de banco com registros usando `spoke_a_receiver`/`spoke_b_receiver`, reiniciar o serviço, e verificar que cada registro agora possui `source_spoke_id = 'spoke-a'`, `dest_spoke_id = 'spoke-b'`, e os receivers preservados nas novas colunas.

**Acceptance Scenarios**:

1. **Given** registros com `spoke_a_receiver = 'recv@bank-a'` e `spoke_b_receiver = 'recv@bank-b'`, **When** o serviço inicializa, **Then** esses registros possuem `source_spoke_id = 'spoke-a'`, `dest_spoke_id = 'spoke-b'`, `source_receiver = 'recv@bank-a'`, `dest_receiver = 'recv@bank-b'`
2. **Given** um registro com `spoke_a_receiver = NULL`, **When** o backfill é executado, **Then** `source_receiver` é mapeado para string vazia (sem erro)
3. **Given** migração concluída, **When** o schema é inspecionado, **Then** as colunas `spoke_a_receiver` e `spoke_b_receiver` não existem mais na tabela `fx_agreements`
4. **Given** migration concluída, **When** o serviço reinicia novamente, **Then** o startup completa sem erros (idempotência pós-migração)

---

### User Story 3 — Migração Parcialmente Interrompida (Priority: P2)

O processo é interrompido durante a migração — por exemplo, após o backfill ter sucesso e a remoção da primeira coluna legada ter ocorrido, mas antes da remoção da segunda. Na reinicialização, a migração deve completar o trabalho pendente sem corromper dados.

**Why this priority**: Garante resiliência operacional. Sem isso, uma falha de infraestrutura durante a migração pode deixar o banco em estado inconsistente permanente.

**Independent Test**: Simular estado parcial (um DROP feito, outro não), reiniciar o serviço, e verificar que a coluna remanescente é removida e os dados permanecem corretos.

**Acceptance Scenarios**:

1. **Given** o banco está com `spoke_a_receiver` já removida mas `spoke_b_receiver` ainda presente, **When** o serviço inicializa, **Then** a migração detecta o estado parcial e conclui removendo `spoke_b_receiver`
2. **Given** estado parcial com dados já backfillados, **When** a migração completa, **Then** nenhum dado é duplicado ou perdido

---

### Edge Cases

- O que acontece quando a tabela `fx_agreements` não existe ainda (novo banco)? A migração deve ser pulada sem erro.
- O que acontece quando ambas as colunas legadas já foram removidas (schema novo)? A migração deve ser no-op.
- O que acontece quando há zero registros na tabela durante a migração? O backfill deve executar sem erro (zero linhas afetadas é válido).
- O que acontece quando apenas um dos dois campos legados tem valor NULL? O mapeamento deve preencher com string vazia sem falhar.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O schema da tabela `fx_agreements` DEVE conter as colunas `source_spoke_id`, `dest_spoke_id`, `source_receiver` e `dest_receiver` após qualquer inicialização do serviço
- **FR-002**: O processo de migração DEVE traduzir `spoke_a_receiver` para `source_receiver` e atribuir `source_spoke_id = 'spoke-a'` para todos os registros legados
- **FR-003**: O processo de migração DEVE traduzir `spoke_b_receiver` para `dest_receiver` e atribuir `dest_spoke_id = 'spoke-b'` para todos os registros legados
- **FR-004**: O processo de migração DEVE remover as colunas `spoke_a_receiver` e `spoke_b_receiver` da tabela `fx_agreements` somente após o backfill ter sido concluído com sucesso
- **FR-005**: A migração DEVE ser idempotente — executar múltiplas vezes no mesmo banco não deve causar erro nem alterar dados já migrados
- **FR-006**: A migração DEVE detectar e completar estados parciais (ex.: uma coluna legada já removida, a outra ainda presente)
- **FR-007**: Em banco vazio ou com schema já atualizado, a migração DEVE concluir como no-op sem erros
- **FR-008**: Valores NULL nas colunas legadas DEVEM ser mapeados para string vazia nas colunas novas, sem causar falha na migração
- **FR-009**: Após a migração, qualquer novo registro de FX agreement DEVE obrigatoriamente fornecer `source_spoke_id`, `dest_spoke_id`, `source_receiver` e `dest_receiver`

### Key Entities

- **FXAgreement**: Registro de um acordo de câmbio entre dois spokes. Após a migração, cada perna é identificada pelo ID do spoke (ex.: `'spoke-a'`, `'spoke-brl'`) e pelo identificador do receptor naquele spoke — não mais por posição (A/B)
- **Leg de liquidação**: Par (spoke_id, receiver) que representa um dos lados do trade. Substituí o conceito posicional `spoke_a_receiver`/`spoke_b_receiver`

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% dos registros legados possuem `source_spoke_id` e `dest_spoke_id` preenchidos após a migração — zero registros com valores NULL ou vazios nessas colunas em bancos que tinham dados legados
- **SC-002**: Zero perda de dados — cada valor de receiver legado é preservado integralmente na coluna correspondente do novo modelo
- **SC-003**: O serviço inicia com sucesso (sem erros fatais) nos três cenários: banco vazio, banco legado, banco já migrado
- **SC-004**: A migração completa durante o startup do serviço, sem impacto observável no tempo de inicialização para volumes de dados típicos de piloto (< 10.000 registros)
- **SC-005**: Após migração bem-sucedida, as colunas `spoke_a_receiver` e `spoke_b_receiver` são completamente ausentes do schema da tabela
- **SC-006**: A migração é resiliente a interrupções — um estado parcial causado por falha durante o processo é corrigido na próxima inicialização sem intervenção manual

---

## Assumptions

- O backfill utiliza `source_spoke_id = 'spoke-a'` e `dest_spoke_id = 'spoke-b'` como valores fixos para todos os registros legados, pois o modelo anterior assumia exatamente esses dois spokes
- A migração é executada durante o startup do serviço, não como script avulso — isso é intencional e aceito para o volume de dados do ambiente piloto
- A tabela `fx_agreement_events` não armazena campos de receiver e portanto não requer migração de dados; apenas o schema da `fx_agreements` é afetado
- O ambiente usa PostgreSQL; comportamentos de DDL (ex.: `ALTER TABLE ... DROP COLUMN IF EXISTS`) são compatíveis com a versão em uso
- A migração ocorre dentro do Scenario A (Enhanced Correspondent Banking) apenas; o Scenario B mantém schema independente por isolamento de cenários
- Não há mecanismo de rollback automático — se a migração falhar, o banco deve ser restaurado a partir de backup antes de nova tentativa
