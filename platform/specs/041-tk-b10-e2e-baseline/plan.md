# Implementation Plan: Toolkit do Cenário B — E2E completo + baseline (TK-B10)

**Branch**: `041-tk-b10-e2e-baseline` | **Date**: 2026-07-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/041-tk-b10-e2e-baseline/spec.md`

## Summary

Fechar o toolkit com (1) um **E2E de pipeline completo** (build tag `e2e`) que compõe os modos já
entregues — `found-hub` → `found-spoke` ×2 (CB-A/CB-B) → `join` (banco) → cauda soberana — via
`apply.Apply` contra Docker real, e então exercita o **caminho de negócio**: swap na AMM
(`swapTokensForExactTokens`), circuit breaker (`pause`/`signResume`, `isPaused`) e `SpokeBridge`
(`lock`/`release`, com o **timeout/refund** como sub-teste skipável); (2) valida **idempotência** de
ponta a ponta (re-`apply` → passos `skipped`/`done`); (3) um **baseline de performance toolkit-native**
(harness em Go) medindo p95 de quote/swap; e (4) o doc **`toolkit/E2E-STATUS.md`**. Todo E2E/baseline
faz **skip-com-aviso** quando o ambiente está ausente. Reusa os E2E por modo (TK-B7..B9), o
`apply.Apply` e os helpers `requireTool`/`requireEnv`/`hasCode`.

**Mapa de realidade (contratos).** O roadmap fala em "lock→mint / burn→unlock / timeout-refund"; o
`SpokeBridge` real expõe apenas `lock(token,amount,txId)` + `release(txId)` (GOVERNANCE) + `getLock`.
O **mint no hub** é **mediado pelo relay** (observa `AssetLocked` → hub minta), não é função do
`SpokeBridge`; **não há timeout on-chain** — `release` é o caminho de reclaim/refund. O E2E mapeia:
lock via `lock`, refund/unlock via `release`, e a "cunhagem no hub" como asserção relay-mediada (se
viável, senão documentada). A AMM tem só `swapTokensForExactTokens` (não há `swapExactTokensForTokens`).

## Technical Context

**Language/Version**: Go 1.26 (testes `e2e`/`perf` no módulo `scenario-b/toolkit`).
**Primary Dependencies**: **nenhuma nova** — `testing`, `os/exec` (`cast`/`forge`), `net/http`
(RPC/quote), `time` (p95), stdlib. Ferramentas externas (runtime): Docker Compose v2, Foundry
(`cast`/`forge`), `hyperledger/besu:25.8.0`, relay Cacti, Keycloak.
**Storage**: nenhum novo — o E2E lê estado on-chain (via `cast`/RPC) e o report do `apply`; o baseline
não persiste (emite métricas no log/`E2E-STATUS.md`).
**Testing**: `go test -tags e2e ./tests/e2e/...` (pipeline + business path + idempotência;
skip-com-aviso) e `go test -tags perf ./tests/perf/...` (baseline p95 quote/swap; skip-com-aviso).
**Target Platform**: binário `cbweb3b` já entregue; esta fase é **teste + doc**, sem novo código de
produção do toolkit.
**Project Type**: CLI + biblioteca Go (fase de verificação).
**Performance Goals**: baseline informativo (p95 de quote/swap) — sem threshold de gate nesta fase.
**Constraints**: skip-com-aviso (nunca falso verde); reusar modos/`apply` (sem duplicar lógica);
**não** alterar contratos/Makefiles/`deploy/local`; **não** importar `scenario-a/`; sem novas deps Go;
timeout/refund como sub-teste skipável; baseline toolkit-native (não reusar o k6 de `tests/performance`).
**Scale/Scope**: 1 E2E de pipeline (novo), 1 harness de baseline (novo), 1 doc `E2E-STATUS.md`; 0
contrato novo; 0 modo novo.

## Constitution Check

*GATE: deve passar antes da Fase 0; re-checado após a Fase 1.*

| Princípio | Avaliação (TK-B10) |
|---|---|
| **I. Scenario-Scoped Independence** | ✅ Só toolkit + contratos/serviços existentes do B; **não** importa `scenario-a/`; não edita contratos/Makefiles/`deploy/local`. |
| **II. Privacy by Design** | ✅ O E2E não introduz segredos; usa chaves via seams/flags como os modos; nada serializado em bundle/estado. |
| **III. Atomic Settlement** | ✅ **Reforça/valida**: o E2E **verifica** o breaker antes dos swaps (`pause` bloqueia, `resume` quorum 2 retoma) e o `lock`/`release` do `SpokeBridge` **sem liquidação parcial** — exatamente o Princípio III. |
| **IV. Compliance Gate** | ✅ O swap/bridge são gated por `onlyVerified` (IdentityRegistry); o E2E opera sobre participantes registrados, não contorna o gate. |
| **V. Test-First** | ✅ **Cumpre**: é a **suíte E2E do cenário** (Princípio V exige E2E por cenário) + baseline (production-grade). |
| **VI. Observability** | ✅ Falhas do E2E reportam passo + report parcial; sem swallow; o baseline emite métricas. |

**Novas dependências**: nenhuma. **Resultado (pré-Fase 0 e pós-Fase 1)**: PASS — cumpre V e reforça III.

## Project Structure

### Documentation (this feature)

```text
specs/041-tk-b10-e2e-baseline/
├── plan.md, spec.md
├── research.md          # Fase 0 — mapa lock/release, swap fn, breaker, skip-com-aviso, baseline Go
├── data-model.md        # Fase 1 — topologia E2E, env vars, fases do pipeline, métricas do baseline
├── quickstart.md        # Fase 1 — como rodar o E2E e o baseline
├── contracts/           # Fase 1 — contrato do E2E (env, ordem, asserções) + baseline
└── checklists/requirements.md
```

### Source Code (repository)

```text
scenario-b/toolkit/
├── tests/e2e/
│   ├── pipeline_e2e_test.go     # NEW — E2E de pipeline completo (found-hub→spoke×2→join→cauda) + swap/breaker/bridge + idempotência
│   ├── rpc_e2e.go               # (existente) helpers hasCode/RPC — estender com castCall/castSend/isPaused se preciso
│   ├── found_hub_e2e_test.go    # (existente) reusado/inalterado
│   ├── found_spoke_e2e_test.go  # (existente)
│   ├── join_e2e_test.go         # (existente)
│   └── sovereign_pair_e2e_test.go # (existente)
├── tests/perf/
│   └── baseline_test.go         # NEW — baseline toolkit-native (build tag `perf`): p95 de quote/swap
└── E2E-STATUS.md                # NEW — estado do E2E (espelho do scenario-a/toolkit/E2E-STATUS.md)
```

**Structure Decision**: TK-B10 é **fase de verificação** — adiciona testes + doc, sem código de
produção. O E2E de pipeline reusa `apply.Apply` (compõe os modos) e os helpers do pacote `e2e`; o
business path usa `cast`/RPC contra os contratos provisionados. O baseline é um teste Go sob build tag
`perf` (separado do `e2e` para poder rodar latência sem o pipeline completo, contra uma stack já no
ar). Ambos skip-com-aviso via `requireTool`/`requireEnv`.

## Complexity Tracking

| Deviation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| Mapear "lock→mint/burn→unlock/timeout-refund" (roadmap/spec) para `lock`+`release` (+ mint relay-mediado) | O `SpokeBridge` real só tem `lock`/`release`; o mint é do relay/hub e **não há timeout on-chain**. O E2E precisa asserir o que existe. | Asserir `mint`/`unlock`/`timeout` como funções do `SpokeBridge` falharia (não existem). O refund é `release`; o timeout vira sub-teste skipável (reclaim de um lock não settled). |
| Baseline como teste Go sob tag `perf` (não `e2e`) | Permite medir latência contra uma stack já no ar sem re-rodar o pipeline inteiro; separa "provou que funciona" de "mediu latência". | Embutir o baseline no `e2e` acoplaria a métrica ao pipeline completo (mais lento/frágil) e misturaria dois objetivos. |
