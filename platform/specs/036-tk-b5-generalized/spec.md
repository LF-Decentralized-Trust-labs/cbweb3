# Feature Specification: Toolkit do Cenário B — Relay generalizado (TK-B5)

**Feature Branch**: `036-tk-b5-generalized`
**Created**: 2026-07-10
**Status**: Draft
**Input**: User description: "TK-B5 — relay generalizado, conforme
`scenario-b/docs/design/scenario-b-toolkit-roadmap.md` (§9, §14.B)."

> Generalizar o relay Cacti (`interop/hub-and-spoke/cacti/`) de **dois spokes fixos** para **N
> spokes dinâmicos**: boot neutro (≥0 spokes), registry dinâmico, conectores/watchers por spoke,
> roteamento cross-currency por `spoke_out`, **registro em runtime** (`POST /api/v1/spokes`, sem
> restart) com **registro persistido** para sobreviver a reinícios, e **validação de circuit
> breaker (`isPaused`)** antes de encaminhar swaps (Princípio III). A **autenticação por CB**
> (§14.D) fica **fora de escopo** (pendente de aprovação do project-lead) — o segredo compartilhado
> atual permanece como fallback local/dev.

## Clarifications

### Session 2026-07-10

- Q: Como o relay obtém o endereço do AMM do par para checar `isPaused()`? → A: O payload do
  `bridge-out` passa a incluir `amm_address` (o endereço do AMM do par, conhecido pelo gateway de
  origem que executou o swap); o relay **lê `isPaused()` on-chain nesse endereço** antes de
  encaminhar (verificação autoritativa, não confiança cega). Endereço ausente/inválido ou leitura
  indisponível ⇒ falha segura (não encaminha).
- Nota (testabilidade de SC-001, achado C1): a fiação de conectores/watchers será **extraída em
  função pura** (`createSpokeRuntimes(spokes, factories)`) com factory injetável, de modo que
  "um conector/watcher por spoke" seja verificável sem um Besu real (N → N runtimes).

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Boot neutro e registry dinâmico de N spokes (Priority: P1)

Como operador do hub, quero que o relay **suba sem nenhum spoke fixo** e opere com **N spokes**
descobertos de uma lista/registry, de modo que 1, 3 ou mais spokes funcionem sem editar código —
hoje o relay não sobe com 1 spoke e ignora o 3º.

**Why this priority**: o hardcode de dois spokes é o bloqueador central (§14.B); sem o registry
dinâmico nada mais do relay generalizado é possível. É o MVP.

**Independent Test**: iniciar o relay com zero spokes (boot neutro OK); iniciar com uma lista de N
spokes e confirmar que há **um conector/watcher por spoke** (N = 1, 3), sem referência fixa a
`spoke-a`/`spoke-b`.

**Acceptance Scenarios**:

1. **Given** nenhuma configuração de spoke, **When** o relay inicia, **Then** ele sobe com sucesso
   (boot neutro) e reporta 0 spokes ativos, sem erro fatal.
2. **Given** uma lista de N spokes (ex.: 1 e depois 3), **When** o relay inicia, **Then** existe
   exatamente um conector/watcher por spoke e nenhum spoke é ignorado.
3. **Given** o código do relay, **When** inspecionado, **Then** não há identificador de spoke fixo
   (`spoke-a`/`spoke-b`) nem variável fatal `SPOKE_A/B_BESU_RPC`.

---

### User Story 2 — Registro de spokes em runtime, persistido (Priority: P1)

Como o toolkit (no `found-spoke`), quero **registrar um spoke novo em runtime** via
`POST /api/v1/spokes` para que o relay crie conector/watcher e a entrada de roteamento **sem
reiniciar**, e que esse registro **sobreviva a reinícios** do relay.

**Why this priority**: evita janela de manutenção no relay neutro compartilhado e encaixa no modelo
declarativo N-spokes; é a metade "runtime" do relay generalizado.

**Independent Test**: com o relay no ar (0 spokes), enviar `POST /api/v1/spokes` com os dados de um
spoke; confirmar que o spoke passa a ter conector/watcher/roteamento ativos sem restart; reiniciar
o relay e confirmar que o spoke registrado é recarregado do armazenamento persistido.

**Acceptance Scenarios**:

1. **Given** o relay no ar sem o spoke X, **When** recebo `POST /api/v1/spokes` para X, **Then** X
   passa a ser observado/roteável **sem restart** e a chamada é idempotente (re-registrar X não
   duplica).
2. **Given** um spoke X registrado, **When** o relay reinicia, **Then** X é recarregado do registro
   persistido e volta a operar sem novo POST.
3. **Given** um `POST /api/v1/spokes` com payload inválido (campos faltando), **When** processado,
   **Then** é rejeitado com erro claro e o registro não é alterado.

---

### User Story 3 — Roteamento cross-currency por spoke de destino (Priority: P1)

Como o relay, preciso rotear o `bridge-out` cross-currency para o **gateway do spoke de destino**
(`spoke_out`), e não para um CB fixo, de modo que qualquer par de N spokes seja atendido
corretamente — hoje o destino é sempre um `CB_B_GATEWAY_URL` fixo.

**Why this priority**: sem o lookup por `spoke_out`, um bridge-out para um terceiro spoke é
mal-roteado; é core para N spokes.

**Independent Test**: registrar dois spokes com gateways distintos; disparar um bridge-out com
`spoke_out` = cada um deles; confirmar que o encaminhamento vai ao gateway correto por lookup.

**Acceptance Scenarios**:

1. **Given** spokes registrados com gateways distintos, **When** um bridge-out chega com
   `spoke_out = X`, **Then** o relay encaminha ao gateway de X (lookup por spoke id), não a um CB
   fixo.
2. **Given** um bridge-out com `spoke_out` desconhecido, **When** processado, **Then** é rejeitado
   com erro claro (spoke não registrado).

---

### User Story 4 — Validação de circuit breaker antes do swap (Priority: P1)

Como o relay, devo **verificar `isPaused()`** no `AutomatedMarketMaker` do par antes de encaminhar
um swap, recusando o encaminhamento quando o corredor está pausado — hoje o relay não faz essa
checagem, o que descumpre o Princípio III da Constituição.

**Why this priority**: é uma exigência de atomicidade/segurança da Constituição (Princípio III) e um
defeito atual de conformidade; não é opcional.

**Independent Test**: com o par pausado (`isPaused` verdadeiro), disparar um swap e confirmar que o
relay **recusa** encaminhar; com o par ativo, confirmar que encaminha.

**Acceptance Scenarios**:

1. **Given** um corredor cujo AMM está pausado, **When** chega um swap para ele, **Then** o relay
   **não encaminha** e registra a recusa (motivo: circuit breaker).
2. **Given** um corredor ativo (não pausado), **When** chega um swap, **Then** o relay encaminha
   normalmente.

---

### User Story 5 — Interface `RelayRegistrar` no toolkit (Priority: P2)

Como o toolkit (motor, fase futura), quero uma **interface plugável `RelayRegistrar`** que registre
um spoke no relay, com **implementação local in-memory** (para testes/dev) e **stub de produção**
que fala com `POST /api/v1/spokes`, selecionável por **URI** — espelhando `KeyProvider`/`CertSource`.

**Why this priority**: torna o registro de spoke uma fronteira plugável consumível pelo motor
(TK-B6) e testável isoladamente; depende do endpoint do relay (US2) existir como contrato.

**Independent Test**: registrar um spoke pela implementação local e confirmar que o registry
in-memory reflete o spoke (idempotente); a factory resolve `local` vs. `relay://…` (prod); o stub de
prod expõe a chamada HTTP sem exigir um relay no ar no teste unitário.

**Acceptance Scenarios**:

1. **Given** a implementação local, **When** registro um spoke, **Then** o registry in-memory passa
   a contê-lo; re-registrar o mesmo id é idempotente.
2. **Given** uma URI não suportada, **When** construo o `RelayRegistrar`, **Then** recebo erro de
   configuração.

### Edge Cases

- Boot com 0 spokes → sucesso (neutro), 0 conectores.
- Re-registro do mesmo spoke → idempotente (sem duplicar conector/roteamento).
- `POST /api/v1/spokes` malformado → rejeitado, registro inalterado.
- `bridge-out`/swap para `spoke_out` não registrado → rejeitado com erro claro.
- Consulta de `isPaused` indisponível (RPC do hub fora) → falha segura: não encaminhar (não assumir
  "ativo").
- Reinício do relay → spokes persistidos recarregados; o `cacti-relay-store.json` legado (plano)
  não é usado.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O relay MUST **iniciar sem nenhum spoke fixo** (boot neutro, ≥0 spokes) e MUST NÃO
  falhar por ausência de `SPOKE_A/B_BESU_RPC` (essas variáveis fatais são removidas).
- **FR-002**: O relay MUST manter um **registry dinâmico** de spokes (coleção indexada por id de
  spoke), suportando N ≥ 0, sem identificadores fixos (`spoke-a`/`spoke-b`).
- **FR-003**: O relay MUST instanciar **um conector e um watcher por spoke** a partir do registry
  (em vez de dois literais).
- **FR-004**: O relay MUST expor `POST /api/v1/spokes` para **registrar um spoke em runtime**
  (dados mínimos: id do spoke, RPC/WS do Besu, URL do gateway), criando conector/watcher e entrada
  de roteamento **sem reiniciar**; a operação MUST ser **idempotente** por id de spoke.
- **FR-005**: O registro de spokes MUST ser **persistido** e **recarregado no boot**, sobrevivendo a
  reinícios do relay; o `cacti-relay-store.json` legado (plano) MUST NÃO ser a fonte.
- **FR-006**: `POST /api/v1/spokes` MUST **validar o payload** e rejeitar entradas inválidas com
  erro claro, sem alterar o registro.
- **FR-007**: O roteamento cross-currency (`bridge-out`) MUST resolver o destino por **lookup do
  gateway pelo `spoke_out`** (id do spoke de destino), não por um CB fixo; `spoke_out` desconhecido
  MUST ser rejeitado com erro claro.
- **FR-008**: Antes de encaminhar um `bridge-out`/swap, o relay MUST **consultar `isPaused()`
  on-chain** no AMM do par — cujo endereço vem no campo **`amm_address`** do payload — e **recusar**
  o encaminhamento quando pausado; `amm_address` ausente/inválido ou consulta indisponível ⇒ MUST
  falhar de forma segura (não encaminhar). O relay **não** confia num flag do payload: lê o estado
  do contrato no endereço informado.
- **FR-009**: O relay MUST registrar (log estruturado) o ciclo relevante — spoke registrado,
  match/roteamento, recusa por circuit breaker — sem engolir erros (Princípio VI).
- **FR-010**: A **autenticação por CB** (§14.D) MUST permanecer **fora de escopo** nesta fase; o
  mecanismo de segredo compartilhado atual permanece como fallback local/dev, e a recomendação de
  auth-por-CB fica documentada para implementação posterior (requer aprovação do project-lead).
- **FR-011**: `env-sample` e o `docker-compose.yaml` do relay MUST ser ajustados para o modelo
  dinâmico (sem exigir `SPOKE_A/B_*`), preservando o comportamento neutro no boot.
- **FR-012**: O toolkit MUST prover a interface `RelayRegistrar` (registrar um spoke: id, RPC/WS,
  gateway) com **implementação local in-memory** (registry idempotente) e **stub de produção** que
  encaminha para `POST /api/v1/spokes`, selecionáveis por **factory por URI** (`local` → in-memory;
  `relay://…` → stub de produção; URI não suportada → erro de configuração).
- **FR-013**: O `RelayRegistrar` MUST usar **erros tipados** e MUST NÃO persistir segredos; nenhum
  material sensível cruza a fronteira (coerente com as demais interfaces plugáveis).

### Key Entities

- **Spoke (registro do relay)**: id do spoke, RPC/WS do Besu, URL do gateway; unidade do registry e
  do registro persistido.
- **Spoke Registry**: coleção em memória, indexada por id, que dirige conectores/watchers e o
  roteamento; hidratada do registro persistido no boot e mutável em runtime.
- **Registro persistido de spokes**: representação durável do conjunto de spokes registrados, lida
  no boot e atualizada a cada registro.
- **Rota de gateway**: associação `spoke_out → URL do gateway` usada pelo roteamento cross-currency.
- **Payload de bridge-out**: `correlation_id`, `swap_tx_hash`, `amount_out`, `beneficiary_bank_id`,
  `spoke_out` e **`amm_address`** (endereço do AMM do par, para a checagem `isPaused` on-chain).
- **RelayRegistrar** (toolkit): fronteira plugável que registra um spoke no relay; impl. local
  in-memory + stub de produção (HTTP), selecionada por URI.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: O relay inicia com **0 spokes** (boot neutro) sem erro e com **N spokes** (testado com
  N = 1 e N = 3) com exatamente um conector/watcher por spoke.
- **SC-002**: Um spoke registrado via `POST /api/v1/spokes` passa a ser observado/roteável **sem
  restart**; re-registro do mesmo id é idempotente (não duplica).
- **SC-003**: Após reinício do relay, **100%** dos spokes previamente registrados são recarregados
  do registro persistido.
- **SC-004**: Um `bridge-out` com `spoke_out = X` é encaminhado ao **gateway de X**; `spoke_out`
  desconhecido é rejeitado com erro claro.
- **SC-005**: Com o par **pausado**, o relay **recusa** encaminhar o swap; com o par ativo,
  encaminha. `amm_address` ausente/inválido **ou** consulta de `isPaused` indisponível ⇒ recusa
  (falha segura).
- **SC-006**: O relay **não contém** identificadores de spoke fixos (`spoke-a`/`spoke-b`) nem
  variáveis fatais `SPOKE_A/B_BESU_RPC` (verificável).
- **SC-007**: Nenhuma mudança de autenticação é introduzida (auth-por-CB permanece fora de escopo);
  o comportamento de segredo compartilhado é preservado.
- **SC-008**: A factory do `RelayRegistrar` resolve `local` (in-memory) e `relay://…` (stub de
  prod); a implementação local registra um spoke de forma idempotente; URI não suportada gera erro.

## Assumptions

- **Escopo (decisão 2026-07-10) = duas metades**: (1) **generalização do relay Cacti
  (TypeScript)** em `interop/hub-and-spoke/cacti/` (edição cirúrgica de código existente, permitida
  pelo roadmap §9) + ajustes de `env-sample`/`docker-compose.yaml`; e (2) a interface Go
  **`RelayRegistrar`** no toolkit (interface + local in-memory + stub de produção HTTP que chama
  `POST /api/v1/spokes` + factory por URI), por simetria com `KeyProvider`/`CertSource` (TK-B2/B3),
  completando o registro em runtime ponta-a-ponta. O motor (TK-B6) consome o `RelayRegistrar`.
- **Registro persistido** = um arquivo durável (JSON) no volume do relay, lido no boot e atualizado
  a cada registro; substitui o `cacti-relay-store.json` legado/plano. (Padrão razoável; a feature
  `031-relay-spoke-registry` do roadmap descreve esse RelayStore.)
- **Auth por CB fora de escopo** (§14.D) — sensível a compliance, requer aprovação do project-lead;
  o segredo compartilhado permanece como fallback local/dev.
- O `LiquidityCommitRegistry` e os endereços de AMM de par soberano nascem em runtime; o relay
  aprende o(s) registry/AMM dinamicamente (sem hardcode), coerente com o modelo N-spokes.
- Não se altera a lógica do circuit breaker nos contratos (Princípio III preservado); apenas o relay
  passa a **validar** `isPaused()`.
- Isolamento de cenário: mudanças restritas a `scenario-b/` (relay + toolkit); nada de `scenario-a/`.
