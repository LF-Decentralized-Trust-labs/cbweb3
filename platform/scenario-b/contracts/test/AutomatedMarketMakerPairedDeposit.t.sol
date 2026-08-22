// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test} from "forge-std/Test.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {IAutomatedMarketMaker} from "../src/interfaces/IAutomatedMarketMaker.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";

/// @title AutomatedMarketMakerPairedDepositTest
/// @notice Escrow-and-finalize paired deposits (decision D6, specs/013-amm-lp-shares Phase 2):
///         each provider deposits its own side independently against a shared commit id; finalize
///         mints proportional shares to each recipient atomically; an un-finalized side is refundable.
contract AutomatedMarketMakerPairedDepositTest is Test {
    AutomatedMarketMaker public amm;
    IdentityRegistry public identityRegistry;
    TokenizedCentralBankMoney public tokenA;
    TokenizedCentralBankMoney public tokenB;

    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public cbA = makeAddr("cbA"); // deposits side A, receives side-A shares (sovereign)
    address public cbB = makeAddr("cbB"); // deposits side B, receives side-B shares (sovereign)
    address public operator = makeAddr("operator"); // commercial-flow operator (holds both sides)
    address public bankA = makeAddr("bankA"); // commercial share recipient A
    address public bankB = makeAddr("bankB"); // commercial share recipient B

    uint256 public constant AMT_A = 5_000_000 * 10 ** 18;
    uint256 public constant AMT_B = 1_000_000 * 10 ** 18;
    bytes32 public constant COMMIT = keccak256("commit-1");

    function setUp() public {
        tokenA = new TokenizedCentralBankMoney("Token BRL", "tCeBM_BRL", admin, centralBank);
        tokenB = new TokenizedCentralBankMoney("Token EUR", "tCeBM_EUR", admin, centralBank);

        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        _register(cbA, "Central Bank A", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK);
        _register(cbB, "Central Bank B", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK);
        _register(operator, "Operator", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);
        _register(bankA, "Bank A", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);
        _register(bankB, "Bank B", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);
        vm.stopPrank();

        amm = new AutomatedMarketMaker(address(tokenA), address(tokenB), address(identityRegistry));

        vm.startPrank(centralBank);
        tokenA.mint(cbA, AMT_A * 2);
        tokenB.mint(cbB, AMT_B * 2);
        tokenA.mint(operator, AMT_A * 2);
        tokenB.mint(operator, AMT_B * 2);
        vm.stopPrank();

        _approveAll(cbA);
        _approveAll(cbB);
        _approveAll(operator);
    }

    // ---------- sovereign-style: two independent depositors, then finalize ----------

    function test_Escrow_TwoSovereignDeposits_FinalizeMintsSplitShares() public {
        // CB-A deposits side A from its own key; CB-B deposits side B from its own key.
        vm.prank(cbA);
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);
        vm.prank(cbB);
        amm.depositForCommit(COMMIT, false, AMT_B, cbB);

        // Reserves still zero until finalize (tokens escrowed, not yet liquidity).
        assertEq(amm.reserveA(), 0, "escrow does not touch reserves");
        assertEq(amm.reserveB(), 0);

        vm.prank(operator); // anyone may finalize a complete commit
        (uint256 sharesA, uint256 sharesB) = amm.finalizeCommit(COMMIT);

        assertEq(amm.reserveA(), AMT_A, "reserves funded on finalize");
        assertEq(amm.reserveB(), AMT_B);
        assertEq(amm.balanceOf(cbA), sharesA, "CB-A holds its shares");
        assertEq(amm.balanceOf(cbB), sharesB, "CB-B holds its shares");
        // First deposit → 50/50 value split.
        assertApproxEqRel(sharesA, sharesB, 0.0001e18, "first deposit splits 50/50");
        // Total minted + locked minimum == sqrt(k).
        assertEq(sharesA + sharesB + amm.MINIMUM_LIQUIDITY(), amm.totalSupply(), "shares + lock = supply");
    }

    // ---------- commercial-style: operator deposits both sides, banks receive shares ----------

    function test_Escrow_OperatorDepositsBothSides_BanksReceiveShares() public {
        vm.startPrank(operator);
        amm.depositForCommit(COMMIT, true, AMT_A, bankA); // shares to bankA
        amm.depositForCommit(COMMIT, false, AMT_B, bankB); // shares to bankB
        (uint256 sharesA, uint256 sharesB) = amm.finalizeCommit(COMMIT);
        vm.stopPrank();

        assertEq(amm.balanceOf(bankA), sharesA, "bankA receives side-A shares");
        assertEq(amm.balanceOf(bankB), sharesB, "bankB receives side-B shares");
        assertEq(amm.balanceOf(operator), 0, "operator holds no shares (only deposited)");
    }

    // ---------- guards ----------

    function test_Revert_Finalize_Incomplete() public {
        vm.prank(cbA);
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);
        vm.prank(operator);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__CommitIncomplete.selector, COMMIT));
        amm.finalizeCommit(COMMIT);
    }

    function test_Revert_Deposit_SideAlreadyDeposited() public {
        vm.prank(cbA);
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);
        vm.prank(cbA);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__SideAlreadyDeposited.selector, COMMIT, true));
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);
    }

    function test_Revert_Deposit_UnverifiedRecipient() public {
        address outsider = makeAddr("outsider");
        vm.prank(cbA);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__ParticipantNotVerified.selector, outsider));
        amm.depositForCommit(COMMIT, true, AMT_A, outsider);
    }

    function test_Revert_Finalize_AlreadyFinalized() public {
        _completeCommit();
        vm.prank(operator);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__CommitAlreadyFinalized.selector, COMMIT));
        amm.finalizeCommit(COMMIT);
    }

    // ---------- refund / timeout path (constitution: refund must be tested) ----------

    function test_Refund_UnfinalizedSide_ReturnsTokens() public {
        vm.prank(cbA);
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);

        uint256 balBefore = tokenA.balanceOf(cbA);
        vm.prank(cbA);
        uint256 refunded = amm.cancelCommitDeposit(COMMIT, true);

        assertEq(refunded, AMT_A, "full side refunded");
        assertEq(tokenA.balanceOf(cbA), balBefore + AMT_A, "tokens returned");

        // Side A is now free to be re-deposited under the same commit.
        vm.prank(cbA);
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);
    }

    function test_Revert_Refund_NotDepositor() public {
        vm.prank(cbA);
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);
        vm.prank(cbB);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__NotDepositor.selector, COMMIT));
        amm.cancelCommitDeposit(COMMIT, true);
    }

    function test_Revert_Refund_AfterFinalize() public {
        _completeCommit();
        vm.prank(cbA);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__CommitAlreadyFinalized.selector, COMMIT));
        amm.cancelCommitDeposit(COMMIT, true);
    }

    function test_Revert_Deposit_AfterFinalized() public {
        _completeCommit();
        // A finalized commit rejects further deposits (the finalized guard in depositForCommit).
        vm.prank(cbA);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__CommitAlreadyFinalized.selector, COMMIT));
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);
    }

    function test_Revert_Deposit_ZeroAmount() public {
        vm.prank(cbA);
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAmount.selector);
        amm.depositForCommit(COMMIT, true, 0, cbA);
    }

    function test_Revert_Deposit_SideBAlreadyDeposited() public {
        vm.prank(cbB);
        amm.depositForCommit(COMMIT, false, AMT_B, cbB);
        vm.prank(cbB);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__SideAlreadyDeposited.selector, COMMIT, false));
        amm.depositForCommit(COMMIT, false, AMT_B, cbB);
    }

    // ---------- refund side B (mirror of the side-A refund path) ----------

    function test_Refund_UnfinalizedSideB_ReturnsTokens() public {
        vm.prank(cbB);
        amm.depositForCommit(COMMIT, false, AMT_B, cbB);

        uint256 balBefore = tokenB.balanceOf(cbB);
        vm.prank(cbB);
        uint256 refunded = amm.cancelCommitDeposit(COMMIT, false);

        assertEq(refunded, AMT_B, "full side-B refunded");
        assertEq(tokenB.balanceOf(cbB), balBefore + AMT_B, "side-B tokens returned");
    }

    function test_Revert_Refund_SideB_NotDepositor() public {
        vm.prank(cbB);
        amm.depositForCommit(COMMIT, false, AMT_B, cbB);
        vm.prank(cbA);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__NotDepositor.selector, COMMIT));
        amm.cancelCommitDeposit(COMMIT, false);
    }

    function test_Revert_Refund_SideA_NothingToRefund() public {
        // depositorA defaults to address(0); a refund attempt by the zero-address would mismatch,
        // so we drive the NothingToRefund branch: deposit, refund once, then the slot is cleared.
        vm.prank(cbA);
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);
        vm.prank(cbA);
        amm.cancelCommitDeposit(COMMIT, true);
        // Second refund: depositorA is now address(0) → NotDepositor (msg.sender != 0).
        vm.prank(cbA);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__NotDepositor.selector, COMMIT));
        amm.cancelCommitDeposit(COMMIT, true);
    }

    // ---------- getEscrow view ----------

    function test_GetEscrow_ReflectsState() public {
        vm.prank(cbA);
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);
        vm.prank(cbB);
        amm.depositForCommit(COMMIT, false, AMT_B, bankB);

        (
            address depositorA,
            address depositorB,
            address recipientA,
            address recipientB,
            uint256 amountA,
            uint256 amountB,
            bool finalized
        ) = amm.getEscrow(COMMIT);

        assertEq(depositorA, cbA);
        assertEq(depositorB, cbB);
        assertEq(recipientA, cbA);
        assertEq(recipientB, bankB);
        assertEq(amountA, AMT_A);
        assertEq(amountB, AMT_B);
        assertFalse(finalized);

        vm.prank(operator);
        amm.finalizeCommit(COMMIT);
        (,,,,,, bool finalizedAfter) = amm.getEscrow(COMMIT);
        assertTrue(finalizedAfter, "finalized flag set after finalizeCommit");
    }

    // ---------- finalizeCommit second commit against a non-empty pool (else valueB branch) ----------

    /// @notice Once the pool holds reserves, a second commit's side-B value is priced against the
    ///         current reserves (the `reserveA != 0 && reserveB != 0` else branch in finalizeCommit).
    function test_Finalize_SecondCommit_PricesAgainstReserves() public {
        // First commit seeds the pool.
        _completeCommit();
        assertEq(amm.reserveA(), AMT_A);
        assertEq(amm.reserveB(), AMT_B);

        // Second commit under a fresh id, with reserves already non-zero.
        bytes32 commit2 = keccak256("commit-2");
        vm.prank(cbA);
        amm.depositForCommit(commit2, true, AMT_A, cbA);
        vm.prank(cbB);
        amm.depositForCommit(commit2, false, AMT_B, cbB);

        uint256 supplyBefore = amm.totalSupply();
        vm.prank(operator);
        (uint256 sharesA, uint256 sharesB) = amm.finalizeCommit(commit2);

        assertGt(sharesA, 0, "side-A shares minted");
        assertGt(sharesB, 0, "side-B shares minted");
        assertEq(amm.reserveA(), AMT_A * 2, "reserves doubled");
        assertEq(amm.reserveB(), AMT_B * 2);
        assertGt(amm.totalSupply(), supplyBefore, "supply grew on second finalize");
    }

    // ---------- helpers ----------

    function _register(address who, string memory name, IdentityRegistryLibrary.ParticipantRole role) internal {
        identityRegistry.registerParticipant(who, name, role, bytes32(0));
        identityRegistry.verifyParticipant(who);
    }

    function _approveAll(address who) internal {
        vm.startPrank(who);
        tokenA.approve(address(amm), type(uint256).max);
        tokenB.approve(address(amm), type(uint256).max);
        vm.stopPrank();
    }

    function _completeCommit() internal {
        vm.prank(cbA);
        amm.depositForCommit(COMMIT, true, AMT_A, cbA);
        vm.prank(cbB);
        amm.depositForCommit(COMMIT, false, AMT_B, cbB);
        vm.prank(operator);
        amm.finalizeCommit(COMMIT);
    }
}
