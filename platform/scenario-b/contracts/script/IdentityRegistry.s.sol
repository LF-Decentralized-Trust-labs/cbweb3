// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Script} from "forge-std/Script.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";

/// @title DeployIdentityRegistry
/// @notice This script deploys the IdentityRegistry contract for the CBWeb3 project.
/// @dev Uses Foundry's vm.broadcast to sign and send transactions to the ledger.
contract DeployIdentityRegistry is Script {
    IdentityRegistry public identityRegistry;

    /// @notice Sets up the initial state for the contract or test environment.
    function setUp() external {}

    /// @notice Executes the deployment of the IdentityRegistry.
    /// @dev Retrieves 'DEPLOYER_PRIVATE_KEY' and 'ADMIN_ADDRESS' from the environment.
    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");

        /// @dev The adminAddress will be granted DEFAULT_ADMIN_ROLE and GOVERNANCE_ROLE
        address adminAddress = vm.envAddress("ADMIN_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        identityRegistry = new IdentityRegistry(adminAddress);

        vm.stopBroadcast();
    }
}
