# Feature Specification: Toolkit do Cenário B — KeyProvider + CertSource (TK-B2/B3)

**Feature Branch**: `034-tk-b2-keyprovider-certsource`
**Created**: 2026-07-10
**Status**: Draft
**Input**: User description: "Implementar TK-B2/B3 (KeyProvider + CertSource) usando como
referência `scenario-b/docs/design/scenario-b-toolkit-roadmap.md`, dando continuidade ao toolkit."

> As duas **interfaces plugáveis de custódia** do toolkit do Cenário B (roadmap §10, §15):
> **KeyProvider** (custódia de chaves blockchain) e **CertSource** (CA do spoke + emissão de
> leaf). Cada uma com **implementação local in-memory** + **stub de produção**, atrás de
> factories por URI. **Nenhum segredo** entra em manifesto/estado/bundle. Chaves **por
> entidade**. CB atua como **CA do spoke**, emitindo o leaf a partir de um CSR. Não inclui
> motor de steps, bundles nem execução (fases seguintes consomem estas interfaces).

## User Scenarios & Testing *(mandatory)*

### User Story 1 — KeyProvider: custódia de chaves por entidade (Priority: P1)

Como o toolkit, preciso de uma fronteira de custódia que **gere e guarde uma chave própria por
entidade**, **assine** digests e **exponha apenas a chave pública/endereço** — sem que a chave
privada vaze para manifesto/estado/bundle.

**Why this priority**: fases seguintes (deploy de contratos, grants, registro on-chain, relay)
dependem de assinar transações por entidade; é fundação de segurança do toolkit.

**Independent Test**: para um id de entidade, gerar chave → obter pubkey/endereço → assinar um
digest → verificar a assinatura contra a pubkey; ids distintos produzem chaves/endereços
distintos; a chave privada não aparece em nenhuma saída serializável.

**Acceptance Scenarios**:

1. **Given** um id de entidade, **When** gero a chave e assino um digest, **Then** a assinatura
   verifica contra a pubkey retornada, e o `GenerateKey` é idempotente (mesma pubkey em re-chamada).
2. **Given** dois ids distintos (ex.: `central-bank-a`, `bank-a`), **When** gero as chaves,
   **Then** os endereços EVM resultantes são **distintos** (isolamento por entidade).
3. **Given** um `keyProvider` de produção (`kms://…` não-emulador), **When** chamo qualquer
   operação, **Then** recebo um erro "não implementado" (stub), sem falha silenciosa.

---

### User Story 2 — CertSource: CB como CA do spoke, emissão de leaf (Priority: P1)

Como o toolkit, preciso que o **banco central seja a CA do seu spoke**, emitindo o certificado
leaf de um banco a partir de um **CSR** (o banco detém a própria chave), e expondo a **âncora de
confiança** (cert da CA) — com a chave da CA **nunca** saindo do processo.

**Why this priority**: é o modelo de PKI decidido (single-tier, CB-issued — roadmap §15) e
fundação do onboarding; sem ele o `found-spoke`/`join` não têm âncora nem emissão.

**Independent Test**: gerar um CSR P-256 com `OU=ROLE_COMMERCIAL_BANK`; `IssueLeafCert` retorna
um leaf que **verifica** contra a âncora (`GetTrustAnchor`); CSRs inválidos (curva errada, sem o
OU, CSR malformado) são rejeitados com erro específico; a chave da CA não vai a disco/log/bundle.

**Acceptance Scenarios**:

1. **Given** um CSR válido (ECDSA P-256, `OU=ROLE_COMMERCIAL_BANK`), **When** emito o leaf,
   **Then** o leaf verifica contra a CA retornada por `GetTrustAnchor` do mesmo spoke.
2. **Given** um CSR com chave não-P-256 **ou** sem `OU=ROLE_COMMERCIAL_BANK` **ou** malformado,
   **When** tento emitir, **Then** recebo erro específico (algoritmo/role/CSR inválido) e nenhum
   cert é emitido.
3. **Given** um `certSource` de produção (`ca://…`), **When** chamo qualquer operação, **Then**
   recebo um erro "não implementado" (stub).

---

### User Story 3 — Factories por URI e garantia de "sem segredos" (Priority: P2)

Como operador, quero selecionar a implementação por **URI** (`kms://`, `self-signed`, `ca://`) e
ter a **garantia** de que nenhuma chave privada é serializada.

**Why this priority**: torna as interfaces plugáveis (local↔prod) e impõe a invariante de
segurança transversal.

**Independent Test**: `kms://local-emulator` → local; outro `kms://…` → stub; `self-signed` →
local; `ca://…` → stub. Uma varredura das saídas serializáveis (report/estado/bundle simulados)
não contém material de chave privada.

**Acceptance Scenarios**:

1. **Given** cada URI suportada, **When** construo a implementação, **Then** obtenho local
   (emulador/self-signed) ou stub de prod, conforme a tabela.
2. **Given** uma URI não suportada, **When** construo, **Then** recebo erro de configuração.

### Edge Cases

- `GenerateKey` chamado duas vezes para o mesmo id → idempotente (mesma chave), sem erro.
- `GetPublicKey` para id inexistente → erro claro (não gera implicitamente).
- `GetTrustAnchor` para um spoke sem CA ainda inicializada → `ErrTrustAnchorNotFound` (não cria CA).
- CSR cuja assinatura não confere com a própria chave → rejeitado.
- Provider/CertSource de prod → toda operação retorna "não implementado" (sem efeito colateral).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O `KeyProvider` MUST expor `GenerateKey(id) → pubkey` (idempotente),
  `Sign(id, digest) → assinatura` e `GetPublicKey(id) → pubkey`; nunca retornar a chave privada
  por essas operações.
- **FR-002**: As chaves blockchain MUST ser **secp256k1**, com **endereço EVM** derivável da
  pubkey (compatível com assinatura de transações Besu/QBFT).
- **FR-003**: A implementação **local** MUST manter as chaves **apenas em memória**, gerar sob
  demanda de forma **determinística por id** (endereço estável entre execuções — útil para
  genesis/grants) e **semear** uma chave dev conhecida sob um id conhecido (emulador local).
- **FR-004**: A implementação local MAY expor um **export de chave privada apenas-local**
  (assinatura na camada Besu pelo backend); a implementação de **produção MUST NÃO** expô-lo.
- **FR-005**: Cada entidade (CB, banco, hub-signer, relayer) MUST ter a **sua própria chave**;
  o endereço de um banco MUST ser **único e desacoplado** do signer do CB (evita débito cruzado).
- **FR-006**: A factory do `KeyProvider` MUST resolver `kms://local-emulator` → local semeado, e
  qualquer outro `kms://…` → **stub de produção** (retorna "não implementado").
- **FR-007**: O `CertSource` MUST expor `IssueLeafCert(csrPEM, spokeID) → certPEM` e
  `GetTrustAnchor(spokeID) → caCertPEM`.
- **FR-008**: A implementação **local** do `CertSource` MUST gerar uma **CA self-signed ECDSA
  P-256 por spoke, apenas em memória**; a chave privada da CA MUST **nunca** ser escrita em
  disco, log ou variável de ambiente.
- **FR-009**: `IssueLeafCert` MUST validar, nesta ordem: parse PEM/PKCS#10 + auto-assinatura do
  CSR; chave **ECDSA P-256**; `OU` do subject contém `ROLE_COMMERCIAL_BANK`; então assinar o leaf
  (uso `ClientAuth`+`ServerAuth`, não-CA, validade ~1 ano). Falhas retornam erro específico.
- **FR-010**: `GetTrustAnchor` MUST retornar o cert PEM da CA do spoke; MUST **não** inicializar
  uma CA (só `IssueLeafCert` inicializa); MUST retornar `ErrTrustAnchorNotFound` se ausente.
- **FR-011**: A factory do `CertSource` MUST resolver `self-signed` | `self-signed://…` → local,
  e `ca://…` → **stub de produção**.
- **FR-012**: **Sem segredos**: nenhuma chave privada (do `KeyProvider` ou da CA do `CertSource`)
  MUST ser serializada para manifesto, estado ou bundle; apenas pubkeys/certs cruzam essas
  fronteiras.
- **FR-013**: Erros MUST ser tipados e distinguíveis: "não implementado" (prod), papel proibido,
  algoritmo de chave não suportado, CSR inválido, âncora de confiança ausente.

### Key Entities

- **KeyProvider**: fronteira de custódia de chaves blockchain (interface + local in-memory + stub
  de prod). Guarda chaves por id de entidade; expõe pubkey/endereço e assinatura.
- **CertSource**: fronteira de CA do spoke (interface + local in-memory + stub de prod). CB como
  CA; emite leaf a partir de CSR; expõe âncora de confiança.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Para um id, o local gera chave, `GetPublicKey` a retorna, `Sign` produz assinatura
  **verificável** contra a pubkey; `GenerateKey` repetido é idempotente.
- **SC-002**: Ids distintos → endereços EVM **distintos** (isolamento por entidade).
- **SC-003**: `kms://…` de produção → toda operação retorna "não implementado".
- **SC-004**: CSR P-256 com `OU=ROLE_COMMERCIAL_BANK` → leaf emitido **verifica** contra a âncora
  do mesmo spoke.
- **SC-005**: `IssueLeafCert` **rejeita** (com erro específico) CSR com chave não-P-256, sem o OU,
  ou malformado.
- **SC-006**: A chave privada da CA (e as chaves do `KeyProvider`) **não** aparecem em nenhuma
  saída serializável nem em disco (verificável).
- **SC-007**: As factories resolvem `kms://local-emulator`, `self-signed`, `ca://…` conforme a
  tabela (prod → stub), e URIs não suportadas geram erro de configuração.

## Assumptions

- `environment: local` primeiro: as implementações de **produção** (`kms://…`, `ca://…`) são
  **stubs** nesta fase; a integração real (KMS/PKI) fica para uma fase posterior.
- Curvas: **secp256k1** para chaves blockchain (endereço EVM); **ECDSA P-256** para PKI/TLS.
- As interfaces **espelham** as do toolkit de referência (Cenário A: `engine/keyprovider`,
  `engine/certsource`), **reimplementadas** no módulo `scenario-b/toolkit` (Constituição,
  Princípio I) — não importadas.
- A geração de CSR (para testes de `IssueLeafCert`) usa material gerado no próprio teste; o fluxo
  de onboarding em runtime (credential-request) é de fase posterior.
- Escopo exclui motor/steps/bundles/execução (fases TK-B5+).
