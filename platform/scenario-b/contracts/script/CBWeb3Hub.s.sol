// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {PairRegistry} from "../src/PairRegistry.sol";
import {CurrencyRegistry} from "../src/CurrencyRegistry.sol";
import {FXAgreement} from "../src/FXAgreement.sol";
import {ManualOracle} from "../src/ManualOracle.sol";

/// @title DeployCBWeb3Hub
/// @notice Hub deployment script - deploys all core platform contracts on the hub ledger.
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

    /// @notice PairRegistry for multi-pair AMM routing (D9 — 005-cooperative-liquidity)
    PairRegistry public pairRegistry;

    /// @notice CurrencyRegistry for hub currency discovery (006-hub-currency-registry)
    CurrencyRegistry public currencyRegistry;

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

        /// @dev 5b: Deploy PairRegistry (multi-pair routing — D9 / 005-cooperative-liquidity)
        /// @notice PairRegistry authorises new AMM pairs via bilateral CB approval.
        ///         PAIR_REGISTRY_CONTRACT_ADDRESS env var must be set in the API Gateway.
        pairRegistry = new PairRegistry(address(identityRegistry));

        /// @dev 5c: Deploy CurrencyRegistry (hub currency discovery — 006-hub-currency-registry)
        /// @notice Stores (symbol, countryName, tokenAddress, proposerCB) for each hub currency.
        ///         Auth delegated to IdentityRegistry.getCentralBankOf(tokenAddress).
        ///         CURRENCY_REGISTRY_CONTRACT_ADDRESS env var must be set in the API Gateway.
        currencyRegistry = new CurrencyRegistry(address(identityRegistry));

        /// @dev 6: Deploy Manual FX Oracle (REQ-FX-002)
        oracle = new ManualOracle(adminAddress, centralBankAddress);

        vm.stopBroadcast();

        /// @dev 7: Grant Liquidity Provider role to Central Bank (005-cooperative-liquidity / FR-004).
        /// @notice Must be executed by the admin (DEFAULT_ADMIN_ROLE holder), not the deployer.
        ///         Uses ADMIN_PRIVATE_KEY env var (same pattern as SeedHub.s.sol).
        uint256 adminPrivateKey = vm.envOr("ADMIN_PRIVATE_KEY", uint256(0));
        if (adminPrivateKey != 0) {
            vm.startBroadcast(adminPrivateKey);
            identityRegistry.grantLiquidityProvider(centralBankAddress);
            console.log("grantLiquidityProvider: Central Bank =", centralBankAddress);

            // CENTRAL_BANK_ROLE on tokenEur (token_b) for CB-B:
            // Each token issuer must be able to call mint() on the token it issues.
            // tokenBrl is issued by CB-A (centralBankAddress, set at construction).
            // tokenEur is issued by CB-B (CENTRAL_BANK_B_ADDRESS) — grant here.
            // Without this, CB-B's mint-and-approve call reverts on-chain.
            address centralBankBAddress = vm.envOr("CENTRAL_BANK_B_ADDRESS", address(0));
            if (centralBankBAddress != address(0)) {
                tokenEur.grantRole(tokenEur.CENTRAL_BANK_ROLE(), centralBankBAddress);
                console.log("grantRole CENTRAL_BANK_ROLE(tokenEur): CB-B =", centralBankBAddress);
            } else {
                console.log("INFO: CENTRAL_BANK_B_ADDRESS not set - CB-B role on tokenEur skipped.");
                console.log("      Run: make contracts.grant-central-bank-b-role CENTRAL_BANK_B_ADDRESS=<addr>");
            }

            // MLP (Path B) — optional, activated when MLP_ADDRESS is set.
            // The MLP is an independent entity with its own Ethereum keypair.
            // Approval occurs off-chain (consortium agreement); the admin executes
            // the grant after approval (FR-004 / spec clarification Q2).
            address mlpAddress = vm.envOr("MLP_ADDRESS", address(0));
            if (mlpAddress != address(0)) {
                identityRegistry.grantLiquidityProvider(mlpAddress);
                console.log("grantLiquidityProvider: MLP          =", mlpAddress);
            } else {
                console.log("INFO: MLP_ADDRESS not set - MLP grantLiquidityProvider skipped.");
                console.log("      Set MLP_ADDRESS env var and re-run admin section to enable MLP.");
            }

            vm.stopBroadcast();
        } else {
            console.log("WARN: ADMIN_PRIVATE_KEY not set - skipping grantLiquidityProvider.");
            console.log("      Run: identityRegistry.grantLiquidityProvider(centralBankAddress) as admin.");
        }

        // OUTPUT LOGS (Required for Backend & API Gateway configuration)
        console.log("\n===============================================");
        console.log("      CBWEB3 HUB SUCCESSFULLY DEPLOYED         ");
        console.log("===============================================");
        console.log("Identity Registry: ", address(identityRegistry));
        console.log("tCeBM_BRL Address: ", address(tokenBrl));
        console.log("tCeBM_EUR Address: ", address(tokenEur));
        console.log("HTLC Address:      ", address(htlc));
        console.log("AMM Address:       ", address(amm));
        console.log("PairRegistry:      ", address(pairRegistry));
        console.log("CurrencyRegistry:  ", address(currencyRegistry));
        console.log("FX Agreement:      ", address(fxAgreement));
        console.log("Manual Oracle:     ", address(oracle));
        console.log("===============================================\n");
    }
}
