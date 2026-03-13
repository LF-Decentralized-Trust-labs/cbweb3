# TokenizedCentralBankMoney
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/4cd0c0e1e4b2b4fc9cca7cd214e363327de8cb65/src/TokenizedCentralBankMoney.sol)

**Inherits:**
ERC20, AccessControl

**Title:**
TokenizedCentralBankMoney (tCeBm)

This contract acts as the base fiat-pegged asset for the HTLC and AMM pools. It includes RBAC to restrict minting and burning capabilities strictly to the designated Central Bank Authority.

Core asset mocked implementation for the CBWeb3 MVP


## State Variables
### CENTRAL_BANK_ROLE
Role identifier for the Central Bank, which is allowed to mint and burn tokens.

Utilises the SafeERC20 library for all IERC20 token operations within this contract, ensuring safer interactions with ERC20 tokens by handling potential failures and return values according to the ERC20 standard.


```solidity
bytes32 public constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE")
```


## Functions
### constructor

Constructor to initialize the tCeBm token and establish governance.


```solidity
constructor(string memory name_, string memory symbol_, address admin, address centralBank) ERC20(name_, symbol_);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`name_`|`string`|The name of the token (e.g., "Tokenized BRL").|
|`symbol_`|`string`|The symbol of the token (e.g., "tCeBM_BRL").|
|`admin`|`address`|The address to be granted the DEFAULT_ADMIN_ROLE (Network Governor).|
|`centralBank`|`address`|The address to be granted the CENTRAL_BANK_ROLE (Monetary Authority).|


### mint

Mints new tokens to a specified address.

Reverts if the caller does not have the CENTRAL_BANK_ROLE.


```solidity
function mint(address to, uint256 amount) external onlyRole(CENTRAL_BANK_ROLE);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`to`|`address`|The address to receive the newly minted tokens.|
|`amount`|`uint256`|The amount of tokens to mint.|


### burn

Burns tokens from a specified address.

Reverts if the caller does not have the CENTRAL_BANK_ROLE.

Note: As the supreme authority on the domestic ledger, the CB can burn from any address.


```solidity
function burn(address from, uint256 amount) external onlyRole(CENTRAL_BANK_ROLE);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`from`|`address`|The address from which tokens will be burned.|
|`amount`|`uint256`|The amount of tokens to burn.|


