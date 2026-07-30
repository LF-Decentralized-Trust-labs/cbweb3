---
description: "Task list — TK-B6 (motor + found-hub + hub bundle + apply)"
---

# Tasks: Toolkit do Cenário B — Motor + found-hub + hub bundle + apply (TK-B6)

**Input**: Design documents from `/specs/037-tk-b6-orchestration/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli-and-engine.md, quickstart.md

**Tests**: INCLUÍDOS (test-first). Motor/bundle/apply com **FakeRunner** (sem Docker). Suíte **E2E**
(build tag `e2e`) funda o hub real — **skip com aviso** se forge/docker/besu ausentes.

**Organization**: tarefas agrupadas por user story (US1–US4). Como as quatro são P1, as fases são
ordenadas por **dependência**: US1 (motor) → US4 (bundle) → US3 (steps found-hub) → US2 (apply/CLI).

**Paths**: módulo `scenario-b/toolkit`. Caminhos relativos à raiz do repositório.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizável (arquivo distinto, sem dependência pendente)
- **[Story]**: US1 / US2 / US3 / US4

---

## Phase 1: Setup

- [X] T001 Criar os diretórios `scenario-b/toolkit/engine/{orchestrator,exec,bundle,apply,addrs}/` e `scenario-b/toolkit/tests/e2e/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: a fronteira injetável de efeitos externos (`CommandRunner`) que US2 (dry) e US3 (steps)
usam, e que torna o motor testável sem Docker.

**⚠️ CRITICAL**: bloqueia US3 (steps executam via runner) e o dry-run de US2.

- [X] T002 Teste em `scenario-b/toolkit/engine/exec/runner_test.go`: `FakeRunner` grava as invocações (nome+args) e retorna saídas programadas; `dryRunner` **não** executa (só registra o plano)
- [X] T003 Implementar `scenario-b/toolkit/engine/exec/runner.go`: interface `CommandRunner`, `realRunner` (`os/exec`), `FakeRunner` e `dryRunner` (conforme contracts/cli-and-engine.md)

**Checkpoint**: `go test ./engine/exec/...` verde.

---

## Phase 3: User Story 1 — Motor de steps idempotente (Priority: P1) 🎯 MVP

**Goal**: motor que executa steps com `Check`→skip / `Run`→persiste, em ordem de dependência, com
estado durável e lock.

**Independent Test**: com steps fake, verificar skip por `Check`, ordem topológica, persistência por
step, retomada após falha, e recusa de execução concorrente (lock).

### Tests for User Story 1 ⚠️

- [X] T004 [US1] Teste em `scenario-b/toolkit/engine/orchestrator/orchestrator_test.go`: steps fake com deps → ordem topológica; `Check` satisfeito → `skipped`; segunda execução idempotente; falha interrompe e persiste; retomada pula `done`; lock recusa execução concorrente; lock órfão tratado

### Implementation for User Story 1

- [X] T005 [P] [US1] `scenario-b/toolkit/engine/orchestrator/step.go`: tipo `Step{Name,Deps,Check,Run}` + tipo `Status`/`Report` (conforme contracts)
- [X] T006 [P] [US1] `scenario-b/toolkit/engine/orchestrator/state.go`: `State` persistido em `<dataDir>/.provisioning-state.yaml` (ler no início, escrever por step)
- [X] T007 [P] [US1] `scenario-b/toolkit/engine/orchestrator/lock.go`: `flock` em `<dataDir>/.provisioning.lock` + política de lock órfão (pid/idade)
- [X] T008 [US1] `scenario-b/toolkit/engine/orchestrator/orchestrator.go`: `New(...)` + `Run(ctx)` — ordena topologicamente, aplica `Check`/`Run`, respeita `dryRun` (status `planned`), persiste estado e monta o `Report`

**Checkpoint**: `go test ./engine/orchestrator/...` verde (idempotência, ordem, lock, retomada).

---

## Phase 4: User Story 4 — Emissão do hub bundle (Priority: P1)

**Goal**: emitir/carregar/validar o hub bundle (público, sem segredos). Antecipado por dependência —
o step `emit-hub-bundle` (US3) o consome.

**Independent Test**: emitir um bundle a partir de endereços+config, recarregar e validar
(round-trip); rejeitar bundle sem campos obrigatórios; garantir ausência de segredos.

### Tests for User Story 4 ⚠️

- [X] T009 [US4] Teste em `scenario-b/toolkit/engine/bundle/bundle_test.go`: `EmitHub`→`LoadHub` round-trip (endereços/chainId/rpc batem); `ValidateHub` rejeita campos faltando; `ValidateHub` rejeita material que aparente chave privada (sem segredos — SC-007)

### Implementation for User Story 4

- [X] T010 [P] [US4] `scenario-b/toolkit/engine/bundle/types.go`: tipo `HubBundle{Version,ChainID,HubRPC,HubWS,Contracts}`
- [X] T011 [US4] `scenario-b/toolkit/engine/bundle/hub.go`: `EmitHub` (yaml versionado em `<outDir>/bundles/hub.bundle.yaml`), `LoadHub`, `ValidateHub`

**Checkpoint**: `go test ./engine/bundle/...` verde.

---

## Phase 5: User Story 3 — Steps do found-hub (Priority: P1)

**Goal**: inventário de steps do `found-hub` executando via `CommandRunner`, na ordem de dependência,
com gates de prontidão e write-back de Keycloak idempotente; emite o hub bundle.

**Independent Test**: com `FakeRunner`, verificar a ordem/deps dos steps, que os gates de prontidão
são consultados antes dos dependentes, que o deploy usa `forge script CBWeb3Hub.s.sol`, que os
endereços saem do broadcast JSON, e que o write-back de secrets é idempotente.

### Tests for User Story 3 ⚠️

- [X] T012 [P] [US3] Teste em `scenario-b/toolkit/engine/addrs/addrs_test.go`: parse do broadcast JSON (`contracts/broadcast/CBWeb3Hub.s.sol/<chainId>/run-latest.json`) → mapa nome→endereço dos 7 contratos; escrita de env idempotente
- [X] T013 [US3] Teste em `scenario-b/toolkit/engine/orchestrator/step_found_hub_test.go`: com `FakeRunner`, o found-hub monta os steps na ordem (build → besu-hub → deploy-contracts → keycloak → render-env → infra/backend/frontend → relay → noc → emit-bundle); o gate de RPC é consultado antes de `deploy-contracts`; `provision-keycloak` grava secrets e é idempotente (re-run não regrava); `deploy-contracts` invoca `forge script CBWeb3Hub.s.sol:DeployCBWeb3Hub`

### Implementation for User Story 3

- [X] T014 [P] [US3] `scenario-b/toolkit/engine/addrs/broadcast.go` + `env.go`: extrair endereços do broadcast Foundry; `AppendAddr`/escrita idempotente nos `.env`
- [X] T015 [US3] `scenario-b/toolkit/engine/orchestrator/step_found_hub.go`: os steps (`build-contracts`, `start-besu-hub`, `deploy-hub-contracts`, `provision-keycloak-hub` + write-back, `render-hub-env`, `start-hub-infra/backend/frontend`, `start-relay`, `start-noc`, `emit-hub-bundle`) via `CommandRunner`, cada um com `Check` (idempotência/gate)
- [X] T016 [US3] `scenario-b/toolkit/engine/orchestrator/deps.go`: `FoundHubSteps(cfg) []Step` — o conjunto ordenado com as dependências declaradas

**Checkpoint**: `go test ./engine/orchestrator/... ./engine/addrs/...` verde (found-hub com FakeRunner).

---

## Phase 6: User Story 2 — Comando `apply` com dry-run e report (Priority: P1)

**Goal**: `apply -f <manifest> [--dry-run] [-o json|yaml]` despacha por modo (`found-hub`), sempre
emite report; dry-run não aciona efeitos.

**Independent Test**: `apply --dry-run` produz report `planned` sem acionar o runner real; `-o` muda
formato; modo não suportado/manifesto inválido → erro antes de efeitos; sinal → report parcial.

### Tests for User Story 2 ⚠️

- [X] T017 [US2] Teste em `scenario-b/toolkit/engine/apply/apply_test.go`: dispatch `found-hub`; `--dry-run` ⇒ `dryRunner` (runner real **não** acionado); report em json e yaml; modo `found-spoke`/`join` → erro "not supported yet"; manifesto inválido → erro antes de efeitos
- [X] T018 [US2] Teste em `scenario-b/toolkit/cmd/cbweb3b/apply_test.go`: `apply -f ... --dry-run -o json` sai 0 com report; manifesto inválido sai ≠ 0; (sinal → report parcial, se testável)

### Implementation for User Story 2

- [X] T019 [US2] `scenario-b/toolkit/engine/apply/apply.go` + `report.go`: `Apply(ctx, Options)` — resolve modo, monta os steps (found-hub via `deps.FoundHubSteps`), roda o motor (dry/real), retorna `Report`; render json/yaml
- [X] T020 [US2] Estender `scenario-b/toolkit/cmd/cbweb3b/main.go`: subcomando `apply` chama `engine/apply` (não mais só validar); flags `--data-dir`/`--out-dir`; `signal.NotifyContext` → report parcial; exit codes (0/1/2). **Reconciliar `scenario-b/toolkit/cmd/cbweb3b/main_test.go` (TK-B1)** para a nova semântica do `apply` — o teste antigo esperava `exitInvalid` para `apply` sem `--dry-run` (agora executa o motor); atualizar sem quebrar o subcomando `validate` (resolução I1)

**Checkpoint**: `go test ./engine/apply/... ./cmd/cbweb3b/...` verde.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T021 Suíte E2E em `scenario-b/toolkit/tests/e2e/found_hub_e2e_test.go` (build tag `e2e`): pré-check de `forge`/`docker`/imagem besu → `t.Skip` com aviso se ausente; caso presente, roda `apply found-hub` real e valida os endereços do hub bundle contra o RPC (código on-chain ≠ vazio) — SC-009/FR-015
- [X] T022 [P] `gofmt`/`go vet ./...` e suíte de unidade `cd scenario-b/toolkit && go test ./...` (motor/bundle/apply/exec/addrs); validar os passos do `quickstart.md`
- [X] T023 [P] Atualizar nota de status TK-B6 em `scenario-b/docs/design/scenario-b-toolkit-roadmap.md` (§15) e a tabela do toolkit em `scenario-b/README.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sem dependências.
- **Foundational (Phase 2, exec)**: depois do Setup; bloqueia US3 e o dry-run de US2.
- **US1 (Phase 3, motor)**: depois do Setup; independente de exec (testado com steps fake).
- **US4 (Phase 4, bundle)**: depois do Setup; independente. Antecipado porque o step `emit-hub-bundle`
  (US3) o consome.
- **US3 (Phase 5, steps found-hub)**: depende de Foundational (exec), US1 (motor) e US4 (bundle) +
  `engine/addrs`.
- **US2 (Phase 6, apply/CLI)**: depende de US1 (motor) e US3 (conjunto de steps found-hub); amarra tudo.
- **Polish (Phase 7)**: depois das user stories; a E2E depende do found-hub completo.

### Within Each User Story

- Teste escrito e **falhando** antes da implementação (test-first).
- Efeitos externos sempre via `CommandRunner`; dry-run nunca aciona o runner real.

### Parallel Opportunities

- US1: T005/T006/T007 [P] (arquivos distintos) antes de T008 (orchestrator, que os usa).
- US4: T010 [P] antes de T011.
- US3: T012/T014 (addrs) [P] em relação ao início dos steps.
- Polish: T022/T023 [P].

---

## Implementation Strategy

### MVP First (US1 + dry-run de US2)

1. Setup → Foundational (exec).
2. US1 (motor) → `go test ./engine/orchestrator/...` verde.
3. **STOP & VALIDATE**: motor idempotente + lock + retomada — fundação do toolkit executável.

### Incremental Delivery

1. Setup + Foundational (exec).
2. US1 (motor) → MVP do motor.
3. US4 (hub bundle).
4. US3 (steps found-hub) → found-hub montável e testável com FakeRunner.
5. US2 (apply/CLI) → `apply found-hub` utilizável (dry-run + real).
6. Polish (E2E real, fmt/vet, roadmap/README).

---

## Notes

- [P] = arquivos distintos, sem dependência pendente.
- **Manter** o Keycloak no found-hub (decisão 2026-07-10): governança + NOC + gate IV.
- Deploy dos contratos = **um** `forge script CBWeb3Hub.s.sol:DeployCBWeb3Hub`; a ordem é interna ao
  Solidity; endereços do broadcast JSON.
- `--dry-run` nunca aciona efeitos; report distingue done/skipped/failed/planned.
- Sem novas dependências Go; não importar `scenario-a/`; não alterar Makefiles/`deploy/local`.
- E2E: skip-com-aviso quando forge/docker/besu ausentes (nunca falso verde).

---

## Phase 8: Convergence

> Origem: achado F1 do `/speckit-converge` (2026-07-10). O `found-hub` sobe o nó do hub via o
> template `hub` (Besu com `--genesis-file=genesis/genesis.json`), mas nenhum step gera/semeia o
> `genesis.json` no volume `hub_genesis` — logo o `found-hub` real não sobe o nó e o SC-009 (fundar
> o hub E2E) é inalcançável. Trabalho remanescente para tornar a fundação E2E executável.

- [X] T024 [US3] Adicionar um step `gen-genesis-hub` (QBFT, chain id do manifesto, alloc de chaves do operador/deployer) **antes** de `start-besu-hub` em `scenario-b/toolkit/engine/orchestrator/step_found_hub.go`, semeando `genesis.json` no volume/dir `hub_genesis` (via `engine/dockervolume` ou dir de estado) de forma idempotente (`Check` = genesis já presente) per SC-009/FR-007 (missing)
- [X] T025 [US3] Teste em `scenario-b/toolkit/engine/orchestrator/step_found_hub_test.go` (ou pacote de genesis) para o step de genesis: gera/semeia genesis com o chainId esperado e é idempotente (re-run pula); com `FakeRunner`/tmp, sem Besu real per SC-009 (missing)
