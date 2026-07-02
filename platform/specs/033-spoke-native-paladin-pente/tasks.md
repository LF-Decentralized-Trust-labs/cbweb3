---
description: "Task list — 033 topologia nativa do toolkit (found CB-only + join dinâmico Paladin/Pente)"
---

# Tasks: Topologia nativa do toolkit — found CB-only e join dinâmico de Paladin/Pente

**Input**: `/specs/033-spoke-native-paladin-pente/` (spec.md, plan.md)
**Tests**: incluídos e **obrigatórios** (constituição: test-first em toda camada). Cada teste é escrito antes da implementação e deve falhar primeiro.
**Organization**: agrupado por user story (US1–US4) para implementação/validação independentes.

## Format: `[ID] [P?] [Story] Descrição`

- **[P]**: pode rodar em paralelo (arquivos distintos, sem dependência)
- Caminhos relativos a `scenario-a/` salvo indicação contrária.

## Convenções de caminho

- Motor: `toolkit/engine/orchestrator/`
- Bundle: `toolkit/engine/bundle/`
- Templates: `provisioning/templates/{central-bank,commercial-bank}/`
- Testes: co-localizados (`*_test.go`), padrão do repo.

---

## Phase 1: Setup

**Purpose**: Documentos de design da feature (spec-kit Fase 0/1).

- [ ] T001 Criar `specs/033-spoke-native-paladin-pente/research.md` confirmando as APIs Paladin necessárias: JSON-RPC de criação de grupo Pente (privacy group), deploy de contrato dentro do grupo, e o ABI `registerIdentity` do IdentityRegistry (extrair de `register_nodes_test.go` sem modificá-lo).
- [ ] T002 [P] Criar `specs/033-spoke-native-paladin-pente/data-model.md`: `PaladinNodeIdentity`, `PenteContext`, `ProvisioningState` (found 9 / join 15), `JoinBundle` revisado.
- [ ] T003 [P] Criar `specs/033-spoke-native-paladin-pente/contracts/` com as assinaturas Go das funções nativas: `RegisterPaladinNode`, `CreatePenteContext`, `DeployFXAInPente`.

---

## Phase 2: Foundational (bloqueia US1–US3)

**Purpose**: Primitivas nativas compartilhadas por found e join. **Nenhuma user story começa antes desta fase.**

- [ ] T004 [US-] Teste falhando para um cliente Paladin RPC nativo (`paladin_rpc_test.go`): health, chamada genérica JSON-RPC, com `httptest` mock.
- [ ] T005 [US-] Implementar `toolkit/engine/orchestrator/paladin_rpc.go`: cliente JSON-RPC mínimo para Paladin (reusar padrão de `join_rpc.go`).
- [ ] T006 [US-] Teste falhando para `RegisterPaladinNode` (`paladin_registry_test.go`): registra identidade com nome derivado de `spokeID`/`bankID`, chave-dono via `keyProvider` (mock), idempotente por nome ("já registrado" → sucesso), **sem** dev-key hardcoded e **sem** `switch` por spoke.
- [ ] T007 [US-] Implementar `toolkit/engine/orchestrator/paladin_registry.go`: `RegisterPaladinNode(ctx, deps, nodeName, ownerKeyID, certPath, grpcEndpoint)` — encode `registerIdentity` (ABI), assina/envia via BesuRPC, lê hash do evento; chave-dono obtida via `keyProvider.GetPublicKey`/`GenerateKey`.

**Checkpoint**: primitivas nativas prontas e testadas (unit, sem containers).

---

## Phase 3: User Story 1 — found CB-only (Priority: P1) 🎯 MVP

**Goal**: `found` provisiona apenas o CB para um `spec.spoke.id` arbitrário; sem `bank-a`/`bank-c`; sem Pente/FXA.

**Independent Test**: `apply` de found com `spoke.id` arbitrário → IdentityRegistry só com a identidade do CB do spoke; nenhum container `*-bank-a/-c`; bundle sem exigir Pente/FXA.

### Tests (escrever primeiro, devem falhar) ⚠️

- [ ] T008 [P] [US1] Teste: `CanonicalStepOrder` (found) tem 9 passos e **não** contém `create-pente-context`/`deploy-fxa-pente` (`step_test.go`).
- [ ] T009 [P] [US1] Teste: `register-nodes` (found) chama `RegisterPaladinNode` só para o nó CB derivado de `spec.spoke.id`, sem ler `spokeNodes()`/scripts de referência (`step_register_nodes_test.go`, mock registry).
- [ ] T010 [P] [US1] Teste: `render-configs` (found) renderiza só `central-bank` (`step_render_configs_test.go`).
- [ ] T011 [P] [US1] Teste: `readContracts` (bundle) exige só `registry/zetoFactory/penteFactory/zetoToken`; ausência de `PENTE_CONTEXT_*`/`FX_AGREEMENT_*` não falha (`bundle_test.go`, `bundle_validators_test.go`).
- [ ] T012 [P] [US1] Teste de template: `central-bank/paladin-compose.yaml` valida via `docker compose config` com apenas o serviço CB (sem `bank-a/bank-c`) (`provisioning/tests/test-central-bank-template.sh`).

### Implementação

- [ ] T013 [US1] `step.go`: remover `StepCreatePente` e `StepDeployFXAPente` de `CanonicalStepOrder` (found).
- [ ] T014 [US1] `orchestrator.go` `buildSteps`: remover `newCreatePenteStep` e `newDeployFXAStep` do found.
- [ ] T015 [US1] Reescrever `step_register_nodes.go` (found): usar `RegisterPaladinNode` nativo para o nó CB (`<spoke-id>-cb`), removendo o `go test TestRegisterPaladinNodes`.
- [ ] T016 [US1] `step_render_configs.go`: renderizar apenas `central-bank` (remover `bank-a`/`bank-c`).
- [ ] T017 [US1] `provisioning/templates/central-bank/paladin-compose.yaml`: remover serviços `paladin-bank-a`/`paladin-bank-c`, manter `paladin-cb` + `paladin-data-init` (só `cb`).
- [ ] T018 [US1] `bundle/bundle.go` `readContracts`: relaxar a lista obrigatória para os 4 contratos de nível-spoke; tornar campos Pente/FXA opcionais em `ContractsSpec`.
- [ ] T019 [US1] Atualizar contagens/asserts nos testes afetados (found 11→9): `orchestrator_test.go`, `orchestrator_integration_test.go`, `apply/run_internal_test.go`, `apply/apply_test.go`, `cmd/cbweb3/main_test.go`, `cmd/cbweb3/integration_test.go`.
- [ ] T020 [US1] Validar em run real: `apply` found CB-only com `spoke-brl` reaching `register-relay` (relay local via `start-cacti.sh`); confirmar identidade `spoke-brl-cb` on-chain e ausência de `spoke-a-*`.

**Checkpoint**: found CB-only funcional e verde, isolado.

---

## Phase 4: User Story 2 — banco entra com Paladin dinâmico (Priority: P1)

**Goal**: `mode: join` sobe o nó Paladin do banco (cert+config+container) e registra a identidade on-chain com nome derivado de `spec.bankId`.

**Independent Test**: sobre found CB-only, `apply` join de um banco → container Paladin do banco no ar + identidade `bank-itau` on-chain; sem nome hardcoded.

### Tests (escrever primeiro, devem falhar) ⚠️

- [ ] T021 [P] [US2] Teste: `CanonicalJoinStepOrder` contém, na ordem, `…proof-of-possession → gen-tls-join → render-config-join → start-paladin-join → register-paladin-node → create-pente-context → deploy-fxa-pente → start-backend` (`step_test.go`).
- [ ] T022 [P] [US2] Teste: `gen-tls-join` gera cert com CN/SAN derivados de `spec.bankId` (ex.: `paladin-<spoke>-bank-itau`), distinto da chave/CSR blockchain (`step_gen_tls_join_test.go`).
- [ ] T023 [P] [US2] Teste: `register-paladin-node` registra `<spoke>-<bankId>` via `RegisterPaladinNode`, chave-dono via `keyProvider`, idempotente (`step_register_paladin_node_test.go`).
- [ ] T024 [P] [US2] Teste de template: `commercial-bank/paladin-compose.yaml` valida com `BANK_ID`/`SPOKE_ID`/portas, mtls montado (`provisioning/tests/test-commercial-bank-paladin-template.sh`).

### Implementação

- [ ] T025 [P] [US2] Criar `provisioning/templates/commercial-bank/paladin-compose.yaml` (1 nó Paladin do banco; monta `config.yaml`+`tls.crt`+`tls.key` em `/etc/paladin`; rede externa do spoke).
- [ ] T026 [US2] `step_gen_tls_join.go`: gerar cert TLS do nó Paladin do banco (SAN por `bankId`), gravar em `<dataDir>/paladin/<bankId>/tls.{crt,key}`.
- [ ] T027 [US2] `step_render_config_join.go`: renderizar `config.yaml` do Paladin do banco (template parametrizado por `bankId`, `registryAddress` do bundle).
- [ ] T028 [US2] `step_start_paladin_join.go`: `docker compose up -d` do template commercial-bank Paladin (env: `BANK_ID`, portas, `PALADIN_IMAGE`, rede) + health-poll.
- [ ] T029 [US2] `step_register_paladin_node.go`: chamar `RegisterPaladinNode` para o nó do banco.
- [ ] T030 [US2] `orchestrator.go` `buildJoinSteps` + `step.go` `CanonicalJoinStepOrder`: inserir os 4 passos na ordem definida.
- [ ] T031 [US2] Atualizar contagens/asserts dos testes de join afetados (9→…): `run_join_internal_test.go`, `apply_join_test.go`, `apply/pendingJoinSteps`.
- [ ] T032 [US2] Validar em run real: join de 1 banco sobre o found CB-only; Paladin do banco no ar + identidade on-chain; logs do Paladin do CB sem erro TLS (descoberta reativa, ADR-002).

**Checkpoint**: banco entra dinamicamente com Paladin; sem topologia fixa.

---

## Phase 5: User Story 3 — Pente + FXAgreement no relacionamento CB↔banco (Priority: P2)

**Goal**: criar o grupo Pente bilateral (CB↔banco) e deployar o FXAgreement dentro dele, no join.

**Independent Test**: após join, grupo Pente com membros exatamente {CB, banco} e FXAgreement deployado no grupo.

### Tests (escrever primeiro, devem falhar) ⚠️

- [ ] T033 [P] [US3] Teste: `CreatePenteContext` cria grupo com os dois nós (CB, banco) como membros, idempotente (grupo existente → não recria) (`pente_test.go`, mock Paladin RPC).
- [ ] T034 [P] [US3] Teste: `DeployFXAInPente` deploya no grupo e retorna groupId/address; persiste em `.deployed-addrs.env` (`pente_test.go`).
- [ ] T035 [P] [US3] Teste: `create-pente-context`/`deploy-fxa-pente` (join) usam `penteFactoryAddress` do bundle e membros derivados (`step_*_test.go`).

### Implementação

- [ ] T036 [US3] `pente.go`: `CreatePenteContext(ctx, deps, members[])` e `DeployFXAInPente(ctx, deps, groupId)` via Paladin RPC nativo (parametrizado, sem `bank-a` fixo).
- [ ] T037 [US3] `step_create_pente_context_join.go` + `step_deploy_fxa_pente_join.go`: passos do join chamando a lógica nativa; persistir resultados.
- [ ] T038 [US3] Wire em `buildJoinSteps`/`CanonicalJoinStepOrder` (após `register-paladin-node`).
- [ ] T039 [US3] Validar em run real: grupo Pente {CB, banco} + FXAgreement no grupo.

**Checkpoint**: relacionamento pairwise criado dinamicamente.

---

## Phase 6: User Story 4 — idempotência e dry-run (Priority: P2)

**Goal**: re-`apply` retoma do ponto de falha; `-dry-run` lista os novos passos sem efeitos.

### Tests ⚠️

- [ ] T040 [P] [US4] Teste: re-`apply` join após falha em `start-paladin-join` pula anteriores e retoma (`run_join_internal_test.go`).
- [ ] T041 [P] [US4] Teste: `apply -dry-run` found lista 9 passos sem `create-pente`/`deploy-fxa`; join lista 15 passos (`apply/dryrun` + `cmd/cbweb3`).

### Implementação

- [ ] T042 [US4] Garantir `Check()` idempotente em todos os passos novos (cert/config/container/registro/pente) e persistência atômica de estado.
- [ ] T043 [US4] Ajustar `apply/result.go` `pendingSteps`/`pendingJoinSteps` e `buildStepResults` para a nova ordem.

**Checkpoint**: UX de idempotência/dry-run preservada em found e join.

---

## Phase 7: Polish & Cross-Cutting

- [ ] T044 [P] Atualizar `scenario-a/samples/README.md`: found agora é CB-only; o join traz o Paladin do banco e cria Pente; remover qualquer menção a topologia fixa.
- [ ] T045 [P] Atualizar `CLAUDE.md`/README do Scenario B? (N/A — só Scenario A); atualizar status no README do cenário.
- [ ] T046 Rodar `go test ./...` (toolkit) + `npx vitest run` (relay) + `go vet` — tudo verde.
- [ ] T047 Run E2E manual: 2 spokes (`spoke-brl`, `spoke-cop`) + 1–2 bancos cada via samples; confirmar SC-001..SC-007.
- [ ] T048 Atualizar a memória do projeto (remover/ajustar `found-pipeline-realrun-status` com o desfecho).

---

## Dependencies & Execution Order

- **Phase 1 (Setup)**: sem dependências.
- **Phase 2 (Foundational)**: bloqueia US1–US3 (primitivas nativas `RegisterPaladinNode`, Paladin RPC).
- **US1 (P1)**: depende de Foundational. É o MVP — entrega found CB-only correto.
- **US2 (P1)**: depende de Foundational (e do template commercial-bank); independe de US3.
- **US3 (P2)**: depende de US2 (precisa do nó Paladin do banco no ar para o grupo bilateral).
- **US4 (P2)**: transversal; valida idempotência/dry-run após US1–US3.
- **Polish**: depende das stories desejadas.

### Within each story

- Testes primeiro (devem falhar) → implementação → run real.
- `paladin_rpc`/`paladin_registry` (Foundational) antes dos passos que os usam.

---

## Parallel Opportunities

- T002/T003 (docs) em paralelo.
- Testes marcados [P] dentro de cada story em paralelo (arquivos distintos).
- US1 e US2 podem ser tocadas em paralelo após Foundational (US3 espera US2).

---

## Implementation Strategy

1. Phase 1 + Phase 2 (primitivas nativas).
2. **US1 (MVP)**: found CB-only → validar isolado (run real `spoke-brl`).
3. **US2**: join dinâmico de Paladin → validar 1 banco.
4. **US3**: Pente+FXA no join → validar grupo bilateral.
5. **US4 + Polish**: idempotência/dry-run, samples, E2E 2 spokes × 2 bancos.

## Notes

- Não modificar `deploy/local` nem os scripts go-test de referência (FR-015).
- Sem dev-keys hardcoded: chave-dono dos nós Paladin via `keyProvider` (FR-009).
- Sem chaves privadas em arquivo/env/log (constituição).
- Cada passo idempotente; genesis nunca regenerado.
- Cada PR carrega seção Constitution Check (workflow do repo); Samuel é reviewer dos P1.
