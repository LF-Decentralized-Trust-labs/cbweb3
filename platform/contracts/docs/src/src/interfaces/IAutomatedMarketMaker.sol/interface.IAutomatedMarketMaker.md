# IAutomatedMarketMaker
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/c83cda8b94a84ac16a0315dd4782ddc7f679cccf/src/interfaces/IAutomatedMarketMaker.sol)

**Title:**
IAutomatedMarketMaker

Interface for the Constant Product AMM Liquidity Pool.


## Functions
### addLiquidity

Adds initial or subsequent liquidity to the pool.


```solidity
function addLiquidity(uint256 amountA, uint256 amountB) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`amountA`|`uint256`|The amount of token A to add.|
|`amountB`|`uint256`|The amount of token B to add.|


### swapTokensForExactTokens

Swaps tokens aiming for an EXACT output amount (Exact-Output pricing).

Implements slippage protection via maxAmountIn.


```solidity
function swapTokensForExactTokens(
    address tokenIn,
    address tokenOut,
    uint256 amountOut,
    uint256 maxAmountIn,
    address to
) external returns (uint256 amountIn);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`tokenIn`|`address`|The address of the token the user is paying.|
|`tokenOut`|`address`|The address of the token the user wants to receive.|
|`amountOut`|`uint256`|The exact amount of tokenOut the user wants.|
|`maxAmountIn`|`uint256`|The maximum amount of tokenIn the user is willing to pay (Slippage protection).|
|`to`|`address`|The address that will receive the output tokens.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`amountIn`|`uint256`|The calculated amount of tokenIn actually deducted.|


### getAmountIn

Calculates the required input amount for a desired output amount.


```solidity
function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut)
    external
    pure
    returns (uint256 amountIn);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`reserveIn`|`uint256`|The current reserve of the input token.|
|`reserveOut`|`uint256`|The current reserve of the output token.|
|`amountOut`|`uint256`|The exact amount of output tokens desired.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`amountIn`|`uint256`|The mathematically required amount of input tokens.|


## Events
### LogLiquidityAdded
Emitted when liquidity is added to the pool.


```solidity
event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB);
```

### LogSwap
Emitted when a swap is successfully executed.


```solidity
event LogSwap(
    address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut
);
```

## Errors
### AMM__ZeroAddress
Custom errors for exact-output pricing and pool interactions.


```solidity
error AMM__ZeroAddress();
```

### AMM__ZeroAmount

```solidity
error AMM__ZeroAmount();
```

### AMM__InvalidToken

```solidity
error AMM__InvalidToken();
```

### AMM__InsufficientLiquidity

```solidity
error AMM__InsufficientLiquidity();
```

### AMM__InsufficientOutputAmount

```solidity
error AMM__InsufficientOutputAmount();
```

### AMM__SlippageExceeded

```solidity
error AMM__SlippageExceeded(uint256 requiredAmountIn, uint256 maxAmountIn);
```

