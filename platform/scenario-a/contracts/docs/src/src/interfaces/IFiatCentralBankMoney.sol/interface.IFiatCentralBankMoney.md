# IFiatCentralBankMoney
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/interfaces/IFiatCentralBankMoney.sol)

**Title:**
IFiatCentralBankMoney

Interface for the FiatCentralBankMoney (fCeBM) ERC-20 contract.

The fCeBM represents fiat central bank money on the Besu ledger. It is minted
by the Central Bank upon deposit approval and burned during the escrow flow
(fCeBM → tCeBM swap). Only the CENTRAL_BANK_ROLE may invoke mint and burn.


## Functions
### mint

Mints new fCeBM tokens to a specified address.

Caller must hold the CENTRAL_BANK_ROLE.


```solidity
function mint(address to, uint256 amount) external;
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
function burn(address from, uint256 amount) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`from`|`address`|The address from which tokens will be burned.|
|`amount`|`uint256`|The amount of tokens to burn.|


## Events
### FiatMinted
Emitted when the Central Bank mints fCeBM tokens to an address.


```solidity
event FiatMinted(address indexed to, uint256 amount);
```

**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`to`|`address`|The recipient address.|
|`amount`|`uint256`|The amount of tokens minted.|

### FiatBurned
Emitted when the Central Bank burns fCeBM tokens from an address.


```solidity
event FiatBurned(address indexed from, uint256 amount);
```

**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`from`|`address`|The address whose tokens are burned.|
|`amount`|`uint256`|The amount of tokens burned.|

