# IHashTimeLockedContract
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/c83cda8b94a84ac16a0315dd4782ddc7f679cccf/src/interfaces/IHashTimeLockedContract.sol)

**Title:**
IHashTimeLockedContract

Interface for the Hash Time-Lock Contract (Scenario A: Enhanced Correspondent Banking).

Orchestrates the cross-border atomic settlement via cryptographic escrows.


## Functions
### lock

Locks the specified amount of tCeBm tokens into the contract.


```solidity
function lock(
    bytes32 contractId,
    address receiver,
    address token,
    uint256 amount,
    bytes32 hashLock,
    uint256 timeLock
) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`contractId`|`bytes32`|Unique identifier for the agreement.|
|`receiver`|`address`|The address of the beneficiary.|
|`token`|`address`|The address of the tCeBm ERC20 token.|
|`amount`|`uint256`|The amount of tokens to lock.|
|`hashLock`|`bytes32`|The SHA-256 hash of the secret.|
|`timeLock`|`uint256`|The Unix timestamp after which the funds can be refunded.|


### settle

Settles the HTLC by providing the secret preimage.


```solidity
function settle(bytes32 contractId, bytes32 secret) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`contractId`|`bytes32`|The unique identifier of the locked contract.|
|`secret`|`bytes32`|The plaintext string/bytes that hashes to the hashLock.|


### refund

Refunds the locked tokens to the sender if the timeLock has expired.


```solidity
function refund(bytes32 contractId) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`contractId`|`bytes32`|The unique identifier of the locked contract.|


### getLockDetails

Retrieves the full details of a specific lock contract.


```solidity
function getLockDetails(bytes32 contractId) external view returns (HashTimeLockedContractLibrary.LockDetails memory);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`contractId`|`bytes32`|The unique identifier of the locked contract.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`<none>`|`HashTimeLockedContractLibrary.LockDetails`|LockDetails struct containing the current state and parameters.|


## Events
### LogHTLCLocked
Emitted when funds are successfully locked in escrow.


```solidity
event LogHTLCLocked(
    bytes32 indexed contractId,
    address indexed sender,
    address indexed receiver,
    address token,
    uint256 amount,
    bytes32 hashLock,
    uint256 timeLock
);
```

### LogHTLCClaimed
Emitted when the correct secret is provided and funds are transferred to the receiver.


```solidity
event LogHTLCClaimed(bytes32 indexed contractId, bytes32 secret);
```

### LogHTLCRefunded
Emitted when the time-lock expires and funds are returned to the sender.


```solidity
event LogHTLCRefunded(bytes32 indexed contractId);
```

## Errors
### HTLC__ContractAlreadyExists
Custom errors for gas-efficient HTLC failure paths.


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

