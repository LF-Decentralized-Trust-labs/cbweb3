# Scenario A — AWS EC2 performance run (2026-06-19)

Performance evidence captured on a dedicated AWS box, to test whether the Scenario A
throughput ceiling is hardware-bound.

## Environment
- **Instance:** AWS EC2 `c6a.8xlarge` — 32 vCPU, 64 GB RAM, AMD EPYC (on-demand, us-east-1)
- **OS:** Ubuntu 24.04 LTS
- **Toolchain:** Docker 29.6 + Compose, Java 21, Foundry 1.7.1, k6 v2.0.0, Go 1.26.4
- **Stack:** full `make spoke-all` (both spokes + Paladin + Cacti relay), `DURATION=2m`

## Headline finding — the bottleneck is NOT hardware
8× the cores of a laptop barely moved the lock throughput:

| Metric | Local (Mac) | **EC2 (32 vCPU)** |
|---|---|---|
| HTLC lock 50 TPS — achieved | 4.4 TPS | **2.9 TPS** (5,106 dropped iterations) |
| write latency p95 | 60s (timeout) | **60s (timeout)** |
| Full HTLC settlement lifecycle p95 (D6 ≤60s) | 30.6s | **32.9s** |
| Zeto escrow 15 TPS | PASS | **PASS** (100%, p95 5.6ms) |
| API read latency | PASS | PASS |

The lock path tops out at ~3 TPS and the settlement lifecycle stays ~31s regardless of
hardware. The limiter is the **synchronous, blocking Paladin/Zeto lock path** (latency-bound
ZKP UTXO state operations, ~5s each) — not CPU, not single-signer/nonce. An async-admission
redesign (return on submit, confirm via the existing event/relay machinery) is the lever.

## D6 HTLC settlement lifecycle (EC2) — all PASS
- Full lifecycle p95 **32.9s** (≤60s); API sync p95 **7.1s** (≤30s);
  relayer cross-chain p95 **3.8s** (≤15s); completion **100% (6/6)**.

## Files
- `RESULTS-2026-06-19T170108Z.md` — full `perf-all` run (component benchmarks; the
  happy-path here failed at the cross-spoke step — see operational note below).
- `RESULTS-2026-06-19T172948Z.md` — happy-path benchmark re-run, **all D6 gates PASS**.
- `artifacts/` — k6 `--summary-export` JSONs backing the reports.

## Operational notes (for reproducing on a fresh Linux VM)
1. The host needs **Go** in addition to Docker — the Paladin deploy helpers run `go` on
   the host (the run fails at `paladin.deploy-contracts-spoke-a` with `go: not found`).
2. The Cacti relay's spoke-a watcher can hang on a fresh bring-up
   (`[spoke-a] poll cycle error: connection not open on send()`), so FX proposals never
   mirror and the happy path stalls at "proposal mirrored to custodian". A
   `docker restart cbweb3-cacti-relay` reconnects it (then the happy-path passed 6/6).
   It is **not** a `host.docker.internal` issue (that resolves; bank-d is reachable).
