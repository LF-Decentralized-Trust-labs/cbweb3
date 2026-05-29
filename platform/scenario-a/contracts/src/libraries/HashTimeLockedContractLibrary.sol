// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

/// @title HashTimeLockedContractLibrary
/// @dev Core data structures for the Scenario A HTLC coordination layer.
/// @dev This is a coordination-only contract: actual token movement is handled
///      privately via Zeto lock/unlock through Paladin. Values and amounts are
///      kept confidential (ZK commitments); only hashLock, timeLock, and events
///      are publicly observable for cross-chain atomicity via Cacti.
library HashTimeLockedContractLibrary {
    /// @dev Represents the Finite State Machine (FSM) of a specific lock contract.
    enum HTLCState {
        INVALID,
        PENDING,
        LOCKED,
        SETTLED,
        REFUNDED
    }

    /// @dev Coordination record linking the public HTLC to a private Zeto lock.
    struct LockDetails {
        address sender;
        address receiver;
        bytes32 hashLock;
        uint256 timeLock;
        bytes32 secret;
        bytes32 zetoLockRef;
        HTLCState state;
    }
}
