# Feature Specification: Topologia nativa do toolkit — found CB-only e join dinâmico de Paladin/Pente

**Feature Branch**: `033-spoke-native-paladin-pente`
**Created**: 2026-06-29
**Status**: Draft
**Input**: Descrição do usuário: "O found deve criar a rede do país (CB) pronta a operar, e os bancos comerciais entram dinamicamente; o registro de bancos não pode ser hardcoded. Usar como referência os scripts dos spikes spk-01/spk-02, que implementam a mudança de arquitetura do concat.md."

## Contexto e problema-raiz *(motivação)*

A arquitetura-alvo (concat.md §2, §4, §5) é: **Scenario A = liquidação pairwise entre N spokes; cada spoke é fundado pelo seu banco central; bancos comerciais entram por configuração (`mode: join`), não por edição de código.** A premissa "2 spokes fixos / 2 bancos fixos" deve ser rejeitada.

Durante a validação em run real do `mode: found` (toolkit, branch `fix/toolkit`), descobriu-se que os passos de Paladin/Pente do motor **reutilizam scripts de referência que hardcodam a topologia `spoke-a`/`spoke-b`** e, portanto, **não suportam spokes arbitrários nem bancos dinâmicos**:

- `deploy/local/paladin/scripts/register_nodes_test.go` → `spokeNodes()` usa `switch spokeName()` com casos `spoke-b` (1 nó) e `default: // spoke-a` (3 nós fixos: `cb`+`bank-a`+`bank-c`, lendo certs de `../spoke-a/config/...`). Para qualquer spoke novo (ex.: `spoke-brl`), o código cai no `default` e registra os nós do `spoke-a` usando os certificados commitados do `spoke-a` — **silenciosamente incorreto**.
- `create_pente_context_test.go` ("Bilateral") e `deploy_fxagreement_pente_test.go` assumem 2 nós Paladin fixos no mesmo spoke.

Consequência: a estratégia do concat.md de "reusar os scripts existentes como building blocks" funciona para o **deploy de contratos de nível-spoke** (IdentityRegistry, ZetoFactory, PenteFactory) mas **quebra** em `register-nodes`, `create-pente-context` e `deploy-fxa-pente`. Suportar N spokes e bancos dinâmicos exige **lógica nativa do toolkit** parametrizada por spoke/banco para esses passos. Os spikes `spk-01` (enode cross-stack) e `spk-02` (live join: voto QBFT + registro dinâmico de nó Paladin) são a referência de design validada.

Decisão arquitetural já tomada (com o usuário): **`found` é CB-only**; o contexto **Pente + FXAgreement-in-Pente passam a ser criados no `mode: join`** (pairwise, dinâmico), quando o primeiro banco comercial entra.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — `found` cria um spoke CB-only pronto a operar (Priority: P1)

Um banco central executa `cbweb3 apply -f central-bank-brazil.yaml` (`mode: found`). Ao final, o spoke do país está no ar com **apenas** a infraestrutura do banco central: nó Besu (bootnode), nó Paladin do CB, contratos de nível-spoke deployados (IdentityRegistry, ZetoFactory, PenteFactory), token Zeto criado, identidade do CB onboarded no IdentityRegistry, e o spoke registrado no relay. O join bundle é emitido. **Nenhum nó `bank-a`/`bank-c` é criado.**

**Why this priority**: É o pré-requisito de tudo. Sem um `found` que funcione para um spoke arbitrário (sem hardcode de spoke-a), não é possível provisionar `spoke-brl`/`spoke-cop` corretamente. Hoje o `found` registra os nós do spoke-a por engano.

**Independent Test**: `cbweb3 apply` de um manifesto found com `spec.spoke.id` arbitrário (ex.: `spoke-brl`), em ambiente local com Docker; verificar que os nós/identidades registrados on-chain referem-se ao `spoke-brl` (não ao spoke-a) e que nenhum container `*-bank-a`/`*-bank-c` foi criado.

**Acceptance Scenarios**:

1. **Given** um manifesto found com `spec.spoke.id = spoke-brl`, **When** `cbweb3 apply` executa, **Then** o IdentityRegistry on-chain contém a identidade do nó Paladin do CB nomeada por `spoke-brl` (ex.: `spoke-brl-cb`), e nenhuma identidade `spoke-a-*` é registrada.
2. **Given** o found completou, **When** o operador lista os containers, **Then** existe exatamente um nó Paladin (CB) para o spoke e nenhum `bank-a`/`bank-c`.
3. **Given** o found completou, **When** o bundle é emitido, **Then** ele contém os endereços de nível-spoke (registry, zetoFactory, penteFactory, zetoToken) e **não** exige `PENTE_CONTEXT_*`/`FX_AGREEMENT_*` (criados no join).
4. **Given** dois manifestos found para spokes distintos (`spoke-brl`, `spoke-cop`) no mesmo host, **When** ambos são aplicados, **Then** cada um registra suas próprias identidades sem colisão e sem reaproveitar certs/identidades do outro.

---

### User Story 2 — Banco comercial entra com seu nó Paladin dinamicamente (`mode: join`) (Priority: P1)

Um banco comercial executa `cbweb3 apply -f bank-itau.yaml` (`mode: join`) consumindo o join bundle do spoke. Além do que o TK-9 já faz (Besu join + voto QBFT + CSR→CB→cert + IdentityRegistry), o fluxo agora **sobe o nó Paladin do banco dinamicamente**: gera o cert TLS do banco, renderiza a config do Paladin do banco, inicia o container e **registra a identidade do nó Paladin on-chain** — espelhando `spk-02/join-paladin.sh` e `register-paladin-nodes.sh`.

**Why this priority**: É o objetivo central da arquitetura ("adicionar bancos por configuração, dinamicamente"). Sem isso, o banco entra na rede Besu mas não tem capacidade de privacidade (Paladin) — não consegue participar de liquidação com privacidade.

**Independent Test**: Sobre um `found` CB-only, aplicar um manifesto join de banco; verificar que o nó Paladin do banco está no ar, registrado on-chain com nome derivado de `spec.bankId`, e que o nó Paladin do CB o descobre sem restart (ADR-002).

**Acceptance Scenarios**:

1. **Given** um spoke CB-only no ar e um join bundle válido, **When** `cbweb3 apply` de um manifesto `mode: join` com `spec.bankId = bank-itau` executa, **Then** existe um container Paladin do banco e sua identidade `bank-itau` consta no IdentityRegistry on-chain.
2. **Given** o cert do banco é gerado pelo fluxo de join, **When** o cert é inspecionado, **Then** seu CN/SAN deriva de `spec.bankId` (ex.: `paladin-spoke-brl-bank-itau`), gerado dinamicamente — não há nenhum nome de banco hardcoded no toolkit.
3. **Given** o nó Paladin do banco subiu, **When** os logs do Paladin do CB são inspecionados, **Then** não há erro de TLS e o novo par é descoberto reativamente (sem restart de nós existentes — ADR-002).

---

### User Story 3 — Contexto Pente + FXAgreement criados no relacionamento CB↔banco (Priority: P2)

Quando o primeiro banco comercial entra, o fluxo de join cria o **contexto Pente bilateral** (grupo de privacidade CB↔banco) e faz o **deploy do FXAgreement dentro desse contexto** — parametrizado pelos dois participantes reais, não por `bank-a` fixo.

**Why this priority**: Move a parte bilateral (que hoje quebra no found) para onde ela faz sentido na arquitetura pairwise. É necessária para liquidação real, mas pode vir logo após US1/US2 estarem verdes.

**Independent Test**: Após um join, verificar on-chain/via Paladin que existe um grupo Pente com exatamente os membros CB e o banco que entrou, e que o FXAgreement está deployado nesse grupo.

**Acceptance Scenarios**:

1. **Given** um banco que completou o join, **When** o contexto Pente é criado, **Then** o grupo de privacidade tem como membros o nó Paladin do CB e o nó Paladin do banco (derivados de `spec.bankId`), e nenhum membro fixo `bank-a`/`bank-c`.
2. **Given** o contexto Pente existe, **When** o FXAgreement é deployado, **Then** ele é instanciado dentro daquele grupo e seu endereço/grupo é persistido no estado de provisionamento do banco.

---

### User Story 4 — Idempotência e dry-run preservados (Priority: P2)

Re-executar `apply` (found ou join) retoma do ponto de falha; `-dry-run` lista os passos sem efeitos colaterais. Mantém a paridade de UX já estabelecida em TK-5/TK-7/TK-9.

**Why this priority**: Idempotência é requisito da constituição; não pode regredir com a mudança de topologia.

**Independent Test**: Injetar falha em um passo novo (ex.: register-paladin-node) e re-executar; verificar skip dos passos anteriores.

**Acceptance Scenarios**:

1. **Given** um join que falhou em `start-paladin-join`, **When** `cbweb3 apply` é re-executado, **Then** os passos anteriores são "skipped" e a execução retoma a partir de `start-paladin-join`.
2. **Given** um found CB-only completo, **When** `apply -dry-run` é executado, **Then** os passos do found são listados sem `create-pente`/`deploy-fxa` e sem efeitos colaterais.

### Edge Cases

- O que acontece se o `spec.spoke.id` não tem nenhuma config de nó nativa? → Não há mais fallback para `spoke-a`; o toolkit deriva a config do CB a partir de `spec.spoke.id` e `role`, parametricamente.
- O que acontece se dois bancos com o mesmo `bankId` tentam entrar no mesmo spoke? → O registro on-chain deve ser idempotente por nome; a segunda tentativa é tratada como "já registrado".
- O que acontece se o join tenta criar o contexto Pente mas o nó Paladin do CB não está acessível? → O passo falha com erro claro, sem estado parcial inconsistente.
- O que acontece com um spoke fundado antes desta mudança (bundle com `PENTE_CONTEXT_*`)? → O emitter/consumidor deve tolerar a ausência desses campos (não mais obrigatórios no bundle).
- O que acontece se o operador roda o `found` antigo (3 nós) e o novo (CB-only) no mesmo dataDir? → Fora de escopo; assume-se dataDir limpo por spoke.
- E se o `keyProvider` ainda não gerou a chave do participante quando o registro de nó Paladin precisa dela? → O passo gera a chave idempotentemente via `keyProvider.GenerateKey` (mesmo padrão de `request-cert`), nunca uma dev-key hardcoded.
- E se o contexto Pente já existe para o par CB↔banco (re-`apply`)? → `create-pente-context` é idempotente: detecta o grupo existente e não recria.

### Achado de implementação (US1, run real) — bloqueio do `onboard-registry`

Durante a implementação do US1 descobriu-se que o passo `onboard-registry` (TK-5) tem um bug **pré-existente** independente da topologia: ele chama `registerParticipant` em `REGISTRY_CONTRACT_ADDRESS`, mas esse endereço é o **registry de nós do Paladin** (artifact `core_v1alpha1_smartcontractdeployment_registry`, função `registerIdentity`), e **não** o `contracts/src/IdentityRegistry.sol` (whitelist de participantes, função `registerParticipant`, `onlyRole(GOVERNANCE_ROLE)`). Resultado: a tx reverte. Além disso, o `IdentityRegistry.sol` não é deployado pelo toolkit, e o registry de nós é deployado com admin `0x01`.

**Requisito adicional (FR-018):** o found DEVE deployar o `IdentityRegistry.sol` (whitelist) como contrato de nível-spoke separado, com admin = chave de governança do CB (genesis-funded no local), registrar seu endereço (ex.: `PARTICIPANT_REGISTRY_ADDRESS`) em `.deployed-addrs.env` e no bundle, e o `onboard-registry` DEVE chamar `registerParticipant` nesse endereço, assinado pela chave de governança. O `RoleCentralBank` correto é `3` (enum `CENTRAL_BANK`).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O toolkit DEVE registrar identidades de nó Paladin on-chain com lógica **nativa** (não os scripts de referência `register_nodes_test.go`), parametrizada por `spec.spoke.id` e por nó (CB no found; banco no join). NÃO DEVE haver nenhum `switch` por nome de spoke nem fallback para `spoke-a`.
- **FR-002**: No `mode: found`, o motor DEVE provisionar **apenas** o nó Paladin do banco central. NÃO DEVE criar nós `bank-a`/`bank-c`.
- **FR-003**: O template Compose Paladin do `found` DEVE conter **apenas** o serviço do CB (mais o `data-init` correspondente). Os serviços `bank-a`/`bank-c` DEVEM ser removidos.
- **FR-004**: O passo `render-configs` do `found` DEVE renderizar **apenas** a config do nó CB.
- **FR-005**: O passo `gen-tls` do `found` DEVE gerar **apenas** o certificado do nó CB (CN/SAN derivados de `spec.spoke.id`), sem nomes de banco hardcoded. *(Já implementado em `fix/toolkit`.)*
- **FR-006**: A sequência canônica do `found` DEVE **remover** `create-pente-context` e `deploy-fxa-pente`. A nova ordem: `start-besu → deploy-contracts → gen-tls → render-configs → register-nodes → start-paladin → create-zeto-token → onboard-registry → register-relay`.
- **FR-007**: O `mode: join` DEVE adicionar passos para subir o nó Paladin do banco e o relacionamento Pente, **nesta ordem**, após `proof-of-possession` (banco já registrado no IdentityRegistry) e antes de `start-backend`: `gen-tls-join` (cert TLS do nó Paladin do banco) → `render-config-join` (config do Paladin do banco, com `registryAddress` do bundle) → `start-paladin-join` (container Paladin do banco) → `register-paladin-node` (identidade do nó on-chain, nome derivado de `spec.bankId`) → `create-pente-context` (grupo CB↔banco) → `deploy-fxa-pente` (FXAgreement dentro do grupo).
- **FR-008**: O cert TLS do nó Paladin (`gen-tls-join`) é distinto da chave/CSR blockchain do banco (`gen-csr`/`request-cert`): o primeiro é o material de transporte gRPC do nó Paladin (auto-assinado, CN/SAN derivados de `spec.bankId`); o segundo é a identidade blockchain assinada pelo CB. Os dois fluxos NÃO DEVEM ser confundidos nem reutilizar a mesma chave.
- **FR-009**: O registro de nó Paladin on-chain (found e join) DEVE usar como chave-dono a chave blockchain do participante obtida via `keyProvider` — NÃO uma dev-key hardcoded por nó (como fazem os scripts de referência). O nome do nó é derivado de `spec.spoke.id` (CB) / `spec.bankId` (banco), nunca de uma lista fixa.
- **FR-010**: O `mode: join` DEVE criar o contexto Pente bilateral (CB↔banco) usando `penteFactoryAddress` do bundle, com exatamente os dois nós Paladin (CB e banco) como membros, e fazer o deploy do FXAgreement **dentro** desse grupo. Os endereços/groupId resultantes DEVEM ser persistidos no `.provisioning-state.yaml`/`.deployed-addrs.env` do banco.
- **FR-011**: O cert TLS de cada nó Paladin DEVE ser gerado dinamicamente pelo participante dono daquele nó (CB no found; banco no join), espelhando `spk-02/generate-paladin-certs.sh` — nunca um cert compartilhado com SANs de bancos fixos.
- **FR-012**: O join bundle (TK-6) NÃO DEVE mais exigir `PENTE_CONTEXT_GROUP_ID`, `PENTE_CONTEXT_ADDRESS` nem `FX_AGREEMENT_DEPLOYED_AT`; esses são produzidos no join. O bundle DEVE continuar exigindo (e carregando) os endereços de nível-spoke que o join consome: `registry`, `zetoFactory`, `penteFactory`, `zetoToken`.
- **FR-013**: DEVE existir um template Compose Paladin para o **banco comercial** (novo; o TK-8 atual é Besu-only), parametrizado por `BANK_ID`/`SPOKE_ID`/imagem/portas/rede, análogo ao `central-bank/paladin-compose.yaml` mas com um único nó (o do banco) e montando `config.yaml`+`tls.crt`+`tls.key` em `/etc/paladin`.
- **FR-014**: A lógica nativa de registro de nó e de criação de Pente DEVE ser coberta por testes unitários sem depender dos scripts de referência nem de containers (stubs/mocks de BesuRPC/Paladin/IdentityRegistry), seguindo o padrão de TK-5/TK-9.
- **FR-015**: A rede de referência (`deploy/local` + `make/*.mk`, incl. `register_nodes_test.go`, `create_pente_context_test.go`, `deploy_fxagreement_pente_test.go`) NÃO DEVE ser modificada; permanece como rede de amostra verde. A lógica nativa vive no toolkit.
- **FR-016**: Idempotência e `-dry-run` DEVEM funcionar para todos os novos passos (found e join), persistindo estado por passo em `.provisioning-state.yaml`.
- **FR-017**: O registro de nó Paladin on-chain DEVE ser idempotente por nome (re-registro do mesmo nome é tratado como sucesso "já registrado").

### Key Entities

- **PaladinNodeIdentity**: identidade de nó Paladin registrada no IdentityRegistry — nome (derivado de `spec.spoke.id`/`spec.bankId`), endereço dono, hash do cert, endpoint gRPC. Gerada parametricamente, sem topologia fixa.
- **PenteContext (per-relacionamento)**: grupo de privacidade Paladin entre dois participantes (CB↔banco). Criado no join, não no found. Atributos: groupId, address, membros.
- **JoinBundle (revisado)**: contratos de nível-spoke obrigatórios (registry, zetoFactory, penteFactory, zetoToken); `PENTE_CONTEXT_*`/`FX_AGREEMENT_*` deixam de ser obrigatórios.
- **ProvisioningState (found e join revisados)**:
  - **found** (9 passos): `start-besu`, `deploy-contracts`, `gen-tls`, `render-configs`, `register-nodes`, `start-paladin`, `create-zeto-token`, `onboard-registry`, `register-relay`. (Perde `create-pente-context` e `deploy-fxa-pente`.)
  - **join** (15 passos): os 9 atuais (`write-genesis` … `proof-of-possession`) + `gen-tls-join`, `render-config-join`, `start-paladin-join`, `register-paladin-node`, `create-pente-context`, `deploy-fxa-pente`, e por fim `start-backend`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Um `found` com `spec.spoke.id` arbitrário registra on-chain apenas identidades daquele spoke (verificável por leitura do IdentityRegistry); zero identidades `spoke-a-*` para um spoke que não é `spoke-a`.
- **SC-002**: Dois spokes (`spoke-brl`, `spoke-cop`) podem ser fundados no mesmo host sem colisão de identidade/cert e sem reuso cruzado.
- **SC-003**: Um banco comercial entra via `mode: join` e seu nó Paladin fica no ar e registrado on-chain com nome derivado de `spec.bankId`, sem nenhum nome de banco hardcoded no código do toolkit (verificável por `grep` ausente de `bank-a`/`bank-c` na lógica nativa).
- **SC-004**: Após o join do primeiro banco, existe um contexto Pente com exatamente os membros CB e o banco, e o FXAgreement está deployado nesse contexto.
- **SC-005**: O `found` CB-only completa ponta a ponta em run real (Besu+Paladin+contratos+Zeto+onboard+register-relay) com o relay local (start-cacti.sh) no ar.
- **SC-006**: Toda a lógica nativa passa em `go test ./...` sem containers/redes externas; nenhuma modificação nos scripts de referência.
- **SC-007**: Idempotência: re-`apply` após falha em qualquer passo novo retoma do ponto de falha em < 10s de overhead.

## Assumptions

- Os spikes `spk-01` e `spk-02` são autoritativos para o design: `spk-02/join-paladin.sh`, `register-paladin-nodes.sh`, `generate-paladin-certs.sh`, `render-paladin-configs.sh`, `vote-in-validator.sh` mostram a sequência dinâmica validada. A lógica nativa do toolkit reproduz essas etapas em Go (não invoca os scripts de referência hardcoded).
- O ADR-002 (SP-02) continua autoritativo: descoberta reativa de pares, sem restart de nós Paladin existentes.
- As mudanças já entregues em `fix/toolkit` são pré-requisito e permanecem: passo `start-besu` no found; resolução de path (`CBWEB3_HOME` + busca por âncora); criação do diretório de `.deployed-addrs.env`; genesis com funding + config EVM (shanghai/cancun/zeroBaseFee); `register-relay` hard; RL-1 no relay (`POST/GET /api/v1/spokes`) + `start-cacti.sh`; correção de interpolação de nome de serviço no `paladin-compose.yaml`; plumbing de `PALADIN_IMAGE`/portas/rede; `gen-tls` CB-only.
- O endereço `cbEndpoint` e o fluxo CSR→CB→cert do TK-9 permanecem; esta feature adiciona o nó Paladin do banco e o Pente, não altera a PKI.
- Imagens fixadas: `hyperledger/besu:25.8.0` e `docker.io/lfdecentralizedtrust/paladin:v0.15.0-rc.1`.
- Esta feature é Scenario A apenas; FASE 4 (KMS/CA prod) permanece fora de escopo.
- Migração de bundles antigos (com `PENTE_CONTEXT_*`) não é necessária; assume-se dataDir limpo por spoke em ambiente local.

## Out of Scope

- Implementação prod de `keyProvider`/`certSource` (FASE 4).
- Generalização do relay para N-routing (Fase 2 / RL-2/RL-3); RL-1 (registro) já entregue.
- Qualquer alteração na rede de referência `deploy/local` + `make/*.mk`.
- Liquidação ponta a ponta (HTLC) entre dois spokes via relay — depende desta feature mas é um esforço subsequente.
