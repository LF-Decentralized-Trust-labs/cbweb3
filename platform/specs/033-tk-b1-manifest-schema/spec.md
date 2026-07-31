# Feature Specification: Toolkit do Cenário B — Schema de Manifesto e Validação (TK-B1)

**Feature Branch**: `033-tk-b1-manifest-schema`
**Created**: 2026-07-10
**Status**: Draft
**Input**: User description: "Toolkit do Cenário B. Usando como referência
`scenario-b/docs/design/scenario-b-toolkit-roadmap.md`, iniciar a implementação do item TK-B1,
que é a porta de entrada do toolkit."

> Fundação (porta de entrada) do toolkit do Cenário B: o modelo declarativo de manifesto
> (`ParticipantDeployment`, `cbweb3b/v1`) com três modos (`found-hub`/`found-spoke`/`join`), o
> JSON-Schema de validação e a lógica de validação. **Não** inclui motor de steps, bundles,
> templates de compose nem execução — apenas parse + validação + report. As fases seguintes
> (TK-B2+) consomem este modelo. Fonte de decisões: roadmap §3, §4, §12, §13.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Validar um manifesto de um modo (Priority: P1)

Como operador, escrevo um manifesto YAML para um dos três modos e o toolkit **valida** e
reporta: aceita se válido; rejeita com erro específico e acionável se inválido.

**Why this priority**: é o MVP — sem um modelo de manifesto validado, nenhuma fase seguinte
(motor, steps, bundles) tem entrada confiável. Entrega valor sozinha (validação declarativa).

**Independent Test**: rodar a validação sobre os três manifestos-exemplo do roadmap (§4.1–4.3):
todos passam; introduzir erros (campo faltante, modo inválido, segredo embutido): cada um é
rejeitado com mensagem apontando o campo.

**Acceptance Scenarios**:

1. **Given** um manifesto `mode: found-hub` bem-formado, **When** valido, **Then** passa sem erros.
2. **Given** um manifesto `mode: found-spoke` sem `hubBundleRef`, **When** valido, **Then** falha
   com erro identificando `hubBundleRef` como obrigatório no modo.
3. **Given** um manifesto com `apiVersion`/`kind` incorretos, **When** valido, **Then** falha
   antes de qualquer outra checagem.

---

### User Story 2 — Detectar colisões e unicidade (Priority: P2)

Como operador provisionando N spokes no mesmo host, quero que o toolkit **detecte colisões** de
`chainId`, portas e nome de rede/spoke entre manifestos, em vez de auto-atribuir.

**Why this priority**: N spokes dinâmicos sem colisão é decisão estrutural (roadmap §13, §14.C);
sem essa checagem, o operador só descobre o conflito em runtime.

**Independent Test**: validar dois manifestos com o mesmo `chainId` (ou faixa de portas
sobreposta, ou mesmo `spoke.id`): erro de colisão nomeando o recurso e os dois manifestos.

**Acceptance Scenarios**:

1. **Given** dois manifestos com `spec.spoke.chainId` iguais, **When** valido o conjunto,
   **Then** falha apontando a colisão de chainId.
2. **Given** dois manifestos cujas bandas de porta (rpc/ws/p2p + derivadas) se sobrepõem,
   **When** valido, **Then** falha apontando a sobreposição.

---

### User Story 3 — Requisitos por modo (Priority: P3)

Como autor de manifesto, quero que campos obrigatórios/proibidos sejam **específicos por modo**,
para o schema me guiar (ex.: `join` exige `bankId` + `joinBundleRef`; `found-hub` exige `hub{}`).

**Why this priority**: reduz erro humano e documenta o contrato; complementa a US1.

**Independent Test**: para cada modo, remover um campo obrigatório e incluir um campo de outro
modo: a validação rejeita ambos com mensagens específicas.

**Acceptance Scenarios**:

1. **Given** `mode: join` sem `bankId`, **When** valido, **Then** falha (bankId obrigatório em join).
2. **Given** `mode: found-hub` com `hubBundleRef` (campo de found-spoke), **When** valido,
   **Then** falha (campo não pertence ao modo).

---

### Edge Cases

- `environment` diferente de `local` → rejeitado (só `local` implementado nesta fase).
- Manifesto contendo material de chave privada (bloco `PRIVATE KEY`, chave hex de conta) →
  rejeitado (nenhum segredo no manifesto).
- `certSource` fora de `self-signed` / `ca://…`; `keyProvider` fora de `kms://…` → rejeitado.
- `node.validator: true` em `join` → o modelo é full node não-validador (roadmap §6/§13); ver FR-013.
- YAML malformado / múltiplos documentos → erro de parse claro, sem stack trace.
- `dataDir` relativo → resolvido em relação ao cwd, sem falhar.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema MUST parsear um manifesto YAML `apiVersion: cbweb3b/v1`,
  `kind: ParticipantDeployment` e rejeitar qualquer outro `apiVersion`/`kind`.
- **FR-002**: O sistema MUST reconhecer `spec.mode ∈ {found-hub, found-spoke, join}` e
  `spec.topology.role ∈ {hub, central-bank, commercial-bank}` como discriminadores.
- **FR-003**: O sistema MUST exigir os campos por modo — `found-hub`: `hub{chainId,currency[]}`;
  `found-spoke`: `spoke{id,chainId,currency}` + `hubBundleRef`; `join`: `spoke{...}` +
  `joinBundleRef` + `bankId` — e MUST rejeitar campos pertencentes a outro modo.
- **FR-004**: O sistema MUST validar `node{advertisedHost, rpc/ws/p2p{port}, dataDir}` em todos
  os modos (`advertisedHost` obrigatório; portas inteiras válidas).
- **FR-005**: O sistema MUST aceitar `environment: local` e rejeitar `staging`/`prod` nesta fase.
- **FR-006**: O sistema MUST rejeitar qualquer manifesto que contenha material de segredo
  (blocos `PRIVATE KEY`, chaves privadas hex). Chaves e certificados são mediados por
  `keyProvider`/`certSource`, nunca inline.
- **FR-007**: O sistema MUST validar `keyProvider` (`kms://…`) e `certSource`
  (`self-signed` | `self-signed://…` | `ca://…`).
- **FR-008**: `spec.spoke.chainId` (e `hub.chainId`) MUST ser **informado pelo operador** (o
  sistema NÃO auto-atribui). Ao validar um **conjunto** de manifestos, o sistema MUST detectar
  colisões **respeitando a topologia de rede**: um `chainId` só colide entre **redes distintas**
  (um banco em `join` **compartilha legitimamente** o `chainId` e o `spoke.id` do CB fundador da
  mesma rede — não é colisão); um `spoke.id` só colide se **fundado por mais de um** manifesto
  `found-spoke`; as **portas de nó declaradas** (`rpc/ws/p2p`) MUST ser globalmente únicas; e
  `metadata.name` MUST ser único.
- **FR-009**: O sistema MUST oferecer validação por **JSON-Schema** (para edição/CI) **e**
  validação **programática** (regras semânticas que o schema não expressa: colisões, requisitos
  por modo, ausência de segredos), e as duas MUST concordar nas regras estruturais.
- **FR-010**: O sistema MUST expor um caminho de invocação que **apenas** parseia + valida +
  reporta, **sem executar steps** (ex.: `--dry-run`/`validate`), com saída selecionável
  (JSON ou YAML).
- **FR-011**: Erros de validação MUST identificar o campo/recurso e ser acionáveis; múltiplas
  violações MUST ser **coletadas e reportadas juntas** (sem parar na primeira).
- **FR-012**: No `found-spoke`, quando o par soberano estiver presente
  (`pair{proposerCB, confirmerCB, symbolA, symbolB}`), o sistema MUST validá-lo, incluindo
  `proposerCB ≠ confirmerCB` e `symbolA ≠ symbolB`.
- **FR-013**: Para `join`, o modelo é **full node não-validador** (roadmap §6/§13). Quando um
  manifesto de `join` declarar `node.validator: true`, o sistema MUST **aceitar e emitir um
  warning** de divergência do modelo (não rejeitar) — deixando aberta a promoção a validador
  diferida (`vote-qbft`) sem mudança de schema.

### Key Entities

- **ParticipantDeployment**: o manifesto declarativo (um por participante/modo). Atributos:
  `apiVersion`, `kind`, `metadata.name`, `spec{scenario, environment, mode, topology.role,
  displayName, hub?|spoke?, hubBundleRef?|joinBundleRef?, bankId?, pair?, node, image,
  keyProvider, certSource, relay, cbEndpoint?, frontendHost, adminUsers[]}`. Nunca contém segredos.
- **JSON-Schema v1**: o contrato estrutural (em `provisioning/schema/v1/`) consumido por
  editores/CI e espelhado pela validação programática.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Os três manifestos-exemplo do roadmap (§4.1–4.3) validam sem erro.
- **SC-002**: 100% dos casos-borda listados (modo inválido, campo faltante/estranho por modo,
  `environment` não-local, segredo embutido, `certSource`/`keyProvider` inválidos, colisão de
  chainId/porta) são **rejeitados** com mensagem que nomeia o campo/recurso.
- **SC-003**: Validar um conjunto com **colisão real** entre 2+ manifestos **falha** e nomeia os
  conflitantes — `chainId` repetido em **redes distintas**, `spoke.id` **fundado** por mais de um
  manifesto, ou **porta de nó declarada** repetida. Um conjunto `found-spoke` + `join` do mesmo
  spoke (que compartilham `chainId`/`spoke.id`) é **válido** (não é colisão).
- **SC-004**: A validação por JSON-Schema e a programática **concordam** nas regras estruturais
  (nenhum manifesto passa em uma e falha na outra para as mesmas regras estruturais).
- **SC-005**: Nenhum manifesto que contenha material de chave privada é aceito.

## Assumptions

- Só `environment: local` é suportado nesta fase; `staging`/`prod` ficam para fase futura.
- `kind` único `ParticipantDeployment` para os três modos (discriminadores `mode` +
  `topology.role`).
- `chainId`/portas vêm do manifesto (operador); o toolkit valida, não atribui (roadmap §13).
- Segredos (chaves/certs) são mediados por `keyProvider`/`certSource` — fora do manifesto.
- O escopo desta fase exclui motor de steps, bundles, templates de compose e execução (fases
  TK-B2+).
- Isolamento de cenário (Constituição, Princípio I): módulo próprio do Cenário B; o padrão de
  schema/validação do toolkit de referência (Cenário A) é **reimplementado, não importado**.
