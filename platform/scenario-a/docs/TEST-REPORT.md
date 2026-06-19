# Scenario A — Test Report

Consolidated, measured test evidence for **Scenario A — Enhanced Correspondent Banking
(dual-layer HTLC)**. Generated 2026-06-19. Numbers below are from real runs on this
branch; the CI gates that enforce them are noted per section.

> Companion docs: [`tests/TEST-CATALOG.md`](../tests/TEST-CATALOG.md) (full test inventory),
> [`docs/test-execution-plan.md`](./test-execution-plan.md) (QA strategy), and the
> performance evidence under [`docs/performance/`](./performance/).

## Test layers

| Layer | Tooling | Scope | Where it runs |
|-------|---------|-------|---------------|
| Smart-contract unit | Foundry (`forge test`, 256 fuzz runs) | HTLC, FXAgreement, tCeBM/fCeBM, IdentityRegistry, … | CI: `contracts-scenario-a.yml` |
| Backend unit + coverage gate | `go test` + D6 80% core gate | payment-orchestrator, compliance, auth, shared/identity | CI: `backend-scenario-a.yml` |
| Integration (hermetic) | `go test -tags integration_lite` (bufconn + fakes) | cross-ledger HTLC atomicity, AML gate | CI (PR gate) + local |
| E2E (live stack) | `go test -tags integration` (`livehappy_*`) | full FX + cross-spoke HTLC settlement | local (`make scenario-a.test-integration`) |
| Performance (R1-12.3) | k6 | throughput, latency, D6 HTLC lifecycle | local + AWS EC2 |

## 1. Backend unit-test coverage (D6 80% core gate)

`make scenario-a.test-backend-coverage` — each module scopes `-coverpkg` to its core
business logic and fails below 80%.

| Module | Coverage | Gate | Result |
|--------|---------:|:----:|:------:|
| `payment-orchestrator` | 82.0% | 80% | ✅ PASS |
| `compliance` | 90.9% | 80% | ✅ PASS |
| `auth` | 85.6% | 80% | ✅ PASS |
| `shared/identity` | 80.5% | 80% | ✅ PASS |

All core backend modules meet the D6 gate. Enforced per-PR by `backend-scenario-a.yml`
(matrix per service) which also runs `go vet` and `gosec`.

## 2. Smart-contract coverage (Foundry)

`make contracts.coverage` (`FOUNDRY_FUZZ_RUNS=256`) — **183 tests, 0 failures.**

| Metric | Coverage | Gate |
|--------|---------:|:----:|
| Lines | 97.95% (287/293) | 80% |
| Statements | 97.89% (279/285) | 80% |
| Branches | 98.46% (64/65) | 80% |
| Functions | 100.00% (55/55) | 80% |

Well above the gate. Enforced by `contracts-scenario-a.yml` (fmt check → build → test →
coverage). _Local note: Foundry auto-loads `contracts/.env`; the dev `CENTRAL_BANK_ADDRESS`
in it makes the `FiatCentralBankMoney` deploy-script test assert against the wrong address.
CI is unaffected (`.env` is git-ignored). Run coverage with `contracts/.env` absent._

## 3. E2E / integration flows

**Hermetic (`integration_lite`)** — `scenario-a/tests/integration/` (bufconn + fakes, no
infra): cross-ledger two-leg HTLC happy path + refund (atomicity), and the AML/CFT
compliance gate. PR-gated.

**Live-stack E2E (`integration`)** — `make scenario-a.test-integration` (`TestFullHappyPath`)
drives the full correspondent-banking happy path end to end against `make spoke-all`:
bank-a (originator) → custodian bank-d → beneficiary bank-b.

Live run captured 2026-06-19 — **PASS (56.1s total)**:

| Phase | What it validates | Result |
|-------|-------------------|:------:|
| 0 Readiness | all 4 gateways healthy | ✅ |
| 1 Login | bank-a/b/d + cb-a/b authenticate | ✅ |
| 2 Onboard | PKI onboarding (opt-in `ONBOARD=1`) | ⏭️ SKIP |
| 3 Mint | central banks fund originator + custodian (treasury) | ✅ |
| 4 FX propose | bank-a proposes; persistence + audit | ✅ |
| 5 Cross-spoke sync | relay mirrors to bank-d → accept → ACCEPTED both spokes | ✅ |
| 6 HTLC lock | originator + custodian dual-leg lock; timelock invariant | ✅ |
| 7 Settle | secret reveal on spoke-a; relay settles spoke-b | ✅ |
| 8 Verify | **both legs SETTLED — atomic cross-spoke settlement** | ✅ |

## 4. Performance (R1-12.3) — summary

Full evidence in [`docs/performance/`](./performance/) (`RESULTS-2026-06-19T160230Z.md`
local; `ec2-run-20260619/` on AWS c6a.8xlarge).

- **D6 HTLC settlement lifecycle: PASS** — full lifecycle p95 ~31s (≤60s), API sync ~7s
  (≤30s), relayer cross-chain ~3.8s (≤15s), completion 100%.
- Zeto 15 TPS PASS; API read latency PASS.
- **HTLC lock 50 TPS: not met** (~3–4 TPS) — bounded by the synchronous Paladin/Zeto
  privacy lock path (latency-bound ZKP UTXO ops), confirmed hardware-independent on EC2.

## CI/CD enforcement map

| Workflow | Gates |
|----------|-------|
| `backend-scenario-a.yml` | per-service coverage (D6 80%), `go vet`, `gosec`, `integration_lite` |
| `contracts-scenario-a.yml` | `forge fmt --check`, build, test, coverage |

Live-stack E2E and performance are run out-of-CI (require the full ~30-container stack).
