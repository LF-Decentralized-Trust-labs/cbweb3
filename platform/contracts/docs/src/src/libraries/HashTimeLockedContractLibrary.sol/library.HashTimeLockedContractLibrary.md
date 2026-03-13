# HashTimeLockedContractLibrary
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/4cd0c0e1e4b2b4fc9cca7cd214e363327de8cb65/src/libraries/HashTimeLockedContractLibrary.sol)

**Title:**
HashTimeLockedContractLibrary

Core data structures and custom errors for the Scenario A HTLC logic.


## Errors
### HTLC__ContractAlreadyExists
Custom errors for gas optimization


```solidity
error HTLC__ContractAlreadyExists();
```

### HTLC__ContractNotLocked

```solidity
error HTLC__ContractNotLocked();
```

### HTLC__InvalidSecret

```solidity
error HTLC__InvalidSecret();
```

### HTLC__TimeLockNotExpired

```solidity
error HTLC__TimeLockNotExpired();
```

### HTLC__TimeLockExpired

```solidity
error HTLC__TimeLockExpired();
```

### HTLC__InvalidAmount

```solidity
error HTLC__InvalidAmount();
```

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

