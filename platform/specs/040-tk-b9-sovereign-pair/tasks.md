---
description: "Task list — TK-B9 (par soberano + liquidez cooperativa + seed-oracle)"
---

# Tasks: Toolkit do Cenário B — par soberano + liquidez cooperativa + seed-oracle (TK-B9)

**Input**: Design documents from `/specs/040-tk-b9-sovereign-pair/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/sovereign-pair.md, quickstart.md

**Tests**: INCLUÍDOS (test-first). Steps/leituras com **FakeRunner** + seams injetáveis (sem
Foundry/Besu). Suíte **E2E** (build tag `e2e`) abre um par entre dois spokes fundados — **skip com
aviso** se o ambiente/hub/relay ausente.

**Organization**: por user story (US1–US3). Estende o `found-spoke` (TK-B7) com uma **cauda soberana
soft** disparada por `spec.pair`. Reusa motor/exec/addrs do TK-B6/B7 e o Step soft.

**Paths**: módulo `scenario-b/toolkit`. Caminhos relativos à raiz do repositório.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizável (arquivo distinto, sem dependência pendente)
- **[Story]**: US1 / US2 / US3

---

## Phase 1: Setup

- [X] T001 Garantir que `scenario-b/toolkit/engine/manifest/testdata/found-spoke.yaml` tem o bloco `spec.pair` completo (`proposerCB`, `confirmerCB`, `symbolA`, `symbolB`) para o dispatch/dry-run da cauda soberana; estender se faltar `confirmerCB`/`symbolB`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: leitura on-chain de idempotência + extensão de config, usadas pelas 3 user stories.

**⚠️ CRITICAL**: `pairstate` (US1) e `SpokeConfig.Pair`/`FoundSpokeSteps` (todas as US) bloqueiam as US.

- [X] T002 [P] Teste em `scenario-b/toolkit/engine/orchestrator/pairstate_test.go`: `pairID("BRL","ARS")` == `"W-BRL-ARS"`; `pairStatus` (FakeRunner) parseia `PROPOSED`/`ACTIVE` do stdout de `cast call getPair(string)` e retorna `exists=false` em revert/vazio
- [X] T003 Implementar `scenario-b/toolkit/engine/orchestrator/pairstate.go`: `pairID(symbolA, symbolB) string` e `pairStatus(ctx, runner, rpc, pairRegistry, pairId) (status string, exists bool, err error)` via `cast call ... "getPair(string)"`
- [X] T004 Estender `SpokeConfig` em `scenario-b/toolkit/engine/orchestrator/step_found_spoke.go` (+ `Pair *PairConfig`, `Environment`, `HubPairRegistry`, `HubManualOracle`, `HubIdentityRegistry`, `RelayerAddr`, `HubAdminKey`, `CBHubKey`) + o tipo `PairConfig` (ProposerCB, ConfirmerCB, SymbolA, SymbolB, Rate, CurrentCB); `WithDefaults()` inalterado para os campos existentes
- [X] T005 Teste em `scenario-b/toolkit/engine/orchestrator/step_sovereign_pair_test.go`: `FoundSpokeSteps` **anexa** os 3 steps soberanos (Soft) quando `Pair != nil` (Deps em `add-noc-agent`; `emit-spoke-bundle` depende deles) e **não** os anexa quando `Pair == nil`; topoSort sem ciclo
- [X] T006 Implementar em `scenario-b/toolkit/engine/orchestrator/step_found_spoke.go` a fiação condicional: quando `c.Pair != nil`, anexar `sovereignPairSteps(c)` e re-apontar as Deps de `emit-spoke-bundle`

**Checkpoint**: `go test ./engine/orchestrator/...` verde (pairstate, fiação condicional).

---

## Phase 3: User Story 1 — open-sovereign-pair (Priority: P1) 🎯 MVP

**Goal**: abrir o corredor bilateral — proponente faz scaffolding + `proposePair`; confirmador faz
`confirmPair`; idempotência por `getPair` status; soberania estrita (nenhum run usa a chave da
contraparte).

**Independent Test**: proponente (CB-A) deploya/dedup W-tokens/AMM/LCR + `setCentralBankOf`/grant +
`proposePair`; confirmador (CB-B) faz `confirmPair` só se `PROPOSED`; `ACTIVE`→skip; pré-condição
ausente→pending (soft).

### Tests for User Story 1 ⚠️

- [X] T007 [US1] Teste em `scenario-b/toolkit/engine/orchestrator/step_sovereign_pair_test.go`: com `CurrentCB == ProposerCB` (FakeRunner), `open-sovereign-pair` invoca `forge create` dos W-tokens (dedup por moeda — não redeploya símbolo já presente), da AMM e do LCR, `cast setCentralBankOf`/`grantRole` e `cast ... proposePair`; **não** invoca `confirmPair`
- [X] T008 [US1] Teste em `scenario-b/toolkit/engine/orchestrator/step_sovereign_pair_test.go`: com `CurrentCB == ConfirmerCB` e `pairStatus == PROPOSED`, `open-sovereign-pair` invoca `cast ... confirmPair`; com `ACTIVE` o `Check` pula; com o par inexistente (sou confirmador) o step fica `pending` (erro soft, não fatal)

### Implementation for User Story 1

- [X] T009 [US1] Implementar `sovereignPairSteps(cfg SpokeConfig) []Step` (parte `open-sovereign-pair`) em `scenario-b/toolkit/engine/orchestrator/step_sovereign_pair.go`: `Check` via `pairStatus` (ACTIVE→skip; PROPOSED+proponente→skip); `Run` ramifica por papel (proponente: scaffolding com `HubAdminKey` + `proposePair` com `CBHubKey`; confirmador: `confirmPair` com `CBHubKey` só se PROPOSED, senão pending); `Soft: true`

**Checkpoint**: par abre (PROPOSED por CB-A; ACTIVE por CB-B) com FakeRunner; idempotente.

---

## Phase 4: User Story 2 — commit-liquidity (Priority: P1)

**Goal**: cada CB contribui liquidez só da própria moeda via `registerCommit`; o relay casa
`CommitMatched`.

**Independent Test**: `commit-liquidity` invoca `registerCommit` só do lado da moeda do CB corrente;
re-run não duplica (Check por commit pendente).

### Tests for User Story 2 ⚠️

- [X] T010 [US2] Teste em `scenario-b/toolkit/engine/orchestrator/step_sovereign_pair_test.go`: `commit-liquidity` (FakeRunner) invoca `cast ... registerCommit(poolPair, side, amount, wToken)` **só** do lado da moeda do CB corrente; `Check` pula quando já há commit pendente do lado do CB; step `Soft`

### Implementation for User Story 2

- [X] T011 [US2] Implementar a parte `commit-liquidity` em `scenario-b/toolkit/engine/orchestrator/step_sovereign_pair.go`: `Run` = `registerCommit` do lado do CB corrente; `Check` via `getPendingCommit(poolPair, side)`; `Deps: ["open-sovereign-pair"]`, `Soft: true`

**Checkpoint**: commit do lado do CB registrado; idempotente; casamento delegado ao relay.

---

## Phase 5: User Story 3 — seed-oracle (Priority: P2)

**Goal**: semear a taxa inicial do par no `ManualOracle` (`setRate`), apenas em `local`.

**Independent Test**: `seed-oracle` invoca `setRate` em `local`; **pulado** fora de `local`; idempotente.

### Tests for User Story 3 ⚠️

- [X] T012 [US3] Teste em `scenario-b/toolkit/engine/orchestrator/step_sovereign_pair_test.go`: com `Environment == "local"`, `seed-oracle` invoca `cast ... setRate(tokenA, tokenB, rate)`; com `Environment != "local"` o `Check` pula; step `Soft`

### Implementation for User Story 3

- [X] T013 [US3] Implementar a parte `seed-oracle` em `scenario-b/toolkit/engine/orchestrator/step_sovereign_pair.go`: `Check` = skip se não-`local`; `Run` = `setRate` no `HubManualOracle`; `Deps: ["open-sovereign-pair"]`, `Soft: true`

**Checkpoint**: oráculo semeado em local; pulado fora de local.

---

## Phase 6: Integration — apply wiring + CLI

- [X] T014 Teste em `scenario-b/toolkit/engine/apply/apply_test.go`: `apply found-spoke` com `spec.pair` (dry-run) **planeja** os 3 steps soberanos (planned; sem efeitos); sem `spec.pair` os 3 **não** aparecem
- [X] T015 Implementar em `scenario-b/toolkit/engine/apply/apply.go` (`applyFoundSpoke`): quando `pd.Spec.Pair != nil`, popular `SpokeConfig.Pair` (proposerCB/confirmerCB/symbolA/symbolB/rate + `CurrentCB` do spoke), `Environment` (do manifesto), `HubPairRegistry`/`HubManualOracle`/`HubIdentityRegistry` (do hub bundle), `RelayerAddr`, e as chaves `HubAdminKey`/`CBHubKey` (KeyProvider/flags; ausentes fora de local → atos pending)
- [X] T016 [P] Adicionar flags em `scenario-b/toolkit/cmd/cbweb3b/main.go` se necessário (`--relayer-addr`, `--pair-rate`) e passá-las em `apply.Options`; caso os valores venham só do manifesto/bundle, registrar que nenhuma flag nova é necessária

**Checkpoint**: `apply found-spoke --dry-run` com par planeja a cauda; `go test ./engine/apply/... ./cmd/cbweb3b/...` verde.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T017 Suíte E2E em `scenario-b/toolkit/tests/e2e/sovereign_pair_e2e_test.go` (build tag `e2e`): requer Docker/Foundry + hub fundado + relay (`CBWEB3B_E2E_PAIR_CBA_MANIFEST`, `CBWEB3B_E2E_PAIR_CBB_MANIFEST`, `CBWEB3B_E2E_REPO_ROOT`, `CBWEB3B_E2E_HUB_RPC`); ausente ⇒ `t.Skip` com aviso; caso presente, CB-A propõe → CB-B confirma (par `ACTIVE`), ambos commitam (relay casa), oráculo semeado (SC-008)
- [X] T018 [P] `gofmt`/`go vet ./...` e suíte `cd scenario-b/toolkit && go test ./...` (+ `go build -tags e2e ./tests/e2e/...`); validar os passos do `quickstart.md`
- [X] T019 [P] Atualizar a nota de status TK-B9 em `scenario-b/docs/design/scenario-b-toolkit-roadmap.md` (§15) e a tabela do toolkit em `scenario-b/README.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: fixture com `spec.pair`.
- **Foundational (Phase 2)**: depois do Setup; `pairstate` + `SpokeConfig.Pair`/fiação bloqueiam US1–US3.
- **US1 (Phase 3)**: depois da Foundational; base para US2/US3 (Deps `open-sovereign-pair`).
- **US2 (Phase 4)**: depois de US1.
- **US3 (Phase 5)**: depois de US1.
- **Integration (Phase 6)**: depois de US1–US3 (apply popula a config).
- **Polish (Phase 7)**: por último; E2E depende da cauda completa + hub + relay.

### Within Each User Story / Notes

- Teste escrito e **falhando** antes da implementação.
- US1–US3 vivem no mesmo `step_sovereign_pair.go` ⇒ sequenciais nele; `pairstate.go` é arquivo distinto
  → `[P]` na Foundational.
- **Deviation (plan/Complexity Tracking)**: **não** reusar `SeedNewSovereignPair.s.sol` (exige ambas as
  chaves) — atos discretos via `cast`/`forge create`, honrando a soberania estrita (Q2) e "não alterar
  contratos".

### Parallel Opportunities

- Foundational: T002/T003 (`pairstate.go`) [P] em relação ao `step_found_spoke.go`/`step_sovereign_pair.go`.
- Integration: T016 [P]. Polish: T018/T019 [P].

---

## Implementation Strategy

### MVP First (US1)

1. Setup + Foundational (pairstate + config + fiação condicional).
2. US1 (open-sovereign-pair) → o corredor abre (PROPOSED/ACTIVE) com soberania estrita.

### Incremental Delivery

1. Foundational → US1 → US2 → US3 → Integração (apply/CLI) → Polish (E2E, docs).
2. Cada US validável isolada com FakeRunner; a Integração amarra a cauda ao `found-spoke`.

---

## Notes

- **Cauda soft do found-spoke** disparada por `spec.pair`; sem par → não anexa; falha soft não bloqueia.
- **Soberania estrita**: cada run só o ato do CB corrente; CB-A propõe, CB-B confirma (run separado);
  nenhum run detém a chave da contraparte.
- **Idempotência on-chain**: `getPair(pairId).status` (ACTIVE pula; PROPOSED aguarda).
- **W-token dedup por moeda**; breaker opção A (sem mudança de contrato); `seed-oracle` local-only.
- Sem novas dependências Go; não importar `scenario-a/`; não alterar contratos/scripts/Makefiles/`deploy/local`.
- E2E: skip-com-aviso quando hub/relay/foundry/docker ausentes (nunca falso verde).

---

## Phase 8: Convergence

- [X] T020 Mapear **ambos** os W-tokens via `setCentralBankOf` no `scaffoldPair` (`scenario-b/toolkit/engine/orchestrator/step_sovereign_pair.go`): hoje só `setCentralBankOf(tokenA, c.CBAddress)` roda; adicionar o mapeamento de `tokenB → endereço do CB confirmador`. Introduzir `PairConfig.ConfirmerCBAddress` (e, se necessário, `ProposerCBAddress`) preenchido em `applyFoundSpoke` (flag/manifesto) e mapear `setCentralBankOf(tokenA, proposerCBAddr)` + `setCentralBankOf(tokenB, confirmerCBAddr)` — ato de admin (hub admin key), sem usar a chave da contraparte (soberania preservada). Sem isso, a auth do `LiquidityCommitRegistry` recusa o `registerCommit` do lado do CB-B (quebra `commit-liquidity`) per FR-003/US2 (partial)
