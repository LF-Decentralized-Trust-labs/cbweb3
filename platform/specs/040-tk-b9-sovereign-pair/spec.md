# Feature Specification: Toolkit do Cenário B — par soberano + liquidez cooperativa + seed-oracle (TK-B9)

**Feature Branch**: `040-tk-b9-sovereign-pair`
**Created**: 2026-07-11
**Status**: Draft
**Input**: User description: "TK-B9 — par soberano (open-sovereign-pair) + liquidez cooperativa
(commit-reveal) + seed-oracle, conforme `scenario-b/docs/design/scenario-b-toolkit-roadmap.md`."

> A **cauda soberana** do `found-spoke`, deferida do TK-B7 para cá: quando um `found-spoke` carrega um
> bloco `spec.pair`, o toolkit **abre um corredor bilateral** entre dois bancos centrais soberanos —
> `open-sovereign-pair` (implanta/reutiliza os W-tokens por moeda, a AMM dedicada do par com o seu
> circuit breaker, o `LiquidityCommitRegistry`, mapeia `setCentralBankOf`, concede papéis ao relayer,
> e executa `proposePair` (CB-A) + `confirmPair` (CB-B) no `PairRegistry`, dirigido pela máquina de
> estados on-chain `PROPOSED→ACTIVE`) → `commit-liquidity` (cada CB contribui **apenas com a própria
> moeda** por commit-reveal no `LiquidityCommitRegistry`; o relay casa `CommitMatched`) → `seed-oracle`
> (alimenta o `ManualOracle` do hub com a taxa inicial do par). Os três steps são **soft** (falha não
> bloqueia o `found-spoke`) e a idempotência vive **on-chain** (re-`apply` converge: `ACTIVE` pula,
> `PROPOSED` aguarda a contraparte). **Soberania estrita** (inclusive em `local`): cada `apply` executa
> só o ato do CB do manifesto atual — CB-A propõe, um `found-spoke` separado de CB-B confirma; nenhum
> run detém a chave da contraparte, e o par fica `PROPOSED`/`pending` entre os dois atos. **Fora de escopo**:
> mudanças de contrato (breaker opção C, remoção da AMM base), numéraire/multi-hop, MLP, feeder
> contínuo de FX, e a implementação de produção (KMS/CA reais) — deferidos como o resto do toolkit.

## Clarifications

### Session 2026-07-11

- Q: Onde vivem os três steps soberanos (open-sovereign-pair, commit-liquidity, seed-oracle)? → A:
  **Cauda soft do `found-spoke`**, disparada pelo bloco opcional `spec.pair` (roadmap §404-407). O
  TK-B9 **estende o `found-spoke`** (TK-B7); não há modo standalone.
- Q: Em `local`, um único run detém as chaves das duas partes e completa tudo, ou só o ato do CB
  corrente? → A: **Só o ato do CB corrente** — cada `apply` executa apenas o ato soberano do CB do
  manifesto atual: CB-A **propõe** (`proposePair`), um `found-spoke` **separado** de CB-B **confirma**
  (`confirmPair`); cada CB faz `commit-liquidity` só do seu lado; entre propor e confirmar o par fica
  `PROPOSED`/`pending`. Nenhum run detém a chave da contraparte (soberania estrita, inclusive em local).

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Abrir o par soberano (open-sovereign-pair) (Priority: P1)

Como um banco central soberano (CB-A), ao fundar o meu spoke com um bloco `spec.pair`, quero **abrir um
corredor bilateral** com a contraparte (CB-B): implantar/reutilizar os W-tokens por moeda, a AMM
dedicada do par (com o seu circuit breaker), o `LiquidityCommitRegistry`, mapear `setCentralBankOf`,
conceder papel ao relayer, e **propor** o par no `PairRegistry` — de modo que o corredor exista e
avance para `ACTIVE` quando a contraparte confirmar.

**Why this priority**: sem o par aberto (`PROPOSED→ACTIVE`) não há AMM nem corredor; é a base do FX
soberano e o MVP desta feature.

**Independent Test**: dado um `found-spoke` com `spec.pair`, o step reutiliza `SeedNewSovereignPair`:
implanta/deduplica W-tokens por moeda, cria a AMM + registry, `setCentralBankOf`, grants ao relayer, e
executa `proposePair` (CB-A); o `Check()` lê `getPair(pairId).status` — `ACTIVE` pula, `PROPOSED`
aguarda, inexistente propõe (se eu for o CB de `tokenA`); a confirmação (`confirmPair`, CB-B)
transiciona para `ACTIVE`.

**Acceptance Scenarios**:

1. **Given** um `found-spoke` com `spec.pair` e eu sendo o CB de `tokenA`, **When** `open-sovereign-pair`
   roda, **Then** os W-tokens/AMM/registry existem (dedup por moeda), o relayer recebe papel, e
   `proposePair(pairId)` é emitido (`PairProposed`); o `pairId` é determinístico (ex.: `W-BRL-ARS`).
2. **Given** um par em `PROPOSED` e eu sendo o CB de `tokenB` (num `found-spoke` separado de CB-B),
   **When** `open-sovereign-pair` roda, **Then** `confirmPair(pairId)` transiciona `PROPOSED→ACTIVE`
   (`PairRegistered`); re-rodar com o par `ACTIVE` **pula** (idempotente). Enquanto só CB-A propôs, o
   step de CB-A fica `pending` (não falha) aguardando a confirmação de CB-B.

---

### User Story 2 — Liquidez cooperativa (commit-liquidity) (Priority: P1)

Como um CB soberano, quero **contribuir liquidez apenas com a minha própria moeda** por commit-reveal no
`LiquidityCommitRegistry`, de modo que a AMM do par receba liquidez de ambos os lados sem que nenhum CB
exponha ou custodie a moeda do outro; o relay casa os dois lados (`CommitMatched`).

**Why this priority**: sem liquidez a AMM não faz swaps; a contribuição cooperativa (cada CB só a sua
moeda) é o design de soberania central do Cenário B.

**Independent Test**: `commit-liquidity` registra o commit do CB corrente (lado da sua moeda) via
`registerCommit`; quando ambos os lados existem, o relay casa e emite `CommitMatched`; re-rodar não
duplica o commit (idempotente pelo estado do registry).

**Acceptance Scenarios**:

1. **Given** um par `ACTIVE`, **When** `commit-liquidity` roda para o CB-A, **Then** um commit do lado
   da moeda de CB-A é registrado (`registerCommit`); o CB-A **não** contribui com a moeda de CB-B.
2. **Given** commits de ambos os lados, **When** o relay observa, **Then** ele casa e emite
   `CommitMatched`; re-rodar `commit-liquidity` com o commit já pendente/casado **não** duplica.

---

### User Story 3 — Semear o oráculo (seed-oracle) (Priority: P2)

Como um CB soberano (em `local`), quero **semear a taxa inicial** do par recém-aberto no `ManualOracle`
do hub (`setRate`), de modo que os primeiros swaps do corredor tenham preço; sem taxa, os swaps falham.

**Why this priority**: destrava os swaps do corredor recém-aberto, mas é conveniência de bootstrap
(`local`), não parte do núcleo soberano; por isso P2 e soft.

**Independent Test**: `seed-oracle` chama `setRate(tokenA, tokenB, rate)` no `ManualOracle` com a taxa do
`spec.pair`; re-rodar é idempotente (mesma taxa → sem efeito material); apenas `local`.

**Acceptance Scenarios**:

1. **Given** um par `ACTIVE` em `local`, **When** `seed-oracle` roda, **Then** `setRate` do par é
   gravado no `ManualOracle`; `getRate` passa a devolver a taxa.
2. **Given** ambiente **não**-`local`, **When** o `found-spoke` roda, **Then** `seed-oracle` é **pulado**
   (é conveniência local-only).

---

### Edge Cases

- `found-spoke` **sem** `spec.pair` → os três steps são **pulados** (a cauda soberana só roda com par).
- Qualquer dos três steps falha → **soft**: o `found-spoke` **não** é bloqueado; o report marca
  não-fatal (e o spoke bundle é emitido mesmo assim).
- Par já `ACTIVE` → `open-sovereign-pair` pula (idempotência on-chain); `PROPOSED` aguardando a
  contraparte → `pending` (não falha).
- CB-A propôs mas CB-B ainda não confirmou (ou falta aprovação de governança fora de `local`) → o ato
  soberano fica `pending` (nota de governança/contraparte), sem falhar o step. Nenhum run assina pela
  contraparte (soberania estrita, inclusive em `local`).
- W-token da moeda já implantado (outro corredor) → **reutiliza** (dedup por moeda), não redeploya.
- `commit-liquidity` sem o relay no ar → o commit é registrado, mas o casamento (`CommitMatched`) só
  ocorre quando o relay observar; re-rodar não duplica.
- `seed-oracle` em ambiente não-`local` → pulado.
- `--dry-run` → nenhum efeito externo; report do plano.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A cauda soberana (`open-sovereign-pair`, `commit-liquidity`, `seed-oracle`) MUST rodar
  **apenas** quando o manifesto `found-spoke` carrega `spec.pair`; ausente → os três steps são pulados.
- **FR-002**: Os três steps MUST ser **soft** (não-fatais): a falha de qualquer um **não** bloqueia o
  `found-spoke` nem a emissão do spoke bundle; o report marca não-fatal.
- **FR-003**: `open-sovereign-pair` MUST implantar/**reutilizar por moeda** os W-tokens, criar a AMM
  dedicada do par (com o seu circuit breaker) e o `LiquidityCommitRegistry` (reutilizando se já existir),
  executar `setCentralBankOf` (cada W-token → seu CB) e conceder `CENTRAL_BANK_ROLE` ao relayer nos
  tokens soberanos — reusando `SeedNewSovereignPair.s.sol`/`contracts.seed-sovereign-pair`.
- **FR-004**: `open-sovereign-pair` MUST executar `proposePair(pairId, tokenA, tokenB, amm)` quando eu for
  o CB de `tokenA` e o par não existir; o `pairId` é **determinístico** (ex.: `W-<A>-<B>`).
- **FR-005**: `open-sovereign-pair` MUST executar `confirmPair(pairId)` (transição `PROPOSED→ACTIVE`)
  quando eu for o CB de `tokenB` e o par estiver `PROPOSED`. Cada `apply` executa **apenas** o ato
  soberano do CB do manifesto atual — nenhum run detém a chave da contraparte (inclusive em `local`);
  a confirmação vem de um `found-spoke` **separado** de CB-B.
- **FR-006**: `open-sovereign-pair` MUST ser **idempotente pelo estado on-chain**: `Check()` lê
  `getPair(pairId).status` — `ACTIVE` → pula; `PROPOSED` → aguarda a contraparte (`pending`, não falha);
  inexistente → propõe (se eu for o CB de `tokenA`).
- **FR-007**: `commit-liquidity` MUST registrar a contribuição do CB corrente **apenas com a própria
  moeda** via `registerCommit` no `LiquidityCommitRegistry`; nenhum CB contribui/custodia a moeda da
  contraparte.
- **FR-008**: `commit-liquidity` MUST ser idempotente (não duplica um commit já pendente/casado); o
  casamento (`CommitMatched`) é feito pelo **relay** ao observar ambos os lados.
- **FR-009**: `seed-oracle` MUST executar `setRate(tokenA, tokenB, rate)` no `ManualOracle` do hub com a
  taxa do `spec.pair`, **apenas** em `environment: local`; fora de `local` é **pulado**; idempotente
  (mesma taxa não gera efeito material).
- **FR-010**: Um ato soberano cuja pré-condição ainda não existe (ex.: CB-A propôs mas CB-B ainda não
  confirmou; ou, fora de `local`, falta aprovação de governança) MUST ficar `pending` (nota de
  governança/contraparte) **sem** falhar o step — mesmo padrão request→approve→gated do onboarding.
  Cada CB só assina o próprio ato; nenhum run atua pela contraparte.
- **FR-011**: Todos os efeitos externos MUST passar pela fronteira injetável (executor / seams), de modo
  que os steps sejam **testáveis sem** Docker/Foundry/Besu reais e que `--dry-run` não os acione.
- **FR-012**: A cauda soberana MUST usar o mesmo motor/estado/lock/report do TK-B6/B7 (Step soft, Check
  on-chain); erros tipados, nunca silenciosos; **não** alterar os contratos nem os Makefiles/`deploy/
  local`; **não** importar `scenario-a/`.
- **FR-013**: A feature MUST incluir uma **suíte E2E** (build tag `e2e`) que abre um par entre dois
  spokes fundados, contribui liquidez de ambos os lados (casada pelo relay) e semeia o oráculo; **skip
  com aviso** quando o ambiente E2E está ausente (nunca falso verde).

### Key Entities

- **`spec.pair` (manifesto found-spoke, opcional)**: `proposerCB`, `confirmerCB`, `symbolA`, `symbolB` —
  já presente no modelo de manifesto; dispara a cauda soberana quando presente.
- **Par soberano (`PairRegistry`)**: `pairId` determinístico, `tokenA`/`tokenB` (W-tokens), `amm`,
  estado `PROPOSED→ACTIVE`; eventos `PairProposed`/`PairRegistered`.
- **W-token (`W-tCeBM` por moeda)**: token soberano por moeda, **reutilizável** entre corredores (dedup
  por moeda, não por par).
- **AMM do par (`AutomatedMarketMaker`)**: pool dedicado do corredor com circuit breaker embutido (pause
  1-de-N, resume quorum 2) — **sem** mudança de lógica do breaker (opção A).
- **`LiquidityCommitRegistry`**: commit-reveal cooperativo; `registerCommit` por lado; `CommitMatched`
  casado pelo relay.
- **`ManualOracle`**: taxa do par (`setRate`/`getRate`), semeado em `local`.
- **Step set (cauda soberana do found-spoke)**: `open-sovereign-pair`, `commit-liquidity`, `seed-oracle`
  — todos **soft**, após `register-relay-spoke`/`add-noc-agent` e antes de `emit-spoke-bundle`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Um `found-spoke` **sem** `spec.pair` **não** executa nenhum dos três steps (pulados).
- **SC-002**: Com `spec.pair`, o `found-spoke` de CB-A deixa o par `PROPOSED`; um `found-spoke` separado
  de CB-B o leva a `ACTIVE` (`confirmPair`); re-rodar com `ACTIVE` pula (idempotente). Entre os dois
  atos, o step de CB-A fica `pending` (não falha).
- **SC-003**: W-token por moeda é **reutilizado** entre corredores (não há redeploy da mesma moeda).
- **SC-004**: `commit-liquidity` registra a contribuição **só da moeda do CB corrente**; commits de
  ambos os lados resultam em `CommitMatched` (via relay); re-rodar não duplica.
- **SC-005**: `seed-oracle` grava a taxa no `ManualOracle` em `local` (getRate devolve a taxa) e é
  **pulado** fora de `local`.
- **SC-006**: A falha de qualquer dos três steps **não** bloqueia o `found-spoke` (soft) e o spoke
  bundle ainda é emitido.
- **SC-007**: Um ato soberano cuja pré-condição/contraparte ainda não existe fica `pending` (não falha);
  nenhum run assina pela contraparte.
- **SC-008** (E2E): num ambiente com dois spokes fundados + relay no ar, o `found-spoke` de CB-A propõe,
  o `found-spoke` de CB-B confirma (par → `ACTIVE`), cada CB contribui liquidez do seu lado (casada pelo
  relay) e o oráculo é semeado — de ponta a ponta; ambiente ausente ⇒ **skip com aviso**.

## Assumptions

- **Padrão de execução (consistente com TK-B4..B8): E2E completo** com executor/seams injetáveis —
  steps reais, testes de unidade com fakes, suíte E2E sob build tag `e2e` (skip-com-aviso).
- **Placement (resolvido, Q1)**: os três steps são a **cauda soft do `found-spoke`**, disparada pelo
  bloco opcional `spec.pair` (roadmap §404-407). O TK-B9 **estende o `found-spoke` do TK-B7** (toque
  cross-feature previsto: a cauda foi deferida de B7 para B9); **não** há modo/comando standalone.
- **Soberania estrita (resolvido, Q2)**: cada `apply` executa **apenas** o ato soberano do CB do
  manifesto atual — CB-A propõe, um `found-spoke` **separado** de CB-B confirma; cada CB faz
  `commit-liquidity` só do seu lado. **Nenhum run detém a chave da contraparte**, inclusive em `local`;
  o par fica `PROPOSED`/`pending` entre propor e confirmar. Fechar um corredor em `local` exige **dois**
  `found-spoke` (CB-A e CB-B) + o relay para casar os commits. Produção (KMS/CA reais, gates de
  governança vivos) permanece **deferida**.
- **corridor-request**: no caminho do toolkit, a proposta é dirigida **on-chain** (via
  `SeedNewSovereignPair`/`proposePair`, análogo ao `register-cb`); a variante "API no gateway do hub"
  (roadmap §446) é a UX de produção, fora do escopo desta fase.
- **Reuso**: `SeedNewSovereignPair.s.sol` + `contracts.seed-sovereign-pair` (scaffolding + propose +
  confirm), `PairRegistry`/`LiquidityCommitRegistry`/`AutomatedMarketMaker`/`ManualOracle` (já em
  `contracts/src`), o motor/estado/lock/report/executor (TK-B6), o `RelayRegistrar` (TK-B5, para o
  casamento de commits observado pelo relay) e o padrão de Step soft (TK-B7).
- **Circuit breaker**: **opção A** — o toolkit não altera a lógica do breaker; só garante o conjunto de
  governança correto e a validação de `isPaused` pelo relay (já coberto). Opção C (breaker por par) é
  mudança de contrato **futura**, fora de escopo.
- **W-token por moeda, dedup**: o `SeedNewSovereignPair` hoje deploya por par; o toolkit **deduplica por
  moeda** (reutiliza o W-token da moeda entre corredores).
- Não alterar contratos, Makefiles nem `deploy/local`; não importar `scenario-a/` (Princípio I). Sem
  novas dependências Go.
