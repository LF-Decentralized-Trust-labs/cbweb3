// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";
import {AccessControl} from "@openzeppelin-contracts/access/AccessControl.sol";

/// @title CommitmentHashRegistry
/// @notice Registry of FX agreement commitment hashes for HTLC gates when Pente is not available.
/// @dev Stores keccak256(tradeId || originAmount || counterAmount || rate) commitments per trade.
///      Used as fallback gate in HashTimeLockedContract when FXAgreement.sol is deployed privately in Pente.
contract CommitmentHashRegistry is AccessControl {
    /// @notice Access control role for governance
    bytes32 public constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");

    /// @notice Reference to shared identity registry for clearance gates
    IIdentityRegistry public immutable IDENTITY_REGISTRY;

    /// @notice Commitment hash state for an FX trade
    enum CommitmentState {
        INVALID, // Not registered or explicitly cancelled
        PENDING, // Registered, awaiting acceptance
        ACCEPTED, // Bilateral acceptance confirmed
        SETTLED, // Settlement completed, no further locks allowed
        CANCELLED // Trade cancelled before acceptance
    }

    /// @notice Commitment record for an FX trade
    struct Commitment {
        bytes32 tradeId;
        bytes32 commitmentHash; // keccak256(tradeId || originAmount || counterAmount || rate)
        address originator;
        address counterpartyB;
        uint256 originAmount;
        uint256 counterAmount;
        uint256 rate;
        CommitmentState state;
        uint256 registeredAt;
        uint256 acceptedAt;
    }

    /// @notice Mapping: commitmentHash → commitment details
    mapping(bytes32 => Commitment) private _commitments;

    /// @notice Mapping: tradeId → commitmentHash (for quick lookup)
    mapping(bytes32 => bytes32) private _tradeIdToHash;

    /// @notice Emitted when a commitment is registered
    event CommitmentRegistered(
        bytes32 indexed tradeId,
        bytes32 indexed commitmentHash,
        address indexed originator,
        address counterpartyB,
        uint256 originAmount,
        uint256 counterAmount,
        uint256 rate
    );

    /// @notice Emitted when a commitment is accepted
    event CommitmentAccepted(bytes32 indexed tradeId, bytes32 indexed commitmentHash);

    /// @notice Emitted when a commitment is cancelled
    event CommitmentCancelled(bytes32 indexed tradeId, bytes32 indexed commitmentHash);

    /// @notice Emitted when a commitment is settled
    event CommitmentSettled(bytes32 indexed tradeId, bytes32 indexed commitmentHash);

    /// @dev Commitment does not exist or is in INVALID state
    error CRG__CommitmentNotFound();

    /// @dev Commitment is not in ACCEPTED state
    error CRG__CommitmentNotAccepted();

    /// @dev Commitment already exists
    error CRG__CommitmentAlreadyExists();

    /// @dev Caller is not authorized
    error CRG__Unauthorized();

    /// @dev Invalid parameters
    error CRG__InvalidParameters();

    /// @dev Invalid state transition
    error CRG__InvalidStateTransition();

    modifier onlyGovernance() {
        if (!IDENTITY_REGISTRY.canGovern(msg.sender)) {
            revert CRG__Unauthorized();
        }
        _;
    }

    constructor(address _identityRegistry) {
        if (_identityRegistry == address(0)) {
            revert CRG__InvalidParameters();
        }
        IDENTITY_REGISTRY = IIdentityRegistry(_identityRegistry);
        _grantRole(GOVERNANCE_ROLE, msg.sender);
    }

    /// @notice Registers an FX agreement commitment hash (for fallback HTLC gating)
    /// @dev Only callable by governance. Requires originator to be verified in identity registry.
    /// @param tradeId Unique identifier for the FX trade
    /// @param originator Address of the deal initiator
    /// @param counterpartyB Address of the accepting party
    /// @param originAmount Amount in origin currency
    /// @param counterAmount Amount in counter currency
    /// @param rate Exchange rate (scaled by 1e18)
    function registerCommitment(
        bytes32 tradeId,
        address originator,
        address counterpartyB,
        uint256 originAmount,
        uint256 counterAmount,
        uint256 rate
    ) external onlyGovernance {
        if (tradeId == bytes32(0) || originator == address(0) || counterpartyB == address(0)) {
            revert CRG__InvalidParameters();
        }
        if (originAmount == 0 || counterAmount == 0 || rate == 0) {
            revert CRG__InvalidParameters();
        }

        bytes32 commitmentHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        if (_commitments[commitmentHash].state != CommitmentState.INVALID) {
            revert CRG__CommitmentAlreadyExists();
        }

        _commitments[commitmentHash] = Commitment({
            tradeId: tradeId,
            commitmentHash: commitmentHash,
            originator: originator,
            counterpartyB: counterpartyB,
            originAmount: originAmount,
            counterAmount: counterAmount,
            rate: rate,
            state: CommitmentState.PENDING,
            registeredAt: block.timestamp,
            acceptedAt: 0
        });

        _tradeIdToHash[tradeId] = commitmentHash;

        emit CommitmentRegistered(tradeId, commitmentHash, originator, counterpartyB, originAmount, counterAmount, rate);
    }

    /// @notice Accepts a pending commitment (both parties have agreed)
    /// @dev Only callable by governance
    /// @param commitmentHash The commitment hash to accept
    function acceptCommitment(bytes32 commitmentHash) external onlyGovernance {
        Commitment storage commitment = _commitments[commitmentHash];

        if (commitment.state != CommitmentState.PENDING) {
            revert CRG__InvalidStateTransition();
        }

        commitment.state = CommitmentState.ACCEPTED;
        commitment.acceptedAt = block.timestamp;

        emit CommitmentAccepted(commitment.tradeId, commitmentHash);
    }

    /// @notice Settles a commitment (settlement completed)
    /// @dev Only callable by governance. Prevents further HTLC locks using this commitment.
    /// @param commitmentHash The commitment hash to settle
    function settleCommitment(bytes32 commitmentHash) external onlyGovernance {
        Commitment storage commitment = _commitments[commitmentHash];

        if (commitment.state != CommitmentState.ACCEPTED) {
            revert CRG__InvalidStateTransition();
        }

        commitment.state = CommitmentState.SETTLED;

        emit CommitmentSettled(commitment.tradeId, commitmentHash);
    }

    /// @notice Cancels a commitment (trade aborted)
    /// @dev Only callable by governance. Can be called from any non-terminal state.
    /// @param commitmentHash The commitment hash to cancel
    function cancelCommitment(bytes32 commitmentHash) external onlyGovernance {
        Commitment storage commitment = _commitments[commitmentHash];

        if (
            commitment.state == CommitmentState.INVALID || commitment.state == CommitmentState.SETTLED
                || commitment.state == CommitmentState.CANCELLED
        ) {
            revert CRG__InvalidStateTransition();
        }

        commitment.state = CommitmentState.CANCELLED;

        emit CommitmentCancelled(commitment.tradeId, commitmentHash);
    }

    /// @notice Checks if a commitment is currently accepted and allows HTLC locking
    /// @param commitmentHash The commitment hash to check
    /// @return True if commitment is in ACCEPTED state
    function isAccepted(bytes32 commitmentHash) external view returns (bool) {
        Commitment storage commitment = _commitments[commitmentHash];
        return commitment.state == CommitmentState.ACCEPTED;
    }

    /// @notice Retrieves the current state of a commitment
    /// @param commitmentHash The commitment hash to query
    /// @return The commitment state (INVALID, PENDING, ACCEPTED, SETTLED, CANCELLED)
    function getCommitmentState(bytes32 commitmentHash) external view returns (CommitmentState) {
        return _commitments[commitmentHash].state;
    }

    /// @notice Retrieves full commitment details
    /// @param commitmentHash The commitment hash to query
    /// @return The commitment struct (reverts if not found)
    function getCommitment(bytes32 commitmentHash) external view returns (Commitment memory) {
        Commitment memory commitment = _commitments[commitmentHash];
        if (commitment.state == CommitmentState.INVALID) {
            revert CRG__CommitmentNotFound();
        }
        return commitment;
    }

    /// @notice Retrieves commitment hash for a trade ID
    /// @param tradeId The trade identifier
    /// @return The commitment hash (reverts if not found)
    function getCommitmentHashByTradeId(bytes32 tradeId) external view returns (bytes32) {
        bytes32 commitmentHash = _tradeIdToHash[tradeId];
        if (_commitments[commitmentHash].state == CommitmentState.INVALID) {
            revert CRG__CommitmentNotFound();
        }
        return commitmentHash;
    }
}
