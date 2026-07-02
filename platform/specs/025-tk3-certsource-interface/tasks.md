# Tasks: TK-3 — Interface certSource

**Input**: Design documents from `specs/025-tk3-certsource-interface/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅

**Disciplina TDD**: Testes são escritos **antes** de cada implementação. Os stubs da Fase 2 garantem que os testes compilem; eles falham com "not implemented" até a implementação da Fase 3+.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependência de tarefa incompleta)
- **[Story]**: User story correspondente ([US1], [US2], [US3])
- Caminhos relativos à raiz do repositório

---

## Phase 1: Setup

**Objetivo**: Criar a estrutura de diretório do novo pacote `certsource`.

- [x] T001 Criar diretório `scenario-a/toolkit/engine/certsource/` (o módulo Go já existe em `scenario-a/toolkit/go.mod` — nenhuma dependência nova é adicionada)

**Checkpoint**: Diretório existe; `go.mod` inalterado.

---

## Phase 2: Foundational — Interface, Erros e Stubs

**Objetivo**: Definir o contrato público e criar stubs que permitam que os testes TDD compilem (e falhem com "not implemented") antes de qualquer implementação real.

**⚠️ CRÍTICO**: Nenhuma user story pode ser implementada antes desta fase. Os stubs são a base que torna os testes compiláveis.

- [x] T002 Criar `scenario-a/toolkit/engine/certsource/certsource.go` com a interface `CertSource` (métodos `IssueLeafCert` e `GetTrustAnchor` com assinaturas conforme data-model.md) e os cinco erros sentinela (`ErrTrustAnchorNotFound`, `ErrForbiddenRole`, `ErrUnsupportedKeyAlgorithm`, `ErrInvalidCSR`, `ErrNotImplemented`) e comentário de pacote
- [x] T003 [P] Criar `scenario-a/toolkit/engine/certsource/prod.go` com o tipo `prodCertSource` (não exportado), verificação de interface `var _ CertSource = (*prodCertSource)(nil)`, e métodos retornando `ErrNotImplemented`
- [x] T004 [P] Criar `scenario-a/toolkit/engine/certsource/factory.go` com a função `New(uri string) (CertSource, error)` — aceita `self-signed` (valor canônico, retorna `NewLocalCertSource()`), `self-signed://<x>` (forma tolerada, retorna `NewLocalCertSource()`), `ca://<x>` (retorna `&prodCertSource{}`), e qualquer outro valor retorna erro descritivo
- [x] T005 [P] Criar `scenario-a/toolkit/engine/certsource/local.go` com: tipo interno `spokeCert` (`key *ecdsa.PrivateKey`, `cert *x509.Certificate`), struct `LocalCertSource` (`mu sync.RWMutex`, `spokes map[string]*spokeCert`, `leafValidity time.Duration`), função `NewLocalCertSource()`, verificação de interface `var _ CertSource = (*LocalCertSource)(nil)`, e stubs de `IssueLeafCert` e `GetTrustAnchor` retornando `errors.New("not implemented: ...")`
- [x] T006 [P] Criar `scenario-a/toolkit/engine/certsource/helpers_test.go` com as funções de teste auxiliares: `generateTestCSR(t *testing.T, commonName, ou string) []byte` (gera par ECDSA P-256 + CSR PKCS#10 PEM com OU configurável) e `verifyLeafCert(t *testing.T, certPEM, caCertPEM []byte, cn string)` (verifica assinatura do cert folha contra o pool da CA e valida CN)

**Verificação pós-setup**: `go build ./scenario-a/toolkit/engine/certsource/...` deve compilar sem erros.

**Checkpoint**: Contrato público definido; stubs compiláveis; helpers de teste prontos.

---

## Phase 3: User Story 1 — IssueLeafCert (Priority: P1) 🎯 MVP

**Goal**: O motor de provisionamento consegue emitir um certificado TLS assinado pela CA do spoke para um banco comercial, usando apenas o CSR e o `spokeID`. A CA é gerada em memória na primeira emissão e reutilizada em chamadas subsequentes.

**Independent Test**: `go test -race ./scenario-a/toolkit/engine/certsource/... -run TestIssueLeafCert -v` — todos os testes de `IssueLeafCert` passam; nenhum arquivo `.key` ou `-ca.*` é criado.

### Testes para User Story 1 (escrever PRIMEIRO — devem FALHAR antes da implementação)

- [x] T007 [US1] Escrever em `scenario-a/toolkit/engine/certsource/local_test.go` os cinco testes de `IssueLeafCert`:
  - `TestIssueLeafCert_ValidCSR_IsVerifiableByCACert` — CSR válido com `OU=ROLE_COMMERCIAL_BANK`; cert emitido verificável pelo trust anchor de `GetTrustAnchor`
  - `TestIssueLeafCert_MalformedPEM_ReturnsError` — PEM inválido retorna erro; sem material criptográfico criado
  - `TestIssueLeafCert_ForbiddenRole_ReturnsErrForbiddenRole` — CSR com `OU=ROLE_CENTRAL_BANK` retorna `ErrForbiddenRole`
  - `TestIssueLeafCert_UnsupportedKeyAlgorithm_ReturnsError` — CSR com chave RSA retorna `ErrUnsupportedKeyAlgorithm`
  - `TestIssueLeafCert_InvalidCSRSignature_ReturnsError` — CSR com assinatura corrompida retorna `ErrInvalidCSR`
- [x] T008 [US1] Acrescentar em `scenario-a/toolkit/engine/certsource/local_test.go` o teste de ausência de arquivo: `TestLocalCertSource_NoPrivateKeyFile` — após execução completa dos testes, nenhum arquivo `*.key` ou `*-ca.*` existe no diretório temporário
- [x] T009 [US1] Verificar RED: `go test ./scenario-a/toolkit/engine/certsource/... -run "TestIssueLeafCert|TestLocalCertSource_NoPrivateKeyFile" 2>&1` — deve mostrar falhas "not implemented"

### Implementação de User Story 1

- [x] T010 [US1] Implementar `initSpoke(spokeID string) (*spokeCert, error)` (método privado) em `scenario-a/toolkit/engine/certsource/local.go`: gerar par ECDSA P-256, criar template X.509 de CA (`IsCA=true`, `KeyUsageCertSign|KeyUsageCRLSign`, `BasicConstraintsValid=true`, `SerialNumber` de 128 bits aleatórios, `CN="cbweb3-ca-{spokeID}"`, `NotAfter=now+10 anos`), self-sign com `x509.CreateCertificate`, fazer parse do DER resultante com `x509.ParseCertificate`, e armazenar em `c.spokes[spokeID]`
- [x] T011 [US1] Implementar `IssueLeafCert` em `scenario-a/toolkit/engine/certsource/local.go` seguindo o fluxo do data-model.md: decode PEM → parse CSR → `csr.CheckSignature()` → validar ECDSA P-256 → validar `OU=ROLE_COMMERCIAL_BANK` → double-check locking para obter ou criar spoke CA → criar template de cert folha com subject do CSR e `NotAfter=now+leafValidity` → assinar com CA do spoke → PEM encode e retornar
- [x] T012 [US1] Verificar GREEN: `go test -race ./scenario-a/toolkit/engine/certsource/... -run "TestIssueLeafCert|TestLocalCertSource_NoPrivateKeyFile" -v` — todos os 6 testes passam

**Checkpoint**: `IssueLeafCert` funcional e testado. MVP mínimo viável para uso pelo motor de orquestração.

---

## Phase 4: User Story 2 — GetTrustAnchor (Priority: P1)

**Goal**: O motor de provisionamento consegue recuperar o certificado CA do spoke (âncora de confiança) para incluí-lo no join bundle. A chamada é idempotente e retorna erro descritivo para spoke desconhecido.

**Independent Test**: `go test -race ./scenario-a/toolkit/engine/certsource/... -run TestGetTrustAnchor -v` — todos os testes de `GetTrustAnchor` passam.

### Testes para User Story 2 (escrever PRIMEIRO — devem FALHAR)

- [x] T013 [US2] Acrescentar em `scenario-a/toolkit/engine/certsource/local_test.go` os três testes de `GetTrustAnchor`:
  - `TestGetTrustAnchor_UnknownSpokeID_ReturnsErrNotFound` — `spokeID` nunca inicializado retorna `ErrTrustAnchorNotFound`
  - `TestGetTrustAnchor_AfterIssue_ReturnsSelfSignedCACert` — após `IssueLeafCert`, `GetTrustAnchor` retorna cert self-signed com `IsCA=true` e `Subject.CN` correto
  - `TestGetTrustAnchor_Idempotent` — duas chamadas consecutivas retornam PEM byte-a-byte idêntico
- [x] T014 [US2] Acrescentar em `scenario-a/toolkit/engine/certsource/local_test.go` os dois testes cross-story:
  - `TestIssueLeafCert_MultipleCalls_SameCA` — dois certs emitidos para o mesmo spoke são ambos verificáveis pelo mesmo trust anchor
  - `TestIssueLeafCert_DifferentSpokes_DifferentCAs` — trust anchors de dois `spokeID` distintos são certificados diferentes
- [x] T015 [US2] Verificar RED: `go test ./scenario-a/toolkit/engine/certsource/... -run "TestGetTrustAnchor|TestIssueLeafCert_MultipleCalls|TestIssueLeafCert_DifferentSpokes" 2>&1` — deve mostrar falhas "not implemented"

### Implementação de User Story 2

- [x] T016 [US2] Implementar `GetTrustAnchor` em `scenario-a/toolkit/engine/certsource/local.go`: read lock → lookup `c.spokes[spokeID]` → se não encontrado retornar `ErrTrustAnchorNotFound` → `pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: sc.cert.Raw})` → retornar
- [x] T017 [US2] Verificar GREEN: `go test -race ./scenario-a/toolkit/engine/certsource/... -run "TestGetTrustAnchor|TestIssueLeafCert_MultipleCalls|TestIssueLeafCert_DifferentSpokes" -v` — todos os 5 testes passam

**Checkpoint**: `GetTrustAnchor` funcional. US1 + US2 cobrem a interface completa da implementação local.

---

## Phase 5: User Story 3 — Factory e Stub de Produção (Priority: P3)

**Goal**: O factory `New(uri string)` instancia o tipo correto baseado na URI `certSource:` do manifesto. O stub de produção retorna `ErrNotImplemented` em todos os métodos sem pânico. URIs inválidas retornam erro descritivo antes de qualquer operação criptográfica.

**Independent Test**: `go test ./scenario-a/toolkit/engine/certsource/... -run TestFactory -v` — todos os testes de factory passam.

### Testes para User Story 3 (escrever PRIMEIRO — devem FALHAR)

- [x] T018 [US3] Acrescentar em `scenario-a/toolkit/engine/certsource/local_test.go` os três testes do factory:
  - `TestFactory_SelfSignedURI_ReturnsLocalCertSource` — `New("self-signed")` (valor canônico) e `New("self-signed://local")` (forma tolerada) retornam implementação que produz cert verificável
  - `TestFactory_ProdURI_ReturnsStubWithErrNotImplemented` — `New("ca://lnet-pki")` retorna stub; `IssueLeafCert` retorna `ErrNotImplemented`; `GetTrustAnchor` retorna `ErrNotImplemented`
  - `TestFactory_InvalidURI_ReturnsError` — valor sem forma reconhecida (`"http://..."`, `""`, `"lnet-pki"`) retorna erro descritivo; nenhuma implementação criada
- [x] T019 [US3] Verificar RED: `go test ./scenario-a/toolkit/engine/certsource/... -run TestFactory 2>&1` — deve mostrar falhas para os novos testes

### Implementação de User Story 3

- [x] T020 [US3] Verificar que `factory.go` (criado em T004) já cobre os casos de `self-signed` (canônico), `self-signed://<x>` (tolerado), `ca://`, e valor inválido conforme spec; ajustar se necessário para passar os testes T018
- [x] T021 [US3] Verificar GREEN: `go test ./scenario-a/toolkit/engine/certsource/... -run TestFactory -v` — todos os 3 testes do factory passam

**Checkpoint**: Factory funcional. Todas as user stories cobertas.

---

## Phase 6: Teste de Concorrência

**Objetivo**: Validar thread safety da `LocalCertSource` com race detector antes da fase de polish.

- [x] T022 Acrescentar em `scenario-a/toolkit/engine/certsource/local_test.go` o teste de concorrência: `TestLocalCertSource_RaceCondition` — 20 goroutines chamando `IssueLeafCert` e `GetTrustAnchor` concorrentemente para o mesmo `spokeID`
- [x] T023 Executar `go test -race ./scenario-a/toolkit/engine/certsource/... -run TestLocalCertSource_RaceCondition -v` — deve passar sem data race

---

## Phase 7: Polish & Cross-Cutting

**Objetivo**: Documentação de ponto de integração; validação final da suite completa.

- [x] T024 [P] Atualizar comentário de package em `scenario-a/toolkit/engine/certsource/certsource.go` documentando onde no fluxo de orquestração `IssueLeafCert` e `GetTrustAnchor` são chamados (ver seção "Phase 4 — Integration Point Documentation" do plan.md): motor `mode: found` chama `GetTrustAnchor` para emitir join bundle (TK-6); motor `mode: join` chama `IssueLeafCert` para onboarding do banco comercial (TK-9)
- [x] T025 Executar suite completa: `go test -race -count=1 ./scenario-a/toolkit/engine/certsource/... -v` — todos os 15 testes devem passar; sem data race; output mostra "PASS" para cada caso
- [x] T026 Verificar que `go vet ./scenario-a/toolkit/engine/certsource/...` retorna clean; e que `go build ./scenario-a/toolkit/...` compila o módulo completo sem erros

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: Sem dependências — pode iniciar imediatamente
- **Phase 2 (Foundational)**: Depende de Phase 1 — **bloqueia** todas as user stories
- **Phase 3 (US1 — P1)**: Depende de Phase 2 completa
- **Phase 4 (US2 — P1)**: Depende de Phase 3 completa (GetTrustAnchor lê o estado criado por IssueLeafCert)
- **Phase 5 (US3 — P3)**: Depende de Phase 2; pode ser executada em paralelo com Phase 3 por um segundo desenvolvedor
- **Phase 6 (Race)**: Depende de Phase 3 + Phase 4 (exercita IssueLeafCert e GetTrustAnchor juntos)
- **Phase 7 (Polish)**: Depende de todas as fases anteriores

### User Story Dependencies

- **US1 (P1) — IssueLeafCert**: Pode começar após Phase 2. Sem dependência em US2 ou US3.
- **US2 (P1) — GetTrustAnchor**: Depende de US1 (CA é criada por IssueLeafCert; GetTrustAnchor lê o mesmo estado). Implementação mais simples mas acoplada ao mesmo `LocalCertSource`.
- **US3 (P3) — Factory**: Pode começar após Phase 2 em paralelo com US1. A maior parte está nos stubs (T003, T004) — apenas T020 é pós-implementação.

### Ordem crítica dentro de cada User Story

```
Escrever testes (RED) → Verificar falha → Implementar → Verificar GREEN
```

Nunca inverter esta ordem. Testes devem falhar antes da implementação.

### Parallel Opportunities

| Fase | Tarefas paralelizáveis |
|------|----------------------|
| Phase 2 | T003, T004, T005, T006 (arquivos diferentes, sem dependências entre si) |
| Phase 5 | Pode rodar em paralelo com Phase 3 por outro desenvolvedor |
| Phase 7 | T024 é independente de T025/T026 |

---

## Parallel Example: Phase 2 (Foundational)

```bash
# Todas as tarefas podem ser executadas simultaneamente (arquivos distintos):
Task T003: "Criar prod.go com prodCertSource stub"
Task T004: "Criar factory.go com New(uri)"
Task T005: "Criar local.go com LocalCertSource stubs"
Task T006: "Criar helpers_test.go com generateTestCSR e verifyLeafCert"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 apenas)

1. Completar Phase 1: Setup
2. Completar Phase 2: Foundational (crítico — bloqueia tudo)
3. Completar Phase 3: US1 — `IssueLeafCert`
4. Completar Phase 4: US2 — `GetTrustAnchor`
5. **PARAR E VALIDAR**: `go test -race ./scenario-a/toolkit/engine/certsource/...` — todos os testes das US1+US2 passam
6. O pacote já é utilizável pelo motor de orquestração (TK-5/TK-6)

### Incremental Delivery

1. Phase 1 + 2 → contrato público e stubs compiláveis
2. Phase 3 → `IssueLeafCert` funcional → TK-5 pode começar a integrar
3. Phase 4 → `GetTrustAnchor` funcional → TK-6 (join bundle) pode buscar o trust anchor
4. Phase 5 → factory completo → TK-7 (CLI apply) pode instanciar via URI do manifesto
5. Phase 6 + 7 → produção-ready (race-free, documentado)

### Resumo de contagem

| Fase | Tarefas | Testes cobertos |
|------|---------|-----------------|
| Phase 1 (Setup) | 1 | — |
| Phase 2 (Foundational) | 5 | helpers (T006) |
| Phase 3 (US1) | 6 | 6 testes (T007-T008) |
| Phase 4 (US2) | 4 | 5 testes (T013-T014) |
| Phase 5 (US3) | 4 | 3 testes (T018) |
| Phase 6 (Race) | 2 | 1 teste de concorrência |
| Phase 7 (Polish) | 3 | suite completa |
| **Total** | **25** | **15 testes** |

---

## Notes

- `[P]` = arquivos diferentes, sem dependências entre si
- `[Story]` mapeia a tarefa à user story para rastreabilidade
- Disciplina RED-GREEN é obrigatória: testes escritos e falhando antes de implementar
- Race detector (`-race`) obrigatório em todas as execuções de verificação
- Nenhum arquivo `*.key`, `*-ca.*` deve ser criado em nenhum momento
- Nenhuma dependência externa nova em `scenario-a/toolkit/go.mod`
- Fazer commit após cada fase ou grupo lógico de tarefas
