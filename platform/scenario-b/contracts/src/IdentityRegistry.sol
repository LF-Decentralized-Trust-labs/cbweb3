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

    /// @notice Tracks addresses that hold the Liquidity Provider role (005-cooperative-liquidity).
    mapping(address => bool) private _liquidityProviders;

    /// @notice Maps each token contract to its issuing Central Bank on-chain address.
    /// @dev Populated via setCentralBankOf. Consumed by PairRegistry for bilateral authorization (D9).
    mapping(address => address) private _centralBankOf;

    /// @notice Role definition for governance administrators (Central Banks) who register participants.
    bytes32 public constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");

    /// @notice Role definition for compliance authorities who verify (approve) registered participants.
    /// @dev Separation of duties: registration (GOVERNANCE_ROLE) and verification (VERIFIER_ROLE) are
    ///      distinct roles so the entity that onboards a participant is not necessarily the one that
    ///      approves it for transacting. In production these SHOULD be held by different entities. The
    ///      bootstrap admin is granted both so single-operator/local-dev flows keep working.
    bytes32 public constant VERIFIER_ROLE = keccak256("VERIFIER_ROLE");

    /// @notice Emitted when an address is granted the Liquidity Provider role.
    event LogLiquidityProviderGranted(address indexed account);

    /// @notice Emitted when the Liquidity Provider role is revoked from an address.
    event LogLiquidityProviderRevoked(address indexed account);

    /// @notice Emitted when a token is mapped to its issuing Central Bank address.
    event LogCentralBankOfTokenSet(address indexed token, address indexed centralBank);

    /// @notice Initializes the registry and assigns the primary administrator.
    /// @param admin The address granted initial governance and admin rights.
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

        // lastUpdate is written from block.timestamp on purpose. Unlike Scenario A — whose
        // IdentityRegistry is deployed INSIDE Pente privacy groups (see the payment-orchestrator
        // `paladin` adapter and the toolkit's setupBilateralFXAContext) where any block.* value on
        // an endorsed-state path diverges across endorsers and wedges the private tx — Scenario B's
        // IdentityRegistry only ever lives on the base hub/spoke ledger. FXAgreement here is
        // deployed on that same base ledger (CBWeb3Hub.s.sol: `new FXAgreement(identityRegistry)`),
        // reached via the base-ledger `besu` adapter, and privacy is provided by Zeto/Noto tokens,
        // not by putting this registry in a group. block.timestamp is therefore deterministic here
        // and the on-chain audit timestamp is preserved. If this registry is ever deployed in a
        // Pente group, switch these writes to the Scenario A convention (lastUpdate = 0).
        _participants[account] = IdentityRegistryLibrary.Participant({
            legalName: name,
            institutionId: institutionId,
            role: role,
            status: IdentityRegistryLibrary.KycStatus.Pending,
            zkPointer: zkPointer,
            certFingerprint: bytes32(0),
            lastUpdate: block.timestamp
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
        _participants[account].lastUpdate = block.timestamp;

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
        _participants[account].lastUpdate = block.timestamp;

        emit IdentityUpdated(account, oldStatus, newStatus);
    }

    /// @inheritdoc IIdentityRegistry
    /// @dev Binds an X.509 certificate to a participant's on-chain identity.
    function setCertFingerprint(address account, bytes32 fingerprint) external override onlyRole(GOVERNANCE_ROLE) {
        _participants[account].certFingerprint = fingerprint;
        _participants[account].lastUpdate = block.timestamp;

        emit CertificateRegistered(account, fingerprint);
    }

    /// @inheritdoc IIdentityRegistry
    function getCertFingerprint(address account) external view override returns (bytes32) {
        return _participants[account].certFingerprint;
    }

    // =========================================================================
    //               LIQUIDITY PROVIDER ROLE (005-cooperative-liquidity)
    // =========================================================================

    /// @notice Returns true if `account` has been granted the Liquidity Provider role.
    /// @dev Used by AutomatedMarketMaker.onlyLiquidityProvider to gate addSingleSidedLiquidity.
    function isLiquidityProvider(address account) external view returns (bool) {
        return _liquidityProviders[account];
    }

    /// @notice Grants the Liquidity Provider role to `account`.
    /// @dev Only callable by the DEFAULT_ADMIN_ROLE (deployer/owner). Off-chain consortium
    ///      approval is required before calling this function (FR-004 / spec clarification Q2).
    function grantLiquidityProvider(address account) external onlyRole(DEFAULT_ADMIN_ROLE) {
        if (account == address(0)) revert InvalidIdentityData();
        _liquidityProviders[account] = true;
        emit LogLiquidityProviderGranted(account);
    }

    /// @notice Revokes the Liquidity Provider role from `account`.
    /// @dev Only callable by the DEFAULT_ADMIN_ROLE.
    function revokeLiquidityProvider(address account) external onlyRole(DEFAULT_ADMIN_ROLE) {
        _liquidityProviders[account] = false;
        emit LogLiquidityProviderRevoked(account);
    }

    /// @notice Associates `token` with its issuing Central Bank on-chain address.
    /// @dev Must be called for each tCeBM before any PairRegistry.proposePair referencing it.
    ///      Only callable by the DEFAULT_ADMIN_ROLE.
    function setCentralBankOf(address token, address centralBank) external onlyRole(DEFAULT_ADMIN_ROLE) {
        if (token == address(0) || centralBank == address(0)) revert InvalidIdentityData();
        _centralBankOf[token] = centralBank;
        emit LogCentralBankOfTokenSet(token, centralBank);
    }

    /// @notice Returns the Central Bank address that is the authorized issuer of `token`.
    /// @dev Returns address(0) if the token has not been registered via setCentralBankOf.
    function getCentralBankOf(address token) external view returns (address) {
        return _centralBankOf[token];
    }
}
