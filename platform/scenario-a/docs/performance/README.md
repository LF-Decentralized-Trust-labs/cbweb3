# Scenario A — Performance Benchmark Harness & Methodology (R1-12.3)

> Addresses **Report 1 — Deliverable 12, Finding 12.3 (P0)**: performance thresholds were
> *restated, not measured*. This directory provides a **turnkey harness + methodology** so a
> human can stand up real devnet infra, run each benchmark, and record measured numbers in
> [`RESULTS-TEMPLATE.md`](./RESULTS-TEMPLATE.md).
>
> **No measured numbers live here.** An isolated worktree cannot produce credible measurements;
> the numbers are intentionally deferred to a real infrastructure run.

---

## 0. TL;DR — one command (zero-config)

```bash
make scenario-a.perf-all        # from scenario-a/
```

That single command needs **no arguments and no manual setup**. It:

1. ~~**Stands up the stack** if it isn't already reachable~~ — **broken since the legacy path
   was retired.** `lib/stack.sh` shells out to `make spoke-a` / `make spoke-all`, and neither
   target exists. Stand the stack up first with `cd samples && ./deploy-all.sh`; the harness
   then finds it reachable and proceeds. Tracked as DEF-022.
2. **Mints `AUTH_TOKEN`** for bank-a and central-bank-a via `/api/v1/auth/login`, reading
   `KC_CLIENT_ID`/`KC_CLIENT_SECRET` from `backend/config/.env.infra.{bank-a,central-bank-a}`
   (`lib/auth.sh`).
3. **Funds the sender** (deposit → approve → fiat-exchange) so a 50 TPS run does not exhaust
   fCeBM and trip the error gate (`lib/fund.sh`).
4. **Runs the threshold benchmarks** — baseline (read/write p95), 50 TPS HTLC transfer,
   15 TPS Zeto escrow — each with its built-in k6 gates and `--summary-export` evidence.
5. **Correlates on-chain TTF** (`HTLCLocked` events via `eth_getLogs`, `lib/ttf.sh`).
6. **Writes `docs/performance/RESULTS-<UTC>.md`** with measured numbers + PASS/FAIL per
   threshold (`lib/results.sh`); raw evidence lands under `tests/performance/.artifacts/<run-id>/`.

The run exits non-zero if any k6 threshold gate is breached. Everything is overridable by env
var (`API_GW_URL`, `RECEIVER`, `DURATION`, `TRANSFER_TPS`, …) but **nothing is required**.

Related targets:

| Target | What it does |
|--------|--------------|
| `make scenario-a.perf-all` | Full zero-config threshold suite (above). |
| `make scenario-a.perf-all-dry` | Validates the orchestrator end-to-end with **no infra** (CI-safe smoke). |
| `make scenario-a.perf-soak-all` | **Opt-in** 12h soak (dedicated infra only; never in CI). |
| `make scenario-a.perf-{baseline,transfer,zeto,soak}` | Single benchmarks (require `AUTH_TOKEN`); see §3. |

The sections below document the methodology and the manual single-benchmark path that
`perf-all` automates.

---

## 1. Authoritative threshold table

| # | Threshold | Target | Scope | Source | Metric / Tool |
|---|-----------|--------|-------|--------|---------------|
| 1 | Token-transfer throughput | **50 TPS** sustained | HTLC lock initiation (`POST /api/v1/htlc/lock`) | Report 1 — D12 / Finding 12.3 | `htlc_locked_total` + `http_req_failed` via `k6/htlc-transfer-throughput.js` (`TRANSFER_TPS=50`) |
| 2 | Zeto (privacy) throughput | **15 TPS** sustained | Escrow request (`POST /api/v1/payments/escrows`) | Report 1 — D12 / Finding 12.3 | `zeto_escrow_submitted_total` via `k6/zeto-escrow-throughput.js` (`ZETO_TPS=15`) |
| 3 | Time-To-Finality (TTF) per spoke | **< 5s** | HTLC lock to on-chain `HTLCLocked` event | Report 1 — D12 / Finding 12.3 | Event correlation via Cacti relay logs — see §4 |
| 4 | API read p95 latency | **< 500ms** | `GET /api/v1/htlc/search`, `GET /api/v1/payments/fx/agreements` | docs/test-execution-plan.md | `htlc_search_latency_ms` p95 in `scenario-a-perf.js` |
| 5 | API write p95 latency | **< 1500ms** | `POST /api/v1/htlc/lock` (admission, not finality) | docs/test-execution-plan.md | `htlc_lock_latency_ms` p95 in `scenario-a-perf.js` |
| 6 | Error rate (steady state) | **< 1%** | All load scenarios | Report 1 — D12 / docs/test-execution-plan.md | `http_req_failed` rate in every k6 script |
| 7 | Resource stability | No memory leaks / node crash over **12h soak** | Full stack under moderate load | Report 1 — D12 / docs/test-execution-plan.md | `k6/soak.js` + out-of-band container/host metrics (§5) |

> A measured p95 that exceeds its gate by **>20%** MUST block merge (Decision 13).

### Threshold ↔ harness mapping

| Threshold | Script | Key env |
|-----------|--------|---------|
| 1 (50 TPS HTLC) | `k6/htlc-transfer-throughput.js` | `TRANSFER_TPS=50` |
| 2 (15 TPS Zeto) | `k6/zeto-escrow-throughput.js` | `ZETO_TPS=15` |
| 3 (TTF < 5s) | event correlation (§4) | seed ids via `PRINT_IDS=1` |
| 4 (read p95) | `tests/performance/scenario-a-perf.js` | `LOAD_MODEL=vus` (default) |
| 5 (write p95) | `tests/performance/scenario-a-perf.js` | `LOAD_MODEL=vus` (default) |
| 6 (<1% error) | every script (built-in `http_req_failed` threshold) | — |
| 7 (12h soak) | `k6/soak.js` | `DURATION=12h` |

---

## 2. Prerequisites

- [`k6`](https://k6.io) installed (`k6 version`).
- A running Scenario A stack, brought up **before** the harness: `cd samples && ./deploy-all.sh`
  from `scenario-a/`. The harness's own auto-bring-up is broken (DEF-022).
- An `AUTH_TOKEN`: a JWT from `/api/v1/auth/login` for a `commercial_bank` user on the entity
  being tested. Scenario A uses cookie-based auth; the scripts pass this value as the
  `access_token` cookie.
- A valid `RECEIVER` Paladin identity on the target spoke (e.g., `funded_operator@spoke-a-bank-c`).

### Stand up the stack

From `scenario-a/`:

```bash
cd samples && ./deploy-all.sh                 # full stack (Besu QBFT spokes A+B + Paladin + backend)
# ...or bring up layers:
#   (the per-phase deploy.up-* targets were removed with the legacy path)
```

### Obtain an AUTH_TOKEN

```bash
export AUTH_TOKEN=$(curl -s -c /tmp/cookies.txt \
  -X POST http://localhost:3001/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"<bank-user>","password":"<pw>"}' \
  | jq -r '.access_token // empty')
# If the token is set as a cookie (Set-Cookie: access_token=...), extract it:
export AUTH_TOKEN=$(grep access_token /tmp/cookies.txt | awk '{print $NF}')
```

(Use the same credentials the integration tests use — see `tests/integration/`.)

---

## 3. Running each benchmark

All commands run from `scenario-a/`. Override `API_GW_URL` to point at the bank-a API
gateway (default `http://localhost:3001`). Pass `--summary-export=<file>.json` to capture
machine-readable evidence.

### 3a. API latency baseline (thresholds 4, 5, 6)

```bash
make scenario-a.perf-baseline
# equivalent to:
API_GW_URL=http://localhost:3001 AUTH_TOKEN=$AUTH_TOKEN \
  RECEIVER=funded_operator@spoke-a-bank-c \
  k6 run tests/performance/scenario-a-perf.js
```

### 3b. 50 TPS HTLC token-transfer throughput (threshold 1)

```bash
make scenario-a.perf-transfer
# equivalent to:
API_GW_URL=http://localhost:3001 AUTH_TOKEN=$AUTH_TOKEN \
  RECEIVER=funded_operator@spoke-a-bank-c TRANSFER_TPS=50 DURATION=10m \
  k6 run tests/performance/k6/htlc-transfer-throughput.js
```

### 3c. 15 TPS Zeto escrow throughput (threshold 2)

```bash
make scenario-a.perf-zeto
# equivalent to:
API_GW_URL=http://localhost:3001 AUTH_TOKEN=$AUTH_TOKEN \
  ZETO_TPS=15 DURATION=10m \
  k6 run tests/performance/k6/zeto-escrow-throughput.js
```

### 3d. 12-hour soak (threshold 7)

```bash
make scenario-a.perf-soak     # WARNING: runs for 12 hours; dedicated infra only
# equivalent to:
API_GW_URL=http://localhost:3001 AUTH_TOKEN=$AUTH_TOKEN \
  RECEIVER=funded_operator@spoke-a-bank-c DURATION=12h \
  k6 run tests/performance/k6/soak.js
```

> Do **not** run the soak in CI or as a smoke test.

---

## 4. Measuring Time-To-Finality (TTF < 5s per spoke) — threshold 3

A black-box HTTP probe cannot measure end-to-end on-chain finality: `POST /api/v1/htlc/lock`
returns 201 once the transaction is submitted, but the HTLC is only finalized after QBFT
consensus (~1 block, 2s block time) and the Zeto proof is verified.

To measure real TTF:

1. Run the transfer harness with `PRINT_IDS=1` so it logs each contract_id:
   ```bash
   AUTH_TOKEN=$AUTH_TOKEN RECEIVER=funded_operator@spoke-a-bank-c \
   TRANSFER_TPS=10 DURATION=2m PRINT_IDS=1 \
   k6 run tests/performance/k6/htlc-transfer-throughput.js 2>&1 | grep CONTRACT_ID
   ```
2. For each contract_id, capture two timestamps:
   - **t0** = client send time (k6 iteration start, or the gateway's structured request log).
   - **t1** = the on-chain `HTLCLocked` event timestamp, read from the **Besu node logs** or
     via `eth_getLogs` for the HTLC coordination contract on Spoke-A.
3. `TTF = t1 - t0`. Report p50/p95 across the run. Gate: **p95 < 5s per spoke**.
4. For the cross-spoke leg (Cacti relay propagation to Spoke-B), also record:
   - **t2** = on-chain `HTLCLocked` event on Spoke-B (from Spoke-B Besu logs).
   - Cross-spoke TTF = t2 - t0. This is expected to be ≤ 15s per the relay propagation
     budget, but Report 1 gates only the per-spoke TTF at < 5s.

Record the methodology you actually used (log source, clock sync assumptions) in the results
report for reproducibility.

---

## 5. 12-hour soak — leak & drift evidence (threshold 7)

The k6 soak only generates load. The *evidence* comes from out-of-band observation:

- **Memory / goroutines**: scrape container RSS and Go runtime metrics (e.g. `/metrics`,
  `docker stats`, Prometheus) for each backend service. A sustained upward RSS/goroutine
  trend = leak.
- **DB connections**: watch Postgres `pg_stat_activity` connection count for unbounded growth.
- **Node health**: confirm no Besu node restart/crash on either spoke across the full window.
- **Cacti relay**: confirm no relay process restarts or event processing backlog growth.
- **Paladin / Zeto**: confirm no Paladin node crash; ZKP proof queue should not grow unbounded.

Capture before/after snapshots and the metric time series; reference them in the results report.

---

## 6. After the run

Fill in [`RESULTS-TEMPLATE.md`](./RESULTS-TEMPLATE.md) with the measured values, mark each
threshold PASS/FAIL, and attach raw k6 summaries (use `k6 run --summary-export=<file>.json`)
and the soak metric snapshots as evidence.
