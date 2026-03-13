# AutomatedMarketMaker
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/4cd0c0e1e4b2b4fc9cca7cd214e363327de8cb65/src/AutomatedMarketMaker.sol)

**Inherits:**
[IAutomatedMarketMaker](/src/interfaces/IAutomatedMarketMaker.sol/interface.IAutomatedMarketMaker.md), ReentrancyGuard, Pausable, AccessControl

**Title:**
Automated Market Maker (AMM)

Constant Product Liquidity Pool for Scenario B (Exact-Output pricing).


## State Variables
### GOVERNANCE_ROLE
Role identifier for Governance, which can trigger the circuit breaker.


```solidity
bytes32 public constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE")
```


### TOKEN_A
The ERC20 tokens in the liquidity pool


```solidity
IERC20 public immutable TOKEN_A
```


### TOKEN_B

```solidity
IERC20 public immutable TOKEN_B
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

Initializes the AMM with the token pair and RBAC.


```solidity
constructor(address _tokenA, address _tokenB, address _admin, address _governance) ;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`_tokenA`|`address`|Address of the first token (e.g., tCeBM_BRL).|
|`_tokenB`|`address`|Address of the second token (e.g., tCeBM_EUR).|
|`_admin`|`address`|Address to receive DEFAULT_ADMIN_ROLE.|
|`_governance`|`address`|Address to receive GOVERNANCE_ROLE.|


### setPause

Circuit breaker: Pauses or unpauses all pool operations.

Only the GOVERNANCE_ROLE can call this function.


```solidity
function setPause(bool status) external onlyRole(GOVERNANCE_ROLE);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`status`|`bool`|True to pause, False to unpause.|


### addLiquidity

Adds initial or subsequent liquidity to the pool.


```solidity
function addLiquidity(uint256 amountA, uint256 amountB) external nonReentrant whenNotPaused;
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
) external nonReentrant whenNotPaused returns (uint256 amountIn);
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


