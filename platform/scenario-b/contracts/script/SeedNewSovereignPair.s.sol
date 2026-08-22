// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {LiquidityCommitRegistry} from "../src/LiquidityCommitRegistry.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {PairRegistry} from "../src/PairRegistry.sol";

/// @title SeedNewSovereignPair
/// @notice Deploys and wires a new sovereign CB liquidity pair on the Hub.
///
/// @dev Execution sequence:
///      1. Deploy W-tCeBM_A (token for CB-A) and W-tCeBM_B (token for CB-B).
///      2. Register both CBs as CENTRAL_BANK_ROLE on their respective tokens.
///      3. Map each token to its issuing CB in the IdentityRegistry
///         (setCentralBankOf) — required by LiquidityCommitRegistry auth.
///      4. Optionally deploy a new LiquidityCommitRegistry, or reuse an existing
///         one if LIQUIDITY_COMMIT_REGISTRY_ADDRESS is set.
///      5. Deploy a new AMM for the sovereign pair.
///      6. CB-A proposes the pair in PairRegistry (proposePair).
///      7. CB-B confirms the pair (confirmPair) — pair status becomes ACTIVE.
///      8. Grant CENTRAL_BANK_ROLE on each token to the RELAYER_ADDR (the
///         Cacti watcher address that calls addSingleSidedLiquidity on behalf of
///         the sovereign CB's signer). Optional: skip if 0x0.
///
/// Required environment variables:
///   TOKEN_SYMBOL_A           — Symbol suffix for token A (e.g. "BRL")
///   TOKEN_SYMBOL_B           — Symbol suffix for token B (e.g. "ARS")
///   PAIR_ID                  — Pair identifier string (e.g. "W-BRL-ARS")
///   CB_A_HUB_PRIVATE_KEY     — Private key hex for CB-A signer (no 0x prefix)
///   CB_B_HUB_PRIVATE_KEY     — Private key hex for CB-B signer (no 0x prefix)
///   HUB_IDENTITY_REGISTRY    — Address of deployed IdentityRegistry on Hub
///   PAIR_REGISTRY_ADDRESS    — Address of deployed PairRegistry on Hub
///   ADMIN_ADDRESS            — Admin address (receives DEFAULT_ADMIN_ROLE on tokens)
///
/// Optional environment variables:
///   LIQUIDITY_COMMIT_REGISTRY_ADDRESS  — Reuse existing LCR (deploy new if absent)
///   RELAYER_ADDR                       — Cacti watcher address to grant CENTRAL_BANK_ROLE
///
/// Feature: 007-bridge-based-cb-liquidity
contract SeedNewSovereignPair is Script {
    function setUp() public {}

    function run() public {
        // ─── Load environment variables ──────────────────────────────────────
        string memory symbolA = vm.envString("TOKEN_SYMBOL_A");
        string memory symbolB = vm.envString("TOKEN_SYMBOL_B");
        string memory pairId = vm.envString("PAIR_ID");

        uint256 cbAKey = vm.envUint("CB_A_HUB_PRIVATE_KEY");
        uint256 cbBKey = vm.envUint("CB_B_HUB_PRIVATE_KEY");

        address hubIdentityRegistry = vm.envAddress("HUB_IDENTITY_REGISTRY");
        address pairRegistryAddr = vm.envAddress("PAIR_REGISTRY_ADDRESS");
        address adminAddr = vm.envAddress("ADMIN_ADDRESS");

        address existingLCR = vm.envOr("LIQUIDITY_COMMIT_REGISTRY_ADDRESS", address(0));
        address relayerAddr = vm.envOr("RELAYER_ADDR", address(0));

        address cbAAddr = vm.addr(cbAKey);
        address cbBAddr = vm.addr(cbBKey);

        // ─── Step 1: Deploy W-tCeBM tokens (admin broadcasts) ───────────────
        // Tokens need admin as DEFAULT_ADMIN_ROLE and the respective CB as CENTRAL_BANK_ROLE.
        uint256 adminKey = vm.envUint("ADMIN_PRIVATE_KEY");
        vm.startBroadcast(adminKey);

        string memory nameA = string.concat("Wrapped tCeBM ", symbolA);
        string memory tickA = string.concat("W-tCeBM_", symbolA);
        TokenizedCentralBankMoney tokenA = new TokenizedCentralBankMoney(nameA, tickA, adminAddr, cbAAddr);

        string memory nameB = string.concat("Wrapped tCeBM ", symbolB);
        string memory tickB = string.concat("W-tCeBM_", symbolB);
        TokenizedCentralBankMoney tokenB = new TokenizedCentralBankMoney(nameB, tickB, adminAddr, cbBAddr);

        console.log("W-tCeBM_%s deployed: %s", symbolA, address(tokenA));
        console.log("W-tCeBM_%s deployed: %s", symbolB, address(tokenB));

        // ─── Step 2: Map tokens to their sovereign CBs in IdentityRegistry ──
        IdentityRegistry registry = IdentityRegistry(hubIdentityRegistry);
        registry.setCentralBankOf(address(tokenA), cbAAddr);
        registry.setCentralBankOf(address(tokenB), cbBAddr);

        console.log(unicode"setCentralBankOf(tokenA \u2192 cbA): %s \u2192 %s", address(tokenA), cbAAddr);
        console.log(unicode"setCentralBankOf(tokenB \u2192 cbB): %s \u2192 %s", address(tokenB), cbBAddr);

        // ─── Step 3: Optionally deploy LiquidityCommitRegistry ───────────────
        address lcrAddr = existingLCR;
        if (lcrAddr == address(0)) {
            LiquidityCommitRegistry lcr = new LiquidityCommitRegistry(hubIdentityRegistry);
            lcrAddr = address(lcr);
            console.log("LiquidityCommitRegistry deployed: %s", lcrAddr);
        } else {
            console.log("LiquidityCommitRegistry reused:   %s", lcrAddr);
        }

        // ─── Step 4: Grant CENTRAL_BANK_ROLE to relayer if provided ──────────
        if (relayerAddr != address(0)) {
            tokenA.grantRole(tokenA.CENTRAL_BANK_ROLE(), relayerAddr);
            tokenB.grantRole(tokenB.CENTRAL_BANK_ROLE(), relayerAddr);
            console.log("CENTRAL_BANK_ROLE granted to relayer: %s (on both tokens)", relayerAddr);
        }

        vm.stopBroadcast();

        // ─── Step 5: CB-A deploys AMM and proposes the pair ──────────────────
        vm.startBroadcast(cbAKey);

        AutomatedMarketMaker amm = new AutomatedMarketMaker(address(tokenA), address(tokenB), hubIdentityRegistry);
        console.log("AMM deployed (sovereign pair): %s", address(amm));

        PairRegistry pairRegistry = PairRegistry(pairRegistryAddr);
        pairRegistry.proposePair(pairId, address(tokenA), address(tokenB), address(amm));
        console.log("proposePair('%s') sent by CB-A (%s)", pairId, cbAAddr);

        vm.stopBroadcast();

        // ─── Step 6: CB-B confirms the pair ──────────────────────────────────
        vm.startBroadcast(cbBKey);

        pairRegistry.confirmPair(pairId);
        console.log("confirmPair('%s') confirmed by CB-B (%s)", pairId, cbBAddr);

        vm.stopBroadcast();

        // ─── Output summary ───────────────────────────────────────────────────
        console.log("\n===============================================");
        console.log(" SOVEREIGN PAIR SEEDED SUCCESSFULLY");
        console.log("===============================================");
        console.log("Pair ID:                          %s", pairId);
        console.log("W-tCeBM_%s:                       %s", symbolA, address(tokenA));
        console.log("W-tCeBM_%s:                       %s", symbolB, address(tokenB));
        console.log("AMM Address:                      %s", address(amm));
        console.log("LiquidityCommitRegistry:          %s", lcrAddr);
        console.log("PairRegistry:                     %s", pairRegistryAddr);
        console.log("CB-A Signer:                      %s", cbAAddr);
        console.log("CB-B Signer:                      %s", cbBAddr);
        console.log("===============================================");
        console.log("Set in your gateway compose files:");
        console.log("  LIQUIDITY_COMMIT_REGISTRY_ADDRESS=%s", lcrAddr);
        console.log("  SOVEREIGN_PAIR_AMM_MAP={\\\"W-%s-%s\\\":\\\"%s\\\"}", symbolA, symbolB, address(amm));
        console.log("  SOVEREIGN_PAIR_IDS=W-%s-%s", symbolA, symbolB);
        console.log("  W_TOKEN_%s_ADDRESS=%s", symbolA, address(tokenA));
        console.log("  W_TOKEN_%s_ADDRESS=%s", symbolB, address(tokenB));
        console.log("  LOCAL_CB_HUB_SIGNER=<your CB's signer address>");
        console.log("===============================================\n");
    }
}
