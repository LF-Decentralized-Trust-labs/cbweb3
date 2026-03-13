// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {AutomatedMarketMakerLibrary} from "../src/libraries/AutomatedMarketMakerLibrary.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {DeployAMM} from "../script/AutomatedMarketMaker.s.sol";
import {IAccessControl} from "@openzeppelin-contracts/access/IAccessControl.sol";

/// @title AutomatedMarketMakerTest
/// @notice Unit tests for the AutomatedMarketMaker constant product AMM contract.
/// @dev Tests cover liquidity provision, token swaps, slippage protection, and circuit breaker functionality.
contract AutomatedMarketMakerTest is Test {
    /// @notice AMM contract under test
    AutomatedMarketMaker public amm;

    /// @notice Mock tokens representing tokenised central bank money pairs
    TokenizedCentralBankMoney public tokenA;
    TokenizedCentralBankMoney public tokenB;

    /// @notice Test accounts
    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public governance = makeAddr("governance");
    address public liquidityProvider = makeAddr("liquidityProvider");
    address public swapper = makeAddr("swapper");

    /// @notice Default amounts for test scenarios
    uint256 public constant INITIAL_LIQUIDITY = 100_000 * 10 ** 18;
    uint256 public constant SWAPPER_BALANCE = 10_000 * 10 ** 18;

    /// @notice Deploys and configures test fixtures for AMM flows.
    /// @dev Mints tokens to liquidity provider and swapper, sets allowances for the AMM contract.
    function setUp() public {
        /// @dev 1. Deploy the two tCeBM mock assets
        tokenA = new TokenizedCentralBankMoney("Token BRL", "tCeBM_BRL", admin, centralBank);
        tokenB = new TokenizedCentralBankMoney("Token EUR", "tCeBM_EUR", admin, centralBank);

        /// @dev 2. Deploy the AMM
        amm = new AutomatedMarketMaker(address(tokenA), address(tokenB), admin, governance);

        /// @dev 3. Fund the liquidity provider and swapper
        vm.startPrank(centralBank);
        tokenA.mint(liquidityProvider, INITIAL_LIQUIDITY);
        tokenB.mint(liquidityProvider, INITIAL_LIQUIDITY);
        tokenA.mint(swapper, SWAPPER_BALANCE);
        tokenB.mint(swapper, SWAPPER_BALANCE);
        vm.stopPrank();

        /// @dev 4. Approve the AMM to spend tokens
        vm.startPrank(liquidityProvider);
        tokenA.approve(address(amm), type(uint256).max);
        tokenB.approve(address(amm), type(uint256).max);
        vm.stopPrank();

        vm.startPrank(swapper);
        tokenA.approve(address(amm), type(uint256).max);
        tokenB.approve(address(amm), type(uint256).max);
        vm.stopPrank();
    }

    /// @dev Test that adding liquidity succeeds and updates reserves correctly.
    function test_AddLiquidity_Success() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        assertEq(amm.reserveA(), INITIAL_LIQUIDITY);
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY);
        assertEq(tokenA.balanceOf(address(amm)), INITIAL_LIQUIDITY);
        assertEq(tokenB.balanceOf(address(amm)), INITIAL_LIQUIDITY);
    }

    /// @dev Test that adding liquidity reverts when amount is zero.
    function test_Revert_AddLiquidity_ZeroAmount() public {
        vm.prank(liquidityProvider);
        vm.expectRevert(AutomatedMarketMakerLibrary.AMM__ZeroAmount.selector);
        amm.addLiquidity(0, INITIAL_LIQUIDITY);
    }

    /// @dev Test the constant product formula for calculating required input amount.
    function test_GetAmountIn_Math() public view {
        uint256 reserveIn = 1000 * 10 ** 18;
        uint256 reserveOut = 1000 * 10 ** 18;
        uint256 amountOut = 100 * 10 ** 18;

        /// @dev Formula: (x * dy) / (y - dy) + 1
        /// @dev (1000 * 100) / (1000 - 100) = 100000 / 900 = 111.11...
        uint256 expectedIn = ((reserveIn * amountOut) / (reserveOut - amountOut)) + 1;
        uint256 actualIn = amm.getAmountIn(reserveIn, reserveOut, amountOut);

        assertEq(actualIn, expectedIn);
    }

    /// @dev Test that `getAmountIn` reverts when attempting to withdraw >= reserve.
    function test_Revert_GetAmountIn_InsufficientLiquidity() public {
        vm.expectRevert(AutomatedMarketMakerLibrary.AMM__InsufficientLiquidity.selector);
        amm.getAmountIn(1000, 1000, 1000);
    }

    /// @dev Test that swapping tokens for exact output succeeds and updates balances correctly.
    function test_SwapTokensForExactTokens_Success() public {
        /// @dev Setup: Add liquidity first
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 calculatedAmountIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 maxAmountIn = calculatedAmountIn;

        uint256 swapperBalanceBef = tokenA.balanceOf(swapper);
        uint256 swapperBalanceOutBef = tokenB.balanceOf(swapper);

        /// @dev Execute Swap
        vm.prank(swapper);
        uint256 actualIn =
            amm.swapTokensForExactTokens(address(tokenA), address(tokenB), amountOutDesired, maxAmountIn, swapper);

        /// @dev Assertions
        assertEq(actualIn, calculatedAmountIn, "Amount In mismatch");
        assertEq(tokenA.balanceOf(swapper), swapperBalanceBef - actualIn, "Sender TokenA balance incorrect");
        assertEq(tokenB.balanceOf(swapper), swapperBalanceOutBef + amountOutDesired, "Sender TokenB balance incorrect");
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY + actualIn, "ReserveA not updated");
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY - amountOutDesired, "ReserveB not updated");
    }

    /// @dev Test that swap reverts when slippage tolerance is exceeded.
    function test_Revert_Swap_SlippageExceeded() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 calculatedAmountIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);

        /// @dev Simulate user only accepting less than market requires
        uint256 maxAmountIn = calculatedAmountIn - 1;

        vm.prank(swapper);
        vm.expectRevert(
            abi.encodeWithSelector(
                AutomatedMarketMakerLibrary.AMM__SlippageExceeded.selector, calculatedAmountIn, maxAmountIn
            )
        );
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), amountOutDesired, maxAmountIn, swapper);
    }

    /// @dev Test that swap reverts when invalid token pair is provided.
    function test_Revert_Swap_InvalidToken() public {
        vm.prank(swapper);
        vm.expectRevert(AutomatedMarketMakerLibrary.AMM__InvalidToken.selector);
        amm.swapTokensForExactTokens(address(tokenA), address(tokenA), 100, 100, swapper);
    }

    /// @dev Test that circuit breaker pauses the contract successfully.
    function test_CircuitBreaker_Pause_Success() public {
        /// @dev Pause the contract using the governance account
        vm.prank(governance);
        amm.setPause(true);

        assertTrue(amm.paused());

        /// @dev Attempting to add liquidity should fail with OpenZeppelin's EnforcedPause error
        vm.prank(liquidityProvider);
        vm.expectRevert();
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);
    }

    /// @dev Test that circuit breaker unpauses the contract successfully.
    function test_CircuitBreaker_Unpause_Success() public {
        /// @dev First pause the contract
        vm.startPrank(governance);
        amm.setPause(true);
        assertTrue(amm.paused());

        /// @dev Then unpause it
        amm.setPause(false);
        vm.stopPrank();

        assertFalse(amm.paused());

        /// @dev Verify operations work again
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY);
    }

    /// @dev Test that circuit breaker reverts when called by unauthorised account.
    function test_Revert_CircuitBreaker_Unauthorized() public {
        /// @dev Attempt to pause with a non-governance account
        vm.expectRevert(
            abi.encodeWithSelector(
                IAccessControl.AccessControlUnauthorizedAccount.selector, swapper, amm.GOVERNANCE_ROLE()
            )
        );
        vm.prank(swapper);
        amm.setPause(true);
    }

    /// @dev Test that swap reverts when amountOut is zero.
    function test_Revert_Swap_ZeroAmount() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        vm.expectRevert(AutomatedMarketMakerLibrary.AMM__ZeroAmount.selector);
        vm.prank(swapper);
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), 0, 100, swapper);
    }

    /// @dev Test that swapping tokenB for tokenA succeeds (reverse direction).
    function test_SwapTokensForExactTokens_ReverseDirection() public {
        /// @dev Setup: Add liquidity first
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 calculatedAmountIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 maxAmountIn = calculatedAmountIn;

        uint256 swapperBalanceBBef = tokenB.balanceOf(swapper);
        uint256 swapperBalanceABef = tokenA.balanceOf(swapper);

        /// @dev Execute Swap: B → A (reverse direction)
        vm.prank(swapper);
        uint256 actualIn =
            amm.swapTokensForExactTokens(address(tokenB), address(tokenA), amountOutDesired, maxAmountIn, swapper);

        /// @dev Assertions
        assertEq(actualIn, calculatedAmountIn, "Amount In mismatch");
        assertEq(tokenB.balanceOf(swapper), swapperBalanceBBef - actualIn, "Sender TokenB balance incorrect");
        assertEq(tokenA.balanceOf(swapper), swapperBalanceABef + amountOutDesired, "Sender TokenA balance incorrect");
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY + actualIn, "ReserveB not updated");
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY - amountOutDesired, "ReserveA not updated");
    }

    /// @dev Test that constructor reverts when tokenA is zero address.
    function test_Revert_Constructor_ZeroAddressTokenA() public {
        vm.expectRevert(AutomatedMarketMakerLibrary.AMM__ZeroAddress.selector);
        new AutomatedMarketMaker(address(0), address(tokenB), admin, governance);
    }

    /// @dev Test that constructor reverts when tokenB is zero address.
    function test_Revert_Constructor_ZeroAddressTokenB() public {
        vm.expectRevert(AutomatedMarketMakerLibrary.AMM__ZeroAddress.selector);
        new AutomatedMarketMaker(address(tokenA), address(0), admin, governance);
    }
}

/// @title DeployAMMTest
/// @notice Unit tests for the DeployAMM deployment script.
/// @dev Tests verify that the AMM contract is deployed correctly with proper configuration.
contract DeployAMMTest is Test {
    /// @notice Test deployment of AutomatedMarketMaker via script
    DeployAMM public deployScript;

    /// @dev Deployment configuration read from environment
    uint256 private deployerPrivateKey;
    address private expectedDeployer;
    address private expectedTokenA;
    address private expectedTokenB;
    address private expectedAdmin;
    address private expectedGovernance;

    /// @dev Environment variable names
    string private constant ENV_DEPLOYER_PRIVATE_KEY = "DEPLOYER_PRIVATE_KEY";
    string private constant ENV_TOKEN_A_ADDRESS = "TOKEN_A_ADDRESS";
    string private constant ENV_TOKEN_B_ADDRESS = "TOKEN_B_ADDRESS";
    string private constant ENV_ADMIN_ADDRESS = "ADMIN_ADDRESS";
    string private constant ENV_GOVERNANCE_ADDRESS = "GOVERNANCE_ADDRESS";

    /// @dev Role identifiers for RBAC
    bytes32 private constant DEFAULT_ADMIN_ROLE = 0x00;
    bytes32 private constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");

    /// @notice Sets up the test environment by instantiating the deployment script.
    /// @dev Reads deployment parameters from environment variables.
    function setUp() public {
        deployScript = new DeployAMM();
        deployScript.setUp();

        /// @dev Read script inputs from .env or use default test values
        deployerPrivateKey = vm.envOr(ENV_DEPLOYER_PRIVATE_KEY, uint256(0x1));
        expectedDeployer = vm.addr(deployerPrivateKey);
        expectedTokenA = vm.envOr(ENV_TOKEN_A_ADDRESS, address(0x3456789012345678901234567890123456789012));
        expectedTokenB = vm.envOr(ENV_TOKEN_B_ADDRESS, address(0x4567890123456789012345678901234567890123));
        expectedAdmin = vm.envOr(ENV_ADMIN_ADDRESS, address(0x1234567890123456789012345678901234567890));
        expectedGovernance = vm.envOr(ENV_GOVERNANCE_ADDRESS, address(0x5678901234567890123456789012345678901234));

        /// @dev Set environment variables for the script if not already set
        vm.setEnv(ENV_DEPLOYER_PRIVATE_KEY, vm.toString(deployerPrivateKey));
        vm.setEnv(ENV_TOKEN_A_ADDRESS, vm.toString(expectedTokenA));
        vm.setEnv(ENV_TOKEN_B_ADDRESS, vm.toString(expectedTokenB));
        vm.setEnv(ENV_ADMIN_ADDRESS, vm.toString(expectedAdmin));
        vm.setEnv(ENV_GOVERNANCE_ADDRESS, vm.toString(expectedGovernance));
    }

    /// @dev The script should successfully deploy the AMM contract using env vars.
    function test_ScriptRun_Success() public {
        /// @dev Act
        deployScript.run();
        AutomatedMarketMaker amm = deployScript.amm();

        /// @dev Assert: contract deployment
        assertTrue(address(amm) != address(0), "Contract was not deployed");
        assertGt(address(amm).code.length, 0, "Deployed contract has no runtime bytecode");

        /// @dev Assert: token pair configuration
        assertEq(address(amm.TOKEN_A()), expectedTokenA, "TokenA address mismatch");
        assertEq(address(amm.TOKEN_B()), expectedTokenB, "TokenB address mismatch");

        /// @dev Assert: RBAC seeded from env values
        assertTrue(amm.hasRole(DEFAULT_ADMIN_ROLE, expectedAdmin), "Admin role not granted to expected address");
        assertTrue(amm.hasRole(GOVERNANCE_ROLE, expectedGovernance), "Governance role not granted to expected address");

        /// @dev Assert: deployer does not receive privileged roles by default
        assertFalse(amm.hasRole(DEFAULT_ADMIN_ROLE, expectedDeployer), "Deployer should not be admin");
        assertFalse(amm.hasRole(GOVERNANCE_ROLE, expectedDeployer), "Deployer should not be governance");

        /// @dev Assert: initial reserves are zero
        assertEq(amm.reserveA(), 0, "Initial reserveA should be zero");
        assertEq(amm.reserveB(), 0, "Initial reserveB should be zero");

        /// @dev Assert: contract is not paused initially
        assertFalse(amm.paused(), "Contract should not be paused initially");

        /// @dev Assert: ensure env-derived deployer address is valid
        assertTrue(expectedDeployer != address(0), "Expected deployer should not be zero address");
    }
}
