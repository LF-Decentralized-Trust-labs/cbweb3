# FiatCentralBankMoney
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/FiatCentralBankMoney.sol)

**Inherits:**
ERC20, AccessControl, [IFiatCentralBankMoney](/src/interfaces/IFiatCentralBankMoney.sol/interface.IFiatCentralBankMoney.md)

**Title:**
FiatCentralBankMoney (fCeBM)

This contract represents the on-chain fiat asset that commercial banks receive
upon deposit approval by the Central Bank. The fCeBM tokens can later be exchanged
for TokenizedCentralBankMoney (tCeBM) via the escrow flow:
- Escrow: Central Bank burns fCeBM from the commercial bank's Besu wallet and mints
the equivalent tCeBM on the Paladin/Zeto privacy layer.
- Redeem: Central Bank burns tCeBM on Paladin/Zeto and mints fCeBM back to the
commercial bank's Besu wallet.
Only the CENTRAL_BANK_ROLE may mint and burn tokens. The DEFAULT_ADMIN_ROLE
(Network Governor) manages role assignments.

ERC-20 representation of fiat central bank money on the Besu ledger.


## State Variables
### CENTRAL_BANK_ROLE
Role identifier for the Central Bank, which is allowed to mint and burn tokens.

Utilises the SafeERC20 library for all IERC20 token operations within this contract,
ensuring safer interactions with ERC20 tokens by handling potential failures and
return values according to the ERC20 standard.


```solidity
bytes32 public constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE")
```


## Functions
### constructor

Constructor to initialize the fCeBM token and establish governance.


```solidity
constructor(string memory name_, string memory symbol_, address admin, address centralBank) ERC20(name_, symbol_);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`name_`|`string`|The name of the token (e.g., "Fiat BRL").|
|`symbol_`|`string`|The symbol of the token (e.g., "fCeBM_BRL").|
|`admin`|`address`|The address to be granted the DEFAULT_ADMIN_ROLE (Network Governor).|
|`centralBank`|`address`|The address to be granted the CENTRAL_BANK_ROLE (Monetary Authority).|


### mint

Mints new fCeBM tokens to a specified address.

Caller must hold the CENTRAL_BANK_ROLE.


```solidity
function mint(address to, uint256 amount) external onlyRole(CENTRAL_BANK_ROLE);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`to`|`address`|The address to receive the newly minted tokens.|
|`amount`|`uint256`|The amount of tokens to mint.|


### burn

Burns fCeBM tokens from a specified address.

Caller must hold the CENTRAL_BANK_ROLE. The Central Bank, as the sovereign
monetary authority, may burn from any address during escrow settlement.


```solidity
function burn(address from, uint256 amount) external onlyRole(CENTRAL_BANK_ROLE);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`from`|`address`|The address from which tokens will be burned.|
|`amount`|`uint256`|The amount of tokens to burn.|


