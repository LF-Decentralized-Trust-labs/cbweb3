// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";

/// @title PairRegistry
/// @notice Manages the lifecycle of AMM token pairs with bilateral Central Bank approval.
/// @dev A pair transitions from PROPOSED → ACTIVE only when both the tokenA issuer
///      (proposer) and the tokenB issuer (confirmer) have approved it on-chain.
///      Each active pair maps to a dedicated AutomatedMarketMaker contract, enabling
///      multi-par support without modifying the AMM itself (D9, 005-cooperative-liquidity).
///
///      Authorization model:
///        • proposePair  — requires msg.sender == IIdentityRegistry.getCentralBankOf(tokenA)
///        • confirmPair  — requires msg.sender == IIdentityRegistry.getCentralBankOf(tokenB)
///
///      Sovereignty constraint: each Central Bank can only propose pairs for tokens it issued.
contract PairRegistry {
    // ───────────────────────────── Types ─────────────────────────────

    /// @notice Lifecycle status of a currency pair.
    enum PairStatus {
        PROPOSED, // tokenA CB proposed; awaiting tokenB CB confirmation
        ACTIVE // both CBs approved; AMM is live
    }

    /// @notice Full metadata for a registered currency pair.
    struct PairEntry {
        string pairId; // human-readable, e.g. "BRL-USD"
        address ammAddress; // dedicated AutomatedMarketMaker for this pair
        address tokenA; // tCeBM address for currency A
        address tokenB; // tCeBM address for currency B
        PairStatus status;
        address proposer; // CB that issued tokenA and proposed the pair
        address confirmer; // CB that issued tokenB and confirmed the pair
    }

    // ───────────────────────────── State ─────────────────────────────

    /// @notice Immutable reference to the IdentityRegistry used for CB authorization.
    IIdentityRegistry public immutable REGISTRY;

    /// @dev Primary storage: keccak256(pairId) → PairEntry.
    mapping(bytes32 => PairEntry) private _pairs;

    /// @dev Ordered list of pairIds for iteration (getAllActivePairs).
    string[] private _pairIds;

    /// @dev Existence guard to distinguish default-zero struct from a real entry.
    mapping(bytes32 => bool) private _pairExists;

    // ───────────────────────────── Events ────────────────────────────

    /// @notice Emitted when a Central Bank proposes a new pair (status = PROPOSED).
    /// @param pairId   Human-readable pair identifier.
    /// @param proposer On-chain address of the Central Bank that issued tokenA.
    /// @param tokenA   tCeBM address of the first token.
    /// @param tokenB   tCeBM address of the second token.
    event PairProposed(string indexed pairId, address indexed proposer, address tokenA, address tokenB);

    /// @notice Emitted when a pair becomes ACTIVE (both CBs approved).
    /// @param pairId     Human-readable pair identifier.
    /// @param ammAddress Address of the live AutomatedMarketMaker.
    /// @param tokenA     tCeBM address of the first token.
    /// @param tokenB     tCeBM address of the second token.
    event PairRegistered(string indexed pairId, address indexed ammAddress, address tokenA, address tokenB);

    // ───────────────────────────── Errors ────────────────────────────

    /// @notice Thrown when proposePair is called with an already-registered pairId.
    error PairRegistry__AlreadyExists(string pairId);

    /// @notice Thrown when confirmPair / getPair is called for an unknown pairId.
    error PairRegistry__NotFound(string pairId);

    /// @notice Thrown when confirmPair is called on an already-ACTIVE pair.
    error PairRegistry__AlreadyActive(string pairId);

    /// @notice Thrown when msg.sender is not the authorized Central Bank for the token.
    error PairRegistry__Unauthorized();

    /// @notice Thrown when address(0) is passed where a non-zero address is required.
    error PairRegistry__ZeroAddress();

    // ──────────────────────────── Constructor ─────────────────────────

    /// @param registry Address of the deployed IdentityRegistry.
    constructor(address registry) {
        if (registry == address(0)) revert PairRegistry__ZeroAddress();
        REGISTRY = IIdentityRegistry(registry);
    }

    // ──────────────────────────── External ───────────────────────────

    /// @notice Proposes a new currency pair. Caller must be the Central Bank issuer of `tokenA`.
    /// @dev The pair enters PROPOSED status. The Central Bank of `tokenB` must call
    ///      confirmPair to activate the pair and make the AMM live.
    /// @param pairId      Human-readable pair identifier (e.g. "BRL-ARS"). Must be unique.
    /// @param tokenA      Address of the tCeBM contract for the first token (caller's currency).
    /// @param tokenB      Address of the tCeBM contract for the second token.
    /// @param ammAddress  Address of a pre-deployed AutomatedMarketMaker for this pair.
    function proposePair(string calldata pairId, address tokenA, address tokenB, address ammAddress) external {
        if (tokenA == address(0) || tokenB == address(0) || ammAddress == address(0)) {
            revert PairRegistry__ZeroAddress();
        }

        bytes32 key = _key(pairId);
        if (_pairExists[key]) revert PairRegistry__AlreadyExists(pairId);

        address expectedProposer = REGISTRY.getCentralBankOf(tokenA);
        if (msg.sender != expectedProposer) revert PairRegistry__Unauthorized();

        _pairExists[key] = true;
        _pairIds.push(pairId);
        _pairs[key] = PairEntry({
            pairId: pairId,
            ammAddress: ammAddress,
            tokenA: tokenA,
            tokenB: tokenB,
            status: PairStatus.PROPOSED,
            proposer: msg.sender,
            confirmer: address(0)
        });

        emit PairProposed(pairId, msg.sender, tokenA, tokenB);
    }

    /// @notice Confirms a proposed pair. Caller must be the Central Bank issuer of `tokenB`.
    /// @dev Transitions the pair from PROPOSED → ACTIVE and emits PairRegistered.
    ///      The Go PairRouter subscribes to PairRegistered to update its routing cache (D10).
    /// @param pairId  The pair identifier previously passed to proposePair.
    function confirmPair(string calldata pairId) external {
        bytes32 key = _key(pairId);
        if (!_pairExists[key]) revert PairRegistry__NotFound(pairId);

        PairEntry storage entry = _pairs[key];
        if (entry.status == PairStatus.ACTIVE) revert PairRegistry__AlreadyActive(pairId);

        address expectedConfirmer = REGISTRY.getCentralBankOf(entry.tokenB);
        if (msg.sender != expectedConfirmer) revert PairRegistry__Unauthorized();

        entry.status = PairStatus.ACTIVE;
        entry.confirmer = msg.sender;

        emit PairRegistered(pairId, entry.ammAddress, entry.tokenA, entry.tokenB);
    }

    /// @notice Returns all pairs that are currently in ACTIVE status.
    /// @dev Used by the Go PairRouter at startup to initialise its routing cache (D10).
    ///      Pairs in PROPOSED status are excluded.
    function getAllActivePairs() external view returns (PairEntry[] memory) {
        uint256 count;
        for (uint256 i; i < _pairIds.length; ++i) {
            if (_pairs[_key(_pairIds[i])].status == PairStatus.ACTIVE) {
                ++count;
            }
        }
        PairEntry[] memory result = new PairEntry[](count);
        uint256 idx;
        for (uint256 i; i < _pairIds.length; ++i) {
            PairEntry storage e = _pairs[_key(_pairIds[i])];
            if (e.status == PairStatus.ACTIVE) {
                result[idx++] = e;
            }
        }
        return result;
    }

    /// @notice Returns the full entry for a pair regardless of its status.
    /// @dev Reverts with PairRegistry__NotFound if the pairId was never proposed.
    function getPair(string calldata pairId) external view returns (PairEntry memory) {
        bytes32 key = _key(pairId);
        if (!_pairExists[key]) revert PairRegistry__NotFound(pairId);
        return _pairs[key];
    }

    // ──────────────────────────── Internal ───────────────────────────

    /// @dev Stable key derivation to map pairId strings to storage slots.
    function _key(string memory pairId) internal pure returns (bytes32) {
        return keccak256(bytes(pairId));
    }
}
