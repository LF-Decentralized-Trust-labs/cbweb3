# Quickstart — TK-B2/B3 (KeyProvider + CertSource)

Como o motor (fases futuras) e os testes consomem as duas interfaces. Tudo é local/in-memory
nesta fase; produção é stub (`ErrNotImplemented`).

## KeyProvider (chaves blockchain por entidade)

```go
kp, _ := keyprovider.New("kms://local-emulator")

pub, _ := kp.GenerateKey(ctx, "bank-a")     // idempotente
addr, _ := keyprovider.EVMAddress(pub)        // 0x... para genesis alloc / grants

sig, _ := kp.Sign(ctx, "bank-a", digest32)    // assina tx Besu/QBFT

// local-only: caminho de assinatura Besu do backend em dev
if exp, ok := kp.(keyprovider.LocalKeyExporter); ok {
    hex, _ := exp.ExportPrivateKeyHex("bank-a")
    _ = hex
}
```

- Ids distintos ⇒ endereços distintos (isolamento por entidade).
- `kms://<prod>` retorna um provider cujos métodos dão `ErrNotImplemented`.

## CertSource (CB-as-CA do spoke)

```go
cs, _ := certsource.New("self-signed")

// banco gera par + CSR (P-256, OU=ROLE_COMMERCIAL_BANK)
keyPath, csrPath, _ := pki.GenerateBankCSR("bank-a", "Bank A S.A.", outDir)
csrPEM, _ := os.ReadFile(csrPath)

leaf, _ := cs.IssueLeafCert(ctx, csrPEM, "spoke-a")   // cria CA do spoke sob demanda
ca, _ := cs.GetTrustAnchor(ctx, "spoke-a")            // âncora de confiança (só cert)
```

- CSR não-P-256 → `ErrUnsupportedKeyAlgorithm`; OU errada → `ErrForbiddenRole`;
  CSR corrompido → `ErrInvalidCSR`.
- `GetTrustAnchor` de spoke sem CA → `ErrTrustAnchorNotFound` (não cria CA).
- A chave da CA **nunca** aparece em disco/log; só o cert é exposto.

## Rodar os testes

```bash
cd scenario-b/toolkit
go test ./engine/keyprovider/... ./engine/certsource/... ./engine/pki/...
```

Cobre: gerar/assinar/verificar + endereço EVM; emitir leaf a partir de CSR + rejeições tipadas;
factories (local vs prod stub); asserção "sem segredos" (nenhum material privado serializável
sai das interfaces).
