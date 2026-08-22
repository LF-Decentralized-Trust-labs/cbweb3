// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {FiatCentralBankMoney} from "../src/FiatCentralBankMoney.sol";

/// @title DeployFiatCBM
/// @notice Standalone deployment script for the FiatCentralBankMoney (fCeBM) contract.
/// @dev Deploys a single fCeBM ERC-20 token to the target Besu spoke chain.
///      The fCeBM serves as the on-chain fiat representation used in the escrow flow
///      (deposit → fiat-exchange → escrow → redeem).
///
///      Required environment variables:
///        DEPLOYER_PRIVATE_KEY   — private key used to sign the deployment transaction
///        ADMIN_ADDRESS          — address granted DEFAULT_ADMIN_ROLE (Network Governor)
///        CENTRAL_BANK_ADDRESS   — address granted CENTRAL_BANK_ROLE (Monetary Authority)
///        FIAT_TOKEN_NAME        — human-readable token name (e.g., "Fiat BRL")
///        FIAT_TOKEN_SYMBOL      — token symbol (e.g., "fCeBM_BRL")
contract DeployFiatCBM is Script {
    /// @notice The deployed FiatCentralBankMoney instance.
    FiatCentralBankMoney public fiatToken;

    /// @notice Sets up the initial state for the deployment script.
    function setUp() external {}

    /// @notice Executes the deployment of the FiatCentralBankMoney contract.
    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address adminAddress = vm.envAddress("ADMIN_ADDRESS");
        address centralBankAddress = vm.envAddress("CENTRAL_BANK_ADDRESS");
        string memory tokenName = vm.envString("FIAT_TOKEN_NAME");
        string memory tokenSymbol = vm.envString("FIAT_TOKEN_SYMBOL");

        vm.startBroadcast(deployerPrivateKey);

        fiatToken = new FiatCentralBankMoney(tokenName, tokenSymbol, adminAddress, centralBankAddress);

        vm.stopBroadcast();

        console.log("\n===============================================");
        console.log("    FIAT CENTRAL BANK MONEY DEPLOYED           ");
        console.log("===============================================");
        console.log("Fiat Token (%s): ", tokenSymbol, address(fiatToken));
        console.log("Admin:            ", adminAddress);
        console.log("Central Bank:     ", centralBankAddress);
        console.log("===============================================\n");
    }
}
