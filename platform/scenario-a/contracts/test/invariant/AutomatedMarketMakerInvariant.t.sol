// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test} from "forge-std/Test.sol";
import {console} from "forge-std/console.sol";
import {StdInvariant} from "forge-std/StdInvariant.sol";
import {AutomatedMarketMaker} from "../../src/AutomatedMarketMaker.sol";
import {TokenizedCentralBankMoney} from "../../src/TokenizedCentralBankMoney.sol";
import {IdentityRegistry} from "../../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../../src/libraries/IdentityRegistryLibrary.sol";
import {AMMHandler} from "./AMMHandler.sol";

/// @title AutomatedMarketMakerInvariantTest
/// @notice Invariant suite for the Scenario A AMM.
///
/// Deliberately NOT a copy of Scenario B's. The two AMMs are different contracts —
/// 304 lines against 558 — and the difference is structural, not cosmetic: Scenario A
/// mints no LP shares, is not an ERC20, has no withdrawal path and no
/// escrow-and-finalize commit flow. So `sharesAndReservesAgree` and
/// `minimumLiquidityStaysLocked` have nothing to attach to here, and copying them
/// would have produced two tests that pass by vacuity.
///
/// What Scenario A has instead is a stronger accounting property and a richer circuit
/// breaker, and both get invariants of their own below. Recording that asymmetry is the
/// point: see docs/scenario-drift.md.
contract AutomatedMarketMakerInvariantTest is StdInvariant, Test {
    AutomatedMarketMaker internal amm;
    IdentityRegistry internal registry;
    TokenizedCentralBankMoney internal tokenA;
    TokenizedCentralBankMoney internal tokenB;
    AMMHandler internal handler;

    address internal admin = makeAddr("admin");
    address internal centralBank = makeAddr("centralBank");
    address internal lp = makeAddr("lp");
    address internal swapper = makeAddr("swapper");
    address internal govA = makeAddr("govA");
    address internal govB = makeAddr("govB");
    address internal govC = makeAddr("govC");

    function setUp() public {
        tokenA = new TokenizedCentralBankMoney("Token BRL", "tCeBM_BRL", admin, centralBank);
        tokenB = new TokenizedCentralBankMoney("Token EUR", "tCeBM_EUR", admin, centralBank);
        registry = new IdentityRegistry(admin);

        vm.startPrank(admin);
        _verify(lp, "LP Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);
        _verify(swapper, "Swapper Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);
        _verify(govA, "Central Bank A", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK);
        _verify(govB, "Central Bank B", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK);
        _verify(govC, "Central Bank C", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK);
        vm.stopPrank();

        amm = new AutomatedMarketMaker(address(tokenA), address(tokenB), address(registry));
        handler = new AMMHandler(amm, tokenA, tokenB, lp, swapper, govA, govB, govC);

        vm.startPrank(centralBank);
        tokenA.mint(lp, 10_000_000 ether);
        tokenB.mint(lp, 10_000_000 ether);
        tokenA.mint(swapper, 1_000_000 ether);
        tokenB.mint(swapper, 1_000_000 ether);
        vm.stopPrank();

        targetContract(address(handler));
    }

    function _verify(address who, string memory name, IdentityRegistryLibrary.ParticipantRole role) private {
        registry.registerParticipant(who, name, role, bytes32(0), keccak256(abi.encodePacked("inst-", who)));
        registry.verifyParticipant(who);
    }

    // ============================================================================
    //                               INVARIANTS
    // ============================================================================

    /// @notice The pool holds at least the reserves it accounts for. There is no escrow
    ///         term here — Scenario A has no commit flow — so this is the whole of it.
    function invariant_poolIsSolvent() public view {
        assertGe(tokenA.balanceOf(address(amm)), amm.reserveA(), "token A balance is below reserve A");
        assertGe(tokenB.balanceOf(address(amm)), amm.reserveB(), "token B balance is below reserve B");
    }

    /// @notice Liquidity is ONE-WAY in this scenario: there is no withdrawal path, so
    ///         the only way a token leaves is as the output side of a swap. That makes
    ///         the balance fully reconstructible — deposits, plus what swaps charged,
    ///         minus what swaps paid out — which is a stronger statement than solvency
    ///         and is only available here.
    ///
    ///         The charge is tracked separately from the payout on purpose: the quote
    ///         rounds the input UP (one ceiling division shared by quote and swap), so
    ///         it is not the mirror image of the output. Treating it as one netted
    ///         figure failed this invariant by exactly 1 wei.
    ///
    ///         Scenario B cannot have this invariant: removeLiquidity and the escrow
    ///         cancel path both move tokens out for reasons unrelated to swaps.
    function invariant_tokensOnlyLeaveThroughSwaps() public view {
        assertEq(
            tokenA.balanceOf(address(amm)),
            handler.ghostDepositedA() + handler.ghostSwappedInA() - handler.ghostPaidOutA(),
            "token A moved by some path other than a deposit or a swap"
        );
        assertEq(
            tokenB.balanceOf(address(amm)),
            handler.ghostDepositedB() + handler.ghostSwappedInB() - handler.ghostPaidOutB(),
            "token B moved by some path other than a deposit or a swap"
        );
    }

    /// @notice A swap must never shrink the constant product; the fee stays in the
    ///         reserves, so k grows or holds.
    function invariant_swapsNeverShrinkK() public view {
        assertFalse(handler.swapShrankK(), "a swap reduced reserveA * reserveB");
    }

    function invariant_feesStayWithinTheCap() public view {
        assertLe(amm.feeBps(), amm.MAX_FEE_BPS(), "swap fee exceeded the cap");
    }

    /// @notice The breaker means what it says, for swaps and for governance alike:
    ///         Scenario A gates setFeeBps on `whenNotPaused` too, which Scenario B does
    ///         not, so a fee change landing while paused is a violation here.
    function invariant_pausedBlocksSwapsAndFeeChanges() public view {
        assertFalse(handler.swapSucceededWhilePaused(), "a swap settled while the AMM was paused");
        assertFalse(handler.feeChangedWhilePaused(), "the fee changed while the AMM was paused");
    }

    /// @notice The pause epoch only moves forward. It is what binds a resume proposal
    ///         to the pause it answers; a counter that could go back would let an old
    ///         proposal match a new epoch.
    function invariant_pauseEpochOnlyMovesForward() public view {
        assertFalse(handler.pauseEpochWentBackwards(), "pauseEpoch decreased");
    }

    /// @notice A resume proposal raised under an earlier epoch can never reopen the
    ///         pool. Without this, a quorum abandoned under a previous pause could be
    ///         completed after a fresh pause and resume it against the current
    ///         governance intent.
    function invariant_staleProposalsCannotResume() public view {
        // Checked on ACCEPTANCE, not on the resume. The contract auto-resumes only when a
        // signature takes the count to quorum, so a stale proposal that already held
        // quorum can be signed again without reopening the pool — and keying this on the
        // resume let a mutation removing the epoch guard pass. The rule being pinned is
        // "a signature from an old epoch is refused", so that is what is asserted.
        assertFalse(
            handler.staleProposalAccepted(), "a signature was accepted for a proposal from an earlier pause epoch"
        );
        assertFalse(handler.staleProposalResumed(), "a proposal from an earlier pause epoch resumed the AMM");
    }

    function afterInvariant() public view {
        console.log("addLiquidity    ", handler.callsAddLiquidity());
        console.log("swap            ", handler.callsSwap());
        console.log("breaker         ", handler.callsBreaker());
        console.log("fee changes     ", handler.callsFee());
        console.log("stale proposals ", handler.callsStaleProposal());
    }
}
