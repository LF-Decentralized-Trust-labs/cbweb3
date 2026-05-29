# Feature Specification: Harden FX Agreement for Production

**Feature Branch**: `feature/agreement-v2`  
**Created**: 2026-04-14  
**Status**: Draft (updated 2026-04-15 for local Paladin Zeto+Pente readiness)  
**Input**: User description: "preciso adicionar, concluir, alterar o FX-Agreement para produção com persistência, relay confiável, integração Pente, integração HTLC, expiração automática e auditoria"

---

## Architecture Overview *(mandatory)*

### Deployment Model: Hybrid Besu/Pente

FX Agreement lifecycle is deployed across two blockchain environments for optimal privacy and enforcement:

```
┌─────────────────────────────────────────────────────────────┐
│ Besu (Public Blockchain)                                    │
│                                                             │
│  ┌──────────────────┐  ┌────────────────────────────────┐  │
│  │ IdentityRegistry │  │ CommitmentHashRegistry         │  │
│  │ - canTransact()  │  │ - registerCommitment()         │  │
│  │ - canGovern()    │  │ - acceptCommitment()           │  │
│  │                  │  │ - isAccepted() [HTLC gate]     │  │
│  └──────────────────┘  └────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ HashTimeLockedContract                               │  │
│  │ - lock() with dual-gate enforcement:                 │  │
│  │   Gate 1: FXAgreement.accept() (if Pente available)  │  │
│  │   Gate 2: CommitmentHashRegistry.isAccepted() (fback)│  │
│  │ - settle(secret)                                     │  │
│  │ - refund()                                           │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Pente (Private Network) - Per Bilateral Context             │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ FXAgreement Contract (Private)                       │  │
│  │ - One per bilateral pair (GroupID)                   │  │
│  │ - propose() / proposeOnBehalf()                      │  │
│  │ - accept() / acceptOnBehalf()                        │  │
│  │ - reject() / cancel() / settle()                     │  │
│  │ - Only parties and designated governance can access  │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ PostgreSQL (Service-Layer State)                            │
│                                                             │
│  ┌────────────────┐  ┌────────────────┐                    │
│  │ fx_agreements  │  │ fx_agreement_  │                    │
│  │ - state        │  │    events      │                    │
│  │ - GroupID      │  │ - ctor, state  │                    │
│  │ - ContractAddr │  │   transition   │                    │
│  └────────────────┘  └────────────────┘                    │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ relay_delivery_records (cross-spoke sync)            │  │
│  │ - idempotency keys, retry tracking                   │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

1. **Private Pente Contracts**: FXAgreement deployed in private bilateral contexts (one per counterparty pair) eliminates on-chain exposure of confidential terms
2. **CommitmentHashRegistry Fallback**: Public registry stores `keccak256(tradeId || amounts || rate)` hashes for HTLC gating when Pente contract unavailable
3. **Defense-in-Depth Enforcement**: HTLC.lock() enforced at 3 levels:
   - Service-layer agreement validation (PRIMARY)
   - On-chain FXAgreement state check from Pente (if available)
   - CommitmentHashRegistry fallback gate (if Pente unavailable)
4. **Durable State + Relay**: PostgreSQL persistence + cross-spoke relay with exponential backoff retry and idempotency guarantees
5. **Phase Architecture**: Phase A (current) uses service + commitment hash; Phase B (future) enables atomic Pente externalCalls

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Persistir e auditar acordos FX (Priority: P1)

Como equipe operacional e de compliance, preciso que acordos FX e suas transições de estado sejam persistidos e auditáveis para não perder histórico em reinícios e permitir reconciliação regulatória.

**Why this priority**: Sem persistência e trilha de auditoria, qualquer falha de serviço pode invalidar acordos em andamento e comprometer exigências de governança.

**Independent Test**: Pode ser testado de forma independente criando um acordo, executando transições de estado, reiniciando os serviços e confirmando recuperação completa do estado e do histórico de eventos.

**Acceptance Scenarios**:

1. **Given** um acordo FX proposto, **When** o serviço é reiniciado, **Then** o acordo permanece disponível com o mesmo estado e dados de negócio.
2. **Given** uma transição de estado de acordo FX, **When** a transição é concluída, **Then** um evento de auditoria imutável é registrado com ator, horário e mudança de estado.
3. **Given** um auditor autorizado, **When** consulta o histórico de um `trade_id`, **Then** recebe a sequência completa e ordenada de eventos do ciclo de vida do acordo.

---

### User Story 2 - Sincronizar acordos entre spokes com confiabilidade (Priority: P2)

Como banco participante em spoke distinto, preciso receber atualizações de acordo FX com entrega confiável para evitar divergência de estado entre contraparte e originador.

**Why this priority**: A divergência de estados entre spokes impede execução segura de PvP e aumenta risco operacional.

**Independent Test**: Pode ser testado de forma independente simulando indisponibilidade temporária de uma contraparte durante propagação de eventos e verificando reentrega até confirmação.

**Acceptance Scenarios**:

1. **Given** um evento de acordo pendente de propagação, **When** a contraparte está indisponível, **Then** o sistema agenda nova tentativa com retentativas até confirmação de entrega.
2. **Given** uma reinicialização do mecanismo de sincronização, **When** ele retoma operação, **Then** não duplica eventos já confirmados e continua processando eventos pendentes.
3. **Given** um acordo que muda para `CANCELLED` ou `SETTLED`, **When** a sincronização ocorre, **Then** o mesmo estado terminal é refletido na contraparte.

---

### User Story 3 - Garantir acordo bilateral privado com execução HTLC segura (Priority: P3)

Como banco originador e banco contraparte, preciso que os termos do acordo sejam gerenciados em ambiente bilateral privado e que a execução HTLC só ocorra para acordos válidos e aceitos, eliminando bypass fora do fluxo autorizado.

**Why this priority**: Privacidade bilateral e acoplamento forte com HTLC reduzem risco de execução indevida e fortalecem confiança entre instituições.

**Independent Test**: Pode ser testado de forma independente ao criar acordo bilateral privado, aceitar o acordo pelas partes e validar que bloqueios HTLC são aceitos apenas com termos consistentes e estado elegível.

**Acceptance Scenarios**:

1. **Given** um acordo bilateral ainda não aceito, **When** uma tentativa de lock HTLC é enviada, **Then** a operação é rejeitada por estado inválido do acordo.
2. **Given** um acordo bilateral aceito e não expirado, **When** é solicitado lock HTLC com valores e destinatários consistentes, **Then** a operação é autorizada.
3. **Given** tentativa de lock com `trade_id` inexistente, expirado ou com termos divergentes, **When** a validação é executada, **Then** a operação é recusada com motivo auditável.
4. **Given** ambiente local com Paladin em ambos os spokes, **When** o setup completo é executado, **Then** os fluxos Zeto e Pente ficam disponíveis simultaneamente para operação real de ponta a ponta.

---

### Edge Cases

- Acordo criado com data de expiração já vencida deve ser recusado na origem.
- Acordo em `PROPOSED` que ultrapassa expiração sem ação das partes deve ser encerrado automaticamente.
- Reentrega de evento já confirmado não pode gerar transição duplicada de estado.
- Atualizações fora de ordem entre spokes não podem sobrescrever estado terminal já confirmado.
- Tentativa de settlement manual sem confirmação dos pré-requisitos de liquidação deve ser bloqueada.
- Falha parcial durante propagação cross-spoke deve preservar consistência eventual com reprocessamento seguro.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema MUST persistir acordos FX de forma durável, mantendo disponibilidade após reinício de serviços.
- **FR-002**: O sistema MUST registrar transições de estado em trilha de auditoria append-only para cada `trade_id`.
- **FR-003**: O sistema MUST permitir consulta de acordo por identificador único e listagem por filtros de estado e contraparte.
- **FR-004**: O sistema MUST validar consistência de termos financeiros do acordo no momento da proposta (valores, moedas e taxa implícita).
- **FR-005**: O sistema MUST aplicar controle de expiração, impedindo ações de negócio sobre acordos vencidos.
- **FR-006**: O sistema MUST executar cancelamento automático de acordos vencidos que permaneçam em estado não terminal.
- **FR-007**: O sistema MUST sincronizar mudanças de estado entre spokes com mecanismo de reentrega até confirmação.
- **FR-008**: O sistema MUST impedir duplicação de processamento para eventos já confirmados entre spokes.
- **FR-009**: O sistema MUST propagar estados de ciclo completo, incluindo `PROPOSED`, `ACCEPTED`, `REJECTED`, `CANCELLED` e `SETTLED`.
- **FR-010**: O sistema MUST proteger o canal interno de sincronização entre serviços com autenticação de serviço para serviço.
- **FR-011**: O sistema MUST criar e manter contexto bilateral privado para cada par de instituições envolvido no acordo.
- **FR-012**: O sistema MUST registrar referência ao contexto privado bilateral e ao contrato de acordo associado para cada `trade_id`.
- **FR-013**: O sistema MUST exigir que operações HTLC vinculadas a acordo validem estado `ACCEPTED`, não expiração e correspondência de termos.
- **FR-014**: O sistema MUST rejeitar execução HTLC quando não houver vínculo verificável com acordo válido e autorizado.
- **FR-015**: O sistema MUST implementar dual-gate enforcement on-chain para HTLC usando `CommitmentHashRegistry` como gate primário on-chain:
  - **Phase A (Current)**: Service-layer validates agreement state (PRIMARY) + CommitmentHashRegistry gate on Besu (FALLBACK)
    - Service persists agreement state in PostgreSQL and validates before HTLC.lock()
    - CommitmentHashRegistry stores `keccak256(tradeId || originAmount || counterAmount || rate)` hashes for additional verification
    - HTLC.lock() requires isAccepted(commitmentHash) == true on CommitmentHashRegistry
  - **Phase B (Future)**: When Pente externalCalls ready, enable atomic accept() → lock() coupling within private transaction
  - **Rationale**: Service-layer enforcement prevents bypass if on-chain gate compromised; CommitmentHashRegistry gate ensures on-chain verifiability; Phase B adds atomicity when infrastructure ready
- **FR-016**: O sistema MUST registrar ator, timestamp e transição de estado (de/para) em todas as mutações de acordo.
- **FR-017**: O sistema MUST tornar histórico e estado de acordos disponíveis para reconciliação operacional e auditoria regulatória.
- **FR-018**: O sistema MUST manter persistência de referência ao contexto privado bilateral (GroupID e ContractAddress Pente) para cada `trade_id` no banco de dados.
- **FR-019**: O sistema MUST impedir proposições de HTLC contra `trade_id` sem correspondente entrada em CommitmentHashRegistry ou FXAgreement, exceto em modo desenvolvimento explícito.
- **FR-020**: O sistema MUST prover bootstrap local automatizado para Paladin com disponibilidade simultânea dos domínios Zeto e Pente em spoke-a e spoke-b, incluindo configuração, deploy e validação operacional mínima.

### Key Entities *(include if feature involves data)*

- **FX Agreement**: Representa o acordo bilateral pré-liquidação com `trade_id`, partes, valores, moedas, taxa, expiração e estado.
- **FX Agreement Event**: Representa cada mutação de ciclo de vida com referência ao acordo, estado anterior, estado novo, ator e horário.
- **Commitment Hash Record**: Representa o hash verificável `keccak256(tradeId || originAmount || counterAmount || rate)` de um acordo para gating on-chain HTLC. Armazenado em CommitmentHashRegistry com estado (PENDING, ACCEPTED, SETTLED, CANCELLED).
- **Cross-Spoke Relay Item**: Representa um evento de sincronização pendente, em tentativa ou confirmado, com metadados de deduplicação e tentativas.
- **Private Bilateral Context**: Representa o identificador do contexto privado entre as duas instituições (GroupID) e a referência do contrato de acordo Pente (ContractAddress).
- **HTLC Lock Record**: Representa a coordenação de lock (não o escrow de tokens, que é privado via Zeto) com vínculo verificável ao acordo FX via CommitmentHashRegistry ou Pente FXAgreement.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% dos acordos criados permanecem recuperáveis após reinício planejado ou não planejado dos serviços.
- **SC-002**: 100% das transições de estado de acordos possuem registro de auditoria com ator, horário e mudança de estado.
- **SC-003**: Pelo menos 99,9% dos eventos de sincronização cross-spoke são confirmados em até 2 minutos em condição normal de rede.
- **SC-004**: 100% dos eventos que falham na primeira tentativa entram em fluxo de reentrega até sucesso ou status terminal tratável.
- **SC-005**: 100% das tentativas de lock HTLC com acordo inválido, expirado ou inconsistente são bloqueadas na service-layer (PRIMARY gate).
- **SC-006**: 100% dos hashes de commitment registrados em CommitmentHashRegistry são verificáveis on-chain e consistentes com valores do acordo.
- **SC-007**: 100% dos acordos expirados em estado não terminal são encerrados automaticamente dentro da janela operacional definida.
- **SC-008**: Em testes de reconciliação bilateral, divergência de estado entre spokes para o mesmo `trade_id` é inferior a 0,1% por ciclo diário.
- **SC-009**: Em auditoria de amostra, 100% dos acordos possuem encadeamento completo e verificável entre proposta, aceite/rejeição/cancelamento e encerramento.
- **SC-010**: Para acordos com contexto privado Pente: 100% possuem referência válida (GroupID + ContractAddress) persistida após aceitação.
- **SC-011**: Em testes de deployment: HTLC com CommitmentHashRegistry funciona sem Pente; HTLC com Pente FXAgreement funciona quando contexto privado disponível.
- **SC-012**: Em ambiente local OBRIGATÓRIO, execução de setup automatizado DEVE disponibilizar Zeto E Pente (ambos REQUIRED, não opcionais) no Paladin dos dois spokes com evidência clara de operação (Zeto token instance criada E criação de contexto FX Pente com retorno de `group_id` e `contract_address`). Este é um deliverable crítico para validação de produção.

## Assumptions

- O escopo cobre evolução do modelo atual para operação de produção sem substituir o fluxo de negócio principal de PvP.
- Os participantes institucionais e papéis de negócio já existentes (originador, contraparte e autoridade liquidante) permanecem válidos.
- **Phase A (Current)**: CommitmentHashRegistry em Besu é a abordagem de deploy imediato; não requer Pente externalCalls para funcionar.
- **Phase B (Future)**: O mecanismo de contexto privado bilateral (Pente) será disponibilizado para integração mais profunda com externalCalls atômicas quando infraestrutura estiver pronta.
- A sincronização cross-spoke continuará necessária mesmo com persistência local, para manter espelhamento entre instituições.
- A automação de expiração utilizará janela operacional definida pelo time de negócio e compliance.
- Notificações pró-ativas de expiração são desejáveis, porém opcionais nesta fase.
- CommitmentHashRegistry em Besu é OBRIGATÓRIO para operação de produção.
- **OPERAÇÃO LOCAL (Paladin) — OBRIGATÓRIO**: Zeto E Pente DEVEM ambos estar operacionais (não opcionais/feature-flagged) para validação de ambiente local. Phase 7 (T043–T050) é deliverable crítico que garante operação simultânea e confiável de ambos os subsistemas.
- FXAgreement em Pente será deployado sob demanda quando contexto bilateral é criado (via PenteClient HTTP API) durante fase de aceitação do acordo.
- **Implementation Status (2026-04-15)**: CommitmentHashRegistry implementado e testado em Phase A (atual). Integração de aplicação com Pente (`PENTE_ENABLED`, `PENTE_BASE_URL`, `PenteClient`) implementada. Phase 7 (Paladin local Zeto+Pente orchestration) é DELIVERABLE CRÍTICO ainda pendente — garante operação real simultânea de ambos os subsistemas conforme exigido para produção.
