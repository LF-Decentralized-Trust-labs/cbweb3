# Scenario A — Measured Performance Results (R1-12.3)

> **Template.** Copy to `RESULTS-<YYYY-MM-DD>.md` and fill in after a real devnet run.
> All "Measured" cells are intentionally empty here — **do not fabricate numbers**.
> Run procedure: [`README.md`](./README.md).

## Run metadata

| Field | Value |
|-------|-------|
| Date (ISO-8601) | _e.g. 2026-06-17T14:00:00Z_ |
| Operator | _name_ |
| Git commit | _SHA under test_ |
| Environment | _devnet host(s), CPU/RAM, network topology (spoke-a + spoke-b)_ |
| Besu version / consensus | 25.8.0 / QBFT |
| Stack brought up via | `make spoke-all` (note any deviations) |
| k6 version | _`k6 version` output_ |
| Cacti relay version | v2.1.0 (note any deviations) |
| Paladin version | v0.15 (note any deviations) |
| RECEIVER identity used | _funded_operator@spoke-a-bank-c or other_ |
| Raw evidence | _paths to k6 `--summary-export` JSONs, soak metric snapshots_ |

## Threshold results

| # | Threshold | Target | Measured | Pass/Fail | Notes |
|---|-----------|--------|----------|-----------|-------|
| 1 | HTLC token-transfer throughput | 50 TPS sustained | _____ TPS | ☐ PASS / ☐ FAIL | |
| 2 | Zeto (privacy) escrow throughput | 15 TPS sustained | _____ TPS | ☐ PASS / ☐ FAIL | |
| 3 | Time-To-Finality per spoke (TTF) | p95 < 5s | p50 ___ / p95 ___ s | ☐ PASS / ☐ FAIL | method: HTLCLocked event correlation (README §4) |
| 4 | API read p95 latency | < 500ms | _____ ms | ☐ PASS / ☐ FAIL | `htlc_search_latency_ms` + `fx_list_latency_ms` |
| 5 | API write p95 latency | < 1500ms | _____ ms | ☐ PASS / ☐ FAIL | `htlc_lock_latency_ms` (admission only, not finality) |
| 6 | Error rate (steady state) | < 1% | _____ % | ☐ PASS / ☐ FAIL | `http_req_failed` |
| 7 | Resource stability (12h soak) | no leak / crash | _____ | ☐ PASS / ☐ FAIL | RSS/goroutine/DB-conn trend, Besu + relay + Paladin health |

> A measured p95 exceeding its gate by **>20%** MUST block merge (Decision 13). Flag any such
> result explicitly in Notes.

## Per-scenario detail (paste k6 summaries)

### API latency baseline (`scenario-a-perf.js`, thresholds 4–5, 6)
```
<paste k6 summary>
```

### 50 TPS HTLC transfer (`htlc-transfer-throughput.js`, threshold 1)
```
<paste k6 summary>
```

### 15 TPS Zeto escrow (`zeto-escrow-throughput.js`, threshold 2)
```
<paste k6 summary>
```

### Time-To-Finality (event correlation, threshold 3)
```
<paste TTF p50/p95 + how t0/t1 were captured (log source, clock sync)>
```

### 12h soak (`soak.js`, threshold 7)
```
<paste k6 summary + before/after RSS, goroutine, DB-conn snapshots>
<Besu node health across full window>
<Cacti relay health (no restarts, no backlog)>
<Paladin/Zeto health (no crashes, ZKP queue stable)>
```

## Overall verdict

- Run status: ☐ All thresholds PASS / ☐ One or more FAIL (block merge per Decision 13)
- Follow-ups / tickets: _____________________________________________
