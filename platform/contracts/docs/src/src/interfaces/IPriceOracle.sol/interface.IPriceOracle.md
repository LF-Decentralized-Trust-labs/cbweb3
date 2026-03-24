# IPriceOracle
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/e19c9456a3de8cd6d3345c4656e5c68bf8aefa97/src/interfaces/IPriceOracle.sol)

**Title:**
IPriceOracle

Designed to be compatible with future Chainlink AggregatorV3-style feeds.

Swappable oracle interface for FX rate retrieval (REQ-FX-002).


## Functions
### getRate

Returns the exchange rate between two tokens.


```solidity
function getRate(address token0, address token1) external view returns (uint256 rate, uint8 decimals);
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


