// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

/// @title IdentityLib
/// @notice Shared data structures and constants for the CBDC network identity management.
/// @dev This library centralizes enums and structs to ensure byte-code consistency across the ecosystem.
library IdentityRegistryLibrary {
    /// @notice Possible compliance states for an institutional participant.
    enum KycStatus {
        None,
        Pending,
        Verified,
        Suspended,
        Expired
    }

    /// @notice Functional roles assigned to network participants.
    enum ParticipantRole {
        NONE,
        TREASURY,
        GOVERNANCE,
        CENTRAL_BANK,
        COMMERCIAL_BANK,
        MLP
    }

    /// @notice Core entity representing an institutional identity on the ledger.
    /// @param legalName Registered legal name of the institution.
    /// @param institutionId Opaque institution-level identifier used for quorum de-duplication.
    ///        Every wallet belonging to the same institution shares this value, which lets a
    ///        governance quorum (the AMM circuit-breaker resume) require approvals from distinct
    ///        institutions rather than merely distinct keys.
    /// @param role Functional role (e.g., Commercial Bank) governing access rights.
    /// @param status Current KYC/AML verification state.
    /// @param zkPointer Hash reference to private credentials handled by the privacy layer.
    /// @param certFingerprint SHA-256 fingerprint of the X.509 certificate binding PKI identity to this wallet.
    /// @param lastUpdate Unix timestamp of the last identity modification.
    struct Participant {
        string legalName;
        bytes32 institutionId;
        ParticipantRole role;
        KycStatus status;
        bytes32 zkPointer;
        bytes32 certFingerprint;
        uint256 lastUpdate;
    }
}
