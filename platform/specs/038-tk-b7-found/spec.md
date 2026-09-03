# Feature Specification: Toolkit do Cenário B — found-spoke + spoke bundle (TK-B7)

**Feature Branch**: `038-tk-b7-found`
**Created**: 2026-07-11
**Status**: Draft
**Input**: User description: "TK-B7 — found-spoke (register-cb + contratos de spoke + Keycloak
write-back + register-relay-spoke + add-noc-agent) + spoke bundle emitter, conforme
`scenario-b/docs/design/scenario-b-toolkit-roadmap.md`."

> O segundo modo executável do toolkit: o **`found-spoke`** — um banco central soberano funda o seu
> spoke. Consome o **hub bundle** (TK-B6), auto-registra o CB no `IdentityRegistry` do hub, sobe a
> rede do spoke (CB como validador), deploya os contratos do spoke, conecta os endereços de hub no
> backend, provisiona o Keycloak (com write-back), sobe infra/backend/frontend, **registra o spoke no
> relay** (runtime, TK-B5) e sobe um **`noc-agent`** (soft), emitindo ao final o **spoke bundle**
> (que carrega genesis + enode para os bancos comerciais fazerem `join` no TK-B8). Reusa o motor, as
> interfaces (KeyProvider, CertSource, RelayRegistrar) e os templates já entregues. **Fora de
> escopo**: par soberano, liquidez cooperativa e seed-oracle (são TK-B9).

## Clarifications

### Session 2026-07-11

- Q (A1): Como o `register-cb` cobre o grant de provedor de liquidez? → A: o `register-cb` executa
  **duas** ações, ambas idempotentes: (1) `registerParticipant(CENTRAL_BANK)` (via
  `RegisterParticipants.s.sol`) e (2) **`grantLiquidityProvider(CB)`** no `IdentityRegistry` do hub —
  tentado **automaticamente** no hub. O grant é `onlyRole(DEFAULT_ADMIN_ROLE)`: em `local` o toolkit
  detém a chave do **admin do hub** e completa; fora de `local` o grant é ato da **governança do
  hub** e, se a permissão faltar, o step registra pendência (nota futura) sem falhar o registro.
  Idempotência: `isParticipant`/`isLiquidityProvider` (pula se já feito).

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Consumir o hub bundle e auto-registrar o CB (Priority: P1)

Como um banco central soberano, quero **consumir o hub bundle** (endereços do hub) e **auto-registrar
o CB** no `IdentityRegistry` do hub (como `CENTRAL_BANK`, com provedor de liquidez), de modo que o
spoke conheça o hub e o CB passe a ser participante/signatário de governança — sem hardcode.

**Why this priority**: sem consumir o bundle e registrar o CB, o spoke não se integra ao hub; é a
porta de entrada do `found-spoke` e MVP.

**Independent Test**: dado um hub bundle válido, o modo carrega/valida os endereços do hub e, contra
o RPC do hub, registra o CB (`registerParticipant` CENTRAL_BANK + grant de provedor de liquidez); o
step é idempotente (CB já registrado → pula).

**Acceptance Scenarios**:

1. **Given** um hub bundle válido, **When** o `found-spoke` inicia, **Then** os endereços do hub são
   carregados e validados (falha clara se o bundle for inválido/ausente).
2. **Given** o CB ainda não registrado no hub, **When** o step `register-cb` roda, **Then** o CB é
   registrado como `CENTRAL_BANK` no `IdentityRegistry` do hub; re-rodar é idempotente (não
   re-registra).

---

### User Story 2 — Subir a rede do spoke e deployar os contratos do spoke (Priority: P1)

Como o CB soberano, quero **gerar o genesis do spoke**, **subir o nó do spoke** (CB como validador) e
**deployar os contratos do spoke** (IdentityRegistry → tCeBM doméstico → SpokeBridge → fCeBM, com
`GOVERNANCE_ROLE` ao CB), de modo que a rede soberana exista e opere.

**Why this priority**: é o núcleo da soberania do spoke — a rede e os contratos próprios do CB.

**Independent Test**: o genesis do spoke é gerado (chain própria do spoke), o nó sobe (RPC no ar),
os contratos do spoke são deployados na ordem e os endereços ficam disponíveis; re-rodar pula o que
já existe.

**Acceptance Scenarios**:

1. **Given** o spoke sem genesis, **When** o step de genesis roda, **Then** o genesis do spoke
   (chain id próprio do manifesto) é gerado antes de subir o nó.
2. **Given** o nó do spoke no ar, **When** `deploy-spoke-contracts` roda, **Then** os contratos do
   spoke são deployados e o CB recebe `GOVERNANCE_ROLE`; a ordem de dependência é respeitada.

---

### User Story 3 — Wire de endereços do hub + Keycloak + serviços (Priority: P1)

Como o CB soberano, quero **conectar os endereços do hub** (do bundle) no env do backend do spoke,
**provisionar o Keycloak** (realms/clients + write-back de secrets) e **subir infra/backend/frontend**
do spoke, de modo que os serviços do spoke operem autenticados e cientes do hub.

**Why this priority**: sem o wire dos endereços e o Keycloak, os serviços do spoke não funcionam nem
autenticam (gate de compliance, Princípio IV).

**Independent Test**: os endereços do hub (do bundle) aparecem no env do backend; o Keycloak do spoke
provisiona realm/client e grava os secrets (write-back idempotente); infra/backend/frontend sobem.

**Acceptance Scenarios**:

1. **Given** o hub bundle, **When** `wire-hub-addresses` roda, **Then** os endereços do hub são
   escritos no env do backend do spoke (idempotente).
2. **Given** o Keycloak do spoke, **When** `provision-keycloak-spoke` roda, **Then** os client
   secrets são gravados (write-back) e o step é idempotente.

---

### User Story 4 — Registrar o spoke no relay e subir o noc-agent (Priority: P2)

Como o CB soberano, quero **registrar o spoke no relay** em runtime (`POST /api/v1/spokes`, TK-B5) e
subir um **`noc-agent`** ligado ao Besu do spoke, de modo que o relay passe a observar/rotear o novo
spoke e a observabilidade cubra o spoke. O `add-noc-agent` é **soft**: sua falha **não** bloqueia o
`found-spoke`.

**Why this priority**: integra o spoke ao relay neutro e à observabilidade, mas depende de o spoke já
existir (US1–US3); o noc-agent é observabilidade (soft).

**Independent Test**: `register-relay-spoke` chama o registro do relay com o WS do spoke + URL do
gateway (idempotente); `add-noc-agent` sobe o agente e, se falhar, o `found-spoke` prossegue (o step
é marcado como não-fatal).

**Acceptance Scenarios**:

1. **Given** o spoke no ar, **When** `register-relay-spoke` roda, **Then** o spoke é registrado no
   relay (id, WS, gateway) sem restart; re-registrar é idempotente.
2. **Given** o `noc-agent`, **When** ele falha ao subir, **Then** o `found-spoke` **não** falha (soft)
   e o report marca o step como falha não-bloqueante.

---

### User Story 5 — Emissão do spoke bundle (Priority: P1)

Como o CB soberano, quero que, ao final do `found-spoke`, o toolkit **emita o spoke bundle** — um
artefato versionado com o **genesis do spoke**, o **enode** do CB (bootnode), o chainId, os endereços
dos contratos do spoke e a config de consumo — de modo que os bancos comerciais façam `join` (TK-B8)
sem hardcode.

**Why this priority**: o spoke bundle é o hand-off do `found-spoke` para o `join`; sem ele, os bancos
não têm como entrar no spoke.

**Independent Test**: dado o estado final do `found-spoke`, emitir o spoke bundle e recarregá-lo/
validá-lo: contém genesis + enode + chainId + endereços de spoke; é versionado; **não contém
segredos** (nenhuma chave privada).

**Acceptance Scenarios**:

1. **Given** o spoke fundado, **When** o bundle é emitido, **Then** ele contém o genesis do spoke, o
   enode do CB, o chainId e os endereços de contrato do spoke, versionado.
2. **Given** um spoke bundle emitido, **When** é recarregado/validado, **Then** passa a validação
   (round-trip) e **não** contém material de chave privada.

### Edge Cases

- Hub bundle ausente/inválido → `found-spoke` falha com erro claro **antes** de qualquer efeito.
- Re-rodar `apply found-spoke` após sucesso → steps idempotentes pulados; spoke bundle reemitido de
  forma estável.
- `register-cb` quando o CB já é participante → idempotente (não re-registra).
- `register-relay-spoke` repetido → idempotente (upsert por id de spoke, TK-B5).
- `add-noc-agent` falha → **não bloqueia** o `found-spoke` (soft), report marca não-fatal.
- Gate de prontidão (RPC do spoke, Keycloak) nunca satisfeito → step falha com erro claro.
- `--dry-run` → nenhum efeito externo; report do plano.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O modo `found-spoke` MUST **consumir e validar o hub bundle** (endereços do hub) antes
  de qualquer efeito; bundle inválido/ausente → erro claro.
- **FR-002**: O step `register-cb` MUST executar, contra o RPC do hub e de forma **idempotente**,
  duas ações: (a) **`registerParticipant(CENTRAL_BANK)`** (via `RegisterParticipants.s.sol`; Check
  `isParticipant`) e (b) **`grantLiquidityProvider(CB)`** no `IdentityRegistry` do hub, **tentado
  automaticamente** (Check `isLiquidityProvider`). O grant é `onlyRole(DEFAULT_ADMIN_ROLE)`: em
  `local` o toolkit detém a chave do **admin do hub** e completa; fora de `local`, se a permissão
  faltar, o step **registra a pendência** (ato futuro da governança do hub) **sem falhar** o
  registro do participante.
- **FR-003**: O modo MUST **gerar o genesis do spoke** (chain id próprio do manifesto) **antes** de
  subir o nó do spoke (CB como validador/bootnode), com gate de RPC no ar.
- **FR-004**: O step `deploy-spoke-contracts` MUST deployar os contratos do spoke (IdentityRegistry →
  tCeBM doméstico → SpokeBridge → fCeBM) e conceder `GOVERNANCE_ROLE` ao CB; idempotente (endereços
  já presentes → pula).
- **FR-005**: O step `wire-hub-addresses` MUST escrever os endereços do hub (do bundle) no env do
  backend do spoke, de forma idempotente.
- **FR-006**: O step `provision-keycloak-spoke` MUST provisionar realm/client do spoke e fazer
  **write-back** dos client secrets nos env, idempotente.
- **FR-007**: O modo MUST subir **infra/backend/frontend** do spoke (após render do env).
- **FR-008**: O step `register-relay-spoke` MUST registrar o spoke no relay em runtime (id do spoke +
  WS + URL do gateway) via a fronteira `RelayRegistrar` (TK-B5), **sem restart** e idempotente.
- **FR-009**: O step `add-noc-agent` MUST ser **soft**: subir um `noc-agent` ligado ao Besu do spoke;
  a sua falha **não** bloqueia o `found-spoke` (report marca não-fatal).
- **FR-010**: Ao final, o modo MUST **emitir o spoke bundle** — versionado, com **genesis do spoke**,
  **enode** do CB, chainId e endereços de contrato do spoke — carregável e validável.
- **FR-011**: O spoke bundle MUST **não conter segredos** (nenhuma chave privada); genesis, enode,
  chainId e endereços são dados públicos/de rede.
- **FR-012**: Todos os efeitos externos MUST passar pela fronteira injetável (executor / interfaces),
  de modo que o modo seja **testável sem** Docker/Foundry/Besu reais e que `--dry-run` não os acione.
- **FR-013**: O modo MUST ser **idempotente** (re-`apply` converge) e usar o mesmo motor/estado/lock/
  report do TK-B6; erros tipados, nunca silenciosos.
- **FR-014**: A feature MUST incluir uma **suíte E2E** (build tag `e2e`) que funda um spoke de verdade
  contra um hub fundado (Docker + Foundry + Besu) e valida o spoke bundle; **skip com aviso** quando
  o ambiente E2E está ausente (nunca falso verde).

### Key Entities

- **Hub bundle (consumido)**: entrada com os endereços do hub + RPC/chainId (produzido pelo TK-B6).
- **Spoke (manifesto found-spoke)**: id do spoke, chain id próprio, CB, moeda, referência ao hub
  bundle, node/portas.
- **Step set do `found-spoke`**: consume-hub-bundle, register-cb, gen-genesis-spoke, start-besu-spoke,
  deploy-spoke-contracts, wire-hub-addresses, provision-keycloak-spoke, render-spoke-env,
  infra/backend/frontend, register-relay-spoke, add-noc-agent (soft), emit-spoke-bundle.
- **Spoke bundle**: artefato versionado com genesis do spoke + enode do CB + chainId + endereços de
  contrato do spoke (público, sem segredos) — consumido pelo `join` (TK-B8).
- **Step soft**: step cuja falha não interrompe o modo (ex.: `add-noc-agent`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Um hub bundle inválido/ausente faz o `found-spoke` **falhar antes** de qualquer efeito.
- **SC-002**: `register-cb` registra o CB no hub como `CENTRAL_BANK` **e** concede o papel de
  provedor de liquidez (grant automático); re-rodado, **não** re-registra nem re-concede
  (idempotente via `isParticipant`/`isLiquidityProvider`).
- **SC-003**: O genesis do spoke é gerado **antes** do nó subir; o nó do spoke sobe (gate RPC).
- **SC-004**: `deploy-spoke-contracts` produz os endereços dos contratos do spoke (idempotente) e o
  CB detém `GOVERNANCE_ROLE`.
- **SC-005**: `wire-hub-addresses` e `provision-keycloak-spoke` são idempotentes (re-run não
  duplica/regrava).
- **SC-006**: `register-relay-spoke` registra o spoke no relay sem restart; repetir é idempotente.
- **SC-007**: `add-noc-agent` que falha **não** derruba o `found-spoke` (report marca não-fatal).
- **SC-008**: O spoke bundle emitido **recarrega e valida** (round-trip), contém genesis + enode +
  chainId + endereços, e **não** contém segredos.
- **SC-009** (E2E): num ambiente com Docker + Foundry + Besu e um hub fundado, `apply found-spoke`
  funda o spoke de ponta a ponta e o spoke bundle valida; ambiente ausente ⇒ **skip com aviso**.

## Assumptions

- **Padrão de execução (consistente com TK-B6): E2E completo** com executor/interfaces injetáveis —
  steps reais, testes de unidade com fakes, suíte E2E sob build tag `e2e` (skip-com-aviso). O E2E do
  `found-spoke` pressupõe um **hub já fundado** (hub bundle disponível).
- **Fora de escopo (TK-B9)**: `open-sovereign-pair`, `commit-liquidity` e `seed-oracle` — os steps
  *soft* de par soberano/liquidez/oracle não fazem parte do `found-spoke` desta fase.
- **Reuso**: o motor/estado/lock/report/executor (TK-B6), `KeyProvider`/`CertSource`/`RelayRegistrar`
  (TK-B2/B3/B5), os templates (TK-B4) e o `gen-genesis` (parametrizado para a chain do spoke). Nada
  importado de `scenario-a/`.
- **Deploy dos contratos do spoke** = um `forge script` (análogo ao `CBWeb3Spoke.s.sol` /
  `contracts.deploy-spoke`), com a ordem interna ao Solidity; endereços do broadcast JSON.
- **register-cb** = auto-registro dirigido por solicitação (automático em `local`); a governança do
  hub autorizar no futuro é uma nota (não implementada nesta fase).
- **Spoke bundle** difere do hub bundle: **inclui genesis + enode** (lidos dos named volumes do Besu),
  pois o `join` precisa deles; o hub bundle era RPC-only.
- **Auth por CB no relay** permanece **fora de escopo** (§14.D — pendente do project-lead); o
  `register-relay-spoke` usa o segredo compartilhado atual.
- Não alterar Makefiles nem `deploy/local`; não importar `scenario-a/` (Princípio I).
