# Feature Specification: TK-3 — Interface certSource

**Feature Branch**: `025-tk3-certsource-interface`  
**Created**: 2026-06-27  
**Status**: Draft  
**Input**: User description: "TK-3 — CRIAR: interface certSource para o provisioning toolkit do Scenario A (Fase 1B)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Emitir certificado folha para banco comercial (Priority: P1)

O motor de provisionamento precisa emitir um certificado TLS para um banco comercial após receber o CSR gerado pelo banco. O banco central, que é a CA do spoke, assina o CSR e devolve o certificado folha. O banco retém apenas sua chave privada; o banco central retém apenas sua chave de CA.

**Why this priority**: É o fluxo de onboarding real do Scenario A. Sem este passo, o banco comercial não pode autenticar no gateway via PKI nonce challenge-response, bloqueando toda integração entre participantes.

**Independent Test**: Pode ser testado gerando um CSR de teste (P-256), chamando `IssueLeafCert` com o CSR e um trust anchor válido, e verificando que o certificado emitido (a) tem `CN` do banco, (b) tem `OU=ROLE_COMMERCIAL_BANK`, (c) é assinado pela CA do spoke (verificável com `GetTrustAnchor`), e (d) não vaza a chave privada da CA.

**Acceptance Scenarios**:

1. **Given** um manifesto com `certSource: self-signed` e um CSR PKCS#10 válido com `OU=ROLE_COMMERCIAL_BANK`, **When** o motor chama `IssueLeafCert(csr, spokeTrustAnchor)`, **Then** o certificado retornado é assinado pela CA do spoke, contém o `CN` e `OU` do CSR, e tem validade configurável.
2. **Given** um CSR malformado (PEM inválido ou campos obrigatórios ausentes), **When** `IssueLeafCert` é chamado, **Then** o método retorna um erro descritivo sem criar nenhum material criptográfico.
3. **Given** um CSR de teste com `OU=ROLE_CENTRAL_BANK` (proibido — CB não emite cert para outro CB neste fluxo), **When** `IssueLeafCert` é chamado, **Then** o método retorna `ErrForbiddenRole` sem assinar o CSR.

---

### User Story 2 — Recuperar a âncora de confiança do spoke (Priority: P1)

Após o banco central provisionar o spoke via `mode: found`, o motor emite o join bundle. O join bundle deve conter o certificado CA do spoke (âncora de confiança) para que bancos comerciais possam verificar os certs emitidos. O motor consulta `GetTrustAnchor(spokeId)` para obter este certificado.

**Why this priority**: O join bundle sem a âncora de confiança é incompleto; o banco comercial não consegue validar os certs do spoke e não pode executar `mode: join`. Bloqueia TK-6 (emissor do join bundle).

**Independent Test**: Pode ser testado instanciando a implementação local, chamando `GetTrustAnchor` para um spoke inicializado, e verificando que o certificado retornado é self-signed, tem `CN` condizente com o spoke, e pode ser usado para verificar um cert emitido por `IssueLeafCert`.

**Acceptance Scenarios**:

1. **Given** uma instância local de `certSource` inicializada para `spoke-brl`, **When** o motor chama `GetTrustAnchor("spoke-brl")`, **Then** o PEM do certificado CA do spoke é retornado e pode ser carregado como `*x509.Certificate`.
2. **Given** um `spokeId` inexistente (spoke nunca inicializado), **When** `GetTrustAnchor` é chamado, **Then** o método retorna `ErrTrustAnchorNotFound` com o `spokeId` na mensagem de erro.
3. **Given** a implementação local com o spoke recém-inicializado, **When** `GetTrustAnchor` é chamado duas vezes para o mesmo `spokeId`, **Then** o mesmo certificado CA é retornado em ambas as chamadas (idempotente).

---

### User Story 3 — Extensibilidade para CA de produção sem alterar o motor (Priority: P3)

Um operador que migra para produção consegue trocar a fonte de certificados apenas alterando `certSource:` no manifesto. O motor de orquestração não precisa de nenhuma modificação de código; a interface permanece a mesma.

**Why this priority**: Garante que a Fase 4 (prod) não quebre lógica já testada — é o que torna o toolkit "mesmo toolset, local e prod". A prod vai plugar uma CA real (PKI da LNET ou equivalente) na mesma interface.

**Independent Test**: Pode ser testado confirmando que o stub de produção implementa a mesma interface que a implementação local e que o factory retorna o tipo correto com base na URI `certSource:` do manifesto.

**Acceptance Scenarios**:

1. **Given** um manifesto com URI `ca://lnet-pki` (produção), **When** o factory é invocado, **Then** o stub de produção é retornado — todos os métodos retornam `ErrNotImplemented` de forma informativa, sem pânico.
2. **Given** uma URI `certSource:` malformada (sem prefixo reconhecido), **When** o factory tenta instanciar o provedor, **Then** um erro descritivo é retornado antes que qualquer operação criptográfica ocorra.

---

### Edge Cases

- O que acontece quando `IssueLeafCert` recebe um CSR com chave pública de algoritmo não suportado (ex: RSA em vez de ECDSA P-256)?
- O que acontece quando `GetTrustAnchor` é chamado antes de qualquer spoke ser inicializado (estado zero)?
- O que acontece quando `IssueLeafCert` é chamado concorrentemente para o mesmo `spokeId` com CSRs diferentes?
- Qual é a validade padrão do certificado folha emitido? Quem define?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A interface `CertSource` MUST expor exatamente dois métodos: `IssueLeafCert(ctx, csr []byte, spokeID string) (cert []byte, err error)` e `GetTrustAnchor(ctx context.Context, spokeID string) (caCert []byte, err error)`.
- **FR-002**: `IssueLeafCert` MUST rejeitar CSRs com `OU` diferente de `ROLE_COMMERCIAL_BANK` com `ErrForbiddenRole`.
- **FR-003**: `IssueLeafCert` MUST rejeitar CSRs com algoritmo de chave diferente de ECDSA P-256 com `ErrUnsupportedKeyAlgorithm`.
- **FR-004**: A implementação local MUST gerar a CA do spoke em memória na primeira chamada que referencia um `spokeID`; a chave privada da CA MUST nunca aparecer em arquivo, log, variável de ambiente ou manifesto.
- **FR-005**: `GetTrustAnchor` MUST retornar `ErrTrustAnchorNotFound` quando o `spokeID` nunca foi inicializado.
- **FR-006**: A implementação local MUST ser thread-safe (chamadas concorrentes para o mesmo ou diferente `spokeID` não devem causar data races).
- **FR-007**: O factory `New(uri string) (CertSource, error)` MUST aceitar o prefixo `self-signed://` para instanciar a implementação local e qualquer outra URI com prefixo `ca://` para instanciar o stub de produção.
- **FR-008**: O stub de produção MUST retornar `ErrNotImplemented` em todos os métodos (implementação Fase 4).
- **FR-009**: A validade do certificado folha emitido pela implementação local MUST ser configurável via opção (padrão: 1 ano).

### Key Entities

- **CertSource**: Interface Go que abstrai a emissão de certificados folha e recuperação de âncoras de confiança. Análoga à interface `KeyProvider` de TK-2.
- **LocalCertSource**: Implementação in-memory da interface. Mantém um mapa `spokeID → CA (chave privada + cert)` em memória. Cada spoke recebe sua própria CA gerada na primeira referência ao `spokeID`.
- **prodCertSource**: Stub de produção. Implementa a interface; todos os métodos retornam `ErrNotImplemented`.
- **CA do Spoke**: Par (chave privada ECDSA P-256 + certificado X.509 self-signed) gerado pelo banco central para um spoke específico. Nunca serializado para disco pela implementação local.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `go test ./scenario-a/toolkit/engine/certsource/...` passa com 100% dos testes, incluindo cenários de erro e casos de concorrência (race detector habilitado: `go test -race`).
- **SC-002**: O certificado emitido por `IssueLeafCert` é verificável via `x509.Certificate.Verify` usando o pool de CAs retornado por `GetTrustAnchor`.
- **SC-003**: Nenhum arquivo `*-ca.key` ou `*-ca.crt` é criado pela implementação local durante a execução dos testes.
- **SC-004**: A implementação local pode ser instanciada e utilizada pelo motor de orquestração (TK-5) sem dependência de serviço externo.
- **SC-005**: O factory `New` retorna o tipo correto para `self-signed://local` e para `ca://qualquer-coisa`, e retorna erro para URI malformada.

## Assumptions

- Go 1.26+ com `crypto/x509` stdlib — sem dependências externas para PKI na implementação local.
- `github.com/ethereum/go-ethereum v1.17.1` já está em `toolkit/go.mod` (adicionado em TK-2 / spec 024); não é necessário para `certSource` (PKI usa stdlib), mas o módulo já está configurado.
- A chave privada da CA do spoke é gerada em memória na implementação local; não há requisito de persistência entre reinicializações — comportamento análogo ao `LocalKeyProvider`.
- O `spokeID` é um identificador de string opaco (ex: `"spoke-brl"`); o factory e a implementação local não validam seu formato.
- Certificados folha têm `OU=ROLE_COMMERCIAL_BANK` conforme o modelo PKI do Scenario A documentado em `concat.md` §2.
- Mobile support e frontend estão fora do escopo; esta spec é exclusivamente para o pacote Go `toolkit/engine/certsource`.
- A implementação prod (PR-2, Fase 4) está fora do escopo desta spec — apenas o stub.
