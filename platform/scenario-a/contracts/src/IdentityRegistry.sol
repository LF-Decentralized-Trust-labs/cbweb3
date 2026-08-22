// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {AccessControl} from "@openzeppelin-contracts/access/AccessControl.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";
import {IdentityRegistryLibrary} from "./libraries/IdentityRegistryLibrary.sol";

/// @title IdentityRegistry
/// @notice Implementation of the institutional registry for the CBDC ecosystem.
/// @dev Uses AccessControl for governance and IdentityRegistryLibrary for data integrity.
contract IdentityRegistry is IIdentityRegistry, AccessControl {
    /// @dev Internal storage mapping addresses to their institutional profiles.
    mapping(address => IdentityRegistryLibrary.Participant) private _participants;

    /// @notice Role definition for governance administrators (Central Banks) who register participants.
    bytes32 public constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");

    /// @notice Role definition for compliance authorities who verify (approve) registered participants.
    /// @dev Separation of duties: registration (GOVERNANCE_ROLE) and verification (VERIFIER_ROLE) are
    ///      distinct roles so the entity that onboards a participant is not necessarily the one that
    ///      approves it for transacting. In production these SHOULD be held by different entities. The
    ///      bootstrap admin is granted both so single-operator/local-dev flows keep working.
    bytes32 public constant VERIFIER_ROLE = keccak256("VERIFIER_ROLE");

    /// @notice Initializes the registry and assigns the primary administrator.
    /// @param admin The address granted initial governance, verifier and admin rights.
    constructor(address admin) {
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        _grantRole(GOVERNANCE_ROLE, admin);
        _grantRole(VERIFIER_ROLE, admin);
    }

    /// @inheritdoc IIdentityRegistry
    /// @dev Registration is restricted to GOVERNANCE_ROLE holders. The participant is created in
    ///      the Pending state; a separate verifyParticipant call is required before it can transact.
    function registerParticipant(
        address account,
        string calldata name,
        IdentityRegistryLibrary.ParticipantRole role,
        bytes32 zkPointer,
        bytes32 institutionId
    ) external override onlyRole(GOVERNANCE_ROLE) {
        // A zero institutionId is rejected rather than stored: the AMM resume quorum counts
        // distinct institutions, and an unset id would collide across every participant that
        // also left it unset — turning the institution check into a no-op for all of them.
        if (account == address(0) || institutionId == bytes32(0)) {
            revert InvalidIdentityData();
        }

        // lastUpdate is intentionally NOT set to block.timestamp. This contract is also
        // deployed inside Pente privacy groups, where every group member re-executes the
        // transaction to endorse it. Pente derives block.timestamp from the executor's base
        // block, so an assembler and an endorser that anchor to different base blocks (which
        // happens whenever the base chain advances during a slow cross-node endorsement)
        // compute different values, diverge on the resulting storage root, and the endorsement
        // is rejected with "Execution state mismatch detected in endorsement" — wedging the
        // private transaction forever. Any block.* value written into endorsed state is
        // therefore forbidden here. The field is kept (as 0) to preserve the ABI; the audit
        // timestamp is available off-chain from the ParticipantRegistered event's block.
        _participants[account] = IdentityRegistryLibrary.Participant({
            legalName: name,
            institutionId: institutionId,
            role: role,
            status: IdentityRegistryLibrary.KycStatus.Pending,
            zkPointer: zkPointer,
            certFingerprint: bytes32(0),
            lastUpdate: 0
        });

        emit ParticipantRegistered(account, role, name);
    }

    /// @inheritdoc IIdentityRegistry
    /// @dev Step 2 of onboarding. Restricted to VERIFIER_ROLE holders (distinct from the
    ///      GOVERNANCE_ROLE that registers). Only a Pending participant may be verified.
    function verifyParticipant(address account) external override onlyRole(VERIFIER_ROLE) {
        if (_participants[account].status != IdentityRegistryLibrary.KycStatus.Pending) {
            revert ParticipantNotPending(account);
        }
        _participants[account].status = IdentityRegistryLibrary.KycStatus.Verified;
        // lastUpdate deliberately not written: block.timestamp is nondeterministic across
        // Pente endorsers (see registerParticipant). Audit time is in the IdentityUpdated event.

        emit IdentityUpdated(
            account, IdentityRegistryLibrary.KycStatus.Pending, IdentityRegistryLibrary.KycStatus.Verified
        );
    }

    /// @inheritdoc IIdentityRegistry
    function getInstitutionId(address account) external view override returns (bytes32) {
        return _participants[account].institutionId;
    }

    /// @inheritdoc IIdentityRegistry
    /// @dev Returns true only if the status is exactly 'Verified'.
    function canTransact(address account) external view override returns (bool) {
        IdentityRegistryLibrary.Participant memory p = _participants[account];
        return (p.status == IdentityRegistryLibrary.KycStatus.Verified
                && p.role != IdentityRegistryLibrary.ParticipantRole.NONE);
    }

    /// @inheritdoc IIdentityRegistry
    function canGovern(address account) external view override returns (bool) {
        IdentityRegistryLibrary.Participant memory p = _participants[account];
        return (p.status == IdentityRegistryLibrary.KycStatus.Verified
                && (p.role == IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK
                    || p.role == IdentityRegistryLibrary.ParticipantRole.GOVERNANCE));
    }

    /// @inheritdoc IIdentityRegistry
    function isWhitelisted(address account) external view override returns (bool) {
        return _participants[account].status == IdentityRegistryLibrary.KycStatus.Verified;
    }

    /// @inheritdoc IIdentityRegistry
    function getParticipant(address account)
        external
        view
        override
        returns (IdentityRegistryLibrary.Participant memory)
    {
        return _participants[account];
    }

    /// @inheritdoc IIdentityRegistry
    /// @dev Critical for regulatory compliance and account freezing.
    ///      Separation of duties (R1-10.6 / R2-10.6): promoting an account TO Verified is a
    ///      verification act and is reserved for VERIFIER_ROLE — the same authority gate as
    ///      verifyParticipant. Every other transition (Suspended / Expired freezing, or
    ///      Pending to re-open verification) is a governance/regulatory act under GOVERNANCE_ROLE.
    ///      Without this split a GOVERNANCE_ROLE holder could mint Verified directly through
    ///      updateStatus, bypassing the two-step onboarding control that registerParticipant +
    ///      verifyParticipant enforce.
    function updateStatus(address account, IdentityRegistryLibrary.KycStatus newStatus) external override {
        if (newStatus == IdentityRegistryLibrary.KycStatus.Verified) {
            _checkRole(VERIFIER_ROLE);
        } else {
            _checkRole(GOVERNANCE_ROLE);
        }

        IdentityRegistryLibrary.KycStatus oldStatus = _participants[account].status;
        _participants[account].status = newStatus;
        // lastUpdate deliberately not written: block.timestamp is nondeterministic across
        // Pente endorsers (see registerParticipant). Audit time is in the IdentityUpdated event.

        emit IdentityUpdated(account, oldStatus, newStatus);
    }

    /// @inheritdoc IIdentityRegistry
    /// @dev Binds an X.509 certificate to a participant's on-chain identity.
    function setCertFingerprint(address account, bytes32 fingerprint) external override onlyRole(GOVERNANCE_ROLE) {
        _participants[account].certFingerprint = fingerprint;
        // lastUpdate deliberately not written: block.timestamp is nondeterministic across
        // Pente endorsers (see registerParticipant). Audit time is in the CertificateRegistered event.

        emit CertificateRegistered(account, fingerprint);
    }

    /// @inheritdoc IIdentityRegistry
    function getCertFingerprint(address account) external view override returns (bytes32) {
        return _participants[account].certFingerprint;
    }
}
