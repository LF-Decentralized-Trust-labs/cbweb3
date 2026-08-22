// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test} from "forge-std/Test.sol";
import {HashTimeLockedContract} from "../../src/HashTimeLockedContract.sol";
import {IHashTimeLockedContract} from "../../src/interfaces/IHashTimeLockedContract.sol";
import {HashTimeLockedContractLibrary as HTLCLib} from "../../src/libraries/HashTimeLockedContractLibrary.sol";
import {IdentityRegistry} from "../../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../../src/libraries/IdentityRegistryLibrary.sol";
import {HTLCHandler} from "./HTLCHandler.sol";

/// @title HTLCHandlerReachabilityTest
/// @notice Proves each action the HTLC handler offers reaches the contract, and — for
///         the actions that MUST fail — that they fail for the stated reason rather
///         than an accidental one.
///
/// That second half is the point, and it is not hypothetical. The handler's relock
/// attempt computed `sha256(...)` inside the call's argument list; sha256 is a
/// precompile, so evaluating it there spent the `vm.prank` on the precompile and the
/// lock arrived from the handler itself — an unverified address. Every attempt reverted
/// with ParticipantNotVerified, so the "a contractId is locked only once" invariant
/// passed without once exercising the guard it names. Removing that guard from the
/// contract changed nothing, which is how it was found.
///
/// A negative test that cannot distinguish "rejected for the right reason" from
/// "rejected for the wrong one" is not a test of the rule.
contract HTLCHandlerReachabilityTest is Test {
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

        htlc = new HashTimeLockedContract(address(registry), address(0), address(0));
        handler = new HTLCHandler(htlc, sender, receiver, stranger);
    }

    function _verify(address who, string memory name) private {
        registry.registerParticipant(who, name, IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0));
        registry.verifyParticipant(who);
    }

    function test_lockIsReachable() public {
        handler.lock(1, 2 days);
        assertEq(handler.callsLock(), 1, "lock never succeeded");
        assertEq(handler.trackedCount(), 1, "the lock was not tracked");
    }

    function test_settleIsReachable() public {
        handler.lock(2, 2 days);
        handler.settle(0);
        assertEq(handler.callsSettle(), 1, "settle never succeeded");
    }

    function test_refundAfterExpiryIsReachable() public {
        handler.lock(3, 2 days);
        handler.refundAfterExpiry(0);
        assertEq(handler.callsRefund(), 1, "refund never succeeded");
    }

    /// @notice A settled lock must refuse a second settle, and refuse it as "not
    ///         locked" rather than for any other reason.
    ///
    /// The invariant that covers this is only able to see the violation because the
    /// handler records a success on an already-terminal lock. That is a subtle enough
    /// mechanism to deserve a deterministic partner: SETTLED -> SETTLED changes no
    /// state, so the sweep comparing recorded state against reported state sees
    /// nothing, and a mutation permitting it passed all six invariants until the
    /// handler learned to flag it.
    ///
    /// It matters because a second settle re-emits LogHTLCClaimed for one contractId,
    /// and the relay reads those events to release private value.
    function test_settleIsRefusedOnAnAlreadySettledLock() public {
        handler.lock(9, 2 days);
        handler.settle(0);
        assertEq(handler.callsSettle(), 1, "the first settle did not succeed");

        HTLCLib.LockDetails memory d = htlc.getLockDetails(_firstContractId(9));
        vm.prank(receiver);
        vm.expectRevert(IHashTimeLockedContract.HTLC__ContractNotLocked.selector);
        htlc.settle(_firstContractId(9), d.secret);
    }

    /// @notice And the same on the other terminal state.
    function test_refundIsRefusedOnAnAlreadyRefundedLock() public {
        handler.lock(10, 2 days);
        handler.refundAfterExpiry(0);
        assertEq(handler.callsRefund(), 1, "the first refund did not succeed");

        vm.prank(sender);
        vm.expectRevert(IHashTimeLockedContract.HTLC__ContractNotLocked.selector);
        htlc.refund(_firstContractId(10));
    }

    /// @dev Mirrors the handler's id derivation for the first lock of a fresh handler.
    function _firstContractId(uint256 seed) private pure returns (bytes32) {
        return keccak256(abi.encode("lock", seed, uint256(0)));
    }

    /// @notice The relock attempt must reach the CONTRACT'S guard. If it is rejected
    ///         before that — an unverified caller, an expired timelock — the invariant
    ///         built on it proves nothing.
    function test_relockReachesTheContractGuard() public {
        handler.lock(4, 2 days);
        handler.relock(0);

        assertEq(handler.callsRelockAttempt(), 1, "relock was not attempted");
        assertFalse(handler.relockSucceeded(), "an existing contractId was locked again");
        assertEq(
            bytes4(handler.lastRelockRevert()),
            IHashTimeLockedContract.HTLC__ContractAlreadyExists.selector,
            "relock was rejected for the wrong reason; the guard under test was never reached"
        );
    }

    /// @notice Same requirement for the wrong-secret attempt: it must be the hash gate
    ///         that rejects it.
    function test_wrongSecretIsRejectedByTheHashGate() public {
        handler.lock(5, 2 days);
        handler.settleWithWrongSecret(5);
        assertFalse(handler.settledWithWrongSecret(), "a wrong secret settled a lock");
        // The lock must still be claimable with the right one afterwards.
        handler.settle(0);
        assertEq(handler.callsSettle(), 1, "the lock became unsettleable after a wrong-secret attempt");
    }

    function test_earlyRefundIsAttemptedAndRejected() public {
        handler.lock(6, 2 days);
        handler.refundEarly(0);
        assertEq(handler.callsRefundEarlyAttempt(), 1, "the early-refund path was never attempted");
        assertFalse(handler.refundedBeforeTimeLock(), "a refund landed before the timelock");
    }

    function test_strangerRefundIsAttemptedAndRejected() public {
        handler.lock(7, 2 days);
        handler.refundAsStranger(0);
        assertFalse(handler.refundedByNonSender(), "a non-sender refunded a lock");
        // And the real sender can still refund, so the rejection was about identity.
        handler.refundAfterExpiry(0);
        assertEq(handler.callsRefund(), 1, "the sender could not refund after a stranger tried");
    }

    function test_sweepSeesTerminalStates() public {
        handler.lock(8, 2 days);
        handler.settle(0);
        handler.sweep();
        assertFalse(handler.terminalStateChanged(), "sweep reported a terminal-state change on a clean run");
        assertFalse(handler.secretSetWithoutSettle(), "sweep reported a stray secret on a clean run");
        HTLCLib.LockDetails memory d = htlc.getLockDetails(keccak256(abi.encode("lock", uint256(8), uint256(0))));
        assertEq(uint8(d.state), uint8(HTLCLib.HTLCState.SETTLED), "the lock did not reach SETTLED");
    }
}
