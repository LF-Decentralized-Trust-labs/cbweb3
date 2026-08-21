// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {IHashTimeLockedContract} from "../src/interfaces/IHashTimeLockedContract.sol";
import {HashTimeLockedContractLibrary} from "../src/libraries/HashTimeLockedContractLibrary.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {FXAgreement} from "../src/FXAgreement.sol";
import {IFXAgreement} from "../src/interfaces/IFXAgreement.sol";
import {FXAgreementLibrary} from "../src/libraries/FXAgreementLibrary.sol";
import {DeployHTLC} from "../script/HashTimeLockedContract.s.sol";
import {CommitmentHashRegistry} from "../src/CommitmentHashRegistry.sol";

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
    FXAgreement public fxAgreement;

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
            sender,
            "Commercial Bank A",
            IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK,
            bytes32(0),
            bytes32("inst-sender")
        );
        identityRegistry.verifyParticipant(sender);
        identityRegistry.registerParticipant(
            receiver,
            "Commercial Bank B",
            IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK,
            bytes32(0),
            bytes32("inst-receiver")
        );
        identityRegistry.verifyParticipant(receiver);
        vm.stopPrank();

        fxAgreement = new FXAgreement(address(identityRegistry));
        htlc = new HashTimeLockedContract(address(identityRegistry), address(fxAgreement), address(0));
        timeLock = block.timestamp + 1 hours;
    }

    function test_Lock_Success() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

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
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));
    }

    function test_Settle_Success() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

        htlc.settle(contractId, secret);

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(contractId);

        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.SETTLED));
        assertEq(details.secret, secret);
    }

    function test_Settle_EmitsEvent() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

        vm.expectEmit(true, true, true, true);
        emit LogHTLCClaimed(contractId, secret);

        htlc.settle(contractId, secret);
    }

    function test_Refund_Success() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

        vm.warp(timeLock + 1 seconds);

        vm.prank(sender);
        htlc.refund(contractId);

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(contractId);

        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.REFUNDED));
    }

    function test_Refund_EmitsEvent() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

        vm.warp(timeLock + 1 seconds);

        vm.expectEmit(true, true, true, true);
        emit LogHTLCRefunded(contractId);

        vm.prank(sender);
        htlc.refund(contractId);
    }

    function test_Revert_Lock_AlreadyExists() public {
        vm.startPrank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

        vm.expectRevert(IHashTimeLockedContract.HTLC__ContractAlreadyExists.selector);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));
        vm.stopPrank();
    }

    function test_Revert_Lock_TimeLockExpired() public {
        bytes32 newContractId = keccak256("FX_AGREEMENT_EXPIRED_TIMELOCK");
        uint256 expiredTimeLock = block.timestamp;

        vm.prank(sender);
        vm.expectRevert(IHashTimeLockedContract.HTLC__TimeLockExpired.selector);
        htlc.lock(newContractId, receiver, hashLock, expiredTimeLock, zetoLockRef, bytes32(0));
    }

    function test_Revert_Settle_InvalidSecret() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

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
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

        vm.expectRevert(IHashTimeLockedContract.HTLC__TimeLockNotExpired.selector);
        htlc.refund(contractId);
    }

    function test_Revert_Refund_ContractNotLocked() public {
        bytes32 nonExistingContractId = keccak256("FX_AGREEMENT_NON_EXISTING_REFUND");

        vm.expectRevert(IHashTimeLockedContract.HTLC__ContractNotLocked.selector);
        htlc.refund(nonExistingContractId);
    }

    function test_Revert_Refund_NotSender() public {
        // R2-H-1: only the original lock sender may trigger the refund. A third
        // party calling refund() after expiry must revert, otherwise the public
        // coordination layer can be desynchronized from the private Zeto layer.
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

        vm.warp(timeLock + 1 seconds);

        address attacker = makeAddr("refundAttacker");
        vm.prank(attacker);
        vm.expectRevert(abi.encodeWithSelector(IHashTimeLockedContract.HTLC__NotSender.selector, attacker));
        htlc.refund(contractId);

        // Control: the original sender can still refund after the third-party attempt.
        vm.prank(sender);
        htlc.refund(contractId);

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(contractId);
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.REFUNDED));
    }

    function test_Revert_Lock_UnverifiedSender() public {
        address unverified = makeAddr("unverifiedSender");
        bytes32 newContractId = keccak256("FX_AGREEMENT_UNVERIFIED_SENDER");

        vm.prank(unverified);
        vm.expectRevert(
            abi.encodeWithSelector(IHashTimeLockedContract.HTLC__ParticipantNotVerified.selector, unverified)
        );
        htlc.lock(newContractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));
    }

    function test_Revert_Lock_UnverifiedReceiver() public {
        address unverifiedReceiver = makeAddr("unverifiedReceiver");
        bytes32 newContractId = keccak256("FX_AGREEMENT_UNVERIFIED_RECEIVER");

        vm.prank(sender);
        vm.expectRevert(
            abi.encodeWithSelector(IHashTimeLockedContract.HTLC__ParticipantNotVerified.selector, unverifiedReceiver)
        );
        htlc.lock(newContractId, unverifiedReceiver, hashLock, timeLock, zetoLockRef, bytes32(0));
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

    function test_Lock_WithAcceptedAgreement_Success() public {
        // Register sender as centralBank role to allow governance for proposeOnBehalf
        // Instead, have sender propose directly and counterpartyB accept
        bytes32 fxTradeId = keccak256("FX_HTLC_TEST_001");
        uint256 fxExpiry = block.timestamp + 1 days;

        vm.prank(sender);
        fxAgreement.propose(
            fxTradeId,
            receiver,
            address(0),
            address(0),
            address(0),
            1e18,
            1e18,
            bytes32("BRL"),
            bytes32("EUR"),
            5e18,
            fxExpiry,
            FXAgreementLibrary.Routing({
                sourceSpokeId: "",
                destSpokeId: "",
                originatorId: "",
                counterpartyId: "",
                settlementAgentId: "",
                custodianId: "",
                beneficiaryId: "",
                sourceReceiverId: "",
                destReceiverId: "",
                tradeRef: ""
            })
        );

        vm.prank(receiver);
        fxAgreement.accept(fxTradeId);

        vm.prank(sender);
        bytes32 newContractId = keccak256("HTLC_WITH_FX_001");
        htlc.lock(newContractId, receiver, hashLock, timeLock, zetoLockRef, fxTradeId);

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(newContractId);
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.LOCKED));
    }

    function test_Revert_Lock_AgreementNotAccepted() public {
        bytes32 fxTradeId = keccak256("FX_HTLC_TEST_002");
        uint256 fxExpiry = block.timestamp + 1 days;

        vm.prank(sender);
        fxAgreement.propose(
            fxTradeId,
            receiver,
            address(0),
            address(0),
            address(0),
            1e18,
            1e18,
            bytes32("BRL"),
            bytes32("EUR"),
            5e18,
            fxExpiry,
            FXAgreementLibrary.Routing({
                sourceSpokeId: "",
                destSpokeId: "",
                originatorId: "",
                counterpartyId: "",
                settlementAgentId: "",
                custodianId: "",
                beneficiaryId: "",
                sourceReceiverId: "",
                destReceiverId: "",
                tradeRef: ""
            })
        );

        vm.prank(sender);
        bytes32 newContractId = keccak256("HTLC_WITH_FX_002");
        vm.expectRevert(IHashTimeLockedContract.HTLC__AgreementNotAccepted.selector);
        htlc.lock(newContractId, receiver, hashLock, timeLock, zetoLockRef, fxTradeId);
    }

    function test_Revert_Lock_AgreementExpired() public {
        bytes32 fxTradeId = keccak256("FX_HTLC_TEST_003");
        uint256 fxExpiry = block.timestamp + 1 hours;

        vm.prank(sender);
        fxAgreement.propose(
            fxTradeId,
            receiver,
            address(0),
            address(0),
            address(0),
            1e18,
            1e18,
            bytes32("BRL"),
            bytes32("EUR"),
            5e18,
            fxExpiry,
            FXAgreementLibrary.Routing({
                sourceSpokeId: "",
                destSpokeId: "",
                originatorId: "",
                counterpartyId: "",
                settlementAgentId: "",
                custodianId: "",
                beneficiaryId: "",
                sourceReceiverId: "",
                destReceiverId: "",
                tradeRef: ""
            })
        );

        vm.prank(receiver);
        fxAgreement.accept(fxTradeId);

        // Warp past FX agreement expiry but keep HTLC timeLock valid
        vm.warp(fxExpiry + 1);
        uint256 newTimeLock = block.timestamp + 1 hours;

        vm.prank(sender);
        bytes32 newContractId = keccak256("HTLC_WITH_FX_003");
        vm.expectRevert(IHashTimeLockedContract.HTLC__AgreementExpired.selector);
        htlc.lock(newContractId, receiver, hashLock, newTimeLock, zetoLockRef, fxTradeId);
    }

    function test_Lock_ZeroAgreementId_Success() public {
        vm.prank(sender);
        bytes32 newContractId = keccak256("HTLC_ZERO_AGREEMENT");
        htlc.lock(newContractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(newContractId);
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.LOCKED));
    }

    function test_Lock_NoFXAgreement_Success() public {
        HashTimeLockedContract htlcNoFx = new HashTimeLockedContract(address(identityRegistry), address(0), address(0));

        vm.prank(sender);
        bytes32 newContractId = keccak256("HTLC_NO_FX");
        htlcNoFx.lock(newContractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));

        HashTimeLockedContractLibrary.LockDetails memory details = htlcNoFx.getLockDetails(newContractId);
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.LOCKED));
    }

    // --- double-spend / re-state reverts on settle & refund ---

    function test_Revert_Settle_AlreadySettled() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));
        htlc.settle(contractId, secret);

        // second settle: state is SETTLED, not LOCKED
        vm.expectRevert(IHashTimeLockedContract.HTLC__ContractNotLocked.selector);
        htlc.settle(contractId, secret);
    }

    function test_Revert_Refund_AlreadySettled() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));
        htlc.settle(contractId, secret);

        vm.warp(timeLock + 1);
        // refund after settle: state SETTLED, not LOCKED
        vm.expectRevert(IHashTimeLockedContract.HTLC__ContractNotLocked.selector);
        htlc.refund(contractId);
    }

    function test_Revert_Settle_AfterRefund() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, hashLock, timeLock, zetoLockRef, bytes32(0));
        vm.warp(timeLock + 1);
        vm.prank(sender);
        htlc.refund(contractId);

        // settle after refund: state REFUNDED, not LOCKED
        vm.expectRevert(IHashTimeLockedContract.HTLC__ContractNotLocked.selector);
        htlc.settle(contractId, secret);
    }

    // --- setCommitmentHashRegistry auth branches ---

    function test_Revert_SetCommitmentHashRegistry_Unauthorized() public {
        vm.prank(sender); // not governance
        vm.expectRevert(abi.encodeWithSelector(IHashTimeLockedContract.HTLC__ParticipantNotVerified.selector, sender));
        htlc.setCommitmentHashRegistry(address(0x1234));
    }

    function test_SetCommitmentHashRegistry_Success() public {
        address centralBank = makeAddr("htlcCentralBank");
        vm.prank(admin);
        identityRegistry.registerParticipant(
            centralBank,
            "Central Bank",
            IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK,
            bytes32(0),
            bytes32("inst-centralBank")
        );
        vm.prank(admin);
        identityRegistry.verifyParticipant(centralBank);

        CommitmentHashRegistry reg = new CommitmentHashRegistry(address(identityRegistry));
        vm.prank(centralBank);
        htlc.setCommitmentHashRegistry(address(reg));

        assertEq(address(htlc.COMMITMENT_HASH_REGISTRY()), address(reg));
    }

    // --- CommitmentHashRegistry fallback gate (FX_AGREEMENT == address(0)) ---

    function _deployHtlcWithCommitmentRegistry()
        internal
        returns (HashTimeLockedContract htlcFb, CommitmentHashRegistry reg, address centralBank)
    {
        centralBank = makeAddr("fbCentralBank");
        vm.prank(admin);
        identityRegistry.registerParticipant(
            centralBank,
            "Central Bank",
            IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK,
            bytes32(0),
            bytes32("inst-centralBank")
        );
        vm.prank(admin);
        identityRegistry.verifyParticipant(centralBank);
        reg = new CommitmentHashRegistry(address(identityRegistry));
        htlcFb = new HashTimeLockedContract(address(identityRegistry), address(0), address(reg));
    }

    function test_Lock_CommitmentFallback_Accepted_Success() public {
        (HashTimeLockedContract htlcFb, CommitmentHashRegistry reg, address centralBank) =
            _deployHtlcWithCommitmentRegistry();

        bytes32 fbTradeId = keccak256("FB_TRADE_001");
        uint256 oAmt = 1e18;
        uint256 cAmt = 5e18;
        uint256 r = 5e18;
        bytes32 commitmentHash = keccak256(abi.encodePacked(fbTradeId, oAmt, cAmt, r));

        vm.startPrank(centralBank);
        reg.registerCommitment(fbTradeId, sender, receiver, oAmt, cAmt, r);
        reg.acceptCommitment(commitmentHash);
        vm.stopPrank();

        vm.prank(sender);
        bytes32 newContractId = keccak256("HTLC_FB_OK");
        htlcFb.lock(newContractId, receiver, hashLock, timeLock, zetoLockRef, commitmentHash);

        HashTimeLockedContractLibrary.LockDetails memory details = htlcFb.getLockDetails(newContractId);
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.LOCKED));
    }

    function test_Revert_Lock_CommitmentFallback_NotAccepted() public {
        (HashTimeLockedContract htlcFb, CommitmentHashRegistry reg, address centralBank) =
            _deployHtlcWithCommitmentRegistry();

        bytes32 fbTradeId = keccak256("FB_TRADE_002");
        uint256 oAmt = 1e18;
        uint256 cAmt = 5e18;
        uint256 r = 5e18;
        bytes32 commitmentHash = keccak256(abi.encodePacked(fbTradeId, oAmt, cAmt, r));

        // registered but only PENDING (not accepted)
        vm.prank(centralBank);
        reg.registerCommitment(fbTradeId, sender, receiver, oAmt, cAmt, r);

        vm.prank(sender);
        bytes32 newContractId = keccak256("HTLC_FB_FAIL");
        vm.expectRevert(IHashTimeLockedContract.HTLC__CommitmentNotAccepted.selector);
        htlcFb.lock(newContractId, receiver, hashLock, timeLock, zetoLockRef, commitmentHash);
    }
}

contract DeployHTLCTest is Test {
    DeployHTLC public deployScript;

    uint256 private deployerPrivateKey;
    address private expectedDeployer;

    string private constant ENV_DEPLOYER_PRIVATE_KEY = "DEPLOYER_PRIVATE_KEY";
    string private constant ENV_IDENTITY_REGISTRY_ADDRESS = "IDENTITY_REGISTRY_ADDRESS";
    string private constant ENV_COMMITMENT_HASH_REGISTRY_ADDRESS = "COMMITMENT_HASH_REGISTRY_ADDRESS";

    function setUp() public {
        deployScript = new DeployHTLC();
        deployScript.setUp();

        deployerPrivateKey = vm.envOr(ENV_DEPLOYER_PRIVATE_KEY, uint256(0x1));
        expectedDeployer = vm.addr(deployerPrivateKey);

        vm.setEnv(ENV_DEPLOYER_PRIVATE_KEY, vm.toString(deployerPrivateKey));
        vm.setEnv(ENV_IDENTITY_REGISTRY_ADDRESS, vm.toString(address(0x6789012345678901234567890123456789012345)));
        vm.setEnv(
            ENV_COMMITMENT_HASH_REGISTRY_ADDRESS, vm.toString(address(0x1234567890123456789012345678901234567890))
        );
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
