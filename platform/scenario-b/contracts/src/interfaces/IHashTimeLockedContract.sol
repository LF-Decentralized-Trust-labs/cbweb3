// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {HashTimeLockedContractLibrary} from "../libraries/HashTimeLockedContractLibrary.sol";

/// @title IHashTimeLockedContract
/// @dev Coordination-only interface for the Hash Time-Lock Contract (Scenario A).
/// @dev Records hashLock/timeLock and emits events for cross-chain relay (Cacti).
///      Actual token movement is handled by Zeto lock/unlock via Paladin sidecar.
interface IHashTimeLockedContract {
    /// @notice Emitted when a lock coordination record is created.
    event LogHTLCLocked(
        bytes32 indexed contractId,
        address indexed sender,
        address indexed receiver,
        bytes32 hashLock,
        uint256 timeLock,
        bytes32 zetoLockRef
    );

    /// @notice Emitted when the correct secret is provided and the lock is settled.
    event LogHTLCClaimed(bytes32 indexed contractId, bytes32 secret);

    /// @notice Emitted when the time-lock expires and the lock is refunded.
    event LogHTLCRefunded(bytes32 indexed contractId);

    /// @dev Custom errors for gas-efficient HTLC failure paths.
    error HTLC__ContractAlreadyExists();
    error HTLC__ContractNotLocked();
    error HTLC__InvalidSecret();
    error HTLC__TimeLockNotExpired();
    error HTLC__TimeLockExpired();
    error HTLC__ParticipantNotVerified(address account);
    error HTLC__AgreementNotAccepted();
    error HTLC__AgreementExpired();
    error HTLC__CommitmentNotAccepted();

    /// @notice Records a lock coordination entry linked to a private Zeto lock.
    /// @param contractId Unique identifier for the agreement.
    /// @param receiver The address of the beneficiary.
    /// @param hashLock The SHA-256 hash of the secret.
    /// @param timeLock The Unix timestamp after which the lock can be refunded.
    /// @param zetoLockRef Reference to the private Zeto lock transaction.
    /// @param agreementId The FX agreement ID to gate against (bytes32(0) to skip).
    function lock(
        bytes32 contractId,
        address receiver,
        bytes32 hashLock,
        uint256 timeLock,
        bytes32 zetoLockRef,
        bytes32 agreementId
    ) external;

    /// @notice Settles the HTLC by providing the secret preimage.
    /// @param contractId The unique identifier of the locked contract.
    /// @param secret The plaintext bytes that hash to the hashLock.
    function settle(bytes32 contractId, bytes32 secret) external;

    /// @notice Marks the lock as refunded after the timeLock has expired.
    /// @param contractId The unique identifier of the locked contract.
    function refund(bytes32 contractId) external;

    /// @notice Retrieves the full details of a specific lock contract.
    /// @param contractId The unique identifier of the locked contract.
    /// @return LockDetails struct containing the current state and parameters.
    function getLockDetails(bytes32 contractId) external view returns (HashTimeLockedContractLibrary.LockDetails memory);
}
