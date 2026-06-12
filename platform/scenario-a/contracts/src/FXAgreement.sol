// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {IFXAgreement} from "./interfaces/IFXAgreement.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";
import {FXAgreementLibrary} from "./libraries/FXAgreementLibrary.sol";
import {ReentrancyGuard} from "@openzeppelin-contracts/utils/ReentrancyGuard.sol";

/// @title FXAgreement
/// @notice On-chain bilateral FX agreement registry (REQ-FX-001, REQ-FX-003).
/// @dev Manages the lifecycle of cross-border FX deals: propose → accept/reject/cancel → settle.
///      All participants must be verified in the IdentityRegistry (clearance gate).
///      Settlement is restricted to governance-capable accounts (Central Banks).
contract FXAgreement is IFXAgreement, ReentrancyGuard {
    /// @notice The Identity Registry used for participant clearance gates.
    IIdentityRegistry public immutable IDENTITY_REGISTRY;

    /// @dev Stores FX agreements indexed by `tradeId`.
    mapping(bytes32 => FXAgreementLibrary.FxAgreement) private _agreements;

    /// @notice Initializes the FXAgreement registry with the IdentityRegistry for clearance gates.
    /// @param _identityRegistry Address of the IdentityRegistry contract.
    constructor(address _identityRegistry) {
        if (_identityRegistry == address(0)) revert FXA__InvalidParameters();
        IDENTITY_REGISTRY = IIdentityRegistry(_identityRegistry);
    }

    /// @notice Ensures the given account is a verified participant in the IdentityRegistry.
    modifier onlyVerified(address account) {
        _onlyVerified(account);
        _;
    }

    /// @dev Internal check extracted from the modifier to reduce bytecode duplication at call sites.
    function _onlyVerified(address account) internal view {
        if (!IDENTITY_REGISTRY.canTransact(account)) {
            revert FXA__ParticipantNotVerified(account);
        }
    }

    /// @inheritdoc IFXAgreement
    /// @dev Reverts if the trade already exists, addresses are zero, amounts are zero, or the proposal is expired.
    function propose(
        bytes32 tradeId,
        address counterpartyB,
        address settlementAgent,
        address custodian,
        address beneficiary,
        uint256 originAmount,
        uint256 counterAmount,
        bytes32 originCurrency,
        bytes32 counterCurrency,
        uint256 rate,
        uint256 expiryDate
    ) external onlyVerified(msg.sender) {
        if (_agreements[tradeId].state != FXAgreementLibrary.AgreementState.INVALID) {
            revert FXA__TradeAlreadyExists();
        }
        if (counterpartyB == address(0)) {
            revert FXA__InvalidParameters();
        }
        if (originAmount == 0 || counterAmount == 0) {
            revert FXA__InvalidParameters();
        }
        if (originCurrency == bytes32(0) || counterCurrency == bytes32(0)) {
            revert FXA__InvalidParameters();
        }
        if (rate == 0) {
            revert FXA__InvalidParameters();
        }
        if (expiryDate <= block.timestamp) {
            revert FXA__AgreementExpired();
        }

        _agreements[tradeId] = FXAgreementLibrary.FxAgreement({
            tradeId: tradeId,
            originator: msg.sender,
            counterpartyB: counterpartyB,
            settlementAgent: settlementAgent,
            custodian: custodian,
            beneficiary: beneficiary,
            originAmount: originAmount,
            counterAmount: counterAmount,
            originCurrency: originCurrency,
            counterCurrency: counterCurrency,
            rate: rate,
            expiryDate: expiryDate,
            state: FXAgreementLibrary.AgreementState.PROPOSED
        });

        emit AgreementProposed(
            tradeId, msg.sender, counterpartyB, settlementAgent, custodian, beneficiary,
            originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate
        );
    }

    /// @inheritdoc IFXAgreement
    /// @dev Only callable by governance. Does NOT verify originator via canTransact (remote party).
    function proposeOnBehalf(
        bytes32 tradeId,
        address originator,
        address counterpartyB,
        address settlementAgent,
        address custodian,
        address beneficiary,
        uint256 originAmount,
        uint256 counterAmount,
        bytes32 originCurrency,
        bytes32 counterCurrency,
        uint256 rate,
        uint256 expiryDate
    ) external {
        if (!IDENTITY_REGISTRY.canGovern(msg.sender)) {
            revert FXA__Unauthorized();
        }
        if (_agreements[tradeId].state != FXAgreementLibrary.AgreementState.INVALID) {
            revert FXA__TradeAlreadyExists();
        }
        if (counterpartyB == address(0) || originator == address(0)) {
            revert FXA__InvalidParameters();
        }
        if (originAmount == 0 || counterAmount == 0) {
            revert FXA__InvalidParameters();
        }
        if (originCurrency == bytes32(0) || counterCurrency == bytes32(0)) {
            revert FXA__InvalidParameters();
        }
        if (rate == 0) {
            revert FXA__InvalidParameters();
        }
        if (expiryDate <= block.timestamp) {
            revert FXA__AgreementExpired();
        }

        _agreements[tradeId] = FXAgreementLibrary.FxAgreement({
            tradeId: tradeId,
            originator: originator,
            counterpartyB: counterpartyB,
            settlementAgent: settlementAgent,
            custodian: custodian,
            beneficiary: beneficiary,
            originAmount: originAmount,
            counterAmount: counterAmount,
            originCurrency: originCurrency,
            counterCurrency: counterCurrency,
            rate: rate,
            expiryDate: expiryDate,
            state: FXAgreementLibrary.AgreementState.PROPOSED
        });

        emit AgreementProposed(
            tradeId, originator, counterpartyB, settlementAgent, custodian, beneficiary,
            originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate
        );
    }

    /// @inheritdoc IFXAgreement
    /// @dev Only callable by governance. Transitions PROPOSED → ACCEPTED without checking msg.sender == counterpartyB.
    function acceptOnBehalf(bytes32 tradeId) external {
        if (!IDENTITY_REGISTRY.canGovern(msg.sender)) {
            revert FXA__Unauthorized();
        }

        FXAgreementLibrary.FxAgreement storage agreement = _agreements[tradeId];
        if (agreement.state != FXAgreementLibrary.AgreementState.PROPOSED) {
            revert FXA__InvalidStateTransition();
        }
        if (agreement.expiryDate > 0 && block.timestamp > agreement.expiryDate) {
            revert FXA__AgreementExpired();
        }

        agreement.state = FXAgreementLibrary.AgreementState.ACCEPTED;

        emit AgreementAccepted(tradeId);
    }

    /// @inheritdoc IFXAgreement
    /// @dev Only callable by governance. Transitions PROPOSED → REJECTED.
    function rejectOnBehalf(bytes32 tradeId) external {
        if (!IDENTITY_REGISTRY.canGovern(msg.sender)) {
            revert FXA__Unauthorized();
        }

        FXAgreementLibrary.FxAgreement storage agreement = _agreements[tradeId];
        if (agreement.state != FXAgreementLibrary.AgreementState.PROPOSED) {
            revert FXA__InvalidStateTransition();
        }

        agreement.state = FXAgreementLibrary.AgreementState.REJECTED;

        emit AgreementRejected(tradeId);
    }

    /// @inheritdoc IFXAgreement
    /// @dev Only callable by counterpartyB while the agreement is in PROPOSED state and not expired.
    function accept(bytes32 tradeId) external onlyVerified(msg.sender) {
        FXAgreementLibrary.FxAgreement storage agreement = _agreements[tradeId];
        if (agreement.state != FXAgreementLibrary.AgreementState.PROPOSED) {
            revert FXA__InvalidStateTransition();
        }
        if (msg.sender != agreement.counterpartyB) {
            revert FXA__Unauthorized();
        }
        if (block.timestamp > agreement.expiryDate) {
            revert FXA__AgreementExpired();
        }

        agreement.state = FXAgreementLibrary.AgreementState.ACCEPTED;

        emit AgreementAccepted(tradeId);
    }

    /// @inheritdoc IFXAgreement
    /// @dev Only callable by counterpartyB while the agreement is in PROPOSED state.
    function reject(bytes32 tradeId) external onlyVerified(msg.sender) {
        FXAgreementLibrary.FxAgreement storage agreement = _agreements[tradeId];
        if (agreement.state != FXAgreementLibrary.AgreementState.PROPOSED) {
            revert FXA__InvalidStateTransition();
        }
        if (msg.sender != agreement.counterpartyB) {
            revert FXA__Unauthorized();
        }

        agreement.state = FXAgreementLibrary.AgreementState.REJECTED;

        emit AgreementRejected(tradeId);
    }

    /// @inheritdoc IFXAgreement
    /// @dev Only callable by counterpartyA while the agreement is in PROPOSED state.
    function cancel(bytes32 tradeId) external onlyVerified(msg.sender) {
        FXAgreementLibrary.FxAgreement storage agreement = _agreements[tradeId];
        if (agreement.state != FXAgreementLibrary.AgreementState.PROPOSED) {
            revert FXA__InvalidStateTransition();
        }
        if (msg.sender != agreement.originator) {
            revert FXA__Unauthorized();
        }

        agreement.state = FXAgreementLibrary.AgreementState.CANCELLED;

        emit AgreementCancelled(tradeId);
    }

    /// @inheritdoc IFXAgreement
    /// @dev Restricted to governance-capable accounts (Central Banks). Requires ACCEPTED state.
    function settle(bytes32 tradeId) external nonReentrant {
        if (!IDENTITY_REGISTRY.canGovern(msg.sender)) {
            revert FXA__Unauthorized();
        }

        FXAgreementLibrary.FxAgreement storage agreement = _agreements[tradeId];
        if (agreement.state != FXAgreementLibrary.AgreementState.ACCEPTED) {
            revert FXA__InvalidStateTransition();
        }

        agreement.state = FXAgreementLibrary.AgreementState.SETTLED;

        emit AgreementSettled(tradeId);
    }

    /// @inheritdoc IFXAgreement
    /// @dev Reverts with FXA__TradeNotFound when the trade does not exist.
    function getAgreement(bytes32 tradeId) external view returns (FXAgreementLibrary.FxAgreement memory) {
        FXAgreementLibrary.FxAgreement memory agreement = _agreements[tradeId];
        if (agreement.state == FXAgreementLibrary.AgreementState.INVALID) {
            revert FXA__TradeNotFound();
        }
        return agreement;
    }
}
