// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {IdentityRegistryLibrary} from "../libraries/IdentityRegistryLibrary.sol";

/// @title IIdentityRegistry
/// @notice Interface for the central Identity and Compliance Registry.
/// @dev Standardizes how external modules (AMM, HTLC) interact with identity data.
interface IIdentityRegistry {
    /// @notice Emitted when a new institution is onboarded to the network.
    event ParticipantRegistered(
        address indexed account, IdentityRegistryLibrary.ParticipantRole role, string name
    );

    /// @notice Emitted when an institution's status is modified by governance.
    event IdentityUpdated(
        address indexed account,
        IdentityRegistryLibrary.KycStatus oldStatus,
        IdentityRegistryLibrary.KycStatus newStatus
    );

    /// @notice Error thrown when an unverified account attempts a restricted operation.
    error ParticipantNotVerified(address account);

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

    /// @notice Registers a new participant. Restricted to governance authorities.
    /// @param account Target wallet address.
    /// @param name Legal name of the entity.
    /// @param role Assigned functional role in the network.
    /// @param zkPointer Link to the entity's private compliance proofs.
    function registerParticipant(
        address account,
        string calldata name,
        IdentityRegistryLibrary.ParticipantRole role,
        bytes32 zkPointer
    ) external;

    /// @notice Modifies the status of a participant (e.g., suspension).
    /// @param account The address to be updated.
    /// @param newStatus The target KycStatus.
    function updateStatus(address account, IdentityRegistryLibrary.KycStatus newStatus) external;
}
