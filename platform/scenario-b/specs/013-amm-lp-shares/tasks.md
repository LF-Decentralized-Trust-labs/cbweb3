---
description: "Task list for 013-amm-lp-shares — on-chain LP-share AMM (subsumes P0 TASK-12/13)"
---

# Tasks: On-Chain LP-Share Model for the Scenario B Hub AMM

**Input**: `plan.md` (decisions D1–D7), `task-analysis.md` (worktree root)
**Validation gate**: `make scenario-b.test-integration` (`TestFullHappyPath`)

## Phase 1 — LP-share token in the AMM contract (TASK-12/13) ✅

- [X] T001 ERC20 LP-share token (`CBW3-LP`); proportional mint; `MINIMUM_LIQUIDITY` lock (TASK-12/13)
- [X] T002 Withdrawal gated by share ownership — pool-drain vector closed (TASK-12)
- [X] T003 Empty-pool swap guard; `getAmountOut` exact-input helper (TASK-13)
- [X] T004 D1 home-currency zap-out withdrawal + `minAmountOut`; `removeLiquidityEmergency` (paused exit)
- [X] T005 D5 governance-configurable `withdrawalFeeBps` (+ preserved `feeBps`), capped + evented
- [X] T006 D2 restricted LP-share transfers (verified participants only)
- [X] T007 Foundry suite incl. §2.2 worked-example numbers — 61/61 green

## Phase 2 — Escrow-and-finalize paired deposits (D6) ✅

- [X] T008 `depositForCommit` / `finalizeCommit` / `cancelCommitDeposit` + `getEscrow`
- [X] T009 Refund/timeout path tested (constitution: refund paths MUST be tested)

## Phase 3–4 — Backend on the on-chain model ✅

- [X] T010 amm client: escrow methods, `removeLiquidity(shares, homeIsTokenA, minOut)`, `LPBalanceOf`, `LPTotalSupply`
- [X] T011 Sovereign flow: per-side escrow against shared derived key; best-effort finalize (D4/D6)
- [X] T012 `LogCommitFinalized` decoded → real share counts persisted on `LiquidityPosition`
- [X] T013 Withdrawal wiring: owner resolved by signer address; CB-signed burn; unit tests
- [X] T014 **D7**: commercial provision path removed (`executeMatchedCommits`, DB counterpart-matching,
      `LinkCounterpart`) — commercial banks are pool users only

## Phase 5 — Governance portal ✅

- [X] T015 `GET /api/v2/amm/lp-balance` (live on-chain CBW3-LP position of the CB)
- [X] T016 "Central Bank On-Chain Position" card: LP shares, pool ownership %, tCeBM balance
- [ ] T017 Withdrawal quote UI: expected output + price-impact % + `minAmountOut` slippage bound
      (DEX-style confirm; plan §2.1/§2.2) — **open**

## Phase 6 — Validation ✅ / remaining

- [X] T018 `TestFullHappyPath` green end-to-end (deposit → pool ACTIVE → swap → receipt)
- [X] T019 `phase_6_lp_withdrawal` added: lp-balance read → burn → home-currency out → balance decreases
- [X] T020 Full integration suite green on a pristine stack incl. `phase_6_lp_withdrawal`
      (2026-06-11: 8/8 phases PASS; CB-A 50.00% → burn → 0 on-chain). Note: the realized-amount
      decode (`LogLiquidityRemoved` → `token_a_amount`) landed right after that run — the new
      `not-a-tx-hash` assertion validates it on the next run
- [ ] T021 `make scenario-b.perf-baseline` against the new AMM — **open**
- [ ] T022 Update `tryout-*liquidity*.sh` scripts still calling removed single-sided endpoints — **open**

## Merge

- [ ] T023 PR → `develop` (per project lead decision, 2026-06-11)
