// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Script} from "forge-std/Script.sol";
import {console} from "forge-std/console.sol";
import {CurrencyRegistry} from "../src/CurrencyRegistry.sol";

/// @title DeployCurrencyRegistry
/// @notice Deploys CurrencyRegistry with the given IdentityRegistry address.
/// @dev Usage:
///   forge script script/DeployCurrencyRegistry.s.sol \
///     --rpc-url $HUB_RPC_URL \
///     --broadcast \
///     --sig "run(address)" $IDENTITY_REGISTRY_ADDRESS
contract DeployCurrencyRegistry is Script {
    CurrencyRegistry public currencyRegistry;

    function setUp() external {}

    /// @param identityRegistry Address of the deployed IdentityRegistry contract.
    function run(address identityRegistry) external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");

        vm.startBroadcast(deployerPrivateKey);

        currencyRegistry = new CurrencyRegistry(identityRegistry);

        vm.stopBroadcast();

        console.log("CurrencyRegistry deployed at:", address(currencyRegistry));
        console.log("  IdentityRegistry:           ", identityRegistry);
    }
}
