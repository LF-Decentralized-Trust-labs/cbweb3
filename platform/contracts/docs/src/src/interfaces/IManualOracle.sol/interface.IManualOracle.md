# IManualOracle
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/interfaces/IManualOracle.sol)

**Inherits:**
[IPriceOracle](/src/interfaces/IPriceOracle.sol/interface.IPriceOracle.md)

**Title:**
IManualOracle

Interface for the manually-governed FX rate oracle (REQ-FX-002).

Extends IPriceOracle with administrative functions for Central Bank rate management.
Designed to be swapped for a Chainlink AggregatorV3-style feed in production.


## Functions
### setRate

Sets the exchange rate for a token pair.


```solidity
function setRate(address token0, address token1, uint256 rate) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`token0`|`address`|Address of the base token.|
|`token1`|`address`|Address of the quote token.|
|`rate`|`uint256`|The exchange rate scaled by 1e18.|


## Events
### RateUpdated
Emitted when a Central Bank updates the exchange rate for a token pair.


```solidity
event RateUpdated(address indexed token0, address indexed token1, uint256 rate);
```

## Errors
### Oracle__RateNotSet
The requested rate has not been set for this token pair.


```solidity
error Oracle__RateNotSet();
```

### Oracle__InvalidParameters
Invalid input parameters (zero address, zero rate, etc.).


```solidity
error Oracle__InvalidParameters();
```

