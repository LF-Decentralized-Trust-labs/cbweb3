# Data Model: TK-2 — Interface keyProvider

**Date**: 2026-06-27 | **Branch**: `024-tk2-keyprovider-interface`

## Entidades

### KeyProvider (interface)

Abstração de gerenciamento de chaves. Não possui estado persistente próprio — o estado é responsabilidade da implementação concreta.

| Operação | Entrada | Saída | Invariante |
|----------|---------|-------|------------|
| `GenerateKey` | `ctx`, `id string` | `pubkey []byte, err error` | Idempotente; chave privada nunca sai do provider |
| `Sign` | `ctx`, `id string`, `digest []byte` | `sig []byte, err error` | `digest` deve ter exatamente 32 bytes; `id` deve existir |
| `GetPublicKey` | `ctx`, `id string` | `pubkey []byte, err error` | Retorna `ErrKeyNotFound` se `id` não existe |

**Formato de `pubkey`**: 65 bytes, chave pública secp256k1 não comprimida (`0x04 || X || Y`). Padrão `gethcrypto.FromECDSAPub()`.  
**Formato de `sig`**: 65 bytes, assinatura Ethereum-compatible (`r[32] || s[32] || v[1]`). Padrão `gethcrypto.Sign()`.

---

### LocalKeyProvider (implementação concreta — in-memory)

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `mu` | `sync.RWMutex` | Protege acesso ao mapa de chaves |
| `keys` | `map[string]*ecdsa.PrivateKey` | Keystore em memória; nunca serializado |

**Ciclo de vida**: criado pelo factory; válido por toda a sessão do processo; descartado ao encerrar o processo. Sem persistência.

**Invariantes de segurança**:
- `keys` nunca é serializado para arquivo, variável de ambiente, ou log
- Nenhum método retorna `*ecdsa.PrivateKey` para chamadores externos
- `GenerateKey` usa double-check sob write-lock para garantir idempotência sob concorrência

---

### ProdKeyProvider (stub — implementação futura)

Struct vazia. Todos os métodos retornam `ErrNotImplemented`. Slot reservado para PR-1 (Fase 4).

---

## Erros Sentinela

| Constante | Significado |
|-----------|-------------|
| `ErrKeyNotFound` | Nenhuma chave gerada para o `id` solicitado |
| `ErrNotImplemented` | Operação não implementada (stub de produção) |
| `ErrInvalidDigest` | Payload para `Sign` com tamanho diferente de 32 bytes |

---

## Factory

### Mapeamento URI → Implementação

| URI | Implementação retornada |
|-----|------------------------|
| `kms://local-emulator` | `*LocalKeyProvider` (nova instância, keystore vazio) |
| `kms://<qualquer outro>` | `*ProdKeyProvider` (stub) |
| Qualquer string sem prefixo `kms://` | `error` descritivo |
| String vazia | `error` descritivo |

---

## Helper EVMAddress

Derivação determinística da chave pública para endereço EVM:

```
EVMAddress(pubkey []byte) → "0x<checksum-address>"
```

Fórmula: `Keccak256(pubkey[1:])[12:]` formatado com EIP-55 checksum.  
Implementação via `gethcrypto.PubkeyToAddress(*ecdsa.PublicKey).Hex()`.

---

## Transições de Estado (LocalKeyProvider)

```
ID desconhecido
    │
    ├─ GenerateKey(id) ──────────────────→ ID com chave gerada
    │                                           │
    ├─ GetPublicKey(id) → ErrKeyNotFound        ├─ GenerateKey(id) → mesma pubkey (idempotente)
    └─ Sign(id, _)     → ErrKeyNotFound         ├─ GetPublicKey(id) → pubkey
                                                └─ Sign(id, digest) → sig (se |digest|==32)
```

---

## Contrato da Interface (para callers)

O caller (motor de orquestração, TK-5) interage com `KeyProvider` da seguinte forma:

1. Factory instancia o provedor a partir de `manifest.Spec.KeyProvider`
2. `GenerateKey(ctx, participantID)` → obtém pubkey → deriva endereço EVM para registro no IdentityRegistry
3. `Sign(ctx, participantID, sha256Hash)` → obtém assinatura → usa no proof-of-possession do onboarding
4. O provedor é passado como dependência para o motor; o motor nunca acessa `*ecdsa.PrivateKey`
