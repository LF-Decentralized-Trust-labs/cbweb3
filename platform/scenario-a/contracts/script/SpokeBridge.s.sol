// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {SpokeBridge} from "../src/SpokeBridge.sol";

/// @title DeploySpokeBridge
/// @notice Standalone deployment script for the SpokeBridge contract.
/// @dev Reads DEPLOYER_PRIVATE_KEY, ADMIN_ADDRESS, and IDENTITY_REGISTRY_ADDRESS from environment.
contract DeploySpokeBridge is Script {
    SpokeBridge public spokeBridge;

    function setUp() public {}

    function run() public {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address adminAddress = vm.envAddress("ADMIN_ADDRESS");
        address identityRegistryAddress = vm.envAddress("IDENTITY_REGISTRY_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        spokeBridge = new SpokeBridge(identityRegistryAddress, adminAddress);

        vm.stopBroadcast();

        console.log("SpokeBridge deployed at:", address(spokeBridge));
    }
}
