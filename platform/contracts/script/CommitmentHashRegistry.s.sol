// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Script} from "forge-std/Script.sol";
import {CommitmentHashRegistry} from "../src/CommitmentHashRegistry.sol";

/// @title DeployCommitmentHashRegistry
/// @notice Standalone deployment script for the CommitmentHashRegistry contract on public Besu.
contract DeployCommitmentHashRegistry is Script {
    CommitmentHashRegistry public commitmentHashRegistry;

    function setUp() external {}

    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address identityRegistryAddress = vm.envAddress("IDENTITY_REGISTRY_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        commitmentHashRegistry = new CommitmentHashRegistry(identityRegistryAddress);

        vm.stopBroadcast();
    }
}
