// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

/// @title AutomatedMarketMakerLibrary
/// @dev Core data structures and custom errors from AMM logic.
library AutomatedMarketMakerLibrary {
    /// @dev Custom errors for exact-output pricing and pool interactions
    error AMM__ZeroAddress();
    error AMM__ZeroAmount();
    error AMM__InvalidToken();
    error AMM__InsufficientLiquidity();
    error AMM__InsufficientOutputAmount();
    error AMM__SlippageExceeded(uint256 requiredAmountIn, uint256 maxAmountIn);
}
