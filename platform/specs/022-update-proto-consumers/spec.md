# Feature Specification: MD-3 — Atualizar Consumidores do Proto para Campos Spoke-Keyed

**Feature Branch**: `022-update-proto-consumers`
**Created**: 2026-06-27
**Status**: Draft
**Scope**: Scenario A (Enhanced Correspondent Banking) only

---

> **Nota sobre Scenario B**: O Scenario B (`scenario-b/`) ainda utiliza os campos posicionais `spoke_a_receiver`/`spoke_b_receiver` em seu domain model, GORM model, gRPC server e frontend. Essa spec **não cobre** o Scenario B. Qualquer atualização no Scenario B deve ser uma PR separada com justificativa explícita, respeitando a regra de isolamento de cenários (constitution §isolation). O Scenario B não deve ser tocado neste trabalho.

---

## Contexto

O MD-1 (atualização do arquivo `.proto`) já foi aplicado: os campos posicionais `spoke_a_receiver`/`spoke_b_receiver` foram substituídos por `source_spoke_id`, `dest_spoke_id`, `source_receiver` e `dest_receiver` nas mensagens `FXAgreement` e `ProposeFXAgreementRequest`.

O objetivo deste item (MD-3) é propagar essa mudança para todos os consumidores do proto no Scenario A, garantindo que o sistema leia, escreva e roteie acordos FX pelos novos campos spoke-keyed em todos os pontos de integração.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Proposta de FX Agreement com campos spoke-keyed no backend (Priority: P1)

Um engenheiro integrador chama o endpoint de proposta de FX Agreement no `payment-orchestrator` informando o spoke de origem (`source_spoke_id`), o spoke de destino (`dest_spoke_id`) e os receptores correspondentes. O serviço persiste e retorna os dados corretamente pelos novos campos.

**Why this priority**: É o ponto de entrada de dados no sistema. Se o backend não ler e gravar os novos campos corretamente, nenhum outro componente funciona — o relay não recebe spoke ids para rotear e o frontend não exibe informações corretas.

**Independent Test**: Pode ser testado isoladamente chamando o gRPC `ProposeFXAgreement` com `source_spoke_id`/`dest_spoke_id`/`source_receiver`/`dest_receiver` preenchidos e verificando que a resposta e o registro no banco refletem os mesmos valores. Entrega valor imediato pois valida que o backend aceita o novo contrato de dados.

**Acceptance Scenarios**:

1. **Given** o serviço `payment-orchestrator` do Scenario A executando com o proto regenerado, **When** um cliente envia uma proposta de FX Agreement com `source_spoke_id = "spoke-a"`, `dest_spoke_id = "spoke-b"`, `source_receiver = "<identidade-paladin-a>"` e `dest_receiver = "<identidade-paladin-b>"`, **Then** o serviço aceita a requisição sem erro, persiste os quatro campos no banco e os retorna na resposta.

2. **Given** um FX Agreement existente criado com o schema antigo (`spoke_a_receiver`/`spoke_b_receiver`), **When** a migração de banco (MD-2) já foi executada e os registros foram backfillados, **Then** o `payment-orchestrator` retorna esses registros com os campos novos preenchidos corretamente (sem campos posicionais antigos).

3. **Given** uma requisição de proposta com `dest_spoke_id` em branco ou ausente, **When** o serviço valida a entrada, **Then** retorna erro descritivo indicando campo obrigatório ausente.

---

### User Story 2 - Relay roteia FX Agreement pelo dest_spoke_id (Priority: P1)

O relay Cacti (`htlc-relay.ts`) recebe um evento de FX Agreement do Scenario A, lê `dest_spoke_id` do payload e roteia o lock HTLC para o spoke de destino correto, sem assumir posição fixa de contraparte.

**Why this priority**: Igual em prioridade ao backend — sem o roteamento correto pelo relay, a liquidação bilateral entre dois spokes provisionados independentemente não funciona. É o bloqueador do "caminho rápido" (Phase 2 step 8 do plano).

**Independent Test**: Pode ser testado isoladamente levantando dois spokes locais e verificando que o relay, ao receber um FX Agreement com `dest_spoke_id` populado, executa o lock HTLC no spoke correto. O relay já foi atualizado para ler os novos campos da API REST; este teste valida o pipeline completo ponta-a-ponta.

**Acceptance Scenarios**:

1. **Given** dois spokes Scenario A rodando (`spoke-a` e `spoke-b`) com o relay Cacti ativo, **When** um FX Agreement é proposto com `source_spoke_id = "spoke-a"` e `dest_spoke_id = "spoke-b"`, **Then** o relay executa o lock HTLC no spoke `spoke-b` (não em `spoke-a`) e registra no log o `dest_spoke_id` utilizado para roteamento.

2. **Given** o relay recebendo um acordo via polling REST, **When** o backend retorna os campos `source_spoke_id`/`dest_spoke_id`/`source_receiver`/`dest_receiver`, **Then** o relay os mapeia corretamente para os campos internos `sourceSpokeId`/`destSpokeId`/`sourceReceiver`/`destReceiver` sem perda de dados.

3. **Given** um FX Agreement com `dest_spoke_id` diferente de qualquer spoke registrado na config do relay, **When** o relay tenta resolver o destino, **Then** loga erro descritivo indicando spoke desconhecido e não executa nenhuma transação.

---

### User Story 3 - Frontend Scenario A exibe informações spoke-keyed ao usuário (Priority: P2)

Um usuário do app de banco do Scenario A visualiza e preenche acordos FX usando os campos de spoke de origem/destino em vez dos campos posicionais. O formulário de proposta e a listagem de acordos refletem os novos campos.

**Why this priority**: Dependente do backend (US1) estar completo. O frontend do Scenario A já tem os tipos TypeScript atualizados; o trabalho restante é verificar que nenhum formulário ou componente ainda envia os campos antigos.

**Independent Test**: Pode ser testado abrindo o app `bank` do Scenario A, propondo um FX Agreement pelo formulário e verificando na aba Network do browser que o payload enviado usa `source_spoke_id`/`dest_spoke_id` e não os campos antigos.

**Acceptance Scenarios**:

1. **Given** o app `bank` do Scenario A rodando, **When** um usuário preenche e submete o formulário de proposta de FX Agreement, **Then** o payload enviado à API contém `source_spoke_id`, `dest_spoke_id`, `source_receiver` e `dest_receiver` — sem `spoke_a_receiver` ou `spoke_b_receiver`.

2. **Given** um FX Agreement existente (pós-migração) listado no frontend, **When** o usuário visualiza os detalhes do acordo, **Then** os campos exibidos correspondem a `source_spoke_id`/`dest_spoke_id` e não a labels posicionais "Spoke A"/"Spoke B" hardcoded.

---

### Edge Cases

- O que acontece se o código Go regenerado do proto apresentar divergência de versão com o vendor do `api-gateway`? O build deve falhar com erro claro, não silenciosamente.
- Como o relay se comporta se o backend ainda retornar os campos antigos (downgrade temporário)? Deve logar warning indicando campos inesperados, não travar silenciosamente.
- O que acontece se `make proto-gen` for executado sem o MD-1 estar no branch correto? O build gera código com os campos antigos — prevenido pela pré-condição documentada.
- Como validar que nenhum consumidor referencia os campos antigos após a mudança? A build Go deve falhar (campos removidos no proto), e o TypeScript deve falhar no type-check.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O código Go gerado pelo proto no Scenario A DEVE ser regenerado a partir da definição atualizada do MD-1, removendo os getters `SpokeAReceiver()`/`SpokeBReceiver()` e expondo `SourceSpokeId()`, `DestSpokeId()`, `SourceReceiver()`, `DestReceiver()`.

- **FR-002**: O handler gRPC `ProposeFXAgreement` no `payment-orchestrator` (Scenario A) DEVE ler os campos `source_spoke_id`, `dest_spoke_id`, `source_receiver` e `dest_receiver` da requisição proto e persistir no domain model — sem referência aos campos posicionais antigos.

- **FR-003**: A resposta gRPC de consulta de FX Agreements no `payment-orchestrator` (Scenario A) DEVE converter o domain model para proto usando os novos campos spoke-keyed.

- **FR-004**: O relay Cacti (`htlc-relay.ts`) DEVE rotear cada FX Agreement pelo `dest_spoke_id` lido do payload REST — a função de resolução de contraparte NÃO DEVE inferir destino por eliminação posicional.

- **FR-005**: Nenhum arquivo de código-fonte do Scenario A (exceto comentários de migração e arquivos de teste de cenários de upgrade) DEVE referenciar `spoke_a_receiver` ou `spoke_b_receiver` após esta implementação.

- **FR-006**: O frontend do Scenario A (`apps/bank`) NÃO DEVE enviar `spoke_a_receiver` ou `spoke_b_receiver` em nenhum payload de criação ou atualização de FX Agreement.

- **FR-007**: A build do Scenario A (Go + TypeScript) DEVE passar sem erros após a mudança, com os novos campos sendo a única interface exposta.

- **FR-008**: Os testes unitários existentes do `payment-orchestrator` DEVEM ser atualizados para usar os novos campos; nenhum teste pode referenciar os campos posicionais antigos.

### Key Entities

- **FXAgreement**: Acordo de câmbio entre dois spokes. Após MD-3, identificado por `source_spoke_id` (spoke de origem da transação), `dest_spoke_id` (spoke de destino), `source_receiver` (identidade Paladin no spoke de origem) e `dest_receiver` (identidade Paladin no spoke de destino). Campos posicionais removidos.

- **ProposeFXAgreementRequest**: Mensagem de entrada para proposta de FX Agreement. Mesma mudança: campos spoke-keyed substituem os posicionais.

- **Spoke**: Rede Besu independente gerida por um banco central. Identificado por `spoke_id` (string, ex: `"spoke-a"`). O roteamento do relay usa este identificador.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A build completa do Scenario A (contratos, backend, frontend) passa sem erros após a regeneração do proto e atualização dos consumidores — zero falhas de compilação relacionadas a campos do FXAgreement.

- **SC-002**: A suite de testes do `payment-orchestrator` passa integralmente (`go test ./...`) sem referências a `spoke_a_receiver` ou `spoke_b_receiver` no código de teste.

- **SC-003**: Um FX Agreement proposto end-to-end (frontend → api-gateway → payment-orchestrator → relay → spoke de destino) completa a liquidação corretamente entre dois spokes do Scenario A, com o relay roteando pelo `dest_spoke_id`.

- **SC-004**: Busca de texto por `spoke_a_receiver` e `spoke_b_receiver` no código-fonte do Scenario A (excluindo arquivos de migração e histórico git) retorna zero ocorrências.

- **SC-005**: O type-check do TypeScript no frontend do Scenario A (`apps/bank`) passa sem erros relacionados aos tipos de FXAgreement.

---

## Assumptions

- O MD-1 (definição `.proto` atualizada com `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`) já está aplicado e disponível no branch atual.
- O MD-2 (migração de banco e backfill) já está implementado — os dados existentes já foram migrados para os novos campos; o backend não precisa lidar com o schema antigo em runtime.
- O relay Cacti (`htlc-relay.ts`) já tem as interfaces TypeScript internas atualizadas para os novos campos camelCase (`sourceSpokeId`, `destSpokeId`) — o trabalho restante é garantir que o roteamento usa esses campos corretamente.
- O frontend do Scenario A (`apps/bank`) já tem os tipos TypeScript atualizados (`fx-agreement.types.ts`) — o trabalho restante é verificar formulários e chamadas de API.
- O Scenario B está fora do escopo desta spec e não será modificado.
- `make proto-gen` é o comando canônico para regenerar código Go a partir do `.proto` atualizado no Scenario A.
- Testes de integração devem usar SQLite (conforme padrão do projeto para testes de repositório).
