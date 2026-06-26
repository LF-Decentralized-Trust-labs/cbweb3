// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

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
///      ONLY the Central Bank genesis account (the governance bootstrap, which
///      approves KYC and therefore cannot self-onboard):
///
///        | Address                                    | Role         | Key (BESU_OPERATOR_KEY) |
///        |--------------------------------------------|--------------|-------------------------|
///        | 0x627306090abaB3A6e1400e9345bC60c78a8BEf57 | CENTRAL_BANK | c87509a1… (CB)          |
///
///      Commercial banks are NOT registered here — they go through the real
///      3-phase onboarding flow (see Phase2_Onboard in the happy-path test).
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
    }

    function run() public {
        // The admin key has GOVERNANCE_ROLE. In local dev this is the CB key
        // (ADMIN_ADDRESS=0x627306…), NOT the deployer key.
        uint256 adminKey = vm.envUint("ADMIN_PRIVATE_KEY");
        address registryAddr = vm.envAddress("IDENTITY_REGISTRY");

        IIdentityRegistry registry = IIdentityRegistry(registryAddr);

        // Governance bootstrap ONLY: the Central Bank holds GOVERNANCE_ROLE and
        // approves KYC, so it cannot be onboarded through the commercial-bank flow
        // and must be registered directly.
        //
        // Commercial banks (Bank-A/B → 0xC5fdf4…, Bank-C/D → 0xc110…) are NO LONGER
        // registered here: they are registered+verified via the real 3-phase
        // onboarding flow (credential request → KYC approval → PoP), exercised by
        // the happy-path integration test (Phase2_Onboard) and the tryout scripts.
        // The local-dev KMS seeding (KMS_SEED_* on each bank's auth service) makes
        // the onboarded wallet equal that bank's BESU_OPERATOR_KEY address, so the
        // onboarded identity is the one that signs HTLC txs and passes onlyVerified.
        Participant[1] memory participants = [Participant(
                0x627306090abaB3A6e1400e9345bC60c78a8BEf57,
                "Central Bank",
                IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK
            )];

        vm.startBroadcast(adminKey);

        for (uint256 i = 0; i < participants.length; i++) {
            // Skip if already registered (canTransact returns true).
            if (registry.canTransact(participants[i].account)) {
                console.log("Already registered:", participants[i].name, participants[i].account);
                continue;
            }
            registry.registerParticipant(
                participants[i].account,
                participants[i].name,
                participants[i].role,
                keccak256(abi.encodePacked("local_dev_", participants[i].name))
            );
            console.log("Registered:", participants[i].name, participants[i].account);
        }

        vm.stopBroadcast();
    }
}
