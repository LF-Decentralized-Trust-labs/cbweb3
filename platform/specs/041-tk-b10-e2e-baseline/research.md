# Phase 0 — Research: TK-B10 E2E completo + baseline

NEEDS CLARIFICATION resolvidos no `/speckit.clarify` (Q1 = timeout/refund como sub-teste skipável; Q2 =
baseline toolkit-native Go). Decisões técnicas consolidadas.

## D1 — E2E de pipeline compõe os modos via apply.Apply

- **Decisão**: um único teste `pipeline_e2e_test.go` chama `apply.Apply` em sequência: `found-hub` →
  `found-spoke` (CB-A) → `found-spoke` (CB-B) → `join` (banco) → (a cauda soberana roda dentro do
  `found-spoke` quando o manifesto tem `spec.pair`). Manifestos vêm de env vars
  (`CBWEB3B_E2E_*_MANIFEST`), `repoRoot`/`hubRPC` de env.
- **Rationale**: `apply.Apply` já é o ponto de composição; reusa os modos sem duplicar lógica (FR-010).
- **Alternativas**: shell script orquestrando o binário — rejeitado (menos testável, sai do padrão Go).

## D2 — Business path mapeado ao contrato real

- **Decisão**:
  - **swap**: `cast send <AMM> "swapTokensForExactTokens(address,address,uint256,uint256,address)"
    tokenIn tokenOut amountOut maxAmountIn to` → confirma via `getReserves`/saldo.
  - **breaker**: `cast send <AMM> "pause(string)" "e2e"` → `cast call <AMM> "isPaused()"` == true → um
    swap **reverte** (`whenNotPaused`); `signResume(proposalId)` por **dois** CBs (quorum 2) → auto-
    resume → `isPaused()` == false → swap volta a funcionar.
  - **bridge**: `cast send <SpokeBridge> "lock(address,uint256,bytes32)" token amount txId` →
    `getLock(txId)` mostra amount/released=false; **release/refund**: `cast send <SpokeBridge>
    "release(bytes32)" txId` (GOVERNANCE) → `getLock` released=true (fundos devolvidos).
- **Rationale**: são as funções que existem (AMM `swapTokensForExactTokens`; `SpokeBridge`
  `lock`/`release`/`getLock`). O "mint no hub" é relay-mediado (asserir no hub se viável).
- **Alternativas**: `mint`/`unlock`/`timeout` como fns do bridge — **não existem** (rejeitado).

## D3 — timeout/refund como sub-teste skipável (Q1)

- **Decisão**: o caminho de refund é `release` (reclaim de um lock não settled). Como **não há timeout
  on-chain**, o sub-teste `TestPipeline_BridgeRefund` exercita `lock` seguido de `release` para provar
  o reembolso sem liquidação parcial; se o round-trip lock→(mint relay)→settle não puder ser arranjado
  no ambiente, faz **skip-com-aviso**.
- **Rationale**: FR-004/SC-003 + Q1; fiel ao contrato (release) sem inventar timeout.
- **Alternativas**: avançar `block.timestamp` para um timeout inexistente — rejeitado (não há timeout).

## D4 — Idempotência de ponta a ponta

- **Decisão**: após o pipeline, re-chamar `apply.Apply` de cada modo e asserir que os `StepResult`
  vêm `skipped`/`done` (via o report do orquestrador) — nada recriado (mesmo genesis/endereços; par
  ainda `ACTIVE` via `cast call getPair`).
- **Rationale**: FR-005/SC-004; o estado on-chain + `.provisioning-state.yaml` garantem convergência.
- **Alternativas**: diffar arquivos — rejeitado (o report já expõe skip/done).

## D5 — skip-com-aviso (nunca falso verde)

- **Decisão**: reusar `requireTool(t,"docker"/"cast")` + `requireEnv(t,...)` do pacote `e2e`; ausência
  → `t.Skip` com aviso. Falha de provisionamento/asserção → `t.Fatal` com passo + report parcial.
- **Rationale**: FR-006/FR-007/SC-005; padrão de todos os E2E do toolkit.
- **Alternativas**: falhar quando o ambiente falta — rejeitado (falso vermelho em CI sem Docker).

## D6 — Baseline toolkit-native em Go (Q2)

- **Decisão**: `tests/perf/baseline_test.go` sob build tag **`perf`** — mede a latência de **quote**
  (`cast call <AMM> getAmountOut(...)` ou o endpoint de quote do gateway) e de **swap**
  (`swapTokensForExactTokens`) repetidas N vezes, computa **p95** com `time` + ordenação (stdlib), e
  loga as métricas (para copiar ao `E2E-STATUS.md`). Skip-com-aviso sem a stack.
- **Rationale**: Q2 (toolkit-native, não k6); sem novas deps (stdlib `time`/`sort`).
- **Alternativas**: reusar o k6 de `tests/performance` — rejeitado por Q2; wrk/vegeta — nova dep.

## D7 — E2E-STATUS.md (espelho do Cenário A)

- **Decisão**: `scenario-b/toolkit/E2E-STATUS.md` com: resumo, topologia coberta (hub + 2 spokes + 1
  banco + par ACTIVE), passos do E2E, comandos (`go test -tags e2e ...`, `-tags perf ...`), métricas
  p95 e estado atual (o que passa / o que é skip). Formato do `scenario-a/toolkit/E2E-STATUS.md`.
- **Rationale**: FR-009/SC-007; consistência com o Cenário A.
- **Alternativas**: seção no README — rejeitado (o A usa arquivo dedicado; espelhar).
