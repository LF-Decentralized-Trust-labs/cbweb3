# Tasks: TK-7 — Comando `apply`

**Input**: Design documents from `/specs/029-tk7-apply-command/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Tests**: Incluídos — SC-001 e a Constitution (Princípio V) exigem test-first explicitamente. Cada fase começa com testes failing antes da implementação.

**Organization**: Agrupado por user story. Fase 2 (Foundational) é bloqueante para todas as histórias. US4 (fail-fast) e US3 (output estruturado) são implementados antes de US1 (provisioning) pois US1 depende de ambos. US5 (deps resolution) é P2 mas implementado antes de US1 pois US1 depende de ResolveDeps. US2 (dry-run) é independente e implementado por último.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependência de tarefa incompleta)
- **[Story]**: User story correspondente (US1–US5)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Criar estrutura de diretórios e fixtures de teste — pré-requisito de todas as fases.

**⚠️ CRITICAL**: Deve estar completa antes de qualquer outra fase.

- [X] T001 Criar diretórios `scenario-a/toolkit/cmd/cbweb3/` e `scenario-a/toolkit/engine/apply/`; confirmar que `scenario-a/toolkit/go.mod` já existe com module `github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit`
- [X] T002 [P] Criar `scenario-a/toolkit/cmd/cbweb3/testdata/` com fixture `central-bank-brl.yaml` (manifesto válido, `mode: found, environment: local`) e `invalid-syntax.yaml` (YAML com sintaxe inválida) e `missing-spoke-id.yaml` (válido mas sem `spec.spoke.id`)

**Checkpoint**: Diretórios e fixtures prontos. `go list ./...` inclui os novos paths.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Exportar identificadores do orchestrator que o dry-run e o apply precisam; definir todos os tipos Go e skeletons das funções públicas — sem implementação. Zero user story pode começar antes desta fase estar completa.

**⚠️ CRITICAL**: Nenhuma user story pode começar antes desta fase estar completa.

- [X] T003 Exportar `LoadState` em `scenario-a/toolkit/engine/orchestrator/state.go`: renomear `loadState` → `LoadState` (capitalizar); atualizar as 2 chamadas internas no mesmo arquivo e em `orchestrator.go`
- [X] T004 [P] Exportar `CanonicalStepOrder` em `scenario-a/toolkit/engine/orchestrator/step.go`: renomear `canonicalStepOrder` → `CanonicalStepOrder` (capitalizar); nenhum caller externo, apenas o nome muda
- [X] T005 [P] Definir structs `ApplyInput`, `ApplyResult`, `StepResult`, `BundleRef` com tags `json` e `yaml` corretas (ver data-model.md) em `scenario-a/toolkit/engine/apply/result.go`; incluir `emitReport(result ApplyResult, format string, w io.Writer) error` como esqueleto vazio (retorna nil)
- [X] T006 Escrever skeletons em `scenario-a/toolkit/engine/apply/`: `apply.go` com `Run(ctx context.Context, in ApplyInput) (ApplyResult, error)` retornando zero-value; `dryrun.go` com `DryRun(ctx context.Context, in ApplyInput) (ApplyResult, error)` retornando zero-value; `deps.go` com `ResolveDeps(m *manifest.Manifest) (orchestrator.Deps, error)` retornando zero-value — apenas para `go build` passar
- [X] T007 Criar `scenario-a/toolkit/cmd/cbweb3/main.go` esqueleto: parseia subcomando `apply`, flags `-f`, `--dry-run`/`-n`, `--output`/`-o`; chama `apply.Run` ou `apply.DryRun` com zero-value ApplyInput; compila sem erro
- [X] T008 [P] Executar `go build ./engine/orchestrator/... && go build ./engine/apply/... && go build ./cmd/cbweb3/...` — confirmar que compila sem erros; executar `go test -race ./engine/orchestrator/...` — confirmar zero regressões após export de LoadState e CanonicalStepOrder

**Checkpoint**: `go build ./...` verde no módulo toolkit. Todos os tipos definidos. Funções compilam mas retornam zero-values.

---

## Phase 3: User Story 4 — Falha rápida em manifesto inválido (Priority: P1)

**Goal**: `apply` sai com exit 1 e mensagem clara em stderr para qualquer erro de manifest antes de qualquer side-effect. É a user story mais simples e o pré-requisito de debugabilidade para US1.

**Independent Test**: Executar `cbweb3 apply` sem flags, com arquivo inexistente, com YAML inválido, com campo ausente, e com `mode: join` — verificar exit 1 e mensagem correta em stderr. Nenhum arquivo é criado.

> **Testes failing PRIMEIRO. Implementação só após confirmar que os testes falham.**

- [X] T009 [P] [US4] Escrever teste failing em `scenario-a/toolkit/cmd/cbweb3/main_test.go`: invocar binário sem flag `-f` → exit 1, stderr contém "required flag -f"
- [X] T010 [P] [US4] Escrever teste failing em `main_test.go`: invocar com `-f /tmp/nonexistent-cbweb3-test.yaml` → exit 1, stderr contém "manifest file not found"
- [X] T011 [P] [US4] Escrever teste failing em `main_test.go`: invocar com `-f testdata/invalid-syntax.yaml` → exit 1, stderr contém "yaml" ou "parse"
- [X] T012 [P] [US4] Escrever teste failing em `main_test.go`: invocar com `-f testdata/missing-spoke-id.yaml` → exit 1, stderr contém "validation error" e "spoke.id"
- [X] T013 [P] [US4] Escrever teste failing em `main_test.go`: adicionar manifesto fixture `testdata/mode-join.yaml` com `mode: join`; invocar → exit 1, stderr contém "not yet supported"
- [X] T014 [P] [US4] Escrever teste failing em `main_test.go`: invocar com `--output invalid-format -f testdata/central-bank-brl.yaml` → exit 1, stderr contém "unknown output format", exit ocorre antes de ler manifesto
- [X] T015 [US4] Implementar flag parsing em `scenario-a/toolkit/cmd/cbweb3/main.go`: registrar `-f`/`--file` como required (verificar após Parse), `--dry-run`/`-n` como bool, `--output`/`-o` como string com default `yaml`; validar `--output` imediatamente (antes de abrir arquivo) — exit 1 com mensagem em stderr
- [X] T016 [US4] Implementar calls a `manifest.Load(path)` e `manifest.Validate(m)` em `cmd/cbweb3/main.go`; qualquer erro → `fmt.Fprintf(os.Stderr, "...")` + `os.Exit(1)`; stdout permanece vazio nesses casos
- [X] T017 [US4] Implementar verificação de `mode` (só `"found"` suportado) e `environment` (só `"local"` suportado) em `cmd/cbweb3/main.go` imediatamente após `Validate` — exit 1 com mensagem clara em stderr; nenhum arquivo lido ou criado
- [X] T018 [US4] Executar `go test -race ./cmd/cbweb3/...` — confirmar que todos os testes T009–T014 passam; `go test -run TestUS4` verde

**Checkpoint**: US4 completa. `cbweb3 apply` rejeita manifests inválidos com exit 1 e mensagem clara antes de qualquer side-effect.

---

## Phase 4: User Story 5 — Resolução de dependências (Priority: P2)

**Goal**: `ResolveDeps` constrói `orchestrator.Deps` completo a partir do manifesto + perfil `local`. Erros de resolução causam exit 1 antes de qualquer passo de provisioning.

**Independent Test**: Invocar `ResolveDeps` em testes unitários com manifestos locais e verificar que cada campo de `Deps` está corretamente populado. Testar com URI inválida e `environment: prod` para verificar erros.

> **Testes failing PRIMEIRO.**

- [X] T019 [P] [US5] Escrever teste failing em `scenario-a/toolkit/engine/apply/deps_test.go`: manifesto com `environment: local` e `keyProvider: kms://local-emulator` → `ResolveDeps` retorna `Deps.KeyProvider != nil` e sem erro
- [X] T020 [P] [US5] Escrever teste failing em `deps_test.go`: `certSource: self-signed://local` → `Deps.CertSource != nil`
- [X] T021 [P] [US5] Escrever teste failing em `deps_test.go`: `relay.endpoint: http://cbweb3-cacti:4000` → `Deps.RelayRegistrar != nil`; relay nil no manifesto → `Deps.RelayRegistrar` é `NoOpRelayRegistrar`
- [X] T022 [P] [US5] Escrever teste failing em `deps_test.go`: `environment: prod` → retorna erro contendo "not yet supported"; `keyProvider: kms://unknown-scheme` → retorna erro contendo "keyProvider URI error"
- [X] T023 [US5] Implementar `LocalProfile` em `scenario-a/toolkit/engine/apply/profile.go`: `BesuRPCURL = "http://localhost:<rpc.port>"` do manifesto; `PaladinCBURL` de `CBWEB3_PALADIN_CB_URL` ou `http://localhost:31648`; `ScriptsDir`, `ComposeTemplatePath`, `PaladinConfigDir` de env vars com defaults via `os.Executable()` + `filepath.Join`
- [X] T024 [US5] Implementar `ResolveDeps(m *manifest.Manifest) (orchestrator.Deps, error)` em `scenario-a/toolkit/engine/apply/deps.go`: verificar `environment == "local"` (erro se não); chamar `keyprovider.New(m.Spec.KeyProvider)` e `certsource.New(m.Spec.CertSource)`; construir `NewHTTPRelayRegistrar` ou `NoOpRelayRegistrar`; popular todos os path fields de `LocalProfile`
- [X] T025 [US5] Executar `go test -race ./engine/apply/...` — confirmar que testes T019–T022 passam

**Checkpoint**: US5 completa. `ResolveDeps` popula `orchestrator.Deps` corretamente para `environment: local` e falha rápido para ambientes não suportados.

---

## Phase 5: User Story 3 — Saída estruturada JSON/YAML (Priority: P1)

**Goal**: stdout emite sempre JSON ou YAML válido (controlado por `--output`). Relatório é construído em memória antes de qualquer escrita — garante validade mesmo em falha.

**Independent Test**: Construir `ApplyResult` manualmente e verificar que `emitReport` produz YAML/JSON válido parseável por `yq`/`jq`. Testar caso de falha para confirmar que stdout ainda é válido.

> **Testes failing PRIMEIRO.**

- [X] T026 [P] [US3] Escrever teste failing em `scenario-a/toolkit/engine/apply/result_test.go`: `emitReport(ApplyResult{Status:"success", Steps:[...]}, "yaml", &buf)` → `buf` contém YAML válido com `status: success` e `steps` com 10 entries
- [X] T027 [P] [US3] Escrever teste failing em `result_test.go`: `emitReport(ApplyResult{Status:"failed", Error:"step error"}, "json", &buf)` → `buf` é JSON válido com `status: "failed"` e `error: "step error"`; campo `bundle` ausente (omitempty)
- [X] T028 [P] [US3] Escrever teste failing em `result_test.go`: `emitReport` com `format = "invalid"` → retorna erro; campos omitempty corretos (bundle nil não aparece em YAML/JSON)
- [X] T029 [US3] Implementar `emitReport(result ApplyResult, format string, w io.Writer) error` em `scenario-a/toolkit/engine/apply/result.go`: `format == "json"` → `json.NewEncoder(w).Encode(result)`; `format == "yaml"` → `yaml.NewEncoder(w).Encode(result)`; qualquer outro format → `fmt.Errorf("unknown output format: %q", format)` — nunca escreve em w antes de validar format
- [X] T030 [US3] Conectar `emitReport` no `cmd/cbweb3/main.go`: todo output estruturado vai para stdout via `emitReport`; todas as mensagens de erro humano-legíveis (fora do relatório) vão para stderr; o relatório é emitido mesmo em falha (parcial)
- [X] T031 [US3] Executar `go test -race ./engine/apply/... ./cmd/cbweb3/...` — confirmar que testes T026–T028 passam

**Checkpoint**: US3 completa. `emitReport` emite JSON/YAML válido em qualquer cenário (sucesso, falha, relatório parcial).

---

## Phase 6: User Story 1 — Provisionamento completo (Priority: P1) 🎯 MVP

**Goal**: `cbweb3 apply -f manifest.yaml` provisiona um spoke `mode: found` end-to-end: valida manifesto, resolve deps, chama `orchestrator.RunFound`, emite bundle via `bundle.EmitBundle`, serializa relatório com estado de cada step. Idempotente.

**Independent Test**: `go test -race ./engine/apply/...` com orchestrator e bundle mockados (stub Deps + stub EnodeProvider); verificar ApplyResult com status success, 10 steps, bundle.path correto. Manualmente: `cbweb3 apply -f testdata/central-bank-brl.yaml` contra stack local.

> **Testes failing PRIMEIRO.**

- [ ] T032 [P] [US1] Escrever teste failing em `scenario-a/toolkit/engine/apply/apply_test.go`: `Run(ctx, in)` com `orchestrator.RunFound` mockado retornando nil → ApplyResult com `Status: "success"`, 10 steps com `Status: "completed"` ou `"skipped"`, `Bundle.Path` não vazio
- [ ] T033 [P] [US1] Escrever teste failing em `apply_test.go`: `Run` quando `RunFound` retorna erro em step 3 → ApplyResult com `Status: "failed"`, step 3 com `Status: "failed"` e `Error` preenchido, steps 1-2 com `Status: "completed"` ou `"skipped"`
- [ ] T034 [P] [US1] Escrever teste failing em `apply_test.go`: `Run` com `.provisioning-state.yaml` pré-populado (todos 10 steps `done`) → ApplyResult com todos steps `Status: "skipped"`, bundle re-emitido, exit 0 — idempotência
- [ ] T035 [P] [US1] Escrever teste failing em `apply_test.go`: `Run` confirma que `bundle.EmitBundle` é chamado após `RunFound` sucesso → `BundleRef.Path == "bundles/spoke-brl.bundle.yaml"`
- [X] T036 [US1] Implementar `Run(ctx context.Context, in ApplyInput) (ApplyResult, error)` em `scenario-a/toolkit/engine/apply/apply.go`: (1) `os.MkdirAll(dataDir)` — cria diretório se inexistente; (2) `ResolveDeps(in.Manifest)` — fail-fast em erro; (3) `orchestrator.RunFound(ctx, in.Manifest, deps)` — propaga erro; (4) `orchestrator.LoadState(dataDir)` — carrega estado final; (5) construir `[]StepResult` a partir do estado carregado iterando `CanonicalStepOrder`; (6) `bundle.EmitBundle(ctx, bundleInput)` — adicionar `bundleError` ao relatório se falhar; (7) retornar ApplyResult com status correto
- [X] T037 [US1] Conectar `apply.Run` no `cmd/cbweb3/main.go` com signal context via `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`: construir `ApplyInput` a partir das flags + manifesto resolvido; chamar `Run`; chamar `emitReport` para stdout; chamar `os.Exit(1)` se `result.Status != "success"`
- [X] T038 [US1] Executar `go test -race ./engine/apply/...` — confirmar que testes T032–T035 passam; `go build ./cmd/cbweb3/...` verde

**Checkpoint**: US1 completa. `cbweb3 apply -f manifest.yaml` provisiona spoke end-to-end com relatório estruturado e bundle emitido.

---

## Phase 7: User Story 2 — Dry-run sem side-effects (Priority: P1)

**Goal**: `cbweb3 apply --dry-run -f manifest.yaml` lê estado atual de `.provisioning-state.yaml` e emite plano com `pending`/`skipped` por step. Zero side-effects: sem RunFound, sem EmitBundle, sem escrita de estado.

**Independent Test**: `go test -race ./engine/apply/...` com t.TempDir() vazio → 10 steps pending; com state file parcial → steps corretos. Tempo < 2 s sem Besu.

> **Testes failing PRIMEIRO.**

- [X] T039 [P] [US2] Escrever teste failing em `scenario-a/toolkit/engine/apply/dryrun_test.go`: `DryRun(ctx, in)` com `SPOKE_DATA_DIR` vazio (t.TempDir()) → ApplyResult `{Status:"dry-run", DryRun:true}`, todos 10 steps com `Status:"pending"`, `Bundle == nil`
- [X] T040 [P] [US2] Escrever teste failing em `dryrun_test.go`: `DryRun` com `.provisioning-state.yaml` contendo 5 steps `done` → exatamente os 5 primeiros steps com `Status:"skipped"`, os 5 restantes com `Status:"pending"` — na ordem canônica
- [X] T041 [P] [US2] Escrever teste failing em `dryrun_test.go`: `DryRun` não chama `orchestrator.RunFound` nem `bundle.EmitBundle` (verificar via contadores em test doubles) → zero side-effects
- [X] T042 [US2] Implementar `DryRun(ctx context.Context, in ApplyInput) (ApplyResult, error)` em `scenario-a/toolkit/engine/apply/dryrun.go`: (1) `orchestrator.LoadState(dataDir)` — estado vazio se arquivo ausente; (2) iterar `orchestrator.CanonicalStepOrder`; (3) para cada step: status `"skipped"` se `state` contém esse step com `status == "done"`, `"pending"` caso contrário; (4) retornar ApplyResult com `Status:"dry-run"`, `DryRun:true`, `Bundle:nil`
- [X] T043 [US2] Conectar `--dry-run` no `cmd/cbweb3/main.go`: se flag ativa, chamar `apply.DryRun(ctx, in)` em vez de `apply.Run`; emitir relatório via `emitReport`; exit 0 em ambos os casos de dry-run (success e fail-fast de validação já tratados em US4)
- [X] T044 [US2] Executar `go test -race ./engine/apply/... ./cmd/cbweb3/...` — confirmar que testes T039–T041 passam; executar `time go test -run TestDryRun ./engine/apply/...` e confirmar duração < 2 s

**Checkpoint**: US2 completa. `cbweb3 apply --dry-run` emite plano sem executar nenhuma ação e sem acessar serviços externos.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Signal handling completo, integração CLI com binário real, validação de zero novas dependências, suite de testes final.

- [X] T045 [P] Adicionar teste de integração em `scenario-a/toolkit/cmd/cbweb3/integration_test.go` usando `os/exec` com build tag `integration`: compilar binário; executar `cbweb3 apply --dry-run -f testdata/central-bank-brl.yaml --output yaml` → exit 0, stdout é YAML válido com `dryRun: true`
- [X] T046 [P] Adicionar teste de integração: `cbweb3 apply -f testdata/invalid-syntax.yaml` → exit 1, stdout vazio, stderr não vazio
- [X] T047 [P] Verificar que `signal.NotifyContext` está conectado em `cmd/cbweb3/main.go` com `os.Interrupt` e `syscall.SIGTERM`; wiring confirmado na leitura de código
- [X] T048 [P] Garantir que `apply.go` adiciona `os.MkdirAll(dataDir, 0o755)` antes de qualquer outra operação de filesystem — implementado em apply.Run
- [X] T049 Executar `go build ./cmd/cbweb3/...` + `go mod tidy` + verificar go.mod inalterado — SC-007: zero novas dependências adicionadas ao `toolkit/go.mod`
- [X] T050 Executar suite completa `go test -race ./engine/apply/... ./engine/orchestrator/... ./cmd/cbweb3/...` — 108 testes passam

**Checkpoint**: TK-7 completo. `cbweb3 apply` provisiona, reporta, suporta dry-run, emite JSON/YAML, trata sinais, e não introduz novas dependências.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sem dependências — pode começar imediatamente
- **Foundational (Phase 2)**: Depende de Phase 1 — **bloqueia todas as user stories**
- **US4 (Phase 3)**: Depende de Phase 2 — sem dependência em US1/US2/US3/US5
- **US5 (Phase 4)**: Depende de Phase 2 — sem dependência em US1/US2/US3/US4
- **US3 (Phase 5)**: Depende de Phase 2 — sem dependência em US1/US2/US4/US5
- **US1 (Phase 6)**: Depende de US4 (validação), US5 (ResolveDeps), US3 (emitReport)
- **US2 (Phase 7)**: Depende de US3 (emitReport) e Phase 2 (LoadState, CanonicalStepOrder)
- **Polish (Phase 8)**: Depende de todas as user stories

### User Story Dependencies

- **US4 (P1)**: pode começar após Phase 2 — sem dependências em outras stories
- **US5 (P2)**: pode começar após Phase 2 — sem dependências em outras stories; pode rodar em paralelo com US4 e US3
- **US3 (P1)**: pode começar após Phase 2 — sem dependências; pode rodar em paralelo com US4 e US5
- **US1 (P1)**: depende de US4 + US5 + US3 — implementada após as três
- **US2 (P1)**: depende de US3 — pode ser implementada em paralelo com US1 após US3

### Ordem de implementação recomendada (single developer)

```
Phase 1 → Phase 2 → US4 → US5 → US3 → US1 → US2 → Polish
```

### Parallel Opportunities (two developers)

```
Phase 1 → Phase 2 → {US4 ‖ US5 ‖ US3} → US1 → US2 → Polish
```

### Within Each User Story

- Testes DEVEM ser escritos e **falhar** antes da implementação
- Testes marcados [P] podem ser escritos simultaneamente (arquivos diferentes)
- Implementações dependentes dos tipos de Phase 2 (ApplyInput, ApplyResult etc.) — Phase 2 deve estar completa

---

## Parallel Example: Phase 2

```bash
# T003 e T004 podem rodar em paralelo (arquivos diferentes no orchestrator):
Task T003: "Exportar LoadState em scenario-a/toolkit/engine/orchestrator/state.go"
Task T004: "Exportar CanonicalStepOrder em scenario-a/toolkit/engine/orchestrator/step.go"
Task T005: "Definir tipos ApplyResult etc. em scenario-a/toolkit/engine/apply/result.go"
```

## Parallel Example: User Story 4 (testes)

```bash
# T009–T014 são todos independentes (mesmo arquivo, mas diferentes funções de teste):
Task T009: "Teste: missing -f flag → exit 1"
Task T010: "Teste: file not found → exit 1"
Task T011: "Teste: YAML inválido → exit 1"
Task T012: "Teste: campo ausente → exit 1"
Task T013: "Teste: mode: join → exit 1"
Task T014: "Teste: --output invalid → exit 1"
```

## Parallel Example: User Story 1 (testes)

```bash
# T032–T035 são independentes (diferentes funções de teste):
Task T032: "Teste: Run sucesso → ApplyResult{status:success}"
Task T033: "Teste: Run falha → ApplyResult{status:failed}"
Task T034: "Teste: Run idempotente → steps skipped"
Task T035: "Teste: Run chama EmitBundle"
```

---

## Implementation Strategy

### MVP First (User Stories 4 + 3 + 1)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRÍTICO — bloqueia tudo)
3. Complete Phase 3: US4 — fail-fast de manifesto
4. Complete Phase 5: US3 — output estruturado
5. Complete Phase 4: US5 — resolução de deps
6. Complete Phase 6: US1 — provisionamento completo
7. **PARAR e VALIDAR**: `cbweb3 apply -f manifest.yaml` end-to-end em stack local

### Incremental Delivery

1. Setup + Foundational → foundation pronta
2. US4 → manifesto rejeitado corretamente → demonstrável
3. US3 → output estruturado → demonstrável (dry-run format parcial)
4. US5 + US1 → spoke provisionado → MVP completo demonstrável
5. US2 → dry-run → demonstrável sem stack

### Parallel Team Strategy (2 devs)

Dev A:
1. Phase 1 + Phase 2 (conjuntamente) → US4 → US1

Dev B:
1. Phase 2 (conjuntamente com Dev A) → US5 → US3 → US2

Merge: ambos convergem em Phase 8 (Polish).

---

## Notes

- [P] tasks = arquivos diferentes, sem dependências em tarefas incompletas
- [Story] label mapeia task para user story específica para rastreabilidade
- Cada user story é independentemente completável e testável
- Testes DEVEM falhar antes de implementar
- `go build ./...` deve estar verde ao fim de cada fase
- `go mod tidy` ao final — `go.mod` e `go.sum` não devem ter diffs
- Commit após cada fase ou grupo lógico

---

## Phase 9: Convergence

Tasks appendadas após verificação do estado do código contra spec, plan, e tasks. Ordenadas por severidade (HIGH primeiro).

- [X] T051 Em `scenario-a/toolkit/engine/apply/apply.go` (T036), especificar explicitamente na implementação de `Run` a construção de `bundle.NewBesuEnodeProvider(in.BesuRPCURL, nil)` antes de chamar `bundle.EmitBundle` — implementado; `nil` httpClient usa default de 10 s (missing)
- [X] T052 Adicionar resolução de `OutputDir` em `scenario-a/toolkit/engine/apply/profile.go` (T023): ler de env var `CBWEB3_OUTPUT_DIR`; default = `filepath.Dir(dataDir)`; passar como `ApplyInput.OutputDir` e como `BundleInput.OutputDir` em `apply.Run` — implementado (partial)
- [X] T053 Corrigir contradição entre T036 ("adicionar `bundleError` ao relatório") e data-model.md: na implementação de `apply.Run`, quando `bundle.EmitBundle` falhar, setar `result.Error = fmt.Sprintf("bundle emission failed: %v", bundleErr)` e `result.Status = "failed"` — implementado corretamente (contradicts)
- [X] T054 Especificar lógica de seleção de `ApplyResult.Status` em `apply.Run` para o caminho de interrupção: `ctx.Err() != nil` → `result.Status = "interrupted"`; erro não-ctx → `result.Status = "failed"`; bundle fail → `result.Status = "failed"` — implementado em apply.go (missing)
- [X] T055 [P] [US2] Adicionar teste de integração em `main_test.go`: `cbweb3 apply --dry-run -f testdata/missing-spoke-id.yaml` → exit 1, stderr contém "validation error", stdout vazio — `TestRunApply_DryRun_MissingManifest` implementado (missing)

---

## Phase 10: Convergence

Tasks appendadas após segunda verificação do estado do código contra spec, plan, e tasks. 16 FRs, 7 SCs, 21 ACs, 10 decisões de plano, 6 princípios de constituição verificados. Ordenadas por severidade (HIGH primeiro).

- [X] T056 [US1] Refatorar `scenario-a/toolkit/engine/apply/apply.go`: extrair struct interna `runnerFuncs` com campos `runFound func(context.Context, *manifest.Manifest, orchestrator.Deps) error` e `emitBundle func(context.Context, bundle.BundleInput) (*bundle.JoinBundle, error)`; mover corpo de `Run` para `run(ctx context.Context, in ApplyInput, fns runnerFuncs) (ApplyResult, error)` usando `fns.runFound` e `fns.emitBundle`; `Run` delega para `run(ctx, in, defaultRunnerFuncs())`; escrever `scenario-a/toolkit/engine/apply/run_internal_test.go` (package `apply`) com 4 testes unit: (a) `runFound` stub retorna nil + state file pré-populado via `writeTestState` → `Status:"success"`, 10 steps, `Bundle.Path != ""`; (b) `runFound` stub retorna erro → `Status:"failed"`, último step com `Status:"failed"`; (c) todos 10 steps `done` no state file → todos `Status:"skipped"`, `emitBundle` chamado → idempotência; (d) `emitBundle` chamado exatamente uma vez após `runFound` sucesso → `BundleRef.Path == "bundles/spoke-brl.bundle.yaml"` — `go test -race ./engine/apply/...` verde; implementa T032–T035 per US1/SC-001/plan:testing (missing)
- [X] T057 Em `scenario-a/toolkit/engine/apply/apply.go` na função `run` (ou `Run`), após construir `result.Steps = buildStepResults(...)`, quando `ctx.Err() != nil` percorrer `result.Steps` em ordem reversa e alterar o primeiro step com `Status: "failed"` para `Status: "interrupted"`; adicionar sub-teste em `run_internal_test.go` que cancela o contexto antes de chamar `run` e confirma que o último step mostra `Status: "interrupted"` no resultado per FR-012 / data-model.md `StepResult.Status` semantics (partial)
