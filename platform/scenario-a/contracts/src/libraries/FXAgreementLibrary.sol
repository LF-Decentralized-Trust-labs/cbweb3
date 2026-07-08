// SPDX-License-Identifier: Apache-2.0
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

    /// @notice Cross-spoke routing metadata for a bilateral FX deal.
    /// @dev Carries the network topology (spoke IDs) and the off-chain-addressable Paladin
    ///      identities for each party. These are held on-chain *inside the private Pente group*
    ///      (never on the public ledger) so the coordinating central bank — a member of the
    ///      group — reads the full deal without an out-of-band channel, and the relay can route
    ///      the destination leg by identity string (robust to the shared-key address collision
    ///      in local dev, where distinct parties resolve to one address).
    /// @param sourceSpokeId Spoke where the deal originates (e.g. "spoke-brl").
    /// @param destSpokeId Spoke where the counter leg settles (e.g. "spoke-cop").
    /// @param originatorId Paladin identity of the originator on the source spoke.
    /// @param counterpartyId Paladin identity of the accepting counterparty on the dest spoke.
    /// @param settlementAgentId Paladin identity of the settlement agent.
    /// @param custodianId Paladin identity of the custodian on the dest spoke.
    /// @param beneficiaryId Paladin identity of the beneficiary.
    /// @param sourceReceiverId Paladin identity that receives the source-spoke leg.
    /// @param destReceiverId Paladin identity that receives the dest-spoke leg.
    /// @param tradeRef Off-chain trade reference (the UUID string). The mapping key `tradeId` is
    ///        `sha256(tradeRef)` — a one-way hash — so the reference is stored here to let the
    ///        coordinating central bank recover the original id from chain state alone (the relay
    ///        reuses it end-to-end for idempotent cross-spoke redelivery).
    struct Routing {
        string sourceSpokeId;
        string destSpokeId;
        string originatorId;
        string counterpartyId;
        string settlementAgentId;
        string custodianId;
        string beneficiaryId;
        string sourceReceiverId;
        string destReceiverId;
        string tradeRef;
    }
}
