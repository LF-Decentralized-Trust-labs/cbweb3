// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {HashTimeLockedContractLibrary} from "../libraries/HashTimeLockedContractLibrary.sol";

/// @title IHashTimeLockedContract
/// @dev Interface for the Hash Time-Lock Contract (Scenario A: Enhanced Correspondent Banking).
/// @dev Orchestrates the cross-border atomic settlement via cryptographic escrows.
interface IHashTimeLockedContract {
    /// @notice Emitted when funds are successfully locked in escrow.
    event LogHTLCLocked(
        bytes32 indexed contractId,
        address indexed sender,
        address indexed receiver,
        address token,
        uint256 amount,
        bytes32 hashLock,
        uint256 timeLock
    );

    /// @notice Emitted when the correct secret is provided and funds are transferred to the receiver.
    event LogHTLCClaimed(bytes32 indexed contractId, bytes32 secret);

    /// @notice Emitted when the time-lock expires and funds are returned to the sender.
    event LogHTLCRefunded(bytes32 indexed contractId);

    /// @notice Locks the specified amount of tCeBm tokens into the contract.
    /// @param contractId Unique identifier for the agreement.
    /// @param receiver The address of the beneficiary.
    /// @param token The address of the tCeBm ERC20 token.
    /// @param amount The amount of tokens to lock.
    /// @param hashLock The SHA-256 hash of the secret.
    /// @param timeLock The Unix timestamp after which the funds can be refunded.
    function lock(
        bytes32 contractId,
        address receiver,
        address token,
        uint256 amount,
        bytes32 hashLock,
        uint256 timeLock
    ) external;

    /// @notice Settles the HTLC by providing the secret preimage.
    /// @param contractId The unique identifier of the locked contract.
    /// @param secret The plaintext string/bytes that hashes to the hashLock.
    function settle(bytes32 contractId, bytes32 secret) external;

    /// @notice Refunds the locked tokens to the sender if the timeLock has expired.
    /// @param contractId The unique identifier of the locked contract.
    function refund(bytes32 contractId) external;

    /// @notice Retrieves the full details of a specific lock contract.
    /// @param contractId The unique identifier of the locked contract.
    /// @return LockDetails struct containing the current state and parameters.
    function getLockDetails(bytes32 contractId) external view returns (HashTimeLockedContractLibrary.LockDetails memory);
}
