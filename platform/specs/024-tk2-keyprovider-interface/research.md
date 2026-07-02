# Research: TK-2 — Interface keyProvider

**Date**: 2026-06-27 | **Branch**: `024-tk2-keyprovider-interface`

## Decisões

### 1. Linguagem e localização

**Decision**: Go; pacote `scenario-a/toolkit/engine/keyprovider/`  
**Rationale**: O toolkit já é um módulo Go isolado (`scenario-a/toolkit/go.mod`). TypeScript não tem presença no toolkit. A localização `engine/keyprovider/` é paralela a `engine/manifest/`, `engine/genesis/` e `engine/pki/` — coerente com a arquitetura existente.  
**Alternatives considered**: TypeScript foi descartado por ausência de precedente no toolkit; um pacote separado (`toolkit/keyprovider/`) foi descartado por quebrar a convenção `engine/` já estabelecida.

---

### 2. Biblioteca criptográfica

**Decision**: `github.com/ethereum/go-ethereum/crypto` (importado como `gethcrypto`)  
**Rationale**: Já é dependência padrão no Scenario A em múltiplos módulos (`backend/shared/blockchain/go.mod v1.17.1`, `backend/services/auth/go.mod`, `backend/services/payment-orchestrator/go.mod`). A implementação local do auth service (`auth/internal/kms/providers/local.go`) já usa `gethcrypto.GenerateKey()`, `gethcrypto.Sign()`, e `gethcrypto.PubkeyToAddress()` — exatamente o que precisamos.  
**Alternatives considered**: `github.com/decred/dcrd/dcrec/secp256k1/v4` — mais leve, mas não está no projeto; introduziria nova dependência não versionada pelo time. `crypto/ecdsa` com `elliptic.P256()` — biblioteca padrão, mas P-256 é a curva errada para EVM (secp256k1).

**Versão**: `v1.17.1` — mantida igual aos módulos backend para consistência de supply chain.

---

### 3. Implementação local — padrão de referência

**Decision**: Espelhar `backend/services/auth/internal/kms/providers/local.go`  
**Rationale**: Esse arquivo já implementa em produção o mesmo padrão que precisamos: map `id → *ecdsa.PrivateKey`, `sync.RWMutex`, double-check-under-write-lock para idempotência, `gethcrypto.Sign()` para assinatura de digest de 32 bytes.  
**Alternatives considered**: `sync.Map` — eliminaria o mutex explícito, mas o double-check de idempotência sob write-lock é mais claro com `sync.RWMutex` e esse é o padrão já estabelecido.

---

### 4. Parse de URI do keyProvider

**Decision**: Parse mínimo baseado em `strings.HasPrefix(uri, "kms://")` + extração do host  
**Rationale**: A spec define apenas dois casos para TK-2: `kms://local-emulator` (local) e qualquer outro valor `kms://` (stub prod). Parse de URL completa com `net/url` seria overkill para esse escopo e introduziria validações extras fora do escopo de TK-2.  
**Alternatives considered**: `net/url.Parse()` — mais robusto, mas `net/url` já é stdlib; pode ser adotado em PR-1 quando o prod precisa parsear host:port real.

---

### 5. Stub de produção

**Decision**: `prodKeyProvider` struct vazia com todos os métodos retornando `ErrNotImplemented`  
**Rationale**: Mesmo padrão de `auth/internal/kms/providers/aws.go` e de `engine/pki/csr.go` já existente no toolkit. O contrato da interface é o que importa agora; a implementação real vem na Fase 4 (PR-1).  
**Alternatives considered**: Panic — rejeitado por violar a regra de não panic em produção.

---

### 6. Assinatura de contexto das operações

**Decision**: Todos os métodos aceitam `context.Context` como primeiro argumento  
**Rationale**: Padrão estabelecido no auth service e nas boas práticas Go para operações potencialmente bloqueantes ou com I/O. O emulador local não usa o context ativamente, mas o respeita para cancelamento — sem mudar a assinatura quando PR-1 adicionar a implementação de KMS real (que vai precisar de context para chamadas de rede).  
**Alternatives considered**: Sem context — mais simples para o emulador local, mas quebraria a assinatura na Fase 4 sem um refactor.

---

### 7. Helper EVMAddress

**Decision**: Função `EVMAddress(pubkey []byte) (string, error)` no pacote `keyprovider`  
**Rationale**: Callers que precisam registrar o participante no IdentityRegistry precisam do endereço EVM derivado da chave pública. Centralizar essa derivação no pacote evita reimplementações e garante a fórmula correta: `Keccak256(pubkey[1:])[12:]` formatado como checksum address via `gethcrypto.PubkeyToAddress()`.  
**Alternatives considered**: Deixar para o caller calcular — rejeitado porque a derivação correta (offset [1:] para pular o prefixo `0x04`) é um detalhe não óbvio que seria reimplementado erroneamente.

---

## Sem Unknowns Abertos

Todos os NEEDS CLARIFICATION foram resolvidos pelos padrões existentes no repositório. Nenhuma pesquisa externa necessária.
