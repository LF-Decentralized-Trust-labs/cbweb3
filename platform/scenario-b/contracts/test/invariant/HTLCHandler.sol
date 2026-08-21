// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {CommonBase} from "forge-std/Base.sol";
import {StdCheats} from "forge-std/StdCheats.sol";
import {StdUtils} from "forge-std/StdUtils.sol";
import {HashTimeLockedContract} from "../../src/HashTimeLockedContract.sol";
import {HashTimeLockedContractLibrary as HTLCLib} from "../../src/libraries/HashTimeLockedContractLibrary.sol";

/// @title HTLCHandler
/// @notice Bounded action driver for the HTLC state-machine invariants.
///
/// This contract holds no tokens and neither does the HTLC: it is the public
/// coordination layer for a Zeto lock that lives on the Paladin sidecar, referenced by
/// `zetoLockRef`. So the properties worth pinning are not about balances — they are
/// about the state machine staying a state machine: LOCKED goes to SETTLED or
/// REFUNDED, once, and never comes back.
///
/// The handler keeps its own record of every lock it created, with the preimage, the
/// timelock and the sender. That record is what makes the invariants checkable: the
/// contract exposes the current state, but only the handler knows what the state was
/// SUPPOSED to become given what it asked for.
contract HTLCHandler is CommonBase, StdCheats, StdUtils {
    HashTimeLockedContract public immutable HTLC;

    address public immutable SENDER;
    address public immutable RECEIVER;
    address public immutable STRANGER;

    struct Tracked {
        bytes32 contractId;
        bytes32 secret;
        uint256 timeLock;
        bool settledByUs;
        bool refundedByUs;
    }

    Tracked[] private _tracked;
    mapping(bytes32 => uint256) private _indexOf; // contractId -> index+1

    // ----- violations, recorded rather than asserted here so each surfaces as a
    // ----- named invariant with its own message.
    bool public settledWithWrongSecret;
    bool public refundedBeforeTimeLock;
    bool public refundedByNonSender;
    bool public relockSucceeded;
    bool public terminalStateChanged;
    bool public bothSettledAndRefunded;
    bool public secretSetWithoutSettle;

    /// @notice Why the last relock attempt failed. Without this a relock that always
    ///         reverts for an unrelated reason looks like a guard doing its job.
    bytes public lastRelockRevert;

    uint256 public callsLock;
    uint256 public callsSettle;
    uint256 public callsRefund;
    uint256 public callsRelockAttempt;
    uint256 public callsRefundEarlyAttempt;

    constructor(HashTimeLockedContract htlc_, address sender_, address receiver_, address stranger_) {
        HTLC = htlc_;
        SENDER = sender_;
        RECEIVER = receiver_;
        STRANGER = stranger_;
    }

    // ============================================================================
    //                                 ACTIONS
    // ============================================================================

    /// @notice Create a lock with a preimage the handler remembers.
    function lock(uint256 seed, uint256 ttl) external {
        bytes32 secret = keccak256(abi.encode("secret", seed));
        bytes32 contractId = keccak256(abi.encode("lock", seed, _tracked.length));
        bytes32 hashLock = sha256(abi.encodePacked(secret));
        ttl = _bound(ttl, 1 hours, 30 days);
        uint256 timeLock = block.timestamp + ttl;

        vm.prank(SENDER);
        try HTLC.lock(contractId, RECEIVER, hashLock, timeLock, bytes32(0), bytes32(0)) {
            callsLock++;
            _tracked.push(Tracked(contractId, secret, timeLock, false, false));
            _indexOf[contractId] = _tracked.length;
        } catch {}
    }

    /// @notice Settle a tracked lock with the CORRECT preimage.
    function settle(uint256 seed) external {
        if (_tracked.length == 0) return;
        uint256 i = _bound(seed, 0, _tracked.length - 1);
        Tracked storage t = _tracked[i];

        vm.prank(RECEIVER);
        try HTLC.settle(t.contractId, t.secret) {
            callsSettle++;
            t.settledByUs = true;
        } catch {}
    }

    /// @notice Settle with a WRONG preimage. Must always fail; if it ever succeeds the
    ///         hash gate is broken, which is the whole security of the scheme.
    function settleWithWrongSecret(uint256 seed) external {
        if (_tracked.length == 0) return;
        uint256 i = _bound(seed, 0, _tracked.length - 1);
        Tracked storage t = _tracked[i];
        bytes32 wrong = keccak256(abi.encode("wrong", seed));
        if (wrong == t.secret) return;

        vm.prank(RECEIVER);
        try HTLC.settle(t.contractId, wrong) {
            settledWithWrongSecret = true;
        } catch {}
    }

    /// @notice Refund after the timelock, as the original sender (the only legal path).
    function refundAfterExpiry(uint256 seed) external {
        if (_tracked.length == 0) return;
        uint256 i = _bound(seed, 0, _tracked.length - 1);
        Tracked storage t = _tracked[i];

        if (block.timestamp < t.timeLock) {
            vm.warp(t.timeLock + 1);
        }
        vm.prank(SENDER);
        try HTLC.refund(t.contractId) {
            callsRefund++;
            t.refundedByUs = true;
        } catch {}
    }

    /// @notice Refund BEFORE the timelock. Must always fail.
    function refundEarly(uint256 seed) external {
        if (_tracked.length == 0) return;
        uint256 i = _bound(seed, 0, _tracked.length - 1);
        Tracked storage t = _tracked[i];
        if (block.timestamp >= t.timeLock) return; // no longer early; nothing to prove

        callsRefundEarlyAttempt++;
        vm.prank(SENDER);
        try HTLC.refund(t.contractId) {
            refundedBeforeTimeLock = true;
        } catch {}
    }

    /// @notice Refund as someone other than the sender, after expiry. Must always fail
    ///         (R2-H-1): a stranger triggering the refund would desynchronize this
    ///         public layer from the private Zeto lock.
    function refundAsStranger(uint256 seed) external {
        if (_tracked.length == 0) return;
        uint256 i = _bound(seed, 0, _tracked.length - 1);
        Tracked storage t = _tracked[i];

        if (block.timestamp < t.timeLock) {
            vm.warp(t.timeLock + 1);
        }
        vm.prank(STRANGER);
        try HTLC.refund(t.contractId) {
            refundedByNonSender = true;
        } catch {}
    }

    /// @notice Re-lock an existing contractId. Must always fail, whatever state it is
    ///         in — reusing an id would overwrite a settled or refunded record.
    function relock(uint256 seed) external {
        if (_tracked.length == 0) return;
        uint256 i = _bound(seed, 0, _tracked.length - 1);
        Tracked storage t = _tracked[i];

        callsRelockAttempt++;
        // Compute the hash BEFORE the prank. sha256 is a precompile, so evaluating it
        // inside the argument list spends the prank on the precompile call and the lock
        // then arrives from this handler — an unverified address. The attempt reverted
        // with ParticipantNotVerified every time, so the "no relock" invariant passed
        // while never testing the guard it names. Mutation testing found it: removing
        // the state check from the contract changed nothing.
        bytes32 hashLock = sha256(abi.encodePacked(t.secret));
        vm.prank(SENDER);
        try HTLC.lock(t.contractId, RECEIVER, hashLock, block.timestamp + 1 days, bytes32(0), bytes32(0)) {
            relockSucceeded = true;
        } catch (bytes memory err) {
            lastRelockRevert = err;
        }
    }

    /// @notice Let time pass, so refunds become reachable without a targeted action.
    function passTime(uint256 seconds_) external {
        vm.warp(block.timestamp + _bound(seconds_, 1 minutes, 10 days));
    }

    // ============================================================================
    //                        POST-CONDITION SWEEP
    // ============================================================================

    /// @notice Walks every tracked lock and records any state the machine should not be
    ///         able to reach. Called by the invariant so the checks see the same view
    ///         the contract has, rather than a snapshot taken mid-sequence.
    function sweep() external {
        for (uint256 i = 0; i < _tracked.length; i++) {
            Tracked storage t = _tracked[i];
            HTLCLib.LockDetails memory d = HTLC.getLockDetails(t.contractId);

            if (t.settledByUs && t.refundedByUs) bothSettledAndRefunded = true;

            // A lock we settled must still read SETTLED; one we refunded, REFUNDED.
            // Anything else means a terminal state was left behind.
            if (t.settledByUs && d.state != HTLCLib.HTLCState.SETTLED) terminalStateChanged = true;
            if (t.refundedByUs && d.state != HTLCLib.HTLCState.REFUNDED) terminalStateChanged = true;

            // The secret is the settle receipt: it must not appear on a lock that was
            // never settled.
            if (d.state != HTLCLib.HTLCState.SETTLED && d.secret != bytes32(0)) secretSetWithoutSettle = true;
        }
    }

    function trackedCount() external view returns (uint256) {
        return _tracked.length;
    }

    function totalCalls() external view returns (uint256) {
        return callsLock + callsSettle + callsRefund;
    }
}
