// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {IPriceOracle} from "./IPriceOracle.sol";

/// @title IManualOracle
/// @notice Interface for the manually-governed FX rate oracle (REQ-FX-002).
/// @dev Extends IPriceOracle with administrative functions for Central Bank rate management.
///      Designed to be swapped for a Chainlink AggregatorV3-style feed in production.
interface IManualOracle is IPriceOracle {
    /// @notice Emitted when a Central Bank updates the exchange rate for a token pair.
    event RateUpdated(address indexed token0, address indexed token1, uint256 rate);

    /// @dev The requested rate has not been set for this token pair.
    error Oracle__RateNotSet();

    /// @dev Invalid input parameters (zero address, zero rate, etc.).
    error Oracle__InvalidParameters();

    /// @notice Sets the exchange rate for a token pair.
    /// @param token0 Address of the base token.
    /// @param token1 Address of the quote token.
    /// @param rate The exchange rate scaled by 1e18.
    function setRate(address token0, address token1, uint256 rate) external;
}
