---
description: "Task list — TK-B7 (found-spoke + spoke bundle)"
---

# Tasks: Toolkit do Cenário B — found-spoke + spoke bundle (TK-B7)

**Input**: Design documents from `/specs/038-tk-b7-found/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/found-spoke.md, quickstart.md

**Tests**: INCLUÍDOS (test-first). Steps/bundle com **FakeRunner** + seams injetáveis (sem
Docker/Foundry/Besu). Suíte **E2E** (build tag `e2e`) funda um spoke contra um hub fundado — **skip
com aviso** se o ambiente/hub bundle ausente.

**Organization**: por user story (US1–US5). Reusa o motor/exec/addrs/bundle/apply do TK-B6 e as
interfaces KeyProvider/CertSource/RelayRegistrar. Fases ordenadas por dependência.

**Paths**: módulo `scenario-b/toolkit`. Caminhos relativos à raiz do repositório.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizável (arquivo distinto, sem dependência pendente)
- **[Story]**: US1 / US2 / US3 / US4 / US5

---

## Phase 1: Setup

- [X] T001 Garantir que `scenario-b/toolkit/engine/manifest/testdata/found-spoke.yaml` tem `spec.hubBundleRef`, `spec.spoke.chainId` e `spec.node` suficientes para o dispatch/dry-run do `found-spoke` (estender o fixture se faltar campo)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: extensões compartilhadas do motor usadas por várias user stories.

**⚠️ CRITICAL**: soft steps (US4), gen-genesis (US2) e enode (US2/US5) bloqueiam as respectivas US.

- [X] T002 Teste em `scenario-b/toolkit/engine/orchestrator/orchestrator_test.go`: um `Step{Soft:true}` que falha vira `soft-failed` no report e **não** interrompe a execução; steps não-soft mantêm o comportamento (interrompe)
- [X] T003 Estender `scenario-b/toolkit/engine/orchestrator/step.go` (+ `Soft bool`, `StatusSoftFailed`) e `orchestrator.go` (tratamento soft-fail no `Run`)
- [X] T004 [P] Generalizar `scenario-b/toolkit/engine/orchestrator/step_genesis.go`: `genGenesisStep(name, chainID, genesisDir, validators, runner, image)` reutilizável por hub e spoke (preservar o hub e seus testes)
- [X] T005 [P] Teste + impl `scenario-b/toolkit/engine/orchestrator/enode.go`: `EnodeReader` + `adminNodeInfoEnode(ctx, rpcURL)` (JSON-RPC `admin_nodeInfo`); teste com servidor HTTP mock

**Checkpoint**: `go test ./engine/orchestrator/...` verde (soft, genesis generalizado, enode).

---

## Phase 3: User Story 1 — Consumir hub bundle + register-cb (Priority: P1) 🎯 MVP

**Goal**: consumir/validar o hub bundle e auto-registrar o CB no `IdentityRegistry` do hub.

**Independent Test**: hub bundle inválido → erro antes de efeitos; `register-cb` invoca
`RegisterParticipants.s.sol` contra o RPC do hub; idempotente (CB já participante → pula).

### Tests for User Story 1 ⚠️

- [X] T006 [US1] Teste em `scenario-b/toolkit/engine/orchestrator/step_found_spoke_test.go`: `consume-hub-bundle` falha em bundle inválido/ausente; `register-cb` (FakeRunner) invoca `forge script RegisterParticipants.s.sol:RegisterParticipants` **e** `grantLiquidityProvider(CB)` (cast) contra o RPC do hub; `Check` idempotente por `isParticipant`/`isLiquidityProvider`; o grant sem permissão (fora de local) **não** falha o step (registra pendência)

### Implementation for User Story 1

- [X] T007 [US1] Implementar em `scenario-b/toolkit/engine/orchestrator/step_found_spoke.go` os steps `consume-hub-bundle` (via `bundle.LoadHub`) e `register-cb` — **duas ações idempotentes**: `registerParticipant(CENTRAL_BANK)` (`RegisterParticipants.s.sol`, Check `isParticipant`) + `grantLiquidityProvider(CB)` no hub, **tentado automaticamente** (Check `isLiquidityProvider`; falha de permissão fora de local vira pendência não-fatal) — + o tipo `SpokeConfig` (resolução A1)

**Checkpoint**: consume + register-cb testáveis isolados.

---

## Phase 4: User Story 2 — Genesis do spoke + nó + contratos do spoke (Priority: P1)

**Goal**: gerar o genesis do spoke, subir o nó (CB validador, capturar enode) e deployar os contratos
do spoke.

**Independent Test**: genesis do spoke gerado (chain própria); `start-besu-spoke` captura o enode;
`deploy-spoke-contracts` invoca `CBWeb3Spoke.s.sol`; ordem respeitada.

### Tests for User Story 2 ⚠️

- [X] T008 [US2] Teste em `scenario-b/toolkit/engine/orchestrator/step_found_spoke_test.go`: `gen-genesis-spoke` usa o chainId do spoke; `start-besu-spoke` chama o `EnodeReader` (fake) e guarda o enode; `deploy-spoke-contracts` (FakeRunner) invoca `forge script CBWeb3Spoke.s.sol:DeployCBWeb3Spoke` com o RPC do spoke

### Implementation for User Story 2

- [X] T009 [US2] Implementar em `scenario-b/toolkit/engine/orchestrator/step_found_spoke.go` os steps `gen-genesis-spoke` (via `genGenesisStep`), `start-besu-spoke` (sobe o nó + captura enode), `deploy-spoke-contracts` (forge vs spoke, `Check` = broadcast presente)

**Checkpoint**: rede do spoke + contratos montáveis com FakeRunner.

---

## Phase 5: User Story 3 — Wire hub + Keycloak + serviços (Priority: P1)

**Goal**: conectar endereços do hub, provisionar Keycloak (write-back) e subir infra/backend/frontend.

**Independent Test**: `wire-hub-addresses` idempotente; `provision-keycloak-spoke` write-back
idempotente; serviços montam compose.

### Tests for User Story 3 ⚠️

- [X] T010 [US3] Teste em `scenario-b/toolkit/engine/orchestrator/step_found_spoke_test.go`: `wire-hub-addresses` escreve endereços do hub no env (idempotente); `provision-keycloak-spoke` grava secrets (write-back) e `Check` idempotente

### Implementation for User Story 3

- [X] T011 [US3] Implementar em `scenario-b/toolkit/engine/orchestrator/step_found_spoke.go` os steps `wire-hub-addresses`, `provision-keycloak-spoke` (+ write-back), `render-spoke-env`, `start-spoke-infra`/`backend`/`frontend`

**Checkpoint**: serviços do spoke montáveis; env conectado ao hub.

---

## Phase 6: User Story 4 — register-relay-spoke + add-noc-agent (soft) (Priority: P2)

**Goal**: registrar o spoke no relay (runtime) e subir o noc-agent (soft).

**Independent Test**: `register-relay-spoke` via `RelayRegistrar` (local) idempotente;
`add-noc-agent` Soft — falha → `soft-failed`, não bloqueia.

### Tests for User Story 4 ⚠️

- [X] T012 [US4] Teste em `scenario-b/toolkit/engine/orchestrator/step_found_spoke_test.go`: `register-relay-spoke` chama `Registrar.Register` (local in-memory) com id/WS/gateway e é idempotente; `add-noc-agent` é `Soft` e uma falha vira `soft-failed` sem interromper (via motor)

### Implementation for User Story 4

- [X] T013 [US4] Implementar em `scenario-b/toolkit/engine/orchestrator/step_found_spoke.go` os steps `register-relay-spoke` (usa `c.Registrar`, TK-B5) e `add-noc-agent` (`Soft:true`)

**Checkpoint**: spoke registrado no relay; noc-agent não-bloqueante.

---

## Phase 7: User Story 5 — Spoke bundle (Priority: P1)

**Goal**: emitir/carregar/validar o spoke bundle (genesis + enode + endereços, sem segredos).

**Independent Test**: `EmitSpoke`→`LoadSpoke` round-trip; `ValidateSpoke` rejeita campos faltando e
material de chave privada; o step `emit-spoke-bundle` monta o bundle do estado final.

### Tests for User Story 5 ⚠️

- [X] T014 [P] [US5] Teste em `scenario-b/toolkit/engine/bundle/spoke_test.go`: `EmitSpoke`→`LoadSpoke` (spokeId/chainId/enode/genesis/contracts batem); `ValidateSpoke` rejeita campos faltando e rejeita `PRIVATE KEY` (SC-008)
- [X] T015 [US5] Teste em `scenario-b/toolkit/engine/orchestrator/step_found_spoke_test.go`: `emit-spoke-bundle` lê genesis + enode + broadcast e emite `bundles/spoke-<id>.bundle.yaml` válido

### Implementation for User Story 5

- [X] T016 [P] [US5] Implementar `scenario-b/toolkit/engine/bundle/types.go` (`SpokeBundle`) + `scenario-b/toolkit/engine/bundle/spoke.go` (`EmitSpoke`/`LoadSpoke`/`ValidateSpoke`)
- [X] T017 [US5] Implementar o step `emit-spoke-bundle` em `scenario-b/toolkit/engine/orchestrator/step_found_spoke.go`

**Checkpoint**: spoke bundle round-trip; sem segredos.

---

## Phase 8: Integration — FoundSpokeSteps + apply dispatch + CLI

- [X] T018 Teste em `scenario-b/toolkit/engine/orchestrator/step_found_spoke_test.go`: `FoundSpokeSteps(cfg)` monta o conjunto na ordem/deps (consume → register-cb; gen-genesis → besu; deploy+besu → emit); topoSort sem ciclo
- [X] T019 Implementar `FoundSpokeSteps(cfg SpokeConfig) []Step` (ordem/deps) em `scenario-b/toolkit/engine/orchestrator/step_found_spoke.go`
- [X] T020 Teste em `scenario-b/toolkit/engine/apply/apply_test.go`: dispatch `found-spoke` (dry-run → steps planned; sem efeitos); `join` → "not supported yet"; hub bundle ausente → erro
- [X] T021 Implementar `applyFoundSpoke` em `scenario-b/toolkit/engine/apply/apply.go` (monta `SpokeConfig` do manifesto + hub bundle; runner dry/real; RelayRegistrar via `--relay`) e estender `scenario-b/toolkit/cmd/cbweb3b/main.go` (flags `--spoke-rpc`, `--gateway-url`, `--relay`)

**Checkpoint**: `apply found-spoke --dry-run` planeja o conjunto; `go test ./engine/apply/... ./cmd/cbweb3b/...` verde.

---

## Phase 9: Polish & Cross-Cutting Concerns

- [X] T022 Suíte E2E em `scenario-b/toolkit/tests/e2e/found_spoke_e2e_test.go` (build tag `e2e`): requer Docker/forge/besu + um hub bundle (`CBWEB3B_E2E_HUB_BUNDLE`, `..._SPOKE_RPC`); ausente ⇒ `t.Skip` com aviso; caso presente, funda o spoke e valida o spoke bundle (SC-009)
- [X] T023 [P] `gofmt`/`go vet ./...` e suíte `cd scenario-b/toolkit && go test ./...`; validar os passos do `quickstart.md`
- [X] T024 [P] Atualizar nota de status TK-B7 em `scenario-b/docs/design/scenario-b-toolkit-roadmap.md` (§15) e a tabela do toolkit em `scenario-b/README.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sem dependências.
- **Foundational (Phase 2)**: depois do Setup; soft/genesis/enode bloqueiam US2/US4/US5.
- **US1 (Phase 3)**: depois da Foundational. `consume-hub-bundle` é base para US1/US3.
- **US2 (Phase 4)**: depois da Foundational (genesis/enode).
- **US3 (Phase 5)**: depois de US1 (consume p/ wire) e US2 (deploy p/ render).
- **US4 (Phase 6)**: depois de US2 (start-besu) + Foundational (soft); `RelayRegistrar` já existe (TK-B5).
- **US5 (Phase 7)**: depois de US2 (genesis/enode/deploy).
- **Integration (Phase 8)**: depois de US1–US5 (monta o conjunto + dispatch).
- **Polish (Phase 9)**: por último; E2E depende do found-spoke completo + hub.

### Within Each User Story / Notes

- Teste escrito e **falhando** antes da implementação.
- Os construtores de step ficam em `step_found_spoke.go` (mesmo arquivo) ⇒ US1–US5 são sequenciais
  nele; `bundle/spoke.go`, `enode.go`, `step_genesis.go` são arquivos distintos → `[P]`.

### Parallel Opportunities

- Foundational: T004 (genesis) e T005 (enode) [P].
- US5: T014/T016 (`bundle/spoke.go`) [P] em relação ao `step_found_spoke.go`.
- Polish: T023/T024 [P].

---

## Implementation Strategy

### MVP First (US1 + US2)

1. Setup + Foundational (soft/genesis/enode).
2. US1 (consume + register-cb) e US2 (genesis + besu + contratos) → o CB registra no hub e a rede do
   spoke existe.

### Incremental Delivery

1. Foundational → US1 → US2 → US3 → US4 → US5 → Integração (apply/CLI) → Polish (E2E, docs).
2. Cada US validável isolada com FakeRunner; a Integração amarra o `apply found-spoke`.

---

## Notes

- **Fora de escopo (TK-B9)**: par soberano / liquidez / seed-oracle.
- Spoke bundle inclui **genesis + enode** (o join precisa); hub bundle era RPC-only.
- `add-noc-agent` é **soft** (não bloqueia); auth-por-CB no relay fora de escopo (§14.D).
- Deploy do spoke / register-cb = `forge script` (`CBWeb3Spoke.s.sol` / `RegisterParticipants.s.sol`).
- Sem novas dependências Go; não importar `scenario-a/`; não alterar Makefiles/`deploy/local`.
- E2E: skip-com-aviso quando hub/forge/docker/besu ausentes (nunca falso verde).

---

## Phase 10: Convergence

- [X] T025 Wire a `CBRegistered` probe em `applyFoundSpoke` (`scenario-b/toolkit/engine/apply/apply.go`): consultar `isParticipant`/`isLiquidityProvider` no `identityRegistry` do hub (via `HubRPC`) e passá-lo em `SpokeConfig.CBRegistered`, de modo que `register-cb` seja pulado em re-`apply` pela Check do toolkit (não só pela idempotência do contrato) per FR-002/SC-002 (partial)
- [X] T026 Adicionar a flag `--spoke-ws` em `scenario-b/toolkit/cmd/cbweb3b/main.go` e roteá-la para `SpokeConfig.SpokeWS` em `applyFoundSpoke` (`scenario-b/toolkit/engine/apply/apply.go`), substituindo o reuso de `o.HubWS`, para que `register-relay-spoke` e o spoke bundle carreguem o WS real do spoke per FR-008 (partial)
