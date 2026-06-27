# Feature Specification: RL-1/RL-2/RL-3 — Relay: Registro Dinâmico de Spokes

**Feature Branch**: `031-relay-spoke-registry`
**Created**: 2026-06-27
**Status**: Draft
**Input**: User description: "FASE 2 — RL-1/RL-2/RL-3 — Relay: registro dinâmico de spokes, roteamento por dest_spoke_id e polling N-spoke"

## Contexto

O relay Cacti HTLC atualmente suporta exatamente dois spokes (`spokeA` / `spokeB`) fixos em `config.ts` e interliga seus endpoints gRPC estaticamente no startup. A função `resolveCounterpartContractId` em `htlc-relay.ts` encontra o contrato contraparte por exclusão do spoke de origem (`l.spoke !== spokeName`) — premissa que quebra com mais de dois spokes. Este conjunto de itens substitui a topologia bilateral estática por um **registro de spokes** (`spokes[]`) carregado de configuração, recabeia a resolução de contraparte para usar `dest_spoke_id` (já presente nos legs após MD-3) e generaliza o loop de polling para iterar sobre todos os spokes registrados.

**Pré-requisito**: `022-update-proto-consumers` (MD-3) merged — os legs do `FXAgreement` já carregam `source_spoke_id` / `dest_spoke_id`, e o polling REST em `htlc-relay.ts` já lê esses campos.

**Itens cobertos**: RL-1 (`config.ts`), RL-2 (`resolveCounterpart`), RL-3 (polling dinâmico). Entregues juntos por acoplamento direto.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Operador registra N spokes via YAML de configuração (Priority: P1)

Um operador da LNET inicia o relay Cacti com `CACTI_SPOKES_CONFIG=/etc/cacti/spokes.yaml` apontando para um YAML com dois ou mais entries de spoke. O relay inicia, loga cada spoke registrado e começa a fazer polling de todos eles — sem nenhuma alteração de código ou rebuild da imagem.

**Why this priority**: Habilitador central — o relay precisa ser reconfigurável sem edição de código antes que o piloto com banco central possa adicionar qualquer segundo participante.

**Independent Test**: Iniciar o relay com YAML de dois spokes em `CACTI_SPOKES_CONFIG`; verificar que o log de startup lista ambos com `[cacti] registered spoke: <id>`.

**Acceptance Scenarios**:

1. **Given** `CACTI_SPOKES_CONFIG=/etc/cacti/spokes.yaml` com dois entries de spoke válidos, **When** o relay inicia, **Then** loga `[cacti] registered spoke: <id> rpc=<url> htlc=<addr>` para cada entry e continua sem erros fatais.
2. **Given** YAML com campo obrigatório ausente (`besuRpc` faltando no spoke 0), **When** o relay inicia, **Then** encerra imediatamente com `Fatal: spoke[0].besuRpc is required`.
3. **Given** YAML com lista de spokes vazia, **When** o relay inicia, **Then** encerra com `Fatal: at least one spoke must be configured`.
4. **Given** `CACTI_SPOKES_CONFIG` não definido e vars legadas `SPOKE_A_BESU_RPC` + `SPOKE_B_BESU_RPC` presentes, **When** o relay inicia, **Then** constrói registro de dois spokes a partir das vars legadas, loga aviso de deprecação e inicia normalmente — sem alterar arquivos Compose existentes.
5. **Given** dois entries com o mesmo `id` no YAML, **When** o relay inicia, **Then** encerra com `Fatal: duplicate spoke id "<id>"`.

---

### User Story 2 — Relay roteia liquidação HTLC para o spoke de destino correto (Priority: P1)

Quando o spoke-a bloqueia um HTLC para uma trade destinada ao spoke-b, o relay lê `dest_spoke_id` do FX Agreement e encaminha o segredo para o endpoint gRPC do `payment-orchestrator` do spoke-b — obtido do registro, não de um campo `counterpartGrpc` estático.

**Why this priority**: Sem roteamento correto, a liquidação falha para qualquer par de trade além de (spoke-a → spoke-b). O `counterpartGrpc` é a última premissa bilateral hardcoded no caminho quente de liquidação.

**Independent Test**: Teste de integração — dois entries no registro; disparar lock no spoke-a com `dest_spoke_id=spoke-b`; verificar que `SettleHTLC` é chamado no cliente gRPC do spoke-b (não do spoke-a).

**Acceptance Scenarios**:

1. **Given** registro com `spoke-a` e `spoke-b`, **When** spoke-a emite `LogHTLCClaimed` para trade com `dest_spoke_id=spoke-b`, **Then** o relay chama `SettleHTLC` no endpoint gRPC do spoke-b.
2. **Given** registro com `spoke-a` e `spoke-b`, **When** trade chega com `dest_spoke_id=spoke-c` (ausente do registro), **Then** o relay loga erro `[spoke-a] settlement skipped: dest_spoke_id "spoke-c" not in registry` e continua sem crash.
3. **Given** registro com três spokes (a, b, c), **When** spoke-a emite lock para trade com `dest_spoke_id=spoke-c`, **Then** o relay chama `SettleHTLC` no endpoint gRPC do spoke-c (não do spoke-b).
4. **Given** FX proposal com `dest_spoke_id` diferente de qualquer spoke no registro, **When** o relay processa o proposal no loop de polling REST, **Then** loga erro e ignora o proposal sem encaminhá-lo incorretamente.

---

### User Story 3 — Polling dinâmico cobre todos os spokes registrados (Priority: P2)

O loop de polling do relay itera sobre `config.spokes[]` dinamicamente, de modo que adicionar um novo entry ao YAML e reiniciar o relay é suficiente para incluir o novo spoke — sem nenhuma alteração de código.

**Why this priority**: Necessário para o modelo N-spoke; o loop já é estruturalmente um array em `HtlcRelay`, o que torna essa mudança de menor risco.

**Independent Test**: Iniciar o relay com três spokes no YAML; confirmar três loops de polling no log de startup: `[<id>] starting poll loop`.

**Acceptance Scenarios**:

1. **Given** três spokes no registro, **When** o relay inicia, **Then** loga loop de polling para cada spoke e um connector Besu por spoke aparece no mapa de connectors.
2. **Given** um spoke com Besu RPC inacessível, **When** o relay faz polling, **Then** loga warning para aquele spoke e continua fazendo polling dos demais sem interrupção.
3. **Given** terceiro spoke adicionado ao YAML e relay reiniciado, **When** o relay inicia, **Then** o terceiro spoke aparece nos logs de startup sem alteração de código.

---

### Edge Cases

- O que acontece se `grpcEndpoint` de um spoke está inacessível no startup? → O relay inicia normalmente; erros de conexão gRPC surgem por-trade durante liquidação com log de erro (sem fatal).
- O que acontece se o relay reinicia no meio de uma liquidação (relay store com trade em andamento)? → O replay do relay store não é afetado; o lookup usa o `dest_spoke_id` armazenado no evento.
- O que acontece se `CACTI_SPOKES_CONFIG` aponta para arquivo inexistente? → Fatal com `Fatal: cannot read spokes config: <path>: no such file`.
- O que acontece quando `CACTI_SPOKES_CONFIG` e vars legadas `SPOKE_A_*` estão definidos simultaneamente? → `CACTI_SPOKES_CONFIG` tem precedência; as vars legadas são ignoradas silenciosamente.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `config.ts` DEVE exportar `spokes: SpokeConfig[]` em lugar das propriedades nomeadas `spokeA` / `spokeB`.
- **FR-002**: `SpokeConfig` DEVE conter `{ id, besuRpc, besuWs, htlcAddress, internalApiUrl, grpcEndpoint }` e NÃO DEVE conter `counterpartGrpc` nem `counterpartName`.
- **FR-003**: O relay DEVE carregar a config de spokes de YAML especificado por `CACTI_SPOKES_CONFIG`, OU das vars legadas `SPOKE_A_*` / `SPOKE_B_*` (shim de compatibilidade), com YAML tendo precedência.
- **FR-004**: O startup DEVE falhar imediatamente com mensagem clara se: campo obrigatório de spoke ausente, IDs de spoke duplicados, ou lista de spokes vazia.
- **FR-005**: A lógica de resolução de contraparte em `htlc-relay.ts` DEVE ser substituída por lookup no registro usando `dest_spoke_id` do FX Agreement; o cliente gRPC para liquidação DEVE ser selecionado por `dest_spoke_id`.
- **FR-006**: A interface `SpokeDep` em `htlc-relay.ts` DEVE adicionar `grpcEndpoint: string` e DEVE remover `counterpartGrpc` e `counterpartName`.
- **FR-007**: O mapa de connectors Besu DEVE ser construído dinamicamente a partir de `config.spokes` no startup, com uma entrada por spoke keyed por `spoke.id`.
- **FR-008**: O relay DEVE logar cada spoke registrado no startup: `[cacti] registered spoke: <id> rpc=<url> htlc=<addr>`.
- **FR-009**: Quando `dest_spoke_id` de uma trade não for encontrado no registro durante liquidação, o relay DEVE logar erro e ignorar (não crashar).
- **FR-010**: O shim de vars legadas DEVE logar aviso de deprecação explícito indicando que `CACTI_SPOKES_CONFIG` é o método preferido.

### Key Entities

- **SpokeConfig**: `{ id: string, besuRpc: string, besuWs: string, htlcAddress: string, internalApiUrl: string, grpcEndpoint: string }` — substitui a struct anterior com `counterpartGrpc` / `counterpartName`.
- **Registro de spokes**: `config.spokes[]` — fonte única da verdade para todos os spokes ativos em runtime; indexado por `id` para lookup O(1) em liquidações.
- **Shim de compatibilidade**: lógica de carga das vars `SPOKE_A_*` / `SPOKE_B_*` que sintetiza dois entries de registro quando `CACTI_SPOKES_CONFIG` está ausente. Deprecado; mantido apenas para manter a rede de exemplo verde.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: O relay inicia com sucesso com YAML de dois spokes e ambos os loops de polling aparecem nos logs em até 5 segundos do startup.
- **SC-002**: Uma liquidação HTLC end-to-end entre dois spokes provisionados independentemente via manifesto do toolkit completa com sucesso usando config de registro — sem alteração de código além do YAML.
- **SC-003**: `tsc --noEmit` passa com zero erros após a mudança.
- **SC-004**: Todos os testes unitários existentes de `HtlcRelay` passam sem modificação.
- **SC-005**: A rede de exemplo existente (Compose + Makefile do Scenario A) inicia sem modificação — o shim de vars legadas ativa automaticamente.
- **SC-006**: Quando trade com `dest_spoke_id` desconhecido chega, o relay loga erro e continua; nenhuma exceção não tratada ou crash do processo.
- **SC-007**: Adicionar terceiro spoke ao YAML e reiniciar o relay resulta em três loops de polling nos logs — sem alteração de código.

---

## Assumptions

- MD-3 (`022-update-proto-consumers`) está merged: `FXAgreementEvent` já carrega `source_spoke_id` e `dest_spoke_id`; o polling REST em `htlc-relay.ts` já lê esses campos (linhas 613-614 do arquivo atual).
- `js-yaml` está disponível no `package.json` do relay ou pode ser adicionado sem impacto; nenhum outro pacote externo é necessário.
- O cliente gRPC de cada spoke é criado uma única vez no startup e reutilizado (padrão atual); o registro dinâmico não exige criação de cliente por-trade.
- Os arquivos Docker Compose da rede de exemplo definem `SPOKE_A_*` / `SPOKE_B_*`; o shim mantém a rede verde sem tocar nesses arquivos (restrição explícita do planejamento).
- Isolamento de cenário: todas as mudanças ficam dentro de `scenario-a/interop/hub-and-spoke/cacti/`; nenhum arquivo do Scenario B é modificado.

---

## Constitution Check

| Regra | Status |
|---|---|
| Isolamento de cenário | ✅ Todas as mudanças dentro de `scenario-a/interop/hub-and-spoke/cacti/` |
| Privacidade (ZKP/notary) | ✅ Não aplicável — sem mudanças em tokens ou privacidade on-chain |
| Atomicidade | ✅ O fluxo de liquidação HTLC é preservado; muda apenas como a contraparte é identificada |
| Compliance gate | ✅ Não aplicável — relay é infraestrutura, não boundary de compliance |
| Test-first | ⚠️ Testes para `resolveCounterpartContractId` devem ser escritos antes de a implementação ser removida |
| Observabilidade | ✅ FR-008/FR-009/FR-010 adicionam logs estruturados; sem swallowing silencioso |

---

## Complexity Tracking

**Alternativa rejeitada — manter `counterpartGrpc` configurável por spoke**: Exigiria mapeamento estático (spoke-a → spoke-b, spoke-b → spoke-a) que quebra para N > 2. Rejeitado.

**Alternativa rejeitada — rotear por ring buffer de hashLock com N spokes**: O ring buffer cresce O(N) e ainda exige iterar todos os spokes para encontrar o lock correspondente. Rotear por `dest_spoke_id` (já no leg) é O(1) e correto. Rejeitado.

**Decisão de design — shim de compatibilidade para vars legadas**: Mantém a rede de exemplo verde sem alterar os arquivos Compose. O shim é load-time apenas; sintetiza a struct equivalente ao YAML a partir de `SPOKE_A_*` / `SPOKE_B_*`. Logado como aviso de deprecação para que operadores saibam migrar para `CACTI_SPOKES_CONFIG`.
