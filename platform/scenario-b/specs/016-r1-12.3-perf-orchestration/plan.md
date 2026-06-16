# Implementation Plan — R1-12.3 Zero-Config Performance Orchestration

**Feature dir**: `016-r1-12.3-perf-orchestration`
**Scope**: Scenario B only.
**Goal**: A single command — `make scenario-b.perf-all` (run from `scenario-b/`) — that with **no config and no
manual steps** executes the full R1-12.3 performance suite and writes measured numbers + PASS/FAIL into
`docs/performance/RESULTS.md`. The 12-hour soak stays an explicit opt-in (`scenario-b.perf-soak`).

This is the orchestration + correctness layer on top of the already-committed k6 harness
(`tests/performance/scenario-b-perf.js`, `k6/bridge-transfer-throughput.js`, `k6/soak.js`).

---

## Problem statement (what "zero-config" must solve)

The existing perf make targets require the operator to (a) stand the stack up, (b) seed liquidity, (c) hand-mint an
`AUTH_TOKEN`, and (d) interpret raw k6 output by hand. Worse, the committed k6 scripts encode request payloads that
**do not match the real API contract** and would fail the <1% error gate on every iteration:

| Gap | Evidence in code | Consequence if unfixed |
|-----|------------------|------------------------|
| lock-mint expects 202 + `id` | `bridge_handler.go` `LockMint` returns **201 Created** with `position_id`; accepts only `{"amount"}` | every transfer iteration "fails" the `status===202` check → 100% error |
| lock-mint sends bogus fields | k6 sends `spoke/asset/token_kind/recipient/idempotency_key`; handler derives all server-side from JWT+config | fields ignored; `token_kind=zeto` is a no-op (no Zeto switch on this endpoint) |
| swap missing required fields | `swap_handler.go` requires `pair,amount_out,max_amount_in,payer_id,beneficiary_id`; k6 sends `recipient` | every swap → 400 INVALID_REQUEST |
| auth token | integration tests mint via gateway `POST /api/v1/auth/login` `{clientId,clientSecret}` → `{accessToken}` (Keycloak direct HTTP is blocked outside Docker) | operator can't get a token without reverse-engineering it |
| transfer-limit floods | `transfer_limit_checker.go`: a seeded daily limit makes sustained 50 TPS trip `TRANSFER_LIMIT_EXCEEDED` (422) | false 4xx flood fails <1% gate |
| shallow liquidity | pool seeded at 1000 W-BRL / 287000 W-ARS via commit-reveal; deep 30 TPS swaps move price past `max_amount_in` | swap reverts (price impact) |
| circuit breaker | constitution III: breaker MUST be validated before swaps | swaps revert if paused |
| TTF | threshold 4 is prose only | no measured TTF p50/p95 |
| evidence/results | `--summary-export` not wired; `RESULTS-TEMPLATE.md` blank | numbers not machine-captured |

## Approach

A `tests/performance/lib/` directory of small, single-responsibility POSIX-sh helpers, orchestrated by a driver
script `tests/performance/run-all.sh`, surfaced as `make scenario-b.perf-all`.

```
tests/performance/lib/
  log.sh       structured JSON logging to stdout (constitution VI) + helpers
  stack.sh     detect a live stack at API_GW_URL; bring it up (scenario-b.up) only if absent
  auth.sh      mint commercial-bank + central-bank JWTs via gateway /api/v1/auth/login (no Keycloak direct)
  profile.sh   set the perf profile: raise/clear transfer limits (R1-10.1) + ensure circuit breaker RESUMED
  seed.sh      ensure the AMM pair has 30-TPS-sized depth; confirm pool ACTIVE
  ttf.sh       post-process PRINT_IDS=1 position ids → correlate with on-chain Minted/Released events → p50/p95
  metrics.sh   out-of-band soak evidence: container RSS+goroutines, pg_stat_activity, node/relay restart, x*y=k
run-all.sh     driver: stack → auth → profile → seed → baseline → amm → transfer → zeto → TTF → write RESULTS.md
```

Design choices:
- **Reuse, don't rebuild.** The k6 scripts stay the load engine. We fix their payloads to the real contract and add
  `--summary-export` JSON capture, but the scenarios/thresholds are unchanged.
- **Gateway login, not Keycloak.** `auth.sh` mirrors `tests/integration` exactly: `POST {gw}/api/v1/auth/login`.
  Credentials (`KC_CLIENT_SECRET`) are read from `backend/config/.env.infra.*` with the documented local fallbacks.
- **Idempotent profile.** `profile.sh` deletes any existing transfer limits for the perf currencies (so
  `FindApplicableLimit` returns nil → unlimited) and asserts circuit-breaker status is not PAUSED, resuming if needed.
- **Liquidity sizing as a knob, not a fork.** `seed.sh` checks pool status; if reserves are below a 30-TPS depth
  floor it tops up via the existing cooperative commit/reveal API path (same endpoints the integration test uses).
- **TTF without new infra.** `ttf.sh` reads relay (`interop/hub-and-spoke/cacti`) logs / Besu events for the
  `Minted`/`Released` event matching each emitted `position_id`; computes p50/p95 in awk. No new runtime dependency.
- **Results are generated.** `run-all.sh` reads each k6 summary-export JSON and writes a filled `RESULTS.md` with
  measured p95s, throughput, error rate, TTF, and PASS/FAIL per threshold — and explicitly marks the AMM 30 TPS
  DRAFT as VALIDATED or REVISED based on the measured swap rate/error.

## Constitution Check

- **I. Scenario-Scoped Independence** — All work is under `scenario-b/`. No `scenario-a/` paths are read or written.
  No cross-scenario sharing. PASS.
- **II. Privacy by Design** — The harness drives the existing privacy paths through the API gateway; it introduces no
  plaintext PII/amounts on-chain and no new token type. The "Zeto" threshold is exercised via the real mirrored-asset
  path (corrected — see Complexity Tracking re: the no-op `token_kind`). PASS.
- **III. Atomic Settlement Guarantee** — `profile.sh` validates the circuit breaker is RESUMED **before** any swap
  load runs, per the explicit "breaker validated before swaps" rule. The harness never introduces partial-settlement
  paths; TTF correlation observes the relay's lock→mint / burn→unlock events. PASS.
- **IV. Compliance Gate Before Participation** — Tokens are obtained through the gateway's real auth path
  (`/api/v1/auth/login`); no gateway/compliance bypass. Swap/transfer load carries a valid bearer token with the
  required role, exactly as a real client would. PASS.
- **V. Test-First at Every Layer** — Performance is the layer here (constitution V explicitly lists it). A shellcheck
  + dry-run smoke test for the helpers is added; the k6 payload corrections are covered by the existing handler unit
  tests (the contract we align to). PASS.
- **VI. Observability and Auditability** — All helper scripts emit structured JSON logs to stdout (ts ISO-8601,
  service, severity, msg). No silent failures: every step logs success/failure and the driver aborts loudly. PASS.

## Complexity Tracking

| Decision | Why | Rejected alternative |
|----------|-----|----------------------|
| Fix k6 payloads to real contract rather than change handlers | Handlers are the shipped contract with passing unit tests; the scripts were wrong | Changing handlers to accept the bogus fields (would weaken/duplicate the API) |
| `token_kind=zeto` parameter kept but documented as the mirrored-asset privacy path | lock-mint has no per-request domain switch today; the 15-TPS gate still measures the privacy-mirrored transfer path end-to-end through the gateway | Adding a new per-request Zeto switch to lock-mint (out of R1-12.3 scope; would be a feature change) |
| Shell helpers + k6, no Caliper/Prometheus dependency | Stays inside the fixed stack; README §6 already justifies k6-only | New runtime deps (Caliper/Prometheus) — rejected per stack rules |
| `RESULTS.md` generated (template stays as the blank reference) | Zero-config means numbers land automatically | Hand-filling `RESULTS-TEMPLATE.md` (the status quo we're removing) |

## Phases

1. **Fix the k6 contract bugs** (payloads/status/fields) so a real run can pass the error gate.
2. **lib/ helpers** — log, stack, auth, profile, seed (test-first smoke via shellcheck + dry-run).
3. **TTF + metrics** post-processors.
4. **Driver + make targets** (`perf-all` zero-config; `perf-soak` opt-in retained).
5. **Results generation + docs** — `RESULTS.md` writer; update `docs/performance/README.md` for the one-command flow.

## Out of scope / deferred to a live run

The actual measured numbers require real devnet infra (Besu QBFT networks + relay + backends). This worktree wires the
turnkey command and verifies it statically (shellcheck, dry-run, k6 `--no-thresholds` parse). The numbers populate
`RESULTS.md` automatically the first time `make scenario-b.perf-all` runs against a live stack.
