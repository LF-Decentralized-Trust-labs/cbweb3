// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Script} from "forge-std/Script.sol";
import {FXAgreement} from "../src/FXAgreement.sol";

/// @title DeployFXAgreement
/// @notice Standalone deployment script for the FXAgreement contract.
contract DeployFXAgreement is Script {
    FXAgreement public fxAgreement;

    function setUp() external {}

    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address identityRegistryAddress = vm.envAddress("IDENTITY_REGISTRY_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        fxAgreement = new FXAgreement(identityRegistryAddress);

        vm.stopBroadcast();
    }
}
