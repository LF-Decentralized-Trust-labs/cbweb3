// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {SpokeBridge} from "../src/SpokeBridge.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {FiatCentralBankMoney} from "../src/FiatCentralBankMoney.sol";

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

    /// @notice Fiat Central Bank Money ERC-20 for the escrow flow (deposit/escrow/redeem)
    FiatCentralBankMoney public fiatToken;

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
    ///      5. FiatCentralBankMoney (fCeBM — fiat ERC-20 for deposit/escrow/redeem flow)
    function run() public {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        adminAddress = vm.envAddress("ADMIN_ADDRESS");
        centralBankAddress = vm.envAddress("CENTRAL_BANK_ADDRESS");
        string memory tokenName = vm.envString("TOKEN_NAME");
        string memory tokenSymbol = vm.envString("TOKEN_SYMBOL");
        string memory fiatTokenName = vm.envString("FIAT_TOKEN_NAME");
        string memory fiatTokenSymbol = vm.envString("FIAT_TOKEN_SYMBOL");

        vm.startBroadcast(deployerPrivateKey);

        identityRegistry = new IdentityRegistry(adminAddress);

        token = new TokenizedCentralBankMoney(tokenName, tokenSymbol, adminAddress, centralBankAddress);

        htlc = new HashTimeLockedContract(address(identityRegistry), address(0), address(0));

        spokeBridge = new SpokeBridge(address(identityRegistry), adminAddress);

        fiatToken = new FiatCentralBankMoney(fiatTokenName, fiatTokenSymbol, adminAddress, centralBankAddress);

        vm.stopBroadcast();

        console.log("\n===============================================");
        console.log("    CBWEB3 SPOKE SUCCESSFULLY DEPLOYED         ");
        console.log("===============================================");
        console.log("Identity Registry: ", address(identityRegistry));
        console.log("Token (%s):       ", tokenSymbol, address(token));
        console.log("HTLC Address:      ", address(htlc));
        console.log("Spoke Bridge:      ", address(spokeBridge));
        console.log("Fiat Token (%s):  ", fiatTokenSymbol, address(fiatToken));
        console.log("===============================================\n");
    }
}
