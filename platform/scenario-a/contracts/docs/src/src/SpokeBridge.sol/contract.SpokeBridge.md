# SpokeBridge
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/SpokeBridge.sol)

**Inherits:**
[ISpokeBridge](/src/interfaces/ISpokeBridge.sol/interface.ISpokeBridge.md), AccessControl, ReentrancyGuard

**Title:**
SpokeBridge

Lock-and-Mint bridge on the spoke side (REQ-CAP-005, Scenario B).

Verified participants lock domestic tCeBM tokens. A governance relayer calls `release`
to return tokens when the Hub-side wrapped token is burned.
Idempotency: each txId can only be used once (NFR-RES-003).


## State Variables
### GOVERNANCE_ROLE
Role identifier for the governance relayer that can release locked assets.


```solidity
bytes32 public constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE")
```


### IDENTITY_REGISTRY
The Identity Registry used for participant clearance gates.


```solidity
IIdentityRegistry public immutable IDENTITY_REGISTRY
```


### _locks
Stores lock records indexed by txId.


```solidity
mapping(bytes32 => LockRecord) private _locks
```


### _txExists
Tracks whether a txId has been used (idempotency).


```solidity
mapping(bytes32 => bool) private _txExists
```


## Functions
### constructor

Initializes the bridge with governance and identity verification.


```solidity
constructor(address _identityRegistry, address _admin) ;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`_identityRegistry`|`address`|Address of the spoke's IdentityRegistry.|
|`_admin`|`address`|Address that receives DEFAULT_ADMIN_ROLE (can grant GOVERNANCE_ROLE).|


### onlyVerified

Ensures the given account is a verified participant in the IdentityRegistry.


```solidity
modifier onlyVerified(address account) ;
```

### _onlyVerified

Internal check extracted from the modifier to reduce bytecode duplication at call sites.


```solidity
function _onlyVerified(address account) internal view;
```

### lock

Locks `amount` of `token` in the bridge under the given `txId`.

Caller must be verified. Reverts if the txId has already been used (idempotency).


```solidity
function lock(address token, uint256 amount, bytes32 txId) external nonReentrant onlyVerified(msg.sender);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`token`|`address`|The ERC20 token to lock.|
|`amount`|`uint256`|The amount to lock.|
|`txId`|`bytes32`|Unique cross-chain transaction identifier.|


### release

Releases previously locked tokens back to the original depositor.

Only callable by GOVERNANCE_ROLE. Reverts if the txId was not locked or already released.


```solidity
function release(bytes32 txId) external nonReentrant onlyRole(GOVERNANCE_ROLE);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`txId`|`bytes32`|The transaction identifier of the lock to release.|


### getLock

Returns the lock details for a given txId.

Reverts with SB__TxNotFound when the txId does not exist.


```solidity
function getLock(bytes32 txId)
    external
    view
    returns (address sender, address token, uint256 amount, bool released);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`txId`|`bytes32`|The transaction identifier.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`sender`|`address`|The original depositor.|
|`token`|`address`|The locked ERC20 token.|
|`amount`|`uint256`|The locked amount.|
|`released`|`bool`|Whether the lock has been released.|


## Structs
### LockRecord
Internal struct to store lock state.


```solidity
struct LockRecord {
    address sender;
    address token;
    uint256 amount;
    bool released;
}
```

