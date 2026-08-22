// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";

/// @title SeedHub — Local Dev Token Minting & Initial AMM Liquidity
/// @notice Registers hub participants, mints tCeBM_BRL and tCeBM_EUR to all
///         local dev addresses, and seeds the AMM pool so quote/swap work
///         immediately after `make spoke-all`.
///
/// @dev Environment variables:
///        ADMIN_PRIVATE_KEY        — governance/admin key (register participants)
///        CENTRAL_BANK_PRIVATE_KEY — CB key (has CENTRAL_BANK_ROLE on both tokens)
///        TOKEN_BRL                — tCeBM_BRL contract address
///        TOKEN_EUR                — tCeBM_EUR contract address
///        AMM_ADDRESS              — AutomatedMarketMaker contract address
///        HUB_IDENTITY_REGISTRY    — Hub IdentityRegistry address (used by AMM)
///        MINT_AMOUNT              — Amount per address (default: 1_000_000 * 1e18)
///        SEED_AMOUNT              — Initial liquidity per token (default: 100_000 * 1e18)
///
///      Automatically run by `contracts.seed-hub` Makefile target, wired into
///      `contracts.deploy-all-with-sync`.
contract SeedHub is Script {
    // Hardcoded Besu genesis dev addresses.
    address constant BANK_AB_ADDRESS = 0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef;
    address constant BANK_CD_ADDRESS = 0xc110089385bad5026E5083443C3b443806DA42Df;
    // Default AMM signer used by api-gateway containers
    address constant AMM_SIGNER = 0xFE3B557E8Fb62b89F4916B721be55cEb828dBd73;
    // MLP dev signer — Besu genesis key #4 (pre-funded).
    // Private key: f8f8a2f43c8376ccb0871305060d7b27b0554d2cc72bccf41b2705608452f315
    // Override via MLP_ADDRESS env var (set to address(0) to disable MLP seeding).
    address constant MLP_SIGNER_DEFAULT = 0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b;

    function setUp() public {}

    function run() external {
        uint256 adminKey = vm.envUint("ADMIN_PRIVATE_KEY");
        uint256 cbKey = vm.envUint("CENTRAL_BANK_PRIVATE_KEY");
        address tokenBrl = vm.envAddress("TOKEN_BRL");
        address tokenEur = vm.envAddress("TOKEN_EUR");
        address ammAddress = vm.envAddress("AMM_ADDRESS");

        uint256 mintAmount = vm.envOr("MINT_AMOUNT", uint256(1_000_000 * 1e18));
        uint256 seedAmount = vm.envOr("SEED_AMOUNT", uint256(100_000 * 1e18));
        address centralBankSigner = vm.addr(cbKey);
        address mlpSigner = vm.envOr("MLP_ADDRESS", MLP_SIGNER_DEFAULT);

        address[5] memory recipients = [centralBankSigner, BANK_AB_ADDRESS, BANK_CD_ADDRESS, AMM_SIGNER, mlpSigner];

        // 1. Register hub participants (idempotent — skips already-registered addresses).
        //    AMM_SIGNER gets CENTRAL_BANK role so canGovern() also returns true.
        //    mlpSigner gets ParticipantRole.MLP (distinct from CENTRAL_BANK per FR-004).
        address hubRegistryAddr = vm.envOr("HUB_IDENTITY_REGISTRY", address(0));
        if (hubRegistryAddr != address(0)) {
            vm.startBroadcast(adminKey);
            IdentityRegistry hubRegistry = IdentityRegistry(hubRegistryAddr);
            _registerIfNeeded(
                hubRegistry,
                centralBankSigner,
                "Central Bank",
                IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK,
                vm.envOr("CENTRAL_BANK_A_CODE", string("central-bank-a"))
            );
            _registerIfNeeded(
                hubRegistry,
                BANK_AB_ADDRESS,
                "Commercial Bank AB",
                IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK,
                "bank-ab"
            );
            _registerIfNeeded(
                hubRegistry,
                BANK_CD_ADDRESS,
                "Commercial Bank CD",
                IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK,
                "bank-cd"
            );
            // The dev gateway signer holds CENTRAL_BANK role so canGovern() passes, but it is not a
            // sovereign CB — its own code keeps it from silently sharing a real CB's quorum vote.
            _registerIfNeeded(
                hubRegistry,
                AMM_SIGNER,
                "AMM Gateway Dev",
                IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK,
                "amm-gateway-dev"
            );
            _registerIfNeeded(hubRegistry, mlpSigner, "MLP", IdentityRegistryLibrary.ParticipantRole.MLP, "mlp");
            console.log("Hub participants registered");

            // Grant LiquidityProvider role to gateway signers so they can call
            // addSingleSidedLiquidity on the shared AMM (005-cooperative-liquidity / 007).
            // CB-A signer = centralBankSigner; CB-B signer from CENTRAL_BANK_B_ADDRESS env.
            // AMM_SIGNER (deployer key) is also granted for legacy addLiquidity flows.
            address cbBSigner = vm.envOr("CENTRAL_BANK_B_ADDRESS", address(0));
            if (!hubRegistry.isLiquidityProvider(centralBankSigner)) {
                hubRegistry.grantLiquidityProvider(centralBankSigner);
                console.log("[Hub] LiquidityProvider granted to CB-A signer:", centralBankSigner);
            }
            if (!hubRegistry.isLiquidityProvider(AMM_SIGNER)) {
                hubRegistry.grantLiquidityProvider(AMM_SIGNER);
                console.log("[Hub] LiquidityProvider granted to AMM_SIGNER:", AMM_SIGNER);
            }
            if (cbBSigner != address(0) && !hubRegistry.isLiquidityProvider(cbBSigner)) {
                hubRegistry.grantLiquidityProvider(cbBSigner);
                console.log("[Hub] LiquidityProvider granted to CB-B signer:", cbBSigner);
            }

            vm.stopBroadcast();
        } else {
            console.log("HUB_IDENTITY_REGISTRY not set - skipping hub participant registration");
        }

        // 2. Mint tokens to all participants
        vm.startBroadcast(cbKey);
        for (uint256 i = 0; i < recipients.length; i++) {
            TokenizedCentralBankMoney(tokenBrl).mint(recipients[i], mintAmount);
            TokenizedCentralBankMoney(tokenEur).mint(recipients[i], mintAmount);
            console.log("Minted to:", recipients[i]);
        }

        // 3. Approve AMM, then seed initial liquidity (CB is already verified in hub registry)
        TokenizedCentralBankMoney(tokenBrl).approve(ammAddress, seedAmount);
        TokenizedCentralBankMoney(tokenEur).approve(ammAddress, seedAmount);
        AutomatedMarketMaker(ammAddress).addLiquidity(seedAmount, seedAmount);
        console.log("AMM seeded with initial liquidity:", seedAmount);

        vm.stopBroadcast();
    }

    /// @param institutionCode Institution-level code, shared by every wallet of one institution and
    ///        hashed into the on-chain institutionId. It must match the BANK_CODE that institution's
    ///        services run with, or the same institution ends up with two ids and the AMM resume
    ///        quorum — which counts institutions, not keys — stops meaning what it says.
    function _registerIfNeeded(
        IdentityRegistry registry,
        address account,
        string memory name,
        IdentityRegistryLibrary.ParticipantRole role,
        string memory institutionCode
    ) internal {
        if (registry.canTransact(account)) {
            console.log("[Hub] Already registered:", name);
            return;
        }
        // Two-step onboarding: register (Pending) then verify (Pending -> Verified). The admin
        // broadcast key holds both GOVERNANCE_ROLE and VERIFIER_ROLE in local dev.
        registry.registerParticipant(account, name, role, bytes32(0), keccak256(abi.encodePacked(institutionCode)));
        registry.verifyParticipant(account);
        console.log("[Hub] Registered and verified:", name);
    }
}
