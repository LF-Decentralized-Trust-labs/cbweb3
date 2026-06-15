// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {ISpokeBridge} from "./interfaces/ISpokeBridge.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";
import {IERC20} from "@openzeppelin-contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin-contracts/token/ERC20/utils/SafeERC20.sol";
import {AccessControl} from "@openzeppelin-contracts/access/AccessControl.sol";
import {ReentrancyGuard} from "@openzeppelin-contracts/utils/ReentrancyGuard.sol";

/// @title SpokeBridge
/// @notice Lock-and-Mint bridge on the spoke side (REQ-CAP-005, Scenario B).
/// @dev Verified participants lock domestic tCeBM tokens. A governance relayer calls `release`
///      to return tokens when the Hub-side wrapped token is burned.
///      Idempotency: each txId can only be used once (NFR-RES-003).
contract SpokeBridge is ISpokeBridge, AccessControl, ReentrancyGuard {
    using SafeERC20 for IERC20;

    /// @notice Role identifier for the governance relayer that can release locked assets.
    bytes32 public constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");

    /// @notice The Identity Registry used for participant clearance gates.
    IIdentityRegistry public immutable IDENTITY_REGISTRY;

    /// @dev Internal struct to store lock state.
    struct LockRecord {
        address sender;
        address token;
        uint256 amount;
        bool released;
    }

    /// @dev Stores lock records indexed by txId.
    mapping(bytes32 => LockRecord) private _locks;

    /// @dev Tracks whether a txId has been used (idempotency).
    mapping(bytes32 => bool) private _txExists;

    /// @notice Initializes the bridge with governance and identity verification.
    /// @param _identityRegistry Address of the spoke's IdentityRegistry.
    /// @param _admin Address that receives DEFAULT_ADMIN_ROLE (can grant GOVERNANCE_ROLE).
    constructor(address _identityRegistry, address _admin) {
        if (_identityRegistry == address(0) || _admin == address(0)) {
            revert SB__InvalidParameters();
        }
        IDENTITY_REGISTRY = IIdentityRegistry(_identityRegistry);
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(GOVERNANCE_ROLE, _admin);
    }

    /// @notice Ensures the given account is a verified participant in the IdentityRegistry.
    modifier onlyVerified(address account) {
        _onlyVerified(account);
        _;
    }

    /// @dev Internal check extracted from the modifier to reduce bytecode duplication at call sites.
    function _onlyVerified(address account) internal view {
        if (!IDENTITY_REGISTRY.canTransact(account)) {
            revert SB__ParticipantNotVerified(account);
        }
    }

    /// @inheritdoc ISpokeBridge
    /// @dev Caller must be verified. Reverts if the txId has already been used (idempotency).
    function lock(address token, uint256 amount, bytes32 txId) external nonReentrant onlyVerified(msg.sender) {
        if (token == address(0) || amount == 0) revert SB__InvalidParameters();
        if (_txExists[txId]) revert SB__TxAlreadyProcessed();

        _txExists[txId] = true;
        _locks[txId] = LockRecord({sender: msg.sender, token: token, amount: amount, released: false});

        IERC20(token).safeTransferFrom(msg.sender, address(this), amount);

        emit AssetLocked(txId, msg.sender, token, amount, block.timestamp);
    }

    /// @inheritdoc ISpokeBridge
    /// @dev Only callable by GOVERNANCE_ROLE. Reverts if the txId was not locked or already released.
    function release(bytes32 txId) external nonReentrant onlyRole(GOVERNANCE_ROLE) {
        if (!_txExists[txId]) revert SB__TxNotFound();
        LockRecord storage record = _locks[txId];
        if (record.released) revert SB__TxAlreadyProcessed();

        record.released = true;

        IERC20(record.token).safeTransfer(record.sender, record.amount);

        emit AssetReleased(txId, record.sender, record.token, record.amount);
    }

    /// @inheritdoc ISpokeBridge
    /// @dev Reverts with SB__TxNotFound when the txId does not exist.
    function getLock(bytes32 txId)
        external
        view
        returns (address sender, address token, uint256 amount, bool released)
    {
        if (!_txExists[txId]) {
            revert SB__TxNotFound();
        }
        LockRecord storage record = _locks[txId];
        return (record.sender, record.token, record.amount, record.released);
    }
}
