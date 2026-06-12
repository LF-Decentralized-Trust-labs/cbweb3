// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {IAutomatedMarketMaker} from "../src/interfaces/IAutomatedMarketMaker.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {DeployAMM} from "../script/AutomatedMarketMaker.s.sol";
import {IERC20Errors} from "@openzeppelin-contracts/interfaces/draft-IERC6093.sol";

/// @title AutomatedMarketMakerTest
/// @notice Unit tests for the LP-share AMM: proportional mint/burn, home-currency zap-out
///         withdrawal, drain protection, empty-pool guard, emergency exit, fees, circuit breaker.
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
    uint256 public constant MINIMUM_LIQUIDITY = 1000;

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

        // Large balances so the §2.2 worked-example reserves (5M / 1M) are fundable.
        vm.startPrank(centralBank);
        tokenA.mint(liquidityProvider, 10_000_000 * 10 ** 18);
        tokenB.mint(liquidityProvider, 10_000_000 * 10 ** 18);
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

    // ---------- Liquidity: proportional mint + MINIMUM_LIQUIDITY lock (TASK-12/13) ----------

    function test_AddLiquidity_FirstDeposit_MintsSharesAndLocksMinimum() public {
        vm.prank(liquidityProvider);
        uint256 shares = amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        // sqrt(k) total, minus the permanently-locked MINIMUM_LIQUIDITY.
        uint256 expectedTotal = INITIAL_LIQUIDITY; // sqrt(L*L) = L
        assertEq(shares, expectedTotal - MINIMUM_LIQUIDITY, "first-deposit shares");
        assertEq(amm.balanceOf(liquidityProvider), shares, "LP holds its shares");
        assertEq(amm.balanceOf(amm.BURN_ADDRESS()), MINIMUM_LIQUIDITY, "MINIMUM_LIQUIDITY locked");
        assertEq(amm.totalSupply(), expectedTotal, "total supply = sqrt(k)");
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY);
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY);
    }

    function test_AddLiquidity_SecondDeposit_Proportional() public {
        vm.startPrank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);
        uint256 supplyBefore = amm.totalSupply();
        // Add 50% more at the same ratio → ~50% more shares.
        uint256 shares = amm.addLiquidity(INITIAL_LIQUIDITY / 2, INITIAL_LIQUIDITY / 2);
        vm.stopPrank();

        assertEq(shares, supplyBefore / 2, "proportional second-deposit shares");
    }

    function test_Revert_AddLiquidity_FirstDepositBelowMinimum() public {
        // sqrt(amountA*amountB) must exceed MINIMUM_LIQUIDITY.
        vm.prank(liquidityProvider);
        vm.expectRevert(IAutomatedMarketMaker.AMM__InsufficientLiquidity.selector);
        amm.addLiquidity(10, 10); // sqrt(100)=10 <= 1000
    }

    function test_Revert_AddLiquidity_ZeroAmount() public {
        vm.prank(liquidityProvider);
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAmount.selector);
        amm.addLiquidity(0, INITIAL_LIQUIDITY);
    }

    // ---------- Withdrawal: home-currency zap-out (decision D1, §2.2 numbers) ----------

    /// @dev §2.2 large exit: 10% of a 5M/1M pool, single-currency BRL out ≈ 948,785 (≈5.1% haircut).
    function test_RemoveLiquidity_HomeCurrency_LargeExit_MatchesWorkedExample() public {
        _seedWorkedExamplePool(); // 5,000,000 BRL (A) / 1,000,000 EUR (B)

        uint256 supply = amm.totalSupply();
        uint256 tenPct = supply / 10;
        uint256 balBefore = tokenA.balanceOf(liquidityProvider);

        vm.prank(liquidityProvider);
        uint256 out = amm.removeLiquidity(tenPct, address(tokenA), 0);

        // ≈ 948,785e18 (500k pro-rata + ≈448,785 swap proceeds).
        assertApproxEqRel(out, 948_785 * 10 ** 18, 0.005e18, "large-exit BRL out");
        assertEq(tokenA.balanceOf(liquidityProvider), balBefore + out, "received BRL");

        // Notional value at 5 BRL/EUR was 1,000,000 → haircut ~5.1%.
        uint256 notional = 1_000_000 * 10 ** 18;
        uint256 haircutBps = ((notional - out) * 10000) / notional;
        assertGt(haircutBps, 480, "haircut > 4.8%");
        assertLt(haircutBps, 540, "haircut < 5.4%");
    }

    /// @dev §2.2 small exit: 1% of the same pool ≈ 99,353 BRL out (≈0.65% haircut). Size drives slippage.
    function test_RemoveLiquidity_HomeCurrency_SmallExit_LowerSlippage() public {
        _seedWorkedExamplePool();

        uint256 onePct = amm.totalSupply() / 100;
        vm.prank(liquidityProvider);
        uint256 out = amm.removeLiquidity(onePct, address(tokenA), 0);

        assertApproxEqRel(out, 99_353 * 10 ** 18, 0.005e18, "small-exit BRL out");
        uint256 notional = 100_000 * 10 ** 18;
        uint256 haircutBps = ((notional - out) * 10000) / notional;
        assertLt(haircutBps, 100, "small exit haircut < 1%");
    }

    function test_RemoveLiquidity_HomeCurrency_TokenBSide() public {
        _seedWorkedExamplePool();
        uint256 onePct = amm.totalSupply() / 100;
        uint256 balBefore = tokenB.balanceOf(liquidityProvider);

        vm.prank(liquidityProvider);
        uint256 out = amm.removeLiquidity(onePct, address(tokenB), 0);

        assertGt(out, 0, "received EUR");
        assertEq(tokenB.balanceOf(liquidityProvider), balBefore + out, "EUR credited");
    }

    function test_Revert_RemoveLiquidity_SlippageGuard() public {
        _seedWorkedExamplePool();
        uint256 tenPct = amm.totalSupply() / 10;
        // Demand more than achievable → revert.
        vm.prank(liquidityProvider);
        vm.expectRevert(IAutomatedMarketMaker.AMM__InsufficientOutputAmount.selector);
        amm.removeLiquidity(tenPct, address(tokenA), 1_000_000 * 10 ** 18);
    }

    function test_Revert_RemoveLiquidity_InvalidTokenOut() public {
        _seedWorkedExamplePool();
        vm.prank(liquidityProvider);
        vm.expectRevert(IAutomatedMarketMaker.AMM__InvalidToken.selector);
        amm.removeLiquidity(1000, address(0x1234), 0);
    }

    // ---------- Drain protection (TASK-12 core) ----------

    /// @dev A verified participant who never provided liquidity holds zero shares and therefore
    ///      cannot withdraw anything — the prior pool-drain vector is closed by ERC20 burn.
    function test_Revert_RemoveLiquidity_NonProviderCannotDrain() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        // `swapper` is verified but owns no LP shares.
        assertEq(amm.balanceOf(swapper), 0);
        vm.prank(swapper);
        vm.expectRevert(
            abi.encodeWithSelector(IERC20Errors.ERC20InsufficientBalance.selector, swapper, 0, INITIAL_LIQUIDITY / 2)
        );
        amm.removeLiquidity(INITIAL_LIQUIDITY / 2, address(tokenA), 0);
    }

    function test_Revert_RemoveLiquidity_CannotBurnMoreThanOwned() public {
        vm.prank(liquidityProvider);
        uint256 shares = amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        vm.prank(liquidityProvider);
        vm.expectRevert(
            abi.encodeWithSelector(
                IERC20Errors.ERC20InsufficientBalance.selector, liquidityProvider, shares, shares + 1
            )
        );
        amm.removeLiquidity(shares + 1, address(tokenA), 0);
    }

    // ---------- Emergency exit (paused, both sides, no swap) ----------

    function test_RemoveLiquidityEmergency_WhenPaused_ReturnsBothSides() public {
        _seedWorkedExamplePool();
        uint256 tenPct = amm.totalSupply() / 10;

        vm.prank(governanceA);
        amm.pause("incident");

        uint256 aBefore = tokenA.balanceOf(liquidityProvider);
        uint256 bBefore = tokenB.balanceOf(liquidityProvider);

        vm.prank(liquidityProvider);
        (uint256 amountA, uint256 amountB) = amm.removeLiquidityEmergency(tenPct);

        // ~10% of 5M / 1M, no swap, no slippage.
        assertApproxEqRel(amountA, 500_000 * 10 ** 18, 0.001e18, "pro-rata A");
        assertApproxEqRel(amountB, 100_000 * 10 ** 18, 0.001e18, "pro-rata B");
        assertEq(tokenA.balanceOf(liquidityProvider), aBefore + amountA);
        assertEq(tokenB.balanceOf(liquidityProvider), bBefore + amountB);
    }

    function test_Revert_RemoveLiquidityEmergency_WhenNotPaused() public {
        _seedWorkedExamplePool();
        vm.prank(liquidityProvider);
        vm.expectRevert(IAutomatedMarketMaker.AMM__NotPaused.selector);
        amm.removeLiquidityEmergency(1000);
    }

    function test_Revert_RemoveLiquidity_WhenPaused() public {
        _seedWorkedExamplePool();
        vm.prank(governanceA);
        amm.pause("incident");
        vm.prank(liquidityProvider);
        vm.expectRevert(IAutomatedMarketMaker.AMM__AlreadyPaused.selector);
        amm.removeLiquidity(1000, address(tokenA), 0);
    }

    // ---------- Swap math (fee-in-reserve) ----------

    function test_GetAmountIn_Math() public view {
        uint256 reserveIn = 1000 * 10 ** 18;
        uint256 reserveOut = 1000 * 10 ** 18;
        uint256 amountOut = 100 * 10 ** 18;
        uint256 expectedIn = ((reserveIn * amountOut) / (reserveOut - amountOut)) + 1;
        assertEq(amm.getAmountIn(reserveIn, reserveOut, amountOut), expectedIn);
    }

    function test_GetAmountOut_Math() public view {
        // §2.2 zap: sell 100,000 EUR into RB=900,000 / RA=4,500,000 @0.3% ≈ 448,785.
        uint256 out = amm.getAmountOut(100_000 * 10 ** 18, 900_000 * 10 ** 18, 4_500_000 * 10 ** 18, 30);
        assertApproxEqRel(out, 448_785 * 10 ** 18, 0.001e18, "getAmountOut worked example");
    }

    function test_GetAmountOut_EmptyReserves_ReturnsZero() public view {
        assertEq(amm.getAmountOut(100, 0, 1000, 30), 0);
        assertEq(amm.getAmountOut(100, 1000, 0, 30), 0);
        assertEq(amm.getAmountOut(0, 1000, 1000, 30), 0);
    }

    function test_SwapTokensForExactTokens_Success() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 baseIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 grossIn = (baseIn * 10000) / (10000 - amm.feeBps()) + 1; // fee-aware max
        uint256 swapperABef = tokenA.balanceOf(swapper);
        uint256 swapperBBef = tokenB.balanceOf(swapper);

        vm.prank(swapper);
        uint256 actualIn =
            amm.swapTokensForExactTokens(address(tokenA), address(tokenB), amountOutDesired, grossIn, swapper);

        assertEq(actualIn, grossIn, "gross amount in (incl. fee)");
        assertEq(tokenA.balanceOf(swapper), swapperABef - actualIn, "TokenA spent");
        assertEq(tokenB.balanceOf(swapper), swapperBBef + amountOutDesired, "TokenB received");
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY + actualIn, "reserveA grows incl. fee");
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY - amountOutDesired, "reserveB drops by output");
    }

    function test_SwapTokensForExactTokens_ReverseDirection() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 baseIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 grossIn = (baseIn * 10000) / (10000 - amm.feeBps()) + 1;

        vm.prank(swapper);
        uint256 actualIn =
            amm.swapTokensForExactTokens(address(tokenB), address(tokenA), amountOutDesired, grossIn, swapper);

        assertEq(actualIn, grossIn, "gross amount in");
        assertEq(amm.reserveB(), INITIAL_LIQUIDITY + actualIn, "reserveB grows");
        assertEq(amm.reserveA(), INITIAL_LIQUIDITY - amountOutDesired, "reserveA drops");
    }

    function test_Revert_Swap_SlippageExceeded() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 baseIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 grossIn = (baseIn * 10000) / (10000 - amm.feeBps()) + 1;
        uint256 maxAmountIn = grossIn - 1;

        vm.prank(swapper);
        vm.expectRevert(
            abi.encodeWithSelector(IAutomatedMarketMaker.AMM__SlippageExceeded.selector, grossIn, maxAmountIn)
        );
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), amountOutDesired, maxAmountIn, swapper);
    }

    function test_Revert_Swap_EmptyPool() public {
        // No liquidity added → empty-pool guard (TASK-13).
        vm.prank(swapper);
        vm.expectRevert(IAutomatedMarketMaker.AMM__InsufficientLiquidity.selector);
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), 100, type(uint256).max, swapper);
    }

    function test_Revert_Swap_InvalidToken() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);
        vm.prank(swapper);
        vm.expectRevert(IAutomatedMarketMaker.AMM__InvalidToken.selector);
        amm.swapTokensForExactTokens(address(tokenA), address(tokenA), 100, 100, swapper);
    }

    function test_Revert_Swap_ZeroAmount() public {
        vm.prank(liquidityProvider);
        amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);
        vm.expectRevert(IAutomatedMarketMaker.AMM__ZeroAmount.selector);
        vm.prank(swapper);
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), 0, 100, swapper);
    }

    // ---------- Fee accrual to share value ----------

    function test_Fees_AccrueToShareValue() public {
        // LP1 seeds; a swap leaves a 0.3% fee in reserves; LP1's redeemable value rises.
        vm.prank(liquidityProvider);
        uint256 shares = amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        uint256 kBefore = amm.reserveA() * amm.reserveB();

        uint256 amountOutDesired = 1_000 * 10 ** 18;
        uint256 baseIn = amm.getAmountIn(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY, amountOutDesired);
        uint256 grossIn = (baseIn * 10000) / (10000 - amm.feeBps()) + 1;
        vm.prank(swapper);
        amm.swapTokensForExactTokens(address(tokenA), address(tokenB), amountOutDesired, grossIn, swapper);

        // Constant product grew because the fee stayed in the pool.
        assertGt(amm.reserveA() * amm.reserveB(), kBefore, "k grew from retained fee");
        assertEq(amm.balanceOf(liquidityProvider), shares, "share count unchanged");
    }

    // ---------- Restricted LP-share transfers (D2) ----------

    function test_Transfer_BetweenVerified_Succeeds() public {
        vm.prank(liquidityProvider);
        uint256 shares = amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        vm.prank(liquidityProvider);
        amm.transfer(swapper, shares / 4); // swapper is verified
        assertEq(amm.balanceOf(swapper), shares / 4);
    }

    function test_Revert_Transfer_ToUnverified() public {
        vm.prank(liquidityProvider);
        uint256 shares = amm.addLiquidity(INITIAL_LIQUIDITY, INITIAL_LIQUIDITY);

        address outsider = makeAddr("outsider");
        vm.prank(liquidityProvider);
        vm.expectRevert(abi.encodeWithSelector(IAutomatedMarketMaker.AMM__ParticipantNotVerified.selector, outsider));
        amm.transfer(outsider, shares / 4);
    }

    // ---------- Asymmetric Circuit Breaker (FR-043 / FR-044) ----------

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
        assertFalse(amm.isPaused(), "resumed after quorum 2-of-N");
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

    // ---------- Constructor ----------

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

    // ---------- helpers ----------

    /// @dev Seeds the §2.2 worked-example pool: 5,000,000 (A/BRL) : 1,000,000 (B/EUR), price 5:1.
    function _seedWorkedExamplePool() internal {
        vm.prank(liquidityProvider);
        amm.addLiquidity(5_000_000 * 10 ** 18, 1_000_000 * 10 ** 18);
    }
}

/// @title DeployAMMTest
/// @notice Unit tests for the DeployAMM deployment script.
contract DeployAMMTest is Test {
    DeployAMM public deployScript;

    uint256 private deployerPrivateKey;
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
        assertEq(address(amm.IDENTITY_REGISTRY()), expectedIdentityRegistry, "IdentityRegistry mismatch");
        assertEq(amm.reserveA(), 0, "Initial reserveA should be zero");
        assertEq(amm.reserveB(), 0, "Initial reserveB should be zero");
        assertFalse(amm.isPaused(), "Contract should not be paused initially");
    }
}
