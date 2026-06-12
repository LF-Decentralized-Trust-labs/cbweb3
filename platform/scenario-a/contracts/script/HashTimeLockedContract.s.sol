// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Script} from "forge-std/Script.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";

/// @title DeployHTLC
/// @notice This script deploys the HashTimeLockedContract (HTLC)
contract DeployHTLC is Script {
    HashTimeLockedContract public htlc;

    /// @notice Sets up the initial state for the contract or test environment.
    /// @dev This function is intended to be called before running tests or executing scripts.
    function setUp() external {}

    /// @notice Executes the main logic of the deployment script.
    /// @dev Reads `DEPLOYER_PRIVATE_KEY` from environment and broadcasts the deployment transaction.
    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address identityRegistryAddress = vm.envAddress("IDENTITY_REGISTRY_ADDRESS");
        address commitmentHashRegistryAddress = vm.envAddress("COMMITMENT_HASH_REGISTRY_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        htlc = new HashTimeLockedContract(identityRegistryAddress, address(0), commitmentHashRegistryAddress);

        vm.stopBroadcast();
    }
}
