# AutomatedMarketMaker
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/AutomatedMarketMaker.sol)

**Inherits:**
[IAutomatedMarketMaker](/src/interfaces/IAutomatedMarketMaker.sol/interface.IAutomatedMarketMaker.md), ReentrancyGuard, Pausable

**Title:**
Automated Market Maker (AMM)

Constant Product Liquidity Pool for Scenario B (Exact-Output pricing).
All identity and role checks are delegated to the IdentityRegistry (single source of truth).


## State Variables
### TOKEN_A
The ERC20 tokens in the liquidity pool


```solidity
IERC20 public immutable TOKEN_A
```


### TOKEN_B

```solidity
IERC20 public immutable TOKEN_B
```


### IDENTITY_REGISTRY
The Identity Registry used for participant clearance gates.


```solidity
IIdentityRegistry public immutable IDENTITY_REGISTRY
```


### reserveA
The current reserves of the pool to compute the constant product (x * y = k)


```solidity
uint256 public reserveA
```


### reserveB

```solidity
uint256 public reserveB
```


## Functions
### constructor

Initializes the AMM with the token pair and IdentityRegistry.


```solidity
constructor(address _tokenA, address _tokenB, address _identityRegistry) ;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`_tokenA`|`address`|Address of the first token (e.g., tCeBM_BRL).|
|`_tokenB`|`address`|Address of the second token (e.g., tCeBM_EUR).|
|`_identityRegistry`|`address`|Address of the IdentityRegistry (single source of truth for roles).|


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

### onlyGovernance

Restricts access to accounts with a governance-capable role in the IdentityRegistry.


```solidity
modifier onlyGovernance() ;
```

### _onlyGovernance

Internal governance check — delegates to the IdentityRegistry (single source of truth).


```solidity
function _onlyGovernance() internal view;
```

### setPause

Circuit breaker: Pauses or unpauses all pool operations.

Only governance-capable participants (CENTRAL_BANK, GOVERNANCE) can call this.


```solidity
function setPause(bool status) external onlyGovernance;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`status`|`bool`|True to pause, False to unpause.|


### addLiquidity

Adds initial or subsequent liquidity to the pool.


```solidity
function addLiquidity(uint256 amountA, uint256 amountB)
    external
    nonReentrant
    whenNotPaused
    onlyVerified(msg.sender);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`amountA`|`uint256`|The amount of token A to add.|
|`amountB`|`uint256`|The amount of token B to add.|


### getAmountIn

Calculates the required input amount for a desired output amount.


```solidity
function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut)
    public
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
) external nonReentrant whenNotPaused onlyVerified(msg.sender) onlyVerified(to) returns (uint256 amountIn);
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


