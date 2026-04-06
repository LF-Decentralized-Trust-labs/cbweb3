// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {ERC20} from "@openzeppelin-contracts/token/ERC20/ERC20.sol";
import {IERC20} from "@openzeppelin-contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin-contracts/token/ERC20/utils/SafeERC20.sol";
import {AccessControl} from "@openzeppelin-contracts/access/AccessControl.sol";
import {IFiatCentralBankMoney} from "./interfaces/IFiatCentralBankMoney.sol";

/// @title FiatCentralBankMoney (fCeBM)
/// @dev ERC-20 representation of fiat central bank money on the Besu ledger.
/// @notice This contract represents the on-chain fiat asset that commercial banks receive
///         upon deposit approval by the Central Bank. The fCeBM tokens can later be exchanged
///         for TokenizedCentralBankMoney (tCeBM) via the escrow flow:
///         - Escrow: Central Bank burns fCeBM from the commercial bank's Besu wallet and mints
///           the equivalent tCeBM on the Paladin/Zeto privacy layer.
///         - Redeem: Central Bank burns tCeBM on Paladin/Zeto and mints fCeBM back to the
///           commercial bank's Besu wallet.
///         Only the CENTRAL_BANK_ROLE may mint and burn tokens. The DEFAULT_ADMIN_ROLE
///         (Network Governor) manages role assignments.
contract FiatCentralBankMoney is ERC20, AccessControl, IFiatCentralBankMoney {
    /// @dev Utilises the SafeERC20 library for all IERC20 token operations within this contract,
    ///      ensuring safer interactions with ERC20 tokens by handling potential failures and
    ///      return values according to the ERC20 standard.
    using SafeERC20 for IERC20;

    /// @notice Role identifier for the Central Bank, which is allowed to mint and burn tokens.
    bytes32 public constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE");

    /// @notice Constructor to initialize the fCeBM token and establish governance.
    /// @param name_ The name of the token (e.g., "Fiat BRL").
    /// @param symbol_ The symbol of the token (e.g., "fCeBM_BRL").
    /// @param admin The address to be granted the DEFAULT_ADMIN_ROLE (Network Governor).
    /// @param centralBank The address to be granted the CENTRAL_BANK_ROLE (Monetary Authority).
    constructor(string memory name_, string memory symbol_, address admin, address centralBank) ERC20(name_, symbol_) {
        /// @dev Grant the capabilities to add/remove other roles
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        /// @dev Grant the capabilities to mint and burn tokens
        _grantRole(CENTRAL_BANK_ROLE, centralBank);
    }

    /// @inheritdoc IFiatCentralBankMoney
    function mint(address to, uint256 amount) external onlyRole(CENTRAL_BANK_ROLE) {
        _mint(to, amount);
        emit FiatMinted(to, amount);
    }

    /// @inheritdoc IFiatCentralBankMoney
    function burn(address from, uint256 amount) external onlyRole(CENTRAL_BANK_ROLE) {
        _burn(from, amount);
        emit FiatBurned(from, amount);
    }
}
