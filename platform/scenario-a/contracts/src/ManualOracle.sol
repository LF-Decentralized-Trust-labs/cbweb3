// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {IManualOracle} from "./interfaces/IManualOracle.sol";
import {IPriceOracle} from "./interfaces/IPriceOracle.sol";
import {AccessControl} from "@openzeppelin-contracts/access/AccessControl.sol";

/// @title ManualOracle
/// @notice MVP oracle implementation where Central Banks manually set FX rates (REQ-FX-002).
/// @dev Implements IManualOracle (which extends IPriceOracle). Designed to be swapped for a
///      Chainlink integration later. Only accounts with CENTRAL_BANK_ROLE can update rates.
contract ManualOracle is IManualOracle, AccessControl {
    /// @notice Role identifier for the Central Bank rate-setter.
    bytes32 public constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE");

    /// @dev Internal structure for rate storage with an existence flag.
    struct RateEntry {
        uint256 rate;
        bool isSet;
    }

    /// @dev Stores rates indexed by keccak256(token0, token1).
    mapping(bytes32 => RateEntry) private _rates;

    /// @notice Initializes the oracle with admin and central bank roles.
    /// @param admin The address granted DEFAULT_ADMIN_ROLE.
    /// @param centralBank The address granted CENTRAL_BANK_ROLE.
    constructor(address admin, address centralBank) {
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        _grantRole(CENTRAL_BANK_ROLE, centralBank);
    }

    /// @inheritdoc IManualOracle
    /// @dev Only callable by CENTRAL_BANK_ROLE. Rate is stored with 18 decimal precision.
    function setRate(address token0, address token1, uint256 rate) external onlyRole(CENTRAL_BANK_ROLE) {
        if (token0 == address(0) || token1 == address(0)) revert Oracle__InvalidParameters();
        if (rate == 0) revert Oracle__InvalidParameters();

        bytes32 key = _pairKey(token0, token1);
        _rates[key] = RateEntry({rate: rate, isSet: true});

        emit RateUpdated(token0, token1, rate);
    }

    /// @inheritdoc IPriceOracle
    /// @dev Returns the rate with a fixed 18 decimal precision.
    function getRate(address token0, address token1) external view override returns (uint256 rate, uint8 decimals) {
        bytes32 key = _pairKey(token0, token1);
        RateEntry memory entry = _rates[key];
        if (!entry.isSet) revert Oracle__RateNotSet();
        return (entry.rate, 18);
    }

    /// @dev Computes a deterministic key for a token pair (order-sensitive).
    function _pairKey(address token0, address token1) internal pure returns (bytes32 key) {
        assembly {
            mstore(0x00, token0)
            mstore(0x20, token1)
            key := keccak256(0x00, 0x40)
        }
    }
}
