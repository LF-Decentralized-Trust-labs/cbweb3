---
description: "Task list — TK-B2/B3 (KeyProvider + CertSource)"
---

# Tasks: Toolkit do Cenário B — KeyProvider + CertSource (TK-B2/B3)

**Input**: Design documents from `/specs/034-tk-b2-keyprovider-certsource/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/interfaces.md, quickstart.md

**Tests**: INCLUÍDOS (test-first). A Constituição (Princípio V) e a spec exigem teste que falha
antes da implementação. Escreva cada teste, veja-o **falhar**, depois implemente.

**Organization**: Tarefas agrupadas por user story (US1, US2, US3), independentemente testáveis.

**Module root**: `scenario-b/toolkit/` (módulo `github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit`).
Todos os caminhos abaixo são relativos à raiz do repositório.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizável (arquivo distinto, sem dependência pendente)
- **[Story]**: US1 / US2 / US3

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: estrutura dos pacotes do toolkit.

- [X] T001 Criar os diretórios de pacote `scenario-b/toolkit/engine/keyprovider/`, `scenario-b/toolkit/engine/certsource/` e `scenario-b/toolkit/engine/pki/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: dependência de cripto que bloqueia o KeyProvider (secp256k1/EVM).

**⚠️ CRITICAL**: nenhuma user story que use secp256k1 pode começar antes disto.

- [X] T002 Adicionar `github.com/ethereum/go-ethereum` a `scenario-b/toolkit/go.mod` e rodar `go mod tidy` (justificado no plan.md — secp256k1 + endereço EVM)

**Checkpoint**: dependências prontas — as user stories podem começar.

---

## Phase 3: User Story 1 — KeyProvider: custódia de chaves por entidade (Priority: P1) 🎯 MVP

**Goal**: fronteira de custódia que gera/guarda uma chave secp256k1 por entidade, assina digests
e expõe apenas pubkey/endereço; chave privada nunca em saída serializável.

**Independent Test**: gerar chave por id → obter pubkey/endereço → assinar digest → verificar
assinatura; ids distintos ⇒ endereços distintos; `GenerateKey` idempotente.

### Tests for User Story 1 ⚠️ (escrever primeiro, ver falhar)

- [X] T003 [P] [US1] Teste de unidade em `scenario-b/toolkit/engine/keyprovider/keyprovider_test.go`: `GenerateKey` idempotente; `Sign`→assinatura verifica contra `GetPublicKey`; `EVMAddress` estável; dois ids ⇒ endereços distintos; `GetPublicKey` de id inexistente → `ErrKeyNotFound`

### Implementation for User Story 1

- [X] T004 [US1] `scenario-b/toolkit/engine/keyprovider/keyprovider.go`: interface `KeyProvider` (`GenerateKey/Sign/GetPublicKey`), interface `LocalKeyExporter`, erros tipados (`ErrNotImplemented`, `ErrKeyNotFound`), função `EVMAddress(pubkey)` (keccak256 + últimos 20 bytes) — conforme contracts/interfaces.md
- [X] T005 [US1] `scenario-b/toolkit/engine/keyprovider/local.go`: impl in-memory secp256k1, derivação determinística por id + seed base, chave dev semeada sob id conhecido, lock; implementa `LocalKeyExporter.ExportPrivateKeyHex` (apenas-local)
- [X] T006 [US1] `scenario-b/toolkit/engine/keyprovider/prod.go`: stub de produção — todo método retorna `ErrNotImplemented`; **não** implementa `LocalKeyExporter`

**Checkpoint**: US1 funcional e testável isolada (`go test ./engine/keyprovider/...`).

---

## Phase 4: User Story 2 — CertSource: CB como CA do spoke, emissão de leaf (Priority: P1)

**Goal**: CB é a CA do spoke; emite leaf a partir de CSR (banco detém a chave); expõe a âncora;
chave da CA nunca sai do processo.

**Independent Test**: gerar CSR P-256 `OU=ROLE_COMMERCIAL_BANK`; `IssueLeafCert` retorna leaf que
verifica contra `GetTrustAnchor`; CSRs inválidos rejeitados com erro específico.

### Tests for User Story 2 ⚠️ (escrever primeiro, ver falhar)

- [X] T007 [P] [US2] Teste em `scenario-b/toolkit/engine/pki/csr_test.go`: `GenerateBankCSR` produz par P-256 + CSR com subject `CN/O/OU=ROLE_COMMERCIAL_BANK/C=BR`, escreve `<bankCode>.key` (0600) e `.csr`, e **não** cria material de CA
- [X] T008 [P] [US2] Teste em `scenario-b/toolkit/engine/certsource/certsource_test.go`: leaf emitido verifica contra a âncora do mesmo spoke; rejeições tipadas para CSR não-P-256 (`ErrUnsupportedKeyAlgorithm`), sem OU (`ErrForbiddenRole`), malformado/auto-assinatura inválida (`ErrInvalidCSR`); `GetTrustAnchor` de spoke sem CA → `ErrTrustAnchorNotFound` (não cria CA)

### Implementation for User Story 2

- [X] T009 [US2] `scenario-b/toolkit/engine/pki/csr.go`: `GenerateBankCSR(bankCode, institution, outDir)` — par ECDSA P-256 + CSR PKCS#10, escreve key (0600)/csr; nunca gera CA (habilita os testes de US2 e fases futuras)
- [X] T010 [US2] `scenario-b/toolkit/engine/certsource/certsource.go`: interface `CertSource` (`IssueLeafCert/GetTrustAnchor`) + erros tipados (`ErrNotImplemented`, `ErrInvalidCSR`, `ErrUnsupportedKeyAlgorithm`, `ErrForbiddenRole`, `ErrTrustAnchorNotFound`)
- [X] T011 [US2] `scenario-b/toolkit/engine/certsource/local.go`: CA self-signed P-256 por spoke **só em memória** (criação preguiçosa no 1º `IssueLeafCert`, double-check lock, chave nunca serializada); cadeia de validação do CSR na ordem do FR-009; `GetTrustAnchor` não inicializa CA
- [X] T012 [US2] `scenario-b/toolkit/engine/certsource/prod.go`: stub de produção — todo método retorna `ErrNotImplemented`

**Checkpoint**: US2 funcional e testável isolada (`go test ./engine/certsource/... ./engine/pki/...`).

---

## Phase 5: User Story 3 — Factories por URI e garantia de "sem segredos" (Priority: P2)

**Goal**: seleção da implementação por URI (`kms://`, `self-signed`, `ca://`) e invariante de
que nenhuma chave privada é serializada.

**Independent Test**: URIs resolvem local vs. stub conforme a tabela; URI não suportada → erro;
varredura de saídas serializáveis não contém material de chave privada.

### Tests for User Story 3 ⚠️ (escrever primeiro, ver falhar)

- [X] T013 [P] [US3] Teste em `scenario-b/toolkit/engine/keyprovider/factory_test.go`: `New("kms://local-emulator")`→local; outro `kms://…`→stub (`ErrNotImplemented` nas operações); URI não suportada→erro de config
- [X] T014 [P] [US3] Teste em `scenario-b/toolkit/engine/certsource/factory_test.go`: `New("self-signed")` e `New("self-signed://…")`→local; `New("ca://…")`→stub; URI não suportada→erro de config
- [X] T015 [P] [US3] Teste "sem segredos" em `scenario-b/toolkit/engine/keyprovider/nosecrets_test.go` e `scenario-b/toolkit/engine/certsource/nosecrets_test.go`: nenhuma saída pública (`GetPublicKey`, `IssueLeafCert`, `GetTrustAnchor`) contém bloco `PRIVATE KEY`; o único caminho a expor privada é `LocalKeyExporter` (local) — o stub de prod não o implementa

### Implementation for User Story 3

- [X] T016 [US3] `scenario-b/toolkit/engine/keyprovider/factory.go`: `New(uri)` — `kms://local-emulator[?seed=…]`→local semeado; qualquer outro `kms://…`→prod stub; URI não suportada→erro
- [X] T017 [US3] `scenario-b/toolkit/engine/certsource/factory.go`: `New(uri)` — `self-signed`|`self-signed://…`→local; `ca://…`→prod stub; URI não suportada→erro

**Checkpoint**: as três user stories independentemente funcionais.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T018 [P] `gofmt`/`go vet ./engine/...` no módulo `scenario-b/toolkit`
- [X] T019 Rodar a suíte completa `cd scenario-b/toolkit && go test ./engine/...` e validar os passos do `quickstart.md`
- [X] T020 [P] Atualizar nota de status TK-B2/B3 em `scenario-b/docs/design/scenario-b-toolkit-roadmap.md` (§15) para refletir a implementação
- [X] T021 [P] Referenciar a justificativa da nova dependência `github.com/ethereum/go-ethereum` (secp256k1/EVM no `toolkit`) em `scenario-b/README.md` — exigência da Constituição (Technology Stack Constraints: nova dependência de runtime deve ser justificada no PR e referenciada no README do cenário)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sem dependências.
- **Foundational (Phase 2)**: depende do Setup; **bloqueia** US1 e US3 (secp256k1).
- **US1 (Phase 3)**: depois da Foundational. Independente das demais.
- **US2 (Phase 4)**: depois do Setup (só stdlib crypto — não depende de go-ethereum nem de US1). Pode correr em paralelo à US1.
- **US3 (Phase 5)**: depois de US1 **e** US2 (as factories constroem as impls locais/stub de ambos).
- **Polish (Phase 6)**: depois das user stories desejadas.

### Within Each User Story

- Teste escrito e **falhando** antes da implementação.
- Interface + erros antes das impls; local e prod são arquivos distintos.

### Parallel Opportunities

- US1 e US2 podem ser desenvolvidas em paralelo após a Foundational (arquivos/pacotes distintos).
- Testes marcados [P] em arquivos distintos correm juntos.
- Em US3, T013/T014/T015 são [P] (arquivos distintos).

---

## Parallel Example: após a Foundational

```bash
# US1 e US2 em paralelo (pacotes distintos):
Task: "T003 keyprovider_test.go — gerar/assinar/verificar"        # US1
Task: "T007 pki/csr_test.go — GenerateBankCSR"                    # US2
Task: "T008 certsource_test.go — emitir/validar leaf"            # US2
```

---

## Implementation Strategy

### MVP First (US1)

1. Phase 1 Setup → Phase 2 Foundational.
2. Phase 3 US1 (KeyProvider) → `go test ./engine/keyprovider/...` verde.
3. **STOP & VALIDATE**: fronteira de custódia de chaves pronta e isolada.

### Incremental Delivery

1. Setup + Foundational.
2. US1 (KeyProvider) → testar → MVP.
3. US2 (CertSource + PKI) → testar.
4. US3 (factories + sem-segredos) → testar.
5. Polish.

---

## Notes

- [P] = arquivos distintos, sem dependência pendente.
- Invariante transversal: **nenhum segredo** em saída serializável (FR-012); a chave da CA nunca a disco/log (FR-008); só `LocalKeyExporter` (local) expõe privada.
- Prod = stub (`ErrNotImplemented`); integração real KMS/PKI é fase posterior.
- Não importar `scenario-a/toolkit` (Princípio I) — padrão reimplementado.
- `go 1.26` no `go.mod` (alinhado ao stack do repositório — "Go 1.26+"), conforme plan.md.
