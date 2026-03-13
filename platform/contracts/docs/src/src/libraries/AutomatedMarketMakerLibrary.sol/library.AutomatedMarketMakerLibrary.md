# AutomatedMarketMakerLibrary
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/4cd0c0e1e4b2b4fc9cca7cd214e363327de8cb65/src/libraries/AutomatedMarketMakerLibrary.sol)

**Title:**
AutomatedMarketMakerLibrary

Core data structures and custom errors from AMM logic.


## Errors
### AMM__ZeroAddress
Custom errors for exact-output pricing and pool interactions


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

