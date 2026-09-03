// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {CommonBase} from "forge-std/Base.sol";
import {StdCheats} from "forge-std/StdCheats.sol";
import {StdUtils} from "forge-std/StdUtils.sol";
import {AutomatedMarketMaker} from "../../src/AutomatedMarketMaker.sol";
import {TokenizedCentralBankMoney} from "../../src/TokenizedCentralBankMoney.sol";

/// @title AMMHandler
/// @notice Bounded action driver for the AutomatedMarketMaker invariant suite.
///
/// Why a handler at all. Turning the fuzzer loose on the AMM directly produces almost
/// nothing: every call arrives with random calldata from a random sender, so it is
/// rejected by `onlyVerified`, by the empty-pool guard, or by slippage before it ever
/// touches the reserves. The pool would sit untouched and the invariants would pass
/// vacuously — the worst outcome for a test suite, because it reports safety it never
/// checked. This handler is the AMM's own vocabulary: verified callers, funded
/// balances, amounts bounded to what could actually succeed.
///
/// What it deliberately does NOT do is pre-compute the answer. Bounds keep calls
/// plausible; they never assert what the reserves should become. That is the
/// invariants' job, and a handler that computed the expected result would only be
/// testing its own arithmetic.
contract AMMHandler is CommonBase, StdCheats, StdUtils {
    AutomatedMarketMaker public immutable AMM;
    TokenizedCentralBankMoney public immutable TOKEN_A;
    TokenizedCentralBankMoney public immutable TOKEN_B;

    address public immutable LP;
    address public immutable SWAPPER;
    address public immutable GOV_A;
    address public immutable GOV_B;

    // ----- ghost accounting -------------------------------------------------
    // The AMM tracks escrowed commit deposits in a private per-commitId mapping with
    // no total, so solvency cannot be checked from its public surface alone. The
    // handler knows what it deposited, finalized and cancelled, so it keeps the total.
    uint256 public ghostEscrowedA;
    uint256 public ghostEscrowedB;

    /// @notice Lowest constant product observed immediately after any swap, and the
    ///         value it was compared against. A swap must never shrink k: the fee
    ///         stays in the reserves, so k grows or (with feeBps 0) holds.
    bool public swapShrankK;
    uint256 public lastKBeforeSwap;
    uint256 public lastKAfterSwap;

    /// @notice A swap attempted while paused must revert. Recorded rather than
    ///         asserted here so the failure surfaces as a named invariant.
    bool public swapSucceededWhilePaused;

    /// @notice Why the last swap attempt failed, and how many bailed before even trying.
    ///         Without these a vacuous run says only "0 swaps", which is the symptom and
    ///         not the cause — the first run of this suite cost real time for that reason.
    bytes public lastSwapRevert;
    uint256 public swapBailedNoLiquidity;
    uint256 public swapBailedUnfunded;
    uint256 public swapAttempted;

    // Call counters, printed by the suite so a vacuous run is visible instead of
    // looking like a pass.
    uint256 public callsAddLiquidity;
    uint256 public callsSwap;
    uint256 public callsRemoveLiquidity;
    uint256 public callsCommitFlow;
    uint256 public callsBreaker;
    uint256 public callsFee;

    bytes32[] private _openCommits;

    constructor(
        AutomatedMarketMaker amm_,
        TokenizedCentralBankMoney tokenA_,
        TokenizedCentralBankMoney tokenB_,
        address lp_,
        address swapper_,
        address govA_,
        address govB_
    ) {
        AMM = amm_;
        TOKEN_A = tokenA_;
        TOKEN_B = tokenB_;
        LP = lp_;
        SWAPPER = swapper_;
        GOV_A = govA_;
        GOV_B = govB_;
    }

    // ============================================================================
    //                                 ACTIONS
    // ============================================================================

    /// @notice Add liquidity as the verified LP. Bounded to the LP's balance so the
    ///         call fails on pool logic, never on a plain ERC20 shortfall.
    function addLiquidity(uint256 amountA, uint256 amountB) external {
        if (AMM.isPaused()) return;
        amountA = _bound(amountA, 1e18, TOKEN_A.balanceOf(LP) / 4 + 1);
        amountB = _bound(amountB, 1e18, TOKEN_B.balanceOf(LP) / 4 + 1);
        vm.startPrank(LP);
        TOKEN_A.approve(address(AMM), amountA);
        TOKEN_B.approve(address(AMM), amountB);
        try AMM.addLiquidity(amountA, amountB) {
            callsAddLiquidity++;
        } catch {}
        vm.stopPrank();
    }

    /// @notice Swap for an exact output. The output is bounded strictly below the
    ///         out-side reserve, because the contract rejects draining a side
    ///         entirely — an unbounded amount would only ever produce reverts.
    function swapExactOut(bool aToB, uint256 amountOut) external {
        uint256 reserveOut = aToB ? AMM.reserveB() : AMM.reserveA();
        uint256 reserveIn = aToB ? AMM.reserveA() : AMM.reserveB();
        if (reserveIn == 0 || reserveOut <= 1) {
            swapBailedNoLiquidity++;
            return;
        }

        // Bounded to a slice of the out-side reserve, not the whole of it. A near-total
        // draw makes quoteExactOutput return an amountIn far past any funded balance, the
        // handler bails, and the swap path is never exercised — which is what the first
        // run of this suite actually did: 0 swaps in ~1500 attempts.
        amountOut = _bound(amountOut, 1, (reserveOut / 20) + 1);
        uint256 kBefore = reserveIn * reserveOut;
        uint256 amountIn = AMM.quoteExactOutput(reserveIn, reserveOut, amountOut, AMM.feeBps());

        TokenizedCentralBankMoney tokenIn = aToB ? TOKEN_A : TOKEN_B;
        if (tokenIn.balanceOf(SWAPPER) < amountIn) {
            swapBailedUnfunded++;
            return;
        }
        swapAttempted++;

        bool pausedBefore = AMM.isPaused();
        vm.startPrank(SWAPPER);
        tokenIn.approve(address(AMM), amountIn);
        try AMM.swapTokensForExactTokens(
            aToB ? address(TOKEN_A) : address(TOKEN_B),
            aToB ? address(TOKEN_B) : address(TOKEN_A),
            amountOut,
            amountIn,
            SWAPPER
        ) {
            callsSwap++;
            if (pausedBefore) swapSucceededWhilePaused = true;
            uint256 kAfter = AMM.reserveA() * AMM.reserveB();
            lastKBeforeSwap = kBefore;
            lastKAfterSwap = kAfter;
            if (kAfter < kBefore) swapShrankK = true;
        } catch (bytes memory err) {
            lastSwapRevert = err;
        }
        vm.stopPrank();
    }

    /// @notice Withdraw as the LP, in one token (the zap-out path).
    function removeLiquidity(uint256 shares, bool tokenAOut) external {
        if (AMM.isPaused()) return;
        uint256 held = AMM.balanceOf(LP);
        if (held == 0) return;
        shares = _bound(shares, 1, held);
        vm.startPrank(LP);
        try AMM.removeLiquidity(shares, tokenAOut ? address(TOKEN_A) : address(TOKEN_B), 0) {
            callsRemoveLiquidity++;
        } catch {}
        vm.stopPrank();
    }

    /// @notice One side of a cooperative commit deposit. Tracked in the ghost totals,
    ///         since this is money the AMM holds but does not count as reserves.
    function depositForCommit(uint256 seed, bool isTokenA, uint256 amount) external {
        if (AMM.isPaused()) return;
        bytes32 commitId = keccak256(abi.encode("commit", seed));
        TokenizedCentralBankMoney token = isTokenA ? TOKEN_A : TOKEN_B;
        amount = _bound(amount, 1e18, token.balanceOf(LP) / 8 + 1);
        vm.startPrank(LP);
        token.approve(address(AMM), amount);
        try AMM.depositForCommit(commitId, isTokenA, amount, LP) {
            callsCommitFlow++;
            if (isTokenA) ghostEscrowedA += amount;
            else ghostEscrowedB += amount;
            _openCommits.push(commitId);
        } catch {}
        vm.stopPrank();
    }

    /// @notice Finalize the oldest open commit: escrow becomes reserves.
    function finalizeCommit(uint256 seed) external {
        if (AMM.isPaused() || _openCommits.length == 0) return;
        uint256 idx = _bound(seed, 0, _openCommits.length - 1);
        bytes32 commitId = _openCommits[idx];
        (uint256 amountA, uint256 amountB) = _escrowOf(commitId);
        vm.prank(LP);
        try AMM.finalizeCommit(commitId) {
            callsCommitFlow++;
            ghostEscrowedA -= amountA;
            ghostEscrowedB -= amountB;
            _removeCommit(idx);
        } catch {}
    }

    /// @notice Cancel one side of an open commit; the escrow returns to the depositor.
    function cancelCommitDeposit(uint256 seed, bool isTokenA) external {
        if (_openCommits.length == 0) return;
        uint256 idx = _bound(seed, 0, _openCommits.length - 1);
        bytes32 commitId = _openCommits[idx];
        vm.prank(LP);
        try AMM.cancelCommitDeposit(commitId, isTokenA) returns (uint256 amount) {
            callsCommitFlow++;
            if (isTokenA) ghostEscrowedA -= amount;
            else ghostEscrowedB -= amount;
        } catch {}
    }

    /// @notice Exercise the asymmetric breaker: 1-of-N pauses, 2-of-N resumes.
    function toggleBreaker(uint256 seed) external {
        if (!AMM.isPaused()) {
            // Pause rarely. Pausing on every call starved every other path: the pool sat
            // paused, addLiquidity/swap/removeLiquidity all bailed on `whenNotPaused`, and
            // the suite passed having exercised almost nothing. The breaker still gets
            // exercised — just not to the exclusion of the pool it protects.
            if (seed % 8 != 0) return;
            vm.prank(GOV_A);
            try AMM.pause("invariant run") {
                callsBreaker++;
            } catch {}
            return;
        }
        // Resume deterministically: propose and reach the 2-of-N quorum in ONE call.
        // Spreading signatures across calls creates a fresh proposal each time, so no
        // single proposal ever reached quorum and the pool never resumed.
        vm.prank(GOV_A);
        try AMM.proposeResume() returns (bytes32 proposalId) {
            callsBreaker++;
            vm.prank(GOV_A);
            try AMM.signResume(proposalId) {} catch {}
            vm.prank(GOV_B);
            try AMM.signResume(proposalId) {} catch {}
        } catch {}
    }

    /// @notice Governance fee changes, including values above the cap (which must revert).
    function setFees(uint256 feeBps, uint256 withdrawalFeeBps) external {
        feeBps = _bound(feeBps, 0, AMM.MAX_FEE_BPS() + 500);
        withdrawalFeeBps = _bound(withdrawalFeeBps, 0, AMM.MAX_FEE_BPS() + 500);
        vm.startPrank(GOV_A);
        try AMM.setFeeBps(feeBps) {
            callsFee++;
        } catch {}
        try AMM.setWithdrawalFeeBps(withdrawalFeeBps) {
            callsFee++;
        } catch {}
        vm.stopPrank();
    }

    // ============================================================================
    //                                 HELPERS
    // ============================================================================

    function _escrowOf(bytes32 commitId) private view returns (uint256 amountA, uint256 amountB) {
        (,,,, uint256 a, uint256 b,) = AMM.getEscrow(commitId);
        return (a, b);
    }

    function _removeCommit(uint256 idx) private {
        _openCommits[idx] = _openCommits[_openCommits.length - 1];
        _openCommits.pop();
    }

    function totalCalls() external view returns (uint256) {
        return callsAddLiquidity + callsSwap + callsRemoveLiquidity + callsCommitFlow + callsBreaker + callsFee;
    }
}
