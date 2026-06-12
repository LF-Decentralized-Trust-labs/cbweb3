# Scenario B — Performance Benchmark Harness & Methodology (R1-12.3)

> Addresses **Report 1 — Deliverable 12, Finding 12.3 (P0)**: performance thresholds were
> *restated, not measured*. This directory provides a **turnkey harness + methodology** so a
> human can stand up real devnet infra, run each benchmark, and record measured numbers in
> [`RESULTS-TEMPLATE.md`](./RESULTS-TEMPLATE.md).
>
> **No measured numbers live here.** This is the SCOPED deliverable (harness + methodology +
> results template). An isolated worktree cannot produce credible measurements; the numbers are
> intentionally deferred to a real infrastructure run.

---

## 1. Authoritative threshold table

| # | Threshold | Target | Scope | Source | Metric / Tool |
|---|-----------|--------|-------|--------|---------------|
| 1 | General value-transfer throughput | **50 TPS** sustained | Scenario B bridge transfer (`lock-mint`) | Report 1 — D12 / Finding 12.3 | `transfer_accepted_total` + `http_req_failed` via `k6/bridge-transfer-throughput.js` (`TRANSFER_TPS=50`) |
| 2 | Privacy-token (Zeto) transfer throughput | **15 TPS** sustained | Scenario B Zeto-mirrored transfer | Report 1 — D12 / Finding 12.3 | `k6/bridge-transfer-throughput.js` with `TOKEN_KIND=zeto TRANSFER_TPS=15` |
| 3 | AMM swap throughput | **30 TPS** sustained — **DRAFT, must be validated** | Scenario B AMM swap | `docs/test-execution-plan.md` (marked draft) / Report 1 — D12 | `scenario-b-perf.js` with `LOAD_MODEL=rate SWAP_TPS=30` |
| 4 | Time-To-Finality (TTF) | **< 5s** | Cross-network transfer end-to-end | Report 1 — D12 / Finding 12.3 | On-chain event correlation, NOT a black-box HTTP probe — see §4 |
| 5 | AMM quote p95 latency | **<= 300ms** | `GET /api/v2/amm/quote/exact-output` | SC-021 / `test-execution-plan.md` | `quote_latency_ms` p95 in `scenario-b-perf.js` |
| 6 | AMM swap p95 latency | **<= 6000ms** | `POST /api/v2/amm/swap/exact-output` (excl. on-chain finality) | SC-022 / `test-execution-plan.md` | `swap_latency_ms` p95 in `scenario-b-perf.js` |
| 7 | Pool status p95 latency | **<= 15000ms** | `GET /api/v2/amm/pool/{pair}/status` | SC-023 / `test-execution-plan.md` | `pool_latency_ms` p95 in `scenario-b-perf.js` |
| 8 | Error rate (steady state) | **< 1%** | All load scenarios | Report 1 — D12 / `test-execution-plan.md` | `http_req_failed` rate in every k6 script |
| 9 | Resource stability | No memory leaks / node crash over **12h soak** | Full stack under moderate load | Report 1 — D12 / `test-execution-plan.md` | `k6/soak.js` + out-of-band container/host metrics (§5) |

> A measured p95 that exceeds its gate by **>20%** MUST block merge (Decision 13).

### Threshold ↔ harness mapping (what runs what)

| Threshold | Script | Key env |
|-----------|--------|---------|
| 1 (50 TPS transfer) | `k6/bridge-transfer-throughput.js` | `TRANSFER_TPS=50 TOKEN_KIND=noto` |
| 2 (15 TPS Zeto) | `k6/bridge-transfer-throughput.js` | `TRANSFER_TPS=15 TOKEN_KIND=zeto` |
| 3 (30 TPS AMM swap, draft) | `tests/performance/scenario-b-perf.js` | `LOAD_MODEL=rate SWAP_TPS=30` |
| 4 (TTF < 5s) | event correlation (§4) | seed ids via `PRINT_IDS=1` |
| 5–7 (latency p95) | `tests/performance/scenario-b-perf.js` | `LOAD_MODEL=vus` (default) |
| 8 (<1% error) | every script (built-in `http_req_failed` threshold) | — |
| 9 (12h soak) | `k6/soak.js` | `DURATION=12h` |

---

## 2. Prerequisites

- [`k6`](https://k6.io) installed (`k6 version`). k6 is the established perf tool in this repo;
  there is **no Caliper** here — see §6 for why k6 covers the on-chain throughput scenarios.
- A running Scenario B stack reachable at `API_GW_URL` (default `http://localhost:3000`).
- An `AUTH_TOKEN`: a Keycloak-issued JWT with the `commercial_bank` realm role (required for
  swap and bridge transfers). Quote/pool endpoints are unauthenticated.

### Stand up the stack

From `scenario-b/`:

```bash
make scenario-b.up                 # full stack (Besu QBFT networks + infra + backend + relayer)
# ...or bring up layers individually:
#   make deploy.up-besu / deploy.up-infra / deploy.up-backend
make scenario-b.seed-sovereign-pair   # ensure the AMM pair (PAIR) has liquidity
```

### Obtain an AUTH_TOKEN

Mint a token for a `commercial_bank` user against the local Keycloak realm, e.g.:

```bash
export AUTH_TOKEN=$(curl -s "$KEYCLOAK_URL/realms/<realm>/protocol/openid-connect/token" \
  -d grant_type=password -d client_id=<client> \
  -d username=<bank-user> -d password=<pw> | jq -r .access_token)
```

(Use the same client/realm the integration tests use — see `tests/integration`.)

---

## 3. Running each benchmark

All commands run from `scenario-b/`. Override `API_GW_URL` if not on localhost.

### 3a. AMM latency baseline (thresholds 5–7, 8)

```bash
make scenario-b.perf-baseline
# equivalent to:
API_GW_URL=http://localhost:3000 AUTH_TOKEN=$AUTH_TOKEN \
  k6 run tests/performance/scenario-b-perf.js
```

### 3b. AMM 30 TPS throughput — validate the DRAFT gate (threshold 3)

```bash
make scenario-b.perf-amm-throughput
# equivalent to:
API_GW_URL=http://localhost:3000 AUTH_TOKEN=$AUTH_TOKEN \
  LOAD_MODEL=rate SWAP_TPS=30 QUOTE_TPS=60 DURATION=10m \
  k6 run tests/performance/scenario-b-perf.js
```

### 3c. 50 TPS value-transfer throughput (threshold 1)

```bash
make scenario-b.perf-transfer
# equivalent to:
API_GW_URL=http://localhost:3000 AUTH_TOKEN=$AUTH_TOKEN \
  TRANSFER_TPS=50 DURATION=10m \
  k6 run tests/performance/k6/bridge-transfer-throughput.js
```

### 3d. 15 TPS Zeto (privacy-token) throughput (threshold 2)

```bash
make scenario-b.perf-zeto
# equivalent to:
API_GW_URL=http://localhost:3000 AUTH_TOKEN=$AUTH_TOKEN \
  TOKEN_KIND=zeto TRANSFER_TPS=15 DURATION=10m \
  k6 run tests/performance/k6/bridge-transfer-throughput.js
```

### 3e. 12-hour soak (threshold 9)

```bash
make scenario-b.perf-soak     # WARNING: runs for 12 hours; dedicated infra only
# equivalent to:
API_GW_URL=http://localhost:3000 AUTH_TOKEN=$AUTH_TOKEN DURATION=12h \
  k6 run tests/performance/k6/soak.js
```

> Do **not** run the soak in CI or as a smoke test.

---

## 4. Measuring Time-To-Finality (TTF < 5s) — threshold 4

A black-box HTTP probe **cannot** measure end-to-end finality: the bridge endpoints return
`202 ACCEPTED` and the relayer finalises asynchronously across two Besu networks. k6 therefore
measures *API admission latency*, not TTF.

To measure real TTF:

1. Run the transfer harness with `PRINT_IDS=1` so it logs each created position id:
   ```bash
   AUTH_TOKEN=$AUTH_TOKEN TRANSFER_TPS=10 DURATION=2m PRINT_IDS=1 \
     k6 run tests/performance/k6/bridge-transfer-throughput.js 2>&1 | grep POSITION_ID
   ```
2. For each position id, capture two timestamps:
   - **t0** = client send time (k6 iteration start, or the gateway's request log).
   - **t1** = the on-chain `Minted` (lock-mint) or `Released` (burn-unlock) event timestamp,
     read from the **relayer logs** (`interop/hub-and-spoke/cacti`) or a Besu event indexer.
3. `TTF = t1 - t0`. Report p50/p95 across the run. Gate: **p95 < 5s**.

Record the methodology you actually used (log source, clock sync assumptions) in the results
report so the number is reproducible.

---

## 5. 12-hour soak — leak & drift evidence (threshold 9)

The k6 soak only generates load. The *evidence* comes from out-of-band observation over the
full window:

- **Memory / goroutines**: scrape container RSS and Go runtime metrics (e.g. `/metrics`,
  `docker stats`, or Prometheus) for each backend service. A sustained upward RSS/goroutine
  trend = leak.
- **DB connections**: watch Postgres `pg_stat_activity` connection count for unbounded growth.
- **Node health**: confirm no Besu node restart/crash across the window.
- **AMM invariant**: the constant-product value `x*y=k` for the pool MUST NOT decrease across
  the run (sample `pool/{pair}/status` before/after; cross-check with the Foundry invariant
  suite). Pool-state drift is a correctness failure, not just a perf one.

Capture before/after snapshots and the metric time series; reference them in the results report.

---

## 6. Why k6 and not Caliper

This repo standardises on **k6** for performance testing (`tests/performance/scenario-b-perf.js`,
the `scenario-b.perf-baseline` make target, and `docs/test-execution-plan.md`). There is no
Hyperledger Caliper config or dependency anywhere in the tree.

The throughput thresholds (50/15/30 TPS) are exercised **through the API Gateway**, which is the
real entry point for transfers and swaps and where the compliance gate is enforced — so driving
load at the REST layer with k6 measures the production path end-to-end (gateway → orchestrator →
ledger-gateway → on-chain). The on-chain settlement itself is observed via the relayer events
(see §4). Introducing Caliper would add a second, redundant load tool and a new runtime
dependency without covering anything k6 + event-correlation does not already cover. If a future
need arises to drive raw on-chain TPS *below* the API (bypassing the gateway), that would be the
point to justify Caliper in a PR per the stack rules — it is out of scope for R1-12.3.

---

## 7. After the run

Fill in [`RESULTS-TEMPLATE.md`](./RESULTS-TEMPLATE.md) with the measured values, mark each
threshold PASS/FAIL, and in particular **validate or revise the AMM 30 TPS draft target** based
on the measured devnet result. Attach raw k6 summaries (use `k6 run --summary-export=...`) and
the soak metric snapshots as evidence.
