// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {IAutomatedMarketMaker} from "../src/interfaces/IAutomatedMarketMaker.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";

/// @title AutomatedMarketMakerCooperativeTest
/// @notice Tests for 005-cooperative-liquidity: single-sided deposits, feeBps, removeSingleSided.
contract AutomatedMarketMakerCooperativeTest is Test {
    // Redeclare events for expectEmit (matches IAutomatedMarketMaker signatures)
    event LogSingleSidedLiquidityAdded(address indexed provider, bool isTokenA, uint256 amount);
    event LogSingleSidedLiquidityRemoved(address indexed provider, bool isTokenA, uint256 amount);
    AutomatedMarketMaker public amm;
    IdentityRegistry public identityRegistry;
    TokenizedCentralBankMoney public tokenA;
    TokenizedCentralBankMoney public tokenB;

    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public lpA = makeAddr("lpA");  // will provide side A (TOKEN_A)
    address public lpB = makeAddr("lpB");  // will provide side B (TOKEN_B)
    address public nonLP = makeAddr("nonLP");
    address public governance = makeAddr("governance");

    uint256 public constant AMOUNT = 50_000 * 10 ** 18;

    function setUp() public {
        tokenA = new TokenizedCentralBankMoney("Token BRL", "tCeBM_BRL", admin, centralBank);
        tokenB = new TokenizedCentralBankMoney("Token USD", "tCeBM_USD", admin, centralBank);

        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            lpA, "LP Bank A", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            lpB, "LP Bank B", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            governance, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            nonLP, "Non LP", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        // Grant LP roles to lpA and lpB
        identityRegistry.grantLiquidityProvider(lpA);
        identityRegistry.grantLiquidityProvider(lpB);
        vm.stopPrank();

        amm = new AutomatedMarketMaker(address(tokenA), address(tokenB), address(identityRegistry));

        vm.startPrank(centralBank);
        tokenA.mint(lpA, AMOUNT * 2);
        tokenB.mint(lpB, AMOUNT * 2);
        tokenA.mint(nonLP, AMOUNT);
        tokenB.mint(nonLP, AMOUNT);
        vm.stopPrank();

        vm.startPrank(lpA);
        tokenA.approve(address(amm), type(uint256).max);
        tokenB.approve(address(amm), type(uint256).max);
        vm.stopPrank();

        vm.startPrank(lpB);
        tokenA.approve(address(amm), type(uint256).max);
        tokenB.approve(address(amm), type(uint256).max);
        vm.stopPrank();

        vm.startPrank(nonLP);
        tokenA.approve(address(amm), type(uint256).max);
        tokenB.approve(address(amm), type(uint256).max);
        vm.stopPrank();
    }

    // ---------- addSingleSidedLiquidity ----------

    function test_addSingleSided_TokenA_success() public {
        vm.prank(lpA);
        amm.addSingleSidedLiquidity(true, AMOUNT);

        assertEq(tokenA.balanceOf(address(amm)), AMOUNT);
        assertEq(tokenB.balanceOf(address(amm)), 0);
    }

    function test_addSingleSided_TokenB_success() public {
        vm.prank(lpB);
        amm.addSingleSidedLiquidity(false, AMOUNT);

        assertEq(tokenB.balanceOf(address(amm)), AMOUNT);
        assertEq(tokenA.balanceOf(address(amm)), 0);
    }

    function test_addSingleSided_emitsEvent() public {
        vm.expectEmit(true, true, false, true);
        emit LogSingleSidedLiquidityAdded(lpA, true, AMOUNT);

        vm.prank(lpA);
        amm.addSingleSidedLiquidity(true, AMOUNT);
    }

    function test_addSingleSided_nonLP_reverts() public {
        vm.prank(nonLP);
        vm.expectRevert();
        amm.addSingleSidedLiquidity(true, AMOUNT);
    }

    function test_addSingleSided_zeroAmount_reverts() public {
        vm.prank(lpA);
        vm.expectRevert();
        amm.addSingleSidedLiquidity(true, 0);
    }

    // ---------- feeBps ----------

    function test_feeBps_defaultIs30() public view {
        assertEq(amm.feeBps(), 30);
    }

    function test_setFeeBps_governance_succeeds() public {
        // Add governance to AMM governance list first
        vm.prank(governance);
        amm.setFeeBps(50);
        assertEq(amm.feeBps(), 50);
    }

    function test_setFeeBps_tooHigh_reverts() public {
        vm.prank(governance);
        vm.expectRevert();
        amm.setFeeBps(1001);
    }

    function test_setFeeBps_nonGovernance_reverts() public {
        vm.prank(lpA);
        vm.expectRevert();
        amm.setFeeBps(50);
    }

    function test_setFeeBps_maxBoundary() public {
        vm.prank(governance);
        amm.setFeeBps(1000); // max allowed = 10%
        assertEq(amm.feeBps(), 1000);
    }

    // ---------- removeSingleSidedLiquidity ----------

    function test_removeSingleSided_TokenA_success() public {
        vm.prank(lpA);
        amm.addSingleSidedLiquidity(true, AMOUNT);

        uint256 balBefore = tokenA.balanceOf(lpA);
        vm.prank(lpA);
        amm.removeSingleSidedLiquidity(true, AMOUNT);

        assertEq(tokenA.balanceOf(lpA), balBefore + AMOUNT);
        assertEq(tokenA.balanceOf(address(amm)), 0);
    }

    function test_removeSingleSided_TokenB_success() public {
        vm.prank(lpB);
        amm.addSingleSidedLiquidity(false, AMOUNT);

        uint256 balBefore = tokenB.balanceOf(lpB);
        vm.prank(lpB);
        amm.removeSingleSidedLiquidity(false, AMOUNT);

        assertEq(tokenB.balanceOf(lpB), balBefore + AMOUNT);
        assertEq(tokenB.balanceOf(address(amm)), 0);
    }

    function test_removeSingleSided_emitsEvent() public {
        vm.prank(lpA);
        amm.addSingleSidedLiquidity(true, AMOUNT);

        vm.expectEmit(true, true, false, true);
        emit LogSingleSidedLiquidityRemoved(lpA, true, AMOUNT);

        vm.prank(lpA);
        amm.removeSingleSidedLiquidity(true, AMOUNT);
    }

    function test_removeSingleSided_nonLP_reverts() public {
        vm.prank(lpA);
        amm.addSingleSidedLiquidity(true, AMOUNT);

        vm.prank(nonLP);
        vm.expectRevert();
        amm.removeSingleSidedLiquidity(true, AMOUNT);
    }

    function test_removeSingleSided_insufficientBalance_reverts() public {
        vm.prank(lpA);
        amm.addSingleSidedLiquidity(true, AMOUNT / 2);

        vm.prank(lpA);
        vm.expectRevert();
        amm.removeSingleSidedLiquidity(true, AMOUNT); // more than deposited
    }
}
