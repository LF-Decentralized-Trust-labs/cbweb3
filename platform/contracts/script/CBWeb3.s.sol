// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";

/// @title DeployCBWeb3
/// @author CBWeb3 Team
/// @notice Master orchestration script to deploy the entire CBWeb3 core platform.
/// @dev Deploys tCeBM tokens, HTLC (Scenario A), and AMM (Scenario B) in a single broadcast.
///      This script reads configuration from environment variables and deploys all contracts
///      to the target Hyperledger Besu network.
contract DeployCBWeb3 is Script {
    /// @notice Tokenised Central Bank Money representing Brazilian Real
    TokenizedCentralBankMoney public tokenBrl;

    /// @notice Tokenised Central Bank Money representing Euro
    TokenizedCentralBankMoney public tokenEur;

    /// @notice Hash Time Locked Contract for atomic cross-border settlements
    HashTimeLockedContract public htlc;

    /// @notice Automated Market Maker for liquidity pool operations
    AutomatedMarketMaker public amm;

    /// @notice Sets up the initial state for the deployment script.
    /// @dev This function is called before `run()` and can be used for pre-deployment configuration.
    function setUp() public {}

    /// @notice Executes the deployment of all core CBWeb3 contracts.
    /// @dev Deploys contracts in three phases:
    ///      1. Base Assets: tCeBM_BRL and tCeBM_EUR tokens
    ///      2. HTLC: For Scenario A (Correspondent Banking with atomic swaps)
    ///      3. AMM: For Scenario B (Liquidity Pool operations)
    function run() public {
        /// @dev Load global environment variables for RBAC and deployer configuration
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address adminAddress = vm.envAddress("ADMIN_ADDRESS");
        address centralBankAddress = vm.envAddress("CENTRAL_BANK_ADDRESS");
        address governanceAddress = vm.envAddress("GOVERNANCE_ADDRESS");

        /// @dev Start broadcasting transactions to the Hyperledger Besu network
        vm.startBroadcast(deployerPrivateKey);

        /// @dev 1: Deploy Base Assets (Tokenised Central Bank Money)
        tokenBrl = new TokenizedCentralBankMoney("Tokenized BRL", "tCeBM_BRL", adminAddress, centralBankAddress);
        tokenEur = new TokenizedCentralBankMoney("Tokenized EUR", "tCeBM_EUR", adminAddress, centralBankAddress);

        /// @dev 2: Deploy HTLC (Scenario A - Correspondent Banking)
        htlc = new HashTimeLockedContract();

        /// @dev 3: Deploy AMM (Scenario B - Liquidity Pool)
        /// @dev The AMM receives the freshly deployed token addresses dynamically
        amm = new AutomatedMarketMaker(address(tokenBrl), address(tokenEur), adminAddress, governanceAddress);

        vm.stopBroadcast();

        // OUTPUT LOGS (For backend configuration in Go services)
        console.log("\n===============================================");
        console.log("      CBWEB3 PLATFORM SUCCESSFULLY DEPLOYED    ");
        console.log("===============================================");
        console.log("tCeBM_BRL Address: ", address(tokenBrl));
        console.log("tCeBM_EUR Address: ", address(tokenEur));
        console.log("HTLC Address:      ", address(htlc));
        console.log("AMM Address:       ", address(amm));
        console.log("===============================================\n");
    }
}
