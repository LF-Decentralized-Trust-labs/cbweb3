# Data Model: TK-3 — Interface certSource

**Feature**: `025-tk3-certsource-interface`  
**Date**: 2026-06-27

---

## Entidades

### `CertSource` (interface Go)

Interface pública do pacote `certsource`. Define o contrato de emissão de certificados folha e recuperação de âncoras de confiança. Análoga a `KeyProvider` do TK-2.

```go
type CertSource interface {
    IssueLeafCert(ctx context.Context, csrPEM []byte, spokeID string) (certPEM []byte, err error)
    GetTrustAnchor(ctx context.Context, spokeID string) (caCertPEM []byte, err error)
}
```

**Campos / Métodos**:

| Método | Entrada | Saída | Notas |
|--------|---------|-------|-------|
| `IssueLeafCert` | `csrPEM []byte` — CSR PKCS#10 PEM; `spokeID string` — identificador do spoke | `certPEM []byte` — cert X.509 PEM assinado pela CA do spoke | Gera CA do spoke na primeira chamada (idempotente). |
| `GetTrustAnchor` | `spokeID string` | `caCertPEM []byte` — cert CA PEM (self-signed na impl. local) | Retorna `ErrTrustAnchorNotFound` se spoke nunca inicializado. |

---

### `LocalCertSource` (struct Go — implementação local)

Implementação em memória da interface `CertSource`. Mantém um mapa de `spokeID → *spokeCert` protegido por `sync.RWMutex`. A CA de cada spoke é gerada na primeira referência ao `spokeID` e permanece exclusivamente em memória.

```go
type LocalCertSource struct {
    mu           sync.RWMutex
    spokes       map[string]*spokeCert
    leafValidity time.Duration   // default: 365 * 24 * time.Hour
}
```

**Invariantes**:
- `c.spokes` é nunca `nil` após `NewLocalCertSource()`.
- `c.spokes[spokeID]` é criado exatamente uma vez por `spokeID`, mesmo sob concorrência (double-check locking).
- Nenhum campo de `spokeCert` é serializado para disco, log, ou variável de ambiente.

---

### `spokeCert` (tipo interno — não exportado)

Par chave privada + certificado CA de um spoke específico. Nunca sai do processo.

```go
type spokeCert struct {
    key  *ecdsa.PrivateKey   // chave privada ECDSA P-256 da CA
    cert *x509.Certificate   // certificado X.509 self-signed da CA
}
```

**Atributos do certificado CA**:
- `IsCA = true`, `BasicConstraintsValid = true`
- `KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign`
- `Subject.CN = "cbweb3-ca-{spokeID}"`
- `SerialNumber` — número aleatório de 128 bits
- `NotBefore = now`, `NotAfter = now + 10 anos`

---

### `prodCertSource` (struct Go — stub de produção)

Implementa `CertSource`; todos os métodos retornam `ErrNotImplemented`. Instanciada pelo factory para qualquer URI com prefixo `ca://`.

```go
type prodCertSource struct{}
var _ CertSource = (*prodCertSource)(nil)
```

---

## Erros Sentinela

| Erro | Quando ocorre |
|------|---------------|
| `ErrTrustAnchorNotFound` | `GetTrustAnchor` chamado para `spokeID` nunca inicializado |
| `ErrForbiddenRole` | CSR não contém `OU=ROLE_COMMERCIAL_BANK` |
| `ErrUnsupportedKeyAlgorithm` | Chave pública do CSR não é ECDSA P-256 |
| `ErrInvalidCSR` | CSR PEM malformado, DER inválido, ou assinatura do CSR inválida |
| `ErrNotImplemented` | Qualquer método do stub de produção |

---

## Fluxo: `IssueLeafCert` (implementação local)

```
csrPEM → pem.Decode → x509.ParseCertificateRequest
       → csr.CheckSignature()
       → validar PublicKeyAlgorithm == ECDSA P-256
       → validar OU contains "ROLE_COMMERCIAL_BANK"
       → double-check: obter ou criar spokeCert para spokeID
       → criar leafTemplate (Subject = csr.Subject, NotAfter = now + leafValidity)
       → x509.CreateCertificate(rand.Reader, leafTemplate, caCert, csr.PublicKey, caKey)
       → pem.EncodeToMemory
       → retornar certPEM
```

---

## Fluxo: `GetTrustAnchor` (implementação local)

```
spokeID → RLock → lookup c.spokes[spokeID]
        → se não encontrado: retornar ErrTrustAnchorNotFound
        → pem.EncodeToMemory(&pem.Block{Type:"CERTIFICATE", Bytes: sc.cert.Raw})
        → retornar caCertPEM
```

---

## Factory — mapeamento URI → implementação

| Valor `certSource` | Tipo instanciado |
|-----------|-----------------|
| `self-signed` (canônico, forma simples) | `*LocalCertSource` via `NewLocalCertSource()` |
| `self-signed://<x>` (forma com esquema, tolerada) | `*LocalCertSource` via `NewLocalCertSource()` |
| `ca://<x>` | `*prodCertSource` (stub) |
| qualquer outro | `error` descritivo |

---

## Transições de Estado — `LocalCertSource`

```
[spokeID desconhecido]
      │
      ▼
IssueLeafCert ou GetTrustAnchor chamado
      │
      ├── IssueLeafCert: initSpoke (gera CA in-memory) → [CA existe] → assina CSR → retorna cert
      │
      └── GetTrustAnchor: retorna ErrTrustAnchorNotFound
                          (spoke não é inicializado implicitamente por GetTrustAnchor)

[CA existe para spokeID]
      │
      ├── IssueLeafCert: assina CSR com CA existente (não regera CA)
      └── GetTrustAnchor: retorna PEM do cert CA
```

**Invariante**: `GetTrustAnchor` NUNCA inicializa a CA de um spoke. Apenas `IssueLeafCert` o faz (porque a CA é criada sob demanda quando a primeira emissão ocorre). Isto evita criação silenciosa de material criptográfico por uma operação de leitura.

---

## Relação com outros pacotes do toolkit

| Pacote | Relação com `certsource` |
|--------|--------------------------|
| `engine/keyprovider` | Padrão de referência (interface + local + prod + factory). Sem dependência de código. |
| `engine/pki` | Stubs para fluxo de banco comercial (TK-9). `certsource` é o CB-side; `pki/csr.go` é o bank-side. |
| `engine/genesis` | Sem dependência; fluxo de orquestração independente. |
| TK-5 (motor `mode: found`) | Importará `certsource` para chamar `IssueLeafCert` no onboarding do banco comercial. |
| TK-6 (join bundle) | Importará `certsource` para chamar `GetTrustAnchor` e incluir o trust anchor no bundle. |
| TK-9 (motor `mode: join`) | Importará `engine/pki` para gerar o CSR; o endpoint do CB recebe o CSR e usa `certsource`. |
