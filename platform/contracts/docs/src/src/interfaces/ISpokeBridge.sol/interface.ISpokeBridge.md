# ISpokeBridge
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/interfaces/ISpokeBridge.sol)

**Title:**
ISpokeBridge

Locks domestic tCeBM tokens on the spoke so that a relayer can mint
wrapped tokens on the Hub for AMM participation. Release returns tokens
when the Hub-side wrapped token is burned.

Interface for the Spoke-side Lock-and-Mint bridge (REQ-CAP-005, Scenario B).


## Functions
### lock

Locks `amount` of `token` in the bridge under the given `txId`.

Caller must be verified in the IdentityRegistry. Reverts if `txId` already used.


```solidity
function lock(address token, uint256 amount, bytes32 txId) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`token`|`address`|The ERC20 token to lock.|
|`amount`|`uint256`|The amount to lock.|
|`txId`|`bytes32`|Unique cross-chain transaction identifier.|


### release

Releases previously locked tokens back to the original depositor.

Only callable by an account with GOVERNANCE_ROLE. Reverts if `txId` not found.


```solidity
function release(bytes32 txId) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`txId`|`bytes32`|The transaction identifier of the lock to release.|


### getLock

Returns the lock details for a given txId.


```solidity
function getLock(bytes32 txId) external view returns (address sender, address token, uint256 amount, bool released);
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


## Events
### AssetLocked
Emitted when tokens are locked in the bridge.


```solidity
event AssetLocked(bytes32 indexed txId, address indexed sender, address token, uint256 amount, uint256 timestamp);
```

### AssetReleased
Emitted when locked tokens are released back to the original sender.


```solidity
event AssetReleased(bytes32 indexed txId, address indexed recipient, address token, uint256 amount);
```

## Errors
### SB__TxAlreadyProcessed
The txId has already been processed (idempotency guard — NFR-RES-003).


```solidity
error SB__TxAlreadyProcessed();
```

### SB__TxNotFound
The txId does not exist or was not locked.


```solidity
error SB__TxNotFound();
```

### SB__Unauthorized
Caller is not authorised for this action.


```solidity
error SB__Unauthorized();
```

### SB__ParticipantNotVerified
Participant not verified in the IdentityRegistry.


```solidity
error SB__ParticipantNotVerified(address account);
```

### SB__InvalidParameters
Invalid input parameters (zero amount, zero address, etc.).


```solidity
error SB__InvalidParameters();
```

