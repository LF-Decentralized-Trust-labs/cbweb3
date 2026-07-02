// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {AccessControl} from "@openzeppelin-contracts/access/AccessControl.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";
import {IdentityRegistryLibrary} from "./libraries/IdentityRegistryLibrary.sol";

/// @title IdentityRegistry
/// @notice Implementation of the institutional registry for the CBDC ecosystem.
/// @dev Uses AccessControl for governance and IdentityRegistryLibrary for data integrity.
contract IdentityRegistry is IIdentityRegistry, AccessControl {
    /// @dev Internal storage mapping addresses to their institutional profiles.
    mapping(address => IdentityRegistryLibrary.Participant) private _participants;

    /// @notice Role definition for governance administrators (Central Banks).
    bytes32 public constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");

    /// @notice Initializes the registry and assigns the primary administrator.
    /// @param admin The address granted initial governance and admin rights.
    constructor(address admin) {
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        _grantRole(GOVERNANCE_ROLE, admin);
    }

    /// @inheritdoc IIdentityRegistry
    /// @dev Registration is restricted to GOVERNANCE_ROLE holders.
    function registerParticipant(
        address account,
        string calldata name,
        IdentityRegistryLibrary.ParticipantRole role,
        bytes32 zkPointer
    ) external override onlyRole(GOVERNANCE_ROLE) {
        if (account == address(0)) {
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
            role: role,
            status: IdentityRegistryLibrary.KycStatus.Verified,
            zkPointer: zkPointer,
            certFingerprint: bytes32(0),
            lastUpdate: 0
        });

        emit ParticipantRegistered(account, role, name);
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
    function updateStatus(address account, IdentityRegistryLibrary.KycStatus newStatus)
        external
        override
        onlyRole(GOVERNANCE_ROLE)
    {
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
