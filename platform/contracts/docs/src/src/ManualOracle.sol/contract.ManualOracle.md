# ManualOracle
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/e19c9456a3de8cd6d3345c4656e5c68bf8aefa97/src/ManualOracle.sol)

**Inherits:**
[IManualOracle](/src/interfaces/IManualOracle.sol/interface.IManualOracle.md), AccessControl

**Title:**
ManualOracle

MVP oracle implementation where Central Banks manually set FX rates (REQ-FX-002).

Implements IManualOracle (which extends IPriceOracle). Designed to be swapped for a
Chainlink integration later. Only accounts with CENTRAL_BANK_ROLE can update rates.


## State Variables
### CENTRAL_BANK_ROLE
Role identifier for the Central Bank rate-setter.


```solidity
bytes32 public constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE")
```


### _rates
Stores rates indexed by keccak256(token0, token1).


```solidity
mapping(bytes32 => RateEntry) private _rates
```


## Functions
### constructor

Initializes the oracle with admin and central bank roles.


```solidity
constructor(address admin, address centralBank) ;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`admin`|`address`|The address granted DEFAULT_ADMIN_ROLE.|
|`centralBank`|`address`|The address granted CENTRAL_BANK_ROLE.|


### setRate

Sets the exchange rate for a token pair.

Only callable by CENTRAL_BANK_ROLE. Rate is stored with 18 decimal precision.


```solidity
function setRate(address token0, address token1, uint256 rate) external onlyRole(CENTRAL_BANK_ROLE);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`token0`|`address`|Address of the base token.|
|`token1`|`address`|Address of the quote token.|
|`rate`|`uint256`|The exchange rate scaled by 1e18.|


### getRate

Returns the exchange rate between two tokens.

Returns the rate with a fixed 18 decimal precision.


```solidity
function getRate(address token0, address token1) external view override returns (uint256 rate, uint8 decimals);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`token0`|`address`|Address of the base token.|
|`token1`|`address`|Address of the quote token.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`rate`|`uint256`|The exchange rate scaled by `decimals`.|
|`decimals`|`uint8`|Number of decimals used in the rate representation.|


### _pairKey

Computes a deterministic key for a token pair (order-sensitive).


```solidity
function _pairKey(address token0, address token1) internal pure returns (bytes32 key);
```

## Structs
### RateEntry
Internal structure for rate storage with an existence flag.


```solidity
struct RateEntry {
    uint256 rate;
    bool isSet;
}
```

