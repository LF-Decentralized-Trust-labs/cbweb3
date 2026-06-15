// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {FXAgreementLibrary} from "../libraries/FXAgreementLibrary.sol";

/// @title IFXAgreement
/// @notice Interface for the on-chain bilateral FX agreement registry (REQ-FX-001).
/// @dev Manages the lifecycle of cross-border FX deals: propose → accept/reject/cancel → settle.
interface IFXAgreement {
    /// @notice Emitted when a new FX agreement is proposed.
    event AgreementProposed(
        bytes32 indexed tradeId,
        address indexed originator,
        address indexed counterpartyB,
        address settlementAgent,
        address custodian,
        address beneficiary,
        uint256 originAmount,
        uint256 counterAmount,
        bytes32 originCurrency,
        bytes32 counterCurrency,
        uint256 rate,
        uint256 expiryDate
    );

    /// @notice Emitted when the counterparty accepts the agreement.
    event AgreementAccepted(bytes32 indexed tradeId);

    /// @notice Emitted when the agreement is settled (funds exchanged).
    event AgreementSettled(bytes32 indexed tradeId);

    /// @notice Emitted when the counterparty rejects the agreement.
    event AgreementRejected(bytes32 indexed tradeId);

    /// @notice Emitted when the initiator cancels a proposed agreement.
    event AgreementCancelled(bytes32 indexed tradeId);

    /// @dev TradeID already exists.
    error FXA__TradeAlreadyExists();

    /// @dev TradeID does not exist or is in INVALID state.
    error FXA__TradeNotFound();

    /// @dev Agreement is not in the expected state for this transition.
    error FXA__InvalidStateTransition();

    /// @dev Caller is not authorised for this action.
    error FXA__Unauthorized();

    /// @dev Participant not verified in the IdentityRegistry.
    error FXA__ParticipantNotVerified(address account);

    /// @dev Agreement has expired.
    error FXA__AgreementExpired();

    /// @dev Invalid input parameters.
    error FXA__InvalidParameters();

    /// @notice Creates a new FX agreement proposal.
    /// @param tradeId Unique identifier for the deal.
    /// @param counterpartyB The address of the accepting party.
    /// @param settlementAgent The address of the settlement agent.
    /// @param custodian The address of the custodian.
    /// @param beneficiary The address of the beneficiary.
    /// @param originAmount Amount in the origin currency.
    /// @param counterAmount Amount in the counter currency.
    /// @param originCurrency ISO 4217 code for the origin currency.
    /// @param counterCurrency ISO 4217 code for the counter currency.
    /// @param rate Agreed exchange rate (scaled by 1e18).
    /// @param expiryDate Unix timestamp after which the proposal expires.
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
    ) external;

    /// @notice Creates a new FX agreement proposal on behalf of a remote originator (governance only).
    /// @param tradeId Unique identifier for the deal.
    /// @param originator The address of the deal initiator (remote party).
    /// @param counterpartyB The address of the accepting party.
    /// @param settlementAgent The address of the settlement agent.
    /// @param custodian The address of the custodian.
    /// @param beneficiary The address of the beneficiary.
    /// @param originAmount Amount in the origin currency.
    /// @param counterAmount Amount in the counter currency.
    /// @param originCurrency ISO 4217 code for the origin currency.
    /// @param counterCurrency ISO 4217 code for the counter currency.
    /// @param rate Agreed exchange rate (scaled by 1e18).
    /// @param expiryDate Unix timestamp after which the proposal expires.
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
    ) external;

    /// @notice Governance accepts the proposed agreement on behalf of counterpartyB.
    /// @param tradeId The trade to accept.
    function acceptOnBehalf(bytes32 tradeId) external;

    /// @notice Governance rejects the proposed agreement on behalf of counterpartyB.
    /// @param tradeId The trade to reject.
    function rejectOnBehalf(bytes32 tradeId) external;

    /// @notice Counterparty accepts the proposed agreement.
    /// @param tradeId The trade to accept.
    function accept(bytes32 tradeId) external;

    /// @notice Counterparty rejects the proposed agreement.
    /// @param tradeId The trade to reject.
    function reject(bytes32 tradeId) external;

    /// @notice Initiator cancels a proposed agreement before acceptance.
    /// @param tradeId The trade to cancel.
    function cancel(bytes32 tradeId) external;

    /// @notice Governance settles an accepted agreement (marks as executed).
    /// @param tradeId The trade to settle.
    function settle(bytes32 tradeId) external;

    /// @notice Returns the full details of an FX agreement.
    /// @param tradeId The trade to query.
    /// @return The FXAgreement struct.
    function getAgreement(bytes32 tradeId) external view returns (FXAgreementLibrary.FxAgreement memory);
}
