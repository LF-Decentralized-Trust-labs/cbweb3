# Quickstart — TK-B10 E2E completo + baseline

Fase de verificação do toolkit: um E2E de pipeline completo + um baseline de performance + o
`E2E-STATUS.md`. Tudo faz **skip-com-aviso** sem o ambiente.

## 1. E2E de pipeline completo (`e2e`)

Requer Docker + Foundry (`cast`) + Besu (+ relay). Manifestos e endereços via env.

```bash
cd scenario-b/toolkit

export CBWEB3B_E2E_HUB_MANIFEST=./hub.found-hub.yaml
export CBWEB3B_E2E_CBA_MANIFEST=./central-bank-a.found-spoke.yaml   # com spec.pair
export CBWEB3B_E2E_CBB_MANIFEST=./central-bank-b.found-spoke.yaml   # com o mesmo spec.pair
export CBWEB3B_E2E_JOIN_MANIFEST=./bank-a.join.yaml
export CBWEB3B_E2E_REPO_ROOT=$(git rev-parse --show-toplevel)
export CBWEB3B_E2E_HUB_RPC=http://host.docker.internal:8845
export CBWEB3B_E2E_AMM=0x...            # AMM do par
export CBWEB3B_E2E_PAIR_REGISTRY=0x...
export CBWEB3B_E2E_SPOKE_BRIDGE=0x...
export CBWEB3B_E2E_GOV_KEY=0x...        # governança (pause / release)
export CBWEB3B_E2E_CB2_KEY=0x...        # segundo CB (signResume → quorum 2)

go test -tags e2e ./tests/e2e/... -run TestPipeline -v
```

Fluxo: provisiona hub → spoke A → spoke B → banco (join) → par soberano; confirma `getPair == ACTIVE`;
faz um swap; pausa o breaker (swap recusado) e retoma com quorum 2; exercita `lock` no `SpokeBridge`;
re-roda `apply` de cada modo e confirma convergência (skipped/done). Ambiente ausente ⇒ **skip com
aviso**.

Sub-teste de refund (skipável):

```bash
go test -tags e2e ./tests/e2e/... -run TestPipeline_BridgeRefund -v
# lock → release (GOVERNANCE) → getLock released=true; skip-com-aviso se o round-trip não é arranjável
```

## 2. Baseline de performance toolkit-native (`perf`)

Requer uma stack já no ar (provisionada pelo toolkit).

```bash
export CBWEB3B_PERF_HUB_RPC=http://host.docker.internal:8845
export CBWEB3B_PERF_AMM=0x...

go test -tags perf ./tests/perf/... -run TestBaseline -v
# → loga quote_p95_ms e swap_p95_ms (informativo; sem threshold de gate)
```

Copie as métricas p95 para `E2E-STATUS.md`.

## 3. E2E-STATUS

```bash
sed -n '1,40p' scenario-b/toolkit/E2E-STATUS.md
# Summary, Topology covered, E2E steps, How to run, Baseline metrics (p95), Current status
```

## 4. Sem ambiente (CI sem Docker)

```bash
go test -tags e2e ./tests/e2e/...    # todos os testes: SKIP com aviso (0 falhas, 0 falsos verdes)
go test -tags perf ./tests/perf/...  # SKIP com aviso
```
