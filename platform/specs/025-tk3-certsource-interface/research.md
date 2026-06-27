# Research: TK-3 — Interface certSource

**Feature**: `025-tk3-certsource-interface`  
**Date**: 2026-06-27  
**Status**: Complete — todas as questões resolvidas; nenhum NEEDS CLARIFICATION aberto.

---

## Questões investigadas

### 1. Emissão de certificados X.509 com stdlib Go — sem dependências externas?

**Decision**: Sim. `crypto/x509` + `crypto/ecdsa` + `crypto/rand` + `encoding/pem` cobrem o fluxo completo de geração de CA self-signed, parsing de CSR PKCS#10, e assinatura de certificado folha. `go.mod` do toolkit não recebe nenhuma nova entrada.

**Rationale**: O padrão do toolkit (TK-2 usado `go-ethereum` para ECDSA secp256k1) foi necessário porque `crypto/elliptic` da stdlib não suporta secp256k1. Para ECDSA P-256 (que é o algoritmo de cert PKI do Scenario A, conforme `onboarding_proxy.go`), a stdlib é suficiente.

**Alternatives considered**:
- `golang.org/x/crypto` — não necessário; `crypto/x509` tem tudo.
- Biblioteca de CA dedicada (e.g., `cloudflare/cfssl`) — dependência externa desnecessária para um scope this small.

---

### 2. Formato da assinatura de método: `IssueLeafCert(csr, spokeTrustAnchor)` ou `IssueLeafCert(csr, spokeID)`?

**Decision**: `IssueLeafCert(ctx context.Context, csrPEM []byte, spokeID string) (certPEM []byte, err error)`

**Rationale**: A especificação original escreveu `spokeTrustAnchor` mas o parâmetro semântico é o identificador do spoke, não o certificado CA. Na implementação local, o `LocalCertSource` é o banco central — ele já detém a CA de cada spoke internamente e a busca pelo `spokeID`. Passar o trust anchor como argumento criaria uma dependência circular (o caller teria que buscar o trust anchor para passá-lo de volta à mesma entidade que o gerou). A assinatura com `spokeID` é paralela a `GetTrustAnchor(ctx, spokeID)` e é consistente com o padrão de `LocalKeyProvider.Sign(ctx, id, digest)`.

**Alternatives considered**:
- `IssueLeafCert(ctx, csrPEM, caCertPEM, caKeyRef)` — exporia material de chave ou referência de chave na assinatura do método, violando o princípio de que a chave privada não sai do provider.

---

### 3. Como garantir que a CA do spoke é gerada exatamente uma vez mesmo com chamadas concorrentes?

**Decision**: Double-checked locking, idêntico ao padrão de `LocalKeyProvider.GenerateKey`:
1. RLock — verificar se `c.spokes[spokeID]` existe → se sim, retornar.
2. RUnlock.
3. Lock (write) — verificar novamente (TOCTOU prevention) → se ainda não existe, chamar `initSpoke`.
4. Unlock.

**Rationale**: Este padrão já é validado pelos testes do TK-2 com `-race`. Reutilizar o mesmo padrão minimiza risco de data race e mantém consistência no codebase.

---

### 4. Validade do certificado folha — quem define?

**Decision**: Campo `leafValidity time.Duration` em `LocalCertSource`, com valor padrão de `365 * 24 * time.Hour` (1 ano). Sem exposição no manifesto neste PR.

**Rationale**: O manifesto YAML ainda não tem campo de validade de cert. Hardcodar 1 ano como default é razoável para desenvolvimento local e CI. A configuração por manifesto pode ser adicionada em TK-7 (CLI apply) se necessário, sem quebrar a interface.

---

### 5. Prefixo URI do factory — `self-signed://` ou `certSource://self-signed`?

**Decision**: `self-signed://local` para a implementação local; `ca://` como prefixo para produção.

**Rationale**: Espelha a estrutura do `keyprovider` que usa `kms://local-emulator` e `kms://<backend>`. O esquema é o tipo de implementação, o host/path é o backend específico. Mantém simetria no manifesto:

```yaml
keyProvider:  kms://local-emulator
certSource:   self-signed://local
```

No prod:
```yaml
keyProvider:  kms://aws-kms
certSource:   ca://lnet-pki
```

---

### 6. Validação do `OU` no CSR — comparação exata ou contains?

**Decision**: `contains` — verificar se algum elemento de `csr.Subject.OrganizationalUnit` é igual a `"ROLE_COMMERCIAL_BANK"`. Não impor que seja o único OU.

**Rationale**: O `onboarding_proxy.go` (modo smart) constrói o CSR com `OU=ROLE_COMMERCIAL_BANK` como OU único, mas a validação defensiva não deve quebrar se um futuro fluxo adicionar OUs adicionais.

---

### 7. Como verificar que nenhum arquivo de CA é criado nos testes?

**Decision**: O teste `TestLocalCertSource_NoPrivateKeyFile` usa `filepath.Walk(os.TempDir(), ...)` + `os.TempDir()` para verificar ausência de arquivos `*.key` ou `*-ca.*` criados durante a execução. Complementarmente, o padrão `helpers_test.go` do pacote `keyprovider` já demonstra como inspecionar que a implementação é in-memory.

**Rationale**: Verificação explícita como teste formal (não apenas documentação) faz parte do Acceptance Criteria SC-003 da spec.

---

## Resumo de decisões

| Questão | Decisão |
|---------|---------|
| Dependências externas | Nenhuma — stdlib Go apenas |
| Assinatura de `IssueLeafCert` | `(ctx, csrPEM []byte, spokeID string)` |
| Thread safety | Double-check locking, padrão TK-2 |
| Validade cert folha | 1 ano (default), campo configurável em `LocalCertSource` |
| URI factory | `self-signed://local` → local; `ca://` → prod stub |
| Validação de OU | `contains("ROLE_COMMERCIAL_BANK")` |
| Verificação no-file | Teste explícito `TestLocalCertSource_NoPrivateKeyFile` |
