// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";

/// @title IdentityRegistryCertTest
/// @notice Tests for certificate fingerprint storage on IdentityRegistry.
contract IdentityRegistryCertTest is Test {
    event CertificateRegistered(address indexed account, bytes32 certFingerprint);

    IdentityRegistry public registry;

    address public admin = address(0x1);
    address public bankA = address(0x2);
    address public maliciousUser = address(0x3);

    string public constant BANK_NAME = "Commercial Bank Alpha";
    bytes32 public constant ZK_POINTER = keccak256("identity_proof_001");

    function setUp() public {
        vm.prank(admin);
        registry = new IdentityRegistry(admin);
    }

    function test_setCertFingerprint_success() public {
        bytes32 fingerprint = keccak256("cert_der_bytes");

        vm.startPrank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
        registry.setCertFingerprint(bankA, fingerprint);
        vm.stopPrank();

        assertEq(registry.getCertFingerprint(bankA), fingerprint);
    }

    function test_setCertFingerprint_emitsEvent() public {
        bytes32 fingerprint = keccak256("x509_fingerprint");

        vm.startPrank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        vm.expectEmit(true, false, false, true);
        emit CertificateRegistered(bankA, fingerprint);
        registry.setCertFingerprint(bankA, fingerprint);
        vm.stopPrank();
    }

    function test_setCertFingerprint_unauthorized() public {
        bytes32 fingerprint = keccak256("unauthorized");

        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        vm.prank(maliciousUser);
        vm.expectRevert();
        registry.setCertFingerprint(bankA, fingerprint);
    }

    function test_getCertFingerprint_default() public {
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        assertEq(registry.getCertFingerprint(bankA), bytes32(0));
    }

    function test_setCertFingerprint_update() public {
        bytes32 first = keccak256("first_cert");
        bytes32 second = keccak256("second_cert");

        vm.startPrank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
        registry.setCertFingerprint(bankA, first);
        assertEq(registry.getCertFingerprint(bankA), first);

        registry.setCertFingerprint(bankA, second);
        vm.stopPrank();

        assertEq(registry.getCertFingerprint(bankA), second);
    }
}
