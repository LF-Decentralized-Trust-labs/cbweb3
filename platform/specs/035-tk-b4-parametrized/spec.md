# Feature Specification: Toolkit do Cenário B — Templates de compose parametrizados (TK-B4)

**Feature Branch**: `035-tk-b4-parametrized`
**Created**: 2026-07-10
**Status**: Draft
**Input**: User description: "TK-B4 — templates de compose parametrizados (hub + entity-* + relay + NOC),
conforme orientação de `scenario-b/docs/design/scenario-b-toolkit-roadmap.md`."

> Conjunto de **templates de compose net-new** sob `scenario-b/provisioning/`, derivados dos
> compose atuais de `deploy/local`, com todos os valores discriminantes externados por variável
> de ambiente, esquema de porta determinístico por entidade, alcance cross-stack e persistência
> em **named volumes determinísticos**. Os compose de `deploy/local` **não são movidos nem
> editados** (caminho legado preservado). Fases seguintes (motor de orquestração, TK-B6+)
> consomem estes templates; TK-B4 não inclui renderização em runtime, subida de containers, nem
> os steps do motor.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Templates parametrizados de hub e entidade (Priority: P1)

Como o toolkit, preciso de **templates de compose para o hub e para cada camada de entidade**
(infra, keycloak, backend, frontend) em que **todo valor discriminante** (nome de container, rede,
portas, chain id, imagens, nomes de volume) é fornecido por variável de ambiente — de modo que uma
mesma família de templates suba N spokes/bancos sem editar arquivo algum e sem tocar os compose de
`deploy/local`.

**Why this priority**: é a fundação do provisionamento — sem templates parametrizados o motor
(TK-B6+) não tem o que renderizar; é o núcleo do TK-B4 e o MVP.

**Independent Test**: para cada template (hub, entity-infra, entity-keycloak, entity-backend,
entity-frontend), fornecer um conjunto de variáveis de exemplo e validar que o compose **interpola
por completo** e passa a verificação sintática/semântica, **sem subir containers** e sem sobra de
placeholders.

**Acceptance Scenarios**:

1. **Given** o template do hub e um conjunto de variáveis de exemplo, **When** o compose é
   validado, **Then** todos os `${VAR}` são resolvidos e o resultado é um compose válido.
2. **Given** um template de entidade e um conjunto de variáveis com **uma variável obrigatória
   ausente**, **When** o compose é validado, **Then** a validação **falha explicitamente**
   (nenhum default silencioso para valor discriminante).
3. **Given** os templates parametrizados, **When** comparo o diretório `deploy/local`, **Then**
   nenhum arquivo de `deploy/local` foi movido ou modificado.

---

### User Story 2 — Persistência em named volumes determinísticos (Priority: P1)

Como o toolkit, preciso que **todo o estado de participante** (dados do nó Besu, genesis, config,
TLS, bancos de dados) seja persistido em **named volumes do Docker nomeados deterministicamente
por spoke/entidade**, substituindo os bind mounts de host usados hoje — com **uma única exceção**:
o diretório `pki/` do banco (chave + CSR), que precisa ser acessível ao host para o fluxo de
onboarding/KYC.

**Why this priority**: a estratégia de named volumes (do toolkit de referência) é o que torna o
provisionamento portátil e o hand-off por bundle possível (o emissor lê genesis/enode de volumes);
é requisito explícito do roadmap §7 e tão fundamental quanto US1.

**Independent Test**: inspecionar cada template e confirmar que o estado de nó/participante é
declarado como **named volume determinístico** (nome derivado de spoke/entidade), que **não há bind
mount de host** para estado de nó, e que a única exceção presente é o `pki/` do banco. O estado de
nó (`besu_data`/`genesis`/`config`/`tls`/`paladin_data`) vive no template **`entity-besu`**.

**Acceptance Scenarios**:

1. **Given** os templates, **When** inspeciono os volumes declarados, **Then** o estado de nó
   (Besu data, genesis, config, TLS) e os bancos de dados usam named volumes com nome
   determinístico por spoke/entidade.
2. **Given** os templates, **When** procuro bind mounts de host, **Then** o único presente é o
   `pki/` do banco; nenhum outro caminho do host é montado para estado.

---

### User Story 3 — Endereçamento determinístico e alcance cross-stack (Priority: P2)

Como o toolkit, preciso de um **esquema de porta determinístico por entidade** (offset por
entidade) e de **alcance cross-stack** (backends de uma entidade alcançam o hub em outro stack),
de modo que **N entidades** coexistam no mesmo host **sem colisão** de portas, nomes de container,
redes ou volumes.

**Why this priority**: habilita multi-entidade no mesmo host (o caso real de N spokes/bancos), mas
depende dos templates de US1/US2 já existirem; é refinamento sobre a fundação.

**Independent Test**: instanciar (validar) dois conjuntos de variáveis para **duas entidades
distintas** e confirmar **zero colisões** de porta, nome de container, rede e volume; e confirmar
que a rota cross-stack (backend → hub) está expressa de forma parametrizada.

**Acceptance Scenarios**:

1. **Given** variáveis para duas entidades distintas, **When** ambos os composes são validados,
   **Then** não há colisão de porta, nome de container, rede ou volume entre elas.
2. **Given** o template de backend de uma entidade, **When** inspeciono o alcance ao hub, **Then**
   a rota cross-stack é parametrizada (host-gateway + porta do hub), sem endereço fixo embutido.

---

### User Story 4 — Templates de relay e NOC (Priority: P2)

Como o toolkit, preciso de **templates parametrizados para o relay** e para o **NOC** (stack do
hub — db, backend, portal — e o `noc-agent` parametrizável por entidade), completando o conjunto
`hub + entity-* + relay + NOC` pedido.

**Why this priority**: relay e NOC integram a topologia operacional pedida, mas o hub e as
entidades (US1/US2) são pré-requisito para que relay/NOC tenham a que se conectar.

**Independent Test**: validar o template do relay e o do NOC (incluindo `noc-agent` para hub e para
um spoke) com variáveis de exemplo; confirmar interpolação completa e ausência de identificadores
de entidade fixos (spoke-a/spoke-b hardcoded).

**Acceptance Scenarios**:

1. **Given** o template do relay e variáveis de exemplo, **When** é validado, **Then** interpola
   por completo e não contém identificadores de spoke fixos.
2. **Given** o template do NOC, **When** é validado com variáveis para um `noc-agent` de um spoke
   nomeado dinamicamente, **Then** o agente é parametrizado por entidade (sem `spoke-a`/`spoke-b`
   embutidos).

### Edge Cases

- Variável obrigatória ausente → falha explícita na validação (sem default silencioso para valor
  discriminante).
- Duas entidades com o mesmo offset/índice → colisão detectável na validação (não é um estado
  válido de operação).
- Um template que tente persistir estado de nó em bind mount de host → viola FR-004 (rejeitado na
  revisão/validação).
- Segredo (chave/cert/senha) embutido no template → viola FR-009.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O toolkit MUST fornecer templates de compose para **hub**, **entity-besu**,
  **entity-infra**, **entity-keycloak**, **entity-backend**, **entity-frontend**, **relay** e
  **NOC** (stack do hub db/backend/portal + `noc-agent` parametrizável por entidade), versionados
  sob `scenario-b/provisioning/` (fora de `deploy/local`). O **`entity-besu`** é o nó Besu (+Paladin)
  do spoke/banco — o lar do estado `besu_data`/`genesis`/`config`/`tls`/`paladin_data`; é consumido
  em runtime pelas fases de fundação/join (TK-B7/B8), mas o **template** é entregue aqui.
- **FR-002**: Todo **valor discriminante** (nome de container, rede, portas, chain id, imagens,
  nomes de volume) MUST ser externado por **variável de ambiente**; a ausência de uma variável
  obrigatória MUST causar **falha explícita** na validação (sem default silencioso para valores
  discriminantes; defaults só para itens não discriminantes, como tags de imagem).
- **FR-003**: Os arquivos de compose de `deploy/local` MUST **não** ser movidos nem editados — o
  caminho legado de Makefile permanece intacto.
- **FR-004**: O estado de participante (dados do nó Besu, genesis, config, TLS, bancos de dados)
  MUST ser persistido em **named volumes determinísticos por spoke/entidade** — sem bind mounts de
  host para estado de nó.
- **FR-005**: A **única exceção** de bind mount permitida MUST ser o diretório `pki/` do banco
  (chave + CSR), acessível ao host para o fluxo de onboarding/KYC.
- **FR-006**: As portas MUST seguir um **esquema determinístico por offset por entidade**,
  **documentado** no contrato de variáveis (a convenção que o operador/motor aplica ao preencher as
  variáveis), de modo que N entidades coexistam no mesmo host sem colisão de porta. O **cálculo** do
  offset/nomes em código não faz parte do TK-B4 (fica no motor, TK-B6).
- **FR-007**: O alcance **cross-stack** (backend de uma entidade → hub em outro stack) MUST ser
  expresso de forma parametrizada (host-gateway + porta do hub), sem endereço fixo embutido.
- **FR-008**: Cada template MUST ser **validável sem subir containers** — dado um conjunto de
  variáveis de exemplo, a validação verifica interpolação completa e a sintaxe/semântica do
  compose.
- **FR-009**: Nenhum **segredo** (chave privada, certificado, senha) MUST ser embutido nos
  templates; material sensível chega em runtime via volumes/variáveis, nunca no template.
- **FR-010**: Nomes de container, redes e volumes MUST ser derivados do identificador de
  entidade/spoke (sem `spoke-a`/`spoke-b` ou nomes fixos embutidos), suportando N entidades
  dinâmicas.
- **FR-011**: Cada template MUST ter um **contrato de variáveis** explícito (o conjunto de
  variáveis obrigatórias e opcionais que ele exige, e a convenção determinística de portas/nomes),
  consultável de forma independente do motor.

### Key Entities

- **Compose Template**: um arquivo de composição parametrizado (hub, entity-*, relay, NOC) sob
  `provisioning/`; declara serviços, redes e volumes com valores discriminantes externados.
- **Contrato de variáveis**: o conjunto nomeado de variáveis (obrigatórias/opcionais) que um
  template exige para interpolar por completo.
- **Esquema de nomes/portas por entidade**: a convenção determinística que deriva portas
  (por offset), nomes de container, redes e nomes de volume a partir do identificador de entidade.
- **Conjunto de named volumes**: os volumes determinísticos por spoke/entidade que persistem o
  estado (`besu_data`, `genesis`, `config`, `tls`, DB, …).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Cada um dos **8 templates** (hub, entity-besu, entity-infra, entity-keycloak,
  entity-backend, entity-frontend, relay, NOC) valida com um conjunto de variáveis de exemplo,
  **100% interpolado** (nenhum `${...}` residual), sem subir containers.
- **SC-002**: Dois conjuntos de variáveis para **entidades distintas** produzem **zero colisões**
  de porta, nome de container, rede e volume.
- **SC-003**: Nenhum template contém bind mount de host, **exceto** o `pki/` do banco (verificável).
- **SC-004**: **Nenhum** arquivo de `deploy/local` é modificado (diff vazio).
- **SC-005**: A ausência de uma variável **obrigatória** causa **erro explícito** na validação
  (nenhum default silencioso para valor discriminante).
- **SC-006**: **Nenhum** segredo/valor sensível está embutido nos templates (verificável).

## Assumptions

- **Escopo do TK-B4** = os *assets* de template (arquivos de compose parametrizados sob
  `provisioning/`) + o **contrato de variáveis** de cada template + a **validação** desses
  templates. A renderização em runtime, a subida de containers, a semeadura/leitura de volumes e os
  steps do motor ficam para fases seguintes (TK-B6+). **Decisão (2026-07-10):** o TK-B4 **não**
  inclui utilitário de derivação em Go — o esquema determinístico de portas/nomes/volumes é
  **documentado no contrato de variáveis** (a convenção que o operador/motor segue ao preencher as
  variáveis); a **derivação em código** (cálculo de offset/nomes por entidade) nasce no motor
  (TK-B6), junto de quem a consome.
- `entity-*` = as camadas de entidade: **besu** (nó Besu + Paladin do spoke/banco), **infra**
  (Postgres/Redis), **keycloak**, **backend**, **frontend**. As quatro últimas derivam do roadmap
  §7; o **besu** foi acrescentado ao resolver o achado C1 (2026-07-10) — é o lar dos volumes de
  estado do nó, que US2/§7 já pressupõem. NOC entra como template adicional (stack do hub +
  `noc-agent`).
- Os templates são **derivados** dos compose atuais de `deploy/local` (`compose.yml`,
  `compose.noc.yml` e os composes de Besu do hub/spoke), preservando o comportamento, apenas
  externando os valores discriminantes e trocando bind mounts por named volumes.
- A validação sem execução usa a interpolação/verificação nativa de compose (parse + resolução de
  variáveis), não a subida de serviços.
- Constituição: isolamento de cenário (nada de `scenario-a/`), sem segredos, `tCeBM` apenas na
  camada de reserva; observabilidade e atomicidade não são exercidas nesta fase (só assets de
  provisionamento).
