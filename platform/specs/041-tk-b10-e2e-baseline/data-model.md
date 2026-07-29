# Phase 1 — Data Model: TK-B10 E2E + baseline

Sem persistência nova. "Entidades" = a topologia do E2E, o contrato de env vars, as fases do pipeline e
as métricas do baseline.

## Topologia do E2E

| Entidade | Papel | Provisionada por |
|---|---|---|
| Hub | rede validadora + contratos base + relay + NOC | `apply found-hub` |
| Spoke A (CB-A) | rede soberana; CB validador único; propõe o par | `apply found-spoke` (CB-A, `spec.pair`) |
| Spoke B (CB-B) | rede soberana; confirma o par | `apply found-spoke` (CB-B, `spec.pair`) |
| Banco comercial | full node não-validador no Spoke A (ou B) | `apply join` |
| Par soberano | `PROPOSED`→`ACTIVE`, AMM + liquidez de ambos os lados | cauda soberana (dentro de found-spoke) |

## Contrato de env vars (skip-com-aviso quando ausentes)

| Env | Uso |
|---|---|
| `CBWEB3B_E2E_HUB_MANIFEST` | manifesto found-hub |
| `CBWEB3B_E2E_CBA_MANIFEST` | manifesto found-spoke CB-A (com `spec.pair`) |
| `CBWEB3B_E2E_CBB_MANIFEST` | manifesto found-spoke CB-B (com `spec.pair`) |
| `CBWEB3B_E2E_JOIN_MANIFEST` | manifesto join (banco) |
| `CBWEB3B_E2E_REPO_ROOT` | raiz do repo (contracts/templates) |
| `CBWEB3B_E2E_HUB_RPC` | RPC do hub (swap/breaker/getPair) |
| `CBWEB3B_E2E_AMM` / `_PAIR_REGISTRY` / `_SPOKE_BRIDGE` | endereços para as asserções de negócio |
| `CBWEB3B_E2E_GOV_KEY` / `_CB2_KEY` | chaves p/ pause + signResume (quorum 2) e release (GOVERNANCE) |
| `CBWEB3B_PERF_HUB_RPC` / `_AMM` (perf) | stack-alvo do baseline |

Ferramentas: `requireTool(t,"docker")`, `requireTool(t,"cast")`.

## Fases do pipeline E2E (`pipeline_e2e_test.go`, tag `e2e`)

| Fase | Ação | Asserção |
|---|---|---|
| 1. provision | `apply.Apply` found-hub → found-spoke A → found-spoke B → join | sem erro; report done/skipped |
| 2. pair active | `cast call getPair(pairId)` | status `ACTIVE` |
| 3. swap | `cast send swapTokensForExactTokens(...)` | saldo/reserva muda; liquida |
| 4. breaker | `pause` → `isPaused`==true → swap reverte; `signResume`×2 → `isPaused`==false → swap ok | pause bloqueia; resume (quorum 2) retoma |
| 5. bridge | `lock(token,amount,txId)` → `getLock` (released=false) | lock registrado |
| 6. idempotência | re-`apply` de cada modo | passos `skipped`/`done`; par ainda `ACTIVE` |

Sub-teste separado `TestPipeline_BridgeRefund` (skipável): `lock` → `release(txId)` → `getLock`
(released=true, fundos devolvidos); skip-com-aviso se o round-trip não é arranjável.

## Métricas do baseline (`tests/perf/baseline_test.go`, tag `perf`)

| Métrica | Como | Saída |
|---|---|---|
| quote p95 | N× `cast call getAmountOut(...)` (ou quote do gateway), medir `time.Since` | p95 ms |
| swap p95 | N× `swapTokensForExactTokens`, medir `time.Since` | p95 ms |

Cálculo p95: coletar durações, `sort`, índice `ceil(0.95*N)-1` (stdlib). Loga
`quote_p95_ms`/`swap_p95_ms`. Skip-com-aviso sem a stack. Sem threshold de gate (informativo).

## E2E-STATUS.md (doc)

Seções: Summary, Topology covered, E2E steps, How to run (`go test -tags e2e`/`-tags perf`), Baseline
metrics (p95), Current status (o que passa / o que é skip). Espelha
`scenario-a/toolkit/E2E-STATUS.md`.
