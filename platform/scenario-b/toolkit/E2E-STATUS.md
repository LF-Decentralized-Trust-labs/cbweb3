# Scenario B Toolkit — E2E Status

> Status of the `cbweb3b` provisioning toolkit (`found-hub` / `found-spoke` /
> `join` + the soft sovereign-pair tail) as exercised end-to-end against real
> Docker + Foundry + Besu. Updated as the E2E is built out (TK-B10).

## Summary

The toolkit provisions a Scenario B CBDC hub-and-spoke topology end-to-end:

- **`found-hub`** — a neutral operator founds the hub (Besu + base contracts +
  relay + NOC + Keycloak).
- **`found-spoke`** — a central bank founds its sovereign spoke (CB as sole QBFT
  validator), registers on the hub, and — when the manifest carries `spec.pair` —
  runs the soft sovereign tail (open-sovereign-pair / commit-liquidity /
  seed-oracle).
- **`join`** — a commercial bank joins its spoke as a non-validating full node
  and generates its CSR (the only PKI step; signing/registration are runtime).

All modes run idempotently (re-running `apply` converges: done steps are skipped).
Every E2E test **skips with a warning** when the environment (Docker/Foundry/Besu/
relay) is absent — never a false green.

## Topology covered

hub + 2 sovereign spokes (CB-A / CB-B) + 1 commercial bank + 1 sovereign pair
(`PROPOSED`→`ACTIVE`) with cooperative liquidity on both sides.

## E2E steps (`TestPipelineEndToEnd`, build tag `e2e`)

1. **Provision** — `apply` found-hub → found-spoke (CB-A) → found-spoke (CB-B) →
   join, via `apply.Apply`. Fails with the partial report on any step error.
2. **Pair ACTIVE** — `cast call getPair(pairId)` returns `ACTIVE`.
3. **Swap** — `swapTokensForExactTokens` on the pair AMM liquidates.
4. **Breaker** — `pause` (1-of-N) → `isPaused()==true` → a swap reverts;
   `signResume` by two CBs (quorum 2) → `isPaused()==false` → swaps resume.
5. **SpokeBridge lock** — `lock(token,amount,txId)` → `getLock` shows the lock.
6. **Idempotency** — re-`apply` each mode → steps `skipped`/`done`; pair still
   `ACTIVE`.

Sub-tests (skip-with-warning):

- **`TestPipeline_HubMint`** — polls the hub token balance after a lock; the mint
  is **relay-mediated** and asynchronous, so it skips with a warning if the relay
  does not mint within the window.
- **`TestPipeline_BridgeRefund`** — `lock` → `release` (GOVERNANCE) → `getLock`
  released; `release` is the refund/reclaim path (there is **no on-chain timeout**
  in `SpokeBridge`). Skips with a warning when the round-trip cannot be arranged.

## Contract reality (mapping the roadmap wording)

- `SpokeBridge` exposes only `lock(token,amount,txId)` + `release(txId)`
  (GOVERNANCE) + `getLock`. There is **no** on-chain `mint`/`burn`/`unlock`/
  `timeout`. The hub mint is relay-mediated; the refund path is `release`.
- The AMM's external swap is `swapTokensForExactTokens`; the breaker is
  `pause(reason)` / `signResume(proposalId)` (RESUME_QUORUM = 2) / `isPaused()`.

## How to run

```bash
cd scenario-b/toolkit
# full pipeline (needs a real environment; see quickstart for the env vars)
go test -tags e2e  ./tests/e2e/...  -run TestPipeline -v
# per-mode E2E (TK-B7..B9)
go test -tags e2e  ./tests/e2e/...  -v
# performance baseline (toolkit-native, p95 quote/swap)
go test -tags perf ./tests/perf/... -run TestBaseline -v
# no environment → every E2E/perf test SKIPS with a warning (0 failures)
```

## Performance baseline

Toolkit-native Go harness (`tests/perf/baseline_test.go`, build tag `perf`).
Measures **quote p95** (`getAmountOut`) and **swap p95** (`swapTokensForExactTokens`)
against the toolkit-provisioned stack; logs `quote_p95_ms` / `swap_p95_ms`.
**Informative only** — no pass/fail threshold gate in this phase (production-grade
gating is deferred, consistent with the toolkit's local-first stance).

- `quote_p95_ms`: _to be recorded from a real run_
- `swap_p95_ms`: _to be recorded from a real run (optional swapper key)_

## Current status

- Per-mode E2E (found-hub / found-spoke / join / sovereign-pair): implemented,
  skip-with-warning.
- Full pipeline E2E + business path (swap / breaker / bridge lock) + idempotency:
  implemented (TK-B10), skip-with-warning.
- Hub mint + bridge refund: implemented as skip-with-warning sub-tests.
- Baseline: implemented; metrics to be recorded against a live stack.

## Deferred

- Production `CertSource`/`KeyProvider` (KMS/real CA), auth-per-CB relay, and a
  threshold-gated performance baseline remain deferred (local-first phase).
