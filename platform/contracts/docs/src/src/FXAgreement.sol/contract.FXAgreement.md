# FXAgreement
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/e19c9456a3de8cd6d3345c4656e5c68bf8aefa97/src/FXAgreement.sol)

**Inherits:**
[IFXAgreement](/src/interfaces/IFXAgreement.sol/interface.IFXAgreement.md), ReentrancyGuard

**Title:**
FXAgreement

On-chain bilateral FX agreement registry (REQ-FX-001, REQ-FX-003).

Manages the lifecycle of cross-border FX deals: propose → accept/reject/cancel → settle.
All participants must be verified in the IdentityRegistry (clearance gate).
Settlement is restricted to governance-capable accounts (Central Banks).


## State Variables
### IDENTITY_REGISTRY
The Identity Registry used for participant clearance gates.


```solidity
IIdentityRegistry public immutable IDENTITY_REGISTRY
```


### _agreements
Stores FX agreements indexed by `tradeId`.


```solidity
mapping(bytes32 => FXAgreementLibrary.FxAgreement) private _agreements
```


## Functions
### constructor

Initializes the FXAgreement registry with the IdentityRegistry for clearance gates.


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

Internal check extracted from the modifier to reduce bytecode duplication at call sites.


```solidity
function _onlyVerified(address account) internal view;
```

### propose

Creates a new FX agreement proposal.

Reverts if the trade already exists, addresses are zero, amounts are zero, or the proposal is expired.


```solidity
function propose(
    bytes32 tradeId,
    address counterpartyB,
    address baseToken,
    address quoteToken,
    uint256 notional,
    uint256 rate,
    uint256 expiryDate
) external onlyVerified(msg.sender) onlyVerified(counterpartyB);
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

Only callable by counterpartyB while the agreement is in PROPOSED state and not expired.


```solidity
function accept(bytes32 tradeId) external onlyVerified(msg.sender);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|The trade to accept.|


### reject

Counterparty rejects the proposed agreement.

Only callable by counterpartyB while the agreement is in PROPOSED state.


```solidity
function reject(bytes32 tradeId) external onlyVerified(msg.sender);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|The trade to reject.|


### cancel

Initiator cancels a proposed agreement before acceptance.

Only callable by counterpartyA while the agreement is in PROPOSED state.


```solidity
function cancel(bytes32 tradeId) external onlyVerified(msg.sender);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|The trade to cancel.|


### settle

Governance settles an accepted agreement (marks as executed).

Restricted to governance-capable accounts (Central Banks). Requires ACCEPTED state.


```solidity
function settle(bytes32 tradeId) external nonReentrant;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|The trade to settle.|


### getAgreement

Returns the full details of an FX agreement.

Reverts with FXA__TradeNotFound when the trade does not exist.


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


