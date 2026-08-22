// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {FXAgreement} from "../src/FXAgreement.sol";
import {ManualOracle} from "../src/ManualOracle.sol";

/// @title DeployCBWeb3Hub
/// @notice Hub deployment script — deploys all core platform contracts on the hub ledger.
/// @dev Deploys Identity Registry, tCeBM tokens (BRL + EUR), HTLC (Scenario A), AMM (Scenario B),
///      FXAgreement (bilateral deal registry), and ManualOracle (CB-set FX rates).
contract DeployCBWeb3Hub is Script {
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

    /// @notice On-chain bilateral FX agreement registry
    FXAgreement public fxAgreement;

    /// @notice Manual FX rate oracle (CB-set rates)
    ManualOracle public oracle;

    /// @notice Admin address used during deployment (for test assertions)
    address public adminAddress;

    /// @notice Central Bank address used during deployment (for test assertions)
    address public centralBankAddress;

    /// @notice Sets up the initial state for the deployment script.
    function setUp() public {}

    /// @notice Executes the deployment of all core CBWeb3 hub contracts.
    /// @dev Deployment Order:
    ///      1. Identity & Governance (Registry)
    ///      2. Base Assets (tCeBM_BRL and tCeBM_EUR)
    ///      3. Settlement Logic (HTLC and AMM)
    ///      4. FX Agreement Registry
    ///      5. Manual FX Oracle
    function run() public {
        /// @dev Load global environment variables
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        adminAddress = vm.envAddress("ADMIN_ADDRESS");
        centralBankAddress = vm.envAddress("CENTRAL_BANK_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        /// @dev 1: Deploy Identity Registry (The Foundation)
        /// @notice This registry manages KYC/AML for all subsequent participants.
        identityRegistry = new IdentityRegistry(adminAddress);

        /// @dev 2: Deploy Base Assets (Tokenised Central Bank Money)
        tokenBrl = new TokenizedCentralBankMoney("Tokenized BRL", "tCeBM_BRL", adminAddress, centralBankAddress);
        tokenEur = new TokenizedCentralBankMoney("Tokenized EUR", "tCeBM_EUR", adminAddress, centralBankAddress);

        /// @dev 3: Deploy FX Agreement Registry (REQ-FX-001)
        fxAgreement = new FXAgreement(address(identityRegistry));

        /// @dev 4: Deploy HTLC (Scenario A - Correspondent Banking, with FX Agreement gate)
        htlc = new HashTimeLockedContract(address(identityRegistry), address(fxAgreement), address(0));

        /// @dev 5: Deploy AMM (Scenario B - Liquidity Pool)
        amm = new AutomatedMarketMaker(address(tokenBrl), address(tokenEur), address(identityRegistry));

        /// @dev 6: Deploy Manual FX Oracle (REQ-FX-002)
        oracle = new ManualOracle(adminAddress, centralBankAddress);

        vm.stopBroadcast();

        // OUTPUT LOGS (Required for Backend & API Gateway configuration)
        console.log("\n===============================================");
        console.log("      CBWEB3 HUB SUCCESSFULLY DEPLOYED         ");
        console.log("===============================================");
        console.log("Identity Registry: ", address(identityRegistry));
        console.log("tCeBM_BRL Address: ", address(tokenBrl));
        console.log("tCeBM_EUR Address: ", address(tokenEur));
        console.log("HTLC Address:      ", address(htlc));
        console.log("AMM Address:       ", address(amm));
        console.log("FX Agreement:      ", address(fxAgreement));
        console.log("Manual Oracle:     ", address(oracle));
        console.log("===============================================\n");
    }
}
