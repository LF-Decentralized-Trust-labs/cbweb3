# P0 Remediation Plan — CBWeb3 LNet Review

19 P0 tasks ordered by structural dependency, security priority, and complexity.
Complexity ratings: **XS** (0.5–1d) · **S** (1–4d) · **M** (3–8d) · **L** (7–16d)

---

## Dependency Graph

```
TASK-01 (Hub Network)
  ├─► TASK-17  (bridge burn verifies AMM event on hub chain)
  ├─► TASK-12/13  (AMM contracts live on the hub chain)
  ├─► TASK-07  (Scenario B E2E needs a working hub)
  ├─► TASK-09  (AMM performance tests)
  └─► TASK-10  (staging env + commercial-swap E2E gaps)

TASK-05 (CI fixed)
  └─► TASK-08  (coverage gate needs CI to run)

TASK-18 (HTLC Postgres)
  ├─► TASK-03  (audit trail requires persistent HTLC state)
  └─► TASK-07  (HTLC records survive restarts during test runs)

TASK-12 + TASK-13 (AMM corrected)
  ├─► TASK-07  (Scenario B AMM E2E invalid against broken LP accounting)
  └─► TASK-09  (AMM performance meaningless against broken AMM)

TASK-03 (Audit pipeline wired)
  └─► TASK-19  (Supervisor portal needs real backend)
      └─► TASK-06  (Supervisor manual must document real system, not mock)

TASK-14 + TASK-15 + TASK-16 (Security patches)
  ├─► prerequisite to sharing any environment externally
  └─► TASK-11  (don't publish a spec that documents vulnerable endpoints)
```

**Key structural facts confirmed from code:**

- `HUB_CHAIN_ID` is hardcoded to `1338` (Spoke-A) across all 4 Scenario B docker-compose files,
  4 `.env` examples, `payment-orchestrator/cmd/main.go:205`, and `api-gateway/app.go:234`.
  TASK-01 is systemic, not cosmetic.
- `GetAuditLogs` route and handler exist but the compliance adapter is a stub. TASK-03 has a
  skeleton to build on.
- Only `fx_agreement_gorm.go` exists as a real GORM repository; HTLC and escrow are both
  in-memory. TASK-18 scope is clear and has a reference pattern.

---

## Wave 0 — Security Patches

> Do immediately. Nothing else can safely proceed until these are closed.
> All three are parallel-safe and can be completed in 2–4 days by one engineer.

| # | Task | Complexity | Effort | What to do |
|---|------|:---:|:---:|---|
| 1 | **TASK-14** · Delete PKI signing oracle | XS | 0.5–1d | Delete `auth.go:359–461` (`POST /auth/resolve-challenge`), remove its route registration, and remove the `PKI_DIR` private-key read logic. The code itself is annotated "MVP-only; remove before production." |
| 2 | **TASK-16** · Authenticate relay event routes + restrict CORS | S | 1–2d | Add constant-time `X-Relay-Auth` check (matching the pattern used on FX endpoints) to `GET /relay/events/settle` at `index.ts:134–137`; change `cors: "*"` at `:241` to a restrictive origin list. |
| 3 | **TASK-15** · Remove HTLC preimage from API responses | S | 2–3d | Strip the `Secret` field from `recordToProto` at `server.go:1119`; add counterparty ownership check on HTLC read routes at `router.go:125–126`. |

> ⚠️ TASK-14, TASK-15, TASK-16 are security-sensitive. Do not discuss exploit details in public issue trackers.

---

## Wave 1 — Infrastructure Foundation

> Unlocks all subsequent scenario work. TASK-01 and TASK-05 are parallel-safe.

| # | Task | Complexity | Effort | What to do |
|---|------|:---:|:---:|---|
| 4 | **TASK-01** · Dedicated hub network (chain 1337) | L | 8–14d | Every Scenario B service reads `HUB_CHAIN_ID=1338` at startup. Create a dedicated Besu genesis + nodes + docker-compose for chain 1337; update all docker-compose and `.env` defaults; write the deployment runbook; coordinate parameters with LNet. TASK-12/13/17/07/09/10 cannot be fully validated until the hub chain is independent. |
| 5 | **TASK-05** · Fix CI/CD pipelines + green runs | S | 1–3d | Move `backend/backend.yml` and `contracts/contracts.yml` from their subdirectories directly into `.github/workflows/` (GitHub only discovers workflows at that level). Add `develop` to both `push` and `pull_request` branch triggers. Demonstrate green runs covering `vet`, tests, coverage gate, `gosec`, `forge fmt/build/test`. |

---

## Wave 2 — Core Correctness

> Requires the Wave 1 environment to be stable. TASK-12/13 and TASK-18 are parallel-safe;
> TASK-17 can begin once TASK-01 (hub chain) is available.

| # | Task | Complexity | Effort | What to do |
|---|------|:---:|:---:|---|
| 6 | **TASK-12 + TASK-13** *(co-deliver)* · AMM LP-share accounting + empty-pool protection | L | 8–12d | **TASK-12:** `addLiquidity` (`AutomatedMarketMaker.sol:174`) mints no LP tokens; `removeLiquidity` (`:252`) is gated only by `amountA <= reserveA`, allowing any whitelisted account to drain the full pool. Implement proportional LP-share minting, per-caller withdrawal guard, and minimum-liquidity lock. **TASK-13:** `getAmountIn` (`:273`) does not guard against `reserveIn == 0`; no minimum-liquidity burn on first deposit. Add `require(reserveA > 0 && reserveB > 0)` before swaps; burn minimum liquidity on first deposit; enforce ratio-consistent deposits. Deliver as a single AMM security sprint. |
| 7 | **TASK-18** · Persist HTLC/escrow state to Postgres | M | 5–8d | `server.go:88` stores HTLCs in a `map[string]*domain.HTLCRecord`; `main.go:167` uses `NewMemoryEscrowRepository()`. A service restart loses all in-flight settlement state. Mirror the existing `fx_agreement_gorm.go` pattern: create GORM repositories for HTLC and escrow, add DB migrations, and reconcile in-flight state from chain on startup. |
| 8 | **TASK-17** · Verify bridge burn on-chain + replay protection | L | 7–12d | `cross_currency_bridge_out_handler.go:71–138` burns wrapped tokens and mints native tCeBM based purely on received JSON — `swap_tx_hash` is never verified on-chain, there is no idempotency check, and all `.env.example` files ship `INTERNAL_RELAY_AUTH_SECRET=cbweb3-relay-shared-secret`. Verify the swap transaction on the hub (decode the AMM event; assert amount/sender/pair) before any burn/mint; persist and enforce a `correlation_id` idempotency key; replace the shared secret with per-CB asymmetric authentication. Requires TASK-01 (real hub chain to query). |

---

## Wave 3 — Compliance + Governance Wiring

> TASK-02, TASK-03, TASK-04, and TASK-08 are mostly parallel-safe.
> TASK-03 has a hard dependency on TASK-18.

| # | Task | Complexity | Effort | What to do |
|---|------|:---:|:---:|---|
| 9 | **TASK-02** · Configurable CB transfer limits | M | 4–7d | No per-participant or per-currency transfer limits exist anywhere in the contracts. Add storage mappings, admin setter functions, and modifier guards across AMM and FX contracts. Include in E2E evidence (TASK-07). |
| 10 | **TASK-03** · Audit pipeline + supervisor permissions end-to-end | M | 4–7d | The `GET /governance/audit/logs` route and handler exist; the compliance adapter is a stub. Requires TASK-18 (persistent HTLC/escrow state to audit). Wire the compliance adapter to real Postgres-backed event storage; configure Keycloak Supervisor role with query permissions; validate end-to-end. |
| 11 | **TASK-04** · Open-source license + SPDX headers | M | 3–6d | No `LICENSE` file exists. All 59 Solidity files declare `SPDX-License-Identifier: UNLICENSED`; 391 Go files have no SPDX headers. Add a root `Apache-2.0` `LICENSE` file; correct all Solidity SPDX identifiers; add headers to Go and TS sources; add `CONTRIBUTING.md`; generate a dependency-license report; produce a DPG-standard compliance checklist. |
| 12 | **TASK-08** · Real coverage reports + CI gate | S | 2–4d | Requires TASK-05 (CI runs first). Run `forge` and Go coverage, attach LCOV/`coverage.out` artifacts, add a coverage badge, and wire the existing 90% threshold gate (`validate-coverage.sh`) into the working CI pipeline. |

---

## Wave 4 — Feature Completion + Spec Accuracy

> Requires Wave 2 correctness and Wave 3 compliance. TASK-10 and TASK-11 are parallel-safe;
> TASK-19 has a hard dependency on TASK-03.

| # | Task | Complexity | Effort | What to do |
|---|------|:---:|:---:|---|
| 13 | **TASK-10** · Close 5 HIGH coverage gaps | L | 8–14d | Five open HIGH-priority gaps: (1) AMM reserve overflow — add overflow guards post TASK-12/13; (2) FXAgreement concurrent settlement idempotency; (3) Cacti `CommitMatched` event detection; (4) commercial-bank swap E2E — submit evidence that the non-governance swap path works; (5) staging environment — requires TASK-01 hub. Close each gap or provide a dated remediation plan. |
| 14 | **TASK-11** · D5 spec update to v2.3.0 + bundled Swagger | M | 3–6d | D5 describes API v1; the delivered gateway is v2.3.0 (75 operations). The Swagger UI at `/docs` is CDN-dependent (fails offline). The Scenario B spec is fragmented and out of sync (served v2.2.0/57 ops vs. documented v2.3.0). Deliver an updated D5 document covering the full v2 surface; a v1→v2 changelog listing breaking changes; and a single self-contained (bundled, no CDN) Swagger spec per scenario. Complete Wave 0 security patches before publishing the spec. |
| 15 | **TASK-19** · Wire Supervisor portal to real auth + backend | S | 2–4d | `mock-db.ts:110–111` accepts any password ≥ 6 characters while the UI displays "SSO session established." Requires TASK-03 (real audit backend). Replace mock auth with Keycloak/API Gateway SSO integration; gate the portal behind a non-production flag until complete; correct the UI copy. |

---

## Wave 5 — Evidence, Validation, and Documentation

> Final wave. Documents and validates the actual shipped product.
> All three tasks benefit from everything in Waves 1–4 being stable.

| # | Task | Complexity | Effort | What to do |
|---|------|:---:|:---:|---|
| 16 | **TASK-07** · E2E test evidence bundles | L | 10–16d | All test directories contain only `.gitkeep`. Requires: hub (T01), corrected AMM (T12/13), bridge verification (T17), HTLC persistence (T18), transfer limits (T02), and green CI (T05). Write and execute 8 Scenario A + 10 Scenario B scripts; capture JSON evidence bundles (tx hashes, block numbers, latencies per step) per the D12 format for every claimed E2E scenario. |
| 17 | **TASK-09** · Measured performance results against all thresholds | L | 8–14d | No k6 or Caliper scripts exist. Requires corrected AMM (T12/13) and working hub (T01) — performance against a broken LP invariant is invalid. Design and implement k6 load harness; run against all thresholds (50 TPS transfers, 15 TPS Zeto, TTF < 5s, p95 latencies, <1% error rate, 12-hour soak); validate or formally revise the AMM draft 30 TPS threshold. |
| 18 | **TASK-06** · Complete portal user manuals (5 portals × both scenarios) | L | 10–16d | Scenario A is missing the Governance portal manual. Scenario B has no portal manuals at all (bank, NOC, supervisor, treasury, governance). The Supervisor manual documents unimplemented modules (mock data). Requires TASK-19 (real Supervisor auth) before the Supervisor manual can be accurate. Deliver all 5 portals per scenario at the same quality bar as the existing Bank Portal guide: end-user tone, screenshots, status reference, troubleshooting section. |

---

## Summary

```
Wave 0 │ T14 · T15 · T16              security patches        2–4d   (parallel)
Wave 1 │ T01 · T05                    infra foundation        8–15d  (parallel)
Wave 2 │ T12+13 · T18 · T17          core correctness        15–30d (partial parallel)
Wave 3 │ T02 · T03 · T04 · T08       compliance + wiring     12–24d (mostly parallel)
Wave 4 │ T10 · T11 · T19             feature completion      13–24d (mostly parallel)
Wave 5 │ T07 · T09 · T06             evidence + docs         28–46d (sequential)
```

| Metric | Value |
|---|---|
| Total tasks | 19 |
| XS | 1 (T14) |
| S | 5 (T05, T08, T15, T16, T19) |
| M | 6 (T02, T03, T04, T11, T13, T18) |
| L | 7 (T01, T06, T07, T09, T10, T12, T17) |
| Sequential total | ~90–155 person-days |
| Critical path (3–4 engineers) | ~35–55 days |

**Critical path:** T01 → T12/13 → T07/T09.
The hub network is the structural spine — nothing in Scenario B can be credibly validated until it exists as a dedicated chain.
