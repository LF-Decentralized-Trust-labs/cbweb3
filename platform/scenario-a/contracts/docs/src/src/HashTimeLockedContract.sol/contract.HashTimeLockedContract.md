# HashTimeLockedContract
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/HashTimeLockedContract.sol)

**Inherits:**
[IHashTimeLockedContract](/src/interfaces/IHashTimeLockedContract.sol/interface.IHashTimeLockedContract.md)

**Title:**
HashTimeLockedContract (HTLC) — Coordination Layer

Records hashLock/timeLock state and emits events for cross-chain relay (Cacti).
Actual token escrow is handled privately via Zeto lock/unlock on the Paladin
sidecar. This contract is the publicly observable coordination point that
enables atomic settlement between sovereign spokes.


## State Variables
### IDENTITY_REGISTRY
The Identity Registry used for participant clearance gates.


```solidity
IIdentityRegistry public immutable IDENTITY_REGISTRY
```


### _locks
Stores lock records indexed by `contractId`.


```solidity
mapping(bytes32 => HashTimeLockedContractLibrary.LockDetails) private _locks
```


## Functions
### constructor

Initializes the HTLC with the IdentityRegistry for clearance gates.


```solidity
constructor(address _identityRegistry) ;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`_identityRegistry`|`address`|Address of the IdentityRegistry contract.|


### onlyVerified

Ensures the given account is a verified participant in the IdentityRegistry.


```solidity
modifier onlyVerified(address account) ;
```

### _onlyVerified

Internal check to reduce bytecode duplication at call sites.


```solidity
function _onlyVerified(address account) internal view;
```

### lock

Records a lock coordination entry linked to a private Zeto lock.


```solidity
function lock(bytes32 contractId, address receiver, bytes32 hashLock, uint256 timeLock, bytes32 zetoLockRef)
    external
    onlyVerified(msg.sender)
    onlyVerified(receiver);
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
function getLockDetails(bytes32 contractId)
    external
    view
    returns (HashTimeLockedContractLibrary.LockDetails memory);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`contractId`|`bytes32`|The unique identifier of the locked contract.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`<none>`|`HashTimeLockedContractLibrary.LockDetails`|LockDetails struct containing the current state and parameters.|


