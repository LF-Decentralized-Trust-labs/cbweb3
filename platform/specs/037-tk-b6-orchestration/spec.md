# Feature Specification: Toolkit do Cenário B — Motor de orquestração + found-hub + hub bundle + apply (TK-B6)

**Feature Branch**: `037-tk-b6-orchestration`
**Created**: 2026-07-10
**Status**: Draft
**Input**: User description: "TK-B6 — motor de orquestração + steps found-hub (start-relay, start-noc,
Keycloak write-back) + hub bundle emitter + comando apply, conforme
`scenario-b/docs/design/scenario-b-toolkit-roadmap.md` (§3, §5, found-hub, §11)."

> O primeiro modo executável do toolkit: um **motor de steps idempotente** (Check→skip/Run, estado
> persistido, lock, dry-run, report), o **inventário de steps do `found-hub`** (subir o nó do hub,
> deployar os contratos do hub na ordem de dependência, provisionar Keycloak com **write-back** de
> secrets, renderizar env, subir infra/backend/frontend, subir **relay** e **NOC** neutros), a
> **emissão do hub bundle** (endereços de contrato + config para os spokes consumirem) e o **comando
> `apply`** (CLI sobre o motor: `apply -f <manifest> [--dry-run] [-o json|yaml]`). Consome as
> interfaces já entregues (KeyProvider, CertSource, RelayRegistrar) e os templates do TK-B4.

## Clarifications

### Session 2026-07-10

- Resolução A1 (deploy dos contratos do hub): o step `deploy-hub-contracts` é **um** `forge script
  CBWeb3Hub.s.sol:DeployCBWeb3Hub` — a ordem IdentityRegistry→tCeBM_BRL→tCeBM_EUR→FXAgreement→
  PairRegistry→CurrencyRegistry→ManualOracle é **interna ao script Solidity**, não orquestrada
  step-a-step pelo toolkit. SC-005 é satisfeito verificando a **invocação do script** + a presença
  dos **7 endereços** no broadcast JSON.
- Resolução I1 (semântica do `apply`): o TK-B6 **muda** o comportamento do `apply` do TK-B1 — `apply`
  sem `--dry-run` passa a **executar o motor** (found-hub); `apply --dry-run` passa a **planejar o
  motor**. O `cmd/cbweb3b/main_test.go` do TK-B1 (que esperava `exitInvalid` para `apply` sem
  dry-run) MUST ser **atualizado** para a nova semântica (evitar regressão).

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Motor de steps idempotente (Priority: P1)

Como operador, quero um motor que execute uma sequência de steps de forma **idempotente** — cada
step tem um `Check` (já satisfeito? pula) e um `Run` (executa e persiste), com **estado durável**
por step e **lock** para não haver duas execuções concorrentes — de modo que re-rodar o `apply`
**converge** sem repetir efeitos nem corromper estado.

**Why this priority**: é a fundação de todos os modos; sem o motor idempotente, nenhum step do
`found-hub` pode ser executado com segurança. É o MVP.

**Independent Test**: dado um conjunto de steps com `Check`/`Run` (fakes), rodar o motor: steps cujo
`Check` já passa são pulados; os demais rodam em ordem de dependência; o estado é persistido por
step; uma segunda execução pula tudo (idempotente); um lock impede execução concorrente.

**Acceptance Scenarios**:

1. **Given** uma sequência de steps com dependências, **When** o motor roda, **Then** cada step roda
   **após** os seus pré-requisitos e o resultado é persistido por step.
2. **Given** um estado com steps já concluídos, **When** o motor roda de novo, **Then** os steps com
   `Check` satisfeito são **pulados** (idempotência) e nenhum efeito é repetido.
3. **Given** uma execução em andamento (lock ativo), **When** uma segunda execução começa, **Then**
   ela é **recusada** com erro claro (não corrompe o estado).
4. **Given** um step que falha, **When** o motor para, **Then** o estado reflete o step falho e uma
   nova execução **retoma** a partir dele (não re-executa os concluídos).

---

### User Story 2 — Comando `apply` com dry-run e report (Priority: P1)

Como operador, quero um comando `apply -f <manifest> [--dry-run] [-o json|yaml]` que leia o
manifesto, **despache pelo modo** (`found-hub`), e **sempre emita um report** do que foi/será feito;
com `--dry-run` ele **planeja sem executar efeitos**.

**Why this priority**: é a interface pela qual o operador usa o motor; o `--dry-run` é a rede de
segurança que torna o motor utilizável sem medo. Junto de US1, forma o MVP utilizável.

**Independent Test**: `apply -f <found-hub-manifest> --dry-run -o json` produz um report listando os
steps planejados (com o que cada um faria) **sem executar** nenhum efeito externo; `-o yaml` muda só
o formato; um manifesto inválido é rejeitado com erro claro antes de qualquer efeito.

**Acceptance Scenarios**:

1. **Given** um manifesto `found-hub` válido, **When** `apply --dry-run`, **Then** o report lista os
   steps na ordem e o estado planejado, **sem** efeitos externos.
2. **Given** `-o json` vs `-o yaml`, **When** `apply` roda, **Then** o report sai no formato pedido.
3. **Given** um manifesto inválido ou modo não suportado, **When** `apply`, **Then** falha com erro
   claro **antes** de qualquer efeito.
4. **Given** uma interrupção (sinal) durante o `apply`, **When** o processo recebe o sinal, **Then**
   um **report parcial** é emitido (o que já rodou).

---

### User Story 3 — Steps do `found-hub` (Priority: P1)

Como operador do hub, quero que o `apply` no modo `found-hub` execute, em ordem de dependência: subir
o **nó validador do hub**, **deployar os contratos do hub** (IdentityRegistry → tCeBM_BRL →
tCeBM_EUR → FXAgreement → PairRegistry → CurrencyRegistry → ManualOracle), **provisionar o Keycloak**
com **write-back** dos secrets nos env, **renderizar o env do hub**, subir **infra/backend/frontend**,
subir o **relay** e o **NOC** neutros — cada step com gate de prontidão (Besu RPC no ar, Keycloak
pronto) e idempotente.

**Why this priority**: é o objetivo funcional do TK-B6 — fundar o hub de ponta a ponta pelo toolkit,
sem os Makefiles de deploy.

**Independent Test**: com os efeitos externos mediados por um executor injetável, rodar `found-hub`
verifica que os steps são montados na ordem de dependência correta, que os gates de prontidão são
consultados antes de cada step dependente, que o write-back grava os secrets no destino esperado, e
que re-rodar pula os steps já satisfeitos.

**Acceptance Scenarios**:

1. **Given** o modo `found-hub`, **When** o motor roda, **Then** os contratos do hub são deployados
   **na ordem de dependência** declarada (IdentityRegistry primeiro; ManualOracle por último).
2. **Given** o step de contratos, **When** ele inicia, **Then** o **Besu RPC do hub** é aguardado no
   ar (gate) antes do deploy.
3. **Given** o step de Keycloak, **When** ele conclui, **Then** os **client secrets** são gravados
   (write-back) nos arquivos de env de destino, e o step é idempotente (não regrava se já feito).
4. **Given** o relay e o NOC, **When** subidos, **Then** são **neutros** (sem spokes fixos) e o hub
   fica pronto para spokes se registrarem depois.

---

### User Story 4 — Emissão do hub bundle (Priority: P1)

Como operador do hub, quero que, ao final do `found-hub`, o toolkit **emita o hub bundle** — um
artefato versionado com os **endereços dos contratos do hub** e a **config** que cada spoke precisa
para consumir o hub (RPC-only) — de modo que o `found-spoke` (TK-B7) o consuma sem hardcode.

**Why this priority**: o bundle é o hand-off entre `found-hub` e `found-spoke`; sem ele, os spokes
não aprendem os endereços do hub.

**Independent Test**: dado o estado final do `found-hub` (endereços de contrato + config), emitir o
hub bundle e **recarregá-lo/validá-lo**: contém os endereços esperados, é versionado, e **não contém
segredos** (só dados públicos de hub — endereços, RPC, chainId).

**Acceptance Scenarios**:

1. **Given** o hub fundado (endereços conhecidos), **When** o bundle é emitido, **Then** ele contém
   os endereços dos contratos do hub e a config de consumo (RPC/chainId), versionado.
2. **Given** um hub bundle emitido, **When** é recarregado/validado, **Then** passa a validação e os
   dados batem com o emitido (round-trip).
3. **Given** o bundle, **When** inspecionado, **Then** **nenhum segredo** (chave privada) está
   presente — apenas dados públicos.

### Edge Cases

- Re-rodar `apply found-hub` após sucesso → todos os steps pulados (idempotente), bundle reemitido de
  forma estável.
- `apply` interrompido no meio → estado persistido; nova execução retoma do step pendente.
- Lock órfão (execução anterior morta) → detectável; o motor não trava indefinidamente (política de
  lock clara).
- Gate de prontidão nunca satisfeito (Besu/Keycloak não sobe) → step falha com erro claro (sem loop
  infinito silencioso).
- `--dry-run` → nenhum efeito externo; report do plano.
- Manifesto de modo ≠ `found-hub` → nesta fase, erro "modo não suportado ainda" (found-spoke/join são
  TK-B7/B8).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O motor MUST executar steps com **`Check`** (idempotência: se satisfeito, pula) e
  **`Run`** (executa + persiste), respeitando **dependências** entre steps (ordem topológica).
- **FR-002**: O motor MUST persistir **estado durável por step** (concluído/falho) e **retomar** de
  onde parou em uma nova execução, sem re-executar steps concluídos.
- **FR-003**: O motor MUST usar um **lock** que impeça duas execuções concorrentes sobre o mesmo
  estado; um lock órfão MUST ser tratado por uma política clara (não travar para sempre).
- **FR-004**: O comando MUST oferecer `apply -f <manifest> [--dry-run] [-o json|yaml]`, despachando
  pelo **modo** do manifesto (`found-hub` nesta fase) e **sempre** emitindo um report em stdout.
- **FR-005**: `--dry-run` MUST **planejar sem executar efeitos externos** (nenhum container, deploy
  ou escrita fora do report).
- **FR-006**: Uma **interrupção (sinal)** durante o `apply` MUST resultar em **report parcial** do
  que já rodou (não deixar o operador sem informação).
- **FR-007**: O modo `found-hub` MUST montar os steps na ordem: subir **nó validador do hub** →
  **deploy dos contratos do hub** → **provision-keycloak** (+ write-back) → **render-env** →
  **infra/backend/frontend** → **relay** → **NOC** → **emit hub bundle**. O step de deploy é **um**
  `forge script CBWeb3Hub.s.sol:DeployCBWeb3Hub`, cuja ordem interna (IdentityRegistry → tCeBM_BRL →
  tCeBM_EUR → FXAgreement → PairRegistry → CurrencyRegistry → ManualOracle) é **garantida pelo
  Solidity** — o toolkit invoca o script e valida os endereços resultantes (ver Resolução A1).
- **FR-008**: Cada step dependente MUST validar um **gate de prontidão** do pré-requisito antes de
  executar (ex.: Besu RPC no ar antes do deploy de contratos; Keycloak pronto antes de provisionar
  realms).
- **FR-009**: O step de Keycloak MUST fazer **write-back** dos client secrets para os arquivos de env
  de destino, de forma idempotente (não regravar se já presentes/iguais).
- **FR-010**: O relay e o NOC subidos no `found-hub` MUST ser **neutros** (sem spokes fixos),
  coerentes com o TK-B5.
- **FR-011**: Ao final do `found-hub`, o toolkit MUST **emitir o hub bundle** — artefato versionado
  com os endereços dos contratos do hub + config de consumo (RPC/chainId) — carregável e validável.
- **FR-012**: O hub bundle MUST **não conter segredos** (nenhuma chave privada); apenas dados
  públicos do hub.
- **FR-013**: Os efeitos externos (containers, deploy de contratos, chamadas ao Keycloak) MUST ser
  mediados por uma **fronteira injetável** (executor), de modo que o motor e os steps sejam
  **testáveis sem** subir Docker/Foundry/Keycloak reais e que o `--dry-run` não os acione.
- **FR-014**: Erros MUST ser tipados/claros e **nunca silenciosos** (Princípio VI); o report MUST
  distinguir step concluído, pulado, falho e planejado (dry-run).
- **FR-015**: A feature MUST incluir uma **suíte E2E** que executa o `found-hub` **de verdade**
  (Docker + Foundry + Besu) e valida o hub bundle contra a chain viva; onde o ambiente E2E está
  ausente, a suíte MUST ser **pulada com aviso explícito** (nunca falha silenciosa nem falso verde).

### Key Entities

- **Step**: unidade de trabalho com `Name`, `Check` (idempotência), `Run` (efeito), e dependências.
- **Motor (orchestrator)**: executa o grafo de steps em ordem, aplica `Check`/`Run`, persiste estado,
  segura o lock, suporta dry-run.
- **Estado de provisionamento**: representação durável do progresso por step (para retomar/idempotir).
- **Report**: saída estruturada (json/yaml) com o status de cada step (concluído/pulado/falho/planejado).
- **Manifesto (found-hub)**: entrada declarativa que descreve o hub a fundar (consumido pelo modo).
- **Hub bundle**: artefato versionado com endereços de contrato do hub + config de consumo (público).
- **Executor**: fronteira injetável para efeitos externos (containers, deploy, Keycloak).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Rodar o motor duas vezes sobre o mesmo estado executa os steps **uma vez**; na segunda,
  **100%** dos steps satisfeitos são pulados (idempotência).
- **SC-002**: Uma segunda execução concorrente (lock ativo) é **recusada** sem corromper o estado.
- **SC-003**: Após uma falha em um step, a nova execução **retoma** a partir do step falho e **não**
  re-executa os concluídos.
- **SC-004**: `apply --dry-run` produz um report completo dos steps **sem** nenhum efeito externo
  (verificável: o executor não é acionado).
- **SC-005**: No modo `found-hub`, o step de contratos **invoca `CBWeb3Hub.s.sol:DeployCBWeb3Hub`**
  e, ao final, os **7 endereços** (IdentityRegistry, tCeBM_BRL, tCeBM_EUR, FXAgreement, PairRegistry,
  CurrencyRegistry, ManualOracle) estão presentes no broadcast e no bundle (a ordem é garantida pelo
  script Solidity).
- **SC-006**: O step de Keycloak grava os secrets no destino esperado e, re-rodado, **não** regrava
  (idempotente).
- **SC-007**: O hub bundle emitido **recarrega e valida** (round-trip) e **não contém** nenhum
  segredo (verificável).
- **SC-008**: Um manifesto inválido ou de modo não suportado é **rejeitado com erro claro antes** de
  qualquer efeito.
- **SC-009** (E2E): num ambiente com Docker + Foundry + Besu, `apply -f <found-hub>` **funda o hub de
  ponta a ponta** — sobe o nó validador, deploya os 7 contratos na ordem, provisiona o Keycloak — e o
  **hub bundle emitido** referencia endereços que **respondem na chain viva** (round-trip contra o
  RPC). Onde o ambiente E2E está ausente, a suíte é **pulada com aviso** (não falha silenciosa).

## Assumptions

- **Escopo = modo `found-hub`** (o primeiro modo executável) + o motor + o `apply` + o hub bundle.
  `found-spoke` e `join` são TK-B7/B8; um manifesto desses modos é rejeitado como "não suportado
  ainda" nesta fase.
- **Profundidade de execução dos steps (decisão 2026-07-10): E2E completo.** Os steps do `found-hub`
  **executam de verdade** os efeitos externos — subir o nó do hub via os templates do TK-B4, deployar
  os contratos Foundry, provisionar o Keycloak — e a feature inclui uma **suíte E2E** que **funda o
  hub de ponta a ponta com Docker + Foundry + Besu reais** e valida o hub bundle contra a chain viva.
  O executor injetável (FR-013) permanece para o `--dry-run` e para os testes de unidade do motor
  (ordem/idempotência/gates), mas a **aceitação final é a execução E2E real**.
- **Pré-requisitos de ambiente**: a suíte E2E requer **Docker Compose v2**, **Foundry (`forge`)** e a
  imagem **`hyperledger/besu:25.8.0`** disponíveis; onde ausentes, a suíte E2E é pulada com aviso
  explícito (não falha silenciosa), mas o `found-hub` real depende deles.
- O motor reusa o padrão do toolkit de referência (Step + estado YAML + flock + guard de genesis),
  reimplementado no módulo `scenario-b/toolkit` (Princípio I) — não importado.
- Em `local`, o toolkit usa as impls locais das interfaces (KeyProvider `kms://local-emulator`,
  CertSource `self-signed`, RelayRegistrar `local`/`relay://`) e os templates do TK-B4.
- Os artefatos Foundry (`contracts/out/`) são pré-buildados ou o toolkit os builda antes do deploy
  (gate de build).
- Não se alteram os Makefiles de deploy nem os compose de `deploy/local` (restrição rígida do
  roadmap); o toolkit provisiona por conta própria.
- Isolamento de cenário: mudanças restritas a `scenario-b/` (toolkit); nada de `scenario-a/`.
