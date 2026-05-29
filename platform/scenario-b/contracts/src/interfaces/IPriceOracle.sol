// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

/// @title IPriceOracle
/// @dev Swappable oracle interface for FX rate retrieval (REQ-FX-002).
/// @notice Designed to be compatible with future Chainlink AggregatorV3-style feeds.
interface IPriceOracle {
    /// @notice Returns the exchange rate between two tokens.
    /// @param token0 Address of the base token.
    /// @param token1 Address of the quote token.
    /// @return rate The exchange rate scaled by `decimals`.
    /// @return decimals Number of decimals used in the rate representation.
    function getRate(address token0, address token1) external view returns (uint256 rate, uint8 decimals);
}
