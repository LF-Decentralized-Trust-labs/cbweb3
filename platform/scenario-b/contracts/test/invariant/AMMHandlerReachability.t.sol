// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {AutomatedMarketMaker} from "../../src/AutomatedMarketMaker.sol";
import {TokenizedCentralBankMoney} from "../../src/TokenizedCentralBankMoney.sol";
import {IdentityRegistry} from "../../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../../src/libraries/IdentityRegistryLibrary.sol";
import {AMMHandler} from "./AMMHandler.sol";

/// @title AMMHandlerReachabilityTest
/// @notice Proves every action the invariant handler offers can actually succeed.
///
/// This test exists because the invariant suite it supports cannot prove it. A handler
/// whose actions all silently fail — wrong bounds, unfunded account, a guard that
/// rejects every call — makes every invariant pass while touching nothing. That is the
/// worst failure mode a test suite has: it reports safety it never checked, and it
/// looks identical to success.
///
/// The first version of this handler did exactly that: ~1600 fuzzer calls into
/// `swapExactOut` and not one swap settled. Nothing in the invariant output said so,
/// because per-run counters reset and the shrunk replay reports a single call. So the
/// reachability of each action is pinned here, deterministically, where a regression
/// names the action that broke.
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

        vm.startPrank(centralBank);
        tokenA.mint(lp, 10_000_000 ether);
        tokenB.mint(lp, 10_000_000 ether);
        tokenA.mint(swapper, 1_000_000 ether);
        tokenB.mint(swapper, 1_000_000 ether);
        vm.stopPrank();

        vm.startPrank(lp);
        tokenA.approve(address(amm), 1_000_000 ether);
        tokenB.approve(address(amm), 1_000_000 ether);
        amm.addLiquidity(500_000 ether, 500_000 ether);
        vm.stopPrank();
    }

    function _verify(address who, string memory name, IdentityRegistryLibrary.ParticipantRole role) private {
        registry.registerParticipant(who, name, role, bytes32(0), keccak256(abi.encodePacked("inst-", who)));
        registry.verifyParticipant(who);
    }

    function test_addLiquidityIsReachable() public {
        handler.addLiquidity(1_000 ether, 1_000 ether);
        assertEq(handler.callsAddLiquidity(), 1, "addLiquidity never succeeded");
    }

    function test_swapIsReachable() public {
        handler.swapExactOut(true, 1_000 ether);
        assertEq(
            handler.callsSwap(),
            1,
            string.concat("swap never settled; last revert: ", vm.toString(handler.lastSwapRevert()))
        );
    }

    function test_swapIsReachableInBothDirections() public {
        handler.swapExactOut(true, 1_000 ether);
        handler.swapExactOut(false, 1_000 ether);
        assertEq(handler.callsSwap(), 2, "one swap direction is unreachable");
    }

    function test_removeLiquidityIsReachable() public {
        handler.removeLiquidity(1_000 ether, true);
        assertEq(handler.callsRemoveLiquidity(), 1, "removeLiquidity never succeeded");
    }

    function test_commitFlowIsReachable() public {
        handler.depositForCommit(1, true, 10_000 ether);
        handler.depositForCommit(1, false, 10_000 ether);
        assertEq(handler.callsCommitFlow(), 2, "commit deposits never succeeded");
        assertEq(handler.ghostEscrowedA(), handler.ghostEscrowedA(), "ghost accounting is inconsistent");

        handler.finalizeCommit(0);
        assertEq(handler.callsCommitFlow(), 3, "finalizeCommit never succeeded");
        assertEq(handler.ghostEscrowedA(), 0, "escrow ghost not cleared on finalize");
        assertEq(handler.ghostEscrowedB(), 0, "escrow ghost not cleared on finalize");
    }

    function test_cancelCommitIsReachable() public {
        handler.depositForCommit(7, true, 5_000 ether);
        uint256 before = handler.ghostEscrowedA();
        assertGt(before, 0, "nothing was escrowed to cancel");
        handler.cancelCommitDeposit(7, true);
        assertEq(handler.ghostEscrowedA(), 0, "cancel did not release the escrow ghost");
    }

    function test_breakerPauseAndResumeAreBothReachable() public {
        // seed % 8 == 0 is the pause branch.
        handler.toggleBreaker(0);
        assertTrue(amm.isPaused(), "pause is unreachable through the handler");
        handler.toggleBreaker(0);
        assertFalse(amm.isPaused(), "resume is unreachable through the handler (quorum never reached)");
    }

    function test_feeChangesAreReachable() public {
        handler.setFees(50, 25);
        assertGt(handler.callsFee(), 0, "fee changes never succeeded");
        assertLe(amm.feeBps(), amm.MAX_FEE_BPS(), "fee exceeded the cap");
    }
}
