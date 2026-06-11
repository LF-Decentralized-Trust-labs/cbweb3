# TASK-12 + TASK-13 — Scoping & Claim Verification

**Branch base:** `001-hub-network-isolation` (AMM contract is byte-identical to `develop`; hub-isolation work does not touch AMM source).
**Subject:** `scenario-b/contracts/src/AutomatedMarketMaker.sol`
**Date:** 2026-06-10
**Author:** scoping pass (Claude)

> Purpose: verify the P0-remediation claims for TASK-12/13 against the *actual* code on the
> branch that will merge to `develop`, and define the real scope — including the parts of
> the prescribed fix that **should not** be built as written.

---

## TL;DR

- **The vulnerability is real.** Any role-holder can drain the pool by calling the contract
  directly — confirmed in three functions, not just the one the plan cites.
- **The prescribed remedy is the wrong shape.** The plan says "implement proportional LP-share
  minting / minimum-liquidity burn / ratio-consistent deposits" — i.e. bolt the **Uniswap V2
  LP-token model** onto the contract. This **contradicts the system's deliberate architecture**,
  where LP-share accounting lives **off-chain in Postgres** (commit-reveal + `shares_percentage`)
  and the on-chain AMM is intentionally a "dumb" reserve primitive fed by a trusted backend.
- **Recommended scope:** close the drain with an **on-chain per-provider contribution guard**
  (a floor invariant), add the cheap **empty-pool swap guard** from TASK-13, and **drop** the
  LP-tokenization / min-liquidity-burn / ratio-consistency items as architecturally inconsistent.
- **One decision needs product/lead sign-off** (see §6): the on-chain vs off-chain ownership model.

---

## 1. What the code actually does (ground truth)

There is **no LP-token / share concept anywhere on-chain** — no `totalSupply`, no `balanceOf`,
no mint/burn of shares in `AutomatedMarketMaker.sol` *or* in `LiquidityCommitRegistry.sol`
(the registry tracks **commits**, not balances). Two distinct liquidity paths coexist:

| Path | Functions | Gate | Status |
|---|---|---|---|
| **Legacy dual-sided** | `addLiquidity` (`:174`), `removeLiquidity` (`:252`) | `onlyVerified` (any onboarded participant) | Explicitly **legacy** — `SeedHub.s.sol:87` ("legacy addLiquidity flows"), e2e script calls it a "fallback" |
| **Cooperative single-sided** | `addSingleSidedLiquidity` (`:198`), `removeSingleSidedLiquidity` (`:221`) | `onlyLiquidityProvider` | **Production path** — driven by backend commit-reveal (`liquidity_provision_service.go`) and sovereign CB flow (`sovereign_liquidity_service.go`) |

**Where share accounting really lives:** off-chain, in the api-gateway service
(`backend/services/api-gateway/internal/services/liquidity_provision_service.go`):
`LPShares`, `SharesPercentage`, `WithdrawalMode`, `recalculateSharesInTx()`, and separate
`removeLegacy` / `removeProportional` paths. The contract comments state this plainly:
> "The backend calculates the correct amount from shares_percentage; this function only performs
> the transfer and reserve update." (`:218-220`)

So the on-chain contract **trusts the backend** to pass correct amounts. The functions are
`external` and gated only by role, so that trust is **unenforced** — anyone holding the role can
bypass the backend and call directly.

---

## 2. Claim-by-claim verification

### TASK-12

| Claim | Verdict | Evidence |
|---|---|---|
| `addLiquidity` mints no LP tokens | ✅ **TRUE** | `:174-191` only does `reserveA += amountA; reserveB += amountB` + event. No shares exist anywhere on-chain. |
| `removeLiquidity` gated only by `amount <= reserve`, lets any whitelisted account drain the full pool | ✅ **TRUE** | `:261` checks only `amountA > reserveA || amountB > reserveB`; gate is `onlyVerified` (`:256`). No per-caller accounting. `removeLiquidity(reserveA, reserveB)` drains everything. |
| (plan's implied remedy) "implement proportional LP-share minting + min-liquidity lock" | ⚠️ **MISALIGNED** | See §3 — duplicates the off-chain ledger and breaks single-sided/sovereign deposits. |

**Plan undercount:** the plan only names `removeLiquidity`. The **same drain exists in
`removeSingleSidedLiquidity`** (`:221`, gate `onlyLiquidityProvider`, guard only `amount > reserve`
at `:230`/`:234`). Any LP-role holder can withdraw another provider's principal. The production
path has the identical hole. **Scope must cover both remove functions.**

### TASK-13

| Claim | Verdict | Evidence |
|---|---|---|
| `getAmountIn` does not guard `reserveIn == 0` | ⚠️ **PARTIALLY TRUE** | `getAmountIn` (`:273`) guards `amountOut >= reserveOut` (`:278`). A **fully empty** pool (both reserves 0) therefore *does* revert. But the **one-sided-empty** case (`reserveIn == 0, reserveOut > 0` — reachable via single-sided adds) is **not** guarded: numerator `= 0`, so `amountIn = 0/denominator + 1 = 1`, letting someone drain `reserveOut` for ~1 unit in. Real bug. |
| no minimum-liquidity burn on first deposit | ⚠️ **TRUE but N/A** | Correct that there's none — but "minimum-liquidity burn" is a property of the **LP-token model** (Uniswap's `MINIMUM_LIQUIDITY` share lock). With no shares, it has nothing to lock. Don't implement as written. |
| add `require(reserveA>0 && reserveB>0)` before swaps | ✅ **VALID, cheap** | Directly closes the one-sided-empty drain above. Keep. |
| "enforce ratio-consistent deposits" | ⚠️ **MISALIGNED** | Ratio-consistency is enforced **off-chain** by commit-reveal matching (`executeMatchedCommits`). On-chain ratio enforcement would break single-sided + sovereign deposits by design. Drop. |

---

## 3. Why the prescribed fix does not fit

The plan prescribes the **Uniswap V2 LP-token playbook**: mint ERC20 LP shares proportional to
deposits, burn `MINIMUM_LIQUIDITY` on first deposit, require ratio-consistent dual-sided deposits.
That model assumes **on-chain shares are the source of truth**. This system was built the opposite
way on purpose:

1. **Share ledger is off-chain** (Postgres `shares_percentage`, `recalculateSharesInTx`). On-chain
   LP tokens would **duplicate and potentially contradict** it — two sources of truth for the same fact.
2. **Deposits are single-sided** (`addSingleSidedLiquidity`) and matched off-chain. LP-token minting
   needs *both* sides deposited proportionally in one call to compute a clean share — incompatible
   with the cooperative single-sided model and the **sovereign CB** path where a Cacti watcher
   deposits on behalf of a central bank (`sovereign_liquidity_service.go`).
3. **Ratio-consistent on-chain deposits** would reject the very deposits the production path makes.

Building LP tokens here is not a hardening of the existing design — it is a **different design**,
and it would require ripping out the off-chain ledger and the commit-reveal/sovereign flows
(005-cooperative-liquidity, 007). That is out of proportion to a P0 security fix and not justified.

---

## 4. Recommended scope (what to actually build)

**A — Close the drain with an on-chain contribution guard (the core fix).**
Track each provider's net on-chain contribution per side and forbid withdrawing more than that:

```solidity
mapping(address => uint256) public contributedA;
mapping(address => uint256) public contributedB;
```
- `addLiquidity` / `addSingleSidedLiquidity`: increment the caller's `contributedX`.
- `removeLiquidity` / `removeSingleSidedLiquidity`: `require(amount <= contributedX[msg.sender])`
  then decrement. This is a **floor invariant** (can't withdraw more than you put in) that
  *complements* the off-chain proportional ledger rather than replacing it. It closes the drain
  in **all three** affected functions without introducing tradeable LP tokens or breaking
  single-sided/sovereign deposits.
- Note the known limitation (acceptable, document it): a pure contribution cap does not credit a
  provider's share of **swap fees / impermanent gain** — those remain governed by the off-chain
  `shares_percentage` ledger. The on-chain guard is a **security floor**, not the accounting system.

**B — Empty-pool swap guard (TASK-13, cheap & clearly correct).**
Add `if (reserveA == 0 || reserveB == 0) revert AMM__InsufficientLiquidity();` at the top of
`swapTokensForExactTokens` (and/or `require(reserveIn > 0)` in `getAmountIn`). Closes the
one-sided-empty drain (§2).

**C — Tests (Constitution Principle V: failing test first).**
- `removeLiquidity` / `removeSingleSidedLiquidity` revert when caller withdraws > own contribution
  (the drain regression test — currently *missing*; existing `test_RemoveLiquidity_Success` and
  `test_removeSingleSided_*` only assert reserve bounds and thereby **bless** the drain).
- swap reverts on one-sided-empty pool (`reserveIn == 0, reserveOut > 0`).
- existing happy-path + circuit-breaker tests still pass.

**D — Backend reconciliation check.**
Confirm `liquidity_provision_service.go` withdrawal amounts never exceed a provider's on-chain
`contributedX` (otherwise legitimate proportional withdrawals including earned fees could now
revert). If the off-chain ledger can legitimately exceed contribution (fee accrual), the guard
needs a fee-aware allowance or the fee settlement must be modeled — **flag for design** (§6).

**Explicitly OUT of scope (reject from the plan):** ERC20 LP-token minting; `MINIMUM_LIQUIDITY`
burn; on-chain ratio-consistent deposit enforcement. All three are LP-token-model artifacts that
don't fit this architecture.

---

## 5. Affected files

| File | Change |
|---|---|
| `scenario-b/contracts/src/AutomatedMarketMaker.sol` | contribution mappings + guards in 2 add / 2 remove fns; empty-pool swap guard |
| `scenario-b/contracts/src/interfaces/IAutomatedMarketMaker.sol` | new error(s) / optional getters if exposed |
| `scenario-b/contracts/test/AutomatedMarketMaker.t.sol` | drain-regression + empty-pool tests; update tests that assume free drain |
| `scenario-b/contracts/test/AutomatedMarketMakerCooperative.t.sol` | per-provider single-sided withdrawal-cap tests |
| `backend/.../liquidity_provision_service.go` | reconcile withdrawal ≤ contribution; possible fee-allowance handling (pending §6) |
| `backend/shared/blockchain/scenariob/amm/client.go` | only if new getters/ABI surface added |

No redeploy-and-resync concerns beyond the standard `contracts.deploy-* → contracts.sync-addresses`
(this is a source change, which the CLAUDE.md gotcha notes is a separate concern from redeploy targets).

---

## 6. Decision required before implementation

**On-chain ownership model.** The team must confirm the intended trust boundary:

- **Option B (recommended):** on-chain contribution floor-guard, off-chain ledger remains source of
  truth for proportional value/fees. Minimal, architecture-consistent, closes the drain.
- **Option A (plan as written):** full on-chain LP tokenization. Correct proportional accounting,
  but a redesign — breaks single-sided + sovereign flows and duplicates the Postgres ledger.
  Not recommended as a P0 fix.
- **Option C:** restrict all `add*/remove*` to a single trusted AMM-operator role so the contract is
  an explicit backend-controlled primitive. Smallest contract change, but concentrates trust and may
  conflict with the sovereign-CB-calls-directly model — needs validation against
  `sovereign_liquidity_service.go`.

Coupled sub-question for Option B: **can a legitimate proportional withdrawal exceed contribution**
(earned fees)? If yes, the floor-guard needs a fee-aware allowance; if fees are not yet distributed
on-chain, the guard is safe as-is.

---

## 7. Revised effort estimate

Plan rates TASK-12+13 as **L (8–12d)** assuming LP tokenization. With the recommended
contribution-guard scope (Option B), realistic effort is **S–M (3–6d)**: ~1d contract + guards,
~1–2d tests (red→green), ~1–2d backend reconciliation + e2e, ~0.5d review/slither. The large
estimate was driven by the misaligned LP-token prescription; the actual security fix is smaller.
**Confirm §6 decision before committing to this number.**
