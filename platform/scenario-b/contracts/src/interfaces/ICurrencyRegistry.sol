// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

/// @title ICurrencyRegistry
/// @notice Interface for the on-chain Currency Registry of the international hub.
/// @dev Each Central Bank registers its currency (symbol, country, tCeBM token address, CB identifier)
///      so that other CBs can discover it for pair proposals without out-of-band coordination.
///      Authorization: caller must be IIdentityRegistry.getCentralBankOf(tokenAddress).
interface ICurrencyRegistry {
    // ─────────────────────────────── Types ───────────────────────────────

    /// @notice Full metadata for a registered hub currency.
    struct CurrencyEntry {
        string symbol; // e.g. "BRL"
        string countryName; // e.g. "Brazil"
        address tokenAddress; // on-chain tCeBM address — unique key 2
        string proposerCB; // human-readable CB identifier, e.g. "central_bank_a"
    }

    // ─────────────────────────────── Events ──────────────────────────────

    /// @notice Emitted when a Central Bank registers its currency.
    /// @param symbol       The currency symbol (indexed for efficient filtering).
    /// @param tokenAddress The tCeBM token address (indexed).
    /// @param countryName  Country name.
    /// @param proposerCB   Human-readable CB identifier.
    event CurrencyRegistered(
        string indexed symbol, address indexed tokenAddress, string countryName, string proposerCB
    );

    /// @notice Emitted when a Central Bank removes its currency from the registry.
    /// @param symbol       The currency symbol that was removed.
    /// @param tokenAddress The tCeBM token address that was freed.
    event CurrencyRemoved(string indexed symbol, address indexed tokenAddress);

    // ─────────────────────────────── Errors ──────────────────────────────

    /// @notice Thrown when registerCurrency is called with a symbol already registered.
    error CurrencyRegistry__AlreadyExists(string symbol);

    /// @notice Thrown when registerCurrency is called with a tokenAddress already registered under any symbol.
    error CurrencyRegistry__TokenAlreadyRegistered(address token);

    /// @notice Thrown when removeCurrency or getCurrency is called for an unknown symbol.
    error CurrencyRegistry__NotFound(string symbol);

    /// @notice Thrown when the caller is not the authorized issuer of the token.
    error CurrencyRegistry__Unauthorized();

    /// @notice Thrown when tokenAddress is address(0).
    error CurrencyRegistry__ZeroAddress();

    /// @notice Thrown when symbol, countryName, or proposerCB is an empty string.
    error CurrencyRegistry__EmptyString();

    // ──────────────────────────── Functions ──────────────────────────────

    /// @notice Registers the caller's currency on the hub.
    /// @dev Caller must be IIdentityRegistry.getCentralBankOf(tokenAddress).
    ///      Both symbol and tokenAddress must be globally unique.
    /// @param symbol       ISO-style currency symbol (e.g. "BRL"). Unique key.
    /// @param countryName  Country name (e.g. "Brazil").
    /// @param tokenAddress On-chain tCeBM address. Unique key. Non-zero.
    /// @param proposerCB   Human-readable CB identifier (e.g. "central_bank_a"). Free-form label.
    function registerCurrency(
        string calldata symbol,
        string calldata countryName,
        address tokenAddress,
        string calldata proposerCB
    ) external;

    /// @notice Removes the caller's currency from the registry, identified by symbol.
    /// @dev Caller must be IIdentityRegistry.getCentralBankOf(entry.tokenAddress).
    ///      After removal, both symbol and tokenAddress can be re-registered.
    /// @param symbol The symbol of the currency to remove (e.g. "BRL").
    function removeCurrency(string calldata symbol) external;

    /// @notice Returns the full entry for a registered currency.
    /// @dev Reverts with CurrencyRegistry__NotFound if the symbol is not registered.
    /// @param symbol The currency symbol to look up.
    function getCurrency(string calldata symbol) external view returns (CurrencyEntry memory);

    /// @notice Returns all currently registered currencies.
    /// @dev Read-only, permissionless. Tombstoned (removed) entries are excluded.
    function getAllCurrencies() external view returns (CurrencyEntry[] memory);
}
