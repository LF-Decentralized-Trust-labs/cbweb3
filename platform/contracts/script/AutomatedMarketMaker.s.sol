// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Script} from "forge-std/Script.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";

/// @title DeployAMM
/// @notice This script deploys the AutomatedMarketMaker contract
contract DeployAMM is Script {
    AutomatedMarketMaker public amm;

    /// @notice Sets up the initial state for the contract or test environment.
    /// @dev This function is intended to be called before running tests or executing scripts.
    function setUp() external {}

    /// @notice Executes the main logic of the script.
    /// @dev This function is intended to be called externally to run the script's operations.
    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address tokenAAddress = vm.envAddress("TOKEN_A_ADDRESS");
        address tokenBAddress = vm.envAddress("TOKEN_B_ADDRESS");
        address adminAddress = vm.envAddress("ADMIN_ADDRESS");
        address governanceAddress = vm.envAddress("GOVERNANCE_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        amm = new AutomatedMarketMaker(tokenAAddress, tokenBAddress, adminAddress, governanceAddress);

        vm.stopBroadcast();
    }
}
