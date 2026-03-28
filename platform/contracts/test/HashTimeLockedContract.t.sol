// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {IHashTimeLockedContract} from "../src/interfaces/IHashTimeLockedContract.sol";
import {HashTimeLockedContractLibrary} from "../src/libraries/HashTimeLockedContractLibrary.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {DeployHTLC} from "../script/HashTimeLockedContract.s.sol";

contract HashTimeLockedContractTest is Test {
    event LogHTLCLocked(
        bytes32 indexed contractId,
        address indexed sender,
        address indexed receiver,
        bytes32 hashLock,
        uint256 timeLock,
        bytes32 zetoLockRef
    );
    event LogHTLCClaimed(bytes32 indexed contractId, bytes32 secret);
    event LogHTLCRefunded(bytes32 indexed contractId);

    HashTimeLockedContract public htlc;
    IdentityRegistry public identityRegistry;

    address public admin = makeAddr("admin");
    address public sender = makeAddr("sender");
    address public receiver = makeAddr("receiver");

    bytes32 public contractId = keccak256("FX_AGREEMENT_001");
    bytes32 public secret = "super_secure_secret_preimage";
    bytes32 public hashLock = sha256(abi.encodePacked(secret));
    bytes32 public zetoLockRef = keccak256("ZETO_LOCK_TX_001");
    uint256 public timeLock;

    function setUp() public {
        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            sender, "Commercial Bank A", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            receiver, "Commercial Bank B", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        vm.stopPrank();

        htlc = new HashTimeLockedContract(address(identityRegistry));
        timeLock = block.timestamp + 1 hours;
    }

    function test_Lock_Success() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef);

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(contractId);

        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.LOCKED));
        assertEq(details.sender, sender);
        assertEq(details.receiver, receiver);
        assertEq(details.hashLock, hashLock);
        assertEq(details.timeLock, timeLock);
        assertEq(details.zetoLockRef, zetoLockRef);
    }

    function test_Lock_EmitsEvent() public {
        vm.expectEmit(true, true, true, true);
        emit LogHTLCLocked(contractId, sender, receiver, hashLock, timeLock, zetoLockRef);

        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef);
    }

    function test_Settle_Success() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef);

        htlc.settle(contractId, secret);

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(contractId);

        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.SETTLED));
        assertEq(details.secret, secret);
    }

    function test_Settle_EmitsEvent() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef);

        vm.expectEmit(true, true, true, true);
        emit LogHTLCClaimed(contractId, secret);

        htlc.settle(contractId, secret);
    }

    function test_Refund_Success() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef);

        vm.warp(timeLock + 1 seconds);

        vm.prank(sender);
        htlc.refund(contractId);

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(contractId);

        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.REFUNDED));
    }

    function test_Refund_EmitsEvent() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef);

        vm.warp(timeLock + 1 seconds);

        vm.expectEmit(true, true, true, true);
        emit LogHTLCRefunded(contractId);

        vm.prank(sender);
        htlc.refund(contractId);
    }

    function test_Revert_Lock_AlreadyExists() public {
        vm.startPrank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef);

        vm.expectRevert(IHashTimeLockedContract.HTLC__ContractAlreadyExists.selector);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef);
        vm.stopPrank();
    }

    function test_Revert_Lock_TimeLockExpired() public {
        bytes32 newContractId = keccak256("FX_AGREEMENT_EXPIRED_TIMELOCK");
        uint256 expiredTimeLock = block.timestamp;

        vm.prank(sender);
        vm.expectRevert(IHashTimeLockedContract.HTLC__TimeLockExpired.selector);
        htlc.lock(newContractId, receiver, hashLock, expiredTimeLock, zetoLockRef);
    }

    function test_Revert_Settle_InvalidSecret() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef);

        bytes32 wrongSecret = "wrong_secret";

        vm.expectRevert(IHashTimeLockedContract.HTLC__InvalidSecret.selector);
        htlc.settle(contractId, wrongSecret);
    }

    function test_Revert_Settle_ContractNotLocked() public {
        bytes32 nonExistingContractId = keccak256("FX_AGREEMENT_NON_EXISTING_SETTLE");

        vm.expectRevert(IHashTimeLockedContract.HTLC__ContractNotLocked.selector);
        htlc.settle(nonExistingContractId, secret);
    }

    function test_Revert_Refund_TimeLockNotExpired() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef);

        vm.expectRevert(IHashTimeLockedContract.HTLC__TimeLockNotExpired.selector);
        htlc.refund(contractId);
    }

    function test_Revert_Refund_ContractNotLocked() public {
        bytes32 nonExistingContractId = keccak256("FX_AGREEMENT_NON_EXISTING_REFUND");

        vm.expectRevert(IHashTimeLockedContract.HTLC__ContractNotLocked.selector);
        htlc.refund(nonExistingContractId);
    }

    function test_Revert_Lock_UnverifiedSender() public {
        address unverified = makeAddr("unverifiedSender");
        bytes32 newContractId = keccak256("FX_AGREEMENT_UNVERIFIED_SENDER");

        vm.prank(unverified);
        vm.expectRevert(
            abi.encodeWithSelector(IHashTimeLockedContract.HTLC__ParticipantNotVerified.selector, unverified)
        );
        htlc.lock(newContractId, receiver, hashLock, timeLock, zetoLockRef);
    }

    function test_Revert_Lock_UnverifiedReceiver() public {
        address unverifiedReceiver = makeAddr("unverifiedReceiver");
        bytes32 newContractId = keccak256("FX_AGREEMENT_UNVERIFIED_RECEIVER");

        vm.prank(sender);
        vm.expectRevert(
            abi.encodeWithSelector(IHashTimeLockedContract.HTLC__ParticipantNotVerified.selector, unverifiedReceiver)
        );
        htlc.lock(newContractId, unverifiedReceiver, hashLock, timeLock, zetoLockRef);
    }

    function test_GetLockDetails_NonExisting() public view {
        bytes32 unknownContractId = keccak256("UNKNOWN_CONTRACT_ID");
        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(unknownContractId);

        assertEq(details.sender, address(0));
        assertEq(details.receiver, address(0));
        assertEq(details.hashLock, bytes32(0));
        assertEq(details.timeLock, 0);
        assertEq(details.secret, bytes32(0));
        assertEq(details.zetoLockRef, bytes32(0));
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.INVALID));
    }
}

contract DeployHTLCTest is Test {
    DeployHTLC public deployScript;

    uint256 private deployerPrivateKey;
    address private expectedDeployer;

    string private constant ENV_DEPLOYER_PRIVATE_KEY = "DEPLOYER_PRIVATE_KEY";
    string private constant ENV_IDENTITY_REGISTRY_ADDRESS = "IDENTITY_REGISTRY_ADDRESS";

    function setUp() public {
        deployScript = new DeployHTLC();
        deployScript.setUp();

        deployerPrivateKey = vm.envOr(ENV_DEPLOYER_PRIVATE_KEY, uint256(0x1));
        expectedDeployer = vm.addr(deployerPrivateKey);

        vm.setEnv(ENV_DEPLOYER_PRIVATE_KEY, vm.toString(deployerPrivateKey));
        vm.setEnv(ENV_IDENTITY_REGISTRY_ADDRESS, vm.toString(address(0x6789012345678901234567890123456789012345)));
    }

    function test_ScriptRun_Success() public {
        deployScript.run();
        HashTimeLockedContract htlc = deployScript.htlc();

        assertTrue(address(htlc) != address(0), "Contract was not deployed");
        assertGt(address(htlc).code.length, 0, "Deployed contract has no runtime bytecode");

        bytes32 unknownContractId = keccak256("UNKNOWN_CONTRACT_ID");
        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(unknownContractId);

        assertEq(details.sender, address(0), "Default sender should be zero address");
        assertEq(details.receiver, address(0), "Default receiver should be zero address");
        assertEq(details.hashLock, bytes32(0), "Default hashLock should be zero");
        assertEq(details.timeLock, 0, "Default timeLock should be zero");
        assertEq(details.secret, bytes32(0), "Default secret should be zero");
        assertEq(details.zetoLockRef, bytes32(0), "Default zetoLockRef should be zero");
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.INVALID));

        assertTrue(expectedDeployer != address(0), "Expected deployer should not be zero address");
    }
}
