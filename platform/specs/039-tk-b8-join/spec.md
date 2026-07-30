# Feature Specification: Toolkit do Cenário B — join (full node não-validador) (TK-B8)

**Feature Branch**: `039-tk-b8-join`
**Created**: 2026-07-11
**Status**: Draft
**Input**: User description: "TK-B8 — join (full node não-validador), conforme
`scenario-b/docs/design/scenario-b-toolkit-roadmap.md`."

> O terceiro (e último dos modos base) modo executável do toolkit: o **`join`** — um banco
> comercial anexa ao spoke soberano do seu CB, entrando como **full node não-validador** (modelo do
> Cenário A). Consome o **spoke bundle** (TK-B7), escreve o genesis do spoke sem regenerá-lo, sobe o
> nó do banco, **aguarda a sincronização** com o CB (validador único), conecta os endereços (do
> bundle) no backend do banco, provisiona o Keycloak do banco, sobe infra/backend/frontend e executa
> a **cauda diferida de PKI**: `gen-csr` — o **único** step de PKI do toolkit (par de chaves + CSR
> local). A **emissão do cert** (o CB assina o CSR via compliance, gated por KYC no portal
> `governance`) e o **registro on-chain** no `IdentityRegistry` são **runtime**, **não** steps do
> toolkit. Reusa o motor, o executor e as interfaces já entregues. **Fora de escopo**: promoção a
> validador (`vote-qbft`, capacidade diferida — ADR-002), assinatura de CSR e registro on-chain.

## Clarifications

### Session 2026-07-11

- Q: O fluxo canônico de `join` (roadmap §6) não inclui registro no relay nem noc-agent. Manter a US4
  (register-relay-bank + add-noc-agent soft) ou seguir só o fluxo canônico? → A: **Seguir o fluxo
  canônico** — a US4 é **removida**. O relay já observa a cadeia do spoke desde o `found-spoke`; um
  full node de banco é apenas mais um peer/RPC na mesma cadeia, então o registro por-banco é
  redundante. `join` não tem step de relay nem de noc-agent.
- Q: O spoke bundle do TK-B7 carrega apenas endereços de spoke. O roadmap §5 sugere reembalar também
  os endereços de hub para o `wire-addresses`. Como resolver no TK-B8? → A: **Wire apenas os endereços
  de spoke** presentes no bundle atual; o TK-B8 **não** altera o `emit-spoke-bundle` do TK-B7. Hub
  addresses para cross-border, se necessário, ficam como follow-up separado.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Escrever o genesis e sincronizar como full node não-validador (Priority: P1)

Como um banco comercial, quero **escrever o genesis do spoke** (do spoke bundle, sem regenerá-lo),
**subir o nó do banco** e **aguardar a sincronização** com o CB (validador único), de modo que o
banco tenha uma cópia completa e verificada da cadeia do spoke — **sem** produzir blocos.

**Why this priority**: sem um full node sincronizado o banco não enxerga o estado do spoke; é a porta
de entrada do `join` e o MVP. O banco entra como **não-validador** (o CB é o validador único).

**Independent Test**: dado um spoke bundle válido e o spoke no ar, o modo escreve o genesis (idêntico
ao do bundle; guard não-destrutivo se já existir), sobe o nó do banco (peer do enode do CB) e o
`wait-sync` confirma a sincronização; o nó **não** aparece como validador QBFT.

**Acceptance Scenarios**:

1. **Given** um spoke bundle válido, **When** o `join` inicia, **Then** o genesis é escrito a partir
   do bundle (falha clara se o bundle for inválido/ausente); re-rodar **não** sobrescreve/regenera um
   genesis já presente (guard não-destrutivo).
2. **Given** o nó do banco no ar, **When** `wait-sync` roda, **Then** o step só conclui quando o nó
   está sincronizado com a cadeia do spoke; o nó do banco **não** integra o conjunto de validadores
   QBFT (segue não-validador).

---

### User Story 2 — Conectar endereços + Keycloak + serviços do banco (Priority: P1)

Como o banco comercial, quero **conectar os endereços** (do spoke bundle) no env do backend do banco,
**provisionar o Keycloak** do banco (realm/client + write-back de secrets) e **subir
infra/backend/frontend**, de modo que os serviços do banco operem autenticados e cientes da cadeia.

**Why this priority**: sem o wire dos endereços e o Keycloak, os serviços do banco não funcionam nem
autenticam (gate de compliance, Princípio IV).

**Independent Test**: os endereços (do bundle) aparecem no env do backend do banco; o Keycloak do
banco provisiona realm/client e grava os secrets (write-back idempotente); infra/backend/frontend
sobem.

**Acceptance Scenarios**:

1. **Given** o spoke bundle, **When** `wire-addresses` roda, **Then** os endereços são escritos no env
   do backend do banco de forma idempotente (re-run não duplica).
2. **Given** o Keycloak do banco, **When** `provision-keycloak-bank` roda, **Then** os client secrets
   são gravados (write-back) e o step é idempotente.

---

### User Story 3 — Cauda diferida de PKI: gen-csr (Priority: P1)

Como o banco comercial, quero que o toolkit **gere localmente o par de chaves e o CSR**
(`OU=ROLE_COMMERCIAL_BANK`, CN do `bankId`), com a chave privada em `<dataDir>/pki/{bank}.key`
(permissão `0600`, **nunca transmitida**), de modo que eu possa submeter o CSR ao CB em runtime. O
toolkit **não** assina o CSR, **não** gera CA de banco e **não** registra on-chain.

**Why this priority**: o `gen-csr` é a fronteira do toolkit com o onboarding de runtime; entrega ao
banco o material que ele controla, mantendo a assinatura/emissão/registro fora do toolkit (evita a
colisão de wallet descrita no Cenário A).

**Independent Test**: `gen-csr` produz `{bank}.key` (`0600`) e `{bank}.csr` (`OU=ROLE_COMMERCIAL_BANK`,
CN=`bankId`) em `<dataDir>/pki/`; re-rodar é idempotente (não regenera se já existem); **nenhum**
arquivo `*-ca.key`/`*-ca.crt` é produzido; a chave privada nunca sai do host.

**Acceptance Scenarios**:

1. **Given** o dir `<dataDir>/pki/` (pré-criado como usuário do host), **When** `gen-csr` roda,
   **Then** o par de chaves + CSR são gravados localmente (chave `0600`), com `OU=ROLE_COMMERCIAL_BANK`
   e CN=`bankId`; re-rodar não regenera.
2. **Given** um `join` completo, **When** os artefatos são auditados, **Then** há **zero**
   `*-ca.key`/`*-ca.crt` no artefato do banco e a chave privada permanece apenas em `<dataDir>/pki/`.

---

### Edge Cases

- Spoke bundle ausente/inválido → `join` falha com erro claro **antes** de qualquer efeito.
- Genesis já presente em `<dataDir>` → **guard não-destrutivo**: não sobrescreve/regenera (mesmo
  hash); divergência de hash entre bundle e genesis local → erro claro.
- `wait-sync` nunca satisfeito (nó não sincroniza) → step falha com erro claro (nunca "verde" falso).
- `manifest.node.validator: true` → **warning** (o banco entra como full node não-validador; a
  promoção a validador é capacidade diferida, fora do `join`).
- Dir `<dataDir>/pki/` criado root-owned pelo Docker antes do `gen-csr` → o toolkit **pré-cria** o dir
  como usuário do host (armadilha do Cenário A) — `gen-csr` não falha por permissão.
- `gen-csr` re-rodado com chaves já presentes → idempotente (não regenera).
- Re-rodar `apply join` após sucesso → steps idempotentes pulados.
- `--dry-run` → nenhum efeito externo; report do plano.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O modo `join` MUST **consumir e validar o spoke bundle** (genesis + enode + chainId +
  endereços) antes de qualquer efeito; bundle inválido/ausente → erro claro.
- **FR-002**: O step `write-genesis` MUST escrever o genesis do spoke **a partir do bundle** (sem
  regenerar), com **guard não-destrutivo**: genesis já presente não é sobrescrito; divergência de
  hash → erro claro.
- **FR-003**: O step `start-besu-join` MUST subir o nó do banco como **full node não-validador**
  (peer do enode do CB do spoke), **sem** integrar o conjunto de validadores QBFT.
- **FR-004**: O step `wait-sync` MUST bloquear até o nó estar **sincronizado** com a cadeia do spoke;
  nunca reportar sucesso sem sincronização.
- **FR-005**: O step `wire-addresses` MUST escrever os endereços (do spoke bundle) no env do backend
  do banco, de forma idempotente.
- **FR-006**: O step `provision-keycloak-bank` MUST provisionar realm/client do banco e fazer
  **write-back** dos client secrets nos env, idempotente.
- **FR-007**: O modo MUST subir **infra/backend/frontend** do banco (após render do env).
- **FR-008**: O step `gen-csr` MUST gerar **localmente** par de chaves + CSR
  (`OU=ROLE_COMMERCIAL_BANK`, CN=`bankId`) em `<dataDir>/pki/`, com a chave privada `0600` e **nunca
  transmitida**; idempotente (não regenera se já existem).
- **FR-009**: O toolkit no `join` MUST **não** assinar o CSR, **não** gerar CA de banco e **não**
  registrar on-chain — essas ações são **runtime** (compliance do CB assina; registro pela governança
  do CB). Zero `*-ca.key`/`*-ca.crt` no artefato do banco.
- **FR-010**: O modo MUST **pré-criar** o dir `<dataDir>/pki/` como usuário do host antes do compose
  montá-lo, evitando o dir root-owned que quebra o `gen-csr` (armadilha do Cenário A).
- **FR-011**: O modo MUST tratar `node.validator: true` como **warning** (entra como não-validador); a
  promoção a validador (`vote-qbft`) está **fora** do fluxo canônico de `join`.
- **FR-012**: Todos os efeitos externos MUST passar pela fronteira injetável (executor / interfaces),
  de modo que o modo seja **testável sem** Docker/Foundry/Besu reais e que `--dry-run` não os acione.
- **FR-013**: O modo MUST ser **idempotente** (re-`apply` converge) e usar o mesmo motor/estado/lock/
  report do TK-B6/B7; erros tipados, nunca silenciosos.
- **FR-014**: A feature MUST incluir uma **suíte E2E** (build tag `e2e`) que faz um banco fazer `join`
  de verdade contra um spoke fundado (Docker + Besu) e valida a sincronização + o CSR gerado; **skip
  com aviso** quando o ambiente E2E está ausente (nunca falso verde).

### Key Entities

- **Spoke bundle (consumido)**: entrada com genesis + enode + chainId + endereços de spoke (produzido
  pelo TK-B7). *(Escopo Q1: pode ou não incluir os endereços de hub reembalados.)*
- **Banco (manifesto join)**: `bankId` (alimenta CSR CN, `IdentityRegistry`, `BANK_ID`), spoke de
  destino (id/chainId/moeda), referência ao spoke bundle, node/portas, `validator: false`.
- **Step set do `join`** (fluxo canônico, roadmap §6): write-genesis, start-besu-join, wait-sync,
  wire-addresses, provision-keycloak-bank, render-bank-env, infra/backend/frontend, gen-csr (cauda
  diferida de PKI). Sem step de relay nem de noc-agent (a cadeia já é observada desde o found-spoke).
- **CSR do banco**: par de chaves + CSR local (`OU=ROLE_COMMERCIAL_BANK`, CN=`bankId`), chave privada
  `0600` em `<dataDir>/pki/`, nunca transmitida — consumido pelo onboarding de runtime.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Um spoke bundle inválido/ausente faz o `join` **falhar antes** de qualquer efeito.
- **SC-002**: `write-genesis` escreve o genesis do bundle; re-rodar **não** sobrescreve/regenera
  (guard não-destrutivo); divergência de hash → erro.
- **SC-003**: O nó do banco sobe e o `wait-sync` só conclui após a sincronização; o nó **não** é
  validador QBFT (não produz blocos).
- **SC-004**: `wire-addresses` e `provision-keycloak-bank` são idempotentes (re-run não duplica/
  regrava).
- **SC-005**: `gen-csr` produz `{bank}.key` (`0600`) + `{bank}.csr` (`OU=ROLE_COMMERCIAL_BANK`,
  CN=`bankId`) em `<dataDir>/pki/`; re-rodar não regenera; **zero** `*-ca.key`/`*-ca.crt`; a chave
  privada nunca sai do host.
- **SC-006**: `node.validator: true` gera **warning** e o banco ainda entra como não-validador.
- **SC-007** (E2E): num ambiente com Docker + Besu e um spoke fundado, `apply join` sincroniza o banco
  de ponta a ponta e o CSR é gerado; ambiente ausente ⇒ **skip com aviso**.

## Assumptions

- **Padrão de execução (consistente com TK-B4..B7): E2E completo** com executor/interfaces injetáveis
  — steps reais, testes de unidade com fakes, suíte E2E sob build tag `e2e` (skip-com-aviso). O E2E do
  `join` pressupõe um **spoke já fundado** (spoke bundle disponível).
- **Banco = full node não-validador; CB = validador único** (genesis `count: 1`) — decidido no
  roadmap (§6/§14). O `vote-qbft` (promoção a validador) é **capacidade diferida** (ADR-002), **fora**
  do escopo de `join`. Diverge do `deploy/local` atual (cujo `startBesu.sh` gera 2 validadores).
- **PKI no `join` = apenas `gen-csr`** (roadmap §15). Assinatura do CSR (compliance do CB), emissão
  gated por KYC (`governance` + compliance) e registro on-chain (`IdentityRegistry`) são **runtime**,
  não steps do toolkit — para não criar registro keyed na wallet do toolkit (colisão do Cenário A).
- **Wire só endereços de spoke (resolvido)**: o spoke bundle do TK-B7 carrega **apenas** os endereços
  de spoke (`identityRegistry`, `tCeBM`, `spokeBridge`, `fCeBM`); o `join` faz wire **apenas** desses
  e **não** altera o `emit-spoke-bundle` do TK-B7. Reembalar hub addresses (roadmap §5) para
  cross-border, se necessário, é **follow-up separado**.
- **Sem relay/noc (resolvido)**: o `join` segue o **fluxo canônico** do roadmap §6 — sem
  `register-relay-bank` e sem `add-noc-agent`. A cadeia do spoke já é observada pelo relay desde o
  `found-spoke`; o banco é apenas mais um full node peer na mesma cadeia.
- **CA cert (âncora de confiança) no bundle**: o roadmap §15 prevê o cert da CA do CB no spoke bundle;
  o TK-B7 não o inclui (PKI cheia diferida). Como o `gen-csr` **não** precisa do cert da CA para
  produzir o CSR (a assinatura é runtime), consumir o cert da CA fica **fora** do escopo de TK-B8.
- **Reuso**: o motor/estado/lock/report/executor (TK-B6), o guard de genesis e o `wait-*` (TK-B6/B7),
  `KeyProvider`/`CertSource` (TK-B2/B3) e os templates (TK-B4). Nada importado de `scenario-a/`.
- Não alterar Makefiles nem `deploy/local`; não importar `scenario-a/` (Princípio I). Sem novas
  dependências Go.
