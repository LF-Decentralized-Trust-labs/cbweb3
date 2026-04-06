// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

/// @title FiatCentralBankMoneyLibrary
/// @dev Core data structures and custom errors for the FiatCentralBankMoney (fCeBM) contract.
/// @dev The fCeBM is an ERC-20 representation of fiat central bank money on the Besu ledger.
///      It serves as the on-chain fiat asset that commercial banks receive upon deposit
///      approval and that can later be exchanged for TokenizedCentralBankMoney (tCeBM)
///      via the escrow flow (burn fCeBM on Besu → mint tCeBM on Paladin/Zeto).
library FiatCentralBankMoneyLibrary {
    /// @dev Emitted when the Central Bank mints fCeBM tokens to a commercial bank.
    /// @param to The address receiving the minted tokens.
    /// @param amount The amount of tokens minted.
    event FiatMinted(address indexed to, uint256 amount);

    /// @dev Emitted when the Central Bank burns fCeBM tokens from an address.
    /// @param from The address from which tokens are burned.
    /// @param amount The amount of tokens burned.
    event FiatBurned(address indexed from, uint256 amount);
}
