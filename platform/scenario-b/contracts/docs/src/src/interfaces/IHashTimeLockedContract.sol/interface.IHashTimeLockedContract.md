# IHashTimeLockedContract
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/interfaces/IHashTimeLockedContract.sol)

**Title:**
IHashTimeLockedContract

Coordination-only interface for the Hash Time-Lock Contract (Scenario A).

Records hashLock/timeLock and emits events for cross-chain relay (Cacti).
Actual token movement is handled by Zeto lock/unlock via Paladin sidecar.


## Functions
### lock

Records a lock coordination entry linked to a private Zeto lock.


```solidity
function lock(bytes32 contractId, address receiver, bytes32 hashLock, uint256 timeLock, bytes32 zetoLockRef)
    external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`contractId`|`bytes32`|Unique identifier for the agreement.|
|`receiver`|`address`|The address of the beneficiary.|
|`hashLock`|`bytes32`|The SHA-256 hash of the secret.|
|`timeLock`|`uint256`|The Unix timestamp after which the lock can be refunded.|
|`zetoLockRef`|`bytes32`|Reference to the private Zeto lock transaction.|


### settle

Settles the HTLC by providing the secret preimage.


```solidity
function settle(bytes32 contractId, bytes32 secret) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`contractId`|`bytes32`|The unique identifier of the locked contract.|
|`secret`|`bytes32`|The plaintext bytes that hash to the hashLock.|


### refund

Marks the lock as refunded after the timeLock has expired.


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
Emitted when a lock coordination record is created.


```solidity
event LogHTLCLocked(
    bytes32 indexed contractId,
    address indexed sender,
    address indexed receiver,
    bytes32 hashLock,
    uint256 timeLock,
    bytes32 zetoLockRef
);
```

### LogHTLCClaimed
Emitted when the correct secret is provided and the lock is settled.


```solidity
event LogHTLCClaimed(bytes32 indexed contractId, bytes32 secret);
```

### LogHTLCRefunded
Emitted when the time-lock expires and the lock is refunded.


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

### HTLC__ParticipantNotVerified

```solidity
error HTLC__ParticipantNotVerified(address account);
```

