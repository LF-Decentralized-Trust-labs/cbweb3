# Tasks: TK-5 — Motor de orquestração (`mode: found`)

**Input**: Design documents from `/specs/027-tk5-orchestration-engine/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Tests**: Incluídos — a spec (SC-001) e a Constitution (Princípio V) exigem test-first explicitamente.

**Organization**: Agrupado por user story. US1+US5 compartilham a fase de implementação dos 10 passos (US5 é o passo 9 de US1). US2/US3/US4 verificam propriedades transversais (idempotência, retomada, logs).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependência de tarefa incompleta)
- **[Story]**: User story correspondente (US1–US5)

---

## Phase 1: Setup — Scaffolding e Templates

**Purpose**: Criar estrutura de diretórios, templates Paladin e tipos de base sem lógica de negócio.

- [ ] T001 Criar estrutura de diretórios do pacote: `scenario-a/toolkit/engine/orchestrator/` e `scenario-a/toolkit/engine/orchestrator/steps/`
- [ ] T002 [P] Criar template Paladin Compose `scenario-a/provisioning/templates/central-bank/paladin-compose.yaml` com variáveis `SPOKE_ID`, `PALADIN_CB_RPC_PORT`, `PALADIN_CB_WS_PORT`, `PALADIN_CB_GRPC_PORT`, `PALADIN_IMAGE`, `SPOKE_DATA_DIR`, `SPOKE_NETWORK_NAME` (decisão R-01 de research.md)
- [ ] T003 [P] Criar template de config do nó CB `scenario-a/provisioning/templates/central-bank/paladin-config/central-bank/config.yaml.tmpl` baseado em `deploy/local/paladin/spoke-a/config/central-bank/config.yaml` com substituição `${REGISTRY_CONTRACT_ADDRESS}`, `${ZETO_FACTORY_ADDRESS}`, `${PENTE_FACTORY_ADDRESS}`, `${BESU_RPC_URL}`
- [ ] T004 [P] Criar template de config de nó banco `scenario-a/provisioning/templates/central-bank/paladin-config/bank/config.yaml.tmpl` análogo ao T003

**Checkpoint**: Estrutura de arquivos pronta para implementação.

---

## Phase 2: Foundational — Tipos base, estado e infraestrutura

**Purpose**: Tipos Go, interface `Step`, gerenciamento de estado, logger JSON e esqueleto de `RunFound`. Estes componentes bloqueiam todos os passos.

**⚠️ CRITICAL**: Fase 2 deve estar completa antes de qualquer user story começar.

- [ ] T005 Definir interface `Step`, constantes `StepDeployContracts`…`StepRegisterRelay` e lista canônica de 10 nomes em `scenario-a/toolkit/engine/orchestrator/step.go`
- [ ] T006 [P] Definir structs `Deps`, `Timeouts`, `SpokeInfo`, interface `RelayRegistrar` e struct `NoOpRelayRegistrar` em `scenario-a/toolkit/engine/orchestrator/deps.go` conforme `contracts/orchestrator-api.md`
- [ ] T007 [P] Implementar `logLine(w io.Writer, entry logEntry)` em `scenario-a/toolkit/engine/orchestrator/logger.go`: emite JSON com campos `ts`, `severity`, `service`, `spoke_id`, `step`, `action` (e `error`/`reason` quando aplicável) — contrato completo em `contracts/orchestrator-api.md`
- [ ] T008 Implementar `parseDeployedAddrs(path string) (DeployedAddrs, error)` e struct `DeployedAddrs` em `scenario-a/toolkit/engine/orchestrator/addrs.go` + testes unitários em `addrs_test.go` (casos: arquivo inexistente, chave ausente, chave com valor vazio, arquivo bem formado)
- [ ] T009 Implementar `ProvisioningState`, `StepState`, `loadState`, `saveState` (escrita atômica via temp+rename), `lockState` (`syscall.Flock LOCK_EX|LOCK_NB`) em `scenario-a/toolkit/engine/orchestrator/state.go` + testes unitários em `state_test.go` (casos: arquivo inexistente → todos pending, estado parcial, lock concorrente → `ErrProvisioningLocked`)
- [ ] T010 Escrever testes failing para `RunFound` em `scenario-a/toolkit/engine/orchestrator/orchestrator_test.go`: (a) `ErrGenesisNotFound` quando genesis ausente, (b) `ErrProvisioningLocked` quando lock ocupado, (c) fluxo completo com todos os steps mockados retornando sucesso → verifica arquivo de estado com todos `done`
- [ ] T011 Implementar esqueleto de `RunFound` em `scenario-a/toolkit/engine/orchestrator/orchestrator.go`: verificação de genesis, file lock, loop sobre steps (chama `check()`→skip ou `run()`→persiste estado), propagação de erro — os testes de T010 devem passar com steps mockados

**Checkpoint**: `go test -race ./engine/orchestrator/... -run TestUnit` verde. Fundação pronta.

---

## Phase 3: US1 + US5 — Fluxo completo de provisionamento (Priority: P1) 🎯 MVP

**Goal**: `RunFound` executa os 10 passos em sequência; ao final o Paladin CB responde, contratos deployados, banco central registrado on-chain, relay ciente do spoke. US5 (onboarding real) é o passo 9 desta sequência.

**Independent Test**: Invocar `RunFound` com manifesto `spoke-brl`, verificar: `ptx_getTransaction` responde `PD020704`, `IdentityRegistry.isParticipant(evmAddress)` retorna `true`, arquivo de estado com 10 entradas `done`.

### Passo 1 — `deploy-contracts`

- [ ] T012 [US1] Escrever testes failing para passo 1 em `scenario-a/toolkit/engine/orchestrator/steps/deploy_contracts_test.go`: `check()` retorna `true` quando `.deployed-addrs.env` tem `REGISTRY_CONTRACT_ADDRESS` não vazio; `check()` retorna `false` quando ausente; `run()` invoca os três `go test` via `os/exec` com `SPOKE=` e `BESU_RPC_URL=` corretos
- [ ] T013 [US1] Implementar passo 1 `deployContractsStep.Check()` + `Run()` em `scenario-a/toolkit/engine/orchestrator/steps/deploy_contracts.go`: `Check` lê `DeployedAddrs`, `Run` invoca `TestDeployEVMRegistry`, `TestDeployZetoFactory`, `TestDeployPenteFactory` via `os/exec` com timeout `Deps.Timeouts.GoTestStep`; copia `.deployed-addrs.env` para `SPOKE_DATA_DIR` após escrita pelos scripts

### Passo 2 — `gen-tls`

- [ ] T014 [P] [US1] Escrever testes failing para passo 2 em `scenario-a/toolkit/engine/orchestrator/steps/gen_tls_test.go`: `check()` retorna `true` quando `<SPOKE_DATA_DIR>/tls/central-bank.crt` existe; `run()` gera self-signed cert para o CB (OU=`ROLE_CENTRAL_BANK`) usando `crypto/x509` stdlib e escreve em `<SPOKE_DATA_DIR>/tls/central-bank.crt` (nota: NÃO usa `CertSource.IssueLeafCert` — veja research R-03; `CertSource` é para `mode:join`)
- [ ] T015 [P] [US1] Implementar passo 2 em `scenario-a/toolkit/engine/orchestrator/steps/gen_tls.go`: gerar keypair ECDSA P-256 via `crypto/elliptic` + `crypto/rand`; obter pubkey via `deps.KeyProvider.GetPublicKey(ctx, spokeID+"/cb")`; criar cert self-signed X.509 com OU=`ROLE_CENTRAL_BANK` via `x509.CreateCertificate`; escrever PEM em `<SPOKE_DATA_DIR>/tls/central-bank.crt`

### Passo 3 — `render-configs`

- [ ] T016 [P] [US1] Escrever testes failing para passo 3 em `scenario-a/toolkit/engine/orchestrator/steps/render_configs_test.go`: `check()` retorna `true` quando `<SPOKE_DATA_DIR>/paladin/central-bank/config.yaml` existe; `run()` renderiza templates de `PaladinConfigTemplateDir` com endereços de `.deployed-addrs.env` e escreve configs em `<SPOKE_DATA_DIR>/paladin/<node>/config.yaml`
- [ ] T017 [P] [US1] Implementar passo 3 em `scenario-a/toolkit/engine/orchestrator/steps/render_configs.go`: ler `DeployedAddrs`, iterar templates de `deps.PaladinConfigTemplateDir` via `text/template`, escrever `config.yaml` para cada nó (central-bank, bank-a, bank-c por padrão)

### Passo 4 — `register-nodes`

- [ ] T018 [US1] Escrever testes failing para passo 4 em `scenario-a/toolkit/engine/orchestrator/steps/register_nodes_test.go`: `check()` retorna `true` somente se arquivo de estado tem passo `done`; `run()` invoca `TestRegisterPaladinNodes` via `os/exec`; erro "already registered" é tratado como sucesso idempotente
- [ ] T019 [US1] Implementar passo 4 em `scenario-a/toolkit/engine/orchestrator/steps/register_nodes.go`: `Check` consulta `ProvisioningState`; `Run` executa `go test -run TestRegisterPaladinNodes` com `SPOKE=` e `BESU_RPC_URL=`; verifica stderr/stdout para "already registered" e retorna nil nesse caso

### Passo 5 — `start-paladin`

- [ ] T020 [US1] Escrever testes failing para passo 5 em `scenario-a/toolkit/engine/orchestrator/steps/start_paladin_test.go`: `check()` retorna `true` quando container em estado `running` E health check `ptx_getTransaction` retorna `PD020704`; `run()` executa `docker compose down`, `docker volume rm`, `docker compose up -d` em sequência e polling do health check
- [ ] T021 [US1] Implementar passo 5 em `scenario-a/toolkit/engine/orchestrator/steps/start_paladin.go`: `Check` executa `docker compose -f <ComposeTemplatePath> ps --format json` + HTTP POST para health check; `Run` executa stop/clean/start via `os/exec` com as variáveis do manifesto; polling com `Deps.Timeouts.PaladinHealthCheck` e `PaladinHealthCheckInterval`

### Passos 6–8 — Zeto, Pente, FXAgreement

- [ ] T022 [P] [US1] Escrever testes failing para passos 6, 7, 8 em `steps/create_zeto_test.go`, `steps/create_pente_test.go`, `steps/deploy_fxa_test.go`: cada `check()` verifica chave específica em `.deployed-addrs.env`; cada `run()` invoca o `go test` correspondente com variáveis corretas
- [ ] T023 [P] [US1] Implementar passo 6 em `scenario-a/toolkit/engine/orchestrator/steps/create_zeto.go`: `Check` lê `ZETO_TOKEN_ADDRESS`; `Run` executa `TestCreateZetoTokenInstance` com `SPOKE=` e `PALADIN_CB_URL=`
- [ ] T024 [P] [US1] Implementar passo 7 em `scenario-a/toolkit/engine/orchestrator/steps/create_pente.go`: `Check` lê `PENTE_CONTEXT_GROUP_ID`; `Run` executa `TestCreatePenteContextBilateral` com `SPOKE=` e `PALADIN_CB_URL=`
- [ ] T025 [P] [US1] Implementar passo 8 em `scenario-a/toolkit/engine/orchestrator/steps/deploy_fxa.go`: `Check` lê `FX_AGREEMENT_DEPLOYED_AT`; `Run` executa `TestDeployFXAgreementPente` com `SPOKE=`, `REGISTRY_CONTRACT_ADDRESS=`, `PENTE_CONTEXT_GROUP_ID=`, `PALADIN_CB_URL=` lidos de `.deployed-addrs.env`

### Passo 9 — `onboard-registry` (US5)

- [ ] T026 [US5] Escrever testes failing para passo 9 em `scenario-a/toolkit/engine/orchestrator/steps/onboard_registry_test.go`: `check()` consulta `IdentityRegistry.isParticipant(evmAddress)` on-chain (mock do `ethclient`) e retorna `true` quando registrado; `run()` obtém pubkey via `KeyProvider.GetPublicKey`, deriva `evmAddress` via `keyprovider.EVMAddress`, assina nonce via `KeyProvider.Sign`, envia tx `registerParticipant` via `ethclient`; `ErrKeyProviderFailure` quando `GetPublicKey` falha
- [ ] T027 [US5] Implementar `check()` do passo 9 em `scenario-a/toolkit/engine/orchestrator/steps/onboard_registry.go`: instanciar `ethclient.Dial(deps.BesuRPCURL)`; ABI de `isParticipant(address) returns (bool)` embarcado como constante; `CallContract` e decodificar resultado bool
- [ ] T028 [US5] Implementar `run()` do passo 9 em `scenario-a/toolkit/engine/orchestrator/steps/onboard_registry.go`: `KeyProvider.GetPublicKey` → `keyprovider.EVMAddress` → nonce = `sha256(evmAddr+spokeID)` → `KeyProvider.Sign` → montar e enviar tx `registerParticipant(evmAddr, "Central Bank "+spokeID, RoleCentralBank, [32]byte{})` → aguardar receipt; em seguida `SetCertFingerprint(evmAddr, sha256(certPEM))`; timeout `Deps.Timeouts.OnboardRegistry`

### Passo 10 — `register-relay`

- [ ] T029 [US1] Escrever testes failing para passo 10 em `scenario-a/toolkit/engine/orchestrator/steps/register_relay_test.go`: `check()` chama `RelayRegistrar.IsRegistered` e retorna `true`/`false`; `run()` chama `RelayRegistrar.Register`; erro de `ErrRelayUnavailable` ou `ErrNotImplemented` NÃO propaga (step marcado `failed` mas engine continua)
- [ ] T030 [US1] Implementar passo 10 em `scenario-a/toolkit/engine/orchestrator/steps/register_relay.go`: `Check` chama `deps.RelayRegistrar.IsRegistered`; `Run` chama `deps.RelayRegistrar.Register` com `SpokeInfo` extraído do manifesto; capturar `ErrRelayUnavailable`/`ErrNotImplemented` e retornar nil (soft failure — passo fica como `failed` no estado mas engine não aborta)

### Wiring e teste de integração

- [ ] T031 [US1] Instanciar e wiring dos 10 steps no `RunFound` em `scenario-a/toolkit/engine/orchestrator/orchestrator.go`; executar teste de integração do fluxo completo com deps mockados (nenhum Docker real): todos os 10 steps executam em sequência, arquivo de estado contém 10 entradas `done`, `RunFound` retorna `nil`

**Checkpoint**: `go test -race ./engine/orchestrator/...` verde com todos os steps cobertos. US1+US5 completos.

---

## Phase 4: US2 — Idempotência total (Priority: P1)

**Goal**: Segunda chamada a `RunFound` num spoke já provisionado completa em < 2 s sem invocar subprocessos.

**Independent Test**: Chamar `RunFound` duas vezes no mesmo `SPOKE_DATA_DIR`; instrumentar `Step.Run()` para contar chamadas; verificar contador zero na segunda chamada.

- [ ] T032 [US2] Escrever testes de idempotência em `scenario-a/toolkit/engine/orchestrator/orchestrator_test.go`: (a) todos os passos `done` no arquivo de estado → `RunFound` retorna `nil` em < 2 s sem chamar nenhum `Run()`; (b) passo 1 `done`, passos 2-10 `pending` → apenas `Run()` dos passos 2-10 é chamado; (c) genesis ausente → `ErrGenesisNotFound` independente do arquivo de estado
- [ ] T033 [US2] Verificar idempotência por fonte externa (não apenas arquivo de estado): escrever teste onde arquivo de estado está ausente mas `.deployed-addrs.env` tem `REGISTRY_CONTRACT_ADDRESS` preenchido → passo 1 `check()` retorna `true` sem chamar `Run()` em `scenario-a/toolkit/engine/orchestrator/steps/deploy_contracts_test.go`
- [ ] T034 [P] [US2] Verificar idempotência dos passos 6, 7, 8 por `.deployed-addrs.env`: escrever testes em `steps/create_zeto_test.go`, `steps/create_pente_test.go`, `steps/deploy_fxa_test.go` confirmando que `Run()` não é invocado quando chave presente
- [ ] T035 [P] [US2] Verificar idempotência do passo 9 on-chain: escrever teste em `steps/onboard_registry_test.go` onde mock do `ethclient` retorna `isParticipant=true` → `check()` retorna `true`, `Run()` não chamado

**Checkpoint**: `go test -race ./engine/orchestrator/... -run TestIdempotent` verde. US2 completo.

---

## Phase 5: US3 — Retomada de execução interrompida (Priority: P2)

**Goal**: Engine interrompido no meio da sequência retoma do passo seguinte na próxima execução.

**Independent Test**: Injetar falha forçada no passo 7; verificar passos 1–6 com `done` no arquivo de estado; corrigir; re-executar e verificar que passos 1–6 são pulados.

- [ ] T036 [US3] Escrever testes de retomada em `scenario-a/toolkit/engine/orchestrator/orchestrator_test.go`: (a) arquivo de estado com passos 1–6 `done`, 7–10 `pending` → `Run()` chamado apenas para 7–10; (b) passo com `status: failed` no arquivo de estado → re-executado (não pulado); (c) passo em estado implicitamente `running` (arquivo ausente após crash) → tratado como `pending` e re-executado
- [ ] T037 [US3] Verificar escrita atômica do arquivo de estado em `scenario-a/toolkit/engine/orchestrator/state_test.go`: simulação de crash após `os.Rename` (arquivo de estado corrompido parcialmente) → `loadState` retorna estado anterior íntegro (sem corrupção)
- [ ] T038 [US3] Verificar que `RunFound` com contexto cancelado interrompe no passo corrente e retorna `context.DeadlineExceeded` em `scenario-a/toolkit/engine/orchestrator/orchestrator_test.go`: estado do passo interrompido permanece `pending` (não `done`)

**Checkpoint**: `go test -race ./engine/orchestrator/... -run TestResume` verde. US3 completo.

---

## Phase 6: US4 — Observabilidade (Priority: P2)

**Goal**: Cada transição de passo emite uma linha JSON válida em stdout com campos obrigatórios. Falhas incluem `error`. Skips incluem `reason: "already-done"`.

**Independent Test**: Capturar stdout de `RunFound`; validar que cada linha é JSON válido com os campos `ts`, `severity`, `service`, `spoke_id`, `step`, `action`; verificar `step_failed` tem `error`; verificar `step_skipped` tem `reason`.

- [ ] T039 [US4] Escrever testes de log em `scenario-a/toolkit/engine/orchestrator/orchestrator_test.go`: (a) execução bem-sucedida → cada passo emite exatamente `step_started` + `step_completed` com campos obrigatórios; (b) passo pulado por idempotência → emite `step_skipped` com `reason: "already-done"`, sem `step_started`; (c) passo falhando → emite `step_started` + `step_failed` com campo `error` não vazio
- [ ] T040 [US4] Escrever teste de formato JSON em `scenario-a/toolkit/engine/orchestrator/logger_test.go`: cada linha emitida por `logLine` é decodificável por `json.Unmarshal`; campo `ts` é ISO-8601 UTC válido; `severity` é `"INFO"` ou `"ERROR"`; campos ausentes quando não aplicável (`error` absent em successo, `reason` absent em falha)
- [ ] T041 [US4] Integrar `logLine` em `RunFound` e nos steps onde ainda não estiver presente; verificar que nenhum erro é swallowed silenciosamente (Constitution VI) — qualquer `Run()` que retorna erro deve ter emitido `step_failed` antes de propagar

**Checkpoint**: `go test -race ./engine/orchestrator/... -run TestLog` verde. US4 completo.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: `httpRelayRegistrar`, testes de integração E2E, documentação.

- [ ] T042 [P] Implementar `httpRelayRegistrar` em `scenario-a/toolkit/engine/orchestrator/deps.go`: `Register` faz `POST <endpoint>/api/v1/spokes` com payload `SpokeInfo` JSON; retorna `ErrRelayUnavailable` em timeout/connection refused; retorna `ErrNotImplemented` em HTTP 404/405 (relay RL-1 ainda não deployado); `IsRegistered` faz `GET <endpoint>/api/v1/spokes/<id>` e retorna `true` em HTTP 200
- [ ] T043 [P] Escrever testes unitários para `httpRelayRegistrar` em `scenario-a/toolkit/engine/orchestrator/deps_test.go` com `httptest.Server` mockando as respostas 200, 404, connection refused
- [ ] T044 Escrever teste de integração E2E em `scenario-a/toolkit/engine/orchestrator/orchestrator_test.go` (build tag `//go:build integration`): invocar `RunFound` completo contra Besu real + Docker; verificar SC-002 a SC-009 da spec; executável com `go test -race -tags integration -timeout 15m`
- [ ] T045 [P] Atualizar `scenario-a/toolkit/engine/orchestrator/orchestrator.go` com `DefaultTimeouts` e documentação inline das precondições de `RunFound`
- [ ] T046 [P] Executar `go vet ./engine/orchestrator/...` e `go test -race ./engine/orchestrator/...` sem flags `integration`; corrigir qualquer warning ou race condition identificado
- [ ] T047 Validar quickstart.md executando o fluxo manual descrito no arquivo — subir Besu com TK-4, invocar engine programaticamente, verificar Paladin health check e IdentityRegistry on-chain

**Checkpoint**: `go test -race ./engine/orchestrator/...` 100% verde. Integration test rodando em < 10 min. US1–US5 todos verificados.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sem dependências — pode começar imediatamente
- **Foundational (Phase 2)**: Depende do Phase 1 (T001–T004) — **bloqueia todas as user stories**
- **US1+US5 (Phase 3)**: Depende da Phase 2 — passos 1–10 em sequência interna
- **US2 (Phase 4)**: Depende da Phase 3 — verifica propriedades dos `check()` implementados
- **US3 (Phase 5)**: Depende da Phase 4 — verifica retomada sobre steps idempotentes
- **US4 (Phase 6)**: Depende da Phase 2 (logger) — pode rodar em paralelo com US2/US3
- **Polish (Phase 7)**: Depende de todas as fases anteriores

### User Story Dependencies

- **US1+US5 (P1)**: Pode começar após Phase 2 — núcleo do trabalho
- **US2 (P1)**: Depende de US1+US5 (verifica `check()` dos steps implementados)
- **US3 (P2)**: Depende de US2 (usa idempotência para validar retomada)
- **US4 (P2)**: Depende apenas de Phase 2 (logger.go) — pode avançar em paralelo com US2

### Parallel Opportunities

- T002, T003, T004 (Phase 1): paralelos entre si
- T006, T007 (Phase 2): paralelos entre si (deps.go e logger.go são arquivos independentes)
- T014+T015, T016+T017 (Phase 3, passos 2-3): paralelos com T012+T013 (passo 1) após fundação
- T022, T023, T024, T025 (Phase 3, passos 6-8): tests paralelos; impls paralelas após passo 5
- T033, T034, T035 (Phase 4): paralelos entre si
- T039, T040 (Phase 6): paralelos
- T042, T043, T045, T046 (Phase 7): paralelos

---

## Parallel Example: Phase 3 — Passos 2, 3 (após passo 1 concluído)

```bash
# Escrever testes dos passos 2 e 3 em paralelo:
Task T014: "Escrever testes failing para passo 2 (gen-tls) em steps/gen_tls_test.go"
Task T016: "Escrever testes failing para passo 3 (render-configs) em steps/render_configs_test.go"

# Implementar em paralelo após testes vermelhos:
Task T015: "Implementar passo 2 em steps/gen_tls.go"
Task T017: "Implementar passo 3 em steps/render_configs.go"
```

## Parallel Example: Phase 3 — Passos 6, 7, 8

```bash
# Testes e impls dos passos 6-8 (todos dependem do passo 5 concluído):
Task T022: "Escrever testes failing para passos 6, 7, 8"
# Após T022 verde (failing):
Task T023: "Implementar passo 6 (create-zeto)"    # arquivos distintos → paralelo
Task T024: "Implementar passo 7 (create-pente)"   # arquivos distintos → paralelo
Task T025: "Implementar passo 8 (deploy-fxa)"     # arquivos distintos → paralelo
```

---

## Implementation Strategy

### MVP First (US1 + US5 — Phase 3)

1. Completar Phase 1: Setup (templates)
2. Completar Phase 2: Foundational (tipos, estado, logger, RunFound skeleton)
3. Completar Phase 3: US1 + US5 (10 steps com test-first)
4. **PARAR e VALIDAR**: `go test -race ./engine/orchestrator/...` verde; invocar RunFound manualmente
5. Demonstrar: spoke-brl provisionado, Paladin respondendo, CB registrado on-chain

### Incremental Delivery

1. Phase 1+2 → Fundação pronta (types, state, logger)
2. Phase 3 → Fluxo completo funciona → MVP!
3. Phase 4 → Idempotência verificada
4. Phase 5 → Retomada testada
5. Phase 6 → Contrato de log validado
6. Phase 7 → Relay, integração E2E, polish

### Parallel Team Strategy

Com dois desenvolvedores após Phase 2:
- Dev A: Phase 3 passos 1–5 (deploy contracts → start Paladin)
- Dev B: Phase 6 (logger tests) + templates Paladin (T002–T004)

Dev A e Dev B reintegram em Phase 3 passos 6–10 e Phase 4.

---

## Notes

- [P] = arquivos distintos, sem dependência de tarefa incompleta no mesmo PR
- Testes **devem** falhar antes da implementação (Constitution V)
- Fazer commit após cada checkpoint de fase
- O arquivo de estado `running` nunca é escrito em disco (invariante crítico — data-model.md)
- Passo 10 (register-relay): falha soft — engine não aborta se relay indisponível
- Nenhum arquivo em `deploy/local/` ou `make/` é modificado neste PR
