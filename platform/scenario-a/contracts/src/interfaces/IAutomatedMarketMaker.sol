// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

/// @title IAutomatedMarketMaker
/// @dev Interface for the Constant Product AMM Liquidity Pool.
interface IAutomatedMarketMaker {
    /// @notice Emitted when liquidity is added to the pool.
    event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB);

    /// @notice Emitted when a swap is successfully executed.
    event LogSwap(
        address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut
    );

    /// @dev Custom errors for exact-output pricing and pool interactions.
    error AMM__ZeroAddress();
    error AMM__ZeroAmount();
    error AMM__InvalidToken();
    error AMM__InsufficientLiquidity();
    error AMM__InsufficientOutputAmount();
    error AMM__SlippageExceeded(uint256 requiredAmountIn, uint256 maxAmountIn);
    error AMM__ParticipantNotVerified(address account);
    error AMM__NotGovernance(address account);

    /// @notice Adds initial or subsequent liquidity to the pool.
    /// @param amountA The amount of token A to add.
    /// @param amountB The amount of token B to add.
    function addLiquidity(uint256 amountA, uint256 amountB) external;

    /// @notice Swaps tokens aiming for an EXACT output amount (Exact-Output pricing).
    /// @dev Implements slippage protection via maxAmountIn.
    /// @param tokenIn The address of the token the user is paying.
    /// @param tokenOut The address of the token the user wants to receive.
    /// @param amountOut The exact amount of tokenOut the user wants.
    /// @param maxAmountIn The maximum amount of tokenIn the user is willing to pay (Slippage protection).
    /// @param to The address that will receive the output tokens.
    /// @return amountIn The calculated amount of tokenIn actually deducted.
    function swapTokensForExactTokens(
        address tokenIn,
        address tokenOut,
        uint256 amountOut,
        uint256 maxAmountIn,
        address to
    ) external returns (uint256 amountIn);

    /// @notice Calculates the required input amount for a desired output amount.
    /// @param reserveIn The current reserve of the input token.
    /// @param reserveOut The current reserve of the output token.
    /// @param amountOut The exact amount of output tokens desired.
    /// @return amountIn The mathematically required amount of input tokens.
    function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut)
        external
        pure
        returns (uint256 amountIn);
}
