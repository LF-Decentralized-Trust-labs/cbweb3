# HashTimeLockedContractLibrary
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/libraries/HashTimeLockedContractLibrary.sol)

**Title:**
HashTimeLockedContractLibrary

Core data structures for the Scenario A HTLC coordination layer.

This is a coordination-only contract: actual token movement is handled
privately via Zeto lock/unlock through Paladin. Values and amounts are
kept confidential (ZK commitments); only hashLock, timeLock, and events
are publicly observable for cross-chain atomicity via Cacti.


## Structs
### LockDetails
Coordination record linking the public HTLC to a private Zeto lock.


```solidity
struct LockDetails {
    address sender;
    address receiver;
    bytes32 hashLock;
    uint256 timeLock;
    bytes32 secret;
    bytes32 zetoLockRef;
    HTLCState state;
}
```

## Enums
### HTLCState
Represents the Finite State Machine (FSM) of a specific lock contract.


```solidity
enum HTLCState {
    INVALID,
    PENDING,
    LOCKED,
    SETTLED,
    REFUNDED
}
```

