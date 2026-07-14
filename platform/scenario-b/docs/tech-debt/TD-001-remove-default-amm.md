# TD-001 — Remove the default (bootstrap) AMM

Status: **OPEN** · Scenario B · Component: api-gateway + hub contracts · Raised: 2026-07-14

## Summary

The hub deploys a single **default AMM** at setup (`contracts/script/CBWeb3Hub.s.sol`,
`amm = new AutomatedMarketMaker(tokenBrl, tokenEur, identityRegistry)`), and its address
is propagated to every gateway as `AMM_CONTRACT_ADDRESS`. This AMM is bound to
`tCeBM_BRL`/`tCeBM_EUR` — **not** the sovereign W-tokens of any real corridor — so it is a
phantom pool that never carries corridor liquidity.

Corridors now get a **dedicated per-pair AMM** deployed on propose
(`PairService.ProposePair` → `PairRegistryClient.DeployDedicatedAMM`), and quote/swap/pool/
liquidity/mint resolve the AMM per `pool_pair` via `pairAMMResolver`. The default AMM should
therefore be removed. It has not been, because it is still load-bearing in several places.

## Why it can't just be deleted (coupling map)

`internal/app/app.go` gates most of the v2 AMM surface on `ammClient != nil` (derived from
`AMM_CONTRACT_ADDRESS`): `QuoteService`, `PoolStatusService`, `SwapService`,
`LiquidityService`, `CircuitBreakerService`, and the cross-currency swap orchestrator.

Worse, several `ammAdapter` methods use the default client `a.c` **directly, with no per-pair
resolution** (`internal/app/amm_adapter.go`):

| Method | Caller | Impact if default AMM removed |
|--------|--------|-------------------------------|
| `GetFeeBps` | `SwapService` fee distribution — **every swap** (`swap_service.go:189`) | swap breaks / wrong fee |
| `PauseCircuitBreaker` / `ProposeResume` / `SignResume` | `CircuitBreakerService` | **circuit breaker (a constitution-required safety control) stops working** |
| `LPBalanceOf` / `LPTotalSupply` / `RemoveLiquidityShares` | sovereign LP management | LP reads/withdrawals break |

The sovereign commit methods (`DepositForCommitAt`, `FinalizeCommitAt`, `TokenBalanceAt`) take an
explicit `ammAddress`, so they only need an RPC/signer connection — but they still ride on `a.c`.

## Required work (per-pair everything)

1. **Fee** — `SwapService` reads fee per swap; switch `AMMFeeReader.GetFeeBps(ctx)` →
   per-pair `GetFeeBpsForPair(ctx, pair)` (already on the adapter). Contained.
2. **Circuit breaker** — make pause/propose-resume/sign-resume target a specific pool's AMM
   (add a `pool_pair`/AMM selector to `CircuitBreakerService` + its handler/route). This alters a
   compliance/safety control → **needs project-lead approval** (see project workflow).
3. **LP global reads** — `LPBalanceOf`/`LPTotalSupply`/`RemoveLiquidityShares` become per-pair.
4. **Gating** — gate v2 AMM services on `pairResolver != nil` (PairRegistry configured) instead
   of `ammClient != nil`; make `AMM_CONTRACT_ADDRESS` optional.
5. **Adapter** — `ammAdapter.clientFor` returns an error (not a nil client) when it cannot
   resolve a pair and there is no default; update every call site to handle it (no nil deref).
6. **Contracts** — remove the default AMM deploy in `CBWeb3Hub.s.sol` (and the `tokenBrl`/`tokenEur`
   only if unused elsewhere); stop propagating `AMM_CONTRACT_ADDRESS` in the toolkit
   (`step_found_hub.go` / `step_found_spoke.go` / `step_join.go`, compose templates, `.env` files).
7. **Tests + E2E** — `make test.api-gateway`, `make contracts.test`, and a clean-deploy
   propose→seed→swap plus a breaker pause/resume check.

## Notes / partial progress

- The UI half is already done: the treasury pool-creation form no longer pre-fills the default
  AMM (commit `0720c61`); propose deploys a dedicated AMM (`25145ea`).
- Recommended sequencing: (1) fee per-pair + gating on resolver + adapter nil-safety + remove the
  contract deploy — validatable via swap E2E; (2) circuit-breaker + LP per-pair as a separate,
  lead-approved change, so the breaker is never silently disabled.
