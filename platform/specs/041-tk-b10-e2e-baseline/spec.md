# Feature Specification: Toolkit do Cenário B — E2E completo + baseline de performance (TK-B10)

**Feature Branch**: `041-tk-b10-e2e-baseline`
**Created**: 2026-07-11
**Status**: Draft
**Input**: User description: "TK-B10 — E2E + baseline, conforme
`scenario-b/docs/design/scenario-b-toolkit-roadmap.md`."

> A fase de **fechamento** do toolkit: um **E2E de pipeline completo** que encadeia os modos já
> entregues — `found-hub` → `found-spoke` (dois CBs) → `join` (banco comercial) → cauda soberana
> (`open-sovereign-pair`/`commit-liquidity`/`seed-oracle`) — contra Docker real, e então **exercita o
> caminho de negócio**: um **swap pela AMM** do par, a **checagem do circuit breaker** (pause 1-de-N /
> resume quorum 2) e os caminhos **lock→mint / burn→unlock** (com **timeout/refund**) do `SpokeBridge`.
> Valida também a **idempotência** (re-`apply` converge) de ponta a ponta. Acompanha um **baseline de
> performance toolkit-native** (harness em Go que mede a latência p95 de quote/swap contra a stack
> provisionada pelo toolkit) e um documento **`E2E-STATUS.md`** que registra o estado do E2E (espelho
> do `scenario-a/toolkit/E2E-STATUS.md`). Como todo E2E do toolkit, **skip com aviso** quando o
> ambiente (Docker/Foundry/Besu/relay) está ausente — nunca falso verde. **Fora de escopo**: novos
> caminhos de produto/contrato; mudanças na lógica de breaker/bridge; a implementação de produção
> (KMS/CA reais) — o E2E exercita o que os modos já provisionam.

## Clarifications

### Session 2026-07-11

- Q: Profundidade do caminho de negócio (US2) — o timeout/refund do `SpokeBridge` pode ser instável em
  local. → A: **Tudo, com timeout/refund como sub-teste skipável** — swap + breaker (pause/resume) +
  `lock→mint`/`burn→unlock` são asserções **firmes**; o **timeout/refund** é um sub-teste **separado**
  com **skip-com-aviso** quando o avanço de tempo/bloco não é viável no ambiente.
- Q: Como fornecer o baseline de performance (US4)? → A: **Baseline toolkit-native novo (Go)** — um
  harness próprio do toolkit (Go benchmark/harness) que mede a latência de quote/swap contra a stack
  provisionada pelo toolkit; **não** se reusa o k6 de `tests/performance`. As métricas p95 são
  registradas no `E2E-STATUS.md`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — E2E de pipeline completo do toolkit (Priority: P1)

Como mantenedor do toolkit, quero um **único E2E** que provisione a topologia inteira via os modos do
toolkit — hub, dois spokes soberanos, um banco comercial e um par soberano aberto com liquidez — contra
Docker real, de modo que eu tenha confiança de que os modos **compõem** de ponta a ponta (não apenas
isolados).

**Why this priority**: os E2E por modo (TK-B7..B9) validam cada modo isolado; falta a prova de que eles
**encadeiam** numa topologia real. É o MVP desta fase.

**Independent Test**: com Docker/Foundry/Besu disponíveis, o E2E roda `apply` de `found-hub` →
`found-spoke` (CB-A, CB-B) → `join` (banco) → cauda soberana, e verifica que o hub, os spokes e o banco
sobem, o par fica `ACTIVE` e o banco sincroniza; ambiente ausente ⇒ **skip com aviso**.

**Acceptance Scenarios**:

1. **Given** um ambiente com Docker/Foundry/Besu, **When** o E2E de pipeline roda, **Then** `found-hub`
   sobe o hub + contratos + relay; dois `found-spoke` sobem os spokes (CB validador único) e registram
   os CBs no hub; o `join` do banco sincroniza como full node não-validador; o par soberano fica
   `ACTIVE` com liquidez de ambos os lados.
2. **Given** o ambiente E2E ausente (sem Docker/Besu), **When** o E2E roda, **Then** ele faz **skip com
   aviso** (nunca falha por ambiente ausente; nunca falso verde).

---

### User Story 2 — Exercício do caminho de negócio (swap + breaker + bridge) (Priority: P1)

Como mantenedor do toolkit, quero que o E2E **exercite o caminho de negócio** sobre a stack
provisionada — um **swap pela AMM** do par, a **checagem do circuit breaker** (pause/resume) e os
caminhos **lock→mint / burn→unlock** do `SpokeBridge`, incluindo **timeout/refund** — de modo a provar
que a stack provisionada pelo toolkit é operacional, não só "no ar".

**Why this priority**: provisionar sem exercitar o fluxo não prova operacionalidade; o roadmap §574
exige swap + breaker + timeout/refund. É P1 junto com o pipeline.

**Independent Test**: sobre a topologia do US1, o E2E executa um swap na AMM e confirma o resultado;
faz `pause` (1 CB) e confirma que swaps são bloqueados, depois `resume` (2 CBs) e confirma retomada;
exercita `lock→mint` e `burn→unlock` do `SpokeBridge` (asserções firmes); o caminho de
**timeout/refund** é um **sub-teste separado** que faz **skip-com-aviso** quando o avanço de tempo/bloco
não é viável no ambiente.

**Acceptance Scenarios**:

1. **Given** um par `ACTIVE` com liquidez, **When** um swap é submetido à AMM, **Then** o swap
   liquida com a taxa do oráculo; **When** o breaker é pausado (1-de-N) **Then** novos swaps são
   recusados; **When** o resume atinge o quorum (2) **Then** os swaps retomam.
2. **Given** um settlement cross-network via `SpokeBridge`, **When** o caminho feliz roda, **Then**
   `lock→mint` (e `burn→unlock`) completa atomicamente.
3. **Given** o sub-teste de **timeout/refund**, **When** o ambiente permite avançar tempo/bloco com um
   settlement deliberadamente não-concluído, **Then** o caminho de timeout/refund reverte/reembolsa (sem
   liquidação parcial); **When** o avanço de tempo/bloco não é viável, **Then** o sub-teste faz
   **skip-com-aviso** (nunca falso verde).

---

### User Story 3 — Idempotência de ponta a ponta (Priority: P2)

Como mantenedor do toolkit, quero que o E2E confirme que **re-rodar `apply`** em cada modo **converge**
(passos concluídos são pulados; nada é duplicado/recriado), de modo a validar a garantia central de
idempotência do toolkit numa topologia real.

**Why this priority**: a idempotência é validada por unidade em cada modo; o E2E prova o comportamento
real (estado on-chain/persistido), mas é secundário ao pipeline+negócio.

**Independent Test**: após o pipeline do US1, re-rodar `apply` de cada modo produz um report em que os
passos aparecem como `skipped` (ou `done` via estado), sem recriar redes/contratos/registros.

**Acceptance Scenarios**:

1. **Given** o pipeline já provisionado, **When** `apply` de qualquer modo é re-executado, **Then** o
   report mostra os passos como `skipped`/`done` e nenhum efeito duplicado ocorre (mesmo genesis, mesmos
   endereços, par ainda `ACTIVE`).

---

### User Story 4 — Baseline de performance (toolkit-native) + E2E-STATUS (Priority: P2)

Como mantenedor do toolkit, quero um **baseline de performance próprio do toolkit** (harness em Go) que
meça a latência de quote/swap contra a stack provisionada pelo toolkit, e um documento
**`E2E-STATUS.md`** que registre o estado do E2E, de modo a satisfazer o requisito "production-grade" do
roadmap e dar visibilidade do que o E2E cobre.

**Why this priority**: baseline e status são exigências de maturidade (roadmap §576/§11), mas dependem
do pipeline (US1) existir; portanto P2.

**Independent Test**: o baseline toolkit-native (Go) roda contra a stack do toolkit e emite métricas de
latência (p95 de quote/swap); o `E2E-STATUS.md` existe e descreve o que o E2E cobre, como rodá-lo e o
estado atual (incl. as métricas p95).

**Acceptance Scenarios**:

1. **Given** a stack provisionada pelo toolkit, **When** o baseline toolkit-native (Go) roda, **Then**
   ele produz métricas de latência (p95 de quote/swap) sem erros de setup; ambiente ausente ⇒
   **skip-com-aviso**.
2. **Given** o repositório, **When** consulto `scenario-b/toolkit/E2E-STATUS.md`, **Then** ele descreve
   a topologia coberta, os passos do E2E, o comando de execução, as métricas p95 e o estado atual (o que
   passa / o que é skip).

### Edge Cases

- Ambiente E2E ausente (sem Docker/Foundry/Besu/relay) → **skip com aviso** em todos os testes E2E
  (nunca falha por ambiente; nunca falso verde).
- Um passo de provisionamento falha no meio do pipeline → o E2E reporta o passo e o report parcial;
  o teste falha com contexto claro (não engole erro).
- Re-rodar o pipeline sobre estado existente → converge (idempotente), não recria.
- Breaker pausado por 1 CB e resume tentado por 1 CB só → não retoma (quorum 2 não atingido).
- Settlement do `SpokeBridge` sem conclusão no prazo → timeout/refund (sem liquidação parcial).
- Baseline sem a stack no ar → **skip com aviso** (nunca métricas falsas).
- Sub-teste de timeout/refund sem meio de avançar tempo/bloco → **skip com aviso** (não falha).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A feature MUST incluir um **E2E de pipeline completo** (build tag `e2e`) que provisione,
  via os modos do toolkit e contra Docker real, a topologia: `found-hub` → `found-spoke` (CB-A e CB-B)
  → `join` (um banco) → cauda soberana (par `ACTIVE` + liquidez de ambos os lados).
- **FR-002**: O E2E MUST **exercitar um swap** pela AMM do par e confirmar a liquidação com a taxa do
  oráculo.
- **FR-003**: O E2E MUST **checar o circuit breaker**: `pause` (1-de-N) bloqueia swaps; `resume`
  (quorum 2) os retoma.
- **FR-004**: O E2E MUST **exercitar o `SpokeBridge`**: `lock→mint` e `burn→unlock` no caminho feliz
  (asserções firmes). O caminho de **timeout/refund** (sem liquidação parcial) MUST ser um **sub-teste
  separado** que faz **skip-com-aviso** quando o ambiente não permite avançar tempo/bloco com um
  settlement não-concluído.
- **FR-005**: O E2E MUST validar a **idempotência**: re-`apply` de cada modo converge (passos
  `skipped`/`done`; sem duplicação de redes/contratos/registros).
- **FR-006**: Todo teste E2E MUST fazer **skip com aviso** quando o ambiente (Docker/Foundry/Besu/relay)
  está ausente — **nunca** falso verde e **nunca** falha por ambiente ausente.
- **FR-007**: Uma falha de provisionamento ou de asserção no E2E MUST reportar contexto claro (passo,
  report parcial, erro) — sem engolir erros (observabilidade).
- **FR-008**: A feature MUST fornecer um **baseline de performance toolkit-native** (harness em Go) que
  meça a latência de quote/swap (p95) contra a stack provisionada pelo toolkit; **skip-com-aviso**
  quando a stack está ausente. **Não** reusa o k6 de `tests/performance`.
- **FR-009**: A feature MUST incluir um documento **`scenario-b/toolkit/E2E-STATUS.md`** descrevendo a
  topologia coberta, os passos do E2E, o comando de execução e o estado atual (espelho do
  `scenario-a/toolkit/E2E-STATUS.md`).
- **FR-010**: A feature MUST **reusar** os E2E por modo já entregues (TK-B7..B9) e o executor/apply do
  toolkit — **sem** duplicar a lógica dos modos e **sem** alterar contratos/Makefiles/`deploy/local`
  nem importar `scenario-a/`.
- **FR-011**: A suíte E2E completa MUST ser executável por um único comando documentado (ex.:
  `go test -tags e2e ./tests/e2e/...`) e o pipeline completo por um teste/alvo identificável.

### Key Entities

- **E2E de pipeline (`e2e` build tag)**: o teste que encadeia `found-hub`→`found-spoke`×2→`join`→cauda
  soberana e exercita swap/breaker/bridge; skip-com-aviso.
- **Baseline de performance (toolkit-native)**: harness em Go do próprio toolkit que mede a latência
  p95 (quote/swap) contra a stack provisionada pelo toolkit; skip-com-aviso.
- **`E2E-STATUS.md`**: documento de estado do E2E do toolkit (topologia, passos, comando, o que passa/é
  skip).
- **Topologia do E2E**: hub + 2 spokes soberanos (CB-A/CB-B) + 1 banco comercial + 1 par soberano
  `ACTIVE` com liquidez.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Num ambiente com Docker/Foundry/Besu, o E2E de pipeline provisiona a topologia completa e
  todas as asserções de US1 passam (hub/spokes/banco no ar; par `ACTIVE`; banco sincronizado).
- **SC-002**: O E2E confirma um **swap** liquidado; **pause** bloqueia swaps; **resume** (quorum 2) os
  retoma (US2).
- **SC-003**: O E2E confirma `lock→mint`/`burn→unlock` no caminho feliz; o sub-teste de
  **timeout/refund** confirma reversão/reembolso sem liquidação parcial **ou** faz skip-com-aviso quando
  o avanço de tempo/bloco não é viável (US2).
- **SC-004**: Re-rodar `apply` de cada modo após o pipeline resulta em passos `skipped`/`done` e **zero**
  efeitos duplicados (US3).
- **SC-005**: Em ambiente sem Docker/Besu, **100%** dos testes E2E fazem **skip com aviso** (0 falhas
  por ambiente; 0 falsos verdes).
- **SC-006**: O baseline toolkit-native (Go) roda contra a stack do toolkit e emite métricas de latência
  p95 (quote/swap) sem erro de setup; ambiente ausente ⇒ skip-com-aviso; o procedimento está documentado.
- **SC-007**: `scenario-b/toolkit/E2E-STATUS.md` existe e cobre topologia, passos, comando de execução e
  estado atual.

## Assumptions

- **Padrão de execução (consistente com TK-B4..B9): E2E sob build tag `e2e`, skip-com-aviso** quando o
  ambiente está ausente; nunca falso verde. O E2E de pipeline pressupõe Docker/Foundry/Besu (+ relay)
  disponíveis para rodar de verdade.
- **Baseline toolkit-native (resolvido, Q2)**: um **harness em Go** novo, no próprio toolkit, mede a
  latência p95 (quote/swap) contra a stack provisionada pelo toolkit; **não** reusa o k6 de
  `tests/performance`. Skip-com-aviso quando a stack está ausente.
- **Caminho de negócio (resolvido, Q1)**: swap + breaker + `lock→mint`/`burn→unlock` são asserções
  firmes; o **timeout/refund** é sub-teste separado com skip-com-aviso (avanço de tempo/bloco pode não
  ser viável em local).
- **Swap/breaker/bridge exercitados via os contratos/serviços já provisionados** (via `cast`/RPC/API do
  gateway) — o E2E é o consumidor; não há novo contrato nem mudança de lógica de breaker/bridge.
- **`E2E-STATUS.md`** espelha o formato do `scenario-a/toolkit/E2E-STATUS.md` (resumo, topologia,
  passos, como rodar, estado atual).
- **Fora de escopo**: TK-B10 não adiciona modos/contratos novos; não altera Makefiles de deploy nem
  `deploy/local`; não importa `scenario-a/`; a implementação de produção (KMS/CA reais) permanece
  diferida como no resto do toolkit.
