// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {IAutomatedMarketMaker} from "../src/interfaces/IAutomatedMarketMaker.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";

/// @title AutomatedMarketMakerFeesTest
/// @notice Fee-parameter tests (specs/013-amm-lp-shares §2.3): every fee is a governance-settable,
///         bounded, event-emitting parameter — never a hardcoded rate. Covers both the swap fee
///         (`feeBps`) and the withdrawal/zap fee (`withdrawalFeeBps`).
/// @dev The 005-cooperative single-sided deposit tests were removed with their functions; paired
///      cooperative deposits are covered in the Phase 2 paired-deposit suite.
contract AutomatedMarketMakerFeesTest is Test {
    // Redeclared for expectEmit (must match IAutomatedMarketMaker signatures).
    event LogFeeRateUpdated(uint256 oldFeeBps, uint256 newFeeBps);
    event LogWithdrawalFeeRateUpdated(uint256 oldWithdrawalFeeBps, uint256 newWithdrawalFeeBps);

    AutomatedMarketMaker public amm;
    IdentityRegistry public identityRegistry;
    TokenizedCentralBankMoney public tokenA;
    TokenizedCentralBankMoney public tokenB;

    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public governance = makeAddr("governance");
    address public nonGovernance = makeAddr("nonGovernance");

    function setUp() public {
        tokenA = new TokenizedCentralBankMoney("Token BRL", "tCeBM_BRL", admin, centralBank);
        tokenB = new TokenizedCentralBankMoney("Token USD", "tCeBM_USD", admin, centralBank);

        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            governance, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            nonGovernance, "Commercial Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        vm.stopPrank();

        amm = new AutomatedMarketMaker(address(tokenA), address(tokenB), address(identityRegistry));
    }

    // ---------- swap fee (feeBps) ----------

    function test_feeBps_defaultIs30() public view {
        assertEq(amm.feeBps(), 30);
    }

    function test_setFeeBps_governance_succeeds() public {
        vm.prank(governance);
        amm.setFeeBps(50);
        assertEq(amm.feeBps(), 50);
    }

    function test_setFeeBps_emitsEvent() public {
        vm.expectEmit(false, false, false, true);
        emit LogFeeRateUpdated(30, 50);
        vm.prank(governance);
        amm.setFeeBps(50);
    }

    function test_setFeeBps_tooHigh_reverts() public {
        vm.prank(governance);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__FeeBpsTooHigh.selector, 1001, 1000));
        amm.setFeeBps(1001);
    }

    function test_setFeeBps_nonGovernance_reverts() public {
        vm.prank(nonGovernance);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__NotGovernance.selector, nonGovernance));
        amm.setFeeBps(50);
    }

    function test_setFeeBps_maxBoundary() public {
        vm.prank(governance);
        amm.setFeeBps(1000);
        assertEq(amm.feeBps(), 1000);
    }

    // ---------- withdrawal fee (withdrawalFeeBps) ----------

    function test_withdrawalFeeBps_defaultIs30() public view {
        assertEq(amm.withdrawalFeeBps(), 30);
    }

    function test_setWithdrawalFeeBps_governance_succeeds() public {
        vm.prank(governance);
        amm.setWithdrawalFeeBps(0); // governance may make exits fee-free
        assertEq(amm.withdrawalFeeBps(), 0);
    }

    function test_setWithdrawalFeeBps_emitsEvent() public {
        vm.expectEmit(false, false, false, true);
        emit LogWithdrawalFeeRateUpdated(30, 75);
        vm.prank(governance);
        amm.setWithdrawalFeeBps(75);
    }

    function test_setWithdrawalFeeBps_tooHigh_reverts() public {
        vm.prank(governance);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__FeeBpsTooHigh.selector, 1001, 1000));
        amm.setWithdrawalFeeBps(1001);
    }

    function test_setWithdrawalFeeBps_nonGovernance_reverts() public {
        vm.prank(nonGovernance);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__NotGovernance.selector, nonGovernance));
        amm.setWithdrawalFeeBps(50);
    }

    function test_feeAndWithdrawalFee_areIndependent() public {
        vm.startPrank(governance);
        amm.setFeeBps(40);
        amm.setWithdrawalFeeBps(10);
        vm.stopPrank();
        assertEq(amm.feeBps(), 40, "swap fee");
        assertEq(amm.withdrawalFeeBps(), 10, "withdrawal fee independent");
    }
}
