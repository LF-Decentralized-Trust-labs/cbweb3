// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

/// @title ISpokeBridge
/// @dev Interface for the Spoke-side Lock-and-Mint bridge (REQ-CAP-005, Scenario B).
/// @notice Locks domestic tCeBM tokens on the spoke so that a relayer can mint
///         wrapped tokens on the Hub for AMM participation. Release returns tokens
///         when the Hub-side wrapped token is burned.
interface ISpokeBridge {
    /// @notice Emitted when tokens are locked in the bridge.
    event AssetLocked(bytes32 indexed txId, address indexed sender, address token, uint256 amount, uint256 timestamp);

    /// @notice Emitted when locked tokens are released back to the original sender.
    event AssetReleased(bytes32 indexed txId, address indexed recipient, address token, uint256 amount);

    /// @dev The txId has already been processed (idempotency guard — NFR-RES-003).
    error SB__TxAlreadyProcessed();

    /// @dev The txId does not exist or was not locked.
    error SB__TxNotFound();

    /// @dev Caller is not authorised for this action.
    error SB__Unauthorized();

    /// @dev Participant not verified in the IdentityRegistry.
    error SB__ParticipantNotVerified(address account);

    /// @dev Invalid input parameters (zero amount, zero address, etc.).
    error SB__InvalidParameters();

    /// @notice Locks `amount` of `token` in the bridge under the given `txId`.
    /// @dev Caller must be verified in the IdentityRegistry. Reverts if `txId` already used.
    /// @param token The ERC20 token to lock.
    /// @param amount The amount to lock.
    /// @param txId Unique cross-chain transaction identifier.
    function lock(address token, uint256 amount, bytes32 txId) external;

    /// @notice Releases previously locked tokens back to the original depositor.
    /// @dev Only callable by an account with GOVERNANCE_ROLE. Reverts if `txId` not found.
    /// @param txId The transaction identifier of the lock to release.
    function release(bytes32 txId) external;

    /// @notice Returns the lock details for a given txId.
    /// @param txId The transaction identifier.
    /// @return sender The original depositor.
    /// @return token The locked ERC20 token.
    /// @return amount The locked amount.
    /// @return released Whether the lock has been released.
    function getLock(bytes32 txId) external view returns (address sender, address token, uint256 amount, bool released);
}
