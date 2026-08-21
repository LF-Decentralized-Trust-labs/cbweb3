// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {CommonBase} from "forge-std/Base.sol";
import {StdCheats} from "forge-std/StdCheats.sol";
import {StdUtils} from "forge-std/StdUtils.sol";
import {AutomatedMarketMaker} from "../../src/AutomatedMarketMaker.sol";
import {TokenizedCentralBankMoney} from "../../src/TokenizedCentralBankMoney.sol";

/// @title AMMHandler
/// @notice Bounded action driver for the Scenario A AMM invariant suite.
///
/// NOT a copy of Scenario B's handler, and the difference is the point. Scenario A's
/// AMM is a different shape: it mints no LP shares (it is not an ERC20), has no
/// withdrawal path at all, and has no escrow-and-finalize commit flow. Liquidity goes
/// in and only ever leaves as the output side of a swap. So two of Scenario B's
/// invariants have nothing to attach to here, and one property exists only here —
/// liquidity is one-way, which makes the reserves fully reconstructible from what was
/// deposited minus what swaps paid out.
///
/// Scenario A's circuit breaker is also richer: every pause opens a new `pauseEpoch`,
/// and a resume proposal is bound to the epoch it was raised in, so a proposal
/// abandoned under an earlier pause cannot be revived later. That rule gets its own
/// action and its own invariant.
contract AMMHandler is CommonBase, StdCheats, StdUtils {
    AutomatedMarketMaker public immutable AMM;
    TokenizedCentralBankMoney public immutable TOKEN_A;
    TokenizedCentralBankMoney public immutable TOKEN_B;

    address public immutable LP;
    address public immutable SWAPPER;
    address public immutable GOV_A;
    address public immutable GOV_B;
    address public immutable GOV_C;

    // ----- ghost accounting -------------------------------------------------
    // Liquidity is one-way here, so these fully account for the pool: whatever was
    // deposited, minus whatever swaps paid out, must still be held.
    uint256 public ghostDepositedA;
    uint256 public ghostDepositedB;
    uint256 public ghostPaidOutA;
    uint256 public ghostPaidOutB;
    // Swap INPUTS also enter the pool, and omitting them was a real defect in the first
    // version of this accounting: the invariant failed by 1 wei because the quote uses
    // ceiling division, so the charge is not simply the mirror of the payout.
    uint256 public ghostSwappedInA;
    uint256 public ghostSwappedInB;

    bool public swapShrankK;
    bool public swapSucceededWhilePaused;
    bool public feeChangedWhilePaused;

    /// @notice Set if a resume proposal raised under an earlier pause epoch was ever
    ///         accepted. That would let a stale quorum reopen the pool after a fresh
    ///         pause — the exact revival the epoch counter exists to prevent.
    bool public staleProposalResumed;

    /// @notice Set if the stale signature was ACCEPTED at all. This, not the resume, is
    ///         the violation: a proposal from an earlier epoch already holding quorum
    ///         does not need to resume the pool to be a bug, and the contract only
    ///         auto-resumes on reaching quorum — so keying the check on the resume let a
    ///         mutation that removed the epoch guard pass unnoticed.
    bool public staleProposalAccepted;

    uint256 public lastPauseEpoch;
    bool public pauseEpochWentBackwards;

    uint256 public callsAddLiquidity;
    uint256 public callsSwap;
    uint256 public callsBreaker;
    uint256 public callsFee;
    uint256 public callsStaleProposal;

    bytes public lastSwapRevert;

    /// @notice Why the stale-proposal signature was rejected. Recorded because the first
    ///         version of this path was rejected by the ALREADY-SIGNED guard instead of
    ///         the epoch guard it was written to test — the same signature reused for
    ///         quorum and for the stale attempt. Mutation testing found it: removing the
    ///         epoch check from the contract changed nothing.
    bytes public lastStaleSignRevert;

    constructor(
        AutomatedMarketMaker amm_,
        TokenizedCentralBankMoney tokenA_,
        TokenizedCentralBankMoney tokenB_,
        address lp_,
        address swapper_,
        address govA_,
        address govB_,
        address govC_
    ) {
        AMM = amm_;
        TOKEN_A = tokenA_;
        TOKEN_B = tokenB_;
        LP = lp_;
        SWAPPER = swapper_;
        GOV_A = govA_;
        GOV_B = govB_;
        GOV_C = govC_;
        lastPauseEpoch = amm_.pauseEpoch();
    }

    // ============================================================================
    //                                 ACTIONS
    // ============================================================================

    function addLiquidity(uint256 amountA, uint256 amountB) external {
        _trackEpoch();
        if (AMM.paused()) return;
        amountA = _bound(amountA, 1e18, TOKEN_A.balanceOf(LP) / 4 + 1);
        amountB = _bound(amountB, 1e18, TOKEN_B.balanceOf(LP) / 4 + 1);
        vm.startPrank(LP);
        TOKEN_A.approve(address(AMM), amountA);
        TOKEN_B.approve(address(AMM), amountB);
        try AMM.addLiquidity(amountA, amountB) {
            callsAddLiquidity++;
            ghostDepositedA += amountA;
            ghostDepositedB += amountB;
        } catch {}
        vm.stopPrank();
    }

    /// @notice Swap for an exact output, bounded to a slice of the out-side reserve. A
    ///         near-total draw quotes an input past any funded balance, the attempt is
    ///         skipped, and the swap path never runs — the failure mode that made the
    ///         first Scenario B run report six passing invariants and zero swaps.
    function swapExactOut(bool aToB, uint256 amountOut) external {
        _trackEpoch();
        uint256 reserveOut = aToB ? AMM.reserveB() : AMM.reserveA();
        uint256 reserveIn = aToB ? AMM.reserveA() : AMM.reserveB();
        if (reserveIn == 0 || reserveOut <= 1) return;

        amountOut = _bound(amountOut, 1, (reserveOut / 20) + 1);
        uint256 kBefore = reserveIn * reserveOut;
        uint256 amountIn = AMM.quoteExactOutput(reserveIn, reserveOut, amountOut, AMM.feeBps());

        TokenizedCentralBankMoney tokenIn = aToB ? TOKEN_A : TOKEN_B;
        if (tokenIn.balanceOf(SWAPPER) < amountIn) return;

        bool pausedBefore = AMM.paused();
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
            if (aToB) {
                ghostSwappedInA += amountIn;
                ghostPaidOutB += amountOut;
            } else {
                ghostSwappedInB += amountIn;
                ghostPaidOutA += amountOut;
            }
            if (AMM.reserveA() * AMM.reserveB() < kBefore) swapShrankK = true;
        } catch (bytes memory err) {
            lastSwapRevert = err;
        }
        vm.stopPrank();
    }

    /// @notice Pause rarely, resume deterministically. Pausing on every call parks the
    ///         pool and starves every other path, which makes the whole suite vacuous.
    function toggleBreaker(uint256 seed) external {
        _trackEpoch();
        if (!AMM.paused()) {
            if (seed % 8 != 0) return;
            vm.prank(GOV_A);
            try AMM.pause("invariant run") {
                callsBreaker++;
            } catch {}
            _trackEpoch();
            return;
        }
        // proposeResume already counts as the proposer's signature, so one more
        // distinct signer reaches the 2-of-N quorum.
        vm.prank(GOV_A);
        try AMM.proposeResume() returns (bytes32 proposalId) {
            callsBreaker++;
            vm.prank(GOV_B);
            try AMM.signResume(proposalId) {} catch {}
        } catch {}
    }

    /// @notice Raise a proposal, let a NEW pause epoch open, then try to sign the stale
    ///         one. Scenario A binds a proposal to its epoch precisely so this cannot
    ///         reopen the pool; without the check a quorum gathered under an old pause
    ///         would still count.
    function signStaleProposal(uint256 seed) external {
        _trackEpoch();
        if (!AMM.paused()) {
            if (seed % 4 != 0) return;
            vm.prank(GOV_A);
            try AMM.pause("stale-proposal setup") {} catch {}
        }
        if (!AMM.paused()) return;

        bytes32 stale;
        vm.prank(GOV_A);
        try AMM.proposeResume() returns (bytes32 id) {
            stale = id;
        } catch {
            return;
        }

        // Close this epoch and open a new one: reach quorum with a THIRD signer, so
        // GOV_B's signature stays unused and the stale attempt below is rejected by the
        // epoch guard rather than by the already-signed guard.
        vm.prank(GOV_C);
        try AMM.signResume(stale) {} catch {}
        if (AMM.paused()) return; // never resumed; nothing stale to test yet
        vm.prank(GOV_A);
        try AMM.pause("new epoch") {} catch {}
        _trackEpoch();

        callsStaleProposal++;
        vm.prank(GOV_B);
        try AMM.signResume(stale) {
            // Accepted under a newer epoch: the guard is gone, whether or not this
            // particular signature also happened to reopen the pool.
            staleProposalAccepted = true;
            if (!AMM.paused()) staleProposalResumed = true;
        } catch (bytes memory err) {
            lastStaleSignRevert = err;
        }
    }

    function setFee(uint256 feeBps) external {
        _trackEpoch();
        feeBps = _bound(feeBps, 0, AMM.MAX_FEE_BPS() + 500);
        bool pausedBefore = AMM.paused();
        vm.prank(GOV_A);
        try AMM.setFeeBps(feeBps) {
            callsFee++;
            if (pausedBefore) feeChangedWhilePaused = true;
        } catch {}
    }

    // ============================================================================
    //                                 HELPERS
    // ============================================================================

    /// @dev The epoch counter must only ever move forward; a decrease would mean a
    ///      proposal from a future epoch could be signed under an older one.
    function _trackEpoch() private {
        uint256 current = AMM.pauseEpoch();
        if (current < lastPauseEpoch) pauseEpochWentBackwards = true;
        lastPauseEpoch = current;
    }

    function totalCalls() external view returns (uint256) {
        return callsAddLiquidity + callsSwap + callsBreaker + callsFee + callsStaleProposal;
    }
}
