// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Script} from "forge-std/Script.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";

/// @title DeployTCeBM
/// @notice This script deploys the TokenizedCentralBankMoney contract
contract DeployTCeBM is Script {
    TokenizedCentralBankMoney public tCeBm;

    /// @notice Sets up the initial state for the contract or test environment.
    /// @dev This function is intended to be called before running tests or executing scripts.
    function setUp() external {}

    /// @notice Executes the main logic of the script.
    /// @dev This function is intended to be called externally to run the script's operations.
    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address adminAddress = vm.envAddress("ADMIN_ADDRESS");
        address centralBankAddress = vm.envAddress("CENTRAL_BANK_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        tCeBm = new TokenizedCentralBankMoney("Tokenized BRL", "tCeBM_BRL", adminAddress, centralBankAddress);

        vm.stopBroadcast();
    }
}
