# Contracts — Go API (KeyProvider + CertSource + PKI)

Fase 1. Assinaturas (contrato) das interfaces plugáveis e das factories por URI.
Estas são a fronteira que o motor (TK-B5+) consome. Não há CLI nesta fase.

## Package `engine/keyprovider`

```go
// KeyProvider é a fronteira de custódia das chaves blockchain (secp256k1).
type KeyProvider interface {
    // GenerateKey cria (ou retorna, idempotente) a chave da entidade id.
    // Retorna a chave pública descomprimida (65 bytes, prefixo 0x04).
    GenerateKey(ctx context.Context, id string) ([]byte, error)

    // Sign assina um digest de 32 bytes com a chave da entidade id.
    // Retorna assinatura de 65 bytes [R||S||V].
    Sign(ctx context.Context, id string, digest []byte) ([]byte, error)

    // GetPublicKey retorna a pubkey (65 bytes) da entidade id sem gerar.
    GetPublicKey(ctx context.Context, id string) ([]byte, error)
}

// LocalKeyExporter é implementado APENAS pela impl. local.
// A impl. de produção NÃO implementa esta interface.
type LocalKeyExporter interface {
    ExportPrivateKeyHex(id string) (string, error)
}

// EVMAddress deriva o endereço EVM (0x-hex) a partir da pubkey (65 bytes).
func EVMAddress(pubkey []byte) (string, error)

// New seleciona a implementação pela URI.
//   kms://local-emulator[?seed=...]  → local (in-memory, determinística)
//   kms://<qualquer-outra>           → prod stub (ErrNotImplemented)
func New(uri string) (KeyProvider, error)

var (
    ErrNotImplemented = errors.New("keyprovider: not implemented in production stub")
    ErrKeyNotFound    = errors.New("keyprovider: key not found for id")
)
```

## Package `engine/certsource`

```go
// CertSource é a fronteira de CA do spoke (CB-as-CA).
type CertSource interface {
    // IssueLeafCert valida o CSR PKCS#10 (PEM) e emite um cert leaf (PEM)
    // assinado pela CA do spokeID. Cria a CA do spoke sob demanda.
    IssueLeafCert(ctx context.Context, csrPEM []byte, spokeID string) ([]byte, error)

    // GetTrustAnchor retorna o cert da CA (PEM) do spokeID. NÃO cria CA.
    GetTrustAnchor(ctx context.Context, spokeID string) ([]byte, error)
}

// New seleciona a implementação pela URI.
//   self-signed | self-signed://<...>  → local (CA P-256 in-memory por spoke)
//   ca://<...>                          → prod stub (ErrNotImplemented)
func New(uri string) (CertSource, error)

var (
    ErrNotImplemented          = errors.New("certsource: not implemented in production stub")
    ErrInvalidCSR              = errors.New("certsource: invalid or unpar. CSR")
    ErrUnsupportedKeyAlgorithm = errors.New("certsource: CSR key algorithm must be ECDSA P-256")
    ErrForbiddenRole           = errors.New("certsource: CSR OU must be ROLE_COMMERCIAL_BANK")
    ErrTrustAnchorNotFound     = errors.New("certsource: no CA for spoke id")
)
```

## Package `engine/pki`

```go
// GenerateBankCSR gera par ECDSA P-256 + CSR PKCS#10 para o banco.
// Escreve <outDir>/<bankCode>.key (0600) e <outDir>/<bankCode>.csr.
// Subject: CN=<bankCode>, O=<institution>, OU=ROLE_COMMERCIAL_BANK, C=BR.
// NUNCA gera material de CA.
func GenerateBankCSR(bankCode, institution, outDir string) (keyPath, csrPath string, err error)
```

## Tabela de factory (URI → implementação)

| Interface   | URI                          | Implementação | Segredos exportáveis |
|-------------|------------------------------|---------------|----------------------|
| KeyProvider | `kms://local-emulator`       | local         | sim (local-only)     |
| KeyProvider | `kms://<prod>`               | prod stub     | não                  |
| CertSource  | `self-signed` / `self-signed://…` | local    | não (só cert público)|
| CertSource  | `ca://<prod>`                | prod stub     | não                  |

## Invariantes de contrato

- Nenhum método retorna chave privada, exceto `LocalKeyExporter.ExportPrivateKeyHex` (só local).
- `Sign` exige `len(digest) == 32`.
- `IssueLeafCert` rejeita CSR não-P-256, sem OU esperada, ou assinatura inválida, com erro tipado.
- Prod stubs retornam `ErrNotImplemented` em todo método.
