# Implementation Plan: TK-3 — Interface certSource

**Branch**: `025-tk3-certsource-interface` | **Date**: 2026-06-27 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `specs/025-tk3-certsource-interface/spec.md`

---

## Summary

Criar a interface Go `CertSource` e suas duas implementações para o provisioning toolkit do Scenario A: (1) `LocalCertSource` — CA por spoke gerada em memória, self-signed, que assina CSRs de bancos comerciais; e (2) `prodCertSource` — stub que retorna `ErrNotImplemented` em todos os métodos (implementado na Fase 4, PR-2). O factory `New(uri string)` instancia o tipo correto a partir do campo `certSource:` do manifesto YAML. Nenhuma dependência externa nova: implementação usa exclusivamente `crypto/x509`, `crypto/ecdsa`, e `crypto/rand` da stdlib Go.

---

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Go stdlib apenas (`crypto/x509`, `crypto/ecdsa`, `crypto/elliptic`, `crypto/rand`, `encoding/pem`, `sync`, `context`)  
**Storage**: Nenhum — implementação local usa `map[string]*spokeCert` em memória; a chave privada da CA do spoke nunca é serializada para disco  
**Testing**: `go test -race ./scenario-a/toolkit/engine/certsource/...` (race detector obrigatório; testes RED antes da implementação)  
**Target Platform**: Linux (local dev) e staging/prod (mesma interface, implementação é parâmetro)  
**Project Type**: Pacote biblioteca dentro do módulo Go standalone do toolkit  
**Performance Goals**: `IssueLeafCert` e `GetTrustAnchor` devem completar em < 50 ms (operações RSA/ECDSA em memória)  
**Constraints**: Zero dependências externas novas; chave privada da CA nunca em arquivo, log, ou variável; thread-safe para chamadas concorrentes com o mesmo `spokeID`

---

## Constitution Check

*GATE: Verificado 2026-06-27. Deve passar antes da Fase 0.*

| Princípio | Status | Notas |
|-----------|--------|-------|
| I. Scenario-Scoped Independence | ✅ PASS | Código exclusivamente em `scenario-a/toolkit/engine/certsource/`. Nenhum código compartilhado com Scenario B. |
| II. Privacy by Design | ✅ PASS (N/A) | PKI/TLS é camada de transporte e autenticação, não transferência de valor on-chain. Não envolve `ZetoToken`, `NotoToken` ou `tCeBM`. |
| III. Atomic Settlement Guarantee | ✅ PASS (N/A) | Nenhum envolvimento com protocolo de liquidação. Operação de provisionamento pré-spoke. |
| IV. Compliance Gate Before Participation | ✅ PASS (N/A) | Opera abaixo da camada de identidade/compliance. O `IssueLeafCert` é parte do fluxo de onboarding, não do fluxo de pagamento. |
| V. Test-First at Every Layer | ✅ PASS | `local_test.go` com testes failing (RED) é a Fase 1, Passo 1. Implementação é o Passo 2. Race detector habilitado. |
| VI. Observability and Auditability | ✅ PASS (N/A) | Pacote biblioteca; sem serviço autônomo. Erros descritivos propagados para cima pelo motor de orquestração que já tem logging estruturado. |

**Todos os gates passam. Nenhuma entrada em Complexity Tracking necessária.**

*Re-check pós-design (Fase 1 completa)*: Mesmo resultado. Nenhuma tecnologia nova; sem cross-scenario concern.

---

## Project Structure

### Documentation (this feature)

```text
specs/025-tk3-certsource-interface/
├── plan.md              ← este arquivo
├── research.md          ← Fase 0 output
├── data-model.md        ← Fase 1 output
└── tasks.md             ← Fase 2 output (/speckit.tasks — ainda não criado)
```

### Source Code

```text
scenario-a/toolkit/
├── go.mod                               (existente — sem mudanças; stdlib PKI não requer deps)
└── engine/
    ├── keyprovider/                     (existente — TK-2; padrão de referência)
    │   ├── keyprovider.go
    │   ├── local.go
    │   ├── prod.go
    │   ├── factory.go
    │   └── local_test.go
    ├── pki/                             (existente — stubs TK-9; não modificado)
    │   ├── csr.go
    │   ├── csr_test.go
    │   └── helpers_test.go
    ├── genesis/                         (existente — TK-guard; não modificado)
    │   ├── guard.go
    │   └── guard_test.go
    └── certsource/                      (NOVO — esta feature)
        ├── certsource.go                interface + erros sentinela
        ├── local.go                     LocalCertSource (in-memory, self-signed CA por spoke)
        ├── prod.go                      prodCertSource stub (ErrNotImplemented)
        ├── factory.go                   New(uri string) factory
        ├── local_test.go                testes RED → GREEN
        └── helpers_test.go              helpers: geração de CSR de teste + verificação de cert
```

Nenhum outro diretório é tocado. Os scripts de amostra em `deploy/local/spoke-besu-*/startBesu.sh` e os targets de `make/05-pki.mk` são preservados integralmente.

**Structure Decision**: Pacote único `engine/certsource` dentro do módulo Go existente. Espelha exatamente o padrão `engine/keyprovider` estabelecido em TK-2 e `engine/genesis` de 017.

---

## Implementation Phases

### Phase 1 — Testes de Contrato (RED)

**Arquivo**: `scenario-a/toolkit/engine/certsource/local_test.go` e `helpers_test.go`

Escrever todos os testes **antes** de qualquer código de implementação. Todos devem falhar com "not implemented" neste passo.

**`helpers_test.go`** — funções reutilizáveis:
- `generateTestCSR(t, commonName, ou string) []byte` — gera par ECDSA P-256 + CSR PKCS#10 PEM; `OU` configurável
- `verifyLeafCert(t, certPEM, caCertPEM []byte, cn string)` — verifica assinatura do cert folha contra a CA e valida `CN`

**`local_test.go`** — casos de teste (seguindo o estilo de `local_test.go` do keyprovider):

1. **`TestIssueLeafCert_ValidCSR_IsVerifiableByCACert`** — CSR válido com `OU=ROLE_COMMERCIAL_BANK`; cert emitido é verificável pelo trust anchor retornado por `GetTrustAnchor`.
2. **`TestIssueLeafCert_MalformedPEM_ReturnsError`** — PEM inválido retorna erro; nenhum material criptográfico criado.
3. **`TestIssueLeafCert_ForbiddenRole_ReturnsErrForbiddenRole`** — CSR com `OU=ROLE_CENTRAL_BANK` retorna `ErrForbiddenRole`.
4. **`TestIssueLeafCert_UnsupportedKeyAlgorithm_ReturnsError`** — CSR com chave RSA retorna `ErrUnsupportedKeyAlgorithm`.
5. **`TestIssueLeafCert_InvalidCSRSignature_ReturnsError`** — CSR com assinatura corrompida retorna `ErrInvalidCSR`.
6. **`TestGetTrustAnchor_UnknownSpokeID_ReturnsErrNotFound`** — `spokeID` nunca inicializado retorna `ErrTrustAnchorNotFound`.
7. **`TestGetTrustAnchor_AfterIssue_ReturnsSelfSignedCACert`** — após `IssueLeafCert`, `GetTrustAnchor` retorna cert self-signed com `IsCA=true`.
8. **`TestGetTrustAnchor_Idempotent`** — duas chamadas retornam PEM byte-a-byte idêntico.
9. **`TestIssueLeafCert_MultipleCalls_SameCA`** — dois certs emitidos para o mesmo spoke são ambos verificáveis pelo mesmo trust anchor.
10. **`TestIssueLeafCert_DifferentSpokes_DifferentCAs`** — trust anchors de dois `spokeID` distintos são certificados diferentes.
11. **`TestLocalCertSource_RaceCondition`** — 20 goroutines chamando `IssueLeafCert` e `GetTrustAnchor` concorrentemente para o mesmo `spokeID`; run com `-race`.
12. **`TestFactory_SelfSignedURI_ReturnsLocalCertSource`** — `New("self-signed://local")` retorna implementação funcional.
13. **`TestFactory_ProdURI_ReturnsStubWithErrNotImplemented`** — `New("ca://lnet-pki")` retorna stub; todos os métodos retornam `ErrNotImplemented`.
14. **`TestFactory_InvalidURI_ReturnsError`** — URI sem prefixo reconhecido retorna erro descritivo.
15. **`TestLocalCertSource_NoPrivateKeyFile`** — após execução completa do teste, nenhum arquivo `*.key` ou `*-ca.*` existe no diretório de trabalho.

Verificar RED: `go test ./engine/certsource/... 2>&1` deve mostrar falhas "not implemented".

### Phase 2 — Definição de Contrato (stub)

**Arquivo**: `scenario-a/toolkit/engine/certsource/certsource.go`

Definir a interface e erros sentinela:

```go
// Package certsource defines the PKI certificate issuance abstraction for the
// Scenario A provisioning toolkit. The Central Bank CA signs CSRs from commercial
// banks; no commercial bank ever holds a CA keypair.
// The local implementation generates a self-signed CA per spoke in process memory;
// the CA private key is never written to disk, logs, or environment variables.
package certsource

import (
    "context"
    "errors"
)

// CertSource is the PKI boundary for spoke certificate issuance.
type CertSource interface {
    // IssueLeafCert signs csrPEM (PKCS#10, PEM-encoded) with the CA of spokeID
    // and returns the signed certificate (PEM-encoded, X.509).
    // The local implementation generates the spoke CA on first use (idempotent).
    // Returns ErrForbiddenRole if the CSR subject OU is not ROLE_COMMERCIAL_BANK.
    // Returns ErrUnsupportedKeyAlgorithm if the CSR public key is not ECDSA P-256.
    // Returns ErrInvalidCSR if the CSR PEM is malformed or the signature is invalid.
    IssueLeafCert(ctx context.Context, csrPEM []byte, spokeID string) (certPEM []byte, err error)

    // GetTrustAnchor returns the CA certificate (PEM-encoded) for spokeID.
    // Returns ErrTrustAnchorNotFound if no CA has been generated for spokeID yet.
    GetTrustAnchor(ctx context.Context, spokeID string) (caCertPEM []byte, err error)
}

var (
    ErrTrustAnchorNotFound      = errors.New("certsource: trust anchor not found for spoke")
    ErrForbiddenRole            = errors.New("certsource: CSR OU must be ROLE_COMMERCIAL_BANK")
    ErrUnsupportedKeyAlgorithm  = errors.New("certsource: CSR public key must be ECDSA P-256")
    ErrInvalidCSR               = errors.New("certsource: CSR is malformed or has invalid signature")
    ErrNotImplemented           = errors.New("certsource: not implemented")
)
```

**Arquivo**: `scenario-a/toolkit/engine/certsource/local.go` (stub)

```go
type LocalCertSource struct { /* ... */ }
func NewLocalCertSource() *LocalCertSource { ... }
func (c *LocalCertSource) IssueLeafCert(_ context.Context, _ []byte, _ string) ([]byte, error) {
    return nil, errors.New("not implemented: IssueLeafCert")
}
func (c *LocalCertSource) GetTrustAnchor(_ context.Context, _ string) ([]byte, error) {
    return nil, errors.New("not implemented: GetTrustAnchor")
}
```

**Arquivo**: `scenario-a/toolkit/engine/certsource/prod.go` — stub análogo ao `keyprovider/prod.go`:

```go
type prodCertSource struct{}
var _ CertSource = (*prodCertSource)(nil)
func (p *prodCertSource) IssueLeafCert(_ context.Context, _ []byte, _ string) ([]byte, error) {
    return nil, ErrNotImplemented
}
func (p *prodCertSource) GetTrustAnchor(_ context.Context, _ string) ([]byte, error) {
    return nil, ErrNotImplemented
}
```

**Arquivo**: `scenario-a/toolkit/engine/certsource/factory.go`:

```go
const schemePrefix = "self-signed://"
const prodSchemePrefix = "ca://"

func New(uri string) (CertSource, error) {
    switch {
    case strings.HasPrefix(uri, schemePrefix):
        return NewLocalCertSource(), nil
    case strings.HasPrefix(uri, prodSchemePrefix):
        return &prodCertSource{}, nil
    default:
        return nil, fmt.Errorf("certsource: invalid URI %q: must begin with %q or %q", uri, schemePrefix, prodSchemePrefix)
    }
}
```

Testes permanecem RED. Esta fase separa definição de contrato de implementação.

### Phase 3 — Implementação (GREEN)

**Arquivo**: `scenario-a/toolkit/engine/certsource/local.go` (implementação completa)

**Tipo interno**:
```go
type spokeCert struct {
    key  *ecdsa.PrivateKey
    cert *x509.Certificate
}

type LocalCertSource struct {
    mu           sync.RWMutex
    spokes       map[string]*spokeCert
    leafValidity time.Duration
}

func NewLocalCertSource() *LocalCertSource {
    return &LocalCertSource{
        spokes:       make(map[string]*spokeCert),
        leafValidity: 365 * 24 * time.Hour,
    }
}
```

**`initSpoke(spokeID string) (*spokeCert, error)`** (interno, chamado sob write-lock):
1. `ecdsa.GenerateKey(elliptic.P256(), rand.Reader)`
2. Template X.509: `IsCA=true`, `KeyUsage=KeyUsageCertSign|KeyUsageCRLSign`, `BasicConstraintsValid=true`, `SerialNumber` aleatório, `Subject.CN=cbweb3-ca-{spokeID}`
3. `x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)` — self-signed
4. `x509.ParseCertificate(der)` para obter `*x509.Certificate`
5. Armazenar em `c.spokes[spokeID]`

**`IssueLeafCert`**:
1. `pem.Decode(csrPEM)` → se nil, retornar `ErrInvalidCSR`
2. `x509.ParseCertificateRequest(block.Bytes)` → se erro, retornar `ErrInvalidCSR`
3. `csr.CheckSignature()` → se erro, retornar `ErrInvalidCSR`
4. Verificar `csr.PublicKeyAlgorithm == x509.ECDSA` e curva P-256 → senão `ErrUnsupportedKeyAlgorithm`
5. Verificar que algum `csr.Subject.OrganizationalUnit` == `"ROLE_COMMERCIAL_BANK"` → senão `ErrForbiddenRole`
6. Double-check sob write-lock para obter ou criar spoke CA (padrão idêntico ao `LocalKeyProvider.GenerateKey`)
7. Criar template de cert folha: `Subject` = `csr.Subject`, `NotBefore=now`, `NotAfter=now+leafValidity`, `SerialNumber` aleatório, `IsCA=false`
8. `x509.CreateCertificate(rand.Reader, leafTemplate, caCert, csr.PublicKey, caKey)`
9. PEM encode e retornar

**`GetTrustAnchor`**:
1. Read lock
2. Lookup `c.spokes[spokeID]` → se não encontrado, retornar `ErrTrustAnchorNotFound`
3. `pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: sc.cert.Raw})` e retornar

Executar: `go test -race ./engine/certsource/... -v` — todos os 15 testes devem passar GREEN.

### Phase 4 — Documentação de Ponto de Integração

**Arquivo**: `scenario-a/toolkit/engine/certsource/certsource.go` (atualizar comentário de package)

Adicionar comentário de pacote documentando onde no fluxo de orquestração `IssueLeafCert` e `GetTrustAnchor` são chamados:

> O motor de orquestração DEVE chamar `GetTrustAnchor` após `mode: found` completar,
> para incluir o trust anchor no join bundle (TK-6).
> O motor DEVE chamar `IssueLeafCert` no fluxo `mode: join` (TK-9), após receber o CSR
> do banco comercial via endpoint do relay/CB.
> A chave privada da CA do spoke permanece em memória e é destruída quando o processo encerra;
> o trust anchor (cert público) é o único material que sai do processo.

---

## Out of Scope

- O motor de orquestração que chama `IssueLeafCert`/`GetTrustAnchor` (TK-5, TK-9) — PRs separados.
- O emissor do join bundle que embute o trust anchor (TK-6) — PR separado que importa este pacote.
- A implementação de produção da CA real (PKI da LNET) — PR-2, Fase 4.
- O fluxo de `SubmitCSRToCB` do banco comercial — stubs existentes em `engine/pki/csr.go` (TK-9).
- A persistência da CA do spoke entre reinicializações — fora do escopo da implementação local.
- Os scripts `deploy/local/spoke-besu-*/startBesu.sh` e targets `make/05-pki.mk` — preservados sem modificação.
