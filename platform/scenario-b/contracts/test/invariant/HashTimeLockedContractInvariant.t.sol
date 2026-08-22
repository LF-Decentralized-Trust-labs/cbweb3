// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {console} from "forge-std/console.sol";
import {StdInvariant} from "forge-std/StdInvariant.sol";
import {HashTimeLockedContract} from "../../src/HashTimeLockedContract.sol";
import {IdentityRegistry} from "../../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../../src/libraries/IdentityRegistryLibrary.sol";
import {HTLCHandler} from "./HTLCHandler.sol";

/// @title HashTimeLockedContractInvariantTest
/// @notice Invariant suite for the HTLC state machine.
///
/// The HTLC holds no tokens: it is the public coordination layer for a Zeto lock on the
/// Paladin sidecar, referenced by `zetoLockRef`. So these invariants are about the
/// machine staying a machine — LOCKED leads to SETTLED or REFUNDED, once, with no way
/// back and no way to reach either without the condition that authorizes it.
///
/// That matters more here than in an ordinary state machine, because the on-chain state
/// is what the relay reads to decide whether to release private value. A lock that
/// could be both settled and refunded, or refunded early, or re-locked over a finished
/// record, would desynchronize this layer from the money.
contract HashTimeLockedContractInvariantTest is StdInvariant, Test {
    HashTimeLockedContract internal htlc;
    IdentityRegistry internal registry;
    HTLCHandler internal handler;

    address internal admin = makeAddr("admin");
    address internal sender = makeAddr("sender");
    address internal receiver = makeAddr("receiver");
    address internal stranger = makeAddr("stranger");

    function setUp() public {
        registry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        _verify(sender, "Sender Bank");
        _verify(receiver, "Receiver Bank");
        _verify(stranger, "Stranger Bank");
        vm.stopPrank();

        // No FXAgreement and no CommitmentHashRegistry: with both at address(0) the
        // agreement gate is skipped, so these invariants exercise the state machine
        // itself rather than the acceptance checks in front of it. Those gates have
        // their own unit tests; mixing them in here would mean most sequences never
        // reached a lock at all.
        htlc = new HashTimeLockedContract(address(registry), address(0), address(0));
        handler = new HTLCHandler(htlc, sender, receiver, stranger);

        targetContract(address(handler));
    }

    function _verify(address who, string memory name) private {
        registry.registerParticipant(
            who,
            name,
            IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK,
            bytes32(0),
            keccak256(abi.encodePacked("inst-", who))
        );
        registry.verifyParticipant(who);
    }

    // ============================================================================
    //                               INVARIANTS
    // ============================================================================

    /// @notice The hash gate is the entire security of the scheme: a settle without the
    ///         preimage would let anyone claim any lock.
    function invariant_settleRequiresThePreimage() public {
        handler.sweep();
        assertFalse(handler.settledWithWrongSecret(), "a lock settled with the wrong secret");
    }

    /// @notice A refund before the deadline would let the sender take the value back
    ///         while the receiver can still legitimately settle — both sides paid.
    function invariant_refundWaitsForTheTimeLock() public {
        handler.sweep();
        assertFalse(handler.refundedBeforeTimeLock(), "a lock was refunded before its timelock");
    }

    /// @notice R2-H-1: only the original sender may refund. A stranger triggering it
    ///         would desynchronize this public layer from the private Zeto lock.
    function invariant_onlyTheSenderCanRefund() public {
        handler.sweep();
        assertFalse(handler.refundedByNonSender(), "a non-sender refunded a lock");
    }

    /// @notice The same contractId must never be locked twice: a second lock would
    ///         overwrite a settled or refunded record with a fresh LOCKED one.
    function invariant_aContractIdIsLockedOnlyOnce() public {
        handler.sweep();
        assertFalse(handler.relockSucceeded(), "an existing contractId was locked again");
    }

    /// @notice SETTLED and REFUNDED are terminal and mutually exclusive. This is the
    ///         property the relay depends on: it reads one state and acts once.
    function invariant_terminalStatesAreFinalAndExclusive() public {
        handler.sweep();
        assertFalse(handler.bothSettledAndRefunded(), "a lock was both settled and refunded");
        assertFalse(handler.terminalStateChanged(), "a terminal state was left or overwritten");
        assertFalse(handler.terminalLockReTransitioned(), "a lock in a terminal state accepted a second transition");
    }

    /// @notice The stored secret is the settle receipt. Finding one on a lock that was
    ///         never settled would mean the preimage leaked into state without the
    ///         transition that justifies publishing it.
    function invariant_theSecretOnlyAppearsOnSettle() public {
        handler.sweep();
        assertFalse(handler.secretSetWithoutSettle(), "a secret is stored on a lock that is not settled");
    }

    /// @dev Per-run counters; state resets between runs, so reachability is proven
    ///      deterministically in HTLCHandlerReachabilityTest rather than asserted here.
    function afterInvariant() public view {
        console.log("locks           ", handler.callsLock());
        console.log("settles         ", handler.callsSettle());
        console.log("refunds         ", handler.callsRefund());
        console.log("relock attempts ", handler.callsRelockAttempt());
        console.log("early refunds   ", handler.callsRefundEarlyAttempt());
        console.log("tracked locks   ", handler.trackedCount());
    }
}
