// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {IAutomatedMarketMaker} from "./interfaces/IAutomatedMarketMaker.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";
import {IERC20} from "@openzeppelin-contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin-contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin-contracts/utils/ReentrancyGuard.sol";
import {Pausable} from "@openzeppelin-contracts/utils/Pausable.sol";

/// @title Automated Market Maker (AMM)
/// @dev Constant Product Liquidity Pool for Scenario B (Exact-Output pricing).
///      All identity and role checks are delegated to the IdentityRegistry (single source of truth).
contract AutomatedMarketMaker is IAutomatedMarketMaker, ReentrancyGuard, Pausable {
    using SafeERC20 for IERC20;

    /// @notice The ERC20 tokens in the liquidity pool
    IERC20 public immutable TOKEN_A;
    IERC20 public immutable TOKEN_B;

    /// @notice The Identity Registry used for participant clearance gates.
    IIdentityRegistry public immutable IDENTITY_REGISTRY;

    /// @notice The current reserves of the pool to compute the constant product (x * y = k)
    uint256 public reserveA;
    uint256 public reserveB;

    /// @notice Initializes the AMM with the token pair and IdentityRegistry.
    /// @param _tokenA Address of the first token (e.g., tCeBM_BRL).
    /// @param _tokenB Address of the second token (e.g., tCeBM_EUR).
    /// @param _identityRegistry Address of the IdentityRegistry (single source of truth for roles).
    constructor(address _tokenA, address _tokenB, address _identityRegistry) {
        if (_tokenA == address(0) || _tokenB == address(0) || _identityRegistry == address(0)) {
            revert AMM__ZeroAddress();
        }

        TOKEN_A = IERC20(_tokenA);
        TOKEN_B = IERC20(_tokenB);
        IDENTITY_REGISTRY = IIdentityRegistry(_identityRegistry);
    }

    /// @notice Ensures the given account is a verified participant in the IdentityRegistry.
    /// @param account The address to verify.
    modifier onlyVerified(address account) {
        _onlyVerified(account);
        _;
    }

    /// @dev Internal check extracted from the modifier to reduce bytecode duplication at call sites.
    function _onlyVerified(address account) internal view {
        if (!IDENTITY_REGISTRY.canTransact(account)) {
            revert AMM__ParticipantNotVerified(account);
        }
    }

    /// @notice Restricts access to accounts with a governance-capable role in the IdentityRegistry.
    modifier onlyGovernance() {
        _onlyGovernance();
        _;
    }

    /// @dev Internal governance check — delegates to the IdentityRegistry (single source of truth).
    function _onlyGovernance() internal view {
        if (!IDENTITY_REGISTRY.canGovern(msg.sender)) {
            revert AMM__NotGovernance(msg.sender);
        }
    }

    /// @notice Circuit breaker: Pauses or unpauses all pool operations.
    /// @dev Only governance-capable participants (CENTRAL_BANK, GOVERNANCE) can call this.
    /// @param status True to pause, False to unpause.
    function setPause(bool status) external onlyGovernance {
        if (status) {
            _pause();
        } else {
            _unpause();
        }
    }

    /// @inheritdoc IAutomatedMarketMaker
    function addLiquidity(uint256 amountA, uint256 amountB)
        external
        nonReentrant
        whenNotPaused
        onlyVerified(msg.sender)
    {
        if (amountA == 0 || amountB == 0) {
            revert AMM__ZeroAmount();
        }

        // [INTERACTIONS] - Pull tokens from liquidity provider
        TOKEN_A.safeTransferFrom(msg.sender, address(this), amountA);
        TOKEN_B.safeTransferFrom(msg.sender, address(this), amountB);

        // [EFFECTS] - Update internal reserves
        reserveA += amountA;
        reserveB += amountB;

        emit LogLiquidityAdded(msg.sender, amountA, amountB);
    }

    /// @inheritdoc IAutomatedMarketMaker
    function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut)
        public
        pure
        returns (uint256 amountIn)
    {
        if (amountOut >= reserveOut) {
            revert AMM__InsufficientLiquidity();
        }

        // Constant Product Formula for Exact Output: dx = (x * dy) / (y - dy)
        // We add +1 to counteract integer truncation, guaranteeing the invariant (k) never decreases.
        uint256 numerator = reserveIn * amountOut;
        uint256 denominator = reserveOut - amountOut;

        amountIn = (numerator / denominator) + 1;
    }

    /// @inheritdoc IAutomatedMarketMaker
    function swapTokensForExactTokens(
        address tokenIn,
        address tokenOut,
        uint256 amountOut,
        uint256 maxAmountIn,
        address to
    ) external nonReentrant whenNotPaused onlyVerified(msg.sender) onlyVerified(to) returns (uint256 amountIn) {
        // [CHECKS]
        if (amountOut == 0) revert AMM__ZeroAmount();
        if (tokenIn == tokenOut) revert AMM__InvalidToken();

        bool isAIn = tokenIn == address(TOKEN_A) && tokenOut == address(TOKEN_B);
        bool isBIn = tokenIn == address(TOKEN_B) && tokenOut == address(TOKEN_A);

        if (!isAIn && !isBIn) revert AMM__InvalidToken();

        uint256 reserveIn = isAIn ? reserveA : reserveB;
        uint256 reserveOut = isAIn ? reserveB : reserveA;

        // Mathematical execution
        amountIn = getAmountIn(reserveIn, reserveOut, amountOut);

        // Slippage Protection
        if (amountIn > maxAmountIn) {
            revert AMM__SlippageExceeded(amountIn, maxAmountIn);
        }

        // [EFFECTS] - Optimistically update state
        if (isAIn) {
            reserveA += amountIn;
            reserveB -= amountOut;
        } else {
            reserveB += amountIn;
            reserveA -= amountOut;
        }

        // [INTERACTIONS] - Transfer assets
        IERC20(tokenIn).safeTransferFrom(msg.sender, address(this), amountIn);
        IERC20(tokenOut).safeTransfer(to, amountOut);

        emit LogSwap(msg.sender, tokenIn, tokenOut, amountIn, amountOut);
    }
}
