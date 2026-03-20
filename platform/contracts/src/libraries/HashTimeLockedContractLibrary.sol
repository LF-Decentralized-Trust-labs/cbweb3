// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

/// @title HashTimeLockedContractLibrary
/// @dev Core data structures and custom errors for the Scenario A HTLC logic.
library HashTimeLockedContractLibrary {
    /// @dev Represents the Finite State Machine (FSM) of a specific lock contract.
    enum HTLCState {
        INVALID,
        PENDING,
        LOCKED,
        SETTLED,
        REFUNDED
    }

    /// @dev Data structure holding the state of a specific atomic swap.
    struct LockDetails {
        address sender;
        address receiver;
        address token;
        uint256 amount;
        bytes32 hashLock;
        uint256 timeLock;
        bytes32 secret;
        HTLCState state;
    }

}
