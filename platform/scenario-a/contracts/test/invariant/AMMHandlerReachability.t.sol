// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {AutomatedMarketMaker} from "../../src/AutomatedMarketMaker.sol";
import {TokenizedCentralBankMoney} from "../../src/TokenizedCentralBankMoney.sol";
import {IdentityRegistry} from "../../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../../src/libraries/IdentityRegistryLibrary.sol";
import {IAutomatedMarketMaker} from "../../src/interfaces/IAutomatedMarketMaker.sol";
import {AMMHandler} from "./AMMHandler.sol";

/// @title AMMHandlerReachabilityTest
/// @notice Proves each action of the Scenario A AMM handler reaches the contract.
///
/// The invariant suite cannot prove this about itself: a handler whose actions all fail
/// silently makes every invariant pass while touching nothing, and looks exactly like
/// success. Scenario B's suite shipped that way for a while — ~1600 fuzzer calls into
/// the swap action and not one swap settled — so each action is pinned here instead.
contract AMMHandlerReachabilityTest is Test {
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
    }

    function _verify(address who, string memory name, IdentityRegistryLibrary.ParticipantRole role) private {
        registry.registerParticipant(who, name, role, bytes32(0), keccak256(abi.encodePacked("inst-", who)));
        registry.verifyParticipant(who);
    }

    function test_addLiquidityIsReachable() public {
        handler.addLiquidity(1_000 ether, 1_000 ether);
        assertEq(handler.callsAddLiquidity(), 1, "addLiquidity never succeeded");
        assertGt(handler.ghostDepositedA(), 0, "the deposit was not tracked");
    }

    function test_swapIsReachableInBothDirections() public {
        handler.addLiquidity(500_000 ether, 500_000 ether);
        handler.swapExactOut(true, 1_000 ether);
        handler.swapExactOut(false, 1_000 ether);
        assertEq(
            handler.callsSwap(),
            2,
            string.concat("a swap direction is unreachable; last revert: ", vm.toString(handler.lastSwapRevert()))
        );
        assertGt(handler.ghostSwappedInA(), 0, "the swap charge was not tracked");
    }

    function test_breakerPauseAndResumeAreBothReachable() public {
        handler.toggleBreaker(0); // seed % 8 == 0 takes the pause branch
        assertTrue(amm.paused(), "pause is unreachable through the handler");
        uint256 epochWhilePaused = amm.pauseEpoch();
        handler.toggleBreaker(0);
        assertFalse(amm.paused(), "resume is unreachable (quorum never reached)");
        assertEq(amm.pauseEpoch(), epochWhilePaused, "resume must not open a new epoch");
    }

    function test_feeChangeIsReachable() public {
        handler.setFee(50);
        assertGt(handler.callsFee(), 0, "fee change never succeeded");
        assertLe(amm.feeBps(), amm.MAX_FEE_BPS(), "fee exceeded the cap");
    }

    /// @notice The stale-proposal action must actually reach the epoch guard: raise a
    ///         proposal, resume, pause again, then try to sign the old one. If the
    ///         sequence bails early the invariant built on it proves nothing.
    function test_staleProposalPathReachesTheEpochGuard() public {
        handler.signStaleProposal(0);
        assertEq(handler.callsStaleProposal(), 1, "the stale-proposal path never reached the second signature");
        assertFalse(handler.staleProposalResumed(), "a stale proposal resumed the AMM");
        assertTrue(amm.paused(), "the AMM should still be paused after a rejected stale signature");
        assertGe(amm.pauseEpoch(), 2, "the path must have opened at least two pause epochs");
        // The REASON matters as much as the rejection. The first version of this path
        // reused GOV_B for both the quorum signature and the stale attempt, so it was
        // rejected by the already-signed guard and never reached the epoch guard —
        // removing the epoch check from the contract changed nothing. Asserting the
        // selector is what makes this a test of the rule it names.
        assertEq(
            bytes4(handler.lastStaleSignRevert()),
            IAutomatedMarketMaker.AMM__ProposalExpired.selector,
            "the stale signature was rejected for the wrong reason; the epoch guard was never reached"
        );
    }

    function test_pauseEpochTrackingIsWired() public {
        assertEq(handler.lastPauseEpoch(), 0, "epoch tracking did not start at zero");
        handler.toggleBreaker(0);
        handler.addLiquidity(1 ether, 1 ether); // any action refreshes the tracker
        assertEq(handler.lastPauseEpoch(), amm.pauseEpoch(), "the handler stopped tracking pauseEpoch");
        assertFalse(handler.pauseEpochWentBackwards(), "false positive on a clean run");
    }
}
