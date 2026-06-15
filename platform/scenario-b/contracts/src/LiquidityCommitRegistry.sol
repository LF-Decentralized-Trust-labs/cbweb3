// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {ILiquidityCommitRegistry} from "./interfaces/ILiquidityCommitRegistry.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";

/// @title LiquidityCommitRegistry
/// @notice On-chain bilateral coordination for sovereign CB liquidity provisioning.
/// @dev Implementation of ILiquidityCommitRegistry. Each CB gateway registers a
///      single-sided deposit intent (commit) for a given pool pair. When both sides
///      (A and B) become PENDING for the same pool_pair, the contract transitions both
///      commits to MATCHED and emits CommitMatched atomically — no inter-gateway
///      communication required.
///
///      Authorization: msg.sender must equal IdentityRegistry.getCentralBankOf(wTokenAddress).
///      Expiry: 72 hours after registration. Any address may call expireCommit() to GC.
///
///      Feature: 007-bridge-based-cb-liquidity
contract LiquidityCommitRegistry is ILiquidityCommitRegistry {

    // ─────────────────────────────────────────────────────────────────────────
    // Constants
    // ─────────────────────────────────────────────────────────────────────────

    /// @dev 72-hour commit TTL in seconds (FR-013 from spec-005).
    uint256 public constant COMMIT_TTL = 72 hours;

    // ─────────────────────────────────────────────────────────────────────────
    // State
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice IdentityRegistry used to verify CB issuer authority.
    IIdentityRegistry public immutable REGISTRY;

    /// @dev Full commit data keyed by commitId.
    mapping(bytes32 => CommitData) private _commits;

    /// @dev Index: keccak256(poolPair) × side → currently PENDING commitId.
    ///      bytes32(0) means no PENDING commit for that slot.
    mapping(bytes32 => mapping(uint8 => bytes32)) private _pendingBySlot;

    /// @dev Internal struct storing commit details.
    struct CommitData {
        address    signer;
        address    wTokenAddr;
        uint256    amount;
        uint256    expiresAt;
        CommitStatus status;
        CommitSide side;
        string     poolPair;
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Constructor
    // ─────────────────────────────────────────────────────────────────────────

    /// @param registry Address of the deployed IdentityRegistry on the Hub.
    constructor(address registry) {
        if (registry == address(0)) revert LCR__InvalidParameters();
        REGISTRY = IIdentityRegistry(registry);
    }

    // ─────────────────────────────────────────────────────────────────────────
    // External — write
    // ─────────────────────────────────────────────────────────────────────────

    /// @inheritdoc ILiquidityCommitRegistry
    function registerCommit(
        string calldata poolPair,
        CommitSide side,
        uint256 amount,
        address wTokenAddress
    ) external override returns (bytes32 commitId) {
        // Validate inputs.
        if (amount == 0 || bytes(poolPair).length == 0 || wTokenAddress == address(0)) {
            revert LCR__InvalidParameters();
        }

        // Authorization: caller must be the sovereign CB of the given W-tCeBM.
        address cbOfToken = REGISTRY.getCentralBankOf(wTokenAddress);
        if (msg.sender != cbOfToken) {
            revert LCR__NotTokenCentralBank(msg.sender, wTokenAddress);
        }

        // Check no PENDING commit already exists for (poolPair, side).
        bytes32 pairKey = keccak256(bytes(poolPair));
        bytes32 existingId = _pendingBySlot[pairKey][uint8(side)];
        if (existingId != bytes32(0)) {
            revert LCR__CommitAlreadyPending(poolPair, side);
        }

        // Derive a deterministic commitId from the commit parameters + block context.
        commitId = keccak256(abi.encode(poolPair, side, msg.sender, amount, wTokenAddress, block.timestamp));

        uint256 exp = block.timestamp + COMMIT_TTL;
        _commits[commitId] = CommitData({
            signer:    msg.sender,
            wTokenAddr: wTokenAddress,
            amount:    amount,
            expiresAt: exp,
            status:    CommitStatus.PENDING,
            side:      side,
            poolPair:  poolPair
        });

        _pendingBySlot[pairKey][uint8(side)] = commitId;

        emit CommitRegistered(commitId, poolPair, side, msg.sender, wTokenAddress, amount, exp);

        // Check if the counterpart side is PENDING — if so, match immediately.
        CommitSide counterSide = side == CommitSide.A ? CommitSide.B : CommitSide.A;
        bytes32 counterCommitId = _pendingBySlot[pairKey][uint8(counterSide)];
        if (counterCommitId != bytes32(0)) {
            CommitData storage counter = _commits[counterCommitId];
            if (counter.status == CommitStatus.PENDING) {
                // Transition both to MATCHED.
                _commits[commitId].status = CommitStatus.MATCHED;
                counter.status = CommitStatus.MATCHED;

                // Clear pending slots.
                _pendingBySlot[pairKey][uint8(side)]        = bytes32(0);
                _pendingBySlot[pairKey][uint8(counterSide)] = bytes32(0);

                // Determine A/B ordering for the event.
                bytes32 commitIdA;
                address signerA;
                uint256 amountA;
                bytes32 commitIdB;
                address signerB;
                uint256 amountB;

                if (side == CommitSide.A) {
                    commitIdA = commitId;      signerA = msg.sender;         amountA = amount;
                    commitIdB = counterCommitId; signerB = counter.signer;   amountB = counter.amount;
                } else {
                    commitIdA = counterCommitId; signerA = counter.signer;   amountA = counter.amount;
                    commitIdB = commitId;        signerB = msg.sender;       amountB = amount;
                }

                emit CommitMatched(poolPair, commitIdA, signerA, amountA, commitIdB, signerB, amountB);
            }
        }
    }

    /// @inheritdoc ILiquidityCommitRegistry
    function cancelCommit(bytes32 commitId) external override {
        CommitData storage c = _commits[commitId];
        if (c.signer == address(0)) revert LCR__CommitNotFound(commitId);
        if (c.status != CommitStatus.PENDING) revert LCR__CommitNotPending(commitId, c.status);
        if (c.signer != msg.sender) revert LCR__NotCommitOwner(commitId, msg.sender);

        c.status = CommitStatus.CANCELLED;

        // Clear pending slot.
        bytes32 pairKey = keccak256(bytes(c.poolPair));
        _pendingBySlot[pairKey][uint8(c.side)] = bytes32(0);

        emit CommitCancelled(commitId);
    }

    /// @inheritdoc ILiquidityCommitRegistry
    function expireCommit(bytes32 commitId) external override {
        CommitData storage c = _commits[commitId];
        if (c.signer == address(0)) revert LCR__CommitNotFound(commitId);
        if (c.status != CommitStatus.PENDING) revert LCR__CommitNotPending(commitId, c.status);
        if (block.timestamp < c.expiresAt) revert LCR__CommitNotExpired(commitId, c.expiresAt);

        string memory poolPair = c.poolPair;
        CommitSide   side      = c.side;
        c.status = CommitStatus.EXPIRED;

        // Clear pending slot.
        bytes32 pairKey = keccak256(bytes(poolPair));
        _pendingBySlot[pairKey][uint8(side)] = bytes32(0);

        emit CommitExpired(commitId, poolPair, side);
    }

    // ─────────────────────────────────────────────────────────────────────────
    // External — view
    // ─────────────────────────────────────────────────────────────────────────

    /// @inheritdoc ILiquidityCommitRegistry
    function getCommit(bytes32 commitId)
        external
        view
        override
        returns (
            address signer,
            address wTokenAddr,
            uint256 amount,
            uint256 expiresAt,
            CommitStatus status,
            CommitSide side
        )
    {
        CommitData storage c = _commits[commitId];
        if (c.signer == address(0)) revert LCR__CommitNotFound(commitId);
        return (c.signer, c.wTokenAddr, c.amount, c.expiresAt, c.status, c.side);
    }

    /// @inheritdoc ILiquidityCommitRegistry
    function getPendingCommit(string calldata poolPair, CommitSide side)
        external
        view
        override
        returns (bytes32 commitId)
    {
        return _pendingBySlot[keccak256(bytes(poolPair))][uint8(side)];
    }
}
