// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {IHashTimeLockedContract} from "./interfaces/IHashTimeLockedContract.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";
import {IFXAgreement} from "./interfaces/IFXAgreement.sol";
import {CommitmentHashRegistry} from "./CommitmentHashRegistry.sol";
import {FXAgreementLibrary} from "./libraries/FXAgreementLibrary.sol";
import {HashTimeLockedContractLibrary} from "./libraries/HashTimeLockedContractLibrary.sol";

/// @title HashTimeLockedContract (HTLC) — Coordination Layer
/// @dev Records hashLock/timeLock state and emits events for cross-chain relay (Cacti).
///      Actual token escrow is handled privately via Zeto lock/unlock on the Paladin
///      sidecar. This contract is the publicly observable coordination point that
///      enables atomic settlement between sovereign spokes.
contract HashTimeLockedContract is IHashTimeLockedContract {
    /// @notice The Identity Registry used for participant clearance gates.
    IIdentityRegistry public immutable IDENTITY_REGISTRY;

    /// @notice The FXAgreement contract used for agreement gating (optional).
    IFXAgreement public immutable FX_AGREEMENT;

    /// @notice The CommitmentHashRegistry for fallback FX gating (optional).
    CommitmentHashRegistry public COMMITMENT_HASH_REGISTRY;

    /// @dev Stores lock records indexed by `contractId`.
    mapping(bytes32 => HashTimeLockedContractLibrary.LockDetails) private _locks;

    /// @notice Initializes the HTLC with dependencies for clearance and agreement gating.
    /// @param _identityRegistry Address of the IdentityRegistry contract.
    /// @param _fxAgreement Address of the FXAgreement contract (address(0) to disable).
    /// @param _commitmentHashRegistry Address of the CommitmentHashRegistry for fallback gating (address(0) to disable).
    constructor(address _identityRegistry, address _fxAgreement, address _commitmentHashRegistry) {
        IDENTITY_REGISTRY = IIdentityRegistry(_identityRegistry);
        FX_AGREEMENT = IFXAgreement(_fxAgreement);
        COMMITMENT_HASH_REGISTRY = CommitmentHashRegistry(_commitmentHashRegistry);
    }

    /// @notice Updates the CommitmentHashRegistry address (for deployment flexibility)
    /// @param _newRegistry The new CommitmentHashRegistry address
    function setCommitmentHashRegistry(address _newRegistry) external {
        if (!IDENTITY_REGISTRY.canGovern(msg.sender)) {
            revert HTLC__ParticipantNotVerified(msg.sender);
        }
        COMMITMENT_HASH_REGISTRY = CommitmentHashRegistry(_newRegistry);
    }

    /// @notice Ensures the given account is a verified participant in the IdentityRegistry.
    modifier onlyVerified(address account) {
        _onlyVerified(account);
        _;
    }

    /// @dev Internal check to reduce bytecode duplication at call sites.
    function _onlyVerified(address account) internal view {
        if (!IDENTITY_REGISTRY.canTransact(account)) {
            revert HTLC__ParticipantNotVerified(account);
        }
    }

    /// @inheritdoc IHashTimeLockedContract
    function lock(
        bytes32 contractId,
        address receiver,
        bytes32 hashLock,
        uint256 timeLock,
        bytes32 zetoLockRef,
        bytes32 agreementId
    ) external onlyVerified(msg.sender) onlyVerified(receiver) {
        if (_locks[contractId].state != HashTimeLockedContractLibrary.HTLCState.INVALID) {
            revert HTLC__ContractAlreadyExists();
        }
        if (timeLock <= block.timestamp) {
            revert HTLC__TimeLockExpired();
        }

        // FX Agreement gate (Primary: Pente FXAgreement on-chain)
        if (address(FX_AGREEMENT) != address(0) && agreementId != bytes32(0)) {
            FXAgreementLibrary.FxAgreement memory agreement = FX_AGREEMENT.getAgreement(agreementId);
            if (agreement.state != FXAgreementLibrary.AgreementState.ACCEPTED) {
                revert HTLC__AgreementNotAccepted();
            }
            if (agreement.expiryDate > 0 && block.timestamp > agreement.expiryDate) {
                revert HTLC__AgreementExpired();
            }
        }
        // Fallback: CommitmentHashRegistry (for when FXAgreement unavailable)
        else if (address(COMMITMENT_HASH_REGISTRY) != address(0) && agreementId != bytes32(0)) {
            if (!COMMITMENT_HASH_REGISTRY.isAccepted(agreementId)) {
                revert HTLC__CommitmentNotAccepted();
            }
        }

        _locks[contractId] = HashTimeLockedContractLibrary.LockDetails({
            sender: msg.sender,
            receiver: receiver,
            hashLock: hashLock,
            timeLock: timeLock,
            secret: bytes32(0),
            zetoLockRef: zetoLockRef,
            state: HashTimeLockedContractLibrary.HTLCState.LOCKED
        });

        emit LogHTLCLocked(contractId, msg.sender, receiver, hashLock, timeLock, zetoLockRef);
    }

    /// @inheritdoc IHashTimeLockedContract
    function settle(bytes32 contractId, bytes32 secret) external {
        HashTimeLockedContractLibrary.LockDetails storage lockDetails = _locks[contractId];

        if (lockDetails.state != HashTimeLockedContractLibrary.HTLCState.LOCKED) {
            revert HTLC__ContractNotLocked();
        }

        if (sha256(abi.encodePacked(secret)) != lockDetails.hashLock) {
            revert HTLC__InvalidSecret();
        }

        lockDetails.secret = secret;
        lockDetails.state = HashTimeLockedContractLibrary.HTLCState.SETTLED;

        emit LogHTLCClaimed(contractId, secret);
    }

    /// @inheritdoc IHashTimeLockedContract
    function refund(bytes32 contractId) external {
        HashTimeLockedContractLibrary.LockDetails storage lockDetails = _locks[contractId];

        if (lockDetails.state != HashTimeLockedContractLibrary.HTLCState.LOCKED) {
            revert HTLC__ContractNotLocked();
        }

        if (block.timestamp < lockDetails.timeLock) {
            revert HTLC__TimeLockNotExpired();
        }

        // R2-H-1: restrict refund to the original lock sender. Without this gate any
        // address could trigger the refund after expiry, desynchronizing this public
        // coordination layer from the private Zeto lock/unlock on the Paladin sidecar.
        if (msg.sender != lockDetails.sender) {
            revert HTLC__NotSender(msg.sender);
        }

        lockDetails.state = HashTimeLockedContractLibrary.HTLCState.REFUNDED;

        emit LogHTLCRefunded(contractId);
    }

    /// @inheritdoc IHashTimeLockedContract
    function getLockDetails(bytes32 contractId)
        external
        view
        returns (HashTimeLockedContractLibrary.LockDetails memory)
    {
        return _locks[contractId];
    }
}
