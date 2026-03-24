// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {SpokeBridge} from "../src/SpokeBridge.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";

/// @title DeployCBWeb3Spoke
/// @notice Spoke deployment script — deploys the contracts required for a regional spoke ledger.
/// @dev Deploys an IdentityRegistry (local KYC), a single TokenizedCentralBankMoney (domestic currency),
///      an HTLC (for domestic legs of Scenario A cross-border settlement),
///      and a SpokeBridge (lock-and-mint for Scenario B AMM participation).
///      Hub-only contracts (AMM) are NOT deployed here.
contract DeployCBWeb3Spoke is Script {
    /// @notice Regional Identity and Compliance Registry
    IdentityRegistry public identityRegistry;

    /// @notice Single Tokenised Central Bank Money for the spoke's domestic currency
    TokenizedCentralBankMoney public token;

    /// @notice Hash Time Locked Contract for domestic settlement legs (Scenario A)
    HashTimeLockedContract public htlc;

    /// @notice Lock-and-Mint bridge for Scenario B AMM participation
    SpokeBridge public spokeBridge;

    /// @notice Admin address used during deployment (for test assertions)
    address public adminAddress;

    /// @notice Central Bank address used during deployment (for test assertions)
    address public centralBankAddress;

    /// @notice Sets up the initial state for the deployment script.
    function setUp() public {}

    /// @notice Executes the deployment of spoke-specific contracts.
    /// @dev Deployment Order:
    ///      1. Identity Registry (local KYC/AML for regional participants)
    ///      2. Tokenised Central Bank Money (domestic currency, e.g., tCeBM_BRL or tCeBM_EUR)
    ///      3. HTLC (domestic settlement leg for Scenario A cross-border flows)
    ///      4. SpokeBridge (lock-and-mint bridge for Scenario B)
    function run() public {
        /// @dev Load environment variables
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        adminAddress = vm.envAddress("ADMIN_ADDRESS");
        centralBankAddress = vm.envAddress("CENTRAL_BANK_ADDRESS");
        string memory tokenName = vm.envString("TOKEN_NAME");
        string memory tokenSymbol = vm.envString("TOKEN_SYMBOL");

        vm.startBroadcast(deployerPrivateKey);

        /// @dev 1: Deploy Identity Registry (regional KYC)
        identityRegistry = new IdentityRegistry(adminAddress);

        /// @dev 2: Deploy domestic currency token
        token = new TokenizedCentralBankMoney(tokenName, tokenSymbol, adminAddress, centralBankAddress);

        /// @dev 3: Deploy HTLC (Scenario A — domestic settlement leg)
        htlc = new HashTimeLockedContract(address(identityRegistry));

        /// @dev 4: Deploy SpokeBridge (Scenario B — lock-and-mint)
        spokeBridge = new SpokeBridge(address(identityRegistry), adminAddress);

        vm.stopBroadcast();

        // OUTPUT LOGS
        console.log("\n===============================================");
        console.log("    CBWEB3 SPOKE SUCCESSFULLY DEPLOYED         ");
        console.log("===============================================");
        console.log("Identity Registry: ", address(identityRegistry));
        console.log("Token (%s):       ", tokenSymbol, address(token));
        console.log("HTLC Address:      ", address(htlc));
        console.log("Spoke Bridge:      ", address(spokeBridge));
        console.log("===============================================\n");
    }
}
