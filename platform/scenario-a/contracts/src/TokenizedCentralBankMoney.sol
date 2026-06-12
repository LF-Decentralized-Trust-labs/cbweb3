// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {ERC20} from "@openzeppelin-contracts/token/ERC20/ERC20.sol";
import {IERC20} from "@openzeppelin-contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin-contracts/token/ERC20/utils/SafeERC20.sol";
import {AccessControl} from "@openzeppelin-contracts/access/AccessControl.sol";

/// @title TokenizedCentralBankMoney (tCeBm)
/// @dev Core asset mocked implementation for the CBWeb3 MVP
/// @notice This contract acts as the base fiat-pegged asset for the HTLC and AMM pools. It includes RBAC to restrict minting and burning capabilities strictly to the designated Central Bank Authority.
contract TokenizedCentralBankMoney is ERC20, AccessControl {
    /// @dev Utilises the SafeERC20 library for all IERC20 token operations within this contract, ensuring safer interactions with ERC20 tokens by handling potential failures and return values according to the ERC20 standard.
    using SafeERC20 for IERC20;

    /// @notice Role identifier for the Central Bank, which is allowed to mint and burn tokens.
    bytes32 public constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE");

    /// @notice Constructor to initialize the tCeBm token and establish governance.
    /// @param name_ The name of the token (e.g., "Tokenized BRL").
    /// @param symbol_ The symbol of the token (e.g., "tCeBM_BRL").
    /// @param admin The address to be granted the DEFAULT_ADMIN_ROLE (Network Governor).
    /// @param centralBank The address to be granted the CENTRAL_BANK_ROLE (Monetary Authority).
    constructor(string memory name_, string memory symbol_, address admin, address centralBank) ERC20(name_, symbol_) {
        /// @dev Let's grant the capabilities to add/remove other roles
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        /// @dev Let's grant the capabilities to mint and burn tokens
        _grantRole(CENTRAL_BANK_ROLE, centralBank);
    }

    /// @notice Mints new tokens to a specified address.
    /// @dev Reverts if the caller does not have the CENTRAL_BANK_ROLE.
    /// @param to The address to receive the newly minted tokens.
    /// @param amount The amount of tokens to mint.
    function mint(address to, uint256 amount) external onlyRole(CENTRAL_BANK_ROLE) {
        _mint(to, amount);
    }

    /// @notice Burns tokens from a specified address.
    /// @dev Reverts if the caller does not have the CENTRAL_BANK_ROLE.
    /// @dev Note: As the supreme authority on the domestic ledger, the CB can burn from any address.
    /// @param from The address from which tokens will be burned.
    /// @param amount The amount of tokens to burn.
    function burn(address from, uint256 amount) external onlyRole(CENTRAL_BANK_ROLE) {
        _burn(from, amount);
    }
}
