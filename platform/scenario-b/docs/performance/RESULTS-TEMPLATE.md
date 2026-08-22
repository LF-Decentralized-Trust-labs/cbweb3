# Scenario B — Measured Performance Results (R1-12.3)

> **Template.** Copy to `RESULTS-<YYYY-MM-DD>.md` and fill in after a real devnet run.
> All "Measured" cells are intentionally empty here — **do not fabricate numbers**.
> Run procedure: [`README.md`](./README.md).

## Run metadata

| Field | Value |
|-------|-------|
| Date (ISO-8601) | _e.g. 2026-06-11T14:00:00Z_ |
| Operator | _name_ |
| Git commit | _SHA under test_ |
| Environment | _devnet host(s), CPU/RAM, network topology (spokes + hub)_ |
| Besu version / consensus | 25.8.0 / QBFT |
| Stack brought up via | `cd samples && ./deploy-all.sh` (note any deviations) |
| k6 version | _`k6 version` output_ |
| AMM pair / liquidity seeded | _PAIR + seeded reserves_ |
| Raw evidence | _paths to k6 `--summary-export` JSONs, soak metric snapshots_ |

## Threshold results

| # | Threshold | Target | Measured | Pass/Fail | Notes |
|---|-----------|--------|----------|-----------|-------|
| 1 | Value-transfer throughput | 50 TPS sustained | _____ TPS | ☐ PASS / ☐ FAIL | |
| 2 | Zeto (privacy) transfer throughput | 15 TPS sustained | _____ TPS | ☐ PASS / ☐ FAIL | |
| 3 | AMM swap throughput **(DRAFT)** | 30 TPS sustained | _____ TPS | ☐ PASS / ☐ FAIL | see "AMM draft validation" below |
| 4 | Time-To-Finality (TTF) | p95 < 5s | p50 ___ / p95 ___ s | ☐ PASS / ☐ FAIL | method: event correlation (README §4) |
| 5 | AMM quote p95 latency | <= 300ms | _____ ms | ☐ PASS / ☐ FAIL | |
| 6 | AMM swap p95 latency | <= 6000ms | _____ ms | ☐ PASS / ☐ FAIL | excl. on-chain finality |
| 7 | Pool status p95 latency | <= 15000ms | _____ ms | ☐ PASS / ☐ FAIL | |
| 8 | Error rate (steady state) | < 1% | _____ % | ☐ PASS / ☐ FAIL | `http_req_failed` |
| 9 | Resource stability (12h soak) | no leak / crash | _____ | ☐ PASS / ☐ FAIL | RSS/goroutine/DB-conn trend, node health |

> Decision 13: a measured p95 exceeding its gate by **>20%** MUST block merge. Flag any such
> result explicitly in Notes.

## Per-scenario detail (paste k6 summaries)

### AMM latency baseline (`scenario-b-perf.js`, thresholds 5–7, 8)
```
<paste k6 summary>
```

### AMM 30 TPS throughput (`scenario-b-perf.js LOAD_MODEL=rate`, threshold 3)
```
<paste k6 summary>
```

### 50 TPS transfer (`bridge-transfer-throughput.js`, threshold 1)
```
<paste k6 summary>
```

### 15 TPS Zeto (`bridge-transfer-throughput.js TOKEN_KIND=zeto`, threshold 2)
```
<paste k6 summary>
```

### Time-To-Finality (event correlation, threshold 4)
```
<paste TTF p50/p95 + how t0/t1 were captured>
```

### 12h soak (`soak.js`, threshold 9)
```
<paste k6 summary + before/after RSS, goroutine, DB-conn snapshots; AMM x*y=k before/after>
```

## AMM draft 30 TPS validation (REQUIRED outcome of this run)

The AMM swap throughput target of **30 TPS is a DRAFT** value (set below the 50 TPS transfer
target due to constant-product math overhead per swap) and must be validated against measured
devnet results before it becomes a formal gate.

- [ ] **Validate** — measured sustained AMM swap TPS >= 30 at <1% error and within latency
      gates ⇒ promote 30 TPS from draft to formal gate; update
      `docs/test-execution-plan.md` and the threshold table in `README.md`.
- [ ] **Revise** — measured ceiling < 30 TPS ⇒ record the actual sustainable TPS, set the new
      gate to the measured value (minus an agreed margin), and document the bottleneck
      (AMM math / gas / mempool / block time).

Decision + rationale: _____________________________________________

## Overall verdict

- Run status: ☐ All thresholds PASS / ☐ One or more FAIL (block merge per Decision 13)
- Follow-ups / tickets: _____________________________________________
