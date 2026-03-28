// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {IIdentityRegistry} from "../src/interfaces/IIdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";

/// @title RegisterParticipants
/// @notice Registers bank operator addresses as verified participants in the
///         compliance IdentityRegistry so that on-chain HTLC calls pass the
///         `onlyVerified` modifier.
/// @dev Run per spoke after contract deployment:
///   IDENTITY_REGISTRY=0x42699A7612A82f1d9C36148af9C77354759b210b \
///   forge script script/RegisterParticipants.s.sol:RegisterParticipants \
///     --rpc-url $SPOKE_RPC_URL --broadcast
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
        // Bank-A / Bank-B share key ae6ae8e5… → 0xf17f52…
        // Bank-C / Bank-D share key 5b02fc9a… → 0xe4add9…
        // Central Bank key c87509a1… → 0x627306…
        Participant[3] memory participants = [
            Participant(0x627306090abaB3A6e1400e9345bC60c78a8BEf57, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK),
            Participant(0xf17f52151EbEF6C7334FAD080c5704D77216b732, "Commercial Bank A/B", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK),
            Participant(0xe4add986E80022C0741874841d4ac231B1d7d254, "Commercial Bank C/D", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK)
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
