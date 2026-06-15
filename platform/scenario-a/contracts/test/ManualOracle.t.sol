// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {ManualOracle} from "../src/ManualOracle.sol";
import {IManualOracle} from "../src/interfaces/IManualOracle.sol";
import {DeployManualOracle} from "../script/ManualOracle.s.sol";

/// @title ManualOracleTest
/// @notice Unit tests for the ManualOracle rate-setting oracle contract.
contract ManualOracleTest is Test {
    /// @notice Oracle contract under test
    ManualOracle public oracle;

    /// @notice Test accounts
    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public unauthorized = makeAddr("unauthorized");

    /// @notice Mock token addresses
    address public tokenBRL = makeAddr("tCeBM_BRL");
    address public tokenEUR = makeAddr("tCeBM_EUR");

    /// @notice Test rate: 5.5 BRL per EUR (scaled by 1e18)
    uint256 public testRate = 5_500 * 10 ** 15;

    function setUp() public {
        oracle = new ManualOracle(admin, centralBank);
    }

    function test_SetRate_Success() public {
        vm.prank(centralBank);
        oracle.setRate(tokenBRL, tokenEUR, testRate);

        (uint256 rate, uint8 decimals) = oracle.getRate(tokenBRL, tokenEUR);
        assertEq(rate, testRate);
        assertEq(decimals, 18);
    }

    function test_SetRate_Update() public {
        vm.startPrank(centralBank);
        oracle.setRate(tokenBRL, tokenEUR, testRate);

        uint256 newRate = 6_000 * 10 ** 15;
        oracle.setRate(tokenBRL, tokenEUR, newRate);
        vm.stopPrank();

        (uint256 rate,) = oracle.getRate(tokenBRL, tokenEUR);
        assertEq(rate, newRate);
    }

    function test_Revert_SetRate_Unauthorized() public {
        vm.prank(unauthorized);
        vm.expectRevert();
        oracle.setRate(tokenBRL, tokenEUR, testRate);
    }

    function test_Revert_SetRate_ZeroToken() public {
        vm.prank(centralBank);
        vm.expectRevert(IManualOracle.Oracle__InvalidParameters.selector);
        oracle.setRate(address(0), tokenEUR, testRate);
    }

    function test_Revert_SetRate_ZeroToken1() public {
        vm.prank(centralBank);
        vm.expectRevert(IManualOracle.Oracle__InvalidParameters.selector);
        oracle.setRate(tokenBRL, address(0), testRate);
    }

    function test_Revert_SetRate_ZeroRate() public {
        vm.prank(centralBank);
        vm.expectRevert(IManualOracle.Oracle__InvalidParameters.selector);
        oracle.setRate(tokenBRL, tokenEUR, 0);
    }

    function test_Revert_GetRate_NotSet() public {
        vm.expectRevert(IManualOracle.Oracle__RateNotSet.selector);
        oracle.getRate(tokenBRL, tokenEUR);
    }

    function test_GetRate_PairDirectionality() public {
        vm.prank(centralBank);
        oracle.setRate(tokenBRL, tokenEUR, testRate);

        /// @dev Reverse direction should NOT have a rate set
        vm.expectRevert(IManualOracle.Oracle__RateNotSet.selector);
        oracle.getRate(tokenEUR, tokenBRL);
    }

    function test_ScriptRun_Success() public {
        DeployManualOracle deployScript = new DeployManualOracle();
        deployScript.setUp();

        vm.setEnv("DEPLOYER_PRIVATE_KEY", vm.toString(uint256(0x1)));
        vm.setEnv("ADMIN_ADDRESS", vm.toString(admin));
        vm.setEnv("CENTRAL_BANK_ADDRESS", vm.toString(centralBank));

        deployScript.run();

        assertTrue(address(deployScript.oracle()) != address(0), "ManualOracle was not deployed");
        assertGt(address(deployScript.oracle()).code.length, 0, "ManualOracle has no bytecode");
    }
}
