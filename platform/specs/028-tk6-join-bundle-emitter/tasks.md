# Tasks: TK-6 — Emissor do Join Bundle

**Input**: Design documents from `/specs/028-tk6-join-bundle-emitter/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Tests**: Incluídos — SC-001 e a Constitution (Princípio V) exigem test-first explicitamente.

**Organization**: Agrupado por user story. US1 (bundle completo) é o MVP e incorpora US3 (enode correto) como componente interno — não há valor num bundle com enode errado. US2 (fail-fast) e US4 (modo exclusivo) são fases separadas que fortalecem a segurança e confiabilidade.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependência de tarefa incompleta)
- **[Story]**: User story correspondente (US1–US4)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Criar estrutura de diretórios e mover `parseDeployedAddrs` para pacote compartilhado `engine/addrs/` — pré-requisito de todas as fases.

**⚠️ CRITICAL**: Esta fase deve estar completa antes de qualquer outra fase. Garante que `go build ./scenario-a/toolkit/engine/...` e `go test ./engine/orchestrator/...` permaneçam verdes após o refactoring.

- [X] T001 Criar diretório `scenario-a/toolkit/engine/addrs/` e `scenario-a/toolkit/engine/bundle/`; mover `DeployedAddrs` e `parseDeployedAddrs` de `scenario-a/toolkit/engine/orchestrator/addrs.go` para `scenario-a/toolkit/engine/addrs/addrs.go` com o novo package header; atualizar import em `orchestrator/addrs.go`
- [X] T002 [P] Atualizar `scenario-a/toolkit/engine/orchestrator/addrs_test.go` para importar de `engine/addrs/`; executar `go test -race ./engine/orchestrator/...` e confirmar zero falhas após refactoring

**Checkpoint**: `go build ./scenario-a/toolkit/engine/...` verde. `parseDeployedAddrs` disponível em `engine/addrs/`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Definir todos os tipos Go, erros sentinela, e interface `EnodeProvider`. Estes tipos bloqueiam todas as user stories.

**⚠️ CRITICAL**: Nenhuma user story pode começar antes desta fase estar completa.

- [X] T003 Definir structs `JoinBundle`, `BundleMetadata`, `BundleSpec`, `BootnodeSpec`, `GenesisSpec`, `ContractsSpec`, `RelaySpec`, `TrustSpec` e constantes `APIVersion = "cbweb3/v1"` e `Kind = "JoinBundle"` em `scenario-a/toolkit/engine/bundle/types.go`
- [X] T004 [P] Definir struct `BundleInput` e erros sentinela `ErrInvalidMode`, `ErrInvalidInput`, `ErrGenesisNotFound`, `ErrCACertNotFound`, `ErrDeployedAddrsIncomplete`, `ErrEnodeUnavailable` em `scenario-a/toolkit/engine/bundle/errors.go`
- [X] T005 [P] Definir interface `EnodeProvider` com método `NodeInfo(ctx context.Context) (string, error)` e struct `staticEnodeProvider{enode string}` (stub para testes) em `scenario-a/toolkit/engine/bundle/enode.go`; struct `BesuEnodeProvider` como esqueleto vazio (implementada em Phase 5)
- [X] T006 Escrever esqueleto vazio de `EmitBundle(ctx context.Context, in BundleInput) (*JoinBundle, error)` em `scenario-a/toolkit/engine/bundle/bundle.go`: retorna `nil, nil` — apenas para `go build` passar antes dos testes
- [X] T007 [P] Executar `go build ./scenario-a/toolkit/engine/bundle/...` e confirmar que compila sem erros

**Checkpoint**: `go build ./scenario-a/toolkit/engine/bundle/...` verde. Tipos, erros e interface definidos.

---

## Phase 3: User Story 1 — Emissão completa do bundle (Priority: P1) 🎯 MVP

**Goal**: `EmitBundle` invocado com `BundleInput` completo produz `bundles/<spoke-id>.bundle.yaml` com todos os 6 campos obrigatórios, enode reescrito com `advertisedHost` do manifesto (US3), e zero conteúdo de chave privada.

**Independent Test**: Montar `SPOKE_DATA_DIR` em `t.TempDir()` com genesis.json, .deployed-addrs.env (7 chaves), tls/central-bank.crt PEM válido; injetar `staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}`; manifesto com `advertisedHost: bank.local` e `p2p.port: 31303`; verificar que `bundles/spoke-brl.bundle.yaml` existe, é YAML válido, `spec.bootnode.enode == "enode://abc@bank.local:31303"`, `spec.genesis.hash` = SHA-256 do genesis.json.

### Enode parsing e reescrita (US3 — componente interno de US1)

- [X] T008 Escrever testes failing para `parseAndRewriteEnode` em `scenario-a/toolkit/engine/bundle/enode_test.go`: (a) `("enode://abc@0.0.0.0:30303", "bank.local", 31303)` → `"enode://abc@bank.local:31303"`; (b) host `[::]` → substituído; (c) enode sem `@` → erro; (d) prefixo ausente → erro; (e) `p2pPort == 0` → erro
- [X] T009 [P] [US1] Implementar `parseAndRewriteEnode(raw, advertisedHost string, p2pPort int) (string, error)` em `scenario-a/toolkit/engine/bundle/enode.go`: localizar `@` com `strings.LastIndex`, extrair prefixo `enode://<id>`, compor `enode://<id>@<advertisedHost>:<p2pPort>`; retornar `ErrEnodeUnavailable` se mal formado

### Coleta de artefatos

- [X] T010 [P] [US1] Escrever testes failing para `readGenesis` em `scenario-a/toolkit/engine/bundle/bundle_test.go`: (a) genesis.json 3 KB → `GenesisSpec.Hash` = `"sha256:" + SHA-256 hex`, `GenesisSpec.Content` = base64 StdEncoding sem newlines, decodificável para o JSON original; (b) arquivo ausente → `ErrGenesisNotFound`
- [X] T011 [P] [US1] Implementar `readGenesis(dataDir string) (GenesisSpec, error)` em `scenario-a/toolkit/engine/bundle/bundle.go`: ler `<dataDir>/genesis/genesis.json`, computar `crypto/sha256`, prefixar `"sha256:"`, `base64.StdEncoding.EncodeToString`; retornar `ErrGenesisNotFound` em `os.IsNotExist`
- [X] T012 [P] [US1] Escrever testes failing para `readCACert` em `scenario-a/toolkit/engine/bundle/bundle_test.go`: (a) PEM `CERTIFICATE` válido → `TrustSpec.CACertPEM` = conteúdo PEM original; (b) arquivo com `PRIVATE KEY` → `ErrCACertNotFound` com detalhe; (c) arquivo sem bloco PEM → `ErrCACertNotFound`; (d) ausente → `ErrCACertNotFound`
- [X] T013 [P] [US1] Implementar `readCACert(dataDir string) (TrustSpec, error)` em `scenario-a/toolkit/engine/bundle/bundle.go`: ler `<dataDir>/tls/central-bank.crt`, chamar `pem.Decode`, verificar `block.Type == "CERTIFICATE"`, buscar `"PRIVATE KEY"` no conteúdo total com `strings.Contains`; retornar `ErrCACertNotFound` nos casos de falha
- [X] T014 [P] [US1] Escrever testes failing para leitura de `.deployed-addrs.env` em `scenario-a/toolkit/engine/bundle/bundle_test.go`: (a) arquivo com 7 chaves preenchidas → `ContractsSpec` com todos os campos; (b) `REGISTRY_CONTRACT_ADDRESS` vazio → `ErrDeployedAddrsIncomplete` com nome da chave na mensagem; (c) arquivo ausente → `ErrDeployedAddrsIncomplete`
- [X] T015 [P] [US1] Implementar `readContracts(dataDir string) (ContractsSpec, error)` em `scenario-a/toolkit/engine/bundle/bundle.go`: chamar `addrs.ParseDeployedAddrs(<dataDir>+"/.deployed-addrs.env")`; verificar cada um dos 7 campos obrigatórios; retornar `fmt.Errorf("%w: %s", ErrDeployedAddrsIncomplete, keyName)` na primeira chave vazia

### Composição e escrita atômica

- [X] T016 [US1] Escrever testes failing end-to-end para `EmitBundle` em `scenario-a/toolkit/engine/bundle/bundle_test.go`: (a) `SPOKE_DATA_DIR` totalmente populado + `staticEnodeProvider` → `bundles/spoke-brl.bundle.yaml` existe, é YAML parseable para `JoinBundle`, `apiVersion == "cbweb3/v1"`, `kind == "JoinBundle"`; (b) `spec.bootnode.enode` usa `advertisedHost` e `p2pPort` do manifesto; (c) `spec.genesis.hash` = SHA-256 do genesis de teste; (d) re-emissão com mesmos artefatos sobrescreve sem erro; (e) `outputDir` inexistente → criado automaticamente
- [X] T017 [US1] Implementar composição de `JoinBundle` em `EmitBundle` em `scenario-a/toolkit/engine/bundle/bundle.go`: chamar `readGenesis`, `readCACert`, `readContracts`, `in.EnodeProvider.NodeInfo` + `parseAndRewriteEnode`; preencher `BundleSpec` com `spec.relay` omitido se `manifest.Spec.Relay == nil`; `metadata.generatedAt = time.Now().UTC().Format(time.RFC3339)`
- [X] T018 [US1] Implementar escrita atômica em `EmitBundle` em `scenario-a/toolkit/engine/bundle/bundle.go`: `os.MkdirAll(<outputDir>/bundles, 0o755)` → `yaml.Marshal(bundle)` → `os.CreateTemp(bundleDir, ".bundle-*.yaml.tmp")` → escrever → `tmp.Close()` → `os.Rename(tmp, bundlePath)`; testes de T016 devem passar com arquivo real no disco

**Checkpoint**: `go test -race ./engine/bundle/... -run TestEmit` verde. Bundle no disco com todos os campos corretos. US1 + US3 completos.

---

## Phase 4: User Story 2 — Fail-fast em artefatos ausentes (Priority: P1)

**Goal**: `EmitBundle` retorna o erro tipado correto para cada artefato ausente antes de qualquer I/O; nenhum bundle parcial é criado em falha.

**Independent Test**: Invocar `EmitBundle` com cada artefato ausente individualmente; verificar tipo de erro com `errors.Is` e ausência do arquivo bundle no `outputDir`.

- [X] T019 [US2] Escrever testes failing para todos os erros de artefatos em `scenario-a/toolkit/engine/bundle/bundle_test.go`: (a) genesis ausente → `ErrGenesisNotFound`; (b) central-bank.crt ausente → `ErrCACertNotFound`; (c) central-bank.crt com `PRIVATE KEY` → `ErrCACertNotFound` + detalhe "private key material"; (d) `.deployed-addrs.env` com chave vazia → `ErrDeployedAddrsIncomplete` + nome da chave; (e) `staticEnodeProvider` retornando erro → `ErrEnodeUnavailable`; (f) enode sem `@` → `ErrEnodeUnavailable` com detalhe; verificar em todos os casos que `bundles/*.yaml` não existe no `outputDir`
- [X] T020 [P] [US2] Escrever testes failing para erros de BundleInput inválido em `scenario-a/toolkit/engine/bundle/bundle_test.go`: (a) `in.Manifest == nil` → `ErrInvalidInput`; (b) `in.DataDir == ""` → `ErrInvalidInput`; (c) `in.EnodeProvider == nil` → `ErrInvalidInput`; (d) `spec.node.p2p == nil` → `ErrInvalidInput`; (e) `spec.node.p2p.port == 0` → `ErrInvalidInput`
- [X] T021 [US2] Implementar todas as validações de precondição em `EmitBundle` (`scenario-a/toolkit/engine/bundle/bundle.go`): validar `in.Manifest`, `in.DataDir`, `in.EnodeProvider`, `spec.node.p2p.port > 0`; as funções `readGenesis`, `readCACert`, `readContracts` e `parseAndRewriteEnode` já retornam os erros corretos — `EmitBundle` propaga sem bundle parcial; testes de T019 e T020 devem passar
- [X] T022 [P] [US2] Escrever teste de context cancellation em `scenario-a/toolkit/engine/bundle/bundle_test.go`: criar contexto cancelado antes de chamar `EmitBundle`; `staticEnodeProvider` que verifica `ctx.Done()` antes de retornar; verificar que `EmitBundle` retorna `context.Canceled` e não cria arquivo de bundle
- [X] T023 [P] [US2] Implementar propagação de `ctx` em `EmitBundle`: passar ctx para `in.EnodeProvider.NodeInfo(ctx)` e verificar `ctx.Err()` logo após a chamada RPC; retornar `ctx.Err()` se não-nil

**Checkpoint**: `go test -race ./engine/bundle/... -run TestFailFast` verde. US2 completo — todos os erros tipados retornam corretamente.

---

## Phase 5: User Story 4 — Modo exclusivo e `BesuEnodeProvider` HTTP (Priority: P2)

**Goal**: `EmitBundle` rejeita `mode != "found"` antes de qualquer I/O; `BesuEnodeProvider` obtém o enode-id via JSON-RPC `admin_nodeInfo` com propagação de ctx e timeout de 10 s.

**Independent Test**: (a) Invocar `EmitBundle` com `mode: join` → `ErrInvalidMode` sem ler nenhum arquivo. (b) Usar `httptest.NewServer` retornando `admin_nodeInfo` válido → `BesuEnodeProvider.NodeInfo` retorna enode string; servidor fechado → `ErrEnodeUnavailable`.

- [X] T024 [US4] Escrever testes failing para guard de modo em `scenario-a/toolkit/engine/bundle/bundle_test.go`: (a) `mode: join` → `ErrInvalidMode` com mensagem contendo "got: join"; (b) `mode: ""` → `ErrInvalidMode`; (c) `mode: found` passa o guard e prossegue normalmente; verificar que (a) e (b) não criam arquivo bundle
- [X] T025 [US4] Implementar guard de modo no início de `EmitBundle` em `scenario-a/toolkit/engine/bundle/bundle.go`: `if manifest.Spec.Mode != "found" { return nil, fmt.Errorf("%w, got: %s", ErrInvalidMode, manifest.Spec.Mode) }` antes de qualquer outra operação; testes de T024 devem passar
- [X] T026 [P] [US4] Escrever testes failing para `BesuEnodeProvider` em `scenario-a/toolkit/engine/bundle/enode_test.go` com `httptest.NewServer`: (a) servidor retorna `{"result":{"enode":"enode://abc@0.0.0.0:30303"}}` → `NodeInfo` retorna `"enode://abc@0.0.0.0:30303"`; (b) servidor retorna HTTP 500 → `ErrEnodeUnavailable`; (c) servidor fechado (connection refused) → `ErrEnodeUnavailable`; (d) resposta sem campo `result.enode` → `ErrEnodeUnavailable`; (e) contexto cancelado antes da chamada → retorna `ctx.Err()`
- [X] T027 [P] [US4] Implementar `BesuEnodeProvider.NodeInfo` em `scenario-a/toolkit/engine/bundle/enode.go`: se `HTTPClient == nil`, usar `&http.Client{Timeout: 10*time.Second}`; fazer `POST` JSON-RPC `{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}` com `http.NewRequestWithContext(ctx, ...)`; decodificar resposta, extrair `result.enode`; retornar `fmt.Errorf("%w: %v", ErrEnodeUnavailable, err)` em qualquer falha; testes de T026 devem passar
- [X] T028 [P] [US4] Implementar `NewBesuEnodeProvider(rpcURL string, httpClient *http.Client) EnodeProvider` em `scenario-a/toolkit/engine/bundle/enode.go` como constructor público; documentar contrato de nil check no httpClient

**Checkpoint**: `go test -race ./engine/bundle/... -run TestMode` e `-run TestBesuProvider` verdes. US4 completo.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Observabilidade (`log/slog`), segurança (verificação de chave no bundle), teste E2E de integração, e validação final.

- [X] T029 [P] Adicionar logs `log/slog` em `EmitBundle` em `scenario-a/toolkit/engine/bundle/bundle.go`: `slog.InfoContext(ctx, "bundle: emitting", "spoke_id", spokeID)` no início; `slog.InfoContext(ctx, "bundle: written", "spoke_id", spokeID, "path", bundlePath)` na conclusão; `slog.ErrorContext(ctx, "bundle: failed", "spoke_id", spokeID, "error", err)` antes de qualquer `return ..., err` — sem silent failures (Constitution VI)
- [X] T030 [P] Escrever testes de invariante de segurança em `scenario-a/toolkit/engine/bundle/bundle_test.go`: (a) chamar `EmitBundle` com artefatos válidos; ler o YAML serializado do bundle; verificar com `strings.Contains(yaml, "PRIVATE KEY") == false` (SC-006); (b) verificar `spec.trust.caCertPEM` começa com `"-----BEGIN CERTIFICATE-----"` (SC-002); (c) verificar `spec.bootnode.enode` não contém o host `"0.0.0.0"` nem `"172."` (SC-004)
- [X] T031 Escrever teste de integração E2E em `scenario-a/toolkit/engine/bundle/bundle_integration_test.go` (build tag `//go:build integration`): montar `SPOKE_DATA_DIR` real com genesis de rede local, cert TLS gerado via `crypto/x509`, deployed-addrs preenchidos; usar `httptest.NewServer` como mock do Besu; invocar `EmitBundle`; verificar SC-002 a SC-009 da spec; verificar bundle parseable e campo a campo; executável com `go test -race -tags integration -timeout 5m ./engine/bundle/...`
- [X] T032 [P] Executar `go vet ./scenario-a/toolkit/engine/bundle/...` e corrigir qualquer warning; executar `go test -race ./scenario-a/toolkit/engine/bundle/...` sem flags de integration e verificar 100% de passagem; confirmar SC-009

**Checkpoint**: `go test -race ./engine/bundle/...` 100% verde. `go vet` zero warnings. US1–US4 verificados. Integration test executável.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sem dependências — pode começar imediatamente; **bloqueia todas as fases**
- **Foundational (Phase 2)**: Depende de Phase 1 — **bloqueia todas as user stories**
- **US1 + US3 (Phase 3)**: Depende de Phase 2 — MVP; passos T008–T015 têm amplo paralelismo
- **US2 (Phase 4)**: Depende de Phase 3 — as funções de coleta já existem; adiciona paths de erro
- **US4 (Phase 5)**: Depende de Phase 2 — pode começar após Phase 2 em paralelo com Phase 3; usa `EnodeProvider` da Phase 2
- **Polish (Phase 6)**: Depende de todas as fases anteriores

### User Story Dependencies

- **US1 (P1 🎯 MVP)**: Pode começar após Phase 2; inclui US3 (enode) como componente interno
- **US2 (P1)**: Depende de US1 — valida os error paths das funções de coleta já implementadas
- **US3 (P1)**: Componente de US1 — `parseAndRewriteEnode` (T008–T009) pode ser implementado antes de US1 como building block
- **US4 (P2)**: Depende de Phase 2 apenas; BesuEnodeProvider pode ser implementado em paralelo com US1/US2

### Parallel Opportunities

- T003, T004, T005 (Phase 2): paralelos entre si
- T009, T010+T011, T012+T013, T014+T015 (Phase 3): totalmente paralelos — arquivos de test e impl distintos
- T019, T020 (Phase 4): paralelos entre si
- T022, T023 (Phase 4): paralelos entre si
- T026, T027, T028 (Phase 5): T026 e T027 em sequência; T028 paralelo com T026
- T029, T030, T032 (Phase 6): paralelos entre si

---

## Parallel Example: Phase 3 — Coleta de artefatos

```bash
# Escrever e implementar os 3 coletores em paralelo após T008 concluído:
Task T010: "Testes + impl readGenesis em bundle_test.go + bundle.go"
Task T012: "Testes + impl readCACert em bundle_test.go + bundle.go"
Task T014: "Testes + impl readContracts em bundle_test.go + bundle.go"
# Todos em arquivos com funções independentes — zero conflito.
```

## Parallel Example: Phase 5 — BesuEnodeProvider

```bash
# BesuEnodeProvider pode avançar após Phase 2, em paralelo com US1:
Task T026: "Escrever testes failing para BesuEnodeProvider com httptest"
# Após T026 failing:
Task T027: "Implementar BesuEnodeProvider.NodeInfo"
Task T028: "Implementar NewBesuEnodeProvider constructor"
# T028 é independente de T026/T027 — pode rodar em paralelo
```

---

## Implementation Strategy

### MVP First (User Story 1 — Phase 3)

1. Phase 1: Mover addrs → `engine/addrs/`
2. Phase 2: Tipos, erros, interface, skeleton
3. Phase 3: US1 + US3 — enode parsing, coleta, composição, escrita no disco
4. **PARAR e VALIDAR**: `go test -race ./engine/bundle/...` verde; inspecionar `bundles/spoke-brl.bundle.yaml` manualmente; confirmar SC-002–SC-006

### Incremental Delivery

1. Phase 1+2 → Fundação
2. Phase 3 → Bundle no disco → **MVP!** Bundle distribuível
3. Phase 4 → Fail-fast → Operação segura
4. Phase 5 → BesuEnodeProvider HTTP → Produção-ready
5. Phase 6 → Logs, segurança, E2E → Production quality

### Parallel Team Strategy

Com dois desenvolvedores após Phase 2:
- Dev A: Phase 3 (US1+US3) — enode parsing + coleta + composição
- Dev B: Phase 5 (US4) — BesuEnodeProvider HTTP com httptest

Dev A e Dev B integram na Phase 4 (US2) e Phase 6 (Polish).

---

## Notes

- [P] = arquivos distintos ou funções independentes, paralelo seguro
- Testes **devem** falhar antes da implementação (Constitution V) — Red-Green-Refactor
- `parseDeployedAddrs` deve ser importada de `engine/addrs/` em T001; não duplicar
- Escrita atômica via temp+rename é obrigatória (FR-010)
- SC-006: `grep "PRIVATE KEY" bundles/*.bundle.yaml` deve retornar vazio — testar em T030
- Nenhum arquivo em `deploy/local/` ou `make/` é modificado neste PR
- `mode: join` guard (US4, T024–T025) é a primeira verificação em `EmitBundle` — antes de qualquer I/O

---

## Phase 7: Convergence

- [X] T033 Em `scenario-a/toolkit/engine/bundle/bundle_test.go`, adicionar caso de teste para `manifest.Spec.Relay == nil`: invocar `EmitBundle` com manifesto sem campo `relay`; ler e desserializar o YAML produzido; verificar que o campo `spec.relay` está ausente do documento (i.e., `jb.Spec.Relay == nil` após parse) — confirma o comportamento `omitempty` de FR-009 per FR-009, spec edge case "relay nil" (partial)
- [X] T034 Em `scenario-a/toolkit/engine/bundle/bundle.go`, na função `readGenesis`, após ler o arquivo com `os.ReadFile`, adicionar: `if len(genesisBytes) > 1<<20 { slog.WarnContext(ctx, "bundle: genesis exceeds 1 MB", "bytes", len(genesisBytes)) }`; propagar `ctx` para `readGenesis(ctx context.Context, dataDir string)`; atualizar a assinatura em T011 e a chamada em `EmitBundle` per research.md R-02, plan.md Phase 0 decisions (missing)
- [X] T035 [P] Em `scenario-a/toolkit/engine/bundle/bundle_test.go`, tornar o teste de re-emissão (SC-008) explicitamente field-by-field: invocar `EmitBundle` duas vezes com os mesmos artefatos; desserializar ambos os YAMLs para `JoinBundle`; zerar `metadata.generatedAt` em ambos; usar `assert.Equal` (ou comparação de struct) para verificar que todos os campos estruturais são idênticos — detecta serialização não-determinística ou campos descartados per SC-008, FR-012 (partial)

## Phase 8: Convergence

- [X] T036 Em `scenario-a/toolkit/engine/bundle/bundle_test.go`, adicionar verificação do valor exato do hash em `TestReadGenesis_Success`: dado `genesisContent` com valor fixo e conhecido, computar `expectedHash := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(genesisContent)))` e verificar `spec.Hash == expectedHash`; adicionalmente verificar que `base64.StdEncoding.DecodeString(spec.Content)` retorna os bytes originais sem erro — garante corretude numérica do hash e decodificabilidade do content per SC-005, FR-005 (partial)
- [X] T037 [P] Em `scenario-a/toolkit/engine/bundle/bundle_test.go`, remover os dois closures mortos de `TestEmitBundle_GenesisHashMatchesSHA256` (`import_sha256` e `import_crypto_sha256`, ambos atribuídos a `_` e sem efeito); simplificar o corpo do teste mantendo apenas as verificações de formato já existentes ou unificá-lo com o teste de hash exato de T036 se as coberturas se sobrepuserem — remove código morto que induz leitura errada (unrequested)
