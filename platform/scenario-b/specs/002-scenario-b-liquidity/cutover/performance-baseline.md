# Performance Baseline — Scenario B (T105 / FR-037 / Decision 13)

> Baseline de performance exigido como gate de merge. Desvio >20% dos targets
> bloqueia cutover (SC-021 / SC-022 / SC-023).

## Targets (gates canonicos)

| Endpoint                              | Gate p95 | FR / SC |
|---------------------------------------|----------|---------|
| `GET  /api/v2/amm/quote/exact-output` | ≤ 300ms  | SC-021  |
| `POST /api/v2/amm/swap/exact-output`  | ≤ 6 s    | SC-022  |
| `GET  /api/v2/amm/pool/{pair}/status` | ≤ 15 s   | SC-023 (liquidity monitor cadence) |

## Ferramenta

- **k6** (https://k6.io) — script em `tests/performance/scenario-b-perf.js`.
- Thresholds configurados nativos no script: p95 gates + `http_req_failed < 1%`.

## Como executar

```bash
# Pre-requisitos: stack Scenario B up, AUTH_TOKEN com role commercial_bank.
AUTH_TOKEN=$(curl -sS -X POST "$KEYCLOAK_URL/realms/cbweb3/protocol/openid-connect/token" \
  -d grant_type=password -d client_id=api-gateway \
  -d username=$COMMERCIAL_BANK_USER -d password=$COMMERCIAL_BANK_PASS | jq -r .access_token)

API_GW_URL=http://localhost:3000 AUTH_TOKEN=$AUTH_TOKEN \
  k6 run tests/performance/scenario-b-perf.js

# Ou via Makefile:
make scenario-b.perf-baseline
```

## Resultado baseline (primeiro ciclo local — preenchido pela execucao)

> O bloco abaixo deve ser atualizado com os numeros reais de cada execucao.
> Estrutura padrao para capturar evidencia.

| Endpoint     | Iteracoes | p50   | p95   | p99   | Erro | Gate    | Delta |
|--------------|-----------|-------|-------|-------|------|---------|-------|
| quote        | -         | -     | -     | -     | -    | ≤300ms  | -     |
| swap         | -         | -     | -     | -     | -    | ≤6s     | -     |
| pool/status  | -         | -     | -     | -     | -    | ≤15s    | -     |

- **Ambiente**: local dev (laptop) — referencia, nao vinculante.
- **Data/Hora**: `<timestamp>`
- **Commit**: `<git rev-parse HEAD>`
- **Versao Go**: `go version`
- **Versao Foundry**: `forge --version`

## Interpretacao

1. **p95 dentro do gate**: cutover autorizado.
2. **p95 entre 100%-120% do gate**: gate aceito com risco documentado no
   `risk-register.md`.
3. **p95 > 120% do gate**: **BLOQUEIA MERGE** (Decision 13). Acao requerida:
   profiling + replan.

## Fatores conhecidos que influenciam

- Latencia de RPC Besu (synchronous signing)
- Round-trip para Postgres em persistencia de `SwapOrderScenarioB`
- Validacao ZK-Pointer (`ComplianceGate`) — atualmente lookup simples
- Latencia do Cacti Relayer para Lock/Burn (fora do gate de quote/swap)

## Itens NAO medidos nesta baseline (por decisao)

- Throughput saturado (fora do escopo — nao ha SLO de RPS)
- Latencia end-to-end com Relayer (depende de 2 chains + Cacti, ver tryout E2E)
- Observabilidade estruturada (Decision 16 — nao ha OTel nesta iteracao)
- Rate limiting (Decision 17 — sem middleware)

## Evolucao

- Baseline deve ser reexecutado apos mudancas que toquem:
  - `QuoteService`, `SwapService`, `PoolStatusService`
  - Clientes EVM em `backend/shared/blockchain/scenariob/*`
  - Schema GORM dos modelos criticos
- Regressoes > 20% em relacao ao baseline anterior sao **CRITICAL**.
