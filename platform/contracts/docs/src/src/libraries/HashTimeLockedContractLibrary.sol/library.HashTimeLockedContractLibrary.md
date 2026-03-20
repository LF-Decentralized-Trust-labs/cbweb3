# HashTimeLockedContractLibrary
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/c83cda8b94a84ac16a0315dd4782ddc7f679cccf/src/libraries/HashTimeLockedContractLibrary.sol)

**Title:**
HashTimeLockedContractLibrary

Core data structures and custom errors for the Scenario A HTLC logic.


## Structs
### LockDetails
Data structure holding the state of a specific atomic swap.


```solidity
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

