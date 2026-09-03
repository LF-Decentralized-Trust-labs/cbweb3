# IFXAgreement
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/interfaces/IFXAgreement.sol)

**Title:**
IFXAgreement

Interface for the on-chain bilateral FX agreement registry (REQ-FX-001).

Manages the lifecycle of cross-border FX deals: propose → accept/reject/cancel → settle.


## Functions
### propose

Creates a new FX agreement proposal.


```solidity
function propose(
    bytes32 tradeId,
    address counterpartyB,
    address baseToken,
    address quoteToken,
    uint256 notional,
    uint256 rate,
    uint256 expiryDate
) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|Unique identifier for the deal.|
|`counterpartyB`|`address`|The address of the accepting party.|
|`baseToken`|`address`|Token address of the base currency.|
|`quoteToken`|`address`|Token address of the quote currency.|
|`notional`|`uint256`|Notional amount of the deal.|
|`rate`|`uint256`|Agreed exchange rate (scaled by 1e18).|
|`expiryDate`|`uint256`|Unix timestamp after which the proposal expires.|


### accept

Counterparty accepts the proposed agreement.


```solidity
function accept(bytes32 tradeId) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|The trade to accept.|


### reject

Counterparty rejects the proposed agreement.


```solidity
function reject(bytes32 tradeId) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|The trade to reject.|


### cancel

Initiator cancels a proposed agreement before acceptance.


```solidity
function cancel(bytes32 tradeId) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|The trade to cancel.|


### settle

Governance settles an accepted agreement (marks as executed).


```solidity
function settle(bytes32 tradeId) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|The trade to settle.|


### getAgreement

Returns the full details of an FX agreement.


```solidity
function getAgreement(bytes32 tradeId) external view returns (FXAgreementLibrary.FxAgreement memory);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|The trade to query.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`<none>`|`FXAgreementLibrary.FxAgreement`|The FXAgreement struct.|


## Events
### AgreementProposed
Emitted when a new FX agreement is proposed.


```solidity
event AgreementProposed(
    bytes32 indexed tradeId, address indexed counterpartyA, address indexed counterpartyB, uint256 notional
);
```

### AgreementAccepted
Emitted when the counterparty accepts the agreement.


```solidity
event AgreementAccepted(bytes32 indexed tradeId);
```

### AgreementSettled
Emitted when the agreement is settled (funds exchanged).


```solidity
event AgreementSettled(bytes32 indexed tradeId);
```

### AgreementRejected
Emitted when the counterparty rejects the agreement.


```solidity
event AgreementRejected(bytes32 indexed tradeId);
```

### AgreementCancelled
Emitted when the initiator cancels a proposed agreement.


```solidity
event AgreementCancelled(bytes32 indexed tradeId);
```

## Errors
### FXA__TradeAlreadyExists
TradeID already exists.


```solidity
error FXA__TradeAlreadyExists();
```

### FXA__TradeNotFound
TradeID does not exist or is in INVALID state.


```solidity
error FXA__TradeNotFound();
```

### FXA__InvalidStateTransition
Agreement is not in the expected state for this transition.


```solidity
error FXA__InvalidStateTransition();
```

### FXA__Unauthorized
Caller is not authorised for this action.


```solidity
error FXA__Unauthorized();
```

### FXA__ParticipantNotVerified
Participant not verified in the IdentityRegistry.


```solidity
error FXA__ParticipantNotVerified(address account);
```

### FXA__AgreementExpired
Agreement has expired.


```solidity
error FXA__AgreementExpired();
```

### FXA__InvalidParameters
Invalid input parameters.


```solidity
error FXA__InvalidParameters();
```

