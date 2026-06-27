# Tasks: TK-2 — Interface keyProvider

**Input**: Design documents from `specs/024-tk2-keyprovider-interface/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅

**Tests**: **OBRIGATÓRIOS** — Princípio V da constituição exige test-first para todos os layers de backend/toolkit. O ciclo Red-Green-Refactor é estritamente exigido: teste falhando → implementação → refactor sem quebrar.

**Organization**: Tasks organizadas por User Story para implementação e teste independentes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependências incompletas)
- **[Story]**: User Story a que pertence (US1, US2, US3)

---

## Phase 1: Setup (Infraestrutura compartilhada)

**Purpose**: Adicionar dependência criptográfica e criar estrutura do pacote

- [x] T001 Adicionar `github.com/ethereum/go-ethereum v1.17.1` ao `scenario-a/toolkit/go.mod` e executar `go mod tidy` no diretório `scenario-a/toolkit/`

---

## Phase 2: Foundational (Pré-requisito bloqueante)

**Purpose**: Definir a interface Go e os erros sentinela. NADA mais pode ser implementado antes disso — todos os arquivos subsequentes dependem desses tipos.

**⚠️ CRÍTICO**: Nenhum work de User Story pode começar antes desta fase estar completa.

- [x] T002 Criar `scenario-a/toolkit/engine/keyprovider/keyprovider.go` com: interface `KeyProvider` (métodos `GenerateKey`, `Sign`, `GetPublicKey`, todos com `context.Context`), sentinel errors `ErrKeyNotFound`/`ErrNotImplemented`/`ErrInvalidDigest`, e função `EVMAddress(pubkey []byte) (string, error)` usando `gethcrypto.PubkeyToAddress`

**Checkpoint**: Interface e tipos definidos — User Stories podem começar.

---

## Phase 3: User Story 1 — Provisionar sem chaves em arquivos (Priority: P1) 🎯 MVP

**Goal**: Garantir que o motor de orquestração obtém chaves públicas e assinaturas sem nunca receber material privado. Local emulator funcional.

**Independent Test**: Executar `go test ./engine/keyprovider/... -run "TestLocalKeyProvider_GenerateKey|TestLocalKeyProvider_Sign|TestEVMAddress"` e verificar que (a) nenhum arquivo de chave privada foi criado no filesystem, (b) a assinatura retornada é verificável via `gethcrypto.VerifySignature`.

### Testes para User Story 1 — escrever PRIMEIRO, garantir que FALHAM antes da implementação

- [x] T003 [US1] Criar `scenario-a/toolkit/engine/keyprovider/local_test.go` com casos de teste que referenciam a interface (compile-time) e testam `LocalKeyProvider` antes de ele existir:
  - `TestLocalKeyProvider_GenerateKey` — resultado tem 65 bytes, começa com `0x04`
  - `TestLocalKeyProvider_GenerateKey_Idempotent` — segunda chamada com mesmo ID retorna mesma pubkey
  - `TestLocalKeyProvider_Sign_Verifiable` — assinatura verificável com `gethcrypto.VerifySignature(pubkey, digest, sig[:64])`
  - `TestLocalKeyProvider_GetPublicKey_AfterGenerate` — retorna mesma pubkey que `GenerateKey`
  - `TestEVMAddress_Derivation` — endereço derivado bate com `gethcrypto.PubkeyToAddress()` formatado como hex

  **Verificar**: `go test ./engine/keyprovider/...` FALHA com erro de compilação (struct não existe). Isso confirma o ciclo Red correto.

### Implementação de User Story 1

- [x] T004 [US1] Criar `scenario-a/toolkit/engine/keyprovider/local.go` com `LocalKeyProvider`:
  - Struct com `mu sync.RWMutex` e `keys map[string]*ecdsa.PrivateKey`
  - `GenerateKey`: double-check sob write-lock (padrão de `auth/internal/kms/providers/local.go`), usa `gethcrypto.GenerateKey()`, armazena em `keys`, retorna `gethcrypto.FromECDSAPub(pubkey)` (65 bytes)
  - `Sign`: valida `len(digest) == 32` → `ErrInvalidDigest`; lookup → `ErrKeyNotFound`; chama `gethcrypto.Sign(digest, privkey)` (retorna 65 bytes `r||s||v`)
  - `GetPublicKey`: lookup → `ErrKeyNotFound`; retorna `gethcrypto.FromECDSAPub(&privkey.PublicKey)`
  - Nenhum campo ou método retorna `*ecdsa.PrivateKey` para chamadores externos

  **Verificar**: `go test ./engine/keyprovider/... -run "TestLocalKeyProvider_GenerateKey|TestLocalKeyProvider_Sign|TestEVMAddress"` PASSA (ciclo Green).

**Checkpoint**: User Story 1 totalmente funcional e testável de forma independente.

---

## Phase 4: User Story 2 — Uso local sem dependência de serviço externo (Priority: P2)

**Goal**: Emulador local cobre todos os cenários de erro e comportamento in-memory esperados pelo operador local.

**Independent Test**: Executar `go test ./engine/keyprovider/... -run "TestLocalKeyProvider_NotFound|TestLocalKeyProvider_InvalidDigest|TestLocalKeyProvider_DifferentIDs|TestLocalKeyProvider_Concurrent"` e verificar que todos os erros são descritivos e o emulador não faz chamadas de rede (verificável via mock/timeout).

### Testes para User Story 2 — adicionar ao `local_test.go`, garantir que FALHAM antes de qualquer ajuste

- [x] T005 [US2] Adicionar casos de teste ao arquivo `scenario-a/toolkit/engine/keyprovider/local_test.go`:
  - `TestLocalKeyProvider_GetPublicKey_NotFound` — retorna `ErrKeyNotFound` para ID desconhecido (sem criar chave)
  - `TestLocalKeyProvider_Sign_InvalidDigestLength` — retorna `ErrInvalidDigest` para payload com ≠ 32 bytes (testar com 31, 33, 0 bytes)
  - `TestLocalKeyProvider_Sign_UnknownID` — retorna `ErrKeyNotFound` para ID sem chave gerada
  - `TestLocalKeyProvider_GenerateKey_DifferentIDs` — dois IDs distintos produzem pubkeys distintas
  - `TestLocalKeyProvider_GenerateKey_Concurrent` — 10 goroutines chamando `GenerateKey` com o mesmo ID concorrentemente; todas recebem a mesma pubkey sem race condition (rodar com `-race`)

  **Verificar**: `go test -race ./engine/keyprovider/... -run "TestLocalKeyProvider_NotFound|TestLocalKeyProvider_InvalidDigest|TestLocalKeyProvider_DifferentIDs|TestLocalKeyProvider_Concurrent"`. Se `local.go` já cobre tudo (esperado), todos PASSAM. Se algum falha, corrigir `local.go`.

**Checkpoint**: User Stories 1 E 2 funcionam independentemente.

---

## Phase 5: User Story 3 — Extensibilidade para KMS de produção (Priority: P3)

**Goal**: Factory instancia o provedor correto a partir da URI do manifesto. Stub de produção implementa a mesma interface e retorna erro informativo em tudo.

**Independent Test**: Executar `go test ./engine/keyprovider/... -run "TestFactory|TestProdKeyProvider"` e verificar que (a) `New("kms://local-emulator")` retorna `*LocalKeyProvider`, (b) stub prod retorna `ErrNotImplemented` em todos os métodos, (c) URI inválida retorna error descritivo.

### Testes para User Story 3 — adicionar ao `local_test.go`, garantir que FALHAM antes da implementação

- [x] T006 [US3] Adicionar casos de teste ao arquivo `scenario-a/toolkit/engine/keyprovider/local_test.go`:
  - `TestFactory_LocalEmulator` — `New("kms://local-emulator")` retorna instância não-nil que satisfaz `KeyProvider` e `GenerateKey` funciona
  - `TestFactory_ProdStub_GenerateKey_NotImplemented` — `New("kms://vault://prod")` retorna stub; `GenerateKey` retorna `ErrNotImplemented`
  - `TestFactory_ProdStub_Sign_NotImplemented` — `Sign` no stub retorna `ErrNotImplemented`
  - `TestFactory_ProdStub_GetPublicKey_NotImplemented` — `GetPublicKey` no stub retorna `ErrNotImplemented`
  - `TestFactory_InvalidURI_NoScheme` — `New("vault://prod")` (sem `kms://`) retorna error descritivo
  - `TestFactory_InvalidURI_Empty` — `New("")` retorna error descritivo

  **Verificar**: `go test ./engine/keyprovider/... -run "TestFactory"` FALHA com erro de compilação (`New` não existe). Ciclo Red correto.

### Implementação de User Story 3

- [x] T007 [P] [US3] Criar `scenario-a/toolkit/engine/keyprovider/prod.go` com `prodKeyProvider` (struct vazia) implementando `KeyProvider`: todos os métodos retornam `ErrNotImplemented` com mensagem `"not implemented: prod key provider — see PR-1"`

- [x] T008 [US3] Criar `scenario-a/toolkit/engine/keyprovider/factory.go` com `New(uri string) (KeyProvider, error)`:
  - Validar que `uri` começa com `kms://`; se não, retornar error descritivo com o valor recebido
  - Extrair host: `strings.TrimPrefix(uri, "kms://")`
  - `"local-emulator"` → retornar `&LocalKeyProvider{keys: make(map[string]*ecdsa.PrivateKey)}`
  - Qualquer outro valor reconhecido como `kms://` → retornar `&prodKeyProvider{}`

  **Verificar**: `go test ./engine/keyprovider/... -run "TestFactory"` PASSA (ciclo Green).

**Checkpoint**: Todos as User Stories funcionam independentemente. Trocar `kms://local-emulator` por `kms://outro` no manifesto retorna o stub correto sem alterar nenhum outro arquivo.

---

## Phase 6: Polish & Validação Final

**Purpose**: Garantia de qualidade transversal antes de abrir o PR.

- [x] T009 [P] Executar `go vet ./...` no diretório `scenario-a/toolkit/` e corrigir qualquer warning reportado

- [x] T010 Executar `go test -race ./...` no diretório `scenario-a/toolkit/` (inclui `engine/manifest/`, `engine/genesis/`, `engine/pki/` além do novo pacote) e garantir que todos os testes existentes continuam passando (regressão zero)

- [x] T011 Verificar manualmente que nenhum método de `LocalKeyProvider` ou `prodKeyProvider` retorna `*ecdsa.PrivateKey` ou qualquer tipo que expõe material de chave privada — inspeção de código, não apenas testes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sem dependências — pode começar imediatamente
- **Foundational (Phase 2)**: Depende de Phase 1 — BLOQUEIA todas as User Stories
- **US1 (Phase 3)**: Depende de Phase 2 — pode começar após `keyprovider.go` existir
- **US2 (Phase 4)**: Depende de Phase 3 (T004 deve estar completo para os novos testes fazerem sentido)
- **US3 (Phase 5)**: Depende de Phase 2; T007 e T008 dependem de T006
- **Polish (Phase 6)**: Depende de todas as fases anteriores estarem completas

### User Story Dependencies

- **US1 (P1)**: Começa após Foundational (Phase 2) — sem dependência em outras stories
- **US2 (P2)**: Começa após US1 (Phase 3) — testa comportamentos da mesma `LocalKeyProvider`
- **US3 (P3)**: Pode começar após Foundational (Phase 2) — `prod.go` [T007] é paralelo à US1

### Within Each User Story

- **Testes PRIMEIRO** — escrever e confirmar falha antes de qualquer implementação
- `local_test.go` cresce incrementalmente por fase: T003 → T005 → T006
- `local.go` criado em T004; ajustado se T005 detectar lacunas
- `prod.go` (T007) e `factory.go` (T008) dependem de T006 (testes de factory existirem e falhando)

### Parallel Opportunities

- T007 (`prod.go`) pode rodar em paralelo com outros trabalhos após T006 (testes escritos)
- T009 (`go vet`) e T010 (`go test -race`) podem rodar em paralelo entre si

---

## Parallel Example: User Story 3

```bash
# Após T006 (testes de factory escritos e falhando):
# Estes dois podem rodar em paralelo:
Task T007: "Criar prod.go com stub ProdKeyProvider"
Task T008: "Criar factory.go com New()" # depende de T007 estar compilando
# Na prática T008 depende de T007 existir, então rodam em sequência rápida
```

---

## Implementation Strategy

### MVP First (User Story 1 Apenas)

1. Completar Phase 1: Setup (T001)
2. Completar Phase 2: Foundational (T002)
3. Completar Phase 3: User Story 1 (T003 → T004)
4. **PARAR E VALIDAR**: `go test -race ./engine/keyprovider/... -run "TestLocalKeyProvider|TestEVMAddress"` passa
5. Demo: instanciar `New("kms://local-emulator")`, `GenerateKey`, `Sign`, verificar assinatura

### Incremental Delivery

1. Setup + Foundational → interface definida
2. US1 → emulador local funcional com garantia de não-exposição de chave privada (MVP!)
3. US2 → cobertura completa de error cases e concorrência
4. US3 → factory + stub; troca de provedor sem alterar o motor
5. Cada Story adiciona valor sem quebrar as anteriores

---

## Notes

- [P] tasks = arquivos diferentes, sem dependências incompletas
- [Story] label mapeia task para User Story para rastreabilidade
- Cada User Story é completável e testável de forma independente
- **Testes DEVEM falhar antes da implementação** (Princípio V da constituição)
- Verificar `go test -race` antes de marcar qualquer task de implementação como completa
- Nenhum arquivo fora de `scenario-a/toolkit/engine/keyprovider/` e `scenario-a/toolkit/go.mod` deve ser modificado
