// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

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

    /// @dev Whether a symbol has ever been pushed into `_symbols`, independently of whether it is
    ///      currently active. removeCurrency tombstones the entry and deliberately leaves
    ///      `_symbols` untouched so paging offsets stay stable across a removal; without this
    ///      flag, re-registering the same symbol pushed a second copy of the string, and every
    ///      reader that walks `_symbols` filtering on `_symbolExists` then reported the single
    ///      live entry once per copy.
    mapping(bytes32 => bool) private _symbolListed;

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
        // Push only on first registration. A symbol that was registered, removed and registered
        // again is already in `_symbols`; pushing it again would duplicate it in every listing.
        if (!_symbolListed[key]) {
            _symbolListed[key] = true;
            _symbols.push(symbol);
        }
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

    /// @notice Largest page a single paged read may return.
    /// @dev getAllCurrencies() returns every active entry, which is `external view` and so costs
    ///      no gas on-chain — the cost is the response size and the RPC round trip, and it grows
    ///      with the number of registered currencies (finding R2-M-14). A caller that wants a
    ///      slice can now ask for one, and cannot ask for an unbounded one.
    uint256 public constant MAX_PAGE_SIZE = 100;

    /// @notice Default page size when a caller passes limit = 0.
    uint256 public constant DEFAULT_PAGE_SIZE = 50;

    /// @notice Number of active (non-tombstoned) currencies.
    /// @dev Lets a caller size its paging loop, and tell an empty page from an overshot offset.
    function currencyCount() external view returns (uint256 count) {
        for (uint256 i; i < _symbols.length; ++i) {
            if (_symbolExists[_key(_symbols[i])]) {
                ++count;
            }
        }
    }

    /// @notice Reads a bounded window of the active currencies.
    /// @param offset How many active entries to skip. An offset at or past the end returns an
    ///        empty page rather than reverting, so a paging loop terminates on a short read.
    /// @param limit Page size. Zero means DEFAULT_PAGE_SIZE; anything above MAX_PAGE_SIZE is
    ///        clamped to it, so the bound cannot be defeated by asking for more.
    /// @return page The window, in registration order.
    /// @return total The number of active entries, so a caller knows whether more remain.
    /// @dev COST. This bounds the RESPONSE, not the work. Both passes scan the whole _symbols
    ///      array whatever the window, because removeCurrency tombstones in place rather than
    ///      compacting — so one page is O(n), and walking the whole set in pages of `limit` is
    ///      O(n^2/limit), which is more total node work than a single getAllCurrencies() at O(n).
    ///      That is the right trade while the response is what fails first (an oversized eth_call
    ///      fails worse than several small ones) and while n is small. If the registry ever grows
    ///      enough for the scan itself to be the problem, the fix is a compacted index of active
    ///      symbols maintained on write, which would let a page seek directly instead of scanning.
    function getCurrenciesPaged(uint256 offset, uint256 limit)
        external
        view
        returns (CurrencyEntry[] memory page, uint256 total)
    {
        if (limit == 0) {
            limit = DEFAULT_PAGE_SIZE;
        } else if (limit > MAX_PAGE_SIZE) {
            limit = MAX_PAGE_SIZE;
        }

        // First pass: total actives, and how many of them fall inside the window.
        uint256 active;
        uint256 inWindow;
        for (uint256 i; i < _symbols.length; ++i) {
            if (!_symbolExists[_key(_symbols[i])]) continue;
            if (active >= offset && inWindow < limit) {
                ++inWindow;
            }
            ++active;
        }
        total = active;

        page = new CurrencyEntry[](inWindow);
        if (inWindow == 0) {
            return (page, total);
        }

        // Second pass: fill the window.
        uint256 seen;
        uint256 idx;
        for (uint256 i; i < _symbols.length && idx < inWindow; ++i) {
            bytes32 key = _key(_symbols[i]);
            if (!_symbolExists[key]) continue;
            if (seen >= offset) {
                page[idx++] = _bySymbolKey[key];
            }
            ++seen;
        }
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
