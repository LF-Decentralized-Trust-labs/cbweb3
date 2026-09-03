// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {PairRegistry} from "../src/PairRegistry.sol";
import {CurrencyRegistry} from "../src/CurrencyRegistry.sol";
import {FXAgreement} from "../src/FXAgreement.sol";
import {ManualOracle} from "../src/ManualOracle.sol";
import {LiquidityCommitRegistry} from "../src/LiquidityCommitRegistry.sol";

/// @title DeployCBWeb3Hub
/// @notice Hub deployment script - deploys core platform contracts on the hub ledger.
/// @dev tCeBM reserve tokens (TokenizedCentralBankMoney) are NOT deployed here — they are
///      created later when each CB registers a currency (CurrencyRegistry + token deploy).
contract DeployCBWeb3Hub is Script {
    /// @notice Central Identity and Compliance Registry
    IdentityRegistry public identityRegistry;

    /// @notice Hash Time Locked Contract for atomic cross-border settlements
    HashTimeLockedContract public htlc;

    /// @notice PairRegistry for multi-pair AMM routing (D9 — 005-cooperative-liquidity)
    PairRegistry public pairRegistry;

    /// @notice CurrencyRegistry for hub currency discovery (006-hub-currency-registry)
    CurrencyRegistry public currencyRegistry;

    /// @notice On-chain bilateral FX agreement registry
    FXAgreement public fxAgreement;

    /// @notice Manual FX rate oracle (CB-set rates)
    ManualOracle public oracle;

    /// @notice Hub-wide registry for cooperative sovereign-liquidity commits +
    /// the cross-currency bridge lock-mint / burn-release positions.
    LiquidityCommitRegistry public liquidityCommitRegistry;

    /// @notice Admin address used during deployment (for test assertions)
    address public adminAddress;

    /// @notice Central Bank address used during deployment (for test assertions)
    address public centralBankAddress;

    /// @notice Sets up the initial state for the deployment script.
    function setUp() public {}

    /// @notice Executes the deployment of core CBWeb3 hub contracts (no tCeBM tokens).
    function run() public {
        /// @dev Load global environment variables
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        adminAddress = vm.envAddress("ADMIN_ADDRESS");
        centralBankAddress = vm.envAddress("CENTRAL_BANK_ADDRESS");

        vm.startBroadcast(deployerPrivateKey);

        /// @dev 1: Deploy Identity Registry (The Foundation)
        identityRegistry = new IdentityRegistry(adminAddress);

        /// @dev 2: Deploy FX Agreement Registry (REQ-FX-001)
        fxAgreement = new FXAgreement(address(identityRegistry));

        /// @dev 3: Deploy HTLC (Scenario A - Correspondent Banking, with FX Agreement gate)
        htlc = new HashTimeLockedContract(address(identityRegistry), address(fxAgreement), address(0));

        /// @dev 4: No default AMM (TD-001). Each corridor gets a dedicated per-pair
        ///      AutomatedMarketMaker deployed at propose time (PairRegistry.proposePair).

        /// @dev 4b: Deploy PairRegistry (multi-pair routing — D9 / 005-cooperative-liquidity)
        pairRegistry = new PairRegistry(address(identityRegistry));

        /// @dev 4c: Deploy CurrencyRegistry (hub currency discovery — 006-hub-currency-registry)
        currencyRegistry = new CurrencyRegistry(address(identityRegistry));

        /// @dev 5: Deploy Manual FX Oracle (REQ-FX-002)
        oracle = new ManualOracle(adminAddress, centralBankAddress);

        /// @dev 6: Deploy the hub-wide LiquidityCommitRegistry
        liquidityCommitRegistry = new LiquidityCommitRegistry(address(identityRegistry));
        console.log("LiquidityCommitRegistry:", address(liquidityCommitRegistry));

        vm.stopBroadcast();

        /// @dev Grant Liquidity Provider role to Central Bank (005-cooperative-liquidity / FR-004).
        uint256 adminPrivateKey = vm.envOr("ADMIN_PRIVATE_KEY", uint256(0));
        if (adminPrivateKey != 0) {
            vm.startBroadcast(adminPrivateKey);
            identityRegistry.grantLiquidityProvider(centralBankAddress);
            console.log("grantLiquidityProvider: Central Bank =", centralBankAddress);

            address mlpAddress = vm.envOr("MLP_ADDRESS", address(0));
            if (mlpAddress != address(0)) {
                identityRegistry.grantLiquidityProvider(mlpAddress);
                console.log("grantLiquidityProvider: MLP          =", mlpAddress);
            } else {
                console.log("INFO: MLP_ADDRESS not set - MLP grantLiquidityProvider skipped.");
            }

            vm.stopBroadcast();
        } else {
            console.log("WARN: ADMIN_PRIVATE_KEY not set - skipping grantLiquidityProvider.");
            console.log("      Run: identityRegistry.grantLiquidityProvider(centralBankAddress) as admin.");
        }

        console.log("\n===============================================");
        console.log("      CBWEB3 HUB SUCCESSFULLY DEPLOYED         ");
        console.log("===============================================");
        console.log("Identity Registry: ", address(identityRegistry));
        console.log("HTLC Address:      ", address(htlc));
        console.log("PairRegistry:      ", address(pairRegistry));
        console.log("CurrencyRegistry:  ", address(currencyRegistry));
        console.log("FX Agreement:      ", address(fxAgreement));
        console.log("Manual Oracle:     ", address(oracle));
        console.log("NOTE: tCeBM tokens are deployed later (per-CB currency registration).");
        console.log("===============================================\n");
    }
}
