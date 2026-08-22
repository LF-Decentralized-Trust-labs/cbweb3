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
/// @notice Invariant suite for the Scenario B AMM.
///
/// Why invariants and not more fuzz tests. The existing fuzz tests each pin one
/// property of one call — `testFuzz_R2H3_SwapPreservesConstantProduct` proves a single
/// swap does not shrink k. Nothing said what happens after a hundred interleaved
/// deposits, withdrawals, commit finalizations, pauses and fee changes, which is
/// exactly where a pool loses track of what it owes. These invariants hold across
/// arbitrary sequences instead of single calls.
///
/// Every invariant below is written to be falsifiable and was checked to fail when the
/// property it guards is broken; a test that cannot fail documents nothing.
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

    function setUp() public {
        tokenA = new TokenizedCentralBankMoney("Token BRL", "tCeBM_BRL", admin, centralBank);
        tokenB = new TokenizedCentralBankMoney("Token EUR", "tCeBM_EUR", admin, centralBank);
        registry = new IdentityRegistry(admin);

        vm.startPrank(admin);
        _verify(lp, "LP Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);
        _verify(swapper, "Swapper Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);
        _verify(govA, "Central Bank A", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK);
        _verify(govB, "Central Bank B", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK);
        vm.stopPrank();

        amm = new AutomatedMarketMaker(address(tokenA), address(tokenB), address(registry));
        handler = new AMMHandler(amm, tokenA, tokenB, lp, swapper, govA, govB);

        // The handler acts through verified accounts, so the accounts hold the funds.
        vm.startPrank(centralBank);
        tokenA.mint(lp, 10_000_000 ether);
        tokenB.mint(lp, 10_000_000 ether);
        tokenA.mint(swapper, 1_000_000 ether);
        tokenB.mint(swapper, 1_000_000 ether);
        vm.stopPrank();

        // Seed the pool so swaps are reachable from the first run. Without this the
        // fuzzer spends most sequences bouncing off the empty-pool guard and the suite
        // passes without having swapped once.
        vm.startPrank(lp);
        tokenA.approve(address(amm), 1_000_000 ether);
        tokenB.approve(address(amm), 1_000_000 ether);
        amm.addLiquidity(500_000 ether, 500_000 ether);
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

    /// @notice The pool must always hold at least what it owes: the reserves it
    ///         accounts for, plus commit deposits it is holding in escrow.
    ///
    /// This is the invariant that would catch the worst class of bug — escrowed money
    /// being counted as reserves, or reserves being paid out twice. The AMM keeps
    /// escrows in a private per-commitId mapping with no running total, so the
    /// handler's ghost accounting supplies the second term; it is the only way to state
    /// solvency from outside the contract.
    function invariant_poolIsSolvent() public view {
        assertGe(
            tokenA.balanceOf(address(amm)),
            amm.reserveA() + handler.ghostEscrowedA(),
            "token A balance is below reserves + escrow"
        );
        assertGe(
            tokenB.balanceOf(address(amm)),
            amm.reserveB() + handler.ghostEscrowedB(),
            "token B balance is below reserves + escrow"
        );
    }

    /// @notice A swap must never shrink the constant product. The fee stays in the
    ///         reserves, so k grows with a fee and holds without one; a k that fell
    ///         means value left the pool on a trade.
    function invariant_swapsNeverShrinkK() public view {
        assertFalse(handler.swapShrankK(), "a swap reduced reserveA * reserveB");
    }

    /// @notice Shares and reserves agree about whether the pool exists. Shares
    ///         outstanding against an empty side would let the next depositor mint
    ///         against nothing, which is the share-inflation shape MINIMUM_LIQUIDITY
    ///         exists to prevent.
    function invariant_sharesAndReservesAgree() public view {
        if (amm.totalSupply() == 0) {
            assertEq(amm.reserveA(), 0, "shares are gone but reserve A remains");
            assertEq(amm.reserveB(), 0, "shares are gone but reserve B remains");
        } else {
            assertGt(amm.reserveA(), 0, "shares outstanding against an empty reserve A");
            assertGt(amm.reserveB(), 0, "shares outstanding against an empty reserve B");
        }
    }

    /// @notice Once seeded, the locked minimum liquidity is never recoverable and the
    ///         supply never drops below it.
    function invariant_minimumLiquidityStaysLocked() public view {
        if (amm.totalSupply() == 0) return;
        assertGe(amm.totalSupply(), amm.MINIMUM_LIQUIDITY(), "supply fell below the locked minimum");
        assertEq(amm.balanceOf(amm.BURN_ADDRESS()), amm.MINIMUM_LIQUIDITY(), "the locked minimum liquidity moved");
    }

    /// @notice Governance cannot push either fee past the cap, whatever the sequence.
    function invariant_feesStayWithinTheCap() public view {
        assertLe(amm.feeBps(), amm.MAX_FEE_BPS(), "swap fee exceeded the cap");
        assertLe(amm.withdrawalFeeBps(), amm.MAX_FEE_BPS(), "withdrawal fee exceeded the cap");
    }

    /// @notice The circuit breaker means what it says: no swap settles while paused.
    ///         The handler attempts one on every pause, so this is not vacuous.
    function invariant_pausedBlocksSwaps() public view {
        assertFalse(handler.swapSucceededWhilePaused(), "a swap settled while the AMM was paused");
    }

    /// @notice Printed once per run so a reader can see WHICH paths the fuzzer reached.
    ///         A run with 0 swaps still passes every invariant above, and the counts are
    ///         the only way to notice that from the output.
    /// @dev The vacuity check lives here, not in an `invariant_` function: Foundry calls
    ///      every invariant once during environment setup, before any sequence runs, where
    ///      the counters are legitimately zero. Asserting there fails the suite for the
    ///      wrong reason — which is exactly what the first version of this file did.
    function afterInvariant() public view {
        // Counters for the sequence just executed. State resets between runs, so these
        // are per-run figures, not campaign totals — which is why the "did the handler
        // actually do anything" check lives in AMMHandlerReachabilityTest as a
        // deterministic test, not as an assertion here. Requiring a swap in EVERY random
        // sequence fails runs that were simply short, and the shrunk replay then reports
        // one call, hiding the cause.
        console.log("addLiquidity   ", handler.callsAddLiquidity());
        console.log("swap           ", handler.callsSwap());
        console.log("removeLiquidity", handler.callsRemoveLiquidity());
        console.log("commit flow    ", handler.callsCommitFlow());
        console.log("breaker        ", handler.callsBreaker());
        console.log("fee changes    ", handler.callsFee());
        console.log("swap attempted ", handler.swapAttempted());
        console.log("swap bail: no liquidity", handler.swapBailedNoLiquidity());
        console.log("swap bail: unfunded    ", handler.swapBailedUnfunded());
        console.logBytes(handler.lastSwapRevert());
    }
}
