---
description: "Task list — TK-B10 (E2E completo + baseline de performance)"
---

# Tasks: Toolkit do Cenário B — E2E completo + baseline (TK-B10)

**Input**: Design documents from `/specs/041-tk-b10-e2e-baseline/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/e2e-baseline.md, quickstart.md

**Tests**: esta é a **fase de verificação** (testes + doc); os "testes" são o próprio deliverable
(E2E de pipeline + baseline). Tudo sob build tags (`e2e`/`perf`) com **skip-com-aviso**; nunca falso
verde. Reusa `apply.Apply`, os E2E por modo (TK-B7..B9) e os helpers `requireTool`/`requireEnv`/`hasCode`.

**Organization**: por user story (US1–US4). Sem código de produção novo — só tests + doc.

**Paths**: módulo `scenario-b/toolkit`. Caminhos relativos à raiz do repositório.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizável (arquivo distinto, sem dependência pendente)
- **[Story]**: US1 / US2 / US3 / US4

---

## Phase 1: Setup

- [X] T001 Estender os helpers do pacote e2e em `scenario-b/toolkit/tests/e2e/rpc_e2e.go` (build tag `e2e`): `castCall(t, rpcURL, addr, sig, args...) string` e `castSend(t, rpcURL, key, addr, sig, args...)` (via `os/exec` `cast`), reusados pelas asserções de swap/breaker/bridge — sem duplicar o que já existe (`hasCode`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: o esqueleto do E2E de pipeline (skip-com-aviso + provisionamento) que as US de negócio usam.

**⚠️ CRITICAL**: o provisionamento (US1) é pré-requisito das asserções de negócio (US2) e idempotência (US3).

- [X] T002 Criar `scenario-b/toolkit/tests/e2e/pipeline_e2e_test.go` (build tag `e2e`) com o esqueleto de `TestPipelineEndToEnd`: `requireTool(t,"docker")`/`requireTool(t,"cast")` + `requireEnv` dos manifestos/endereços/chaves (`CBWEB3B_E2E_*`); `t.Skip` com aviso quando ausente; helper interno para chamar `apply.Apply` e falhar com passo + report parcial em erro

---

## Phase 3: User Story 1 — Pipeline completo (Priority: P1) 🎯 MVP

**Goal**: provisionar hub + 2 spokes soberanos + banco + par `ACTIVE` via `apply.Apply`, contra Docker.

**Independent Test**: `TestPipelineEndToEnd` roda found-hub → found-spoke (CB-A) → found-spoke (CB-B)
→ join, confirma que sobem e que o par fica `ACTIVE` e o banco sincroniza; ambiente ausente ⇒ skip.

### Tests / Implementation for User Story 1

- [X] T003 [US1] Implementar em `scenario-b/toolkit/tests/e2e/pipeline_e2e_test.go` a fase de provisionamento: `apply.Apply` sequencial (found-hub → found-spoke CB-A → found-spoke CB-B → join) usando os manifestos das env; asserção de sucesso (sem erro; report done/skipped)
- [X] T004 [US1] Adicionar em `scenario-b/toolkit/tests/e2e/pipeline_e2e_test.go` a asserção de par `ACTIVE`: `castCall` de `getPair(string)` no `PAIR_REGISTRY` → status `ACTIVE`; (opcional) `hasCode` dos endereços de AMM/tokens

**Checkpoint**: pipeline provisiona a topologia; par `ACTIVE` (skip-com-aviso sem ambiente).

---

## Phase 4: User Story 2 — Caminho de negócio: swap + breaker + bridge (Priority: P1)

**Goal**: exercitar swap na AMM, circuit breaker (pause/resume) e `SpokeBridge` (lock/release) sobre a
stack provisionada.

**Independent Test**: swap liquida; `pause` bloqueia; `signResume`×2 retoma; `lock` registra; sub-teste
`release`/refund (skipável).

### Tests / Implementation for User Story 2

- [X] T005 [US2] Adicionar em `scenario-b/toolkit/tests/e2e/pipeline_e2e_test.go` a asserção de **swap**: `castSend` de `swapTokensForExactTokens(address,address,uint256,uint256,address)` no `AMM`; confirmar via `getReserves`/saldo que o swap liquidou
- [X] T006 [US2] Adicionar a asserção de **circuit breaker**: `castSend pause(string)` (GOV_KEY) → `castCall isPaused()`==true → um swap **reverte**; `signResume(bytes32)` por dois CBs (GOV_KEY + CB2_KEY, quorum 2) → `isPaused()`==false → swap volta a funcionar
- [X] T007 [US2] Adicionar a asserção de **SpokeBridge lock**: `castSend lock(address,uint256,bytes32)` no `SPOKE_BRIDGE` → `castCall getLock(bytes32)` mostra amount/released=false
- [X] T008 [US2] Implementar o sub-teste **`TestPipeline_BridgeRefund`** (build tag `e2e`, skipável): `lock` → `castSend release(bytes32)` (GOV_KEY) → `getLock` released=true (fundos devolvidos, sem liquidação parcial); `t.Skip` com aviso quando o round-trip lock→mint→settle não é arranjável

**Checkpoint**: swap/breaker/bridge exercitados; refund como sub-teste skipável.

---

## Phase 5: User Story 3 — Idempotência de ponta a ponta (Priority: P2)

**Goal**: re-`apply` de cada modo converge (passos `skipped`/`done`; nada recriado).

**Independent Test**: após o pipeline, re-rodar `apply.Apply` de cada modo → report com passos
`skipped`/`done`; par ainda `ACTIVE`.

### Tests / Implementation for User Story 3

- [X] T009 [US3] Adicionar em `scenario-b/toolkit/tests/e2e/pipeline_e2e_test.go` a fase de **idempotência**: re-`apply.Apply` de found-hub/found-spoke/join e asserir que os `StepResult` vêm `skipped`/`done`; re-`castCall getPair` ainda `ACTIVE`; nenhum efeito duplicado

**Checkpoint**: re-apply converge; estado on-chain estável.

---

## Phase 6: User Story 4 — Baseline toolkit-native (Go) + E2E-STATUS (Priority: P2)

**Goal**: baseline de performance próprio (p95 quote/swap) + documento `E2E-STATUS.md`.

**Independent Test**: `go test -tags perf ./tests/perf/...` emite `quote_p95_ms`/`swap_p95_ms`
(skip-com-aviso sem stack); `E2E-STATUS.md` existe e cobre topologia/passos/comandos/métricas/estado.

### Tests / Implementation for User Story 4

- [X] T010 [P] [US4] Criar `scenario-b/toolkit/tests/perf/baseline_test.go` (build tag `perf`): `TestBaseline` mede a latência de **quote** (N× `getAmountOut`/quote) e **swap** (N× `swapTokensForExactTokens`) via `cast`/RPC, computa **p95** com `time`+`sort` (stdlib) e loga `quote_p95_ms`/`swap_p95_ms`; `requireTool`/`requireEnv` (`CBWEB3B_PERF_HUB_RPC`/`_AMM`) → skip-com-aviso; sem threshold de gate
- [X] T011 [P] [US4] Criar `scenario-b/toolkit/E2E-STATUS.md` (espelho do `scenario-a/toolkit/E2E-STATUS.md`): Summary, Topology covered (hub + 2 spokes + 1 banco + par ACTIVE), E2E steps, How to run (`go test -tags e2e`/`-tags perf`), Baseline metrics (p95), Current status (o que passa / o que é skip)

**Checkpoint**: baseline roda e loga p95; `E2E-STATUS.md` publicado.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T012 `gofmt`/`go vet ./...` e confirmar que os testes E2E/perf **compilam** com as tags (`go build -tags e2e ./tests/e2e/...` e `go build -tags perf ./tests/perf/...`) e **fazem skip-com-aviso** sem ambiente (`go test -tags e2e ./tests/e2e/...` → SKIP, 0 falhas); rodar a suíte padrão `go test ./...` (sem tags) verde
- [X] T013 [P] Atualizar a nota de status TK-B10 em `scenario-b/docs/design/scenario-b-toolkit-roadmap.md` (§15) e a tabela do toolkit em `scenario-b/README.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: helpers `castCall`/`castSend`.
- **Foundational (Phase 2)**: esqueleto do E2E (skip + apply helper) — bloqueia US1–US3.
- **US1 (Phase 3)**: depois da Foundational; provisiona a topologia (base para US2/US3).
- **US2 (Phase 4)**: depois de US1 (precisa do par `ACTIVE` + endereços).
- **US3 (Phase 5)**: depois de US1 (re-apply sobre o estado provisionado).
- **US4 (Phase 6)**: baseline + doc; o baseline depende de uma stack no ar (independe do teste de
  pipeline em si → `[P]` em relação ao arquivo `pipeline_e2e_test.go`).
- **Polish (Phase 7)**: por último.

### Within Each User Story / Notes

- US1–US3 vivem no mesmo `pipeline_e2e_test.go` ⇒ sequenciais nele; `tests/perf/baseline_test.go` e
  `E2E-STATUS.md` são arquivos distintos → `[P]`.
- **Mapa de realidade (plan)**: `SpokeBridge` = `lock`+`release` (+ mint relay-mediado; sem timeout
  on-chain); AMM swap = `swapTokensForExactTokens`; breaker = `pause`/`signResume`(quorum 2)/`isPaused`.
- Skip-com-aviso é obrigatório em todos os testes E2E/perf (nunca falha por ambiente; nunca falso verde).

### Parallel Opportunities

- US4: T010 (`baseline_test.go`) e T011 (`E2E-STATUS.md`) [P] entre si e em relação ao pipeline.
- Polish: T013 [P].

---

## Implementation Strategy

### MVP First (US1)

1. Setup (helpers) + Foundational (esqueleto do E2E).
2. US1 (pipeline provisiona a topologia; par `ACTIVE`) — prova que os modos compõem.

### Incremental Delivery

1. Foundational → US1 → US2 → US3 → US4 (baseline + doc) → Polish.
2. Cada US é uma asserção adicional no mesmo E2E (US1–US3) ou um artefato distinto (US4).

---

## Notes

- **Fase de verificação**: sem código de produção novo; deliverables = E2E de pipeline + baseline Go +
  `E2E-STATUS.md`.
- **Reuso**: `apply.Apply` (compõe os modos), os E2E por modo (TK-B7..B9), helpers `e2e`.
- **Baseline toolkit-native** (Q2): Go + stdlib (`time`/`sort`); **não** reusa o k6 de `tests/performance`.
- **timeout/refund** (Q1): sub-teste skipável exercitando `release` (não há timeout on-chain).
- Sem novas dependências Go; não importar `scenario-a/`; não alterar contratos/Makefiles/`deploy/local`.

---

## Phase 8: Convergence

- [X] T014 Estender o sub-teste de **idempotência** em `scenario-b/toolkit/tests/e2e/pipeline_e2e_test.go`: hoje re-`apply` só found-hub; re-`apply.Apply` também found-spoke (CB-A e CB-B) e join, asserindo que os `StepResult` de **cada** modo vêm `skipped`/`done` e que o par permanece `ACTIVE` — FR-005/SC-004 pedem "re-apply de **cada** modo converge" (partial)
