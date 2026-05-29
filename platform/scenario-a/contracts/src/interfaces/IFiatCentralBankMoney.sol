// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

/// @title IFiatCentralBankMoney
/// @dev Interface for the FiatCentralBankMoney (fCeBM) ERC-20 contract.
/// @dev The fCeBM represents fiat central bank money on the Besu ledger. It is minted
///      by the Central Bank upon deposit approval and burned during the escrow flow
///      (fCeBM → tCeBM swap). Only the CENTRAL_BANK_ROLE may invoke mint and burn.
interface IFiatCentralBankMoney {
    /// @notice Emitted when the Central Bank mints fCeBM tokens to an address.
    /// @param to The recipient address.
    /// @param amount The amount of tokens minted.
    event FiatMinted(address indexed to, uint256 amount);

    /// @notice Emitted when the Central Bank burns fCeBM tokens from an address.
    /// @param from The address whose tokens are burned.
    /// @param amount The amount of tokens burned.
    event FiatBurned(address indexed from, uint256 amount);

    /// @notice Mints new fCeBM tokens to a specified address.
    /// @dev Caller must hold the CENTRAL_BANK_ROLE.
    /// @param to The address to receive the newly minted tokens.
    /// @param amount The amount of tokens to mint.
    function mint(address to, uint256 amount) external;

    /// @notice Burns fCeBM tokens from a specified address.
    /// @dev Caller must hold the CENTRAL_BANK_ROLE. The Central Bank, as the sovereign
    ///      monetary authority, may burn from any address during escrow settlement.
    /// @param from The address from which tokens will be burned.
    /// @param amount The amount of tokens to burn.
    function burn(address from, uint256 amount) external;
}
