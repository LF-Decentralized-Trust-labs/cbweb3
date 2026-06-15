// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {ICurrencyRegistry} from "./interfaces/ICurrencyRegistry.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";

/// @title CurrencyRegistry
/// @notice On-chain registry of hub currencies, enabling Central Banks to register
///         and discover each other's tCeBM tokens for pair proposal (006-hub-currency-registry).
///
/// @dev Authorization model:
///        • registerCurrency — requires msg.sender == IIdentityRegistry.getCentralBankOf(tokenAddress)
///        • removeCurrency   — requires msg.sender == IIdentityRegistry.getCentralBankOf(entry.tokenAddress)
///      Read operations (getAllCurrencies, getCurrency) are permissionless.
///
///      Uniqueness model (FR-006, spec clarification Q1):
///        • symbol      is globally unique — no two entries share the same symbol
///        • tokenAddress is globally unique — no two entries share the same tokenAddress
///
///      Storage pattern mirrors PairRegistry.sol (dual-mapping + ordered array + tombstone on remove).
contract CurrencyRegistry is ICurrencyRegistry {
    // ────────────────────────────── State ────────────────────────────────

    /// @notice Immutable reference to the IdentityRegistry used for CB issuer authorization.
    IIdentityRegistry public immutable REGISTRY;

    /// @dev Primary storage: keccak256(symbol) → CurrencyEntry.
    mapping(bytes32 => CurrencyEntry) private _bySymbolKey;

    /// @dev Secondary index: tokenAddress → symbolKey; enforces unique tokenAddress constraint.
    mapping(address => bytes32) private _tokenToSymbolKey;

    /// @dev Existence guard: distinguishes an active entry from a tombstoned/absent one.
    mapping(bytes32 => bool) private _symbolExists;

    /// @dev Ordered list of symbols for getAllCurrencies() iteration.
    string[] private _symbols;

    // ─────────────────────────── Constructor ─────────────────────────────

    /// @param registry Address of the deployed IdentityRegistry.
    constructor(address registry) {
        if (registry == address(0)) revert CurrencyRegistry__ZeroAddress();
        REGISTRY = IIdentityRegistry(registry);
    }

    // ──────────────────────────── External ───────────────────────────────

    /// @inheritdoc ICurrencyRegistry
    function registerCurrency(
        string calldata symbol,
        string calldata countryName,
        address tokenAddress,
        string calldata proposerCB
    ) external {
        if (bytes(symbol).length == 0 || bytes(countryName).length == 0 || bytes(proposerCB).length == 0) {
            revert CurrencyRegistry__EmptyString();
        }
        if (tokenAddress == address(0)) revert CurrencyRegistry__ZeroAddress();

        bytes32 key = _key(symbol);

        if (_symbolExists[key]) revert CurrencyRegistry__AlreadyExists(symbol);

        bytes32 existingKey = _tokenToSymbolKey[tokenAddress];
        if (existingKey != bytes32(0) && _symbolExists[existingKey]) {
            revert CurrencyRegistry__TokenAlreadyRegistered(tokenAddress);
        }

        address expectedCB = REGISTRY.getCentralBankOf(tokenAddress);
        if (msg.sender != expectedCB) revert CurrencyRegistry__Unauthorized();

        _symbolExists[key] = true;
        _tokenToSymbolKey[tokenAddress] = key;
        _symbols.push(symbol);
        _bySymbolKey[key] = CurrencyEntry({
            symbol: symbol, countryName: countryName, tokenAddress: tokenAddress, proposerCB: proposerCB
        });

        emit CurrencyRegistered(symbol, tokenAddress, countryName, proposerCB);
    }

    /// @inheritdoc ICurrencyRegistry
    function removeCurrency(string calldata symbol) external {
        bytes32 key = _key(symbol);
        if (!_symbolExists[key]) revert CurrencyRegistry__NotFound(symbol);

        CurrencyEntry storage entry = _bySymbolKey[key];

        address expectedCB = REGISTRY.getCentralBankOf(entry.tokenAddress);
        if (msg.sender != expectedCB) revert CurrencyRegistry__Unauthorized();

        address token = entry.tokenAddress;

        // Tombstone: mark as deleted without shifting the _symbols array.
        _symbolExists[key] = false;
        delete _tokenToSymbolKey[token];

        emit CurrencyRemoved(symbol, token);
    }

    /// @inheritdoc ICurrencyRegistry
    function getCurrency(string calldata symbol) external view returns (CurrencyEntry memory) {
        bytes32 key = _key(symbol);
        if (!_symbolExists[key]) revert CurrencyRegistry__NotFound(symbol);
        return _bySymbolKey[key];
    }

    /// @inheritdoc ICurrencyRegistry
    function getAllCurrencies() external view returns (CurrencyEntry[] memory) {
        // Count active (non-tombstoned) entries.
        uint256 count;
        for (uint256 i; i < _symbols.length; ++i) {
            if (_symbolExists[_key(_symbols[i])]) {
                ++count;
            }
        }

        CurrencyEntry[] memory result = new CurrencyEntry[](count);
        uint256 idx;
        for (uint256 i; i < _symbols.length; ++i) {
            bytes32 key = _key(_symbols[i]);
            if (_symbolExists[key]) {
                result[idx++] = _bySymbolKey[key];
            }
        }
        return result;
    }

    // ──────────────────────────── Internal ───────────────────────────────

    /// @dev Derives a stable storage key from a currency symbol string.
    function _key(string memory symbol) internal pure returns (bytes32) {
        return keccak256(bytes(symbol));
    }
}
