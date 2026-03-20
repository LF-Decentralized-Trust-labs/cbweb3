// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Test, console} from "forge-std/Test.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IIdentityRegistry} from "../src/interfaces/IIdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";

/// @title IdentityRegistryTest
/// @notice Unit tests for the IdentityRegistry contract.
/// @dev Implements tests for onboarding, RBAC, and status management.
contract IdentityRegistryTest is Test {
    /// @dev Local event redeclarations for vm.expectEmit assertions (Forge pattern).
    event ParticipantRegistered(address indexed account, IdentityRegistryLibrary.ParticipantRole role, string name);
    event IdentityUpdated(
        address indexed account,
        IdentityRegistryLibrary.KycStatus oldStatus,
        IdentityRegistryLibrary.KycStatus newStatus
    );

    IdentityRegistry public registry;

    // Test Actors
    address public admin = address(0x1);
    address public bankA = address(0x2);
    address public maliciousUser = address(0x3);

    // Test Data
    string public constant BANK_NAME = "Commercial Bank Alpha";
    bytes32 public constant ZK_POINTER = keccak256("identity_proof_001");

    /// @notice Sets up the test environment by deploying the registry.
    function setUp() public {
        vm.prank(admin);
        registry = new IdentityRegistry(admin);
    }

    /// @notice Verifies that a participant can be registered by an admin.
    function test_RegisterParticipant_Success() public {
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        assertTrue(registry.isWhitelisted(bankA));
        assertTrue(registry.canTransact(bankA));

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(p.legalName, BANK_NAME);
        assertEq(uint256(p.role), uint256(IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK));
    }

    /// @notice Verifies that non-admin addresses cannot register participants.
    /// @dev Tests the AccessControl restriction.
    function test_RegisterParticipant_RevertIf_NotAdmin() public {
        vm.prank(maliciousUser);

        // Expecting AccessControl revert from OpenZeppelin
        vm.expectRevert();
        registry.registerParticipant(
            bankA, "Fake Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
    }

    /// @notice Verifies that an identity can be suspended and blocked from transacting.
    function test_UpdateStatus_Suspension() public {
        // 1. Onboard
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        // 2. Suspend
        vm.prank(admin);
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Suspended);

        // 3. Assertions
        assertFalse(registry.isWhitelisted(bankA));
        assertFalse(registry.canTransact(bankA));

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(uint256(p.status), uint256(IdentityRegistryLibrary.KycStatus.Suspended));
    }

    /// @notice Verifies that the canTransact check works correctly for different roles.
    function test_CanTransact_RoleValidation() public {
        vm.startPrank(admin);

        // Register with role NONE (should not be able to transact)
        registry.registerParticipant(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.NONE, ZK_POINTER);

        assertFalse(registry.canTransact(bankA));

        // Update to Commercial Bank
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        assertTrue(registry.canTransact(bankA));
        vm.stopPrank();
    }

    /// @notice Verifies that registration with address(0) reverts.
    function test_Register_RevertIf_AddressZero() public {
        vm.prank(admin);
        vm.expectRevert(IIdentityRegistry.InvalidIdentityData.selector);
        registry.registerParticipant(
            address(0), "Zero Address Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
    }

    /// @notice Verifies that non-governance addresses cannot update participant status.
    function test_UpdateStatus_RevertIf_NotGovernance() public {
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        vm.prank(maliciousUser);
        vm.expectRevert();
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Suspended);
    }

    /// @notice Verifies that a suspended participant can be re-activated (Suspended → Verified).
    function test_UpdateStatus_Reactivation() public {
        vm.startPrank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Suspended);
        assertFalse(registry.canTransact(bankA));

        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Verified);
        vm.stopPrank();

        assertTrue(registry.isWhitelisted(bankA));
        assertTrue(registry.canTransact(bankA));
    }

    /// @notice Verifies that a participant with KycStatus.Pending cannot transact or be whitelisted.
    function test_CanTransact_PendingStatus() public {
        vm.startPrank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Pending);
        vm.stopPrank();

        assertFalse(registry.isWhitelisted(bankA));
        assertFalse(registry.canTransact(bankA));
    }

    /// @notice Verifies that a participant with KycStatus.Expired cannot transact or be whitelisted.
    function test_CanTransact_ExpiredStatus() public {
        vm.startPrank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Expired);
        vm.stopPrank();

        assertFalse(registry.isWhitelisted(bankA));
        assertFalse(registry.canTransact(bankA));
    }

    /// @notice Verifies that an unregistered address cannot transact and is not whitelisted.
    function test_UnregisteredAddress_CannotTransact() public view {
        assertFalse(registry.isWhitelisted(maliciousUser));
        assertFalse(registry.canTransact(maliciousUser));
    }

    /// @notice Verifies that getParticipant returns zero-value struct for an unregistered address.
    function test_GetParticipant_DefaultValues() public view {
        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(maliciousUser);
        assertEq(p.legalName, "");
        assertEq(uint256(p.role), uint256(IdentityRegistryLibrary.ParticipantRole.NONE));
        assertEq(uint256(p.status), uint256(IdentityRegistryLibrary.KycStatus.None));
        assertEq(p.zkPointer, bytes32(0));
        assertEq(p.lastUpdate, 0);
    }

    /// @notice Verifies that CENTRAL_BANK and LIQUIDITY_PROVIDER roles can transact when Verified.
    function test_CanTransact_AllTransactableRoles() public {
        address centralBankAddr = address(0x4);
        address lpAddr = address(0x5);

        vm.startPrank(admin);
        registry.registerParticipant(
            centralBankAddr, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, ZK_POINTER
        );
        registry.registerParticipant(
            lpAddr, "Liquidity Provider", IdentityRegistryLibrary.ParticipantRole.LIQUIDITY_PROVIDER, ZK_POINTER
        );
        vm.stopPrank();

        assertTrue(registry.canTransact(centralBankAddr));
        assertTrue(registry.canTransact(lpAddr));
    }

    /// @notice Verifies that ParticipantRegistered event is emitted on registration.
    function test_RegisterParticipant_EmitsEvent() public {
        vm.prank(admin);
        vm.expectEmit(true, true, false, true);
        emit ParticipantRegistered(bankA, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, BANK_NAME);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
    }

    /// @notice Verifies that IdentityUpdated event is emitted on status change.
    function test_UpdateStatus_EmitsEvent() public {
        vm.startPrank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        vm.expectEmit(true, false, false, true);
        emit IdentityUpdated(
            bankA, IdentityRegistryLibrary.KycStatus.Verified, IdentityRegistryLibrary.KycStatus.Suspended
        );
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Suspended);
        vm.stopPrank();
    }

    /// @notice Verifies that re-registering an existing participant overwrites their data.
    function test_RegisterParticipant_Overwrite() public {
        vm.startPrank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
        bytes32 newPointer = keccak256("new_proof");
        registry.registerParticipant(
            bankA, "Updated Bank Name", IdentityRegistryLibrary.ParticipantRole.LIQUIDITY_PROVIDER, newPointer
        );
        vm.stopPrank();

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(p.legalName, "Updated Bank Name");
        assertEq(uint256(p.role), uint256(IdentityRegistryLibrary.ParticipantRole.LIQUIDITY_PROVIDER));
        assertEq(p.zkPointer, newPointer);
        assertEq(uint256(p.status), uint256(IdentityRegistryLibrary.KycStatus.Verified));
    }

    /// @notice Verifies that lastUpdate is set to block.timestamp on registerParticipant.
    function test_RegisterParticipant_SetsLastUpdateTimestamp() public {
        uint256 ts = 1_700_000_000;
        vm.warp(ts);

        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(p.lastUpdate, ts);
    }

    /// @notice Verifies that lastUpdate is refreshed on updateStatus.
    function test_UpdateStatus_RefreshesLastUpdateTimestamp() public {
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        uint256 laterTs = 1_800_000_000;
        vm.warp(laterTs);
        vm.prank(admin);
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Suspended);

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(p.lastUpdate, laterTs);
    }

    /// @notice Verifies that the constructor grants both DEFAULT_ADMIN_ROLE and GOVERNANCE_ROLE to admin.
    function test_Constructor_AdminRoles() public view {
        assertTrue(registry.hasRole(registry.DEFAULT_ADMIN_ROLE(), admin));
        assertTrue(registry.hasRole(registry.GOVERNANCE_ROLE(), admin));
    }
}
