// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {IHashTimeLockedContract} from "./interfaces/IHashTimeLockedContract.sol";
import {HashTimeLockedContractLibrary} from "./libraries/HashTimeLockedContractLibrary.sol";
import {IERC20} from "@openzeppelin-contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin-contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin-contracts/utils/ReentrancyGuard.sol";

/// @title HashTimeLockedContract (HTLC)
/// @dev Escrow contract for atomic settlement flows using hash-lock and time-lock controls.
/// @notice This contract locks ERC20 funds, allows settlement with the correct secret preimage,
/// and allows refund after expiry. It is protected against re-entrancy on state-changing flows.
contract HashTimeLockedContract is IHashTimeLockedContract, ReentrancyGuard {
    /// @dev Utilises SafeERC20 wrappers for secure ERC20 transfers.
    using SafeERC20 for IERC20;

    /// @dev Stores lock records indexed by `contractId`.
    mapping(bytes32 => HashTimeLockedContractLibrary.LockDetails) private _locks;

    /// @notice Locks funds in escrow under a hash-lock and time-lock.
    /// @dev Reverts if the lock already exists, amount is zero, or time-lock is already expired.
    /// @inheritdoc IHashTimeLockedContract
    function lock(
        bytes32 contractId,
        address receiver,
        address token,
        uint256 amount,
        bytes32 hashLock,
        uint256 timeLock
    ) external nonReentrant {
        if (_locks[contractId].state != HashTimeLockedContractLibrary.HTLCState.INVALID) {
            revert HTLC__ContractAlreadyExists();
        }
        if (amount == 0) {
            revert HTLC__InvalidAmount();
        }
        if (timeLock <= block.timestamp) {
            revert HTLC__TimeLockExpired();
        }

        _locks[contractId] = HashTimeLockedContractLibrary.LockDetails({
            sender: msg.sender,
            receiver: receiver,
            token: token,
            amount: amount,
            hashLock: hashLock,
            timeLock: timeLock,
            secret: bytes32(0),
            state: HashTimeLockedContractLibrary.HTLCState.LOCKED
        });

        IERC20(token).safeTransferFrom(msg.sender, address(this), amount);

        emit LogHTLCLocked(contractId, msg.sender, receiver, token, amount, hashLock, timeLock);
    }

    /// @notice Settles a lock by revealing a valid secret preimage.
    /// @dev Reverts unless the lock is in `LOCKED` state and the secret hashes to the stored hash-lock.
    /// @inheritdoc IHashTimeLockedContract
    function settle(bytes32 contractId, bytes32 secret) external nonReentrant {
        HashTimeLockedContractLibrary.LockDetails storage lockDetails = _locks[contractId];

        if (lockDetails.state != HashTimeLockedContractLibrary.HTLCState.LOCKED) {
            revert HTLC__ContractNotLocked();
        }

        if (sha256(abi.encodePacked(secret)) != lockDetails.hashLock) {
            revert HTLC__InvalidSecret();
        }

        lockDetails.secret = secret;
        lockDetails.state = HashTimeLockedContractLibrary.HTLCState.SETTLED;

        IERC20(lockDetails.token).safeTransfer(lockDetails.receiver, lockDetails.amount);

        emit LogHTLCClaimed(contractId, secret);
    }

    /// @notice Refunds locked funds to the original sender after expiry.
    /// @dev Reverts unless the lock is in `LOCKED` state and the time-lock has expired.
    /// @inheritdoc IHashTimeLockedContract
    function refund(bytes32 contractId) external nonReentrant {
        HashTimeLockedContractLibrary.LockDetails storage lockDetails = _locks[contractId];

        if (lockDetails.state != HashTimeLockedContractLibrary.HTLCState.LOCKED) {
            revert HTLC__ContractNotLocked();
        }

        if (block.timestamp < lockDetails.timeLock) {
            revert HTLC__TimeLockNotExpired();
        }

        lockDetails.state = HashTimeLockedContractLibrary.HTLCState.REFUNDED;

        IERC20(lockDetails.token).safeTransfer(lockDetails.sender, lockDetails.amount);

        emit LogHTLCRefunded(contractId);
    }

    /// @notice Returns the full lock details for a given contract identifier.
    /// @dev Returns zero-initialised values when the lock does not exist.
    /// @inheritdoc IHashTimeLockedContract
    function getLockDetails(bytes32 contractId)
        external
        view
        returns (HashTimeLockedContractLibrary.LockDetails memory)
    {
        return _locks[contractId];
    }
}
