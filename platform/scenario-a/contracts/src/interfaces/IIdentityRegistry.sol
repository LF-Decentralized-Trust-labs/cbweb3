// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {IdentityRegistryLibrary} from "../libraries/IdentityRegistryLibrary.sol";

/// @title IIdentityRegistry
/// @notice Interface for the central Identity and Compliance Registry.
/// @dev Standardizes how external modules (AMM, HTLC) interact with identity data.
interface IIdentityRegistry {
    /// @notice Emitted when a new institution is onboarded to the network.
    event ParticipantRegistered(address indexed account, IdentityRegistryLibrary.ParticipantRole role, string name);

    /// @notice Emitted when an institution's status is modified by governance.
    event IdentityUpdated(
        address indexed account,
        IdentityRegistryLibrary.KycStatus oldStatus,
        IdentityRegistryLibrary.KycStatus newStatus
    );

    /// @notice Emitted when a certificate fingerprint is registered or updated.
    event CertificateRegistered(address indexed account, bytes32 certFingerprint);

    /// @notice Error thrown when an unverified account attempts a restricted operation.
    error ParticipantNotVerified(address account);

    /// @notice Error thrown when verifyParticipant targets an account that is not in Pending.
    /// @dev The verification step is only valid as a Pending -> Verified transition. Verifying an
    ///      unregistered account, or one that is already Verified/Suspended/Expired, is rejected.
    error ParticipantNotPending(address account);

    /// @notice Error thrown when registration data (e.g., address zero) is invalid.
    error InvalidIdentityData();

    /// @notice Verifies if an address is active and whitelisted.
    /// @param account The wallet address to check.
    /// @return True if the participant is verified, false otherwise.
    function isWhitelisted(address account) external view returns (bool);

    /// @notice Returns the full identity profile for an institution.
    /// @param account The wallet address of the participant.
    /// @return A Participant struct with all registered metadata.
    function getParticipant(address account) external view returns (IdentityRegistryLibrary.Participant memory);

    /// @notice Gatekeeper function to authorize or reject financial transactions.
    /// @param account The address initiating a transfer or swap.
    /// @return bool True if authorized, false if the account is blocked or unknown.
    function canTransact(address account) external view returns (bool);

    /// @notice Gatekeeper function to verify governance-level authorization.
    /// @param account The address to verify.
    /// @return bool True if the account is Verified with a governance-capable role (CENTRAL_BANK or GOVERNANCE).
    function canGovern(address account) external view returns (bool);

    /// @notice Registers a new participant in the Pending state. Restricted to governance authorities.
    /// @dev Step 1 of the two-step onboarding. Registration alone does NOT make the account
    ///      transactable: the participant is created with KycStatus.Pending and a separate
    ///      verifyParticipant call (by a VERIFIER_ROLE holder) is required to reach Verified.
    ///      This enforces separation of duties — the entity that registers must not be the same
    ///      one that approves the participant for transacting.
    /// @param account Target wallet address.
    /// @param name Legal name of the entity.
    /// @param role Assigned functional role in the network.
    /// @param zkPointer Link to the entity's private compliance proofs.
    /// @param institutionId Opaque institution-level identifier shared by all wallets of the same
    ///        institution. Used by quorum mechanisms (the circuit-breaker resume) to enforce
    ///        approval from distinct institutions rather than distinct addresses. Must be non-zero.
    function registerParticipant(
        address account,
        string calldata name,
        IdentityRegistryLibrary.ParticipantRole role,
        bytes32 zkPointer,
        bytes32 institutionId
    ) external;

    /// @notice Verifies a previously registered participant, moving them Pending -> Verified.
    /// @dev Step 2 of the two-step onboarding. Restricted to VERIFIER_ROLE holders. Reverts with
    ///      ParticipantNotPending if the account is not currently in the Pending state.
    /// @param account The wallet address to promote from Pending to Verified.
    function verifyParticipant(address account) external;

    /// @notice Returns the institution identifier for a registered participant.
    /// @param account The wallet address to query.
    /// @return The institution-level bytes32 identifier, or bytes32(0) if not registered.
    function getInstitutionId(address account) external view returns (bytes32);

    /// @notice Modifies the status of a participant (e.g., suspension).
    /// @param account The address to be updated.
    /// @param newStatus The target KycStatus.
    function updateStatus(address account, IdentityRegistryLibrary.KycStatus newStatus) external;

    /// @notice Registers or updates the X.509 certificate fingerprint for a participant.
    /// @param account The wallet address whose certificate is being bound.
    /// @param fingerprint SHA-256 hash of the DER-encoded X.509 certificate.
    function setCertFingerprint(address account, bytes32 fingerprint) external;

    /// @notice Returns the certificate fingerprint for a participant.
    /// @param account The wallet address to query.
    /// @return The SHA-256 fingerprint, or bytes32(0) if not set.
    function getCertFingerprint(address account) external view returns (bytes32);
}
