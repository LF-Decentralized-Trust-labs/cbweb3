# Data Model — TK-B2/B3 (KeyProvider + CertSource)

Fase 1. Interfaces, estado interno, regras e erros. Sem persistência (tudo em memória).

## KeyProvider (fronteira de custódia de chaves blockchain)

**Interface**
- `GenerateKey(ctx, id) → pubkey65` — idempotente (mesma pubkey em re-chamada).
- `Sign(ctx, id, digest32) → sig65` — assina um digest de 32 bytes.
- `GetPublicKey(ctx, id) → pubkey65` — não gera implicitamente (erro se ausente).
- (local-only) `ExportPrivateKeyHex(id) → hex` — só na impl. local; ausente na prod.

**Estado (local)**: `map[id]*ecdsa.PrivateKey` (secp256k1), protegido por lock; derivação
determinística por id + seed base; uma chave dev semeada sob id conhecido.

**Regras**
- Chave = **secp256k1**; endereço EVM = keccak256(pubkey)[12:].
- Ids distintos ⇒ chaves/endereços distintos (isolamento por entidade).
- Chave privada **nunca** retornada por `Sign`/`GetPublicKey` nem serializada.

**Erros**: `ErrNotImplemented` (prod), erro de "id não encontrado" (GetPublicKey sem gerar).

**Factory**: `New(uri)` → `kms://local-emulator` = local semeado; outro `kms://…` = prod stub.

## CertSource (fronteira de CA do spoke)

**Interface**
- `IssueLeafCert(ctx, csrPEM, spokeID) → certPEM`.
- `GetTrustAnchor(ctx, spokeID) → caCertPEM`.

**Estado (local)**: `map[spokeID]*spokeCA{ key *ecdsa.PrivateKey (P-256), cert *x509.Certificate }`;
CA criada preguiçosamente no primeiro `IssueLeafCert`; chave da CA só em memória.

**Regras `IssueLeafCert`** (ordem):
1. PEM-decode + parse PKCS#10 + `CheckSignature` → `ErrInvalidCSR`.
2. pubkey do CSR = **ECDSA P-256** senão `ErrUnsupportedKeyAlgorithm`.
3. subject OU contém `ROLE_COMMERCIAL_BANK` senão `ErrForbiddenRole`.
4. assina leaf (serial aleatório, `KeyUsageDigitalSignature`, `ExtKeyUsage{ClientAuth,ServerAuth}`,
   `IsCA:false`, validade ~1 ano) com a chave da CA do spoke.

**Regras `GetTrustAnchor`**: retorna o cert PEM da CA; **não** cria CA; `ErrTrustAnchorNotFound`
se o spoke ainda não tem CA.

**Invariante**: a chave da CA **nunca** é escrita em disco/log/env nem serializada; só o **cert**
(âncora) é público. Rejeitar qualquer material contendo `PRIVATE KEY` ao ler/retornar âncora.

**Erros**: `ErrNotImplemented` (prod), `ErrInvalidCSR`, `ErrUnsupportedKeyAlgorithm`,
`ErrForbiddenRole`, `ErrTrustAnchorNotFound`.

**Factory**: `New(uri)` → `self-signed` | `self-signed://…` = local; `ca://…` = prod stub.

## PKI helper (para testes e fases futuras)

- `GenerateBankCSR(bankCode, institution, outDir) → (key, csr)` — gera par ECDSA P-256 + CSR com
  subject `CN=<bankCode>, O=<institution>, OU=ROLE_COMMERCIAL_BANK, C=BR`. Escreve `{bankCode}.key`
  (0600) e `{bankCode}.csr`; **nunca** cria `*-ca.*`.

## Transições de estado

Nenhuma persistida. Estado é o mapa em memória (chaves / CAs), criado sob demanda por processo.
