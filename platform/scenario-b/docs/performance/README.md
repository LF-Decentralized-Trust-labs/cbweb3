# Scenario B — Performance Benchmark Harness & Methodology (R1-12.3)

> Addresses **Report 1 — Deliverable 12, Finding 12.3 (P0)**: performance thresholds were
> *restated, not measured*. This directory provides a **turnkey harness + methodology** plus a
> **single zero-config command** that runs the full suite and records measured numbers.

---

## 0. One command (zero-config) — `make scenario-b.perf-all`

```bash
# from scenario-b/
make scenario-b.perf-all
```

That's it. With **no arguments and no manual steps** the driver
(`tests/performance/run-all.sh`, orchestrating the helpers in `tests/performance/lib/`):

1. **Stack** — reuses a stack already serving at `API_GW_URL`, else runs `make scenario-b.up-perf`
   (the full settlement stack **minus the NOC monitoring portal**, which is not on the perf path;
   override the target with `PERF_UP_TARGET`).
2. **Auth** — mints a `commercial_bank` and a `central_bank` JWT via the gateway login
   `POST /api/v1/auth/login` (`{clientId,clientSecret}` → `{accessToken}`) — the same path
   `tests/integration` uses; **no direct Keycloak call** (it is blocked outside Docker).
   Client secrets are read from `backend/config/.env.infra.*` with local fallbacks.
3. **Perf profile** — clears any R1-10.1 daily transfer limits (so sustained 50 TPS does not
   trip `TRANSFER_LIMIT_EXCEEDED` and flood the <1% gate) and **asserts the circuit breaker is
   RESUMED before any swap runs** (constitution III).
4. **Seed** — ensures the AMM pair is `ACTIVE` with depth sized for the 30-TPS run (cooperative
   commit-reveal via `scenario-b.tryout-us1` when needed); warns if reserves are shallow.
5. **Benchmarks** — latency baseline, 50 TPS transfer, 15 TPS Zeto, plus the **three-way swap
   decomposition** (see below), each with `--summary-export` JSON capture.
6. **TTF** — measures end-to-end Time-To-Finality by on-chain correlation (§4).
7. **Results** — writes measured numbers + verdicts into [`RESULTS.md`](./RESULTS.md) and
   **validates-or-revises the AMM 30 TPS DRAFT** against the hub-only swap (3a).

### Swap throughput is measured three ways (R1-12.3 decomposition)

A single "AMM swap TPS" conflates a millisecond pool operation with multi-second cross-chain
bridge legs, so the suite splits it:

- **3a — Hub-only AMM swap** (`SWAP_MODE=amm`, `POST /swap/exact-output`): tokens already on the
  hub, no bridging. Isolates **pool capacity** — this is where the 30 TPS target is fair. Requires
  the payer onboarded + funded + `approve-amm` (done once by `lib/provision-swap.sh`).
- **3b — Cross-chain bridge finality** (transfer + TTF): settlement time for a lock-mint.
- **3c — Full cross-currency payment** (`SWAP_MODE=xc`, `POST /swap/cross-currency`): bridge-in →
  AMM → bridge-out across 3 networks. Reported as an end-to-end SLA, **not** gated at 30 TPS —
  it is bridge-bound by single-signer nonce serialisation + the 2s block cadence. Probed at
  `XC_TPS` (default 5). The "buffer" strategy (banks pre-holding hub balances) collapses 3c → 3a.

All helpers emit structured JSON logs to stdout. Raw evidence (k6 summaries, TTF samples) lands
in `tests/performance/results/<timestamp>/` (gitignored). The **12-hour soak is a SEPARATE
opt-in**: `make scenario-b.perf-soak` (runs the load generator + the out-of-band metrics
collector in parallel; never run it in CI).

Useful overrides (all optional): `DURATION` (default `3m` per throughput run; use `10m` for a
publication run), `SWAP_TPS`, `TRANSFER_TPS`, `ZETO_TPS`, `PAIR`, `API_GW_URL`.

The sections below document the methodology and the individual `make scenario-b.perf-*` targets
the one-command flow composes — use them to drive a single threshold in isolation.

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
- A running Scenario B stack reachable at `API_GW_URL` (default `http://localhost:18080`).
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

`make scenario-b.perf-all` mints this automatically. To get one by hand for the individual
targets, use the **gateway login** (direct Keycloak HTTP is blocked outside Docker — this is the
path `tests/integration` and `tests/performance/lib/auth.sh` use):

```bash
# commercial_bank token (for swap + transfer):
export AUTH_TOKEN=$(curl -s -H 'Content-Type: application/json' \
  -d '{"clientId":"bank-a-client","clientSecret":"<KC_CLIENT_SECRET>"}' \
  "$API_GW_URL/api/v1/auth/login" | jq -r .accessToken)
```

The client secret lives in `backend/config/.env.infra.bank-a` (`KC_CLIENT_SECRET`), with the
local fallback `bank-a-local-secret`. The central-bank token (for governance/profile calls) comes
from the CB-A gateway (`API_GW_CENTRAL_BANK_A_URL`, client `central-bank-a-client`).

---

## 3. Running each benchmark

All commands run from `scenario-b/`. Override `API_GW_URL` if not on localhost.

### 3a. AMM latency baseline (thresholds 5–7, 8)

```bash
make scenario-b.perf-baseline
# equivalent to:
API_GW_URL=http://localhost:18080 AUTH_TOKEN=$AUTH_TOKEN \
  k6 run tests/performance/scenario-b-perf.js
```

### 3b. AMM 30 TPS throughput — validate the DRAFT gate (threshold 3)

```bash
make scenario-b.perf-amm-throughput
# equivalent to:
API_GW_URL=http://localhost:18080 AUTH_TOKEN=$AUTH_TOKEN \
  LOAD_MODEL=rate SWAP_TPS=30 QUOTE_TPS=60 DURATION=10m \
  k6 run tests/performance/scenario-b-perf.js
```

### 3c. 50 TPS value-transfer throughput (threshold 1)

```bash
make scenario-b.perf-transfer
# equivalent to:
API_GW_URL=http://localhost:18080 AUTH_TOKEN=$AUTH_TOKEN \
  TRANSFER_TPS=50 DURATION=10m \
  k6 run tests/performance/k6/bridge-transfer-throughput.js
```

### 3d. 15 TPS Zeto (privacy-token) throughput (threshold 2)

```bash
make scenario-b.perf-zeto
# equivalent to:
API_GW_URL=http://localhost:18080 AUTH_TOKEN=$AUTH_TOKEN \
  TOKEN_KIND=zeto TRANSFER_TPS=15 DURATION=10m \
  k6 run tests/performance/k6/bridge-transfer-throughput.js
```

### 3e. 12-hour soak (threshold 9)

```bash
make scenario-b.perf-soak     # WARNING: runs for 12 hours; dedicated infra only
# equivalent to:
API_GW_URL=http://localhost:18080 AUTH_TOKEN=$AUTH_TOKEN DURATION=12h \
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

`make scenario-b.perf-all` **auto-generates** [`RESULTS.md`](./RESULTS.md) with the measured
values, a PASS/FAIL per threshold, and an explicit **validate-or-revise verdict for the AMM
30 TPS draft target** (`write-results.sh` parses the `--summary-export` JSON + the TTF result).
Raw k6 summaries and TTF samples are kept under `tests/performance/results/<timestamp>/`.

If you ran the individual `make scenario-b.perf-*` targets by hand instead of the one-command
flow, fill in [`RESULTS-TEMPLATE.md`](./RESULTS-TEMPLATE.md) yourself (it stays as the blank
reference form) and attach the raw summaries + soak metric snapshots as evidence. After a 12h
soak, inspect `results/soak-*/soak-metrics.jsonl` for RSS/connection trends, container restarts,
and the before/after AMM `x*y=k` invariant.
