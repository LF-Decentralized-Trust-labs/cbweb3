// SPDX-License-Identifier: UNLICENSED
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
/// @dev Covers liquidity, swaps, slippage, and the asymmetric circuit breaker (FR-043/FR-044).
contract AutomatedMarketMakerTest is Test {
    AutomatedMarketMaker public amm;
    IdentityRegistry public identityRegistry;
    TokenizedCentralBankMoney public tokenA;
    TokenizedCentralBankMoney public tokenB;

    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public governanceA = makeAddr("governanceA");
    address public governanceB = makeAddr("governanceB");
    address public governanceC = makeAddr("governanceC");
    address public liquidityProvider = makeAddr("liquidityProvider");
    address public swapper = makeAddr("swapper");

    uint256 public constant INITIAL_LIQUIDITY = 100_000 * 10 ** 18;
    uint256 public constant SWAPPER_BALANCE = 10_000 * 10 ** 18;

    function setUp() public {
        tokenA = new TokenizedCentralBankMoney("Token BRL", "tCeBM_BRL", admin, centralBank);
        tokenB = new TokenizedCentralBankMoney("Token EUR", "tCeBM_EUR", admin, centralBank);

        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            liquidityProvider, "LP Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            swapper, "Commercial Bank A", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            governanceA, "Central Bank A", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            governanceB, "Central Bank B", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            governanceC, "Central Bank C", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        vm.stopPrank();

        amm = new AutomatedMarketMaker(address(tokenA), address(tokenB), address(identityRegistry));

        vm.startPrank(centralBank);
        tokenA.mint(liquidityProvider, INITIAL_LIQUIDITY);
        tokenB.mint(liquidityProvider, INITIAL_LIQUIDITY);
        tokenA.mint(swapper, SWAPPER_BALANCE);
        tokenB.mint(swapper, SWAPPER_BALANCE);
        vm.stopPrank();

        vm.startPrank(liquidityProvider);
        tokenA.approve(address(amm), type(uint256).max);
        tokenB.approve(address(amm), type(uint256).max);
        vm.stopPrank();

        vm.startPrank(swapper);
        tokenA.approve(address(amm), type(uint256).max);
        tokenB.approve(address(amm), type(uint256).max);
        vm.stopPrank();
    }

    // ---------- Liquidity & swap ----------

    function test_AddLiquidity_Success() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        assertEq(amm.reserveA(), INITIAL_LIQUIDITY);
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY);
        assertEq(tokenA.balanceOf(address(amm)), INITIAL_LIQUIDITY);
        assertEq(tokenB.balanceOf(address(amm)), INITIAL_LIQUIDITY);
    }

    function test_Revert_AddLiquidity_ZeroAmount() public {
        vm.prank(liquidityProvider);
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAmount.selector);
        amm.addLiquidity(0, INITIAL_LIQUIDITY);
    }

    function test_RemoveLiquidity_Success() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        vm.prank(liquidityProvider);
        amm.removeLiquidity(INITIAL_LIQUIDITY / 2, INITIAL_LIQUIDITY / 2);

        assertEq(amm.reserveA(), INITIAL_LIQUIDITY / 2);
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY / 2);
    }

    function test_Revert_RemoveLiquidity_InsufficientReserve() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        vm.prank(liquidityProvider);
        vm.expectRevert(IAutomatedMarketMaker.AMM__InsufficientLiquidity.selector);
        amm.removeLiquidity(INITIAL_LIQUIDITY + 1, 0);
    }

    function test_GetAmountIn_Math() public view {
        uint256 reserveIn = 1000 * 10 ** 18;
        uint256 reserveOut = 1000 * 10 ** 18;
        uint256 amountOut = 100 * 10 ** 18;

        uint256 expectedIn = ((reserveIn * amountOut) / (reserveOut - amountOut)) + 1;
        uint256 actualIn = amm.getAmountIn(reserveIn, reserveOut, amountOut);

        assertEq(actualIn, expectedIn);
    }

    function test_Revert_GetAmountIn_InsufficientLiquidity() public {
        vm.expectRevert(IAutomatedMarketMaker.AMM__InsufficientLiquidity.selector);
        amm.getAmountIn(1000, 1000, 1000);
    }

    function test_SwapTokensForExactTokens_Success() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 calculatedAmountIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 maxAmountIn = calculatedAmountIn;

        uint256 swapperBalanceBef = tokenA.balanceOf(swapper);
        uint256 swapperBalanceOutBef = tokenB.balanceOf(swapper);

        vm.prank(swapper);
        uint256 actualIn =
            amm.swapTokensForExactTokens(address(tokenA), address(tokenB), amountOutDesired, maxAmountIn, swapper);

        assertEq(actualIn, calculatedAmountIn, "Amount In mismatch");
        assertEq(tokenA.balanceOf(swapper), swapperBalanceBef - actualIn, "Sender TokenA balance incorrect");
        assertEq(tokenB.balanceOf(swapper), swapperBalanceOutBef + amountOutDesired, "Sender TokenB balance incorrect");
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY + actualIn, "ReserveA not updated");
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY - amountOutDesired, "ReserveB not updated");
    }

    function test_Revert_Swap_SlippageExceeded() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 calculatedAmountIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 maxAmountIn = calculatedAmountIn - 1;

        vm.prank(swapper);
        vm.expectRevert(
            abi.encodeWithSelector(
                IAutomatedMarketMaker.AMM__SlippageExceeded.selector, calculatedAmountIn, maxAmountIn
            )
        );
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), amountOutDesired, maxAmountIn, swapper);
    }

    function test_Revert_Swap_InvalidToken() public {
        vm.prank(swapper);
        vm.expectRevert(IAutomatedMarketMaker.AMM__InvalidToken.selector);
        amm.swapTokensForExactTokens(address(tokenA), address(tokenA), 100, 100, swapper);
    }

    // ---------- Asymmetric Circuit Breaker (FR-043 / FR-044 / SC-017 / SC-026) ----------

    function test_CircuitBreaker_Pause_OneOfN() public {
        vm.prank(governanceA);
        amm.pause("Market stress test");
        assertTrue(amm.isPaused(), "AMM should be paused after 1-of-N pause");

        vm.prank(liquidityProvider);
        vm.expectRevert(IAutomatedMarketMaker.AMM__AlreadyPaused.selector);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);
    }

    function test_Revert_CircuitBreaker_Pause_Unauthorized() public {
        vm.prank(swapper);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__NotGovernance.selector, swapper));
        amm.pause("unauthorized attempt");
    }

    function test_Revert_CircuitBreaker_DoublePause() public {
        vm.prank(governanceA);
        amm.pause("first");
        vm.prank(governanceB);
        vm.expectRevert(IAutomatedMarketMaker.AMM__AlreadyPaused.selector);
        amm.pause("second");
    }

    function test_CircuitBreaker_Resume_QuorumReached() public {
        vm.prank(governanceA);
        amm.pause("incident");

        vm.prank(governanceA);
        bytes32 proposalId = amm.proposeResume();
        assertEq(amm.resumeSignatures(proposalId), 1, "proposer counts as first signature");
        assertTrue(amm.isPaused(), "still paused with only 1 signature");

        vm.prank(governanceB);
        amm.signResume(proposalId);
        assertFalse(amm.isPaused(), "AMM must be resumed after reaching quorum 2-of-N");
        assertEq(amm.resumeSignatures(proposalId), 2);

        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY);
    }

    function test_Revert_CircuitBreaker_Resume_DoubleSign() public {
        vm.prank(governanceA);
        amm.pause("incident");

        vm.prank(governanceA);
        bytes32 proposalId = amm.proposeResume();

        vm.prank(governanceA);
        vm.expectRevert(
            abi.encodeWithSelector(IAutomatedMarketMaker.AMM__AlreadySigned.selector, proposalId, governanceA)
        );
        amm.signResume(proposalId);
    }

    function test_Revert_CircuitBreaker_Resume_ProposalNotFound() public {
        vm.prank(governanceA);
        amm.pause("incident");

        vm.prank(governanceB);
        vm.expectRevert(
            abi.encodeWithSelector(IAutomatedMarketMaker.AMM__ProposalNotFound.selector, bytes32(uint256(0xdead)))
        );
        amm.signResume(bytes32(uint256(0xdead)));
    }

    function test_Revert_CircuitBreaker_Resume_CannotProposeWhenNotPaused() public {
        vm.prank(governanceA);
        vm.expectRevert(IAutomatedMarketMaker.AMM__NotPaused.selector);
        amm.proposeResume();
    }

    function test_Revert_Swap_ZeroAmount() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAmount.selector);
        vm.prank(swapper);
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), 0, 100, swapper);
    }

    function test_SwapTokensForExactTokens_ReverseDirection() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 calculatedAmountIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 maxAmountIn = calculatedAmountIn;

        uint256 swapperBalanceBBef = tokenB.balanceOf(swapper);
        uint256 swapperBalanceABef = tokenA.balanceOf(swapper);

        vm.prank(swapper);
        uint256 actualIn =
            amm.swapTokensForExactTokens(address(tokenB), address(tokenA), amountOutDesired, maxAmountIn, swapper);

        assertEq(actualIn, calculatedAmountIn, "Amount In mismatch");
        assertEq(tokenB.balanceOf(swapper), swapperBalanceBBef - actualIn, "Sender TokenB balance incorrect");
        assertEq(tokenA.balanceOf(swapper), swapperBalanceABef + amountOutDesired, "Sender TokenA balance incorrect");
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY + actualIn, "ReserveB not updated");
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY - amountOutDesired, "ReserveA not updated");
    }

    function test_Revert_Constructor_ZeroAddressTokenA() public {
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAddress.selector);
        new AutomatedMarketMaker(address(0), address(tokenB), address(identityRegistry));
    }

    function test_Revert_Constructor_ZeroAddressTokenB() public {
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAddress.selector);
        new AutomatedMarketMaker(address(tokenA), address(0), address(identityRegistry));
    }

    function test_Revert_Constructor_ZeroAddressIdentityRegistry() public {
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAddress.selector);
        new AutomatedMarketMaker(address(tokenA), address(tokenB), address(0));
    }

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
contract DeployAMMTest is Test {
    DeployAMM public deployScript;

    uint256 private deployerPrivateKey;
    address private expectedDeployer;
    address private expectedTokenA;
    address private expectedTokenB;
    address private expectedIdentityRegistry;

    string private constant ENV_DEPLOYER_PRIVATE_KEY = "DEPLOYER_PRIVATE_KEY";
    string private constant ENV_TOKEN_A_ADDRESS = "TOKEN_A_ADDRESS";
    string private constant ENV_TOKEN_B_ADDRESS = "TOKEN_B_ADDRESS";
    string private constant ENV_IDENTITY_REGISTRY_ADDRESS = "IDENTITY_REGISTRY_ADDRESS";

    function setUp() public {
        deployScript = new DeployAMM();
        deployScript.setUp();

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

    function test_ScriptRun_Success() public {
        deployScript.run();
        AutomatedMarketMaker amm = deployScript.amm();

        assertTrue(address(amm) != address(0), "Contract was not deployed");
        assertGt(address(amm).code.length, 0, "Deployed contract has no runtime bytecode");

        assertEq(address(amm.TOKEN_A()), expectedTokenA, "TokenA address mismatch");
        assertEq(address(amm.TOKEN_B()), expectedTokenB, "TokenB address mismatch");

        assertEq(address(amm.IDENTITY_REGISTRY()), expectedIdentityRegistry, "IdentityRegistry address mismatch");

        assertEq(amm.reserveA(), 0, "Initial reserveA should be zero");
        assertEq(amm.reserveB(), 0, "Initial reserveB should be zero");

        assertFalse(amm.isPaused(), "Contract should not be paused initially");

        assertTrue(expectedDeployer != address(0), "Expected deployer should not be zero address");
    }
}
