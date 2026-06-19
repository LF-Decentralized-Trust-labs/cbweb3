# Scenario B — Test Report

Consolidated, measured test evidence for **Scenario B — International Hub
(FXAgreement + AMM + LiquidityCommitRegistry, with relay + circuit breaker)**.
Generated 2026-06-19. Numbers below are from real runs on this branch; the CI gates
that enforce them are noted per section.

> Companion docs: [`tests/TEST-CATALOG.md`](../tests/TEST-CATALOG.md) (full test inventory),
> [`docs/test-execution-plan.md`](./test-execution-plan.md) (QA strategy), and the
> performance evidence in [`docs/performance/RESULTS.md`](./performance/RESULTS.md).

## Test layers

| Layer | Tooling | Scope | Where it runs |
|-------|---------|-------|---------------|
| Smart-contract unit | Foundry (`forge test`, 256 fuzz runs) | FXAgreement, AMM, LiquidityCommitRegistry, HTLC, tCeBM/Zeto/Noto, IdentityRegistry, … | CI: `contracts-scenario-b.yml` |
| Backend unit + coverage gate | `go test` + D6 80% core gate | api-gateway, auth, compliance, payment-orchestrator, shared/identity | CI: `backend-scenario-b.yml` |
| Integration (hermetic) | `go test -tags integration_lite` | orchestrator, compliance-gate | CI (PR gate) + local |
| E2E (live stack) | `go test -tags integration` (`happy_path_test.go`) | full cross-currency swap (bridge-in → AMM → bridge-out) | local (`make scenario-b.test-integration`) |
| Performance (R1-12.3) | k6 | AMM/transfer/Zeto throughput, latency, cross-currency SLA | local |

## 1. Backend unit-test coverage (D6 80% core gate)

`make scenario-b.test-backend-coverage` — each module scopes `-coverpkg` to its core
business logic and fails below 80%.

| Module | Coverage | Gate | Result |
|--------|---------:|:----:|:------:|
| `api-gateway` | 80.0% | 80% | ✅ PASS |
| `auth` | 93.2% | 80% | ✅ PASS |
| `compliance` | 88.8% | 80% | ✅ PASS |
| `payment-orchestrator` | 83.5% | 80% | ✅ PASS |
| `shared/identity` | 88.2% | 80% | ✅ PASS |

All core backend modules meet the D6 gate. Enforced per-PR by `backend-scenario-b.yml`
(matrix per service) which also runs `go vet`, `gosec`, and the `integration_lite` tier.

## 2. Smart-contract coverage (Foundry)

`make contracts.coverage` (`FOUNDRY_FUZZ_RUNS=256`) — **286 tests passed, 0 failed, 1 skipped.**

| Metric | Coverage | Gate |
|--------|---------:|:----:|
| Lines | 98.12% (626/638) | 80% |
| Statements | 97.22% (700/720) | 80% |
| Branches | 92.36% (133/144) | 80% |
| Functions | 100.00% (95/95) | 80% |

Well above the gate. Enforced by `contracts-scenario-b.yml` (fmt check → build → test →
coverage).

## 3. E2E / integration flows

**Hermetic (`integration_lite`)** — orchestrator + compliance-gate suites (bufconn +
fakes, no infra). PR-gated.

**Live-stack E2E (`integration`)** — `make scenario-b.test-integration` (`TestFullHappyPath`)
drives the full hub happy path against `make scenario-b.up`.

Live run captured 2026-06-19 — **PASS (63.1s total)**:

| Phase | What it validates | Result |
|-------|-------------------|:------:|
| 0 Readiness | gateways healthy | ✅ |
| 1 Liquidity provision | dual-sided commit-reveal → pool ACTIVE | ✅ |
| 2 Onboarding | PKI onboarding (payer + beneficiary) | ✅ |
| 3 Fiat issuance | deposit → CB approve → fCeBM minted | ✅ |
| 3b Reserve tokenisation | fCeBM → tCeBM escrow | ✅ |
| 4 Cross-currency transfer | **bridge-in → AMM swap → bridge-out** | ✅ |
| 5 Bank-B receipt | beneficiary balance increased | ✅ |
| 6 LP withdrawal | partial zap-out (position stays ACTIVE) | ✅ |

## 4. Performance (R1-12.3) — summary

Full evidence in [`docs/performance/RESULTS.md`](./performance/RESULTS.md)
(run `20260618T162301Z`).

- Value-transfer 50 TPS **PASS**; Zeto 15 TPS **PASS**; quote/swap/pool latency **PASS**.
- AMM swap 30 TPS (hub-only): **REVISE** — ~88.9 TPS but 33% error when over-driven; the
  doc attributes the ceiling to single-signer EVM nonce serialisation (fix: multi-key signing).
- Cross-currency end-to-end (3c): **CHARACTERIZED** (~9.4 TPS, p95 60s — bridge/nonce-bound).
- TTF: **UNKNOWN** (n=0); steady-state error rate: **FAIL** (2.37%).

## CI/CD enforcement map

| Workflow | Gates |
|----------|-------|
| `backend-scenario-b.yml` | per-service coverage (D6 80%), `go vet`, `gosec`, `integration_lite` |
| `contracts-scenario-b.yml` | `forge fmt --check`, build, test, coverage |

Live-stack E2E and performance are run out-of-CI (require the full multi-network stack).
