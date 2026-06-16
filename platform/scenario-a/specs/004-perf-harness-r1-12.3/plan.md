# Implementation Plan: R1-12.3 Zero-Config Performance Harness (Scenario A)

**Branch**: `test/r1-12.3-perf-harness` | **Date**: 2026-06-16
**Input**: Report 1 — Deliverable 12, Finding 12.3 (P0); `scenario-a/docs/performance/README.md`

## Summary

Report 1 / Finding 12.3 restated performance thresholds without measuring them. The k6 scripts
and per-threshold make targets already exist but require a human to (a) stand up the stack, (b)
mint and pass an `AUTH_TOKEN`, (c) ensure identities are funded, (d) run each benchmark, (e)
post-process TTF by hand, and (f) transcribe numbers into the results doc.

This plan closes the orchestration gap: a single zero-config command —
`make scenario-a.perf-all` — that stands up the stack if needed, mints the auth token, runs the
threshold benchmarks (baseline + 50 TPS transfer + 15 TPS Zeto + on-chain TTF correlation),
captures machine-readable evidence, and writes measured numbers + PASS/FAIL into a generated
results file. The 12h soak stays opt-in (`scenario-a.perf-soak-all`).

No k6 scripts are rewritten — they are reused as-is. All new logic is a thin orchestration layer
(POSIX shell + jq) under `scenario-a/tests/performance/`.

## Technical Context

**Language/Version**: `bash` + `jq` (orchestration); k6 (already a dep) for load.
**Primary Dependencies**: k6, curl, jq — all already used by tryouts/perf. No new runtime deps.
**Storage**: results written to `docs/performance/RESULTS-<UTC>.md`; evidence JSON under
`tests/performance/.artifacts/<run-id>/`.
**Testing**: shell helpers are linted (`bash -n`) and exercised by a dry-run mode that needs no
infra; k6 scripts keep their built-in threshold gates.
**Target Platform**: local devnet (`make spoke-a` / `spoke-all`), Docker Compose, Besu QBFT.
**Performance Goals (the gates under test)**: 50 TPS HTLC transfer; 15 TPS Zeto escrow;
per-spoke TTF p95 < 5s; read p95 < 500ms; write p95 < 1500ms; error < 1%; 12h soak stability.
**Constraints**: zero-config (no args); scenario-a only; minimal new deps; structured output.
**Scale/Scope**: ~30k HTLC locks over a 10m × 50 TPS run; funding must not exhaust.

### Key facts established during research

- Auth: `POST {gw}/api/v1/auth/login` with `{clientId, clientSecret}` returns
  `{accessToken,...}` in the JSON body AND sets the `access_token` cookie
  (`api-gateway/internal/http/handlers/auth.go:169`). The k6 scripts want the raw JWT in
  `AUTH_TOKEN`, which they set as the `access_token` cookie. So: log in, read `.accessToken`.
- Credentials come from `backend/config/.env.infra.<entity>` (`KC_CLIENT_ID`,
  `KC_CLIENT_SECRET`), exactly as the tryout scripts do.
- Bank-A API gateway is exposed on host port **18080** (`deploy/local/README.md`), not 3001.
  Perf scripts default to 3001; the driver sets `API_GW_URL=http://localhost:18080` explicitly.
- Spoke-A bank-a Besu RPC is on host port **8646** (`spoke-besu-a/startBesu.sh`), chain id 1338.
- HTLC coordination contract address is synced into `.env.infra.bank-a` as `HTLC_ADDRESS`.
- TTF: `LogHTLCLocked(bytes32 indexed contractId, address indexed sender, address indexed
  receiver, bytes32 hashLock, uint256 timeLock, bytes32 zetoLockRef)`
  (`contracts/src/interfaces/IHashTimeLockedContract.sol`). topic1 == `contractId` returned by
  the lock API -> correlate via `eth_getLogs` and read the block timestamp for t1.
- Funding: the escrow tryout (`tryouts/tryout-escrow-flow.sh`) seeds fCeBM via deposit ->
  approve -> fiat-exchange (CB governance). The driver reuses that flow to fund the sender before
  the throughput runs so funding exhaustion doesn't trip the <1% error gate.

## Constitution Check

*GATE: re-checked after design. Result: PASS.*

- **I. Scenario-Scoped Independence** — All new files live under `scenario-a/`. Nothing references
  `scenario-b/`. No shared library introduced. PASS.
- **II. Privacy by Design** — Harness only drives existing endpoints (HTLC lock, escrow, search,
  FX list). It logs `contract_id`s (already non-PII identifiers) and on-chain event topics. No
  plaintext PII or amounts are emitted beyond the synthetic test amounts the scripts already use.
  PASS.
- **III. Atomic Settlement Guarantee** — Read-only with respect to settlement logic; the harness
  exercises the existing lock/escrow paths and does not introduce partial-settlement code. TTF
  correlation reads `LogHTLCLocked` events only. PASS.
- **IV. Compliance gate** — Auth helper logs in through the real gateway `/auth/login` (Keycloak
  OIDC), i.e. it uses the production compliance path; it does not bypass any check. PASS.
- **V. Test-first, every layer** — k6 scripts carry their own threshold gates (failing-by-default
  until the SUT meets them). The driver fails the run when any gate is breached. A no-infra
  dry-run validates the orchestration itself. PASS.
- **VI. Observability** — Driver emits structured JSON log lines (ts, level, msg, fields) to
  stdout, exports k6 `--summary-export` JSON, and persists all evidence under a per-run artifact
  dir referenced from the results doc. PASS.
- **Stack discipline** — No new runtime dependency: k6, curl, jq are already required by the
  existing perf docs and tryout scripts. PASS.

## Project Structure

```text
scenario-a/
  specs/004-perf-harness-r1-12.3/plan.md          # this file
  tests/performance/
    lib/
      log.sh            # structured JSON logging + run-id/artifact helpers
      auth.sh           # mint AUTH_TOKEN from .env.infra via /auth/login
      stack.sh          # detect / stand up the stack (spoke-a) if needed
      fund.sh           # seed+fund sender identity (deposit->approve->fiat-exchange)
      ttf.sh            # eth_getLogs correlation of contract_id -> LogHTLCLocked -> TTF p50/p95
      results.sh        # write RESULTS-<UTC>.md with measured numbers + PASS/FAIL
    run-all.sh          # the zero-config orchestrator (driver)
    .artifacts/         # per-run evidence (gitignored)
  make/20-tests.mk      # + scenario-a.perf-all, scenario-a.perf-soak-all
  docs/performance/README.md   # documents the one-command flow
```

## Phasing

1. **lib helpers** (log, auth, stack-detect, fund, ttf, results) — each independently sourceable
   and dry-run safe.
2. **run-all.sh** orchestrator wiring the phases with fail-fast + evidence capture.
3. **make targets** `scenario-a.perf-all` (zero-config) and `scenario-a.perf-soak-all` (opt-in).
4. **docs** update + generated results file wiring.
5. **validation**: `bash -n` on all scripts, `--dry-run` of the driver (no infra), commit.

## Complexity Tracking

- TTF correlation needs host RPC access (port 8646) + the `HTLC_ADDRESS`. Rejected alternative:
  parsing Besu container logs (brittle, log-format-dependent). Chosen: `eth_getLogs` by topic,
  which is stable and gives an authoritative block timestamp. Justified deviation: TTF cannot be
  measured black-box over HTTP (the README already states this).
- Driver is shell, not Go: it is glue over existing CLIs (make, k6, curl, jq); a Go binary would
  add a build step and a module with no reuse value. Shell keeps it zero-build and matches the
  tryout-script convention already in the repo.
