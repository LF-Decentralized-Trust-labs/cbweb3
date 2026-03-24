// SPDX-License-Identifier: UNLICENSED
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

        _participants[account] = IdentityRegistryLibrary.Participant({
            legalName: name,
            role: role,
            status: IdentityRegistryLibrary.KycStatus.Verified,
            zkPointer: zkPointer,
            lastUpdate: block.timestamp
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
        _participants[account].lastUpdate = block.timestamp;

        emit IdentityUpdated(account, oldStatus, newStatus);
    }
}
