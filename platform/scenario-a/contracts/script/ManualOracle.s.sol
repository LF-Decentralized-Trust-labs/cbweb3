// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Script} from "forge-std/Script.sol";
import {ManualOracle} from "../src/ManualOracle.sol";

/// @title DeployManualOracle
/// @notice Standalone deployment script for the ManualOracle contract.
contract DeployManualOracle is Script {
    ManualOracle public oracle;

    function setUp() external {}

    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address adminAddress = vm.envAddress("ADMIN_ADDRESS");
        address centralBankAddress = vm.envAddress("CENTRAL_BANK_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        oracle = new ManualOracle(adminAddress, centralBankAddress);

        vm.stopBroadcast();
    }
}
