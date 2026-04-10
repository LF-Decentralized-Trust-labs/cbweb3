// SPDX-License-Identifier: UNLICENSED
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
        // Central Bank key c87509a1… → 0x627306…
        Participant[3] memory participants = [
            Participant(
                0x627306090abaB3A6e1400e9345bC60c78a8BEf57,
                "Central Bank",
                IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK
            ),
            Participant(
                0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef,
                "Commercial Bank A/B",
                IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK
            ),
            Participant(
                0xc110089385bad5026E5083443C3b443806DA42Df,
                "Commercial Bank C/D",
                IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK
            )
        ];

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
