// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

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
    /// @param originator Address of the deal initiator (Bank A).
    /// @param counterpartyB Address of the acceptance authority (Bank D locally).
    /// @param settlementAgent Address of the settlement agent (Bank C).
    /// @param custodian Address of the custodian (Bank D).
    /// @param beneficiary Address of the beneficiary (Bank B).
    /// @param originAmount Amount in the origin currency.
    /// @param counterAmount Amount in the counter currency.
    /// @param originCurrency ISO 4217 currency code of the origin currency (e.g. bytes32("BRL")).
    /// @param counterCurrency ISO 4217 currency code of the counter currency (e.g. bytes32("EUR")).
    /// @param rate Agreed exchange rate scaled by 1e18.
    /// @param expiryDate Unix timestamp after which the proposal expires.
    /// @param state Current lifecycle state of the agreement.
    struct FxAgreement {
        bytes32 tradeId;
        address originator;
        address counterpartyB;
        address settlementAgent;
        address custodian;
        address beneficiary;
        uint256 originAmount;
        uint256 counterAmount;
        bytes32 originCurrency;
        bytes32 counterCurrency;
        uint256 rate;
        uint256 expiryDate;
        AgreementState state;
    }
}
