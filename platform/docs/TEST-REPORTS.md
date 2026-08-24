# cbweb3-platform — Test Reports

Top-level index of measured test evidence across both scenarios. Generated 2026-06-19.
Each scenario has a detailed report; this page is the at-a-glance summary and the entry
point for the project test documentation.

- **Scenario A — Enhanced Correspondent Banking (dual-layer HTLC):**
  [`scenario-a/docs/TEST-REPORT.md`](../scenario-a/docs/TEST-REPORT.md)
- **Scenario B — International Hub (FXAgreement + AMM + relay):**
  [`scenario-b/docs/TEST-REPORT.md`](../scenario-b/docs/TEST-REPORT.md)

## Coverage at a glance

### Backend unit-test coverage — D6 80% core gate (all PASS)

| Module | Scenario A | Scenario B |
|--------|:----------:|:----------:|
| api-gateway | — | 80.0% |
| auth | 85.6% | 93.2% |
| compliance | 90.9% | 88.8% |
| payment-orchestrator | 82.0% | 83.5% |
| shared/identity | 80.5% | 88.2% |

### Smart-contract coverage — Foundry (gate 80%, all PASS)

| Metric | Scenario A (183 tests) | Scenario B (286 tests) |
|--------|:----------------------:|:----------------------:|
| Lines | 97.95% | 98.12% |
| Branches | 98.46% | 92.36% |
| Functions | 100% | 100% |

## E2E / performance status

| | Scenario A | Scenario B |
|---|---|---|
| Hermetic integration (`integration_lite`, PR gate) | ✅ | ✅ |
| Live-stack E2E happy path | ✅ PASS (8 phases, 56s) | ✅ PASS (8 phases, 63s) |
| Performance (R1-12.3) | D6 HTLC lifecycle PASS; lock 50 TPS not met (Paladin/Zeto-bound) | transfer/Zeto PASS; AMM 30 TPS REVISE; error-rate FAIL |

Performance evidence: [`scenario-a/docs/performance/`](../scenario-a/docs/performance/)
and [`scenario-b/docs/performance/RESULTS.md`](../scenario-b/docs/performance/RESULTS.md).

## How the gates are enforced (CI)

| Workflow | Enforces |
|----------|----------|
| `backend-scenario-{a,b}.yml` | per-service coverage (D6 80%), `go vet`, `gosec`, `integration_lite` |
| `contracts-scenario-{a,b}.yml` | `forge fmt --check`, build, test, coverage |

Live-stack E2E and performance run out-of-CI (they need the full multi-container stacks).

## Reproduce locally

```bash
# Backend coverage (hermetic, ~minutes)
cd scenario-a && make scenario-a.test-backend-coverage   # or scenario-b/...

# Contract coverage (Foundry, 256 fuzz)
make contracts.coverage
# Note (scenario-a): run with contracts/.env absent — Foundry auto-loads it and the dev
# CENTRAL_BANK_ADDRESS breaks a deploy-script test locally (CI is unaffected; .env is git-ignored).

# Live-stack E2E happy path (needs an already-running stack: since the legacy
# deploy/local path was retired on 2026-08-21, no make target brings one up — and the
# test still names the retired topology, see docs/deliverables/D12-defect-log.md DEF-007)
make scenario-a.test-integration            # scenario-b: make scenario-b.test-integration

# Performance suite
make scenario-a.perf-all                    # scenario-b: make scenario-b.perf-all
```

## Execution record (D12)

Measured results answer *what the tests found*. These three answer *what was run, when, and
what is still open*:

| Document | Holds |
|----------|-------|
| [`deliverables/D12-timeline-actuals.md`](deliverables/D12-timeline-actuals.md) | Planned vs actual dates per phase, and the nine recorded deviations |
| [`deliverables/D12-defect-log.md`](deliverables/D12-defect-log.md) | The defect register — format, severity/SLA, lifecycle, and the 14 defects the executed phases produced |
| [`deliverables/D12-uat-records.md`](deliverables/D12-uat-records.md) | Phase 5 UAT record and sign-off formats, plus the preconditions still blocking UAT |

Phases 1–4 have executed; Phase 5 (bank-led UAT) has not, and there are no UAT records on
file. Six Phase 4 threshold findings remain open.

## Next pass
- Side-by-side perf deep-dive (the two throughput ceilings: A = Paladin/Zeto blocking lock
  path; B = single-signer EVM nonce — different root causes, different fixes).
