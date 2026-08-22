// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test, console} from "forge-std/Test.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IIdentityRegistry} from "../src/interfaces/IIdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";

/// @title IdentityRegistryTest
/// @notice Unit tests for the IdentityRegistry contract.
/// @dev Implements tests for the two-step onboarding (Pending -> Verified), RBAC,
///      and status management.
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
    address public registrar = address(0x20); // holds GOVERNANCE_ROLE only
    address public verifier = address(0x21); // holds VERIFIER_ROLE only

    // Test Data
    string public constant BANK_NAME = "Commercial Bank Alpha";
    bytes32 public constant ZK_POINTER = keccak256("identity_proof_001");

    /// @notice Sets up the test environment by deploying the registry.
    function setUp() public {
        vm.prank(admin);
        registry = new IdentityRegistry(admin);
    }

    /// @dev Helper: registers then verifies `account` as `admin` (who holds both roles).
    function _registerAndVerify(address account, string memory name, IdentityRegistryLibrary.ParticipantRole role)
        internal
    {
        vm.startPrank(admin);
        registry.registerParticipant(account, name, role, ZK_POINTER);
        registry.verifyParticipant(account);
        vm.stopPrank();
    }

    // =========================================================================
    //                      TWO-STEP ONBOARDING (R1-10.6)
    // =========================================================================

    /// @notice registerParticipant MUST create the participant in Pending, not Verified.
    /// @dev A single governance call must not produce a transactable participant.
    function test_RegisterParticipant_CreatesPending() public {
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(uint256(p.status), uint256(IdentityRegistryLibrary.KycStatus.Pending));

        // A Pending participant is neither whitelisted nor able to transact.
        assertFalse(registry.isWhitelisted(bankA));
        assertFalse(registry.canTransact(bankA));
    }

    /// @notice Only verifyParticipant may move a participant Pending -> Verified.
    function test_VerifyParticipant_MovesPendingToVerified() public {
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
        assertFalse(registry.canTransact(bankA));

        vm.prank(admin);
        registry.verifyParticipant(bankA);

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(uint256(p.status), uint256(IdentityRegistryLibrary.KycStatus.Verified));
        assertTrue(registry.isWhitelisted(bankA));
        assertTrue(registry.canTransact(bankA));
    }

    /// @notice verifyParticipant emits IdentityUpdated(Pending -> Verified).
    function test_VerifyParticipant_EmitsIdentityUpdated() public {
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        vm.expectEmit(true, false, false, true);
        emit IdentityUpdated(
            bankA, IdentityRegistryLibrary.KycStatus.Pending, IdentityRegistryLibrary.KycStatus.Verified
        );
        vm.prank(admin);
        registry.verifyParticipant(bankA);
    }

    /// @notice verifyParticipant reverts when the participant is not Pending (e.g., never
    ///         registered, or already Verified) — enforcing the state machine.
    function test_VerifyParticipant_RevertIf_NotPending() public {
        // Never registered => status None.
        vm.prank(admin);
        vm.expectRevert(abi.encodeWithSelector(IIdentityRegistry.ParticipantNotPending.selector, bankA));
        registry.verifyParticipant(bankA);

        // Register + verify once, then a second verify must revert (no longer Pending).
        _registerAndVerify(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);
        vm.prank(admin);
        vm.expectRevert(abi.encodeWithSelector(IIdentityRegistry.ParticipantNotPending.selector, bankA));
        registry.verifyParticipant(bankA);
    }

    /// @notice verifyParticipant is restricted to VERIFIER_ROLE holders.
    function test_VerifyParticipant_RevertIf_NotVerifier() public {
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        vm.prank(maliciousUser);
        vm.expectRevert();
        registry.verifyParticipant(bankA);
    }

    /// @notice Segregation of duties: a registrar (GOVERNANCE_ROLE only) cannot verify, and a
    ///         verifier (VERIFIER_ROLE only) cannot register. Registrar != verifier.
    function test_TwoStep_SeparationOfDuties() public {
        vm.startPrank(admin);
        registry.grantRole(registry.GOVERNANCE_ROLE(), registrar);
        registry.grantRole(registry.VERIFIER_ROLE(), verifier);
        vm.stopPrank();

        // Registrar can register (Pending) but cannot verify.
        vm.prank(registrar);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
        assertFalse(registry.canTransact(bankA));

        vm.prank(registrar);
        vm.expectRevert();
        registry.verifyParticipant(bankA);

        // Verifier cannot register.
        vm.prank(verifier);
        vm.expectRevert();
        registry.registerParticipant(
            maliciousUser, "X", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        // Verifier can move the registrar-created participant to Verified.
        vm.prank(verifier);
        registry.verifyParticipant(bankA);
        assertTrue(registry.canTransact(bankA));
    }

    /// @notice verifyParticipant MUST NOT write block.timestamp into lastUpdate (Pente determinism).
    /// @dev block.timestamp is nondeterministic across Pente endorsers (see registerParticipant).
    function test_VerifyParticipant_DoesNotStoreBlockTimestamp() public {
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        vm.warp(1_900_000_000);
        vm.prank(admin);
        registry.verifyParticipant(bankA);

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(p.lastUpdate, 0);
    }

    // =========================================================================
    //                            REGISTRATION / RBAC
    // =========================================================================

    /// @notice Verifies that a participant can be registered and verified by an admin.
    function test_RegisterParticipant_Success() public {
        _registerAndVerify(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);

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
        // 1. Onboard (register + verify)
        _registerAndVerify(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);
        assertTrue(registry.canTransact(bankA));

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

        // Register with role NONE and verify: still cannot transact (role gate).
        registry.registerParticipant(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.NONE, ZK_POINTER);
        registry.verifyParticipant(bankA);
        assertFalse(registry.canTransact(bankA));

        // Re-register as Commercial Bank (resets to Pending) then verify.
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );
        registry.verifyParticipant(bankA);
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
        _registerAndVerify(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);

        vm.prank(maliciousUser);
        vm.expectRevert();
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Suspended);
    }

    /// @notice Verifies that a suspended participant can be re-activated (Suspended -> Verified).
    function test_UpdateStatus_Reactivation() public {
        _registerAndVerify(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);

        vm.prank(admin);
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Suspended);
        assertFalse(registry.canTransact(bankA));

        vm.prank(admin);
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Verified);

        assertTrue(registry.isWhitelisted(bankA));
        assertTrue(registry.canTransact(bankA));
    }

    /// @notice Regression (R1-10.6 / R2-10.6): a GOVERNANCE_ROLE holder that is NOT a verifier
    ///         cannot mint Verified through updateStatus — that bypass would defeat the two-step
    ///         onboarding control. Promotion to Verified requires VERIFIER_ROLE.
    function test_UpdateStatus_RevertIf_GovernanceMintsVerified() public {
        vm.startPrank(admin);
        registry.grantRole(registry.GOVERNANCE_ROLE(), registrar);
        registry.grantRole(registry.VERIFIER_ROLE(), verifier);
        vm.stopPrank();

        // registrar (GOVERNANCE_ROLE only) can register (-> Pending) but cannot promote to Verified.
        vm.prank(registrar);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        vm.prank(registrar);
        vm.expectRevert();
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Verified);

        // Sanity: still not transactable.
        assertFalse(registry.canTransact(bankA));

        // A VERIFIER_ROLE holder may promote via updateStatus (mirrors verifyParticipant).
        vm.prank(verifier);
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Verified);
        assertTrue(registry.canTransact(bankA));
    }

    /// @notice Verifies that a participant left in Pending cannot transact or be whitelisted.
    function test_CanTransact_PendingStatus() public {
        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        assertFalse(registry.isWhitelisted(bankA));
        assertFalse(registry.canTransact(bankA));
    }

    /// @notice Verifies that a participant with KycStatus.Expired cannot transact or be whitelisted.
    function test_CanTransact_ExpiredStatus() public {
        _registerAndVerify(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);
        vm.prank(admin);
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Expired);

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

    /// @notice Verifies that CENTRAL_BANK and COMMERCIAL_BANK roles can transact when Verified.
    function test_CanTransact_AllTransactableRoles() public {
        address centralBankAddr = address(0x4);
        address lpAddr = address(0x5);

        _registerAndVerify(centralBankAddr, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK);
        _registerAndVerify(lpAddr, "Liquidity Provider", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);

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
        _registerAndVerify(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);

        vm.expectEmit(true, false, false, true);
        emit IdentityUpdated(
            bankA, IdentityRegistryLibrary.KycStatus.Verified, IdentityRegistryLibrary.KycStatus.Suspended
        );
        vm.prank(admin);
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Suspended);
    }

    /// @notice Verifies that re-registering an existing participant resets them to Pending.
    /// @dev Re-registration re-opens verification: a previously Verified participant must be
    ///      re-verified after their profile is rewritten.
    function test_RegisterParticipant_Overwrite() public {
        _registerAndVerify(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);

        bytes32 newPointer = keccak256("new_proof");
        vm.prank(admin);
        registry.registerParticipant(
            bankA, "Updated Bank Name", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, newPointer
        );

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(p.legalName, "Updated Bank Name");
        assertEq(uint256(p.role), uint256(IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK));
        assertEq(p.zkPointer, newPointer);
        assertEq(uint256(p.status), uint256(IdentityRegistryLibrary.KycStatus.Pending));
        assertFalse(registry.canTransact(bankA));
    }

    /// @notice Verifies that lastUpdate is NOT set to block.timestamp on registerParticipant.
    /// @dev block.timestamp is nondeterministic across Pente endorsers (it derives from the
    /// executor's base block), so writing it into endorsed state wedges the private tx. The
    /// field is kept at 0 for ABI stability; audit time is sourced from the emitted event.
    function test_RegisterParticipant_DoesNotStoreBlockTimestamp() public {
        vm.warp(1_700_000_000);

        vm.prank(admin);
        registry.registerParticipant(
            bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, ZK_POINTER
        );

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(p.lastUpdate, 0);
    }

    /// @notice Verifies that updateStatus does NOT write block.timestamp into lastUpdate.
    function test_UpdateStatus_DoesNotStoreBlockTimestamp() public {
        _registerAndVerify(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);

        vm.warp(1_800_000_000);
        vm.prank(admin);
        registry.updateStatus(bankA, IdentityRegistryLibrary.KycStatus.Suspended);

        IdentityRegistryLibrary.Participant memory p = registry.getParticipant(bankA);
        assertEq(p.lastUpdate, 0);
    }

    /// @notice Verifies that canGovern returns true for CENTRAL_BANK and GOVERNANCE roles when Verified.
    function test_CanGovern_GovernanceRoles() public {
        address centralBankAddr = address(0x10);
        address governanceAddr = address(0x11);

        _registerAndVerify(centralBankAddr, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK);
        _registerAndVerify(governanceAddr, "Governance Entity", IdentityRegistryLibrary.ParticipantRole.GOVERNANCE);

        assertTrue(registry.canGovern(centralBankAddr));
        assertTrue(registry.canGovern(governanceAddr));
    }

    /// @notice Verifies that canGovern returns false for non-governance roles.
    function test_CanGovern_ReturnsFalseForNonGovernanceRoles() public {
        _registerAndVerify(bankA, BANK_NAME, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK);

        assertFalse(registry.canGovern(bankA));
    }

    /// @notice Verifies that canGovern returns false for suspended governance participants.
    function test_CanGovern_ReturnsFalseWhenSuspended() public {
        address centralBankAddr = address(0x10);

        _registerAndVerify(centralBankAddr, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK);
        vm.prank(admin);
        registry.updateStatus(centralBankAddr, IdentityRegistryLibrary.KycStatus.Suspended);

        assertFalse(registry.canGovern(centralBankAddr));
    }

    /// @notice Verifies that canGovern returns false for unregistered addresses.
    function test_CanGovern_ReturnsFalseForUnregistered() public view {
        assertFalse(registry.canGovern(maliciousUser));
    }

    /// @notice Verifies that the constructor grants DEFAULT_ADMIN_ROLE, GOVERNANCE_ROLE and VERIFIER_ROLE to admin.
    function test_Constructor_AdminRoles() public view {
        assertTrue(registry.hasRole(registry.DEFAULT_ADMIN_ROLE(), admin));
        assertTrue(registry.hasRole(registry.GOVERNANCE_ROLE(), admin));
        assertTrue(registry.hasRole(registry.VERIFIER_ROLE(), admin));
    }
}
