// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

/// @title ILiquidityCommitRegistry
/// @notice On-chain coordination contract for bilateral CB liquidity provisioning.
/// @dev Each CB gateway registers a deposit intent (commit) for a pool pair side.
///      When both sides (A and B) are PENDING for the same pool_pair, the contract
///      automatically emits CommitMatched. Each gateway's event watcher then calls
///      addSingleSidedLiquidity using the local sovereign signer — no inter-gateway
///      communication is required.
///
///      Authorization model: msg.sender must equal
///        IIdentityRegistry.getCentralBankOf(wTokenAddress)
///      ensuring that only the sovereign CB emitter of a given token can register
///      a commit for that token's side.
///
///      Expiration: commits expire after 72h without a counterpart (FR-013 from
///      spec-005). Any address may call expireCommit() to trigger garbage collection.
///
///      Scalability: supports N CBs without protocol change — each new CB deploys
///      its own W-tCeBM, registers it in IdentityRegistry, and can immediately
///      register commits for any active pool pair.
///
///      Feature: 007-bridge-based-cb-liquidity
interface ILiquidityCommitRegistry {

    // -------------------------------------------------------------------------
    // Enums
    // -------------------------------------------------------------------------

    enum CommitStatus { PENDING, MATCHED, EXPIRED, CANCELLED }
    enum CommitSide { A, B }

    // -------------------------------------------------------------------------
    // Events
    // -------------------------------------------------------------------------

    /// @notice Emitted when a CB registers a deposit intent.
    /// @param commitId   Unique identifier for this commit (bytes32 hash).
    /// @param poolPair   Pool pair identifier (e.g., "W-BRL-ARS").
    /// @param side       CommitSide.A or CommitSide.B.
    /// @param signer     msg.sender — sovereign CB signer address.
    /// @param wTokenAddr Address of the W-tCeBM contract on the Hub.
    /// @param amount     Deposit amount in wei.
    /// @param expiresAt  block.timestamp when this commit expires (now + 72h).
    event CommitRegistered(
        bytes32 indexed commitId,
        string  poolPair,
        CommitSide side,
        address indexed signer,
        address wTokenAddr,
        uint256 amount,
        uint256 expiresAt
    );

    /// @notice Emitted when both sides (A and B) of a pool pair are PENDING —
    ///         triggers autonomous execution by each gateway.
    /// @param poolPair   Pool pair identifier.
    /// @param commitIdA  Commit ID for side A.
    /// @param signerA    Sovereign signer of CB-A.
    /// @param amountA    Deposit amount for side A in wei.
    /// @param commitIdB  Commit ID for side B.
    /// @param signerB    Sovereign signer of CB-B.
    /// @param amountB    Deposit amount for side B in wei.
    event CommitMatched(
        string  indexed poolPair,
        bytes32 commitIdA,
        address signerA,
        uint256 amountA,
        bytes32 commitIdB,
        address signerB,
        uint256 amountB
    );

    /// @notice Emitted when a commit expires without a counterpart.
    /// @param commitId  The expired commit ID.
    /// @param poolPair  Pool pair identifier.
    /// @param side      Which side expired.
    event CommitExpired(bytes32 indexed commitId, string poolPair, CommitSide side);

    /// @notice Emitted when a CB cancels its own pending commit.
    /// @param commitId  The cancelled commit ID.
    event CommitCancelled(bytes32 indexed commitId);

    // -------------------------------------------------------------------------
    // Errors
    // -------------------------------------------------------------------------

    /// @dev Caller is not the sovereign CB of the given wTokenAddress.
    error LCR__NotTokenCentralBank(address caller, address wTokenAddress);

    /// @dev A PENDING commit already exists for this (poolPair, side).
    error LCR__CommitAlreadyPending(string poolPair, CommitSide side);

    /// @dev The commit does not exist.
    error LCR__CommitNotFound(bytes32 commitId);

    /// @dev The commit is not in PENDING status.
    error LCR__CommitNotPending(bytes32 commitId, CommitStatus currentStatus);

    /// @dev Only the original signer can cancel.
    error LCR__NotCommitOwner(bytes32 commitId, address caller);

    /// @dev The commit has not yet expired.
    error LCR__CommitNotExpired(bytes32 commitId, uint256 expiresAt);

    /// @dev Invalid input (zero amount, empty poolPair, zero address).
    error LCR__InvalidParameters();

    // -------------------------------------------------------------------------
    // Functions
    // -------------------------------------------------------------------------

    /// @notice Registers a deposit intent for one side of a pool pair.
    /// @dev Caller (msg.sender) must equal IdentityRegistry.getCentralBankOf(wTokenAddress).
    ///      Reverts if a PENDING commit already exists for (poolPair, side).
    ///      If the counterpart side is already PENDING, both commits transition to
    ///      MATCHED and CommitMatched is emitted atomically.
    /// @param poolPair      Pool pair identifier string (e.g., "W-BRL-ARS").
    /// @param side          CommitSide.A or CommitSide.B.
    /// @param amount        Deposit amount in wei (> 0).
    /// @param wTokenAddress Address of the W-tCeBM contract on the Hub.
    /// @return commitId     Unique ID assigned to this commit.
    function registerCommit(
        string calldata poolPair,
        CommitSide side,
        uint256 amount,
        address wTokenAddress
    ) external returns (bytes32 commitId);

    /// @notice Cancels a PENDING commit owned by msg.sender.
    /// @dev Only callable by the original signer. Emits CommitCancelled.
    ///      Reverts if commit is not in PENDING status.
    /// @param commitId  The commit ID to cancel.
    function cancelCommit(bytes32 commitId) external;

    /// @notice Expires a commit past its expiresAt timestamp.
    /// @dev Callable by any address (permissionless garbage collection).
    ///      Reverts if expiresAt has not been reached.
    ///      Emits CommitExpired.
    /// @param commitId  The commit ID to expire.
    function expireCommit(bytes32 commitId) external;

    // -------------------------------------------------------------------------
    // Views
    // -------------------------------------------------------------------------

    /// @notice Returns commit details for a given commitId.
    /// @param commitId  The commit ID to query.
    /// @return signer       The CB signer address that registered the commit.
    /// @return wTokenAddr   W-tCeBM address.
    /// @return amount       Deposit amount in wei.
    /// @return expiresAt    Expiry timestamp.
    /// @return status       Current CommitStatus.
    /// @return side         CommitSide.
    function getCommit(bytes32 commitId)
        external
        view
        returns (
            address signer,
            address wTokenAddr,
            uint256 amount,
            uint256 expiresAt,
            CommitStatus status,
            CommitSide side
        );

    /// @notice Returns the commit ID currently PENDING for a given (poolPair, side).
    ///         Returns bytes32(0) if none.
    /// @param poolPair  Pool pair identifier.
    /// @param side      CommitSide.A or CommitSide.B.
    /// @return commitId  The pending commit ID, or bytes32(0).
    function getPendingCommit(string calldata poolPair, CommitSide side)
        external
        view
        returns (bytes32 commitId);
}
