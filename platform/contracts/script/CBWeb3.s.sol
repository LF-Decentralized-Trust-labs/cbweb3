// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";

/// @title DeployCBWeb3
/// @author CBWeb3 Team
/// @notice Master orchestration script to deploy the entire CBWeb3 core platform.
/// @dev Deploys Identity Registry, tCeBM tokens, HTLC (Scenario A), and AMM (Scenario B).
contract DeployCBWeb3 is Script {
    /// @notice Central Identity and Compliance Registry
    IdentityRegistry public identityRegistry;

    /// @notice Tokenised Central Bank Money representing Brazilian Real
    TokenizedCentralBankMoney public tokenBrl;

    /// @notice Tokenised Central Bank Money representing Euro
    TokenizedCentralBankMoney public tokenEur;

    /// @notice Hash Time Locked Contract for atomic cross-border settlements
    HashTimeLockedContract public htlc;

    /// @notice Automated Market Maker for liquidity pool operations
    AutomatedMarketMaker public amm;

    /// @notice Sets up the initial state for the deployment script.
    function setUp() public {}

    /// @notice Executes the deployment of all core CBWeb3 contracts.
    /// @dev Deployment Order:
    ///      1. Identity & Governance (Registry)
    ///      2. Base Assets (tCeBM_BRL and tCeBM_EUR)
    ///      3. Settlement Logic (HTLC and AMM)
    function run() public {
        /// @dev Load global environment variables
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address adminAddress = vm.envAddress("ADMIN_ADDRESS");
        address centralBankAddress = vm.envAddress("CENTRAL_BANK_ADDRESS");
        address governanceAddress = vm.envAddress("GOVERNANCE_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        /// @dev 1: Deploy Identity Registry (The Foundation)
        /// @notice This registry manages KYC/AML for all subsequent participants.
        identityRegistry = new IdentityRegistry(adminAddress);

        /// @dev 2: Deploy Base Assets (Tokenised Central Bank Money)
        tokenBrl = new TokenizedCentralBankMoney("Tokenized BRL", "tCeBM_BRL", adminAddress, centralBankAddress);
        tokenEur = new TokenizedCentralBankMoney("Tokenized EUR", "tCeBM_EUR", adminAddress, centralBankAddress);

        /// @dev 3: Deploy HTLC (Scenario A - Correspondent Banking)
        htlc = new HashTimeLockedContract();

        /// @dev 4: Deploy AMM (Scenario B - Liquidity Pool)
        amm = new AutomatedMarketMaker(address(tokenBrl), address(tokenEur), adminAddress, governanceAddress);

        vm.stopBroadcast();

        // OUTPUT LOGS (Required for Backend & API Gateway configuration)
        console.log("\n===============================================");
        console.log("      CBWEB3 PLATFORM SUCCESSFULLY DEPLOYED    ");
        console.log("===============================================");
        console.log("Identity Registry: ", address(identityRegistry));
        console.log("tCeBM_BRL Address: ", address(tokenBrl));
        console.log("tCeBM_EUR Address: ", address(tokenEur));
        console.log("HTLC Address:      ", address(htlc));
        console.log("AMM Address:       ", address(amm));
        console.log("===============================================\n");
    }
}
