// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

/// @title FXAgreementLibrary
/// @notice Shared data structures for the on-chain bilateral FX agreement lifecycle.
/// @dev Centralizes enums and structs to ensure byte-code consistency across the FX module.
library FXAgreementLibrary {
    /// @notice Represents the finite state machine of an FX agreement.
    enum AgreementState {
        INVALID,
        PROPOSED,
        ACCEPTED,
        SETTLED,
        REJECTED,
        CANCELLED
    }

    /// @notice Immutable record of a bilateral FX deal.
    /// @param tradeId Unique identifier for the deal.
    /// @param counterpartyA Address of the deal initiator.
    /// @param counterpartyB Address of the accepting party.
    /// @param baseToken Token address of the base currency.
    /// @param quoteToken Token address of the quote currency.
    /// @param notional Notional amount of the base currency.
    /// @param rate Agreed exchange rate scaled by 1e18.
    /// @param expiryDate Unix timestamp after which the proposal expires.
    /// @param state Current lifecycle state of the agreement.
    struct FxAgreement {
        bytes32 tradeId;
        address counterpartyA;
        address counterpartyB;
        address baseToken;
        address quoteToken;
        uint256 notional;
        uint256 rate;
        uint256 expiryDate;
        AgreementState state;
    }
}
