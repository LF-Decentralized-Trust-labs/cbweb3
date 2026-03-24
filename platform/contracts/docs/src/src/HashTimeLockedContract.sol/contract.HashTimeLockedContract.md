# HashTimeLockedContract
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/e19c9456a3de8cd6d3345c4656e5c68bf8aefa97/src/HashTimeLockedContract.sol)

**Inherits:**
[IHashTimeLockedContract](/src/interfaces/IHashTimeLockedContract.sol/interface.IHashTimeLockedContract.md), ReentrancyGuard

**Title:**
HashTimeLockedContract (HTLC)

This contract locks ERC20 funds, allows settlement with the correct secret preimage,
and allows refund after expiry. It is protected against re-entrancy on state-changing flows.

Escrow contract for atomic settlement flows using hash-lock and time-lock controls.


## State Variables
### IDENTITY_REGISTRY
The Identity Registry used for participant clearance gates.

Utilises SafeERC20 wrappers for secure ERC20 transfers.


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
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The address to verify.|


### _onlyVerified

Internal check extracted from the modifier to reduce bytecode duplication at call sites.


```solidity
function _onlyVerified(address account) internal view;
```

### lock

Locks funds in escrow under a hash-lock and time-lock.

Reverts if the lock already exists, amount is zero, or time-lock is already expired.


```solidity
function lock(
    bytes32 contractId,
    address receiver,
    address token,
    uint256 amount,
    bytes32 hashLock,
    uint256 timeLock
) external nonReentrant onlyVerified(msg.sender) onlyVerified(receiver);
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

Settles a lock by revealing a valid secret preimage.

Reverts unless the lock is in `LOCKED` state and the secret hashes to the stored hash-lock.


```solidity
function settle(bytes32 contractId, bytes32 secret) external nonReentrant;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`contractId`|`bytes32`|The unique identifier of the locked contract.|
|`secret`|`bytes32`|The plaintext string/bytes that hashes to the hashLock.|


### refund

Refunds locked funds to the original sender after expiry.

Reverts unless the lock is in `LOCKED` state and the time-lock has expired.


```solidity
function refund(bytes32 contractId) external nonReentrant;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`contractId`|`bytes32`|The unique identifier of the locked contract.|


### getLockDetails

Returns the full lock details for a given contract identifier.

Returns zero-initialised values when the lock does not exist.


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


