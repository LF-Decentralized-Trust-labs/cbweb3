// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {IAutomatedMarketMaker} from "../src/interfaces/IAutomatedMarketMaker.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {DeployAMM} from "../script/AutomatedMarketMaker.s.sol";

/// @title AutomatedMarketMakerTest
/// @notice Unit tests for the AutomatedMarketMaker constant product AMM contract.
/// @dev Tests cover liquidity provision, token swaps, slippage protection, and circuit breaker functionality.
contract AutomatedMarketMakerTest is Test {
    /// @notice AMM contract under test
    AutomatedMarketMaker public amm;

    /// @notice Identity Registry for clearance gate tests
    IdentityRegistry public identityRegistry;

    /// @notice Mock tokens representing tokenised central bank money pairs
    TokenizedCentralBankMoney public tokenA;
    TokenizedCentralBankMoney public tokenB;

    /// @notice Test accounts
    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public governance = makeAddr("governance");
    address public governance2 = makeAddr("governance2");
    address public liquidityProvider = makeAddr("liquidityProvider");
    address public swapper = makeAddr("swapper");

    /// @notice Default amounts for test scenarios
    uint256 public constant INITIAL_LIQUIDITY = 100_000 * 10 ** 18;
    uint256 public constant SWAPPER_BALANCE = 10_000 * 10 ** 18;

    /// @notice Default swap fee (0.3%) charged by the ported fee model.
    uint256 public constant FEE_BPS = 30;

    /// @dev Grosses up a pre-fee constant-product input by the swap fee, mirroring the contract:
    ///      grossIn = (preFeeIn * 10000) / (10000 - feeBps) + 1. The fee stays in the reserves.
    function _grossIn(uint256 preFeeIn) internal pure returns (uint256) {
        return (preFeeIn * 10000) / (10000 - FEE_BPS) + 1;
    }

    /// @notice Deploys and configures test fixtures for AMM flows.
    /// @dev Mints tokens to liquidity provider and swapper, sets allowances for the AMM contract.
    function setUp() public {
        /// @dev 1. Deploy the two tCeBM mock assets
        tokenA = new TokenizedCentralBankMoney("Token BRL", "tCeBM_BRL", admin, centralBank);
        tokenB = new TokenizedCentralBankMoney("Token EUR", "tCeBM_EUR", admin, centralBank);

        /// @dev 2. Deploy the IdentityRegistry and register test participants
        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            liquidityProvider, "LP Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            swapper, "Commercial Bank A", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            governance, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            governance2, "Central Bank B", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        vm.stopPrank();

        /// @dev 3. Deploy the AMM
        amm = new AutomatedMarketMaker(address(tokenA), address(tokenB), address(identityRegistry));

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
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAmount.selector);
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
        vm.expectRevert(IAutomatedMarketMaker.AMM__InsufficientLiquidity.selector);
        amm.getAmountIn(1000, 1000, 1000);
    }

    /// @dev Test that swapping tokens for exact output succeeds and updates balances correctly.
    function test_SwapTokensForExactTokens_Success() public {
        /// @dev Setup: Add liquidity first
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 preFeeAmountIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        /// @dev With the 0.3% fee, the user pays the grossed-up input; the fee remains in the reserves.
        uint256 expectedGrossIn = _grossIn(preFeeAmountIn);
        uint256 maxAmountIn = expectedGrossIn;

        uint256 swapperBalanceBef = tokenA.balanceOf(swapper);
        uint256 swapperBalanceOutBef = tokenB.balanceOf(swapper);

        /// @dev Execute Swap
        vm.prank(swapper);
        uint256 actualIn =
            amm.swapTokensForExactTokens(address(tokenA), address(tokenB), amountOutDesired, maxAmountIn, swapper);

        /// @dev Assertions
        assertEq(actualIn, expectedGrossIn, "Amount In mismatch");
        assertGt(actualIn, preFeeAmountIn, "Gross input must exceed the pre-fee input");
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
        uint256 preFeeAmountIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 expectedGrossIn = _grossIn(preFeeAmountIn);

        /// @dev Simulate user only accepting less than the fee-inclusive market price
        uint256 maxAmountIn = expectedGrossIn - 1;

        vm.prank(swapper);
        vm.expectRevert(
            abi.encodeWithSelector(IAutomatedMarketMaker.AMM__SlippageExceeded.selector, expectedGrossIn, maxAmountIn)
        );
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), amountOutDesired, maxAmountIn, swapper);
    }

    /// @dev Test that swap reverts when invalid token pair is provided.
    function test_Revert_Swap_InvalidToken() public {
        vm.prank(swapper);
        vm.expectRevert(IAutomatedMarketMaker.AMM__InvalidToken.selector);
        amm.swapTokensForExactTokens(address(tokenA), address(tokenA), 100, 100, swapper);
    }

    /// @dev Circuit breaker pause is a 1-of-N fail-safe: a single Central Bank can pause everything.
    function test_CircuitBreaker_Pause_ByOneGovernor() public {
        vm.prank(governance);
        amm.pause("liquidity anomaly");

        assertTrue(amm.paused(), "AMM should be paused");
        assertTrue(amm.isPaused(), "isPaused() should mirror paused()");

        /// @dev Attempting to add liquidity should fail while paused.
        vm.prank(liquidityProvider);
        vm.expectRevert();
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);
    }

    /// @dev Only governance-capable participants may pause (swapper is COMMERCIAL_BANK).
    function test_Revert_CircuitBreaker_Pause_Unauthorized() public {
        vm.prank(swapper);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__NotGovernance.selector, swapper));
        amm.pause("unauthorized attempt");
    }

    /// @dev A single governor CANNOT resume: proposing a resume alone leaves the AMM paused (quorum = 2).
    function test_Revert_SingleGovernor_CannotResume() public {
        vm.prank(governance);
        amm.pause("halt");

        vm.prank(governance);
        bytes32 proposalId = amm.proposeResume();

        /// @dev One signature is not enough — the breaker stays engaged.
        assertTrue(amm.paused(), "AMM must remain paused with only one signature");
        assertEq(amm.resumeSignatures(proposalId), 1, "Proposal should carry the proposer's single signature");
        assertEq(amm.resumeQuorum(), 2, "Resume quorum must be 2-of-N");
    }

    /// @dev Resume requires a SECOND distinct governor to sign; quorum (2) auto-resumes the AMM.
    function test_CircuitBreaker_Resume_RequiresTwoSignatures() public {
        vm.prank(governance);
        amm.pause("halt");

        vm.prank(governance);
        bytes32 proposalId = amm.proposeResume();
        assertTrue(amm.paused(), "Still paused after first signature");

        /// @dev Second, distinct Central Bank signs → quorum reached → AMM resumes.
        vm.prank(governance2);
        amm.signResume(proposalId);

        assertFalse(amm.paused(), "AMM should resume once quorum is reached");
        assertEq(amm.resumeSignatures(proposalId), 2, "Proposal should hold two signatures");

        /// @dev Operations work again after resume.
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY);
    }

    /// @dev The same governor cannot sign a resume proposal twice (no self-quorum).
    function test_Revert_Resume_DoubleSignBySameGovernor() public {
        vm.prank(governance);
        amm.pause("halt");

        vm.prank(governance);
        bytes32 proposalId = amm.proposeResume();

        vm.prank(governance);
        vm.expectRevert(
            abi.encodeWithSelector(IAutomatedMarketMaker.AMM__AlreadySigned.selector, proposalId, governance)
        );
        amm.signResume(proposalId);

        assertTrue(amm.paused(), "AMM must remain paused; a governor cannot form quorum alone");
    }

    /// @dev Signing a non-existent proposal reverts.
    function test_Revert_SignResume_ProposalNotFound() public {
        vm.prank(governance);
        amm.pause("halt");

        bytes32 bogus = keccak256("does-not-exist");
        vm.prank(governance2);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__ProposalNotFound.selector, bogus));
        amm.signResume(bogus);
    }

    /// @dev A non-governance account cannot propose a resume.
    function test_Revert_ProposeResume_Unauthorized() public {
        vm.prank(governance);
        amm.pause("halt");

        vm.prank(swapper);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__NotGovernance.selector, swapper));
        amm.proposeResume();
    }

    // ============================================================================
    //                       FEE MODEL (0.3% swap fee)
    // ============================================================================

    /// @dev The swap fee accumulates inside the pool: reserves (and k) grow by the full gross input,
    ///      including the fee portion, so LPs earn the fee.
    function test_Swap_FeeAccumulatesInPool() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        assertEq(amm.feeBps(), FEE_BPS, "Default fee should be 0.3%");

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 preFeeAmountIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 expectedGrossIn = _grossIn(preFeeAmountIn);
        uint256 feePortion = expectedGrossIn - preFeeAmountIn;

        uint256 kBefore = amm.reserveA() * amm.reserveB();

        vm.prank(swapper);
        uint256 actualIn =
            amm.swapTokensForExactTokens(address(tokenA), address(tokenB), amountOutDesired, expectedGrossIn, swapper);

        /// @dev The full gross input (fee included) is pulled into the reserves.
        assertEq(actualIn, expectedGrossIn, "Swapper must pay the fee-inclusive input");
        assertGt(feePortion, 0, "Fee portion must be non-zero at 0.3%");
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY + expectedGrossIn, "Fee must accrue into reserveA");
        assertEq(tokenA.balanceOf(address(amm)), INITIAL_LIQUIDITY + expectedGrossIn, "Pool balance must hold the fee");

        /// @dev k strictly increases because the fee stays in the pool.
        uint256 kAfter = amm.reserveA() * amm.reserveB();
        assertGt(kAfter, kBefore, "Constant-product k must grow as the fee accrues");
    }

    /// @dev Governance can update the fee rate; a non-governance caller cannot.
    function test_SetFeeBps_GovernanceOnly() public {
        vm.prank(governance);
        amm.setFeeBps(50);
        assertEq(amm.feeBps(), 50, "Fee should update to 0.5%");

        vm.prank(swapper);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__NotGovernance.selector, swapper));
        amm.setFeeBps(10);
    }

    /// @dev The fee rate is capped at MAX_FEE_BPS (10%).
    function test_Revert_SetFeeBps_TooHigh() public {
        vm.prank(governance);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__FeeBpsTooHigh.selector, 1001, 1000));
        amm.setFeeBps(1001);
    }

    /// @dev Test that swap reverts when amountOut is zero.
    function test_Revert_Swap_ZeroAmount() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAmount.selector);
        vm.prank(swapper);
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), 0, 100, swapper);
    }

    /// @dev Test that swapping tokenB for tokenA succeeds (reverse direction).
    function test_SwapTokensForExactTokens_ReverseDirection() public {
        /// @dev Setup: Add liquidity first
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 preFeeAmountIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 expectedGrossIn = _grossIn(preFeeAmountIn);
        uint256 maxAmountIn = expectedGrossIn;

        uint256 swapperBalanceBBef = tokenB.balanceOf(swapper);
        uint256 swapperBalanceABef = tokenA.balanceOf(swapper);

        /// @dev Execute Swap: B → A (reverse direction)
        vm.prank(swapper);
        uint256 actualIn =
            amm.swapTokensForExactTokens(address(tokenB), address(tokenA), amountOutDesired, maxAmountIn, swapper);

        /// @dev Assertions
        assertEq(actualIn, expectedGrossIn, "Amount In mismatch");
        assertEq(tokenB.balanceOf(swapper), swapperBalanceBBef - actualIn, "Sender TokenB balance incorrect");
        assertEq(tokenA.balanceOf(swapper), swapperBalanceABef + amountOutDesired, "Sender TokenA balance incorrect");
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY + actualIn, "ReserveB not updated");
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY - amountOutDesired, "ReserveA not updated");
    }

    /// @dev Test that constructor reverts when tokenA is zero address.
    function test_Revert_Constructor_ZeroAddressTokenA() public {
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAddress.selector);
        new AutomatedMarketMaker(address(0), address(tokenB), address(identityRegistry));
    }

    /// @dev Test that constructor reverts when tokenB is zero address.
    function test_Revert_Constructor_ZeroAddressTokenB() public {
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAddress.selector);
        new AutomatedMarketMaker(address(tokenA), address(0), address(identityRegistry));
    }

    /// @dev Test that constructor reverts when identityRegistry is zero address.
    function test_Revert_Constructor_ZeroAddressIdentityRegistry() public {
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAddress.selector);
        new AutomatedMarketMaker(address(tokenA), address(tokenB), address(0));
    }

    /// @dev Test that addLiquidity reverts when caller is not verified in the IdentityRegistry.
    function test_Revert_AddLiquidity_UnverifiedCaller() public {
        address unverified = makeAddr("unverified");
        vm.startPrank(centralBank);
        tokenA.mint(unverified, INITIAL_LIQUIDITY);
        tokenB.mint(unverified, INITIAL_LIQUIDITY);
        vm.stopPrank();

        vm.startPrank(unverified);
        tokenA.approve(address(amm), type(uint256).max);
        tokenB.approve(address(amm), type(uint256).max);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__ParticipantNotVerified.selector, unverified));
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);
        vm.stopPrank();
    }

    /// @dev Test that swap reverts when msg.sender is not verified.
    function test_Revert_Swap_UnverifiedSender() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        address unverified = makeAddr("unverifiedSwapper");
        vm.startPrank(centralBank);
        tokenA.mint(unverified, SWAPPER_BALANCE);
        vm.stopPrank();

        vm.startPrank(unverified);
        tokenA.approve(address(amm), type(uint256).max);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__ParticipantNotVerified.selector, unverified));
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), 100 * 10 ** 18, type(uint256).max, swapper);
        vm.stopPrank();
    }

    /// @dev Test that swap reverts when the `to` address is not verified.
    function test_Revert_Swap_UnverifiedTo() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        address unverifiedTo = makeAddr("unverifiedTo");

        vm.prank(swapper);
        vm.expectRevert(
            abi.encodeWithSelector(IAutomatedMarketMaker.AMM__ParticipantNotVerified.selector, unverifiedTo)
        );
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), 100 * 10 ** 18, type(uint256).max, unverifiedTo);
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
    address private expectedIdentityRegistry;

    /// @dev Environment variable names
    string private constant ENV_DEPLOYER_PRIVATE_KEY = "DEPLOYER_PRIVATE_KEY";
    string private constant ENV_TOKEN_A_ADDRESS = "TOKEN_A_ADDRESS";
    string private constant ENV_TOKEN_B_ADDRESS = "TOKEN_B_ADDRESS";
    string private constant ENV_IDENTITY_REGISTRY_ADDRESS = "IDENTITY_REGISTRY_ADDRESS";

    /// @notice Sets up the test environment by instantiating the deployment script.
    /// @dev Reads deployment parameters from environment variables.
    function setUp() public {
        deployScript = new DeployAMM();
        deployScript.setUp();

        /// @dev Always set explicit defaults first to avoid env contamination from other test suites
        deployerPrivateKey = vm.envOr(ENV_DEPLOYER_PRIVATE_KEY, uint256(0x1));
        expectedDeployer = vm.addr(deployerPrivateKey);
        expectedTokenA = address(0x3456789012345678901234567890123456789012);
        expectedTokenB = address(0x4567890123456789012345678901234567890123);
        expectedIdentityRegistry = address(0x6789012345678901234567890123456789012345);

        vm.setEnv(ENV_DEPLOYER_PRIVATE_KEY, vm.toString(deployerPrivateKey));
        vm.setEnv(ENV_TOKEN_A_ADDRESS, vm.toString(expectedTokenA));
        vm.setEnv(ENV_TOKEN_B_ADDRESS, vm.toString(expectedTokenB));
        vm.setEnv(ENV_IDENTITY_REGISTRY_ADDRESS, vm.toString(expectedIdentityRegistry));
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

        /// @dev Assert: IdentityRegistry wiring
        assertEq(address(amm.IDENTITY_REGISTRY()), expectedIdentityRegistry, "IdentityRegistry address mismatch");

        /// @dev Assert: initial reserves are zero
        assertEq(amm.reserveA(), 0, "Initial reserveA should be zero");
        assertEq(amm.reserveB(), 0, "Initial reserveB should be zero");

        /// @dev Assert: contract is not paused initially
        assertFalse(amm.paused(), "Contract should not be paused initially");

        /// @dev Assert: ensure env-derived deployer address is valid
        assertTrue(expectedDeployer != address(0), "Expected deployer should not be zero address");
    }
}
