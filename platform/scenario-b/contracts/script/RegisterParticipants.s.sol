// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {IIdentityRegistry} from "../src/interfaces/IIdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";

/// @title RegisterParticipants — Local Dev Convenience Script
/// @notice Registers hardcoded Besu genesis addresses as verified participants
///         in the compliance IdentityRegistry so that the on-chain HTLC
///         coordination contract's `onlyVerified` modifier does not revert.
///
/// @dev **Purpose**
///      This script exists solely to make the happy-path testing of the
///      cross-spoke atomic swap easier when running the tryout scripts
///      (e.g. `./tryout-htlc-cross-spoke.sh`).
///
///      In production the participant registration is handled by the full
///      3-phase onboarding flow (credential request → KYC approval → PoP),
///      which is exercised by `./tryout-spoke-a-bank-a.sh`.
///      This script bypasses that flow by writing directly to the on-chain
///      IdentityRegistry with the governance key.
///
///      **When is it run?**
///      Automatically as part of:
///        • `make spoke-all`  (via `contracts.register-participants-spoke-{a,b}`)
///        • `make dev.up`     (via `contracts.deploy-all-with-sync`)
///      Or manually:
///        • `make contracts.register-participants`
///
///      **What it registers**
///      Three Besu genesis accounts used by the local dev payment-orchestrator
///      containers (configured via BESU_OPERATOR_KEY in docker-compose):
///
///        | Address                                    | Role            | Key (BESU_OPERATOR_KEY)   |
///        |--------------------------------------------|-----------------|--------------------------|
///        | 0x627306090abaB3A6e1400e9345bC60c78a8BEf57 | CENTRAL_BANK    | c87509a1… (CB)            |
///        | 0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef | COMMERCIAL_BANK | 0dbbe8e4… (Bank-A/B)      |
///        | 0x821aEa9a577a9b44299B9c15c88cf3087F3b5544 | COMMERCIAL_BANK | c88b703f… (Bank-C/D)      |
///
///      The script is idempotent — it skips addresses that are already registered.
///
///      **Environment variables**
///        ADMIN_PRIVATE_KEY  — Private key with GOVERNANCE_ROLE (CB key in local dev)
///        IDENTITY_REGISTRY  — Address of the compliance IdentityRegistry contract
///
///      **Example (standalone)**
///        ADMIN_PRIVATE_KEY=0xc87509a1… IDENTITY_REGISTRY=0x42699A76… \
///          forge script script/RegisterParticipants.s.sol:RegisterParticipants \
///          --rpc-url http://127.0.0.1:8645 --broadcast
contract RegisterParticipants is Script {
    struct Participant {
        address account;
        string name;
        IdentityRegistryLibrary.ParticipantRole role;
        /// @dev Institution code, NOT a per-wallet value. Every wallet of the same institution must
        ///      carry the same code, because the AMM resume quorum counts distinct institutions and
        ///      derives them from keccak256(bankCode) — the same derivation the Go services use
        ///      (backend/shared/blockchain/registry.InstitutionIDForParticipant, fed from BANK_CODE,
        ///      which the compose template sets to the manifest entity id). A second wallet of one
        ///      central bank registered under a different code would count as a second institution
        ///      and could satisfy the 2-of-N resume on its own.
        string bankCode;
    }

    function run() public {
        // The admin key has GOVERNANCE_ROLE. In local dev this is the CB key
        // (ADMIN_ADDRESS=0x627306…), NOT the deployer key.
        uint256 adminKey = vm.envUint("ADMIN_PRIVATE_KEY");
        address registryAddr = vm.envAddress("IDENTITY_REGISTRY");

        IIdentityRegistry registry = IIdentityRegistry(registryAddr);

        // Spoke operator addresses (Besu genesis accounts used in local dev).
        // Bank-A / Bank-B share key 0dbbe8e4… → 0xC5fdf4…
        // Bank-C / Bank-D share key c88b703f… → 0xc110…
        // Platform admin/deployer key c87509a1… → 0x627306… (ADMIN_ADDRESS; registry/bridge admin,
        //   hub DEFAULT_ADMIN_ROLE — NOT a sovereign CB identity).
        // Central Bank A key 8f2a5594… → 0xfe3b55… (spoke-a CB + hub identity for CB-A).
        // Central Bank B key ae6ae8e5… → 0xf17f52… (spoke-b CB + hub identity for CB-B).
        // Institution codes must match the BANK_CODE each entity's services run with (the compose
        // template sets BANK_CODE to the manifest entity id), so a wallet seeded here and a wallet
        // onboarded later by that entity's auth service resolve to the SAME institution. Overridable
        // because the manifest ids differ per deployment; the defaults match samples/.
        string memory cbACode = vm.envOr("CENTRAL_BANK_A_CODE", string("central-bank-a"));
        string memory cbBCode = vm.envOr("CENTRAL_BANK_B_CODE", string("central-bank-b"));

        Participant[5] memory participants = [
            // The platform admin is explicitly NOT a sovereign CB (see the key comment above), so it
            // carries its own institution code rather than borrowing a central bank's.
            Participant(
                0x627306090abaB3A6e1400e9345bC60c78a8BEf57,
                "Platform Admin",
                IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK,
                "platform-admin"
            ),
            Participant(
                0xFE3B557E8Fb62b89F4916B721be55cEb828dBd73,
                "Central Bank A",
                IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK,
                cbACode
            ),
            Participant(
                0xf17f52151EbEF6C7334FAD080c5704D77216b732,
                "Central Bank B",
                IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK,
                cbBCode
            ),
            // One key serves banks A and B in local dev, so it is one institution here by
            // construction — the shared key, not the code, is what makes it so.
            Participant(
                0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef,
                "Commercial Bank A/B",
                IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK,
                "bank-ab"
            ),
            Participant(
                0xc110089385bad5026E5083443C3b443806DA42Df,
                "Commercial Bank C/D",
                IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK,
                "bank-cd"
            )
        ];

        vm.startBroadcast(adminKey);

        for (uint256 i = 0; i < participants.length; i++) {
            // Skip if already registered (canTransact returns true).
            if (registry.canTransact(participants[i].account)) {
                console.log("Already registered:", participants[i].name, participants[i].account);
                continue;
            }
            // Two-step onboarding: registerParticipant creates the participant in Pending;
            // verifyParticipant (VERIFIER_ROLE) promotes it to Verified so canTransact returns true.
            // The ADMIN key holds both GOVERNANCE_ROLE and VERIFIER_ROLE in local dev.
            registry.registerParticipant(
                participants[i].account,
                participants[i].name,
                participants[i].role,
                keccak256(abi.encodePacked("local_dev_", participants[i].name)),
                keccak256(abi.encodePacked(participants[i].bankCode))
            );
            registry.verifyParticipant(participants[i].account);
            console.log("Registered and verified:", participants[i].name, participants[i].account);
        }

        vm.stopBroadcast();

        // Also register in the Hub IdentityRegistry (used by AMM, FXAgreement on the hub chain).
        // HUB_IDENTITY_REGISTRY is optional — skipped if not set.
        address hubRegistryAddr = vm.envOr("HUB_IDENTITY_REGISTRY", address(0));
        if (hubRegistryAddr == address(0)) {
            console.log("HUB_IDENTITY_REGISTRY not set - skipping hub registry registration");
            return;
        }

        IIdentityRegistry hubRegistry = IIdentityRegistry(hubRegistryAddr);
        vm.startBroadcast(adminKey);

        for (uint256 i = 0; i < participants.length; i++) {
            if (hubRegistry.canTransact(participants[i].account)) {
                console.log("[Hub] Already registered:", participants[i].name);
                continue;
            }
            hubRegistry.registerParticipant(
                participants[i].account,
                participants[i].name,
                participants[i].role,
                keccak256(abi.encodePacked("hub_local_dev_", participants[i].name)),
                // Same institution code as the spoke registration above: the resume quorum lives on
                // the hub AMM, so the hub registry is the one whose institution keying decides it.
                keccak256(abi.encodePacked(participants[i].bankCode))
            );
            hubRegistry.verifyParticipant(participants[i].account);
            console.log("[Hub] Registered and verified:", participants[i].name, participants[i].account);
        }

        vm.stopBroadcast();
    }
}
