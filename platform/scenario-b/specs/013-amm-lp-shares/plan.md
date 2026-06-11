# Plan: Standard Web3 LP-Share Model for the Scenario B Hub AMM

**Status:** Draft for review
**Scope:** `scenario-b/` only (scenario isolation respected)
**Subsumes:** P0-Remediation **TASK-12** (LP-share accounting + withdrawal guard + min-liquidity) and **TASK-13** (empty-pool protection + first-deposit min-liquidity burn + ratio-consistent deposits)
**Base branch:** `001-hub-network-isolation` (merging to `develop`)
**Companion:** see `task-analysis.md` (claim verification + why this replaces the off-chain-ledger drain fix)

---

## 0. Implementation status (2026-06-10)

| Phase | Status | Commit |
|---|---|---|
| 1 — LP-share token in AMM (TASK-12/13) | ✅ done, 52 tests | `09b0690e` |
| 2 — Escrow-and-finalize paired deposits (D6) | ✅ done, +9 tests (61 total) | `a19aaf2f` |
| 3 — Backend ledger → on-chain shares | ✅ done, service tests green | `c589acf8` |
| 4 — Sovereign flow (escrow per-side, shared key) | ✅ done | `c589acf8` |
| 5 — Frontend LP-share label/types | ✅ done (API-compatible; rich price-impact UI = follow-up) | — |
| 6 — Specs/docs + live integration test | ✅ done — `TestFullHappyPath` PASS (all 7 phases, 64s) | — |

**Known follow-ups (flagged, off the happy path):** (a) commercial multi-bank withdrawal needs the
owning bank to sign the burn (sovereign CB-signed withdrawal is already correct); (b) governance
frontend price-impact/slippage quote UI for the home-currency zap-out.

## 1. Goal

Replace the current "dumb reserves + off-chain Postgres share ledger" AMM with the **canonical
Uniswap-V2-style LP-share model**, where an on-chain ERC20 LP-share token is the **single source
of truth** for pool ownership. This delivers proportional ownership, burn-to-withdraw safety,
automatic fee accrual, and an empty-pool guard — which *are* TASK-12 and TASK-13 — as intrinsic
properties of the standard model rather than as bolt-on patches.

**Non-negotiable outcomes (from the remediation tasks):**
- TASK-12: `addLiquidity` mints proportional LP shares; withdrawal is gated by share ownership
  (you can only burn what you hold → no pool drain); minimum-liquidity locked on first deposit.
- TASK-13: swaps revert on an empty/one-sided-empty pool; `MINIMUM_LIQUIDITY` burned on first
  deposit; deposits are ratio-consistent.

---

## 2. The core design knot and its resolution

**Problem.** Standard LP minting requires a single provider to deposit **both** sides in **one
atomic call**, in pool ratio. But in this CBDC hub each central bank holds only **its own**
currency, so liquidity arrives **single-sided** (`addSingleSidedLiquidity`) as **two separate
transactions from two different providers**, matched off-chain via commit-reveal
(`LiquidityCommitRegistry` → `executeMatchedCommits`). That model is *why* single-sided exists.

**Decision (recommended): Paired atomic mint with proportional per-provider share allocation.**
- The AMM itself becomes the ERC20 LP-share token (one AMM instance per pair already exists — same
  shape as a UniV2 pair being its own LP token).
- A new entrypoint `addLiquidityPaired(providerA, amountA, providerB, amountB)` (operator/router
  gated) performs **one atomic balanced deposit**: pulls `amountA` from `providerA` and `amountB`
  from `providerB`, mints total shares `L`, and allocates them per provider **proportional to the
  value each contributed** at current pool price. For a ratio-matched pair `valueA == valueB`, so
  each gets `L/2`; the contract computes it generally so off-ratio matches degrade gracefully.
- The commit-reveal matcher feeds this entrypoint with an already-matched pair. **This eliminates
  the current `RECONCILIATION_REQUIRED` partial-failure state** (two independent txns that can
  half-succeed) — the paired deposit is atomic-or-revert, which also *strengthens* Constitution
  atomicity compliance (partial settlement forbidden).
- LP shares are **transfer-restricted to verified participants** (`onlyVerified` on `transfer`/
  `transferFrom`, delegating to `IdentityRegistry`) — standard ERC20 accounting semantics, but no
  leakage of pool ownership to non-participants. (Decision point D2 below: restricted-transfer vs
  fully non-transferable.)

**Rejected alternatives (Constitution Complexity Tracking):**
- *Each CB deposits both sides itself.* Rejected: contradicts the domain — CBs hold one currency.
- *Single-sided zap on **deposit** (deposit one side, swap half on-chain to balance).* Rejected:
  moves the FX price on every deposit. (Note: the symmetric cost reappears on **withdrawal** because
  of decision D1 — see §2.1; we accept it on exit but not on entry, since entry is paired/balanced.)
- *Mint pooled LP shares to a router that sub-accounts providers off-chain.* Rejected: reintroduces
  the off-chain ownership ledger we are removing — defeats the purpose.

### 2.1 Home-currency withdrawal mechanics (per decision D1)

`removeLiquidity(uint256 shares, address tokenOut, uint256 minAmountOut)`:
1. Compute pro-rata `amountA = shares·reserveA/totalSupply`, `amountB = shares·reserveB/totalSupply`.
2. Burn `shares`; decrement reserves by `amountA`/`amountB`.
3. **Zap-out:** swap the non-`tokenOut` side into `tokenOut` along the constant-product curve against
   the post-burn reserves, charging a **governance-configurable** withdrawal fee (see §2.3).
4. Transfer `amountOut = (home-side pro-rata) + (swap proceeds)` to the provider; revert if
   `amountOut < minAmountOut` (caller-supplied slippage bound).

**Consequences this introduces (accepted, but must be handled):**
- **Price impact / slippage.** The zap-swap moves the pool price; a large LP exiting gets a worse
  effective rate than a small one. Withdrawal value is therefore *not* a clean pro-rata of reserves.
  → `minAmountOut` slippage guard is mandatory; the UI must quote expected output + impact.
- **Paused-state exit.** Because withdrawal now contains a swap, it must respect the circuit breaker.
  We add an **emergency both-sided exit** `removeLiquidityEmergency(shares)` that returns pro-rata
  `(amountA, amountB)` **without** swapping, usable `whenPaused`, so LPs are never trapped when the
  pool is halted. (This is the "both-sided" path from the rejected D1 option, repurposed as a safety
  valve.)
- **First/last-unit edge cases.** Withdrawing the final shares must not leave the pool one-sided
  (which would brick swaps via the §5 empty-pool guard); covered by tests.

### 2.2 Worked example (the slippage behavior, in numbers)

These numbers should become Phase-1 `forge` test assertions (the constant-product + 0.3% fee math
is exact and deterministic).

**Pool:** BRL↔EUR priced at 1 EUR = 5 BRL. Reserves: **5,000,000 BRL / 1,000,000 EUR**
(`k = 5e12`). Total LP shares ≈ `sqrt(5,000,000·1,000,000)` ≈ **2,236,068** (MINIMUM_LIQUIDITY lock
omitted for clarity). Provider **BCB** owns shares; home currency = **BRL**.

**Step 1 — pro-rata both sides** (for a holding of fraction `f`): `f·reserveA` BRL + `f·reserveB` EUR.
**Step 2 — burn + remove** (ratio preserved, price unmoved).
**Step 3 — zap:** swap the EUR slice into BRL against post-burn reserves, exact-input with 0.3% fee:
`amountOut = (amountIn·997·reserveOut) / (reserveIn·1000 + amountIn·997)`.
**Step 4 — pay out** `(home pro-rata) + (swap proceeds)` in BRL; revert if `< minAmountOut`.

| Case | Owns | Pro-rata | Reserves after burn | Zap (EUR→BRL) | **Total BRL out** | Notional @5 | **Haircut** |
|---|---|---|---|---|---|---|---|
| **Large exit** | 10% (223,606 sh) | 500,000 BRL + 100,000 EUR | 4,500,000 / 900,000 | 100,000 EUR → **≈448,785** | **≈948,785** | 1,000,000 | **≈5.1%** |
| **Small exit** | 1% (22,360 sh) | 50,000 BRL + 10,000 EUR | 4,950,000 / 990,000 | 10,000 EUR → **≈49,353** | **≈99,353** | 100,000 | **≈0.65%** |

**Lesson:** single-currency exit **self-penalizes by size** — you trade against the liquidity you
just removed. A 10% exit loses ~5.1%; a 1% exit loses ~0.65% (≈ the 0.3% fee + minor impact). This
is inherent to single-asset exit, not a defect, and is exactly why `minAmountOut` + a UI
price-impact quote are mandatory.

**Emergency path (paused):** `removeLiquidityEmergency` skips Step 3 — the 10% holder receives the
raw **500,000 BRL + 100,000 EUR**, no swap, no slippage, and converts the EUR elsewhere.

**Fee tailwind:** swap fees accrue into reserves while `totalSupply` is fixed, so shares redeem for
more over time; the Step-1 pro-rata grows with pool volume, partially offsetting the Step-3 exit
slippage.

**Implementation note:** the contract currently exposes only exact-*output* math (`getAmountIn`); the
zap needs the exact-*input* direction (`getAmountOut`), added in Phase 1 and rounded in the pool's
favor.

### 2.3 Fee policy — all fees are governance-configurable parameters (no hardcoded rates)

**Hard requirement:** every fee in the AMM is a **storage parameter with a governance setter, a
bounded cap, and an event** — never a hardcoded constant baked into the math. This already holds for
the swap fee today (`feeBps` storage, default 30 = 0.3%; `setFeeBps()` `onlyGovernance`; bounded by
`MAX_FEE_BPS = 1000`; `LogFeeRateUpdated` event) and **must be preserved and extended** by this work.

| Fee | Parameter | Setter | Cap | Event | Default |
|---|---|---|---|---|---|
| Swap fee | `feeBps` *(exists)* | `setFeeBps()` `onlyGovernance` | `MAX_FEE_BPS` (10%) | `LogFeeRateUpdated` | 30 (0.3%) |
| Withdrawal/zap fee | `withdrawalFeeBps` *(new)* | `setWithdrawalFeeBps()` `onlyGovernance` | `MAX_FEE_BPS` | `LogWithdrawalFeeUpdated` | 30 (0.3%) |

**Decision D5 — withdrawal fee knob: RESOLVED → separate `withdrawalFeeBps`.** The §2.1 zap charges
its own governance-configurable rate, independent of the swap fee, so governance can price exits on
their own (set to `0` for fee-free exits, or higher to discourage churn) without touching swap
economics. **Default = 30 bps (0.3%)**, which keeps the §2.2 worked numbers valid as written. Rejected
the "reuse `feeBps`" alternative: one knob is simpler but couples two distinct policy levers, and
decoupling costs only one extra storage slot + setter.

Rules that apply to **every** fee parameter (existing and new): setter is `onlyGovernance`; value is
validated against an on-chain cap (reverting `AMM__FeeBpsTooHigh`-style on excess); each change emits
a structured event for audit/observability (Constitution Principle VI); the cap itself stays a
`constant` (it bounds the parameter, it is not a fee). `MINIMUM_LIQUIDITY` is **not** a fee and stays
constant. Phase 1 tests must cover: default value, governance can change it, non-governance reverts,
over-cap reverts, and the new rate takes effect on the next swap/withdrawal.

---

## 3. Source-of-truth migration strategy

**Clean cutover, no on-chain state migration.** Because the hub starts at **zero TVL** (per
001-hub-network-isolation; TVL baseline is zero until the first cross-chain lock), we deploy the new
AMM fresh and seed liquidity through the new path. There are **no live positions to migrate**.

- On-chain: LP-share `balanceOf` / `totalSupply` become authoritative for ownership and value.
- Off-chain `LiquidityPosition` (Postgres) is **demoted to a read-model/index**: it keeps metadata
  the chain doesn't (commit linkage `commit_id`, `provider_bank_id`, timestamps, audit) and **caches**
  share balances rebuilt from on-chain `Mint`/`Burn`/`Transfer` events. It is no longer consulted to
  *authorize* withdrawals.
- Fields removed/repurposed: `SharesPercentage` and `FeeClaimAccumulated` and the
  `recalculateSharesInTx` machinery are deleted — proportional value and fees are now intrinsic to
  share redemption against current reserves. `LPShares` is mirrored from chain. `DepositSide` is
  retained only as descriptive metadata of how the provider funded the paired deposit.

---

## 4. Constitution Check

| Principle | Impact | Verdict |
|---|---|---|
| **Scenario isolation** | All changes in `scenario-b/`; no scenario-a coupling. | ✅ Pass |
| **Privacy** | Per-provider LP-share balances expose each CB's liquidity contribution on-chain. Pool reserves are *already* public; this is reserve-layer FX among known CBs, not retail PII/value. **Flag D3:** confirm exposing per-provider share balances on the hub chain is acceptable; if not, balances must be access-gated (view restricted) — does not change the mint/burn core. | ⚠️ Decision D3 |
| **Atomicity** | Paired mint is atomic-or-revert; removes the `RECONCILIATION_REQUIRED` partial state. Withdrawal's burn+zap-swap (D1) is a single atomic call — no partial exit. `removeLiquidityEmergency` guarantees a non-swap exit path when paused. Net improvement. | ✅ Pass (improves) |
| **Compliance gate** | Deposit/withdraw still flow through the api-gateway compliance + IdentityRegistry checks; LP-share transfers gated by `onlyVerified`. | ✅ Pass |
| **Test-first, every layer** | Plan is red→green per layer (Foundry → Go → E2E). No implementation before a failing test. | ✅ Pass |
| **Observability** | New `Mint`/`Burn` events with provider, shares, amounts; structured logs across the pairing flow; relay/lifecycle logging unchanged. | ✅ Pass |

**Decisions (RESOLVED 2026-06-10):**
- **D1 — Withdrawal output: HOME CURRENCY.** Burning shares returns the provider's **own**
  currency only. The contract computes the pro-rata `(amountA, amountB)`, then **swaps the
  non-home side back into the home side** ("zap-out") and returns a single-currency amount. This
  is a deliberate departure from the simplest standard model — see §2.1 for mechanics and §8 for
  the price-impact/slippage consequences it introduces.
- **D2 — Share transferability: RESTRICTED.** Standard ERC20 `transfer`/`transferFrom`, but they
  revert unless both parties are verified in the `IdentityRegistry`.
- **D3 — Per-provider balance privacy: EXPOSED.** Per-CB share balances are openly readable
  on-chain (consistent with already-public reserves). No privacy layer in this work.
- **D4 — Sovereign share recipient: THE CB'S OWN ADDRESS.** Sovereign/watcher deposits mint shares
  to the central bank's address, not the operator. (Confirm the exact CB key/address in Phase 0.)
- **D6 — Paired deposit mechanism: ESCROW-AND-FINALIZE.** The atomic single-tx paired mint conflicts
  with the sovereign flow (each CB deposits its own side independently from its own gateway/key, two
  txns). Instead the AMM escrows each side against a commit id (`depositForCommit`) and mints
  proportional shares to each recipient when both sides are present (`finalizeCommit`); an
  un-finalized side is refundable (`cancelCommitDeposit`). Unifies the commercial + sovereign flows,
  preserves sovereignty, keeps shares on-chain per provider, atomic at finalize. Implemented in
  Phase 2 (contract + tests green).

---

## 5. Phased implementation (test-first)

### Phase 0 — Decisions & contracts of record (0.5d)
- Resolve D1–D4. Write the new `IAutomatedMarketMaker` interface surface (LP-share functions,
  events, errors) and the pairing entrypoint signature as the contract of record.

### Phase 1 — LP-share token in the AMM contract (TASK-12 + TASK-13 core) 🎯
*Foundry, red→green.*
- Make `AutomatedMarketMaker` an ERC20 LP-share token (`totalSupply`, `balanceOf`, restricted
  `transfer`).
- `addLiquidity(amountA, amountB)` (single-provider, dual-sided): first deposit mints
  `sqrt(amountA·amountB) − MINIMUM_LIQUIDITY` and **locks `MINIMUM_LIQUIDITY`** (TASK-13); later
  deposits mint `min(amountA·totalSupply/reserveA, amountB·totalSupply/reserveB)` and enforce
  **ratio consistency** within tolerance (TASK-13).
- `removeLiquidity(shares, tokenOut, minAmountOut)`: burn `shares`, compute pro-rata both sides, then
  **zap-swap the non-home side into `tokenOut`** and return single-currency output (decision D1, §2.1).
  Ownership is enforced by ERC20 balance — **the drain is closed** (TASK-12); no per-caller bookkeeping.
- `removeLiquidityEmergency(shares)`: `whenPaused` both-sided pro-rata exit (no swap) so LPs are never
  trapped when the circuit breaker is engaged (§2.1).
- Swap guard: revert when `reserveA == 0 || reserveB == 0` (TASK-13); keep fee-in-reserve (fees now
  auto-accrue to share value).
- **Fees stay governance-configurable parameters (§2.3):** preserve `feeBps`/`setFeeBps`/cap/event;
  add `withdrawalFeeBps` + setter/cap/event per D5. No fee may be a hardcoded literal in the math.
- Delete `addSingleSidedLiquidity` / `removeSingleSidedLiquidity` (replaced by Phase 2 pairing).
- **Tests (write failing first):** proportional mint; first-deposit min-liquidity lock; inflation/
  donation attack reverts; withdrawal returns pro-rata; non-owner cannot withdraw others' liquidity
  (drain regression); empty-pool swap reverts; ratio-inconsistent deposit reverts; fee accrues to
  share value; circuit-breaker + identity gating still pass. Run `forge fmt/build/test` + slither.
- **D1 withdrawal tests (add):** home-currency zap-out returns a single currency; `minAmountOut`
  slippage guard reverts on excess impact; large-LP exit reflects price impact; `removeLiquidityEmergency`
  returns both sides `whenPaused` and is blocked `whenNotPaused`; withdrawing the final shares cannot
  leave the pool one-sided.

### Phase 2 — Paired-deposit periphery + cooperative refactor (1–1.5d)
*Foundry + Go.*
- Add `addLiquidityPaired(providerA, amountA, providerB, amountB)` (operator-gated): atomic balanced
  pull + proportional per-provider mint.
- Rework `executeMatchedCommits` to call the single atomic paired entrypoint instead of two
  independent `addSingleSidedLiquidity` txns; remove the `RECONCILIATION_REQUIRED` partial path.
- `LiquidityCommitRegistry` semantics unchanged as a *matcher*; only its consumer changes.
- Tests: matched pair mints correct split; off-ratio pair handled/reverted per policy; failed pair
  fully reverts (no partial deposit).

### Phase 3 — Backend: ledger → cache (1–1.5d)
*Go.*
- `liquidity_provision_service.go`: delete `removeProportional`/`removeLegacy`/`recalculateSharesInTx`/
  `fee_claim_accumulated`; withdrawal now reads on-chain share balance and calls `removeLiquidity(shares)`.
- AMM Go client (`backend/shared/blockchain/scenariob/amm/client.go`): add `LPBalanceOf`,
  `LPTotalSupply`, `AddLiquidityPaired`, `RemoveLiquidity(shares)`; drop single-sided methods.
- `LiquidityPosition` repo becomes an event-sourced index (rebuild from `Mint`/`Burn`); DB migration
  to drop/repurpose columns. amm_adapter updated.
- Tests: service unit tests against the new client interface; reconciliation-from-events test.

### Phase 4 — Sovereign liquidity flow (0.5–1d)
*Go.*
- `sovereign_liquidity_service.go`: deposit on behalf of a CB now mints shares to the **CB address**
  (D4) via the paired/sovereign entrypoint; synthetic direct-deposit path updated; idempotency
  preserved.

### Phase 5 — Frontend (1–1.5d)
*React.*
- Governance `liquidity` feature (`LiquidityManagementPage`, `CooperativeLiquidityWizard`,
  `liquidity.store`, `liquidity.api`, `liquidity.types`) and bank `LiquidityTransfersPage`/
  `AMMTradingPage`: show on-chain LP-share balance + redeemable value; remove `shares_percentage`
  UI; deposit wizard targets the paired flow.

### Phase 6 — Specs, docs, E2E, perf (1d)
- Update **005-cooperative-liquidity** and **007-bridge-based-cb-liquidity** specs + `data-model.md`
  to the on-chain-share model; note the removed off-chain ledger authority.
- Update `tryout-*.sh` e2e scripts that call single-sided liquidity.
- Run `scenario-b.test`, the AMM perf baseline (TASK-09 prerequisite), and a full deposit→swap→
  withdraw E2E proving share mint/burn and fee accrual.

---

## 6. Blast radius

| Layer | Files |
|---|---|
| Contract | `AutomatedMarketMaker.sol`, `interfaces/IAutomatedMarketMaker.sol` |
| Contract tests | `AutomatedMarketMaker.t.sol`, `AutomatedMarketMakerCooperative.t.sol`, `LiquidityCommitRegistry.t.sol` |
| Deploy/seed | `script/SeedHub.s.sol`, `script/SeedNewSovereignPair.s.sol` |
| Backend (Go) | `amm/client.go`, `amm_adapter.go`, `liquidity_provision_service.go`, `sovereign_liquidity_service.go`, `domain/liquidity_position.go` + GORM migration, commit consumer |
| Frontend | governance `features/liquidity/*`, `services/api/liquidity.api.ts`, `types/liquidity.types.ts`; bank `LiquidityTransfersPage.tsx`, `AMMTradingPage.tsx` |
| Specs/docs | `specs/005-*`, `specs/007-*`, `data-model.md`, deployment runbook liquidity section |
| E2E | `tryouts/tryout-*liquidity*.sh`, `tryout-scenario-b-e2e.sh`, perf baseline |

---

## 7. TASK-12 / TASK-13 traceability

| Remediation requirement | Delivered by |
|---|---|
| TASK-12 proportional LP-share minting | Phase 1 `addLiquidity` mint formula |
| TASK-12 per-caller withdrawal guard (no drain) | Phase 1 burn-your-own-shares (ERC20 balance) |
| TASK-12 minimum-liquidity lock | Phase 1 `MINIMUM_LIQUIDITY` burn on first deposit |
| TASK-13 `reserveIn==0` / empty-pool swap guard | Phase 1 swap precondition |
| TASK-13 min-liquidity burn on first deposit | Phase 1 (same as above) |
| TASK-13 ratio-consistent deposits | Phase 1 ratio check + Phase 2 paired balance |

Both tasks are **fully subsumed**; no separate TASK-12/13 work remains after this plan lands.

---

## 8. Effort & risk (Claude-Code-assisted)

| | Authoring (with Claude Code) | Wall-clock to merge |
|---|---|---|
| Phases 1–2 (contract + pairing) | ~0.5–1d | gated by money-safety review + slither/audit |
| Phases 3–4 (backend) | ~0.5–1d | gated by integration tests |
| Phase 5 (frontend) | ~0.5d | review |
| Phase 6 (specs/E2E/perf) | ~0.5d | gated by live hub stack |

The *code* is days, not weeks. The residual wall-clock is the **irreducible** part: D1–D4 decisions,
**money-safety review of a value-custody ERC20** (inflation attack, rounding, reentrancy), and live
E2E/perf validation — none of which should be compressed just because authoring is fast.

**Top risks:** (1) first-deposit inflation/donation attack — mitigated by `MINIMUM_LIQUIDITY` lock +
test; (2) rounding direction on mint/burn (always round against the LP) — covered by invariant tests;
(3) the paired off-ratio allocation policy — pin in Phase 2; (4) event-sourced cache drift — add a
periodic reconcile + a chain-vs-cache assertion in E2E; (5) **D1 withdrawal price-impact** — single-
currency exit zap-swaps and moves price, so large exits self-penalize and could be gamed around
swaps; mitigated by the mandatory `minAmountOut` guard, charging `feeBps` on the zap, and the
`removeLiquidityEmergency` non-swap path. In a permissioned network of known CBs, sandwich/MEV risk
is low but should still be noted in review.

---

## 9. Status: ready to start

D1–D5 are **resolved** (§4, §2.3): home-currency withdrawal · restricted transfers · exposed per-CB
balances · shares to the CB address · separate `withdrawalFeeBps` (default 0.3%). The only residual
input is confirming the exact CB key/address for the sovereign mint (D4), captured in Phase 0 without
blocking. Everything else is mechanical and test-gated — Phase 1 (contract + failing Foundry tests)
can begin.
