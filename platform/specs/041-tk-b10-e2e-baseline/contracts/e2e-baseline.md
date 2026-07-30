# Contract — TK-B10 E2E pipeline + baseline

Fase de verificação: testes + doc. Sem CLI/API novos; consome `apply.Apply` e os contratos
provisionados.

## E2E de pipeline (`tests/e2e/pipeline_e2e_test.go`, build tag `e2e`)

- **Gatilho**: `go test -tags e2e ./tests/e2e/... -run TestPipeline`.
- **Skip-com-aviso**: `requireTool(t,"docker")`, `requireTool(t,"cast")`, `requireEnv(t, ...)` para os
  manifestos/endereços/chaves; ausente ⇒ `t.Skip` com aviso (nunca falso verde).
- **Fases** (ver data-model): provision (found-hub→spoke A→spoke B→join via `apply.Apply`) → par
  `ACTIVE` → swap → breaker (pause/resume) → bridge lock → idempotência (re-apply).
- **Falha**: `t.Fatal` com o passo + report parcial do `apply` (sem swallow).

### Invariantes asseguráveis
1. Pipeline provisiona sem erro; report done/skipped (SC-001).
2. `getPair(pairId)` == `ACTIVE` (SC-001).
3. Swap liquida (saldo/reserva muda) (SC-002).
4. `pause` → `isPaused()`==true → swap reverte; `signResume`×2 → `isPaused()`==false → swap ok (SC-002).
5. `lock` registra (`getLock` released=false); sub-teste `release`→released=true (refund), skipável
   (SC-003).
6. Re-`apply` de cada modo → passos `skipped`/`done`; par ainda `ACTIVE` (SC-004).
7. Ambiente ausente ⇒ skip-com-aviso (SC-005).

## Sub-teste de refund (`TestPipeline_BridgeRefund`, tag `e2e`)

- `lock(token,amount,txId)` → `release(txId)` (GOVERNANCE) → `getLock` released=true; fundos devolvidos
  (sem liquidação parcial). **Skip-com-aviso** se o round-trip lock→mint→settle não é arranjável.

## Baseline toolkit-native (`tests/perf/baseline_test.go`, build tag `perf`)

- **Gatilho**: `go test -tags perf ./tests/perf/... -run TestBaseline`.
- **Skip-com-aviso**: `requireTool`/`requireEnv` (RPC + AMM da stack-alvo).
- Mede **quote p95** (N× getAmountOut/quote) e **swap p95** (N× swapTokensForExactTokens); computa p95
  com `sort` (stdlib); loga `quote_p95_ms`/`swap_p95_ms`. Informativo (sem gate). (SC-006)

## E2E-STATUS.md

- `scenario-b/toolkit/E2E-STATUS.md`: Summary, Topology, E2E steps, How to run, Baseline metrics,
  Current status. Espelho do `scenario-a/toolkit/E2E-STATUS.md`. (SC-007)

## Restrições
- Reusa `apply.Apply` + os E2E por modo (TK-B7..B9) + helpers `e2e`; **não** duplica lógica de modo.
- **Não** altera contratos/Makefiles/`deploy/local`; **não** importa `scenario-a/`; sem novas deps Go.
